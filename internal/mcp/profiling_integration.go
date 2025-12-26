package mcp

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

// ProfilingMetrics tracks performance metrics for MCP operations
type ProfilingMetrics struct {
	mu sync.RWMutex

	// Operation counts
	SearchOperations        int64
	CodebaseIntelligenceOps int64
	SemanticScoringOps      int64

	// Timing metrics
	TotalSearchTime          time.Duration
	TotalCodebaseIntelTime   time.Duration
	TotalSemanticScoringTime time.Duration

	// Memory metrics
	PeakMemoryUsage    uint64
	CurrentMemoryUsage uint64

	// Cache metrics
	SemanticCacheHits   int64
	SemanticCacheMisses int64

	// Error counts
	SearchErrors          int64
	CodebaseIntelErrors   int64
	SemanticScoringErrors int64

	// Distribution metrics
	SearchLatencyHistogram map[string]int64 // Buckets: <10ms, 10-50ms, 50-100ms, 100-500ms, >500ms
	ResultSizeHistogram    map[string]int64 // Buckets: 0, 1-10, 11-50, 51-100, >100
}

// NewProfilingMetrics creates a new metrics tracker
func NewProfilingMetrics() *ProfilingMetrics {
	return &ProfilingMetrics{
		SearchLatencyHistogram: map[string]int64{
			"<10ms":     0,
			"10-50ms":   0,
			"50-100ms":  0,
			"100-500ms": 0,
			">500ms":    0,
		},
		ResultSizeHistogram: map[string]int64{
			"0":      0,
			"1-10":   0,
			"11-50":  0,
			"51-100": 0,
			">100":   0,
		},
	}
}

// RecordSearch records metrics for a search operation
func (pm *ProfilingMetrics) RecordSearch(duration time.Duration, resultCount int, err error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.SearchOperations++
	pm.TotalSearchTime += duration

	if err != nil {
		pm.SearchErrors++
	}

	// Update latency histogram
	switch {
	case duration < 10*time.Millisecond:
		pm.SearchLatencyHistogram["<10ms"]++
	case duration < 50*time.Millisecond:
		pm.SearchLatencyHistogram["10-50ms"]++
	case duration < 100*time.Millisecond:
		pm.SearchLatencyHistogram["50-100ms"]++
	case duration < 500*time.Millisecond:
		pm.SearchLatencyHistogram["100-500ms"]++
	default:
		pm.SearchLatencyHistogram[">500ms"]++
	}

	// Update result size histogram
	switch {
	case resultCount == 0:
		pm.ResultSizeHistogram["0"]++
	case resultCount <= 10:
		pm.ResultSizeHistogram["1-10"]++
	case resultCount <= 50:
		pm.ResultSizeHistogram["11-50"]++
	case resultCount <= 100:
		pm.ResultSizeHistogram["51-100"]++
	default:
		pm.ResultSizeHistogram[">100"]++
	}
}

// RecordCodebaseIntelligence records metrics for codebase intelligence operations
func (pm *ProfilingMetrics) RecordCodebaseIntelligence(duration time.Duration, mode string, err error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.CodebaseIntelligenceOps++
	pm.TotalCodebaseIntelTime += duration

	if err != nil {
		pm.CodebaseIntelErrors++
	}
}

// RecordSemanticScoring records metrics for semantic scoring operations
func (pm *ProfilingMetrics) RecordSemanticScoring(duration time.Duration, candidateCount int, cacheHit bool, err error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.SemanticScoringOps++
	pm.TotalSemanticScoringTime += duration

	if cacheHit {
		pm.SemanticCacheHits++
	} else {
		pm.SemanticCacheMisses++
	}

	if err != nil {
		pm.SemanticScoringErrors++
	}
}

// UpdateMemoryUsage updates memory usage statistics
func (pm *ProfilingMetrics) UpdateMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.CurrentMemoryUsage = m.Alloc
	if m.Alloc > pm.PeakMemoryUsage {
		pm.PeakMemoryUsage = m.Alloc
	}
}

