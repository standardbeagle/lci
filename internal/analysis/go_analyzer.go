package analysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// GoAnalyzer implements language-specific analysis for Go code
type GoAnalyzer struct {
	fileSet *token.FileSet
}

// NewGoAnalyzer creates a new Go analyzer
func NewGoAnalyzer() *GoAnalyzer {
	return &GoAnalyzer{
		fileSet: token.NewFileSet(),
	}
}

// GetLanguageName returns the language name
func (ga *GoAnalyzer) GetLanguageName() string {
	return "go"
}

// ExtractSymbols extracts all symbols from Go code
func (ga *GoAnalyzer) ExtractSymbols(fileID types.FileID, content, filePath string) ([]*types.UniversalSymbolNode, error) {
	// Parse Go source code
	astFile, err := parser.ParseFile(ga.fileSet, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	var symbols []*types.UniversalSymbolNode
	localSymbolID := uint32(1) // Start from 1

	// Extract package declaration
	if astFile.Name != nil {
		packageSymbol := ga.createPackageSymbol(fileID, localSymbolID, astFile.Name, astFile.Doc)
		symbols = append(symbols, packageSymbol)
		localSymbolID++
	}

	// Walk the AST to extract symbols
	ast.Inspect(astFile, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			symbol := ga.createFunctionSymbol(fileID, localSymbolID, n)
			symbols = append(symbols, symbol)
			localSymbolID++

		case *ast.GenDecl:
			// Handle type, const, var declarations
			for _, spec := range n.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					symbol := ga.createTypeSymbol(fileID, localSymbolID, s, n)
					symbols = append(symbols, symbol)
					localSymbolID++

				case *ast.ValueSpec:
					// Variables and constants
					for _, nameIdent := range s.Names {
						symbol := ga.createVariableSymbol(fileID, localSymbolID, nameIdent, s, n)
						symbols = append(symbols, symbol)
						localSymbolID++
					}
				}
			}
		}
		return true
	})

	return symbols, nil
}

// AnalyzeExtends analyzes Go inheritance (embedding) relationships
func (ga *GoAnalyzer) AnalyzeExtends(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	// Parse the file to analyze embedding relationships
	astFile, err := parser.ParseFile(ga.fileSet, filePath, content, 0)
	if err != nil {
		return nil, err
	}

	var extends []types.CompositeSymbolID

	// Find the symbol's AST node and analyze embedded types
	ast.Inspect(astFile, func(node ast.Node) bool {
		if typeSpec, ok := node.(*ast.TypeSpec); ok {
			if typeSpec.Name.Name == symbol.Identity.Name {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					// Analyze embedded fields (Go's form of inheritance)
					for _, field := range structType.Fields.List {
						if len(field.Names) == 0 { // Embedded field
							if embeddedType := ga.extractTypeFromExpr(field.Type); embeddedType != "" {
								// Create a placeholder CompositeSymbolID for the embedded type
								// This would need to be resolved by the symbol linker
								extends = append(extends, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, 0))
							}
						}
					}
				}
			}
		}
		return true
	})

	return extends, nil
}

// AnalyzeImplements analyzes Go interface implementation relationships
func (ga *GoAnalyzer) AnalyzeImplements(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	// In Go, interface implementation is implicit
	// This would require cross-file analysis to determine which interfaces a type implements
	// For now, return empty slice - this would be filled by cross-file analysis
	return []types.CompositeSymbolID{}, nil
}

// AnalyzeContains analyzes containment relationships (methods, fields, etc.)
func (ga *GoAnalyzer) AnalyzeContains(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	astFile, err := parser.ParseFile(ga.fileSet, filePath, content, 0)
	if err != nil {
		return nil, err
	}

	var contains []types.CompositeSymbolID

	// Find the symbol and its contained elements
	ast.Inspect(astFile, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.TypeSpec:
			if n.Name.Name == symbol.Identity.Name {
				switch typeDecl := n.Type.(type) {
				case *ast.StructType:
					// Struct fields
					for _, field := range typeDecl.Fields.List {
						for range field.Names {
							// Create CompositeSymbolID for field
							// This is a simplified approach - real implementation would track actual IDs
							contains = append(contains, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, uint32(len(contains)+100)))
						}
					}
				case *ast.InterfaceType:
					// Interface methods
					for _, method := range typeDecl.Methods.List {
						for range method.Names {
							contains = append(contains, types.NewCompositeSymbolID(symbol.Identity.ID.FileID, uint32(len(contains)+200)))
						}
					}
				}
			}
		}
		return true
	})

	return contains, nil
}

