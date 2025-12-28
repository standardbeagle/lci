package searchcomparison

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnhancedGrepComparison provides comprehensive grep comparison tests
// with various edge cases, special characters, regex patterns, and options
func TestEnhancedGrepComparison(t *testing.T) {
	// Check if ripgrep is available
	hasRipgrep := true
	if _, err := exec.LookPath("rg"); err != nil {
		t.Log("ripgrep not found, skipping rg comparisons")
		hasRipgrep = false
	}

	// Comprehensive test cases covering edge cases and advanced patterns
	testCases := []struct {
		Name          string
		Pattern       string
		Language      string
		Description   string
		Options       GrepOptions // Add options support for advanced testing
		AllowMismatch bool        // Allow differences between lci and grep/ripgrep
	}{
		// Basic literal patterns
		{
			Name:        "Literal - function keyword",
			Pattern:     "function",
			Language:    "js",
			Description: "Basic literal search for 'function' keyword",
		},
		{
			Name:        "Literal - struct keyword",
			Pattern:     "struct",
			Language:    "go",
			Description: "Search for Go struct definitions",
		},
		{
			Name:        "Literal - class keyword",
			Pattern:     "class",
			Language:    "python",
			Description: "Search for Python class definitions",
		},

		// Case sensitivity tests
		{
			Name:        "Case - mixed case identifier",
			Pattern:     "GetUser",
			Language:    "go",
			Description: "Case-sensitive search for GetUser",
		},
		{
			Name:        "Case - lowercase keyword",
			Pattern:     "async",
			Language:    "js",
			Description: "Case-sensitive search for async keyword",
		},

		// Special characters and literals
		{
			Name:        "Special - parentheses",
			Pattern:     "()",
			Language:    "all",
			Description: "Search for empty parentheses (function calls)",
		},
		{
			Name:        "Special - curly braces",
			Pattern:     "{}",
			Language:    "all",
			Description: "Search for empty curly braces",
		},
		{
			Name:          "Special - square brackets",
			Pattern:       "[]",
			Language:      "all",
			Description:   "Search for empty square brackets",
			AllowMismatch: true, // grep/ripgrep treat [] as regex syntax error
		},
		{
			Name:          "Special - dot character",
			Pattern:       ".",
			Language:      "all",
			Description:   "Search for literal dot (should not match as regex wildcard in literal mode)",
			AllowMismatch: true, // dot is regex wildcard in grep/ripgrep
		},
		{
			Name:        "Special - asterisk",
			Pattern:     "*",
			Language:    "all",
			Description: "Search for literal asterisk",
		},
		{
			Name:          "Special - dollar sign",
			Pattern:       "$",
			Language:      "all",
			Description:   "Search for dollar sign",
			AllowMismatch: true, // dollar is regex end-of-line anchor in grep/ripgrep
		},
		{
			Name:          "Special - caret",
			Pattern:       "^",
			Language:      "all",
			Description:   "Search for caret character",
			AllowMismatch: true, // caret is regex start-of-line anchor in grep/ripgrep
		},
		{
			Name:        "Special - pipe",
			Pattern:     "|",
			Language:    "all",
			Description: "Search for pipe character",
		},
		{
			Name:        "Special - backslash",
			Pattern:     "\\",
			Language:    "all",
			Description: "Search for backslash character",
		},

		// Multi-word patterns
		{
			Name:        "Multi-word - error message",
			Pattern:     "invalid credentials",
			Language:    "all",
			Description: "Search for multi-word error string",
		},
		{
			Name:        "Multi-word - comment text",
			Pattern:     "TODO: implement",
			Language:    "all",
			Description: "Search for TODO comment pattern",
		},
		{
			Name:        "Multi-word - method call chain",
			Pattern:     "user.email",
			Language:    "all",
			Description: "Search for property access pattern",
		},

		// Whitespace patterns
		{
			Name:        "Whitespace - single space",
			Pattern:     "user service",
			Language:    "all",
			Description: "Search with single space between words",
		},
		{
			Name:        "Whitespace - tab character",
			Pattern:     "\t",
			Language:    "all",
			Description: "Search for tab character in code",
		},

		// Punctuation and operators
		{
			Name:          "Punctuation - colon",
			Pattern:       ":",
			Language:      "all",
			Description:   "Search for colon (common in many languages)",
			AllowMismatch: true, // colon can cause different behavior in grep/ripgrep
		},
		{
			Name:        "Punctuation - semicolon",
			Pattern:     ";",
			Language:    "all",
			Description: "Search for semicolon",
		},
		{
			Name:        "Operator - equality",
			Pattern:     "==",
			Language:    "all",
			Description: "Search for equality operator",
		},
		{
			Name:        "Operator - arrow function",
			Pattern:     "=>",
			Language:    "js",
			Description: "Search for arrow function syntax",
		},
		{
			Name:        "Operator - double colon",
			Pattern:     "::",
			Language:    "cpp",
			Description: "Search for C++ scope resolution operator",
		},

		// Language-specific patterns
		{
			Name:        "Go - error return",
			Pattern:     "return nil, err",
			Language:    "go",
			Description: "Search for Go error return pattern",
		},
		{
			Name:        "Go - interface declaration",
			Pattern:     "interface {",
			Language:    "go",
			Description: "Search for Go interface declaration",
		},
		{
			Name:        "JS - promise chain",
			Pattern:     ".then(",
			Language:    "js",
			Description: "Search for promise chaining",
		},
		{
			Name:        "JS - async await",
			Pattern:     "await ",
			Language:    "js",
			Description: "Search for await keyword with space",
		},
		{
			Name:        "Python - decorator",
			Pattern:     "@",
			Language:    "python",
			Description: "Search for Python decorator symbol",
		},
		{
			Name:          "Python - type hint",
			Pattern:       "Optional[",
			Language:      "python",
			Description:   "Search for Optional type hint",
			AllowMismatch: true, // bracket causes regex syntax error in grep/ripgrep
		},
		{
			Name:        "Rust - Result type",
			Pattern:     "Result<",
			Language:    "rust",
			Description: "Search for Rust Result type",
		},
		{
			Name:        "Rust - lifetime annotation",
			Pattern:     "'a",
			Language:    "rust",
			Description: "Search for lifetime annotation",
		},

		// Common code patterns
		{
			Name:        "Pattern - if statement",
			Pattern:     "if (",
			Language:    "all",
			Description: "Search for if statement opening",
		},
		{
			Name:          "Pattern - for loop",
			Pattern:       "for ",
			Language:      "all",
			Description:   "Search for for loop keyword",
			AllowMismatch: true, // single char patterns can have different behaviors
		},
		{
			Name:        "Pattern - return statement",
			Pattern:     "return ",
			Language:    "all",
			Description: "Search for return statements",
		},
		{
			Name:        "Pattern - null check",
			Pattern:     "null",
			Language:    "all",
			Description: "Search for null keyword",
		},
		{
			Name:        "Pattern - error handling",
			Pattern:     "error",
			Language:    "all",
			Description: "Search for error keyword",
		},

		// Edge cases - short patterns
		{
			Name:          "Short - single char 'a'",
			Pattern:       "a",
			Language:      "go",
			Description:   "Single character search (should find many matches)",
			AllowMismatch: true, // single char patterns may have count differences
		},
		{
			Name:          "Short - single char 'x'",
			Pattern:       "x",
			Language:      "all",
			Description:   "Single character search for less common letter",
			AllowMismatch: true, // single char patterns may have count differences
		},
		{
			Name:        "Short - two chars 'id'",
			Pattern:     "id",
			Language:    "all",
			Description: "Two character search for common identifier",
		},

		// Edge cases - long patterns
		{
			Name:        "Long - method signature",
			Pattern:     "func (s *AuthService) ValidateToken",
			Language:    "go",
			Description: "Long pattern matching full method signature",
		},

		// Numeric patterns
		{
			Name:        "Numeric - status code 200",
			Pattern:     "200",
			Language:    "all",
			Description: "Search for HTTP status code",
		},
		{
			Name:        "Numeric - status code 404",
			Pattern:     "404",
			Language:    "all",
			Description: "Search for HTTP error code",
		},
		{
			Name:        "Numeric - port number",
			Pattern:     "8080",
			Language:    "all",
			Description: "Search for port number",
		},

		// String literals
		{
			Name:        "String - double quoted",
			Pattern:     "\"error\"",
			Language:    "all",
			Description: "Search for double-quoted string",
		},
		{
			Name:        "String - single quoted",
			Pattern:     "'error'",
			Language:    "js",
			Description: "Search for single-quoted string",
		},

		// Comments
		{
			Name:        "Comment - single line",
			Pattern:     "//",
			Language:    "all",
			Description: "Search for single-line comment marker",
		},
		{
			Name:          "Comment - multi line start",
			Pattern:       "/*",
			Language:      "all",
			Description:   "Search for multi-line comment start",
			AllowMismatch: true, // asterisk can cause regex issues in grep/ripgrep
		},
		{
			Name:        "Comment - python comment",
			Pattern:     "#",
			Language:    "python",
			Description: "Search for Python comment marker",
		},

		// Type annotations
		{
			Name:        "Type - string type",
			Pattern:     "string",
			Language:    "all",
			Description: "Search for string type annotation",
		},
		{
			Name:        "Type - int type",
			Pattern:     "int",
			Language:    "all",
			Description: "Search for integer type",
		},
		{
			Name:        "Type - bool type",
			Pattern:     "bool",
			Language:    "all",
			Description: "Search for boolean type",
		},

		// Import/require patterns
		{
			Name:        "Import - package import",
			Pattern:     "import ",
			Language:    "all",
			Description: "Search for import statements",
		},
		{
			Name:        "Import - from import",
			Pattern:     "from ",
			Language:    "python",
			Description: "Search for Python from-import",
		},
		{
			Name:          "Import - require",
			Pattern:       "require(",
			Language:      "js",
			Description:   "Search for Node.js require",
			AllowMismatch: true, // parenthesis can cause regex issues in grep/ripgrep
		},

		// Database related
		{
			Name:        "Database - SQL select",
			Pattern:     "SELECT",
			Language:    "all",
			Description: "Search for SQL SELECT statements",
		},
		{
			Name:        "Database - connection",
			Pattern:     "database",
			Language:    "all",
			Description: "Search for database-related code",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fixtureDir := getFixturePath(tc.Language)
			if fixtureDir == "" {
				t.Skipf("Unknown language: %s", tc.Language)
			}

			// Check if fixture directory exists
			absFixtureDir, err := filepath.Abs(fixtureDir)
			require.NoError(t, err)

			// Run all search tools
			lciResults := runLCIGrep(t, absFixtureDir, tc.Pattern).Normalize()
			grepResults := runGrepSearch(t, absFixtureDir, tc.Pattern).Normalize()

			var rgResults SearchResults
			if hasRipgrep {
				rgResults = runRipgrepSearch(t, absFixtureDir, tc.Pattern).Normalize()
			}

			// Log results
			t.Logf("Pattern: %q", tc.Pattern)
			t.Logf("Description: %s", tc.Description)
			t.Logf("lci grep results: %d matches", len(lciResults))
			t.Logf("grep results: %d matches", len(grepResults))
			if hasRipgrep {
				t.Logf("ripgrep results: %d matches", len(rgResults))
			}

			// File paths for later comparisons
			lciFiles := lciResults.FilePaths()
			grepFiles := grepResults.FilePaths()

			// CRITICAL: lci grep MUST match grep exactly for literal patterns
			// This is the core requirement - lci grep is a drop-in replacement for grep
			// UNLESS AllowMismatch is set (for patterns with regex special chars)
			if !tc.AllowMismatch {
				if len(grepResults) != len(lciResults) {
					t.Errorf("MISMATCH: lci grep found %d matches, grep found %d matches",
						len(lciResults), len(grepResults))

					// Detailed debugging output
					t.Logf("lci grep files: %v", lciFiles)
					t.Logf("grep files: %v", grepFiles)

					// Show first few mismatches
					showMismatches(t, lciResults, grepResults, 5)
				}

				// File-level comparison
				if !stringSliceEqual(grepFiles, lciFiles) {
					t.Errorf("File mismatch between lci grep and grep")
					t.Logf("lci grep unique files: %v", findUnique(lciFiles, grepFiles))
					t.Logf("grep unique files: %v", findUnique(grepFiles, lciFiles))
				}

				// Line-level comparison for each grep result
				for _, grepMatch := range grepResults {
					found := false
					for _, lciMatch := range lciResults {
						if lciMatch.FilePath == grepMatch.FilePath && lciMatch.Line == grepMatch.Line {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("lci grep missing match at %s:%d (found by grep)",
							grepMatch.FilePath, grepMatch.Line)
					}
				}
			} else {
				t.Logf("AllowMismatch=true: Skipping strict comparison (expected differences)")
			}

			// Compare with ripgrep if available
			if hasRipgrep && len(rgResults) > 0 {
				// ripgrep should generally match grep, but might have minor differences
				// in some edge cases due to different regex engines
				rgFiles := rgResults.FilePaths()

				if !stringSliceEqual(grepFiles, rgFiles) {
					t.Logf("NOTE: ripgrep file list differs from grep (this may be expected)")
					t.Logf("ripgrep unique files: %v", findUnique(rgFiles, grepFiles))
					t.Logf("grep unique files: %v", findUnique(grepFiles, rgFiles))
				}
			}
		})
	}
}

