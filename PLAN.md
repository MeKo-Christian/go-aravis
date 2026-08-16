# go-aravis — Project Plan

This document tracks planned work on the go-aravis library (a CGO binding for the
Aravis 0.8 machine-vision library).

## Phase 1 — Remediation: fix what can be fixed

A detailed quality review (source, tests, docs, examples, build/CI) surfaced a set
of correctness bugs, false documentation claims, and build/tooling breakage. This
phase captures the concrete, fixable items, ordered by priority. Checkboxes track
progress.

### P0 — Correctness bugs (code can crash, corrupt, or silently misbehave)

- [x] **32-bit/64-bit pointer mismatch in out-params.** In `camera.go`,
      `GetSensorSize`, `GetRegion`, `GetHeightBounds`, `GetWidthBounds`, and
      `GetBinning` declared Go `int` locals (64-bit) and passed
      `(*C.gint)(unsafe.Pointer(&x))` to Aravis, which writes only 4 bytes. Fixed:
      declare `var x, y C.gint` and convert on return.
- [x] **`bayer.go` `At()` reads out of bounds at image edges.** The red/green/blue
      cases indexed `x+1`, `y+1`, `x-1`, `y-1` with no bounds checks, so the last
      row/column panicked. Fixed: added a `sample()` accessor with a `reflectCoord`
      helper that **mirrors** out-of-bounds neighbors back across the edge instead of
      clamping — clamping would collapse onto the current Bayer site and yield the
      wrong color phase on odd-sized images (raised in PR review). Also corrected the
      alpha channel from `0` (fully transparent) to `0xff` and fixed the green sample
      in the bottom-right case. Covered by `tests/bayer_test.go`.
- [x] **`GetDataSlice` uses deprecated `reflect.SliceHeader`.** Replaced with
      `unsafe.Slice((*byte)(data), size)` (plus a nil/zero-size guard) and
      documented the C-memory-aliasing lifetime.
- [x] **`get_image` example error handling.** `http.Error(...)` calls now
      `return`; added an explicit error check after `TimeoutPopBuffer`; and the
      bad-buffer-status branch now reports the status instead of dereferencing a
      possibly-nil `err`.
- [x] **Device setters silently swallow errors.** `SetStringFeatureValue`,
      `SetIntegerFeatureValue`, `SetFloatFeatureValue`, `SetNodeFeatureValue`
      (`device.go`) now pass `&gerror` and return `error`. The
      `arv_set_node_feature_value` C helper was updated to accept/propagate a
      `GError**` and to guard against a NULL feature (which would otherwise crash);
      the NULL-feature case now sets `ARV_DEVICE_ERROR_FEATURE_NOT_FOUND` so the
      error contract holds (raised in PR review).
- [x] **`SetLineRate` discards its error.** Now returns the error from
      `SetFrameRate`.

### P1 — API honesty (public surface that lies about what it does)

- [ ] **Remove or implement the 17 empty `// TODO` stub methods.** These are
      exported and silently no-op: `camera.go` — `SetBinning`, `SetPixelFormat`,
      `GetPixelFormat`, `GetPixelFormatAsString`, `SetPixelFormatFromString`,
      `GetAvailablePixelFormats`, `GetAvailablePixelFormatsAsDisplayNames`,
      `GetAvailablePixelFormatsAsStrings`, `GetExposureTimeBounds`,
      `GetExposureTimeAuto`, `SetGainAuto`; `interface.go` — `OpenDevice`,
      `InterfaceGetDeviceId`, `InterfaceGetDevicePhysicalId`,
      `InterfaceGetDeviceAddress`, `InterfaceGetNumDevices`, `InterfaceOpenDevice`.
      Decision: implement the high-value ones (pixel format, binning, exposure
      bounds) and delete the rest until implemented.
- [ ] **`SetControlLostHandler` uses a single package-global handler.** All
      cameras share one `controlLostHandler`, and it is read from a C callback
      goroutine with no synchronization (data race). Make it per-camera and
      synchronized, or document the single-camera limitation explicitly.
- [ ] **Fix typo `GetNumInferface` → `GetNumInterface`** in `interface.go`
      (keep a deprecated alias if compatibility matters).
- [ ] **`Close()` methods are unguarded.** `Camera.Close`/`Stream.Close` call
      `g_object_unref` with no nil check and no double-close protection; add guards
      and consider `runtime.SetFinalizer` for leak safety.

