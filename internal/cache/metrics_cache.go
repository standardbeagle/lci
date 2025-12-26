package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cache configuration constants
const (
	DefaultMaxContentEntries = 400
	DefaultMaxSymbolEntries  = 400
	DefaultMaxParserEntries  = 200
	DefaultTTL               = 2 * time.Hour
	DefaultCleanupInterval   = 10 * time.Minute
	EstimatedBytesPerEntry   = 322.0
)

// CachedMetrics represents cached metrics data
type CachedMetrics struct {
	Data        interface{}
	CachedAt    int64 // Unix nano for atomic compare
	AccessCount int64 // Atomic counter
	ContentHash string
	SymbolName  string
	FileID      int
}

// MetricsCache provides lock-free caching using sync.Map
type MetricsCache struct {
	contentCache sync.Map // map[string]*CachedMetrics
	symbolCache  sync.Map
	parserCache  sync.Map

	// Configuration (read-only after creation)
	maxEntries     int
	maxParserCache int
	ttlNanos       int64 // TTL in nanoseconds for atomic ops
	enableContent  bool
	enableSymbol   bool
	enableParser   bool

	// Atomic counters - simple interlocked operations
	hits          int64
	misses        int64
	evictions     int64
	totalRequests int64
	parserHits    int64

	// Approximate entry counts (updated on cleanup)
	contentCount int64
	symbolCount  int64
	parserCount  int64

	createdAt   time.Time
	lastCleanup int64
}

// CacheConfig defines configuration options
type CacheConfig struct {
	MaxContentEntries int
	MaxSymbolEntries  int
	MaxParserEntries  int
	TTL               time.Duration
	EnableContent     bool
	EnableSymbol      bool
	EnableParser      bool
	AutoCleanup       bool
	CleanupInterval   time.Duration
}

// DefaultCacheConfig returns default configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxContentEntries: DefaultMaxContentEntries,
		MaxSymbolEntries:  DefaultMaxSymbolEntries,
		MaxParserEntries:  DefaultMaxParserEntries,
		TTL:               DefaultTTL,
		EnableContent:     true,
		EnableSymbol:      true,
		EnableParser:      true,
		AutoCleanup:       true,
		CleanupInterval:   DefaultCleanupInterval,
	}
}

// NewMetricsCache creates a new cache
func NewMetricsCache(config CacheConfig) *MetricsCache {
	cache := &MetricsCache{
		maxEntries:     config.MaxContentEntries,
		maxParserCache: config.MaxParserEntries,
		ttlNanos:       config.TTL.Nanoseconds(),
		enableContent:  config.EnableContent,
		enableSymbol:   config.EnableSymbol,
		enableParser:   config.EnableParser,
		createdAt:      time.Now(),
		lastCleanup:    time.Now().UnixNano(),
	}

	if config.AutoCleanup {
		go cache.startAutoCleanup(config.CleanupInterval)
	}

	return cache
}

// generateContentKey creates a cache key from content and symbol name
func generateContentKey(content []byte, symbolName string) string {
	hash := sha256.Sum256(content)
	var b strings.Builder
	b.Grow(32 + 1 + len(symbolName))
	b.WriteString(hex.EncodeToString(hash[:16]))
	b.WriteByte(':')
	b.WriteString(symbolName)
	return b.String()
}

// generateSymbolKey creates a cache key from file ID and symbol name
func generateSymbolKey(fileID int, symbolName string) string {
	var b strings.Builder
	b.Grow(11 + len(symbolName))
	b.WriteString(strconv.Itoa(fileID))
	b.WriteByte(':')
	b.WriteString(symbolName)
	return b.String()
}

// generateParserKey creates a cache key for parser-specific caching
func generateParserKey(language string, content []byte, symbolName string) string {
	hash := sha256.Sum256(content)
	var b strings.Builder
	b.Grow(len(language) + 25 + len(symbolName))
	b.WriteString(language)
	b.WriteByte(':')
	b.WriteString(hex.EncodeToString(hash[:12]))
	b.WriteByte(':')
	b.WriteString(symbolName)
	return b.String()
}

// Get retrieves cached metrics
func (mc *MetricsCache) Get(content []byte, fileID int, symbolName string) interface{} {
	atomic.AddInt64(&mc.totalRequests, 1)
	now := time.Now().UnixNano()

	// Try content cache first
	if mc.enableContent && content != nil {
		key := generateContentKey(content, symbolName)
		if val, ok := mc.contentCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				return cached.Data
			}
			// Expired - delete lazily
			mc.contentCache.Delete(key)
		}
	}

	// Try symbol cache
	if mc.enableSymbol {
		key := generateSymbolKey(fileID, symbolName)
		if val, ok := mc.symbolCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				return cached.Data
			}
			mc.symbolCache.Delete(key)
		}
	}

	atomic.AddInt64(&mc.misses, 1)
	return nil
}

