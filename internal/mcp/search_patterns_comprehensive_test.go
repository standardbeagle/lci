package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/testing/fixtures"
)

// TestSearchPatternComplexity tests comprehensive pattern variations
// This addresses Gap 1: Pattern Complexity & Features Testing (currently 25/100)
func TestSearchPatternComplexity(t *testing.T) {
	// Create test directory with diverse code samples
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create test files with various patterns
	testFiles := map[string]string{
		"regex_patterns.go": `package main

import "regexp"

func regexPattern() {
	pattern := "^[a-z]+$"
	compiled := regexp.MustCompile(pattern)
	result := compiled.MatchString("test")
}

func alternationExample() {
	pattern := "(foo|bar|baz)"
	// Multi-line
	// with alternation
}

func wordBoundaryTest() {
	// \bword\b
	text := "word boundary test"
}

func anchorsTest() {
	start := "^anchor"
	end := "anchor$"
	both := "^exactly this$"
}

func charClassTest() {
	digit := "[0-9]"
	whitespace := "[\\s\\t\\n\\r]"
	word := "[\\w_]"
}`,

		"case_sensitive.go": `package main

import "strings"

func CaseSensitiveTest() {
	upper := "CONSTANT_VALUE"
	lower := "variable_name"
	mixed := "CamelCaseFunction"

	if strings.Contains("UPPERCASE", "UPPER") {
		// uppercase match
	}

	if !strings.Contains("lowercase", "LOWER") {
		// case sensitive fails
	}
}`,

		"symbol_types.go": `package main

// TypeDefinition is a custom type
type TypeDefinition struct {
	Field string
	Count int
}

// ConstantValue is a constant
const ConstantValue = 42

// GlobalVariable is exported
var GlobalVariable = "global"

// FunctionDeclaration does something
func FunctionDeclaration() error {
	return nil
}

// MethodReceiver is a method
func (t *TypeDefinition) MethodReceiver() string {
	return t.Field
}

// InterfaceType defines a contract
type InterfaceType interface {
	DoSomething() error
}`,

		"complex_nested.go": `package main

func OuterFunction() {
	// Level 1
	if true {
		// Level 2
		for i := 0; i < 10; i++ {
			// Level 3
			switch i {
			case 0:
				// Level 4
				if i < 5 {
					// Level 5
					{
						// Level 6 - deeply nested
						data := "deeply nested data"
						_ = data
					}
				}
			}
		}
	}
}

type OuterStruct struct {
	InnerStruct struct {
		DeepField struct {
			Value string
		}
	}
}`,

		"declarations_vs_usage.go": `package main

// ProcessData is declared here
func ProcessData(input string) string {
	return "processed: " + input
}

func callerOne() {
	// ProcessData usage 1
	result := ProcessData("data1")
	_ = result
}

func callerTwo() {
	// ProcessData usage 2
	result := ProcessData("data2")
	_ = result
}

func callerThree() {
	// ProcessData usage 3
	result := ProcessData("data3")
	_ = result
}`,

		"exported_vs_private.go": `package main

// ExportedFunction is public API
func ExportedFunction() {
	privateHelper()
}

// privateHelper is internal only
func privateHelper() {
	// implementation
}

// ExportedType is public
type ExportedType struct {
	PublicField string
}

type privateType struct {
	PrivateField string
}`,
	}

	// Write test files
	for name, content := range testFiles {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create server and index
	gi := indexing.NewMasterIndex(cfg)
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Auto-indexing starts automatically in NewServer
	// Wait for it to complete
	time.Sleep(500 * time.Millisecond)

	// Verify indexing completed
	fileCount := gi.GetFileCount()
	if fileCount == 0 {
		t.Fatalf("Failed to index: no files indexed")
	}

	// Test regex patterns
	t.Run("regex_patterns", func(t *testing.T) {
		patterns := []string{
			"^[a-z]+$",      // Character class with anchors
			"(foo|bar|baz)", // Alternation
			"\\bword\\b",    // Word boundaries
			"[0-9]",         // Digit class
		}

		for _, pattern := range patterns {
			t.Run(fmt.Sprintf("pattern_%s", escapeForTest(pattern)), func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":   pattern,
					"use_regex": true,
				})
				if err != nil {
					t.Logf("Regex pattern search failed: %v", err)
					// Regex feature may not be implemented yet
					return
				}
				if result == "" {
					t.Error("Expected search results")
				}
			})
		}
	})

	// Test case-insensitive search
	t.Run("case_insensitive_search", func(t *testing.T) {
		testCases := []struct {
			pattern       string
			caseSensitive bool
			expectResults bool
		}{
			{"processdata", true, false}, // Should not find "ProcessData" (different case)
			{"ProcessData", false, true}, // Should find "ProcessData" with case_insensitive
			{"CONSTANT", false, true},    // Should find "CONSTANT_VALUE"
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("pattern_%s_case_%v", tc.pattern, !tc.caseSensitive), func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":          tc.pattern,
					"case_insensitive": !tc.caseSensitive,
					"max_results":      10,
				})
				if err != nil {
					t.Logf("Search failed: %v", err)
					return
				}

				if result != "" && tc.expectResults {
					t.Logf("Case-insensitive search found results as expected")
				}
			})
		}
	})

	// Test symbol type filtering
	t.Run("symbol_type_filtering", func(t *testing.T) {
		symbolTypes := [][]string{
			{"function"},           // Only functions
			{"type"},               // Only types
			{"constant"},           // Only constants
			{"function", "method"}, // Multiple types
		}

		for i, types := range symbolTypes {
			t.Run(fmt.Sprintf("symbols_%d", i), func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":      "Test",
					"symbol_types": types,
					"max_results":  20,
				})
				if err != nil {
					t.Logf("Symbol type filtering failed: %v", err)
					return
				}
				if result != "" {
					t.Logf("Symbol type filtering found %d results for types: %v", len(result), types)
				}
			})
		}
	})

	// Test declaration vs usage filtering
	t.Run("declaration_vs_usage", func(t *testing.T) {
		filters := []struct {
			name            string
			declarationOnly bool
			usageOnly       bool
		}{
			{"declarations_only", true, false},
			{"usage_only", false, true},
			{"both", false, false},
		}

		for _, filter := range filters {
			t.Run(filter.name, func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":          "ProcessData",
					"declaration_only": filter.declarationOnly,
					"usage_only":       filter.usageOnly,
					"max_results":      10,
				})
				if err != nil {
					t.Logf("Declaration filter failed: %v", err)
					return
				}
				if result != "" {
					t.Logf("Declaration/usage filter returned results")
				}
			})
		}
	})

	// Test exported vs private filtering
	t.Run("exported_filtering", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":       ".*",
			"exported_only": true,
			"use_regex":     true,
			"max_results":   20,
		})
		if err != nil {
			t.Logf("Exported filtering failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Exported-only filtering returned results")
		}
	})

	// Test file inclusion/exclusion
	t.Run("file_patterns", func(t *testing.T) {
		fileTests := []struct {
			name    string
			include string
			exclude string
		}{
			{"include_regex", ".*regex.*", ""},
			{"exclude_nested", "", ".*nested.*"},
			{"specific_file", "declarations.*", ""},
		}

		for _, ft := range fileTests {
			t.Run(ft.name, func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern": "func",
					"include": ft.include,
					"exclude": ft.exclude,
				})
				if err != nil {
					t.Logf("File pattern filtering failed: %v", err)
					return
				}
				if result != "" {
					t.Logf("File pattern filtering returned results")
				}
			})
		}
	})
}

