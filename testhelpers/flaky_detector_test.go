package testhelpers

import (
	"os"
	"testing"
	"time"
)

// TestFlakyTestDetector_MinimalFunctionality tests the core functionality with minimal setup
func TestFlakyTestDetector_MinimalFunctionality(t *testing.T) {
	detector := NewFlakyTestDetector(FlakyDetectorConfig{
		MinimumRuns:             3,
		FlakinessThreshold:      30.0, // 30% failure rate
		RecentRunsWindow:        7 * 24 * time.Hour,
		PatternDetectionEnabled: true,
		Environment:             "test",
	})

	// Test with no data
	report := detector.GenerateFlakyTestReport()
	if report.TotalFlakyTests != 0 {
		t.Errorf("Expected 0 flaky tests, got %d", report.TotalFlakyTests)
	}

	// Add test results - need enough runs to trigger flaky detection
	for i := 0; i < 5; i++ {
		startTime := time.Now().Add(time.Duration(i) * time.Minute)
		status := StatusPass
		errorMsg := ""

		// Make tests 2, 3, and 4 fail to create flaky behavior (60% failure rate)
		if i == 2 || i == 3 || i == 4 {
			status = StatusFail
			errorMsg = "WARNING: DATA RACE detected"
		}

		result := &TestResult{
			PackageName:  "testpackage",
			TestName:     "TestExample",
			Status:       status,
			ErrorMessage: errorMsg,
			Duration:     100 * time.Millisecond,
			StartTime:    startTime,
			EndTime:      startTime.Add(100 * time.Millisecond),
		}

		detector.AnalyzeTestResult(result)
	}

	// Now should be flaky - 60% failure rate exceeds 30% threshold
	report = detector.GenerateFlakyTestReport()
	if report.TotalFlakyTests != 1 {
		t.Errorf("Expected 1 flaky test, got %d", report.TotalFlakyTests)
	}

	// Check categorization
	categories := detector.GetFlakyTestCategories()
	if len(categories) == 0 {
		t.Error("Expected categories to be generated")
	}

	// Check recommendations
	recommendations := detector.GetFlakyTestRecommendations()
	if len(recommendations) == 0 {
		t.Error("Expected recommendations to be generated")
	}

	// Test basic functionality
	flakyTests := detector.GetFlakyTests()
	if len(flakyTests) != 1 {
		t.Errorf("Expected 1 flaky test, got %d", len(flakyTests))
	}

	if flakyTests[0].TestName != "TestExample" {
		t.Errorf("Expected test name 'TestExample', got '%s'", flakyTests[0].TestName)
	}

	// Test scoring
	score := detector.CalculateFlakinessScore(flakyTests[0])
	if score <= 0 {
		t.Errorf("Expected positive flakiness score, got %.2f", score)
	}
}

// TestFlakyTestDetector_Persistence tests basic persistence functionality
func TestFlakyTestDetector_Persistence(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "flaky_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	detector := NewFlakyTestDetector(FlakyDetectorConfig{
		MinimumRuns:             2,
		FlakinessThreshold:      30.0,
		PatternDetectionEnabled: true,
		Environment:             "test",
	})

	// Add test data
	startTime := time.Now()
	result := &TestResult{
		PackageName:  "testpackage",
		TestName:     "TestPersistence",
		Status:       StatusFail,
		ErrorMessage: "WARNING: DATA RACE detected",
		Duration:     100 * time.Millisecond,
		StartTime:    startTime,
		EndTime:      startTime.Add(100 * time.Millisecond),
	}

	detector.AnalyzeTestResult(result)

	// Save state
	err = detector.SaveState(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Create new detector and load state
	newDetector := NewFlakyTestDetector(FlakyDetectorConfig{
		MinimumRuns:             2,
		FlakinessThreshold:      30.0,
		PatternDetectionEnabled: true,
		Environment:             "test",
	})

	err = newDetector.LoadState(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify data was loaded
	flakyTests := newDetector.GetFlakyTests()
	if len(flakyTests) != 1 {
		t.Errorf("Expected 1 flaky test after loading, got %d", len(flakyTests))
	}

	if flakyTests[0].TestName != "TestPersistence" {
		t.Errorf("Expected test name 'TestPersistence', got '%s'", flakyTests[0].TestName)
	}
}
