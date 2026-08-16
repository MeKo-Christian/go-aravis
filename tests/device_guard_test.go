package tests

// Guards around the Device type: the GigE-only control calls, the zero-size
// memory read, and the nil receiver every other method used to dereference
// blindly. All of these used to reach the C layer, where they produced a GLib
// CRITICAL, a bad cast or a Go panic instead of an error.

import (
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// TestControlCallsRejectNonGVDevice pins the guard on the two GigE-only calls.
// The Fake camera is not a GigE Vision device, so TakeControl and LeaveControl
// must report that rather than casting through ARV_GV_DEVICE(), which tripped
//
//	invalid cast from 'ArvFakeDevice' to 'ArvGvDevice'
//
// and then handed Aravis a pointer to something that is not an ArvGvDevice.
func TestControlCallsRejectNonGVDevice(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	isGV, err := camera.IsGVDevice()
	if err != nil {
		t.Fatalf("IsGVDevice() returned error: %v", err)
	}

	// requireFakeCamera returns Fake_1 in both modes, and Fake is never a GigE
	// Vision device — so this is a precondition, not a reason to skip.
	if isGV {
		t.Fatalf("the %s camera reports itself as a GigE Vision device", fakeDeviceID)
	}

	device, err := camera.GetDevice()
	if err != nil {
		t.Fatalf("GetDevice() returned error: %v", err)
	}

	t.Run("TakeControl", func(t *testing.T) {
		ok, err := device.TakeControl()
		if err == nil {
			t.Error("TakeControl() on a non-GigE device returned nil error; want an error")
		}

		if ok {
			t.Error("TakeControl() on a non-GigE device reported success")
		}
	})

	t.Run("LeaveControl", func(t *testing.T) {
		ok, err := device.LeaveControl()
		if err == nil {
			t.Error("LeaveControl() on a non-GigE device returned nil error; want an error")
		}

		if ok {
			t.Error("LeaveControl() on a non-GigE device reported success")
		}
	})
}

// TestReadMemoryZeroSize covers the panic: ReadMemory allocated a zero-length
// slice and then took the address of its first element, which is out of range.
// A zero-size read is now rejected the way WriteMemory rejects an empty write.
func TestReadMemoryZeroSize(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	device, err := camera.GetDevice()
	if err != nil {
		t.Fatalf("GetDevice() returned error: %v", err)
	}

	data, err := device.ReadMemory(0x0, 0)
	if err == nil {
		t.Error("ReadMemory(0x0, 0) returned nil error; want an error, as WriteMemory gives for an empty write")
	}

	if data != nil {
		t.Errorf("ReadMemory(0x0, 0) returned %d bytes; want nil", len(data))
	}
}

// TestZeroValueDeviceMethods runs every method that talks to Aravis against the
// zero value. Only Close, IsClosed and IsNil used to check the receiver; the
// rest passed a NULL ArvDevice straight into the C library.
func TestZeroValueDeviceMethods(t *testing.T) {
	var device aravis.Device

	if !device.IsNil() {
		t.Fatal("the zero-value Device does not report IsNil")
	}

	tests := []struct {
		name string
		call func() error
	}{
		{"TakeControl", func() error { _, err := device.TakeControl(); return err }},
		{"LeaveControl", func() error { _, err := device.LeaveControl(); return err }},
		{"SetStringFeatureValue", func() error { return device.SetStringFeatureValue("Width", "512") }},
		{"GetStringFeatureValue", func() error { _, err := device.GetStringFeatureValue("Width"); return err }},
		{"SetIntegerFeatureValue", func() error { return device.SetIntegerFeatureValue("Width", 512) }},
		{"GetIntegerFeatureValue", func() error { _, err := device.GetIntegerFeatureValue("Width"); return err }},
		{"SetFloatFeatureValue", func() error { return device.SetFloatFeatureValue("Gain", 1) }},
		{"GetFloatFeatureValue", func() error { _, err := device.GetFloatFeatureValue("Gain"); return err }},
		{"SetNodeFeatureValue", func() error { return device.SetNodeFeatureValue("Width", "512") }},
		{"ExecuteCommand", func() error { return device.ExecuteCommand("AcquisitionStart") }},
		{"ReadMemory", func() error { _, err := device.ReadMemory(0x0, 4); return err }},
		{"WriteMemory", func() error { return device.WriteMemory(0x0, []byte{1, 2, 3, 4}) }},
		{"ReadRegister", func() error { _, err := device.ReadRegister(0x0); return err }},
		{"WriteRegister", func() error { return device.WriteRegister(0x0, 0) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Errorf("%s on a zero-value Device returned nil error; want an error", tc.name)
			}
		})
	}
}

