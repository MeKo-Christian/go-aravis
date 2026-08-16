package tests

import (
	"math"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// The Fake camera's fixed identity and geometry. These are properties of
// Aravis's built-in backend, not of this binding, which is what makes them
// usable as expected values.
const (
	fakeVendor = "Aravis"
	fakeModel  = "Fake"
	fakeSerial = "1"

	fakeWidth  = 512
	fakeHeight = 512
	// The sensor is larger than the default region: Fake reports a 2048x2048
	// sensor and streams a 512x512 window out of it.
	fakeSensorWidth  = 2048
	fakeSensorHeight = 2048
	// Mono8, so one byte per pixel.
	fakePayloadSize = fakeWidth * fakeHeight

	fakeFrameRate    = 25.0
	fakeExposureTime = 10000.0
)

// TestNewCameraRejectsUnknownDevice covers the failure path. The old version
// logged both outcomes, so it passed whether NewCamera returned an error or
// not, and it never checked the camera it got back.
//
// The error text comes from GLib, so only its presence is asserted.
func TestNewCameraRejectsUnknownDevice(t *testing.T) {
	camera, err := aravis.NewCamera("nonexistent-device-12345")
	if err == nil {
		t.Fatal("NewCamera(nonexistent) returned a nil error; want a failure")
	}

	if !camera.IsNil() {
		t.Error("NewCamera(nonexistent) returned a non-nil camera alongside an error")
	}
}

// TestNewCameraFirstAvailable exercises the NULL sentinel: since P1 an empty id
// means "first available device" rather than a device literally named "".
//
// This mirrors TestOpenDeviceFirstAvailable, and skips for the same reason:
// Aravis's Fake backend does not implement the first-device lookup, so
// arv_camera_new(NULL) reports "device not found" even while the interface
// enumerates Fake_1. Asserting the error instead would pin a backend
// limitation as if it were this binding's contract.
func TestNewCameraFirstAvailable(t *testing.T) {
	camera, err := aravis.NewCamera("")
	if err != nil {
		t.Skipf("NewCamera(\"\") = %v; no backend here implements the first-device lookup", err)
	}

	defer camera.Close()

	if model, err := camera.GetModelName(); err != nil || model != fakeModel {
		t.Errorf("GetModelName() = %q, %v; want %q, nil", model, err, fakeModel)
	}
}

// TestFakeCameraIdentity asserts the strings the identity accessors return.
// Exact values are right here: the backend is fixed, and an empty or garbled
// string is exactly how the C.GoString wrappers fail.
func TestFakeCameraIdentity(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	tests := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"GetVendorName", camera.GetVendorName, fakeVendor},
		{"GetModelName", camera.GetModelName, fakeModel},
		{"GetDeviceSerialNumber", camera.GetDeviceSerialNumber, fakeSerial},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if err != nil {
				t.Fatalf("%s() returned error: %v", tt.name, err)
			}

			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestFakeCameraGeometry is the regression test for the P0 out-param bug: these
// accessors declared Go int locals (8 bytes) and handed Aravis a *C.gint, which
// writes 4, so the upper half kept whatever was on the stack. On a
// little-endian machine the low half still read correctly, which is why the old
// log-only version never noticed; a wrong value here is now a failure.
func TestFakeCameraGeometry(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	width, err := camera.GetWidth()
	if err != nil {
		t.Fatalf("GetWidth() returned error: %v", err)
	}

	if width != fakeWidth {
		t.Errorf("GetWidth() = %d, want %d", width, fakeWidth)
	}

	height, err := camera.GetHeight()
	if err != nil {
		t.Fatalf("GetHeight() returned error: %v", err)
	}

	if height != fakeHeight {
		t.Errorf("GetHeight() = %d, want %d", height, fakeHeight)
	}

	sensorWidth, sensorHeight, err := camera.GetSensorSize()
	if err != nil {
		t.Fatalf("GetSensorSize() returned error: %v", err)
	}

	if sensorWidth != fakeSensorWidth || sensorHeight != fakeSensorHeight {
		t.Errorf("GetSensorSize() = %dx%d, want %dx%d",
			sensorWidth, sensorHeight, fakeSensorWidth, fakeSensorHeight)
	}

	x, y, regionWidth, regionHeight, err := camera.GetRegion()
	if err != nil {
		t.Fatalf("GetRegion() returned error: %v", err)
	}

	if x != 0 || y != 0 || regionWidth != fakeWidth || regionHeight != fakeHeight {
		t.Errorf("GetRegion() = (%d,%d,%d,%d), want (0,0,%d,%d)",
			x, y, regionWidth, regionHeight, fakeWidth, fakeHeight)
	}

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Fatalf("GetPayloadSize() returned error: %v", err)
	}

	if payloadSize != fakePayloadSize {
		t.Errorf("GetPayloadSize() = %d, want %d (Mono8 over %dx%d)",
			payloadSize, fakePayloadSize, fakeWidth, fakeHeight)
	}
}

