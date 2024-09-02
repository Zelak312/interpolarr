package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

type Worker struct {
	// TODO: for context, always maybe add
	// a time constrained context to make sure
	// nothing is undefinitely running and blocking
	id         int
	logger     *logrus.Entry
	poolWorker *PoolWorker
}

func NewWorker(id int, logger *logrus.Entry, poolWoker *PoolWorker) *Worker {
	return &Worker{
		logger:     logger,
		poolWorker: poolWoker,
	}
}

func (w *Worker) start() {
	for video := range w.poolWorker.workChannel {
		w.poolWorker.waitGroup.Add(1)
		err := w.doWork(&video)
		if w.poolWorker.ctx.Err() != nil {
			w.logger.Debug("Ctx error is: ", w.poolWorker.ctx.Err())
			if w.poolWorker.ctx.Err() == context.Canceled {
				w.logger.Debug("Ctx was canceled")

				// End function so call return
				w.poolWorker.waitGroup.Done()
				return
			}
		}

		if err != nil {
			// TODO: make a place where I can store warnings
			// So I can store warning for each videos
			// Because the otherwise the issues from runWorker (that doesn't retry)
			// Won't show anywhere
		}

		w.poolWorker.waitGroup.Done()
	}
}

func (w *Worker) doWork(video *Video) error {
	output, processVideoOutput := w.processVideo(video)
	if w.poolWorker.ctx.Err() != nil {
		// The context is cancelled, just return
		// it's handled in start
		return nil
	}

	if processVideoOutput.err != nil {
		w.handleProcessVideoError(video, output, &processVideoOutput)
		// Error was handled already
		return nil
	}

	if processVideoOutput.skip && *w.poolWorker.config.CopyFileToDestinationOnSkip {
		w.logger.WithField("srcPath", video.Path).
			WithField("destPath", video.OutputPath).
			Debug("Copying file to destination since it has been skipped")
		ok, err := IsSamePath(video.Path, video.OutputPath)
		if err != nil {
			w.logger.Error("Failed to match same path: ", err)
			return err
		}

		if !ok {
			err := CopyFile(video.Path, video.OutputPath)
			if err != nil {
				w.logger.Error("Failed to copy file to destination: ", err)
				return err
			}

			w.logger.Info("Video file copied sucessfully")
		} else {
			w.logger.Warn("Can't copy file with same path as output path")
			return err
		}
	}

	if processVideoOutput.videoNotFound {
		w.logger.Error("Video to process wasn't found: ", video.Path)
		notFoundErr := errors.New("source video not found")
		w.failVideo(video, output, notFoundErr)
		return notFoundErr
	}

	err := sqlite.MarkVideoAsDone(video)
	if err != nil {
		w.logger.Error("Failed to mark video as done: ", err)
		return err
	}

	if *w.poolWorker.config.DeleteInputFileWhenFinished && !processVideoOutput.outputFileAlreadyExist {
		w.logger.Debug("Deleting input file")
		ok, err := IsSamePath(video.Path, video.OutputPath)
		if err != nil {
			w.logger.Error("Error while looking up same path: ", err)
			return err
		}

		if !ok {
			err = os.Remove(video.Path)
			if err != nil {
				w.logger.WithFields(StructFields(video)).Error("Failed to delete vidoe: ", err)
			}

			w.logger.WithField("file", video.Path).Info("Deleted input file")
		} else {
			w.logger.WithFields(StructFields(video)).Warn("Detected same path with delete input file option, not deleting anything!")
		}
	}

	w.logger.Info("Finished processing video")
	return nil
}

func (w *Worker) handleProcessVideoError(video *Video, output string, processVideoOutput *ProcessVideoOutput) {
	w.logger.WithFields(StructFields(video)).Error("Error processing video: ", processVideoOutput.err)
	if output != "" {
		w.logger.Debug("Process ouput: ", output)
	}

	retries, err := sqlite.GetVideoRetries(video)
	if err != nil {
		w.logger.WithFields(StructFields(video)).Error("Failed to get retries: ", err)
		return
	}

	if retries >= retryLimit {
		_ = w.failVideo(video, output, processVideoOutput.err)
		return
	}

	retries++
	err = sqlite.UpdateVideoRetries(video, retries)
	if err != nil {
		w.logger.WithFields(StructFields(video)).Error("Failed to update video retries: ", err)
		return
	}

	w.poolWorker.queue.Enqueue(*video)
	w.logger.WithFields(StructFields(video)).Info("Requeue video (back of the queue and retrying)")
}

