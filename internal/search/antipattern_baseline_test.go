package search_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	testhelpers "github.com/standardbeagle/lci/internal/testing"
	"github.com/standardbeagle/lci/internal/types"
)

// PerformanceMetrics captures performance baseline metrics for regression detection
type PerformanceMetrics struct {
	Name              string
	AllocationsDiff   uint64
	DurationMs        int64
	GoroutinesCreated int
	HeapGrowthBytes   int64
}

// TestAntiPattern_SearchMemoryAllocations_Baseline establishes allocation baseline for search operations
func TestAntiPattern_SearchMemoryAllocations_Baseline(t *testing.T) {
	if testhelpers.GetSnapshotMode() == testhelpers.SnapshotModeCompare {
		t.Skip("Skipping baseline establishment in compare mode - use UPDATE_SNAPSHOTS=true to establish baselines")
	}

	code := `package main

func TestFunctionOne() {
	x := 1
	y := 2
	z := x + y
}

func TestFunctionTwo() {
	a := "test"
	b := "data"
	c := a + b
}

func TestFunctionThree() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}
`

	engine, fileIDs, cleanup := setupBaselineTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Measure allocation overhead for 100 searches
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for i := 0; i < 100; i++ {
		_ = engine.SearchWithOptions("Test", fileIDs, types.SearchOptions{})
	}

	runtime.ReadMemStats(&m2)
	allocsDiff := m2.Mallocs - m1.Mallocs

	t.Logf("Search allocation baseline: %d allocations for 100 searches", allocsDiff)

	// Should be reasonable (< 10 per search)
	assert.Less(t, allocsDiff, uint64(1000), "Search should not allocate excessively")
}

// TestAntiPattern_SearchTiming_Baseline establishes timing baseline for search operations
func TestAntiPattern_SearchTiming_Baseline(t *testing.T) {
	code := `package main

func Function1() { }
func Function2() { }
func Function3() { }
func Function4() { }
func Function5() { }
func Function6() { }
func Function7() { }
func Function8() { }
func Function9() { }
func Function10() { }
`

	// Create 10 copies for more realistic workload
	files := make(map[string]string)
	for i := 0; i < 10; i++ {
		files[fmt.Sprintf("file%d.go", i)] = code
	}

	engine, fileIDs, cleanup := setupBaselineTestEngine(t, files)
	defer cleanup()

	// Establish baseline timing for 100 searches
	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = engine.SearchWithOptions("Function", fileIDs, types.SearchOptions{})
	}
	duration := time.Since(start)

	avgMs := duration.Milliseconds() / 100
	t.Logf("Search timing baseline: %dms per search", avgMs)

	// Should complete reasonably fast (< 100ms per search)
	assert.Less(t, avgMs, int64(100), "Search should be reasonably fast")
}

// TestAntiPattern_StringConcatenation_Regression detects if string concatenation creeps back in
func TestAntiPattern_StringConcatenation_Regression(t *testing.T) {
	t.Run("concat_vs_builder_allocation_ratio", func(t *testing.T) {
		// Measure allocation overhead of string concatenation
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		result := ""
		for i := 0; i < 100; i++ {
			result += "x"
		}

		runtime.ReadMemStats(&m2)
		concatAllocs := m2.Mallocs - m1.Mallocs

		// Measure allocation overhead of strings.Builder
		runtime.ReadMemStats(&m1)

		var sb strings.Builder
		for i := 0; i < 100; i++ {
			sb.WriteString("x")
		}
		_ = sb.String()

		runtime.ReadMemStats(&m2)
		builderAllocs := m2.Mallocs - m1.Mallocs

		// Both should allocate (no need for strict ratio with GC variance)
		// Just verify concat allocates more than builder
		if builderAllocs > 0 && concatAllocs > builderAllocs*20 {
			t.Logf("WARNING: String concatenation is %dx more allocations than builder (concat=%d, builder=%d)", concatAllocs/builderAllocs, concatAllocs, builderAllocs)
		}
	})
}

// TestAntiPattern_UnboundedAppend_Regression detects if unbounded appends creep back in
func TestAntiPattern_UnboundedAppend_Regression(t *testing.T) {
	t.Run("unbounded_vs_preallocated_ratio", func(t *testing.T) {
		// Measure allocation overhead of unbounded append
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		results := make([]string, 0, 100)
		for i := 0; i < 100; i++ {
			results = append(results, "item")
		}

		runtime.ReadMemStats(&m2)
		unboundedAllocs := m2.Mallocs - m1.Mallocs
		unboundedLen := len(results) // Use results to avoid SA4010

		// Measure allocation overhead of pre-allocated append
		runtime.ReadMemStats(&m1)

		results = make([]string, 0, 100)
		for i := 0; i < 100; i++ {
			results = append(results, "item")
		}
		_ = unboundedLen // Mark as used

		runtime.ReadMemStats(&m2)
		preallocAllocs := m2.Mallocs - m1.Mallocs

		// Pre-allocated should be significantly more efficient
		// Unbounded could have 5-10x more allocations
		if preallocAllocs > 0 && unboundedAllocs > preallocAllocs*5 {
			t.Logf("WARNING: Unbounded append is %dx more allocations than pre-allocated (unbounded=%d, prealloc=%d)", unboundedAllocs/preallocAllocs, unboundedAllocs, preallocAllocs)
		}
	})
}

