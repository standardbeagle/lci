package searchtypes

import (
	"errors"
	"fmt"

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

// EncodeSymbolID encodes a SymbolID as a dense base-63 string
func EncodeSymbolID(symbolID types.SymbolID) string {
	if symbolID == 0 {
		return "A" // Minimum non-empty encoding
	}

	id := uint64(symbolID)
	var result []byte
	const base = 63

	// Encode using base-63 (A-Za-z0-9_)
	for id > 0 {
		val := id % base
		var c byte
		if val < 26 {
			c = byte('A' + val)
		} else if val < 52 {
			c = byte('a' + (val - 26))
		} else if val < 62 {
			c = byte('0' + (val - 52))
		} else {
			c = '_'
		}
		result = append(result, c)
		id /= base
	}

	// Reverse for correct order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// DecodeSymbolID decodes a base-63 string back to a SymbolID
func DecodeSymbolID(encoded string) (types.SymbolID, error) {
	if encoded == "" {
		return 0, errors.New("empty encoded string")
	}

	var id uint64
	const base = 63

	for _, c := range encoded {
		var val uint64
		if c >= 'A' && c <= 'Z' {
			val = uint64(c - 'A')
		} else if c >= 'a' && c <= 'z' {
			val = uint64(c-'a') + 26
		} else if c >= '0' && c <= '9' {
			val = uint64(c-'0') + 52
		} else if c == '_' {
			val = 62
		} else {
			return 0, fmt.Errorf("invalid character in encoded string: %c", c)
		}
		id = id*base + val
	}

	return types.SymbolID(id), nil
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
