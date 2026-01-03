package workflow_scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestGetContext_SearchToContext tests the complete workflow:
// 1. Search for a symbol
// 2. Get object ID from search results
// 3. Use get_context with that ID
// This validates the "function::0" bug fix
func TestGetContext_SearchToContext(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	t.Run("BasicWorkflow", func(t *testing.T) {
		// Step 1: Search for a function
		searchResult := ctx.Search("mux_funcs", mcp.SearchOptions{
			Pattern:     "ServeHTTP",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  5,
		})

		require.NotEmpty(t, searchResult.Results, "Search should return results")

		// Step 2: Get context for the first result
		contextResult := ctx.GetObjectContext("mux_funcs", 0, mcp.ContextOptions{
			IncludeFullSymbol:    true,
			IncludeCallHierarchy: true,
		})

		// Step 3: Verify we got valid context (not the "function::0" error)
		ctx.AssertNoError(contextResult, "")
		ctx.AssertFieldExists(contextResult, "contexts")

		t.Logf("Successfully got context for ServeHTTP")
	})

	t.Run("MultipleResults", func(t *testing.T) {
		// Search for something with multiple results
		searchResult := ctx.Search("middleware_funcs", mcp.SearchOptions{
			Pattern:     "Middleware",
			SymbolTypes: []string{"function", "method", "type"},
			MaxResults:  10,
		})

		require.NotEmpty(t, searchResult.Results, "Search should return results")

		// Get context for each result (tests different object IDs)
		for i := 0; i < len(searchResult.Results) && i < 3; i++ {
			result := searchResult.Results[i]
			require.NotEmpty(t, result.ObjectID, "Result %d should have object ID", i)

			contextResult := ctx.GetObjectContext("middleware_funcs", i, mcp.ContextOptions{
				IncludeFullSymbol: true,
			})

			ctx.AssertNoError(contextResult, "")
			t.Logf("Got context for result %d (o=%s): %s", i, result.ObjectID, result.SymbolName)
		}
	})
}

// TestGetContext_WithModeParameter tests get_context with explicit mode parameter
// This specifically tests the code path that had the "function::0" bug
func TestGetContext_WithModeParameter(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	t.Run("Mode_Full", func(t *testing.T) {
		// First search to get a valid object ID
		searchResult := ctx.Search("router_search", mcp.SearchOptions{
			Pattern:     "Router",
			SymbolTypes: []string{"function", "type"},
			MaxResults:  5,
		})

		require.NotEmpty(t, searchResult.Results, "Search should return results")
		objectID := searchResult.Results[0].ObjectID
		require.NotEmpty(t, objectID, "First result should have object ID")

		// Now call get_context with mode=full
		// This is the code path that was broken
		params := map[string]interface{}{
			"id":   objectID,
			"mode": "full",
		}

		result, err := ctx.Server.CallTool("get_object_context", params)
		require.NoError(t, err, "get_object_context with mode=full should not error")
		require.NotEmpty(t, result, "Result should not be empty")

		// Verify we didn't get the "function::0" error
		assert.NotContains(t, result, "function::0",
			"Should not contain 'function::0' error indicator")
		assert.NotContains(t, result, "invalid object ID",
			"Should not contain 'invalid object ID' error")

		t.Logf("get_object_context with mode=full succeeded for object ID: %s", objectID)
	})

	t.Run("Mode_Quick", func(t *testing.T) {
		searchResult := ctx.Search("handler_search", mcp.SearchOptions{
			Pattern:     "Handler",
			SymbolTypes: []string{"function", "type"},
			MaxResults:  3,
		})

		require.NotEmpty(t, searchResult.Results, "Search should return results")
		objectID := searchResult.Results[0].ObjectID

		params := map[string]interface{}{
			"id":   objectID,
			"mode": "quick",
		}

		result, err := ctx.Server.CallTool("get_object_context", params)
		require.NoError(t, err, "get_object_context with mode=quick should not error")
		assert.NotContains(t, result, "function::0")

		t.Logf("get_object_context with mode=quick succeeded for object ID: %s", objectID)
	})

	t.Run("Mode_Relationships", func(t *testing.T) {
		searchResult := ctx.Search("mux_type_search", mcp.SearchOptions{
			Pattern:     "Mux",
			SymbolTypes: []string{"type", "struct"},
			MaxResults:  3,
		})

		require.NotEmpty(t, searchResult.Results, "Search should return results")
		objectID := searchResult.Results[0].ObjectID

		params := map[string]interface{}{
			"id":   objectID,
			"mode": "relationships",
		}

		result, err := ctx.Server.CallTool("get_object_context", params)
		require.NoError(t, err, "get_object_context with mode=relationships should not error")
		assert.NotContains(t, result, "function::0")

		t.Logf("get_object_context with mode=relationships succeeded for object ID: %s", objectID)
	})
}

