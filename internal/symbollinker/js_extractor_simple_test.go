package symbollinker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/standardbeagle/lci/internal/types"
)

// TestJSExtractor_SimpleFunctions tests the j s extractor simple functions.
func TestJSExtractor_SimpleFunctions(t *testing.T) {
	code := `
export function add(a, b) {
  return a + b;
}

function subtract(x, y) {
  return x - y;
}

export const multiply = (a, b) => a * b;
`

	extractor := NewJSExtractor()
	parser := sitter.NewParser()
	_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_javascript.Language()))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	fileID := types.FileID(1)
	symbolTable, err := extractor.ExtractSymbols(fileID, []byte(code), tree)
	require.NoError(t, err)
	require.NotNil(t, symbolTable)

	// Check that symbols were extracted
	assert.Greater(t, len(symbolTable.Symbols), 0, "Should extract at least one symbol")

	// Check specific symbols exist
	symbolNames := make(map[string]bool)
	for _, symbol := range symbolTable.Symbols {
		symbolNames[symbol.Name] = true
	}

	assert.True(t, symbolNames["add"], "Should extract 'add' function")
	assert.True(t, symbolNames["subtract"], "Should extract 'subtract' function")
	assert.True(t, symbolNames["multiply"], "Should extract 'multiply' function")
}

// TestJSExtractor_SimpleClass tests the j s extractor simple class.
func TestJSExtractor_SimpleClass(t *testing.T) {
	code := `
export class User {
  constructor(name) {
    this.name = name;
  }
  
  getName() {
    return this.name;
  }
}
`

	extractor := NewJSExtractor()
	parser := sitter.NewParser()
	_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_javascript.Language()))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	fileID := types.FileID(1)
	symbolTable, err := extractor.ExtractSymbols(fileID, []byte(code), tree)
	require.NoError(t, err)
	require.NotNil(t, symbolTable)

	// Check that symbols were extracted
	symbolNames := make(map[string]bool)
	symbolKinds := make(map[string]types.SymbolKind)
	for _, symbol := range symbolTable.Symbols {
		symbolNames[symbol.Name] = true
		symbolKinds[symbol.Name] = symbol.Kind
	}

	assert.True(t, symbolNames["User"], "Should extract 'User' class")
	assert.Equal(t, types.SymbolKindClass, symbolKinds["User"], "'User' should be a class")
}

// TestTSExtractor_SimpleInterface tests the t s extractor simple interface.
func TestTSExtractor_SimpleInterface(t *testing.T) {
	code := `
export interface User {
  name: string;
  age: number;
}

export type Status = 'active' | 'inactive';
`

	extractor := NewTSExtractor()
	parser := sitter.NewParser()
	_ = parser.SetLanguage(sitter.NewLanguage(typescript.LanguageTypescript()))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	fileID := types.FileID(1)
	symbolTable, err := extractor.ExtractSymbols(fileID, []byte(code), tree)
	require.NoError(t, err)
	require.NotNil(t, symbolTable)

	// Check that symbols were extracted
	symbolNames := make(map[string]bool)
	symbolKinds := make(map[string]types.SymbolKind)
	for _, symbol := range symbolTable.Symbols {
		symbolNames[symbol.Name] = true
		symbolKinds[symbol.Name] = symbol.Kind
	}

	assert.True(t, symbolNames["User"], "Should extract 'User' interface")
	assert.Equal(t, types.SymbolKindInterface, symbolKinds["User"], "'User' should be an interface")

	assert.True(t, symbolNames["Status"], "Should extract 'Status' type")
	assert.Equal(t, types.SymbolKindType, symbolKinds["Status"], "'Status' should be a type")
}

// TestJSExtractor_CanHandle tests the j s extractor can handle.
func TestJSExtractor_CanHandle(t *testing.T) {
	extractor := NewJSExtractor()

	assert.True(t, extractor.CanHandle("app.js"))
	assert.True(t, extractor.CanHandle("component.jsx"))
	assert.True(t, extractor.CanHandle("module.mjs"))
	assert.False(t, extractor.CanHandle("app.ts"))
	assert.False(t, extractor.CanHandle("types.tsx"))
	assert.False(t, extractor.CanHandle("main.go"))
}

// TestTSExtractor_CanHandle tests the t s extractor can handle.
func TestTSExtractor_CanHandle(t *testing.T) {
	extractor := NewTSExtractor()

	assert.True(t, extractor.CanHandle("app.ts"))
	assert.True(t, extractor.CanHandle("component.tsx"))
	assert.True(t, extractor.CanHandle("types.d.ts"))
	assert.False(t, extractor.CanHandle("app.js"))
	assert.False(t, extractor.CanHandle("component.jsx"))
	assert.False(t, extractor.CanHandle("main.go"))
}

// TestJSExtractor_Language tests the j s extractor language.
func TestJSExtractor_Language(t *testing.T) {
	jsExtractor := NewJSExtractor()
	assert.Equal(t, "javascript", jsExtractor.GetLanguage())

	tsExtractor := NewTSExtractor()
	assert.Equal(t, "typescript", tsExtractor.GetLanguage())
}
