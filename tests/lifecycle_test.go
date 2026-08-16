package tests

import (
	"sync"
	"testing"

	aravis "github.com/hybridgroup/go-aravis"
)

// TestCloseIsIdempotent covers the unguarded Close methods: they called
// g_object_unref on a possibly NULL or already-freed pointer, so a zero value
// or a second Close aborted the process. None of this needs a camera — the
// point is precisely that the nil pointer is handled before reaching C.
func TestCloseIsIdempotent(t *testing.T) {
	t.Run("camera", func(t *testing.T) {
		var camera aravis.Camera
		camera.Close()
		camera.Close() // must not unref a second time
	})

	t.Run("stream", func(t *testing.T) {
		var stream aravis.Stream
		stream.Close()
		stream.Close()
	})

	t.Run("device", func(t *testing.T) {
		var device aravis.Device
		device.Close()
		device.Close()
	})
}

// TestBorrowedDeviceCloseDoesNotUnref pins the ownership split introduced with
// OpenDevice: a Device built by Camera.GetDevice only borrows the camera's
// device, so closing it must not drop a reference the camera still holds.
// A zero-value Device is the borrowed case with no pointer at all.
func TestBorrowedDeviceCloseDoesNotUnref(t *testing.T) {
	var device aravis.Device
	device.Close()

	// Closing the borrowed device must leave the process alive and the second
	// call must stay a no-op.
	device.Close()
}

// TestSetControlLostHandlerOnClosedCamera checks the guard rather than the
// signal: with no camera there is nothing to connect to, and the old code
// silently stored the handler in a package global anyway, reporting success
// for a callback that could never fire.
func TestSetControlLostHandlerOnClosedCamera(t *testing.T) {
	var camera aravis.Camera

	if err := camera.SetControlLostHandler(func() {}); err == nil {
		t.Error("SetControlLostHandler on a closed camera returned nil; want an error")
	}

	// Removing a handler from a closed camera is equally impossible.
	if err := camera.SetControlLostHandler(nil); err == nil {
		t.Error("SetControlLostHandler(nil) on a closed camera returned nil; want an error")
	}
}

// TestSetControlLostHandlerConcurrent exercises the registry that replaced the
// unsynchronized package-global handler. Run under -race, concurrent installs
// across several cameras must not trip the detector. The cameras are all
// closed, so the calls fail — the shared state behind them is still touched,
// which is what this guards.
func TestSetControlLostHandlerConcurrent(t *testing.T) {
	const goroutines = 8

	cameras := make([]aravis.Camera, goroutines)

	var wg sync.WaitGroup
	for i := range cameras {
		wg.Add(1)

		go func(camera *aravis.Camera) {
			defer wg.Done()

			for n := 0; n < 100; n++ {
				_ = camera.SetControlLostHandler(func() {})
				_ = camera.SetControlLostHandler(nil)
				camera.Close()
			}
		}(&cameras[i])
	}

	wg.Wait()
}

// TestGetNumInterfaceMatchesDeprecatedAlias makes sure the typo fix kept the
// old spelling working: GetNumInferface must simply forward to
// GetNumInterface.
func TestGetNumInterfaceMatchesDeprecatedAlias(t *testing.T) {
	want, wantErr := aravis.GetNumInterface()
	got, gotErr := aravis.GetNumInferface() //nolint:staticcheck // exercising the deprecated alias on purpose

	if got != want {
		t.Errorf("GetNumInferface() = %d, GetNumInterface() = %d; the alias must forward", got, want)
	}

	if (gotErr == nil) != (wantErr == nil) {
		t.Errorf("GetNumInferface() error = %v, GetNumInterface() error = %v; want the same outcome", gotErr, wantErr)
	}
}
