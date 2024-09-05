package main

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
)

type VideoInfo struct {
	Fps        float64
	FrameCount int64
}

// func appendVideoCodecArgs(args []string, config FfmpegOptions) []string {
// 	if config.VideoCodec != "" {
// 		args = append(args, "-c:v", config.VideoCodec)
// 	}

// 	return args
// }

// func appendHWAccelArgs(args []string, config FfmpegOptions) []string {
// 	if config.HWAccel != "" {
// 		args = append(args, "-hwaccel", config.HWAccel)
// 	}

// 	if config.HWAccelDecodeFlag != "" {
// 		args = append(args, "-c:v", config.HWAccelDecodeFlag)
// 	}

// 	return args
// }

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

// func ConvertVideoToFPS(ctx context.Context, config FfmpegOptions, inputPath string, outputPath string, fps float64) (string, error) {
// 	args := []string{}
// 	args = appendHWAccelArgs(args, config)
// 	args = append(args, "-i", inputPath, "-filter:v", fmt.Sprintf("fps=%g", fps))
// 	args = appendVideoCodecArgs(args, config)
// 	args = append(args, outputPath)
// 	cmd := CommandContextLogger(ctx, "ffmpeg", args...)
// 	output, err := cmd.CombinedOutput()
// 	return string(output), err
// }

func ExtractAudio(ctx context.Context, inputPath string, outputPath string) (string, error) {
	cmd := NewCommandContext(ctx, "ffmpeg", "-i", inputPath, "-vn", "-acodec", "copy", outputPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ExtractFrames(ctx context.Context, config FfmpegOptions, inputPath string, outputPath string) (string, error) {
	outputPathTemplate := path.Join(outputPath, "frame_%08d.png")
	args := []string{}
	if config.HWAccelDecodeFlag != "" {
		args = append(args, "-c:v", config.HWAccelDecodeFlag)
	}

	args = append(args, "-i", inputPath, "-fps_mode", "passthrough", outputPathTemplate)
	cmd := NewCommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ConstructVideoToFPS(ctx context.Context, config FfmpegOptions, inputPath string, audioPath string, outputPath string, fps float64) (string, error) {
	inputPathTemplate := path.Join(inputPath, "%08d.png")
	args := []string{"-framerate", fmt.Sprintf("%g", fps), "-i", inputPathTemplate, "-i", audioPath, "-c:a", "copy"}
	args = appendHWAccelEncodeArgs(args, config)
	args = append(args, "-crf", "20", "-pix_fmt", "yuv420p", outputPath)
	cmd := NewCommandContext(ctx, "ffmpeg", args...)
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
