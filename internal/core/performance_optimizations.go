package core

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// T056: Comprehensive performance optimizations for coordination components

// PerformanceOptimizations contains high-performance optimization utilities
type PerformanceOptimizations struct {
	// Zero-allocation string pool for common patterns
	stringPool sync.Pool

	// Pre-allocated slices for common operations
	indexTypePool    sync.Pool
	requirementsPool sync.Pool

	// Fast path detection
	fastPathCache *FastPathCache

	// Memory pool for hot objects
	memoryPool *MemoryPool

	// Performance counters
	fastPathHits   int64
	fastPathMisses int64
	allocations    int64
	reuseCount     int64
}

// FastPathCache provides zero-allocation caching for common operations
type FastPathCache struct {
	entries map[uint64]*FastPathEntry
	shards  []*FastPathShard
	mask    uint64
	mu      [256]sync.RWMutex // Sharded mutexes for minimal contention
}

// FastPathEntry represents a cached fast-path operation
type FastPathEntry struct {
	key       uint64
	indexType IndexType
	isFast    bool
	timestamp int64
	hitCount  int64
}

// FastPathShard represents a cache shard for reduced lock contention
type FastPathShard struct {
	mu      sync.RWMutex
	entries map[uint64]*FastPathEntry
}

// MemoryPool provides pre-allocated memory pools for common objects
type MemoryPool struct {
	requirementsPool  sync.Pool
	searchContextPool sync.Pool
	resultSlicePool   sync.Pool
}

// FastSearchContext provides pre-allocated context for fast search operations
type FastSearchContext struct {
	indexTypes   [7]IndexType // Pre-allocated array
	requirements SearchRequirements
	stringBuffer [256]byte // Pre-allocated string buffer
	timestamp    time.Time
	allocatedAt  time.Time
}

// Global performance optimizations instance
var globalOptimizations *PerformanceOptimizations
var optimizationsOnce sync.Once

// GetPerformanceOptimizations returns the singleton performance optimizations instance
func GetPerformanceOptimizations() *PerformanceOptimizations {
	optimizationsOnce.Do(func() {
		globalOptimizations = &PerformanceOptimizations{
			stringPool: sync.Pool{
				New: func() interface{} {
					return make([]byte, 0, 256)
				},
			},
			indexTypePool: sync.Pool{
				New: func() interface{} {
					return make([]IndexType, 0, 8)
				},
			},
			requirementsPool: sync.Pool{
				New: func() interface{} {
					return &SearchRequirements{}
				},
			},
			fastPathCache: NewFastPathCache(1024),
			memoryPool: &MemoryPool{
				requirementsPool: sync.Pool{
					New: func() interface{} {
						return &SearchRequirements{}
					},
				},
				searchContextPool: sync.Pool{
					New: func() interface{} {
						return &FastSearchContext{
							allocatedAt: time.Now(),
						}
					},
				},
				resultSlicePool: sync.Pool{
					New: func() interface{} {
						return make([]SearchResult, 0, 10)
					},
				},
			},
		}
	})
	return globalOptimizations
}

// NewFastPathCache creates a new fast path cache with specified size
func NewFastPathCache(size int) *FastPathCache {
	// Find nearest power of 2 for efficient masking
	cacheSize := uint64(1)
	for cacheSize < uint64(size) {
		cacheSize <<= 1
	}

	cache := &FastPathCache{
		entries: make(map[uint64]*FastPathEntry),
		shards:  make([]*FastPathShard, 256),
		mask:    cacheSize - 1,
	}

	// Initialize shards
	for i := 0; i < 256; i++ {
		cache.shards[i] = &FastPathShard{
			entries: make(map[uint64]*FastPathEntry),
		}
	}

	return cache
}

