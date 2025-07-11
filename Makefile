# go-aravis Makefile
# Modern Go project build system with CGO support

# Configuration
SHELL := /bin/bash
.DEFAULT_GOAL := help

# Project information
PROJECT_NAME := go-aravis
MODULE_NAME := github.com/hybridgroup/go-aravis
GO_VERSION := 1.21

# Directories
BIN_DIR := bin
EXAMPLES_DIR := examples
CMD_DIR := cmd

# Go build settings
GO := go
GOOS := $(shell $(GO) env GOOS)
GOARCH := $(shell $(GO) env GOARCH)
CGO_ENABLED := 1

# Build flags
LDFLAGS := -ldflags="-s -w"
BUILD_FLAGS := -v $(LDFLAGS)

# Examples to build (all examples are now in subdirectories)
EXAMPLES := list_devices device_info advanced_buffer register_access get_image performance_demo

# effectively .PHONY: *
MAKEFLAGS += --always-make

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
BOLD := \033[1m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "$(BOLD)$(PROJECT_NAME) - Go Aravis Library Wrapper$(NC)"
	@echo
	@echo "$(BOLD)Usage:$(NC) make [target]"
	@echo
	@echo "$(BOLD)Targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(BLUE)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo
	@echo "$(BOLD)Examples:$(NC)"
	@echo "  $(BLUE)make build$(NC)      - Build the main library"
	@echo "  $(BLUE)make examples$(NC)   - Build all examples"
	@echo "  $(BLUE)make clean$(NC)      - Clean build artifacts"
	@echo "  $(BLUE)make test$(NC)       - Run tests"

build: ## Build the main library
	@echo "$(BOLD)Building $(PROJECT_NAME)...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(BUILD_FLAGS) .
	@echo "$(GREEN)✓ Build completed successfully$(NC)"

examples: $(BIN_DIR) ## Build all examples
	@echo "$(BOLD)Building examples...$(NC)"
	@$(MAKE) --no-print-directory build-examples
	@echo "$(GREEN)✓ All examples built successfully$(NC)"

build-examples:
	@for example in $(EXAMPLES); do \
		echo "$(BLUE)Building $$example...$(NC)"; \
		CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(BUILD_FLAGS) \
			-o $(BIN_DIR)/$$example \
			./$(EXAMPLES_DIR)/$$example/main.go; \
		if [ $$? -eq 0 ]; then \
			echo "$(GREEN)  ✓ $$example built$(NC)"; \
		else \
			echo "$(RED)  ✗ $$example failed$(NC)"; \
			exit 1; \
		fi; \
	done

test: ## Run tests
	@echo "$(BOLD)Running tests...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v ./tests/
	@echo "$(GREEN)✓ Tests completed$(NC)"

test-all: ## Run all tests including integration
	@echo "$(BOLD)Running all tests...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v ./tests/
	@echo "$(GREEN)✓ All tests completed$(NC)"

test-unit: ## Run unit tests only (mock/stub tests)
	@echo "$(BOLD)Running unit tests...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v ./tests/ -run "TestMock|TestError|TestStructural|TestConstants|TestBoundary"
	@echo "$(GREEN)✓ Unit tests completed$(NC)"

test-integration: ## Run integration tests (requires camera)
	@echo "$(BOLD)Running integration tests...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v ./tests/ -run "TestFull|TestStreaming|TestMultiple"
	@echo "$(GREEN)✓ Integration tests completed$(NC)"

test-short: ## Run short tests only
	@echo "$(BOLD)Running short tests...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v -short ./tests/
	@echo "$(GREEN)✓ Short tests completed$(NC)"

test-coverage: ## Run tests with coverage
	@echo "$(BOLD)Running tests with coverage...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v -race -coverprofile=coverage.out ./tests/
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

benchmark: ## Run benchmarks
	@echo "$(BOLD)Running benchmarks...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -bench=. -benchmem ./tests/

benchmark-performance: ## Run performance benchmarks only
	@echo "$(BOLD)Running performance benchmarks...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -bench=BenchmarkParameter -benchmem ./tests/
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -bench=BenchmarkBuffer -benchmem ./tests/
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) test -bench=BenchmarkCombined -benchmem ./tests/

fmt: ## Format code
	@echo "$(BOLD)Formatting code...$(NC)"
	@if command -v treefmt >/dev/null 2>&1; then \
		treefmt --allow-missing-formatter; \
	else \
		$(GO) fmt ./...; \
	fi
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint: ## Run linter
	@echo "$(BOLD)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config .golangci.toml; \
	else \
		echo "$(YELLOW)⚠ golangci-lint not found, running go vet instead$(NC)"; \
		$(GO) vet ./...; \
	fi
	@echo "$(GREEN)✓ Linting completed$(NC)"

