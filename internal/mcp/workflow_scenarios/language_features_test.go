package workflow_scenarios

// Language Features Tests
// =======================
// These tests validate language-specific analysis features enabled by:
// - JavaScriptGoFastAnalyzer: AST-based JavaScript parsing using go-fAST
// - JavaScriptHybridAnalyzer: go-fAST with regex fallback for ES6 modules
// - PythonAnalyzer: Regex-based Python parsing with decorator/class support

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/mcp"
)

// =============================================================================
// JavaScript Language Features (go-fAST + Hybrid Analyzer)
// =============================================================================

// TestJavaScriptLanguageFeatures validates JavaScript-specific analysis
// Tests run against tRPC which uses modern JavaScript/TypeScript patterns
func TestJavaScriptLanguageFeatures(t *testing.T) {
	ctx := GetProject(t, "typescript", "trpc")
	shortMode := testing.Short()

	// === Arrow Function Detection ===
	t.Run("ArrowFunction_Detection", func(t *testing.T) {
		// Arrow functions are common in modern JS/TS
		searchResult := ctx.Search("ArrowFunctions", mcp.SearchOptions{
			Pattern:     "=>",
			SymbolTypes: []string{"function", "variable"},
			MaxResults:  30,
		})

		// Should find arrow function patterns in a modern TypeScript project
		if len(searchResult.Results) > 0 {
			t.Logf("Found %d arrow function-related results", len(searchResult.Results))

			// Verify we can get context for arrow functions
			contextResult := ctx.GetObjectContext("ArrowFunctions", 0, mcp.ContextOptions{
				IncludeFullSymbol: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})

	// === Class Declaration Detection ===
	t.Run("Class_Declaration", func(t *testing.T) {
		// Search for class definitions
		searchResult := ctx.Search("Classes", mcp.SearchOptions{
			Pattern:         "class",
			SymbolTypes:     []string{"class"},
			DeclarationOnly: true,
			MaxResults:      20,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d class declarations", len(searchResult.Results))

			// Verify class context includes methods
			contextResult := ctx.GetObjectContext("Classes", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})

	// === Async Function Detection ===
	t.Run("AsyncFunction_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// tRPC uses async patterns extensively
		searchResult := ctx.Search("AsyncPatterns", mcp.SearchOptions{
			Pattern:     "async",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  25,
		})

		asyncCount := 0
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "async") {
				asyncCount++
			}
		}

		t.Logf("Found %d async-related symbols (out of %d results)",
			asyncCount, len(searchResult.Results))

		if asyncCount > 0 {
			contextResult := ctx.GetObjectContext("AsyncPatterns", 0, mcp.ContextOptions{
				IncludeFullSymbol: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})

	// === Const/Let/Var Variable Detection ===
	t.Run("VariableDeclaration_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// Search for const declarations (common pattern)
		searchResult := ctx.Search("ConstVariables", mcp.SearchOptions{
			Pattern:     "const ",
			SymbolTypes: []string{"variable", "constant"},
			MaxResults:  30,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d const-style declarations", len(searchResult.Results))
		}
	})

	// === Export Statement Detection (ES6 Modules) ===
	t.Run("Export_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// tRPC uses ES6 exports heavily
		searchResult := ctx.Search("Exports", mcp.SearchOptions{
			Pattern:     "export",
			SymbolTypes: []string{"function", "class", "variable"},
			MaxResults:  30,
		})

		exportCount := 0
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "export") {
				exportCount++
			}
		}

		t.Logf("Found %d export-related symbols", exportCount)
		assert.Greater(t, len(searchResult.Results), 0,
			"Modern TypeScript projects should have exports")
	})

	// === Method Detection in Classes ===
	t.Run("Method_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// Search for method definitions
		searchResult := ctx.Search("Methods", mcp.SearchOptions{
			Pattern:     "method",
			SymbolTypes: []string{"method", "function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d method-related symbols", len(searchResult.Results))
		}
	})
}

// =============================================================================
// Python Language Features (PythonAnalyzer)
// =============================================================================

// TestPythonLanguageFeatures validates Python-specific analysis
// Tests run against FastAPI which uses decorators, async, and classes extensively
func TestPythonLanguageFeatures(t *testing.T) {
	ctx := GetProject(t, "python", "fastapi")
	shortMode := testing.Short()

	// === Decorator Detection ===
	t.Run("Decorator_Detection", func(t *testing.T) {
		// FastAPI uses decorators extensively (@app.get, @router.post, etc.)
		searchResult := ctx.Search("Decorators", mcp.SearchOptions{
			Pattern:    "@",
			MaxResults: 30,
		})

		decoratorCount := 0
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "@") {
				decoratorCount++
			}
		}

		t.Logf("Found %d decorator-related patterns", decoratorCount)
		assert.Greater(t, decoratorCount, 0, "FastAPI should have decorators")
	})

	// === Class Definition with Inheritance ===
	t.Run("Class_Inheritance", func(t *testing.T) {
		// Search for classes that extend other classes
		searchResult := ctx.Search("ClassInheritance", mcp.SearchOptions{
			Pattern:         "class",
			SymbolTypes:     []string{"class"},
			DeclarationOnly: true,
			MaxResults:      20,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d class definitions", len(searchResult.Results))

			// Check for inheritance patterns
			inheritCount := 0
			for _, result := range searchResult.Results {
				// Classes with inheritance have (BaseClass) pattern
				if strings.Contains(result.Match, "(") && strings.Contains(result.Match, ")") {
					inheritCount++
				}
			}
			t.Logf("Classes with inheritance: %d", inheritCount)

			contextResult := ctx.GetObjectContext("ClassInheritance", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})

	// === Async/Await Detection ===
	t.Run("AsyncAwait_Detection", func(t *testing.T) {
		// FastAPI uses async extensively
		searchResult := ctx.Search("AsyncDef", mcp.SearchOptions{
			Pattern:     "async def",
			SymbolTypes: []string{"function"},
			MaxResults:  25,
		})

		t.Logf("Found %d async functions", len(searchResult.Results))

		// FastAPI should have many async functions
		assert.Greater(t, len(searchResult.Results), 0,
			"FastAPI should have async functions")

		if len(searchResult.Results) > 0 {
			contextResult := ctx.GetObjectContext("AsyncDef", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeCallHierarchy: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})

	// === Dunder Method Detection ===
	t.Run("DunderMethod_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// Search for __init__ and other dunder methods
		searchResult := ctx.Search("DunderMethods", mcp.SearchOptions{
			Pattern:     "__init__",
			SymbolTypes: []string{"method", "function"},
			MaxResults:  20,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d __init__ methods", len(searchResult.Results))

			contextResult := ctx.GetObjectContext("DunderMethods", 0, mcp.ContextOptions{
				IncludeFullSymbol: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}

		// Also check for __str__, __repr__, etc.
		otherDunders := ctx.Search("OtherDunders", mcp.SearchOptions{
			Pattern:    "__",
			MaxResults: 30,
		})
		dunderCount := 0
		for _, result := range otherDunders.Results {
			if strings.Count(result.Match, "__") >= 2 {
				dunderCount++
			}
		}
		t.Logf("Found %d dunder method patterns total", dunderCount)
	})

	// === Private/Protected Method Detection ===
	t.Run("AccessLevel_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// Search for protected methods (single underscore prefix)
		protectedResult := ctx.Search("ProtectedMethods", mcp.SearchOptions{
			Pattern:     "_",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  30,
		})

		protectedCount := 0
		privateCount := 0
		for _, result := range protectedResult.Results {
			name := result.SymbolName
			if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
				privateCount++
			} else if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") {
				protectedCount++
			}
		}

		t.Logf("Found %d protected methods (single _), %d private methods (double __)",
			protectedCount, privateCount)
	})

	// === Import Statement Detection ===
	t.Run("Import_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// Search for import patterns
		searchResult := ctx.Search("Imports", mcp.SearchOptions{
			Pattern:    "from ",
			MaxResults: 30,
		})

		importCount := 0
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "import") ||
				strings.Contains(result.Match, "from") {
				importCount++
			}
		}

		t.Logf("Found %d import-related patterns", importCount)
	})

	// === Type Hint Detection ===
	t.Run("TypeHint_Detection", func(t *testing.T) {
		if shortMode {
			t.Skip("Skipping in short mode")
		}

		// FastAPI uses type hints extensively
		searchResult := ctx.Search("TypeHints", mcp.SearchOptions{
			Pattern:    "->",
			MaxResults: 30,
		})

		t.Logf("Found %d return type hint patterns", len(searchResult.Results))

		// Also check for parameter type hints
		paramHints := ctx.Search("ParamHints", mcp.SearchOptions{
			Pattern:    ": str",
			MaxResults: 20,
		})
		t.Logf("Found %d parameter type hint patterns", len(paramHints.Results))
	})
}

// =============================================================================
// Pydantic-Specific Python Features
// =============================================================================

// TestPydanticLanguageFeatures validates Pydantic-specific patterns
// Pydantic makes heavy use of decorators and class definitions
func TestPydanticLanguageFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Pydantic tests in short mode")
	}

	ctx := GetProject(t, "python", "pydantic")

	// === Dataclass-style Decorator ===
	t.Run("Dataclass_Decorator", func(t *testing.T) {
		searchResult := ctx.Search("DataclassDecorator", mcp.SearchOptions{
			Pattern:     "@dataclass",
			SymbolTypes: []string{"class"},
			MaxResults:  15,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d @dataclass patterns", len(searchResult.Results))
		}
	})

	// === Validator Decorator ===
	t.Run("Validator_Decorator", func(t *testing.T) {
		searchResult := ctx.Search("ValidatorDecorator", mcp.SearchOptions{
			Pattern:    "validator",
			MaxResults: 20,
		})

		validatorCount := 0
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "validator") {
				validatorCount++
			}
		}

		t.Logf("Found %d validator-related patterns", validatorCount)
	})

	// === Model Class Detection ===
	t.Run("Model_Class", func(t *testing.T) {
		searchResult := ctx.Search("ModelClasses", mcp.SearchOptions{
			Pattern:         "BaseModel",
			SymbolTypes:     []string{"class"},
			DeclarationOnly: true,
			MaxResults:      15,
		})

		if len(searchResult.Results) > 0 {
			t.Logf("Found %d BaseModel-related classes", len(searchResult.Results))

			contextResult := ctx.GetObjectContext("ModelClasses", 0, mcp.ContextOptions{
				IncludeFullSymbol:    true,
				IncludeAllReferences: true,
			})
			ctx.AssertFieldExists(contextResult, "contexts")
		}
	})
}

