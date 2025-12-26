package core

import (
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// Simple performance test to validate invalidation speed improvement
func TestInvalidationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	const numFiles = 100
	const fileSize = 500

	// Create test data
	files := make(map[types.FileID][]byte)
	for i := 0; i < numFiles; i++ {
		fileID := types.FileID(i + 1)
		content := generateTestContent(fileSize, i)
		files[fileID] = content
	}

	// Test invalidation approach
	invalidationIndex := NewTrigramIndex()
	for fileID, content := range files {
		invalidationIndex.IndexFile(fileID, content)
	}

	start := time.Now()
	// Remove 20% of files using invalidation
	filesToRemove := numFiles / 5
	for i := 0; i < filesToRemove; i++ {
		fileID := types.FileID(i + 1)
		content := files[fileID]
		invalidationIndex.RemoveFile(fileID, content)
	}
	invalidationTime := time.Since(start)

	// Test immediate cleanup approach
	cleanupIndex := NewTrigramIndex()
	for fileID, content := range files {
		cleanupIndex.IndexFile(fileID, content)
	}

	start = time.Now()
	// Remove same files with immediate cleanup
	for i := 0; i < filesToRemove; i++ {
		fileID := types.FileID(i + 1)
		content := files[fileID]
		cleanupIndex.RemoveFile(fileID, content)
		cleanupIndex.ForceCleanup() // Simulate old expensive approach
	}
	cleanupTime := time.Since(start)

	// Verify both approaches produce same results after cleanup
	invalidationIndex.ForceCleanup()

	invalidationCandidates := invalidationIndex.FindCandidates("test")
	cleanupCandidates := cleanupIndex.FindCandidates("test")

	if len(invalidationCandidates) != len(cleanupCandidates) {
		t.Errorf("Different result counts: invalidation=%d, cleanup=%d",
			len(invalidationCandidates), len(cleanupCandidates))
	}

	t.Logf("Invalidation approach: %v", invalidationTime)
	t.Logf("Immediate cleanup approach: %v", cleanupTime)

	if cleanupTime > 0 {
		speedup := float64(cleanupTime) / float64(invalidationTime)
		t.Logf("Speedup: %.2fx", speedup)

		// Invalidation should be significantly faster
		if speedup < 2.0 {
			t.Logf("Warning: Expected >2x speedup, got %.2fx", speedup)
		}
	}
}
