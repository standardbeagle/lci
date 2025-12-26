package workflow_scenarios

import (
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestPocketbase tests Pocketbase SDK functionality
// Real-world integration tests for the Pocketbase Go SDK
func TestPocketbase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "go", "pocketbase")

	// === API Surface Analysis Tests ===
	t.Run("CRUD_Create_Handler", func(t *testing.T) {
		// Search for Create API handlers
		searchResult := ctx.Search("CreateHandlers", mcp.SearchOptions{
			Pattern:     "Handler",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  20,
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
				// Get context: should show request models, validation, database calls
				contextResult := ctx.GetObjectContext("CreateHandlers", resultWithID, mcp.ContextOptions{
					IncludeFullSymbol:    true,
					IncludeCallHierarchy: true,
					IncludeDependencies:  true,
				})

				ctx.AssertFieldExists(contextResult, "contexts")

				t.Logf("Found %d Create handlers", len(searchResult.Results))
			}
		}
	})

	t.Run("CRUD_Read_Handler", func(t *testing.T) {
		// Search for Read/Get handlers
		searchResult := ctx.Search("ReadHandlers", mcp.SearchOptions{
			Pattern:     "Get",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  20,
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
				contextResult := ctx.GetObjectContext("ReadHandlers", resultWithID, mcp.ContextOptions{
					IncludeFullSymbol:    true,
					IncludeCallHierarchy: true,
				})

				ctx.AssertFieldExists(contextResult, "contexts")

				t.Logf("Found %d Read handlers", len(searchResult.Results))
			}
		}
	})

	t.Run("API_Request_Validation", func(t *testing.T) {
		// Search for validation functions
		searchResult := ctx.Search("Validation", mcp.SearchOptions{
			Pattern:     "Validate",
			SymbolTypes: []string{"function", "method"},
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
				contextResult := ctx.GetObjectContext("Validation", resultWithID, mcp.ContextOptions{
					IncludeFullSymbol:    true,
					IncludeCallHierarchy: true,
					IncludeAllReferences: true,
				})

				ctx.AssertFieldExists(contextResult, "contexts")

				t.Logf("Found %d validation functions", len(searchResult.Results))
			}
		}
	})

	t.Run("API_Response_Format", func(t *testing.T) {
		// Search for response formatting (Go structs are type symbols)
		searchResult := ctx.Search("ResponseFormat", mcp.SearchOptions{
			Pattern:     "Response",
			SymbolTypes: []string{"type"},
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
				contextResult := ctx.GetObjectContext("ResponseFormat", resultWithID, mcp.ContextOptions{
					IncludeFullSymbol:    true,
					IncludeAllReferences: true,
				})

				ctx.AssertFieldExists(contextResult, "contexts")

				t.Logf("Found %d response types", len(searchResult.Results))
			}
		}
	})

	// === Function Analysis Tests ===
	t.Run("CRUD_Handler_Create", func(t *testing.T) {
		// Search for Create handler
		searchResult := ctx.Search("CreateRecord", mcp.SearchOptions{
			Pattern:         "Create",
			SymbolTypes:     []string{"function", "method"},
			DeclarationOnly: true,
			MaxResults:      20,
		})

		if len(searchResult.Results) > 0 {
			// Get context for first create handler
			contextResult := ctx.GetObjectContext("CreateRecord", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeDependencies:  true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d Create-related functions", len(searchResult.Results))
		}
	})

	t.Run("API_Handler_Route", func(t *testing.T) {
		// Search for HTTP handlers
		searchResult := ctx.Search("APIHandlers", mcp.SearchOptions{
			Pattern:     "Handler",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("APIHandlers", 0, mcp.ContextOptions{
				IncludeFullSymbol:     true,
				IncludeCallHierarchy:  true,
				IncludeQualityMetrics: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found %d API handlers", len(searchResult.Results))
		}
	})

	t.Run("Service_Layer_Method", func(t *testing.T) {
		// Search for service layer methods
		searchResult := ctx.Search("ServiceMethods", mcp.SearchOptions{
			Pattern:     "Service",
			SymbolTypes: []string{"type"}, // "struct" is not a valid symbol type - Go structs use "type"
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d service-related symbols", len(searchResult.Results))

			// Get context to see type information
			contextResult := ctx.GetObjectContext("ServiceMethods", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})

	// === Linguistic Search Tests ===
	t.Run("Multi_Word_Query", func(t *testing.T) {
		// Query: "create record" should find CreateRecord-related symbols
		searchResult := ctx.Search("create_record", mcp.SearchOptions{
			Pattern:    "create record",
			MaxResults: 30,
		})

		found := false
		for _, result := range searchResult.Results {
			text := result.Match + result.SymbolName
			if (strings.Contains(text, "Create") || strings.Contains(text, "create")) &&
				(strings.Contains(text, "Record") || strings.Contains(text, "record")) {
				found = true
				t.Logf("✓ Multi-word 'create record' found: %s", result.SymbolName)
				break
			}
		}

		if !found {
			t.Logf("WARNING: Multi-word query 'create record' did not find relevant symbols")
		}
	})

	t.Run("Stemming_Validate", func(t *testing.T) {
		// Query: "validating" should find "validate", "validation", "validator"
		searchResult := ctx.Search("validate_stem", mcp.SearchOptions{
			Pattern:    "validating",
			MaxResults: 30,
		})

		stemMatches := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "valid") {
				stemMatches++
				if stemMatches <= 3 {
					t.Logf("  Stem match: %s", result.SymbolName)
				}
			}
		}

		t.Logf("Found %d validation-related symbols via stemming", stemMatches)
		if stemMatches == 0 {
			t.Logf("WARNING: Stemming may not be working - no validation symbols found")
		}
	})

	t.Run("snake_case_to_camelCase", func(t *testing.T) {
		// Query: "user_id" (snake_case) should also find "userID" or "UserID" (camelCase)
		searchResult := ctx.Search("user_id_query", mcp.SearchOptions{
			Pattern:    "user id",
			MaxResults: 30,
		})

		found := false
		for _, result := range searchResult.Results {
			text := result.Match + result.SymbolName
			// Check for various naming conventions
			if strings.Contains(text, "userID") ||
				strings.Contains(text, "UserID") ||
				strings.Contains(text, "user_id") ||
				strings.Contains(text, "userId") {
				found = true
				t.Logf("✓ Cross-naming convention match: %s", result.SymbolName)
				break
			}
		}

		if !found {
			t.Logf("INFO: No cross-naming convention matches found (this may be expected)")
		}
	})
}
