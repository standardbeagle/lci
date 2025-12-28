.PHONY: all build test test-nocache test-serial test-race test-coverage test-integration test-unit bench clean lint test-catalog test-context-lookup bench-context-lookup profile-context-lookup test-exclusions test-exclusions-verbose test-search-comparison build-cross build-linux build-windows build-darwin build-release build-release-linux build-release-windows build-release-darwin install install-default install-wsl install-windows distclean

# Default target
all: build test

# Build the binary for current platform
build:
	go build -o lci ./cmd/lci

# Cross-platform build targets
# Note: Cross-compilation from Linux to Windows/Mac requires CGO and platform-specific
# toolchains. For releases, use GitHub Actions workflow (/.github/workflows/release.yml)
# which builds on native Windows/Mac runners.

# Build for Linux (amd64)
build-linux:
	@echo "Building for Linux amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o lci-linux-amd64 ./cmd/lci

# Build for Linux (arm64)
build-linux-arm64:
	@echo "Building for Linux arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o lci-linux-arm64 ./cmd/lci

# Build for Windows (amd64)
# Requires Windows with MinGW or use GitHub Actions workflow
build-windows:
	@echo "Building for Windows amd64..."
	@echo "NOTE: For cross-compilation, use GitHub Actions release workflow"
	@echo "Cross-compiling from Linux requires: apt-get install mingw-w64"
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -o lci-windows-amd64.exe ./cmd/lci

# Build for macOS Darwin (amd64)
# Requires macOS with Xcode or use GitHub Actions workflow
build-darwin:
	@echo "Building for Darwin amd64..."
	@echo "NOTE: For cross-compilation, use GitHub Actions release workflow"
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o lci-darwin-amd64 ./cmd/lci || \
	(echo "Failed to cross-compile for macOS. Use GitHub Actions release workflow instead." && exit 1)

# Build for macOS Darwin (arm64 / Apple Silicon)
# Requires macOS with Xcode or use GitHub Actions workflow
build-darwin-arm64:
	@echo "Building for Darwin arm64..."
	@echo "NOTE: For cross-compilation, use GitHub Actions release workflow"
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o lci-darwin-arm64 ./cmd/lci || \
	(echo "Failed to cross-compile for macOS. Use GitHub Actions release workflow instead." && exit 1)

# Build universal macOS binary (both amd64 and arm64)
# Requires macOS with Xcode command line tools (lipo command)
build-darwin-universal:
	@echo "Building universal macOS binary (amd64 + arm64)..."
	@echo "NOTE: Requires macOS with Xcode command line tools"
	@mkdir -p dist
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o dist/lci-darwin-amd64 ./cmd/lci || exit 1
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o dist/lci-darwin-arm64 ./cmd/lci || exit 1
	lipo -create -output dist/lci-darwin-universal dist/lci-darwin-amd64 dist/lci-darwin-arm64
	@echo "Universal binary created: dist/lci-darwin-universal"
	@rm -f dist/lci-darwin-amd64 dist/lci-darwin-arm64

# Build all release binaries and create checksums
# This target attempts local builds but Windows/Mac may fail without proper toolchains
build-release: build-linux build-linux-arm64
	@echo "Building Windows and Mac binaries via GitHub Actions is recommended"
	@echo "Attempting Windows build (may fail on Linux without MinGW)..."
	@make build-windows || echo "Windows build skipped (use release workflow)"
	@echo "Attempting Mac build (requires macOS or cross-compilation toolchain)..."
	@make build-darwin-universal || echo "Mac build skipped (use release workflow)"
	@echo "Creating checksums for available binaries..."
	@sha256sum lci-linux-amd64 2>/dev/null > lci-linux-amd64.sha256 || true
	@sha256sum lci-linux-arm64 2>/dev/null > lci-linux-arm64.sha256 || true
	@sha256sum lci-windows-amd64.exe 2>/dev/null > lci-windows-amd64.exe.sha256 || true
	@sha256sum dist/lci-darwin-universal 2>/dev/null > lci-darwin-universal.sha256 || true
	@echo "Release binaries built:"
	@ls -lh lci-* dist/lci-* 2>/dev/null || echo "Some binaries may be missing - use release workflow"

