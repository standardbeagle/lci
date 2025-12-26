package testhelpers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// FlakyTest represents a test with inconsistent behavior
type FlakyTest struct {
	PackageName    string      `json:"package_name"`
	TestName       string      `json:"test_name"`
	FirstDetected  time.Time   `json:"first_detected"`
	LastOccurrence time.Time   `json:"last_occurrence"`
	TotalRuns      int         `json:"total_runs"`
	FailureCount   int         `json:"failure_count"`
	FailureRate    float64     `json:"failure_rate"`
	Status         FlakyStatus `json:"status"`
	Notes          string      `json:"notes"`
	Patterns       []string    `json:"patterns"`
}

// FlakyStatus represents the current status of a flaky test
type FlakyStatus string

const (
	StatusActive   FlakyStatus = "active"
	StatusIsolated FlakyStatus = "isolated"
	StatusFixed    FlakyStatus = "fixed"
	StatusIgnored  FlakyStatus = "ignored"
)

// FlakyTestOccurrence represents an individual instance of flaky behavior
type FlakyTestOccurrence struct {
	PackageName   string        `json:"package_name"`
	TestName      string        `json:"test_name"`
	TestRunID     string        `json:"test_run_id"`
	ErrorMessage  string        `json:"error_message"`
	PassedOnRetry bool          `json:"passed_on_retry"`
	Environment   string        `json:"environment"`
	Timestamp     time.Time     `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
}

// TestResult represents a single test execution result
type TestResult struct {
	PackageName     string        `json:"package_name"`
	TestName        string        `json:"test_name"`
	Status          TestStatus    `json:"status"`
	Duration        time.Duration `json:"duration"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	ErrorMessage    string        `json:"error_message"`
	StackTrace      string        `json:"stack_trace"`
	MemoryUsed      int64         `json:"memory_used"`
	GoroutinesStart int           `json:"goroutines_start"`
	GoroutinesEnd   int           `json:"goroutines_end"`
	Allocations     int64         `json:"allocations"`
}

// TestStatus represents the outcome of a test
type TestStatus string

const (
	StatusPass  TestStatus = "pass"
	StatusFail  TestStatus = "fail"
	StatusSkip  TestStatus = "skip"
	StatusPanic TestStatus = "panic"
)

// FlakyTestDetector analyzes test results to detect flaky behavior
type FlakyTestDetector struct {
	config      FlakyDetectorConfig
	testHistory map[string]*FlakyTest // Key: "package_name:test_name"
	patterns    []FlakyPattern
	knownFlaky  map[string]bool // Cache for known flaky tests
}

// FlakyDetectorConfig configures the flaky detection algorithm
type FlakyDetectorConfig struct {
	MinimumRuns             int           `json:"minimum_runs"`
	FlakinessThreshold      float64       `json:"flakiness_threshold"`
	RecentRunsWindow        time.Duration `json:"recent_runs_window"`
	PatternDetectionEnabled bool          `json:"pattern_detection_enabled"`
	Environment             string        `json:"environment"`
}

// FlakyPattern represents a common flaky test pattern
type FlakyPattern struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Detector    func(*TestResult) bool `json:"-"`
	Weight      float64                `json:"weight"`
}

// NewFlakyTestDetector creates a new flaky test detector
func NewFlakyTestDetector(config FlakyDetectorConfig) *FlakyTestDetector {
	detector := &FlakyTestDetector{
		config:      config,
		testHistory: make(map[string]*FlakyTest),
		knownFlaky:  make(map[string]bool),
		patterns: []FlakyPattern{
			{
				Name:        "race_condition",
				Description: "Test fails due to race conditions",
				Detector:    detectRaceCondition,
				Weight:      1.0,
			},
			{
				Name:        "timeout",
				Description: "Test fails due to timeout",
				Detector:    detectTimeout,
				Weight:      0.8,
			},
			{
				Name:        "memory_pressure",
				Description: "Test fails under memory pressure",
				Detector:    detectMemoryPressure,
				Weight:      0.9,
			},
			{
				Name:        "goroutine_leak",
				Description: "Test leaves goroutines running",
				Detector:    detectGoroutineLeak,
				Weight:      1.0,
			},
			{
				Name:        "environment_dependency",
				Description: "Test fails in specific environments",
				Detector:    detectEnvironmentDependency,
				Weight:      0.7,
			},
			{
				Name:        "timing_sensitive",
				Description: "Test fails due to timing issues",
				Detector:    detectTimingSensitivity,
				Weight:      0.6,
			},
		},
	}

	// Load existing flaky test history
	detector.loadFlakyHistory()

	return detector
}

