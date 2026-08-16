package tests

import (
	"bytes"
	"fmt"
	"testing"
	"unsafe"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// TestBufferCreation checks that NewBuffer hands back a usable buffer across
// the size range, including zero. The old version discarded the result with
// `_ = buffer` and so could only fail if NewBuffer returned an error, never if
// it returned a nil buffer.
func TestBufferCreation(t *testing.T) {
	for _, size := range []uint{0, 1, 1024, 4096, 1 << 20} {
		t.Run(fmt.Sprintf("size %d", size), func(t *testing.T) {
			buffer, err := aravis.NewBuffer(size)
			if err != nil {
				t.Fatalf("NewBuffer(%d) returned error: %v", size, err)
			}

			if buffer.IsNil() {
				t.Errorf("NewBuffer(%d) returned a nil buffer", size)
			}
		})
	}
}

// TestFreshBufferContract pins what a buffer that has never been filled
// reports. It is the union of the old TestBufferDataAccess, TestBufferStatus,
// TestBufferChunks, TestBufferOperationsWithoutCamera and the buffer half of
// TestBoundaryConditions, none of which could fail.
//
// arv_buffer_get_data returns the *received* size, which stays zero until an
// acquisition fills the buffer — hence the empty payloads. The status is
// CLEARED, which the old switch did not even list and reported as "Unknown".
//
// GetPartWidth and GetPartHeight are deliberately not called here: on a part
// that is not an image they trip a GLib CRITICAL, which the old
// TestBufferMultipart provoked twice and reported as a pass. They are asserted
// on a seeded buffer instead, where part 0 really is an image.
func TestFreshBufferContract(t *testing.T) {
	for _, size := range []uint{0, 1, 1024, 4096, 1 << 20} {
		t.Run(fmt.Sprintf("size %d", size), func(t *testing.T) {
			buf, err := aravis.NewBuffer(size)
			if err != nil {
				t.Fatalf("NewBuffer(%d) returned error: %v", size, err)
			}

			if buf.IsNil() {
				t.Fatalf("NewBuffer(%d) returned a nil buffer", size)
			}

			if status, err := buf.GetStatus(); err != nil || status != aravis.BUFFER_STATUS_CLEARED {
				t.Errorf("GetStatus() = %d, %v; want %d (CLEARED), nil",
					status, err, aravis.BUFFER_STATUS_CLEARED)
			}

			if data, err := buf.GetData(); err != nil || len(data) != 0 {
				t.Errorf("GetData() = %d bytes, %v; want 0 bytes, nil", len(data), err)
			}

			if slice, err := buf.GetDataSlice(); err != nil || slice != nil {
				t.Errorf("GetDataSlice() = %v, %v; want nil, nil for an unfilled buffer", slice, err)
			}

			dest := make([]byte, 16)
			if n, err := buf.GetDataInto(dest); err != nil || n != 0 {
				t.Errorf("GetDataInto() = %d, %v; want 0, nil", n, err)
			}

			if _, n, err := buf.GetDataUnsafe(); err != nil || n != 0 {
				t.Errorf("GetDataUnsafe() reported size %d, %v; want 0, nil", n, err)
			}

			if buf.HasChunks() {
				t.Error("HasChunks() = true on a fresh buffer; want false")
			}
		})
	}
}

// TestBufferAccessorsAgreeOnSeededData is the test the suite was missing: the
// four ways of reaching a buffer's payload must return the same bytes.
//
// The version this replaces acquired a real frame and then compared exactly one
// byte — data[0] against dataSlice[0] — and logged everything else, so a
// wrapper that returned a truncated or misaligned payload would have passed.
func TestBufferAccessorsAgreeOnSeededData(t *testing.T) {
	buf, want := seededBuffer(t)

	data, err := buf.GetData()
	if err != nil {
		t.Fatalf("GetData() returned error: %v", err)
	}

	if !bytes.Equal(data, want) {
		t.Errorf("GetData() returned %d bytes that differ from the seeded payload of %d", len(data), len(want))
	}

	slice, err := buf.GetDataSlice()
	if err != nil {
		t.Fatalf("GetDataSlice() returned error: %v", err)
	}

	if !bytes.Equal(slice, want) {
		t.Errorf("GetDataSlice() returned %d bytes that differ from the seeded payload of %d", len(slice), len(want))
	}

	dest := make([]byte, len(want))

	n, err := buf.GetDataInto(dest)
	if err != nil {
		t.Fatalf("GetDataInto() returned error: %v", err)
	}

	if n != len(want) {
		t.Errorf("GetDataInto() = %d, want %d", n, len(want))
	}

	if !bytes.Equal(dest, want) {
		t.Error("GetDataInto() wrote bytes that differ from the seeded payload")
	}

	ptr, size, err := buf.GetDataUnsafe()
	if err != nil {
		t.Fatalf("GetDataUnsafe() returned error: %v", err)
	}

	if ptr == unsafe.Pointer(nil) {
		t.Fatal("GetDataUnsafe() returned a nil pointer for a filled buffer")
	}

	if size != len(want) {
		t.Fatalf("GetDataUnsafe() reported size %d, want %d", size, len(want))
	}

	// Checking the size and the pointer's non-nilness is not enough: a wrapper
	// handing back a correctly sized pointer at the wrong offset would satisfy
	// both. Compare what it actually points at.
	if !bytes.Equal(unsafe.Slice((*byte)(ptr), size), want) {
		t.Error("GetDataUnsafe() points at bytes that differ from the seeded payload")
	}
}

// TestBufferPartsOnSeededData covers the multipart accessors against a buffer
// whose single part is a real image.
//
// The old test ran them on a fresh NewBuffer, where the part is not an image:
// GetPartWidth and GetPartHeight each tripped
// "assertion 'arv_buffer_part_is_image (buffer, part_id)' failed", returned 0,
// and the test logged the 0 and passed.
func TestBufferPartsOnSeededData(t *testing.T) {
	buf, want := seededBuffer(t)

	numParts, err := buf.GetNumParts()
	if err != nil {
		t.Fatalf("GetNumParts() returned error: %v", err)
	}

	// The Fake camera sends a single-component Mono8 image.
	if numParts != 1 {
		t.Fatalf("GetNumParts() = %d, want 1", numParts)
	}

	partData, err := buf.GetPartData(0)
	if err != nil {
		t.Fatalf("GetPartData(0) returned error: %v", err)
	}

	if !bytes.Equal(partData, want) {
		t.Errorf("GetPartData(0) returned %d bytes that differ from the seeded payload of %d",
			len(partData), len(want))
	}

	width, err := buf.GetPartWidth(0)
	if err != nil {
		t.Fatalf("GetPartWidth(0) returned error: %v", err)
	}

	height, err := buf.GetPartHeight(0)
	if err != nil {
		t.Fatalf("GetPartHeight(0) returned error: %v", err)
	}

	// Mono8: one byte per pixel, so the geometry must account for the payload.
	if width*height != len(partData) {
		t.Errorf("GetPartWidth(0)*GetPartHeight(0) = %d*%d = %d, want %d bytes of part data",
			width, height, width*height, len(partData))
	}

	if componentID, err := buf.GetPartComponentId(0); err != nil || componentID != 0 {
		t.Errorf("GetPartComponentId(0) = %d, %v; want 0, nil", componentID, err)
	}

	if buf.HasChunks() {
		t.Error("HasChunks() = true; the Fake camera sends no chunk data")
	}
}
