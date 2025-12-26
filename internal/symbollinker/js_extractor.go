package symbollinker

import (
	"errors"
	"strings"

	"github.com/standardbeagle/lci/internal/types"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// JSExtractor extracts symbols from JavaScript and TypeScript source code
type JSExtractor struct {
	*BaseExtractor
	isTypeScript bool
}

// NewJSExtractor creates a new JavaScript symbol extractor
func NewJSExtractor() *JSExtractor {
	return &JSExtractor{
		BaseExtractor: NewBaseExtractor("javascript", []string{".js", ".jsx", ".mjs", ".cjs"}),
		isTypeScript:  false,
	}
}

// NewTSExtractor creates a new TypeScript symbol extractor
func NewTSExtractor() *JSExtractor {
	return &JSExtractor{
		BaseExtractor: NewBaseExtractor("typescript", []string{".ts", ".tsx", ".mts", ".cts"}),
		isTypeScript:  true,
	}
}

// ExtractSymbols extracts all symbols from a JavaScript/TypeScript AST
func (je *JSExtractor) ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error) {
	if tree == nil {
		return nil, errors.New("tree is nil")
	}

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("root node is nil")
	}

	builder := NewSymbolTableBuilder(fileID, je.language)
	scopeManager := NewScopeManager()

	// Extract imports and exports
	je.extractImports(root, content, builder, fileID)
	je.extractExports(root, content, builder, fileID)

	// Extract symbols with scope tracking
	je.extractSymbolsFromNode(root, content, builder, scopeManager, fileID)

	// Build and return the symbol table
	return builder.Build(), nil
}

// extractImports extracts all import statements
func (je *JSExtractor) extractImports(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) {
	je.traverseNode(root, func(node *sitter.Node) bool {
		nodeKind := node.Kind()

		switch nodeKind {
		case "import_statement":
			je.extractImportStatement(node, content, builder, fileID)

		case "import_clause":
			// Handled by import_statement

		case "require_call":
			// CommonJS require()
			je.extractRequireCall(node, content, builder, fileID)
		}

		return true // Continue traversal
	})
}

// extractImportStatement extracts ES6 import statements
func (je *JSExtractor) extractImportStatement(node *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) {
	imp := types.ImportInfo{
		Location: GetNodeLocation(node, fileID),
	}

	// Extract source (from clause)
	sourceNode := FindChildByType(node, "string")
	if sourceNode != nil {
		source := GetNodeText(sourceNode, content)
		if len(source) >= 2 {
			imp.ImportPath = source[1 : len(source)-1] // Remove quotes
		}
	}

	// Check if it's a type-only import (TypeScript)
	if je.isTypeScript {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "import" {
				// Check if next child is "type"
				if i+1 < node.ChildCount() {
					next := node.Child(i + 1)
					if next != nil && GetNodeText(next, content) == "type" {
						imp.IsTypeOnly = true
						break
					}
				}
			}
		}
	}

	// Process import clause
	importClause := FindChildByType(node, "import_clause")
	if importClause != nil {
		// Default import
		defaultImport := FindChildByType(importClause, "identifier")
		if defaultImport != nil {
			name := GetNodeText(defaultImport, content)
			imp.Alias = name
			imp.IsDefault = true
			builder.AddImport(imp)

			// If there are also named imports, create separate entries
			imp = types.ImportInfo{
				Location:   GetNodeLocation(node, fileID),
				ImportPath: imp.ImportPath,
				IsTypeOnly: imp.IsTypeOnly,
			}
		}

		// Named imports
		namedImports := FindChildByType(importClause, "named_imports")
		if namedImports != nil {
			var importedNames []string

			for i := uint(0); i < namedImports.ChildCount(); i++ {
				child := namedImports.Child(i)
				if child != nil && child.Kind() == "import_specifier" {
					// Check for aliased imports (import { foo as bar })
					nameNode := FindChildByType(child, "identifier")
					aliasNode := child.Child(child.ChildCount() - 1) // Last child is the alias

					if nameNode != nil {
						originalName := GetNodeText(nameNode, content)
						importedNames = append(importedNames, originalName)

						// Check if it's aliased
						if aliasNode != nil && aliasNode != nameNode {
							aliasName := GetNodeText(aliasNode, content)
							if aliasName != originalName && aliasName != "as" {
								// Create separate import entry for alias
								aliasImp := imp
								aliasImp.Alias = aliasName
								aliasImp.ImportedNames = []string{originalName}
								builder.AddImport(aliasImp)
							}
						}
					}
				} else if child != nil && child.Kind() == "identifier" {
					// Simple named import
					name := GetNodeText(child, content)
					if name != "as" { // Skip the "as" keyword
						importedNames = append(importedNames, name)
					}
				}
			}

			if len(importedNames) > 0 {
				imp.ImportedNames = importedNames
				builder.AddImport(imp)
			}
		}

		// Namespace import (import * as name)
		namespaceImport := FindChildByType(importClause, "namespace_import")
		if namespaceImport != nil {
			// Find the identifier after "as"
			for i := uint(0); i < namespaceImport.ChildCount(); i++ {
				child := namespaceImport.Child(i)
				if child != nil && child.Kind() == "identifier" {
					imp.Alias = GetNodeText(child, content)
					imp.IsNamespace = true
					builder.AddImport(imp)
					break
				}
			}
		}
	}
}

