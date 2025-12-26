package symbollinker

import (
	"errors"
	"strings"

	"github.com/standardbeagle/lci/internal/types"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// CSharpExtractor extracts symbols from C# source code
type CSharpExtractor struct {
	*BaseExtractor
}

// NewCSharpExtractor creates a new C# symbol extractor
func NewCSharpExtractor() *CSharpExtractor {
	return &CSharpExtractor{
		BaseExtractor: NewBaseExtractor("csharp", []string{".cs", ".csx"}),
	}
}

// ExtractSymbols extracts all symbols from a C# AST
func (ce *CSharpExtractor) ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error) {
	if tree == nil {
		return nil, errors.New("tree is nil")
	}

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("root node is nil")
	}

	builder := NewSymbolTableBuilder(fileID, "csharp")
	scopeManager := NewScopeManager()

	// Extract namespace and add to symbol table
	namespaceInfo := ce.extractNamespaces(root, content, builder, scopeManager, fileID)

	// Track using statements for resolution
	usings := ce.extractUsingStatements(root, content, builder, fileID)

	// Add usings to the builder as imports
	for _, using := range usings {
		builder.AddImport(using)
	}

	// Extract symbols with scope tracking
	ce.extractSymbolsFromNode(root, content, builder, scopeManager, fileID, namespaceInfo)

	// Build and return the symbol table
	table := builder.Build()

	return table, nil
}

// extractNamespaces extracts namespace declarations and creates namespace symbols
func (ce *CSharpExtractor) extractNamespaces(root *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) string {
	// Find namespace declarations
	ce.traverseNode(root, func(node *sitter.Node) bool {
		if node.Kind() == "namespace_declaration" || node.Kind() == "file_scoped_namespace_declaration" {
			ce.extractNamespaceDeclaration(node, content, builder, scopeManager, fileID)
		}
		return true
	})

	// Return the first namespace found for backward compatibility
	namespaceNode := FindChildByType(root, "namespace_declaration")
	if namespaceNode == nil {
		namespaceNode = FindChildByType(root, "file_scoped_namespace_declaration")
	}

	if namespaceNode == nil {
		return ""
	}

	nameNode := FindChildByType(namespaceNode, "qualified_name")
	if nameNode == nil {
		nameNode = FindChildByType(namespaceNode, "identifier")
	}

	if nameNode == nil {
		return ""
	}

	return GetNodeText(nameNode, content)
}

// extractNamespaceDeclaration extracts a single namespace declaration
func (ce *CSharpExtractor) extractNamespaceDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "qualified_name")
	if nameNode == nil {
		nameNode = FindChildByType(node, "identifier")
	}

	if nameNode == nil {
		return
	}

	namespaceName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Add namespace symbol
	builder.AddSymbol(
		namespaceName,
		types.SymbolKindNamespace,
		location,
		scopeManager.CurrentScope(),
		true, // Namespaces are public by default
	)
}

// extractUsingStatements extracts all using statements
func (ce *CSharpExtractor) extractUsingStatements(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []types.ImportInfo {
	var usings []types.ImportInfo

	ce.traverseNode(root, func(node *sitter.Node) bool {
		if node.Kind() == "using_directive" {
			using := ce.extractUsingDirective(node, content, fileID)
			if using.ImportPath != "" {
				usings = append(usings, using)
			}
		}
		return true
	})

	return usings
}

// extractUsingDirective extracts a single using directive
func (ce *CSharpExtractor) extractUsingDirective(node *sitter.Node, content []byte, fileID types.FileID) types.ImportInfo {
	imp := types.ImportInfo{
		Location: GetNodeLocation(node, fileID),
	}

	// Handle different using patterns:
	// using System;
	// using System.Collections.Generic;
	// using Alias = System.Collections.Generic.List<int>;
	// using static System.Math;

	isStatic := false
	hasAlias := false

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "static":
			isStatic = true
		case "qualified_name", "identifier":
			if !hasAlias {
				imp.ImportPath = GetNodeText(child, content)
			}
		case "name_equals":
			// This is an alias: using Alias = Namespace;
			hasAlias = true
			for j := uint(0); j < child.ChildCount(); j++ {
				aliasChild := child.Child(j)
				if aliasChild != nil && aliasChild.Kind() == "identifier" {
					imp.Alias = GetNodeText(aliasChild, content)
					break
				}
			}
		}
	}

	// Mark static using
	if isStatic {
		imp.IsTypeOnly = true // Reuse this field for static imports
	}

	return imp
}

