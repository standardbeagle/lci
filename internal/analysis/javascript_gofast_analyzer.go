package analysis

import (
	"strings"
	"time"

	"github.com/t14raptor/go-fast/ast"
	"github.com/t14raptor/go-fast/parser"

	"github.com/standardbeagle/lci/internal/types"
)

// JavaScriptGoFastAnalyzer implements language-specific analysis for JavaScript code
// Uses go-fAST for accurate AST-based parsing instead of regex patterns
type JavaScriptGoFastAnalyzer struct{}

// NewJavaScriptGoFastAnalyzer creates a new JavaScript analyzer using go-fAST
func NewJavaScriptGoFastAnalyzer() *JavaScriptGoFastAnalyzer {
	return &JavaScriptGoFastAnalyzer{}
}

// GetLanguageName returns the language name
func (jga *JavaScriptGoFastAnalyzer) GetLanguageName() string {
	return "javascript"
}

// ExtractSymbols extracts all symbols from JavaScript code using go-fAST
func (jga *JavaScriptGoFastAnalyzer) ExtractSymbols(fileID types.FileID, content, filePath string) ([]*types.UniversalSymbolNode, error) {
	program, err := parser.ParseFile(content)
	if err != nil {
		// go-fAST doesn't support ES6 modules or TypeScript
		// Return the error so hybrid analyzer can fall back to regex
		return nil, err
	}

	var symbols []*types.UniversalSymbolNode
	localSymbolID := uint32(1)

	// Create a visitor to extract symbols
	visitor := &symbolExtractor{
		fileID:        fileID,
		localSymbolID: &localSymbolID,
		symbols:       &symbols,
		content:       content,
	}

	// Walk the AST
	for _, stmt := range program.Body {
		jga.visitStatement(stmt.Stmt, visitor, nil)
	}

	return symbols, nil
}

// visitStatement visits a statement and extracts symbols
func (jga *JavaScriptGoFastAnalyzer) visitStatement(stmt ast.Stmt, visitor *symbolExtractor, parentClass *types.UniversalSymbolNode) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.FunctionDeclaration:
		if s.Function != nil && s.Function.Name != nil {
			symbol := visitor.createFunctionSymbol(
				s.Function.Name.Name,
				int(s.Function.Function),
				s.Function.Async,
				s.Function.Generator,
				false, // not a method
			)
			*visitor.symbols = append(*visitor.symbols, symbol)
			*visitor.localSymbolID++

			// Visit function body for nested symbols
			if s.Function.Body != nil {
				for _, bodyStmt := range s.Function.Body.List {
					jga.visitStatement(bodyStmt.Stmt, visitor, nil)
				}
			}
		}

	case *ast.ClassDeclaration:
		if s.Class != nil && s.Class.Name != nil {
			symbol := visitor.createClassSymbol(
				s.Class.Name.Name,
				int(s.Class.Class),
				s.Class.SuperClass,
			)
			*visitor.symbols = append(*visitor.symbols, symbol)
			*visitor.localSymbolID++

			// Visit class body for methods and fields
			for _, element := range s.Class.Body {
				jga.visitClassElement(element.Element, visitor, symbol)
			}
		}

	case *ast.VariableDeclaration:
		for _, decl := range s.List {
			if decl.Target != nil && decl.Target.Target != nil {
				name := jga.extractBindingName(decl.Target.Target)
				if name != "" {
					// Check if this is a function expression or arrow function
					if decl.Initializer != nil && decl.Initializer.Expr != nil {
						switch init := decl.Initializer.Expr.(type) {
						case *ast.FunctionLiteral:
							symbol := visitor.createFunctionSymbol(
								name,
								int(s.Idx),
								init.Async,
								init.Generator,
								false,
							)
							*visitor.symbols = append(*visitor.symbols, symbol)
							*visitor.localSymbolID++
							continue
						case *ast.ArrowFunctionLiteral:
							symbol := visitor.createFunctionSymbol(
								name,
								int(s.Idx),
								init.Async,
								false, // arrows can't be generators
								false,
							)
							*visitor.symbols = append(*visitor.symbols, symbol)
							*visitor.localSymbolID++
							continue
						}
					}
					// Regular variable
					symbol := visitor.createVariableSymbol(
						name,
						int(s.Idx),
						s.Token.String(),
					)
					*visitor.symbols = append(*visitor.symbols, symbol)
					*visitor.localSymbolID++
				}
			}
		}

	case *ast.BlockStatement:
		for _, bodyStmt := range s.List {
			jga.visitStatement(bodyStmt.Stmt, visitor, parentClass)
		}
	}
}

