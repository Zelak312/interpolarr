package main

import (
	"bufio"
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type VideoInfo struct {
	Fps        float64
	FrameCount int64
}

func appendHWAccelEncodeArgs(args []string, config FfmpegOptions) []string {
	if config.HWAccelEncodeFlag != "" {
		args = append(args, "-c:v", config.HWAccelEncodeFlag)
	}

	return args
}

func GetVideoInfo(ctx context.Context, inputPath string) (*VideoInfo, error) {
	cmd := NewCommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-count_frames",
		"-show_entries", "stream=r_frame_rate,nb_read_frames", "-of", "csv=p=0", inputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithField("inputPath", inputPath).Error("GetVideoInfo error: ", output)
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(output), ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected two parts in the output, got %d", len(parts))
	}

	// Parse the FPS using the parseFPS function.
	fps, err := parseFPS(parts[0])
	if err != nil {
		return nil, err
	}

	// Parse the frame count.
	frameCount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid frame count: %v", err)
	}

	return &VideoInfo{Fps: fps, FrameCount: frameCount}, nil
}

func ExtractAudio(ctx context.Context, inputPath string, outputPath string, progressChan chan<- float64) (string, error) {
	cmd := NewCommandContext(ctx, "ffmpeg", "-i", inputPath, "-vn", "-acodec", "copy", "-progress", "pipe:2", outputPath)
	go parseProgressFFmpeg(cmd, progressChan)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ExtractFrames(ctx context.Context, config FfmpegOptions, inputPath string, outputPath string, progressChan chan<- float64) (string, error) {
	outputPathTemplate := path.Join(outputPath, "frame_%08d.png")
	args := []string{}
	if config.HWAccelDecodeFlag != "" {
		args = append(args, "-c:v", config.HWAccelDecodeFlag)
	}

	args = append(args, "-i", inputPath, "-fps_mode", "passthrough", "-progress", "pipe:2", outputPathTemplate)
	cmd := NewCommandContext(ctx, "ffmpeg", args...)
	go parseProgressFFmpeg(cmd, progressChan)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ConstructVideoToFPS(ctx context.Context, config FfmpegOptions, inputPath string, audioPath string, outputPath string, fps float64, progressChan chan<- float64) (string, error) {
	inputPathTemplate := path.Join(inputPath, "%08d.png")
	args := []string{"-framerate", fmt.Sprintf("%g", fps), "-i", inputPathTemplate, "-i", audioPath, "-c:a", "copy"}
	args = appendHWAccelEncodeArgs(args, config)
	args = append(args, "-crf", "20", "-pix_fmt", "yuv420p", "-progress", "pipe:2", outputPath)
	cmd := NewCommandContext(ctx, "ffmpeg", args...)
	go parseProgressFFmpeg(cmd, progressChan)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func parseFPS(fpsFraction string) (float64, error) {
	parts := strings.Split(fpsFraction, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid FPS format")
	}

	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}

	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}

	return numerator / denominator, nil
}

// TODO: handle errors in here
func parseProgressFFmpeg(cmd *Command, progressChan chan<- float64) {
	var totalDuration float64
	// TODO: check if it should be better to compile the regex only once
	durationRegex := regexp.MustCompile(`Duration: (\d{2}):(\d{2}):(\d{2})\.(\d{2})`)

	scanner := bufio.NewScanner(cmd.stderrPipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Capture the total duration from ffmpeg's initial output
		if matches := durationRegex.FindStringSubmatch(line); matches != nil && totalDuration == 0 {
			hours, _ := strconv.ParseFloat(matches[1], 64)
			minutes, _ := strconv.ParseFloat(matches[2], 64)
			seconds, _ := strconv.ParseFloat(matches[3], 64)
			totalDuration = hours*3600 + minutes*60 + seconds
		}

		// Parse the out_time_ms line to calculate progress
		if strings.HasPrefix(line, "out_time_ms=") {
			outTimeMs, _ := strconv.ParseFloat(strings.Split(line, "=")[1], 64)
			progressSeconds := outTimeMs / 1000000.0 // Convert ms to seconds

			// Calculate progress percentage
			if totalDuration > 0 {
				progressChan <- (progressSeconds / totalDuration) * 100
			}
		}
	}
}
