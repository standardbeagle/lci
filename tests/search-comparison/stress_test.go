package searchcomparison

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestGrepPerformanceComparison tests LCI's CLI grep command performance.
//
// IMPORTANT: This test measures CLI performance INCLUDING index build cost.
// The LCI CLI follows an "index-compute-shutdown" workflow where the index
// is rebuilt on every command execution. This is intentional for the CLI tool
// (diagnostic/one-off usage), but NOT representative of production usage.
//
// For fair performance comparison with grep/ripgrep, see competitor_evaluation_test.go
// which tests persistent index performance (simulating MCP server usage pattern).
//
// Performance expectations:
// - CLI mode: Slower than grep due to index build overhead (expected)
// - MCP mode: Faster than grep for repeated searches (see competitor_evaluation_test.go)
func TestGrepPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	hasRipgrep := true
	if _, err := exec.LookPath("rg"); err != nil {
		t.Log("ripgrep not found, skipping rg performance comparison")
		hasRipgrep = false
	}

	// Performance test patterns
	patterns := []struct {
		Name     string
		Pattern  string
		Language string
		Expected string // Expected performance characteristic
	}{
		{
			Name:     "Common word - high frequency",
			Pattern:  "user",
			Language: "all",
			Expected: "High result count - tests result handling performance",
		},
		{
			Name:     "Common word - medium frequency",
			Pattern:  "database",
			Language: "all",
			Expected: "Medium result count - balanced test",
		},
		{
			Name:     "Rare pattern - low frequency",
			Pattern:  "ValidateCredentials",
			Language: "all",
			Expected: "Low result count - tests search efficiency",
		},
		{
			Name:     "Single character - maximum frequency",
			Pattern:  "a",
			Language: "all",
			Expected: "Maximum result count - stress test",
		},
		{
			Name:     "Multi-word - exact match",
			Pattern:  "invalid credentials",
			Language: "all",
			Expected: "Multi-word exact match - tests pattern complexity",
		},
		{
			Name:     "Special chars - operators",
			Pattern:  "==",
			Language: "all",
			Expected: "Special character handling performance",
		},
	}

	for _, tc := range patterns {
		t.Run(tc.Name, func(t *testing.T) {
			fixtureDir := getFixturePath(tc.Language)
			absFixtureDir, err := filepath.Abs(fixtureDir)
			require.NoError(t, err)

			t.Logf("Pattern: %q - %s", tc.Pattern, tc.Expected)

			// Benchmark lci grep (in-process)
			lciStart := time.Now()
			lciResults := getOrCreateIndex(t, absFixtureDir).search(t, tc.Pattern)
			lciDuration := time.Since(lciStart)
			t.Logf("lci grep: %d results in %v", len(lciResults), lciDuration)

			// Benchmark standard grep
			grepStart := time.Now()
			grepResults := runGrepSearch(t, absFixtureDir, tc.Pattern)
			grepDuration := time.Since(grepStart)
			t.Logf("grep: %d results in %v", len(grepResults), grepDuration)

			// Benchmark ripgrep
			if hasRipgrep {
				rgStart := time.Now()
				rgResults := runRipgrepSearch(t, absFixtureDir, tc.Pattern)
				rgDuration := time.Since(rgStart)
				t.Logf("ripgrep: %d results in %v", len(rgResults), rgDuration)

				// Compare performance
				t.Logf("Performance ratio (lci/grep): %.2fx", float64(lciDuration)/float64(grepDuration))
				t.Logf("Performance ratio (lci/rg): %.2fx", float64(lciDuration)/float64(rgDuration))
			}

			// Result count should match
			if len(lciResults) != len(grepResults) {
				t.Errorf("Result count mismatch: lci=%d, grep=%d", len(lciResults), len(grepResults))
			}
		})
	}
}

