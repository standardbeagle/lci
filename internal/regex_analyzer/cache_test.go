package regex_analyzer

import (
	"regexp"
	"testing"
	"time"

	"github.com/standardbeagle/lci/testhelpers"
)

// TestRegexCacheBasicOperations tests basic cache operations
func TestRegexCacheBasicOperations(t *testing.T) {
	cache := NewRegexCache(5, 5)

	// Test empty cache
	simple, complex := cache.GetRegex("func", false)
	if simple != nil || complex != nil {
		t.Errorf("Empty cache should return nil, got simple=%v, complex=%v", simple, complex)
	}

	// Cache a simple pattern
	simplePattern := &SimpleRegexPattern{
		Pattern:  "func",
		Literals: []string{"func"},
		Compiled: regexp.MustCompile("func"),
	}
	cache.CacheSimple(simplePattern, false)

	// Retrieve cached simple pattern
	simple, complex = cache.GetRegex("func", false)
	if simple == nil || complex != nil {
		t.Errorf("Should retrieve simple pattern, got simple=%v, complex=%v", simple, complex)
	}
	if simple.Pattern != "func" {
		t.Errorf("Pattern mismatch, expected 'func', got %q", simple.Pattern)
	}

	// Cache a complex pattern (using a pattern with backreference which Go doesn't support)
	// We'll use a simpler "complex" pattern for this test
	complexPattern := regexp.MustCompile(`func.*{`)
	cache.CacheComplex("func.*\\{", complexPattern, false)

	// Retrieve cached complex pattern
	simple, complex = cache.GetRegex("func.*\\{", false)
	if simple != nil || complex == nil {
		t.Errorf("Should retrieve complex pattern, got simple=%v, complex=%v", simple, complex)
	}
}

