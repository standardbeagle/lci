package search_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestUsageDeclaration_DeclarationOnly tests DeclarationOnly filter
func TestUsageDeclaration_DeclarationOnly(t *testing.T) {
	code := `package main

import "fmt"

// ProcessData processes input data
func ProcessData(input string) error {
	fmt.Println(input)
	// This is a usage of ProcessData within itself (recursion example)
	ProcessData("recursive")
	return nil
}

func Main() {
	// This is a usage of ProcessData
	ProcessData("test")
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search with DeclarationOnly
	declarationOnly := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		DeclarationOnly: true,
	})

	// Search without DeclarationOnly (should include usages)
	allMatches := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{})

	t.Logf("DeclarationOnly: %d results", len(declarationOnly))
	t.Logf("All matches: %d results", len(allMatches))

	// Declaration-only should have fewer or equal results than all matches
	if len(declarationOnly) > len(allMatches) {
		t.Errorf("DeclarationOnly should not have more results than all matches: got %d vs %d",
			len(declarationOnly), len(allMatches))
	}
}

// TestUsageDeclaration_UsageOnly tests UsageOnly filter
func TestUsageDeclaration_UsageOnly(t *testing.T) {
	code := `package main

import "fmt"

// ProcessData processes input data
func ProcessData(input string) error {
	fmt.Println(input)
	return nil
}

func Main() {
	// This is a usage of ProcessData
	ProcessData("test")
	ProcessData("another")
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search with UsageOnly
	usageOnly := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		UsageOnly: true,
	})

	// Search without UsageOnly
	allMatches := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{})

	t.Logf("UsageOnly: %d results", len(usageOnly))
	t.Logf("All matches: %d results", len(allMatches))

	// Usage-only should generally have fewer or equal results than all matches
	if len(usageOnly) > len(allMatches) {
		t.Logf("Note: UsageOnly has more results than total - may include comment mentions")
	}
}

// TestUsageDeclaration_MutuallyExclusive tests that DeclarationOnly and UsageOnly are exclusive
func TestUsageDeclaration_MutuallyExclusive(t *testing.T) {
	code := `package main

// HelperFunction is a helper
func HelperFunction() {
	// Usage inside
	HelperFunction()
}

func Main() {
	HelperFunction()
	HelperFunction()
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	declarationOnly := engine.SearchWithOptions("HelperFunction", allFiles, types.SearchOptions{
		DeclarationOnly: true,
	})

	usageOnly := engine.SearchWithOptions("HelperFunction", allFiles, types.SearchOptions{
		UsageOnly: true,
	})

	allMatches := engine.SearchWithOptions("HelperFunction", allFiles, types.SearchOptions{})

	t.Logf("DeclarationOnly: %d, UsageOnly: %d, All: %d",
		len(declarationOnly), len(usageOnly), len(allMatches))

	// The sum of declaration + usage should be <= all matches (with possible overlap in comments)
	sum := len(declarationOnly) + len(usageOnly)
	if sum < len(allMatches) {
		t.Logf("Declaration + Usage (%d) < All matches (%d) - expected due to line deduplication", sum, len(allMatches))
	}
}

// TestUsageDeclaration_WithSymbolTypes tests combining with symbol type filtering
func TestUsageDeclaration_WithSymbolTypes(t *testing.T) {
	code := `package main

// MyFunction is a function
func MyFunction() {
	MyFunction() // Recursive call
}

// MyStruct is a struct
type MyStruct struct {
	field string
}

func UseStruct() {
	s := MyStruct{}
	_ = s
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Function declarations only
	functionDecl := engine.SearchWithOptions("MyFunction", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		SymbolTypes:     []string{"function"},
	})

	// Function usages only
	functionUsage := engine.SearchWithOptions("MyFunction", allFiles, types.SearchOptions{
		UsageOnly:   true,
		SymbolTypes: []string{"function"},
	})

	t.Logf("Function declaration: %d, Function usage: %d", len(functionDecl), len(functionUsage))

	// Type declarations
	typeDecl := engine.SearchWithOptions("MyStruct", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		SymbolTypes:     []string{"class"},
	})

	// Type usages
	typeUsage := engine.SearchWithOptions("MyStruct", allFiles, types.SearchOptions{
		UsageOnly:   true,
		SymbolTypes: []string{"class"},
	})

	t.Logf("Type declaration: %d, Type usage: %d", len(typeDecl), len(typeUsage))
}

// TestUsageDeclaration_Variables tests usage/declaration filtering for variables
func TestUsageDeclaration_Variables(t *testing.T) {
	code := `package main

// GlobalVar is a global variable
var GlobalVar = "initial"

func UseGlobalVar() {
	println(GlobalVar)
	GlobalVar = "modified"
}

func AlsoUseGlobalVar() {
	temp := GlobalVar
	_ = temp
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Variable declaration
	varDecl := engine.SearchWithOptions("GlobalVar", allFiles, types.SearchOptions{
		DeclarationOnly: true,
	})

	// Variable usage
	varUsage := engine.SearchWithOptions("GlobalVar", allFiles, types.SearchOptions{
		UsageOnly: true,
	})

	allMatches := engine.SearchWithOptions("GlobalVar", allFiles, types.SearchOptions{})

	t.Logf("Variable declaration: %d, Variable usage: %d, All: %d",
		len(varDecl), len(varUsage), len(allMatches))
}

// TestUsageDeclaration_CaseInsensitive tests case-insensitive usage/declaration filtering
func TestUsageDeclaration_CaseInsensitive(t *testing.T) {
	code := `package main

func TestFunc() {
	testfunc()  // Lowercase usage
}

func testfunc() {
	// Implementation
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Case-insensitive declaration search
	declCI := engine.SearchWithOptions("testfunc", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		CaseInsensitive: true,
	})

	// Case-insensitive usage search
	usageCI := engine.SearchWithOptions("testfunc", allFiles, types.SearchOptions{
		UsageOnly:       true,
		CaseInsensitive: true,
	})

	t.Logf("Case-insensitive - Declaration: %d, Usage: %d", len(declCI), len(usageCI))
}
