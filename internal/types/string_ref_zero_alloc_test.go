package types

import (
	"testing"
)

func TestZeroAllocStringRef_BasicOperations(t *testing.T) {
	data := []byte("Hello, World! This is a test string.")
	ref := FromBytes(data)

	tests := []struct {
		name string
		test func(ZeroAllocStringRef) bool
	}{
		{
			name: "IsEmpty",
			test: func(r ZeroAllocStringRef) bool { return !r.IsEmpty() },
		},
		{
			name: "Len",
			test: func(r ZeroAllocStringRef) bool { return r.Len() == len(data) },
		},
		{
			name: "IsValid",
			test: func(r ZeroAllocStringRef) bool { return r.IsValid() },
		},
		{
			name: "IsEmptyString",
			test: func(r ZeroAllocStringRef) bool { return !r.IsEmptyString() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.test(ref) {
				t.Errorf("ZeroAllocStringRef.%s failed", tt.name)
			}
		})
	}
}

func TestZeroAllocStringRef_Bytes(t *testing.T) {
	data := []byte("Hello, World!")
	ref := FromBytes(data)

	bytes := ref.Bytes()
	if string(bytes) != string(data) {
		t.Errorf("Bytes() = %v, want %v", bytes, data)
	}

	// Test empty reference
	empty := EmptyZeroAllocStringRef
	if empty.Bytes() != nil {
		t.Errorf("EmptyZeroAllocStringRef.Bytes() should return nil")
	}
}

func TestZeroAllocStringRef_String(t *testing.T) {
	data := []byte("Hello, World!")
	ref := FromBytes(data)

	str := ref.String()
	if str != "Hello, World!" {
		t.Errorf("String() = %q, want %q", str, "Hello, World!")
	}

	// Test empty reference
	empty := EmptyZeroAllocStringRef
	if empty.String() != "" {
		t.Errorf("EmptyZeroAllocStringRef.String() should return empty string")
	}
}

func TestZeroAllocStringRef_Equality(t *testing.T) {
	data1 := []byte("Hello")
	data2 := []byte("Hello")
	data3 := []byte("World")

	ref1 := FromBytes(data1)
	ref2 := FromBytes(data2)
	ref3 := FromBytes(data3)

	// Test Equal
	if !ref1.Equal(ref2) {
		t.Error("Equal() should return true for identical content")
	}
	if ref1.Equal(ref3) {
		t.Error("Equal() should return false for different content")
	}

	// Test EqualString
	if !ref1.EqualString("Hello") {
		t.Error("EqualString() should return true for matching string")
	}
	if ref1.EqualString("World") {
		t.Error("EqualString() should return false for non-matching string")
	}
}

