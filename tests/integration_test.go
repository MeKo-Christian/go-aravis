package tests

import (
	"testing"
	"time"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestFullWorkflow tests a complete camera workflow from discovery to image acquisition.
func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Step 1: Device discovery
	t.Log("Step 1: Device discovery")
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("Failed to get device count: %v", err)
	}

	if numDevices == 0 {
		t.Skip("No cameras connected, skipping integration test")
		return
	}

	t.Logf("Found %d device(s)", numDevices)

	// Step 2: Camera connection
	t.Log("Step 2: Camera connection")

	deviceId, err := aravis.GetDeviceId(0)
	if err != nil {
		t.Fatalf("Failed to get device ID: %v", err)
	}

	camera, err := aravis.NewCamera(deviceId)
	if err != nil {
		t.Fatalf("Failed to create camera: %v", err)
	}
	defer camera.Close()

	t.Logf("Connected to camera: %s", deviceId)

	// Step 3: Camera information gathering
	t.Log("Step 3: Camera information")
	testCameraInformation(t, camera)

	// Step 4: Camera configuration
	t.Log("Step 4: Camera configuration")
	testCameraConfiguration(t, camera)

	// Step 5: Stream creation and buffer management
	t.Log("Step 5: Stream and buffer setup")

	stream, _ := testStreamSetup(t, camera)
	if stream.IsNil() {
		return // Skip if stream setup failed
	}
	defer stream.Close()

	// Step 6: Image acquisition
	t.Log("Step 6: Image acquisition")
	testImageAcquisition(t, camera, stream)

	t.Log("Integration test completed successfully")
}

func testCameraInformation(t *testing.T, camera aravis.Camera) {
	vendor, err := camera.GetVendorName()
	if err == nil {
		t.Logf("  Vendor: %s", vendor)
	}

	model, err := camera.GetModelName()
	if err == nil {
		t.Logf("  Model: %s", model)
	}

	serial, err := camera.GetDeviceSerialNumber()
	if err == nil {
		t.Logf("  Serial: %s", serial)
	}

	width, err := camera.GetWidth()
	if err == nil {
		t.Logf("  Width: %d", width)
	}

	height, err := camera.GetHeight()
	if err == nil {
		t.Logf("  Height: %d", height)
	}

	payloadSize, err := camera.GetPayloadSize()
	if err == nil {
		t.Logf("  Payload size: %d bytes", payloadSize)
	}
}

func testCameraConfiguration(t *testing.T, camera aravis.Camera) {
	// Set acquisition mode
	err := camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS)
	if err != nil {
		t.Logf("  Failed to set acquisition mode: %v", err)
	} else {
		t.Log("  Set acquisition mode to continuous")
	}

	// Get and potentially adjust frame rate
	originalFPS, err := camera.GetFrameRate()
	if err == nil {
		t.Logf("  Original frame rate: %.2f FPS", originalFPS)

		// Try to set a conservative frame rate
		testFPS := 5.0 // 5 FPS should be safe for most cameras

		err = camera.SetFrameRate(testFPS)
		if err == nil {
			t.Logf("  Set frame rate to %.2f FPS", testFPS)

			// Restore original
			camera.SetFrameRate(originalFPS)
			t.Log("  Restored original frame rate")
		} else {
			t.Logf("  Failed to set frame rate: %v", err)
		}
	}

	// Get and potentially adjust exposure
	originalExposure, err := camera.GetExposureTime()
	if err == nil {
		t.Logf("  Original exposure: %.2f μs", originalExposure)
	}

	// Set thread priority
	camera.ThreadPriority = aravis.ThreadPriorityHigh

	t.Log("  Set thread priority to high")
}

