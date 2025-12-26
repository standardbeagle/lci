// Package testhelpers provides test performance monitoring and optimization suggestions
package testhelpers

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

// TestPerformanceMonitor tracks test execution performance and generates optimization suggestions
type TestPerformanceMonitor struct {
	mu                sync.RWMutex
	testResults       map[string]*TestPerformanceResult // Key: "package.TestName"
	slowTests         []*SlowTestDefinition
	optimizationRules []OptimizationRule
	baseline          *PerformanceBaseline
	monitoringEnabled bool
	config            MonitorConfig
}

// TestPerformanceResult represents performance data for a single test
type TestPerformanceResult struct {
	PackageName             string        `json:"package_name"`
	TestName                string        `json:"test_name"`
	ExecutionCount          int           `json:"execution_count"`
	TotalDuration           time.Duration `json:"total_duration"`
	AverageDuration         time.Duration `json:"average_duration"`
	MinDuration             time.Duration `json:"min_duration"`
	MaxDuration             time.Duration `json:"max_duration"`
	LastExecution           time.Time     `json:"last_execution"`
	SlowestExecution        time.Duration `json:"slowest_execution"`
	MemoryAllocated         int64         `json:"memory_allocated"`
	GoroutineDelta          int           `json:"goroutine_delta"`
	IsSlow                  bool          `json:"is_slow"`
	SlowReason              []string      `json:"slow_reason"`
	OptimizationSuggestions []string      `json:"optimization_suggestions"`
}

// SlowTestDefinition defines criteria for identifying slow tests
type SlowTestDefinition struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Threshold   time.Duration `json:"threshold"`
	Unit        string        `json:"unit"`
	Severity    string        `json:"severity"` // "warning", "critical", "info"
	Category    string        `json:"category"` // "setup", "execution", "cleanup", "resource"
}

// OptimizationRule provides suggestions for improving test performance
type OptimizationRule struct {
	Pattern       string        `json:"pattern"`        // Test name or package pattern
	Category      string        `json:"category"`       // "setup", "execution", "cleanup", "resource"
	Condition     ConditionType `json:"condition"`      // When to apply this rule
	Suggestion    string        `json:"suggestion"`     // Human-readable suggestion
	CodeExample   string        `json:"code_example"`   // Example code fix
	Severity      string        `json:"severity"`       // Impact level
	EstimatedGain time.Duration `json:"estimated_gain"` // Expected improvement
}

// ConditionType defines when optimization rules apply
type ConditionType string

const (
	ConditionSlowExecution ConditionType = "slow_execution"
	ConditionMemoryLeak    ConditionType = "memory_leak"
	ConditionGoroutineLeak ConditionType = "goroutine_leak"
	ConditionLargeSetup    ConditionType = "large_setup"
	ConditionNoCleanup     ConditionType = "no_cleanup"
	ConditionExpensiveOps  ConditionType = "expensive_ops"
)

// MonitorConfig configures the performance monitor
type MonitorConfig struct {
	SlowTestThresholds    []SlowTestDefinition `json:"slow_test_thresholds"`
	EnableDetailedLogging bool                 `json:"enable_detailed_logging"`
	HistoryRetentionDays  int                  `json:"history_retention_days"`
	BaselineFile          string               `json:"baseline_file"`
	OutputDir             string               `json:"output_dir"`
	AutoSaveInterval      time.Duration        `json:"auto_save_interval"`
}

