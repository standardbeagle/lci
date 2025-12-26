package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
)

// TestRelativePathMatching verifies that glob patterns work correctly
// with relative paths when using --root flag or running from different directories
func TestRelativePathMatching(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()

	// Create subdirectory with test files
	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create test files in subdirectory
	testFile1 := filepath.Join(srcDir, "main.go")
	testFile2 := filepath.Join(srcDir, "util.go")
	rootFile := filepath.Join(tmpDir, "README.md")

	err = os.WriteFile(testFile1, []byte("package main\nfunc main() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(testFile2, []byte("package main\nfunc helper() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(rootFile, []byte("# Test Project\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}

	// Test Case 1: Using absolute path as root
	t.Run("absolute path as root", func(t *testing.T) {
		cfg := &config.Config{
			Project: config.Project{Root: tmpDir},
			Index:   config.Index{MaxFileSize: 1024 * 1024},
			Include: []string{"**/*.go", "**/*.md"},
			Exclude: []string{},
		}

		scanner := NewFileScanner(cfg, 100)

		fileCount, _, err := scanner.CountFiles(context.Background(), tmpDir)
		if err != nil {
			t.Fatalf("CountFiles failed: %v", err)
		}

		// Should find 2 .go files and 1 .md file in subdirectories
		if fileCount != 3 {
			t.Errorf("Expected 3 files, got %d", fileCount)
		}

		t.Logf("Found %d files", fileCount)
	})

	// Test Case 2: Using relative path (simulates --root flag with relative path)
	t.Run("relative path as root", func(t *testing.T) {
		// Change to temp directory to simulate running from project root
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		err = os.Chdir(tmpDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}

		cfg := &config.Config{
			Project: config.Project{Root: "."},
			Index:   config.Index{MaxFileSize: 1024 * 1024},
			Include: []string{"**/*.go", "**/*.md"},
			Exclude: []string{},
		}

		scanner := NewFileScanner(cfg, 100)

		fileCount, _, err := scanner.CountFiles(context.Background(), ".")
		if err != nil {
			t.Fatalf("CountFiles failed: %v", err)
		}

		// Should find same files as absolute path test
		if fileCount != 3 {
			t.Errorf("Expected 3 files, got %d", fileCount)
		}

		t.Logf("Found %d files", fileCount)
	})

	// Test Case 3: Exclude patterns with subdirectories
	t.Run("exclude patterns with subdirectories", func(t *testing.T) {
		cfg := &config.Config{
			Project: config.Project{Root: tmpDir},
			Index:   config.Index{MaxFileSize: 1024 * 1024},
			Include: []string{"**/*.go", "**/*.md"},
			Exclude: []string{"**/src/**"}, // Exclude src directory
		}

		scanner := NewFileScanner(cfg, 100)

		fileCount, _, err := scanner.CountFiles(context.Background(), tmpDir)
		if err != nil {
			t.Fatalf("CountFiles failed: %v", err)
		}

		// Should only find README.md, not the .go files in src/
		if fileCount != 1 {
			t.Errorf("Expected 1 file (README.md), got %d", fileCount)
		}

		t.Logf("Found %d files after excluding src/", fileCount)
	})

	// Test Case 4: Pattern with ** prefix matches files in subdirectories
	t.Run("pattern with double-glob matches subdirectories", func(t *testing.T) {
		cfg := &config.Config{
			Project: config.Project{Root: tmpDir},
			Index:   config.Index{MaxFileSize: 1024 * 1024},
			Include: []string{"**/*.go", "**/*.md"}, // With ** prefix for recursive matching
			Exclude: []string{},
		}

		scanner := NewFileScanner(cfg, 100)

		fileCount, _, err := scanner.CountFiles(context.Background(), tmpDir)
		if err != nil {
			t.Fatalf("CountFiles failed: %v", err)
		}

		// Patterns with ** match recursively across directories
		// So **/*.go matches both src/main.go and src/util.go
		if fileCount != 3 {
			t.Errorf("Expected 3 files (2 .go + 1 .md), got %d", fileCount)
		}

		t.Logf("Found %d files with recursive pattern", fileCount)
	})
}

// TestMCPModeRelativePaths tests that MCP server mode (running from project root)
// correctly indexes files using relative path patterns
func TestMCPModeRelativePaths(t *testing.T) {
	// Create temporary project structure
	tmpDir := t.TempDir()

	// Create typical project structure
	dirs := []string{
		"cmd/server",
		"internal/core",
		"pkg/utils",
		"test/fixtures",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tmpDir, dir)
		err := os.MkdirAll(fullPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create test files in each directory
	testFiles := map[string]string{
		"cmd/server/main.go":      "package main",
		"internal/core/engine.go": "package core",
		"pkg/utils/helpers.go":    "package utils",
		"test/fixtures/sample.js": "console.log('test')",
		"README.md":               "# Project",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Simulate MCP running from project root with default config
	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 1024 * 1024},
		Include: []string{
			"**/*.go", "**/*.js", "**/*.md", // Default patterns with **
		},
		Exclude: []string{
			"**/node_modules/**",
			"**/.git/**",
		},
	}

	scanner := NewFileScanner(cfg, 100)

	fileCount, _, err := scanner.CountFiles(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("CountFiles failed: %v", err)
	}

	// Should find all 5 files (3 .go, 1 .js, 1 .md)
	expectedCount := 5
	if fileCount != expectedCount {
		t.Errorf("Expected %d files, got %d", expectedCount, fileCount)
	}

	t.Logf("MCP mode indexed %d files from project root", fileCount)
}
