package utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// ObjectID represents a parsed object identifier with type information
type ObjectID struct {
	Type     string // "symbol", "file", "reference"
	FileID   types.FileID
	SymbolID types.SymbolID
	Line     uint32
}

// ParseObjectID parses a base-63 encoded objectID string into structured data
// Supports formats:
//   - symbol:123 (SymbolID)
//   - file:123 (FileID)
//   - file:123+line:42 (FileID with line number)
//   - base63-encoded (A-Z, a-z, 0-9, _ like "GU", "ABC123")
//   - 123 (numeric backward compatibility)
func ParseObjectID(objectID string) (*ObjectID, error) {
	parts := strings.Split(objectID, ":")
	if len(parts) == 0 {
		return nil, errors.New("empty object ID")
	}

	// Handle prefixed format
	if len(parts) >= 1 {
		switch parts[0] {
		case "symbol":
			return parseSymbolObjectID(parts)
		case "file":
			return parseFileObjectID(parts)
		}
	}

	// Try base63 format first (e.g., "GU", "ABC123", "test_var")
	if parsed, err := parseBase63ObjectID(parts[0]); err == nil {
		return parsed, nil
	}

	// Try numeric format (backward compatibility)
	return parseNumericObjectID(parts[0])
}

// parseSymbolObjectID parses symbol:123 format
func parseSymbolObjectID(parts []string) (*ObjectID, error) {
	if len(parts) < 2 {
		return nil, errors.New("invalid symbol ID format: expected 'symbol:id'")
	}

	id, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol ID: %w", err)
	}

	return &ObjectID{
		Type:     "symbol",
		SymbolID: types.SymbolID(id),
	}, nil
}

// parseFileObjectID parses file:123 or file:123+line:42 format
func parseFileObjectID(parts []string) (*ObjectID, error) {
	if len(parts) < 2 {
		return nil, errors.New("invalid file ID format: expected 'file:id' or 'file:id+line:num'")
	}

	var fileID types.FileID
	var lineNum uint32

	// Extract file ID from parts[1] (format like "123+line:42")
	fileParts := strings.Split(parts[1], "+")
	if id, err := strconv.ParseUint(fileParts[0], 10, 32); err == nil {
		fileID = types.FileID(id)
	}

	// Extract line number if present
	for i := 1; i < len(parts); i++ {
		subParts := strings.Split(parts[i], "+")
		for j := 0; j < len(subParts); j++ {
			if strings.HasPrefix(subParts[j], "line:") {
				lineStr := strings.TrimPrefix(subParts[j], "line:")
				if ln, err := strconv.ParseUint(lineStr, 10, 32); err == nil {
					lineNum = uint32(ln)
				}
			}
		}
	}

	return &ObjectID{
		Type:   "file",
		FileID: fileID,
		Line:   lineNum,
	}, nil
}

// parseBase63ObjectID handles base63-encoded IDs (e.g., "GU", "ABC123", "test_var")
// This is the primary format for symbol IDs
func parseBase63ObjectID(idStr string) (*ObjectID, error) {
	// Try to decode as base63
	var id uint64
	const base = 63

	for _, c := range idStr {
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
			return nil, fmt.Errorf("invalid character in base63 ID: %c", c)
		}
		id = id*base + val
	}

	// Default to symbol for base63 IDs
	return &ObjectID{
		Type:     "symbol",
		SymbolID: types.SymbolID(id),
	}, nil
}

// parseNumericObjectID handles backward compatibility with numeric IDs
func parseNumericObjectID(idStr string) (*ObjectID, error) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid numeric ID: %w", err)
	}

	// Default to symbol for numeric IDs (backward compatibility)
	return &ObjectID{
		Type:     "symbol",
		SymbolID: types.SymbolID(id),
	}, nil
}

// ParseSymbolID decodes a base-63 encoded objectID to SymbolID
// Simple wrapper for backward compatibility
func ParseSymbolID(objectID string) (types.SymbolID, error) {
	parsed, err := ParseObjectID(objectID)
	if err != nil {
		return 0, err
	}
	return parsed.SymbolID, nil
}

// DecodeSymbolID decodes a base-63 encoded objectID to SymbolID
// Simple wrapper for backward compatibility - delegates to searchtypes
func DecodeSymbolID(objectID string) (types.SymbolID, error) {
	return searchtypes.DecodeSymbolID(objectID)
}

// ParseFileID extracts file ID and line number from object ID
// Returns (FileID, line number, error)
// Simple wrapper for backward compatibility
func ParseFileID(objectID string) (types.FileID, uint32, error) {
	parsed, err := ParseObjectID(objectID)
	if err != nil {
		return 0, 0, err
	}
	return parsed.FileID, parsed.Line, nil
}

// ParseObjectIDToSymbolID converts objectID string to SymbolID
// Simple wrapper for backward compatibility
func ParseObjectIDToSymbolID(objectID string) types.SymbolID {
	parsed, err := ParseObjectID(objectID)
	if err != nil {
		return 0
	}
	return parsed.SymbolID
}
