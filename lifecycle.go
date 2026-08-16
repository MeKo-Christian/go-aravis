package aravis

import "sync/atomic"

// closeFlag decides which of a wrapper's copies gets to release the underlying
// C object.
//
// Camera, Stream and Device are value types that this package hands out by
// value, so callers copy them freely — a copy carries the same C pointer but
// is a separate Go value. A "have I been closed?" bool stored in the wrapper
// would therefore be per-copy, and closing two copies would unref the same
// object twice. Every copy shares one *closeFlag instead, so the reference is
// dropped exactly once no matter which copy is closed, and from whichever
// goroutine.
//
// A nil *closeFlag means "this value does not own anything": the zero value of
// a wrapper, or a borrowed handle such as the device behind Camera.GetDevice.
// claim reports false for it, so Close stays a no-op.
type closeFlag struct {
	closed atomic.Bool
}

func newCloseFlag() *closeFlag {
	return &closeFlag{}
}

// claim reports whether the caller is the one that must release the object. It
// returns true at most once per flag.
func (f *closeFlag) claim() bool {
	return f != nil && f.closed.CompareAndSwap(false, true)
}

// isClosed reports whether the object has already been released. A nil flag
// owns nothing and is never considered closed.
func (f *closeFlag) isClosed() bool {
	return f != nil && f.closed.Load()
}
