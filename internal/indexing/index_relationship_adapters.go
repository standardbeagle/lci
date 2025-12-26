package indexing

// Adapters bridging MasterIndex components to analysis interfaces.

import (
	analysis "github.com/standardbeagle/lci/internal/analysis"
	core "github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// symbolLinkerAdapter wraps ReferenceTracker to implement SymbolLinkerInterface
// for the RelationshipAnalyzer without exposing internal core types directly.
type symbolLinkerAdapter struct {
	refTracker *core.ReferenceTracker
}

func (sla *symbolLinkerAdapter) ResolveReferences(symbolID types.CompositeSymbolID) ([]analysis.ResolvedReference, error) {
	// Convert CompositeSymbolID back to regular SymbolID for reference tracker
	// For now, use the LocalSymbolID part (this is a simplification)
	localSymbolID := types.SymbolID(symbolID.LocalSymbolID)

	refs := sla.refTracker.GetSymbolReferences(localSymbolID, "all")
	resolvedRefs := make([]analysis.ResolvedReference, len(refs))

	for i, ref := range refs {
		resolvedRefs[i] = analysis.ResolvedReference{
			Target: types.CompositeSymbolID{FileID: ref.FileID, LocalSymbolID: uint32(ref.TargetSymbol)},
			SourceLocation: types.SymbolLocation{
				FileID: ref.FileID,
				Line:   ref.Line,
				Column: ref.Column,
			},
			ImportPath: "",                   // Not available in current reference tracker
			Confidence: 1.0,                  // Default confidence
			RefType:    analysis.RefTypeCall, // Default type - should be mapped from ref.Type
		}
	}

	return resolvedRefs, nil
}
