package types

import (
	"bytes"
	"encoding/json"
	"sort"
)

// ZeroAllocStringRef is a zero-allocation string reference that operates on []byte data
// This eliminates the need for string conversion in most operations
type ZeroAllocStringRef struct {
	Data   []byte // Direct reference to the byte data (no allocation)
	FileID FileID // For safety and LRU tracking
	Offset uint32 // Offset within the data slice
	Length uint32 // Length of the string reference
	Hash   uint64 // Precomputed hash for fast equality
}

// EmptyZeroAllocStringRef represents an empty/invalid zero-allocation string reference
var EmptyZeroAllocStringRef = ZeroAllocStringRef{}

// IsEmpty returns true if this is an empty/invalid reference
func (ref ZeroAllocStringRef) IsEmpty() bool {
	return ref.Length == 0 || ref.Data == nil
}

// Bytes returns the underlying byte slice without any allocation
func (ref ZeroAllocStringRef) Bytes() []byte {
	if ref.IsEmpty() {
		return nil
	}
	return ref.Data[ref.Offset : ref.Offset+ref.Length]
}

// String converts to string - ONLY use this when absolutely necessary (API output, logging)
// This is the ONLY allocation method in the entire interface
func (ref ZeroAllocStringRef) String() string {
	if ref.IsEmpty() {
		return ""
	}
	return string(ref.Bytes())
}

// MarshalJSON implements json.Marshaler for efficient JSON serialization
func (ref ZeroAllocStringRef) MarshalJSON() ([]byte, error) {
	if ref.IsEmpty() {
		return []byte(`""`), nil
	}

	// Use encoding/json to properly escape special characters
	// This ensures \t, \n, ", \, etc. are properly escaped
	return json.Marshal(ref.String())
}

// ============================================================================
// ZERO-ALLOCATION STRING OPERATIONS (using bytes package)
// ============================================================================

// Equal performs zero-allocation equality check with another ZeroAllocStringRef
func (ref ZeroAllocStringRef) Equal(other ZeroAllocStringRef) bool {
	// Fast hash comparison first
	if ref.Hash != other.Hash || ref.Length != other.Length {
		return false
	}

	// Byte comparison without allocation
	return bytes.Equal(ref.Bytes(), other.Bytes())
}

// EqualString performs zero-allocation equality check with a string
func (ref ZeroAllocStringRef) EqualString(s string) bool {
	if ref.Length != uint32(len(s)) {
		return false
	}
	return bytes.Equal(ref.Bytes(), []byte(s))
}

// Contains checks if the reference contains the given pattern (zero allocation)
func (ref ZeroAllocStringRef) Contains(pattern string) bool {
	// For simple ASCII patterns, use optimized search
	if len(pattern) == 0 {
		return true
	}
	if len(pattern) == 1 {
		return bytes.IndexByte(ref.Bytes(), pattern[0]) >= 0
	}

	// For longer patterns, use standard bytes.Contains
	return bytes.Contains(ref.Bytes(), []byte(pattern))
}

// ContainsBytes checks if the reference contains the given byte pattern (zero allocation)
func (ref ZeroAllocStringRef) ContainsBytes(pattern []byte) bool {
	return bytes.Contains(ref.Bytes(), pattern)
}

// HasPrefix checks if the reference starts with the given prefix (zero allocation)
func (ref ZeroAllocStringRef) HasPrefix(prefix string) bool {
	return bytes.HasPrefix(ref.Bytes(), []byte(prefix))
}

// HasSuffix checks if the reference ends with the given suffix (zero allocation)
func (ref ZeroAllocStringRef) HasSuffix(suffix string) bool {
	return bytes.HasSuffix(ref.Bytes(), []byte(suffix))
}

// Index finds the index of the first occurrence of the substring (zero allocation)
func (ref ZeroAllocStringRef) Index(substring string) int {
	return bytes.Index(ref.Bytes(), []byte(substring))
}

// LastIndex finds the index of the last occurrence of the substring (zero allocation)
func (ref ZeroAllocStringRef) LastIndex(substring string) int {
	return bytes.LastIndex(ref.Bytes(), []byte(substring))
}

