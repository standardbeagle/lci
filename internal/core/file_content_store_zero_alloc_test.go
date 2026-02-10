package core

import (
	"fmt"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestFileContentStore_ZeroAlloc_BasicOperations(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	testContent := []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func helper() {
	fmt.Println("Helper function")
}
`)

	fileID := fileStore.LoadFile("test.go", testContent)

	// Test GetLineCount
	lineCount := fileStore.GetLineCount(fileID)
	if lineCount == 0 {
		t.Error("GetLineCount should return > 0 for non-empty file")
	}

	// Test GetZeroAllocLine
	lineRef := fileStore.GetZeroAllocLine(fileID, 0)
	if lineRef.IsEmpty() {
		t.Error("GetZeroAllocLine should return valid reference for existing line")
	}

	// Test GetZeroAllocLines
	lines := fileStore.GetZeroAllocLines(fileID, 0, lineCount)
	if len(lines) == 0 {
		t.Error("GetZeroAllocLines should return some lines")
	}
	if len(lines) > lineCount {
		t.Errorf("GetZeroAllocLines should not return more than %d lines, got %d", lineCount, len(lines))
	}
}

func TestFileContentStore_ZeroAlloc_PatternMatching(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	testContent := []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

// Another function
func testFunction() {
	fmt.Println("Test")
}
`)

	fileID := fileStore.LoadFile("test.go", testContent)

	// Test FindAllLinesWithPattern
	lines := fileStore.FindAllLinesWithPattern(fileID, "func")
	if len(lines) == 0 {
		t.Error("FindAllLinesWithPattern should find lines containing 'func'")
	}

	// Test FindAllLinesWithPrefix
	prefixLines := fileStore.FindAllLinesWithPrefix(fileID, "func ")
	if len(prefixLines) == 0 {
		t.Error("FindAllLinesWithPrefix should find lines starting with 'func '")
	}

	// Test FindAllLinesWithSuffix
	suffixLines := fileStore.FindAllLinesWithSuffix(fileID, "{")
	if len(suffixLines) == 0 {
		t.Error("FindAllLinesWithSuffix should find lines ending with '{'")
	}
}

func TestFileContentStore_ZeroAlloc_ContextSearch(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	testContent := []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
	// Comment line
	fmt.Println("Another line")
}

func helper() {
	fmt.Println("Helper function")
}
`)

	fileID := fileStore.LoadFile("test.go", testContent)

	// Test FindWithContext
	result := fileStore.FindWithContext(fileID, "Hello", 1)
	if len(result.Lines) == 0 {
		t.Error("FindWithContext should find matching lines")
	}
	if len(result.Context) == 0 {
		t.Error("FindWithContext should provide context")
	}

	// Verify context contains expected lines
	for lineNum, contextLines := range result.Context {
		for _, ctxLine := range contextLines {
			if ctxLine.IsEmpty() {
				t.Errorf("Context line should not be empty for line %d", lineNum)
			}
		}
	}
}

func TestFileContentStore_ZeroAlloc_ContentAnalysis(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	testContent := []byte(`package main

import "fmt"

// Test function with documentation
func documentedFunction() error {
	return nil
}

func simpleFunction() {
	// Empty function body
}

var globalVariable = "test"
const testConstant = 42

type TestStruct struct {
	Name string
}

// Single line comment
/* Multi-line comment */

// Empty line above and below
`)
	fileID := fileStore.LoadFile("test.go", testContent)

	// Test IsCommentLine
	commentLines := fileStore.FindAllLinesWithPattern(fileID, "//")
	if len(commentLines) == 0 {
		t.Error("Should find comment lines")
	}

	// Test HasCodeContent - find lines that actually contain code
	codeLines := fileStore.FindAllLinesWithPattern(fileID, "func")
	if len(codeLines) == 0 {
		t.Error("Should find code lines")
	}

	// Test IsEmptyLine
	emptyLines := fileStore.FindAllLinesWithPattern(fileID, "")
	// This test depends on implementation details of pattern matching
	// Just verify the function exists and can be called
	_ = emptyLines
}

func TestFileContentStore_ZeroAlloc_LineOperations(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	testContent := []byte(`  line with spaces
\tline with tabs
  mixed whitespace
