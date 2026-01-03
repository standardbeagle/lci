package types

import (
	"testing"

	"github.com/standardbeagle/lci/internal/encoding"
)

// Test Base63ValueToChar helper function (from encoding package)
func TestValueToChar(t *testing.T) {
	tests := []struct {
		val      uint64
		expected byte
	}{
		{0, 'A'},
		{25, 'Z'},
		{26, 'a'},
		{51, 'z'},
		{52, '0'},
		{61, '9'},
		{62, '_'},
	}

	for _, tt := range tests {
		result := encoding.Base63ValueToChar(tt.val)
		if result != tt.expected {
			t.Errorf("Base63ValueToChar(%d) = %c, expected %c", tt.val, result, tt.expected)
		}
	}
}

// Test Base63CharToValue helper function (from encoding package)
func TestCharToValue(t *testing.T) {
	tests := []struct {
		input    rune
		expected uint64
		hasError bool
	}{
		{'A', 0, false},
		{'Z', 25, false},
		{'a', 26, false},
		{'z', 51, false},
		{'0', 52, false},
		{'9', 61, false},
		{'_', 62, false},
		{'!', 0, true}, // Invalid character
	}

	for _, tt := range tests {
		result, err := encoding.Base63CharToValue(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("Base63CharToValue(%c) expected error but got none", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Base63CharToValue(%c) got unexpected error: %v", tt.input, err)
			} else if result != tt.expected {
				t.Errorf("Base63CharToValue(%c) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

// Test that CompactString and ParseCompactString are inverses
func TestCompactStringRoundTrip(t *testing.T) {
	tests := []struct {
		name            string
		fileID          FileID
		localSymbolID   uint32
		shouldRoundTrip bool
	}{
		{"basic", 1, 100, true},
		{"max values", FileID(1<<30 - 1), 1<<24 - 1, true},
		{"zeros", 0, 0, false}, // Zero values produce empty compact string
		{"mixed", 12345, 67890, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewCompositeSymbolID(tt.fileID, tt.localSymbolID)

			// Convert to compact string
			compact := original.CompactString()

			// Parse back
			parsed, err := ParseCompactString(compact)

			if !tt.shouldRoundTrip {
				// For zeros, we expect an error because compact string is empty
				if err == nil {
					t.Errorf("Expected error for zero values, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseCompactString failed: %v", err)
			}

			// Verify
			if parsed != original {
				t.Errorf("Round trip failed: original=%v, parsed=%v", original, parsed)
			}
		})
	}
}

// Test JSON marshaling roundtrip
func TestCompositeSymbolID_JSONRoundTrip(t *testing.T) {
	original := NewCompositeSymbolID(123, 456)

	// Marshal
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Unmarshal
	var parsed CompositeSymbolID
	err = parsed.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	// Verify
	if parsed != original {
		t.Errorf("JSON round trip failed: original=%v, parsed=%v", original, parsed)
	}
}

// Test symbolKindStrings map completeness
func TestSymbolKindStrings(t *testing.T) {
	// Verify all symbol kinds have string representations
	knownKinds := []SymbolKind{
		SymbolKindPackage, SymbolKindImport, SymbolKindType,
		SymbolKindInterface, SymbolKindStruct, SymbolKindClass,
		SymbolKindFunction, SymbolKindMethod, SymbolKindConstructor,
		SymbolKindVariable, SymbolKindConstant, SymbolKindField,
		SymbolKindProperty, SymbolKindParameter, SymbolKindLabel,
		SymbolKindModule, SymbolKindNamespace, SymbolKindEnum,
		SymbolKindEnumMember, SymbolKindTrait, SymbolKindEvent,
		SymbolKindDelegate, SymbolKindRecord, SymbolKindAttribute,
	}

	for _, kind := range knownKinds {
		if str := kind.String(); str == "" || str == "unknown" {
			t.Errorf("SymbolKind %v has no string representation", kind)
		}
	}
}

// Test resolutionTypeStrings map completeness
func TestResolutionTypeStrings(t *testing.T) {
	// Verify all resolution types have string representations
	knownTypes := []ResolutionType{
		ResolutionFile, ResolutionDirectory, ResolutionPackage,
		ResolutionModule, ResolutionBuiltin, ResolutionExternal,
		ResolutionNotFound, ResolutionInternal, ResolutionError,
	}

	for _, rt := range knownTypes {
		if str := rt.String(); str == "" || str == "unknown" {
			t.Errorf("ResolutionType %v has no string representation", rt)
		}
	}
}

// Benchmark CompactString for performance regression testing
func BenchmarkCompactString(b *testing.B) {
	s := NewCompositeSymbolID(12345, 67890)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = s.CompactString()
	}
}

// Benchmark ParseCompactString for performance regression testing
func BenchmarkParseCompactString(b *testing.B) {
	compact := NewCompositeSymbolID(12345, 67890).CompactString()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ParseCompactString(compact)
	}
}
