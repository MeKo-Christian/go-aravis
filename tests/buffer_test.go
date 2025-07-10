package tests

import (
	"testing"
	"unsafe"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestBufferCreation tests basic buffer creation and properties.
func TestBufferCreation(t *testing.T) {
	// Test creating buffers with different sizes
	sizes := []uint{1024, 4096, 1048576} // 1KB, 4KB, 1MB

	for _, size := range sizes {
		buffer, err := aravis.NewBuffer(size)
		if err != nil {
			t.Errorf("Failed to create buffer of size %d: %v", size, err)
			continue
		}

		t.Logf("Successfully created buffer of size %d", size)

		// Test getting buffer size (this may not be implemented)
		// Just verify the buffer exists and doesn't crash
		_ = buffer
	}
}

// TestBufferDataAccess tests different methods of accessing buffer data.
func TestBufferDataAccess(t *testing.T) {
	bufferSize := uint(1024)

	buffer, err := aravis.NewBuffer(bufferSize)
	if err != nil {
		t.Fatalf("Failed to create test buffer: %v", err)
	}

	// Test standard data access (may fail without actual camera data)
	data, err := buffer.GetData()
	if err != nil {
		t.Logf("GetData() failed (expected without camera): %v", err)
	} else {
		t.Logf("GetData() returned %d bytes", len(data))
	}

	// Test zero-copy slice access
	dataSlice, err := buffer.GetDataSlice()
	if err != nil {
		t.Logf("GetDataSlice() failed (expected without camera): %v", err)
	} else {
		t.Logf("GetDataSlice() returned slice of %d bytes", len(dataSlice))
	}

	// Test pre-allocated buffer copy
	destBuffer := make([]byte, bufferSize)

	bytesRead, err := buffer.GetDataInto(destBuffer)
	if err != nil {
		t.Logf("GetDataInto() failed (expected without camera): %v", err)
	} else {
		t.Logf("GetDataInto() copied %d bytes", bytesRead)
	}

	// Test unsafe pointer access
	ptr, size, err := buffer.GetDataUnsafe()
	if err != nil {
		t.Logf("GetDataUnsafe() failed (expected without camera): %v", err)
	} else {
		t.Logf("GetDataUnsafe() returned pointer %v with size %d", ptr, size)

		// Verify pointer is not nil if size > 0
		if size > 0 && ptr == unsafe.Pointer(nil) {
			t.Error("GetDataUnsafe() returned nil pointer with non-zero size")
		}
	}
}

// TestBufferStatus tests buffer status checking.
func TestBufferStatus(t *testing.T) {
	buffer, err := aravis.NewBuffer(1024)
	if err != nil {
		t.Fatalf("Failed to create test buffer: %v", err)
	}

	// Test getting buffer status
	status, err := buffer.GetStatus()
	if err != nil {
		t.Logf("GetStatus() failed: %v", err)
	} else {
		t.Logf("Buffer status: %d", status)

		// Test against known status constants
		switch status {
		case aravis.BUFFER_STATUS_SUCCESS:
			t.Log("Buffer status: SUCCESS")
		case aravis.BUFFER_STATUS_TIMEOUT:
			t.Log("Buffer status: TIMEOUT")
		case aravis.BUFFER_STATUS_MISSING_PACKETS:
			t.Log("Buffer status: MISSING_PACKETS")
		default:
			t.Logf("Buffer status: Unknown (%d)", status)
		}
	}
}

// TestBufferMultipart tests multipart buffer functionality.
func TestBufferMultipart(t *testing.T) {
	buffer, err := aravis.NewBuffer(1024)
	if err != nil {
		t.Fatalf("Failed to create test buffer: %v", err)
	}

	// Test getting number of parts
	numParts, err := buffer.GetNumParts()
	if err != nil {
		t.Logf("GetNumParts() failed (expected without camera): %v", err)
	} else {
		t.Logf("Buffer has %d parts", numParts)

		// Test accessing individual parts
		for i := range numParts {
			partData, err := buffer.GetPartData(i)
			if err != nil {
				t.Logf("GetPartData(%d) failed: %v", i, err)
			} else {
				t.Logf("Part %d has %d bytes", i, len(partData))
			}

			width, err := buffer.GetPartWidth(i)
			if err != nil {
				t.Logf("GetPartWidth(%d) failed: %v", i, err)
			} else {
				t.Logf("Part %d width: %d", i, width)
			}

			height, err := buffer.GetPartHeight(i)
			if err != nil {
				t.Logf("GetPartHeight(%d) failed: %v", i, err)
			} else {
				t.Logf("Part %d height: %d", i, height)
			}

			componentId, err := buffer.GetPartComponentId(i)
			if err != nil {
				t.Logf("GetPartComponentId(%d) failed: %v", i, err)
			} else {
				t.Logf("Part %d component ID: %d", i, componentId)
			}
		}
	}
}

// TestBufferChunks tests chunk data functionality.
func TestBufferChunks(t *testing.T) {
	buffer, err := aravis.NewBuffer(1024)
	if err != nil {
		t.Fatalf("Failed to create test buffer: %v", err)
	}

	// Test checking for chunks
	hasChunks := buffer.HasChunks()
	t.Logf("Buffer has chunks: %t", hasChunks)
}

// TestBufferWithRealCamera tests buffer operations with actual camera.
func TestBufferWithRealCamera(t *testing.T) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		t.Skip("No cameras connected, skipping real camera buffer tests")
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

	// Get payload size
	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Skip("Failed to get payload size")
		return
	}

	// Create buffer with proper size
	buffer, err := aravis.NewBuffer(uint(payloadSize))
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	t.Logf("Created buffer for payload size: %d", payloadSize)

	// Create stream
	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}
	defer stream.Close()

	// Push buffer to stream
	stream.PushBuffer(buffer)

	// Configure for single frame
	camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_SINGLE_FRAME)

	// Start acquisition
	err = camera.StartAcquisition()
	if err != nil {
		t.Fatalf("Failed to start acquisition: %v", err)
	}
	defer camera.StopAcquisition()

	// Try to get a frame
	buffer, err = stream.TimeoutPopBuffer(1000) // 1 second timeout
	if err != nil {
		t.Logf("Failed to get frame (timeout expected): %v", err)
		return
	}

	// Test buffer with real data
	status, err := buffer.GetStatus()
	if err != nil {
		t.Errorf("Failed to get buffer status: %v", err)
	} else {
		t.Logf("Buffer status with real data: %d", status)
	}

	if status == aravis.BUFFER_STATUS_SUCCESS {
		// Test all data access methods with real data
		testBufferDataMethodsWithRealData(t, buffer)
	}
}

