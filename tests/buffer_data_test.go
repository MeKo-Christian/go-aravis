package tests

import "testing"

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