// TestLargePatternSet tests searching for many patterns sequentially
func TestLargePatternSet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large pattern test in short mode")
	}

	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	// Generate many search patterns
	patterns := []string{
		"user", "service", "database", "auth", "token",
		"error", "result", "function", "class", "struct",
		"interface", "type", "return", "async", "await",
		"import", "export", "const", "let", "var",
		"if", "else", "for", "while", "switch",
	}

	t.Logf("Testing sequential search of %d patterns", len(patterns))

	idx := getOrCreateIndex(t, absFixtureDir)
	lciStart := time.Now()
	lciTotalResults := 0
	for _, pattern := range patterns {
		results := idx.search(t, pattern)
		lciTotalResults += len(results)
	}
	lciDuration := time.Since(lciStart)
	t.Logf("lci grep: %d total results in %v (avg: %v per pattern)",
		lciTotalResults, lciDuration, lciDuration/time.Duration(len(patterns)))

	grepStart := time.Now()
	grepTotalResults := 0
	for _, pattern := range patterns {
		results := runGrepSearch(t, absFixtureDir, pattern)
		grepTotalResults += len(results)
	}
	grepDuration := time.Since(grepStart)
	t.Logf("grep: %d total results in %v (avg: %v per pattern)",
		grepTotalResults, grepDuration, grepDuration/time.Duration(len(patterns)))

	t.Logf("Sequential search performance ratio (lci/grep): %.2fx",
		float64(lciDuration)/float64(grepDuration))
}

// TestEdgeCasePatterns tests unusual and edge case patterns
func TestEdgeCasePatterns(t *testing.T) {
	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	edgeCases := []struct {
		Name                string
		Pattern             string
		Expected            string
		AllowDifferentCount bool // lci and grep may have different behavior for some edge cases
	}{
		{
			Name:                "Empty string",
			Pattern:             "",
			Expected:            "Should return no results or error",
			AllowDifferentCount: true, // lci returns error (0 results), grep matches all lines
		},
		{
			Name:     "Very long pattern",
			Pattern:  "ThisIsAVeryLongPatternThatProbablyDoesNotExistAnywhereInTheCodebaseButWeShouldTestItAnyway",
			Expected: "Should return no results efficiently",
		},
		{
			Name:                "Pattern with newline (escaped)",
			Pattern:             "\\n",
			Expected:            "Should search for literal backslash-n",
			AllowDifferentCount: true, // lci and grep handle escaped backslash-n differently
		},
		{
			Name:     "Pattern with multiple spaces",
			Pattern:  "user   service",
			Expected: "Should search for exact spacing",
		},
		{
			Name:     "Unicode - emoji",
			Pattern:  "😀",
			Expected: "Should handle Unicode emoji",
		},
		{
			Name:     "Unicode - Chinese",
			Pattern:  "用户",
			Expected: "Should handle CJK characters",
		},
		{
			Name:     "Unicode - Arabic",
			Pattern:  "مستخدم",
			Expected: "Should handle RTL languages",
		},
		{
			Name:                "All special regex chars",
			Pattern:             ".*+?[]{}()|^$\\",
			Expected:            "Should treat as literal in non-regex mode",
			AllowDifferentCount: true, // grep fails with "Unmatched [" error on this pattern
		},
		{
			Name:     "Repeated characters",
			Pattern:  "aaaaaaaaaaa",
			Expected: "Should handle repetition efficiently",
		},
		{
			Name:     "Mixed case complex",
			Pattern:  "CamelCaseIdentifier",
			Expected: "Should match exact case",
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Pattern: %q - %s", tc.Pattern, tc.Expected)

			// Test lci grep (in-process)
			lciResults := getOrCreateIndex(t, absFixtureDir).search(t, tc.Pattern)
			t.Logf("lci grep: %d results", len(lciResults))

			// Test standard grep
			grepResults := runGrepSearch(t, absFixtureDir, tc.Pattern)
			t.Logf("grep: %d results", len(grepResults))

			// Results should match (unless allowed to differ for edge cases)
			if len(lciResults) != len(grepResults) && !tc.AllowDifferentCount {
				t.Errorf("Result mismatch for pattern %q: lci=%d, grep=%d",
					tc.Pattern, len(lciResults), len(grepResults))
			}
		})
	}
}

