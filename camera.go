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

// ThreadPriorityType selects the scheduling priority Aravis applies to the
// thread that receives stream buffers. It is set through the Camera's
// ThreadPriority field and is read by CreateStream when the stream is created.
type ThreadPriorityType int

const (
	// ThreadPriorityNormal leaves the stream thread at its default scheduling
	// priority. This is the zero value, so it applies unless ThreadPriority is
	// set explicitly.
	ThreadPriorityNormal ThreadPriorityType = iota
	// ThreadPriorityRealtime asks Aravis to move the stream thread to realtime
	// scheduling (arv_make_thread_realtime with priority 10) when the thread
	// starts. This usually requires elevated privileges or an RLIMIT_RTPRIO
	// allowance; on failure Aravis only prints a message and the thread keeps
	// its normal priority.
	ThreadPriorityRealtime
	// ThreadPriorityHigh asks Aravis to raise the stream thread's priority
	// without realtime scheduling (arv_make_thread_high_priority with nice
	// value -10). On failure Aravis only prints a message and the thread keeps
	// its normal priority.
	ThreadPriorityHigh
)

// cameraState is the lifecycle state that belongs to the underlying camera
// rather than to one Go value wrapping it. Camera is handed out by value and
// copied freely, so a copy must see the same registration and the same closed
// flag — otherwise two copies could each install a control-lost handler, or
// each unref the camera.
type cameraState struct {
	closed closeFlag

	mu sync.Mutex
	// controlLostKey identifies this camera's entry in controlLost.handlers,
	// and doubles as the user_data the GLib signal carries back. Zero means no
	// handler has been installed yet.
	controlLostKey uintptr
	// controlLostID is the GLib signal handler id, needed to disconnect.
	controlLostID C.gulong
}

// Camera is a handle on an Aravis camera (ArvCamera), the high-level entry
// point for controlling a GigE Vision or USB3 Vision device.
//
// Camera is a value type and may be copied freely: every copy refers to the
// same underlying camera and shares its lifecycle state. Close must be called
// once the camera is no longer needed; it is idempotent per underlying camera,
// so calling it on several copies still unrefs the camera exactly once, and
// neither the closed value nor any copy of it may be used afterwards. The zero
// value owns nothing, and its Close is a no-op.
//
// A Camera is not otherwise synchronized: concurrent calls that talk to the
// same device should be serialized by the caller.
type Camera struct {
	camera *C.struct__ArvCamera
	// ThreadPriority selects the scheduling priority of the stream receiving
	// thread. It is only read by CreateStream, so it has to be set before the
	// stream is created; changing it afterwards has no effect on streams that
	// already exist. There is no setter method — assign the field directly.
	ThreadPriority ThreadPriorityType

	// state is shared by every copy of this Camera. Nil for the zero value.
	state *cameraState
}

// Acquisition modes accepted by SetAcquisitionMode. They mirror the
// ArvAcquisitionMode enumeration.
const (
	// ACQUISITION_MODE_CONTINUOUS keeps acquiring frames until acquisition is
	// stopped.
	ACQUISITION_MODE_CONTINUOUS = C.ARV_ACQUISITION_MODE_CONTINUOUS
	// ACQUISITION_MODE_SINGLE_FRAME acquires a single frame per acquisition
	// start.
	ACQUISITION_MODE_SINGLE_FRAME = C.ARV_ACQUISITION_MODE_SINGLE_FRAME
)

// Auto modes for the camera's automatic feature control. They mirror the
// ArvAuto enumeration and are accepted by SetExposureTimeAuto and SetGainAuto,
// and returned by GetExposureTimeAuto.
const (
	// AUTO_OFF disables the automatic control; the feature keeps its manually
	// set value.
	AUTO_OFF = C.ARV_AUTO_OFF
	// AUTO_ONCE runs the automatic control until it converges, then switches
	// the feature back to manual.
	AUTO_ONCE = C.ARV_AUTO_ONCE
	// AUTO_CONTINUOUS keeps the automatic control running permanently.
	AUTO_CONTINUOUS = C.ARV_AUTO_CONTINUOUS
)

