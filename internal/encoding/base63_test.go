package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase63Encode_Zero(t *testing.T) {
	result := Base63Encode(0)
	assert.Equal(t, "A", result, "Zero should encode to 'A'")
}

func TestBase63Encode_SingleDigits(t *testing.T) {
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
			result := Base63Encode(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBase63Encode_MultiDigit(t *testing.T) {
	tests := []struct {
		value    uint64
		expected string
	}{
		{63, "BA"},    // 1*63 + 0 = 63
		{64, "BB"},    // 1*63 + 1 = 64
		{125, "B_"},   // 1*63 + 62 = 125
		{126, "CA"},   // 2*63 + 0 = 126
		{3969, "BAA"}, // 63^2 = 3969
		{5130, "BSb"}, // Bug report value
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := Base63Encode(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBase63Decode_SingleDigits(t *testing.T) {
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
			result, err := Base63Decode(tc.encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBase63RoundTrip(t *testing.T) {
	testValues := []uint64{
		0, 1, 62, 63, 64, 100, 1000, 10000, 100000, 1000000,
		5130,               // Bug report value
		0xFFFFFFFF,         // Max uint32
		0xFFFFFFFFFFFFFFFF, // Max uint64
	}

	for _, value := range testValues {
		encoded := Base63Encode(value)
		decoded, err := Base63Decode(encoded)
		require.NoError(t, err, "Failed to decode %s (from %d)", encoded, value)
		assert.Equal(t, value, decoded)
	}
}

func TestBase63EncodeNoZero(t *testing.T) {
	assert.Equal(t, "", Base63EncodeNoZero(0), "Zero should encode to empty string")
	assert.Equal(t, "B", Base63EncodeNoZero(1), "Non-zero should encode normally")
}

func TestBase63IsValid(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"A", true},
		{"ABC", true},
		{"", false},
		{"!", false},
		{"AB CD", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.valid, Base63IsValid(tc.input))
		})
	}
}

func TestPackUnpackUint32Pair(t *testing.T) {
	tests := []struct {
		lower uint32
		upper uint32
	}{
		{0, 0},
		{1, 0},
		{0, 1},
		{123, 456},
		{0xFFFFFFFF, 0xFFFFFFFF},
	}

	for _, tc := range tests {
		packed := PackUint32Pair(tc.lower, tc.upper)
		gotLower, gotUpper := UnpackUint32Pair(packed)
		assert.Equal(t, tc.lower, gotLower)
		assert.Equal(t, tc.upper, gotUpper)
	}
}

// Benchmark
func BenchmarkBase63Encode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Base63Encode(uint64(i))
	}
}

func BenchmarkBase63Decode(b *testing.B) {
	encoded := Base63Encode(12345678)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Base63Decode(encoded)
	}
}
