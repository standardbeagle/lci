package search

import (
	"runtime"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// BenchmarkTraditionalStringRef tests the current allocation-heavy StringRef approach
func BenchmarkTraditionalStringRef(b *testing.B) {
	// Setup test data
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(1000) // 1KB test content
	fileID := fileStore.LoadFile("test.go", testContent)

	// Get line references
	lineRefs := make([]types.StringRef, 0, 100)
	for i := 0; i < 100; i++ {
		if ref, ok := fileStore.GetLine(fileID, i); ok {
			lineRefs = append(lineRefs, ref)
		}
	}

	patterns := []string{"func", "var", "const", "import", "package", "struct", "interface"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		for _, lineRef := range lineRefs {
			// This allocation happens for every line!
			if lineStr, err := fileStore.GetString(lineRef); err == nil {
				// These allocations happen for every pattern match!
				if strings.Contains(lineStr, pattern) {
					// Process match (simulated work)
					_ = len(lineStr)
				}
			}
		}
	}
}

// BenchmarkZeroAllocStringRef tests the new zero-allocation StringRef approach
func BenchmarkZeroAllocStringRef(b *testing.B) {
	// Setup test data
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(1000) // 1KB test content
	fileID := fileStore.LoadFile("test.go", testContent)

	// Get zero-allocation line references directly from FileContentStore
	lineRefs := make([]types.ZeroAllocStringRef, 0, 100)
	for i := 0; i < 100; i++ {
		if zeroRef := fileStore.GetZeroAllocLine(fileID, i); !zeroRef.IsEmpty() {
			lineRefs = append(lineRefs, zeroRef)
		}
	}

	patterns := []string{"func", "var", "const", "import", "package", "struct", "interface"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		for _, lineRef := range lineRefs {
			// ZERO ALLOCATION! Direct byte operations
			if lineRef.Contains(pattern) {
				// Process match (simulated work)
				_ = lineRef.Len()
			}
		}
	}
}

// BenchmarkContextSearchTraditional tests traditional context search with allocations
func BenchmarkContextSearchTraditional(b *testing.B) {
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(5000) // 5KB test content
	fileID := fileStore.LoadFile("test.go", testContent)

	pattern := "function"
	contextLines := 3

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Find matches (allocations)
		var matches []types.StringRef
		lineCount := fileStore.GetLineCount(fileID)
		for j := 0; j < lineCount; j++ {
			if lineRef, ok := fileStore.GetLine(fileID, j); ok {
				if lineStr, err := fileStore.GetString(lineRef); err == nil {
					if strings.Contains(lineStr, pattern) {
						matches = append(matches, lineRef)
					}
				}
			}
		}

		// Get context for each match (more allocations)
		for _, match := range matches {
			// Get line number from StringRef (this is complex in traditional approach)
			lineNum := findLineNumberForStringRef(fileStore, fileID, match)
			if lineNum >= 0 {
				start := maxForBenchmark(0, lineNum-contextLines)
				end := lineNum + contextLines + 1
				for k := start; k < end; k++ {
					if ctxRef, ok := fileStore.GetLine(fileID, k); ok {
						if ctxStr, err := fileStore.GetString(ctxRef); err == nil {
							_ = len(ctxStr) // Simulate processing
						}
					}
				}
			}
		}
	}
}

// BenchmarkContextSearchZeroAlloc tests zero-allocation context search
func BenchmarkContextSearchZeroAlloc(b *testing.B) {
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(5000) // 5KB test content
	fileID := fileStore.LoadFile("test.go", testContent)

	pattern := "function"
	contextLines := 3

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// ZERO ALLOCATION! Direct context search
		result := fileStore.FindWithContext(fileID, pattern, contextLines)

		// Process results without allocation
		for _, lineRef := range result.Lines {
			_ = lineRef.Len()
		}
		for _, ctxLines := range result.Context {
			for _, ctxLine := range ctxLines {
				_ = ctxLine.Len()
			}
		}
	}
}

// BenchmarkStringOperations compares traditional vs zero-alloc string operations
func BenchmarkStringOperations(b *testing.B) {
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(1000)
	fileID := fileStore.LoadFile("test.go", testContent)
	lineRef, _ := fileStore.GetLine(fileID, 50) // Get a line to work with
	zeroRef := fileStore.GetZeroAllocLine(fileID, 50)

	tests := []struct {
		name        string
		traditional func(string) bool
		zeroAlloc   func(types.ZeroAllocStringRef) bool
	}{
		{
			"Contains",
			func(s string) bool { return strings.Contains(s, "func") },
			func(ref types.ZeroAllocStringRef) bool { return ref.Contains("func") },
		},
		{
			"HasPrefix",
			func(s string) bool { return strings.HasPrefix(s, "func") },
			func(ref types.ZeroAllocStringRef) bool { return ref.HasPrefix("func") },
		},
		{
			"HasSuffix",
			func(s string) bool { return strings.HasSuffix(s, "()") },
			func(ref types.ZeroAllocStringRef) bool { return ref.HasSuffix("()") },
		},
		{
			"Equal",
			func(s string) bool { return s == "func test() {" },
			func(ref types.ZeroAllocStringRef) bool { return ref.EqualString("func test() {") },
		},
	}

	for _, test := range tests {
		b.Run(test.name+"_Traditional", func(b *testing.B) {
			lineStr, _ := fileStore.GetString(lineRef)
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = test.traditional(lineStr)
			}
		})

		b.Run(test.name+"_ZeroAlloc", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = test.zeroAlloc(zeroRef)
			}
		})
	}
}

