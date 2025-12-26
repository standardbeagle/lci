package search_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

var benchEngine *search.Engine
var benchFileIDs []types.FileID

func setupBenchmark(b *testing.B) {
	if benchEngine != nil {
		return
	}

	tempDir := b.TempDir()
	numFiles := 50
	numSymbolsPerFile := 20

	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf("package pkg%d\n\n", i)
		for j := 0; j < numSymbolsPerFile; j++ {
			content += fmt.Sprintf(`
// Function%d does something
func Function%d() string {
	return "result %d"
}

type Type%d struct {
	Field1 string
	Field2 int
}

func (t *Type%d) Method%d() string {
	return t.Field1
}
`, j, j, j, j, j, j)
		}

		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		os.WriteFile(filename, []byte(content), 0644)
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Include: []string{"**/*.go"},
	}

	indexer := indexing.NewMasterIndex(cfg)
	indexer.IndexDirectory(context.Background(), tempDir)

	benchEngine = search.NewEngine(indexer)
	benchFileIDs = indexer.GetAllFileIDs()
}

func BenchmarkSearchType2000Results(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results := benchEngine.Search("Type", benchFileIDs, 100)
		if len(results) == 0 {
			b.Fatal("expected results")
		}
	}
}

func BenchmarkSearchFunction100Results(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results := benchEngine.Search("Function10", benchFileIDs, 100)
		if len(results) == 0 {
			b.Fatal("expected results")
		}
	}
}

func BenchmarkSearchNoResults(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = benchEngine.Search("nonexistent", benchFileIDs, 100)
	}
}
