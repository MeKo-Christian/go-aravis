//go:build linux

package aravis

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// fakeFDTable is a fake /proc/self/fd and /sys/dev/char that behaves like the
// kernel where it matters here: a duplicate takes the lowest free descriptor
// number from dupFrom up, and its link points where the original's does.
type fakeFDTable struct {
	t       *testing.T
	fdDir   string
	charDir string
	// devices maps a link target to the major:minor of its device.
	devices map[string][2]uint32
	// dupFrom is the lowest number a duplicate may take.
	dupFrom int
	// closedAfterListing are descriptors that are gone by the time the scan
	// duplicates anything, the way ReadDir's own one is.
	closedAfterListing []string
	// failDup makes duplicating these descriptors fail.
	failDup map[int]error
	// open lists the duplicates the scan has not closed.
	open []int
}

func newFakeFDTable(t *testing.T, links map[string]string, devices map[string][2]uint32,
	serials map[string]string,
) *fakeFDTable {
	t.Helper()

	root := t.TempDir()
	f := &fakeFDTable{
		t:       t,
		fdDir:   filepath.Join(root, "fd"),
		charDir: filepath.Join(root, "char"),
		devices: devices,
		dupFrom: 100,
	}

	for _, dir := range []string{f.fdDir, f.charDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(f.fdDir, name)); err != nil {
			t.Fatal(err)
		}
	}

	for dev, serial := range serials {
		node := filepath.Join(f.charDir, dev)
		if err := os.Mkdir(node, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(node, "serial"), []byte(serial+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return f
}

func (f *fakeFDTable) link(fd int) string { return filepath.Join(f.fdDir, strconv.Itoa(fd)) }

func (f *fakeFDTable) dup(fd int) (int, error) {
	if err := f.failDup[fd]; err != nil {
		return -1, err
	}

	for _, name := range f.closedAfterListing {
		_ = os.Remove(filepath.Join(f.fdDir, name))
	}
	f.closedAfterListing = nil

	target, err := os.Readlink(f.link(fd))
	if err != nil {
		return -1, syscall.EBADF
	}

	n := f.dupFrom
	for {
		if _, err := os.Lstat(f.link(n)); os.IsNotExist(err) {
			break
		}
		n++
	}

	if err := os.Symlink(target, f.link(n)); err != nil {
		f.t.Fatal(err)
	}

	f.open = append(f.open, n)

	return n, nil
}

func (f *fakeFDTable) close(fd int) error {
	i := slices.Index(f.open, fd)
	if i < 0 {
		f.t.Errorf("closed descriptor %d, which is not an open duplicate", fd)

		return syscall.EBADF
	}

	f.open = slices.Delete(f.open, i, i+1)

	return os.Remove(f.link(fd))
}

func (f *fakeFDTable) rdev(fd int) (uint32, uint32, bool) {
	target, err := os.Readlink(f.link(fd))
	if err != nil {
		return 0, 0, false
	}

	dev, ok := f.devices[target]

	return dev[0], dev[1], ok
}

func (f *fakeFDTable) locator() usbfsLocator {
	return usbfsLocator{fdDir: f.fdDir, charDir: f.charDir, dup: f.dup, close: f.close, rdev: f.rdev}
}

const (
	cameraNode = "/dev/bus/usb/002/003"
	pedalNode  = "/dev/bus/usb/001/004"
)

var (
	stationDevices = map[string][2]uint32{cameraNode: {189, 130}, pedalNode: {189, 3}}
	stationSerials = map[string]string{"189:130": "23485031", "189:3": "FOOTPEDAL"}
)

// The camera's usbfs file among a few descriptors that are not usbfs, and a
// second USB device that is.
func TestUSBFSFindsTheCameraBySerial(t *testing.T) {
	f := newFakeFDTable(t,
		map[string]string{
			"3":    "/dev/null",
			"7":    cameraNode,
			"9":    pedalNode,
			"self": cameraNode, // not a descriptor number
		},
		stationDevices, stationSerials,
	)

	fd, err := f.locator().usbfsFD("23485031")
	if err != nil {
		t.Fatalf("usbfsFD = %v", err)
	}

	if target, _ := os.Readlink(f.link(fd)); target != cameraNode {
		t.Errorf("usbfsFD = %d, a duplicate of %q, want one of the camera", fd, target)
	}

	if !slices.Equal(f.open, []int{fd}) {
		t.Errorf("open duplicates = %v, want only the returned one", f.open)
	}
}

// What happened on terminal2210004: ReadDir listed /proc/self/fd through
// descriptor 43 and closed it, the camera was 27, and its duplicate became
// 43. Reading link 43 only after that found the camera a second time, through
// the scan's own duplicate, and refused it as ambiguous.
func TestUSBFSDoesNotCountItsOwnDuplicate(t *testing.T) {
	f := newFakeFDTable(t,
		map[string]string{"27": cameraNode, "43": "/proc/1234/fd"},
		stationDevices, stationSerials,
	)
	f.dupFrom = 43
	f.closedAfterListing = []string{"43"}

	fd, err := f.locator().usbfsFD("23485031")
	if err != nil {
		t.Fatalf("usbfsFD = %v", err)
	}

	if fd != 43 || !slices.Equal(f.open, []int{43}) {
		t.Errorf("usbfsFD = %d with %v open, want the one duplicate 43", fd, f.open)
	}
}

func TestUSBFSRefusesNoAndTwoMatches(t *testing.T) {
	tests := []struct {
		name    string
		serial  string
		wantErr error
	}{
		{name: "no device carries the serial", serial: "00000000", wantErr: errUSBFSNotFound},
		// Two descriptors of their own on the same device: the camera opened
		// twice. Mapping on the wrong one would silently keep the kernel
		// copy, so neither is picked.
		{name: "two descriptors carry it", serial: "23485031", wantErr: errUSBFSAmbiguous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFDTable(t,
				map[string]string{"7": cameraNode, "8": cameraNode},
				stationDevices, stationSerials,
			)

			fd, err := f.locator().usbfsFD(tt.serial)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("usbfsFD = %d, %v, want %v", fd, err, tt.wantErr)
			}

			if len(f.open) != 0 {
				t.Errorf("duplicates left open: %v", f.open)
			}
		})
	}
}

