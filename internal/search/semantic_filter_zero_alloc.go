package search

import (
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// ZeroAllocSearchMatch represents a zero-allocation search result
type ZeroAllocSearchMatch struct {
	Line    int    // Line number (1-based)
	Start   int    // Start position in bytes
	End     int    // End position in bytes
	Pattern string // Search pattern
}

// ZeroAllocSemanticFilter provides zero-allocation semantic filtering operations
// This eliminates string allocations from semantic filtering functions
type ZeroAllocSemanticFilter struct {
	store *core.FileContentStore
}

// NewZeroAllocSemanticFilter creates a new zero-allocation semantic filter
func NewZeroAllocSemanticFilter(fileStore *core.FileContentStore) *ZeroAllocSemanticFilter {
	return &ZeroAllocSemanticFilter{
		store: fileStore,
	}
}

// ApplySemanticFilteringZeroAlloc performs zero-allocation semantic filtering
func (zasf *ZeroAllocSemanticFilter) ApplySemanticFilteringZeroAlloc(fileInfo *types.FileInfo, matches []ZeroAllocSearchMatch, pattern string, options types.SearchOptions) []ZeroAllocSearchMatch {
	if len(options.SymbolTypes) == 0 && !options.DeclarationOnly && !options.UsageOnly &&
		!options.ExportedOnly && !options.ExcludeTests && !options.ExcludeComments &&
		!options.MutableOnly && !options.GlobalOnly {
		return matches // No semantic filtering requested
	}

	var filteredMatches []ZeroAllocSearchMatch

	for _, match := range matches {
		if zasf.passesSemanticFilterZeroAlloc(fileInfo, match, pattern, options) {
			filteredMatches = append(filteredMatches, match)
		}
	}

	return filteredMatches
}

// passesSemanticFilterZeroAlloc checks if a match passes semantic filtering using zero-alloc operations
func (zasf *ZeroAllocSemanticFilter) passesSemanticFilterZeroAlloc(fileInfo *types.FileInfo, match ZeroAllocSearchMatch, pattern string, options types.SearchOptions) bool {
	line := match.Line

	// Check if in comment (if ExcludeComments is enabled)
	if options.ExcludeComments && zasf.isInCommentZeroAlloc(fileInfo, line) {
		return false
	}

	// ExcludeTests option is deprecated - test file filtering should be done at index/config level
	// Files should be excluded via Include/Exclude patterns in config

	// Find symbol at this location
	var matchingSymbol *types.EnhancedSymbol

	// First, try to find an exact match for the pattern
	if pattern != "" {
		for _, symbol := range fileInfo.EnhancedSymbols {
			// Check if this symbol matches our search pattern
			if symbol.Name == pattern && symbol.Line == line {
				matchingSymbol = symbol
				break
			}
		}
	}

	// If no exact match, look for any symbol on this line
	if matchingSymbol == nil {
		for _, symbol := range fileInfo.EnhancedSymbols {
			if symbol.Line == line {
				// For declaration searches, we need a symbol on this line
				matchingSymbol = symbol
				break
			}
		}
	}

	// Apply symbol-based filters
	if matchingSymbol != nil {
		// Filter by symbol types
		if len(options.SymbolTypes) > 0 {
			// Use the String() method to get proper string representation
			if !zasf.containsStringSlice(options.SymbolTypes, matchingSymbol.Type.String()) {
				return false
			}
		}

		// Declaration only filter
		if options.DeclarationOnly {
			// Only show symbols at their definition location
			return true
		}

		// Usage only filter - exclude declarations
		if options.UsageOnly {
			// Symbol found at this location means it's a declaration, not usage
			return false
		}

		// Exported only filter
		if options.ExportedOnly && !zasf.isExportedSymbolZeroAlloc(matchingSymbol) {
			return false
		}

		// Mutable only filter
		if options.MutableOnly && !zasf.isMutableSymbolZeroAlloc(matchingSymbol, fileInfo, line) {
			return false
		}

		// Global only filter
		if options.GlobalOnly && !zasf.isGlobalSymbolZeroAlloc(matchingSymbol, fileInfo) {
			return false
		}
	} else {
		// No symbol found at this location - this is likely a usage
		if options.DeclarationOnly {
			return false // Only want declarations, this is usage
		}

		// For usage-only filter, allow if we couldn't find a symbol (likely usage)
		if options.UsageOnly {
			return true
		}
	}

	return true
}

// isExportedSymbolZeroAlloc checks if a symbol is exported using zero-alloc operations
func (zasf *ZeroAllocSemanticFilter) isExportedSymbolZeroAlloc(symbol *types.EnhancedSymbol) bool {
	// Check the IsExported field first
	if symbol.IsExported {
		return true
	}
	// For most languages, a symbol is exported if it starts with uppercase
	if len(symbol.Name) == 0 {
		return false
	}

	// Check first character - this is zero-alloc since we're accessing the string directly
	firstChar := symbol.Name[0]
	return firstChar >= 'A' && firstChar <= 'Z'
}

// isMutableSymbolZeroAlloc checks if a symbol is mutable using zero-alloc operations
func (zasf *ZeroAllocSemanticFilter) isMutableSymbolZeroAlloc(symbol *types.EnhancedSymbol, fileInfo *types.FileInfo, line int) bool {
	// Check the IsMutable field first
	if symbol.IsMutable {
		return true
	}
	// Get the line content as ZeroAllocStringRef
	lineRef := zasf.store.GetZeroAllocLine(fileInfo.ID, line-1)
	if lineRef.IsEmpty() {
		return false
	}

	// Check for mutable patterns using zero-alloc operations
	// Variables (not constants)
	if lineRef.Contains("var ") || lineRef.Contains("let ") || lineRef.Contains(":=") {
		return true
	}

	// Function parameters (mutable in most contexts)
	if lineRef.Contains("func ") && lineRef.Contains("(") {
		// Check if this is a function parameter
		return true
	}

	// Struct fields (can be mutated)
	if lineRef.Contains("type ") && lineRef.Contains("struct") {
		return false // Struct definitions themselves aren't mutable
	}

	// Default to false for unknown patterns
	return false
}

// isGlobalSymbolZeroAlloc checks if a symbol is global using zero-alloc operations
func (zasf *ZeroAllocSemanticFilter) isGlobalSymbolZeroAlloc(symbol *types.EnhancedSymbol, fileInfo *types.FileInfo) bool {
	// Check the VariableType field first
	if symbol.VariableType == types.VariableTypeGlobal {
		return true
	}
	// Get the line content as ZeroAllocStringRef
	lineRef := zasf.store.GetZeroAllocLine(fileInfo.ID, symbol.Line-1)
	if lineRef.IsEmpty() {
		return false
	}

	// Check for global patterns using zero-alloc operations
	// Variables at package level (not inside functions)
	trimmed := lineRef.TrimSpace()
	if trimmed.HasPrefix("var ") || trimmed.HasPrefix("const ") {
		return true
	}

	// Functions at package level
	if trimmed.HasPrefix("func ") {
		return true
	}

	// Types at package level
	if trimmed.HasPrefix("type ") {
		return true
	}

	// Default to false for unknown patterns
	return false
}

// isInCommentZeroAlloc checks if a line is in a comment using zero-alloc operations
func (zasf *ZeroAllocSemanticFilter) isInCommentZeroAlloc(fileInfo *types.FileInfo, line int) bool {
	lineRef := zasf.store.GetZeroAllocLine(fileInfo.ID, line-1)
	if lineRef.IsEmpty() {
		return false
	}

	trimmed := lineRef.TrimSpace()

	// Single line comment
	if trimmed.HasPrefix("//") || trimmed.HasPrefix("#") {
		return true
	}

	// Multi-line comment start/end
	if trimmed.HasPrefix("/*") || trimmed.Contains("*/") {
		return true
	}

	return false
}

// containsStringSlice checks if a string slice contains a specific string (zero-alloc helper)
func (zasf *ZeroAllocSemanticFilter) containsStringSlice(slice []string, target string) bool {
	for _, item := range slice {
		// Case-insensitive comparison for symbol types
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

// ConvertMatchesToZeroAlloc converts traditional matches to zero-alloc matches
func (zasf *ZeroAllocSemanticFilter) ConvertMatchesToZeroAlloc(matches []interface{}, fileInfo *types.FileInfo) []ZeroAllocSearchMatch {
	result := make([]ZeroAllocSearchMatch, 0, len(matches))

	for _, match := range matches {
		// Extract Start and End from the match struct
		var start, end int

		// Use type assertion to extract fields
		if m, ok := match.(struct{ Start, End int }); ok {
			start = m.Start
			end = m.End
		} else {
			// If not the expected type, skip
			continue
		}

		// Find which line this byte position is on
		lineNum := zasf.findLineForBytePosition(fileInfo.ID, start)

		result = append(result, ZeroAllocSearchMatch{
			Line:    lineNum,
			Start:   start,
			End:     end,
			Pattern: "", // Pattern not available from byte positions
		})
	}

	return result
}

// findLineForBytePosition finds which line (1-based) a byte position is on
func (zasf *ZeroAllocSemanticFilter) findLineForBytePosition(fileID types.FileID, bytePos int) int {
	currentPos := 0

	// Iterate through all lines
	for i := 0; i < 10000; i++ { // Reasonable max lines limit
		lineRef := zasf.store.GetZeroAllocLine(fileID, i) // 0-based

		// Calculate line length (including newline)
		lineLen := lineRef.Len() + 1 // +1 for newline

		// Check if byte position is on this line
		if currentPos <= bytePos && bytePos < currentPos+lineLen {
			return i + 1 // Return 1-based line number
		}

		currentPos += lineLen

		// Stop if we've gone past the byte position
		if currentPos > bytePos+1000 {
			break
		}
	}

	return 1 // Default to line 1 if not found
}

// ConvertZeroAllocMatches converts zero-alloc matches back to traditional format
func (zasf *ZeroAllocSemanticFilter) ConvertZeroAllocMatches(matches []ZeroAllocSearchMatch) []searchtypes.Match {
	result := make([]searchtypes.Match, 0, len(matches))

	for _, match := range matches {
		result = append(result, searchtypes.Match{
			Start: match.Start,
			End:   match.End,
			Exact: false, // Default to false
		})
	}

	return result
}
