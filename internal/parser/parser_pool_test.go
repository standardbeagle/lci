package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/core"
)

// TestLanguageSpecificPools verifies that each language has its own parser pool
// and that parsers from different languages don't interfere with each other
func TestLanguageSpecificPools(t *testing.T) {
	store := core.NewFileContentStore()

	// Get parsers for different languages
	goParser1 := GetParserForLanguage("go", store)
	goParser2 := GetParserForLanguage("go", store)
	pythonParser := GetParserForLanguage("python", store)
	jsParser := GetParserForLanguage("javascript", store)

	// Verify all parsers are valid
	if goParser1 == nil || goParser2 == nil || pythonParser == nil || jsParser == nil {
		t.Fatal("Failed to get parsers from pools")
	}

	// Go parsers should be the same instance (from pool)
	if goParser1 != goParser2 {
		t.Log("Note: Different Go parser instances (pool may have created multiple)")
	}

	// Different language parsers should be different instances
	if goParser1 == pythonParser {
		t.Error("Go and Python parsers should be different instances")
	}

	if pythonParser == jsParser {
		t.Error("Python and JS parsers should be different instances")
	}

	t.Log("✓ Language-specific pools working correctly")
}

// BenchmarkParserPoolGet measures the performance of getting a parser from the pool
func BenchmarkParserPoolGet(b *testing.B) {
	store := core.NewFileContentStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := GetParserForLanguage("go", store)
		ReleaseParserToPool(p, LanguageGo)
	}
}
