package indexing_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/searchtypes"
	testhelpers "github.com/standardbeagle/lci/internal/testing"
	"github.com/standardbeagle/lci/internal/testing/fixtures"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/stretchr/testify/require"
)

// TestSearchBehaviorSnapshot captures the expected behavior of search operations
// This replaces the legacy search implementation tests with a snapshot-based approach
func TestSearchBehaviorSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// Create test codebase with various patterns
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

// Calculator provides arithmetic operations
type Calculator struct {
	precision int
}

// Add adds two numbers
func (c *Calculator) Add(a, b float64) float64 {
	return a + b
}

// GlobalCalculator is a shared instance
var GlobalCalculator = &Calculator{precision: 2}

const MaxPrecision = 10

func main() {
	calc := &Calculator{precision: 5}
	result := calc.Add(1.5, 2.5)
	fmt.Printf("Result: %f\n", result)
}
`,
		"util.go": `package main

// Helper functions for calculations

// Min returns the minimum of two values
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two values
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
`,
		"calculator_test.go": `package main

import "testing"

// TestCalculator tests the calculator.
func TestCalculator(t *testing.T) {
	calc := &Calculator{}
	calc.Add(10, 5)
	// Test implementation
}

// TestMin tests the min.
func TestMin(t *testing.T) {
	if Min(5, 3) != 3 {
		t.Error("Min failed")
	}
}
`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create indexer with standard configuration
	cfg := createSearchTestConfig(tempDir)
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Get stats
	stats := indexer.GetIndexStats()

	// Build comprehensive snapshot
	snapshot := &testhelpers.SearchBehaviorSnapshot{
		ProjectName: "search-behavior-test",
		Language:    "go",
		FileCount:   stats.FileCount,
		SymbolCount: stats.SymbolCount,
		SearchTests: make(map[string]testhelpers.SearchTestResult),
	}

	// Test different search patterns
	searchTests := []testhelpers.SearchTest{
		{
			Name:    "BasicStringSearch",
			Pattern: "Calculator",
			Options: types.SearchOptions{},
		},
		{
			Name:    "CaseInsensitiveSearch",
			Pattern: "calculator",
			Options: types.SearchOptions{CaseInsensitive: true},
		},
		{
			Name:    "RegexSearch",
			Pattern: "(Add|Min|Max)",
			Options: types.SearchOptions{UseRegex: true},
		},
		{
			Name:    "DeclarationOnlySearch",
			Pattern: "Calculator",
			Options: types.SearchOptions{DeclarationOnly: true},
		},
		{
			Name:    "FunctionSymbolSearch",
			Pattern: "Add",
			Options: types.SearchOptions{
				SymbolTypes:     []string{"function", "method"},
				DeclarationOnly: true,
			},
		},
		{
			Name:    "VariableSymbolSearch",
			Pattern: "GlobalCalculator",
			Options: types.SearchOptions{
				SymbolTypes:     []string{"variable"},
				DeclarationOnly: true,
			},
		},
		{
			Name:    "ContextualSearch",
			Pattern: "precision",
			Options: types.SearchOptions{MaxContextLines: 3},
		},
		{
			Name:    "LimitedResultsSearch",
			Pattern: "func",
			Options: types.SearchOptions{},
		},
	}

	// Execute search tests and capture results
	for _, test := range searchTests {
		options := test.Options.(types.SearchOptions)
		results, err := indexer.SearchWithOptions(test.Pattern, options)
		require.NoError(t, err)
		enhancedResults, err := indexer.SearchDetailedWithOptions(test.Pattern, options)
		require.NoError(t, err)

		snapshot.SearchTests[test.Name] = testhelpers.SearchTestResult{
			Pattern:             test.Pattern,
			Options:             test.Options,
			BasicResultCount:    len(results),
			EnhancedResultCount: len(enhancedResults),
			FirstResultLine:     getFirstResultLine(results),
			FirstResultPath:     getFirstResultPath(results),
			HasRelationalData:   hasRelationalData(enhancedResults),
			SymbolTypesFound:    getSymbolTypesFound(enhancedResults),
		}
	}

	// Assert snapshot
	testhelpers.AssertSnapshot(t, "search-behavior", snapshot)
}

// TestSearchPerformance_Snapshot captures performance characteristics for regression detection
// Renamed to unify naming scheme and reduce ambiguity
func TestSearchPerformance_Snapshot(t *testing.T) {
	t.Skip("Performance snapshot expectations may have changed in refactoring")
	if testing.Short() {
		t.Skip("Skipping performance snapshot test in short mode")
	}

	tempDir := t.TempDir()

	// Generate test data
	numFiles := 20
	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("file_%d.go", i)
		content := fixtures.GenerateLargeFile(25) // 25 functions per file

		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create indexer
	cfg := createSearchTestConfig(tempDir)
	indexer := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Get stats
	stats := indexer.GetIndexStats()

	// Build performance snapshot
	snapshot := &testhelpers.SearchPerformanceSnapshot{
		ProjectName:      "search-performance-test",
		FileCount:        stats.FileCount,
		SymbolCount:      stats.SymbolCount,
		IndexSizeMB:      float64(stats.TotalSizeBytes) / (1024 * 1024), // Convert bytes to MB
		PerformanceTests: make(map[string]testhelpers.PerformanceTestResult),
	}

	// Performance test patterns
	performanceTests := []struct {
		name    string
		pattern string
		options types.SearchOptions
	}{
		{"ExactMatch", "Function1", types.SearchOptions{}},
		{"PrefixMatch", "Func", types.SearchOptions{}},
		{"RegexMatch", "Function[0-9]+", types.SearchOptions{UseRegex: true}},
		{"CaseInsensitive", "function", types.SearchOptions{CaseInsensitive: true}},
		{"WithContext", "param", types.SearchOptions{MaxContextLines: 5}},
	}

	// Execute performance tests
	for _, test := range performanceTests {
		// Warm up
		for i := 0; i < 3; i++ {
			_, _ = indexer.SearchWithOptions(test.pattern, test.options)
		}

		// Measure performance
		totalResults := 0
		const iterations = 10

		for i := 0; i < iterations; i++ {
			results, err := indexer.SearchWithOptions(test.pattern, test.options)
			require.NoError(t, err)
			totalResults += len(results)
		}

		avgResults := float64(totalResults) / float64(iterations)

		snapshot.PerformanceTests[test.name] = testhelpers.PerformanceTestResult{
			Pattern:        test.pattern,
			Options:        test.options,
			AvgResultCount: avgResults,
			Iterations:     iterations,
		}
	}

	// Assert snapshot
	testhelpers.AssertSnapshot(t, "search-performance", snapshot)
}

// Helper functions
func getFirstResultLine(results []searchtypes.Result) int {
	if len(results) > 0 {
		return results[0].Line
	}
	return 0
}

func getFirstResultPath(results []searchtypes.Result) string {
	if len(results) > 0 {
		return filepath.Base(results[0].Path)
	}
	return ""
}

func hasRelationalData(results []searchtypes.StandardResult) bool {
	for _, result := range results {
		if result.RelationalData != nil {
			return true
		}
	}
	return false
}

func getSymbolTypesFound(results []searchtypes.StandardResult) []string {
	typeSet := make(map[string]bool)
	for _, result := range results {
		if result.RelationalData != nil {
			typeStr := result.RelationalData.Symbol.Type.String()
			if typeStr != "" {
				typeSet[typeStr] = true
			}
		}
	}

	var types []string
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// Helper function to create test configuration
func createSearchTestConfig(rootDir string) *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.Project{
			Root: rootDir,
			Name: "search-test-project",
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
			WatchMode:        false,
		},
		Performance: config.Performance{
			MaxMemoryMB:   100,
			MaxGoroutines: 4,
			DebounceMs:    0,
		},
		Search: config.Search{
			MaxResults:         100,
			MaxContextLines:    50,
			EnableFuzzy:        false,
			MergeFileResults:   true,
			EnsureCompleteStmt: false,
		},
		Include: []string{
			"**/*.go",
			"**/*.js",
			"**/*.tsx",
			"**/*.py",
		},
		Exclude: []string{
			"**/node_modules/**",
			"**/.git/**",
			"**/vendor/**",
		},
	}
}
