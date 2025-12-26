package git

import (
	"sync"
	"sync/atomic"
	"time"
)

// FrequencyCache provides lightweight in-memory caching for frequency analysis
// Uses sync.Map for lock-free concurrent access
type FrequencyCache struct {
	entries    sync.Map // map[string]*cacheEntry
	ttl        time.Duration
	maxEntries int

	// Statistics
	hits   atomic.Uint64
	misses atomic.Uint64

	// Cleanup control
	cleanupMu     sync.Mutex
	lastCleanup   time.Time
	cleanupPeriod time.Duration
}

// cacheEntry holds a cached value with expiration
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
	createdAt time.Time
}

// NewFrequencyCache creates a new cache with the specified TTL
func NewFrequencyCache(ttl time.Duration) *FrequencyCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	return &FrequencyCache{
		ttl:           ttl,
		maxEntries:    1000,
		cleanupPeriod: time.Minute,
		lastCleanup:   time.Now(),
	}
}

// CacheKey generates a cache key for frequency analysis
func CacheKey(filePath string, window TimeWindow, granularity FrequencyGranularity) string {
	return "freq:" + filePath + ":" + string(window) + ":" + string(granularity)
}

// CacheKeyFile generates a cache key for file-level analysis
func CacheKeyFile(filePath string, window TimeWindow) string {
	return CacheKey(filePath, window, GranularityFile)
}

// CacheKeySymbol generates a cache key for symbol-level analysis
func CacheKeySymbol(filePath string, symbolName string, window TimeWindow) string {
	return "freq:" + filePath + ":" + symbolName + ":" + string(window) + ":symbol"
}

// CacheKeyPattern generates a cache key for pattern analysis
func CacheKeyPattern(pattern string, window TimeWindow) string {
	return "freq:pattern:" + pattern + ":" + string(window)
}

// Get retrieves a value from the cache
func (c *FrequencyCache) Get(key string) (interface{}, bool) {
	value, ok := c.entries.Load(key)
	if !ok {
		c.misses.Add(1)
		return nil, false
	}

	entry := value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		// Entry expired
		c.entries.Delete(key)
		c.misses.Add(1)
		return nil, false
	}

	c.hits.Add(1)
	return entry.data, true
}

// Set stores a value in the cache
func (c *FrequencyCache) Set(key string, data interface{}) {
	// Trigger cleanup if needed
	c.maybeCleanup()

	entry := &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
		createdAt: time.Now(),
	}

	c.entries.Store(key, entry)
}

// SetWithTTL stores a value with a custom TTL
func (c *FrequencyCache) SetWithTTL(key string, data interface{}, ttl time.Duration) {
	c.maybeCleanup()

	entry := &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
		createdAt: time.Now(),
	}

	c.entries.Store(key, entry)
}

// Delete removes a value from the cache
func (c *FrequencyCache) Delete(key string) {
	c.entries.Delete(key)
}

// Clear removes all entries from the cache
func (c *FrequencyCache) Clear() {
	c.entries.Range(func(key, _ interface{}) bool {
		c.entries.Delete(key)
		return true
	})
}

// maybeCleanup triggers cleanup if enough time has passed
func (c *FrequencyCache) maybeCleanup() {
	c.cleanupMu.Lock()
	defer c.cleanupMu.Unlock()

	if time.Since(c.lastCleanup) < c.cleanupPeriod {
		return
	}

	c.lastCleanup = time.Now()
	go c.cleanup()
}

// cleanup removes expired entries
func (c *FrequencyCache) cleanup() {
	now := time.Now()
	count := 0

	c.entries.Range(func(key, value interface{}) bool {
		entry := value.(*cacheEntry)
		if now.After(entry.expiresAt) {
			c.entries.Delete(key)
		} else {
			count++
		}
		return true
	})

	// If still over max entries, remove oldest
	if count > c.maxEntries {
		c.evictOldest(count - c.maxEntries)
	}
}

// evictOldest removes the N oldest entries
func (c *FrequencyCache) evictOldest(n int) {
	type entryWithKey struct {
		key       string
		createdAt time.Time
	}

	var entries []entryWithKey

	c.entries.Range(func(key, value interface{}) bool {
		entry := value.(*cacheEntry)
		entries = append(entries, entryWithKey{
			key:       key.(string),
			createdAt: entry.createdAt,
		})
		return true
	})

	// Sort by creation time (oldest first)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].createdAt.Before(entries[i].createdAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Delete oldest N
	for i := 0; i < n && i < len(entries); i++ {
		c.entries.Delete(entries[i].key)
	}
}

// Stats returns cache statistics
func (c *FrequencyCache) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()

	var count int
	c.entries.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CacheStats{
		Hits:       hits,
		Misses:     misses,
		HitRate:    hitRate,
		EntryCount: count,
		TTL:        c.ttl,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits       uint64        `json:"hits"`
	Misses     uint64        `json:"misses"`
	HitRate    float64       `json:"hit_rate"`
	EntryCount int           `json:"entry_count"`
	TTL        time.Duration `json:"ttl_seconds"`
}

// GetFileFrequency retrieves cached file frequency data
func (c *FrequencyCache) GetFileFrequency(filePath string, window TimeWindow) (*FileChangeFrequency, bool) {
	key := CacheKeyFile(filePath, window)
	data, ok := c.Get(key)
	if !ok {
		return nil, false
	}
	freq, ok := data.(*FileChangeFrequency)
	return freq, ok
}

// SetFileFrequency caches file frequency data
func (c *FrequencyCache) SetFileFrequency(filePath string, window TimeWindow, freq *FileChangeFrequency) {
	key := CacheKeyFile(filePath, window)
	c.Set(key, freq)
}

// GetReport retrieves a cached analysis report
func (c *FrequencyCache) GetReport(pattern string, window TimeWindow) (*ChangeFrequencyReport, bool) {
	key := CacheKeyPattern(pattern, window)
	data, ok := c.Get(key)
	if !ok {
		return nil, false
	}
	report, ok := data.(*ChangeFrequencyReport)
	return report, ok
}

// SetReport caches an analysis report
func (c *FrequencyCache) SetReport(pattern string, window TimeWindow, report *ChangeFrequencyReport) {
	key := CacheKeyPattern(pattern, window)
	c.Set(key, report)
}

// InvalidateFile removes all cached data for a file
func (c *FrequencyCache) InvalidateFile(filePath string) {
	windows := []TimeWindow{Window7Days, Window30Days, Window90Days, Window1Year}

	for _, w := range windows {
		c.Delete(CacheKeyFile(filePath, w))
	}
}

// InvalidatePattern removes cached data for a pattern
func (c *FrequencyCache) InvalidatePattern(pattern string) {
	windows := []TimeWindow{Window7Days, Window30Days, Window90Days, Window1Year}

	for _, w := range windows {
		c.Delete(CacheKeyPattern(pattern, w))
	}
}

// GetOrCompute retrieves from cache or computes and caches the result
func (c *FrequencyCache) GetOrCompute(key string, compute func() (interface{}, error)) (interface{}, error) {
	// Check cache first
	if data, ok := c.Get(key); ok {
		return data, nil
	}

	// Compute the value
	data, err := compute()
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.Set(key, data)

	return data, nil
}

// Warm pre-populates the cache with frequently accessed data
func (c *FrequencyCache) Warm(entries map[string]interface{}) {
	for key, data := range entries {
		c.Set(key, data)
	}
}
