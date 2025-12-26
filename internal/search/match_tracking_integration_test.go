package search

import (
	"testing"
	"github.com/standardbeagle/lci/internal/types"
)

// TestMatchTrackingIntegration tests that the match tracking actually works
func TestMatchTrackingIntegration(t *testing.T) {
	// Create test content
	content := `func example() {
	test := "first"
	test = "second"
	test = "third"
}`

	// Find all matches
	pattern := "test"
	matches := findAllMatches([]byte(content), []byte(pattern))

	// Verify that findAllMatches finds all occurrences
	if len(matches) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(matches))
	}

	// Verify matches are on the correct lines
	matchesByLine := make(map[int]int)
	for _, match := range matches {
		line := bytesToLine([]byte(content), match.Start)
		matchesByLine[line]++
	}

	// Each line should have one match
	for line := 2; line <= 4; line++ {
		if matchesByLine[line] != 1 {
			t.Errorf("Line %d: expected 1 match, got %d", line, matchesByLine[line])
		}
	}

	t.Logf("SUCCESS: findAllMatches correctly found %d matches", len(matches))
}

// TestMultipleMatchesPerLine tests tracking when a line has multiple matches
func TestMultipleMatchesPerLine(t *testing.T) {
	content := `func process() {
	// test test test - multiple matches
	result := test + test
}`

	pattern := "test"
	// Use findAllMatchesWithOptions to include comment matches
	matches := findAllMatchesWithOptions([]byte(content), []byte(pattern), types.SearchOptions{
		CaseInsensitive: false,
		ExcludeComments: false, // Include matches in comments for this test
	})

	// Line 2 has 3 matches, line 3 has 2 matches = 5 total
	if len(matches) != 5 {
		t.Errorf("Expected 5 total matches, got %d", len(matches))
	}

	// Count matches per line
	matchesByLine := make(map[int]int)
	for _, match := range matches {
		line := bytesToLine([]byte(content), match.Start)
		matchesByLine[line]++
	}

	// Verify line 2 has 3 matches
	if matchesByLine[2] != 3 {
		t.Errorf("Line 2: expected 3 matches, got %d", matchesByLine[2])
	}

	// Verify line 3 has 2 matches
	if matchesByLine[3] != 2 {
		t.Errorf("Line 3: expected 2 matches, got %d", matchesByLine[3])
	}

	t.Logf("SUCCESS: Correctly found %d matches across %d lines (including multiple matches per line)",
		len(matches), len(matchesByLine))
}
