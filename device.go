package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
/*
void arv_set_node_feature_value(ArvDevice *device, char *name, char *value, GError **error) {
	ArvGcNode *feature;
	feature = arv_device_get_feature (device, name);
	if (feature == NULL) {
		g_set_error (error, ARV_DEVICE_ERROR, ARV_DEVICE_ERROR_FEATURE_NOT_FOUND,
			"feature '%s' not found", name);
		return;
	}
	arv_gc_feature_node_set_value_from_string (ARV_GC_FEATURE_NODE (feature), value, error);
}

gboolean arv_device_take_control(ArvDevice *device, GError **error) {
	return arv_gv_device_take_control(ARV_GV_DEVICE(device), error);
}

gboolean arv_device_leave_control(ArvDevice *device, GError **error) {
	return arv_gv_device_leave_control(ARV_GV_DEVICE(device), error);
}

*/
import "C"

import (
	"errors"
	"unsafe"
)

// Device error codes, mirroring the ArvDeviceError enumeration of Aravis.
// They correspond to the Code field of [AravisError], the error type this
// package returns for failures reported by Aravis, so a caller can compare
// against them to tell one failure apart from another.
const (
	// DEVICE_ERROR_WRONG_FEATURE indicates that a feature was accessed with
	// the wrong type, for example an integer feature read as a float.
	DEVICE_ERROR_WRONG_FEATURE = C.ARV_DEVICE_ERROR_WRONG_FEATURE
	// DEVICE_ERROR_FEATURE_NOT_FOUND indicates that the GenICam feature node
	// with the requested name does not exist on the device.
	DEVICE_ERROR_FEATURE_NOT_FOUND = C.ARV_DEVICE_ERROR_FEATURE_NOT_FOUND
	// DEVICE_ERROR_NOT_CONNECTED indicates that the device is no longer
	// connected.
	DEVICE_ERROR_NOT_CONNECTED = C.ARV_DEVICE_ERROR_NOT_CONNECTED
	// DEVICE_ERROR_PROTOCOL_ERROR indicates a violation of the camera control
	// protocol, such as an unexpected or malformed answer from the device.
	DEVICE_ERROR_PROTOCOL_ERROR = C.ARV_DEVICE_ERROR_PROTOCOL_ERROR
	// DEVICE_ERROR_TRANSFER_ERROR indicates that a memory or register transfer
	// to or from the device failed.
	DEVICE_ERROR_TRANSFER_ERROR = C.ARV_DEVICE_ERROR_TRANSFER_ERROR
	// DEVICE_ERROR_TIMEOUT indicates that the device did not answer in time.
	DEVICE_ERROR_TIMEOUT = C.ARV_DEVICE_ERROR_TIMEOUT
	// DEVICE_ERROR_NOT_FOUND indicates that the requested device does not
	// exist.
	DEVICE_ERROR_NOT_FOUND = C.ARV_DEVICE_ERROR_NOT_FOUND
	// DEVICE_ERROR_INVALID_PARAMETER indicates that a parameter passed to the
	// device was not valid.
	DEVICE_ERROR_INVALID_PARAMETER = C.ARV_DEVICE_ERROR_INVALID_PARAMETER
	// DEVICE_ERROR_GENICAM_NOT_FOUND indicates that the GenICam description of
	// the device could not be retrieved.
	DEVICE_ERROR_GENICAM_NOT_FOUND = C.ARV_DEVICE_ERROR_GENICAM_NOT_FOUND
	// DEVICE_ERROR_NO_STREAM_CHANNEL indicates that the device offers no
	// stream channel, so no stream can be created.
	DEVICE_ERROR_NO_STREAM_CHANNEL = C.ARV_DEVICE_ERROR_NO_STREAM_CHANNEL
	// DEVICE_ERROR_NOT_CONTROLLER indicates that the operation requires
	// control access, which this application does not hold. See
	// [Device.TakeControl].
	DEVICE_ERROR_NOT_CONTROLLER = C.ARV_DEVICE_ERROR_NOT_CONTROLLER
	// DEVICE_ERROR_UNKNOWN indicates a failure Aravis could not classify.
	DEVICE_ERROR_UNKNOWN = C.ARV_DEVICE_ERROR_UNKNOWN
)