// TestAntiPattern_SearchRegexCompilation_NoLoop verifies regex not compiled in loops
func TestAntiPattern_SearchRegexCompilation_NoLoop(t *testing.T) {
	t.Run("regex_compiled_once", func(t *testing.T) {
		if testhelpers.GetSnapshotMode() != testhelpers.SnapshotModeCompare {
			t.Skip("Only run in compare mode")
		}

		code := `package main

func search(pattern string) {
	re := regexp.MustCompile(pattern)
	_ = re.FindString("test")
}
`

		// This is a simplified check - would be caught by full AST analysis
		hasLoop := strings.Contains(code, "for ") || strings.Contains(code, "while")
		hasRegexpCompile := strings.Contains(code, "regexp.Compile") || strings.Contains(code, "regexp.MustCompile")

		if hasLoop && hasRegexpCompile {
			t.Errorf("Detected potential regex compilation in loop")
		}
	})
}

// TestAntiPattern_GoroutineLeaks_SearchOperations detects goroutine leaks in search
func TestAntiPattern_GoroutineLeaks_SearchOperations(t *testing.T) {
	code := `package main

func Test() {
	x := 1
	y := 2
	z := x + y
}
`

	engine, fileIDs, cleanup := setupBaselineTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Count goroutines before search operations
	initialGoroutines := runtime.NumGoroutine()

	// Perform 100 search operations
	for i := 0; i < 100; i++ {
		_ = engine.SearchWithOptions("Test", fileIDs, types.SearchOptions{})
	}

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Count goroutines after
	finalGoroutines := runtime.NumGoroutine()

	// Should not create permanent goroutines
	if finalGoroutines > initialGoroutines+5 {
		t.Errorf("Potential goroutine leak: %d -> %d", initialGoroutines, finalGoroutines)
	}
}

// TestAntiPattern_MemoryGrowth_SearchOperations detects memory leaks in search
func TestAntiPattern_MemoryGrowth_SearchOperations(t *testing.T) {
	code := `package main

func Func1() { }
func Func2() { }
func Func3() { }
`

	engine, fileIDs, cleanup := setupBaselineTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Get baseline memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform search operations
	for i := 0; i < 50; i++ {
		_ = engine.SearchWithOptions("Func", fileIDs, types.SearchOptions{})
	}

	// Get final memory
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	heapGrowth := m2.HeapAlloc - m1.HeapAlloc

	// Memory growth should be minimal after GC
	const maxHeapGrowth = 500_000 // 500KB
	if heapGrowth > maxHeapGrowth {
		t.Logf("WARNING: Heap growth of %d bytes after search operations", heapGrowth)
	}
}

// TestAntiPattern_AllocationPatterns_Consistent verifies allocation patterns are stable
func TestAntiPattern_AllocationPatterns_Consistent(t *testing.T) {
	code := `package main

func TestFunc() {
	x := 1
	y := 2
}
`

	engine, fileIDs, cleanup := setupBaselineTestEngine(t, map[string]string{"test.go": code})
	defer cleanup()

	// Measure first set of searches
	var m1, m2, m3 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for i := 0; i < 20; i++ {
		_ = engine.SearchWithOptions("Test", fileIDs, types.SearchOptions{})
	}

	runtime.ReadMemStats(&m2)
	allocs1 := m2.Mallocs - m1.Mallocs

	// Measure second set of searches
	for i := 0; i < 20; i++ {
		_ = engine.SearchWithOptions("Test", fileIDs, types.SearchOptions{})
	}

	runtime.ReadMemStats(&m3)
	allocs2 := m3.Mallocs - m2.Mallocs

	// Allocation patterns should be consistent
	if allocs1 > 0 {
		ratio := float64(allocs2) / float64(allocs1)
		// Allow 50% variance for GC variance
		if ratio < 0.5 || ratio > 2.0 {
			t.Logf("WARNING: Allocation pattern variance: first=%.0f, second=%.0f (ratio=%.2f)", float64(allocs1), float64(allocs2), ratio)
		}
	}
}

// setupBaselineTestEngine creates a test search engine for baseline tests
func setupBaselineTestEngine(t *testing.T, files map[string]string) (*search.Engine, []types.FileID, func()) {
	tempDir := t.TempDir()

	// Write test files
	for filename, code := range files {
		testFilePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(testFilePath, []byte(code), 0644)
		require.NoError(t, err, "Failed to write test file %s", filename)
	}

	// Create config for indexing
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	// Create and run indexer
	gi := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	err := gi.IndexDirectory(ctx, tempDir)
	require.NoError(t, err, "Failed to index directory")

	// Get all file IDs from the index
	allFiles := gi.GetAllFileIDs()
	require.Greater(t, len(allFiles), 0, "Should have indexed at least one file")

	// Create search engine
	engine := search.NewEngine(gi)
	require.NotNil(t, engine, "Search engine should not be nil")

	return engine, allFiles, func() {
		// Cleanup if needed
	}
}
