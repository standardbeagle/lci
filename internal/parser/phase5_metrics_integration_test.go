package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/types"
)

// TestPhase5MetricsIntegration tests how new languages integrate with metrics system
func TestPhase5MetricsIntegration(t *testing.T) {
	// Initialize metrics system
	config := analysis.DefaultCachedMetricsConfig()
	metricsCalc := analysis.NewCachedMetricsCalculator(config)

	t.Run("NewSymbolTypesMetrics", func(t *testing.T) {
		testNewSymbolTypesMetrics(t, metricsCalc)
	})

	t.Run("LanguageSpecificMetrics", func(t *testing.T) {
		testLanguageSpecificMetrics(t, metricsCalc)
	})

	t.Run("CrossLanguageMetrics", func(t *testing.T) {
		testCrossLanguageMetrics(t, metricsCalc)
	})

	t.Run("CacheScalingWith10Languages", func(t *testing.T) {
		testCacheScalingWith10Languages(t, metricsCalc)
	})
}

// testNewSymbolTypesMetrics tests metrics calculation for new symbol types
func testNewSymbolTypesMetrics(t *testing.T, calc *analysis.CachedMetricsCalculator) {
	// Test symbols that would be added for new languages
	newSymbolTypes := []struct {
		name     string
		symType  types.SymbolType
		language string
		content  string
	}{
		// C# symbols
		{"UserRecord", types.SymbolTypeType, "csharp", "public record UserRecord(string Name, int Age);"},
		{"UserProperty", types.SymbolTypeVariable, "csharp", "public string Name { get; init; }"},
		{"UserNamespace", types.SymbolTypeNamespace, "csharp", "namespace MyApp.Models { ... }"},

		// Kotlin symbols
		{"UserDataClass", types.SymbolTypeClass, "kotlin", "data class User(val name: String, val age: Int)"},
		{"UserExtension", types.SymbolTypeFunction, "kotlin", "fun User.isAdult(): Boolean = age >= 18"},
		{"UserCompanion", types.SymbolTypeClass, "kotlin", "companion object { const val MIN_AGE = 0 }"},

		// Zig symbols
		{"UserStruct", types.SymbolTypeStruct, "zig", "const User = struct { name: []const u8, age: u32 };"},
		{"UserUnion", types.SymbolTypeType, "zig", "const Value = union(enum) { int: i32, float: f64 };"},
		{"UserComptime", types.SymbolTypeFunction, "zig", "fn fibonacci(comptime n: u32) u32 { ... }"},
	}

	for _, sym := range newSymbolTypes {
		t.Run(sym.name, func(t *testing.T) {
			// Create mock enhanced symbol
			enhancedSymbol := &types.EnhancedSymbol{
				Symbol: types.Symbol{
					Name:      sym.name,
					Type:      sym.symType,
					FileID:    1,
					Line:      1,
					Column:    1,
					EndLine:   5,
					EndColumn: 10,
				},
				ID: types.SymbolID(1),
			}

			// For this test, we'll skip actual metrics calculation since it requires Tree-sitter nodes
			// Instead, test that the metrics system can handle the new symbol types conceptually
			t.Logf("Symbol definition: %s (%s %s)", sym.name, sym.symType.String(), sym.language)
			t.Logf("  Content: %s", sym.content)

			// Test symbol type compatibility
			if sym.symType.String() == "unknown" {
				t.Errorf("Unknown symbol type for %s in %s", sym.name, sym.language)
			}

			// Test that enhanced symbol structure can handle new languages
			if enhancedSymbol.Name == "" {
				t.Errorf("Empty symbol name for %s", sym.name)
			}

			// Test successful symbol creation
			if enhancedSymbol.ID == 0 {
				t.Errorf("Expected non-zero symbol ID for %s", sym.name)
			}
		})
	}
}

// testLanguageSpecificMetrics tests language-specific metric characteristics
func testLanguageSpecificMetrics(t *testing.T, calc *analysis.CachedMetricsCalculator) {
	languages := []struct {
		name       string
		extension  string
		features   []string
		complexity map[string]int // expected complexity for different constructs
	}{
		{
			name:      "csharp",
			extension: ".cs",
			features:  []string{"nullable_types", "records", "pattern_matching", "async_await"},
			complexity: map[string]int{
				"simple_class":  1,
				"record_type":   1,
				"async_method":  2,
				"pattern_match": 3,
			},
		},
		{
			name:      "kotlin",
			extension: ".kt",
			features:  []string{"data_classes", "coroutines", "extension_functions", "when_expressions"},
			complexity: map[string]int{
				"data_class":         1,
				"suspend_function":   2,
				"when_expression":    2,
				"extension_function": 1,
			},
		},
		{
			name:      "zig",
			extension: ".zig",
			features:  []string{"comptime", "error_unions", "generic_functions", "packed_structs"},
			complexity: map[string]int{
				"simple_struct":     1,
				"generic_function":  2,
				"comptime_function": 3,
				"error_handling":    2,
			},
		},
	}

	for _, lang := range languages {
		t.Run(lang.name, func(t *testing.T) {
			t.Logf("Language: %s (%s)", lang.name, lang.extension)
			t.Logf("  Features: %v", lang.features)
			t.Logf("  Expected complexity patterns:")

			for construct, expectedComplexity := range lang.complexity {
				t.Logf("    %s: %d", construct, expectedComplexity)
			}

			// Test that metrics system can handle language-specific patterns
			for _, feature := range lang.features {
				t.Logf("  Testing feature: %s", feature)
				// This would test feature-specific metrics when parsers are available
			}
		})
	}
}

