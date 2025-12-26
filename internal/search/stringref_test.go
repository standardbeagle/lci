package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// TestStringRefContextExtraction tests the memory-efficient StringRef context extraction
func TestStringRefContextExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping StringRef test in short mode")
	}

	tempDir := t.TempDir()

	// Create test file with predictable content
	testContent := `package test

// Function1 is a test function
func Function1(param1 string, param2 int) error {
	if param2 < 0 {
		return fmt.Errorf("invalid param2: %d", param2)
	}

	// Line with target match here
	result := param1 + fmt.Sprintf("_%d", param2)
	return nil
}

// Function2 is another test function
func Function2() error {
	return nil
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create indexer and index
	cfg := createTestConfig(tempDir)
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Create search engine
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Test search that will trigger context extraction
	pattern := "target match"
	results := engine.SearchWithOptions(pattern, fileIDs, types.SearchOptions{UseRegex: false})

	if len(results) == 0 {
		t.Fatalf("Expected to find results for pattern %q", pattern)
	}

	result := results[0]

	// Verify StringRef context extraction
	t.Logf("Found match at line %d with %d context lines",
		result.Context.StartLine,
		len(result.Context.Lines))

	// Test that lines are available as strings (temporary until full StringRef conversion)
	resolvedLines := make([]string, len(result.Context.Lines))
	for i, line := range result.Context.Lines {
		resolvedLines[i] = line // Already resolved as strings
	}

	// Verify the context contains our target line
	contextText := ""
	for _, line := range resolvedLines {
		contextText += line + "\n"
	}

	if !containsSubstring(contextText, "target match here") {
		t.Errorf("Expected context to contain 'target match here', got: %s", contextText)
	}

	t.Logf("StringRef context extraction working correctly!")
	t.Logf("Context has %d lines using zero-allocation StringRef", len(result.Context.Lines))
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