// A descriptor closed after the listing is no concern of the scan's, but
// running out of descriptors is: reporting that as "no device file" would send
// whoever reads the log looking for the wrong problem.
func TestUSBFSDuplicateFailures(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "closed in the meantime", err: syscall.EBADF},
		{name: "out of descriptors", err: syscall.EMFILE, wantErr: syscall.EMFILE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The camera matches first, so a failure afterwards has an open
			// duplicate to release.
			f := newFakeFDTable(t,
				map[string]string{"7": cameraNode, "8": pedalNode},
				stationDevices, stationSerials,
			)
			f.failDup = map[int]error{8: tt.err}

			fd, err := f.locator().usbfsFD("23485031")
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("usbfsFD = %v, want the camera", err)
				}
				if !slices.Equal(f.open, []int{fd}) {
					t.Errorf("open duplicates = %v, want only the returned one", f.open)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("usbfsFD = %d, %v, want %v", fd, err, tt.wantErr)
			}
			if len(f.open) != 0 {
				t.Errorf("duplicates left open: %v", f.open)
			}
		})
	}
}

// The duplicate is close-on-exec from the start; a child started by any
// goroutine must never inherit the camera.
func TestDupCloseOnExecSetsTheFlag(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	dup, err := dupCloseOnExec(int(f.Fd()))
	if err != nil {
		t.Fatalf("dupCloseOnExec = %v", err)
	}
	defer syscall.Close(dup)

	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(dup), syscall.F_GETFD, 0)
	if errno != 0 {
		t.Fatal(errno)
	}
	if flags&syscall.FD_CLOEXEC == 0 {
		t.Error("duplicate is not close-on-exec")
	}
}

// A camera without a serial number cannot be told apart from any other USB
// device, so the search does not even start.
func TestUSBFSNeedsASerial(t *testing.T) {
	f := newFakeFDTable(t, nil, nil, nil)

	if _, err := f.locator().usbfsFD(""); err == nil {
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
