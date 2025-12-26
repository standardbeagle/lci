package core

import (
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// Helper function to create a test ASTStore with a minimal FileContentStore
func createTestASTStore(t *testing.T) *ASTStore {
	contentStore := NewFileContentStore()
	t.Cleanup(func() { contentStore.Close() })
	return NewASTStore(contentStore)
}

// Helper function to store AST with content in both FileContentStore and ASTStore
func storeASTWithContent(store *ASTStore, fileID types.FileID, tree *tree_sitter.Tree, content []byte, path string, language string) types.FileID {
	// Load content into FileContentStore (generates its own FileID)
	actualFileID := store.fileContentStore.LoadFile(path, content)
	// Then store AST with the actual FileID used
	store.StoreAST(actualFileID, tree, path, language)
	return actualFileID
}

// Helper function to create a parser and parse Go code
func parseGoCode(t *testing.T, code []byte) *tree_sitter.Tree {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_go.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	return tree
}

// TestASTStore_StoreAndGetAST tests the a s t store store and get a s t.
func TestASTStore_StoreAndGetAST(t *testing.T) {
	store := createTestASTStore(t)
	defer store.Clear() // Clean up at end

	// Create a simple Go AST
	code := []byte(`package main

func hello() string {
	return "world"
}`)

	tree := parseGoCode(t, code)
	// ASTStore takes ownership

	// Store the AST
	fileID := types.FileID(1)
	path := "test.go"
	language := ".go"

	fileID = storeASTWithContent(store, fileID, tree, code, path, language)

	// Retrieve the AST
	retrievedTree, retrievedContent, retrievedPath, retrievedLang, exists := store.GetAST(fileID)
	if !exists {
		t.Fatal("AST not found after storing")
	}

	if retrievedTree == nil {
		t.Fatal("Retrieved tree is nil")
	}

	if string(retrievedContent) != string(code) {
		t.Errorf("Content mismatch. Expected: %s, Got: %s", string(code), string(retrievedContent))
	}

	if retrievedPath != path {
		t.Errorf("Path mismatch. Expected: %s, Got: %s", path, retrievedPath)
	}

	if retrievedLang != language {
		t.Errorf("Language mismatch. Expected: %s, Got: %s", language, retrievedLang)
	}
}

// TestASTStore_RemoveFile tests the a s t store remove file.
func TestASTStore_RemoveFile(t *testing.T) {
	store := createTestASTStore(t)

	// Create and store an AST
	code := []byte(`package main

func hello() string {
	return "world"
}`)

	tree := parseGoCode(t, code)
	// Note: ASTStore takes ownership, so we don't defer close here

	fileID := types.FileID(1)
	fileID = storeASTWithContent(store, fileID, tree, code, "test.go", ".go")

	// Verify it exists
	_, _, _, _, exists := store.GetAST(fileID)
	if !exists {
		t.Fatal("AST should exist before removal")
	}

	// Remove the file (this will close the tree)
	store.RemoveFile(fileID)

	// Verify it's gone
	_, _, _, _, exists = store.GetAST(fileID)
	if exists {
		t.Fatal("AST should not exist after removal")
	}
}

// TestASTStore_GetAllFiles tests the a s t store get all files.
func TestASTStore_GetAllFiles(t *testing.T) {
	store := createTestASTStore(t)
	defer store.Clear() // Clean up at end

	// Create multiple ASTs
	code := []byte(`package main

func hello() string {
	return "world"
}`)

	tree1 := parseGoCode(t, code)
	tree2 := parseGoCode(t, code)
	// ASTStore takes ownership

	fileID1 := types.FileID(1)
	fileID2 := types.FileID(2)

	fileID1 = storeASTWithContent(store, fileID1, tree1, code, "test1.go", ".go")
	fileID2 = storeASTWithContent(store, fileID2, tree2, code, "test2.go", ".go")

	// Get all files
	files := store.GetAllFiles()

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	// Check that both file IDs are present
	foundFile1, foundFile2 := false, false
	for _, fileID := range files {
		if fileID == fileID1 {
			foundFile1 = true
		}
		if fileID == fileID2 {
			foundFile2 = true
		}
	}

	if !foundFile1 || !foundFile2 {
		t.Error("Not all expected file IDs found in GetAllFiles result")
	}
}

// TestASTStore_Clear tests the a s t store clear.
func TestASTStore_Clear(t *testing.T) {
	store := createTestASTStore(t)

	// Create multiple ASTs
	code := []byte(`package main

func hello() string {
	return "world"
}`)

	tree1 := parseGoCode(t, code)
	tree2 := parseGoCode(t, code)
	// ASTStore takes ownership

	fileID1 := types.FileID(1)
	fileID2 := types.FileID(2)

	fileID1 = storeASTWithContent(store, fileID1, tree1, code, "test1.go", ".go")
	fileID2 = storeASTWithContent(store, fileID2, tree2, code, "test2.go", ".go")

	// Verify they exist
	if store.GetFileCount() != 2 {
		t.Errorf("Expected 2 files before clear, got %d", store.GetFileCount())
	}

	// Clear all
	store.Clear()

	// Verify all are gone
	if store.GetFileCount() != 0 {
		t.Errorf("Expected 0 files after clear, got %d", store.GetFileCount())
	}

	files := store.GetAllFiles()
	if len(files) != 0 {
		t.Errorf("Expected empty file list after clear, got %d files", len(files))
	}
}

// TestASTStore_ExecuteQuery tests the a s t store execute query.
func TestASTStore_ExecuteQuery(t *testing.T) {
	store := createTestASTStore(t)
	defer store.Clear() // Clean up at end

	// Create a Go AST with a function
	code := []byte(`package main

func hello() string {
	return "world"
}

func goodbye() int {
	return 42
}`)

	tree := parseGoCode(t, code)
	// ASTStore takes ownership

	fileID := types.FileID(1)
	fileID = storeASTWithContent(store, fileID, tree, code, "test.go", ".go")

	// Query for all function declarations
	query := "(function_declaration name: (identifier) @function.name)"
	results, err := store.ExecuteQuery(query, []types.FileID{fileID}, ".go")

	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 file result, got %d", len(results))
		return
	}

	result := results[0]
	if result.FileID != fileID {
		t.Errorf("Wrong file ID in result. Expected: %v, Got: %v", fileID, result.FileID)
	}

	if result.Language != ".go" {
		t.Errorf("Wrong language in result. Expected: .go, Got: %s", result.Language)
	}

	// Should find two function names: hello and goodbye
	if len(result.Matches) != 2 {
		t.Errorf("Expected 2 matches (hello and goodbye), got %d", len(result.Matches))
		return
	}

	// Check the captured function names
	foundHello, foundGoodbye := false, false
	for _, match := range result.Matches {
		if len(match.Captures) > 0 {
			funcName := match.Captures[0].Text
			if funcName == "hello" {
				foundHello = true
			}
			if funcName == "goodbye" {
				foundGoodbye = true
			}
		}
	}

	if !foundHello {
		t.Error("Expected to find 'hello' function in query results")
	}
	if !foundGoodbye {
		t.Error("Expected to find 'goodbye' function in query results")
	}
}