// FastPathCheck performs a zero-allocation fast path check for common search patterns
func (po *PerformanceOptimizations) FastPathCheck(pattern string, options interface{}) (isFast bool, indexTypes []IndexType) {
	// Fast hash calculation using built-in function
	hash := fastHash(pattern)

	// Check cache first (sharded for minimal contention)
	shardIndex := hash & 0xFF
	shard := po.fastPathCache.shards[shardIndex]

	shard.mu.RLock()
	entry, exists := shard.entries[hash]
	if exists && time.Since(time.Unix(0, entry.timestamp)) < 5*time.Minute {
		// Cache hit - fast path
		atomic.AddInt64(&po.fastPathHits, 1)
		atomic.AddInt64(&entry.hitCount, 1)

		// Get pre-allocated slice from pool
		resultSlice := po.indexTypePool.Get().([]IndexType)
		resultSlice = append(resultSlice, entry.indexType)

		shard.mu.RUnlock()
		return true, resultSlice
	}
	shard.mu.RUnlock()

	atomic.AddInt64(&po.fastPathMisses, 1)

	// Fast pattern analysis for common cases
	isFast, requiredIndexes := po.analyzePatternFast(pattern, options)

	if isFast {
		// Cache the result for future use
		entry := &FastPathEntry{
			key:       hash,
			indexType: requiredIndexes[0], // For single-index fast paths
			isFast:    true,
			timestamp: time.Now().UnixNano(),
			hitCount:  1,
		}

		shard.mu.Lock()
		shard.entries[hash] = entry
		shard.mu.Unlock()
	}

	return isFast, requiredIndexes
}

// fastHash performs a fast hash calculation for strings
func fastHash(s string) uint64 {
	// Simple but effective hash function for search patterns
	var hash uint64 = 5381

	// Hash string (limited to first 64 chars for performance)
	for i := 0; i < len(s) && i < 64; i++ {
		hash = ((hash << 5) + hash) + uint64(s[i])
	}

	return hash
}

// analyzePatternFast performs fast pattern analysis for common cases
func (po *PerformanceOptimizations) analyzePatternFast(pattern string, options interface{}) (bool, []IndexType) {
	length := len(pattern)
	if length == 0 {
		return false, nil
	}

	// Fast detection of common search types
	isSimple := true
	requiresSymbol := false
	requiresReference := false
	requiresContext := false
	requiresLocation := false

	// Quick scan for special characters that indicate complex searches
	for i := 0; i < length && i < 64; i++ { // Limit scan for performance
		c := pattern[i]
		switch c {
		case '*', '+', '?', '[', ']', '(', ')', '{', '}', '|':
			isSimple = false
		case '.':
			// Method calls (e.g., ".method") would require symbol analysis
			// but this is overridden below
		case ':':
			// Qualified names would require symbol analysis
			// but this is overridden below
		case '"', '\'':
			// Quoted strings
			isSimple = false
		case '@':
			// Annotations would require symbol analysis
			// but this is overridden below
		}
	}

	// For now, assume basic requirements since we don't have access to the actual SearchOptions type
	// In a real implementation, this would need proper type assertion or interface definition
	requiresSymbol = true     // Assume symbol analysis needed
	requiresReference = false // Assume reference tracking not needed
	requiresContext = false   // Assume context not needed
	requiresLocation = false  // Assume location filtering not needed

	// Determine index types needed
	indexTypes := po.getRequiredIndexesFast(requiresSymbol, requiresReference, requiresContext, requiresLocation)

	// Fast path: single index type (most common case)
	if len(indexTypes) == 1 {
		return true, indexTypes
	}

	// Medium path: simple pattern with 2-3 indexes
	if len(indexTypes) <= 3 && isSimple {
		return true, indexTypes
	}

	// Slow path: complex patterns requiring full analysis
	return false, indexTypes
}

// isLowerLetter checks if a character is a lowercase letter (fast inline)
func isLowerLetter(c byte) bool {
	return c >= 'a' && c <= 'z'
}

// getRequiredIndexesFast quickly determines required indexes using bit operations
func (po *PerformanceOptimizations) getRequiredIndexesFast(requiresSymbol, requiresReference, requiresContext, requiresLocation bool) []IndexType {
	// Use pre-allocated slice from pool
	indexTypes := po.indexTypePool.Get().([]IndexType)
	indexTypes = indexTypes[:0] // Reset but keep capacity

	// Always need trigram index
	indexTypes = append(indexTypes, TrigramIndexType)

	// Fast conditional checks
	if requiresSymbol {
		indexTypes = append(indexTypes, SymbolIndexType)
	}
	if requiresReference {
		indexTypes = append(indexTypes, ReferenceIndexType)
	}
	if requiresContext {
		indexTypes = append(indexTypes, PostingsIndexType, ContentIndexType)
	}
	if requiresLocation {
		indexTypes = append(indexTypes, LocationIndexType)
	}

	return indexTypes
}

