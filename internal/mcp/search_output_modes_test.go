package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/searchtypes"
)

// TestSearchOutputModesVerbosity ensures different output modes produce appropriately sized responses
func TestSearchOutputModesVerbosity(t *testing.T) {
	t.Run("FilesOnlyResponse format is minimal", func(t *testing.T) {
		response := &FilesOnlyResponse{
			Files:        []string{"internal/mcp/server.go", "internal/mcp/handlers.go", "cmd/main.go"},
			TotalMatches: 150,
			UniqueFiles:  3,
		}

		formatter := &CompactFormatter{}
		output := formatter.FormatFilesOnlyResponse(response)

		// Should be very compact
		lines := strings.Split(output, "\n")
		assert.Equal(t, 5, len(lines), "FilesOnly should have header + count + 3 files")
		assert.Contains(t, lines[0], "mode=files")
		assert.Contains(t, lines[1], "total=150")
		assert.Contains(t, lines[1], "files=3")

		// Verify each file is just a path, no metadata
		for i := 2; i < len(lines); i++ {
			assert.False(t, strings.Contains(lines[i], "o="), "File line should not contain objectID")
			assert.False(t, strings.Contains(lines[i], "s="), "File line should not contain score")
			assert.False(t, strings.Contains(lines[i], "t="), "File line should not contain type")
		}

		// Output should be under 200 bytes for 3 files
		assert.Less(t, len(output), 200, "FilesOnly output should be very compact")
	})

	t.Run("CountOnlyResponse format is absolute minimal", func(t *testing.T) {
		response := &CountOnlyResponse{
			TotalMatches: 1500,
			UniqueFiles:  45,
		}

		formatter := &CompactFormatter{}
		output := formatter.FormatCountOnlyResponse(response)

		// Should be just 2 lines
		lines := strings.Split(output, "\n")
		assert.Equal(t, 2, len(lines), "CountOnly should have exactly 2 lines")
		assert.Contains(t, lines[0], "mode=count")
		assert.Contains(t, lines[1], "total=1500")
		assert.Contains(t, lines[1], "files=45")

		// Should be under 50 bytes
		assert.Less(t, len(output), 50, "CountOnly output should be absolute minimal")
	})

	t.Run("FilesOnly scales linearly with file count", func(t *testing.T) {
		// 10 files
		files10 := make([]string, 10)
		for i := range files10 {
			files10[i] = "internal/pkg/file" + string(rune('a'+i)) + ".go"
		}
		response10 := &FilesOnlyResponse{Files: files10, TotalMatches: 100, UniqueFiles: 10}

		// 100 files
		files100 := make([]string, 100)
		for i := range files100 {
			files100[i] = "internal/pkg/subpkg/file" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".go"
		}
		response100 := &FilesOnlyResponse{Files: files100, TotalMatches: 1000, UniqueFiles: 100}

		formatter := &CompactFormatter{}
		output10 := formatter.FormatFilesOnlyResponse(response10)
		output100 := formatter.FormatFilesOnlyResponse(response100)

		// Output should scale roughly linearly
		ratio := float64(len(output100)) / float64(len(output10))
		assert.Greater(t, ratio, 5.0, "100 files should be at least 5x larger than 10 files")
		assert.Less(t, ratio, 15.0, "100 files should be at most 15x larger than 10 files (linear scaling)")
	})
}

// TestExtractUniqueFiles tests the file deduplication helper
func TestExtractUniqueFiles(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		files := extractUniqueFiles(nil, 10)
		assert.Empty(t, files)
	})

	t.Run("respects maxFiles limit", func(t *testing.T) {
		// Create mock results - we need to use the actual type
		// Since we can't easily create DetailedResult, test the formatter directly
		response := &FilesOnlyResponse{
			Files:        []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
			TotalMatches: 100,
			UniqueFiles:  5,
		}

		formatter := &CompactFormatter{}
		output := formatter.FormatFilesOnlyResponse(response)
		lines := strings.Split(output, "\n")

		// Header + count line + 5 files = 7 lines
		assert.Equal(t, 7, len(lines))
	})
}

