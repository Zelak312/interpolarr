package main

import (
	"context"
	"fmt"
	"math"
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

func (p *PoolWorker) worker(id int, workChannel <-chan Video) {
	for video := range workChannel {
		p.waitGroup.Add(1)
		output, err := p.processVideo(id, video)
		// check if context was canceled
		if p.ctx.Err() == context.Canceled {
			log.Debug("Ctx was canceled")
			p.waitGroup.Done()
			return
		}

		if err != nil {
			if output != "" {
				log.Debug(output)
			}

			log.Panic(err)
		}

		// TODO: dependency injection for sqlite
		// I don't like the idea of having it global
		err = sqlite.MarkVideoAsDone(&video)
		if err != nil {
			log.Panic(err)
		}

		p.waitGroup.Done()
	}
}

func (p *PoolWorker) processVideo(id int, video Video) (string, error) {
	log.WithFields(StructFields(video)).Info("Processing video")

	processFolderWorker := path.Join(p.config.ProcessFolder, fmt.Sprintf("worker_%d", id))
	if _, err := os.Stat(processFolderWorker); err == nil {
		err := os.RemoveAll(processFolderWorker)
		if err != nil {
			return "", err
		}
	}

	err := os.MkdirAll(processFolderWorker, os.ModePerm)
	if err != nil {
		return "", err
	}

	fps, err := GetVideoFPS(p.ctx, video.Path)
	if err != nil {
		return "", err
	}

	log.Debugf("fps: %f", fps)
	targetFPS := p.config.MinimumFPS
	if fps > targetFPS/2 {
		targetFPS = fps * 2
	}

	if *p.config.StabilizeFPS {
		targetFPS = math.Floor(targetFPS)
	}

	log.Debugf("target FPS: %f", targetFPS)
	fpsConversionOutput := path.Join(processFolderWorker, "video.mp4")
	output, err := ConvertVideoToFPS(p.ctx, p.config.FfmpegOptions, video.Path, fpsConversionOutput, targetFPS/2)
	if err != nil {
		return output, err
	}

	log.Debug("Finished converting to 30 fps")
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err = ExtractAudio(p.ctx, fpsConversionOutput, audioPath)
	if err != nil {
		return output, err
	}

	log.Debug("Finished extracting audio")
	framesFolder := path.Join(processFolderWorker, "frames")
	err = os.Mkdir(framesFolder, os.ModePerm)
	if err != nil {
		return "", err
	}

	output, err = ExtractFrames(p.ctx, p.config.FfmpegOptions, fpsConversionOutput, framesFolder)
	if err != nil {
		return output, err
	}

	log.Debug("Finished extracting frames")
	interpolatedFolder := path.Join(processFolderWorker, "interpolated_frames")
	err = os.Mkdir(interpolatedFolder, os.ModePerm)
	if err != nil {
		return "", err
	}

	output, err = InterpolateVideo(p.ctx, p.config.RifeBinary, framesFolder, interpolatedFolder, p.config.ModelPath)
	if err != nil {
		return output, err
	}

	// make sure the folder exist
	baseOutputPath := path.Dir(video.OutputPath)
	if _, err := os.Stat(baseOutputPath); err != nil {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			return "", err
		}
	}

	log.Debug("Finished interpolating video")
	output, err = ConstructVideoToFPS(p.ctx, p.config.FfmpegOptions, interpolatedFolder, audioPath, video.OutputPath, targetFPS)
	if err != nil {
		return output, err
	}

	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		return "", err
	}

	return "", nil
}
