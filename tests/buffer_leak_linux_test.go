package tests

// The _linux suffix is a build constraint, so this file simply does not exist
// on other platforms. That is deliberate: a t.Skip would have to be added to
// the CI skip guard and to the allowed-skip table in README.md, and this test
// is not skipped for a reason worth documenting there — /proc/self/statm just
// does not exist elsewhere.

import (
	"os"
	"strconv"
	"strings"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// virtualSizeBytes returns the process's total program size, the first field of
// /proc/self/statm, in bytes.
//
// Virtual size rather than RSS: arv_buffer_new g_mallocs its payload and never
// writes to it, so an unclosed buffer may not fault a single page in and would
// leave RSS unchanged. Address space, on the other hand, is reserved either
// way — and for allocations of this size glibc serves them with mmap and
// returns them to the kernel on free, so a released buffer really does give the
// address space back.
func virtualSizeBytes(tb testing.TB) uint64 {
	tb.Helper()

	statm, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		tb.Fatalf("reading /proc/self/statm: %v", err)
	}

	fields := strings.Fields(string(statm))
	if len(fields) == 0 {
		tb.Fatalf("/proc/self/statm is empty")
	}

	pages, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		tb.Fatalf("parsing /proc/self/statm field 0 %q: %v", fields[0], err)
	}

	return pages * uint64(os.Getpagesize())
}

// TestBufferCloseReleasesAddressSpace observes the leak itself rather than the
// bookkeeping around it.
//
// Aravis exposes no allocation counter, and a GObject reference count cannot be
// read once the last reference is dropped, so the measurement is indirect: each
// buffer g_mallocs a 1 MiB payload, and 256 unreleased buffers are 256 MiB of
// address space that /proc/self/statm reports. With Close doing its job the
// figure barely moves; with the g_object_unref commented out it moves by a
// quarter of a gigabyte, which no tolerance can absorb.
func TestBufferCloseReleasesAddressSpace(t *testing.T) {
	const (
		bufferSize = 1 << 20
		iterations = 256
		// A quarter of what a full leak would cost. Generous enough that
		// allocator behaviour, the race detector's shadow memory or another
		// goroutine's allocation cannot push it over, and nowhere near the
		// 256 MiB the leak produces.
		toleranceBytes = 64 << 20
	)

	allocateAndClose := func(n int) {
		for range n {
			buffer, err := aravis.NewBuffer(bufferSize)
			if err != nil {
				t.Fatalf("NewBuffer(%d) returned error: %v", bufferSize, err)
			}

			buffer.Close()
		}
	}

	// Warm up first, so that any one-off growth of the allocator's arenas is
	// not charged to the measured loop.
	allocateAndClose(iterations)

	before := virtualSizeBytes(t)

	allocateAndClose(iterations)

	after := virtualSizeBytes(t)

	if after > before+toleranceBytes {
		t.Errorf("virtual size grew by %d bytes over %d allocate/close cycles of %d bytes; "+
			"want at most %d. Buffer.Close is not releasing the payload",
			after-before, iterations, bufferSize, toleranceBytes)
	}
}
