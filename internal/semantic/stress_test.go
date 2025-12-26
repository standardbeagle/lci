package semantic

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSemanticScorerStress tests the semantic scorer under high load
func TestSemanticScorerStress(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Initialize performance scaler for adaptive thresholds
	scaler := newStressTestScaler(t)
	scaler.logScalingFactors(t)

	// Initialize scorer
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()
	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	// Generate test data
	candidates := generateCandidates(1000)

	// Stress test parameters
	numGoroutines := runtime.NumCPU() * 2
	duration := 5 * time.Second
	queries := []string{
		"getUserName",
		"setUserEmail",
		"validatePassword",
		"authenticateUser",
		"updateProfile",
		"deleteAccount",
	}

	// Metrics
	var operations int64
	var totalLatency int64
	var maxLatency int64

	// Start time
	start := time.Now()
	stop := make(chan struct{})

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					// Pick a random query
					query := queries[workerID%len(queries)]

					// Measure operation
					opStart := time.Now()
					results := scorer.ScoreMultiple(query, candidates)
					latency := time.Since(opStart).Nanoseconds()

					// Update metrics
					atomic.AddInt64(&operations, 1)
					atomic.AddInt64(&totalLatency, latency)

					// Track max latency
					for {
						current := atomic.LoadInt64(&maxLatency)
						if latency <= current || atomic.CompareAndSwapInt64(&maxLatency, current, latency) {
							break
						}
					}

					// Note: Empty results are valid when no candidates meet the MinScore threshold
					// This is not an error, but we track it for statistics
					_ = results
				}
			}
		}(i)
	}

	// Run for duration
	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// Calculate metrics
	elapsed := time.Since(start)
	totalOps := atomic.LoadInt64(&operations)
	avgLatency := time.Duration(atomic.LoadInt64(&totalLatency) / totalOps)
	maxLat := time.Duration(atomic.LoadInt64(&maxLatency))
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	// Report results
	t.Logf("Stress Test Results:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Total Operations: %d", totalOps)
	t.Logf("  Operations/sec: %.2f", opsPerSec)
	t.Logf("  Average Latency: %v", avgLatency)
	t.Logf("  Max Latency: %v", maxLat)

	// Note: We no longer track "errors" because empty results are valid

	// Performance assertions - base thresholds scaled for environment
	// Base values assume ideal conditions; scaler adjusts for CI/race detector
	// Note: Max latency is inherently variable due to goroutine scheduling -
	// a single slow operation can spike to 2-3x the average under system load
	baseMaxAvgLatency := 100.0  // milliseconds - average should be stable
	baseMaxMaxLatency := 1000.0 // milliseconds - max allows for scheduler spikes
	baseMinOpsPerSec := 50.0

	// Scale thresholds based on environment (CI, race detector, GOMAXPROCS)
	maxAvgLatency := time.Duration(scaler.scaleDuration(baseMaxAvgLatency)) * time.Millisecond
	maxMaxLatency := time.Duration(scaler.scaleDuration(baseMaxMaxLatency)) * time.Millisecond
	minOpsPerSec := scaler.scaleThroughput(baseMinOpsPerSec)

	t.Logf("Scaled thresholds: maxAvg=%v, maxMax=%v, minOps=%.2f", maxAvgLatency, maxMaxLatency, minOpsPerSec)

	if avgLatency > maxAvgLatency {
		t.Errorf("Average latency too high: %v (max: %v)", avgLatency, maxAvgLatency)
	}

	if maxLat > maxMaxLatency {
		t.Errorf("Max latency too high: %v (max: %v)", maxLat, maxMaxLatency)
	}

	if opsPerSec < minOpsPerSec {
		t.Errorf("Throughput too low: %.2f ops/sec (min: %.0f)", opsPerSec, minOpsPerSec)
	}
}