// Common GigE Vision bootstrap register addresses, for use with
// [Device.ReadRegister] and [Device.WriteRegister]. These are meant for
// advanced users: prefer GenICam feature access, and note that writing to
// bootstrap registers can misconfigure or damage a camera. The addresses only
// apply to GigE Vision devices.
const (
	// GVBS_VERSION_REGISTER holds the GigE Vision protocol version supported
	// by the device.
	GVBS_VERSION_REGISTER = 0x0000
	// GVBS_DEVICE_MODE_REGISTER holds the device mode, including endianness
	// and character-set information.
	GVBS_DEVICE_MODE_REGISTER = 0x0004
	// GVBS_DEVICE_MAC_HIGH_REGISTER holds the upper 16 bits of the MAC address
	// of the first network interface.
	GVBS_DEVICE_MAC_HIGH_REGISTER = 0x0008
	// GVBS_DEVICE_MAC_LOW_REGISTER holds the lower 32 bits of the MAC address
	// of the first network interface.
	GVBS_DEVICE_MAC_LOW_REGISTER = 0x000C
	// GVBS_DEVICE_IP_REGISTER holds the current IP address of the first
	// network interface.
	GVBS_DEVICE_IP_REGISTER = 0x0014
	// GVBS_DEVICE_SUBNET_REGISTER holds the current subnet mask of the first
	// network interface.
	GVBS_DEVICE_SUBNET_REGISTER = 0x0018
	// GVBS_DEVICE_GATEWAY_REGISTER holds the current default gateway of the
	// first network interface.
	GVBS_DEVICE_GATEWAY_REGISTER = 0x001C
	// GVBS_MANUFACTURER_NAME_REGISTER is the start of the manufacturer name
	// string, which is read with [Device.ReadMemory] rather than as a single
	// register.
	GVBS_MANUFACTURER_NAME_REGISTER = 0x0048
	// GVBS_MODEL_NAME_REGISTER is the start of the model name string, which is
	// read with [Device.ReadMemory] rather than as a single register.
	GVBS_MODEL_NAME_REGISTER = 0x0068
	// GVBS_DEVICE_VERSION_REGISTER is the start of the device version string,
	// which is read with [Device.ReadMemory] rather than as a single register.
	GVBS_DEVICE_VERSION_REGISTER = 0x0088
)

// Device gives low-level access to a camera: GenICam feature nodes, commands,
// and — for GigE Vision devices — raw register and memory access.
//
// Device is a value type and may be copied freely; all copies refer to the same
// underlying Aravis device. Ownership, however, depends on where the value came
// from. [OpenDevice] returns an owned device: the caller holds a reference of
// its own and must release it with [Device.Close]. [Camera.GetDevice] only
// borrows the device belonging to the camera; that value must not be closed,
// because the camera releases the device when it is closed, and it must not be
// used after the camera is closed. The owned field records which of the two a
// given value is.
type Device struct {
	device *C.struct__ArvDevice

	// owned is non-nil exactly when this Device holds a reference of its own,
	// which is the case for [OpenDevice]. [Camera.GetDevice] only borrows the
	// camera's device — the camera releases that one — and leaves this nil.
	//
	// The flag is shared by every copy of the Device, so the reference is
	// dropped once even though callers copy the value around.
	owned *closeFlag
}

// Close releases the device if this Device owns a reference to it, which is
// the case for devices obtained from [OpenDevice]. For a device borrowed from
// [Camera.GetDevice] it does nothing — that one belongs to the camera.
//
// Close is safe to call more than once, and safe to call on any copy of the
// same Device: the reference is dropped exactly once. Neither this Device nor
// any copy of it may be used afterwards.
func (d *Device) Close() {
	if d.device == nil || !d.owned.claim() {
		return
	}

	C.g_object_unref(C.gpointer(d.device))
}

// IsClosed reports whether an owned device has been released, by this value or
// by any copy of it. A borrowed device is never closed by this package.
func (d *Device) IsClosed() bool {
	return d.device == nil || d.owned.isClosed()
}

