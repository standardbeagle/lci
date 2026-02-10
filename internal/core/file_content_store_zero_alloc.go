package core

import (
	"bytes"
	"encoding/json"
	"github.com/standardbeagle/lci/internal/types"
	"unicode/utf8"
)

// ============================================================================
// ZERO-ALLOCATION GETTER METHODS
// ============================================================================

// GetZeroAllocStringRef returns a ZeroAllocStringRef without any allocation
func (fcs *FileContentStore) GetZeroAllocStringRef(ref types.StringRef) types.ZeroAllocStringRef {
	if content, ok := fcs.GetContent(ref.FileID); ok {
		end := ref.Offset + ref.Length
		if ref.Offset < uint32(len(content)) && end <= uint32(len(content)) {
			return types.ZeroAllocStringRef{
				Data:   content,
				FileID: ref.FileID,
				Offset: ref.Offset,
				Length: ref.Length,
				Hash:   ref.Hash,
			}
		}
	}
	return types.EmptyZeroAllocStringRef
}

// GetZeroAllocLine returns a ZeroAllocStringRef for a specific line without allocation
func (fcs *FileContentStore) GetZeroAllocLine(fileID types.FileID, lineNum int) types.ZeroAllocStringRef {
	if ref, ok := fcs.GetLine(fileID, lineNum); ok {
		return fcs.GetZeroAllocStringRef(ref)
	}
	return types.EmptyZeroAllocStringRef
}

// GetZeroAllocLines returns ZeroAllocStringRefs for a range of lines without allocation
// Parameters: startLine (0-indexed), count (number of lines to return)
func (fcs *FileContentStore) GetZeroAllocLines(fileID types.FileID, startLine, count int) []types.ZeroAllocStringRef {
	if content, ok := fcs.GetContent(fileID); ok {
		// Get line offsets
		lineOffsets := fcs.getZeroAllocLineOffsets(fileID)
		if lineOffsets == nil || len(lineOffsets) == 0 {
			return nil
		}

		// Bounds checking
		if startLine < 0 {
			startLine = 0
		}
		if count <= 0 {
			return nil
		}

		// Calculate end line from start + count
		endLine := startLine + count
		if endLine > len(lineOffsets) {
			endLine = len(lineOffsets)
		}
		if startLine >= len(lineOffsets) {
			return nil
		}

		refs := make([]types.ZeroAllocStringRef, 0, endLine-startLine)
		for i := startLine; i < endLine; i++ {
			start := lineOffsets[i]
			var end uint32

			if i+1 < len(lineOffsets) {
				end = lineOffsets[i+1]
				// Remove newline character
				if end > start && content[end-1] == '\n' {
					end--
				}
			} else {
				end = uint32(len(content))
			}

			length := end - start
			// Include empty lines as well - they're valid context
			var hash uint64
			if length > 0 && int(end) <= len(content) {
				hash = types.ComputeHash(content[start:end])
			}
			refs = append(refs, types.ZeroAllocStringRef{
				Data:   content,
				FileID: fileID,
				Offset: start,
				Length: length,
				Hash:   hash,
			})
		}

		return refs
	}

	return nil
}

// GetZeroAllocContextLines returns ZeroAllocStringRefs for context lines without allocation
func (fcs *FileContentStore) GetZeroAllocContextLines(fileID types.FileID, lineNum, before, after int) []types.ZeroAllocStringRef {
	start := lineNum - before
	count := before + 1 + after // before lines + match line + after lines
	return fcs.GetZeroAllocLines(fileID, start, count)
}

// ============================================================================
// ZERO-ALLOCATION SEARCH AND PATTERN MATCHING
// ============================================================================

// SearchInLine searches for a pattern in a specific line without allocation
func (fcs *FileContentStore) SearchInLine(fileID types.FileID, lineNum int, pattern string) bool {
	lineRef := fcs.GetZeroAllocLine(fileID, lineNum)
	return lineRef.Contains(pattern)
}

// SearchInLines searches for a pattern in multiple lines without allocation
func (fcs *FileContentStore) SearchInLines(fileID types.FileID, startLine, endLine int, pattern string) []int {
	count := endLine - startLine
	if count <= 0 {
		return nil
	}
	lines := fcs.GetZeroAllocLines(fileID, startLine, count)
	var matches []int

	for i, lineRef := range lines {
		if lineRef.Contains(pattern) {
			matches = append(matches, startLine+i)
		}
	}

	return matches
}

