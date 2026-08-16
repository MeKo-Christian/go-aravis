package tests

// The benchmarks in this file run against Aravis's Fake backend, on a buffer
// that has actually been filled.
//
// Both of those were previously untrue, and together they made every number
// meaningless. Each benchmark skipped unless a camera was attached, so none
// ever ran in CI; and the buffer benchmarks measured a fresh NewBuffer, whose
// received size is zero, so they timed the `if size == 0 { return }` early
// return rather than any data access.
//
// What running them in CI buys is that the bodies execute and their errors are
// checked, not that the timings are comparable between runs — figures from a
// shared runner are not, which is the point PERFORMANCE.md already makes.

import (
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// benchInt times a standard/fast pair of int accessors, failing rather than
// timing an error path. Without the probe, a benchmark whose every iteration
// fails reads as an exceptionally fast one.
func benchInt(b *testing.B, name string, standard, fast func() (int, error)) {
	b.Helper()

	if _, err := standard(); err != nil {
		b.Fatalf("Get%s() returned error: %v", name, err)
	}

	if _, err := fast(); err != nil {
		b.Fatalf("Get%sFast() returned error: %v", name, err)
	}

	b.Run(name+"/Standard", func(b *testing.B) {
		for range b.N {
			_, _ = standard()
		}
	})

	b.Run(name+"/Fast", func(b *testing.B) {
		for range b.N {
			_, _ = fast()
		}
	})
}

// benchFloat is the same for float accessors, except that the fast path may be
// genuinely unavailable: GetExposureTimeFast and GetGainFast address the fixed
// ExposureTime and Gain GenICam nodes, which the Fake camera does not expose.
// Timing that error path would produce a fast, meaningless number, so the pair
// is skipped instead.
func benchFloat(b *testing.B, name string, standard, fast func() (float64, error)) {
	b.Helper()

	if _, err := standard(); err != nil {
		b.Fatalf("Get%s() returned error: %v", name, err)
	}

	b.Run(name+"/Standard", func(b *testing.B) {
		for range b.N {
			_, _ = standard()
		}
	})

	b.Run(name+"/Fast", func(b *testing.B) {
		if _, err := fast(); err != nil {
			b.Skipf("Get%sFast() = %v; this camera exposes no matching GenICam node", name, err)
		}

		// The probe above ran with the timer already started, so without this
		// its cost lands in the reported figure — at CI's BENCHTIME=10x that is
		// one call in eleven.
		b.ResetTimer()

		for range b.N {
			_, _ = fast()
		}
	})
}

// probeBuffer checks every buffer accessor once, outside any timed region, so
// that a broken one fails the benchmark instead of being timed on its error
// path — which would report an encouragingly small number for a call that does
// nothing.
func probeBuffer(b *testing.B, buffer aravis.Buffer, dest []byte) {
	b.Helper()

	if _, err := buffer.GetData(); err != nil {
		b.Fatalf("GetData() returned error: %v", err)
	}

	if _, err := buffer.GetDataSlice(); err != nil {
		b.Fatalf("GetDataSlice() returned error: %v", err)
	}

	if _, err := buffer.GetDataInto(dest); err != nil {
		b.Fatalf("GetDataInto() returned error: %v", err)
	}

	if _, _, err := buffer.GetDataUnsafe(); err != nil {
		b.Fatalf("GetDataUnsafe() returned error: %v", err)
	}
}

// probeCameraGeometry checks the camera accessors the streaming-loop
// benchmarks use, for the same reason as probeBuffer.
func probeCameraGeometry(b *testing.B, camera aravis.Camera) {
	b.Helper()

	if _, err := camera.GetWidth(); err != nil {
		b.Fatalf("GetWidth() returned error: %v", err)
	}

	if _, err := camera.GetHeight(); err != nil {
		b.Fatalf("GetHeight() returned error: %v", err)
	}

	if _, err := camera.GetWidthFast(); err != nil {
		b.Fatalf("GetWidthFast() returned error: %v", err)
	}

	if _, err := camera.GetHeightFast(); err != nil {
		b.Fatalf("GetHeightFast() returned error: %v", err)
	}

	if _, err := camera.GetExposureTime(); err != nil {
		b.Fatalf("GetExposureTime() returned error: %v", err)
	}
}

// BenchmarkParameterAccess compares the standard and *Fast parameter
// accessors. It absorbs the near-identical BenchmarkCameraParameterAccess that
// used to sit in camera_test.go.
func BenchmarkParameterAccess(b *testing.B) {
	camera := requireFakeCamera(b)
	defer camera.Close()

	benchInt(b, "Width", camera.GetWidth, camera.GetWidthFast)
	benchInt(b, "Height", camera.GetHeight, camera.GetHeightFast)
	benchFloat(b, "ExposureTime", camera.GetExposureTime, camera.GetExposureTimeFast)
	benchFloat(b, "Gain", camera.GetGain, camera.GetGainFast)
}

// BenchmarkBufferDataAccess compares the four ways of reaching a payload. It
// absorbs the duplicate of the same name from buffer_test.go.
//
// SetBytes is what makes the comparison legible: GetData copies, GetDataInto
// copies into memory the caller already owns, and GetDataSlice and
// GetDataUnsafe do not copy at all, so throughput separates them where a bare
// ns/op does not.
func BenchmarkBufferDataAccess(b *testing.B) {
	buffer, want := seededBuffer(b)
	destBuffer := make([]byte, len(want))

	probeBuffer(b, buffer, destBuffer)

	b.Run("GetData", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _ = buffer.GetData()
		}
	})

	b.Run("GetDataSlice", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _ = buffer.GetDataSlice()
		}
	})

	b.Run("GetDataInto", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("GetDataUnsafe", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _, _ = buffer.GetDataUnsafe()
		}
	})
}

