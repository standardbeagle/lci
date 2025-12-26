package search_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	testutil "github.com/standardbeagle/lci/internal/testing"
)

// TestIndexingPerformance_RequirementsSnapshot validates that indexing meets performance requirements
func TestIndexingPerformance_RequirementsSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// Generate a reasonable sized codebase
	numFiles := 50
	numSymbolsPerFile := 20
	expectedDuration := 4 * time.Second // Increased to account for CI variability

	// Scale down for quick validation in CI
	if testing.Short() {
		numFiles = 10
		numSymbolsPerFile = 10
		expectedDuration = 2 * time.Second
	}

	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf(`package pkg%d

`, i)
		for j := 0; j < numSymbolsPerFile; j++ {
			content += fmt.Sprintf(`
// Function%d does something
func Function%d() string {
	return "result %d"
}

type Type%d struct {
	Field1 string
	Field2 int
}

func (t *Type%d) Method%d() string {
	return t.Field1
}
`, j, j, j, j, j, j)
		}

		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create indexer
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Include: []string{"**/*.go"},
	}

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	// Time indexing ONLY
	indexStart := time.Now()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index: %v", err)
	}
	indexDuration := time.Since(indexStart)

	stats := indexer.GetIndexStats()
	t.Logf("Indexed %d files with %d symbols in %v", stats.FileCount, stats.SymbolCount, indexDuration)

	// Indexing should be fast for this size
	if indexDuration > expectedDuration {
		t.Errorf("Indexing too slow: %v (expected <%v)", indexDuration, expectedDuration)
	}
}

