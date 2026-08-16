package tests

// Receiver guards for the twelve *Fast methods in performance.go.
//
// These were the gap left by the device.go guard sweep: every method there
// checks its receiver, but the *Fast methods live in another file and handed
// c.camera / d.device to Aravis unchecked. A NULL trips a GLib assertion; a
// *closed* handle is worse, because Close drops the reference and leaves the
// pointer in place, so the value is dangling rather than NULL and no assertion
// inside Aravis can catch it. Both halves are covered below.

import (
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// fastCameraCalls returns every *Fast method on Camera, bound to the given
// camera. Both guard tests drive the same list so neither can drift.
func fastCameraCalls(camera *aravis.Camera) []struct {
	name string
	call func() error
} {
	return []struct {
		name string
		call func() error
	}{
		{"GetWidthFast", func() error { _, err := camera.GetWidthFast(); return err }},
		{"GetHeightFast", func() error { _, err := camera.GetHeightFast(); return err }},
		{"GetExposureTimeFast", func() error { _, err := camera.GetExposureTimeFast(); return err }},
		{"GetGainFast", func() error { _, err := camera.GetGainFast(); return err }},
		{"SetExposureTimeFast", func() error { return camera.SetExposureTimeFast(10000) }},
		{"SetGainFast", func() error { return camera.SetGainFast(1) }},
	}
}

// fastDeviceCalls returns every *Fast method on Device, bound to the given
// device.
func fastDeviceCalls(device *aravis.Device) []struct {
	name string
	call func() error
} {
	return []struct {
		name string
		call func() error
	}{
		{"GetStringFeatureValueFast", func() error {
			_, err := device.GetStringFeatureValueFast("DeviceVendorName")

			return err
		}},
		{"SetStringFeatureValueFast", func() error {
			return device.SetStringFeatureValueFast("DeviceVendorName", "x")
		}},
		{"GetIntegerFeatureValueFast", func() error {
			_, err := device.GetIntegerFeatureValueFast("Width")

			return err
		}},
		{"SetIntegerFeatureValueFast", func() error {
			return device.SetIntegerFeatureValueFast("Width", 512)
		}},
		{"GetFloatFeatureValueFast", func() error {
			_, err := device.GetFloatFeatureValueFast("ExposureTimeAbs")

			return err
		}},
		{"SetFloatFeatureValueFast", func() error {
			return device.SetFloatFeatureValueFast("ExposureTimeAbs", 10000)
		}},
	}
}

// TestZeroValueFastMethods runs all twelve against the zero value, which holds
// no underlying Aravis object at all.
func TestZeroValueFastMethods(t *testing.T) {
	var camera aravis.Camera

	var device aravis.Device

	for _, tc := range fastCameraCalls(&camera) {
		t.Run("Camera/"+tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Errorf("%s on a zero-value Camera returned nil error; want an error", tc.name)
			}
		})
	}

	for _, tc := range fastDeviceCalls(&device) {
		t.Run("Device/"+tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Errorf("%s on a zero-value Device returned nil error; want an error", tc.name)
			}
		})
	}
}

// TestClosedFastMethods is the half a nil check cannot catch: the pointer is
// still set after Close, and only the shared close flag knows it is dangling.
func TestClosedFastMethods(t *testing.T) {
	camera := requireFakeCamera(t)
	camera.Close()

	if !camera.IsClosed() {
		t.Fatal("the camera does not report itself closed after Close")
	}

	device, err := aravis.OpenDevice(fakeDeviceID)
	if err != nil {
		t.Fatalf("OpenDevice(%s) returned error: %v", fakeDeviceID, err)
	}

	device.Close()

	if !device.IsClosed() {
		t.Fatal("the device does not report itself closed after Close")
	}

	for _, tc := range fastCameraCalls(&camera) {
		t.Run("Camera/"+tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Errorf("%s on a closed Camera returned nil error; want an error", tc.name)
			}
		})
	}

	for _, tc := range fastDeviceCalls(&device) {
		t.Run("Device/"+tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Errorf("%s on a closed Device returned nil error; want an error", tc.name)
			}
		})
	}
}

// TestOpenFastMethodsStillWork is the positive control. Without it the guards
// above would pass just as well if every *Fast method had been changed to
// return an error unconditionally.
//
// Only the accessors the Fake backend actually supports are asserted to
// succeed: Fake has no "ExposureTime" and no "Gain" node, which is exactly the
// mismatch the package documentation warns about, so those two are excluded
// rather than pinning a backend quirk as this binding's contract.
func TestOpenFastMethodsStillWork(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	device := requireFakeDevice(t)

	if _, err := camera.GetWidthFast(); err != nil {
		t.Errorf("GetWidthFast() on an open camera returned error: %v", err)
	}

	if _, err := camera.GetHeightFast(); err != nil {
		t.Errorf("GetHeightFast() on an open camera returned error: %v", err)
	}

	if _, err := device.GetStringFeatureValueFast("DeviceVendorName"); err != nil {
		t.Errorf("GetStringFeatureValueFast(DeviceVendorName) on an open device returned error: %v", err)
	}

	if _, err := device.GetIntegerFeatureValueFast("Width"); err != nil {
		t.Errorf("GetIntegerFeatureValueFast(Width) on an open device returned error: %v", err)
	}

	if _, err := device.GetFloatFeatureValueFast("ExposureTimeAbs"); err != nil {
		t.Errorf("GetFloatFeatureValueFast(ExposureTimeAbs) on an open device returned error: %v", err)
	}
}