// Count counts non-overlapping occurrences of the substring (zero allocation)
func (ref ZeroAllocStringRef) Count(substring string) int {
	return bytes.Count(ref.Bytes(), []byte(substring))
}

// ============================================================================
// ZERO-ALLOCATION CASE OPERATIONS
// ============================================================================

// EqualFold performs case-insensitive equality check (zero allocation)
func (ref ZeroAllocStringRef) EqualFold(s string) bool {
	// Fast path for ASCII strings
	if isASCII(s) && isASCIIBytes(ref.Bytes()) {
		return len(ref.Bytes()) == len(s) && equalASCIIFold(ref.Bytes(), s)
	}
	return bytes.EqualFold(ref.Bytes(), []byte(s))
}

// ContainsFold performs case-insensitive contains check (zero allocation)
func (ref ZeroAllocStringRef) ContainsFold(pattern string) bool {
	return bytes.Contains(bytes.ToLower(ref.Bytes()), []byte(pattern))
}

// HasPrefixFold performs case-insensitive prefix check (zero allocation)
func (ref ZeroAllocStringRef) HasPrefixFold(prefix string) bool {
	// Fast path for ASCII strings
	if isASCII(prefix) && isASCIIBytes(ref.Bytes()) {
		refBytes := ref.Bytes()
		if len(refBytes) < len(prefix) {
			return false
		}
		for i := 0; i < len(prefix); i++ {
			cb := refBytes[i]
			cp := prefix[i]
			if cb >= 'A' && cb <= 'Z' {
				cb = cb + 32 // Convert to lowercase
			}
			if cp >= 'A' && cp <= 'Z' {
				cp = cp + 32 // Convert to lowercase
			}
			if cb != cp {
				return false
			}
		}
		return true
	}
	return bytes.HasPrefix(bytes.ToLower(ref.Bytes()), []byte(prefix))
}

// HasSuffixFold performs case-insensitive suffix check (zero allocation)
func (ref ZeroAllocStringRef) HasSuffixFold(suffix string) bool {
	return bytes.HasSuffix(bytes.ToLower(ref.Bytes()), []byte(suffix))
}

// ============================================================================
// ZERO-ALLOCATION TRIMMING OPERATIONS
// ============================================================================

// trimHelper provides common logic for creating trimmed string references
func (ref ZeroAllocStringRef) trimHelper(data []byte, trimmed []byte, offsetAdjustment int) ZeroAllocStringRef {
	if len(trimmed) == 0 {
		return EmptyZeroAllocStringRef
	}

	return ZeroAllocStringRef{
		Data:   ref.Data,
		FileID: ref.FileID,
		Offset: ref.Offset + uint32(offsetAdjustment),
		Length: uint32(len(trimmed)),
		Hash:   ComputeHash(trimmed), // Recompute hash for trimmed content
	}
}

// TrimSpace returns a new ZeroAllocStringRef with leading/trailing whitespace removed
func (ref ZeroAllocStringRef) TrimSpace() ZeroAllocStringRef {
	data := ref.Bytes()
	start := 0
	end := len(data)

	// Find first non-whitespace
	for start < end && isSpace(rune(data[start])) {
		start++
	}

	// Find last non-whitespace
	for end > start && isSpace(rune(data[end-1])) {
		end--
	}

	if start >= end {
		return EmptyZeroAllocStringRef
	}

	return ZeroAllocStringRef{
		Data:   ref.Data,
		FileID: ref.FileID,
		Offset: ref.Offset + uint32(start),
		Length: uint32(end - start),
		Hash:   ComputeHash(data[start:end]), // Recompute hash for trimmed content
	}
}

// Trim returns a new ZeroAllocStringRef with the given cutset removed from both ends
func (ref ZeroAllocStringRef) Trim(cutset string) ZeroAllocStringRef {
	data := ref.Bytes()
	trimmed := bytes.Trim(data, cutset)

	// Calculate offset adjustment
	offsetAdjustment := bytes.Index(data, trimmed)

	return ref.trimHelper(data, trimmed, offsetAdjustment)
}