// TestSearchPerformance_RequirementsSnapshot validates that search (not indexing) meets performance requirements
func TestSearchPerformance_RequirementsSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// Generate a reasonable sized codebase
	numFiles := 50
	numSymbolsPerFile := 20

	// Scale down for quick validation in CI
	if testing.Short() {
		numFiles = 10
		numSymbolsPerFile = 10
	}

	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf(`package pkg%d

`, i)
		for j := 0; j < numSymbolsPerFile; j++ {
			content += fmt.Sprintf(`
// Function%d does something
func Function%d() string {
	return "result %d"
}

type Type%d struct {
	Field1 string
	Field2 int
}

func (t *Type%d) Method%d() string {
	return t.Field1
}
`, j, j, j, j, j, j)
		}

		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create indexer
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Include: []string{"**/*.go"},
	}

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	// Build index BEFORE starting search performance tests
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index: %v", err)
	}

	stats := indexer.GetIndexStats()
	t.Logf("Pre-indexed %d files with %d symbols for search testing", stats.FileCount, stats.SymbolCount)

	// Create search engine
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Warmup: run a few searches to prime caches and trigger GC before measurements
	for i := 0; i < 3; i++ {
		_ = engine.Search("warmup", fileIDs, 10)
	}
	runtime.GC()

	// Get performance scaler for adaptive thresholds
	scaler := testutil.NewPerformanceScaler(t)
	scaler.LogScalingFactors(t)

	// Test basic search performance
	// In short mode we have Function0-Function9, in full mode Function0-Function19
	exactSearchPattern := "Function5" // Works in both modes
	if !testing.Short() && numFiles >= 20 {
		exactSearchPattern = "Function10" // More specific in full mode
	}

	// Search performance thresholds:
	// - Base search overhead: ~0.5ms
	// - Per-result processing: ~2.5µs with LineOffsets (was ~10µs with strings.Split)
	// - Target: search_time < 5ms for most queries
	searchPatterns := []struct {
		pattern      string
		expectHits   bool
		baseDuration float64 // in milliseconds
	}{
		{exactSearchPattern, true, 5}, // Exact match (~100 results) - must be <5ms
		{"Type", true, 15},            // High-match pattern (~2000 results) - must be <15ms
		{"Method", true, 10},          // Medium-match pattern (~1000 results) - must be <10ms
		{"nonexistent", false, 2},     // No results - must be <2ms (instant rejection)
	}

	for _, test := range searchPatterns {
		t.Run("Search_"+test.pattern, func(t *testing.T) {
			// Scale the duration expectation based on runtime conditions
			maxDuration := time.Duration(scaler.ScaleDuration(test.baseDuration)) * time.Millisecond

			start := time.Now()
			results := engine.Search(test.pattern, fileIDs, 100)
			duration := time.Since(start)

			t.Logf("Search for %q: %d results in %v (limit: %v)", test.pattern, len(results), duration, maxDuration)

			if duration > maxDuration {
				t.Errorf("Search too slow: %v (expected <%v)", duration, maxDuration)
			}

			if test.expectHits && len(results) == 0 {
				t.Error("Expected results but got none")
			}
			if !test.expectHits && len(results) > 0 {
				t.Errorf("Expected no results but got %d", len(results))
			}
		})
	}

	// Test enhanced search performance
	t.Run("EnhancedSearchPerformance", func(t *testing.T) {
		// Enhanced search has higher base threshold
		maxDuration := time.Duration(scaler.ScaleDuration(200)) * time.Millisecond

		start := time.Now()
		results := engine.SearchDetailed("Function", fileIDs, 10)
		duration := time.Since(start)

		t.Logf("Detailed search: %d results in %v (limit: %v)", len(results), duration, maxDuration)

		// Detailed search can be slower but should still be reasonable
		if duration > maxDuration {
			t.Errorf("Detailed search too slow: %v (expected <%v)", duration, maxDuration)
		}

		// Verify enhanced results have proper structure
		for i, result := range results {
			if result.RelationalData == nil {
				t.Errorf("Result %d missing relational data", i)
			}
		}
	})

	// Test concurrent search performance
	t.Run("ConcurrentSearchPerformance", func(t *testing.T) {
		concurrency := 10
		done := make(chan time.Duration, concurrency)

		// Scale thresholds based on environment (CI, race detector, GOMAXPROCS)
		maxTotalDuration := time.Duration(scaler.ScaleDuration(100)) * time.Millisecond
		maxIndividualDuration := time.Duration(scaler.ScaleDuration(50)) * time.Millisecond

		start := time.Now()
		for i := 0; i < concurrency; i++ {
			go func(id int) {
				pattern := fmt.Sprintf("Function%d", id)
				searchStart := time.Now()
				results := engine.Search(pattern, fileIDs, 10)
				duration := time.Since(searchStart)

				if len(results) == 0 {
					t.Errorf("Concurrent search %d found no results", id)
				}
				done <- duration
			}(i)
		}

		// Collect all durations
		var maxDuration time.Duration
		for i := 0; i < concurrency; i++ {
			d := <-done
			if d > maxDuration {
				maxDuration = d
			}
		}
		totalDuration := time.Since(start)

		t.Logf("Completed %d concurrent searches in %v (max individual: %v)",
			concurrency, totalDuration, maxDuration)
		t.Logf("Scaled thresholds: maxTotal=%v, maxIndividual=%v", maxTotalDuration, maxIndividualDuration)

		// Concurrent searches should complete efficiently
		// Retry with backoff to handle timing variance
		maxRetries := 2
		for attempt := 0; attempt < maxRetries; attempt++ {
			if totalDuration <= maxTotalDuration && maxDuration <= maxIndividualDuration {
				break // Success
			}
			if attempt < maxRetries-1 {
				t.Logf("Concurrent search attempt %d/%d: total=%v max=%v, retrying...", attempt+1, maxRetries, totalDuration, maxDuration)
				time.Sleep(200 * time.Millisecond)
				// Re-run concurrent searches
				start = time.Now()
				done = make(chan time.Duration, concurrency)
				for i := 0; i < concurrency; i++ {
					go func(idx int) {
						s := time.Now()
						_ = engine.Search(fmt.Sprintf("Function%d", idx), fileIDs, 10)
						done <- time.Since(s)
					}(i)
				}
				// Recalculate durations
				maxDuration = 0
				for i := 0; i < concurrency; i++ {
					d := <-done
					if d > maxDuration {
						maxDuration = d
					}
				}
				totalDuration = time.Since(start)
				continue
			}
			if totalDuration > maxTotalDuration {
				t.Errorf("Concurrent searches too slow: %v (expected <%v)", totalDuration, maxTotalDuration)
			}
			if maxDuration > maxIndividualDuration {
				t.Errorf("Individual concurrent search too slow: %v (expected <%v)", maxDuration, maxIndividualDuration)
			}
		}
	})
}