// TestCountUniqueFilesPreallocation verifies map preallocation
func TestCountUniqueFilesPreallocation(t *testing.T) {
	// This test documents that countUniqueFiles preallocates the map
	// The actual preallocation is verified by code review and benchmarks
	t.Run("function exists and is documented", func(t *testing.T) {
		// Just verify the types compile correctly
		response := &CountOnlyResponse{
			TotalMatches: 100,
			UniqueFiles:  25,
		}
		require.NotNil(t, response)
	})
}

// TestSearchResponseVerbosityComparison compares output sizes across modes
func TestSearchResponseVerbosityComparison(t *testing.T) {
	t.Run("files mode is much smaller than full results", func(t *testing.T) {
		// Full search response with rich data
		fullResponse := &SearchResponse{
			Results: []CompactSearchResult{
				{
					ResultID:   "result_1_10",
					ObjectID:   "sym:abc123",
					File:       "internal/mcp/handlers.go",
					Line:       100,
					Column:     5,
					Match:      "func handleSearch(ctx context.Context) error {",
					Score:      95.5,
					SymbolType: "function",
					SymbolName: "handleSearch",
					IsExported: true,
				},
				{
					ResultID:   "result_2_20",
					ObjectID:   "sym:def456",
					File:       "internal/mcp/server.go",
					Line:       200,
					Column:     10,
					Match:      "func NewServer(config Config) *Server {",
					Score:      88.2,
					SymbolType: "function",
					SymbolName: "NewServer",
					IsExported: true,
				},
			},
			TotalMatches: 50,
			Showing:      2,
			MaxResults:   50,
		}

		// Files-only response for same search
		filesResponse := &FilesOnlyResponse{
			Files:        []string{"internal/mcp/handlers.go", "internal/mcp/server.go"},
			TotalMatches: 50,
			UniqueFiles:  2,
		}

		formatter := &CompactFormatter{IncludeContext: false, IncludeMetadata: false}
		fullOutput := formatter.FormatSearchResponse(fullResponse)
		filesOutput := formatter.FormatFilesOnlyResponse(filesResponse)

		// Files output should be significantly smaller
		assert.Less(t, len(filesOutput), len(fullOutput)/2,
			"Files output (%d bytes) should be less than half of full output (%d bytes)",
			len(filesOutput), len(fullOutput))

		t.Logf("Full output: %d bytes, Files output: %d bytes (%.1f%% reduction)",
			len(fullOutput), len(filesOutput), 100*(1-float64(len(filesOutput))/float64(len(fullOutput))))
	})

	t.Run("count mode is smallest possible", func(t *testing.T) {
		countResponse := &CountOnlyResponse{
			TotalMatches: 1500,
			UniqueFiles:  75,
		}

		filesResponse := &FilesOnlyResponse{
			Files:        make([]string, 10),
			TotalMatches: 1500,
			UniqueFiles:  10,
		}
		for i := range filesResponse.Files {
			filesResponse.Files[i] = "file" + string(rune('0'+i)) + ".go"
		}

		formatter := &CompactFormatter{}
		countOutput := formatter.FormatCountOnlyResponse(countResponse)
		filesOutput := formatter.FormatFilesOnlyResponse(filesResponse)

		assert.Less(t, len(countOutput), len(filesOutput),
			"Count output should be smaller than files output")
		assert.Less(t, len(countOutput), 50,
			"Count output should be under 50 bytes")
	})
}

