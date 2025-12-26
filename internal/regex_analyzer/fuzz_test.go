package regex_analyzer

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// FuzzRegexClassification fuzz tests the regex classifier with random inputs
func FuzzRegexClassification(f *testing.F) {
	// Add seed corpus based on our test cases
	seedPatterns := []string{
		"func",
		"class.*extends",
		"(func|function|def)",
		"async.*await",
		"[A-Za-z_][A-Za-z0-9_]*",
		"\\w+Error",
		"function\\s+\\w+\\s*\\(",
		"type\\s+\\w+\\s+struct",
		"(?<=func\\s+)\\w+", // Complex
		"(\\w+)\\s+\\1",     // Complex
		"(?>\\w+)",          // Complex
		"",                  // Edge case
		"*",                 // Invalid
		"(",                 // Unbalanced
		"))))",              // Unbalanced reverse
		"\\",                // Backslash at end
		"(?",                // Incomplete
		"a{1000,}",          // Large quantifier
		"(a|b|c|d|e|f|g|h|i|j|k|l|m|n|o|p|q|r|s|t|u|v|w|x|y|z)", // Long alternation
	}

	for _, pattern := range seedPatterns {
		f.Add(pattern)
	}

	classifier := NewRegexClassifier()
	f.Fuzz(func(t *testing.T, pattern string) {
		// These operations should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Pattern %q caused panic: %v", pattern, r)
			}
		}()

		// Test classification
		isSimple := classifier.IsSimple(pattern)

		// Test literal extraction
		extractor := NewLiteralExtractor()
		literals := extractor.ExtractLiterals(pattern)

		// Validate extracted literals
		for _, literal := range literals {
			// All literals should be valid UTF-8
			if !utf8.ValidString(literal) {
				t.Errorf("Extracted literal %q is not valid UTF-8", literal)
			}

			// All literals should be reasonable length
			if len(literal) > 1000 {
				t.Errorf("Extracted literal %q is too long (%d chars)", literal, len(literal))
			}

			// All literals should contain at least one alphanumeric character
			hasAlphanumeric := false
			for _, r := range literal {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
					hasAlphanumeric = true
					break
				}
			}
			if !hasAlphanumeric {
				t.Errorf("Extracted literal %q contains no alphanumeric characters", literal)
			}
		}

		// Try to compile with Go's regexp engine to ensure it's generally valid
		// (some patterns may be Go-specific, but they shouldn't crash our classifier)
		if isSimple {
			// Simple patterns should generally be compilable by Go
			_, err := regexp.Compile(pattern)
			if err != nil {
				t.Logf("Simple pattern %q not compilable by Go: %v", pattern, err)
				// This is not necessarily a test failure - our definition of "simple"
				// may be broader than Go's regex support
			}
		}

		// Log interesting patterns for analysis
		if len(literals) > 5 {
			t.Logf("Pattern %q (simple=%v) extracted %d literals: %v",
				pattern, isSimple, len(literals), literals)
		}
	})
}

// FuzzRegexConsistency fuzz tests that our classification is consistent
func FuzzRegexConsistency(f *testing.F) {
	// Add seed corpus
	seedPatterns := []string{
		"func", "class", "async", "await", "error", "type", "interface",
		"func.*", "class.*", "async.*", "\\w+Error",
		"(func|function)", "(import|require)", "\\w+\\(.*\\)",
		"", "*", "(", ")", "\\", "(?", "a{1000}",
	}

	for _, pattern := range seedPatterns {
		f.Add(pattern)
	}

	classifier := NewRegexClassifier()
	f.Fuzz(func(t *testing.T, pattern string) {
		// Multiple calls should return the same result
		result1 := classifier.IsSimple(pattern)
		result2 := classifier.IsSimple(pattern)
		result3 := classifier.IsSimple(pattern)

		if result1 != result2 || result2 != result3 {
			t.Errorf("Inconsistent classification for pattern %q: %v, %v, %v",
				pattern, result1, result2, result3)
		}

		// Literal extraction should also be consistent
		extractor := NewLiteralExtractor()
		literals1 := extractor.ExtractLiterals(pattern)
		literals2 := extractor.ExtractLiterals(pattern)

		if len(literals1) != len(literals2) {
			t.Errorf("Inconsistent literal count for pattern %q: %d vs %d",
				pattern, len(literals1), len(literals2))
		}

		// Check content consistency (order might differ due to map iteration)
		litMap1 := make(map[string]bool)
		litMap2 := make(map[string]bool)
		for _, lit := range literals1 {
			litMap1[lit] = true
		}
		for _, lit := range literals2 {
			litMap2[lit] = true
		}

		if len(litMap1) != len(litMap2) {
			t.Errorf("Inconsistent literal sets for pattern %q: %v vs %v",
				pattern, literals1, literals2)
		}

		for lit := range litMap1 {
			if !litMap2[lit] {
				t.Errorf("Missing literal in second extraction for pattern %q: %q", pattern, lit)
			}
		}
	})
}