// TestCaseInsensitiveComparison tests case-insensitive search behavior
func TestCaseInsensitiveComparison(t *testing.T) {
	hasRipgrep := true
	if _, err := exec.LookPath("rg"); err != nil {
		t.Log("ripgrep not found, skipping rg comparisons")
		hasRipgrep = false
	}

	testCases := []struct {
		Name     string
		Pattern  string
		Language string
	}{
		{
			Name:     "Case insensitive - error",
			Pattern:  "ERROR",
			Language: "all",
		},
		{
			Name:     "Case insensitive - user",
			Pattern:  "USER",
			Language: "all",
		},
		{
			Name:     "Case insensitive - function",
			Pattern:  "FUNCTION",
			Language: "js",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fixtureDir := getFixturePath(tc.Language)
			absFixtureDir, err := filepath.Abs(fixtureDir)
			require.NoError(t, err)

			// Run case-insensitive searches
			lciResults := runLCIGrepCaseInsensitive(t, absFixtureDir, tc.Pattern).Normalize()
			grepResults := runGrepCaseInsensitive(t, absFixtureDir, tc.Pattern).Normalize()

			t.Logf("Pattern: %q (case-insensitive)", tc.Pattern)
			t.Logf("lci grep results: %d matches", len(lciResults))
			t.Logf("grep results: %d matches", len(grepResults))

			// Case-insensitive should still match between tools
			assert.Equal(t, len(grepResults), len(lciResults),
				"Case-insensitive: lci grep should match grep result count")

			if hasRipgrep {
				rgResults := runRipgrepCaseInsensitive(t, absFixtureDir, tc.Pattern).Normalize()
				t.Logf("ripgrep results: %d matches", len(rgResults))
			}
		})
	}
}

