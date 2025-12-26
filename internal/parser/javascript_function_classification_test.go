package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestJavaScriptFunctionClassification validates that JavaScript functions are classified correctly
// and that arrow functions have dual variable/function behavior (which is correct)
func TestJavaScriptFunctionClassification(t *testing.T) {
	jsCode := `
// Regular function declaration - should be ONLY function
function regularFunction() {
    return "regular";
}

// Arrow function assignment - should be BOTH variable AND function
const arrowFunc = (a, b) => a + b;

// Function expression assignment - should be BOTH variable AND function
const funcExpr = function(x) { return x * 2; };

// Generator function assignment - should be BOTH variable AND function
const generatorFunc = function* () { yield 1; };

// Regular variable - should be ONLY variable
const regularVar = "hello";
let mutableVar = 42;
var oldStyleVar = true;

// Class declaration - should be ONLY class
class TestClass {
    // Method - should be ONLY method
    testMethod() {
        return "method";
    }
}
`

	// Create TreeSitterParser and parse
	tsParser := NewTreeSitterParser()
	blocks, symbols, _ := tsParser.ParseFile("test.js", []byte(jsCode))

	t.Logf("Found %d symbols and %d blocks", len(symbols), len(blocks))

	// Organize symbols by name for easier testing
	symbolsByName := make(map[string][]types.Symbol)
	for _, sym := range symbols {
		symbolsByName[sym.Name] = append(symbolsByName[sym.Name], sym)
	}

	// Test cases
	tests := []struct {
		name            string
		expectedTypes   []types.SymbolType
		description     string
	}{
		{
			name:            "regularFunction",
			expectedTypes:   []types.SymbolType{types.SymbolTypeFunction},
			description:     "Regular function declaration should be function only",
		},
		{
			name:            "arrowFunc",
			expectedTypes:   []types.SymbolType{types.SymbolTypeFunction, types.SymbolTypeVariable},
			description:     "Arrow function assignment should be both function and variable",
		},
		{
			name:            "funcExpr",
			expectedTypes:   []types.SymbolType{types.SymbolTypeFunction, types.SymbolTypeVariable},
			description:     "Function expression assignment should be both function and variable",
		},
		{
			name:            "generatorFunc",
			expectedTypes:   []types.SymbolType{types.SymbolTypeFunction, types.SymbolTypeVariable},
			description:     "Generator function assignment should be both function and variable",
		},
		{
			name:            "regularVar",
			expectedTypes:   []types.SymbolType{types.SymbolTypeVariable},
			description:     "Regular variable should be variable only",
		},
		{
			name:            "mutableVar",
			expectedTypes:   []types.SymbolType{types.SymbolTypeVariable},
			description:     "Let variable should be variable only",
		},
		{
			name:            "oldStyleVar",
			expectedTypes:   []types.SymbolType{types.SymbolTypeVariable},
			description:     "Var variable should be variable only",
		},
		{
			name:            "TestClass",
			expectedTypes:   []types.SymbolType{types.SymbolTypeClass},
			description:     "Class should be class only",
		},
		{
			name:            "testMethod",
			expectedTypes:   []types.SymbolType{types.SymbolTypeMethod},
			description:     "Class method should be method only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			symbols, exists := symbolsByName[test.name]
			if !exists {
				t.Fatalf("Symbol %q not found", test.name)
			}

			// Check we have the right number of symbols
			if len(symbols) != len(test.expectedTypes) {
				t.Errorf("Expected %d symbols for %q, got %d",
					len(test.expectedTypes), test.name, len(symbols))
				for i, sym := range symbols {
					t.Logf("  Symbol %d: %s (type: %s)", i, sym.Name, sym.Type)
				}
				return
			}

			// Check all expected types are present
			foundTypes := make(map[types.SymbolType]bool)
			for _, sym := range symbols {
				foundTypes[sym.Type] = true
			}

			for _, expectedType := range test.expectedTypes {
				if !foundTypes[expectedType] {
					t.Errorf("Expected symbol type %s not found for %q", expectedType, test.name)
				}
			}

			t.Logf("✅ %s: %s", test.name, test.description)
		})
	}
}

// TestJavaScriptSearchBehavior validates that the original search issue is fixed
func TestJavaScriptSearchBehavior(t *testing.T) {
	jsCode := `
const add = (a, b) => a + b;
const multiply = function(x, y) { return x * y; };

function regularFunction() {
    return "test";
}
`

	// This test simulates the original failing scenario:
	// 1. Arrow functions should be indexed as functions
	// 2. DeclarationOnly search with SymbolTypes=["function"] should find them
	// 3. This was the critical bug that was breaking search results

	// Create TreeSitterParser and parse
	tsParser := NewTreeSitterParser()
	_, symbols, _ := tsParser.ParseFile("test.js", []byte(jsCode))

	// Count function symbols (the critical requirement)
	functionCount := 0
	functionNames := []string{}

	for _, sym := range symbols {
		if sym.Type == types.SymbolTypeFunction {
			functionCount++
			functionNames = append(functionNames, sym.Name)
		}
	}

	// Validate: we should have 3 function symbols (add, multiply, regularFunction)
	expectedFunctions := []string{"add", "multiply", "regularFunction"}
	if functionCount != len(expectedFunctions) {
		t.Errorf("Expected %d function symbols, got %d", len(expectedFunctions), functionCount)
		t.Logf("Found function symbols: %v", functionNames)
	}

	// Ensure each expected function is found
	foundFunctions := make(map[string]bool)
	for _, name := range functionNames {
		foundFunctions[name] = true
	}

	for _, expectedFunc := range expectedFunctions {
		if !foundFunctions[expectedFunc] {
			t.Errorf("Expected function %q not found in symbols", expectedFunc)
		}
	}

	t.Logf("✅ All JavaScript functions properly classified for DeclarationOnly search")
}