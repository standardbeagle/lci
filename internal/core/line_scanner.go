package core

import (
	"bytes"
)

// LineScanner provides zero-allocation line iteration over byte content.
// It performs a single-pass scan, computing line offsets on-demand without
// allocating a slice of strings like strings.Split does.
//
// Usage:
//
//	scanner := NewLineScanner(content)
//	for scanner.Scan() {
//	    line := scanner.Bytes() // Zero-copy access to current line
//	    lineNum := scanner.LineNumber() // 1-based line number
//	}
//
// For counting lines before processing:
//
//	count := CountLines(content)
//	results := make([]Result, 0, count) // Pre-allocate with exact capacity
type LineScanner struct {
	data        []byte
	start       int  // Start of current line
	end         int  // End of current line (exclusive, before newline)
	pos         int  // Current position in data
	lineNum     int  // Current line number (1-based)
	done        bool // Whether scanning is complete
	includeCRLF bool // Whether to strip CRLF (default: strip)
}

// NewLineScanner creates a new line scanner for the given content.
// The scanner strips trailing \r\n or \n from each line.
func NewLineScanner(data []byte) *LineScanner {
	return &LineScanner{
		data:    data,
		lineNum: 0,
	}
}

// NewLineScannerKeepNewlines creates a scanner that keeps newline characters.
func NewLineScannerKeepNewlines(data []byte) *LineScanner {
	return &LineScanner{
		data:        data,
		lineNum:     0,
		includeCRLF: true,
	}
}

// Scan advances to the next line. Returns false when done.
func (ls *LineScanner) Scan() bool {
	if ls.done {
		return false
	}

	// Check if we've reached the end
	if ls.pos >= len(ls.data) {
		ls.done = true
		return false
	}

	ls.start = ls.pos
	ls.lineNum++

	// Find end of line
	idx := bytes.IndexByte(ls.data[ls.pos:], '\n')
	if idx < 0 {
		// Last line without trailing newline
		ls.end = len(ls.data)
		ls.pos = len(ls.data)
	} else {
		ls.end = ls.pos + idx
		ls.pos = ls.pos + idx + 1 // Skip past newline
	}

	// Strip trailing \r if present (CRLF handling)
	if !ls.includeCRLF && ls.end > ls.start && ls.data[ls.end-1] == '\r' {
		ls.end--
	}

	return true
}

// Bytes returns the current line as a byte slice (zero-copy).
// The returned slice is valid until the next Scan call.
func (ls *LineScanner) Bytes() []byte {
	if ls.start >= len(ls.data) || ls.end > len(ls.data) {
		return nil
	}
	return ls.data[ls.start:ls.end]
}

// Text returns the current line as a string.
// Note: This allocates a new string. Use Bytes() for zero-allocation access.
func (ls *LineScanner) Text() string {
	return string(ls.Bytes())
}

// LineNumber returns the current line number (1-based).
func (ls *LineScanner) LineNumber() int {
	return ls.lineNum
}

// Offset returns the byte offset of the current line start.
func (ls *LineScanner) Offset() int {
	return ls.start
}

// EndOffset returns the byte offset of the current line end (exclusive).
func (ls *LineScanner) EndOffset() int {
	return ls.end
}

// Length returns the length of the current line in bytes.
func (ls *LineScanner) Length() int {
	return ls.end - ls.start
}

// Reset resets the scanner to the beginning.
func (ls *LineScanner) Reset() {
	ls.start = 0
	ls.end = 0
	ls.pos = 0
	ls.lineNum = 0
	ls.done = false
}

// CountLines counts the number of lines in content without allocation.
// This is a fast single-pass count that can be used to pre-allocate slices.
// Uses bytes.Count for optimal performance (SIMD on supported platforms).
func CountLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	// Count newlines using optimized bytes.Count
	newlines := bytes.Count(data, []byte{'\n'})

	// If content doesn't end with newline, there's one more line
	if data[len(data)-1] != '\n' {
		return newlines + 1
	}

	// Content ends with newline - newline count equals line count
	// (unless content is just a newline, then it's 1 empty line)
	if newlines == 0 {
		return 0
	}
	return newlines
}

// CountLinesWithCallback counts lines while calling a callback for each.
// This is useful when you need to both count and process in a single pass.
// The callback receives the line bytes and 1-based line number.
// Returning false from the callback stops iteration.
func CountLinesWithCallback(data []byte, callback func(line []byte, lineNum int) bool) int {
	scanner := NewLineScanner(data)
	for scanner.Scan() {
		if !callback(scanner.Bytes(), scanner.LineNumber()) {
			break
		}
	}
	return scanner.LineNumber()
}