// extractSymbolsFromNode recursively extracts symbols from an AST node
func (ce *CSharpExtractor) extractSymbolsFromNode(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, namespace string) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()

	switch nodeKind {
	case "method_declaration":
		ce.extractMethod(node, content, builder, scopeManager, fileID)
		return

	case "constructor_declaration":
		ce.extractConstructor(node, content, builder, scopeManager, fileID)
		return

	case "destructor_declaration":
		ce.extractDestructor(node, content, builder, scopeManager, fileID)
		return

	case "class_declaration":
		ce.extractClass(node, content, builder, scopeManager, fileID)
		return

	case "interface_declaration":
		ce.extractInterface(node, content, builder, scopeManager, fileID)
		return

	case "struct_declaration":
		ce.extractStruct(node, content, builder, scopeManager, fileID)
		return

	case "enum_declaration":
		ce.extractEnum(node, content, builder, scopeManager, fileID)
		return

	case "property_declaration":
		ce.extractProperty(node, content, builder, scopeManager, fileID)
		return

	case "field_declaration":
		ce.extractField(node, content, builder, scopeManager, fileID)
		return

	case "event_field_declaration":
		ce.extractEventField(node, content, builder, scopeManager, fileID)
		return

	case "event_declaration":
		ce.extractEvent(node, content, builder, scopeManager, fileID)
		return

	case "delegate_declaration":
		ce.extractDelegate(node, content, builder, scopeManager, fileID)
		return

	case "record_declaration":
		ce.extractRecord(node, content, builder, scopeManager, fileID)
		return

	case "local_function_statement":
		ce.extractLocalFunction(node, content, builder, scopeManager, fileID)
		return

	case "variable_declaration":
		ce.extractVariableDeclaration(node, content, builder, scopeManager, fileID)
		return

	case "block":
		// Enter block scope
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, "", startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			ce.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, namespace)
		}

		// Exit block scope
		scopeManager.PopScope()
		return

	case "if_statement", "for_statement", "while_statement", "switch_statement", "foreach_statement":
		// These create new scopes
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, nodeKind, startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			ce.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, namespace)
		}

		scopeManager.PopScope()
		return
	}

	// Process children for other node types
	for i := uint(0); i < node.ChildCount(); i++ {
		ce.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, namespace)
	}
}

