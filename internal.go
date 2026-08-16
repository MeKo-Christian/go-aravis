package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
import "C"

func toBool(x C.gboolean) bool {
	if int(x) != 0 {
		return true
	} else {
		return false
	}
}

// errorMessages is a lookup table mapping known Aravis device error codes to
// stable, human readable messages. It holds only immutable strings and never
// stores or hands out error values: every error returned to a caller is freshly
// allocated, so a caller can never mutate shared package state.
var errorMessages = map[int]string{
	int(C.ARV_DEVICE_ERROR_TIMEOUT):           "device timeout",
	int(C.ARV_DEVICE_ERROR_NOT_FOUND):         "device not found",
	int(C.ARV_DEVICE_ERROR_NOT_CONNECTED):     "device not connected",
	int(C.ARV_DEVICE_ERROR_PROTOCOL_ERROR):    "protocol error",
	int(C.ARV_DEVICE_ERROR_TRANSFER_ERROR):    "transfer error",
	int(C.ARV_DEVICE_ERROR_FEATURE_NOT_FOUND): "feature not found",
}

// AravisError provides structured error information with error code
type AravisError struct {
	Code    int
	Message string
}

func (e *AravisError) Error() string {
	return e.Message
}

// newAravisError builds a caller-owned error for an Aravis error code. Known
// codes get a stable message from errorMessages; anything else keeps the
// message GLib produced. Never returns shared package state.
func newAravisError(code int, glibMessage string) *AravisError {
	if msg, ok := errorMessages[code]; ok {
		return &AravisError{Code: code, Message: msg}
	}
	return &AravisError{Code: code, Message: glibMessage}
}

func errorFromGError(gerr *C.GError) error {
	defer C.g_error_free(gerr)
	return newAravisError(int(gerr.code), goString(gerr.message))
}

func goString(cstr *C.gchar) string {
	return C.GoString(cstr)
}
