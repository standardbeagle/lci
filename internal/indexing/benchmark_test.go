package indexing

import (
	"context"
	"fmt"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/testhelpers"
)

// Benchmark tests to ensure all operations meet performance requirements

// BenchmarkSearch tests search performance with various patterns
func BenchmarkSearch(b *testing.B) {
	gi := setupBenchmarkIndex(b, 100)

	patterns := []struct {
		name    string
		pattern string
		options types.SearchOptions
	}{
		{"simple", "function", types.SearchOptions{}},
		{"case_insensitive", "ERROR", types.SearchOptions{CaseInsensitive: true}},
		{"regex", "func.*process", types.SearchOptions{UseRegex: true}},
		{"with_context", "TODO", types.SearchOptions{MaxContextLines: 5}},
	}

	for _, p := range patterns {
		b.Run(p.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := gi.SearchWithOptions(p.pattern, p.options)
				if err != nil {
					b.Fatal("Search error:", err)
				}
				if len(results) == 0 {
					b.Fatal("No results found")
				}
			}
		})
	}
}

// BenchmarkIndexingSymbolLookup tests symbol lookup performance focused on indexing structures
// Renamed from BenchmarkSymbolLookup to avoid duplicate name with core integration benchmark
func BenchmarkIndexingSymbolLookup(b *testing.B) {
	gi := setupBenchmarkIndex(b, 100)

	// Pre-find some symbols
	symbolNames := []string{"Function1", "processData", "Component5", "DataProcessor10"}

	for _, name := range symbolNames {
		b.Run(name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				symbols := gi.FindSymbolsByName(name)
				if len(symbols) == 0 {
					b.Fatal("Symbol not found")
				}
			}
		})
	}
}

// BenchmarkSymbolDefinitionLookup tests symbol definition lookup performance
// Uses FindSymbolsByName which is the preferred API over deprecated FindDefinition
func BenchmarkSymbolDefinitionLookup(b *testing.B) {
	gi := setupBenchmarkIndex(b, 100)

	symbols := []string{"processData", "Component10", "validate"}

	for _, sym := range symbols {
		b.Run(sym, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results := gi.FindSymbolsByName(sym)
				if len(results) == 0 {
					b.Fatal("Symbol not found")
				}
			}
		})
	}
}

// BenchmarkSymbolReferenceLookup tests symbol reference lookup performance
// Uses GetSymbolReferences which is the preferred API over deprecated FindReferences
func BenchmarkSymbolReferenceLookup(b *testing.B) {
	gi := setupBenchmarkIndex(b, 100)

	symbols := []string{"processData", "validate", "Function1"}

	for _, sym := range symbols {
		b.Run(sym, func(b *testing.B) {
			// Pre-find the symbol to get its ID
			syms := gi.FindSymbolsByName(sym)
			if len(syms) == 0 {
				b.Skip("Symbol not found")
			}
			symbolID := syms[0].ID

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				refs := gi.GetSymbolReferences(symbolID)
				_ = refs // References might be empty
			}
		})
	}
}

// BenchmarkFileAccess tests file information retrieval performance
func BenchmarkFileAccess(b *testing.B) {
	gi := setupBenchmarkIndex(b, 100)

	fileIDs := gi.GetAllFileIDs()
	if len(fileIDs) == 0 {
		b.Fatal("No files indexed")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fileID := fileIDs[i%len(fileIDs)]
		fileInfo := gi.GetFileInfo(fileID)
		if fileInfo == nil {
			b.Fatal("File info not found")
		}
	}
}

// BenchmarkConcurrentOperations tests performance under concurrent load
func BenchmarkConcurrentOperations(b *testing.B) {
	gi := setupBenchmarkIndex(b, 100)

	b.RunParallel(func(pb *testing.PB) {
		ops := 0
		for pb.Next() {
			switch ops % 4 {
			case 0:
				_, _ = gi.SearchWithOptions("test", types.SearchOptions{})
			case 1:
				gi.FindSymbolsByName("Function1")
			case 2:
				gi.FindSymbolsByName("processData")
			case 3:
				fileIDs := gi.GetAllFileIDs()
				if len(fileIDs) > 0 {
					gi.GetFileInfo(fileIDs[0])
				}
			}
			ops++
		}
	})
}

// BenchmarkIndexBuildTime tests indexing performance for various project sizes
func BenchmarkIndexBuildTime(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d_files", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				testDir := createLargeTestProject(b, size)
				cfg := createTestConfig(testDir)
				gi := NewMasterIndex(cfg)
				ctx := context.Background()
				b.StartTimer()

				err := gi.IndexDirectory(ctx, testDir)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMemoryEfficiency tests memory usage for large projects
func BenchmarkMemoryEfficiency(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d_files", size), func(b *testing.B) {
			testDir := createLargeTestProject(b, size)
			cfg := createTestConfig(testDir)
			gi := NewMasterIndex(cfg)
			ctx := context.Background()

			err := gi.IndexDirectory(ctx, testDir)
			if err != nil {
				b.Fatal(err)
			}

			stats := gi.Stats()
			b.ReportMetric(float64(stats.TotalFiles), "files")
			b.ReportMetric(float64(stats.TotalSymbols), "symbols")
			b.ReportMetric(float64(stats.IndexSize/1024/1024), "MB")
			b.ReportMetric(float64(stats.IndexSize/int64(stats.TotalFiles)/1024), "KB/file")
		})
	}
}

// Helper function to set up a benchmark index
func setupBenchmarkIndex(b testing.TB, numFiles int) *MasterIndex {
	testDir := createLargeTestProject(b, numFiles)
	cfg := createTestConfig(testDir)
	gi := NewMasterIndex(cfg)
	ctx := context.Background()

	err := gi.IndexDirectory(ctx, testDir)
	if err != nil {
		b.Fatal(err)
	}

	return gi
}

// Helper function to create test config
func createTestConfig(rootDir string) *config.Config {
	return testhelpers.NewTestConfigBuilder(rootDir).
		WithIncludePatterns("*.go", "*.js", "*.py").
		Build()
}

// Benchmark results should show:
// - Search operations: <5ms
// - Symbol lookup: <1ms
// - File access: <10μs (memory access)
// - Concurrent operations scale linearly
// - Memory usage: ~1MB per 10 files