// extractMethod extracts a method declaration
func (ce *CSharpExtractor) extractMethod(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	methodName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Extract modifiers
	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isStatic := ce.hasModifier(modifiers, "static")
	isAbstract := ce.hasModifier(modifiers, "abstract")
	isVirtual := ce.hasModifier(modifiers, "virtual")
	isOverride := ce.hasModifier(modifiers, "override")
	isAsync := ce.hasModifier(modifiers, "async")

	// Extract attributes
	attributes := ce.extractAttributes(node, content)

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
	localID := builder.AddSymbol(
		fullName,
		types.SymbolKindMethod,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		metadata := make(map[string]interface{})
		metadata["visibility"] = visibility
		if isStatic {
			metadata["static"] = true
		}
		if isAbstract {
			metadata["abstract"] = true
		}
		if isVirtual {
			metadata["virtual"] = true
		}
		if isOverride {
			metadata["override"] = true
		}
		if isAsync {
			metadata["async"] = true
		}

		signature := ce.extractMethodSignature(node, content)
		symbol.Signature = signature

		// Attach attributes
		if len(attributes) > 0 {
			symbol.Attributes = attributes
		}
		symbol.Type = visibility

		// Extract return type
		returnType := ce.extractReturnType(node, content)
		if returnType != "" {
			metadata["return_type"] = returnType
		}
	}

	// Enter method scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeMethod, fullName, startPos, endPos)

	// Extract parameters
	ce.extractParameters(node, content, builder, scopeManager, fileID)

	// Process method body
	body := FindChildByType(node, "block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit method scope
	scopeManager.PopScope()
}

// extractConstructor extracts a constructor declaration
func (ce *CSharpExtractor) extractConstructor(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	constructorName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isStatic := ce.hasModifier(modifiers, "static")

	// Get class name from scope
	className := ""
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
	}

	fullName := constructorName
	if className != "" {
		fullName = className + "." + constructorName
	}

	// Add constructor symbol
	localID := builder.AddSymbol(
		fullName,
		types.SymbolKindConstructor,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Type = visibility + " constructor"
		if isStatic {
			symbol.Type = "static " + symbol.Type
		}

		signature := ce.extractMethodSignature(node, content)
		symbol.Signature = signature
	}

	// Enter constructor scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeFunction, fullName, startPos, endPos)

	// Extract parameters
	ce.extractParameters(node, content, builder, scopeManager, fileID)

	// Process constructor body
	body := FindChildByType(node, "block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit constructor scope
	scopeManager.PopScope()
}

// extractDestructor extracts a destructor declaration
func (ce *CSharpExtractor) extractDestructor(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	destructorName := "~" + GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Get class name from scope
	className := ""
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
	}

	fullName := destructorName
	if className != "" {
		fullName = className + "." + destructorName
	}

	// Add destructor symbol
	builder.AddSymbol(
		fullName,
		types.SymbolKindFunction,
		location,
		scopeManager.CurrentScope(),
		false, // Destructors are not public in the API sense
	)
}

