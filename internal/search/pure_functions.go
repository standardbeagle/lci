package search

import (
	"bytes"
	"strings"
)

// Pure functions for search operations
// These functions have no side effects and depend only on their inputs,
// making them ideal for property-based testing and mutation testing.

// LineCalculations provides pure functions for line number calculations

// ComputeLineNumber returns the 1-based line number for a byte offset in content.
// This is a pure function that depends only on the content and offset.
func ComputeLineNumber(content []byte, offset int) int {
	if len(content) == 0 {
		return 1
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(content) {
		offset = len(content) - 1
	}
	if offset < 0 {
		return 1
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}

// ComputeLineStart returns the byte offset of the start of the line containing offset.
// This is a pure function.
func ComputeLineStart(content []byte, offset int) int {
	if len(content) == 0 || offset <= 0 {
		return 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	idx := bytes.LastIndexByte(content[:offset], '\n')
	if idx < 0 {
		return 0
	}
	return idx + 1
}

// ComputeLineEnd returns the byte offset of the end of the line containing offset.
// End is the position of the newline or the end of content.
func ComputeLineEnd(content []byte, offset int) int {
	if len(content) == 0 {
		return 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(content) {
		return len(content)
	}
	idx := bytes.IndexByte(content[offset:], '\n')
	if idx < 0 {
		return len(content)
	}
	return offset + idx
}

// ComputeColumn returns the 1-based column number for a byte offset within its line.
func ComputeColumn(content []byte, offset int) int {
	lineStart := ComputeLineStart(content, offset)
	return offset - lineStart + 1
}

// ExtractLine returns the content of the line containing the given offset.
func ExtractLine(content []byte, offset int) []byte {
	if len(content) == 0 {
		return nil
	}
	start := ComputeLineStart(content, offset)
	end := ComputeLineEnd(content, offset)
	if start > end || start >= len(content) {
		return nil
	}
	return content[start:end]
}

// TextMatching provides pure functions for text pattern matching

// IsWordCharacter returns true if the byte is a word character (alphanumeric or underscore).
func IsWordCharacter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// IsWordBoundary checks if there's a word boundary at the given position.
// A word boundary exists where a word character meets a non-word character.
func IsWordBoundary(content []byte, pos int) bool {
	if pos < 0 || pos > len(content) {
		return true // Start/end of content is always a boundary
	}

	var prevIsWord, currIsWord bool

	if pos > 0 {
		prevIsWord = IsWordCharacter(content[pos-1])
	} else {
		prevIsWord = false // Start of content
	}

	if pos < len(content) {
		currIsWord = IsWordCharacter(content[pos])
	} else {
		currIsWord = false // End of content
	}

	return prevIsWord != currIsWord
}

// FindLiteralOccurrences finds all occurrences of pattern in content.
// Returns a slice of start offsets. This is a pure function.
func FindLiteralOccurrences(content, pattern []byte) []int {
	if len(pattern) == 0 || len(content) == 0 || len(pattern) > len(content) {
		return nil
	}

	var positions []int
	offset := 0

	for {
		idx := bytes.Index(content[offset:], pattern)
		if idx < 0 {
			break
		}
		positions = append(positions, offset+idx)
		offset = offset + idx + 1 // Allow overlapping matches
	}

	return positions
}

// FindLiteralOccurrencesCaseInsensitive finds case-insensitive occurrences.
func FindLiteralOccurrencesCaseInsensitive(content, pattern []byte) []int {
	if len(pattern) == 0 || len(content) == 0 {
		return nil
	}

	lower := bytes.ToLower(content)
	lowerPattern := bytes.ToLower(pattern)

	return FindLiteralOccurrences(lower, lowerPattern)
}

// FindWholeWordOccurrences finds occurrences where pattern appears as a whole word.
func FindWholeWordOccurrences(content, pattern []byte) []int {
	positions := FindLiteralOccurrences(content, pattern)

	var wholeWords []int
	for _, pos := range positions {
		// Check for word boundaries at start and end
		if IsWordBoundary(content, pos) && IsWordBoundary(content, pos+len(pattern)) {
			wholeWords = append(wholeWords, pos)
		}
	}

	return wholeWords
}

// Scoring functions for search ranking

// ScoreFileTypeByExtension returns a score adjustment based on file extension.
// Positive for code files, negative for docs, small positive for config.
func ScoreFileTypeByExtension(ext string) float64 {
	ext = strings.ToLower(ext)

	// Code files get boost
	codeExts := map[string]bool{
		".go": true, ".rs": true, ".py": true, ".js": true, ".jsx": true,
		".ts": true, ".tsx": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".cs": true, ".php": true, ".rb": true,
	}
	if codeExts[ext] {
		return 50.0
	}

	// Documentation files get penalty
	docExts := map[string]bool{
		".md": true, ".txt": true, ".rst": true, ".adoc": true,
	}
	if docExts[ext] {
		return -20.0
	}

	// Config files get small boost
	configExts := map[string]bool{
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".kdl": true,
	}
	if configExts[ext] {
		return 10.0
	}

	return 0.0
}

// IsTestFile determines if a file path appears to be a test file.
func IsTestFile(path string) bool {
	lower := strings.ToLower(path)

	// Common test file patterns
	patterns := []string{
		"_test.", ".test.", ".spec.", "test_",
	}

	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Check for test directories
	testDirs := []string{"/test/", "/tests/", "/__tests__/", "/spec/"}
	for _, dir := range testDirs {
		if strings.Contains(lower, dir) {
			return true
		}
	}

	return false
}

// Context extraction helpers

// ExpandToLineContext expands a match position to include full lines.
// Returns (startOffset, endOffset) of the expanded context.
func ExpandToLineContext(content []byte, matchStart, matchEnd int) (int, int) {
	lineStart := ComputeLineStart(content, matchStart)
	lineEnd := ComputeLineEnd(content, matchEnd)
	return lineStart, lineEnd
}

// CountLines returns the number of lines in content.
func CountLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	count := bytes.Count(content, []byte("\n")) + 1
	// If content ends with newline, don't count empty last line
	if content[len(content)-1] == '\n' {
		count--
	}
	return count
}

// GetLineByNumber returns the content of a specific 1-based line number.
// Returns nil if line number is out of range.
func GetLineByNumber(content []byte, lineNum int) []byte {
	if lineNum < 1 || len(content) == 0 {
		return nil
	}

	currentLine := 1
	start := 0

	for i, b := range content {
		if currentLine == lineNum {
			// Find end of this line
			end := bytes.IndexByte(content[i:], '\n')
			if end < 0 {
				return content[start:]
			}
			return content[start : i+end]
		}
		if b == '\n' {
			currentLine++
			start = i + 1
		}
	}

	// Handle last line without trailing newline
	if currentLine == lineNum {
		return content[start:]
	}

	return nil
}

// String normalization for comparison

// NormalizeWhitespace replaces multiple whitespace chars with single space.
func NormalizeWhitespace(s string) string {
	var result strings.Builder
	inWhitespace := false

	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
		}
	}

	return strings.TrimSpace(result.String())
}

// TrimLinePrefix removes common prefixes from code lines (comments, whitespace).
func TrimLinePrefix(line string) string {
	trimmed := strings.TrimLeft(line, " \t")

	// Remove common comment prefixes
	commentPrefixes := []string{"//", "#", "*", "/*", "*/", "--", ";"}
	for _, prefix := range commentPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			trimmed = strings.TrimPrefix(trimmed, prefix)
			trimmed = strings.TrimLeft(trimmed, " \t")
		}
	}

	return trimmed
}

