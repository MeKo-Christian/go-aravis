package tests

import (
	"sync"
	"testing"

	aravis "github.com/hybridgroup/go-aravis"
)

// fakeCamera returns a camera backed by Aravis's built-in "Fake" interface, so
// that the lifecycle tests operate on a real ArvCamera with a real reference
// count instead of a zero value. Without one, every Close returns at the nil
// guard and no test here would ever reach an unref.
//
// The caller owns the camera and is responsible for closing it — that is
// precisely what most of these tests are about.
func fakeCamera(t *testing.T) aravis.Camera {
	t.Helper()

	aravis.EnableInterface("Fake")
	aravis.UpdateDeviceList()

	camera, err := aravis.NewCamera("Fake_1")
	if err != nil {
		t.Fatalf("NewCamera(Fake_1) returned error: %v", err)
	}
	if camera.IsNil() {
		t.Fatal("NewCamera(Fake_1) returned a nil camera")
	}

	return camera
}

// TestCloseOnZeroValue covers the nil guards: an unguarded Close called
// g_object_unref on a NULL pointer.
func TestCloseOnZeroValue(t *testing.T) {
	t.Run("camera", func(t *testing.T) {
		var camera aravis.Camera
		camera.Close()
		camera.Close()
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

// TestCameraCloseUnrefsOnceAcrossCopies is the real double-free test. Camera is
// handed out by value, so a "closed" bool living in the wrapper would be
// per-copy: closing two copies would unref the same ArvCamera twice, which
// GLib reports as a critical and which corrupts the object. The close state is
// shared between copies instead, so exactly one of these calls unrefs.
func TestCameraCloseUnrefsOnceAcrossCopies(t *testing.T) {
	camera := fakeCamera(t)
	duplicate := camera

	camera.Close()

	if !camera.IsClosed() {
		t.Error("IsClosed() = false after Close(); want true")
	}
	if !duplicate.IsClosed() {
		t.Error("the copy reports IsClosed() = false; close state must be shared")
	}

	duplicate.Close() // must not unref a second time
	camera.Close()
}

// TestStreamCloseUnrefsOnceAcrossCopies is the same guarantee for Stream, which
// the test suite also passes around by value.
func TestStreamCloseUnrefsOnceAcrossCopies(t *testing.T) {
	camera := fakeCamera(t)
	defer camera.Close()

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}

	duplicate := stream
	stream.Close()

	if !duplicate.IsClosed() {
		t.Error("the copy reports IsClosed() = false; close state must be shared")
	}

	duplicate.Close()
	stream.Close()
}

// TestOwnedDeviceCloseUnrefsOnceAcrossCopies covers the reference OpenDevice
// hands to the caller: it must be dropped exactly once, however many copies of
// the Device exist.
func TestOwnedDeviceCloseUnrefsOnceAcrossCopies(t *testing.T) {
	aravis.EnableInterface("Fake")
	aravis.UpdateDeviceList()

	device, err := aravis.OpenDevice("Fake_1")
	if err != nil {
		t.Fatalf("OpenDevice(Fake_1) returned error: %v", err)
	}

	duplicate := device
	device.Close()

	if !device.IsClosed() {
		t.Error("IsClosed() = false after Close(); want true")
	}
	if !duplicate.IsClosed() {
		t.Error("the copy reports IsClosed() = false; ownership must be shared")
	}

	duplicate.Close()
	device.Close()
}

// TestOpenDeviceFirstAvailable exercises the NULL sentinel: Aravis opens the
// first available device only when device_id is NULL, and C.CString("") would
// instead look for a device whose id is the empty string, which nothing
// matches.
//
// This cannot be asserted without real hardware. Aravis's Fake backend does not
// implement the first-device lookup — arv_open_device(NULL) reports
// "device not found" even while the interface enumerates Fake_1 — so the test
// skips rather than pretending to cover the contract.
func TestOpenDeviceFirstAvailable(t *testing.T) {
	aravis.EnableInterface("Fake")
	aravis.UpdateDeviceList()

	device, err := aravis.OpenDevice("")
	if err != nil {
		t.Skipf("OpenDevice(\"\") = %v; no backend here implements the first-device lookup", err)
	}
	defer device.Close()

	if device.IsClosed() {
		t.Error("OpenDevice(\"\") returned a closed device")
	}
}

// TestBorrowedDeviceCloseDoesNotUnref pins the other half of the ownership
// split: Camera.GetDevice only borrows the camera's device, so closing it must
// not drop a reference the camera still holds. The camera has to stay usable
// afterwards, which is what the trailing call checks.
func TestBorrowedDeviceCloseDoesNotUnref(t *testing.T) {
	camera := fakeCamera(t)
	defer camera.Close()

	device, err := camera.GetDevice()
	if err != nil {
		t.Fatalf("GetDevice() returned error: %v", err)
	}

	device.Close()
	device.Close()

	if _, err := camera.GetVendorName(); err != nil {
		t.Errorf("GetVendorName() after closing the borrowed device: %v; the camera must still own it", err)
	}
}

// TestSetControlLostHandlerOnClosedCamera checks the guard. The old code stored
// the handler in a package global regardless, reporting success for a callback
// that could never fire.
func TestSetControlLostHandlerOnClosedCamera(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var camera aravis.Camera

		if err := camera.SetControlLostHandler(func() {}); err == nil {
			t.Error("SetControlLostHandler on a zero-value camera returned nil; want an error")
		}
	})

	t.Run("closed camera", func(t *testing.T) {
		camera := fakeCamera(t)
		camera.Close()

		if err := camera.SetControlLostHandler(func() {}); err == nil {
			t.Error("SetControlLostHandler on a closed camera returned nil; want an error")
		}
	})
}

