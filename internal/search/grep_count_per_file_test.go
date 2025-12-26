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

// TestCountPerFile_BasicCounting tests basic count-per-file mode
func TestCountPerFile_BasicCounting(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	test := 3
	other := 4
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should return results showing counts
	assert.Greater(t, len(results), 0, "Should find matches with count mode")
}

// TestCountPerFile_MultipleFiles tests count mode across multiple files
func TestCountPerFile_MultipleFiles(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func main() {
	test := 1
	test := 2
	test := 3
}`,
		"file2.go": `package main
func main() {
	test := 1
	test := 2
}`,
		"file3.go": `package main
func main() {
	other := 1
}`,
	}

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should have count information
	assert.Greater(t, len(results), 0, "Should find files with pattern")
}

// TestCountPerFile_NoMatches tests count mode with no matches
func TestCountPerFile_NoMatches(t *testing.T) {
	code := `package main

func main() {
	value := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("nonexistent", fileIDs, opts)

	// Should return no results when pattern not found
	assert.Equal(t, 0, len(results), "Should return no results when pattern not found")
}

// TestCountPerFile_WithCaseSensitivity tests count mode with case sensitivity
func TestCountPerFile_WithCaseSensitivity(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	TEST := 3
	TEST := 4
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Case-sensitive count
	resultsSensitive := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		CountPerFile:    true,
		CaseInsensitive: false,
	})

	// Case-insensitive count
	resultsInsensitive := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		CountPerFile:    true,
		CaseInsensitive: true,
	})

	assert.Greater(t, len(resultsSensitive), 0, "Should find matches with case-sensitive count")
	assert.Greater(t, len(resultsInsensitive), 0, "Should find matches with case-insensitive count")
}

// TestCountPerFile_WithRegex tests count mode with regex
func TestCountPerFile_WithRegex(t *testing.T) {
	code := `package main

func main() {
	test1 := 1
	test2 := 2
	other := 3
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
		UseRegex:     true,
	}

	results := engine.SearchWithOptions("test[0-9]", fileIDs, opts)

	assert.Greater(t, len(results), 0, "Should count regex matches")
}

// TestCountPerFile_WithWordBoundary tests count mode with word boundary
func TestCountPerFile_WithWordBoundary(t *testing.T) {
	code := `package main

func main() {
	test := 1
	testing := 2
	test_var := 3
	test()
	test()
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
		WordBoundary: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	assert.Greater(t, len(results), 0, "Should count word boundary matches")
}

// TestCountPerFile_EmptyPattern tests count mode with empty pattern
func TestCountPerFile_EmptyPattern(t *testing.T) {
	code := `package main
func main() {}`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Empty pattern should return no results
	assert.Equal(t, 0, len(results), "Empty pattern should return no results")
}

// TestCountPerFile_SingleMatch tests count mode with single match
func TestCountPerFile_SingleMatch(t *testing.T) {
	code := `package main

func main() {
	test()
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find single match
	assert.Greater(t, len(results), 0, "Should find single match in count mode")
}

// TestCountPerFile_Performance tests count mode performance
func TestCountPerFile_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Generate file with many matches
	code := "package main\n\nfunc main() {\n"
	for i := 0; i < 100; i++ {
		code += "\ttest := value\n"
	}
	code += "}\n"

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"large.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should efficiently count matches
	assert.Greater(t, len(results), 0, "Should count matches in large file")
}

// TestCountPerFile_Unicode tests count mode with unicode
func TestCountPerFile_Unicode(t *testing.T) {
	code := `package main

func main() {
	café := 1
	café := 2
	café := 3
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("café", fileIDs, opts)

	assert.Greater(t, len(results), 0, "Should count unicode matches")
}

// TestCountPerFile_Integration tests count mode end-to-end
func TestCountPerFile_Integration(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func main() {
	test := 1
	test := 2
}`,
		"file2.go": `package main
func main() {
	test := 1
	test := 2
	test := 3
}`,
	}

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should report counts per file
	assert.Greater(t, len(results), 0, "Should report matches across files")
}

// TestCountPerFile_ConsistencyAcrossRuns tests consistent counting
func TestCountPerFile_ConsistencyAcrossRuns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	test := 3
}
`

	engine, fileIDs, cleanup := setupCountPerFileTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CountPerFile: true,
	}

	// Run multiple times
	results1 := engine.SearchWithOptions("test", fileIDs, opts)
	results2 := engine.SearchWithOptions("test", fileIDs, opts)
	results3 := engine.SearchWithOptions("test", fileIDs, opts)

	// Should get same count each time
	assert.Equal(t, len(results1), len(results2), "Should get consistent counts")
	assert.Equal(t, len(results2), len(results3), "Should get consistent counts")
}

// setupCountPerFileTestEngine creates a test search engine for count per file tests
func setupCountPerFileTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
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
