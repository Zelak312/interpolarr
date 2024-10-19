package main

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func InterpolateVideo(ctx context.Context, binaryPath string, inputPath string, outputPath string,
	model string, frameCount int64, extraArgs string, progressChan chan<- float64) (string, error) {

	cmd := NewCommandContext(ctx, binaryPath, "-i", inputPath, "-o", outputPath, "-m", model, "-n", fmt.Sprint(frameCount), "-v", extraArgs)
	go parseProgressRife(cmd, frameCount, progressChan)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// TODO: handle errors in here
func parseProgressRife(cmd *Command, frameCount int64, progressChan chan<- float64) {
	// TODO: check if it should be better to compile the regex only once
	frameRegex := regexp.MustCompile(`-> .+/0{0,}(\d+)\.`)

	scanner := bufio.NewScanner(cmd.stderrPipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if matches := frameRegex.FindStringSubmatch(line); matches != nil {
			frameNumberStr := matches[1]
			frameNumber, err := strconv.Atoi(frameNumberStr)
			if err != nil {
				continue
			}

			progressChan <- (float64(frameNumber) / float64(frameCount)) * 100
		}
	}
}
