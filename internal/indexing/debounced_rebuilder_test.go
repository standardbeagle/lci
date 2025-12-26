package indexing

import (
	"sync"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// TestDebouncedRebuilder_BasicDebounce tests that rebuilds are triggered.
func TestDebouncedRebuilder_BasicDebounce(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 10) // Very short debounce
	defer rebuilder.Shutdown()

	done := make(chan struct{})
	rebuilder.SetOnRebuildComplete(func() {
		close(done)
	})

	rebuilder.ScheduleRebuild(types.FileID(1))
	rebuilder.ScheduleRebuild(types.FileID(2))
	rebuilder.ScheduleRebuild(types.FileID(3))

	if count := rebuilder.GetPendingCount(); count != 3 {
		t.Errorf("Expected 3 pending files, got %d", count)
	}

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for rebuild")
	}

	if count := rebuilder.GetPendingCount(); count != 0 {
		t.Errorf("Expected 0 pending files after rebuild, got %d", count)
	}
}

// TestDebouncedRebuilder_ForceRebuild tests immediate rebuild.
func TestDebouncedRebuilder_ForceRebuild(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 10000) // Very long debounce
	defer rebuilder.Shutdown()

	rebuilder.ScheduleRebuild(types.FileID(1))

	// Force immediate rebuild (no waiting)
	rebuilder.ForceRebuild()

	if count := rebuilder.GetPendingCount(); count != 0 {
		t.Errorf("Expected 0 pending files after force rebuild, got %d", count)
	}
}

// TestDebouncedRebuilder_SetDebounceTime tests changing debounce time.
func TestDebouncedRebuilder_SetDebounceTime(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 200)
	defer rebuilder.Shutdown()

	// Change to very short debounce
	rebuilder.SetDebounceTime(10)

	done := make(chan struct{})
	rebuilder.SetOnRebuildComplete(func() {
		close(done)
	})

	rebuilder.ScheduleRebuild(types.FileID(1))

	select {
	case <-done:
		// Success - short debounce worked
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout - debounce time change may not have worked")
	}
}

// TestDebouncedRebuilder_DefaultDebounce tests default debounce value.
func TestDebouncedRebuilder_DefaultDebounce(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 0) // Should default to 50ms
	defer rebuilder.Shutdown()

	// Just verify it doesn't crash and can complete
	done := make(chan struct{})
	rebuilder.SetOnRebuildComplete(func() {
		close(done)
	})

	rebuilder.ScheduleRebuild(types.FileID(1))

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout with default debounce")
	}
}

// TestDebouncedRebuilder_ConcurrentAccess tests concurrent scheduling.
func TestDebouncedRebuilder_ConcurrentAccess(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 10)
	defer rebuilder.Shutdown()

	var rebuilds sync.WaitGroup
	rebuilds.Add(1)
	rebuilder.SetOnRebuildComplete(func() {
		rebuilds.Done()
	})

	// Concurrent scheduling
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rebuilder.ScheduleRebuild(types.FileID(id))
		}(i)
	}

	wg.Wait() // Wait for all schedules

	rebuilds.Wait() // Wait for rebuild

	if count := rebuilder.GetPendingCount(); count != 0 {
		t.Errorf("Expected 0 pending files after concurrent rebuild, got %d", count)
	}
}

// TestDebouncedRebuilder_Shutdown tests clean shutdown.
func TestDebouncedRebuilder_Shutdown(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 10000)

	rebuilder.ScheduleRebuild(types.FileID(1))
	rebuilder.ScheduleRebuild(types.FileID(2))

	// Shutdown should not hang
	rebuilder.Shutdown()

	// Verify we can check pending count after shutdown (should be safe)
	_ = rebuilder.GetPendingCount()
}

// TestDebouncedRebuilder_NegativeDebounce tests negative debounce value.
func TestDebouncedRebuilder_NegativeDebounce(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, -100) // Should default to 50ms
	defer rebuilder.Shutdown()

	done := make(chan struct{})
	rebuilder.SetOnRebuildComplete(func() {
		close(done)
	})

	rebuilder.ScheduleRebuild(types.FileID(1))

	select {
	case <-done:
		// Success - negative value handled
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout - negative debounce not handled properly")
	}
}

// TestDebouncedRebuilder_DuplicateFiles tests duplicate file scheduling.
func TestDebouncedRebuilder_DuplicateFiles(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 10)
	defer rebuilder.Shutdown()

	// Schedule same file multiple times
	rebuilder.ScheduleRebuild(types.FileID(1))
	rebuilder.ScheduleRebuild(types.FileID(1))
	rebuilder.ScheduleRebuild(types.FileID(1))

	// Should only count once (map deduplication)
	if count := rebuilder.GetPendingCount(); count != 1 {
		t.Errorf("Expected 1 unique pending file, got %d", count)
	}

	done := make(chan struct{})
	rebuilder.SetOnRebuildComplete(func() {
		close(done)
	})

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout")
	}
}

// TestDebouncedRebuilder_EmptyRebuild tests rebuild with no pending files.
func TestDebouncedRebuilder_EmptyRebuild(t *testing.T) {
	refTracker := core.NewReferenceTrackerForTest()
	rebuilder := NewDebouncedRebuilder(refTracker, 10)
	defer rebuilder.Shutdown()

	// Force rebuild with no pending files - should not crash
	rebuilder.ForceRebuild()

	if count := rebuilder.GetPendingCount(); count != 0 {
		t.Errorf("Expected 0 pending files, got %d", count)
	}
}
