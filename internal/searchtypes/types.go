package searchtypes

import (
	"github.com/standardbeagle/lci/internal/idcodec"
	"github.com/standardbeagle/lci/internal/types"
)

// GrepResult represents a basic search result (grep-like, fast)
type GrepResult struct {
	FileID         types.FileID     `json:"file_id"`
	Path           string           `json:"path"`
	Line           int              `json:"line"`
	Column         int              `json:"column"`
	Match          string           `json:"match"`
	Context        ExtractedContext `json:"context"`
	Score          float64          `json:"score"`
	FileMatchCount int              `json:"file_match_count,omitempty"` // Total matches in this file (for CountPerFile mode)
}

// ExtractedContext represents the context around a search match
type ExtractedContext struct {
	Lines      []string `json:"lines"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	BlockType  string   `json:"block_type,omitempty"`
	BlockName  string   `json:"block_name,omitempty"`
	IsComplete bool     `json:"is_complete"`

	// New fields to track all matches in the context
	MatchedLines []int `json:"matched_lines,omitempty"` // Line numbers that contain matches (1-based)
	MatchCount   int   `json:"match_count,omitempty"`   // Total number of matches in this context
}

// StandardResult includes relational data with the search result (index-optimized)
type StandardResult struct {
	Result         GrepResult               `json:"result"`
	RelationalData *types.RelationalContext `json:"relational_data,omitempty"`
	ObjectID       string                   `json:"object_id,omitempty"` // Dense object ID for context lookup
}

// Type aliases for cleaner API naming
type Result = GrepResult             // Alias: GrepResult is the canonical type
type DetailedResult = StandardResult // Alias: StandardResult is the canonical type

// Match represents a byte-level match in the content
type Match struct {
	Start  int
	End    int
	Exact  bool
	FileID types.FileID // Source file (set by hybrid regex engine)
}

// SearchOptions configures search behavior
type SearchOptions struct {
	MaxResults         int
	CaseInsensitive    bool
	WholeWord          bool
	UseRegex           bool
	IncludeTests       bool
	FilePattern        string
	MaxContextLines    int
	MergeFileResults   bool
	EnsureCompleteStmt bool
	IncludeObjectIDs   bool // Include dense object IDs for context search (default: true)
}

// PopulateDenseObjectIDs adds dense symbol-based object IDs to search results
// Encodes the actual SymbolID as a dense string (A-Za-z0-9_)
func PopulateDenseObjectIDs(results []StandardResult) {
	for i := range results {
		// Only generate ObjectID if we have relational data with a valid symbol ID
		if results[i].RelationalData != nil && results[i].RelationalData.Symbol.ID > 0 {
			symbolID := results[i].RelationalData.Symbol.ID

			// Extract LocalSymbolID from the packed SymbolID (lower 32 bits)
			// Format: FileID in upper 32 bits, LocalSymbolID in lower 32 bits
			localSymbolID := uint32(symbolID & 0xFFFFFFFF)

			// Skip if LocalSymbolID is zero (invalid symbol)
			if localSymbolID == 0 {
				continue
			}

			// Use a simple base-63 encoding of the SymbolID directly
			results[i].ObjectID = EncodeSymbolID(symbolID)
		}
		// Otherwise leave ObjectID empty (for plain text matches without symbols)
	}
}

// EncodeSymbolID encodes a SymbolID as a dense base-63 string.
// Delegates to idcodec for the canonical implementation.
func EncodeSymbolID(symbolID types.SymbolID) string {
	return idcodec.EncodeSymbolID(symbolID)
}

// DecodeSymbolID decodes a base-63 string back to a SymbolID.
// Delegates to idcodec for the canonical implementation.
func DecodeSymbolID(encoded string) (types.SymbolID, error) {
	return idcodec.DecodeSymbolID(encoded)
}

// DefaultSearchOptions returns search options with sensible defaults
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		MaxResults:         100,
		CaseInsensitive:    false,
		WholeWord:          false,
		UseRegex:           false,
		IncludeTests:       true,
		FilePattern:        "",
		MaxContextLines:    3,
		MergeFileResults:   true,
		EnsureCompleteStmt: false,
		IncludeObjectIDs:   true, // Default enabled for easy migration
	}
}