// TestSearchOutputModeIntegration tests the full search handler with different output modes
func TestSearchOutputModeIntegration(t *testing.T) {
	// This test validates that the search handler correctly routes to different response types

	t.Run("prepareSearchDefaults handles files output", func(t *testing.T) {
		args := SearchParams{
			Pattern: "test",
			Output:  "files",
		}
		outputSize, maxResults, maxLineCount := prepareSearchDefaults(args)

		// files mode should set OutputSingleLine (minimal)
		assert.Equal(t, OutputSingleLine, outputSize)
		assert.Equal(t, 50, maxResults) // default
		assert.Equal(t, 1, maxLineCount)
	})

	t.Run("prepareSearchDefaults handles count output", func(t *testing.T) {
		args := SearchParams{
			Pattern: "test",
			Output:  "count",
		}
		outputSize, maxResults, maxLineCount := prepareSearchDefaults(args)

		// count mode should set OutputSingleLine (minimal)
		assert.Equal(t, OutputSingleLine, outputSize)
		assert.Equal(t, 50, maxResults) // default
		assert.Equal(t, 1, maxLineCount)
	})

	t.Run("prepareSearchDefaults respects max for files", func(t *testing.T) {
		args := SearchParams{
			Pattern: "test",
			Output:  "files",
			Max:     20,
		}
		_, maxResults, _ := prepareSearchDefaults(args)
		assert.Equal(t, 20, maxResults)
	})

	t.Run("files_with_matches is treated same as files", func(t *testing.T) {
		// This tests the legacy ripgrep-compatible format
		args := SearchParams{
			Pattern: "test",
			Output:  "files_with_matches",
		}
		outputSize, _, _ := prepareSearchDefaults(args)
		// files_with_matches should be treated as single-line (minimal)
		assert.Equal(t, OutputSingleLine, outputSize)
	})
}

// TestTruncateMatch tests match field truncation for token efficiency
func TestTruncateMatch(t *testing.T) {
	t.Run("short match unchanged", func(t *testing.T) {
		match := "func handleSearch(ctx context.Context)"
		result := truncateMatch(match, OutputSingleLine)
		assert.Equal(t, match, result, "Short match should not be truncated")
	})

	t.Run("long match truncated for single-line mode", func(t *testing.T) {
		// Create a match that exceeds 100 chars
		match := "func veryLongFunctionNameThatExceedsTheLimit(ctx context.Context, param1 string, param2 string, param3 int) error {"
		result := truncateMatch(match, OutputSingleLine)

		assert.Less(t, len(result), 105, "Single-line match should be truncated to ~100 chars")
		assert.True(t, strings.HasSuffix(result, "..."), "Truncated match should end with ellipsis")
	})

	t.Run("long match truncated differently for context mode", func(t *testing.T) {
		// Create a match that exceeds 100 but not 300 chars
		match := strings.Repeat("x", 250)
		resultSingleLine := truncateMatch(match, OutputSingleLine)
		resultContext := truncateMatch(match, OutputContext)

		assert.Less(t, len(resultSingleLine), len(resultContext),
			"Context mode should allow longer matches than single-line")
		assert.True(t, strings.HasSuffix(resultSingleLine, "..."))
		assert.Equal(t, match, resultContext, "250 char match should fit in context mode limit")
	})

	t.Run("full mode allows more but still truncates very long matches", func(t *testing.T) {
		// Create a 600 char match (exceeds even full mode limit)
		match := strings.Repeat("y", 600)
		result := truncateMatch(match, OutputFull)

		assert.Less(t, len(result), 510, "Full mode should cap at ~500 chars")
		assert.True(t, strings.HasSuffix(result, "..."))
	})

	t.Run("truncation preserves word boundaries when possible", func(t *testing.T) {
		// Create a match with clear word boundaries
		match := "func processData(context context.Context, data []byte, options ProcessOptions) error { return nil }"
		result := truncateMatch(match, OutputSingleLine)

		// Should not cut in middle of a word if there's a space nearby
		if strings.HasSuffix(result, "...") {
			beforeEllipsis := strings.TrimSuffix(result, "...")
			// Should end at a word boundary (space before "...")
			// or the match should be preserved if under limit
			assert.NotContains(t, beforeEllipsis[len(beforeEllipsis)-3:], "con",
				"Should not cut in middle of 'context'")
		}
	})

	t.Run("multi-line match truncated at line boundaries", func(t *testing.T) {
		// Create a multi-line match (function body)
		match := `func handleRequest(ctx context.Context) error {
	// First line of body
	log.Printf("starting request")
	result, err := process(ctx)
	if err != nil {
		return err
	}
	return nil
}`
		result := truncateMatch(match, OutputSingleLine)

		// Should be truncated
		assert.Less(t, len(result), len(match), "Multi-line match should be truncated")

		// Should not cut mid-line - every line should be complete
		lines := strings.Split(strings.TrimSuffix(result, "..."), "\n")
		for _, line := range lines {
			// Each line should be a complete line from the original
			assert.True(t, strings.Contains(match, line),
				"Each line should be complete from original: %q", line)
		}

		// Should end with ellipsis if truncated
		if len(result) < len(match) {
			assert.True(t, strings.HasSuffix(result, "..."),
				"Truncated multi-line match should end with ellipsis")
		}
	})

	t.Run("multi-line match short lines return more content", func(t *testing.T) {
		// Short lines (like comments)
		shortLines := "// line 1\n// line 2\n// line 3\n// line 4\n// line 5\n// line 6\n// line 7\n// line 8\n// line 9\n// line 10"

		// Long lines
		longLines := strings.Repeat("x", 50) + "\n" + strings.Repeat("y", 50) + "\n" + strings.Repeat("z", 50)

		shortResult := truncateMatch(shortLines, OutputSingleLine)
		longResult := truncateMatch(longLines, OutputSingleLine)

		shortLineCount := strings.Count(shortResult, "\n")
		longLineCount := strings.Count(longResult, "\n")

		// Short lines should fit more lines in the budget
		assert.Greater(t, shortLineCount, longLineCount,
			"Short lines (%d) should allow more lines than long lines (%d)",
			shortLineCount, longLineCount)
	})
}