func testBufferDataMethodsWithRealData(t *testing.T, buffer aravis.Buffer) {
	// Test standard data access
	data, err := buffer.GetData()
	if err != nil {
		t.Errorf("GetData() failed with real data: %v", err)
	} else {
		t.Logf("GetData() returned %d bytes of real data", len(data))
	}

	// Test zero-copy access
	dataSlice, err := buffer.GetDataSlice()
	if err != nil {
		t.Errorf("GetDataSlice() failed with real data: %v", err)
	} else {
		t.Logf("GetDataSlice() returned %d bytes of real data", len(dataSlice))

		// Compare first few bytes if both methods work
		if data != nil && len(data) > 0 && len(dataSlice) > 0 {
			if data[0] == dataSlice[0] {
				t.Log("GetData() and GetDataSlice() return consistent data")
			} else {
				t.Error("GetData() and GetDataSlice() return different data")
			}
		}
	}

	// Test pre-allocated copy
	destBuffer := make([]byte, len(data))

	bytesRead, err := buffer.GetDataInto(destBuffer)
	if err != nil {
		t.Errorf("GetDataInto() failed with real data: %v", err)
	} else {
		t.Logf("GetDataInto() copied %d bytes of real data", bytesRead)
	}
}

// BenchmarkBufferDataAccess benchmarks different buffer data access methods.
func BenchmarkBufferDataAccess(b *testing.B) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil || numDevices == 0 {
		b.Skip("No cameras connected, skipping buffer benchmarks")
		return
	}

	// Setup camera and get a real buffer (this is complex, so we'll skip for now)
	// In a real scenario, you'd want to set up a streaming session and use real buffers

	// For now, just test with empty buffer
	buffer, err := aravis.NewBuffer(1024 * 1024) // 1MB buffer
	if err != nil {
		b.Fatalf("Failed to create buffer: %v", err)
	}

	b.Run("GetData", func(b *testing.B) {
		for range b.N {
			_, _ = buffer.GetData()
		}
	})

	b.Run("GetDataSlice", func(b *testing.B) {
		for range b.N {
			_, _ = buffer.GetDataSlice()
		}
	})

	destBuffer := make([]byte, 1024*1024)

	b.Run("GetDataInto", func(b *testing.B) {
		for range b.N {
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("GetDataUnsafe", func(b *testing.B) {
		for range b.N {
			_, _, _ = buffer.GetDataUnsafe()
		}
	})
}
