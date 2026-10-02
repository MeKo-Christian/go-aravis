//go:build linux

package aravis

// This file is deliberately free of cgo: it finds the usbfs device file Aravis
// opened for a camera, which is plain file-system work, and keeping C out of it
// lets the tests drive it against a fake /proc and /sys.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// usbfsPrefix is where udev puts the usbfs device files libusb opens.
const usbfsPrefix = "/dev/bus/usb/"

// usbfsLocator finds, among the process's open file descriptors, the one usbfs
// device file that belongs to a camera. Its fields are the operating system
// interfaces it uses, so the tests can stand in a fake /proc and /sys.
type usbfsLocator struct {
	// fdDir lists the process's open descriptors as symlinks to what they
	// refer to.
	fdDir string
	// charDir resolves a character device's major:minor to its sysfs node,
	// which for a USB device holds the "serial" attribute.
	charDir string
	// dup duplicates a descriptor. The duplicate shares the open file
	// description, which is what makes a mapping on it count for Aravis's
	// descriptor too.
	dup func(fd int) (int, error)
	// close releases a duplicate that turned out not to match.
	close func(fd int) error
	// rdev reports the device number of the character device fd refers to,
	// and false when fd is not a character device.
	rdev func(fd int) (major, minor uint32, ok bool)
}

// systemUSBFS is the locator for the running process.
var systemUSBFS = usbfsLocator{
	fdDir:   "/proc/self/fd",
	charDir: "/sys/dev/char",
	dup:     dupCloseOnExec,
	close:   syscall.Close,
	rdev:    charDeviceNumber,
}

// errUSBFSNotFound and errUSBFSAmbiguous are the two ways the search can come
// up empty; usbfsFD wraps them with the serial it looked for.
var (
	errUSBFSNotFound  = errors.New("no open usbfs device file carries this serial")
	errUSBFSAmbiguous = errors.New("more than one open usbfs device file carries this serial")
)

// usbfsFD returns a duplicate of the one open usbfs descriptor whose USB
// device reports serial, which the caller must close.
//
// Aravis keeps its libusb handle private, so the descriptor is found from the
// outside: every descriptor linked to /dev/bus/usb/ is duplicated, and the
// duplicate - not the original number, which another goroutine could close and
// reuse in the meantime - is checked against the device's sysfs serial. More
// than one match is refused rather than guessed at, because a mapping on the
// wrong device's descriptor is silently useless.
func (l usbfsLocator) usbfsFD(serial string) (int, error) {
	if serial == "" {
		return -1, errors.New("camera reports no serial number to match its usbfs device file by")
	}

	candidates, err := l.usbfsCandidates()
	if err != nil {
		return -1, err
	}

	found, matches := -1, 0
	// The duplicates made here. A candidate number that turns up among them was
	// closed by someone else after the listing and reused by one of ours, and
	// would count the same device twice.
	ours := map[int]bool{}

	for _, fd := range candidates {
		if ours[fd] {
			continue
		}

		dup, err := l.dup(fd)
		if err != nil {
			// Closed between the listing and now: not ours to worry about.
			continue
		}

		ours[dup] = true

		if l.serialOf(dup) != serial {
			_ = l.close(dup)

			continue
		}

		matches++
		if found >= 0 {
			_ = l.close(found)
		}
		found = dup
	}

	switch {
	case matches == 0:
		return -1, fmt.Errorf("%w: %q", errUSBFSNotFound, serial)
	case matches > 1:
		_ = l.close(found)

		return -1, fmt.Errorf("%w: %q", errUSBFSAmbiguous, serial)
	}

	return found, nil
}

// usbfsCandidates lists the process's descriptors that refer to a usbfs
// device file.
//
// Every link is read before anything is duplicated. A duplicate takes the
// lowest free number, which can be one the listing still holds - the
// directory's own descriptor, which ReadDir has closed by then - and reading
// that link afterwards would find the camera again, through the scan's own
// duplicate. On terminal2210004 the camera was descriptor 27, the listing's 43,
// and the first duplicate became 43.
func (l usbfsLocator) usbfsCandidates() ([]int, error) {
	entries, err := os.ReadDir(l.fdDir)
	if err != nil {
		return nil, fmt.Errorf("list open descriptors: %w", err)
	}

	var candidates []int

	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		target, err := os.Readlink(filepath.Join(l.fdDir, entry.Name()))
		if err != nil || !strings.HasPrefix(target, usbfsPrefix) {
			continue
		}

		candidates = append(candidates, fd)
	}

	return candidates, nil
}

// serialOf reads the USB serial number of the device fd refers to, or "" when
// fd is not a character device or its device has no serial.
func (l usbfsLocator) serialOf(fd int) string {
	major, minor, ok := l.rdev(fd)
	if !ok {
		return ""
	}

	// The sysfs root and two numbers from fstat: nothing from outside the
	// process reaches this path.
	node := filepath.Join(l.charDir, fmt.Sprintf("%d:%d", major, minor), "serial")

	raw, err := os.ReadFile(filepath.Clean(node))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(raw))
}

// dupCloseOnExec duplicates fd with close-on-exec set, so a child process
// started while the duplicate is open does not inherit the camera.
func dupCloseOnExec(fd int) (int, error) {
	dup, err := syscall.Dup(fd)
	if err != nil {
		return -1, err
	}

	syscall.CloseOnExec(dup)

	return dup, nil
}

// charDeviceNumber reports the device number of the character device fd
// refers to.
func charDeviceNumber(fd int) (major, minor uint32, ok bool) {
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil || st.Mode&syscall.S_IFMT != syscall.S_IFCHR {
		return 0, 0, false
	}

	// The glibc encoding of dev_t, which is what the kernel reports: the
	// minor's low eight bits sit below the major, the rest above it.
	dev := uint64(st.Rdev) //nolint:unconvert // Rdev is uint64 on amd64 but not on every architecture
	major = uint32((dev>>8)&0xfff | (dev>>32)&^0xfff)
	minor = uint32(dev&0xff | (dev>>12)&^0xff)

	return major, minor, true
}
