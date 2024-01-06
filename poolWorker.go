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

var workChannel chan Video

func Dispatcher(ctx context.Context, queue *Queue, config *Config, waitGroup *sync.WaitGroup) {
	workChannel = make(chan Video, config.Workers)

	for i := 0; i < config.Workers; i++ {
		go worker(i, ctx, queue, config, workChannel, waitGroup)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			video, ok := queue.DequeueItem()
			if ok {
				workChannel <- video
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func worker(id int, ctx context.Context, queue *Queue, config *Config, workChannel <-chan Video, waitGroup *sync.WaitGroup) {
	for video := range workChannel {
		waitGroup.Add(1)
		processVideo(id, ctx, queue, video, config)
		// TODO: dependency injection for sqlite
		// I don't like the idea of having it global
		err := sqlite.MarkVideoAsDone(&video)
		if err != nil {
			log.Panic(err)
		}

		waitGroup.Done()
	}
}

func processVideo(id int, ctx context.Context, queue *Queue, video Video, config *Config) {
	log.WithFields(StructFields(video)).Info("Processing video")

	processFolderWorker := path.Join(config.ProcessFolder, fmt.Sprintf("worker_%d", id))
	if _, err := os.Stat(processFolderWorker); err == nil {
		err := os.RemoveAll(processFolderWorker)
		if err != nil {
			log.Panic(err)
		}
	}

	err := os.MkdirAll(processFolderWorker, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}

	fps, err := GetVideoFPS(video.Path)
	if err != nil {
		log.Panic(err)
	}

	log.Debugf("fps: %f", fps)
	if fps >= 30 {
		log.Info("FPS is higher then 30, skipping")
		// TODO: implement real skip, right now it won't skip
		// it will mark is as done
		return
	}

	fps30Output := path.Join(processFolderWorker, "video.mp4")
	output, err := ConvertVideoTo30FPS(ctx, video.Path, fps30Output)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished converting to 30 fps")
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err = ExtractAudio(ctx, fps30Output, audioPath)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished extracting audio")
	framesFolder := path.Join(processFolderWorker, "frames")
	err = os.Mkdir(framesFolder, os.ModePerm)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	output, err = ExtractFrames(ctx, fps30Output, framesFolder)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished extracting frames")
	interpolatedFolder := path.Join(processFolderWorker, "interpolated_frames")
	err = os.Mkdir(interpolatedFolder, os.ModePerm)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	output, err = InterpolateVideo(ctx, config.RifeBinary, framesFolder, interpolatedFolder, config.Model)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	// make sure the folder exist
	baseOutputPath := path.Base(video.OutputPath)
	if _, err := os.Stat(baseOutputPath); err != nil {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			log.Debug(output)
			log.Panic(err)
		}
	}

	log.Debug("Finished interpolating video")
	output, err = ConstructVideoTo60FPS(ctx, interpolatedFolder, audioPath, video.OutputPath)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		log.Panic(err)
	}
}
