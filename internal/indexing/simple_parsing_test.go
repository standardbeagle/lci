package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/parser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleGoParsing tests if basic Go parsing is working
func TestSimpleGoParsing(t *testing.T) {
	// Create a simple Go file
	goCode := `package main

func main() {
	println("Hello")
	helper()
}

func helper() {
	println("Helper")
}`

	// Test parser directly first
	t.Run("DirectParser", func(t *testing.T) {
		contentStore := core.NewFileContentStore()
		p := parser.GetParserForLanguage("go", contentStore)
		defer parser.ReleaseParserToPool(p, parser.LanguageGo)

		// Parse the code
		ast, _, symbols, _, _, references, scopes := p.ParseFileEnhancedWithAST("test.go", []byte(goCode))

		// Check results
		assert.NotNil(t, ast, "AST should not be nil")
		t.Logf("Symbols found: %d", len(symbols))
		for _, sym := range symbols {
			t.Logf("  Symbol: %s (type=%d, line=%d)", sym.Name, sym.Type, sym.Line)
		}
		assert.GreaterOrEqual(t, len(symbols), 2, "Should have at least 2 function symbols (main and helper)")

		t.Logf("References found: %d", len(references))
		for _, ref := range references {
			t.Logf("  Reference: %s (type=%d, line=%d)", ref.ReferencedName, ref.Type, ref.Line)
		}

		t.Logf("Scopes found: %d", len(scopes))
	})

	// Test through MasterIndex
	t.Run("ThroughMasterIndex", func(t *testing.T) {
		// Create temp directory
		tempDir := t.TempDir()

		// Write the Go file
		goFile := filepath.Join(tempDir, "test.go")
		require.NoError(t, os.WriteFile(goFile, []byte(goCode), 0644))

		// Create config - explicitly set to include Go files
		cfg := &config.Config{
			Project: config.Project{
				Root: tempDir,
			},
			Performance: config.Performance{
				MaxMemoryMB: 100,
			},
			Index: config.Index{
				RespectGitignore: false,            // Don't use gitignore
				MaxFileSize:      10 * 1024 * 1024, // 10MB
			},
			Include: []string{"*.go"}, // Explicitly include Go files
			Exclude: []string{},       // No exclusions
		}

		// Create MasterIndex
		mi := NewMasterIndex(cfg)
		defer mi.Close()

		// Run indexing
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := mi.IndexDirectory(ctx, tempDir)
		require.NoError(t, err, "Indexing should complete without error")

		// Check symbols in SymbolIndex
		allDefs := mi.symbolIndex.GetAllDefinitions()
		symbolCount := 0
		t.Log("Symbols in SymbolIndex:")
		for name, symbols := range allDefs {
			symbolCount += len(symbols)
			for _, sym := range symbols {
				t.Logf("  %s (type=%d, line=%d)", name, sym.Type, sym.Line)
			}
		}
		assert.Greater(t, symbolCount, 0, "Should have extracted symbols")

		// Check statistics
		stats := mi.GetStats()
		t.Logf("Statistics:")
		t.Logf("  Files: %v", stats["file_count"])
		t.Logf("  Symbols: %v", stats["symbol_count"])
	})
}

// TestLanguageDetection tests if language detection is working for Go files
func TestLanguageDetection(t *testing.T) {
	tempDir := t.TempDir()

	// Create a Go file
	goFile := filepath.Join(tempDir, "test.go")
	goCode := `package main
func main() {}`
	require.NoError(t, os.WriteFile(goFile, []byte(goCode), 0644))

	// Create config
	cfg := &config.Config{
		Project: config.Project{
			Root: tempDir,
		},
		Index: config.Index{
			RespectGitignore: false,
			MaxFileSize:      10 * 1024 * 1024,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
	}

	// Create a file scanner to test language detection
	scanner := NewFileScanner(cfg, 1)

	// Scan the directory
	taskChan := make(chan FileTask, 10)
	progressTracker := NewProgressTracker()
	go func() {
		scanner.ScanDirectory(context.Background(), tempDir, taskChan, progressTracker)
		close(taskChan)
	}()

	// Check the task
	var foundGo bool
	for task := range taskChan {
		t.Logf("File: %s, Language: %s", task.Path, task.Language)
		if filepath.Base(task.Path) == "test.go" {
			foundGo = true
			assert.Equal(t, "go", task.Language, "Go file should be detected as 'go' language")
		}
	}

	assert.True(t, foundGo, "Should have found the Go file")
}
