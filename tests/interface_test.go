package tests

import (
	"testing"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestDeviceDiscovery tests basic device discovery functionality.
func TestDeviceDiscovery(t *testing.T) {
	// Update device list
	aravis.UpdateDeviceList()

	// Test getting number of devices
	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("Failed to get number of devices: %v", err)
	}

	t.Logf("Found %d devices", numDevices)

	// Test getting device IDs
	for i := range numDevices {
		deviceId, err := aravis.GetDeviceId(i)
		if err != nil {
			t.Errorf("Failed to get device ID for device %d: %v", i, err)
			continue
		}

		if deviceId == "" {
			t.Errorf("Device %d returned empty ID", i)
		}

		t.Logf("Device %d: %s", i, deviceId)
	}
}

// TestInterfaceDiscovery tests interface enumeration.
func TestInterfaceDiscovery(t *testing.T) {
	// Test getting number of interfaces
	numInterfaces, err := aravis.GetNumInferface() // Note: keeping original typo for compatibility
	if err != nil {
		t.Fatalf("Failed to get number of interfaces: %v", err)
	}

	t.Logf("Found %d interfaces", numInterfaces)

	// Test getting interface IDs
	for i := range numInterfaces {
		interfaceId, err := aravis.GetInterfaceId(i)
		if err != nil {
			t.Errorf("Failed to get interface ID for interface %d: %v", i, err)
			continue
		}

		if interfaceId == "" {
			t.Errorf("Interface %d returned empty ID", i)
		}

		t.Logf("Interface %d: %s", i, interfaceId)
	}
}

// TestDeviceAccessWithoutCamera tests that we can access device functions without actual cameras.
func TestDeviceAccessWithoutCamera(t *testing.T) {
	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("Failed to get device count: %v", err)
	}

	// Test boundary conditions
	_, err = aravis.GetDeviceId(numDevices + 100) // Should not crash
	if err == nil {
		t.Log("Getting non-existent device ID returned nil error (may be expected)")
	}
}

// TestInterfaceEnableDisable tests interface control functions.
func TestInterfaceEnableDisable(t *testing.T) {
	// These functions shouldn't crash with invalid inputs
	aravis.EnableInterface("nonexistent-interface")
	aravis.DisableInterface("nonexistent-interface")

	// Test with empty string
	aravis.EnableInterface("")
	aravis.DisableInterface("")

	t.Log("Interface enable/disable functions completed without crashing")
}

// TestShutdown tests the shutdown function.
func TestShutdown(t *testing.T) {
	// This should be safe to call
	aravis.Shutdown()
	t.Log("Shutdown completed successfully")
}
