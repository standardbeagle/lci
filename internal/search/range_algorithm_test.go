package search

import (
	"github.com/standardbeagle/lci/internal/types"
	"testing"
)

// Test the core range building logic without mocks
func TestBuildRangesWithWindow(t *testing.T) {
	tests := []struct {
		name        string
		totalLines  int
		matchLines  []int
		window      int
		wantRanges  []struct{ start, end int }
		description string
	}{
		{
			name:        "single_match_with_window",
			totalLines:  10,
			matchLines:  []int{5},
			window:      2,
			wantRanges:  []struct{ start, end int }{{3, 7}},
			description: "Single match at line 5 should give range 3-7 with ±2 window",
		},
		{
			name:        "boundary_at_start",
			totalLines:  10,
			matchLines:  []int{2},
			window:      2,
			wantRanges:  []struct{ start, end int }{{1, 4}},
			description: "Match at line 2 should give range 1-4 (clamped at start)",
		},
		{
			name:        "boundary_at_end",
			totalLines:  10,
			matchLines:  []int{9},
			window:      2,
			wantRanges:  []struct{ start, end int }{{7, 10}},
			description: "Match at line 9 should give range 7-10 (clamped at end)",
		},
		{
			name:        "overlapping_ranges_merge",
			totalLines:  10,
			matchLines:  []int{4, 6},
			window:      2,
			wantRanges:  []struct{ start, end int }{{2, 8}},
			description: "Matches at lines 4 and 6 should merge into single range 2-8",
		},
		{
			name:        "non_overlapping_ranges",
			totalLines:  15,
			matchLines:  []int{3, 10},
			window:      2,
			wantRanges:  []struct{ start, end int }{{1, 5}, {8, 12}},
			description: "Matches at lines 3 and 10 should create two separate ranges",
		},
		{
			name:        "three_overlapping_ranges",
			totalLines:  15,
			matchLines:  []int{5, 7, 9},
			window:      2,
			wantRanges:  []struct{ start, end int }{{3, 11}},
			description: "Three overlapping matches should merge into one range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := buildRangesWithWindow(tt.matchLines, tt.window, tt.totalLines)

			if len(ranges) != len(tt.wantRanges) {
				t.Errorf("%s: got %d ranges, want %d", tt.description, len(ranges), len(tt.wantRanges))
				return
			}

			for i, r := range ranges {
				if r.start != tt.wantRanges[i].start || r.end != tt.wantRanges[i].end {
					t.Errorf("%s: range %d = {%d, %d}, want {%d, %d}",
						tt.description, i, r.start, r.end,
						tt.wantRanges[i].start, tt.wantRanges[i].end)
				}
			}
		})
	}
}

// Test function expansion logic
func TestExpandFirstMatchToFunction(t *testing.T) {
	tests := []struct {
		name         string
		matchLine    int
		funcStart    int
		funcEnd      int
		isInFunction bool
		wantExpanded bool
		wantStart    int
		wantEnd      int
		description  string
	}{
		{
			name:         "match_inside_function",
			matchLine:    10,
			funcStart:    8,
			funcEnd:      15,
			isInFunction: true,
			wantExpanded: true,
			wantStart:    8,
			wantEnd:      15,
			description:  "Match inside function should expand to function boundaries",
		},
		{
			name:         "match_at_function_declaration",
			matchLine:    8,
			funcStart:    8,
			funcEnd:      15,
			isInFunction: true,
			wantExpanded: false,
			wantStart:    6,  // matchLine - 2
			wantEnd:      10, // matchLine + 2
			description:  "Match at function declaration line should NOT expand",
		},
		{
			name:         "match_outside_function",
			matchLine:    5,
			funcStart:    0,
			funcEnd:      0,
			isInFunction: false,
			wantExpanded: false,
			wantStart:    3,
			wantEnd:      7,
			description:  "Match outside function should use ±2 window",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var symbol *types.EnhancedSymbol
			if tt.isInFunction {
				symbol = &types.EnhancedSymbol{
					Symbol: types.Symbol{
						Type:    types.SymbolTypeFunction,
						Line:    tt.funcStart,
						EndLine: tt.funcEnd,
					},
				}
			}

			startLine, endLine, expanded := expandToFunctionIfInside(tt.matchLine, symbol, 100)

			if expanded != tt.wantExpanded {
				t.Errorf("%s: expanded = %v, want %v", tt.description, expanded, tt.wantExpanded)
			}
			if startLine != tt.wantStart {
				t.Errorf("%s: startLine = %d, want %d", tt.description, startLine, tt.wantStart)
			}
			if endLine != tt.wantEnd {
				t.Errorf("%s: endLine = %d, want %d", tt.description, endLine, tt.wantEnd)
			}
		})
	}
}

