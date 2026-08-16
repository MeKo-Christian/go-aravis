package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
//
// // arv_buffer_get_data reports the size through an out-param. Passing a Go
// // pointer for it forces cgo to heap-allocate the local on every call, which
// // is the only allocation left in the hot GetDataInto path. Returning both
// // values in a struct keeps every pointer on the C side, so the accessors
// // below allocate nothing.
// //
// // #cgo noescape would fix the same allocation, but the Go 1.23 toolchain
// // rejects it ("#cgo noescape disabled until Go 1.23") even when go.mod
// // declares go 1.23 — its own version gate is off by one, so the directive
// // only works from Go 1.24 onwards. This wrapper needs no minimum version.
// typedef struct {
//     const void *data;
//     size_t size;
// } ArvGoBufferData;
//
// static ArvGoBufferData arv_go_buffer_get_data(ArvBuffer *buffer) {
//     ArvGoBufferData result;
//     result.size = 0;
//     result.data = arv_buffer_get_data(buffer, &result.size);
//     return result;
// }
import "C"

import (
	"fmt"
	"unsafe"
)

// Buffer status values, as returned by Buffer.GetStatus. Only
// BUFFER_STATUS_SUCCESS means the buffer holds a complete, trustworthy image:
// always check GetStatus before reading pixel data out of a buffer popped from
// a stream, because a failed acquisition still hands back a buffer whose
// contents are stale, partial or undefined.
const (
	// BUFFER_STATUS_UNKNOWN means the acquisition status is not known. A
	// freshly allocated buffer that has never been filled reports this.
	BUFFER_STATUS_UNKNOWN = C.ARV_BUFFER_STATUS_UNKNOWN
	// BUFFER_STATUS_SUCCESS means the buffer was filled completely and its
	// payload is valid. This is the only status for which the pixel data may
	// be trusted.
	BUFFER_STATUS_SUCCESS = C.ARV_BUFFER_STATUS_SUCCESS
	// BUFFER_STATUS_CLEARED means the buffer was cleared, i.e. it was
	// returned to the caller without having been filled.
	BUFFER_STATUS_CLEARED = C.ARV_BUFFER_STATUS_CLEARED
	// BUFFER_STATUS_TIMEOUT means the acquisition of this buffer timed out
	// before the payload was complete.
	BUFFER_STATUS_TIMEOUT = C.ARV_BUFFER_STATUS_TIMEOUT
	// BUFFER_STATUS_MISSING_PACKETS means packets of the payload were lost
	// and could not be resent, so the image has holes. Typically caused by
	// network or host-side packet loss on GigE Vision links.
	BUFFER_STATUS_MISSING_PACKETS = C.ARV_BUFFER_STATUS_MISSING_PACKETS
	// BUFFER_STATUS_WRONG_PACKET_ID means a packet arrived with an
	// unexpected identifier, so the payload could not be reassembled.
	BUFFER_STATUS_WRONG_PACKET_ID = C.ARV_BUFFER_STATUS_WRONG_PACKET_ID
	// BUFFER_STATUS_SIZE_MISMATCH means the received payload does not match
	// the size the buffer was allocated for, usually because the camera's
	// payload size changed after the buffers were created.
	BUFFER_STATUS_SIZE_MISMATCH = C.ARV_BUFFER_STATUS_SIZE_MISMATCH
	// BUFFER_STATUS_FILLING means the buffer is still being filled and its
	// contents are incomplete.
	BUFFER_STATUS_FILLING = C.ARV_BUFFER_STATUS_FILLING
	// BUFFER_STATUS_ABORTED means the acquisition of this buffer was
	// aborted, for example because the stream was stopped mid-frame.
	BUFFER_STATUS_ABORTED = C.ARV_BUFFER_STATUS_ABORTED
)

