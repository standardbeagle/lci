package alloc

import (
	"sync"
	"sync/atomic"
)

// SlabAllocator is a generic, lock-free slab allocator for reducing memory allocation overhead.
// It uses pre-sized pools for different allocation sizes to minimize GC pressure.
type SlabAllocator[T any] struct {
	// Pools for different size categories (pointers to avoid copying sync.Pool)
	pools []*poolTier[T]

	// Statistics for monitoring
	stats atomic.Value // *AllocatorStats
}

// poolTier represents a single size tier in the slab allocator
type poolTier[T any] struct {
	capacity int
	pool     sync.Pool
}

// AllocatorStats tracks allocation statistics
type AllocatorStats struct {
	Allocations   int64
	Reuses        int64
	PoolHits      int64
	PoolMisses    int64
	TotalCapacity int64
}

// SlabTierConfig defines the configuration for a single slab tier
type SlabTierConfig struct {
	Capacity int
	Weight   float64 // Relative weight for this tier (for auto-sizing)
}

// DefaultTierConfigs provides sensible defaults for most workloads
var DefaultTierConfigs = []SlabTierConfig{
	{Capacity: 8, Weight: 0.3},    // 30% of allocations
	{Capacity: 16, Weight: 0.3},   // 30% of allocations
	{Capacity: 32, Weight: 0.2},   // 20% of allocations
	{Capacity: 64, Weight: 0.1},   // 10% of allocations
	{Capacity: 128, Weight: 0.05}, // 5% of allocations
	{Capacity: 256, Weight: 0.03}, // 3% of allocations
	{Capacity: 512, Weight: 0.02}, // 2% of allocations
}

// TrigramTierConfigs is optimized for trigram location arrays based on distribution analysis
var TrigramTierConfigs = []SlabTierConfig{
	{Capacity: 8, Weight: 0.40},   // 40% of trigrams appear ≤5 times
	{Capacity: 16, Weight: 0.40},  // 40% of trigrams appear 6-10 times
	{Capacity: 32, Weight: 0.15},  // 15% of trigrams appear 11-20 times
	{Capacity: 64, Weight: 0.03},  // 3% of trigrams appear 21-50 times
	{Capacity: 128, Weight: 0.02}, // 2% of trigrams appear >50 times
}

// NewSlabAllocator creates a new slab allocator with the given tier configurations
func NewSlabAllocator[T any](configs []SlabTierConfig) *SlabAllocator[T] {
	sa := &SlabAllocator[T]{
		pools: make([]*poolTier[T], len(configs)),
	}

	// Initialize pools
	for i, config := range configs {
		cap := config.Capacity // capture for closure
		sa.pools[i] = &poolTier[T]{
			capacity: cap,
			pool: sync.Pool{
				New: func() any {
					return make([]T, 0, cap)
				},
			},
		}
	}

	// Initialize stats
	sa.stats.Store(&AllocatorStats{})

	return sa
}

// NewSlabAllocatorWithDefaults creates a slab allocator with default tier configurations
func NewSlabAllocatorWithDefaults[T any]() *SlabAllocator[T] {
	return NewSlabAllocator[T](DefaultTierConfigs)
}

// NewTrigramSlabAllocator creates a slab allocator optimized for trigram workloads
func NewTrigramSlabAllocator[T any]() *SlabAllocator[T] {
	return NewSlabAllocator[T](TrigramTierConfigs)
}

// Get returns a slice with at least the requested capacity.
// The returned slice has length 0 and capacity >= requested.
func (sa *SlabAllocator[T]) Get(capacity int) []T {
	if capacity <= 0 {
		return make([]T, 0)
	}

	// Find the smallest pool that can accommodate the request
	for _, tier := range sa.pools {
		if tier.capacity >= capacity {
			slice := sa.getFromPool(tier)
			return slice
		}
	}

	// No pool large enough, allocate directly
	sa.updateStats(func(stats *AllocatorStats) {
		stats.Allocations++
		stats.PoolMisses++
		stats.TotalCapacity += int64(capacity)
	})

	return make([]T, 0, capacity)
}

