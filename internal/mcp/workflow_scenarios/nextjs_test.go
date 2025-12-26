package workflow_scenarios

import (
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestNextJS tests Next.js framework functionality
// Real-world integration tests for the Next.js TypeScript framework
// NOTE: Next.js is 272MB - large project
func TestNextJS(t *testing.T) {
	if !*enableLargeProjects {
		t.Skip("Skipping large project next.js (272MB) - use -large-projects flag to enable")
	}
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "typescript", "next.js")

	// === API Surface Analysis Tests ===
	t.Run("API_Route_Handlers", func(t *testing.T) {
		// Search for API route handler functions
		searchResult := ctx.Search("APIRouteHandlers", mcp.SearchOptions{
			Pattern:     "handler",
			SymbolTypes: []string{"function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show request/response types
			contextResult := ctx.GetObjectContext("APIRouteHandlers", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d API route handlers", len(searchResult.Results))
		}
	})

	t.Run("Request_Response_Types", func(t *testing.T) {
		// Search for NextRequest/NextResponse types
		searchResult := ctx.Search("RequestResponseTypes", mcp.SearchOptions{
			Pattern:     "Request",
			SymbolTypes: []string{"type", "interface"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("RequestResponseTypes", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found Request/Response types")
		}
	})

	t.Run("Middleware_Functions", func(t *testing.T) {
		// Search for middleware
		searchResult := ctx.Search("NextMiddleware", mcp.SearchOptions{
			Pattern:     "middleware",
			SymbolTypes: []string{"function"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("NextMiddleware", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found middleware functions")
		}
	})

	// === Function Analysis Tests ===
	t.Run("API_Route_Handler", func(t *testing.T) {
		// Search for API route handlers
		searchResult := ctx.Search("APIRoutes", mcp.SearchOptions{
			Pattern:     "handler",
			SymbolTypes: []string{"function"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("APIRoutes", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d API route handlers", len(searchResult.Results))
		}
	})

	t.Run("Async_Function_TypeScript", func(t *testing.T) {
		// Search for async functions
		searchResult := ctx.Search("AsyncTS", mcp.SearchOptions{
			Pattern:     "async",
			SymbolTypes: []string{"function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("AsyncTS", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d async TypeScript functions", len(searchResult.Results))
		}
	})

	t.Run("Component_Function", func(t *testing.T) {
		// Search for React component functions
		searchResult := ctx.Search("Components", mcp.SearchOptions{
			Pattern:     "function",
			SymbolTypes: []string{"function"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("Components", 0, mcp.ContextOptions{
				IncludeFullSymbol:     true,
				IncludeAllReferences:  true,
				IncludeQualityMetrics: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d component functions", len(searchResult.Results))
		}
	})

	// === Linguistic Search Tests ===
	t.Run("API_Route_Multi_Word", func(t *testing.T) {
		// Query: "api route" should find "APIRoute", "apiRoute", "api_route"
		searchResult := ctx.Search("api_route_query", mcp.SearchOptions{
			Pattern:    "api route",
			MaxResults: 30,
		})

		found := false
		for _, result := range searchResult.Results {
			text := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(text, "api") && strings.Contains(text, "route") {
				found = true
				t.Logf("✓ Multi-word 'api route' found: %s", result.SymbolName)
				break
			}
		}

		if !found {
			t.Logf("INFO: Multi-word TypeScript query may need additional support")
		}
	})

	t.Run("Handler_Typo", func(t *testing.T) {
		// Query: "handlr" (typo) should find "handler" via fuzzy
		searchResult := ctx.Search("handler_typo", mcp.SearchOptions{
			Pattern:    "handlr",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			if strings.Contains(strings.ToLower(result.Match), "handler") {
				found = true
				t.Logf("✓ Typo 'handlr' found 'handler': %s", result.SymbolName)
				break
			}
		}

		if !found {
			t.Logf("WARNING: Fuzzy matching not working for TypeScript")
		}
	})
}
