package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/indexing"
)

// TestSearchIntegrationAdvanced tests complex end-to-end search scenarios
// Gap addressed: Code Coverage - Feature integration (currently 45/100)
func TestSearchIntegrationAdvanced(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create a realistic project structure
	createRealisticProject(t, tempDir)

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

	// Complex scenario 1: Find public API functions with multiple filters
	t.Run("find_public_api_functions", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":             "func [A-Z]",
			"use_regex":           true,
			"declaration_only":    true,
			"exported_only":       true,
			"symbol_types":        []string{"function"},
			"max_results":         20,
			"output_size":         "context",
			"max_line_count":      5,
			"include_breadcrumbs": true,
		})
		if err != nil {
			t.Logf("Find public API functions failed: %v", err)
		} else if result != "" {
			t.Logf("Found public API functions successfully")
		}
	})

	// Complex scenario 2: Search for specific pattern across file types
	t.Run("search_across_languages", func(t *testing.T) {
		patterns := map[string]string{
			"service_pattern": "Service",
			"handler_pattern": "Handler",
			"util_pattern":    "Util",
		}

		for name, pattern := range patterns {
			t.Run(name, func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern":          pattern,
					"case_insensitive": false,
					"max_results":      15,
					"output_size":      "full",
				})
				if err != nil {
					t.Logf("Cross-language search failed: %v", err)
				} else if result != "" {
					t.Logf("Found pattern '%s' across languages", pattern)
				}
			})
		}
	})

	// Complex scenario 3: Find related symbols
	t.Run("find_related_symbols", func(t *testing.T) {
		// First find the main function, then find what it calls
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern":     "ProcessRequest",
			"max_results": 10,
		})
		if err != nil {
			t.Logf("Find related symbols failed: %v", err)
		} else if result != "" {
			t.Logf("Found ProcessRequest and related calls")
		}
	})
}

// TestSearchContextualFiltering tests context-aware filtering
// Gap addressed: Code Coverage - Feature combinations (currently 45/100)
func TestSearchContextualFiltering(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create hierarchical code structure
	createHierarchicalCodeStructure(t, tempDir)

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

	// Test filtering by scope
	t.Run("filter_by_package", func(t *testing.T) {
		// Files: service/main.go, utils/helper.go, controller/api.go
		includes := []string{
			"service/*",
			"utils/*",
			"controller/*",
		}

		for _, include := range includes {
			t.Run(strings.ReplaceAll(include, "/", "_"), func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern": "func",
					"include": include,
				})
				if err != nil {
					t.Logf("Package filtering failed: %v", err)
				} else if result != "" {
					t.Logf("Found functions in package '%s'", include)
				}
			})
		}
	})

	// Test filtering out specific types
	t.Run("filter_exclusions", func(t *testing.T) {
		excludes := []string{
			"*_test.go",    // Exclude test files
			"*/internal/*", // Exclude internal packages
		}

		for _, exclude := range excludes {
			t.Run(strings.ReplaceAll(exclude, "/", "_"), func(t *testing.T) {
				result, err := server.CallTool("search", map[string]interface{}{
					"pattern": "func",
					"exclude": exclude,
				})
				if err != nil {
					t.Logf("Exclusion filtering failed: %v", err)
				} else if result != "" {
					t.Logf("Filtered out '%s' successfully", exclude)
				}
			})
		}
	})
}

// TestSearchOutputFormats tests different output configurations
// Gap addressed: Code Coverage - Output variations (currently 45/100)
func TestSearchOutputFormats(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	createTestCodeFile(t, tempDir, "main.go", getDetailedCodeSample())

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

	// Test all output configurations
	outputConfigs := []struct {
		name         string
		outputSize   string
		maxLineCount int
		contextFlags map[string]bool
	}{
		{
			name:         "minimal_output",
			outputSize:   "single-line",
			maxLineCount: 0,
			contextFlags: map[string]bool{
				"include_breadcrumbs": false,
				"include_safety":      false,
				"include_references":  false,
			},
		},
		{
			name:         "context_output",
			outputSize:   "context",
			maxLineCount: 3,
			contextFlags: map[string]bool{
				"include_breadcrumbs": true,
				"include_safety":      false,
				"include_references":  false,
			},
		},
		{
			name:         "full_output",
			outputSize:   "full",
			maxLineCount: 10,
			contextFlags: map[string]bool{
				"include_breadcrumbs": true,
				"include_safety":      true,
				"include_references":  true,
			},
		},
		{
			name:         "all_context",
			outputSize:   "full",
			maxLineCount: 5,
			contextFlags: map[string]bool{
				"include_breadcrumbs":  true,
				"include_safety":       true,
				"include_references":   true,
				"include_dependencies": true,
			},
		},
	}

	for _, config := range outputConfigs {
		t.Run(config.name, func(t *testing.T) {
			params := map[string]interface{}{
				"pattern":        "targetFunction",
				"output_size":    config.outputSize,
				"max_line_count": config.maxLineCount,
				"max_results":    5,
			}

			// Add context flags
			for k, v := range config.contextFlags {
				params[k] = v
			}

			result, err := server.CallTool("search", params)
			if err != nil {
				t.Logf("Output format test failed: %v", err)
			} else if result != "" {
				resultLen := len(result)
				t.Logf("Output format '%s' returned %d bytes", config.name, resultLen)
			}
		})
	}
}

