package tests

import (
	"errors"
	"fmt"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// partAccessor names one of the eight accessors that take a part index, so the
// range checks can be table-driven across all of them instead of asserting on
// whichever one a test happened to call.
type partAccessor struct {
	name string
	call func(buf aravis.Buffer, partIndex int) error
	// image records whether Aravis grants this accessor only for an image part,
	// which is the arv_buffer_part_is_image precondition.
	image bool
}

func partAccessors() []partAccessor {
	return []partAccessor{
		{name: "GetPartData", call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartData(i)

			return err
		}},
		{name: "GetPartComponentId", call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartComponentId(i)

			return err
		}},
		{name: "GetPartDataType", call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartDataType(i)

			return err
		}},
		{name: "GetPartPixelFormat", image: true, call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartPixelFormat(i)

			return err
		}},
		{name: "GetPartWidth", image: true, call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartWidth(i)

			return err
		}},
		{name: "GetPartHeight", image: true, call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartHeight(i)

			return err
		}},
		{name: "GetPartX", image: true, call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartX(i)

			return err
		}},
		{name: "GetPartY", image: true, call: func(b aravis.Buffer, i int) error {
			_, err := b.GetPartY(i)

			return err
		}},
	}
}

// TestPartAccessorsRejectOutOfRangeIndex covers all eight accessors against
// every index outside the buffer's single part. Aravis asserts
// `part_id < n_parts` internally, which logged a GLib CRITICAL and then
// returned 0 — a value indistinguishable from a real width or component id.
//
// A fresh buffer has exactly one part (arv_buffer_new sets n_parts = 1), so the
// interesting indices are -1, 1 and 2. The negative one is the case Go alone
// can catch: partIndex is an int and the C parameter is a guint, so -1 would
// have been converted to 4294967295.
func TestPartAccessorsRejectOutOfRangeIndex(t *testing.T) {
	buf := newTestBuffer(t)
	defer buf.Close()

	numParts := buf.GetNumParts()
	if numParts != 1 {
		t.Fatalf("GetNumParts() = %d on a fresh buffer, want 1", numParts)
	}

	for _, accessor := range partAccessors() {
		for _, partIndex := range []int{-1, numParts, numParts + 1} {
			t.Run(fmt.Sprintf("%s/index %d", accessor.name, partIndex), func(t *testing.T) {
				if err := accessor.call(buf, partIndex); !errors.Is(err, aravis.ErrPartOutOfRange) {
					t.Errorf("%s(%d) = %v; want ErrPartOutOfRange", accessor.name, partIndex, err)
				}
			})
		}
	}
}

// TestGeometryAccessorsRejectNonImagePart covers the other precondition. A
// fresh buffer's single part is in range but is not an image — it has not been
// acquired, its payload type is unknown and its data type is not one of the
// image types — so the five accessors guarded by arv_buffer_part_is_image used
// to log "assertion 'arv_buffer_part_is_image (buffer, part_id)' failed" and
// return 0.
//
// The three accessors that Aravis grants for any part in range must still
// succeed, or the check would be over-strict.
func TestGeometryAccessorsRejectNonImagePart(t *testing.T) {
	buf := newTestBuffer(t)
	defer buf.Close()

	for _, accessor := range partAccessors() {
		t.Run(accessor.name, func(t *testing.T) {
			err := accessor.call(buf, 0)

			if accessor.image {
				if !errors.Is(err, aravis.ErrPartNotImage) {
					t.Errorf("%s(0) on a fresh buffer = %v; want ErrPartNotImage", accessor.name, err)
				}

				return
			}

			if err != nil {
				t.Errorf("%s(0) on a fresh buffer = %v; want nil", accessor.name, err)
			}
		})
	}
}

// TestPartAccessorsRejectNilBuffer pins the last unguarded path: every one of
// these calls asserts ARV_IS_BUFFER inside Aravis, so the zero Buffer produced
// a CRITICAL per call.
func TestPartAccessorsRejectNilBuffer(t *testing.T) {
	var buf aravis.Buffer

	for _, accessor := range partAccessors() {
		t.Run(accessor.name, func(t *testing.T) {
			if err := accessor.call(buf, 0); !errors.Is(err, aravis.ErrNilBuffer) {
				t.Errorf("%s(0) on a zero Buffer = %v; want ErrNilBuffer", accessor.name, err)
			}
		})
	}
}

// TestPartAccessorsAcceptARealImagePart is the positive control for the
// allow-list: a genuinely acquired frame must satisfy every one of the eight
// accessors. Without it the checks above could all be passed by a guard that
// rejects everything, and an allow-list missing a data type Aravis accepts
// would go unnoticed.
func TestPartAccessorsAcceptARealImagePart(t *testing.T) {
	buf, _ := seededBuffer(t)

	for _, accessor := range partAccessors() {
		t.Run(accessor.name, func(t *testing.T) {
			if err := accessor.call(buf, 0); err != nil {
				t.Errorf("%s(0) on an acquired frame = %v; want nil", accessor.name, err)
			}
		})
	}
}
