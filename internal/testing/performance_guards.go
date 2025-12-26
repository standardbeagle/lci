package testing

import (
	"fmt"
	"runtime"
	stdStrings "strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Performance Guard Types
// ============================================================================

// PerformanceGuard monitors and enforces performance constraints
type PerformanceGuard struct {
	t            *testing.T
	scaler       *PerformanceScaler
	thresholds   map[string]PerformanceThreshold
	measurements map[string][]time.Duration
	mu           sync.Mutex
}

// PerformanceThreshold defines acceptable performance bounds
type PerformanceThreshold struct {
	Name           string
	MaxDuration    time.Duration
	MaxAllocations int64         // Maximum heap allocations
	MaxAllocBytes  int64         // Maximum allocated bytes
	P50Threshold   time.Duration // 50th percentile threshold
	P95Threshold   time.Duration // 95th percentile threshold
	P99Threshold   time.Duration // 99th percentile threshold
}

// PerformanceMeasurement holds a single measurement
type PerformanceMeasurement struct {
	Name        string
	Duration    time.Duration
	Allocations int64
	AllocBytes  int64
	Timestamp   time.Time
}

// PerformanceResult holds the result of a performance check
type PerformanceResult struct {
	Name        string
	Passed      bool
	Duration    time.Duration
	Threshold   time.Duration
	Allocations int64
	MaxAllocs   int64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Violations  []string
}

// ============================================================================
// Guard Creation
// ============================================================================

// NewPerformanceGuard creates a new performance guard
func NewPerformanceGuard(t *testing.T) *PerformanceGuard {
	return &PerformanceGuard{
		t:            t,
		scaler:       NewPerformanceScaler(t),
		thresholds:   make(map[string]PerformanceThreshold),
		measurements: make(map[string][]time.Duration),
	}
}

// WithThreshold adds a performance threshold
func (g *PerformanceGuard) WithThreshold(name string, threshold PerformanceThreshold) *PerformanceGuard {
	threshold.Name = name
	g.thresholds[name] = threshold
	return g
}

// WithSearchThreshold adds standard search performance thresholds
func (g *PerformanceGuard) WithSearchThreshold() *PerformanceGuard {
	return g.WithThreshold("search", PerformanceThreshold{
		MaxDuration:  time.Duration(float64(5*time.Millisecond) * g.scaler.ScaleDuration(1.0)),
		P50Threshold: time.Duration(float64(2*time.Millisecond) * g.scaler.ScaleDuration(1.0)),
		P95Threshold: time.Duration(float64(5*time.Millisecond) * g.scaler.ScaleDuration(1.0)),
		P99Threshold: time.Duration(float64(10*time.Millisecond) * g.scaler.ScaleDuration(1.0)),
	})
}

// WithIndexingThreshold adds standard indexing performance thresholds
func (g *PerformanceGuard) WithIndexingThreshold(filesPerSecond int) *PerformanceGuard {
	// Calculate max duration per file based on target files/sec
	maxPerFile := time.Second / time.Duration(filesPerSecond)
	scaled := time.Duration(float64(maxPerFile) * g.scaler.ScaleDuration(1.0))

	return g.WithThreshold("indexing", PerformanceThreshold{
		MaxDuration:  scaled,
		P50Threshold: scaled / 2,
		P95Threshold: scaled,
		P99Threshold: scaled * 2,
	})
}

// ============================================================================
// Measurement Functions
// ============================================================================

// Measure runs a function and records its performance
func (g *PerformanceGuard) Measure(name string, fn func()) PerformanceMeasurement {
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	start := time.Now()
	fn()
	duration := time.Since(start)

	runtime.ReadMemStats(&memAfter)

	measurement := PerformanceMeasurement{
		Name:        name,
		Duration:    duration,
		Allocations: int64(memAfter.Mallocs - memBefore.Mallocs),
		AllocBytes:  int64(memAfter.TotalAlloc - memBefore.TotalAlloc),
		Timestamp:   time.Now(),
	}

	g.mu.Lock()
	g.measurements[name] = append(g.measurements[name], duration)
	g.mu.Unlock()

	return measurement
}

// MeasureN runs a function N times and collects measurements
func (g *PerformanceGuard) MeasureN(name string, n int, fn func()) []PerformanceMeasurement {
	measurements := make([]PerformanceMeasurement, n)

	// Warm up
	for i := 0; i < min(3, n/10+1); i++ {
		fn()
	}

	// Actual measurements
	for i := 0; i < n; i++ {
		measurements[i] = g.Measure(name, fn)
	}

	return measurements
}

// ============================================================================
// Checking Functions
// ============================================================================

// Check verifies that measurements meet the threshold
func (g *PerformanceGuard) Check(name string) PerformanceResult {
	g.mu.Lock()
	durations := g.measurements[name]
	g.mu.Unlock()

	threshold, hasThreshold := g.thresholds[name]
	if !hasThreshold {
		return PerformanceResult{
			Name:   name,
			Passed: true,
		}
	}

	result := PerformanceResult{
		Name:      name,
		Threshold: threshold.MaxDuration,
		MaxAllocs: threshold.MaxAllocations,
	}

	if len(durations) == 0 {
		result.Violations = append(result.Violations, "No measurements recorded")
		return result
	}

	// Calculate statistics
	result.Duration = average(durations)
	result.P50 = percentile(durations, 50)
	result.P95 = percentile(durations, 95)
	result.P99 = percentile(durations, 99)

	// Check violations
	if threshold.MaxDuration > 0 && result.Duration > threshold.MaxDuration {
		result.Violations = append(result.Violations,
			fmt.Sprintf("Average duration %v exceeds max %v", result.Duration, threshold.MaxDuration))
	}

	if threshold.P50Threshold > 0 && result.P50 > threshold.P50Threshold {
		result.Violations = append(result.Violations,
			fmt.Sprintf("P50 %v exceeds threshold %v", result.P50, threshold.P50Threshold))
	}

	if threshold.P95Threshold > 0 && result.P95 > threshold.P95Threshold {
		result.Violations = append(result.Violations,
			fmt.Sprintf("P95 %v exceeds threshold %v", result.P95, threshold.P95Threshold))
	}

	if threshold.P99Threshold > 0 && result.P99 > threshold.P99Threshold {
		result.Violations = append(result.Violations,
			fmt.Sprintf("P99 %v exceeds threshold %v", result.P99, threshold.P99Threshold))
	}

	result.Passed = len(result.Violations) == 0
	return result
}

// AssertPassed fails the test if performance checks failed
func (g *PerformanceGuard) AssertPassed(name string) {
	result := g.Check(name)
	if !result.Passed {
		g.t.Errorf("Performance check failed for %s:\n%s", name, formatViolations(result.Violations))
	}
}

// AssertAllPassed fails the test if any performance check failed
func (g *PerformanceGuard) AssertAllPassed() {
	for name := range g.thresholds {
		g.AssertPassed(name)
	}
}

// ============================================================================
// Performance Regression Detection
// ============================================================================

// RegressionGuard detects performance regressions by comparing to baselines
type RegressionGuard struct {
	baselines map[string]time.Duration
	current   map[string]time.Duration
	maxDelta  float64 // Maximum allowed regression (e.g., 0.1 = 10% slower)
	mu        sync.Mutex
}

// NewRegressionGuard creates a new regression guard
func NewRegressionGuard(maxDeltaPct float64) *RegressionGuard {
	return &RegressionGuard{
		baselines: make(map[string]time.Duration),
		current:   make(map[string]time.Duration),
		maxDelta:  maxDeltaPct / 100.0,
	}
}

// SetBaseline sets a baseline duration for a measurement
func (g *RegressionGuard) SetBaseline(name string, duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.baselines[name] = duration
}

// RecordMeasurement records a current measurement
func (g *RegressionGuard) RecordMeasurement(name string, duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.current[name] = duration
}

// CheckRegression checks if there's a regression for a specific measurement
func (g *RegressionGuard) CheckRegression(name string) (hasRegression bool, delta float64) {
	g.mu.Lock()
	baseline, hasBaseline := g.baselines[name]
	current, hasCurrent := g.current[name]
	g.mu.Unlock()

	if !hasBaseline || !hasCurrent {
		return false, 0
	}

	if baseline == 0 {
		return false, 0
	}

	delta = float64(current-baseline) / float64(baseline)
	return delta > g.maxDelta, delta
}

// CheckAllRegressions checks all measurements for regressions
func (g *RegressionGuard) CheckAllRegressions() map[string]float64 {
	regressions := make(map[string]float64)

	g.mu.Lock()
	names := make([]string, 0, len(g.baselines))
	for name := range g.baselines {
		names = append(names, name)
	}
	g.mu.Unlock()

	for _, name := range names {
		if hasRegression, delta := g.CheckRegression(name); hasRegression {
			regressions[name] = delta
		}
	}

	return regressions
}

// AssertNoRegressions fails the test if any regressions are detected
func (g *RegressionGuard) AssertNoRegressions(t *testing.T) {
	regressions := g.CheckAllRegressions()

	if len(regressions) > 0 {
		var msg stdStrings.Builder
		msg.WriteString("Performance regressions detected:\n")
		for name, delta := range regressions {
			msg.WriteString(fmt.Sprintf("  - %s: %.1f%% slower than baseline\n", name, delta*100))
		}
		t.Error(msg.String())
	}
}

// ============================================================================
// Memory Guards
// ============================================================================

// MemoryGuard monitors memory usage during tests
type MemoryGuard struct {
	t           *testing.T
	maxHeapMB   int64
	maxAllocMB  int64
	checkpoints []runtime.MemStats
	mu          sync.Mutex
}

// NewMemoryGuard creates a new memory guard
func NewMemoryGuard(t *testing.T) *MemoryGuard {
	return &MemoryGuard{
		t:           t,
		maxHeapMB:   100, // Default 100MB max heap
		maxAllocMB:  500, // Default 500MB total allocations
		checkpoints: make([]runtime.MemStats, 0, 10),
	}
}

// WithMaxHeapMB sets the maximum heap size in MB
func (g *MemoryGuard) WithMaxHeapMB(mb int64) *MemoryGuard {
	g.maxHeapMB = mb
	return g
}

// WithMaxAllocMB sets the maximum total allocations in MB
func (g *MemoryGuard) WithMaxAllocMB(mb int64) *MemoryGuard {
	g.maxAllocMB = mb
	return g
}

// Checkpoint records current memory state
func (g *MemoryGuard) Checkpoint() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	g.mu.Lock()
	g.checkpoints = append(g.checkpoints, m)
	g.mu.Unlock()
}

