package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
import "C"

import (
	"unsafe"
)

// Common GenICam feature names pre-allocated as C strings.
// This eliminates repeated C.CString allocations for frequently used features.
//
// The cache is populated once by init and never written again, so it needs no
// locking and its size is fixed at len(commonFeatures) — a few hundred bytes
// that live for the lifetime of the process. It is deliberately never freed:
// getCachedCString hands out raw *C.char pointers with no refcounting or
// lifetime tracking, so reclaiming them could not be made safe. Keeping the
// table immutable is what bounds it; see getCachedCString for how names outside
// commonFeatures are handled.
var cStringCache = make(map[string]*C.char, len(commonFeatures))

// Commonly used GenICam feature names
var commonFeatures = []string{
	"Width",
	"Height",
	"PixelFormat",
	"ExposureTime",
	"Gain",
	"TriggerMode",
	"TriggerSource",
	"AcquisitionMode",
	"AcquisitionFrameRate",
	"DeviceVendorName",
	"DeviceModelName",
	"DeviceSerialNumber",
	"DeviceVersion",
	"DeviceTemperature",
	"DeviceLinkSpeed",
	"GevSCPSPacketSize",
	"GevSCPD",
	"PayloadSize",
	"OffsetX",
	"OffsetY",
	"BinningHorizontal",
	"BinningVertical",
	"TestPattern",
	"ReverseX",
	"ReverseY",
}

// Initialize commonly used C strings at startup
func init() {
	for _, feature := range commonFeatures {
		cStringCache[feature] = C.CString(feature)
	}
}

// getCachedCString returns a C string for s, avoiding the malloc/free round
// trip for the feature names in commonFeatures.
//
// The second return value reports whether the caller owns the string and must
// free it. Only names interned at startup are cached; an arbitrary name — every
// Device.*FeatureValueFast method forwards its caller's argument here — gets a
// temporary allocation instead. Caching those permanently would let a process
// that generates or accepts feature names grow the C heap without bound, and
// the cache has no eviction path (see the note on cStringCache).
func getCachedCString(s string) (cstr *C.char, mustFree bool) {
	if cached, exists := cStringCache[s]; exists {
		return cached, false
	}

	return C.CString(s), true
}

// Fast versions of common camera operations using cached strings.
//
// The *Fast methods look their GenICam feature name up in the intern table
// built at startup instead of converting it with C.CString on every call. Only
// names listed in commonFeatures are served from that table; any other name
// still gets a temporary C allocation that is freed before the call returns.
//
// They are not always drop-in replacements. Reading a named feature is what the
// regular width and height accessors do too, so those pairs match. The exposure
// and gain variants do not: the regular Camera methods call the dedicated
// arv_camera_* accessors, which resolve whichever feature a given camera
// actually implements, whereas the Fast variants address the fixed "ExposureTime"
// and "Gain" nodes. On a camera that Aravis maps to a differently named feature
// the regular method works and the Fast one fails, so treat these as an
// optimization to reach for once a camera is known to support them, not as a
// default.
//
// The table is
// never freed by design — it is bounded by the fixed set of feature names the
// package interns, and the raw *C.char pointers it hands out have no lifetime
// tracking that would make reclaiming them safe.

// GetWidthFast returns the camera's current image width in pixels by reading
// the "Width" integer feature. It is the allocation-free variant of the regular
// width accessor: the feature name is interned, so no per-call C.CString
// conversion happens.
func (c *Camera) GetWidthFast() (int, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("Width")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_camera_get_integer(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return 0, errorFromGError(gerror)
	}
	return int(cvalue), nil
}

// GetHeightFast returns the camera's current image height in pixels by reading
// the "Height" integer feature. The feature name is interned, so the call
// avoids a per-call C.CString conversion.
func (c *Camera) GetHeightFast() (int, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("Height")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_camera_get_integer(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return 0, errorFromGError(gerror)
	}
	return int(cvalue), nil
}

