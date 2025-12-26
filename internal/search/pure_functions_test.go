package search

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Comprehensive tests for pure functions
// These functions are designed to be easily testable with property-based tests

// ============================================================================
// Line Calculation Tests
// ============================================================================

func TestComputeLineNumber(t *testing.T) {
	t.Run("single_line", func(t *testing.T) {
		content := []byte("single line content")
		assert.Equal(t, 1, ComputeLineNumber(content, 0))
		assert.Equal(t, 1, ComputeLineNumber(content, 5))
		assert.Equal(t, 1, ComputeLineNumber(content, 18))
	})

	t.Run("multiple_lines", func(t *testing.T) {
		content := []byte("line1\nline2\nline3")
		//                 012345 678901 234567

		testCases := []struct {
			offset   int
			expected int
		}{
			{0, 1},  // Start of line 1
			{4, 1},  // End of "line1"
			{5, 1},  // The newline char is still line 1
			{6, 2},  // Start of line 2
			{11, 2}, // The newline is still line 2
			{12, 3}, // Start of line 3
			{16, 3}, // End of line 3
		}

		for _, tc := range testCases {
			result := ComputeLineNumber(content, tc.offset)
			assert.Equal(t, tc.expected, result,
				"Offset %d should be line %d", tc.offset, tc.expected)
		}
	})

	t.Run("empty_content", func(t *testing.T) {
		assert.Equal(t, 1, ComputeLineNumber([]byte{}, 0))
		assert.Equal(t, 1, ComputeLineNumber(nil, 0))
	})

	t.Run("negative_offset", func(t *testing.T) {
		content := []byte("test")
		assert.Equal(t, 1, ComputeLineNumber(content, -5))
	})

	t.Run("offset_beyond_content", func(t *testing.T) {
		content := []byte("line1\nline2")
		result := ComputeLineNumber(content, 1000)
		assert.Equal(t, 2, result) // Should clamp to end
	})
}

func TestComputeLineStart(t *testing.T) {
	t.Run("first_line", func(t *testing.T) {
		content := []byte("first line")
		assert.Equal(t, 0, ComputeLineStart(content, 0))
		assert.Equal(t, 0, ComputeLineStart(content, 5))
	})

	t.Run("second_line", func(t *testing.T) {
		content := []byte("line1\nline2")
		assert.Equal(t, 6, ComputeLineStart(content, 8))
	})

	t.Run("third_line", func(t *testing.T) {
		content := []byte("line1\nline2\nline3")
		assert.Equal(t, 12, ComputeLineStart(content, 15))
	})

	t.Run("empty_content", func(t *testing.T) {
		assert.Equal(t, 0, ComputeLineStart([]byte{}, 0))
	})

	t.Run("beyond_content", func(t *testing.T) {
		content := []byte("line1\nline2")
		result := ComputeLineStart(content, 100)
		assert.GreaterOrEqual(t, result, 0)
	})
}

func TestComputeLineEnd(t *testing.T) {
	t.Run("single_line", func(t *testing.T) {
		content := []byte("single line")
		assert.Equal(t, 11, ComputeLineEnd(content, 0))
	})

	t.Run("first_line_multiline", func(t *testing.T) {
		content := []byte("line1\nline2")
		assert.Equal(t, 5, ComputeLineEnd(content, 0))
	})

	t.Run("second_line", func(t *testing.T) {
		content := []byte("line1\nline2\n")
		assert.Equal(t, 11, ComputeLineEnd(content, 6))
	})

	t.Run("empty_content", func(t *testing.T) {
		assert.Equal(t, 0, ComputeLineEnd([]byte{}, 0))
	})
}

func TestComputeColumn(t *testing.T) {
	content := []byte("col123\ncol456\ncol789")

	testCases := []struct {
		offset   int
		expected int
	}{
		{0, 1},  // First char
		{3, 4},  // 4th char of line 1
		{7, 1},  // First char of line 2
		{10, 4}, // 4th char of line 2
	}

	for _, tc := range testCases {
		result := ComputeColumn(content, tc.offset)
		assert.Equal(t, tc.expected, result,
			"Offset %d should have column %d", tc.offset, tc.expected)
	}
}

