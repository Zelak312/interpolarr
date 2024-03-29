package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type PoolWorker struct {
	ctx       context.Context
	queue     *Queue
	config    *Config
	waitGroup *sync.WaitGroup
}

func NewPoolWorker(ctx context.Context, queue *Queue,
	config *Config, waitGroup *sync.WaitGroup) PoolWorker {
	return PoolWorker{
		ctx:       ctx,
		queue:     queue,
		config:    config,
		waitGroup: waitGroup,
	}
}

func (p *PoolWorker) RunDispatcher() {
	workChannel := make(chan Video, p.config.Workers)

	for i := 0; i < p.config.Workers; i++ {
		go p.worker(i, workChannel)
	}

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			video, ok := p.queue.Dequeue()
			if ok {
				workChannel <- video
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

var retryLimit int = 5

// Make sure to call waitGroup.Done() at when no errors
// Otherwise the cancel process will get stuck waiting
func (p *PoolWorker) worker(id int, workChannel <-chan Video) {
	for video := range workChannel {
		p.waitGroup.Add(1)
		output, skip, processError := p.processVideo(id, video)
		// check if context was canceled
		if p.ctx.Err() != nil {
			log.Debug("Ctx error is: ", p.ctx.Err())
			if p.ctx.Err() == context.Canceled {
				log.Debug("Ctx was canceled")
				p.waitGroup.Done()
				return
			}

			log.Error("Unknown ctx error")
			p.waitGroup.Done()
			return
		}

		if processError != nil {
			log.WithFields(StructFields(video)).Error("Error processing video: ", processError)
			if output != "" {
				log.Debug("Process ouput: ", output)
			}

			retries, err := sqlite.GetVideoRetries(&video)
			if err != nil {
				log.WithFields(StructFields(video)).Error("Failed to get retries: ", err)
			}

			if retries >= retryLimit {
				log.WithFields(StructFields(video)).Info("Video failed, removing it from queue")
				err = sqlite.FailVideo(&video, output, processError.Error())
				if err != nil {
					log.WithFields(StructFields(video)).Error("Failed to fail the video: ", err)
					p.waitGroup.Done()
					continue
				}

				p.waitGroup.Done()
				continue
			} else if err == nil {
				retries++
				err = sqlite.UpdateVideoRetries(&video, retries)
				if err != nil {
					log.WithFields(StructFields(video)).Error("Failed to update video retries: ", err)
					p.waitGroup.Done()
					continue
				}
			}

			p.queue.Enqueue(video)
			log.WithFields(StructFields(video)).Info("Requeue video (back of the queue and retrying)")
			p.waitGroup.Done()
			continue
		}

		if skip {
			log.WithField("srcPath", video.Path).
				WithField("destPath", video.OutputPath).
				Debug("Copying file to destination since it has been skipped")
			ok, err := IsSamePath(video.Path, video.OutputPath)
			if err != nil {
				log.Error("Failed to match same path: ", err)
				p.waitGroup.Done()
				continue
			}

			if !ok {
				err := CopyFile(video.Path, video.OutputPath)
				if err != nil {
					log.Error("Failed to copy file to destination: ", err)
					p.waitGroup.Done()
					continue
				}

				log.Info("Video file copied sucessfully")
			} else {
				log.Warn("Can't copy file with same path as output path")
				p.waitGroup.Done()
				continue
			}
		}

		err := sqlite.MarkVideoAsDone(&video)
		if err != nil {
			log.Error("Failed to mark video as done: ", err)
			p.waitGroup.Done()
			continue
		}

		if *p.config.DeleteInputFileWhenFinished {
			log.Debug("Deleting input file")
			ok, err := IsSamePath(video.Path, video.OutputPath)
			if err != nil {
				log.Error("Error while looking up same path: ", err)
				p.waitGroup.Done()
				continue
			}

			if !ok {
				err = os.Remove(video.Path)
				if err != nil {
					log.WithFields(StructFields(video)).Error("Failed to delete vidoe: ", err)
				}

				log.WithField("file", video.Path).Info("Deleted input file")
			} else {
				log.WithFields(StructFields(video)).Warn("Detected same path with delete input file option, not deleting anything!")
			}
		}

		log.Info("Finished processing video")
		p.waitGroup.Done()
	}
}

func (p *PoolWorker) processVideo(id int, video Video) (string, bool, error) {
	log.WithFields(StructFields(video)).Info("Processing video")

	processFolderWorker := path.Join(p.config.ProcessFolder, fmt.Sprintf("worker_%d", id))
	if _, err := os.Stat(processFolderWorker); err == nil {
		err := os.RemoveAll(processFolderWorker)
		if err != nil {
			return "", false, err
		}
	}

	fps, err := GetVideoFPS(p.ctx, video.Path)
	if err != nil {
		return "", false, err
	}

	targetFPS := p.config.TargetFPS
	log.Info("fps: ", fps)
	log.Info("target FPS: ", targetFPS)

	if fps >= targetFPS {
		log.Info(`Video is already higher or equal to target FPS, skipping`)
		return "", true, nil
	}

	if *p.config.BypassHighFPS && fps > targetFPS/2 {
		log.Info("Bypassing video because of high FPS, skipping")
		return "", true, nil
	}

	log.Debug("Creating worker folder if doesn't exist")
	err = os.MkdirAll(processFolderWorker, os.ModePerm)
	if err != nil {
		return "", false, err
	}

	log.Infof("Converting video to %g fps", targetFPS/2)
	fpsConversionOutput := path.Join(processFolderWorker, "video.mp4")
	output, err := ConvertVideoToFPS(p.ctx, p.config.FfmpegOptions, video.Path, fpsConversionOutput, targetFPS/2)
	if err != nil {
		return output, false, err
	}

	log.Info("Extracting audio from the video")
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err = ExtractAudio(p.ctx, fpsConversionOutput, audioPath)
	if err != nil {
		return output, false, err
	}

	log.Debug("Creating frames folder")
	framesFolder := path.Join(processFolderWorker, "frames")
	err = os.Mkdir(framesFolder, os.ModePerm)
	if err != nil {
		return "", false, err
	}

	log.Info("Extracting video frames")
	output, err = ExtractFrames(p.ctx, p.config.FfmpegOptions, fpsConversionOutput, framesFolder)
	if err != nil {
		return output, false, err
	}

	log.Debug("Creating interpolation frames folder")
	interpolatedFolder := path.Join(processFolderWorker, "interpolated_frames")
	err = os.Mkdir(interpolatedFolder, os.ModePerm)
	if err != nil {
		return "", false, err
	}

	log.Info("Interpolating video")
	output, err = InterpolateVideo(p.ctx, p.config.RifeBinary, framesFolder, interpolatedFolder, p.config.ModelPath)
	if err != nil {
		return output, false, err
	}

	// make sure the folder exist
	baseOutputPath := path.Dir(video.OutputPath)
	log.WithField("baseOutputPath", baseOutputPath).
		Debug("Creating output folder if it doesn't exist")
	if _, err := os.Stat(baseOutputPath); err != nil {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			return "", false, err
		}
	}

	log.Infof("Reconstructing video with audio and interpolated frames to %g fps", targetFPS)
	output, err = ConstructVideoToFPS(p.ctx, p.config.FfmpegOptions, interpolatedFolder, audioPath, video.OutputPath, targetFPS)
	if err != nil {
		return output, false, err
	}

	log.Debug("Removing worker folder")
	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		return "", false, err
	}

	return "", false, nil
}