// extractRequireCall extracts CommonJS require() calls
func (je *JSExtractor) extractRequireCall(node *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) {
	// Look for the string argument to require()
	var source string
	je.traverseNode(node, func(child *sitter.Node) bool {
		if child.Kind() == "string" {
			text := GetNodeText(child, content)
			if len(text) >= 2 {
				source = text[1 : len(text)-1] // Remove quotes
			}
			return false // Stop traversal
		}
		return true
	})

	if source != "" {
		imp := types.ImportInfo{
			ImportPath: source,
			Location:   GetNodeLocation(node, fileID),
		}

		// Check if it's assigned to a variable
		parent := node.Parent()
		if parent != nil && parent.Kind() == "variable_declarator" {
			nameNode := FindChildByType(parent, "identifier")
			if nameNode != nil {
				imp.Alias = GetNodeText(nameNode, content)
			}
		}

		builder.AddImport(imp)
	}
}

// extractExports extracts all export statements
func (je *JSExtractor) extractExports(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) {
	je.traverseNode(root, func(node *sitter.Node) bool {
		nodeKind := node.Kind()

		switch nodeKind {
		case "export_statement":
			je.extractExportStatement(node, content, builder, fileID)

		case "export_default_declaration":
			je.extractDefaultExport(node, content, builder, fileID)

		case "export_specifier":
			// Handled by export_statement
		}

		return true
	})
}

// extractExportStatement extracts named exports
func (je *JSExtractor) extractExportStatement(node *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) {
	exp := types.ExportInfo{
		Location: GetNodeLocation(node, fileID),
	}

	// Check if it's a type-only export (TypeScript)
	if je.isTypeScript {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && GetNodeText(child, content) == "type" {
				exp.IsTypeOnly = true
				break
			}
		}
	}

	// Check for export specifiers (export { a, b })
	exportClause := FindChildByType(node, "export_clause")
	if exportClause != nil {
		for i := uint(0); i < exportClause.ChildCount(); i++ {
			child := exportClause.Child(i)
			if child != nil && child.Kind() == "export_specifier" {
				// Get local and exported names
				var localName, exportedName string

				identCount := 0
				for j := uint(0); j < child.ChildCount(); j++ {
					grandchild := child.Child(j)
					if grandchild != nil && grandchild.Kind() == "identifier" {
						name := GetNodeText(grandchild, content)
						if identCount == 0 {
							localName = name
							exportedName = name // Default to same name
						} else if name != "as" {
							exportedName = name // Name after "as"
						}
						identCount++
					}
				}

				if localName != "" {
					exp.LocalName = localName
					exp.ExportedName = exportedName
					builder.AddExport(exp)
				}
			}
		}

		// Check for re-export (export { ... } from '...')
		sourceNode := FindChildByType(node, "string")
		if sourceNode != nil {
			source := GetNodeText(sourceNode, content)
			if len(source) >= 2 {
				exp.SourcePath = source[1 : len(source)-1]
				exp.IsReExport = true
			}
		}
	}

	// Check for declaration exports (export const, export function, etc.)
	declarationNode := FindChildByType(node, "lexical_declaration")
	if declarationNode == nil {
		declarationNode = FindChildByType(node, "function_declaration")
	}
	if declarationNode == nil {
		declarationNode = FindChildByType(node, "class_declaration")
	}

	if declarationNode != nil {
		// Extract the name of what's being exported
		nameNode := FindChildByType(declarationNode, "identifier")
		if nameNode != nil {
			exp.LocalName = GetNodeText(nameNode, content)
			exp.ExportedName = exp.LocalName
			builder.AddExport(exp)
		}
	}
}

