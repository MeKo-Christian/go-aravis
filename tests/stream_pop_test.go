package tests

import (
	"errors"
	"testing"
	"time"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// idleStream returns a stream on the Fake camera that no acquisition is
// feeding, which is what makes every pop below deterministic: the output queue
// stays empty for as long as the test needs it to.
func idleStream(t *testing.T) aravis.Stream {
	t.Helper()

	camera := requireFakeCamera(t)
	t.Cleanup(camera.Close)

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	t.Cleanup(stream.Close)

	return stream
}

// TestTimeoutPopBufferReportsTimeout is the point of the sentinel: a dropped
// frame must be distinguishable from a real failure. Before this change the
// call returned a freshly allocated errors.New, so errors.Is could never match
// it and every caller was left comparing strings or ignoring the difference.
func TestTimeoutPopBufferReportsTimeout(t *testing.T) {
	stream := idleStream(t)

	start := time.Now()

	buffer, err := stream.TimeoutPopBuffer(50 * time.Millisecond)
	if !errors.Is(err, aravis.ErrTimeout) {
		t.Errorf("TimeoutPopBuffer() error = %v; want ErrTimeout", err)
	}

	if !buffer.IsNil() {
		t.Error("TimeoutPopBuffer() returned a non-nil buffer on timeout")
	}

	// The deadline must actually be honoured, not truncated away: this is the
	// half of the contract that catches a conversion returning early.
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("TimeoutPopBuffer(50ms) returned after %v; it must wait out the deadline", elapsed)
	}
}

// TestTimeoutPopBufferRoundsSubMicrosecondUp covers the conversion. A duration
// below one microsecond used to truncate to a timeout of zero, so the call
// returned at once instead of waiting for the interval that was asked for —
// the same class of defect as the historical TimeoutPopBuffer(1000) = 1 µs bug.
// Rounding up can never turn a requested wait into no wait.
func TestTimeoutPopBufferRoundsSubMicrosecondUp(t *testing.T) {
	stream := idleStream(t)

	buffer, err := stream.TimeoutPopBuffer(500 * time.Nanosecond)
	if !errors.Is(err, aravis.ErrTimeout) {
		t.Errorf("TimeoutPopBuffer(500ns) error = %v; want ErrTimeout", err)
	}

	if !buffer.IsNil() {
		t.Error("TimeoutPopBuffer(500ns) returned a non-nil buffer")
	}
}

// TestTimeoutPopBufferRejectsNegativeTimeout pins the caller-bug case. Aravis
// takes an unsigned microsecond count, so a negative time.Duration converted to
// an enormous one and the call blocked for roughly forever.
//
// The call therefore runs in a goroutine behind a deadline: without that, the
// pre-fix behaviour would hang the whole test binary instead of failing this
// one test. In the failing case that goroutine is parked in C code and leaks —
// which is acceptable, because by then the test has already failed and the run
// is over.
func TestTimeoutPopBufferRejectsNegativeTimeout(t *testing.T) {
	stream := idleStream(t)

	type result struct {
		buffer aravis.Buffer
		err    error
	}

	done := make(chan result, 1)

	go func() {
		buffer, err := stream.TimeoutPopBuffer(-time.Second)
		done <- result{buffer: buffer, err: err}
	}()

	select {
	case got := <-done:
		if !errors.Is(got.err, aravis.ErrNegativeTimeout) {
			t.Errorf("TimeoutPopBuffer(-1s) error = %v; want ErrNegativeTimeout", got.err)
		}

		if !got.buffer.IsNil() {
			t.Error("TimeoutPopBuffer(-1s) returned a non-nil buffer")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TimeoutPopBuffer(-1s) did not return within 2s; a negative duration is still " +
			"being converted to an enormous unsigned timeout")
	}
}

// TestTryPopBufferOnEmptyQueue pins the one pop whose empty result is not an
// error at all: polling an empty output queue is the expected outcome, so it
// yields a nil buffer and a nil error, and callers branch on IsNil.
func TestTryPopBufferOnEmptyQueue(t *testing.T) {
	stream := idleStream(t)

	buffer, err := stream.TryPopBuffer()
	if err != nil {
		t.Errorf("TryPopBuffer() on an empty queue returned error %v; want nil", err)
	}

	if !buffer.IsNil() {
		t.Error("TryPopBuffer() on an empty queue returned a non-nil buffer")
	}
}

