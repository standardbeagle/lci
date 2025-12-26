// Context: ProgressTracker split from pipeline.go for separation of concerns.
// External deps: standard library sync, time, log, atomic; references to IndexingError/IndexingProgress from this package.
// Prompt-log: See root prompt-log.md (2025-09-05) for session details.
package indexing

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ProgressTracker tracks indexing progress with thread-safe operations
// Purpose: Provide lock-free counters and minimal locking for current file and errors.
// Gotchas: Keep updates cheap; avoid holding locks during I/O-heavy sections; this is hot-path.
type ProgressTracker struct {
	totalFiles     int64 // atomic
	currentFile    string
	currentFileMu  sync.RWMutex
	startTime      time.Time
	errors         []IndexingError
	errorsMu       sync.RWMutex
	lastUpdateTime time.Time
	lastUpdateMu   sync.RWMutex

	// Scanning phase tracking
	isScanning   int32 // atomic: 1 if scanning, 0 if indexing
	scannedFiles int64 // atomic: files discovered during scan

	// Sharded processed file counters to reduce atomic contention
	processedFiles   []int64        // shard-local counters (atomic)
	processedShards  []uint32       // shard batch counters (atomic)
	lastFlushTime    []atomic.Int64 // Unix nanos for lock-free time checks
	flushedProcessed int64          // atomic: total of all flushed shards

	// Callback to notify parent index when total files is set
	onTotalSet   func(total int)
	onTotalSetMu sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	const numShards = 8
	pt := &ProgressTracker{
		startTime:       time.Now(),
		lastUpdateTime:  time.Now(),
		isScanning:      1, // Start in scanning phase
		processedFiles:  make([]int64, numShards),
		processedShards: make([]uint32, numShards),
		lastFlushTime:   make([]atomic.Int64, numShards),
	}
	// Initialize flush times atomically
	nowNanos := time.Now().UnixNano()
	for i := range pt.lastFlushTime {
		pt.lastFlushTime[i].Store(nowNanos)
	}
	return pt
}

// SetOnTotalSet sets the callback to notify when total files is set
func (pt *ProgressTracker) SetOnTotalSet(callback func(total int)) {
	pt.onTotalSetMu.Lock()
	pt.onTotalSet = callback
	pt.onTotalSetMu.Unlock()
}

// SetTotal sets the total number of files to process
func (pt *ProgressTracker) SetTotal(total int) {
	atomic.StoreInt64(&pt.totalFiles, int64(total))
	// Transition from scanning to indexing phase
	atomic.StoreInt32(&pt.isScanning, 0)

	// Notify parent index if callback is set
	pt.onTotalSetMu.RLock()
	if pt.onTotalSet != nil {
		pt.onTotalSet(total)
	}
	pt.onTotalSetMu.RUnlock()
}

// IncrementScanned increments the scanned file count during discovery
func (pt *ProgressTracker) IncrementScanned() { atomic.AddInt64(&pt.scannedFiles, 1) }

// IncrementProcessed increments the processed file count using sharded counters
// to reduce atomic contention. Flushes every 10 files or after 100ms.
func (pt *ProgressTracker) IncrementProcessed(currentFile string) {
	// Use a simple hash of current file name to select shard deterministically
	// This avoids unsafe pointer usage and provides good distribution
	var hash uint32 = 5381
	for _, c := range currentFile {
		hash = ((hash << 5) + hash) + uint32(c)
	}
	shardIdx := hash % uint32(len(pt.processedFiles))

	// Increment shard-local counter atomically to prevent race conditions
	atomic.AddInt64(&pt.processedFiles[shardIdx], 1)

	// Update current file (only on flush to reduce lock contention)
	shouldFlush := false
	shardCount := atomic.AddUint32(&pt.processedShards[shardIdx], 1)
	if shardCount >= 10 {
		// Batch flush after 10 files
		shouldFlush = true
	} else {
		// Time-based flush to avoid stale updates (lock-free check)
		lastFlush := pt.lastFlushTime[shardIdx].Load()
		if time.Now().UnixNano()-lastFlush >= int64(100*time.Millisecond) {
			shouldFlush = true
		}
	}

	if shouldFlush {
		// Flush all shards atomically
		nowNanos := time.Now().UnixNano()
		var total int64
		for i := range pt.processedFiles {
			total += atomic.SwapInt64(&pt.processedFiles[i], 0)
			atomic.StoreUint32(&pt.processedShards[i], 0)
			pt.lastFlushTime[i].Store(nowNanos)
		}
		// Update global atomic counter with batched total
		atomic.AddInt64(&pt.flushedProcessed, total)

		// Update current file info (only on flush)
		pt.currentFileMu.Lock()
		pt.currentFile = currentFile
		pt.currentFileMu.Unlock()
		pt.lastUpdateMu.Lock()
		pt.lastUpdateTime = time.Now()
		pt.lastUpdateMu.Unlock()
	}
}