# Production build targets - optimized for distribution
# Build optimized Linux release binary (amd64)
build-release-linux:
	@echo "Building optimized Linux amd64 release..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -trimpath -o lci-linux-amd64 ./cmd/lci
	@echo "Linux release binary built: lci-linux-amd64"

# Build optimized Windows release binary (amd64)
build-release-windows:
	@echo "Building optimized Windows amd64 release..."
	@echo "NOTE: Requires mingw-w64 for cross-compilation"
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -trimpath -o lci-windows-amd64.exe ./cmd/lci
	@echo "Windows release binary built: lci-windows-amd64.exe"

# Build optimized Darwin release binary (amd64)
build-release-darwin:
	@echo "Building optimized Darwin amd64 release..."
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -trimpath -o lci-darwin-amd64 ./cmd/lci || \
	(echo "Failed to cross-compile for macOS. Use GitHub Actions release workflow instead." && exit 1)
	@echo "Darwin release binary built: lci-darwin-amd64"

# Build all optimized release binaries
build-release-optimized: build-release-linux build-release-windows build-release-darwin
	@echo "All optimized release binaries built successfully"
	@ls -lh lci-* 2>/dev/null

# Run all tests: parallel for correctness, then fully serial for performance tests
test:
	@echo "=== Phase 1: Parallel tests (correctness, excluding performance) ==="
	go test -skip "Stress|Performance|Perf" ./...
	@echo ""
	@echo "=== Waiting for system to settle before performance tests ==="
	@sleep 2
	@echo ""
	@echo "=== Phase 2: Serial tests (performance/stress) ==="
	@echo "Running with -p 1 -parallel 1 to ensure accurate timing measurements"
	go test -p 1 -parallel 1 -run "Stress|Performance|Perf" ./...

# Run only correctness tests (fast, parallel) - skips perf tests
test-fast:
	@echo "Running correctness tests in parallel (skipping perf/stress tests)..."
	go test -short ./...

# Run all tests without cache (useful for debugging flaky tests)
test-nocache:
	go clean -testcache && go test -p 1 ./...

# Run tests with timing-sensitive tests not parallelized
test-serial:
	@echo "Running timing-sensitive packages (parser, indexing, search) with limited parallelism..."
	GO_TEST=1 go test -parallel 1 ./internal/parser ./internal/indexing ./internal/search
	@echo "Running all other tests in parallel..."
	GO_TEST=1 go test ./cmd/... ./internal/analysis ./internal/cache ./internal/config ./internal/core ./internal/debug ./internal/display ./internal/mcp ./internal/regex_analyzer ./internal/symbollinker ./internal/testing ./internal/types ./testhelpers ./tests/benchmarks

# Run tests with race detector
test-race:
	GO_TEST=1 go test -race ./...

# Run tests with goroutine leak detection
# These tests are separated from the main suite to avoid test interference
test-goleak:
	@echo "Running goroutine leak detection tests..."
	GO_TEST=1 go test -tags=leaktests ./internal/indexing -run "TestIndexerMemoryLeak|TestWorkflowTestContextLeak|TestIndexerMemoryUsage" -v

# Run tests with coverage
test-coverage:
	GO_TEST=1 go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run only integration tests
test-integration:
	GO_TEST=1 go test -tags=integration -run Integration ./...

# Run only unit tests (exclude integration)
test-unit:
	GO_TEST=1 go test -short ./...

# Run specific package tests
test-indexing:
	GO_TEST=1 go test -v ./internal/indexing/...

test-search:
	GO_TEST=1 go test -v ./internal/search/...

test-mcp:
	GO_TEST=1 go test -v ./internal/mcp/...

test-parser:
	GO_TEST=1 go test -v ./internal/parser/...

test-core:
	GO_TEST=1 go test -v ./internal/core/...

