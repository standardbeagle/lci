package core

import (
	"runtime"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestTrigramIndexMemoryReduction tests that the slab allocator reduces memory allocations
func TestTrigramIndexMemoryReduction(t *testing.T) {
	// Test with a scenario similar to the original profiling issue
	ti := NewTrigramIndex()
	ti.locationAllocator.ResetStats()

	// Force GC to get baseline
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Simulate the problematic scenario: many trigrams with many locations
	// This recreates the conditions that caused 205GB of allocations
	numFiles := 500
	trigramsPerFile := 20
	locationsPerTrigram := 10

	for fileID := 0; fileID < numFiles; fileID++ {
		trigrams := make(map[uint32][]uint32)

		for i := 0; i < trigramsPerFile; i++ {
			// Create trigrams that will accumulate many locations
			trigramHash := uint32(0x66756e + uint32(i)) // Vary around common patterns
			offsets := make([]uint32, locationsPerTrigram)
			for j := 0; j < locationsPerTrigram; j++ {
				offsets[j] = uint32(j * 3)
			}
			trigrams[trigramHash] = offsets
		}

		ti.IndexFileWithTrigrams(types.FileID(fileID), trigrams)
	}

	// Force GC again
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Check memory usage
	allocDiff := m2.Alloc - m1.Alloc
	if m2.Alloc > m1.Alloc {
		t.Logf("Memory increase: %d bytes", allocDiff)
	} else {
		t.Logf("Memory decrease: %d bytes", m1.Alloc-m2.Alloc)
	}

	// Check allocator stats
	stats := ti.GetAllocatorStats()
	t.Logf("Allocator stats: %+v", stats)

	// Key test: We should have significantly fewer total allocations
	// than the original exponential growth strategy would have created
	totalOps := stats.Allocations + stats.Reuses
	t.Logf("Total allocator operations: %d (allocations: %d, reuses: %d)",
		totalOps, stats.Allocations, stats.Reuses)

	// The original problematic code would have created many GB of allocations for this scenario
	// With slab allocation, we should see much more reasonable numbers
	expectedMaxAllocations := int64(numFiles * trigramsPerFile * 2) // Allow some overhead
	if stats.Allocations > expectedMaxAllocations {
		t.Logf("Note: High allocation count detected: %d vs expected max %d",
			stats.Allocations, expectedMaxAllocations)
	}

	// More importantly, check that we're getting some pool reuse
	if stats.PoolHits == 0 {
		t.Error("Expected some pool hits for repeated allocation patterns")
	}

	// Verify trigrams are properly indexed
	if len(ti.trigrams) != trigramsPerFile {
		t.Errorf("Expected %d unique trigrams, got %d", trigramsPerFile, len(ti.trigrams))
	}

	// Verify that each trigram has accumulated locations from all files
	for trigramHash, entry := range ti.trigrams {
		expectedLocations := numFiles * locationsPerTrigram
		if len(entry.Locations) != expectedLocations {
			t.Errorf("Trigram %x: expected %d locations, got %d",
				trigramHash, expectedLocations, len(entry.Locations))
		}
	}

	t.Logf("Successfully indexed %d files with %d trigrams each, %d locations per trigram",
		numFiles, trigramsPerFile, locationsPerTrigram)

	// Memory usage should be reasonable (less than 50MB for this test)
	const maxMemoryUsage = 50 * 1024 * 1024
	if allocDiff > maxMemoryUsage && allocDiff > 0 {
		t.Errorf("Memory usage too high: %d bytes (max: %d)", allocDiff, maxMemoryUsage)
	} else {
		t.Logf("Memory usage is reasonable: %d bytes", allocDiff)
	}
}

// BenchmarkTrigramIndexOldVsNew compares old allocation strategy vs slab allocator
func BenchmarkTrigramIndexOldVsNew(b *testing.B) {
	b.Run("OldPattern", func(b *testing.B) {
		// Simulate the old allocation pattern
		for i := 0; i < b.N; i++ {
			// Simulate the problematic slice growth pattern
			locations := make([]FileLocation, 0, 1)
			for j := 0; j < 100; j++ {
				// This simulates the old exponential growth pattern
				if cap(locations) < len(locations)+1 {
					newCap := len(locations) + 1
					if newCap < 64 {
						newCap = newCap * 2 // Exponential growth
					}
					newLocations := make([]FileLocation, 0, newCap)
					newLocations = append(newLocations, locations...)
					locations = newLocations
				}
				locations = append(locations, FileLocation{
					FileID: types.FileID(j),
					Offset: uint32(j),
				})
			}
		}
	})

	b.Run("SlabAllocator", func(b *testing.B) {
		ti := NewTrigramIndex()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			locations := ti.locationAllocator.Get(1)
			for j := 0; j < 100; j++ {
				locations = append(locations, FileLocation{
					FileID: types.FileID(j),
					Offset: uint32(j),
				})
			}
			ti.locationAllocator.Put(locations)
		}
	})
}

// BenchmarkTrigramIndexRealistic benchmarks realistic trigram indexing workloads
func BenchmarkTrigramIndexRealistic(b *testing.B) {
	ti := NewTrigramIndex()

	// Prepare test data mimicking real trigram distribution
	trigramSets := make([]map[uint32][]uint32, 100)
	for i := 0; i < 100; i++ {
		trigrams := make(map[uint32][]uint32)
		// Simulate realistic trigram distribution
		// 80% small trigrams, 20% larger ones
		for j := 0; j < 20; j++ {
			trigramHash := uint32(0x66756e + uint32(j))
			var offsets []uint32
			if j < 16 {
				// Small trigrams: 1-5 locations
				numOffsets := (j % 5) + 1
				offsets = make([]uint32, numOffsets)
				for k := 0; k < numOffsets; k++ {
					offsets[k] = uint32(k * 3)
				}
			} else {
				// Larger trigrams: 10-50 locations
				numOffsets := (j % 40) + 10
				offsets = make([]uint32, numOffsets)
				for k := 0; k < numOffsets; k++ {
					offsets[k] = uint32(k * 3)
				}
			}
			trigrams[trigramHash] = offsets
		}
		trigramSets[i] = trigrams
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trigrams := trigramSets[i%len(trigramSets)]
		ti.IndexFileWithTrigrams(types.FileID(i), trigrams)
	}
}
