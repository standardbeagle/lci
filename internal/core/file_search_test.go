package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// TestFileSearchEngine_Basic tests the file search engine basic.
func TestFileSearchEngine_Basic(t *testing.T) {
	engine := NewFileSearchEngine()

	// Test indexing files
	testFiles := map[types.FileID]string{
		1: "internal/core/file_search.go",
		2: "internal/core/file_search_test.go",
		3: "internal/mcp/handlers.go",
		4: "internal/indexing/goroutine_index.go",
		5: "cmd/lci/main.go",
		6: "README.md",
		7: "internal/tui/view_main.go",
		8: "internal/tui/view_search.go",
	}

	// Index all files
	for fileID, path := range testFiles {
		engine.IndexFile(fileID, path)
	}

	// Test glob pattern search
	t.Run("GlobSearch", func(t *testing.T) {
		options := types.FileSearchOptions{
			Pattern:    "internal/core/*.go",
			Type:       "glob",
			MaxResults: 10,
		}

		results, err := engine.SearchFiles(options)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) < 2 {
			t.Errorf("Expected at least 2 results, got %d", len(results))
		}

		// Verify results contain expected files
		foundFiles := make(map[string]bool)
		for _, result := range results {
			foundFiles[result.Path] = true
		}

		expectedFiles := []string{
			"internal/core/file_search.go",
			"internal/core/file_search_test.go",
		}

		for _, expectedFile := range expectedFiles {
			if !foundFiles[expectedFile] {
				t.Errorf("Expected file %s not found in results", expectedFile)
			}
		}
	})

	// Test wildcard pattern search
	t.Run("WildcardSearch", func(t *testing.T) {
		options := types.FileSearchOptions{
			Pattern:    "internal/tui/*view*.go",
			Type:       "glob",
			MaxResults: 10,
		}

		results, err := engine.SearchFiles(options)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		// Verify results contain expected files
		foundFiles := make(map[string]bool)
		for _, result := range results {
			foundFiles[result.Path] = true
		}

		expectedFiles := []string{
			"internal/tui/view_main.go",
			"internal/tui/view_search.go",
		}

		for _, expectedFile := range expectedFiles {
			if !foundFiles[expectedFile] {
				t.Errorf("Expected file %s not found in results", expectedFile)
			}
		}
	})

	// Test regex search
	t.Run("RegexSearch", func(t *testing.T) {
		options := types.FileSearchOptions{
			Pattern:    `internal/.*\.go$`,
			Type:       "regex",
			MaxResults: 10,
		}

		results, err := engine.SearchFiles(options)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find all Go files in internal/
		if len(results) < 6 {
			t.Errorf("Expected at least 6 results, got %d", len(results))
		}

		// Verify all results are Go files in internal/
		for _, result := range results {
			if result.Extension != ".go" {
				t.Errorf("Expected .go extension, got %s for %s", result.Extension, result.Path)
			}
			if !strings.HasPrefix(result.Path, "internal/") {
				t.Errorf("Expected path to start with 'internal/', got %s", result.Path)
			}
		}
	})

	// Test extension filtering
	t.Run("ExtensionFilter", func(t *testing.T) {
		options := types.FileSearchOptions{
			Pattern:    "*",
			Type:       "glob",
			Extensions: []string{".md"},
			MaxResults: 10,
		}

		results, err := engine.SearchFiles(options)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		if len(results) > 0 && results[0].Path != "README.md" {
			t.Errorf("Expected README.md, got %s", results[0].Path)
		}
	})

	// Test exclusion patterns
	t.Run("ExclusionFilter", func(t *testing.T) {
		options := types.FileSearchOptions{
			Pattern:    "**/*.go",
			Type:       "glob",
			Exclude:    []string{"**/test*.go", "**/main.go"},
			MaxResults: 10,
		}

		results, err := engine.SearchFiles(options)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Verify no test files or main.go in results
		for _, result := range results {
			if strings.Contains(result.Path, "test") {
				t.Errorf("Test file should be excluded: %s", result.Path)
			}
			if strings.Contains(result.Path, "main.go") {
				t.Errorf("main.go should be excluded: %s", result.Path)
			}
		}
	})

	// Test statistics
	t.Run("Statistics", func(t *testing.T) {
		stats := engine.GetStats()

		totalFiles := stats["total_files"].(int)
		if totalFiles != len(testFiles) {
			t.Errorf("Expected %d total files, got %d", len(testFiles), totalFiles)
		}

		extCounts := stats["extension_counts"].(map[string]int)
		if extCounts[".go"] != 7 {
			t.Errorf("Expected 7 .go files, got %d", extCounts[".go"])
		}

		if extCounts[".md"] != 1 {
			t.Errorf("Expected 1 .md file, got %d", extCounts[".md"])
		}
	})
}

// TestFileSearchEngine_Performance tests the file search engine performance.
func TestFileSearchEngine_Performance(t *testing.T) {
	engine := NewFileSearchEngine()

	// Create a larger test set
	for i := types.FileID(1); i <= 1000; i++ {
		path := fmt.Sprintf("internal/package%d/file%d.go", i%10, i)
		engine.IndexFile(i, path)
	}

	// Test search performance
	options := types.FileSearchOptions{
		Pattern:    "internal/package5/*.go",
		Type:       "glob",
		MaxResults: 100,
	}

	start := time.Now()
	results, err := engine.SearchFiles(options)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 100 {
		t.Errorf("Expected 100 results, got %d", len(results))
	}

	// Verify search is fast (<5ms as per requirements)
	// Retry with backoff to handle timing variance
	maxRetries := 2
	for attempt := 0; attempt < maxRetries; attempt++ {
		if duration <= 5*time.Millisecond {
			break // Success
		}
		if attempt < maxRetries-1 {
			t.Logf("Search duration attempt %d/%d: %v (threshold: 5ms), retrying...", attempt+1, maxRetries, duration)
			time.Sleep(100 * time.Millisecond)
			// Re-run search
			start = time.Now()
			results, err = engine.SearchFiles(options)
			duration = time.Since(start)
			continue
		}
		t.Errorf("Search took %v, expected < 5ms", duration)
	}

	t.Logf("Search completed in %v with %d results", duration, len(results))
}
