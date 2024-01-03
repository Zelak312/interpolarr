package main

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

func GetVideoFPS(inputPath string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=r_frame_rate", "-of", "default=noprint_wrappers=1:nokey=1", inputPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	fpsDivision := strings.TrimSpace(string(output))
	fps, err := parseFPS(fpsDivision)
	if err != nil {
		return 0, err
	}

	return fps, nil
}

func ConvertVideoTo30FPS(ctx context.Context, inputPath string, outputPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", inputPath, "-filter:v", "fps=30", outputPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ExtractAudio(ctx context.Context, inputPath string, outputPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", inputPath, "-vn", "-acodec", "copy", outputPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ExtractFrames(ctx context.Context, inputPath string, outputPath string) (string, error) {
	outputPathTemplate := path.Join(outputPath, "frame_%08d.png")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", inputPath, outputPathTemplate)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func ConstructVideoTo60FPS(ctx context.Context, inputPath string, audioPath string, outputPath string) (string, error) {
	inputPathTemplate := path.Join(inputPath, "frame_%08d.png")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-framerate", "60", "-i", inputPathTemplate, "-i", audioPath, "-c:a", "copy",
		"-crf", "20", "-c:v", "h264_nvenc", "-pix_fmt", "yuv420p", outputPath)
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
