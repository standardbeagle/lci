package symbollinker

import (
	"errors"
	"strings"

	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// GoExtractor extracts symbols from Go source code
type GoExtractor struct {
	*BaseExtractor
}

// NewGoExtractor creates a new Go symbol extractor
func NewGoExtractor() *GoExtractor {
	return &GoExtractor{
		BaseExtractor: NewBaseExtractor("go", []string{".go"}),
	}
}

// ExtractSymbols extracts all symbols from a Go AST
func (ge *GoExtractor) ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error) {
	if tree == nil {
		return nil, errors.New("tree is nil")
	}

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("root node is nil")
	}

	debug.Printf("GoExtractor.ExtractSymbols called, root has %d children\n", root.ChildCount())
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child != nil {
			debug.Printf("  Root child[%d]: kind=%s\n", i, child.Kind())
		}
	}

	builder := NewSymbolTableBuilder(fileID, "go")
	scopeManager := NewScopeManager()

	// Extract package name
	packageName := ge.extractPackageName(root, content)

	// Track imports for resolution
	imports := ge.extractImports(root, content, builder, fileID)

	// Add imports to the builder
	for _, imp := range imports {
		builder.AddImport(imp)
	}

	// Extract symbols with scope tracking
	ge.extractSymbolsFromNode(root, content, builder, scopeManager, fileID, packageName)

	// Build and return the symbol table
	table := builder.Build()

	return table, nil
}

// extractPackageName extracts the package name from the source file
func (ge *GoExtractor) extractPackageName(root *sitter.Node, content []byte) string {
	packageNode := FindChildByType(root, "package_clause")
	if packageNode == nil {
		return ""
	}

	packageIdent := FindChildByType(packageNode, "package_identifier")
	if packageIdent == nil {
		return ""
	}

	return GetNodeText(packageIdent, content)
}

// extractImports extracts all import statements
func (ge *GoExtractor) extractImports(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []types.ImportInfo {
	var imports []types.ImportInfo

	// Find all import declarations
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil || child.Kind() != "import_declaration" {
			continue
		}

		// Handle both single import and import groups
		specList := FindChildByType(child, "import_spec_list")
		if specList != nil {
			// Grouped imports: import ( "fmt"; "os" )
			for j := uint(0); j < specList.ChildCount(); j++ {
				spec := specList.Child(j)
				if spec != nil && spec.Kind() == "import_spec" {
					imp := ge.extractImportSpec(spec, content, fileID)
					if imp.ImportPath != "" {
						imports = append(imports, imp)
					}
				}
			}
		} else {
			// Single import: import "fmt"
			spec := FindChildByType(child, "import_spec")
			if spec != nil {
				imp := ge.extractImportSpec(spec, content, fileID)
				if imp.ImportPath != "" {
					imports = append(imports, imp)
				}
			}
		}
	}

	return imports
}

// extractImportSpec extracts a single import specification
func (ge *GoExtractor) extractImportSpec(spec *sitter.Node, content []byte, fileID types.FileID) types.ImportInfo {
	imp := types.ImportInfo{
		Location: GetNodeLocation(spec, fileID),
	}

	// Check for import alias
	for i := uint(0); i < spec.ChildCount(); i++ {
		child := spec.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "package_identifier", "blank_identifier":
			imp.Alias = GetNodeText(child, content)
		case "interpreted_string_literal":
			// Remove quotes from import path
			path := GetNodeText(child, content)
			if len(path) >= 2 {
				imp.ImportPath = path[1 : len(path)-1]
			}
		case "dot":
			imp.Alias = "."
		}
	}

	// If no alias, use the last part of the import path
	if imp.Alias == "" && imp.ImportPath != "" {
		parts := strings.Split(imp.ImportPath, "/")
		imp.Alias = parts[len(parts)-1]
	}

	return imp
}

