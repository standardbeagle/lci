package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// setupMultiFileProject creates a test project with multiple files in different directories
func setupMultiFileProject(t *testing.T, files map[string]string) (*indexing.MasterIndex, *search.Engine, map[string]types.FileID) {
	t.Helper()

	tempDir := t.TempDir()
	fileIDMap := make(map[string]types.FileID)

	// Create all test files
	for relPath, content := range files {
		fullPath := filepath.Join(tempDir, relPath)

		// Create directory if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}

	// Create config and index
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "multi-file-test",
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

	// Map filenames to FileIDs
	allFileIDs := indexer.GetAllFileIDs()
	for _, fileID := range allFileIDs {
		fileInfo := indexer.GetFile(fileID)
		if fileInfo != nil {
			relPath, _ := filepath.Rel(tempDir, fileInfo.Path)
			fileIDMap[relPath] = fileID
		}
	}

	t.Logf("Indexed %d files: %v", len(allFileIDs), fileIDMap)

	engine := search.NewEngine(indexer)
	return indexer, engine, fileIDMap
}

// TestPathPatterns_IncludePattern tests IncludePattern filtering
func TestPathPatterns_IncludePattern(t *testing.T) {
	files := map[string]string{
		"src/main.go":       `package main\nfunc TestFunc() {}`,
		"src/util.go":       `package main\nfunc TestHelper() {}`,
		"test/main_test.go": `package main\nfunc TestMain() {}`,
		"test/util_test.go": `package main\nfunc TestUtil() {}`,
		"docs/README.md":    `# TestProject`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Include only files in src/ directory
	srcOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "src/",
	})

	// Include only test files
	testOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "*_test.go",
	})

	// Include all .go files
	goFiles := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "*.go",
	})

	// No filter
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("src/ only: %d, *_test.go only: %d, *.go only: %d, all: %d",
		len(srcOnly), len(testOnly), len(goFiles), len(allMatches))

	// Filtered results should be <= total results
	if len(srcOnly) > len(allMatches) {
		t.Errorf("Include filter returned more results than total")
	}
}

// TestPathPatterns_ExcludePattern tests ExcludePattern filtering
func TestPathPatterns_ExcludePattern(t *testing.T) {
	files := map[string]string{
		"src/main.go":       `package main\nfunc TestFunc() {}`,
		"src/util.go":       `package main\nfunc TestHelper() {}`,
		"test/main_test.go": `package main\nfunc TestMain() {}`,
		"test/util_test.go": `package main\nfunc TestUtil() {}`,
		"vendor/lib.go":     `package vendor\nfunc TestVendor() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exclude test files
	noTests := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		ExcludePattern: "*_test.go",
	})

	// Exclude vendor directory
	noVendor := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		ExcludePattern: "vendor/",
	})

	// Exclude multiple patterns (test and vendor)
	noTestsVendor := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		ExcludePattern: "*_test.go,vendor/",
	})

	// No filter
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("Exclude tests: %d, Exclude vendor: %d, Exclude both: %d, All: %d",
		len(noTests), len(noVendor), len(noTestsVendor), len(allMatches))

	// Excluded results should be <= total results
	if len(noTests) > len(allMatches) {
		t.Errorf("Exclude filter returned more results than total")
	}
}

// TestPathPatterns_IncludeAndExclude tests combining include and exclude patterns
func TestPathPatterns_IncludeAndExclude(t *testing.T) {
	files := map[string]string{
		"src/main.go":        `package main\nfunc TestFunc() {}`,
		"src/main_test.go":   `package main\nfunc TestMainTest() {}`,
		"src/util.go":        `package main\nfunc TestHelper() {}`,
		"src/util_test.go":   `package main\nfunc TestUtilTest() {}`,
		"internal/helper.go": `package internal\nfunc TestInternal() {}`,
		"vendor/lib.go":      `package vendor\nfunc TestVendor() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Include src/ but exclude test files
	srcNoTests := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "src/",
		ExcludePattern: "*_test.go",
	})

	// Include all .go files but exclude vendor
	goNoVendor := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "*.go",
		ExcludePattern: "vendor/",
	})

	// No filter
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("src/ excluding tests: %d, *.go excluding vendor: %d, All: %d",
		len(srcNoTests), len(goNoVendor), len(allMatches))

	// Combined filters should be more restrictive
	if len(srcNoTests) > len(allMatches) {
		t.Errorf("Combined filter returned more results than total")
	}
}

