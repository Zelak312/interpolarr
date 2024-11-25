/*
Package rife provides Go bindings for RIFE (Real-Time Intermediate Flow Estimation),
a video frame interpolation library.

Basic usage:

	// Create a new RIFE instance
	config := rife.DefaultConfig(1280, 720)
	r, err := rife.New(config)
	if err != nil {
	    log.Fatal(err)
	}
	defer r.Close()

	// Load the model
	err = r.LoadModel("path/to/model")
	if err != nil {
	    log.Fatal(err)
	}

	// Interpolate between two frames
	result, err := r.Interpolate(frame1, frame2, 0.5)
	if err != nil {
	    log.Fatal(err)
	}
*/
package rife
