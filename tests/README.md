# go-aravis Test Suite

This directory contains a comprehensive test suite for the go-aravis library, designed to test both with and without real cameras connected.

## Test Organization

### Test Files

- **`interface_test.go`** - Tests device discovery and interface enumeration
- **`camera_test.go`** - Tests camera creation, configuration, and operations
- **`buffer_test.go`** - Tests buffer creation, data access, and multipart support
- **`performance_test.go`** - Tests and benchmarks performance optimizations
- **`integration_test.go`** - Full workflow and streaming tests
- **`mock_test.go`** - Tests that work without real cameras (mocks/stubs)

### Test Categories

#### Unit Tests (No Camera Required)

These tests verify library functionality without requiring connected cameras:

- Basic library operations
- Error handling with invalid inputs
- Buffer creation and basic operations
- Constants and structure validation
- Boundary condition testing

#### Integration Tests (Camera Required)

These tests require at least one connected camera:

- Full camera workflow (discovery → connection → configuration → streaming)
- Real image acquisition and data validation
- Performance measurement with actual hardware
- Multiple camera operations

## Running Tests

### All Tests

```bash
# Run all tests
make test

# Run all tests with verbose output
make test-all
```

### Specific Test Categories

```bash
# Run only unit tests (no camera required)
make test-unit

# Run only integration tests (camera required)
make test-integration

# Run short tests only (skips long-running tests)
make test-short
```

### Coverage and Benchmarks

```bash
# Run tests with coverage report
make test-coverage

# Run all benchmarks
make benchmark

# Run only performance benchmarks
make benchmark-performance
```

### Manual Test Execution

```bash
# Run specific test functions
go test -v ./tests/ -run TestDeviceDiscovery
go test -v ./tests/ -run TestCameraWithRealDevice

# Run benchmarks for specific functions
go test -bench=BenchmarkParameterAccess ./tests/
go test -bench=BenchmarkBufferDataAccess ./tests/
```

## Test Environment Setup

### Without Cameras (Unit Tests)

Unit tests are designed to work on any system with the Aravis library installed:

- No cameras required
- Tests basic functionality and error handling
- Validates library structure and constants
- Safe to run in CI/CD environments

### With Cameras (Integration Tests)

For full integration testing, connect one or more cameras:

#### GigE Vision Setup

```bash
# Configure network interface for GigE cameras
sudo ip link set <interface> mtu 9000

# Check firewall settings
sudo ufw allow 3956/udp    # GigE Vision Discovery
sudo ufw allow 3956/tcp    # GigE Vision Control
```

#### USB3 Vision Setup

```bash
# Check USB permissions
ls -la /dev/bus/usb/

# Add user to appropriate groups if needed
sudo usermod -a -G plugdev $USER
```

#### Multiple Camera Testing

- Connect multiple cameras for multi-device tests
- Tests validate independent camera operation
- Useful for verifying resource management

## Test Structure

### Mock/Stub Testing Approach

The test suite is designed to be robust whether cameras are connected or not:

- **Graceful Degradation**: Tests skip camera-dependent operations when no cameras are available
- **Error Validation**: Tests verify proper error handling for various conditions
- **Boundary Testing**: Tests edge cases and invalid inputs
- **Performance Testing**: Benchmarks work with or without real data

### Test Data Validation

When cameras are available, tests validate:

- Data consistency between different access methods
- Buffer status and error conditions
- Frame acquisition timing and performance
- Multi-part buffer support (if supported by camera)

## Performance Benchmarks

The test suite includes comprehensive performance benchmarks:

### Parameter Access Benchmarks

- Standard vs Fast methods for camera parameters
- Memory allocation patterns
- String caching effectiveness

### Buffer Access Benchmarks

- `GetData()` vs `GetDataSlice()` vs `GetDataInto()`
- Zero-copy vs copy performance
- Memory allocation overhead

### Streaming Benchmarks

- Sustained frame rate measurement
- CPU usage optimization
- Memory efficiency validation

## Expected Test Results

### Without Cameras

- Unit tests should all pass
- Integration tests should skip gracefully
- Performance benchmarks may show limited results

### With Single Camera

- All tests should pass (camera-dependent ones)
- Performance benchmarks show real-world results
- Streaming tests validate sustained operation

### With Multiple Cameras

- Multi-device tests validate independent operation
- Resource management tests ensure proper cleanup
- Performance comparison across devices

## Troubleshooting

### Common Issues

**"No cameras found" in integration tests**

- Verify camera connections (USB/Ethernet)
- Check camera power and status
- Verify network configuration for GigE cameras
- Run `arv-tool-0.8 list` to verify Aravis can see cameras

**Permission errors**

- Add user to required groups: `sudo usermod -a -G plugdev,dialout $USER`
- Restart session after group changes
- Check USB device permissions

**Performance test failures**

- Ensure adequate system resources
- Check network configuration for GigE cameras
- Verify camera supports requested frame rates
- Monitor system CPU and memory usage

**CGO compilation errors**

- Verify Aravis development headers: `pkg-config --exists aravis-0.8`
- Install required packages: `sudo apt install libaravis-0.8-dev`
- Check Go CGO support: `go env CGO_ENABLED`

### Debug Mode

Run tests with additional debugging:

```bash
# Verbose output with timing
go test -v -x ./tests/

# Run with race detection
go test -race ./tests/

# Generate CPU profile
go test -cpuprofile=cpu.prof ./tests/
```

## Contributing

When adding new tests:

1. Follow the existing pattern of graceful degradation
2. Test both success and failure cases
3. Include appropriate benchmarks for performance-critical code
4. Document any special requirements (specific camera types, etc.)
5. Ensure tests work in both unit and integration modes