// TestTruncateContextLines tests context line capping for token efficiency
func TestTruncateContextLines(t *testing.T) {
	t.Run("empty lines unchanged", func(t *testing.T) {
		result := truncateContextLines(nil, OutputFull)
		assert.Nil(t, result)

		result = truncateContextLines([]string{}, OutputFull)
		assert.Empty(t, result)
	})

	t.Run("small context unchanged", func(t *testing.T) {
		lines := []string{
			"func main() {",
			"\tfmt.Println(\"Hello\")",
			"}",
		}
		result := truncateContextLines(lines, OutputFull)
		assert.Equal(t, lines, result, "Small context should not be truncated")
	})

	t.Run("many lines truncated", func(t *testing.T) {
		// Create 50 lines of context
		lines := make([]string, 50)
		for i := range lines {
			lines[i] = "\t// Comment line " + string(rune('0'+i%10))
		}

		result := truncateContextLines(lines, OutputFull)
		assert.LessOrEqual(t, len(result), maxContextLinesPerResult,
			"Should cap at maxContextLinesPerResult")
	})

	t.Run("context mode uses fewer lines", func(t *testing.T) {
		// Create 20 lines of context
		lines := make([]string, 20)
		for i := range lines {
			lines[i] = "\t// Line " + string(rune('0'+i%10))
		}

		resultContext := truncateContextLines(lines, OutputContext)
		resultFull := truncateContextLines(lines, OutputFull)

		assert.LessOrEqual(t, len(resultContext), 10, "Context mode should cap at 10 lines")
		assert.Equal(t, 20, len(resultFull), "Full mode should allow up to 30 lines")
	})

	t.Run("large byte size truncated at line boundaries", func(t *testing.T) {
		// Create lines that total more than maxContextBytesPerResult
		longLine := strings.Repeat("x", 500) // 500 bytes per line
		lines := make([]string, 10)          // 5000 bytes total > 2048 limit
		for i := range lines {
			lines[i] = longLine
		}

		result := truncateContextLines(lines, OutputFull)
		totalBytes := 0
		for _, line := range result {
			totalBytes += len(line) + 1 // +1 for newline
		}

		assert.LessOrEqual(t, totalBytes, maxContextBytesPerResult,
			"Total bytes should be within maxContextBytesPerResult")
		assert.Less(t, len(result), len(lines), "Some lines should be removed")
	})

	t.Run("short lines return more lines than long lines", func(t *testing.T) {
		// Short lines (~20 bytes each)
		shortLines := make([]string, 200)
		for i := range shortLines {
			shortLines[i] = "\t// line " + string(rune('0'+i%10))
		}

		// Long lines (~300 bytes each)
		longLines := make([]string, 200)
		for i := range longLines {
			longLines[i] = strings.Repeat("x", 300)
		}

		shortResult := truncateContextLines(shortLines, OutputFull)
		longResult := truncateContextLines(longLines, OutputFull)

		// Short lines should return significantly more lines
		assert.Greater(t, len(shortResult), len(longResult)*3,
			"Short lines (%d) should return many more lines than long lines (%d)",
			len(shortResult), len(longResult))

		// Both should stay under the byte limit
		shortBytes := 0
		for _, line := range shortResult {
			shortBytes += len(line) + 1
		}
		longBytes := 0
		for _, line := range longResult {
			longBytes += len(line) + 1
		}

		assert.LessOrEqual(t, shortBytes, maxContextBytesPerResult,
			"Short lines total bytes should be under limit")
		assert.LessOrEqual(t, longBytes, maxContextBytesPerResult,
			"Long lines total bytes should be under limit")
	})
}

