package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// IndexStatusSnapshot represents a snapshot of index status at a point in time (T033)
type IndexStatusSnapshot struct {
	Timestamp              time.Time     `json:"timestamp"`
	IndexType              IndexType     `json:"indexType"`
	IsIndexing             bool          `json:"isIndexing"`
	Progress               float64       `json:"progress"`
	CurrentOperation       int64         `json:"currentOperation"`
	FilesProcessed         int64         `json:"filesProcessed"`
	TotalFiles             int64         `json:"totalFiles"`
	BytesProcessed         int64         `json:"bytesProcessed"`
	TotalBytes             int64         `json:"totalBytes"`
	LockHolders            int           `json:"lockHolders"`
	QueueDepth             int           `json:"queueDepth"`
	LastUpdate             time.Time     `json:"lastUpdate"`
	UpdateCount            int64         `json:"updateCount"`
	HasError               bool          `json:"hasError"`
	ErrorMessage           string        `json:"errorMessage,omitempty"`
	EstimatedTimeRemaining time.Duration `json:"estimatedTimeRemaining"`
}

// IndexState represents the current state and coordination state of each index system
type IndexState struct {
	// Core index identification
	Type IndexType

	// Atomic state fields
	isIndexing  int32 // Current indexing status flag (0 = not indexing, 1 = indexing)
	lastUpdate  int64 // Timestamp of last update completion (Unix nanoseconds)
	updateCount int64 // Number of updates performed

	// Progress tracking (T033)
	progress         int64        // Current progress percentage (0-100)
	progressMu       sync.RWMutex // Protects progress operations
	startTime        int64        // Start time of current operation (Unix nanoseconds)
	estimatedTime    int64        // Estimated completion time (Unix nanoseconds)
	currentOperation int64        // Type of current operation (using OperationType values)
	filesProcessed   int64        // Number of files processed in current operation
	totalFiles       int64        // Total number of files to process
	bytesProcessed   int64        // Bytes processed in current operation
	totalBytes       int64        // Total bytes to process

	// Synchronization
	mu sync.RWMutex // Read-write lock for index operations

	// Lock management
	currentReaders int32 // Number of current read lock holders
	currentWriter  int32 // Flag indicating if write lock is held (0/1)
	waitingReaders int32 // Number of readers waiting for lock
	waitingWriters int32 // Number of writers waiting for lock

	// Queue management
	queueDepth int32      // Current queue depth for operations
	queueMu    sync.Mutex // Protects queue operations

	// Error handling
	lastError     error
	lastErrorTime int64 // Unix nanoseconds of last error

	// Status history (T033)
	statusHistory []IndexStatusSnapshot
	historyMu     sync.RWMutex // Protects status history
}

// NewIndexState creates a new IndexState for the given index type
func NewIndexState(indexType IndexType) *IndexState {
	return &IndexState{
		Type:       indexType,
		lastUpdate: time.Now().UnixNano(), // Initialize to current time to avoid Unix epoch timestamps
	}
}

// IsIndexing returns whether the index is currently being updated (atomic operation)
func (is *IndexState) IsIndexing() bool {
	return atomic.LoadInt32(&is.isIndexing) == 1
}

// SetIndexing atomically sets the indexing status
func (is *IndexState) SetIndexing(indexing bool) {
	var val int32 = 0
	if indexing {
		val = 1
	}
	atomic.StoreInt32(&is.isIndexing, val)
}

// GetLastUpdate returns the timestamp of the last update completion (atomic operation)
func (is *IndexState) GetLastUpdate() time.Time {
	nanos := atomic.LoadInt64(&is.lastUpdate)
	return time.Unix(0, nanos)
}

// SetLastUpdate atomically sets the last update timestamp
func (is *IndexState) SetLastUpdate(t time.Time) {
	atomic.StoreInt64(&is.lastUpdate, t.UnixNano())
}

// GetUpdateCount returns the number of updates performed (atomic operation)
func (is *IndexState) GetUpdateCount() int64 {
	return atomic.LoadInt64(&is.updateCount)
}