// Buffer wraps an ArvBuffer, the container Aravis fills with one acquired
// image (plus any chunk or multipart data that came with it).
//
// Buffer is a small value type holding a C pointer; copying it copies the
// pointer, not the payload. Ownership is worth following closely, because it
// moves back and forth between the caller and the stream:
//
//   - A buffer you allocate with NewBuffer starts out yours.
//   - Stream.PushBuffer transfers it to the stream. Stream.Close releases every
//     buffer still sitting in the stream's queues at that point.
//   - Stream.PopBuffer, Stream.TryPopBuffer and Stream.TimeoutPopBuffer transfer
//     it back to you. While a buffer is popped it is outside the stream's
//     queues, so Stream.Close will not free it.
//
// A buffer in your hands is released either by pushing it to a stream or by
// calling Close on it. Do exactly one of the two: a buffer that is neither
// pushed nor closed leaks, and one that is pushed twice, or closed after being
// pushed, would be freed twice. The owned flag below enforces that: a second
// push is refused with ErrBufferNotOwned and a superfluous Close does nothing,
// rather than either of them corrupting the heap.
//
// Branch on Buffer.IsNil rather than on the error when deciding whether there
// is a buffer to release: Stream.TryPopBuffer legitimately returns a nil buffer
// with a nil error when the output queue is empty.
type Buffer struct {
	buffer *C.struct__ArvBuffer

	// owned carries the caller's claim on the underlying ArvBuffer and is
	// shared by every copy of this Buffer, exactly as Device.owned is. Aravis
	// hands ownership back and forth — arv_stream_push_buffer takes it
	// (transfer-ownership="full" on the parameter) and the pops give it back
	// (transfer-ownership="full" on the result) — so a push claims this flag
	// and every pop mints a fresh one. The invariant is that at most one Go
	// Buffer value holds an unclaimed flag for a given ArvBuffer at a time,
	// which is what makes both Close and PushBuffer safe on a value that has
	// been copied around.
	//
	// Nil for the zero value, which owns nothing.
	owned *closeFlag
}

// ownedBuffer wraps an ArvBuffer whose ownership Aravis has just transferred to
// the caller, which is the case for every buffer coming out of a pop.
func ownedBuffer(buffer *C.struct__ArvBuffer) Buffer {
	return Buffer{buffer: buffer, owned: newCloseFlag()}
}

// Close releases the buffer, dropping the reference the caller holds. Use it
// for a buffer that will not be given back to a stream: one that NewBuffer
// created and nothing ever pushed, and one that was popped and is not being
// pushed back. Both of those used to leak, because Stream.Close frees only the
// buffers still sitting in the stream's queues.
//
// Close is safe to call more than once, and safe to call on any copy of the
// same Buffer: the reference is dropped exactly once. It does nothing for the
// zero value and for a buffer that has already been handed to a stream with
// Stream.PushBuffer — that one belongs to the stream now.
//
// Neither this Buffer nor any copy of it may be used afterwards, and neither
// may any slice or pointer obtained from GetDataSlice or GetDataUnsafe: Close
// frees the payload memory they alias, exactly as PushBuffer invalidates them
// by handing the memory back to the stream.
func (b *Buffer) Close() {
	if b.buffer == nil || !b.owned.claim() {
		return
	}

	C.g_object_unref(C.gpointer(b.buffer))
}

// IsClosed reports whether this buffer is no longer the caller's, which is the
// case once Close has released it or Stream.PushBuffer has handed it to a
// stream — through this value or through any copy of it. The zero value is
// never anybody's and reports true.
func (b *Buffer) IsClosed() bool {
	return b.buffer == nil || b.owned.isClosed()
}

// NewBuffer allocates a new buffer with room for size bytes of payload,
// wrapping arv_buffer_new. Size should be at least the camera's payload size
// (see Camera.GetPayloadSize), otherwise acquisition fails with
// BUFFER_STATUS_SIZE_MISMATCH.
//
// The returned buffer belongs to the caller. Hand it to a stream with
// Stream.PushBuffer, which takes over ownership, or release it with
// Buffer.Close; see Buffer for the ownership rules. arv_buffer_new has no error
// channel, so the only failure this can report is a NULL result, in which case
// a zero Buffer (IsNil reports true) is returned together with
// ErrBufferAllocation.
func NewBuffer(size uint) (Buffer, error) {
	buffer := C.arv_buffer_new(C.size_t(size), nil)
	if buffer == nil {
		return Buffer{}, ErrBufferAllocation
	}

	return ownedBuffer(buffer), nil
}

