package indexing

import (
	"testing"
	"time"
)

func TestIndexingError(t *testing.T) {
	t.Run("create indexing error", func(t *testing.T) {
		err := &IndexingError{
			File:     "test.go",
			FilePath: "/path/to/test.go",
			Message:  "Test error message",
			Stage:    "parsing",
			Error:    "detailed error info",
		}

		if err.File != "test.go" {
			t.Errorf("Expected File = 'test.go', got '%s'", err.File)
		}

		if err.FilePath != "/path/to/test.go" {
			t.Errorf("Expected FilePath = '/path/to/test.go', got '%s'", err.FilePath)
		}

		if err.Message != "Test error message" {
			t.Errorf("Expected Message = 'Test error message', got '%s'", err.Message)
		}

		if err.Stage != "parsing" {
			t.Errorf("Expected Stage = 'parsing', got '%s'", err.Stage)
		}

		if err.Error != "detailed error info" {
			t.Errorf("Expected Error = 'detailed error info', got '%s'", err.Error)
		}
	})

	t.Run("indexing error with minimal fields", func(t *testing.T) {
		err := &IndexingError{
			File:    "minimal.go",
			Message: "Minimal error",
		}

		if err.File != "minimal.go" {
			t.Errorf("Expected File = 'minimal.go', got '%s'", err.File)
		}

		if err.Message != "Minimal error" {
			t.Errorf("Expected Message = 'Minimal error', got '%s'", err.Message)
		}

		// Default/zero values
		if err.FilePath != "" {
			t.Errorf("Expected FilePath = '', got '%s'", err.FilePath)
		}
	})
}

func TestIndexingInProgressError(t *testing.T) {
	t.Run("create indexing in progress error", func(t *testing.T) {
		progress := IndexingProgress{
			FilesProcessed:    50,
			TotalFiles:        100,
			ElapsedTime:       2 * time.Second,
			CurrentFile:       "current.go",
			FilesPerSecond:    25.0,
			EstimatedTimeLeft: 2 * time.Second,
			Errors:            []IndexingError{},
			ScanningProgress:  100.0,
			IndexingProgress:  50.0,
			IsScanning:        false,
		}

		err := &IndexingInProgressError{
			Message:  "Indexing is currently in progress",
			Progress: progress,
		}

		if err.Error() != "Indexing is currently in progress" {
			t.Errorf("Expected Error() = 'Indexing is currently in progress', got '%s'", err.Error())
		}

		if err.Progress.FilesProcessed != 50 {
			t.Errorf("Expected FilesProcessed = 50, got %d", err.Progress.FilesProcessed)
		}

		if err.Progress.TotalFiles != 100 {
			t.Errorf("Expected TotalFiles = 100, got %d", err.Progress.TotalFiles)
		}

		if err.Progress.IsScanning != false {
			t.Errorf("Expected IsScanning = false, got %v", err.Progress.IsScanning)
		}
	})

	t.Run("indexing in progress error with no progress", func(t *testing.T) {
		err := &IndexingInProgressError{
			Message: "Indexing started",
		}

		if err.Error() != "Indexing started" {
			t.Errorf("Expected Error() = 'Indexing started', got '%s'", err.Error())
		}

		// Zero progress struct
		if err.Progress.FilesProcessed != 0 {
			t.Errorf("Expected FilesProcessed = 0, got %d", err.Progress.FilesProcessed)
		}
	})
}

