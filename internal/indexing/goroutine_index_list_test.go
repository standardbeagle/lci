package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
)

// TestListFiles tests the list files.
func TestListFiles(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go":          "package main\nfunc main() {}",
		"helper.go":        "package main\nfunc helper() {}",
		"test/test.go":     "package test\nfunc TestMain() {}",
		"docs/readme.md":   "# Documentation",
		"build/output.bin": "binary content",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	tests := []struct {
		name     string
		include  []string
		exclude  []string
		minFiles int // minimum number of files expected
	}{
		{
			name:     "Go files only",
			include:  []string{"**/*.go"},
			exclude:  []string{},
			minFiles: 3,
		},
		{
			name:     "All files",
			include:  []string{"**/*.*"},
			exclude:  []string{},
			minFiles: 4, // should find most files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Project: config.Project{Root: tmpDir},
				Include: tt.include,
				Exclude: tt.exclude,
				Index: config.Index{
					MaxFileSize:      10 * 1024 * 1024, // 10MB
					MaxTotalSizeMB:   100,
					MaxFileCount:     1000,
					SmartSizeControl: false,
				},
				Performance: config.Performance{
					MaxMemoryMB:   100,
					MaxGoroutines: 2,
				},
			}

			indexer := NewMasterIndex(cfg)
			defer indexer.Close() // Ensure cleanup to prevent goroutine leaks

			// Capture output by redirecting stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run ListFiles with verbose=false
			err := indexer.ListFiles(context.Background(), tmpDir, false)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout

			output := make([]byte, 1024)
			n, _ := r.Read(output)
			outputStr := string(output[:n])

			if err != nil {
				t.Fatalf("ListFiles failed: %v", err)
			}

			// Count actual lines (excluding empty lines and totals)
			lines := strings.Split(strings.TrimSpace(outputStr), "\n")
			actualFileCount := 0
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "Total:") {
					actualFileCount++
				}
			}

			if actualFileCount < tt.minFiles {
				t.Errorf("Expected at least %d files, got %d files. Output: %s",
					tt.minFiles, actualFileCount, outputStr)
			}
		})
	}
}

// TestListFilesVerbose tests the list files verbose.
func TestListFilesVerbose(t *testing.T) {
	// Create temporary directory with one test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			SmartSizeControl: false,
		},
		Performance: config.Performance{
			MaxMemoryMB:   100,
			MaxGoroutines: 2,
		},
	}

	indexer := NewMasterIndex(cfg)
	defer indexer.Close() // Ensure cleanup to prevent goroutine leaks

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run ListFiles with verbose=true
	err := indexer.ListFiles(context.Background(), tmpDir, true)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout

	output := make([]byte, 1024)
	n, _ := r.Read(output)
	outputStr := string(output[:n])

	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Check that verbose output contains file details
	if !strings.Contains(outputStr, testFile) {
		t.Errorf("Expected file path %s not found in output", testFile)
	}
	if !strings.Contains(outputStr, "priority:") {
		t.Errorf("Expected 'priority:' not found in verbose output")
	}
	if !strings.Contains(outputStr, "size:") {
		t.Errorf("Expected 'size:' not found in verbose output")
	}
	if !strings.Contains(outputStr, "bytes") {
		t.Errorf("Expected 'bytes' not found in verbose output")
	}
}

// TestListFilesEmptyDirectory tests the list files empty directory.
func TestListFilesEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			SmartSizeControl: false,
		},
		Performance: config.Performance{
			MaxMemoryMB:   100,
			MaxGoroutines: 2,
		},
	}

	indexer := NewMasterIndex(cfg)

	// Capture stderr for the total count
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := indexer.ListFiles(context.Background(), tmpDir, false)

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr

	output := make([]byte, 1024)
	n, _ := r.Read(output)
	outputStr := string(output[:n])

	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Should show "Total: 0 files would be indexed"
	if !strings.Contains(outputStr, "Total: 0 files") {
		t.Errorf("Expected 'Total: 0 files' in output, got: %s", outputStr)
	}
}