func (w *Worker) failVideo(video *Video, output string, failError error) error {
	w.logger.WithFields(StructFields(video)).Info("Video failed, removing it from queue")
	err := sqlite.FailVideo(video, output, failError.Error())
	if err != nil {
		w.logger.WithFields(StructFields(video)).Error("Failed to fail the video: ", err)
		return err
	}

	return nil
}

func (w *Worker) processVideo(video *Video) (string, ProcessVideoOutput) {
	w.logger.WithFields(StructFields(video)).Info("Processing video")

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
	if outputExist && *w.poolWorker.config.DeleteOutputIfAlreadyExist {
		w.logger.Debug("Output already exist, deleting file")
		err = os.Remove(video.OutputPath)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	} else if outputExist {
		w.logger.Debug("Output already exist, skipping")
		return "", ProcessVideoOutput{outputFileAlreadyExist: true}
	}

	processFolderWorker := path.Join(w.poolWorker.config.ProcessFolder, fmt.Sprintf("worker_%d", w.id))
	if _, err := os.Stat(processFolderWorker); err == nil {
		err := os.RemoveAll(processFolderWorker)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	}

	w.logger.Info("Getting video fps and framecount")
	videoInfo, err := GetVideoInfo(w.poolWorker.ctx, video.Path)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	w.logger.Info("fps: ", videoInfo.Fps)
	w.logger.Info("target FPS: ", w.poolWorker.config.TargetFPS)
	w.logger.Info("framecount: ", videoInfo.FrameCount)

	if videoInfo.Fps >= w.poolWorker.config.TargetFPS {
		w.logger.Info(`Video is already higher or equal to target FPS, skipping`)
		return "", ProcessVideoOutput{skip: true}
	}

	targetFrameCount := int64(float64(videoInfo.FrameCount) / videoInfo.Fps * w.poolWorker.config.TargetFPS)
	w.logger.Info("Calculated frame target: ", targetFrameCount)

	w.logger.Debug("Creating worker folder if doesn't exist")
	err = os.MkdirAll(processFolderWorker, os.ModePerm)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	// Not used anymore but is needed for older rife models
	// so keeping it here
	// w.logger.Infof("Converting video to %g fps", p.con)
	// fpsConversionOutput := path.Join(processFolderWorker, "video.mp4")
	// output, err := ConvertVideoToFPS(p.ctx, p.config.FfmpegOptions, video.Path, fpsConversionOutput, targetFPS/2)
	// if err != nil {
	// 	return output, ProcessVideoOutput{err: err}
	// }

	w.logger.Info("Extracting audio from the video")
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err := ExtractAudio(w.poolWorker.ctx, video.Path, audioPath)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	w.logger.Debug("Creating frames folder")
	framesFolder := path.Join(processFolderWorker, "frames")
	err = os.Mkdir(framesFolder, os.ModePerm)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	w.logger.Info("Extracting video frames")
	output, err = ExtractFrames(w.poolWorker.ctx, w.poolWorker.config.FfmpegOptions, video.Path, framesFolder)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	w.logger.Debug("Creating interpolation frames folder")
	interpolatedFolder := path.Join(processFolderWorker, "interpolated_frames")
	err = os.Mkdir(interpolatedFolder, os.ModePerm)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	w.logger.Info("Interpolating video")
	output, err = InterpolateVideo(w.poolWorker.ctx, w.poolWorker.config.RifeBinary, framesFolder, interpolatedFolder,
		w.poolWorker.config.ModelPath, targetFrameCount, w.poolWorker.config.RifeExtraArguments)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	// make sure the folder exist
	baseOutputPath := path.Dir(video.OutputPath)
	w.logger.WithField("baseOutputPath", baseOutputPath).
		Debug("Creating output folder if it doesn't exist")
	if _, err := os.Stat(baseOutputPath); err != nil {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	}

	w.logger.Infof("Reconstructing video with audio and interpolated frames to %g fps", w.poolWorker.config.TargetFPS)
	output, err = ConstructVideoToFPS(w.poolWorker.ctx, w.poolWorker.config.FfmpegOptions, interpolatedFolder, audioPath, video.OutputPath, w.poolWorker.config.TargetFPS)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	w.logger.Debug("Removing worker folder")
	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	return "", ProcessVideoOutput{}
}
