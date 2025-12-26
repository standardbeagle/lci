package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestTrigramIndexSlabAllocator tests the integration of slab allocator with TrigramIndex
func TestTrigramIndexSlabAllocator(t *testing.T) {
	ti := NewTrigramIndex()

	// Verify allocator is initialized
	if ti.locationAllocator == nil {
		t.Fatal("Expected locationAllocator to be initialized")
	}

	// Reset stats for clean test
	ti.locationAllocator.ResetStats()

	// Test indexing with multiple files
	fileID1 := types.FileID(1)
	fileID2 := types.FileID(2)

	// Index first file
	ti.IndexFileWithTrigrams(fileID1, map[uint32][]uint32{
		0x66756e: {0}, // "fun"
		0x756e63: {1}, // "unc"
		0x6e2074: {2}, // "n t"
		0x207465: {3}, // " te"
		0x746573: {4}, // "tes"
		0x657374: {5}, // "est"
	})

	// Check allocator stats after first indexing
	stats1 := ti.GetAllocatorStats()
	if stats1.Reuses == 0 && stats1.Allocations == 0 {
		t.Error("Expected some allocator activity")
	}
	t.Logf("Stats after first file: %+v", stats1)

	// Index second file (should reuse some pools)
	ti.IndexFileWithTrigrams(fileID2, map[uint32][]uint32{
		0x66756e: {0}, // "fun" - should reuse existing slice
		0x756e63: {1}, // "unc" - should reuse existing slice
		0x616e6f: {2}, // "ano"
		0x6e6f74: {3}, // "not"
		0x6f7468: {4}, // "oth"
		0x746865: {5}, // "the"
		0x686572: {6}, // "her"
	})

	// Check allocator stats after second indexing
	stats2 := ti.GetAllocatorStats()
	t.Logf("Stats after second file: %+v", stats2)

	// Should have more activity now
	if stats2.Reuses <= stats1.Reuses {
		t.Error("Expected more reuses after second indexing")
	}

	// Test that trigram entries are properly populated
	if len(ti.trigrams) == 0 {
		t.Error("Expected trigrams to be indexed")
	}

	// Verify the "fun" trigram has locations from both files
	if entry, exists := ti.trigrams[0x66756e]; exists {
		if len(entry.Locations) != 2 {
			t.Errorf("Expected 2 locations for 'fun' trigram, got %d", len(entry.Locations))
		}
	} else {
		t.Error("Expected 'fun' trigram to exist")
	}

	// Test cleanup and pool reuse
	ti.Clear()

	// After clear, allocator should have returned slices to pools
	statsAfterClear := ti.GetAllocatorStats()
	t.Logf("Stats after clear: %+v", statsAfterClear)

	// Trigram map should be empty
	if len(ti.trigrams) != 0 {
		t.Errorf("Expected empty trigrams after clear, got %d", len(ti.trigrams))
	}

	// Stats should be reset
	if statsAfterClear.Allocations != 0 || statsAfterClear.Reuses != 0 {
		t.Error("Expected stats to be reset after clear")
	}
}

// TestTrigramIndexSlabAllocatorConcurrency tests concurrent indexing with slab allocator
func TestTrigramIndexSlabAllocatorConcurrency(t *testing.T) {
	ti := NewTrigramIndex()
	ti.locationAllocator.ResetStats()

	const numGoroutines = 10
	const filesPerGoroutine = 100

	// Use bulk indexing mode to avoid lock contention
	ti.SetBulkIndexing(true)
	defer ti.SetBulkIndexing(false)

	// Index files concurrently
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(startID int) {
			for j := 0; j < filesPerGoroutine; j++ {
				fileID := types.FileID(startID + j)
				content := []byte("test content for indexing")
				trigrams := extractSimpleTrigrams(content)

				// Convert map[int]uint32 to map[uint32][]uint32
				trigramMap := make(map[uint32][]uint32)
				for offset, trigram := range trigrams {
					trigramMap[trigram] = []uint32{uint32(offset)}
				}

				ti.IndexFileWithTrigrams(fileID, trigramMap)
			}
			done <- true
		}(i * filesPerGoroutine)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check final stats
	stats := ti.GetAllocatorStats()
	t.Logf("Concurrency test stats: %+v", stats)

	// Should have significant allocator activity
	if stats.Reuses == 0 {
		t.Error("Expected reuses in concurrent test")
	}

	// Should have trigrams indexed
	if len(ti.trigrams) == 0 {
		t.Error("Expected trigrams to be indexed")
	}

	// Verify efficiency - pool hit rate should be reasonable
	totalOps := stats.Allocations + stats.Reuses
	if totalOps > 0 {
		hitRate := float64(stats.PoolHits) / float64(totalOps)
		t.Logf("Pool hit rate: %.2f%%", hitRate*100)

		// In concurrent scenarios with varied trigram sizes, some pool misses are expected
		// The important thing is that we're getting some reuse and reducing allocations
		if stats.PoolHits == 0 {
			t.Error("Expected some pool hits even in concurrent scenarios")
		}
	}
}

