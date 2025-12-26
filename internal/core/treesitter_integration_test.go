package core

import (
	"testing"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// Helper function to create a test ASTStore with a minimal FileContentStore
func createTestASTStoreForIntegrationTest(t *testing.T) *ASTStore {
	contentStore := NewFileContentStore()
	t.Cleanup(func() { contentStore.Close() })
	return NewASTStore(contentStore)
}

// Helper function to store AST with content in both FileContentStore and ASTStore
func storeASTWithContentForIntegrationTest(store *ASTStore, fileID types.FileID, tree *tree_sitter.Tree, content []byte, path string, language string) types.FileID {
	// Load content into FileContentStore (generates its own FileID)
	actualFileID := store.fileContentStore.LoadFile(path, content)
	// Then store AST with the actual FileID used
	store.StoreAST(actualFileID, tree, path, language)
	return actualFileID
}

// Helper function to create a parser for a specific language
func createParser(t *testing.T, language string) *tree_sitter.Parser {
	parser := tree_sitter.NewParser()

	var languagePtr unsafe.Pointer
	switch language {
	case ".go":
		languagePtr = tree_sitter_go.Language()
	case ".js":
		languagePtr = tree_sitter_javascript.Language()
	case ".py":
		languagePtr = tree_sitter_python.Language()
	default:
		t.Fatalf("Unsupported language: %s", language)
	}

	lang := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(lang)
	if err != nil {
		t.Fatalf("Failed to set language %s: %v", language, err)
	}

	return parser
}

// TestTreeSitterIntegration_GoQueries tests the tree sitter integration go queries.
func TestTreeSitterIntegration_GoQueries(t *testing.T) {
	store := createTestASTStoreForIntegrationTest(t)
	parser := createParser(t, ".go")

	code := []byte(`package main

func add(a int, b int) int {
	return a + b
}

func multiply(x, y int) int {
	return x * y
}

type Person struct {
	Name string
	Age  int
}

func (p Person) Greet() string {
	return "Hello, " + p.Name
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	fileID := types.FileID(1)
	fileID = storeASTWithContentForIntegrationTest(store, fileID, tree, code, "test.go", ".go")

	// Test function name queries
	t.Run("FunctionNames", func(t *testing.T) {
		// Query for regular function declarations
		query := "(function_declaration name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".go")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find add and multiply functions (not Greet, which is a method)
		expectedFunctions := map[string]bool{"add": false, "multiply": false}
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 {
				funcName := match.Captures[0].Text
				if _, exists := expectedFunctions[funcName]; exists {
					expectedFunctions[funcName] = true
				}
			}
		}

		for funcName, found := range expectedFunctions {
			if !found {
				t.Errorf("Expected to find function '%s'", funcName)
			}
		}
	})

	// Test method queries (methods with receivers)
	t.Run("MethodNames", func(t *testing.T) {
		query := "(method_declaration name: (field_identifier) @method.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".go")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find Greet method
		foundGreet := false
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 && match.Captures[0].Text == "Greet" {
				foundGreet = true
				break
			}
		}

		if !foundGreet {
			t.Error("Expected to find 'Greet' method")
		}
	})

	// Test type queries
	t.Run("TypeDeclarations", func(t *testing.T) {
		query := "(type_declaration (type_spec name: (type_identifier) @type.name))"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".go")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 || len(results[0].Matches) != 1 {
			t.Errorf("Expected to find exactly 1 type declaration")
			return
		}

		typeName := results[0].Matches[0].Captures[0].Text
		if typeName != "Person" {
			t.Errorf("Expected type name 'Person', got '%s'", typeName)
		}
	})
}

// TestTreeSitterIntegration_JavaScriptQueries tests the tree sitter integration java script queries.
func TestTreeSitterIntegration_JavaScriptQueries(t *testing.T) {
	store := createTestASTStoreForIntegrationTest(t)
	parser := createParser(t, ".js")

	code := []byte(`function greet(name) {
    return "Hello, " + name;
}

const add = (a, b) => {
    return a + b;
};

class Calculator {
    multiply(x, y) {
        return x * y;
    }
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse JavaScript code")
	}
	defer tree.Close()

	fileID := storeASTWithContentForIntegrationTest(store, types.FileID(0), tree, code, "test.js", ".js")

	// Test function declarations
	t.Run("FunctionDeclarations", func(t *testing.T) {
		query := "(function_declaration name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".js")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find greet function
		found := false
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 && match.Captures[0].Text == "greet" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected to find 'greet' function")
		}
	})

	// Test class methods
	t.Run("ClassMethods", func(t *testing.T) {
		query := "(method_definition name: (property_identifier) @method.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".js")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find multiply method
		found := false
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 && match.Captures[0].Text == "multiply" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected to find 'multiply' method")
		}
	})
}

// TestTreeSitterIntegration_PythonQueries tests the tree sitter integration python queries.
func TestTreeSitterIntegration_PythonQueries(t *testing.T) {
	store := createTestASTStoreForIntegrationTest(t)
	parser := createParser(t, ".py")

	code := []byte(`def greet(name):
    return f"Hello, {name}"

def add(a, b):
    return a + b

class Calculator:
    def multiply(self, x, y):
        return x * y
    
    def subtract(self, x, y):
        return x - y`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Python code")
	}
	defer tree.Close()

	fileID := types.FileID(3)
	fileID = storeASTWithContentForIntegrationTest(store, fileID, tree, code, "test.py", ".py")

	// Test function definitions
	t.Run("FunctionDefinitions", func(t *testing.T) {
		query := "(function_definition name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".py")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find greet, add, multiply, and subtract functions
		expectedFunctions := map[string]bool{"greet": false, "add": false, "multiply": false, "subtract": false}
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 {
				funcName := match.Captures[0].Text
				if _, exists := expectedFunctions[funcName]; exists {
					expectedFunctions[funcName] = true
				}
			}
		}

		for funcName, found := range expectedFunctions {
			if !found {
				t.Errorf("Expected to find function '%s'", funcName)
			}
		}
	})
}

