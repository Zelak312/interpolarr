package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/sirupsen/logrus"
)

type Worker struct {
	// TODO: for context, always maybe add
	// a time constrained context to make sure
	// nothing is undefinitely running and blocking
	logger     *logrus.Entry
	poolWorker *PoolWorker
	hub        *Hub
	sync.RWMutex

	workerInfo WorkerInfo
}

type WorkerInfo struct {
	ID       int     `json:"id"`
	Active   bool    `json:"active"`
	Step     string  `json:"step"`
	Progress float64 `json:"progress"`
	Video    *Video  `json:"video"`
}

func NewWorker(id int, logger *logrus.Entry, poolWoker *PoolWorker, hub *Hub) *Worker {
	return &Worker{
		logger:     logger,
		poolWorker: poolWoker,
		hub:        hub,
	}
}

func ShouldUseTempFile(video *Video, deleteOutputIfAlreadyExist bool) (bool, error) {
	samePath, err := IsSamePath(video.Path, video.OutputPath)
	if err != nil {
		return false, err
	}

	if samePath {
		return true, nil
	}

	outputExist, err := PathExist(video.OutputPath)
	if err != nil {
		return false, err
	}

	if outputExist && deleteOutputIfAlreadyExist {
		return true, nil
	}

	return false, nil
}

func (w *Worker) start() {
	for video := range w.poolWorker.workChannel {
		w.Lock()
		w.workerInfo.Active = true
		w.poolWorker.waitGroup.Add(1)
		w.workerInfo.Video = &video
		w.Unlock()
		err := w.doWork(&video)
		w.Lock()
		w.workerInfo.Video = nil
		w.Unlock()
		if w.poolWorker.ctx.Err() != nil {
			w.logger.Debug("Ctx error is: ", w.poolWorker.ctx.Err())
			if w.poolWorker.ctx.Err() == context.Canceled {
				w.logger.Debug("Ctx was canceled")

				// End function so call return
				w.Lock()
				w.workerInfo.Active = false
				w.workerInfo.Video = nil
				w.poolWorker.waitGroup.Done()
				w.Unlock()
				return
			}
		}

		if err != nil {
			w.logger.Warn(err)
			// TODO: make a place where I can store warnings
			// So I can store warning for each videos
			// Because the otherwise the issues from runWorker (that doesn't retry)
			// Won't show anywhere
		}

		w.Lock()
		w.poolWorker.waitGroup.Done()
		w.workerInfo.Active = false
		w.Unlock()
		w.sendUpdate()
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

	videoExist, err := PathExist(video.Path)
	if err != nil {
		return "", ProcessVideoOutput{}
	}

	if !videoExist {
		return "", ProcessVideoOutput{videoNotFound: true}
	}

	baseOutputPath := path.Dir(video.OutputPath)
	w.logger.WithField("baseOutputPath", baseOutputPath).
		Debug("Creating output folder if it doesn't exist")
	outputBasePathExist, err := PathExist(video.OutputPath)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	useTmpFile := false
	outputPath := video.OutputPath
	if !outputBasePathExist {
		err := os.MkdirAll(baseOutputPath, os.ModePerm)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	} else {
		useTmpFile, err = ShouldUseTempFile(video, *w.poolWorker.config.DeleteOutputIfAlreadyExist)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	}

	if useTmpFile {
		w.logger.Debug("Using tmp file")
		outputPath = outputPath + ".tmp"

    log.Debugf("checking tmp path: %s", outputPath)
		videoTmpExist, err := PathExist(outputPath)
		if err != nil {
			return "", ProcessVideoOutput{}
		}

		if videoTmpExist {
			log.Warn("Tmp video file output already exist, skipping")
			return "", ProcessVideoOutput{skip: true}
		}
	}

	processFolderWorker := path.Join(w.poolWorker.config.ProcessFolder, fmt.Sprintf("worker_%d", w.workerInfo.ID))
	if _, err := os.Stat(processFolderWorker); err == nil {
		err := os.RemoveAll(processFolderWorker)
		if err != nil {
			return "", ProcessVideoOutput{err: err}
		}
	}

	progressChan := make(chan float64)
	defer close(progressChan)
	go w.updateProgress(progressChan)

	w.logger.Info("Getting video fps and framecount")
	w.updateStep("Getting FPS Framecount")
	videoInfo, output, err := GetVideoInfo(w.poolWorker.ctx, video.Path)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
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
	w.updateStep("Extracting audio")
	audioPath := path.Join(processFolderWorker, "audio.m4a")
	output, err = ExtractAudio(w.poolWorker.ctx, video.Path, audioPath, progressChan)
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
	w.updateStep("Extracting frames")
	output, err = ExtractFrames(w.poolWorker.ctx, w.poolWorker.config.FfmpegOptions, video.Path, framesFolder, progressChan)
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
	w.updateStep("Interpolating frames")
	output, err = InterpolateVideo(w.poolWorker.ctx, w.poolWorker.config.RifeBinary, framesFolder, interpolatedFolder,
		w.poolWorker.config.ModelPath, targetFrameCount, w.poolWorker.config.RifeExtraArguments, progressChan)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	w.logger.Infof("Reconstructing video with audio and interpolated frames to %g fps", w.poolWorker.config.TargetFPS)
	w.updateStep("Reconstructing video")
	output, err = ConstructVideoToFPS(w.poolWorker.ctx, w.poolWorker.config.FfmpegOptions,
		interpolatedFolder, audioPath, outputPath, w.poolWorker.config.TargetFPS, progressChan)
	if err != nil {
		return output, ProcessVideoOutput{err: err}
	}

	if useTmpFile {
		w.logger.Debug("Moving tmp file to output path since everything was succesful")
		RenameOverwrite(outputPath, video.OutputPath)
	}

	w.logger.Debug("Removing worker folder")
	err = os.RemoveAll(processFolderWorker)
	if err != nil {
		return "", ProcessVideoOutput{err: err}
	}

	return "", ProcessVideoOutput{}
}

func (w *Worker) updateStep(step string) {
	w.Lock()
	w.workerInfo.Step = step
	w.workerInfo.Progress = 0

	w.Unlock()
	w.sendUpdate()
}

func (w *Worker) updateProgress(progressChan <-chan float64) {
	for progress := range progressChan {
		w.Lock()
		w.workerInfo.Progress = progress

		w.Unlock()
		w.sendUpdate()
	}
}

func (w *Worker) sendUpdate() {
	w.Lock()
	defer w.Unlock()

	packet := WsWorkerProgress{
		WsBaseMessage: WsBaseMessage{
			Type: "worker_progress",
		},
		WorkerInfo: w.workerInfo,
	}

	w.hub.BroadcastMessage(packet)
}

func (w *Worker) GetInfo() WorkerInfo {
	w.RLock() // Shared lock for reading
	defer w.RUnlock()

	return w.workerInfo
}
