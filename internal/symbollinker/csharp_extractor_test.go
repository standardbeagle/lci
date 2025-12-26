package symbollinker

import (
	"os"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// TestCSharpExtractor tests the c sharp extractor.
func TestCSharpExtractor(t *testing.T) {
	// Read test file
	content, err := os.ReadFile("testdata/csharp_project/Simple.cs")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_csharp.Language())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse C# code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewCSharpExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Language verification", func(t *testing.T) {
		if table == nil {
			t.Fatal("Symbol table is nil")
		}

		if table.Language != "csharp" {
			t.Errorf("Expected language 'csharp', got %s", table.Language)
		}
	})

	t.Run("Namespace extraction", func(t *testing.T) {
		// Check for namespace declarations
		myAppExamples := findSymbolsByName(table, "MyApp.Examples")
		if len(myAppExamples) == 0 {
			t.Error("Namespace 'MyApp.Examples' not found")
		} else {
			symbol := myAppExamples[0]
			if symbol.Kind != types.SymbolKindNamespace {
				t.Errorf("Expected namespace, got %v", symbol.Kind)
			}
		}

		myAppUtilities := findSymbolsByName(table, "MyApp.Utilities")
		if len(myAppUtilities) == 0 {
			t.Error("Namespace 'MyApp.Utilities' not found")
		}
	})

	t.Run("Using statement extraction", func(t *testing.T) {
		expectedUsings := map[string]string{
			"System":                     "System",
			"System.Collections.Generic": "System.Collections.Generic",
			"System.ComponentModel":      "System.ComponentModel",
			"System.Threading.Tasks":     "System.Threading.Tasks",
		}

		if len(table.Imports) < len(expectedUsings) {
			t.Errorf("Expected at least %d imports, got %d", len(expectedUsings), len(table.Imports))
		}

		foundUsings := make(map[string]bool)
		for _, imp := range table.Imports {
			foundUsings[imp.ImportPath] = true
		}

		for expectedUsing := range expectedUsings {
			if !foundUsings[expectedUsing] {
				t.Errorf("Using statement '%s' not found", expectedUsing)
			}
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
				t.Error("SimpleClass should be exported (public)")
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

		// Check sealed class
		finalClass := findSymbolsByName(table, "FinalClass")
		if len(finalClass) == 0 {
			t.Error("FinalClass not found")
		}
	})

	t.Run("Interface extraction", func(t *testing.T) {
		// Check interface
		exampleInterface := findSymbolsByName(table, "IExampleInterface")
		if len(exampleInterface) == 0 {
			t.Error("IExampleInterface not found")
		} else {
			symbol := exampleInterface[0]
			if symbol.Kind != types.SymbolKindInterface {
				t.Errorf("Expected interface, got %v", symbol.Kind)
			}
		}

		// Check generic interface
		genericInterface := findSymbolsByName(table, "IGenericInterface")
		if len(genericInterface) == 0 {
			t.Error("IGenericInterface not found")
		}
	})

	t.Run("Struct extraction", func(t *testing.T) {
		simpleStruct := findSymbolsByName(table, "SimpleStruct")
		if len(simpleStruct) == 0 {
			t.Error("SimpleStruct not found")
		} else {
			symbol := simpleStruct[0]
			if symbol.Kind != types.SymbolKindStruct {
				t.Errorf("Expected struct, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Property extraction", func(t *testing.T) {
		// Check properties with different accessors
		publicProperty := findSymbolsByName(table, "SimpleClass.PublicProperty")
		if len(publicProperty) == 0 {
			t.Error("SimpleClass.PublicProperty not found")
		} else {
			symbol := publicProperty[0]
			if symbol.Kind != types.SymbolKindProperty {
				t.Errorf("Expected property, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("PublicProperty should be exported")
			}
		}

		// Check auto property
		autoProperty := findSymbolsByName(table, "SimpleClass.AutoProperty")
		if len(autoProperty) == 0 {
			t.Error("SimpleClass.AutoProperty not found")
		}

		// Check private property
		privateProperty := findSymbolsByName(table, "SimpleClass.PrivateProperty")
		if len(privateProperty) == 0 {
			t.Error("SimpleClass.PrivateProperty not found")
		} else {
			if privateProperty[0].IsExported {
				t.Error("PrivateProperty should not be exported")
			}
		}
	})

	t.Run("Field extraction", func(t *testing.T) {
		// Check public field
		publicField := findSymbolsByName(table, "SimpleClass.PublicField")
		if len(publicField) == 0 {
			t.Error("SimpleClass.PublicField not found")
		} else {
			symbol := publicField[0]
			if symbol.Kind != types.SymbolKindField {
				t.Errorf("Expected field, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("PublicField should be exported")
			}
		}

		// Check private field
		privateField := findSymbolsByName(table, "SimpleClass._privateField")
		if len(privateField) == 0 {
			t.Error("SimpleClass._privateField not found")
		} else {
			if privateField[0].IsExported {
				t.Error("_privateField should not be exported")
			}
		}
	})

	t.Run("Method extraction", func(t *testing.T) {
		// Check public method
		publicMethod := findSymbolsByName(table, "SimpleClass.PublicMethod")
		if len(publicMethod) == 0 {
			t.Error("SimpleClass.PublicMethod not found")
		} else {
			symbol := publicMethod[0]
			if symbol.Kind != types.SymbolKindMethod {
				t.Errorf("Expected method, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("PublicMethod should be exported")
			}
			if symbol.Signature == "" {
				t.Error("PublicMethod should have a signature")
			}
		}

		// Check private method
		privateMethod := findSymbolsByName(table, "SimpleClass.PrivateMethod")
		if len(privateMethod) == 0 {
			t.Error("SimpleClass.PrivateMethod not found")
		} else {
			if privateMethod[0].IsExported {
				t.Error("PrivateMethod should not be exported")
			}
		}

		// Check static method
		staticMethod := findSymbolsByName(table, "SimpleClass.StaticMethod")
		if len(staticMethod) == 0 {
			t.Error("SimpleClass.StaticMethod not found")
		}

		// Check async method
		asyncMethod := findSymbolsByName(table, "SimpleClass.AsyncMethod")
		if len(asyncMethod) == 0 {
			t.Error("SimpleClass.AsyncMethod not found")
		}
	})

	t.Run("Constructor extraction", func(t *testing.T) {
		// Check default constructor
		defaultConstructor := findSymbolsByName(table, "SimpleClass.SimpleClass")
		if len(defaultConstructor) == 0 {
			// Try alternative naming
			defaultConstructor = findSymbolsByName(table, "SimpleClass..ctor")
		}
		if len(defaultConstructor) == 0 {
			t.Error("SimpleClass constructor not found")
		} else {
			symbol := defaultConstructor[0]
			if symbol.Kind != types.SymbolKindConstructor {
				t.Errorf("Expected constructor, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Constant extraction", func(t *testing.T) {
		// Check class constants
		publicConst := findSymbolsByName(table, "SimpleClass.PUBLIC_CONSTANT")
		if len(publicConst) == 0 {
			t.Error("SimpleClass.PUBLIC_CONSTANT not found")
		} else {
			symbol := publicConst[0]
			if symbol.Kind != types.SymbolKindConstant {
				t.Errorf("Expected constant, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("PUBLIC_CONSTANT should be exported")
			}
		}

		privateConst := findSymbolsByName(table, "SimpleClass.PRIVATE_CONSTANT")
		if len(privateConst) == 0 {
			t.Error("SimpleClass.PRIVATE_CONSTANT not found")
		} else {
			if privateConst[0].IsExported {
				t.Error("PRIVATE_CONSTANT should not be exported")
			}
		}
	})

	t.Run("Event extraction", func(t *testing.T) {
		// Check events
		propertyChanged := findSymbolsByName(table, "SimpleClass.PropertyChanged")
		if len(propertyChanged) == 0 {
			t.Error("SimpleClass.PropertyChanged event not found")
		} else {
			symbol := propertyChanged[0]
			if symbol.Kind != types.SymbolKindEvent {
				t.Errorf("Expected event, got %v", symbol.Kind)
			}
		}

		customEvent := findSymbolsByName(table, "SimpleClass.CustomEvent")
		if len(customEvent) == 0 {
			t.Error("SimpleClass.CustomEvent not found")
		}
	})

	t.Run("Enum extraction", func(t *testing.T) {
		// Check enums
		statusEnum := findSymbolsByName(table, "Status")
		if len(statusEnum) == 0 {
			t.Error("Status enum not found")
		} else {
			symbol := statusEnum[0]
			if symbol.Kind != types.SymbolKindEnum {
				t.Errorf("Expected enum, got %v", symbol.Kind)
			}
		}

		// Check enum members
		pendingMember := findSymbolsByName(table, "Status.Pending")
		if len(pendingMember) == 0 {
			t.Error("Status.Pending not found")
		} else {
			if pendingMember[0].Kind != types.SymbolKindEnumMember {
				t.Errorf("Expected enum member, got %v", pendingMember[0].Kind)
			}
		}

		activeMember := findSymbolsByName(table, "Status.Active")
		if len(activeMember) == 0 {
			t.Error("Status.Active not found")
		}
	})

	t.Run("Delegate extraction", func(t *testing.T) {
		// Check delegate
		simpleDelegate := findSymbolsByName(table, "SimpleDelegate")
		if len(simpleDelegate) == 0 {
			t.Error("SimpleDelegate not found")
		} else {
			symbol := simpleDelegate[0]
			if symbol.Kind != types.SymbolKindDelegate {
				t.Errorf("Expected delegate, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Record extraction", func(t *testing.T) {
		// Check record
		personRecord := findSymbolsByName(table, "PersonRecord")
		if len(personRecord) == 0 {
			t.Error("PersonRecord not found")
		} else {
			symbol := personRecord[0]
			if symbol.Kind != types.SymbolKindRecord {
				t.Errorf("Expected record, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Symbol count", func(t *testing.T) {
		// Ensure we're extracting a reasonable number of symbols
		if len(table.Symbols) < 50 {
			t.Errorf("Expected at least 50 symbols, got %d", len(table.Symbols))
		}

		t.Logf("Total C# symbols extracted: %d", len(table.Symbols))

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

// TestCSharpExtractorCanHandle tests the c sharp extractor can handle.
func TestCSharpExtractorCanHandle(t *testing.T) {
	extractor := NewCSharpExtractor()

	tests := []struct {
		filepath string
		expected bool
	}{
		{"Program.cs", true},
		{"Class.cs", true},
		{"file.CS", false}, // Case sensitive
		{"test.csx", true},
		{"test.js", false},
		{"test.py", false},
		{"/path/to/file.cs", true},
		{"", false},
	}

	for _, test := range tests {
		if extractor.CanHandle(test.filepath) != test.expected {
			t.Errorf("CanHandle(%q) = %v, expected %v",
				test.filepath, !test.expected, test.expected)
		}
	}
}

// TestCSharpExtractorLanguage tests the c sharp extractor language.
func TestCSharpExtractorLanguage(t *testing.T) {
	extractor := NewCSharpExtractor()

	if extractor.GetLanguage() != "csharp" {
		t.Errorf("Expected language 'csharp', got %s", extractor.GetLanguage())
	}
}
