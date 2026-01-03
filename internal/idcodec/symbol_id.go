package idcodec

import (
	"github.com/standardbeagle/lci/internal/types"
)

// EncodeSymbolID encodes a SymbolID to a base-63 string.
// This is the canonical function for encoding symbol IDs throughout LCI.
//
// SymbolID in LCI is a raw uint64 index into the symbol store.
// It is NOT a packed FileID+LocalSymbolID (that's CompositeSymbolID).
func EncodeSymbolID(id types.SymbolID) string {
	return Encode(uint64(id))
}

// DecodeSymbolID decodes a base-63 string to a SymbolID.
// Returns error for invalid input.
func DecodeSymbolID(encoded string) (types.SymbolID, error) {
	value, err := Decode(encoded)
	if err != nil {
		return 0, err
	}
	return types.SymbolID(value), nil
}

// MustDecodeSymbolID decodes a base-63 string to a SymbolID.
// Panics on error - use only when the input is known to be valid.
func MustDecodeSymbolID(encoded string) types.SymbolID {
	id, err := DecodeSymbolID(encoded)
	if err != nil {
		panic("idcodec: MustDecodeSymbolID: " + err.Error())
	}
	return id
}

// IsValidSymbolID checks if a string is a valid base-63 encoded SymbolID.
func IsValidSymbolID(encoded string) bool {
	return IsValid(encoded)
}

// EncodeFileID encodes a FileID to a base-63 string.
func EncodeFileID(id types.FileID) string {
	return Encode(uint64(id))
}

// DecodeFileID decodes a base-63 string to a FileID.
func DecodeFileID(encoded string) (types.FileID, error) {
	value, err := Decode(encoded)
	if err != nil {
		return 0, err
	}
	// FileID is uint32, check for overflow
	if value > uint64(^types.FileID(0)) {
		return 0, ErrOverflow
	}
	return types.FileID(value), nil
}