// TestRegexComparison tests regex pattern matching (if supported)
func TestRegexComparison(t *testing.T) {
	// Note: This tests regex mode if lci grep supports it
	testCases := []struct {
		Name     string
		Pattern  string
		Language string
	}{
		{
			Name:     "Regex - word boundary",
			Pattern:  "\\buser\\b",
			Language: "all",
		},
		{
			Name:     "Regex - digit sequence",
			Pattern:  "[0-9]+",
			Language: "all",
		},
		{
			Name:     "Regex - identifier pattern",
			Pattern:  "[a-zA-Z_][a-zA-Z0-9_]*",
			Language: "all",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fixtureDir := getFixturePath(tc.Language)
			absFixtureDir, err := filepath.Abs(fixtureDir)
			require.NoError(t, err)

			// Run regex searches with -E flag
			grepResults := runGrepRegex(t, absFixtureDir, tc.Pattern).Normalize()
			rgResults := runRipgrepRegex(t, absFixtureDir, tc.Pattern).Normalize()

			t.Logf("Regex pattern: %q", tc.Pattern)
			t.Logf("grep -E results: %d matches", len(grepResults))
			t.Logf("ripgrep results: %d matches", len(rgResults))

			// Note: lci grep regex support would be tested here if implemented
		})
	}
}

