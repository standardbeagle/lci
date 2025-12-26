package alloc

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestNewSlabAllocator tests basic allocator creation
func TestNewSlabAllocator(t *testing.T) {
	configs := []SlabTierConfig{
		{Capacity: 8, Weight: 1.0},
		{Capacity: 16, Weight: 1.0},
	}

	sa := NewSlabAllocator[int](configs)

	if sa == nil {
		t.Fatal("Expected non-nil allocator")
	}

	if len(sa.pools) != 2 {
		t.Errorf("Expected 2 pools, got %d", len(sa.pools))
	}

	// Check pool capacities
	if sa.pools[0].capacity != 8 {
		t.Errorf("Expected first pool capacity 8, got %d", sa.pools[0].capacity)
	}

	if sa.pools[1].capacity != 16 {
		t.Errorf("Expected second pool capacity 16, got %d", sa.pools[1].capacity)
	}
}

// TestNewSlabAllocatorWithDefaults tests default allocator creation
func TestNewSlabAllocatorWithDefaults(t *testing.T) {
	sa := NewSlabAllocatorWithDefaults[string]()

	if sa == nil {
		t.Fatal("Expected non-nil allocator")
	}

	if len(sa.pools) != len(DefaultTierConfigs) {
		t.Errorf("Expected %d pools, got %d", len(DefaultTierConfigs), len(sa.pools))
	}

	// Verify default configurations
	for i, config := range DefaultTierConfigs {
		if sa.pools[i].capacity != config.Capacity {
			t.Errorf("Pool %d: expected capacity %d, got %d", i, config.Capacity, sa.pools[i].capacity)
		}
	}
}

// TestNewTrigramSlabAllocator tests trigram-specific allocator
func TestNewTrigramSlabAllocator(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()

	if sa == nil {
		t.Fatal("Expected non-nil allocator")
	}

	if len(sa.pools) != len(TrigramTierConfigs) {
		t.Errorf("Expected %d pools, got %d", len(TrigramTierConfigs), len(sa.pools))
	}

	// Verify trigram-specific configurations
	for i, config := range TrigramTierConfigs {
		if sa.pools[i].capacity != config.Capacity {
			t.Errorf("Pool %d: expected capacity %d, got %d", i, config.Capacity, sa.pools[i].capacity)
		}
	}
}

// TestSlabAllocatorBasicOperations tests basic Get/Put operations
func TestSlabAllocatorBasicOperations(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()

	// Test getting different sizes
	slice1 := sa.Get(5)
	if cap(slice1) < 5 {
		t.Errorf("Expected capacity >= 5, got %d", cap(slice1))
	}
	if len(slice1) != 0 {
		t.Errorf("Expected length 0, got %d", len(slice1))
	}

	slice2 := sa.Get(20)
	if cap(slice2) < 20 {
		t.Errorf("Expected capacity >= 20, got %d", cap(slice2))
	}

	// Test putting slices back
	sa.Put(slice1)
	sa.Put(slice2)

	// Test getting slices from pools (should reuse)
	slice3 := sa.Get(5)
	if cap(slice3) < 5 {
		t.Errorf("Expected capacity >= 5, got %d", cap(slice3))
	}

	// Test edge cases
	zeroSlice := sa.Get(0)
	if cap(zeroSlice) != 0 {
		t.Errorf("Expected capacity 0, got %d", cap(zeroSlice))
	}

	negativeSlice := sa.Get(-1)
	if cap(negativeSlice) != 0 {
		t.Errorf("Expected capacity 0, got %d", cap(negativeSlice))
	}

	// Test putting nil/empty slices
	sa.Put(nil)
	sa.Put([]int{})
	sa.Put(make([]int, 0))
}