// AnalyzeTestResult processes a test result and updates flaky test information
func (ftd *FlakyTestDetector) AnalyzeTestResult(result *TestResult) {
	key := fmt.Sprintf("%s:%s", result.PackageName, result.TestName)

	// Get or create flaky test record
	flaky, exists := ftd.testHistory[key]
	if !exists {
		flaky = &FlakyTest{
			PackageName:   result.PackageName,
			TestName:      result.TestName,
			FirstDetected: result.EndTime,
			Status:        StatusActive,
		}
		ftd.testHistory[key] = flaky
	}

	// Update statistics
	flaky.TotalRuns++
	flaky.LastOccurrence = result.EndTime

	if result.Status == StatusFail || result.Status == StatusPanic {
		flaky.FailureCount++
	}

	// Calculate failure rate
	flaky.FailureRate = float64(flaky.FailureCount) / float64(flaky.TotalRuns) * 100

	// Detect patterns if enabled
	if ftd.config.PatternDetectionEnabled {
		ftd.detectPatterns(result, flaky)
	}

	// Update status based on failure rate and patterns
	ftd.updateStatus(flaky)

	// Save history
	ftd.saveFlakyHistory()
}

// GetFlakyTests returns all detected flaky tests
func (ftd *FlakyTestDetector) GetFlakyTests() []*FlakyTest {
	tests := make([]*FlakyTest, 0, len(ftd.testHistory))
	for _, test := range ftd.testHistory {
		if test.Status == StatusActive || test.Status == StatusIsolated {
			tests = append(tests, test)
		}
	}

	// Sort by failure rate (highest first)
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].FailureRate > tests[j].FailureRate
	})

	return tests
}

// CalculateFlakinessScore computes a comprehensive flakiness score for a test
func (ftd *FlakyTestDetector) CalculateFlakinessScore(flaky *FlakyTest) float64 {
	// Base score from failure rate
	score := flaky.FailureRate

	// Adjust for recency (more recent failures increase score)
	recentRuns := ftd.getRecentRuns(flaky)
	if recentRuns > 0 {
		recentFailureRate := float64(ftd.countRecentFailures(flaky)) / float64(recentRuns) * 100
		score = (score + recentFailureRate) / 2 // Weight recent failures more heavily
	}

	// Pattern detection bonus
	patternBonus := ftd.calculatePatternBonus(flaky)
	score += patternBonus

	// Frequency penalty (more runs with failures increase score)
	if flaky.TotalRuns > ftd.config.MinimumRuns {
		frequencyPenalty := math.Log(float64(flaky.TotalRuns)) * 2
		score += frequencyPenalty
	}

	// Cap score at 100
	if score > 100 {
		score = 100
	}

	return score
}

// GetFlakyTestCategories categorizes flaky tests by their patterns
func (ftd *FlakyTestDetector) GetFlakyTestCategories() map[string][]*FlakyTest {
	categories := make(map[string][]*FlakyTest)

	for _, test := range ftd.testHistory {
		if test.Status != StatusActive && test.Status != StatusIsolated {
			continue
		}

		// Categorize by dominant pattern
		primaryPattern := ftd.getPrimaryPattern(test)
		categories[primaryPattern] = append(categories[primaryPattern], test)
	}

	return categories
}

// IsTestFlaky checks if a test is currently considered flaky
func (ftd *FlakyTestDetector) IsTestFlaky(packageName, testName string) bool {
	key := fmt.Sprintf("%s:%s", packageName, testName)
	flaky, exists := ftd.testHistory[key]
	if !exists {
		return false
	}

	return flaky.Status == StatusActive && flaky.FailureRate >= ftd.config.FlakinessThreshold
}