// TakeControl requests exclusive control access to a GigE Vision device and
// reports whether control was granted. Only the controlling application may
// change device settings. The device must be a GigE Vision device; calling
// this on any other device type is a programming error.
func (d *Device) TakeControl() (bool, error) {
	var gerror *C.GError
	var err error

	cbool := C.arv_device_take_control(d.device, &gerror)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(cbool), err
}

// LeaveControl releases exclusive control access to a GigE Vision device that
// was acquired with [Device.TakeControl] and reports whether the release
// succeeded. The device must be a GigE Vision device.
func (d *Device) LeaveControl() (bool, error) {
	var gerror *C.GError
	var err error

	cbool := C.arv_device_leave_control(d.device, &gerror)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(cbool), err
}

// SetStringFeatureValue sets the string GenICam feature with the given name to
// value. It returns an error if the feature does not exist, is not a string
// feature, or if the device rejects the write.
func (d *Device) SetStringFeatureValue(feature, value string) error {
	var gerror *C.GError
	var err error

	cfeature := C.CString(feature)
	cvalue := C.CString(value)
	C.arv_device_set_string_feature_value(d.device, cfeature, cvalue, &gerror)
	C.free(unsafe.Pointer(cfeature))
	C.free(unsafe.Pointer(cvalue))

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetStringFeatureValue returns the value of the string GenICam feature with
// the given name.
//
// The returned error does not report GenICam failures: this call passes a nil
// GError out-parameter to Aravis, so the error is only the errno cgo observed
// around the call. A missing feature, a wrong feature type, or a failed read
// therefore yields the empty string and a nil error rather than an error.
func (d *Device) GetStringFeatureValue(feature string) (string, error) {
	cfeature := C.CString(feature)
	cvalue, err := C.arv_device_get_string_feature_value(d.device, cfeature, nil)
	C.free(unsafe.Pointer(cfeature))
	return C.GoString(cvalue), err
}

// SetIntegerFeatureValue sets the integer GenICam feature with the given name
// to value. It returns an error if the feature does not exist, is not an
// integer feature, or if the device rejects the write.
func (d *Device) SetIntegerFeatureValue(feature string, value int64) error {
	var gerror *C.GError
	var err error

	cfeature := C.CString(feature)
	cvalue := C.long(value)
	C.arv_device_set_integer_feature_value(d.device, cfeature, cvalue, &gerror)
	C.free(unsafe.Pointer(cfeature))

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetIntegerFeatureValue returns the value of the integer GenICam feature with
// the given name.
//
// The returned error does not report GenICam failures: this call passes a nil
// GError out-parameter to Aravis, so the error is only the errno cgo observed
// around the call. A missing feature, a wrong feature type, or a failed read
// therefore yields 0 and a nil error rather than an error.
func (d *Device) GetIntegerFeatureValue(feature string) (int64, error) {
	cfeature := C.CString(feature)
	cvalue, err := C.arv_device_get_integer_feature_value(d.device, cfeature, nil)
	C.free(unsafe.Pointer(cfeature))
	return int64(cvalue), err
}

// SetFloatFeatureValue sets the float GenICam feature with the given name to
// value. It returns an error if the feature does not exist, is not a float
// feature, or if the device rejects the write.
func (d *Device) SetFloatFeatureValue(feature string, value float64) error {
	var gerror *C.GError
	var err error

	cfeature := C.CString(feature)
	cvalue := C.double(value)
	C.arv_device_set_float_feature_value(d.device, cfeature, cvalue, &gerror)
	C.free(unsafe.Pointer(cfeature))

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetFloatFeatureValue returns the value of the float GenICam feature with the
// given name.
//
// The returned error does not report GenICam failures: this call passes a nil
// GError out-parameter to Aravis, so the error is only the errno cgo observed
// around the call. A missing feature, a wrong feature type, or a failed read
// therefore yields 0 and a nil error rather than an error.
func (d *Device) GetFloatFeatureValue(feature string) (float64, error) {
	cfeature := C.CString(feature)
	cvalue, err := C.arv_device_get_float_feature_value(d.device, cfeature, nil)
	C.free(unsafe.Pointer(cfeature))
	return float64(cvalue), err
}

// SetNodeFeatureValue sets the GenICam feature node with the given name from
// its string representation, whatever the type of the node is. It returns an
// error with code [DEVICE_ERROR_FEATURE_NOT_FOUND] if no such node exists, and
// an error from the GenICam layer if the string cannot be applied.
func (d *Device) SetNodeFeatureValue(feature, value string) error {
	var gerror *C.GError
	var err error

	cfeature := C.CString(feature)
	cvalue := C.CString(value)
	C.arv_set_node_feature_value(d.device, cfeature, cvalue, &gerror)
	C.free(unsafe.Pointer(cfeature))
	C.free(unsafe.Pointer(cvalue))

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// ExecuteCommand executes the GenICam command feature with the given name. It
// returns an error if the feature does not exist, is not a command, or if the
// device rejects the execution.
func (d *Device) ExecuteCommand(feature string) error {
	var gerror *C.GError
	var err error
	cfeature := C.CString(feature)

	C.arv_device_execute_command(d.device, cfeature, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	C.free(unsafe.Pointer(cfeature))
	return err
}

// IsNil reports whether this Device wraps no underlying Aravis device, which
// is the case for the zero value. Every other method requires a non-nil device.
func (d *Device) IsNil() bool {
	return d.device == nil
}

// ReadMemory reads size bytes from the device memory starting at address and
// returns them.
//
// This is low-level access intended for advanced users. Prefer GenICam feature
// access whenever the information is available as a feature; the register map
// is device specific, so consult the camera documentation before using it. See
// the GVBS_* constants for common GigE Vision bootstrap addresses.
func (d *Device) ReadMemory(address uint64, size uint32) ([]byte, error) {
	var gerror *C.GError
	var err error

	buffer := make([]byte, size)

	success := C.arv_device_read_memory(
		d.device,
		C.guint64(address),
		C.guint32(size),
		unsafe.Pointer(&buffer[0]),
		&gerror,
	)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return nil, err
	}

	if success == 0 {
		return nil, errors.New("memory read failed")
	}

	return buffer, nil
}

// WriteMemory writes data to the device memory starting at address. It returns
// an error if data is empty or if the transfer fails.
//
// This is low-level access intended for advanced users. Writing to device
// memory can misconfigure or permanently damage a camera, so prefer GenICam
// feature access and consult the camera documentation for the register map
// before using it.
func (d *Device) WriteMemory(address uint64, data []byte) error {
	var gerror *C.GError
	var err error

	if len(data) == 0 {
		return errors.New("no data to write")
	}

	success := C.arv_device_write_memory(
		d.device,
		C.guint64(address),
		C.guint32(len(data)),
		unsafe.Pointer(&data[0]),
		&gerror,
	)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return err
	}

	if success == 0 {
		return errors.New("memory write failed")
	}

	return nil
}

// ReadRegister reads the 32 bit register at address and returns its value.
//
// This is low-level access intended for advanced users. Prefer GenICam feature
// access whenever the information is available as a feature; the register map
// is device specific, so consult the camera documentation before using it. See
// the GVBS_* constants for common GigE Vision bootstrap addresses.
func (d *Device) ReadRegister(address uint64) (uint32, error) {
	var gerror *C.GError
	var err error
	var value uint32

	success := C.arv_device_read_register(
		d.device,
		C.guint64(address),
		(*C.guint32)(unsafe.Pointer(&value)),
		&gerror,
	)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return 0, err
	}

	if success == 0 {
		return 0, errors.New("register read failed")
	}

	return value, nil
}

// WriteRegister writes value to the 32 bit register at address.
//
// This is low-level access intended for advanced users. Writing registers can
// misconfigure or permanently damage a camera, so prefer GenICam feature
// access and consult the camera documentation for the register map before
// using it.
func (d *Device) WriteRegister(address uint64, value uint32) error {
	var gerror *C.GError
	var err error

	success := C.arv_device_write_register(
		d.device,
		C.guint64(address),
		C.guint32(value),
		&gerror,
	)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return err
	}

	if success == 0 {
		return errors.New("register write failed")
	}

	return nil
}
