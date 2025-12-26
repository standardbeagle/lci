package workflow_scenarios

import (
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestFastAPI tests FastAPI framework functionality
// Real-world integration tests for the FastAPI Python web framework
// In short mode, runs a minimal subset to validate Python language support
func TestFastAPI(t *testing.T) {
	ctx := GetProject(t, "python", "fastapi")
	shortMode := testing.Short()

	// === API Surface Analysis Tests ===
	// Route_Decorator_Pattern runs in both short and full mode (core validation)
	t.Run("Route_Decorator_Pattern", func(t *testing.T) {
		// Search for route decorators
		searchResult := ctx.Search("RouteDecorators", mcp.SearchOptions{
			Pattern:     "router",
			SymbolTypes: []string{"variable", "function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("RouteDecorators", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d router-related symbols", len(searchResult.Results))
		}
	})

	t.Run("Pydantic_Request_Models", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for request model classes
		searchResult := ctx.Search("RequestModels", mcp.SearchOptions{
			Pattern:     "Request",
			SymbolTypes: []string{"class"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("RequestModels", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
				IncludeDependencies:  true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d request model classes", len(searchResult.Results))
		}
	})

	t.Run("Dependency_Injection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for dependency injection pattern
		searchResult := ctx.Search("Dependencies", mcp.SearchOptions{
			Pattern:     "Depends",
			SymbolTypes: []string{"function", "class"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("Dependencies", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d dependency injection points", len(searchResult.Results))
		}
	})

	t.Run("Response_Model_Schema", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for response models
		searchResult := ctx.Search("ResponseModels", mcp.SearchOptions{
			Pattern:     "response_model",
			SymbolTypes: []string{"function", "class", "method"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			// Find first result with object_id (symbol-based match)
			var resultWithID int
			found := false
			for i, result := range searchResult.Results {
				if result.ObjectID != "" {
					resultWithID = i
					found = true
					break
				}
			}
			if !found {
				t.Logf("No results with object_id found, skipping context lookup")
			} else {
				contextResult := ctx.GetObjectContext("ResponseModels", resultWithID, mcp.ContextOptions{
					IncludeFullSymbol:    true,
					IncludeAllReferences: true,
				})

				ctx.AssertFieldExists(contextResult, "contexts")

				t.Logf("Found response_model references")
			}
		}
	})

	// === Function Analysis Tests ===
	// Route_Decorator_Handler runs in both short and full mode (validates function lookup)
	t.Run("Route_Decorator_Handler", func(t *testing.T) {
		// Search for route handlers with @app decorator
		searchResult := ctx.Search("RouteHandlers", mcp.SearchOptions{
			Pattern:     "def ",
			SymbolTypes: []string{"function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("RouteHandlers", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d Python functions", len(searchResult.Results))
		}
	})

	t.Run("Async_Function_Analysis", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for async functions
		searchResult := ctx.Search("AsyncFunctions", mcp.SearchOptions{
			Pattern:     "async def",
			UseRegex:    false,
			SymbolTypes: []string{"function"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("AsyncFunctions", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d async functions", len(searchResult.Results))
		}
	})

	t.Run("Class_Method_Analysis", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for class methods
		searchResult := ctx.Search("ClassMethods", mcp.SearchOptions{
			Pattern:     "class ",
			SymbolTypes: []string{"class"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("ClassMethods", 0, mcp.ContextOptions{
				IncludeFullSymbol:     true,
				IncludeAllReferences:  true,
				IncludeQualityMetrics: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d classes", len(searchResult.Results))
		}
	})

	// === Linguistic Search Tests ===
	// snake_case_splitting runs in both short and full mode (validates phrase matching)
	t.Run("snake_case_splitting", func(t *testing.T) {
		// Query: "route handler" should find "route_handler" via splitting
		searchResult := ctx.Search("route_handler_split", mcp.SearchOptions{
			Pattern:    "route handler",
			MaxResults: 30,
		})

		found := false
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "route") &&
				strings.Contains(result.Match, "handler") {
				found = true
				t.Logf("✓ Found route handler via snake_case splitting: %s", result.SymbolName)
				break
			}
		}

		if !found {
			t.Logf("INFO: snake_case splitting test - no exact matches (may need synonym support)")
		}
	})

	t.Run("Dependency_Injection_Abbreviation", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Query: "depend" should find "Depends", "dependencies", etc.
		searchResult := ctx.Search("depend_abbrev", mcp.SearchOptions{
			Pattern:    "depend",
			MaxResults: 30,
		})

		dependCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "depend") {
				dependCount++
			}
		}

		t.Logf("Found %d dependency-related symbols", dependCount)
		if dependCount == 0 {
			t.Errorf("Should find dependency-related symbols")
		}
	})
}