### P2 — Remove false performance machinery / claims

- [x] **Error "pool" is a no-op.** `internal.go`'s `sync.Pool` is `Get`-only
      (never `Put`), so it never reuses anything and just adds overhead; the comment
      admits it. Deleted the pool (and the now-unused `sync` import); errors are
      allocated directly.
- [x] **`commonErrors` hands out shared mutable `*AravisError` pointers.**
      Replaced by `errorMessages map[int]string` — a lookup table of immutable
      strings, not a value store. The pure-Go `newAravisError(code, glibMessage)`
      now builds a fresh, caller-owned `*AravisError` on every call, and the
      exported `Reset()` is gone (breaking, but it had zero callers and existed
      only to serve the dead pool). Covered by `internal_test.go`, including a
      distinct-pointer regression test.
- [x] **`GetDataInto` is not zero-allocation.** Rewritten to a single
      `copy(dest, unsafe.Slice((*byte)(data), n))` straight out of the C
      buffer, plus the nil/zero guards `GetDataSlice` already had. Reaching a true
      zero also required removing the `size` out-param: passing a Go pointer to
      `arv_buffer_get_data` makes cgo heap-allocate the local on every call. The
      `arv_go_buffer_get_data` wrapper returns data+size in a struct instead, so
      no Go pointer crosses the boundary. (`#cgo noescape` works too, but needs a
      Go 1.23 language version — it broke the CI build on the 1.23 baseline.)
      Verified by a `testing.AllocsPerRun` assertion in
      `tests/buffer_data_test.go`. Also fixed
      the same 32/64-bit out-param width bug as P0 in `GetData`, `GetDataUnsafe`,
      `GetDataSlice`, and `GetPartData` (`var size int` → `var size C.size_t`).
- [x] **Remove unused `fastError`** in `internal.go` (dead code). Removed (also
      dropped the now-unused `errors` import).
- [x] **`CleanupPerformanceCache` use-after-free hazard.** Removed (breaking).
      It freed cached C strings that in-flight `Fast` calls may still hold, and
      `getCachedCString` hands out raw `*C.char` with no refcounting, so freeing
      could not be made safe. The cache is now documented as deliberately retained:
      interned GenICam feature names, bounded by the feature set a program touches
      (tens of entries, a few hundred bytes). Its two log-only test call sites and
      the `## Cleanup` section of `PERFORMANCE.md` are gone.

### P3 — Documentation accuracy (README / PERFORMANCE.md)

- [ ] **Module path vs repo mismatch.** `go.mod` says
      `github.com/hybridgroup/go-aravis` but the repo is
      `github.com/MeKo-Christian/go-aravis`, so the fork is not `go get`-able by its
      own path. Decide: rename the module to the fork path (and update all imports
      and docs), or document the required `replace` directive.
- [ ] **`SetThreadPriority(...)` does not exist.** README uses it 4× (lines ~152,
      343–345). The real API is the `Camera.ThreadPriority` struct field. Fix the docs.
- [ ] **Remove/annotate vaporware feature claims.** "Multiple Pixel Formats" and
      "Full access to camera parameters" reference stubbed methods (see P1).
- [ ] **Correct the false performance claims.** "Zero-allocation `GetDataInto`"
      and "error pooling" are false at the code level; the benchmark numbers
      (40%/80%/50x/"3.2 GB → 0 KB") are unreproducible (all benchmarks skip without
      hardware and run on never-filled buffers). Remove or clearly mark as
      aspirational, and commit reproducible numbers if kept.
- [ ] **Fix Go version inconsistency.** README says "Go 1.21+", `go.mod` requires
      1.23, Makefile says 1.21, CI uses 1.21/1.22; examples use integer `range`
      (needs 1.22+). Pick one minimum (>= 1.22) and make everything consistent.
- [ ] **Fix non-compiling README snippets** (`SetThreadPriority`, unused
      `err`/`numParts`, undeclared `frameRate`/`payloadSize`) and mislabeled
      "`TimeoutPopBuffer` — Non-blocking" (it blocks with a timeout; `TryPopBuffer`
      is the non-blocking one and is undocumented).
