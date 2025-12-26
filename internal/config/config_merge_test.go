package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for config merging logic

func TestMergeConfigs_ExclusionsMerge(t *testing.T) {
	base := &Config{
		Exclude: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/real_projects/**",
		},
	}

	project := &Config{
		Exclude: []string{
			"**/dist/**",
			"**/build/**",
		},
	}

	merged := mergeConfigs(base, project)

	// Should contain all exclusions from both configs
	assert.Contains(t, merged.Exclude, "**/node_modules/**")
	assert.Contains(t, merged.Exclude, "**/vendor/**")
	assert.Contains(t, merged.Exclude, "**/real_projects/**")
	assert.Contains(t, merged.Exclude, "**/dist/**")
	assert.Contains(t, merged.Exclude, "**/build/**")
	assert.Len(t, merged.Exclude, 5)
}

func TestMergeConfigs_ExclusionsDeduplication(t *testing.T) {
	base := &Config{
		Exclude: []string{
			"**/node_modules/**",
			"**/vendor/**",
		},
	}

	project := &Config{
		Exclude: []string{
			"**/node_modules/**", // Duplicate
			"**/dist/**",
		},
	}

	merged := mergeConfigs(base, project)

	// Should deduplicate
	assert.Len(t, merged.Exclude, 3)
	assert.Contains(t, merged.Exclude, "**/node_modules/**")
	assert.Contains(t, merged.Exclude, "**/vendor/**")
	assert.Contains(t, merged.Exclude, "**/dist/**")
}

func TestMergeConfigs_InclusionsProjectOverride(t *testing.T) {
	base := &Config{
		Include: []string{"*.go", "*.js"},
	}

	project := &Config{
		Include: []string{"*.py", "*.ts"},
	}

	merged := mergeConfigs(base, project)

	// Project inclusions should override base
	assert.Equal(t, project.Include, merged.Include)
	assert.Len(t, merged.Include, 2)
}

func TestMergeConfigs_InclusionsUseBaseIfProjectEmpty(t *testing.T) {
	base := &Config{
		Include: []string{"*.go", "*.js"},
	}

	project := &Config{
		Include: []string{}, // Empty
	}

	merged := mergeConfigs(base, project)

	// Should use base inclusions if project is empty
	assert.Equal(t, base.Include, merged.Include)
}

func TestMergeConfigs_ProjectSettingsTakePrecedence(t *testing.T) {
	base := &Config{
		Index: Index{
			MaxFileSize: 1024 * 1024, // 1MB
		},
		Performance: Performance{
			MaxMemoryMB: 100,
		},
	}

	project := &Config{
		Index: Index{
			MaxFileSize: 10 * 1024 * 1024, // 10MB
		},
		Performance: Performance{
			MaxMemoryMB: 500,
		},
	}

	merged := mergeConfigs(base, project)

	// Project settings should take precedence
	assert.Equal(t, int64(10*1024*1024), merged.Index.MaxFileSize)
	assert.Equal(t, 500, merged.Performance.MaxMemoryMB)
}

func TestMergeConfigs_EmptyBaseExclusions(t *testing.T) {
	base := &Config{
		Exclude: []string{},
	}

	project := &Config{
		Exclude: []string{"**/dist/**"},
	}

	merged := mergeConfigs(base, project)

	// Should just use project exclusions
	assert.Equal(t, project.Exclude, merged.Exclude)
}

// Integration tests for config loading with home directory

func TestLoadWithRoot_MergesGlobalAndProjectConfigs(t *testing.T) {
	// Create temporary directories for testing
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	// Create global config in "home" directory
	globalConfig := `
exclude {
    "**/node_modules/**"
    "**/vendor/**"
    "**/real_projects/**"
}

include {
    "*.go"
    "*.js"
}

index {
    max_file_size "5MB"
}
`
	err := os.WriteFile(filepath.Join(tmpHome, ".lci.kdl"), []byte(globalConfig), 0644)
	require.NoError(t, err)

	// Create project config
	projectConfig := `
project {
    root "."
    name "test-project"
}

exclude {
    "**/dist/**"
    "**/build/**"
}

index {
    max_file_size "10MB"
}
`
	err = os.WriteFile(filepath.Join(tmpProject, ".lci.kdl"), []byte(projectConfig), 0644)
	require.NoError(t, err)

	// Temporarily override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Load config
	cfg, err := LoadWithRoot("", tmpProject)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify exclusions are merged
	assert.Contains(t, cfg.Exclude, "**/node_modules/**", "Should include global exclusion")
	assert.Contains(t, cfg.Exclude, "**/vendor/**", "Should include global exclusion")
	assert.Contains(t, cfg.Exclude, "**/real_projects/**", "Should include global exclusion")
	assert.Contains(t, cfg.Exclude, "**/dist/**", "Should include project exclusion")
	assert.Contains(t, cfg.Exclude, "**/build/**", "Should include project exclusion")

	// Verify project settings take precedence
	assert.Equal(t, int64(10*1024*1024), cfg.Index.MaxFileSize, "Project max file size should override global")

	// Verify project metadata is preserved
	assert.Equal(t, "test-project", cfg.Project.Name)
}

func TestLoadWithRoot_ProjectConfigOnly(t *testing.T) {
	// Create temporary project directory
	tmpProject := t.TempDir()

	// Create project config
	projectConfig := `
project {
    root "."
    name "test-project"
}

exclude {
    "**/dist/**"
}
`
	err := os.WriteFile(filepath.Join(tmpProject, ".lci.kdl"), []byte(projectConfig), 0644)
	require.NoError(t, err)

	// Use a non-existent home directory
	os.Setenv("HOME", "/nonexistent")
	defer os.Unsetenv("HOME")

	// Load config
	cfg, err := LoadWithRoot("", tmpProject)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify project config is loaded
	assert.Contains(t, cfg.Exclude, "**/dist/**")
	assert.Equal(t, "test-project", cfg.Project.Name)
}

func TestLoadWithRoot_GlobalConfigOnly(t *testing.T) {
	// Create temporary home directory
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()

	// Create global config only
	globalConfig := `
exclude {
    "**/node_modules/**"
    "**/real_projects/**"
}
`
	err := os.WriteFile(filepath.Join(tmpHome, ".lci.kdl"), []byte(globalConfig), 0644)
	require.NoError(t, err)

	// Temporarily override home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Load config (no project config exists)
	cfg, err := LoadWithRoot("", tmpProject)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify global exclusions are loaded
	assert.Contains(t, cfg.Exclude, "**/node_modules/**")
	assert.Contains(t, cfg.Exclude, "**/real_projects/**")
}

func TestLoadWithRoot_DefaultConfigFallback(t *testing.T) {
	// Use non-existent directories for both home and project
	tmpProject := t.TempDir()
	os.Setenv("HOME", "/nonexistent")
	defer os.Unsetenv("HOME")

	// Load config (should fall back to defaults)
	cfg, err := LoadWithRoot("", tmpProject)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify default config is returned
	assert.NotEmpty(t, cfg.Exclude, "Should have default exclusions")
	// Default Include is now empty (include everything by default, only exclude binary files)
	assert.Empty(t, cfg.Include, "Should have empty default inclusions (include everything by default)")
}

func TestMergeConfigs_PreservesBaseExclusionsInTests(t *testing.T) {
	// This test specifically validates that real_projects exclusion is preserved
	// This is the key requirement from the user's feedback

	base := &Config{
		Exclude: []string{
			"**/real_projects/**",
			"**/testing/**",
			"**/testdata/**",
		},
	}

	// Simple project config with no exclusions
	project := &Config{
		Project: Project{
			Name: "test-project",
		},
		Exclude: []string{},
	}

	merged := mergeConfigs(base, project)

	// Critical: base exclusions must be preserved even when project has no exclusions
	assert.Contains(t, merged.Exclude, "**/real_projects/**",
		"Base exclusion for real_projects must be preserved for tests")
	assert.Contains(t, merged.Exclude, "**/testing/**",
		"Base exclusion for testing must be preserved")
	assert.Contains(t, merged.Exclude, "**/testdata/**",
		"Base exclusion for testdata must be preserved")
}
