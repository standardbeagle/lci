package parser

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/core"
)

// TestParserDoesNotModifyContent verifies tree-sitter doesn't strip newlines
func TestParserDoesNotModifyContent(t *testing.T) {
	fixtureDir := "../../tests/search-comparison/fixtures/rust-sample"
	absFixtureDir, _ := filepath.Abs(fixtureDir)
	authPath := filepath.Join(absFixtureDir, "src/auth.rs")

	// Skip test if file doesn't exist
	if _, err := os.Stat(authPath); os.IsNotExist(err) {
		t.Skip("Test fixture not found")
	}

	// Read file directly
	content, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	directNewlines := bytes.Count(content, []byte("\n"))
	t.Logf("Original content: %d newlines, %d bytes", directNewlines, len(content))

	// Create FileContentStore and load content
	contentStore := core.NewFileContentStore()
	defer contentStore.Close()
	fileID := contentStore.LoadFile(authPath, content)

	// Check content before parsing
	beforeContent, ok := contentStore.GetContent(fileID)
	if !ok {
		t.Fatal("Failed to get content before parsing")
	}

	beforeNewlines := bytes.Count(beforeContent, []byte("\n"))
	t.Logf("Before parsing: %d newlines, %d bytes", beforeNewlines, len(beforeContent))

	// Create parser and parse
	parser := NewTreeSitterParser()
	parser.SetFileContentStore(contentStore)
	_, _, _, _, _, _ = parser.ParseFileEnhancedFromStore(authPath, fileID)

	// Check content after parsing
	afterContent, ok := contentStore.GetContent(fileID)
	if !ok {
		t.Fatal("Failed to get content after parsing")
	}

	afterNewlines := bytes.Count(afterContent, []byte("\n"))
	t.Logf("After parsing: %d newlines, %d bytes", afterNewlines, len(afterContent))

	// Verify parsing didn't modify content
	if beforeNewlines != afterNewlines {
		t.Errorf("Parsing modified newline count: before=%d after=%d", beforeNewlines, afterNewlines)
	}

	if len(beforeContent) != len(afterContent) {
		t.Errorf("Parsing modified content length: before=%d after=%d", len(beforeContent), len(afterContent))
	}
}
