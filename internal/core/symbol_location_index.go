package core

import (
	"sync"
	"sync/atomic"

	"github.com/standardbeagle/lci/internal/types"
)

// SymbolLocationIndex provides instant symbol lookup by file position
// Uses a 2-level spatial index: file → line → column → symbol for O(1) lookup
type SymbolLocationIndex struct {
	// Per-file symbol locations with spatial indexing
	locations map[types.FileID]*FileSymbolMap
	mu        sync.RWMutex

	// Flag to indicate bulk indexing mode (lock-free when true)
	BulkIndexing int32
}

// FileSymbolMap stores all symbols in a file with spatial indexing
type FileSymbolMap struct {
	// Primary index: line number → symbols that start on that line
	symbolsByLine map[int][]PositionedSymbol
	// Secondary index: line → column range → symbol ID for fast range queries
	// This enables O(1) lookup for most cases
	lineIndex map[int]*LineSymbolIndex
}

// LineSymbolIndex provides fast symbol lookup within a specific line
type LineSymbolIndex struct {
	// Column range → SymbolID for direct lookup
	columnRanges map[ColumnRange]types.SymbolID
	// Fallback list for complex queries
	symbols []PositionedSymbol
}

// ColumnRange represents a column span
type ColumnRange struct {
	Start int
	End   int
}

// Eq checks if two ColumnRanges are equal
func (cr ColumnRange) Eq(other ColumnRange) bool {
	return cr.Start == other.Start && cr.End == other.End
}

// PositionedSymbol represents a symbol with its exact position
type PositionedSymbol struct {
	Symbol   types.Symbol
	SymbolID types.SymbolID // Store ID for O(1) lookup
	StartPos Position
	EndPos   Position
}

// SymbolRange represents the full range a symbol covers
type SymbolRange struct {
	Symbol   types.Symbol
	SymbolID types.SymbolID // Store ID for O(1) lookup
	StartPos Position
	EndPos   Position
}

// Position represents a line/column coordinate
type Position struct {
	Line   int
	Column int
}

// NewSymbolLocationIndex creates a new symbol location index
func NewSymbolLocationIndex() *SymbolLocationIndex {
	return &SymbolLocationIndex{
		locations: make(map[types.FileID]*FileSymbolMap),
	}
}

// IndexFileSymbols indexes all symbols in a file for instant position-based lookup
// enhancedSymbols can be nil for basic symbols (will generate temporary IDs)
func (sli *SymbolLocationIndex) IndexFileSymbols(fileID types.FileID, symbols []types.Symbol, enhancedSymbols []*types.EnhancedSymbol) {
	// Only acquire lock if not in bulk indexing mode (multiple callers)
	// During indexing, FileIntegrator is the only writer (lock-free)
	if atomic.LoadInt32(&sli.BulkIndexing) == 0 {
		sli.mu.Lock()
		defer sli.mu.Unlock()
	}

	// Create file symbol map with spatial indexing
	fileMap := &FileSymbolMap{
		symbolsByLine: make(map[int][]PositionedSymbol),
		lineIndex:     make(map[int]*LineSymbolIndex),
	}

	// Index each symbol by its position
	// OPTIMIZATION: Use index-based matching instead of fmt.Sprintf key lookup
	// Symbols and enhancedSymbols arrays are in the same order (from parser/processor)
	// This eliminates 2n allocations per file from fmt.Sprintf
	for i, symbol := range symbols {
		// Get symbol ID directly from enhanced symbols array by index
		// This is O(1) with zero allocations vs O(n) map lookups with string allocations
		var symbolID types.SymbolID
		if enhancedSymbols != nil && i < len(enhancedSymbols) && enhancedSymbols[i] != nil {
			symbolID = enhancedSymbols[i].ID
		} else {
			// Fallback: generate a temporary ID based on position (shouldn't happen in normal operation)
			// Use bit-packing instead of hash for faster computation
			symbolID = types.SymbolID(uint64(symbol.Line)<<32 | uint64(symbol.Column)<<16 | uint64(i))
		}

		positioned := PositionedSymbol{
			Symbol:   symbol,
			SymbolID: symbolID,
			StartPos: Position{
				Line:   symbol.Line,
				Column: symbol.Column,
			},
			EndPos: Position{
				Line:   symbol.EndLine,
				Column: symbol.EndColumn,
			},
		}

		// Add to line-based index
		fileMap.symbolsByLine[symbol.Line] = append(fileMap.symbolsByLine[symbol.Line], positioned)

		// Build line index for O(1) lookup
		lineIndex, exists := fileMap.lineIndex[symbol.Line]
		if !exists {
			lineIndex = &LineSymbolIndex{
				columnRanges: make(map[ColumnRange]types.SymbolID),
				symbols:      make([]PositionedSymbol, 0, 4),
			}
			fileMap.lineIndex[symbol.Line] = lineIndex
		}
		lineIndex.symbols = append(lineIndex.symbols, positioned)
		lineIndex.columnRanges[ColumnRange{symbol.Column, symbol.EndColumn}] = symbolID
	}

	sli.locations[fileID] = fileMap
}

