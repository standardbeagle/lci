package search

import (
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// ZeroAllocContextExtractor provides zero-allocation context extraction operations
// This eliminates string allocations from context extraction functions
type ZeroAllocContextExtractor struct {
	maxLines            int
	defaultContextLines int
	zeroAllocStore       *core.ZeroAllocFileContentStore
}

// NewZeroAllocContextExtractor creates a new zero-allocation context extractor
func NewZeroAllocContextExtractor(fileStore *core.FileContentStore) *ZeroAllocContextExtractor {
	return &ZeroAllocContextExtractor{
		maxLines:            0, // No limit by default
		defaultContextLines: DefaultContextLines,
		zeroAllocStore:       core.NewZeroAllocFileContentStoreFromStore(fileStore),
	}
}

// NewZeroAllocContextExtractorWithConfig creates a context extractor with custom configuration
func NewZeroAllocContextExtractorWithConfig(fileStore *core.FileContentStore, maxLines, defaultContextLines int) *ZeroAllocContextExtractor {
	return &ZeroAllocContextExtractor{
		maxLines:            maxLines,
		defaultContextLines: defaultContextLines,
		zeroAllocStore:       core.NewZeroAllocFileContentStoreFromStore(fileStore),
	}
}

// ZeroAllocExtractedContext represents zero-allocation context extraction results
type ZeroAllocExtractedContext struct {
	MatchLine    int
	BeforeLines  []types.ZeroAllocStringRef
	MatchLineRef types.ZeroAllocStringRef
	AfterLines   []types.ZeroAllocStringRef
	TotalLines   int
}

// ExtractContextZeroAlloc performs zero-allocation context extraction
func (zce *ZeroAllocContextExtractor) ExtractContextZeroAlloc(fileID types.FileID, matchLine int, maxContextLines int) ZeroAllocExtractedContext {
	// Get the matching line as ZeroAllocStringRef
	matchLineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, matchLine-1) // Convert to 0-based
	if matchLineRef.IsEmpty() {
		return ZeroAllocExtractedContext{
			MatchLine:    matchLine,
			BeforeLines:  []types.ZeroAllocStringRef{},
			MatchLineRef: types.EmptyZeroAllocStringRef,
			AfterLines:   []types.ZeroAllocStringRef{},
			TotalLines:   0,
		}
	}

	// Get context lines using zero-alloc operations
	beforeLines := zce.getBeforeLinesZeroAlloc(fileID, matchLine-1, maxContextLines)
	afterLines := zce.getAfterLinesZeroAlloc(fileID, matchLine-1, maxContextLines)

	return ZeroAllocExtractedContext{
		MatchLine:    matchLine,
		BeforeLines:  beforeLines,
		MatchLineRef: matchLineRef,
		AfterLines:   afterLines,
		TotalLines:   len(beforeLines) + 1 + len(afterLines),
	}
}

// ExtractContextWithPaddingZeroAlloc extracts context with intelligent padding
func (zce *ZeroAllocContextExtractor) ExtractContextWithPaddingZeroAlloc(fileID types.FileID, matchLine int, maxContextLines int) ZeroAllocExtractedContext {
	// First try to extract complete function context if possible
	if maxContextLines == 0 {
		return zce.ExtractFunctionContextZeroAlloc(fileID, matchLine)
	}

	// Otherwise, use intelligent padding
	return zce.extractContextWithIntelligentPadding(fileID, matchLine, maxContextLines)
}

// ExtractFunctionContextZeroAlloc extracts complete function context using zero-alloc operations
func (zce *ZeroAllocContextExtractor) ExtractFunctionContextZeroAlloc(fileID types.FileID, matchLine int) ZeroAllocExtractedContext {
	// Find function boundaries using zero-alloc operations
	functionStart, functionEnd := zce.findFunctionBoundariesZeroAlloc(fileID, matchLine-1)

	if functionStart == -1 || functionEnd == -1 {
		// If no function found, fall back to line-based context
		return zce.ExtractContextZeroAlloc(fileID, matchLine, zce.defaultContextLines)
	}

	// Get all lines in the function
	functionCount := functionEnd - functionStart + 1
	functionLines := zce.zeroAllocStore.GetZeroAllocLines(fileID, functionStart, functionCount)

	// Calculate match index within function
	matchIndexInFunction := matchLine - 1 - functionStart

	// Validate bounds
	if matchIndexInFunction < 0 || matchIndexInFunction >= len(functionLines) {
		// Fallback to basic context extraction if bounds are invalid
		return zce.ExtractContextZeroAlloc(fileID, matchLine, zce.defaultContextLines)
	}

	return ZeroAllocExtractedContext{
		MatchLine:    matchLine,
		BeforeLines:  functionLines[:matchIndexInFunction],
		MatchLineRef: functionLines[matchIndexInFunction],
		AfterLines:   functionLines[matchIndexInFunction+1:],
		TotalLines:   len(functionLines),
	}
}

