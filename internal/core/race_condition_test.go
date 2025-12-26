package core

import (
	"sync"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestFileSearchEngine_ConcurrentAccess tests concurrent access to FileSearchEngine
func TestFileSearchEngine_ConcurrentAccess(t *testing.T) {
	fse := NewFileSearchEngine()

	// Test concurrent IndexFile calls
	const numGoroutines = 100
	const numFiles = 1000

	var wg sync.WaitGroup
	fileIDs := make([]types.FileID, numFiles)

	// Generate file IDs and paths
	for i := 0; i < numFiles; i++ {
		fileIDs[i] = types.FileID(i + 1)
	}

	// Concurrent file indexing
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for j := 0; j < numFiles/numGoroutines; j++ {
				fileID := fileIDs[start+j]
				filePath := "/test/path/file.go"
				fse.IndexFile(fileID, filePath)
			}
		}(i * (numFiles / numGoroutines))
	}

	wg.Wait()

	// Verify all files were indexed
	stats := fse.GetStats()
	if stats["total_files"] != numFiles {
		t.Errorf("Expected %d files, got %v", numFiles, stats["total_files"])
	}

	// Test concurrent search calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results, err := fse.SearchFiles(types.FileSearchOptions{
				Pattern:    "*.go",
				Type:       "glob",
				MaxResults: 100,
			})
			if err != nil {
				t.Errorf("Search failed: %v", err)
			}
			if len(results) == 0 {
				t.Error("Expected search results, got none")
			}
		}()
	}

	wg.Wait()
}

// TestFileSearchEngine_ConcurrentReadWrites tests concurrent read/write operations
func TestFileSearchEngine_ConcurrentReadWrites(t *testing.T) {
	fse := NewFileSearchEngine()

	const numOps = 1000
	var wg sync.WaitGroup

	// Start concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps/5; j++ {
				fileID := types.FileID(id*1000 + j)
				filePath := "/test/path/file.go"
				fse.IndexFile(fileID, filePath)
			}
		}(i)
	}

	// Start concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOps/5; j++ {
				fse.GetStats()
				_, _ = fse.SearchFiles(types.FileSearchOptions{
					Pattern:    "*.go",
					Type:       "glob",
					MaxResults: 10,
				})
			}
		}()
	}

	wg.Wait()
}

// TestFileSearchEngine_ConcurrentResets tests concurrent reset operations
func TestFileSearchEngine_ConcurrentResets(t *testing.T) {
	fse := NewFileSearchEngine()

	// Index some initial data
	for i := 0; i < 100; i++ {
		fse.IndexFile(types.FileID(i), "/test/path/file.go")
	}

	var wg sync.WaitGroup

	// Start concurrent reset operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fse.Reset()
			// Add some data after reset
			for j := 0; j < 10; j++ {
				fse.IndexFile(types.FileID(j), "/test/path/file.go")
			}
		}()
	}

	// Start concurrent read operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				fse.GetStats()
			}
		}()
	}

	wg.Wait()
}

// TestFileSearchEngine_ClearRace tests for race conditions in Clear
func TestFileSearchEngine_ClearRace(t *testing.T) {
	fse := NewFileSearchEngine()

	// Index initial data
	for i := 0; i < 100; i++ {
		fse.IndexFile(types.FileID(i), "/test/path/file.go")
	}

	var wg sync.WaitGroup
	done := make(chan bool, 2)

	// Start concurrent clear operations
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			fse.Clear()
		}
		done <- true
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			fileID := types.FileID(i)
			filePath := "/test/path/file.go"
			fse.IndexFile(fileID, filePath)
		}
		done <- true
	}()

	// Wait for both to signal completion
	<-done
	<-done
	wg.Wait()
}
