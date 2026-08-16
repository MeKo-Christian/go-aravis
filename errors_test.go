package aravis

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// sentinels is the full set errors.go declares. Anything added there belongs
// here too: the properties below are what callers rely on when they reach for
// errors.Is.
func sentinels() map[string]error {
	return map[string]error{
		"ErrTimeout":          ErrTimeout,
		"ErrNoBuffer":         ErrNoBuffer,
		"ErrNegativeTimeout":  ErrNegativeTimeout,
		"ErrNilStream":        ErrNilStream,
		"ErrStreamClosed":     ErrStreamClosed,
		"ErrNilBuffer":        ErrNilBuffer,
		"ErrBufferNotOwned":   ErrBufferNotOwned,
		"ErrBufferAllocation": ErrBufferAllocation,
		"ErrPartOutOfRange":   ErrPartOutOfRange,
		"ErrPartNotImage":     ErrPartNotImage,
	}
}

// TestSentinelsAreUsableWithErrorsIs is the property the whole file exists for.
// The values these sentinels replaced were freshly allocated errors.New results
// created inside the failing call, so two timeouts were never the same error and
// errors.Is could not match either of them. A package-level value can be.
func TestSentinelsAreUsableWithErrorsIs(t *testing.T) {
	for name, err := range sentinels() {
		t.Run(name, func(t *testing.T) {
			if err == nil {
				t.Fatal("sentinel is nil")
			}

			if !errors.Is(err, err) { //nolint:err113 // the point is the identity itself
				t.Error("errors.Is does not match the sentinel against itself")
			}

			// Callers must still match it after the package has added context,
			// which is how the part checks report the offending index.
			wrapped := fmt.Errorf("some context: %w", err)
			if !errors.Is(wrapped, err) {
				t.Error("errors.Is does not match through a %w wrap")
			}
		})
	}
}

// TestSentinelsAreDistinct guards against a copy-paste that gives two sentinels
// the same value, which would make errors.Is answer yes for the wrong one.
func TestSentinelsAreDistinct(t *testing.T) {
	all := sentinels()

	for name, err := range all {
		for otherName, other := range all {
			if name == otherName {
				continue
			}

			if errors.Is(err, other) {
				t.Errorf("%s and %s are the same error value", name, otherName)
			}
		}
	}
}

// TestSentinelMessagesCarryThePackagePrefix keeps the messages recognisable in
// a log that mixes them with GLib and Go runtime output. The rest of the
// package uses the same "aravis: " prefix for its own errors.
func TestSentinelMessagesCarryThePackagePrefix(t *testing.T) {
	for name, err := range sentinels() {
		if !strings.HasPrefix(err.Error(), "aravis: ") {
			t.Errorf("%s message %q does not start with \"aravis: \"", name, err.Error())
		}
	}
}
