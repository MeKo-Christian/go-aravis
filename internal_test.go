package aravis

// This file deliberately contains no cgo: Go forbids `import "C"` in _test.go
// files. It exercises the pure-Go half of the error plumbing (newAravisError)
// plus toBool, which accepts untyped constants without a C conversion.

import "testing"

// glibFallback stands in for the message GLib produced. Known error codes must
// override it with the table message; unknown codes must keep it verbatim.
const glibFallback = "glib says something else"

// TestErrorMessagesTableNotEmpty guards the table-driven tests below against
// vacuously passing if errorMessages ever loses its entries.
func TestErrorMessagesTableNotEmpty(t *testing.T) {
	if len(errorMessages) == 0 {
		t.Fatal("errorMessages is empty; the known-code tests would pass vacuously")
	}
}

// TestNewAravisErrorKnownCode checks that every known Aravis device code is
// reported with its stable table message rather than whatever GLib carried.
func TestNewAravisErrorKnownCode(t *testing.T) {
	if len(errorMessages) == 0 {
		t.Fatal("errorMessages is empty")
	}

	for code, want := range errorMessages {
		err := newAravisError(code, glibFallback)

		if err.Code != code {
			t.Errorf("newAravisError(%d).Code = %d, want %d", code, err.Code, code)
		}
		if err.Message != want {
			t.Errorf("newAravisError(%d).Message = %q, want %q", code, err.Message, want)
		}
		if err.Message == glibFallback {
			t.Errorf("newAravisError(%d) kept the GLib text %q, want table message %q", code, glibFallback, want)
		}
		if got := err.Error(); got != want {
			t.Errorf("newAravisError(%d).Error() = %q, want %q", code, got, want)
		}
	}
}

// TestNewAravisErrorUnknownCode checks that an unmapped code keeps the message
// GLib produced, verbatim.
func TestNewAravisErrorUnknownCode(t *testing.T) {
	const code = -12345

	if msg, ok := errorMessages[code]; ok {
		t.Fatalf("code %d unexpectedly present in errorMessages with message %q", code, msg)
	}

	err := newAravisError(code, glibFallback)

	if err.Code != code {
		t.Errorf("Code = %d, want %d", err.Code, code)
	}
	if err.Message != glibFallback {
		t.Errorf("Message = %q, want %q", err.Message, glibFallback)
	}
}

// TestNewAravisErrorReturnsDistinctValues is the regression test for the
// shared-pointer bug: known codes used to return one package-level *AravisError
// to every caller, so any mutation was visible to all of them.
func TestNewAravisErrorReturnsDistinctValues(t *testing.T) {
	var code int
	var want string
	for c, msg := range errorMessages {
		code, want = c, msg
		break
	}
	if want == "" {
		t.Fatal("errorMessages has no entry to test with")
	}

	first := newAravisError(code, "first")
	second := newAravisError(code, "second")

	if first == second {
		t.Fatalf("newAravisError returned the same pointer twice (%p), want distinct values", first)
	}

	first.Message = "mutated by caller"

	if second.Message != want {
		t.Errorf("second.Message = %q after mutating first, want %q", second.Message, want)
	}
	if msg := errorMessages[code]; msg != want {
		t.Errorf("errorMessages[%d] = %q after mutating a returned error, want %q", code, msg, want)
	}
}

// TestToBool covers the C gboolean to Go bool conversion, including nonzero
// values other than 1.
func TestToBool(t *testing.T) {
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"zero is false", toBool(0), false},
		{"one is true", toBool(1), true},
		{"nonzero is true", toBool(2), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("toBool = %v, want %v", tt.got, tt.want)
			}
		})
	}
}
