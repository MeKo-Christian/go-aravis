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

## [0.2.0] - 2026-08-16

Changes since the last upstream commit. A quality review of the source, tests, docs,
examples and CI produced the work tracked in `PLAN.md`; these are the results.

`v0.1.0` was tagged part-way through that review, before this section was split into
releases, so it is a snapshot of the same body of work rather than a separate set of
changes. Everything below is present in `v0.2.0`; the subset that predates `v0.1.0` is
identifiable from the git log, not from this file.

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
- **`Stream.PushBuffer` now returns `error`.** It previously handed every argument
  straight to Aravis: a nil stream and a nil buffer each trip a GLib assertion there, and
  pushing a buffer the caller had already given up is a double free. It now returns
  `ErrNilStream`, `ErrStreamClosed`, `ErrNilBuffer` or `ErrBufferNotOwned`. Statement and
  `defer` calls compile unchanged; only assigning the method to a `func(aravis.Buffer)`
  value breaks.
- **Device setters now return `error`.** `SetStringFeatureValue`, `SetIntegerFeatureValue`,
  `SetFloatFeatureValue` and `SetNodeFeatureValue` previously discarded failures. The
  matching getters kept their signatures but changed behaviour — see *The generic
  `Device` feature getters swallowed every GenICam error* under **Fixed**.
- **The accessors that cannot fail no longer return an `error`.** `Buffer.GetData`,
  `GetDataUnsafe`, `GetDataSlice`, `GetDataInto`, `GetStatus`, `GetNumParts` and
  `FindComponent`, together with the package-level `GetDeviceId`, `GetInterfaceId`,
  `GetNumDevices` and `GetNumInterface` (and the deprecated `GetNumInferface`), are now
  single-valued. None of the C functions behind them has a `GError` parameter, so the
  error had nothing to carry and was documented as always nil — see *`errno` was being
  reported as an error* under **Fixed**, which is where the always-nil state came from.
  Every call site loses an `if err != nil` branch on a condition that could not occur.
  Migrating is mechanical: drop the second result. Because removing the error also
  removed the only way a nil receiver could have been reported, the seven `Buffer`
  accessors — and `HasChunks`, which never returned an error — now answer from Go for a
  `Buffer` holding no `ArvBuffer` instead of tripping `ARV_IS_BUFFER` inside Aravis. A
  zero `Buffer` reports no data, `BUFFER_STATUS_UNKNOWN`, zero parts and a component
  index of -1.

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
- **`Buffer.Close` and `Buffer.IsClosed`.** A buffer in the caller's hands could not be
  released at all: `Stream.Close` frees only the buffers still in the stream's queues, so
  a buffer that `NewBuffer` created and nothing ever pushed, and a popped buffer that was
  never pushed back, both leaked with no way out. `Buffer` now carries the same shared
  close flag as `Camera`, `Stream` and `Device`, so `Close` is idempotent across copies.
  The flag doubles as an ownership flag, because ownership ping-pongs: `PushBuffer` claims
  it (the parameter is `transfer-ownership="full"`), and each pop mints a new `Buffer`
  with a new flag (the result is too). At most one Go value holds an unclaimed flag for a
  given `ArvBuffer` at a time.
- **Sentinel errors** in `errors.go`, so the conditions the package decides for itself can
  be matched with `errors.Is` instead of by string: `ErrTimeout`, `ErrNoBuffer`,
  `ErrNegativeTimeout`, `ErrNilStream`, `ErrStreamClosed`, `ErrNilBuffer`,
  `ErrBufferNotOwned`, `ErrBufferAllocation`, `ErrPartOutOfRange`, `ErrPartNotImage`.
  Failures Aravis reports through a `GError` remain an `*AravisError`.
- `make test-glib-clean`, which fails on any GLib `CRITICAL **` or `WARNING **` in the test
  output, plus the CI step that runs it. `tests/README.md` had documented the check since
  P5, but nothing executed it.
- `CHANGELOG.md` and a provenance section in the README.

### Fixed

- **The part accessors did not range-check `partIndex`.** `GetPartData` and the seven
  other accessors passed the index straight to Aravis, which asserted internally, logged a
  GLib CRITICAL and returned 0 — a value the caller could not tell from a real width or
  component id. A negative index was worse: `partIndex` is an `int` and the C parameter a
  `guint`, so -1 arrived as 4294967295. All eight now check the index against
  `GetNumParts` and return `ErrPartOutOfRange`, or `ErrNilBuffer` for a buffer holding no
  `ArvBuffer`.
- **The geometry accessors ignored the image-part precondition.** `GetPartWidth`,
  `GetPartHeight`, `GetPartX`, `GetPartY` and `GetPartPixelFormat` are guarded inside
  Aravis by `arv_buffer_part_is_image`, which is static in `arvbuffer.c` and absent from
  the public header, so calling them on a part that carries no geometry produced
  `assertion 'arv_buffer_part_is_image (buffer, part_id)' failed` and a 0. Its condition —
  a successfully acquired buffer, an image, extended-chunk or multipart payload, and one
  of nine part data types — is now reproduced in Go as an allow-list, so an unrecognised
  future data type is rejected with `ErrPartNotImage` rather than reaching Aravis.
