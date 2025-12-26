package mcp

import (
	"encoding/json"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestSymbolTypeStringConversion is a unit test that validates the fix
// for the symbol type serialization bug where string(SymbolType) was used
// instead of SymbolType.String(), resulting in escape sequences.
func TestSymbolTypeStringConversion(t *testing.T) {
	tests := []struct {
		symbolType types.SymbolType
		want       string
	}{
		{types.SymbolTypeFunction, "function"},
		{types.SymbolTypeClass, "class"},
		{types.SymbolTypeMethod, "method"},
		{types.SymbolTypeVariable, "variable"},
		{types.SymbolTypeConstant, "constant"},
		{types.SymbolTypeInterface, "interface"},
		{types.SymbolTypeType, "type"},
		{types.SymbolTypeStruct, "struct"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			// Test String() method (CORRECT)
			correct := tt.symbolType.String()
			if correct != tt.want {
				t.Errorf("SymbolType.String() = %q, want %q", correct, tt.want)
			}

			// Demonstrate the bug (what was happening before)
			incorrect := string(tt.symbolType)
			if len(incorrect) > 0 && incorrect[0] < 32 {
				t.Logf("✓ Confirmed: string(SymbolType) produces escape sequence %q (bytes: %v)",
					incorrect, []byte(incorrect))
			}

			// Validate they are different
			if correct == incorrect {
				t.Errorf("BUG NOT DEMONSTRATED: string(SymbolType) should produce escape sequence, got %q", incorrect)
			}

			t.Logf("SymbolType(%d).String() = %q (correct)", tt.symbolType, correct)
			t.Logf("string(SymbolType(%d)) = %q (incorrect - escape sequence)", tt.symbolType, incorrect)
		})
	}
}

// TestScopeTypeStringConversion validates ScopeType serialization
func TestScopeTypeStringConversion(t *testing.T) {
	tests := []struct {
		scopeType types.ScopeType
		want      string
	}{
		{types.ScopeTypeFolder, "folder"},
		{types.ScopeTypeFile, "file"},
		{types.ScopeTypeNamespace, "namespace"},
		{types.ScopeTypeClass, "class"},
		{types.ScopeTypeInterface, "interface"},
		{types.ScopeTypeFunction, "function"},
		{types.ScopeTypeMethod, "method"},
		{types.ScopeTypeVariable, "variable"},
		{types.ScopeTypeBlock, "block"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			correct := tt.scopeType.String()
			if correct != tt.want {
				t.Errorf("ScopeType.String() = %q, want %q", correct, tt.want)
			}

			incorrect := string(tt.scopeType)
			if correct == incorrect {
				t.Errorf("BUG NOT DEMONSTRATED: string(ScopeType) should differ from .String()")
			}
		})
	}
}

// TestCompactSearchResultSerialization validates that CompactSearchResult
// serializes symbol_type correctly when marshaled to JSON
func TestCompactSearchResultSerialization(t *testing.T) {
	result := CompactSearchResult{
		ObjectID:   "VE",
		File:       "/test/file.go",
		Line:       42,
		Column:     10,
		Match:      "MyFunction",
		Score:      100.0,
		SymbolType: "function", // This should be set using .String() method
		SymbolName: "MyFunction",
		IsExported: true,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Validate symbol_type field in JSON
	if !containsSubstring(jsonStr, "\"symbol_type\":\"function\"") {
		t.Errorf("JSON does not contain correct symbol_type field")
		t.Errorf("Got JSON: %s", jsonStr)
	}

	// Unmarshal to verify round-trip
	var unmarshaled CompactSearchResult
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if unmarshaled.SymbolType != "function" {
		t.Errorf("Unmarshaled symbol_type = %q, want \"function\"", unmarshaled.SymbolType)
	}

	t.Logf("✓ CompactSearchResult serialization correct: symbol_type=%q", unmarshaled.SymbolType)
}

// TestScopeBreadcrumbSerialization validates ScopeBreadcrumb serialization
func TestScopeBreadcrumbSerialization(t *testing.T) {
	breadcrumb := ScopeBreadcrumb{
		ScopeType:  "function", // Should be set using .String() method
		Name:       "MyFunction",
		StartLine:  10,
		EndLine:    20,
		Language:   "go",
		Visibility: "public",
	}

	jsonBytes, err := json.Marshal(breadcrumb)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(jsonBytes)

	if !containsSubstring(jsonStr, "\"scope_type\":\"function\"") {
		t.Errorf("JSON does not contain correct scope_type field")
		t.Errorf("Got JSON: %s", jsonStr)
	}

	var unmarshaled ScopeBreadcrumb
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if unmarshaled.ScopeType != "function" {
		t.Errorf("Unmarshaled scope_type = %q, want \"function\"", unmarshaled.ScopeType)
	}

	t.Logf("✓ ScopeBreadcrumb serialization correct: scope_type=%q", unmarshaled.ScopeType)
}
