package core

import (
	"sync"

	"github.com/standardbeagle/lci/internal/alloc"
	"github.com/standardbeagle/lci/internal/types"
)

// TrigramBucket holds trigrams for a specific bucket with its own lock
// This enables lock-free parallel merging across different buckets
type TrigramBucket struct {
	mu       sync.Mutex // Per-bucket lock (much finer-grained than global lock)
	trigrams map[uint32]*TrigramEntry
}

// GetMutex returns the bucket's mutex for locking
func (b *TrigramBucket) GetMutex() *sync.Mutex {
	return &b.mu
}

// GetTrigrams returns the bucket's trigram map
func (b *TrigramBucket) GetTrigrams() map[uint32]*TrigramEntry {
	return b.trigrams
}

// ShardedTrigramStorage manages trigram data across multiple buckets
// Each bucket can be updated independently without blocking other buckets
type ShardedTrigramStorage struct {
	buckets     []*TrigramBucket
	bucketCount uint16
	bucketMask  uint32
}

// NewShardedTrigramStorage creates a new sharded storage with the specified number of buckets
func NewShardedTrigramStorage(bucketCount uint16) *ShardedTrigramStorage {
	buckets := make([]*TrigramBucket, bucketCount)
	for i := range buckets {
		buckets[i] = &TrigramBucket{
			trigrams: make(map[uint32]*TrigramEntry),
		}
	}

	return &ShardedTrigramStorage{
		buckets:     buckets,
		bucketCount: bucketCount,
		bucketMask:  uint32(bucketCount - 1),
	}
}

// GetBucket returns the bucket for a given trigram hash
// NOTE: This is for search/read operations only. For writes, use MergeBucketDataForWorker
func (s *ShardedTrigramStorage) GetBucket(trigramHash uint32) *TrigramBucket {
	bucketID := trigramHash & s.bucketMask
	return s.buckets[bucketID]
}

// GetBucketByID returns the bucket for a given bucket ID
// NOTE: This is for search/read operations only. For writes, use MergeBucketDataForWorker
// This method is kept for compatibility but should not be used for writes
func (s *ShardedTrigramStorage) GetBucketByID(bucketID int) *TrigramBucket {
	return s.buckets[bucketID]
}

// GetBucketCount returns the total number of buckets
func (s *ShardedTrigramStorage) GetBucketCount() int {
	return len(s.buckets)
}

// MergeBucketDataForWorker merges trigrams for a specific bucket range
// This is thread-safe and should be used by merger workers instead of accessing buckets directly
func (s *ShardedTrigramStorage) MergeBucketDataForWorker(
	result *BucketedTrigramResult,
	bucketStart, bucketEnd int,
	allocator *alloc.SlabAllocator[FileLocation],
) {
	// Process only buckets in the specified range
	for bucketID := bucketStart; bucketID < bucketEnd && bucketID < len(result.Buckets); bucketID++ {
		bucketData := result.Buckets[bucketID]
		if bucketData.Trigrams == nil || len(bucketData.Trigrams) == 0 {
			continue
		}

		bucket := s.buckets[bucketID]
		bucket.mu.Lock()

		// Merge all trigrams in this bucket
		for trigramHash, offsets := range bucketData.Trigrams {
			entry, exists := bucket.trigrams[trigramHash]
			if !exists {
				entry = &TrigramEntry{}
				bucket.trigrams[trigramHash] = entry
			}

			// Convert offsets to FileLocations
			oldLen := len(entry.Locations)
			newLen := oldLen + len(offsets)

			// Use slab allocator if available for efficient memory management
			if allocator != nil {
				entry.Locations = allocator.GrowSlice(entry.Locations, len(offsets))
				// GrowSlice may return a slice with sufficient capacity but not length
				// We need to extend the length to accommodate new elements
				entry.Locations = entry.Locations[:newLen]
			} else {
				if cap(entry.Locations) < newLen {
					newSlice := make([]FileLocation, newLen, newLen*2)
					copy(newSlice, entry.Locations)
					entry.Locations = newSlice
				} else {
					entry.Locations = entry.Locations[:newLen]
				}
			}

			// Append new locations
			for i, offset := range offsets {
				entry.Locations[oldLen+i] = FileLocation{
					FileID: result.FileID,
					Offset: offset,
				}
			}
		}

		bucket.mu.Unlock()
	}
}

