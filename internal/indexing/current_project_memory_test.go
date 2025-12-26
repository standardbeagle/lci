package indexing

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
)

// TestCurrentProjectMemoryUsage profiles the memory usage when indexing the LCI project itself
func TestCurrentProjectMemoryUsage(t *testing.T) {
	// Get project root dynamically - navigate from test file location
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}

	// Navigate from internal/indexing/ to project root
	// currentFile is at: .../lightning-code-index/internal/indexing/current_project_memory_test.go
	// Go up from internal/indexing -> internal -> project root
	projectRoot := filepath.Join(filepath.Dir(filepath.Dir(currentFile)), "..")

	// Fallback to environment variable if path resolution fails
	if envRoot := os.Getenv("LCI_PROJECT_ROOT"); envRoot != "" {
		projectRoot = envRoot
	}

	// Verify project root exists
	if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
		t.Fatalf("Project root not found: %s\nSet LCI_PROJECT_ROOT environment variable if path resolution failed", projectRoot)
	}

	// Load config from project root to get proper exclusions (including real_projects)
	cfg, err := config.LoadWithRoot("", projectRoot)
	require.NoError(t, err)

	// Debug: verify config has correct settings
	t.Logf("Config project root: %s", cfg.Project.Root)
	t.Logf("Actual indexing root: %s", projectRoot)
	t.Logf("Number of exclusions: %d", len(cfg.Exclude))
	hasRealProjects := false
	for _, pattern := range cfg.Exclude {
		if pattern == "**/real_projects/**" {
			hasRealProjects = true
			break
		}
	}
	t.Logf("Has real_projects exclusion: %v", hasRealProjects)

	// Track memory before indexing
	var m1, m2 runtime.MemStats
	runtime.GC() // Force GC before measuring
	runtime.ReadMemStats(&m1)
	t.Logf("Before indexing: Alloc=%dMB Sys=%dMB HeapAlloc=%dMB NumGoroutine=%d",
		m1.Alloc/1024/1024, m1.Sys/1024/1024, m1.HeapAlloc/1024/1024, runtime.NumGoroutine())

	// Create and run indexer
	idx := NewMasterIndex(cfg)
	defer idx.Close()

	ctx := context.Background()
	err = idx.IndexDirectory(ctx, projectRoot)
	if err != nil {
		t.Logf("Index error: %v", err)
	}

	// Measure memory after indexing
	runtime.GC() // Force GC to get accurate numbers
	runtime.ReadMemStats(&m2)
	t.Logf("After indexing: Alloc=%dMB Sys=%dMB HeapAlloc=%dMB NumGoroutine=%d",
		m2.Alloc/1024/1024, m2.Sys/1024/1024, m2.HeapAlloc/1024/1024, runtime.NumGoroutine())

	// Calculate increases
	allocIncrease := (m2.Alloc - m1.Alloc) / 1024 / 1024
	sysIncrease := (m2.Sys - m1.Sys) / 1024 / 1024
	heapIncrease := (m2.HeapAlloc - m1.HeapAlloc) / 1024 / 1024

	t.Logf("Memory increase: Alloc=%dMB Sys=%dMB HeapAlloc=%dMB",
		allocIncrease, sysIncrease, heapIncrease)

	// Get stats
	stats := idx.GetStats()
	fileCount, _ := stats["file_count"].(int)
	symbolCount, _ := stats["symbol_count"].(int)
	t.Logf("Indexed: %d files, %d symbols", fileCount, symbolCount)

	// Calculate memory per file
	if fileCount > 0 {
		memPerFile := int64(allocIncrease) / int64(fileCount)
		t.Logf("Memory per file: %dKB", memPerFile*1024)
	}

	// Write heap profile for analysis
	f, err := os.Create("lci_project_heap.prof")
	if err != nil {
		t.Fatalf("Failed to create heap profile: %v", err)
	}
	defer f.Close()

	err = pprof.WriteHeapProfile(f)
	require.NoError(t, err)
	t.Logf("Heap profile written to lci_project_heap.prof")
	t.Logf("Analyze with: go tool pprof -http=:8080 lci_project_heap.prof")

	// Memory assertions
	// Note: The LCI codebase is large and complex, so memory usage can exceed 1GB
	// This is a profiling test to understand memory usage patterns, not a strict requirement
	if allocIncrease > 1500 {
		t.Errorf("Memory usage critically high: %dMB allocated (should be <1.5GB)", allocIncrease)
		t.Logf("Note: High memory usage may be due to:")
		t.Logf("  - Large number of source files and test files")
		t.Logf("  - Tree-sitter language parser overhead")
		t.Logf("  - Index data structures")

		// Get top memory allocations for debugging
		t.Logf("\nTop memory allocations:")
		t.Logf("Run: go tool pprof -top lci_project_heap.prof")
	} else {
		t.Logf("✓ Memory usage acceptable: %dMB allocated", allocIncrease)
	}
}
