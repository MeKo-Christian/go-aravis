# go-aravis justfile
# Mirrors the Makefile targets; see `just --list` for an overview.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Project information

project_name := "go-aravis"
module_name := "github.com/MeKo-Christian/go-aravis"
go_version := "1.23"

# Directories

bin_dir := "bin"
examples_dir := "examples"

# Go build settings

go := "go"
export CGO_ENABLED := "1"

# Build flags

build_flags := "-v -ldflags='-s -w'"

# Examples to build (all examples live in subdirectories)

examples_list := "list_devices device_info advanced_buffer register_access get_image performance_demo"

# -run '^$' keeps -bench from re-running the whole test suite alongside the
# benchmarks. BENCHTIME is overridable so CI can execute every benchmark body
# in a few seconds (BENCHTIME=10x) without pretending the timings are
# comparable across runners.

benchtime := env_var_or_default("BENCHTIME", "1s")
bench_flags := "-benchmem -run '^$' -benchtime=" + benchtime

# Colors for output

green := '\033[0;32m'
yellow := '\033[0;33m'
blue := '\033[0;34m'
bold := '\033[1m'
nc := '\033[0m'

# Show this help message
default:
    @echo -e "{{ bold }}{{ project_name }} - Go Aravis Library Wrapper{{ nc }}"
    @echo
    @just --list --unsorted

# Build the main library
build:
    @echo -e "{{ bold }}Building {{ project_name }}...{{ nc }}"
    {{ go }} build {{ build_flags }} .
    @echo -e "{{ green }}✓ Build completed successfully{{ nc }}"

# Build all examples
examples:
    @echo -e "{{ bold }}Building examples...{{ nc }}"
    @mkdir -p {{ bin_dir }}
    @for example in {{ examples_list }}; do \
        echo -e "{{ blue }}Building $example...{{ nc }}"; \
        {{ go }} build {{ build_flags }} -o {{ bin_dir }}/$example ./{{ examples_dir }}/$example/main.go; \
        echo -e "{{ green }}  ✓ $example built{{ nc }}"; \
    done
    @echo -e "{{ green }}✓ All examples built successfully{{ nc }}"

# Run tests
test:
    @echo -e "{{ bold }}Running tests...{{ nc }}"
    {{ go }} test -v ./...
    @echo -e "{{ green }}✓ Tests completed{{ nc }}"

# Run all tests including integration
test-all:
    @echo -e "{{ bold }}Running all tests...{{ nc }}"
    {{ go }} test -v ./...
    @echo -e "{{ green }}✓ All tests completed{{ nc }}"

# Every test in this repository runs against Aravis's built-in Fake backend
# (see tests/fake_test.go), so "unit" is simply the whole suite: no camera, no
# allowlist.

# Run the full hardware-free suite (Fake backend)
test-unit:
    @echo -e "{{ bold }}Running unit tests...{{ nc }}"
    {{ go }} test -race ./...
    @echo -e "{{ green }}✓ Unit tests completed{{ nc }}"

# ARAVIS_TEST_HARDWARE makes requireStreamingCamera select the first non-Fake
# device and fail when there is none, so this recipe cannot report success
# without having driven a physical camera. TestMultipleDevices additionally
# needs two of them and skips otherwise.

# Run against real hardware (requires a connected camera)
test-integration:
    @echo -e "{{ bold }}Running integration tests...{{ nc }}"
    ARAVIS_TEST_HARDWARE=1 {{ go }} test -v ./tests/ \
        -run "TestFullWorkflow|TestStreamingPerformance|TestMultipleDevices"
    @echo -e "{{ green }}✓ Integration tests completed{{ nc }}"

# Run short tests only
test-short:
    @echo -e "{{ bold }}Running short tests...{{ nc }}"
    {{ go }} test -short -race ./...
    @echo -e "{{ green }}✓ Short tests completed{{ nc }}"

# -coverpkg is what makes this report anything: ./tests is an *external* test
# package with no non-test code of its own, so without it the profile
# instruments nothing. ./... also picks up the root-package tests.

# Run tests with coverage
test-coverage:
    @echo -e "{{ bold }}Running tests with coverage...{{ nc }}"
    {{ go }} test -race -coverpkg={{ module_name }} -coverprofile=coverage.out ./...
    {{ go }} tool cover -html=coverage.out -o coverage.html
    {{ go }} tool cover -func=coverage.out | tail -1
    @echo -e "{{ green }}✓ Coverage report generated: coverage.html{{ nc }}"

# Run benchmarks
benchmark:
    @echo -e "{{ bold }}Running benchmarks...{{ nc }}"
    {{ go }} test -bench=. {{ bench_flags }} ./tests/

# Run performance benchmarks only
benchmark-performance:
    @echo -e "{{ bold }}Running performance benchmarks...{{ nc }}"
    {{ go }} test -bench=BenchmarkParameter {{ bench_flags }} ./tests/
    {{ go }} test -bench=BenchmarkBuffer {{ bench_flags }} ./tests/
    {{ go }} test -bench=BenchmarkCombined {{ bench_flags }} ./tests/

