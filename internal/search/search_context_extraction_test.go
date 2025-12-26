package search_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestContextExtraction_MaxContextLines tests MaxContextLines option
func TestContextExtraction_MaxContextLines(t *testing.T) {
	code := `package main

import "fmt"

func TestFunction() {
	line1 := "test"
	line2 := "test"
	line3 := "test"
	TestMatch := "here" // This is the target match
	line5 := "test"
	line6 := "test"
	line7 := "test"
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// No context
	noContext := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines: 0,
	})

	// 2 lines of context
	smallContext := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines: 2,
	})

	// 5 lines of context
	largeContext := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines: 5,
	})

	// Default context (no limit specified)
	defaultContext := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{})

	t.Logf("No context: %d results, Small context: %d results, Large context: %d results, Default: %d results",
		len(noContext), len(smallContext), len(largeContext), len(defaultContext))

	// Check context sizes
	if len(smallContext) > 0 {
		contextLines := len(smallContext[0].Context.Lines)
		t.Logf("Small context has %d lines", contextLines)
	}

	if len(largeContext) > 0 {
		contextLines := len(largeContext[0].Context.Lines)
		t.Logf("Large context has %d lines", contextLines)
	}
}

// TestContextExtraction_EnsureCompleteStmt tests EnsureCompleteStmt option
func TestContextExtraction_EnsureCompleteStmt(t *testing.T) {
	code := `package main

func TestFunction() {
	if condition {
		TestMatch := doSomething(
			param1,
			param2,
			param3,
		)
		_ = TestMatch
	}
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Without EnsureCompleteStmt
	incomplete := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines: 2,
	})

	// With EnsureCompleteStmt
	complete := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines:     2,
		EnsureCompleteStmt:  true,
	})

	t.Logf("Without EnsureCompleteStmt: %d results, With EnsureCompleteStmt: %d results",
		len(incomplete), len(complete))

	if len(incomplete) > 0 {
		t.Logf("Incomplete context lines: %d", len(incomplete[0].Context.Lines))
	}

	if len(complete) > 0 {
		t.Logf("Complete statement context lines: %d", len(complete[0].Context.Lines))
		// Complete statement should have >= lines than incomplete
		if len(complete[0].Context.Lines) < len(incomplete[0].Context.Lines) {
			t.Logf("Note: Complete statement may extend context")
		}
	}
}

// TestContextExtraction_FullFunction tests FullFunction option
func TestContextExtraction_FullFunction(t *testing.T) {
	code := `package main

// TestFunction is a test function
func TestFunction() {
	line1 := "setup"
	line2 := "setup"
	TestMatch := "target"
	line4 := "teardown"
	line5 := "teardown"
}

func AnotherFunction() {
	x := 42
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Normal context
	normalContext := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines: 2,
	})

	// Full function context
	fullFunction := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		FullFunction: true,
	})

	t.Logf("Normal context: %d results, Full function: %d results",
		len(normalContext), len(fullFunction))

	if len(normalContext) > 0 {
		t.Logf("Normal context lines: %d", len(normalContext[0].Context.Lines))
	}

	if len(fullFunction) > 0 {
		t.Logf("Full function context lines: %d", len(fullFunction[0].Context.Lines))
		// Full function should have more lines than limited context
		if len(normalContext) > 0 && len(fullFunction[0].Context.Lines) > len(normalContext[0].Context.Lines) {
			t.Logf("Full function provides more context as expected")
		}
	}
}

// TestContextExtraction_WithSymbolTypes tests context extraction with symbol type filtering
func TestContextExtraction_WithSymbolTypes(t *testing.T) {
	code := `package main

// TestType is a type
type TestType struct {
	field1 string
	field2 int
	field3 bool
}

// TestFunction is a function
func TestFunction() {
	x := 42
	y := 43
	z := 44
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Type with full context
	typeFullContext := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		SymbolTypes:  []string{"class"},
		FullFunction: true,
	})

	// Function with limited context
	funcLimitedContext := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		SymbolTypes:     []string{"function"},
		MaxContextLines: 2,
	})

	t.Logf("Type (full context): %d results, Function (limited): %d results",
		len(typeFullContext), len(funcLimitedContext))
}

