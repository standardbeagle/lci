package analysis

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// PythonAnalyzer implements language-specific analysis for Python code
// Uses Tree-Sitter for parsing with enhanced semantic analysis
type PythonAnalyzer struct {
	// Regex patterns for additional analysis
	classPattern     *regexp.Regexp
	functionPattern  *regexp.Regexp
	importPattern    *regexp.Regexp
	fromImportPattern *regexp.Regexp
	decoratorPattern *regexp.Regexp
	typeHintPattern  *regexp.Regexp
	asyncPattern     *regexp.Regexp
}

// NewPythonAnalyzer creates a new Python analyzer
func NewPythonAnalyzer() *PythonAnalyzer {
	return &PythonAnalyzer{
		classPattern:     regexp.MustCompile(`(?m)^(\s*)class\s+(\w+)(?:\s*\(([^)]*)\))?\s*:`),
		functionPattern:  regexp.MustCompile(`(?m)^(\s*)(async\s+)?def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*([^:]+))?\s*:`),
		importPattern:    regexp.MustCompile(`(?m)^import\s+(.+)$`),
		fromImportPattern: regexp.MustCompile(`(?m)^from\s+(\S+)\s+import\s+(.+)$`),
		decoratorPattern: regexp.MustCompile(`(?m)^(\s*)@(\w+(?:\.\w+)*)(?:\s*\(([^)]*)\))?`),
		typeHintPattern:  regexp.MustCompile(`(\w+)\s*:\s*([^=,\)]+)`),
		asyncPattern:     regexp.MustCompile(`\basync\s+def\b`),
	}
}

// GetLanguageName returns the language name
func (pa *PythonAnalyzer) GetLanguageName() string {
	return "python"
}

// ExtractSymbols extracts all symbols from Python code
func (pa *PythonAnalyzer) ExtractSymbols(fileID types.FileID, content, filePath string) ([]*types.UniversalSymbolNode, error) {
	var symbols []*types.UniversalSymbolNode
	localSymbolID := uint32(1)
	lines := strings.Split(content, "\n")

	// Track class context for method detection
	var currentClass *types.UniversalSymbolNode
	var currentClassIndent int

	// Track decorators for the next definition
	var pendingDecorators []types.ContextAttribute

	for lineNum, line := range lines {
		lineNumber := lineNum + 1

		// Check for decorators
		if match := pa.decoratorPattern.FindStringSubmatch(line); match != nil {
			decorator := types.ContextAttribute{
				Type:  types.AttrTypeDecorator,
				Value: "@" + match[2],
				Line:  lineNumber,
			}
			if match[3] != "" {
				decorator.Value += "(" + match[3] + ")"
			}
			pendingDecorators = append(pendingDecorators, decorator)
			continue
		}

		// Check for class definition
		if match := pa.classPattern.FindStringSubmatch(line); match != nil {
			indent := len(match[1])
			className := match[2]
			bases := match[3]

			// If we were in a class and this class has same or less indent, exit the class
			if currentClass != nil && indent <= currentClassIndent {
				currentClass = nil
			}

			symbol := pa.createClassSymbol(fileID, localSymbolID, className, lineNumber, bases)

			// Apply pending decorators
			if len(pendingDecorators) > 0 {
				symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, pendingDecorators...)
				pendingDecorators = nil
			}

			symbols = append(symbols, symbol)
			localSymbolID++

			currentClass = symbol
			currentClassIndent = indent
			continue
		}

		// Check for function/method definition
		if match := pa.functionPattern.FindStringSubmatch(line); match != nil {
			indent := len(match[1])
			isAsync := match[2] != ""
			funcName := match[3]
			params := match[4]
			returnType := strings.TrimSpace(match[5])

			// Determine if this is a method (inside a class)
			isMethod := currentClass != nil && indent > currentClassIndent

			// Exit class context if we're at same or less indent
			if currentClass != nil && indent <= currentClassIndent {
				currentClass = nil
			}

			symbol := pa.createFunctionSymbol(fileID, localSymbolID, funcName, lineNumber, isAsync, isMethod, params, returnType)

			// Apply pending decorators
			if len(pendingDecorators) > 0 {
				symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, pendingDecorators...)
				pendingDecorators = nil
			}

			// If it's a method, set containment relationship
			if isMethod && currentClass != nil {
				symbol.Relationships.ContainedBy = &currentClass.Identity.ID
				currentClass.Relationships.Contains = append(currentClass.Relationships.Contains, symbol.Identity.ID)
			}

			symbols = append(symbols, symbol)
			localSymbolID++
			continue
		}

		// Clear pending decorators if we hit a non-decorator, non-definition line that's not empty/comment
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "@") {
			pendingDecorators = nil
		}
	}

	return symbols, nil
}

