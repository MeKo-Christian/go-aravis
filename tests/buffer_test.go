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

			if status := buf.GetStatus(); status != aravis.BUFFER_STATUS_CLEARED {
				t.Errorf("GetStatus() = %d, want %d (CLEARED)",
					status, aravis.BUFFER_STATUS_CLEARED)
			}

			if data := buf.GetData(); len(data) != 0 {
				t.Errorf("GetData() = %d bytes, want 0", len(data))
			}

			if slice := buf.GetDataSlice(); slice != nil {
				t.Errorf("GetDataSlice() = %v, want nil for an unfilled buffer", slice)
			}

			dest := make([]byte, 16)
			if n := buf.GetDataInto(dest); n != 0 {
				t.Errorf("GetDataInto() = %d, want 0", n)
			}

			if _, n := buf.GetDataUnsafe(); n != 0 {
				t.Errorf("GetDataUnsafe() reported size %d, want 0", n)
			}

			if buf.HasChunks() {
				t.Error("HasChunks() = true on a fresh buffer; want false")
			}
		})
	}
}

// TestErrorlessAccessorsHandleNilBuffer covers the eight accessors that report
// no error at all, on the one receiver that has nothing to read: the zero
// Buffer.
//
// They used to hand the NULL straight to Aravis, where every one of them
// asserts ARV_IS_BUFFER — a GLib CRITICAL per call, and a zero return the
// caller could not tell from a real one. Dropping the error return took away
// the last channel through which that could have been reported, so each now
// answers from the Go side instead. This is the counterpart of
// TestPartAccessorsRejectNilBuffer, which covers the accessors that do have an
// error to return.
//
// The CRITICAL half of the claim is enforced by `make test-glib-clean`, not by
// the assertions below: a guard that was removed would still return zero here
// while logging a diagnostic.
func TestErrorlessAccessorsHandleNilBuffer(t *testing.T) {
	var buf aravis.Buffer

	if data := buf.GetData(); data != nil {
		t.Errorf("GetData() = %v on a zero Buffer; want nil", data)
	}

	if ptr, n := buf.GetDataUnsafe(); ptr != nil || n != 0 {
		t.Errorf("GetDataUnsafe() = %v, %d on a zero Buffer; want nil, 0", ptr, n)
	}

	if slice := buf.GetDataSlice(); slice != nil {
		t.Errorf("GetDataSlice() = %v on a zero Buffer; want nil", slice)
	}

	if n := buf.GetDataInto(make([]byte, 16)); n != 0 {
		t.Errorf("GetDataInto() = %d on a zero Buffer; want 0", n)
	}

	if status := buf.GetStatus(); status != aravis.BUFFER_STATUS_UNKNOWN {
		t.Errorf("GetStatus() = %d on a zero Buffer; want %d (UNKNOWN)",
			status, aravis.BUFFER_STATUS_UNKNOWN)
	}

	// Zero parts is what makes the part accessors' ErrPartOutOfRange
	// unreachable on a zero Buffer: they report ErrNilBuffer first.
	if n := buf.GetNumParts(); n != 0 {
		t.Errorf("GetNumParts() = %d on a zero Buffer; want 0", n)
	}

	if index := buf.FindComponent(0); index != -1 {
		t.Errorf("FindComponent(0) = %d on a zero Buffer; want -1 (not found)", index)
	}

	if buf.HasChunks() {
		t.Error("HasChunks() = true on a zero Buffer; want false")
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

	data := buf.GetData()
	if !bytes.Equal(data, want) {
		t.Errorf("GetData() returned %d bytes that differ from the seeded payload of %d", len(data), len(want))
	}

	slice := buf.GetDataSlice()
	if !bytes.Equal(slice, want) {
		t.Errorf("GetDataSlice() returned %d bytes that differ from the seeded payload of %d", len(slice), len(want))
	}

	dest := make([]byte, len(want))

	n := buf.GetDataInto(dest)
	if n != len(want) {
		t.Errorf("GetDataInto() = %d, want %d", n, len(want))
	}

	if !bytes.Equal(dest, want) {
		t.Error("GetDataInto() wrote bytes that differ from the seeded payload")
	}

	ptr, size := buf.GetDataUnsafe()
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

	numParts := buf.GetNumParts()

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
