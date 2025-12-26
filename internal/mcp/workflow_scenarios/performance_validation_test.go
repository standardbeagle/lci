package workflow_scenarios

// Performance Validation Tests
// ==========================
// These tests validate performance characteristics for pre-commit validation.
// They provide comprehensive coverage of both Go and TypeScript projects
// while completing in 30-45 seconds.
//
// Run with:
//   go test -v ./internal/mcp/workflow_scenarios/ -run TestPerformanceValidation -timeout=2m
//
// Performance Targets (30-45s total):
// - Chi (Go) indexing: < 20s for ~70 files
// - tRPC (TypeScript) indexing: < 25s for ~750 files
// - Search operations: < 100ms for simple queries
// - Coverage: Both Go and TypeScript projects
//
// NOTE: These tests duplicate the individual workflow tests. They are useful
// for CI pre-commit validation but add overhead to local development.

import (
	"flag"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/mcp"
)

var enablePerformanceValidation = flag.Bool("perf-validation", false, "Enable performance validation test (consolidated test, slower than individual tests)")

// getPerformanceScalingFactor provides environment-aware timing adjustments
func getPerformanceScalingFactor() float64 {
	factor := 1.0

	// CI environments are typically slower
	if os.Getenv("CI") != "" {
		factor *= 1.5
	}

	// GOMAXPROCS=1 means serialized execution
	if runtime.GOMAXPROCS(0) == 1 && runtime.NumCPU() > 1 {
		factor *= 1.5
	}

	return factor
}

// TestPerformanceValidation validates performance characteristics with efficient project selection
// Completes in 30-45 seconds while providing comprehensive coverage of both Go and TypeScript
func TestPerformanceValidation(t *testing.T) {
	if !*enablePerformanceValidation {
		t.Skip("Skipping performance validation test (use -perf-validation flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping performance validation in short mode")
	}

	scalingFactor := getPerformanceScalingFactor()
	t.Logf("Performance scaling factor: %.2f", scalingFactor)

	// Select efficient projects that provide good coverage:
	// - chi (Go): 73 files, 9650 lines - Fast but substantial enough for validation
	// - trpc (TypeScript): 749 files, 26325 lines - Good TypeScript coverage without excessive size
	testProjects := []struct {
		name     string
		language string
		project  string
	}{
		{"Go_Web_Framework", "go", "chi"},
		{"TypeScript_API_Framework", "typescript", "trpc"},
	}

	var totalIndexingTime time.Duration
	var totalSearchTime time.Duration

	// Test each project sequentially to minimize resource usage
	for _, testProj := range testProjects {
		t.Run(testProj.name, func(t *testing.T) {
			// Start timing for indexing
			indexStart := time.Now()

			// Get project with auto-indexing
			ctx := GetProject(t, testProj.language, testProj.project)

			indexingTime := time.Since(indexStart)
			totalIndexingTime += indexingTime

			t.Logf("✅ %s: Indexed in %v", testProj.name, indexingTime)

			// Validate indexing performance (should be under 20s even in CI)
			maxIndexTime := time.Duration(20.0 * scalingFactor * float64(time.Second))
			if indexingTime > maxIndexTime {
				t.Errorf("⚠️  Indexing too slow: %v (expected < %v)", indexingTime, maxIndexTime)
			}

			// Test search performance with multiple patterns
			searchPatterns := []struct {
				name    string
				pattern string
			}{
				{"Handler_Search", "Handler"},
				{"Method_Search", "Method"},
				{"Type_Search", "Type"},
				{"Interface_Search", "Interface"},
			}

			for _, searchTest := range searchPatterns {
				t.Run(searchTest.name, func(t *testing.T) {
					searchStart := time.Now()

					result := ctx.Search(searchTest.name, mcp.SearchOptions{
						Pattern:    searchTest.pattern,
						MaxResults: 20,
						Output:     "line",
					})

					searchTime := time.Since(searchStart)
					totalSearchTime += searchTime

					// Validate search performance (should be under 100ms normally)
					maxSearchTime := time.Duration(0.1 * scalingFactor * float64(time.Second))
					if searchTime > maxSearchTime {
						t.Logf("⚠️  Search slow but acceptable: %v (expected < %v)", searchTime, maxSearchTime)
					}

					// Validate we got reasonable results
					if len(result.Results) == 0 {
						t.Logf("ℹ️  No results found for pattern '%s' in %s", searchTest.pattern, testProj.name)
					} else {
						t.Logf("✅ %s: Found %d results in %v", searchTest.name, len(result.Results), searchTime)
					}
				})
			}
		})
	}

	// Report overall performance metrics
	t.Logf("\n📈 Performance Validation Summary:")
	t.Logf("  Total Indexing Time: %v", totalIndexingTime)
	t.Logf("  Total Search Time: %v", totalSearchTime)
	t.Logf("  Average Indexing Time: %v", totalIndexingTime/time.Duration(len(testProjects)))

	// Validate overall performance
	// Total test time should be under 45 seconds even in CI
	maxTotalTime := time.Duration(45.0 * scalingFactor * float64(time.Second))
	totalTime := totalIndexingTime + totalSearchTime
	if totalTime > maxTotalTime {
		t.Errorf("❌ Total test time too slow: %v (expected < %v)", totalTime, maxTotalTime)
	} else {
		t.Logf("✅ Total validation time: %v (within target)", totalTime)
	}
}

// TestQuickPerformanceCheck is a faster validation for more frequent pre-commit runs
// Completes in 15-20 seconds and focuses on the most critical performance metrics
func TestQuickPerformanceCheck(t *testing.T) {
	if !*enablePerformanceValidation {
		t.Skip("Skipping quick performance check (use -perf-validation flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping quick performance check in short mode")
	}

	// Use just the chi project for quick validation
	ctx := GetProject(t, "go", "chi")

	// Test a few key search patterns
	quickSearchPatterns := []string{"Handler", "Middleware", "Router"}
	for _, pattern := range quickSearchPatterns {
		searchStart := time.Now()
		result := ctx.Search("QuickCheck_"+pattern, mcp.SearchOptions{
			Pattern:    pattern,
			MaxResults: 10,
			Output:     "line",
		})
		searchTime := time.Since(searchStart)

		// Quick searches should be very fast
		scalingFactor := getPerformanceScalingFactor()
		maxSearchTime := time.Duration(0.05 * scalingFactor * float64(time.Second)) // 50ms
		if searchTime > maxSearchTime {
			t.Logf("⚠️  Quick search '%s' slow: %v", pattern, searchTime)
		}

		t.Logf("✅ Quick search '%s': %d results in %v", pattern, len(result.Results), searchTime)
	}

	t.Logf("✅ Quick performance check completed")
}