func TestIndexingProgress(t *testing.T) {
	t.Run("create indexing progress", func(t *testing.T) {
		errors := []IndexingError{
			{
				File:    "error1.go",
				Message: "Syntax error",
			},
			{
				File:    "error2.go",
				Message: "Import error",
			},
		}

		progress := IndexingProgress{
			FilesProcessed:    75,
			TotalFiles:        100,
			ElapsedTime:       5 * time.Second,
			CurrentFile:       "processing.go",
			FilesPerSecond:    15.0,
			EstimatedTimeLeft: 2 * time.Second,
			Errors:            errors,
			ScanningProgress:  100.0,
			IndexingProgress:  75.0,
			IsScanning:        false,
		}

		if progress.FilesProcessed != 75 {
			t.Errorf("Expected FilesProcessed = 75, got %d", progress.FilesProcessed)
		}

		if progress.TotalFiles != 100 {
			t.Errorf("Expected TotalFiles = 100, got %d", progress.TotalFiles)
		}

		if progress.ElapsedTime != 5*time.Second {
			t.Errorf("Expected ElapsedTime = 5s, got %v", progress.ElapsedTime)
		}

		if progress.FilesPerSecond != 15.0 {
			t.Errorf("Expected FilesPerSecond = 15.0, got %f", progress.FilesPerSecond)
		}

		if len(progress.Errors) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(progress.Errors))
		}

		if progress.ScanningProgress != 100.0 {
			t.Errorf("Expected ScanningProgress = 100.0, got %f", progress.ScanningProgress)
		}

		if progress.IndexingProgress != 75.0 {
			t.Errorf("Expected IndexingProgress = 75.0, got %f", progress.IndexingProgress)
		}

		if progress.IsScanning != false {
			t.Errorf("Expected IsScanning = false, got %v", progress.IsScanning)
		}
	})

	t.Run("indexing progress during scanning phase", func(t *testing.T) {
		progress := IndexingProgress{
			FilesProcessed:    0,
			TotalFiles:        0, // Unknown during scanning
			ElapsedTime:       1 * time.Second,
			CurrentFile:       "",
			FilesPerSecond:    0,
			EstimatedTimeLeft: 0,
			Errors:            []IndexingError{},
			ScanningProgress:  45.0,
			IndexingProgress:  0.0,
			IsScanning:        true,
		}

		if progress.IsScanning != true {
			t.Errorf("Expected IsScanning = true, got %v", progress.IsScanning)
		}

		if progress.ScanningProgress != 45.0 {
			t.Errorf("Expected ScanningProgress = 45.0, got %f", progress.ScanningProgress)
		}

		if progress.IndexingProgress != 0.0 {
			t.Errorf("Expected IndexingProgress = 0.0 during scanning, got %f", progress.IndexingProgress)
		}
	})

	t.Run("indexing progress calculation edge cases", func(t *testing.T) {
		// Test with zero total files (should not panic)
		progress := IndexingProgress{
			FilesProcessed:   0,
			TotalFiles:       0,
			ElapsedTime:      0,
			FilesPerSecond:   0,
			ScanningProgress: 0.0,
			IndexingProgress: 0.0,
		}

		if progress.FilesPerSecond != 0 {
			t.Errorf("Expected FilesPerSecond = 0, got %f", progress.FilesPerSecond)
		}

		// Test with negative values (should be handled gracefully)
		progress.ElapsedTime = -1 * time.Second
		// This would be handled by the calculation logic in real implementation
		_ = progress.ElapsedTime
	})
}

func TestIndexImplementationType(t *testing.T) {
	t.Run("goroutine implementation type", func(t *testing.T) {
		implType := GoroutineImplementation
		if implType != "goroutine" {
			t.Errorf("Expected GoroutineImplementation = 'goroutine', got '%s'", string(implType))
		}
	})

	t.Run("implementation type string conversion", func(t *testing.T) {
		implType := IndexImplementationType("custom")
		if string(implType) != "custom" {
			t.Errorf("Expected IndexImplementationType('custom') = 'custom', got '%s'", string(implType))
		}
	})
}

func TestIndexStats(t *testing.T) {
	t.Run("create index stats", func(t *testing.T) {
		additionalStats := map[string]interface{}{
			"symbols_per_file": 10.5,
			"avg_file_size":    2048,
		}

		stats := IndexStats{
			TotalFiles:      100,
			TotalSymbols:    1000,
			IndexSize:       1048576, // 1MB
			BuildDuration:   10 * time.Second,
			LastBuilt:       time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			Implementation:  GoroutineImplementation,
			AdditionalStats: additionalStats,
		}

		if stats.TotalFiles != 100 {
			t.Errorf("Expected TotalFiles = 100, got %d", stats.TotalFiles)
		}

		if stats.TotalSymbols != 1000 {
			t.Errorf("Expected TotalSymbols = 1000, got %d", stats.TotalSymbols)
		}

		if stats.IndexSize != 1048576 {
			t.Errorf("Expected IndexSize = 1048576, got %d", stats.IndexSize)
		}

		if stats.BuildDuration != 10*time.Second {
			t.Errorf("Expected BuildDuration = 10s, got %v", stats.BuildDuration)
		}

		if stats.Implementation != GoroutineImplementation {
			t.Errorf("Expected Implementation = GoroutineImplementation, got '%s'", string(stats.Implementation))
		}

		if len(stats.AdditionalStats) != 2 {
			t.Errorf("Expected 2 additional stats, got %d", len(stats.AdditionalStats))
		}

		if stats.AdditionalStats["symbols_per_file"] != 10.5 {
			t.Errorf("Expected symbols_per_file = 10.5, got %v", stats.AdditionalStats["symbols_per_file"])
		}
	})

	t.Run("index stats with minimal values", func(t *testing.T) {
		stats := IndexStats{
			TotalFiles:     0,
			TotalSymbols:   0,
			IndexSize:      0,
			BuildDuration:  0,
			Implementation: GoroutineImplementation,
		}

		if stats.TotalFiles != 0 {
			t.Errorf("Expected TotalFiles = 0, got %d", stats.TotalFiles)
		}

		if stats.AdditionalStats != nil {
			t.Errorf("Expected AdditionalStats = nil, got %v", stats.AdditionalStats)
		}
	})
}

