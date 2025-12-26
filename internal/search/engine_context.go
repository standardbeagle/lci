package search

import (
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

func (ce *ContextExtractor) Extract(fileInfo *types.FileInfo, matchLine int, maxContextLines int) ExtractedContext {
	return ce.ExtractWithOptions(fileInfo, matchLine, maxContextLines, false)
}

func (ce *ContextExtractor) ExtractWithOptions(fileInfo *types.FileInfo, matchLine int, maxContextLines int, ensureCompleteStmt bool) ExtractedContext {
	// If maxContextLines is 0, use block boundaries
	if maxContextLines == 0 {
		return ce.extractBlockContextEnhanced(fileInfo, matchLine, ensureCompleteStmt)
	}

	// New approach: Try to extract full function context first, then expand with focused context
	return ce.extractFunctionContextWithPadding(fileInfo, matchLine, maxContextLines)
}

// ExtractWithSearchOptions provides full control over context extraction
func (ce *ContextExtractor) ExtractWithSearchOptions(fileInfo *types.FileInfo, matchLine int, options types.SearchOptions) ExtractedContext {
	lineCount := ce.lineProvider.GetLineCount(fileInfo)

	// If FullFunction is requested, try to show the complete function
	if options.FullFunction {
		// Find the function containing this match
		for i := range fileInfo.Blocks {
			block := &fileInfo.Blocks[i]
			if (block.Type == types.BlockTypeFunction || block.Type == types.BlockTypeMethod) &&
				block.Start+1 <= matchLine && block.End+1 >= matchLine {

				// Check if function is within size limits
				functionLines := block.End - block.Start + 1
				maxFuncLines := options.MaxFunctionLines
				if maxFuncLines == 0 {
					maxFuncLines = 500 // Default max
				}

				if functionLines <= maxFuncLines {
					start := block.Start
					end := min(block.End+1, lineCount)

					// Include leading comments if requested
					if options.EnsureCompleteStmt {
						start = ce.findFunctionStart(fileInfo, start, block.Name)
					}

					// If function is > 100 lines, limit to 100 lines centered on match
					finalLines := end - start
					if finalLines > 102 { // Use 102 to account for off-by-one differences and package line
						// Center the 100 lines around the match
						matchOffset := matchLine - 1 - start // 0-based offset within function
						contextStart := max(0, matchOffset-50)
						contextEnd := min(finalLines, contextStart+99)

						// Adjust if we're near the boundaries
						if contextEnd-contextStart < 100 && finalLines >= 102 {
							if contextEnd == finalLines {
								contextStart = finalLines - 99
							} else if contextStart == 0 {
								contextEnd = 99
							}
						}

						return ExtractedContext{
							Lines:      ce.lineProvider.GetLineRange(fileInfo, start+contextStart+1, start+contextEnd),
							StartLine:  start + contextStart + 1,
							EndLine:    start + contextEnd,
							BlockType:  "function",
							BlockName:  block.Name,
							IsComplete: finalLines <= 102,
						}
					}

					return ExtractedContext{
						Lines:      ce.lineProvider.GetLineRange(fileInfo, start+1, end),
						StartLine:  start + 1,
						EndLine:    end,
						BlockType:  "function",
						BlockName:  block.Name,
						IsComplete: true,
					}
				} else {
					// Function is too large (> maxFuncLines), fall back to simple context (±5 lines)
					return ce.extractLineContext(fileInfo, matchLine, 5)
				}
			}
		}
	}

	// Fall back to normal context extraction with padding
	return ce.ExtractWithOptions(fileInfo, matchLine, options.MaxContextLines, options.EnsureCompleteStmt)
}

func (ce *ContextExtractor) extractBlockContext(fileInfo *types.FileInfo, matchLine int) ExtractedContext {
	lineCount := ce.lineProvider.GetLineCount(fileInfo)

	// Find the smallest block containing the match
	var containingBlock *types.BlockBoundary

	for i := range fileInfo.Blocks {
		block := &fileInfo.Blocks[i]
		// Tree-sitter lines are 0-based, our lines are 1-based
		if block.Start+1 <= matchLine && block.End+1 >= matchLine {
			if containingBlock == nil ||
				(block.End-block.Start) < (containingBlock.End-containingBlock.Start) {
				containingBlock = block
			}
		}
	}

	if containingBlock == nil {
		// No containing block, fall back to default context lines
		return ce.extractLineContext(fileInfo, matchLine, ce.defaultContextLines*2)
	}

	start := containingBlock.Start
	end := min(containingBlock.End+1, lineCount)

	contextLines := ce.lineProvider.GetLineRange(fileInfo, start+1, end)

	// Apply max lines limit if needed
	isComplete := true
	if ce.maxLines > 0 && len(contextLines) > ce.maxLines {
		// Try to keep the match line visible
		matchOffset := matchLine - 1 - start
		halfMax := ce.maxLines / 2

		newStart := max(0, matchOffset-halfMax)
		newEnd := min(len(contextLines), newStart+ce.maxLines)

		contextLines = contextLines[newStart:newEnd]
		start += newStart
		isComplete = false
	}

	return ExtractedContext{
		Lines:      contextLines,
		StartLine:  start + 1, // Convert to 1-based
		EndLine:    start + len(contextLines),
		BlockType:  containingBlock.Type.String(),
		BlockName:  containingBlock.Name,
		IsComplete: isComplete,
	}
}

func (ce *ContextExtractor) extractLineContext(fileInfo *types.FileInfo, matchLine int, numLines int) ExtractedContext {
	lineCount := ce.lineProvider.GetLineCount(fileInfo)
	halfLines := numLines / 2
	start := max(0, matchLine-1-halfLines)
	end := min(lineCount, matchLine+halfLines)

	contextLines := ce.lineProvider.GetLineRange(fileInfo, start+1, end)

	return ExtractedContext{
		Lines:      contextLines,
		StartLine:  start + 1,
		EndLine:    end,
		BlockType:  "lines",
		BlockName:  "",
		IsComplete: true,
	}
}

// extractFunctionContextWithPadding extracts the full function containing the match,
// then shows entire function unless >100 lines, then clamps at ±5 from match
func (ce *ContextExtractor) extractFunctionContextWithPadding(fileInfo *types.FileInfo, matchLine int, maxContextLines int) ExtractedContext {
	lineCount := ce.lineProvider.GetLineCount(fileInfo)

	// First, try to find the function containing this match
	var containingFunction *types.BlockBoundary
	for i := range fileInfo.Blocks {
		block := &fileInfo.Blocks[i]
		// Check if this block contains the match line
		if (block.Type == types.BlockTypeFunction || block.Type == types.BlockTypeMethod) &&
			block.Start+1 <= matchLine && block.End+1 >= matchLine {
			// Find the smallest containing function
			if containingFunction == nil ||
				(block.End-block.Start) < (containingFunction.End-containingFunction.Start) {
				containingFunction = block
			}
		}
	}

	var start, end int
	var blockType, blockName string
	var isComplete bool = true

	if containingFunction != nil {
		// Start with the full function (including leading comments)
		start = ce.findFunctionStart(fileInfo, containingFunction.Start, containingFunction.Name)
		end = min(containingFunction.End+1, lineCount)
		blockType = containingFunction.Type.String()
		blockName = containingFunction.Name

		// Sanity check: if the detected function is unreasonably large (>500 lines),
		// it's likely a Tree-sitter parsing error, so fall back to smaller context
		functionLength := end - start
		if functionLength > 500 {
			// Tree-sitter boundary seems wrong, use smaller context around match
			padding := 5
			start = max(0, matchLine-1-padding)
			end = min(lineCount, matchLine+padding)
			isComplete = false
		} else if functionLength > 100 {
			// Function is legitimately long, show up to 100 lines centered on match
			// Calculate how to center the match within 100 lines
			matchOffsetInFunction := matchLine - 1 - start
			halfWindow := 50 // 100 lines / 2

			// Try to center the match in the 100-line window
			windowStart := max(0, matchOffsetInFunction-halfWindow)
			windowEnd := min(functionLength, windowStart+100)

			// Adjust if we hit the end
			if windowEnd == functionLength {
				windowStart = max(0, functionLength-100)
			}

			// Update start and end to show the 100-line window
			start = start + windowStart
			end = start + 100
			isComplete = false
		}
		// If function is ≤100 lines, keep the full function (no changes to start/end)

	} else {
		// No containing function found, fall back to ±5 line context around match
		padding := 5
		start = max(0, matchLine-1-padding)
		end = min(lineCount, matchLine+padding)
		blockType = "context"
		blockName = ""
	}

	contextLines := ce.lineProvider.GetLineRange(fileInfo, start+1, end)

	return ExtractedContext{
		Lines:      contextLines,
		StartLine:  start + 1,
		EndLine:    end,
		BlockType:  blockType,
		BlockName:  blockName,
		IsComplete: isComplete,
	}
}
func (ce *ContextExtractor) extractBlockContextEnhanced(fileInfo *types.FileInfo, matchLine int, ensureCompleteStmt bool) ExtractedContext {
	lineCount := ce.lineProvider.GetLineCount(fileInfo)

	// Find the smallest block containing the match
	var containingBlock *types.BlockBoundary

	for i := range fileInfo.Blocks {
		block := &fileInfo.Blocks[i]
		// Tree-sitter lines are 0-based, our lines are 1-based
		if block.Start+1 <= matchLine && block.End+1 >= matchLine {
			if containingBlock == nil ||
				(block.End-block.Start) < (containingBlock.End-containingBlock.Start) {
				containingBlock = block
			}
		}
	}

	if containingBlock == nil {
		// No containing block, fall back to default context lines
		return ce.extractLineContext(fileInfo, matchLine, ce.defaultContextLines*2)
	}

	start := containingBlock.Start
	end := min(containingBlock.End+1, lineCount)

	// Sanity check: if the detected block is unreasonably large (>500 lines),
	// it's likely a Tree-sitter parsing error, so fall back to smaller context
	blockLength := end - start
	if blockLength > 500 {
		// Tree-sitter boundary seems wrong, use smaller context around match
		return ce.extractLineContext(fileInfo, matchLine, ce.defaultContextLines*2)
	}

	// For functions, ensure we include leading comments and complete signatures
	if ensureCompleteStmt && (containingBlock.Type == types.BlockTypeFunction || containingBlock.Type == types.BlockTypeMethod) {
		start = ce.findFunctionStart(fileInfo, start, containingBlock.Name)
	}

	contextLines := ce.lineProvider.GetLineRange(fileInfo, start+1, end)

	// Apply max lines limit if needed
	isComplete := true
	if ce.maxLines > 0 && len(contextLines) > ce.maxLines {
		// For functions, try to preserve the signature at the beginning
		if ensureCompleteStmt && (containingBlock.Type == types.BlockTypeFunction || containingBlock.Type == types.BlockTypeMethod) {
			signatureEnd := ce.findSignatureEnd(contextLines)
			if signatureEnd > 0 && signatureEnd < ce.maxLines {
				// Keep full signature plus remaining lines
				contextLines = contextLines[:ce.maxLines]
			} else {
				// Signature too long, center on match line
				matchOffset := matchLine - 1 - start
				halfMax := ce.maxLines / 2
				newStart := max(0, matchOffset-halfMax)
				newEnd := min(len(contextLines), newStart+ce.maxLines)
				contextLines = contextLines[newStart:newEnd]
				start += newStart
			}
		} else {
			// Try to keep the match line visible
			matchOffset := matchLine - 1 - start
			halfMax := ce.maxLines / 2
			newStart := max(0, matchOffset-halfMax)
			newEnd := min(len(contextLines), newStart+ce.maxLines)
			contextLines = contextLines[newStart:newEnd]
			start += newStart
		}
		isComplete = false
	}

	return ExtractedContext{
		Lines:      contextLines,
		StartLine:  start + 1, // Convert to 1-based
		EndLine:    start + len(contextLines),
		BlockType:  containingBlock.Type.String(),
		BlockName:  containingBlock.Name,
		IsComplete: isComplete,
	}
}

// findFunctionStart finds the actual start of a function including leading comments
func (ce *ContextExtractor) findFunctionStart(fileInfo *types.FileInfo, blockStart int, functionName string) int {
	// Look backwards from block start to find leading comments and annotations
	start := blockStart
	for i := blockStart - 1; i >= 0; i-- {
		lineContent := ce.lineProvider.GetLine(fileInfo, i+1) // Convert to 1-based
		line := strings.TrimSpace(lineContent)
		if line == "" {
			// Empty line, continue looking
			continue
		}
		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") ||
			strings.HasPrefix(line, "*") || strings.HasPrefix(line, "@") ||
			strings.HasPrefix(line, "#") {
			// Comment or annotation, include it
			start = i
			continue
		}
		// Non-comment, non-empty line, stop here
		break
	}
	return start
}

// findSignatureEnd finds the end of a function signature (opening brace or arrow)
func (ce *ContextExtractor) findSignatureEnd(lines []string) int {
	for i, line := range lines {
		if strings.Contains(line, "{") || strings.Contains(line, "=>") {
			return i + 1
		}
		// For languages like Python, look for colon
		if strings.HasSuffix(strings.TrimSpace(line), ":") {
			return i + 1
		}
	}
	return min(3, len(lines)) // Default to first 3 lines if no clear signature end
}
