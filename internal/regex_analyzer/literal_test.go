package regex_analyzer_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/regex_analyzer"
)

// TestLiteralExtraction tests literal extraction from regex patterns
func TestLiteralExtraction(t *testing.T) {
	extractor := regex_analyzer.NewLiteralExtractor()

	testCases := []struct {
		pattern      string
		expectedLits []string
		description   string
	}{
		{
			pattern:      "Function[0-9]+",
			expectedLits: []string{"Function"},
			description:   "Regex with character class should extract literal prefix",
		},
		{
			pattern:      "test.*[0-9]+",
			expectedLits: []string{"test"},
			description:   "Regex with wildcard should extract literal part",
		},
		{
			pattern:      "(Function|Method)[0-9]+",
			expectedLits: []string{"Function", "Method"},
			description:   "Alternation should extract both alternatives",
		},
		{
			pattern:      "[0-9]+",
			expectedLits: []string{},
			description:   "Only character class should extract no literals",
		},
		{
			pattern:      "a.*b.*c",
			expectedLits: []string{},
			description:   "Single character literals too short for trigrams",
		},
		{
			pattern:      "abc.*def",
			expectedLits: []string{"abc", "def"},
			description:   "Multiple 3+ char literals should be extracted",
		},
		{
			pattern:      "CreateUser.*[a-zA-Z]+",
			expectedLits: []string{"CreateUser"},
			description:   "Should extract CreateUser literal",
		},
		{
			pattern:      "(class|function).*test",
			expectedLits: []string{"class", "function", "test"},
			description:   "Should extract all three literals",
		},
		{
			pattern:      "get.*[a-zA-Z]+ById",
			expectedLits: []string{"get", "ById"},
			description:   "Should extract get and ById literals",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			literals := extractor.ExtractLiterals(tc.pattern)
			t.Logf("Pattern '%s' extracts: %v", tc.pattern, literals)

			// Check if expected literals are found
			for _, expected := range tc.expectedLits {
				found := false
				for _, actual := range literals {
					if actual == expected {
						found = true
						t.Logf("✅ Found expected literal '%s'", expected)
						break
					}
				}
				if !found {
					t.Errorf("Expected literal '%s' not found in extracted: %v", expected, literals)
				}
			}

			// Check for unexpected literals
			if len(literals) != len(tc.expectedLits) {
				t.Errorf("Expected %d literals, got %d: %v", len(tc.expectedLits), len(literals), literals)
			}
		})
	}
}

// TestSpecificPatternForProfiling tests the exact pattern from our profiling
func TestSpecificPatternForProfiling(t *testing.T) {
	extractor := regex_analyzer.NewLiteralExtractor()

	pattern := "Function[0-9]+"
	literals := extractor.ExtractLiterals(pattern)

	t.Logf("=== Profiling Pattern Analysis ===")
	t.Logf("Pattern: '%s'", pattern)
	t.Logf("Extracted literals: %v", literals)

	// This is the key insight - the regex engine should extract "Function"
	// and use the trigram index to find files containing "Function"
	hasFunctionLiteral := false
	for _, lit := range literals {
		if lit == "Function" {
			hasFunctionLiteral = true
			t.Logf("✅ SUCCESS: Pattern '%s' extracts literal 'Function'", pattern)
			t.Logf("   This literal can be used with trigram index for fast filtering")
			break
		}
	}

	if !hasFunctionLiteral {
		t.Errorf("❌ FAILURE: Pattern '%s' should extract literal 'Function'", pattern)
		t.Logf("   Without literal extraction, regex will fall back to linear scanning")
	}

	// Test that the extracted literal is suitable for trigrams
	if hasFunctionLiteral {
		t.Logf("✅ Trigram suitability:")
		t.Logf("   - Literal 'Function' length: %d chars (>= 3, good for trigrams)", len("Function"))
		t.Logf("   - Contains alphanumeric chars: yes (required for trigrams)")
		t.Logf("   - Can generate trigrams: ['Fun', 'unc', 'nct', 'cti', 'tio', 'ion']")
	}
}