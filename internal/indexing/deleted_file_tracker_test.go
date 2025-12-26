package indexing

import (
	"sync"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestDeletedFileTracker_NewTracker(t *testing.T) {
	tracker := NewDeletedFileTracker()
	if tracker == nil {
		t.Fatal("NewDeletedFileTracker returned nil")
	}
	if tracker.GetDeletedCount() != 0 {
		t.Errorf("expected 0 deleted files, got %d", tracker.GetDeletedCount())
	}
}

func TestDeletedFileTracker_MarkDeleted(t *testing.T) {
	tracker := NewDeletedFileTracker()

	// Mark a single file as deleted
	tracker.MarkDeleted(types.FileID(1))
	if !tracker.IsDeleted(types.FileID(1)) {
		t.Error("file 1 should be marked as deleted")
	}
	if tracker.GetDeletedCount() != 1 {
		t.Errorf("expected 1 deleted file, got %d", tracker.GetDeletedCount())
	}

	// Mark another file
	tracker.MarkDeleted(types.FileID(2))
	if !tracker.IsDeleted(types.FileID(2)) {
		t.Error("file 2 should be marked as deleted")
	}
	if tracker.GetDeletedCount() != 2 {
		t.Errorf("expected 2 deleted files, got %d", tracker.GetDeletedCount())
	}

	// Marking same file again should be idempotent
	tracker.MarkDeleted(types.FileID(1))
	if tracker.GetDeletedCount() != 2 {
		t.Errorf("expected 2 deleted files after re-marking, got %d", tracker.GetDeletedCount())
	}
}

func TestDeletedFileTracker_MarkDeletedBatch(t *testing.T) {
	tracker := NewDeletedFileTracker()

	fileIDs := []types.FileID{1, 2, 3, 4, 5}
	tracker.MarkDeletedBatch(fileIDs)

	if tracker.GetDeletedCount() != 5 {
		t.Errorf("expected 5 deleted files, got %d", tracker.GetDeletedCount())
	}

	for _, id := range fileIDs {
		if !tracker.IsDeleted(id) {
			t.Errorf("file %d should be marked as deleted", id)
		}
	}

	// Adding overlapping batch should work
	tracker.MarkDeletedBatch([]types.FileID{4, 5, 6, 7})
	if tracker.GetDeletedCount() != 7 {
		t.Errorf("expected 7 deleted files, got %d", tracker.GetDeletedCount())
	}
}

func TestDeletedFileTracker_IsDeleted(t *testing.T) {
	tracker := NewDeletedFileTracker()

	// Non-existent file should not be deleted
	if tracker.IsDeleted(types.FileID(100)) {
		t.Error("file 100 should not be deleted initially")
	}

	tracker.MarkDeleted(types.FileID(100))
	if !tracker.IsDeleted(types.FileID(100)) {
		t.Error("file 100 should be deleted after marking")
	}
}

func TestDeletedFileTracker_FilterCandidates(t *testing.T) {
	tracker := NewDeletedFileTracker()

	// Mark some files as deleted
	tracker.MarkDeleted(types.FileID(2))
	tracker.MarkDeleted(types.FileID(4))

	candidates := []types.FileID{1, 2, 3, 4, 5}
	filtered := tracker.FilterCandidates(candidates)

	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered candidates, got %d", len(filtered))
	}

	// Check that deleted files are not in the result
	for _, id := range filtered {
		if id == 2 || id == 4 {
			t.Errorf("deleted file %d should not be in filtered results", id)
		}
	}

	// Check that non-deleted files are in the result
	found := make(map[types.FileID]bool)
	for _, id := range filtered {
		found[id] = true
	}
	for _, expected := range []types.FileID{1, 3, 5} {
		if !found[expected] {
			t.Errorf("file %d should be in filtered results", expected)
		}
	}
}

func TestDeletedFileTracker_FilterCandidates_Empty(t *testing.T) {
	tracker := NewDeletedFileTracker()

	// No deleted files - should return original slice
	candidates := []types.FileID{1, 2, 3}
	filtered := tracker.FilterCandidates(candidates)

	if len(filtered) != len(candidates) {
		t.Errorf("expected %d candidates when no deletions, got %d", len(candidates), len(filtered))
	}
}