// extractSymbolsFromNode recursively extracts symbols from an AST node
func (ge *GoExtractor) extractSymbolsFromNode(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, packageName string) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()
	// Debug: log all node types we encounter
	if nodeKind == "type_declaration" || strings.Contains(nodeKind, "type") {
		debug.Printf("Encountered node kind=%s\n", nodeKind)
	}

	switch nodeKind {
	case "function_declaration":
		ge.extractFunction(node, content, builder, scopeManager, fileID, false)

	case "method_declaration":
		ge.extractMethod(node, content, builder, scopeManager, fileID)

	case "type_declaration":
		ge.extractTypeDeclaration(node, content, builder, scopeManager, fileID)

	case "var_declaration", "const_declaration":
		ge.extractVariableDeclaration(node, content, builder, scopeManager, fileID, nodeKind == "const_declaration")

	case "short_var_declaration":
		ge.extractShortVarDeclaration(node, content, builder, scopeManager, fileID)

	case "go_statement":
		ge.extractGoroutine(node, content, builder, scopeManager, fileID)

	case "block":
		// Enter block scope
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, "", startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			ge.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, packageName)
		}

		// Exit block scope
		scopeManager.PopScope()
		return

	case "if_statement", "for_statement", "switch_statement":
		// These create new scopes
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, nodeKind, startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			ge.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, packageName)
		}

		scopeManager.PopScope()
		return
	}

	// Process children for other node types
	for i := uint(0); i < node.ChildCount(); i++ {
		ge.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, packageName)
	}
}

// extractFunction extracts a function declaration
func (ge *GoExtractor) extractFunction(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, isMethod bool) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	funcName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Determine if exported
	isExported := CommonVisibilityRules.GoCapitalization(funcName, node, content)

	// Extract type parameters if present
	typeParams := ge.extractTypeParameters(node, content)

	// Add function symbol
	kind := types.SymbolKindFunction
	if isMethod {
		kind = types.SymbolKindMethod
	}

	localID := builder.AddSymbol(
		funcName,
		kind,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Extract function signature and attach type parameters
	signature := ge.extractFunctionSignature(node, content)
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Signature = signature
		if len(typeParams) > 0 {
			symbol.TypeParameters = typeParams
		}
	}

	// Enter function scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeFunction, funcName, startPos, endPos)

	// Extract parameters
	ge.extractParameters(node, content, builder, scopeManager, fileID)

	// Process function body
	body := FindChildByType(node, "block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ge.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit function scope
	scopeManager.PopScope()
}

// extractMethod extracts a method declaration
func (ge *GoExtractor) extractMethod(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Extract receiver type
	receiverNode := FindChildByType(node, "parameter_list")
	var receiverType string
	if receiverNode != nil && receiverNode.ChildCount() > 0 {
		// Get the receiver parameter
		for i := uint(0); i < receiverNode.ChildCount(); i++ {
			param := receiverNode.Child(i)
			if param != nil && param.Kind() == "parameter_declaration" {
				typeNode := FindChildByType(param, "type_identifier")
				if typeNode == nil {
					// Check for pointer type
					ptrType := FindChildByType(param, "pointer_type")
					if ptrType != nil {
						typeNode = FindChildByType(ptrType, "type_identifier")
						if typeNode != nil {
							receiverType = "*" + GetNodeText(typeNode, content)
						}
					}
				} else {
					receiverType = GetNodeText(typeNode, content)
				}
				break
			}
		}
	}

	// Extract method name
	nameNode := FindChildByType(node, "field_identifier")
	if nameNode == nil {
		return
	}

	methodName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Methods follow the same export rules as functions
	isExported := CommonVisibilityRules.GoCapitalization(methodName, node, content)

	// Add method symbol
	// For consistent naming, remove pointer prefix from receiver type in the symbol name
	baseReceiverType := strings.TrimPrefix(receiverType, "*")

	fullName := methodName
	if baseReceiverType != "" {
		fullName = baseReceiverType + "." + methodName
	}

	localID := builder.AddSymbol(
		fullName,
		types.SymbolKindMethod,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Extract method signature
	signature := ge.extractFunctionSignature(node, content)
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Signature = signature
		symbol.Type = receiverType
	}

	// Enter method scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeMethod, fullName, startPos, endPos)

	// Extract parameters (skip receiver)
	paramsNode := FindChildByType(node, "parameter_list")
	if paramsNode != nil {
		// Skip first parameter list (receiver) and process the second
		foundReceiver := false
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "parameter_list" {
				if foundReceiver {
					ge.extractParametersFromList(child, content, builder, scopeManager, fileID)
					break
				}
				foundReceiver = true
			}
		}
	}

	// Process method body
	body := FindChildByType(node, "block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ge.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit method scope
	scopeManager.PopScope()
}

