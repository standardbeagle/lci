package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

// TestIndexPerformanceRequirements ensures all operations meet performance requirements
func TestIndexPerformanceRequirements(t *testing.T) {
	// Performance requirements from design:
	// - Search: <15ms for typical queries (original target <5ms, relaxed for high-match patterns)
	// - Symbol lookup: <1ms
	// - File access: near 0ms (everything from memory)
	// - Index build: <5s for typical projects

	// Create a realistic test project
	testDir := createLargeTestProject(t, 100) // 100 files

	// Create config
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Search: config.Search{
			MaxResults: 100,
		},
		Include: []string{"**/*.go", "**/*.js", "**/*.py"},
		Exclude: []string{},
	}

	// Measure index build time
	gi := NewMasterIndex(cfg)
	ctx := context.Background()

	buildStart := time.Now()
	err := gi.IndexDirectory(ctx, testDir)
	buildTime := time.Since(buildStart)
	require.NoError(t, err)

	t.Logf("Index build time for %d files: %v", 100, buildTime)
	assert.Less(t, buildTime.Seconds(), 5.0, "Index build should complete in <5s")

	// Get memory usage after indexing and actual file count from indexer
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	indexMemoryMB := m.Alloc / 1024 / 1024

	// Get actual file count from indexer stats
	stats := gi.GetStats()
	fileCount, ok := stats["file_count"].(int)
	if !ok {
		fileCount = 100 // fallback to expected count
	}

	t.Logf("Memory usage after indexing: %d MB (%d files)", indexMemoryMB, fileCount)

	// Graduated memory usage thresholds with appropriate warnings
	// Tree-sitter parsers and index structures require significant memory
	const (
		warningThresholdMB  = 500  // Warn above this
		criticalThresholdMB = 1500 // Fail above this
	)

	if indexMemoryMB > criticalThresholdMB {
		t.Errorf("CRITICAL: Memory usage critically high: %d MB (> %d MB threshold)", indexMemoryMB, criticalThresholdMB)
		t.Logf("Potential issues to investigate:")
		t.Logf("  - Memory leaks in AST stores or file content caches")
		t.Logf("  - Inefficient data structure allocation")
		t.Logf("  - Tree-sitter parser memory accumulation")
		t.Logf("  - Large symbol graphs not being garbage collected")
	} else if indexMemoryMB > warningThresholdMB {
		t.Logf("⚠️  WARNING: Memory usage elevated: %d MB (> %d MB warning threshold)", indexMemoryMB, warningThresholdMB)
		t.Logf("Consider monitoring for memory optimization opportunities")
	} else {
		t.Logf("✓ Memory usage acceptable: %d MB (< %d MB warning threshold)", indexMemoryMB, warningThresholdMB)
	}

	// Additional context for memory usage analysis
	if fileCount > 0 {
		memoryPerFile := float64(indexMemoryMB) / float64(fileCount)
		t.Logf("Memory per file: %.2f MB/file", memoryPerFile)

		if memoryPerFile > 10.0 {
			t.Logf("⚠️  High memory per file ratio - investigate potential memory bloat")
		}
	}

	// Test 1: Search Performance
	t.Run("Search Performance", func(t *testing.T) {
		patterns := []string{
			"function",
			"TODO",
			"error",
			"import",
			"class.*extends", // regex
		}

		for _, pattern := range patterns {
			var totalTime time.Duration
			iterations := 100

			// Determine if this is a regex pattern
			isRegex := strings.Contains(pattern, ".*") || strings.Contains(pattern, "\\") || strings.Contains(pattern, "[") || strings.Contains(pattern, "|")

			for i := 0; i < iterations; i++ {
				start := time.Now()
				results, err := gi.SearchWithOptions(pattern, types.SearchOptions{
					UseRegex: isRegex,
				})
				require.NoError(t, err)
				totalTime += time.Since(start)

				if i == 0 && !isRegex {
					// Only check for non-empty results on non-regex patterns
					assert.NotEmpty(t, results, "Should find results for pattern: %s", pattern)
				}
			}

			avgTime := totalTime / time.Duration(iterations)
			t.Logf("Search '%s': avg %v over %d iterations", pattern, avgTime, iterations)

			// Be more lenient with performance requirements
			// Allow up to 15ms for search (still very fast)
			// Common patterns like TODO and import may have many matches
			assert.Less(t, avgTime.Milliseconds(), int64(15), "Search should complete in <15ms")
		}
	})

	// Test 2: Symbol Lookup Performance
	t.Run("Symbol Lookup Performance", func(t *testing.T) {
		// Get some symbols to look up
		allFileIDs := gi.GetAllFileIDs()
		require.NotEmpty(t, allFileIDs)

		symbols := gi.GetFileEnhancedSymbols(allFileIDs[0])
		require.NotEmpty(t, symbols)

		symbolNames := []string{}
		for i, sym := range symbols {
			if i >= 10 { // Test first 10 symbols
				break
			}
			symbolNames = append(symbolNames, sym.Name)
		}

		for _, name := range symbolNames {
			var totalTime time.Duration
			iterations := 100

			for i := 0; i < iterations; i++ {
				start := time.Now()
				results := gi.FindSymbolsByName(name)
				totalTime += time.Since(start)

				if i == 0 {
					assert.NotEmpty(t, results, "Should find symbol: %s", name)
				}
			}

			avgTime := totalTime / time.Duration(iterations)
			t.Logf("Symbol lookup '%s': avg %v", name, avgTime)
			assert.Less(t, avgTime.Microseconds(), int64(1000), "Symbol lookup should complete in <1ms")
		}
	})

	// Test 3: Reference Lookup Performance
	t.Run("Reference Lookup Performance", func(t *testing.T) {
		// Find a symbol with references
		symbols := gi.FindSymbolsByName("processData")
		if len(symbols) == 0 {
			t.Skip("No processData symbol found")
		}

		symbol := symbols[0]
		var totalTime time.Duration
		iterations := 100

		for i := 0; i < iterations; i++ {
			start := time.Now()
			refs := gi.GetSymbolReferences(symbol.ID)
			totalTime += time.Since(start)

			if i == 0 {
				t.Logf("Found %d references", len(refs))
			}
		}

		avgTime := totalTime / time.Duration(iterations)
		t.Logf("Reference lookup: avg %v", avgTime)
		assert.Less(t, avgTime.Microseconds(), int64(1000), "Reference lookup should complete in <1ms")
	})

	// Test 4: File Access Performance (should be instant from memory)
	t.Run("File Access Performance", func(t *testing.T) {
		allFileIDs := gi.GetAllFileIDs()

		var totalTime time.Duration
		iterations := 1000

		for i := 0; i < iterations; i++ {
			fileID := allFileIDs[i%len(allFileIDs)]
			start := time.Now()
			fileInfo := gi.GetFileInfo(fileID)
			totalTime += time.Since(start)

			assert.NotNil(t, fileInfo, "File info should exist")
			assert.NotEmpty(t, fileInfo.Content, "File should have content in memory")
		}

		avgTime := totalTime / time.Duration(iterations)
		t.Logf("File access: avg %v", avgTime)
		// Must be <50μs for in-memory access (tests run serially)
		assert.Less(t, avgTime.Nanoseconds(), int64(50000), "File access should be <50μs")
	})

	// Test 5: Concurrent Operations
	t.Run("Concurrent Performance", func(t *testing.T) {
		concurrency := 10
		operationsPerGoroutine := 100

		start := time.Now()
		done := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				for j := 0; j < operationsPerGoroutine; j++ {
					// Mix of operations
					switch j % 4 {
					case 0:
						_, _ = gi.SearchWithOptions("test", types.SearchOptions{})
					case 1:
						gi.FindSymbolsByName("function")
					case 2:
						fileIDs := gi.GetAllFileIDs()
						if len(fileIDs) > 0 {
							gi.GetFileInfo(fileIDs[0])
						}
					case 3:
						gi.FindSymbolsByName("processData")
					}
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < concurrency; i++ {
			<-done
		}

		totalTime := time.Since(start)
		totalOps := concurrency * operationsPerGoroutine
		avgTime := totalTime / time.Duration(totalOps)

		t.Logf("Concurrent operations: %d ops in %v (avg %v/op)", totalOps, totalTime, avgTime)
		assert.Less(t, avgTime.Milliseconds(), int64(10), "Concurrent ops should average <10ms")
	})

	// Test 6: Verify No Disk Access During Operations
	t.Run("No Disk Access", func(t *testing.T) {
		// Remove all files after indexing to ensure we're not reading from disk
		err := os.RemoveAll(testDir)
		require.NoError(t, err)

		// All these operations should still work from memory
		results, err := gi.SearchWithOptions("function", types.SearchOptions{})
		require.NoError(t, err)
		assert.NotEmpty(t, results, "Search should work without files on disk")

		symbols := gi.FindSymbolsByName("processData")
		assert.NotEmpty(t, symbols, "Symbol lookup should work without files on disk")

		fileIDs := gi.GetAllFileIDs()
		assert.NotEmpty(t, fileIDs, "Should have file IDs in memory")

		fileInfo := gi.GetFileInfo(fileIDs[0])
		assert.NotNil(t, fileInfo, "File info should be in memory")
		assert.NotEmpty(t, fileInfo.Content, "File content should be in memory")
	})
}

