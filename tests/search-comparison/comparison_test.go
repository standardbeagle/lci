package searchcomparison

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SearchResult represents a search result from any tool
type SearchResult struct {
	FilePath string
	Line     int
	Content  string
}

// SearchResults is a slice of SearchResult with helper methods
type SearchResults []SearchResult

// Normalize sorts and deduplicates results for comparison
func (r SearchResults) Normalize() SearchResults {
	// Sort by file path, then line number
	sort.Slice(r, func(i, j int) bool {
		if r[i].FilePath != r[j].FilePath {
			return r[i].FilePath < r[j].FilePath
		}
		return r[i].Line < r[j].Line
	})

	// Deduplicate
	if len(r) == 0 {
		return r
	}

	unique := make(SearchResults, 0, len(r))
	unique = append(unique, r[0])
	for i := 1; i < len(r); i++ {
		last := unique[len(unique)-1]
		if r[i].FilePath != last.FilePath || r[i].Line != last.Line {
			unique = append(unique, r[i])
		}
	}

	return unique
}

// FilePaths returns unique file paths from results
func (r SearchResults) FilePaths() []string {
	pathMap := make(map[string]bool)
	for _, result := range r {
		pathMap[result.FilePath] = true
	}

	paths := make([]string, 0, len(pathMap))
	for path := range pathMap {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// TestCase represents a search comparison test case
type TestCase struct {
	Name        string
	Pattern     string
	Description string
	Language    string // "go", "js", "python", "rust", "cpp", "java", "all"
}

// getFixturePath returns the path to the fixture directory for a language
func getFixturePath(language string) string {
	if language == "all" {
		return "fixtures"
	}

	languageMap := map[string]string{
		"go":     "go-sample",
		"js":     "js-sample",
		"python": "python-sample",
		"rust":   "rust-sample",
		"cpp":    "cpp-sample",
		"java":   "java-sample",
	}

	if dir, ok := languageMap[language]; ok {
		return filepath.Join("fixtures", dir)
	}
	return ""
}

// findLciBinary finds the lci binary in the project root
func findLciBinary() (string, error) {
	// Try to find lci binary relative to test directory
	candidates := []string{
		"../../lci",    // From tests/search-comparison
		"./lci",        // From project root
		"../../../lci", // If running from deeper nesting
	}

	for _, candidate := range candidates {
		if absPath, err := filepath.Abs(candidate); err == nil {
			if info, err := os.Stat(absPath); err == nil {
				// Ensure it's a regular file, not a directory
				if info.Mode().IsRegular() {
					return absPath, nil
				}
			}
		}
	}

	// Try looking in PATH
	return exec.LookPath("lci")
}

// runLCIGrep executes lci grep (pure trigram text search - should match grep exactly)
func runLCIGrep(t *testing.T, fixtureDir, pattern string) SearchResults {
	// Find lci binary
	lciBinary, err := findLciBinary()
	require.NoError(t, err, "Failed to find lci binary. Run 'make build' first")

	// Get absolute path to fixture directory
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err, "Failed to get absolute path for fixture directory")

	// Use lci grep with --root flag to specify the exact directory to index
	// This ensures lci only indexes the fixture directory, not the entire project
	// Binary file exclusion is handled automatically through .lci.kdl configuration loading
	cmd := exec.Command(lciBinary, "--root", absFixtureDir, "grep", pattern)

	output, err := cmd.CombinedOutput()

	// Allow exit code 1 (no results found) and 0 (success)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Fatalf("lci grep failed: %v\noutput: %s", err, string(output))
			}
		} else {
			t.Fatalf("lci grep failed: %v", err)
		}
	}

	// Pass the absolute fixture dir to the parser to strip the prefix
	return parseLCIGrepOutputWithPrefix(string(output), absFixtureDir)
}

