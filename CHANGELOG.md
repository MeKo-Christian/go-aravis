# Changelog

All notable changes to this fork are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Provenance

This repository is a fork of [hybridgroup/go-aravis](https://github.com/hybridgroup/go-aravis),
which is itself a fork of [thinkski/go-aravis](https://github.com/thinkski/go-aravis) by
Chris Hiszpanski. The original targeted libaravis 0.6; the Hybrid Group port moved it to
Aravis 0.8 between 2019 and 2022. This fork continues from there. Both prior copyrights
are retained in [LICENSE](LICENSE), and the project remains BSD 3-Clause.

Entries below cover changes made in this fork. Upstream history is in the git log.

## [Unreleased]

Changes since the last upstream commit. A quality review of the source, tests, docs,
examples and CI produced the work tracked in `PLAN.md`; these are the results.

### Breaking

- **Module path renamed** from `github.com/hybridgroup/go-aravis` to
  `github.com/MeKo-Christian/go-aravis`. The old path did not match the repository, so
  the fork could not be resolved by `go get`. The package name (`aravis`) is unchanged,
  so migrating is an import-path edit.
- **`Interface*` stubs removed.** `InterfaceGetNDevices`, `InterfaceGetDeviceId` and the
  other three empty stubs are gone. All of them needed an `ArvInterface *` handle, and
  this wrapper has no `Interface` type to hold one, so they could never have worked.
- **`OpenDevice` signature changed** to `OpenDevice(id string) (Device, error)`. It was
  previously an empty stub.
- **`CleanupPerformanceCache` removed.** It freed cached C strings that in-flight `Fast`
  calls could still hold, and `getCachedCString` hands out raw `*C.char` with no
  refcounting, so the use-after-free could not be fixed while keeping the function. The
  cache is now deliberately retained for the process lifetime: it interns GenICam feature
  names, bounded by the feature set a program touches.
- **`Reset()` removed** from the error type. It existed only to serve the deleted error
  pool and had no callers.
- **Device setters now return `error`.** `SetStringFeatureValue`, `SetIntegerFeatureValue`,
  `SetFloatFeatureValue` and `SetNodeFeatureValue` previously discarded failures.

### Added

- Package documentation (`doc.go`) covering the acquisition call order, the value-type
  and Close semantics, buffer-data lifetime rules, error handling, concurrency, and GigE
  Vision setup. Godoc comments on essentially every exported symbol.
- `Camera.IsClosed`, `Stream.IsClosed`, `Device.IsClosed`.
- `Device.Close`, with an explicit ownership model: `OpenDevice` transfers ownership,
  `Camera.GetDevice` only borrows.
- Implementations for the 11 empty `camera.go` stubs: `SetBinning`, `SetPixelFormat`,
  `GetPixelFormat`, `GetPixelFormatAsString`, `SetPixelFormatFromString`,
  `GetAvailablePixelFormats`, `GetAvailablePixelFormatsAsDisplayNames`,
  `GetAvailablePixelFormatsAsStrings`, `GetExposureTimeBounds`, `GetExposureTimeAuto`,
  `SetGainAuto`.
- `GetNumInterface`, replacing the misspelled `GetNumInferface`, which remains as a
  deprecated forwarding wrapper.
- `Camera.GetGainAuto`, the missing counterpart to `SetGainAuto`. It wraps
  `arv_camera_get_gain_auto` and returns one of the `AUTO_*` constants, so an automatic
  gain mode can now be read back.
- Tests that can fail: Bayer edge handling, error mapping and `toBool`, `GetDataInto`
  semantics including a `testing.AllocsPerRun` zero-allocation assertion, and a `-race`
  concurrency test for control-lost handlers. Buffer tests seed a real payload through
  Aravis's built-in Fake backend, so they need no hardware.
- `CHANGELOG.md` and a provenance section in the README.

### Fixed

- **32/64-bit out-param corruption.** `GetSensorSize`, `GetRegion`, `GetHeightBounds`,
  `GetWidthBounds` and `GetBinning` passed a pointer to a 64-bit Go `int` where Aravis
  writes 4 bytes, leaving half the value undefined. The same bug affected the `size`
  out-param in `GetData`, `GetDataUnsafe`, `GetDataSlice` and `GetPartData`.
- **`BayerRG.At` read out of bounds** at the image edges and panicked on the last
  row/column. Out-of-range neighbors are now mirrored back across the edge, which
  preserves the CFA phase — clamping would collapse onto the current Bayer site and
  produce the wrong color on odd-sized images. The alpha channel was also `0` (fully
  transparent) instead of `0xff`.
- **`BayerRG.At` used the wrong CFA phase for a rect with an odd origin.** It derived the
  Bayer site from the absolute coordinates (`x&1`, `y&1`) while `sample` indexes `Pix`
  relative to `Rect.Min`, so for any `Rect` with an odd `Min.X` or `Min.Y` red and blue
  swapped and the green samples came from the wrong neighbors. The phase is now taken
  relative to `Rect.Min`, so the first sample of `Pix` is red for any origin.
- **`GetFrameRateBounds` and `GetGainBounds` wrote their C out-parameters through a
  `*float64` reinterpreted as `*C.double`.** This is the same class as the 32/64-bit bug
  above and worked only because Go `float64` and C `double` happen to be 8 bytes on the
  supported platforms. Both now declare `C.double` locals and convert on return, as
  `GetExposureTimeBounds` already did.
- **Empty device id passed to Aravis as `""` instead of NULL.** Aravis documents NULL as
  the "first available device" sentinel, so `NewCamera("")` and `OpenDevice("")` asked for
  a device whose id is the empty string, which nothing matches.
- **`Close` was unguarded and not idempotent.** Because `Camera`, `Stream` and `Device`
  are copied by value, a per-value guard still let two copies unref the same object
  twice. Lifecycle state now lives behind a shared close flag, so Close succeeds exactly
  once per underlying object. Control-lost registration moved into the same shared state
  for the same reason.
- **`SetControlLostHandler` used a single package-global handler,** so a second camera
  overwrote the first camera's callback. Handlers now live in a mutex-guarded per-camera
  registry, with the key travelling through GLib as the signal's `user_data`. Two further
  bugs fell out: the signal was only connected inside `CreateStream`, so a handler on a
  camera that never streamed could never fire, and it was reconnected on every
  `CreateStream` call, stacking duplicates. Connecting now happens once, in
  `SetControlLostHandler` itself, and `Close` disconnects.
- **`SetLineRate` discarded its error.**
- **`arv_set_node_feature_value` crashed on a NULL feature.** It now reports
  `ARV_DEVICE_ERROR_FEATURE_NOT_FOUND` instead.
- `GetDataSlice` no longer uses the deprecated `reflect.SliceHeader`.
- **`get_image` example error handling**: `http.Error` calls fell through instead of
  returning, `TimeoutPopBuffer`'s error went unchecked, and a bad-status branch
  dereferenced a possibly-nil error.

### Changed

- **`GetDataInto` is genuinely zero-allocation.** Reaching zero required removing the
  `size` out-param: passing a Go pointer to `arv_buffer_get_data` makes cgo heap-allocate
  the local on every call, so a C wrapper now returns data and size together in a struct.
  Asserted by `TestGetDataIntoZeroAllocations`.
- **Documentation corrected.** The README used a `SetThreadPriority` method that does not
  exist (`Camera.ThreadPriority` is a struct field), described the blocking
  `TimeoutPopBuffer` as non-blocking while omitting the genuinely non-blocking
  `TryPopBuffer`, named `SetGVPacketSize`/`SetGVPacketDelay` instead of
  `GVSetPacketSize`/`GVSetPacketDelay`, claimed Go 1.21 against a 1.23 module, and shipped
  snippets that could not compile. Overstated feature claims — "chunk data processing",
  "comprehensive error detection and recovery", "full access to camera parameters" — were
  narrowed to what the code does.
- **Unreproducible benchmark figures removed** from README and PERFORMANCE.md. The
  published table (40% / 80% / 50x / "3.2 GB → 0 KB") was not produced by any benchmark in
  this repository, and the benchmarks that exist skip without hardware. PERFORMANCE.md also
  claimed error pooling reduced allocation overhead after the pool had been deleted.

### Removed

- **No-op error pool.** The `sync.Pool` in `internal.go` was `Get`-only — it never put
  anything back — so it reused nothing and only added overhead.
- **Shared mutable error values.** `commonErrors` handed out shared `*AravisError`
  pointers that any caller could mutate. Replaced by an immutable message table, with a
  fresh caller-owned error built per call.
- Dead `fastError` helper, and commented-out policy methods in `device.go`.

### Build and CI

- **Dockerfile rewritten**: `golang:1.23-bookworm`, installs `libaravis-dev`, builds the
  correct package path, drops a bogus `-mod=vendor`.
- **Go version unified at 1.23** across `go.mod`, the Makefile, the Dockerfile and all CI
  jobs, which previously could not build the module.
- **Aravis dev package renamed.** `ubuntu-latest` moved to 24.04, where
  `libaravis-0.8-dev` became `libaravis-dev` (still Aravis 0.8 — there is no stable 0.10).
  This, not any source change, was the cause of the red CI.
- **Deprecated actions replaced**: the deleted `securecodewarrior/github-action-gosec` for
  `securego/gosec`, plus bumps to `upload-artifact` v4, `cache` v4, `setup-go` v5,
  `checkout` v5, `codecov-action` v5 and `codeql-action` v3.
- **`.golangci.toml`** replaced a copy-pasted template with a curated config for this
  library.
- **Formatting is now enforced**: the CI check runs `gofmt -l .` and fails on any
  unformatted file.
- Removed the arm64 "multi-platform" job, which was a placeholder that built nothing.
  A real arm64 job needs a cross-compiled Aravis, not just `GOARCH`.
- `.gitignore` covers `coverage.out` and `coverage.html`.
