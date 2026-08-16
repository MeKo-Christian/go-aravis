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
//	id := aravis.GetDeviceId(0)        // pick a device
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
//		if err := stream.PushBuffer(buffer); err != nil {
//			buffer.Close()
//			return err
//		}
//	}
//
//	if err := camera.StartAcquisition(); err != nil {
//		return err
//	}
//	defer camera.StopAcquisition()
//
//	buffer, err := stream.TimeoutPopBuffer(time.Second)
//	if errors.Is(err, aravis.ErrTimeout) {
//		// No frame this round. Not a failure.
//		return nil
//	}
//	if err != nil {
//		return err
//	}
//	// A popped buffer belongs to us, so give it back: push it to keep the queue
//	// full, or Close it to release it.
//	defer func() {
//		if err := stream.PushBuffer(buffer); err != nil {
//			log.Printf("returning the buffer: %v", err)
//		}
//	}()
//
//	if buffer.GetStatus() == aravis.BUFFER_STATUS_SUCCESS {
//		data := buffer.GetData()
//		_ = data
//	}
//
// A buffer popped from the stream must be given back. This is not merely about keeping
// the queue full: Aravis transfers ownership of a popped buffer to the caller, and
// [Stream.Close] frees only the buffers still sitting in the stream's queues. Return it
// with [Stream.PushBuffer], or release it with [Buffer.Close] when the frame was the
// last one wanted; a buffer that gets neither leaks.
//
// Ownership moves in both directions, and the package tracks it. A push hands the buffer
// to the stream, which makes that Go value and every copy of it inert — a second push, or
// a Close after the push, is refused with [ErrBufferNotOwned] rather than freeing the
// buffer twice. Each pop mints a fresh Buffer value that owns the buffer again.
//
// Always check [Buffer.GetStatus] before trusting pixel data: a buffer can be returned
// with missing packets or a timeout and still be non-nil.
//
// Call [Shutdown] to release Aravis's global state when the process is done with
// cameras entirely.
//
// # Value types and Close
//
// [Camera], [Stream], [Device] and [Buffer] are small structs wrapping a C pointer, and
// they are handed out and copied by value. Copies of one of these values all refer to
// the same underlying C object and share a single close flag, so Close is idempotent per
// underlying object rather than per Go value: closing two copies unrefs once, not twice.
//
// None of these types has a finalizer. On a freely copied value type a finalizer would
// unref while a copy is still live, which trades a leak for a crash, so cleanup is
// explicit. Every Camera, Stream, owned Device and unpushed Buffer must be released by
// the caller.
//
// Ownership of a [Device] depends on where it came from:
//
//   - [OpenDevice] transfers ownership. The caller must Close the result.
//   - [Camera.GetDevice] only borrows the camera's device. Do not Close it, and do not
//     use it after the camera is closed.
//
// A [Buffer]'s close flag doubles as its ownership flag, because ownership of a buffer
// ping-pongs between the caller and the stream. [Stream.PushBuffer] claims the flag: the
// buffer belongs to the stream from then on and is released when the stream is closed.
// Every pop mints a new Buffer with a new flag, because Aravis transfers ownership back.
// [Buffer.Close] is for a buffer that is nobody else's: one [NewBuffer] created and
// nothing ever pushed, and one that was popped and is not going back.
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
// stream releases the buffer. [Buffer.Close] invalidates such a slice in exactly the same
// way, and more abruptly — it frees the payload there and then. Neither Go's garbage
// collector nor its race detector can see either hazard. Consume the data, or copy what
// you need to keep, before giving the buffer up.
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
// Everything the package decides for itself — a pop that timed out, a nil or closed
// handle, an out-of-range part index — is reported as one of the package-level
// sentinels in errors.go, matched with errors.Is:
//
//	buffer, err := stream.TimeoutPopBuffer(time.Second)
//	if errors.Is(err, aravis.ErrTimeout) {
//		// no frame this round, not a failure
//	}
//
// The set is [ErrTimeout], [ErrNoBuffer], [ErrNegativeTimeout], [ErrNilStream],
// [ErrStreamClosed], [ErrNilBuffer], [ErrBufferNotOwned], [ErrBufferAllocation],
// [ErrPartOutOfRange] and [ErrPartNotImage]. Some are wrapped with the offending
// value before being returned, which is why errors.Is is the right test rather than
// an == comparison.
//
// Only a GError decides that a call failed. An accessor wrapping a C function that has
// no GError out-parameter therefore has nothing to report, and returns no error at all:
// [Buffer.GetData], [Buffer.GetDataUnsafe], [Buffer.GetDataSlice], [Buffer.GetDataInto],
// [Buffer.GetStatus], [Buffer.GetNumParts], [Buffer.FindComponent], [GetDeviceId],
// [GetInterfaceId], [GetNumDevices] and [GetNumInterface] are single-valued, and the
// returned value is the whole contract. Called on a receiver or index that has nothing
// to give — the zero [Buffer], an index past the end of the device list — they report
// emptiness rather than failure: no data, BUFFER_STATUS_UNKNOWN, zero parts, a component
// index of -1, an empty id.
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
