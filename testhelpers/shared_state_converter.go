package testhelpers

import (
	"fmt"
	"regexp"
	"strings"
)

// SharedStateConverter helps convert shared package-level variables to isolated test data
type SharedStateConverter struct {
	varPattern     *regexp.Regexp
	constPattern   *regexp.Regexp
	typePattern    *regexp.Regexp
	funcPattern    *regexp.Regexp
	commentPattern *regexp.Regexp
}

// NewSharedStateConverter creates a new converter for shared state
func NewSharedStateConverter() *SharedStateConverter {
	return &SharedStateConverter{
		varPattern:     regexp.MustCompile(`var\s+(\w+)\s+([^=]+?)\s*=\s*(.+)`),
		constPattern:   regexp.MustCompile(`const\s+(\w+)\s+([^=]+?)\s*=\s*(.+)`),
		typePattern:    regexp.MustCompile(`type\s+(\w+)\s+(.+)`),
		funcPattern:    regexp.MustCompile(`func\s+(\w+)\s*\(([^)]*)\)\s*(?:\{([^}]*)\})?`),
		commentPattern: regexp.MustCompile(`//.*|/\*[\s\S]*?\*/`),
	}
}

// PackageDefinition represents a Go package structure
type PackageDefinition struct {
	Name     string
	Imports  []string
	Vars     []VarDefinition
	Consts   []ConstDefinition
	Types    []TypeDefinition
	Funcs    []FuncDefinition
	Comments []string
}

// VarDefinition represents a variable declaration
type VarDefinition struct {
	Name  string
	Type  string
	Value string
}

// ConstDefinition represents a constant declaration
type ConstDefinition struct {
	Name  string
	Type  string
	Value string
}

// TypeDefinition represents a type declaration
type TypeDefinition struct {
	Name string
	Def  string
}

// FuncDefinition represents a function declaration
type FuncDefinition struct {
	Name       string
	Parameters []string
	ReturnType string
	Body       string
	Exported   bool
}

// ConvertSharedStateFile converts shared state content to isolated test data builder
func (ssc *SharedStateConverter) ConvertSharedStateFile(content string, packageName string) *TestDataBuilder {
	// Remove comments first
	cleanContent := ssc.commentPattern.ReplaceAllString(content, "")

	// Parse package structure
	pkgDef := ssc.parsePackage(cleanContent)

	// Build test data using the parsed structure
	builder := NewTestDataBuilder()

	// Create Go file with all the elements
	elements := []GoFileElement{PackageDecl{Name: packageName}}

	// Add imports if any
	if len(pkgDef.Imports) > 0 {
		elements = append(elements, ImportDecl{Imports: pkgDef.Imports})
	}

	// Add variables
	for _, variable := range pkgDef.Vars {
		elements = append(elements, GlobalVar{
			Name:  variable.Name,
			Type:  variable.Type,
			Value: variable.Value,
		})
	}

	// Add constants
	for _, constDef := range pkgDef.Consts {
		elements = append(elements, ConstElement{
			Name:  constDef.Name,
			Type:  constDef.Type,
			Value: constDef.Value,
		})
	}

	// Add types
	for _, typeDef := range pkgDef.Types {
		elements = append(elements, TypeElement{
			Name: typeDef.Name,
			Def:  typeDef.Def,
		})
	}

	// Add functions
	for _, funcDef := range pkgDef.Funcs {
		elements = append(elements, FunctionDecl{
			Name:       funcDef.Name,
			Parameters: funcDef.Parameters,
			ReturnType: funcDef.ReturnType,
			Body:       funcDef.Body,
			Exported:   funcDef.Exported,
		})
	}

	return builder.AddGoFile("test.go", elements...)
}

