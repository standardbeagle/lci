package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
)

// createTestIndexer creates a MasterIndex for testing
func createTestIndexer(t *testing.T) (*MasterIndex, string) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
			WatchMode:        false,
		},
		Performance: config.Performance{
			MaxMemoryMB:   50,
			MaxGoroutines: 2, // Limited for testing
			DebounceMs:    0, // No debounce in tests
		},
		Search: config.Search{
			MaxResults:         100,
			MaxContextLines:    50,
			EnableFuzzy:        true,
			MergeFileResults:   true,
			EnsureCompleteStmt: false,
		},
	}

	indexer := NewMasterIndex(cfg)
	return indexer, tempDir
}

// indexTestFiles indexes the provided files into the test directory
func indexTestFiles(t *testing.T, indexer *MasterIndex, tempDir string, files map[string]string) error {
	// Create test files
	for filename, content := range files {
		path := filepath.Join(tempDir, filename)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	// Index the directory
	ctx := context.Background()
	return indexer.IndexDirectory(ctx, tempDir)
}