// GetData returns a copy of the buffer payload in a freshly allocated Go
// slice. This is the safe default: the returned slice is owned by the Go
// garbage collector and stays valid after the buffer is pushed back to the
// stream or the stream is closed.
//
// The copy costs one allocation and one memcpy of the whole payload per call.
// In an acquisition loop prefer GetDataInto, which reuses a destination slice,
// or GetDataSlice / GetDataUnsafe if you can respect their lifetime rules.
//
// Check GetStatus first: GetData returns whatever bytes are in the buffer even
// when acquisition failed.
//
// The returned error is always nil; the underlying Aravis call cannot report a
// failure.
func (b *Buffer) GetData() ([]byte, error) {
	buf := C.arv_go_buffer_get_data(b.buffer)

	return C.GoBytes(buf.data, C.int(buf.size)), nil
}

// GetDataUnsafe returns a pointer into the C payload memory together with its
// length in bytes. Nothing is copied and nothing is allocated.
//
// The pointer aliases memory owned by Aravis, and its lifetime is bounded by
// the buffer's turn in the stream: it becomes invalid the moment the buffer is
// handed back with Stream.PushBuffer, because the stream may start refilling
// it with the next frame right away. It is also invalid after the owning
// stream is closed. Read or copy everything you need before pushing the buffer
// back, and never retain the pointer past that point.
//
// Prefer GetDataSlice for a typed view of the same memory, or GetDataInto if
// you want an allocation-free copy you can keep.
//
// The returned error is always nil; the underlying Aravis call cannot report a
// failure.
func (b *Buffer) GetDataUnsafe() (unsafe.Pointer, int, error) {
	buf := C.arv_go_buffer_get_data(b.buffer)

	return buf.data, int(buf.size), nil
}

// GetDataSlice returns a []byte that aliases the C payload memory directly.
// No bytes are copied and no Go memory is allocated for the payload, which
// makes it the cheapest way to look at a frame. An empty payload yields a nil
// slice.
//
// The slice is backed by memory owned by Aravis, not by the Go garbage
// collector, and its lifetime is bounded by the buffer's turn in the stream:
// it becomes invalid the moment the buffer is handed back with
// Stream.PushBuffer, since the stream may immediately reuse the memory for the
// next frame, and it is likewise invalid after the owning stream is closed.
// Keeping the slice past that point yields silently corrupted data. Do not
// store it, do not pass it to code that outlives the current iteration, and
// use append or copy into your own slice if you need the bytes to survive.
//
// Use GetData for a self-contained copy, or GetDataInto to copy without
// allocating.
//
// The returned error is always nil; the underlying Aravis call cannot report a
// failure.
func (b *Buffer) GetDataSlice() ([]byte, error) {
	buf := C.arv_go_buffer_get_data(b.buffer)

	if buf.data == nil || buf.size == 0 {
		return nil, nil
	}

	// unsafe.Slice aliases the C buffer memory directly (zero-copy). The
	// returned slice is only valid until the buffer is freed or reused.
	return unsafe.Slice((*byte)(buf.data), int(buf.size)), nil
}

// GetDataInto copies the buffer payload into the caller-supplied dest and
// returns the number of bytes copied, which is min(payload size, len(dest)) —
// a dest shorter than the payload silently truncates, so size it from
// Camera.GetPayloadSize if you need the whole frame. A nil or empty dest, or
// an empty payload, copies nothing and returns 0.
//
// This is the accessor to use in an acquisition loop: it performs a single
// memcpy out of the C buffer and allocates nothing at all, verified by
// TestGetDataIntoZeroAllocations in tests/buffer_data_test.go via
// testing.AllocsPerRun. Unlike GetDataSlice and GetDataUnsafe the resulting
// bytes live in dest, so they remain valid after the buffer is pushed back to
// the stream; unlike GetData no new slice is allocated per frame.
//
// The returned error is always nil; the underlying Aravis call cannot report a
// failure.
func (b *Buffer) GetDataInto(dest []byte) (int, error) {
	buf := C.arv_go_buffer_get_data(b.buffer)
	if buf.data == nil || buf.size == 0 || len(dest) == 0 {
		return 0, nil
	}

	n := int(buf.size)
	if n > len(dest) {
		n = len(dest)
	}
	return copy(dest, unsafe.Slice((*byte)(buf.data), n)), nil
}

