package core

import (
	"fmt"
	"runtime"
	"github.com/standardbeagle/lci/internal/types"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLockFreeBasicOperations tests basic CRUD operations
func TestLockFreeBasicOperations(t *testing.T) {
	store := NewFileContentStore()
	defer store.Close()

	// Test LoadFile
	content1 := []byte("Hello, World!\nLine 2\nLine 3")
	fileID1 := store.LoadFile("test1.txt", content1)
	if fileID1 == 0 {
		t.Fatal("Expected valid FileID")
	}

	// Test GetContent
	retrieved, ok := store.GetContent(fileID1)
	if !ok {
		t.Fatal("Failed to retrieve content")
	}
	if string(retrieved) != string(content1) {
		t.Errorf("Content mismatch: got %q, want %q", string(retrieved), string(content1))
	}

	// Test GetLineOffsets
	offsets, ok := store.GetLineOffsets(fileID1)
	if !ok {
		t.Fatal("Failed to retrieve line offsets")
	}
	if len(offsets) != 3 {
		t.Errorf("Expected 3 line offsets, got %d", len(offsets))
	}

	// Test GetLineCount
	lineCount := store.GetLineCount(fileID1)
	if lineCount != 3 {
		t.Errorf("Expected 3 lines, got %d", lineCount)
	}

	// Test InvalidateFile
	store.InvalidateFile("test1.txt")
	_, ok = store.GetContent(fileID1)
	if ok {
		t.Error("Content should not be available after invalidation")
	}
}

// TestLockFreeConcurrentReads tests concurrent read operations for race conditions
func TestLockFreeConcurrentReads(t *testing.T) {
	store := NewFileContentStore()
	defer store.Close()

	// Load test data
	numFiles := 100
	fileIDs := make([]types.FileID, numFiles)
	for i := 0; i < numFiles; i++ {
		content := []byte(fmt.Sprintf("File %d content\nLine 2\nLine 3", i))
		fileIDs[i] = store.LoadFile(fmt.Sprintf("file%d.txt", i), content)
	}

	// Wait for all files to be loaded
	time.Sleep(10 * time.Millisecond)

	// Concurrent readers
	numReaders := 50
	numReadsPerReader := 1000
	var wg sync.WaitGroup
	errorCount := atomic.Int32{}

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < numReadsPerReader; j++ {
				fileIdx := (readerID + j) % numFiles
				fileID := fileIDs[fileIdx]

				// Test GetContent
				_, ok := store.GetContent(fileID)
				if !ok {
					errorCount.Add(1)
					continue
				}

				// Test GetLineOffsets
				offsets, ok := store.GetLineOffsets(fileID)
				if !ok || len(offsets) != 3 {
					errorCount.Add(1)
					continue
				}

				// Test GetLine
				line, ok := store.GetLine(fileID, 0)
				if !ok {
					errorCount.Add(1)
					continue
				}

				// Test GetString
				str, err := store.GetString(line)
				if err != nil {
					errorCount.Add(1)
					continue
				}

				expectedContent := fmt.Sprintf("File %d content", fileIdx)
				if str != expectedContent {
					errorCount.Add(1)
				}

				// Yield to increase chances of race detection
				if j%100 == 0 {
					runtime.Gosched()
				}
			}
		}(i)
	}

	wg.Wait()

	if errors := errorCount.Load(); errors > 0 {
		t.Errorf("Encountered %d errors during concurrent reads", errors)
	}
}

// TestLockFreeReadWriteRace tests concurrent reads with writes
func TestLockFreeReadWriteRace(t *testing.T) {
	store := NewFileContentStore()
	defer store.Close()

	// Initial data
	initialContent := []byte("Initial content\nLine 2")
	fileID := store.LoadFile("test.txt", initialContent)

	stopChan := make(chan struct{})
	var wg sync.WaitGroup

	// Writer goroutine - continuously updates the file
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateCount := 0
		for {
			select {
			case <-stopChan:
				return
			default:
				newContent := []byte(fmt.Sprintf("Update %d\nLine 2\nLine 3", updateCount))
				store.LoadFile("test.txt", newContent)
				updateCount++
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Multiple reader goroutines
	numReaders := 10
	readErrors := atomic.Int32{}

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopChan:
					return
				default:
					// Perform various read operations
					content, ok := store.GetContent(fileID)
					if !ok {
						// File might be invalidated, this is OK
						continue
					}

					// Verify content is not corrupted
					if len(content) == 0 {
						readErrors.Add(1)
					}

					offsets, _ := store.GetLineOffsets(fileID)
					if offsets != nil && len(offsets) > 10 {
						readErrors.Add(1) // Suspicious number of lines
					}

					lineCount := store.GetLineCount(fileID)
					if lineCount < 0 || lineCount > 10 {
						readErrors.Add(1)
					}

					runtime.Gosched()
				}
			}
		}()
	}

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	if errors := readErrors.Load(); errors > 0 {
		t.Errorf("Encountered %d read errors during concurrent read/write", errors)
	}
}

