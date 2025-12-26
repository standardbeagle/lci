package indexing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/types"
)

// TestNewASTNodeLookup tests creating a new AST node lookup
func TestNewASTNodeLookup(t *testing.T) {
	lookup := NewASTNodeLookup()
	require.NotNil(t, lookup)
	assert.IsType(t, &ASTNodeLookup{}, lookup)
}

// TestASTNodeLookup_FindSymbolASTNode_NilTree tests with nil tree
func TestASTNodeLookup_FindSymbolASTNode_NilTree(t *testing.T) {
	lookup := NewASTNodeLookup()

	// Test with nil tree
	result := lookup.FindSymbolASTNode(nil, types.Symbol{})
	assert.Nil(t, result, "Should return nil for nil tree")
}

// TestASTNodeLookup_FindEnhancedSymbolASTNode_NilInputs tests with nil inputs
func TestASTNodeLookup_FindEnhancedSymbolASTNode_NilInputs(t *testing.T) {
	lookup := NewASTNodeLookup()

	// Test with nil tree and nil enhanced symbol
	result := lookup.FindEnhancedSymbolASTNode(nil, nil)
	assert.Nil(t, result, "Should return nil for nil inputs")

	// Test with nil enhanced symbol
	result = lookup.FindEnhancedSymbolASTNode(nil, nil)
	assert.Nil(t, result, "Should return nil for nil enhanced symbol")
}

// TestASTNodeLookup_FindEnhancedSymbolASTNode_NilTree tests enhanced symbol with nil tree
func TestASTNodeLookup_FindEnhancedSymbolASTNode_NilTree(t *testing.T) {
	lookup := NewASTNodeLookup()

	symbol := types.Symbol{
		Line:   1,
		Column: 1,
	}
	enhancedSymbol := &types.EnhancedSymbol{
		Symbol: symbol,
	}

	result := lookup.FindEnhancedSymbolASTNode(nil, enhancedSymbol)
	assert.Nil(t, result, "Should return nil for nil tree")
}

// TestASTNodeLookup_findNodeAtPosition_EdgeCases tests edge cases for node finding
func TestASTNodeLookup_findNodeAtPosition_EdgeCases(t *testing.T) {
	lookup := NewASTNodeLookup()

	// Test with nil node
	result := lookup.findNodeAtPosition(nil, 1, 1)
	assert.Nil(t, result, "Should return nil for nil node")
}

// BenchmarkASTNodeLookup_FindSymbolASTNode benchmarks symbol AST node finding
func BenchmarkASTNodeLookup_FindSymbolASTNode(b *testing.B) {
	lookup := NewASTNodeLookup()
	symbol := types.Symbol{
		Line:   1,
		Column: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lookup.FindSymbolASTNode(nil, symbol)
	}
}

// BenchmarkASTNodeLookup_findNodeAtPosition benchmarks internal node finding
func BenchmarkASTNodeLookup_findNodeAtPosition(b *testing.B) {
	lookup := NewASTNodeLookup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lookup.findNodeAtPosition(nil, 1, 1)
	}
}