// SearchLinesWithAnyPrefix finds lines that start with any of the given prefixes without allocation
func (fcs *FileContentStore) SearchLinesWithAnyPrefix(fileID types.FileID, startLine, endLine int, prefixes ...string) []int {
	count := endLine - startLine
	if count <= 0 {
		return nil
	}
	lines := fcs.GetZeroAllocLines(fileID, startLine, count)
	var matches []int

	for i, lineRef := range lines {
		if lineRef.HasAnyPrefix(prefixes...) {
			matches = append(matches, startLine+i)
		}
	}

	return matches
}

// SearchLinesWithAnySuffix finds lines that end with any of the given suffixes without allocation
func (fcs *FileContentStore) SearchLinesWithAnySuffix(fileID types.FileID, startLine, endLine int, suffixes ...string) []int {
	count := endLine - startLine
	if count <= 0 {
		return nil
	}
	lines := fcs.GetZeroAllocLines(fileID, startLine, count)
	var matches []int

	for i, lineRef := range lines {
		if lineRef.HasAnySuffix(suffixes...) {
			matches = append(matches, startLine+i)
		}
	}

	return matches
}

// ============================================================================
// ZERO-ALLOCATION TEXT PROCESSING
// ============================================================================

// TrimWhitespaceLines removes leading/trailing whitespace from lines without allocation
func (fcs *FileContentStore) TrimWhitespaceLines(lines []types.ZeroAllocStringRef) []types.ZeroAllocStringRef {
	result := make([]types.ZeroAllocStringRef, 0, len(lines))
	for _, line := range lines {
		trimmed := line.TrimSpace()
		if !trimmed.IsEmpty() {
			result = append(result, trimmed)
		}
	}
	return result
}

// FilterEmptyLines removes empty lines without allocation
func (fcs *FileContentStore) FilterEmptyLines(lines []types.ZeroAllocStringRef) []types.ZeroAllocStringRef {
	result := make([]types.ZeroAllocStringRef, 0, len(lines))
	for _, line := range lines {
		if !line.IsEmptyString() {
			result = append(result, line)
		}
	}
	return result
}

// FilterCommentLines removes comment lines without allocation
func (fcs *FileContentStore) FilterCommentLines(lines []types.ZeroAllocStringRef, commentPrefixes ...string) []types.ZeroAllocStringRef {
	if len(commentPrefixes) == 0 {
		commentPrefixes = []string{"//", "#", "/*", "*", "*/"}
	}

	result := make([]types.ZeroAllocStringRef, 0, len(lines))
	for _, line := range lines {
		trimmed := line.TrimSpace()
		if !trimmed.HasAnyPrefix(commentPrefixes...) {
			result = append(result, line)
		}
	}
	return result
}

// ============================================================================
// ZERO-ALLOCATION CODE ANALYSIS HELPERS
// ============================================================================

// ExtractFunctionName attempts to extract a function name from a line without allocation
func (fcs *FileContentStore) ExtractFunctionName(lineRef types.ZeroAllocStringRef) types.ZeroAllocStringRef {
	trimmed := lineRef.TrimSpace()

	// Common function patterns
	patterns := []string{
		"func ",
		"function ",
		"def ",
		"void ",
		"int ",
		"string ",
		"bool ",
		"float ",
		"double ",
		"public ",
		"private ",
		"protected ",
		"static ",
		"async ",
	}

	trimmedLower := trimmed.ToLower()
	for _, pattern := range patterns {
		if idx := trimmedLower.Index(pattern); idx >= 0 {
			// Extract name after pattern
			nameStart := idx + len(pattern)
			if nameStart < trimmed.Len() {
				nameLine := trimmed.Substring(nameStart, trimmed.Len())

				// Find end of name (before '(' or '{' or ':')
				if parenIdx := nameLine.Index("("); parenIdx >= 0 {
					return nameLine.Substring(0, parenIdx).TrimSpace()
				}
				if braceIdx := nameLine.Index("{"); braceIdx >= 0 {
					return nameLine.Substring(0, braceIdx).TrimSpace()
				}
				if colonIdx := nameLine.Index(":"); colonIdx >= 0 {
					return nameLine.Substring(0, colonIdx).TrimSpace()
				}

				return nameLine.TrimSpace()
			}
		}
	}

	return types.EmptyZeroAllocStringRef
}

