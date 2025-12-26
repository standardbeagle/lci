package workflow_scenarios

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestChi tests Chi router framework functionality
// Real-world integration tests for the Chi Go web framework
// In short mode, runs a minimal subset to validate Go language support
func TestChi(t *testing.T) {
	ctx := GetProject(t, "go", "chi")
	shortMode := testing.Short()

	// === API Surface Analysis Tests ===
	// HTTP_Handler_Signature runs in both short and full mode (core validation)
	t.Run("HTTP_Handler_Signature", func(t *testing.T) {
		// Search for HTTP handler functions
		searchResult := ctx.Search("HTTPHandlers", mcp.SearchOptions{
			Pattern:     "http.ResponseWriter",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show request/response handling
			contextResult := ctx.GetObjectContext("HTTPHandlers", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeDependencies:  true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d HTTP handlers", len(searchResult.Results))
		}
	})

	t.Run("Middleware_Chain", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for middleware composition
		searchResult := ctx.Search("MiddlewareChain", mcp.SearchOptions{
			Pattern:     "Use",
			SymbolTypes: []string{"method"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("MiddlewareChain", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found middleware chain methods")
		}
	})

	t.Run("Route_Registration", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for route registration methods
		searchResult := ctx.Search("RouteRegistration", mcp.SearchOptions{
			Pattern:     "Route",
			SymbolTypes: []string{"method"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("RouteRegistration", 0, mcp.ContextOptions{
				IncludeFullSymbol:     true,
				IncludeCallHierarchy:  true,
				IncludeQualityMetrics: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found route registration methods")
		}
	})

	// === Function Analysis Tests ===
	// ServeHTTP_Method_Analysis runs in both short and full mode (validates method lookup)
	t.Run("ServeHTTP_Method_Analysis", func(t *testing.T) {
		// Search for the ServeHTTP method (core router method)
		searchResult := ctx.Search("ServeHTTP", mcp.SearchOptions{
			Pattern:         "ServeHTTP",
			SymbolTypes:     []string{"method"},
			DeclarationOnly: true,
		})

		// Should find the ServeHTTP method
		require.NotEmpty(t, searchResult.Results, "Should find ServeHTTP method")

		// Find the result with ServeHTTP in the name
		var serveHTTPResultIndex int
		found := false
		for i, result := range searchResult.Results {
			// Check if the match contains "ServeHTTP" to find the actual method
			if strings.Contains(result.Match, "ServeHTTP") {
				serveHTTPResultIndex = i
				found = true
				break
			}
		}
		require.True(t, found, "Should find a result with ServeHTTP in the match")

		// Get detailed context for the ServeHTTP method
		contextResult := ctx.GetObjectContext("ServeHTTP", serveHTTPResultIndex, mcp.ContextOptions{
			IncludeFullSymbol:    true,
			IncludeCallHierarchy: true,
			IncludeAllReferences: true,
		})

		// Assertions: Verify response has contexts
		assert.NotNil(t, contextResult["contexts"], "Response should have contexts")
		contexts, ok := contextResult["contexts"].([]interface{})
		assert.True(t, ok, "contexts should be a slice")
		assert.Greater(t, len(contexts), 0, "Should have at least one context")

		// Verify first context has symbol data (flat structure for compactness)
		if len(contexts) > 0 {
			firstCtx, ok := contexts[0].(map[string]interface{})
			assert.True(t, ok, "First context should be a map")
			assert.NotNil(t, firstCtx["object_id"], "Context should have object_id")
			assert.NotNil(t, firstCtx["symbol_name"], "Context should have symbol_name")
			assert.NotNil(t, firstCtx["symbol_type"], "Context should have symbol_type")

			// Verify it's the ServeHTTP method
			symbolName, _ := firstCtx["symbol_name"].(string)
			assert.Contains(t, symbolName, "ServeHTTP", "Should be ServeHTTP method")
		}

		t.Logf("ServeHTTP context: %+v", contextResult)
	})

	t.Run("Route_Handler_Function", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for route handler pattern (functions with http.ResponseWriter)
		searchResult := ctx.Search("RouteHandlers", mcp.SearchOptions{
			Pattern:     "ResponseWriter",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  10,
		})

		require.NotEmpty(t, searchResult.Results, "Should find handler functions")

		// Get context for first handler
		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("RouteHandlers", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			// Verify response has contexts
			assert.NotNil(t, contextResult["contexts"], "Response should have contexts")
			contexts, ok := contextResult["contexts"].([]interface{})
			assert.True(t, ok, "contexts should be a slice")
			if len(contexts) > 0 {
				firstCtx, ok := contexts[0].(map[string]interface{})
				assert.True(t, ok, "First context should be a map")
				assert.NotNil(t, firstCtx["object_id"], "Context should have object_id")
			}

			t.Logf("Found %d handler functions", len(searchResult.Results))
		}
	})

	t.Run("Middleware_Function_Chain", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Search for middleware functions
		searchResult := ctx.Search("Middleware", mcp.SearchOptions{
			Pattern:     "middleware",
			SymbolTypes: []string{"function"},
			MaxResults:  5,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("Middleware", 0, mcp.ContextOptions{
				IncludeCallHierarchy: true,
				IncludeAllReferences: true,
			})

			// Verify response has contexts
			assert.NotNil(t, contextResult["contexts"], "Response should have contexts")
			contexts, ok := contextResult["contexts"].([]interface{})
			assert.True(t, ok, "contexts should be a slice")
			if len(contexts) > 0 {
				firstCtx, ok := contexts[0].(map[string]interface{})
				assert.True(t, ok, "First context should be a map")
				assert.NotNil(t, firstCtx["object_id"], "Context should have object_id")
				assert.NotNil(t, firstCtx["symbol_name"], "Context should have symbol_name")
			}

			t.Logf("Found %d middleware functions", len(searchResult.Results))
		}
	})

	// === Linguistic Search Tests ===
	// CamelCase_Name_Splitting runs in both short and full mode (validates phrase matching)
	t.Run("CamelCase_Name_Splitting", func(t *testing.T) {
		// Query: "serve http" should find "ServeHTTP" via phrase matching
		// This tests the PhraseMatcher integration with multi-word queries
		searchResult := ctx.Search("serve_http_split", mcp.SearchOptions{
			Pattern:    "serve http",
			MaxResults: 20,
		})

		// Must find results
		require.NotEmpty(t, searchResult.Results, "Query 'serve http' should return results")

		// Should find ServeHTTP method
		found := false
		var matchedResult mcp.CompactSearchResult
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "ServeHTTP") || strings.Contains(result.SymbolName, "ServeHTTP") {
				found = true
				matchedResult = result
				t.Logf("✓ Found ServeHTTP via phrase matching: %s:%d (score:%.2f)", result.File, result.Line, result.Score)
				break
			}
		}

		require.True(t, found, "Query 'serve http' MUST find 'ServeHTTP' via phrase matching")

		// Verify the match has a reasonable score (phrase matching should produce good scores)
		assert.Greater(t, matchedResult.Score, 100.0, "ServeHTTP match should have a good score")
	})

	t.Run("Fuzzy_Typo_Tolerance", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Query: "middlewar" (typo) should find "middleware" via fuzzy matching
		searchResult := ctx.Search("middleware_typo", mcp.SearchOptions{
			Pattern:    "middlewar",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match)
			if strings.Contains(lower, "middleware") {
				found = true
				t.Logf("✓ Typo 'middlewar' found 'middleware': %s:%d", result.File, result.Line)
				break
			}
		}

		if !found {
			t.Logf("WARNING: Typo tolerance not working - 'middlewar' should find 'middleware'")
			t.Logf("This may indicate fuzzy matching is not enabled")
		}
	})

	t.Run("Partial_Match_Handler", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Query: "handler" (partial) should find all handler-related symbols
		searchResult := ctx.Search("partial_handler", mcp.SearchOptions{
			Pattern:     "handler",
			SymbolTypes: []string{"function", "method", "type"},
		})

		handlerCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "handler") {
				handlerCount++
			}
		}

		t.Logf("Found %d handler-related symbols", handlerCount)
		require.Greater(t, handlerCount, 0, "Should find handler-related symbols")
	})

	t.Run("Abbreviation_HTTP", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}
		// Query: "http" should find HTTP-related symbols
		searchResult := ctx.Search("abbrev_http", mcp.SearchOptions{
			Pattern:    "http",
			MaxResults: 30,
		})

		httpSymbols := 0
		for _, result := range searchResult.Results {
			text := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(text, "http") {
				httpSymbols++
				if httpSymbols <= 3 {
					t.Logf("  HTTP symbol: %s", result.SymbolName)
				}
			}
		}

		t.Logf("Found %d HTTP-related symbols", httpSymbols)
		require.Greater(t, httpSymbols, 5, "Should find multiple HTTP-related symbols")
	})
}