// extractTypeDeclaration extracts type declarations
func (ge *GoExtractor) extractTypeDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	debug.Printf("extractTypeDeclaration called for node kind=%s\n", node.Kind())

	// Handle both type_spec (type definitions) and type_alias (type aliases)
	specList := FindChildByType(node, "type_spec")
	if specList == nil {
		specList = FindChildByType(node, "type_alias")
	}
	if specList == nil {
		debug.Printf("No type_spec or type_alias found in type_declaration\n")
		return
	}
	debug.Printf("Found spec node kind=%s\n", specList.Kind())

	nameNode := FindChildByType(specList, "type_identifier")
	if nameNode == nil {
		return
	}

	typeName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)
	isExported := CommonVisibilityRules.GoCapitalization(typeName, node, content)

	// Extract type parameters if present
	typeParams := ge.extractTypeParameters(specList, content)

	// Determine type kind
	kind := types.SymbolKindType
	for i := uint(0); i < specList.ChildCount(); i++ {
		child := specList.Child(i)
		if child != nil {
			switch child.Kind() {
			case "struct_type":
				kind = types.SymbolKindStruct
				// Extract struct fields
				ge.extractStructFields(child, content, builder, scopeManager, fileID, typeName)
			case "interface_type":
				kind = types.SymbolKindInterface
				// Extract interface methods (pass type parameters so methods inherit them)
				ge.extractInterfaceMethods(child, content, builder, scopeManager, fileID, typeName, typeParams)
			}
		}
	}

	localID := builder.AddSymbol(
		typeName,
		kind,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Attach type parameters to the symbol
	if len(typeParams) > 0 {
		if symbol, ok := builder.symbols[localID]; ok {
			symbol.TypeParameters = typeParams
		}
	}

	debug.Printf("Added type symbol: name=%s, kind=%v, exported=%v, typeParams=%d\n", typeName, kind, isExported, len(typeParams))
}

// extractStructFields extracts fields from a struct type
func (ge *GoExtractor) extractStructFields(structNode *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, structName string) {
	fieldList := FindChildByType(structNode, "field_declaration_list")
	if fieldList == nil {
		return
	}

	for i := uint(0); i < fieldList.ChildCount(); i++ {
		field := fieldList.Child(i)
		if field != nil && field.Kind() == "field_declaration" {
			nameNode := FindChildByType(field, "field_identifier")
			if nameNode != nil {
				fieldName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)
				isExported := CommonVisibilityRules.GoCapitalization(fieldName, field, content)

				fullName := structName + "." + fieldName
				localID := builder.AddSymbol(
					fullName,
					types.SymbolKindField,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)

				// Extract field type
				typeNode := ge.findTypeNode(field)
				if typeNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Type = GetNodeText(typeNode, content)
					}
				}
			}
		}
	}
}

// extractInterfaceMethods extracts method signatures from an interface
func (ge *GoExtractor) extractInterfaceMethods(interfaceNode *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, interfaceName string, interfaceTypeParams []types.TypeParameter) {
	// Interface methods are direct children of interface_type as method_elem nodes
	for i := uint(0); i < interfaceNode.ChildCount(); i++ {
		method := interfaceNode.Child(i)
		if method != nil && method.Kind() == "method_elem" {
			nameNode := FindChildByType(method, "field_identifier")
			if nameNode != nil {
				methodName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				// Interface methods inherit export status from the method name itself, not the interface
				isExported := CommonVisibilityRules.GoCapitalization(methodName, method, content)

				fullName := interfaceName + "." + methodName
				localID := builder.AddSymbol(
					fullName,
					types.SymbolKindMethod,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)

				// Extract method signature and attach type parameters from interface
				signature := ge.extractMethodSignature(method, content)
				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Signature = signature
					// Methods inherit type parameters from their interface
					if len(interfaceTypeParams) > 0 {
						symbol.TypeParameters = interfaceTypeParams
					}
				}
			}
		}
	}
}