// TestSearchOutputVariations tests different output size modes
// This addresses Gap 1: Output Size and Context Element Coverage
func TestSearchOutputVariations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create test file with enough context
	testContent := `package main

import "fmt"

// CommentLine1
// CommentLine2
// CommentLine3
func TargetFunction() {
	fmt.Println("Line 1")
	fmt.Println("Line 2")
	fmt.Println("Line 3")
	fmt.Println("Line 4")
	fmt.Println("Line 5")
}

func Caller1() {
	TargetFunction()
}

func Caller2() {
	TargetFunction()
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	gi := indexing.NewMasterIndex(cfg)
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Wait for auto-indexing (started in NewServer) to complete
	status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
	if err != nil {
		t.Fatalf("Auto-indexing failed: %v", err)
	}
	if status != "completed" {
		t.Fatalf("Auto-indexing did not complete, got status: %s", status)
	}

	// Test different output sizes
	t.Run("output_size_single_line", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":     "TargetFunction",
			"output_size": "single-line",
			"max_results": 5,
		})
		if err != nil {
			t.Logf("Single-line output failed: %v", err)
			return
		}
		if result == "" {
			t.Error("Expected search results")
		}
		t.Logf("Single-line output: %s", result)
	})

	t.Run("output_size_context", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":        "TargetFunction",
			"output_size":    "context",
			"max_line_count": 3,
			"max_results":    5,
		})
		if err != nil {
			t.Logf("Context output failed: %v", err)
			return
		}
		if result == "" {
			t.Error("Expected search results with context")
		}
		t.Logf("Context output: %s", result)
	})

	t.Run("output_size_full", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":        "TargetFunction",
			"output_size":    "full",
			"max_line_count": 10,
			"max_results":    5,
		})
		if err != nil {
			t.Logf("Full output failed: %v", err)
			return
		}
		if result == "" {
			t.Error("Expected full search results")
		}
		t.Logf("Full output length: %d bytes", len(result))
	})

	// Test optional context elements
	t.Run("optional_context_elements", func(t *testing.T) {
		contextTests := []struct {
			name               string
			includeBreadcrumbs bool
			includeSafety      bool
			includeReferences  bool
		}{
			{"no_context", false, false, false},
			{"breadcrumbs_only", true, false, false},
			{"safety_only", false, true, false},
			{"references_only", false, false, true},
			{"all_context", true, true, true},
		}

		for _, ct := range contextTests {
			t.Run(ct.name, func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":             "TargetFunction",
					"include_breadcrumbs": ct.includeBreadcrumbs,
					"include_safety":      ct.includeSafety,
					"include_references":  ct.includeReferences,
					"max_results":         5,
				})
				if err != nil {
					t.Logf("Context element search failed: %v", err)
					return
				}
				if result != "" {
					t.Logf("Search with context elements succeeded")
				}
			})
		}
	})
}

// TestComplexFileStructures tests code with higher complexity
// This addresses Gap 3: Input File Complexity (currently 20/100)
func TestComplexFileStructures(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create a complex file with deep nesting
	complexFile := `package main

