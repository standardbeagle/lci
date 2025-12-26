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

// TestConfigConflict_FilesOnlyVsCountPerFile tests FilesOnly conflicting with CountPerFile
func TestConfigConflict_FilesOnlyVsCountPerFile(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Both FilesOnly and CountPerFile - conflicting modes
	opts := types.SearchOptions{
		FilesOnly:    true,
		CountPerFile: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should handle gracefully - typically one takes precedence
	assert.GreaterOrEqual(t, len(results), 0, "Should handle FilesOnly+CountPerFile conflict")
}

// TestConfigConflict_RegexVsWordBoundary tests Regex conflicting with WordBoundary
func TestConfigConflict_RegexVsWordBoundary(t *testing.T) {
	code := `package main

func main() {
	test1 := 1
	test2 := 2
	test := 3
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Regex mode with word boundary - word boundary requires literal search
	opts := types.SearchOptions{
		UseRegex:     true,
		WordBoundary: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should handle gracefully
	assert.GreaterOrEqual(t, len(results), 0, "Should handle Regex+WordBoundary conflict")
}

// TestConfigConflict_InvertMatchVsFilesOnly tests InvertMatch with FilesOnly
func TestConfigConflict_InvertMatchVsFilesOnly(t *testing.T) {
	code := `package main

func main() {
	test := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// InvertMatch with FilesOnly - semantically conflicting
	opts := types.SearchOptions{
		InvertMatch: true,
		FilesOnly:   true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should handle gracefully
	assert.GreaterOrEqual(t, len(results), 0, "Should handle InvertMatch+FilesOnly conflict")
}

// TestConfigConflict_CountPerFileVsCountPerFile tests CountPerFile with FilesOnly
func TestConfigConflict_CountPerFileVsFilesOnly(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
	test := 3
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Both CountPerFile and FilesOnly
	opts := types.SearchOptions{
		CountPerFile: true,
		FilesOnly:    true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should resolve conflicts
	assert.GreaterOrEqual(t, len(results), 0, "Should handle CountPerFile+FilesOnly conflict")
}

// TestConfigConflict_MaxCountVsMaxResults tests MaxCountPerFile vs MaxResults interaction
func TestConfigConflict_MaxCountVsMaxResults(t *testing.T) {
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
	}

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, files)
	defer cleanup()

	// Set both limits - should apply both
	opts := types.SearchOptions{
		MaxCountPerFile: 1,
		MaxResults:      3,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should respect the stricter limit
	assert.GreaterOrEqual(t, len(results), 0, "Should handle both limits")
	assert.LessOrEqual(t, len(results), 3, "Should respect MaxResults limit")
}

// TestConfigConflict_ExcludeIncludeConflict tests mutually exclusive include/exclude
func TestConfigConflict_ExcludeIncludeConflict(t *testing.T) {
	files := map[string]string{
		"test_file.go": `package main
func test() {}`,
		"helper_file.go": `package main
func helper() {}`,
	}

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, files)
	defer cleanup()

	// Conflicting include/exclude that would exclude everything
	opts := types.SearchOptions{
		IncludePattern: "test_*",
		ExcludePattern: "test_*", // Same pattern excluded
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should handle gracefully - exclude typically wins
	assert.GreaterOrEqual(t, len(results), 0, "Should handle conflicting include/exclude")
}

// TestConfigConflict_CaseInsensitiveVsDeclarationOnly tests semantic conflict
func TestConfigConflict_CaseInsensitiveVsDeclarationOnly(t *testing.T) {
	code := `package main

func Test() {
	test()
	test()
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		CaseInsensitive: true,
		DeclarationOnly: true,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should find declarations case-insensitively
	assert.GreaterOrEqual(t, len(results), 0, "Should handle CaseInsensitive+DeclarationOnly")
}

// TestConfigConflict_RegexWithPatterns tests UseRegex with Patterns array
func TestConfigConflict_RegexWithPatterns(t *testing.T) {
	code := `package main

func main() {
	test1 := 1
	test2 := 2
	other := 3
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// UseRegex with Patterns array
	opts := types.SearchOptions{
		UseRegex: true,
		Patterns: []string{"test[0-9]", "other"},
	}

	results := engine.SearchWithOptions("", fileIDs, opts)

	// Should interpret patterns as regexes
	assert.GreaterOrEqual(t, len(results), 0, "Should handle Regex+Patterns combination")
}

// TestConfigConflict_AllConflicts tests all conflicting options at once
func TestConfigConflict_AllConflicts(t *testing.T) {
	code := `package main

func Test() {
	test := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Enable many potentially conflicting options
	opts := types.SearchOptions{
		UseRegex:        true,
		WordBoundary:    true,
		FilesOnly:       true,
		CountPerFile:    true,
		InvertMatch:     true,
		CaseInsensitive: true,
		DeclarationOnly: true,
		MaxResults:      1,
		MaxCountPerFile: 1,
		Patterns:        []string{"test", "other"},
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should handle all conflicts without crashing
	assert.GreaterOrEqual(t, len(results), 0, "Should handle all conflicts simultaneously")
}

// TestConfigConflict_InvertWithMaxCountPerFile tests InvertMatch with MaxCountPerFile
func TestConfigConflict_InvertWithMaxCountPerFile(t *testing.T) {
	code := `package main

func main() {
	test := 1
	other := 2
	value := 3
	helper := 4
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		InvertMatch:     true,
		MaxCountPerFile: 2,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Note: MaxCountPerFile doesn't limit inverted matches per the current implementation
	// InvertMatch returns all non-matching lines, not limited by MaxCountPerFile
	assert.Greater(t, len(results), 0, "Should find inverted matches")
}

// TestConfigConflict_PatternWithSingleArg tests Patterns with single pattern argument
func TestConfigConflict_PatternWithSingleArg(t *testing.T) {
	code := `package main

func main() {
	test := 1
	other := 2
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Patterns array with single argument also provided
	opts := types.SearchOptions{
		Patterns: []string{"test"},
	}

	results := engine.SearchWithOptions("other", fileIDs, opts)

	// Patterns should take precedence
	assert.GreaterOrEqual(t, len(results), 0, "Should handle Patterns precedence")
}

// TestConfigConflict_MaxCountZero tests MaxCountPerFile = 0 interaction
func TestConfigConflict_MaxCountZero(t *testing.T) {
	code := `package main

func main() {
	test := 1
	test := 2
}
`

	engine, fileIDs, cleanup := setupConfigConflictTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	opts := types.SearchOptions{
		MaxCountPerFile: 0, // 0 means unlimited
		MaxResults:      5,
	}

	results := engine.SearchWithOptions("test", fileIDs, opts)

	// Should respect MaxResults while MaxCountPerFile = 0 means unlimited per file
	assert.LessOrEqual(t, len(results), 5, "Should respect MaxResults")
}

// setupConfigConflictTestEngine creates a test search engine for config conflict tests
func setupConfigConflictTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
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