// NewTestPerformanceMonitor creates a new test performance monitor
func NewTestPerformanceMonitor(config MonitorConfig) *TestPerformanceMonitor {
	monitor := &TestPerformanceMonitor{
		testResults: make(map[string]*TestPerformanceResult),
		config:      config,
		baseline:    &PerformanceBaseline{metrics: make(map[string]*BaselineMetric)},
	}

	// Initialize slow test definitions
	monitor.slowTests = append(monitor.slowTests,
		&SlowTestDefinition{
			Name:        "unit_test_threshold",
			Description: "Unit tests should complete quickly",
			Threshold:   100 * time.Millisecond,
			Unit:        "ms",
			Severity:    "warning",
			Category:    "execution",
		},
		&SlowTestDefinition{
			Name:        "integration_test_threshold",
			Description: "Integration tests may take longer",
			Threshold:   1 * time.Second,
			Unit:        "s",
			Severity:    "warning",
			Category:    "execution",
		},
		&SlowTestDefinition{
			Name:        "performance_test_threshold",
			Description: "Performance tests can be longer",
			Threshold:   5 * time.Second,
			Unit:        "s",
			Severity:    "info",
			Category:    "execution",
		},
		&SlowTestDefinition{
			Name:        "critical_threshold",
			Description: "Any test taking longer than this is critical",
			Threshold:   10 * time.Second,
			Unit:        "s",
			Severity:    "critical",
			Category:    "execution",
		},
		&SlowTestDefinition{
			Name:        "setup_threshold",
			Description: "Test setup should be minimal",
			Threshold:   50 * time.Millisecond,
			Unit:        "ms",
			Severity:    "warning",
			Category:    "setup",
		},
	)

	// Initialize optimization rules
	monitor.optimizationRules = append(monitor.optimizationRules,
		OptimizationRule{
			Pattern:    "*Test*",
			Category:   "setup",
			Condition:  ConditionLargeSetup,
			Suggestion: "Consider using table-driven tests or shared test fixtures to reduce setup overhead",
			CodeExample: `func TestCases(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"case1", "input1", "expected1"},
		{"case2", "input2", "expected2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := process(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}`,
			Severity:      "medium",
			EstimatedGain: 30 * time.Millisecond,
		},
		OptimizationRule{
			Pattern:    "*Test*",
			Category:   "execution",
			Condition:  ConditionExpensiveOps,
			Suggestion: "Move expensive operations outside of test loops or use caching",
			CodeExample: `func BenchmarkExpensiveOp(b *testing.B) {
	// Cache expensive setup outside the loop
	data := setupExpensiveData()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		process(data)
	}
}`,
			Severity:      "high",
			EstimatedGain: 100 * time.Millisecond,
		},
		OptimizationRule{
			Pattern:    "*Test*",
			Category:   "cleanup",
			Condition:  ConditionNoCleanup,
			Suggestion: "Use t.Cleanup() for reliable resource cleanup",
			CodeExample: `func TestWithCleanup(t *testing.T) {
	// Setup resource that needs cleanup
	resource := setupExpensiveResource()

	// Register cleanup function
	t.Cleanup(func() {
		if err := resource.Close(); err != nil {
			t.Logf("Warning: failed to close resource: %v", err)
		}
	})

	// Test logic here
	// Resource will be cleaned up automatically
}`,
			Severity:      "high",
			EstimatedGain: 50 * time.Millisecond,
		},
		OptimizationRule{
			Pattern:    "*Test*",
			Category:   "resource",
			Condition:  ConditionMemoryLeak,
			Suggestion: "Fix memory leaks by properly managing allocations and cleanup",
			CodeExample: `func TestMemoryManagement(t *testing.T) {
	// Use defer for cleanup
	data := make([]byte, 1024*1024) // 1MB allocation
	defer func() {
		// Clear sensitive data
		for i := range data {
			data[i] = 0
		}
	}()

	// Test logic here
	// Memory will be properly cleaned up
}`,
			Severity:      "critical",
			EstimatedGain: 200 * time.Millisecond,
		},
		OptimizationRule{
			Pattern:    "*Test*",
			Category:   "resource",
			Condition:  ConditionGoroutineLeak,
			Suggestion: "Ensure all goroutines are properly closed or waited for",
			CodeExample: `func TestGoroutineManagement(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Do work here
			process(id)
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}`,
			Severity:      "critical",
			EstimatedGain: 150 * time.Millisecond,
		},
	)

	// Load baseline if specified
	if config.BaselineFile != "" {
		if baseline, err := LoadBaseline(config.BaselineFile); err == nil {
			monitor.baseline = baseline
		}
	}

	return monitor
}

// StartMonitoring begins monitoring a test
func (tpm *TestPerformanceMonitor) StartMonitoring(packageName, testName string) *TestSession {
	tpm.mu.Lock()
	defer tpm.mu.Unlock()

	key := fmt.Sprintf("%s.%s", packageName, testName)

	// Get or create result
	result, exists := tpm.testResults[key]
	if !exists {
		result = &TestPerformanceResult{
			PackageName: packageName,
			TestName:    testName,
			MinDuration: time.Duration(math.MaxInt64),
		}
		tpm.testResults[key] = result
	}

	session := &TestSession{
		StartTime:        time.Now(),
		PackageName:      packageName,
		TestName:         testName,
		MemoryBefore:     getMemoryUsage(),
		GoroutinesBefore: runtime.NumGoroutine(),
		result:           result,
		monitor:          tpm,
	}

	return session
}

