# go-aravis Test Suite

This directory holds the external test package for the go-aravis library. It
runs against Aravis's built-in **Fake** backend, so the whole suite works
without a camera attached — and, more importantly, without pretending to.

## The Fake backend

Aravis ships a software interface named `Fake` that produces a real
`ArvDevice`, a real `ArvStream` and real, filled buffers. It is not a mock: the
calls go through the same C layer a physical camera would, which is what makes
it useful for a binding whose bugs live in that layer — pointer widths,
`GError` versus `errno`, ownership and unref counts, NULL sentinels.

`fake_test.go` owns the selection. Its `TestMain` enables `Fake` and disables
the GigE and USB3 interfaces before any test runs, which makes the suite:

- **deterministic** — exactly one device, `Fake_1`, with a fixed identity
  (vendor `Aravis`, model `Fake`, serial `1`), a 2048×2048 sensor, a 512×512
  region, 25 FPS and a 262144-byte Mono8 payload. Those are the expected values
  the tests assert against.
- **hermetic** — a camera on your desk cannot change what the suite asserts.
- **fast** — skipping the GigE/USB discovery scan takes the package from ~37 s
  to ~6 s.

Set `ARAVIS_TEST_HARDWARE=1` to leave the real interfaces enabled and run
against a physical camera instead; `make test-integration` does this.

### No silent skips

The suite **fails** if the Fake backend is unavailable rather than skipping.
Fake is compiled into every libaravis 0.8 build, so its absence is a broken
environment, not "no hardware", and a green run full of skips is exactly the
false signal this suite used to give. CI enforces it: any skip outside the
three documented below fails the build.

The three allowed skips are all properties of the Fake backend, not of this
binding:

| Skip | Why |
|---|---|
| `TestMultipleDevices` | Fake produces exactly one device and offers no way to ask for a second |
| `TestNewCameraFirstAvailable`, `TestOpenDeviceFirstAvailable` | Fake does not implement the first-device (NULL id) lookup |
| `TestFastAccessorsMatchStandard/{ExposureTime,Gain}` | the `*Fast` accessors address the fixed `ExposureTime`/`Gain` GenICam nodes, and Fake exposes only `ExposureTimeAbs` |

### Order independence

Any single test can be run on its own and asserts exactly what it asserts in a
full-package run:

```bash
go test ./tests/ -run TestFakeCameraGeometry -v
```

This did not used to hold. Enabling Fake is global process state, and it used
to be done by whichever test ran first, so the tests gated on
`GetNumDevices() == 0` silently ran against `Fake_1` in a full run and skipped
in isolation.

## Test files

| File | Covers |
|---|---|
| `fake_test.go` | `TestMain`, `requireFakeCamera`, `seededBuffer` — the backend every other file builds on |
| `interface_test.go` | device and interface discovery, enable/disable, out-of-range ids |
| `camera_test.go` | camera identity, geometry, parameter round-trips, `*Fast` accessors, stream creation |
| `buffer_test.go` | the fresh-buffer contract and the filled-buffer accessors, including multipart |
| `buffer_data_test.go` | `GetDataInto` clamping, overrun, empty dest, and its zero-allocation guarantee |
| `lifecycle_test.go` | `Close` idempotence across copies, owned vs borrowed devices, control-lost handlers under `-race` |
| `integration_test.go` | the full acquisition workflow and sustained streaming |
| `performance_test.go` | benchmarks |
| `bayer_test.go` | the debayering edge cases (pure Go, no backend needed) |

Pure-Go units — the error mapping, `toBool`, `closeFlag`, the C-string cache —
are tested in the **root package** instead (`internal_test.go`,
`lifecycle_test.go`, `performance_cache_test.go`), because they need access to
unexported identifiers. Go forbids `import "C"` in `_test.go` files, which is
why those files carefully avoid naming any C type.

## Running

```bash
make test-unit         # the whole suite, -race, no hardware
make test-short        # skips the acquisition-heavy tests
make test-coverage     # -race + coverage of the library (not just this package)
make benchmark         # all benchmarks; BENCHTIME=10x for a quick pass
make test-integration  # ARAVIS_TEST_HARDWARE=1; needs a real camera
```

Single tests and benchmarks:

```bash
go test -v ./tests/ -run TestBufferAccessorsAgreeOnSeededData
go test -bench=BenchmarkBufferDataAccess -benchmem -run '^$' ./tests/
```

## Benchmarks

Benchmarks run on Fake-**seeded** buffers, so their bodies actually execute and
their errors are checked. They previously skipped without a camera and, worse,
measured a fresh `NewBuffer` whose received size is zero — timing the
early-return path rather than any data access.

Absolute numbers depend on the machine; only same-machine comparisons mean
anything, and none are published (see the note at the top of
`../PERFORMANCE.md`). The one quantitative claim the project makes — that
`GetDataInto` copies without allocating — is asserted by
`TestGetDataIntoZeroAllocations`, not by a benchmark figure.

## Hardware testing

With `ARAVIS_TEST_HARDWARE=1` the real interfaces stay enabled and the suite
uses whatever it finds.

### GigE Vision

```bash
sudo ip link set <interface> mtu 9000   # jumbo frames
sudo ufw allow 3956/udp                 # discovery
sudo ufw allow 3956/tcp                 # control
```

### USB3 Vision

```bash
ls -la /dev/bus/usb/
sudo usermod -a -G plugdev $USER        # restart your session afterwards
```

## Troubleshooting

**"cannot set up the Aravis Fake backend"** — the libaravis build in use has no
`Fake` interface, which is unusual. Check `arv-tool-0.8` and the package
version.

**CGO compilation errors** — verify the development headers:
`pkg-config --exists aravis-0.8`. The package is `libaravis-dev` on Ubuntu
24.04 and later, `libaravis-0.8-dev` before that; both ship Aravis 0.8.

**GLib `CRITICAL` messages** — these mean a test is calling an Aravis function
outside its preconditions, and they are treated as a defect in the test, not as
noise. The suite currently produces none:

```bash
go test ./... 2>&1 | grep -c CRITICAL   # must be 0
```

## Contributing

1. Assert something that can fail. A test that only calls `t.Logf` inflates
   coverage without providing signal, and removing that class of test is what
   P5 in `../PLAN.md` was about.
2. Use `requireFakeCamera` and `seededBuffer` rather than re-deriving the
   backend setup.
3. Do not call `aravis.Shutdown()` from a test — `TestMain` owns it, and
   calling it mid-suite tears down the interface list the remaining tests
   depend on.
4. If something genuinely cannot be asserted, skip with a message that says
   *why*, and add it to the documented-skip list above and to the CI guard.
