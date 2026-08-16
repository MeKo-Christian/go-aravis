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

// Stream wraps an ArvStream, the object that receives image data from a
// camera. Create one with Camera.CreateStream.
//
// A stream owns two buffer queues. Buffers you allocate with NewBuffer and
// hand over with PushBuffer go into the input queue, where the receiving
// thread fills them; filled buffers move to the output queue and are taken out
// again with PopBuffer, TryPopBuffer or TimeoutPopBuffer. The usual pattern is
// to push a handful of buffers before starting acquisition and then to recycle
// each popped buffer with PushBuffer once its data has been read or copied.
// The stream owns every buffer it holds and releases them when it is closed.
//
// Stream is a value type and may be copied freely: every copy refers to the
// same underlying ArvStream and shares one close flag (see lifecycle.go), so
// Close is idempotent and unrefs the stream exactly once no matter how many
// copies are closed, from whichever goroutine. The zero value owns nothing and
// its Close is a no-op.
type Stream struct {
	stream *C.struct__ArvStream

	// closed is shared by every copy of this Stream, so the stream is unreffed
	// exactly once however many copies are closed. Nil for the zero value.
	closed *closeFlag
}

// PushBuffer hands a buffer to the stream's input queue so it can be filled
// with the next frame, wrapping arv_stream_push_buffer. The stream takes
// ownership of the buffer and releases it when the stream is closed.
//
// Push both freshly allocated buffers (from NewBuffer) and buffers you have
// finished reading after a pop. From the moment of the push the payload memory
// belongs to the stream again, so any slice or pointer obtained from
// Buffer.GetDataSlice or Buffer.GetDataUnsafe for that buffer becomes invalid
// here — copy the data out first if you still need it.
func (s *Stream) PushBuffer(b Buffer) {
	C.arv_stream_push_buffer(s.stream, b.buffer)
}

// PopBuffer takes the next filled buffer from the stream's output queue,
// wrapping arv_stream_pop_buffer. It blocks indefinitely until a buffer
// becomes available; there is no timeout and no way to cancel the wait other
// than stopping acquisition or destroying the stream, so a camera that never
// delivers a frame blocks the calling goroutine forever. Use
// TimeoutPopBuffer when you need a deadline, or TryPopBuffer to poll.
//
// The returned buffer remains owned by the stream: check Buffer.GetStatus,
// read the data, then return it with PushBuffer. A returned buffer may be nil
// (Buffer.IsNil) if the stream had nothing to hand out.
func (s *Stream) PopBuffer() (Buffer, error) {
	var b Buffer
	var err error

	b.buffer, err = C.arv_stream_pop_buffer(s.stream)

	return b, err
}

// TryPopBuffer takes the next filled buffer from the stream's output queue if
// one is already available, wrapping arv_stream_try_pop_buffer. This is the
// non-blocking accessor: it returns immediately, and when no buffer is ready
// it returns a nil Buffer (Buffer.IsNil reports true) rather than waiting.
// Always test the result with IsNil before using it.
//
// As with PopBuffer the buffer stays owned by the stream and should be
// returned with PushBuffer after use.
func (s *Stream) TryPopBuffer() (Buffer, error) {
	var b Buffer
	var err error

	b.buffer, err = C.arv_stream_try_pop_buffer(s.stream)

	return b, err
}

// TimeoutPopBuffer takes the next filled buffer from the stream's output
// queue, blocking for at most t, and wraps arv_stream_timeout_pop_buffer. It
// is a blocking call with a deadline, not a polling one — TryPopBuffer is the
// non-blocking accessor — and it is the usual choice in an acquisition loop
// that must stay responsive when a frame is dropped.
//
// Aravis takes the timeout in microseconds, so t is divided by 1000 (a
// time.Duration counts nanoseconds). The division truncates: any t below one
// microsecond becomes a timeout of 0, which makes the call return at once
// instead of waiting for the sub-microsecond interval that was asked for.
// Sub-millisecond values are also rounded down to whole microseconds.
//
// If no buffer arrives within the timeout, the call returns the zero Buffer
// together with an error stating that Aravis returned a null pointer; a
// timeout is therefore not distinguishable from other null results. On success
// the buffer stays owned by the stream and should be returned with PushBuffer
// after its status has been checked and its data read.
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

// SetPropertyLong sets an integer GObject property on the underlying stream,
// for example the GigE Vision tuning knobs "packet-timeout",
// "frame-retention", "packet-resend" or "socket-buffer-size" (all in
// microseconds where they denote a duration). It calls g_object_set through a
// small C wrapper.
//
// There is no error return and no validation: an unknown property name, or one
// whose type is not a long, only produces a GLib runtime warning on stderr and
// leaves the stream unchanged. Use SetPropertyDouble for floating point
// properties.
func (s *Stream) SetPropertyLong(property string, value int64) {
	cprop := C.CString(property)
	cvalue := C.long(value)
	C.arv_set_stream_property_long(s.stream, cprop, cvalue)
	C.free(unsafe.Pointer(cprop))
}

// SetPropertyDouble sets a floating point GObject property on the underlying
// stream, for example "packet-request-ratio". The value is widened to a C
// double and passed to g_object_set through a small C wrapper.
//
// There is no error return and no validation: an unknown property name, or one
// whose type is not a double, only produces a GLib runtime warning on stderr
// and leaves the stream unchanged. Use SetPropertyLong for integer properties.
func (s *Stream) SetPropertyDouble(property string, value float32) {
	cprop := C.CString(property)
	cvalue := C.double(value)
	C.arv_set_stream_property_double(s.stream, cprop, cvalue)
	C.free(unsafe.Pointer(cprop))
}

// IsNil reports whether the Stream holds no underlying ArvStream, which is the
// case for the zero value and for the stream returned by a failed
// Camera.CreateStream. Unlike IsClosed it says nothing about whether a real
// stream has already been released.
func (s *Stream) IsNil() bool {
	return s.stream == nil
}