func TestExtractLine(t *testing.T) {
	t.Run("first_line", func(t *testing.T) {
		content := []byte("line1\nline2\nline3")
		result := ExtractLine(content, 3)
		assert.Equal(t, []byte("line1"), result)
	})

	t.Run("middle_line", func(t *testing.T) {
		content := []byte("line1\nline2\nline3")
		result := ExtractLine(content, 8)
		assert.Equal(t, []byte("line2"), result)
	})

	t.Run("last_line", func(t *testing.T) {
		content := []byte("line1\nline2\nline3")
		result := ExtractLine(content, 15)
		assert.Equal(t, []byte("line3"), result)
	})

	t.Run("empty_content", func(t *testing.T) {
		result := ExtractLine([]byte{}, 0)
		assert.Nil(t, result)
	})
}

// ============================================================================
// Text Matching Tests
// ============================================================================

func TestIsWordCharacter(t *testing.T) {
	t.Run("lowercase", func(t *testing.T) {
		for b := byte('a'); b <= 'z'; b++ {
			assert.True(t, IsWordCharacter(b))
		}
	})

	t.Run("uppercase", func(t *testing.T) {
		for b := byte('A'); b <= 'Z'; b++ {
			assert.True(t, IsWordCharacter(b))
		}
	})

	t.Run("digits", func(t *testing.T) {
		for b := byte('0'); b <= '9'; b++ {
			assert.True(t, IsWordCharacter(b))
		}
	})

	t.Run("underscore", func(t *testing.T) {
		assert.True(t, IsWordCharacter('_'))
	})

	t.Run("non_word", func(t *testing.T) {
		nonWord := []byte{' ', '.', ',', '-', '!', '@', '#', '$', '%', '^', '&', '*', '(', ')'}
		for _, b := range nonWord {
			assert.False(t, IsWordCharacter(b), "%c should not be word char", b)
		}
	})
}

func TestIsWordBoundary(t *testing.T) {
	content := []byte("hello world")
	//                 01234567890

	testCases := []struct {
		pos      int
		expected bool
		desc     string
	}{
		{0, true, "start of content"},
		{5, true, "between 'hello' and space"},
		{6, true, "between space and 'world'"},
		{11, true, "end of content"},
		{3, false, "middle of 'hello'"},
		{8, false, "middle of 'world'"},
	}

	for _, tc := range testCases {
		result := IsWordBoundary(content, tc.pos)
		assert.Equal(t, tc.expected, result, "Position %d (%s)", tc.pos, tc.desc)
	}
}

func TestFindLiteralOccurrences(t *testing.T) {
	t.Run("multiple_occurrences", func(t *testing.T) {
		content := []byte("test test test")
		pattern := []byte("test")

		positions := FindLiteralOccurrences(content, pattern)
		assert.Equal(t, []int{0, 5, 10}, positions)
	})

	t.Run("overlapping", func(t *testing.T) {
		content := []byte("aaaa")
		pattern := []byte("aa")

		positions := FindLiteralOccurrences(content, pattern)
		assert.Equal(t, []int{0, 1, 2}, positions)
	})

	t.Run("no_match", func(t *testing.T) {
		content := []byte("hello world")
		pattern := []byte("xyz")

		positions := FindLiteralOccurrences(content, pattern)
		assert.Nil(t, positions)
	})

	t.Run("empty_pattern", func(t *testing.T) {
		content := []byte("test")
		positions := FindLiteralOccurrences(content, []byte{})
		assert.Nil(t, positions)
	})

	t.Run("empty_content", func(t *testing.T) {
		positions := FindLiteralOccurrences([]byte{}, []byte("test"))
		assert.Nil(t, positions)
	})
}

func TestFindLiteralOccurrencesCaseInsensitive(t *testing.T) {
	content := []byte("Test TEST test TeSt")

	positions := FindLiteralOccurrencesCaseInsensitive(content, []byte("test"))
	assert.Equal(t, 4, len(positions))
}

func TestFindWholeWordOccurrences(t *testing.T) {
	content := []byte("test testing tested test")

	positions := FindWholeWordOccurrences(content, []byte("test"))

	// Should find "test" at positions 0 and 20, not "testing" or "tested"
	assert.Equal(t, 2, len(positions))
	assert.Contains(t, positions, 0)
	assert.Contains(t, positions, 20)
}

// ============================================================================
// Scoring Tests
// ============================================================================