// MergeBucketedTrigrams merges pre-bucketed trigrams into storage
// Each bucket is processed independently, allowing parallel merging
func (s *ShardedTrigramStorage) MergeBucketedTrigrams(
	result *BucketedTrigramResult,
	allocator *alloc.SlabAllocator[FileLocation],
) {
	// Process each bucket independently
	for bucketID, bucketData := range result.Buckets {
		if bucketData.Trigrams == nil || len(bucketData.Trigrams) == 0 {
			continue
		}

		bucket := s.buckets[bucketID]
		bucket.mu.Lock()

		// Merge all trigrams in this bucket
		for trigramHash, offsets := range bucketData.Trigrams {
			entry, exists := bucket.trigrams[trigramHash]
			if !exists {
				entry = &TrigramEntry{}
				bucket.trigrams[trigramHash] = entry
			}

			// Convert offsets to FileLocations
			oldLen := len(entry.Locations)
			newLen := oldLen + len(offsets)

			// Use slab allocator if available for efficient memory management
			if allocator != nil {
				entry.Locations = allocator.GrowSlice(entry.Locations, len(offsets))
				// GrowSlice may return a slice with sufficient capacity but not length
				// We need to extend the length to accommodate new elements
				entry.Locations = entry.Locations[:newLen]
			} else {
				if cap(entry.Locations) < newLen {
					newSlice := make([]FileLocation, newLen, newLen*2)
					copy(newSlice, entry.Locations)
					entry.Locations = newSlice
				} else {
					entry.Locations = entry.Locations[:newLen]
				}
			}

			// Append new locations
			for i, offset := range offsets {
				entry.Locations[oldLen+i] = FileLocation{
					FileID: result.FileID,
					Offset: offset,
				}
			}
		}

		bucket.mu.Unlock()
	}
}

// SearchTrigram searches for a trigram across all buckets
func (s *ShardedTrigramStorage) SearchTrigram(trigramHash uint32) []FileLocation {
	bucket := s.GetBucket(trigramHash)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	entry, exists := bucket.trigrams[trigramHash]
	if !exists {
		return nil
	}

	// Return a copy to avoid data races
	result := make([]FileLocation, len(entry.Locations))
	copy(result, entry.Locations)
	return result
}

// Clear removes all trigrams from all buckets
func (s *ShardedTrigramStorage) Clear() {
	for _, bucket := range s.buckets {
		bucket.mu.Lock()
		bucket.trigrams = make(map[uint32]*TrigramEntry)
		bucket.mu.Unlock()
	}
}

// RemoveFile removes all occurrences of a file from all buckets
func (s *ShardedTrigramStorage) RemoveFile(fileID types.FileID) {
	var wg sync.WaitGroup

	// Process buckets in parallel for fast removal
	for _, bucket := range s.buckets {
		wg.Add(1)
		go func(b *TrigramBucket) {
			defer wg.Done()

			b.mu.Lock()
			defer b.mu.Unlock()

			// Scan all trigrams in this bucket
			for trigramHash, entry := range b.trigrams {
				// Filter out locations for this file
				filtered := entry.Locations[:0]
				for _, loc := range entry.Locations {
					if loc.FileID != fileID {
						filtered = append(filtered, loc)
					}
				}

				if len(filtered) == 0 {
					// Remove empty entry
					delete(b.trigrams, trigramHash)
				} else {
					entry.Locations = filtered
				}
			}
		}(bucket)
	}

	wg.Wait()
}