// TestSession represents a single test execution session
type TestSession struct {
	StartTime        time.Time
	EndTime          time.Time
	PackageName      string
	TestName         string
	MemoryBefore     int64
	MemoryAfter      int64
	GoroutinesBefore int
	GoroutinesAfter  int
	result           *TestPerformanceResult
	monitor          *TestPerformanceMonitor
}

// FinishMonitoring ends monitoring and records the test performance
func (ts *TestSession) FinishMonitoring(t *testing.T) {
	ts.EndTime = time.Now()
	ts.MemoryAfter = getMemoryUsage()
	ts.GoroutinesAfter = runtime.NumGoroutine()

	duration := ts.EndTime.Sub(ts.StartTime)
	memoryAllocated := ts.MemoryAfter - ts.MemoryBefore
	goroutineDelta := ts.GoroutinesAfter - ts.GoroutinesBefore

	ts.monitor.recordTestResult(ts, duration, memoryAllocated, goroutineDelta)
}

// recordTestResult updates the performance data for a test
func (tpm *TestPerformanceMonitor) recordTestResult(session *TestSession, duration time.Duration, memoryAllocated int64, goroutineDelta int) {
	tpm.mu.Lock()
	defer tpm.mu.Unlock()

	result := session.result

	// Update statistics
	result.ExecutionCount++
	result.TotalDuration += duration
	result.AverageDuration = result.TotalDuration / time.Duration(result.ExecutionCount)
	result.LastExecution = session.EndTime

	// Update min/max
	if duration < result.MinDuration {
		result.MinDuration = duration
	}
	if duration > result.MaxDuration {
		result.MaxDuration = duration
		result.SlowestExecution = duration
	}

	// Update resource metrics
	result.MemoryAllocated = memoryAllocated
	result.GoroutineDelta = goroutineDelta

	// Analyze performance
	tpm.analyzeTestPerformance(result)
}

// analyzeTestPerformance determines if a test is slow and provides suggestions
func (tpm *TestPerformanceMonitor) analyzeTestPerformance(result *TestPerformanceResult) {
	result.SlowReason = []string{}
	result.OptimizationSuggestions = []string{}
	result.IsSlow = false

	// Check against slow test definitions
	for _, slowDef := range tpm.slowTests {
		if result.AverageDuration > slowDef.Threshold {
			result.IsSlow = true
			reason := fmt.Sprintf("%s: Average duration %v exceeds %v threshold",
				slowDef.Severity, result.AverageDuration, slowDef.Threshold)
			result.SlowReason = append(result.SlowReason, reason)
		}
	}

	// Check resource usage
	if result.MemoryAllocated > 10*1024*1024 { // 10MB
		result.IsSlow = true
		result.SlowReason = append(result.SlowReason, "High memory allocation detected")
	}

	if result.GoroutineDelta > 0 {
		result.IsSlow = true
		result.SlowReason = append(result.SlowReason, "Goroutine leak detected")
	}

	// Generate optimization suggestions
	for _, rule := range tpm.optimizationRules {
		if tpm.matchesRule(result, rule) {
			result.OptimizationSuggestions = append(result.OptimizationSuggestions, rule.Suggestion)
		}
	}

	// Add baseline comparison if available
	if tpm.baseline != nil {
		baselineName := fmt.Sprintf("test_%s_%s", result.PackageName, result.TestName)
		compResult := tpm.baseline.CompareMetric(baselineName, float64(result.AverageDuration.Milliseconds()), "ms")
		if !compResult.Passed {
			result.SlowReason = append(result.SlowReason, fmt.Sprintf("Performance regression: %s", compResult.Message))
		}
	}
}

// matchesRule checks if a test result matches an optimization rule
func (tpm *TestPerformanceMonitor) matchesRule(result *TestPerformanceResult, rule OptimizationRule) bool {
	switch rule.Condition {
	case ConditionSlowExecution:
		return result.AverageDuration > 500*time.Millisecond
	case ConditionMemoryLeak:
		return result.MemoryAllocated > 5*1024*1024 // 5MB
	case ConditionGoroutineLeak:
		return result.GoroutineDelta > 0
	case ConditionLargeSetup:
		// Heuristic: check if test name suggests setup work
		return result.AverageDuration > 100*time.Millisecond &&
			(contains(result.TestName, "Setup") || contains(result.TestName, "Init"))
	case ConditionNoCleanup:
		// Heuristic: check for potential cleanup issues
		return result.GoroutineDelta > 0 && result.AverageDuration > 200*time.Millisecond
	case ConditionExpensiveOps:
		// Heuristic: check for expensive operations
		return result.AverageDuration > 1*time.Second && result.MemoryAllocated > 1*1024*1024
	default:
		return false
	}
}