// parseLCIGrepOutputWithPrefix is like parseLCIGrepOutput but strips the prefix path
func parseLCIGrepOutputWithPrefix(output string, prefixPath string) SearchResults {
	// Normalize the prefix path with trailing slash
	if !strings.HasSuffix(prefixPath, "/") {
		prefixPath = prefixPath + "/"
	}

	results := make(SearchResults, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Handle validation errors gracefully (e.g., empty pattern) - return empty results
		if strings.HasPrefix(line, "[INDEX-ERROR]") {
			// Check if it's a validation error (pattern validation)
			if strings.Contains(line, "search pattern cannot be empty") ||
				strings.Contains(line, "validation error") {
				return results // Return empty results for validation errors
			}
			// Other INDEX errors should still panic
			panic(fmt.Sprintf("INDEX error in output: %s", line))
		}
		// Fail on INDEX warnings - these should not occur in successful searches
		if strings.HasPrefix(line, "[INDEX-WARN]") {
			panic(fmt.Sprintf("INDEX warning in output: %s", line))
		}

		// Skip info/debug lines and empty lines
		if strings.HasPrefix(line, "[INDEX-INFO]") ||
			strings.HasPrefix(line, "2025/") ||
			strings.HasPrefix(line, "Indexed ") ||
			strings.HasPrefix(line, "Found ") ||
			strings.TrimSpace(line) == "" {
			continue
		}

		// Parse format: "filename:line:column:content"
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 3 {
			var lineNum int
			_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)

			content := ""
			if len(parts) >= 4 {
				content = strings.TrimSpace(parts[3])
			}

			filePath := strings.TrimSpace(parts[0])

			// Strip the prefix path if it's present
			if strings.HasPrefix(filePath, prefixPath) {
				filePath = strings.TrimPrefix(filePath, prefixPath)
			}

			results = append(results, SearchResult{
				FilePath: filePath,
				Line:     lineNum,
				Content:  content,
			})
		}
	}

	return results
}

// parseLCIGrepOutput parses lci grep output format: filename:line:column:content
func parseLCIGrepOutput(output string) SearchResults {
	// Detect if we're using absolute paths by checking for the fixture path prefix
	fixturePrefix := ""
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "/tests/search-comparison/fixtures/") {
			// Extract the prefix up to and including fixtures/
			idx := strings.Index(line, "/tests/search-comparison/fixtures/")
			if idx >= 0 {
				fixturePrefix = line[:idx+len("/tests/search-comparison/fixtures/")]
				break
			}
		}
	}

	results := make(SearchResults, 0)

	for _, line := range lines {
		// Handle validation errors gracefully (e.g., empty pattern) - return empty results
		if strings.HasPrefix(line, "[INDEX-ERROR]") {
			// Check if it's a validation error (pattern validation)
			if strings.Contains(line, "search pattern cannot be empty") ||
				strings.Contains(line, "validation error") {
				return results // Return empty results for validation errors
			}
			// Other INDEX errors should still panic
			panic(fmt.Sprintf("INDEX error in output: %s", line))
		}
		// Fail on INDEX warnings - these should not occur in successful searches
		if strings.HasPrefix(line, "[INDEX-WARN]") {
			panic(fmt.Sprintf("INDEX warning in output: %s", line))
		}

		// Skip info/debug lines and empty lines
		if strings.HasPrefix(line, "[INDEX-INFO]") ||
			strings.HasPrefix(line, "2025/") ||
			strings.HasPrefix(line, "Indexed ") ||
			strings.HasPrefix(line, "Found ") ||
			strings.TrimSpace(line) == "" {
			continue
		}

		// Parse format: "filename:line:column:content"
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 3 {
			var lineNum int
			_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)

			content := ""
			if len(parts) >= 4 {
				content = strings.TrimSpace(parts[3])
			}

			filePath := strings.TrimSpace(parts[0])

			// If we detected an absolute path prefix, strip it to get relative path
			if fixturePrefix != "" && strings.HasPrefix(filePath, fixturePrefix) {
				filePath = strings.TrimPrefix(filePath, fixturePrefix)
			}

			results = append(results, SearchResult{
				FilePath: filePath,
				Line:     lineNum,
				Content:  content,
			})
		}
	}

	return results
}