// AnalyzeExtends analyzes Python inheritance relationships
func (pa *PythonAnalyzer) AnalyzeExtends(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	if symbol.Identity.Kind != types.SymbolKindClass {
		return nil, nil
	}

	var extends []types.CompositeSymbolID

	// Find the class definition and extract base classes
	pattern := regexp.MustCompile(fmt.Sprintf(`class\s+%s\s*\(([^)]+)\)\s*:`, regexp.QuoteMeta(symbol.Identity.Name)))
	matches := pattern.FindStringSubmatch(content)

	if len(matches) > 1 {
		bases := strings.Split(matches[1], ",")
		for _, base := range bases {
			base = strings.TrimSpace(base)
			// Skip common non-class bases
			if base != "" && base != "object" && !strings.HasPrefix(base, "metaclass=") {
				// Remove any generic parameters
				if idx := strings.Index(base, "["); idx != -1 {
					base = base[:idx]
				}
				// Create placeholder - would be resolved by symbol linker
				extends = append(extends, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, 0))
			}
		}
	}

	return extends, nil
}

// AnalyzeImplements analyzes interface implementation (Python uses duck typing)
func (pa *PythonAnalyzer) AnalyzeImplements(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	// Python uses duck typing - no explicit interface implementation
	// Could potentially detect Protocol implementations from type hints
	return nil, nil
}

// AnalyzeContains analyzes containment relationships
func (pa *PythonAnalyzer) AnalyzeContains(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	// Containment is already populated during ExtractSymbols
	return symbol.Relationships.Contains, nil
}

// AnalyzeDependencies analyzes import dependencies
func (pa *PythonAnalyzer) AnalyzeDependencies(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.SymbolDependency, error) {
	var dependencies []types.SymbolDependency
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		// Check for 'import x' statements
		if match := pa.importPattern.FindStringSubmatch(line); match != nil {
			modules := strings.Split(match[1], ",")
			for _, module := range modules {
				module = strings.TrimSpace(module)
				// Handle 'import x as y'
				if idx := strings.Index(module, " as "); idx != -1 {
					module = module[:idx]
				}
				dependencies = append(dependencies, types.SymbolDependency{
					Target:     types.NewCompositeSymbolID(0, 0),
					Type:       types.DependencyImport,
					Strength:   types.DependencyModerate,
					Context:    fmt.Sprintf("import at line %d", lineNum+1),
					ImportPath: module,
				})
			}
		}

		// Check for 'from x import y' statements
		if match := pa.fromImportPattern.FindStringSubmatch(line); match != nil {
			modulePath := match[1]
			imports := match[2]

			// Handle wildcard imports
			if strings.TrimSpace(imports) == "*" {
				dependencies = append(dependencies, types.SymbolDependency{
					Target:     types.NewCompositeSymbolID(0, 0),
					Type:       types.DependencyImport,
					Strength:   types.DependencyStrong, // Wildcard imports create stronger coupling
					Context:    fmt.Sprintf("from %s import * at line %d", modulePath, lineNum+1),
					ImportPath: modulePath,
				})
			} else {
				// Individual imports
				for _, imp := range strings.Split(imports, ",") {
					imp = strings.TrimSpace(imp)
					// Handle 'import x as y'
					if idx := strings.Index(imp, " as "); idx != -1 {
						imp = imp[:idx]
					}
					dependencies = append(dependencies, types.SymbolDependency{
						Target:     types.NewCompositeSymbolID(0, 0),
						Type:       types.DependencyImport,
						Strength:   types.DependencyModerate,
						Context:    fmt.Sprintf("from %s import %s at line %d", modulePath, imp, lineNum+1),
						ImportPath: modulePath + "." + imp,
					})
				}
			}
		}
	}

	return dependencies, nil
}

