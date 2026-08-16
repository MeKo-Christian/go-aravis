package tests

// Contract tests for the P6 error rule: only a GError may decide that an Aravis
// call failed.
//
// Every accessor below wraps a C function with no GError out-parameter at all,
// or one whose GError it already binds correctly. None of them can fail, so
// their error return must be nil — which is exactly what their documentation now
// promises. Each used to bind cgo's two-result form instead, whose second value
// is errno and not a failure report.
//
// What this file does *not* do is reproduce the defect. errno is only set by
// syscalls, and the Fake backend performs none: the whole point of it is that
// nothing leaves the process. Reproducing it needs a C call that fails a syscall
// internally and succeeds anyway, which is what internal/cerrno provides and
// internal/cerrno/cerrno_test.go asserts. The structural guard against the form
// coming back is cgo_form_test.go in the root package.

import (
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// TestAccessorsWithoutErrorChannelReturnNilError table-drives every accessor the
// errno fix touched.
//
// The pop methods are deliberately absent: Stream.PopBuffer blocks forever
// without an acquiring stream. They are covered through the Fake acquisition
// path in fake_test.go and integration_test.go instead, both of which now treat
// a non-nil error from a pop as implying a zero Buffer.
func TestAccessorsWithoutErrorChannelReturnNilError(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	device := requireFakeDevice(t)

	buffer, err := aravis.NewBuffer(1024)
	if err != nil {
		t.Fatalf("NewBuffer(1024) returned error: %v", err)
	}

	dest := make([]byte, 1024)

	tests := []struct {
		name string
		call func() error
	}{
		// interface.go — none of the four C functions takes a GError.
		{"GetNumInterface", func() error { _, err := aravis.GetNumInterface(); return err }},
		{"GetInterfaceId", func() error { _, err := aravis.GetInterfaceId(0); return err }},
		{"GetNumDevices", func() error { _, err := aravis.GetNumDevices(); return err }},
		{"GetDeviceId", func() error { _, err := aravis.GetDeviceId(0); return err }},

		// buffer.go — likewise, and NewBuffer must no longer drop a buffer it
		// successfully allocated.
		{"NewBuffer", func() error {
			b, err := aravis.NewBuffer(1024)
			if err == nil && b.IsNil() {
				t.Error("NewBuffer returned a nil buffer with a nil error")
			}

			return err
		}},
		{"GetData", func() error { _, err := buffer.GetData(); return err }},
		{"GetDataUnsafe", func() error { _, _, err := buffer.GetDataUnsafe(); return err }},
		{"GetDataSlice", func() error { _, err := buffer.GetDataSlice(); return err }},
		{"GetDataInto", func() error { _, err := buffer.GetDataInto(dest); return err }},
		{"GetStatus", func() error { _, err := buffer.GetStatus(); return err }},
		{"GetNumParts", func() error { _, err := buffer.GetNumParts(); return err }},
		{"GetPartData", func() error { _, err := buffer.GetPartData(0); return err }},

		// camera.go — these do have a GError, and it stays unset here.
		{"Camera.GetDeviceId", func() error { _, err := camera.GetDeviceId(); return err }},
		{"Camera.GetDeviceSerialNumber", func() error { _, err := camera.GetDeviceSerialNumber(); return err }},

		// performance.go — the *Fast getters bound the GError correctly but
		// returned errno on the success path.
		{"Camera.GetWidthFast", func() error { _, err := camera.GetWidthFast(); return err }},
		{"Camera.GetHeightFast", func() error { _, err := camera.GetHeightFast(); return err }},
		{"Device.GetStringFeatureValueFast", func() error {
			_, err := device.GetStringFeatureValueFast("DeviceVendorName")

			return err
		}},
		{"Device.GetIntegerFeatureValueFast", func() error {
			_, err := device.GetIntegerFeatureValueFast("Width")

			return err
		}},
		{"Device.GetFloatFeatureValueFast", func() error {
			_, err := device.GetFloatFeatureValueFast("ExposureTimeAbs")

			return err
		}},

		// device.go — the generic getters now bind a real GError, which stays
		// unset for a feature that exists with the requested type.
		{"Device.GetStringFeatureValue", func() error {
			_, err := device.GetStringFeatureValue("DeviceVendorName")

			return err
		}},
		{"Device.GetIntegerFeatureValue", func() error {
			_, err := device.GetIntegerFeatureValue("Width")

			return err
		}},
		{"Device.GetFloatFeatureValue", func() error {
			_, err := device.GetFloatFeatureValue("ExposureTimeAbs")

			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err != nil {
				t.Fatalf("%s returned error %v on a call that succeeded; "+
					"only a GError may decide that a call failed", tt.name, err)
			}
		})
	}
}