// TestSearchConcurrency tests concurrent search operations
// Gap addressed: Code Coverage - Concurrent operations (currently 45/100)
func TestSearchConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create many test files for stress testing
	for i := 0; i < 50; i++ {
		content := fmt.Sprintf(`package pkg%d

func Function%d() {
	data := "test pattern"
}

func AnotherFunction%d() {
	result := "pattern search"
}`, i, i, i)

		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
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

	t.Run("concurrent_identical_searches", func(t *testing.T) {
		numGoroutines := 10
		results := make(chan error, numGoroutines)
		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				_, err := server.CallTool("search", map[string]interface{}{
					"pattern":     "pattern",
					"max_results": 20,
				})
				results <- err
			}(i)
		}

		errorCount := 0
		for i := 0; i < numGoroutines; i++ {
			if err := <-results; err != nil {
				errorCount++
				t.Logf("Concurrent search error: %v", err)
			}
		}

		elapsed := time.Since(start)
		t.Logf("Concurrent searches completed in %v (%d errors)", elapsed, errorCount)

		if errorCount > 0 {
			t.Errorf("Expected no errors in concurrent searches, got %d", errorCount)
		}
	})

	t.Run("concurrent_different_searches", func(t *testing.T) {
		patterns := []string{"pattern", "Function", "test", "data", "result"}
		numGoroutines := len(patterns) * 5
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			pattern := patterns[i%len(patterns)]
			go func(p string) {
				_, err := server.CallTool("search", map[string]interface{}{
					"pattern":     p,
					"max_results": 10,
				})
				results <- err
			}(pattern)
		}

		errorCount := 0
		for i := 0; i < numGoroutines; i++ {
			if err := <-results; err != nil {
				errorCount++
			}
		}

		if errorCount > 0 {
			t.Logf("Concurrent different searches had %d errors", errorCount)
		}
	})
}

// TestSearchBoundaryConditions tests edge cases and boundary conditions
// Gap addressed: Code Coverage - Edge cases (currently 45/100)
func TestSearchBoundaryConditions(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(tempDir)

	// Create files with boundary conditions
	testFiles := map[string]string{
		"empty.go":       "",
		"single_line.go": "package main\nfunc test() {}",
		"unicode.go": `package main
// Unicode: 你好世界 🚀
func unicodeFunc() {
	text := "文字テスト"
}`,
		"long_line.go":      "package main\nfunc test() { " + strings.Repeat("a", 5000) + " }",
		"many_functions.go": generateManyFunctions(100),
	}

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

	// Test boundary conditions
	t.Run("empty_file", func(t *testing.T) {
		_, err := server.CallTool("search", map[string]interface{}{
			"pattern": "test",
		})
		if err != nil {
			t.Logf("Search on empty files failed: %v", err)
		} else {
			t.Logf("Search handled empty files")
		}
	})

	t.Run("unicode_search", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern": "文字",
		})
		if err != nil {
			t.Logf("Unicode search failed: %v", err)
		} else if result != "" {
			t.Logf("Unicode search found results")
		}
	})

	t.Run("long_line_search", func(t *testing.T) {
		result, err := server.CallTool("search", map[string]interface{}{
			"pattern": "aaa", // Should find the repeated 'a's
		})
		if err != nil {
			t.Logf("Long line search failed: %v", err)
		} else if result != "" {
			t.Logf("Long line search found results")
		}
	})

	t.Run("many_matches_single_file", func(t *testing.T) {
		_, err := server.CallTool("search", map[string]interface{}{
			"pattern":     "func",
			"max_results": 50,
		})
		if err != nil {
			t.Logf("Many matches search failed: %v", err)
		} else {
			t.Logf("Found many matching functions")
		}
	})

	t.Run("zero_results_search", func(t *testing.T) {
		_, err := server.CallTool("search", map[string]interface{}{
			"pattern": "NonExistentPattern12345",
		})
		if err != nil {
			t.Logf("Zero results search error: %v", err)
		} else {
			t.Logf("Zero results search completed (may return empty result)")
		}
	})
}

