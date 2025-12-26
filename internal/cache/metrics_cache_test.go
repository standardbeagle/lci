package cache

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestMetricsCache_Creation tests the metrics cache creation.
func TestMetricsCache_Creation(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	if cache == nil {
		t.Fatal("NewMetricsCache returned nil")
	}

	info := cache.GetCacheInfo()
	if info.MaxEntries != config.MaxContentEntries {
		t.Errorf("Expected max entries %d, got %d", config.MaxContentEntries, info.MaxEntries)
	}

	if info.TTL != config.TTL {
		t.Errorf("Expected TTL %v, got %v", config.TTL, info.TTL)
	}

	if !info.EnableContent || !info.EnableSymbol {
		t.Error("Expected both content and symbol caching to be enabled")
	}
}

// TestMetricsCache_DefaultConfig tests the metrics cache default config.
func TestMetricsCache_DefaultConfig(t *testing.T) {
	config := DefaultCacheConfig()

	// Validate Phase 5 optimized defaults from assumption testing
	if config.MaxContentEntries != DefaultMaxContentEntries {
		t.Errorf("Expected default max content entries %d, got %d", DefaultMaxContentEntries, config.MaxContentEntries)
	}

	if config.MaxSymbolEntries != DefaultMaxSymbolEntries {
		t.Errorf("Expected default max symbol entries %d, got %d", DefaultMaxSymbolEntries, config.MaxSymbolEntries)
	}

	if config.TTL != DefaultTTL {
		t.Errorf("Expected default TTL %v, got %v", DefaultTTL, config.TTL)
	}

	if !config.EnableContent || !config.EnableSymbol {
		t.Error("Expected both caching strategies enabled by default")
	}
}

// TestMetricsCache_BasicOperations tests the metrics cache basic operations.
func TestMetricsCache_BasicOperations(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Test data
	content := []byte("function test() { return 42; }")
	fileID := 1
	symbolName := "test"
	metrics := map[string]interface{}{
		"complexity": 1.0,
		"lines":      3,
	}

	// Test miss
	result := cache.Get(content, fileID, symbolName)
	if result != nil {
		t.Error("Expected cache miss, got hit")
	}

	// Test put
	cache.Put(content, fileID, symbolName, metrics)

	// Test hit
	result = cache.Get(content, fileID, symbolName)
	if result == nil {
		t.Error("Expected cache hit, got miss")
	}

	// Validate returned data
	returnedMetrics, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Returned data is not the expected type")
	}

	if returnedMetrics["complexity"] != metrics["complexity"] {
		t.Error("Returned metrics don't match stored metrics")
	}
}

// TestMetricsCache_DualCacheStrategy tests the metrics cache dual cache strategy.
func TestMetricsCache_DualCacheStrategy(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	content := []byte("function test() { return 42; }")
	fileID := 1
	symbolName := "test"
	metrics := map[string]interface{}{"value": "test"}

	// Store in cache
	cache.Put(content, fileID, symbolName, metrics)

	// Test content-based retrieval
	result1 := cache.Get(content, fileID, symbolName)
	if result1 == nil {
		t.Error("Content-based cache retrieval failed")
	}

	// Test symbol-based retrieval (same fileID + symbolName, different content)
	differentContent := []byte("function test() { return 43; }") // Different content
	result2 := cache.Get(differentContent, fileID, symbolName)
	if result2 == nil {
		t.Error("Symbol-based cache retrieval failed")
	}

	// Both should return the same cached data
	if fmt.Sprintf("%v", result1) != fmt.Sprintf("%v", result2) {
		t.Error("Dual cache strategy returned different results")
	}
}

