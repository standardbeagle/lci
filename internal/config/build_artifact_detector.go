// Build artifact detection from language-specific configuration files
// Parses package.json, tsconfig.json, Cargo.toml, etc. to find output directories
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// BuildArtifactDetector finds language-specific build output directories
type BuildArtifactDetector struct {
	projectRoot string
}

// NewBuildArtifactDetector creates a new build artifact detector
func NewBuildArtifactDetector(projectRoot string) *BuildArtifactDetector {
	return &BuildArtifactDetector{projectRoot: projectRoot}
}

// DetectOutputDirectories scans for build configuration files and extracts output directories
// Returns glob patterns to exclude (e.g., "**/dist/**", "**/target/**")
func (bad *BuildArtifactDetector) DetectOutputDirectories() []string {
	var patterns []string

	// JavaScript/TypeScript: package.json, tsconfig.json, vite.config.js, etc.
	patterns = append(patterns, bad.detectJavaScriptOutputs()...)

	// Rust: Cargo.toml
	patterns = append(patterns, bad.detectRustOutputs()...)

	// Go: go.mod (usually no custom output, but check for common patterns)
	patterns = append(patterns, bad.detectGoOutputs()...)

	// Python: setup.py, pyproject.toml
	patterns = append(patterns, bad.detectPythonOutputs()...)

	// Java/Kotlin: build.gradle, pom.xml
	patterns = append(patterns, bad.detectJavaOutputs()...)

	return patterns
}

// detectJavaScriptOutputs finds JS/TS build outputs
func (bad *BuildArtifactDetector) detectJavaScriptOutputs() []string {
	var patterns []string

	// Check package.json for build scripts and output directories
	packageJSON := filepath.Join(bad.projectRoot, "package.json")
	if data, err := os.ReadFile(packageJSON); err == nil {
		var pkg map[string]interface{}
		if json.Unmarshal(data, &pkg) == nil {
			// Check for common output directory configurations
			if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
				// Look for build output hints in scripts
				for _, script := range scripts {
					if scriptStr, ok := script.(string); ok {
						// Extract common output directories from build scripts
						if strings.Contains(scriptStr, "--outDir") || strings.Contains(scriptStr, "-outDir") {
							// Parse outDir from command line
							parts := strings.Fields(scriptStr)
							for i, part := range parts {
								if (part == "--outDir" || part == "-outDir") && i+1 < len(parts) {
									outDir := strings.Trim(parts[i+1], "\"'")
									patterns = append(patterns, "**/"+outDir+"/**")
								}
							}
						}
					}
				}
			}

			// Check for explicit build configuration
			if buildConfig, ok := pkg["build"].(map[string]interface{}); ok {
				if outDir, ok := buildConfig["outDir"].(string); ok {
					patterns = append(patterns, "**/"+outDir+"/**")
				}
			}
		}
	}

	// Check tsconfig.json
	tsconfigJSON := filepath.Join(bad.projectRoot, "tsconfig.json")
	if data, err := os.ReadFile(tsconfigJSON); err == nil {
		var tsconfig map[string]interface{}
		if json.Unmarshal(data, &tsconfig) == nil {
			if compilerOptions, ok := tsconfig["compilerOptions"].(map[string]interface{}); ok {
				if outDir, ok := compilerOptions["outDir"].(string); ok {
					patterns = append(patterns, "**/"+outDir+"/**")
				}
			}
		}
	}

	// Check vite.config.js/ts for output directory (common pattern: build.outDir)
	for _, viteConfig := range []string{"vite.config.js", "vite.config.ts"} {
		viteConfigPath := filepath.Join(bad.projectRoot, viteConfig)
		if data, err := os.ReadFile(viteConfigPath); err == nil {
			content := string(data)
			// Simple heuristic: look for outDir: 'dist' or outDir: "dist"
			if strings.Contains(content, "outDir") {
				// Extract directory name (simple regex-free approach)
				if idx := strings.Index(content, "outDir"); idx != -1 {
					substr := content[idx+6:] // Skip "outDir"
					if colonIdx := strings.Index(substr, ":"); colonIdx != -1 {
						substr = substr[colonIdx+1:]
						// Find quoted string
						for _, quote := range []string{"'", "\""} {
							if strings.Contains(substr, quote) {
								parts := strings.Split(substr, quote)
								if len(parts) >= 2 {
									dirName := strings.TrimSpace(parts[1])
									if dirName != "" {
										patterns = append(patterns, "**/"+dirName+"/**")
									}
								}
								break
							}
						}
					}
				}
			}
		}
	}

	return patterns
}

// detectRustOutputs finds Rust build outputs (Cargo.toml)
func (bad *BuildArtifactDetector) detectRustOutputs() []string {
	var patterns []string

	cargoTOML := filepath.Join(bad.projectRoot, "Cargo.toml")
	if data, err := os.ReadFile(cargoTOML); err == nil {
		var cargo map[string]interface{}
		if toml.Unmarshal(data, &cargo) == nil {
			// Check for custom target directory
			if profile, ok := cargo["profile"].(map[string]interface{}); ok {
				if release, ok := profile["release"].(map[string]interface{}); ok {
					if targetDir, ok := release["target-dir"].(string); ok {
						patterns = append(patterns, "**/"+targetDir+"/**")
					}
				}
			}

			// Rust always outputs to target/ by default
			// (already in default exclusions, but being explicit)
		}
	}

	return patterns
}

// detectGoOutputs finds Go build outputs
func (bad *BuildArtifactDetector) detectGoOutputs() []string {
	// Go typically doesn't have custom output directories in go.mod
	// But we can check for common patterns in Makefiles or build scripts
	return nil
}

// detectPythonOutputs finds Python build outputs
func (bad *BuildArtifactDetector) detectPythonOutputs() []string {
	var patterns []string

	// Check pyproject.toml
	pyprojectTOML := filepath.Join(bad.projectRoot, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectTOML); err == nil {
		var pyproject map[string]interface{}
		if toml.Unmarshal(data, &pyproject) == nil {
			// Check for build output directory
			if tool, ok := pyproject["tool"].(map[string]interface{}); ok {
				// Poetry
				if poetry, ok := tool["poetry"].(map[string]interface{}); ok {
					if build, ok := poetry["build"].(map[string]interface{}); ok {
						if targetDir, ok := build["target-dir"].(string); ok {
							patterns = append(patterns, "**/"+targetDir+"/**")
						}
					}
				}
			}
		}
	}

	return patterns
}

// detectJavaOutputs finds Java/Kotlin build outputs
func (bad *BuildArtifactDetector) detectJavaOutputs() []string {
	// Java projects typically output to build/ or target/
	// Already covered in default exclusions
	return nil
}

// DeduplicatePatterns removes duplicate exclusion patterns
func DeduplicatePatterns(patterns []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(patterns))

	for _, pattern := range patterns {
		if !seen[pattern] {
			seen[pattern] = true
			result = append(result, pattern)
		}
	}

	return result
}
