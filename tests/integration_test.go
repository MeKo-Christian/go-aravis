package tests

import (
	"bytes"
	"errors"
	"testing"
	"time"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// TestFullWorkflow drives the whole acquisition sequence end to end: connect,
// configure, create a stream, fill it with buffers, acquire, and hand each
// buffer back.
//
// It used to skip whenever no camera was attached, so in CI it never ran and
// its assertions were dead. It now runs against the Fake backend; only the
// -short gate remains, because it still costs a real acquisition.
func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	camera, isFake := requireStreamingCamera(t)
	defer camera.Close()

	if err := camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS); err != nil {
		t.Fatalf("SetAcquisitionMode(CONTINUOUS) returned error: %v", err)
	}

	stream, payloadSize := setUpStream(t, camera)
	defer stream.Close()

	acquireFrames(t, camera, stream, payloadSize, isFake)
}

// setUpStream creates a stream and primes it with buffers, returning the
// payload size the caller needs to size its own destination buffer.
func setUpStream(t *testing.T, camera aravis.Camera) (aravis.Stream, uint) {
	t.Helper()

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		stream.Close()
		t.Fatalf("GetPayloadSize() returned error: %v", err)
	}

	const numBuffers = 5

	for i := range numBuffers {
		buffer, err := aravis.NewBuffer(payloadSize)
		if err != nil {
			stream.Close()
			t.Fatalf("NewBuffer(%d) for buffer %d returned error: %v", payloadSize, i, err)
		}

		stream.PushBuffer(buffer)
	}

	return stream, payloadSize
}

// acquireFrames pops frames until it has the number it wants or the stream
// stops delivering, checking each one.
func acquireFrames(t *testing.T, camera aravis.Camera, stream aravis.Stream, payloadSize uint, isFake bool) {
	t.Helper()

	if err := camera.StartAcquisition(); err != nil {
		t.Fatalf("StartAcquisition() returned error: %v", err)
	}

	defer func() {
		if err := camera.StopAcquisition(); err != nil {
			t.Errorf("StopAcquisition() returned error: %v", err)
		}
	}()

	const (
		maxFrames = 10
		// A hardware link may drop frames, so the loop retries; maxAttempts
		// bounds that retrying. Without it the loop is bounded only by frames
		// successfully acquired, and a camera delivering nothing but bad
		// buffers would spin here indefinitely.
		maxAttempts = 100
	)

	framesAcquired := 0

	for attempts := 0; framesAcquired < maxFrames; attempts++ {
		if attempts >= maxAttempts {
			t.Errorf("gave up after %d attempts with %d of %d frames acquired",
				attempts, framesAcquired, maxFrames)

			break
		}

		buffer, err := stream.TimeoutPopBuffer(time.Second)
		if err != nil {
			// A non-nil error implies a zero Buffer, so there is nothing to
			// hand back here. Anything other than a timeout is a defect rather
			// than a dropped frame, and the sentinel is what lets the loop tell
			// them apart.
			if !errors.Is(err, aravis.ErrTimeout) {
				t.Errorf("TimeoutPopBuffer() returned %v; want a frame or ErrTimeout", err)
			}

			t.Logf("stopped after %d frames: %v", framesAcquired, err)

			break
		}

		status, err := buffer.GetStatus()

		// Both of the failures below used to `continue` without advancing
		// framesAcquired, so a backend that kept returning bad buffers would
		// spin here forever — the loop is bounded by frames acquired, not by
		// iterations or by a deadline. Since either condition is already a hard
		// failure, return the buffer and stop.
		if err != nil {
			t.Errorf("GetStatus() on frame %d returned error: %v", framesAcquired, err)
			stream.PushBuffer(buffer)

			break
		}

		// The Fake backend drops no packets, so anything but SUCCESS is a real
		// failure rather than the flaky-link case a hardware run allows for.
		if status != aravis.BUFFER_STATUS_SUCCESS {
			if isFake {
				t.Errorf("frame %d has status %d, want %d (SUCCESS)",
					framesAcquired, status, aravis.BUFFER_STATUS_SUCCESS)
				stream.PushBuffer(buffer)

				break
			}

			// On real hardware a dropped frame is not a test failure; retry
			// until the pop above times out.
			t.Logf("frame %d has status %d; retrying", framesAcquired, status)
			stream.PushBuffer(buffer)

			continue
		}

		checkFrameData(t, buffer, framesAcquired, payloadSize)

		framesAcquired++

		stream.PushBuffer(buffer)
	}

	if framesAcquired != maxFrames {
		t.Errorf("acquired %d frames, want %d", framesAcquired, maxFrames)
	}
}

