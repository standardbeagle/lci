package core

import (
	"fmt"
	"github.com/standardbeagle/lci/internal/types"
	"testing"
)

func TestDenseObjectID_Encoding(t *testing.T) {
	tests := []struct {
		name          string
		fileID        uint32
		localSymbolID uint32
		symbolType    uint32
		wantNonEmpty  bool
	}{
		{
			name:          "simple values",
			fileID:        1,
			localSymbolID: 1,
			symbolType:    0,
			wantNonEmpty:  true,
		},
		{
			name:          "medium values",
			fileID:        1000,
			localSymbolID: 5000,
			symbolType:    5,
			wantNonEmpty:  true,
		},
		{
			name:          "large values",
			fileID:        65535,
			localSymbolID: 65535,
			symbolType:    10,
			wantNonEmpty:  true,
		},
		{
			name:          "zero values",
			fileID:        0,
			localSymbolID: 0,
			symbolType:    0,
			wantNonEmpty:  false, // Will produce empty encoding
		},
		{
			name:          "max uint32 values",
			fileID:        4294967295,
			localSymbolID: 4294967295,
			symbolType:    255,
			wantNonEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create dense object ID
			doid := NewDenseObjectID(tt.fileID, tt.localSymbolID, tt.symbolType)
			encoded := doid.String()

			// Check non-empty requirement
			if tt.wantNonEmpty && encoded == "" {
				t.Errorf("Expected non-empty encoding, got empty")
			}

			// Verify it's valid
			if !doid.IsValid() && tt.wantNonEmpty {
				t.Errorf("Expected valid dense object ID, got invalid")
			}

			// Test extraction methods
			extractedFileID := doid.ExtractFileID()
			extractedLocalSymbolID := doid.ExtractLocalSymbolID()

			// For non-zero inputs, verify round-trip works
			if tt.fileID > 0 || tt.localSymbolID > 0 {
				if extractedFileID != types.FileID(tt.fileID) {
					t.Errorf("FileID mismatch: want %d, got %d", tt.fileID, extractedFileID)
				}
				if extractedLocalSymbolID != tt.localSymbolID {
					t.Errorf("LocalSymbolID mismatch: want %d, got %d", tt.localSymbolID, extractedLocalSymbolID)
				}
			}

			// Verify encoded string only contains valid characters
			for _, c := range encoded {
				valid := (c >= 'A' && c <= 'Z') ||
					(c >= 'a' && c <= 'z') ||
					(c >= '0' && c <= '9') ||
					c == '_'
				if !valid {
					t.Errorf("Invalid character '%c' in encoded string", c)
				}
			}
		})
	}
}

func TestDenseObjectID_EncodeDecode(t *testing.T) {
	// Test round-trip encoding and decoding
	testCases := []struct {
		fileID        uint32
		localSymbolID uint32
	}{
		{1, 1},
		{100, 200},
		{1000, 5000},
		{65535, 65535},
		{1234567, 7654321},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("file%d_symbol%d", tc.fileID, tc.localSymbolID), func(t *testing.T) {
			// Encode
			encoded := encodeSymbol(tc.fileID, tc.localSymbolID)
			if encoded == "" && (tc.fileID > 0 || tc.localSymbolID > 0) {
				t.Fatal("Expected non-empty encoding")
			}

			// Decode
			decodedFileID, decodedLocalSymbolID, err := decodeSymbol(encoded)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			// Verify
			if decodedFileID != types.FileID(tc.fileID) {
				t.Errorf("FileID mismatch: want %d, got %d", tc.fileID, decodedFileID)
			}
			if decodedLocalSymbolID != tc.localSymbolID {
				t.Errorf("LocalSymbolID mismatch: want %d, got %d", tc.localSymbolID, decodedLocalSymbolID)
			}
		})
	}
}

func TestDenseObjectID_EdgeCases(t *testing.T) {
	t.Run("empty encoded string", func(t *testing.T) {
		_, _, err := decodeSymbol("")
		if err == nil {
			t.Error("Expected error for empty string, got nil")
		}
	})

	t.Run("invalid character", func(t *testing.T) {
		_, _, err := decodeSymbol("ABC@123") // @ is not in symbol set
		if err == nil {
			t.Error("Expected error for invalid character, got nil")
		}
	})

	t.Run("extract from invalid", func(t *testing.T) {
		doid := DenseObjectID{encoded: ""}
		if doid.ExtractFileID() != 0 {
			t.Error("Expected 0 for invalid FileID extraction")
		}
		if doid.ExtractLocalSymbolID() != 0 {
			t.Error("Expected 0 for invalid LocalSymbolID extraction")
		}
		if doid.ExtractSymbolType() != 0 {
			t.Error("Expected 0 for symbol type (not encoded)")
		}
	})
}