// GetSlowTests returns all tests identified as slow
func (tpm *TestPerformanceMonitor) GetSlowTests() []*TestPerformanceResult {
	tpm.mu.RLock()
	defer tpm.mu.RUnlock()

	var slowTests []*TestPerformanceResult
	for _, result := range tpm.testResults {
		if result.IsSlow {
			slowTests = append(slowTests, result)
		}
	}

	// Sort by average duration (slowest first)
	sort.Slice(slowTests, func(i, j int) bool {
		return slowTests[i].AverageDuration > slowTests[j].AverageDuration
	})

	return slowTests
}

// GetTestResult returns performance data for a specific test
func (tpm *TestPerformanceMonitor) GetTestResult(packageName, testName string) (*TestPerformanceResult, bool) {
	tpm.mu.RLock()
	defer tpm.mu.RUnlock()

	key := fmt.Sprintf("%s.%s", packageName, testName)
	result, exists := tpm.testResults[key]
	return result, exists
}

// GetAllTestResults returns all recorded test performance data
func (tpm *TestPerformanceMonitor) GetAllTestResults() map[string]*TestPerformanceResult {
	tpm.mu.RLock()
	defer tpm.mu.RUnlock()

	result := make(map[string]*TestPerformanceResult)
	for k, v := range tpm.testResults {
		result[k] = v
	}
	return result
}

// GeneratePerformanceReport creates a comprehensive performance report
func (tpm *TestPerformanceMonitor) GeneratePerformanceReport() *PerformanceReport {
	tpm.mu.RLock()
	defer tpm.mu.RUnlock()

	slowTests := tpm.GetSlowTests()

	// Calculate statistics
	var totalTests, slowTestCount int
	var totalDuration time.Duration
	var totalMemory int64
	var totalGoroutineLeaks int

	for _, result := range tpm.testResults {
		totalTests++
		totalDuration += result.AverageDuration
		totalMemory += result.MemoryAllocated
		if result.GoroutineDelta > 0 {
			totalGoroutineLeaks++
		}
		if result.IsSlow {
			slowTestCount++
		}
	}

	avgDuration := time.Duration(0)
	if totalTests > 0 {
		avgDuration = totalDuration / time.Duration(totalTests)
	}

	return &PerformanceReport{
		GeneratedAt:             time.Now(),
		TotalTests:              totalTests,
		SlowTests:               slowTestCount,
		SlowTestPercentage:      float64(slowTestCount) / float64(totalTests) * 100,
		AverageTestDuration:     avgDuration,
		TotalMemoryAllocated:    totalMemory,
		GoroutineLeaks:          totalGoroutineLeaks,
		SlowTestDetails:         slowTests,
		OptimizationSuggestions: tpm.generateGlobalSuggestions(slowTests),
	}
}

// PerformanceReport represents a comprehensive test performance report
type PerformanceReport struct {
	GeneratedAt             time.Time                `json:"generated_at"`
	TotalTests              int                      `json:"total_tests"`
	SlowTests               int                      `json:"slow_tests"`
	SlowTestPercentage      float64                  `json:"slow_test_percentage"`
	AverageTestDuration     time.Duration            `json:"average_test_duration"`
	TotalMemoryAllocated    int64                    `json:"total_memory_allocated"`
	GoroutineLeaks          int                      `json:"goroutine_leaks"`
	SlowTestDetails         []*TestPerformanceResult `json:"slow_test_details"`
	OptimizationSuggestions []string                 `json:"optimization_suggestions"`
}

// generateGlobalSuggestions creates high-level optimization suggestions
func (tpm *TestPerformanceMonitor) generateGlobalSuggestions(slowTests []*TestPerformanceResult) []string {
	suggestions := []string{}

	if len(slowTests) == 0 {
		suggestions = append(suggestions, "✅ All tests are performing well within acceptable thresholds")
		return suggestions
	}

	// Analyze patterns
	setupSlow := 0
	executionSlow := 0
	resourceLeaks := 0

	for _, test := range slowTests {
		for _, reason := range test.SlowReason {
			if contains(reason, "setup") {
				setupSlow++
			}
			if contains(reason, "execution") {
				executionSlow++
			}
			if contains(reason, "leak") {
				resourceLeaks++
			}
		}
	}

	if setupSlow > 0 {
		suggestions = append(suggestions, fmt.Sprintf("🔧 %d tests have slow setup - consider using table-driven tests or shared fixtures", setupSlow))
	}

	if executionSlow > 0 {
		suggestions = append(suggestions, fmt.Sprintf("⚡ %d tests have slow execution - look for expensive operations or infinite loops", executionSlow))
	}

	if resourceLeaks > 0 {
		suggestions = append(suggestions, fmt.Sprintf("🧹 %d tests have resource leaks - add proper cleanup using t.Cleanup()", resourceLeaks))
	}

	if len(slowTests) > 5 {
		suggestions = append(suggestions, "📊 Consider implementing test performance baselines and regression detection")
	}

	return suggestions
}