// checkFrameData asserts that the three copying accessors agree on a live
// frame. The version this replaces compared data[0] and data[1] against the
// zero-copy slice and logged the rest.
func checkFrameData(t *testing.T, buffer aravis.Buffer, frameNum int, payloadSize uint) {
	t.Helper()

	data, err := buffer.GetData()
	if err != nil {
		t.Errorf("frame %d: GetData() returned error: %v", frameNum, err)

		return
	}

	if uint(len(data)) != payloadSize {
		t.Errorf("frame %d: GetData() returned %d bytes, want the payload size %d",
			frameNum, len(data), payloadSize)
	}

	slice, err := buffer.GetDataSlice()
	if err != nil {
		t.Errorf("frame %d: GetDataSlice() returned error: %v", frameNum, err)

		return
	}

	if !bytes.Equal(data, slice) {
		t.Errorf("frame %d: GetData() and GetDataSlice() disagree", frameNum)
	}

	dest := make([]byte, len(data))

	n, err := buffer.GetDataInto(dest)
	if err != nil {
		t.Errorf("frame %d: GetDataInto() returned error: %v", frameNum, err)

		return
	}

	if n != len(data) {
		t.Errorf("frame %d: GetDataInto() = %d, want %d", frameNum, n, len(data))
	}

	if !bytes.Equal(dest, data) {
		t.Errorf("frame %d: GetDataInto() wrote bytes that differ from GetData()", frameNum)
	}
}

// TestStreamingPerformance sustains a stream and checks the throughput
// assertions that were previously unreachable in CI.
//
// The duration is short on purpose: the Fake camera runs at 25 FPS, so two
// seconds is ~50 frames, which is plenty to catch a stream that stalls or
// delivers nothing.
func TestStreamingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming performance test in short mode")
	}

	camera, isFake := requireStreamingCamera(t)
	defer camera.Close()

	if err := camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS); err != nil {
		t.Fatalf("SetAcquisitionMode(CONTINUOUS) returned error: %v", err)
	}

	camera.ThreadPriority = aravis.ThreadPriorityHigh

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	defer stream.Close()

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Fatalf("GetPayloadSize() returned error: %v", err)
	}

	const numBuffers = 10

	for range numBuffers {
		buffer, err := aravis.NewBuffer(payloadSize)
		if err != nil {
			t.Fatalf("NewBuffer(%d) returned error: %v", payloadSize, err)
		}

		stream.PushBuffer(buffer)
	}

	if err := camera.StartAcquisition(); err != nil {
		t.Fatalf("StartAcquisition() returned error: %v", err)
	}

	defer func() {
		if err := camera.StopAcquisition(); err != nil {
			t.Errorf("StopAcquisition() returned error: %v", err)
		}
	}()

	const testDuration = 2 * time.Second

	var (
		startTime    = time.Now()
		frameCount   int
		errorCount   int
		timeoutCount int
		destBuffer   = make([]byte, payloadSize)
	)

	for time.Since(startTime) < testDuration {
		buffer, err := stream.TimeoutPopBuffer(100 * time.Millisecond)
		if err != nil {
			// Only a timeout may be counted as one: before the sentinel existed
			// this branch swallowed every other failure as "no frame this
			// round".
			if !errors.Is(err, aravis.ErrTimeout) {
				t.Fatalf("TimeoutPopBuffer() returned %v; want a frame or ErrTimeout", err)
			}

			timeoutCount++

			continue
		}

		status, err := buffer.GetStatus()
		if err != nil || status != aravis.BUFFER_STATUS_SUCCESS {
			errorCount++

			stream.PushBuffer(buffer)

			continue
		}

		frameCount++

		// Exercise the zero-allocation copy on the way past.
		if frameCount%10 == 0 {
			if _, err := buffer.GetDataInto(destBuffer); err != nil {
				t.Errorf("GetDataInto() on frame %d returned error: %v", frameCount, err)
			}
		}

		stream.PushBuffer(buffer)
	}

	elapsed := time.Since(startTime)
	fps := float64(frameCount) / elapsed.Seconds()

	t.Logf("%d frames in %.2fs (%.2f FPS, %.2f MB/s), %d timeouts, %d errors",
		frameCount, elapsed.Seconds(), fps, fps*float64(payloadSize)/1024/1024, timeoutCount, errorCount)

	if frameCount == 0 {
		t.Error("no frames acquired")
	}

	if fps < 1.0 {
		t.Errorf("average frame rate %.2f FPS is below 1 FPS", fps)
	}

	// Fake delivers every frame intact, so unlike a hardware run this tolerates
	// no bad buffers at all. A physical link legitimately drops some, so there
	// the check is a rate rather than an absolute.
	if isFake {
		if errorCount != 0 {
			t.Errorf("%d buffers came back with a non-success status; want 0 on the Fake backend", errorCount)
		}
	} else if errorCount > frameCount {
		t.Errorf("%d bad buffers against %d good ones; the link is dropping more than half the frames",
			errorCount, frameCount)
	}
}