// TestMetricsCache_TTLExpiration tests the metrics cache TTL expiration.
func TestMetricsCache_TTLExpiration(t *testing.T) {
	config := CacheConfig{
		MaxContentEntries: 100,
		MaxSymbolEntries:  100,
		TTL:               50 * time.Millisecond, // Very short TTL for testing
		EnableContent:     true,
		EnableSymbol:      true,
	}
	cache := NewMetricsCache(config)

	content := []byte("function test() { return 42; }")
	fileID := 1
	symbolName := "test"
	metrics := map[string]interface{}{"value": "test"}

	// Store in cache
	cache.Put(content, fileID, symbolName, metrics)

	// Immediate retrieval should work
	result := cache.Get(content, fileID, symbolName)
	if result == nil {
		t.Error("Immediate retrieval failed")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should be expired now (Get returns nil and deletes lazily)
	result = cache.Get(content, fileID, symbolName)
	if result != nil {
		t.Error("Expected expired entry, got hit")
	}

	// Verify miss was recorded
	stats := cache.Stats()
	if stats.Misses == 0 {
		t.Error("Expected misses > 0 after expired entry access")
	}
}

// TestMetricsCache_SizeEviction tests cache eviction when size limit is exceeded.
func TestMetricsCache_SizeEviction(t *testing.T) {
	config := CacheConfig{
		MaxContentEntries: 3, // Small cache for testing eviction
		MaxSymbolEntries:  3,
		TTL:               1 * time.Hour,
		EnableContent:     true,
		EnableSymbol:      true,
	}
	cache := NewMetricsCache(config)

	// Fill cache beyond capacity - should trigger eviction
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("function test%d() { return %d; }", i, i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
		time.Sleep(time.Millisecond) // Ensure different timestamps for eviction order
	}

	// Check that we have evictions
	stats := cache.Stats()
	t.Logf("Cache stats: entries=%d, evictions=%d", stats.TotalEntries, stats.Evictions)

	// Should have some evictions since we added 5 items to a cache of size 3
	if stats.Evictions == 0 {
		t.Error("Expected evictions > 0 when exceeding cache capacity")
	}

	// Latest entries should still be accessible
	content4 := []byte("function test4() { return 4; }")
	result4 := cache.Get(content4, 4, "test4")
	if result4 == nil {
		t.Error("Most recent entry should still be in cache")
	}
}

// TestMetricsCache_ConcurrentAccess tests the metrics cache concurrent access.
func TestMetricsCache_ConcurrentAccess(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	numGoroutines := runtime.NumCPU() * 2
	operationsPerGoroutine := 1000

	var wg sync.WaitGroup
	start := time.Now()

	// Launch concurrent goroutines (similar to assumption testing)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				symbolName := fmt.Sprintf("symbol_%d_%d", goroutineID, j%20) // 20 unique symbols per goroutine
				content := []byte(fmt.Sprintf("function %s() { return %d; }", symbolName, j))
				fileID := goroutineID

				// Try cache first
				result := cache.Get(content, fileID, symbolName)
				if result == nil {
					// Cache miss - store result
					metrics := map[string]interface{}{
						"complexity": float64(j % 10),
						"goroutine":  goroutineID,
						"iteration":  j,
					}
					cache.Put(content, fileID, symbolName, metrics)
				}
			}
		}(i)
	}

	// Wait for all goroutines
	wg.Wait()
	duration := time.Since(start)

	// Analyze results
	stats := cache.Stats()
	totalOperations := int(stats.TotalRequests)
	operationsPerSecond := float64(totalOperations) / duration.Seconds()

	t.Logf("Concurrent test results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Total operations: %d", totalOperations)
	t.Logf("  Operations/second: %.0f", operationsPerSecond)
	t.Logf("  Hit rate: %.2f%%", stats.HitRate*100)
	t.Logf("  Cache entries: %d", stats.TotalEntries)

	// Validate correctness, not performance (performance varies by system load)
	// Hit rate should be reasonable with concurrent access
	if stats.HitRate < 0.3 { // At least 30% hit rate expected
		t.Errorf("Hit rate too low: %.2f%%", stats.HitRate*100)
	}

	// Should have some cache entries
	if stats.TotalEntries == 0 {
		t.Error("No cache entries after concurrent test")
	}

	// Total operations should match expected
	expectedOps := numGoroutines * operationsPerGoroutine
	if totalOperations < expectedOps/2 {
		t.Errorf("Too few operations recorded: %d (expected ~%d)", totalOperations, expectedOps)
	}
}

// TestMetricsCache_Statistics tests the metrics cache statistics.
func TestMetricsCache_Statistics(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Add some test data
	for i := 0; i < 10; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
	}

	// Generate some hits and misses
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Get(content, i, fmt.Sprintf("test%d", i)) // Should be hits
	}

	for i := 10; i < 15; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Get(content, i, fmt.Sprintf("test%d", i)) // Should be misses
	}

	stats := cache.Stats()

	// Validate statistics
	if stats.Hits != 5 {
		t.Errorf("Expected 5 hits, got %d", stats.Hits)
	}

	if stats.Misses != 5 {
		t.Errorf("Expected 5 misses, got %d", stats.Misses)
	}

	if stats.TotalRequests != 10 {
		t.Errorf("Expected 10 total requests, got %d", stats.TotalRequests)
	}

	expectedHitRate := 0.5 // 5 hits out of 10 requests
	if stats.HitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.2f, got %.2f", expectedHitRate, stats.HitRate)
	}

	if stats.TotalEntries != 20 { // Dual cache: 10 content + 10 symbol entries
		t.Errorf("Expected 20 total entries (dual cache), got %d", stats.TotalEntries)
	}

	// Memory estimation should be reasonable
	expectedMemoryKB := float64(stats.TotalEntries) * EstimatedBytesPerEntry / 1024 // Based on assumption testing
	if stats.EstimatedMemoryKB != expectedMemoryKB {
		t.Errorf("Expected memory estimate %.2f KB, got %.2f KB", expectedMemoryKB, stats.EstimatedMemoryKB)
	}
}

// TestMetricsCache_CleanExpired tests the metrics cache clean expired.
func TestMetricsCache_CleanExpired(t *testing.T) {
	config := CacheConfig{
		MaxContentEntries: 100,
		MaxSymbolEntries:  100,
		TTL:               50 * time.Millisecond,
		EnableContent:     true,
		EnableSymbol:      true,
	}
	cache := NewMetricsCache(config)

	// Add entries
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Manual cleanup
	cleaned := cache.CleanExpired()

	if cleaned == 0 {
		t.Error("Expected some entries to be cleaned")
	}

	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("Expected 0 entries after cleanup, got %d", stats.TotalEntries)
	}
}