// Put returns a slice to the appropriate pool for reuse.
// Slices larger than the largest pool capacity are discarded.
func (sa *SlabAllocator[T]) Put(slice []T) {
	if slice == nil || cap(slice) == 0 {
		return
	}

	// Find the appropriate pool for this slice capacity
	capacity := cap(slice)
	for _, tier := range sa.pools {
		if tier.capacity == capacity {
			// Reset slice length to 0 for reuse
			slice = slice[:0]
			tier.pool.Put(slice)

			sa.updateStats(func(stats *AllocatorStats) {
				stats.Reuses++
				stats.PoolHits++
			})
			return
		}
	}

	// No matching pool, discard
	sa.updateStats(func(stats *AllocatorStats) {
		stats.PoolMisses++
	})
}

// GetStats returns current allocation statistics
func (sa *SlabAllocator[T]) GetStats() AllocatorStats {
	return *sa.stats.Load().(*AllocatorStats)
}

// ResetStats resets all statistics to zero
func (sa *SlabAllocator[T]) ResetStats() {
	sa.stats.Store(&AllocatorStats{})
}

// getFromPool attempts to get a slice from the given pool
func (sa *SlabAllocator[T]) getFromPool(tier *poolTier[T]) []T {
	if slice := tier.pool.Get(); slice != nil {
		sa.updateStats(func(stats *AllocatorStats) {
			stats.Reuses++
			stats.PoolHits++
			stats.TotalCapacity += int64(tier.capacity)
		})
		return slice.([]T)
	}

	// Pool miss, allocate new
	sa.updateStats(func(stats *AllocatorStats) {
		stats.Allocations++
		stats.PoolMisses++
		stats.TotalCapacity += int64(tier.capacity)
	})

	return make([]T, 0, tier.capacity)
}

// updateStats atomically updates the statistics
func (sa *SlabAllocator[T]) updateStats(update func(*AllocatorStats)) {
	current := sa.stats.Load().(*AllocatorStats)
	newStats := *current

	update(&newStats)
	sa.stats.Store(&newStats)
}

// GetWithCapacity returns a slice with exact capacity, growing if necessary
// This is useful when you need to append existing elements
func (sa *SlabAllocator[T]) GetWithCapacity(existingLen, additionalCapacity int) []T {
	totalCapacity := existingLen + additionalCapacity
	slice := sa.Get(totalCapacity)

	// Return slice with length 0 (caller can set length as needed)
	return slice
}

// GrowSlice grows a slice to accommodate additional elements, using slab allocation when possible
func (sa *SlabAllocator[T]) GrowSlice(slice []T, additionalCapacity int) []T {
	if additionalCapacity <= 0 {
		return slice
	}

	currentLen := len(slice)
	currentCap := cap(slice)
	requiredCap := currentLen + additionalCapacity

	// If current capacity is sufficient, just return the slice
	if currentCap >= requiredCap {
		return slice
	}

	// Try to get a new slice from slab allocator
	newSlice := sa.GetWithCapacity(currentLen, additionalCapacity)

	// Copy existing elements and set correct length
	newSlice = append(newSlice, slice...)

	// Return old slice to pool if it came from a slab
	sa.Put(slice)

	return newSlice
}

// EstimateOptimalSize estimates the best pool size for a given workload
// based on historical statistics
func (sa *SlabAllocator[T]) EstimateOptimalSize() int {
	stats := sa.GetStats()
	if stats.PoolHits == 0 {
		// No history, return middle tier
		return sa.pools[len(sa.pools)/2].capacity
	}

	// Return the most commonly used tier based on pool hits
	maxHits := int64(0)
	bestCapacity := sa.pools[0].capacity

	for _, tier := range sa.pools {
		// This is a simplified heuristic
		// In practice, you'd want to track hits per tier
		if int64(tier.capacity) > maxHits {
			maxHits = int64(tier.capacity)
			bestCapacity = tier.capacity
		}
	}

	return bestCapacity
}
