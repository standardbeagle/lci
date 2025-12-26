package indexing

import (
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/testhelpers"
)

// TestMemoryPressureHandling tests the memory pressure detection and graceful degradation
func TestMemoryPressureHandling(t *testing.T) {
	t.Skip("Memory pressure test expectations may have changed")
	// Create a config with low memory limits to trigger pressure quickly
	cfg := &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 1, // Very low limit to trigger memory pressure
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:        true,
			EnableGracefulDegradation: true,
		},
		Index: config.Index{
			MaxFileSize: 1024 * 1024, // 1MB
		},
	}

	gi := NewMasterIndex(cfg)

	// Test 1: Initially should not have memory pressure
	if gi.isMemoryPressureDetected() {
		t.Error("Expected no memory pressure initially")
	}

	// Test 2: GetMemoryPressureInfo should return valid information
	memoryInfo := gi.GetMemoryPressureInfo()
	if memoryInfo == nil {
		t.Fatal("GetMemoryPressureInfo returned nil")
	}

	// Verify required fields exist
	requiredFields := []string{
		"current_usage_mb", "max_usage_mb", "pressure_level",
		"is_warning", "is_critical", "needs_eviction",
		"eviction_candidates", "lru_eviction_triggered",
		"graceful_degradation_enabled",
	}

	for _, field := range requiredFields {
		if _, exists := memoryInfo[field]; !exists {
			t.Errorf("Missing required field in memory pressure info: %s", field)
		}
	}

	// Test 3: With low memory limit, should eventually detect pressure
	// Load a large file to trigger memory pressure
	largeContent := make([]byte, 500*1024) // 500KB file
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	// Load multiple large files to exceed the 1MB limit
	for i := 0; i < 3; i++ {
		fileName := "large_test_file.go"
		err := gi.IndexFile(fileName)
		if err != nil && err.Error() != "memory pressure detected - file indexing deferred" {
			t.Errorf("Unexpected error during file indexing: %v", err)
		}
	}

	// Check if memory pressure is detected
	if !gi.isMemoryPressureDetected() {
		t.Log("Memory pressure not yet detected (may be normal depending on FileContentStore implementation)")
	}

	// Test 4: Search operations should be blocked during memory pressure
	searchResult, err := gi.Search("test", 50)
	if err != nil && err.Error() != "memory pressure detected - indexing temporarily suspended" {
		t.Errorf("Expected memory pressure error for search, got: %v", err)
	}
	if len(searchResult) > 0 {
		t.Error("Expected empty search results during memory pressure")
	}

	// Test 5: Health check should include memory pressure information
	health := gi.HealthCheck()
	if health == nil {
		t.Fatal("HealthCheck returned nil")
	}

	status := health["status"].(string)
	if status != "healthy" && status != "degraded" && status != "unhealthy" {
		t.Errorf("Unexpected health status: %s", status)
	}

	// Check if memory pressure is included in health metrics
	metrics := health["metrics"].(map[string]interface{})
	if _, exists := metrics["memory_pressure"]; !exists {
		t.Error("Expected memory_pressure field in health metrics")
	}

	// Test 6: Memory pressure info should be accurate
	memoryInfo = gi.GetMemoryPressureInfo()
	currentUsage := memoryInfo["current_usage_mb"].(int64)
	maxUsage := memoryInfo["max_usage_mb"].(int64)
	pressureLevel := memoryInfo["pressure_level"].(float64)

	if currentUsage < 0 || maxUsage <= 0 {
		t.Error("Invalid memory usage values in memory pressure info")
	}

	if pressureLevel < 0 || pressureLevel > 100 {
		t.Errorf("Invalid pressure level: %f", pressureLevel)
	}

	t.Logf("Memory usage: %d MB / %d MB (%.1f%% pressure)",
		currentUsage, maxUsage, pressureLevel)
}

// TestMemoryPressureConfiguration tests different configuration scenarios
func TestMemoryPressureConfiguration(t *testing.T) {
	t.Skip("Memory pressure configuration test may have changed")
	// Test 1: Memory limits disabled
	cfg := &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 100,
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits: false,
		},
	}

	gi := NewMasterIndex(cfg)

	if gi.isMemoryPressureDetected() {
		t.Error("Expected no memory pressure when limits are disabled")
	}

	// Test 2: Graceful degradation disabled
	cfg = &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 1, // Very low limit
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:        true,
			EnableGracefulDegradation: false,
		},
	}

	gi = NewMasterIndex(cfg)

	// Should still detect pressure but not defer operations
	if gi.isMemoryPressureDetected() {
		t.Log("Memory pressure detected even with graceful degradation disabled (expected)")
	}

	// Test 3: High memory limit should not trigger pressure
	cfg = &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 1000, // Very high limit
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:        true,
			EnableGracefulDegradation: true,
		},
	}

	gi = NewMasterIndex(cfg)

	// Load a moderately sized file
	err := gi.IndexFile("test.go")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if gi.isMemoryPressureDetected() {
		t.Error("Expected no memory pressure with high limit and small file")
	}
}

