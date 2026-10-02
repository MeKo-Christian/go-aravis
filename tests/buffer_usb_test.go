package tests

import (
	"errors"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// The Fake camera is not a USB3 Vision device, so it has no usbfs file to map
// a buffer from. NewUSBBuffer says so through its sentinel, which is how a
// caller knows to fall back to NewBuffer, and allocates nothing.
func TestNewUSBBufferRefusesACameraThatIsNotUSB(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	buffer, err := camera.NewUSBBuffer(1 << 20)
	if !errors.Is(err, aravis.ErrUSBBufferUnavailable) {
		t.Fatalf("NewUSBBuffer on %s = %v, want ErrUSBBufferUnavailable", fakeDeviceID, err)
	}

	if !buffer.IsNil() {
		t.Error("NewUSBBuffer returned a buffer alongside its error")
	}
}

func TestNewUSBBufferOnAClosedCamera(t *testing.T) {
	camera := requireFakeCamera(t)
	camera.Close()

	if _, err := camera.NewUSBBuffer(1 << 20); err == nil {
		t.Error("NewUSBBuffer on a closed camera succeeded")
	}
}