// Test the full range merging algorithm
func TestMergeOverlappingRanges(t *testing.T) {
	tests := []struct {
		name       string
		ranges     []lineRange
		wantMerged []lineRange
	}{
		{
			name: "no_overlap",
			ranges: []lineRange{
				{start: 1, end: 5, matchLine: 3},
				{start: 10, end: 15, matchLine: 12},
			},
			wantMerged: []lineRange{
				{start: 1, end: 5, matchLine: 3},
				{start: 10, end: 15, matchLine: 12},
			},
		},
		{
			name: "adjacent_ranges",
			ranges: []lineRange{
				{start: 1, end: 5, matchLine: 3},
				{start: 6, end: 10, matchLine: 8},
			},
			wantMerged: []lineRange{
				{start: 1, end: 10, matchLine: 3},
			},
		},
		{
			name: "overlapping_ranges",
			ranges: []lineRange{
				{start: 1, end: 7, matchLine: 4},
				{start: 5, end: 10, matchLine: 7},
			},
			wantMerged: []lineRange{
				{start: 1, end: 10, matchLine: 4},
			},
		},
		{
			name: "multiple_overlaps",
			ranges: []lineRange{
				{start: 1, end: 5, matchLine: 3},
				{start: 4, end: 8, matchLine: 6},
				{start: 7, end: 11, matchLine: 9},
				{start: 15, end: 20, matchLine: 17},
			},
			wantMerged: []lineRange{
				{start: 1, end: 11, matchLine: 3},
				{start: 15, end: 20, matchLine: 17},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := mergeOverlappingRanges(tt.ranges)

			if len(merged) != len(tt.wantMerged) {
				t.Errorf("got %d merged ranges, want %d", len(merged), len(tt.wantMerged))
				return
			}

			for i, r := range merged {
				want := tt.wantMerged[i]
				if r.start != want.start || r.end != want.end {
					t.Errorf("merged[%d] = {%d-%d}, want {%d-%d}",
						i, r.start, r.end, want.start, want.end)
				}
			}
		})
	}
}

// Helper functions that implement the algorithm logic

type lineRange struct {
	start, end int
	matchLine  int
	isFunction bool
	score      float64
}

func buildRangesWithWindow(matchLines []int, window int, totalLines int) []lineRange {
	var ranges []lineRange

	for _, line := range matchLines {
		start := line - window
		if start < 1 {
			start = 1
		}
		end := line + window
		if end > totalLines {
			end = totalLines
		}

		ranges = append(ranges, lineRange{
			start:     start,
			end:       end,
			matchLine: line,
		})
	}

	// Sort and merge
	return mergeOverlappingRanges(ranges)
}

func expandToFunctionIfInside(matchLine int, symbol *types.EnhancedSymbol, totalLines int) (int, int, bool) {
	if symbol == nil || (symbol.Type != types.SymbolTypeFunction && symbol.Type != types.SymbolTypeMethod) {
		// Not in a function, use ±2 window
		start := matchLine - 2
		if start < 1 {
			start = 1
		}
		end := matchLine + 2
		if end > totalLines {
			end = totalLines
		}
		return start, end, false
	}

	// Check if match is INSIDE the function (not at declaration)
	if matchLine > symbol.Line && matchLine <= symbol.EndLine {
		return symbol.Line, symbol.EndLine, true
	}

	// Match is at function declaration or outside, use window
	start := matchLine - 2
	if start < 1 {
		start = 1
	}
	end := matchLine + 2
	if end > totalLines {
		end = totalLines
	}
	return start, end, false
}

func mergeOverlappingRanges(ranges []lineRange) []lineRange {
	if len(ranges) == 0 {
		return ranges
	}

	// Sort by start line
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].start < ranges[i].start {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}

	// Merge overlapping or adjacent ranges
	var merged []lineRange
	current := ranges[0]

	for i := 1; i < len(ranges); i++ {
		next := ranges[i]

		// Check if overlapping or adjacent (within 1 line)
		if next.start <= current.end+1 {
			// Merge
			if next.end > current.end {
				current.end = next.end
			}
			// Keep the better score/match position
			if next.score > current.score {
				current.matchLine = next.matchLine
				current.score = next.score
			}
			if next.isFunction {
				current.isFunction = true
			}
		} else {
			// No overlap, save current and start new
			merged = append(merged, current)
			current = next
		}
	}

	// Don't forget the last range
	merged = append(merged, current)

	return merged
}

// Test removing ranges contained in function ranges
func TestRemoveContainedRanges(t *testing.T) {
	tests := []struct {
		name       string
		ranges     []lineRange
		wantRanges []lineRange
	}{
		{
			name: "remove_range_inside_function",
			ranges: []lineRange{
				{start: 5, end: 20, matchLine: 10, isFunction: true},
				{start: 8, end: 12, matchLine: 11, isFunction: false},
				{start: 25, end: 30, matchLine: 27, isFunction: false},
			},
			wantRanges: []lineRange{
				{start: 5, end: 20, matchLine: 10, isFunction: true},
				{start: 25, end: 30, matchLine: 27, isFunction: false},
			},
		},
		{
			name: "keep_non_overlapping",
			ranges: []lineRange{
				{start: 5, end: 10, matchLine: 7, isFunction: true},
				{start: 15, end: 20, matchLine: 17, isFunction: false},
			},
			wantRanges: []lineRange{
				{start: 5, end: 10, matchLine: 7, isFunction: true},
				{start: 15, end: 20, matchLine: 17, isFunction: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeContainedRanges(tt.ranges)

			if len(result) != len(tt.wantRanges) {
				t.Errorf("got %d ranges, want %d", len(result), len(tt.wantRanges))
				return
			}

			for i, r := range result {
				want := tt.wantRanges[i]
				if r.start != want.start || r.end != want.end || r.isFunction != want.isFunction {
					t.Errorf("range[%d] = {%d-%d func:%v}, want {%d-%d func:%v}",
						i, r.start, r.end, r.isFunction,
						want.start, want.end, want.isFunction)
				}
			}
		})
	}
}

func removeContainedRanges(ranges []lineRange) []lineRange {
	var filtered []lineRange

	for _, r := range ranges {
		contained := false
		for _, fr := range ranges {
			if fr.isFunction && !r.isFunction &&
				r.start >= fr.start && r.end <= fr.end &&
				r.matchLine != fr.matchLine {
				contained = true
				break
			}
		}
		if !contained {
			filtered = append(filtered, r)
		}
	}

	return filtered
}
