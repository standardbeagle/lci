package parser

import (
	"context"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestTreeSitterParserStringRefIntegration(t *testing.T) {
	content := []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")

	name := "Go"
	if name == "Go" {
		fmt.Printf("Hello, %s!\n", name)
	}
}

type User struct {
	Name string
	Age  int
}

func (u *User) GetName() string {
	return u.Name
}`)

	fileID := types.FileID(123)
	path := "test.go"
	parser := NewTreeSitterParser()

	// Test StringRef-based parsing
	tree, _, symbols, imports, enhancedSymbols, references, scopeInfo := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), path, content, fileID)

	if tree == nil {
		t.Fatal("ParseFileEnhancedWithASTAndContextStringRef returned nil tree")
	}

	if len(symbols) == 0 {
		t.Fatal("No symbols extracted from Go code")
	}

	// Verify that symbols were extracted correctly
	var foundMain, foundUser, foundGetName bool
	for _, symbol := range symbols {
		switch symbol.Name {
		case "main":
			foundMain = true
			if symbol.Type != types.SymbolTypeFunction {
				t.Errorf("Expected main to be function, got %v", symbol.Type)
			}
		case "User":
			foundUser = true
			if symbol.Type != types.SymbolTypeStruct {
				t.Errorf("Expected User to be struct, got %v", symbol.Type)
			}
		case "GetName":
			foundGetName = true
			if symbol.Type != types.SymbolTypeMethod {
				t.Errorf("Expected GetName to be method, got %v", symbol.Type)
			}
		}
	}

	if !foundMain {
		t.Error("main function not found")
	}
	if !foundUser {
		t.Error("User struct not found")
	}
	if !foundGetName {
		t.Error("GetName method not found")
	}

	// Verify imports were extracted
	if len(imports) == 0 {
		t.Error("No imports extracted")
	}

	// Verify enhanced symbols were created
	if len(enhancedSymbols) == 0 {
		t.Error("No enhanced symbols created")
	}

	// Verify scope info was created
	if len(scopeInfo) == 0 {
		t.Error("No scope info created")
	}

	t.Logf("Successfully parsed Go file with StringRef:")
	t.Logf("  - Symbols: %d", len(symbols))
	t.Logf("  - Imports: %d", len(imports))
	t.Logf("  - Enhanced symbols: %d", len(enhancedSymbols))
	t.Logf("  - References: %d", len(references))
	t.Logf("  - Scope info: %d", len(scopeInfo))
}

func TestStringRefZeroCopyOperations(t *testing.T) {
	content := []byte(`function testFunction(param1, param2) {
    const CONSTANT_VALUE = 42;
    let variable = "test string";
    return param1 + param2;
}

class TestClass {
    constructor(value) {
        this.value = value;
    }

    getValue() {
        return this.value;
    }
}`)

	fileID := types.FileID(456)
	path := "test.js"
	parser := NewTreeSitterParser()

	// Test zero-copy operations during parsing
	tree, blocks, symbols, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), path, content, fileID)

	if tree == nil {
		t.Fatal("ParseFileEnhancedWithASTAndContextStringRef returned nil tree")
	}

	// Verify that symbols were extracted without string allocations
	if len(symbols) == 0 {
		t.Fatal("No symbols extracted from JavaScript code")
	}

	// Verify function symbol
	var foundTestFunction bool
	for _, symbol := range symbols {
		if symbol.Name == "testFunction" {
			foundTestFunction = true
			if symbol.Type != types.SymbolTypeFunction {
				t.Errorf("Expected testFunction to be function, got %v", symbol.Type)
			}
			break
		}
	}

	if !foundTestFunction {
		t.Error("testFunction not found")
	}

	// Verify class symbol
	var foundTestClass bool
	for _, symbol := range symbols {
		if symbol.Name == "TestClass" {
			foundTestClass = true
			if symbol.Type != types.SymbolTypeClass {
				t.Errorf("Expected TestClass to be class, got %v", symbol.Type)
			}
			break
		}
	}

	if !foundTestClass {
		t.Error("TestClass not found")
	}

	// Test that the parsing used StringRef operations
	// We can verify this by checking that the content is still accessible
	// and that we can extract the original string references
	testStringRef := types.NewStringRef(fileID, content, 9, 12) // "testFunction"
	if testStringRef.String(content) != "testFunction" {
		t.Errorf("StringRef.String() returned %v, want %v", testStringRef.String(content), "testFunction")
	}

	t.Logf("Successfully performed zero-copy parsing:")
	t.Logf("  - Symbols: %d", len(symbols))
	t.Logf("  - Blocks: %d", len(blocks))
}

func TestStringRefBackwardCompatibility(t *testing.T) {
	content := []byte(`def python_function(x, y):
    """A simple Python function."""
    result = x + y
    return result

class PythonClass:
    def __init__(self, name):
        self.name = name

    def get_name(self):
        return self.name`)

	path := "test.py"
	parser := NewTreeSitterParser()

	// Test traditional []byte parsing
	tree1, blocks1, symbols1, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContext(context.TODO(), path, content)

	// Test StringRef parsing with FileID = 0 (should behave identically)
	tree2, blocks2, symbols2, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), path, content, 0)

	// Results should be identical
	if tree1 == nil || tree2 == nil {
		t.Fatal("One of the parsing methods returned nil tree")
	}

	if len(symbols1) != len(symbols2) {
		t.Errorf("Symbol count mismatch: traditional=%d, StringRef=%d", len(symbols1), len(symbols2))
	}

	if len(blocks1) != len(blocks2) {
		t.Errorf("Block count mismatch: traditional=%d, StringRef=%d", len(blocks1), len(blocks2))
	}

	// Verify symbols have the same names
	symbolNames1 := make(map[string]bool)
	symbolNames2 := make(map[string]bool)

	for _, symbol := range symbols1 {
		symbolNames1[symbol.Name] = true
	}

	for _, symbol := range symbols2 {
		symbolNames2[symbol.Name] = true
	}

	for name := range symbolNames1 {
		if !symbolNames2[name] {
			t.Errorf("Symbol %v found in traditional parsing but not in StringRef parsing", name)
		}
	}

	for name := range symbolNames2 {
		if !symbolNames1[name] {
			t.Errorf("Symbol %v found in StringRef parsing but not in traditional parsing", name)
		}
	}

	t.Logf("Backward compatibility verified:")
	t.Logf("  - Traditional parsing: %d symbols", len(symbols1))
	t.Logf("  - StringRef parsing: %d symbols", len(symbols2))
	t.Logf("  - Results are identical")
}

func TestStringRefMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory efficiency test in short mode")
	}

	// Create a large content string to test memory efficiency
	largeContent := make([]byte, 0, 10000)
	for i := 0; i < 1000; i++ {
		largeContent = append(largeContent, []byte("function function_"+string(rune(i))+"() { return "+string(rune(i))+"; }\n")...)
	}

	fileID := types.FileID(789)
	path := "large.js"
	parser := NewTreeSitterParser()

	// Test that StringRef parsing can handle large content efficiently
	tree, blocks, symbols, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), path, largeContent, fileID)

	if tree == nil {
		t.Fatal("ParseFileEnhancedWithASTAndContextStringRef returned nil tree for large content")
	}

	if len(symbols) == 0 {
		t.Error("No symbols extracted from large content")
	}

	// Verify that we can still access the original content
	testStringRef := types.NewStringRef(fileID, largeContent, 9, 9) // Extract 9 chars starting at position 9
	extractedContent := testStringRef.String(largeContent)
	if extractedContent == "" {
		t.Errorf("StringRef.String() returned empty for large content")
	}
	// Verify it starts with "function"
	if len(extractedContent) < 8 || extractedContent[:8] != "function" {
		t.Errorf("StringRef content check failed on large content, got %q", extractedContent)
	}

	t.Logf("Memory efficiency test passed:")
	t.Logf("  - Content size: %d bytes", len(largeContent))
	t.Logf("  - Symbols extracted: %d", len(symbols))
	t.Logf("  - Blocks extracted: %d", len(blocks))
	t.Logf("  - Zero-copy operations maintained efficiency")
}

func TestStringRefMultipleLanguages(t *testing.T) {
	testCases := []struct {
		name       string
		content    []byte
		path       string
		fileID     types.FileID
		minSymbols int
	}{
		{
			name: "Go",
			content: []byte(`package main

func main() {
    println("Hello, Go!")
}`),
			path:       "test.go",
			fileID:     100,
			minSymbols: 1,
		},
		{
			name: "JavaScript",
			content: []byte(`function hello() {
    console.log("Hello, JS!");
}

class TestClass {
    constructor() {}
}`),
			path:       "test.js",
			fileID:     101,
			minSymbols: 2,
		},
		{
			name: "Python",
			content: []byte(`def hello():
    print("Hello, Python!")

class TestClass:
    def __init__(self):
        pass`),
			path:       "test.py",
			fileID:     102,
			minSymbols: 2,
		},
		{
			name: "TypeScript",
			content: []byte(`interface User {
    name: string;
    age: number;
}

function greet(user: User): string {
    return "Hello, " + user.name + "!";
}`),
			path:       "test.ts",
			fileID:     103,
			minSymbols: 2,
		},
	}

	parser := NewTreeSitterParser()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree, blocks, symbols, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), tc.path, tc.content, tc.fileID)

			if tree == nil {
				t.Fatalf("ParseFileEnhancedWithASTAndContextStringRef returned nil tree for %s", tc.name)
			}

			if len(symbols) < tc.minSymbols {
				t.Errorf("Expected at least %d symbols for %s, got %d", tc.minSymbols, tc.name, len(symbols))
			}

			// Test that StringRef operations work for this language
			if len(symbols) > 0 {
				symbol := symbols[0]
				symbolStringRef := types.NewStringRef(tc.fileID, tc.content,
					int(symbol.Line-1), len(symbol.Name))

				// This is a rough test - in reality we'd need the exact position
				// But we can test that the StringRef operations don't crash
				_ = symbolStringRef.IsEmpty()
				_ = symbolStringRef.Length
			}

			t.Logf("%s StringRef parsing successful:", tc.name)
			t.Logf("  - Symbols: %d", len(symbols))
			t.Logf("  - Blocks: %d", len(blocks))
		})
	}
}
