// Package aravis provides Go bindings for the Aravis 0.8 machine-vision library,
// giving access to GigE Vision and USB3 Vision cameras.
//
// The package is a cgo wrapper. It requires libaravis 0.8 with its development
// headers and locates them through pkg-config (aravis-0.8), so CGO_ENABLED=1 and a
// working C toolchain are mandatory. There is no pure-Go fallback and no support for
// the older 0.6 series.
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
//	if err != nil {
//		return err
//	}
//	if status, _ := buffer.GetStatus(); status == aravis.BUFFER_STATUS_SUCCESS {
//		data, _ := buffer.GetData()
//		_ = data
//	}
//	stream.PushBuffer(buffer) // hand it back for reuse
//
// A buffer popped from the stream must be pushed back, or the queue drains and
// acquisition stalls. Always check [Buffer.GetStatus] before trusting pixel data: a
// buffer can be returned with missing packets or a timeout and still be non-nil.
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
// The two aliasing accessors return memory owned by Aravis. It is invalidated as soon
// as the buffer is handed back with [Stream.PushBuffer], so the data must be consumed
// or copied before that call. Retaining such a slice past PushBuffer is a
// use-after-free that Go's race detector and garbage collector cannot see.
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
// Failures from Aravis are reported as [*AravisError], which carries the GLib message
// along with a Code drawn from the DEVICE_ERROR_* constants. Match it with errors.As:
//
//	var aerr *aravis.AravisError
//	if errors.As(err, &aerr) && aerr.Code == aravis.DEVICE_ERROR_TIMEOUT {
//		// ...
//	}
//
// Not every call reports GenICam failures. The generic feature getters on [Device]
// notably do not; their documentation says so individually.
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