// IncrementUpdateCount atomically increments the update count
func (is *IndexState) IncrementUpdateCount() int64 {
	return atomic.AddInt64(&is.updateCount, 1)
}

// T033: Progress tracking methods

// GetProgress returns the current progress percentage (0-100)
func (is *IndexState) GetProgress() float64 {
	return float64(atomic.LoadInt64(&is.progress))
}

// SetProgress atomically sets the progress percentage (0-100)
func (is *IndexState) SetProgress(progress float64) {
	// Clamp progress to 0-100 range
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}
	atomic.StoreInt64(&is.progress, int64(progress))

	// Create status snapshot
	is.captureStatusSnapshot()
}

// GetStartTime returns the start time of the current operation
func (is *IndexState) GetStartTime() time.Time {
	nanos := atomic.LoadInt64(&is.startTime)
	return time.Unix(0, nanos)
}

// SetStartTime atomically sets the start time of the current operation
func (is *IndexState) SetStartTime(t time.Time) {
	atomic.StoreInt64(&is.startTime, t.UnixNano())
}

// GetEstimatedTime returns the estimated completion time of the current operation
func (is *IndexState) GetEstimatedTime() time.Time {
	nanos := atomic.LoadInt64(&is.estimatedTime)
	return time.Unix(0, nanos)
}

// SetEstimatedTime atomically sets the estimated completion time of the current operation
func (is *IndexState) SetEstimatedTime(t time.Time) {
	atomic.StoreInt64(&is.estimatedTime, t.UnixNano())
}

// GetCurrentOperation returns the type of current operation
func (is *IndexState) GetCurrentOperation() int64 {
	return atomic.LoadInt64(&is.currentOperation)
}

// SetCurrentOperation atomically sets the type of current operation
func (is *IndexState) SetCurrentOperation(operationType int64) {
	atomic.StoreInt64(&is.currentOperation, operationType)
}

// GetFilesProcessed returns the number of files processed in current operation
func (is *IndexState) GetFilesProcessed() int64 {
	return atomic.LoadInt64(&is.filesProcessed)
}

// SetFilesProcessed atomically sets the number of files processed
func (is *IndexState) SetFilesProcessed(count int64) {
	atomic.StoreInt64(&is.filesProcessed, count)
}

// AddFilesProcessed atomically adds to the number of files processed
func (is *IndexState) AddFilesProcessed(count int64) {
	atomic.AddInt64(&is.filesProcessed, count)
}

// GetTotalFiles returns the total number of files to process
func (is *IndexState) GetTotalFiles() int64 {
	return atomic.LoadInt64(&is.totalFiles)
}

// SetTotalFiles atomically sets the total number of files to process
func (is *IndexState) SetTotalFiles(count int64) {
	atomic.StoreInt64(&is.totalFiles, count)
}

// GetBytesProcessed returns the number of bytes processed in current operation
func (is *IndexState) GetBytesProcessed() int64 {
	return atomic.LoadInt64(&is.bytesProcessed)
}

// SetBytesProcessed atomically sets the number of bytes processed
func (is *IndexState) SetBytesProcessed(count int64) {
	atomic.StoreInt64(&is.bytesProcessed, count)
}

// AddBytesProcessed atomically adds to the number of bytes processed
func (is *IndexState) AddBytesProcessed(count int64) {
	atomic.AddInt64(&is.bytesProcessed, count)
}

// GetTotalBytes returns the total number of bytes to process
func (is *IndexState) GetTotalBytes() int64 {
	return atomic.LoadInt64(&is.totalBytes)
}

// SetTotalBytes atomically sets the total number of bytes to process
func (is *IndexState) SetTotalBytes(count int64) {
	atomic.StoreInt64(&is.totalBytes, count)
}

