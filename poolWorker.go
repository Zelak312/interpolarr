package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	log "github.com/sirupsen/logrus"
)

var workChannel chan Video

func Dispatcher(ctx context.Context, queue *Queue, config *Config) {
	workChannel = make(chan Video, config.Workers)

	for i := 0; i < config.Workers; i++ {
		go worker(i, ctx, queue, config, workChannel)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			video, ok := queue.GetItem()
			if ok {
				workChannel <- video
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func worker(id int, ctx context.Context, queue *Queue, config *Config, workChannel <-chan Video) {
	for video := range workChannel {
		processVideo(id, ctx, queue, video, config)
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
		_, found, err := queue.DequeueVideoByID(video.ID)
		if err != nil {
			log.Panic(err)
		}

		if !found {
			log.Error("Why is video not found?")
		}
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

	_, found, err := queue.DequeueVideoByID(video.ID)
	if err != nil {
		log.Panic(err)
	}

	if !found {
		log.Error("Why is video not found?")
	}
	log.Debug("Finished processing video")
}
