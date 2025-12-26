package indexing

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// ASTNodeLookup provides helper functions for finding AST nodes for symbols
type ASTNodeLookup struct{}

// NewASTNodeLookup creates a new AST node lookup helper
func NewASTNodeLookup() *ASTNodeLookup {
	return &ASTNodeLookup{}
}

// FindSymbolASTNode finds the Tree-sitter AST node corresponding to a symbol
// This uses the same approach as the duplicate detector for consistency
func (lookup *ASTNodeLookup) FindSymbolASTNode(tree *tree_sitter.Tree, symbol types.Symbol) *tree_sitter.Node {
	if tree == nil {
		return nil
	}

	rootNode := tree.RootNode()
	return lookup.findNodeAtPosition(rootNode, symbol.Line-1, symbol.Column-1)
}

// findNodeAtPosition recursively searches for the AST node at the given line/column position
// This follows the same pattern as the existing duplicate detector
func (lookup *ASTNodeLookup) findNodeAtPosition(node *tree_sitter.Node, targetLine, targetColumn int) *tree_sitter.Node {
	if node == nil {
		return nil
	}

	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// Check if target position is within this node's range
	if int(startPoint.Row) > targetLine || int(endPoint.Row) < targetLine {
		return nil // Outside line range
	}

	// If target is at start line, check column bounds
	if int(startPoint.Row) == targetLine && int(startPoint.Column) > targetColumn {
		return nil // Before node start
	}

	// If target is at end line, check column bounds
	if int(endPoint.Row) == targetLine && int(endPoint.Column) < targetColumn {
		return nil // After node end
	}

	// Position is within this node's range
	// Check children for more specific match
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if childMatch := lookup.findNodeAtPosition(child, targetLine, targetColumn); childMatch != nil {
			return childMatch
		}
	}

	// No child matched, return this node as the best match
	return node
}

// FindEnhancedSymbolASTNode finds AST node for an enhanced symbol
func (lookup *ASTNodeLookup) FindEnhancedSymbolASTNode(tree *tree_sitter.Tree, symbol *types.EnhancedSymbol) *tree_sitter.Node {
	if tree == nil || symbol == nil {
		return nil
	}

	// Use the underlying Symbol data for position lookup
	return lookup.FindSymbolASTNode(tree, symbol.Symbol)
}
