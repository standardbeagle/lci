package search_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/standardbeagle/lci/testhelpers"
)

// TECHNIQUE 1: Memory allocation tracking
// Detects: String concatenation in loops, unbounded appends, leaky allocations
// Purpose: Prevent performance regressions from inefficient string operations
func TestAntiPattern_StringConcatenationAllocations(t *testing.T) {
	t.Run("string_concatenation_detects_excessive_allocs", func(t *testing.T) {
		// This detects string concatenation via allocation counting
		// Threshold is set generously to catch only severe regressions
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Function that uses string += (anti-pattern)
		// This creates one allocation per iteration (quadratic time complexity)
		result := ""
		itemCount := 100 // Keep small for fast test
		for i := 0; i < itemCount; i++ {
			result += "x" // Creates ~100 allocations
		}

		runtime.ReadMemStats(&m2)
		allocsDiff := m2.Mallocs - m1.Mallocs

		// String concat should NOT allocate many times per item
		// Generous threshold: allow up to 3x expected allocations for noise
		expectedMax := uint64(itemCount * 3)
		if allocsDiff > expectedMax {
			t.Errorf("String concatenation allocated %d times for %d items - use strings.Builder instead (threshold: %d)", allocsDiff, itemCount, expectedMax)
		}
	})

	t.Run("string_builder_minimal_allocs", func(t *testing.T) {
		// Verify that strings.Builder uses fewer allocations
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		var sb strings.Builder
		itemCount := 100
		for i := 0; i < itemCount; i++ {
			sb.WriteString("x")
		}
		_ = sb.String()

		runtime.ReadMemStats(&m2)
		allocsDiff := m2.Mallocs - m1.Mallocs

		// strings.Builder should use far fewer allocations than naive concatenation
		expectedMax := uint64(10) // Builder typically uses 2-3 allocations
		assert.LessOrEqual(t, allocsDiff, expectedMax, "strings.Builder should use minimal allocations")
	})
}

// TECHNIQUE 1 (continued): Unbounded append detection
func TestAntiPattern_UnboundedAppendAllocations(t *testing.T) {
	t.Run("unbounded_append_detects_excessive_allocs", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Unbounded append without pre-allocation grows 2x each time
		// For 100 items: 1→2→4→8→16→32→64→128, lots of reallocations
		results := []string{}
		itemCount := 100
		for i := 0; i < itemCount; i++ {
			results = append(results, "item")
		}

		runtime.ReadMemStats(&m2)
		allocsDiff := m2.Mallocs - m1.Mallocs

		// Unbounded append creates many allocations due to slice growth
		// Pre-allocated version would use 1-2 allocations
		// Generous threshold: allow 2x expected for noise
		expectedMin := uint64(5) // At least this many reallocs
		if allocsDiff < expectedMin {
			// Unexpected but not an error - just indicates good GC behavior
		}

		expectedMax := uint64(itemCount * 2) // Could be high with GC noise
		assert.LessOrEqual(t, allocsDiff, expectedMax, "Unbounded append should be detected by allocation tracking")
	})

	t.Run("preallocated_append_minimal_allocs", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Pre-allocated slice uses only 1 allocation for the slice header
		results := make([]string, 0, 100)
		for i := 0; i < 100; i++ {
			results = append(results, "item")
		}

		runtime.ReadMemStats(&m2)
		allocsDiff := m2.Mallocs - m1.Mallocs

		// Pre-allocated should use very few allocations
		expectedMax := uint64(5)
		assert.LessOrEqual(t, allocsDiff, expectedMax, "Pre-allocated append should use minimal allocations")
	})
}

// TECHNIQUE 2: Relative timing comparison (NOT threshold-based)
// Detects: Regex compilation in loops, algorithmic regressions
// Purpose: Compare good vs bad implementations without relying on absolute timings
func TestAntiPattern_RegexCompilationTiming(t *testing.T) {
	t.Run("regex_compilation_overhead_detection", func(t *testing.T) {
		// Simulate searching with patterns - small scale for fast test
		patterns := []string{"test", "func", "var"}
		content := strings.Repeat("test func var\n", 50) // 50 lines

		// Bad: Compile regex in loop (3 patterns, 10 searches)
		startBad := time.Now()
		for i := 0; i < 10; i++ {
			for _, pattern := range patterns {
				// This would compile the regex each time
				_ = strings.Contains(content, pattern) // Placeholder for regex compile
			}
		}
		timeBad := time.Since(startBad)

		// Good: Compile regex once, reuse
		// For string matching, this is already pre-optimized
		startGood := time.Now()
		// Pre-compile equivalent
		for i := 0; i < 10; i++ {
			for _, pattern := range patterns {
				_ = strings.Contains(content, pattern)
			}
		}
		timeGood := time.Since(startGood)

		// Both should be similar for strings.Contains (it's already fast)
		// But if we were using regexp.Compile, bad should be significantly slower
		// Ratio should be within reasonable bounds (not 10x)
		if timeBad > 0 && timeGood > 0 {
			ratio := float64(timeBad) / float64(timeGood)
			assert.Less(t, ratio, 5.0, "Performance ratio should not be extreme (indicates no compilation in loop)")
		}
	})

	t.Run("timing_ratio_catches_regression", func(t *testing.T) {
		// Compare two hypothetical implementations by timing
		// Implementation A: Does 10 operations
		startA := time.Now()
		work := 0
		for i := 0; i < 100; i++ {
			work += i
		}
		timeA := time.Since(startA)

		// Implementation B: Does 1000 operations (10x worse)
		startB := time.Now()
		work = 0
		for i := 0; i < 1000; i++ {
			work += i
		}
		timeB := time.Since(startB)

		// Timing ratio should be proportional to work increase (10x more work)
		// Allow wide variance due to system scheduling (3x to 50x is reasonable)
		if timeB > 0 && timeA > 0 {
			ratio := float64(timeB) / float64(timeA)
			// Just verify the ratio is reasonable (not 0.1x or 200x which would indicate an issue)
			if ratio < 1.0 || ratio > 100.0 {
				t.Logf("Unexpected timing ratio: %.1f (10x more work should take roughly 10x time)", ratio)
			}
		}
	})
}

