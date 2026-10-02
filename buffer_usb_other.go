//go:build !linux

package aravis

import (
	"errors"
	"fmt"
)

// NewUSBBuffer allocates a buffer in memory the USB host controller writes
// into directly. That needs Linux usbfs, so on this platform it always returns
// an error wrapping ErrUSBBufferUnavailable; use NewBuffer instead. See the
// Linux implementation for the details.
func (c *Camera) NewUSBBuffer(size uint) (Buffer, error) {
	if c.IsClosed() {
		return Buffer{}, errors.New("aravis: camera is closed")
	}

	return Buffer{}, fmt.Errorf("%w: usbfs mapping needs Linux", ErrUSBBufferUnavailable)
}