// FindSymbolAtPosition finds the symbol at a specific line/column in a file
// Returns the most specific symbol (smallest range) that contains the position
func (sli *SymbolLocationIndex) FindSymbolAtPosition(fileID types.FileID, line, column int) *types.Symbol {
	sli.mu.RLock()
	defer sli.mu.RUnlock()

	fileMap, exists := sli.locations[fileID]
	if !exists {
		return nil
	}

	target := Position{Line: line, Column: column}
	var bestMatch *PositionedSymbol
	var bestSize int = -1

	// Check symbols starting at this line (fast path for single-line symbols)
	if lineIndex, exists := fileMap.lineIndex[line]; exists {
		for i := range lineIndex.symbols {
			positioned := &lineIndex.symbols[i]
			// OPTIMIZED: Inline containsPositionOnLine to reduce function call overhead
			// Check if position is within the symbol's line range
			if target.Line < positioned.StartPos.Line || target.Line > positioned.EndPos.Line {
				continue
			}
			// If on the start line, check column is at or after start column
			if target.Line == positioned.StartPos.Line && target.Column < positioned.StartPos.Column {
				continue
			}
			// If on the end line, check column is at or before end column (inclusive)
			if target.Line == positioned.EndPos.Line && target.Column > positioned.EndPos.Column {
				continue
			}

			// Calculate symbol size (prefer smaller/more specific symbols)
			size := (positioned.EndPos.Line-positioned.StartPos.Line)*1000 +
				(positioned.EndPos.Column - positioned.StartPos.Column)

			if bestMatch == nil || size < bestSize {
				bestMatch = positioned
				bestSize = size
			}
		}
	}

	// If we found a perfect match, return it
	if bestMatch != nil && bestMatch.StartPos.Line == line {
		return &bestMatch.Symbol
	}

	// Check all symbols that might span this line
	// For efficiency, check a window around the target line
	for lineOffset := -5; lineOffset <= 5; lineOffset++ {
		checkLine := line + lineOffset
		if checkLine <= 0 {
			continue
		}

		if lineIndex, exists := fileMap.lineIndex[checkLine]; exists {
			for i := range lineIndex.symbols {
				positioned := &lineIndex.symbols[i]
				// Only check if this symbol could span the target line
				if positioned.StartPos.Line > line || positioned.EndPos.Line < line {
					continue
				}

				// OPTIMIZED: Inline containsPositionOnLine to reduce function call overhead
				// Check if position is within the symbol's line range
				if target.Line < positioned.StartPos.Line || target.Line > positioned.EndPos.Line {
					continue
				}
				// If on the start line, check column is at or after start column
				if target.Line == positioned.StartPos.Line && target.Column < positioned.StartPos.Column {
					continue
				}
				// If on the end line, check column is at or before end column (inclusive)
				if target.Line == positioned.EndPos.Line && target.Column > positioned.EndPos.Column {
					continue
				}

				// Calculate symbol size (prefer smaller/more specific symbols)
				size := (positioned.EndPos.Line-positioned.StartPos.Line)*1000 +
					(positioned.EndPos.Column - positioned.StartPos.Column)

				if bestMatch == nil || size < bestSize {
					bestMatch = positioned
					bestSize = size
				}
			}
		}
	}

	if bestMatch != nil {
		return &bestMatch.Symbol
	}
	return nil
}