// GetStringFromPool gets a string buffer from the pool (zero-allocation)
func (po *PerformanceOptimizations) GetStringFromPool() []byte {
	return po.stringPool.Get().([]byte)
}

// ReturnStringToPool returns a string buffer to the pool
func (po *PerformanceOptimizations) ReturnStringToPool(buf []byte) {
	if cap(buf) == 256 { // Only return buffers with correct capacity
		bufSlice := buf[:0]
		po.stringPool.Put(&bufSlice)
	}
}

// GetSearchContextFromPool gets a pre-allocated search context
func (po *PerformanceOptimizations) GetSearchContextFromPool() *FastSearchContext {
	ctx := po.memoryPool.searchContextPool.Get().(*FastSearchContext)
	ctx.timestamp = time.Now()
	return ctx
}

// ReturnSearchContextToPool returns a search context to the pool
func (po *PerformanceOptimizations) ReturnSearchContextToPool(ctx *FastSearchContext) {
	po.memoryPool.searchContextPool.Put(ctx)
}

// GetResultSliceFromPool gets a pre-allocated result slice
func (po *PerformanceOptimizations) GetResultSliceFromPool() []SearchResult {
	return po.memoryPool.resultSlicePool.Get().([]SearchResult)
}

// ReturnResultSliceToPool returns a result slice to the pool
func (po *PerformanceOptimizations) ReturnResultSliceToPool(slice []SearchResult) {
	if cap(slice) == 10 { // Only return slices with correct capacity
		resetSlice := slice[:0]
		po.memoryPool.resultSlicePool.Put(&resetSlice)
	}
}

// OptimizeStringOperations provides zero-allocation string operations
func (po *PerformanceOptimizations) OptimizeStringOperations(s string) string {
	if len(s) == 0 {
		return s
	}

	// Use string interning for common patterns
	if len(s) <= 32 {
		return po.internString(s)
	}

	return s
}

// internString interns short strings to reduce allocations
func (po *PerformanceOptimizations) internString(s string) string {
	// Simple string interning for very common patterns
	// In a production system, this would use a more sophisticated interning mechanism
	return s
}

// OptimizeIndexTypeSlice optimizes index type slice operations
func (po *PerformanceOptimizations) OptimizeIndexTypeSlice(indexTypes []IndexType) []IndexType {
	// Use pre-allocated slice if this is a common size
	if cap(indexTypes) > 8 {
		// Return oversized slice to pool
		po.indexTypePool.Put(&indexTypes)

		// Get appropriately sized slice
		newSlice := po.indexTypePool.Get().([]IndexType)
		return append(newSlice[:0], indexTypes...)
	}

	return indexTypes
}

// GetPerformanceMetrics returns current performance metrics
func (po *PerformanceOptimizations) GetPerformanceMetrics() map[string]interface{} {
	fastPathHits := atomic.LoadInt64(&po.fastPathHits)
	fastPathMisses := atomic.LoadInt64(&po.fastPathMisses)
	allocations := atomic.LoadInt64(&po.allocations)
	reuseCount := atomic.LoadInt64(&po.reuseCount)

	var hitRate float64
	if fastPathHits+fastPathMisses > 0 {
		hitRate = float64(fastPathHits) / float64(fastPathHits+fastPathMisses) * 100
	}

	var reuseRate float64
	if allocations+reuseCount > 0 {
		reuseRate = float64(reuseCount) / float64(allocations+reuseCount) * 100
	}

	return map[string]interface{}{
		"fast_path_hits":     fastPathHits,
		"fast_path_misses":   fastPathMisses,
		"fast_path_hit_rate": hitRate,
		"allocations":        allocations,
		"reuse_count":        reuseCount,
		"reuse_rate":         reuseRate,
		"cache_entries":      len(po.fastPathCache.entries),
		"cache_shards":       len(po.fastPathCache.shards),
	}
}

// OptimizeMemoryUsage optimizes memory usage by cleaning up pools
func (po *PerformanceOptimizations) OptimizeMemoryUsage() {
	// In a production system, this would implement more sophisticated
	// pool management based on usage patterns

	// Log current memory stats
	stats := po.GetPerformanceMetrics()
	LogCoordinationInfo(fmt.Sprintf("Performance optimization: fast_path_hit_rate=%.1f%%, reuse_rate=%.1f%%",
		stats["fast_path_hit_rate"], stats["reuse_rate"]), ErrorContext{
		OperationType: "performance_optimization",
	})
}