// TestFastAccessorsMatchStandard pins the contract the *Fast methods actually
// have: they are a shortcut to the same value, not a different reading. The old
// version logged "succeeded" or "may not be supported" and checked neither.
func TestFastAccessorsMatchStandard(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	intPairs := []struct {
		name               string
		standard, fastCall func() (int, error)
	}{
		{"Width", camera.GetWidth, camera.GetWidthFast},
		{"Height", camera.GetHeight, camera.GetHeightFast},
	}

	for _, tt := range intPairs {
		t.Run(tt.name, func(t *testing.T) {
			want, err := tt.standard()
			if err != nil {
				t.Fatalf("Get%s() returned error: %v", tt.name, err)
			}

			got, err := tt.fastCall()
			if err != nil {
				t.Fatalf("Get%sFast() returned error: %v", tt.name, err)
			}

			if got != want {
				t.Errorf("Get%sFast() = %d, Get%s() = %d; the fast path must read the same value",
					tt.name, got, tt.name, want)
			}
		})
	}

	// The Fast float accessors address the fixed "ExposureTime" and "Gain"
	// GenICam nodes (performance.go), while arv_camera_get_exposure_time also
	// accepts "ExposureTimeAbs". Aravis's Fake camera implements only the
	// latter, so these report "feature not found" against it. That is a
	// property of the Fake device description, not of this binding, so they are
	// probed and skipped rather than asserting Fake's feature set.
	floatPairs := []struct {
		name               string
		standard, fastCall func() (float64, error)
	}{
		{"ExposureTime", camera.GetExposureTime, camera.GetExposureTimeFast},
		{"Gain", camera.GetGain, camera.GetGainFast},
	}

	for _, tt := range floatPairs {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fastCall()
			if err != nil {
				t.Skipf("Get%sFast() = %v; this camera exposes no matching GenICam node", tt.name, err)
			}

			want, err := tt.standard()
			if err != nil {
				t.Fatalf("Get%s() returned error: %v", tt.name, err)
			}

			if got != want {
				t.Errorf("Get%sFast() = %v, Get%s() = %v", tt.name, got, tt.name, want)
			}
		})
	}
}

// TestCameraSettingsRoundTrip asserts that what was set is what comes back. The
// old version logged "setting failed (may not be supported)" on every error
// path and never compared the value it read against the one it wrote.
func TestCameraSettingsRoundTrip(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	t.Run("acquisition mode", func(t *testing.T) {
		if err := camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS); err != nil {
			t.Errorf("SetAcquisitionMode(CONTINUOUS) returned error: %v", err)
		}
	})

	t.Run("frame rate", func(t *testing.T) {
		original, err := camera.GetFrameRate()
		if err != nil {
			t.Fatalf("GetFrameRate() returned error: %v", err)
		}

		if original != fakeFrameRate {
			t.Errorf("GetFrameRate() = %v, want the Fake default of %v", original, fakeFrameRate)
		}

		t.Cleanup(func() { _ = camera.SetFrameRate(original) })

		const want = 10.0

		if err := camera.SetFrameRate(want); err != nil {
			t.Fatalf("SetFrameRate(%v) returned error: %v", want, err)
		}

		got, err := camera.GetFrameRate()
		if err != nil {
			t.Fatalf("GetFrameRate() returned error: %v", err)
		}

		if math.Abs(got-want) > 0.001 {
			t.Errorf("GetFrameRate() = %v after SetFrameRate(%v)", got, want)
		}
	})

	t.Run("exposure time", func(t *testing.T) {
		original, err := camera.GetExposureTime()
		if err != nil {
			t.Fatalf("GetExposureTime() returned error: %v", err)
		}

		if original != fakeExposureTime {
			t.Errorf("GetExposureTime() = %v, want the Fake default of %v", original, fakeExposureTime)
		}

		t.Cleanup(func() { _ = camera.SetExposureTime(original) })

		want := original * 2

		if err := camera.SetExposureTime(want); err != nil {
			t.Fatalf("SetExposureTime(%v) returned error: %v", want, err)
		}

		got, err := camera.GetExposureTime()
		if err != nil {
			t.Fatalf("GetExposureTime() returned error: %v", err)
		}

		if math.Abs(got-want) > 0.001 {
			t.Errorf("GetExposureTime() = %v after SetExposureTime(%v)", got, want)
		}
	})
}

// TestCameraStreamCreation checks that CreateStream hands back a live stream.
// The old version logged "Stream created successfully" and then assigned
// ThreadPriority twice, logging each assignment; a struct field assignment has
// no contract of its own. The field is exercised here for what it actually
// does, which is to be read by the next CreateStream call.
func TestCameraStreamCreation(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	defer stream.Close()

	if stream.IsNil() {
		t.Fatal("CreateStream() returned a nil stream")
	}

	if stream.IsClosed() {
		t.Error("CreateStream() returned an already-closed stream")
	}

	// Realtime priority is deliberately not exercised: it needs privileges CI
	// does not have.
	camera.ThreadPriority = aravis.ThreadPriorityHigh

	highPriority, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() with ThreadPriorityHigh returned error: %v", err)
	}

	defer highPriority.Close()

	if highPriority.IsNil() {
		t.Error("CreateStream() with ThreadPriorityHigh returned a nil stream")
	}
}