// GetFlakyTestRecommendations provides recommendations for all flaky tests
func (ftd *FlakyTestDetector) GetFlakyTestRecommendations() []FlakyTestRecommendation {
	flakyTests := ftd.GetFlakyTests()
	var recommendations []FlakyTestRecommendation

	for _, test := range flakyTests {
		testRecs := ftd.GetFlakyTestRecommendationsForTest(test)
		recommendations = append(recommendations, FlakyTestRecommendation{
			TestName:        fmt.Sprintf("%s/%s", test.PackageName, test.TestName),
			PackageName:     test.PackageName,
			FailureRate:     test.FailureRate,
			Patterns:        test.Patterns,
			Recommendations: testRecs,
			Priority:        ftd.calculateRecommendationPriority(test),
		})
	}

	// Sort by priority (highest first)
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Priority > recommendations[j].Priority
	})

	return recommendations
}

// calculateRecommendationPriority calculates priority score for a flaky test
func (ftd *FlakyTestDetector) calculateRecommendationPriority(test *FlakyTest) int {
	priority := 0

	// Base priority from failure rate
	if test.FailureRate > 80 {
		priority += 100
	} else if test.FailureRate > 60 {
		priority += 80
	} else if test.FailureRate > 40 {
		priority += 60
	} else if test.FailureRate > 20 {
		priority += 40
	}

	// Bonus for high-impact patterns
	for _, pattern := range test.Patterns {
		switch pattern {
		case "race_condition", "goroutine_leak":
			priority += 50
		case "timeout", "memory_pressure":
			priority += 30
		case "timing_sensitive":
			priority += 20
		case "environment_dependency":
			priority += 10
		}
	}

	// Recency bonus (if test failed recently)
	if time.Since(test.LastOccurrence) < 24*time.Hour {
		priority += 25
	} else if time.Since(test.LastOccurrence) < 7*24*time.Hour {
		priority += 10
	}

	return priority
}

// GetFlakyTestRecommendationsForTest provides recommendations for a specific flaky test
func (ftd *FlakyTestDetector) GetFlakyTestRecommendationsForTest(flaky *FlakyTest) []string {
	recommendations := []string{}

	// Based on detected patterns
	for _, pattern := range flaky.Patterns {
		switch pattern {
		case "race_condition":
			recommendations = append(recommendations,
				"Use sync.Mutex or atomic operations for shared data",
				"Remove data races by eliminating shared mutable state",
				"Use proper synchronization with channels or WaitGroups",
			)
		case "timeout":
			recommendations = append(recommendations,
				"Increase test timeout or optimize test performance",
				"Use context with timeout for better cancellation handling",
				"Reduce test data size or optimize algorithms",
			)
		case "memory_pressure":
			recommendations = append(recommendations,
				"Reduce memory usage in test",
				"Use smaller test datasets",
				"Free resources explicitly in test cleanup",
			)
		case "goroutine_leak":
			recommendations = append(recommendations,
				"Ensure all goroutines are properly closed",
				"Use t.Cleanup() for goroutine cleanup",
				"Add goleak verification to test",
			)
		case "timing_sensitive":
			recommendations = append(recommendations,
				"Replace time.Sleep() with proper synchronization",
				"Use deterministic timing or wait for conditions",
				"Add buffering or retry logic for timing-dependent operations",
			)
		case "environment_dependency":
			recommendations = append(recommendations,
				"Make test environment-independent",
				"Use mocking for external dependencies",
				"Skip test in incompatible environments with build tags",
			)
		}
	}

	// General recommendations based on failure rate
	if flaky.FailureRate > 50 {
		recommendations = append(recommendations,
			"Consider moving test to flaky test suite",
			"Investigate root cause before fixing",
			"Add proper error handling and retries",
		)
	}

	return recommendations
}

// Private helper methods

func (ftd *FlakyTestDetector) detectPatterns(result *TestResult, flaky *FlakyTest) {
	detectedPatterns := make([]string, 0)

	for _, pattern := range ftd.patterns {
		if pattern.Detector(result) {
			detectedPatterns = append(detectedPatterns, pattern.Name)
		}
	}

	flaky.Patterns = detectedPatterns
}