// getBeforeLinesZeroAlloc gets lines before the match line using zero-alloc operations
func (zce *ZeroAllocContextExtractor) getBeforeLinesZeroAlloc(fileID types.FileID, lineIndex int, maxLines int) []types.ZeroAllocStringRef {
	if maxLines <= 0 {
		return []types.ZeroAllocStringRef{}
	}

	start := contextMax(0, lineIndex-maxLines)
	count := lineIndex - start
	lines := zce.zeroAllocStore.GetZeroAllocLines(fileID, start, count)

	// Return in original order (oldest to newest)
	result := make([]types.ZeroAllocStringRef, len(lines))
	for i, line := range lines {
		result[i] = line
	}

	return result
}

// getAfterLinesZeroAlloc gets lines after the match line using zero-alloc operations
func (zce *ZeroAllocContextExtractor) getAfterLinesZeroAlloc(fileID types.FileID, lineIndex int, maxLines int) []types.ZeroAllocStringRef {
	if maxLines <= 0 {
		return []types.ZeroAllocStringRef{}
	}

	start := lineIndex + 1
	totalLines := zce.zeroAllocStore.GetLineCount(fileID)

	// Don't go beyond file end
	end := contextMin(start+maxLines, totalLines)
	if start >= totalLines {
		return []types.ZeroAllocStringRef{}
	}

	count := end - start
	return zce.zeroAllocStore.GetZeroAllocLines(fileID, start, count)
}

// findFunctionBoundariesZeroAlloc finds function start and end using zero-alloc operations
// Uses brace depth tracking to find the actual function end, not just the first closing brace
func (zce *ZeroAllocContextExtractor) findFunctionBoundariesZeroAlloc(fileID types.FileID, lineIndex int) (int, int) {
	totalLines := zce.zeroAllocStore.GetLineCount(fileID)

	// Search backwards for function start
	start := lineIndex
	for start >= 0 {
		lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, start)
		if lineRef.IsEmpty() {
			start--
			continue
		}

		trimmed := lineRef.TrimSpace()
		// Look for function definition patterns
		if trimmed.HasPrefix("func ") || trimmed.HasPrefix("function ") || trimmed.HasPrefix("def ") {
			break
		}
		start--
	}

	// If no function start found, return error
	if start < 0 {
		return -1, -1
	}

	// Search forwards for function end using brace depth tracking
	braceDepth := 0
	functionStarted := false
	end := start

	for end < totalLines {
		lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, end)
		lineStr := lineRef.String()

		// Count braces (simple heuristic - doesn't handle strings/comments perfectly)
		for _, ch := range lineStr {
			if ch == '{' {
				braceDepth++
				functionStarted = true
			} else if ch == '}' {
				braceDepth--
				// When we close the function's opening brace, we found the end
				if functionStarted && braceDepth == 0 {
					return start, end
				}
			}
		}
		end++
	}

	// If we reached here, function wasn't properly closed
	// Return what we have if it's reasonable
	if end > start && end <= totalLines {
		return start, end - 1
	}

	return -1, -1
}

