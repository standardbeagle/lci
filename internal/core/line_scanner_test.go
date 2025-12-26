package core

import (
	"bytes"
	"strings"
	"testing"
)

func TestLineScanner_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single line no newline",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "single line with newline",
			input:    "hello\n",
			expected: []string{"hello"},
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "multiple lines with trailing newline",
			input:    "line1\nline2\nline3\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "CRLF endings",
			input:    "line1\r\nline2\r\nline3\r\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "empty lines",
			input:    "line1\n\nline3\n",
			expected: []string{"line1", "", "line3"},
		},
		{
			name:     "only newlines",
			input:    "\n\n\n",
			expected: []string{"", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewLineScanner([]byte(tt.input))
			var lines []string
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			if tt.expected == nil && lines != nil {
				t.Errorf("expected nil, got %v", lines)
				return
			}

			if len(lines) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d: %v", len(tt.expected), len(lines), lines)
				return
			}

			for i, line := range lines {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i+1, tt.expected[i], line)
				}
			}
		})
	}
}

func TestLineScanner_LineNumber(t *testing.T) {
	content := []byte("first\nsecond\nthird")
	scanner := NewLineScanner(content)

	expected := []struct {
		lineNum int
		text    string
	}{
		{1, "first"},
		{2, "second"},
		{3, "third"},
	}

	i := 0
	for scanner.Scan() {
		if i >= len(expected) {
			t.Fatalf("too many lines, expected %d", len(expected))
		}
		if scanner.LineNumber() != expected[i].lineNum {
			t.Errorf("line %d: expected lineNum %d, got %d", i, expected[i].lineNum, scanner.LineNumber())
		}
		if scanner.Text() != expected[i].text {
			t.Errorf("line %d: expected text %q, got %q", i, expected[i].text, scanner.Text())
		}
		i++
	}

	if i != len(expected) {
		t.Errorf("expected %d lines, got %d", len(expected), i)
	}
}

func TestLineScanner_Offsets(t *testing.T) {
	content := []byte("abc\ndefgh\ni")
	scanner := NewLineScanner(content)

	expected := []struct {
		offset    int
		endOffset int
		length    int
	}{
		{0, 3, 3},   // "abc"
		{4, 9, 5},   // "defgh"
		{10, 11, 1}, // "i"
	}

	i := 0
	for scanner.Scan() {
		if i >= len(expected) {
			t.Fatalf("too many lines")
		}
		if scanner.Offset() != expected[i].offset {
			t.Errorf("line %d: expected offset %d, got %d", i+1, expected[i].offset, scanner.Offset())
		}
		if scanner.EndOffset() != expected[i].endOffset {
			t.Errorf("line %d: expected endOffset %d, got %d", i+1, expected[i].endOffset, scanner.EndOffset())
		}
		if scanner.Length() != expected[i].length {
			t.Errorf("line %d: expected length %d, got %d", i+1, expected[i].length, scanner.Length())
		}
		i++
	}
}