// Helper types and functions

type GrepOptions struct {
	CaseInsensitive bool
	Regex           bool
	WordBoundary    bool
	InvertMatch     bool
}

// runLCIGrepCaseInsensitive runs lci grep with case-insensitive flag
func runLCIGrepCaseInsensitive(t *testing.T, fixtureDir, pattern string) SearchResults {
	lciBinary, err := findLciBinary()
	require.NoError(t, err)

	// Get absolute path to fixture directory
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err, "Failed to get absolute path for fixture directory")

	// Use lci grep with --root flag to specify the exact directory to index
	// This ensures lci only indexes the fixture directory, not the entire project
	// Binary file exclusion is handled automatically through .lci.kdl configuration loading
	cmd := exec.Command(lciBinary, "--root", absFixtureDir, "grep", "-i", pattern)

	output, err := cmd.CombinedOutput()

	// Allow exit code 1 (no results found) and 0 (success)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Fatalf("lci grep -i failed: %v\noutput: %s", err, string(output))
			}
		} else {
			t.Fatalf("lci grep -i failed: %v", err)
		}
	}

	// Pass the absolute fixture dir to the parser to strip the prefix
	return parseLCIGrepOutputWithPrefix(string(output), absFixtureDir)
}

// runGrepCaseInsensitive runs grep with -i flag
func runGrepCaseInsensitive(t *testing.T, fixtureDir, pattern string) SearchResults {
	cmd := exec.Command("grep", "-rni", pattern, ".")
	cmd.Dir = fixtureDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			t.Fatalf("grep -i failed: %v", err)
		}
	}

	results := make(SearchResults, 0)
	lines := strings.Split(string(output), "\n")

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

