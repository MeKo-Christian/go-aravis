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

- [x] **Remove or implement the 17 empty `// TODO` stub methods.** All 11
      `camera.go` stubs are now implemented against the real Aravis API:
      `SetBinning`, `SetPixelFormat`, `GetPixelFormat`, `GetPixelFormatAsString`,
      `SetPixelFormatFromString`, `GetAvailablePixelFormats`,
      `GetAvailablePixelFormatsAsDisplayNames`,
      `GetAvailablePixelFormatsAsStrings`, `GetExposureTimeBounds`,
      `GetExposureTimeAuto`, `SetGainAuto`. The three
      `arv_camera_dup_available_pixel_formats*` wrappers `g_free` the array they
      are handed (the `dup` prefix transfers ownership) while leaving the strings
      it points at to Aravis. `GetBinning` kept its signature but its locals were
      renamed `dx, dy` — it returns the current binning, not bounds, and the old
      `minBin, maxBin` names said otherwise. In `interface.go`, `OpenDevice`
      became `OpenDevice(id string) (Device, error)`; the five `Interface*` stubs
      were deleted, since they all need an `ArvInterface *` handle and this
      wrapper has no `Interface` type to hang one on.
- [x] **`SetControlLostHandler` uses a single package-global handler.** Handlers
      now live in a mutex-guarded `controlLost` registry keyed per camera, and
      the key travels through GLib as the signal's `user_data`, so each camera's
      callback finds its own. Two further bugs fell out of this: the signal was
      only connected inside `CreateStream` (so a handler set on a camera that
      never streamed could never fire) and it was reconnected on *every*
      `CreateStream` call, stacking duplicate handlers. Connecting now happens in
      `SetControlLostHandler` itself, exactly once, and `Close` disconnects.
      Passing nil removes the handler; calling it on a closed camera reports an
      error instead of silently succeeding. Covered by a `-race` concurrency test.
- [x] **Fix typo `GetNumInferface` → `GetNumInterface`** in `interface.go`.
      `GetNumInferface` remains as a `// Deprecated:` wrapper that forwards, so
      existing callers keep compiling; in-repo callers were moved over.
- [x] **`Close()` methods are unguarded.** `Camera.Close` and `Stream.Close` now
      return early on a nil pointer and unref exactly once. The first attempt
      stored a per-value guard, which PR review correctly rejected: `Camera`,
      `Stream` and `Device` are handed out **by value**, so a copy carries the
      same C pointer but its own guard, and closing two copies still unreffed
      twice. The lifecycle state now sits behind shared identity — a
      `*closeFlag` (`lifecycle.go`) whose `claim()` succeeds once, shared by
      every copy — so `Close` is idempotent per underlying object rather than
      per Go value. `Camera`'s control-lost registration moved into the same
      shared `cameraState` for the same reason: two copies would otherwise each
      see key zero and connect a separate signal handler. Each type also gained
      `IsClosed()`.
      No `runtime.SetFinalizer`: on a freely copied value type a finalizer would
      unref while a copy is still live, trading a leak for a crash.
      Implementing `OpenDevice` also forced the ownership question for `Device`,
      which had no `Close` at all — `OpenDevice` hands over a reference the caller
      owns (`transfer-ownership="full"`), while `Camera.GetDevice` only borrows
      the camera's. A non-nil `owned` flag marks the former; `Close` unrefs only
      then.
- [x] **Empty id/name passed to Aravis as `""` instead of NULL.** Raised in PR
      review for `OpenDevice`; `NewCamera` had the identical bug. Aravis
      documents NULL as the "first available device" sentinel (`nullable="1"` in
      the GIR), so `C.CString("")` asked for a device whose id is the empty
      string, which nothing matches. Both now pass a nil `*C.char`.
      Not covered by a test: Aravis's Fake backend does not implement the
      first-device lookup either — `arv_open_device(NULL)` reports "device not
      found" while the interface enumerates `Fake_1` — so
      `TestOpenDeviceFirstAvailable` skips rather than faking coverage.

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

- [x] **Module path vs repo mismatch.** Renamed the module from
      `github.com/hybridgroup/go-aravis` to `github.com/MeKo-Christian/go-aravis`, so
      the fork resolves by its own path. All 22 in-repo import sites, `go.mod` and the
      Makefile's `MODULE_NAME` were updated. Breaking for anyone on the old path; the
      README and CHANGELOG document the one-line `go mod edit -replace` migration.
