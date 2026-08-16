package tests

import (
	"testing"
	"time"

	aravis "github.com/hybridgroup/go-aravis"
)

// seededBuffer returns a filled buffer whose bytes follow a deterministic
// pattern, plus a copy of that pattern for comparison.
//
// A buffer straight out of aravis.NewBuffer reports a data size of zero
// (arv_buffer_get_data returns the received size, which is only set once the
// buffer has been filled), so seeding requires a real acquisition. Aravis
// ships a built-in "Fake" interface that produces buffers without any
// hardware; once such a buffer has been popped, GetDataSlice aliases its C
// memory and lets us overwrite the payload with a known pattern.
func seededBuffer(t *testing.T) (aravis.Buffer, []byte) {
	t.Helper()

	aravis.EnableInterface("Fake")
	aravis.UpdateDeviceList()

	camera, err := aravis.NewCamera("Fake_1")
	if err != nil {
		t.Fatalf("NewCamera(Fake_1) returned error: %v", err)
	}
	t.Cleanup(camera.Close)

	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		t.Fatalf("GetPayloadSize() returned error: %v", err)
	}
	if payloadSize == 0 {
		t.Fatalf("GetPayloadSize() = 0, want a non-zero payload")
	}

	stream, err := camera.CreateStream()
	if err != nil {
		t.Fatalf("CreateStream() returned error: %v", err)
	}
	t.Cleanup(stream.Close)

	buf, err := aravis.NewBuffer(payloadSize)
	if err != nil {
		t.Fatalf("NewBuffer(%d) returned error: %v", payloadSize, err)
	}
	if buf.IsNil() {
		t.Fatalf("NewBuffer(%d) returned a nil buffer", payloadSize)
	}
	stream.PushBuffer(buf)

	if err := camera.StartAcquisition(); err != nil {
		t.Fatalf("StartAcquisition() returned error: %v", err)
	}
	t.Cleanup(func() { _ = camera.StopAcquisition() })

	filled, err := stream.TimeoutPopBuffer(5 * time.Second)
	if err != nil {
		t.Fatalf("TimeoutPopBuffer() returned error: %v", err)
	}
	if filled.IsNil() {
		t.Fatalf("TimeoutPopBuffer() returned a nil buffer")
	}
	if status, err := filled.GetStatus(); err != nil || status != aravis.BUFFER_STATUS_SUCCESS {
		t.Fatalf("GetStatus() = %d, %v; want %d, nil", status, err, aravis.BUFFER_STATUS_SUCCESS)
	}

	slice, err := filled.GetDataSlice()
	if err != nil {
		t.Fatalf("GetDataSlice() returned error: %v", err)
	}
	if len(slice) == 0 {
		t.Fatalf("GetDataSlice() returned %d bytes, want a non-empty payload", len(slice))
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

// TestGetDataIntoCopies covers the three length relations between dest and the
// source buffer: shorter, equal, and longer. Every case also guards against
// overrun by pre-filling dest with a sentinel and checking the untouched tail.
func TestGetDataIntoCopies(t *testing.T) {
	const sentinel = 0xEE

	buf, want := seededBuffer(t)
	size := len(want)

	tests := []struct {
		name    string
		destLen int
		wantN   int
	}{
		{name: "dest of one byte", destLen: 1, wantN: 1},
		{name: "dest shorter than buffer", destLen: size / 2, wantN: size / 2},
		{name: "dest equal to buffer", destLen: size, wantN: size},
		{name: "dest longer than buffer", destLen: size + 32, wantN: size},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := make([]byte, tt.destLen)
			for i := range dest {
				dest[i] = sentinel
			}

			n, err := buf.GetDataInto(dest)
			if err != nil {
				t.Fatalf("GetDataInto() returned error: %v", err)
			}
			if n != tt.wantN {
				t.Fatalf("GetDataInto() = %d, want %d", n, tt.wantN)
			}

			for i := 0; i < n; i++ {
				if dest[i] != want[i] {
					t.Fatalf("dest[%d] = %#x, want %#x", i, dest[i], want[i])
				}
			}

			// Nothing beyond n bytes may have been written.
			for i := n; i < len(dest); i++ {
				if dest[i] != sentinel {
					t.Fatalf("dest[%d] = %#x, want untouched sentinel %#x", i, dest[i], sentinel)
				}
			}
		})
	}
}

// TestGetDataIntoEmptyDest checks that a zero-length and a nil destination are
// handled without panicking and copy nothing.
func TestGetDataIntoEmptyDest(t *testing.T) {
	buf, _ := seededBuffer(t)

	tests := []struct {
		name string
		dest []byte
	}{
		{name: "empty slice", dest: []byte{}},
		{name: "nil slice", dest: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := buf.GetDataInto(tt.dest)
			if err != nil {
				t.Fatalf("GetDataInto() returned error: %v", err)
			}
			if n != 0 {
				t.Errorf("GetDataInto() = %d, want 0", n)
			}
		})
	}
}

// TestGetDataIntoZeroAllocations is the point of the rewrite: the copy must run
// straight out of the C buffer, without the intermediate C.GoBytes allocation
// the previous implementation made.
func TestGetDataIntoZeroAllocations(t *testing.T) {
	buf, want := seededBuffer(t)
	dest := make([]byte, len(want))

	allocs := testing.AllocsPerRun(100, func() {
		if _, err := buf.GetDataInto(dest); err != nil {
			t.Fatalf("GetDataInto() returned error: %v", err)
		}
	})

	if allocs > 0 {
		t.Errorf("GetDataInto() allocated %.1f times per run, want 0", allocs)
	}
}
