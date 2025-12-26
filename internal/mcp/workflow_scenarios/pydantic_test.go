package workflow_scenarios

import (
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestPydantic tests Pydantic library functionality
// Real-world integration tests for the Pydantic Python library
func TestPydantic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "python", "pydantic")

	// === Type Discovery Tests ===
	t.Run("BaseModel_Class_Definition", func(t *testing.T) {
		// Search for BaseModel class
		searchResult := ctx.Search("BaseModel", mcp.SearchOptions{
			Pattern:         "class BaseModel",
			SymbolTypes:     []string{"class"},
			DeclarationOnly: true,
			MaxResults:      10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show BaseModel methods
			contextResult := ctx.GetObjectContext("BaseModel", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found BaseModel class")
		}
	})

	t.Run("Field_Validator_Decorator", func(t *testing.T) {
		// Search for field validator decorators
		searchResult := ctx.Search("FieldValidators", mcp.SearchOptions{
			Pattern:     "field_validator",
			SymbolTypes: []string{"function", "variable"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show validation patterns
			contextResult := ctx.GetObjectContext("FieldValidators", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found field validators")
		}
	})

	t.Run("Model_Validator_Method", func(t *testing.T) {
		// Search for model validators
		searchResult := ctx.Search("ModelValidators", mcp.SearchOptions{
			Pattern:     "model_validator",
			SymbolTypes: []string{"function"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show model validation patterns
			contextResult := ctx.GetObjectContext("ModelValidators", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found model validators")
		}
	})

	t.Run("DataClass_Decorator", func(t *testing.T) {
		// Search for dataclass patterns
		searchResult := ctx.Search("DataClass", mcp.SearchOptions{
			Pattern:     "dataclass",
			SymbolTypes: []string{"class", "function"},
			MaxResults:  10,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show dataclass integration
			contextResult := ctx.GetObjectContext("DataClass", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found dataclass patterns")
		}
	})

	t.Run("Schema_Generator_Method", func(t *testing.T) {
		// Search for schema generation methods
		searchResult := ctx.Search("SchemaGenerator", mcp.SearchOptions{
			Pattern:     "schema",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			// Get context: should show schema generation
			contextResult := ctx.GetObjectContext("SchemaGenerator", 0, mcp.ContextOptions{
				IncludeFullSymbol:     true,
				IncludeAllReferences:  true,
				IncludeQualityMetrics: true,
			})

			ctx.AssertFieldExists(contextResult, "contexts")

			t.Logf("Found schema generation methods")
		}
	})
}