// GetStatus returns the acquisition status of the buffer, one of the
// BUFFER_STATUS_* constants. It wraps arv_buffer_get_status.
//
// Every buffer popped from a stream must be checked with GetStatus before its
// payload is used: a stream hands back buffers for failed acquisitions too,
// and only BUFFER_STATUS_SUCCESS means the pixel data is complete and
// trustworthy. Anything else (BUFFER_STATUS_MISSING_PACKETS and
// BUFFER_STATUS_TIMEOUT are the common ones in practice) indicates a frame
// that should be discarded rather than decoded.
//
// The returned error is always nil; the underlying Aravis call cannot report a
// failure. The acquisition outcome is the status value itself.
func (b *Buffer) GetStatus() (int, error) {
	return int(C.arv_buffer_get_status(b.buffer)), nil
}

// IsNil reports whether the buffer holds no underlying ArvBuffer. This is true
// for the zero Buffer and for the value returned by a failed NewBuffer. Calling
// any other method on such a buffer passes a NULL pointer to Aravis.
func (b *Buffer) IsNil() bool {
	return b.buffer == nil
}

// GetNumParts returns the number of parts contained in the buffer, wrapping
// arv_buffer_get_n_parts. Ordinary single-image buffers have exactly one part;
// multipart payloads, as produced by multi-tap, 3D or multi-spectral cameras,
// have several, each with its own component id, pixel format and geometry.
//
// Valid part indices for the other Get Part accessors are 0 to the returned
// count minus one.
//
// The returned error is always nil; the underlying Aravis call cannot report a
// failure.
func (b *Buffer) GetNumParts() (int, error) {
	return int(C.arv_buffer_get_n_parts(b.buffer)), nil
}

// checkPart reports whether partIndex may be handed to Aravis. The part
// accessors assert `part_id < n_parts` internally, which for an out-of-range
// index logs a GLib CRITICAL and returns 0 — a value the caller cannot
// distinguish from a real one.
func (b *Buffer) checkPart(partIndex int) error {
	if b.buffer == nil {
		return ErrNilBuffer
	}

	numParts := int(C.arv_buffer_get_n_parts(b.buffer))
	if partIndex < 0 || partIndex >= numParts {
		return fmt.Errorf("%w: part %d of %d", ErrPartOutOfRange, partIndex, numParts)
	}

	return nil
}

