package regex_analyzer

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// Performance comparison test between hybrid regex engine and direct regex
func TestHybridRegexPerformanceComparison(t *testing.T) {
	// Create test data with multiple files
	testData := createPerformanceTestData()

	// Create hybrid engine
	engine := NewHybridRegexEngine(1000, 1000, nil)

	// File provider for the test data
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if int(id) < len(testData) {
			return testData[id].Content, true
		}
		return nil, false
	}

	// Test patterns of varying complexity
	patterns := []struct {
		name    string
		pattern string
		desc    string
	}{
		{"simple_literal", "func", "Simple literal that should benefit from trigram filtering"},
		{"simple_class", "class", "Simple class literal"},
		{"complex_wildcard", "func.*\\{", "Complex pattern with wildcard"},
		{"complex_alternation", "(func|class|struct)", "Complex alternation pattern"},
		{"realworld_function", "function\\s+\\w+\\s*\\(", "Real-world function pattern"},
		{"realworld_error", "\\w+Error\\s*\\(", "Real-world error pattern"},
	}

	for _, tc := range patterns {
		t.Run(tc.name, func(t *testing.T) {
			// Get all file IDs
			var allFileIDs []types.FileID
			for i := range testData {
				allFileIDs = append(allFileIDs, types.FileID(i))
			}

			// Warm up both systems
			engine.SearchWithRegex(tc.pattern, false, contentProvider, allFileIDs)
			directRegex := regexp.MustCompile(tc.pattern)
			for _, file := range testData {
				directRegex.FindAllIndex(file.Content, -1)
			}

			// Benchmark hybrid engine
			start := time.Now()
			hybridMatches := 0
			for i := 0; i < 100; i++ {
				matches, _ := engine.SearchWithRegex(tc.pattern, false, contentProvider, allFileIDs)
				hybridMatches += len(matches)
			}
			hybridTime := time.Since(start)

			// Benchmark direct regex
			start = time.Now()
			directMatches := 0
			for i := 0; i < 100; i++ {
				for _, file := range testData {
					matches := directRegex.FindAllIndex(file.Content, -1)
					directMatches += len(matches)
				}
			}
			directTime := time.Since(start)

			// Verify results are equivalent
			if hybridMatches != directMatches {
				t.Errorf("Match count mismatch: hybrid=%d, direct=%d", hybridMatches, directMatches)
			}

			// Calculate performance improvement
			speedup := float64(directTime) / float64(hybridTime)

			t.Logf("Pattern: %s (%s)", tc.pattern, tc.desc)
			t.Logf("  Hybrid engine: %v (%d matches)", hybridTime, hybridMatches)
			t.Logf("  Direct regex:  %v (%d matches)", directTime, directMatches)
			t.Logf("  Speedup: %.2fx", speedup)

			// We expect significant speedup for simple patterns with good trigram filtering
			if tc.name == "simple_literal" || tc.name == "simple_class" {
				if speedup < 2.0 {
					t.Logf("WARNING: Expected significant speedup for simple pattern, got %.2fx", speedup)
				}
			}
		})
	}
}

// Test cache hit performance
func TestHybridRegexCachePerformance(t *testing.T) {
	engine := NewHybridRegexEngine(100, 100, nil)

	testData := createPerformanceTestData()
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if int(id) < len(testData) {
			return testData[id].Content, true
		}
		return nil, false
	}

	var allFileIDs []types.FileID
	for i := range testData {
		allFileIDs = append(allFileIDs, types.FileID(i))
	}

	pattern := "function"

	// First run (cache miss)
	start := time.Now()
	matches1, result1 := engine.SearchWithRegex(pattern, false, contentProvider, allFileIDs)
	firstRun := time.Since(start)

	// Second run (cache hit)
	start = time.Now()
	matches2, result2 := engine.SearchWithRegex(pattern, false, contentProvider, allFileIDs)
	secondRun := time.Since(start)

	// Verify cache behavior
	if !result1.CacheHit {
		t.Logf("First run was cache miss as expected")
	} else {
		t.Errorf("First run should be cache miss, got hit")
	}

	if !result2.CacheHit {
		t.Errorf("Second run should be cache hit, got miss")
	} else {
		t.Logf("Second run was cache hit as expected")
	}

	// Verify results are identical
	if len(matches1) != len(matches2) {
		t.Errorf("Results should be identical: %d vs %d matches", len(matches1), len(matches2))
	}

	// Cache should provide significant speedup
	cacheSpeedup := float64(firstRun) / float64(secondRun)
	t.Logf("First run (cache miss): %v", firstRun)
	t.Logf("Second run (cache hit): %v", secondRun)
	t.Logf("Cache speedup: %.2fx", cacheSpeedup)

	if cacheSpeedup < 5.0 {
		t.Logf("WARNING: Expected significant cache speedup, got %.2fx", cacheSpeedup)
	}
}

