package tests

import (
	"testing"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestMockOperations tests operations that should work without real cameras.
func TestMockOperations(t *testing.T) {
	// Test library initialization
	aravis.UpdateDeviceList()
	t.Log("UpdateDeviceList() completed")

	// Test getting device count (should work even with 0 devices)
	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Errorf("GetNumDevices() failed: %v", err)
	} else {
		t.Logf("GetNumDevices() returned: %d", numDevices)
	}

	// Test getting interface count
	numInterfaces, err := aravis.GetNumInferface()
	if err != nil {
		t.Errorf("GetNumInferface() failed: %v", err)
	} else {
		t.Logf("GetNumInferface() returned: %d", numInterfaces)
	}

	// Test shutdown
	aravis.Shutdown()
	t.Log("Shutdown() completed")
}

// TestErrorHandling tests error handling with invalid inputs.
func TestErrorHandling(t *testing.T) {
	// Test with invalid device indices
	_, err := aravis.GetDeviceId(999)
	if err != nil {
		t.Logf("GetDeviceId(999) correctly returned error: %v", err)
	} else {
		t.Log("GetDeviceId(999) returned nil error (may be expected)")
	}

	// Test with invalid interface indices
	_, err = aravis.GetInterfaceId(999)
	if err != nil {
		t.Logf("GetInterfaceId(999) correctly returned error: %v", err)
	} else {
		t.Log("GetInterfaceId(999) returned nil error (may be expected)")
	}

	// Test camera creation with invalid device
	camera, err := aravis.NewCamera("invalid-device-name-12345")
	if err != nil {
		t.Logf("NewCamera with invalid device correctly returned error: %v", err)
	} else {
		t.Log("NewCamera with invalid device returned nil error (unexpected)")

		if !camera.IsNil() {
			camera.Close()
		}
	}

	// Test empty device name
	camera, err = aravis.NewCamera("")
	if err != nil {
		t.Logf("NewCamera with empty device name correctly returned error: %v", err)
	} else {
		t.Log("NewCamera with empty device name returned nil error (may be expected)")

		if !camera.IsNil() {
			camera.Close()
		}
	}
}

// TestBufferOperationsWithoutCamera tests buffer operations that don't require cameras.
func TestBufferOperationsWithoutCamera(t *testing.T) {
	// Test buffer creation with various sizes
	testSizes := []uint{0, 1, 1024, 1048576}

	for _, size := range testSizes {
		buffer, err := aravis.NewBuffer(size)
		if err != nil {
			t.Logf("NewBuffer(%d) failed: %v", size, err)
			continue
		}

		t.Logf("NewBuffer(%d) succeeded", size)

		// Test operations that might work without real data
		_, err = buffer.GetStatus()
		if err != nil {
			t.Logf("GetStatus() on empty buffer failed: %v (expected)", err)
		}

		_, err = buffer.GetData()
		if err != nil {
			t.Logf("GetData() on empty buffer failed: %v (expected)", err)
		}

		_, err = buffer.GetDataSlice()
		if err != nil {
			t.Logf("GetDataSlice() on empty buffer failed: %v (expected)", err)
		}

		_, err = buffer.GetNumParts()
		if err != nil {
			t.Logf("GetNumParts() on empty buffer failed: %v (expected)", err)
		}

		hasChunks := buffer.HasChunks()
		t.Logf("HasChunks() on empty buffer returned: %t", hasChunks)
	}
}

// TestPerformanceCacheOperations tests performance optimization functions.
func TestPerformanceCacheOperations(t *testing.T) {
	// Test cleanup function (should be safe to call)
	aravis.CleanupPerformanceCache()
	t.Log("CleanupPerformanceCache() completed successfully")

	// Call it multiple times to ensure it's safe
	aravis.CleanupPerformanceCache()
	aravis.CleanupPerformanceCache()
	t.Log("Multiple CleanupPerformanceCache() calls completed successfully")
}

// TestInterfaceOperations tests interface control operations.
func TestInterfaceOperations(t *testing.T) {
	// Test enable/disable with various inputs
	testInterfaces := []string{"", "fake", "usb", "gige", "nonexistent-interface-12345"}

	for _, iface := range testInterfaces {
		// These should not crash
		aravis.EnableInterface(iface)
		t.Logf("EnableInterface('%s') completed", iface)

		aravis.DisableInterface(iface)
		t.Logf("DisableInterface('%s') completed", iface)
	}
}

