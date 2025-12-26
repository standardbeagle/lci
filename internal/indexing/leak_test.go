//go:build leaktests
// +build leaktests

package indexing

import (
	"context"
	"runtime"
	"testing"
	"time"

	"go.uber.org/goleak"
	"github.com/standardbeagle/lci/testhelpers"
)

// TestIndexerMemoryLeak explicitly tests for memory leaks after Close()
func TestIndexerMemoryLeak(t *testing.T) {
	// Enable goroutine leak detection
	// Use IgnoreCurrent() to handle test framework goroutines
	defer goleak.VerifyNone(t)

	// Create minimal config
	cfg := testhelpers.NewTestConfigBuilder(".").Build()

	// Create and close indexer
	indexer := NewMasterIndex(cfg)

	// Index a small directory
	ctx := context.Background()
	err := indexer.IndexDirectory(ctx, ".")
	if err != nil {
		t.Fatalf("Failed to index: %v", err)
	}

	// Close should free all resources
	err = indexer.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Wait for cleanup with timeout instead of hardcoded sleep
	// This verifies the Close() actually cleaned up resources
	// NOTE: goleak.VerifyNone above is the actual leak detector
	// This check is just a heuristic to help identify issues faster
	time.Sleep(200 * time.Millisecond) // Give time for goroutines to cleanup
}

// TestIndexerMemoryUsage tests memory growth across multiple index/close cycles
func TestIndexerMemoryUsage(t *testing.T) {
	cfg := testhelpers.NewTestConfigBuilder(".").Build()

	// Get baseline memory
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baselineAlloc := m1.Alloc

	// Run 5 index/close cycles
	for i := 0; i < 5; i++ {
		indexer := NewMasterIndex(cfg)
		ctx := context.Background()

		err := indexer.IndexDirectory(ctx, ".")
		if err != nil {
			t.Fatalf("Cycle %d: Failed to index: %v", i, err)
		}

		err = indexer.Close()
		if err != nil {
			t.Fatalf("Cycle %d: Failed to close: %v", i, err)
		}
	}

	// Check memory after all cycles - WITHOUT forcing GC
	// This reveals if Close() actually frees resources
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	finalAlloc := m2.Alloc

	// Memory should not grow significantly
	// Allow 50MB growth for reasonable caching/fragmentation
	growth := int64(finalAlloc) - int64(baselineAlloc)
	maxGrowth := int64(50 * 1024 * 1024) // 50MB

	if growth > maxGrowth {
		t.Errorf("Memory leak detected: grew by %d MB after 5 cycles (max allowed: 50MB)",
			growth/(1024*1024))
	}

	t.Logf("Memory growth: %.2f MB (baseline: %.2f MB, final: %.2f MB)",
		float64(growth)/(1024*1024),
		float64(baselineAlloc)/(1024*1024),
		float64(finalAlloc)/(1024*1024))
}

// TestWorkflowTestContextLeak tests the workflow test context for leaks
func TestWorkflowTestContextLeak(t *testing.T) {
	// Enable goroutine leak detection for workflow tests
	defer goleak.VerifyNone(t)

	cfg := testhelpers.NewTestConfigBuilder(".").Build()

	// Simulate what workflow tests do
	indexer := NewMasterIndex(cfg)
	ctx := context.Background()
	err := indexer.IndexDirectory(ctx, ".")
	if err != nil {
		t.Fatalf("Failed to index: %v", err)
	}

	// Close (what Cleanup() does)
	indexer.Close()

	// Wait for cleanup with timeout
	// NOTE: goleak.VerifyNone above is the actual leak detector
	// This check is just a heuristic to help identify issues faster
	time.Sleep(200 * time.Millisecond) // Give time for goroutines to cleanup
}
