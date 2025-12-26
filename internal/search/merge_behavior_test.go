package search

import (
	"strings"
	"testing"
)

// TestMergeBehaviorShowsLostMatchLines demonstrates that merging loses track of which lines contain matches
func TestMergeBehaviorShowsLostMatchLines(t *testing.T) {
	// Example content with multiple matches
	content := `package main

func process() {
	// First match here
	match := "value1"
	
	// Some other code
	x := 10
	
	// Second match here
	result := match + "suffix"
	
	// Third match in comment
	// This is another match
	
	// Final match
	println(match)
}`

	pattern := "match"

	// Find all occurrences
	contentLower := strings.ToLower(content)
	lines := strings.Split(content, "\n")

	// Track which lines have matches
	matchingLines := []int{}
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), pattern) {
			matchingLines = append(matchingLines, i+1) // 1-based line numbers
		}
	}

	// Count total matches
	totalMatches := strings.Count(contentLower, pattern)

	t.Logf("Pattern '%s' appears %d times across %d lines", pattern, totalMatches, len(matchingLines))
	t.Logf("Lines with matches: %v", matchingLines)

	// The PROBLEM we're highlighting:
	// When results are merged, we only get ONE Line value per merged result
	// but there are actually MULTIPLE lines with matches in the context

	t.Run("MergedResultLosesMatchLines", func(t *testing.T) {
		// Simulate what happens with merging:
		// If these matches get merged into one result, we lose track of individual match lines

		if len(matchingLines) > 1 {
			t.Logf("PROBLEM DEMONSTRATED: %d lines contain matches, but merged result would only track 1 primary line", len(matchingLines))
			t.Logf("Lost match line information for lines: %v", matchingLines[1:])
		}

		// This is the behavior we want to fix - merged results should somehow
		// track ALL lines that contain matches, not just one
	})
}

// TestMergeConditionsMatrix tests various merge scenarios to document current behavior
func TestMergeConditionsMatrix(t *testing.T) {
	scenarios := []struct {
		name           string
		description    string
		lineLayout     string // Visual representation of match positions
		expectedMerges string // How we expect them to merge
	}{
		{
			name:        "adjacent_lines",
			description: "Matches on consecutive lines",
			lineLayout: `
Line 1: [MATCH]
Line 2: [MATCH]  
Line 3: [MATCH]
`,
			expectedMerges: "All 3 merge into 1 result (lines 1-3 shown, but only line 1 tracked)",
		},
		{
			name:        "within_window",
			description: "Matches within ±2 line merge window",
			lineLayout: `
Line 1: [MATCH]
Line 2: (no match)
Line 3: [MATCH]
`,
			expectedMerges: "Both merge into 1 result (lines 1-3 shown, but only line 1 tracked)",
		},
		{
			name:        "beyond_window",
			description: "Matches beyond ±2 line merge window",
			lineLayout: `
Line 1: [MATCH]
Line 2: (no match)
Line 3: (no match)
Line 4: (no match)
Line 5: (no match)
Line 6: [MATCH]
`,
			expectedMerges: "2 separate results (lines 1-3 and 4-6 shown)",
		},
		{
			name:        "function_expansion",
			description: "First match in function expands to function bounds",
			lineLayout: `
Line 1: func example() {
Line 2:     [MATCH]      <- First match triggers expansion
Line 3:     code
Line 4:     [MATCH]      <- This gets absorbed  
Line 5: }
Line 6: [MATCH]          <- Outside function, separate result
`,
			expectedMerges: "2 results: one for entire function (lines 1-5), one for line 6±2",
		},
		{
			name:        "multiple_per_line",
			description: "Multiple matches on same line",
			lineLayout: `
Line 1: [MATCH] and [MATCH] and [MATCH]
`,
			expectedMerges: "1 result tracking line 1 (but loses info about 3 matches)",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Scenario: %s", scenario.description)
			t.Logf("Layout:%s", scenario.lineLayout)
			t.Logf("Current behavior: %s", scenario.expectedMerges)
			t.Logf("ISSUE: Only one match line is tracked per merged result")
		})
	}
}

// TestProposedSolutions suggests ways to fix the match line tracking issue
func TestProposedSolutions(t *testing.T) {
	t.Skip("This test documents proposed solutions, not current behavior")

	// Solution 1: Add MatchLines field to track all lines with matches
	type Solution1_Result struct {
		Line       int   // Primary match line (for compatibility)
		MatchLines []int // ALL lines that contain matches in this context
		Context    ExtractedContext
	}

	// Solution 2: Add match indicators to context
	type Solution2_Context struct {
		StartLine    int
		EndLine      int
		Lines        []string
		MatchedLines []bool // Parallel array indicating which lines have matches
	}

	// Solution 3: Return multiple results with same context but different match lines
	// This maintains backward compatibility but returns more results

	t.Log("Solution 1: Add MatchLines []int field to track all match locations")
	t.Log("Solution 2: Add MatchedLines []bool to ExtractedContext")
	t.Log("Solution 3: Return multiple GrepResults sharing context but with different Line values")

	// The best solution depends on:
	// - Backward compatibility requirements
	// - Performance considerations
	// - How the results are consumed by MCP/CLI
}
