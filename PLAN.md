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
  row/column panicked. Fixed: added a `sample()` accessor that clamps neighbor
  coordinates to the image bounds (edge replication); also corrected the alpha
  channel from `0` (fully transparent) to `0xff` and fixed the green sample in
  the bottom-right case.
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
  `GError**` and to guard against a NULL feature (which would otherwise crash).
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

- [ ] **Error "pool" is a no-op.** `internal.go`'s `sync.Pool` is `Get`-only
  (never `Put`), so it never reuses anything and just adds overhead; the comment
  admits it. Delete the pool and allocate directly, or implement real reuse.
- [ ] **`commonErrors` hands out shared mutable `*AravisError` pointers.**
  Combined with the exported `Reset()`, two goroutines can share one struct.
  Return copies, or make the shared errors immutable and drop `Reset()`.
- [ ] **`GetDataInto` is not zero-allocation.** It calls `C.GoBytes` (a full
  allocating copy) then copies again — slower and more allocating than a plain
  copy. Rewrite to copy directly from the C pointer into `dest` via
  `unsafe.Slice`, with no intermediate allocation.
- [ ] **Remove unused `fastError`** in `internal.go` (dead code).
- [ ] **`CleanupPerformanceCache` use-after-free hazard.** It frees cached C
  strings that may still be in use by in-flight `Fast` calls / cached elsewhere.
  Document the contract clearly or make it safe.

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

- [ ] **Dockerfile is broken.** `go build -mod=vendor -o /src/listdevices
  ./examples/list_devices.go` references a nonexistent path (it is
  `examples/list_devices/main.go`) and `-mod=vendor` with no `vendor/` dir; base
  image `golang:1.21` can't satisfy `go 1.23`. `make docker-build`/`docker-test`
  cannot pass.
- [ ] **CI Go version can't build the module.** `go.mod` needs 1.23 but CI jobs
  run 1.21/1.22 → build failure. Align CI matrix with the chosen minimum.
- [ ] **Deprecated/insecure GitHub Actions.** `securecodewarrior/github-action-gosec@master`
  (unmaintained, pinned to a moving `master`; use `securego/gosec`),
  `actions/upload-artifact@v3` and `actions/cache@v3` (EOL),
  `setup-go@v4`, `codecov-action@v3`, `codeql-action@v2`. Pin to current major
  versions.
- [ ] **`.golangci.toml` is a copy-pasted template from another project.** It
  references `viper.Bind`, `cobra.MinimumNArgs`, `cmd/.*`, `http.ResponseWriter`,
  `*Gateway`, envconfig/release-please — none exist here. Replace with a config
  that matches this library.
- [ ] **Formatting is not actually enforced.** CI runs `make fmt` (treefmt with
  `--allow-missing-formatter`) but `install-tools` never installs
  prettier/gofumpt/gci, so the formatters silently skip and the diff check
  passes vacuously.
- [ ] **arm64 "multi-platform" job is a placeholder no-op** that just echoes a
  message — remove it or implement real cross-compilation, don't imply support.
- [ ] **`.gitignore` misses `coverage.out`/`coverage.html`** which the Makefile
  generates.

### P5 — Tests (make the suite mean something)

- [ ] **Add real unit tests for pure-Go logic** (no hardware needed):
  `bayer.go` `At()` (incl. the edge-panic regression), `internal.go`
  `errorFromGError` mapping and `toBool`, `GetDataInto` clamp, and
  `getCachedCString` caching correctness. Today these files have ~0% coverage.
- [ ] **Replace log-only "tests" with real assertions or delete them.** Many
  tests (`TestErrorHandling`, `TestConstants`, `TestCameraCreationWithoutDevice`,
  etc.) only `t.Logf` and cannot fail, inflating a false sense of coverage.
- [ ] **Introduce a seam over the C calls** so a genuine fake can be injected
  (`mock_test.go` currently contains no mock and calls the real C layer).
- [ ] **Add `-race` tests** that hammer `getCachedCString`/`CleanupPerformanceCache`
  and `SetControlLostHandler` concurrently (both have real race exposure).
- [ ] **Fix `TimeoutPopBuffer(1000)` unit bug** in `buffer_test.go` — the literal
  is 1000 ns (1 µs), not the "1 second" the comment claims.
- [ ] **Stop advertising unrunnable benchmarks** as measured results, or make
  them run on synthetic filled buffers so numbers are reproducible in CI.

### Suggested execution order

1. P0 correctness bugs (safety) → 2. P4 build/CI (so changes are verifiable) →
3. P5 pure-Go tests (lock in behavior) → 4. P1/P2 API + perf machinery →
5. P3 docs (describe the now-true reality).