// Binary search helpers for line offset arrays

// BinarySearchLineOffset finds the line number for an offset using sorted line offsets.
// lineOffsets[i] is the byte offset where line i+1 starts.
// Returns 1-based line number.
func BinarySearchLineOffset(lineOffsets []int, offset int) int {
	if len(lineOffsets) == 0 {
		return 1
	}

	// Binary search for the largest offset <= target
	lo, hi := 0, len(lineOffsets)-1

	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineOffsets[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	return lo + 1 // Convert to 1-based
}

// ComputeLineOffsets creates a slice of line start offsets for content.
// lineOffsets[0] is always 0 (start of line 1).
func ComputeLineOffsets(content []byte) []int {
	if len(content) == 0 {
		return []int{0}
	}

	offsets := []int{0}

	for i, b := range content {
		if b == '\n' && i+1 < len(content) {
			offsets = append(offsets, i+1)
		}
	}

	return offsets
}

// Pattern analysis helpers

// CalculatePatternComplexity returns a complexity score for a pattern.
// Higher complexity suggests more specific searches.
func CalculatePatternComplexity(pattern string) int {
	if len(pattern) == 0 {
		return 0
	}

	complexity := len(pattern)

	// CamelCase adds complexity (more specific)
	for i := 1; i < len(pattern); i++ {
		if pattern[i] >= 'A' && pattern[i] <= 'Z' &&
			pattern[i-1] >= 'a' && pattern[i-1] <= 'z' {
			complexity += 2
		}
	}

	// Underscores suggest specific naming
	complexity += strings.Count(pattern, "_")

	// Numbers suggest specific identifiers
	for _, c := range pattern {
		if c >= '0' && c <= '9' {
			complexity++
		}
	}

	return complexity
}

// SplitCamelCase splits a CamelCase string into words.
func SplitCamelCase(s string) []string {
	if len(s) == 0 {
		return nil
	}

	var words []string
	wordStart := 0

	for i := 1; i < len(s); i++ {
		// Split at lowercase-to-uppercase transition
		if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			words = append(words, s[wordStart:i])
			wordStart = i
		}
		// Split at uppercase followed by lowercase (handles acronyms like XMLParser -> XML, Parser)
		if i > 1 && s[i] >= 'a' && s[i] <= 'z' && s[i-1] >= 'A' && s[i-1] <= 'Z' &&
			s[i-2] >= 'A' && s[i-2] <= 'Z' {
			words = append(words, s[wordStart:i-1])
			wordStart = i - 1
		}
	}

	// Add the last word
	if wordStart < len(s) {
		words = append(words, s[wordStart:])
	}

	return words
}

// Match scoring helpers

// CalculateMatchQuality returns a quality score for a match based on context.
// Higher scores indicate better matches.
func CalculateMatchQuality(content []byte, matchStart, matchEnd int, pattern []byte) float64 {
	if matchEnd <= matchStart || matchStart < 0 || matchEnd > len(content) {
		return 0.0
	}

	score := 100.0 // Base score

	// Exact word boundary bonus
	if IsWordBoundary(content, matchStart) && IsWordBoundary(content, matchEnd) {
		score += 50.0
	}

	// Beginning of line bonus
	lineStart := ComputeLineStart(content, matchStart)
	lineContent := content[lineStart:]
	trimmedStart := lineStart
	for i, b := range lineContent {
		if b != ' ' && b != '\t' {
			trimmedStart = lineStart + i
			break
		}
	}
	if matchStart == trimmedStart {
		score += 25.0
	}

	// Exact case match bonus
	if bytes.Equal(content[matchStart:matchEnd], pattern) {
		score += 20.0
	}

	return score
}