// TestStructuralOperations tests operations that test library structure.
func TestStructuralOperations(t *testing.T) {
	// Test that we can call update multiple times
	for i := range 3 {
		aravis.UpdateDeviceList()
		t.Logf("UpdateDeviceList() call %d completed", i+1)
	}

	// Test that device/interface counts are consistent
	devices1, err1 := aravis.GetNumDevices()

	devices2, err2 := aravis.GetNumDevices()
	if err1 != nil || err2 != nil {
		t.Logf("GetNumDevices() errors: %v, %v", err1, err2)
	} else if devices1 != devices2 {
		t.Errorf("Inconsistent device count: %d vs %d", devices1, devices2)
	} else {
		t.Logf("Consistent device count: %d", devices1)
	}

	interfaces1, err1 := aravis.GetNumInferface()

	interfaces2, err2 := aravis.GetNumInferface()
	if err1 != nil || err2 != nil {
		t.Logf("GetNumInferface() errors: %v, %v", err1, err2)
	} else if interfaces1 != interfaces2 {
		t.Errorf("Inconsistent interface count: %d vs %d", interfaces1, interfaces2)
	} else {
		t.Logf("Consistent interface count: %d", interfaces1)
	}
}

// TestConstants tests that constants are defined and accessible.
func TestConstants(t *testing.T) {
	// Test acquisition mode constants
	constants := map[string]interface{}{
		"ACQUISITION_MODE_CONTINUOUS":   aravis.ACQUISITION_MODE_CONTINUOUS,
		"ACQUISITION_MODE_SINGLE_FRAME": aravis.ACQUISITION_MODE_SINGLE_FRAME,
	}

	for name, value := range constants {
		t.Logf("Constant %s = %v", name, value)
	}

	// Test buffer status constants
	statusConstants := map[string]interface{}{
		"BUFFER_STATUS_SUCCESS":         aravis.BUFFER_STATUS_SUCCESS,
		"BUFFER_STATUS_TIMEOUT":         aravis.BUFFER_STATUS_TIMEOUT,
		"BUFFER_STATUS_MISSING_PACKETS": aravis.BUFFER_STATUS_MISSING_PACKETS,
	}

	for name, value := range statusConstants {
		t.Logf("Status constant %s = %v", name, value)
	}

	// Test thread priority constants
	priorityConstants := map[string]interface{}{
		"ThreadPriorityNormal":   aravis.ThreadPriorityNormal,
		"ThreadPriorityHigh":     aravis.ThreadPriorityHigh,
		"ThreadPriorityRealtime": aravis.ThreadPriorityRealtime,
	}

	for name, value := range priorityConstants {
		t.Logf("Priority constant %s = %v", name, value)
	}
}

// TestBoundaryConditions tests edge cases and boundary conditions.
func TestBoundaryConditions(t *testing.T) {
	// Test with maximum uint values
	_, err := aravis.GetDeviceId(^uint(0)) // Maximum uint value
	if err != nil {
		t.Logf("GetDeviceId(max_uint) correctly handled: %v", err)
	}

	_, err = aravis.GetInterfaceId(^uint(0))
	if err != nil {
		t.Logf("GetInterfaceId(max_uint) correctly handled: %v", err)
	}

	// Test buffer creation with large sizes
	largeSize := uint(1024 * 1024 * 1024) // 1GB

	_, err = aravis.NewBuffer(largeSize)
	if err != nil {
		t.Logf("NewBuffer(1GB) correctly failed: %v", err)
	} else {
		t.Log("NewBuffer(1GB) unexpectedly succeeded (system may have enough memory)")
	}

	// Test buffer creation with zero size
	buffer, err := aravis.NewBuffer(0)
	if err != nil {
		t.Logf("NewBuffer(0) failed: %v", err)
	} else {
		t.Log("NewBuffer(0) succeeded")

		// Test operations on zero-size buffer
		data, err := buffer.GetData()
		if err != nil {
			t.Logf("GetData() on zero-size buffer failed: %v", err)
		} else {
			t.Logf("GetData() on zero-size buffer returned %d bytes", len(data))
		}
	}
}