// extractDefaultExport extracts default exports
func (je *JSExtractor) extractDefaultExport(node *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) {
	exp := types.ExportInfo{
		Location:     GetNodeLocation(node, fileID),
		IsDefault:    true,
		ExportedName: "default",
	}

	// Try to find what's being exported
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			switch child.Kind() {
			case "function_declaration", "class_declaration":
				nameNode := FindChildByType(child, "identifier")
				if nameNode != nil {
					exp.LocalName = GetNodeText(nameNode, content)
				}
			case "identifier":
				exp.LocalName = GetNodeText(child, content)
			}
		}
	}

	builder.AddExport(exp)
}

// extractSymbolsFromNode recursively extracts symbols from an AST node
func (je *JSExtractor) extractSymbolsFromNode(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()

	switch nodeKind {
	case "function_declaration", "function_expression":
		je.extractFunction(node, content, builder, scopeManager, fileID)

	case "arrow_function":
		je.extractArrowFunction(node, content, builder, scopeManager, fileID)

	case "class_declaration":
		je.extractClass(node, content, builder, scopeManager, fileID)

	case "method_definition":
		je.extractMethod(node, content, builder, scopeManager, fileID)

	case "variable_declaration", "lexical_declaration":
		je.extractVariableDeclaration(node, content, builder, scopeManager, fileID)

	case "interface_declaration": // TypeScript
		if je.isTypeScript {
			je.extractInterface(node, content, builder, scopeManager, fileID)
		}

	case "type_alias_declaration": // TypeScript
		if je.isTypeScript {
			je.extractTypeAlias(node, content, builder, scopeManager, fileID)
		}

	case "enum_declaration": // TypeScript
		if je.isTypeScript {
			je.extractEnum(node, content, builder, scopeManager, fileID)
		}

	case "block_statement", "statement_block":
		// Enter block scope
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, "", startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			je.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID)
		}

		// Exit block scope
		scopeManager.PopScope()
		return

	case "if_statement", "for_statement", "while_statement", "do_statement", "switch_statement":
		// These create new scopes
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, nodeKind, startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			je.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID)
		}

		scopeManager.PopScope()
		return
	}

	// Process children for other node types
	for i := uint(0); i < node.ChildCount(); i++ {
		je.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID)
	}
}

// extractFunction extracts function declarations and expressions
func (je *JSExtractor) extractFunction(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	funcName := "anonymous"

	if nameNode != nil {
		funcName = GetNodeText(nameNode, content)
	}

	location := GetNodeLocation(node, fileID)

	// Check if exported (has export keyword in parent)
	isExported := je.isExported(node, content)

	// Add function symbol
	localID := builder.AddSymbol(
		funcName,
		types.SymbolKindFunction,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Extract function signature
	signature := je.extractFunctionSignature(node, content)
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Signature = signature
	}

	// Enter function scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeFunction, funcName, startPos, endPos)

	// Extract parameters
	je.extractParameters(node, content, builder, scopeManager, fileID)

	// Process function body
	body := FindChildByType(node, "statement_block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			je.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID)
		}
	}

	// Exit function scope
	scopeManager.PopScope()
}

// extractArrowFunction extracts arrow function expressions
func (je *JSExtractor) extractArrowFunction(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Arrow functions are usually anonymous, but might be assigned to a variable
	funcName := "arrow"

	// Check if assigned to a variable
	parent := node.Parent()
	if parent != nil && parent.Kind() == "variable_declarator" {
		nameNode := FindChildByType(parent, "identifier")
		if nameNode != nil {
			funcName = GetNodeText(nameNode, content)
		}
	}

	location := GetNodeLocation(node, fileID)

	// Add function symbol
	localID := builder.AddSymbol(
		funcName,
		types.SymbolKindFunction,
		location,
		scopeManager.CurrentScope(),
		false, // Arrow functions are not exported by themselves
	)

	// Extract signature
	signature := je.extractArrowSignature(node, content)
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Signature = signature
	}

	// Enter function scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeFunction, funcName, startPos, endPos)

	// Extract parameters
	je.extractArrowParameters(node, content, builder, scopeManager, fileID)

	// Process function body (might be expression or block)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "statement_block" {
			for j := uint(0); j < child.ChildCount(); j++ {
				je.extractSymbolsFromNode(child.Child(j), content, builder, scopeManager, fileID)
			}
		}
	}

	// Exit function scope
	scopeManager.PopScope()
}