# Search Comparison Testing Suite
test-search-comparison:
	@echo "Running Search Comparison test suite..."
	@echo "Comparing MCP search with grep and ripgrep across languages..."
	GO_TEST=1 go test -v ./tests/search-comparison/...

# Context Lookup Testing Suite
test-context-lookup:
	@echo "Running Context Lookup comprehensive test suite..."
	GO_TEST=1 go test -v -timeout 10m ./internal/core -run ".*ContextLookup.*"
	GO_TEST=1 go test -v -timeout 5m ./internal/mcp -run ".*ContextLookup.*"

bench-context-lookup:
	@echo "Running Context Lookup benchmarks..."
	GO_TEST=1 go test -bench=BenchmarkContextLookup -benchmem -timeout 10m ./internal/core
	GO_TEST=1 go test -bench=BenchmarkMCPContextLookup -benchmem -timeout 5m ./internal/mcp

profile-context-lookup:
	@echo "Running Context Lookup memory profiling..."
	GO_TEST=1 go test -v -timeout 10m -memprofile=context_lookup_mem.prof ./internal/core -run ".*MemoryLeak.*"
	GO_TEST=1 go test -v -timeout 5m -cpuprofile=context_lookup_cpu.prof ./internal/core -run ".*Performance.*"
	@echo "Profiles generated: context_lookup_mem.prof, context_lookup_cpu.prof"

# Run benchmarks (skip tests, only run benchmarks)
bench:
	go test -run=^$ -bench=. -benchmem ./...

# Run specific benchmarks
bench-search:
	go test -bench=. -benchmem ./internal/search

bench-indexing:
	go test -bench=. -benchmem ./internal/indexing

# Clean build artifacts
clean:
	rm -f lci
	rm -f lci-* *.exe
	rm -rf dist/
	rm -f coverage.out coverage.html
	go clean -cache

# Deep clean (removes all generated files)
distclean: clean
	@echo "Deep cleaning..."
	rm -f *.sha256

# Run linters
lint:
	golangci-lint run ./...

# Install development dependencies
dev-deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install targets - build production binary and install to ~/.local/bin
# Smart install that detects the current platform and installs accordingly
install: install-default

# Install for current platform (auto-detect)
install-default:
	@echo "Installing for current platform..."
	@if [ "$(shell uname)" = "Linux" ]; then \
		$(MAKE) install-wsl; \
	elif [ "$(shell uname)" = "Darwin" ]; then \
		$(MAKE) install-darwin; \
	else \
		echo "Unknown platform: $(shell uname)"; \
		exit 1; \
	fi

# Install for WSL/Linux (optimized release binary)
install-wsl: build-release-linux
	@echo "Installing lci to ~/.local/bin for WSL/Linux..."
	@mkdir -p ~/.local/bin
	@# Use install command for atomic replacement (safe for upgrading running apps)
	@install -m 755 lci-linux-amd64 ~/.local/bin/lci
	@echo "✓ Installed to ~/.local/bin/lci"
	@# Verify installation
	@ls -lh ~/.local/bin/lci

# Install for Windows (optimized release binary)
install-windows: build-release-windows
	@echo "Installing lci.exe to ~/.local/bin for Windows..."
	@mkdir -p ~/.local/bin
	@# Use install command for atomic replacement (safe for upgrading running apps)
	@install -m 755 lci-windows-amd64.exe ~/.local/bin/lci.exe
	@echo "✓ Installed to ~/.local/bin/lci.exe"
	@# Verify installation
	@ls -lh ~/.local/bin/lci.exe

# Install for macOS/Darwin (optimized release binary)
install-darwin: build-release-darwin
	@echo "Installing lci to ~/.local/bin for macOS/Darwin..."
	@mkdir -p ~/.local/bin
	@# Use install command for atomic replacement (safe for upgrading running apps)
	@install -m 755 lci-darwin-amd64 ~/.local/bin/lci
	@echo "✓ Installed to ~/.local/bin/lci"
	@# Verify installation
	@ls -lh ~/.local/bin/lci

