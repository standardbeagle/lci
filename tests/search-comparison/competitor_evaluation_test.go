package searchcomparison

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// TestCompetitorEvaluation compares LCI's search performance against grep/ripgrep
// after index build cost is excluded (simulating MCP server persistent index scenario)
func TestCompetitorEvaluation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping competitor evaluation in short mode")
	}

	hasRipgrep := true
	if _, err := exec.LookPath("rg"); err != nil {
		t.Log("ripgrep not found, skipping rg performance comparison")
		hasRipgrep = false
	}

	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	// Build index once (simulating MCP server persistent index)
	t.Logf("Building persistent index for %s...", absFixtureDir)
	indexStart := time.Now()
	idx, cfg := setupPersistentIndex(t, absFixtureDir)
	defer idx.Close()
	indexDuration := time.Since(indexStart)
	t.Logf("Index built in %v (one-time cost, excluded from search measurements)", indexDuration)

	// Performance test patterns
	patterns := []struct {
		Name     string
		Pattern  string
		Expected string
	}{
		{
			Name:     "Common word - high frequency",
			Pattern:  "user",
			Expected: "High result count - tests result handling performance",
		},
		{
			Name:     "Common word - medium frequency",
			Pattern:  "database",
			Expected: "Medium result count - balanced test",
		},
		{
			Name:     "Rare pattern - low frequency",
			Pattern:  "ValidateCredentials",
			Expected: "Low result count - tests search efficiency",
		},
		{
			Name:     "Single character - maximum frequency",
			Pattern:  "a",
			Expected: "Maximum result count - stress test",
		},
		{
			Name:     "Multi-word - exact match",
			Pattern:  "invalid credentials",
			Expected: "Multi-word exact match - tests pattern complexity",
		},
		{
			Name:     "Special chars - operators",
			Pattern:  "==",
			Expected: "Special character handling performance",
		},
	}

	for _, tc := range patterns {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Pattern: %q - %s", tc.Pattern, tc.Expected)

			// Benchmark LCI search (EXCLUDING index build)
			lciStart := time.Now()
			lciResults := searchWithPersistentIndex(t, idx, cfg, tc.Pattern)
			lciDuration := time.Since(lciStart)
			t.Logf("LCI search (persistent index): %d results in %v", len(lciResults), lciDuration)

			// Benchmark standard grep
			grepStart := time.Now()
			grepResults := runGrepSearch(t, absFixtureDir, tc.Pattern)
			grepDuration := time.Since(grepStart)
			t.Logf("grep (no index): %d results in %v", len(grepResults), grepDuration)

			// Benchmark ripgrep
			if hasRipgrep {
				rgStart := time.Now()
				rgResults := runRipgrepSearch(t, absFixtureDir, tc.Pattern)
				rgDuration := time.Since(rgStart)
				t.Logf("ripgrep (no index): %d results in %v", len(rgResults), rgDuration)

				// Compare performance
				grepRatio := float64(lciDuration) / float64(grepDuration)
				rgRatio := float64(lciDuration) / float64(rgDuration)

				t.Logf("Performance ratio (LCI/grep): %.2fx", grepRatio)
				t.Logf("Performance ratio (LCI/ripgrep): %.2fx", rgRatio)

				// Document comparison context
				if grepRatio < 1.0 {
					t.Logf("✅ LCI is %.2fx FASTER than grep (indexed search advantage)", 1.0/grepRatio)
				} else {
					t.Logf("⚠️  LCI is %.2fx slower than grep (expected for small codebases)", grepRatio)
					t.Logf("    Note: Indexed search excels on large codebases with repeated searches")
				}
			}
		})
	}
}

// TestCompetitorEvaluation_RepeatedSearches demonstrates the advantage of persistent indexes
// with multiple searches on the same codebase (realistic MCP server usage pattern)
func TestCompetitorEvaluation_RepeatedSearches(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping repeated search evaluation in short mode")
	}

	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	// Build persistent index once
	t.Logf("Building persistent index...")
	indexStart := time.Now()
	idx, cfg := setupPersistentIndex(t, absFixtureDir)
	defer idx.Close()
	indexBuildCost := time.Since(indexStart)
	t.Logf("Index built in %v", indexBuildCost)

	// Test patterns (realistic AI assistant query sequence)
	patterns := []string{
		"user", "service", "database", "auth", "token",
		"error", "result", "function", "class", "struct",
		"interface", "type", "return", "async", "await",
		"import", "export", "const", "let", "var",
		"if", "else", "for", "while", "switch",
	}

	t.Logf("\nSimulating %d sequential searches (realistic AI assistant usage)...", len(patterns))

	// LCI: Multiple searches on persistent index
	lciStart := time.Now()
	lciTotalResults := 0
	for _, pattern := range patterns {
		results := searchWithPersistentIndex(t, idx, cfg, pattern)
		lciTotalResults += len(results)
	}
	lciTotalDuration := time.Since(lciStart)
	lciAvgDuration := lciTotalDuration / time.Duration(len(patterns))

	t.Logf("LCI (persistent index): %d total results in %v (avg: %v per search)",
		lciTotalResults, lciTotalDuration, lciAvgDuration)

	// grep: Multiple searches from scratch
	grepStart := time.Now()
	grepTotalResults := 0
	for _, pattern := range patterns {
		results := runGrepSearch(t, absFixtureDir, pattern)
		grepTotalResults += len(results)
	}
	grepTotalDuration := time.Since(grepStart)
	grepAvgDuration := grepTotalDuration / time.Duration(len(patterns))

	t.Logf("grep (no index): %d total results in %v (avg: %v per search)",
		grepTotalResults, grepTotalDuration, grepAvgDuration)

	// Performance comparison
	totalRatio := float64(lciTotalDuration) / float64(grepTotalDuration)
	t.Logf("\n=== Performance Summary ===")
	t.Logf("Total searches: %d", len(patterns))
	t.Logf("LCI total time: %v (index build: %v, searches: %v)",
		indexBuildCost+lciTotalDuration, indexBuildCost, lciTotalDuration)
	t.Logf("grep total time: %v (no index cost)", grepTotalDuration)
	t.Logf("Performance ratio (LCI/grep): %.2fx", totalRatio)

	if totalRatio < 1.0 {
		t.Logf("✅ LCI persistent index is %.2fx FASTER for repeated searches", 1.0/totalRatio)
	} else {
		t.Logf("⚠️  LCI is %.2fx slower (expected for small test fixtures)", totalRatio)
		t.Logf("    Note: Advantage increases with codebase size and search frequency")
	}

	// Calculate break-even point
	searchesNeededToAmortize := int(float64(indexBuildCost) / float64(lciAvgDuration))
	t.Logf("\n=== Break-even Analysis ===")
	t.Logf("Index build cost: %v", indexBuildCost)
	t.Logf("Average search time (LCI): %v", lciAvgDuration)
	t.Logf("Average search time (grep): %v", grepAvgDuration)
	t.Logf("Break-even point: ~%d searches", searchesNeededToAmortize)
	t.Logf("Current test: %d searches (%.1fx break-even)",
		len(patterns), float64(len(patterns))/float64(searchesNeededToAmortize))
}

