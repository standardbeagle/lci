package testing

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FileServiceComplianceChecker validates that FileService abstraction is respected
type FileServiceComplianceChecker struct {
	AllowedPackages   []string
	ForbiddenPatterns []*regexp.Regexp
	Exceptions        map[string]bool // file paths that are allowed to violate rules
	Violations        []Violation
	IgnoreTestFiles   bool
	IgnoreToolFiles   bool

	// Performance optimization: cache file contents to avoid repeated I/O
	fileContentCache map[string][]string
}

// Violation represents a FileService abstraction violation
type Violation struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Pattern  string `json:"pattern"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning", "info"
}

// NewFileServiceComplianceChecker creates a new compliance checker
func NewFileServiceComplianceChecker() *FileServiceComplianceChecker {
	checker := &FileServiceComplianceChecker{
		AllowedPackages: []string{
			"github.com/standardbeagle/lci/internal/core",
		},
		fileContentCache: make(map[string][]string),
		Exceptions: map[string]bool{
			// Test files are allowed to use direct filesystem for setup
			"_test.go": true,
			// Test infrastructure directories (anything in these can use filesystem)
			"internal/testing/":   true, // All test infrastructure and helpers
			"testing/":            true, // Top-level testing directory
			"workflow_scenarios/": true, // MCP workflow test scenarios
			"workflow_testdata/":  true, // MCP workflow test data
			"testdata/":           true, // General test data
			"testhelpers/":        true, // Test helper utilities
			"tools/":              true, // Tools directory is outside core system
			"docs/":               true, // Documentation examples
			"investigation/":      true, // Investigation and experimental code
			"requests/":           true, // Request tracking and analysis
			// Package-specific test helpers (must be in same package to access unexported types)
			"test_helpers.go":      true, // Test helper functions
			"workflow_helpers.go":  true, // MCP workflow test infrastructure
			"workflow_snapshot.go": true, // MCP workflow snapshot infrastructure
			// FileService itself is allowed to use os.* operations
			"file_service.go":       true,
			"file_content_store.go": true,
			"file_loader.go":        true,
			// Pipeline and watcher components (allowed to use filesystem for file operations)
			"pipeline.go":         true,
			"pipeline_scanner.go": true, // Binary pre-check optimization needs direct file access
			"watcher.go":          true,
			// Persistence layer is allowed to use filesystem for index persistence
			"persistence.go": true,
			// PropagationConfigManager has fallback filesystem access for compatibility
			"propagation_config.go": true,
			// CLI layer is allowed to use direct filesystem for configuration, profiling, and I/O
			"cmd/":    true,
			"main.go": true,
			// Known architectural limitations
			"js_resolver.go": true, // JS resolver needs package.json access, FileService integration planned
			"kdl_config.go":  true, // Config reading is legitimate direct filesystem use
			// MCP diagnostic logging writes to system temp/home directories before FileService initialization
			"diagnostics.go": true, // MCP diagnostic infrastructure - must work before FileService init
			// Context manifest tool writes/reads manifest files for agent handoff, outside indexed project scope
			"context_manifest_tool.go": true, // MCP context manifest - cross-session portable metadata files
		},
		IgnoreTestFiles: true,
		IgnoreToolFiles: true,
	}

	// Forbidden filesystem patterns
	checker.ForbiddenPatterns = []*regexp.Regexp{
		// Direct os package usage (word boundaries to avoid partial matches)
		regexp.MustCompile(`\bos\.(ReadFile|Stat|WriteFile|Mkdir|MkdirAll|Open|OpenFile)\b`),
		// ioutil package usage (deprecated but still possible)
		regexp.MustCompile(`\bioutil\.(ReadFile|WriteFile|ReadDir|TempDir)\b`),
		// Direct filepath.Walk usage (should use FileService.EstimateDirectorySize)
		regexp.MustCompile(`\bfilepath\.Walk`),
		// Direct file handle operations
		regexp.MustCompile(`\bos\.File\{`),
	}

	return checker
}

// CheckDirectory checks all Go files in a directory for FileService compliance
func (c *FileServiceComplianceChecker) CheckDirectory(rootDir string) error {
	c.Violations = []Violation{}
	// Clear cache to prevent memory leaks
	c.fileContentCache = make(map[string][]string)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only check Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check if file should be ignored
		if c.shouldIgnoreFile(path, rootDir) {
			return nil
		}

		// Check the file
		violations, err := c.CheckFile(path)
		if err != nil {
			return fmt.Errorf("error checking file %s: %w", path, err)
		}

		c.Violations = append(c.Violations, violations...)
		return nil
	})

	return err
}

// CheckFile checks a single Go file for FileService compliance violations
func (c *FileServiceComplianceChecker) CheckFile(filePath string) ([]Violation, error) {
	// Check if file should be ignored (use current directory as root for relative paths)
	if c.shouldIgnoreFile(filePath, ".") {
		return []Violation{}, nil
	}

	var violations []Violation

	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file %s: %w", filePath, err)
	}

	// Walk the AST looking for violations
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Check each node against forbidden patterns
		for _, pattern := range c.ForbiddenPatterns {
			if violation := c.checkNodeForPattern(fset, n, pattern, filePath); violation != nil {
				violations = append(violations, *violation)
			}
		}

		return true
	})

	return violations, nil
}

// checkNodeForPattern checks if an AST node matches a forbidden pattern
func (c *FileServiceComplianceChecker) checkNodeForPattern(fset *token.FileSet, node ast.Node, pattern *regexp.Regexp, filePath string) *Violation {
	// Get the source code for this node
	pos := fset.Position(node.Pos())

	// Get file content from cache or read if not cached
	lines, ok := c.fileContentCache[filePath]
	if !ok {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}
		lines = strings.Split(string(content), "\n")
		c.fileContentCache[filePath] = lines
	}

	if pos.Line <= 0 || pos.Line > len(lines) {
		return nil
	}

	sourceLine := lines[pos.Line-1]

	// Find all matches in the line and check if any match our position
	matches := pattern.FindAllStringIndex(sourceLine, -1)
	for _, match := range matches {
		// Check if this match is at or near our cursor position
		matchStart := match[0] + 1 // Convert to 1-based
		matchEnd := match[1] + 1

		if pos.Column >= matchStart && pos.Column <= matchEnd {
			// Extract the code snippet around the violation
			start := max(0, match[0]-10)
			end := min(len(sourceLine), match[1]+10)
			code := sourceLine[start:end]

			// Determine severity based on pattern
			severity := "error"
			message := "Direct filesystem access detected"

			if strings.Contains(pattern.String(), "filepath.Walk") {
				message = "Use FileService.EstimateDirectorySize instead of filepath.Walk"
				severity = "warning"
			} else if strings.Contains(pattern.String(), "ioutil") {
				message = "Use FileService methods instead of deprecated ioutil package"
				severity = "warning"
			}

			return &Violation{
				File:     filePath,
				Line:     pos.Line,
				Column:   matchStart, // Use the actual match position
				Pattern:  pattern.String(),
				Code:     code,
				Message:  message,
				Severity: severity,
			}
		}
	}

	return nil
}

// shouldIgnoreFile determines if a file should be ignored during compliance checking
func (c *FileServiceComplianceChecker) shouldIgnoreFile(filePath, rootDir string) bool {
	// Get relative path from root
	relPath, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		relPath = filePath
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	// Check exceptions
	for exception := range c.Exceptions {
		if strings.Contains(relPath, exception) {
			return true
		}
	}

	// Ignore test files if configured
	if c.IgnoreTestFiles && strings.HasSuffix(relPath, "_test.go") {
		return true
	}

	// Ignore tool files if configured
	if c.IgnoreToolFiles && strings.HasPrefix(relPath, "tools/") {
		return true
	}

	return false
}

// PrintViolations prints all found violations in a readable format
func (c *FileServiceComplianceChecker) PrintViolations() {
	if len(c.Violations) == 0 {
		fmt.Println("✅ No FileService abstraction violations found!")
		return
	}

	fmt.Printf("🚫 Found %d FileService abstraction violations:\n\n", len(c.Violations))

	// Group violations by severity
	errors := []Violation{}
	warnings := []Violation{}

	for _, violation := range c.Violations {
		if violation.Severity == "error" {
			errors = append(errors, violation)
		} else {
			warnings = append(warnings, violation)
		}
	}

	// Print errors first
	if len(errors) > 0 {
		fmt.Printf("❌ ERRORS (%d):\n", len(errors))
		for _, violation := range errors {
			fmt.Printf("   %s:%d:%d - %s\n", violation.File, violation.Line, violation.Column, violation.Message)
			fmt.Printf("      Code: %s\n", violation.Code)
		}
		fmt.Println()
	}

	// Print warnings
	if len(warnings) > 0 {
		fmt.Printf("⚠️  WARNINGS (%d):\n", len(warnings))
		for _, violation := range warnings {
			fmt.Printf("   %s:%d:%d - %s\n", violation.File, violation.Line, violation.Column, violation.Message)
			fmt.Printf("      Code: %s\n", violation.Code)
		}
		fmt.Println()
	}

	fmt.Printf("📖 See %s for FileService usage guidelines\n", "../../docs/architecture/fileservice-abstraction.md")
}

// HasErrors returns true if there are error-level violations
func (c *FileServiceComplianceChecker) HasErrors() bool {
	for _, violation := range c.Violations {
		if violation.Severity == "error" {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are warning-level violations
func (c *FileServiceComplianceChecker) HasWarnings() bool {
	for _, violation := range c.Violations {
		if violation.Severity == "warning" {
			return true
		}
	}
	return false
}

// GetViolationCount returns the total number of violations by severity
func (c *FileServiceComplianceChecker) GetViolationCount() (errors, warnings, total int) {
	for _, violation := range c.Violations {
		total++
		if violation.Severity == "error" {
			errors++
		} else if violation.Severity == "warning" {
			warnings++
		}
	}
	return errors, warnings, total
}

// ClearCache clears the file content cache to free memory
func (c *FileServiceComplianceChecker) ClearCache() {
	c.fileContentCache = make(map[string][]string)
}

// AssertNoFailures fails the test if any violations are found
func (c *FileServiceComplianceChecker) AssertNoFailures(t *testing.T) {
	if len(c.Violations) == 0 {
		return
	}

	errors, warnings, total := c.GetViolationCount()
	t.Errorf("FileService abstraction compliance check failed:")
	t.Errorf("  Found %d violations (%d errors, %d warnings)", total, errors, warnings)

	if errors > 0 {
		t.Error("  ERROR-LEVEL VIOLATIONS:")
		for _, violation := range c.Violations {
			if violation.Severity == "error" {
				t.Errorf("    %s:%d:%d - %s", violation.File, violation.Line, violation.Column, violation.Message)
			}
		}
	}

	if warnings > 0 {
		t.Error("  WARNING-LEVEL VIOLATIONS:")
		for _, violation := range c.Violations {
			if violation.Severity == "warning" {
				t.Errorf("    %s:%d:%d - %s", violation.File, violation.Line, violation.Column, violation.Message)
			}
		}
	}

	t.Error("  Fix these violations to comply with FileService abstraction requirements")
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CheckFileServiceCompliance is a convenience function for testing
func CheckFileServiceCompliance(t *testing.T, rootDir string) {
	checker := NewFileServiceComplianceChecker()
	err := checker.CheckDirectory(rootDir)
	if err != nil {
		t.Fatalf("Failed to check FileService compliance: %v", err)
	}

	checker.AssertNoFailures(t)
}

// Example usage in tests:
//
// func TestFileServiceCompliance(t *testing.T) {
//     // Check the entire codebase for FileService compliance
//     testing.CheckFileServiceCompliance(t, ".")
// }
//
// func TestSpecificDirectoryCompliance(t *testing.T) {
//     checker := testing.NewFileServiceComplianceChecker()
//     err := checker.CheckDirectory("./internal/indexing")
//     if err != nil {
//         t.Fatalf("Failed to check compliance: %v", err)
//     }
//     checker.AssertNoFailures(t)
// }
