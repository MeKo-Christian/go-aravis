package aravis

import "time"

// timeoutMicroseconds converts a pop timeout into the unsigned microsecond
// count Aravis takes.
//
// It lives in a file of its own, free of cgo, so the root package's tests can
// exercise it directly. The conversion is where both of this call's historical
// defects lived, and neither is observable through TimeoutPopBuffer itself: a
// wrong result either returns immediately, which looks like a dropped frame, or
// blocks for longer than any test can wait.
func timeoutMicroseconds(t time.Duration) (uint64, error) {
	// A negative duration would convert to an enormous unsigned count and block
	// for roughly forever. It is refused rather than clamped to zero, which
	// would make a caller bug look like a dropped frame.
	if t < 0 {
		return 0, ErrNegativeTimeout
	}

	// Round up rather than truncate: rounding down turns a requested wait of
	// less than a microsecond into no wait at all, which is the shape of the
	// historical TimeoutPopBuffer(1000) = 1 µs defect. Rounding up can never do
	// that. Callers passing a microsecond or more see no difference.
	//
	// The rounding is applied to the quotient rather than by adding
	// time.Microsecond-1 to t first. That addition overflows int64 for any t
	// within the last microsecond of the Duration range, and the negative
	// result would convert to exactly the enormous timeout the check above
	// exists to prevent.
	microseconds := int64(t) / int64(time.Microsecond)
	if int64(t)%int64(time.Microsecond) != 0 {
		microseconds++
	}

	return uint64(microseconds), nil
}