// TestMetricsCache_Clear tests the metrics cache clear.
func TestMetricsCache_Clear(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Add test data
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
		cache.Get(content, i, fmt.Sprintf("test%d", i)) // Generate some hits
	}

	// Verify data exists
	statsBefore := cache.Stats()
	if statsBefore.TotalEntries == 0 || statsBefore.Hits == 0 {
		t.Error("Test data not properly added")
	}

	// Clear cache
	cache.Clear()

	// Verify everything cleared
	statsAfter := cache.Stats()
	if statsAfter.TotalEntries != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", statsAfter.TotalEntries)
	}

	if statsAfter.Hits != 0 || statsAfter.Misses != 0 || statsAfter.TotalRequests != 0 {
		t.Error("Statistics not reset after clear")
	}
}

// TestMetricsCache_ConfigurationUpdates tests the metrics cache configuration updates.
func TestMetricsCache_ConfigurationUpdates(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Add some entries
	for i := 0; i < 50; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
	}

	stats := cache.Stats()
	t.Logf("Initial entries: %d", stats.TotalEntries)

	// SetMaxEntries updates the limit for future inserts (sync.Map doesn't retroactively evict)
	cache.SetMaxEntries(25)

	// Update TTL
	newTTL := 30 * time.Minute
	cache.UpdateTTL(newTTL)

	info := cache.GetCacheInfo()
	if info.TTL != newTTL {
		t.Errorf("Expected TTL %v, got %v", newTTL, info.TTL)
	}

	// MaxEntries should be updated
	if info.MaxEntries != 25 {
		t.Errorf("Expected max entries 25, got %d", info.MaxEntries)
	}
}

// TestMetricsCache_HealthStatus tests the metrics cache health status.
func TestMetricsCache_HealthStatus(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Generate excellent hit rate (>95%)
	for i := 0; i < 10; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
	}

	// Generate mostly hits
	for i := 0; i < 100; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i%10))
		cache.Get(content, i%10, fmt.Sprintf("test%d", i%10))
	}

	info := cache.GetCacheInfo()
	if info.Status != "excellent" {
		t.Errorf("Expected excellent status with high hit rate, got %s (hit rate: %.2f%%)",
			info.Status, info.Stats.HitRate*100)
	}
}

// TestMetricsCache_MemoryEstimation tests the metrics cache memory estimation.
func TestMetricsCache_MemoryEstimation(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Add known number of entries
	numEntries := 50
	for i := 0; i < numEntries; i++ {
		content := []byte(fmt.Sprintf("function test%d() { return %d; }", i, i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{
			"complexity": float64(i),
			"lines":      i * 2,
			"id":         i,
		})
	}

	stats := cache.Stats()

	// Memory estimation based on assumption testing
	expectedMemoryKB := float64(stats.TotalEntries) * EstimatedBytesPerEntry / 1024

	if stats.EstimatedMemoryKB != expectedMemoryKB {
		t.Errorf("Memory estimation mismatch: expected %.2f KB, got %.2f KB",
			expectedMemoryKB, stats.EstimatedMemoryKB)
	}

	// Should be reasonable for the number of entries
	if stats.EstimatedMemoryKB <= 0 || stats.EstimatedMemoryKB > 100 {
		t.Errorf("Unreasonable memory estimation: %.2f KB for %d entries",
			stats.EstimatedMemoryKB, stats.TotalEntries)
	}

	t.Logf("Memory usage for %d entries: %.2f KB (%.2f bytes per entry)",
		stats.TotalEntries, stats.EstimatedMemoryKB, EstimatedBytesPerEntry)
}

func BenchmarkMetricsCache_Get(b *testing.B) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Pre-populate cache
	content := []byte("function benchmark() { return 42; }")
	cache.Put(content, 1, "benchmark", map[string]interface{}{"test": true})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(content, 1, "benchmark")
	}
}

func BenchmarkMetricsCache_Put(b *testing.B) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	content := []byte("function benchmark() { return 42; }")
	metrics := map[string]interface{}{"test": true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(content, i, fmt.Sprintf("benchmark_%d", i), metrics)
	}
}

func BenchmarkMetricsCache_ConcurrentAccess(b *testing.B) {
	config := DefaultCacheConfig()
	cache := NewMetricsCache(config)

	// Pre-populate with some data
	for i := 0; i < 100; i++ {
		content := []byte(fmt.Sprintf("function test%d() {}", i))
		cache.Put(content, i, fmt.Sprintf("test%d", i), map[string]interface{}{"id": i})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			symbolName := fmt.Sprintf("test%d", i%100)
			content := []byte(fmt.Sprintf("function %s() {}", symbolName))

			result := cache.Get(content, i%100, symbolName)
			if result == nil {
				cache.Put(content, i%100, symbolName, map[string]interface{}{"id": i})
			}
			i++
		}
	})
}
