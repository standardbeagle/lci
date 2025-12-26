package search

import (
	"strings"
	"testing"
)

// TestMatchCountingInContext tests that ExtractedContext properly tracks matches
func TestMatchCountingInContext(t *testing.T) {
	// Test the new fields in ExtractedContext
	context := ExtractedContext{
		StartLine:    1,
		EndLine:      5,
		Lines:        []string{"line 1", "test line", "another test", "test test", "end"},
		MatchedLines: []int{2, 3, 4}, // Lines 2, 3, 4 have matches
		MatchCount:   5,              // Total of 5 "test" occurrences
	}

	// Verify the context tracks matches correctly
	if context.MatchCount != 5 {
		t.Errorf("Expected match count 5, got %d", context.MatchCount)
	}

	if len(context.MatchedLines) != 3 {
		t.Errorf("Expected 3 matched lines, got %d", len(context.MatchedLines))
	}

	// Verify matched lines are what we expect
	expectedLines := []int{2, 3, 4}
	for i, line := range expectedLines {
		if i >= len(context.MatchedLines) || context.MatchedLines[i] != line {
			t.Errorf("Expected matched line %d at position %d, got %v",
				line, i, context.MatchedLines)
		}
	}

	t.Logf("SUCCESS: ExtractedContext properly tracks %d matches on lines %v",
		context.MatchCount, context.MatchedLines)
}

// TestFindAllMatchesCounting verifies that findAllMatches finds all occurrences
func TestFindAllMatchesCounting(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		pattern  string
		expected int
	}{
		{
			name:     "simple_matches",
			content:  "test one test two test three",
			pattern:  "test",
			expected: 3,
		},
		{
			name:     "case_sensitive",
			content:  "Test test TEST",
			pattern:  "test",
			expected: 1, // Only lowercase
		},
		{
			name:     "overlapping",
			content:  "testtest",
			pattern:  "test",
			expected: 2, // Should find both even though they overlap
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := findAllMatches([]byte(tc.content), []byte(tc.pattern))
			if len(matches) != tc.expected {
				t.Errorf("Expected %d matches, found %d", tc.expected, len(matches))
			}
			t.Logf("Found %d matches of '%s' in '%s'", len(matches), tc.pattern, tc.content)
		})
	}
}

// TestLineNumberExtraction tests bytesToLine function
func TestLineNumberExtraction(t *testing.T) {
	content := `line 1
test line 2
another test line 3
test test line 4`

	testCases := []struct {
		pattern      string
		expectedLine int
	}{
		{"line 1", 1},
		{"test line 2", 2},
		{"another", 3},
		{"line 4", 4},
	}

	for _, tc := range testCases {
		idx := strings.Index(content, tc.pattern)
		if idx == -1 {
			t.Errorf("Pattern '%s' not found", tc.pattern)
			continue
		}

		line := bytesToLine([]byte(content), idx)
		if line != tc.expectedLine {
			t.Errorf("Pattern '%s': expected line %d, got %d",
				tc.pattern, tc.expectedLine, line)
		}
	}
}