func testStreamSetup(t *testing.T, camera aravis.Camera) (aravis.Stream, []aravis.Buffer) {
	// Create stream
	stream, err := camera.CreateStream()
	if err != nil {
		t.Errorf("  Failed to create stream: %v", err)
		return aravis.Stream{}, nil
	}

	t.Log("  Created stream")

	// Get payload size for buffer creation
	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Errorf("  Failed to get payload size: %v", err)
		stream.Close()

		return aravis.Stream{}, nil
	}

	// Create buffers
	numBuffers := 5
	buffers := make([]aravis.Buffer, numBuffers)

	for i := range numBuffers {
		buffer, err := aravis.NewBuffer(uint(payloadSize))
		if err != nil {
			t.Errorf("  Failed to create buffer %d: %v", i, err)
			stream.Close()

			return aravis.Stream{}, nil
		}

		buffers[i] = buffer
		stream.PushBuffer(buffer)
	}

	t.Logf("  Created %d buffers of %d bytes each", numBuffers, payloadSize)

	return stream, buffers
}

func testImageAcquisition(t *testing.T, camera aravis.Camera, stream aravis.Stream) {
	// Start acquisition
	err := camera.StartAcquisition()
	if err != nil {
		t.Errorf("  Failed to start acquisition: %v", err)
		return
	}
	defer camera.StopAcquisition()

	t.Log("  Started acquisition")

	// Capture a few frames
	framesAcquired := 0
	maxFrames := 10
	timeout := 1000 * time.Millisecond

	for framesAcquired < maxFrames {
		buffer, err := stream.TimeoutPopBuffer(timeout)
		if err != nil {
			t.Logf("  Frame %d: timeout (%v)", framesAcquired, err)
			break
		}

		// Check buffer status
		status, err := buffer.GetStatus()
		if err != nil {
			t.Errorf("  Failed to get buffer status: %v", err)
			stream.PushBuffer(buffer)

			continue
		}

		if status == aravis.BUFFER_STATUS_SUCCESS {
			// Test different data access methods
			testFrameDataAccess(t, buffer, framesAcquired)
			framesAcquired++
		} else {
			t.Logf("  Frame %d: status %d", framesAcquired, status)
		}

		// Return buffer to stream
		stream.PushBuffer(buffer)
	}

	t.Logf("  Successfully acquired %d frames", framesAcquired)

	if framesAcquired == 0 {
		t.Error("  No frames were successfully acquired")
	}
}

func testFrameDataAccess(t *testing.T, buffer aravis.Buffer, frameNum int) {
	// Test standard data access
	data, err := buffer.GetData()
	if err == nil && len(data) > 0 {
		t.Logf("    Frame %d: GetData() returned %d bytes", frameNum, len(data))

		// Test zero-copy access and compare
		dataSlice, err := buffer.GetDataSlice()
		if err == nil && len(dataSlice) > 0 {
			if len(data) == len(dataSlice) {
				t.Logf("    Frame %d: GetDataSlice() consistent size", frameNum)

				// Compare first few bytes
				if len(data) > 4 && len(dataSlice) > 4 {
					if data[0] == dataSlice[0] && data[1] == dataSlice[1] {
						t.Logf("    Frame %d: Data consistency verified", frameNum)
					} else {
						t.Errorf("    Frame %d: Data inconsistency detected", frameNum)
					}
				}
			} else {
				t.Errorf("    Frame %d: Size mismatch - GetData:%d vs GetDataSlice:%d",
					frameNum, len(data), len(dataSlice))
			}
		}

		// Test pre-allocated buffer copy
		destBuffer := make([]byte, len(data))

		bytesRead, err := buffer.GetDataInto(destBuffer)
		if err == nil {
			t.Logf("    Frame %d: GetDataInto() copied %d bytes", frameNum, bytesRead)
		}
	} else if err != nil {
		t.Logf("    Frame %d: GetData() failed: %v", frameNum, err)
	}
}