// extractClass extracts class declarations
func (je *JSExtractor) extractClass(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// In TypeScript, class names are type_identifier; in JavaScript, they are identifier
	nameNode := FindChildByType(node, "type_identifier")
	if nameNode == nil {
		nameNode = FindChildByType(node, "identifier")
	}
	if nameNode == nil {
		return
	}

	className := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Extract decorators (TypeScript only)
	decorators := je.extractDecorators(node, content)

	// Check if exported
	isExported := je.isExported(node, content)

	// Add class symbol
	localID := builder.AddSymbol(
		className,
		types.SymbolKindClass,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Attach decorators
	if len(decorators) > 0 {
		if symbol, ok := builder.symbols[localID]; ok {
			symbol.Attributes = decorators
		}
	}

	// Enter class scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, className, startPos, endPos)

	// Extract class body
	body := FindChildByType(node, "class_body")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			if child != nil {
				je.extractSymbolsFromNode(child, content, builder, scopeManager, fileID)
			}
		}
	}

	// Exit class scope
	scopeManager.PopScope()
}

// extractMethod extracts method definitions in classes
func (je *JSExtractor) extractMethod(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "property_identifier")
	if nameNode == nil {
		nameNode = FindChildByType(node, "identifier")
	}

	if nameNode == nil {
		return
	}

	methodName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Extract decorators (TypeScript only)
	decorators := je.extractDecorators(node, content)

	// Check for special methods
	isConstructor := methodName == "constructor"
	isStatic := false
	isPrivate := strings.HasPrefix(methodName, "#") || strings.HasPrefix(methodName, "_")

	// Check for static keyword
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && GetNodeText(child, content) == "static" {
			isStatic = true
			break
		}
	}

	// Get class name from scope
	className := ""
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
	}

	fullName := methodName
	if className != "" {
		fullName = className + "." + methodName
	}

	// Add method symbol
	kind := types.SymbolKindMethod
	if isConstructor {
		kind = types.SymbolKindFunction // Constructor is special
	}

	localID := builder.AddSymbol(
		fullName,
		kind,
		location,
		scopeManager.CurrentScope(),
		!isPrivate, // Public unless explicitly private
	)

	// Add metadata and decorators
	if symbol, ok := builder.symbols[localID]; ok {
		if isStatic {
			symbol.Type = "static"
		}
		signature := je.extractFunctionSignature(node, content)
		symbol.Signature = signature

		// Attach decorators
		if len(decorators) > 0 {
			symbol.Attributes = decorators
		}
	}

	// Enter method scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeMethod, fullName, startPos, endPos)

	// Extract parameters
	je.extractParameters(node, content, builder, scopeManager, fileID)

	// Process method body
	body := FindChildByType(node, "statement_block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			je.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID)
		}
	}

	// Exit method scope
	scopeManager.PopScope()
}

// extractVariableDeclaration extracts variable/constant declarations
func (je *JSExtractor) extractVariableDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Get declaration kind (const, let, var)
	declarationKind := node.Kind()
	isConst := declarationKind == "lexical_declaration" && je.hasChildWithText(node, content, "const")

	// Process all variable declarators
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "variable_declarator" {
			je.extractVariableDeclarator(child, content, builder, scopeManager, fileID, isConst)
		}
	}
}

// extractVariableDeclarator extracts a single variable declarator
func (je *JSExtractor) extractVariableDeclarator(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, isConst bool) {
	// Get variable name
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		// Check for destructuring patterns
		nameNode = FindChildByType(node, "object_pattern")
		if nameNode == nil {
			nameNode = FindChildByType(node, "array_pattern")
		}
		if nameNode != nil {
			// Extract destructured variables
			je.extractDestructuredVariables(nameNode, content, builder, scopeManager, fileID, isConst)
			return
		}
		return
	}

	varName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Determine symbol kind
	kind := types.SymbolKindVariable
	if isConst {
		kind = types.SymbolKindConstant
	}

	// Check if exported
	isExported := je.isExported(node, content)

	// Add variable symbol
	builder.AddSymbol(
		varName,
		kind,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)
}

