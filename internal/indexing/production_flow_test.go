package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// TestProductionFlow_NewFile_Index_Search tests the full flow:
// new file -> index -> search finds content
func TestProductionFlow_NewFile_Index_Search(t *testing.T) {
	testDir := t.TempDir()

	// Initial file
	initialContent := `package test

func InitialFunction() string {
	return "initial"
}
`
	err := os.WriteFile(filepath.Join(testDir, "initial.go"), []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create indexer
	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	// Initial indexing
	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Verify initial function is found
	results := searchAndWait(t, indexer, "InitialFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find InitialFunction after initial index")
	assertResultContainsPath(t, results, "initial.go")

	// Add NEW file to disk
	newContent := `package test

func BrandNewFunction() string {
	return "brand new"
}

func AnotherNewFunction() int {
	return 42
}
`
	newFilePath := filepath.Join(testDir, "newfile.go")
	err = os.WriteFile(newFilePath, []byte(newContent), 0644)
	require.NoError(t, err)

	// Index the new file using IndexFile (reads from disk)
	err = indexer.IndexFile(newFilePath)
	require.NoError(t, err)

	// Search for content in new file
	results = searchAndWait(t, indexer, "BrandNewFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find BrandNewFunction after adding new file")
	assertResultContainsPath(t, results, "newfile.go")

	// Search for another function in new file
	results = searchAndWait(t, indexer, "AnotherNewFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find AnotherNewFunction after adding new file")
	assertResultContainsPath(t, results, "newfile.go")

	// Original file should still be searchable
	results = searchAndWait(t, indexer, "InitialFunction")
	require.GreaterOrEqual(t, len(results), 1, "Original InitialFunction should still be found")
}

// TestProductionFlow_FileEdit_Index_Search tests the full flow:
// edit file -> update index -> search finds NEW content, not OLD content
func TestProductionFlow_FileEdit_Index_Search(t *testing.T) {
	testDir := t.TempDir()

	// Initial file content with unique searchable term
	initialContent := `package test

func OriginalUniqueFunction() string {
	return "original implementation"
}

func UnchangedFunction() string {
	return "unchanged"
}
`
	filePath := filepath.Join(testDir, "editable.go")
	err := os.WriteFile(filePath, []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create indexer
	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	// Initial indexing
	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Verify original function is found
	results := searchAndWait(t, indexer, "OriginalUniqueFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find OriginalUniqueFunction before edit")
	assertResultContainsPath(t, results, "editable.go")

	// Verify replacement function does NOT exist yet
	results = searchAndWait(t, indexer, "ReplacementUniqueFunction")
	assert.Empty(t, results, "ReplacementUniqueFunction should NOT exist before edit")

	// EDIT the file - replace function name
	editedContent := `package test

func ReplacementUniqueFunction() string {
	return "replacement implementation"
}

func UnchangedFunction() string {
	return "unchanged"
}
`
	err = os.WriteFile(filePath, []byte(editedContent), 0644)
	require.NoError(t, err)

	// Re-index the updated file using IndexFile (reads from disk)
	// This is more reliable than UpdateFile which can have lock issues
	err = indexer.IndexFile(filePath)
	require.NoError(t, err)

	// Search for NEW function - should be found
	results = searchAndWait(t, indexer, "ReplacementUniqueFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find ReplacementUniqueFunction after edit")
	assertResultContainsPath(t, results, "editable.go")

	// Search for OLD function - should NOT be found (critical test!)
	results = searchAndWait(t, indexer, "OriginalUniqueFunction")
	for _, r := range results {
		if strings.Contains(r.Path, "editable.go") {
			t.Errorf("OLD function OriginalUniqueFunction should NOT be found in editable.go after edit, but found at line %d", r.Line)
		}
	}

	// Unchanged function should still be found
	results = searchAndWait(t, indexer, "UnchangedFunction")
	require.GreaterOrEqual(t, len(results), 1, "UnchangedFunction should still be found")
}

// TestProductionFlow_FileDelete_Index_Search tests the full flow:
// delete file -> update index -> search NO LONGER finds content from deleted file
func TestProductionFlow_FileDelete_Index_Search(t *testing.T) {
	testDir := t.TempDir()

	// Create two files
	keepContent := `package test

func KeepThisFunction() string {
	return "keep"
}
`
	deleteContent := `package test

func DeleteMeUniqueFunction() string {
	return "delete me"
}
`
	keepPath := filepath.Join(testDir, "keep.go")
	deletePath := filepath.Join(testDir, "delete.go")

	err := os.WriteFile(keepPath, []byte(keepContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(deletePath, []byte(deleteContent), 0644)
	require.NoError(t, err)

	// Create indexer
	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	// Initial indexing
	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Verify both functions are found
	results := searchAndWait(t, indexer, "KeepThisFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find KeepThisFunction before delete")

	results = searchAndWait(t, indexer, "DeleteMeUniqueFunction")
	require.GreaterOrEqual(t, len(results), 1, "Should find DeleteMeUniqueFunction before delete")
	assertResultContainsPath(t, results, "delete.go")

	// DELETE the file from disk
	err = os.Remove(deletePath)
	require.NoError(t, err)

	// Remove from index
	err = indexer.RemoveFile(deletePath)
	require.NoError(t, err)

	// Search for deleted function - should NOT be found (critical test!)
	results = searchAndWait(t, indexer, "DeleteMeUniqueFunction")
	for _, r := range results {
		if strings.Contains(r.Path, "delete.go") {
			t.Errorf("Deleted function DeleteMeUniqueFunction should NOT be found, but found in %s at line %d", r.Path, r.Line)
		}
	}

	// Kept function should still be found
	results = searchAndWait(t, indexer, "KeepThisFunction")
	require.GreaterOrEqual(t, len(results), 1, "KeepThisFunction should still be found after delete")
	assertResultContainsPath(t, results, "keep.go")
}

// TestProductionFlow_MultipleEdits tests multiple sequential edits
func TestProductionFlow_MultipleEdits(t *testing.T) {
	testDir := t.TempDir()

	filePath := filepath.Join(testDir, "multi.go")

	// Version 1
	v1Content := `package test

func Version1Function() {}
`
	err := os.WriteFile(filePath, []byte(v1Content), 0644)
	require.NoError(t, err)

	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	results := searchAndWait(t, indexer, "Version1Function")
	require.GreaterOrEqual(t, len(results), 1, "Should find Version1Function")

	// Version 2
	v2Content := `package test

func Version2Function() {}
`
	err = os.WriteFile(filePath, []byte(v2Content), 0644)
	require.NoError(t, err)
	err = indexer.IndexFile(filePath)
	require.NoError(t, err)

	results = searchAndWait(t, indexer, "Version2Function")
	require.GreaterOrEqual(t, len(results), 1, "Should find Version2Function")

	results = searchAndWait(t, indexer, "Version1Function")
	assertResultNotContainsPath(t, results, "multi.go", "Version1Function should NOT be in multi.go after update to v2")

	// Version 3
	v3Content := `package test

func Version3Function() {}
`
	err = os.WriteFile(filePath, []byte(v3Content), 0644)
	require.NoError(t, err)
	err = indexer.IndexFile(filePath)
	require.NoError(t, err)

	results = searchAndWait(t, indexer, "Version3Function")
	require.GreaterOrEqual(t, len(results), 1, "Should find Version3Function")

	results = searchAndWait(t, indexer, "Version2Function")
	assertResultNotContainsPath(t, results, "multi.go", "Version2Function should NOT be in multi.go after update to v3")

	results = searchAndWait(t, indexer, "Version1Function")
	assertResultNotContainsPath(t, results, "multi.go", "Version1Function should NOT be in multi.go after update to v3")
}

// TestProductionFlow_SymbolSearch tests symbol-specific search after updates
func TestProductionFlow_SymbolSearch(t *testing.T) {
	testDir := t.TempDir()

	initialContent := `package test

type OldStructName struct {
	Field string
}

func (o *OldStructName) OldMethod() {}
`
	filePath := filepath.Join(testDir, "symbols.go")
	err := os.WriteFile(filePath, []byte(initialContent), 0644)
	require.NoError(t, err)

	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Search for old struct
	results := searchAndWait(t, indexer, "OldStructName")
	require.GreaterOrEqual(t, len(results), 1, "Should find OldStructName")

	// Update with renamed struct
	updatedContent := `package test

type NewStructName struct {
	Field string
}

func (n *NewStructName) NewMethod() {}
`
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	require.NoError(t, err)
	err = indexer.IndexFile(filePath)
	require.NoError(t, err)

	// New struct should be found
	results = searchAndWait(t, indexer, "NewStructName")
	require.GreaterOrEqual(t, len(results), 1, "Should find NewStructName after rename")

	// Old struct should NOT be found
	results = searchAndWait(t, indexer, "OldStructName")
	assertResultNotContainsPath(t, results, "symbols.go", "OldStructName should NOT be in symbols.go after rename")

	// New method should be found
	results = searchAndWait(t, indexer, "NewMethod")
	require.GreaterOrEqual(t, len(results), 1, "Should find NewMethod after rename")

	// Old method should NOT be found
	results = searchAndWait(t, indexer, "OldMethod")
	assertResultNotContainsPath(t, results, "symbols.go", "OldMethod should NOT be in symbols.go after rename")
}

// TestProductionFlow_ContentSearch tests content search (not just symbols)
func TestProductionFlow_ContentSearch(t *testing.T) {
	testDir := t.TempDir()

	initialContent := `package test

// This is the ORIGINAL_UNIQUE_COMMENT that should disappear after edit
func SomeFunction() {
	originalUniqueString := "original"
	_ = originalUniqueString
}
`
	filePath := filepath.Join(testDir, "content.go")
	err := os.WriteFile(filePath, []byte(initialContent), 0644)
	require.NoError(t, err)

	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Search for unique content
	results := searchAndWait(t, indexer, "ORIGINAL_UNIQUE_COMMENT")
	require.GreaterOrEqual(t, len(results), 1, "Should find ORIGINAL_UNIQUE_COMMENT")

	results = searchAndWait(t, indexer, "originalUniqueString")
	require.GreaterOrEqual(t, len(results), 1, "Should find originalUniqueString")

	// Update file with different content
	updatedContent := `package test

// This is the REPLACEMENT_UNIQUE_COMMENT after the edit
func SomeFunction() {
	replacementUniqueString := "replacement"
	_ = replacementUniqueString
}
`
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	require.NoError(t, err)
	err = indexer.IndexFile(filePath)
	require.NoError(t, err)

	// New content should be found
	results = searchAndWait(t, indexer, "REPLACEMENT_UNIQUE_COMMENT")
	require.GreaterOrEqual(t, len(results), 1, "Should find REPLACEMENT_UNIQUE_COMMENT")

	results = searchAndWait(t, indexer, "replacementUniqueString")
	require.GreaterOrEqual(t, len(results), 1, "Should find replacementUniqueString")

	// Old content should NOT be found
	results = searchAndWait(t, indexer, "ORIGINAL_UNIQUE_COMMENT")
	assertResultNotContainsPath(t, results, "content.go", "ORIGINAL_UNIQUE_COMMENT should NOT be found after edit")

	results = searchAndWait(t, indexer, "originalUniqueString")
	assertResultNotContainsPath(t, results, "content.go", "originalUniqueString should NOT be found after edit")
}

// TestProductionFlow_ConcurrentSearchDuringUpdate tests that searches during updates are consistent
func TestProductionFlow_ConcurrentSearchDuringUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	testDir := t.TempDir()

	initialContent := `package test

func StableFunction() {}
func ChangingFunction() {}
`
	filePath := filepath.Join(testDir, "concurrent.go")
	err := os.WriteFile(filePath, []byte(initialContent), 0644)
	require.NoError(t, err)

	cfg := createProductionFlowTestConfig(testDir)
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()

	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Run concurrent searches while updating
	done := make(chan bool)
	errors := make(chan error, 100)

	// Searcher goroutine
	go func() {
		for i := 0; i < 50; i++ {
			results, err := indexer.SearchWithOptions("StableFunction", types.SearchOptions{})
			if err != nil {
				errors <- err
				continue
			}
			// StableFunction should always be found (it's never removed)
			if len(results) == 0 {
				errors <- assert.AnError
			}
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Updater goroutine - uses IndexFile instead of UpdateFile to avoid deadlock
	go func() {
		for i := 0; i < 10; i++ {
			content := `package test

func StableFunction() {}
func ChangingFunction` + string(rune('A'+i)) + `() {}
`
			_ = os.WriteFile(filePath, []byte(content), 0644)
			err := indexer.IndexFile(filePath)
			if err != nil {
				errors <- err
			}
			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent operation error: %v", err)
		}
	}
}

// Helper functions

func createProductionFlowTestConfig(testDir string) *config.Config {
	return &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Search:  config.Search{},
		Include: []string{"*.go"},
		Exclude: []string{},
	}
}

func searchAndWait(t *testing.T, indexer *MasterIndex, pattern string) []searchtypes.Result {
	t.Helper()
	results, err := indexer.SearchWithOptions(pattern, types.SearchOptions{
		CaseInsensitive: false,
		MaxContextLines: 0,
	})
	require.NoError(t, err)
	return results
}

func assertResultContainsPath(t *testing.T, results []searchtypes.Result, expectedPath string) {
	t.Helper()
	for _, r := range results {
		if strings.Contains(r.Path, expectedPath) {
			return
		}
	}
	t.Errorf("Expected results to contain path %s, but got: %v", expectedPath, pathsFromResults(results))
}

func assertResultNotContainsPath(t *testing.T, results []searchtypes.Result, forbiddenPath string, msg string) {
	t.Helper()
	for _, r := range results {
		if strings.Contains(r.Path, forbiddenPath) {
			t.Errorf("%s - found in %s at line %d", msg, r.Path, r.Line)
		}
	}
}

func pathsFromResults(results []searchtypes.Result) []string {
	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}
	return paths
}
