package symbollinker

import (
	"os"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// TestDebugGoExtractor tests the debug go extractor.
func TestDebugGoExtractor(t *testing.T) {
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

	// Print all symbols for debugging
	t.Log("=== ALL SYMBOLS ===")
	for id, sym := range table.Symbols {
		t.Logf("  [%d] %s (%s) - Exported: %v", id, sym.Name, sym.Kind, sym.IsExported)
	}

	t.Log("\n=== IMPORTS ===")
	for _, imp := range table.Imports {
		t.Logf("  %s -> %s", imp.Alias, imp.ImportPath)
	}
}

// TestDebugTypeScriptClassAST prints the AST for a TypeScript export class to understand the structure
func TestDebugTypeScriptClassAST(t *testing.T) {
	content := []byte(`export class UserService {
    constructor(private baseUrl: string) {}

    async fetchUser(id: string): Promise<User> {
        return null;
    }
}`)

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(typescript.LanguageTypescript())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse TypeScript code")
	}
	defer tree.Close()

	// Print AST structure
	t.Log("=== AST STRUCTURE ===")
	printNode(t, tree.RootNode(), content, 0)

	// Also test extraction
	extractor := NewTSExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Log("\n=== EXTRACTED SYMBOLS ===")
	for id, sym := range table.Symbols {
		t.Logf("  [%d] %s (kind=%d) - Exported: %v", id, sym.Name, sym.Kind, sym.IsExported)
	}
}

func printNode(t *testing.T, node *sitter.Node, content []byte, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	nodeText := ""
	if node.ChildCount() == 0 && node.EndByte()-node.StartByte() < 50 {
		nodeText = " = " + string(content[node.StartByte():node.EndByte()])
	}

	t.Logf("%s%s%s", prefix, node.Kind(), nodeText)

	for i := uint(0); i < node.ChildCount(); i++ {
		printNode(t, node.Child(i), content, indent+1)
	}
}
