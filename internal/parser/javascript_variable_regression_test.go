package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestJavaScriptVariableRegression ensures regular JavaScript variables are still indexed
// This test protects against the regression where removing variable parsing broke regular vars
func TestJavaScriptVariableRegression(t *testing.T) {
	jsCode := `
// Regular variables - these MUST be indexed as variables
const regularVar = "hello";
let count = 42;
var name = "test";
const flag = true;

// Function variables - these should be BOTH function AND variable
const add = (a, b) => a + b;
const multiply = function(x, y) { return x * y; };

// Other constructs that should work
function regularFunction() {
    const localVar = "local";
    return localVar;
}

class TestClass {
    constructor() {
        this.instanceVar = "instance";
    }

    method() {
        const methodVar = "method";
        return methodVar;
    }
}
`

	// Parse with TreeSitterParser
	parser := NewTreeSitterParser()
	_, symbols, _ := parser.ParseFile("test.js", []byte(jsCode))

	t.Logf("Found %d symbols", len(symbols))

	// Categorize symbols
	variables := []string{}
	functions := []string{}
	classes := []string{}
	methods := []string{}

	for _, sym := range symbols {
		switch sym.Type {
		case types.SymbolTypeVariable:
			variables = append(variables, sym.Name)
		case types.SymbolTypeFunction:
			functions = append(functions, sym.Name)
		case types.SymbolTypeClass:
			classes = append(classes, sym.Name)
		case types.SymbolTypeMethod:
			methods = append(methods, sym.Name)
		}
	}

	t.Logf("Variables: %v", variables)
	t.Logf("Functions: %v", functions)
	t.Logf("Classes: %v", classes)
	t.Logf("Methods: %v", methods)

	// Test expectations
	expectedVariables := map[string]bool{
		"regularVar": true,
		"count":      true,
		"name":       true,
		"flag":       true,
		"add":        true, // Function variables also appear as variables
		"multiply":   true, // Function variables also appear as variables
		"localVar":   true,
		"methodVar":  true,
		// Note: this.instanceVar is not captured as a variable declaration by tree-sitter
		// It's a property assignment, not a variable declaration
	}

	expectedFunctions := map[string]bool{
		"add":             true, // Function variables also appear as functions
		"multiply":        true, // Function variables also appear as functions
		"regularFunction": true,
	}

	expectedClasses := map[string]bool{
		"TestClass": true,
	}

	expectedMethods := map[string]bool{
		"constructor": true,
		"method":      true,
	}

	// Validate variables
	foundVariables := make(map[string]bool)
	for _, varName := range variables {
		foundVariables[varName] = true
	}

	for expectedVar := range expectedVariables {
		if !foundVariables[expectedVar] {
			t.Errorf("Expected variable %q not found", expectedVar)
		}
	}

	// Validate functions
	foundFunctions := make(map[string]bool)
	for _, funcName := range functions {
		foundFunctions[funcName] = true
	}

	for expectedFunc := range expectedFunctions {
		if !foundFunctions[expectedFunc] {
			t.Errorf("Expected function %q not found", expectedFunc)
		}
	}

	// Validate classes
	foundClasses := make(map[string]bool)
	for _, className := range classes {
		foundClasses[className] = true
	}

	for expectedClass := range expectedClasses {
		if !foundClasses[expectedClass] {
			t.Errorf("Expected class %q not found", expectedClass)
		}
	}

	// Validate methods
	foundMethods := make(map[string]bool)
	for _, methodName := range methods {
		foundMethods[methodName] = true
	}

	for expectedMethod := range expectedMethods {
		if !foundMethods[expectedMethod] {
			t.Errorf("Expected method %q not found", expectedMethod)
		}
	}

	// Critical regression check: ensure we have regular variables
	regularVarCount := 0
	for _, varName := range variables {
		if varName == "regularVar" || varName == "count" || varName == "name" || varName == "flag" {
			regularVarCount++
		}
	}

	if regularVarCount < 4 {
		t.Errorf("REGRESSION: Expected at least 4 regular variables (regularVar, count, name, flag), found %d", regularVarCount)
		t.Errorf("This indicates that regular JavaScript variable parsing is broken")
	}

	t.Logf("✅ JavaScript variable regression test passed - found %d regular variables", regularVarCount)
}

// TestJavaScriptDualNatureBehavior validates that function assignments have dual nature
func TestJavaScriptDualNatureBehavior(t *testing.T) {
	jsCode := `
const arrowFunc = (a, b) => a + b;
const funcExpr = function(x) { return x * 2; };
const regularVar = "hello";
function regularFunc() { return "world"; }
`

	// Parse with TreeSitterParser
	parser := NewTreeSitterParser()
	_, symbols, _ := parser.ParseFile("test.js", []byte(jsCode))

	// Group symbols by name
	symbolsByName := make(map[string][]types.SymbolType)
	for _, sym := range symbols {
		symbolsByName[sym.Name] = append(symbolsByName[sym.Name], sym.Type)
	}

	// Test dual nature behavior
	tests := []struct {
		name           string
		expectedTypes  []types.SymbolType
		description    string
	}{
		{
			name:          "arrowFunc",
			expectedTypes: []types.SymbolType{types.SymbolTypeFunction, types.SymbolTypeVariable},
			description:   "Arrow function should be both function and variable",
		},
		{
			name:          "funcExpr",
			expectedTypes: []types.SymbolType{types.SymbolTypeFunction, types.SymbolTypeVariable},
			description:   "Function expression should be both function and variable",
		},
		{
			name:          "regularVar",
			expectedTypes: []types.SymbolType{types.SymbolTypeVariable},
			description:   "Regular variable should be variable only",
		},
		{
			name:          "regularFunc",
			expectedTypes: []types.SymbolType{types.SymbolTypeFunction},
			description:   "Regular function should be function only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			foundTypes, exists := symbolsByName[test.name]
			if !exists {
				t.Fatalf("Symbol %q not found", test.name)
			}

			if len(foundTypes) != len(test.expectedTypes) {
				t.Errorf("Expected %d types for %q, got %d: %v",
					len(test.expectedTypes), test.name, len(foundTypes), foundTypes)
				return
			}

			// Check all expected types are present
			foundTypeMap := make(map[types.SymbolType]bool)
			for _, fType := range foundTypes {
				foundTypeMap[fType] = true
			}

			for _, expectedType := range test.expectedTypes {
				if !foundTypeMap[expectedType] {
					t.Errorf("Expected type %s not found for %q", expectedType, test.name)
				}
			}

			t.Logf("✅ %s", test.description)
		})
	}
}