// GetSnapshot returns a snapshot of current metrics
func (pm *ProfilingMetrics) GetSnapshot() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Calculate averages
	avgSearchTime := time.Duration(0)
	if pm.SearchOperations > 0 {
		avgSearchTime = pm.TotalSearchTime / time.Duration(pm.SearchOperations)
	}

	avgCodebaseIntelTime := time.Duration(0)
	if pm.CodebaseIntelligenceOps > 0 {
		avgCodebaseIntelTime = pm.TotalCodebaseIntelTime / time.Duration(pm.CodebaseIntelligenceOps)
	}

	avgSemanticTime := time.Duration(0)
	if pm.SemanticScoringOps > 0 {
		avgSemanticTime = pm.TotalSemanticScoringTime / time.Duration(pm.SemanticScoringOps)
	}

	cacheHitRate := float64(0)
	totalCacheOps := pm.SemanticCacheHits + pm.SemanticCacheMisses
	if totalCacheOps > 0 {
		cacheHitRate = float64(pm.SemanticCacheHits) / float64(totalCacheOps)
	}

	return map[string]interface{}{
		"operations": map[string]int64{
			"search":                pm.SearchOperations,
			"codebase_intelligence": pm.CodebaseIntelligenceOps,
			"semantic_scoring":      pm.SemanticScoringOps,
		},
		"timing": map[string]string{
			"avg_search_time":         avgSearchTime.String(),
			"avg_codebase_intel_time": avgCodebaseIntelTime.String(),
			"avg_semantic_time":       avgSemanticTime.String(),
		},
		"memory": map[string]uint64{
			"current_bytes": pm.CurrentMemoryUsage,
			"peak_bytes":    pm.PeakMemoryUsage,
		},
		"cache": map[string]interface{}{
			"hit_rate": cacheHitRate,
			"hits":     pm.SemanticCacheHits,
			"misses":   pm.SemanticCacheMisses,
		},
		"errors": map[string]int64{
			"search":                pm.SearchErrors,
			"codebase_intelligence": pm.CodebaseIntelErrors,
			"semantic_scoring":      pm.SemanticScoringErrors,
		},
		"distributions": map[string]interface{}{
			"search_latency": pm.SearchLatencyHistogram,
			"result_sizes":   pm.ResultSizeHistogram,
		},
	}
}

// Reset resets all metrics
func (pm *ProfilingMetrics) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.SearchOperations = 0
	pm.CodebaseIntelligenceOps = 0
	pm.SemanticScoringOps = 0
	pm.TotalSearchTime = 0
	pm.TotalCodebaseIntelTime = 0
	pm.TotalSemanticScoringTime = 0
	pm.SearchErrors = 0
	pm.CodebaseIntelErrors = 0
	pm.SemanticScoringErrors = 0
	pm.SemanticCacheHits = 0
	pm.SemanticCacheMisses = 0

	// Reset histograms
	for k := range pm.SearchLatencyHistogram {
		pm.SearchLatencyHistogram[k] = 0
	}
	for k := range pm.ResultSizeHistogram {
		pm.ResultSizeHistogram[k] = 0
	}
}

// ProfilingMiddleware wraps MCP operations with profiling
type ProfilingMiddleware struct {
	metrics *ProfilingMetrics
	server  *Server
}

// NewProfilingMiddleware creates profiling middleware for the MCP server
func NewProfilingMiddleware(server *Server) *ProfilingMiddleware {
	return &ProfilingMiddleware{
		metrics: NewProfilingMetrics(),
		server:  server,
	}
}

// StartCPUProfile starts CPU profiling
func (pm *ProfilingMiddleware) StartCPUProfile() error {
	var buf bytes.Buffer
	return pprof.StartCPUProfile(&buf)
}

// StopCPUProfile stops CPU profiling
func (pm *ProfilingMiddleware) StopCPUProfile() {
	pprof.StopCPUProfile()
}

// WriteHeapProfile writes a heap profile
func (pm *ProfilingMiddleware) WriteHeapProfile() error {
	var buf bytes.Buffer
	return pprof.WriteHeapProfile(&buf)
}

// EnableProfiling adds profiling endpoints to the MCP server
func (s *Server) EnableProfiling() {
	if s.profilingMetrics == nil {
		s.profilingMetrics = NewProfilingMetrics()
	}

	// Note: MCP resource registration would go here if the API supported it
	// For now, profiling metrics are accessed through GetProfilingReport()

	// Start periodic memory usage updates
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.profilingMetrics.UpdateMemoryUsage()
			case <-time.After(1 * time.Hour): // Exit after 1 hour (no shutdownCh available)
				return
			}
		}
	}()
}

