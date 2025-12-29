package regex_analyzer

import (
	"container/list"
	"regexp"
	"sync"
	"time"
)

// SimpleRegexPattern represents a parsed and cached simple regex pattern
type SimpleRegexPattern struct {
	// Original pattern
	Pattern string

	// Extracted literals for trigram filtering
	Literals []string

	// Compiled regex for execution
	Compiled *regexp.Regexp

	// Cache metadata
	CacheKey     string
	LastAccessed time.Time
	AccessCount  int64
}

// RegexCache provides LRU caching for both simple and complex regex patterns
type RegexCache struct {
	// Caches
	simpleCache  map[string]*SimpleRegexPattern
	complexCache map[string]*regexp.Regexp

	// LRU tracking
	simpleLRU  *list.List
	complexLRU *list.List

	// Synchronization
	mu sync.RWMutex

	// Configuration
	maxSimpleSize    int
	maxComplexSize   int
	maxPatternLength int

	// Statistics
	stats CacheStats
}

// CacheStats tracks cache performance statistics
type CacheStats struct {
	SimpleHits       int64
	SimpleMisses     int64
	ComplexHits      int64
	ComplexMisses    int64
	SimpleEvictions  int64
	ComplexEvictions int64
	TotalRequests    int64
}

// NewRegexCache creates a new regex cache with specified sizes
func NewRegexCache(maxSimpleSize, maxComplexSize int) *RegexCache {
	return &RegexCache{
		simpleCache:      make(map[string]*SimpleRegexPattern),
		complexCache:     make(map[string]*regexp.Regexp),
		simpleLRU:        list.New(),
		complexLRU:       list.New(),
		maxSimpleSize:    maxSimpleSize,
		maxComplexSize:   maxComplexSize,
		maxPatternLength: 1000, // Maximum pattern length to cache
	}
}

// GetRegex retrieves a regex pattern from cache, parsing and caching if necessary
func (rc *RegexCache) GetRegex(pattern string, caseInsensitive bool) (*SimpleRegexPattern, *regexp.Regexp) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.stats.TotalRequests++

	// Check pattern length limits
	if len(pattern) > rc.maxPatternLength {
		return nil, nil // Don't cache extremely long patterns
	}

	cacheKey := rc.buildCacheKey(pattern, caseInsensitive)

	// Check simple cache first
	if simple, exists := rc.simpleCache[cacheKey]; exists {
		simple.LastAccessed = time.Now()
		simple.AccessCount++
		rc.moveSimpleToFront(simple)
		rc.stats.SimpleHits++
		return simple, nil
	}

	// Check complex cache
	if complex, exists := rc.complexCache[cacheKey]; exists {
		rc.moveComplexToFront(cacheKey)
		rc.stats.ComplexHits++
		return nil, complex
	}

	// Cache miss - increment miss counters
	rc.stats.SimpleMisses++
	rc.stats.ComplexMisses++

	return nil, nil
}

// CacheSimple caches a simple regex pattern
func (rc *RegexCache) CacheSimple(pattern *SimpleRegexPattern, caseInsensitive bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	cacheKey := rc.buildCacheKey(pattern.Pattern, caseInsensitive)

	// Check if already exists
	if _, exists := rc.simpleCache[cacheKey]; exists {
		return
	}

	// Evict if necessary
	if len(rc.simpleCache) >= rc.maxSimpleSize {
		rc.evictSimple()
	}

	// Add to cache
	pattern.CacheKey = cacheKey
	pattern.LastAccessed = time.Now()
	pattern.AccessCount = 1

	rc.simpleLRU.PushFront(pattern)
	rc.simpleCache[cacheKey] = pattern
}

// CacheComplex caches a complex regex pattern
func (rc *RegexCache) CacheComplex(pattern string, compiled *regexp.Regexp, caseInsensitive bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	cacheKey := rc.buildCacheKey(pattern, caseInsensitive)

	// Check if already exists
	if _, exists := rc.complexCache[cacheKey]; exists {
		return
	}

	// Evict if necessary
	if len(rc.complexCache) >= rc.maxComplexSize {
		rc.evictComplex()
	}

	// Add to cache
	rc.complexCache[cacheKey] = compiled
	rc.complexLRU.PushFront(cacheKey)
}

// buildCacheKey creates a consistent cache key for a pattern
func (rc *RegexCache) buildCacheKey(pattern string, caseInsensitive bool) string {
	if caseInsensitive {
		return "(?i)" + pattern
	}
	return pattern
}

// moveSimpleToFront moves a simple pattern to the front of the LRU list
func (rc *RegexCache) moveSimpleToFront(pattern *SimpleRegexPattern) {
	// Find the element in the LRU list
	for e := rc.simpleLRU.Front(); e != nil; e = e.Next() {
		if e.Value.(*SimpleRegexPattern) == pattern {
			rc.simpleLRU.MoveToFront(e)
			return
		}
	}
}

