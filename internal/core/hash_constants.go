package core

// FNV-1a hash constants for consistent hashing across the codebase.
// These are shared between reference_tracker.go and symbol_location_index.go
// to ensure consistent hashing behavior and avoid duplicate constant definitions.
//
// FNV-1a algorithm: hash = (hash XOR byte) * prime
// This provides excellent distribution for typical code symbol names and positions.
//
// References:
// - http://www.isthe.com/chongo/tech/comp/fnv/
// - https://en.wikipedia.org/wiki/Fowler%E2%80%93Noll%E2%80%93Vo_hash_function
const (
	// fnvOffset64 is the FNV-1a 64-bit offset basis
	fnvOffset64 = 14695981039346656037

	// fnvPrime64 is the FNV-1a 64-bit prime
	fnvPrime64 = 1099511628211
)

// HashStringFNV1a computes the FNV-1a hash of a string without allocations.
// This is a reusable helper for computing string hashes inline.
func HashStringFNV1a(s string) uint64 {
	h := uint64(fnvOffset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

// HashInt64FNV1a incorporates an int64 value into an existing FNV-1a hash.
// Returns the updated hash value.
func HashInt64FNV1a(h uint64, val int64) uint64 {
	h ^= uint64(val)
	h *= fnvPrime64
	return h
}

// NewFNV1aHash returns a new FNV-1a hash starting with the offset basis.
func NewFNV1aHash() uint64 {
	return uint64(fnvOffset64)
}
