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

// TestPatterns_TwoPatterns tests basic multiple pattern matching
func TestPatterns_TwoPatterns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	helper := 2
	value := 3
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "helper"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find lines matching either pattern
	assert.Greater(t, len(results), 0, "Should find matches for multiple patterns")
}

// TestPatterns_ThreePatterns tests three pattern matching
func TestPatterns_ThreePatterns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	helper := 2
	value := 3
	other := 4
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "helper", "value"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find lines matching any of the patterns
	assert.Greater(t, len(results), 0, "Should find matches for three patterns")
}

// TestPatterns_OverlapPatterns tests overlapping pattern matching
func TestPatterns_OverlapPatterns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	testing := 2
	test_value := 3
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "testing"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find all lines with either pattern
	assert.Greater(t, len(results), 0, "Should find lines with overlapping patterns")
}

// TestPatterns_NoMatches tests patterns with no matches
func TestPatterns_NoMatches(t *testing.T) {
	code := `package main

func main() {
	value := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"nonexistent1", "nonexistent2"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should return no results when patterns not found
	assert.Equal(t, 0, len(results), "Should return no results when patterns not found")
}

// TestPatterns_WithCaseSensitivity tests patterns with case sensitivity
func TestPatterns_WithCaseSensitivity(t *testing.T) {
	code := `package main

func main() {
	Test := 1
	test := 2
	HELPER := 3
	helper := 4
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Case-sensitive
	resultsSensitive := engine.SearchWithOptions("", fileIDs, types.SearchOptions{
		Patterns:        []string{"test", "helper"},
		CaseInsensitive: false,
	})

	// Case-insensitive
	resultsInsensitive := engine.SearchWithOptions("", fileIDs, types.SearchOptions{
		Patterns:        []string{"test", "helper"},
		CaseInsensitive: true,
	})

	assert.Greater(t, len(resultsSensitive), 0, "Should find case-sensitive patterns")
	assert.Greater(t, len(resultsInsensitive), len(resultsSensitive),
		"Case-insensitive should find more matches. Sensitive: %d, Insensitive: %d",
		len(resultsSensitive), len(resultsInsensitive))
}

// TestPatterns_WithRegex tests patterns with regex mode
func TestPatterns_WithRegex(t *testing.T) {
	code := `package main

func main() {
	test1 := 1
	test2 := 2
	helper3 := 3
	helper4 := 4
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test[0-9]", "helper[0-9]"},
		UseRegex: true,
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find regex matches
	assert.Greater(t, len(results), 0, "Should find regex pattern matches")
}

// TestPatterns_WithWordBoundary tests patterns with word boundary
func TestPatterns_WithWordBoundary(t *testing.T) {
	code := `package main

func main() {
	test := 1
	testing := 2
	test_value := 3
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns:     []string{"test"},
		WordBoundary: true,
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find word boundary matches only
	for _, result := range results {
		assert.NotContains(t, result.Match, "testing", "Should respect word boundaries")
	}
}

// TestPatterns_MultipleFiles tests patterns across multiple files
func TestPatterns_MultipleFiles(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func main() {
	test := 1
	other := 2
}`,
		"file2.go": `package main
func main() {
	helper := 1
	value := 2
}`,
	}

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "helper"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find patterns across files
	assert.Greater(t, len(results), 0, "Should find patterns across multiple files")
}

// TestPatterns_EmptyPatternList tests empty pattern list fallback
func TestPatterns_EmptyPatternList(t *testing.T) {
	code := `package main
func main() {}`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{},
	}

	// Empty patterns with empty pattern argument
	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should return no results
	assert.Equal(t, 0, len(results), "Empty patterns should return no results")
}

// TestPatterns_LongPatternList tests many patterns
func TestPatterns_LongPatternList(t *testing.T) {
	code := `package main

func main() {
	alpha := 1
	beta := 2
	gamma := 3
	delta := 4
	epsilon := 5
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"alpha", "beta", "gamma", "delta", "epsilon"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find matches for all patterns
	assert.GreaterOrEqual(t, len(results), 5, "Should find matches for all patterns")
}

// TestPatterns_SpecialCharacters tests patterns with special characters
func TestPatterns_SpecialCharacters(t *testing.T) {
	code := `package main

func main() {
	value := "test[0-9]"
	pattern := ".*\.go"
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "pattern"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find lines containing patterns
	assert.Greater(t, len(results), 0, "Should find special character patterns")
}

// TestPatterns_Unicode tests patterns with unicode
func TestPatterns_Unicode(t *testing.T) {
	code := `package main

func main() {
	café := 1
	café_name := 2
	λ := 3
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"café", "λ"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find unicode patterns
	assert.Greater(t, len(results), 0, "Should find unicode patterns")
}

// TestPatterns_Performance tests patterns performance
func TestPatterns_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Generate file with many lines
	code := "package main\n\nfunc main() {\n"
	for i := 0; i < 100; i++ {
		code += "\tvar" + string(rune(i)) + " := value\n"
	}
	code += "}\n"

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"large.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"var0", "var10", "var50", "var99"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should efficiently find multiple patterns
	assert.Greater(t, len(results), 0, "Should find patterns in large file")
}

// TestPatterns_Integration tests patterns end-to-end
func TestPatterns_Integration(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {}
func helper() {}`,
		"file2.go": `package main
func value() {}
func other() {}`,
	}

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "helper", "value"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should find all patterns across files
	assert.Greater(t, len(results), 0, "Should find all patterns across files")
}

// TestPatterns_ConsistencyAcrossRuns tests consistent pattern matching
func TestPatterns_ConsistencyAcrossRuns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	helper := 2
	value := 3
}
`

	engine, fileIDs, cleanup := setupPatternsTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		Patterns: []string{"test", "helper"},
	}

	// Run multiple times
	results1 := engine.SearchWithOptions("", fileIDs, opts)
	results2 := engine.SearchWithOptions("", fileIDs, opts)
	results3 := engine.SearchWithOptions("", fileIDs, opts)

	// Should get same count each time
	assert.Equal(t, len(results1), len(results2), "Should get consistent results")
	assert.Equal(t, len(results2), len(results3), "Should get consistent results")
}

// setupPatternsTestEngine creates a test search engine for patterns tests
func setupPatternsTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
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
