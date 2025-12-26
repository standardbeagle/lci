// Package testhelpers provides performance baseline management for testing
package testhelpers

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BaselineMetric represents a performance measurement baseline
type BaselineMetric struct {
	Name        string    `json:"name"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Threshold   float64   `json:"threshold"` // Acceptable deviation percentage
	Description string    `json:"description"`
	Source      string    `json:"source"` // "best_case", "average", "manual"
	Established time.Time `json:"established"`
	Updated     time.Time `json:"updated"`
}

// PerformanceBaseline manages performance baselines for operations
type PerformanceBaseline struct {
	metrics map[string]*BaselineMetric
	file    string
}

// NewPerformanceBaseline creates a new performance baseline manager
func NewPerformanceBaseline(filePath string) *PerformanceBaseline {
	return &PerformanceBaseline{
		metrics: make(map[string]*BaselineMetric),
		file:    filePath,
	}
}

// LoadBaseline loads baselines from file or creates defaults
func LoadBaseline(filePath string) (*PerformanceBaseline, error) {
	baseline := NewPerformanceBaseline(filePath)

	// Try to load existing baselines
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &baseline.metrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal baselines: %w", err)
		}
	}

	// Ensure essential baselines exist
	baseline.ensureDefaultBaselines()

	return baseline, nil
}

// ensureDefaultBaselines creates default baselines if they don't exist
func (pb *PerformanceBaseline) ensureDefaultBaselines() {
	defaults := []*BaselineMetric{
		{
			Name:        "search_simple_query",
			Value:       5.0, // 5ms target
			Unit:        "ms",
			Threshold:   20.0, // 20% tolerance
			Description: "Time to execute simple search query",
			Source:      "best_case",
		},
		{
			Name:        "search_complex_query",
			Value:       15.0, // 15ms target
			Unit:        "ms",
			Threshold:   25.0,
			Description: "Time to execute complex search query",
			Source:      "best_case",
		},
		{
			Name:        "indexing_1k_files",
			Value:       500.0, // 500ms target
			Unit:        "ms",
			Threshold:   30.0,
			Description: "Time to index 1,000 files",
			Source:      "best_case",
		},
		{
			Name:        "indexing_10k_files",
			Value:       5000.0, // 5s target
			Unit:        "ms",
			Threshold:   40.0,
			Description: "Time to index 10,000 files",
			Source:      "best_case",
		},
		{
			Name:        "memory_usage_small_project",
			Value:       50.0, // 50MB target
			Unit:        "MB",
			Threshold:   50.0,
			Description: "Memory usage for small project (<1k files)",
			Source:      "best_case",
		},
		{
			Name:        "memory_usage_medium_project",
			Value:       100.0, // 100MB target
			Unit:        "MB",
			Threshold:   50.0,
			Description: "Memory usage for medium project (<10k files)",
			Source:      "best_case",
		},
		{
			Name:        "startup_time",
			Value:       100.0, // 100ms target
			Unit:        "ms",
			Threshold:   30.0,
			Description: "Time to initialize index",
			Source:      "best_case",
		},
		{
			Name:        "file_add_latency",
			Value:       1.0, // 1ms target
			Unit:        "ms",
			Threshold:   50.0,
			Description: "Time to add single file to index",
			Source:      "best_case",
		},
	}

	for _, metric := range defaults {
		if _, exists := pb.metrics[metric.Name]; !exists {
			metric.Established = time.Now()
			metric.Updated = time.Now()
			pb.metrics[metric.Name] = metric
		}
	}
}

// SaveBaseline saves baselines to file
func (pb *PerformanceBaseline) SaveBaseline() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(pb.file), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(pb.metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baselines: %w", err)
	}

	if err := os.WriteFile(pb.file, data, 0644); err != nil {
		return fmt.Errorf("failed to write baselines: %w", err)
	}

	return nil
}

// CompareMetric compares a measurement against baseline
func (pb *PerformanceBaseline) CompareMetric(name string, value float64, unit string) *ComparisonResult {
	metric, exists := pb.metrics[name]
	if !exists {
		return &ComparisonResult{
			Name:     name,
			Passed:   false,
			Message:  fmt.Sprintf("No baseline found for metric: %s", name),
			Value:    value,
			Unit:     unit,
			Baseline: 0,
		}
	}

	// Unit conversion if needed
	convertedValue := pb.convertUnits(value, unit, metric.Unit)

	// Calculate deviation percentage
	deviation := math.Abs(convertedValue-metric.Value) / metric.Value * 100

	passed := deviation <= metric.Threshold

	var message string
	if passed {
		message = fmt.Sprintf("PASS: %.2f%s (baseline: %.2f%s, deviation: %.1f%%)",
			convertedValue, metric.Unit, metric.Value, metric.Unit, deviation)
	} else {
		message = fmt.Sprintf("FAIL: %.2f%s (baseline: %.2f%s, deviation: %.1f%%, threshold: %.1f%%)",
			convertedValue, metric.Unit, metric.Value, metric.Unit, deviation, metric.Threshold)
	}

	return &ComparisonResult{
		Name:        name,
		Passed:      passed,
		Message:     message,
		Value:       convertedValue,
		Unit:        metric.Unit,
		Baseline:    metric.Value,
		Deviation:   deviation,
		Threshold:   metric.Threshold,
		Description: metric.Description,
	}
}

// ComparisonResult represents the result of comparing against a baseline
type ComparisonResult struct {
	Name        string  `json:"name"`
	Passed      bool    `json:"passed"`
	Message     string  `json:"message"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
	Baseline    float64 `json:"baseline"`
	Deviation   float64 `json:"deviation"`
	Threshold   float64 `json:"threshold"`
	Description string  `json:"description"`
}

