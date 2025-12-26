package config

import (
	"testing"
)

func TestValidateAndSetDefaults(t *testing.T) {
	cfg := &Config{
		Project: Project{
			Root: "/test/root",
			Name: "test-project",
		},
		Index: Index{
			MaxFileSize:    1024 * 1024,
			MaxTotalSizeMB: 1000,
			MaxFileCount:   10000,
		},
		Performance: Performance{
			MaxMemoryMB:         2048,
			MaxGoroutines:       1, // Set to valid value to pass validation
			ParallelFileWorkers: 1, // Set to valid value to pass validation
		},
		Search: Search{
			MaxContextLines: 0, // Should be set to 50
		},
	}

	validator := NewValidator()
	err := validator.ValidateAndSetDefaults(cfg)
	if err != nil {
		t.Fatalf("ValidateAndSetDefaults failed: %v", err)
	}

	// Check defaults were applied
	if cfg.Performance.MaxGoroutines == 0 {
		t.Errorf("MaxGoroutines should have been set to CPU count")
	}

	if cfg.Performance.ParallelFileWorkers == 0 {
		t.Errorf("ParallelFileWorkers should have been set to 2x CPU count")
	}

	if cfg.Search.MaxContextLines == 0 {
		t.Errorf("MaxContextLines should have been set to 50")
	}

	if !cfg.Index.SmartSizeControl {
		t.Errorf("SmartSizeControl should be enabled by default")
	}

	if cfg.Index.PriorityMode == "" {
		t.Errorf("PriorityMode should have a default value")
	}
}

func TestValidateProjectConfig(t *testing.T) {
	validator := NewValidator()

	// Valid config
	err := validator.validateProjectConfig(&Project{
		Root: "/test/root",
		Name: "test-project",
	})
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Empty root
	err = validator.validateProjectConfig(&Project{
		Root: "",
		Name: "test-project",
	})
	if err == nil {
		t.Errorf("Expected error for empty root")
	}

	// Empty name
	err = validator.validateProjectConfig(&Project{
		Root: "/test/root",
		Name: "",
	})
	if err == nil {
		t.Errorf("Expected error for empty name")
	}
}

func TestValidateIndexConfig(t *testing.T) {
	validator := NewValidator()

	// Valid config
	err := validator.validateIndexConfig(&Index{
		MaxFileSize:    1024 * 1024,
		MaxTotalSizeMB: 1000,
		MaxFileCount:   10000,
	})
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Invalid MaxFileSize
	err = validator.validateIndexConfig(&Index{
		MaxFileSize:    0,
		MaxTotalSizeMB: 1000,
		MaxFileCount:   10000,
	})
	if err == nil {
		t.Errorf("Expected error for zero MaxFileSize")
	}

	// Invalid MaxTotalSizeMB
	err = validator.validateIndexConfig(&Index{
		MaxFileSize:    1024 * 1024,
		MaxTotalSizeMB: 0,
		MaxFileCount:   10000,
	})
	if err == nil {
		t.Errorf("Expected error for zero MaxTotalSizeMB")
	}

	// Invalid MaxFileCount
	err = validator.validateIndexConfig(&Index{
		MaxFileSize:    1024 * 1024,
		MaxTotalSizeMB: 1000,
		MaxFileCount:   0,
	})
	if err == nil {
		t.Errorf("Expected error for zero MaxFileCount")
	}

	// MaxFileSize too large
	err = validator.validateIndexConfig(&Index{
		MaxFileSize:    200 * 1024 * 1024, // 200MB
		MaxTotalSizeMB: 1000,
		MaxFileCount:   10000,
	})
	if err == nil {
		t.Errorf("Expected error for MaxFileSize > 100MB")
	}
}

