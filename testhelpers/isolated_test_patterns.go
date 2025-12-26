package testhelpers

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/standardbeagle/lci/internal/core"
)

// IsolatedTrigramIndexTest provides a pattern for testing TrigramIndex in isolation
func IsolatedTrigramIndexTest(t *testing.T, testName string, testFunc func(t *testing.T, index *core.TrigramIndex)) {
	ConcurrentSafeTest(t, testName, func(t *testing.T) {
		// Disable performance demo that interferes with tests
		DisablePerformanceDemo()

		// Create isolated index
		index := core.NewTrigramIndex()
		// defer index.Shutdown() // TODO: Implement Shutdown method if needed
		_ = index

		// Run the test function
		testFunc(t, index)
	})
}

// IsolatedIndexCoordinatorTest provides a pattern for testing IndexCoordinator in isolation
func IsolatedIndexCoordinatorTest(t *testing.T, testName string, testFunc func(t *testing.T, coordinator *core.DefaultIndexCoordinator)) {
	ConcurrentSafeTest(t, testName, func(t *testing.T) {
		// Disable performance demo
		DisablePerformanceDemo()

		// Create isolated coordinator (returns interface, assert to concrete type)
		coordinator := core.NewIndexCoordinator().(*core.DefaultIndexCoordinator)
		// defer coordinator.Shutdown() // TODO: Implement Shutdown method if needed
		_ = coordinator

		// Run the test function
		testFunc(t, coordinator)
	})
}

// IsolatedFileContentStoreTest provides a pattern for testing FileContentStore in isolation
func IsolatedFileContentStoreTest(t *testing.T, testName string, testFunc func(t *testing.T, store *core.FileContentStore)) {
	ConcurrentSafeTest(t, testName, func(t *testing.T) {
		// Use a reasonable size for tests to avoid memory issues
		store := core.NewFileContentStore() // TODO: Adjust based on actual constructor

		// Run the test function
		testFunc(t, store)
	})
}

// PropertyTestWrapper provides a pattern for property-based tests with proper isolation
func PropertyTestWrapper(t *testing.T, testName string, iterations int, testFunc func(t *testing.T, iteration int)) {
	ConcurrentSafeTest(t, testName, func(t *testing.T) {
		// Reduce iterations for property tests in isolation mode
		maxIterations := 100
		if iterations > maxIterations {
			iterations = maxIterations
		}

		for i := 0; i < iterations; i++ {
			t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
				testFunc(t, i)
			})
		}
	})
}

// PerformanceTestWrapper provides safe performance testing that doesn't interfere with other tests
func PerformanceTestWrapper(t *testing.T, testName string, testFunc func(t *testing.T)) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ConcurrentSafeTest(t, testName, func(t *testing.T) {
		// Force garbage collection before performance test
		runtime.GC()
		startMem := runtime.MemStats{}
		runtime.ReadMemStats(&startMem)

		// Run the performance test
		testFunc(t)

		// Check memory usage
		endMem := runtime.MemStats{}
		runtime.ReadMemStats(&endMem)
		memDiff := endMem.Alloc - startMem.Alloc

		// Warn if memory usage is excessive
		if memDiff > 100*1024*1024 { // 100MB
			t.Logf("WARNING: High memory usage in %s: %d MB", testName, memDiff/(1024*1024))
		}

		// Force cleanup
		runtime.GC()
	})
}

// ConcurrentTestWrapper provides a safe pattern for concurrent testing
func ConcurrentTestWrapper(t *testing.T, testName string, numGoroutines int, testFunc func(t *testing.T, goroutineID int)) {
	ConcurrentSafeTest(t, testName, func(t *testing.T) {
		var wg sync.WaitGroup
		results := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				// Capture any panics
				defer func() {
					if r := recover(); r != nil {
						results[goroutineID] = fmt.Errorf("panic in goroutine %d: %v", goroutineID, r)
					}
				}()

				// Run the test function
				t.Run(fmt.Sprintf("goroutine_%d", goroutineID), func(t *testing.T) {
					testFunc(t, goroutineID)
				})
			}(i)
		}

		wg.Wait()

		// Check for any errors
		for i, err := range results {
			if err != nil {
				t.Errorf("Goroutine %d failed: %v", i, err)
			}
		}
	})
}
