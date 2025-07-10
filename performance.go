package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
import "C"

import (
	"sync"
	"unsafe"
)

// Common GenICam feature names pre-allocated as C strings
// This eliminates repeated C.CString allocations for frequently used features
var (
	cStringCache      = make(map[string]*C.char)
	cStringCacheMutex sync.RWMutex
)

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

// getCachedCString returns a cached C string or creates a new one
// For high-frequency operations, this eliminates malloc/free overhead
func getCachedCString(s string) *C.char {
	cStringCacheMutex.RLock()
	if cached, exists := cStringCache[s]; exists {
		cStringCacheMutex.RUnlock()
		return cached
	}
	cStringCacheMutex.RUnlock()

	// Not in cache, create and cache it
	cStringCacheMutex.Lock()
	defer cStringCacheMutex.Unlock()

	// Double-check in case another goroutine added it
	if cached, exists := cStringCache[s]; exists {
		return cached
	}

	cached := C.CString(s)
	cStringCache[s] = cached
	return cached
}

// Fast versions of common camera operations using cached strings
// These eliminate C.CString allocations for maximum performance

func (c *Camera) GetWidthFast() (int, error) {
	var gerror *C.GError
	cvalue, err := C.arv_camera_get_integer(c.camera, getCachedCString("Width"), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}
	return int(cvalue), err
}

func (c *Camera) GetHeightFast() (int, error) {
	var gerror *C.GError
	cvalue, err := C.arv_camera_get_integer(c.camera, getCachedCString("Height"), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}
	return int(cvalue), err
}

func (c *Camera) SetExposureTimeFast(exposureTimeUs float64) error {
	var gerror *C.GError
	var err error
	C.arv_camera_set_float(c.camera, getCachedCString("ExposureTime"), C.double(exposureTimeUs), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

func (c *Camera) GetExposureTimeFast() (float64, error) {
	var gerror *C.GError
	cvalue, err := C.arv_camera_get_float(c.camera, getCachedCString("ExposureTime"), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0.0, err
	}
	return float64(cvalue), err
}

func (c *Camera) SetGainFast(gain float64) error {
	var gerror *C.GError
	var err error
	C.arv_camera_set_float(c.camera, getCachedCString("Gain"), C.double(gain), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

func (c *Camera) GetGainFast() (float64, error) {
	var gerror *C.GError
	cvalue, err := C.arv_camera_get_float(c.camera, getCachedCString("Gain"), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0.0, err
	}
	return float64(cvalue), err
}

// Fast device feature access using cached strings
func (d *Device) GetStringFeatureValueFast(feature string) (string, error) {
	var gerror *C.GError
	cvalue, err := C.arv_device_get_string_feature_value(d.device, getCachedCString(feature), &gerror)
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

	C.arv_device_set_string_feature_value(d.device, getCachedCString(feature), cvalue, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

func (d *Device) GetIntegerFeatureValueFast(feature string) (int64, error) {
	var gerror *C.GError
	cvalue, err := C.arv_device_get_integer_feature_value(d.device, getCachedCString(feature), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}
	return int64(cvalue), err
}

func (d *Device) SetIntegerFeatureValueFast(feature string, value int64) error {
	var gerror *C.GError
	var err error
	C.arv_device_set_integer_feature_value(d.device, getCachedCString(feature), C.long(value), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

func (d *Device) GetFloatFeatureValueFast(feature string) (float64, error) {
	var gerror *C.GError
	cvalue, err := C.arv_device_get_float_feature_value(d.device, getCachedCString(feature), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0.0, err
	}
	return float64(cvalue), err
}

func (d *Device) SetFloatFeatureValueFast(feature string, value float64) error {
	var gerror *C.GError
	var err error
	C.arv_device_set_float_feature_value(d.device, getCachedCString(feature), C.double(value), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

// Cleanup function for graceful shutdown (optional)
// Call this before program exit to free cached C strings
func CleanupPerformanceCache() {
	cStringCacheMutex.Lock()
	defer cStringCacheMutex.Unlock()

	for _, cstr := range cStringCache {
		C.free(unsafe.Pointer(cstr))
	}
	cStringCache = make(map[string]*C.char)
}