// GetEstimatedTimeRemaining calculates estimated time remaining for current operation
func (is *IndexState) GetEstimatedTimeRemaining() time.Duration {
	progress := is.GetProgress()
	if progress <= 0 {
		return 0
	}

	startTime := is.GetStartTime()
	if startTime.IsZero() {
		return 0
	}

	elapsed := time.Since(startTime)
	if elapsed <= 0 || progress >= 100 {
		return 0
	}

	// Calculate remaining time based on current progress
	remaining := time.Duration(float64(elapsed) * (100.0 - progress) / progress)
	return remaining
}

// StartOperation starts tracking a new operation
func (is *IndexState) StartOperation(operationType int64, totalFiles, totalBytes int64) {
	is.SetCurrentOperation(operationType)
	is.SetProgress(0)
	is.SetStartTime(time.Now())
	is.SetFilesProcessed(0)
	is.SetTotalFiles(totalFiles)
	is.SetBytesProcessed(0)
	is.SetTotalBytes(totalBytes)
	is.SetEstimatedTime(time.Time{}) // Will be calculated as we progress

	// Create initial status snapshot
	is.captureStatusSnapshot()
}

// UpdateOperation updates the progress of the current operation
func (is *IndexState) UpdateOperation(filesProcessed, bytesProcessed int64) {
	if filesProcessed > 0 {
		is.SetFilesProcessed(filesProcessed)
	}
	if bytesProcessed > 0 {
		is.SetBytesProcessed(bytesProcessed)
	}

	// Calculate progress based on files and bytes
	totalFiles := is.GetTotalFiles()
	totalBytes := is.GetTotalBytes()

	var fileProgress, byteProgress float64

	if totalFiles > 0 {
		fileProgress = float64(is.GetFilesProcessed()) / float64(totalFiles) * 100
	}

	if totalBytes > 0 {
		byteProgress = float64(is.GetBytesProcessed()) / float64(totalBytes) * 100
	}

	// Use the higher of the two progress values
	var progress float64
	if fileProgress > byteProgress {
		progress = fileProgress
	} else {
		progress = byteProgress
	}

	is.SetProgress(progress)

	// Update estimated completion time if we have enough data
	if progress > 5 { // Wait for at least 5% progress before estimating
		startTime := is.GetStartTime()
		if !startTime.IsZero() {
			elapsed := time.Since(startTime)
			if progress > 0 {
				totalEstimated := time.Duration(float64(elapsed) * 100.0 / progress)
				estimatedCompletion := startTime.Add(totalEstimated)
				is.SetEstimatedTime(estimatedCompletion)
			}
		}
	}

	// Create status snapshot
	is.captureStatusSnapshot()
}

// CompleteOperation marks the current operation as complete
func (is *IndexState) CompleteOperation() {
	is.SetProgress(100)
	is.SetLastUpdate(time.Now())
	is.IncrementUpdateCount()

	// Create final status snapshot
	is.captureStatusSnapshot()
}

// captureStatusSnapshot creates a snapshot of the current status (T033)
func (is *IndexState) captureStatusSnapshot() {
	snapshot := IndexStatusSnapshot{
		Timestamp:              time.Now(),
		IndexType:              is.Type,
		IsIndexing:             is.IsIndexing(),
		Progress:               is.GetProgress(),
		CurrentOperation:       is.GetCurrentOperation(),
		FilesProcessed:         is.GetFilesProcessed(),
		TotalFiles:             is.GetTotalFiles(),
		BytesProcessed:         is.GetBytesProcessed(),
		TotalBytes:             is.GetTotalBytes(),
		LockHolders:            int(atomic.LoadInt32(&is.currentReaders) + atomic.LoadInt32(&is.currentWriter)),
		QueueDepth:             int(atomic.LoadInt32(&is.queueDepth)),
		LastUpdate:             is.GetLastUpdate(),
		UpdateCount:            is.GetUpdateCount(),
		EstimatedTimeRemaining: is.GetEstimatedTimeRemaining(),
	}

	// Add error information
	lastErr, _ := is.GetLastError()
	if lastErr != nil {
		snapshot.HasError = true
		snapshot.ErrorMessage = lastErr.Error()
	}

	// Add to history
	is.historyMu.Lock()
	defer is.historyMu.Unlock()

	// Keep only last 100 snapshots to prevent memory growth
	if len(is.statusHistory) >= 100 {
		// Remove oldest snapshot
		is.statusHistory = append(is.statusHistory[1:], snapshot)
	} else {
		is.statusHistory = append(is.statusHistory, snapshot)
	}
}

