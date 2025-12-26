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

// TestTrigramVsLinearScanning compares trigram index usage vs linear scanning
func TestTrigramVsLinearScanning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping trigram test in short mode")
	}

	tempDir := t.TempDir()

	// Create larger test file to highlight the difference
	numFiles := 20
	numFunctionsPerFile := 100

	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("trigram_test_%d.go", i)
		content := generateTrigramTestFile(numFunctionsPerFile)

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

	// Test 1: Literal search (should use trigram index when capped)
	t.Run("LiteralSearch_WithCap", func(t *testing.T) {
		pattern := "Function50" // Literal pattern, 3+ chars

		// Use MaxResults to enable trigram fast path
		options := types.SearchOptions{
			MaxResults: 100, // This enables trigram fast path
			UseRegex:   false,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Literal search '%s' with cap: %d results in %v", pattern, len(results), duration)

		// This should be fast because it uses trigram index
		if duration > 50*time.Millisecond {
			t.Logf("WARNING: Literal search with cap took %v (expected <50ms for trigram index)", duration)
		}
	})

	// Test 2: Literal search without cap (falls back to linear scanning)
	t.Run("LiteralSearch_NoCap", func(t *testing.T) {
		pattern := "Function50" // Literal pattern, 3+ chars

		// No MaxResults = no cap = falls back to linear scanning
		options := types.SearchOptions{
			MaxResults: 0, // No cap = linear scanning
			UseRegex:   false,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Literal search '%s' no cap: %d results in %v", pattern, len(results), duration)

		// This should be slower due to linear scanning
		if duration < 10*time.Millisecond {
			t.Logf("Unexpected: Linear scan completed in %v (might still be using trigram)", duration)
		}
	})

	// Test 3: Regex search (always linear scanning)
	t.Run("RegexSearch", func(t *testing.T) {
		pattern := "Function[0-9]+" // Regex pattern

		options := types.SearchOptions{
			MaxResults: 100,
			UseRegex:   true,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Regex search '%s': %d results in %v", pattern, len(results), duration)

		// Regex will be slower due to linear scanning
		if duration > 100*time.Millisecond {
			t.Logf("Regex search took %v (expected to be slower than trigram)", duration)
		}
	})

	// Test 4: Short pattern (no trigram index available)
	t.Run("ShortPattern", func(t *testing.T) {
		pattern := "f1" // 2 chars = no trigram

		options := types.SearchOptions{
			MaxResults: 100,
			UseRegex:   false,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Short pattern '%s': %d results in %v", pattern, len(results), duration)

		// Short patterns can't use trigram index
		if len(results) > 0 {
			t.Logf("Short pattern found %d matches (trigram not available for <3 chars)", len(results))
		}
	})

	// Test 5: Case-insensitive search
	t.Run("CaseInsensitive", func(t *testing.T) {
		pattern := "function50" // Lowercase

		options := types.SearchOptions{
			MaxResults:      100,
			UseRegex:        false,
			CaseInsensitive: true,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Case-insensitive search '%s': %d results in %v", pattern, len(results), duration)
	})
}

func generateTrigramTestFile(numFunctions int) string {
	var content string
	content += "package test\n\n"

	for i := 1; i <= numFunctions; i++ {
		content += fmt.Sprintf("// Function%d is a test function\n", i)
		content += fmt.Sprintf("func Function%d(param1 string, param2 int) error {\n", i)
		content += fmt.Sprintf("    // Logic for Function%d\n", i)
		content += "    if param2 < 0 {\n"
		content += "        return fmt.Errorf(\"invalid param2: %d\", param2)\n"
		content += "    }\n"
		content += fmt.Sprintf("    result := fmt.Sprintf(\"Function%d_%%s\", param1)\n", i)
		content += "    return result\n"
		content += "}\n\n"
	}

	return content
}
