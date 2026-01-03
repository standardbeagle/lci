package core

import (
	"github.com/standardbeagle/lci/internal/encoding"
	"github.com/standardbeagle/lci/internal/types"
)

// DenseObjectID provides high-density symbol-based encoding for code objects
// Uses variable-length encoding with full character set for maximum density
// Target: 3-8 character IDs using A-Za-z0-9_ (62 symbols) for human readability
//
// Note: This type uses the consolidated encoding package for base-63 operations.
type DenseObjectID struct {
	encoded string
}

// encodeSymbol maps file ID and local symbol ID to dense string.
// Uses the consolidated encoding package.
func encodeSymbol(fileID uint32, localSymbolID uint32) string {
	combined := encoding.PackUint32Pair(fileID, localSymbolID)
	return encoding.Base63EncodeNoZero(combined)
}

// decodeSymbol extracts file ID and local symbol ID from dense string.
// Uses the consolidated encoding package.
func decodeSymbol(encoded string) (types.FileID, uint32, error) {
	if encoded == "" {
		return 0, 0, encoding.ErrEmptyString
	}

	combined, err := encoding.Base63Decode(encoded)
	if err != nil {
		return 0, 0, err
	}

	lower, upper := encoding.UnpackUint32Pair(combined)
	return types.FileID(lower), upper, nil
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
