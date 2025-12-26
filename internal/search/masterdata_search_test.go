package search_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// TestMasterDataRegexSearch tests the specific regex pattern that failed
// Pattern: \\.(masterData|state|viewManager|keys|width|height)\\s*=
func TestMasterDataRegexSearch(t *testing.T) {
	// Read the test file with various masterData patterns
	testFilePath := "testdata/test_masterdata_search.go"

	// Check if file exists - if not, skip the test
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Skip("Test file testdata/test_masterdata_search.go not found - skipping test")
	}

	code, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Setup test project
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_masterdata_search.go")

	if err := os.WriteFile(filePath, code, 0644); err != nil {
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
	if len(fileIDs) == 0 {
		t.Fatalf("No files indexed")
	}

	// THE TEST: Use the pattern that failed
	// Note: In the user's case, they had double backslashes which suggests
	// they were trying to escape in a shell or another context
	// The actual regex pattern should be: \.(masterData|state|viewManager|keys|width|height)\s*=
	//
	// When passed through CLI, \\ becomes a single \ in the regex
	pattern := `\.(masterData|state|viewManager|keys|width|height)\s*=`

	t.Logf("Testing pattern: %s", pattern)

	results := engine.SearchWithOptions(pattern, fileIDs, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Search found %d matches", len(results))

	if len(results) == 0 {
		t.Error("Expected to find matches for pattern", pattern)
		t.Logf("File content (first 500 chars):\n%s", string(code[:min(500, len(code))]))
	}

	// Analyze results
	matchCount := 0
	for i, result := range results {
		t.Logf("Match %d: Line %d, Context: %s",
			i+1, result.Line, extractFirstLine(result.Context.Lines))

		// Check if this is actually an assignment
		if containsAssignment(result.Context.Lines) {
			matchCount++
		}
	}

	t.Logf("Confirmed assignment matches: %d", matchCount)

	// We expect multiple matches (all the obj.field = value patterns)
	if matchCount < 5 {
		t.Errorf("Expected at least 5 assignment matches, got %d. Pattern may not be working correctly.", matchCount)
	}
}

// extractFirstLine extracts the first non-empty line from context
func extractFirstLine(lines []string) string {
	for _, line := range lines {
		if len(line) > 0 {
			return line
		}
	}
	return ""
}

// containsAssignment checks if any line in the context contains an assignment
func containsAssignment(lines []string) bool {
	for _, line := range lines {
		// Simple check for assignment operators
		if strings.Contains(line, "=") && !strings.Contains(line, "==") && !strings.Contains(line, "!=") {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestMasterDataRegexSearchWithDoubleBackslash tests the EXACT pattern as entered by user
// This simulates what happens when user types \\ in CLI which becomes \\ in their input
func TestMasterDataRegexSearchWithDoubleBackslash(t *testing.T) {
	// This is what the user likely entered: \\.(masterData|state|viewManager|keys|width|height)\\s*=
	// In Go string literal, this would be: "\\.(masterData|state|viewManager|keys|width|height)\\s*="
	// Which evaluates to: \.(masterData|state|viewManager|keys|width|height)\s*=

	code := `package main

func Test() {
	obj.masterData = lines
	this.state = "value"
	view.width = 100
}
`

	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")

	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
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

	// The EXACT pattern from user's failed search
	// When they type: \\.(masterData|state|viewManager|keys|width|height)\\s*=
	// And we receive it as a string, it becomes: \.(masterData|state|viewManager|keys|width|height)\s*=
	userPattern := `\.(masterData|state|viewManager|keys|width|height)\s*=`

	t.Logf("User pattern (as received): %s", userPattern)

	results := engine.SearchWithOptions(userPattern, fileIDs, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Found %d matches for user pattern", len(results))

	// This should find at least 3 matches (masterData, state, width)
	if len(results) < 3 {
		t.Errorf("Expected at least 3 matches, got %d. Pattern may be malformed.", len(results))
		for i, result := range results {
			t.Logf("Match %d: %s", i+1, result.Context.Lines)
		}
	}
}
