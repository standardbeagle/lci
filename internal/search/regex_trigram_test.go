package search_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/regex_analyzer"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// Simple contains check for slices
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TestRegexTrigramExtraction tests if regex patterns are properly using trigram index
func TestRegexTrigramExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping regex trigram test in short mode")
	}

	tempDir := t.TempDir()

	// Create test file with the pattern
	testContent := `package test

// Function1 is a test function
func Function1(param1 string, param2 int) error {
	return nil
}

// Function2 is another test function
func Function2(param1 string) error {
	return nil
}

// DifferentFunction is different
func DifferentFunction() error {
	return nil
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create indexer and index
	cfg := createTestConfig(tempDir)
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Create regex analyzer to test literal extraction
	analyzer := regex_analyzer.NewHybridRegexEngine(100, 100, indexer)

	// Test pattern 1: Function[0-9]+ should extract "Function"
	pattern1 := "Function[0-9]+"
	literals1 := analyzer.ExtractLiterals(pattern1)
	t.Logf("Pattern '%s' extracts literals: %v", pattern1, literals1)

	if len(literals1) == 0 {
		t.Errorf("Expected pattern '%s' to extract at least one literal", pattern1)
	}

	// Should extract "Function" for trigram index
	hasFunctionLiteral := false
	for _, lit := range literals1 {
		if lit == "Function" {
			hasFunctionLiteral = true
			t.Logf("✅ Found literal 'Function' for trigram index")
		}
	}
	if !hasFunctionLiteral {
		t.Errorf("Expected pattern '%s' to extract literal 'Function'", pattern1)
	}

	// Test pattern 2: [0-9]+ should not extract anything (too generic)
	pattern2 := "[0-9]+"
	literals2 := analyzer.ExtractLiterals(pattern2)
	t.Logf("Pattern '%s' extracts literals: %v", pattern2, literals2)

	if len(literals2) > 0 {
		t.Errorf("Expected pattern '%s' to extract no literals (no alphanumeric sequences)", pattern2)
	}

	// Test pattern 3: DifferentFunction should extract "DifferentFunction"
	pattern3 := "DifferentFunction"
	literals3 := analyzer.ExtractLiterals(pattern3)
	t.Logf("Pattern '%s' extracts literals: %v", pattern3, literals3)

	hasDifferentFunctionLiteral := false
	for _, lit := range literals3 {
		if lit == "DifferentFunction" {
			hasDifferentFunctionLiteral = true
			t.Logf("✅ Found literal 'DifferentFunction' for trigram index")
		}
	}
	if !hasDifferentFunctionLiteral {
		t.Errorf("Expected pattern '%s' to extract literal 'DifferentFunction'", pattern3)
	}

	// Test actual regex search with trigram filtering
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Test regex search performance
	t.Run("RegexSearch_TrigramOptimized", func(t *testing.T) {
		pattern := "Function[0-9]+"
		options := types.SearchOptions{
			UseRegex:   true,
			MaxResults: 100,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Regex search '%s': %d results in %v", pattern, len(results), duration)

		// Should find Function1 and Function2 in both comments and definitions
		expectedMatches := 4
		if len(results) != expectedMatches {
			t.Errorf("Expected %d results, got %d", expectedMatches, len(results))
		}

		// Verify we got the right matches
		matchTexts := make([]string, len(results))
		for i, result := range results {
			matchTexts[i] = result.Match
		}

		// Check that we found matches containing Function1 and Function2
		foundFunction1 := false
		foundFunction2 := false
		for _, match := range matchTexts {
			if strings.Contains(match, "Function1") {
				foundFunction1 = true
			}
			if strings.Contains(match, "Function2") {
				foundFunction2 = true
			}
		}
		if !foundFunction1 {
			t.Errorf("Expected to find 'Function1' in results: %v", matchTexts)
		}
		if !foundFunction2 {
			t.Errorf("Expected to find 'Function2' in results: %v", matchTexts)
		}

		// Performance check: should be fast if using trigram index
		if duration > 100*time.Millisecond {
			t.Logf("WARNING: Regex search took %v (might not be using trigram index effectively)", duration)
		} else {
			t.Logf("✅ Regex search completed in %v (good performance)", duration)
		}
	})

	// Test that regex without extractable literals is slower
	t.Run("RegexSearch_NoLiterals", func(t *testing.T) {
		pattern := "[0-9]+" // This should not extract any literals
		options := types.SearchOptions{
			UseRegex:   true,
			MaxResults: 100,
		}

		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, options)
		duration := time.Since(start)

		t.Logf("Regex search '%s': %d results in %v", pattern, len(results), duration)

		// Should still find matches but might be slower
		if len(results) == 0 {
			t.Errorf("Expected to find results for pattern '%s'", pattern)
		}
	})
}

// TestRegexLiteralExtractionDirectly tests the literal extraction directly
func TestRegexLiteralExtractionDirectly(t *testing.T) {
	extractor := regex_analyzer.NewLiteralExtractor()

	testCases := []struct {
		pattern      string
		expectedLits []string
		description  string
	}{
		{
			pattern:      "Function[0-9]+",
			expectedLits: []string{"Function"},
			description:  "Regex with character class should extract literal prefix",
		},
		{
			pattern:      "test.*[0-9]+",
			expectedLits: []string{"test"},
			description:  "Regex with wildcard should extract literal part",
		},
		{
			pattern:      "(Function|Method)[0-9]+",
			expectedLits: []string{"Function", "Method"},
			description:  "Alternation should extract both alternatives",
		},
		{
			pattern:      "[0-9]+",
			expectedLits: []string{},
			description:  "Only character class should extract no literals",
		},
		{
			pattern:      "a.*b.*c",
			expectedLits: []string{},
			description:  "Single character literals too short for trigrams",
		},
		{
			pattern:      "abc.*def",
			expectedLits: []string{"abc", "def"},
			description:  "Multiple 3+ char literals should be extracted",
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
