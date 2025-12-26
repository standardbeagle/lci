package analysis

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// JavaScriptAnalyzer implements language-specific analysis for JavaScript/TypeScript code
// This is a simplified implementation using regex patterns - a production version would use a proper AST parser
type JavaScriptAnalyzer struct {
	// Regex patterns for different language constructs
	functionPattern   *regexp.Regexp
	classPattern      *regexp.Regexp
	methodPattern     *regexp.Regexp
	variablePattern   *regexp.Regexp
	importPattern     *regexp.Regexp
	exportPattern     *regexp.Regexp
	callPattern       *regexp.Regexp
	extendsPattern    *regexp.Regexp
	implementsPattern *regexp.Regexp
}

// NewJavaScriptAnalyzer creates a new JavaScript analyzer
func NewJavaScriptAnalyzer() *JavaScriptAnalyzer {
	return &JavaScriptAnalyzer{
		functionPattern:   regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`),
		classPattern:      regexp.MustCompile(`(?m)^\s*(?:export\s+)?class\s+(\w+)(?:\s+extends\s+([^\s{]+))?(?:\s+implements\s+([^{]+))?\s*\{`),
		methodPattern:     regexp.MustCompile(`(?m)^\s*(?:async\s+)?(\w+)\s*\([^)]*\)\s*\{`),
		variablePattern:   regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)`),
		importPattern:     regexp.MustCompile(`(?m)^\s*import\s+(?:(?:\{([^}]+)\})|(?:(\w+))|(?:\*\s+as\s+(\w+)))\s+from\s+['"]([^'"]+)['"]`),
		exportPattern:     regexp.MustCompile(`(?m)^\s*export\s+(?:(?:default\s+)?(?:class|function|const|let|var)\s+(\w+)|(?:\{([^}]+)\}))`),
		callPattern:       regexp.MustCompile(`(\w+)\s*\(`),
		extendsPattern:    regexp.MustCompile(`class\s+\w+\s+extends\s+(\w+)`),
		implementsPattern: regexp.MustCompile(`class\s+\w+(?:\s+extends\s+\w+)?\s+implements\s+([^{]+)`),
	}
}

// GetLanguageName returns the language name
func (jsa *JavaScriptAnalyzer) GetLanguageName() string {
	return "javascript"
}

// ExtractSymbols extracts all symbols from JavaScript/TypeScript code
func (jsa *JavaScriptAnalyzer) ExtractSymbols(fileID types.FileID, content, filePath string) ([]*types.UniversalSymbolNode, error) {
	var symbols []*types.UniversalSymbolNode
	localSymbolID := uint32(1)

	lines := strings.Split(content, "\n")

	// Extract functions
	functionMatches := jsa.functionPattern.FindAllStringSubmatch(content, -1)
	for _, match := range functionMatches {
		if len(match) > 1 {
			functionName := match[1]
			lineNum := jsa.findLineNumber(lines, functionName, "function")
			symbol := jsa.createFunctionSymbol(fileID, localSymbolID, functionName, lineNum, false)
			symbols = append(symbols, symbol)
			localSymbolID++
		}
	}

	// Extract classes
	classMatches := jsa.classPattern.FindAllStringSubmatch(content, -1)
	for _, match := range classMatches {
		if len(match) >= 2 && match[1] != "" {
			className := match[1]
			lineNum := jsa.findLineNumber(lines, className, "class")
			symbol := jsa.createClassSymbol(fileID, localSymbolID, className, lineNum)

			// Extract extends relationship if present
			if len(match) > 2 && match[2] != "" {
				extendsTarget := types.NewCompositeSymbolID(fileID, 0) // Placeholder
				symbol.Relationships.Extends = []types.CompositeSymbolID{extendsTarget}
			}

			// Extract implements relationships if present
			if len(match) > 3 && match[3] != "" {
				interfaces := strings.Split(strings.TrimSpace(match[3]), ",")
				for _, iface := range interfaces {
					iface = strings.TrimSpace(iface)
					if iface != "" {
						implementsTarget := types.NewCompositeSymbolID(fileID, 0) // Placeholder
						symbol.Relationships.Implements = append(symbol.Relationships.Implements, implementsTarget)
					}
				}
			}

			symbols = append(symbols, symbol)
			localSymbolID++
		}
	}

	// Extract variables/constants
	variableMatches := jsa.variablePattern.FindAllStringSubmatch(content, -1)
	for _, match := range variableMatches {
		if len(match) > 1 {
			variableName := match[1]
			lineNum := jsa.findLineNumber(lines, variableName, "var")
			symbol := jsa.createVariableSymbol(fileID, localSymbolID, variableName, lineNum)
			symbols = append(symbols, symbol)
			localSymbolID++
		}
	}

	return symbols, nil
}

// AnalyzeExtends analyzes inheritance relationships in JavaScript/TypeScript
func (jsa *JavaScriptAnalyzer) AnalyzeExtends(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	var extends []types.CompositeSymbolID

	// Find extends relationships for this specific symbol
	pattern := regexp.MustCompile(fmt.Sprintf(`class\s+%s\s+extends\s+(\w+)`, regexp.QuoteMeta(symbol.Identity.Name)))
	matches := pattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			// Create placeholder CompositeSymbolID - would be resolved by symbol linker
			extends = append(extends, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, 0))
		}
	}

	return extends, nil
}