tidy: ## Tidy go modules
	@echo "$(BOLD)Tidying Go modules...$(NC)"
	@$(GO) mod tidy
	@echo "$(GREEN)✓ Modules tidied$(NC)"

deps: ## Download dependencies
	@echo "$(BOLD)Downloading dependencies...$(NC)"
	@$(GO) mod download
	@echo "$(GREEN)✓ Dependencies downloaded$(NC)"

verify: ## Verify dependencies
	@echo "$(BOLD)Verifying dependencies...$(NC)"
	@$(GO) mod verify
	@echo "$(GREEN)✓ Dependencies verified$(NC)"

clean: ## Clean build artifacts
	@echo "$(BOLD)Cleaning build artifacts...$(NC)"
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@$(GO) clean -cache
	@echo "$(GREEN)✓ Clean completed$(NC)"

docker-build: ## Build Docker image
	@echo "$(BOLD)Building Docker image...$(NC)"
	@docker build -t $(PROJECT_NAME):latest .
	@echo "$(GREEN)✓ Docker image built$(NC)"

docker-run: ## Run Docker container
	@echo "$(BOLD)Running Docker container...$(NC)"
	@docker run --rm $(PROJECT_NAME):latest

install-tools: ## Install development tools
	@echo "$(BOLD)Installing development tools...$(NC)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(BLUE)Installing golangci-lint...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin; \
	fi
	@if ! command -v treefmt >/dev/null 2>&1; then \
		echo "$(BLUE)Installing treefmt...$(NC)"; \
		$(GO) install github.com/numtide/treefmt/cmd/treefmt@latest; \
	fi
	@echo "$(GREEN)✓ Development tools installed$(NC)"

check-system: ## Check system requirements
	@echo "$(BOLD)Checking system requirements...$(NC)"
	@echo "$(BLUE)Go version:$(NC) $(shell $(GO) version)"
	@echo "$(BLUE)CGO enabled:$(NC) $(CGO_ENABLED)"
	@echo "$(BLUE)OS/Arch:$(NC) $(GOOS)/$(GOARCH)"
	@echo "$(BLUE)Aravis library:$(NC)"
	@if pkg-config --exists aravis-0.8; then \
		echo "  ✓ aravis-0.8 found (version: $(shell pkg-config --modversion aravis-0.8))"; \
	else \
		echo "  ✗ aravis-0.8 not found"; \
	fi
	@echo "$(BLUE)Network MTU (for GigE cameras):$(NC)"
	@ip link show | grep -E "mtu [0-9]+" | head -3 || echo "  Could not determine MTU"

run-example: ## Run an example (usage: make run-example EXAMPLE=list_devices)
	@if [ -z "$(EXAMPLE)" ]; then \
		echo "$(RED)Error: EXAMPLE not specified$(NC)"; \
		echo "$(BLUE)Usage: make run-example EXAMPLE=list_devices$(NC)"; \
		echo "$(BLUE)Available examples:$(NC) $(EXAMPLES)"; \
		exit 1; \
	fi
	@if [ ! -f "$(BIN_DIR)/$(EXAMPLE)" ]; then \
		echo "$(YELLOW)Building $(EXAMPLE)...$(NC)"; \
		$(MAKE) --no-print-directory examples; \
	fi
	@echo "$(BOLD)Running $(EXAMPLE)...$(NC)"
	@./$(BIN_DIR)/$(EXAMPLE)

all: clean deps build examples test ## Build everything

ci: deps build lint test ## Run CI pipeline

release: clean all ## Prepare for release

# Create bin directory if it doesn't exist
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Info targets
info: ## Show project information
	@echo "$(BOLD)Project Information:$(NC)"
	@echo "  Name: $(PROJECT_NAME)"
	@echo "  Module: $(MODULE_NAME)"
	@echo "  Go Version: $(GO_VERSION)"
	@echo "  Build Flags: $(BUILD_FLAGS)"
	@echo "  Examples: $(EXAMPLES)"
	@echo "  Output Directory: $(BIN_DIR)"

list-examples: ## List all available examples
	@echo "$(BOLD)Available Examples:$(NC)"
	@for example in $(EXAMPLES); do \
		echo "  - $$example (examples/$$example/main.go)"; \
	done