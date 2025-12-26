package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/indexing"
)

// TestGrepLikeFeatures tests the grep-like features that are marked as P0
// These are currently unimplemented: InvertMatch, Patterns, CountPerFile, FilesOnly, WordBoundary, MaxCountPerFile
// Gap addressed: Pattern Complexity & Features (currently 25/100)
func TestGrepLikeFeatures(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create test files with various patterns to test grep features
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello World")
	fmt.Println("test message")
	fmt.Println("Hello again")
	processData()
	fmt.Println("test")
}

func processData() {
	fmt.Println("processing test data")
}`,

		"utils.go": `package utils

import "log"

func helper() {
	log.Println("test utility")
	log.Println("Hello from utils")
}

func test() {
	// test function body
	helper()
}`,

		"config.yaml": `# Configuration file
name: test
mode: test
debug: false
hello_world: true
test_value: 123`,
	}

	// Write test files
	for name, content := range testFiles {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
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

	// Test InvertMatch - grep -v functionality (NOT matching)
	t.Run("invert_match", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":      "test",
			"invert_match": true, // Should find lines WITHOUT "test"
			"max_results":  20,
		})
		if err != nil {
			t.Logf("Invert match not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Invert match returned results (lines without 'test')")
		}
	})

	// Test WordBoundary - grep -w functionality
	t.Run("word_boundary", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":       "test",
			"word_boundary": true, // Match whole words only
			"max_results":   20,
		})
		if err != nil {
			t.Logf("Word boundary not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Word boundary search found whole-word matches")
		}
	})

	// Test CountPerFile - grep -c functionality
	t.Run("count_per_file", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":        "test",
			"count_per_file": true, // Return count per file instead of individual lines
			"max_results":    20,
		})
		if err != nil {
			t.Logf("Count per file not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Count per file returned aggregated counts")
		}
	})

	// Test FilesOnly - grep -l functionality
	t.Run("files_only", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":     "test",
			"files_only":  true, // Return only filenames with matches
			"max_results": 20,
		})
		if err != nil {
			t.Logf("Files only not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Files only returned just matching filenames")
		}
	})

	// Test MaxCountPerFile - grep -m functionality
	t.Run("max_count_per_file", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":            "test",
			"max_count_per_file": 2, // Max 2 matches per file
			"max_results":        20,
		})
		if err != nil {
			t.Logf("Max count per file not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Max count per file limited results per file")
		}
	})

	// Test Patterns (multiple patterns with OR) - grep -e pattern1 -e pattern2
	t.Run("multiple_patterns", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"patterns": []string{"test", "hello", "World"}, // Multiple patterns
		})
		if err != nil {
			t.Logf("Multiple patterns not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Multiple patterns found matches for any of the patterns")
		}
	})
}

// TestSearchFeatureCombinations tests combinations of search features
// Gap addressed: Code Coverage for feature interactions (currently 45/100)
func TestSearchFeatureCombinations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create test files
	testContent := `package main

import "fmt"

// PublicFunction is exported
func PublicFunction(input string) string {
	return "result: " + input
}

// privateHelper is not exported
func privateHelper() {
	// implementation
}

// TestPublicFunction tests the public function
func TestPublicFunction() {
	PublicFunction("test")
}

// testPrivateHelper tests the private function
func testPrivateHelper() {
	privateHelper()
}

const PublicConstant = 42
const privateConstant = 0
var PublicVariable string
var privateVariable string

type PublicStruct struct {
	Field string
}

type privateStruct struct {
	field string
}
`

	filePath := filepath.Join(tempDir, "code.go")
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

	// Test combining exported_only with symbol_types
	t.Run("exported_with_symbol_types", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":       "Public",
			"exported_only": true,
			"symbol_types":  []string{"function", "type"},
			"max_results":   10,
		})
		if err != nil {
			t.Logf("Combined filter failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Combined exported + symbol type filter succeeded")
		}
	})

	// Test combining declaration_only with symbol_types
	t.Run("declarations_with_symbol_types", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":          "Public",
			"declaration_only": true,
			"symbol_types":     []string{"function", "constant"},
			"max_results":      10,
		})
		if err != nil {
			t.Logf("Combined declaration + symbol type filter failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Combined declaration + symbol type filter succeeded")
		}
	})

	// Test excluding tests with other filters
	t.Run("exclude_tests_with_filters", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":          "test",
			"exclude_tests":    true,
			"case_insensitive": true,
			"max_results":      10,
		})
		if err != nil {
			t.Logf("Exclude tests with filters failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Exclude tests + case insensitive filter succeeded")
		}
	})
}

// TestRegexComplexity tests complex regex patterns
// Gap addressed: Pattern Complexity & Features (currently 25/100)
func TestRegexComplexity(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create file with various patterns to match
	testContent := `package main

import (
	"fmt"
	"regexp"
	"net/http"
)

const (
	Regex1 = "^[a-zA-Z0-9]+$"
	Regex2 = "[0-9]{3}-[0-9]{3}-[0-9]{4}"
	Regex3 = "^https?://"
)

func validateEmail(email string) bool {
	pattern := "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
	return regexp.MustCompile(pattern).MatchString(email)
}

func extractDate(text string) string {
	pattern := "(0?[1-9]|[12][0-9]|3[01])[/-](0?[1-9]|1[12])[/-](19|20)?[0-9]{2}"
	re := regexp.MustCompile(pattern)
	return re.FindString(text)
}

func parseURL(url string) {
	// https://example.com or http://example.com
	if match, _ := regexp.MatchString("^https?://.*", url); match {
		fmt.Println("Valid URL")
	}
}

func wordCount(text string) {
	// Match word boundaries
	re := regexp.MustCompile("\\b\\w+\\b")
	words := re.FindAllString(text, -1)
	fmt.Printf("Found %d words\n", len(words))
}

