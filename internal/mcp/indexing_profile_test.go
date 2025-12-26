package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"

	"github.com/stretchr/testify/require"
)

// TestIndexingPerformanceProfile profiles the complete indexing workflow
// to identify hotspots causing the 275s slowdown
func TestIndexingPerformanceProfile(t *testing.T) {
	// Create CPU profile
	cpuProfile, err := os.Create("indexing_hotspot_cpu.prof")
	require.NoError(t, err)
	defer cpuProfile.Close()

	// Start CPU profiling
	require.NoError(t, pprof.StartCPUProfile(cpuProfile))
	defer pprof.StopCPUProfile()

	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-indexing-profile-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test files (simulate real project)
	for i := 0; i < 50; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf(`package test

func Function%d() int {
	return %d
}

func Helper%d(x int) int {
	return Function%d() + x
}
`, i, i, i, i)
		require.NoError(t, os.WriteFile(filename, []byte(content), 0644))
	}

	// Create go.mod
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644))

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	require.NoError(t, err)
	cfg.Project.Root = tmpDir

	// Measure indexing time
	start := time.Now()

	// Create index
	goroutineIndex := indexing.NewMasterIndex(cfg)

	// Create server (which triggers auto-indexing)
	server, err := NewServer(goroutineIndex, cfg)
	require.NoError(t, err)

	// Wait for indexing to complete
	status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
	require.NoError(t, err)
	require.Equal(t, "completed", status)

	elapsed := time.Since(start)
	t.Logf("Total indexing time: %v", elapsed)
	t.Logf("Time per file: %v", elapsed/50)

	// Verify index works
	ctx := context.Background()
	results, err := server.Search(ctx, SearchParams{
		Pattern: "Function0",
		Max:     10,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results.Results, "Should find Function0")

	// Check if timing is reasonable (should be < 1 second for 50 files)
	if elapsed > 2*time.Second {
		t.Errorf("Indexing too slow: %v for 50 files (expected < 2s)", elapsed)
	}

	t.Logf("✓ Indexing completed successfully in %v", elapsed)
}
