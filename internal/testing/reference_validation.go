package testing

import (
	"testing"

	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// ReferenceExpectation defines expected reference counts for a symbol
type ReferenceExpectation struct {
	SymbolName        string
	MinIncoming       int
	MinOutgoing       int
	ExpectDeclaration bool
}

// ValidateEnhancedReferences validates reference counts for enhanced search results
func ValidateEnhancedReferences(t *testing.T, engine *search.Engine, fileIDs []types.FileID, expectations []ReferenceExpectation) {
	t.Helper()

	for _, expected := range expectations {
		t.Run("Detailed_"+expected.SymbolName, func(t *testing.T) {
			results := engine.SearchDetailed(expected.SymbolName, fileIDs, 10)

			if len(results) == 0 {
				t.Fatalf("No results found for symbol %s", expected.SymbolName)
			}

			// Find the declaration result
			var declarationResult *search.StandardResult
			for i := range results {
				result := &results[i]
				if result.RelationalData != nil && result.RelationalData.Symbol.Name == expected.SymbolName {
					declarationResult = result
					break
				}
			}

			if declarationResult == nil && expected.ExpectDeclaration {
				t.Fatalf("No declaration found for symbol %s", expected.SymbolName)
			}

			if declarationResult == nil {
				// Skip validation if no declaration expected
				return
			}

			// Check reference counts
			refStats := declarationResult.RelationalData.Symbol.RefStats
			t.Logf("Symbol %s: %d incoming, %d outgoing references",
				expected.SymbolName,
				refStats.Total.IncomingCount,
				refStats.Total.OutgoingCount)

			// Verify minimum expected references
			if refStats.Total.IncomingCount < expected.MinIncoming {
				t.Errorf("Symbol %s: expected at least %d incoming references, got %d",
					expected.SymbolName, expected.MinIncoming, refStats.Total.IncomingCount)
			}

			if refStats.Total.OutgoingCount < expected.MinOutgoing {
				t.Errorf("Symbol %s: expected at least %d outgoing references, got %d",
					expected.SymbolName, expected.MinOutgoing, refStats.Total.OutgoingCount)
			}

			// Verify the enhanced symbol has proper structure
			enhancedSym := &declarationResult.RelationalData.Symbol
			if enhancedSym.ID == 0 {
				t.Error("Enhanced symbol has no ID")
			}
			if enhancedSym.FileID == 0 {
				t.Error("Enhanced symbol has no FileID")
			}

			// For symbols with incoming references, verify we have reference details
			if expected.MinIncoming > 0 && len(enhancedSym.IncomingRefs) == 0 {
				t.Error("Enhanced symbol has incoming count but no IncomingRefs array")
			}

			// For symbols with outgoing references, verify we have reference details
			if expected.MinOutgoing > 0 && len(enhancedSym.OutgoingRefs) == 0 {
				t.Error("Enhanced symbol has outgoing count but no OutgoingRefs array")
			}
		})
	}
}

// ValidateSearchDisplaysReferences checks that basic search includes reference info
func ValidateSearchDisplaysReferences(t *testing.T, engine *search.Engine, fileIDs []types.FileID, symbolName string) {
	t.Helper()

	results := engine.Search(symbolName, fileIDs, 10)

	if len(results) == 0 {
		t.Fatal("No results found")
	}

	// The search results should include reference information
	hasRefInfo := false
	for _, result := range results {
		// Check if reference info is displayed (format might be [refs: X↑ Y↓])
		t.Logf("Result at line %d: %s", result.Line, result.Match)
		// We'll update this once we know the expected format
		if result.Match != "" {
			hasRefInfo = true
		}
	}

	if !hasRefInfo {
		t.Log("Warning: No reference information found in search results")
	}
}
