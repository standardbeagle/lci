package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"

	"github.com/stretchr/testify/require"
)

// TestMCPAutoIndexingWithEmptyDirectory verifies auto-indexing completes correctly with empty directory
func TestMCPAutoIndexingWithEmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-mcp-empty-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "empty-mcp-test",
		},
	}

	idx := indexing.NewMasterIndex(cfg)
	server, err := NewServer(idx, cfg)
	require.NoError(t, err)

	// Wait for auto-indexing to complete
	start := time.Now()
	status, err := server.autoIndexManager.waitForCompletion(10 * time.Second)
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Can be "completed" or "idle" depending on implementation
	if status != "completed" && status != "idle" {
		t.Errorf("Expected 'completed' or 'idle', got: %s", status)
	}

	// Verify CheckIndexingComplete returns nil (not an error)
	err = idx.CheckIndexingComplete()
	require.NoError(t, err, "CheckIndexingComplete should return nil for empty completed index")

	// Verify file count is 0
	require.Equal(t, 0, idx.GetFileCount())

	t.Logf("✓ MCP auto-indexing empty dir: completed in %v, status=%s", elapsed, status)
}

// TestMCPAutoIndexingWithFilteredFiles verifies auto-indexing with all files filtered out
func TestMCPAutoIndexingWithFilteredFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-mcp-filtered-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	// Config that filters out all files
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "filtered-mcp-test",
		},
		Index: config.Index{
			MaxFileCount: 0, // Filter everything
		},
	}

	idx := indexing.NewMasterIndex(cfg)
	server, err := NewServer(idx, cfg)
	require.NoError(t, err)

	// Wait for auto-indexing
	start := time.Now()
	status, err := server.autoIndexManager.waitForCompletion(10 * time.Second)
	elapsed := time.Since(start)

	require.NoError(t, err)
	if status != "completed" && status != "idle" {
		t.Errorf("Expected 'completed' or 'idle', got: %s", status)
	}

	// Verify CheckIndexingComplete returns nil
	err = idx.CheckIndexingComplete()
	require.NoError(t, err, "CheckIndexingComplete should return nil even with 0 files")

	// Verify 0 files indexed
	require.Equal(t, 0, idx.GetFileCount())

	t.Logf("✓ MCP auto-indexing filtered: completed in %v, status=%s", elapsed, status)
}

// TestMCPCodebaseIntelligenceWithEmptyIndex verifies codebase_intelligence doesn't timeout on empty index
func TestMCPCodebaseIntelligenceWithEmptyIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-mcp-ci-empty-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "ci-empty-test",
		},
	}

	idx := indexing.NewMasterIndex(cfg)
	server, err := NewServer(idx, cfg)
	require.NoError(t, err)

	// Wait for auto-indexing
	status, err := server.autoIndexManager.waitForCompletion(10 * time.Second)
	require.NoError(t, err)
	t.Logf("Auto-indexing status: %s", status)

	// Call codebase_intelligence - should NOT timeout (should complete immediately)
	start := time.Now()

	result, err := server.CallTool("codebase_intelligence", map[string]interface{}{
		"mode": "overview",
	})
	elapsed := time.Since(start)

	// Should get some result (error or success), but NOT timeout
	if elapsed > 5*time.Second {
		t.Errorf("codebase_intelligence took %v - should not timeout on empty index!", elapsed)
	}

	t.Logf("✓ codebase_intelligence completed in %v (no 120s timeout)", elapsed)

	// Log the result for debugging
	if err != nil {
		t.Logf("Result error: %v", err)
	}
	if result != "" {
		t.Logf("Result: %s", result[:min(200, len(result))])
	}
}

// TestMCPSearchWithEmptyIndex verifies search doesn't timeout on empty index
func TestMCPSearchWithEmptyIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-mcp-search-empty-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "search-empty-test",
		},
	}

	idx := indexing.NewMasterIndex(cfg)
	server, err := NewServer(idx, cfg)
	require.NoError(t, err)

	// Wait for auto-indexing
	status, err := server.autoIndexManager.waitForCompletion(10 * time.Second)
	require.NoError(t, err)
	t.Logf("Auto-indexing status: %s", status)

	// Search should complete quickly (not timeout)
	ctx := context.Background()
	start := time.Now()

	_, err = server.Search(ctx, SearchParams{
		Pattern: "test",
		Max:     10,
	})
	elapsed := time.Since(start)

	// Should complete in <5s (not wait 120s timeout)
	if elapsed > 5*time.Second {
		t.Errorf("Search took %v - should not timeout on empty index!", elapsed)
	}

	t.Logf("✓ Search completed in %v (no timeout)", elapsed)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
