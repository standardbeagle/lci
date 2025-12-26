package core

import (
	"sort"

	"github.com/standardbeagle/lci/internal/types"
)

// ReferenceSpatialLocation represents a symbol's location in a file for spatial indexing
type ReferenceSpatialLocation struct {
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	SymbolID  types.SymbolID
}

// ReferenceSpatialIndex provides O(log n) spatial lookup of symbols by line/column
// Used specifically by ReferenceTracker for fast same-file reference resolution
type ReferenceSpatialIndex struct {
	locations []ReferenceSpatialLocation
	sorted    bool
	nameIndex map[string]types.SymbolID // O(1) lookup by symbol name
}

// NewReferenceSpatialIndex creates a new spatial index for a file's symbols
func NewReferenceSpatialIndex(capacity int) *ReferenceSpatialIndex {
	return &ReferenceSpatialIndex{
		locations: make([]ReferenceSpatialLocation, 0, capacity),
		sorted:    true, // Empty index is sorted
		nameIndex: make(map[string]types.SymbolID, capacity),
	}
}

// AddSymbol adds a symbol to the spatial index
func (rsi *ReferenceSpatialIndex) AddSymbol(symbolID types.SymbolID, symbolName string, startLine, endLine, startCol, endCol int) {
	rsi.locations = append(rsi.locations, ReferenceSpatialLocation{
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
		SymbolID:  symbolID,
	})
	rsi.sorted = false // Mark as needing sort

	// Add to name index for O(1) name-based lookups
	if symbolName != "" {
		rsi.nameIndex[symbolName] = symbolID
	}
}

// ensureSorted sorts the locations if needed (lazy sorting)
func (rsi *ReferenceSpatialIndex) ensureSorted() {
	if !rsi.sorted {
		sort.Slice(rsi.locations, func(i, j int) bool {
			// Sort by start line, then by start column
			if rsi.locations[i].StartLine != rsi.locations[j].StartLine {
				return rsi.locations[i].StartLine < rsi.locations[j].StartLine
			}
			return rsi.locations[i].StartCol < rsi.locations[j].StartCol
		})
		rsi.sorted = true
	}
}

// FindAtLocation finds the symbol at a specific line/column location
// Returns 0 if no symbol found at that location
// Time complexity: O(log n + k) where k is the number of overlapping symbols (typically 1-3)
func (rsi *ReferenceSpatialIndex) FindAtLocation(line, col int) types.SymbolID {
	rsi.ensureSorted()

	if len(rsi.locations) == 0 {
		return 0
	}

	// Binary search for first symbol whose end line >= target line
	idx := sort.Search(len(rsi.locations), func(i int) bool {
		return rsi.locations[i].EndLine >= line
	})

	// Scan forward from idx to find all candidates that could contain this location
	// This is typically very few symbols (1-5) even in large files
	var bestMatch types.SymbolID
	bestSpan := int(^uint(0) >> 1) // Max int

	for i := idx; i < len(rsi.locations); i++ {
		loc := &rsi.locations[i]

		// If we've gone past possible matches, stop
		if loc.StartLine > line {
			break
		}

		// Check if location is within this symbol's bounds
		if loc.StartLine <= line && loc.EndLine >= line {
			// Check column bounds for precision
			if loc.StartLine == line && col < loc.StartCol {
				continue // Before symbol starts on this line
			}
			if loc.EndLine == line && col > loc.EndCol {
				continue // After symbol ends on this line
			}

			// Found a match - prefer the smallest span (most specific symbol)
			span := loc.EndLine - loc.StartLine
			if span < bestSpan {
				bestSpan = span
				bestMatch = loc.SymbolID
			}
		}
	}

	return bestMatch
}

// FindByName finds a symbol by name in this file
// Returns 0 if no symbol with that name exists in this file
// Time complexity: O(1)
func (rsi *ReferenceSpatialIndex) FindByName(name string) types.SymbolID {
	if symbolID, exists := rsi.nameIndex[name]; exists {
		return symbolID
	}
	return 0
}

// Count returns the number of symbols in the index
func (rsi *ReferenceSpatialIndex) Count() int {
	return len(rsi.locations)
}

// Clear removes all symbols from the index
func (rsi *ReferenceSpatialIndex) Clear() {
	rsi.locations = rsi.locations[:0]
	rsi.sorted = true
}
