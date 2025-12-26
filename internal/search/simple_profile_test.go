package search_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// SimpleProfileTest creates a minimal test for external profiling
func TestSimpleProfileTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping profiling test in short mode")
	}

	tempDir := t.TempDir()

	// Generate test data
	numFiles := 50 // Smaller for focused profiling
	numFunctionsPerFile := 50

	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("profile_test_%d.go", i)
		content := generateTestFile(numFunctionsPerFile)

		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create indexer and index
	cfg := createTestConfig(tempDir)
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	startIndex := time.Now()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}
	t.Logf("Indexed %d files in %v", numFiles, time.Since(startIndex))

	// Create search engine
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Profile the specific failing search pattern
	pattern := "Function[0-9]+"

	// Multiple runs for consistent measurements
	numRuns := 5
	t.Logf("Running %d searches for pattern: %s", numRuns, pattern)

	for run := 0; run < numRuns; run++ {
		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, types.SearchOptions{UseRegex: true})
		duration := time.Since(start)

		t.Logf("Run %d: %d results in %v", run+1, len(results), duration)
	}

	t.Logf("Profiling test completed")
}

// Helper function to generate test file content
func generateTestFile(numFunctions int) string {
	var content string
	content += "package test\n\n"

	for i := 1; i <= numFunctions; i++ {
		content += fmt.Sprintf("// Function%d is a test function\n", i)
		content += fmt.Sprintf("func Function%d(param1 string, param2 int) error {\n", i)
		content += `	if param2 < 0 {
		return fmt.Errorf("invalid param2: %d", param2)
	}

	// Some logic here
	result := param1 + fmt.Sprintf("_%d", param2)
	return nil
}

`
	}

	return content
}
