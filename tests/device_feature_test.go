package tests

// Tests for the generic GenICam feature getters on Device.
//
// Until P6 these three passed a nil GError out-parameter to Aravis, so a
// missing feature or a wrong-typed read returned the zero value and a nil
// error: every GenICam failure on the read side was silently swallowed. The
// error tests below are the ones that fail against that older behaviour.

import (
	"errors"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// missingFeature is a GenICam node name no camera can plausibly implement.
const missingFeature = "NoSuchFeatureAtAll"

// requireFakeDevice returns the Fake backend's device. The caller owns it,
// hence the Cleanup: OpenDevice transfers ownership, unlike Camera.GetDevice.
func requireFakeDevice(tb testing.TB) aravis.Device {
	tb.Helper()

	device, err := aravis.OpenDevice(fakeDeviceID)
	if err != nil {
		tb.Fatalf("OpenDevice(%s) returned error: %v", fakeDeviceID, err)
	}

	if device.IsNil() {
		tb.Fatalf("OpenDevice(%s) returned a nil device", fakeDeviceID)
	}

	tb.Cleanup(device.Close)

	return device
}

// requireAravisCode asserts that err is an *AravisError carrying want. It is
// what distinguishes a GenICam failure that actually reached the caller from
// any other non-nil error.
func requireAravisCode(tb testing.TB, err error, want int, context string) {
	tb.Helper()

	if err == nil {
		tb.Fatalf("%s returned a nil error; the GenICam failure was swallowed", context)
	}

	var aerr *aravis.AravisError
	if !errors.As(err, &aerr) {
		tb.Fatalf("%s returned %[2]T (%[2]v), want an *aravis.AravisError", context, err)
	}

	if aerr.Code != want {
		tb.Fatalf("%s returned code %d (%v), want %d", context, aerr.Code, err, want)
	}
}

// TestGetFeatureValueMissingFeature is the headline regression: reading a
// feature that does not exist must report DEVICE_ERROR_FEATURE_NOT_FOUND
// instead of returning the zero value and nil.
func TestGetFeatureValueMissingFeature(t *testing.T) {
	device := requireFakeDevice(t)

	t.Run("string", func(t *testing.T) {
		value, err := device.GetStringFeatureValue(missingFeature)
		requireAravisCode(t, err, aravis.DEVICE_ERROR_FEATURE_NOT_FOUND, "GetStringFeatureValue("+missingFeature+")")

		if value != "" {
			t.Errorf("GetStringFeatureValue(%s) = %q on failure, want the empty string", missingFeature, value)
		}
	})

	t.Run("integer", func(t *testing.T) {
		value, err := device.GetIntegerFeatureValue(missingFeature)
		requireAravisCode(t, err, aravis.DEVICE_ERROR_FEATURE_NOT_FOUND, "GetIntegerFeatureValue("+missingFeature+")")

		if value != 0 {
			t.Errorf("GetIntegerFeatureValue(%s) = %d on failure, want 0", missingFeature, value)
		}
	})

	t.Run("float", func(t *testing.T) {
		value, err := device.GetFloatFeatureValue(missingFeature)
		requireAravisCode(t, err, aravis.DEVICE_ERROR_FEATURE_NOT_FOUND, "GetFloatFeatureValue("+missingFeature+")")

		if value != 0 {
			t.Errorf("GetFloatFeatureValue(%s) = %v on failure, want 0", missingFeature, value)
		}
	})
}

// TestGetFeatureValueWrongType covers the other half of the swallowed errors:
// the node exists but is not of the requested type. Aravis reports this as
// DEVICE_ERROR_WRONG_FEATURE ("Not a ArvGcInteger" and friends).
func TestGetFeatureValueWrongType(t *testing.T) {
	device := requireFakeDevice(t)

	t.Run("integer read of a string node", func(t *testing.T) {
		_, err := device.GetIntegerFeatureValue("DeviceVendorName")
		requireAravisCode(t, err, aravis.DEVICE_ERROR_WRONG_FEATURE, "GetIntegerFeatureValue(DeviceVendorName)")
	})

	t.Run("string read of an integer node", func(t *testing.T) {
		_, err := device.GetStringFeatureValue("Width")
		requireAravisCode(t, err, aravis.DEVICE_ERROR_WRONG_FEATURE, "GetStringFeatureValue(Width)")
	})

	t.Run("float read of an integer node", func(t *testing.T) {
		_, err := device.GetFloatFeatureValue("Width")
		requireAravisCode(t, err, aravis.DEVICE_ERROR_WRONG_FEATURE, "GetFloatFeatureValue(Width)")
	})
}

// TestGetFeatureValueHappyPath is the positive control for the tests above: the
// getters must not have become uniformly failing. The feature names are the
// ones the Fake camera's GenICam description actually provides — it has an
// ExposureTimeAbs float node but no ExposureTime and no Gain.
func TestGetFeatureValueHappyPath(t *testing.T) {
	device := requireFakeDevice(t)

	vendor, err := device.GetStringFeatureValue("DeviceVendorName")
	if err != nil {
		t.Fatalf("GetStringFeatureValue(DeviceVendorName) returned error: %v", err)
	}

	if vendor == "" {
		t.Error("GetStringFeatureValue(DeviceVendorName) returned the empty string")
	}

	width, err := device.GetIntegerFeatureValue("Width")
	if err != nil {
		t.Fatalf("GetIntegerFeatureValue(Width) returned error: %v", err)
	}

	if width <= 0 {
		t.Errorf("GetIntegerFeatureValue(Width) = %d, want a positive width", width)
	}

	exposure, err := device.GetFloatFeatureValue("ExposureTimeAbs")
	if err != nil {
		t.Fatalf("GetFloatFeatureValue(ExposureTimeAbs) returned error: %v", err)
	}

	if exposure <= 0 {
		t.Errorf("GetFloatFeatureValue(ExposureTimeAbs) = %v, want a positive exposure time", exposure)
	}
}

// TestFeatureGetterParityWithFast pins the standard getters and their *Fast
// counterparts to the same contract on both paths. Before P6 the two disagreed
// in both directions: the standard getter never reported a GenICam failure, and
// the Fast one returned errno on success.
func TestFeatureGetterParityWithFast(t *testing.T) {
	device := requireFakeDevice(t)

	t.Run("error path", func(t *testing.T) {
		slowValue, slowErr := device.GetStringFeatureValue(missingFeature)
		fastValue, fastErr := device.GetStringFeatureValueFast(missingFeature)

		requireAravisCode(t, slowErr, aravis.DEVICE_ERROR_FEATURE_NOT_FOUND, "GetStringFeatureValue("+missingFeature+")")
		requireAravisCode(t, fastErr, aravis.DEVICE_ERROR_FEATURE_NOT_FOUND, "GetStringFeatureValueFast("+missingFeature+")")

		if slowValue != fastValue {
			t.Errorf("GetStringFeatureValue = %q but GetStringFeatureValueFast = %q", slowValue, fastValue)
		}
	})

	t.Run("happy path", func(t *testing.T) {
		slowValue, slowErr := device.GetStringFeatureValue("DeviceVendorName")
		if slowErr != nil {
			t.Fatalf("GetStringFeatureValue(DeviceVendorName) returned error: %v", slowErr)
		}

		fastValue, fastErr := device.GetStringFeatureValueFast("DeviceVendorName")
		if fastErr != nil {
			t.Fatalf("GetStringFeatureValueFast(DeviceVendorName) returned error: %v", fastErr)
		}

		if slowValue != fastValue {
			t.Errorf("GetStringFeatureValue = %q but GetStringFeatureValueFast = %q", slowValue, fastValue)
		}
	})

	t.Run("integer", func(t *testing.T) {
		slowValue, slowErr := device.GetIntegerFeatureValue("Width")
		fastValue, fastErr := device.GetIntegerFeatureValueFast("Width")

		if slowErr != nil || fastErr != nil {
			t.Fatalf("GetIntegerFeatureValue(Width) = %v, GetIntegerFeatureValueFast(Width) = %v; want both nil",
				slowErr, fastErr)
		}

		if slowValue != fastValue {
			t.Errorf("GetIntegerFeatureValue = %d but GetIntegerFeatureValueFast = %d", slowValue, fastValue)
		}
	})

	t.Run("float", func(t *testing.T) {
		slowValue, slowErr := device.GetFloatFeatureValue("ExposureTimeAbs")
		fastValue, fastErr := device.GetFloatFeatureValueFast("ExposureTimeAbs")

		if slowErr != nil || fastErr != nil {
			t.Fatalf("GetFloatFeatureValue(ExposureTimeAbs) = %v, GetFloatFeatureValueFast(ExposureTimeAbs) = %v; "+
				"want both nil", slowErr, fastErr)
		}

		if slowValue != fastValue {
			t.Errorf("GetFloatFeatureValue = %v but GetFloatFeatureValueFast = %v", slowValue, fastValue)
		}
	})
}