// convertUnits performs basic unit conversion
func (pb *PerformanceBaseline) convertUnits(value float64, from, to string) float64 {
	if from == to {
		return value
	}

	// Time conversions
	if from == "ms" && to == "s" {
		return value / 1000
	}
	if from == "s" && to == "ms" {
		return value * 1000
	}
	if from == "ns" && to == "ms" {
		return value / 1000000
	}
	if from == "ms" && to == "ns" {
		return value * 1000000
	}

	// Memory conversions
	if from == "MB" && to == "KB" {
		return value * 1024
	}
	if from == "KB" && to == "MB" {
		return value / 1024
	}
	if from == "GB" && to == "MB" {
		return value * 1024
	}
	if from == "MB" && to == "GB" {
		return value / 1024
	}

	// Unknown conversion, return original
	return value
}

// UpdateMetric updates a baseline with new measurement
func (pb *PerformanceBaseline) UpdateMetric(name string, value float64, unit, source string) {
	metric, exists := pb.metrics[name]
	if !exists {
		// Create new metric
		metric = &BaselineMetric{
			Name:      name,
			Unit:      unit,
			Threshold: 20.0, // Default 20% threshold
			Source:    source,
		}
		pb.metrics[name] = metric
	}

	// Convert to existing unit if needed
	convertedValue := pb.convertUnits(value, unit, metric.Unit)

	metric.Value = convertedValue
	metric.Updated = time.Now()
	if source != "" {
		metric.Source = source
	}
}

// GetMetric returns a specific baseline metric
func (pb *PerformanceBaseline) GetMetric(name string) (*BaselineMetric, bool) {
	metric, exists := pb.metrics[name]
	return metric, exists
}

// GetAllMetrics returns all baseline metrics
func (pb *PerformanceBaseline) GetAllMetrics() map[string]*BaselineMetric {
	result := make(map[string]*BaselineMetric)
	for k, v := range pb.metrics {
		result[k] = v
	}
	return result
}

// ReportMetric records a performance metric and compares against baseline
func ReportMetric(t *testing.T, baseline *PerformanceBaseline, name string, value float64, unit string) {
	t.Helper()

	result := baseline.CompareMetric(name, value, unit)

	t.Logf("Performance: %s", result.Message)

	if !result.Passed {
		t.Errorf("Performance regression detected for %s: %s", name, result.Message)
	}
}

// BenchmarkResult represents a benchmark execution result
type BenchmarkResult struct {
	Name        string        `json:"name"`
	NsPerOp     int64         `json:"ns_per_op"`
	AllocsPerOp int64         `json:"allocs_per_op"`
	BytesPerOp  int64         `json:"bytes_per_op"`
	Iterations  int64         `json:"iterations"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
}

// SaveBenchmarkResult saves a benchmark result for later comparison
func SaveBenchmarkResult(filePath string, result *BenchmarkResult) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var results []*BenchmarkResult

	// Load existing results
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &results); err != nil {
			// If file is corrupted, start fresh
			results = []*BenchmarkResult{}
		}
	}

	// Add new result
	result.Timestamp = time.Now()
	results = append(results, result)

	// Keep only last 100 results per benchmark name
	filtered := filterRecentResults(results, 100)

	// Save results
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal benchmark results: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write benchmark results: %w", err)
	}

	return nil
}

// filterRecentResults keeps only the most recent N results per benchmark name
func filterRecentResults(results []*BenchmarkResult, maxPerName int) []*BenchmarkResult {
	nameCount := make(map[string]int)

	// Process in reverse order (newest first)
	for i := len(results) - 1; i >= 0; i-- {
		name := results[i].Name
		if nameCount[name] >= maxPerName {
			// Remove this result (already have enough recent ones)
			results = append(results[:i], results[i+1:]...)
		} else {
			nameCount[name]++
		}
	}

	return results
}

// LoadBenchmarkResults loads benchmark results from file
func LoadBenchmarkResults(filePath string) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil // File doesn't exist, return empty results
		}
		return nil, fmt.Errorf("failed to read benchmark results: %w", err)
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal benchmark results: %w", err)
	}

	return results, nil
}