// AnalyzeDependencies analyzes dependency relationships
func (ga *GoAnalyzer) AnalyzeDependencies(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.SymbolDependency, error) {
	astFile, err := parser.ParseFile(ga.fileSet, filePath, content, 0)
	if err != nil {
		return nil, err
	}

	var dependencies []types.SymbolDependency

	// Analyze import dependencies
	for _, importSpec := range astFile.Imports {
		importPath := strings.Trim(importSpec.Path.Value, "\"")
		dependency := types.SymbolDependency{
			Target:     types.NewCompositeSymbolID(0, 0), // Would be resolved by symbol linker
			Type:       types.DependencyImport,
			Strength:   types.DependencyModerate,
			Context:    "import",
			ImportPath: importPath,
			IsOptional: false,
		}
		dependencies = append(dependencies, dependency)
	}

	return dependencies, nil
}

// AnalyzeCalls analyzes function call relationships
func (ga *GoAnalyzer) AnalyzeCalls(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.FunctionCall, error) {
	astFile, err := parser.ParseFile(ga.fileSet, filePath, content, 0)
	if err != nil {
		return nil, err
	}

	var calls []types.FunctionCall

	// Find the function and analyze its calls
	ast.Inspect(astFile, func(node ast.Node) bool {
		if funcDecl, ok := node.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == symbol.Identity.Name {
				// Analyze calls within this function
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					if callExpr, ok := n.(*ast.CallExpr); ok {
						call := ga.createFunctionCall(callExpr, symbol.Identity.ID.FileID)
						if call != nil {
							calls = append(calls, *call)
						}
					}
					return true
				})
			}
		}
		return true
	})

	return calls, nil
}

// Helper methods for creating symbols

func (ga *GoAnalyzer) createPackageSymbol(fileID types.FileID, localID uint32, name *ast.Ident, doc *ast.CommentGroup) *types.UniversalSymbolNode {
	pos := ga.fileSet.Position(name.Pos())

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name.Name,
			FullName: name.Name,
			Kind:     types.SymbolKindPackage,
			Language: "go",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   pos.Line,
				Column: pos.Column,
			},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPackage,
			IsExported: true,
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{
			Documentation: ga.extractDocumentation(doc),
		},
	}

	return symbol
}

func (ga *GoAnalyzer) createFunctionSymbol(fileID types.FileID, localID uint32, funcDecl *ast.FuncDecl) *types.UniversalSymbolNode {
	pos := ga.fileSet.Position(funcDecl.Name.Pos())

	// Determine if it's a method or function
	kind := types.SymbolKindFunction
	var receiverType string
	if funcDecl.Recv != nil {
		kind = types.SymbolKindMethod
		// Extract receiver type
		if len(funcDecl.Recv.List) > 0 {
			receiverType = ga.extractTypeFromExpr(funcDecl.Recv.List[0].Type)
		}
	}

	// Build full name
	fullName := funcDecl.Name.Name
	if receiverType != "" {
		fullName = receiverType + "." + funcDecl.Name.Name
	}

	// Extract function signature
	signature := ga.extractFunctionSignature(funcDecl)

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     funcDecl.Name.Name,
			FullName: fullName,
			Kind:     kind,
			Language: "go",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   pos.Line,
				Column: pos.Column,
			},
			Signature: signature,
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     ga.determineAccessLevel(funcDecl.Name.Name),
			IsExported: ga.isExported(funcDecl.Name.Name),
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{
			Documentation: ga.extractDocumentation(funcDecl.Doc),
		},
	}

	return symbol
}

func (ga *GoAnalyzer) createTypeSymbol(fileID types.FileID, localID uint32, typeSpec *ast.TypeSpec, genDecl *ast.GenDecl) *types.UniversalSymbolNode {
	pos := ga.fileSet.Position(typeSpec.Name.Pos())

	// Determine the kind of type
	kind := types.SymbolKindType
	switch typeSpec.Type.(type) {
	case *ast.StructType:
		kind = types.SymbolKindStruct
	case *ast.InterfaceType:
		kind = types.SymbolKindInterface
	}

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     typeSpec.Name.Name,
			FullName: typeSpec.Name.Name,
			Kind:     kind,
			Language: "go",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   pos.Line,
				Column: pos.Column,
			},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     ga.determineAccessLevel(typeSpec.Name.Name),
			IsExported: ga.isExported(typeSpec.Name.Name),
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{
			Documentation: ga.extractDocumentation(genDecl.Doc),
		},
	}

	return symbol
}

func (ga *GoAnalyzer) createVariableSymbol(fileID types.FileID, localID uint32, name *ast.Ident, valueSpec *ast.ValueSpec, genDecl *ast.GenDecl) *types.UniversalSymbolNode {
	pos := ga.fileSet.Position(name.Pos())

	// Determine if it's a constant or variable
	kind := types.SymbolKindVariable
	if genDecl.Tok == token.CONST {
		kind = types.SymbolKindConstant
	}

	// Extract type information
	var typeInfo string
	if valueSpec.Type != nil {
		typeInfo = ga.extractTypeFromExpr(valueSpec.Type)
	}

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(fileID, localID),
			Name:     name.Name,
			FullName: name.Name,
			Kind:     kind,
			Language: "go",
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   pos.Line,
				Column: pos.Column,
			},
			Type: typeInfo,
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     ga.determineAccessLevel(name.Name),
			IsExported: ga.isExported(name.Name),
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{
			Documentation: ga.extractDocumentation(genDecl.Doc),
		},
	}

	return symbol
}

