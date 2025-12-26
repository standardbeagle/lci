package core

import (
	"testing"

	"github.com/cespare/xxhash/v2"
)

// BenchmarkTwoTierHashing demonstrates the performance benefit of two-tier hashing
func BenchmarkTwoTierHashing(b *testing.B) {
	// Create sample file content (10KB)
	content := make([]byte, 10*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}

	b.Run("xxhash", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = xxhash.Sum64(content)
		}
	})

	b.Run("LoadFile_FirstTime", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			fcs := NewFileContentStore()
			_ = fcs.LoadFile("test.go", content)
			fcs.Close()
		}
	})

	b.Run("LoadFile_Unchanged", func(b *testing.B) {
		fcs := NewFileContentStore()
		defer fcs.Close()
		_ = fcs.LoadFile("test.go", content)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// This should hit the fast path (xxhash match, skip SHA256)
			_ = fcs.LoadFile("test.go", content)
		}
	})

	b.Run("LoadFile_Changed", func(b *testing.B) {
		fcs := NewFileContentStore()
		defer fcs.Close()
		_ = fcs.LoadFile("test.go", content)
		// Modify content slightly
		modifiedContent := make([]byte, len(content))
		copy(modifiedContent, content)
		modifiedContent[0] = 0xFF
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = fcs.LoadFile("test.go", modifiedContent)
		}
	})
}

// BenchmarkBatchLoadFiles measures batch loading performance with two-tier hashing
func BenchmarkBatchLoadFiles(b *testing.B) {
	// Create 100 files with 10KB content each
	files := make([]struct {
		Path    string
		Content []byte
	}, 100)
	for i := range files {
		content := make([]byte, 10*1024)
		for j := range content {
			content[j] = byte((i + j) % 256)
		}
		files[i].Path = "test.go"
		files[i].Content = content
	}

	b.Run("BatchLoad_FirstTime", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			fcs := NewFileContentStore()
			_ = fcs.BatchLoadFiles(files)
			fcs.Close()
		}
	})

	b.Run("BatchLoad_Unchanged", func(b *testing.B) {
		fcs := NewFileContentStore()
		defer fcs.Close()
		_ = fcs.BatchLoadFiles(files)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Should hit fast path for all files
			_ = fcs.BatchLoadFiles(files)
		}
	})
}
