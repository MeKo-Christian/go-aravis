package tests

import (
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// TestDeviceDiscovery pins what discovery reports against the Fake backend
// TestMain selects: exactly one device, named Fake_1. The old version logged
// whatever it found and only checked ids inside a loop that never ran when no
// device was present.
//
// The repeated UpdateDeviceList covers idempotence, which used to be a separate
// test that called it three times and asserted nothing beyond the count being
// stable.
func TestDeviceDiscovery(t *testing.T) {
	numDevices, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("GetNumDevices() returned error: %v", err)
	}

	if numDevices != 1 {
		t.Fatalf("GetNumDevices() = %d, want 1 (the Fake backend produces exactly one device)", numDevices)
	}

	id, err := aravis.GetDeviceId(0)
	if err != nil {
		t.Fatalf("GetDeviceId(0) returned error: %v", err)
	}

	if id != fakeDeviceID {
		t.Errorf("GetDeviceId(0) = %q, want %q", id, fakeDeviceID)
	}

	aravis.UpdateDeviceList()

	again, err := aravis.GetNumDevices()
	if err != nil {
		t.Fatalf("GetNumDevices() after a second UpdateDeviceList returned error: %v", err)
	}

	if again != numDevices {
		t.Errorf("GetNumDevices() = %d after re-scanning, want %d; discovery must be idempotent", again, numDevices)
	}
}

// TestInterfaceDiscovery checks that every enumerated interface has a usable id
// and that the Fake backend is among them.
//
// The count is deliberately not asserted: which interfaces libaravis enumerates
// depends on how it was built, so pinning a number here would fail on a
// perfectly good build rather than catch a bug.
func TestInterfaceDiscovery(t *testing.T) {
	numInterfaces, err := aravis.GetNumInterface()
	if err != nil {
		t.Fatalf("GetNumInterface() returned error: %v", err)
	}

	if numInterfaces == 0 {
		t.Fatal("GetNumInterface() = 0, want at least the Fake interface")
	}

	foundFake := false

	for i := range numInterfaces {
		id, err := aravis.GetInterfaceId(i)
		if err != nil {
			t.Errorf("GetInterfaceId(%d) returned error: %v", i, err)

			continue
		}

		if id == "" {
			t.Errorf("GetInterfaceId(%d) returned an empty id", i)
		}

		if id == fakeInterface {
			foundFake = true
		}
	}

	if !foundFake {
		t.Errorf("no %q interface among the %d enumerated; this libaravis build cannot run the suite",
			fakeInterface, numInterfaces)
	}
}

// TestOutOfRangeIdsReturnEmptyString covers the accessor boundary. Aravis
// returns NULL for an index past the end, which C.GoString turns into "".
//
// The error is deliberately not asserted: GetDeviceId and GetInterfaceId use
// cgo's two-result form, so their second result is errno, not an Aravis
// failure (see P6 in PLAN.md) — both return a nil error here today. Asserting a
// non-nil error would encode that bug into the test suite; asserting a nil one
// would encode the stale-errno behaviour. The string is the contract.
func TestOutOfRangeIdsReturnEmptyString(t *testing.T) {
	numDevices, _ := aravis.GetNumDevices()
	numInterfaces, _ := aravis.GetNumInterface()

	tests := []struct {
		name  string
		index uint
		get   func(uint) (string, error)
	}{
		{"device past the end", numDevices + 100, aravis.GetDeviceId},
		{"device at max uint", ^uint(0), aravis.GetDeviceId},
		{"interface past the end", numInterfaces + 100, aravis.GetInterfaceId},
		{"interface at max uint", ^uint(0), aravis.GetInterfaceId},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if id, _ := tt.get(tt.index); id != "" {
				t.Errorf("id = %q, want the empty string", id)
			}
		})
	}
}

// TestEnableDisableInterfaceChangesDiscovery pins what EnableInterface and
// DisableInterface actually do: they add and remove an interface from the set
// UpdateDeviceList scans. The test this replaces only checked that the calls
// did not crash, which no implementation could fail.
//
// It is the one test that mutates the process-wide backend selection TestMain
// established, so it restores it.
func TestEnableDisableInterfaceChangesDiscovery(t *testing.T) {
	t.Cleanup(func() {
		aravis.EnableInterface(fakeInterface)
		aravis.UpdateDeviceList()
	})

	aravis.DisableInterface(fakeInterface)
	aravis.UpdateDeviceList()

	if n, _ := aravis.GetNumDevices(); n != 0 {
		t.Fatalf("GetNumDevices() = %d after disabling %q, want 0", n, fakeInterface)
	}

	aravis.EnableInterface(fakeInterface)
	aravis.UpdateDeviceList()

	if n, _ := aravis.GetNumDevices(); n != 1 {
		t.Fatalf("GetNumDevices() = %d after enabling %q, want 1", n, fakeInterface)
	}

	// An unknown id is ignored by Aravis: it must not disturb the device list.
	aravis.DisableInterface("nonexistent-interface-12345")
	aravis.EnableInterface("")
	aravis.UpdateDeviceList()

	if n, _ := aravis.GetNumDevices(); n != 1 {
		t.Errorf("GetNumDevices() = %d after enabling/disabling unknown ids, want 1", n)
	}
}
