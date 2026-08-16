package tests

// This file owns the backend the whole package runs against.
//
// Aravis ships a built-in "Fake" interface that produces a real ArvDevice, a
// real ArvStream and real, filled buffers with no hardware attached. Enabling
// it is global process state, and until now it was enabled by whichever test
// happened to run first (seededBuffer, fakeCamera). That made the suite
// order-dependent in the worst way: the tests gated on GetNumDevices() == 0
// quietly ran against Fake_1 in a full-package run and skipped when run on
// their own, so `go test -run TestX ./tests/` and `go test ./tests/` asserted
// different things.

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	aravis "github.com/MeKo-Christian/go-aravis"
)

const (
	// fakeInterface is Aravis's built-in software backend. It is disabled by
	// default, which is why every entry point here enables it explicitly.
	fakeInterface = "Fake"
	// fakeDeviceID is the single device that backend produces. There is never
	// a second one, which is why the multi-camera test cannot use it.
	fakeDeviceID = "Fake_1"
	// popTimeout is generous on purpose: the Fake camera runs at 25 FPS, so a
	// frame is due within 40 ms and anything approaching this bound means the
	// acquisition never started. It is a named time.Duration so no caller can
	// repeat the untyped-literal bug that made TimeoutPopBuffer(1000) a 1 µs
	// timeout.
	popTimeout = 5 * time.Second
)

// TestMain pins the device list before any test runs: Fake enabled, every
// other interface disabled. That makes discovery deterministic (exactly one
// device), hermetic (a GigE or USB3 camera plugged into the developer's
// machine cannot change an assertion), and fast — the GigE/USB discovery scan
// this suite paid on every UpdateDeviceList was ~1 s a call.
//
// Set ARAVIS_TEST_HARDWARE=1 to keep the real interfaces enabled. The
// acquisition tests then drive a physical camera through
// requireStreamingCamera, which fails when none is attached; tests that assert
// Fake's fixed identity keep using Fake_1 either way.
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// hardwareMode reports whether the suite was asked to drive a physical camera.
func hardwareMode() bool {
	return os.Getenv("ARAVIS_TEST_HARDWARE") != ""
}

func run(m *testing.M) int {
	// arv_shutdown must not race with library use, so it runs exactly once,
	// here, after every test has finished. No test may call it: doing so
	// mid-suite dismantled the interface list the remaining tests rely on.
	defer aravis.Shutdown()

	if err := selectFakeBackend(hardwareMode()); err != nil {
		fmt.Fprintf(os.Stderr, "cannot set up the Aravis Fake backend: %v\n", err)

		return 1
	}

	aravis.UpdateDeviceList()

	// Fail loudly and once, rather than letting every test skip or fail with
	// its own version of the same message.
	camera, err := aravis.NewCamera(fakeDeviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "the Fake backend produced no %s: %v\n", fakeDeviceID, err)

		return 1
	}

	camera.Close()

	return m.Run()
}

// selectFakeBackend enables the Fake interface and, unless real hardware was
// asked for, disables every other one.
func selectFakeBackend(keepHardware bool) error {
	n, err := aravis.GetNumInterface()
	if err != nil {
		return fmt.Errorf("GetNumInterface: %w", err)
	}

	if n == 0 {
		return errors.New("aravis reports no interfaces at all")
	}

	found := false

	for i := range n {
		id, err := aravis.GetInterfaceId(i)
		if err != nil {
			return fmt.Errorf("GetInterfaceId(%d): %w", i, err)
		}

		if id == fakeInterface {
			found = true

			continue
		}

		if !keepHardware {
			aravis.DisableInterface(id)
		}
	}

	if !found {
		return fmt.Errorf("this libaravis build has no %q interface", fakeInterface)
	}

	aravis.EnableInterface(fakeInterface)

	return nil
}

// requireFakeCamera returns a camera backed by the Fake interface. The caller
// owns it and must close it — several lifecycle tests are about exactly who
// closes it and how often, so no Cleanup is registered here.
//
// Use this for tests that assert Fake's fixed identity or geometry. Tests that
// only exercise streaming should use requireStreamingCamera, so that the
// hardware target actually reaches the hardware.
func requireFakeCamera(tb testing.TB) aravis.Camera {
	tb.Helper()

	camera, err := aravis.NewCamera(fakeDeviceID)
	if err != nil {
		tb.Fatalf("NewCamera(%s) returned error: %v", fakeDeviceID, err)
	}

	if camera.IsNil() {
		tb.Fatalf("NewCamera(%s) returned a nil camera", fakeDeviceID)
	}

	return camera
}

// requireStreamingCamera returns the camera an acquisition test should drive,
// and whether it is the Fake device.
//
// Under ARAVIS_TEST_HARDWARE it returns the first *non-Fake* device and fails
// when there is none, so `make test-integration` cannot report success without
// having touched a physical camera. Otherwise it returns Fake_1.
//
// The bool is what lets a caller keep its assertions honest in both modes:
// Fake delivers every frame intact, a real GigE or USB3 link does not, so an
// assertion like "no buffer came back with a bad status" is only correct
// against Fake.
func requireStreamingCamera(tb testing.TB) (camera aravis.Camera, isFake bool) {
	tb.Helper()

	if !hardwareMode() {
		return requireFakeCamera(tb), true
	}

	deviceID := firstHardwareDeviceID(tb)

	camera, err := aravis.NewCamera(deviceID)
	if err != nil {
		tb.Fatalf("NewCamera(%s) returned error: %v", deviceID, err)
	}

	if camera.IsNil() {
		tb.Fatalf("NewCamera(%s) returned a nil camera", deviceID)
	}

	return camera, false
}