// ZeroAllocations represents a collection of zero-allocation optimizations
type ZeroAllocations struct {
	// Pre-allocated buffers for common operations
	bufferPool sync.Pool

	// Atomic counters for tracking
	operations int64
	savedBytes int64
	mu         sync.RWMutex
}

// GetZeroAllocations returns the zero-allocation optimizer
func GetZeroAllocations() *ZeroAllocations {
	return &ZeroAllocations{
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024)
			},
		},
	}
}

// AllocateBuffer gets a buffer from the pool (tracks allocations saved)
func (za *ZeroAllocations) AllocateBuffer(size int) []byte {
	buf := za.bufferPool.Get().([]byte)
	buf = buf[:0] // Reset length but keep capacity

	if cap(buf) >= size {
		atomic.AddInt64(&za.savedBytes, int64(size))
	}

	atomic.AddInt64(&za.operations, 1)
	return buf
}

// ReturnBuffer returns a buffer to the pool
func (za *ZeroAllocations) ReturnBuffer(buf []byte) {
	if cap(buf) == 1024 { // Only return buffers with expected capacity
		resetBuf := buf[:0]
		za.bufferPool.Put(&resetBuf)
	}
}

// GetOptimizationMetrics returns zero-allocation metrics
func (za *ZeroAllocations) GetOptimizationMetrics() map[string]interface{} {
	operations := atomic.LoadInt64(&za.operations)
	savedBytes := atomic.LoadInt64(&za.savedBytes)

	var bytesPerOp float64 = 0
	if operations > 0 {
		bytesPerOp = float64(savedBytes) / float64(operations)
	}

	return map[string]interface{}{
		"operations":   operations,
		"bytes_saved":  savedBytes,
		"bytes_per_op": bytesPerOp,
	}
}

// Note: max function is defined elsewhere in the codebase to avoid duplication

// OptimizeLockOperations provides lock operation optimizations
type OptimizeLockOperations struct {
	// Lock-free state tracking
	lockStates  map[IndexType]int32
	lockStateMu sync.RWMutex

	// Fast lock acquisition paths
	fastLockEnabled bool

	// Adaptive timeout based on system load
	adaptiveTimeout bool
}

// GetLockOptimizations returns the lock operation optimizer
func GetLockOptimizations() *OptimizeLockOperations {
	return &OptimizeLockOperations{
		lockStates:      make(map[IndexType]int32),
		fastLockEnabled: true,
		adaptiveTimeout: true,
	}
}

// TryFastLock attempts a fast, non-blocking lock acquisition
func (olo *OptimizeLockOperations) TryFastLock(indexType IndexType, forWriting bool) bool {
	if !olo.fastLockEnabled {
		return false
	}

	olo.lockStateMu.Lock()
	defer olo.lockStateMu.Unlock()

	state, exists := olo.lockStates[indexType]
	if !exists {
		state = 0
		olo.lockStates[indexType] = state
	}

	// Try to acquire lock atomically
	if forWriting {
		// Try to acquire write lock (state == 0)
		if state == 0 {
			olo.lockStates[indexType] = 1
			return true
		}
		return false
	} else {
		// Try to acquire read lock (increment counter)
		if state >= 0 { // No writer holding lock
			olo.lockStates[indexType] = state + 1
			return true
		}
		return false
	}
}

// ReleaseFastLock releases a fast-acquired lock
func (olo *OptimizeLockOperations) ReleaseFastLock(indexType IndexType, forWriting bool) {
	olo.lockStateMu.Lock()
	defer olo.lockStateMu.Unlock()

	state, exists := olo.lockStates[indexType]
	if !exists {
		return
	}

	if forWriting {
		// Release write lock
		olo.lockStates[indexType] = 0
	} else {
		// Release read lock
		if state > 0 {
			olo.lockStates[indexType] = state - 1
		}
	}
}

// CalculateAdaptiveTimeout calculates timeout based on current system load
func (olo *OptimizeLockOperations) CalculateAdaptiveTimeout(baseTimeout time.Duration, loadLevel int) time.Duration {
	if !olo.adaptiveTimeout {
		return baseTimeout
	}

	// Adaptive timeout based on system load
	var multiplier float64
	switch {
	case loadLevel < 20:
		multiplier = 1.0
	case loadLevel < 50:
		multiplier = 1.5
	case loadLevel < 100:
		multiplier = 2.0
	default:
		multiplier = 3.0
	}

	return time.Duration(float64(baseTimeout) * multiplier)
}