// extractClass extracts a class declaration
func (ce *CSharpExtractor) extractClass(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	className := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isAbstract := ce.hasModifier(modifiers, "abstract")
	isSealed := ce.hasModifier(modifiers, "sealed")
	isStatic := ce.hasModifier(modifiers, "static")
	isPartial := ce.hasModifier(modifiers, "partial")

	// Extract attributes
	attributes := ce.extractAttributes(node, content)

	// Add class symbol
	localID := builder.AddSymbol(
		className,
		types.SymbolKindClass,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		metadata := make(map[string]interface{})
		metadata["visibility"] = visibility
		if isAbstract {
			metadata["abstract"] = true
		}
		if isSealed {
			metadata["sealed"] = true
		}
		if isStatic {
			metadata["static"] = true
		}
		if isPartial {
			metadata["partial"] = true
		}

		if isAbstract {
			symbol.Type = "abstract class"
		} else if isSealed {
			symbol.Type = "sealed class"
		} else if isStatic {
			symbol.Type = "static class"
		} else {
			symbol.Type = "class"
		}

		// Attach attributes
		if len(attributes) > 0 {
			symbol.Attributes = attributes
		}
	}

	// Enter class scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, className, startPos, endPos)

	// Extract class body
	body := FindChildByType(node, "declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit class scope
	scopeManager.PopScope()
}

// extractInterface extracts an interface declaration
func (ce *CSharpExtractor) extractInterface(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	interfaceName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)

	// Add interface symbol
	builder.AddSymbol(
		interfaceName,
		types.SymbolKindInterface,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Enter interface scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeInterface, interfaceName, startPos, endPos)

	// Extract interface body
	body := FindChildByType(node, "declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit interface scope
	scopeManager.PopScope()
}

// extractStruct extracts a struct declaration
func (ce *CSharpExtractor) extractStruct(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	structName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)

	// Add struct symbol
	localID := builder.AddSymbol(
		structName,
		types.SymbolKindStruct,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Mark as struct
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Type = "struct"
	}

	// Enter struct scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, structName, startPos, endPos)

	// Extract struct body
	body := FindChildByType(node, "declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit struct scope
	scopeManager.PopScope()
}

// extractEnum extracts an enum declaration
func (ce *CSharpExtractor) extractEnum(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	enumName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)

	// Add enum symbol
	builder.AddSymbol(
		enumName,
		types.SymbolKindEnum,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Extract enum members
	body := FindChildByType(node, "enum_member_declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			if child != nil && child.Kind() == "enum_member_declaration" {
				memberNode := FindChildByType(child, "identifier")
				if memberNode != nil {
					memberName := GetNodeText(memberNode, content)
					fullName := enumName + "." + memberName
					localID := builder.AddSymbol(
						fullName,
						types.SymbolKindEnumMember,
						GetNodeLocation(memberNode, fileID),
						scopeManager.CurrentScope(),
						visibility == "public",
					)

					// Extract enum value if present
					valueNode := FindChildByType(child, "equals_value_clause")
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

// extractProperty extracts a property declaration
func (ce *CSharpExtractor) extractProperty(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	propertyName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Extract attributes
	attributes := ce.extractAttributes(node, content)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isStatic := ce.hasModifier(modifiers, "static")

	// Get class name from scope
	className := ""
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
	}

	fullName := propertyName
	if className != "" {
		fullName = className + "." + propertyName
	}

	// Add property symbol
	localID := builder.AddSymbol(
		fullName,
		types.SymbolKindProperty,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Type = visibility
		if isStatic {
			symbol.Type += " static"
		}

		// Extract property type
		typeNode := ce.findTypeNode(node)
		if typeNode != nil {
			metadata := make(map[string]interface{})
			metadata["property_type"] = GetNodeText(typeNode, content)
		}

		// Attach attributes
		if len(attributes) > 0 {
			symbol.Attributes = attributes
		}
	}
}

// extractField extracts a field declaration
func (ce *CSharpExtractor) extractField(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isStatic := ce.hasModifier(modifiers, "static")
	isReadonly := ce.hasModifier(modifiers, "readonly")
	isConst := ce.hasModifier(modifiers, "const")

	// Find variable declarators
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "variable_declaration" {
			ce.extractVariableDeclaratorsForField(child, content, builder, scopeManager, fileID, visibility, isStatic, isReadonly, isConst)
		}
	}
}

// extractVariableDeclaratorsForField extracts variable declarators in field declarations
func (ce *CSharpExtractor) extractVariableDeclaratorsForField(varDecl *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, visibility string, isStatic, isReadonly, isConst bool) {
	for i := uint(0); i < varDecl.ChildCount(); i++ {
		child := varDecl.Child(i)
		if child != nil && child.Kind() == "variable_declarator" {
			nameNode := FindChildByType(child, "identifier")
			if nameNode != nil {
				fieldName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				// Get class name from scope
				className := ""
				if scopeManager.CurrentScope().Type == types.ScopeClass {
					className = scopeManager.CurrentScope().Name
				}

				fullName := fieldName
				if className != "" {
					fullName = className + "." + fieldName
				}

				kind := types.SymbolKindField
				if isConst {
					kind = types.SymbolKindConstant
				}

				localID := builder.AddSymbol(
					fullName,
					kind,
					location,
					scopeManager.CurrentScope(),
					visibility == "public",
				)

				// Add metadata
				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Type = visibility
					if isStatic {
						symbol.Type += " static"
					}
					if isReadonly {
						symbol.Type += " readonly"
					}

					// Extract field type
					typeNode := ce.findTypeNode(varDecl)
					if typeNode != nil {
						metadata := make(map[string]interface{})
						metadata["field_type"] = GetNodeText(typeNode, content)
					}
				}
			}
		}
	}
}

// extractEventField extracts an event field declaration (simple event)
func (ce *CSharpExtractor) extractEventField(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isStatic := ce.hasModifier(modifiers, "static")

	// Find variable declaration within event field declaration
	varDecl := FindChildByType(node, "variable_declaration")
	if varDecl == nil {
		return
	}

	// Extract variable declarators
	for i := uint(0); i < varDecl.ChildCount(); i++ {
		child := varDecl.Child(i)
		if child != nil && child.Kind() == "variable_declarator" {
			nameNode := FindChildByType(child, "identifier")
			if nameNode != nil {
				eventName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				// Get class name from scope
				className := ""
				if scopeManager.CurrentScope().Type == types.ScopeClass {
					className = scopeManager.CurrentScope().Name
				}

				fullName := eventName
				if className != "" {
					fullName = className + "." + eventName
				}

				// Add event symbol
				localID := builder.AddSymbol(
					fullName,
					types.SymbolKindEvent,
					location,
					scopeManager.CurrentScope(),
					visibility == "public",
				)

				// Add metadata
				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Type = visibility + " event"
					if isStatic {
						symbol.Type += " static"
					}

					// Extract event type
					typeNode := ce.findTypeNode(varDecl)
					if typeNode != nil {
						symbol.Signature = GetNodeText(typeNode, content)
					}
				}
			}
		}
	}
}

