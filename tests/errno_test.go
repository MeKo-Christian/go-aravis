package tests

// Contract tests for the P6 error rule: only a GError may decide that an Aravis
// call failed.
//
// Every accessor below wraps a C function whose GError it binds correctly, or
// none at all. None of them can fail here, so their error return must be nil.
// Each used to bind cgo's two-result form instead, whose second value is errno
// and not a failure report.
//
// The table is shorter than the set the errno fix touched. Ten of those
// accessors have since dropped their error return altogether — the four
// package-level id and count functions in interface.go, and Buffer.GetData,
// GetDataUnsafe, GetDataSlice, GetDataInto, GetStatus and GetNumParts. There is
// no longer an error to assert about, which is a stronger guarantee than this
// test could give: the compiler enforces it at every call site instead.
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

	tests := []struct {
		name string
		call func() error
	}{
		// buffer.go — arv_buffer_new takes no GError, and NewBuffer must no
		// longer drop a buffer it successfully allocated.
		{"NewBuffer", func() error {
			b, err := aravis.NewBuffer(1024)
			if err == nil && b.IsNil() {
				t.Error("NewBuffer returned a nil buffer with a nil error")
			}

			return err
		}},
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
