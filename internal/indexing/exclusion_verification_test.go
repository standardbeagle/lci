package indexing

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
)

// Test to verify that real_projects is excluded when counting files
func TestExclusionsAppliedCorrectly(t *testing.T) {
	// Get project root
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}

	projectRoot := filepath.Join(filepath.Dir(filepath.Dir(currentFile)), "..")

	// Load config
	cfg, err := config.LoadWithRoot("", projectRoot)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	t.Logf("Project root: %s", cfg.Project.Root)
	t.Logf("Exclusions: %d patterns", len(cfg.Exclude))

	// Verify real_projects is in exclusions
	hasRealProjects := false
	for _, pattern := range cfg.Exclude {
		if pattern == "**/real_projects/**" {
			hasRealProjects = true
			break
		}
	}
	require.True(t, hasRealProjects, "real_projects should be in exclusions")

	// Count files that would be indexed
	scanner := NewFileScanner(cfg, 100)
	ctx := context.Background()
	fileCount, totalBytes, err := scanner.CountFiles(ctx, cfg.Project.Root)
	require.NoError(t, err)

	t.Logf("Files to index: %d", fileCount)
	t.Logf("Total size: %d MB", totalBytes/1024/1024)

	// Should be much less than 8815 files if real_projects is excluded
	// The project itself (without test data) should be around 200-500 files
	if fileCount > 1000 {
		t.Errorf("Too many files to index: %d (expected <1000). Exclusions may not be working!", fileCount)
		t.Logf("This suggests real_projects and other test directories are not being excluded properly")
	} else {
		t.Logf("✓ File count looks correct: %d files (exclusions working)", fileCount)
	}
}
