package symbollinker

import (
	"os"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// TestGoExtractor tests the go extractor.
func TestGoExtractor(t *testing.T) {
	// Read test file
	content, err := os.ReadFile("testdata/go/simple.go")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_go.Language())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewGoExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Package extraction", func(t *testing.T) {
		// Package name should be in the symbol table
		if table == nil {
			t.Fatal("Symbol table is nil")
		}

		if table.Language != "go" {
			t.Errorf("Expected language 'go', got %s", table.Language)
		}
	})

	t.Run("Import extraction", func(t *testing.T) {
		expectedImports := map[string]string{
			"fmt":     "fmt",
			"strings": "strings",
			"alias":   "path/to/package",
			".":       "dot/import",
			"_":       "blank/import",
		}

		if len(table.Imports) != len(expectedImports) {
			t.Errorf("Expected %d imports, got %d", len(expectedImports), len(table.Imports))
		}

		for _, imp := range table.Imports {
			expected, ok := expectedImports[imp.Alias]
			if !ok {
				t.Errorf("Unexpected import alias: %s", imp.Alias)
			} else if imp.ImportPath != expected {
				t.Errorf("Import %s: expected path %s, got %s", imp.Alias, expected, imp.ImportPath)
			}
		}
	})

	t.Run("Constant extraction", func(t *testing.T) {
		// Check for exported and unexported constants
		expectedConstants := map[string]bool{
			"PublicConstant":  true,  // exported
			"privateConstant": false, // not exported
			"First":           true,
			"Second":          true,
			"third":           false,
		}

		for name, shouldBeExported := range expectedConstants {
			symbols := findSymbolsByName(table, name)
			if len(symbols) == 0 {
				t.Errorf("Constant %s not found", name)
				continue
			}

			symbol := symbols[0]
			if symbol.Kind != types.SymbolKindConstant {
				t.Errorf("Expected %s to be a constant, got %v", name, symbol.Kind)
			}

			if symbol.IsExported != shouldBeExported {
				t.Errorf("Constant %s: expected IsExported=%v, got %v", name, shouldBeExported, symbol.IsExported)
			}
		}
	})

	t.Run("Variable extraction", func(t *testing.T) {
		expectedVars := map[string]bool{
			"PublicVar":   true,
			"privateVar":  false,
			"GlobalOne":   true,
			"globalTwo":   false,
			"GlobalThree": true,
		}

		for name, shouldBeExported := range expectedVars {
			symbols := findSymbolsByName(table, name)
			if len(symbols) == 0 {
				t.Errorf("Variable %s not found", name)
				continue
			}

			symbol := symbols[0]
			if symbol.Kind != types.SymbolKindVariable {
				t.Errorf("Expected %s to be a variable, got %v", name, symbol.Kind)
			}

			if symbol.IsExported != shouldBeExported {
				t.Errorf("Variable %s: expected IsExported=%v, got %v", name, shouldBeExported, symbol.IsExported)
			}
		}
	})

	t.Run("Struct extraction", func(t *testing.T) {
		// Check struct definitions
		publicStruct := findSymbolsByName(table, "PublicStruct")
		if len(publicStruct) == 0 {
			t.Error("PublicStruct not found")
		} else {
			if publicStruct[0].Kind != types.SymbolKindStruct {
				t.Errorf("Expected PublicStruct to be a struct, got %v", publicStruct[0].Kind)
			}
			if !publicStruct[0].IsExported {
				t.Error("PublicStruct should be exported")
			}
		}

		privateStruct := findSymbolsByName(table, "privateStruct")
		if len(privateStruct) == 0 {
			t.Error("privateStruct not found")
		} else {
			if privateStruct[0].Kind != types.SymbolKindStruct {
				t.Errorf("Expected privateStruct to be a struct, got %v", privateStruct[0].Kind)
			}
			if privateStruct[0].IsExported {
				t.Error("privateStruct should not be exported")
			}
		}

		// Check struct fields
		publicField := findSymbolsByName(table, "PublicStruct.PublicField")
		if len(publicField) == 0 {
			t.Error("PublicStruct.PublicField not found")
		} else {
			if publicField[0].Kind != types.SymbolKindField {
				t.Errorf("Expected field, got %v", publicField[0].Kind)
			}
			if !publicField[0].IsExported {
				t.Error("PublicField should be exported")
			}
		}

		privateField := findSymbolsByName(table, "PublicStruct.privateField")
		if len(privateField) == 0 {
			t.Error("PublicStruct.privateField not found")
		} else {
			if privateField[0].IsExported {
				t.Error("privateField should not be exported")
			}
		}
	})

	t.Run("Interface extraction", func(t *testing.T) {
		// Check interface definition
		publicInterface := findSymbolsByName(table, "PublicInterface")
		if len(publicInterface) == 0 {
			t.Error("PublicInterface not found")
		} else {
			if publicInterface[0].Kind != types.SymbolKindInterface {
				t.Errorf("Expected interface, got %v", publicInterface[0].Kind)
			}
			if !publicInterface[0].IsExported {
				t.Error("PublicInterface should be exported")
			}
		}

		// Check interface methods
		interfaceMethod := findSymbolsByName(table, "PublicInterface.PublicMethod")
		if len(interfaceMethod) == 0 {
			t.Error("PublicInterface.PublicMethod not found")
		} else {
			if interfaceMethod[0].Kind != types.SymbolKindMethod {
				t.Errorf("Expected method, got %v", interfaceMethod[0].Kind)
			}
		}
	})

	t.Run("Function extraction", func(t *testing.T) {
		// Check exported function
		publicFunc := findSymbolsByName(table, "PublicFunction")
		if len(publicFunc) == 0 {
			t.Error("PublicFunction not found")
		} else {
			if publicFunc[0].Kind != types.SymbolKindFunction {
				t.Errorf("Expected function, got %v", publicFunc[0].Kind)
			}
			if !publicFunc[0].IsExported {
				t.Error("PublicFunction should be exported")
			}

			// Check if signature was extracted
			if publicFunc[0].Signature == "" {
				t.Error("PublicFunction should have a signature")
			}
		}

		// Check private function
		privateFunc := findSymbolsByName(table, "privateFunction")
		if len(privateFunc) == 0 {
			t.Error("privateFunction not found")
		} else {
			if privateFunc[0].IsExported {
				t.Error("privateFunction should not be exported")
			}
		}

		// Check variadic function
		variadicFunc := findSymbolsByName(table, "VariadicFunc")
		if len(variadicFunc) == 0 {
			t.Error("VariadicFunc not found")
		}

		// Check init function
		initFunc := findSymbolsByName(table, "init")
		if len(initFunc) == 0 {
			t.Error("init function not found")
		}
	})

	t.Run("Method extraction", func(t *testing.T) {
		// Check methods on exported struct
		publicMethod := findSymbolsByName(table, "PublicStruct.PublicMethod")
		if len(publicMethod) == 0 {
			t.Error("PublicStruct.PublicMethod not found")
		} else {
			if publicMethod[0].Kind != types.SymbolKindMethod {
				t.Errorf("Expected method, got %v", publicMethod[0].Kind)
			}
			if !publicMethod[0].IsExported {
				t.Error("PublicMethod should be exported")
			}
		}

		// Check private method on exported struct
		privateMethod := findSymbolsByName(table, "PublicStruct.privateMethod")
		if len(privateMethod) == 0 {
			t.Error("PublicStruct.privateMethod not found")
		} else {
			if privateMethod[0].IsExported {
				t.Error("privateMethod should not be exported")
			}
		}

		// Check pointer receiver method
		setField := findSymbolsByName(table, "PublicStruct.SetField")
		if len(setField) == 0 {
			t.Error("PublicStruct.SetField not found")
		}
	})

	t.Run("Type extraction", func(t *testing.T) {
		// Check type alias
		typeAlias := findSymbolsByName(table, "TypeAlias")
		if len(typeAlias) == 0 {
			t.Error("TypeAlias not found")
		} else {
			if typeAlias[0].Kind != types.SymbolKindType {
				t.Errorf("Expected type, got %v", typeAlias[0].Kind)
			}
		}

		// Check custom type
		customType := findSymbolsByName(table, "CustomType")
		if len(customType) == 0 {
			t.Error("CustomType not found")
		}
	})

	t.Run("Local variable extraction", func(t *testing.T) {
		// Check that local variables within functions are extracted
		localVar := findSymbolsByName(table, "localVar")
		if len(localVar) == 0 {
			t.Error("Local variable 'localVar' not found")
		} else {
			if localVar[0].IsExported {
				t.Error("Local variables should never be exported")
			}
		}

		// Check short variable declarations
		xVar := findSymbolsByName(table, "x")
		yVar := findSymbolsByName(table, "y")
		if len(xVar) == 0 {
			t.Error("Short var 'x' not found")
		}
		if len(yVar) == 0 {
			t.Error("Short var 'y' not found")
		}
	})

	t.Run("Goroutine extraction", func(t *testing.T) {
		// Check that goroutine launches are tracked
		goSymbols := findSymbolsByPrefix(table, "go ")
		if len(goSymbols) == 0 {
			t.Error("No goroutine symbols found")
		} else {
			// Should find at least "go fmt.Println"
			found := false
			for _, sym := range goSymbols {
				if sym.Name == "go Println" || sym.Name == "go fmt" {
					found = true
					break
				}
			}
			if !found {
				t.Error("Expected to find goroutine launch symbols")
			}
		}
	})

	t.Run("Parameter extraction", func(t *testing.T) {
		// Check function parameters
		param1 := findSymbolsByName(table, "param1")
		param2 := findSymbolsByName(table, "param2")

		if len(param1) == 0 {
			t.Error("Parameter 'param1' not found")
		} else {
			if param1[0].Kind != types.SymbolKindParameter {
				t.Errorf("Expected parameter, got %v", param1[0].Kind)
			}
		}

		if len(param2) == 0 {
			t.Error("Parameter 'param2' not found")
		}

		// Check variadic parameter
		values := findSymbolsByName(table, "...values")
		if len(values) == 0 {
			t.Error("Variadic parameter 'values' not found")
		}
	})

	t.Run("Symbol count", func(t *testing.T) {
		// Ensure we're extracting a reasonable number of symbols
		if len(table.Symbols) < 30 {
			t.Errorf("Expected at least 30 symbols, got %d", len(table.Symbols))
		}

		t.Logf("Total symbols extracted: %d", len(table.Symbols))

		// Log symbol distribution by kind
		kindCounts := make(map[types.SymbolKind]int)
		for _, sym := range table.Symbols {
			kindCounts[sym.Kind]++
		}

		for kind, count := range kindCounts {
			t.Logf("  %s: %d", kind.String(), count)
		}
	})
}

