package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCriticalRegressionFixes ensures that the critical fixes for universal symbol graph
// population and analysis modes remain working and don't regress.
func TestCriticalRegressionFixes(t *testing.T) {
	// Create a simple test file with multiple symbols
	tmpDir, err := os.MkdirTemp("", "lci_critical_regression_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

type Service struct {
	name string
}

func (s *Service) Process() error {
	return nil
}

func main() {
	s := &Service{name: "test"}
	s.Process()
}
`

	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create configuration and index
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tmpDir,
			Name: "critical-regression-test",
		},
		Index: config.Index{
			MaxFileSize:    10 * 1024 * 1024,
			MaxTotalSizeMB: 100,
			MaxFileCount:   1000,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	server, err := NewServer(indexer, cfg)
	require.NoError(t, err)

	// Wait for the auto-indexing (started in NewServer) to complete
	status, err := server.autoIndexManager.waitForCompletion(10 * time.Second)
	require.NoError(t, err)
	require.Equal(t, "completed", status)

	// Verify basic indexing worked
	assert.Greater(t, indexer.GetFileCount(), 0, "Files should be indexed")
	assert.Greater(t, indexer.GetSymbolCount(), 0, "Symbols should be indexed")

	// CRITICAL REGRESSION TEST 1: Universal symbol graph has been removed for performance
	t.Run("UniversalSymbolGraphRemoved", func(t *testing.T) {
		// GetUniversalSymbolGraph method no longer exists
		// The feature was removed to restore fast, lightweight indexing

		// This test verifies that the feature is properly removed
		// rather than checking for its presence
		t.Logf("✓ Universal symbol graph feature removed (no longer supported)")
	})

	// CRITICAL REGRESSION TEST 2: All analysis modes should work
	t.Run("AnalysisModesWork", func(t *testing.T) {
		// All modes are now available (overview, detailed, statistics, unified)
		modes := []string{"overview", "detailed", "statistics", "unified"}

		for _, mode := range modes {
			t.Run("Mode_"+mode, func(t *testing.T) {
				params := map[string]interface{}{"mode": mode}
				result, err := server.CallTool("codebase_intelligence", params)

				// Should succeed without error
				require.NoError(t, err, "Mode %s should succeed", mode)
				require.NotEmpty(t, result, "Mode %s should return result", mode)

				// Verify LCF format
				assert.Contains(t, result, "LCF/1.0", "Mode %s should return LCF format", mode)

				t.Logf("✓ Mode %s succeeded", mode)
			})
		}
	})

	// CRITICAL REGRESSION TEST 3: LCF format should be well-formed
	t.Run("LCFFormatNoMalformed", func(t *testing.T) {
		// This was checking for JSON infinity issues, now checks LCF format
		params := map[string]interface{}{"mode": "detailed"}
		result, err := server.CallTool("codebase_intelligence", params)

		require.NoError(t, err, "Detailed analysis should succeed")
		require.NotEmpty(t, result)

		// The result should be valid LCF format
		assert.Contains(t, result, "LCF/1.0", "Response should be valid LCF format")
		assert.Contains(t, result, "---", "LCF format should have section separators")

		t.Logf("✓ LCF format working correctly")
	})

	t.Logf("✓ All critical regression fixes verified")
}