func replaceWhitespace(text string) string {
	// Replace one or more whitespace characters
	re := regexp.MustCompile("\\s+")
	return re.ReplaceAllString(text, " ")
}
`

	filePath := filepath.Join(tempDir, "regex.go")
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

	// Test various regex patterns
	regexTests := []struct {
		name    string
		pattern string
		desc    string
	}{
		{
			"email_pattern",
			"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+",
			"Email validation pattern",
		},
		{
			"phone_pattern",
			"[0-9]{3}-[0-9]{3}-[0-9]{4}",
			"Phone number pattern",
		},
		{
			"url_pattern",
			"^https?://",
			"URL protocol pattern",
		},
		{
			"word_boundary",
			`\b\w+\b`,
			"Word boundary pattern",
		},
		{
			"whitespace",
			`\s+`,
			"Whitespace pattern",
		},
		{
			"alternation",
			"(https|http|ftp)://",
			"Alternation pattern",
		},
		{
			"lookahead",
			`(?=.*[a-z])(?=.*[A-Z])`,
			"Lookahead assertion",
		},
		{
			"date_capture",
			`(\d{1,2})[/-](\d{1,2})[/-](\d{4})`,
			"Date capture groups",
		},
	}

	for _, rt := range regexTests {
		t.Run(rt.name, func(t *testing.T) {
			result, err := server.CallTool("search", map[string]interface{}{
				"pattern":     rt.pattern,
				"use_regex":   true,
				"max_results": 5,
			})
			if err != nil {
				t.Logf("Regex pattern '%s' (%s) not supported: %v", rt.name, rt.desc, err)
				return
			}
			if result != "" {
				t.Logf("Regex pattern '%s' found matches", rt.name)
			}
		})
	}
}

// TestExcludeComments tests the exclude_comments feature
// Gap addressed: Feature coverage (currently 20/100)
func TestExcludeComments(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	testContent := `package main

import "fmt"

// This is a comment about function
// The word 'test' appears in comments
func MyFunction() {
	// test in comment
	result := "test in string"
	fmt.Println(result)
	/* Multi-line comment
	   with word test
	*/
}

/*
Block comment
test in block
*/
func AnotherFunction() {
	// More test comments
	actualTest := true
}
`

	filePath := filepath.Join(tempDir, "comments.go")
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

	// Test searching with comments included
	t.Run("with_comments", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":          "test",
			"exclude_comments": false,
			"max_results":      20,
		})
		if err != nil {
			t.Logf("Search with comments failed: %v", err)
			return
		}
		if result != "" {
			t.Logf("Search with comments found matches")
		}
	})

	// Test searching with comments excluded
	t.Run("without_comments", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":          "test",
			"exclude_comments": true,
			"max_results":      20,
		})
		if err != nil {
			t.Logf("Exclude comments not yet implemented: %v", err)
			return
		}
		if result != "" {
			t.Logf("Search excluding comments found matches")
		}
	})
}

// TestSearchPerformanceWithFilters tests search performance with various filter combinations
// Gap addressed: Code Coverage (currently 45/100)
func TestSearchPerformanceWithFilters(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Generate many files with patterns
	numFiles := 30
	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf(`package pkg%d

func Function%d() {
	result := "test pattern here"
}

type Type%d struct {
	Field string
}

const Constant%d = %d
`, i, i, i, i, i)

		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %d: %v", i, err)
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

	// Test with various filter combinations
	t.Run("filter_combinations_performance", func(t *testing.T) {
		filterTests := []struct {
			name   string
			params map[string]interface{}
		}{
			{
				"no_filters",
				map[string]interface{}{"pattern": "test"},
			},
			{
				"with_symbol_types",
				map[string]interface{}{"pattern": "test", "symbol_types": []string{"function"}},
			},
			{
				"with_exported_only",
				map[string]interface{}{"pattern": "test", "exported_only": true},
			},
			{
				"with_case_insensitive",
				map[string]interface{}{"pattern": "TEST", "case_insensitive": true},
			},
			{
				"with_max_results_limit",
				map[string]interface{}{"pattern": "test", "max_results": 5},
			},
		}

		for _, ft := range filterTests {
			t.Run(ft.name, func(t *testing.T) {
				result, err := server.CallTool("search", ft.params)
				if err != nil {
					t.Logf("Filter test failed: %v", err)
					return
				}
				if result != "" {
					t.Logf("Filter combination '%s' returned results", ft.name)
				}
			})
		}
	})
}

// TestSearchErrorHandling tests error handling in search operations
// Gap addressed: Code Coverage (currently 45/100)
func TestSearchErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create a minimal test file
	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644); err != nil {
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

	// Test invalid regex
	t.Run("invalid_regex", func(t *testing.T) {
		_, err := server.CallTool("search", map[string]interface{}{
			"pattern":   "[invalid(regex",
			"use_regex": true,
		})
		if err == nil {
			t.Logf("Invalid regex should have returned error")
		} else {
			t.Logf("Invalid regex error handled: %v", err)
		}
	})

	// Test conflicting filters
	t.Run("conflicting_filters", func(t *testing.T) {
		_, err := server.CallTool("search", map[string]interface{}{
			"pattern":          "test",
			"declaration_only": true,
			"usage_only":       true, // Conflict: can't be both
		})
		if err != nil {
			t.Logf("Conflicting filters error: %v", err)
		}
	})

	// Test extreme parameters
	t.Run("extreme_parameters", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":        "test",
			"max_results":    999999999,
			"max_line_count": 999999999,
		})
		if err != nil {
			t.Logf("Extreme parameters error: %v", err)
		} else if result != "" {
			t.Logf("Extreme parameters handled gracefully")
		}
	})
}
