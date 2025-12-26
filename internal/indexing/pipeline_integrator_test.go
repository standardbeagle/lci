package indexing

import (
	"bytes"
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// Test calculateSimpleMetrics with edge cases for line counting
// Verifies that files without trailing newlines are handled correctly
func TestCalculateSimpleMetricsLineCounting(t *testing.T) {
	// Create a minimal FileIntegrator for testing
	tracker := core.NewReferenceTrackerForTest()
	integrator := NewFileIntegrator(
		core.NewTrigramIndex(),
		core.NewSymbolIndex(),
		tracker,
	)

	tests := []struct {
		name     string
		content  []byte
		expected int // Expected total line count
	}{
		{
			name:     "Empty file",
			content:  []byte(""),
			expected: 0,
		},
		{
			name:     "Single line without newline",
			content:  []byte("line1"),
			expected: 1,
		},
		{
			name:     "Single line with newline",
			content:  []byte("line1\n"),
			expected: 1,
		},
		{
			name:     "Two lines without trailing newline",
			content:  []byte("line1\nline2"),
			expected: 2,
		},
		{
			name:     "Two lines with trailing newline",
			content:  []byte("line1\nline2\n"),
			expected: 2,
		},
		{
			name:     "Three lines without trailing newline",
			content:  []byte("line1\nline2\nline3"),
			expected: 3,
		},
		{
			name:     "Three lines with trailing newline",
			content:  []byte("line1\nline2\nline3\n"),
			expected: 3,
		},
		{
			name:     "Multiple lines mixed",
			content:  []byte("line1\nline2\nline3\nline4\nline5"),
			expected: 5,
		},
		{
			name:     "File with only newlines",
			content:  []byte("\n\n\n"),
			expected: 3,
		},
		{
			name:     "File with empty lines between content",
			content:  []byte("line1\n\nline3\n"),
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple symbol that spans lines 1-5
			// This avoids the default +10 expansion in calculateSimpleMetrics
			symbol := &types.EnhancedSymbol{
				Symbol: types.Symbol{
					Name:      "testFunc",
					Line:      1,
					EndLine:   5,
					Column:    0,
					EndColumn: 10,
				},
			}

			// Calculate metrics
			metrics := integrator.calculateSimpleMetrics(symbol, tt.content)

			if metrics == nil {
				t.Fatal("calculateSimpleMetrics returned nil")
			}

			// Extract lines_of_code from metrics
			linesOfCode, ok := metrics["lines_of_code"].(int)
			if !ok {
				t.Errorf("lines_of_code is not an int: %v", metrics["lines_of_code"])
			}

			// The function returns: symbolEndLine - symbolStartLine + 1
			// But capped at: min(symbolEndLine - symbolStartLine + 1, totalLines)
			// For our test symbol (Line: 1, EndLine: 5), linesOfCode = 5
			// This gets capped by the file's total lines
			expectedLinesOfCode := tt.expected
			if expectedLinesOfCode > 5 {
				expectedLinesOfCode = 5
			}

			if linesOfCode != expectedLinesOfCode {
				t.Errorf("Expected lines_of_code %d, got %d (totalLines in file: %d)",
					expectedLinesOfCode, linesOfCode, tt.expected)
			}
		})
	}
}

// Benchmark calculateSimpleMetrics to verify performance
func BenchmarkCalculateSimpleMetrics(b *testing.B) {
	tracker := core.NewReferenceTrackerForTest()
	integrator := NewFileIntegrator(
		core.NewTrigramIndex(),
		core.NewSymbolIndex(),
		tracker,
	)

	// Create a test file with 100 lines
	var content []byte
	for i := 0; i < 100; i++ {
		content = append(content, []byte("line "+string(rune('0'+i%10))+" content\n")...)
	}

	symbol := &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:      "testFunc",
			Line:      50,
			EndLine:   60,
			Column:    0,
			EndColumn: 20,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		integrator.calculateSimpleMetrics(symbol, content)
	}
}

// Test line counting logic directly with bytes.Count edge cases
func TestLineCountingLogic(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected int
	}{
		{
			name:     "Empty content",
			content:  []byte(""),
			expected: 0,
		},
		{
			name:     "No newlines",
			content:  []byte("hello"),
			expected: 1,
		},
		{
			name:     "One newline at end",
			content:  []byte("hello\n"),
			expected: 1,
		},
		{
			name:     "One newline in middle",
			content:  []byte("hello\nworld"),
			expected: 2,
		},
		{
			name:     "Two newlines",
			content:  []byte("line1\nline2\n"),
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Implement the line counting logic from calculateSimpleMetrics
			newlineCount := bytes.Count(tt.content, []byte("\n"))
			var totalLines int
			if len(tt.content) == 0 {
				totalLines = 0
			} else if len(tt.content) > 0 && tt.content[len(tt.content)-1] == '\n' {
				totalLines = newlineCount
			} else {
				totalLines = newlineCount + 1
			}

			if totalLines != tt.expected {
				t.Errorf("Expected %d lines, got %d (content: %q)",
					tt.expected, totalLines, string(tt.content))
			}
		})
	}
}