// The Fake camera's fixed bounds, read from its GenICam description. Like the
// identity constants above, these are properties of the backend.
const (
	fakeFrameRateMin = 0.1
	fakeFrameRateMax = 1000.0

	fakeGainMin = 0.0
	fakeGainMax = 10.0

	fakeExposureTimeMin = 10.0
	fakeExposureTimeMax = 10000000.0
)

// TestCameraBounds covers the three bounds accessors, which had no test at all
// while GetFrameRateBounds and GetGainBounds wrote their C out-parameters
// through a *float64 reinterpreted as a *C.double. That only happened to work
// because both types are 8 bytes on this platform; the values now come back
// through C.double locals.
func TestCameraBounds(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	tests := []struct {
		name     string
		call     func() (float64, float64, error)
		min, max float64
	}{
		{"FrameRate", camera.GetFrameRateBounds, fakeFrameRateMin, fakeFrameRateMax},
		{"Gain", camera.GetGainBounds, fakeGainMin, fakeGainMax},
		{"ExposureTime", camera.GetExposureTimeBounds, fakeExposureTimeMin, fakeExposureTimeMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax, err := tt.call()
			if err != nil {
				t.Fatalf("Get%sBounds() returned error: %v", tt.name, err)
			}

			if gotMin > gotMax {
				t.Errorf("Get%sBounds() = (%v, %v); the minimum must not exceed the maximum",
					tt.name, gotMin, gotMax)
			}

			// Exact equality: a truncated or byte-shuffled out-parameter would
			// not land on the documented value by accident.
			if gotMin != tt.min || gotMax != tt.max {
				t.Errorf("Get%sBounds() = (%v, %v), want the Fake values (%v, %v)",
					tt.name, gotMin, gotMax, tt.min, tt.max)
			}
		})
	}
}

// TestCameraCurrentValuesWithinBounds ties the bounds to the values the camera
// actually reports, so a bounds accessor cannot pass by returning a plausible
// but unrelated pair.
func TestCameraCurrentValuesWithinBounds(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	tests := []struct {
		name   string
		value  func() (float64, error)
		bounds func() (float64, float64, error)
	}{
		{"FrameRate", camera.GetFrameRate, camera.GetFrameRateBounds},
		{"Gain", camera.GetGain, camera.GetGainBounds},
		{"ExposureTime", camera.GetExposureTime, camera.GetExposureTimeBounds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.value()
			if err != nil {
				t.Fatalf("Get%s() returned error: %v", tt.name, err)
			}

			minVal, maxVal, err := tt.bounds()
			if err != nil {
				t.Fatalf("Get%sBounds() returned error: %v", tt.name, err)
			}

			if value < minVal || value > maxVal {
				t.Errorf("Get%s() = %v, outside the reported bounds [%v, %v]",
					tt.name, value, minVal, maxVal)
			}
		})
	}
}

// TestGainAutoRoundTrip covers the new GetGainAuto. SetGainAuto had no getter,
// so nothing could observe whether a mode was applied. Fake implements the
// GainAuto node, so the round trip is assertable here.
func TestGainAutoRoundTrip(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	original, err := camera.GetGainAuto()
	if err != nil {
		t.Fatalf("GetGainAuto() returned error: %v", err)
	}

	if original != aravis.AUTO_OFF {
		t.Errorf("GetGainAuto() = %d, want the Fake default AUTO_OFF (%d)", original, aravis.AUTO_OFF)
	}

	// Restore the mode even when an assertion below calls Fatalf, which unwinds
	// through the defers rather than reaching the end of the loop. This has to
	// be a defer, not a t.Cleanup: cleanups run after the deferred Close, so the
	// restore would address a closed camera and trip a GLib CRITICAL. Declared
	// after the Close defer, it therefore runs before it.
	//
	// Each NewCamera mints its own ArvFakeDevice, so a leaked mode cannot reach
	// another test; this keeps the test self-contained against future edits that
	// reorder the modes or add assertions after the loop.
	defer func() { _ = camera.SetGainAuto(original) }()

	for _, mode := range []int{aravis.AUTO_CONTINUOUS, aravis.AUTO_ONCE, aravis.AUTO_OFF} {
		if err := camera.SetGainAuto(mode); err != nil {
			t.Fatalf("SetGainAuto(%d) returned error: %v", mode, err)
		}

		got, err := camera.GetGainAuto()
		if err != nil {
			t.Fatalf("GetGainAuto() returned error: %v", err)
		}

		if got != mode {
			t.Errorf("GetGainAuto() = %d after SetGainAuto(%d)", got, mode)
		}
	}
}