// extractEvent extracts an event declaration (property-like event)
func (ce *CSharpExtractor) extractEvent(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	eventName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)

	// Get class name from scope
	className := ""
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
	}

	fullName := eventName
	if className != "" {
		fullName = className + "." + eventName
	}

	// Add event symbol
	localID := builder.AddSymbol(
		fullName,
		types.SymbolKindEvent,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Mark as event
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Type = visibility + " event"
	}
}

// extractDelegate extracts a delegate declaration
func (ce *CSharpExtractor) extractDelegate(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	delegateName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)

	// Add delegate symbol
	localID := builder.AddSymbol(
		delegateName,
		types.SymbolKindDelegate,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Mark as delegate and extract signature
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Type = "delegate"
		signature := ce.extractMethodSignature(node, content)
		symbol.Signature = signature
	}
}

// extractRecord extracts a record declaration
func (ce *CSharpExtractor) extractRecord(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	recordName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	modifiers := ce.extractModifiers(node, content)
	visibility := ce.getVisibilityFromModifiers(modifiers)
	isAbstract := ce.hasModifier(modifiers, "abstract")
	isSealed := ce.hasModifier(modifiers, "sealed")

	// Add record symbol
	localID := builder.AddSymbol(
		recordName,
		types.SymbolKindRecord,
		location,
		scopeManager.CurrentScope(),
		visibility == "public",
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		if isAbstract {
			symbol.Type = "abstract record"
		} else if isSealed {
			symbol.Type = "sealed record"
		} else {
			symbol.Type = "record"
		}

		// Extract record parameters as a signature
		params := FindChildByType(node, "parameter_list")
		if params != nil {
			symbol.Signature = recordName + GetNodeText(params, content)
		}
	}

	// Enter record scope (treat like class)
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, recordName, startPos, endPos)

	// Extract record body if present
	body := FindChildByType(node, "declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit record scope
	scopeManager.PopScope()
}

// extractLocalFunction extracts a local function statement
func (ce *CSharpExtractor) extractLocalFunction(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	funcName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Add local function symbol
	localID := builder.AddSymbol(
		funcName,
		types.SymbolKindFunction,
		location,
		scopeManager.CurrentScope(),
		false, // Local functions are not public
	)

	// Extract signature
	if symbol, ok := builder.symbols[localID]; ok {
		signature := ce.extractMethodSignature(node, content)
		symbol.Signature = signature
		symbol.Type = "local function"
	}

	// Enter function scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeFunction, funcName, startPos, endPos)

	// Extract parameters
	ce.extractParameters(node, content, builder, scopeManager, fileID)

	// Process function body
	body := FindChildByType(node, "block")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			ce.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit function scope
	scopeManager.PopScope()
}