// SplitLinesWithCapacity splits content into lines with pre-allocated capacity.
// This is an optimized replacement for strings.Split(s, "\n") that:
// 1. Counts lines first in a single pass
// 2. Pre-allocates the result slice
// 3. Populates without reallocation
//
// Use this when you need a []string result but want to avoid reallocation.
func SplitLinesWithCapacity(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	count := CountLines(data)
	lines := make([]string, 0, count)

	scanner := NewLineScanner(data)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

// SplitLinesBytesWithCapacity splits content into byte slices with pre-allocated capacity.
// Returns [][]byte where each slice is a view into the original data (zero-copy for bytes).
// Note: The returned byte slices share memory with the input data.
func SplitLinesBytesWithCapacity(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	count := CountLines(data)
	lines := make([][]byte, 0, count)

	scanner := NewLineScanner(data)
	for scanner.Scan() {
		// Make a copy to avoid issues if caller modifies the slice
		line := scanner.Bytes()
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		lines = append(lines, lineCopy)
	}

	return lines
}

// GetLineOffsets returns byte offsets for the start of each line.
// This is compatible with the existing computeLineOffsets but uses LineScanner.
func GetLineOffsets(data []byte) []uint32 {
	if len(data) == 0 {
		return nil
	}

	count := CountLines(data)
	offsets := make([]uint32, 0, count)

	scanner := NewLineScanner(data)
	for scanner.Scan() {
		offsets = append(offsets, uint32(scanner.Offset()))
	}

	return offsets
}

// GetLineAtOffset returns the line number (1-based) for a given byte offset.
// Uses binary search on pre-computed offsets for O(log n) performance.
func GetLineAtOffset(offsets []uint32, byteOffset int) int {
	if len(offsets) == 0 {
		return 0
	}

	target := uint32(byteOffset)

	// Binary search for the largest offset <= target
	l, r := 0, len(offsets)-1
	for l < r {
		m := (l + r + 1) / 2
		if offsets[m] <= target {
			l = m
		} else {
			r = m - 1
		}
	}

	return l + 1 // Convert to 1-based line number
}

// ForEachLine iterates over lines calling the callback for each.
// This is a convenience function for common iteration patterns.
// Returning false from the callback stops iteration.
func ForEachLine(data []byte, callback func(line []byte, lineNum int) bool) {
	scanner := NewLineScanner(data)
	for scanner.Scan() {
		if !callback(scanner.Bytes(), scanner.LineNumber()) {
			return
		}
	}
}

// FindLine finds the first line matching a predicate.
// Returns the line bytes, line number (1-based), and ok=true if found.
func FindLine(data []byte, predicate func(line []byte) bool) ([]byte, int, bool) {
	scanner := NewLineScanner(data)
	for scanner.Scan() {
		if predicate(scanner.Bytes()) {
			return scanner.Bytes(), scanner.LineNumber(), true
		}
	}
	return nil, 0, false
}

// FindLineContaining finds the first line containing the given substring.
// Returns the line bytes, line number (1-based), and ok=true if found.
func FindLineContaining(data []byte, substr []byte) ([]byte, int, bool) {
	return FindLine(data, func(line []byte) bool {
		return bytes.Contains(line, substr)
	})
}

// CountLinesMatching counts lines that match a predicate.
func CountLinesMatching(data []byte, predicate func(line []byte) bool) int {
	count := 0
	scanner := NewLineScanner(data)
	for scanner.Scan() {
		if predicate(scanner.Bytes()) {
			count++
		}
	}
	return count
}

// GetLineRange returns lines from start to end (inclusive, 1-based).
// Pre-allocates the result slice for efficiency.
func GetLineRange(data []byte, start, end int) []string {
	if start < 1 {
		start = 1
	}
	if end < start {
		return nil
	}

	// Estimate capacity
	capacity := end - start + 1
	if capacity > 100 {
		capacity = 100 // Cap to avoid over-allocation for large ranges
	}

	lines := make([]string, 0, capacity)
	scanner := NewLineScanner(data)
	for scanner.Scan() {
		lineNum := scanner.LineNumber()
		if lineNum > end {
			break
		}
		if lineNum >= start {
			lines = append(lines, scanner.Text())
		}
	}

	return lines
}

// GetLineBytesRange returns lines as byte slices from start to end (inclusive, 1-based).
// Each returned slice is a view into the original data (zero-copy).
func GetLineBytesRange(data []byte, start, end int) [][]byte {
	if start < 1 {
		start = 1
	}
	if end < start {
		return nil
	}

	capacity := end - start + 1
	if capacity > 100 {
		capacity = 100
	}

	lines := make([][]byte, 0, capacity)
	scanner := NewLineScanner(data)
	for scanner.Scan() {
		lineNum := scanner.LineNumber()
		if lineNum > end {
			break
		}
		if lineNum >= start {
			lines = append(lines, scanner.Bytes())
		}
	}

	return lines
}
