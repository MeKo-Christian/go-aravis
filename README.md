# go-aravis

**A comprehensive Go wrapper around libaravis 0.8.25+ for GigE Vision and USB3 Vision camera control**

This library provides Go bindings for the Aravis library, enabling high-performance machine vision applications with support for GigE Vision and USB3 Vision cameras. Built with CGO for optimal performance and direct access to advanced camera features.

```bash
go get github.com/MeKo-Christian/go-aravis
```

If you need libaravis 0.6 support, go to the original package at https://github.com/thinkski/go-aravis

> **This is a fork.** See [Provenance](#-provenance) for how it relates to the upstream
> projects, and [CHANGELOG.md](CHANGELOG.md) for what it changed.

## 🚀 Features

### Core Camera Control

- **Device Discovery**: Detection and enumeration of connected cameras
- **Camera Management**: Camera lifecycle management with explicit resource cleanup
- **Image Acquisition**: Streaming with caller-managed buffer queues
- **Parameter Control**: Exposure, gain, frame rate and trigger configuration, plus
  generic GenICam feature access through `Device`
- **Region of Interest**: Control over capture area, binning and sensor configuration

### Advanced Capabilities

- **Thread Priority Control**: Real-time, high-priority, and normal priority streaming modes
- **GigE Vision Optimization**: Packet size and delay control for network performance
- **Serial Number Access**: Device identification and inventory management
- **Multipart Buffer Support**: Per-part data, geometry and component metadata for
  multi-tap and multi-spectral cameras
- **Chunk Data Detection**: `Buffer.HasChunks` and `Camera.GetChunkMode` report whether
  a frame carries chunk metadata. Decoding the chunks themselves is not yet wrapped.
- **Register/Memory Access**: Low-level hardware control for advanced users

### Image Processing

- **Bayer Pattern Debayering**: `BayerRG` exposes a raw RGGB frame as an `image.Image`
  using nearest-neighbor demosaicing. Other Bayer phases are not implemented.
- **Pixel Format Control**: Read and set the pixel format, and enumerate the formats a
  camera advertises, as numeric IDs, GenICam strings, or display names. The package
  exports no pixel-format constants — supply the raw `uint32` or string.
- **Buffer Status Reporting**: `Buffer.GetStatus` returns the acquisition status
  (success, timeout, missing packets, …) so callers can detect bad frames.

### Developer Experience

- **Modern Go Support**: Built for Go 1.23+ with modern development practices
- **Comprehensive Examples**: Production-ready code samples for all features
- **Docker Support**: Containerized development environment included
- **Professional Build System**: Modern Makefile with colored output and CI/CD support

## 📋 Requirements

### System Dependencies

- **libaravis 0.8.25+**: Core Aravis library with development headers. The floor is
  0.8.25 rather than 0.8 because `Buffer.FindComponent` wraps
  `arv_buffer_find_component`, added in that release; against an older 0.8 the package
  fails to compile.
- **Go 1.23+**: Matches the `go` directive in `go.mod`; CGO must be enabled
- **pkg-config**: For library linking configuration

### Network Configuration (GigE Vision)

- **MTU 9000**: Jumbo frames for optimal network performance
- **Dedicated Network**: Separate network interface recommended for high-bandwidth cameras

### Ubuntu/Debian Installation

```bash
# Install Aravis library.
# The dev package was renamed on Ubuntu 24.04: it is `libaravis-dev` there and
# `libaravis-0.8-dev` on older releases. Both still ship Aravis 0.8.
sudo apt update
sudo apt install libaravis-dev pkg-config

# Configure network interface for GigE cameras (replace enp2s0 with your interface)
sudo ip link set enp2s0 mtu 9000

# Verify installation
pkg-config --modversion aravis-0.8
```

### Other Distributions

- **Fedora/RHEL**: `dnf install aravis-devel`
- **Arch Linux**: `pacman -S aravis`
- **macOS**: `brew install aravis`

## 🏃 Quick Start

### Basic Device Enumeration

```go
package main

import (
    "fmt"
    "log"

    aravis "github.com/MeKo-Christian/go-aravis"
)

func main() {
    // Discover connected cameras
    aravis.UpdateDeviceList()

    // Get device count
    n, err := aravis.GetNumDevices()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d camera(s)\n", n)

    // List all devices
    for i := uint(0); i < n; i++ {
        deviceId, err := aravis.GetDeviceId(i)
        if err != nil {
            log.Printf("Error getting device %d: %v", i, err)
            continue
        }
        fmt.Printf("Device %d: %s\n", i, deviceId)
    }
}
```

### Complete Image Acquisition

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "time"

    aravis "github.com/MeKo-Christian/go-aravis"
)

