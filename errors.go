package aravis

// This file is deliberately free of cgo: it holds nothing but package-level
// error values, so the root package can test them without a C toolchain being
// involved in the test file itself (Go forbids `import "C"` in _test.go).

import "errors"

// Sentinel errors returned by this package for conditions Aravis does not
// report through a GError.
//
// Every one of them is a package-level value, so callers match them with
// errors.Is:
//
//	buffer, err := stream.TimeoutPopBuffer(time.Second)
//	switch {
//	case errors.Is(err, aravis.ErrTimeout):
//		// no frame arrived in time; not a failure
//	case err != nil:
//		return err
//	}
//
// Some of them are wrapped with additional context (the offending index, for
// example) before they are returned, which is why errors.Is is the right test
// and an == comparison is not.
//
// Failures Aravis itself reports, through a GError, are not in this list: they
// become an [*AravisError] and are matched with errors.As.
var (
	// ErrTimeout reports that [Stream.TimeoutPopBuffer] waited out its
	// deadline without a buffer arriving. It is the ordinary outcome of a
	// dropped frame, not a malfunction.
	ErrTimeout = errors.New("aravis: timed out waiting for a buffer")

	// ErrNoBuffer reports that [Stream.PopBuffer] came back empty. Since that
	// call blocks until a buffer exists, this only happens when Aravis rejects
	// the stream itself.
	ErrNoBuffer = errors.New("aravis: no buffer available")

	// ErrNegativeTimeout reports that a negative timeout was passed to
	// [Stream.TimeoutPopBuffer]. Aravis takes an unsigned microsecond count, so
	// a negative duration would convert to an enormous one and block for
	// roughly forever; that is a caller bug and is refused rather than clamped,
	// which would make it indistinguishable from a dropped frame.
	ErrNegativeTimeout = errors.New("aravis: timeout must not be negative")

	// ErrNilStream reports that a [Stream] method was called on a value holding
	// no ArvStream — the zero value, or the result of a failed
	// [Camera.CreateStream].
	ErrNilStream = errors.New("aravis: stream is nil")

	// ErrStreamClosed reports that a [Stream] method was called after
	// [Stream.Close]. Close drops the reference but leaves the pointer in
	// place, so the call would otherwise reach Aravis with a dangling pointer,
	// which no assertion inside the library catches.
	ErrStreamClosed = errors.New("aravis: stream is closed")

	// ErrNilBuffer reports that a [Buffer] holding no ArvBuffer was used — the
	// zero value, or the result of a failed [NewBuffer].
	ErrNilBuffer = errors.New("aravis: buffer is nil")

	// ErrBufferNotOwned reports that a buffer the caller no longer owns was
	// handed to [Stream.PushBuffer]. Ownership moves to the stream on a push
	// and is given up entirely by [Buffer.Close], so a second push of the same
	// buffer — through the original value or any copy of it — is a double free
	// and is refused.
	ErrBufferNotOwned = errors.New("aravis: buffer is no longer owned by the caller")

	// ErrBufferAllocation reports that arv_buffer_new returned NULL, which
	// means the payload allocation failed.
	ErrBufferAllocation = errors.New("aravis: buffer allocation failed")

	// ErrPartOutOfRange reports a part index outside 0..[Buffer.GetNumParts]-1.
	ErrPartOutOfRange = errors.New("aravis: buffer part index out of range")

	// ErrPartNotImage reports that a part accessor requiring image geometry was
	// called on a part that carries none. Aravis grants width, height, x, y and
	// the pixel format only for a successfully acquired buffer whose payload is
	// an image, extended chunk data or multipart, and whose part is a 2D or 3D
	// image, one of their planar variants, or a confidence map.
	ErrPartNotImage = errors.New("aravis: buffer part is not an image")
)
