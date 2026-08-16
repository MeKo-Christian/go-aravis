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

// check reports whether this Stream may be handed to Aravis. The buffer-queue
// methods call it first: arv_stream_push_buffer and the three pops all assert
// ARV_IS_STREAM, which on a NULL pointer logs a GLib CRITICAL and turns a pop
// into a silent empty result. A closed stream is rejected too — Close drops the
// reference but leaves the pointer in place, so the call would otherwise reach
// Aravis with a dangling pointer, which nothing inside the library catches.
func (s *Stream) check() error {
	if s == nil || s.stream == nil {
		return ErrNilStream
	}

	if s.closed.isClosed() {
		return ErrStreamClosed
	}

	return nil
}

// PushBuffer hands a buffer to the stream's input queue so it can be filled
// with the next frame, wrapping arv_stream_push_buffer. The stream takes
// ownership of the buffer and releases it, along with every other buffer still
// in its queues, when the stream is closed. This is the only way to give a
// buffer back: a buffer the caller holds is never freed by Stream.Close.
//
// Push both freshly allocated buffers (from NewBuffer) and buffers you have
// finished reading after a pop. From the moment of the push the payload memory
// belongs to the stream again, which may refill it with the next frame at any
// time, so any slice or pointer obtained from Buffer.GetDataSlice or
// Buffer.GetDataUnsafe for that buffer must not be used past this call — copy
// the data out first if you still need it.
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
// Aravis transfers ownership of the returned buffer to the caller, so it is
// yours until you hand it back: check Buffer.GetStatus, read the data, then
// return it with PushBuffer. Stream.Close frees only the buffers still in the
// stream's queues, so a popped buffer that is never pushed back leaks.
//
// Because the call waits for a buffer, an empty result can only mean Aravis
// rejected the stream; it is reported as ErrNoBuffer. A nil or closed stream
// returns ErrNilStream or ErrStreamClosed without calling Aravis at all.
func (s *Stream) PopBuffer() (Buffer, error) {
	if err := s.check(); err != nil {
		return Buffer{}, err
	}

	buffer := C.arv_stream_pop_buffer(s.stream)
	if buffer == nil {
		return Buffer{}, ErrNoBuffer
	}

	return Buffer{buffer: buffer}, nil
}

// TryPopBuffer takes the next filled buffer from the stream's output queue if
// one is already available, wrapping arv_stream_try_pop_buffer. This is the
// non-blocking accessor: it returns immediately, and when no buffer is ready
// it returns a nil Buffer (Buffer.IsNil reports true) rather than waiting.
//
// An empty output queue is the expected result of a poll, not a failure, so it
// yields a nil Buffer together with a nil error. Test the result with IsNil
// before using it. A nil or closed stream returns ErrNilStream or
// ErrStreamClosed.
//
// As with PopBuffer, ownership of a non-nil buffer passes to the caller, which
// must return it with PushBuffer after use, or it leaks.
func (s *Stream) TryPopBuffer() (Buffer, error) {
	if err := s.check(); err != nil {
		return Buffer{}, err
	}

	buffer := C.arv_stream_try_pop_buffer(s.stream)
	if buffer == nil {
		return Buffer{}, nil
	}

	return Buffer{buffer: buffer}, nil
}

// TimeoutPopBuffer takes the next filled buffer from the stream's output
// queue, blocking for at most t, and wraps arv_stream_timeout_pop_buffer. It
// is a blocking call with a deadline, not a polling one — TryPopBuffer is the
// non-blocking accessor — and it is the usual choice in an acquisition loop
// that must stay responsive when a frame is dropped.
//
// A timeout is reported as ErrTimeout together with the zero Buffer, so a
// dropped frame is distinguishable from a real failure:
//
//	buffer, err := stream.TimeoutPopBuffer(time.Second)
//	if errors.Is(err, aravis.ErrTimeout) {
//		// no frame this round; carry on
//	}
//
// t must not be negative: Aravis takes the timeout as an unsigned count of
// microseconds, so a negative duration would convert to an enormous one and
// block for roughly forever. That is refused with ErrNegativeTimeout rather
// than clamped to zero, which would make a caller bug look like a dropped
// frame. The conversion from nanoseconds rounds up, so a sub-microsecond t
// still waits rather than returning at once.
//
// On success Aravis transfers ownership of the buffer to the caller, which
// must return it with PushBuffer once its status has been checked and its data
// read — Stream.Close will not free a buffer that is still popped.
func (s *Stream) TimeoutPopBuffer(t time.Duration) (Buffer, error) {
	if err := s.check(); err != nil {
		return Buffer{}, err
	}

	if t < 0 {
		return Buffer{}, ErrNegativeTimeout
	}

	// Round up rather than truncate: rounding down turns a requested wait of
	// less than a microsecond into no wait at all, which is the shape of the
	// historical TimeoutPopBuffer(1000) = 1 µs defect. Rounding up can never do
	// that. Callers passing a microsecond or more see no difference.
	microseconds := (int64(t) + int64(time.Microsecond) - 1) / int64(time.Microsecond)

	buffer := C.arv_stream_timeout_pop_buffer(s.stream, C.guint64(microseconds))
	if buffer == nil {
		return Buffer{}, ErrTimeout
	}

	return Buffer{buffer: buffer}, nil
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
