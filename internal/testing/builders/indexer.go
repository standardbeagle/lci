package builders

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/types"
)

// TestIndexer provides a real indexer configured for testing
type TestIndexer struct {
	*indexing.MasterIndex
	tempDir string
}

// NewTestIndexer creates a new indexer optimized for testing
func NewTestIndexer(t *testing.T) *TestIndexer {
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
		Include: []string{"*"},
		Exclude: []string{},
	}

	indexer := indexing.NewMasterIndex(cfg)

	ti := &TestIndexer{
		MasterIndex: indexer,
		tempDir:     tempDir,
	}

	// Register cleanup to close the indexer when test completes
	t.Cleanup(func() {
		ti.Close()
	})

	return ti
}

// IndexString indexes a string as if it were a file
func (ti *TestIndexer) IndexString(filename, content string) error {
	// Create the file in temp directory
	fullPath := filepath.Join(ti.tempDir, filename)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Index the directory
	ctx := context.Background()
	return ti.IndexDirectory(ctx, ti.tempDir)
}

// IndexFiles indexes multiple files at once
func (ti *TestIndexer) IndexFiles(files map[string]string) error {
	for filename, content := range files {
		if err := ti.IndexString(filename, content); err != nil {
			return err
		}
	}
	return nil
}

// GetSymbolsByName finds symbols by name
func (ti *TestIndexer) GetSymbolsByName(name string) []*types.EnhancedSymbol {
	return ti.FindSymbolsByName(name)
}

// AssertSymbolExists verifies a symbol exists with the given properties
func (ti *TestIndexer) AssertSymbolExists(t *testing.T, name string, symbolType types.SymbolType) {
	symbols := ti.GetSymbolsByName(name)
	for _, sym := range symbols {
		if sym.Name == name && sym.Type == symbolType {
			return
		}
	}
	t.Errorf("Symbol %s of type %v not found", name, symbolType)
}

// GetFileCount returns the number of indexed files
func (ti *TestIndexer) GetFileCount() int {
	return len(ti.GetAllFileIDs())
}

// GetSymbolCount returns the total number of symbols
func (ti *TestIndexer) GetSymbolCount() int {
	count := 0
	for _, fileID := range ti.GetAllFileIDs() {
		symbols := ti.GetFileEnhancedSymbols(fileID)
		count += len(symbols)
	}
	return count
}

// GetFileReferences returns references for a file
func (ti *TestIndexer) GetFileReferences(fileID types.FileID) []types.Reference {
	// Delegate to embedded MasterIndex
	return ti.MasterIndex.GetFileReferences(fileID)
}

// GetFileSymbols returns symbols for a file
func (ti *TestIndexer) GetFileSymbols(fileID types.FileID) []types.Symbol {
	// Delegate to embedded MasterIndex
	return ti.MasterIndex.GetFileSymbols(fileID)
}

// Close cleans up the indexer and stops background goroutines
func (ti *TestIndexer) Close() error {
	return ti.MasterIndex.Close()
}

// TestFile represents a file for testing
type TestFile struct {
	Path    string
	Content string
}