// ExtractVariableName attempts to extract a variable name from a line without allocation
func (fcs *FileContentStore) ExtractVariableName(lineRef types.ZeroAllocStringRef) types.ZeroAllocStringRef {
	trimmed := lineRef.TrimSpace()

	// Common variable patterns
	patterns := []string{
		"var ",
		"let ",
		"const ",
		"string ",
		"int ",
		"bool ",
		"float ",
		"double ",
		"char ",
		"byte ",
	}

	trimmedLower := trimmed.ToLower()
	for _, pattern := range patterns {
		if idx := trimmedLower.Index(pattern); idx >= 0 {
			// Extract name after pattern
			nameStart := idx + len(pattern)
			if nameStart < trimmed.Len() {
				nameLine := trimmed.Substring(nameStart, trimmed.Len())

				// Find end of name (before '=', ';', or ',')
				if eqIdx := nameLine.Index("="); eqIdx >= 0 {
					return nameLine.Substring(0, eqIdx).TrimSpace()
				}
				if semicolonIdx := nameLine.Index(";"); semicolonIdx >= 0 {
					return nameLine.Substring(0, semicolonIdx).TrimSpace()
				}
				if commaIdx := nameLine.Index(","); commaIdx >= 0 {
					return nameLine.Substring(0, commaIdx).TrimSpace()
				}

				return nameLine.TrimSpace()
			}
		}
	}

	return types.EmptyZeroAllocStringRef
}

// ExtractImportPath attempts to extract an import path from a line without allocation
func (fcs *FileContentStore) ExtractImportPath(lineRef types.ZeroAllocStringRef) types.ZeroAllocStringRef {
	trimmed := lineRef.TrimSpace()

	// Common import patterns
	patterns := []string{
		"import ",
		"from ",
		"#include ",
		"require ",
		"use ",
	}

	trimmedLower := trimmed.ToLower()
	for _, pattern := range patterns {
		if idx := trimmedLower.Index(pattern); idx >= 0 {
			// Extract path after pattern
			pathStart := idx + len(pattern)
			if pathStart < trimmed.Len() {
				pathLine := trimmed.Substring(pathStart, trimmed.Len())

				// Remove quotes if present
				pathLine = pathLine.Trim("\"")
				pathLine = pathLine.Trim("'")
				pathLine = pathLine.Trim("`")

				// Remove trailing semicolons or commas
				pathLine = pathLine.TrimRight(";,")

				return pathLine.TrimSpace()
			}
		}
	}

	return types.EmptyZeroAllocStringRef
}

// ============================================================================
// ZERO-ALLOCATION BULK OPERATIONS
// ============================================================================

// ProcessLinesInBulk processes multiple lines with a function without allocation
func (fcs *FileContentStore) ProcessLinesInBulk(fileID types.FileID, startLine, endLine int, processor func(types.ZeroAllocStringRef) bool) []int {
	count := endLine - startLine
	if count <= 0 {
		return nil
	}
	lines := fcs.GetZeroAllocLines(fileID, startLine, count)
	var results []int

	for i, line := range lines {
		if processor(line) {
			results = append(results, startLine+i)
		}
	}

	return results
}

// FindAllLinesWithPattern finds all lines containing a pattern without allocation
func (fcs *FileContentStore) FindAllLinesWithPattern(fileID types.FileID, pattern string) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchInLines(fileID, 0, lineCount, pattern)
}

// FindAllLinesWithPrefix finds all lines starting with a prefix without allocation
func (fcs *FileContentStore) FindAllLinesWithPrefix(fileID types.FileID, prefix string) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchLinesWithAnyPrefix(fileID, 0, lineCount, prefix)
}

// FindAllLinesWithSuffix finds all lines ending with a suffix without allocation
func (fcs *FileContentStore) FindAllLinesWithSuffix(fileID types.FileID, suffix string) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchLinesWithAnySuffix(fileID, 0, lineCount, suffix)
}