- [ ] **Add package doc / godoc comments.** No `doc.go`, most exported symbols in
      `camera.go`/`device.go`/`interface.go`/`stream.go` are undocumented.
- [ ] **Clarify fork provenance/attribution.** LICENSE (Chris Hiszpanski),
      module owner (hybridgroup), and repo owner (MeKo-Christian) are three parties;
      state the fork's provenance and add a CHANGELOG for what this fork changed.

### P4 — Build, CI, and tooling

Context: investigating the red CI on PR #1 showed the failures were pre-existing
environmental rot, not the P0 changes. `ubuntu-latest` moved to 24.04, where the
Aravis dev package was **renamed** from `libaravis-0.8-dev` to `libaravis-dev`
(still Aravis 0.8 — there is no stable 0.10). Two CI actions were also deleted or
hard-deprecated. The library stays on Aravis 0.8 (current stable).

- [x] **Dockerfile is broken.** Rewritten: `golang:1.23-bookworm` base, installs
      `libaravis-dev` via apt, builds the correct package path
      (`./examples/list_devices`), drops the bogus `-mod=vendor` and the
      `arv-tool-0.8` invocation. (Base-image pull is proxy-blocked in the review
      sandbox, so the final `docker build` runs in CI; the build command and package
      availability were verified locally.)
- [x] **CI Go version can't build the module.** All jobs now use Go 1.23 via a
      single `GO_VERSION` env, matching `go.mod`.
- [x] **Deprecated/insecure GitHub Actions.** Replaced the deleted
      `securecodewarrior/github-action-gosec` with `securego/gosec`; bumped
      `upload-artifact`→v4, `cache`→v4, `setup-go`→v5, `checkout`→v5,
      `codecov-action`→v5, `codeql-action`→v3.
- [x] **`.golangci.toml` was a copy-pasted template.** Replaced with a curated
      config for this library (standard linters + a few high-value ones, examples/
      and tests excluded from `errcheck`). Verified clean with `golangci-lint` 2.5.0.
- [x] **Formatting is not actually enforced.** The CI format check now runs
      `gofmt -l .` directly and fails on any unformatted file.
- [x] **arm64 "multi-platform" job is a placeholder no-op.** Removed; kept a real
      linux/amd64 build job, with a comment on what a real arm64 cross-build needs.
- [x] **`.gitignore` misses `coverage.out`/`coverage.html`.** Added.

### P5 — Tests (make the suite mean something)

- [~] **Add real unit tests for pure-Go logic** (no hardware needed): **Done for
  `bayer.go`** — `tests/bayer_test.go` covers the edge-panic regression, the
  opaque-alpha fix, and the CFA edge-phase reflection. **Done for the error
  mapping and `toBool`** — `internal_test.go` (repo root, `package aravis`;
  note Go forbids `import "C"` in `_test.go` files, which is why the pure-Go
  `newAravisError` was split out of the cgo glue). **Done for `GetDataInto`** —
  `tests/buffer_data_test.go` covers the clamp, overrun, empty/nil-dest, and
  zero-allocation cases, seeding a real payload through Aravis's built-in Fake
  interface (no hardware). Still TODO: `getCachedCString` caching correctness.
- [ ] **Replace log-only "tests" with real assertions or delete them.** Many
      tests (`TestErrorHandling`, `TestConstants`, `TestCameraCreationWithoutDevice`,
      etc.) only `t.Logf` and cannot fail, inflating a false sense of coverage.
- [ ] **Introduce a seam over the C calls** so a genuine fake can be injected
      (`mock_test.go` currently contains no mock and calls the real C layer).
- [ ] **Add `-race` tests** that hammer `getCachedCString` and
      `SetControlLostHandler` concurrently (both have real race exposure).
      `CleanupPerformanceCache` no longer exists (see P2).
- [ ] **Fix `TimeoutPopBuffer(1000)` unit bug** in `buffer_test.go` — the literal
      is 1000 ns (1 µs), not the "1 second" the comment claims.
- [ ] **Stop advertising unrunnable benchmarks** as measured results, or make
      them run on synthetic filled buffers so numbers are reproducible in CI.

### Suggested execution order

1. P0 correctness bugs (safety) → 2. P4 build/CI (so changes are verifiable) →
2. P5 pure-Go tests (lock in behavior) → 4. P1/P2 API + perf machinery →
3. P3 docs (describe the now-true reality).