func (ftd *FlakyTestDetector) updateStatus(flaky *FlakyTest) {
	if flaky.TotalRuns < ftd.config.MinimumRuns {
		return // Not enough data yet
	}

	if flaky.FailureRate >= ftd.config.FlakinessThreshold {
		if flaky.Status == StatusFixed {
			flaky.Status = StatusActive // Reactivate if flakiness returns
		}
		// Keep active if already active (no action needed)
	} else {
		if flaky.Status == StatusActive {
			// Consider test stable, but keep monitoring
			if flaky.TotalRuns > ftd.config.MinimumRuns*2 {
				flaky.Status = StatusFixed
			}
		}
	}
}

func (ftd *FlakyTestDetector) getRecentRuns(flaky *FlakyTest) int {
	// This is a simplified implementation
	// In a real system, we'd track individual test runs with timestamps
	return flaky.TotalRuns
}

func (ftd *FlakyTestDetector) countRecentFailures(flaky *FlakyTest) int {
	// Simplified: count recent failures based on total runs and patterns
	// In a real system, we'd track individual test runs
	return flaky.FailureCount
}

func (ftd *FlakyTestDetector) calculatePatternBonus(flaky *FlakyTest) float64 {
	bonus := 0.0

	// Bonus for having multiple patterns (indicates more complex issues)
	if len(flaky.Patterns) > 1 {
		bonus += float64(len(flaky.Patterns)) * 5.0
	}

	// Bonus for specific high-impact patterns
	for _, pattern := range flaky.Patterns {
		switch pattern {
		case "race_condition", "goroutine_leak":
			bonus += 10.0
		case "timeout", "memory_pressure":
			bonus += 7.5
		case "timing_sensitive":
			bonus += 5.0
		}
	}

	return bonus
}

func (ftd *FlakyTestDetector) getPrimaryPattern(flaky *FlakyTest) string {
	if len(flaky.Patterns) == 0 {
		return "unknown"
	}

	// Return the most severe pattern
	patternSeverity := map[string]float64{
		"race_condition":         1.0,
		"goroutine_leak":         1.0,
		"timeout":                0.8,
		"memory_pressure":        0.9,
		"timing_sensitive":       0.6,
		"environment_dependency": 0.7,
	}

	maxSeverity := 0.0
	primaryPattern := flaky.Patterns[0]

	for _, pattern := range flaky.Patterns {
		if severity, exists := patternSeverity[pattern]; exists && severity > maxSeverity {
			maxSeverity = severity
			primaryPattern = pattern
		}
	}

	return primaryPattern
}

func (ftd *FlakyTestDetector) loadFlakyHistory() {
	// Implementation would load from persistent storage
	// For now, we'll start with an empty history
}

func (ftd *FlakyTestDetector) saveFlakyHistory() {
	// Implementation would save to persistent storage
	// For now, we'll just keep it in memory
}

// Pattern detection functions

func detectRaceCondition(result *TestResult) bool {
	return strings.Contains(strings.ToLower(result.ErrorMessage), "race") ||
		strings.Contains(strings.ToLower(result.StackTrace), "data race") ||
		strings.Contains(strings.ToLower(result.ErrorMessage), "concurrent") ||
		result.GoroutinesEnd > result.GoroutinesStart
}

func detectTimeout(result *TestResult) bool {
	return strings.Contains(strings.ToLower(result.ErrorMessage), "timeout") ||
		strings.Contains(strings.ToLower(result.ErrorMessage), "deadline exceeded") ||
		result.Duration >= 30*time.Second // Long running test
}

func detectMemoryPressure(result *TestResult) bool {
	return strings.Contains(strings.ToLower(result.ErrorMessage), "out of memory") ||
		strings.Contains(strings.ToLower(result.ErrorMessage), "cannot allocate") ||
		result.MemoryUsed > 100*1024*1024 // >100MB memory usage
}

func detectGoroutineLeak(result *TestResult) bool {
	return result.GoroutinesEnd > result.GoroutinesStart
}

func detectEnvironmentDependency(result *TestResult) bool {
	// Check for environment-specific error messages
	envDependentPatterns := []string{
		"connection refused",
		"network unreachable",
		"permission denied",
		"file not found",
		"command not found",
	}

	errorMsg := strings.ToLower(result.ErrorMessage)
	for _, pattern := range envDependentPatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}
	return false
}