func TestDeletedFileTracker_Clear(t *testing.T) {
	tracker := NewDeletedFileTracker()

	// Mark some files
	tracker.MarkDeletedBatch([]types.FileID{1, 2, 3, 4, 5})
	if tracker.GetDeletedCount() != 5 {
		t.Errorf("expected 5 deleted files, got %d", tracker.GetDeletedCount())
	}

	// Clear
	tracker.Clear()
	if tracker.GetDeletedCount() != 0 {
		t.Errorf("expected 0 deleted files after clear, got %d", tracker.GetDeletedCount())
	}

	// Previously deleted files should no longer be marked
	if tracker.IsDeleted(types.FileID(1)) {
		t.Error("file 1 should not be deleted after clear")
	}
}

func TestDeletedFileTracker_GetDeletedFileIDs(t *testing.T) {
	tracker := NewDeletedFileTracker()

	fileIDs := []types.FileID{10, 20, 30}
	tracker.MarkDeletedBatch(fileIDs)

	deleted := tracker.GetDeletedFileIDs()
	if len(deleted) != 3 {
		t.Errorf("expected 3 deleted file IDs, got %d", len(deleted))
	}

	// Check all IDs are present (order may vary)
	found := make(map[types.FileID]bool)
	for _, id := range deleted {
		found[id] = true
	}
	for _, expected := range fileIDs {
		if !found[expected] {
			t.Errorf("file ID %d should be in deleted list", expected)
		}
	}
}

func TestDeletedFileTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewDeletedFileTracker()
	var wg sync.WaitGroup
	numGoroutines := 100
	filesPerGoroutine := 10

	// Concurrent marking
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < filesPerGoroutine; j++ {
				tracker.MarkDeleted(types.FileID(base*filesPerGoroutine + j))
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := numGoroutines * filesPerGoroutine
	if tracker.GetDeletedCount() != expectedTotal {
		t.Errorf("expected %d deleted files, got %d", expectedTotal, tracker.GetDeletedCount())
	}
}

func TestDeletedFileTracker_ConcurrentMarkAndFilter(t *testing.T) {
	tracker := NewDeletedFileTracker()
	var wg sync.WaitGroup

	// Pre-populate some deleted files
	tracker.MarkDeletedBatch([]types.FileID{1, 3, 5, 7, 9})

	candidates := []types.FileID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Start concurrent readers and writers
	for i := 0; i < 50; i++ {
		wg.Add(2)

		// Writer: mark more files as deleted
		go func(id int) {
			defer wg.Done()
			tracker.MarkDeleted(types.FileID(100 + id))
		}(i)

		// Reader: filter candidates
		go func() {
			defer wg.Done()
			filtered := tracker.FilterCandidates(candidates)
			// At minimum, files 1,3,5,7,9 should be filtered out
			// More may be filtered as new deletions occur
			if len(filtered) > 5 {
				// This is expected - only the pre-populated deletions affect candidates
			}
		}()
	}

	wg.Wait()

	// Verify final state
	if tracker.GetDeletedCount() < 5 {
		t.Errorf("expected at least 5 deleted files, got %d", tracker.GetDeletedCount())
	}
}

func TestDeletedFileSet_Contains(t *testing.T) {
	set := newDeletedFileSet()

	// Empty set should not contain anything
	if set.Contains(types.FileID(1)) {
		t.Error("empty set should not contain file 1")
	}

	// Len should be 0
	if set.Len() != 0 {
		t.Errorf("expected length 0, got %d", set.Len())
	}
}

func BenchmarkDeletedFileTracker_MarkDeleted(b *testing.B) {
	tracker := NewDeletedFileTracker()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.MarkDeleted(types.FileID(i))
	}
}

func BenchmarkDeletedFileTracker_IsDeleted(b *testing.B) {
	tracker := NewDeletedFileTracker()
	// Pre-populate with some deleted files
	for i := 0; i < 1000; i++ {
		tracker.MarkDeleted(types.FileID(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.IsDeleted(types.FileID(i % 2000))
	}
}

func BenchmarkDeletedFileTracker_FilterCandidates(b *testing.B) {
	tracker := NewDeletedFileTracker()
	// Mark every 10th file as deleted
	for i := 0; i < 100; i++ {
		tracker.MarkDeleted(types.FileID(i * 10))
	}

	candidates := make([]types.FileID, 1000)
	for i := 0; i < 1000; i++ {
		candidates[i] = types.FileID(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.FilterCandidates(candidates)
	}
}

func BenchmarkDeletedFileTracker_ConcurrentMark(b *testing.B) {
	tracker := NewDeletedFileTracker()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			tracker.MarkDeleted(types.FileID(id))
			id++
		}
	})
}