// TestASTStore_GetMemoryStats tests the a s t store get memory stats.
func TestASTStore_GetMemoryStats(t *testing.T) {
	store := createTestASTStore(t)
	defer store.Clear() // Clean up at end

	// Create an AST
	code := []byte(`package main

func hello() string {
	return "world"
}`)

	tree := parseGoCode(t, code)
	// ASTStore takes ownership

	fileID := types.FileID(1)
	fileID = storeASTWithContent(store, fileID, tree, code, "test.go", ".go")

	// Get memory stats
	stats := store.GetMemoryStats()

	fileCount, ok := stats["file_count"].(int)
	if !ok || fileCount != 1 {
		t.Errorf("Expected file_count to be 1, got %v", stats["file_count"])
	}

	astTrees, ok := stats["ast_trees"].(int)
	if !ok || astTrees != 1 {
		t.Errorf("Expected ast_trees to be 1, got %v", stats["ast_trees"])
	}

	// Verify note field exists (content managed by FileContentStore)
	note, ok := stats["note"].(string)
	if !ok || !strings.Contains(note, "FileContentStore") {
		t.Errorf("Expected note about FileContentStore, got %v", stats["note"])
	}
}

// TestASTStore_GetSupportedLanguages tests the a s t store get supported languages.
func TestASTStore_GetSupportedLanguages(t *testing.T) {
	store := createTestASTStore(t)
	defer store.Clear() // Clean up at end

	// Initially empty
	languages := store.GetSupportedLanguages()
	if len(languages) != 0 {
		t.Errorf("Expected no languages initially, got %d", len(languages))
	}

	// Add a Go file
	code := []byte(`package main

func hello() string {
	return "world"
}`)

	tree := parseGoCode(t, code)
	// ASTStore takes ownership

	fileID := types.FileID(1)
	fileID = storeASTWithContent(store, fileID, tree, code, "test.go", ".go")

	// Should now have Go language
	languages = store.GetSupportedLanguages()
	if len(languages) != 1 {
		t.Errorf("Expected 1 language, got %d", len(languages))
	}

	if languages[0] != ".go" {
		t.Errorf("Expected .go language, got %s", languages[0])
	}
}
