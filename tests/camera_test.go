package tests

import (
	"testing"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestCameraCreationWithoutDevice tests camera creation with invalid device.
func TestCameraCreationWithoutDevice(t *testing.T) {
	// Test with non-existent device
	_, err := aravis.NewCamera("nonexistent-device-12345")
	if err == nil {
		t.Log("Creating camera with non-existent device returned nil error (may be expected)")
	} else {
		t.Logf("Creating camera with non-existent device returned error: %v", err)
	}

	// Test with empty device name
	_, err = aravis.NewCamera("")
	if err == nil {
		t.Log("Creating camera with empty device name returned nil error (may be expected)")
	} else {
		t.Logf("Creating camera with empty device name returned error: %v", err)
	}
}

// TestCameraWithRealDevice tests camera operations with actual connected cameras.
func TestCameraWithRealDevice(t *testing.T) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("Failed to get device count: %v", err)
	}

	if numDevices == 0 {
		t.Skip("No cameras connected, skipping real camera tests")
		return
	}

	// Get first device
	deviceId, err := aravis.GetDeviceId(0)
	if err != nil {
		t.Fatalf("Failed to get device ID: %v", err)
	}

	// Create camera
	camera, err := aravis.NewCamera(deviceId)
	if err != nil {
		t.Fatalf("Failed to create camera: %v", err)
	}
	defer camera.Close()

	t.Logf("Successfully created camera for device: %s", deviceId)

	// Test basic camera information
	vendor, err := camera.GetVendorName()
	if err == nil {
		t.Logf("Vendor: %s", vendor)
	}

	model, err := camera.GetModelName()
	if err == nil {
		t.Logf("Model: %s", model)
	}

	serial, err := camera.GetDeviceSerialNumber()
	if err == nil {
		t.Logf("Serial: %s", serial)
	}

	// Test sensor information
	testCameraSensorInfo(t, camera)

	// Test performance methods
	testCameraPerformanceMethods(t, camera)
}

func testCameraSensorInfo(t *testing.T, camera aravis.Camera) {
	// Test getting sensor size
	width, err := camera.GetWidth()
	if err == nil {
		t.Logf("Width: %d", width)
	}

	height, err := camera.GetHeight()
	if err == nil {
		t.Logf("Height: %d", height)
	}

	// Test getting region
	x, y, width2, height2, err := camera.GetRegion()
	if err == nil {
		t.Logf("Region: x=%d, y=%d, width=%d, height=%d", x, y, width2, height2)
	}

	// Test exposure time
	exposure, err := camera.GetExposureTime()
	if err == nil {
		t.Logf("Exposure time: %.2f μs", exposure)
	}

	// Test gain
	gain, err := camera.GetGain()
	if err == nil {
		t.Logf("Gain: %.2f", gain)
	}

	// Test frame rate
	fps, err := camera.GetFrameRate()
	if err == nil {
		t.Logf("Frame rate: %.2f FPS", fps)
	}

	// Test payload size
	payloadSize, err := camera.GetPayloadSize()
	if err == nil {
		t.Logf("Payload size: %d bytes", payloadSize)
	}
}

func testCameraPerformanceMethods(t *testing.T, camera aravis.Camera) {
	// Test fast methods
	width, err := camera.GetWidthFast()
	if err == nil {
		t.Logf("Width (fast): %d", width)
	}

	height, err := camera.GetHeightFast()
	if err == nil {
		t.Logf("Height (fast): %d", height)
	}

	exposure, err := camera.GetExposureTimeFast()
	if err == nil {
		t.Logf("Exposure (fast): %.2f μs", exposure)
	}

	gain, err := camera.GetGainFast()
	if err == nil {
		t.Logf("Gain (fast): %.2f", gain)
	}
}

// TestCameraSettings tests camera parameter setting.
func TestCameraSettings(t *testing.T) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		t.Skip("No cameras connected, skipping camera settings tests")
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

	// Test setting acquisition mode
	err = camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS)
	if err != nil {
		t.Logf("Setting acquisition mode failed: %v (may not be supported)", err)
	}

	// Test frame rate setting (be conservative)
	originalFPS, err := camera.GetFrameRate()
	if err == nil {
		testFPS := originalFPS * 0.5 // Set to half the current rate

		err = camera.SetFrameRate(testFPS)
		if err == nil {
			// Restore original
			camera.SetFrameRate(originalFPS)
			t.Logf("Frame rate setting test successful")
		} else {
			t.Logf("Frame rate setting failed: %v (may not be supported)", err)
		}
	}

	// Test exposure time setting (be conservative)
	originalExposure, err := camera.GetExposureTime()
	if err == nil && originalExposure > 0 {
		testExposure := originalExposure * 1.1 // Slightly increase exposure

		err = camera.SetExposureTime(testExposure)
		if err == nil {
			// Restore original
			camera.SetExposureTime(originalExposure)
			t.Logf("Exposure time setting test successful")
		} else {
			t.Logf("Exposure time setting failed: %v (may not be supported)", err)
		}
	}
}

// TestCameraStreamCreation tests creating camera streams.
func TestCameraStreamCreation(t *testing.T) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		t.Skip("No cameras connected, skipping stream creation tests")
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

	// Test stream creation
	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}
	defer stream.Close()

	t.Log("Stream created successfully")

	// Test thread priority settings
	camera.ThreadPriority = aravis.ThreadPriorityNormal

	t.Log("Set thread priority to normal")

	camera.ThreadPriority = aravis.ThreadPriorityHigh

	t.Log("Set thread priority to high")

	// Don't test realtime priority as it requires special permissions
}

// BenchmarkCameraParameterAccess benchmarks parameter access performance.
func BenchmarkCameraParameterAccess(b *testing.B) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		b.Skip("No cameras connected, skipping benchmarks")
		return
	}

	deviceId, err := aravis.GetDeviceId(0)
	if err != nil {
		b.Skip("Failed to get device ID")
		return
	}

	camera, err := aravis.NewCamera(deviceId)
	if err != nil {
		b.Skip("Failed to create camera")
		return
	}
	defer camera.Close()

	b.Run("StandardWidth", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetWidth()
		}
	})

	b.Run("FastWidth", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetWidthFast()
		}
	})

	b.Run("StandardExposure", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetExposureTime()
		}
	})

	b.Run("FastExposure", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetExposureTimeFast()
		}
	})
}
