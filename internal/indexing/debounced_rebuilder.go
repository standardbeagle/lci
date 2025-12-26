package indexing

import (
	"context"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

// DebouncedRebuilder manages debounced rebuilding of global index structures
type DebouncedRebuilder struct {
	refTracker *core.ReferenceTracker

	// Debounce settings
	debounceTime time.Duration
	timer        *time.Timer
	mu           sync.Mutex

	// Files that need rebuilding
	pendingFiles map[types.FileID]bool

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Optional callback for test synchronization
	onRebuildComplete func()
}

// NewDebouncedRebuilder creates a new debounced rebuilder
func NewDebouncedRebuilder(refTracker *core.ReferenceTracker, debounceMs int) *DebouncedRebuilder {
	if debounceMs <= 0 {
		debounceMs = 50 // Default to 50ms
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DebouncedRebuilder{
		refTracker:   refTracker,
		debounceTime: time.Duration(debounceMs) * time.Millisecond,
		pendingFiles: make(map[types.FileID]bool),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// ScheduleRebuild schedules a rebuild for the given file after the debounce period
func (dr *DebouncedRebuilder) ScheduleRebuild(fileID types.FileID) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Add file to pending set
	dr.pendingFiles[fileID] = true

	// Reset timer
	if dr.timer != nil {
		dr.timer.Stop()
	}

	dr.timer = time.AfterFunc(dr.debounceTime, dr.performRebuild)

	debug.LogIndexing("Scheduled rebuild for file %d (pending: %d files)\n", fileID, len(dr.pendingFiles))
}

// performRebuild executes the rebuild for all pending files
func (dr *DebouncedRebuilder) performRebuild() {
	dr.mu.Lock()
	files := dr.pendingFiles
	dr.pendingFiles = make(map[types.FileID]bool)
	callback := dr.onRebuildComplete
	dr.mu.Unlock()

	if len(files) == 0 {
		return
	}

	debug.LogIndexing("Starting debounced rebuild for %d files\n", len(files))
	startTime := time.Now()

	// For now, we'll do a full ProcessAllReferences rebuild
	// In the future, this could be optimized to only rebuild affected references
	dr.refTracker.ProcessAllReferences()

	duration := time.Since(startTime)
	debug.LogIndexing("Completed debounced rebuild in %v\n", duration)

	// Notify tests that rebuild is complete
	if callback != nil {
		callback()
	}
}

// Shutdown stops the debounced rebuilder
func (dr *DebouncedRebuilder) Shutdown() {
	dr.cancel()

	dr.mu.Lock()
	if dr.timer != nil {
		dr.timer.Stop()
	}
	dr.mu.Unlock()

	dr.wg.Wait()
}

// SetDebounceTime updates the debounce time
func (dr *DebouncedRebuilder) SetDebounceTime(ms int) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.debounceTime = time.Duration(ms) * time.Millisecond
}

// GetPendingCount returns the number of files pending rebuild
func (dr *DebouncedRebuilder) GetPendingCount() int {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	return len(dr.pendingFiles)
}

// ForceRebuild immediately triggers a rebuild without waiting for debounce
func (dr *DebouncedRebuilder) ForceRebuild() {
	dr.mu.Lock()
	if dr.timer != nil {
		dr.timer.Stop()
	}
	dr.mu.Unlock()

	dr.performRebuild()
}

// SetOnRebuildComplete sets a callback to be invoked when rebuild completes (for testing).
func (dr *DebouncedRebuilder) SetOnRebuildComplete(callback func()) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.onRebuildComplete = callback
}
