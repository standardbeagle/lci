package search

import (
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/searchtypes"
)

// TestMergeKeepsTrackOfMatchLines tests that merged results lose track of which lines contain matches
// This test demonstrates the current behavior (which we want to improve)
func TestMergeKeepsTrackOfMatchLines(t *testing.T) {
	// Create test content with multiple matches
	content := `package main

// This test function has multiple matches
func TestExample() {
	// First test in comment
	test := "value"
	
	// Another line without matches
	x := 10
	
	// Second test here
	result := test + " more"
	
	// Final test match
	println(test)
}`

	// Test the internal mergeFileResults function behavior
	t.Run("InternalMergeLogic", func(t *testing.T) {
		// We're just testing the match finding logic here
		_ = strings.Split(content, "\n")

		// Find all matches of "test"
		pattern := "test"
		matches := findAllMatches([]byte(content), []byte(pattern))

		// Count how many unique lines have matches
		uniqueMatchLines := make(map[int]bool)
		for _, match := range matches {
			line := bytesToLine([]byte(content), match.Start)
			uniqueMatchLines[line] = true
		}

		t.Logf("Found %d matches on %d unique lines", len(matches), len(uniqueMatchLines))
		t.Logf("Match lines: %v", uniqueMatchLines)

		// The merged result will only track ONE match line
		// This is the problem we want to highlight
		if len(matches) > 1 {
			t.Logf("CURRENT BEHAVIOR: Multiple matches (%d) but merged result only tracks one primary match line", len(matches))
			t.Logf("This means we lose information about which lines actually contain matches")
		}
	})
}

// TestMergeScenarios tests various merge scenarios with expected behaviors
func TestMergeScenarios(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		pattern           string
		expectMergedCount int
		description       string
	}{
		{
			name: "adjacent_lines",
			content: `line one
test here
test there
test again
line five`,
			pattern:           "test",
			expectMergedCount: 1,
			description:       "Adjacent matches should merge into one result",
		},
		{
			name: "separated_matches",
			content: `test at top
line two
line three
line four
line five
test at bottom`,
			pattern:           "test",
			expectMergedCount: 2,
			description:       "Separated matches should create multiple results",
		},
		{
			name: "within_context_window",
			content: `line one
test here
line three
test there
line five`,
			pattern:           "test",
			expectMergedCount: 1,
			description:       "Matches within ±2 line window should merge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find matches
			matches := findAllMatches([]byte(tt.content), []byte(tt.pattern))

			// Count unique match lines
			uniqueLines := make(map[int]bool)
			for _, match := range matches {
				line := bytesToLine([]byte(tt.content), match.Start)
				uniqueLines[line] = true
			}

			t.Logf("%s: Found %d matches on %d unique lines",
				tt.description, len(matches), len(uniqueLines))

			// In the current implementation, merged results lose track
			// of individual match lines
			if len(uniqueLines) > 1 {
				t.Logf("WARNING: Merging will lose match line information for %d lines",
					len(uniqueLines)-1)
			}
		})
	}
}

// TestDesiredMergeTracking shows what we WANT the behavior to be
func TestDesiredMergeTracking(t *testing.T) {
	t.Skip("Skipping - this shows desired future behavior")

	// Desired behavior options:
	// 1. Add MatchLines []int to ExtractedContext or Result
	// 2. Create multiple Results with shared context but different Line values
	// 3. Add highlighting/marking to context lines that contain matches

	// Example of option 1:
	type DesiredResult struct {
		searchtypes.GrepResult
		MatchLines []int // All lines that contain matches within this context
	}

	// Example of option 3:
	type DesiredContext struct {
		searchtypes.ExtractedContext
		MatchedLines map[int]bool // Which lines in Lines array contain matches
	}
}