// TestLockFreeBatchOperations tests batch loading
func TestLockFreeBatchOperations(t *testing.T) {
	store := NewFileContentStore()
	defer store.Close()

	// Prepare batch data
	files := []struct {
		Path    string
		Content []byte
	}{
		{"file1.txt", []byte("Content 1")},
		{"file2.txt", []byte("Content 2\nLine 2")},
		{"file3.txt", []byte("Content 3\nLine 2\nLine 3")},
	}

	// Batch load
	fileIDs := store.BatchLoadFiles(files)
	if len(fileIDs) != 3 {
		t.Fatalf("Expected 3 file IDs, got %d", len(fileIDs))
	}

	// Verify all files loaded correctly
	for i, id := range fileIDs {
		content, ok := store.GetContent(id)
		if !ok {
			t.Errorf("Failed to get content for file %d", i)
			continue
		}
		if string(content) != string(files[i].Content) {
			t.Errorf("Content mismatch for file %d", i)
		}
	}
}

// TestLockFreeMemoryLimit tests LRU eviction under memory pressure
func TestLockFreeMemoryLimit(t *testing.T) {
	// Small memory limit to trigger eviction
	store := NewFileContentStoreWithLimit(1024) // 1KB limit
	defer store.Close()

	// Load files that exceed the limit
	largeContent := make([]byte, 512) // 512 bytes each
	for i := 0; i < len(largeContent); i++ {
		largeContent[i] = byte('A' + (i % 26))
	}

	fileID1 := store.LoadFile("file1.txt", largeContent)
	fileID2 := store.LoadFile("file2.txt", largeContent)
	fileID3 := store.LoadFile("file3.txt", largeContent) // Should trigger eviction

	// Allow time for updates to process
	time.Sleep(10 * time.Millisecond)

	// At least one file should be evicted
	_, ok1 := store.GetContent(fileID1)
	_, ok2 := store.GetContent(fileID2)
	_, ok3 := store.GetContent(fileID3)

	evictedCount := 0
	if !ok1 {
		evictedCount++
	}
	if !ok2 {
		evictedCount++
	}
	if !ok3 {
		evictedCount++
	}

	if evictedCount == 0 {
		t.Error("At least one file should have been evicted")
	}
	if evictedCount == 3 {
		t.Error("Not all files should be evicted")
	}

	// Check memory usage is within limit
	memUsage := store.GetMemoryUsage()
	if memUsage > 1536 { // Allow some overhead
		t.Errorf("Memory usage %d exceeds expected limit", memUsage)
	}
}

// TestLockFreeStringRefOperations tests StringRef functionality
func TestLockFreeStringRefOperations(t *testing.T) {
	store := NewFileContentStore()
	defer store.Close()

	content := []byte("Hello, World!\nSecond line\nThird line")
	fileID := store.LoadFile("test.txt", content)

	// Test GetLine
	lineRef, ok := store.GetLine(fileID, 1) // Second line
	if !ok {
		t.Fatal("Failed to get line reference")
	}

	// Test GetString
	lineStr, err := store.GetString(lineRef)
	if err != nil {
		t.Fatalf("Failed to get string: %v", err)
	}
	if lineStr != "Second line" {
		t.Errorf("Line content mismatch: got %q, want %q", lineStr, "Second line")
	}

	// Test GetBytes
	lineBytes, err := store.GetBytes(lineRef)
	if err != nil {
		t.Fatalf("Failed to get bytes: %v", err)
	}
	if string(lineBytes) != "Second line" {
		t.Errorf("Line bytes mismatch: got %q, want %q", string(lineBytes), "Second line")
	}
}

// TestLockFreeNoDeadlock ensures no deadlocks occur
func TestLockFreeNoDeadlock(t *testing.T) {
	store := NewFileContentStore()
	defer store.Close()

	// Create a scenario that might cause deadlock in a lock-based implementation
	var wg sync.WaitGroup

	// Rapid file updates
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			content := []byte(fmt.Sprintf("Content %d", i))
			store.LoadFile("rapid.txt", content)
		}
	}()

	// Rapid invalidations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			store.InvalidateFile("rapid.txt")
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Rapid reads
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			store.GetContent(types.FileID(1))
			runtime.Gosched()
		}
	}()

	// Clear operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			store.Clear()
		}
	}()

	// Set a timeout to detect deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

