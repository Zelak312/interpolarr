package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var retryLimit int = 5

type PoolWorker struct {
	ctx       context.Context
	queue     *Queue
	config    *Config
	waitGroup *sync.WaitGroup
}

// TODO: add process output in this
type ProcessVideoOutput struct {
	skip                   bool
	outputFileAlreadyExist bool
	videoNotFound          bool
	err                    error
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

		go p.initWorker(i, logger, workChannel)
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

func (p *PoolWorker) initWorker(id int, log *logrus.Entry, workChannel <-chan Video) {
	for video := range workChannel {
		p.waitGroup.Add(1)
		err := p.runWorker(id, log, &video)
		if p.ctx.Err() != nil {
			log.Debug("Ctx error is: ", p.ctx.Err())
			if p.ctx.Err() == context.Canceled {
				log.Debug("Ctx was canceled")

				// End function so call return
				p.waitGroup.Done()
				return
			}
		}

		if err != nil {
			// TODO: make a place where I can store warnings
			// So I can store warning for each videos
			// Because the otherwise the issues from runWorker (that doesn't retry)
			// Won't show anywhere
		}

		p.waitGroup.Done()
	}
}

func (p *PoolWorker) runWorker(id int, log *logrus.Entry, video *Video) error {
	output, processVideoOutput := p.processVideo(id, log, video)
	if processVideoOutput.err != nil {
		return processVideoOutput.err
	}

	if processVideoOutput.err != nil {
		p.handleProcessVideoError(video, output, &processVideoOutput)
		// Error was handled already
		return nil
	}

	if processVideoOutput.skip && *p.config.CopyFileToDestinationOnSkip {
		log.WithField("srcPath", video.Path).
			WithField("destPath", video.OutputPath).
			Debug("Copying file to destination since it has been skipped")
		ok, err := IsSamePath(video.Path, video.OutputPath)
		if err != nil {
			log.Error("Failed to match same path: ", err)
			return err
		}

		if !ok {
			err := CopyFile(video.Path, video.OutputPath)
			if err != nil {
				log.Error("Failed to copy file to destination: ", err)
				return err
			}

			log.Info("Video file copied sucessfully")
		} else {
			log.Warn("Can't copy file with same path as output path")
			return err
		}
	}

	if processVideoOutput.videoNotFound {
		log.Error("Video to process wasn't found: ", video.Path)
		notFoundErr := errors.New("source video not found")
		p.failVideo(video, output, notFoundErr)
		return notFoundErr
	}

	err := sqlite.MarkVideoAsDone(video)
	if err != nil {
		log.Error("Failed to mark video as done: ", err)
		return err
	}

	if *p.config.DeleteInputFileWhenFinished && !processVideoOutput.outputFileAlreadyExist {
		log.Debug("Deleting input file")
		ok, err := IsSamePath(video.Path, video.OutputPath)
		if err != nil {
			log.Error("Error while looking up same path: ", err)
			return err
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
	return nil
}

func (p *PoolWorker) handleProcessVideoError(video *Video, output string, processVideoOutput *ProcessVideoOutput) {
	log.WithFields(StructFields(video)).Error("Error processing video: ", processVideoOutput.err)
	if output != "" {
		log.Debug("Process ouput: ", output)
	}

	retries, err := sqlite.GetVideoRetries(video)
	if err != nil {
		log.WithFields(StructFields(video)).Error("Failed to get retries: ", err)
		return
	}

	if retries >= retryLimit {
		_ = p.failVideo(video, output, processVideoOutput.err)
		return
	}

	retries++
	err = sqlite.UpdateVideoRetries(video, retries)
	if err != nil {
		log.WithFields(StructFields(video)).Error("Failed to update video retries: ", err)
		return
	}

	p.queue.Enqueue(*video)
	log.WithFields(StructFields(video)).Info("Requeue video (back of the queue and retrying)")
}

func (p *PoolWorker) failVideo(video *Video, output string, failError error) error {
	log.WithFields(StructFields(video)).Info("Video failed, removing it from queue")
	err := sqlite.FailVideo(video, output, failError.Error())
	if err != nil {
		log.WithFields(StructFields(video)).Error("Failed to fail the video: ", err)
		return err
	}

	return nil
}

func (p *PoolWorker) processVideo(id int, log *logrus.Entry, video *Video) (string, ProcessVideoOutput) {
	log.WithFields(StructFields(video)).Info("Processing video")

	videoExist, err := FileExist(video.Path)
	if err != nil {
		return "", ProcessVideoOutput{}
	}

	if !videoExist {
		return "", ProcessVideoOutput{videoNotFound: true}
	}

	outputExist, err := FileExist(video.OutputPath)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	// TODO: I think deleting the output should be done
	// right before the other output is going to be created
	// Actually, future zelak here, I should do that yes
	// but also use move somewhere, delete, then create the file
	// so if there is an issue, I can move back the old file
	// without loss
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