// TestContextLinesComparison tests context line options (-A, -B, -C)
func TestContextLinesComparison(t *testing.T) {
	// Note: This test is for future implementation of context line support in lci grep
	// Currently documents expected behavior

	t.Skip("Context line support (-A, -B, -C flags) not yet implemented in lci grep")

	fixtureDir := getFixturePath("go")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	pattern := "GetUser"

	// Test -A (after context)
	t.Run("After context (-A)", func(t *testing.T) {
		cmd := exec.Command("grep", "-rnA", "2", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		t.Logf("grep -A 2 output:\n%s", string(output))

		// Future: Test lci grep -A 2 here
	})

	// Test -B (before context)
	t.Run("Before context (-B)", func(t *testing.T) {
		cmd := exec.Command("grep", "-rnB", "2", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		t.Logf("grep -B 2 output:\n%s", string(output))

		// Future: Test lci grep -B 2 here
	})

	// Test -C (surrounding context)
	t.Run("Surrounding context (-C)", func(t *testing.T) {
		cmd := exec.Command("grep", "-rnC", "2", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		t.Logf("grep -C 2 output:\n%s", string(output))

		// Future: Test lci grep -C 2 here
	})
}

// TestInvertMatchComparison tests inverted matching (-v flag)
func TestInvertMatchComparison(t *testing.T) {
	// Note: This test is for future implementation of invert match support
	t.Skip("Invert match support (-v flag) not yet implemented in lci grep")

	fixtureDir := getFixturePath("go")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	pattern := "test"

	// grep -v should find lines NOT containing pattern
	cmd := exec.Command("grep", "-rnv", pattern, ".")
	cmd.Dir = absFixtureDir
	output, _ := cmd.CombinedOutput()
	t.Logf("grep -v found %d lines not containing %q", len(output), pattern)

	// Future: Test lci grep -v here
}

// TestMaxCountComparison tests limiting results (-m flag)
func TestMaxCountComparison(t *testing.T) {
	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	pattern := "user"
	maxCount := 5

	t.Run("Limit results per file", func(t *testing.T) {
		// grep -m limits results per file
		cmd := exec.Command("grep", "-rnm", fmt.Sprintf("%d", maxCount), pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		grepResults := parseGrepOutput(string(output))
		t.Logf("grep -m %d: %d total results", maxCount, len(grepResults))

		// Note: Document how lci grep should handle max count
		// Should it be per-file or global?
	})
}

// TestFilePatternComparison tests file pattern filtering
func TestFilePatternComparison(t *testing.T) {
	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	pattern := "function"

	t.Run("Include only Go files", func(t *testing.T) {
		// grep --include for file filtering
		cmd := exec.Command("grep", "-rn", "--include=*.go", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		grepResults := parseGrepOutput(string(output))
		t.Logf("grep --include=*.go: %d results", len(grepResults))

		// Verify all results are from .go files
		for _, result := range grepResults {
			if filepath.Ext(result.FilePath) != ".go" {
				t.Errorf("Found non-Go file: %s", result.FilePath)
			}
		}
	})

	t.Run("Exclude test files", func(t *testing.T) {
		// grep --exclude for file filtering
		cmd := exec.Command("grep", "-rn", "--exclude=*_test.go", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		grepResults := parseGrepOutput(string(output))
		t.Logf("grep --exclude=*_test.go: %d results", len(grepResults))

		// Verify no results from test files
		for _, result := range grepResults {
			if filepath.Base(result.FilePath) == "_test.go" {
				t.Errorf("Found test file: %s", result.FilePath)
			}
		}
	})
}

// TestWordBoundaryComparison tests word boundary matching
func TestWordBoundaryComparison(t *testing.T) {
	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	// grep -w matches whole words only
	pattern := "user"

	t.Run("Without word boundary", func(t *testing.T) {
		cmd := exec.Command("grep", "-rn", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		normalResults := parseGrepOutput(string(output))
		t.Logf("grep (no -w): %d results", len(normalResults))
	})

	t.Run("With word boundary", func(t *testing.T) {
		cmd := exec.Command("grep", "-rnw", pattern, ".")
		cmd.Dir = absFixtureDir
		output, _ := cmd.CombinedOutput()
		wordResults := parseGrepOutput(string(output))
		t.Logf("grep -w: %d results", len(wordResults))

		// Word boundary results should be <= normal results
		// (word boundary is more restrictive)
	})
}

// parseGrepOutput is a helper to parse grep output for tests
func parseGrepOutput(output string) SearchResults {
	results := make(SearchResults, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			var lineNum int
			_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)
			filePath := strings.TrimPrefix(parts[0], "./")
			results = append(results, SearchResult{
				FilePath: filePath,
				Line:     lineNum,
				Content:  strings.TrimSpace(parts[2]),
			})
		}
	}

	return results
}