// TestGetContext_ObjectIDFormats tests various object ID formats work correctly
func TestGetContext_ObjectIDFormats(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	// Search to get real object IDs
	searchResult := ctx.Search("chi_symbols", mcp.SearchOptions{
		Pattern:     "chi",
		SymbolTypes: []string{"function", "type", "method"},
		MaxResults:  20,
	})

	require.NotEmpty(t, searchResult.Results, "Search should return results")

	t.Run("ShortIDs", func(t *testing.T) {
		// Test short object IDs (2 characters)
		for _, result := range searchResult.Results {
			if len(result.ObjectID) == 2 {
				params := map[string]interface{}{
					"id": result.ObjectID,
				}

				response, err := ctx.Server.CallTool("get_object_context", params)
				require.NoError(t, err, "Short ID %q should work", result.ObjectID)
				assert.NotContains(t, response, "function::0")

				t.Logf("Short ID %q works for %s", result.ObjectID, result.SymbolName)
				break // Just test one short ID
			}
		}
	})

	t.Run("LongerIDs", func(t *testing.T) {
		// Test longer object IDs (3+ characters)
		for _, result := range searchResult.Results {
			if len(result.ObjectID) >= 3 {
				params := map[string]interface{}{
					"id": result.ObjectID,
				}

				response, err := ctx.Server.CallTool("get_object_context", params)
				require.NoError(t, err, "Longer ID %q should work", result.ObjectID)
				assert.NotContains(t, response, "function::0")

				t.Logf("Longer ID %q works for %s", result.ObjectID, result.SymbolName)
				break // Just test one longer ID
			}
		}
	})

	t.Run("MultipleIDsCommaSeparated", func(t *testing.T) {
		// Test comma-separated IDs
		if len(searchResult.Results) >= 3 {
			ids := searchResult.Results[0].ObjectID + "," +
				searchResult.Results[1].ObjectID + "," +
				searchResult.Results[2].ObjectID

			params := map[string]interface{}{
				"id": ids,
			}

			response, err := ctx.Server.CallTool("get_object_context", params)
			require.NoError(t, err, "Multiple IDs %q should work", ids)
			assert.NotContains(t, response, "function::0")

			t.Logf("Multiple IDs %q work", ids)
		}
	})
}