func TestDenseObjectID_CharacterSet(t *testing.T) {
	// Test that our encoding covers the expected range
	// A-Z: 0-25, a-z: 26-51, 0-9: 52-61, _: 62
	testCases := []struct {
		val  uint64
		want byte
	}{
		{0, 'A'}, {25, 'Z'},   // A-Z
		{26, 'a'}, {51, 'z'},   // a-z
		{52, '0'}, {61, '9'},   // 0-9
		{62, '_'},              // underscore
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("encode_%d", tc.val), func(t *testing.T) {
			got := encodeChar(tc.val)
			if got != tc.want {
				t.Errorf("encodeChar(%d) = %c, want %c", tc.val, got, tc.want)
			}

			// Test round-trip
			decoded, err := decodeChar(got)
			if err != nil {
				t.Errorf("decodeChar(%c) error: %v", got, err)
			}
			if decoded != tc.val {
				t.Errorf("decodeChar(%c) = %d, want %d", got, decoded, tc.val)
			}
		})
	}
}

func TestDenseObjectID_Stats(t *testing.T) {
	stats := DenseStats()

	// Verify expected stats exist
	expectedKeys := []string{
		"encoding_type",
		"alphabet",
		"alphabet_size",
		"max_supported_files",
		"max_supported_symbols_per_file",
		"avg_id_length",
		"max_id_length",
		"compression_ratio",
	}

	for _, key := range expectedKeys {
		if _, ok := stats[key]; !ok {
			t.Errorf("Missing expected stat key: %s", key)
		}
	}

	// Verify alphabet size matches expected
	if size, ok := stats["alphabet_size"].(int); ok {
		expectedSize := 63 // A-Za-z0-9_ = 26+26+10+1
		if size != expectedSize {
			t.Errorf("Alphabet size mismatch: stat says %d, expected %d", size, expectedSize)
		}
	}
}

func BenchmarkDenseObjectID_Encode(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewDenseObjectID(uint32(i%65536), uint32(i%65536), 0)
	}
}

func BenchmarkDenseObjectID_Decode(b *testing.B) {
	// Pre-generate some encoded IDs
	encodedIDs := make([]string, 1000)
	for i := range encodedIDs {
		doid := NewDenseObjectID(uint32(i), uint32(i*2), 0)
		encodedIDs[i] = doid.String()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded := encodedIDs[i%len(encodedIDs)]
		_, _, _ = decodeSymbol(encoded)
	}
}

func BenchmarkDenseObjectID_ExtractFileID(b *testing.B) {
	doid := NewDenseObjectID(12345, 67890, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doid.ExtractFileID()
	}
}

func BenchmarkDenseObjectID_ExtractLocalSymbolID(b *testing.B) {
	doid := NewDenseObjectID(12345, 67890, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doid.ExtractLocalSymbolID()
	}
}

func TestDenseObjectID_ComparisonWithOldSystem(t *testing.T) {
	// This test compares the dense encoding with what a traditional
	// hex or base64 encoding would produce
	fileID := uint32(1234)
	localSymbolID := uint32(5678)

	doid := NewDenseObjectID(fileID, localSymbolID, 0)
	denseEncoded := doid.String()

	// Traditional hex encoding (what might be used in old system)
	hexEncoded := fmt.Sprintf("%08x%08x", fileID, localSymbolID)

	// Compare lengths
	t.Logf("Dense encoding: %s (length: %d)", denseEncoded, len(denseEncoded))
	t.Logf("Hex encoding:   %s (length: %d)", hexEncoded, len(hexEncoded))

	// Dense should be significantly shorter
	if len(denseEncoded) >= len(hexEncoded) {
		t.Errorf("Dense encoding not shorter: dense=%d, hex=%d", len(denseEncoded), len(hexEncoded))
	}

	// Verify both can represent the same data
	extractedFileID := doid.ExtractFileID()
	extractedLocalSymbolID := doid.ExtractLocalSymbolID()

	if extractedFileID != types.FileID(fileID) {
		t.Errorf("FileID mismatch in dense encoding: want %d, got %d", fileID, extractedFileID)
	}
	if extractedLocalSymbolID != localSymbolID {
		t.Errorf("LocalSymbolID mismatch in dense encoding: want %d, got %d", localSymbolID, extractedLocalSymbolID)
	}
}