// Helper function to find symbols by name
func findSymbolsByName(table *types.SymbolTable, name string) []*types.EnhancedSymbolInfo {
	var results []*types.EnhancedSymbolInfo

	for _, symbol := range table.Symbols {
		if symbol.Name == name {
			results = append(results, symbol)
		}
	}

	return results
}

// Helper function to find symbols by prefix
func findSymbolsByPrefix(table *types.SymbolTable, prefix string) []*types.EnhancedSymbolInfo {
	var results []*types.EnhancedSymbolInfo

	for _, symbol := range table.Symbols {
		if len(symbol.Name) >= len(prefix) && symbol.Name[:len(prefix)] == prefix {
			results = append(results, symbol)
		}
	}

	return results
}

// TestGoExtractorCanHandle tests the go extractor can handle.
func TestGoExtractorCanHandle(t *testing.T) {
	extractor := NewGoExtractor()

	tests := []struct {
		filepath string
		expected bool
	}{
		{"main.go", true},
		{"test.go", true},
		{"file.GO", false}, // Case sensitive
		{"test.js", false},
		{"test.py", false},
		{"/path/to/file.go", true},
		{"", false},
	}

	for _, test := range tests {
		if extractor.CanHandle(test.filepath) != test.expected {
			t.Errorf("CanHandle(%q) = %v, expected %v",
				test.filepath, !test.expected, test.expected)
		}
	}
}

// TestGoExtractorLanguage tests the go extractor language.
func TestGoExtractorLanguage(t *testing.T) {
	extractor := NewGoExtractor()

	if extractor.GetLanguage() != "go" {
		t.Errorf("Expected language 'go', got %s", extractor.GetLanguage())
	}
}