func TestPerformanceStats(t *testing.T) {
	t.Run("create performance stats", func(t *testing.T) {
		stats := PerformanceStats{
			AverageSearchTime:     0.001, // 1ms
			AverageIndexTime:      0.100, // 100ms
			ThroughputFilesPerSec: 50.0,
			ConcurrentOperations:  5,
		}

		if stats.AverageSearchTime != 0.001 {
			t.Errorf("Expected AverageSearchTime = 0.001, got %f", stats.AverageSearchTime)
		}

		if stats.AverageIndexTime != 0.100 {
			t.Errorf("Expected AverageIndexTime = 0.100, got %f", stats.AverageIndexTime)
		}

		if stats.ThroughputFilesPerSec != 50.0 {
			t.Errorf("Expected ThroughputFilesPerSec = 50.0, got %f", stats.ThroughputFilesPerSec)
		}

		if stats.ConcurrentOperations != 5 {
			t.Errorf("Expected ConcurrentOperations = 5, got %d", stats.ConcurrentOperations)
		}
	})

	t.Run("performance stats with zero values", func(t *testing.T) {
		stats := PerformanceStats{}

		if stats.AverageSearchTime != 0 {
			t.Errorf("Expected AverageSearchTime = 0, got %f", stats.AverageSearchTime)
		}

		if stats.ThroughputFilesPerSec != 0 {
			t.Errorf("Expected ThroughputFilesPerSec = 0, got %f", stats.ThroughputFilesPerSec)
		}

		if stats.ConcurrentOperations != 0 {
			t.Errorf("Expected ConcurrentOperations = 0, got %d", stats.ConcurrentOperations)
		}
	})
}

func TestFeatureFlags(t *testing.T) {
	t.Run("feature flag constants", func(t *testing.T) {
		expectedFlags := []string{
			FeatureConcurrentSearch,
			FeatureConcurrentIndexing,
			FeatureProgressTracking,
			FeatureGracefulShutdown,
			FeatureHotReload,
			FeatureIncrementalUpdate,
			FeatureMemoryOptimized,
			FeatureHighThroughput,
			FeatureRealTimeSearch,
			FeatureSymbolAnalysis,
			FeatureFunctionTree,
			FeatureReferenceTracking,
		}

		for i, flag := range expectedFlags {
			if flag == "" {
				t.Errorf("Feature flag %d should not be empty", i)
			}
		}
	})

	t.Run("all features list", func(t *testing.T) {
		if len(allFeatures) != 12 {
			t.Errorf("Expected 12 features in allFeatures, got %d", len(allFeatures))
		}

		// Verify that all expected features are in the list
		expectedSet := make(map[string]bool)
		for _, flag := range allFeatures {
			expectedSet[flag] = true
		}

		testFeatures := []string{
			FeatureConcurrentSearch,
			FeatureConcurrentIndexing,
			FeatureProgressTracking,
			FeatureGracefulShutdown,
			FeatureHotReload,
			FeatureIncrementalUpdate,
			FeatureMemoryOptimized,
			FeatureHighThroughput,
			FeatureRealTimeSearch,
			FeatureSymbolAnalysis,
			FeatureFunctionTree,
			FeatureReferenceTracking,
		}

		for _, flag := range testFeatures {
			if !expectedSet[flag] {
				t.Errorf("Feature '%s' not found in allFeatures", flag)
			}
		}
	})

	t.Run("feature flag uniqueness", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, flag := range allFeatures {
			if seen[flag] {
				t.Errorf("Duplicate feature flag found: '%s'", flag)
			}
			seen[flag] = true
		}
	})
}

func TestAllFeaturesList(t *testing.T) {
	t.Run("all features list should not be empty", func(t *testing.T) {
		if len(allFeatures) == 0 {
			t.Error("allFeatures list should not be empty")
		}
	})

	t.Run("all features should be valid strings", func(t *testing.T) {
		for i, feature := range allFeatures {
			if feature == "" {
				t.Errorf("Feature at index %d should not be empty", i)
			}
			if len(feature) == 0 {
				t.Errorf("Feature at index %d should have non-zero length", i)
			}
		}
	})
}

// Benchmark tests for type creation
func BenchmarkIndexingProgressCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = IndexingProgress{
			FilesProcessed:    100,
			TotalFiles:        1000,
			ElapsedTime:       10 * time.Second,
			CurrentFile:       "test.go",
			FilesPerSecond:    10.0,
			EstimatedTimeLeft: 90 * time.Second,
			Errors:            []IndexingError{},
			ScanningProgress:  100.0,
			IndexingProgress:  10.0,
			IsScanning:        false,
		}
	}
}

func BenchmarkIndexStatsCreation(b *testing.B) {
	additionalStats := map[string]interface{}{
		"test_metric": 42.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexStats{
			TotalFiles:      100,
			TotalSymbols:    1000,
			IndexSize:       1048576,
			BuildDuration:   10 * time.Second,
			LastBuilt:       time.Now(),
			Implementation:  GoroutineImplementation,
			AdditionalStats: additionalStats,
		}
	}
}