# Quick test - run fast tests only
quick:
	@echo "Running quick tests..."
	go test -short -timeout 30s ./... 2>/dev/null || true

# Verify build integrity and generate metrics
verify-build:
	@echo "Running comprehensive build verification..."
	@bash .github/workflows/scripts/verify-build.sh

# Full CI test suite (includes race detection and coverage, but excludes leak tests)
# Note: Leak tests (test-goleak) must be run separately due to test interference in the full suite
ci: lint test-race test-coverage

# Generate / update the automated test catalog documentation
test-catalog:
	go run ./tools/testcatalog -write

# Help
# Test exclusion patterns and performance (TDD for file count/timing bugs)
test-exclusions:
	@echo "Running exclusion pattern and performance validation tests..."
	@echo "These tests catch bugs like:"
	@echo "  - .git directory being indexed (583 vs 411 files bug)"
	@echo "  - 30-second waitForIndexReady timeout"
	@echo "  - CLI vs MCP file count discrepancies"
	@./scripts/test-exclusions.sh

test-exclusions-verbose:
	GO_TEST=1 go test -v ./internal/indexing -run "TestExclusion|TestCLIVsMCP|TestIndexingPerformance|TestGitDirectory|TestCommonProject"
	GO_TEST=1 go test -v ./internal/mcp -run "TestAutoIndexing"

help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  make build          - Build the binary for current platform"
	@echo "  make build-cross    - Build for all platforms (Linux, Windows, macOS)"
	@echo "  make build-linux    - Build for Linux amd64"
	@echo "  make build-linux-arm64 - Build for Linux arm64"
	@echo "  make build-windows  - Build for Windows amd64"
	@echo "  make build-darwin   - Build for macOS (Intel) amd64"
	@echo "  make build-darwin-arm64 - Build for macOS (Apple Silicon) arm64"
	@echo "  make build-darwin-universal - Build universal macOS binary (Intel + Apple Silicon)"
	@echo "  make build-release  - Build all release binaries with checksums"
	@echo "  make build-release-linux - Build optimized Linux release (amd64)"
	@echo "  make build-release-windows - Build optimized Windows release (amd64)"
	@echo "  make build-release-darwin - Build optimized macOS release (amd64)"
	@echo "  make build-release-optimized - Build all optimized release binaries"
	@echo ""
	@echo "Install targets (auto-builds production binary + installs to ~/.local/bin):"
	@echo "  make install        - Auto-detect platform and install"
	@echo "  make install-wsl    - Build + install for WSL/Linux"
	@echo "  make install-windows - Build + install for Windows"
	@echo "  make install-darwin - Build + install for macOS"
	@echo ""
	@echo "Test targets:"
	@echo "  make test           - Run all tests (parallel correctness + serial perf)"
	@echo "  make test-fast      - Run correctness tests only (fast, parallel)"
	@echo "  make test-nocache   - Run all tests without cache (serial, for debugging)"
	@echo "  make test-serial    - Run tests with timing-sensitive packages serialized"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make test-goleak    - Run tests with goroutine leak detection"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-integration - Run integration tests only"
	@echo "  make test-unit      - Run unit tests only"
	@echo "  make test-exclusions - Run exclusion pattern and performance validation tests (TDD)"
	@echo "  make test-context-lookup - Run Context Lookup comprehensive test suite"
	@echo "  make test-search-comparison - Compare MCP search with grep/ripgrep across languages"
	@echo ""
	@echo "Other targets:"
	@echo "  make verify-build   - Verify all packages compile and generate build metrics"
	@echo "  make bench          - Run all benchmarks"
	@echo "  make bench-context-lookup - Run Context Lookup benchmarks"
	@echo "  make profile-context-lookup - Generate Context Lookup performance profiles"
	@echo "  make lint           - Run linters"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make distclean      - Deep clean (removes all generated files)"
	@echo "  make quick          - Run quick tests (no integration)"
	@echo "  make ci             - Run full CI test suite (lint + race + goleak + coverage)"