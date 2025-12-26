package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolTypeResolver_ExactMatch(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	tests := []struct {
		input    string
		expected string
	}{
		{"function", "function"},
		{"FUNCTION", "function"}, // Case insensitive
		{"Function", "function"},
		{"variable", "variable"},
		{"class", "class"},
		{"type", "type"},
		{"constant", "constant"},
		{"method", "method"},
		{"interface", "interface"},
		{"struct", "struct"},
		{"module", "module"},
		{"namespace", "namespace"},
		{"property", "property"},
		{"event", "event"},
		{"delegate", "delegate"},
		{"enum", "enum"},
		{"record", "record"},
		{"operator", "operator"},
		{"indexer", "indexer"},
		{"object", "object"},
		{"companion", "companion"},
		{"extension", "extension"},
		{"annotation", "annotation"},
		{"field", "field"},
		{"enum_member", "enum_member"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := resolver.Resolve(tc.input)
			assert.Equal(t, tc.expected, result.Resolved)
			assert.Equal(t, "exact", result.MatchType)
			assert.Empty(t, result.Warning)
		})
	}
}

func TestSymbolTypeResolver_AliasMatch(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	tests := []struct {
		input    string
		expected string
	}{
		// Common abbreviations
		{"func", "function"},
		{"var", "variable"},
		{"const", "constant"},
		{"cls", "class"},
		{"meth", "method"},
		{"iface", "interface"},
		{"prop", "property"},
		{"ns", "namespace"},
		{"mod", "module"},
		// Language-specific (Python)
		{"def", "function"},
		// Language-specific (Rust)
		{"fn", "function"},
		// Note: "trait" and "impl" are now first-class types, not aliases
		// Language-specific (Swift)
		{"protocol", "interface"},
		// Language-specific (JS/Kotlin/Scala)
		{"let", "variable"},
		{"val", "variable"},
		// Plural forms
		{"functions", "function"},
		{"variables", "variable"},
		{"classes", "class"},
		{"methods", "method"},
		{"interfaces", "interface"},
		{"constants", "constant"},
		{"structs", "struct"},
		{"enums", "enum"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := resolver.Resolve(tc.input)
			assert.Equal(t, tc.expected, result.Resolved)
			assert.Equal(t, "alias", result.MatchType)
			assert.Empty(t, result.Warning) // Aliases don't generate warnings
		})
	}
}

func TestSymbolTypeResolver_PrefixMatch(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	tests := []struct {
		input    string
		expected string
	}{
		{"fun", "function"},
		{"cla", "class"},
		{"str", "struct"},
		{"met", "method"},
		{"int", "interface"},
		{"var", "variable"}, // This is actually an alias, not prefix
		{"con", "constant"},
		{"nam", "namespace"},
		{"pro", "property"},
		{"ann", "annotation"},
		{"del", "delegate"},
		{"ext", "extension"},
		{"enu", "enum"},
		{"rec", "record"},
		{"ope", "operator"},
		{"ind", "indexer"},
		{"fie", "field"},
		{"com", "companion"},
		{"obj", "object"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := resolver.Resolve(tc.input)
			assert.Equal(t, tc.expected, result.Resolved)
			// Could be alias or prefix depending on input
			if result.MatchType == "prefix" {
				assert.NotEmpty(t, result.Warning)
				assert.Contains(t, result.Warning, "prefix match")
			}
		})
	}
}

func TestSymbolTypeResolver_FuzzyMatch(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	tests := []struct {
		input    string
		expected string
	}{
		{"functon", "function"},   // Missing 'i'
		{"functoin", "function"},  // Transposed 'io'
		{"fuction", "function"},   // Missing 'n'
		{"variabel", "variable"},  // Typo
		{"calss", "class"},        // Transposition
		{"methd", "method"},       // Missing 'o'
		{"interace", "interface"}, // Missing 'f'
		{"strcut", "struct"},      // Transposition
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := resolver.Resolve(tc.input)
			assert.Equal(t, tc.expected, result.Resolved)
			assert.Equal(t, "fuzzy", result.MatchType)
			assert.NotEmpty(t, result.Warning)
			assert.Contains(t, result.Warning, "did you mean")
		})
	}
}

func TestSymbolTypeResolver_NoMatch(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	tests := []string{
		"xyz",
		"foobar",
		"invalid",
		"notarealtype",
		"ab", // Too short for prefix
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			result := resolver.Resolve(input)
			assert.Empty(t, result.Resolved)
			assert.Equal(t, "none", result.MatchType)
			assert.NotEmpty(t, result.Warning)
			assert.Contains(t, result.Warning, "unknown symbol type")
		})
	}
}