// Check verifies memory usage is within bounds
func (g *MemoryGuard) Check() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	heapMB := int64(m.HeapAlloc) / (1024 * 1024)
	allocMB := int64(m.TotalAlloc) / (1024 * 1024)

	if heapMB > g.maxHeapMB {
		return fmt.Errorf("heap usage %dMB exceeds max %dMB", heapMB, g.maxHeapMB)
	}

	if allocMB > g.maxAllocMB {
		return fmt.Errorf("total allocations %dMB exceeds max %dMB", allocMB, g.maxAllocMB)
	}

	return nil
}

// AssertWithinBounds fails the test if memory exceeds bounds
func (g *MemoryGuard) AssertWithinBounds() {
	err := g.Check()
	assert.NoError(g.t, err, "Memory guard check failed")
}

// GetMemoryGrowth returns the memory growth between first and last checkpoint
func (g *MemoryGuard) GetMemoryGrowth() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.checkpoints) < 2 {
		return 0
	}

	first := g.checkpoints[0]
	last := g.checkpoints[len(g.checkpoints)-1]

	return int64(last.HeapAlloc) - int64(first.HeapAlloc)
}

// ============================================================================
// Throughput Guards
// ============================================================================

// ThroughputGuard monitors operation throughput
type ThroughputGuard struct {
	t               *testing.T
	minOpsPerSec    float64
	targetOpsPerSec float64
	measurements    []float64
	mu              sync.Mutex
}