func main() {
    // Initialize camera system
    aravis.UpdateDeviceList()
    n, err := aravis.GetNumDevices()
    if err != nil || n == 0 {
        log.Fatal("No cameras found")
    }

    // Connect to first camera
    deviceId, _ := aravis.GetDeviceId(0)
    camera, err := aravis.NewCamera(deviceId)
    if err != nil {
        log.Fatal(err)
    }
    defer camera.Close()

    // Get camera information
    vendor, _ := camera.GetVendorName()
    model, _ := camera.GetModelName()
    serial, _ := camera.GetDeviceSerialNumber()
    fmt.Printf("Connected to: %s %s (S/N: %s)\n", vendor, model, serial)

    // Configure acquisition
    camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_CONTINUOUS)
    camera.SetFrameRate(30.0)  // 30 FPS

    // Create stream with high priority for real-time performance.
    // ThreadPriority is a struct field, read by CreateStream — set it first.
    camera.ThreadPriority = aravis.ThreadPriorityHigh
    stream, err := camera.CreateStream()
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()

    // Prepare buffers
    payloadSize, _ := camera.GetPayloadSize()
    for i := 0; i < 5; i++ {  // Use 5 buffers for smooth streaming
        buffer, err := aravis.NewBuffer(payloadSize)
        if err != nil {
            log.Fatal(err)
        }
        if err := stream.PushBuffer(buffer); err != nil {
            buffer.Close()
            log.Fatal(err)
        }
    }

    // Start acquisition
    err = camera.StartAcquisition()
    if err != nil {
        log.Fatal(err)
    }
    defer camera.StopAcquisition()

    // Capture frames
    fmt.Println("Capturing frames... (Press Ctrl+C to stop)")
    frameCount := 0
    for frameCount < 100 {  // Capture 100 frames
        buffer, err := stream.TimeoutPopBuffer(time.Second)
        if errors.Is(err, aravis.ErrTimeout) {
            // No frame arrived in time. Not a failure — just try again.
            log.Printf("Frame timeout")
            continue
        }
        if err != nil {
            log.Fatal(err)
        }

        // Check frame quality
        status, _ := buffer.GetStatus()
        if status == aravis.BUFFER_STATUS_SUCCESS {
            data, _ := buffer.GetData()
            fmt.Printf("Frame %d: %d bytes\n", frameCount, len(data))
            frameCount++

            // Process your image data here
            // ...
        }

        // A popped buffer belongs to us: give it back, or it leaks. Pushing it
        // also keeps the queue full — skipping this would stall acquisition.
        // Use buffer.Close() instead when the frame is the last one wanted.
        if err := stream.PushBuffer(buffer); err != nil {
            log.Fatal(err)
        }
    }
}
```

## 🔧 Build System

This project includes a modern, professional build system with comprehensive development tools.

### Available Commands

```bash
# Build the library
make build

# Build all examples (outputs to bin/ directory)
make examples

# Run a specific example
make run-example EXAMPLE=list_devices

# Run tests
make test
make test-coverage

# Code quality
make fmt        # Format code
make lint       # Run linter
make tidy       # Clean dependencies

# Development tools
make install-tools    # Install development dependencies
make check-system     # Verify system requirements

# Docker development
make docker-build     # Build development container
make docker-run       # Run examples in container

# Comprehensive targets
make all        # Build everything
make ci         # CI pipeline (build, lint, test)
make clean      # Clean build artifacts

# Get help
make help       # Show all available commands
```

### Example Building

All examples are automatically built to the `bin/` directory:

```bash
make examples
ls bin/
# Output: advanced_buffer device_info get_image list_devices performance_demo register_access
```

### Example Directory Structure

Each example lives in its own directory:

```text
examples/
├── list_devices/main.go          # Basic device enumeration
├── device_info/main.go           # Comprehensive device information
├── advanced_buffer/main.go       # Multipart and chunk data analysis
├── register_access/main.go       # Low-level register/memory access
├── get_image/main.go             # HTTP image server
└── performance_demo/main.go      # High-performance streaming demo
```

You can run examples individually or build specific ones:

```bash
# Build and run a specific example
make run-example EXAMPLE=device_info