// GetWithLanguage retrieves with parser cache priority
func (mc *MetricsCache) GetWithLanguage(content []byte, fileID int, symbolName string, language string) interface{} {
	atomic.AddInt64(&mc.totalRequests, 1)
	now := time.Now().UnixNano()

	// Try parser cache first
	if mc.enableParser && content != nil && language != "" {
		key := generateParserKey(language, content, symbolName)
		if val, ok := mc.parserCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				atomic.AddInt64(&mc.parserHits, 1)
				return cached.Data
			}
			mc.parserCache.Delete(key)
		}
	}

	// Try content cache
	if mc.enableContent && content != nil {
		key := generateContentKey(content, symbolName)
		if val, ok := mc.contentCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				return cached.Data
			}
			mc.contentCache.Delete(key)
		}
	}

	// Try symbol cache
	if mc.enableSymbol {
		key := generateSymbolKey(fileID, symbolName)
		if val, ok := mc.symbolCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				return cached.Data
			}
			mc.symbolCache.Delete(key)
		}
	}

	atomic.AddInt64(&mc.misses, 1)
	return nil
}

// GetWithPrecomputedHash retrieves using pre-computed hash
func (mc *MetricsCache) GetWithPrecomputedHash(contentHash [32]byte, fileID int, symbolName string) interface{} {
	atomic.AddInt64(&mc.totalRequests, 1)
	now := time.Now().UnixNano()

	if mc.enableContent {
		var b strings.Builder
		b.Grow(32 + 1 + len(symbolName))
		b.WriteString(hex.EncodeToString(contentHash[:16]))
		b.WriteByte(':')
		b.WriteString(symbolName)
		key := b.String()

		if val, ok := mc.contentCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				return cached.Data
			}
			mc.contentCache.Delete(key)
		}
	}

	if mc.enableSymbol {
		key := generateSymbolKey(fileID, symbolName)
		if val, ok := mc.symbolCache.Load(key); ok {
			cached := val.(*CachedMetrics)
			if now-atomic.LoadInt64(&cached.CachedAt) <= mc.ttlNanos {
				atomic.AddInt64(&cached.AccessCount, 1)
				atomic.AddInt64(&mc.hits, 1)
				return cached.Data
			}
			mc.symbolCache.Delete(key)
		}
	}

	atomic.AddInt64(&mc.misses, 1)
	return nil
}

// Put stores metrics in cache with size limiting
func (mc *MetricsCache) Put(content []byte, fileID int, symbolName string, metrics interface{}) {
	now := time.Now().UnixNano()
	cached := &CachedMetrics{
		Data:        metrics,
		CachedAt:    now,
		AccessCount: 1,
		SymbolName:  symbolName,
		FileID:      fileID,
	}

	if mc.enableContent && content != nil {
		key := generateContentKey(content, symbolName)
		cached.ContentHash = key
		// Check if key already exists (update vs insert)
		if _, loaded := mc.contentCache.LoadOrStore(key, cached); !loaded {
			// New entry - check size limit
			count := atomic.AddInt64(&mc.contentCount, 1)
			if count > int64(mc.maxEntries) {
				mc.evictOldestFromContent()
			}
		}
	}

	if mc.enableSymbol {
		key := generateSymbolKey(fileID, symbolName)
		if _, loaded := mc.symbolCache.LoadOrStore(key, cached); !loaded {
			count := atomic.AddInt64(&mc.symbolCount, 1)
			if count > int64(mc.maxEntries) {
				mc.evictOldestFromSymbol()
			}
		}
	}
}