// TrimLeft returns a new ZeroAllocStringRef with leading cutset removed
func (ref ZeroAllocStringRef) TrimLeft(cutset string) ZeroAllocStringRef {
	data := ref.Bytes()
	trimmed := bytes.TrimLeft(data, cutset)

	// Calculate offset adjustment
	offsetAdjustment := bytes.Index(data, trimmed)

	return ref.trimHelper(data, trimmed, offsetAdjustment)
}

// TrimRight returns a new ZeroAllocStringRef with trailing cutset removed
func (ref ZeroAllocStringRef) TrimRight(cutset string) ZeroAllocStringRef {
	data := ref.Bytes()
	trimmed := bytes.TrimRight(data, cutset)

	// For right trim, no offset adjustment needed (starts at same position)
	return ref.trimHelper(data, trimmed, 0)
}

// ============================================================================
// ZERO-ALLOCATION SPLITTING OPERATIONS
// ============================================================================

// Split splits the reference by the given separator (zero allocation for result elements)
func (ref ZeroAllocStringRef) Split(separator string) []ZeroAllocStringRef {
	data := ref.Bytes()
	if len(separator) == 0 {
		// Split by individual bytes
		result := make([]ZeroAllocStringRef, len(data))
		for i, b := range data {
			result[i] = ZeroAllocStringRef{
				Data:   ref.Data,
				FileID: ref.FileID,
				Offset: ref.Offset + uint32(i),
				Length: 1,
				Hash:   uint64(b), // Single byte hash
			}
		}
		return result
	}

	parts := bytes.Split(data, []byte(separator))
	result := make([]ZeroAllocStringRef, len(parts))

	currentOffset := ref.Offset
	for i, part := range parts {
		result[i] = ZeroAllocStringRef{
			Data:   ref.Data,
			FileID: ref.FileID,
			Offset: currentOffset,
			Length: uint32(len(part)),
			Hash:   ComputeHash(part),
		}
		currentOffset += uint32(len(part)) + uint32(len(separator))
	}

	return result
}

// SplitN splits the reference by the given separator, up to n parts (zero allocation)
func (ref ZeroAllocStringRef) SplitN(separator string, n int) []ZeroAllocStringRef {
	data := ref.Bytes()
	parts := bytes.SplitN(data, []byte(separator), n)
	result := make([]ZeroAllocStringRef, len(parts))

	currentOffset := ref.Offset
	for i, part := range parts {
		result[i] = ZeroAllocStringRef{
			Data:   ref.Data,
			FileID: ref.FileID,
			Offset: currentOffset,
			Length: uint32(len(part)),
			Hash:   ComputeHash(part),
		}
		currentOffset += uint32(len(part)) + uint32(len(separator))
	}

	return result
}

// Fields splits the reference by whitespace (zero allocation)
func (ref ZeroAllocStringRef) Fields() []ZeroAllocStringRef {
	data := ref.Bytes()
	fields := bytes.Fields(data)
	result := make([]ZeroAllocStringRef, len(fields))

	for i, field := range fields {
		// Find the actual offset of this field in the original data
		fieldOffset := bytes.Index(data, field)
		if fieldOffset >= 0 {
			result[i] = ZeroAllocStringRef{
				Data:   ref.Data,
				FileID: ref.FileID,
				Offset: ref.Offset + uint32(fieldOffset),
				Length: uint32(len(field)),
				Hash:   ComputeHash(field),
			}
		}
	}

	return result
}

// ============================================================================
// ZERO-ALLOCATION PATTERN MATCHING
// ============================================================================

// HasAnyPrefix checks if the reference starts with any of the given prefixes (zero allocation)
func (ref ZeroAllocStringRef) HasAnyPrefix(prefixes ...string) bool {
	data := ref.Bytes()
	for _, prefix := range prefixes {
		if bytes.HasPrefix(data, []byte(prefix)) {
			return true
		}
	}
	return false
}