# Build just one example
go build -o bin/device_info ./examples/device_info/main.go

# List all available examples
make list-examples
```

## 📚 Comprehensive Examples

### 1. Device Information (`examples/device_info/main.go`)

Comprehensive device discovery and information gathering:

- Device enumeration and identification
- Camera specifications and capabilities
- Serial number extraction for inventory
- GigE Vision vs USB3 Vision detection

### 2. Advanced Buffer Processing (`examples/advanced_buffer/main.go`)

Demonstrates advanced imaging capabilities:

- Multipart buffer handling for multi-tap cameras
- Component identification and metadata extraction
- Chunk data detection and analysis
- Multi-spectral imaging support

### 3. HTTP Image Server (`examples/get_image/main.go`)

Production-ready web service:

- Real-time image streaming over HTTP
- JPEG encoding and web delivery
- Error handling and resource management
- Performance monitoring

### 4. Register Access (`examples/register_access/main.go`)

Low-level hardware control for advanced users:

- Direct register read/write operations
- GigE Vision bootstrap register access
- Memory dump utilities with hex formatting
- Safety guidelines and best practices

### 5. Basic Listing (`examples/list_devices/main.go`)

Simple device enumeration for testing and debugging

### 6. Performance Demo (`examples/performance_demo/main.go`)

Demonstrates high-performance optimizations for streaming applications:

- Fast parameter access with cached strings  
- Zero-copy buffer access methods
- Pre-allocated buffer techniques
- Performance measurement and benchmarking

## 🔬 Advanced Features

### Thread Priority Control

Optimize streaming performance for real-time applications:

`ThreadPriority` is a field on `Camera`, not a setter method. `CreateStream` reads it,
so assign it before creating the stream:

```go
camera.ThreadPriority = aravis.ThreadPriorityRealtime // Requires privileges
camera.ThreadPriority = aravis.ThreadPriorityHigh     // Recommended
camera.ThreadPriority = aravis.ThreadPriorityNormal   // Default

stream, err := camera.CreateStream()
```

### Multipart Buffer Processing

Handle advanced cameras with multiple image sensors:

```go
// Check for multipart data
numParts, err := buffer.GetNumParts()
if err != nil {
    return err
}
if numParts > 1 {
    for i := 0; i < numParts; i++ {
        partData, _ := buffer.GetPartData(i)
        width, _ := buffer.GetPartWidth(i)
        height, _ := buffer.GetPartHeight(i)
        componentId, _ := buffer.GetPartComponentId(i)

        // Process each image part separately
        fmt.Printf("Part %d: %dx%d, component %d, %d bytes\n",
            i, width, height, componentId, len(partData))
    }
}
```

### GigE Vision Optimization

Maximize network performance:

```go
// Check if camera supports GigE Vision
if isGV, _ := camera.IsGVDevice(); isGV {
    // Optimize packet size for your network (requires MTU 9000 end to end)
    camera.GVSetPacketSize(9000)

    // Adjust inter-packet delay if the host drops packets
    camera.GVSetPacketDelay(1000)

    // Estimate the stream bandwidth
    payloadSize, _ := camera.GetPayloadSize()
    frameRate, _ := camera.GetFrameRate()
    fmt.Printf("Stream bandwidth: %.2f MB/s\n",
        float64(payloadSize)*frameRate/1024/1024)
}
```

### Register-Level Access (Advanced)

Direct hardware control for specialized applications:

```go
device, _ := camera.GetDevice()

// Read GigE Vision registers
version, _ := device.ReadRegister(aravis.GVBS_VERSION_REGISTER)
ipAddr, _ := device.ReadRegister(aravis.GVBS_DEVICE_IP_REGISTER)

// Memory access for firmware interaction
memData, _ := device.ReadMemory(0x0000, 64)

// ⚠️ CAUTION: Write operations can damage cameras
// err := device.WriteRegister(address, value)  // Use with extreme care
```

### High-Performance Optimizations

For streaming applications, the package offers a few lower-overhead paths alongside the
straightforward API.

#### Fast Parameter Access

The `*Fast` variants reuse interned C strings for the feature name instead of allocating
one per call. This only applies to the feature names in the package's internal table; it
changes allocation behavior, not the cost of the underlying device round-trip, which
dominates on real hardware.

```go
// Standard method: allocates a C string for the feature name on every call
width, err := camera.GetWidth()