// NewThroughputGuard creates a new throughput guard
func NewThroughputGuard(t *testing.T, minOpsPerSec float64) *ThroughputGuard {
	return &ThroughputGuard{
		t:               t,
		minOpsPerSec:    minOpsPerSec,
		targetOpsPerSec: minOpsPerSec * 1.5, // Target is 50% above minimum
		measurements:    make([]float64, 0, 100),
	}
}

// RecordOps records operations completed in a duration
func (g *ThroughputGuard) RecordOps(ops int, duration time.Duration) {
	opsPerSec := float64(ops) / duration.Seconds()

	g.mu.Lock()
	g.measurements = append(g.measurements, opsPerSec)
	g.mu.Unlock()
}

// MeasureOps measures the throughput of an operation
func (g *ThroughputGuard) MeasureOps(ops int, fn func()) float64 {
	start := time.Now()
	fn()
	duration := time.Since(start)

	opsPerSec := float64(ops) / duration.Seconds()
	g.RecordOps(ops, duration)

	return opsPerSec
}

// GetAverageThroughput returns the average throughput
func (g *ThroughputGuard) GetAverageThroughput() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.measurements) == 0 {
		return 0
	}

	var sum float64
	for _, m := range g.measurements {
		sum += m
	}
	return sum / float64(len(g.measurements))
}

