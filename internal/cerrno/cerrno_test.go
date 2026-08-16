package cerrno_test

// These tests demonstrate the defect the P6 error-contract fix removes from the
// binding. They deliberately assert the *broken* behaviour of cgo's two-result
// call form, because that is what justifies never using it: no wrapper in this
// module may report a failure that only errno saw.

import (
	"errors"
	"runtime"
	"syscall"
	"testing"

	"github.com/MeKo-Christian/go-aravis/internal/cerrno"
)

// TestTwoResultFormReportsASuccessAsAFailure is the whole argument in one test.
// The C function succeeds — it returns SuccessValue both ways — but the
// two-result form still hands back a non-nil error, purely because a syscall
// made inside the call failed and left errno set.
//
// A wrapper written that way returns that error to its caller as if the Aravis
// call had failed.
func TestTwoResultFormReportsASuccessAsAFailure(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	value, err := cerrno.SucceedingCallTwoResult()
	if value != cerrno.SuccessValue {
		t.Fatalf("SucceedingCallTwoResult() = %d, want %d; the helper did not succeed, "+
			"so this test would prove nothing", value, cerrno.SuccessValue)
	}

	if err == nil {
		t.Fatal("SucceedingCallTwoResult() returned a nil error; " +
			"the helper no longer leaves errno set, so the guard below is vacuous")
	}

	var errno syscall.Errno
	if !errors.As(err, &errno) {
		t.Fatalf("SucceedingCallTwoResult() returned %[1]T (%[1]v), want a syscall.Errno", err)
	}

	if !errors.Is(err, syscall.ENOENT) {
		t.Errorf("SucceedingCallTwoResult() returned %v, want ENOENT from the failed open", err)
	}
}

// TestSingleResultFormHasNoErrorToGetWrong is the counterpart: the same C
// function, called the way every wrapper in this module calls it, has no second
// value at all. Failure can then only be reported by something that actually
// knows about it — for Aravis, a GError.
func TestSingleResultFormHasNoErrorToGetWrong(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if value := cerrno.SucceedingCallSingleResult(); value != cerrno.SuccessValue {
		t.Fatalf("SucceedingCallSingleResult() = %d, want %d", value, cerrno.SuccessValue)
	}
}