// Fast method: reuses an interned feature-name string
width, err = camera.GetWidthFast()
height, err := camera.GetHeightFast()
exposure, err := camera.GetExposureTimeFast()
```

#### Zero-Copy Buffer Access

Three ways to get at pixel data, trading safety for copies:

```go
// Copies the frame into a freshly allocated slice
data, err := buffer.GetData()

// Aliases the C buffer with no copy at all. The returned slice is only valid
// until the buffer is handed back with stream.PushBuffer or released with
// buffer.Close().
dataSlice, err := buffer.GetDataSlice()

// Copies into a caller-owned slice, allocating nothing. The copy survives
// PushBuffer, but reusing one destination means each frame overwrites the last.
payloadSize, _ := camera.GetPayloadSize()
dataBuffer := make([]byte, payloadSize) // Pre-allocate once, reuse every frame
bytesRead, err := buffer.GetDataInto(dataBuffer)
```

#### What is actually measured

`GetDataInto` performs zero allocations. That is the one claim here backed by a test:
`TestGetDataIntoZeroAllocations` in `tests/buffer_data_test.go` asserts it with
`testing.AllocsPerRun`, and it runs in CI against Aravis's Fake camera backend.

No end-to-end throughput or CPU figures are published for this library. The benchmarks in
`tests/` skip without camera hardware, so any such number would not be reproducible from
this repository. Measure on your own camera and network before sizing a system.

See `PERFORMANCE.md` for the optimization guide and `examples/performance_demo/` for
working examples.

## 🐳 Docker Development

For consistent development environments across teams:

```bash
# Build the development container
make docker-build

# Run examples in container
make docker-run

# Interactive development
docker run -it --rm go-aravis:latest bash
```

The container is based on `golang:1.23-bookworm`, installs `libaravis-dev`, and builds
the `list_devices` example to `/usr/local/bin/listdevices`, which is what `make docker-run`
executes.

## 🔧 Troubleshooting

### Network Configuration (GigE Vision)

**MTU Configuration**: GigE Vision cameras require jumbo frames for optimal performance:

```bash
# Check current MTU
ip link show enp2s0

# Set MTU to 9000 (adjust interface name)
sudo ip link set enp2s0 mtu 9000

# Make permanent (Ubuntu/Debian)
echo 'enp2s0 mtu 9000' | sudo tee -a /etc/network/interfaces

# Verify the toolchain and Aravis installation
make check-system
```

**Firewall Configuration**: Ensure GigE Vision ports are open:

```bash
# Ubuntu/Debian
sudo ufw allow 3956/udp    # GigE Vision Discovery
sudo ufw allow 3956/tcp    # GigE Vision Control

# Check network connectivity
ping <camera-ip>
```

### Performance Optimization

**Buffer Management**: Optimize for your use case:

The number of buffers you push before starting acquisition trades latency against
tolerance for scheduling jitter:

```go
payloadSize, _ := camera.GetPayloadSize()

bufferCount := 10 // High frame rates: more buffers absorb scheduling jitter
// bufferCount := 2 // Low latency: fewer buffers keep frames fresh

for i := 0; i < bufferCount; i++ {
    buffer, err := aravis.NewBuffer(payloadSize)
    if err != nil {
        return err
    }
    if err := stream.PushBuffer(buffer); err != nil {
        buffer.Close()
        return err
    }
}
```

**Thread Priorities**: Requires system configuration:

```bash
# Allow real-time priorities (add to /etc/security/limits.conf)
echo "@realtime - rtprio 99" | sudo tee -a /etc/security/limits.conf
echo "@realtime - memlock unlimited" | sudo tee -a /etc/security/limits.conf

