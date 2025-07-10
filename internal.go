package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
import "C"

import (
	"errors"
	"sync"
)

func toBool(x C.gboolean) bool {
	if int(x) != 0 {
		return true
	} else {
		return false
	}
}

// Error pool to reduce allocations for common errors
var (
	errorPool = sync.Pool{
		New: func() interface{} {
			return &AravisError{}
		},
	}

	// Common pre-allocated errors to avoid string allocations
	commonErrors = map[C.int]*AravisError{
		C.ARV_DEVICE_ERROR_TIMEOUT:           {Code: int(C.ARV_DEVICE_ERROR_TIMEOUT), Message: "device timeout"},
		C.ARV_DEVICE_ERROR_NOT_FOUND:         {Code: int(C.ARV_DEVICE_ERROR_NOT_FOUND), Message: "device not found"},
		C.ARV_DEVICE_ERROR_NOT_CONNECTED:     {Code: int(C.ARV_DEVICE_ERROR_NOT_CONNECTED), Message: "device not connected"},
		C.ARV_DEVICE_ERROR_PROTOCOL_ERROR:    {Code: int(C.ARV_DEVICE_ERROR_PROTOCOL_ERROR), Message: "protocol error"},
		C.ARV_DEVICE_ERROR_TRANSFER_ERROR:    {Code: int(C.ARV_DEVICE_ERROR_TRANSFER_ERROR), Message: "transfer error"},
		C.ARV_DEVICE_ERROR_FEATURE_NOT_FOUND: {Code: int(C.ARV_DEVICE_ERROR_FEATURE_NOT_FOUND), Message: "feature not found"},
	}
)

// AravisError provides structured error information with error code
type AravisError struct {
	Code    int
	Message string
}

func (e *AravisError) Error() string {
	return e.Message
}

func (e *AravisError) Reset() {
	e.Code = 0
	e.Message = ""
}

func errorFromGError(gerr *C.GError) error {
	defer C.g_error_free(gerr)

	// Check if this is a common error we can reuse
	if commonErr, exists := commonErrors[gerr.code]; exists {
		return commonErr
	}

	// For uncommon errors, use the pool to reduce allocations
	pooledErr := errorPool.Get().(*AravisError)
	pooledErr.Code = int(gerr.code)
	pooledErr.Message = goString(gerr.message)

	// Note: We don't put the error back in the pool immediately because
	// the caller might keep a reference to it. The GC will handle cleanup.

	return pooledErr
}

// Fast error creation for performance-critical paths
func fastError(message string) error {
	return errors.New(message) // For simple cases, stick with stdlib
}

func goString(cstr *C.gchar) string {
	return C.GoString((*C.char)(cstr))
}
