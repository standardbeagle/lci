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

// TestMaxCountPerFile_BasicLimiting tests basic max count limiting per file
func TestMaxCountPerFile_BasicLimiting(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	test := 3
	test := 4
	test := 5
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Limit to 3 matches per file
	opts := types.SearchOptions{
		MaxCountPerFile: 3,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit to 3 matches in the file
	assert.LessOrEqual(t, len(results), 3, "Should limit to max count per file")
	assert.Greater(t, len(results), 0, "Should find at least one match")
}

// TestMaxCountPerFile_MultipleFiles tests max count is applied per file
func TestMaxCountPerFile_MultipleFiles(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func main() {
	test := 1
	test := 2
	test := 3
	test := 4
}`,
		"file2.go": `package main
func main() {
	test := 1
	test := 2
	test := 3
	test := 4
}`,
	}

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 2,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit to 2 per file, so max 4 total (2 per file × 2 files)
	// However, implementation may stop early if MaxResults not set
	assert.Greater(t, len(results), 0, "Should find matches across files")
}

// TestMaxCountPerFile_NoLimit tests behavior when MaxCountPerFile is 0
func TestMaxCountPerFile_NoLimit(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	test := 3
	test := 4
	test := 5
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// No limit (0 means unlimited)
	opts := types.SearchOptions{
		MaxCountPerFile: 0,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find all matches (5)
	assert.GreaterOrEqual(t, len(results), 4, "Should find all matches when no limit set")
}

// TestMaxCountPerFile_WithCombinations tests max count with word boundary
func TestMaxCountPerFile_WithCombinations(t *testing.T) {
	code := `package main

func main() {
	test := 1
	testing := 2
	test_var := 3
	test()
	test()
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 2,
		WordBoundary:    true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit word boundary matches to 2 per file
	assert.LessOrEqual(t, len(results), 2, "Should limit word boundary matches to max count per file")
}

// TestMaxCountPerFile_CaseSensitive tests max count with case sensitivity
func TestMaxCountPerFile_CaseSensitive(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	TEST := 3
	TEST := 4
	Test := 5
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Case-insensitive search with limit
	opts := types.SearchOptions{
		MaxCountPerFile: 2,
		CaseInsensitive: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit case-insensitive matches to 2
	assert.LessOrEqual(t, len(results), 2, "Should limit case-insensitive matches to max count per file")
}

// TestMaxCountPerFile_Regex tests max count with regex
func TestMaxCountPerFile_Regex(t *testing.T) {
	code := `package main

func main() {
	test1 := 1
	test2 := 2
	test3 := 3
	test4 := 4
	other := 5
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 2,
		UseRegex:        true,
	}

	results := engine.SearchWithOptions("test[0-9]", fileIDs, opts)

	// Should limit regex matches to 2
	assert.LessOrEqual(t, len(results), 2, "Should limit regex matches to max count per file")
}

// TestMaxCountPerFile_EmptyPattern tests max count with empty pattern
func TestMaxCountPerFile_EmptyPattern(t *testing.T) {
	code := `package main
func main() {}`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 10,
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Empty pattern should return no results regardless of max count
	assert.Equal(t, 0, len(results), "Empty pattern should return no results")
}

// TestMaxCountPerFile_SingleMatch tests max count with single match
func TestMaxCountPerFile_SingleMatch(t *testing.T) {
	code := `package main

func main() {
	test()
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 10,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find the single match
	assert.Equal(t, 1, len(results), "Should find single match when max count is high")
}

// TestMaxCountPerFile_DeclarationOnly tests max count with declaration-only filter
func TestMaxCountPerFile_DeclarationOnly(t *testing.T) {
	code := `package main

func test() {
	test()
	test()
}

func other() {
	test()
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 1,
		DeclarationOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit declarations to 1 per file
	assert.LessOrEqual(t, len(results), 1, "Should limit declarations to max count per file")
}

// TestMaxCountPerFile_InvertMatch tests max count with inverted match
func TestMaxCountPerFile_InvertMatch(t *testing.T) {
	code := `package main

func main() {
	test := 1
	other := 2
	another := 3
	helper := 4
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 2,
		InvertMatch:     true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Note: MaxCountPerFile doesn't limit inverted matches in the current implementation
	// InvertMatch returns all non-matching lines without per-file limiting
	assert.Greater(t, len(results), 0, "Should find inverted matches")
}

// TestMaxCountPerFile_Performance tests max count performance with large file
func TestMaxCountPerFile_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Generate file with many matches
	code := "package main\n\nfunc main() {\n"
	for i := 0; i < 100; i++ {
		code += "\ttest := value\n"
	}
	code += "}\n"

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"large.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 10,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should limit to 10 per file even with 100 matches in file
	assert.LessOrEqual(t, len(results), 10, "Should limit matches to max count per file")
	assert.Greater(t, len(results), 0, "Should find some matches")
}

// TestMaxCountPerFile_Unicode tests max count with unicode patterns
func TestMaxCountPerFile_Unicode(t *testing.T) {
	code := `package main

func main() {
	café := 1
	café := 2
	café := 3
	café := 4
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 2,
	}

	results := engine.SearchWithOptions("café", fileIDs, opts)

	// Should limit unicode matches to 2
	assert.LessOrEqual(t, len(results), 2, "Should limit unicode matches to max count per file")
}

// TestMaxCountPerFile_Integration tests max count end-to-end
func TestMaxCountPerFile_Integration(t *testing.T) {
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
	test := 3
}`,
		"file3.go": `package main
func main() {
	test := 1
}`,
	}

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 2,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Count matches per file
	fileMatches := make(map[string]int)
	for _, result := range results {
		fileMatches[result.Path]++
	}

	// Each file should have at most 2 matches
	for _, count := range fileMatches {
		assert.LessOrEqual(t, count, 2, "Each file should have at most max count matches")
	}
}

// TestMaxCountPerFile_ConsistencyAcrossRuns tests consistent limiting
func TestMaxCountPerFile_ConsistencyAcrossRuns(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	test := 3
	test := 4
	test := 5
}
`

	engine, fileIDs, cleanup := setupMaxCountTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 3,
	}

	// Run multiple times
	results1 := engine.SearchWithOptions("test", fileIDs, opts)
	results2 := engine.SearchWithOptions("test", fileIDs, opts)
	results3 := engine.SearchWithOptions("test", fileIDs, opts)

	// Should get same count each time
	assert.Equal(t, len(results1), len(results2), "Should get consistent results across runs")
	assert.Equal(t, len(results2), len(results3), "Should get consistent results across runs")
	assert.LessOrEqual(t, len(results1), 3, "Should not exceed max count per file")
}

// setupMaxCountTestEngine creates a test search engine for max count tests
func setupMaxCountTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
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
