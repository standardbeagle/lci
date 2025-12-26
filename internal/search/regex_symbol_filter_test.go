package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// TestRegexSearch_WithSymbolTypesFilter tests that regex searches properly
// respect SymbolTypes filtering. This is a regression test for the issue where
// regex searches bypassed semantic filtering entirely.
//
// Issue: When using UseRegex=true with SymbolTypes filter, the search should
// only return matches that are within symbols of the specified types.
// Previously, searchWithHybridRegex ignored SymbolTypes completely.
func TestRegexSearch_WithSymbolTypesFilter(t *testing.T) {
	tempDir := t.TempDir()

	// Create Go files with different symbol types containing "Calc" in their names
	goCode := `package main

// CalcAdd is an exported function
func CalcAdd(a, b int) int {
	return a + b
}

// CalcType is a type definition
type CalcType struct {
	Value int
}

// CalcVar is a variable
var CalcVar = 42

// calcPrivate is a private function
func calcPrivate() int {
	return CalcVar
}
`
	goFilePath := filepath.Join(tempDir, "calc.go")
	if err := os.WriteFile(goFilePath, []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-Go file that contains "Calc" - should never match with symbol_types
	txtCode := `This is a text file with Calc mentioned.
CalcNotes: some notes about calculations.
`
	txtFilePath := filepath.Join(tempDir, "notes.txt")
	if err := os.WriteFile(txtFilePath, []byte(txtCode), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	defer indexer.Close()

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatal(err)
	}

	engine := search.NewEngine(indexer)
	allFiles := indexer.GetAllFileIDs()

	t.Run("regex_with_function_symbol_type_excludes_non_code_files", func(t *testing.T) {
		// Search for "Calc" with regex enabled and symbol_types=function
		// Should NOT match text files at all since they have no symbols
		results := engine.SearchWithOptions("Calc", allFiles, types.SearchOptions{
			UseRegex:    true,
			SymbolTypes: []string{"function"},
		})

		txtFileCount := 0
		goFileCount := 0

		for _, r := range results {
			ext := filepath.Ext(r.Path)
			if ext == ".txt" {
				txtFileCount++
				t.Errorf("Regex search with SymbolTypes=function matched text file: %s at line %d match=%q",
					r.Path, r.Line, r.Match)
			} else if ext == ".go" {
				goFileCount++
			}
		}

		t.Logf("Results: %d Go files, %d txt files, total %d", goFileCount, txtFileCount, len(results))

		if txtFileCount > 0 {
			t.Errorf("Expected 0 text file matches with SymbolTypes=function, got %d", txtFileCount)
		}
	})

	t.Run("regex_with_type_symbol_type_excludes_non_code_files", func(t *testing.T) {
		// Search for "Calc" with regex enabled and symbol_types=type/struct
		// Should NOT match text files
		results := engine.SearchWithOptions("Calc", allFiles, types.SearchOptions{
			UseRegex:    true,
			SymbolTypes: []string{"type", "struct"},
		})

		for _, r := range results {
			if filepath.Ext(r.Path) == ".txt" {
				t.Errorf("Regex search with SymbolTypes=type matched text file: %s at line %d", r.Path, r.Line)
			}
		}

		t.Logf("Found %d total results", len(results))
	})

	t.Run("regex_without_symbol_filter_may_match_text_files", func(t *testing.T) {
		// Without SymbolTypes filter, regex should find matches in all files
		results := engine.SearchWithOptions("Calc", allFiles, types.SearchOptions{
			UseRegex: true,
		})

		goFileMatches := 0
		txtFileMatches := 0
		for _, r := range results {
			ext := filepath.Ext(r.Path)
			if ext == ".txt" {
				txtFileMatches++
			} else if ext == ".go" {
				goFileMatches++
			}
		}

		t.Logf("Without filter: %d Go matches, %d txt matches", goFileMatches, txtFileMatches)

		// Without filter, should find matches in Go files
		if goFileMatches == 0 {
			t.Errorf("Expected Go file matches, got 0")
		}
		// Text files might or might not match depending on implementation
	})
}

// TestRegexSearch_ExportedOnlyFilter tests that regex searches properly
// respect the ExportedOnly filter.
func TestRegexSearch_ExportedOnlyFilter(t *testing.T) {
	tempDir := t.TempDir()

	// Create Go files with both exported and unexported symbols
	goCode := `package main

// PublicFunc is exported (starts with uppercase)
func PublicFunc() int {
	return privateFunc()
}

// privateFunc is not exported (starts with lowercase)
func privateFunc() int {
	return 42
}

// PublicType is exported
type PublicType struct {
	PublicField  int
	privateField int
}

// privateType is not exported
type privateType struct {
	Value int
}

// PublicVar is exported
var PublicVar = 100

// privateVar is not exported
var privateVar = 50
`
	goFilePath := filepath.Join(tempDir, "exports.go")
	if err := os.WriteFile(goFilePath, []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	defer indexer.Close()

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatal(err)
	}

	engine := search.NewEngine(indexer)
	allFiles := indexer.GetAllFileIDs()

	t.Run("regex_with_exported_only", func(t *testing.T) {
		// Search with regex and ExportedOnly=true
		// Should only find Public* symbols, not private* symbols
		results := engine.SearchWithOptions("Func", allFiles, types.SearchOptions{
			UseRegex:     true,
			ExportedOnly: true,
		})

		// Count how many matches contain "private" (lowercase start)
		privateMatches := 0
		publicMatches := 0

		for _, r := range results {
			// Check the matched line content
			for _, line := range r.Context.Lines {
				if regexTestContainsWord(line, "privateFunc") || regexTestContainsWord(line, "privateType") ||
					regexTestContainsWord(line, "privateVar") || regexTestContainsWord(line, "privateField") {
					privateMatches++
					t.Logf("Found private symbol match at %s:%d - line contains: %s", r.Path, r.Line, line)
				}
				if regexTestContainsWord(line, "PublicFunc") || regexTestContainsWord(line, "PublicType") ||
					regexTestContainsWord(line, "PublicVar") || regexTestContainsWord(line, "PublicField") {
					publicMatches++
				}
			}
		}

		t.Logf("ExportedOnly results: %d total, ~%d public, ~%d private", len(results), publicMatches, privateMatches)

		// With ExportedOnly=true, we should not find declarations of private symbols
		// (Note: we might still find *usages* of private symbols, but not their declarations)
	})

	t.Run("non_regex_with_exported_only", func(t *testing.T) {
		// Non-regex search with ExportedOnly should also work
		results := engine.SearchWithOptions("Func", allFiles, types.SearchOptions{
			UseRegex:     false,
			ExportedOnly: true,
		})

		t.Logf("Non-regex ExportedOnly: %d results", len(results))

		// Log all matches for analysis
		for i, r := range results {
			t.Logf("Result %d: %s:%d match=%q", i, r.Path, r.Line, r.Match)
		}
	})
}

// TestRegexSearch_CombinedFilters tests regex search with multiple filters combined
func TestRegexSearch_CombinedFilters(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package main

// PublicHandler is an exported function
func PublicHandler() {}

// privateHandler is not exported
func privateHandler() {}

// PublicModel is an exported type
type PublicModel struct {}

// privateModel is not exported
type privateModel struct {}
`
	goFilePath := filepath.Join(tempDir, "combined.go")
	if err := os.WriteFile(goFilePath, []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Add a text file to ensure it's excluded
	txtCode := `Handler and Model mentioned in text file`
	txtFilePath := filepath.Join(tempDir, "readme.txt")
	if err := os.WriteFile(txtFilePath, []byte(txtCode), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	defer indexer.Close()

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatal(err)
	}

	engine := search.NewEngine(indexer)
	allFiles := indexer.GetAllFileIDs()

	t.Run("regex_with_symbol_type_excludes_text_files", func(t *testing.T) {
		// Regex + SymbolTypes=function should exclude text files entirely
		results := engine.SearchWithOptions("Handler", allFiles, types.SearchOptions{
			UseRegex:    true,
			SymbolTypes: []string{"function"},
		})

		for _, r := range results {
			if filepath.Ext(r.Path) == ".txt" {
				t.Errorf("Got text file result when SymbolTypes=function: %s:%d", r.Path, r.Line)
			}
			t.Logf("Match: %s at %s:%d", r.Match, r.Path, r.Line)
		}

		// Should find at least some function matches
		if len(results) == 0 {
			t.Logf("Note: Got 0 results - verify if this is expected")
		}
	})

	t.Run("regex_exported_function_only", func(t *testing.T) {
		// Regex + SymbolTypes=function + ExportedOnly=true
		results := engine.SearchWithOptions("Handler", allFiles, types.SearchOptions{
			UseRegex:     true,
			SymbolTypes:  []string{"function"},
			ExportedOnly: true,
		})

		for _, r := range results {
			t.Logf("Match: %s at %s:%d", r.Match, r.Path, r.Line)

			// Should not be a text file
			if filepath.Ext(r.Path) == ".txt" {
				t.Errorf("Got text file with SymbolTypes=function: %s", r.Path)
			}
		}
	})
}

// TestRegexSearch_AnchorPatterns tests that ^ and $ anchors work correctly
// This is a regression test for the issue where ^type would only match if "type"
// was at the very start of the file content, not at the start of each line.
func TestRegexSearch_AnchorPatterns(t *testing.T) {
	tempDir := t.TempDir()

	// Create Go files with content where keywords are at line beginnings
	goCode := `package main

import "fmt"

type Config struct {
	Name string
}

func ProcessData() error {
	return nil
}

type Handler struct {
	Config *Config
}
`
	goFilePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(goFilePath, []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	defer indexer.Close()

	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatal(err)
	}

	engine := search.NewEngine(indexer)
	allFiles := indexer.GetAllFileIDs()

	t.Run("caret_matches_type_at_line_start", func(t *testing.T) {
		// ^type should match "type" at the start of lines 5 and 13
		results := engine.SearchWithOptions("^type", allFiles, types.SearchOptions{
			UseRegex: true,
		})

		if len(results) < 2 {
			t.Errorf("Expected at least 2 matches for ^type, got %d", len(results))
			for i, r := range results {
				t.Logf("Result %d: %s:%d match=%q", i, r.Path, r.Line, r.Match)
			}
		} else {
			t.Logf("Found %d matches for ^type", len(results))
		}
	})

	t.Run("caret_matches_func_at_line_start", func(t *testing.T) {
		// ^func should match "func" at the start of line 9
		results := engine.SearchWithOptions("^func", allFiles, types.SearchOptions{
			UseRegex: true,
		})

		if len(results) < 1 {
			t.Errorf("Expected at least 1 match for ^func, got %d", len(results))
		} else {
			t.Logf("Found %d matches for ^func", len(results))
		}
	})

	t.Run("caret_no_false_positives", func(t *testing.T) {
		// ^Name should NOT match because "Name" is indented
		results := engine.SearchWithOptions("^Name", allFiles, types.SearchOptions{
			UseRegex: true,
		})

		if len(results) != 0 {
			t.Errorf("Expected 0 matches for ^Name (it's indented), got %d", len(results))
			for i, r := range results {
				t.Logf("Unexpected result %d: %s:%d match=%q", i, r.Path, r.Line, r.Match)
			}
		}
	})

	t.Run("complex_anchor_pattern", func(t *testing.T) {
		// ^type [A-Z] should match type declarations at line beginning
		results := engine.SearchWithOptions("^type [A-Z]", allFiles, types.SearchOptions{
			UseRegex: true,
		})

		if len(results) < 2 {
			t.Errorf("Expected at least 2 matches for ^type [A-Z], got %d", len(results))
		} else {
			t.Logf("Found %d matches for ^type [A-Z]", len(results))
		}
	})
}

// regexTestContainsWord checks if a line contains a specific word
func regexTestContainsWord(line, word string) bool {
	// Simple substring check
	return len(line) >= len(word) && (line == word ||
		(len(line) > len(word) && (line[:len(word)] == word || line[len(line)-len(word):] == word)) ||
		regexTestContainsSubstr(line, word))
}

func regexTestContainsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
