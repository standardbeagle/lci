package core

import (
	"github.com/standardbeagle/lci/internal/alloc"
	"github.com/standardbeagle/lci/internal/types"
)

// Bucketing interface methods for sharded pipeline indexing
// These methods allow processors to shard trigrams during extraction

// GetBucketForTrigram returns the bucket ID for a trigram hash
// This ensures consistent bucketing across all components
func (ti *TrigramIndex) GetBucketForTrigram(trigramHash uint32) uint16 {
	return uint16(trigramHash & ti.bucketMask)
}

// GetBucketCount returns the total number of buckets
func (ti *TrigramIndex) GetBucketCount() int {
	return int(ti.bucketCount)
}

// BucketedTrigramData holds trigrams for a specific bucket from one file
type BucketedTrigramData struct {
	Trigrams map[uint32][]uint32 // trigramHash -> []offsets
}

// BucketedTrigramResult is produced by processors with pre-sharded trigrams
type BucketedTrigramResult struct {
	FileID  types.FileID
	Buckets []BucketedTrigramData // One per bucket (sparse - only populated buckets have data)
}

// CreateBucketedResult creates a properly sized result structure
// Processors call this to get the right size array for bucketing
func (ti *TrigramIndex) CreateBucketedResult(fileID types.FileID) *BucketedTrigramResult {
	return &BucketedTrigramResult{
		FileID:  fileID,
		Buckets: make([]BucketedTrigramData, ti.bucketCount),
	}
}

// IndexFileWithBucketedTrigrams indexes a file using pre-bucketed trigrams
// This is the new channel-friendly API that avoids lock contention
func (ti *TrigramIndex) IndexFileWithBucketedTrigrams(result *BucketedTrigramResult) {
	// For now, convert bucketed format to the existing format
	// Later we'll optimize this to use the buckets directly
	trigrams := make(map[uint32][]uint32)

	for _, bucket := range result.Buckets {
		if bucket.Trigrams == nil {
			continue
		}
		for trigramHash, offsets := range bucket.Trigrams {
			trigrams[trigramHash] = append(trigrams[trigramHash], offsets...)
		}
	}

	// Use existing implementation
	ti.IndexFileWithTrigrams(result.FileID, trigrams)
}

// GetAllocator returns the slab allocator for use by merger pipeline
func (ti *TrigramIndex) GetAllocator() *alloc.SlabAllocator[FileLocation] {
	return ti.locationAllocator
}