// extractVariableDeclaration extracts var and const declarations
func (ge *GoExtractor) extractVariableDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, isConstant bool) {
	// Handle both single and grouped declarations
	var specs []*sitter.Node

	// Look for spec_list first (grouped declarations)
	specList := FindChildByType(node, "var_spec_list")
	if specList == nil {
		specList = FindChildByType(node, "const_spec_list")
	}

	if specList != nil {
		// Grouped declaration with explicit spec list - collect all specs
		for i := uint(0); i < specList.ChildCount(); i++ {
			child := specList.Child(i)
			if child != nil && (child.Kind() == "var_spec" || child.Kind() == "const_spec") {
				specs = append(specs, child)
			}
		}
	} else {
		// Check if we have spec nodes as direct children (alternative AST structure)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && (child.Kind() == "var_spec" || child.Kind() == "const_spec") {
				specs = append(specs, child)
			}
		}

		// If still no specs, look for single declaration
		if len(specs) == 0 {
			spec := FindChildByType(node, "var_spec")
			if spec == nil {
				spec = FindChildByType(node, "const_spec")
			}
			if spec != nil {
				specs = append(specs, spec)
			}
		}
	}

	// Process each spec
	for _, spec := range specs {
		// Extract all identifiers in the spec
		for i := uint(0); i < spec.ChildCount(); i++ {
			child := spec.Child(i)
			if child != nil && child.Kind() == "identifier" {
				varName := GetNodeText(child, content)
				location := GetNodeLocation(child, fileID)
				isExported := CommonVisibilityRules.GoCapitalization(varName, node, content)

				kind := types.SymbolKindVariable
				if isConstant {
					kind = types.SymbolKindConstant
				}

				localID := builder.AddSymbol(
					varName,
					kind,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)

				// Try to extract type
				typeNode := ge.findTypeNode(spec)
				if typeNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Type = GetNodeText(typeNode, content)
					}
				}

				// For constants, try to extract value
				if isConstant {
					valueNode := FindChildByType(spec, "expression_list")
					if valueNode != nil && localID > 0 {
						if symbol, ok := builder.symbols[localID]; ok {
							symbol.Value = GetNodeText(valueNode, content)
						}
					}
				}
			}
		}
	}
}

// extractShortVarDeclaration extracts short variable declarations (x := value)
func (ge *GoExtractor) extractShortVarDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	leftSide := FindChildByType(node, "expression_list")
	if leftSide == nil {
		return
	}

	// Extract all identifiers on the left side
	for i := uint(0); i < leftSide.ChildCount(); i++ {
		child := leftSide.Child(i)
		if child != nil && child.Kind() == "identifier" {
			varName := GetNodeText(child, content)
			location := GetNodeLocation(child, fileID)

			// Short var declarations are never exported (local scope)
			builder.AddSymbol(
				varName,
				types.SymbolKindVariable,
				location,
				scopeManager.CurrentScope(),
				false,
			)
		}
	}
}

// extractGoroutine extracts goroutine launches
func (ge *GoExtractor) extractGoroutine(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Find the function being called
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "call_expression" {
			funcNode := FindChildByType(child, "identifier")
			if funcNode == nil {
				// Try selector expression for method calls
				selector := FindChildByType(child, "selector_expression")
				if selector != nil {
					funcNode = FindChildByType(selector, "field_identifier")
				}
			}

			if funcNode != nil {
				funcName := GetNodeText(funcNode, content)
				location := GetNodeLocation(node, fileID) // Use go statement location

				// Add a special goroutine launch symbol
				builder.AddSymbol(
					"go "+funcName,
					types.SymbolKindFunction,
					location,
					scopeManager.CurrentScope(),
					false, // Goroutines are not exported
				)
			}
		}
	}
}

// extractParameters extracts function parameters
func (ge *GoExtractor) extractParameters(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	paramList := FindChildByType(node, "parameter_list")
	if paramList != nil {
		ge.extractParametersFromList(paramList, content, builder, scopeManager, fileID)
	}
}

// extractParametersFromList extracts parameters from a parameter list
func (ge *GoExtractor) extractParametersFromList(paramList *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	for i := uint(0); i < paramList.ChildCount(); i++ {
		param := paramList.Child(i)
		if param != nil && (param.Kind() == "parameter_declaration" || param.Kind() == "variadic_parameter_declaration") {
			// Determine if this is variadic from the node type
			isVariadic := param.Kind() == "variadic_parameter_declaration"

			// Extract parameter names
			for j := uint(0); j < param.ChildCount(); j++ {
				child := param.Child(j)
				if child != nil && child.Kind() == "identifier" {
					paramName := GetNodeText(child, content)
					if isVariadic {
						paramName = "..." + paramName
					}
					location := GetNodeLocation(child, fileID)

					localID := builder.AddSymbol(
						paramName,
						types.SymbolKindParameter,
						location,
						scopeManager.CurrentScope(),
						false, // Parameters are not exported
					)

					// Extract parameter type
					typeNode := ge.findTypeNode(param)
					if typeNode != nil && localID > 0 {
						if symbol, ok := builder.symbols[localID]; ok {
							symbol.Type = GetNodeText(typeNode, content)
						}
					}
				}
			}
		}
	}
}