// TestRegexCacheLRUEviction tests LRU eviction behavior
func TestRegexCacheLRUEviction(t *testing.T) {
	cache := NewRegexCache(3, 3) // Small cache to test eviction

	// Fill simple cache to capacity
	patterns := []*SimpleRegexPattern{
		{Pattern: "func", Compiled: regexp.MustCompile("func")},
		{Pattern: "class", Compiled: regexp.MustCompile("class")},
		{Pattern: "async", Compiled: regexp.MustCompile("async")},
	}

	for i, pattern := range patterns {
		cache.CacheSimple(pattern, false)

		// Access patterns to establish LRU order
		cache.GetRegex(pattern.Pattern, false)

		if i < len(patterns)-1 {
			// Sleep a bit to ensure different access times
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Cache should be full
	simpleCount, _ := cache.GetSize()
	if simpleCount != 3 {
		t.Errorf("Expected simple cache size 3, got %d", simpleCount)
	}

	// Add one more pattern (should evict the least recently used)
	newPattern := &SimpleRegexPattern{
		Pattern:  "error",
		Literals: []string{"error"},
		Compiled: regexp.MustCompile("error"),
	}
	cache.CacheSimple(newPattern, false)

	// Check that "func" was evicted (it was the least recently used)
	simple, _ := cache.GetRegex("func", false)
	if simple != nil {
		t.Errorf("Pattern 'func' should have been evicted, but was found")
	}

	// Check that "class" is still cached
	simple, _ = cache.GetRegex("class", false)
	if simple == nil {
		t.Errorf("Pattern 'class' should still be cached")
	}
}

// TestRegexCacheCaseInsensitive tests case-insensitive caching
func TestRegexCacheCaseInsensitive(t *testing.T) {
	cache := NewRegexCache(10, 10)

	// Cache case-insensitive pattern
	pattern := &SimpleRegexPattern{
		Pattern:  "func",
		Literals: []string{"func"},
		Compiled: regexp.MustCompile("(?i)func"),
	}
	cache.CacheSimple(pattern, true)

	// Should find with case-insensitive flag
	simple, _ := cache.GetRegex("func", true)
	if simple == nil {
		t.Errorf("Should find case-insensitive pattern")
	}

	// Should NOT find without case-insensitive flag
	simple, _ = cache.GetRegex("func", false)
	if simple != nil {
		t.Errorf("Should not find pattern without case-insensitive flag")
	}

	// Should find with same pattern and case-insensitive flag
	simple, _ = cache.GetRegex("func", true)
	if simple == nil {
		t.Errorf("Should find pattern with same pattern and case-insensitive flag")
	}
}

// TestRegexCacheStatistics tests cache statistics
func TestRegexCacheStatistics(t *testing.T) {
	cache := NewRegexCache(10, 10)

	// Initial stats
	stats := cache.GetStats()
	if stats.SimpleHits != 0 || stats.SimpleMisses != 0 {
		t.Errorf("Initial stats should be zero, got hits=%d, misses=%d",
			stats.SimpleHits, stats.SimpleMisses)
	}

	// Cache miss
	_, _ = cache.GetRegex("func", false)
	stats = cache.GetStats()
	if stats.SimpleMisses != 1 {
		t.Errorf("Should have 1 simple miss, got %d", stats.SimpleMisses)
	}

	// Cache a pattern
	pattern := &SimpleRegexPattern{
		Pattern:  "func",
		Literals: []string{"func"},
		Compiled: regexp.MustCompile("func"),
	}
	cache.CacheSimple(pattern, false)

	// Cache hit
	_, _ = cache.GetRegex("func", false)
	stats = cache.GetStats()
	if stats.SimpleHits != 1 {
		t.Errorf("Should have 1 simple hit, got %d", stats.SimpleHits)
	}

	// Test hit ratio
	ratio := cache.GetHitRatio()
	expected := 1.0 / 2.0 // 1 hit out of 2 total requests
	if ratio != expected {
		t.Errorf("Expected hit ratio %.2f, got %.2f", expected, ratio)
	}
}

// TestRegexCacheConcurrentAccess tests thread safety
func TestRegexCacheConcurrentAccess(t *testing.T) {
	cache := NewRegexCache(100, 100)

	// Use simpler, non-overlapping patterns to avoid race conditions
	patterns := []string{
		"func", "class", "async", "error", "type",
		"interface", "trait", "struct", "enum", "const",
	}

	// Number of goroutines (one per pattern)
	numGoroutines := len(patterns)
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines, each working on its own pattern
	for i, pattern := range patterns {
		go func(id int, testPattern string) {
			for j := 0; j < numOperations; j++ {
				// Try to get pattern (might be cache miss or hit depending on timing)
				simple, complex := cache.GetRegex(testPattern, false)

				// If not cached, cache it
				if simple == nil && complex == nil {
					compiled, err := regexp.Compile(testPattern)
					if err == nil {
						patternObj := &SimpleRegexPattern{
							Pattern:  testPattern,
							Literals: []string{testPattern},
							Compiled: compiled,
						}
						cache.CacheSimple(patternObj, false)
					}
				}

				// Try to get again (should be cached now)
				simple, complex = cache.GetRegex(testPattern, false)
				if simple == nil && complex == nil {
					t.Errorf("Pattern %s should be cached by now", testPattern)
				}
			}
			done <- true
		}(i, pattern)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify cache has the expected patterns
	simpleCount, _ := cache.GetSize()
	if simpleCount < len(patterns) {
		t.Errorf("Cache should have at least %d items, but has %d", len(patterns), simpleCount)
	}
	if simpleCount > len(patterns) {
		t.Errorf("Cache should have at most %d items, but has %d", len(patterns), simpleCount)
	}
}

// TestRegexCacheCleanup tests cache cleanup functionality
func TestRegexCacheCleanup(t *testing.T) {
	cache := NewRegexCache(10, 10)

	// Cache some patterns
	for i := 0; i < 5; i++ {
		pattern := &SimpleRegexPattern{
			Pattern:  "pattern" + string(rune('a'+i)),
			Literals: []string{"pattern" + string(rune('a'+i))},
			Compiled: regexp.MustCompile("pattern" + string(rune('a'+i))),
		}
		cache.CacheSimple(pattern, false)
	}

	// Verify cache size
	simpleCount, _ := cache.GetSize()
	if simpleCount != 5 {
		t.Errorf("Expected cache size 5, got %d", simpleCount)
	}

	// Wait to ensure time difference with timeout
	waitStart := time.Now()
	testhelpers.WaitFor(t, func() bool {
		return time.Since(waitStart) >= 10*time.Millisecond
	}, 100*time.Millisecond)

	// Add more patterns and access some
	for i := 5; i < 7; i++ {
		pattern := &SimpleRegexPattern{
			Pattern:  "pattern" + string(rune('a'+i)),
			Literals: []string{"pattern" + string(rune('a'+i))},
			Compiled: regexp.MustCompile("pattern" + string(rune('a'+i))),
		}
		cache.CacheSimple(pattern, false)
		// Access the new pattern to make it recently used
		cache.GetRegex(pattern.Pattern, false)
	}

	// Cleanup with very short max age (should remove old patterns)
	cache.CleanupExpired(5 * time.Millisecond)

	// Check cache size - some patterns should be cleaned up
	simpleCount, _ = cache.GetSize()
	if simpleCount == 7 {
		t.Errorf("Expected some patterns to be cleaned up, but cache size is still %d", simpleCount)
	}
}

// TestRegexCacheLongPatternHandling tests handling of very long patterns
func TestRegexCacheLongPatternHandling(t *testing.T) {
	cache := NewRegexCache(10, 10)

	// Create a very long pattern
	longPattern := "func" + string(make([]byte, 2000)) // 2004 characters total

	// Try to retrieve it (should not be cached due to length limit)
	simple, _ := cache.GetRegex(longPattern, false)
	if simple != nil {
		t.Errorf("Very long pattern should not be cached, but was found")
	}

	// Try to cache it manually (this should work, but GetRegex won't cache it)
	compiled, err := regexp.Compile("func")
	if err == nil {
		pattern := &SimpleRegexPattern{
			Pattern:  longPattern,
			Literals: []string{"func"},
			Compiled: compiled,
		}
		cache.CacheSimple(pattern, false)
	}

	// Verify GetRegex still doesn't find it (length check prevents caching)
	simple, _ = cache.GetRegex(longPattern, false)
	if simple != nil {
		t.Errorf("Very long pattern should not be cached by GetRegex, but was found")
	}
}

// TestRegexCacheClear tests cache clearing
func TestRegexCacheClear(t *testing.T) {
	cache := NewRegexCache(10, 10)

	// Add some patterns
	for i := 0; i < 3; i++ {
		pattern := &SimpleRegexPattern{
			Pattern:  "pattern" + string(rune('a'+i)),
			Literals: []string{"pattern" + string(rune('a'+i))},
			Compiled: regexp.MustCompile("pattern" + string(rune('a'+i))),
		}
		cache.CacheSimple(pattern, false)
	}

	// Verify cache has items
	simpleCount, _ := cache.GetSize()
	if simpleCount != 3 {
		t.Errorf("Expected cache size 3, got %d", simpleCount)
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	simpleCount, _ = cache.GetSize()
	if simpleCount != 0 {
		t.Errorf("Cache should be empty after clear, but has %d items", simpleCount)
	}

	// Verify stats are reset
	stats := cache.GetStats()
	if stats.SimpleHits != 0 || stats.SimpleMisses != 0 {
		t.Errorf("Stats should be reset after clear, got hits=%d, misses=%d",
			stats.SimpleHits, stats.SimpleMisses)
	}
}

// TestRegexCacheMostAccessed tests most accessed patterns functionality
func TestRegexCacheMostAccessed(t *testing.T) {
	cache := NewRegexCache(10, 10)

	// Cache patterns with different access patterns
	patterns := []string{"func", "class", "async", "error"}
	for _, patternName := range patterns {
		pattern := &SimpleRegexPattern{
			Pattern:  patternName,
			Literals: []string{patternName},
			Compiled: regexp.MustCompile(patternName),
		}
		cache.CacheSimple(pattern, false)
	}

	// Access patterns different number of times
	accessCounts := []int{5, 3, 10, 1} // func:5, class:3, async:10, error:1
	for i, patternName := range patterns {
		for j := 0; j < accessCounts[i]; j++ {
			cache.GetRegex(patternName, false)
		}
	}

	// Get most accessed patterns
	mostAccessed := cache.GetMostAccessedSimple(3)

	// Should be ordered by access count: async(11), func(6), class(4), error(2)
	expectedOrder := []string{"async", "func", "class"}
	for i, expected := range expectedOrder {
		if i >= len(mostAccessed) {
			t.Errorf("Expected at least %d patterns, got %d", len(expectedOrder), len(mostAccessed))
			break
		}
		if mostAccessed[i].Pattern != expected {
			t.Errorf("Expected pattern %q at position %d, got %q (access counts: %v)",
				expected, i, mostAccessed[i].Pattern, getAccessCounts(mostAccessed))
		}
	}

	// Verify access counts are correct (each pattern starts with 1 access when cached + explicit accesses + final verification access)
	for i, patternName := range patterns {
		pattern, _ := cache.GetRegex(patternName, false)
		expectedCount := int64(accessCounts[i] + 2) // +1 for initial cache, +1 for this verification access
		if pattern.AccessCount != expectedCount {
			t.Errorf("Pattern %q should have access count %d, got %d",
				patternName, expectedCount, pattern.AccessCount)
		}
	}
}

// Helper function to get access counts for debugging
func getAccessCounts(patterns []*SimpleRegexPattern) []int64 {
	counts := make([]int64, len(patterns))
	for i, pattern := range patterns {
		counts[i] = pattern.AccessCount
	}
	return counts
}

// BenchmarkRegexCachePerformance benchmarks cache performance
func BenchmarkRegexCachePerformance(b *testing.B) {
	cache := NewRegexCache(1000, 1000)

	// Pre-populate cache
	patterns := make([]*SimpleRegexPattern, 100)
	for i := 0; i < 100; i++ {
		patternName := "pattern" + string(rune('a'+i%26))
		patterns[i] = &SimpleRegexPattern{
			Pattern:  patternName,
			Literals: []string{patternName},
			Compiled: regexp.MustCompile(patternName),
		}
		cache.CacheSimple(patterns[i], false)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pattern := patterns[b.N%len(patterns)]
			cache.GetRegex(pattern.Pattern, false)
		}
	})
}
