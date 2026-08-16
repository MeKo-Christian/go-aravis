package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
// #include <stdio.h>
/*
extern void go_control_lost_handler(guintptr key);

// The handler key is carried through GLib as the signal's user_data, so each
// camera's callback finds its own Go handler instead of a package global.
static void control_lost_cb (ArvDevice *device, gpointer user_data)
{
	go_control_lost_handler((guintptr) user_data);
}

static gulong connect_control_lost_cb(ArvCamera *camera, guintptr key)
{
	ArvDevice *device = arv_camera_get_device(camera);
	if (device == NULL)
		return 0;

	return g_signal_connect(device, "control-lost",
		G_CALLBACK (control_lost_cb), (gpointer) key);
}

static void disconnect_control_lost_cb(ArvCamera *camera, gulong handler_id)
{
	ArvDevice *device = arv_camera_get_device(camera);
	if (device == NULL || handler_id == 0)
		return;

	g_signal_handler_disconnect(device, handler_id);
}

static void stream_cb_rt(void *user_data, ArvStreamCallbackType type, ArvBuffer *buffer)
{
	if (type == ARV_STREAM_CALLBACK_TYPE_INIT) {
		if (!arv_make_thread_realtime (10))
			printf ("Failed to make stream thread realtime\n");
	}
}

static ArvStream* arv_camera_create_rt_stream(ArvCamera *camera, void *user_data, GError **error) {
	return arv_camera_create_stream(camera, stream_cb_rt, user_data, error);
}

static void stream_cb_hp(void *user_data, ArvStreamCallbackType type, ArvBuffer *buffer)
{
	if (type == ARV_STREAM_CALLBACK_TYPE_INIT) {
		if (!arv_make_thread_high_priority (-10))
			printf ("Failed to make stream thread high priority\n");
	}
}

static ArvStream* arv_camera_create_hp_stream(ArvCamera *camera, void *user_data, GError **error) {
	return arv_camera_create_stream(camera, stream_cb_hp, user_data, error);
}
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

type ThreadPriorityType int

const (
	ThreadPriorityNormal ThreadPriorityType = iota
	ThreadPriorityRealtime
	ThreadPriorityHigh
)

type Camera struct {
	camera         *C.struct__ArvCamera
	ThreadPriority ThreadPriorityType

	// controlLostKey identifies this camera's entry in controlLost.handlers,
	// and doubles as the user_data the GLib signal carries back. Zero means no
	// handler has been installed yet.
	controlLostKey uintptr
	// controlLostID is the GLib signal handler id, needed to disconnect.
	controlLostID C.gulong
}

const (
	ACQUISITION_MODE_CONTINUOUS   = C.ARV_ACQUISITION_MODE_CONTINUOUS
	ACQUISITION_MODE_SINGLE_FRAME = C.ARV_ACQUISITION_MODE_SINGLE_FRAME
)

const (
	AUTO_OFF        = C.ARV_AUTO_OFF
	AUTO_ONCE       = C.ARV_AUTO_ONCE
	AUTO_CONTINUOUS = C.ARV_AUTO_CONTINUOUS
)

func NewCamera(name string) (Camera, error) {
	var cam Camera
	var gerror *C.GError
	var err error

	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	cam.camera = C.arv_camera_new(cs, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return cam, err
}

func (c *Camera) CreateStream() (Stream, error) {
	var stream Stream
	var gerror *C.GError
	var err error

	switch c.ThreadPriority {
	case ThreadPriorityRealtime:
		stream.stream = C.arv_camera_create_rt_stream(
			c.camera,
			nil,
			&gerror,
		)

	case ThreadPriorityHigh:
		stream.stream = C.arv_camera_create_hp_stream(
			c.camera,
			nil,
			&gerror,
		)

	default:
		stream.stream = C.arv_camera_create_stream(
			c.camera,
			nil,
			nil,
			&gerror,
		)
	}

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	if stream.stream == nil {
		return Stream{}, err
	}

	return stream, err
}

func (c *Camera) GetDevice() (Device, error) {
	var d Device
	var err error

	d.device = C.arv_camera_get_device(c.camera)

	return d, err
}

func (c *Camera) GetVendorName() (string, error) {
	var gerror *C.GError

	name := C.arv_camera_get_vendor_name(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err := errorFromGError(gerror)
		return "", err
	}

	return C.GoString(name), nil
}

func (c *Camera) GetModelName() (string, error) {
	var gerror *C.GError

	name := C.arv_camera_get_model_name(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err := errorFromGError(gerror)
		return "", err
	}

	return C.GoString(name), nil
}

func (c *Camera) GetDeviceId() (string, error) {
	var gerror *C.GError
	var err error

	id := C.arv_camera_get_device_id(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err := errorFromGError(gerror)
		return "", err
	}

	return C.GoString(id), err
}

func (c *Camera) GetDeviceSerialNumber() (string, error) {
	var gerror *C.GError
	var err error

	serialNumber := C.arv_camera_get_device_serial_number(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return "", err
	}

	return C.GoString(serialNumber), err
}

func (c *Camera) GetSensorSize() (int, int, error) {
	var gerror *C.GError
	var err error

	var width, height C.gint
	C.arv_camera_get_sensor_size(
		c.camera,
		&width,
		&height,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(width), int(height), err
}

func (c *Camera) SetRegion(x, y, width, height int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_region(c.camera,
		C.gint(x),
		C.gint(y),
		C.gint(width),
		C.gint(height),
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetRegion() (int, int, int, int, error) {
	var gerror *C.GError
	var err error

	var x, y, width, height C.gint
	C.arv_camera_get_region(
		c.camera,
		&x,
		&y,
		&width,
		&height,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(x), int(y), int(width), int(height), err
}

func (c *Camera) GetHeight() (int, error) {
	var gerror *C.GError
	var err error
	cs := C.CString("Height")
	defer C.free(unsafe.Pointer(cs))

	val := C.arv_camera_get_integer(
		c.camera,
		cs,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(val), err
}

func (c *Camera) GetHeightBounds() (int, int, error) {
	var gerror *C.GError
	var err error

	var min, max C.gint
	C.arv_camera_get_height_bounds(
		c.camera,
		&min,
		&max,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(min), int(max), err
}

func (c *Camera) GetWidth() (int, error) {
	var gerror *C.GError
	var err error
	cs := C.CString("Width")
	defer C.free(unsafe.Pointer(cs))

	val := C.arv_camera_get_integer(
		c.camera,
		cs,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(val), err
}

func (c *Camera) GetWidthBounds() (int, int, error) {
	var gerror *C.GError
	var err error

	var minVal, maxVal C.gint
	C.arv_camera_get_width_bounds(
		c.camera,
		&minVal,
		&maxVal,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(minVal), int(maxVal), err
}

func (c *Camera) SetBinning(dx, dy int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_binning(c.camera, C.gint(dx), C.gint(dy), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetBinning returns the current horizontal and vertical binning factors.
func (c *Camera) GetBinning() (int, int, error) {
	var gerror *C.GError
	var err error

	var dx, dy C.gint
	C.arv_camera_get_binning(
		c.camera,
		&dx,
		&dy,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(dx), int(dy), err
}

func (c *Camera) SetPixelFormat(format uint32) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_pixel_format(c.camera, C.ArvPixelFormat(format), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetPixelFormat() (uint32, error) {
	var gerror *C.GError
	var err error

	format := C.arv_camera_get_pixel_format(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return uint32(format), err
}

func (c *Camera) GetPixelFormatAsString() (string, error) {
	var gerror *C.GError
	var err error

	// The returned string is owned by Aravis and must not be freed.
	format := C.arv_camera_get_pixel_format_as_string(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return "", err
	}

	return C.GoString(format), err
}

func (c *Camera) SetPixelFormatFromString(format string) error {
	var gerror *C.GError
	var err error

	cs := C.CString(format)
	defer C.free(unsafe.Pointer(cs))

	C.arv_camera_set_pixel_format_from_string(c.camera, cs, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetAvailablePixelFormats() ([]uint32, error) {
	var gerror *C.GError
	var err error
	var n C.guint

	// The returned array is owned by the caller and has to be freed.
	formats := C.arv_camera_dup_available_pixel_formats(c.camera, &n, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	if formats == nil {
		return nil, err
	}
	defer C.g_free(C.gpointer(formats))

	if n == 0 {
		return nil, err
	}

	result := make([]uint32, 0, int(n))
	for _, format := range unsafe.Slice(formats, int(n)) {
		result = append(result, uint32(format))
	}

	return result, err
}

func (c *Camera) GetAvailablePixelFormatsAsDisplayNames() ([]string, error) {
	var gerror *C.GError
	var err error
	var n C.guint

	// Only the array itself is owned by the caller, not the strings it holds.
	names := C.arv_camera_dup_available_pixel_formats_as_display_names(c.camera, &n, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	if names == nil {
		return nil, err
	}
	defer C.g_free(C.gpointer(names))

	if n == 0 {
		return nil, err
	}

	result := make([]string, 0, int(n))
	for _, name := range unsafe.Slice(names, int(n)) {
		result = append(result, C.GoString(name))
	}

	return result, err
}

func (c *Camera) GetAvailablePixelFormatsAsStrings() ([]string, error) {
	var gerror *C.GError
	var err error
	var n C.guint

	// Only the array itself is owned by the caller, not the strings it holds.
	formats := C.arv_camera_dup_available_pixel_formats_as_strings(c.camera, &n, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	if formats == nil {
		return nil, err
	}
	defer C.g_free(C.gpointer(formats))

	if n == 0 {
		return nil, err
	}

	result := make([]string, 0, int(n))
	for _, format := range unsafe.Slice(formats, int(n)) {
		result = append(result, C.GoString(format))
	}

	return result, err
}

func (c *Camera) StartAcquisition() error {
	var gerror *C.GError
	var err error

	C.arv_camera_start_acquisition(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) StopAcquisition() error {
	var gerror *C.GError
	var err error

	C.arv_camera_stop_acquisition(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) AbortAcquisition() error {
	var gerror *C.GError
	var err error

	C.arv_camera_abort_acquisition(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) SetAcquisitionMode(mode int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_acquisition_mode(c.camera, C.ArvAcquisitionMode(mode), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) SetFrameRate(frameRate float64) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_frame_rate(c.camera, C.double(frameRate), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetFrameRate() (float64, error) {
	var gerror *C.GError
	var err error

	fr := C.arv_camera_get_frame_rate(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(fr), err
}

func (c *Camera) GetFrameRateBounds() (float64, float64, error) {
	var gerror *C.GError
	var err error

	var minVal, maxVal float64
	C.arv_camera_get_frame_rate_bounds(
		c.camera,
		(*C.double)(unsafe.Pointer(&minVal)),
		(*C.double)(unsafe.Pointer(&maxVal)),
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return float64(minVal), float64(maxVal), err
}

func (c *Camera) SetLineRate(lineRate float64) error {
	return c.SetFrameRate(lineRate)
}

func (c *Camera) GetLineRate() (float64, error) {
	return c.GetFrameRate()
}

func (c *Camera) SetTrigger(source string) error {
	var gerror *C.GError
	var err error

	csource := C.CString(source)
	C.arv_camera_set_trigger(c.camera, csource, &gerror)
	C.free(unsafe.Pointer(csource))

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) SetTriggerSource(source string) error {
	var gerror *C.GError
	var err error

	csource := C.CString(source)
	C.arv_camera_set_trigger_source(c.camera, csource, &gerror)
	C.free(unsafe.Pointer(csource))

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetTriggerSource() (string, error) {
	var gerror *C.GError
	var err error

	csource := C.arv_camera_get_trigger_source(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
		return "", err
	}

	return C.GoString(csource), err
}

func (c *Camera) SoftwareTrigger() error {
	var gerror *C.GError
	var err error

	C.arv_camera_software_trigger(c.camera, &gerror)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) ClearTriggers() error {
	var gerror *C.GError
	var err error

	C.arv_camera_clear_triggers(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) IsExposureTimeAvailable() (bool, error) {
	var gerror *C.GError
	var err error

	gboolean := C.arv_camera_is_exposure_time_available(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(gboolean), err
}

func (c *Camera) IsExposureAutoAvailable() (bool, error) {
	var gerror *C.GError
	var err error

	gboolean := C.arv_camera_is_exposure_auto_available(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return toBool(gboolean), err
}

func (c *Camera) SetExposureTime(time float64) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_exposure_time(c.camera, C.double(time), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetExposureTime() (float64, error) {
	var gerror *C.GError
	var err error

	cdouble := C.arv_camera_get_exposure_time(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(cdouble), err
}

func (c *Camera) GetExposureTimeBounds() (float64, float64, error) {
	var gerror *C.GError
	var err error

	var minVal, maxVal C.double
	C.arv_camera_get_exposure_time_bounds(
		c.camera,
		&minVal,
		&maxVal,
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(minVal), float64(maxVal), err
}

func (c *Camera) SetExposureTimeAuto(mode int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_exposure_time_auto(c.camera, C.ArvAuto(mode), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetExposureTimeAuto() (int, error) {
	var gerror *C.GError
	var err error

	mode := C.arv_camera_get_exposure_time_auto(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(mode), err
}

func (c *Camera) SetGain(gain float64) error {
	var gerror *C.GError
	var err error
	C.arv_camera_set_gain(c.camera, C.double(gain), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetGain() (float64, error) {
	var gerror *C.GError
	var err error

	cgain := C.arv_camera_get_gain(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(cgain), err
}

func (c *Camera) GetGainBounds() (float64, float64, error) {
	var gerror *C.GError
	var err error

	var minVal, maxVal float64
	C.arv_camera_get_gain_bounds(
		c.camera,
		(*C.double)(unsafe.Pointer(&minVal)),
		(*C.double)(unsafe.Pointer(&maxVal)),
		&gerror,
	)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(minVal), float64(maxVal), err
}

func (c *Camera) SetGainAuto(mode int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_gain_auto(c.camera, C.ArvAuto(mode), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetPayloadSize() (uint, error) {
	var gerror *C.GError
	var err error

	csize := C.arv_camera_get_payload(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return uint(csize), err
}

func (c *Camera) IsGVDevice() (bool, error) {
	cbool := C.arv_camera_is_gv_device(c.camera)

	return toBool(cbool), nil
}

func (c *Camera) GVGetNumStreamChannels() (int, error) {
	var gerror *C.GError
	var err error

	cint := C.arv_camera_gv_get_n_stream_channels(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(cint), err
}

func (c *Camera) GVSelectStreamChannels(id int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_gv_select_stream_channel(c.camera, C.gint(id), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GVGetCurrentStreamChannel() (int, error) {
	var gerror *C.GError
	var err error

	cint := C.arv_camera_gv_get_current_stream_channel(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(cint), err
}

func (c *Camera) GVGetPacketDelay() (int64, error) {
	var gerror *C.GError
	var err error

	cint64 := C.arv_camera_gv_get_packet_delay(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int64(cint64), err
}

func (c *Camera) GVSetPacketDelay(delay int64) error {
	var gerror *C.GError
	var err error

	C.arv_camera_gv_set_packet_delay(c.camera, C.gint64(delay), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

func (c *Camera) GVGetPacketSize() (int, error) {
	var gerror *C.GError
	var err error

	csize := C.arv_camera_gv_get_packet_size(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(csize), err
}

func (c *Camera) GVSetPacketSize(size int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_gv_set_packet_size(c.camera, C.gint(size), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

func (c *Camera) GetChunkMode() (bool, error) {
	var gerror *C.GError
	var err error

	mode := C.arv_camera_get_chunk_mode(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(mode), err
}

// Close releases the camera. It is safe to call more than once; subsequent
// calls do nothing. Any control-lost handler installed on this camera is
// disconnected first. The Camera must not be used afterwards.
func (c *Camera) Close() {
	if c.camera == nil {
		return
	}

	c.clearControlLostHandler()

	C.g_object_unref(C.gpointer(c.camera))
	c.camera = nil
}

// controlLost maps handler keys to the Go callbacks they belong to. The C
// callback runs on an Aravis thread, so every access has to be synchronized;
// the previous package-global handler variable was both shared between all
// cameras and read from that thread without any locking.
var controlLost = struct {
	sync.Mutex
	handlers map[uintptr]func()
	nextKey  uintptr
}{
	handlers: make(map[uintptr]func()),
}

// SetControlLostHandler installs hdl as this camera's control-lost callback,
// replacing any handler set before. Passing nil removes the handler.
//
// hdl is invoked from an Aravis thread, not from the goroutine that called
// this method, so it must be safe to run concurrently with the rest of the
// program. Handlers are per-camera: installing one on a camera has no effect
// on any other.
func (c *Camera) SetControlLostHandler(hdl func()) error {
	if c.camera == nil {
		return errors.New("aravis: camera is closed")
	}

	if hdl == nil {
		c.clearControlLostHandler()
		return nil
	}

	if c.controlLostKey != 0 {
		// Already connected to the signal — just swap the callback.
		controlLost.Lock()
		controlLost.handlers[c.controlLostKey] = hdl
		controlLost.Unlock()

		return nil
	}

	controlLost.Lock()
	controlLost.nextKey++
	key := controlLost.nextKey
	controlLost.handlers[key] = hdl
	controlLost.Unlock()

	// Register the callback only after the handler is reachable, so a signal
	// arriving immediately finds it.
	handlerID := C.connect_control_lost_cb(c.camera, C.guintptr(key))
	if handlerID == 0 {
		controlLost.Lock()
		delete(controlLost.handlers, key)
		controlLost.Unlock()

		return errors.New("aravis: could not connect the control-lost signal")
	}

	c.controlLostKey = key
	c.controlLostID = handlerID

	return nil
}

// clearControlLostHandler disconnects the signal and drops the callback, if
// one was installed.
func (c *Camera) clearControlLostHandler() {
	if c.controlLostKey == 0 {
		return
	}

	C.disconnect_control_lost_cb(c.camera, c.controlLostID)

	controlLost.Lock()
	delete(controlLost.handlers, c.controlLostKey)
	controlLost.Unlock()

	c.controlLostKey = 0
	c.controlLostID = 0
}

func (c *Camera) IsNil() bool {
	return c.camera == nil
}

//export go_control_lost_handler
func go_control_lost_handler(key C.guintptr) {
	controlLost.Lock()
	hdl := controlLost.handlers[uintptr(key)]
	controlLost.Unlock()

	// Called without the lock held: the handler may well call back into this
	// package, and holding the lock would deadlock it.
	if hdl != nil {
		hdl()
	}
}
