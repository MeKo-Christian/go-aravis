//go:build linux

package aravis

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// fakeUSBFS builds a locator over a fake /proc/self/fd and /sys/dev/char.
// links maps descriptor names to what they point at; devices maps a
// descriptor to the major:minor its duplicate reports, and serials maps
// major:minor to the serial sysfs holds for it. Duplicates are the original
// number plus 100, and the returned slice records which ones are still open.
func fakeUSBFS(t *testing.T, links map[string]string, devices map[int][2]uint32,
	serials map[string]string,
) (usbfsLocator, *[]int) {
	t.Helper()

	root := t.TempDir()
	fdDir := filepath.Join(root, "fd")
	charDir := filepath.Join(root, "char")

	for _, dir := range []string{fdDir, charDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(fdDir, name)); err != nil {
			t.Fatal(err)
		}
	}

	for dev, serial := range serials {
		node := filepath.Join(charDir, dev)
		if err := os.Mkdir(node, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(node, "serial"), []byte(serial+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	open := &[]int{}
	locator := usbfsLocator{
		fdDir:   fdDir,
		charDir: charDir,
		dup: func(fd int) (int, error) {
			*open = append(*open, fd+100)

			return fd + 100, nil
		},
		close: func(fd int) error {
			i := slices.Index(*open, fd)
			if i < 0 {
				t.Errorf("closed descriptor %d, which is not an open duplicate", fd)

				return syscall.EBADF
			}

			*open = slices.Delete(*open, i, i+1)

			return nil
		},
		rdev: func(fd int) (uint32, uint32, bool) {
			dev, ok := devices[fd-100]

			return dev[0], dev[1], ok
		},
	}

	return locator, open
}

// The station this was written for: the camera's usbfs file among a few
// descriptors that are not usbfs, and a second USB device that is.
func TestUSBFSFindsTheCameraBySerial(t *testing.T) {
	locator, open := fakeUSBFS(t,
		map[string]string{
			"3":    "/dev/null",
			"7":    "/dev/bus/usb/002/003",
			"9":    "/dev/bus/usb/001/004",
			"self": "/dev/bus/usb/002/003", // not a descriptor number
		},
		map[int][2]uint32{7: {189, 130}, 9: {189, 3}},
		map[string]string{"189:130": "23485031", "189:3": "FOOTPEDAL"},
	)

	fd, err := locator.usbfsFD("23485031")
	if err != nil {
		t.Fatalf("usbfsFD = %v", err)
	}

	if fd != 107 {
		t.Errorf("usbfsFD = %d, want the duplicate of descriptor 7 (107)", fd)
	}

	if !slices.Equal(*open, []int{107}) {
		t.Errorf("open duplicates = %v, want only the returned one", *open)
	}
}

func TestUSBFSRefusesNoAndTwoMatches(t *testing.T) {
	tests := []struct {
		name    string
		serial  string
		wantErr error
	}{
		{name: "no device carries the serial", serial: "00000000", wantErr: errUSBFSNotFound},
		// Two descriptors on the same device: the camera opened twice, or a
		// second process-wide handle. Mapping on the wrong one would silently
		// keep the kernel copy, so neither is picked.
		{name: "two descriptors carry it", serial: "23485031", wantErr: errUSBFSAmbiguous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locator, open := fakeUSBFS(t,
				map[string]string{"7": "/dev/bus/usb/002/003", "8": "/dev/bus/usb/002/003"},
				map[int][2]uint32{7: {189, 130}, 8: {189, 130}},
				map[string]string{"189:130": "23485031"},
			)

			fd, err := locator.usbfsFD(tt.serial)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("usbfsFD = %d, %v, want %v", fd, err, tt.wantErr)
			}

			if len(*open) != 0 {
				t.Errorf("duplicates left open: %v", *open)
			}
		})
	}
}

// A camera without a serial number cannot be told apart from any other USB
// device, so the search does not even start.
func TestUSBFSNeedsASerial(t *testing.T) {
	locator, _ := fakeUSBFS(t, nil, nil, nil)

	if _, err := locator.usbfsFD(""); err == nil {
		t.Error("usbfsFD(\"\") succeeded")
	}
}

// /dev/null is character device 1:3 on every Linux system, which pins the
// dev_t decoding.
func TestCharDeviceNumberDecodesDevNull(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	major, minor, ok := charDeviceNumber(int(f.Fd()))
	if !ok || major != 1 || minor != 3 {
		t.Errorf("charDeviceNumber(/dev/null) = %d:%d, %v, want 1:3, true", major, minor, ok)
	}

	dir, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()

	if _, _, ok := charDeviceNumber(int(dir.Fd())); ok {
		t.Error("charDeviceNumber accepted a directory")
	}
}

// The mapped buffer's payload is the mapping itself, shared with the file it
// came from, and closing the buffer unmaps it. An ordinary file stands in for
// the usbfs device file: the C side is the same mmap call either way.
func TestMappedBufferSharesTheFileAndUnmapsOnClose(t *testing.T) {
	const size = 8192

	path := filepath.Join(t.TempDir(), "usbfs-stand-in")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}

	buffer, err := newMappedBuffer(int(f.Fd()), size)
	if err != nil {
		t.Fatalf("newMappedBuffer = %v", err)
	}

	// GetDataSlice is sized by what the camera delivered, which for a buffer
	// nothing has filled yet is zero bytes; the pointer is the payload either
	// way.
	payload, _ := buffer.GetDataUnsafe()
	if payload == nil {
		t.Fatal("mapped buffer has no payload")
	}

	data := unsafe.Slice((*byte)(payload), size)
	data[0], data[size-1] = 0xAB, 0xCD

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if onDisk[0] != 0xAB || onDisk[size-1] != 0xCD {
		t.Error("a write to the payload did not reach the mapped file")
	}

	if n := mappingsOf(t, path); n != 1 {
		t.Fatalf("%d mappings of the file while the buffer is open, want 1", n)
	}

	buffer.Close()

	if n := mappingsOf(t, path); n != 0 {
		t.Errorf("%d mappings of the file after Close, want 0", n)
	}
}

func TestMappedBufferReportsTheKernelsRefusal(t *testing.T) {
	_, err := newMappedBuffer(-1, 4096)
	if !errors.Is(err, ErrUSBBufferUnavailable) || !errors.Is(err, syscall.EBADF) {
		t.Errorf("newMappedBuffer(-1) = %v, want ErrUSBBufferUnavailable wrapping EBADF", err)
	}
}

// mappingsOf counts the process's mappings of path.
func mappingsOf(t *testing.T, path string) int {
	t.Helper()

	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		t.Fatal(err)
	}

	n := 0

	for _, line := range strings.Split(string(maps), "\n") {
		if strings.HasSuffix(line, " "+path) {
			n++
		}
	}

	return n
}