// TestPopsRejectNilStream covers the guard that removes the GLib CRITICAL: all
// three pops asserted ARV_IS_STREAM inside Aravis, which logged a critical and
// then handed back an empty result indistinguishable from "no frame".
func TestPopsRejectNilStream(t *testing.T) {
	t.Run("PopBuffer", func(t *testing.T) {
		var stream aravis.Stream

		buffer, err := stream.PopBuffer()
		if !errors.Is(err, aravis.ErrNilStream) {
			t.Errorf("PopBuffer() error = %v; want ErrNilStream", err)
		}

		if !buffer.IsNil() {
			t.Error("PopBuffer() on a zero stream returned a non-nil buffer")
		}
	})

	t.Run("TryPopBuffer", func(t *testing.T) {
		var stream aravis.Stream

		buffer, err := stream.TryPopBuffer()
		if !errors.Is(err, aravis.ErrNilStream) {
			t.Errorf("TryPopBuffer() error = %v; want ErrNilStream", err)
		}

		if !buffer.IsNil() {
			t.Error("TryPopBuffer() on a zero stream returned a non-nil buffer")
		}
	})

	t.Run("TimeoutPopBuffer", func(t *testing.T) {
		var stream aravis.Stream

		buffer, err := stream.TimeoutPopBuffer(time.Millisecond)
		if !errors.Is(err, aravis.ErrNilStream) {
			t.Errorf("TimeoutPopBuffer() error = %v; want ErrNilStream", err)
		}

		if !buffer.IsNil() {
			t.Error("TimeoutPopBuffer() on a zero stream returned a non-nil buffer")
		}
	})
}

// TestPopsRejectClosedStream covers the other half of the guard. Close drops
// the reference but leaves the pointer in the struct, so an unguarded pop would
// reach Aravis with a dangling pointer — worse than NULL, because no assertion
// inside the library catches it.
func TestPopsRejectClosedStream(t *testing.T) {
	camera := requireFakeCamera(t)
	defer camera.Close()

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	stream.Close()

	if _, err := stream.TryPopBuffer(); !errors.Is(err, aravis.ErrStreamClosed) {
		t.Errorf("TryPopBuffer() on a closed stream = %v; want ErrStreamClosed", err)
	}

	if _, err := stream.TimeoutPopBuffer(time.Millisecond); !errors.Is(err, aravis.ErrStreamClosed) {
		t.Errorf("TimeoutPopBuffer() on a closed stream = %v; want ErrStreamClosed", err)
	}

	if _, err := stream.PopBuffer(); !errors.Is(err, aravis.ErrStreamClosed) {
		t.Errorf("PopBuffer() on a closed stream = %v; want ErrStreamClosed", err)
	}
}

// TestPopsDeliverUnderAcquisition is the positive control for every guard
// above: without it they could all pass by rejecting everything. Each of the
// three pops must hand back a real frame from a stream that is actually being
// fed.
func TestPopsDeliverUnderAcquisition(t *testing.T) {
	camera, _ := requireStreamingCamera(t)
	defer camera.Close()

	stream, payloadSize := setUpStream(t, camera)
	defer stream.Close()

	if err := camera.StartAcquisition(); err != nil {
		t.Fatalf("StartAcquisition() returned error: %v", err)
	}

	defer func() {
		if err := camera.StopAcquisition(); err != nil {
			t.Errorf("StopAcquisition() returned error: %v", err)
		}
	}()

	t.Logf("acquiring frames of %d bytes", payloadSize)

	// Blocking pop: the acquisition is running, so this must return a frame
	// rather than ErrNoBuffer.
	buffer, err := stream.PopBuffer()
	if err != nil {
		t.Fatalf("PopBuffer() under acquisition returned error: %v", err)
	}

	if buffer.IsNil() {
		t.Fatal("PopBuffer() under acquisition returned a nil buffer")
	}

	pushBack(t, stream, buffer)

	// Deadline pop: a Fake frame is due every 40 ms.
	buffer, err = stream.TimeoutPopBuffer(popTimeout)
	if err != nil {
		t.Fatalf("TimeoutPopBuffer() under acquisition returned error: %v", err)
	}

	if buffer.IsNil() {
		t.Fatal("TimeoutPopBuffer() under acquisition returned a nil buffer")
	}

	pushBack(t, stream, buffer)

	// Polling pop: it may legitimately find the queue empty, so retry until a
	// frame has had time to arrive. What must never happen is a non-nil error.
	deadline := time.Now().Add(popTimeout)

	for {
		buffer, err = stream.TryPopBuffer()
		if err != nil {
			t.Fatalf("TryPopBuffer() under acquisition returned error: %v", err)
		}

		if !buffer.IsNil() {
			pushBack(t, stream, buffer)

			break
		}

		if time.Now().After(deadline) {
			t.Fatal("TryPopBuffer() never found a frame under acquisition")
		}

		time.Sleep(5 * time.Millisecond)
	}
}
