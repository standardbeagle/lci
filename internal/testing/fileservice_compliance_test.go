package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileServiceCompliance_SmokeTest performs a basic smoke test for FileService compliance
func TestFileServiceCompliance_SmokeTest(t *testing.T) {
	// This test focuses on critical directories that MUST be compliant
	criticalDirs := []string{
		"../../internal/indexing",
		"../../internal/core",
		"../../internal/search",
		"../../internal/mcp",
	}

	checker := NewFileServiceComplianceChecker()
	checker.IgnoreTestFiles = true // Focus on production code first
	checker.IgnoreToolFiles = true

	totalViolations := 0
	errorViolations := 0

	for _, dir := range criticalDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Skipf("Directory %s does not exist, skipping", dir)
			continue
		}

		t.Logf("Checking FileService compliance in %s...", dir)
		err := checker.CheckDirectory(dir)
		if err != nil {
			t.Errorf("Error checking directory %s: %v", dir, err)
			continue
		}

		violations := checker.Violations

		if len(violations) > 0 {
			t.Logf("Found %d violations in %s:", len(violations), dir)

			for _, violation := range violations {
				t.Logf("  %s:%d - %s",
					filepath.Base(violation.File),
					violation.Line,
					violation.Message)

				if violation.Severity == "error" {
					errorViolations++
				}
			}
		}

		totalViolations += len(violations)
	}

	if errorViolations > 0 {
		t.Errorf("Found %d error-level FileService violations in critical directories", errorViolations)
		t.Errorf("These violations compromise the core architecture and must be fixed")
	} else if totalViolations > 0 {
		t.Logf("⚠️  Found %d warning-level violations - should be addressed", totalViolations)
	} else {
		t.Logf("✅ No FileService violations found in critical directories")
	}
}

// TestFileServiceCompliance_MasterIndexSpecific specifically tests MasterIndex compliance
func TestFileServiceCompliance_MasterIndexSpecific(t *testing.T) {
	// MasterIndex is the central orchestrator and must be fully compliant
	goroutineIndexPath := "../../internal/indexing/goroutine_index.go"

	if _, err := os.Stat(goroutineIndexPath); os.IsNotExist(err) {
		t.Skipf("MasterIndex file not found: %s", goroutineIndexPath)
	}

	// Read the file content and check for forbidden patterns
	content, err := os.ReadFile(goroutineIndexPath)
	if err != nil {
		t.Fatalf("Failed to read MasterIndex: %v", err)
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	// Patterns to check for
	forbiddenPatterns := map[string]string{
		"os.ReadFile":   "Use FileService.LoadFile() instead",
		"os.Stat":       "Use FileService.Exists() or GetFileSize() instead",
		"os.WriteFile":  "Use FileService.WriteFile() instead (only for config management)",
		"os.Mkdir":      "Use FileService.MkdirAll() instead",
		"filepath.Walk": "Use FileService.EstimateDirectorySize() instead",
	}

	violations := []string{}
	for lineNum, line := range lines {
		for pattern, suggestion := range forbiddenPatterns {
			if strings.Contains(line, pattern) {
				// Check if this is in a comment or string
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
					continue // Skip comments
				}

				violations = append(violations, fmt.Sprintf("Line %d: %s (%s)",
					lineNum+1, pattern, suggestion))
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("MasterIndex violates FileService abstraction:")
		for _, violation := range violations {
			t.Errorf("  %s", violation)
		}
		t.Errorf("MasterIndex must use FileService for all filesystem operations")
	} else {
		t.Logf("✅ MasterIndex complies with FileService abstraction")
	}
}

// TestFileServiceCompliance_CoreComponents checks core components for compliance
func TestFileServiceCompliance_CoreComponents(t *testing.T) {
	// Use the compliance checker with proper exceptions for legitimate use cases
	checker := NewFileServiceComplianceChecker()
	checker.IgnoreTestFiles = true
	checker.IgnoreToolFiles = true

	// Check other core components that should be compliant
	coreFiles := []string{
		"../../internal/core/file_content_store.go",
		"../../internal/core/propagation_config.go",
		"../../internal/core/persistence.go",
		"../../internal/parser/tree_sitter_parser.go",
	}

	violations := []string{}

	for _, file := range coreFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue // Skip files that don't exist
		}

		// Use the compliance checker to check this specific file
		fileViolations, err := checker.CheckFile(file)
		if err != nil {
			t.Errorf("Failed to check %s: %v", file, err)
			continue
		}

		// Only count error-level violations (warnings are allowed for legitimate exceptions)
		for _, violation := range fileViolations {
			if violation.Severity == "error" {
				violations = append(violations, fmt.Sprintf("%s:%d: %s",
					filepath.Base(file), violation.Line, violation.Message))
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("Core components have FileService abstraction violations:")
		for _, violation := range violations {
			t.Errorf("  %s", violation)
		}
	} else {
		t.Logf("✅ Core components comply with FileService abstraction")
	}
}

// TestFileServiceCompliance_DocumentationTest ensures documentation exists
func TestFileServiceCompliance_DocumentationTest(t *testing.T) {
	// Check that FileService documentation exists (relative to project root)
	docPath := "../../docs/architecture/fileservice-abstraction.md"

	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		t.Errorf("FileService abstraction documentation not found: %s", docPath)
		t.Errorf("Documentation is required for this architectural boundary")
		return
	}

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Errorf("Failed to read documentation: %v", err)
		return
	}

	contentStr := string(content)

	// Check that key sections exist
	requiredSections := []string{
		"## Purpose and Critical Importance",
		"## Strict Compliance Rules",
		"## Valid Usage Patterns",
		"## Component Responsibilities",
		"## Migration Strategy",
	}

	for _, section := range requiredSections {
		if !strings.Contains(contentStr, section) {
			t.Errorf("Documentation missing required section: %s", section)
		}
	}

	// Check that it mentions FileService prominently
	fileServiceCount := strings.Count(contentStr, "FileService")
	if fileServiceCount < 10 {
		t.Errorf("Documentation should mention FileService prominently (found %d times)", fileServiceCount)
	}

	t.Logf("✅ FileService abstraction documentation exists and is comprehensive")
}

// TestFileServiceCompliance_CliCompliance checks that CLI tool uses FileService properly
func TestFileServiceCompliance_CliCompliance(t *testing.T) {
	// Use the compliance checker with proper exceptions for CLI layer
	checker := NewFileServiceComplianceChecker()
	checker.IgnoreTestFiles = true
	checker.IgnoreToolFiles = true

	// Check that the main CLI tool uses FileService for file operations
	cliFiles := []string{
		"../../cmd/lci/main.go",
		"../../cmd/lci/search.go",
		"../../cmd/lci/definition.go",
	}

	violations := []string{}

	for _, file := range cliFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue // Skip files that don't exist
		}

		// Use the compliance checker to check this specific file
		fileViolations, err := checker.CheckFile(file)
		if err != nil {
			t.Errorf("Failed to check %s: %v", file, err)
			continue
		}

		// Only count error-level violations (warnings are allowed for CLI layer exceptions)
		for _, violation := range fileViolations {
			if violation.Severity == "error" {
				violations = append(violations, fmt.Sprintf("%s:%d: %s",
					filepath.Base(file), violation.Line, violation.Message))
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("CLI files have FileService abstraction violations:")
		for _, violation := range violations {
			t.Errorf("  %s", violation)
		}
	} else {
		t.Logf("✅ CLI tools properly delegate file operations through indexer")
	}
}
