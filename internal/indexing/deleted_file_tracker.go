package indexing

import (
	"sync/atomic"

	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

// DeletedFileSet represents an immutable snapshot of deleted file IDs for lock-free reads.
// Uses the same copy-on-write pattern as FileSnapshot.
type DeletedFileSet struct {
	files map[types.FileID]struct{}
}

// newDeletedFileSet creates a new empty deleted file set
func newDeletedFileSet() *DeletedFileSet {
	return &DeletedFileSet{
		files: make(map[types.FileID]struct{}),
	}
}

// Contains checks if a file ID is in the deleted set (lock-free read)
func (d *DeletedFileSet) Contains(fileID types.FileID) bool {
	_, exists := d.files[fileID]
	return exists
}

// Len returns the number of deleted files in the set
func (d *DeletedFileSet) Len() int {
	return len(d.files)
}

// DeletedFileTracker manages tracking of deleted files between full index rebuilds.
// It uses a lock-free atomic pointer pattern for the deleted files set, similar to
// how FileSnapshot works, ensuring fast concurrent reads during search operations.
//
// The tracker integrates with the DebouncedRebuilder schedule - when the rebuilder
// triggers a full reindex, the deleted files set should be cleared.
type DeletedFileTracker struct {
	// Atomic pointer for lock-free reads of the deleted files set
	deletedFiles atomic.Pointer[DeletedFileSet]
}

// NewDeletedFileTracker creates a new deleted file tracker
func NewDeletedFileTracker() *DeletedFileTracker {
	tracker := &DeletedFileTracker{}
	tracker.deletedFiles.Store(newDeletedFileSet())
	return tracker
}

// MarkDeleted adds a file ID to the deleted files set using copy-on-write
func (dt *DeletedFileTracker) MarkDeleted(fileID types.FileID) {
	for {
		oldSet := dt.deletedFiles.Load()

		// Check if already marked (avoid unnecessary copy)
		if oldSet.Contains(fileID) {
			return
		}

		// Create new set with the file added (copy-on-write)
		newSet := &DeletedFileSet{
			files: make(map[types.FileID]struct{}, len(oldSet.files)+1),
		}
		for id := range oldSet.files {
			newSet.files[id] = struct{}{}
		}
		newSet.files[fileID] = struct{}{}

		// Atomically swap - retry if another goroutine modified it
		if dt.deletedFiles.CompareAndSwap(oldSet, newSet) {
			debug.LogIndexing("Marked file %d as deleted (total deleted: %d)\n", fileID, len(newSet.files))
			return
		}
		// CAS failed, another goroutine modified the set, retry
	}
}

// MarkDeletedBatch adds multiple file IDs to the deleted files set using copy-on-write
func (dt *DeletedFileTracker) MarkDeletedBatch(fileIDs []types.FileID) {
	if len(fileIDs) == 0 {
		return
	}

	for {
		oldSet := dt.deletedFiles.Load()

		// Count how many new files we need to add
		newCount := 0
		for _, fileID := range fileIDs {
			if !oldSet.Contains(fileID) {
				newCount++
			}
		}

		// If all files already marked, nothing to do
		if newCount == 0 {
			return
		}

		// Create new set with all files added (copy-on-write)
		newSet := &DeletedFileSet{
			files: make(map[types.FileID]struct{}, len(oldSet.files)+newCount),
		}
		for id := range oldSet.files {
			newSet.files[id] = struct{}{}
		}
		for _, fileID := range fileIDs {
			newSet.files[fileID] = struct{}{}
		}

		// Atomically swap - retry if another goroutine modified it
		if dt.deletedFiles.CompareAndSwap(oldSet, newSet) {
			debug.LogIndexing("Marked %d files as deleted (total deleted: %d)\n", newCount, len(newSet.files))
			return
		}
		// CAS failed, another goroutine modified the set, retry
	}
}

// IsDeleted checks if a file ID has been marked as deleted (lock-free read)
func (dt *DeletedFileTracker) IsDeleted(fileID types.FileID) bool {
	return dt.deletedFiles.Load().Contains(fileID)
}

// GetDeletedSet returns the current deleted file set (lock-free atomic read)
// This method exists to support the symbol filtering interface
// Returns the set as an interface to avoid circular dependencies with core package
func (dt *DeletedFileTracker) GetDeletedSet() interface {
	Contains(types.FileID) bool
	Len() int
} {
	return dt.deletedFiles.Load()
}

// FilterCandidates returns a new slice with deleted files removed.
// This is the primary method used during search to filter out stale index entries.
func (dt *DeletedFileTracker) FilterCandidates(candidates []types.FileID) []types.FileID {
	deletedSet := dt.deletedFiles.Load()

	// Fast path: no deleted files
	if deletedSet.Len() == 0 {
		return candidates
	}

	// Filter out deleted files
	result := make([]types.FileID, 0, len(candidates))
	for _, fileID := range candidates {
		if !deletedSet.Contains(fileID) {
			result = append(result, fileID)
		}
	}

	return result
}

// Clear resets the deleted files set. Called when a full reindex completes.
func (dt *DeletedFileTracker) Clear() {
	oldSet := dt.deletedFiles.Load()
	oldCount := oldSet.Len()

	dt.deletedFiles.Store(newDeletedFileSet())

	if oldCount > 0 {
		debug.LogIndexing("Cleared deleted file tracker (was tracking %d deleted files)\n", oldCount)
	}
}

// GetDeletedCount returns the number of files currently marked as deleted
func (dt *DeletedFileTracker) GetDeletedCount() int {
	return dt.deletedFiles.Load().Len()
}

// GetDeletedFileIDs returns a copy of all deleted file IDs (for diagnostics)
func (dt *DeletedFileTracker) GetDeletedFileIDs() []types.FileID {
	deletedSet := dt.deletedFiles.Load()
	result := make([]types.FileID, 0, deletedSet.Len())
	for id := range deletedSet.files {
		result = append(result, id)
	}
	return result
}