// BenchmarkMultiPatternSearch tests multi-pattern search performance
func BenchmarkMultiPatternSearch(b *testing.B) {
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(2000) // 2KB test content
	fileID := fileStore.LoadFile("test.go", testContent)

	patterns := []string{"func", "var", "const", "import", "package", "struct", "interface", "type"}

	b.Run("Traditional", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results := make(map[string]int)
			for _, pattern := range patterns {
				count := 0
				lineCount := fileStore.GetLineCount(fileID)
				for j := 0; j < lineCount; j++ {
					if lineRef, ok := fileStore.GetLine(fileID, j); ok {
						if lineStr, err := fileStore.GetString(lineRef); err == nil {
							if strings.Contains(lineStr, pattern) {
								count++
							}
						}
					}
				}
				results[pattern] = count
			}
			_ = results
		}
	})

	b.Run("ZeroAlloc", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results := make(map[string]int)
			for _, pattern := range patterns {
				lineNumbers := fileStore.FindAllLinesWithPattern(fileID, pattern)
				results[pattern] = len(lineNumbers)
			}
			_ = results
		}
	})
}

// BenchmarkMemoryUsage compares memory usage between approaches
func BenchmarkMemoryUsage(b *testing.B) {
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(1000)
	fileID := fileStore.LoadFile("test.go", testContent)

	b.Run("Traditional_Memory", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lineCount := fileStore.GetLineCount(fileID)
			for j := 0; j < lineCount; j++ {
				if lineRef, ok := fileStore.GetLine(fileID, j); ok {
					if lineStr, err := fileStore.GetString(lineRef); err == nil {
						if strings.Contains(lineStr, "func") {
							_ = lineStr // Use the string to prevent optimization
						}
					}
				}
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc), "bytes_allocated")
	})

	b.Run("ZeroAlloc_Memory", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lineNumbers := fileStore.FindAllLinesWithPattern(fileID, "func")
			_ = lineNumbers // Use the result to prevent optimization
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc), "bytes_allocated")
	})
}

// BenchmarkConcurrentSearch tests concurrent search performance
func BenchmarkConcurrentSearch(b *testing.B) {
	fileStore := core.NewFileContentStore()
	defer fileStore.Close()
	testContent := createTestContent(2000)
	fileID := fileStore.LoadFile("test.go", testContent)

	pattern := "function"

	b.Run("Traditional_Concurrent", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				lineCount := fileStore.GetLineCount(fileID)
				for j := 0; j < lineCount; j++ {
					if lineRef, ok := fileStore.GetLine(fileID, j); ok {
						if lineStr, err := fileStore.GetString(lineRef); err == nil {
							if strings.Contains(lineStr, pattern) {
								_ = len(lineStr)
							}
						}
					}
				}
			}
		})
	})

	b.Run("ZeroAlloc_Concurrent", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				lineNumbers := fileStore.FindAllLinesWithPattern(fileID, pattern)
				_ = len(lineNumbers)
			}
		})
	})
}

// Helper functions

func createTestContent(size int) []byte {
	content := make([]byte, size)
	lines := []string{
		"package main",
		"import \"fmt\"",
		"",
		"// This is a test function",
		"func testFunction(param1 string, param2 int) error {",
		"    if param1 == \"\" {",
		"        return fmt.Errorf(\"empty string\")",
		"    }",
		"    result := param2 * 2",
		"    fmt.Println(\"Processing:\", param1, result)",
		"    return nil",
		"}",
		"",
		"var globalVariable string = \"test\"",
		"const someConstant int = 42",
		"",
		"type TestStruct struct {",
		"    Field1 string",
		"    Field2 int",
		"}",
		"",
		"func (ts *TestStruct) Method() {",
		"    fmt.Println(\"Method called\")",
		"}",
	}

	// Repeat lines to reach desired size
	lineText := strings.Join(lines, "\n")
	for i := 0; i < size; i += len(lineText) {
		end := i + len(lineText)
		if end > size {
			end = size
		}
		copy(content[i:], lineText[:end-i])
	}

	return content
}

func findLineNumberForStringRef(fileStore *core.FileContentStore, fileID types.FileID, ref types.StringRef) int {
	// This is inefficient - we have to search through all lines
	lineCount := fileStore.GetLineCount(fileID)
	for i := 0; i < lineCount; i++ {
		if lineRef, ok := fileStore.GetLine(fileID, i); ok {
			if lineRef.FileID == ref.FileID && lineRef.Offset == ref.Offset {
				return i
			}
		}
	}
	return -1
}

func maxForBenchmark(a, b int) int {
	if a > b {
		return a
	}
	return b
}