func detectTimingSensitivity(result *TestResult) bool {
	// Check for timing-related issues
	timingPatterns := []string{
		"unexpected timeout",
		"took too long",
		"deadline",
		"sleep",
		"timer",
	}

	errorMsg := strings.ToLower(result.ErrorMessage)
	for _, pattern := range timingPatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}
	return false
}

// RunFlakyTestDetection runs the flaky test detection algorithm on test results
func RunFlakyTestDetection(t *testing.T, testResults []*TestResult) ([]*FlakyTest, error) {
	t.Helper()

	config := FlakyDetectorConfig{
		MinimumRuns:             3,
		FlakinessThreshold:      20.0, // 20% failure rate threshold
		RecentRunsWindow:        24 * time.Hour,
		PatternDetectionEnabled: true,
		Environment:             "test",
	}

	detector := NewFlakyTestDetector(config)

	// Analyze all test results
	for _, result := range testResults {
		detector.AnalyzeTestResult(result)
	}

	// Get detected flaky tests
	flakyTests := detector.GetFlakyTests()

	// Log summary
	t.Logf("Flaky test detection completed:")
	t.Logf("  Total tests analyzed: %d", len(testResults))
	t.Logf("  Flaky tests detected: %d", len(flakyTests))
	if len(flakyTests) > 0 {
		t.Logf("  Average failure rate: %.1f%%", calculateAverageFailureRate(flakyTests))
	}

	return flakyTests, nil
}

// RunMultipleTestAttempts runs a test multiple times to detect flakiness
func RunMultipleTestAttempts(t *testing.T, testName string, attempts int, testFunc func(t *testing.T)) (*FlakyTest, error) {
	t.Helper()

	results := make([]*TestResult, 0, attempts)

	for i := 0; i < attempts; i++ {
		// Create a sub-test for each attempt
		t.Run(fmt.Sprintf("Attempt_%d", i+1), func(t *testing.T) {
			t.Helper()

			start := time.Now()

			// Capture goroutine state before test
			goroutinesStart := runtime.NumGoroutine()

			// Capture memory usage before test
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Run the test
			testFunc(t)

			// Capture final state
			end := time.Now()
			goroutinesEnd := runtime.NumGoroutine()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			// Create test result
			status := StatusPass
			if t.Failed() {
				status = StatusFail
			} else if t.Skipped() {
				status = StatusSkip
			}

			duration := end.Sub(start)

			result := &TestResult{
				PackageName:     "unknown", // Would be extracted from test name
				TestName:        testName,
				Status:          status,
				Duration:        duration,
				StartTime:       start,
				EndTime:         end,
				GoroutinesStart: goroutinesStart,
				GoroutinesEnd:   goroutinesEnd,
				MemoryUsed:      int64(m2.Alloc - m1.Alloc),
				Allocations:     int64(m2.Mallocs - m1.Mallocs),
			}

			// Extract error message if test failed
			// Note: t.Logs() doesn't exist in Go testing, so we'll use a placeholder
			// In a real implementation, you'd need to capture test output differently
			if status == StatusFail {
				result.ErrorMessage = "Test failed - check test output for details"
			}

			results = append(results, result)
		})
	}

	// Analyze results for flakiness
	flakyTests, err := RunFlakyTestDetection(t, results)
	if err != nil {
		return nil, err
	}

	// Return the specific test if it was detected as flaky
	key := fmt.Sprintf("unknown:%s", testName)
	for _, flaky := range flakyTests {
		if fmt.Sprintf("%s:%s", flaky.PackageName, flaky.TestName) == key {
			return flaky, nil
		}
	}

	return nil, nil
}