// PutWithLanguage stores with parser cache and size limiting
func (mc *MetricsCache) PutWithLanguage(content []byte, fileID int, symbolName string, language string, metrics interface{}) {
	now := time.Now().UnixNano()
	cached := &CachedMetrics{
		Data:        metrics,
		CachedAt:    now,
		AccessCount: 1,
		SymbolName:  symbolName,
		FileID:      fileID,
	}

	if mc.enableParser && content != nil && language != "" {
		key := generateParserKey(language, content, symbolName)
		cached.ContentHash = key
		if _, loaded := mc.parserCache.LoadOrStore(key, cached); !loaded {
			count := atomic.AddInt64(&mc.parserCount, 1)
			if count > int64(mc.maxParserCache) {
				mc.evictOldestFromParser()
			}
		}
	}

	if mc.enableContent && content != nil {
		key := generateContentKey(content, symbolName)
		cached.ContentHash = key
		if _, loaded := mc.contentCache.LoadOrStore(key, cached); !loaded {
			count := atomic.AddInt64(&mc.contentCount, 1)
			if count > int64(mc.maxEntries) {
				mc.evictOldestFromContent()
			}
		}
	}

	if mc.enableSymbol {
		key := generateSymbolKey(fileID, symbolName)
		if _, loaded := mc.symbolCache.LoadOrStore(key, cached); !loaded {
			count := atomic.AddInt64(&mc.symbolCount, 1)
			if count > int64(mc.maxEntries) {
				mc.evictOldestFromSymbol()
			}
		}
	}
}

// evictOldestFromContent removes the oldest entry from content cache
func (mc *MetricsCache) evictOldestFromContent() {
	var oldestKey interface{}
	var oldestTime int64 = time.Now().UnixNano()

	mc.contentCache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedMetrics)
		cachedAt := atomic.LoadInt64(&cached.CachedAt)
		if cachedAt < oldestTime {
			oldestTime = cachedAt
			oldestKey = key
		}
		return true
	})

	if oldestKey != nil {
		mc.contentCache.Delete(oldestKey)
		atomic.AddInt64(&mc.contentCount, -1)
		atomic.AddInt64(&mc.evictions, 1)
	}
}

// evictOldestFromSymbol removes the oldest entry from symbol cache
func (mc *MetricsCache) evictOldestFromSymbol() {
	var oldestKey interface{}
	var oldestTime int64 = time.Now().UnixNano()

	mc.symbolCache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedMetrics)
		cachedAt := atomic.LoadInt64(&cached.CachedAt)
		if cachedAt < oldestTime {
			oldestTime = cachedAt
			oldestKey = key
		}
		return true
	})

	if oldestKey != nil {
		mc.symbolCache.Delete(oldestKey)
		atomic.AddInt64(&mc.symbolCount, -1)
		atomic.AddInt64(&mc.evictions, 1)
	}
}

// evictOldestFromParser removes the oldest entry from parser cache
func (mc *MetricsCache) evictOldestFromParser() {
	var oldestKey interface{}
	var oldestTime int64 = time.Now().UnixNano()

	mc.parserCache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedMetrics)
		cachedAt := atomic.LoadInt64(&cached.CachedAt)
		if cachedAt < oldestTime {
			oldestTime = cachedAt
			oldestKey = key
		}
		return true
	})

	if oldestKey != nil {
		mc.parserCache.Delete(oldestKey)
		atomic.AddInt64(&mc.parserCount, -1)
		atomic.AddInt64(&mc.evictions, 1)
	}
}

// CleanExpired removes expired entries
func (mc *MetricsCache) CleanExpired() int {
	now := time.Now().UnixNano()
	cleaned := int64(0)

	// Clean content cache
	contentCount := int64(0)
	mc.contentCache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedMetrics)
		if now-atomic.LoadInt64(&cached.CachedAt) > mc.ttlNanos {
			mc.contentCache.Delete(key)
			cleaned++
		} else {
			contentCount++
		}
		return true
	})
	atomic.StoreInt64(&mc.contentCount, contentCount)

	// Clean symbol cache
	symbolCount := int64(0)
	mc.symbolCache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedMetrics)
		if now-atomic.LoadInt64(&cached.CachedAt) > mc.ttlNanos {
			mc.symbolCache.Delete(key)
			cleaned++
		} else {
			symbolCount++
		}
		return true
	})
	atomic.StoreInt64(&mc.symbolCount, symbolCount)

	// Clean parser cache
	parserCount := int64(0)
	mc.parserCache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedMetrics)
		if now-atomic.LoadInt64(&cached.CachedAt) > mc.ttlNanos {
			mc.parserCache.Delete(key)
			cleaned++
		} else {
			parserCount++
		}
		return true
	})
	atomic.StoreInt64(&mc.parserCount, parserCount)

	atomic.AddInt64(&mc.evictions, cleaned)
	atomic.StoreInt64(&mc.lastCleanup, now)
	return int(cleaned)
}

// startAutoCleanup runs periodic cleanup
func (mc *MetricsCache) startAutoCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		mc.CleanExpired()
	}
}

