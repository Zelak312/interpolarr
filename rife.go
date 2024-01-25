package main

import (
	"context"
)

func InterpolateVideo(ctx context.Context, binaryPath string, inputPath string, outputPath string, model string) (string, error) {
	cmd := CommandContextLogger(ctx, binaryPath, "-i", inputPath, "-o", outputPath, "-m", model)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