// extractErrorMessage extracts the most relevant error message from test logs
func extractErrorMessage(logs []string) string {
	// Get the last error message from logs
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		if strings.Contains(log, "FAIL:") {
			// Extract the error message after "FAIL: "
			parts := strings.SplitN(log, "FAIL:", 2)
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Test failed"
}

// calculateAverageFailureRate calculates the average failure rate across flaky tests
func calculateAverageFailureRate(flakyTests []*FlakyTest) float64 {
	if len(flakyTests) == 0 {
		return 0.0
	}

	totalRate := 0.0
	for _, test := range flakyTests {
		totalRate += test.FailureRate
	}

	return totalRate / float64(len(flakyTests))
}

// SaveState saves the detector state to a file
func (ftd *FlakyTestDetector) SaveState(filename string) error {
	// Convert test history to JSON
	data, err := json.MarshalIndent(ftd.testHistory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal flaky test data: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadState loads the detector state from a file
func (ftd *FlakyTestDetector) LoadState(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read flaky test data: %w", err)
	}

	return json.Unmarshal(data, &ftd.testHistory)
}

// detectPatternsForTest detects patterns in a test result (returns pattern objects for tests)
func (ftd *FlakyTestDetector) detectPatternsForTest(result *TestResult) []FlakyPattern {
	var detectedPatterns []FlakyPattern

	for _, pattern := range ftd.patterns {
		if pattern.Detector(result) {
			detectedPatterns = append(detectedPatterns, pattern)
		}
	}

	return detectedPatterns
}

// GenerateFlakyTestReport generates a comprehensive report
func (ftd *FlakyTestDetector) GenerateFlakyTestReport() FlakyTestReport {
	flakyTests := ftd.GetFlakyTests()

	// Categorize tests
	categories := ftd.GetFlakyTestCategories()
	categoryReports := make([]FlakyTestCategoryReport, 0, len(categories))

	for categoryName, tests := range categories {
		categoryReports = append(categoryReports, FlakyTestCategoryReport{
			Name:  categoryName,
			Count: len(tests),
			Tests: tests,
		})
	}

	// Sort categories by count (descending)
	sort.Slice(categoryReports, func(i, j int) bool {
		return categoryReports[i].Count > categoryReports[j].Count
	})

	// Get top flaky tests (limit to 20)
	topFlaky := flakyTests
	if len(topFlaky) > 20 {
		topFlaky = flakyTests[:20]
	}

	return FlakyTestReport{
		GeneratedAt:     time.Now(),
		TotalFlakyTests: len(flakyTests),
		Categories:      categoryReports,
		TopFlakyTests:   topFlaky,
		AllFlakyTests:   flakyTests,
	}
}

// FlakyTestReport represents a comprehensive flaky test report
type FlakyTestReport struct {
	GeneratedAt     time.Time                 `json:"generated_at"`
	TotalFlakyTests int                       `json:"total_flaky_tests"`
	Categories      []FlakyTestCategoryReport `json:"categories"`
	TopFlakyTests   []*FlakyTest              `json:"top_flaky_tests"`
	AllFlakyTests   []*FlakyTest              `json:"all_flaky_tests"`
}

// FlakyTestCategoryReport represents a category of flaky tests
type FlakyTestCategoryReport struct {
	Name  string       `json:"name"`
	Count int          `json:"count"`
	Tests []*FlakyTest `json:"tests"`
}

// FlakyTestRecommendation represents a recommendation for fixing a flaky test
type FlakyTestRecommendation struct {
	TestName        string   `json:"test_name"`
	PackageName     string   `json:"package_name"`
	FailureRate     float64  `json:"failure_rate"`
	Patterns        []string `json:"patterns"`
	Recommendations []string `json:"recommendations"`
	Priority        int      `json:"priority"`
}

// GenerateFlakyTestReport creates a comprehensive report of flaky tests (legacy function)
func GenerateFlakyTestReport(flakyTests []*FlakyTest) string {
	report := strings.Builder{}

	report.WriteString("# Flaky Test Report\n\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Total Flaky Tests: %d\n\n", len(flakyTests)))

	if len(flakyTests) == 0 {
		report.WriteString("No flaky tests detected! 🎉\n")
		return report.String()
	}

	// Categorize tests by primary pattern
	categories := make(map[string][]*FlakyTest)
	for _, test := range flakyTests {
		primaryPattern := "unknown"
		if len(test.Patterns) > 0 {
			primaryPattern = test.Patterns[0] // Simplified - would use getPrimaryPattern
		}
		categories[primaryPattern] = append(categories[primaryPattern], test)
	}

	// Generate summary table
	report.WriteString("## Summary\n\n")
	report.WriteString("| Pattern | Count | Avg Failure Rate |\n")
	report.WriteString("|---------|-------|----------------|\n")

	for pattern, tests := range categories {
		avgRate := calculateAverageFailureRate(tests)
		report.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", pattern, len(tests), avgRate))
	}

	// Detailed test information
	report.WriteString("\n## Detailed Results\n\n")

	for _, test := range flakyTests {
		report.WriteString(fmt.Sprintf("### %s/%s\n", test.PackageName, test.TestName))
		report.WriteString(fmt.Sprintf("- **Failure Rate**: %.1f%% (%d/%d runs)\n",
			test.FailureRate, test.FailureCount, test.TotalRuns))
		report.WriteString(fmt.Sprintf("- **First Detected**: %s\n", test.FirstDetected.Format("2006-01-02")))
		report.WriteString(fmt.Sprintf("- **Last Occurrence**: %s\n", test.LastOccurrence.Format("2006-01-02")))
		report.WriteString(fmt.Sprintf("- **Status**: %s\n", test.Status))

		if len(test.Patterns) > 0 {
			report.WriteString(fmt.Sprintf("- **Detected Patterns**: %s\n", strings.Join(test.Patterns, ", ")))
		}

		if test.Notes != "" {
			report.WriteString(fmt.Sprintf("- **Notes**: %s\n", test.Notes))
		}

		report.WriteString("\n")
	}

	// Recommendations section
	report.WriteString("## Recommendations\n\n")

	// Count tests by category
	patternCounts := make(map[string]int)
	for _, test := range flakyTests {
		for _, pattern := range test.Patterns {
			patternCounts[pattern]++
		}
	}

	if patternCounts["race_condition"] > 0 {
		report.WriteString("### Race Condition Issues\n")
		report.WriteString("- Use sync.Mutex or atomic operations\n")
		report.WriteString("- Eliminate shared mutable state\n")
		report.WriteString("- Add proper synchronization\n\n")
	}

	if patternCounts["timeout"] > 0 {
		report.WriteString("### Timeout Issues\n")
		report.WriteString("- Increase test timeout or optimize performance\n")
		report.WriteString("- Use context with timeout for better cancellation\n")
		report.WriteString("- Reduce test data size\n\n")
	}

	if patternCounts["timing_sensitive"] > 0 {
		report.WriteString("### Timing-Sensitive Issues\n")
		report.WriteString("- Replace time.Sleep() with proper synchronization\n")
		report.WriteString("- Use deterministic waiting conditions\n")
		report.WriteString("- Add retry logic for timing-dependent operations\n\n")
	}

	return report.String()
}

// SaveFlakyTestReport saves the flaky test report to a file
func SaveFlakyTestReport(report string, filepath string) error {
	return os.WriteFile(filepath, []byte(report), 0644)
}

// LoadFlakyTestReport loads a flaky test report from a file
func LoadFlakyTestReport(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FindFlakyTestReports searches for existing flaky test reports
func FindFlakyTestReports(rootDir string) ([]string, error) {
	var reports []string

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		// Look for flaky test report files
		if strings.HasSuffix(path, "-flaky-report.md") ||
			strings.HasSuffix(path, "flaky-tests.md") ||
			strings.HasSuffix(path, "flaky-analysis.md") {
			reports = append(reports, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return reports, nil
}

// ParseTestName extracts package and test name from a test identifier
func ParseTestName(testIdentifier string) (packageName, testName string) {
	// Handle different test identifier formats
	if strings.Contains(testIdentifier, ".") {
		parts := strings.SplitN(testIdentifier, ".", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return "", testIdentifier
}

// DefaultFlakyDetectorConfig returns a sensible default configuration
func DefaultFlakyDetectorConfig() FlakyDetectorConfig {
	return FlakyDetectorConfig{
		MinimumRuns:             5,
		FlakinessThreshold:      20.0,
		RecentRunsWindow:        24 * time.Hour,
		PatternDetectionEnabled: true,
		Environment:             "local",
	}
}