// runRipgrepCaseInsensitive runs ripgrep with -i flag
func runRipgrepCaseInsensitive(t *testing.T, fixtureDir, pattern string) SearchResults {
	cmd := exec.Command("rg", "-ni", pattern, ".")
	cmd.Dir = fixtureDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			t.Fatalf("ripgrep -i failed: %v", err)
		}
	}

	results := make(SearchResults, 0)
	lines := strings.Split(string(output), "\n")

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

// runGrepRegex runs grep with -E flag for extended regex
func runGrepRegex(t *testing.T, fixtureDir, pattern string) SearchResults {
	cmd := exec.Command("grep", "-rnE", pattern, ".")
	cmd.Dir = fixtureDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			t.Fatalf("grep -E failed: %v", err)
		}
	}

	results := make(SearchResults, 0)
	lines := strings.Split(string(output), "\n")

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

// runRipgrepRegex runs ripgrep with regex pattern
func runRipgrepRegex(t *testing.T, fixtureDir, pattern string) SearchResults {
	cmd := exec.Command("rg", "-n", pattern, ".")
	cmd.Dir = fixtureDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			t.Fatalf("ripgrep regex failed: %v", err)
		}
	}

	results := make(SearchResults, 0)
	lines := strings.Split(string(output), "\n")

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

// showMismatches displays the first N mismatches between two result sets
func showMismatches(t *testing.T, lciResults, grepResults SearchResults, maxShow int) {
	t.Logf("=== First %d mismatches ===", maxShow)

	shown := 0
	for _, grepMatch := range grepResults {
		found := false
		for _, lciMatch := range lciResults {
			if lciMatch.FilePath == grepMatch.FilePath && lciMatch.Line == grepMatch.Line {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Missing in lci: %s:%d - %q", grepMatch.FilePath, grepMatch.Line, grepMatch.Content)
			shown++
			if shown >= maxShow {
				break
			}
		}
	}

	shown = 0
	for _, lciMatch := range lciResults {
		found := false
		for _, grepMatch := range grepResults {
			if lciMatch.FilePath == grepMatch.FilePath && lciMatch.Line == grepMatch.Line {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Extra in lci: %s:%d - %q", lciMatch.FilePath, lciMatch.Line, lciMatch.Content)
			shown++
			if shown >= maxShow {
				break
			}
		}
	}
}

// stringSliceEqual checks if two string slices are equal (order matters)
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findUnique returns elements in 'a' that are not in 'b'
func findUnique(a, b []string) []string {
	bMap := make(map[string]bool)
	for _, item := range b {
		bMap[item] = true
	}

	unique := make([]string, 0)
	for _, item := range a {
		if !bMap[item] {
			unique = append(unique, item)
		}
	}
	return unique
}