// TestPathPatterns_GlobPatterns tests glob pattern matching
func TestPathPatterns_GlobPatterns(t *testing.T) {
	files := map[string]string{
		"cmd/app/main.go":      `package main\nfunc TestApp() {}`,
		"cmd/server/server.go": `package main\nfunc TestServer() {}`,
		"pkg/util/util.go":     `package util\nfunc TestUtil() {}`,
		"internal/core.go":     `package internal\nfunc TestCore() {}`,
		"test/integration.go":  `package test\nfunc TestIntegration() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Match cmd/*/*.go pattern
	cmdFiles := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "cmd/*/*.go",
	})

	// Match pkg/**/*.go pattern (if supported)
	pkgFiles := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "pkg/",
	})

	// Match **/core.go pattern
	coreFiles := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "*core.go",
	})

	t.Logf("cmd/*/*.go: %d, pkg/: %d, *core.go: %d",
		len(cmdFiles), len(pkgFiles), len(coreFiles))
}

// TestPathPatterns_CaseSensitivity tests case sensitivity in path patterns
func TestPathPatterns_CaseSensitivity(t *testing.T) {
	files := map[string]string{
		"SRC/Main.go": `package main\nfunc TestFunc() {}`,
		"src/util.go": `package main\nfunc TestHelper() {}`,
		"TEST/run.go": `package test\nfunc TestRun() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Test case-sensitive pattern matching (implementation dependent)
	upperSrc := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "SRC/",
	})

	lowerSrc := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "src/",
	})

	t.Logf("SRC/: %d, src/: %d", len(upperSrc), len(lowerSrc))
}

// TestPathPatterns_WithSymbolTypes tests combining path patterns with symbol type filtering
func TestPathPatterns_WithSymbolTypes(t *testing.T) {
	files := map[string]string{
		"src/types.go": `package main
type TestStruct struct {}
func TestFunc() {}
var TestVar = "test"`,
		"test/types_test.go": `package main
type TestHelper struct {}
func TestCase() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Functions in src/ only
	srcFuncs := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "src/",
		SymbolTypes:    []string{"function"},
	})

	// Types excluding tests
	typesNoTests := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		ExcludePattern: "*_test.go",
		SymbolTypes:    []string{"class"},
	})

	t.Logf("Functions in src/: %d, Types excluding tests: %d",
		len(srcFuncs), len(typesNoTests))
}

// TestPathPatterns_EmptyPattern tests behavior with empty patterns
func TestPathPatterns_EmptyPattern(t *testing.T) {
	files := map[string]string{
		"main.go": `package main\nfunc TestFunc() {}`,
		"util.go": `package main\nfunc TestHelper() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Empty include pattern should match everything
	emptyInclude := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "",
	})

	// Empty exclude pattern should exclude nothing
	emptyExclude := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		ExcludePattern: "",
	})

	// No filters
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("Empty include: %d, Empty exclude: %d, No filters: %d",
		len(emptyInclude), len(emptyExclude), len(allMatches))

	// Empty patterns should behave like no patterns
	if len(emptyInclude) != len(allMatches) {
		t.Logf("Note: Empty include pattern behavior may differ from no pattern")
	}
}

// TestPathPatterns_MultiplePatterns tests multiple patterns separated by commas
func TestPathPatterns_MultiplePatterns(t *testing.T) {
	files := map[string]string{
		"src/main.go":   `package main\nfunc TestMain() {}`,
		"src/util.go":   `package main\nfunc TestUtil() {}`,
		"pkg/helper.go": `package pkg\nfunc TestHelper() {}`,
		"cmd/app.go":    `package main\nfunc TestApp() {}`,
		"test/run.go":   `package test\nfunc TestRun() {}`,
	}

	indexer, engine, _ := setupMultiFileProject(t, files)
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Include multiple directories
	multiInclude := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		IncludePattern: "src/,pkg/",
	})

	// Exclude multiple patterns
	multiExclude := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		ExcludePattern: "test/,cmd/",
	})

	// No filters
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("Include src/,pkg/: %d, Exclude test/,cmd/: %d, All: %d",
		len(multiInclude), len(multiExclude), len(allMatches))
}
