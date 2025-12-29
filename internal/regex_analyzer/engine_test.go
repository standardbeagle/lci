package regex_analyzer

import (
	"regexp"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestHybridRegexEngineBasic tests basic functionality
func TestHybridRegexEngineBasic(t *testing.T) {
	// Use nil indexer - tests will fallback to linear search
	engine := NewHybridRegexEngine(10, 10, nil)

	// Test content
	content := []byte(`
func processData() error {
	if err := validateInput(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return processResult()
}
`)

	// Mock content provider
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return content, true
		}
		return nil, false
	}

	// Test simple pattern search
	matches, result := engine.SearchWithRegex("func", false, contentProvider, []types.FileID{1})

	if result.ExecutionPath != PathSimpleTrigramFiltered {
		t.Errorf("Expected PathSimpleTrigramFiltered, got %v", result.ExecutionPath)
	}

	if len(matches) == 0 {
		t.Errorf("Expected matches for 'func', got none")
	}

	if !result.CacheHit {
		t.Logf("First search was cache miss as expected, time=%v", result.TotalTime)
	} else {
		t.Errorf("First search should be cache miss, got hit=%v, time=%v", result.CacheHit, result.TotalTime)
	}
}

// TestHybridRegexEngineCaching tests caching behavior
func TestHybridRegexEngineCaching(t *testing.T) {
	engine := NewHybridRegexEngine(5, 5, nil)

	content := []byte("func main() {}")
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return content, true
		}
		return nil, false
	}

	// First search (cache miss)
	matches1, result1 := engine.SearchWithRegex("func", false, contentProvider, []types.FileID{1})

	if result1.CacheHit {
		t.Errorf("First search should be cache miss, got hit")
	}

	// Second search (cache hit)
	matches2, result2 := engine.SearchWithRegex("func", false, contentProvider, []types.FileID{1})

	if !result2.CacheHit {
		t.Errorf("Second search should be cache hit, got miss")
	}

	if result2.TotalTime > result1.TotalTime {
		t.Errorf("Cached search should be faster: %v > %v", result2.TotalTime, result1.TotalTime)
	}

	if len(matches1) != len(matches2) {
		t.Errorf("Cached search should return same results: %d vs %d", len(matches1), len(matches2))
	}
}

// TestHybridRegexEngineLiteralExtraction tests trigram filtering
func TestHybridRegexEngineLiteralExtraction(t *testing.T) {
	engine := NewHybridRegexEngine(10, 10, nil)

	// Test content with specific literals
	content := []byte(`
func processData() error {
	return fmt.Errorf("error")
}

class MyClass { void method() {} }
class AnotherClass { void anotherMethod() {} }
`)

	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	// Test pattern with multiple literals that will match on same line
	matches, result := engine.SearchWithRegex("class.*method", false, contentProvider, []types.FileID{1})

	t.Logf("ExecutionPath: %v, CandidatesFiltered: %d, Matches: %d",
		result.ExecutionPath, result.CandidatesFiltered, len(matches))

	if len(matches) == 0 {
		t.Errorf("Expected matches for 'class.*method', got none")
	}

	if result.ExecutionPath != PathSimpleTrigramFiltered {
		t.Errorf("Expected trigram filtering, got %v", result.ExecutionPath)
	}

	if result.CandidatesFiltered == 0 {
		t.Errorf("Expected some candidates to be filtered, got %d", result.CandidatesFiltered)
	}
}

// TestHybridRegexEngineComplexPatterns tests complex regex handling
func TestHybridRegexEngineComplexPatterns(t *testing.T) {
	engine := NewHybridRegexEngine(10, 10, nil)

	// Test content
	content := []byte("func main() { var x = 1; x = x + 1; return x; }")
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	// Test complex pattern with backreference (should be classified as complex)
	_, result := engine.SearchWithRegex("(\\w+)\\s+\\1", false, contentProvider, []types.FileID{1})

	// Note: Go's regex doesn't support backreferences, so this will likely be handled gracefully
	if result.ExecutionPath != PathError {
		t.Errorf("Complex backreference pattern should result in error or direct execution, got %v", result.ExecutionPath)
	}

	// Test another complex pattern that Go can handle
	matches2, result2 := engine.SearchWithRegex("func.*\\{", false, contentProvider, []types.FileID{1})

	t.Logf("Complex pattern - ExecutionPath: %v, Matches: %d", result2.ExecutionPath, len(matches2))
	if len(matches2) == 0 {
		t.Errorf("Expected matches for complex pattern, got none")
	}
}