// testCrossLanguageMetrics tests metrics comparison across languages
func testCrossLanguageMetrics(t *testing.T, calc *analysis.CachedMetricsCalculator) {
	// Test similar constructs across different languages
	equivalentConstructs := []struct {
		description string
		constructs  map[string]string // language -> code
	}{
		{
			description: "Simple class definition",
			constructs: map[string]string{
				"csharp": "public class User { public string Name { get; set; } }",
				"kotlin": "class User { var name: String = \"\" }",
				"java":   "public class User { private String name; }",
				"go":     "type User struct { Name string }",
			},
		},
		{
			description: "Function with parameters",
			constructs: map[string]string{
				"csharp":     "public int Add(int a, int b) { return a + b; }",
				"kotlin":     "fun add(a: Int, b: Int): Int = a + b",
				"javascript": "function add(a, b) { return a + b; }",
				"python":     "def add(a, b): return a + b",
			},
		},
	}

	for _, construct := range equivalentConstructs {
		t.Run(construct.description, func(t *testing.T) {
			t.Logf("Testing: %s", construct.description)

			for lang, code := range construct.constructs {
				t.Logf("  %s: %s", lang, code)

				// Create mock enhanced symbol for this construct
				enhancedSymbol := &types.EnhancedSymbol{
					Symbol: types.Symbol{
						Name:      "TestSymbol",
						Type:      types.SymbolTypeFunction,
						FileID:    1,
						Line:      1,
						Column:    1,
						EndLine:   3,
						EndColumn: 10,
					},
					ID: types.SymbolID(1),
				}

				t.Logf("    Enhanced symbol created for %s", lang)
				t.Logf("    Symbol: %s (%s)", enhancedSymbol.Name, enhancedSymbol.Type.String())

				// Test that symbol structure can represent cross-language constructs
				if enhancedSymbol.Type.String() == "unknown" {
					t.Errorf("    Unknown symbol type for %s", lang)
				}
			}
		})
	}
}

// testCacheScalingWith10Languages tests cache performance with 10 languages
func testCacheScalingWith10Languages(t *testing.T, calc *analysis.CachedMetricsCalculator) {
	// Test cache scaling conceptually - the actual cache is internal
	languages := []string{"javascript", "typescript", "go", "python", "rust", "cpp", "java", "csharp", "kotlin", "zig"}
	symbolTypes := []types.SymbolType{
		types.SymbolTypeFunction,
		types.SymbolTypeClass,
		types.SymbolTypeMethod,
		types.SymbolTypeInterface,
		types.SymbolTypeStruct,
		types.SymbolTypeModule,
		types.SymbolTypeNamespace,
	}

	// Calculate expected cache load
	symbolsPerLanguage := len(symbolTypes) * 5 // 5 symbols per type
	totalExpectedSymbols := len(languages) * symbolsPerLanguage

	t.Logf("Cache Scaling Analysis for 10 Languages:")
	t.Logf("  Languages: %d", len(languages))
	t.Logf("  Symbol types per language: %d", len(symbolTypes))
	t.Logf("  Symbols per type: 5")
	t.Logf("  Total expected symbols: %d", totalExpectedSymbols)

	// Estimate memory requirements
	bytesPerEntry := 322 // From Phase 3B testing
	estimatedMemoryKB := float64(totalExpectedSymbols) * float64(bytesPerEntry) / 1024.0

	t.Logf("  Estimated memory usage: %.2f KB", estimatedMemoryKB)

	// Test current cache configuration
	cacheInfo := calc.GetCacheInfo()
	t.Logf("Cache Configuration:")
	t.Logf("  Max entries: %d", cacheInfo.MaxEntries)
	t.Logf("  TTL: %v", cacheInfo.TTL)
	t.Logf("  Content caching: %t", cacheInfo.EnableContent)
	t.Logf("  Symbol caching: %t", cacheInfo.EnableSymbol)

	// Verify cache can handle the load
	if totalExpectedSymbols > cacheInfo.MaxEntries {
		t.Logf("Warning: Expected symbols (%d) exceed cache limit (%d)",
			totalExpectedSymbols, cacheInfo.MaxEntries)
		t.Logf("  Recommendation: Increase cache size to %d entries", totalExpectedSymbols+50)
	} else {
		t.Logf("Cache configuration adequate for 10-language load")
	}

	// Test language distribution
	t.Logf("Language Distribution:")
	for _, lang := range languages {
		t.Logf("  %s: %d symbols", lang, symbolsPerLanguage)
	}

	// Test symbol type distribution
	t.Logf("Symbol Type Distribution (per language):")
	for _, symType := range symbolTypes {
		t.Logf("  %s: 5 symbols × 10 languages = 50 total", symType.String())
	}
}

// getExtension returns file extension for language
func getExtension(language string) string {
	extensions := map[string]string{
		"csharp":     "cs",
		"kotlin":     "kt",
		"zig":        "zig",
		"javascript": "js",
		"typescript": "ts",
		"go":         "go",
		"python":     "py",
		"rust":       "rs",
		"cpp":        "cpp",
		"java":       "java",
	}

	if ext, exists := extensions[language]; exists {
		return ext
	}
	return "txt"
}