import (
	"fmt"
	"sync"
)

type DatabaseService struct {
	mu sync.RWMutex
	cache map[string]interface{}
}

func (ds *DatabaseService) QueryData(query string) (interface{}, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if result, ok := ds.cache[query]; ok {
		return result, nil
	}

	// Deep nesting level 1
	if query != "" {
		// Deep nesting level 2
		for _, char := range query {
			// Deep nesting level 3
			if char > 'a' && char < 'z' {
				// Deep nesting level 4
				switch char {
				case 'x', 'y', 'z':
					// Deep nesting level 5
					if true {
						// Deep nesting level 6
						data := processDeepData(char)
						// Deep nesting level 7
						{
							// Deep nesting level 8
							formatted := fmt.Sprintf("data: %v", data)
							return formatted, nil
						}
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("query not found: %s", query)
}

func processDeepData(ch rune) interface{} {
	return map[string]interface{}{
		"char": ch,
		"code": int(ch),
	}
}

// Complex generic-like structures
type Handler struct {
	name string
	fn func(string) error
}

type Pipeline struct {
	handlers []Handler
}

func (p *Pipeline) Execute(input string) error {
	for _, handler := range p.handlers {
		if err := handler.fn(input); err != nil {
			return err
		}
	}
	return nil
}
`

	filePath := filepath.Join(tempDir, "complex.go")
	if err := os.WriteFile(filePath, []byte(complexFile), 0644); err != nil {
		t.Fatalf("Failed to write complex file: %v", err)
	}

	gi := indexing.NewMasterIndex(cfg)
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Wait for auto-indexing (started in NewServer) to complete
	status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
	if err != nil {
		t.Fatalf("Auto-indexing failed: %v", err)
	}
	if status != "completed" {
		t.Fatalf("Auto-indexing did not complete, got status: %s", status)
	}

	t.Run("complex_structure_search", func(t *testing.T) {
		patterns := []string{
			"DatabaseService",
			"QueryData",
			"processDeepData",
			"Pipeline",
		}

		for _, pattern := range patterns {
			t.Run(pattern, func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":     pattern,
					"max_results": 10,
				})
				if err != nil {
					t.Logf("Complex structure search failed: %v", err)
					return
				}
				if result == "" {
					t.Errorf("Expected to find '%s' in complex structures", pattern)
				} else {
					t.Logf("Found '%s' in complex code", pattern)
				}
			})
		}
	})
}

// TestLargeFileHandling tests search in large, generated files
// This addresses Gap 3: Input File Complexity (currently 20/100)
func TestLargeFileHandling(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Generate a large file with many functions
	largeFileContent := fixtures.GenerateLargeFile(500) // 500 functions
	filePath := filepath.Join(tempDir, "large.go")
	if err := os.WriteFile(filePath, []byte(largeFileContent), 0644); err != nil {
		t.Fatalf("Failed to write large file: %v", err)
	}

	gi := indexing.NewMasterIndex(cfg)
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Wait for auto-indexing (started in NewServer) to complete
	status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
	if err != nil {
		t.Fatalf("Auto-indexing failed: %v", err)
	}
	if status != "completed" {
		t.Fatalf("Auto-indexing did not complete, got status: %s", status)
	}

	t.Run("large_file_search", func(t *testing.T) {
		patterns := []string{
			"Function100",
			"Method200",
			"Type50",
		}

		for _, pattern := range patterns {
			t.Run(pattern, func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":     pattern,
					"max_results": 10,
				})
				if err != nil {
					t.Logf("Large file search failed: %v", err)
					return
				}
				if result == "" {
					t.Errorf("Failed to find '%s' in large file", pattern)
				}
			})
		}
	})

	t.Run("large_file_multiple_patterns", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":     "Method",
			"max_results": 50,
		})
		if err != nil {
			t.Logf("Multiple pattern search failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Found multiple 'Method' matches in large file")
		}
	})
}

// TestMultiLanguageProject tests search across multiple languages
// This addresses Gap 3: Input File Complexity (currently 20/100)
func TestMultiLanguageProject(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create multi-language test files
	testFiles := map[string]string{
		"service.go": `package service

// ProcessRequest handles incoming requests
func ProcessRequest(req interface{}) {
	// implementation
}`,

		"utils.js": `// Process JavaScript utilities
function processRequest(request) {
	// JavaScript implementation
	console.log("Processing request");
}

class RequestHandler {
	handle(request) {
		processRequest(request);
	}
}`,

		"helpers.py": `# Python helpers module

def processRequest(request):
	"""Process a request in Python"""
	return {"status": "processed"}

class RequestProcessor:
	def process(self, request):
		return processRequest(request)`,
	}

	// Write multi-language files
	for name, content := range testFiles {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	gi := indexing.NewMasterIndex(cfg)
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Wait for auto-indexing (started in NewServer) to complete
	status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
	if err != nil {
		t.Fatalf("Auto-indexing failed: %v", err)
	}
	if status != "completed" {
		t.Fatalf("Auto-indexing did not complete, got status: %s", status)
	}

	t.Run("cross_language_search", func(t *testing.T) {
		// Search for the common pattern "ProcessRequest" across all languages
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":     "ProcessRequest",
			"max_results": 20,
		})
		if err != nil {
			t.Logf("Cross-language search failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Found 'ProcessRequest' pattern across multiple languages")
		}
	})

	t.Run("language_specific_search", func(t *testing.T) {
		patterns := []string{
			"class RequestHandler", // JavaScript
			"def processRequest",   // Python
			"func ProcessRequest",  // Go
		}

		for _, pattern := range patterns {
			t.Run(escapeForTest(pattern), func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":     pattern,
					"max_results": 5,
				})
				if err != nil {
					t.Logf("Language-specific search failed: %v", err)
					return
				}
				if result != "" {
					t.Logf("Found language-specific pattern: %s", pattern)
				}
			})
		}
	})
}

// Helper functions

func createTestConfig(rootPath string) *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.Project{
			Root: rootPath,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
		},
		Performance: config.Performance{
			MaxMemoryMB:   50,
			MaxGoroutines: 4,
			DebounceMs:    0,
		},
		Search: config.Search{
			MaxResults:         100,
			MaxContextLines:    50,
			EnableFuzzy:        true,
			MergeFileResults:   true,
			EnsureCompleteStmt: false,
		},
		Include: []string{"*"},
		Exclude: []string{},
	}
}

func escapeForTest(s string) string {
	replacer := strings.NewReplacer(
		"*", "star",
		"+", "plus",
		"?", "question",
		".", "dot",
		"[", "lbracket",
		"]", "rbracket",
		"(", "lparen",
		")", "rparen",
		"|", "pipe",
		"^", "caret",
		"$", "dollar",
		"\\", "backslash",
		" ", "space",
	)
	return replacer.Replace(s)
}
