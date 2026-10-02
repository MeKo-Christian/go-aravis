//go:build linux

package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <errno.h>
// #include <stdlib.h>
// #include <sys/mman.h>
//
// typedef struct {
//     void *mem;
//     size_t size;
// } ArvGoMapping;
//
// static void arv_go_mapping_release(gpointer data) {
//     ArvGoMapping *mapping = data;
//     munmap(mapping->mem, mapping->size);
//     free(mapping);
// }
//
// // arv_go_buffer_new_mapped wraps size bytes mapped from fd in an ArvBuffer
// // that unmaps them when it is finalized. Aravis never frees preallocated
// // data itself, so the mapping lives exactly as long as the buffer.
// //
// // On failure it returns NULL and reports errno through *err_no, explicitly:
// // reading errno through cgo's two-result call form would also pick up errno
// // left behind by calls that succeeded (see internal/cerrno).
// static ArvBuffer *arv_go_buffer_new_mapped(int fd, size_t size, int *err_no) {
//     ArvGoMapping *mapping;
//     void *mem;
//
//     *err_no = 0;
//     mem = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
//     if (mem == MAP_FAILED) {
//         *err_no = errno;
//         return NULL;
//     }
//     mapping = malloc(sizeof *mapping);
//     if (mapping == NULL) {
//         *err_no = ENOMEM;
//         munmap(mem, size);
//         return NULL;
//     }
//     mapping->mem = mem;
//     mapping->size = size;
//     return arv_buffer_new_full(size, mem, mapping, arv_go_mapping_release);
// }
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
)

// usbfsMu serialises NewUSBBuffer from finding the device file to closing its
// duplicate. While one call holds a duplicate, it is one more descriptor on the
// camera in /proc/self/fd, and a concurrent call would count it as a second
// device file and refuse.
var usbfsMu sync.Mutex

// NewUSBBuffer allocates a buffer with room for size bytes of payload in
// memory the USB host controller writes into directly, for a USB3 Vision
// camera on Linux. Use it in place of NewBuffer for the buffers pushed to this
// camera's stream; ownership works exactly as for NewBuffer.
//
// A NewBuffer payload is ordinary process memory, so the kernel's usbfs driver
// receives each USB transfer into a buffer of its own, which it allocates and
// zeroes per transfer, and then copies into the payload. At camera data rates
// that copy is most of the CPU an acquisition costs. A NewUSBBuffer payload is
// mapped from the camera's usbfs device file instead - what
// libusb_dev_mem_alloc does, and what Aravis itself only does from 0.9 on - so
// the controller writes the image straight into it and both the copy and the
// allocation disappear.
//
// The mapping has to be made on the very file Aravis's libusb handle submits
// its transfers on, which Aravis 0.8 does not expose. NewUSBBuffer finds it
// among the process's open descriptors by the camera's serial number; create
// the stream first, as Aravis opens the device file no later than that.
//
// The memory counts against the kernel's usbcore.usbfs_memory_mb budget (16 MB
// by default, shared by every usbfs user on the machine), so a stream of n
// buffers needs n times the payload size of it on top of what is in flight.
// When the budget is spent, the mapping fails with ENOMEM.
//
// It returns an error wrapping ErrUSBBufferUnavailable when the camera is not
// USB3 Vision, its device file cannot be identified, or the kernel refuses the
// mapping. NewBuffer still works then, so the usual response is to fall back
// to it.
//
// It is safe to call from several goroutines; the calls take turns.
func (c *Camera) NewUSBBuffer(size uint) (Buffer, error) {
	if c.IsClosed() {
		return Buffer{}, errors.New("aravis: camera is closed")
	}

	if size == 0 {
		return Buffer{}, fmt.Errorf("%w: zero-sized buffer", ErrUSBBufferUnavailable)
	}

	if !toBool(C.arv_camera_is_uv_device(c.camera)) {
		return Buffer{}, fmt.Errorf("%w: not a USB3 Vision camera", ErrUSBBufferUnavailable)
	}

	serial, err := c.GetDeviceSerialNumber()
	if err != nil {
		return Buffer{}, fmt.Errorf("%w: read serial number: %w", ErrUSBBufferUnavailable, err)
	}

	usbfsMu.Lock()
	defer usbfsMu.Unlock()

	fd, err := systemUSBFS.usbfsFD(serial)
	if err != nil {
		return Buffer{}, fmt.Errorf("%w: %w", ErrUSBBufferUnavailable, err)
	}
	// The mapping keeps the open file alive on its own; the duplicate is only
	// needed to make it.
	defer syscall.Close(fd)

	return newMappedBuffer(fd, size)
}

// newMappedBuffer maps size bytes of fd and wraps them in an owned Buffer
// whose finalization unmaps them.
func newMappedBuffer(fd int, size uint) (Buffer, error) {
	var errNo C.int

	buffer := C.arv_go_buffer_new_mapped(C.int(fd), C.size_t(size), &errNo)
	if buffer == nil {
		return Buffer{}, fmt.Errorf("%w: map %d bytes: %w",
			ErrUSBBufferUnavailable, size, syscall.Errno(errNo))
	}

	return ownedBuffer(buffer), nil
}