// NewCamera opens the camera identified by name and returns a handle on it.
// The name is matched by Aravis against device ids, vendor/model aliases and
// serial numbers; an empty name selects the first available camera.
//
// The returned Camera must be released with Close. On error the returned
// Camera is unusable — check it with IsNil, or check the error.
func NewCamera(name string) (Camera, error) {
	var cam Camera
	var gerror *C.GError
	var err error

	// Aravis takes NULL, not the empty string, as the sentinel for "the first
	// available camera". C.CString("") would produce a non-NULL pointer to an
	// empty name, which no camera matches.
	var cs *C.char
	if name != "" {
		cs = C.CString(name)
		defer C.free(unsafe.Pointer(cs))
	}

	cam.camera = C.arv_camera_new(cs, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	if cam.camera != nil {
		cam.state = &cameraState{}
	}

	return cam, err
}

// CreateStream creates a new image stream for this camera. The stream's
// receiving thread is set up according to the camera's ThreadPriority field,
// which is read here and nowhere else.
//
// The returned Stream owns its underlying object and must be released with
// Close. If the stream could not be created, the zero Stream is returned
// together with the error.
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

	stream.closed = newCloseFlag()

	return stream, err
}

// GetDevice returns the low-level Device the camera talks through, for feature
// and register access that the Camera API does not expose.
//
// The returned Device only borrows the camera's device reference: it does not
// own it, and the caller must not Close it. Its lifetime is that of the
// Camera, so it must not be used after the Camera has been closed. This is the
// opposite of OpenDevice, which hands ownership to the caller.
//
// The returned error is always nil; it exists for signature symmetry with the
// rest of the API.
func (c *Camera) GetDevice() (Device, error) {
	var d Device
	var err error

	d.device = C.arv_camera_get_device(c.camera)

	return d, err
}

// GetVendorName returns the camera's vendor name as reported by the device.
func (c *Camera) GetVendorName() (string, error) {
	var gerror *C.GError

	name := C.arv_camera_get_vendor_name(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err := errorFromGError(gerror)
		return "", err
	}

	return C.GoString(name), nil
}

// GetModelName returns the camera's model name as reported by the device.
func (c *Camera) GetModelName() (string, error) {
	var gerror *C.GError

	name := C.arv_camera_get_model_name(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err := errorFromGError(gerror)
		return "", err
	}

	return C.GoString(name), nil
}

// GetDeviceId returns the camera's device id, the same identifier used for
// device enumeration and accepted by NewCamera.
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

// GetDeviceSerialNumber returns the camera's serial number as reported by the
// device.
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

// GetSensorSize returns the width and height of the camera's sensor in pixels.
// This is the physical sensor size, independent of the currently configured
// region of interest or binning.
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

// SetRegion sets the region of interest to the given offset and size, in
// pixels. Aravis writes the OffsetX, OffsetY, Width and Height features; the
// camera may clamp or round the values to what it supports, so read them back
// with GetRegion if the exact geometry matters.
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

// GetRegion returns the current region of interest as x, y, width and height
// in pixels.
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

// GetHeight returns the current image height in pixels. It reads the GenICam
// "Height" integer feature directly rather than going through the region
// accessors.
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

// GetHeightBounds returns the minimum and maximum image height the camera
// accepts, in pixels.
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

// GetWidth returns the current image width in pixels. It reads the GenICam
// "Width" integer feature directly rather than going through the region
// accessors.
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

// GetWidthBounds returns the minimum and maximum image width the camera
// accepts, in pixels.
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

// SetBinning sets the horizontal and vertical binning factors, dx and dy. A
// factor of 1 disables binning on that axis. Not every camera supports binning
// or arbitrary factors; read the applied values back with GetBinning.
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

