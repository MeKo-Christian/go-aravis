package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
/*
void arv_set_node_feature_value(ArvDevice *device, char *name, char *value) {
	ArvGcNode *feature;
	feature = arv_device_get_feature (device, name);
	arv_gc_feature_node_set_value_from_string (ARV_GC_FEATURE_NODE (feature), value, NULL);
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

const (
	DEVICE_ERROR_WRONG_FEATURE     = C.ARV_DEVICE_ERROR_WRONG_FEATURE
	DEVICE_ERROR_FEATURE_NOT_FOUND = C.ARV_DEVICE_ERROR_FEATURE_NOT_FOUND
	DEVICE_ERROR_NOT_CONNECTED     = C.ARV_DEVICE_ERROR_NOT_CONNECTED
	DEVICE_ERROR_PROTOCOL_ERROR    = C.ARV_DEVICE_ERROR_PROTOCOL_ERROR
	DEVICE_ERROR_TRANSFER_ERROR    = C.ARV_DEVICE_ERROR_TRANSFER_ERROR
	DEVICE_ERROR_TIMEOUT           = C.ARV_DEVICE_ERROR_TIMEOUT
	DEVICE_ERROR_NOT_FOUND         = C.ARV_DEVICE_ERROR_NOT_FOUND
	DEVICE_ERROR_INVALID_PARAMETER = C.ARV_DEVICE_ERROR_INVALID_PARAMETER
	DEVICE_ERROR_GENICAM_NOT_FOUND = C.ARV_DEVICE_ERROR_GENICAM_NOT_FOUND
	DEVICE_ERROR_NO_STREAM_CHANNEL = C.ARV_DEVICE_ERROR_NO_STREAM_CHANNEL
	DEVICE_ERROR_NOT_CONTROLLER    = C.ARV_DEVICE_ERROR_NOT_CONTROLLER
	DEVICE_ERROR_UNKNOWN           = C.ARV_DEVICE_ERROR_UNKNOWN

	// Common GigE Vision bootstrap register addresses (for advanced users).
	GVBS_VERSION_REGISTER           = 0x0000
	GVBS_DEVICE_MODE_REGISTER       = 0x0004
	GVBS_DEVICE_MAC_HIGH_REGISTER   = 0x0008
	GVBS_DEVICE_MAC_LOW_REGISTER    = 0x000C
	GVBS_DEVICE_IP_REGISTER         = 0x0014
	GVBS_DEVICE_SUBNET_REGISTER     = 0x0018
	GVBS_DEVICE_GATEWAY_REGISTER    = 0x001C
	GVBS_MANUFACTURER_NAME_REGISTER = 0x0048
	GVBS_MODEL_NAME_REGISTER        = 0x0068
	GVBS_DEVICE_VERSION_REGISTER    = 0x0088
)

type Device struct {
	device *C.struct__ArvDevice
}

func (d *Device) TakeControl() (bool, error) {
	var gerror *C.GError
	var err error

	cbool := C.arv_device_take_control(d.device, &gerror)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(cbool), err
}

func (d *Device) LeaveControl() (bool, error) {
	var gerror *C.GError
	var err error

	cbool := C.arv_device_leave_control(d.device, &gerror)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(cbool), err
}

func (d *Device) SetStringFeatureValue(feature, value string) {
	cfeature := C.CString(feature)
	cvalue := C.CString(value)
	C.arv_device_set_string_feature_value(d.device, cfeature, cvalue, nil)
	C.free(unsafe.Pointer(cfeature))
	C.free(unsafe.Pointer(cvalue))
}

func (d *Device) GetStringFeatureValue(feature string) (string, error) {
	cfeature := C.CString(feature)
	cvalue, err := C.arv_device_get_string_feature_value(d.device, cfeature, nil)
	C.free(unsafe.Pointer(cfeature))
	return C.GoString(cvalue), err
}

func (d *Device) SetIntegerFeatureValue(feature string, value int64) {
	cfeature := C.CString(feature)
	cvalue := C.long(value)
	C.arv_device_set_integer_feature_value(d.device, cfeature, cvalue, nil)
	C.free(unsafe.Pointer(cfeature))
}

func (d *Device) GetIntegerFeatureValue(feature string) (int64, error) {
	cfeature := C.CString(feature)
	cvalue, err := C.arv_device_get_integer_feature_value(d.device, cfeature, nil)
	C.free(unsafe.Pointer(cfeature))
	return int64(cvalue), err
}

func (d *Device) SetFloatFeatureValue(feature string, value float64) {
	cfeature := C.CString(feature)
	cvalue := C.double(value)
	C.arv_device_set_float_feature_value(d.device, cfeature, cvalue, nil)
	C.free(unsafe.Pointer(cfeature))
}

func (d *Device) GetFloatFeatureValue(feature string) (float64, error) {
	cfeature := C.CString(feature)
	cvalue, err := C.arv_device_get_float_feature_value(d.device, cfeature, nil)
	C.free(unsafe.Pointer(cfeature))
	return float64(cvalue), err
}

func (d *Device) SetNodeFeatureValue(feature, value string) {
	cfeature := C.CString(feature)
	cvalue := C.CString(value)
	C.arv_set_node_feature_value(d.device, cfeature, cvalue)
	C.free(unsafe.Pointer(cfeature))
	C.free(unsafe.Pointer(cvalue))
}

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

func (d *Device) IsNil() bool {
	return d.device == nil
}

// Low-level register and memory access functions for advanced users

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

// Policy configuration functions for advanced users (available in newer versions)
// These functions may not be available in all Aravis 0.8.x versions

// func (d *Device) SetRegisterCachePolicy(policy int) {
//	C.arv_device_set_register_cache_policy(d.device, C.ArvRegisterCachePolicy(policy))
// }

// func (d *Device) SetRangeCheckPolicy(policy int) {
//	C.arv_device_set_range_check_policy(d.device, C.ArvRangeCheckPolicy(policy))
// }

// func (d *Device) SetAccessCheckPolicy(policy int) {
//	C.arv_device_set_access_check_policy(d.device, C.ArvAccessCheckPolicy(policy))
// }