// extractDestructuredVariables extracts variables from destructuring patterns
func (je *JSExtractor) extractDestructuredVariables(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, isConst bool) {
	nodeKind := node.Kind()

	if nodeKind == "object_pattern" {
		// Extract object destructuring: { a, b: alias }
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil {
				if child.Kind() == "shorthand_property_identifier_pattern" || child.Kind() == "identifier" {
					varName := GetNodeText(child, content)
					if varName != "," && varName != "{" && varName != "}" {
						kind := types.SymbolKindVariable
						if isConst {
							kind = types.SymbolKindConstant
						}
						builder.AddSymbol(
							varName,
							kind,
							GetNodeLocation(child, fileID),
							scopeManager.CurrentScope(),
							false,
						)
					}
				} else if child.Kind() == "pair_pattern" {
					// Handle renamed destructuring { original: alias }
					for j := uint(0); j < child.ChildCount(); j++ {
						grandchild := child.Child(j)
						if grandchild != nil && grandchild.Kind() == "identifier" {
							// Use the second identifier (the alias)
							if j > 0 {
								varName := GetNodeText(grandchild, content)
								if varName != ":" {
									kind := types.SymbolKindVariable
									if isConst {
										kind = types.SymbolKindConstant
									}
									builder.AddSymbol(
										varName,
										kind,
										GetNodeLocation(grandchild, fileID),
										scopeManager.CurrentScope(),
										false,
									)
									break
								}
							}
						}
					}
				}
			}
		}
	} else if nodeKind == "array_pattern" {
		// Extract array destructuring: [a, b, ...rest]
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "identifier" {
				varName := GetNodeText(child, content)
				if varName != "," && varName != "[" && varName != "]" {
					kind := types.SymbolKindVariable
					if isConst {
						kind = types.SymbolKindConstant
					}
					builder.AddSymbol(
						varName,
						kind,
						GetNodeLocation(child, fileID),
						scopeManager.CurrentScope(),
						false,
					)
				}
			} else if child != nil && child.Kind() == "rest_pattern" {
				// Handle ...rest
				restIdent := FindChildByType(child, "identifier")
				if restIdent != nil {
					varName := GetNodeText(restIdent, content)
					kind := types.SymbolKindVariable
					if isConst {
						kind = types.SymbolKindConstant
					}
					builder.AddSymbol(
						varName,
						kind,
						GetNodeLocation(restIdent, fileID),
						scopeManager.CurrentScope(),
						false,
					)
				}
			}
		}
	}
}

// extractInterface extracts TypeScript interface declarations
func (je *JSExtractor) extractInterface(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "type_identifier")
	if nameNode == nil {
		return
	}

	interfaceName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Check if exported
	isExported := je.isExported(node, content)

	// Add interface symbol
	builder.AddSymbol(
		interfaceName,
		types.SymbolKindInterface,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Extract interface members
	body := FindChildByType(node, "object_type")
	if body != nil {
		je.extractInterfaceMembers(body, content, builder, scopeManager, fileID, interfaceName)
	}
}

// extractInterfaceMembers extracts members of a TypeScript interface
func (je *JSExtractor) extractInterfaceMembers(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, interfaceName string) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			switch child.Kind() {
			case "property_signature":
				// Extract property
				nameNode := FindChildByType(child, "property_identifier")
				if nameNode == nil {
					nameNode = FindChildByType(child, "identifier")
				}
				if nameNode != nil {
					propName := GetNodeText(nameNode, content)
					fullName := interfaceName + "." + propName
					builder.AddSymbol(
						fullName,
						types.SymbolKindProperty,
						GetNodeLocation(nameNode, fileID),
						scopeManager.CurrentScope(),
						false, // Interface members are not directly exported
					)
				}

			case "method_signature":
				// Extract method signature
				nameNode := FindChildByType(child, "property_identifier")
				if nameNode == nil {
					nameNode = FindChildByType(child, "identifier")
				}
				if nameNode != nil {
					methodName := GetNodeText(nameNode, content)
					fullName := interfaceName + "." + methodName
					localID := builder.AddSymbol(
						fullName,
						types.SymbolKindMethod,
						GetNodeLocation(nameNode, fileID),
						scopeManager.CurrentScope(),
						false,
					)

					// Extract signature
					signature := je.extractMethodSignature(child, content)
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Signature = signature
					}
				}
			}
		}
	}
}

