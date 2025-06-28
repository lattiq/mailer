# Makefile for lattiq/mailer

# Version and build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Package information
PACKAGE := github.com/lattiq/mailer

# ldflags for version injection
LDFLAGS := -ldflags "\
	-X $(PACKAGE).Version=$(VERSION) \
	-X $(PACKAGE).GitCommit=$(COMMIT) \
	-X $(PACKAGE).GitBranch=$(BRANCH) \
	-X $(PACKAGE).BuildDate=$(BUILD_DATE) \
	-X $(PACKAGE).GoVersion=$(GO_VERSION)"

# Build directory
BUILD_DIR := build
BIN_DIR := bin

# Default target
.PHONY: all
all: test lint build

# Build the library (creates example binary for testing)
.PHONY: build
build:
	@echo "Building with version: $(VERSION)"
	go build $(LDFLAGS) ./...

# Build with specific version
.PHONY: build-version
build-version:
	@if [ -z "$(V)" ]; then echo "Usage: make build-version V=v1.0.1"; exit 1; fi
	@echo "Building with version: $(V)"
	go build -ldflags "-X $(PACKAGE).Version=$(V) -X $(PACKAGE).GitCommit=$(COMMIT) -X $(PACKAGE).BuildDate=$(BUILD_DATE)" ./...

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -race -cover ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	go test -race -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated: $(BUILD_DIR)/coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Lint the code
.PHONY: lint
lint:
	@echo "Linting code..."
	@which staticcheck > /dev/null || (echo "Installing staticcheck..." && go install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck ./...
	go vet ./...
	@if [ -n "$$(gofmt -l .)" ]; then echo "Code needs formatting. Run 'make fmt'"; exit 1; fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	gofmt -w .

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	go vet ./...

# Security scan
.PHONY: security
security:
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec ./...

# Fix linting issues
.PHONY: lint-fix
lint-fix: fmt tidy

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	go clean ./...
	rm -rf $(BUILD_DIR) $(BIN_DIR)

# Show current version information
.PHONY: version
version:
	@echo "Current version: $(VERSION)"
	@echo "Git commit: $(COMMIT)"
	@echo "Git branch: $(BRANCH)"
	@echo "Build date: $(BUILD_DATE)"
	@echo "Go version: $(GO_VERSION)"

# Create a new release
.PHONY: release
release:
	@if [ -z "$(V)" ]; then echo "Usage: make release V=v1.0.1"; exit 1; fi
	@echo "Creating release $(V)..."
	@if git rev-parse $(V) >/dev/null 2>&1; then echo "Tag $(V) already exists!"; exit 1; fi
	git tag $(V)
	@echo "Release $(V) created. Push with: git push origin $(V)"

# Development build (no version injection)
.PHONY: dev
dev:
	@echo "Development build..."
	go build ./...

# Install development dependencies
.PHONY: deps
deps:
	@echo "Installing development dependencies..."
	go mod download
	go install honnef.co/go/tools/cmd/staticcheck@latest

# Tidy up go modules
.PHONY: tidy
tidy:
	@echo "Tidying go modules..."
	go mod tidy

# Verify dependencies
.PHONY: verify
verify:
	@echo "Verifying dependencies..."
	go mod verify

# Cross-platform builds
.PHONY: build-all
build-all: build-linux build-windows build-darwin

.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/mailer-linux-amd64

.PHONY: build-windows  
build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/mailer-windows-amd64.exe

.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/mailer-darwin-amd64

# Run all checks (useful for CI)
.PHONY: check
check: test vet lint build

# Run all checks including security
.PHONY: check-all
check-all: test vet lint security build

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Run tests, lint, and build"
	@echo "  build        - Build with auto-detected version"
	@echo "  build-version V=v1.0.1 - Build with specific version"
	@echo "  build-all    - Build for all platforms"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  bench        - Run benchmarks"
	@echo "  lint         - Run linters"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  security     - Run security scan (gosec)"
	@echo "  lint-fix     - Fix linting issues"
	@echo "  clean        - Clean build artifacts"
	@echo "  version      - Show version information"
	@echo "  release V=v1.0.1 - Create a new release tag"
	@echo "  dev          - Development build (no version injection)"
	@echo "  deps         - Install development dependencies"
	@echo "  tidy         - Tidy go modules"
	@echo "  verify       - Verify dependencies"
	@echo "  check        - Run all checks (test + lint + build)"
	@echo "  check-all    - Run all checks including security"
	@echo "  help         - Show this help" 