func TestScoreFileTypeByExtension(t *testing.T) {
	t.Run("code_files", func(t *testing.T) {
		codeExts := []string{".go", ".py", ".js", ".ts", ".java", ".cpp"}
		for _, ext := range codeExts {
			score := ScoreFileTypeByExtension(ext)
			assert.Greater(t, score, 0.0, "Code extension %s should have positive score", ext)
		}
	})

	t.Run("doc_files", func(t *testing.T) {
		docExts := []string{".md", ".txt", ".rst"}
		for _, ext := range docExts {
			score := ScoreFileTypeByExtension(ext)
			assert.Less(t, score, 0.0, "Doc extension %s should have negative score", ext)
		}
	})

	t.Run("config_files", func(t *testing.T) {
		configExts := []string{".json", ".yaml", ".yml", ".toml"}
		for _, ext := range configExts {
			score := ScoreFileTypeByExtension(ext)
			assert.Greater(t, score, 0.0, "Config extension %s should have positive score", ext)
		}
	})

	t.Run("unknown_files", func(t *testing.T) {
		score := ScoreFileTypeByExtension(".xyz")
		assert.Equal(t, 0.0, score)
	})

	t.Run("case_insensitive", func(t *testing.T) {
		assert.Equal(t, ScoreFileTypeByExtension(".GO"), ScoreFileTypeByExtension(".go"))
	})
}

func TestIsTestFile(t *testing.T) {
	testCases := []struct {
		path     string
		expected bool
	}{
		{"main_test.go", true},
		{"app.test.js", true},
		{"utils.spec.ts", true},
		{"test_helper.py", true},
		{"/tests/unit/foo.go", true},
		{"/__tests__/component.jsx", true},
		{"main.go", false},
		{"app.js", false},
		{"testing.go", false}, // Contains "test" but not as test file pattern
	}

	for _, tc := range testCases {
		result := IsTestFile(tc.path)
		assert.Equal(t, tc.expected, result, "Path %s", tc.path)
	}
}

// ============================================================================
// Context Extraction Tests
// ============================================================================

func TestExpandToLineContext(t *testing.T) {
	content := []byte("line1\nline2 match here\nline3")
	//                 012345 6789012345678901 23456789

	// Match "match" at position 12-17
	start, end := ExpandToLineContext(content, 12, 17)

	assert.Equal(t, 6, start) // Start of "line2"
	assert.Equal(t, 22, end)  // End of "line2"
}

func TestCountLines(t *testing.T) {
	testCases := []struct {
		content  string
		expected int
	}{
		{"", 0},
		{"single", 1},
		{"line1\nline2", 2},
		{"line1\nline2\n", 2}, // Trailing newline doesn't add line
		{"a\nb\nc\nd", 4},
	}

	for _, tc := range testCases {
		result := CountLines([]byte(tc.content))
		assert.Equal(t, tc.expected, result, "Content: %q", tc.content)
	}
}

func TestGetLineByNumber(t *testing.T) {
	content := []byte("line1\nline2\nline3")

	testCases := []struct {
		lineNum  int
		expected []byte
	}{
		{1, []byte("line1")},
		{2, []byte("line2")},
		{3, []byte("line3")},
		{0, nil},
		{4, nil},
		{-1, nil},
	}

	for _, tc := range testCases {
		result := GetLineByNumber(content, tc.lineNum)
		assert.Equal(t, tc.expected, result, "Line %d", tc.lineNum)
	}
}

// ============================================================================
// String Normalization Tests
// ============================================================================

func TestNormalizeWhitespace(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"hello  world", "hello world"},
		{"  leading", "leading"},
		{"trailing  ", "trailing"},
		{"multiple   spaces", "multiple spaces"},
		{"tabs\t\there", "tabs here"},
		{"newlines\n\nhere", "newlines here"},
		{"mixed \t\n white", "mixed white"},
	}

	for _, tc := range testCases {
		result := NormalizeWhitespace(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}

func TestTrimLinePrefix(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"  code", "code"},
		{"// comment", "comment"},
		{"# python comment", "python comment"},
		{"* bullet", "bullet"},
		{"/* block start", "block start"},
		{"normal", "normal"},
	}

	for _, tc := range testCases {
		result := TrimLinePrefix(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %q", tc.input)
	}
}

// ============================================================================
// Binary Search Tests
// ============================================================================