// visitClassElement visits a class element (method, field, etc.)
func (jga *JavaScriptGoFastAnalyzer) visitClassElement(element ast.Element, visitor *symbolExtractor, parentClass *types.UniversalSymbolNode) {
	if element == nil {
		return
	}

	switch e := element.(type) {
	case *ast.MethodDefinition:
		if e.Key != nil && e.Key.Expr != nil && e.Body != nil {
			name := jga.extractExpressionName(e.Key.Expr)
			if name != "" {
				symbol := visitor.createMethodSymbol(
					name,
					int(e.Idx),
					e.Body.Async,
					e.Body.Generator,
					e.Static,
					string(e.Kind),
				)

				// Set containment relationship
				symbol.Relationships.ContainedBy = &parentClass.Identity.ID
				parentClass.Relationships.Contains = append(parentClass.Relationships.Contains, symbol.Identity.ID)

				*visitor.symbols = append(*visitor.symbols, symbol)
				*visitor.localSymbolID++
			}
		}

	case *ast.FieldDefinition:
		if e.Key != nil && e.Key.Expr != nil {
			name := jga.extractExpressionName(e.Key.Expr)
			if name != "" {
				symbol := visitor.createFieldSymbol(
					name,
					int(e.Idx),
					e.Static,
				)

				// Set containment relationship
				symbol.Relationships.ContainedBy = &parentClass.Identity.ID
				parentClass.Relationships.Contains = append(parentClass.Relationships.Contains, symbol.Identity.ID)

				*visitor.symbols = append(*visitor.symbols, symbol)
				*visitor.localSymbolID++
			}
		}
	}
}

// extractBindingName extracts the name from a binding target
func (jga *JavaScriptGoFastAnalyzer) extractBindingName(target ast.Target) string {
	if target == nil {
		return ""
	}
	if ident, ok := target.(*ast.Identifier); ok {
		return ident.Name
	}
	return ""
}

// extractExpressionName extracts a name from an expression (for keys)
func (jga *JavaScriptGoFastAnalyzer) extractExpressionName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Name
	case *ast.PrivateIdentifier:
		if e.Identifier != nil {
			return "#" + e.Identifier.Name
		}
	case *ast.StringLiteral:
		return e.Value
	}
	return ""
}

// symbolExtractor is a helper for extracting symbols
type symbolExtractor struct {
	fileID        types.FileID
	localSymbolID *uint32
	symbols       *[]*types.UniversalSymbolNode
	content       string
}

func (se *symbolExtractor) getLineFromIdx(idx int) int {
	line := 1
	for i := 0; i < idx && i < len(se.content); i++ {
		if se.content[i] == '\n' {
			line++
		}
	}
	return line
}

func (se *symbolExtractor) createFunctionSymbol(name string, idx int, isAsync, isGenerator, isMethod bool) *types.UniversalSymbolNode {
	line := se.getLineFromIdx(idx)
	kind := types.SymbolKindFunction
	if isMethod {
		kind = types.SymbolKindMethod
	}

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:        types.NewCompositeSymbolID(se.fileID, *se.localSymbolID),
			Name:      name,
			FullName:  name,
			Kind:      kind,
			Language:  "javascript",
			Location:  types.SymbolLocation{FileID: se.fileID, Line: line, Column: 1},
			Signature: name + "()",
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPublic,
			IsExported: true,
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}

	// Add attributes
	if isAsync {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type:  types.AttrTypeAsync,
			Value: "async",
			Line:  line,
		})
	}
	if isGenerator {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type:  types.AttrTypeGenerator,
			Value: "generator",
			Line:  line,
		})
	}

	return symbol
}

func (se *symbolExtractor) createClassSymbol(name string, idx int, superClass *ast.Expression) *types.UniversalSymbolNode {
	line := se.getLineFromIdx(idx)

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(se.fileID, *se.localSymbolID),
			Name:     name,
			FullName: name,
			Kind:     types.SymbolKindClass,
			Language: "javascript",
			Location: types.SymbolLocation{FileID: se.fileID, Line: line, Column: 1},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPublic,
			IsExported: true,
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}

	// Add extends relationship if there's a superclass
	if superClass != nil && superClass.Expr != nil {
		symbol.Relationships.Extends = append(symbol.Relationships.Extends,
			types.NewCompositeSymbolID(se.fileID, 0)) // Placeholder, resolved by linker
	}

	return symbol
}