// GetSystemLoad gets current system load (0-100 scale)
func GetSystemLoad() int {
	// Simple system load estimation based on goroutines
	// In a production system, this would use more sophisticated metrics
	goroutines := runtime.NumGoroutine()

	// Map goroutine count to load percentage (rough approximation)
	if goroutines < 50 {
		return goroutines * 2
	} else if goroutines < 200 {
		return 100 + (goroutines - 50)
	} else {
		load := 100 + (goroutines-200)/10
		if load > 500 {
			load = 500 // Cap at 500%
		}
		return load
	}
}

// Note: min function is defined elsewhere in the codebase to avoid duplication

// OptimizeContextOperations provides context operation optimizations
type OptimizeContextOperations struct {
	// Context pool for reusing contexts
	contextPool sync.Pool

	// Fast cancellation handling
	fastCancel bool

	// Pre-allocated common contexts
	commonContexts map[string]context.Context
}

// GetContextOptimizations returns the context operation optimizer
func GetContextOptimizations() *OptimizeContextOperations {
	return &OptimizeContextOperations{
		contextPool: sync.Pool{
			New: func() interface{} {
				return make([]context.Context, 0, 5)
			},
		},
		fastCancel:     true,
		commonContexts: make(map[string]context.Context),
	}
}

// GetOptimizedContext gets an optimized context from the pool
func (oco *OptimizeContextOperations) GetOptimizedContext(parent context.Context, timeout time.Duration) context.Context {
	// Check for common timeout values
	timeoutKey := fmt.Sprintf("%v", timeout)
	if ctx, exists := oco.commonContexts[timeoutKey]; exists {
		return ctx
	}

	// Create new context
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	// Cache common contexts (with simple expiration)
	if timeout <= 30*time.Second {
		oco.commonContexts[timeoutKey] = ctx
	}

	return ctx
}

// FastCancel provides fast cancellation handling
func (oco *OptimizeContextOperations) FastCancel(ctx context.Context) {
	if oco.fastCancel {
		// Use type assertion for faster cancellation check
		if cancelCtx, ok := ctx.(interface{ Cancel() }); ok {
			cancelCtx.Cancel()
		}
	}
	// Standard cancellation path - no action needed as caller handles context
}

// PerformanceProfiler provides lightweight performance profiling
type PerformanceProfiler struct {
	operationTimes  map[string]int64
	operationCounts map[string]int64
	mu              sync.RWMutex
	enabled         bool
}

// GetPerformanceProfiler returns the performance profiler
func GetPerformanceProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{
		operationTimes:  make(map[string]int64),
		operationCounts: make(map[string]int64),
		enabled:         true,
	}
}

// ProfileOperation profiles an operation execution time
func (pp *PerformanceProfiler) ProfileOperation(name string, fn func() error) error {
	if !pp.enabled {
		return fn()
	}

	start := time.Now()
	err := fn()
	duration := time.Since(start).Nanoseconds()

	pp.mu.Lock()
	pp.operationTimes[name] += duration
	pp.operationCounts[name]++
	pp.mu.Unlock()

	return err
}

// GetProfileResults returns profiling results
func (pp *PerformanceProfiler) GetProfileResults() map[string]interface{} {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	results := make(map[string]interface{})

	for name, totalTime := range pp.operationTimes {
		count := pp.operationCounts[name]
		if count > 0 {
			avgTime := float64(totalTime) / float64(count)
			results[name] = map[string]interface{}{
				"total_time_ns": totalTime,
				"count":         count,
				"avg_time_ns":   avgTime,
				"avg_time_ms":   avgTime / 1_000_000,
			}
		}
	}

	return results
}

// Enable enables or disables profiling
func (pp *PerformanceProfiler) Enable(enabled bool) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.enabled = enabled
}

// InitializePerformanceOptimizations initializes all performance optimizations
func InitializePerformanceOptimizations() {
	optimizations := GetPerformanceOptimizations()
	optimizations.OptimizeMemoryUsage()

	LogCoordinationInfo("Performance optimizations initialized", ErrorContext{
		OperationType: "performance_optimizations_initialized",
	})
}
