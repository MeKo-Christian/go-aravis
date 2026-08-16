package tests

import (
	"errors"
	"sync"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// newTestBuffer allocates a buffer of a size big enough to be interesting and
// fails the test if Aravis refuses.
func newTestBuffer(tb testing.TB) aravis.Buffer {
	tb.Helper()

	const size = 1 << 20

	buffer, err := aravis.NewBuffer(size)
	if err != nil {
		tb.Fatalf("NewBuffer(%d) returned error: %v", size, err)
	}

	if buffer.IsNil() {
		tb.Fatalf("NewBuffer(%d) returned a nil buffer", size)
	}

	return buffer
}

// TestBufferCloseUnrefsOnceAcrossCopies is the Buffer version of the guarantee
// Camera, Stream and Device already had. Buffer is handed out by value, so a
// "closed" bool living in the wrapper would be per-copy and closing two copies
// would unref the same ArvBuffer twice.
func TestBufferCloseUnrefsOnceAcrossCopies(t *testing.T) {
	buffer := newTestBuffer(t)
	duplicate := buffer

	if buffer.IsClosed() {
		t.Error("a freshly allocated buffer reports IsClosed() = true")
	}

	buffer.Close()

	if !buffer.IsClosed() {
		t.Error("IsClosed() = false after Close(); want true")
	}

	if !duplicate.IsClosed() {
		t.Error("the copy reports IsClosed() = false; the close flag must be shared")
	}

	// Neither of these may unref a second time.
	duplicate.Close()
	buffer.Close()
}

// TestBufferCloseOnZeroValue covers the nil guard: an unguarded Close would
// call g_object_unref on a NULL pointer.
func TestBufferCloseOnZeroValue(t *testing.T) {
	var buffer aravis.Buffer

	if !buffer.IsClosed() {
		t.Error("the zero Buffer reports IsClosed() = false; it owns nothing")
	}

	buffer.Close()
	buffer.Close()
}

// TestPushedBufferIsNoLongerTheCallers pins the ownership hand-off. Aravis
// declares arv_stream_push_buffer's parameter transfer-ownership="full", so the
// stream owns the buffer from the push onwards and the Go value — along with
// every copy of it — must stop acting as an owner. Closing it would be a double
// free, and so would pushing it again.
func TestPushedBufferIsNoLongerTheCallers(t *testing.T) {
	stream := idleStream(t)

	buffer := newTestBuffer(t)
	duplicate := buffer

	if err := stream.PushBuffer(buffer); err != nil {
		t.Fatalf("PushBuffer() returned error: %v", err)
	}

	if !buffer.IsClosed() {
		t.Error("IsClosed() = false after the push; the stream owns the buffer now")
	}

	if !duplicate.IsClosed() {
		t.Error("the copy reports IsClosed() = false after the push")
	}

	// Both of these must be inert rather than unreffing a buffer the stream is
	// about to free when it is closed.
	buffer.Close()
	duplicate.Close()

	if err := stream.PushBuffer(buffer); !errors.Is(err, aravis.ErrBufferNotOwned) {
		t.Errorf("second PushBuffer() = %v; want ErrBufferNotOwned", err)
	}

	if err := stream.PushBuffer(duplicate); !errors.Is(err, aravis.ErrBufferNotOwned) {
		t.Errorf("PushBuffer() of the copy = %v; want ErrBufferNotOwned", err)
	}
}

// TestPushAfterCloseIsRejected is the other direction of the same invariant:
// Close gives the buffer up entirely, so a later push would hand Aravis memory
// that has already been freed.
func TestPushAfterCloseIsRejected(t *testing.T) {
	stream := idleStream(t)

	buffer := newTestBuffer(t)
	buffer.Close()

	if err := stream.PushBuffer(buffer); !errors.Is(err, aravis.ErrBufferNotOwned) {
		t.Errorf("PushBuffer() after Close() = %v; want ErrBufferNotOwned", err)
	}
}

// TestPushBufferRejectsBadArguments covers the three arguments that used to go
// straight to Aravis: a nil buffer and a nil stream each trip a GLib assertion
// there, and a closed stream is a dangling pointer nothing catches.
func TestPushBufferRejectsBadArguments(t *testing.T) {
	t.Run("nil buffer", func(t *testing.T) {
		stream := idleStream(t)

		var buffer aravis.Buffer

		if err := stream.PushBuffer(buffer); !errors.Is(err, aravis.ErrNilBuffer) {
			t.Errorf("PushBuffer(zero Buffer) = %v; want ErrNilBuffer", err)
		}
	})

	t.Run("nil stream", func(t *testing.T) {
		var stream aravis.Stream

		buffer := newTestBuffer(t)
		defer buffer.Close()

		if err := stream.PushBuffer(buffer); !errors.Is(err, aravis.ErrNilStream) {
			t.Errorf("PushBuffer() on a zero Stream = %v; want ErrNilStream", err)
		}

		// The rejected push must not have taken ownership: the buffer is still
		// the caller's to release.
		if buffer.IsClosed() {
			t.Error("a rejected push claimed the buffer anyway")
		}
	})

	t.Run("closed stream", func(t *testing.T) {
		camera := requireFakeCamera(t)
		defer camera.Close()

		stream, err := camera.CreateStream()
		if err != nil {
			t.Fatalf("CreateStream() returned error: %v", err)
		}

		stream.Close()

		buffer := newTestBuffer(t)
		defer buffer.Close()

		if err := stream.PushBuffer(buffer); !errors.Is(err, aravis.ErrStreamClosed) {
			t.Errorf("PushBuffer() on a closed stream = %v; want ErrStreamClosed", err)
		}
	})
}

// TestPoppedBufferIsClosable is the case that could not be expressed at all
// before: a popped buffer belongs to the caller, and until Buffer.Close existed
// the only way to release it was to push it back to a stream that was still
// alive.
//
// The acquisition here is deliberately hand-rolled rather than taken from
// seededBuffer, whose cleanup pushes its buffer back — this test must be the
// only thing that disposes of the frame.
func TestPoppedBufferIsClosable(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Fatalf("GetPayloadSize() returned error: %v", err)
	}

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	defer stream.Close()

	buffer, err := aravis.NewBuffer(payloadSize)
	if err != nil {
		t.Fatalf("NewBuffer(%d) returned error: %v", payloadSize, err)
	}

	if err := stream.PushBuffer(buffer); err != nil {
		t.Fatalf("PushBuffer() returned error: %v", err)
	}

	if err := camera.StartAcquisition(); err != nil {
		t.Fatalf("StartAcquisition() returned error: %v", err)
	}

	filled, popErr := stream.TimeoutPopBuffer(popTimeout)

	if err := camera.StopAcquisition(); err != nil {
		t.Errorf("StopAcquisition() returned error: %v", err)
	}

	if popErr != nil {
		t.Fatalf("TimeoutPopBuffer() returned error: %v", popErr)
	}

	// The pop mints a fresh ownership flag, so the popped value owns the buffer
	// again even though the very same ArvBuffer was pushed a moment ago.
	if filled.IsClosed() {
		t.Fatal("a popped buffer reports IsClosed() = true; the pop transfers ownership back")
	}

	filled.Close()

	if !filled.IsClosed() {
		t.Error("IsClosed() = false after closing a popped buffer")
	}

	// Idempotent here as everywhere else, and the stream must survive being
	// closed afterwards with one fewer buffer in its queues.
	filled.Close()
}

// TestBufferCloseIsRaceFree hammers one buffer with many closers, mirroring the
// concurrency test the control-lost handler registry has. It is meaningful only
// under -race, which is how `make test-unit` and `make test-coverage` run: the
// close flag is what turns 64 concurrent Close calls into exactly one unref.
func TestBufferCloseIsRaceFree(t *testing.T) {
	const closers = 64

	buffer := newTestBuffer(t)

	var wg sync.WaitGroup

	start := make(chan struct{})

	for range closers {
		wg.Add(1)

		// Each goroutine closes its own copy of the value, which is how callers
		// use it and which must still unref exactly once.
		go func(buffer aravis.Buffer) {
			defer wg.Done()

			<-start

			buffer.Close()
		}(buffer)
	}

	close(start)
	wg.Wait()

	if !buffer.IsClosed() {
		t.Error("IsClosed() = false after 64 concurrent Close calls")
	}
}