// parsePackage parses Go code into a PackageDefinition
func (ssc *SharedStateConverter) parsePackage(content string) *PackageDefinition {
	lines := strings.Split(content, "\n")

	pkgDef := &PackageDefinition{
		Vars:   make([]VarDefinition, 0),
		Consts: make([]ConstDefinition, 0),
		Types:  make([]TypeDefinition, 0),
		Funcs:  make([]FuncDefinition, 0),
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse variables
		if matches := ssc.varPattern.FindStringSubmatch(line); len(matches) > 3 {
			pkgDef.Vars = append(pkgDef.Vars, VarDefinition{
				Name:  strings.TrimSpace(matches[1]),
				Type:  strings.TrimSpace(matches[2]),
				Value: strings.TrimSpace(matches[3]),
			})
			continue
		}

		// Parse constants
		if matches := ssc.constPattern.FindStringSubmatch(line); len(matches) > 3 {
			pkgDef.Consts = append(pkgDef.Consts, ConstDefinition{
				Name:  strings.TrimSpace(matches[1]),
				Type:  strings.TrimSpace(matches[2]),
				Value: strings.TrimSpace(matches[3]),
			})
			continue
		}

		// Parse types
		if matches := ssc.typePattern.FindStringSubmatch(line); len(matches) > 2 {
			pkgDef.Types = append(pkgDef.Types, TypeDefinition{
				Name: strings.TrimSpace(matches[1]),
				Def:  strings.TrimSpace(matches[2]),
			})
			continue
		}

		// Parse functions
		if matches := ssc.funcPattern.FindStringSubmatch(line); len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			exported := len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'

			// Parse parameters
			var params []string
			if len(matches) > 2 && matches[2] != "" {
				paramStr := strings.TrimSpace(matches[2])
				if paramStr != "" {
					params = strings.Split(paramStr, ",")
					for i := range params {
						params[i] = strings.TrimSpace(params[i])
					}
				}
			}

			pkgDef.Funcs = append(pkgDef.Funcs, FuncDefinition{
				Name:       name,
				Parameters: params,
				Exported:   exported,
			})
		}
	}

	return pkgDef
}

// ConvertTestFile converts an existing test file that uses shared state to use isolated test data
func (ssc *SharedStateConverter) ConvertTestFile(testContent string, sharedStateContent string) string {
	// This would be a more complex conversion that analyzes test patterns
	// and replaces shared state references with isolated test data
	// For now, return a template for manual conversion

	return `// CONVERTED TEST - Uses isolated test data instead of shared state
func TestExampleIsolated(t *testing.T) {
	testhelpers.RunIsolatedTest(t, "Example", func(t *testing.T) {
		// Convert this shared state:
		/*
		` + sharedStateContent + `
		*/

		// To isolated test data:
		testData := testhelpers.NewSharedStateConverter().
			ConvertSharedStateFile(` + "`" + sharedStateContent + "`" + `, "main").
			Build()

		// Use testData.FileStore and other isolated resources
		// instead of shared package-level variables
	})
}`
}

// CommonTestPatterns provides conversion patterns for common shared state scenarios
type CommonTestPatterns struct {
	GlobalVarPattern   string
	GlobalConstPattern string
	GlobalTypePattern  string
	GlobalFuncPattern  string
}

// GetCommonPatterns returns common patterns for converting shared state
func GetCommonPatterns() *CommonTestPatterns {
	return &CommonTestPatterns{
		GlobalVarPattern:   `var (\w+) = (.+) -> testhelpers.GlobalVar{Name: "$1", Type: "string", Value: $2}`,
		GlobalConstPattern: `const (\w+) = (.+) -> "$1 := $2"`,
		GlobalTypePattern:  `type (\w+) (.+) -> testhelpers.TypeDecl{Name: "$1", Def: "$2"}`,
		GlobalFuncPattern:  `func (\w+) -> testhelpers.FunctionDecl{Name: "$1", Exported: true}`,
	}
}

// GenerateIsolatedTestTemplate creates a template for converting a specific test
func (ssc *SharedStateConverter) GenerateIsolatedTestTemplate(originalTestName string, sharedVars []string) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf(`func %sIsolated(t *testing.T) {
	testhelpers.RunIsolatedTest(t, "%s", func(t *testing.T) {
`, originalTestName+"Isolated", originalTestName))

	builder.WriteString(`		// Create isolated test data instead of using shared variables
		testData := testhelpers.NewTestDataBuilder().
`)

	for _, varName := range sharedVars {
		builder.WriteString(fmt.Sprintf(`			// Previously: var %s = "shared_value"
`, varName))
	}

	builder.WriteString(`			Build()

		// Use testData instead of shared state
		// All global state is now isolated to this test
	})
}`)

	return builder.String()
}
