package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
//
// // arv_buffer_get_data reports the size through an out-param. Passing a Go
// // pointer for it forces cgo to heap-allocate the local on every call, which
// // is the only allocation left in the hot GetDataInto path. Returning both
// // values in a struct keeps every pointer on the C side, so the accessors
// // below allocate nothing. (#cgo noescape would also work, but it requires a
// // Go 1.23 language version and this module still supports the 1.23 baseline.)
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
	"unsafe"
)

const (
	BUFFER_STATUS_UNKNOWN         = C.ARV_BUFFER_STATUS_UNKNOWN
	BUFFER_STATUS_SUCCESS         = C.ARV_BUFFER_STATUS_SUCCESS
	BUFFER_STATUS_CLEARED         = C.ARV_BUFFER_STATUS_CLEARED
	BUFFER_STATUS_TIMEOUT         = C.ARV_BUFFER_STATUS_TIMEOUT
	BUFFER_STATUS_MISSING_PACKETS = C.ARV_BUFFER_STATUS_MISSING_PACKETS
	BUFFER_STATUS_WRONG_PACKET_ID = C.ARV_BUFFER_STATUS_WRONG_PACKET_ID
	BUFFER_STATUS_SIZE_MISMATCH   = C.ARV_BUFFER_STATUS_SIZE_MISMATCH
	BUFFER_STATUS_FILLING         = C.ARV_BUFFER_STATUS_FILLING
	BUFFER_STATUS_ABORTED         = C.ARV_BUFFER_STATUS_ABORTED
)

type Buffer struct {
	buffer *C.struct__ArvBuffer
}

func NewBuffer(size uint) (Buffer, error) {
	var buf Buffer

	buffer, err := C.arv_buffer_new(C.size_t(size), nil)
	if err != nil || buffer == nil {
		return Buffer{nil}, err
	} else {
		buf.buffer = buffer
		return buf, err
	}
}

func (b *Buffer) GetData() ([]byte, error) {
	buf, err := C.arv_go_buffer_get_data(b.buffer)

	return C.GoBytes(buf.data, C.int(buf.size)), err
}

// GetDataUnsafe returns a direct pointer to the buffer data for zero-copy access
// WARNING: The returned pointer is only valid until the buffer is freed or reused
// This is for high-performance applications that need to avoid memory copies
func (b *Buffer) GetDataUnsafe() (unsafe.Pointer, int, error) {
	buf, err := C.arv_go_buffer_get_data(b.buffer)
	if err != nil {
		return nil, 0, err
	}

	return buf.data, int(buf.size), nil
}

// GetDataSlice returns a Go slice that directly references the C buffer memory
// WARNING: The slice is only valid until the buffer is freed or reused
// This provides zero-copy access but requires careful memory management
func (b *Buffer) GetDataSlice() ([]byte, error) {
	buf, err := C.arv_go_buffer_get_data(b.buffer)
	if err != nil {
		return nil, err
	}

	if buf.data == nil || buf.size == 0 {
		return nil, nil
	}

	// unsafe.Slice aliases the C buffer memory directly (zero-copy). The
	// returned slice is only valid until the buffer is freed or reused.
	return unsafe.Slice((*byte)(buf.data), int(buf.size)), nil
}

// GetDataInto copies buffer data into dest and returns the number of bytes
// copied, truncating to len(dest). It performs a single copy out of the C
// buffer and allocates nothing.
func (b *Buffer) GetDataInto(dest []byte) (int, error) {
	buf, err := C.arv_go_buffer_get_data(b.buffer)
	if err != nil {
		return 0, err
	}
	if buf.data == nil || buf.size == 0 || len(dest) == 0 {
		return 0, nil
	}

	n := int(buf.size)
	if n > len(dest) {
		n = len(dest)
	}
	return copy(dest, unsafe.Slice((*byte)(buf.data), n)), nil
}

func (b *Buffer) GetStatus() (int, error) {
	status, err := C.arv_buffer_get_status(b.buffer)
	return int(status), err
}

func (b *Buffer) IsNil() bool {
	return b.buffer == nil
}

// Multipart buffer support functions

func (b *Buffer) GetNumParts() (int, error) {
	numParts, err := C.arv_buffer_get_n_parts(b.buffer)
	return int(numParts), err
}

func (b *Buffer) GetPartData(partIndex int) ([]byte, error) {
	var size C.size_t

	data, err := C.arv_buffer_get_part_data(
		b.buffer,
		C.guint(partIndex),
		&size,
	)
	if err != nil {
		return nil, err
	}

	return C.GoBytes(data, C.int(size)), nil
}

func (b *Buffer) GetPartComponentId(partIndex int) (uint, error) {
	componentId := C.arv_buffer_get_part_component_id(
		b.buffer,
		C.guint(partIndex),
	)

	return uint(componentId), nil
}

func (b *Buffer) GetPartDataType(partIndex int) (int, error) {
	dataType := C.arv_buffer_get_part_data_type(
		b.buffer,
		C.guint(partIndex),
	)

	return int(dataType), nil
}

func (b *Buffer) GetPartPixelFormat(partIndex int) (uint, error) {
	pixelFormat := C.arv_buffer_get_part_pixel_format(
		b.buffer,
		C.guint(partIndex),
	)

	return uint(pixelFormat), nil
}

func (b *Buffer) FindComponent(componentId uint) (int, error) {
	partIndex := C.arv_buffer_find_component(b.buffer, C.guint(componentId))
	return int(partIndex), nil
}

// Chunk data support functions

func (b *Buffer) HasChunks() bool {
	hasChunks := C.arv_buffer_has_chunks(b.buffer)
	return hasChunks != 0
}

// Additional part information functions

func (b *Buffer) GetPartWidth(partIndex int) (int, error) {
	width := C.arv_buffer_get_part_width(b.buffer, C.guint(partIndex))
	return int(width), nil
}

func (b *Buffer) GetPartHeight(partIndex int) (int, error) {
	height := C.arv_buffer_get_part_height(b.buffer, C.guint(partIndex))
	return int(height), nil
}

func (b *Buffer) GetPartX(partIndex int) (int, error) {
	x := C.arv_buffer_get_part_x(b.buffer, C.guint(partIndex))
	return int(x), nil
}

func (b *Buffer) GetPartY(partIndex int) (int, error) {
	y := C.arv_buffer_get_part_y(b.buffer, C.guint(partIndex))
	return int(y), nil
}
