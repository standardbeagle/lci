package idcodec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncode_Zero(t *testing.T) {
	result := Encode(0)
	assert.Equal(t, "A", result, "Zero should encode to 'A'")
}

func TestEncode_SingleDigits(t *testing.T) {
	tests := []struct {
		value    uint64
		expected string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "a"},
		{51, "z"},
		{52, "0"},
		{61, "9"},
		{62, "_"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := Encode(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEncode_MultiDigit(t *testing.T) {
	tests := []struct {
		value    uint64
		expected string
	}{
		{63, "BA"},    // Base case: 1*63 + 0 = 63
		{64, "BB"},    // 1*63 + 1 = 64
		{125, "B_"},   // 1*63 + 62 = 125
		{126, "CA"},   // 2*63 + 0 = 126
		{3969, "BAA"}, // 63^2 = 3969
		{5130, "BSb"}, // The bug report value: 1*63*63 + 18*63 + 27
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := Encode(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDecode_SingleDigits(t *testing.T) {
	tests := []struct {
		encoded  string
		expected uint64
	}{
		{"A", 0},
		{"B", 1},
		{"Z", 25},
		{"a", 26},
		{"z", 51},
		{"0", 52},
		{"9", 61},
		{"_", 62},
	}

	for _, tc := range tests {
		t.Run(tc.encoded, func(t *testing.T) {
			result, err := Decode(tc.encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDecode_MultiDigit(t *testing.T) {
	tests := []struct {
		encoded  string
		expected uint64
	}{
		{"BA", 63},
		{"BB", 64},
		{"BAA", 3969},
		{"BSb", 5130}, // 1*63*63 + 18*63 + 27
	}

	for _, tc := range tests {
		t.Run(tc.encoded, func(t *testing.T) {
			result, err := Decode(tc.encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	testValues := []uint64{
		0, 1, 62, 63, 64, 100, 1000, 10000, 100000, 1000000,
		5130,               // Bug report value
		0xFFFFFFFF,         // Max uint32
		0x0000FFFFFFFFFFFF, // Large value
		0xFFFFFFFFFFFFFFFF, // Max uint64
	}

	for _, value := range testValues {
		encoded := Encode(value)
		decoded, err := Decode(encoded)
		require.NoError(t, err, "Failed to decode %s (from %d)", encoded, value)
		assert.Equal(t, value, decoded, "Round trip failed for %d -> %s -> %d", value, encoded, decoded)
	}
}

func TestDecode_EmptyString(t *testing.T) {
	_, err := Decode("")
	assert.ErrorIs(t, err, ErrEmptyString)
}

func TestDecode_InvalidCharacters(t *testing.T) {
	invalidStrings := []string{
		"!",
		"@",
		"#",
		"$",
		" ",
		"AB@CD",
		"hello world",
	}

	for _, s := range invalidStrings {
		t.Run(s, func(t *testing.T) {
			_, err := Decode(s)
			assert.Error(t, err, "Should error on invalid character")
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"A", true},
		{"z", true},
		{"0", true},
		{"_", true},
		{"ABC", true},
		{"abc123", true},
		{"test_var", true},
		{"", false},
		{"!", false},
		{"AB CD", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := IsValid(tc.input)
			assert.Equal(t, tc.valid, result)
		})
	}
}

func TestEncodeNoZero(t *testing.T) {
	assert.Equal(t, "", EncodeNoZero(0), "Zero should encode to empty string")
	assert.Equal(t, "B", EncodeNoZero(1), "Non-zero should encode normally")
}

// Benchmark encoding
func BenchmarkEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Encode(uint64(i))
	}
}

// Benchmark decoding
func BenchmarkDecode(b *testing.B) {
	encoded := Encode(12345678)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decode(encoded)
	}
}