// GetStatusHistory returns recent status snapshots (T033)
func (is *IndexState) GetStatusHistory(limit int) []IndexStatusSnapshot {
	is.historyMu.RLock()
	defer is.historyMu.RUnlock()

	history := make([]IndexStatusSnapshot, len(is.statusHistory))
	copy(history, is.statusHistory)

	if limit > 0 && len(history) > limit {
		// Return the most recent snapshots
		return history[len(history)-limit:]
	}

	return history
}

// GetDetailedStatus returns detailed status information (T033)
func (is *IndexState) GetDetailedStatus() IndexStatusSnapshot {
	return IndexStatusSnapshot{
		Timestamp:              time.Now(),
		IndexType:              is.Type,
		IsIndexing:             is.IsIndexing(),
		Progress:               is.GetProgress(),
		CurrentOperation:       is.GetCurrentOperation(),
		FilesProcessed:         is.GetFilesProcessed(),
		TotalFiles:             is.GetTotalFiles(),
		BytesProcessed:         is.GetBytesProcessed(),
		TotalBytes:             is.GetTotalBytes(),
		LockHolders:            int(atomic.LoadInt32(&is.currentReaders) + atomic.LoadInt32(&is.currentWriter)),
		QueueDepth:             int(atomic.LoadInt32(&is.queueDepth)),
		LastUpdate:             is.GetLastUpdate(),
		UpdateCount:            is.GetUpdateCount(),
		EstimatedTimeRemaining: is.GetEstimatedTimeRemaining(),
	}
}

// GetProgressStats returns progress statistics (T033)
func (is *IndexState) GetProgressStats() struct {
	Progress               float64
	FilesPerSecond         float64
	BytesPerSecond         float64
	Elapsed                time.Duration
	EstimatedTimeRemaining time.Duration
	IsComplete             bool
} {
	progress := is.GetProgress()
	startTime := is.GetStartTime()
	var elapsed time.Duration
	if !startTime.IsZero() {
		elapsed = time.Since(startTime)
	}

	filesPerSecond := 0.0
	bytesPerSecond := 0.0
	if elapsed > 0 {
		filesProcessed := float64(is.GetFilesProcessed())
		bytesProcessed := float64(is.GetBytesProcessed())
		elapsedSeconds := elapsed.Seconds()
		if elapsedSeconds > 0 {
			filesPerSecond = filesProcessed / elapsedSeconds
			bytesPerSecond = bytesProcessed / elapsedSeconds
		}
	}

	return struct {
		Progress               float64
		FilesPerSecond         float64
		BytesPerSecond         float64
		Elapsed                time.Duration
		EstimatedTimeRemaining time.Duration
		IsComplete             bool
	}{
		Progress:               progress,
		FilesPerSecond:         filesPerSecond,
		BytesPerSecond:         bytesPerSecond,
		Elapsed:                elapsed,
		EstimatedTimeRemaining: is.GetEstimatedTimeRemaining(),
		IsComplete:             progress >= 100,
	}
}