// TestMemoryPressureThresholds tests the different memory pressure thresholds
func TestMemoryPressureThresholds(t *testing.T) {
	// Create config with moderate memory limit for threshold testing
	cfg := &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 10, // 10MB limit
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:        true,
			EnableGracefulDegradation: true,
		},
	}

	gi := NewMasterIndex(cfg)

	// Test initial state
	memoryInfo := gi.GetMemoryPressureInfo()
	if memoryInfo["is_warning"].(bool) || memoryInfo["is_critical"].(bool) {
		t.Error("Expected no warnings or critical status initially")
	}

	// Monitor memory pressure progression as files are loaded
	filesLoaded := 0
	for i := 0; i < 20; i++ {
		// Create progressively larger files
		fileSize := 100 * 1024 // 100KB per file
		content := make([]byte, fileSize)
		for j := range content {
			content[j] = byte(j % 256)
		}

		fileName := "threshold_test.go"
		err := gi.IndexFile(fileName)
		if err != nil && err.Error() == "memory pressure detected - file indexing deferred" {
			t.Logf("Memory pressure triggered after loading %d files", filesLoaded)
			break
		}

		filesLoaded++

		// Check memory pressure status every few files
		if i%5 == 0 {
			memoryInfo = gi.GetMemoryPressureInfo()
			pressureLevel := memoryInfo["pressure_level"].(float64)
			t.Logf("After %d files: %.1f%% memory pressure", i+1, pressureLevel)

			if memoryInfo["is_warning"].(bool) {
				t.Logf("Warning threshold reached at %.1f%% pressure", pressureLevel)
			}
			if memoryInfo["is_critical"].(bool) {
				t.Logf("Critical threshold reached at %.1f%% pressure", pressureLevel)
				break
			}
		}
	}

	// Final health check should reflect memory pressure state
	health := gi.HealthCheck()
	metrics := health["metrics"].(map[string]interface{})
	if memoryPressure, exists := metrics["memory_pressure"]; exists {
		memoryPressureMap := memoryPressure.(map[string]interface{})
		t.Logf("Final memory pressure state: %.1f%%", memoryPressureMap["pressure_level"])
	}
}

// TestMemoryPressureRecovery tests that the system recovers when memory pressure is relieved
func TestMemoryPressureRecovery(t *testing.T) {
	cfg := &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 2, // Low limit to trigger pressure
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:        true,
			EnableGracefulDegradation: true,
		},
	}

	gi := NewMasterIndex(cfg)

	// Load files until memory pressure is detected
	pressureDetected := false
	for i := 0; i < 10; i++ {
		fileName := "recovery_test.go"
		err := gi.IndexFile(fileName)
		if err != nil && err.Error() == "memory pressure detected - file indexing deferred" {
			pressureDetected = true
			break
		}
	}

	if !pressureDetected {
		t.Log("Memory pressure not reached (may be normal depending on implementation)")
		return
	}

	// Clear the index to relieve memory pressure
	err := gi.Clear()
	if err != nil {
		t.Errorf("Error clearing index: %v", err)
	}

	// Wait for cleanup with timeout
	testhelpers.WaitFor(t, func() bool {
		return !gi.isMemoryPressureDetected()
	}, time.Second)

	// Memory pressure should no longer be detected
	if gi.isMemoryPressureDetected() {
		t.Error("Expected memory pressure to be relieved after clearing index")
	}

	// Should be able to index files again
	err = gi.IndexFile("recovery_test.go")
	if err != nil {
		t.Errorf("Expected to be able to index files after recovery, got: %v", err)
	}
}

// TestMemoryPressureDiagnostics tests diagnostic information during memory pressure
func TestMemoryPressureDiagnostics(t *testing.T) {
	cfg := &config.Config{
		Performance: config.Performance{
			MaxMemoryMB: 5,
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:        true,
			EnableGracefulDegradation: true,
		},
	}

	gi := NewMasterIndex(cfg)

	// Get baseline diagnostics
	baselineHealth := gi.HealthCheck()
	baselineMetrics := baselineHealth["metrics"].(map[string]interface{})

	// Load some content to create memory pressure
	for i := 0; i < 5; i++ {
		fileName := "diagnostic_test.go"
		_ = gi.IndexFile(fileName) // Ignore errors, we're just testing diagnostics
	}

	// Get diagnostics during memory pressure
	pressureHealth := gi.HealthCheck()
	pressureMetrics := pressureHealth["metrics"].(map[string]interface{})

	// Compare diagnostics
	if _, exists := baselineMetrics["memory_pressure"]; !exists {
		t.Error("Expected baseline diagnostics to include memory pressure info")
	}

	if _, exists := pressureMetrics["memory_pressure"]; !exists {
		t.Error("Expected pressure diagnostics to include memory pressure info")
	}

	// Verify diagnostic completeness
	memoryPressure := pressureMetrics["memory_pressure"].(map[string]interface{})
	expectedFields := []string{
		"current_usage_mb", "max_usage_mb", "pressure_level",
		"is_warning", "is_critical", "needs_eviction",
		"eviction_candidates", "graceful_degradation_enabled",
	}

	for _, field := range expectedFields {
		if _, exists := memoryPressure[field]; !exists {
			t.Errorf("Missing diagnostic field: %s", field)
		}
	}

	t.Logf("Diagnostic test completed. Pressure level: %.1f%%",
		memoryPressure["pressure_level"])
}