// TestSetControlLostHandlerReplacesAcrossCopies makes sure a copy of a Camera
// replaces the one registration instead of connecting a second signal handler
// of its own. With the registration state stored per Go value, both copies
// would see key zero and each connect.
func TestSetControlLostHandlerReplacesAcrossCopies(t *testing.T) {
	camera := fakeCamera(t)
	defer camera.Close()

	duplicate := camera

	if err := camera.SetControlLostHandler(func() {}); err != nil {
		t.Fatalf("SetControlLostHandler() returned error: %v", err)
	}
	if err := duplicate.SetControlLostHandler(func() {}); err != nil {
		t.Fatalf("SetControlLostHandler() on the copy returned error: %v", err)
	}

	// Removing through either value must clear the single registration, so a
	// second removal has nothing left to do and must stay quiet.
	if err := duplicate.SetControlLostHandler(nil); err != nil {
		t.Fatalf("SetControlLostHandler(nil) returned error: %v", err)
	}
	if err := camera.SetControlLostHandler(nil); err != nil {
		t.Fatalf("SetControlLostHandler(nil) on the original returned error: %v", err)
	}
}

// TestSetControlLostHandlerConcurrent hammers the registry that replaced the
// unsynchronized package-global handler. The camera is real, so every call
// gets past the guards and actually reads and writes controlLost.handlers —
// with a zero-value camera the calls would return at the nil check and this
// test would stay green even with the synchronization removed.
func TestSetControlLostHandlerConcurrent(t *testing.T) {
	const (
		goroutines = 8
		iterations = 50
	)

	camera := fakeCamera(t)
	defer camera.Close()

	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		// Each goroutine works through its own copy of the Camera value, which
		// is how callers use it and which must still address one registration.
		go func(camera aravis.Camera) {
			defer wg.Done()

			for n := 0; n < iterations; n++ {
				if err := camera.SetControlLostHandler(func() {}); err != nil {
					t.Errorf("SetControlLostHandler() returned error: %v", err)
					return
				}
				if err := camera.SetControlLostHandler(nil); err != nil {
					t.Errorf("SetControlLostHandler(nil) returned error: %v", err)
					return
				}
			}
		}(camera)
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
