package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"
)

// GetDeviceId returns the id of the device at index in the device list, which
// can be passed to [NewCamera] or [OpenDevice]. Call [UpdateDeviceList] before
// enumerating; index must be less than the count returned by [GetNumDevices].
func GetDeviceId(index uint) (string, error) {
	s, err := C.arv_get_device_id(C.uint(index))
	return C.GoString(s), err
}

// GetInterfaceId returns the id of the interface at index, for example "GigEVision"
// or "USB3Vision". The index must be less than the count returned by
// [GetNumInterface]. The id is what [EnableInterface] and [DisableInterface]
// expect.
func GetInterfaceId(index uint) (string, error) {
	s, err := C.arv_get_interface_id(C.uint(index))
	return C.GoString(s), err
}

// DisableInterface removes the interface with the given id from the list of
// interfaces scanned by [UpdateDeviceList], so its devices are no longer
// discovered. Unknown ids are ignored.
func DisableInterface(id string) {
	cs := C.CString(id)
	C.arv_disable_interface(cs)
	C.free(unsafe.Pointer(cs))
}

// EnableInterface adds the interface with the given id back to the list of
// interfaces scanned by [UpdateDeviceList]. All supported interfaces are
// enabled by default, so this is only needed after [DisableInterface].
// Unknown ids are ignored.
func EnableInterface(id string) {
	cs := C.CString(id)
	C.arv_enable_interface(cs)
	C.free(unsafe.Pointer(cs))
}

// GetNumDevices returns the number of devices found by the last device scan.
// Call [UpdateDeviceList] first, otherwise the count reflects an empty or
// outdated list.
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

// UpdateDeviceList scans all enabled interfaces and rebuilds the list of
// available devices. It must be called before [GetNumDevices] and
// [GetDeviceId], and again whenever devices may have been connected or
// disconnected, because the list is only a snapshot of the last scan.
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

	// Aravis takes NULL, not the empty string, as the sentinel for "the first
	// available device". C.CString("") would produce a non-NULL pointer to an
	// empty id, which no device matches.
	var cs *C.char
	if id != "" {
		cs = C.CString(id)
		defer C.free(unsafe.Pointer(cs))
	}

	d.device = C.arv_open_device(cs, &gerror)
	if unsafe.Pointer(gerror) != nil {
		return Device{}, errorFromGError(gerror)
	}

	if d.device == nil {
		return Device{}, errors.New("aravis returned a null pointer")
	}
	d.owned = newCloseFlag()

	return d, nil
}

// Shutdown releases the resources held by the Aravis library: the cached
// interface instances and their device lists, and the internal discovery
// threads and sockets. Call it once when the application is done with Aravis,
// after every camera, stream, buffer, and owned device has been closed. Using
// this package afterwards restarts the library from a clean state.
func Shutdown() {
	C.arv_shutdown()
}