func (ga *GoAnalyzer) createFunctionCall(callExpr *ast.CallExpr, fileID types.FileID) *types.FunctionCall {
	pos := ga.fileSet.Position(callExpr.Pos())

	// Extract function name from call expression
	var functionName string
	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		functionName = fun.Name
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			functionName = ident.Name + "." + fun.Sel.Name
		} else {
			functionName = fun.Sel.Name
		}
	default:
		functionName = "anonymous"
	}

	call := &types.FunctionCall{
		Target:   types.NewCompositeSymbolID(0, 0), // Would be resolved by symbol linker
		CallType: types.CallDirect,
		Location: types.SymbolLocation{
			FileID: fileID,
			Line:   pos.Line,
			Column: pos.Column,
		},
		Context:   functionName,
		IsAsync:   false, // Go doesn't have async/await
		Arguments: ga.extractCallArguments(callExpr.Args),
	}

	return call
}

// Helper utility methods

func (ga *GoAnalyzer) extractDocumentation(commentGroup *ast.CommentGroup) []string {
	if commentGroup == nil {
		return nil
	}

	var docs []string
	for _, comment := range commentGroup.List {
		// Remove comment markers
		text := strings.TrimPrefix(comment.Text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
		if text != "" {
			docs = append(docs, text)
		}
	}

	return docs
}

func (ga *GoAnalyzer) extractTypeFromExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + ga.extractTypeFromExpr(t.X)
	case *ast.ArrayType:
		return "[]" + ga.extractTypeFromExpr(t.Elt)
	case *ast.MapType:
		return "map[" + ga.extractTypeFromExpr(t.Key) + "]" + ga.extractTypeFromExpr(t.Value)
	case *ast.ChanType:
		return "chan " + ga.extractTypeFromExpr(t.Value)
	case *ast.FuncType:
		return "func"
	default:
		return "unknown"
	}
}

func (ga *GoAnalyzer) extractFunctionSignature(funcDecl *ast.FuncDecl) string {
	var sig strings.Builder

	sig.WriteString(funcDecl.Name.Name)
	sig.WriteString("(")

	// Parameters
	if funcDecl.Type.Params != nil {
		for i, param := range funcDecl.Type.Params.List {
			if i > 0 {
				sig.WriteString(", ")
			}

			// Parameter names
			if len(param.Names) > 0 {
				for j, name := range param.Names {
					if j > 0 {
						sig.WriteString(", ")
					}
					sig.WriteString(name.Name)
				}
				sig.WriteString(" ")
			}

			// Parameter type
			sig.WriteString(ga.extractTypeFromExpr(param.Type))
		}
	}

	sig.WriteString(")")

	// Return types
	if funcDecl.Type.Results != nil {
		if len(funcDecl.Type.Results.List) == 1 && len(funcDecl.Type.Results.List[0].Names) == 0 {
			// Single unnamed return type
			sig.WriteString(" ")
			sig.WriteString(ga.extractTypeFromExpr(funcDecl.Type.Results.List[0].Type))
		} else {
			// Multiple or named return types
			sig.WriteString(" (")
			for i, result := range funcDecl.Type.Results.List {
				if i > 0 {
					sig.WriteString(", ")
				}

				if len(result.Names) > 0 {
					for j, name := range result.Names {
						if j > 0 {
							sig.WriteString(", ")
						}
						sig.WriteString(name.Name)
					}
					sig.WriteString(" ")
				}

				sig.WriteString(ga.extractTypeFromExpr(result.Type))
			}
			sig.WriteString(")")
		}
	}

	return sig.String()
}

func (ga *GoAnalyzer) extractCallArguments(args []ast.Expr) []types.CallArgument {
	var callArgs []types.CallArgument

	for i, arg := range args {
		argName := fmt.Sprintf("arg%d", i)
		argType := "unknown"
		argValue := ""
		isLiteral := false

		switch a := arg.(type) {
		case *ast.BasicLit:
			isLiteral = true
			argValue = a.Value
			switch a.Kind {
			case token.STRING:
				argType = "string"
			case token.INT:
				argType = "int"
			case token.FLOAT:
				argType = "float"
			case token.CHAR:
				argType = "rune"
			}
		case *ast.Ident:
			argValue = a.Name
			argType = "identifier"
		}

		callArgs = append(callArgs, types.CallArgument{
			Name:      argName,
			Type:      argType,
			Value:     argValue,
			IsLiteral: isLiteral,
		})
	}

	return callArgs
}

func (ga *GoAnalyzer) isExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func (ga *GoAnalyzer) determineAccessLevel(name string) types.AccessLevel {
	if ga.isExported(name) {
		return types.AccessPublic
	}
	return types.AccessPrivate
}