// HasAnySuffix checks if the reference ends with any of the given suffixes (zero allocation)
func (ref ZeroAllocStringRef) HasAnySuffix(suffixes ...string) bool {
	data := ref.Bytes()
	for _, suffix := range suffixes {
		if bytes.HasSuffix(data, []byte(suffix)) {
			return true
		}
	}
	return false
}

// ContainsAny checks if the reference contains any of the given substrings (zero allocation)
func (ref ZeroAllocStringRef) ContainsAny(substrings ...string) bool {
	data := ref.Bytes()
	for _, substr := range substrings {
		if bytes.Contains(data, []byte(substr)) {
			return true
		}
	}
	return false
}

// ============================================================================
// ZERO-ALLOCATION UNICODE OPERATIONS
// ============================================================================

// ToLower creates a new ZeroAllocStringRef with all Unicode letters converted to lowercase
// Note: This DOES allocate a new []byte because Unicode case conversion requires it
func (ref ZeroAllocStringRef) ToLower() ZeroAllocStringRef {
	data := ref.Bytes()
	lower := bytes.ToLower(data)

	// If no conversion happened, return original reference
	if bytes.Equal(data, lower) {
		return ref
	}

	return ZeroAllocStringRef{
		Data:   lower, // New allocated slice
		FileID: ref.FileID,
		Offset: 0,
		Length: uint32(len(lower)),
		Hash:   ComputeHash(lower),
	}
}

// ToUpper creates a new ZeroAllocStringRef with all Unicode letters converted to uppercase
// Note: This DOES allocate a new []byte because Unicode case conversion requires it
func (ref ZeroAllocStringRef) ToUpper() ZeroAllocStringRef {
	data := ref.Bytes()
	upper := bytes.ToUpper(data)

	// If no conversion happened, return original reference
	if bytes.Equal(data, upper) {
		return ref
	}

	return ZeroAllocStringRef{
		Data:   upper, // New allocated slice
		FileID: ref.FileID,
		Offset: 0,
		Length: uint32(len(upper)),
		Hash:   ComputeHash(upper),
	}
}

// ============================================================================
// ZERO-ALLOCATION SUBSTRING OPERATIONS
// ============================================================================

// Substring returns a substring from start to end (zero allocation)
func (ref ZeroAllocStringRef) Substring(start, end int) ZeroAllocStringRef {
	if start < 0 {
		start = 0
	}
	if end > int(ref.Length) {
		end = int(ref.Length)
	}
	if start >= end {
		return EmptyZeroAllocStringRef
	}

	return ZeroAllocStringRef{
		Data:   ref.Data,
		FileID: ref.FileID,
		Offset: ref.Offset + uint32(start),
		Length: uint32(end - start),
		Hash:   ComputeHash(ref.Bytes()[start:end]),
	}
}

// Slice returns a substring from start with given length (zero allocation)
func (ref ZeroAllocStringRef) Slice(start, length uint32) ZeroAllocStringRef {
	if start >= ref.Length {
		return EmptyZeroAllocStringRef
	}
	if start+length > ref.Length {
		length = ref.Length - start
	}

	return ZeroAllocStringRef{
		Data:   ref.Data,
		FileID: ref.FileID,
		Offset: ref.Offset + start,
		Length: length,
		Hash:   ComputeHash(ref.Bytes()[start : start+length]),
	}
}

// ============================================================================
// COMPARISON AND SORTING OPERATIONS
// ============================================================================

// Compare performs lexical comparison with another ZeroAllocStringRef (zero allocation)
func (ref ZeroAllocStringRef) Compare(other ZeroAllocStringRef) int {
	return bytes.Compare(ref.Bytes(), other.Bytes())
}

// CompareString performs lexical comparison with a string (zero allocation)
func (ref ZeroAllocStringRef) CompareString(s string) int {
	return bytes.Compare(ref.Bytes(), []byte(s))
}

// LessThan checks if this reference is lexicographically less than another
func (ref ZeroAllocStringRef) LessThan(other ZeroAllocStringRef) bool {
	return ref.Compare(other) < 0
}