- **A pop timeout was indistinguishable from a real failure.** `TimeoutPopBuffer` reported
  every empty result as a freshly allocated `errors.New`, which no `errors.Is` could match,
  so a dropped frame and a broken stream looked the same. Each pop now says what its empty
  result means: `TimeoutPopBuffer` returns `ErrTimeout`, `PopBuffer` — which blocks until a
  buffer exists — returns `ErrNoBuffer`, and `TryPopBuffer` reports an empty output queue as
  a nil buffer with a **nil error**, since polling an empty queue is the expected outcome
  rather than a failure.
- **A negative `TimeoutPopBuffer` duration blocked forever.** Aravis takes an unsigned
  microsecond count, so a negative `time.Duration` converted to an enormous one. It now
  returns `ErrNegativeTimeout`; clamping to zero was rejected because it would make a caller
  bug look exactly like a dropped frame. The nanosecond-to-microsecond conversion also
  rounds up instead of truncating, so a sub-microsecond timeout still waits — truncation is
  what turned the historical `TimeoutPopBuffer(1000)` into a 1 µs timeout.
- **The pops handed a NULL or dangling `ArvStream` to Aravis.** All three now check the
  stream first and return `ErrNilStream` or `ErrStreamClosed`, where previously the zero
  value tripped an `ARV_IS_STREAM` assertion (a GLib CRITICAL followed by an empty result
  indistinguishable from "no frame") and a closed stream reached Aravis with a dangling
  pointer that nothing catches.
- **`errno` was being reported as an error.** Many wrappers used cgo's two-result call
  form (`v, err := C.arv_...`), whose second value is `errno`. `errno` is a syscall side
  effect, not a failure report: any syscall the C function makes internally and that
  fails benignly — a `recvfrom` returning `EAGAIN` on a GigE Vision socket, a `stat` of a
  file that is not there — left the wrapper reporting a failure that never happened.
  Only a `GError` decides failure now. Sixteen accessors
  wrap C functions with no `GError` out-parameter at all — the `interface.go` id and
  count accessors, eight `Buffer` accessors and the three `Stream` pop methods — and
  their error return became always nil. It did not stay that way: the pops report real
  sentinels and the part accessors range-check (both under **Breaking**), and the
  remaining eleven have since dropped the error return altogether. The seven `*Fast` getters no
  longer leak `errno` on their success path. Worst case was `NewBuffer`, where
  `if err != nil || buffer == nil` both reported a failure that had not happened *and*
  dropped the successfully allocated `ArvBuffer` on the floor, leaking it; it now keys
  off the NULL check alone and returns a real error when Aravis hands back NULL, which
  it previously reported as success.
- **The generic `Device` feature getters swallowed every GenICam error.**
  `GetStringFeatureValue`, `GetIntegerFeatureValue` and `GetFloatFeatureValue` passed
  `nil` for the `GError` out-parameter, so a missing feature or a wrong-typed read
  returned the zero value with a nil error. They now bind a real `GError` exactly as
  the setters do (see *Device setters now return `error`* under **Breaking** — this is
  the read-side counterpart), and report an `*AravisError` carrying, for example,
  `DEVICE_ERROR_FEATURE_NOT_FOUND`. Code that treated a nil error as "the feature
  exists" will start seeing the failures it was missing.
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
- **`Device.ReadMemory(address, 0)` panicked.** It allocated a zero-length slice and then
  took the address of its first element. A zero-size read is now rejected with an error,
  matching how `WriteMemory` rejects an empty write.
- **`TakeControl`/`LeaveControl` cast unconditionally through `ARV_GV_DEVICE()`.** cgo
  compiles with `-O2`, which makes GLib compile its cast checks out, so calling either on
  a non-GigE device — Aravis's own Fake backend, or any USB3 Vision camera — did not even
  produce a CRITICAL: it dereferenced the wrong struct and segfaulted the process. Both
  helpers now check `ARV_IS_GV_DEVICE` and report `ARV_DEVICE_ERROR_WRONG_FEATURE`.
- **`Camera.GetDevice` and `Camera.IsGVDevice` always returned a nil error,** and
  `GetDevice` never checked `arv_camera_get_device` for NULL, so a caller could receive a
  `Device` wrapping nothing with no indication anything had failed. Both now return a real
  error, and every `Device` method in `device.go` guards its receiver against a nil *and* a
  closed device instead of passing NULL — or, after `Close`, a dangling pointer — to
  Aravis, which asserted and logged a CRITICAL. The twelve `*Fast` accessors in
  `performance.go` are guarded the same way — the six on `Device` reuse that file's
  `check`, the six on `Camera` use `Camera.IsClosed`.
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