// SaveReport saves the performance report to a file
func (tpm *TestPerformanceMonitor) SaveReport(report *PerformanceReport, filePath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// SaveState saves the monitor state to a file
func (tpm *TestPerformanceMonitor) SaveState(filePath string) error {
	tpm.mu.RLock()
	defer tpm.mu.RUnlock()

	data, err := json.MarshalIndent(struct {
		TestResults map[string]*TestPerformanceResult `json:"test_results"`
		Config      MonitorConfig                     `json:"config"`
	}{
		TestResults: tpm.testResults,
		Config:      tpm.config,
	}, "", "  ")

	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

// LoadState loads the monitor state from a file
func (tpm *TestPerformanceMonitor) LoadState(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, start fresh
		}
		return fmt.Errorf("failed to read state: %w", err)
	}

	var state struct {
		TestResults map[string]*TestPerformanceResult `json:"test_results"`
		Config      MonitorConfig                     `json:"config"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	tpm.mu.Lock()
	tpm.testResults = state.TestResults
	tpm.config = state.Config
	tpm.mu.Unlock()

	return nil
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getMemoryUsage returns current memory usage in bytes
func getMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

// MonitorTest is a helper function that monitors a test execution
func MonitorTest(t *testing.T, monitor *TestPerformanceMonitor, packageName, testName string, testFunc func()) {
	t.Helper()

	session := monitor.StartMonitoring(packageName, testName)
	defer session.FinishMonitoring(t)

	// Execute the test
	testFunc()
}

// MonitorTestWithBaseline is a helper that also compares against baselines
func MonitorTestWithBaseline(t *testing.T, monitor *TestPerformanceMonitor, packageName, testName string, testFunc func()) {
	t.Helper()

	session := monitor.StartMonitoring(packageName, testName)
	defer session.FinishMonitoring(t)

	// Execute the test
	testFunc()

	// Check against baseline if available
	if monitor.baseline != nil {
		result, exists := monitor.GetTestResult(packageName, testName)
		if exists {
			baselineName := fmt.Sprintf("test_%s_%s", packageName, testName)
			ReportMetric(t, monitor.baseline, baselineName, float64(result.AverageDuration.Milliseconds()), "ms")
		}
	}
}

// CleanupOldData removes performance data older than the retention period
func (tpm *TestPerformanceMonitor) CleanupOldData() {
	tpm.mu.Lock()
	defer tpm.mu.Unlock()

	if tpm.config.HistoryRetentionDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -tpm.config.HistoryRetentionDays)

	for key, result := range tpm.testResults {
		if result.LastExecution.Before(cutoff) {
			delete(tpm.testResults, key)
		}
	}
}

// GetPerformanceSummary returns a concise summary of test performance
func (tpm *TestPerformanceMonitor) GetPerformanceSummary() string {
	tpm.mu.RLock()
	defer tpm.mu.RUnlock()

	slowTests := tpm.GetSlowTests()

	summary := "Test Performance Summary:\n"
	summary += fmt.Sprintf("  Total Tests: %d\n", len(tpm.testResults))
	summary += fmt.Sprintf("  Slow Tests: %d (%.1f%%)\n", len(slowTests),
		float64(len(slowTests))/float64(len(tpm.testResults))*100)

	if len(slowTests) > 0 {
		summary += fmt.Sprintf("  Slowest Test: %s (%v)\n",
			slowTests[0].PackageName+"."+slowTests[0].TestName,
			slowTests[0].AverageDuration)

		if len(slowTests) >= 3 {
			summary += fmt.Sprintf("  Top 3 Slow Tests:\n")
			for i := 0; i < 3 && i < len(slowTests); i++ {
				test := slowTests[i]
				summary += fmt.Sprintf("    %d. %s (%v)\n", i+1,
					test.PackageName+"."+test.TestName, test.AverageDuration)
			}
		}
	} else {
		summary += "  ✅ All tests are performing well!\n"
	}

	return summary
}