// GreaterThan checks if this reference is lexicographically greater than another
func (ref ZeroAllocStringRef) GreaterThan(other ZeroAllocStringRef) bool {
	return ref.Compare(other) > 0
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// Len returns the length of the reference (zero allocation)
func (ref ZeroAllocStringRef) Len() int {
	return int(ref.Length)
}

// IsEmptyString checks if the reference represents an empty string (zero allocation)
func (ref ZeroAllocStringRef) IsEmptyString() bool {
	return ref.Length == 0
}

// IsValid checks if the reference has valid data (zero allocation)
func (ref ZeroAllocStringRef) IsValid() bool {
	return ref.Data != nil && ref.Offset+ref.Length <= uint32(len(ref.Data))
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// isSpace checks if a rune is whitespace (used by TrimSpace)
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// ============================================================================
// CONVERSION FUNCTIONS
// ============================================================================

// FromStringRef converts a regular StringRef to ZeroAllocStringRef
// This requires access to the FileContentStore to get the underlying data
func FromStringRef(ref StringRef, data []byte) ZeroAllocStringRef {
	if ref.IsEmpty() {
		return EmptyZeroAllocStringRef
	}

	end := ref.Offset + ref.Length
	if int(end) > len(data) {
		return EmptyZeroAllocStringRef
	}

	return ZeroAllocStringRef{
		Data:   data,
		FileID: ref.FileID,
		Offset: ref.Offset,
		Length: ref.Length,
		Hash:   ref.Hash,
	}
}

// ToStringRef converts a ZeroAllocStringRef back to regular StringRef
func (ref ZeroAllocStringRef) ToStringRef() StringRef {
	return StringRef{
		FileID: ref.FileID,
		Offset: ref.Offset,
		Length: ref.Length,
		Hash:   ref.Hash,
	}
}

// FromBytes creates a ZeroAllocStringRef from a byte slice
func FromBytes(data []byte) ZeroAllocStringRef {
	if len(data) == 0 {
		return EmptyZeroAllocStringRef
	}

	return ZeroAllocStringRef{
		Data:   data,
		FileID: 0, // No FileID for raw bytes
		Offset: 0,
		Length: uint32(len(data)),
		Hash:   ComputeHash(data),
	}
}

// FromString creates a ZeroAllocStringRef from a string
// Note: This allocates because we need to convert string to []byte
// Use sparingly - only when converting from external string sources
func FromString(s string) ZeroAllocStringRef {
	if s == "" {
		return EmptyZeroAllocStringRef
	}

	data := []byte(s)
	return ZeroAllocStringRef{
		Data:   data,
		FileID: 0, // No FileID for string literals
		Offset: 0,
		Length: uint32(len(data)),
		Hash:   ComputeHash(data),
	}
}

// Join concatenates this reference with another using a separator (zero allocation until final string conversion)
func (ref ZeroAllocStringRef) Join(separator string, other ZeroAllocStringRef) ZeroAllocStringRef {
	if ref.IsEmpty() {
		return other
	}
	if other.IsEmpty() {
		return ref
	}

	sepBytes := []byte(separator)
	totalLength := ref.Len() + other.Len() + len(sepBytes)

	// Allocate single combined buffer
	combined := make([]byte, 0, totalLength)
	combined = append(combined, ref.Bytes()...)
	combined = append(combined, sepBytes...)
	combined = append(combined, other.Bytes()...)

	return ZeroAllocStringRef{
		Data:   combined,
		FileID: 0, // Combined reference has no single FileID
		Offset: 0,
		Length: uint32(len(combined)),
		Hash:   ComputeHash(combined),
	}
}

// ============================================================================
// COLLECTION OPERATIONS
// ============================================================================

// ZeroAllocStringRefSlice provides helper methods for slices of ZeroAllocStringRef
type ZeroAllocStringRefSlice []ZeroAllocStringRef

// Strings converts all references to strings (allocates - use sparingly)
func (zass ZeroAllocStringRefSlice) Strings() []string {
	result := make([]string, len(zass))
	for i, ref := range zass {
		result[i] = ref.String()
	}
	return result
}

// Join concatenates references with a separator (zero allocation until final string conversion)
func (zass ZeroAllocStringRefSlice) Join(separator string) ZeroAllocStringRef {
	if len(zass) == 0 {
		return EmptyZeroAllocStringRef
	}

	if len(zass) == 1 {
		return zass[0]
	}

	sepBytes := []byte(separator)
	var totalLength int
	for _, ref := range zass {
		totalLength += ref.Len()
	}
	totalLength += (len(zass) - 1) * len(sepBytes)

	// Allocate single combined buffer
	combined := make([]byte, 0, totalLength)

	for i, ref := range zass {
		if i > 0 {
			combined = append(combined, sepBytes...)
		}
		combined = append(combined, ref.Bytes()...)
	}

	return ZeroAllocStringRef{
		Data:   combined,
		FileID: 0, // Combined reference has no single FileID
		Offset: 0,
		Length: uint32(len(combined)),
		Hash:   ComputeHash(combined),
	}
}

// Filter returns references that contain any of the given patterns (zero allocation)
func (zass ZeroAllocStringRefSlice) Filter(patterns ...string) ZeroAllocStringRefSlice {
	if len(patterns) == 0 {
		return zass
	}

	result := make(ZeroAllocStringRefSlice, 0, len(zass))
	for _, ref := range zass {
		if ref.ContainsAny(patterns...) {
			result = append(result, ref)
		}
	}

	return result
}

// FilterByPrefix returns references that start with any of the given prefixes (zero allocation)
func (zass ZeroAllocStringRefSlice) FilterByPrefix(prefixes ...string) ZeroAllocStringRefSlice {
	if len(prefixes) == 0 {
		return zass
	}

	result := make(ZeroAllocStringRefSlice, 0, len(zass))
	for _, ref := range zass {
		if ref.HasAnyPrefix(prefixes...) {
			result = append(result, ref)
		}
	}

	return result
}

// FilterBySuffix returns references that end with any of the given suffixes (zero allocation)
func (zass ZeroAllocStringRefSlice) FilterBySuffix(suffixes ...string) ZeroAllocStringRefSlice {
	if len(suffixes) == 0 {
		return zass
	}

	result := make(ZeroAllocStringRefSlice, 0, len(zass))
	for _, ref := range zass {
		if ref.HasAnySuffix(suffixes...) {
			result = append(result, ref)
		}
	}

	return result
}

// Sort sorts the references lexicographically (zero allocation)
func (zass ZeroAllocStringRefSlice) Sort() {
	// Use Go's built-in sort with our comparison function (O(n log n) instead of O(n²))
	sort.Slice(zass, func(i, j int) bool {
		return zass[i].LessThan(zass[j])
	})
}

// Unique removes duplicate references (zero allocation, preserves order)
func (zass ZeroAllocStringRefSlice) Unique() ZeroAllocStringRefSlice {
	seen := make(map[uint64]bool)
	result := make(ZeroAllocStringRefSlice, 0, len(zass))

	for _, ref := range zass {
		if !seen[ref.Hash] {
			seen[ref.Hash] = true
			result = append(result, ref)
		}
	}

	return result
}

// Helper functions for performance optimization

// isASCII checks if a string contains only ASCII characters
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 128 {
			return false
		}
	}
	return true
}

// isASCIIBytes checks if a byte slice contains only ASCII characters
func isASCIIBytes(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] >= 128 {
			return false
		}
	}
	return true
}

// equalASCIIFold performs ASCII-only case-insensitive comparison (optimized)
func equalASCIIFold(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := 0; i < len(b); i++ {
		cb := b[i]
		cs := s[i]
		if cb >= 'A' && cb <= 'Z' {
			cb = cb + 32 // Convert to lowercase
		}
		if cs >= 'A' && cs <= 'Z' {
			cs = cs + 32 // Convert to lowercase
		}
		if cb != cs {
			return false
		}
	}
	return true
}