// ============================================================================
// ZERO-ALLOCATION JSON SERIALIZATION
// ============================================================================

// LinesToJSON converts lines to JSON without intermediate string allocation
func (fcs *FileContentStore) LinesToJSON(lines []types.ZeroAllocStringRef) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("[")

	for i, line := range lines {
		if i > 0 {
			buf.WriteString(",")
		}

		lineJSON, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		buf.Write(lineJSON)
	}

	buf.WriteString("]")
	return buf.Bytes(), nil
}

// ContextToJSON converts context lines to JSON without intermediate string allocation
func (fcs *FileContentStore) ContextToJSON(fileID types.FileID, lineNum, before, after int) ([]byte, error) {
	lines := fcs.GetZeroAllocContextLines(fileID, lineNum, before, after)
	return fcs.LinesToJSON(lines)
}

// ============================================================================
// ZERO-ALLOCATION SEARCH RESULT PROCESSING
// ============================================================================

// SearchResults represents zero-allocation search results
type SearchResults struct {
	LineNumbers []int
	Lines       []types.ZeroAllocStringRef
	Context     map[int][]types.ZeroAllocStringRef
}

// FindWithContext finds all matches and provides context without allocation
func (fcs *FileContentStore) FindWithContext(fileID types.FileID, pattern string, contextLines int) SearchResults {
	matches := fcs.FindAllLinesWithPattern(fileID, pattern)
	results := SearchResults{
		LineNumbers: matches,
		Lines:       make([]types.ZeroAllocStringRef, len(matches)),
		Context:     make(map[int][]types.ZeroAllocStringRef),
	}

	for i, lineNum := range matches {
		results.Lines[i] = fcs.GetZeroAllocLine(fileID, lineNum)
		if contextLines > 0 {
			results.Context[lineNum] = fcs.GetZeroAllocContextLines(fileID, lineNum, contextLines, contextLines)
		}
	}

	return results
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// getZeroAllocLineOffsets returns line offsets for a file (internal helper)
func (fcs *FileContentStore) getZeroAllocLineOffsets(fileID types.FileID) []uint32 {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	if fcVal, ok := snapshot.files.Load(fileID); ok {
		return fcVal.(*FileContent).LineOffsets
	}
	return nil
}

// IsCommentLine checks if a line is a comment without allocation
func (fcs *FileContentStore) IsCommentLine(lineRef types.ZeroAllocStringRef) bool {
	trimmed := lineRef.TrimSpace()
	return trimmed.HasAnyPrefix("//", "#", "/*", "*", "*/", "<!--", "-->")
}

// IsEmptyLine checks if a line is empty or contains only whitespace without allocation
func (fcs *FileContentStore) IsEmptyLine(lineRef types.ZeroAllocStringRef) bool {
	return lineRef.IsEmptyString() || lineRef.TrimSpace().IsEmptyString()
}

// HasCodeContent checks if a line contains actual code (not just comments/whitespace) without allocation
func (fcs *FileContentStore) HasCodeContent(lineRef types.ZeroAllocStringRef) bool {
	return !fcs.IsEmptyLine(lineRef) && !fcs.IsCommentLine(lineRef)
}

// ExtractBraceContent extracts content between braces without allocation
func (fcs *FileContentStore) ExtractBraceContent(lineRef types.ZeroAllocStringRef) types.ZeroAllocStringRef {
	if openIdx := lineRef.Index("{"); openIdx >= 0 {
		if closeIdx := lineRef.Index("}"); closeIdx > openIdx {
			return lineRef.Substring(openIdx+1, closeIdx)
		}
	}
	return types.EmptyZeroAllocStringRef
}

// ExtractParenContent extracts content between parentheses without allocation
func (fcs *FileContentStore) ExtractParenContent(lineRef types.ZeroAllocStringRef) types.ZeroAllocStringRef {
	if openIdx := lineRef.Index("("); openIdx >= 0 {
		if closeIdx := lineRef.Index(")"); closeIdx > openIdx {
			return lineRef.Substring(openIdx+1, closeIdx)
		}
	}
	return types.EmptyZeroAllocStringRef
}

// ============================================================================
// ZERO-ALLOCATION PATTERN MATCHING FOR COMMON CODE PATTERNS
// ============================================================================

// FindFunctionDefinitions finds all function definition lines without allocation
func (fcs *FileContentStore) FindFunctionDefinitions(fileID types.FileID) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchLinesWithAnyPrefix(fileID, 0, lineCount,
		"func ", "function ", "def ", "void ", "int ", "string ", "bool ")
}