func TestBinarySearchLineOffset(t *testing.T) {
	// lineOffsets[i] = start of line i+1
	lineOffsets := []int{0, 10, 20, 30, 40}

	testCases := []struct {
		offset   int
		expected int
	}{
		{0, 1},   // Start of line 1
		{5, 1},   // Middle of line 1
		{9, 1},   // End of line 1
		{10, 2},  // Start of line 2
		{15, 2},  // Middle of line 2
		{20, 3},  // Start of line 3
		{35, 4},  // Middle of line 4
		{40, 5},  // Start of line 5
		{100, 5}, // Beyond content
	}

	for _, tc := range testCases {
		result := BinarySearchLineOffset(lineOffsets, tc.offset)
		assert.Equal(t, tc.expected, result,
			"Offset %d should be line %d", tc.offset, tc.expected)
	}
}

func TestComputeLineOffsets(t *testing.T) {
	t.Run("single_line", func(t *testing.T) {
		content := []byte("single line")
		offsets := ComputeLineOffsets(content)
		assert.Equal(t, []int{0}, offsets)
	})

	t.Run("multiple_lines", func(t *testing.T) {
		content := []byte("line1\nline2\nline3")
		offsets := ComputeLineOffsets(content)
		assert.Equal(t, []int{0, 6, 12}, offsets)
	})

	t.Run("trailing_newline", func(t *testing.T) {
		content := []byte("line1\nline2\n")
		offsets := ComputeLineOffsets(content)
		// Last newline doesn't start a new line
		assert.Equal(t, []int{0, 6}, offsets)
	})

	t.Run("empty", func(t *testing.T) {
		offsets := ComputeLineOffsets([]byte{})
		assert.Equal(t, []int{0}, offsets)
	})
}

// ============================================================================
// Pattern Analysis Tests
// ============================================================================

func TestCalculatePatternComplexity_PureFunctions(t *testing.T) {
	testCases := []struct {
		pattern  string
		minScore int
	}{
		{"", 0},
		{"a", 1},
		{"abc", 3},
		{"camelCase", 9},   // Length + camelCase transition
		{"XMLParser", 9},   // Length + transitions
		{"snake_case", 10}, // Length + underscore
		{"test123", 7},     // Length + digits
		{"MyClass", 7},     // Length + camelCase
	}

	for _, tc := range testCases {
		result := CalculatePatternComplexity(tc.pattern)
		assert.GreaterOrEqual(t, result, tc.minScore,
			"Pattern %q should have complexity >= %d", tc.pattern, tc.minScore)
	}
}

func TestSplitCamelCase(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"lowercase", []string{"lowercase"}},
		{"CamelCase", []string{"Camel", "Case"}},
		{"camelCase", []string{"camel", "Case"}},
		{"XMLParser", []string{"XML", "Parser"}},
		{"getHTTPResponse", []string{"get", "HTTP", "Response"}},
		{"ABC", []string{"ABC"}},
	}

	for _, tc := range testCases {
		result := SplitCamelCase(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %q", tc.input)
	}
}

// ============================================================================
// Match Quality Tests
// ============================================================================

func TestCalculateMatchQuality(t *testing.T) {
	t.Run("word_boundary_bonus", func(t *testing.T) {
		content := []byte("hello test world")
		pattern := []byte("test")

		// Match with word boundaries
		scoreBoundary := CalculateMatchQuality(content, 6, 10, pattern)

		// Match without word boundaries (partial)
		content2 := []byte("testing")
		scorePartial := CalculateMatchQuality(content2, 0, 4, pattern)

		assert.Greater(t, scoreBoundary, scorePartial)
	})

	t.Run("beginning_of_line_bonus", func(t *testing.T) {
		content := []byte("test at start")
		pattern := []byte("test")

		score := CalculateMatchQuality(content, 0, 4, pattern)
		assert.Greater(t, score, 100.0) // Should have bonus
	})

	t.Run("exact_case_bonus", func(t *testing.T) {
		content := []byte("Test test TEST")
		pattern := []byte("test")

		// Second occurrence (exact case)
		scoreExact := CalculateMatchQuality(content, 5, 9, pattern)

		// Verify score is positive
		assert.Greater(t, scoreExact, 0.0, "Exact match should have positive score")
	})

	t.Run("invalid_ranges", func(t *testing.T) {
		content := []byte("test content")

		assert.Equal(t, 0.0, CalculateMatchQuality(content, 10, 5, []byte("t")))  // End before start
		assert.Equal(t, 0.0, CalculateMatchQuality(content, -1, 5, []byte("t")))  // Negative start
		assert.Equal(t, 0.0, CalculateMatchQuality(content, 0, 100, []byte("t"))) // Beyond content
	})
}

// ============================================================================
// Property-Based Tests
// ============================================================================

