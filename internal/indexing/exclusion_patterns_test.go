package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

// TestDefaultExclusionPatterns tests that the default config properly excludes expected directories
func TestDefaultExclusionPatterns(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "lci-exclusion-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test directory structure
	testDirs := []string{
		".git",
		".git/objects",
		".git/objects/pack",
		".svn",
		".hg",
		"node_modules",
		"node_modules/react",
		"vendor",
		"vendor/github.com",
		"dist",
		"build",
		"target",
		"__pycache__",
	}

	testFiles := []string{
		"main.go",
		"index.js",
		".git/config",
		".git/HEAD",
		".git/objects/pack/pack-abc123.idx",
		".git/objects/pack/pack-abc123.pack",
		"node_modules/react/index.js",
		"vendor/github.com/pkg/errors/errors.go",
		"dist/bundle.js",
		"build/output.js",
		"src/app.js",
		"README.md",
		"test.pyc",
		"debug.log",
	}

	// Create all directories
	for _, dir := range testDirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create all files
	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Load default configuration
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.Project.Root = tmpDir

	// Create master index with default config
	idx := NewMasterIndex(cfg)
	defer idx.Close()

	// Index the directory
	ctx := context.Background()
	err = idx.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Logf("Index error (may be expected): %v", err)
	}

	// Get indexed files
	stats := idx.GetStats()
	fileCount, _ := stats["file_count"].(int)

	// Check that only expected files were indexed
	expectedFiles := []string{
		"main.go",
		"index.js",
		"src/app.js",
		"README.md",
	}

	// Files that should NOT be indexed
	excludedFiles := []string{
		".git/config",
		".git/HEAD",
		".git/objects/pack/pack-abc123.idx",
		"node_modules/react/index.js",
		"vendor/github.com/pkg/errors/errors.go",
		"dist/bundle.js",
		"build/output.js",
		"test.pyc",
		"debug.log",
	}

	t.Logf("Indexed %d files", fileCount)

	// Verify expected file count
	assert.LessOrEqual(t, fileCount, len(expectedFiles),
		"Should index at most %d files, but indexed %d", len(expectedFiles), fileCount)

	// Search for content that should be indexed
	for _, file := range expectedFiles {
		results, err := idx.SearchWithOptions("test content", types.SearchOptions{
			IncludePattern: file,
		})
		if err == nil && len(results) > 0 {
			t.Logf("✓ File %s was indexed (found in search)", file)
		}
	}

	// Verify excluded files are NOT indexed
	for _, file := range excludedFiles {
		results, err := idx.SearchWithOptions("test content", types.SearchOptions{
			IncludePattern: file,
		})
		assert.NoError(t, err)
		assert.Empty(t, results, "File %s should NOT have been indexed", file)
	}
}

// TestGitFolderExclusion specifically tests that .git folders are excluded at all levels
func TestGitFolderExclusion(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "lci-git-exclusion-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create nested git repositories (common in monorepos)
	gitPaths := []string{
		".git/config",
		".git/objects/pack/pack-123.idx",
		"subproject/.git/config",
		"deeply/nested/path/.git/HEAD",
	}

	normalFiles := []string{
		"main.go",
		"subproject/app.js",
		"deeply/nested/path/code.py",
	}

	// Create all files
	for _, file := range append(gitPaths, normalFiles...) {
		fullPath := filepath.Join(tmpDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("content: "+file), 0644)
		require.NoError(t, err)
	}

	// Load default configuration
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.Project.Root = tmpDir

	// Create and run indexing
	idx := NewMasterIndex(cfg)
	defer idx.Close()

	ctx := context.Background()
	err = idx.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Logf("Index error (may be expected): %v", err)
	}

	stats := idx.GetStats()
	fileCount, _ := stats["file_count"].(int)

	// Should only index the normal files, not .git contents
	assert.Equal(t, len(normalFiles), fileCount,
		"Should have indexed exactly %d files, but got %d", len(normalFiles), fileCount)

	// Verify .git files are not searchable
	for _, gitFile := range gitPaths {
		results, err := idx.SearchWithOptions("content:", types.SearchOptions{})
		found := false
		if err == nil {
			for _, result := range results {
				if filepath.Join(tmpDir, gitFile) == result.Path {
					found = true
					break
				}
			}
		}
		assert.False(t, found, ".git file %s should NOT be indexed", gitFile)
	}
}

// TestExclusionPatternsWithRealConfig tests exclusion patterns using actual config loading
func TestExclusionPatternsWithRealConfig(t *testing.T) {
	// Load the actual default config
	cfg, err := config.Load("")
	require.NoError(t, err)

	// Verify critical exclusion patterns are present
	expectedPatterns := []string{
		"**/.*/**",           // All hidden directories
		"**/node_modules/**", // Node modules
		"**/vendor/**",       // Go vendor
		"**/.git/**",         // Git directories (if not caught by .*/**)
		"**/__pycache__/**",  // Python cache
		"**/dist/**",         // Build outputs
		"**/*.log",           // Log files
	}

	for _, pattern := range expectedPatterns {
		found := false
		for _, exclude := range cfg.Exclude {
			if exclude == pattern {
				found = true
				break
			}
		}
		// The .*/** pattern should cover .git
		if pattern == "**/.git/**" {
			found = found || containsPattern(cfg.Exclude, "**/.*/**")
		}
		assert.True(t, found, "Expected exclusion pattern %s not found in config", pattern)
	}
}

func containsPattern(patterns []string, pattern string) bool {
	for _, p := range patterns {
		if p == pattern {
			return true
		}
	}
	return false
}
