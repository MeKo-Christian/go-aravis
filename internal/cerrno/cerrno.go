// Package cerrno makes the "errno is not an error" defect observable from a
// test.
//
// cgo lets any C call be written in a two-result form, `v, err := C.f(...)`,
// whose second value is errno. errno is not a failure report: C sets it as a
// side effect of syscalls and never clears it on success, so binding it to a Go
// error turns unrelated noise into a reported failure. That is the defect P6
// removes from this binding; only a GError may decide that an Aravis call
// failed.
//
// The exact exposure is narrower than it looks, and worth writing down because
// it is easy to get wrong. Since Go 1.12 cgo emits `errno = 0` immediately
// before the call in the two-result form (golang.org/issue/28832), so an errno
// left behind by an *earlier* call cannot leak into a *later* one. What survives
// is errno set *during* the call, by any syscall the C function makes
// internally. Those syscalls fail benignly all the time — a recvfrom returning
// EAGAIN on a GigE Vision socket, a stat of a file that is not there during
// GenICam lookup — and the wrapper then reports a failure that did not happen.
//
// SucceedingCallTwoResult and SucceedingCallSingleResult below are the smallest
// possible instance of that: one C function, called both ways.
//
// Go forbids `import "C"` inside _test.go files, which is why this lives in a
// normal package. internal/ keeps it out of the module's public API.
package cerrno

// #include <errno.h>
// #include <fcntl.h>
// #include <unistd.h>
//
// // arv_go_succeed_after_failed_syscall stands in for any Aravis or GLib call
// // that succeeds while a syscall it made along the way did not. The open is
// // guaranteed to fail with ENOENT and to leave that in errno; the function
// // itself then succeeds and returns 42.
// static int arv_go_succeed_after_failed_syscall(void) {
//     int fd = open("/nonexistent/go-aravis/errno/probe", O_RDONLY);
//     if (fd >= 0) {
//         close(fd);
//     }
//     return 42;
// }
import "C"

// SuccessValue is what both calls below return. It is a plain success: neither
// form of the call failed.
const SuccessValue = 42

// SucceedingCallTwoResult invokes the helper through cgo's two-result form. The
// call always succeeds, yet the returned error is non-nil, because a syscall
// made inside the C function set errno. This is the defect, in one line.
func SucceedingCallTwoResult() (int, error) {
	value, err := C.arv_go_succeed_after_failed_syscall()

	return int(value), err
}

// SucceedingCallSingleResult invokes the same helper through cgo's single-result
// form, which is what every wrapper in this module uses. There is no error to
// get wrong.
func SucceedingCallSingleResult() int {
	return int(C.arv_go_succeed_after_failed_syscall())
}