- [x] **`SetThreadPriority(...)` does not exist.** All four uses replaced with
      assignment to the `Camera.ThreadPriority` field, and the surrounding prose now
      says explicitly that it is a field read by `CreateStream`, not a setter. Two
      further invented names turned up in the same sweep and were fixed:
      `SetGVPacketSize`/`SetGVPacketDelay` → `GVSetPacketSize`/`GVSetPacketDelay`.
- [x] **Remove/annotate vaporware feature claims.** The P1 work made the pixel-format
      claim true, so it stayed (noting the package exports no pixel-format constants).
      The rest were narrowed to what the code does: "Chunk Data Processing" → detection
      only, since only `HasChunks`/`GetChunkMode` exist and no chunk decoding is
      wrapped; "comprehensive error detection and recovery" → status reporting, there
      is no recovery logic; "Full access to camera parameters" → the parameters
      actually covered, plus generic GenICam access via `Device`; "Bayer Pattern
      Debayering" → RGGB only, nearest-neighbor only.
- [x] **Correct the false performance claims.** Every benchmark figure was deleted
      from both documents, with a note at the top of `PERFORMANCE.md` explaining why
      (nothing in this repository produced them, and the benchmarks that exist skip
      without hardware). The "error pooling reduces allocation overhead" claim was
      not merely unmeasured but false after P2 deleted the pool — error handling is now
      described as a correctness feature that costs one allocation per error. The one
      surviving quantitative claim is the zero-allocation `GetDataInto`, which P2 made
      true and `TestGetDataIntoZeroAllocations` asserts in CI.
- [x] **Fix Go version inconsistency.** `go.mod`, the Makefile, the Dockerfile and CI
      were already unified on 1.23 by P4; only the README still claimed 1.21/1.22, in
      four places (including a fictitious "tests run on Go 1.21 and 1.22" matrix). All
      now say 1.23. The apt package name was stale too — `libaravis-0.8-dev` was
      renamed `libaravis-dev` on Ubuntu 24.04, which is what broke CI in P4.
- [x] **Fix non-compiling README snippets.** Verified mechanically rather than by
      inspection: all 17 fenced Go blocks in `README.md` and `PERFORMANCE.md` are
      extracted, the fragments wrapped in a function against the real package, and
      compiled. All 17 build. The mislabeled "`TimeoutPopBuffer` — Non-blocking" is
      corrected, and `TryPopBuffer` — the genuinely non-blocking call, previously
      absent from both documents — is now listed alongside `PopBuffer` with the
      blocking behavior of each spelled out.
- [x] **Add package doc / godoc comments.** Added `doc.go` covering the acquisition
      call order, the value-type/shared-close-flag semantics and why there is no
      finalizer, the owned-vs-borrowed `Device` split, the pixel-data accessor tradeoffs
      and the lifetime rule that aliasing slices die at `PushBuffer`, error handling,
      concurrency, and GigE Vision setup. Every exported symbol is now documented:
      179 of 179, with none non-conforming, checked with a go/ast pass rather than by
      eye. Three floating section comments that godoc had been misattributing to a
      single following method were folded into per-symbol docs, and the dead
      commented-out policy methods in `device.go` were deleted.
- [x] **Clarify fork provenance/attribution.** Added a Provenance section to the README
      naming all three parties and their roles, and `CHANGELOG.md` recording what this
      fork changed across P0–P4, with breaking changes called out separately.

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

The finding that reframed this phase: Aravis ships a built-in **Fake** software
backend that produces a real `ArvDevice`, a real `ArvStream` and real, filled
buffers with no hardware. Two test files already used it — but enabling it is
global process state, so whichever test ran first decided what the rest of the
suite saw. `go test -run TestCameraWithRealDevice ./tests/` skipped with "No
cameras connected" while the same test in a full-package run passed against
`Fake_1`. So the ~20 tests gated on `GetNumDevices() == 0` were Fake tests in
disguise, and a test run alone asserted something different from the same test
run in company. Making the backend explicit is what made the rest of P5
possible.

