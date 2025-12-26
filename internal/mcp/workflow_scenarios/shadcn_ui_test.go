package workflow_scenarios

import (
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestShadcnUI tests shadcn/ui library functionality
// Real-world integration tests for the shadcn/ui TypeScript component library
// NOTE: shadcn-ui is 88MB - large project
func TestShadcnUI(t *testing.T) {
	if !*enableLargeProjects {
		t.Skip("Skipping large project shadcn-ui (88MB) - use -large-projects flag to enable")
	}
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "typescript", "shadcn-ui")

	// === Type Discovery Tests ===
	t.Run("Button_Component_Definition", func(t *testing.T) {
		// Search for Button component
		searchResult := ctx.Search("Button", mcp.SearchOptions{
			Pattern:         "Button",
			SymbolTypes:     []string{"function", "type"},
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show Button props and usage
			contextResult := ctx.GetObjectContext("Button", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Button component")
		}
	})

	t.Run("Card_Component_Composition", func(t *testing.T) {
		// Search for Card components
		searchResult := ctx.Search("Card", mcp.SearchOptions{
			Pattern:     "Card",
			SymbolTypes: []string{"function", "type"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show card composition patterns
			contextResult := ctx.GetObjectContext("Card", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Card components")
		}
	})

	t.Run("Input_Component_TypeDef", func(t *testing.T) {
		// Search for Input component
		searchResult := ctx.Search("Input", mcp.SearchOptions{
			Pattern:         "Input",
			SymbolTypes:     []string{"type", "function"},
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show input type definitions
			contextResult := ctx.GetObjectContext("Input", 0, mcp.ContextOptions{
				IncludeFullSymbol:     true,
				IncludeAllReferences:  true,
				IncludeQualityMetrics: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Input component")
		}
	})

	t.Run("Dialog_Component_State", func(t *testing.T) {
		// Search for Dialog components
		searchResult := ctx.Search("Dialog", mcp.SearchOptions{
			Pattern:     "Dialog",
			SymbolTypes: []string{"function", "type"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show dialog state management
			contextResult := ctx.GetObjectContext("Dialog", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Dialog components")
		}
	})

	t.Run("Utility_Class_Pattern", func(t *testing.T) {
		// Search for utility classes or functions
		searchResult := ctx.Search("Utils", mcp.SearchOptions{
			Pattern:     "cn\\|clsx",
			SymbolTypes: []string{"function"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show utility patterns
			contextResult := ctx.GetObjectContext("Utils", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found utility patterns")
		}
	})
}