// AnalyzeImplements analyzes interface implementation relationships
func (jsa *JavaScriptAnalyzer) AnalyzeImplements(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	var implements []types.CompositeSymbolID

	// Find implements relationships for this specific symbol (TypeScript)
	pattern := regexp.MustCompile(fmt.Sprintf(`class\s+%s(?:\s+extends\s+\w+)?\s+implements\s+([^{]+)`, regexp.QuoteMeta(symbol.Identity.Name)))
	matches := pattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			interfaces := strings.Split(strings.TrimSpace(match[1]), ",")
			for _, iface := range interfaces {
				iface = strings.TrimSpace(iface)
				if iface != "" {
					// Create placeholder CompositeSymbolID - would be resolved by symbol linker
					implements = append(implements, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, 0))
				}
			}
		}
	}

	return implements, nil
}

// AnalyzeContains analyzes containment relationships (methods in classes, etc.)
func (jsa *JavaScriptAnalyzer) AnalyzeContains(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	var contains []types.CompositeSymbolID

	// For classes, find methods within the class
	if symbol.Identity.Kind == types.SymbolKindClass {
		// This is a simplified approach - a real implementation would parse the class body properly
		classPattern := regexp.MustCompile(fmt.Sprintf(`class\s+%s[^{]*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`, regexp.QuoteMeta(symbol.Identity.Name)))
		classMatches := classPattern.FindStringSubmatch(content)

		if len(classMatches) > 1 {
			classBody := classMatches[1]
			methodMatches := jsa.methodPattern.FindAllStringSubmatch(classBody, -1)

			for _, methodMatch := range methodMatches {
				if len(methodMatch) > 1 {
					// Create placeholder CompositeSymbolID for method
					contains = append(contains, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, uint32(len(contains)+1000)))
				}
			}
		}
	}

	return contains, nil
}

// AnalyzeDependencies analyzes dependency relationships
func (jsa *JavaScriptAnalyzer) AnalyzeDependencies(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.SymbolDependency, error) {
	var dependencies []types.SymbolDependency

	// Extract import dependencies
	importMatches := jsa.importPattern.FindAllStringSubmatch(content, -1)
	for _, match := range importMatches {
		if len(match) > 4 {
			importPath := match[4]
			dependency := types.SymbolDependency{
				Target:        types.NewCompositeSymbolID(0, 0), // Would be resolved by symbol linker
				Type:          types.DependencyImport,
				Strength:      types.DependencyModerate,
				Context:       "import",
				ImportPath:    importPath,
				IsOptional:    false,
				IsConditional: false,
			}
			dependencies = append(dependencies, dependency)
		}
	}

	return dependencies, nil
}

// AnalyzeCalls analyzes function call relationships
func (jsa *JavaScriptAnalyzer) AnalyzeCalls(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.FunctionCall, error) {
	var calls []types.FunctionCall

	// Find function calls within the symbol's scope
	// This is a simplified approach - real implementation would need proper scope analysis
	lines := strings.Split(content, "\n")
	startLine := symbol.Identity.Location.Line

	// Simple heuristic: look for calls in the next 50 lines (assuming average function length)
	endLine := startLine + 50
	if endLine > len(lines) {
		endLine = len(lines)
	}

	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		if lineNum-1 < len(lines) {
			line := lines[lineNum-1]
			callMatches := jsa.callPattern.FindAllStringSubmatch(line, -1)

			for _, match := range callMatches {
				if len(match) > 1 {
					functionName := match[1]
					call := types.FunctionCall{
						Target:   types.NewCompositeSymbolID(0, 0), // Would be resolved by symbol linker
						CallType: types.CallDirect,
						Location: types.SymbolLocation{
							FileID: symbol.Identity.ID.FileID,
							Line:   lineNum,
							Column: 1, // Simplified
						},
						Context:     functionName,
						IsAsync:     strings.Contains(line, "await"),
						IsRecursive: functionName == symbol.Identity.Name,
						Arguments:   []types.CallArgument{}, // Simplified
					}
					calls = append(calls, call)
				}
			}
		}
	}

	return calls, nil
}

// Helper methods for creating symbols

func (jsa *JavaScriptAnalyzer) createFunctionSymbol(fileID types.FileID, localID uint32, name string, line int, isAsync bool) *types.UniversalSymbolNode {
	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name,
			FullName: name,
			Kind:     types.SymbolKindFunction,
			Language: "javascript",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   line,
				Column: 1,
			},
			Signature: name + "()",
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPublic,
			IsExported: true, // Simplified assumption
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{
			Attributes: jsa.createAttributes(isAsync),
		},
	}

	return symbol
}

func (jsa *JavaScriptAnalyzer) createClassSymbol(fileID types.FileID, localID uint32, name string, line int) *types.UniversalSymbolNode {
	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name,
			FullName: name,
			Kind:     types.SymbolKindClass,
			Language: "javascript",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   line,
				Column: 1,
			},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPublic,
			IsExported: true, // Simplified assumption
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

func (jsa *JavaScriptAnalyzer) createVariableSymbol(fileID types.FileID, localID uint32, name string, line int) *types.UniversalSymbolNode {
	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name,
			FullName: name,
			Kind:     types.SymbolKindVariable,
			Language: "javascript",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   line,
				Column: 1,
			},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPublic,
			IsExported: false, // Simplified assumption
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

// Helper utility methods

func (jsa *JavaScriptAnalyzer) findLineNumber(lines []string, symbolName, context string) int {
	for i, line := range lines {
		if strings.Contains(line, symbolName) && strings.Contains(line, context) {
			return i + 1
		}
	}
	return 1 // Default to line 1 if not found
}

func (jsa *JavaScriptAnalyzer) createAttributes(isAsync bool) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	if isAsync {
		attributes = append(attributes, types.ContextAttribute{
			Type:  types.AttrTypeAsync,
			Value: "async",
		})
	}

	return attributes
}