// TestMultipleDevices is the one test that genuinely cannot run without
// hardware: Aravis's Fake interface produces exactly one device, Fake_1, and
// offers no way to ask for a second, so independent-camera operation cannot be
// exercised against it.
//
// Run it with ARAVIS_TEST_HARDWARE=1 and two cameras attached (see
// `make test-integration`).
func TestMultipleDevices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multiple device test in short mode")
	}

	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("GetNumDevices() returned error: %v", err)
	}

	// Fake_1 is excluded deliberately. With one physical camera attached the
	// device list holds two entries, and counting Fake as the second would let
	// this test report success on independent multi-camera operation while
	// only ever having driven one real device.
	deviceIDs := make([]string, 0, numDevices)

	for i := range numDevices {
		id, err := aravis.GetDeviceId(i)
		if err != nil {
			t.Fatalf("GetDeviceId(%d) returned error: %v", i, err)
		}

		if id != fakeDeviceID {
			deviceIDs = append(deviceIDs, id)
		}
	}

	if len(deviceIDs) < 2 {
		t.Skipf("%d physical device(s) present; this test needs two, and the Fake backend provides only itself",
			len(deviceIDs))
	}

	cameras := make([]aravis.Camera, 0, len(deviceIDs))

	defer func() {
		for _, camera := range cameras {
			camera.Close()
		}
	}()

	for _, deviceID := range deviceIDs {
		camera, err := aravis.NewCamera(deviceID)
		if err != nil {
			t.Errorf("NewCamera(%s) returned error: %v", deviceID, err)

			continue
		}

		cameras = append(cameras, camera)
	}

	if len(cameras) != len(deviceIDs) {
		t.Fatalf("opened %d of %d devices", len(cameras), len(deviceIDs))
	}

	// Distinct serials are the actual contract: they are what shows each Camera
	// wraps its own ArvDevice rather than aliasing another's. Merely checking
	// that the accessors return without error would pass even if every camera
	// pointed at the same device.
	seen := make(map[string]string, len(cameras))

	for i, camera := range cameras {
		serial, err := camera.GetDeviceSerialNumber()
		if err != nil {
			t.Errorf("camera %d (%s): GetDeviceSerialNumber() returned error: %v", i, deviceIDs[i], err)

			continue
		}

		if previous, ok := seen[serial]; ok {
			t.Errorf("camera %s reports serial %q, already reported by %s; the cameras are not independent",
				deviceIDs[i], serial, previous)
		}

		seen[serial] = deviceIDs[i]
	}

	// Each camera must also stream on its own.
	for i, camera := range cameras {
		stream, payloadSize := setUpStream(t, camera)
		acquireFrames(t, camera, stream, payloadSize, false)
		stream.Close()

		t.Logf("camera %d (%s) delivered frames of %d bytes", i, deviceIDs[i], payloadSize)
	}
}
