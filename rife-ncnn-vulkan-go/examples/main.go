// File: examples/main.go

package main

import (
	"image"
	"image/png"
	"log"
	"os"

	"github.com/Zelak312/interpolarr/rife-ncnn-vulkan-go"
)

func main() {
	// Create configuration
	config := rife.DefaultConfig(1280, 720)

	// Create RIFE instance
	r, err := rife.New(config)
	if err != nil {
		log.Fatalf("Failed to create RIFE: %v", err)
	}
	defer r.Close()

	// Load model
	err = r.LoadModel("../rife_wrapper/rife-ncnn-vulkan/models/rife-v4.26")
	if err != nil {
		log.Fatalf("Failed to load model: %v", err)
	}

	// Load input images
	img1, err := loadImage("frame1.png")
	if err != nil {
		log.Fatalf("Failed to load first image: %v", err)
	}

	img2, err := loadImage("frame2.png")
	if err != nil {
		log.Fatalf("Failed to load second image: %v", err)
	}

	// Interpolate frames
	result, err := r.Interpolate(img1, img2, 0.5)
	if err != nil {
		log.Fatalf("Failed to interpolate frames: %v", err)
	}

	// Save result
	err = saveImage(result, "interpolated.png")
	if err != nil {
		log.Fatalf("Failed to save result: %v", err)
	}
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f) // Changed from jpeg.Decode to png.Decode
	if err != nil {
		return nil, err
	}
	return img, nil
}

func saveImage(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img) // Changed from jpeg.Encode to png.Encode
}