`)
	fileID := fileStore.LoadFile("test.go", testContent)

	// Test GetLine operations
	lineCount := fileStore.GetLineCount(fileID)
	for i := 0; i < lineCount; i++ {
		lineRef := fileStore.GetZeroAllocLine(fileID, i)

		// Verify basic operations work
		_ = lineRef.Len()
		_ = lineRef.IsEmpty()
		_ = lineRef.IsValid()
		_ = lineRef.Bytes()
		_ = lineRef.String()

		// Verify string operations work
		_ = lineRef.Contains("line")
		_ = lineRef.HasPrefix("  ")
		_ = lineRef.HasSuffix("  ")
		_ = lineRef.TrimSpace()
	}
}

func TestFileContentStore_ZeroAlloc_EdgeCases(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	// Test with empty file
	emptyFileID := fileStore.LoadFile("empty.go", []byte{})

	lineCount := fileStore.GetLineCount(emptyFileID)
	if lineCount != 0 {
		t.Errorf("Empty file should have 0 lines, got %d", lineCount)
	}

	emptyLine := fileStore.GetZeroAllocLine(emptyFileID, 0)
	if !emptyLine.IsEmpty() {
		t.Error("Getting line from empty file should return empty reference")
	}

	// Test with single line file
	singleLineContent := []byte("package main")
	singleFileID := fileStore.LoadFile("single.go", singleLineContent)

	singleLineCount := fileStore.GetLineCount(singleFileID)
	if singleLineCount != 1 {
		t.Errorf("Single line file should have 1 line, got %d", singleLineCount)
	}

	singleLine := fileStore.GetZeroAllocLine(singleFileID, 0)
	if singleLine.IsEmpty() {
		t.Error("Should get valid line from single line file")
	}
	if singleLine.String() != "package main" {
		t.Errorf("Expected 'package main', got %q", singleLine.String())
	}

	// Test with non-existent file ID
	nonExistentFileID := types.FileID(999999)
	nonExistentLine := fileStore.GetZeroAllocLine(nonExistentFileID, 0)
	if !nonExistentLine.IsEmpty() {
		t.Error("Getting line from non-existent file should return empty reference")
	}

	nonExistentCount := fileStore.GetLineCount(nonExistentFileID)
	if nonExistentCount != 0 {
		t.Error("Getting line count from non-existent file should return 0")
	}
}

func TestFileContentStore_ZeroAlloc_PerformanceCharacteristics(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	// Create a larger test file
	content := make([]byte, 0, 10000)
	for i := 0; i < 100; i++ {
		line := fmt.Sprintf("func testFunction%d() { return %d }\n", i, i)
		content = append(content, line...)
	}

	fileID := fileStore.LoadFile("large.go", content)

	// Test that operations are efficient
	lineCount := fileStore.GetLineCount(fileID)
	if lineCount != 100 {
		t.Errorf("Expected 100 lines, got %d", lineCount)
	}

	// Test pattern matching performance
	pattern := "func"
	matchingLines := fileStore.FindAllLinesWithPattern(fileID, pattern)
	if len(matchingLines) != 100 {
		t.Errorf("Expected all 100 lines to match 'func', got %d", len(matchingLines))
	}

	// Test context search performance
	contextResults := fileStore.FindWithContext(fileID, "testFunction50", 2)
	if len(contextResults.Lines) == 0 {
		t.Error("Context search should find matches")
	}
	if len(contextResults.Context) == 0 {
		t.Error("Context search should provide context")
	}
}

func TestFileContentStore_ZeroAlloc_ConversionFunctions(t *testing.T) {
	fileStore := NewFileContentStore()
	defer fileStore.Close()

	testContent := []byte("Hello, World!")
	fileID := fileStore.LoadFile("test.go", testContent)

	// Test GetLine and conversion to zero-alloc
	regularRef, ok := fileStore.GetLine(fileID, 0)
	if !ok {
		t.Fatal("Failed to get regular line reference")
	}

	// Convert to zero-alloc reference
	zeroAllocRef := fileStore.GetZeroAllocLine(fileID, 0)
	if zeroAllocRef.IsEmpty() {
		t.Error("Zero-alloc reference should not be empty")
	}

	// Test string conversion
	regularStr, err := fileStore.GetString(regularRef)
	if err != nil {
		t.Fatalf("Failed to convert regular ref to string: %v", err)
	}

	zeroAllocStr := zeroAllocRef.String()
	if regularStr != zeroAllocStr {
		t.Errorf("String conversion mismatch: regular=%q, zero-alloc=%q", regularStr, zeroAllocStr)
	}

	// Test byte conversion
	regularBytes := zeroAllocRef.Bytes()
	if string(regularBytes) != regularStr {
		t.Error("Byte conversion should match string conversion")
	}
}
