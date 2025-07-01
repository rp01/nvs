# NVS - Node Version Switcher Makefile

# Variables
BINARY_NAME=nvs
MAIN_PACKAGE=.
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)

# Run tests
.PHONY: test
test:
	go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -f nvs-*
	rm -f coverage.out coverage.html

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Cross-compilation targets
.PHONY: build-all
build-all: build-linux build-macos build-windows

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o nvs-linux-x64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o nvs-linux-arm64 $(MAIN_PACKAGE)

.PHONY: build-macos
build-macos:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o nvs-macos-x64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o nvs-macos-arm64 $(MAIN_PACKAGE)

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o nvs-windows-x64.exe $(MAIN_PACKAGE)

# Run with arguments
.PHONY: run
run:
	go run $(MAIN_PACKAGE) $(ARGS)

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Install development tools
.PHONY: install-tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Release build (optimized)
.PHONY: release
release:
	CGO_ENABLED=0 go build $(LDFLAGS) -a -installsuffix cgo -o $(BINARY_NAME) $(MAIN_PACKAGE)

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build for current platform"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  build-all     - Cross-compile for all platforms"
	@echo "  build-linux   - Build for Linux (amd64 and arm64)"
	@echo "  build-macos   - Build for macOS (amd64 and arm64)"
	@echo "  build-windows - Build for Windows (amd64)"
	@echo "  run           - Run with arguments (use ARGS=...)"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  install-tools - Install development tools"
	@echo "  release       - Optimized release build"
	@echo "  help          - Show this help" 