// Stats returns cache statistics
func (mc *MetricsCache) Stats() CacheStats {
	hits := atomic.LoadInt64(&mc.hits)
	misses := atomic.LoadInt64(&mc.misses)
	totalRequests := atomic.LoadInt64(&mc.totalRequests)

	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(hits) / float64(totalRequests)
	}

	parserHits := atomic.LoadInt64(&mc.parserHits)
	parserHitRate := float64(0)
	if totalRequests > 0 {
		parserHitRate = float64(parserHits) / float64(totalRequests)
	}

	contentEntries := int(atomic.LoadInt64(&mc.contentCount))
	symbolEntries := int(atomic.LoadInt64(&mc.symbolCount))
	parserEntries := int(atomic.LoadInt64(&mc.parserCount))
	totalEntries := contentEntries + symbolEntries + parserEntries

	return CacheStats{
		Hits:              hits,
		Misses:            misses,
		Evictions:         atomic.LoadInt64(&mc.evictions),
		TotalRequests:     totalRequests,
		HitRate:           hitRate,
		ContentEntries:    contentEntries,
		SymbolEntries:     symbolEntries,
		ParserEntries:     parserEntries,
		TotalEntries:      totalEntries,
		CreatedAt:         mc.createdAt,
		LastCleanup:       time.Unix(0, atomic.LoadInt64(&mc.lastCleanup)),
		Uptime:            time.Since(mc.createdAt),
		ParserHits:        parserHits,
		ParserHitRate:     parserHitRate,
		EstimatedMemoryKB: float64(totalEntries) * EstimatedBytesPerEntry / 1024,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits              int64
	Misses            int64
	Evictions         int64
	TotalRequests     int64
	HitRate           float64
	ContentEntries    int
	SymbolEntries     int
	ParserEntries     int
	TotalEntries      int
	CreatedAt         time.Time
	LastCleanup       time.Time
	Uptime            time.Duration
	ParserHits        int64
	ParserHitRate     float64
	EstimatedMemoryKB float64
}

// Clear removes all entries and resets statistics
func (mc *MetricsCache) Clear() {
	// Clear maps by replacing with new empty Range calls
	mc.contentCache.Range(func(key, _ interface{}) bool {
		mc.contentCache.Delete(key)
		return true
	})
	mc.symbolCache.Range(func(key, _ interface{}) bool {
		mc.symbolCache.Delete(key)
		return true
	})
	mc.parserCache.Range(func(key, _ interface{}) bool {
		mc.parserCache.Delete(key)
		return true
	})

	// Reset counters
	atomic.StoreInt64(&mc.hits, 0)
	atomic.StoreInt64(&mc.misses, 0)
	atomic.StoreInt64(&mc.evictions, 0)
	atomic.StoreInt64(&mc.totalRequests, 0)
	atomic.StoreInt64(&mc.parserHits, 0)
	atomic.StoreInt64(&mc.contentCount, 0)
	atomic.StoreInt64(&mc.symbolCount, 0)
	atomic.StoreInt64(&mc.parserCount, 0)
	atomic.StoreInt64(&mc.lastCleanup, time.Now().UnixNano())
}

// GetCacheInfo returns cache configuration and status
func (mc *MetricsCache) GetCacheInfo() CacheInfo {
	stats := mc.Stats()
	return CacheInfo{
		MaxEntries:    mc.maxEntries,
		TTL:           time.Duration(mc.ttlNanos),
		EnableContent: mc.enableContent,
		EnableSymbol:  mc.enableSymbol,
		Stats:         stats,
		Status:        getHealthStatus(stats.HitRate),
	}
}

// CacheInfo provides cache information
type CacheInfo struct {
	MaxEntries    int
	TTL           time.Duration
	EnableContent bool
	EnableSymbol  bool
	Stats         CacheStats
	Status        string
}

func getHealthStatus(hitRate float64) string {
	switch {
	case hitRate >= 0.95:
		return "excellent"
	case hitRate >= 0.85:
		return "good"
	case hitRate >= 0.70:
		return "fair"
	default:
		return "poor"
	}
}

// SetMaxEntries updates max entries (no-op for sync.Map, kept for API compatibility)
func (mc *MetricsCache) SetMaxEntries(maxEntries int) {
	mc.maxEntries = maxEntries
	// sync.Map doesn't enforce limits - cleanup handles eviction
}

// UpdateTTL updates TTL and cleans expired entries
func (mc *MetricsCache) UpdateTTL(ttl time.Duration) {
	atomic.StoreInt64(&mc.ttlNanos, ttl.Nanoseconds())
	mc.CleanExpired()
}
