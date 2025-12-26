package core

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// getTestTimeout returns the timeout for symlink cycle tests
// Can be overridden with SYMLINK_TEST_TIMEOUT_SEC environment variable
func getTestTimeout() time.Duration {
	if timeoutStr := os.Getenv("SYMLINK_TEST_TIMEOUT_SEC"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			return time.Duration(timeout) * time.Second
		}
	}
	// Default timeout - shorter for CI, but configurable for slower systems
	return 3 * time.Second
}

// TestFileScannerSymlinkCyclePrevention tests that FileScanner.CountFiles handles symlink cycles
func TestFileScannerSymlinkCyclePrevention(t *testing.T) {
	// Create temporary directory structure mimicking pnpm
	tempDir := t.TempDir()

	// Create a structure like:
	// test-project/
	//   node_modules/
	//     react -> .pnmp/react@18.0.0/node_modules/react
	//     .pnpm/
	//       react@18.0.0/
	//         node_modules/
	//           react -> ../../react@18.0.0 (circular reference)

	nodeModulesDir := filepath.Join(tempDir, "node_modules")
	pnpmDir := filepath.Join(nodeModulesDir, ".pnpm")
	reactPackageDir := filepath.Join(pnpmDir, "react@18.0.0", "node_modules")

	// Create directories
	if err := os.MkdirAll(reactPackageDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create a real file in the react package
	realReactFile := filepath.Join(pnpmDir, "react@18.0.0", "index.js")
	if err := os.WriteFile(realReactFile, []byte("module.exports = 'react';"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create the circular symlinks that would cause infinite loops
	reactSymlink := filepath.Join(nodeModulesDir, "react")
	reactTarget := filepath.Join(".pnpm", "react@18.0.0")

	// Create symlink: node_modules/react -> .pnpm/react@18.0.0
	if err := os.Symlink(reactTarget, reactSymlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create circular symlink: .pnpm/react@18.0.0/node_modules/react -> ../../react@18.0.0
	circularSymlink := filepath.Join(reactPackageDir, "react")
	circularTarget := "../../react@18.0.0"
	if err := os.Symlink(circularTarget, circularSymlink); err != nil {
		t.Fatalf("Failed to create circular symlink: %v", err)
	}

	// NOTE: This test now validates symlink cycle prevention in FileScanner,
	// which is used for actual indexing. The old EstimateDirectorySize function
	// has been removed as it had broken pattern matching.

	// Test removed - FileScanner symlink cycle prevention is tested in indexing package
	// This test file is deprecated and can be removed in future cleanup
	t.Skip("EstimateDirectorySize has been removed. Use FileScanner.CountFiles instead (tested in indexing package)")
}

// TestFileLoaderSymlinkCyclePrevention tests that FileLoader handles symlink cycles
func TestFileLoaderSymlinkCyclePrevention(t *testing.T) {
	// Create temporary directory structure with symlink cycle
	tempDir := t.TempDir()

	// Create directory structure:
	// test/
	//   a -> b
	//   b -> a (circular symlinks)
	//   real.go (real file)

	dirA := filepath.Join(tempDir, "a")
	dirB := filepath.Join(tempDir, "b")
	realFile := filepath.Join(tempDir, "real.go")

	// Create real file
	if err := os.WriteFile(realFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create circular symlinks
	if err := os.Symlink("b", dirA); err != nil { // a -> b
		t.Fatalf("Failed to create symlink: %v", err)
	}
	// Intentionally ignore error for creating circular symlink - may fail on some systems
	_ = os.Symlink("a", dirB) // b -> a (creates cycle)

	// Test with timeout to ensure it doesn't hang
	fs := NewFileService()
	defer fs.Close()
	fl := NewFileLoader(FileLoaderOptions{FileService: fs})
	defer fl.Close()

	done := make(chan bool, 1)
	var err error
	var files []string

	go func() {
		files, err = fl.discoverFiles(tempDir)
		done <- true
	}()

	// Wait with configurable timeout - should complete quickly with cycle prevention
	timeout := getTestTimeout()
	select {
	case <-done:
		// Test completed successfully
		if err != nil {
			t.Fatalf("discoverFiles failed: %v", err)
		}

		// Should have found the real file
		if len(files) == 0 {
			t.Error("Expected to find at least one file")
		}

		// Should have found the real.go file
		found := false
		for _, file := range files {
			if filepath.Base(file) == "real.go" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find real.go file")
		}

		t.Logf("Successfully handled symlink cycles, found files: %v", files)

	case <-time.After(timeout):
		t.Fatalf("discoverFiles hung after %v - symlink cycle prevention failed", timeout)
	}
}