// AcquireReadLock acquires a read lock for the index (shared access)
// Returns a release function and an error if the lock cannot be acquired
func (is *IndexState) AcquireReadLock(ctx context.Context, timeout time.Duration) (LockRelease, error) {
	// Increment waiting readers
	atomic.AddInt32(&is.waitingReaders, 1)
	defer atomic.AddInt32(&is.waitingReaders, -1)

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for write lock to be released and indexing to complete
	for {
		// Check if we can acquire read lock
		if atomic.LoadInt32(&is.currentWriter) == 0 && !is.IsIndexing() {
			// Try to acquire read lock
			if atomic.AddInt32(&is.currentReaders, 1) > 0 {
				// Successfully acquired read lock
				atomic.AddInt32(&is.queueDepth, -1)
				return func() {
					atomic.AddInt32(&is.currentReaders, -1)
				}, nil
			}
			// Failed to acquire, roll back
			atomic.AddInt32(&is.currentReaders, -1)
		}

		// Check for timeout
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("read lock acquisition timeout for %s", is.Type.String())
		default:
			// Increment queue depth
			atomic.AddInt32(&is.queueDepth, 1)
			// Small delay to prevent busy waiting
			time.Sleep(100 * time.Microsecond)
		}
	}
}

// AcquireWriteLock acquires a write lock for the index (exclusive access)
// Returns a release function and an error if the lock cannot be acquired
func (is *IndexState) AcquireWriteLock(ctx context.Context, timeout time.Duration) (LockRelease, error) {
	// Increment waiting writers
	atomic.AddInt32(&is.waitingWriters, 1)
	defer atomic.AddInt32(&is.waitingWriters, -1)

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for all readers and current writer to be released
	for {
		// Check if we can acquire write lock
		if atomic.LoadInt32(&is.currentReaders) == 0 &&
			atomic.LoadInt32(&is.currentWriter) == 0 {
			// Try to acquire write lock
			if atomic.CompareAndSwapInt32(&is.currentWriter, 0, 1) {
				// Successfully acquired write lock, mark as indexing
				is.SetIndexing(true)
				atomic.AddInt32(&is.queueDepth, -1)
				return func() {
					atomic.StoreInt32(&is.currentWriter, 0)
					is.SetIndexing(false)
					is.SetLastUpdate(time.Now())
					is.IncrementUpdateCount()
				}, nil
			}
		}

		// Check for timeout
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("write lock acquisition timeout for %s", is.Type.String())
		default:
			// Increment queue depth
			atomic.AddInt32(&is.queueDepth, 1)
			// Small delay to prevent busy waiting
			time.Sleep(100 * time.Microsecond)
		}
	}
}

// GetStatus returns the current status of the index
func (is *IndexState) GetStatus() IndexStatus {
	return IndexStatus{
		Type:        is.Type,
		IsIndexing:  is.IsIndexing(),
		LastUpdate:  is.GetLastUpdate(),
		UpdateCount: is.GetUpdateCount(),
		LockHolders: int(atomic.LoadInt32(&is.currentReaders) + atomic.LoadInt32(&is.currentWriter)),
		QueueDepth:  int(atomic.LoadInt32(&is.queueDepth)),
	}
}

// WaitForIndexUpdate waits until the index is not indexing or timeout occurs
func (is *IndexState) WaitForIndexUpdate(ctx context.Context, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for %s index update", is.Type.String())
		case <-ticker.C:
			if !is.IsIndexing() {
				return nil
			}
		}
	}
}

// SetError records an error for the index
func (is *IndexState) SetError(err error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.lastError = err
	atomic.StoreInt64(&is.lastErrorTime, time.Now().UnixNano())
}

// GetLastError returns the last error and when it occurred
func (is *IndexState) GetLastError() (error, time.Time) {
	is.mu.RLock()
	defer is.mu.RUnlock()

	nanos := atomic.LoadInt64(&is.lastErrorTime)
	return is.lastError, time.Unix(0, nanos)
}

// ClearError clears any recorded error
func (is *IndexState) ClearError() {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.lastError = nil
	atomic.StoreInt64(&is.lastErrorTime, 0)
}

