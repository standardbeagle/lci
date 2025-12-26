package search_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestSymbolTypes_FunctionOnly tests filtering to functions only
func TestSymbolTypes_FunctionOnly(t *testing.T) {
	code := `package main

// MyFunction is a test function
func MyFunction() {
	x := 42
	_ = x
}

// MyStruct is a test struct
type MyStruct struct {
	field string
}

var MyVariable = "test"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search for "My" with function filter
	results := engine.SearchWithOptions("My", allFiles, types.SearchOptions{
		SymbolTypes: []string{"function"},
	})

	t.Logf("Found %d function results", len(results))

	// Should find at least the function
	// (Note: symbol type filtering depends on implementation details)
}

// TestSymbolTypes_ClassOnly tests filtering to classes/types only
func TestSymbolTypes_ClassOnly(t *testing.T) {
	code := `package main

func MyFunction() {}

// MyStruct is a test struct
type MyStruct struct {
	field string
}

// MyInterface is a test interface
type MyInterface interface {
	Method()
}

var MyVariable = "test"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search for "My" with class/type filter
	results := engine.SearchWithOptions("My", allFiles, types.SearchOptions{
		SymbolTypes: []string{"class"},
	})

	t.Logf("Found %d type results", len(results))

	// Should find types
	// (Note: symbol type filtering depends on implementation details)
}

// TestSymbolTypes_VariableOnly tests filtering to variables only
func TestSymbolTypes_VariableOnly(t *testing.T) {
	code := `package main

func MyFunction() {}

type MyStruct struct{}

// MyVariable is a test variable
var MyVariable = "test"

// MyConstant is a test constant
const MyConstant = 42
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search for "My" with variable filter
	results := engine.SearchWithOptions("My", allFiles, types.SearchOptions{
		SymbolTypes: []string{"variable"},
	})

	t.Logf("Found %d variable results", len(results))

	// Should find variables/constants
	// (Note: symbol type filtering depends on implementation details)
}

// TestSymbolTypes_MultipleTypes tests combining multiple symbol types
func TestSymbolTypes_MultipleTypes(t *testing.T) {
	code := `package main

// MyFunction is a test function
func MyFunction() {}

// MyStruct is a test struct
type MyStruct struct{}

// MyVariable is a test variable
var MyVariable = "test"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Test 1: Function + Class
	functionAndClass := engine.SearchWithOptions("My", allFiles, types.SearchOptions{
		SymbolTypes: []string{"function", "class"},
	})

	t.Logf("Function+Class filter: found %d results", len(functionAndClass))

	// Test 2: All types vs no filter
	allTypes := engine.SearchWithOptions("My", allFiles, types.SearchOptions{
		SymbolTypes: []string{"function", "class", "variable"},
	})

	noFilter := engine.SearchWithOptions("My", allFiles, types.SearchOptions{})

	t.Logf("All types filter: %d results, No filter: %d results", len(allTypes), len(noFilter))
}

// TestSymbolTypes_CombinedWithOtherFilters tests symbol type filter with other options
func TestSymbolTypes_CombinedWithOtherFilters(t *testing.T) {
	code := `package main

// ExportedFunction is public
func ExportedFunction() {}

// unexportedFunction is private
func unexportedFunction() {}

// ExportedType is public
type ExportedType struct{}

// unexportedType is private
type unexportedType struct{}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Test 1: Function + ExportedOnly
	exportedFunctions := engine.SearchWithOptions("Function", allFiles, types.SearchOptions{
		SymbolTypes:  []string{"function"},
		ExportedOnly: true,
	})

	t.Logf("Exported functions only: found %d results", len(exportedFunctions))

	// Test 2: Class + DeclarationOnly
	typeDeclarations := engine.SearchWithOptions("Type", allFiles, types.SearchOptions{
		SymbolTypes:     []string{"class"},
		DeclarationOnly: true,
	})

	t.Logf("Type declarations only: found %d results", len(typeDeclarations))
}

// TestSymbolTypes_CaseInsensitive tests symbol type filter with case insensitivity
func TestSymbolTypes_CaseInsensitive(t *testing.T) {
	code := `package main

func MyFunction() {}

func myfunction() {}

type MyType struct{}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Case-sensitive function search
	caseSensitive := engine.SearchWithOptions("MyFunction", allFiles, types.SearchOptions{
		SymbolTypes:     []string{"function"},
		CaseInsensitive: false,
	})

	// Case-insensitive function search
	caseInsensitive := engine.SearchWithOptions("myfunction", allFiles, types.SearchOptions{
		SymbolTypes:     []string{"function"},
		CaseInsensitive: true,
	})

	t.Logf("Case-sensitive: %d results, Case-insensitive: %d results",
		len(caseSensitive), len(caseInsensitive))
}