// SetExposureTimeFast sets the camera's exposure time in microseconds by
// writing the "ExposureTime" float feature. The feature name is interned, so
// the call avoids a per-call C.CString conversion.
func (c *Camera) SetExposureTimeFast(exposureTimeUs float64) error {
	var gerror *C.GError
	var err error
	cfeature, mustFree := getCachedCString("ExposureTime")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	C.arv_camera_set_float(c.camera, cfeature, C.double(exposureTimeUs), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

// GetExposureTimeFast returns the camera's exposure time in microseconds by
// reading the "ExposureTime" float feature. The feature name is interned, so
// the call avoids a per-call C.CString conversion.
func (c *Camera) GetExposureTimeFast() (float64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("ExposureTime")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_camera_get_float(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return 0.0, errorFromGError(gerror)
	}
	return float64(cvalue), nil
}

// SetGainFast sets the camera's gain by writing the "Gain" float feature. The
// unit is camera specific. The feature name is interned, so the call avoids a
// per-call C.CString conversion.
func (c *Camera) SetGainFast(gain float64) error {
	var gerror *C.GError
	var err error
	cfeature, mustFree := getCachedCString("Gain")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	C.arv_camera_set_float(c.camera, cfeature, C.double(gain), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

// GetGainFast returns the camera's gain by reading the "Gain" float feature.
// The unit is camera specific. The feature name is interned, so the call avoids
// a per-call C.CString conversion.
func (c *Camera) GetGainFast() (float64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("Gain")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_camera_get_float(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return 0.0, errorFromGError(gerror)
	}
	return float64(cvalue), nil
}

// Fast device feature access using cached strings.
//
// The Device *Fast methods take the feature name from the caller and look it up
// in the intern table instead of converting it with C.CString on every call.
// Only names listed in commonFeatures are found there; any other name falls
// back to a temporary allocation that is freed before the call returns, so
// passing arbitrary names is correct but gains nothing. The table is fixed at
// startup and never freed by design.

// GetStringFeatureValueFast returns the value of the named GenICam string
// feature. The feature name avoids a C.CString conversion when it is one of the
// interned common feature names; other names are converted per call.
func (d *Device) GetStringFeatureValueFast(feature string) (string, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_device_get_string_feature_value(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return "", errorFromGError(gerror)
	}
	return C.GoString(cvalue), nil
}

// SetStringFeatureValueFast sets the named GenICam string feature to value.
// Only the feature name can come from the intern table; value varies per call
// and is always converted and freed.
func (d *Device) SetStringFeatureValueFast(feature, value string) error {
	var gerror *C.GError
	var err error

	// Only cache the feature name, not the value (which varies)
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))

	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	C.arv_device_set_string_feature_value(d.device, cfeature, cvalue, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

// GetIntegerFeatureValueFast returns the value of the named GenICam integer
// feature. The feature name avoids a C.CString conversion when it is one of the
// interned common feature names; other names are converted per call.
func (d *Device) GetIntegerFeatureValueFast(feature string) (int64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_device_get_integer_feature_value(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return 0, errorFromGError(gerror)
	}
	return int64(cvalue), nil
}

// SetIntegerFeatureValueFast sets the named GenICam integer feature to value.
// The feature name avoids a C.CString conversion when it is one of the interned
// common feature names; other names are converted per call.
func (d *Device) SetIntegerFeatureValueFast(feature string, value int64) error {
	var gerror *C.GError
	var err error
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	C.arv_device_set_integer_feature_value(d.device, cfeature, C.long(value), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

// GetFloatFeatureValueFast returns the value of the named GenICam float
// feature. The feature name avoids a C.CString conversion when it is one of the
// interned common feature names; other names are converted per call.
func (d *Device) GetFloatFeatureValueFast(feature string) (float64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue := C.arv_device_get_float_feature_value(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return 0.0, errorFromGError(gerror)
	}
	return float64(cvalue), nil
}

// SetFloatFeatureValueFast sets the named GenICam float feature to value. The
// feature name avoids a C.CString conversion when it is one of the interned
// common feature names; other names are converted per call.
func (d *Device) SetFloatFeatureValueFast(feature string, value float64) error {
	var gerror *C.GError
	var err error
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	C.arv_device_set_float_feature_value(d.device, cfeature, C.double(value), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}