// FindVariableDeclarations finds all variable declaration lines without allocation
func (fcs *FileContentStore) FindVariableDeclarations(fileID types.FileID) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchLinesWithAnyPrefix(fileID, 0, lineCount,
		"var ", "let ", "const ", "string ", "int ", "bool ", "float ")
}

// FindImportStatements finds all import statement lines without allocation
func (fcs *FileContentStore) FindImportStatements(fileID types.FileID) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchLinesWithAnyPrefix(fileID, 0, lineCount,
		"import ", "from ", "#include ", "require ", "use ")
}

// FindClassDefinitions finds all class definition lines without allocation
func (fcs *FileContentStore) FindClassDefinitions(fileID types.FileID) []int {
	lineCount := fcs.GetLineCount(fileID)
	return fcs.SearchLinesWithAnyPrefix(fileID, 0, lineCount,
		"class ", "interface ", "struct ", "type ", "enum ")
}

// ============================================================================
// ZERO-ALLOCATION TEXT TRANSFORMATION UTILITIES
// ============================================================================

// RemoveCommonPrefix removes common prefix from multiple lines without allocation
func (fcs *FileContentStore) RemoveCommonPrefix(lines []types.ZeroAllocStringRef) []types.ZeroAllocStringRef {
	if len(lines) == 0 {
		return lines
	}

	// Find common prefix
	commonPrefix := lines[0]
	for i := 1; i < len(lines); i++ {
		commonPrefix = fcs.findCommonPrefix(commonPrefix, lines[i])
		if commonPrefix.IsEmpty() {
			break
		}
	}

	if commonPrefix.IsEmpty() {
		return lines
	}

	// Remove common prefix from all lines
	result := make([]types.ZeroAllocStringRef, 0, len(lines))
	for _, line := range lines {
		if line.HasPrefix(commonPrefix.String()) {
			result = append(result, line.Substring(commonPrefix.Len(), line.Len()))
		} else {
			result = append(result, line)
		}
	}

	return result
}

// findCommonPrefix finds common prefix between two ZeroAllocStringRefs
func (fcs *FileContentStore) findCommonPrefix(a, b types.ZeroAllocStringRef) types.ZeroAllocStringRef {
	minLen := a.Len()
	if b.Len() < minLen {
		minLen = b.Len()
	}

	i := 0
	for i < minLen && a.Bytes()[i] == b.Bytes()[i] {
		i++
	}

	return a.Substring(0, i)
}

// ============================================================================
// ZERO-ALLOCATION UNICODE AWARE OPERATIONS
// ============================================================================

// CountRunes counts Unicode runes in the reference without allocation
func (fcs *FileContentStore) CountRunes(ref types.ZeroAllocStringRef) int {
	return utf8.RuneCount(ref.Bytes())
}

// ExtractRunes extracts Unicode runes within a range without allocation
func (fcs *FileContentStore) ExtractRunes(ref types.ZeroAllocStringRef, startRune, endRune int) types.ZeroAllocStringRef {
	bytes := ref.Bytes()

	// Convert rune positions to byte positions
	runeCount := 0
	byteStart := 0
	byteEnd := len(bytes)

	for i := 0; i < len(bytes) && runeCount <= endRune; {
		_, size := utf8.DecodeRune(bytes[i:])
		if i == 0 {
			byteStart = i
		}

		if runeCount == startRune {
			byteStart = i
		}
		if runeCount == endRune {
			byteEnd = i
			break
		}

		i += size
		runeCount++
	}

	return types.ZeroAllocStringRef{
		Data:   ref.Data,
		FileID: ref.FileID,
		Offset: ref.Offset + uint32(byteStart),
		Length: uint32(byteEnd - byteStart),
		Hash:   types.ComputeHash(bytes[byteStart:byteEnd]),
	}
}