// firstHardwareDeviceID returns the id of the first device that is not the
// Fake one, failing when the only thing attached is Fake itself.
func firstHardwareDeviceID(tb testing.TB) string {
	tb.Helper()

	aravis.UpdateDeviceList()

	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		tb.Fatalf("GetNumDevices() returned error: %v", err)
	}

	for i := range numDevices {
		id, err := aravis.GetDeviceId(i)
		if err != nil {
			tb.Fatalf("GetDeviceId(%d) returned error: %v", i, err)
		}

		if id != fakeDeviceID {
			return id
		}
	}

	tb.Fatalf("ARAVIS_TEST_HARDWARE is set but the only device present is %s; "+
		"attach a camera or drop the variable", fakeDeviceID)

	return ""
}

// pushBack returns a popped buffer to its stream. Every acquisition test goes
// through it so that no call site can silently drop the buffer, and so that the
// error PushBuffer now returns is checked rather than discarded — pushing a
// buffer the caller no longer owns is exactly the double free the ownership
// flag exists to catch.
func pushBack(tb testing.TB, stream aravis.Stream, buffer aravis.Buffer) {
	tb.Helper()

	if err := stream.PushBuffer(buffer); err != nil {
		tb.Errorf("PushBuffer() returned error: %v", err)
	}
}

// seededBuffer returns a filled buffer whose bytes follow a deterministic
// pattern, plus a copy of that pattern for comparison.
//
// A buffer straight out of aravis.NewBuffer reports a data size of zero
// (arv_buffer_get_data returns the received size, which is only set once the
// buffer has been filled), so seeding requires a real acquisition. Once a Fake
// buffer has been popped, GetDataSlice aliases its C memory and lets us
// overwrite the payload with a known pattern.
//
// It takes a testing.TB so benchmarks can seed the same way tests do; that is
// what makes the buffer benchmarks measure the filled path instead of the
// early return on an empty buffer.
func seededBuffer(tb testing.TB) (aravis.Buffer, []byte) {
	tb.Helper()

	camera := requireFakeCamera(tb)
	tb.Cleanup(camera.Close)

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		tb.Fatalf("GetPayloadSize() returned error: %v", err)
	}

	if payloadSize == 0 {
		tb.Fatalf("GetPayloadSize() = 0, want a non-zero payload")
	}

	stream, err := camera.CreateStream()
	if err != nil {
		tb.Fatalf("CreateStream() returned error: %v", err)
	}

	tb.Cleanup(stream.Close)

	buf, err := aravis.NewBuffer(payloadSize)
	if err != nil {
		tb.Fatalf("NewBuffer(%d) returned error: %v", payloadSize, err)
	}

	if buf.IsNil() {
		tb.Fatalf("NewBuffer(%d) returned a nil buffer", payloadSize)
	}

	pushBack(tb, stream, buf)

	if err := camera.StartAcquisition(); err != nil {
		tb.Fatalf("StartAcquisition() returned error: %v", err)
	}

	filled, popErr := stream.TimeoutPopBuffer(popTimeout)

	// Stop as soon as the frame is in hand: nothing should keep acquiring
	// underneath a benchmark.
	if err := camera.StopAcquisition(); err != nil {
		tb.Errorf("StopAcquisition() returned error: %v", err)
	}

	// A non-nil error from a pop now implies a zero Buffer, so the error can be
	// checked before there is anything to hand back.
	if popErr != nil {
		tb.Fatalf("TimeoutPopBuffer() returned error: %v", popErr)
	}

	if filled.IsNil() {
		tb.Fatalf("TimeoutPopBuffer() returned a nil buffer")
	}

	// A popped buffer belongs to the caller: Stream.Close frees only what is
	// still sitting in the stream's queues, so it has to be given back. Pushing
	// it is what this helper does — Buffer.Close would release it too, but the
	// push is the shape every caller of seededBuffer wants, since it leaves the
	// stream able to keep streaming.
	//
	// This is registered after the stream's own cleanup, and t.Cleanup runs
	// last-in-first-out, so the push-back happens before Stream.Close — which
	// is what makes Stream.Close able to free it. With ownership enforced, the
	// order is no longer just tidy: pushing after Stream.Close would now be
	// rejected with ErrStreamClosed instead of silently handing a buffer to a
	// released stream.
	tb.Cleanup(func() { pushBack(tb, stream, filled) })

	if status, err := filled.GetStatus(); err != nil || status != aravis.BUFFER_STATUS_SUCCESS {
		tb.Fatalf("GetStatus() = %d, %v; want %d, nil", status, err, aravis.BUFFER_STATUS_SUCCESS)
	}

	slice, err := filled.GetDataSlice()
	if err != nil {
		tb.Fatalf("GetDataSlice() returned error: %v", err)
	}

	if len(slice) != int(payloadSize) {
		tb.Fatalf("GetDataSlice() returned %d bytes, want the full payload of %d", len(slice), payloadSize)
	}

	// Overwrite the acquired payload in place so the expected bytes are known.
	want := make([]byte, len(slice))
	for i := range slice {
		v := byte(i*7 + 1)
		slice[i] = v
		want[i] = v
	}

	return filled, want
}