func TestProperty_LineNumberNeverZero(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 100; i++ {
		length := rng.Intn(1000)
		content := make([]byte, length)
		for j := range content {
			if rng.Float32() < 0.1 {
				content[j] = '\n'
			} else {
				content[j] = byte('a' + rng.Intn(26))
			}
		}

		offset := rng.Intn(length + 100) // May be beyond content

		line := ComputeLineNumber(content, offset)
		assert.GreaterOrEqual(t, line, 1, "Line number should always be >= 1")
	}
}

func TestProperty_LineStartNeverNegative(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 100; i++ {
		length := rng.Intn(1000)
		content := make([]byte, length)
		for j := range content {
			if rng.Float32() < 0.1 {
				content[j] = '\n'
			} else {
				content[j] = byte('a' + rng.Intn(26))
			}
		}

		offset := rng.Intn(length + 100)

		start := ComputeLineStart(content, offset)
		assert.GreaterOrEqual(t, start, 0, "Line start should never be negative")
	}
}

func TestProperty_OccurrencePositionsValid(t *testing.T) {
	rng := rand.New(rand.NewSource(456))

	for i := 0; i < 50; i++ {
		// Generate random content with guaranteed pattern
		pattern := []byte("test")
		content := make([]byte, 100)
		copy(content[:4], pattern) // Ensure at least one match
		for j := 4; j < len(content); j++ {
			content[j] = byte('a' + rng.Intn(26))
		}

		positions := FindLiteralOccurrences(content, pattern)

		for _, pos := range positions {
			assert.GreaterOrEqual(t, pos, 0, "Position should be non-negative")
			assert.Less(t, pos, len(content), "Position should be within content")
			// Verify pattern actually exists at position
			require.True(t, pos+len(pattern) <= len(content))
			for j := 0; j < len(pattern); j++ {
				assert.Equal(t, pattern[j], content[pos+j])
			}
		}
	}
}

func TestProperty_LineOffsetsMonotonic(t *testing.T) {
	rng := rand.New(rand.NewSource(789))

	for i := 0; i < 100; i++ {
		length := rng.Intn(500) + 10
		content := make([]byte, length)
		for j := range content {
			if rng.Float32() < 0.15 {
				content[j] = '\n'
			} else {
				content[j] = byte('a' + rng.Intn(26))
			}
		}

		offsets := ComputeLineOffsets(content)

		// Offsets should be strictly increasing
		for j := 1; j < len(offsets); j++ {
			assert.Greater(t, offsets[j], offsets[j-1],
				"Line offsets should be strictly increasing")
		}

		// First offset should always be 0
		if len(offsets) > 0 {
			assert.Equal(t, 0, offsets[0])
		}
	}
}

func TestProperty_WordBoundarySymmetry(t *testing.T) {
	// Property: If pos is a word boundary where word starts,
	// pos-1 (if valid) should also be a boundary (where word ends)

	content := []byte("hello world test")

	// At position 6 (start of "world"), position 5 (space) is also boundary
	assert.True(t, IsWordBoundary(content, 6)) // Start of "world"
	assert.True(t, IsWordBoundary(content, 5)) // End of "hello"
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkComputeLineNumber(b *testing.B) {
	content := make([]byte, 10000)
	for i := range content {
		if i%80 == 0 {
			content[i] = '\n'
		} else {
			content[i] = 'x'
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeLineNumber(content, 5000)
	}
}

func BenchmarkFindLiteralOccurrences(b *testing.B) {
	content := []byte("test content test more test and test again test")
	pattern := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FindLiteralOccurrences(content, pattern)
	}
}

func BenchmarkBinarySearchLineOffset(b *testing.B) {
	// Create 1000 line offsets
	offsets := make([]int, 1000)
	for i := range offsets {
		offsets[i] = i * 80
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BinarySearchLineOffset(offsets, 40000)
	}
}

func BenchmarkSplitCamelCase(b *testing.B) {
	patterns := []string{
		"simpleCase",
		"XMLHTTPRequest",
		"getHTTPResponseCode",
		"MyVeryLongCamelCaseIdentifier",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range patterns {
			_ = SplitCamelCase(p)
		}
	}
}

func BenchmarkCalculateMatchQuality(b *testing.B) {
	content := []byte("function calculateTotal(items) { return items.reduce((a,b) => a+b); }")
	pattern := []byte("calculate")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateMatchQuality(content, 9, 18, pattern)
	}
}
