package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// New iteration

type FFProbeOutput struct {
	Streams []struct {
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		FrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
}

type Frame struct {
	Data   []byte
	Width  int
	Height int
}

type VideoProcessor struct {
	inputPath string
	width     int
	height    int
	frameRate int
	frameSize int

	// I/O handlers
	reader *exec.Cmd
	writer *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func NewVideoProcessor(inputPath string) (*VideoProcessor, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate",
		"-of", "json",
		inputPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe error: %v", err)
	}

	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("parsing probe output: %v", err)
	}

	if len(probeOutput.Streams) == 0 {
		return nil, fmt.Errorf("no video streams found")
	}

	stream := probeOutput.Streams[0]

	parts := strings.Split(stream.FrameRate, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid framerate format")
	}

	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("parsing framerate numerator: %v", err)
	}

	den, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parsing framerate denominator: %v", err)
	}

	frameRate := num / den
	frameSize := stream.Width * stream.Height * 3

	return &VideoProcessor{
		inputPath: inputPath,
		width:     stream.Width,
		height:    stream.Height,
		frameRate: frameRate,
		frameSize: frameSize,
	}, nil
}

func (vp *VideoProcessor) StartReading() error {
	vp.reader = exec.Command("ffmpeg",
		"-hwaccel", "cuda",
		"-i", vp.inputPath,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"pipe:1")

	stdout, err := vp.reader.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %v", err)
	}
	vp.stdout = stdout

	return vp.reader.Start()
}

func (vp *VideoProcessor) StartWriting(outputPath string, outputFrameRate int) error {
	vp.writer = exec.Command("ffmpeg",
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-video_size", fmt.Sprintf("%dx%d", vp.width, vp.height),
		"-framerate", fmt.Sprintf("%d", outputFrameRate),
		"-i", "pipe:0",
		"-c:v", "h264_nvenc",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		outputPath)

	stdin, err := vp.writer.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %v", err)
	}
	vp.stdin = stdin

	return vp.writer.Start()
}

func (vp *VideoProcessor) ReadFrame() (Frame, error) {
	buf := make([]byte, vp.frameSize)
	_, err := io.ReadFull(vp.stdout, buf)
	if err != nil {
		return Frame{}, err
	}

	return Frame{
		Data:   buf,
		Width:  vp.width,
		Height: vp.height,
	}, nil
}

func (vp *VideoProcessor) WriteFrame(frame Frame) error {
	_, err := vp.stdin.Write(frame.Data)
	return err
}

func (vp *VideoProcessor) Close() error {
	var errors []error

	if vp.stdin != nil {
		if err := vp.stdin.Close(); err != nil {
			errors = append(errors, fmt.Errorf("closing stdin: %v", err))
		}
	}

	if vp.stdout != nil {
		if err := vp.stdout.Close(); err != nil {
			errors = append(errors, fmt.Errorf("closing stdout: %v", err))
		}
	}

	if vp.writer != nil {
		if err := vp.writer.Wait(); err != nil {
			errors = append(errors, fmt.Errorf("waiting for writer: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors during close: %v", errors)
	}
	return nil
}

// Getters for video properties
func (vp *VideoProcessor) Width() int     { return vp.width }
func (vp *VideoProcessor) Height() int    { return vp.height }
func (vp *VideoProcessor) FrameRate() int { return vp.frameRate }
func (vp *VideoProcessor) FrameSize() int { return vp.frameSize }

var durationRegex = regexp.MustCompile(`Duration: (\d{2}):(\d{2}):(\d{2})\.(\d{2})`)

func ExtractAudio(ctx context.Context, inputPath string, outputPath string, progressChan chan<- float64) (string, error) {
	cmd := NewCommandContext(ctx, "ffmpeg", "-i", inputPath, "-vn", "-acodec", "copy", "-progress", "pipe:2", outputPath)
	go parseProgressFFmpeg(cmd, progressChan)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// TODO: handle errors in here
func parseProgressFFmpeg(cmd *Command, progressChan chan<- float64) {
	var totalDuration float64
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