// TestClosedOwnedDeviceMethods covers the second half of the receiver guard,
// raised in review: Close drops the reference but leaves the pointer in the
// struct, so a nil check alone still let every method hand Aravis a freed
// ArvDevice — a dangling pointer no assertion inside the library catches.
func TestClosedOwnedDeviceMethods(t *testing.T) {
	device, err := aravis.OpenDevice(fakeDeviceID)
	if err != nil {
		t.Fatalf("OpenDevice(%s) returned error: %v", fakeDeviceID, err)
	}

	device.Close()

	if !device.IsClosed() {
		t.Fatal("the device does not report itself closed after Close")
	}

	tests := []struct {
		name string
		call func() error
	}{
		{"TakeControl", func() error { _, err := device.TakeControl(); return err }},
		{"GetStringFeatureValue", func() error { _, err := device.GetStringFeatureValue("Width"); return err }},
		{"SetIntegerFeatureValue", func() error { return device.SetIntegerFeatureValue("Width", 512) }},
		{"ExecuteCommand", func() error { return device.ExecuteCommand("AcquisitionStart") }},
		{"ReadMemory", func() error { _, err := device.ReadMemory(0x0, 4); return err }},
		{"ReadRegister", func() error { _, err := device.ReadRegister(0x0); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Errorf("%s on a closed Device returned nil error; want an error", tc.name)
			}
		})
	}
}

// TestClosedCameraDeviceAccess is the same point one level up, also raised in
// review: Camera.Close unrefs the camera but leaves c.camera set, so both
// methods used to reach Aravis with a freed ArvCamera.
func TestClosedCameraDeviceAccess(t *testing.T) {
	camera := requireFakeCamera(t)
	camera.Close()

	if !camera.IsClosed() {
		t.Fatal("the camera does not report itself closed after Close")
	}

	device, err := camera.GetDevice()
	if err == nil {
		t.Error("GetDevice() on a closed camera returned nil error; want an error")
	}

	if !device.IsNil() {
		t.Error("GetDevice() on a closed camera returned a non-nil device")
	}

	if _, err := camera.IsGVDevice(); err == nil {
		t.Error("IsGVDevice() on a closed camera returned nil error; want an error")
	}
}

// TestGetDeviceOnFakeCamera is the positive control for the guards above: they
// must not pass by rejecting everything. A camera's device is borrowed, so it
// comes back usable, with a nil error, and must not be closed by the caller —
// see TestBorrowedDeviceCloseDoesNotUnref for that half.
func TestGetDeviceOnFakeCamera(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	device, err := camera.GetDevice()
	if err != nil {
		t.Fatalf("GetDevice() returned error: %v", err)
	}

	if device.IsNil() {
		t.Fatal("GetDevice() returned a nil device for a live camera")
	}

	if device.IsClosed() {
		t.Error("GetDevice() returned a device that reports itself closed")
	}

	// A feature read proves the pointer is the camera's real device and not
	// something the guards let through. The error is not asserted here: the
	// generic getters pass a nil GError to Aravis and report cgo's errno
	// instead, which is a separate P6 item.
	if vendor, _ := device.GetStringFeatureValue("DeviceVendorName"); vendor != "Aravis" {
		t.Errorf("GetStringFeatureValue(DeviceVendorName) = %q; want %q", vendor, "Aravis")
	}
}

// TestGetDeviceOnZeroValueCamera is the failure half of the same contract.
// Both calls used to hardcode a nil error, so a caller had no way to learn that
// what came back was a Device wrapping nothing.
func TestGetDeviceOnZeroValueCamera(t *testing.T) {
	var camera aravis.Camera

	device, err := camera.GetDevice()
	if err == nil {
		t.Error("GetDevice() on a zero-value camera returned nil error; want an error")
	}

	if !device.IsNil() {
		t.Error("GetDevice() on a zero-value camera returned a non-nil device")
	}

	if _, err := camera.IsGVDevice(); err == nil {
		t.Error("IsGVDevice() on a zero-value camera returned nil error; want an error")
	}
}

// TestIsGVDeviceOnFakeCamera is the second positive control: the Fake camera is
// not a GigE Vision device, and asking must succeed rather than fail.
func TestIsGVDeviceOnFakeCamera(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	isGV, err := camera.IsGVDevice()
	if err != nil {
		t.Fatalf("IsGVDevice() returned error: %v", err)
	}

	if isGV {
		t.Error("IsGVDevice() reported true for the Fake camera")
	}
}
