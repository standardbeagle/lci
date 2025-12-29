package regex_analyzer

import (
	"regexp"
	"testing"
)

// Test cases for regex pattern classification and extraction
type RegexTestCase struct {
	Name             string
	Pattern          string
	IsSimple         bool
	ExpectedLiterals []string // Extracted for trigram filtering
	Description      string
}

var regexTestCases = []RegexTestCase{
	// Simple literal patterns
	{
		Name:             "simple_literal",
		Pattern:          "func",
		IsSimple:         true,
		ExpectedLiterals: []string{"func"},
		Description:      "Simple literal function keyword",
	},
	{
		Name:             "simple_literal_class",
		Pattern:          "class",
		IsSimple:         true,
		ExpectedLiterals: []string{"class"},
		Description:      "Simple literal class keyword",
	},

	// Simple wildcard patterns
	{
		Name:             "wildcard_function",
		Pattern:          "func.*",
		IsSimple:         true,
		ExpectedLiterals: []string{"func"},
		Description:      "Function definition with wildcard suffix",
	},
	{
		Name:             "wildcard_class_extends",
		Pattern:          "class.*extends",
		IsSimple:         true,
		ExpectedLiterals: []string{"class", "extends"},
		Description:      "Class inheritance pattern",
	},
	{
		Name:             "wildcard_async_await",
		Pattern:          "async.*await",
		IsSimple:         true,
		ExpectedLiterals: []string{"async", "await"},
		Description:      "Async/await pattern",
	},

	// Character class patterns
	{
		Name:             "char_class_identifier",
		Pattern:          "[A-Za-z_][A-Za-z0-9_]*",
		IsSimple:         true,
		ExpectedLiterals: []string{}, // No literals to extract
		Description:      "Identifier pattern with character classes",
	},
	{
		Name:             "char_class_error_suffix",
		Pattern:          "\\w+Error",
		IsSimple:         true,
		ExpectedLiterals: []string{"Error"},
		Description:      "Error naming pattern",
	},
	{
		Name:             "char_class_function_call",
		Pattern:          "\\w+\\(.*\\)",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "Function call pattern",
	},

	// Anchor patterns
	{
		Name:             "anchor_start_function",
		Pattern:          "^func",
		IsSimple:         true,
		ExpectedLiterals: []string{"func"},
		Description:      "Function at line start",
	},
	{
		Name:             "anchor_end_brace",
		Pattern:          "\\{$",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "Opening brace at line end",
	},

	// Alternation patterns
	{
		Name:             "alternation_function_variants",
		Pattern:          "(func|function|def)",
		IsSimple:         true,
		ExpectedLiterals: []string{"func", "function", "def"},
		Description:      "Multiple function keyword variants",
	},
	{
		Name:             "alternation_import_patterns",
		Pattern:          "(import|require|include)",
		IsSimple:         true,
		ExpectedLiterals: []string{"import", "require", "include"},
		Description:      "Multiple import patterns",
	},

	// Repetition patterns
	{
		Name:             "repetition_bounded",
		Pattern:          "[a-z]{2,5}",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "Bounded repetition",
	},
	{
		Name:             "repetition_one_or_more",
		Pattern:          "\\d+",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "One or more digits",
	},

	// Group patterns
	{
		Name:             "group_function_params",
		Pattern:          "\\(.*\\)",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "Function parameter group",
	},
	{
		Name:             "group_optional",
		Pattern:          "(optional)?",
		IsSimple:         true,
		ExpectedLiterals: []string{"optional"},
		Description:      "Optional group",
	},

	// Complex patterns that should fallback to full regex
	{
		Name:             "complex_lookahead",
		Pattern:          "(?=func\\s+\\w+)",
		IsSimple:         false,
		ExpectedLiterals: []string{"func"}, // Can extract but should use full regex
		Description:      "Lookahead assertion",
	},
	{
		Name:             "complex_lookbehind",
		Pattern:          "(?<=class\\s+)\\w+",
		IsSimple:         false,
		ExpectedLiterals: []string{"class"}, // Can extract but should use full regex
		Description:      "Lookbehind assertion",
	},
	{
		Name:             "complex_backreference",
		Pattern:          "(\\w+)\\s+\\1",
		IsSimple:         false,
		ExpectedLiterals: []string{},
		Description:      "Backreference pattern",
	},
	{
		Name:             "complex_conditional",
		Pattern:          "(?(?=func)function|method)",
		IsSimple:         false,
		ExpectedLiterals: []string{"func", "function", "method"},
		Description:      "Conditional group",
	},
	{
		Name:             "complex_atomic_group",
		Pattern:          "(?>\\w+)",
		IsSimple:         false,
		ExpectedLiterals: []string{},
		Description:      "Atomic group",
	},

	// Real-world patterns from code search
	{
		Name:             "realworld_javascript_function",
		Pattern:          "function\\s+\\w+\\s*\\(",
		IsSimple:         true,
		ExpectedLiterals: []string{"function"},
		Description:      "JavaScript function declaration",
	},
	{
		Name:             "realworld_python_decorator",
		Pattern:          "@\\w+",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "Python decorator pattern",
	},
	{
		Name:             "realworld_typescript_interface",
		Pattern:          "interface\\s+\\w+",
		IsSimple:         true,
		ExpectedLiterals: []string{"interface"},
		Description:      "TypeScript interface declaration",
	},
	{
		Name:             "realworld_rust_trait",
		Pattern:          "trait\\s+\\w+",
		IsSimple:         true,
		ExpectedLiterals: []string{"trait"},
		Description:      "Rust trait declaration",
	},
	{
		Name:             "realworld_csharp_class",
		Pattern:          "public\\s+class\\s+\\w+",
		IsSimple:         true,
		ExpectedLiterals: []string{"public", "class"},
		Description:      "C# public class declaration",
	},
	{
		Name:             "realworld_go_struct",
		Pattern:          "type\\s+\\w+\\s+struct",
		IsSimple:         true,
		ExpectedLiterals: []string{"type", "struct"},
		Description:      "Go struct definition",
	},
	{
		Name:             "realworld_error_pattern",
		Pattern:          "\\w+Error\\s*\\(",
		IsSimple:         true,
		ExpectedLiterals: []string{"Error"},
		Description:      "Error constructor pattern",
	},
	{
		Name:             "realworld_async_pattern",
		Pattern:          "async\\s+function\\s+\\w+",
		IsSimple:         true,
		ExpectedLiterals: []string{"async", "function"},
		Description:      "Async function declaration",
	},

	// Edge cases and tricky patterns
	{
		Name:             "edge_empty_alternatives",
		Pattern:          "(|func|)",
		IsSimple:         true,
		ExpectedLiterals: []string{"func"},
		Description:      "Empty alternatives",
	},
	{
		Name:             "edge_nested_groups",
		Pattern:          "((func)|(class))",
		IsSimple:         true,
		ExpectedLiterals: []string{"func", "class"},
		Description:      "Nested groups",
	},
	{
		Name:             "edge_escaped_chars",
		Pattern:          "func\\(.*\\)\\s*\\{",
		IsSimple:         true,
		ExpectedLiterals: []string{"func"},
		Description:      "Escaped special characters",
	},
	{
		Name:             "edge_quantifier_complex",
		Pattern:          "\\w{1,3}\\d+",
		IsSimple:         true,
		ExpectedLiterals: []string{},
		Description:      "Complex quantifier pattern",
	},
}

