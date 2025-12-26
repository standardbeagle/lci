package core

import (
	"errors"
	"fmt"
	"github.com/standardbeagle/lci/internal/types"
)

// DenseObjectID provides high-density symbol-based encoding for code objects
// Uses variable-length encoding with full character set for maximum density
// Target: 3-8 character IDs using A-Za-z0-9_ (62 symbols) for human readability
type DenseObjectID struct {
	encoded string
}

// encodeChar converts a value 0-62 to its character representation
// 0-25: A-Z, 26-51: a-z, 52-61: 0-9, 62: _
func encodeChar(val uint64) byte {
	if val < 26 {
		return byte('A' + val)
	} else if val < 52 {
		return byte('a' + (val - 26))
	} else if val < 62 {
		return byte('0' + (val - 52))
	} else if val == 62 {
		return '_'
	}
	panic("invalid encode value")
}

// decodeChar converts a character to its value 0-62
func decodeChar(c byte) (uint64, error) {
	if c >= 'A' && c <= 'Z' {
		return uint64(c - 'A'), nil
	} else if c >= 'a' && c <= 'z' {
		return uint64(c - 'a' + 26), nil
	} else if c >= '0' && c <= '9' {
		return uint64(c - '0' + 52), nil
	} else if c == '_' {
		return 62, nil
	}
	return 0, fmt.Errorf("invalid character '%c' in dense encoding", c)
}

// encodeSymbol maps file ID and local symbol ID to dense string
func encodeSymbol(fileID uint32, localSymbolID uint32) string {
	// Combine 32-bit FileID and 32-bit LocalSymbolID into 64-bit value
	// FileID in lower 32 bits, LocalSymbolID in upper 32 bits
	combined := uint64(fileID) | (uint64(localSymbolID) << 32)

	if combined == 0 {
		return "" // Return empty for zero values
	}

	// Use base-63 encoding for maximum density
	var result []byte
	const base = 63

	// Encode the entire combined value
	for combined > 0 {
		result = append(result, encodeChar(combined%base))
		combined /= base
	}

	// Reverse since we encoded least significant digits first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// decodeSymbol extracts file ID and local symbol ID from dense string
func decodeSymbol(encoded string) (types.FileID, uint32, error) {
	if encoded == "" {
		return 0, 0, errors.New("empty encoded symbol")
	}

	var combined uint64
	const base = 63

	// Decode from most significant to least significant
	for i := 0; i < len(encoded); i++ {
		charValue, err := decodeChar(encoded[i])
		if err != nil {
			return 0, 0, err
		}
		combined = combined*base + charValue
	}

	// Extract components
	// FileID is in lower 32 bits, LocalSymbolID in upper 32 bits
	fileID := types.FileID(combined & 0xFFFFFFFF)
	localSymbolID := uint32((combined >> 32) & 0xFFFFFFFF)

	return fileID, localSymbolID, nil
}

// NewDenseObjectID creates a dense object ID from file ID and local symbol ID
func NewDenseObjectID(fileID uint32, localSymbolID uint32, symbolType uint32) DenseObjectID {
	encoded := encodeSymbol(fileID, localSymbolID)
	return DenseObjectID{encoded: encoded}
}

// String returns the dense encoded string
func (doid DenseObjectID) String() string {
	return doid.encoded
}

// ExtractFileID extracts the file ID from dense object ID
func (doid DenseObjectID) ExtractFileID() types.FileID {
	fileID, _, err := decodeSymbol(doid.encoded)
	if err != nil {
		return 0 // Default fallback
	}
	return fileID
}

// ExtractLocalSymbolID extracts the local symbol ID from dense object ID
func (doid DenseObjectID) ExtractLocalSymbolID() uint32 {
	_, localSymbolID, err := decodeSymbol(doid.encoded)
	if err != nil {
		return 0 // Default fallback
	}
	return localSymbolID
}

// ExtractSymbolType extracts the symbol type (not encoded in dense format)
func (doid DenseObjectID) ExtractSymbolType() uint8 {
	return 0 // Dense format doesn't encode type, return 0 as placeholder
}

// IsValid checks if the dense object ID is valid
func (doid DenseObjectID) IsValid() bool {
	_, _, err := decodeSymbol(doid.encoded)
	return err == nil
}

// DenseStats provides information about the dense object ID system
func DenseStats() map[string]interface{} {
	return map[string]interface{}{
		"encoding_type":                  "base-63 dense",
		"alphabet":                       "A-Za-z0-9_",
		"alphabet_size":                  63,
		"max_supported_files":            4294967295, // Full uint32 range
		"max_supported_symbols_per_file": 4294967295, // Full uint32 range
		"avg_id_length":                  "~6 chars for typical projects",
		"max_id_length":                  "11 chars for maximum values", // 64-bit in base-63
		"compression_ratio":              "~70% vs hex encoding",
	}
}
