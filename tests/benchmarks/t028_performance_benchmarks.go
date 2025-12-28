package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// BenchmarkIndexing10kFiles benchmarks indexing performance with 10,000 files
func BenchmarkIndexing10kFiles(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large benchmark in short mode")
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lci-10k-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Generate 10,000 test files across multiple packages
	b.Logf("Generating 10,000 test files...")
	start := time.Now()

	// Create different package structures
	packages := []string{"auth", "database", "api", "utils", "models", "services", "controllers", "middleware"}
	filesPerPackage := 1250 // 10,000 / 8 packages

	for _, pkg := range packages {
		pkgDir := filepath.Join(tempDir, pkg)
		err := os.MkdirAll(pkgDir, 0755)
		require.NoError(b, err)

		for i := 0; i < filesPerPackage; i++ {
			fileName := filepath.Join(pkgDir, fmt.Sprintf("%s_%d.go", pkg, i))

			// Simple content to avoid format string issues
			content := `package ` + pkg + `

import (
	"context"
	"fmt"
	"time"
)

// Service` + strconv.Itoa(i) + ` represents a business service
type Service` + strconv.Itoa(i) + ` struct {
	ID   string
	Name string
}

// NewService` + strconv.Itoa(i) + ` creates a new service instance
func NewService` + strconv.Itoa(i) + `(name string) *Service` + strconv.Itoa(i) + ` {
	return &Service` + strconv.Itoa(i) + `{
		ID:   "svc_` + strconv.Itoa(i) + `",
		Name: name,
	}
}

// Process` + strconv.Itoa(i) + ` processes the request
func (s *Service` + strconv.Itoa(i) + `) Process` + strconv.Itoa(i) + `(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data for service %s", s.Name)
	}
	return nil
}

// Validate` + strconv.Itoa(i) + ` validates the service
func (s *Service` + strconv.Itoa(i) + `) Validate` + strconv.Itoa(i) + `() error {
	if s.ID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}
	if s.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	return nil
}
`

			err := os.WriteFile(fileName, []byte(content), 0644)
			require.NoError(b, err)
		}
	}

	generationTime := time.Since(start)
	b.Logf("Generated 10,000 files in %v", generationTime)

	// Load config and create indexer
	cfg, err := config.Load(tempDir)
	require.NoError(b, err)
	indexer := indexing.NewMasterIndex(cfg)
	require.NotNil(b, indexer)

	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark indexing performance
	for i := 0; i < b.N; i++ {
		// Index the directory
		err := indexer.IndexDirectory(context.Background(), tempDir)
		require.NoError(b, err)

		// Note: indexer will be garbage collected, but we create new one each iteration
		// to measure cold start performance
		b.StopTimer()

		// Create new indexer for next iteration
		indexer = indexing.NewMasterIndex(cfg)
		require.NotNil(b, indexer)

		b.StartTimer()
	}
}

