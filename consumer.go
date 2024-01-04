package main

import (
	"context"
	"os"
	"path"
	"time"

	log "github.com/sirupsen/logrus"
)

func StartConsumer(ctx context.Context, queue *Queue, processFolder string, rifeBinary string, model string) {
	for {
		select {
		case <-ctx.Done():
			// Context is cancelled, exit the goroutine
			return
		default:
			// Regular operation
			video, ok := queue.GetItem()
			if ok {
				processVideo(ctx, queue, video, processFolder, rifeBinary, model)
			} else {
				// Queue is empty, sleep briefly
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func processVideo(ctx context.Context, queue *Queue, video Video, processFolder string, rifeBinary string, model string) {
	log.WithFields(StructFields(video)).Info("Processing video")
	if _, err := os.Stat(processFolder); err == nil {
		err := os.RemoveAll(processFolder)
		if err != nil {
			log.Panic(err)
		}
	}

	err := os.Mkdir(processFolder, os.ModePerm)
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
		queue.Dequeue()
		return
	}

	fps30Output := path.Join(processFolder, "video.mp4")
	output, err := ConvertVideoTo30FPS(ctx, video.Path, fps30Output)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished converting to 30 fps")
	audioPath := path.Join(processFolder, "audio.m4a")
	output, err = ExtractAudio(ctx, fps30Output, audioPath)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished extracting audio")
	framesFolder := path.Join(processFolder, "frames")
	os.Mkdir(framesFolder, os.ModePerm)
	output, err = ExtractFrames(ctx, fps30Output, framesFolder)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished extracting frames")
	interpolatedFolder := path.Join(processFolder, "interpolated_frames")
	os.Mkdir(interpolatedFolder, os.ModePerm)
	output, err = InterpolateVideo(ctx, rifeBinary, framesFolder, interpolatedFolder, model)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	log.Debug("Finished interpolating video")
	output, err = ConstructVideoTo60FPS(ctx, interpolatedFolder, audioPath, video.OutputPath)
	if err != nil {
		log.Debug(output)
		log.Panic(err)
	}

	err = os.RemoveAll(processFolder)
	if err != nil {
		log.Panic(err)
	}

	_, found, err := queue.Dequeue()
	if err != nil {
		log.Panic(err)
	}

	if !found {
		log.Error("Why is video not found?")
	}
	log.Debug("Finished processing video")
}
