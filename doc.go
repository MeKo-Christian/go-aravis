// Package aravis provides Go bindings for the Aravis 0.8 machine-vision library,
// giving access to GigE Vision and USB3 Vision cameras.
//
// The package is a cgo wrapper. It requires libaravis 0.8.25 or later with its
// development headers and locates them through pkg-config (aravis-0.8), so
// CGO_ENABLED=1 and a working C toolchain are mandatory. The 0.8.25 floor comes from
// [Buffer.FindComponent], which wraps arv_buffer_find_component; against an earlier
// 0.8 release the package fails to compile rather than degrading at runtime. There is
// no pure-Go fallback and no support for the older 0.6 series.
//
// # Acquiring a frame
//
// The call order matters, because Aravis maintains global discovery state and the
// stream owns the buffer queue:
//
//	aravis.UpdateDeviceList()          // populate the device list
//	id, _ := aravis.GetDeviceId(0)     // pick a device
//
//	camera, err := aravis.NewCamera(id)
//	if err != nil {
//		return err
//	}
//	defer camera.Close()
//
//	stream, err := camera.CreateStream()
//	if err != nil {
//		return err
//	}
//	defer stream.Close()
//
//	// Fill the acquisition queue before starting.
//	payloadSize, _ := camera.GetPayloadSize()
//	for range 5 {
//		buffer, err := aravis.NewBuffer(payloadSize)
//		if err != nil {
//			return err
//		}
//		stream.PushBuffer(buffer)
//	}
//
//	if err := camera.StartAcquisition(); err != nil {
//		return err
//	}
//	defer camera.StopAcquisition()
//
//	buffer, err := stream.TimeoutPopBuffer(time.Second)
//	if !buffer.IsNil() {
//		// Push back whatever we got, even on error: a popped buffer belongs to us,
//		// and nothing else will free it.
//		defer stream.PushBuffer(buffer)
//	}
//	if err != nil {
//		return err
//	}
//	if status, _ := buffer.GetStatus(); status == aravis.BUFFER_STATUS_SUCCESS {
//		data, _ := buffer.GetData()
//		_ = data
//	}
//
// A buffer popped from the stream must be pushed back. This is not merely about
// keeping the queue full: Aravis transfers ownership of a popped buffer to the caller,
// [Stream.Close] frees only the buffers still sitting in the stream's queues, and this
// package exposes no Buffer.Close. A popped buffer that is never pushed back therefore
// leaks, with no way to release it.
//
// The pop methods can also return a non-nil buffer together with a non-nil error, so
// test the buffer rather than the error before deciding whether to push it back.
//
// Always check [Buffer.GetStatus] before trusting pixel data: a buffer can be returned
// with missing packets or a timeout and still be non-nil.
//
// Call [Shutdown] to release Aravis's global state when the process is done with
// cameras entirely.
//
// # Value types and Close
//
// [Camera], [Stream] and [Device] are small structs wrapping a C pointer, and they are
// handed out and copied by value. Copies of one of these values all refer to the same
// underlying C object and share a single close flag, so Close is idempotent per
// underlying object rather than per Go value: closing two copies unrefs once, not twice.
//
// None of these types has a finalizer. On a freely copied value type a finalizer would
// unref while a copy is still live, which trades a leak for a crash, so cleanup is
// explicit. Every Camera, Stream and owned Device must be closed by the caller.
//
// Ownership of a [Device] depends on where it came from:
//
//   - [OpenDevice] transfers ownership. The caller must Close the result.
//   - [Camera.GetDevice] only borrows the camera's device. Do not Close it, and do not
//     use it after the camera is closed.
//
// [Buffer] has no Close. A buffer from [NewBuffer] that is pushed to a stream belongs to
// that stream from then on and is released when the stream is closed.
//
// # Pixel data
//
// [Buffer] offers four ways to reach the pixel data, trading safety against copies:
//
//   - [Buffer.GetData] copies the frame into a newly allocated slice. Safest, and
//     allocates a full frame every call.
//   - [Buffer.GetDataInto] copies into a caller-supplied slice and allocates nothing.
//     The right choice for a streaming loop.
//   - [Buffer.GetDataSlice] returns a slice aliasing the C buffer, with no copy.
//   - [Buffer.GetDataUnsafe] returns the raw pointer and length for C interop.
//
// The two aliasing accessors return memory owned by Aravis, and the slice stops being
// yours the moment the buffer goes back with [Stream.PushBuffer]: the stream may refill
// that buffer with the next frame immediately, so a retained alias first sees its
// contents change underneath it, and becomes a genuinely dangling pointer once the
// stream releases the buffer. Neither Go's garbage collector nor its race detector can
// see either hazard. Consume the data, or copy what you need to keep, before pushing
// the buffer back.
//
// [Buffer.GetDataInto] has no such constraint — the copy is yours and survives
// PushBuffer — but reusing one destination slice across iterations means each frame
// overwrites the last. Anything that retains a frame beyond the current iteration needs
// its own copy or its own destination.
//
// Cameras with multiple taps or spectral bands deliver multipart buffers; see
// [Buffer.GetNumParts] and the GetPart* accessors. [Buffer.HasChunks] reports whether a
// frame carries chunk metadata, though decoding the chunks is not wrapped.
//
// [BayerRG] adapts a raw RGGB frame to image.Image with nearest-neighbor demosaicing,
// which is enough to encode a preview and not intended as a quality debayer.
//
// # Errors
//
// Failures that Aravis reports through a GError become an [*AravisError], which carries
// the GLib message along with a Code drawn from the DEVICE_ERROR_* constants. Match
// those with errors.As:
//
//	var aerr *aravis.AravisError
//	if errors.As(err, &aerr) && aerr.Code == aravis.DEVICE_ERROR_TIMEOUT {
//		// ...
//	}
//
// Not every error is one of those, so errors.As is not guaranteed to match and a
// caller must handle the plain-error case too. Two other kinds reach callers today:
//
//   - The pop methods report an empty result as a plain error, so a timeout is not
//     distinguishable from a real failure.
//   - Many wrappers use cgo's two-result call form, whose second value is errno. errno
//     is not cleared by a successful call, so these can surface a non-nil plain error
//     even when nothing failed. This is a known defect, tracked as P6 in PLAN.md.
//
// Not every call reports GenICam failures at all. The generic feature getters on
// [Device] notably do not; their documentation says so individually.
//
// # Concurrency
//
// The package adds no locking of its own beyond the close flags and the control-lost
// handler registry. Aravis permits a stream's buffer queue to be driven from a
// different goroutine than the one configuring the camera, but concurrent use of a
// single Camera, Stream, Device or Buffer otherwise needs external synchronization.
//
// [Camera.SetControlLostHandler] callbacks are invoked on an Aravis thread, not on the
// goroutine that registered them.
//
// # GigE Vision notes
//
// GigE Vision cameras need jumbo frames to reach their rated throughput. Set the host
// interface to MTU 9000 and match it with [Camera.GVSetPacketSize]; a packet size above
// the path MTU results in silent packet loss that surfaces as
// BUFFER_STATUS_MISSING_PACKETS. [Camera.GVSetPacketDelay] throttles the camera when
// the host cannot keep up.
//
// The register and memory accessors on [Device] bypass GenICam entirely. They exist for
// vendor-specific features and debugging; prefer named feature access whenever it is
// available, and treat writes as capable of misconfiguring the camera.
package aravis