// extractTypeAlias extracts TypeScript type alias declarations
func (je *JSExtractor) extractTypeAlias(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "type_identifier")
	if nameNode == nil {
		return
	}

	typeName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Check if exported
	isExported := je.isExported(node, content)

	// Add type alias symbol
	builder.AddSymbol(
		typeName,
		types.SymbolKindType,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)
}

// extractEnum extracts TypeScript enum declarations
func (je *JSExtractor) extractEnum(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	enumName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Check if exported
	isExported := je.isExported(node, content)

	// Add enum symbol
	builder.AddSymbol(
		enumName,
		types.SymbolKindEnum,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Extract enum members
	body := FindChildByType(node, "enum_body")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			if child != nil && child.Kind() == "enum_assignment" {
				memberNode := FindChildByType(child, "property_identifier")
				if memberNode == nil {
					memberNode = FindChildByType(child, "identifier")
				}
				if memberNode != nil {
					memberName := GetNodeText(memberNode, content)
					fullName := enumName + "." + memberName
					builder.AddSymbol(
						fullName,
						types.SymbolKindEnumMember,
						GetNodeLocation(memberNode, fileID),
						scopeManager.CurrentScope(),
						false, // Enum members are not directly exported
					)
				}
			}
		}
	}
}

// Helper methods

// isExported checks if a node is exported
func (je *JSExtractor) isExported(node *sitter.Node, content []byte) bool {
	parent := node.Parent()
	if parent != nil && (parent.Kind() == "export_statement" || parent.Kind() == "export_default_declaration") {
		return true
	}

	// Check if there's an export keyword before this node
	if parent != nil {
		for i := uint(0); i < parent.ChildCount(); i++ {
			child := parent.Child(i)
			if child == node {
				break
			}
			if child != nil && GetNodeText(child, content) == "export" {
				return true
			}
		}
	}

	return false
}

// hasChildWithText checks if a node has a child with specific text
func (je *JSExtractor) hasChildWithText(node *sitter.Node, content []byte, text string) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && GetNodeText(child, content) == text {
			return true
		}
	}
	return false
}

// extractParameters extracts function parameters
func (je *JSExtractor) extractParameters(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	params := FindChildByType(node, "formal_parameters")
	if params == nil {
		return
	}

	for i := uint(0); i < params.ChildCount(); i++ {
		child := params.Child(i)
		if child != nil {
			switch child.Kind() {
			case "identifier":
				paramName := GetNodeText(child, content)
				if paramName != "," && paramName != "(" && paramName != ")" {
					builder.AddSymbol(
						paramName,
						types.SymbolKindParameter,
						GetNodeLocation(child, fileID),
						scopeManager.CurrentScope(),
						false,
					)
				}

			case "required_parameter", "optional_parameter":
				// TypeScript parameters
				nameNode := FindChildByType(child, "identifier")
				if nameNode != nil {
					paramName := GetNodeText(nameNode, content)
					builder.AddSymbol(
						paramName,
						types.SymbolKindParameter,
						GetNodeLocation(nameNode, fileID),
						scopeManager.CurrentScope(),
						false,
					)
				}

			case "rest_pattern":
				// Rest parameters (...args)
				nameNode := FindChildByType(child, "identifier")
				if nameNode != nil {
					paramName := GetNodeText(nameNode, content)
					builder.AddSymbol(
						paramName,
						types.SymbolKindParameter,
						GetNodeLocation(nameNode, fileID),
						scopeManager.CurrentScope(),
						false,
					)
				}
			}
		}
	}
}

// extractArrowParameters extracts arrow function parameters
func (je *JSExtractor) extractArrowParameters(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Arrow functions can have different parameter patterns:
	// x => ...           (single parameter)
	// (x, y) => ...      (multiple parameters)
	// () => ...          (no parameters)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			if child.Kind() == "formal_parameters" {
				// Multiple parameters case
				je.extractParameters(node, content, builder, scopeManager, fileID)
				return
			} else if child.Kind() == "identifier" && i == 0 {
				// Single parameter case
				paramName := GetNodeText(child, content)
				builder.AddSymbol(
					paramName,
					types.SymbolKindParameter,
					GetNodeLocation(child, fileID),
					scopeManager.CurrentScope(),
					false,
				)
				return
			}
		}
	}
}