// extractVariableDeclaration extracts local variable declarations
func (ce *CSharpExtractor) extractVariableDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "variable_declarator" {
			nameNode := FindChildByType(child, "identifier")
			if nameNode != nil {
				varName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				// Add local variable symbol
				builder.AddSymbol(
					varName,
					types.SymbolKindVariable,
					location,
					scopeManager.CurrentScope(),
					false, // Local variables are not public
				)
			}
		}
	}
}

// Helper methods

// extractModifiers extracts all modifiers from a node
func (ce *CSharpExtractor) extractModifiers(node *sitter.Node, content []byte) []string {
	var modifiers []string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		kind := child.Kind()

		// Handle modifier nodes (which contain the actual modifier text)
		if kind == "modifier" {
			// Look for modifier children
			for j := uint(0); j < child.ChildCount(); j++ {
				modChild := child.Child(j)
				if modChild != nil {
					modifierText := GetNodeText(modChild, content)
					modifiers = append(modifiers, modifierText)
				}
			}
		} else if kind == "public" || kind == "private" || kind == "protected" || kind == "internal" ||
			kind == "static" || kind == "abstract" || kind == "virtual" || kind == "override" ||
			kind == "sealed" || kind == "partial" || kind == "async" || kind == "readonly" || kind == "const" ||
			kind == "event" || kind == "delegate" {
			// Direct modifier keywords
			modifiers = append(modifiers, kind)
		}
	}

	return modifiers
}

// getVisibilityFromModifiers determines visibility from modifiers list
func (ce *CSharpExtractor) getVisibilityFromModifiers(modifiers []string) string {
	for _, modifier := range modifiers {
		switch modifier {
		case "public":
			return "public"
		case "private":
			return "private"
		case "protected":
			return "protected"
		case "internal":
			return "internal"
		case "protected internal":
			return "protected internal"
		case "private protected":
			return "private protected"
		}
	}
	return "private" // Default visibility in C#
}

// hasModifier checks if a modifier is present in the list
func (ce *CSharpExtractor) hasModifier(modifiers []string, modifier string) bool {
	for _, m := range modifiers {
		if m == modifier {
			return true
		}
	}
	return false
}

// extractParameters extracts method/function parameters
func (ce *CSharpExtractor) extractParameters(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	paramsList := FindChildByType(node, "parameter_list")
	if paramsList == nil {
		return
	}

	for i := uint(0); i < paramsList.ChildCount(); i++ {
		child := paramsList.Child(i)
		if child != nil && child.Kind() == "parameter" {
			nameNode := FindChildByType(child, "identifier")
			if nameNode != nil {
				paramName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				localID := builder.AddSymbol(
					paramName,
					types.SymbolKindParameter,
					location,
					scopeManager.CurrentScope(),
					false,
				)

				// Extract parameter type
				typeNode := ce.findTypeNode(child)
				if typeNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Type = GetNodeText(typeNode, content)
					}
				}

				// Check for parameter modifiers (ref, out, in, params)
				for j := uint(0); j < child.ChildCount(); j++ {
					modChild := child.Child(j)
					if modChild != nil {
						modText := GetNodeText(modChild, content)
						if modText == "ref" || modText == "out" || modText == "in" || modText == "params" {
							if symbol, ok := builder.symbols[localID]; ok {
								symbol.Type = modText + " " + symbol.Type
							}
							break
						}
					}
				}
			}
		}
	}
}