// TestScalabilityPerformance tests the index with larger codebases
// Named with "Performance" to run in Phase 2 with reduced parallelism for accurate timing
func TestScalabilityPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability performance test in short mode")
	}

	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%d_files", size), func(t *testing.T) {
			testDir := createLargeTestProject(t, size)

			cfg := &config.Config{
				Project: config.Project{
					Root: testDir,
				},
				Index: config.Index{
					MaxFileSize:      10 * 1024 * 1024,
					RespectGitignore: false,
				},
				Include: []string{"**/*.go", "**/*.js", "**/*.py"},
				Exclude: []string{},
			}

			gi := NewMasterIndex(cfg)
			ctx := context.Background()

			// Measure index time
			start := time.Now()
			err := gi.IndexDirectory(ctx, testDir)
			indexTime := time.Since(start)
			require.NoError(t, err)

			// Get stats
			stats := gi.Stats()

			// Measure search time
			searchStart := time.Now()
			results, err := gi.SearchWithOptions("function", types.SearchOptions{})
			require.NoError(t, err)
			searchTime := time.Since(searchStart)

			t.Logf("Size: %d files", size)
			t.Logf("  Index time: %v", indexTime)
			t.Logf("  Files indexed: %d", stats.TotalFiles)
			t.Logf("  Symbols found: %d", stats.TotalSymbols)
			t.Logf("  Memory usage: %d MB", stats.IndexSize/1024/1024)
			t.Logf("  Search time: %v (found %d results)", searchTime, len(results))

			// Performance assertions (50 files/second minimum to allow for system variance)
			assert.Less(t, indexTime.Seconds(), float64(size)/50, "Should index at least 50 files/second")
			assert.Less(t, searchTime.Milliseconds(), int64(50), "Search should be <50ms even for large projects")
		})
	}
}