// TestLRUCacheStress tests the LRU cache under concurrent load
func TestLRUCacheStress(t *testing.T) {
	// Adjust workload for short mode vs full testing
	numGoroutines := runtime.NumCPU() * 2
	opsPerGoroutine := 10000
	minOpsPerSec := 200000.0 // Realistic threshold for concurrent cache

	if testing.Short() {
		// Lighter workload for short mode - still tests concurrency
		numGoroutines = 4
		opsPerGoroutine = 1000
		minOpsPerSec = 100000.0 // Lower threshold for smaller workload
	}

	cache := NewLRUCache(1000)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j%100)

				// 70% reads, 30% writes
				if j%10 < 7 {
					_, _ = cache.Get(key)
				} else {
					value := &normalizedQuery{
						original: key,
						words:    []string{"test"},
						stems:    []string{"test"},
					}
					cache.Set(key, value)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalOps := numGoroutines * opsPerGoroutine
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	t.Logf("LRU Cache Stress Test:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Total Operations: %d", totalOps)
	t.Logf("  Operations/sec: %.2f", opsPerSec)
	t.Logf("  Final Cache Size: %d", cache.Size())

	// Retry throughput check with backoff to handle timing variance
	throughputRetries := 2
	for attempt := 0; attempt < throughputRetries; attempt++ {
		if opsPerSec >= minOpsPerSec {
			break // Success
		}
		if attempt < throughputRetries-1 {
			t.Logf("Cache throughput attempt %d/%d: %.2f ops/sec (threshold: %.0f), retrying...", attempt+1, throughputRetries, opsPerSec, minOpsPerSec)
			time.Sleep(300 * time.Millisecond)
			// Recalculate with fresh timing
			elapsed = time.Since(start)
			opsPerSec = float64(totalOps) / elapsed.Seconds()
			continue
		}
		t.Errorf("Cache throughput too low: %.2f ops/sec (expected >= %.0f)", opsPerSec, minOpsPerSec)
	}
}

// TestMemoryStability tests for memory leaks
func TestMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stability test in short mode")
	}

	// Initialize scorer
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()
	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	candidates := generateCandidates(100)

	// Get initial memory stats
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Run many iterations
	iterations := 10000
	for i := 0; i < iterations; i++ {
		query := fmt.Sprintf("query%d", i%100)
		_ = scorer.ScoreMultiple(query, candidates)

		// Clear cache periodically to test cache eviction
		if i%1000 == 0 {
			scorer.ClearCache()
		}
	}

	// Get final memory stats
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate memory growth
	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	heapGrowthMB := float64(heapGrowth) / (1024 * 1024)

	t.Logf("Memory Stability Test:")
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Initial Heap: %.2f MB", float64(m1.HeapAlloc)/(1024*1024))
	t.Logf("  Final Heap: %.2f MB", float64(m2.HeapAlloc)/(1024*1024))
	t.Logf("  Heap Growth: %.2f MB", heapGrowthMB)
	t.Logf("  GC Runs: %d", m2.NumGC-m1.NumGC)

	// Allow up to 10MB growth (accounting for GC not being perfect)
	if heapGrowthMB > 10 {
		t.Errorf("Excessive memory growth: %.2f MB", heapGrowthMB)
	}
}

// stressTestScaler provides scaling factors for stress tests based on runtime conditions.
// This is a local copy to avoid import cycles with internal/testing package.
type stressTestScaler struct {
	cpuCount       int
	goMaxProcs     int
	isCI           bool
	isRaceDetector bool
}

func newStressTestScaler(t *testing.T) *stressTestScaler {
	return &stressTestScaler{
		cpuCount:       runtime.NumCPU(),
		goMaxProcs:     runtime.GOMAXPROCS(0),
		isCI:           isRunningInCI(),
		isRaceDetector: raceEnabled,
	}
}

// scaleDuration adjusts a duration based on runtime conditions
func (s *stressTestScaler) scaleDuration(base float64) float64 {
	scaled := base

	// Race detector adds 2-10x overhead
	if s.isRaceDetector {
		scaled *= 2.5
	} else if s.isCI {
		scaled *= 1.5
	}

	// GOMAXPROCS=1 means serialized execution
	if s.goMaxProcs == 1 {
		scaled *= 1.5
	}

	return scaled
}

// scaleThroughput adjusts throughput expectations (ops/sec)
func (s *stressTestScaler) scaleThroughput(base float64) float64 {
	scaled := base

	if s.isRaceDetector {
		scaled *= 0.4 // Race detector can reduce throughput by 60%
	} else if s.isCI {
		scaled *= 0.7
	}

	if s.goMaxProcs == 1 && s.cpuCount > 1 {
		scaled *= 0.6
	}

	return scaled
}

// logScalingFactors logs the current scaling factors for debugging
func (s *stressTestScaler) logScalingFactors(t *testing.T) {
	t.Logf("Performance scaling factors:")
	t.Logf("  CPUs: %d, GOMAXPROCS: %d", s.cpuCount, s.goMaxProcs)
	t.Logf("  CI: %v, Race: %v", s.isCI, s.isRaceDetector)
	t.Logf("  Duration scaling: %.2fx", s.scaleDuration(1.0))
	t.Logf("  Throughput scaling: %.2fx", s.scaleThroughput(1.0))
}

// isRunningInCI checks common CI environment variables
func isRunningInCI() bool {
	ciVars := []string{
		"CI", "CONTINUOUS_INTEGRATION", "GITHUB_ACTIONS",
		"GITLAB_CI", "JENKINS", "CIRCLECI", "TRAVIS", "BUILDKITE", "DRONE",
	}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}