// BenchmarkCombinedOperations times the shape of a real streaming loop: read a
// few parameters, then take the payload. The three variants differ only in
// which accessors they reach for.
//
// The exposure read is deliberately the standard call in all three: its fast
// counterpart is unavailable on Fake, and substituting an error path into one
// arm of a comparison would make the comparison a lie.
func BenchmarkCombinedOperations(b *testing.B) {
	buffer, want := seededBuffer(b)

	camera := requireFakeCamera(b)
	defer camera.Close()

	destBuffer := make([]byte, len(want))

	probeBuffer(b, buffer, destBuffer)
	probeCameraGeometry(b, camera)

	b.Run("StreamingLoop/Standard", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidth()
			_, _ = camera.GetHeight()
			_, _ = camera.GetExposureTime()
			_, _ = buffer.GetData()
		}
	})

	b.Run("StreamingLoop/Optimized", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidthFast()
			_, _ = camera.GetHeightFast()
			_, _ = camera.GetExposureTime()
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("StreamingLoop/ZeroCopy", func(b *testing.B) {
		b.SetBytes(int64(len(want)))
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidthFast()
			_, _ = camera.GetHeightFast()
			_, _ = camera.GetExposureTime()
			_, _ = buffer.GetDataSlice()
		}
	})
}

// BenchmarkMemoryAllocations is the allocation counterpart: it is what backs
// the one quantitative claim PERFORMANCE.md still makes, that GetDataInto
// copies without allocating. TestGetDataIntoZeroAllocations asserts that; this
// shows it next to the alternatives.
func BenchmarkMemoryAllocations(b *testing.B) {
	buffer, want := seededBuffer(b)

	camera := requireFakeCamera(b)
	defer camera.Close()

	destBuffer := make([]byte, len(want))

	probeBuffer(b, buffer, destBuffer)
	probeCameraGeometry(b, camera)

	b.Run("Allocations/StandardMethods", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidth()
			_, _ = buffer.GetData()
		}
	})

	b.Run("Allocations/FastMethods", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidthFast()
			_, _ = buffer.GetDataInto(destBuffer)
		}
	})

	b.Run("Allocations/ZeroCopy", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, _ = camera.GetWidthFast()
			_, _ = buffer.GetDataSlice()
		}
	})
}