// TestSlabAllocatorStats tests statistics tracking
func TestSlabAllocatorStats(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()

	// Reset stats
	sa.ResetStats()
	stats := sa.GetStats()
	if stats.Allocations != 0 {
		t.Errorf("Expected 0 allocations, got %d", stats.Allocations)
	}

	// Perform some operations
	slice1 := sa.Get(5) // Should allocate
	slice2 := sa.Get(5) // Should allocate

	sa.Put(slice1)      // Should reuse
	slice3 := sa.Get(5) // Should reuse

	sa.Put(slice2)
	sa.Put(slice3)

	// Check stats - we should have some activity
	stats = sa.GetStats()
	if stats.Reuses == 0 {
		t.Error("Expected > 0 reuses")
	}

	if stats.PoolHits == 0 {
		t.Error("Expected > 0 pool hits")
	}

	t.Logf("Stats: %+v", stats)
}

// TestSlabAllocatorConcurrency tests concurrent access
func TestSlabAllocatorConcurrency(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()
	sa.ResetStats()

	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()

	// Start many goroutines doing Get/Put operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Get slices of various sizes
				size := (id*numOperations+j)%128 + 1
				slice := sa.Get(size)

				// Use the slice (add some elements)
				for k := 0; k < size/2; k++ {
					slice = append(slice, k)
				}

				// Put it back
				sa.Put(slice[:0]) // Reset to length 0
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	stats := sa.GetStats()
	t.Logf("Concurrency test completed in %v", elapsed)
	t.Logf("Stats: %+v", stats)

	// Verify we had some pool activity
	if stats.PoolHits == 0 {
		t.Error("Expected > 0 pool hits in concurrent test")
	}

	// Verify no race conditions occurred
	if stats.Allocations < 0 || stats.Reuses < 0 {
		t.Error("Statistics should not be negative")
	}
}

// TestSlabAllocatorGrowSlice tests the GrowSlice functionality
func TestSlabAllocatorGrowSlice(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()

	// Test growing from small slice
	slice := make([]int, 3)
	slice[0], slice[1], slice[2] = 1, 2, 3

	grown := sa.GrowSlice(slice, 10)
	if cap(grown) < 13 {
		t.Errorf("Expected capacity >= 13, got %d", cap(grown))
	}
	if len(grown) != 3 {
		t.Errorf("Expected length 3, got %d", len(grown))
	}
	if grown[0] != 1 || grown[1] != 2 || grown[2] != 3 {
		t.Error("Original elements not preserved")
	}

	// Test growing when no growth needed
	noGrowth := sa.GrowSlice(grown, 0)
	if cap(noGrowth) != cap(grown) {
		t.Error("Expected no capacity change when no growth needed")
	}

	// Test growing with negative additional capacity
	noGrowth = sa.GrowSlice(grown, -5)
	if cap(noGrowth) != cap(grown) {
		t.Error("Expected no capacity change with negative additional capacity")
	}
}

// TestSlabAllocatorGetWithCapacity tests GetWithCapacity functionality
func TestSlabAllocatorGetWithCapacity(t *testing.T) {
	sa := NewTrigramSlabAllocator[string]()

	// Test exact capacity
	slice := sa.GetWithCapacity(5, 10)
	if cap(slice) < 15 {
		t.Errorf("Expected capacity >= 15, got %d", cap(slice))
	}
	if len(slice) != 0 {
		t.Errorf("Expected length 0, got %d", len(slice))
	}

	// Test capacity larger than available pools
	slice2 := sa.GetWithCapacity(100, 1000)
	if cap(slice2) < 1100 {
		t.Errorf("Expected capacity >= 1100, got %d", cap(slice2))
	}
}

// TestSlabAllocatorEdgeCases tests edge cases and error conditions
func TestSlabAllocatorEdgeCases(t *testing.T) {
	sa := NewSlabAllocatorWithDefaults[int]()

	// Test with very large capacity
	largeSlice := sa.Get(1000000)
	if cap(largeSlice) < 1000000 {
		t.Errorf("Expected capacity >= 1000000, got %d", cap(largeSlice))
	}

	// Test putting back slices with different capacities
	sa.Put(largeSlice) // Should be discarded (no matching pool)

	// Test with custom type
	type CustomStruct struct {
		ID   int
		Name string
	}

	sa2 := NewSlabAllocatorWithDefaults[CustomStruct]()
	customSlice := sa2.Get(10)
	if len(customSlice) != 0 {
		t.Errorf("Expected length 0, got %d", len(customSlice))
	}
	if cap(customSlice) < 10 {
		t.Errorf("Expected capacity >= 10, got %d", cap(customSlice))
	}
}

