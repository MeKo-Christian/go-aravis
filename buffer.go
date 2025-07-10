package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
import "C"

import (
	"reflect"
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
	var size int

	data, err := C.arv_buffer_get_data(
		b.buffer,
		(*C.size_t)(unsafe.Pointer(&size)),
	)

	return C.GoBytes(data, C.int(size)), err
}

// GetDataUnsafe returns a direct pointer to the buffer data for zero-copy access
// WARNING: The returned pointer is only valid until the buffer is freed or reused
// This is for high-performance applications that need to avoid memory copies
func (b *Buffer) GetDataUnsafe() (unsafe.Pointer, int, error) {
	var size int

	data, err := C.arv_buffer_get_data(
		b.buffer,
		(*C.size_t)(unsafe.Pointer(&size)),
	)
	if err != nil {
		return nil, 0, err
	}

	return unsafe.Pointer(data), size, nil
}

// GetDataSlice returns a Go slice that directly references the C buffer memory
// WARNING: The slice is only valid until the buffer is freed or reused
// This provides zero-copy access but requires careful memory management
func (b *Buffer) GetDataSlice() ([]byte, error) {
	var size int

	data, err := C.arv_buffer_get_data(
		b.buffer,
		(*C.size_t)(unsafe.Pointer(&size)),
	)
	if err != nil {
		return nil, err
	}

	// Create Go slice header that references C memory directly
	var slice []byte
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&slice))
	sliceHeader.Data = uintptr(unsafe.Pointer(data))
	sliceHeader.Len = size
	sliceHeader.Cap = size

	return slice, nil
}

// GetDataInto copies buffer data into the provided slice
// Returns the number of bytes copied. This avoids allocations when you
// have a pre-allocated buffer to receive the data
func (b *Buffer) GetDataInto(dest []byte) (int, error) {
	var size int

	data, err := C.arv_buffer_get_data(
		b.buffer,
		(*C.size_t)(unsafe.Pointer(&size)),
	)
	if err != nil {
		return 0, err
	}

	// Copy only what fits in destination buffer
	copySize := size
	if copySize > len(dest) {
		copySize = len(dest)
	}

	// Use C.GoBytes to copy the memory
	srcSlice := C.GoBytes(data, C.int(copySize))
	copy(dest, srcSlice)

	return copySize, nil
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
	var size int

	data, err := C.arv_buffer_get_part_data(
		b.buffer,
		C.guint(partIndex),
		(*C.size_t)(unsafe.Pointer(&size)),
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
