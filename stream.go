package aravis

// #cgo pkg-config: aravis-0.8
// #include <arv.h>
// #include <stdlib.h>
/*
void arv_set_stream_property_long(ArvStream *stream, char *property, long value) {
	g_object_set (stream, property, value, NULL);
}

void arv_set_stream_property_double(ArvStream *stream, char *property, double value) {
	g_object_set (stream, property, value, NULL);
}
*/
import "C"

import (
	"errors"
	"time"
	"unsafe"
)

type Stream struct {
	stream *C.struct__ArvStream

	// closed is shared by every copy of this Stream, so the stream is unreffed
	// exactly once however many copies are closed. Nil for the zero value.
	closed *closeFlag
}

func (s *Stream) PushBuffer(b Buffer) {
	C.arv_stream_push_buffer(s.stream, b.buffer)
}

func (s *Stream) PopBuffer() (Buffer, error) {
	var b Buffer
	var err error

	b.buffer, err = C.arv_stream_pop_buffer(s.stream)

	return b, err
}

func (s *Stream) TryPopBuffer() (Buffer, error) {
	var b Buffer
	var err error

	b.buffer, err = C.arv_stream_try_pop_buffer(s.stream)

	return b, err
}

func (s *Stream) TimeoutPopBuffer(t time.Duration) (Buffer, error) {
	var buf Buffer
	var err error

	buf.buffer, err = C.arv_stream_timeout_pop_buffer(s.stream, C.guint64(t/1000))

	if buf.buffer == nil {
		return Buffer{}, errors.New("aravis returned a null pointer")
	}

	return buf, err
}

// Close releases the underlying stream. It is safe to call Close more than
// once, and safe to call on any copy of the same Stream: the stream is
// unreffed exactly once. Neither this Stream nor any copy of it may be used
// afterwards.
func (s *Stream) Close() {
	if s.stream == nil || !s.closed.claim() {
		return
	}

	C.g_object_unref(C.gpointer(s.stream))
}

// IsClosed reports whether the stream has been released, by this value or by
// any copy of it.
func (s *Stream) IsClosed() bool {
	return s.stream == nil || s.closed.isClosed()
}

func (s *Stream) SetPropertyLong(property string, value int64) {
	cprop := C.CString(property)
	cvalue := C.long(value)
	C.arv_set_stream_property_long(s.stream, cprop, cvalue)
	C.free(unsafe.Pointer(cprop))
}

func (s *Stream) SetPropertyDouble(property string, value float32) {
	cprop := C.CString(property)
	cvalue := C.double(value)
	C.arv_set_stream_property_double(s.stream, cprop, cvalue)
	C.free(unsafe.Pointer(cprop))
}

func (s *Stream) IsNil() bool {
	return s.stream == nil
}