// FuzzRegexPerformance fuzz tests with performance monitoring
func FuzzRegexPerformance(f *testing.F) {
	// Add seed corpus with patterns of varying complexity
	seedPatterns := []string{
		// Simple literals
		"func", "class", "async", "await", "error", "type", "interface",

		// Wildcards
		"func.*", "class.*extends", "async.*await", "\\w+Error",

		// Alternations
		"(func|function|def)", "(import|require|include)", "\\w+\\(.*\\)",

		// Character classes
		"[A-Za-z_][A-Za-z0-9_]*", "\\d+", "[a-z]{2,5}",

		// Complex (should be marked as complex)
		"(?<=func\\s+)\\w+", "(\\w+)\\s+\\1", "(?>\\w+)",

		// Edge cases
		"", "*", "(", ")", "\\", "(?", "a{1000}",

		// Very long pattern
		strings.Repeat("func|", 100) + "class",
	}

	for _, pattern := range seedPatterns {
		f.Add(pattern)
	}

	classifier := NewRegexClassifier()
	extractor := NewLiteralExtractor()

	f.Fuzz(func(t *testing.T, pattern string) {
		// Performance must be fast - 10ms max per operation
		const maxTimePerOperation = 10 * time.Millisecond

		start := time.Now()
		isSimple := classifier.IsSimple(pattern)
		classificationTime := time.Since(start)

		if classificationTime > maxTimePerOperation {
			t.Errorf("Classification too slow for pattern %q (len=%d): %v (max: %v)",
				pattern, len(pattern), classificationTime, maxTimePerOperation)
		}

		start = time.Now()
		literals := extractor.ExtractLiterals(pattern)
		extractionTime := time.Since(start)

		if extractionTime > maxTimePerOperation {
			t.Errorf("Extraction too slow for pattern %q (len=%d): %v (max: %v)",
				pattern, len(pattern), extractionTime, maxTimePerOperation)
		}

		// Very long patterns should generally be classified as complex
		if len(pattern) > 1000 && isSimple {
			t.Logf("Very long pattern classified as simple: len=%d, simple=%v", len(pattern), isSimple)
		}

		// Patterns with many alternations should generally be classified as complex
		alternationCount := strings.Count(pattern, "|")
		if alternationCount > 50 && isSimple {
			t.Logf("Pattern with many alternations classified as simple: %d alternations, simple=%v",
				alternationCount, isSimple)
		}

		// Log performance characteristics for analysis
		if classificationTime > 1*time.Millisecond || extractionTime > 1*time.Millisecond {
			t.Logf("Performance for pattern %q (len=%d, simple=%v): classification=%v, extraction=%v, literals=%d",
				pattern, len(pattern), isSimple, classificationTime, extractionTime, len(literals))
		}
	})
}

// FuzzRegexRegression tests specific regression cases
func FuzzRegexRegression(f *testing.F) {
	// Add regression seed cases
	regressionCases := []string{
		// Cases that caused issues in development
		"func.*",           // Should extract "func"
		"class.*extends",   // Should extract "class", "extends"
		"(a|b|c)",          // Should extract alternations
		"(func|)",          // Empty alternative
		"((func)|(class))", // Nested groups
		"func\\(.*\\)\\{",  // Escaped characters
		"[a-zA-Z0-9]*",     // Character class
		"a{1000,}",         // Large quantifier
		"(\\w+){1000}",     // Group with large quantifier
	}

	for _, pattern := range regressionCases {
		f.Add(pattern)
	}

	classifier := NewRegexClassifier()
	extractor := NewLiteralExtractor()

	f.Fuzz(func(t *testing.T, pattern string) {
		// Test that we can handle the pattern without panicking
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Regression pattern %q caused panic: %v", pattern, r)
			}
		}()

		isSimple := classifier.IsSimple(pattern)
		literals := extractor.ExtractLiterals(pattern)

		// Basic sanity checks
		if isSimple && len(literals) == 0 && len(pattern) > 2 {
			// Simple patterns with no literals might indicate we're missing something
			t.Logf("Simple pattern with no literals extracted: %q", pattern)
		}

		// All literals should be reasonable
		for _, literal := range literals {
			if len(literal) < 3 {
				t.Errorf("Literal too short: %q from pattern %q", literal, pattern)
			}
			if len(literal) > len(pattern) {
				t.Errorf("Literal longer than original pattern: %q from %q", literal, pattern)
			}
		}
	})
}