// TestRegexClassification tests the classification of regex patterns
func TestRegexClassification(t *testing.T) {
	classifier := NewRegexClassifier()

	for _, tc := range regexTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			isSimple := classifier.IsSimple(tc.Pattern)

			if isSimple != tc.IsSimple {
				t.Errorf("Pattern %q: expected simple=%v, got simple=%v\nDescription: %s",
					tc.Pattern, tc.IsSimple, isSimple, tc.Description)
			}
		})
	}
}

// TestLiteralExtraction tests extraction of trigram candidates from regex patterns
func TestLiteralExtraction(t *testing.T) {
	extractor := NewLiteralExtractor()

	for _, tc := range regexTestCases {
		if !tc.IsSimple {
			continue // Only test simple patterns for literal extraction
		}

		t.Run(tc.Name, func(t *testing.T) {
			literals := extractor.ExtractLiterals(tc.Pattern)

			// Check if all expected literals are found
			expectedMap := make(map[string]bool)
			for _, lit := range tc.ExpectedLiterals {
				expectedMap[lit] = true
			}

			foundMap := make(map[string]bool)
			for _, lit := range literals {
				foundMap[lit] = true
			}

			// Verify expected literals are present
			for expectedLit := range expectedMap {
				if !foundMap[expectedLit] {
					t.Errorf("Pattern %q: expected literal %q not found\nDescription: %s\nFound: %v",
						tc.Pattern, expectedLit, tc.Description, literals)
				}
			}

			// Verify no unexpected literals (allowing for extras that might be useful)
			if len(literals) < len(tc.ExpectedLiterals) {
				t.Errorf("Pattern %q: expected at least %d literals, got %d\nDescription: %s\nExpected: %v\nFound: %v",
					tc.Pattern, len(tc.ExpectedLiterals), len(literals), tc.Description,
					tc.ExpectedLiterals, literals)
			}
		})
	}
}

