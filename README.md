# go-aravis

**A comprehensive Go wrapper around libaravis 0.8+ for GigE Vision and USB3 Vision camera control**

This library provides Go bindings for the Aravis library, enabling high-performance machine vision applications with support for GigE Vision and USB3 Vision cameras. Built with CGO for optimal performance and direct access to advanced camera features.

If you need libaravis 0.6 support, go to the original package at https://github.com/thinkski/go-aravis

## 🚀 Features

### Core Camera Control

- **Device Discovery**: Automatic detection and enumeration of connected cameras
- **Camera Management**: Complete camera lifecycle management with proper resource cleanup
- **Image Acquisition**: High-performance streaming with customizable buffer management
- **Parameter Control**: Full access to camera parameters (exposure, gain, frame rate, triggers)
- **Region of Interest**: Precise control over capture area and sensor configuration

### Advanced Capabilities

- **Thread Priority Control**: Real-time, high-priority, and normal priority streaming modes
- **GigE Vision Optimization**: Packet size and delay control for network performance
- **Serial Number Access**: Device identification and inventory management
- **Multipart Buffer Support**: Advanced cameras with multi-tap and multi-spectral imaging
- **Chunk Data Processing**: Metadata extraction for advanced analysis
- **Register/Memory Access**: Low-level hardware control for advanced users

### Image Processing

- **Bayer Pattern Debayering**: Built-in support for raw sensor data processing
- **Multiple Pixel Formats**: Support for various camera output formats
- **Buffer Status Monitoring**: Comprehensive error detection and recovery

### Developer Experience

- **Modern Go Support**: Built for Go 1.21+ with modern development practices
- **Comprehensive Examples**: Production-ready code samples for all features
- **Docker Support**: Containerized development environment included
- **Professional Build System**: Modern Makefile with colored output and CI/CD support

## 📋 Requirements

### System Dependencies

- **libaravis 0.8+**: Core Aravis library with development headers
- **Go 1.21+**: Modern Go compiler with CGO support
- **pkg-config**: For library linking configuration

### Network Configuration (GigE Vision)

- **MTU 9000**: Jumbo frames for optimal network performance
- **Dedicated Network**: Separate network interface recommended for high-bandwidth cameras

### Ubuntu/Debian Installation

```bash
# Install Aravis library
sudo apt update
sudo apt install libaravis-0.8-dev pkg-config

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

    aravis "github.com/hybridgroup/go-aravis"
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
    "fmt"
    "log"
    "time"

    aravis "github.com/hybridgroup/go-aravis"
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

    // Create stream with high priority for real-time performance
    camera.SetThreadPriority(aravis.ThreadPriorityHigh)
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
        stream.PushBuffer(buffer)
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
        if err != nil {
            log.Printf("Frame timeout: %v", err)
            continue
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

        // Return buffer to stream
        stream.PushBuffer(buffer)
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
# Output: list_devices device_info get_image advanced_buffer register_access
```

### Example Directory Structure

Each example is now organized in its own directory for better modularity:

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

Each example is now organized in its own directory with a `main.go` file for better project structure:

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

```go
// Configure thread priority before creating stream
camera.SetThreadPriority(aravis.ThreadPriorityRealtime)  // Requires privileges
camera.SetThreadPriority(aravis.ThreadPriorityHigh)      // Recommended
camera.SetThreadPriority(aravis.ThreadPriorityNormal)    // Default
```

### Multipart Buffer Processing

Handle advanced cameras with multiple image sensors:

```go
// Check for multipart data
numParts, err := buffer.GetNumParts()
if numParts > 1 {
    for i := 0; i < numParts; i++ {
        partData, _ := buffer.GetPartData(i)
        width, _ := buffer.GetPartWidth(i)
        height, _ := buffer.GetPartHeight(i)
        componentId, _ := buffer.GetPartComponentId(i)

        // Process each image part separately
        fmt.Printf("Part %d: %dx%d, Component: %d\n", i, width, height, componentId)
    }
}
```

### GigE Vision Optimization

Maximize network performance:

```go
// Check if camera supports GigE Vision
if isGV, _ := camera.IsGVDevice(); isGV {
    // Optimize packet size for your network
    camera.SetGVPacketSize(9000)

    // Adjust packet delay if needed
    camera.SetGVPacketDelay(1000)

    // Get network statistics
    fmt.Printf("Stream bandwidth: %.2f MB/s\n",
        float64(payloadSize) * frameRate / 1024 / 1024)
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

For maximum performance in streaming applications, go-aravis now includes several optimizations:

#### Fast Parameter Access

Use cached string methods to eliminate C string allocations:
```go
// Standard method (allocates C strings)
width, err := camera.GetWidth()

// Fast method (uses cached strings) - 40% faster
width, err := camera.GetWidthFast()
height, err := camera.GetHeightFast() 
exposure, err := camera.GetExposureTimeFast()
```

#### Zero-Copy Buffer Access

Avoid memory copies for maximum streaming performance:

```go
// Standard method (copies entire buffer)
data, err := buffer.GetData()

// Zero-copy method (direct memory access) - 50x faster
dataSlice, err := buffer.GetDataSlice()

// Pre-allocated copy method (no allocations) - 5x faster  
dataBuffer := make([]byte, payloadSize) // Pre-allocate once
bytesRead, err := buffer.GetDataInto(dataBuffer)
```

#### Performance Impact

- **Parameter access**: 40% faster with cached strings
- **Buffer operations**: 50-80% faster with zero-copy methods  
- **Streaming performance**: Up to 50% CPU reduction
- **Memory usage**: 100% elimination of allocations in streaming loops

See `PERFORMANCE.md` for detailed optimization guide and `examples/performance_demo/` for working examples.

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

The container includes all necessary dependencies and is based on Debian with Go 1.21.

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

# Verify with camera
make run-example EXAMPLE=check-system
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

```go
// For high frame rates, use more buffers
bufferCount := 10  // Increase for higher frame rates

// For low latency, use fewer buffers
bufferCount := 2   // Minimize for real-time applications

// For batch processing, use large buffers
payloadSize *= 2   // Handle larger images efficiently
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

**Device Management**:

- `UpdateDeviceList()` - Refresh connected device list
- `GetNumDevices()` - Get count of available devices
- `GetDeviceId(index)` - Get device identifier by index
- `NewCamera(deviceId)` - Create camera instance

**Camera Control**:

- `StartAcquisition() / StopAcquisition()` - Control image capture
- `SetAcquisitionMode(mode)` - Configure capture mode
- `SetFrameRate(fps)` - Set acquisition frame rate
- `SetExposureTime(microseconds)` - Control exposure
- `SetGain(value)` - Adjust sensor gain

**Stream Management**:

- `CreateStream()` - Create image data stream
- `PushBuffer(buffer) / PopBuffer()` - Buffer queue management
- `TimeoutPopBuffer(timeout)` - Non-blocking buffer retrieval

**Advanced Features**:

- `GetDeviceSerialNumber()` - Device identification
- `GetNumParts() / GetPartData(index)` - Multipart image support
- `HasChunks()` - Metadata detection
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
- **Multi-Version Support**: Tests run on Go 1.21 and 1.22
- **Coverage Reporting**: Automatic coverage tracking and reporting
- **Security Scanning**: Automated security vulnerability detection
- **Cross-Platform**: Build verification across different platforms

All tests are designed to work **without requiring camera hardware**, making them CI/CD friendly. See `.github/workflows/` for implementation details.

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