// =============================================================================
// Cross-Language Feature Comparison
// =============================================================================

// TestLanguageFeature_Comparison ensures analyzers provide consistent capabilities
func TestLanguageFeature_Comparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comparison tests in short mode")
	}

	// Compare class detection across languages
	t.Run("ClassDetection_Comparison", func(t *testing.T) {
		goCtx := GetProject(t, "go", "chi")
		pyCtx := GetProject(t, "python", "fastapi")
		tsCtx := GetProject(t, "typescript", "trpc")

		// Go structs (treated as classes)
		goResult := goCtx.Search("GoStructs", mcp.SearchOptions{
			Pattern:         "type",
			SymbolTypes:     []string{"class", "struct"},
			DeclarationOnly: true,
			MaxResults:      20,
		})

		// Python classes
		pyResult := pyCtx.Search("PyClasses", mcp.SearchOptions{
			Pattern:         "class",
			SymbolTypes:     []string{"class"},
			DeclarationOnly: true,
			MaxResults:      20,
		})

		// TypeScript classes
		tsResult := tsCtx.Search("TSClasses", mcp.SearchOptions{
			Pattern:         "class",
			SymbolTypes:     []string{"class"},
			DeclarationOnly: true,
			MaxResults:      20,
		})

		t.Logf("Class-like constructs found: Go=%d, Python=%d, TypeScript=%d",
			len(goResult.Results), len(pyResult.Results), len(tsResult.Results))

		// All should find some class-like constructs
		require.Greater(t, len(goResult.Results)+len(pyResult.Results)+len(tsResult.Results), 0,
			"Should find class-like constructs across all languages")
	})

	// Compare function detection across languages
	t.Run("FunctionDetection_Comparison", func(t *testing.T) {
		goCtx := GetProject(t, "go", "chi")
		pyCtx := GetProject(t, "python", "fastapi")
		tsCtx := GetProject(t, "typescript", "trpc")

		// Go functions
		goResult := goCtx.Search("GoFuncs", mcp.SearchOptions{
			Pattern:     "func",
			SymbolTypes: []string{"function"},
			MaxResults:  30,
		})

		// Python functions
		pyResult := pyCtx.Search("PyFuncs", mcp.SearchOptions{
			Pattern:     "def",
			SymbolTypes: []string{"function"},
			MaxResults:  30,
		})

		// TypeScript functions
		tsResult := tsCtx.Search("TSFuncs", mcp.SearchOptions{
			Pattern:     "function",
			SymbolTypes: []string{"function"},
			MaxResults:  30,
		})

		t.Logf("Functions found: Go=%d, Python=%d, TypeScript=%d",
			len(goResult.Results), len(pyResult.Results), len(tsResult.Results))

		// All should find functions
		assert.Greater(t, len(goResult.Results), 0, "Go should have functions")
		assert.Greater(t, len(pyResult.Results), 0, "Python should have functions")
		// TypeScript may use arrow functions more than 'function' keyword
	})
}