// Test candidate filtering effectiveness
func TestHybridRegexCandidateFiltering(t *testing.T) {
	engine := NewHybridRegexEngine(100, 100, nil)

	// Create test data with clearly separated content
	testData := []*types.FileInfo{
		{ID: 1, Path: "file1.go", Content: []byte("func processData() { return true; }")},
		{ID: 2, Path: "file2.go", Content: []byte("class MyClass { void method() {} }")},
		{ID: 3, Path: "file3.go", Content: []byte("struct Point { x int; y int; }")},
		{ID: 4, Path: "file4.js", Content: []byte("let calculate = () => { return 42; }")},
		{ID: 5, Path: "file5.go", Content: []byte("error ProcessFailed { message string; }")},
	}

	contentProvider := func(id types.FileID) ([]byte, bool) {
		if int(id) <= 5 {
			return testData[int(id)-1].Content, true
		}
		return nil, false
	}

	allFileIDs := []types.FileID{1, 2, 3, 4, 5}

	testCases := []struct {
		pattern            string
		expectedCandidates int // Number of files that should be filtered
		expectedMatches    int // Number of files that should actually match
	}{
		{"func", 1, 1},   // Only file1 contains "func"
		{"class", 1, 1},  // Only file2 contains "class"
		{"let", 1, 1},    // Only file4 contains "let"
		{"error", 1, 1},  // Only file5 contains "error"
		{"struct", 1, 1}, // Only file3 contains "struct"
	}

	for _, tc := range testCases {
		t.Run(tc.pattern, func(t *testing.T) {
			_, result := engine.SearchWithRegex(tc.pattern, false, contentProvider, allFileIDs)

			t.Logf("Pattern: %s", tc.pattern)
			t.Logf("  Total candidates: %d", result.CandidatesTotal)
			t.Logf("  Filtered candidates: %d", result.CandidatesFiltered)
			t.Logf("  Expected filtered: %d", tc.expectedCandidates)
			t.Logf("  Matches found: %d", result.MatchesFound)
			t.Logf("  Expected matches: %d", tc.expectedMatches)
			t.Logf("  Execution path: %v", result.ExecutionPath)

			// Verify filtering worked correctly
			if result.CandidatesFiltered != tc.expectedCandidates {
				t.Errorf("Expected %d filtered candidates, got %d", tc.expectedCandidates, result.CandidatesFiltered)
			}

			// Verify we found the expected matches
			if result.MatchesFound != tc.expectedMatches {
				t.Errorf("Expected %d matches, got %d", tc.expectedMatches, result.MatchesFound)
			}

			// Should use trigram filtering for simple patterns
			if result.ExecutionPath != PathSimpleTrigramFiltered {
				t.Errorf("Expected trigram filtering, got %v", result.ExecutionPath)
			}
		})
	}
}

// Helper function to create test data for performance testing
func createPerformanceTestData() []*types.FileInfo {
	contents := []string{
		// Go files
		`func processData() error {
			if err := validateInput(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			return processResult()
		}`,

		`func calculateTotal(items []Item) float64 {
			var total float64
			for _, item := range items {
				total += item.Price * item.Quantity
			}
			return total
		}`,

		// JavaScript files (no func keyword)
		`let processData = (data) => {
			return data.map(item => ({
				...item,
				processed: true
			}));
		}`,

		`class MyClass {
			constructor(name) {
				this.name = name;
			}

			method() {
				return this.name.toUpperCase();
			}
		}`,

		// Python files (no func keyword)
		`def process_data(data):
			"""Process the input data and return results."""
			return [transform(item) for item in data]`,

		`class DataProcessor:
			def __init__(self, config):
				self.config = config

			def process(self, data):
				return self.apply_filters(data)`,

		// Error handling
		`type ValidationError struct {
			Field   string
			Message string
		}`,

		`class ProcessingError extends Error {
			constructor(message, code) {
				super(message);
				this.code = code;
			}
		}`,
	}

	var files []*types.FileInfo
	for i, content := range contents {
		files = append(files, &types.FileInfo{
			ID:      types.FileID(i + 1),
			Path:    fmt.Sprintf("file%d.txt", i+1),
			Content: []byte(content),
		})
	}

	return files
}

// BenchmarkHybridRegexVsDirect benchmarks hybrid engine vs direct regex
func BenchmarkHybridRegexVsDirect(b *testing.B) {
	testData := createPerformanceTestData()
	engine := NewHybridRegexEngine(1000, 1000, nil)

	contentProvider := func(id types.FileID) ([]byte, bool) {
		if int(id) < len(testData) {
			return testData[id].Content, true
		}
		return nil, false
	}

	var allFileIDs []types.FileID
	for i := range testData {
		allFileIDs = append(allFileIDs, types.FileID(i))
	}

	patterns := []string{"func", "class", "function", "error", "struct"}

	for _, pattern := range patterns {
		b.Run("Hybrid_"+pattern, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				engine.SearchWithRegex(pattern, false, contentProvider, allFileIDs)
			}
		})

		directRegex := regexp.MustCompile(pattern)
		b.Run("Direct_"+pattern, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, file := range testData {
					directRegex.FindAllIndex(file.Content, -1)
				}
			}
		})
	}
}
