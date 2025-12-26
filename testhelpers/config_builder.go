// Package testhelpers provides shared utilities for testing Lightning Code Index
package testhelpers

import (
	"github.com/standardbeagle/lci/internal/config"
)

// TestConfigBuilder provides a fluent API for building test configs with safe defaults
// This is intentionally in a separate file to avoid circular dependencies with indexing tests
// Usage:
//
//	cfg := testhelpers.NewTestConfigBuilder(projectPath).
//		WithExclusions(".git/**", "vendor/**").
//		WithIncludePatterns("*.go", "*.ts").
//		Build()
type TestConfigBuilder struct {
	projectRoot string
	exclusions  []string
	inclusions  []string
}

// NewTestConfigBuilder creates a config builder with safe defaults for a project path
func NewTestConfigBuilder(projectRoot string) *TestConfigBuilder {
	return &TestConfigBuilder{
		projectRoot: projectRoot,
		// Start with critical safe defaults
		exclusions: []string{
			".git/**",
			".git_/**",
			"node_modules/**",
			"vendor/**",
			"dist/**",
			"build/**",
			"target/**",
			"__pycache__/**",
			"testdata/**",
			"*.min.js",
			"*.test.go",
			// Exclude test artifacts created by other tests
			"file_*.go",
			"error_*.go",
			"temp_*.go",
			"test_*.go",
		},
		inclusions: []string{
			"*.go", "*.js", "*.jsx", "*.ts", "*.tsx", "*.py",
			"*.java", "*.c", "*.cpp", "*.h", "*.hpp", "*.rs",
			"*.md", "*.txt",
		},
	}
}

// WithExclusions adds additional exclusion patterns
func (b *TestConfigBuilder) WithExclusions(patterns ...string) *TestConfigBuilder {
	b.exclusions = append(b.exclusions, patterns...)
	return b
}

// WithIncludePatterns sets the include patterns (replaces defaults)
func (b *TestConfigBuilder) WithIncludePatterns(patterns ...string) *TestConfigBuilder {
	b.inclusions = patterns
	return b
}

// AddIncludePatterns adds to the existing include patterns
func (b *TestConfigBuilder) AddIncludePatterns(patterns ...string) *TestConfigBuilder {
	b.inclusions = append(b.inclusions, patterns...)
	return b
}

// Build creates the final test config with all settings
func (b *TestConfigBuilder) Build() *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.Project{
			Root: b.projectRoot,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   50,               // 50MB for tests
			MaxFileCount:     1000,             // Limited for test performance
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
			RespectGitignore: false, // Disabled for tests
			WatchMode:        false, // Disabled for tests
		},
		Performance: config.Performance{
			MaxMemoryMB:   100, // Reduced for tests
			MaxGoroutines: 4,   // Limited for predictable behavior
			DebounceMs:    10,  // Fast debounce for tests
		},
		Search: config.Search{
			DefaultContextLines:    3,
			MaxResults:             50,
			EnableFuzzy:            true,
			MaxContextLines:        20,
			MergeFileResults:       true,
			EnsureCompleteStmt:     true,
			IncludeLeadingComments: true,
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:          true,
			EnableGracefulDegradation:   true,
			EnablePerformanceMonitoring: false,
			EnableDetailedErrorLogging:  false,
			EnableFeatureFlagLogging:    false,
		},
		Include: b.inclusions,
		Exclude: b.exclusions,
	}
}
