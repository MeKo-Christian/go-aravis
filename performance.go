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

// Fast versions of common camera operations using cached strings
// These eliminate C.CString allocations for maximum performance

func (c *Camera) GetWidthFast() (int, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("Width")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_camera_get_integer(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}
	return int(cvalue), err
}

func (c *Camera) GetHeightFast() (int, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("Height")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_camera_get_integer(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}
	return int(cvalue), err
}

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

func (c *Camera) GetExposureTimeFast() (float64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("ExposureTime")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_camera_get_float(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0.0, err
	}
	return float64(cvalue), err
}

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

func (c *Camera) GetGainFast() (float64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString("Gain")
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_camera_get_float(c.camera, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0.0, err
	}
	return float64(cvalue), err
}

// Fast device feature access using cached strings
func (d *Device) GetStringFeatureValueFast(feature string) (string, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_device_get_string_feature_value(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return "", err
	}
	return C.GoString(cvalue), err
}

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

func (d *Device) GetIntegerFeatureValueFast(feature string) (int64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_device_get_integer_feature_value(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}
	return int64(cvalue), err
}

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

func (d *Device) GetFloatFeatureValueFast(feature string) (float64, error) {
	var gerror *C.GError
	cfeature, mustFree := getCachedCString(feature)
	if mustFree {
		defer C.free(unsafe.Pointer(cfeature))
	}
	cvalue, err := C.arv_device_get_float_feature_value(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0.0, err
	}
	return float64(cvalue), err
}

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