// BenchmarkLargeScaleSearch benchmarks search performance on large indexes
func BenchmarkLargeScaleSearch(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large benchmark in short mode")
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lci-large-search-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Generate a substantial codebase (1000 files)
	b.Logf("Generating 1000 files for search benchmark...")
	start := time.Now()

	for i := 0; i < 1000; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("search_file_%d.go", i))

		// Simple content to avoid format string issues
		content := `package main

import (
	"context"
	"fmt"
	"time"
)

// Type` + strconv.Itoa(i) + ` represents a complex type
type Type` + strconv.Itoa(i) + ` struct {
	ID       string
	Name     string
	Data     []byte
	Metadata map[string]interface{}
}

// Method` + strconv.Itoa(i) + ` performs an operation
func (t *Type` + strconv.Itoa(i) + `) Method` + strconv.Itoa(i) + `(ctx context.Context) error {
	if err := t.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := t.ProcessData(ctx); err != nil {
		return fmt.Errorf("operation failed: %w", err)
	}

	return t.UpdateMetadata(ctx)
}

// Validate validates the type
func (t *Type` + strconv.Itoa(i) + `) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	return nil
}

// ProcessData performs the primary operation
func (t *Type` + strconv.Itoa(i) + `) ProcessData(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Microsecond):
		t.Data = append(t.Data, 1)
		return nil
	}
}

// UpdateMetadata performs the secondary operation
func (t *Type` + strconv.Itoa(i) + `) UpdateMetadata(ctx context.Context) error {
	if t.Metadata == nil {
		t.Metadata = make(map[string]interface{})
	}
	t.Metadata["last_operation"] = "op_processed"
	return nil
}

// Function` + strconv.Itoa(i) + ` is a standalone function
func Function` + strconv.Itoa(i) + `(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	// Process the data
	for i, b := range data {
		data[i] = b ^ 1
	}

	return nil
}
`

		err := os.WriteFile(fileName, []byte(content), 0644)
		require.NoError(b, err)
	}

	generationTime := time.Since(start)
	b.Logf("Generated 1000 files in %v", generationTime)

	// Create indexer and build index
	cfg, err := config.Load(tempDir)
	require.NoError(b, err)
	indexer := indexing.NewMasterIndex(cfg)
	require.NotNil(b, indexer)

	b.Logf("Building index...")
	indexStart := time.Now()
	err = indexer.IndexDirectory(context.Background(), tempDir)
	require.NoError(b, err)
	indexTime := time.Since(indexStart)
	b.Logf("Index built in %v", indexTime)

	b.ResetTimer()
	b.ReportAllocs()

	// Test different search patterns
	searchPatterns := []string{
		"Method",      // Function methods
		"Validate",    // Validation functions
		"ProcessData", // Data processing
		"Function",    // Standalone functions
		"config",      // Common field names
		"context",     // Common parameters
		"error",       // Error handling
		"fmt.Errorf",  // Error formatting
		"time.Now",    // Time operations
	}

	for i := 0; i < b.N; i++ {
		pattern := searchPatterns[i%len(searchPatterns)]

		_, err := indexer.Search(pattern, 50)
		if err != nil && err.Error() != "no results found" {
			b.Fatalf("Search failed for pattern '%s': %v", pattern, err)
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lci-memory-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Generate memory-intensive test files
	for i := 0; i < 500; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("memory_%d.go", i))

		// Simple content to avoid format string issues
		content := `package memory

import (
	"bytes"
	"fmt"
)

// LargeStruct` + strconv.Itoa(i) + ` represents a memory-intensive structure
type LargeStruct` + strconv.Itoa(i) + ` struct {
	ID       string
	Name     string
	Data     []byte
	Metadata map[string]interface{}
	Chunks   [][]byte
	Index    map[string]int
	Buffer   *bytes.Buffer
}

// NewLargeStruct` + strconv.Itoa(i) + ` creates a new large structure
func NewLargeStruct` + strconv.Itoa(i) + `(id string, size int) *LargeStruct` + strconv.Itoa(i) + ` {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	chunks := make([][]byte, 10)
	for i := range chunks {
		chunks[i] = make([]byte, size/10)
		for j := range chunks[i] {
			chunks[i][j] = byte((i + j) % 256)
		}
	}

	metadata := make(map[string]interface{})
	for j := 0; j < 50; j++ {
		metadata[fmt.Sprintf("key_%d", j)] = fmt.Sprintf("value_%d_%d", j, ` + strconv.Itoa(i) + `)
	}

	index := make(map[string]int)
	for j := 0; j < 100; j++ {
		index[fmt.Sprintf("idx_%d", j)] = j * 7
	}

	return &LargeStruct` + strconv.Itoa(i) + `{
		ID:       id,
		Name:     fmt.Sprintf("LargeStruct_%d", ` + strconv.Itoa(i) + `),
		Data:     data,
		Metadata: metadata,
		Chunks:   chunks,
		Index:    index,
		Buffer:   bytes.NewBuffer(data),
	}
}

// Process processes the large structure
func (ls *LargeStruct` + strconv.Itoa(i) + `) Process() error {
	// Memory-intensive processing
	for i, chunk := range ls.Chunks {
		for j, b := range chunk {
			chunk[j] = b ^ byte((i + j) % 256)
		}
	}

	// Update buffer
	ls.Buffer.Reset()
	ls.Buffer.Write(ls.Data)

	// Update index
	for key, value := range ls.Index {
		ls.Index[key] = value * 2
	}

	return nil
}
`

		err := os.WriteFile(fileName, []byte(content), 0644)
		require.NoError(b, err)
	}

	// Force GC before benchmarking
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Create indexer and measure memory usage
	cfg, err := config.Load(tempDir)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Force GC and measure memory before
		runtime.GC()
		runtime.ReadMemStats(&m1)

		indexer := indexing.NewMasterIndex(cfg)
		require.NotNil(b, indexer)

		b.StartTimer()

		// Index the directory
		err := indexer.IndexDirectory(context.Background(), tempDir)
		require.NoError(b, err)

		b.StopTimer()

		// Measure memory after indexing
		runtime.ReadMemStats(&m2)

		// Report memory usage
		memUsed := m2.Alloc - m1.Alloc
		b.ReportMetric(float64(memUsed)/1024/1024, "MB/operation")

		// Report GC stats
		b.ReportMetric(float64(m2.NumGC-m1.NumGC), "GC/operation")

		b.StartTimer()
	}
}

// BenchmarkConcurrentIndexing benchmarks concurrent indexing performance
func BenchmarkConcurrentIndexing(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping concurrent benchmark in short mode")
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lci-concurrent-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Generate test files in multiple directories
	dirs := []string{"pkg1", "pkg2", "pkg3", "pkg4"}
	filesPerDir := 250

	for dirIdx, dir := range dirs {
		dirPath := filepath.Join(tempDir, dir)
		err := os.MkdirAll(dirPath, 0755)
		require.NoError(b, err)

		for i := 0; i < filesPerDir; i++ {
			fileName := filepath.Join(dirPath, fmt.Sprintf("file_%d.go", i))
			value := dirIdx*1000 + i

			// Simple content to avoid format string issues
			content := `package ` + dir + `

// Function` + strconv.Itoa(value) + ` is a test function
func Function` + strconv.Itoa(value) + `() int {
	return ` + strconv.Itoa(value) + `
}

// Struct` + strconv.Itoa(value) + ` is a test struct
type Struct` + strconv.Itoa(value) + ` struct {
	Value int
	Name  string
}

// Method` + strconv.Itoa(value) + ` is a method on Struct` + strconv.Itoa(value) + `
func (s *Struct` + strconv.Itoa(value) + `) Method` + strconv.Itoa(value) + `() int {
	return s.Value * 2
}
`

			err := os.WriteFile(fileName, []byte(content), 0644)
			require.NoError(b, err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark concurrent indexing
	for i := 0; i < b.N; i++ {
		// Load config and create indexer
		cfg, err := config.Load(tempDir)
		require.NoError(b, err)
		indexer := indexing.NewMasterIndex(cfg)
		require.NotNil(b, indexer)

		// Index the directory concurrently
		err = indexer.IndexDirectory(context.Background(), tempDir)
		require.NoError(b, err)
	}
}
