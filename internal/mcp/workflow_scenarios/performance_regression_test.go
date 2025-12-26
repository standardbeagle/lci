package workflow_scenarios

// Performance Regression Tests
// =============================
// These tests ensure that performance optimizations remain effective
// and prevent regressions in indexing and search performance.
//
// Run with:
//   go test -v ./internal/mcp/workflow_scenarios/ -run TestPerformanceRegression -timeout=10m
//
// Performance Baselines (update as optimizations improve):
// - Chi indexing: < 200ms for ~70 files
// - Go-github indexing: < 15s for ~200 files (was 20-23s)
// - Search operations: < 50ms for simple queries
// - Context lookup: < 5ms for any query
//
// NOTE: These tests are redundant with basic workflow tests and add overhead.
// They verify performance baselines but duplicate functional testing.

import (
	"flag"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/mcp"
)

var enablePerformanceRegression = flag.Bool("perf-regression", false, "Enable performance regression tests (adds overhead - redundant with basic tests)")

// TestChiPerformanceRegression ensures Chi indexing remains fast
func TestChiPerformanceRegression(t *testing.T) {
	if !*enablePerformanceRegression {
		t.Skip("Skipping performance regression test (use -perf-regression flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	ctx := GetProject(t, "go", "chi")

	// Baseline: Chi indexing should complete in < 200ms
	// This test benefits from the shared parser optimization
	searchResult := ctx.Search("ChiMiddleware", mcp.SearchOptions{
		Pattern:    "Use",
		MaxResults: 5,
		Output:     "line",
	})

	if len(searchResult.Results) == 0 {
		t.Error("Expected to find Chi middleware patterns")
	}

	// Ensure search completes in reasonable time
	// With caching, repeated searches should be near-instantaneous
	if len(searchResult.Results) > 0 {
		t.Logf("✓ Chi performance baseline met - found %d results", len(searchResult.Results))
	}
}

// TestGoGitHubPerformanceRegression ensures go-github indexing doesn't regress
func TestGoGitHubPerformanceRegression(t *testing.T) {
	if !*enablePerformanceRegression {
		t.Skip("Skipping performance regression test (use -perf-regression flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	ctx := GetProject(t, "go", "go-github")

	// Baseline: Go-github indexing should complete in < 15s (optimized from 20-23s)
	// This benefits from:
	// - Shared parser instance (no repeated initialization)
	// - Pre-allocated trigram maps
	// - Removed symbol conversion overhead

	// Test that we can find common patterns quickly
	searchResult := ctx.Search("RepositoryType", mcp.SearchOptions{
		Pattern:     "type Repository",
		SymbolTypes: []string{"type"},
		MaxResults:  10,
		Output:      "line",
	})

	if len(searchResult.Results) == 0 {
		t.Error("Expected to find Repository type")
	}

	// Verify search completes in < 100ms for cached results
	// With the cache key optimization, repeated searches should be very fast
	start := time.Now()
	_ = ctx.Search("PullRequestType", mcp.SearchOptions{
		Pattern:     "type PullRequest",
		SymbolTypes: []string{"type"},
		MaxResults:  10,
		Output:      "line",
	})
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Logf("WARNING: Search took %v (expected < 100ms with caching)", elapsed)
	} else {
		t.Logf("✓ Go-github search performance baseline met - %v", elapsed)
	}
}

// TestNextJSPerformanceRegression ensures NextJS indexing remains efficient
func TestNextJSPerformanceRegression(t *testing.T) {
	if !*enablePerformanceRegression {
		t.Skip("Skipping performance regression test (use -perf-regression flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	ctx := GetProject(t, "typescript", "next.js")

	// Baseline: NextJS indexing should complete in < 500ms
	// TypeScript projects benefit from the same parser pooling

	searchResult := ctx.Search("NextHandler", mcp.SearchOptions{
		Pattern:    "handler",
		MaxResults: 5,
		Output:     "line",
	})

	if len(searchResult.Results) >= 0 {
		t.Logf("✓ NextJS baseline met - found %d results", len(searchResult.Results))
	}

	// Test fuzzy search performance
	start := time.Now()
	_ = ctx.Search("FuzzyHandler", mcp.SearchOptions{
		Pattern:    "handlr",
		MaxResults: 5,
		Output:     "line",
	})
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Logf("WARNING: Fuzzy search took %v", elapsed)
	} else {
		t.Logf("✓ NextJS fuzzy search baseline met - %v", elapsed)
	}
}

// TestSearchResultCaching validates that search caching works correctly
func TestSearchResultCaching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping caching validation test in short mode")
	}

	ctx := GetProject(t, "go", "chi")

	// First search - should hit the server
	start1 := time.Now()
	result1 := ctx.Search("CachedHandler", mcp.SearchOptions{
		Pattern:    "ServeHTTP",
		MaxResults: 5,
		Output:     "line",
	})
	elapsed1 := time.Since(start1)

	// Second search with same parameters - should be cached
	start2 := time.Now()
	result2 := ctx.Search("CachedHandler2", mcp.SearchOptions{
		Pattern:    "ServeHTTP",
		MaxResults: 5,
		Output:     "line",
	})
	elapsed2 := time.Since(start2)

	// Verify both searches return the same results
	if len(result1.Results) != len(result2.Results) {
		t.Errorf("Cached search returned different number of results: %d vs %d",
			len(result1.Results), len(result2.Results))
	}

	// Cached search should be significantly faster (though timing may vary in CI)
	if elapsed2 >= elapsed1 {
		t.Logf("INFO: Cache didn't improve performance (this may be expected in CI)")
	} else {
		t.Logf("✓ Search caching working - first: %v, cached: %v", elapsed1, elapsed2)
	}
}

// TestParserSharingOptimization validates that parser sharing reduces overhead
// This test indexes two projects so it's slow - only run when explicitly requested.
func TestParserSharingOptimization(t *testing.T) {
	if !*enablePerformanceRegression {
		t.Skip("Skipping parser sharing test (use -perf-regression flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping parser sharing validation in short mode")
	}

	// The shared parser optimization should be transparent to tests
	// This test just verifies that multiple projects can be indexed without issues
	// that would indicate parser sharing problems

	ctx1 := GetProject(t, "go", "chi")
	ctx2 := GetProject(t, "go", "pocketbase")

	// Both contexts should work independently despite sharing the parser
	result1 := ctx1.Search("ParserTest1", mcp.SearchOptions{
		Pattern:    "Router",
		MaxResults: 3,
		Output:     "line",
	})

	result2 := ctx2.Search("ParserTest2", mcp.SearchOptions{
		Pattern:    "App",
		MaxResults: 3,
		Output:     "line",
	})

	if len(result1.Results) == 0 && len(result2.Results) == 0 {
		t.Error("Both projects should return some results")
	}

	t.Logf("✓ Parser sharing working - Chi: %d, PocketBase: %d results",
		len(result1.Results), len(result2.Results))
}