// AnalyzeCalls analyzes function call relationships
func (pa *PythonAnalyzer) AnalyzeCalls(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.FunctionCall, error) {
	if symbol.Identity.Kind != types.SymbolKindFunction && symbol.Identity.Kind != types.SymbolKindMethod {
		return nil, nil
	}

	var calls []types.FunctionCall
	lines := strings.Split(content, "\n")

	// Find the function body
	startLine := symbol.Identity.Location.Line
	funcIndent := -1
	inFunction := false

	callPattern := regexp.MustCompile(`(\w+(?:\.\w+)*)\s*\(`)
	awaitPattern := regexp.MustCompile(`\bawait\s+(\w+(?:\.\w+)*)\s*\(`)

	for lineNum := startLine - 1; lineNum < len(lines); lineNum++ {
		line := lines[lineNum]

		// Detect function start and indent
		if lineNum == startLine-1 {
			inFunction = true
			// Find the indentation of the function definition
			match := regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+`).FindStringSubmatch(line)
			if match != nil {
				funcIndent = len(match[1])
			}
			continue
		}

		if !inFunction {
			continue
		}

		// Check if we've exited the function (less or equal indent on non-empty line)
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if currentIndent <= funcIndent && lineNum > startLine-1 {
				break
			}
		}

		// Find function calls
		callMatches := callPattern.FindAllStringSubmatchIndex(line, -1)
		for _, match := range callMatches {
			callName := line[match[2]:match[3]]

			// Skip keywords and built-in statements
			if isKeyword(callName) {
				continue
			}

			isAsync := false
			// Check if this is an awaited call
			if awaitPattern.MatchString(line) {
				isAsync = true
			}

			call := types.FunctionCall{
				Target:   types.NewCompositeSymbolID(0, 0),
				CallType: pa.determineCallType(callName),
				Location: types.SymbolLocation{
					FileID: symbol.Identity.ID.FileID,
					Line:   lineNum + 1,
					Column: match[2] + 1,
				},
				Context:     callName,
				IsAsync:     isAsync,
				IsRecursive: callName == symbol.Identity.Name,
			}
			calls = append(calls, call)
		}
	}

	return calls, nil
}

// Helper methods

func (pa *PythonAnalyzer) createClassSymbol(fileID types.FileID, localID uint32, name string, line int, bases string) *types.UniversalSymbolNode {
	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name,
			FullName: name,
			Kind:     types.SymbolKindClass,
			Language: "python",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   line,
				Column: 1,
			},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     pa.determineAccessLevel(name),
			IsExported: !strings.HasPrefix(name, "_"),
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}

	return symbol
}

func (pa *PythonAnalyzer) createFunctionSymbol(fileID types.FileID, localID uint32, name string, line int, isAsync, isMethod bool, params, returnType string) *types.UniversalSymbolNode {
	kind := types.SymbolKindFunction
	if isMethod {
		kind = types.SymbolKindMethod
	}

	// Build signature
	signature := name + "(" + params + ")"
	if returnType != "" {
		signature += " -> " + returnType
	}

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name,
			FullName: name,
			Kind:     kind,
			Language: "python",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   line,
				Column: 1,
			},
			Signature: signature,
			Type:      returnType,
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     pa.determineAccessLevel(name),
			IsExported: !strings.HasPrefix(name, "_"),
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}

	// Add async attribute
	if isAsync {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type:  types.AttrTypeAsync,
			Value: "async",
			Line:  line,
		})
	}

	return symbol
}

func (pa *PythonAnalyzer) determineAccessLevel(name string) types.AccessLevel {
	// Dunder methods (__init__, __str__, etc.) are public
	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		return types.AccessPublic
	}
	// Name mangling (double underscore prefix without suffix)
	if strings.HasPrefix(name, "__") {
		return types.AccessPrivate
	}
	// Single underscore prefix is protected by convention
	if strings.HasPrefix(name, "_") {
		return types.AccessProtected
	}
	return types.AccessPublic
}

func (pa *PythonAnalyzer) determineCallType(callName string) types.CallType {
	if strings.Contains(callName, ".") {
		return types.CallMethod
	}
	return types.CallDirect
}

func isKeyword(name string) bool {
	keywords := map[string]bool{
		"if": true, "elif": true, "else": true, "for": true, "while": true,
		"try": true, "except": true, "finally": true, "with": true, "as": true,
		"import": true, "from": true, "class": true, "def": true, "return": true,
		"yield": true, "raise": true, "assert": true, "pass": true, "break": true,
		"continue": true, "del": true, "in": true, "not": true, "and": true,
		"or": true, "is": true, "lambda": true, "global": true, "nonlocal": true,
		"True": true, "False": true, "None": true, "async": true, "await": true,
		"print": true, "len": true, "range": true, "str": true, "int": true,
		"float": true, "list": true, "dict": true, "set": true, "tuple": true,
		"type": true, "isinstance": true, "issubclass": true, "super": true,
		"self": true, "cls": true,
	}
	// Just check the base name for method calls
	baseName := name
	if idx := strings.LastIndex(name, "."); idx != -1 {
		baseName = name[idx+1:]
	}
	return keywords[baseName]
}
