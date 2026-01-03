// Package idcodec provides consolidated ID encoding/decoding operations.
// This package delegates to internal/encoding for the core base-63 algorithm
// and provides type-safe wrappers for LCI-specific ID types.
//
// Base-63 Alphabet: A-Z (0-25), a-z (26-51), 0-9 (52-61), _ (62)
// This provides ~6 character IDs for typical projects (vs ~16 for hex).
package idcodec

import (
	"github.com/standardbeagle/lci/internal/encoding"
)

// Re-export constants from encoding package for convenience
const (
	Base     = encoding.Base63
	Alphabet = encoding.Alphabet63
)

// Re-export errors from encoding package for use with errors.Is
var (
	ErrEmptyString = encoding.ErrEmptyString
	ErrInvalidChar = encoding.ErrInvalidChar
	ErrOverflow    = encoding.ErrOverflow
)

// Encode encodes a uint64 value to a base-63 string.
// Returns "A" for zero (minimum non-empty encoding).
// Delegates to encoding.Base63Encode.
func Encode(value uint64) string {
	return encoding.Base63Encode(value)
}

// EncodeNoZero encodes a uint64 value to a base-63 string.
// Returns empty string for zero value (used for composite IDs where 0 means "none").
// Delegates to encoding.Base63EncodeNoZero.
func EncodeNoZero(value uint64) string {
	return encoding.Base63EncodeNoZero(value)
}

// Decode decodes a base-63 string to a uint64 value.
// Returns error for empty strings or invalid characters.
// Delegates to encoding.Base63Decode.
func Decode(encoded string) (uint64, error) {
	return encoding.Base63Decode(encoded)
}

// IsValid checks if a string is a valid base-63 encoded value.
// Delegates to encoding.Base63IsValid.
func IsValid(encoded string) bool {
	return encoding.Base63IsValid(encoded)
}

// charToValue is kept for internal use (delegates to encoding).
func charToValue(c rune) (uint64, error) {
	return encoding.Base63CharToValue(c)
}

// valueToChar is kept for internal use (delegates to encoding).
func valueToChar(val uint64) byte {
	return encoding.Base63ValueToChar(val)
}
