package workflow_scenarios

import (
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestTRPC tests tRPC framework functionality
// Real-world integration tests for the tRPC TypeScript framework
// In short mode, runs a minimal subset to validate TypeScript language support
func TestTRPC(t *testing.T) {
	ctx := GetProject(t, "typescript", "trpc")
	shortMode := testing.Short()

	// === API Surface Analysis Tests ===
	// Procedure_Definition runs in both short and full mode (core validation)
	t.Run("Procedure_Definition", func(t *testing.T) {
		// Search for procedure definitions
		searchResult := ctx.Search("Procedures", mcp.SearchOptions{
			Pattern:     "procedure",
			SymbolTypes: []string{"variable", "function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show input/output schemas
			contextResult := ctx.GetObjectContext("Procedures", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeDependencies:  true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d procedures", len(searchResult.Results))
		}
	})

	t.Run("Router_Composition", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for router definitions
		searchResult := ctx.Search("Routers", mcp.SearchOptions{
			Pattern:     "router",
			SymbolTypes: []string{"variable", "constant"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("Routers", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d routers", len(searchResult.Results))
		}
	})

	t.Run("Input_Validation_Schemas", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for input schemas (Zod)
		searchResult := ctx.Search("InputSchemas", mcp.SearchOptions{
			Pattern:     "input",
			SymbolTypes: []string{"variable", "constant"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("InputSchemas", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found input schema definitions")
		}
	})
}