// Reset resets the index state to initial values
func (is *IndexState) Reset() {
	atomic.StoreInt32(&is.isIndexing, 0)
	atomic.StoreInt64(&is.lastUpdate, 0)
	atomic.StoreInt64(&is.updateCount, 0)
	atomic.StoreInt32(&is.currentReaders, 0)
	atomic.StoreInt32(&is.currentWriter, 0)
	atomic.StoreInt32(&is.waitingReaders, 0)
	atomic.StoreInt32(&is.waitingWriters, 0)
	atomic.StoreInt32(&is.queueDepth, 0)

	// Reset progress tracking (T033)
	atomic.StoreInt64(&is.progress, 0)
	atomic.StoreInt64(&is.startTime, 0)
	atomic.StoreInt64(&is.estimatedTime, 0)
	atomic.StoreInt64(&is.currentOperation, 0)
	atomic.StoreInt64(&is.filesProcessed, 0)
	atomic.StoreInt64(&is.totalFiles, 0)
	atomic.StoreInt64(&is.bytesProcessed, 0)
	atomic.StoreInt64(&is.totalBytes, 0)

	is.mu.Lock()
	is.lastError = nil
	atomic.StoreInt64(&is.lastErrorTime, 0)

	// Clear status history (T033)
	is.statusHistory = nil
	is.mu.Unlock()
}

// IndexStateRegistry manages a collection of index states
type IndexStateRegistry struct {
	states map[IndexType]*IndexState
	mu     sync.RWMutex
}

// NewIndexStateRegistry creates a new registry for managing index states
func NewIndexStateRegistry() *IndexStateRegistry {
	registry := &IndexStateRegistry{
		states: make(map[IndexType]*IndexState),
	}

	// Initialize all index types
	for _, indexType := range GetAllIndexTypes() {
		registry.states[indexType] = NewIndexState(indexType)
	}

	return registry
}

// GetIndexState returns the state for a specific index type
func (r *IndexStateRegistry) GetIndexState(indexType IndexType) *IndexState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[indexType]
	if !exists {
		// Create new state if it doesn't exist
		state = NewIndexState(indexType)
		r.mu.Lock()
		r.states[indexType] = state
		r.mu.Unlock()
	}

	return state
}

// GetAllStatus returns the status of all indexes
func (r *IndexStateRegistry) GetAllStatus() map[IndexType]IndexStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[IndexType]IndexStatus)
	for indexType, state := range r.states {
		result[indexType] = state.GetStatus()
	}

	return result
}

// ResetAll resets all index states to initial values
func (r *IndexStateRegistry) ResetAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, state := range r.states {
		state.Reset()
	}
}

// T033: Progress tracking methods for registry

// StartOperationOnAll starts tracking operations on all indexes
func (r *IndexStateRegistry) StartOperationOnAll(operationType int64, filesPerIndex map[IndexType]int64, bytesPerIndex map[IndexType]int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for indexType, state := range r.states {
		totalFiles := filesPerIndex[indexType]
		totalBytes := bytesPerIndex[indexType]
		state.StartOperation(operationType, totalFiles, totalBytes)
	}
}

// GetProgressSummary returns a summary of progress across all indexes (T033)
func (r *IndexStateRegistry) GetProgressSummary() map[IndexType]struct {
	Progress               float64
	FilesProcessed         int64
	TotalFiles             int64
	BytesProcessed         int64
	TotalBytes             int64
	IsIndexing             bool
	EstimatedTimeRemaining time.Duration
	IsComplete             bool
	ProgressStats          struct {
		Progress               float64
		FilesPerSecond         float64
		BytesPerSecond         float64
		Elapsed                time.Duration
		EstimatedTimeRemaining time.Duration
		IsComplete             bool
	}
} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summary := make(map[IndexType]struct {
		Progress               float64
		FilesProcessed         int64
		TotalFiles             int64
		BytesProcessed         int64
		TotalBytes             int64
		IsIndexing             bool
		EstimatedTimeRemaining time.Duration
		IsComplete             bool
		ProgressStats          struct {
			Progress               float64
			FilesPerSecond         float64
			BytesPerSecond         float64
			Elapsed                time.Duration
			EstimatedTimeRemaining time.Duration
			IsComplete             bool
		}
	})

	for indexType, state := range r.states {
		progressStats := state.GetProgressStats()
		summary[indexType] = struct {
			Progress               float64
			FilesProcessed         int64
			TotalFiles             int64
			BytesProcessed         int64
			TotalBytes             int64
			IsIndexing             bool
			EstimatedTimeRemaining time.Duration
			IsComplete             bool
			ProgressStats          struct {
				Progress               float64
				FilesPerSecond         float64
				BytesPerSecond         float64
				Elapsed                time.Duration
				EstimatedTimeRemaining time.Duration
				IsComplete             bool
			}
		}{
			Progress:               progressStats.Progress,
			FilesProcessed:         state.GetFilesProcessed(),
			TotalFiles:             state.GetTotalFiles(),
			BytesProcessed:         state.GetBytesProcessed(),
			TotalBytes:             state.GetTotalBytes(),
			IsIndexing:             state.IsIndexing(),
			EstimatedTimeRemaining: progressStats.EstimatedTimeRemaining,
			IsComplete:             progressStats.IsComplete,
			ProgressStats:          progressStats,
		}
	}

	return summary
}

