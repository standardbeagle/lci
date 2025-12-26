package search

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestSemanticFilterConsolidated uses data-driven testing to consolidate semantic filter tests
func TestSemanticFilterConsolidated(t *testing.T) {
	testCases := []SemanticFilterTestCase{
		{
			Name: "CommentFiltering",
			Content: CreateTestContent(
				"package main",
				"",
				`import "fmt"`,
				"",
				"// Comment",
				"func commentedFunction() {",
				"\t// Another comment",
				`\tfmt.Println("test")`,
				"}",
				"",
				"/* Multi-line comment",
				"   spanning multiple lines */",
				"func multiLineFunction() {",
				`\tfmt.Println("test")`,
				"}",
			),
			EnhancedSymbols: func() []*types.EnhancedSymbol {
				content := CreateTestContent(
					"package main", "", `import "fmt"`, "",
					"// Comment", "func commentedFunction() {",
					"\t// Another comment", `\tfmt.Println("test")`, "}",
					"", "/* Multi-line comment", "   spanning multiple lines */",
					"func multiLineFunction() {", `\tfmt.Println("test")`, "}",
				)
				return []*types.EnhancedSymbol{
					{Symbol: types.Symbol{Name: "commentedFunction", Type: types.SymbolTypeFunction, Line: FindLineNumber(content, "func commentedFunction")}},
					{Symbol: types.Symbol{Name: "multiLineFunction", Type: types.SymbolTypeFunction, Line: FindLineNumber(content, "func multiLineFunction")}},
				}
			}(),
			Matches: func() []ZeroAllocSearchMatch {
				content := CreateTestContent(
					"package main", "", `import "fmt"`, "",
					"// Comment", "func commentedFunction() {",
					"\t// Another comment", `\tfmt.Println("test")`, "}",
					"", "/* Multi-line comment", "   spanning multiple lines */",
					"func multiLineFunction() {", `\tfmt.Println("test")`, "}",
				)
				return []ZeroAllocSearchMatch{
					{Line: FindLineNumber(content, "// Comment"), Start: 3, End: 10, Pattern: "comment"},
					{Line: FindLineNumber(content, "func commentedFunction"), Start: 6, End: 23, Pattern: "commentedFunction"},
					{Line: FindLineNumber(content, "// Another comment"), Start: 3, End: 10, Pattern: "Another"},
					{Line: FindLineNumber(content, "fmt.Println"), Start: 9, End: 14, Pattern: "fmt"},
					{Line: FindLineNumber(content, "/* Multi-line comment"), Start: 3, End: 13, Pattern: "Multi-line"},
					{Line: FindLineNumber(content, "func multiLineFunction"), Start: 6, End: 24, Pattern: "multiLineFunction"},
				}
			}(),
			FilterOptions:    types.SearchOptions{ExcludeComments: true},
			ExpectedCount:    3, // function definitions + fmt line
			ShouldContain:    []string{"commentedFunction", "multiLineFunction", "fmt"},
			ShouldNotContain: []string{"comment", "Another", "Multi-line"},
		},
		{
			Name: "DeclarationUsageFiltering_DeclarationOnly",
			Content: CreateTestContent(
				"package main",
				"",
				`import "fmt"`,
				"",
				"func main() {",
				`\tvar localVar = "test"`,
				"\tconst localConst = 42",
				"",
				"\tfmt.Println(localVar)",
				"\tfmt.Println(localConst)",
				"}",
			),
			EnhancedSymbols: func() []*types.EnhancedSymbol {
				content := CreateTestContent(
					"package main", "", `import "fmt"`, "", "func main() {",
					`\tvar localVar = "test"`, "\tconst localConst = 42", "",
					"\tfmt.Println(localVar)", "\tfmt.Println(localConst)", "}",
				)
				return []*types.EnhancedSymbol{
					{Symbol: types.Symbol{Name: "main", Type: types.SymbolTypeFunction, Line: FindLineNumber(content, "func main")}},
					{Symbol: types.Symbol{Name: "localVar", Type: types.SymbolTypeVariable, Line: FindLineNumber(content, "var localVar")}},
					{Symbol: types.Symbol{Name: "localConst", Type: types.SymbolTypeConstant, Line: FindLineNumber(content, "const localConst")}},
				}
			}(),
			Matches: func() []ZeroAllocSearchMatch {
				content := CreateTestContent(
					"package main", "", `import "fmt"`, "", "func main() {",
					`\tvar localVar = "test"`, "\tconst localConst = 42", "",
					"\tfmt.Println(localVar)", "\tfmt.Println(localConst)", "}",
				)
				return []ZeroAllocSearchMatch{
					{Line: FindLineNumber(content, "var localVar"), Start: 6, End: 14, Pattern: "localVar"},
					{Line: FindLineNumber(content, "const localConst"), Start: 7, End: 16, Pattern: "localConst"},
					{Line: FindLineNumber(content, "fmt.Println(localVar)"), Start: 13, End: 21, Pattern: "localVar"},
					{Line: FindLineNumber(content, "fmt.Println(localConst)"), Start: 13, End: 22, Pattern: "localConst"},
				}
			}(),
			FilterOptions: types.SearchOptions{DeclarationOnly: true},
			ExpectedCount: 2, // only declarations
		},
		{
			Name: "DeclarationUsageFiltering_UsageOnly",
			Content: CreateTestContent(
				"package main",
				"",
				`import "fmt"`,
				"",
				"func main() {",
				`\tvar localVar = "test"`,
				"\tconst localConst = 42",
				"",
				"\tfmt.Println(localVar)",
				"\tfmt.Println(localConst)",
				"}",
			),
			EnhancedSymbols: func() []*types.EnhancedSymbol {
				content := CreateTestContent(
					"package main", "", `import "fmt"`, "", "func main() {",
					`\tvar localVar = "test"`, "\tconst localConst = 42", "",
					"\tfmt.Println(localVar)", "\tfmt.Println(localConst)", "}",
				)
				return []*types.EnhancedSymbol{
					{Symbol: types.Symbol{Name: "main", Type: types.SymbolTypeFunction, Line: FindLineNumber(content, "func main")}},
					{Symbol: types.Symbol{Name: "localVar", Type: types.SymbolTypeVariable, Line: FindLineNumber(content, "var localVar")}},
					{Symbol: types.Symbol{Name: "localConst", Type: types.SymbolTypeConstant, Line: FindLineNumber(content, "const localConst")}},
				}
			}(),
			Matches: func() []ZeroAllocSearchMatch {
				content := CreateTestContent(
					"package main", "", `import "fmt"`, "", "func main() {",
					`\tvar localVar = "test"`, "\tconst localConst = 42", "",
					"\tfmt.Println(localVar)", "\tfmt.Println(localConst)", "}",
				)
				return []ZeroAllocSearchMatch{
					{Line: FindLineNumber(content, "var localVar"), Start: 6, End: 14, Pattern: "localVar"},
					{Line: FindLineNumber(content, "const localConst"), Start: 7, End: 16, Pattern: "localConst"},
					{Line: FindLineNumber(content, "fmt.Println(localVar)"), Start: 13, End: 21, Pattern: "localVar"},
					{Line: FindLineNumber(content, "fmt.Println(localConst)"), Start: 13, End: 22, Pattern: "localConst"},
				}
			}(),
			FilterOptions: types.SearchOptions{UsageOnly: true},
			ExpectedCount: 2, // only usage
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			RunSemanticFilterTest(t, tc)
		})
	}
}
