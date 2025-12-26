package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
)

// Test 1: Completely empty directory (no files at all)
func TestCompletelyEmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "empty-test",
		},
	}

	idx := NewMasterIndex(cfg)

	// Index the empty directory
	ctx := context.Background()
	start := time.Now()
	err = idx.IndexDirectory(ctx, tmpDir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("IndexDirectory failed on empty directory: %v", err)
	}

	// Check if indexing is complete
	err = idx.CheckIndexingComplete()
	if err != nil {
		t.Errorf("CheckIndexingComplete failed after indexing empty dir: %v", err)
	}

	// Verify 0 files
	if count := idx.GetFileCount(); count != 0 {
		t.Errorf("Expected 0 files, got %d", count)
	}

	t.Logf("✓ Empty directory: indexed in %v, CheckIndexingComplete returned nil", elapsed)
}

// Test 2: Directory with files but all filtered out by config
func TestAllFilesFilteredOut(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-filtered-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	files := []string{"test.go", "main.go", "utils.go"}
	for _, filename := range files {
		content := fmt.Sprintf("package test\nfunc %s() {}\n", filename)
		if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Config that filters out all files (max file count = 0)
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "filtered-test",
		},
		Index: config.Index{
			MaxFileCount: 0, // Filter out all files
		},
	}

	idx := NewMasterIndex(cfg)

	// Index with restrictive config
	ctx := context.Background()
	start := time.Now()
	err = idx.IndexDirectory(ctx, tmpDir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("IndexDirectory failed with restrictive config: %v", err)
	}

	// Check if indexing is complete
	err = idx.CheckIndexingComplete()
	if err != nil {
		t.Errorf("CheckIndexingComplete failed after indexing with filters: %v", err)
	}

	// Verify 0 files (all filtered out)
	if count := idx.GetFileCount(); count != 0 {
		t.Errorf("Expected 0 files (all filtered), got %d", count)
	}

	t.Logf("✓ Filtered directory: indexed in %v, CheckIndexingComplete returned nil", elapsed)
}

// Test 3: Directory with only non-indexable files (like .md, .txt)
func TestOnlyNonIndexableFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nonindexable-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create only non-code files
	files := map[string]string{
		"README.md": "# Test Project",
		"notes.txt": "Some notes",
		"data.json": `{"test": true}`,
	}
	for filename, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "nonindexable-test",
		},
	}

	idx := NewMasterIndex(cfg)

	// Index directory with only non-code files
	ctx := context.Background()
	start := time.Now()
	err = idx.IndexDirectory(ctx, tmpDir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("IndexDirectory failed with non-indexable files: %v", err)
	}

	// Check if indexing is complete
	err = idx.CheckIndexingComplete()
	if err != nil {
		t.Errorf("CheckIndexingComplete failed after indexing non-code files: %v", err)
	}

	// Verify 0 files (none are indexable code files)
	if count := idx.GetFileCount(); count != 0 {
		t.Errorf("Expected 0 files (no code files), got %d", count)
	}

	t.Logf("✓ Non-indexable files: indexed in %v, CheckIndexingComplete returned nil", elapsed)
}

// Test 4: Directory with files excluded by auto-exclusion patterns
func TestAutoExcludedFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-excluded-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create node_modules and vendor directories (auto-excluded)
	nodeModules := filepath.Join(tmpDir, "node_modules")
	vendor := filepath.Join(tmpDir, "vendor")
	os.Mkdir(nodeModules, 0755)
	os.Mkdir(vendor, 0755)

	// Create files in excluded directories
	os.WriteFile(filepath.Join(nodeModules, "lib.js"), []byte("module.exports = {}"), 0644)
	os.WriteFile(filepath.Join(vendor, "lib.go"), []byte("package vendor"), 0644)

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "excluded-test",
		},
	}

	idx := NewMasterIndex(cfg)

	// Index directory
	ctx := context.Background()
	start := time.Now()
	err = idx.IndexDirectory(ctx, tmpDir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Check if indexing is complete
	err = idx.CheckIndexingComplete()
	if err != nil {
		t.Errorf("CheckIndexingComplete failed: %v", err)
	}

	// Verify 0 files (all excluded)
	if count := idx.GetFileCount(); count != 0 {
		t.Errorf("Expected 0 files (all excluded), got %d", count)
	}

	t.Logf("✓ Auto-excluded files: indexed in %v, CheckIndexingComplete returned nil", elapsed)
}

// Test 5: Directory with very small max file size filter
func TestMaxFileSizeFilter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-filesize-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file larger than filter
	largeFile := filepath.Join(tmpDir, "large.go")
	content := "package test\n"
	for i := 0; i < 1000; i++ {
		content += fmt.Sprintf("func Function%d() { }\n", i)
	}
	if err := os.WriteFile(largeFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Config with very small max file size (1 byte - filters out everything)
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "filesize-test",
		},
		Index: config.Index{
			MaxFileSize: 1, // 1 byte - filters out all real files
		},
	}

	idx := NewMasterIndex(cfg)

	// Index with size filter
	ctx := context.Background()
	start := time.Now()
	err = idx.IndexDirectory(ctx, tmpDir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("IndexDirectory failed with size filter: %v", err)
	}

	// Check if indexing is complete
	err = idx.CheckIndexingComplete()
	if err != nil {
		t.Errorf("CheckIndexingComplete failed: %v", err)
	}

	// Verify 0 files (file too large)
	if count := idx.GetFileCount(); count != 0 {
		t.Errorf("Expected 0 files (size filtered), got %d", count)
	}

	t.Logf("✓ Size filtered: indexed in %v, CheckIndexingComplete returned nil", elapsed)
}