// TestHybridRegexEngineCaseInsensitive tests case-insensitive search
func TestHybridRegexEngineCaseInsensitive(t *testing.T) {
	engine := NewHybridRegexEngine(10, 10, nil)

	content := []byte("FUNC main() { return; }")
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	// Test case-insensitive search
	matches, _ := engine.SearchWithRegex("func", true, contentProvider, []types.FileID{1})

	if len(matches) == 0 {
		t.Errorf("Expected matches for case-insensitive 'func', got none")
	}

	// Test case-sensitive search (should not match)
	matches2, _ := engine.SearchWithRegex("func", false, contentProvider, []types.FileID{1})

	if len(matches2) > 0 {
		t.Errorf("Expected no matches for case-sensitive 'func' in uppercase content, got %d", len(matches2))
	}
}

// TestHybridRegexEnginePerformanceMetrics tests performance tracking
func TestHybridRegexEnginePerformanceMetrics(t *testing.T) {
	engine := NewHybridRegexEngine(10, 10, nil)

	content := []byte("func test() { return; } class Test { void method() {} }")
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	// Perform multiple searches to test metrics
	patterns := []string{"func", "class", "method", "test"}
	for _, pattern := range patterns {
		matches, result := engine.SearchWithRegex(pattern, false, contentProvider, []types.FileID{1})

		if result.MatchesFound != len(matches) {
			t.Errorf("MatchesFound %d doesn't match actual matches %d", result.MatchesFound, len(matches))
		}

		if result.TotalTime < 0 {
			t.Errorf("Negative time reported: %v", result.TotalTime)
		}

		if result.CandidatesTotal != 1 {
			t.Errorf("Expected 1 candidate total, got %d", result.CandidatesTotal)
		}
	}
}

// TestHybridRegexEngineMultipleCandidates tests multiple file candidates
func TestHybridRegexEngineMultipleCandidates(t *testing.T) {
	engine := NewHybridRegexEngine(10, 10, nil)

	// Create multiple files
	files := map[types.FileID]*types.FileInfo{
		1: {ID: 1, Path: "file1.go", Content: []byte("func test1() {}")},
		2: {ID: 2, Path: "file2.go", Content: []byte("class Test2 {}")},
		3: {ID: 3, Path: "file3.go", Content: []byte("var test3 int")},
	}

	contentProvider := func(id types.FileID) ([]byte, bool) {
		if file, ok := files[id]; ok {
			return file.Content, true
		}
		return nil, false
	}

	allFileIDs := []types.FileID{1, 2, 3}

	// Search for "func" - should only match file1
	matches, result := engine.SearchWithRegex("func", false, contentProvider, allFileIDs)

	if len(matches) == 0 {
		t.Errorf("Expected matches for 'func', got none")
	}

	// Verify we have some matches and they are in the right position (0-4 bytes in file1)
	if len(matches) > 0 {
		for _, match := range matches {
			if match.Start < 0 || match.End > len(files[1].Content) {
				t.Errorf("Match position out of bounds: start=%d, end=%d", match.Start, match.End)
			}
		}
	}

	if result.CandidatesTotal != 3 {
		t.Errorf("Expected 3 total candidates, got %d", result.CandidatesTotal)
	}

	// Should filter to only file1 since it contains "func"
	if result.CandidatesFiltered != 1 {
		t.Errorf("Expected 1 filtered candidate, got %d", result.CandidatesFiltered)
	}
}

// TestHybridRegexEngineCacheEviction tests cache eviction behavior
func TestHybridRegexEngineCacheEviction(t *testing.T) {
	engine := NewHybridRegexEngine(2, 2, nil) // Small cache to test eviction

	content := []byte("test content")
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	// Fill cache beyond capacity
	patterns := []string{"pattern1", "pattern2", "pattern3", "pattern4"}
	for _, pattern := range patterns {
		engine.SearchWithRegex(pattern, false, contentProvider, []types.FileID{1})
	}

	// Check cache size
	simple, _ := engine.GetCacheSize()
	// With cache size of 2, we expect at most 2 items after adding 4 patterns
	if simple > 2 {
		t.Errorf("Expected cache size at most 2, got %d", simple)
	}
	if simple == 0 {
		t.Errorf("Expected some items to be cached, got 0")
	}

	// Get cache stats
	stats := engine.GetCacheStats()
	if stats.SimpleMisses < int64(len(patterns)) {
		t.Errorf("Expected at least %d cache misses, got %d", len(patterns), stats.SimpleMisses)
	}
}

