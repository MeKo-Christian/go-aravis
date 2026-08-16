package aravis

import (
	"errors"
	"math"
	"testing"
	"time"
)

// TestTimeoutMicroseconds pins the pop-timeout conversion. TimeoutPopBuffer
// cannot assert most of these itself: a timeout that comes out too small
// returns at once and is indistinguishable from a dropped frame, and one that
// comes out too large blocks for longer than any test can wait. That is why the
// arithmetic is a plain Go function.
func TestTimeoutMicroseconds(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want uint64
	}{
		{"zero", 0, 0},
		{"one microsecond", time.Microsecond, 1},
		{"exact multiple", 5 * time.Microsecond, 5},
		{"one nanosecond rounds up", time.Nanosecond, 1},
		{"sub-microsecond rounds up", 500 * time.Nanosecond, 1},
		{"remainder rounds up", 1500 * time.Nanosecond, 2},
		{"one second", time.Second, 1_000_000},

		// The Duration range ends 807 ns past a microsecond boundary, so
		// MaxInt64 exercises both the rounding and the overflow at once.
		// Computing the rounding as (t + time.Microsecond - 1) overflows int64
		// here and yields a negative count, which converts to an unsigned
		// timeout of roughly 1.8e19 µs — the call would block for longer than
		// the requested 292 years rather than shorter.
		{"max duration", time.Duration(math.MaxInt64), math.MaxInt64/1000 + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeoutMicroseconds(tt.in)
			if err != nil {
				t.Fatalf("timeoutMicroseconds(%v) returned error: %v", tt.in, err)
			}

			if got != tt.want {
				t.Errorf("timeoutMicroseconds(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestTimeoutMicrosecondsRejectsNegative covers the caller-bug case at the
// level the arithmetic happens; tests/stream_pop_test.go covers that
// TimeoutPopBuffer surfaces it rather than blocking.
func TestTimeoutMicrosecondsRejectsNegative(t *testing.T) {
	for _, in := range []time.Duration{-time.Nanosecond, -time.Second, math.MinInt64} {
		got, err := timeoutMicroseconds(in)
		if !errors.Is(err, ErrNegativeTimeout) {
			t.Errorf("timeoutMicroseconds(%v) error = %v, want ErrNegativeTimeout", in, err)
		}

		if got != 0 {
			t.Errorf("timeoutMicroseconds(%v) = %d, want 0 alongside the error", in, got)
		}
	}
}

// TestTimeoutMicrosecondsNeverRoundsDownToZero is the property the rounding
// exists for: no positive duration may become a timeout of zero, because that
// turns a requested wait into an immediate return.
func TestTimeoutMicrosecondsNeverRoundsDownToZero(t *testing.T) {
	for _, in := range []time.Duration{1, 2, 499, 500, 999, time.Microsecond - 1} {
		got, err := timeoutMicroseconds(in)
		if err != nil {
			t.Fatalf("timeoutMicroseconds(%v) returned error: %v", in, err)
		}

		if got == 0 {
			t.Errorf("timeoutMicroseconds(%v) = 0; a positive wait must not become no wait", in)
		}
	}
}
