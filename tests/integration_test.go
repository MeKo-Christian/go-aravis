package tests

import (
	"bytes"
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

	camera := requireFakeCamera(t)
	defer camera.Close()

	if err := camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS); err != nil {
		t.Fatalf("SetAcquisitionMode(CONTINUOUS) returned error: %v", err)
	}

	stream, payloadSize := setUpStream(t, camera)
	defer stream.Close()

	acquireFrames(t, camera, stream, payloadSize)
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
func acquireFrames(t *testing.T, camera aravis.Camera, stream aravis.Stream, payloadSize uint) {
	t.Helper()

	if err := camera.StartAcquisition(); err != nil {
		t.Fatalf("StartAcquisition() returned error: %v", err)
	}

	defer func() {
		if err := camera.StopAcquisition(); err != nil {
			t.Errorf("StopAcquisition() returned error: %v", err)
		}
	}()

	const maxFrames = 10

	framesAcquired := 0

	for framesAcquired < maxFrames {
		buffer, err := stream.TimeoutPopBuffer(time.Second)
		if err != nil {
			t.Logf("stopped after %d frames: %v", framesAcquired, err)

			break
		}

		status, err := buffer.GetStatus()
		if err != nil {
			t.Errorf("GetStatus() on frame %d returned error: %v", framesAcquired, err)
			stream.PushBuffer(buffer)

			continue
		}

		// The Fake backend drops no packets, so anything but SUCCESS is a real
		// failure rather than the flaky-network case a hardware run allows for.
		if status != aravis.BUFFER_STATUS_SUCCESS {
			t.Errorf("frame %d has status %d, want %d (SUCCESS)",
				framesAcquired, status, aravis.BUFFER_STATUS_SUCCESS)
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

	camera := requireFakeCamera(t)
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
	// no bad buffers at all.
	if errorCount != 0 {
		t.Errorf("%d buffers came back with a non-success status; want 0 on the Fake backend", errorCount)
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

	if numDevices < 2 {
		t.Skipf("%d device(s) present; this test needs two, which the Fake backend cannot provide", numDevices)
	}

	cameras := make([]aravis.Camera, 0, numDevices)

	defer func() {
		for _, camera := range cameras {
			camera.Close()
		}
	}()

	for i := range numDevices {
		deviceID, err := aravis.GetDeviceId(i)
		if err != nil {
			t.Errorf("GetDeviceId(%d) returned error: %v", i, err)

			continue
		}

		camera, err := aravis.NewCamera(deviceID)
		if err != nil {
			t.Errorf("NewCamera(%s) returned error: %v", deviceID, err)

			continue
		}

		cameras = append(cameras, camera)
	}

	if len(cameras) != int(numDevices) {
		t.Fatalf("opened %d of %d devices", len(cameras), numDevices)
	}

	// Each camera must answer for itself rather than aliasing another's device.
	for i, camera := range cameras {
		if _, err := camera.GetVendorName(); err != nil {
			t.Errorf("camera %d: GetVendorName() returned error: %v", i, err)
		}

		if _, err := camera.GetDeviceSerialNumber(); err != nil {
			t.Errorf("camera %d: GetDeviceSerialNumber() returned error: %v", i, err)
		}
	}
}