// FindSymbolIDAtPosition finds the symbol ID at a specific line/column in a file
// Returns the most specific symbol ID (smallest range) that contains the position
// Uses spatial indexing for O(1) average case lookup
func (sli *SymbolLocationIndex) FindSymbolIDAtPosition(fileID types.FileID, line, column int) types.SymbolID {
	sli.mu.RLock()
	defer sli.mu.RUnlock()

	fileMap, exists := sli.locations[fileID]
	if !exists {
		return 0
	}

	target := Position{Line: line, Column: column}
	var bestMatch *PositionedSymbol
	var bestSize int = -1

	// Check symbols starting at this line (fast path for single-line symbols)
	if lineIndex, exists := fileMap.lineIndex[line]; exists {
		for i := range lineIndex.symbols {
			positioned := &lineIndex.symbols[i]
			// OPTIMIZED: Inline containsPositionOnLine to reduce function call overhead
			// Check if position is within the symbol's line range
			if target.Line < positioned.StartPos.Line || target.Line > positioned.EndPos.Line {
				continue
			}
			// If on the start line, check column is at or after start column
			if target.Line == positioned.StartPos.Line && target.Column < positioned.StartPos.Column {
				continue
			}
			// If on the end line, check column is at or before end column (inclusive)
			if target.Line == positioned.EndPos.Line && target.Column > positioned.EndPos.Column {
				continue
			}

			// Calculate symbol size (prefer smaller/more specific symbols)
			size := (positioned.EndPos.Line-positioned.StartPos.Line)*1000 +
				(positioned.EndPos.Column - positioned.StartPos.Column)

			if bestMatch == nil || size < bestSize {
				bestMatch = positioned
				bestSize = size
			}
		}
	}

	// If we found a perfect match, return it
	if bestMatch != nil && bestMatch.StartPos.Line == line {
		return bestMatch.SymbolID
	}

	// Check all symbols that might span this line
	// For efficiency, check a window around the target line
	for lineOffset := -5; lineOffset <= 5; lineOffset++ {
		checkLine := line + lineOffset
		if checkLine <= 0 {
			continue
		}

		if lineIndex, exists := fileMap.lineIndex[checkLine]; exists {
			for i := range lineIndex.symbols {
				positioned := &lineIndex.symbols[i]
				// Only check if this symbol could span the target line
				if positioned.StartPos.Line > line || positioned.EndPos.Line < line {
					continue
				}

				// OPTIMIZED: Inline containsPositionOnLine to reduce function call overhead
				// Check if position is within the symbol's line range
				if target.Line < positioned.StartPos.Line || target.Line > positioned.EndPos.Line {
					continue
				}
				// If on the start line, check column is at or after start column
				if target.Line == positioned.StartPos.Line && target.Column < positioned.StartPos.Column {
					continue
				}
				// If on the end line, check column is at or before end column (inclusive)
				if target.Line == positioned.EndPos.Line && target.Column > positioned.EndPos.Column {
					continue
				}

				// Calculate symbol size (prefer smaller/more specific symbols)
				size := (positioned.EndPos.Line-positioned.StartPos.Line)*1000 +
					(positioned.EndPos.Column - positioned.StartPos.Column)

				if bestMatch == nil || size < bestSize {
					bestMatch = positioned
					bestSize = size
				}
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.SymbolID
	}
	return 0
}

// containsPosition checks if a symbol range contains a position
func (sli *SymbolLocationIndex) containsPosition(symbolRange *SymbolRange, pos Position) bool {
	// Check if position is within the symbol's range
	if pos.Line < symbolRange.StartPos.Line || pos.Line > symbolRange.EndPos.Line {
		return false
	}

	// If on the start line, check column is at or after start column
	if pos.Line == symbolRange.StartPos.Line && pos.Column < symbolRange.StartPos.Column {
		return false
	}

	// If on the end line, check column is at or before end column (inclusive)
	if pos.Line == symbolRange.EndPos.Line && pos.Column > symbolRange.EndPos.Column {
		return false
	}

	return true
}

// GetFileSymbols returns all symbols in a file
func (sli *SymbolLocationIndex) GetFileSymbols(fileID types.FileID) []types.Symbol {
	sli.mu.RLock()
	defer sli.mu.RUnlock()

	fileMap, exists := sli.locations[fileID]
	if !exists {
		return nil
	}

	var symbols []types.Symbol
	// Collect all symbols from line index
	for _, lineIndex := range fileMap.lineIndex {
		for _, positioned := range lineIndex.symbols {
			symbols = append(symbols, positioned.Symbol)
		}
	}
	return symbols
}

// Clear removes all indexed symbols
func (sli *SymbolLocationIndex) Clear() {
	sli.mu.Lock()
	defer sli.mu.Unlock()
	sli.locations = make(map[types.FileID]*FileSymbolMap)
}

// RemoveFile removes all symbols for a specific file
func (sli *SymbolLocationIndex) RemoveFile(fileID types.FileID) {
	sli.mu.Lock()
	defer sli.mu.Unlock()
	delete(sli.locations, fileID)
}

// GetStats returns statistics about the symbol location index
func (sli *SymbolLocationIndex) GetStats() map[string]interface{} {
	sli.mu.RLock()
	defer sli.mu.RUnlock()

	totalSymbols := 0
	linesIndexed := 0
	for _, fileMap := range sli.locations {
		for _, lineIndex := range fileMap.lineIndex {
			totalSymbols += len(lineIndex.symbols)
			linesIndexed++
		}
	}

	return map[string]interface{}{
		"indexed_files": len(sli.locations),
		"total_symbols": totalSymbols,
		"indexed_lines": linesIndexed,
		"avg_per_file":  float64(totalSymbols) / float64(len(sli.locations)),
		"avg_per_line":  float64(totalSymbols) / float64(linesIndexed),
	}
}

// Shutdown performs graceful shutdown with resource cleanup
func (sli *SymbolLocationIndex) Shutdown() error {
	// Clear all data
	sli.Clear()
	return nil
}