// extractContextWithIntelligentPadding provides intelligent context extraction
func (zce *ZeroAllocContextExtractor) extractContextWithIntelligentPadding(fileID types.FileID, matchLine int, maxContextLines int) ZeroAllocExtractedContext {
	// Try to get complete function first
	functionStart, functionEnd := zce.findFunctionBoundaries(fileID, matchLine-1)

	if functionStart != -1 && functionEnd != -1 {
		// We found a function, try to show as much as possible
		functionLines := functionEnd - functionStart + 1

		if functionLines <= maxContextLines {
			// Show entire function
			return zce.ExtractFunctionContextZeroAlloc(fileID, matchLine)
		} else {
			// Show centered context around match
			padding := maxContextLines / 2
			contextStart := contextMax(functionStart, matchLine-1-padding)
			contextEnd := contextMin(functionEnd, matchLine-1+padding)

			contextCount := contextEnd - contextStart + 1
			contextLines := zce.zeroAllocStore.GetZeroAllocLines(fileID, contextStart, contextCount)

			matchIndex := matchLine-1 - contextStart

			return ZeroAllocExtractedContext{
				MatchLine:    matchLine,
				BeforeLines:  contextLines[:matchIndex],
				MatchLineRef: zce.zeroAllocStore.GetZeroAllocLine(fileID, matchLine-1),
				AfterLines:   contextLines[matchIndex+1:],
				TotalLines:   len(contextLines),
			}
		}
	}

	// Fall back to simple line-based context
	return zce.ExtractContextZeroAlloc(fileID, matchLine, maxContextLines)
}

// findFunctionBoundaries finds function boundaries (regular version)
func (zce *ZeroAllocContextExtractor) findFunctionBoundaries(fileID types.FileID, lineIndex int) (int, int) {
	return zce.findFunctionBoundariesZeroAlloc(fileID, lineIndex)
}

// ExtractBlockContextZeroAlloc extracts block context using zero-alloc operations
func (zce *ZeroAllocContextExtractor) ExtractBlockContextZeroAlloc(fileID types.FileID, matchLine int) ZeroAllocExtractedContext {
	// Find the start of the current block (indentation level)
	lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, matchLine-1)
	if lineRef.IsEmpty() {
		return ZeroAllocExtractedContext{}
	}

	baseIndent := zce.calculateIndentationZeroAlloc(lineRef)

	// Find block boundaries based on indentation
	start := matchLine - 1
	for start >= 0 {
		lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, start)
		if lineRef.IsEmpty() {
			break
		}

		indent := zce.calculateIndentation(lineRef)
		trimmed := lineRef.TrimSpace()
		if indent < baseIndent && !trimmed.IsEmpty() {
			break
		}
		start--
	}

	// Find end of block
	end := matchLine - 1
	totalLines := zce.zeroAllocStore.GetLineCount(fileID)
	for end < totalLines {
		lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, end)
		if lineRef.IsEmpty() {
			break
		}

		indent := zce.calculateIndentation(lineRef)
		trimmed := lineRef.TrimSpace()
		if indent < baseIndent && !trimmed.IsEmpty() {
			break
		}
		end++
	}

	// Extract block lines
	blockCount := end - start - 1
	blockLines := zce.zeroAllocStore.GetZeroAllocLines(fileID, start+1, blockCount)
	matchIndex := matchLine - start - 2 // Adjust for 1-based indexing

	if matchIndex < 0 || matchIndex >= len(blockLines) {
		return ZeroAllocExtractedContext{}
	}

	return ZeroAllocExtractedContext{
		MatchLine:    matchLine,
		BeforeLines:  blockLines[:matchIndex],
		MatchLineRef: blockLines[matchIndex],
		AfterLines:   blockLines[matchIndex+1:],
		TotalLines:   len(blockLines),
	}
}

// calculateIndentation calculates indentation level using zero-alloc operations
func (zce *ZeroAllocContextExtractor) calculateIndentationZeroAlloc(lineRef types.ZeroAllocStringRef) int {
	trimmedLeft := lineRef.TrimLeft(" \t")
	return lineRef.Len() - trimmedLeft.Len()
}

// calculateIndentation calculates indentation level (regular version)
func (zce *ZeroAllocContextExtractor) calculateIndentation(lineRef types.ZeroAllocStringRef) int {
	return zce.calculateIndentationZeroAlloc(lineRef)
}

// IsCommentLineZeroAlloc checks if a line is a comment using zero-alloc operations
// Handles single-line (//, #) and multi-line (/* */, <!-- -->) comments
func (zce *ZeroAllocContextExtractor) IsCommentLineZeroAlloc(lineRef types.ZeroAllocStringRef) bool {
	trimmed := lineRef.TrimSpace()

	// Empty lines are not comments
	if trimmed.IsEmpty() {
		return false
	}

	// Single-line comments
	if trimmed.HasPrefix("//") || trimmed.HasPrefix("#") {
		return true
	}

	// Multi-line comment start
	if trimmed.HasPrefix("/*") {
		return true
	}

	// Multi-line comment end or middle
	if trimmed.HasPrefix("*") || trimmed.HasSuffix("*/") {
		return true
	}

	// HTML comments
	if trimmed.HasPrefix("<!--") || trimmed.HasSuffix("-->") {
		return true
	}

	return false
}