# Add your user to realtime group
sudo groupadd realtime
sudo usermod -a -G realtime $USER
```

### Common Issues

**"No cameras found"**:

1. Check USB3/GigE connections
2. Verify driver installation
3. Check permissions (`ls -la /dev/bus/usb/`)
4. Run device detection: `arv-tool-0.8 list`

**"Register read/write failed"**:

1. Ensure camera supports register access
2. Check if you have control permissions
3. Use feature-based access when possible
4. Verify register addresses in camera documentation

**"Buffer timeout"**:

1. Increase timeout duration
2. Check network MTU settings
3. Verify camera trigger configuration
4. Monitor system CPU/memory usage

### Development Tools

**System Requirements Check**:

```bash
make check-system
```

**Install Development Tools**:

```bash
make install-tools  # Installs golangci-lint, treefmt, etc.
```

**Continuous Integration**:

```bash
make ci  # Full CI pipeline: build, lint, test
```

## 🧪 Testing

This project includes a comprehensive test suite in the `tests/` directory that works both with and without connected cameras.

### Test Categories

**Unit Tests (No Camera Required)**:

```bash
make test-unit    # Tests that work without cameras
```

- Library initialization and basic operations
- Error handling with invalid inputs
- Buffer creation and data access validation
- Constants and API structure verification

**Integration Tests (Camera Required)**:

```bash
make test-integration    # Requires connected cameras
```

- Full camera workflow testing
- Real image acquisition and streaming
- Performance measurement with actual hardware
- Multiple camera operations

**Performance Benchmarks**:
```bash
make benchmark    # Comprehensive performance testing
make benchmark-performance    # Performance optimization benchmarks only
```

### Test Execution Options

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run only short tests (skips long-running integration tests)
make test-short

# Run specific test patterns
go test -v ./tests/ -run TestDeviceDiscovery
go test -v ./tests/ -run TestCamera
```

### Test Environment

The test suite is designed to gracefully handle different environments:
- **No cameras**: Unit tests validate library structure and error handling
- **Single camera**: Full integration testing with real hardware
- **Multiple cameras**: Advanced multi-device testing and resource management

Tests automatically skip camera-dependent operations when no cameras are available, making them safe to run in CI/CD environments.

For detailed testing information, see `tests/README.md`.

## 📖 API Documentation

### Core Functions

The authoritative reference is the godoc for the package itself; this is an orientation
map. Run `go doc github.com/MeKo-Christian/go-aravis` for the full surface.

**Device Management**:

- `UpdateDeviceList()` - Refresh connected device list; call before enumerating
- `GetNumDevices()` - Get count of available devices
- `GetDeviceId(index)` - Get device identifier by index
- `GetNumInterface() / GetInterfaceId(index)` - Enumerate transport interfaces
- `EnableInterface(id) / DisableInterface(id)` - Restrict which backends are used
- `NewCamera(deviceId)` - Create camera instance; an empty id selects the first device
- `OpenDevice(id)` - Open a device without a camera. The caller owns the result and
  must `Close` it, unlike the borrowed device from `Camera.GetDevice`.
- `Shutdown()` - Release Aravis's global state

**Camera Control**:

- `StartAcquisition() / StopAcquisition() / AbortAcquisition()` - Control image capture
- `SetAcquisitionMode(mode)` - Configure capture mode
- `SetFrameRate(fps)` - Set acquisition frame rate
- `SetExposureTime(t)` - Control exposure, in the unit the camera's `ExposureTime`
  feature uses (microseconds on essentially all devices; the package does not convert)
- `SetGain(value)` - Adjust sensor gain
- `SetRegion() / GetRegion() / SetBinning() / GetBinning()` - Sensor geometry
- `SetPixelFormat() / GetAvailablePixelFormats()` - Pixel format selection
- `Close() / IsClosed()` - Release the camera; idempotent
- `SetControlLostHandler(fn)` - Register a per-camera control-lost callback

**Stream Management**:

- `CreateStream()` - Create image data stream, using `Camera.ThreadPriority`
- `PushBuffer(buffer) error` - Hand a buffer to the acquisition queue, transferring
  ownership to the stream. Rejects a buffer the caller no longer owns
  (`ErrBufferNotOwned`), a nil buffer and a nil or closed stream.
- `PopBuffer()` - Blocks indefinitely until a frame is available; `ErrNoBuffer` if the
  stream itself is rejected
- `TimeoutPopBuffer(timeout)` - Blocks until a frame arrives or the timeout expires,
  which is reported as `ErrTimeout`
- `TryPopBuffer()` - The non-blocking variant; an empty queue is a nil buffer with a
  **nil error**

**Buffer Ownership**:

- `NewBuffer(size)` - Allocate a buffer the caller owns
- `Close() / IsClosed()` - Release a buffer the caller owns; idempotent across copies.
  A buffer is released either by pushing it to a stream or by closing it — exactly one
  of the two.

**Advanced Features**:

- `GetDeviceSerialNumber()` - Device identification
- `GetNumParts() / GetPartData(index)` - Multipart image support
- `GetPartWidth/Height/X/Y(index)` - Per-part geometry
- `GetPartComponentId/DataType/PixelFormat(index)`, `FindComponent(id)` - Part metadata
- `HasChunks()` - Reports whether the frame carries chunk metadata
- `ReadRegister() / WriteRegister()` - Hardware-level access