- [x] **Add real unit tests for pure-Go logic** (no hardware needed). Done for
      `bayer.go` (`tests/bayer_test.go`: the edge-panic regression, the
      opaque-alpha fix, the CFA edge-phase reflection), for the error mapping and
      `toBool` (`internal_test.go`, repo root, `package aravis` — Go forbids
      `import "C"` in `_test.go` files, which is why the pure-Go
      `newAravisError` was split out of the cgo glue), for `closeFlag`
      (`lifecycle_test.go`) and for `getCachedCString` (`performance_cache_test.go`,
      which pins the bound the cache depends on: interned names are reused,
      arbitrary names get a temporary the caller frees, and the cache never grows
      after `init`). `GetDataInto` is covered by `tests/buffer_data_test.go` —
      clamp, overrun, empty/nil dest, and the zero-allocation assertion.
      The "still TODO: `getCachedCString`" note this item used to carry was
      stale; that test landed with P2.
- [x] **Replace log-only "tests" with real assertions or delete them.** Sixteen
      tests and helpers could not fail. `mock_test.go` was deleted outright: it
      contained no mock, called the real C layer, and five of its seven tests
      only logged. Its content was redistributed rather than dropped.
      Three things were deleted rather than converted, because no contract
      existed to assert: `TestConstants` (every constant is a compile-time alias
      of a C enum, so any assertion restates its own definition — and they are
      already compile-checked by the tests that use them); `TestShutdown`
      (`Shutdown()` returns nothing and has no observable Go-level contract, and
      the only property one could assert, that the library still works
      afterwards, was exactly the mid-suite teardown being removed); and the
      `NewBuffer(1 GB)` case (its own log accepted both outcomes, so it had no
      failing case, and it allocated a gigabyte in CI to reach it).
      The rest became assertions against the Fake backend: the exact identity and
      geometry — which doubles as the regression test for the P0 32/64-bit
      out-param bug — parameter round-trips, `*Fast`/standard equality, the
      fresh-buffer contract (status `CLEARED`, empty payload, one part, no
      chunks), byte-for-byte agreement between `GetData`, `GetDataSlice`,
      `GetDataInto` and `GetDataUnsafe` on a seeded payload, and — the largest
      gain — `EnableInterface`/`DisableInterface` actually changing what
      `UpdateDeviceList` finds, where the old test only checked that the calls
      returned.
      One test was actively harmful rather than merely useless: the old
      `TestBufferMultipart` called `GetPartWidth`/`GetPartHeight` on a fresh
      buffer, whose part is not an image, tripping
      `assertion 'arv_buffer_part_is_image' failed` twice and logging the
      resulting 0 as a result. Run against a seeded buffer the accessors are
      assertable. `go test ./... 2>&1 | grep -c CRITICAL` went from 2 to 0, and
      that check is now part of the verification routine.
      Three contracts were deliberately *not* asserted, because Aravis does not
      offer them: `GetDeviceId`/`GetInterfaceId` past the end return a nil error
      (that result is cgo's `errno`, see P6), so only the empty string is
      asserted; the Fake camera exposes no `ExposureTime`/`Gain` GenICam node, so
      the corresponding `*Fast` accessors are probed and skipped rather than
      pinning a backend quirk as this binding's contract; and
      `NewCamera("")`/`OpenDevice("")` still skip, for the reason recorded under
      P1.
- [ ] **Introduce a seam over the C calls** so a genuine fake can be injected.
      **Deferred, not resolved.** In the meantime Aravis's built-in Fake
      interface serves as the explicit test seam: `tests/fake_test.go`'s
      `TestMain` enables it and disables the GigE and USB3 interfaces, so
      discovery is deterministic (exactly one device), hermetic (a camera on the
      developer's machine cannot change an assertion) and fast (`./tests` went
      from ~37 s to ~6 s). It hard-fails rather than skipping when Fake is
      unreachable, since Fake ships in every libaravis 0.8 build and a green run
      full of skips is the false signal this phase exists to remove.
      `ARAVIS_TEST_HARDWARE=1` keeps the real interfaces enabled, and in that
      mode the acquisition tests bind to the first non-Fake device and fail when
      none is attached — otherwise `make test-integration` would report success
      on hardware coverage it never exercised (raised in PR review).
      What that does *not* give is error-path injection: there is no way to make
      `arv_camera_set_region` fail on demand. Weigh that against the cost — a Go
      interface over the ~180 cgo calls would have to be threaded through every
      wrapper type and constructor, a breaking change across ~110 methods — and
      against the fact that the bugs this repository actually had (pointer
      widths, `GError` versus `errno`, unref counts, NULL sentinels) live in the
      layer such a seam would replace, so a mock would have passed every one of
      them. Left open deliberately; `mock_test.go`, the file this item named, is
      gone either way.
- [x] **Add `-race` tests.** `SetControlLostHandler` is hammered by concurrent
      installs and removals across copies of a real Fake camera
      (`tests/lifecycle_test.go`), and `closeFlag.claim` by 64 concurrent closers
      (`lifecycle_test.go` in the root package). `getCachedCString` no longer has
      a race to test: the cache became write-once under P2, so its mutex is gone,
      and `CleanupPerformanceCache` no longer exists. `-race` is no longer
      optional — `make test-unit`, `make test-short` and `make test-coverage` all
      pass it.
- [x] **Fix `TimeoutPopBuffer(1000)` unit bug** in `buffer_test.go`. The literal
      was 1000 ns (1 µs), not the "1 second" the comment claimed, because the
      wrapper divides a `time.Duration` by 1000 to reach Aravis's microseconds —
      so the pop always timed out and the test returned early without ever
      reaching the data accessors. The surrounding `t.Logf("timeout expected")`
      is what kept it invisible. The call now goes through the shared
      `seededBuffer` helper with a `time.Duration`-typed constant, and every
      remaining call site passes a typed duration.
- [x] **Stop advertising unrunnable benchmarks.** All six skipped without
      hardware, so none had ever run in CI, and the buffer ones measured an
      unfilled `NewBuffer` — received size zero, i.e. the early return — so their
      numbers described nothing. That is why P3 had to delete every published
      figure. They now run on Fake-seeded buffers, report bytes and allocations,
      and `make benchmark` no longer ends in `|| echo`, so a broken benchmark
      fails the build. Two pairs were exact duplicates across `buffer_test.go`
      and `performance_test.go` and were merged. Every benchmark probes before
      timing: one whose every iteration fails otherwise reads as an exceptionally
      fast one, and the exposure/gain fast paths skip on Fake rather than timing
      a `feature not found` error.
      The numbers independently confirm the one quantitative claim
      `PERFORMANCE.md` still makes — `GetDataInto` at 0 allocs/op against
      `GetData` at 262144 B/op — but none are published: figures from a shared CI
      runner are not comparable across runs, which is the point that document
      already makes.

Also fixed along the way, since they were what made the suite's coverage claim
hollow: `make test-unit` carried a hand-maintained 19-alternative `-run`
allowlist that silently omitted every hardware-free test added after it was
written, and `make test-coverage` measured `[no statements]` — `tests` is an
*external* test package with no non-test code, so without `-coverpkg` the
profile instrumented nothing, and running only `./tests/` excluded the
root-package tests entirely. Coverage now reports 35.5% of the library. CI also
gained a guard that fails on any undocumented skip, since a skip is precisely
the silence this phase removed.

### P6 — Correctness bugs found while documenting (new, not yet fixed)

Writing godoc for every exported symbol meant reading every function body against the
Aravis call it wraps, which surfaced a fresh set of bugs. None were fixed in the P3
docs-only pass; the documentation describes current behavior, including where that
behavior is wrong. Ordered by severity.

- [ ] **`errno` is being reported as an error across the package.** Many methods use
      cgo's two-result form (`v, err := C.arv_...`), where the second value is `errno`.
      `errno` is not cleared by a successful call, so a stale or incidental value makes
      a successful call return a non-nil error. Only the `GError` should decide failure.
      Affects all 12 `*Fast` methods in `performance.go`, plus `GetData`, `GetDataSlice`,
      `GetDataUnsafe`, `GetDataInto`, `GetStatus`, `GetNumParts`, `GetPartData` and the
      three pop methods. Worst case is `NewBuffer` (`buffer.go`), whose
      `if err != nil || buffer == nil` reports failure *and* drops the successfully
      allocated `ArvBuffer` on the floor, leaking it — it should key off `buffer == nil`
      alone.
- [x] **`Device.ReadMemory(addr, 0)` panics.** It does `make([]byte, size)` then takes
      `&buffer[0]`, which is out of range for a zero size. `WriteMemory` has the
      equivalent guard; `ReadMemory` does not.
      **Fixed.** A zero size now returns an error rather than an empty slice, so the two
      memory calls agree: a zero-length transfer is a caller mistake, not a request worth
      forwarding. The doc comment, which described the panic as the contract, says so.
- [ ] **The generic `Device` getters swallow every GenICam error.**
      `GetStringFeatureValue`, `GetIntegerFeatureValue` and `GetFloatFeatureValue` pass
      `nil` for the `GError` out-param, so a failure returns the zero value with no
      error — and the error they *do* return is the `errno` above. This is the read-side
      counterpart of the P0 fix to the setters.
- [x] **`TakeControl`/`LeaveControl` cast unconditionally via `ARV_GV_DEVICE()`** with no
      `ARV_IS_GV_DEVICE` check, so calling either on a USB3 Vision device trips a GLib
      critical and passes a bad pointer.
      **Fixed.** It was worse than a critical: GLib compiles its cast checks out under
      `__OPTIMIZE__`, and cgo builds with `-O2`, so the macro was a plain pointer cast and
      the call segfaulted — which is exactly what the new test observed against Aravis's
      own Fake device before the fix. Both C helpers now check the type themselves and
      `g_set_error` with `ARV_DEVICE_ERROR_WRONG_FEATURE`, chosen because it is not in the
      `errorMessages` table, so the message "device is not a GigE Vision device" reaches
      the caller instead of being rewritten to `ARV_DEVICE_ERROR_NOT_FOUND`'s misleading
      "device not found".
- [x] **`Camera.GetDevice` and `Camera.IsGVDevice` always return a nil error**, and
      `GetDevice` never checks `arv_camera_get_device` for NULL, so a caller can receive
      a `Device` wrapping a nil pointer with no indication anything failed. No method on
      `Device` except `Close`/`IsClosed` nil-guards its receiver, so that NULL then
      reaches Aravis.
      **Fixed.** Both methods now guard the camera and check the result, returning the
      house `errors.New` messages the rest of the package uses — no sentinels, which are
      the separate error-contract item. Every `Device` method that dereferences the
      underlying pointer calls a shared `check()` first, so the fourteen that used to hand
      Aravis a NULL and collect an `ARV_IS_DEVICE` assertion now return an error. A
      positive control against Fake keeps the guards from passing by rejecting everything.
- [ ] **A timeout is indistinguishable from a real failure.** `TimeoutPopBuffer` reports
      both as a freshly allocated `errors.New("aravis returned a null pointer")`, which
      is non-comparable, so callers cannot use `errors.Is` to detect a dropped frame.
      A package-level sentinel would fix it. Separately, a negative `time.Duration`
      converts to a huge unsigned value, turning a nonsensical timeout into an
      effectively infinite block.
- [ ] **`BayerRG.At` uses the wrong CFA phase for an odd-origin rect.** `At` derives the
      phase from absolute `x&1`/`y&1` while `sample` indexes relative to `Rect.Min`, so
      for any rect with an odd `Min.X`/`Min.Y` every color is off by one Bayer site.
      `At` should use `(x-Rect.Min.X)&1` / `(y-Rect.Min.Y)&1`.
- [ ] **A `Buffer` in the caller's hands cannot be freed.** `Buffer` has no `Close`, and
      `Stream.Close` releases only the buffers still sitting in the stream's queues.
      Aravis gives ownership of a popped buffer to the caller
      (`arv_stream_pop_buffer` is `transfer-ownership="full"`), so *two* cases leak with
      no way to release them: a buffer from `NewBuffer` that is never pushed, and a
      popped buffer that is never pushed back. Raised in PR review, where it also turned
      out the P3 docs had this backwards — they claimed a popped buffer stayed
      stream-owned. The docs are corrected; the API gap stands.
- [ ] **Minor:** `GetPartData` and the part accessors do not range-check `partIndex`
      against `GetNumParts`; `SetGainAuto` has no `GetGainAuto` counterpart although
      `arv_camera_get_gain_auto` exists; `GetFrameRateBounds`/`GetGainBounds` cast
      `*float64` to `*C.double` through `unsafe.Pointer` where `GetExposureTimeBounds`
      correctly declares `C.double` locals.

### Suggested execution order

1. P0 correctness bugs (safety) — done
2. P4 build/CI (so changes are verifiable) — done
3. P5 pure-Go tests (lock in behavior) — done
4. P1/P2 API + perf machinery — done
5. P3 docs (describe the now-true reality) — done
6. P5 tests (make the suite mean something) — done, except the deferred
   C-call seam
7. P6 correctness bugs surfaced by the P3 doc pass — next. The suite that
   lands them is now one that can fail.