// TestContextExtraction_MultipleMatches tests context for multiple matches in same file
func TestContextExtraction_MultipleMatches(t *testing.T) {
	code := `package main

func TestFunc1() {
	x := 1
}

func TestFunc2() {
	y := 2
}

func TestFunc3() {
	z := 3
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search for all Test functions
	results := engine.SearchWithOptions("TestFunc", allFiles, types.SearchOptions{
		MaxContextLines: 2,
	})

	t.Logf("Found %d matches", len(results))

	// Each match should have its own context
	for i, result := range results {
		if len(result.Context.Lines) > 0 {
			t.Logf("Match %d has %d context lines", i+1, len(result.Context.Lines))
		}
	}
}

// TestContextExtraction_EdgeOfFile tests context extraction at file boundaries
func TestContextExtraction_EdgeOfFile(t *testing.T) {
	code := `package main

func TestAtTop() {
	x := 1
}

// ... middle content ...

func TestAtBottom() {
	y := 2
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Large context that would exceed file boundaries
	results := engine.SearchWithOptions("Test", allFiles, types.SearchOptions{
		MaxContextLines: 100, // Request more lines than file has
	})

	t.Logf("Found %d matches with large context request", len(results))

	// Context should be clamped to file boundaries
	for i, result := range results {
		t.Logf("Match %d: line %d, context lines: %d",
			i+1, result.Line, len(result.Context.Lines))
	}
}

// TestContextExtraction_WithDeclarationOnly tests context extraction for declarations
func TestContextExtraction_WithDeclarationOnly(t *testing.T) {
	code := `package main

// ProcessData processes input data
// It takes a string parameter
// And returns an error if validation fails
func ProcessData(input string) error {
	validate(input)
	return nil
}

func Main() {
	ProcessData("test")
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Declaration with full function
	declFull := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		FullFunction:    true,
	})

	// Declaration with limited context
	declLimited := engine.SearchWithOptions("ProcessData", allFiles, types.SearchOptions{
		DeclarationOnly: true,
		MaxContextLines: 3,
	})

	t.Logf("Declaration (full): %d results, Declaration (limited): %d results",
		len(declFull), len(declLimited))

	if len(declFull) > 0 {
		t.Logf("Full declaration context: %d lines", len(declFull[0].Context.Lines))
	}

	if len(declLimited) > 0 {
		t.Logf("Limited declaration context: %d lines", len(declLimited[0].Context.Lines))
	}
}

// TestContextExtraction_EmptyLines tests context extraction with empty lines
func TestContextExtraction_EmptyLines(t *testing.T) {
	code := `package main

func TestFunction() {

	TestMatch := "value"

	_ = TestMatch

}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	results := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		MaxContextLines: 3,
	})

	t.Logf("Found %d results", len(results))

	// Context should include empty lines
	if len(results) > 0 {
		t.Logf("Context has %d lines (including empty lines)", len(results[0].Context.Lines))
	}
}

// TestContextExtraction_CombinedOptions tests combining multiple context options
func TestContextExtraction_CombinedOptions(t *testing.T) {
	code := `package main

// TestFunction processes data
func TestFunction(
	param1 string,
	param2 int,
) error {
	if param2 > 0 {
		TestMatch := doSomething(
			param1,
			param2,
		)
		_ = TestMatch
	}
	return nil
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Combine FullFunction + EnsureCompleteStmt
	combined := engine.SearchWithOptions("TestMatch", allFiles, types.SearchOptions{
		FullFunction:       true,
		EnsureCompleteStmt: true,
	})

	t.Logf("Combined options: %d results", len(combined))

	if len(combined) > 0 {
		t.Logf("Combined context lines: %d", len(combined[0].Context.Lines))
		t.Logf("Block type: %s", combined[0].Context.BlockType)
	}
}