### Constants and Enums

**Acquisition Modes**:

- `ACQUISITION_MODE_CONTINUOUS` - Continuous streaming
- `ACQUISITION_MODE_SINGLE_FRAME` - Single frame capture

**Buffer Status**:

- `BUFFER_STATUS_SUCCESS` - Frame acquired successfully
- `BUFFER_STATUS_TIMEOUT` - Acquisition timeout
- `BUFFER_STATUS_MISSING_PACKETS` - Network packet loss

**Thread Priorities**:

- `ThreadPriorityNormal` - Standard priority
- `ThreadPriorityHigh` - High priority (recommended)
- `ThreadPriorityRealtime` - Real-time priority (requires privileges)

## 🤝 Contributing

We welcome contributions! Please see our development workflow:

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Install development tools**: `make install-tools`
4. **Make your changes** with proper testing
5. **Run quality checks**: `make ci`
6. **Commit with clear messages**: `git commit -am 'Add amazing feature'`
7. **Push to your fork**: `git push origin feature/amazing-feature`
8. **Create a Pull Request**

### Development Guidelines

- **Follow Go best practices** and existing code style
- **Add tests** for new functionality
- **Update documentation** for API changes
- **Test with real cameras** when possible
- **Consider backward compatibility**

### Code Quality

This project maintains high code quality standards:

- **Formatting**: `make fmt` (treefmt + gofmt)
- **Linting**: `make lint` (golangci-lint)
- **Testing**: `make test` with coverage reporting
- **Dependencies**: `make tidy` for clean module management

### Continuous Integration

The project includes comprehensive CI/CD workflows:

- **Automated Testing**: Every commit runs unit tests, integration tests, and benchmarks
- **Single Go Version**: All jobs use the Go version in `go.mod` (1.23), pinned via the
  `GO_VERSION` variable in the workflow
- **Coverage Reporting**: Automatic coverage tracking and reporting
- **Security Scanning**: `gosec` runs on every commit and uploads SARIF results
- **Build Verification**: A `linux/amd64` build job. There is no cross-platform or arm64
  matrix — a real arm64 build needs a cross-compiled Aravis, not just `GOARCH`.

All tests are designed to work **without requiring camera hardware**, making them CI/CD friendly. See `.github/workflows/` for implementation details.

## 🧬 Provenance

Three parties appear in this repository's history, which is worth spelling out because the
LICENSE, the old module path, and the repository owner are all different names:

| Role | Who |
| --- | --- |
| Original author, copyright holder | Chris Hiszpanski ([thinkski/go-aravis](https://github.com/thinkski/go-aravis)), for libaravis 0.6 |
| Upstream fork, Aravis 0.8 port | The Hybrid Group ([hybridgroup/go-aravis](https://github.com/hybridgroup/go-aravis)), 2019–2022 |
| This fork | [MeKo-Christian/go-aravis](https://github.com/MeKo-Christian/go-aravis) |

Both prior copyrights are retained in [LICENSE](LICENSE); this fork adds no new copyright
claim and stays BSD 3-Clause.

The module path is `github.com/MeKo-Christian/go-aravis`. It was previously
`github.com/hybridgroup/go-aravis`, which did not match the repository and so could not be
resolved by `go get`. If you are migrating from the upstream module, update your imports:

```bash
go mod edit -replace github.com/hybridgroup/go-aravis=github.com/MeKo-Christian/go-aravis@latest
```

or simply change the import path — the package name (`aravis`) is unchanged.

See [CHANGELOG.md](CHANGELOG.md) for what this fork changed relative to upstream, including
the breaking changes.

## 📄 License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Original go-aravis**: Based on the excellent work by [thinkski](https://github.com/thinkski/go-aravis)
- **Aravis Library**: Built on the powerful [Aravis](https://github.com/AravisProject/aravis) library
- **GigE Vision**: Implementing the [AIA GigE Vision](https://www.visiononline.org/) standard
- **USB3 Vision**: Supporting the [AIA USB3 Vision](https://www.visiononline.org/) standard

## 🔗 Related Projects

- **Original go-aravis**: https://github.com/thinkski/go-aravis (for libaravis 0.6)
- **Aravis Library**: https://github.com/AravisProject/aravis
- **GoCV**: https://github.com/hybridgroup/gocv (Computer vision processing)
- **Vision Standards**: https://www.visiononline.org/ (AIA standards)

---

**Built with ❤️ for the machine vision community**
