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
// Set ARAVIS_TEST_HARDWARE=1 to keep the real interfaces enabled; the tests
// that genuinely need a camera then find one, and everything else keeps using
// Fake_1.
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	// arv_shutdown must not race with library use, so it runs exactly once,
	// here, after every test has finished. No test may call it: doing so
	// mid-suite dismantled the interface list the remaining tests rely on.
	defer aravis.Shutdown()

	if err := selectFakeBackend(os.Getenv("ARAVIS_TEST_HARDWARE") != ""); err != nil {
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
	// The error from GetNumInterface is cgo's errno, not an Aravis failure
	// (see P6 in PLAN.md), so only the count is trustworthy here.
	n, _ := aravis.GetNumInterface()
	if n == 0 {
		return errors.New("aravis reports no interfaces at all")
	}

	found := false

	for i := range n {
		id, _ := aravis.GetInterfaceId(i)
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

	stream.PushBuffer(buf)

	if err := camera.StartAcquisition(); err != nil {
		tb.Fatalf("StartAcquisition() returned error: %v", err)
	}

	filled, popErr := stream.TimeoutPopBuffer(popTimeout)

	// Stop as soon as the frame is in hand: the popped buffer belongs to us
	// (it was never pushed back), so it stays valid, and no acquisition thread
	// keeps running underneath a benchmark.
	if err := camera.StopAcquisition(); err != nil {
		tb.Errorf("StopAcquisition() returned error: %v", err)
	}

	if popErr != nil {
		tb.Fatalf("TimeoutPopBuffer() returned error: %v", popErr)
	}

	if filled.IsNil() {
		tb.Fatalf("TimeoutPopBuffer() returned a nil buffer")
	}

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