// wrapWithProfiling wraps an operation with profiling
func (s *Server) wrapWithProfiling(
	operation string,
	fn func() (interface{}, error),
) (interface{}, error) {
	start := time.Now()

	result, err := fn()

	duration := time.Since(start)

	// Record metrics based on operation type
	if s.profilingMetrics != nil {
		switch operation {
		case "search":
			resultCount := 0
			if results, ok := result.([]interface{}); ok {
				resultCount = len(results)
			}
			s.profilingMetrics.RecordSearch(duration, resultCount, err)

		case "codebase_intelligence":
			s.profilingMetrics.RecordCodebaseIntelligence(duration, "", err)

		case "semantic_scoring":
			candidateCount := 0
			if candidates, ok := result.([]interface{}); ok {
				candidateCount = len(candidates)
			}
			s.profilingMetrics.RecordSemanticScoring(duration, candidateCount, false, err)
		}
	}

	return result, err
}

// GetProfilingReport generates a comprehensive profiling report
func (s *Server) GetProfilingReport() string {
	if s.profilingMetrics == nil {
		return "Profiling not enabled"
	}

	snapshot := s.profilingMetrics.GetSnapshot()

	report := fmt.Sprintf(`
=== MCP Server Profiling Report ===

Operations:
  Search:                %d operations
  Codebase Intelligence: %d operations
  Semantic Scoring:      %d operations

Performance:
  Avg Search Time:       %v
  Avg CI Time:          %v
  Avg Semantic Time:    %v

Memory:
  Current Usage:        %.2f MB
  Peak Usage:           %.2f MB

Cache Performance:
  Hit Rate:             %.2f%%
  Total Hits:           %d
  Total Misses:         %d

Error Rates:
  Search Errors:        %d
  CI Errors:            %d
  Semantic Errors:      %d

Search Latency Distribution:
  <10ms:                %d
  10-50ms:              %d
  50-100ms:             %d
  100-500ms:            %d
  >500ms:               %d

Result Size Distribution:
  0 results:            %d
  1-10 results:         %d
  11-50 results:        %d
  51-100 results:       %d
  >100 results:         %d
`,
		snapshot["operations"].(map[string]int64)["search"],
		snapshot["operations"].(map[string]int64)["codebase_intelligence"],
		snapshot["operations"].(map[string]int64)["semantic_scoring"],
		snapshot["timing"].(map[string]string)["avg_search_time"],
		snapshot["timing"].(map[string]string)["avg_codebase_intel_time"],
		snapshot["timing"].(map[string]string)["avg_semantic_time"],
		float64(snapshot["memory"].(map[string]uint64)["current_bytes"])/(1024*1024),
		float64(snapshot["memory"].(map[string]uint64)["peak_bytes"])/(1024*1024),
		snapshot["cache"].(map[string]interface{})["hit_rate"].(float64)*100,
		snapshot["cache"].(map[string]interface{})["hits"].(int64),
		snapshot["cache"].(map[string]interface{})["misses"].(int64),
		snapshot["errors"].(map[string]int64)["search"],
		snapshot["errors"].(map[string]int64)["codebase_intelligence"],
		snapshot["errors"].(map[string]int64)["semantic_scoring"],
		snapshot["distributions"].(map[string]interface{})["search_latency"].(map[string]int64)["<10ms"],
		snapshot["distributions"].(map[string]interface{})["search_latency"].(map[string]int64)["10-50ms"],
		snapshot["distributions"].(map[string]interface{})["search_latency"].(map[string]int64)["50-100ms"],
		snapshot["distributions"].(map[string]interface{})["search_latency"].(map[string]int64)["100-500ms"],
		snapshot["distributions"].(map[string]interface{})["search_latency"].(map[string]int64)[">500ms"],
		snapshot["distributions"].(map[string]interface{})["result_sizes"].(map[string]int64)["0"],
		snapshot["distributions"].(map[string]interface{})["result_sizes"].(map[string]int64)["1-10"],
		snapshot["distributions"].(map[string]interface{})["result_sizes"].(map[string]int64)["11-50"],
		snapshot["distributions"].(map[string]interface{})["result_sizes"].(map[string]int64)["51-100"],
		snapshot["distributions"].(map[string]interface{})["result_sizes"].(map[string]int64)[">100"],
	)

	return report
}