// BenchmarkLockFreeVsLocked compares performance (run with existing implementation)
func BenchmarkLockFreeReads(b *testing.B) {
	store := NewFileContentStore()
	defer store.Close()

	// Load test data
	numFiles := 1000
	fileIDs := make([]types.FileID, numFiles)
	for i := 0; i < numFiles; i++ {
		content := []byte(fmt.Sprintf("File %d content with some text", i))
		fileIDs[i] = store.LoadFile(fmt.Sprintf("file%d.txt", i), content)
	}

	// Allow updates to complete
	time.Sleep(10 * time.Millisecond)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			fileID := fileIDs[i%numFiles]
			store.GetContent(fileID)
			i++
		}
	})
}

// BenchmarkLockFreeConcurrentReadWrite benchmarks mixed operations
func BenchmarkLockFreeConcurrentReadWrite(b *testing.B) {
	store := NewFileContentStore()
	defer store.Close()

	// Initial data
	fileID := store.LoadFile("bench.txt", []byte("Initial content"))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				// 10% writes
				content := []byte(fmt.Sprintf("Update %d", i))
				store.LoadFile("bench.txt", content)
			} else {
				// 90% reads
				store.GetContent(fileID)
			}
			i++
		}
	})
}

// TestFileContentStoreCloseEdgeCases tests edge cases for Close() method
func TestFileContentStoreCloseEdgeCases(t *testing.T) {
	t.Run("Multiple Close Calls", func(t *testing.T) {
		store := NewFileContentStore()

		// Load some data
		content := []byte("test content")
		store.LoadFile("test.txt", content)

		// Call Close() multiple times - should not panic
		store.Close()
		store.Close()
		store.Close()

		// Verify the store is closed (attempting operations should not hang)
		done := make(chan bool)
		go func() {
			store.Clear() // Should not hang even after Close()
			done <- true
		}()

		select {
		case <-done:
			// Good, Clear() didn't hang
		case <-time.After(1 * time.Second):
			t.Fatal("Clear() hung after Close()")
		}
	})

	t.Run("Concurrent Close Calls", func(t *testing.T) {
		store := NewFileContentStore()

		// Load some data
		content := []byte("test content")
		store.LoadFile("test.txt", content)

		// Call Close() concurrently - should not panic or race
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				store.Close()
			}()
		}
		wg.Wait()
	})

	t.Run("Clear After Close", func(t *testing.T) {
		store := NewFileContentStore()

		// Load some data
		content := []byte("test content")
		fileID := store.LoadFile("test.txt", content)

		// Verify data is loaded
		if _, ok := store.GetContent(fileID); !ok {
			t.Fatal("Content should be available before Close()")
		}

		// Close the store
		store.Close()

		// Clear should not hang after Close
		done := make(chan bool)
		go func() {
			store.Clear()
			done <- true
		}()

		select {
		case <-done:
			// Good, Clear() completed
		case <-time.After(1 * time.Second):
			t.Fatal("Clear() hung after Close()")
		}
	})

	t.Run("Operations After Close", func(t *testing.T) {
		store := NewFileContentStore()
		store.Close()

		// These operations should not hang or panic after Close()
		done := make(chan bool)
		go func() {
			// LoadFile after Close
			fileID := store.LoadFile("test.txt", []byte("content"))
			// Should return but might not succeed
			_ = fileID

			// Clear after Close
			store.Clear()

			done <- true
		}()

		select {
		case <-done:
			// Good, operations completed (even if unsuccessfully)
		case <-time.After(1 * time.Second):
			t.Fatal("Operations hung after Close()")
		}
	})

	t.Run("Close During Concurrent Operations", func(t *testing.T) {
		store := NewFileContentStore()

		// Start concurrent operations
		stop := make(chan bool)
		var wg sync.WaitGroup

		// Writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						content := []byte(fmt.Sprintf("content %d", id))
						store.LoadFile(fmt.Sprintf("file%d.txt", id), content)
						time.Sleep(1 * time.Millisecond)
					}
				}
			}(i)
		}

		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						store.GetContent(types.FileID(1))
						time.Sleep(1 * time.Millisecond)
					}
				}
			}()
		}

		// Let operations run briefly
		time.Sleep(10 * time.Millisecond)

		// Close during operations
		store.Close()

		// Stop operations
		close(stop)

		// Wait for goroutines to finish
		done := make(chan bool)
		go func() {
			wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			// Good, all operations completed
		case <-time.After(2 * time.Second):
			t.Fatal("Operations didn't complete after Close()")
		}
	})
}