// extractFunctionSignature extracts the complete function signature
func (ge *GoExtractor) extractFunctionSignature(node *sitter.Node, content []byte) string {
	// Find the range from function name to the end of result type or parameter list
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		nameNode = FindChildByType(node, "field_identifier") // For methods
	}

	if nameNode == nil {
		return ""
	}

	startPos := nameNode.StartByte()
	endPos := startPos

	// Find the end position (either result type or parameter list)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			kind := child.Kind()
			if kind == "parameter_list" || kind == "result" || kind == "type_identifier" {
				if child.EndByte() > endPos {
					endPos = child.EndByte()
				}
			}
		}
	}

	if endPos > startPos && endPos <= uint(len(content)) {
		return string(content[startPos:endPos])
	}

	return ""
}

// extractMethodSignature extracts method signature from interface
func (ge *GoExtractor) extractMethodSignature(node *sitter.Node, content []byte) string {
	startPos := node.StartByte()
	endPos := node.EndByte()

	// Find the end of the signature (before any comments)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "comment" {
			endPos = child.StartByte()
			break
		}
	}

	if endPos > startPos && endPos <= uint(len(content)) {
		return strings.TrimSpace(string(content[startPos:endPos]))
	}

	return ""
}

// findTypeNode finds a type node in the given node's children
func (ge *GoExtractor) findTypeNode(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			kind := child.Kind()
			if kind == "type_identifier" || kind == "pointer_type" ||
				kind == "slice_type" || kind == "array_type" ||
				kind == "map_type" || kind == "channel_type" ||
				kind == "interface_type" || kind == "struct_type" {
				return child
			}
		}
	}

	return nil
}

// extractTypeParameters extracts generic type parameters from a node
// Handles syntax like [T any], [K comparable, V any], etc.
func (ge *GoExtractor) extractTypeParameters(node *sitter.Node, content []byte) []types.TypeParameter {
	var typeParams []types.TypeParameter

	// Look for type_parameter_list node
	typeParamList := FindChildByType(node, "type_parameter_list")
	if typeParamList == nil {
		return nil
	}

	// Iterate through type parameter declarations
	for i := uint(0); i < typeParamList.ChildCount(); i++ {
		child := typeParamList.Child(i)
		if child == nil || child.Kind() != "type_parameter_declaration" {
			continue
		}

		// Extract parameter name (can be multiple names: K, V any)
		var paramNames []string
		var constraint string

		for j := uint(0); j < child.ChildCount(); j++ {
			paramChild := child.Child(j)
			if paramChild == nil {
				continue
			}

			switch paramChild.Kind() {
			case "type_identifier":
				// This could be either a parameter name or the constraint
				text := GetNodeText(paramChild, content)
				// If we haven't found a constraint yet, this is a name
				// The last type_identifier is typically the constraint
				if constraint == "" {
					paramNames = append(paramNames, text)
				} else {
					// Already have constraint, so previous name was actually a constraint
					if len(paramNames) > 0 {
						constraint = paramNames[len(paramNames)-1]
						paramNames = paramNames[:len(paramNames)-1]
					}
					paramNames = append(paramNames, text)
				}
			case "interface_type", "struct_type":
				// Constraint is an interface or struct literal
				constraint = GetNodeText(paramChild, content)
			}
		}

		// The last identifier in paramNames is actually the constraint
		if len(paramNames) > 0 && constraint == "" {
			constraint = paramNames[len(paramNames)-1]
			paramNames = paramNames[:len(paramNames)-1]
		}

		// If we still don't have a constraint, default to "any"
		if constraint == "" {
			constraint = "any"
		}

		// Create TypeParameter entries for each name with the same constraint
		for _, name := range paramNames {
			typeParams = append(typeParams, types.TypeParameter{
				Name:       name,
				Constraint: constraint,
			})
		}
	}

	return typeParams
}