// runGrepSearch executes grep search
func runGrepSearch(t *testing.T, fixtureDir, pattern string) SearchResults {
	cmd := exec.Command("grep", "-rn", pattern, ".")
	cmd.Dir = fixtureDir

	output, err := cmd.CombinedOutput()
	// grep returns exit code 1 if no matches found, which is okay
	// grep returns exit code 2 for syntax errors - log and return empty results
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				// Syntax error in pattern - log and return empty results
				t.Logf("grep syntax error (exit 2): %s", string(output))
				return make(SearchResults, 0)
			} else if exitErr.ExitCode() != 1 {
				t.Fatalf("grep failed: %v, output: %s", err, string(output))
			}
		}
	}

	results := make(SearchResults, 0)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse format: "./filepath:line: content"
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

// runRipgrepSearch executes ripgrep search
func runRipgrepSearch(t *testing.T, fixtureDir, pattern string) SearchResults {
	cmd := exec.Command("rg", "-n", pattern, ".")
	cmd.Dir = fixtureDir

	output, err := cmd.CombinedOutput()
	// rg returns exit code 1 if no matches found, which is okay
	// rg returns exit code 2 for regex syntax errors - log and return empty results
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				// Regex syntax error in pattern - log and return empty results
				t.Logf("ripgrep regex syntax error (exit 2): %s", string(output))
				return make(SearchResults, 0)
			} else if exitErr.ExitCode() != 1 {
				t.Fatalf("ripgrep failed: %v, output: %s", err, string(output))
			}
		}
	}

	results := make(SearchResults, 0)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse format: "filepath:line: content" or "./filepath:line: content"
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

// TestSearchComparison compares MCP search results with grep and ripgrep
func TestSearchComparison(t *testing.T) {
	// Check if ripgrep is available
	hasRipgrep := true
	if _, err := exec.LookPath("rg"); err != nil {
		t.Log("ripgrep not found, skipping rg comparisons")
		hasRipgrep = false
	}

	// Define test cases
	testCases := []TestCase{
		{
			Name:        "Type definition - UserService",
			Pattern:     "UserService",
			Description: "Search for UserService type/class across all languages",
			Language:    "all",
		},
		{
			Name:        "Type definition - AuthService",
			Pattern:     "AuthService",
			Description: "Search for AuthService type/class",
			Language:    "all",
		},
		{
			Name:        "Interface/trait - Database",
			Pattern:     "Database",
			Description: "Search for Database interface/trait",
			Language:    "all",
		},
		{
			Name:        "Error string - invalid credentials",
			Pattern:     "invalid credentials",
			Description: "Search for error message string",
			Language:    "all",
		},
		{
			Name:        "Error string - invalid token",
			Pattern:     "invalid token",
			Description: "Search for token validation error",
			Language:    "all",
		},
		{
			Name:        "Go - GetUser function",
			Pattern:     "GetUser",
			Description: "Search for GetUser function in Go (capitalized)",
			Language:    "go",
		},
		{
			Name:        "JavaScript - async keyword",
			Pattern:     "async",
			Description: "Search for async functions in JavaScript",
			Language:    "js",
		},
		{
			Name:        "Python - type hints",
			Pattern:     "Optional",
			Description: "Search for Optional type hints in Python",
			Language:    "python",
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
			if _, err := os.Stat(absFixtureDir); os.IsNotExist(err) {
				t.Skipf("Fixture directory not found: %s", absFixtureDir)
			}

			// Run all search tools
			lciResults := runLCIGrep(t, absFixtureDir, tc.Pattern).Normalize()
			grepResults := runGrepSearch(t, absFixtureDir, tc.Pattern).Normalize()

			var rgResults SearchResults
			if hasRipgrep {
				rgResults = runRipgrepSearch(t, absFixtureDir, tc.Pattern).Normalize()
			}

			// Compare results
			t.Logf("Pattern: %s", tc.Pattern)
			t.Logf("Description: %s", tc.Description)
			t.Logf("lci grep results: %d matches", len(lciResults))
			t.Logf("grep results: %d matches", len(grepResults))
			if hasRipgrep {
				t.Logf("ripgrep results: %d matches", len(rgResults))
			}

			// EXACT MATCH REQUIREMENT: lci grep should match grep exactly
			assert.Equal(t, len(grepResults), len(lciResults),
				"lci grep should find EXACTLY the same number of matches as grep")

			// Get file paths for comparison
			lciFiles := lciResults.FilePaths()
			grepFiles := grepResults.FilePaths()

			// Log file paths for debugging
			t.Logf("lci grep files: %v", lciFiles)
			t.Logf("grep files: %v", grepFiles)

			// EXACT file coverage match required
			assert.ElementsMatch(t, grepFiles, lciFiles,
				"lci grep should find EXACTLY the same files as grep")

			// Verify each specific match (file:line pairs)
			for _, grepMatch := range grepResults {
				found := false
				var lciLine int
				for _, lciMatch := range lciResults {
					if lciMatch.FilePath == grepMatch.FilePath {
						lciLine = lciMatch.Line
						if lciMatch.Line == grepMatch.Line {
							found = true
							break
						}
					}
				}
				if !found && lciLine > 0 {
					t.Logf("DEBUG: grep found %s:%d, but lci found line %d", grepMatch.FilePath, grepMatch.Line, lciLine)
				}
				assert.True(t, found,
					"lci grep should find match at %s:%d that grep found",
					grepMatch.FilePath, grepMatch.Line)
			}

			if hasRipgrep {
				rgFiles := rgResults.FilePaths()
				t.Logf("ripgrep files: %v", rgFiles)

				// Compare with ripgrep (should also be exact)
				assert.ElementsMatch(t, rgFiles, lciFiles,
					"lci grep should find EXACTLY the same files as ripgrep")
			}

			// Generate comparison report
			reportComparison(t, tc, lciResults, grepResults, rgResults)
		})
	}
}