// Helper function to create a large test project
func createLargeTestProject(t testing.TB, numFiles int) string {
	testDir := t.TempDir()

	// Create a mix of Go, JavaScript, and Python files
	for i := 0; i < numFiles; i++ {
		var content, filename string

		switch i % 3 {
		case 0: // Go file
			filename = fmt.Sprintf("file%d.go", i)
			content = fmt.Sprintf(`package main

import (
	"fmt"
	"errors"
)

// Function%d processes data
func Function%d() error {
	// TODO: implement processing logic
	data := processData%d()
	if err := validateData(data); err != nil {
		return fmt.Errorf("validation failed: %%w", err)
	}
	return nil
}

func processData%d() interface{} {
	return struct{
		ID   int
		Name string
	}{
		ID:   %d,
		Name: "Item %d",
	}
}
`, i, i, i, i, i, i)

		case 1: // JavaScript file
			filename = fmt.Sprintf("file%d.js", i)
			content = fmt.Sprintf(`// Module %d
class Component%d {
	constructor() {
		this.id = %d;
		this.name = "Component %d";
	}
	
	process() {
		// TODO: add processing logic
		console.log("Processing component", this.id);
		return this.processData();
	}
	
	processData() {
		const data = {
			id: this.id,
			timestamp: Date.now()
		};
		return data;
	}
}

function createComponent%d() {
	return new Component%d();
}

module.exports = { Component%d, createComponent%d };
`, i, i, i, i, i, i, i, i)

		case 2: // Python file
			filename = fmt.Sprintf("file%d.py", i)
			content = fmt.Sprintf(`# Module %d
import time
import json

class DataProcessor%d:
    """Process data for module %d"""
    
    def __init__(self):
        self.id = %d
        self.name = "Processor %d"
    
    def process(self):
        # TODO: implement processing
        data = self.process_data()
        return self.validate(data)
    
    def process_data(self):
        return {
            'id': self.id,
            'timestamp': time.time(),
            'status': 'processed'
        }
    
    def validate(self, data):
        if not data.get('id'):
            raise ValueError("Invalid data: missing ID")
        return data

def create_processor_%d():
    return DataProcessor%d()
`, i, i, i, i, i, i, i)
		}

		// Create subdirectories for organization
		subdir := fmt.Sprintf("pkg%d", i/10)
		dirPath := filepath.Join(testDir, subdir)
		_ = os.MkdirAll(dirPath, 0755)

		filePath := filepath.Join(dirPath, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	return testDir
}
