package indexing

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	testutil "github.com/standardbeagle/lci/internal/testing"
	"github.com/standardbeagle/lci/testhelpers"
)

// TestRaceConditionFix verifies that the Clear() vs UpdateFile() race condition is fixed
func TestRaceConditionFix(t *testing.T) {
	tempDir := t.TempDir()
	cfg := testhelpers.NewTestConfigBuilder(tempDir).Build()
	gi := NewMasterIndex(cfg)

	// Create a test file in the temp directory
	testContent := []byte("package test\n\nfunc TestFunction() {}")
	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Index the file
	err := gi.IndexFile(filePath)
	if err != nil {
		t.Fatalf("Failed to index test file: %v", err)
	}

	var wg sync.WaitGroup
	const iterations = 100

	// Start concurrent Clear operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations/5; j++ {
				_ = gi.Clear()
				time.Sleep(1 * time.Microsecond) // Small delay to increase chance of contention
			}
		}()
	}

	// Start concurrent UpdateFile operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations/5; j++ {
				newContent := []byte("package test\n\nfunc UpdatedFunction" + string(rune('A'+id)) + "() {}")
				_ = gi.UpdateFile("test.go", newContent)
				time.Sleep(1 * time.Microsecond) // Small delay to increase chance of contention
			}
		}(i)
	}

	// Start concurrent IndexFile operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations/5; j++ {
				testFile := "test" + string(rune('A'+id)) + ".go"
				content := []byte("package test\n\nfunc TestFunction" + string(rune('A'+id)) + "() {}")
				_ = gi.fileService.LoadFileFromMemory(testFile, content)
				_ = gi.IndexFile(testFile)
				time.Sleep(1 * time.Microsecond)
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()

	t.Log("Race condition fix test completed without crashes")
}

// TestFineGrainedLocking verifies that snapshot and bulk operations don't block each other
func TestFineGrainedLocking(t *testing.T) {
	cfg := testhelpers.NewTestConfigBuilder(t.TempDir()).Build()
	gi := NewMasterIndex(cfg)

	// Load test files
	for i := 0; i < 10; i++ {
		filename := "test.go"
		content := []byte("package test\n\nfunc Function" + string(rune('0'+i)) + "() {}")
		_ = gi.fileService.LoadFileFromMemory(filename, content)
	}

	// Use stampede prevention retry for timing-sensitive test
	// Updated threshold from 10s to 5s with scaling for CI/race detector
	scaler := testutil.NewPerformanceScaler(t)
	threshold := time.Duration(scaler.ScaleDuration(5.0)) * time.Second

	testutil.RetryTimingAssertion(t, 2, func() (time.Duration, error) {
		var wg sync.WaitGroup
		startTime := time.Now()

		// Start concurrent snapshot operations (IndexFile, UpdateFile)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_ = gi.IndexFile("test.go")
			}(i)
		}

		// Start a bulk operation (IndexDirectory) - this should use bulkMu
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			// Use a very small directory to avoid timeout
			_ = gi.IndexDirectory(ctx, ".")
		}()

		// Wait for completion
		wg.Wait()

		duration := time.Since(startTime)
		return duration, nil
	}, threshold, "Fine-grained locking test")
}
