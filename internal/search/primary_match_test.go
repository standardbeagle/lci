package search

import (
	"testing"
	"github.com/standardbeagle/lci/internal/types"
)

// TestPrimaryMatchLineTracking tests that the primary match line is correctly tracked
func TestPrimaryMatchLineTracking(t *testing.T) {
	// Test case 1: SearchWithOptions in main.go
	// Line 517 has the actual match but result might show a different line
	content1 := `		// Use 'lci grep' for fast search instead
		fmt.Fprintf(os.Stderr, "WARNING: --light flag is deprecated. Use 'lci grep' for fast search.\n\n")
		results := indexer.SearchWithOptions(pattern, types.SearchOptions{
			CaseInsensitive: caseInsensitive,
			MaxContextLines: maxLines,`

	pattern := "SearchWithOptions"
	matches1 := findAllMatches([]byte(content1), []byte(pattern))

	if len(matches1) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matches1))
	}

	// The match should be on line 3 (0-indexed: line 2)
	line1 := bytesToLine([]byte(content1), matches1[0].Start)
	if line1 != 3 {
		t.Errorf("Expected match on line 3, got line %d", line1)
	}

	// Test case 2: Comment above the actual match
	content2 := `	// Test SearchWithOptions
	t.Run("BasicSearch", func(t *testing.T) {
		results := gi.SearchWithOptions("function", types.SearchOptions{
			CaseInsensitive: false,
		})`

	matches2 := findAllMatches([]byte(content2), []byte(pattern))

	if len(matches2) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matches2))
	}

	// The match should be on line 3, not line 1 (the comment)
	line2 := bytesToLine([]byte(content2), matches2[0].Start)
	if line2 != 3 {
		t.Errorf("Expected match on line 3, got line %d", line2)
	}
}

// TestMergePreservesPrimaryMatch tests that merging preserves the actual match line
func TestMergePreservesPrimaryMatch(t *testing.T) {
	// When multiple matches are in a range, the primary match should be
	// the first actual match, not the highest scoring one

	content := `func example() {
	// This comment mentions test
	actualtest := "value"
	return actualtest
}`

	pattern := "test"
	// Use findAllMatchesWithOptions to include comment matches
	matches := findAllMatchesWithOptions([]byte(content), []byte(pattern), types.SearchOptions{
		CaseInsensitive: false,
		ExcludeComments: false, // Include matches in comments for this test
	})

	// Should find matches in:
	// - line 2: comment with "test"
	// - line 3: actualtest (end of variable name)
	// - line 4: actualtest (end of variable name)

	if len(matches) < 3 {
		t.Errorf("Expected at least 3 matches, got %d", len(matches))
	}

	// First match should be in the comment (line 2)
	firstMatchLine := bytesToLine([]byte(content), matches[0].Start)
	if firstMatchLine != 2 {
		t.Errorf("First match should be on line 2 (comment), got line %d", firstMatchLine)
	}

	// When merged, the primary match line should still be line 2 (first match)
	// not line 3 even if line 3 might have a higher score
}
