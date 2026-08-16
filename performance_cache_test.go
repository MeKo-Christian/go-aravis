package aravis

// This file deliberately contains no cgo: Go forbids `import "C"` in _test.go
// files. getCachedCString's first result is a C type, so it is only ever
// assigned to the blank identifier here — the interesting result is the
// ownership flag.

import "testing"

// TestGetCachedCStringInternsOnlyCommonFeatures pins the bound on the string
// cache: names interned at startup are reused, anything else gets a temporary
// the caller frees. Without this, a process passing generated or user-supplied
// feature names to the *FeatureValueFast methods would grow the C heap forever,
// and the cache has no eviction path.
func TestGetCachedCStringInternsOnlyCommonFeatures(t *testing.T) {
	if len(commonFeatures) == 0 {
		t.Fatal("commonFeatures is empty; the cached-name case would pass vacuously")
	}

	sizeBefore := len(cStringCache)

	for _, feature := range commonFeatures {
		if _, mustFree := getCachedCString(feature); mustFree {
			t.Errorf("getCachedCString(%q) reported a temporary; common features must be interned", feature)
		}
	}

	// Names the library never interned must not enter the cache, however many
	// distinct ones a caller invents.
	uncommon := []string{
		"Uncommon_Feature_0",
		"Uncommon_Feature_1",
		"Uncommon_Feature_2",
	}
	for _, feature := range uncommon {
		if _, mustFree := getCachedCString(feature); !mustFree {
			t.Errorf("getCachedCString(%q) returned a cached string; arbitrary names must be temporary", feature)
		}
	}

	if got := len(cStringCache); got != sizeBefore {
		t.Errorf("cache grew from %d to %d entries; it must stay fixed after init", sizeBefore, got)
	}
}

// TestCStringCacheCoversCommonFeatures checks that init actually interned every
// declared name — the cached path above is only meaningful if it did.
func TestCStringCacheCoversCommonFeatures(t *testing.T) {
	if len(cStringCache) != len(commonFeatures) {
		t.Errorf("cache holds %d entries, want %d (one per common feature)", len(cStringCache), len(commonFeatures))
	}

	for _, feature := range commonFeatures {
		if _, ok := cStringCache[feature]; !ok {
			t.Errorf("common feature %q was not interned at startup", feature)
		}
	}
}