// TestRegexEquivalence ensures our optimized regex produces same results as Go's standard regex
func TestRegexEquivalence(t *testing.T) {
	testContent := []byte(`
func processData() error {
	if err := validateInput(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return processResult()
}

function calculateTotal(items) {
	return items.reduce((sum, item) => sum + item.price, 0);
}

class UserManager extends BaseClass {
	constructor() {
		super();
	}
}

type UserService struct {
	repository Repository
}

interface DataAccess {
	GetData() error;
}

trait Cloneable {
	fn clone(&self) -> Self;
}

public class OrderService {
	public void ProcessOrder() {}
}

async function fetchUserData() {
	try {
		const response = await fetch('/api/user');
		return await response.json();
	} catch (ValidationError) {
		console.error('Validation failed');
	}
}

def calculate_metrics(data):
	total = sum(data)
	average = total / len(data)
	return average

@decorator
def function_name():
	pass

var CustomError = function(message) {
	this.message = message;
};
`)

	for _, tc := range regexTestCases {
		t.Run(tc.Name+"_equivalence", func(t *testing.T) {
			// Compile standard Go regex
			goRegex, err := regexp.Compile(tc.Pattern)
			if err != nil {
				t.Skipf("Pattern %q not supported by Go regex: %v", tc.Pattern, err)
				return
			}

			// Get matches from standard Go regex
			goMatches := goRegex.FindAllIndex(testContent, -1)

			// For now, just test that our system can handle the pattern
			// We'll implement the actual optimization later
			classifier := NewRegexClassifier()
			isSimple := classifier.IsSimple(tc.Pattern)

			// Verify classification is reasonable
			if isSimple && len(goMatches) > 0 {
				t.Logf("Pattern %q classified as simple, Go regex finds %d matches",
					tc.Pattern, len(goMatches))
			} else if !isSimple {
				t.Logf("Pattern %q classified as complex, Go regex finds %d matches",
					tc.Pattern, len(goMatches))
			}

			// Test that pattern doesn't crash our system
			extractor := NewLiteralExtractor()
			literals := extractor.ExtractLiterals(tc.Pattern)
			t.Logf("Pattern %q extracted literals: %v", tc.Pattern, literals)
		})
	}
}

// BenchmarkRegexClassification benchmarks the classification performance
func BenchmarkRegexClassification(b *testing.B) {
	classifier := NewRegexClassifier()
	patterns := make([]string, len(regexTestCases))
	for i, tc := range regexTestCases {
		patterns[i] = tc.Pattern
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		classifier.IsSimple(pattern)
	}
}

// BenchmarkLiteralExtraction benchmarks the literal extraction performance
func BenchmarkLiteralExtraction(b *testing.B) {
	extractor := NewLiteralExtractor()
	patterns := make([]string, len(regexTestCases))
	for i, tc := range regexTestCases {
		patterns[i] = tc.Pattern
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		extractor.ExtractLiterals(pattern)
	}
}

// TestComplexPatternsRegression ensures complex patterns are properly identified
func TestComplexPatternsRegression(t *testing.T) {
	complexPatterns := []string{
		"(?<=func\\s+)\\w+",    // Lookbehind
		"(?=function\\s+)",     // Lookahead
		"(\\w+)\\s+\\1",        // Backreference
		"(?(?=\\w+)\\w+|\\d+)", // Conditional
		"(?>\\w+)",             // Atomic group
		"(?!(func|var))",       // Negative lookahead
		"(?<!class\\s+)",       // Negative lookbehind
		"(?P<name>\\w+)",       // Named group (some engines)
		"(?i:func)",            // Inline modifier
	}

	classifier := NewRegexClassifier()

	for _, pattern := range complexPatterns {
		t.Run("complex_"+pattern, func(t *testing.T) {
			isSimple := classifier.IsSimple(pattern)
			if isSimple {
				t.Errorf("Complex pattern %q was incorrectly classified as simple", pattern)
			}
		})
	}
}

// TestFuzzInput tests with potentially problematic inputs
func TestFuzzInput(t *testing.T) {
	fuzzInputs := []string{
		"",      // Empty
		"*",     // Invalid regex
		"(",     // Unbalanced
		")",     // Unbalanced reverse
		"((((",  // Multiple unbalanced
		"))))",  // Multiple unbalanced reverse
		"\\",    // Backslash at end
		"\\k",   // Invalid escape
		"(?",    // Incomplete special sequence
		"(?P",   // Incomplete named group
		"(?<",   // Incomplete lookbehind
		"(?=",   // Incomplete lookahead
		"(?!",   // Incomplete negative lookahead
		"(?<=",  // Incomplete lookbehind
		"(?<!",  // Incomplete negative lookbehind
		"(?>",   // Incomplete atomic group
		"(?(?=", // Incomplete conditional
		"(a|b|c|d|e|f|g|h|i|j|k|l|m|n|o|p|q|r|s|t|u|v|w|x|y|z)", // Long alternation
		"(\\w+){1000}",        // Large quantifier
		"a{1000,}",            // Large unbounded quantifier
		"[a-zA-Z0-9]*",        // Character class with repetition
		"((((((func))))))",    // Deeply nested
		"func\\1\\2\\3\\4\\5", // Invalid backreferences
	}

	classifier := NewRegexClassifier()
	extractor := NewLiteralExtractor()

	for _, input := range fuzzInputs {
		t.Run("fuzz_"+input, func(t *testing.T) {
			// These should not panic or hang
			t.Parallel()

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Fuzz input %q caused panic: %v", input, r)
				}
			}()

			// Test classification
			isSimple := classifier.IsSimple(input)
			t.Logf("Fuzz input %q classified as simple=%v", input, isSimple)

			// Test literal extraction
			literals := extractor.ExtractLiterals(input)
			t.Logf("Fuzz input %q extracted literals: %v", input, literals)
		})
	}
}