// GetOverallProgress calculates overall progress across all indexes (T033)
func (r *IndexStateRegistry) GetOverallProgress() struct {
	OverallProgress        float64
	TotalFilesProcessed    int64
	TotalFiles             int64
	TotalBytesProcessed    int64
	TotalBytes             int64
	IndexesCompleted       int
	IndexesInProgress      int
	IndexesTotal           int
	EstimatedTimeRemaining time.Duration
	AverageFilesPerSecond  float64
	AverageBytesPerSecond  float64
} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalProgress float64
	var totalFilesProcessed int64
	var totalFiles int64
	var totalBytesProcessed int64
	var totalBytes int64
	var completedIndexes int
	var inProgressIndexes int
	var totalEstimatedTime time.Duration
	var totalElapsed time.Duration
	var fileOpsPerSecond float64
	var byteOpsPerSecond float64
	var indexCount int

	for _, state := range r.states {
		progressStats := state.GetProgressStats()
		progress := progressStats.Progress
		totalProgress += progress
		totalFilesProcessed += state.GetFilesProcessed()
		totalFiles += state.GetTotalFiles()
		totalBytesProcessed += state.GetBytesProcessed()
		totalBytes += state.GetTotalBytes()
		totalEstimatedTime += progressStats.EstimatedTimeRemaining
		totalElapsed += progressStats.Elapsed

		if progress >= 100 {
			completedIndexes++
		} else if progress > 0 {
			inProgressIndexes++
		}

		fileOpsPerSecond += progressStats.FilesPerSecond
		byteOpsPerSecond += progressStats.BytesPerSecond
		indexCount++
	}

	averageProgress := float64(0)
	if indexCount > 0 {
		averageProgress = totalProgress / float64(indexCount)
	}

	return struct {
		OverallProgress        float64
		TotalFilesProcessed    int64
		TotalFiles             int64
		TotalBytesProcessed    int64
		TotalBytes             int64
		IndexesCompleted       int
		IndexesInProgress      int
		IndexesTotal           int
		EstimatedTimeRemaining time.Duration
		AverageFilesPerSecond  float64
		AverageBytesPerSecond  float64
	}{
		OverallProgress:        averageProgress,
		TotalFilesProcessed:    totalFilesProcessed,
		TotalFiles:             totalFiles,
		TotalBytesProcessed:    totalBytesProcessed,
		TotalBytes:             totalBytes,
		IndexesCompleted:       completedIndexes,
		IndexesInProgress:      inProgressIndexes,
		IndexesTotal:           indexCount,
		EstimatedTimeRemaining: totalEstimatedTime,
		AverageFilesPerSecond:  fileOpsPerSecond,
		AverageBytesPerSecond:  byteOpsPerSecond,
	}
}

// GetAllDetailedStatus returns detailed status for all indexes (T033)
func (r *IndexStateRegistry) GetAllDetailedStatus() map[IndexType]IndexStatusSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[IndexType]IndexStatusSnapshot)
	for indexType, state := range r.states {
		result[indexType] = state.GetDetailedStatus()
	}

	return result
}
