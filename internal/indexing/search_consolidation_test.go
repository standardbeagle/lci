package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

// TestSearchNoDuplicates verifies that search doesn't return duplicate results
func TestSearchNoDuplicates(t *testing.T) {
	// Create test directory
	testDir := t.TempDir()

	// Create test file with multiple occurrences on same line
	testContent := `package test
func NewHandler(callback func(string)) func() error {
	return func() error {
		callback("test")
		return nil
	}
}`

	// Write test file
	err := os.WriteFile(filepath.Join(testDir, "test.go"), []byte(testContent), 0644)
	require.NoError(t, err)

	// Create test configuration
	cfg := &config.Config{
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

	// Create indexer
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()
	err = indexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Search pattern that appears multiple times on same line
	pattern := "func"
	options := types.SearchOptions{
		CaseInsensitive: false,
		MaxContextLines: 0,
	}

	// Run search
	results, err := indexer.SearchWithOptions(pattern, options)
	require.NoError(t, err)

	// Check for duplicates
	seen := make(map[string]bool)
	for _, r := range results {
		key := fmt.Sprintf("%s:%d", r.Path, r.Line)
		if seen[key] {
			t.Errorf("Duplicate result found for %s line %d", r.Path, r.Line)
		}
		seen[key] = true
	}

	// Verify we found the expected lines (lines 2 and 3 have 'func')
	assert.GreaterOrEqual(t, len(results), 2, "Should find at least 2 lines with 'func'")

	// Check that each result has unique line numbers
	lines := make(map[int]int)
	for _, r := range results {
		lines[r.Line]++
	}

	for line, count := range lines {
		if count > 1 {
			t.Errorf("Line %d appears %d times in results (should be 1)", line, count)
		}
	}
}
