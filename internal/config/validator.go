package config

import (
	"errors"
	"fmt"
	"runtime"

	lcierrors "github.com/standardbeagle/lci/internal/errors"
)

// Validator validates configuration and sets smart defaults
type Validator struct{}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateAndSetDefaults validates configuration and applies smart defaults
// Returns an error if validation fails
func (v *Validator) ValidateAndSetDefaults(cfg *Config) error {
	if err := v.validateProjectConfig(&cfg.Project); err != nil {
		return lcierrors.NewConfigError("project", "", err)
	}

	if err := v.validateIndexConfig(&cfg.Index); err != nil {
		return lcierrors.NewConfigError("index", "", err)
	}

	if err := v.validatePerformanceConfig(&cfg.Performance); err != nil {
		return lcierrors.NewConfigError("performance", "", err)
	}

	if err := v.validateSearchConfig(&cfg.Search); err != nil {
		return lcierrors.NewConfigError("search", "", err)
	}

	v.setSmartDefaults(cfg)
	return nil
}

// validateProjectConfig validates project configuration
func (v *Validator) validateProjectConfig(project *Project) error {
	if project.Root == "" {
		return errors.New("project root cannot be empty")
	}

	if project.Name == "" {
		return errors.New("project name cannot be empty")
	}

	return nil
}

// validateIndexConfig validates index configuration
func (v *Validator) validateIndexConfig(index *Index) error {
	if index.MaxFileSize <= 0 {
		return fmt.Errorf("MaxFileSize must be positive, got %d", index.MaxFileSize)
	}

	if index.MaxTotalSizeMB <= 0 {
		return fmt.Errorf("MaxTotalSizeMB must be positive, got %d", index.MaxTotalSizeMB)
	}

	if index.MaxFileCount <= 0 {
		return fmt.Errorf("MaxFileCount must be positive, got %d", index.MaxFileCount)
	}

	if index.MaxFileSize > 100*1024*1024 {
		return fmt.Errorf("MaxFileSize should not exceed 100MB, got %d", index.MaxFileSize)
	}

	return nil
}

// validatePerformanceConfig validates performance configuration
func (v *Validator) validatePerformanceConfig(perf *Performance) error {
	if perf.MaxMemoryMB < 100 {
		return fmt.Errorf("MaxMemoryMB must be at least 100MB, got %d", perf.MaxMemoryMB)
	}

	// MaxGoroutines: 0 means auto-detect (will be set by smart defaults)
	if perf.MaxGoroutines < 0 {
		return fmt.Errorf("MaxGoroutines cannot be negative, got %d", perf.MaxGoroutines)
	}

	// ParallelFileWorkers: 0 means auto-detect (will be set by smart defaults)
	if perf.ParallelFileWorkers < 0 {
		return fmt.Errorf("ParallelFileWorkers cannot be negative, got %d", perf.ParallelFileWorkers)
	}

	return nil
}

// validateSearchConfig validates search configuration
func (v *Validator) validateSearchConfig(search *Search) error {
	if search.MaxContextLines < 0 {
		return fmt.Errorf("MaxContextLines cannot be negative, got %d", search.MaxContextLines)
	}

	if search.MaxResults < 0 {
		return fmt.Errorf("MaxResults cannot be negative, got %d", search.MaxResults)
	}

	return nil
}

// setSmartDefaults applies smart defaults based on system capabilities
func (v *Validator) setSmartDefaults(cfg *Config) {
	// Set default MaxGoroutines based on CPU count if not configured
	// Use cores-1 to leave headroom for the system, minimum of 1
	if cfg.Performance.MaxGoroutines == 0 {
		numCPU := runtime.NumCPU()
		cfg.Performance.MaxGoroutines = max(1, numCPU-1)
	}

	// Set default parallel workers to cores-1 to prevent overwhelming the system
	// This leaves one core free for the OS and other applications
	if cfg.Performance.ParallelFileWorkers == 0 {
		numCPU := runtime.NumCPU()
		cfg.Performance.ParallelFileWorkers = max(1, numCPU-1)
	}

	// Set default memory limit based on available memory if not configured
	if cfg.Performance.MaxMemoryMB == 0 {
		// Conservative default of 1GB
		cfg.Performance.MaxMemoryMB = 1024
	}

	// Set default max context lines if not configured
	if cfg.Search.MaxContextLines == 0 {
		cfg.Search.MaxContextLines = 50
	}

	// Enable smart size control by default
	if !cfg.Index.SmartSizeControl {
		cfg.Index.SmartSizeControl = true
	}

	// Set default priority mode
	if cfg.Index.PriorityMode == "" {
		cfg.Index.PriorityMode = "recent"
	}
}

// ValidateConfig is a convenience function for quick validation
func ValidateConfig(cfg *Config) error {
	validator := NewValidator()
	return validator.ValidateAndSetDefaults(cfg)
}
