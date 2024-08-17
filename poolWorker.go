package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
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

func (p *PoolWorker) RunDispatcherBlocking() {
	workChannel := make(chan Video, p.config.Workers)

	for i := 0; i < p.config.Workers; i++ {
		// Setup Worker Logger
		logger, err := CreateLogger(fmt.Sprintf("worker%d", i))
		if err != nil {
			log.Panicf("Couldn't create logger for worker: %d", i)
		}

		go p.worker(i, logger, workChannel)
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
// TODO: split this function, it's getting pretty big
// TODO: recheck the structure and content off this function
// it seems like this could be way better
func (p *PoolWorker) worker(id int, log *logrus.Entry, workChannel <-chan Video) {
	for video := range workChannel {
		p.waitGroup.Add(1)
		output, processVideoOutput := p.processVideo(id, log, video)
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

		if processVideoOutput.err != nil {
			log.WithFields(StructFields(video)).Error("Error processing video: ", processVideoOutput.err)
			if output != "" {
				log.Debug("Process ouput: ", output)
			}

			retries, err := sqlite.GetVideoRetries(&video)
			if err != nil {
				log.WithFields(StructFields(video)).Error("Failed to get retries: ", err)
			}

			if retries >= retryLimit {
				log.WithFields(StructFields(video)).Info("Video failed, removing it from queue")
				err = sqlite.FailVideo(&video, output, processVideoOutput.err.Error())
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

		if processVideoOutput.skip {
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

// TODO: add process output in this
type ProcessVideoOutput struct {
	skip                   bool
	outputFileAlreadyExist bool
	err                    error
}

// TODO: split this function, it's getting quite big
func (p *PoolWorker) processVideo(id int, log *logrus.Entry, video Video) (string, ProcessVideoOutput) {
	log.WithFields(StructFields(video)).Info("Processing video")

	// TODO: recheck video.Path is valid
	// need to make sure the video actully exist!!

	outputExist, err := FileExist(video.OutputPath)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	// TODO: I think deleting the output should be done
	// when the output is going to be created
	if outputExist && *p.config.DeleteOutputIfAlreadyExist {
		log.Debug("Output already exist, deleting file")
		err = os.Remove(video.OutputPath)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	} else if outputExist {
		log.Debug("Output already exist, skipping")
		return "", ProcessVideoOutput{outputFileAlreadyExist: true}
	}

	processFolderWorker := path.Join(p.config.ProcessFolder, fmt.Sprintf("worker_%d", id))
	if _, err := os.Stat(processFolderWorker); err == nil {
		err := os.RemoveAll(processFolderWorker)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	}

	fps, err := GetVideoFPS(p.ctx, video.Path)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	targetFPS := p.config.TargetFPS
	log.Info("fps: ", fps)
	log.Info("target FPS: ", targetFPS)

	if fps >= targetFPS {
		log.Info(`Video is already higher or equal to target FPS, skipping`)
		return "", ProcessVideoOutput{skip: true}
	}

	if *p.config.BypassHighFPS && fps > targetFPS/2 {
		log.Info("Bypassing video because of high FPS, skipping")
		return "", ProcessVideoOutput{skip: true}
	}

	log.Debug("Creating worker folder if doesn't exist")
	err = os.MkdirAll(processFolderWorker, os.ModePerm)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	log.Infof("Converting video to %g fps", targetFPS/2)
	fpsConversionOutput := path.Join(processFolderWorker, "video.mp4")
	output, err := ConvertVideoToFPS(p.ctx, p.config.FfmpegOptions, video.Path, fpsConversionOutput, targetFPS/2)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	log.Info("Extracting audio from the video")
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err = ExtractAudio(p.ctx, fpsConversionOutput, audioPath)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	log.Debug("Creating frames folder")
	framesFolder := path.Join(processFolderWorker, "frames")
	err = os.Mkdir(framesFolder, os.ModePerm)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	log.Info("Extracting video frames")
	output, err = ExtractFrames(p.ctx, p.config.FfmpegOptions, fpsConversionOutput, framesFolder)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	log.Debug("Creating interpolation frames folder")
	interpolatedFolder := path.Join(processFolderWorker, "interpolated_frames")
	err = os.Mkdir(interpolatedFolder, os.ModePerm)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	log.Info("Interpolating video")
	output, err = InterpolateVideo(p.ctx, p.config.RifeBinary, framesFolder, interpolatedFolder, p.config.ModelPath)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	// make sure the folder exist
	baseOutputPath := path.Dir(video.OutputPath)
	log.WithField("baseOutputPath", baseOutputPath).
		Debug("Creating output folder if it doesn't exist")
	if _, err := os.Stat(baseOutputPath); err != nil {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	}

	log.Infof("Reconstructing video with audio and interpolated frames to %g fps", targetFPS)
	output, err = ConstructVideoToFPS(p.ctx, p.config.FfmpegOptions, interpolatedFolder, audioPath, video.OutputPath, targetFPS)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	log.Debug("Removing worker folder")
	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	return "", ProcessVideoOutput{}
}
