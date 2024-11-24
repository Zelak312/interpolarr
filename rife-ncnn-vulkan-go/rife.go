package rife

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"unsafe"
)

// #include <stdlib.h>
// #include <string.h>
// #cgo CFLAGS: -I${SRCDIR}/rife-wrapper
// #cgo linux,amd64 LDFLAGS: -L${SRCDIR}/rife-wrapper/build -L${SRCDIR}/rife-wrapper/build/rife-ncnn-vulkan/src/ncnn/src -L${SRCDIR}/rife-wrapper/build/rife-ncnn-vulkan/src/ncnn/glslang/glslang -L${SRCDIR}/rife-wrapper/build/rife-ncnn-vulkan/src/ncnn/glslang/SPIRV -L${SRCDIR}/rife-wrapper/build/rife-ncnn-vulkan/src/ncnn/glslang/glslang/OSDependent/Unix -L${SRCDIR}/rife-wrapper/build/rife-ncnn-vulkan/src/ncnn/glslang/OGLCompilersDLL -l:librife_ncnn_vulkan_wrapper.a -l:libncnn.a -l:libglslang.a -l:libSPIRV.a -l:libMachineIndependent.a -l:libOSDependent.a -l:libOGLCompiler.a -l:libGenericCodeGen.a -lvulkan -fopenmp -static-libgcc -static-libstdc++ -lstdc++ -lm -ldl -lpthread
// #cgo windows LDFLAGS: -L${SRCDIR}/rife-wrapper/build -lrife_ncnn_vulkan_wrapper -lvulkan-1 -lstdc++ -lm
// #cgo darwin LDFLAGS: -L${SRCDIR}/rife-wrapper/build -l:librife_ncnn_vulkan_wrapper.a -framework Vulkan -lstdc++ -lm
// #include "rife_c_wrapper.h"
import "C"

// Rife represents a RIFE context for frame interpolation
type Rife struct {
	ctx      *C.Rife_Ctx
	width    int
	height   int
	channels int
}

// Config holds the configuration options for RIFE
type Config struct {
	GPUID       int
	Width       int
	Height      int
	TTAMode     bool
	TTATemporal bool
	UHDMode     bool
	NumThreads  int
	RIFEv2      bool
	RIFEv4      bool
	Padding     int
}

// DefaultConfig returns a default configuration for RIFE
func DefaultConfig(width, height int) *Config {
	return &Config{
		GPUID:       0,
		Width:       width,
		Height:      height,
		TTAMode:     false,
		TTATemporal: false,
		UHDMode:     false,
		NumThreads:  1,
		RIFEv2:      false,
		RIFEv4:      true,
		Padding:     64,
	}
}

// New creates a new RIFE instance with the given configuration
func New(config *Config) (*Rife, error) {
	ctx := C.rife_create(
		C.int(config.GPUID),
		btoi(config.TTAMode),
		btoi(config.TTATemporal),
		btoi(config.UHDMode),
		C.int(config.NumThreads),
		btoi(config.RIFEv2),
		btoi(config.RIFEv4),
		C.int(config.Padding),
	)

	if ctx == nil {
		return nil, fmt.Errorf("failed to create RIFE context")
	}

	return &Rife{
		ctx:      ctx,
		width:    config.Width,
		height:   config.Height,
		channels: 3,
	}, nil
}

// LoadModel loads the RIFE model from the specified directory
func (r *Rife) LoadModel(modelDir string) error {
	absPath, err := filepath.Abs(modelDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	if _, err = os.Stat(modelDir); err != nil {
		return err
	}

	cModelDir := C.CString(absPath)
	defer C.free(unsafe.Pointer(cModelDir))

	if result := C.rife_load(r.ctx, cModelDir); result != 0 {
		return fmt.Errorf("failed to load model from %s, error code: %d", absPath, result)
	}

	return nil
}

// Close releases resources associated with the RIFE context
func (r *Rife) Close() {
	if r.ctx != nil {
		C.rife_destroy(r.ctx)
		r.ctx = nil
	}
}

// InterpolateBGR generates an interpolated frame between two BGR buffers
func (r *Rife) InterpolateBGR(bgr1, bgr2 []byte, timestep float32) ([]byte, error) {
	if timestep == 0.0 {
		outBuf := make([]byte, len(bgr1))
		copy(outBuf, bgr1)
		return outBuf, nil
	}
	if timestep == 1.0 {
		outBuf := make([]byte, len(bgr2))
		copy(outBuf, bgr2)
		return outBuf, nil
	}

	// Verify buffer sizes
	expectedSize := r.width * r.height * 3
	if len(bgr1) != expectedSize || len(bgr2) != expectedSize {
		return nil, fmt.Errorf("invalid buffer size: expected %d, got %d and %d", expectedSize, len(bgr1), len(bgr2))
	}

	// Allocate output buffer
	outBuf := make([]byte, expectedSize)

	// Process frames
	if C.rife_process_frames(r.ctx,
		(*C.uchar)(&bgr1[0]),
		(*C.uchar)(&bgr2[0]),
		C.int(r.width),
		C.int(r.height),
		C.int(3), // elempack
		(*C.uchar)(&outBuf[0]),
		C.float(timestep)) != 0 {
		return nil, fmt.Errorf("failed to process frames")
	}

	return outBuf, nil
}

// Interpolate generates an interpolated frame between two image.Image
func (r *Rife) Interpolate(img1, img2 image.Image, timestep float32) (image.Image, error) {
	if timestep == 0.0 {
		return img1, nil
	}
	if timestep == 1.0 {
		return img2, nil
	}

	// Convert images to BGR buffers
	frameSize := r.width * r.height * 3
	bgr1 := make([]byte, frameSize)
	bgr2 := make([]byte, frameSize)

	if err := imageToBGRBuffer(img1, bgr1, r.width, r.height); err != nil {
		return nil, fmt.Errorf("failed to convert first image: %v", err)
	}

	if err := imageToBGRBuffer(img2, bgr2, r.width, r.height); err != nil {
		return nil, fmt.Errorf("failed to convert second image: %v", err)
	}

	// Use InterpolateBGR for processing
	outBuf, err := r.InterpolateBGR(bgr1, bgr2, timestep)
	if err != nil {
		return nil, err
	}

	return bgrBufferToImage(outBuf, r.width, r.height), nil
}

// Helper function to convert image.Image to BGR buffer
func imageToBGRBuffer(img image.Image, bgrBuf []byte, width, height int) error {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			idx := (y*width + x) * 3
			bgrBuf[idx] = byte(b >> 8)
			bgrBuf[idx+1] = byte(g >> 8)
			bgrBuf[idx+2] = byte(r >> 8)
		}
	}
	return nil
}

// Helper function to convert BGR buffer to image.RGBA
func bgrBufferToImage(bgrBuf []byte, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 3
			img.Set(x, y, color.RGBA{
				R: bgrBuf[idx+2],
				G: bgrBuf[idx+1],
				B: bgrBuf[idx],
				A: 255,
			})
		}
	}
	return img
}

// GetGPUCount returns the number of available GPUs
func GetGPUCount() int {
	return int(C.rife_get_gpu_count())
}

func btoi(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
