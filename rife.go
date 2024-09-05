package main

import (
	"context"
	"fmt"
)

func InterpolateVideo(ctx context.Context, binaryPath string, inputPath string, outputPath string,
	model string, frameCount int64, extraArgs string) (string, error) {

	cmd := NewCommandContext(ctx, binaryPath, "-i", inputPath, "-o", outputPath, "-m", model, "-n", fmt.Sprint(frameCount), extraArgs)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
