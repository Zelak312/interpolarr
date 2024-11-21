package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Zelak312/rife-ncnn-vulkan-go"
)

// New iteration

type VideoProcessor struct {
	inputPath string
	width     int
	height    int
	frameRate int
}

type FFProbeOutput struct {
	Streams []struct {
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		FrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
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

	return &VideoProcessor{
		inputPath: inputPath,
		width:     stream.Width,
		height:    stream.Height,
		frameRate: frameRate,
	}, nil
}

func (vp *VideoProcessor) Process(outputPath string) error {

	config := rife.DefaultConfig(1280, 720)

	// Create RIFE instance
	r, err := rife.New(config)
	if err != nil {
		return fmt.Errorf("failed to create RIFE: %v", err)
	}
	defer r.Close()

	// Load model
	err = r.LoadModel("/home/zelak/space/rife/rife-v4.24")
	if err != nil {
		return fmt.Errorf("failed to load model: %v", err)
	}

	reader := exec.Command("ffmpeg",
		"-hwaccel", "cuda",
		"-i", vp.inputPath,
		"-f", "rawvideo",
		"-pix_fmt", "bgr24",
		"pipe:1")

	stdin, err := reader.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %v", err)
	}

	if err := reader.Start(); err != nil {
		return fmt.Errorf("starting reader: %v", err)
	}

	writer := exec.Command("ffmpeg",
		"-f", "rawvideo",
		"-pix_fmt", "bgr24",
		"-video_size", fmt.Sprintf("%dx%d", vp.width, vp.height),
		"-framerate", fmt.Sprintf("%d", vp.frameRate*2),
		"-i", "pipe:0",
		"-c:v", "h264_nvenc",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		outputPath)

	// writer.Stderr = os.Stderr // Add this line
	// writer.Stdout = os.Stdout // Optional, for full output

	stdout, err := writer.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %v", err)
	}

	if err := writer.Start(); err != nil {
		return fmt.Errorf("starting writer: %v", err)
	}

	frameSize := vp.width * vp.height * 3
	buf1 := make([]byte, frameSize)
	buf2 := make([]byte, frameSize)
	outBuf := make([]byte, frameSize)

	beforeStart := time.Now()
	var secondFrame []byte
	firstIteration := true

	for {
		// Read first frame (or reuse second frame from previous iteration)
		if secondFrame != nil {
			copy(buf1, secondFrame)
		} else if !firstIteration {
			break
		} else {
			_, err := io.ReadFull(stdin, buf1)
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("reading first frame: %v", err)
			}
		}

		elapsed := time.Now().Sub(beforeStart)
		fmt.Printf("Took getting first frame %s\n", elapsed)

		start := time.Now()
		_, err := io.ReadFull(stdin, buf2)
		if err == io.EOF {
			if !firstIteration {
				// Write the last frame only if we haven't written it yet
				if _, err := stdout.Write(buf1); err != nil {
					return fmt.Errorf("writing final frame: %v", err)
				}
			}
			break
		}
		if err != nil {
			return fmt.Errorf("reading second frame: %v", err)
		}

		elapsed = time.Now().Sub(start)
		fmt.Printf("Took getting second frame %s\n", elapsed)
		// porcess frame
		start = time.Now()
		interpolated, err := r.InterpolateBGR(buf1, buf2, 0.5)
		if err != nil {
			return fmt.Errorf("interpolating frames: %v", err)
		}
		copy(outBuf, interpolated)

		elapsed = time.Now().Sub(start)
		fmt.Printf("Took processing frames %s\n", elapsed)

		// Write frames in sequence
		start = time.Now()
		if firstIteration {
			if _, err := stdout.Write(buf1); err != nil {
				return fmt.Errorf("writing first frame: %v", err)
			}
		}
		if _, err := stdout.Write(outBuf); err != nil {
			return fmt.Errorf("writing interpolated frame: %v", err)
		}
		if _, err := stdout.Write(buf2); err != nil {
			return fmt.Errorf("writing second frame: %v", err)
		}

		elapsed = time.Now().Sub(start)
		fmt.Printf("Took writing frames %s\n", elapsed)

		// Save second frame for next iteration
		secondFrame = make([]byte, frameSize)
		copy(secondFrame, buf2)
		firstIteration = false

		beforeStart = time.Now()
		// }
	}

	stdout.Close()
	stdin.Close()

	if err := writer.Wait(); err != nil {
		return fmt.Errorf("finalizing video: %v", err)
	}

	return nil
}

// End of new iteration

var durationRegex = regexp.MustCompile(`Duration: (\d{2}):(\d{2}):(\d{2})\.(\d{2})`)

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

func GetVideoInfo(ctx context.Context, inputPath string) (*VideoInfo, string, error) {
	cmd := NewCommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-count_frames",
		"-show_entries", "stream=r_frame_rate,nb_read_frames", "-of", "csv=p=0", inputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, output, err
	}

	parts := strings.Split(strings.TrimSpace(output), ",")
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("expected two parts in the output, got %d", len(parts))
	}

	// Parse the FPS using the parseFPS function.
	fps, err := parseFPS(parts[0])
	if err != nil {
		return nil, "", err
	}

	// Parse the frame count.
	frameCount, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, "", fmt.Errorf("invalid frame count: %v", err)
	}

	return &VideoInfo{Fps: fps, FrameCount: frameCount}, "", nil
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
