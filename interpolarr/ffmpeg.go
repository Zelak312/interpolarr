package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

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
	options   FFmpegOptions

	shouldReadFrame        int32
	currentFrameBuffer     []byte
	readFrameBufferChannel chan Frame
	readFrameError         error

	shouldWriteFrame        int32
	writeFrameBufferChannel chan Frame
	writeFrameError         error

	// I/O handlers
	reader *Command
	writer *Command
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

func parseVideoInfoFFProbeOutput(output string) (*FFProbeOutput, error) {
	var probeOutput FFProbeOutput
	if err := json.Unmarshal([]byte(output), &probeOutput); err != nil {
		return nil, fmt.Errorf("parsing probe output: %v\n%v", err, output)
	}

	if len(probeOutput.Streams) == 0 {
		return nil, fmt.Errorf("no video streams found")
	}

	return &probeOutput, nil
}

func GetVideoInfo(ctx context.Context, inputPath string) (*VideoInfo, string, error) {
	cmd := NewCommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate,nb_frames",
		"-of", "json",
		inputPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, output, err
	}

	ffprobeOutput, err := parseVideoInfoFFProbeOutput(output)
	if err != nil {
		return nil, output, err
	}

	mainStream := ffprobeOutput.Streams[0]
	parts := strings.Split(mainStream.FrameRate, "/")
	if len(parts) != 2 {
		return nil, output, fmt.Errorf("invalid framerate format")
	}

	num, err := strconv.ParseFloat(parts[0], 32)
	if err != nil {
		return nil, output, fmt.Errorf("parsing framerate numerator: %v", err)
	}

	den, err := strconv.ParseFloat(parts[1], 32)
	if err != nil {
		return nil, output, fmt.Errorf("parsing framerate denominator: %v", err)
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
			return nil, output, err
		}

		videoInfo.FrameCount = frameCount
		return &videoInfo, "", nil
	}

	// container doesn't have frame count, counting frames
	cmd = NewCommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames",
		"-of", "json",
		inputPath)

	output, err = cmd.CombinedOutput()
	if err != nil {
		return nil, output, err
	}

	ffprobeCountOutput, err := parseVideoInfoFFProbeOutput(output)
	if err != nil {
		return nil, output, err
	}

	frameCount, err := strconv.ParseInt(ffprobeCountOutput.Streams[0].FrameCountRead, 10, 64)
	if err != nil {
		return nil, output, err
	}

	videoInfo.FrameCount = frameCount
	return &videoInfo, output, nil
}

func NewVideoProcessor(frameBufferChannelLengths int, videoInfo *VideoInfo, options FFmpegOptions) (*VideoProcessor, error) {
	frameSize := videoInfo.Width * videoInfo.Height * 3

	return &VideoProcessor{
		videoInfo:               *videoInfo,
		options:                 options,
		currentFrameBuffer:      make([]byte, frameSize),
		readFrameBufferChannel:  make(chan Frame, frameBufferChannelLengths),
		writeFrameBufferChannel: make(chan Frame, frameBufferChannelLengths),
	}, nil
}

func (vp *VideoProcessor) StartReading(ctx context.Context) error {
	args := []string{}
	if vp.options.HWAccelDecodeFlag != "" {
		args = append(args, "-hwaccel", vp.options.HWAccelDecodeFlag)
	}

	args = append(args, "-i", vp.videoInfo.InputPath,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"pipe:1")

	vp.reader = NewCommandContext(ctx, "ffmpeg", args...)

	vp.reader.DisableOutputBuffer()
	stdout, err := vp.reader.GetStdout()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %v", err)
	}

	vp.stdout = stdout

	// Start reading frame in goroutine to fill up
	// The read frame buffer channel
	// TODO: use ctx to close
	atomic.StoreInt32(&vp.shouldReadFrame, 1)
	go func() {
		defer close(vp.readFrameBufferChannel)
		for atomic.LoadInt32(&vp.shouldReadFrame) == 1 {
			err := vp.readFrameInternal()
			if err != nil {
				atomic.StoreInt32(&vp.shouldReadFrame, 0)
				vp.readFrameError = err
				break
			}
		}
	}()

	return vp.reader.Start()
}

func (vp *VideoProcessor) StartWriting(ctx context.Context, outputPath string, outputFrameRate float64) error {
	args := []string{
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-video_size", fmt.Sprintf("%dx%d", vp.videoInfo.Width, vp.videoInfo.Height),
		"-framerate", fmt.Sprintf("%f", outputFrameRate),
		"-i", "pipe:0",
		"-i", vp.videoInfo.InputPath,
	}
	if vp.options.HWAccelDecodeFlag != "" {
		args = append(args, "-c:v", vp.options.HWAccelEncodeFlag)
	}

	args = append(args, "-c:v", "h264_nvenc",
		"-c:a", "copy",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		outputPath)

	vp.writer = NewCommandContext(ctx, "ffmpeg", args...)

	stdin, err := vp.writer.GetStdin()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %v", err)
	}

	vp.stdin = stdin

	// Start writing frame in goroutine
	// TODO: use ctx to close
	atomic.StoreInt32(&vp.shouldWriteFrame, 1)
	go func() {
		defer close(vp.writeFrameBufferChannel)
		for atomic.LoadInt32(&vp.shouldWriteFrame) == 1 {
			frame := <-vp.writeFrameBufferChannel
			err := vp.writeFrameInternal(frame)
			if err != nil {
				atomic.StoreInt32(&vp.shouldWriteFrame, 0)
				vp.writeFrameError = err
				break
			}
		}
	}()

	return vp.writer.Start()
}

func (vp *VideoProcessor) readFrameInternal() error {
	_, err := io.ReadFull(vp.stdout, vp.currentFrameBuffer)
	if err != nil {
		return err
	}

	vp.readFrameBufferChannel <- Frame{
		Data:   slices.Clone(vp.currentFrameBuffer),
		Width:  vp.videoInfo.Width,
		Height: vp.videoInfo.Height,
	}

	return nil
}

func (vp *VideoProcessor) ReadFrame() (Frame, error) {
	frame, ok := <-vp.readFrameBufferChannel
	if !ok {
		return Frame{}, vp.readFrameError
	}

	return frame, nil
}

func (vp *VideoProcessor) writeFrameInternal(frame Frame) error {
	_, err := vp.stdin.Write(frame.Data)
	return err
}

func (vp *VideoProcessor) WriteFrame(frame Frame) error {
	if atomic.LoadInt32(&vp.shouldWriteFrame) == 0 {
		return vp.writeFrameError
	}

	vp.writeFrameBufferChannel <- frame
	return nil
}

func (vp *VideoProcessor) Close() error {
	var errors []error

	// Stop reading loop
	// Will also close channel
	atomic.StoreInt32(&vp.shouldReadFrame, 0)
	atomic.StoreInt32(&vp.shouldWriteFrame, 0)

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