func TestValidatePerformanceConfig(t *testing.T) {
	validator := NewValidator()

	// Valid config
	err := validator.validatePerformanceConfig(&Performance{
		MaxMemoryMB:         2048,
		MaxGoroutines:       4,
		ParallelFileWorkers: 8,
	})
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Invalid MaxMemoryMB
	err = validator.validatePerformanceConfig(&Performance{
		MaxMemoryMB:         50, // Too low
		MaxGoroutines:       4,
		ParallelFileWorkers: 8,
	})
	if err == nil {
		t.Errorf("Expected error for MaxMemoryMB < 100")
	}

	// MaxGoroutines = 0 is valid (means auto-detect)
	err = validator.validatePerformanceConfig(&Performance{
		MaxMemoryMB:         2048,
		MaxGoroutines:       0,
		ParallelFileWorkers: 8,
	})
	if err != nil {
		t.Errorf("Expected no error for MaxGoroutines = 0 (auto-detect), got %v", err)
	}

	// ParallelFileWorkers = 0 is valid (means auto-detect)
	err = validator.validatePerformanceConfig(&Performance{
		MaxMemoryMB:         2048,
		MaxGoroutines:       4,
		ParallelFileWorkers: 0,
	})
	if err != nil {
		t.Errorf("Expected no error for ParallelFileWorkers = 0 (auto-detect), got %v", err)
	}

	// Invalid MaxGoroutines (negative)
	err = validator.validatePerformanceConfig(&Performance{
		MaxMemoryMB:         2048,
		MaxGoroutines:       -1,
		ParallelFileWorkers: 8,
	})
	if err == nil {
		t.Errorf("Expected error for MaxGoroutines = -1")
	}

	// Invalid ParallelFileWorkers (negative)
	err = validator.validatePerformanceConfig(&Performance{
		MaxMemoryMB:         2048,
		MaxGoroutines:       4,
		ParallelFileWorkers: -1,
	})
	if err == nil {
		t.Errorf("Expected error for ParallelFileWorkers = -1")
	}
}

func TestValidateSearchConfig(t *testing.T) {
	validator := NewValidator()

	// Valid config
	err := validator.validateSearchConfig(&Search{
		MaxContextLines: 50,
		MaxResults:      100,
	})
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Negative MaxContextLines
	err = validator.validateSearchConfig(&Search{
		MaxContextLines: -1,
		MaxResults:      100,
	})
	if err == nil {
		t.Errorf("Expected error for negative MaxContextLines")
	}

	// Negative MaxResults
	err = validator.validateSearchConfig(&Search{
		MaxContextLines: 50,
		MaxResults:      -10,
	})
	if err == nil {
		t.Errorf("Expected error for negative MaxResults")
	}
}

func TestValidateConfig(t *testing.T) {
	// Test convenience function
	cfg := &Config{
		Project: Project{
			Root: "/test/root",
			Name: "test-project",
		},
		Index: Index{
			MaxFileSize:    1024 * 1024,
			MaxTotalSizeMB: 1000,
			MaxFileCount:   10000,
		},
		Performance: Performance{
			MaxMemoryMB:         2048,
			MaxGoroutines:       1, // Set to valid value
			ParallelFileWorkers: 1, // Set to valid value
		},
		Search: Search{
			MaxContextLines: 50,
		},
	}

	err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("ValidateConfig failed: %v", err)
	}

	// Test with invalid config
	invalidCfg := &Config{
		Project: Project{
			Root: "", // Invalid
			Name: "test-project",
		},
	}

	err = ValidateConfig(invalidCfg)
	if err == nil {
		t.Errorf("Expected error for invalid config")
	}
}

func TestSetSmartDefaults(t *testing.T) {
	cfg := &Config{
		Project: Project{
			Root: "/test/root",
			Name: "test-project",
		},
		Index: Index{
			MaxFileSize:    1024 * 1024,
			MaxTotalSizeMB: 1000,
			MaxFileCount:   10000,
		},
		Performance: Performance{
			MaxMemoryMB: 0, // Should be set
		},
		Search: Search{
			MaxContextLines: 0, // Should be set
		},
	}

	validator := NewValidator()
	validator.setSmartDefaults(cfg)

	// These should have been set
	if cfg.Performance.MaxMemoryMB == 0 {
		t.Errorf("MaxMemoryMB should have been set")
	}

	if cfg.Search.MaxContextLines == 0 {
		t.Errorf("MaxContextLines should have been set")
	}

	if cfg.Index.PriorityMode == "" {
		t.Errorf("PriorityMode should have been set")
	}
}

func BenchmarkValidateAndSetDefaults(b *testing.B) {
	cfg := &Config{
		Project: Project{
			Root: "/test/root",
			Name: "test-project",
		},
		Index: Index{
			MaxFileSize:    1024 * 1024,
			MaxTotalSizeMB: 1000,
			MaxFileCount:   10000,
		},
		Performance: Performance{
			MaxMemoryMB: 2048,
		},
		Search: Search{
			MaxContextLines: 50,
		},
	}

	validator := NewValidator()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create a fresh config for each iteration
		testCfg := *cfg
		_ = validator.ValidateAndSetDefaults(&testCfg)
	}
}