// TestSlabAllocatorPoolEfficiency tests pool efficiency metrics
func TestSlabAllocatorPoolEfficiency(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()
	sa.ResetStats()

	// Simulate a realistic workload pattern
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		// Most requests are small (simulate trigram distribution)
		var size int
		switch {
		case i < iterations*40/100: // 40% are very small
			size = i%8 + 1
		case i < iterations*80/100: // 40% are small
			size = i%16 + 1
		case i < iterations*95/100: // 15% are medium
			size = i%64 + 1
		default: // 5% are large
			size = i%128 + 1
		}

		slice := sa.Get(size)
		sa.Put(slice)
	}

	stats := sa.GetStats()
	t.Logf("Efficiency test stats: %+v", stats)

	// Verify pool efficiency
	totalRequests := stats.Allocations + stats.Reuses
	if totalRequests == 0 {
		t.Error("Expected some requests to be made")
	}

	poolHitRate := float64(stats.PoolHits) / float64(totalRequests)
	t.Logf("Pool hit rate: %.2f%%", poolHitRate*100)

	// We should have reasonable pool efficiency
	if poolHitRate < 0.1 { // At least 10% hit rate
		t.Errorf("Pool hit rate too low: %.2f%%", poolHitRate*100)
	}
}

// TestSlabAllocatorMemoryUsage tests memory usage patterns
func TestSlabAllocatorMemoryUsage(t *testing.T) {
	sa := NewTrigramSlabAllocator[int]()

	// Force GC to get baseline
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform many allocations
	const numAllocations = 10000
	slices := make([][]int, 0, numAllocations)

	for i := 0; i < numAllocations; i++ {
		slice := sa.Get(i%128 + 1)
		slices = append(slices, slice)
	}

	// Return all slices to pools
	for _, slice := range slices {
		sa.Put(slice)
	}
	slices = nil

	// Force GC again
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Check memory usage (handle potential overflow)
	var allocDiff uint64
	if m2.Alloc > m1.Alloc {
		allocDiff = m2.Alloc - m1.Alloc
	}
	t.Logf("Memory usage difference: %d bytes", allocDiff)

	// Memory usage should be reasonable (slab allocator pools keep memory for reuse)
	// Allow up to 20MB overhead for pool structures and retained memory
	if allocDiff > 20*1024*1024 {
		t.Errorf("Memory usage too high: %d bytes", allocDiff)
	}

	// Check that pools are actually being used
	stats := sa.GetStats()
	if stats.PoolHits == 0 {
		t.Error("Expected some pool hits")
	}
}

// BenchmarkSlabAllocatorVsDirect benchmarks slab allocator vs direct allocation
func BenchmarkSlabAllocatorVsDirect(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			size := i%128 + 1
			slice := make([]int, 0, size)
			// Simulate some usage
			for j := 0; j < size/2; j++ {
				slice = append(slice, j)
			}
			// Slice goes out of scope and gets GC'd
		}
	})

	b.Run("SlabAllocator", func(b *testing.B) {
		sa := NewTrigramSlabAllocator[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			size := i%128 + 1
			slice := sa.Get(size)
			// Simulate some usage
			for j := 0; j < size/2; j++ {
				slice = append(slice, j)
			}
			sa.Put(slice[:0])
		}
	})
}

// BenchmarkSlabAllocatorConcurrency benchmarks concurrent performance
func BenchmarkSlabAllocatorConcurrency(b *testing.B) {
	sa := NewTrigramSlabAllocator[int]()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			size := i%128 + 1
			slice := sa.Get(size)
			slice = append(slice, i)
			sa.Put(slice[:0])
			i++
		}
	})
}