func TestSymbolTypeResolver_EmptyInput(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	tests := []string{"", "  ", "\t", "\n"}

	for _, input := range tests {
		t.Run("empty", func(t *testing.T) {
			result := resolver.Resolve(input)
			assert.Empty(t, result.Resolved)
			assert.Equal(t, "none", result.MatchType)
		})
	}
}

func TestSymbolTypeResolver_ResolveAll(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	t.Run("comma-separated valid types", func(t *testing.T) {
		resolved, warnings := resolver.ResolveAll("function,class,method")
		assert.Equal(t, []string{"function", "class", "method"}, resolved)
		assert.Empty(t, warnings)
	})

	t.Run("comma-separated with aliases", func(t *testing.T) {
		resolved, warnings := resolver.ResolveAll("func,cls,meth")
		assert.Equal(t, []string{"function", "class", "method"}, resolved)
		assert.Empty(t, warnings) // Aliases don't warn
	})

	t.Run("comma-separated with fuzzy matches", func(t *testing.T) {
		resolved, warnings := resolver.ResolveAll("functon,calss")
		assert.Equal(t, []string{"function", "class"}, resolved)
		assert.Len(t, warnings, 2)
	})

	t.Run("deduplication", func(t *testing.T) {
		resolved, _ := resolver.ResolveAll("func,function,fn,def")
		assert.Equal(t, []string{"function"}, resolved)
	})

	t.Run("mixed valid and invalid", func(t *testing.T) {
		resolved, warnings := resolver.ResolveAll("function,invalid,class")
		assert.Equal(t, []string{"function", "class"}, resolved)
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "unknown symbol type")
	})

	t.Run("handles whitespace", func(t *testing.T) {
		resolved, _ := resolver.ResolveAll(" function , class , method ")
		assert.Equal(t, []string{"function", "class", "method"}, resolved)
	})

	t.Run("empty input", func(t *testing.T) {
		resolved, warnings := resolver.ResolveAll("")
		assert.Nil(t, resolved)
		assert.Nil(t, warnings)
	})
}

func TestCanonicalSymbolTypes_Complete(t *testing.T) {
	// Verify all 26 canonical types are defined (matches types.SymbolType enum)
	assert.Len(t, CanonicalSymbolTypes, 26, "Should have 26 canonical symbol types")

	// Verify expected types are present (matches SymbolType enum order in types.go)
	expectedTypes := []string{
		"function", "class", "method", "variable", "constant", "interface", "type",
		"struct", "module", "namespace",
		"property", "event", "delegate", "enum", "record", "operator", "indexer",
		"object", "companion", "extension", "annotation",
		"field", "enum_member",
		// Rust specific types (first-class, not aliases)
		"trait", "impl",
		// Constructor
		"constructor",
	}

	for _, expected := range expectedTypes {
		found := false
		for _, canonical := range CanonicalSymbolTypes {
			if canonical == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "CanonicalSymbolTypes should contain '%s'", expected)
	}
}

func TestGetValidTypesDescription(t *testing.T) {
	desc := GetValidTypesDescription()

	// Should mention all canonical types
	for _, canonical := range CanonicalSymbolTypes {
		assert.Contains(t, desc, canonical)
	}

	// Should mention key aliases
	assert.Contains(t, desc, "func->function")
	assert.Contains(t, desc, "def->function")
	assert.Contains(t, desc, "fn->function")

	// Should mention fuzzy matching
	assert.Contains(t, strings.ToLower(desc), "fuzzy")
}

func TestSymbolTypeResolver_CaseInsensitive(t *testing.T) {
	resolver := NewSymbolTypeResolver()

	testCases := []string{"FUNCTION", "Function", "FuNcTiOn", "function"}

	for _, tc := range testCases {
		result := resolver.Resolve(tc)
		assert.Equal(t, "function", result.Resolved)
		assert.Equal(t, "exact", result.MatchType)
	}
}

func BenchmarkSymbolTypeResolver_Resolve(b *testing.B) {
	resolver := NewSymbolTypeResolver()
	inputs := []string{"function", "func", "functon", "xyz"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			resolver.Resolve(input)
		}
	}
}

func BenchmarkSymbolTypeResolver_ResolveAll(b *testing.B) {
	resolver := NewSymbolTypeResolver()
	input := "function,func,class,method,struct,interface"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.ResolveAll(input)
	}
}