// TestTreeSitterIntegration_CrossLanguageQueries tests the tree sitter integration cross language queries.
func TestTreeSitterIntegration_CrossLanguageQueries(t *testing.T) {
	store := createTestASTStoreForIntegrationTest(t)

	// Add Go file
	goParser := createParser(t, ".go")
	goCode := []byte(`package main
func hello() string {
	return "world"
}`)
	goTree := goParser.Parse(goCode, nil)
	if goTree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer goTree.Close()
	_ = storeASTWithContentForIntegrationTest(store, types.FileID(0), goTree, goCode, "test.go", ".go")

	// Add JavaScript file
	jsParser := createParser(t, ".js")
	jsCode := []byte(`function hello() {
    return "world";
}`)
	jsTree := jsParser.Parse(jsCode, nil)
	if jsTree == nil {
		t.Fatal("Failed to parse JavaScript code")
	}
	defer jsTree.Close()
	_ = storeASTWithContentForIntegrationTest(store, types.FileID(0), jsTree, jsCode, "test.js", ".js")

	// Query all files - should find functions from both languages
	t.Run("AllFiles", func(t *testing.T) {
		// Note: This query uses Go syntax, so should only match Go files
		query := "(function_declaration name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{}, ".go") // Filter to Go only

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 Go file result, got %d", len(results))
			return
		}

		if results[0].Language != ".go" {
			t.Errorf("Expected Go file, got %s", results[0].Language)
		}
	})

	// Query JavaScript files only
	t.Run("JavaScriptOnly", func(t *testing.T) {
		query := "(function_declaration name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{}, ".js") // Filter to JS only

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 JavaScript file result, got %d", len(results))
			return
		}

		if results[0].Language != ".js" {
			t.Errorf("Expected JavaScript file, got %s", results[0].Language)
		}
	})
}

// TestTreeSitterIntegration_ComplexQueries tests the tree sitter integration complex queries.
func TestTreeSitterIntegration_ComplexQueries(t *testing.T) {
	store := createTestASTStoreForIntegrationTest(t)
	parser := createParser(t, ".go")

	code := []byte(`package main

import (
	"fmt"
	"strconv"
)

func processData(input string) (int, error) {
	value, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("invalid input: %w", err)
	}
	return value * 2, nil
}

func main() {
	result, err := processData("42")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Result: %d\n", result)
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	fileID := types.FileID(1)
	fileID = storeASTWithContentForIntegrationTest(store, fileID, tree, code, "test.go", ".go")

	// Test function calls
	t.Run("FunctionCalls", func(t *testing.T) {
		query := "(call_expression function: (identifier) @function.call)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".go")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find function calls like processData
		foundProcessData := false
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 && match.Captures[0].Text == "processData" {
				foundProcessData = true
				break
			}
		}

		if !foundProcessData {
			t.Error("Expected to find 'processData' function call")
		}
	})

	// Test import statements
	t.Run("ImportStatements", func(t *testing.T) {
		query := "(import_spec path: (interpreted_string_literal) @import.path)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".go")

		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 file result, got %d", len(results))
			return
		}

		// Should find fmt and strconv imports
		imports := make(map[string]bool)
		for _, match := range results[0].Matches {
			if len(match.Captures) > 0 {
				importPath := match.Captures[0].Text
				imports[importPath] = true
			}
		}

		expectedImports := []string{`"fmt"`, `"strconv"`}
		for _, expectedImport := range expectedImports {
			if !imports[expectedImport] {
				t.Errorf("Expected to find import %s", expectedImport)
			}
		}
	})
}

// TestTreeSitterIntegration_ErrorHandling tests the tree sitter integration error handling.
func TestTreeSitterIntegration_ErrorHandling(t *testing.T) {
	store := createTestASTStoreForIntegrationTest(t)

	code := []byte(`package main
func hello() string {
	return "world"
}`)

	tree := parseGoCode(t, code)
	defer tree.Close()

	fileID := types.FileID(1)
	fileID = storeASTWithContentForIntegrationTest(store, fileID, tree, code, "test.go", ".go")

	// Test invalid query
	t.Run("InvalidQuery", func(t *testing.T) {
		invalidQuery := "(invalid_syntax @capture"
		_, err := store.ExecuteQuery(invalidQuery, []types.FileID{fileID}, ".go")

		if err == nil {
			t.Error("Expected error for invalid query syntax")
		} else {
			t.Logf("Got expected error: %v", err)
		}
	})

	// Test unsupported language filter - should return empty results, not error
	t.Run("UnsupportedLanguageFilter", func(t *testing.T) {
		query := "(function_declaration name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".unsupported")

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for unsupported language filter, got %d", len(results))
		}
	})

	// Test query on non-existent file
	t.Run("NonExistentFile", func(t *testing.T) {
		query := "(function_declaration name: (identifier) @function.name)"
		results, err := store.ExecuteQuery(query, []types.FileID{types.FileID(999)}, ".go")

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for non-existent file, got %d", len(results))
		}
	})
}