// checkImagePart reports whether the part at partIndex carries image geometry.
//
// The five accessors that need it (width, height, x, y and the pixel format)
// are guarded inside Aravis by arv_buffer_part_is_image, which is static in
// arvbuffer.c and absent from the public header, so its condition is
// reproduced here: the buffer must have been acquired successfully, its payload
// must be an image, extended chunk data or multipart, and the part's data type
// must be one Aravis recognises as an image.
//
// The data types are an allow-list rather than a deny-list on purpose. A data
// type this list does not know — a future addition to the enumeration — is then
// rejected with a Go error instead of reaching Aravis and producing a CRITICAL.
// The list is a transcription of arv_buffer_part_is_image in Aravis 0.8.30.
func (b *Buffer) checkImagePart(partIndex int) error {
	if err := b.checkPart(partIndex); err != nil {
		return err
	}

	if int(C.arv_buffer_get_status(b.buffer)) != int(C.ARV_BUFFER_STATUS_SUCCESS) {
		return fmt.Errorf("%w: part %d of a buffer that was not acquired successfully",
			ErrPartNotImage, partIndex)
	}

	switch int(C.arv_buffer_get_payload_type(b.buffer)) {
	case int(C.ARV_BUFFER_PAYLOAD_TYPE_IMAGE),
		int(C.ARV_BUFFER_PAYLOAD_TYPE_EXTENDED_CHUNK_DATA),
		int(C.ARV_BUFFER_PAYLOAD_TYPE_MULTIPART):
	default:
		return fmt.Errorf("%w: part %d of a payload that carries no image", ErrPartNotImage, partIndex)
	}

	switch int(C.arv_buffer_get_part_data_type(b.buffer, C.guint(partIndex))) {
	case int(C.ARV_BUFFER_PART_DATA_TYPE_2D_IMAGE),
		int(C.ARV_BUFFER_PART_DATA_TYPE_2D_PLANE_BIPLANAR),
		int(C.ARV_BUFFER_PART_DATA_TYPE_2D_PLANE_TRIPLANAR),
		int(C.ARV_BUFFER_PART_DATA_TYPE_2D_PLANE_QUADPLANAR),
		int(C.ARV_BUFFER_PART_DATA_TYPE_3D_IMAGE),
		int(C.ARV_BUFFER_PART_DATA_TYPE_3D_PLANE_BIPLANAR),
		int(C.ARV_BUFFER_PART_DATA_TYPE_3D_PLANE_TRIPLANAR),
		int(C.ARV_BUFFER_PART_DATA_TYPE_3D_PLANE_QUADPLANAR),
		int(C.ARV_BUFFER_PART_DATA_TYPE_CONFIDENCE_MAP):
		return nil
	default:
		return fmt.Errorf("%w: part %d is not an image part", ErrPartNotImage, partIndex)
	}
}

// GetPartData returns a copy of the payload of the part at partIndex, wrapping
// arv_buffer_get_part_data. Like GetData it allocates a fresh Go slice, so the
// result stays valid after the buffer is pushed back to the stream; there is
// no zero-copy variant for parts.
//
// partIndex is checked against GetNumParts, and an index outside it yields
// ErrPartOutOfRange; a buffer holding no ArvBuffer yields ErrNilBuffer. An
// empty part yields a nil slice and a nil error.
func (b *Buffer) GetPartData(partIndex int) ([]byte, error) {
	if err := b.checkPart(partIndex); err != nil {
		return nil, err
	}

	var size C.size_t

	data := C.arv_buffer_get_part_data(
		b.buffer,
		C.guint(partIndex),
		&size,
	)
	if data == nil || size == 0 {
		return nil, nil
	}

	return C.GoBytes(data, C.int(size)), nil
}

// GetPartComponentId returns the GenICam component id of the part at
// partIndex, wrapping arv_buffer_get_part_component_id. The component id says
// what the part represents (intensity, disparity, confidence, and so on) and
// is the value FindComponent searches for.
//
// An index outside 0..GetNumParts-1 yields ErrPartOutOfRange, and a buffer
// holding no ArvBuffer ErrNilBuffer.
func (b *Buffer) GetPartComponentId(partIndex int) (uint, error) {
	if err := b.checkPart(partIndex); err != nil {
		return 0, err
	}

	componentId := C.arv_buffer_get_part_component_id(
		b.buffer,
		C.guint(partIndex),
	)

	return uint(componentId), nil
}

// GetPartDataType returns the data type of the part at partIndex as the
// numeric value of the ArvBufferPartDataType enumeration (2D image, 3D image,
// confidence map, chunk data, and so on), wrapping
// arv_buffer_get_part_data_type.
//
// An index outside 0..GetNumParts-1 yields ErrPartOutOfRange, and a buffer
// holding no ArvBuffer ErrNilBuffer.
func (b *Buffer) GetPartDataType(partIndex int) (int, error) {
	if err := b.checkPart(partIndex); err != nil {
		return 0, err
	}

	dataType := C.arv_buffer_get_part_data_type(
		b.buffer,
		C.guint(partIndex),
	)

	return int(dataType), nil
}

