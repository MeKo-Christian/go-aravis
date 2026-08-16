package aravis

// This file deliberately contains no cgo: Go forbids `import "C"` in _test.go
// files. closeFlag is pure Go precisely so the release-exactly-once rule can be
// tested here, without a camera and without the C layer.

import (
	"sync"
	"testing"
)

// TestCloseFlagClaimOnce is the rule the wrapper types depend on: whichever
// copy calls Close first gets to unref, and no one else does.
func TestCloseFlagClaimOnce(t *testing.T) {
	flag := newCloseFlag()

	if flag.isClosed() {
		t.Error("isClosed() = true on a fresh flag; want false")
	}

	if !flag.claim() {
		t.Error("first claim() = false; want true")
	}

	if flag.claim() {
		t.Error("second claim() = true; the object would be released twice")
	}

	if !flag.isClosed() {
		t.Error("isClosed() = false after a successful claim; want true")
	}
}

// TestCloseFlagNilOwnsNothing covers the borrowed case: Camera.GetDevice hands
// out a device the camera owns, and a zero-value wrapper owns nothing at all.
// Both carry a nil flag, and neither may ever unref.
func TestCloseFlagNilOwnsNothing(t *testing.T) {
	var flag *closeFlag

	if flag.claim() {
		t.Error("claim() on a nil flag = true; a borrowed object must never be released")
	}

	if flag.isClosed() {
		t.Error("isClosed() on a nil flag = true; want false")
	}
}

// TestCloseFlagClaimIsAtomic checks that exactly one of many concurrent closers
// wins. Wrapper values are copied across goroutines, so two of them racing on
// Close must still produce a single unref.
func TestCloseFlagClaimIsAtomic(t *testing.T) {
	const goroutines = 64

	flag := newCloseFlag()

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		claims int
	)

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			if flag.claim() {
				mu.Lock()
				claims++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if claims != 1 {
		t.Errorf("claim() succeeded %d times across %d goroutines; want exactly 1", claims, goroutines)
	}
}