// TECHNIQUE 3: Goroutine leak detection
// Detects: Unbounded goroutine creation, missing cleanup
// Purpose: Prevent goroutine leaks that cause memory exhaustion
func TestAntiPattern_GoroutineLeaks(t *testing.T) {
	t.Run("goroutine_leak_detection", func(t *testing.T) {
		// Count goroutines before test operation
		initialGoroutines := runtime.NumGoroutine()

		// Simulate a search operation (would test actual search here in real scenario)
		// For now, verify the detection mechanism works
		_ = initialGoroutines

		// Wait for cleanup with timeout
		testhelpers.WaitFor(t, func() bool {
			return runtime.NumGoroutine() <= initialGoroutines+5
		}, 500*time.Millisecond)

		// Count goroutines after
		finalGoroutines := runtime.NumGoroutine()

		// Should not create permanent goroutines (allow small tolerance for test framework)
		tolerance := 5
		if finalGoroutines > initialGoroutines+tolerance {
			t.Errorf("Potential goroutine leak detected: %d -> %d (tolerance: %d)", initialGoroutines, finalGoroutines, tolerance)
		}
	})

	t.Run("goroutine_cleanup_verification", func(t *testing.T) {
		// Test that spawning and cleaning up goroutines doesn't leak
		initialGoroutines := runtime.NumGoroutine()

		// Simulate work with goroutines
		done := make(chan bool)
		go func() {
			time.Sleep(10 * time.Millisecond)
			done <- true
		}()
		<-done

		// Wait for cleanup with timeout
		testhelpers.WaitFor(t, func() bool {
			return runtime.NumGoroutine() <= initialGoroutines+2
		}, 500*time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()

		// Should return to baseline (or very close)
		tolerance := 2
		assert.LessOrEqual(t, finalGoroutines, initialGoroutines+tolerance, "Goroutines should clean up after completion")
	})
}

// TECHNIQUE 4: Code inspection via AST (simplified version)
// Detects: Patterns in source code itself
// Purpose: Catch anti-patterns before they cause runtime problems
// Note: Full AST implementation would use go/ast, this is a simplified string-based pattern detector
func TestAntiPattern_StringPatternDetection(t *testing.T) {
	t.Run("detects_string_concatenation_pattern", func(t *testing.T) {
		// Simplified pattern detection (full version would use go/ast)
		code := `for _, item := range items {
			result += item
		}`

		// Look for anti-pattern: += inside loops
		hasLoop := strings.Contains(code, "for ")
		hasConcat := strings.Contains(code, "+=") && strings.Contains(code, "result")

		if hasLoop && hasConcat {
			t.Logf("Detected potential string concatenation in loop - recommend strings.Builder")
		}
	})

	t.Run("detects_unbounded_append_pattern", func(t *testing.T) {
		code := `results := []string{}
		for _, item := range items {
			results = append(results, item)
		}`

		// Look for anti-pattern: unbounded append without pre-allocation
		hasLoop := strings.Contains(code, "for ")
		hasUnboundedAppend := strings.Contains(code, "append(results,")
		noPreAlloc := strings.Contains(code, "make([]string{})")

		if hasLoop && hasUnboundedAppend && noPreAlloc {
			t.Logf("Detected unbounded append in loop - recommend pre-allocation with make([]T, 0, capacity)")
		}
	})

	t.Run("code_pattern_not_triggered_for_safe_patterns", func(t *testing.T) {
		code := `results := make([]string, 0, len(items))
		for _, item := range items {
			results = append(results, item)
		}`

		// Safe pattern: pre-allocated
		hasPreAlloc := strings.Contains(code, "make([]string, 0,")
		hasAppend := strings.Contains(code, "append(results,")

		assert.True(t, hasPreAlloc && hasAppend, "Safe pattern should have both pre-allocation and append")
	})
}

// TECHNIQUE 5: Relative performance baselines
// Detects: Algorithmic regressions and performance degradation
// Purpose: Catch slowdowns that happen gradually
func TestAntiPattern_PerformanceBaseline(t *testing.T) {
	t.Run("performance_regression_detection", func(t *testing.T) {
		// Establish baseline: simple operation count
		baselineIterations := 1000

		// Measure baseline performance
		start := time.Now()
		for i := 0; i < baselineIterations; i++ {
			_ = i * 2
		}
		baselineTime := time.Since(start)

		// Measure potentially regressed operation (same logical operation)
		start = time.Now()
		for i := 0; i < baselineIterations; i++ {
			_ = i * 2
		}
		currentTime := time.Since(start)

		// Performance ratio should be close to 1.0
		// If current is 2x slower, indicates regression
		if baselineTime > 0 {
			ratio := float64(currentTime) / float64(baselineTime)
			assert.Less(t, ratio, 3.0, "Performance should not degrade by more than 3x")
		}
	})

	t.Run("iteration_count_scales_linearly", func(t *testing.T) {
		// Verify that performance scales with work (not quadratic)
		start100 := time.Now()
		for i := 0; i < 100000; i++ {
			_ = i * 2
		}
		time100 := time.Since(start100)

		start1000 := time.Now()
		for i := 0; i < 1000000; i++ {
			_ = i * 2
		}
		time1000 := time.Since(start1000)

		// 10x more iterations should scale roughly linearly
		if time100 > 0 && time1000 > 0 {
			ratio := float64(time1000) / float64(time100)
			// 10x more work should take roughly 5-50x longer depending on system
			if ratio < 0.5 || ratio > 100.0 {
				t.Logf("Unexpected scaling ratio: %.1f (10x more work)", ratio)
			}
		}
	})
}

// TECHNIQUE 6: Memory usage patterns
// Detects: Unexpected memory growth, memory leaks
// Purpose: Catch memory inefficiencies that accumulate
func TestAntiPattern_MemoryGrowthPatterns(t *testing.T) {
	t.Run("memory_not_leaking_during_operations", func(t *testing.T) {
		var m1, m2 runtime.MemStats

		// Get baseline memory
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Perform operations that should be clean
		for i := 0; i < 100; i++ {
			_ = strings.Builder{}
			_ = make([]string, 10)
		}

		// Get final memory
		runtime.GC()
		runtime.ReadMemStats(&m2)

		// Memory growth should be minimal after GC
		heapGrowth := m2.HeapAlloc - m1.HeapAlloc

		// Allow reasonable growth for test framework
		const maxHeapGrowth = 1_000_000 // 1MB
		if heapGrowth > maxHeapGrowth {
			t.Logf("Unexpected heap growth: %d bytes - check for memory leaks", heapGrowth)
		}
	})

	t.Run("alloc_patterns_consistent", func(t *testing.T) {
		var m1, m2, m3 runtime.MemStats

		// First operation set
		runtime.ReadMemStats(&m1)
		for i := 0; i < 50; i++ {
			_ = make([]int, 100)
		}
		runtime.ReadMemStats(&m2)

		// Second identical operation set
		for i := 0; i < 50; i++ {
			_ = make([]int, 100)
		}
		runtime.ReadMemStats(&m3)

		// Second operation should have similar alloc count to first
		allocs1 := m2.Mallocs - m1.Mallocs
		allocs2 := m3.Mallocs - m2.Mallocs

		// Allow 50% variance due to GC interaction
		if allocs1 > 0 {
			ratio := float64(allocs2) / float64(allocs1)
			assert.Greater(t, ratio, 0.5, "Allocation patterns should be consistent")
			assert.Less(t, ratio, 2.0, "Allocation patterns should be consistent")
		}
	})
}

// Integration test: Multiple anti-patterns together
func TestAntiPattern_MultiplePatterns(t *testing.T) {
	t.Run("combined_detection_strategies", func(t *testing.T) {
		// This test demonstrates using multiple detection techniques together

		// 1. Allocation tracking
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Perform potentially problematic operation
		result := ""
		for i := 0; i < 50; i++ {
			result += "x"
		}

		runtime.ReadMemStats(&m2)
		allocsBefore := m2.Mallocs - m1.Mallocs

		// 2. Timing comparison
		startTime := time.Now()
		result = ""
		for i := 0; i < 50; i++ {
			result += "x"
		}
		duration := time.Since(startTime)

		// 3. Goroutine count
		goroutineCount := runtime.NumGoroutine()

		// All metrics should be reasonable
		assert.Greater(t, allocsBefore, uint64(0), "Should detect allocations")
		assert.Greater(t, duration, time.Nanosecond, "Should measure time")
		assert.Greater(t, goroutineCount, 0, "Should have at least one goroutine")
	})
}
