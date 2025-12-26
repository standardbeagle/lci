package lightning_code_index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

func TestShadcnMemoryBlowup(t *testing.T) {
	projectRoot := "/tmp/shadcn-test"

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	fmt.Printf("Before indexing:\n")
	fmt.Printf("  Alloc:    %8d MB\n", m1.Alloc/1024/1024)
	fmt.Printf("  Sys:      %8d MB\n", m1.Sys/1024/1024)
	fmt.Printf("  HeapAlloc: %8d MB\n", m1.HeapAlloc/1024/1024)

	cfg := &config.Config{
		Project: config.Project{Root: projectRoot},
		Index: config.Index{
			MaxFileSize:    10 * 1024 * 1024,
			MaxTotalSizeMB: 2000,
		},
		Include: []string{"*.tsx", "*.ts", "*.js", "*.jsx"},
		Exclude: []string{"node_modules/*", ".git/*"},
	}

	idx := indexing.NewMasterIndex(cfg)
	defer idx.Close()

	start := time.Now()
	ctx := context.Background()
	err := idx.IndexDirectory(ctx, projectRoot)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Index error: %v\n", err)
	}

	runtime.ReadMemStats(&m2)

	fmt.Printf("\nAfter indexing (%v):\n", elapsed)
	fmt.Printf("  Alloc:    %8d MB\n", m2.Alloc/1024/1024)
	fmt.Printf("  Sys:      %8d MB\n", m2.Sys/1024/1024)
	fmt.Printf("  HeapAlloc: %8d MB\n", m2.HeapAlloc/1024/1024)
	fmt.Printf("  Increase: %8d MB\n", (m2.Alloc-m1.Alloc)/1024/1024)

	stats := idx.GetStats()
	if fileCount, ok := stats["file_count"].(int); ok {
		fmt.Printf("\nIndexed %d files\n", fileCount)
	}

	// Count actual files in directory
	fileCount := 0
	var totalSize int64
	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && (filepath.Ext(path) == ".tsx" || filepath.Ext(path) == ".ts") {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})
	fmt.Printf("Actual: %d files, %d MB total size\n", fileCount, totalSize/1024/1024)
	fmt.Printf("Memory per file: %.2f KB\n", float64((m2.Alloc-m1.Alloc))/float64(fileCount)/1024)

	// Create heap profile
	heapFile, err := os.Create("/tmp/shadcn_heap.prof")
	if err != nil {
		t.Fatalf("Failed to create heap profile: %v", err)
	}
	defer heapFile.Close()

	runtime.GC()
	if err := pprof.WriteHeapProfile(heapFile); err != nil {
		t.Fatalf("Failed to write heap profile: %v", err)
	}

	fmt.Printf("\nHeap profile written to /tmp/shadcn_heap.prof\n")
	fmt.Printf("Analyze with: go tool pprof /tmp/shadcn_heap.prof\n")
}
