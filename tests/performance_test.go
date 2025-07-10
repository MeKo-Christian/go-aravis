package tests

import (
	"testing"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestPerformanceStringCaching tests the string caching functionality.
func TestPerformanceStringCaching(t *testing.T) {
	// Test that cleanup function exists and doesn't crash
	aravis.CleanupPerformanceCache()
	t.Log("CleanupPerformanceCache() completed successfully")

	// Test with camera if available
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		t.Skip("No cameras connected, skipping performance string caching tests")
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

	// Test that fast methods work
	_, err = camera.GetWidthFast()
	if err != nil {
		t.Logf("GetWidthFast() failed: %v (may not be supported)", err)
	} else {
		t.Log("GetWidthFast() succeeded")
	}

	_, err = camera.GetHeightFast()
	if err != nil {
		t.Logf("GetHeightFast() failed: %v (may not be supported)", err)
	} else {
		t.Log("GetHeightFast() succeeded")
	}

	_, err = camera.GetExposureTimeFast()
	if err != nil {
		t.Logf("GetExposureTimeFast() failed: %v (may not be supported)", err)
	} else {
		t.Log("GetExposureTimeFast() succeeded")
	}

	_, err = camera.GetGainFast()
	if err != nil {
		t.Logf("GetGainFast() failed: %v (may not be supported)", err)
	} else {
		t.Log("GetGainFast() succeeded")
	}

	// Test setting methods
	originalExposure, err := camera.GetExposureTimeFast()
	if err == nil && originalExposure > 0 {
		err = camera.SetExposureTimeFast(originalExposure * 1.01)
		if err == nil {
			camera.SetExposureTimeFast(originalExposure) // Restore
			t.Log("SetExposureTimeFast() succeeded")
		} else {
			t.Logf("SetExposureTimeFast() failed: %v (may not be supported)", err)
		}
	}

	originalGain, err := camera.GetGainFast()
	if err == nil && originalGain > 0 {
		err = camera.SetGainFast(originalGain * 1.01)
		if err == nil {
			camera.SetGainFast(originalGain) // Restore
			t.Log("SetGainFast() succeeded")
		} else {
			t.Logf("SetGainFast() failed: %v (may not be supported)", err)
		}
	}
}

// BenchmarkParameterAccessComparison benchmarks standard vs fast parameter access.
func BenchmarkParameterAccessComparison(b *testing.B) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		b.Skip("No cameras connected, skipping parameter access benchmarks")
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

	// Benchmark width access
	b.Run("Width/Standard", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetWidth()
		}
	})

	b.Run("Width/Fast", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetWidthFast()
		}
	})

	// Benchmark height access
	b.Run("Height/Standard", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetHeight()
		}
	})

	b.Run("Height/Fast", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetHeightFast()
		}
	})

	// Benchmark exposure time access
	b.Run("ExposureTime/Standard", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetExposureTime()
		}
	})

	b.Run("ExposureTime/Fast", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetExposureTimeFast()
		}
	})

	// Benchmark gain access
	b.Run("Gain/Standard", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetGain()
		}
	})

	b.Run("Gain/Fast", func(b *testing.B) {
		for range b.N {
			_, _ = camera.GetGainFast()
		}
	})
}

// BenchmarkBufferDataAccessComparison benchmarks buffer data access methods.
func BenchmarkBufferDataAccessComparison(b *testing.B) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		b.Skip("No cameras connected, skipping buffer access benchmarks")
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

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		b.Skip("Failed to get payload size")
		return
	}

	buffer, err := aravis.NewBuffer(uint(payloadSize))
	if err != nil {
		b.Skip("Failed to create buffer")
		return
	}

	// Pre-allocate destination buffer
	destBuffer := make([]byte, payloadSize)

	b.Run("BufferAccess/GetData", func(b *testing.B) {
		for range b.N {
			_, _ = buffer.GetData()
		}
	})

	b.Run("BufferAccess/GetDataSlice", func(b *testing.B) {
		for range b.N {
			_, _ = buffer.GetDataSlice()
		}
	})

	b.Run("BufferAccess/GetDataInto", func(b *testing.B) {
		for range b.N {
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("BufferAccess/GetDataUnsafe", func(b *testing.B) {
		for range b.N {
			_, _, _ = buffer.GetDataUnsafe()
		}
	})
}

// BenchmarkCombinedOperations benchmarks realistic usage patterns.
func BenchmarkCombinedOperations(b *testing.B) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		b.Skip("No cameras connected, skipping combined operations benchmarks")
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

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		b.Skip("Failed to get payload size")
		return
	}

	buffer, err := aravis.NewBuffer(uint(payloadSize))
	if err != nil {
		b.Skip("Failed to create buffer")
		return
	}

	destBuffer := make([]byte, payloadSize)

	// Benchmark typical streaming loop operations
	b.Run("StreamingLoop/Standard", func(b *testing.B) {
		for range b.N {
			// Typical operations in a streaming loop
			_, _ = camera.GetWidth()
			_, _ = camera.GetHeight()
			_, _ = camera.GetExposureTime()
			_, _ = buffer.GetData()
		}
	})

	b.Run("StreamingLoop/Optimized", func(b *testing.B) {
		for range b.N {
			// Optimized operations in a streaming loop
			_, _ = camera.GetWidthFast()
			_, _ = camera.GetHeightFast()
			_, _ = camera.GetExposureTimeFast()
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("StreamingLoop/ZeroCopy", func(b *testing.B) {
		for range b.N {
			// Zero-copy operations in a streaming loop
			_, _ = camera.GetWidthFast()
			_, _ = camera.GetHeightFast()
			_, _ = camera.GetExposureTimeFast()
			_, _ = buffer.GetDataSlice()
		}
	})
}

// BenchmarkMemoryAllocations measures memory allocation patterns.
func BenchmarkMemoryAllocations(b *testing.B) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		b.Skip("No cameras connected, skipping memory allocation benchmarks")
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

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		b.Skip("Failed to get payload size")
		return
	}

	buffer, err := aravis.NewBuffer(uint(payloadSize))
	if err != nil {
		b.Skip("Failed to create buffer")
		return
	}

	destBuffer := make([]byte, payloadSize)

	// Measure allocations per operation
	b.Run("Allocations/StandardMethods", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidth()
			_, _ = buffer.GetData()
		}
	})

	b.Run("Allocations/FastMethods", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidthFast()
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("Allocations/ZeroCopy", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidthFast()
			_, _ = buffer.GetDataSlice()
		}
	})
}