// IsCodeLineZeroAlloc checks if a line contains code using zero-alloc operations
func (zce *ZeroAllocContextExtractor) IsCodeLineZeroAlloc(lineRef types.ZeroAllocStringRef) bool {
	trimmed := lineRef.TrimSpace()

	// Empty lines are not code
	if trimmed.IsEmpty() {
		return false
	}

	// Check if it's not a comment
	return !zce.IsCommentLineZeroAlloc(lineRef)
}

// ExtractSimilarPatternsZeroAlloc finds similar patterns using zero-alloc operations
func (zce *ZeroAllocContextExtractor) ExtractSimilarPatternsZeroAlloc(fileID types.FileID, targetLineRef types.ZeroAllocStringRef, maxPatterns int) []types.ZeroAllocStringRef {
	totalLines := zce.zeroAllocStore.GetLineCount(fileID)
	similarPatterns := make([]types.ZeroAllocStringRef, 0, maxPatterns)

	// Improved similarity check: look for lines with similar structure
	// Use shorter prefix/suffix for better matching
	targetTrimmed := targetLineRef.TrimSpace()

	for i := 0; i < totalLines && len(similarPatterns) < maxPatterns; i++ {
		lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, i)
		if lineRef.IsEmpty() {
			continue
		}

		// Skip the target line itself
		if lineRef.Equal(targetLineRef) {
			continue
		}

		// Check for similar patterns
		trimmed := lineRef.TrimSpace()
		targetStr := targetTrimmed.String()
		trimmedStr := trimmed.String()

		// Similarity check with lower thresholds for better matches
		minLen := 5 // Lower threshold from 10 to 5
		if len(targetStr) >= minLen && len(trimmedStr) >= minLen {
			prefixLen := contextMin(minLen, contextMin(len(targetStr), len(trimmedStr)))
			suffixLen := contextMin(minLen, contextMin(len(targetStr), len(trimmedStr)))

			// Match if similar prefix OR suffix (not requiring both)
			hasSimilarPrefix := targetStr[:prefixLen] == trimmedStr[:prefixLen]
			hasSimilarSuffix := len(targetStr) >= suffixLen && len(trimmedStr) >= suffixLen &&
				targetStr[len(targetStr)-suffixLen:] == trimmedStr[len(trimmedStr)-suffixLen:]

			if hasSimilarPrefix || hasSimilarSuffix {
				similarPatterns = append(similarPatterns, lineRef)
			}
		}
	}

	return similarPatterns
}

// CalculatePatternUniquenessZeroAlloc calculates how unique a pattern is using zero-alloc operations
func (zce *ZeroAllocContextExtractor) CalculatePatternUniquenessZeroAlloc(fileID types.FileID, patternRef types.ZeroAllocStringRef) float64 {
	totalLines := zce.zeroAllocStore.GetLineCount(fileID)
	if totalLines == 0 {
		return 1.0
	}

	matchCount := 0
	trimmedPattern := patternRef.TrimSpace()

	for i := 0; i < totalLines; i++ {
		lineRef := zce.zeroAllocStore.GetZeroAllocLine(fileID, i)
		if lineRef.IsEmpty() {
			continue
		}

		// Simple similarity check
		trimmed := lineRef.TrimSpace()
		if trimmed.EqualString(trimmedPattern.String()) {
			matchCount++
		}
	}

	// Return uniqueness score (1.0 = completely unique, 0.0 = very common)
	return 1.0 - (float64(matchCount) / float64(totalLines))
}

// ToStrings converts ZeroAllocExtractedContext to string array (only when needed)
func (zec *ZeroAllocExtractedContext) ToStrings() []string {
	result := make([]string, 0, zec.TotalLines)

	// Add before lines
	for _, line := range zec.BeforeLines {
		result = append(result, line.String())
	}

	// Add match line
	if !zec.MatchLineRef.IsEmpty() {
		result = append(result, zec.MatchLineRef.String())
	}

	// Add after lines
	for _, line := range zec.AfterLines {
		result = append(result, line.String())
	}

	return result
}


// Helper functions using local names to avoid conflicts
func contextMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func contextMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}