// GetPartPixelFormat returns the PFNC pixel format of the part at partIndex,
// wrapping arv_buffer_get_part_pixel_format. Different parts of the same
// buffer may use different formats, so decode each part according to its own
// value rather than the camera's current pixel format.
//
// Aravis grants the pixel format only for an image part, so this reports
// ErrPartNotImage for a part that carries none — including every part of a
// buffer that has not been filled by a successful acquisition. An index outside
// 0..GetNumParts-1 yields ErrPartOutOfRange, and a buffer holding no ArvBuffer
// ErrNilBuffer.
func (b *Buffer) GetPartPixelFormat(partIndex int) (uint, error) {
	if err := b.checkImagePart(partIndex); err != nil {
		return 0, err
	}

	pixelFormat := C.arv_buffer_get_part_pixel_format(
		b.buffer,
		C.guint(partIndex),
	)

	return uint(pixelFormat), nil
}

// FindComponent returns the index of the part carrying the given GenICam
// component id, or -1 if the buffer has no such component. It wraps
// arv_buffer_find_component (Aravis 0.8.25 and later) and is the convenient way
// to locate, say, the disparity or confidence part of a multipart payload
// without scanning GetPartComponentId over every part.
//
// The returned error is always nil; a missing component is reported through the
// -1 index, not through an error.
func (b *Buffer) FindComponent(componentId uint) (int, error) {
	partIndex := C.arv_buffer_find_component(b.buffer, C.guint(componentId))
	return int(partIndex), nil
}

// HasChunks reports whether the buffer's payload type carries GenICam chunk
// data, i.e. per-frame metadata such as timestamps, exposure or gain appended
// by the camera. It wraps arv_buffer_has_chunks.
//
// It only tells you that chunks are present, not what they contain; this
// binding exposes no accessor for the individual chunks yet.
func (b *Buffer) HasChunks() bool {
	hasChunks := C.arv_buffer_has_chunks(b.buffer)
	return hasChunks != 0
}

// GetPartWidth returns the width in pixels of the part at partIndex, wrapping
// arv_buffer_get_part_width. Together with GetPartHeight, GetPartX and
// GetPartY it describes the region the part covers.
//
// The four geometry accessors are granted by Aravis only for an image part, so
// each reports ErrPartNotImage for a part that carries no geometry — including
// every part of a buffer that has not been filled by a successful acquisition.
// An index outside 0..GetNumParts-1 yields ErrPartOutOfRange, and a buffer
// holding no ArvBuffer ErrNilBuffer.
func (b *Buffer) GetPartWidth(partIndex int) (int, error) {
	if err := b.checkImagePart(partIndex); err != nil {
		return 0, err
	}

	width := C.arv_buffer_get_part_width(b.buffer, C.guint(partIndex))

	return int(width), nil
}

// GetPartHeight returns the height in pixels of the part at partIndex,
// wrapping arv_buffer_get_part_height. See GetPartWidth for the errors it can
// report.
func (b *Buffer) GetPartHeight(partIndex int) (int, error) {
	if err := b.checkImagePart(partIndex); err != nil {
		return 0, err
	}

	height := C.arv_buffer_get_part_height(b.buffer, C.guint(partIndex))

	return int(height), nil
}

// GetPartX returns the horizontal offset of the part at partIndex within the
// sensor's coordinate system, wrapping arv_buffer_get_part_x. See GetPartWidth
// for the errors it can report.
func (b *Buffer) GetPartX(partIndex int) (int, error) {
	if err := b.checkImagePart(partIndex); err != nil {
		return 0, err
	}

	x := C.arv_buffer_get_part_x(b.buffer, C.guint(partIndex))

	return int(x), nil
}

// GetPartY returns the vertical offset of the part at partIndex within the
// sensor's coordinate system, wrapping arv_buffer_get_part_y. See GetPartWidth
// for the errors it can report.
func (b *Buffer) GetPartY(partIndex int) (int, error) {
	if err := b.checkImagePart(partIndex); err != nil {
		return 0, err
	}

	y := C.arv_buffer_get_part_y(b.buffer, C.guint(partIndex))

	return int(y), nil
}