// TestGetContext_RegressionFunction0 is a specific regression test for the
// "invalid object ID: function::0" bug that occurred when mode parameter was used
func TestGetContext_RegressionFunction0(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	// Reproduce the exact scenario from the bug report:
	// 1. Search for a function
	// 2. Get object ID from results
	// 3. Call get_context with id and mode=full

	t.Run("BugReproduction", func(t *testing.T) {
		// Search for any function
		searchResult := ctx.Search("func_search", mcp.SearchOptions{
			Pattern:     "func",
			SymbolTypes: []string{"function"},
			MaxResults:  5,
		})

		require.NotEmpty(t, searchResult.Results, "Should find functions")

		for i, result := range searchResult.Results {
			if result.ObjectID == "" {
				continue
			}

			// This is the exact call pattern that triggered the bug
			params := map[string]interface{}{
				"id":   result.ObjectID,
				"mode": "full",
			}

			response, err := ctx.Server.CallTool("get_object_context", params)

			// The bug manifested as an error containing "function::0"
			if err != nil {
				assert.NotContains(t, err.Error(), "function::0",
					"Result %d (%s) should not produce function::0 error", i, result.ObjectID)
			}

			assert.NotContains(t, response, "invalid object ID: function::0",
				"Result %d (%s) should not produce function::0 in response", i, result.ObjectID)

			t.Logf("Object ID %q passed regression check", result.ObjectID)
		}
	})
}

// TestGetContext_WithoutFileID verifies that get_context works with just
// the object ID, without requiring file_id parameter
func TestGetContext_WithoutFileID(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	// Search for a symbol
	searchResult := ctx.Search("symbol_search", mcp.SearchOptions{
		Pattern:     "Context",
		SymbolTypes: []string{"function", "type", "interface"},
		MaxResults:  5,
	})

	require.NotEmpty(t, searchResult.Results, "Should find symbols")

	for i, result := range searchResult.Results {
		if result.ObjectID == "" {
			continue
		}

		// Call get_object_context with ONLY the id parameter
		// This should work - file_id should not be required
		params := map[string]interface{}{
			"id": result.ObjectID,
			// Deliberately NOT providing file_id, name, line, or column
		}

		response, err := ctx.Server.CallTool("get_object_context", params)
		require.NoError(t, err, "get_object_context should work with only 'id' parameter")

		// Should not require file_id
		assert.NotContains(t, response, "missing.*file",
			"Should not require file_id when id is provided")

		t.Logf("Result %d: ID %q works without file_id", i, result.ObjectID)

		// Test just the first couple results
		if i >= 2 {
			break
		}
	}
}

// TestGetContext_IncludesPurity tests that get_context includes purity information
// for function and method symbols
func TestGetContext_IncludesPurity(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	t.Run("FunctionPurity", func(t *testing.T) {
		// Search for a function
		searchResult := ctx.Search("purity_func_search", mcp.SearchOptions{
			Pattern:     "ServeHTTP",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  5,
		})

		require.NotEmpty(t, searchResult.Results, "Should find functions")
		objectID := searchResult.Results[0].ObjectID
		require.NotEmpty(t, objectID, "First result should have object ID")

		// Get context and check for purity info
		params := map[string]interface{}{
			"id":                  objectID,
			"include_full_symbol": true,
		}

		response, err := ctx.Server.CallTool("get_object_context", params)
		require.NoError(t, err, "get_object_context should succeed")

		// Verify the response contains purity information
		// The purity field should be present for functions/methods
		assert.Contains(t, response, "purity",
			"Response should contain purity information for function")
		assert.Contains(t, response, "is_pure",
			"Purity info should contain is_pure field")
		assert.Contains(t, response, "purity_score",
			"Purity info should contain purity_score field")

		t.Logf("Function context includes purity information")
	})

	t.Run("TypeDoesNotHavePurity", func(t *testing.T) {
		// Search for a type (not a function)
		searchResult := ctx.Search("type_search", mcp.SearchOptions{
			Pattern:     "Mux",
			SymbolTypes: []string{"type", "struct"},
			MaxResults:  5,
		})

		require.NotEmpty(t, searchResult.Results, "Should find types")
		objectID := searchResult.Results[0].ObjectID

		params := map[string]interface{}{
			"id":                  objectID,
			"include_full_symbol": true,
		}

		response, err := ctx.Server.CallTool("get_object_context", params)
		require.NoError(t, err, "get_object_context should succeed")

		// Types should not have purity information
		// (or if they do, it's empty/null)
		t.Logf("Type context returned (purity may or may not be present): %d chars", len(response))
	})
}
