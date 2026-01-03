package idcodec

import (
	"github.com/standardbeagle/lci/internal/encoding"
	"github.com/standardbeagle/lci/internal/types"
)

// CompositeSymbolID packing:
// - Lower 32 bits: FileID
// - Upper 32 bits: LocalSymbolID
//
// This is different from raw SymbolID which is just an index.
// CompositeSymbolID is used when you need to reference both file and local symbol.

// EncodeComposite encodes a FileID and LocalSymbolID into a single base-63 string.
// The values are packed as: uint64(fileID) | (uint64(localSymbolID) << 32)
func EncodeComposite(fileID types.FileID, localSymbolID uint32) string {
	combined := encoding.PackUint32Pair(uint32(fileID), localSymbolID)
	return EncodeNoZero(combined)
}

// DecodeComposite decodes a base-63 string to FileID and LocalSymbolID.
// Returns error for invalid input.
func DecodeComposite(encoded string) (types.FileID, uint32, error) {
	if encoded == "" {
		return 0, 0, ErrEmptyString
	}

	combined, err := Decode(encoded)
	if err != nil {
		return 0, 0, err
	}

	lower, upper := encoding.UnpackUint32Pair(combined)
	return types.FileID(lower), upper, nil
}

// PackComposite packs FileID and LocalSymbolID into a uint64.
// Use this when you need the raw packed value.
func PackComposite(fileID types.FileID, localSymbolID uint32) uint64 {
	return encoding.PackUint32Pair(uint32(fileID), localSymbolID)
}

// UnpackComposite unpacks a uint64 into FileID and LocalSymbolID.
func UnpackComposite(packed uint64) (types.FileID, uint32) {
	lower, upper := encoding.UnpackUint32Pair(packed)
	return types.FileID(lower), upper
}
