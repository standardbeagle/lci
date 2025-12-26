package indexing

import (
	"github.com/standardbeagle/lci/internal/types"
)

// SymbolSemanticData contains pre-computed semantic information for a single symbol
// This is collected during the map phase (per-file processing)
type SymbolSemanticData struct {
	SymbolID   types.SymbolID `json:"symbol_id"`
	Name       string         `json:"name"`
	Words      []string       `json:"words"`      // Pre-split words
	Stems      []string       `json:"stems"`      // Pre-computed word stems
	Phonetic   string         `json:"phonetic"`   // Phonetic code for fuzzy matching
	Expansions []string       `json:"expansions"` // Abbreviation expansions
}

// SemanticMapResult contains semantic data for all symbols in a file
// This is the output of the map phase for semantic indexing
type SemanticMapResult struct {
	FileID  types.SymbolID       `json:"file_id"` // The file ID that generated this result
	Symbols []SymbolSemanticData `json:"symbols"` // All symbols in the file with their semantic data
}

// NewSemanticMapResult creates a new semantic map result for a file
func NewSemanticMapResult(fileID types.SymbolID) *SemanticMapResult {
	return &SemanticMapResult{
		FileID:  fileID,
		Symbols: make([]SymbolSemanticData, 0),
	}
}

// AddSymbol adds semantic data for a symbol to this map result
func (smr *SemanticMapResult) AddSymbol(
	symbolID types.SymbolID,
	name string,
	words, stems, expansions []string,
	phonetic string,
) {
	smr.Symbols = append(smr.Symbols, SymbolSemanticData{
		SymbolID:   symbolID,
		Name:       name,
		Words:      words,
		Stems:      stems,
		Phonetic:   phonetic,
		Expansions: expansions,
	})
}

// SymbolCount returns the number of symbols in this map result
func (smr *SemanticMapResult) SymbolCount() int {
	return len(smr.Symbols)
}

// IsEmpty returns true if this map result contains no symbols
func (smr *SemanticMapResult) IsEmpty() bool {
	return len(smr.Symbols) == 0
}
