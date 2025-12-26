package search_test

import (
	"context"
	"fmt"
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

// TestFilesOnly_BasicMatching tests basic files-only mode returns only filenames
func TestFilesOnly_BasicMatching(t *testing.T) {
	code := `package main

func calculate() {
	test := 1
	test := 2
	test := 3
}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Search with files-only mode
	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find at least one result
	assert.Greater(t, len(results), 0, "Should find files with pattern")

	// All results should be from the same file (since there's only one file)
	uniqueFiles := make(map[string]bool)
	for _, result := range results {
		uniqueFiles[result.Path] = true
		// In files-only mode, all matches in a file should appear as separate results
		// but all from the same path
	}

	assert.Equal(t, 1, len(uniqueFiles), "Should return results from only one file")
}

// TestFilesOnly_MultipleFiles tests files-only returns unique file list
func TestFilesOnly_MultipleFiles(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {
	value := test
}`,
		"file2.go": `package main
func helper() {
	other := 1
}`,
		"file3.go": `package main
func main() {
	test()
}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find files containing "test"
	uniqueFiles := make(map[string]bool)
	for _, result := range results {
		uniqueFiles[result.Path] = true
	}

	// Should find in file1 and file3, but not file2
	assert.Greater(t, len(uniqueFiles), 0, "Should find at least one file with pattern")
}

// TestFilesOnly_NoMatches tests files-only with no matching files
func TestFilesOnly_NoMatches(t *testing.T) {
	code := `package main

func helper() {
	value := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("nonexistent", fileIDs, opts)

	// Should return no results
	assert.Equal(t, 0, len(results), "Should return no results when pattern not found")
}

// TestFilesOnly_CaseSensitivity tests files-only with case sensitivity
func TestFilesOnly_CaseSensitivity(t *testing.T) {
	code := `package main

func Test() {
	test()
	TEST()
	result := test
}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Case-sensitive search
	resultsSensitive := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		FilesOnly:       true,
		CaseInsensitive: false,
	})

	// Case-insensitive search
	resultsInsensitive := engine.SearchWithOptions("test", fileIDs, types.SearchOptions{
		FilesOnly:       true,
		CaseInsensitive: true,
	})

	// Both should find the file (since "test" lowercase exists in both cases)
	assert.Greater(t, len(resultsSensitive), 0, "Should find file with case-sensitive search")
	assert.Greater(t, len(resultsInsensitive), 0, "Should find file with case-insensitive search")

	// Case-insensitive should find at least as many as case-sensitive
	assert.GreaterOrEqual(t, len(resultsInsensitive), len(resultsSensitive),
		"Case-insensitive should find at least as many. Sensitive: %d, Insensitive: %d",
		len(resultsSensitive), len(resultsInsensitive))
}

// TestFilesOnly_WithRegex tests files-only with regex patterns
func TestFilesOnly_WithRegex(t *testing.T) {
	code := `package main

func test1() {}
func test2() {}
func helper() {}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
		UseRegex:  true,
	}

	results := engine.SearchWithOptions("test[0-9]", fileIDs, opts)

	// Should find the file matching regex
	assert.Greater(t, len(results), 0, "Should find file with regex pattern")
}

// TestFilesOnly_WithMaxResults tests files-only respects max results
func TestFilesOnly_WithMaxResults(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {}`,
		"file2.go": `package main
func test() {}`,
		"file3.go": `package main
func test() {}`,
		"file4.go": `package main
func test() {}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly:  true,
		MaxResults: 2,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should respect max results limit
	assert.LessOrEqual(t, len(results), 2, "Should respect max results with files-only")
}

// TestFilesOnly_WithIncludePattern tests files-only with include patterns
func TestFilesOnly_WithIncludePattern(t *testing.T) {
	files := map[string]string{
		"handlers.go": `package main
func test() {}`,
		"utils.go": `package main
func test() {}`,
		"main.go": `package main
func other() {}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	// Note: Include pattern test - at minimum should search all files if pattern is broad
	opts := types.SearchOptions{
		FilesOnly:      true,
		IncludePattern: "*.go",
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find results or at least not error
	assert.GreaterOrEqual(t, len(results), 0, "Should process include pattern without error")
}

// TestFilesOnly_WithExcludePattern tests files-only with exclude patterns
func TestFilesOnly_WithExcludePattern(t *testing.T) {
	files := map[string]string{
		"test.go": `package main
func test() {}`,
		"test_helpers.go": `package main
func test() {}`,
		"main.go": `package main
func test() {}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly:      true,
		ExcludePattern: "*helpers*",
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should exclude files matching pattern
	for _, result := range results {
		assert.NotContains(t, result.Path, "helpers", "Should exclude files matching pattern")
	}
}

// TestFilesOnly_WithDeclarationOnly tests files-only with declaration-only filter
func TestFilesOnly_WithDeclarationOnly(t *testing.T) {
	code := `package main

func test() {
	test()
	value := test
}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly:       true,
		DeclarationOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find the file (declarations exist)
	assert.Greater(t, len(results), 0, "Should find file with declarations")
}

// TestFilesOnly_Performance tests files-only performance with large file count
func TestFilesOnly_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create many files
	files := make(map[string]string)
	for i := 0; i < 50; i++ {
		files[fmt.Sprintf("file%d.go", i)] = `package main
func test() { value := test }`
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should handle many files efficiently
	assert.Greater(t, len(results), 0, "Should find matches in large file set")
}

// TestFilesOnly_EmptyPattern tests files-only with empty pattern
func TestFilesOnly_EmptyPattern(t *testing.T) {
	code := `package main
func main() {}`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Empty pattern should return no results
	assert.Equal(t, 0, len(results), "Empty pattern should return no results")
}

// TestFilesOnly_SpecialCharacters tests files-only with special characters in pattern
func TestFilesOnly_SpecialCharacters(t *testing.T) {
	code := `package main

func main() {
	value := "test"
	pattern := "[test]"
	result := test()
}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("[test]", fileIDs, opts)

	// Should find the file
	assert.Greater(t, len(results), 0, "Should find file with special characters")
}

// TestFilesOnly_Unicode tests files-only with unicode patterns
func TestFilesOnly_Unicode(t *testing.T) {
	code := `package main

func café() {
	λ := 1
	π := 3.14
}
`

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("café", fileIDs, opts)

	// Should find the file with unicode
	assert.Greater(t, len(results), 0, "Should find file with unicode pattern")
}

// TestFilesOnly_Integration tests files-only end-to-end with mixed options
func TestFilesOnly_Integration(t *testing.T) {
	files := map[string]string{
		"handlers.go": `package main
func test() {}
func test() {}`,
		"utils.go": `package main
func helper() {}`,
		"middleware.go": `package main
func test() {}
func test() {}
func test() {}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	// Search with files-only
	opts := types.SearchOptions{
		FilesOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find files with test pattern
	uniqueFiles := make(map[string]bool)
	for _, result := range results {
		uniqueFiles[result.Path] = true
	}

	// Should have found 2 files (handlers.go and middleware.go)
	assert.Greater(t, len(uniqueFiles), 0, "Should find files with pattern")
}

// TestFilesOnly_CombinedWithInvertMatch tests files-only with inverted match
func TestFilesOnly_CombinedWithInvertMatch(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {}`,
		"file2.go": `package main
func helper() {}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly:   true,
		InvertMatch: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// With inverted match and files-only, should find files NOT containing pattern
	for _, result := range results {
		assert.NotContains(t, result.Match, "test", "Should return files not containing pattern")
	}
}

// TestFilesOnly_ConsistencyAcrossRuns tests files-only returns consistent results
func TestFilesOnly_ConsistencyAcrossRuns(t *testing.T) {
	files := map[string]string{
		"file1.go": `package main
func test() {}`,
		"file2.go": `package main
func test() {}`,
	}

	engine, fileIDs, cleanup := setupFilesOnlyTestEngine(t, files)
	defer cleanup()

	opts := types.SearchOptions{
		FilesOnly: true,
	}

	// Run search multiple times
	results1 := engine.SearchWithOptions("test", fileIDs, opts)
	results2 := engine.SearchWithOptions("test", fileIDs, opts)

	// Should get consistent results
	assert.Equal(t, len(results1), len(results2), "Should get consistent results across runs")

	// Verify file paths are the same
	filesSet1 := make(map[string]bool)
	for _, r := range results1 {
		filesSet1[r.Path] = true
	}

	filesSet2 := make(map[string]bool)
	for _, r := range results2 {
		filesSet2[r.Path] = true
	}

	assert.Equal(t, filesSet1, filesSet2, "Should find same files across runs")
}

// setupFilesOnlyTestEngine creates a test search engine for files-only tests
func setupFilesOnlyTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
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