// TestCompetitorEvaluation_EdgeCases tests edge case patterns with persistent index
func TestCompetitorEvaluation_EdgeCases(t *testing.T) {
	fixtureDir := getFixturePath("all")
	absFixtureDir, err := filepath.Abs(fixtureDir)
	require.NoError(t, err)

	// Build persistent index
	idx, cfg := setupPersistentIndex(t, absFixtureDir)
	defer idx.Close()

	edgeCases := []struct {
		Name          string
		Pattern       string
		Expected      string
		ShouldError   bool
		AllowMismatch bool // Allow result count differences
	}{
		{
			Name:        "Empty string",
			Pattern:     "",
			Expected:    "Should return error",
			ShouldError: true,
		},
		{
			Name:     "Very long pattern",
			Pattern:  "ThisIsAVeryLongPatternThatProbablyDoesNotExistAnywhereInTheCodebaseButWeShouldTestItAnyway",
			Expected: "Should return no results efficiently",
		},
		{
			Name:          "Pattern with newline (escaped)",
			Pattern:       "\\n",
			Expected:      "Should search for literal backslash-n",
			AllowMismatch: true, // lci and grep handle escaped backslash-n differently
		},
		{
			Name:     "Pattern with multiple spaces",
			Pattern:  "user   service",
			Expected: "Should search for exact spacing",
		},
		{
			Name:          "Unicode - emoji",
			Pattern:       "😀",
			Expected:      "Should handle Unicode emoji",
			AllowMismatch: true, // Unicode handling may differ
		},
		{
			Name:          "Unicode - Chinese",
			Pattern:       "用户",
			Expected:      "Should handle CJK characters",
			AllowMismatch: true,
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

			// Test LCI with persistent index
			if tc.ShouldError {
				// Expect error for invalid patterns
				opts := types.SearchOptions{

					MaxResults:      100,
					CaseInsensitive: true,
				}
				_, err := idx.SearchWithOptions(tc.Pattern, opts)
				if err == nil {
					t.Errorf("Expected error for pattern %q, got nil", tc.Pattern)
				} else {
					t.Logf("✅ Correctly returned error: %v", err)
				}
				return
			}

			lciResults := searchWithPersistentIndex(t, idx, cfg, tc.Pattern)
			t.Logf("LCI: %d results", len(lciResults))

			// Test standard grep
			grepResults := runGrepSearch(t, absFixtureDir, tc.Pattern)
			t.Logf("grep: %d results", len(grepResults))

			// Compare results
			if !tc.AllowMismatch && len(lciResults) != len(grepResults) {
				t.Errorf("Result mismatch for pattern %q: LCI=%d, grep=%d",
					tc.Pattern, len(lciResults), len(grepResults))
			}
		})
	}
}

// setupPersistentIndex creates a persistent index for testing (simulates MCP server)
func setupPersistentIndex(t *testing.T, projectRoot string) (*indexing.MasterIndex, *config.Config) {
	t.Helper()

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: projectRoot,
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   500,
			MaxFileCount:     10000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:   200,
			MaxGoroutines: 4,
		},
		Search: config.Search{
			MaxResults:         1000,
			MaxContextLines:    5,
			EnableFuzzy:        false,
			MergeFileResults:   true,
			EnsureCompleteStmt: false,
		},
		Include: []string{"**/*"},
		Exclude: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/.git/**",
		},
	}

	idx := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	err := idx.IndexDirectory(ctx, projectRoot)
	require.NoError(t, err, "Failed to build persistent index")

	stats := idx.GetStats()
	fileCount, _ := stats["file_count"].(int)
	symbolCount, _ := stats["symbol_count"].(int)
	t.Logf("Indexed %d files, %d symbols", fileCount, symbolCount)

	return idx, cfg
}

// searchWithPersistentIndex performs a search on an existing index (EXCLUDING build cost)
func searchWithPersistentIndex(t *testing.T, idx *indexing.MasterIndex, cfg *config.Config, pattern string) []searchtypes.GrepResult {
	t.Helper()

	opts := types.SearchOptions{
		MaxResults:      100,
		CaseInsensitive: true,
		MaxContextLines: 0, // Minimal context for performance test
	}

	results, err := idx.SearchWithOptions(pattern, opts)
	require.NoError(t, err, "Search failed for pattern %q", pattern)

	return results
}
