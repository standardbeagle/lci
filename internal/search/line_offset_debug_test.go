package search

import (
	"os"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestActualFileLineOffsets(t *testing.T) {
	// Read the actual Rust file
	content, err := os.ReadFile("../../tests/search-comparison/fixtures/rust-sample/src/auth.rs")
	if err != nil {
		t.Skipf("Could not read test file: %v", err)
	}

	// Compute line offsets
	lineOffsets := types.ComputeLineOffsets(content)

	t.Logf("File size: %d bytes", len(content))
	t.Logf("Total line offsets: %d", len(lineOffsets))

	// Show line offsets around line 24
	for i := 20; i < 30 && i < len(lineOffsets); i++ {
		t.Logf("LineOffsets[%d] = %d (line %d starts here)", i, lineOffsets[i], i+1)
	}

	// Find where "invalid credentials" appears
	searchText := "invalid credentials"
	matchStart := -1
	for i := 0; i < len(content)-len(searchText); i++ {
		if string(content[i:i+len(searchText)]) == searchText {
			matchStart = i
			break
		}
	}

	if matchStart == -1 {
		t.Fatal("Could not find 'invalid credentials' in content")
	}

	t.Logf("Match starts at byte offset: %d", matchStart)

	// Count newlines before match to get actual line number
	newlineCount := 0
	for i := 0; i < matchStart; i++ {
		if content[i] == '\n' {
			newlineCount++
		}
	}
	actualLine := newlineCount + 1
	t.Logf("Actual line (newline count + 1): %d", actualLine)

	// Use bytesToLine helper
	calculatedLine := bytesToLine(content, matchStart)
	t.Logf("Calculated line (bytesToLine): %d", calculatedLine)

	if calculatedLine != actualLine {
		t.Errorf("Line number mismatch! bytesToLine gives %d, actual is %d", calculatedLine, actualLine)
	}
}
