package workflow_scenarios

import (
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestGoGitHub tests GitHub Go SDK functionality
// Real-world integration tests for the Google Go GitHub SDK
func TestGoGitHub(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "go", "go-github")

	// === Type Discovery Tests ===
	t.Run("Repository_Struct_Type", func(t *testing.T) {
		// Search for Repository type definition
		searchResult := ctx.Search("Repository", mcp.SearchOptions{
			Pattern:         "type Repository",
			SymbolTypes:     []string{"type"}, // "struct" is not a valid symbol type - Go structs use "type"
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show all methods and usages
			contextResult := ctx.GetObjectContext("Repository", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
				IncludeDependencies:  true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Repository type")
		}
	})

	t.Run("PullRequest_Method_Set", func(t *testing.T) {
		// Search for PullRequest type
		searchResult := ctx.Search("PullRequest", mcp.SearchOptions{
			Pattern:         "type PullRequest",
			SymbolTypes:     []string{"type"},
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show PR-related methods
			contextResult := ctx.GetObjectContext("PullRequest", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found PullRequest type")
		}
	})

	t.Run("Client_Struct_Configuration", func(t *testing.T) {
		// Search for Client type
		searchResult := ctx.Search("Client", mcp.SearchOptions{
			Pattern:         "type Client",
			SymbolTypes:     []string{"type"},
			DeclarationOnly: true,
			MaxResults:      5,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show client configuration options
			contextResult := ctx.GetObjectContext("Client", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Client type")
		}
	})

	t.Run("API_Method_Interface", func(t *testing.T) {
		// Search for GitHubService interface or similar
		searchResult := ctx.Search("GitHubService", mcp.SearchOptions{
			Pattern:         "type GitHub",
			SymbolTypes:     []string{"type", "interface"},
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show API methods
			contextResult := ctx.GetObjectContext("GitHubService", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found GitHub service types")
		}
	})

	t.Run("Error_Type_Handling", func(t *testing.T) {
		// Search for error types
		searchResult := ctx.Search("ErrorTypes", mcp.SearchOptions{
			Pattern:         "Error",
			SymbolTypes:     []string{"type"},
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show error handling patterns
			contextResult := ctx.GetObjectContext("ErrorTypes", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found error types")
		}
	})

	t.Run("Response_Struct_Parsing", func(t *testing.T) {
		// Search for Response types
		searchResult := ctx.Search("ResponseTypes", mcp.SearchOptions{
			Pattern:         "Response",
			SymbolTypes:     []string{"type"},
			DeclarationOnly: true,
			MaxResults:      15,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show response parsing logic
			contextResult := ctx.GetObjectContext("ResponseTypes", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found response types")
		}
	})
}
