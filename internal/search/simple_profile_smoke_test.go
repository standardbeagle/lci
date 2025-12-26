package search_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// TestSimpleProfileSmoke is a lightweight smoke test version of the profiling test
// This runs for the pre-commit hook, while the full version requires -run=TestSimpleProfileTest
// Purpose: Catch obvious regressions without taking 3+ seconds
func TestSimpleProfileSmoke(t *testing.T) {
	tempDir := t.TempDir()

	// Small workload: 5 files × 5 functions instead of 50 × 50
	numFiles := 5
	numFunctionsPerFile := 5

	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("smoke_test_%d.go", i)
		content := generateTestFile(numFunctionsPerFile)

		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create indexer and index
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	startIndex := time.Now()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}
	indexDuration := time.Since(startIndex)

	// Create search engine
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Verify index worked
	if len(fileIDs) != numFiles {
		t.Errorf("Expected %d files indexed, got %d", numFiles, len(fileIDs))
	}

	// Smoke test: Run regex pattern search a couple times
	pattern := "Function[0-9]"

	for run := 0; run < 2; run++ {
		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, types.SearchOptions{UseRegex: true})
		duration := time.Since(start)

		// Should find results
		if len(results) == 0 {
			t.Errorf("Run %d: Expected results for pattern %s, got none", run+1, pattern)
		}

		// Should be reasonably fast (smoke test, so generous limit)
		if duration > 500*time.Millisecond {
			t.Logf("Run %d: Warning - search took %v (may indicate regression)", run+1, duration)
		}

		t.Logf("Run %d: %d results in %v", run+1, len(results), duration)
	}

	// Overall performance check
	if indexDuration > 5*time.Second {
		t.Logf("Warning: Indexing took %v (may indicate regression)", indexDuration)
	}
}

// TestSimpleProfileSmoke_LiteralSearch tests simple literal search without regex overhead
func TestSimpleProfileSmoke_LiteralSearch(t *testing.T) {
	tempDir := t.TempDir()

	// Generate 3 small files
	for i := 0; i < 3; i++ {
		filename := fmt.Sprintf("literal_%d.go", i)
		content := `package main

func TestOne() { }
func TestTwo() { }
func TestThree() { }
`
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Smoke test: Simple literal search
	start := time.Now()
	results := engine.SearchWithOptions("TestOne", fileIDs, types.SearchOptions{})
	duration := time.Since(start)

	if len(results) == 0 {
		t.Errorf("Expected results for literal search, got none")
	}

	if duration > 100*time.Millisecond {
		t.Logf("Warning: Literal search took %v (may indicate regression)", duration)
	}

	t.Logf("Literal search: %d results in %v", len(results), duration)
}

// TestSimpleProfileSmoke_AllFeatures tests that all search features work in smoke test
func TestSimpleProfileSmoke_AllFeatures(t *testing.T) {
	tempDir := t.TempDir()

	code := `package main

func TestFunc() { }
func TestHelper() { }
func HelperFunc() { }
// Test word boundary
`

	filename := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filename, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	tests := []struct {
		name    string
		pattern string
		opts    types.SearchOptions
	}{
		{
			name:    "literal_search",
			pattern: "TestFunc",
			opts:    types.SearchOptions{},
		},
		{
			name:    "word_boundary",
			pattern: "Test",
			opts:    types.SearchOptions{WordBoundary: true},
		},
		{
			name:    "files_only",
			pattern: "func",
			opts:    types.SearchOptions{FilesOnly: true},
		},
		{
			name:    "invert_match",
			pattern: "XXX",
			opts:    types.SearchOptions{InvertMatch: true},
		},
		{
			name:    "max_count_per_file",
			pattern: "Test",
			opts:    types.SearchOptions{MaxCountPerFile: 1},
		},
		{
			name:    "patterns",
			pattern: "",
			opts:    types.SearchOptions{Patterns: []string{"TestFunc", "HelperFunc"}},
		},
	}

	for _, tt := range tests {
		results := engine.SearchWithOptions(tt.pattern, fileIDs, tt.opts)
		if results == nil {
			t.Errorf("%s: expected results, got none", tt.name)
		}
		t.Logf("%s: %d results", tt.name, len(results))
	}
}
