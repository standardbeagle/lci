// Package encoding provides low-level encoding utilities with no dependencies.
// This is the foundational package for ID encoding used throughout LCI.
//
// Base-63 Alphabet: A-Z (0-25), a-z (26-51), 0-9 (52-61), _ (62)
// This provides ~6 character IDs for typical projects (vs ~16 for hex).
package encoding

import (
	"errors"
	"fmt"
)

// Base-63 encoding constants
const (
	Base63     = 63
	Alphabet63 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"
)

// Common errors for encoding operations
var (
	ErrEmptyString = errors.New("empty encoded string")
	ErrInvalidChar = errors.New("invalid character in encoded string")
	ErrOverflow    = errors.New("decoded value overflow")
)

// Base63Encode encodes a uint64 value to a base-63 string.
// Returns "A" for zero (minimum non-empty encoding).
func Base63Encode(value uint64) string {
	if value == 0 {
		return "A" // Minimum non-empty encoding for zero
	}

	// Pre-allocate buffer for typical ID lengths (11 chars max for uint64)
	var buf [11]byte
	pos := len(buf)

	for value > 0 {
		pos--
		buf[pos] = Alphabet63[value%Base63]
		value /= Base63
	}

	return string(buf[pos:])
}

// Base63EncodeNoZero encodes a uint64 value to a base-63 string.
// Returns empty string for zero value (used for composite IDs where 0 means "none").
func Base63EncodeNoZero(value uint64) string {
	if value == 0 {
		return ""
	}
	return Base63Encode(value)
}

// Base63Decode decodes a base-63 string to a uint64 value.
// Returns error for empty strings or invalid characters.
func Base63Decode(encoded string) (uint64, error) {
	if encoded == "" {
		return 0, ErrEmptyString
	}

	var value uint64

	for _, c := range encoded {
		charVal, err := Base63CharToValue(c)
		if err != nil {
			return 0, err
		}

		// Check for overflow before multiplication
		if value > (^uint64(0))/Base63 {
			return 0, ErrOverflow
		}
		value = value*Base63 + charVal
	}

	return value, nil
}

// Base63IsValid checks if a string is a valid base-63 encoded value.
func Base63IsValid(encoded string) bool {
	if encoded == "" {
		return false
	}
	for _, c := range encoded {
		if _, err := Base63CharToValue(c); err != nil {
			return false
		}
	}
	return true
}

// Base63CharToValue converts a character to its base-63 numeric value (0-62).
func Base63CharToValue(c rune) (uint64, error) {
	switch {
	case c >= 'A' && c <= 'Z':
		return uint64(c - 'A'), nil
	case c >= 'a' && c <= 'z':
		return uint64(c-'a') + 26, nil
	case c >= '0' && c <= '9':
		return uint64(c-'0') + 52, nil
	case c == '_':
		return 62, nil
	default:
		return 0, fmt.Errorf("%w: %c", ErrInvalidChar, c)
	}
}

// Base63ValueToChar converts a base-63 numeric value (0-62) to its character.
// Panics if value > 62 (internal error, should never happen with valid input).
func Base63ValueToChar(val uint64) byte {
	if val < 26 {
		return byte('A' + val)
	} else if val < 52 {
		return byte('a' + (val - 26))
	} else if val < 62 {
		return byte('0' + (val - 52))
	} else if val == 62 {
		return '_'
	}
	panic(fmt.Sprintf("encoding: invalid base63 value %d", val))
}

// PackUint32Pair packs two uint32 values into a single uint64.
// lower goes into lower 32 bits, upper goes into upper 32 bits.
func PackUint32Pair(lower, upper uint32) uint64 {
	return uint64(lower) | (uint64(upper) << 32)
}

// UnpackUint32Pair unpacks a uint64 into two uint32 values.
// Returns (lower 32 bits, upper 32 bits).
func UnpackUint32Pair(packed uint64) (lower, upper uint32) {
	lower = uint32(packed & 0xFFFFFFFF)
	upper = uint32((packed >> 32) & 0xFFFFFFFF)
	return
}