func TestZeroAllocStringRef_Contains(t *testing.T) {
	data := []byte("Hello, World! This is a test.")
	ref := FromBytes(data)

	tests := []struct {
		pattern string
		want    bool
	}{
		{"Hello", true},
		{"World", true},
		{"test", true},
		{"xyz", false},
		{"", true}, // Empty pattern is always contained
	}

	for _, tt := range tests {
		t.Run("Contains_"+tt.pattern, func(t *testing.T) {
			got := ref.Contains(tt.pattern)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestZeroAllocStringRef_HasPrefix(t *testing.T) {
	data := []byte("Hello, World!")
	ref := FromBytes(data)

	tests := []struct {
		prefix string
		want   bool
	}{
		{"Hello", true},
		{"Hello,", true},
		{"Hello, World!", true},
		{"World", false},
		{"", true}, // Empty prefix always matches
	}

	for _, tt := range tests {
		t.Run("HasPrefix_"+tt.prefix, func(t *testing.T) {
			got := ref.HasPrefix(tt.prefix)
			if got != tt.want {
				t.Errorf("HasPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestZeroAllocStringRef_HasSuffix(t *testing.T) {
	data := []byte("Hello, World!")
	ref := FromBytes(data)

	tests := []struct {
		suffix string
		want   bool
	}{
		{"World!", true},
		{"!", true},
		{"Hello, World!", true},
		{"Hello", false},
		{"", true}, // Empty suffix always matches
	}

	for _, tt := range tests {
		t.Run("HasSuffix_"+tt.suffix, func(t *testing.T) {
			got := ref.HasSuffix(tt.suffix)
			if got != tt.want {
				t.Errorf("HasSuffix(%q) = %v, want %v", tt.suffix, got, tt.want)
			}
		})
	}
}

func TestZeroAllocStringRef_CaseOperations(t *testing.T) {
	data := []byte("Hello, WORLD!")
	ref := FromBytes(data)

	// Test EqualFold
	if !ref.EqualFold("hello, world!") {
		t.Error("EqualFold() should return true for case-insensitive match")
	}
	if ref.EqualFold("hello, world") {
		t.Error("EqualFold() should return false for different content")
	}

	// Test ContainsFold
	if !ref.ContainsFold("hello") {
		t.Error("ContainsFold() should return true for case-insensitive contain")
	}
	if ref.ContainsFold("xyz") {
		t.Error("ContainsFold() should return false for non-matching content")
	}

	// Test HasPrefixFold
	if !ref.HasPrefixFold("hello") {
		t.Error("HasPrefixFold() should return true for case-insensitive prefix")
	}
	if ref.HasPrefixFold("world") {
		t.Error("HasPrefixFold() should return false for non-matching prefix")
	}

	// Test HasSuffixFold
	if !ref.HasSuffixFold("world!") {
		t.Error("HasSuffixFold() should return true for case-insensitive suffix")
	}
	if ref.HasSuffixFold("hello") {
		t.Error("HasSuffixFold() should return false for non-matching suffix")
	}
}

func TestZeroAllocStringRef_TrimOperations(t *testing.T) {
	data := []byte("  \t\n  Hello, World!  \t\n  ")
	ref := FromBytes(data)

	// Test TrimSpace
	trimmed := ref.TrimSpace()
	expected := "Hello, World!"
	if trimmed.String() != expected {
		t.Errorf("TrimSpace() = %q, want %q", trimmed.String(), expected)
	}

	// Test Trim
	trimCutset := " \t\n"
	trimmedCutset := ref.Trim(trimCutset)
	if trimmedCutset.String() != expected {
		t.Errorf("Trim(%q) = %q, want %q", trimCutset, trimmedCutset.String(), expected)
	}

	// Test TrimLeft
	leftTrimmed := ref.TrimLeft(trimCutset)
	expectedLeft := "Hello, World!  \t\n  "
	if leftTrimmed.String() != expectedLeft {
		t.Errorf("TrimLeft(%q) = %q, want %q", trimCutset, leftTrimmed.String(), expectedLeft)
	}

	// Test TrimRight
	rightTrimmed := ref.TrimRight(trimCutset)
	expectedRight := "  \t\n  Hello, World!"
	if rightTrimmed.String() != expectedRight {
		t.Errorf("TrimRight(%q) = %q, want %q", trimCutset, rightTrimmed.String(), expectedRight)
	}
}

func TestZeroAllocStringRef_SplitOperations(t *testing.T) {
	data := []byte("a,b,c,d")
	ref := FromBytes(data)

	// Test Split
	parts := ref.Split(",")
	expected := []string{"a", "b", "c", "d"}
	if len(parts) != len(expected) {
		t.Errorf("Split() returned %d parts, want %d", len(parts), len(expected))
	}
	for i, part := range parts {
		if part.String() != expected[i] {
			t.Errorf("Split()[%d] = %q, want %q", i, part.String(), expected[i])
		}
	}

	// Test SplitN
	partsN := ref.SplitN(",", 2)
	expectedN := []string{"a", "b,c,d"}
	if len(partsN) != len(expectedN) {
		t.Errorf("SplitN() returned %d parts, want %d", len(partsN), len(expectedN))
	}
	for i, part := range partsN {
		if part.String() != expectedN[i] {
			t.Errorf("SplitN()[%d] = %q, want %q", i, part.String(), expectedN[i])
		}
	}

	// Test Fields
	fieldsData := []byte("  Hello   World  Test  ")
	fieldsRef := FromBytes(fieldsData)
	fields := fieldsRef.Fields()
	expectedFields := []string{"Hello", "World", "Test"}
	if len(fields) != len(expectedFields) {
		t.Errorf("Fields() returned %d fields, want %d", len(fields), len(expectedFields))
	}
	for i, field := range fields {
		if field.String() != expectedFields[i] {
			t.Errorf("Fields()[%d] = %q, want %q", i, field.String(), expectedFields[i])
		}
	}
}

func TestZeroAllocStringRef_PatternMatching(t *testing.T) {
	data := []byte("function test() { return true; }")
	ref := FromBytes(data)

	// Test HasAnyPrefix
	prefixes := []string{"func", "function", "var"}
	if !ref.HasAnyPrefix(prefixes...) {
		t.Error("HasAnyPrefix() should return true for matching prefixes")
	}

	nonPrefixes := []string{"class", "interface", "struct"}
	if ref.HasAnyPrefix(nonPrefixes...) {
		t.Error("HasAnyPrefix() should return false for non-matching prefixes")
	}

	// Test HasAnySuffix
	suffixes := []string{"}", "true);", "return true; }"}
	if !ref.HasAnySuffix(suffixes...) {
		t.Error("HasAnySuffix() should return true for matching suffixes")
	}

	nonSuffixes := []string{"{", "false);", "return false; }"}
	if ref.HasAnySuffix(nonSuffixes...) {
		t.Error("HasAnySuffix() should return false for non-matching suffixes")
	}

	// Test ContainsAny
	substrings := []string{"function", "test", "return"}
	if !ref.ContainsAny(substrings...) {
		t.Error("ContainsAny() should return true for matching substrings")
	}

	nonSubstrings := []string{"class", "interface", "false"}
	if ref.ContainsAny(nonSubstrings...) {
		t.Error("ContainsAny() should return false for non-matching substrings")
	}
}

func TestZeroAllocStringRef_SubstringOperations(t *testing.T) {
	data := []byte("Hello, World!")
	ref := FromBytes(data)

	// Test Substring
	sub := ref.Substring(7, 12) // "World"
	expected := "World"
	if sub.String() != expected {
		t.Errorf("Substring(7, 12) = %q, want %q", sub.String(), expected)
	}

	// Test Slice
	slice := ref.Slice(7, 5) // "World"
	if slice.String() != expected {
		t.Errorf("Slice(7, 5) = %q, want %q", slice.String(), expected)
	}

	// Test edge cases
	emptySub := ref.Substring(0, 0)
	if !emptySub.IsEmpty() {
		t.Error("Substring(0, 0) should return empty reference")
	}

	outOfBounds := ref.Substring(100, 200)
	if !outOfBounds.IsEmpty() {
		t.Error("Substring with out-of-bounds indices should return empty reference")
	}
}

func TestZeroAllocStringRef_Comparison(t *testing.T) {
	data1 := []byte("Apple")
	data2 := []byte("Banana")

	ref1 := FromBytes(data1)
	ref2 := FromBytes(data2)

	// Test Compare
	if ref1.Compare(ref2) >= 0 {
		t.Error("Compare should return negative for lexicographically smaller")
	}
	if ref2.Compare(ref1) <= 0 {
		t.Error("Compare should return positive for lexicographically larger")
	}
	if ref1.Compare(ref1) != 0 {
		t.Error("Compare should return zero for equal references")
	}

	// Test CompareString
	if ref1.CompareString("Banana") >= 0 {
		t.Error("CompareString should return negative for lexicographically smaller")
	}
	if ref1.CompareString("Apple") != 0 {
		t.Error("CompareString should return zero for equal strings")
	}

	// Test LessThan and GreaterThan
	if !ref1.LessThan(ref2) {
		t.Error("LessThan should return true for smaller reference")
	}
	if ref1.GreaterThan(ref2) {
		t.Error("GreaterThan should return false for smaller reference")
	}
}

func TestZeroAllocStringRef_CaseConversion(t *testing.T) {
	data := []byte("Hello, World!")
	ref := FromBytes(data)

	// Test ToLower
	lower := ref.ToLower()
	expectedLower := "hello, world!"
	if lower.String() != expectedLower {
		t.Errorf("ToLower() = %q, want %q", lower.String(), expectedLower)
	}

	// Test ToUpper
	upper := ref.ToUpper()
	expectedUpper := "HELLO, WORLD!"
	if upper.String() != expectedUpper {
		t.Errorf("ToUpper() = %q, want %q", upper.String(), expectedUpper)
	}

	// Test that original is unchanged
	if ref.String() != "Hello, World!" {
		t.Error("Original reference should be unchanged by case conversion")
	}
}

func TestZeroAllocStringRef_Join(t *testing.T) {
	data1 := []byte("Hello")
	data2 := []byte("World")
	ref1 := FromBytes(data1)
	ref2 := FromBytes(data2)

	// Test Join
	joined := ref1.Join(", ", ref2)
	expected := "Hello, World"
	if joined.String() != expected {
		t.Errorf("Join() = %q, want %q", joined.String(), expected)
	}

	// Test Join with empty references
	empty := EmptyZeroAllocStringRef
	if ref1.Join(", ", empty).String() != "Hello" {
		t.Error("Join with empty reference should return non-empty reference")
	}
	if empty.Join(", ", ref1).String() != "Hello" {
		t.Error("Join with empty first reference should return second reference")
	}
}

func TestZeroAllocStringRef_Collections(t *testing.T) {
	data := []byte("apple,banana,cherry")
	ref := FromBytes(data)
	parts := ref.Split(",")

	slice := ZeroAllocStringRefSlice(parts)

	// Test Strings
	strings := slice.Strings()
	expected := []string{"apple", "banana", "cherry"}
	if len(strings) != len(expected) {
		t.Errorf("Strings() returned %d items, want %d", len(strings), len(expected))
	}
	for i, str := range strings {
		if str != expected[i] {
			t.Errorf("Strings()[%d] = %q, want %q", i, str, expected[i])
		}
	}

	// Test Join
	joined := slice.Join("-")
	if joined.String() != "apple-banana-cherry" {
		t.Errorf("Join() = %q, want %q", joined.String(), "apple-banana-cherry")
	}

	// Test Filter
	filtered := slice.Filter("banana", "cherry")
	if len(filtered) != 2 {
		t.Errorf("Filter() returned %d items, want 2", len(filtered))
	}

	// Test FilterByPrefix
	prefixed := slice.FilterByPrefix("ba")
	if len(prefixed) != 1 || prefixed[0].String() != "banana" {
		t.Errorf("FilterByPrefix() should return only 'banana'")
	}

	// Test FilterBySuffix
	suffixed := slice.FilterBySuffix("ry")
	if len(suffixed) != 1 || suffixed[0].String() != "cherry" {
		t.Errorf("FilterBySuffix() should return only 'cherry'")
	}

	// Test Sort
	unsorted := ZeroAllocStringRefSlice{FromBytes([]byte("c")), FromBytes([]byte("a")), FromBytes([]byte("b"))}
	unsorted.Sort()
	if unsorted[0].String() != "a" || unsorted[1].String() != "b" || unsorted[2].String() != "c" {
		t.Error("Sort() should sort references lexicographically")
	}

	// Test Unique
	duplicate := ZeroAllocStringRefSlice{
		FromBytes([]byte("apple")),
		FromBytes([]byte("banana")),
		FromBytes([]byte("apple")), // duplicate
		FromBytes([]byte("cherry")),
	}
	unique := duplicate.Unique()
	if len(unique) != 3 {
		t.Errorf("Unique() should remove duplicates, got %d items", len(unique))
	}
}

func TestZeroAllocStringRef_ConversionFunctions(t *testing.T) {
	// Test FromString
	strRef := FromString("Hello, World!")
	if strRef.String() != "Hello, World!" {
		t.Errorf("FromString() = %q, want %q", strRef.String(), "Hello, World!")
	}

	// Test FromBytes
	bytesRef := FromBytes([]byte("Hello, World!"))
	if bytesRef.String() != "Hello, World!" {
		t.Errorf("FromBytes() = %q, want %q", bytesRef.String(), "Hello, World!")
	}

	// Test empty conversions
	if !FromString("").IsEmpty() {
		t.Error("FromString(\"\") should return empty reference")
	}
	if !FromBytes(nil).IsEmpty() {
		t.Error("FromBytes(nil) should return empty reference")
	}
}

func TestZeroAllocStringRef_EdgeCases(t *testing.T) {
	// Test with empty data
	emptyRef := FromBytes([]byte(""))
	if !emptyRef.IsEmpty() {
		t.Error("Reference to empty data should be empty")
	}

	// Test with nil data
	nilRef := FromBytes(nil)
	if !nilRef.IsEmpty() {
		t.Error("Reference to nil data should be empty")
	}

	// Test operations on empty references
	if emptyRef.Contains("test") {
		t.Error("Empty reference should not contain any pattern")
	}
	if emptyRef.HasPrefix("test") {
		t.Error("Empty reference should not have any prefix")
	}
	if emptyRef.HasSuffix("test") {
		t.Error("Empty reference should not have any suffix")
	}

	// Test with single character
	singleChar := FromBytes([]byte("a"))
	if singleChar.Len() != 1 {
		t.Errorf("Single character reference should have length 1, got %d", singleChar.Len())
	}
	if !singleChar.Contains("a") {
		t.Error("Single character reference should contain itself")
	}
}