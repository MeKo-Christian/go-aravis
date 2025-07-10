package main

import (
	"fmt"
	"log"
	"time"

	aravis "github.com/hybridgroup/go-aravis"
)

// Example demonstrating high-performance streaming using new optimizations.
func main() {
	fmt.Println("=== High-Performance Streaming Example ===")

	// Update device list
	aravis.UpdateDeviceList()

	// Get number of devices
	n, err := aravis.GetNumDevices()
	if err != nil {
		log.Fatal(err)
	}

	if n == 0 {
		fmt.Println("No cameras found. Connect a camera to test high-performance features.")
		return
	}

	// Use the first device
	deviceId, err := aravis.GetDeviceId(0)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Using device: %s\n", deviceId)

	// Create camera
	camera, err := aravis.NewCamera(deviceId)
	if err != nil {
		log.Fatal(err)
	}
	defer camera.Close()

	// Demonstrate fast camera operations using cached strings
	fmt.Println("\n=== Fast Camera Operations ===")

	// Fast width/height access (no string allocations)
	width, err := camera.GetWidthFast()
	if err == nil {
		fmt.Printf("Width (fast): %d\n", width)
	}

	height, err := camera.GetHeightFast()
	if err == nil {
		fmt.Printf("Height (fast): %d\n", height)
	}

	// Fast exposure time operations
	originalExposure, err := camera.GetExposureTimeFast()
	if err == nil {
		fmt.Printf("Original exposure (fast): %.2f μs\n", originalExposure)

		// Set new exposure using fast method
		newExposure := originalExposure * 1.5

		err = camera.SetExposureTimeFast(newExposure)
		if err == nil {
			fmt.Printf("Set new exposure (fast): %.2f μs\n", newExposure)

			// Restore original
			camera.SetExposureTimeFast(originalExposure)
		}
	}

	// Fast gain operations
	originalGain, err := camera.GetGainFast()
	if err == nil {
		fmt.Printf("Original gain (fast): %.2f\n", originalGain)
	}

	// Demonstrate buffer performance improvements
	fmt.Println("\n=== High-Performance Buffer Operations ===")

	// Configure for streaming
	camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS)

	// Create stream
	stream, err := camera.CreateStream()
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	// Get payload size
	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Payload size: %d bytes\n", payloadSize)

	// Create buffers for streaming
	numBuffers := 5
	buffers := make([]aravis.Buffer, numBuffers)

	for i := range numBuffers {
		buffer, err := aravis.NewBuffer(uint(payloadSize))
		if err != nil {
			log.Fatal(err)
		}

		buffers[i] = buffer
		stream.PushBuffer(buffer)
	}

	// Pre-allocate slice for zero-allocation data access
	dataSlice := make([]byte, payloadSize)

	// Start acquisition
	err = camera.StartAcquisition()
	if err != nil {
		log.Fatal(err)
	}
	defer camera.StopAcquisition()

	fmt.Println("Starting high-performance streaming test...")

	frameCount := 0
	startTime := time.Now()
	testDuration := 5 * time.Second

	for time.Since(startTime) < testDuration {
		// Get buffer with timeout
		buffer, err := stream.TimeoutPopBuffer(100 * time.Millisecond)
		if err != nil {
			continue // Timeout, try again
		}

		// Check buffer status
		status, err := buffer.GetStatus()
		if err != nil || status != aravis.BUFFER_STATUS_SUCCESS {
			stream.PushBuffer(buffer)
			continue
		}

		frameCount++

		// Demonstrate different high-performance data access methods
		switch frameCount % 100 {
		case 1:
			// Method 1: Zero-copy slice access (WARNING: requires careful memory management)
			dataSlice, err := buffer.GetDataSlice()
			if err == nil {
				fmt.Printf("Frame %d - Zero-copy access: %d bytes (first 4 bytes: %x)\n",
					frameCount, len(dataSlice), dataSlice[:4])
			}
		case 50:
			// Method 2: Copy into pre-allocated buffer (no allocations)
			bytesRead, err := buffer.GetDataInto(dataSlice)
			if err == nil {
				fmt.Printf("Frame %d - Pre-allocated copy: %d bytes\n", frameCount, bytesRead)
			}
		}

		// Return buffer to stream for reuse
		stream.PushBuffer(buffer)
	}

	elapsed := time.Since(startTime)
	fps := float64(frameCount) / elapsed.Seconds()

	fmt.Printf("\nPerformance Results:\n")
	fmt.Printf("- Frames captured: %d\n", frameCount)
	fmt.Printf("- Duration: %.2f seconds\n", elapsed.Seconds())
	fmt.Printf("- Average FPS: %.2f\n", fps)
	fmt.Printf("- Frame rate: %.2f MB/s\n", fps*float64(payloadSize)/1024/1024)

	fmt.Println("\n=== Performance Tips ===")
	fmt.Println("1. Use *Fast() methods for frequently accessed camera parameters")
	fmt.Println("2. Use GetDataSlice() for zero-copy access when possible")
	fmt.Println("3. Use GetDataInto() with pre-allocated buffers to avoid allocations")
	fmt.Println("4. Pre-allocate buffers and reuse them in streaming loops")
	fmt.Println("5. Use appropriate thread priorities for real-time applications")
}