// AddError adds an indexing error
func (pt *ProgressTracker) AddError(err IndexingError) {
	pt.errorsMu.Lock()
	pt.errors = append(pt.errors, err)
	pt.errorsMu.Unlock()
	log.Printf("Indexing error in %s (%s): %s", err.FilePath, err.Stage, err.Error)
}

// FlushAllShards forces all sharded counters to flush to the global counter
// This is primarily used for testing to ensure accurate counts
func (pt *ProgressTracker) FlushAllShards() {
	nowNanos := time.Now().UnixNano()
	var total int64
	for i := range pt.processedFiles {
		total += atomic.SwapInt64(&pt.processedFiles[i], 0)
		atomic.StoreUint32(&pt.processedShards[i], 0)
		pt.lastFlushTime[i].Store(nowNanos)
	}
	if total > 0 {
		atomic.AddInt64(&pt.flushedProcessed, total)
		// Update last update time when we actually flush data
		pt.lastUpdateMu.Lock()
		pt.lastUpdateTime = time.Now()
		pt.lastUpdateMu.Unlock()
	}
}

// GetProgress returns current progress information
func (pt *ProgressTracker) GetProgress() IndexingProgress {
	total := atomic.LoadInt64(&pt.totalFiles)
	// Get processed count from flushed atomic plus any unflushed shard counts
	processed := atomic.LoadInt64(&pt.flushedProcessed)
	// Add any unflushed counts from shards
	for i := range pt.processedFiles {
		processed += atomic.LoadInt64(&pt.processedFiles[i])
	}
	scanned := atomic.LoadInt64(&pt.scannedFiles)
	isScanning := atomic.LoadInt32(&pt.isScanning) == 1

	pt.currentFileMu.RLock()
	currentFile := pt.currentFile
	pt.currentFileMu.RUnlock()

	pt.lastUpdateMu.RLock()
	_ = pt.lastUpdateTime // Read but mark as unused to avoid compile error
	pt.lastUpdateMu.RUnlock()

	elapsed := time.Since(pt.startTime)
	var filesPerSecond float64
	var estimatedTimeLeft int64

	if processed > 0 && elapsed > 0 {
		filesPerSecond = float64(processed) / elapsed.Seconds()
		if filesPerSecond > 0 {
			remaining := total - processed
			estimatedTimeLeft = int64(float64(remaining) / filesPerSecond)
		}
	}

	pt.errorsMu.RLock()
	errorsCopy := make([]IndexingError, len(pt.errors))
	copy(errorsCopy, pt.errors)
	pt.errorsMu.RUnlock()

	// Calculate progress percentages
	var scanningProgress, indexingProgress float64
	if isScanning {
		if scanned > 0 {
			scanningProgress = estimatedScanningProgress
		}
	} else {
		scanningProgress = 100.0
		if total > 0 {
			indexingProgress = float64(processed) / float64(total) * 100.0
		}
	}

	return IndexingProgress{
		FilesProcessed:    int(processed),
		TotalFiles:        int(total),
		CurrentFile:       currentFile,
		FilesPerSecond:    filesPerSecond,
		EstimatedTimeLeft: time.Duration(estimatedTimeLeft) * time.Second,
		Errors:            errorsCopy,
		ScanningProgress:  scanningProgress,
		IndexingProgress:  indexingProgress,
		IsScanning:        isScanning,
		ElapsedTime:       elapsed,
	}
}