// TestStreamingPerformance tests sustained streaming performance.
func TestStreamingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming performance test in short mode")
	}

	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		t.Skip("No cameras connected, skipping streaming performance test")
		return
	}

	deviceId, err := aravis.GetDeviceId(0)
	if err != nil {
		t.Skip("Failed to get device ID")
		return
	}

	camera, err := aravis.NewCamera(deviceId)
	if err != nil {
		t.Skip("Failed to create camera")
		return
	}
	defer camera.Close()

	// Configure for performance
	camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS)
	camera.SetFrameRate(30.0) // Try for 30 FPS
	camera.ThreadPriority = aravis.ThreadPriorityHigh

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}
	defer stream.Close()

	// Setup buffers
	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Fatalf("Failed to get payload size: %v", err)
	}

	numBuffers := 10 // More buffers for high-speed streaming
	for range numBuffers {
		buffer, err := aravis.NewBuffer(uint(payloadSize))
		if err != nil {
			t.Fatalf("Failed to create buffer: %v", err)
		}

		stream.PushBuffer(buffer)
	}

	// Start acquisition
	err = camera.StartAcquisition()
	if err != nil {
		t.Fatalf("Failed to start acquisition: %v", err)
	}
	defer camera.StopAcquisition()

	// Stream for 5 seconds and measure performance
	startTime := time.Now()
	testDuration := 5 * time.Second
	frameCount := 0
	errorCount := 0
	timeoutCount := 0

	destBuffer := make([]byte, payloadSize) // Pre-allocated for zero-allocation copies

	t.Log("Starting streaming performance test...")

	for time.Since(startTime) < testDuration {
		buffer, err := stream.TimeoutPopBuffer(100 * time.Millisecond)
		if err != nil {
			timeoutCount++
			continue
		}

		status, err := buffer.GetStatus()
		if err != nil || status != aravis.BUFFER_STATUS_SUCCESS {
			errorCount++

			stream.PushBuffer(buffer)

			continue
		}

		frameCount++

		// Use optimized data access every 10th frame to test performance
		if frameCount%10 == 0 {
			_, _ = buffer.GetDataInto(destBuffer) // Zero-allocation copy
		}

		stream.PushBuffer(buffer)
	}

	elapsed := time.Since(startTime)
	fps := float64(frameCount) / elapsed.Seconds()

	t.Logf("Streaming performance results:")
	t.Logf("  Duration: %.2f seconds", elapsed.Seconds())
	t.Logf("  Frames acquired: %d", frameCount)
	t.Logf("  Timeouts: %d", timeoutCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Average FPS: %.2f", fps)
	t.Logf("  Data rate: %.2f MB/s", fps*float64(payloadSize)/1024/1024)

	// Basic performance expectations
	if frameCount == 0 {
		t.Error("No frames acquired during performance test")
	}

	if fps < 1.0 {
		t.Error("Frame rate is very low (< 1 FPS), check camera configuration")
	}

	if errorCount > frameCount/2 {
		t.Errorf("High error rate: %d errors out of %d attempts", errorCount, frameCount+errorCount+timeoutCount)
	}
}

// TestMultipleDevices tests operations with multiple cameras if available.
func TestMultipleDevices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multiple device test in short mode")
	}

	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("Failed to get device count: %v", err)
	}

	if numDevices < 2 {
		t.Skip("Less than 2 cameras connected, skipping multiple device test")
		return
	}

	t.Logf("Testing with %d devices", numDevices)

	// Test creating cameras for all devices
	cameras := make([]aravis.Camera, 0, numDevices)

	defer func() {
		for _, camera := range cameras {
			camera.Close()
		}
	}()

	for i := range numDevices {
		deviceId, err := aravis.GetDeviceId(i)
		if err != nil {
			t.Errorf("Failed to get device ID for device %d: %v", i, err)
			continue
		}

		camera, err := aravis.NewCamera(deviceId)
		if err != nil {
			t.Errorf("Failed to create camera for device %d (%s): %v", i, deviceId, err)
			continue
		}

		cameras = append(cameras, camera)

		t.Logf("Successfully created camera %d: %s", i, deviceId)
	}

	if len(cameras) == 0 {
		t.Fatal("Failed to create any cameras")
	}

	// Test basic operations on all cameras
	for i, camera := range cameras {
		vendor, _ := camera.GetVendorName()
		model, _ := camera.GetModelName()
		t.Logf("Camera %d: %s %s", i, vendor, model)

		// Test that cameras can be configured independently
		camera.ThreadPriority = aravis.ThreadPriorityNormal
	}

	t.Logf("Successfully tested %d cameras", len(cameras))
}
