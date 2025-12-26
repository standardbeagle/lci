package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryMultiplicationBug investigates the real cause of 15GB usage
func TestMemoryMultiplicationBug(t *testing.T) {
	// Test multiple components individually to find the real culprit

	t.Run("FileContentStoreMemoryAccounting", func(t *testing.T) {
		store := core.NewFileContentStore()
		defer store.Close()

		// Create test content
		testContent := make([]byte, 1024*1024) // 1MB
		for i := range testContent {
			testContent[i] = byte('A' + (i % 26))
		}

		// Load the same content multiple times (simulating multiple files)
		fileIDs := make([]types.FileID, 0, 10)
		for i := 0; i < 10; i++ {
			path := fmt.Sprintf("test%d.js", i)
			fileID := store.LoadFile(path, testContent)
			fileIDs = append(fileIDs, fileID)
		}

		// Get memory usage
		reportedMemory := store.GetMemoryUsage()
		expectedMemory := int64(10 * 1024 * 1024) // 10MB for 10 files

		t.Logf("FileContentStore: loaded %d files, reported=%d MB, expected=%d MB",
			len(fileIDs), reportedMemory/(1024*1024), expectedMemory/(1024*1024))

		// Memory should be reasonable (not 10x or 100x expected)
		assert.Less(t, reportedMemory, expectedMemory*2,
			"FileContentStore should not double-count memory")
	})

	t.Run("ASTStoreMemoryUsage", func(t *testing.T) {
		contentStore := core.NewFileContentStore()
		defer contentStore.Close()
		astStore := core.NewASTStore(contentStore)

		// Create test files with content that will generate ASTs
		testFiles := map[string]string{
			"test1.js": `
				function hello() {
					return "world";
				}
				class MyClass {
					constructor() { this.value = 42; }
					method() { return this.value * 2; }
				}
			`,
			"test2.go": `
				package main
				import "fmt"
				func main() {
					fmt.Println("Hello, World!")
				}
				type MyStruct struct { Field int }
				func (m MyStruct) Method() int { return m.Field * 2 }
			`,
		}

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Add ASTs for each file
		for path, content := range testFiles {
			fileID := contentStore.LoadFile(path, []byte(content))

			// This would normally parse AST - let's simulate the memory cost
			// In real usage, tree-sitter ASTs can be quite large
			astStore.StoreAST(fileID, nil, path, filepath.Ext(path))
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		memDelta := int64(m2.Alloc) - int64(m1.Alloc)
		t.Logf("ASTStore memory delta: %d KB for %d files",
			memDelta/1024, len(testFiles))

		// Get ASTStore stats
		astStats := astStore.GetMemoryStats()
		t.Logf("ASTStore stats: %+v", astStats)
	})

	t.Run("IndexingMemoryProgression", func(t *testing.T) {
		// Test memory growth during indexing process
		cfg := &config.Config{
			Project: config.Project{Root: "."},
			Index: config.Index{
				MaxFileSize:    1 * 1024 * 1024, // 1MB
				MaxTotalSizeMB: 50,              // 50MB limit
				MaxFileCount:   1000,
			},
			Include: []string{"*.go", "*.md"},
			Exclude: []string{".git/*", "*_test.go"}, // Exclude tests to reduce load
		}

		var memSnapshots []uint64
		takeSnapshot := func(label string) {
			var m runtime.MemStats
			runtime.GC()
			time.Sleep(10 * time.Millisecond) // Let GC settle
			runtime.ReadMemStats(&m)
			memSnapshots = append(memSnapshots, m.Alloc)
			t.Logf("%s: %d KB", label, m.Alloc/1024)
		}

		takeSnapshot("Initial")

		// Create components step by step to isolate memory growth
		gi := indexing.NewMasterIndex(cfg)
		takeSnapshot("After MasterIndex creation")

		_, err := NewServer(gi, cfg)
		require.NoError(t, err)
		takeSnapshot("After MCP server creation")

		// Auto-indexing starts automatically in NewServer
		// Wait for it to complete
		time.Sleep(100 * time.Millisecond)
		takeSnapshot("After auto-indexing started")

		// Check for memory growth patterns
		for i := 1; i < len(memSnapshots); i++ {
			growth := int64(memSnapshots[i]) - int64(memSnapshots[i-1])
			t.Logf("Step %d growth: %d KB", i, growth/1024)

			// No single step should consume >100MB
			assert.Less(t, growth/1024, int64(100*1024),
				"Single step consumed >100MB: step %d", i)
		}
	})
}

// TestRealWorldMemoryUsage tests with actual complexity
func TestRealWorldMemoryUsage(t *testing.T) {
	t.Run("HandleLargeJavaScriptFile", func(t *testing.T) {
		// Generate a large but realistic JavaScript file
		content := generateLargeJSFile(1000) // 1000 functions

		// Use FileContentStore's internal memory tracking for accurate measurement
		// This isolates memory measurement to just the store, not background goroutines
		store := core.NewFileContentStore()
		defer store.Close()

		// Load the file
		fileID := store.LoadFile("large.js", content)

		// Use the test helper for two-tier memory validation
		// (global MemStats for quick check + store-specific tracking for accuracy)
		result := testhelpers.ValidateFileContentStoreMemory(t, store, content, 10.0)

		// Additional verification using the result
		assert.Less(t, result.StoreMultiplier, 10.0,
			"Store memory multiplier should be < 10x")
		assert.Greater(t, result.StoreMultiplier, 0.5,
			"Store memory multiplier should be reasonable (> 0.5x)")

		if result.BackgroundRunning {
			t.Logf("Background operations detected during test - global heap: %.2fx, store: %.2fx",
				result.GlobalMultiplier, result.StoreMultiplier)
		}

		_ = fileID // Use fileID to avoid unused variable
	})
}

func generateLargeJSFile(numFunctions int) []byte {
	var content strings.Builder

	content.WriteString("// Large JavaScript file for memory testing\n")
	content.WriteString("const config = { debug: true, version: '1.0.0' };\n\n")

	for i := 0; i < numFunctions; i++ {
		content.WriteString(fmt.Sprintf(`
function processData%d(input) {
	const result = input.map(item => ({
		id: item.id + %d,
		name: 'processed_' + item.name,
		timestamp: Date.now(),
		metadata: {
			processed: true,
			version: config.version,
			debug: config.debug
		}
	}));
	return result.filter(item => item.id > 0);
}
`, i, i))
	}

	content.WriteString("\n// Export all functions\nmodule.exports = {\n")
	for i := 0; i < numFunctions; i++ {
		content.WriteString(fmt.Sprintf("  processData%d,\n", i))
	}
	content.WriteString("};\n")

	return []byte(content.String())
}

// TestShadcnMemoryProfile profiles memory usage for shadcn-ui indexing
// This test is slow (indexes 88MB project) and should only run when explicitly requested.
// Run with: go test -run TestShadcnMemoryProfile -memprofile
func TestShadcnMemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling in short mode")
	}
	// Skip by default - only run when memory profiling is requested
	if os.Getenv("MEMPROFILE") == "" && os.Getenv("CI") == "" {
		t.Skip("Skipping slow shadcn-ui memory profiling (set MEMPROFILE=1 to run)")
	}

	projectRoot := "/home/beagle/work/lightning-docs/lightning-code-index/real_projects/typescript/shadcn-ui"

	// Check if directory exists
	if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
		t.Skip("shadcn-ui fixture not found")
	}

	// Load default config with proper exclusions
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Override specific settings for test
	cfg.Project.Root = projectRoot
	cfg.Index.MaxFileSize = 10 * 1024 * 1024
	cfg.Index.MaxTotalSizeMB = 2000 // Increase limit to avoid hitting it

	idx := indexing.NewMasterIndex(cfg)
	defer idx.Close()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	t.Logf("Before indexing: Alloc=%dMB Sys=%dMB HeapAlloc=%dMB",
		m1.Alloc/1024/1024, m1.Sys/1024/1024, m1.HeapAlloc/1024/1024)

	ctx := context.Background()
	err = idx.IndexDirectory(ctx, projectRoot)
	if err != nil {
		t.Logf("Index error (may be expected): %v", err)
	}

	runtime.ReadMemStats(&m2)
	t.Logf("After indexing: Alloc=%dMB Sys=%dMB HeapAlloc=%dMB",
		m2.Alloc/1024/1024, m2.Sys/1024/1024, m2.HeapAlloc/1024/1024)
	t.Logf("Memory increase: Alloc=%dMB Sys=%dMB HeapAlloc=%dMB",
		(m2.Alloc-m1.Alloc)/1024/1024, (m2.Sys-m1.Sys)/1024/1024,
		(m2.HeapAlloc-m1.HeapAlloc)/1024/1024)

	stats := idx.GetStats()
	fileCount, _ := stats["file_count"].(int)
	symbolCount, _ := stats["symbol_count"].(int)
	t.Logf("Indexed: %d files, %d symbols", fileCount, symbolCount)

	// Write heap profile
	f, err := os.Create("shadcn_heap.prof")
	if err != nil {
		t.Fatalf("Failed to create heap profile: %v", err)
	}
	defer f.Close()

	runtime.GC() // Force GC to get accurate heap snapshot
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("Failed to write heap profile: %v", err)
	}
	t.Logf("Heap profile written to shadcn_heap.prof")
}