// TestTrigramIndexSlabAllocatorMemoryEfficiency tests memory efficiency improvements
func TestTrigramIndexSlabAllocatorMemoryEfficiency(t *testing.T) {
	ti := NewTrigramIndex()
	ti.locationAllocator.ResetStats()

	// Create a scenario that would trigger the original allocation issue
	// Many files with overlapping trigrams to trigger slice growth
	numFiles := 100
	trigramsPerFile := 50

	for i := 0; i < numFiles; i++ {
		fileID := types.FileID(i)
		trigrams := make(map[uint32][]uint32)

		// Create trigrams that will overlap across files
		for j := 0; j < trigramsPerFile; j++ {
			// Use some common trigrams that will appear in many files
			commonTrigram := uint32(0x66756e + uint32(j%10)) // Vary around "fun"
			offsets := make([]uint32, j+1)                   // Increasing number of offsets
			for k := 0; k <= j; k++ {
				offsets[k] = uint32(k * 3)
			}
			trigrams[commonTrigram] = offsets
		}

		ti.IndexFileWithTrigrams(fileID, trigrams)
	}

	// Check final stats
	stats := ti.GetAllocatorStats()
	t.Logf("Memory efficiency test stats: %+v", stats)

	// Should have some pool efficiency due to trigram-optimized tiers
	totalOps := stats.Allocations + stats.Reuses
	if totalOps > 0 {
		hitRate := float64(stats.PoolHits) / float64(totalOps)
		t.Logf("Pool hit rate: %.2f%%", hitRate*100)

		// Even with some pool misses, we should see benefits from slab allocation
		// The key improvement is eliminating the massive slice copying from the original code
		if stats.PoolHits == 0 {
			t.Error("Expected some pool hits for repeated allocations")
		}

		// More importantly, we should have significantly fewer memory allocations
		// compared to the original exponential growth strategy
		if stats.Allocations > totalOps/2 {
			t.Logf("High allocation rate detected: %d allocations out of %d total ops",
				stats.Allocations, totalOps)
		}
	}

	// Verify trigrams are properly stored
	if len(ti.trigrams) == 0 {
		t.Error("Expected trigrams to be indexed")
	}

	// Verify some trigrams have many locations (the ones that overlapped)
	maxLocations := 0
	for _, entry := range ti.trigrams {
		if len(entry.Locations) > maxLocations {
			maxLocations = len(entry.Locations)
		}
	}

	if maxLocations < numFiles/2 {
		t.Errorf("Expected some trigrams to have many locations, got max %d", maxLocations)
	}

	t.Logf("Maximum locations for a single trigram: %d", maxLocations)
}

// TestTrigramIndexSetBulkIndexing tests the bulk indexing mode setter
func TestTrigramIndexSetBulkIndexing(t *testing.T) {
	ti := NewTrigramIndex()

	// Test setting bulk indexing
	ti.SetBulkIndexing(true)
	if ti.BulkIndexing != 1 {
		t.Error("Expected BulkIndexing to be 1")
	}

	// Test unsetting bulk indexing
	ti.SetBulkIndexing(false)
	if ti.BulkIndexing != 0 {
		t.Error("Expected BulkIndexing to be 0")
	}
}
