package symbollinker

import (
	"os"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// TestPythonExtractor tests the python extractor.
func TestPythonExtractor(t *testing.T) {
	// Read test file
	content, err := os.ReadFile("testdata/python_project/simple.py")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_python.Language())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse Python code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewPythonExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Language verification", func(t *testing.T) {
		if table == nil {
			t.Fatal("Symbol table is nil")
		}

		if table.Language != "python" {
			t.Errorf("Expected language 'python', got %s", table.Language)
		}
	})

	t.Run("Import statement extraction", func(t *testing.T) {
		expectedImports := map[string]string{
			"os":          "os",
			"sys":         "sys",
			"typing":      "typing",
			"collections": "collections",
			"dataclasses": "dataclasses",
			"abc":         "abc",
			"enum":        "enum",
			"asyncio":     "asyncio",
		}

		if len(table.Imports) < len(expectedImports) {
			t.Errorf("Expected at least %d imports, got %d", len(expectedImports), len(table.Imports))
		}

		foundImports := make(map[string]bool)
		for _, imp := range table.Imports {
			foundImports[imp.ImportPath] = true
		}

		for expectedImport := range expectedImports {
			if !foundImports[expectedImport] {
				t.Errorf("Import statement '%s' not found", expectedImport)
			}
		}

		// Check for alias imports
		foundAliases := make(map[string]string)
		for _, imp := range table.Imports {
			if imp.Alias != "" {
				foundAliases[imp.Alias] = imp.ImportPath
			}
		}

		if foundAliases["json_module"] != "json" {
			t.Error("Alias import 'json as json_module' not found")
		}
	})

	t.Run("Class extraction", func(t *testing.T) {
		// Check main class
		simpleClass := findSymbolsByName(table, "SimpleClass")
		if len(simpleClass) == 0 {
			t.Error("SimpleClass not found")
		} else {
			symbol := simpleClass[0]
			if symbol.Kind != types.SymbolKindClass {
				t.Errorf("Expected class, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("SimpleClass should be exported")
			}
		}

		// Check abstract class
		abstractBase := findSymbolsByName(table, "AbstractBase")
		if len(abstractBase) == 0 {
			t.Error("AbstractBase not found")
		} else {
			if abstractBase[0].Kind != types.SymbolKindClass {
				t.Errorf("Expected class, got %v", abstractBase[0].Kind)
			}
		}

		// Check derived class
		derivedClass := findSymbolsByName(table, "DerivedClass")
		if len(derivedClass) == 0 {
			t.Error("DerivedClass not found")
		}

		// Check generic class
		genericClass := findSymbolsByName(table, "GenericClass")
		if len(genericClass) == 0 {
			t.Error("GenericClass not found")
		}
	})

	t.Run("Method extraction", func(t *testing.T) {
		// Check public method
		publicMethod := findSymbolsByName(table, "SimpleClass.public_method")
		if len(publicMethod) == 0 {
			t.Error("SimpleClass.public_method not found")
		} else {
			symbol := publicMethod[0]
			if symbol.Kind != types.SymbolKindMethod {
				t.Errorf("Expected method, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("public_method should be exported")
			}
			if symbol.Signature == "" {
				t.Error("public_method should have a signature")
			}
		}

		// Check private method (starts with _)
		privateMethod := findSymbolsByName(table, "SimpleClass._private_method")
		if len(privateMethod) == 0 {
			t.Error("SimpleClass._private_method not found")
		} else {
			if privateMethod[0].IsExported {
				t.Error("_private_method should not be exported")
			}
		}

		// Check static method
		staticMethod := findSymbolsByName(table, "SimpleClass.static_method")
		if len(staticMethod) == 0 {
			t.Error("SimpleClass.static_method not found")
		}

		// Check class method
		classMethod := findSymbolsByName(table, "SimpleClass.class_method")
		if len(classMethod) == 0 {
			t.Error("SimpleClass.class_method not found")
		}

		// Check async method
		asyncMethod := findSymbolsByName(table, "SimpleClass.async_method")
		if len(asyncMethod) == 0 {
			t.Error("SimpleClass.async_method not found")
		}

		// Check dunder methods
		strMethod := findSymbolsByName(table, "SimpleClass.__str__")
		if len(strMethod) == 0 {
			t.Error("SimpleClass.__str__ not found")
		}
	})

	t.Run("Property extraction", func(t *testing.T) {
		// Check property getter
		nameProperty := findSymbolsByName(table, "SimpleClass.name_property")
		if len(nameProperty) == 0 {
			t.Error("SimpleClass.name_property not found")
		} else {
			// Find the property symbol (there might be duplicates)
			foundProperty := false
			for _, symbol := range nameProperty {
				if symbol.Kind == types.SymbolKindProperty {
					foundProperty = true
					break
				}
			}
			if !foundProperty {
				t.Errorf("Expected property, but no property symbol found among %d symbols", len(nameProperty))
			}
		}
	})

	t.Run("Attribute extraction", func(t *testing.T) {
		// Check instance attributes
		nameAttr := findSymbolsByName(table, "SimpleClass.name")
		if len(nameAttr) == 0 {
			t.Error("SimpleClass.name attribute not found")
		} else {
			symbol := nameAttr[0]
			if symbol.Kind != types.SymbolKindAttribute {
				t.Errorf("Expected attribute, got %v", symbol.Kind)
			}
		}

		// Check private attribute
		privateAttr := findSymbolsByName(table, "SimpleClass._private_attr")
		if len(privateAttr) == 0 {
			t.Error("SimpleClass._private_attr not found")
		} else {
			if privateAttr[0].IsExported {
				t.Error("_private_attr should not be exported")
			}
		}

		// Check class variable
		classVar := findSymbolsByName(table, "SimpleClass.class_variable")
		if len(classVar) == 0 {
			t.Error("SimpleClass.class_variable not found")
		}
	})

	t.Run("Function extraction", func(t *testing.T) {
		// Check global function
		globalFunc := findSymbolsByName(table, "global_function")
		if len(globalFunc) == 0 {
			t.Error("global_function not found")
		} else {
			symbol := globalFunc[0]
			if symbol.Kind != types.SymbolKindFunction {
				t.Errorf("Expected function, got %v", symbol.Kind)
			}
			if symbol.Signature == "" {
				t.Error("global_function should have a signature")
			}
			if !symbol.IsExported {
				t.Error("global_function should be exported")
			}
		}

		// Check private function (starts with _)
		privateFunc := findSymbolsByName(table, "_private_function")
		if len(privateFunc) == 0 {
			t.Error("_private_function not found")
		} else {
			if privateFunc[0].IsExported {
				t.Error("_private_function should not be exported")
			}
		}

		// Check async function
		asyncFunc := findSymbolsByName(table, "async_global_function")
		if len(asyncFunc) == 0 {
			t.Error("async_global_function not found")
		}

		// Check variadic function
		variadicFunc := findSymbolsByName(table, "variadic_function")
		if len(variadicFunc) == 0 {
			t.Error("variadic_function not found")
		}

		// Check generator function
		generatorFunc := findSymbolsByName(table, "number_generator")
		if len(generatorFunc) == 0 {
			t.Error("number_generator not found")
		}

		// Check decorated function
		decoratedFunc := findSymbolsByName(table, "decorated_function")
		if len(decoratedFunc) == 0 {
			t.Error("decorated_function not found")
		}
	})

	t.Run("Variable extraction", func(t *testing.T) {
		// Check global constants
		globalConst := findSymbolsByName(table, "GLOBAL_CONSTANT")
		if len(globalConst) == 0 {
			t.Error("GLOBAL_CONSTANT not found")
		} else {
			symbol := globalConst[0]
			if symbol.Kind != types.SymbolKindConstant {
				t.Errorf("Expected constant, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("GLOBAL_CONSTANT should be exported")
			}
		}

		// Check private constant
		privateConst := findSymbolsByName(table, "_PRIVATE_CONSTANT")
		if len(privateConst) == 0 {
			t.Error("_PRIVATE_CONSTANT not found")
		} else {
			if privateConst[0].IsExported {
				t.Error("_PRIVATE_CONSTANT should not be exported")
			}
		}

		// Check global variables
		globalVar := findSymbolsByName(table, "global_var")
		if len(globalVar) == 0 {
			t.Error("global_var not found")
		} else {
			symbol := globalVar[0]
			if symbol.Kind != types.SymbolKindVariable {
				t.Errorf("Expected variable, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Enum extraction", func(t *testing.T) {
		// Check enum class
		statusEnum := findSymbolsByName(table, "Status")
		if len(statusEnum) == 0 {
			t.Error("Status enum not found")
		} else {
			symbol := statusEnum[0]
			if symbol.Kind != types.SymbolKindEnum {
				t.Errorf("Expected enum, got %v", symbol.Kind)
			}
		}

		// Check IntEnum
		priorityEnum := findSymbolsByName(table, "Priority")
		if len(priorityEnum) == 0 {
			t.Error("Priority enum not found")
		}

		// Check enum members
		pendingMember := findSymbolsByName(table, "Status.PENDING")
		if len(pendingMember) == 0 {
			t.Error("Status.PENDING not found")
		} else {
			if pendingMember[0].Kind != types.SymbolKindEnumMember {
				t.Errorf("Expected enum member, got %v", pendingMember[0].Kind)
			}
		}
	})

	t.Run("Dataclass extraction", func(t *testing.T) {
		// Check dataclass
		personDataclass := findSymbolsByName(table, "Person")
		if len(personDataclass) == 0 {
			t.Error("Person dataclass not found")
		} else {
			symbol := personDataclass[0]
			if symbol.Kind != types.SymbolKindClass { // Dataclasses are classes
				t.Errorf("Expected class (dataclass), got %v", symbol.Kind)
			}
		}

		// Check dataclass method
		getDisplayName := findSymbolsByName(table, "Person.get_display_name")
		if len(getDisplayName) == 0 {
			t.Error("Person.get_display_name not found")
		}
	})

	t.Run("Type alias extraction", func(t *testing.T) {
		// Check type aliases
		stringDict := findSymbolsByName(table, "StringDict")
		if len(stringDict) == 0 {
			t.Error("StringDict type alias not found")
		} else {
			symbol := stringDict[0]
			if symbol.Kind != types.SymbolKindType {
				t.Errorf("Expected type alias, got %v", symbol.Kind)
			}
		}

		optionalString := findSymbolsByName(table, "OptionalString")
		if len(optionalString) == 0 {
			t.Error("OptionalString type alias not found")
		}
	})

	t.Run("Exception class extraction", func(t *testing.T) {
		// Check custom exception
		customException := findSymbolsByName(table, "CustomException")
		if len(customException) == 0 {
			t.Error("CustomException not found")
		} else {
			symbol := customException[0]
			if symbol.Kind != types.SymbolKindClass {
				t.Errorf("Expected class (exception), got %v", symbol.Kind)
			}
		}

		specificError := findSymbolsByName(table, "SpecificError")
		if len(specificError) == 0 {
			t.Error("SpecificError not found")
		}
	})

	t.Run("Lambda extraction", func(t *testing.T) {
		// Check lambda functions
		simpleLambda := findSymbolsByName(table, "simple_lambda")
		if len(simpleLambda) == 0 {
			t.Error("simple_lambda not found")
		} else {
			symbol := simpleLambda[0]
			if symbol.Kind != types.SymbolKindFunction {
				t.Errorf("Expected function (lambda), got %v", symbol.Kind)
			}
		}

		complexLambda := findSymbolsByName(table, "complex_lambda")
		if len(complexLambda) == 0 {
			t.Error("complex_lambda not found")
		}
	})

	t.Run("Symbol count", func(t *testing.T) {
		// Ensure we're extracting a reasonable number of symbols
		if len(table.Symbols) < 60 {
			t.Errorf("Expected at least 60 symbols, got %d", len(table.Symbols))
		}

		t.Logf("Total Python symbols extracted: %d", len(table.Symbols))

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

// TestPythonExtractorCanHandle tests the python extractor can handle.
func TestPythonExtractorCanHandle(t *testing.T) {
	extractor := NewPythonExtractor()

	tests := []struct {
		filepath string
		expected bool
	}{
		{"main.py", true},
		{"module.py", true},
		{"file.PY", false}, // Case sensitive
		{"script.pyw", true},
		{"interface.pyi", true},
		{"extension.pyx", true},
		{"test.js", false},
		{"test.php", false},
		{"/path/to/file.py", true},
		{"", false},
	}

	for _, test := range tests {
		if extractor.CanHandle(test.filepath) != test.expected {
			t.Errorf("CanHandle(%q) = %v, expected %v",
				test.filepath, !test.expected, test.expected)
		}
	}
}

// TestPythonExtractorLanguage tests the python extractor language.
func TestPythonExtractorLanguage(t *testing.T) {
	extractor := NewPythonExtractor()

	if extractor.GetLanguage() != "python" {
		t.Errorf("Expected language 'python', got %s", extractor.GetLanguage())
	}
}