// SetPixelFormat sets the pixel format from its numeric PFNC/GenICam code, as
// returned by GetPixelFormat or GetAvailablePixelFormats.
func (c *Camera) SetPixelFormat(format uint32) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_pixel_format(c.camera, C.ArvPixelFormat(format), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetPixelFormat returns the current pixel format as its numeric PFNC/GenICam
// code.
func (c *Camera) GetPixelFormat() (uint32, error) {
	var gerror *C.GError
	var err error

	format := C.arv_camera_get_pixel_format(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return uint32(format), err
}

// GetPixelFormatAsString returns the current pixel format as its GenICam name,
// for example "Mono8" or "BayerRG8".
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

// SetPixelFormatFromString sets the pixel format from its GenICam name, for
// example "Mono8", as listed by GetAvailablePixelFormatsAsStrings.
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

// GetAvailablePixelFormats returns the numeric PFNC/GenICam codes of every
// pixel format the camera supports. The C array Aravis allocates is freed
// before returning, so the result is owned by the caller. It is nil when the
// camera reports no formats.
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

// GetAvailablePixelFormatsAsDisplayNames returns the human readable names of
// every pixel format the camera supports, in the same order as
// GetAvailablePixelFormats. These names are meant for display and are not
// accepted by SetPixelFormatFromString — use
// GetAvailablePixelFormatsAsStrings for that.
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

// GetAvailablePixelFormatsAsStrings returns the GenICam names of every pixel
// format the camera supports, in the same order as GetAvailablePixelFormats.
// These names are the ones SetPixelFormatFromString accepts.
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

// StartAcquisition starts image acquisition. Buffers should already be pushed
// to the stream created with CreateStream before calling this.
func (c *Camera) StartAcquisition() error {
	var gerror *C.GError
	var err error

	C.arv_camera_start_acquisition(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// StopAcquisition stops image acquisition, letting the frame in flight
// complete. Use AbortAcquisition to stop immediately.
func (c *Camera) StopAcquisition() error {
	var gerror *C.GError
	var err error

	C.arv_camera_stop_acquisition(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// AbortAcquisition stops image acquisition immediately, without waiting for
// the frame in flight to complete. Not every camera implements the
// AcquisitionAbort command.
func (c *Camera) AbortAcquisition() error {
	var gerror *C.GError
	var err error

	C.arv_camera_abort_acquisition(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// SetAcquisitionMode sets the acquisition mode, which must be one of the
// ACQUISITION_MODE_* constants.
func (c *Camera) SetAcquisitionMode(mode int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_acquisition_mode(c.camera, C.ArvAcquisitionMode(mode), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// SetFrameRate sets the acquisition frame rate in frames per second. Aravis
// also switches the camera to the free-running trigger configuration that the
// frame rate feature requires.
func (c *Camera) SetFrameRate(frameRate float64) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_frame_rate(c.camera, C.double(frameRate), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetFrameRate returns the current acquisition frame rate in frames per
// second.
func (c *Camera) GetFrameRate() (float64, error) {
	var gerror *C.GError
	var err error

	fr := C.arv_camera_get_frame_rate(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(fr), err
}

// GetFrameRateBounds returns the minimum and maximum frame rate the camera
// accepts, in frames per second, for the current configuration.
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

// SetLineRate forwards lineRate verbatim to SetFrameRate.
//
// It does not touch any AcquisitionLineRate feature: for a line-scan camera
// this sets the frame rate, not the line rate, and the two are only the same
// when the camera happens to expose one line per frame. Set the line rate
// through the device's GenICam feature instead if you need it.
func (c *Camera) SetLineRate(lineRate float64) error {
	return c.SetFrameRate(lineRate)
}

// GetLineRate returns whatever GetFrameRate returns; it is a verbatim forward.
//
// It does not read any AcquisitionLineRate feature, so on a line-scan camera
// the value is the frame rate, not the line rate. Read the line rate through
// the device's GenICam feature instead if you need it.
func (c *Camera) GetLineRate() (float64, error) {
	return c.GetFrameRate()
}

// SetTrigger configures the camera for triggered acquisition on the given
// trigger source, for example "Line1" or "Software". Aravis sets the frame
// start trigger to On, selects the source, and disables the other triggers.
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

// SetTriggerSource sets only the trigger source of the currently selected
// trigger, without changing the trigger mode. Use SetTrigger to enable
// triggered acquisition in the first place.
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

// GetTriggerSource returns the trigger source of the currently selected
// trigger.
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

// SoftwareTrigger fires one software trigger by executing the TriggerSoftware
// command. It only has an effect when the camera has been configured for
// software triggering, for example with SetTrigger("Software").
func (c *Camera) SoftwareTrigger() error {
	var gerror *C.GError
	var err error

	C.arv_camera_software_trigger(c.camera, &gerror)

	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// ClearTriggers disables every trigger of the camera, returning it to
// free-running acquisition.
func (c *Camera) ClearTriggers() error {
	var gerror *C.GError
	var err error

	C.arv_camera_clear_triggers(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// IsExposureTimeAvailable reports whether the camera exposes a manual exposure
// time feature.
func (c *Camera) IsExposureTimeAvailable() (bool, error) {
	var gerror *C.GError
	var err error

	gboolean := C.arv_camera_is_exposure_time_available(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(gboolean), err
}

// IsExposureAutoAvailable reports whether the camera exposes an automatic
// exposure feature, that is, whether SetExposureTimeAuto and
// GetExposureTimeAuto can be used.
func (c *Camera) IsExposureAutoAvailable() (bool, error) {
	var gerror *C.GError
	var err error

	gboolean := C.arv_camera_is_exposure_auto_available(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return toBool(gboolean), err
}

// SetExposureTime sets the exposure time in microseconds. The value has to lie
// within the bounds reported by GetExposureTimeBounds, and automatic exposure
// has to be off (see SetExposureTimeAuto) for it to stick.
func (c *Camera) SetExposureTime(time float64) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_exposure_time(c.camera, C.double(time), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetExposureTime returns the current exposure time in microseconds.
func (c *Camera) GetExposureTime() (float64, error) {
	var gerror *C.GError
	var err error

	cdouble := C.arv_camera_get_exposure_time(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(cdouble), err
}

// GetExposureTimeBounds returns the minimum and maximum exposure time the
// camera accepts, in microseconds.
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

// SetExposureTimeAuto sets the automatic exposure mode, which must be one of
// the AUTO_* constants.
func (c *Camera) SetExposureTimeAuto(mode int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_exposure_time_auto(c.camera, C.ArvAuto(mode), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetExposureTimeAuto returns the current automatic exposure mode as one of
// the AUTO_* constants.
func (c *Camera) GetExposureTimeAuto() (int, error) {
	var gerror *C.GError
	var err error

	mode := C.arv_camera_get_exposure_time_auto(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(mode), err
}

// SetGain sets the analog gain. The unit is camera specific — typically dB,
// but some cameras use a raw device unit. The value has to lie within the
// bounds reported by GetGainBounds, and automatic gain has to be off (see
// SetGainAuto) for it to stick.
func (c *Camera) SetGain(gain float64) error {
	var gerror *C.GError
	var err error
	C.arv_camera_set_gain(c.camera, C.double(gain), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetGain returns the current analog gain, in the camera's gain unit.
func (c *Camera) GetGain() (float64, error) {
	var gerror *C.GError
	var err error

	cgain := C.arv_camera_get_gain(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return float64(cgain), err
}

// GetGainBounds returns the minimum and maximum gain the camera accepts, in
// the camera's gain unit.
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

// SetGainAuto sets the automatic gain mode, which must be one of the AUTO_*
// constants.
func (c *Camera) SetGainAuto(mode int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_set_gain_auto(c.camera, C.ArvAuto(mode), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetPayloadSize returns the size in bytes of one image payload for the
// current camera configuration. Buffers passed to the stream must be at least
// this large, so it should be read after the region, binning and pixel format
// have been set.
func (c *Camera) GetPayloadSize() (uint, error) {
	var gerror *C.GError
	var err error

	csize := C.arv_camera_get_payload(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return uint(csize), err
}

// IsGVDevice reports whether the camera is a GigE Vision device, that is,
// whether the GV* methods apply to it. The returned error is always nil.
func (c *Camera) IsGVDevice() (bool, error) {
	cbool := C.arv_camera_is_gv_device(c.camera)

	return toBool(cbool), nil
}

// GVGetNumStreamChannels returns the number of stream channels the GigE Vision
// camera provides. GigE Vision only.
func (c *Camera) GVGetNumStreamChannels() (int, error) {
	var gerror *C.GError
	var err error

	cint := C.arv_camera_gv_get_n_stream_channels(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(cint), err
}

// GVSelectStreamChannels selects the stream channel the subsequent stream
// related calls apply to. Valid ids range from 0 to
// GVGetNumStreamChannels()-1. GigE Vision only.
func (c *Camera) GVSelectStreamChannels(id int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_gv_select_stream_channel(c.camera, C.gint(id), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GVGetCurrentStreamChannel returns the id of the currently selected stream
// channel. GigE Vision only.
func (c *Camera) GVGetCurrentStreamChannel() (int, error) {
	var gerror *C.GError
	var err error

	cint := C.arv_camera_gv_get_current_stream_channel(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(cint), err
}

// GVGetPacketDelay returns the inter-packet delay in nanoseconds. GigE Vision
// only.
func (c *Camera) GVGetPacketDelay() (int64, error) {
	var gerror *C.GError
	var err error

	cint64 := C.arv_camera_gv_get_packet_delay(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int64(cint64), err
}

// GVSetPacketDelay sets the delay between two stream packets, in nanoseconds.
// Raising it throttles the camera and helps against packet loss when the host
// or the network cannot keep up. GigE Vision only.
func (c *Camera) GVSetPacketDelay(delay int64) error {
	var gerror *C.GError
	var err error

	C.arv_camera_gv_set_packet_delay(c.camera, C.gint64(delay), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}
	return err
}

// GVGetPacketSize returns the stream packet size in bytes. GigE Vision only.
func (c *Camera) GVGetPacketSize() (int, error) {
	var gerror *C.GError
	var err error

	csize := C.arv_camera_gv_get_packet_size(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return int(csize), err
}

// GVSetPacketSize sets the stream packet size in bytes. It must not exceed
// what every hop between camera and host can carry: packets larger than 1500
// bytes require jumbo frames (MTU 9000) on the network interface. GigE Vision
// only.
func (c *Camera) GVSetPacketSize(size int) error {
	var gerror *C.GError
	var err error

	C.arv_camera_gv_set_packet_size(c.camera, C.gint(size), &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return err
}

// GetChunkMode reports whether chunk data is enabled, that is, whether the
// camera appends metadata chunks to the image buffers. See Buffer.HasChunks
// for reading them back.
func (c *Camera) GetChunkMode() (bool, error) {
	var gerror *C.GError
	var err error

	mode := C.arv_camera_get_chunk_mode(c.camera, &gerror)
	if unsafe.Pointer(gerror) != nil {
		err = errorFromGError(gerror)
	}

	return toBool(mode), err
}

// Close releases the camera. It is safe to call more than once, and safe to
// call on any copy of the same Camera: the camera is unreffed exactly once.
// Any control-lost handler installed on it is disconnected first. Neither this
// Camera nor any copy of it may be used afterwards.
func (c *Camera) Close() {
	if c.camera == nil || c.state == nil || !c.state.closed.claim() {
		return
	}

	c.clearControlLostHandler()

	C.g_object_unref(C.gpointer(c.camera))
}

// IsClosed reports whether the camera has been released, by this value or by
// any copy of it.
func (c *Camera) IsClosed() bool {
	return c.camera == nil || c.state == nil || c.state.closed.isClosed()
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
// program. Handlers are per-camera and shared by every copy of a Camera:
// installing one on a camera has no effect on any other camera, and a copy
// replaces the handler rather than adding a second one.
func (c *Camera) SetControlLostHandler(hdl func()) error {
	if c.camera == nil || c.state == nil || c.state.closed.isClosed() {
		return errors.New("aravis: camera is closed")
	}

	if hdl == nil {
		c.clearControlLostHandler()
		return nil
	}

	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	if c.state.controlLostKey != 0 {
		// Already connected to the signal — just swap the callback.
		controlLost.Lock()
		controlLost.handlers[c.state.controlLostKey] = hdl
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

	c.state.controlLostKey = key
	c.state.controlLostID = handlerID

	return nil
}

// clearControlLostHandler disconnects the signal and drops the callback, if
// one was installed.
func (c *Camera) clearControlLostHandler() {
	if c.state == nil {
		return
	}

	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	if c.state.controlLostKey == 0 {
		return
	}

	C.disconnect_control_lost_cb(c.camera, c.state.controlLostID)

	controlLost.Lock()
	delete(controlLost.handlers, c.state.controlLostKey)
	controlLost.Unlock()

	c.state.controlLostKey = 0
	c.state.controlLostID = 0
}

// IsNil reports whether the Camera holds no underlying camera at all — the
// zero value, or the value returned by a failed NewCamera. It says nothing
// about whether the camera has been closed; use IsClosed for that.
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