// moveComplexToFront moves a complex pattern to the front of the LRU list
func (rc *RegexCache) moveComplexToFront(cacheKey string) {
	// Find the element in the LRU list
	for e := rc.complexLRU.Front(); e != nil; e = e.Next() {
		if e.Value.(string) == cacheKey {
			rc.complexLRU.MoveToFront(e)
			return
		}
	}
}

// evictSimple removes the least recently used simple pattern
func (rc *RegexCache) evictSimple() {
	if rc.simpleLRU.Len() == 0 {
		return
	}

	back := rc.simpleLRU.Back()
	if back != nil {
		pattern := back.Value.(*SimpleRegexPattern)
		delete(rc.simpleCache, pattern.CacheKey)
		rc.simpleLRU.Remove(back)
		rc.stats.SimpleEvictions++
	}
}

// evictComplex removes the least recently used complex pattern
func (rc *RegexCache) evictComplex() {
	if rc.complexLRU.Len() == 0 {
		return
	}

	back := rc.complexLRU.Back()
	if back != nil {
		cacheKey := back.Value.(string)
		delete(rc.complexCache, cacheKey)
		rc.complexLRU.Remove(back)
		rc.stats.ComplexEvictions++
	}
}

// GetStats returns cache statistics
func (rc *RegexCache) GetStats() CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.stats
}

// Clear clears all cached patterns
func (rc *RegexCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.simpleCache = make(map[string]*SimpleRegexPattern)
	rc.complexCache = make(map[string]*regexp.Regexp)
	rc.simpleLRU = list.New()
	rc.complexLRU = list.New()

	// Reset statistics
	rc.stats = CacheStats{}
}

// GetSize returns current cache sizes
func (rc *RegexCache) GetSize() (simple, complex int) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return len(rc.simpleCache), len(rc.complexCache)
}

// CleanupExpired removes patterns that haven't been accessed recently
func (rc *RegexCache) CleanupExpired(maxAge time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	now := time.Now()

	// Clean simple cache
	var toRemoveSimple []*SimpleRegexPattern
	for _, pattern := range rc.simpleCache {
		if now.Sub(pattern.LastAccessed) > maxAge {
			toRemoveSimple = append(toRemoveSimple, pattern)
		}
	}

	for _, pattern := range toRemoveSimple {
		delete(rc.simpleCache, pattern.CacheKey)
		// Remove from LRU list
		for e := rc.simpleLRU.Front(); e != nil; e = e.Next() {
			if e.Value.(*SimpleRegexPattern) == pattern {
				rc.simpleLRU.Remove(e)
				break
			}
		}
	}

	// Clean complex cache
	var toRemoveComplex []string
	for cacheKey := range rc.complexCache {
		// For complex patterns, we don't have access time tracking
		// This is a simplified cleanup - in practice, we might want to add it
		if len(rc.complexCache) > rc.maxComplexSize/2 {
			// Remove some entries if cache is getting full
			toRemoveComplex = append(toRemoveComplex, cacheKey)
			if len(toRemoveComplex) >= rc.maxComplexSize/4 {
				break
			}
		}
	}

	for _, cacheKey := range toRemoveComplex {
		delete(rc.complexCache, cacheKey)
		// Remove from LRU list
		for e := rc.complexLRU.Front(); e != nil; e = e.Next() {
			if e.Value.(string) == cacheKey {
				rc.complexLRU.Remove(e)
				break
			}
		}
	}
}

// GetHitRatio returns the cache hit ratio for simple patterns
func (rc *RegexCache) GetHitRatio() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	total := rc.stats.SimpleHits + rc.stats.SimpleMisses
	if total == 0 {
		return 0.0
	}
	return float64(rc.stats.SimpleHits) / float64(total)
}

// GetComplexHitRatio returns the cache hit ratio for complex patterns
func (rc *RegexCache) GetComplexHitRatio() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	total := rc.stats.ComplexHits + rc.stats.ComplexMisses
	if total == 0 {
		return 0.0
	}
	return float64(rc.stats.ComplexHits) / float64(total)
}

// GetMostAccessedSimple returns the most frequently accessed simple patterns
func (rc *RegexCache) GetMostAccessedSimple(limit int) []*SimpleRegexPattern {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	// Create slice of patterns for sorting
	patterns := make([]*SimpleRegexPattern, 0, len(rc.simpleCache))
	for _, pattern := range rc.simpleCache {
		patterns = append(patterns, pattern)
	}

	// Sort by access count (simple bubble sort for small datasets)
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[j].AccessCount > patterns[i].AccessCount {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}

	// Return top patterns
	if len(patterns) > limit {
		patterns = patterns[:limit]
	}

	return patterns
}
