package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestPhase4ParserIntegration tests that all languages work together
func TestPhase4ParserIntegration(t *testing.T) {
	parser := NewTreeSitterParser()

	testCases := []struct {
		name     string
		filename string
		content  string
		expected int // minimum expected symbols
	}{
		{
			"JavaScript",
			"test.js",
			`function hello() { return "world"; }
class Test { method() { return 42; } }`,
			3, // hello function, Test class, method
		},
		{
			"Rust",
			"test.rs",
			`fn main() { println!("Hello"); }
struct Point { x: i32, y: i32 }
impl Point { fn new() -> Point { Point { x: 0, y: 0 } } }`,
			4, // main function, Point struct, impl, new function
		},
		{
			"C++",
			"test.cpp",
			`int add(int a, int b) { return a + b; }
class Calculator { public: int multiply(int x, int y); };`,
			2, // add function, Calculator class (method declarations inside class are not extracted)
		},
		{
			"Java",
			"Test.java",
			`public class Test {
    public void method1() { System.out.println("Hello"); }
    private int method2() { return 42; }
}`,
			3, // Test class, method1, method2
		},
		{
			"Go",
			"test.go",
			`package main
func main() { fmt.Println("Hello") }
type Person struct { Name string }`,
			2, // main function, Person type
		},
		{
			"Python",
			"test.py",
			`def hello():
    print("world")

class Test:
    def method(self):
        return 42`,
			3, // hello function, Test class, method
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blocks, symbols, imports := parser.ParseFile(tc.filename, []byte(tc.content))

			if len(symbols) < tc.expected {
				t.Errorf("Expected at least %d symbols for %s, got %d", tc.expected, tc.name, len(symbols))
				t.Logf("Symbols found:")
				for i, symbol := range symbols {
					t.Logf("  %d: %s (%s) at line %d", i, symbol.Name, symbol.Type, symbol.Line)
				}
				t.Logf("Blocks found:")
				for i, block := range blocks {
					t.Logf("  %d: %s (%s) lines %d-%d", i, block.Name, block.Type, block.Start, block.End)
				}
				t.Logf("Imports found:")
				for i, imp := range imports {
					t.Logf("  %d: %s at line %d", i, imp.Path, imp.Line)
				}
			} else {
				t.Logf("✅ %s parser extracted %d symbols correctly", tc.name, len(symbols))
			}
		})
	}
}

// TestPhase4SymbolTypes validates that new symbol types are working
func TestPhase4SymbolTypes(t *testing.T) {
	parser := NewTreeSitterParser()

	// Test Rust-specific constructs
	rustCode := `
fn function_test() {}
struct MyStruct { field: i32 }
trait MyTrait { fn trait_method(&self); }
impl MyStruct { fn impl_method(&self) {} }
mod my_module { pub fn inner_fn() {} }
`

	_, symbols, _ := parser.ParseFile("test.rs", []byte(rustCode))

	symbolTypes := make(map[types.SymbolType]int)
	for _, symbol := range symbols {
		symbolTypes[symbol.Type]++
	}

	expectedTypes := []types.SymbolType{
		types.SymbolTypeFunction,
		types.SymbolTypeStruct,
		types.SymbolTypeTrait, // Rust traits are now their own type
		types.SymbolTypeModule,
		// Note: SymbolTypeClass not expected for Rust impl blocks (parser limitation)
	}

	for _, expectedType := range expectedTypes {
		if count, exists := symbolTypes[expectedType]; !exists || count == 0 {
			t.Errorf("Expected symbol type %s not found in Rust parsing", expectedType.String())
		} else {
			t.Logf("✅ Found %d symbols of type %s", count, expectedType.String())
		}
	}
}

// TestPhase4FileExtensions validates that all file extensions are recognized
func TestPhase4FileExtensions(t *testing.T) {
	parser := NewTreeSitterParser()

	extensions := []struct {
		ext      string
		language string
	}{
		{".js", "javascript"},
		{".jsx", "javascript"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".go", "go"},
		{".py", "python"},
		{".rs", "rust"},
		{".cpp", "cpp"},
		{".cc", "cpp"},
		{".cxx", "cpp"},
		{".c", "cpp"},
		{".h", "cpp"},
		{".hpp", "cpp"},
		{".java", "java"},
	}

	for _, ext := range extensions {
		t.Run(ext.ext, func(t *testing.T) {
			detected := parser.getLanguageFromExt(ext.ext)
			if detected != ext.language {
				t.Errorf("Extension %s: expected language %s, got %s", ext.ext, ext.language, detected)
			} else {
				t.Logf("✅ Extension %s correctly detected as %s", ext.ext, detected)
			}

			// Verify parser can be initialized for this extension
			// The parser uses lazy initialization, so check if it's registered
			canInit := false
			for lang, exts := range parser.langGroups {
				for _, e := range exts {
					if e == ext.ext {
						canInit = true
						t.Logf("✅ Parser registered for extension %s (language: %s)", ext.ext, lang)
						break
					}
				}
				if canInit {
					break
				}
			}

			if !canInit {
				t.Errorf("No parser registered for extension %s", ext.ext)
			}
		})
	}
}