// BenchmarkHybridRegexEnginePerformance benchmarks the hybrid engine
func BenchmarkHybridRegexEnginePerformance(b *testing.B) {
	engine := NewHybridRegexEngine(1000, 1000, nil)

	content := []byte(`
func processData() error {
	if err := validateInput(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return processResult()
}

class MyClass {
	void myMethod() {
		// Method implementation
	}
}
`)

	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	patterns := []string{"func", "class", "method", "process", "error"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		engine.SearchWithRegex(pattern, false, contentProvider, []types.FileID{1})
	}
}

// BenchmarkHybridRegexEngineVsDirect benchmarks against direct regex execution
func BenchmarkHybridRegexEngineVsDirect(b *testing.B) {
	engine := NewHybridRegexEngine(1000, 1000, nil)

	content := []byte("func test() { return; }")
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
	}
	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return fileInfo.Content, true
		}
		return nil, false
	}

	// Compile direct regex
	directRegex := regexp.MustCompile("func")

	b.Run("HybridEngine", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			engine.SearchWithRegex("func", false, contentProvider, []types.FileID{1})
		}
	})

	b.Run("DirectRegex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			directRegex.FindAllStringIndex(string(content), -1)
		}
	})
}

// TestHybridRegexEngine_AnchorPatterns tests that ^ and $ anchors work in multiline mode
// This is a regression test for the issue where ^type would only match if "type" was at
// the very start of the file content, not at the start of each line.
func TestHybridRegexEngine_AnchorPatterns(t *testing.T) {
	engine := NewHybridRegexEngine(10, 10, nil)

	// Content with multiple lines starting with different keywords
	content := []byte(`package main

import "fmt"

type Config struct {
	Name string
}

func main() {
	fmt.Println("hello")
}

type Handler struct {
	Config *Config
}
`)

	contentProvider := func(id types.FileID) ([]byte, bool) {
		if id == 1 {
			return content, true
		}
		return nil, false
	}

	t.Run("caret_matches_line_start", func(t *testing.T) {
		// ^type should match "type" at the start of lines 5 and 13
		matches, _ := engine.SearchWithRegex("^type", false, contentProvider, []types.FileID{1})

		if len(matches) != 2 {
			t.Errorf("Expected 2 matches for ^type (lines 5 and 13), got %d", len(matches))
			for i, m := range matches {
				t.Logf("Match %d: offset=%d-%d", i, m.Start, m.End)
			}
		}
	})

	t.Run("caret_with_pattern_matches_line_start", func(t *testing.T) {
		// ^func should match "func" at the start of line 9
		matches, _ := engine.SearchWithRegex("^func", false, contentProvider, []types.FileID{1})

		if len(matches) != 1 {
			t.Errorf("Expected 1 match for ^func, got %d", len(matches))
		}
	})

	t.Run("caret_no_false_positives", func(t *testing.T) {
		// ^Name should NOT match because "Name" is indented (tab/space before it)
		matches, _ := engine.SearchWithRegex("^Name", false, contentProvider, []types.FileID{1})

		if len(matches) != 0 {
			t.Errorf("Expected 0 matches for ^Name (it's indented), got %d", len(matches))
		}
	})

	t.Run("dollar_matches_line_end", func(t *testing.T) {
		// }$ should match closing braces at end of lines
		matches, _ := engine.SearchWithRegex("}$", false, contentProvider, []types.FileID{1})

		// Should match: line 7 "}", line 11 "}", line 15 "}" (struct/func closing braces)
		if len(matches) < 3 {
			t.Errorf("Expected at least 3 matches for }$, got %d", len(matches))
		}
	})

	t.Run("complex_anchor_pattern", func(t *testing.T) {
		// ^type [A-Z] should match type declarations starting at line beginning
		matches, _ := engine.SearchWithRegex("^type [A-Z]", false, contentProvider, []types.FileID{1})

		if len(matches) != 2 {
			t.Errorf("Expected 2 matches for ^type [A-Z], got %d", len(matches))
		}
	})
}