func (se *symbolExtractor) createMethodSymbol(name string, idx int, isAsync, isGenerator, isStatic bool, kind string) *types.UniversalSymbolNode {
	line := se.getLineFromIdx(idx)

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:        types.NewCompositeSymbolID(se.fileID, *se.localSymbolID),
			Name:      name,
			FullName:  name,
			Kind:      types.SymbolKindMethod,
			Language:  "javascript",
			Location:  types.SymbolLocation{FileID: se.fileID, Line: line, Column: 1},
			Signature: name + "()",
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     determineJSAccessLevel(name),
			IsExported: true,
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}

	// Add attributes
	if isAsync {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type: types.AttrTypeAsync, Value: "async", Line: line,
		})
	}
	if isGenerator {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type: types.AttrTypeGenerator, Value: "generator", Line: line,
		})
	}
	if isStatic {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type: types.AttrTypeStatic, Value: "static", Line: line,
		})
	}

	return symbol
}

func (se *symbolExtractor) createFieldSymbol(name string, idx int, isStatic bool) *types.UniversalSymbolNode {
	line := se.getLineFromIdx(idx)

	symbol := &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(se.fileID, *se.localSymbolID),
			Name:     name,
			FullName: name,
			Kind:     types.SymbolKindField,
			Language: "javascript",
			Location: types.SymbolLocation{FileID: se.fileID, Line: line, Column: 1},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     determineJSAccessLevel(name),
			IsExported: true,
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}

	if isStatic {
		symbol.Metadata.Attributes = append(symbol.Metadata.Attributes, types.ContextAttribute{
			Type: types.AttrTypeStatic, Value: "static", Line: line,
		})
	}

	return symbol
}

func (se *symbolExtractor) createVariableSymbol(name string, idx int, declarationType string) *types.UniversalSymbolNode {
	line := se.getLineFromIdx(idx)

	kind := types.SymbolKindVariable
	if declarationType == "const" {
		kind = types.SymbolKindConstant
	}

	return &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID:       types.NewCompositeSymbolID(se.fileID, *se.localSymbolID),
			Name:     name,
			FullName: name,
			Kind:     kind,
			Language: "javascript",
			Location: types.SymbolLocation{FileID: se.fileID, Line: line, Column: 1},
		},
		Relationships: types.SymbolRelationships{},
		Visibility: types.SymbolVisibility{
			Access:     types.AccessPublic,
			IsExported: false,
		},
		Usage: types.SymbolUsage{
			FirstSeen:      time.Now(),
			LastModified:   time.Now(),
			LastReferenced: time.Now(),
		},
		Metadata: types.SymbolMetadata{},
	}
}

func determineJSAccessLevel(name string) types.AccessLevel {
	if strings.HasPrefix(name, "#") {
		return types.AccessPrivate
	}
	if strings.HasPrefix(name, "_") {
		return types.AccessProtected // Convention
	}
	return types.AccessPublic
}

// AnalyzeExtends analyzes class inheritance
func (jga *JavaScriptGoFastAnalyzer) AnalyzeExtends(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	// Already populated during ExtractSymbols
	return symbol.Relationships.Extends, nil
}

// AnalyzeImplements analyzes interface implementation (JS doesn't have interfaces)
func (jga *JavaScriptGoFastAnalyzer) AnalyzeImplements(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	return nil, nil
}

// AnalyzeContains returns containment relationships
func (jga *JavaScriptGoFastAnalyzer) AnalyzeContains(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	return symbol.Relationships.Contains, nil
}

// AnalyzeDependencies analyzes import dependencies
// Note: go-fast may not support ES6 imports, so this returns empty for now
func (jga *JavaScriptGoFastAnalyzer) AnalyzeDependencies(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.SymbolDependency, error) {
	// go-fast focuses on ES5+ but may not fully support ES6 modules
	// For import/export analysis, consider using the regex-based analyzer as fallback
	return nil, nil
}

// AnalyzeCalls analyzes function calls
func (jga *JavaScriptGoFastAnalyzer) AnalyzeCalls(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.FunctionCall, error) {
	if symbol.Identity.Kind != types.SymbolKindFunction && symbol.Identity.Kind != types.SymbolKindMethod {
		return nil, nil
	}

	program, err := parser.ParseFile(content)
	if err != nil {
		return nil, err
	}

	var calls []types.FunctionCall
	callVisitor := &callExtractor{
		fileID:     symbol.Identity.ID.FileID,
		symbolName: symbol.Identity.Name,
		content:    content,
		calls:      &calls,
	}

	// Walk AST to find calls
	for _, stmt := range program.Body {
		if stmt.Stmt != nil {
			jga.visitStatementForCalls(stmt.Stmt, callVisitor)
		}
	}

	return calls, nil
}

