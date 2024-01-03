package main

import (
	"context"
	"os/exec"
	"path"
)

func InterpolateVideo(ctx context.Context, binaryPath string, inputPath string, outputPath string, model string) (string, error) {
	modelPath := path.Join(binaryPath, "../", model)
	cmd := exec.CommandContext(ctx, binaryPath, "-i", inputPath, "-o", outputPath, "-m", modelPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
