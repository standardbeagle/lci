package indexing

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/alloc"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
)

// TrigramMergerPipeline manages channel-based merging of bucketed trigrams
// Uses multiple merger goroutines to process different buckets in parallel
type TrigramMergerPipeline struct {
	trigramIndex     *core.TrigramIndex
	storage          *core.ShardedTrigramStorage
	mergerCount      int
	inputChan        chan *core.BucketedTrigramResult
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
	bucketsPerMerger int
	shutdownOnce     sync.Once
	shutdownChan     chan struct{}
	shutdown         atomic.Bool                      // Changed from bool to atomic.Bool for thread-safety
	failedFiles      atomic.Int64                     // Count of failed submissions for reindexing
	retryQueue       chan *core.BucketedTrigramResult // Retry queue for blocked submissions
}

// NewTrigramMergerPipeline creates a new merger pipeline
// mergerCount determines how many parallel merger goroutines to use
// Recommended: 16 mergers for 256 buckets = 16 buckets per merger
func NewTrigramMergerPipeline(trigramIndex *core.TrigramIndex, mergerCount int) *TrigramMergerPipeline {
	if mergerCount <= 0 {
		mergerCount = 16 // Default to 16 mergers
	}

	bucketCount := trigramIndex.GetBucketCount()
	bucketsPerMerger := bucketCount / mergerCount
	if bucketsPerMerger == 0 {
		bucketsPerMerger = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TrigramMergerPipeline{
		trigramIndex:     trigramIndex,
		storage:          core.NewShardedTrigramStorage(uint16(bucketCount)),
		mergerCount:      mergerCount,
		inputChan:        make(chan *core.BucketedTrigramResult, mergerCount*32), // Larger buffer to reduce blocking
		retryQueue:       make(chan *core.BucketedTrigramResult, mergerCount*8),  // Separate retry queue
		ctx:              ctx,
		cancel:           cancel,
		bucketsPerMerger: bucketsPerMerger,
		shutdownChan:     make(chan struct{}),
		// failedFiles is atomic.Int64 and defaults to 0
	}
}

// Start launches the merger goroutines
func (p *TrigramMergerPipeline) Start() {
	// Start merger workers
	for i := 0; i < p.mergerCount; i++ {
		p.wg.Add(1)
		go p.mergerWorker(i)
	}

	// Start retry worker to handle blocked submissions
	p.wg.Add(1)
	go p.retryWorker()
}

// Submit sends a bucketed trigram result to the merger pipeline
func (p *TrigramMergerPipeline) Submit(result *core.BucketedTrigramResult) {
	// Reject submissions if shutting down (use atomic load for thread-safety)
	if p.shutdown.Load() {
		debug.LogIndexing("WARNING: Rejecting submission - pipeline shutting down\n")
		return
	}

	// Try non-blocking submission first
	select {
	case p.inputChan <- result:
		// Submitted successfully
		return
	default:
		// Channel is full, queue for retry instead of blocking
		select {
		case p.retryQueue <- result:
			debug.LogIndexing("Merger pipeline buffer full, queued for retry (FileID: %d)\n", result.FileID)
			return
		case <-p.shutdownChan:
			// Pipeline shutting down - don't accept new submissions
			debug.LogIndexing("WARNING: Submission rejected - pipeline shutting down\n")
			return
		case <-p.ctx.Done():
			// Pipeline shutting down
			debug.LogIndexing("WARNING: Submission rejected - context cancelled\n")
			return
		default:
			// Even retry queue is full - count as failed for reindexing
			p.failedFiles.Add(1)
			debug.LogIndexing("CRITICAL: Both main and retry queues full, flagged for reindexing (FileID: %d)\n", result.FileID)
			return
		}
	}
}

// mergerWorker processes bucketed trigrams from the input channel
// Each worker is assigned a specific range of buckets for TRUE independence
func (p *TrigramMergerPipeline) mergerWorker(workerID int) {
	defer p.wg.Done()

	// Calculate which buckets this worker is responsible for
	bucketStart := workerID * p.bucketsPerMerger
	bucketEnd := bucketStart + p.bucketsPerMerger
	if bucketEnd > p.trigramIndex.GetBucketCount() {
		bucketEnd = p.trigramIndex.GetBucketCount()
	}

	processed := 0
	for {
		select {
		case <-p.ctx.Done():
			// Context cancelled - shutdown requested
			if processed > 0 {
				debug.LogIndexing("Merger %d (buckets %d-%d): processed %d files before shutdown\n",
					workerID, bucketStart, bucketEnd-1, processed)
			}
			return

		case result, ok := <-p.inputChan:
			if !ok {
				// Channel closed - no more work
				if processed > 0 {
					debug.LogIndexing("Merger %d (buckets %d-%d): processed %d files, channel closed\n",
						workerID, bucketStart, bucketEnd-1, processed)
				}
				return
			}

			// Merge ONLY the buckets assigned to this worker
			// This ensures TRUE independence - no two workers ever lock the same bucket
			p.mergeBucketsInRange(result, bucketStart, bucketEnd, p.trigramIndex.GetAllocator())
			processed++

			// Log progress every 100 files
			if processed%100 == 0 {
				debug.LogIndexing("Merger %d (buckets %d-%d): processed %d files\n",
					workerID, bucketStart, bucketEnd-1, processed)
			}
		}
	}
}

// retryWorker processes submissions from the retry queue
func (p *TrigramMergerPipeline) retryWorker() {
	defer p.wg.Done()

	retryCount := 0
	for {
		select {
		case <-p.ctx.Done():
			// Context cancelled - shutdown requested
			if retryCount > 0 {
				debug.LogIndexing("Retry worker: processed %d retries before shutdown\n", retryCount)
			}
			return

		case result, ok := <-p.retryQueue:
			if !ok {
				// Channel closed - no more retries
				if retryCount > 0 {
					debug.LogIndexing("Retry worker: processed %d retries, channel closed\n", retryCount)
				}
				return
			}

			// Try to resubmit to main queue with timeout
			select {
			case p.inputChan <- result:
				// Successfully resubmitted
				retryCount++
				debug.LogIndexing("Retry worker: successfully resubmitted FileID %d\n", result.FileID)
			case <-time.After(5 * time.Second):
				// Still blocked, increment failed count and drop
				p.failedFiles.Add(1)
				debug.LogIndexing("CRITICAL: Retry failed for FileID %d after 5s, flagged for reindexing\n", result.FileID)
			case <-p.ctx.Done():
				// Shutdown during retry attempt
				p.failedFiles.Add(1)
				debug.LogIndexing("Retry worker: shutdown during retry, flagged FileID %d for reindexing\n", result.FileID)
				return
			}
		}
	}
}

// mergeBucketsInRange merges only buckets in the specified range for TRUE independence
func (p *TrigramMergerPipeline) mergeBucketsInRange(
	result *core.BucketedTrigramResult,
	bucketStart, bucketEnd int,
	allocator *alloc.SlabAllocator[core.FileLocation],
) {
	// Use the thread-safe storage method instead of direct bucket access
	// This ensures proper locking and prevents data races
	p.storage.MergeBucketDataForWorker(result, bucketStart, bucketEnd, allocator)
}

// Shutdown gracefully shuts down the merger pipeline
// This method is idempotent and can be called multiple times safely
func (p *TrigramMergerPipeline) Shutdown() {
	p.shutdownOnce.Do(func() {
		// Mark as shutdown to prevent new submissions (use atomic store)
		p.shutdown.Store(true)
		// Close shutdown channel to signal to any waiting Submits
		close(p.shutdownChan)
		// Cancel context to signal shutdown to all workers
		p.cancel()
		// Wait for all workers to finish
		// Workers will exit when they see context is done
		p.wg.Wait()
		// Now it's safe to close the channels since no goroutines are reading from them
		close(p.inputChan)
		close(p.retryQueue)

		// Log final statistics
		failedCount := p.failedFiles.Load()
		if failedCount > 0 {
			debug.LogIndexing("Merger pipeline shutdown: %d files failed and flagged for reindexing\n", failedCount)
		}
	})
}

// GetFailedFileCount returns the number of files that failed to index
// This can be used to determine if a reindex is needed
func (p *TrigramMergerPipeline) GetFailedFileCount() int64 {
	return p.failedFiles.Load()
}

// HasFailures returns true if any files failed to index
func (p *TrigramMergerPipeline) HasFailures() bool {
	return p.failedFiles.Load() > 0
}

// GetStorage returns the sharded storage for search operations
func (p *TrigramMergerPipeline) GetStorage() *core.ShardedTrigramStorage {
	return p.storage
}

// ChannelStats returns statistics about the merger pipeline
type ChannelStats struct {
	BufferSize     int
	BufferCapacity int
	MergerCount    int
	BufferUsage    float64 // Percentage of buffer in use
}

// GetStats returns current pipeline statistics
func (p *TrigramMergerPipeline) GetStats() ChannelStats {
	bufferSize := len(p.inputChan)
	bufferCap := cap(p.inputChan)
	usage := 0.0
	if bufferCap > 0 {
		usage = float64(bufferSize) / float64(bufferCap) * 100.0
	}

	return ChannelStats{
		BufferSize:     bufferSize,
		BufferCapacity: bufferCap,
		MergerCount:    p.mergerCount,
		BufferUsage:    usage,
	}
}
