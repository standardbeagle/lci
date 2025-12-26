package search

import (
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// SemanticFilterTestCase represents a data-driven test case for semantic filtering
type SemanticFilterTestCase struct {
	Name             string
	Content          string
	EnhancedSymbols  []*types.EnhancedSymbol
	Matches          []ZeroAllocSearchMatch
	FilterOptions    types.SearchOptions
	Pattern          string // Optional pattern for filtering
	ExpectedCount    int
	ExpectedLines    []int // Expected line numbers in results
	ShouldContain    []string
	ShouldNotContain []string
}

// RunSemanticFilterTest runs a data-driven semantic filter test
func RunSemanticFilterTest(t *testing.T, tc SemanticFilterTestCase) {
	t.Helper()

	fileStore := core.NewFileContentStore()
	fileID := fileStore.LoadFile("test.go", []byte(tc.Content))

	// Create file info with symbols
	fileInfo := &types.FileInfo{
		ID:              fileID,
		Content:         []byte(tc.Content),
		EnhancedSymbols: tc.EnhancedSymbols,
	}

	filter := NewZeroAllocSemanticFilter(fileStore)
	filtered := filter.ApplySemanticFilteringZeroAlloc(fileInfo, tc.Matches, tc.Pattern, tc.FilterOptions)

	// Check expected count
	if len(filtered) != tc.ExpectedCount {
		t.Errorf("%s: expected %d matches, got %d", tc.Name, tc.ExpectedCount, len(filtered))
		t.Logf("Filtered results:")
		for i, match := range filtered {
			t.Logf("  [%d] Line %d: %q", i, match.Line, match.Pattern)
		}
	}

	// Check expected lines if specified
	if len(tc.ExpectedLines) > 0 {
		actualLines := make([]int, len(filtered))
		for i, match := range filtered {
			actualLines[i] = match.Line
		}

		if len(actualLines) != len(tc.ExpectedLines) {
			t.Errorf("%s: expected lines %v, got %v", tc.Name, tc.ExpectedLines, actualLines)
		} else {
			for i, expectedLine := range tc.ExpectedLines {
				if actualLines[i] != expectedLine {
					t.Errorf("%s: expected line %d at position %d, got %d",
						tc.Name, expectedLine, i, actualLines[i])
				}
			}
		}
	}

	// Check patterns that should be included
	for _, pattern := range tc.ShouldContain {
		found := false
		for _, match := range filtered {
			if match.Pattern == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected to find pattern %q in results", tc.Name, pattern)
		}
	}

	// Check patterns that should be excluded
	for _, pattern := range tc.ShouldNotContain {
		for _, match := range filtered {
			if match.Pattern == pattern {
				t.Errorf("%s: pattern %q should not be in results", tc.Name, pattern)
			}
		}
	}
}

// Helper to create test content with known line numbers
func CreateTestContent(lines ...string) string {
	return strings.Join(lines, "\n")
}

// Helper to find line number (1-based) for a pattern in content
func FindLineNumber(content, pattern string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, pattern) {
			return i + 1 // Return 1-based line number
		}
	}
	return -1
}

// Helper to create matches from patterns in content
func CreateMatchesFromPatterns(content string, patterns []string) []ZeroAllocSearchMatch {
	lines := strings.Split(content, "\n")
	matches := []ZeroAllocSearchMatch{}

	for _, pattern := range patterns {
		for i, line := range lines {
			if idx := strings.Index(line, pattern); idx >= 0 {
				matches = append(matches, ZeroAllocSearchMatch{
					Line:    i + 1, // 1-based
					Start:   idx,
					End:     idx + len(pattern),
					Pattern: pattern,
				})
			}
		}
	}

	return matches
}