// extractFunctionSignature extracts a function's signature
func (je *JSExtractor) extractFunctionSignature(node *sitter.Node, content []byte) string {
	var signature strings.Builder

	// Get parameters
	params := FindChildByType(node, "formal_parameters")
	if params != nil {
		signature.WriteString(GetNodeText(params, content))
	} else {
		signature.WriteString("()")
	}

	// Get return type for TypeScript
	if je.isTypeScript {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "type_annotation" {
				signature.WriteString(": ")
				signature.WriteString(GetNodeText(child, content))
				break
			}
		}
	}

	return signature.String()
}

// extractArrowSignature extracts an arrow function's signature
func (je *JSExtractor) extractArrowSignature(node *sitter.Node, content []byte) string {
	var signature strings.Builder

	// Get parameters
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			if child.Kind() == "formal_parameters" {
				signature.WriteString(GetNodeText(child, content))
				break
			} else if child.Kind() == "identifier" && i == 0 {
				signature.WriteString("(")
				signature.WriteString(GetNodeText(child, content))
				signature.WriteString(")")
				break
			}
		}
	}

	if signature.Len() == 0 {
		signature.WriteString("()")
	}

	// Get return type for TypeScript
	if je.isTypeScript {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "type_annotation" {
				signature.WriteString(": ")
				signature.WriteString(GetNodeText(child, content))
				break
			}
		}
	}

	return signature.String()
}

// extractMethodSignature extracts a method's signature
func (je *JSExtractor) extractMethodSignature(node *sitter.Node, content []byte) string {
	// Similar to function signature but for methods
	return je.extractFunctionSignature(node, content)
}

// extractDecorators extracts TypeScript decorators from a declaration node
// Decorators appear as siblings before the declaration in TypeScript AST
func (je *JSExtractor) extractDecorators(declarationNode *sitter.Node, content []byte) []types.ContextAttribute {
	var decorators []types.ContextAttribute

	// Only process decorators for TypeScript
	if !je.isTypeScript {
		return decorators
	}

	parent := declarationNode.Parent()
	if parent == nil {
		return decorators
	}

	// Look for decorator nodes that precede this declaration
	for i := uint(0); i < parent.ChildCount(); i++ {
		child := parent.Child(i)
		if child == nil {
			continue
		}

		// Stop when we reach the declaration itself
		if child == declarationNode {
			break
		}

		// Process decorator nodes
		if child.Kind() == "decorator" {
			decorator := je.extractSingleDecorator(child, content)
			if decorator.Value != "" {
				decorators = append(decorators, decorator)
			}
		}
	}

	return decorators
}

// extractSingleDecorator extracts a single decorator
func (je *JSExtractor) extractSingleDecorator(decoratorNode *sitter.Node, content []byte) types.ContextAttribute {
	// Get the full decorator text including @ symbol and arguments
	decoratorText := GetNodeText(decoratorNode, content)

	// Remove the leading @ symbol if present
	if strings.HasPrefix(decoratorText, "@") {
		decoratorText = decoratorText[1:]
	}

	// Try to extract just the decorator name without arguments for simpler cases
	// e.g., "@Component({...})" -> "Component" or keep full text
	decoratorName := decoratorText

	// Look for call expression within decorator
	callExpr := FindChildByType(decoratorNode, "call_expression")
	if callExpr != nil {
		// Extract just the function/identifier name
		identifier := FindChildByType(callExpr, "identifier")
		if identifier == nil {
			identifier = FindChildByType(callExpr, "member_expression")
		}
		if identifier != nil {
			decoratorName = GetNodeText(identifier, content)
		}
	}

	return types.ContextAttribute{
		Type:  types.AttrTypeDecorator,
		Value: decoratorName,
	}
}

// traverseNode traverses the AST with a visitor function
func (je *JSExtractor) traverseNode(node *sitter.Node, visitor func(*sitter.Node) bool) {
	if node == nil || !visitor(node) {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		je.traverseNode(node.Child(i), visitor)
	}
}
