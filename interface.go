package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"
)

func GetDeviceId(index uint) (string, error) {
	s, err := C.arv_get_device_id(C.uint(index))
	return C.GoString(s), err
}

func GetInterfaceId(index uint) (string, error) {
	s, err := C.arv_get_interface_id(C.uint(index))
	return C.GoString(s), err
}

func DisableInterface(id string) {
	cs := C.CString(id)
	C.arv_disable_interface(cs)
	C.free(unsafe.Pointer(cs))
}

func EnableInterface(id string) {
	cs := C.CString(id)
	C.arv_enable_interface(cs)
	C.free(unsafe.Pointer(cs))
}

func GetNumDevices() (uint, error) {
	n, err := C.arv_get_n_devices()
	return uint(n), err
}

// GetNumInterface returns the number of available interfaces.
func GetNumInterface() (uint, error) {
	n, err := C.arv_get_n_interfaces()
	return uint(n), err
}

// GetNumInferface is a misspelling of [GetNumInterface].
//
// Deprecated: use GetNumInterface instead.
func GetNumInferface() (uint, error) {
	return GetNumInterface()
}

func UpdateDeviceList() {
	C.arv_update_device_list()
}

// OpenDevice opens the device with the given device id and returns it.
// An empty id opens the first available device.
//
// The caller owns the returned device and must release it with
// [Device.Close]. This is unlike [Camera.GetDevice], which borrows the
// camera's device and must not be closed.
func OpenDevice(id string) (Device, error) {
	var gerror *C.GError
	var d Device

	cs := C.CString(id)
	defer C.free(unsafe.Pointer(cs))

	d.device = C.arv_open_device(cs, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return Device{}, errorFromGError(gerror)
	}

	if d.device == nil {
		return Device{}, errors.New("aravis returned a null pointer")
	}
	d.owned = true

	return d, nil
}

func Shutdown() {
	C.arv_shutdown()
}