// AssertMinThroughput fails if throughput is below minimum
func (g *ThroughputGuard) AssertMinThroughput() {
	avg := g.GetAverageThroughput()
	assert.GreaterOrEqual(g.t, avg, g.minOpsPerSec,
		"Throughput %.2f ops/sec below minimum %.2f ops/sec", avg, g.minOpsPerSec)
}

// ============================================================================
// Helper Functions
// ============================================================================

func average(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Make a copy and sort
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sortDurations(sorted)

	// Calculate index
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}

func sortDurations(durations []time.Duration) {
	// Simple bubble sort (fine for small slices)
	for i := 0; i < len(durations)-1; i++ {
		for j := 0; j < len(durations)-i-1; j++ {
			if durations[j] > durations[j+1] {
				durations[j], durations[j+1] = durations[j+1], durations[j]
			}
		}
	}
}

func formatViolations(violations []string) string {
	var builder stdStrings.Builder
	for _, v := range violations {
		builder.WriteString("  - ")
		builder.WriteString(v)
		builder.WriteString("\n")
	}
	return builder.String()
}

// ============================================================================
// Standard Performance Test Helpers
// ============================================================================

// AssertSearchPerformance validates search meets standard thresholds
func AssertSearchPerformance(t *testing.T, name string, fn func()) {
	guard := NewPerformanceGuard(t).WithSearchThreshold()

	guard.MeasureN("search", 100, fn)
	guard.AssertPassed("search")
}

// AssertIndexingPerformance validates indexing meets standard thresholds
func AssertIndexingPerformance(t *testing.T, name string, filesPerSecond int, fn func()) {
	guard := NewPerformanceGuard(t).WithIndexingThreshold(filesPerSecond)

	guard.MeasureN("indexing", 10, fn)
	guard.AssertPassed("indexing")
}

// AssertMemoryBounds validates memory usage
func AssertMemoryBounds(t *testing.T, maxHeapMB int64, fn func()) {
	guard := NewMemoryGuard(t).WithMaxHeapMB(maxHeapMB)
	guard.Checkpoint()

	fn()

	guard.Checkpoint()
	guard.AssertWithinBounds()

	growth := guard.GetMemoryGrowth() / (1024 * 1024)
	t.Logf("Memory growth: %d MB", growth)
}

// AssertThroughput validates operation throughput
func AssertThroughput(t *testing.T, minOpsPerSec float64, ops int, fn func()) {
	guard := NewThroughputGuard(t, minOpsPerSec)

	throughput := guard.MeasureOps(ops, fn)
	t.Logf("Throughput: %.2f ops/sec", throughput)

	guard.AssertMinThroughput()
}