func (jga *JavaScriptGoFastAnalyzer) visitStatementForCalls(stmt ast.Stmt, visitor *callExtractor) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		if s.Expression != nil && s.Expression.Expr != nil {
			jga.visitExpressionForCalls(s.Expression.Expr, visitor)
		}
	case *ast.BlockStatement:
		for _, bodyStmt := range s.List {
			if bodyStmt.Stmt != nil {
				jga.visitStatementForCalls(bodyStmt.Stmt, visitor)
			}
		}
	case *ast.FunctionDeclaration:
		if s.Function != nil && s.Function.Body != nil {
			for _, bodyStmt := range s.Function.Body.List {
				if bodyStmt.Stmt != nil {
					jga.visitStatementForCalls(bodyStmt.Stmt, visitor)
				}
			}
		}
	case *ast.ReturnStatement:
		if s.Argument != nil && s.Argument.Expr != nil {
			jga.visitExpressionForCalls(s.Argument.Expr, visitor)
		}
	case *ast.IfStatement:
		if s.Test != nil && s.Test.Expr != nil {
			jga.visitExpressionForCalls(s.Test.Expr, visitor)
		}
		if s.Consequent.Stmt != nil {
			jga.visitStatementForCalls(s.Consequent.Stmt, visitor)
		}
		if s.Alternate.Stmt != nil {
			jga.visitStatementForCalls(s.Alternate.Stmt, visitor)
		}
	}
}

func (jga *JavaScriptGoFastAnalyzer) visitExpressionForCalls(expr ast.Expr, visitor *callExtractor) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.CallExpression:
		name := jga.extractCalleeName(e.Callee)
		if name != "" {
			line := visitor.getLineFromIdx(int(e.LeftParenthesis))
			call := types.FunctionCall{
				Target:      types.NewCompositeSymbolID(0, 0),
				CallType:    types.CallDirect,
				Location:    types.SymbolLocation{FileID: visitor.fileID, Line: line, Column: 1},
				Context:     name,
				IsRecursive: name == visitor.symbolName,
			}
			*visitor.calls = append(*visitor.calls, call)
		}

		// Visit arguments
		for _, arg := range e.ArgumentList {
			if arg.Expr != nil {
				jga.visitExpressionForCalls(arg.Expr, visitor)
			}
		}

	case *ast.AwaitExpression:
		if e.Argument != nil && e.Argument.Expr != nil {
			// Mark the call as async if it's a CallExpression
			if call, ok := e.Argument.Expr.(*ast.CallExpression); ok {
				name := jga.extractCalleeName(call.Callee)
				if name != "" {
					line := visitor.getLineFromIdx(int(call.LeftParenthesis))
					asyncCall := types.FunctionCall{
						Target:      types.NewCompositeSymbolID(0, 0),
						CallType:    types.CallAsync,
						Location:    types.SymbolLocation{FileID: visitor.fileID, Line: line, Column: 1},
						Context:     name,
						IsAsync:     true,
						IsRecursive: name == visitor.symbolName,
					}
					*visitor.calls = append(*visitor.calls, asyncCall)
				}
			}
			jga.visitExpressionForCalls(e.Argument.Expr, visitor)
		}
	}
}

func (jga *JavaScriptGoFastAnalyzer) extractCalleeName(callee *ast.Expression) string {
	if callee == nil || callee.Expr == nil {
		return ""
	}
	switch c := callee.Expr.(type) {
	case *ast.Identifier:
		return c.Name
	case *ast.MemberExpression:
		if c.Property != nil && c.Property.Prop != nil {
			if ident, ok := c.Property.Prop.(*ast.Identifier); ok {
				return ident.Name
			}
		}
	}
	return ""
}

type callExtractor struct {
	fileID     types.FileID
	symbolName string
	content    string
	calls      *[]types.FunctionCall
}

func (ce *callExtractor) getLineFromIdx(idx int) int {
	line := 1
	for i := 0; i < idx && i < len(ce.content); i++ {
		if ce.content[i] == '\n' {
			line++
		}
	}
	return line
}