# Format code
fmt:
    @echo -e "{{ bold }}Formatting code...{{ nc }}"
    @if command -v treefmt >/dev/null 2>&1; then \
        treefmt --allow-missing-formatter; \
    else \
        {{ go }} fmt ./...; \
    fi
    @echo -e "{{ green }}✓ Code formatted{{ nc }}"

# Run linter
lint:
    @echo -e "{{ bold }}Running linter...{{ nc }}"
    @if command -v golangci-lint >/dev/null 2>&1; then \
        golangci-lint run --config .golangci.toml; \
    else \
        echo -e "{{ yellow }}⚠ golangci-lint not found, running go vet instead{{ nc }}"; \
        {{ go }} vet ./...; \
    fi
    @echo -e "{{ green }}✓ Linting completed{{ nc }}"

# Tidy go modules
tidy:
    @echo -e "{{ bold }}Tidying Go modules...{{ nc }}"
    {{ go }} mod tidy
    @echo -e "{{ green }}✓ Modules tidied{{ nc }}"

# Download dependencies
deps:
    @echo -e "{{ bold }}Downloading dependencies...{{ nc }}"
    {{ go }} mod download
    @echo -e "{{ green }}✓ Dependencies downloaded{{ nc }}"

# Verify dependencies
verify:
    @echo -e "{{ bold }}Verifying dependencies...{{ nc }}"
    {{ go }} mod verify
    @echo -e "{{ green }}✓ Dependencies verified{{ nc }}"

# Clean build artifacts
clean:
    @echo -e "{{ bold }}Cleaning build artifacts...{{ nc }}"
    rm -rf {{ bin_dir }}
    rm -f coverage.out coverage.html
    {{ go }} clean -cache
    @echo -e "{{ green }}✓ Clean completed{{ nc }}"

# Build Docker image
docker-build:
    @echo -e "{{ bold }}Building Docker image...{{ nc }}"
    docker build -t {{ project_name }}:latest .
    @echo -e "{{ green }}✓ Docker image built{{ nc }}"

# Run Docker container
docker-run:
    @echo -e "{{ bold }}Running Docker container...{{ nc }}"
    docker run --rm {{ project_name }}:latest

# Install development tools
install-tools:
    @echo -e "{{ bold }}Installing development tools...{{ nc }}"
    @if ! command -v golangci-lint >/dev/null 2>&1; then \
        echo -e "{{ blue }}Installing golangci-lint...{{ nc }}"; \
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
            | sh -s -- -b "$({{ go }} env GOPATH)/bin"; \
    fi
    @if ! command -v treefmt >/dev/null 2>&1; then \
        echo -e "{{ blue }}Installing treefmt...{{ nc }}"; \
        {{ go }} install github.com/numtide/treefmt/cmd/treefmt@latest; \
    fi
    @echo -e "{{ green }}✓ Development tools installed{{ nc }}"

# Check system requirements
check-system:
    @echo -e "{{ bold }}Checking system requirements...{{ nc }}"
    @echo -e "{{ blue }}Go version:{{ nc }} $({{ go }} version)"
    @echo -e "{{ blue }}CGO enabled:{{ nc }} ${CGO_ENABLED}"
    @echo -e "{{ blue }}OS/Arch:{{ nc }} $({{ go }} env GOOS)/$({{ go }} env GOARCH)"
    @echo -e "{{ blue }}Aravis library:{{ nc }}"
    @if pkg-config --exists aravis-0.8; then \
        echo "  ✓ aravis-0.8 found (version: $(pkg-config --modversion aravis-0.8))"; \
    else \
        echo "  ✗ aravis-0.8 not found"; \
    fi
    @echo -e "{{ blue }}Network MTU (for GigE cameras):{{ nc }}"
    @ip link show | grep -E "mtu [0-9]+" | head -3 || echo "  Could not determine MTU"

# Run an example (usage: just run-example list_devices)
run-example example:
    @if [ ! -f "{{ bin_dir }}/{{ example }}" ]; then \
        echo -e "{{ yellow }}Building {{ example }}...{{ nc }}"; \
        just examples; \
    fi
    @echo -e "{{ bold }}Running {{ example }}...{{ nc }}"
    ./{{ bin_dir }}/{{ example }}

# Build everything
all: clean deps build examples test

# Run CI pipeline
ci: deps build lint test

# Prepare for release
release: clean all

# Show project information
info:
    @echo -e "{{ bold }}Project Information:{{ nc }}"
    @echo "  Name: {{ project_name }}"
    @echo "  Module: {{ module_name }}"
    @echo "  Go Version: {{ go_version }}"
    @echo "  Build Flags: {{ build_flags }}"
    @echo "  Examples: {{ examples_list }}"
    @echo "  Output Directory: {{ bin_dir }}"

# List all available examples
list-examples:
    @echo -e "{{ bold }}Available Examples:{{ nc }}"
    @for example in {{ examples_list }}; do \
        echo "  - $example (examples/$example/main.go)"; \
    done
