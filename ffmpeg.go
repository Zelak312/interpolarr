package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// New iteration

type FFProbeOutput struct {
	Streams []struct {
		Width          int    `json:"width"`
		Height         int    `json:"height"`
		FrameRate      string `json:"r_frame_rate"`
		FrameCount     string `json:"nb_frames"`
		FrameCountRead string `json:"nb_read_frames"`
	} `json:"streams"`
}

type Frame struct {
	Data   []byte
	Width  int
	Height int
}

type VideoProcessor struct {
	videoInfo VideoInfo
	frameSize int

	// I/O handlers
	reader *exec.Cmd
	writer *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

type VideoInfo struct {
	InputPath  string
	Width      int
	Height     int
	FrameRate  float64
	FrameCount int64
}

func parseVideoInfoFFProbeOutput(output []byte) (*FFProbeOutput, error) {
	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("parsing probe output: %v", err)
	}

	if len(probeOutput.Streams) == 0 {
		return nil, fmt.Errorf("no video streams found")
	}

	return &probeOutput, nil
}

func GetVideoInfo(inputPath string) (*VideoInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate,nb_frames",
		"-of", "json",
		inputPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	ffprobeOutput, err := parseVideoInfoFFProbeOutput(output)
	if err != nil {
		return nil, err
	}

	mainStream := ffprobeOutput.Streams[0]
	parts := strings.Split(mainStream.FrameRate, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid framerate format")
	}

	num, err := strconv.ParseFloat(parts[0], 32)
	if err != nil {
		return nil, fmt.Errorf("parsing framerate numerator: %v", err)
	}

	den, err := strconv.ParseFloat(parts[1], 32)
	if err != nil {
		return nil, fmt.Errorf("parsing framerate denominator: %v", err)
	}

	var videoInfo VideoInfo
	videoInfo.InputPath = inputPath
	videoInfo.Width = mainStream.Width
	videoInfo.Height = mainStream.Height
	videoInfo.FrameRate = num / den

	if mainStream.FrameCount != "" && mainStream.FrameCount != "N/A" {
		// container already contains frame count, no need to count
		frameCount, err := strconv.ParseInt(mainStream.FrameCount, 10, 64)
		if err != nil {
			return nil, err
		}

		videoInfo.FrameCount = frameCount
		return &videoInfo, nil
	}
	// container doesn't have frame count, counting frames

	cmd = exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames",
		"-of", "json",
		inputPath)

	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	ffprobeCountOutput, err := parseVideoInfoFFProbeOutput(output)
	if err != nil {
		return nil, err
	}

	frameCount, err := strconv.ParseInt(ffprobeCountOutput.Streams[0].FrameCountRead, 10, 64)
	if err != nil {
		return nil, err
	}

	videoInfo.FrameCount = frameCount
	return &videoInfo, nil
}

func NewVideoProcessor(videoInfo *VideoInfo) (*VideoProcessor, error) {
	frameSize := videoInfo.Width * videoInfo.Height * 3

	return &VideoProcessor{
		videoInfo: *videoInfo,
		frameSize: frameSize,
	}, nil
}

func (vp *VideoProcessor) StartReading() error {
	vp.reader = exec.Command("ffmpeg",
		"-hwaccel", "cuda",
		"-i", vp.videoInfo.InputPath,
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

func (vp *VideoProcessor) StartWriting(outputPath string, outputFrameRate float64) error {
	vp.writer = exec.Command("ffmpeg",
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-video_size", fmt.Sprintf("%dx%d", vp.videoInfo.Width, vp.videoInfo.Height),
		"-framerate", fmt.Sprintf("%f", outputFrameRate),
		"-i", "pipe:0",
		"-i", vp.videoInfo.InputPath,
		"-c:v", "h264_nvenc",
		"-c:a", "copy",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		outputPath)

	stdin, err := vp.writer.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %v", err)
	}
	vp.stdin = stdin

	vp.writer.Stdout = os.Stdout
	vp.writer.Stderr = os.Stderr

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
		Width:  vp.videoInfo.Width,
		Height: vp.videoInfo.Height,
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
func (vp *VideoProcessor) Width() int         { return vp.videoInfo.Width }
func (vp *VideoProcessor) Height() int        { return vp.videoInfo.Height }
func (vp *VideoProcessor) FrameRate() float64 { return vp.videoInfo.FrameRate }
func (vp *VideoProcessor) FrameSize() int     { return vp.frameSize }

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