func TestLineScanner_Reset(t *testing.T) {
	content := []byte("a\nb\nc")
	scanner := NewLineScanner(content)

	// Scan all lines
	count := 0
	for scanner.Scan() {
		count++
	}
	if count != 3 {
		t.Fatalf("expected 3 lines, got %d", count)
	}

	// Reset and scan again
	scanner.Reset()
	count = 0
	for scanner.Scan() {
		count++
	}
	if count != 3 {
		t.Fatalf("after reset, expected 3 lines, got %d", count)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines", "a\nb", 2},
		{"two lines trailing newline", "a\nb\n", 2},
		{"three lines", "a\nb\nc", 3},
		{"empty lines", "\n\n", 2},
		{"only newline", "\n", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountLines([]byte(tt.input))
			if got != tt.expected {
				t.Errorf("CountLines(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSplitLinesWithCapacity(t *testing.T) {
	content := "line1\nline2\nline3\n"
	expected := []string{"line1", "line2", "line3"}

	result := SplitLinesWithCapacity([]byte(content))

	if len(result) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(result))
	}

	for i, line := range result {
		if line != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestSplitLinesWithCapacity_MatchesStringsSplit(t *testing.T) {
	testCases := []string{
		"",
		"single",
		"a\nb",
		"a\nb\n",
		"a\nb\nc",
		"\n",
		"\n\n",
		"line1\nline2\nline3",
		"line1\nline2\nline3\n",
	}

	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			// strings.Split behavior
			splitResult := strings.Split(input, "\n")
			// Remove trailing empty string if input ends with newline
			if len(input) > 0 && input[len(input)-1] == '\n' {
				splitResult = splitResult[:len(splitResult)-1]
			}
			if input == "" {
				splitResult = nil
			}

			ourResult := SplitLinesWithCapacity([]byte(input))

			// Handle nil vs empty slice
			if len(splitResult) == 0 && len(ourResult) == 0 {
				return
			}

			if len(ourResult) != len(splitResult) {
				t.Errorf("input %q: got %d lines %v, want %d lines %v",
					input, len(ourResult), ourResult, len(splitResult), splitResult)
				return
			}

			for i := range ourResult {
				if ourResult[i] != splitResult[i] {
					t.Errorf("input %q line %d: got %q, want %q",
						input, i, ourResult[i], splitResult[i])
				}
			}
		})
	}
}

func TestGetLineOffsets(t *testing.T) {
	content := []byte("abc\ndefgh\ni")

	offsets := GetLineOffsets(content)

	expected := []uint32{0, 4, 10}
	if len(offsets) != len(expected) {
		t.Fatalf("expected %d offsets, got %d: %v", len(expected), len(offsets), offsets)
	}

	for i, off := range offsets {
		if off != expected[i] {
			t.Errorf("offset %d: expected %d, got %d", i, expected[i], off)
		}
	}
}

func TestGetLineAtOffset(t *testing.T) {
	offsets := []uint32{0, 4, 10} // Lines at: 0-3, 4-9, 10+

	tests := []struct {
		offset   int
		expected int
	}{
		{0, 1},  // Start of line 1
		{2, 1},  // Middle of line 1
		{3, 1},  // End of line 1
		{4, 2},  // Start of line 2
		{7, 2},  // Middle of line 2
		{10, 3}, // Start of line 3
		{15, 3}, // Beyond content (still line 3)
	}

	for _, tt := range tests {
		got := GetLineAtOffset(offsets, tt.offset)
		if got != tt.expected {
			t.Errorf("GetLineAtOffset(%v, %d) = %d, want %d", offsets, tt.offset, got, tt.expected)
		}
	}
}

func TestForEachLine(t *testing.T) {
	content := []byte("first\nsecond\nthird")
	var collected []string

	ForEachLine(content, func(line []byte, lineNum int) bool {
		collected = append(collected, string(line))
		return true
	})

	expected := []string{"first", "second", "third"}
	if len(collected) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(collected))
	}

	for i, line := range collected {
		if line != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestForEachLine_EarlyStop(t *testing.T) {
	content := []byte("first\nsecond\nthird")
	var collected []string

	ForEachLine(content, func(line []byte, lineNum int) bool {
		collected = append(collected, string(line))
		return lineNum < 2 // Stop after second line
	})

	if len(collected) != 2 {
		t.Errorf("expected 2 lines (early stop), got %d: %v", len(collected), collected)
	}
}

func TestFindLineContaining(t *testing.T) {
	content := []byte("apple\nbanana\ncherry")

	line, lineNum, ok := FindLineContaining(content, []byte("nan"))
	if !ok {
		t.Fatal("expected to find line containing 'nan'")
	}
	if lineNum != 2 {
		t.Errorf("expected line 2, got %d", lineNum)
	}
	if string(line) != "banana" {
		t.Errorf("expected 'banana', got %q", line)
	}

	// Not found
	_, _, ok = FindLineContaining(content, []byte("xyz"))
	if ok {
		t.Error("expected not to find 'xyz'")
	}
}

func TestGetLineRange(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\nline5")

	tests := []struct {
		start    int
		end      int
		expected []string
	}{
		{1, 1, []string{"line1"}},
		{2, 4, []string{"line2", "line3", "line4"}},
		{4, 5, []string{"line4", "line5"}},
		{1, 5, []string{"line1", "line2", "line3", "line4", "line5"}},
		{0, 2, []string{"line1", "line2"}}, // start < 1 normalized to 1
		{6, 7, nil},                        // Beyond content
	}

	for _, tt := range tests {
		result := GetLineRange(content, tt.start, tt.end)
		if len(result) != len(tt.expected) {
			t.Errorf("GetLineRange(%d, %d): expected %d lines, got %d: %v",
				tt.start, tt.end, len(tt.expected), len(result), result)
			continue
		}

		for i, line := range result {
			if line != tt.expected[i] {
				t.Errorf("GetLineRange(%d, %d) line %d: expected %q, got %q",
					tt.start, tt.end, i, tt.expected[i], line)
			}
		}
	}
}

func TestCountLinesMatching(t *testing.T) {
	content := []byte("apple\nbanana\napricot\nblueberry")

	// Count lines starting with 'a'
	count := CountLinesMatching(content, func(line []byte) bool {
		return len(line) > 0 && line[0] == 'a'
	})

	if count != 2 { // apple, apricot
		t.Errorf("expected 2 lines starting with 'a', got %d", count)
	}
}

// Benchmarks

func BenchmarkLineScanner_vs_StringsSplit(b *testing.B) {
	// Generate test content
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString("This is line number ")
		sb.WriteString(strings.Repeat("x", 50))
		sb.WriteString("\n")
	}
	content := sb.String()
	contentBytes := []byte(content)

	b.Run("strings.Split", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			lines := strings.Split(content, "\n")
			_ = lines
		}
	})

	b.Run("LineScanner", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			scanner := NewLineScanner(contentBytes)
			for scanner.Scan() {
				_ = scanner.Bytes()
			}
		}
	})

	b.Run("SplitLinesWithCapacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			lines := SplitLinesWithCapacity(contentBytes)
			_ = lines
		}
	})

	b.Run("CountLines", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			count := CountLines(contentBytes)
			_ = count
		}
	})
}

func BenchmarkLineScanner_LargeFile(b *testing.B) {
	// Simulate a large file (100k lines)
	var sb strings.Builder
	for i := 0; i < 100000; i++ {
		sb.WriteString("func processData(input string) (string, error) {\n")
	}
	contentBytes := []byte(sb.String())

	b.Run("LineScanner_Iteration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			scanner := NewLineScanner(contentBytes)
			for scanner.Scan() {
				_ = scanner.Bytes()
			}
		}
	})

	b.Run("strings.Split", func(b *testing.B) {
		content := sb.String()
		for i := 0; i < b.N; i++ {
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				_ = line
			}
		}
	})
}

func BenchmarkCountLines_vs_BytesCount(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString("test line content here\n")
	}
	contentBytes := []byte(sb.String())

	b.Run("CountLines", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			count := CountLines(contentBytes)
			_ = count
		}
	})

	b.Run("bytes.Count", func(b *testing.B) {
		newline := []byte{'\n'}
		for i := 0; i < b.N; i++ {
			count := bytes.Count(contentBytes, newline)
			_ = count
		}
	})
}
