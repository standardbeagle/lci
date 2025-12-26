package types

import (
	"hash/fnv"
)

// StringRef is a lightweight, immutable reference to a substring within a file
type StringRef struct {
	FileID FileID // Which file this string comes from
	Offset uint32 // Byte offset in the file
	Length uint32 // Number of bytes
	Hash   uint64 // Precomputed hash of the string content
}

// EmptyStringRef represents an empty/invalid string reference
var EmptyStringRef = StringRef{}

// NewStringRef creates a new StringRef from the given content
func NewStringRef(fileID FileID, content []byte, start, length int) StringRef {
	// Ensure bounds are valid
	if start < 0 || length < 0 || start+length > len(content) {
		return StringRef{}
	}

	// Extract the actual content for hash computation
	actualContent := content[start : start+length]
	hash := computeHash(actualContent)

	return StringRef{
		FileID: fileID,
		Offset: uint32(start),
		Length: uint32(length),
		Hash:   hash,
	}
}

// NewStringRefWithLine creates a new StringRef with line number information
func NewStringRefWithLine(fileID FileID, content []byte, start, length, lineNum int) StringRef {
	ref := NewStringRef(fileID, content, start, length)
	// Note: LineNumber is not part of the core StringRef struct to keep it minimal
	return ref
}

// IsEmpty returns true if this is an empty/invalid reference
func (ref StringRef) IsEmpty() bool {
	return ref.Length == 0
}

// String returns the actual string content (allocates)
// Use sparingly - prefer zero-copy operations
func (ref StringRef) String(content []byte) string {
	if ref.Offset+ref.Length > uint32(len(content)) {
		return ""
	}
	return string(content[ref.Offset : ref.Offset+ref.Length])
}

// Equal performs fast equality check using hash first
func (ref StringRef) Equal(other StringRef) bool {
	// Fast path: different hashes = definitely not equal
	if ref.Hash != other.Hash {
		return false
	}
	// Same hash and different lengths = not equal (hash collision)
	if ref.Length != other.Length {
		return false
	}
	// Same file and location = definitely equal
	if ref.FileID == other.FileID && ref.Offset == other.Offset {
		return true
	}
	// Need slow path comparison (handled by FileContentStore)
	return false
}

// QuickCompare provides fast comparison for sorting
func (ref StringRef) QuickCompare(other StringRef) int {
	if ref.Hash < other.Hash {
		return -1
	}
	if ref.Hash > other.Hash {
		return 1
	}
	// Equal hashes - compare by file and position for stable sort
	if ref.FileID < other.FileID {
		return -1
	}
	if ref.FileID > other.FileID {
		return 1
	}
	if ref.Offset < other.Offset {
		return -1
	}
	if ref.Offset > other.Offset {
		return 1
	}
	return 0
}

// Contains checks if the given byte offset falls within this reference
func (ref StringRef) Contains(offset uint32) bool {
	return offset >= ref.Offset && offset < ref.Offset+ref.Length
}

// Overlaps checks if two references overlap (must be in same file)
func (ref StringRef) Overlaps(other StringRef) bool {
	if ref.FileID != other.FileID {
		return false
	}
	return ref.Offset < other.Offset+other.Length && other.Offset < ref.Offset+ref.Length
}

// IsSameLocation checks if two refs point to exactly the same location
func (ref StringRef) IsSameLocation(other StringRef) bool {
	return ref.FileID == other.FileID &&
		ref.Offset == other.Offset &&
		ref.Length == other.Length
}

// ExtendTo creates a reference that spans from this ref to another
func (ref StringRef) ExtendTo(other StringRef) StringRef {
	if ref.FileID != other.FileID {
		return ref
	}
	start := min(ref.Offset, other.Offset)
	end := max(ref.Offset+ref.Length, other.Offset+other.Length)
	return StringRef{
		FileID: ref.FileID,
		Offset: start,
		Length: end - start,
		Hash:   0, // Needs recomputation by FileContentStore
	}
}

// Substring creates a sub-reference without accessing memory
func (ref StringRef) Substring(offset, length uint32) StringRef {
	if offset >= ref.Length {
		return EmptyStringRef
	}
	if offset+length > ref.Length {
		length = ref.Length - offset
	}
	return StringRef{
		FileID: ref.FileID,
		Offset: ref.Offset + offset,
		Length: length,
		Hash:   0, // Needs recomputation by FileContentStore
	}
}

// TrimPrefix removes n bytes from the start
func (ref StringRef) TrimPrefix(n uint32) StringRef {
	if n >= ref.Length {
		return EmptyStringRef
	}
	return StringRef{
		FileID: ref.FileID,
		Offset: ref.Offset + n,
		Length: ref.Length - n,
		Hash:   0, // Needs recomputation by FileContentStore
	}
}

// TrimSuffix removes n bytes from the end
func (ref StringRef) TrimSuffix(n uint32) StringRef {
	if n >= ref.Length {
		return EmptyStringRef
	}
	return StringRef{
		FileID: ref.FileID,
		Offset: ref.Offset,
		Length: ref.Length - n,
		Hash:   0, // Needs recomputation by FileContentStore
	}
}

// computeHash computes the FNV-1a hash for the given byte slice
func computeHash(data []byte) uint64 {
	// Fast path for short strings: use simple rolling hash
	if len(data) <= 32 {
		return fastHash(data)
	}

	// For longer strings, use FNV-1a
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}

// fastHash implements a simple but effective hash for short strings (<= 32 bytes)
func fastHash(data []byte) uint64 {
	var h uint64 = 14695981039346656037 // FNV offset basis

	for i := 0; i < len(data); i++ {
		h ^= uint64(data[i])
		h *= 1099511628211 // FNV prime
	}

	return h
}

// ComputeHash is the public version of computeHash for use in zero-allocation code
func ComputeHash(data []byte) uint64 {
	return computeHash(data)
}

// min returns the minimum of two uint32 values
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two uint32 values
func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}
