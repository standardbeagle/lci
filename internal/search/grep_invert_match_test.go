package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// TestInvertMatch_BasicNegation tests basic inverted match
func TestInvertMatch_BasicNegation(t *testing.T) {
	code := `package main

func main() {
	test := 1
	other := 2
	value := 3
}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Normal search (should match "test" line)
	normalResults := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		InvertMatch: false,
	})

	// Inverted search (should match lines without "test")
	invertedResults := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		InvertMatch: true,
	})

	assert.Greater(t, len(normalResults), 0, "Should find lines matching pattern")
	assert.Greater(t, len(invertedResults), 0, "Should find lines not matching pattern")

	// Verify inverted results don't contain the pattern
	for _, result := range invertedResults {
		assert.NotContains(t, result.Match, "test", "Inverted match should return lines without pattern")
	}
}

// TestInvertMatch_AllLines tests inverted match returns all non-matching lines
func TestInvertMatch_AllLines(t *testing.T) {
	code := `package main

func main() {
	line1 := 1
	line2 := 2
	line3 := 3
}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should return lines without "test" pattern
	assert.Greater(t, len(results), 0, "Should find non-matching lines")
}

// TestInvertMatch_WithCaseSensitivity tests inverted match with case sensitivity
func TestInvertMatch_WithCaseSensitivity(t *testing.T) {
	code := `package main

func Test() {}
func test() {}
func other() {}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Case-sensitive inversion
	resultsSensitive := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		InvertMatch:     true,
		CaseInsensitive: false,
	})

	// Case-insensitive inversion
	resultsInsensitive := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		InvertMatch:     true,
		CaseInsensitive: true,
	})

	// Case-sensitive should return more lines (includes "Test()")
	assert.Greater(t, len(resultsSensitive), len(resultsInsensitive),
		"Case-sensitive inversion should return more lines. Sensitive: %d, Insensitive: %d",
		len(resultsSensitive), len(resultsInsensitive))
}

// TestInvertMatch_WithRegex tests inverted match with regex
func TestInvertMatch_WithRegex(t *testing.T) {
	code := `package main

func test1() {}
func test2() {}
func helper() {}
func other() {}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
		UseRegex:    true,
	}

	results := engine.SearchWithOptions("test[0-9]", fileIDs, opts)

	// Should return lines not matching regex
	for _, result := range results {
		assert.NotRegexp(t, "test[0-9]", result.Match, "Should return non-matching lines")
	}
}

// TestInvertMatch_MultipleFiles tests inverted match across files
func TestInvertMatch_MultipleFiles(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {}
func other() {}`,
		"file2.go": `package main
func helper() {}`,
	}

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find non-matching lines from both files
	assert.Greater(t, len(results), 0, "Should find non-matching lines across files")
}

// TestInvertMatch_EmptyPattern tests inverted match with empty pattern
func TestInvertMatch_EmptyPattern(t *testing.T) {
	code := `package main
func main() {}`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Empty pattern should return no results
	assert.Equal(t, 0, len(results), "Empty pattern should return no results")
}

// TestInvertMatch_NoMatches tests inverted match when pattern doesn't exist
func TestInvertMatch_NoMatches(t *testing.T) {
	code := `package main

func main() {
	value := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	// Search for pattern that doesn't exist
	results := engine.SearchWithOptions("nonexistent", fileIDs, opts)

	// Should return all lines (all are non-matching when pattern not found)
	assert.Greater(t, len(results), 0, "Should return all lines when pattern not found")
}

// TestInvertMatch_WithWordBoundary tests inverted match with word boundary
func TestInvertMatch_WithWordBoundary(t *testing.T) {
	code := `package main

func main() {
	test := 1
	testing := 2
	test_var := 3
}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch:  true,
		WordBoundary: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should return lines that don't have "test" as a word
	for _, result := range results {
		assert.NotContains(t, result.Match, "test ", "Should exclude word boundary matches")
	}
}

// TestInvertMatch_WithMaxResults tests inverted match respects max results
func TestInvertMatch_WithMaxResults(t *testing.T) {
	code := `package main

func main() {
	line1 := 1
	line2 := 2
	line3 := 3
	line4 := 4
	line5 := 5
}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
		MaxResults:  2,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit inverted results
	assert.LessOrEqual(t, len(results), 2, "Should respect max results with inverted match")
}

// TestInvertMatch_Performance tests inverted match performance
func TestInvertMatch_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Generate file with many lines
	code := "package main\n\nfunc main() {\n"
	for i := 0; i < 100; i++ {
		code += "\tline" + string(rune(i)) + " := value\n"
	}
	code += "}\n"

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"large.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	results := engine.SearchWithOptions("nonexistent", fileIDs, opts)

	// Should handle large files efficiently
	assert.Greater(t, len(results), 0, "Should find matches in large file")
}

// TestInvertMatch_Unicode tests inverted match with unicode
func TestInvertMatch_Unicode(t *testing.T) {
	code := `package main

func café() {}
func helper() {}
func λ() {}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	results := engine.SearchWithOptions("café", fileIDs, opts)

	// Should return non-cafe lines
	for _, result := range results {
		assert.NotContains(t, result.Match, "café", "Should exclude unicode matches")
	}
}

// TestInvertMatch_Integration tests inverted match end-to-end
func TestInvertMatch_Integration(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {}
func other() {}
func value() {}`,
		"file2.go": `package main
func test() {}
func helper() {}`,
	}

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, files)
	defer cleanup()

	// Find lines without "test"
	normalResults := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		InvertMatch: false,
	})
	invertedResults := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		InvertMatch: true,
	})

	// Verify integrity
	assert.Greater(t, len(normalResults), 0, "Should find matching lines")
	assert.Greater(t, len(invertedResults), 0, "Should find non-matching lines")
}

// TestInvertMatch_ConsistencyAcrossRuns tests consistent inversion
func TestInvertMatch_ConsistencyAcrossRuns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	other := 2
	value := 3
}
`

	engine, fileIDs, cleanup := setupInvertMatchTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch: true,
	}

	// Run multiple times
	results1 := engine.SearchWithOptions("test", fileIDs, opts)
	results2 := engine.SearchWithOptions("test", fileIDs, opts)
	results3 := engine.SearchWithOptions("test", fileIDs, opts)

	// Should get same count each time
	assert.Equal(t, len(results1), len(results2), "Should get consistent results")
	assert.Equal(t, len(results2), len(results3), "Should get consistent results")
}

// setupInvertMatchTestEngine creates a test search engine for invert match tests
func setupInvertMatchTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
	tempDir := t.TempDir()

	// Write test files
	for filename, code := range files {
		testFilePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(testFilePath, []byte(code), 0644)
		require.NoError(t, err, "Failed to write test file %s", filename)
	}

	// Create config for indexing
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

	// Create and run indexer
	gi := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	err := gi.IndexDirectory(ctx, tempDir)
	require.NoError(t, err, "Failed to index directory")

	// Get all file IDs from the index
	allFiles := gi.GetAllFileIDs()
	require.Greater(t, len(allFiles), 0, "Should have indexed at least one file")

	// Create search engine
	engine := search.NewEngine(gi)
	require.NotNil(t, engine, "Search engine should not be nil")

	return engine, allFiles, func() {
		// Cleanup if needed
	}
}
