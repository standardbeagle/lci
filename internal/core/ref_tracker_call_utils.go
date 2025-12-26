package core

import "github.com/standardbeagle/lci/internal/types"

// FunctionTreeNode represents a node in the function call tree
// This replaces the equivalent structure from CallGraph
type FunctionTreeNode struct {
	Name     string                 `json:"name"`
	Children []*FunctionTreeNode    `json:"children,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FilterCallReferences filters references to only include function calls
func FilterCallReferences(refs []types.Reference) []types.Reference {
	var callRefs []types.Reference
	for _, ref := range refs {
		if ref.Type == types.RefTypeCall {
			callRefs = append(callRefs, ref)
		}
	}
	return callRefs
}

// GetCalleeNames gets the names of functions called by a symbol
func (rt *ReferenceTracker) GetCalleeNames(symbolID types.SymbolID) []string {
	outgoingRefs := rt.GetSymbolReferences(symbolID, "outgoing")
	var callees []string
	seen := make(map[string]bool)

	for _, ref := range outgoingRefs {
		if ref.Type == types.RefTypeCall && ref.ReferencedName != "" {
			if !seen[ref.ReferencedName] {
				callees = append(callees, ref.ReferencedName)
				seen[ref.ReferencedName] = true
			}
		}
	}
	return callees
}

// GetCallerNames gets the names of functions that call this symbol
func (rt *ReferenceTracker) GetCallerNames(symbolID types.SymbolID) []string {
	incomingRefs := rt.GetSymbolReferences(symbolID, "incoming")
	var callers []string
	seen := make(map[types.SymbolID]bool)

	for _, ref := range incomingRefs {
		if ref.Type == types.RefTypeCall && ref.SourceSymbol != 0 {
			if !seen[ref.SourceSymbol] {
				if sourceSymbol := rt.GetEnhancedSymbol(ref.SourceSymbol); sourceSymbol != nil {
					callers = append(callers, sourceSymbol.Name)
					seen[ref.SourceSymbol] = true
				}
			}
		}
	}
	return callers
}

// GetCalleeSymbols gets the symbol IDs of functions called by this symbol
func (rt *ReferenceTracker) GetCalleeSymbols(symbolID types.SymbolID) []types.SymbolID {
	outgoingRefs := rt.GetSymbolReferences(symbolID, "outgoing")
	var callees []types.SymbolID
	seen := make(map[types.SymbolID]bool)

	for _, ref := range outgoingRefs {
		if ref.Type == types.RefTypeCall && ref.TargetSymbol != 0 {
			if !seen[ref.TargetSymbol] {
				callees = append(callees, ref.TargetSymbol)
				seen[ref.TargetSymbol] = true
			}
		}
	}
	return callees
}

// GetCallerSymbols gets the symbol IDs of functions that call this symbol
func (rt *ReferenceTracker) GetCallerSymbols(symbolID types.SymbolID) []types.SymbolID {
	incomingRefs := rt.GetSymbolReferences(symbolID, "incoming")
	var callers []types.SymbolID
	seen := make(map[types.SymbolID]bool)

	for _, ref := range incomingRefs {
		if ref.Type == types.RefTypeCall && ref.SourceSymbol != 0 {
			if !seen[ref.SourceSymbol] {
				callers = append(callers, ref.SourceSymbol)
				seen[ref.SourceSymbol] = true
			}
		}
	}
	return callers
}

// BuildFunctionTree builds a tree of function calls from RefTracker
func (rt *ReferenceTracker) BuildFunctionTree(symbolID types.SymbolID, maxDepth int) *FunctionTreeNode {
	visited := make(map[types.SymbolID]bool)
	return rt.buildTreeNode(symbolID, 0, maxDepth, visited)
}

// BuildFunctionTreeByName builds a tree of function calls from a function name
func (rt *ReferenceTracker) BuildFunctionTreeByName(functionName string, maxDepth int) *FunctionTreeNode {
	// Find the symbol by name
	symbols := rt.FindSymbolsByName(functionName)
	if len(symbols) == 0 {
		return nil
	}

	// Use the first match (could be improved to handle multiple matches)
	return rt.BuildFunctionTree(symbols[0].ID, maxDepth)
}

func (rt *ReferenceTracker) buildTreeNode(symbolID types.SymbolID, depth, maxDepth int, visited map[types.SymbolID]bool) *FunctionTreeNode {
	if depth > maxDepth || visited[symbolID] {
		return nil
	}
	visited[symbolID] = true
	// Don't use defer delete - visited map must persist across recursive calls
	// to prevent cycles in the call graph

	symbol := rt.GetEnhancedSymbol(symbolID)
	if symbol == nil {
		return nil
	}

	node := &FunctionTreeNode{
		Name:     symbol.Name,
		Children: []*FunctionTreeNode{},
		Metadata: map[string]interface{}{
			"line":      symbol.Line,
			"column":    symbol.Column,
			"file_id":   symbol.FileID,
			"symbol_id": symbolID,
		},
	}

	// Get callees
	outgoingRefs := rt.GetSymbolReferences(symbolID, "outgoing")
	for _, ref := range outgoingRefs {
		if ref.Type == types.RefTypeCall && ref.TargetSymbol != 0 {
			if child := rt.buildTreeNode(ref.TargetSymbol, depth+1, maxDepth, visited); child != nil {
				node.Children = append(node.Children, child)
			}
		}
	}

	return node
}

// GetCallStats returns statistics about function calls (replaces CallGraph.GetStats)
func (rt *ReferenceTracker) GetCallStats() map[string]interface{} {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	totalNodes := 0
	totalEdges := 0
	nodesWithCalls := 0

	// Count all function/method symbols
	for _, symbolID := range rt.symbols.GetIDs() {
		symbol := rt.symbols.Get(symbolID)
		if symbol != nil && (symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeMethod) {
			totalNodes++

			// Count outgoing call edges
			if outgoing, exists := rt.outgoingRefs[symbolID]; exists {
				hasCallEdge := false
				for _, refID := range outgoing {
					if ref, exists := rt.references[refID]; exists && ref.Type == types.RefTypeCall {
						totalEdges++
						hasCallEdge = true
					}
				}
				if hasCallEdge {
					nodesWithCalls++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_nodes":        totalNodes,
		"total_edges":        totalEdges,
		"nodes_with_calls":   nodesWithCalls,
		"orphaned_nodes":     totalNodes - nodesWithCalls,
		"average_out_degree": float64(totalEdges) / float64(totalNodes),
	}
}
