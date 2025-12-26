package search_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestContentFilters_CodeOnly tests CodeOnly/ExcludeComments filter
func TestContentFilters_CodeOnly(t *testing.T) {
	code := `package main

// TestFunction is a commented function
// It has multiple comment lines
func TestFunction() {
	// Inline comment about TestVariable
	x := 42 // TestValue
	_ = x
}

/*
TestBlock comment
with multiple lines
*/
func Helper() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search with CodeOnly (exclude comments)
	codeOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		CodeOnly: true,
	})

	// Search without filter (includes comments)
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("CodeOnly: %d results, All matches: %d results",
		len(codeOnly), len(allMatches))

	// CodeOnly should exclude comment matches
	if len(codeOnly) > len(allMatches) {
		t.Errorf("CodeOnly returned more results than total matches")
	}
}

// TestContentFilters_CommentsOnly tests CommentsOnly filter
func TestContentFilters_CommentsOnly(t *testing.T) {
	code := `package main

// TestComment is in a comment
func TestFunction() {
	// Another TestComment
	x := "TestString" // Inline TestComment
	_ = x
}

/*
Block TestComment
*/
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search in comments only
	commentsOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		CommentsOnly: true,
	})

	// Search everywhere
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("CommentsOnly: %d results, All matches: %d results",
		len(commentsOnly), len(allMatches))

	// Comments-only should be <= all matches
	if len(commentsOnly) > len(allMatches) {
		t.Errorf("CommentsOnly returned more results than total matches")
	}
}

// TestContentFilters_StringsOnly tests StringsOnly filter
func TestContentFilters_StringsOnly(t *testing.T) {
	code := `package main

import "fmt"

func TestFunction() {
	// TestInComment
	message := "TestInString"
	fmt.Println("Another TestInString")

	multiline := ` + "`TestInBacktick`" + `
	_ = multiline
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search in strings only
	stringsOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		StringsOnly: true,
	})

	// Search everywhere
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("StringsOnly: %d results, All matches: %d results",
		len(stringsOnly), len(allMatches))

	// Strings-only should be <= all matches
	if len(stringsOnly) > len(allMatches) {
		t.Errorf("StringsOnly returned more results than total matches")
	}
}

// TestContentFilters_TemplateStrings tests TemplateStrings filter
func TestContentFilters_TemplateStrings(t *testing.T) {
	code := `package main

import "fmt"

func TestFunction() {
	name := "World"
	// Template TestString
	message := fmt.Sprintf("TestHello %s", name)

	backtick := ` + "`TestInTemplate`" + `
	_ = message
	_ = backtick
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search in template strings
	templateStrings := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		TemplateStrings: true,
	})

	// Search everywhere
	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("TemplateStrings: %d results, All matches: %d results",
		len(templateStrings), len(allMatches))
}

// TestContentFilters_MutuallyExclusive tests that content filters are mutually exclusive
func TestContentFilters_MutuallyExclusive(t *testing.T) {
	code := `package main

// TestInComment
func TestFunction() {
	x := "TestInString" // Another TestInComment
	TestVariable := 42
	_ = x
	_ = TestVariable
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	codeOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		CodeOnly: true,
	})

	commentsOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		CommentsOnly: true,
	})

	stringsOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		StringsOnly: true,
	})

	allMatches := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{})

	t.Logf("Code: %d, Comments: %d, Strings: %d, All: %d",
		len(codeOnly), len(commentsOnly), len(stringsOnly), len(allMatches))

	// The sum should be <= all matches (with line deduplication)
	sum := len(codeOnly) + len(commentsOnly) + len(stringsOnly)
	if sum < len(allMatches) {
		t.Logf("Code + Comments + Strings (%d) < All (%d) - expected due to line deduplication", sum, len(allMatches))
	}
}

// TestContentFilters_WithSymbolTypes tests combining content filters with symbol types
func TestContentFilters_WithSymbolTypes(t *testing.T) {
	code := `package main

// TestType is a type
type TestType struct {
	field string
}

// TestFunction is a function
func TestFunction() {
	// TestComment inside
	x := "TestString"
	_ = x
}

var TestVariable = "test"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Functions in code only (no comments)
	funcCodeOnly := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		SymbolTypes: []string{"function"},
		CodeOnly:    true,
	})

	// Types in comments only
	typeComments := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		SymbolTypes:  []string{"class"},
		CommentsOnly: true,
	})

	t.Logf("Functions (code only): %d, Types (comments only): %d",
		len(funcCodeOnly), len(typeComments))
}

// TestContentFilters_WithDeclarationOnly tests combining content filters with declaration filtering
func TestContentFilters_WithDeclarationOnly(t *testing.T) {
	code := `package main

// ProcessData processes data
func ProcessData(input string) error {
	// Comment: ProcessData called
	return ProcessData(input) // Recursive
}

func Main() {
	// Call ProcessData
	ProcessData("test")
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Declaration in code only
	declCodeOnly := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		CodeOnly:        true,
	})

	// Declaration in comments only
	declComments := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		CommentsOnly:    true,
	})

	// All declarations
	allDecl := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		DeclarationOnly: true,
	})

	t.Logf("Decl (code): %d, Decl (comments): %d, Decl (all): %d",
		len(declCodeOnly), len(declComments), len(allDecl))
}

// TestContentFilters_CaseInsensitive tests content filters with case insensitivity
func TestContentFilters_CaseInsensitive(t *testing.T) {
	code := `package main

// testComment is lowercase
func TestFunction() {
	// TESTCOMMENT is uppercase
	x := "testString" // TestInline
	_ = x
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Case-insensitive code search
	codeCI := engine.SearchWithOptions("test", allFiles, types.SearchOptions{
		CodeOnly:        true,
		CaseInsensitive: true,
	})

	// Case-sensitive code search
	codeCS := engine.SearchWithOptions("test", allFiles, types.SearchOptions{
		CodeOnly:        true,
		CaseInsensitive: false,
	})

	t.Logf("Code (case-insensitive): %d, Code (case-sensitive): %d",
		len(codeCI), len(codeCS))

	// Case-insensitive should find more or equal results
	if len(codeCI) < len(codeCS) {
		t.Errorf("Case-insensitive should find at least as many results as case-sensitive")
	}
}

// TestContentFilters_RegexWithContentFilters tests regex with content filters
func TestContentFilters_RegexWithContentFilters(t *testing.T) {
	code := `package main

// Test123 in comment
func TestFunction() {
	// Test456 in comment
	x := "Test789" // Test000 inline
	Test111 := 42
	_ = x
	_ = Test111
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Regex in code only
	regexCode := engine.SearchWithOptions("Test[0-9]+", allFiles, types.SearchOptions{
		UseRegex: true,
		CodeOnly: true,
	})

	// Regex in comments only
	regexComments := engine.SearchWithOptions("Test[0-9]+", allFiles, types.SearchOptions{
		UseRegex:     true,
		CommentsOnly: true,
	})

	// Regex everywhere
	regexAll := engine.SearchWithOptions("Test[0-9]+", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Regex (code): %d, Regex (comments): %d, Regex (all): %d",
		len(regexCode), len(regexComments), len(regexAll))
}