// extractMethodSignature extracts a method's signature
func (ce *CSharpExtractor) extractMethodSignature(node *sitter.Node, content []byte) string {
	var signature strings.Builder

	// Get method name
	nameNode := FindChildByType(node, "identifier")
	if nameNode != nil {
		signature.WriteString(GetNodeText(nameNode, content))
	}

	// Get generic parameters
	typeParams := FindChildByType(node, "type_parameter_list")
	if typeParams != nil {
		signature.WriteString(GetNodeText(typeParams, content))
	}

	// Get parameters
	params := FindChildByType(node, "parameter_list")
	if params != nil {
		signature.WriteString(GetNodeText(params, content))
	} else {
		signature.WriteString("()")
	}

	// Get return type
	returnType := ce.extractReturnType(node, content)
	if returnType != "" {
		signature.WriteString(" : ")
		signature.WriteString(returnType)
	}

	return signature.String()
}

// extractReturnType extracts the return type of a method
func (ce *CSharpExtractor) extractReturnType(node *sitter.Node, content []byte) string {
	// Look for return type in various positions
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			kind := child.Kind()
			if kind == "predefined_type" || kind == "identifier" || kind == "generic_name" ||
				kind == "nullable_type" || kind == "array_type" || kind == "pointer_type" {
				return GetNodeText(child, content)
			}
		}
	}
	return ""
}

// findTypeNode finds a type node in the given node's children
func (ce *CSharpExtractor) findTypeNode(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			kind := child.Kind()
			if kind == "predefined_type" || kind == "identifier" || kind == "generic_name" ||
				kind == "nullable_type" || kind == "array_type" || kind == "pointer_type" ||
				kind == "tuple_type" {
				return child
			}
		}
	}

	return nil
}

// traverseNode traverses the AST with a visitor function
func (ce *CSharpExtractor) traverseNode(node *sitter.Node, visitor func(*sitter.Node) bool) {
	if node == nil || !visitor(node) {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		ce.traverseNode(node.Child(i), visitor)
	}
}

// extractAttributes extracts C# attributes from a declaration node
// C# attributes appear before the declaration they apply to
func (ce *CSharpExtractor) extractAttributes(declarationNode *sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	// In C# tree-sitter AST, attributes can be found as siblings before the declaration
	// or as children of an attribute_list node
	parent := declarationNode.Parent()
	if parent == nil {
		return attributes
	}

	// Look for attribute_list nodes that precede this declaration
	for i := uint(0); i < parent.ChildCount(); i++ {
		child := parent.Child(i)
		if child == nil {
			continue
		}

		// Stop when we reach the declaration itself
		if child == declarationNode {
			break
		}

		// Process attribute_list nodes
		if child.Kind() == "attribute_list" {
			attrs := ce.extractAttributeListAttributes(child, content)
			attributes = append(attributes, attrs...)
		}
	}

	return attributes
}

// extractAttributeListAttributes extracts individual attributes from an attribute_list node
func (ce *CSharpExtractor) extractAttributeListAttributes(attributeListNode *sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	// Iterate through children to find attribute nodes
	for i := uint(0); i < attributeListNode.ChildCount(); i++ {
		child := attributeListNode.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "attribute" {
			attr := ce.extractSingleAttribute(child, content)
			if attr != nil {
				attributes = append(attributes, *attr)
			}
		}
	}

	return attributes
}

// extractSingleAttribute extracts a single attribute
func (ce *CSharpExtractor) extractSingleAttribute(attributeNode *sitter.Node, content []byte) *types.ContextAttribute {
	// Get the attribute name (first identifier)
	nameNode := FindChildByType(attributeNode, "identifier")
	if nameNode == nil {
		nameNode = FindChildByType(attributeNode, "qualified_name")
	}
	if nameNode == nil {
		return nil
	}

	// Get the full attribute text including arguments
	fullText := GetNodeText(attributeNode, content)

	return &types.ContextAttribute{
		Type:  types.AttrTypeDecorator,
		Value: fullText,
		Line:  int(attributeNode.StartByte()),
	}
}