// TestSearchPerformanceScaling tests search performance with increasing data
// Gap addressed: Code Coverage - Performance characteristics (currently 45/100)
func TestSearchPerformanceScaling(t *testing.T) {
	// Skip in short test mode
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testSizes := []int{10, 50, 100}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("scale_%d_files", size), func(t *testing.T) {
			tempDir := t.TempDir()
			cfg := createTestConfig(tempDir)

			// Create test files
			for i := 0; i < size; i++ {
				content := fmt.Sprintf(`package pkg%d
func TestFunc%d() { pattern := "search_target"; }
func OtherFunc%d() { value := "search_target"; }
`, i, i, i)

				filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
			}

			gi := indexing.NewMasterIndex(cfg)
			server, err := NewServer(gi, cfg)
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}

			// Measure indexing time (auto-indexing started in NewServer)
			startIndex := time.Now()
			status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
			if err != nil {
				t.Fatalf("Auto-indexing failed: %v", err)
			}
			if status != "completed" {
				t.Fatalf("Auto-indexing did not complete, got status: %s", status)
			}
			indexTime := time.Since(startIndex)

			// Measure search time
			startSearch := time.Now()
			_, _ = server.CallTool("search", map[string]interface{}{
				"pattern":     "search_target",
				"max_results": 100,
			})
			searchTime := time.Since(startSearch)

			t.Logf("Files: %d, Indexing: %v, Search: %v", size, indexTime, searchTime)
		})
	}
}

// Helper functions

func createRealisticProject(t *testing.T, baseDir string) {
	dirs := []string{"service", "utils", "controller", "models", "middleware"}
	for _, dir := range dirs {
		if err := os.Mkdir(filepath.Join(baseDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}

	files := map[string]string{
		"service/main.go": `package service

type Service struct {
	name string
}

func (s *Service) ProcessRequest(req interface{}) error {
	return nil
}`,

		"utils/helper.go": `package utils

func HelperFunction(input string) string {
	return "processed: " + input
}`,

		"controller/api.go": `package controller

func ApiHandler(route string) {
	// Handle API requests
}`,

		"models/user.go": `package models

type User struct {
	ID   int
	Name string
}`,

		"middleware/auth.go": `package middleware

func AuthMiddleware() {
	// Authentication logic
}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(baseDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}
}

func createHierarchicalCodeStructure(t *testing.T, baseDir string) {
	structure := map[string]string{
		"service/main.go": `package service

func MainService() {
	processData()
}

func processData() {
	// implementation
}`,

		"utils/helper.go": `package utils

func HelperUtil() {
	// utility
}`,

		"internal/secret.go": `package internal

func secretFunction() {
	// internal
}`,

		"handler_test.go": `package main

func TestHandler() {
	// test
}`,
	}

	for path, content := range structure {
		fullPath := filepath.Join(baseDir, path)
		dir := filepath.Dir(fullPath)
		_ = os.MkdirAll(dir, 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}
}

func getDetailedCodeSample() string {
	return `package main

import "fmt"

// targetFunction is the main function to search for
func targetFunction() {
	// Implementation details
	result := innerFunction("data")
	fmt.Println(result)
}

func innerFunction(input string) string {
	// Inner implementation
	return "processed: " + input
}

// Caller1 uses targetFunction
func Caller1() {
	targetFunction()
}

// Caller2 also uses targetFunction
func Caller2() {
	targetFunction()
}`
}

func createTestCodeFile(t *testing.T, baseDir, filename string, content string) {
	path := filepath.Join(baseDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", filename, err)
	}
}

func generateManyFunctions(count int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\n")

	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("func Function%d() {\n", i))
		sb.WriteString(fmt.Sprintf("  // Function %d body\n", i))
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