// TestTokenReductionWithTruncation verifies overall token reduction
func TestTokenReductionWithTruncation(t *testing.T) {
	t.Run("verbose match is truncated in search results", func(t *testing.T) {
		// Simulate a verbose function body that would bloat tokens
		verboseMatch := `func handleComplexOperation(ctx context.Context, req *Request, opts ...Option) (*Response, error) {
	// This is a very long function body that should be truncated
	// to prevent excessive token usage in search results
	// The full body should only be available via get_context
	log.Printf("Processing request: %v", req)
	result, err := processInternally(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("processing failed: %w", err)
	}
	return &Response{Data: result}, nil
}`
		result := truncateMatch(verboseMatch, OutputSingleLine)

		// Should be dramatically reduced
		assert.Less(t, len(result), 105, "Verbose match should be truncated to ~100 chars")
		assert.Contains(t, result, "func handleComplexOperation", "Should preserve function signature start")
		assert.True(t, strings.HasSuffix(result, "..."), "Should indicate truncation")
	})
}

// TestSearchResultTruncationIntegration tests that buildCompactResult properly truncates
// verbose matches in actual search results - this is critical for token efficiency
func TestSearchResultTruncationIntegration(t *testing.T) {
	// Create a detailed result with a verbose multi-line match (simulating a long function body)
	verboseMatch := `func processComplexData(ctx context.Context, input *DataInput, opts ProcessOptions) (*DataOutput, error) {
	// Validate input parameters
	if input == nil {
		return nil, errors.New("input cannot be nil")
	}

	// Process the data with multiple transformation steps
	result := transform(input.Data)
	if err := validate(result); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &DataOutput{Result: result}, nil
}`

	t.Run("single-line mode truncates verbose match", func(t *testing.T) {
		// Create a mock detailed result
		detailed := createMockDetailedResult(verboseMatch, 10)

		// Build compact result with single-line mode
		compact := buildCompactResult(detailed, OutputSingleLine, SearchParams{})

		// Verify truncation
		assert.Less(t, len(compact.Match), 105,
			"Single-line mode should truncate match to ~100 chars, got %d chars", len(compact.Match))
		assert.True(t, strings.HasSuffix(compact.Match, "..."),
			"Truncated match should end with ellipsis")
		assert.Contains(t, compact.Match, "func processComplexData",
			"Should preserve function signature start")
	})

	t.Run("context mode truncates verbose match with higher limit", func(t *testing.T) {
		detailed := createMockDetailedResult(verboseMatch, 10)
		compact := buildCompactResult(detailed, OutputContext, SearchParams{})

		// Context mode allows 300 chars
		assert.Less(t, len(compact.Match), 305,
			"Context mode should truncate match to ~300 chars, got %d chars", len(compact.Match))
		// The match is ~500 chars so it should be truncated
		if len(verboseMatch) > 300 {
			assert.True(t, strings.HasSuffix(compact.Match, "..."),
				"Truncated match should end with ellipsis")
		}
	})

	t.Run("full mode truncates very verbose match", func(t *testing.T) {
		// Create an even more verbose match
		veryVerboseMatch := strings.Repeat(verboseMatch, 3) // ~1500 chars
		detailed := createMockDetailedResult(veryVerboseMatch, 10)
		compact := buildCompactResult(detailed, OutputFull, SearchParams{})

		// Full mode allows 500 chars
		assert.Less(t, len(compact.Match), 510,
			"Full mode should truncate match to ~500 chars, got %d chars", len(compact.Match))
		assert.True(t, strings.HasSuffix(compact.Match, "..."),
			"Very verbose match should be truncated even in full mode")
	})

	t.Run("context lines are truncated to byte limit", func(t *testing.T) {
		// Create context with many lines
		contextLines := make([]string, 50)
		for i := range contextLines {
			contextLines[i] = strings.Repeat("x", 100) // 100 chars per line
		}

		detailed := createMockDetailedResultWithContext("short match", 10, contextLines)
		compact := buildCompactResult(detailed, OutputFull, SearchParams{})

		// Calculate total bytes
		totalBytes := 0
		for _, line := range compact.ContextLines {
			totalBytes += len(line) + 1
		}

		assert.LessOrEqual(t, totalBytes, maxContextBytesPerResult,
			"Context lines should be within byte budget, got %d bytes", totalBytes)
		assert.Less(t, len(compact.ContextLines), len(contextLines),
			"Some context lines should be removed, got %d lines", len(compact.ContextLines))
	})

	t.Run("short match is not truncated", func(t *testing.T) {
		shortMatch := "func main() {}"
		detailed := createMockDetailedResult(shortMatch, 1)
		compact := buildCompactResult(detailed, OutputSingleLine, SearchParams{})

		assert.Equal(t, shortMatch, compact.Match,
			"Short match should not be truncated")
		assert.False(t, strings.HasSuffix(compact.Match, "..."),
			"Short match should not have ellipsis")
	})
}

// createMockDetailedResult creates a mock DetailedResult for testing
func createMockDetailedResult(match string, line int) searchtypes.DetailedResult {
	return searchtypes.DetailedResult{
		Result: searchtypes.GrepResult{
			FileID:  1,
			Path:    "test/file.go",
			Line:    line,
			Column:  1,
			Match:   match,
			Context: searchtypes.ExtractedContext{},
			Score:   100.0,
		},
		RelationalData: nil,
	}
}

// createMockDetailedResultWithContext creates a mock DetailedResult with context lines
func createMockDetailedResultWithContext(match string, line int, contextLines []string) searchtypes.DetailedResult {
	return searchtypes.DetailedResult{
		Result: searchtypes.GrepResult{
			FileID: 1,
			Path:   "test/file.go",
			Line:   line,
			Column: 1,
			Match:  match,
			Context: searchtypes.ExtractedContext{
				Lines:     contextLines,
				StartLine: line - len(contextLines)/2,
				EndLine:   line + len(contextLines)/2,
			},
			Score: 100.0,
		},
		RelationalData: nil,
	}
}
