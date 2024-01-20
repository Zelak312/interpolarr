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
			log.Debugf("Ctx error is: %s", p.ctx.Err())
			if p.ctx.Err() == context.Canceled {
				log.Debug("Ctx was canceled")
				p.waitGroup.Done()
				return
			}
		}

		if processError != nil {
			if output != "" {
				log.Debug(output)
			}

			retries, err := sqlite.GetVideoRetries(&video)
			if err != nil {
				log.WithFields(StructFields(video)).Error("Failed to get retries: ", err)
			}

			if retries >= retryLimit {
				err = sqlite.FailVideo(&video, output, processError.Error())
				if err != nil {
					log.WithFields(StructFields(video)).Error("Failed to fail the video: ", err)
				}

				p.waitGroup.Done()
				return
			} else if err == nil { // if err from getting retries
				retries++ // don't do this, just silently fail the video
				err = sqlite.UpdateVideoRetries(&video, retries)
				if err != nil {
					log.WithFields(StructFields(video)).Error("Failed to update video retries: ", err)
				}
			}

			// only enqueue if there as been
			// no errors to fail or update the video retries (also get)
			if err == nil {
				p.queue.Enqueue(video)
			} else {
				log.Debug("No re enqueing video du to an error above")
			}

			p.waitGroup.Done()
			return
		}

		// TODO: dependency injection for sqlite
		// I don't like the idea of having it global
		if !skip {
			err := sqlite.MarkVideoAsDone(&video)
			if err != nil {
				log.WithFields(StructFields(video)).Error("Failed to mark video as done: ", err)
			}
		} else {
			log.Debug("Copying file to destination since it has been skipped")
			if video.Path != video.OutputPath {
				err := CopyFile(video.Path, video.OutputPath)
				if err != nil {
					log.WithFields(StructFields(video)).Error("Failed to copy file to destination: ", err)
				}
			}

			err := sqlite.DeleteVideoByID(nil, video.ID)
			if err != nil {
				log.WithFields(StructFields(video)).Error("Failed to delete video: ", err)
			}
		}

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
	log.Debugf("fps: %g", fps)
	log.Debugf("target FPS: %g", targetFPS)

	if fps >= targetFPS {
		log.Debug(`Video is already higher or equal to target FPS, skipping`)
		return "", true, nil
	}

	if *p.config.BypassHighFPS && fps > targetFPS/2 {
		log.Debug("Bypassing video because of high FPS, skipping")
		return "", true, nil
	}

	err = os.MkdirAll(processFolderWorker, os.ModePerm)
	if err != nil {
		return "", false, err
	}

	fpsConversionOutput := path.Join(processFolderWorker, "video.mp4")
	output, err := ConvertVideoToFPS(p.ctx, p.config.FfmpegOptions, video.Path, fpsConversionOutput, targetFPS/2)
	if err != nil {
		return output, false, err
	}

	log.Debugf("Finished converting to %g fps", targetFPS/2)
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err = ExtractAudio(p.ctx, fpsConversionOutput, audioPath)
	if err != nil {
		return output, false, err
	}

	log.Debug("Finished extracting audio")
	framesFolder := path.Join(processFolderWorker, "frames")
	err = os.Mkdir(framesFolder, os.ModePerm)
	if err != nil {
		return "", false, err
	}

	output, err = ExtractFrames(p.ctx, p.config.FfmpegOptions, fpsConversionOutput, framesFolder)
	if err != nil {
		return output, false, err
	}

	log.Debug("Finished extracting frames")
	interpolatedFolder := path.Join(processFolderWorker, "interpolated_frames")
	err = os.Mkdir(interpolatedFolder, os.ModePerm)
	if err != nil {
		return "", false, err
	}

	output, err = InterpolateVideo(p.ctx, p.config.RifeBinary, framesFolder, interpolatedFolder, p.config.ModelPath)
	if err != nil {
		return output, false, err
	}

	// make sure the folder exist
	baseOutputPath := path.Dir(video.OutputPath)
	if _, err := os.Stat(baseOutputPath); err != nil {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			return "", false, err
		}
	}

	log.Debug("Finished interpolating video")
	output, err = ConstructVideoToFPS(p.ctx, p.config.FfmpegOptions, interpolatedFolder, audioPath, video.OutputPath, targetFPS)
	if err != nil {
		return output, false, err
	}

	ok, err := IsSamePath(video.Path, video.OutputPath)
	if !ok && *p.config.DeleteInputFileWhenFinished {
		err = os.Remove(video.Path)
		if err != nil {
			return "", false, err
		}
	} else if ok && *p.config.DeleteInputFileWhenFinished {
		log.Debug("Detected same path with delete input file option, not deleting anything!")
	}

	if err != nil {
		return "", false, err
	}

	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		return "", false, err
	}

	return "", false, nil
}