// reportComparison generates a detailed comparison report
func reportComparison(t *testing.T, tc TestCase, mcpResults, grepResults, rgResults SearchResults) {
	report := map[string]interface{}{
		"test_case":     tc.Name,
		"pattern":       tc.Pattern,
		"language":      tc.Language,
		"mcp_count":     len(mcpResults),
		"grep_count":    len(grepResults),
		"ripgrep_count": len(rgResults),
		"mcp_files":     mcpResults.FilePaths(),
		"grep_files":    grepResults.FilePaths(),
		"ripgrep_files": rgResults.FilePaths(),
	}

	// Save report to file
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal report: %v", err)
		return
	}

	reportDir := "test-reports"
	_ = os.MkdirAll(reportDir, 0755)

	// Replace spaces and slashes to create a valid filename
	safeName := strings.ReplaceAll(tc.Name, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	reportFile := filepath.Join(reportDir, fmt.Sprintf("%s.json", safeName))
	if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
		t.Logf("Failed to write report: %v", err)
	} else {
		t.Logf("Report saved to: %s", reportFile)
	}
}

// TestBenchmarkSearchPerformance benchmarks search performance
func TestBenchmarkSearchPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	pattern := "UserService"
	fixtureDir, err := filepath.Abs(getFixturePath("all"))
	require.NoError(t, err)

	// Benchmark lci grep (trigram search)
	t.Run("lci-grep", func(t *testing.T) {
		results := runLCIGrep(t, fixtureDir, pattern)
		t.Logf("lci grep found %d results", len(results))
	})

	// Benchmark grep
	t.Run("grep", func(t *testing.T) {
		results := runGrepSearch(t, fixtureDir, pattern)
		t.Logf("grep found %d results", len(results))
	})

	// Benchmark ripgrep if available
	if _, err := exec.LookPath("rg"); err == nil {
		t.Run("ripgrep", func(t *testing.T) {
			results := runRipgrepSearch(t, fixtureDir, pattern)
			t.Logf("ripgrep found %d results", len(results))
		})
	}
}
