package symbollinker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/standardbeagle/lci/internal/types"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// PHPExtractor extracts symbols from PHP source code
type PHPExtractor struct {
	*BaseExtractor
}

// NewPHPExtractor creates a new PHP symbol extractor
func NewPHPExtractor() *PHPExtractor {
	return &PHPExtractor{
		BaseExtractor: NewBaseExtractor("php", []string{".php", ".phtml", ".php3", ".php4", ".php5", ".phar"}),
	}
}

// ExtractSymbols extracts all symbols from a PHP AST
func (pe *PHPExtractor) ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error) {
	if tree == nil {
		return nil, errors.New("tree is nil")
	}

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("root node is nil")
	}

	builder := NewSymbolTableBuilder(fileID, "php")
	scopeManager := NewScopeManager()

	// Extract namespace
	namespace := pe.extractNamespace(root, content)

	// Track includes/requires for resolution
	includes := pe.extractIncludes(root, content, builder, fileID)

	// Add includes to the builder as imports
	for _, inc := range includes {
		builder.AddImport(inc)
	}

	// Extract symbols with scope tracking
	pe.extractSymbolsFromNode(root, content, builder, scopeManager, fileID, namespace)

	// Extract WordPress hooks (add_action, add_filter, etc.)
	pe.extractWordPressHooks(root, content, builder, fileID)

	// Extract WordPress metadata (Plugin Name, Theme Name, Template Name, etc.)
	pe.extractWordPressMetadata(root, content, builder, fileID)

	// Extract Gutenberg blocks (register_block_type, etc.)
	pe.extractGutenbergBlocks(root, content, builder, fileID)

	// Build and return the symbol table
	table := builder.Build()

	return table, nil
}

// extractNamespace extracts the namespace declaration from the PHP file
func (pe *PHPExtractor) extractNamespace(root *sitter.Node, content []byte) string {
	namespaceNode := FindChildByType(root, "namespace_definition")
	if namespaceNode == nil {
		return ""
	}

	nameNode := FindChildByType(namespaceNode, "namespace_name")
	if nameNode == nil {
		return ""
	}

	return GetNodeText(nameNode, content)
}

// extractIncludes extracts all include/require statements
func (pe *PHPExtractor) extractIncludes(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []types.ImportInfo {
	var includes []types.ImportInfo

	pe.traverseNode(root, func(node *sitter.Node) bool {
		nodeKind := node.Kind()

		switch nodeKind {
		case "include_expression", "require_expression", "include_once_expression", "require_once_expression":
			inc := pe.extractIncludeExpression(node, content, fileID, nodeKind)
			if inc.ImportPath != "" {
				includes = append(includes, inc)
			}

		case "namespace_use_declaration":
			// PHP use statements
			pe.extractUseDeclaration(node, content, builder, fileID, &includes)
		}

		return true // Continue traversal
	})

	return includes
}

// extractIncludeExpression extracts a single include/require expression
func (pe *PHPExtractor) extractIncludeExpression(node *sitter.Node, content []byte, fileID types.FileID, exprType string) types.ImportInfo {
	imp := types.ImportInfo{
		Location: GetNodeLocation(node, fileID),
	}

	// Find the path expression (usually a string literal)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "string":
			// Remove quotes from path
			path := GetNodeText(child, content)
			if len(path) >= 2 {
				imp.ImportPath = path[1 : len(path)-1]
			}
		case "concatenation":
			// Handle string concatenation in paths
			imp.ImportPath = pe.extractConcatenatedPath(child, content)
		}
	}

	// Set metadata based on include type
	if strings.Contains(exprType, "once") {
		imp.IsTypeOnly = true // Use this field to mark "once" variants
	}

	return imp
}

// extractUseDeclaration extracts PHP use statements
func (pe *PHPExtractor) extractUseDeclaration(node *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID, includes *[]types.ImportInfo) {
	// Handle different use statement types:
	// use Namespace\ClassName;
	// use Namespace\ClassName as Alias;
	// use Namespace\{Class1, Class2};
	// use function Namespace\functionName;
	// use const Namespace\CONSTANT;

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "namespace_use_clause":
			pe.extractUseClause(child, content, builder, fileID, includes)
		case "namespace_name":
			// Handle group imports: use Namespace\{Class1, Class2};
			pe.extractGroupedUseDeclaration(node, child, content, builder, fileID, includes)
		case "namespace_use_group":
			pe.extractUseGroup(child, content, builder, fileID, includes, "")
		}
	}
}

// extractUseClause extracts a single use clause
func (pe *PHPExtractor) extractUseClause(node *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID, includes *[]types.ImportInfo) {
	imp := types.ImportInfo{
		Location: GetNodeLocation(node, fileID),
	}

	var isFunction, isConst bool
	var importPath string

	// Check for function or const keywords first
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			text := GetNodeText(child, content)
			if text == "function" {
				isFunction = true
			} else if text == "const" {
				isConst = true
			}
		}
	}

	// Extract the qualified name or simple name
	qualifiedNode := FindChildByType(node, "qualified_name")
	if qualifiedNode != nil {
		// For qualified names, we need to reconstruct the full path
		importPath = pe.extractQualifiedName(qualifiedNode, content)
	} else {
		// Look for simple name
		nameNode := FindChildByType(node, "name")
		if nameNode != nil {
			importPath = GetNodeText(nameNode, content)
		}
	}

	if importPath != "" {
		imp.ImportPath = importPath

		// Check for alias
		aliasFound := false
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && GetNodeText(child, content) == "as" {
				// Next child should be the alias
				if i+1 < node.ChildCount() {
					aliasNode := node.Child(i + 1)
					if aliasNode != nil {
						imp.Alias = GetNodeText(aliasNode, content)
						aliasFound = true
					}
				}
				break
			}
		}

		// If no alias, use the last part of the import path
		if !aliasFound {
			parts := strings.Split(importPath, "\\")
			imp.Alias = parts[len(parts)-1]
		}

		// Set metadata for function/const imports
		if isFunction {
			imp.IsTypeOnly = true // Reuse this field for function imports
		} else if isConst {
			imp.IsTypeOnly = false // Normal import for constants
		}

		*includes = append(*includes, imp)
	}
}

// extractQualifiedName reconstructs a qualified name from the AST
func (pe *PHPExtractor) extractQualifiedName(node *sitter.Node, content []byte) string {
	var parts []string

	// Walk through the qualified_name node and collect all name parts
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			if child.Kind() == "name" {
				parts = append(parts, GetNodeText(child, content))
			} else if child.Kind() == "namespace_name" {
				// Handle nested namespace_name nodes
				nameText := pe.extractQualifiedName(child, content)
				if nameText != "" {
					parts = append(parts, nameText)
				}
			}
		}
	}

	return strings.Join(parts, "\\")
}

// extractGroupedUseDeclaration extracts grouped use declarations
func (pe *PHPExtractor) extractGroupedUseDeclaration(node *sitter.Node, baseNameNode *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID, includes *[]types.ImportInfo) {
	// Extract the base namespace path
	baseName := pe.extractQualifiedName(baseNameNode, content)
	if baseName == "" {
		baseName = GetNodeText(baseNameNode, content)
	}

	// Find the use group
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "namespace_use_group" {
			pe.extractUseGroup(child, content, builder, fileID, includes, baseName)
		}
	}
}

// extractUseGroup extracts items from a use group
func (pe *PHPExtractor) extractUseGroup(groupNode *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID, includes *[]types.ImportInfo, baseName string) {
	for i := uint(0); i < groupNode.ChildCount(); i++ {
		child := groupNode.Child(i)
		if child != nil && child.Kind() == "namespace_use_clause" {
			imp := types.ImportInfo{
				Location: GetNodeLocation(child, fileID),
			}

			// Extract the name
			nameNode := FindChildByType(child, "name")
			if nameNode != nil {
				relativeName := GetNodeText(nameNode, content)

				// Construct full namespace path
				if baseName != "" {
					imp.ImportPath = baseName + "\\" + relativeName
				} else {
					imp.ImportPath = relativeName
				}

				// Check for alias (in grouped use, alias would be after "as" keyword)
				aliasFound := false
				for j := uint(0); j < child.ChildCount(); j++ {
					childNode := child.Child(j)
					if childNode != nil && GetNodeText(childNode, content) == "as" {
						// Next child should be the alias
						if j+1 < child.ChildCount() {
							aliasNode := child.Child(j + 1)
							if aliasNode != nil {
								imp.Alias = GetNodeText(aliasNode, content)
								aliasFound = true
							}
						}
						break
					}
				}

				// If no alias, use the relative name
				if !aliasFound {
					imp.Alias = relativeName
				}

				if imp.ImportPath != "" {
					*includes = append(*includes, imp)
				}
			}
		}
	}
}

// extractSymbolsFromNode recursively extracts symbols from an AST node
func (pe *PHPExtractor) extractSymbolsFromNode(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, namespace string) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()

	switch nodeKind {
	case "namespace_definition":
		pe.extractNamespaceSymbol(node, content, builder, scopeManager, fileID)

	case "function_definition":
		pe.extractFunction(node, content, builder, scopeManager, fileID)

	case "method_declaration":
		pe.extractMethod(node, content, builder, scopeManager, fileID)

	case "class_declaration":
		pe.extractClass(node, content, builder, scopeManager, fileID)

	case "interface_declaration":
		pe.extractInterface(node, content, builder, scopeManager, fileID)

	case "trait_declaration":
		pe.extractTrait(node, content, builder, scopeManager, fileID)

	case "enum_declaration":
		pe.extractEnum(node, content, builder, scopeManager, fileID)

	case "property_declaration":
		pe.extractProperty(node, content, builder, scopeManager, fileID)

	case "const_declaration":
		// Handle both class constants and global constants
		if scopeManager.CurrentScope().Type == types.ScopeClass {
			pe.extractClassConstant(node, content, builder, scopeManager, fileID)
		} else {
			pe.extractGlobalConstant(node, content, builder, scopeManager, fileID)
		}

	case "expression_statement":
		// Handle variable assignments and function calls
		pe.extractExpressionStatement(node, content, builder, scopeManager, fileID)

	case "compound_statement":
		// Enter block scope
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, "", startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			pe.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, namespace)
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
			pe.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, namespace)
		}

		scopeManager.PopScope()
		return
	}

	// Process children for other node types
	for i := uint(0); i < node.ChildCount(); i++ {
		pe.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID, namespace)
	}
}

// extractFunction extracts a function declaration
func (pe *PHPExtractor) extractFunction(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	funcName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// PHP functions are global unless in a namespace/class
	isExported := true // PHP functions are generally accessible

	// Add function symbol
	localID := builder.AddSymbol(
		funcName,
		types.SymbolKindFunction,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Extract function signature
	signature := pe.extractFunctionSignature(node, content)
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Signature = signature
	}

	// Enter function scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeFunction, funcName, startPos, endPos)

	// Extract parameters
	pe.extractParameters(node, content, builder, scopeManager, fileID)

	// Process function body
	body := FindChildByType(node, "compound_statement")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit function scope
	scopeManager.PopScope()
}

// extractMethod extracts a method declaration
func (pe *PHPExtractor) extractMethod(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	methodName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Extract PHP 8.0+ attributes
	attributes := pe.extractAttributes(node, content, fileID)

	// Extract visibility modifiers
	visibility := pe.extractVisibility(node, content)
	isStatic := pe.hasModifier(node, content, "static")
	isAbstract := pe.hasModifier(node, content, "abstract")
	isFinal := pe.hasModifier(node, content, "final")

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
		if isFinal {
			metadata["final"] = true
		}

		// Add PHP 8.0+ attributes to metadata
		if len(attributes) > 0 {
			attrNames := make([]string, len(attributes))
			for i, attr := range attributes {
				attrNames[i] = attr.Name
			}
			metadata["attributes"] = attrNames
		}

		signature := pe.extractFunctionSignature(node, content)
		symbol.Signature = signature
		symbol.Type = visibility
	}

	// Enter method scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeMethod, fullName, startPos, endPos)

	// Extract parameters
	pe.extractParameters(node, content, builder, scopeManager, fileID)

	// Extract promoted properties for constructors (PHP 8.0+)
	if methodName == "__construct" {
		pe.extractPromotedProperties(node, content, builder, scopeManager, fileID)
	}

	// Process method body
	body := FindChildByType(node, "compound_statement")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit method scope
	scopeManager.PopScope()
}

// extractClass extracts a class declaration
func (pe *PHPExtractor) extractClass(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	className := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Extract PHP 8.0+ attributes
	attributes := pe.extractAttributes(node, content, fileID)

	// Extract class modifiers
	isFinal := pe.hasModifier(node, content, "final")
	isAbstract := pe.hasModifier(node, content, "abstract")

	// Add class symbol
	localID := builder.AddSymbol(
		className,
		types.SymbolKindClass,
		location,
		scopeManager.CurrentScope(),
		true, // PHP classes are accessible
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		if isFinal {
			symbol.Type = "final"
		} else if isAbstract {
			symbol.Type = "abstract"
		}

		// Add PHP 8.0+ attributes to signature for searchability
		if len(attributes) > 0 {
			attrStrs := make([]string, len(attributes))
			for i, attr := range attributes {
				if len(attr.Arguments) > 0 {
					attrStrs[i] = fmt.Sprintf("#[%s(%s)]", attr.Name, strings.Join(attr.Arguments, ", "))
				} else {
					attrStrs[i] = fmt.Sprintf("#[%s]", attr.Name)
				}
			}
			symbol.Signature = strings.Join(attrStrs, " ") + " class " + className
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
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit class scope
	scopeManager.PopScope()
}

// extractInterface extracts an interface declaration
func (pe *PHPExtractor) extractInterface(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	interfaceName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Add interface symbol
	builder.AddSymbol(
		interfaceName,
		types.SymbolKindInterface,
		location,
		scopeManager.CurrentScope(),
		true,
	)

	// Enter interface scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeInterface, interfaceName, startPos, endPos)

	// Extract interface body
	body := FindChildByType(node, "declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit interface scope
	scopeManager.PopScope()
}

// extractTrait extracts a trait declaration
func (pe *PHPExtractor) extractTrait(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	traitName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Add trait symbol
	localID := builder.AddSymbol(
		traitName,
		types.SymbolKindTrait,
		location,
		scopeManager.CurrentScope(),
		true,
	)

	// Mark as trait
	if symbol, ok := builder.symbols[localID]; ok {
		symbol.Type = "trait"
	}

	// Enter trait scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, traitName, startPos, endPos)

	// Extract trait body
	body := FindChildByType(node, "declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID, "")
		}
	}

	// Exit trait scope
	scopeManager.PopScope()
}

// extractProperty extracts a property declaration
func (pe *PHPExtractor) extractProperty(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Extract visibility
	visibility := pe.extractVisibility(node, content)
	isStatic := pe.hasModifier(node, content, "static")

	// Find all property elements in this declaration
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "property_element" {
			nameNode := FindChildByType(child, "variable_name")
			if nameNode != nil {
				propName := GetNodeText(nameNode, content)
				// Remove the $ prefix from PHP variable names
				if strings.HasPrefix(propName, "$") {
					propName = propName[1:]
				}
				location := GetNodeLocation(nameNode, fileID)

				// Get class name from scope
				className := ""
				if scopeManager.CurrentScope().Type == types.ScopeClass {
					className = scopeManager.CurrentScope().Name
				}

				fullName := propName
				if className != "" {
					fullName = className + "." + propName
				}

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
				}
			}
		}
	}
}

// extractClassConstant extracts a class constant declaration
func (pe *PHPExtractor) extractClassConstant(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	visibility := pe.extractVisibility(node, content)

	// Find all constant elements
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "const_element" {
			nameNode := FindChildByType(child, "name")
			if nameNode != nil {
				constName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				// Get class name from scope
				className := ""
				if scopeManager.CurrentScope().Type == types.ScopeClass {
					className = scopeManager.CurrentScope().Name
				}

				fullName := constName
				if className != "" {
					fullName = className + "." + constName
				}

				localID := builder.AddSymbol(
					fullName,
					types.SymbolKindConstant,
					location,
					scopeManager.CurrentScope(),
					visibility == "public" || visibility == "", // Constants are public by default
				)

				// Extract value
				valueNode := FindChildByType(child, "literal")
				if valueNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Value = GetNodeText(valueNode, content)
						symbol.Type = visibility
					}
				}
			}
		}
	}
}

// extractExpressionStatement extracts variable assignments and other expressions
func (pe *PHPExtractor) extractExpressionStatement(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Look for assignment expressions to track variables
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			switch child.Kind() {
			case "assignment_expression":
				pe.extractVariableAssignment(child, content, builder, scopeManager, fileID)
			case "function_call_expression":
				// Handle define() calls for global constants
				pe.extractDefineCall(child, content, builder, scopeManager, fileID)
			}
		}
	}
}

// extractVariableAssignment extracts variable assignments
func (pe *PHPExtractor) extractVariableAssignment(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Find the left side (variable being assigned)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "variable_name" {
			varName := GetNodeText(child, content)
			// Remove the $ prefix from PHP variable names
			if strings.HasPrefix(varName, "$") {
				varName = varName[1:]
			}
			location := GetNodeLocation(child, fileID)

			// Add variable symbol (not exported as it's a local variable)
			builder.AddSymbol(
				varName,
				types.SymbolKindVariable,
				location,
				scopeManager.CurrentScope(),
				false,
			)
			break // Only process first variable (left side of assignment)
		}
	}
}

// extractDefineCall extracts constants from define() function calls
func (pe *PHPExtractor) extractDefineCall(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Check if this is a define() call
	nameNode := FindChildByType(node, "name")
	if nameNode == nil || GetNodeText(nameNode, content) != "define" {
		return
	}

	// Look for arguments
	argList := FindChildByType(node, "arguments")
	if argList == nil {
		return
	}

	// Find the first argument (constant name)
	var constNameArg *sitter.Node
	var valueArg *sitter.Node
	argCount := 0

	for i := uint(0); i < argList.ChildCount(); i++ {
		child := argList.Child(i)
		if child != nil && child.Kind() == "argument" {
			if argCount == 0 {
				constNameArg = child
			} else if argCount == 1 {
				valueArg = child
				break
			}
			argCount++
		}
	}

	if constNameArg != nil {
		// Extract constant name from string
		stringNode := FindChildByType(constNameArg, "string")
		if stringNode != nil {
			constNameContent := FindChildByType(stringNode, "string_content")
			if constNameContent != nil {
				constName := GetNodeText(constNameContent, content)
				location := GetNodeLocation(constNameContent, fileID)

				localID := builder.AddSymbol(
					constName,
					types.SymbolKindConstant,
					location,
					scopeManager.CurrentScope(),
					true, // define() creates global constants
				)

				// Extract value from second argument if available
				if valueArg != nil && localID > 0 {
					valueString := FindChildByType(valueArg, "string")
					if valueString != nil {
						valueContent := FindChildByType(valueString, "string_content")
						if valueContent != nil {
							if symbol, ok := builder.symbols[localID]; ok {
								symbol.Value = GetNodeText(valueContent, content)
							}
						}
					}
				}
			}
		}
	}
}

// extractEnum extracts an enum declaration
func (pe *PHPExtractor) extractEnum(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	enumName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Add enum symbol
	builder.AddSymbol(
		enumName,
		types.SymbolKindEnum,
		location,
		scopeManager.CurrentScope(),
		true, // Enums are generally public
	)

	// Enter enum scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, enumName, startPos, endPos) // Use ScopeClass for enum scope

	// Extract enum members
	body := FindChildByType(node, "enum_declaration_list")
	if body != nil {
		for i := uint(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			if child != nil && child.Kind() == "enum_case" {
				pe.extractEnumMember(child, content, builder, scopeManager, fileID, enumName)
			}
		}
	}

	// Exit enum scope
	scopeManager.PopScope()
}

// extractEnumMember extracts an enum case/member
func (pe *PHPExtractor) extractEnumMember(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, enumName string) {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}

	memberName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Create qualified name
	fullName := enumName + "." + memberName

	// Add enum member symbol
	localID := builder.AddSymbol(
		fullName,
		types.SymbolKindEnumMember,
		location,
		scopeManager.CurrentScope(),
		true, // Enum members are public
	)

	// Extract value if present
	valueNode := FindChildByType(node, "string")
	if valueNode == nil {
		valueNode = FindChildByType(node, "integer")
	}
	if valueNode != nil && localID > 0 {
		if symbol, ok := builder.symbols[localID]; ok {
			symbol.Value = GetNodeText(valueNode, content)
		}
	}
}

// extractGlobalConstant extracts a global constant declaration
func (pe *PHPExtractor) extractGlobalConstant(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Find all constant elements
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "const_element" {
			nameNode := FindChildByType(child, "name")
			if nameNode != nil {
				constName := GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				localID := builder.AddSymbol(
					constName,
					types.SymbolKindConstant,
					location,
					scopeManager.CurrentScope(),
					true, // Global constants are public
				)

				// Extract value
				valueNode := FindChildByType(child, "literal")
				if valueNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Value = GetNodeText(valueNode, content)
					}
				}
			}
		}
	}
}

// extractNamespaceSymbol extracts namespace declaration as a symbol
func (pe *PHPExtractor) extractNamespaceSymbol(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "namespace_name")
	if nameNode == nil {
		return
	}

	namespaceName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Add namespace symbol (always exported)
	builder.AddSymbol(
		namespaceName,
		types.SymbolKindNamespace,
		location,
		scopeManager.CurrentScope(),
		true,
	)
}

// Helper methods

// extractVisibility extracts visibility modifiers from a node
func (pe *PHPExtractor) extractVisibility(node *sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "visibility_modifier" {
			return GetNodeText(child, content)
		}
	}
	return "public" // Default visibility
}

// hasModifier checks if a node has a specific modifier
func (pe *PHPExtractor) hasModifier(node *sitter.Node, content []byte, modifier string) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && GetNodeText(child, content) == modifier {
			return true
		}
	}
	return false
}

// extractParameters extracts function/method parameters
func (pe *PHPExtractor) extractParameters(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	paramsList := FindChildByType(node, "formal_parameters")
	if paramsList == nil {
		return
	}

	for i := uint(0); i < paramsList.ChildCount(); i++ {
		child := paramsList.Child(i)
		if child != nil && child.Kind() == "simple_parameter" {
			nameNode := FindChildByType(child, "variable_name")
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

				// Extract type hint if present
				typeNode := FindChildByType(child, "type")
				if typeNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Type = GetNodeText(typeNode, content)
					}
				}
			}
		}
	}
}

// extractFunctionSignature extracts a function's signature
func (pe *PHPExtractor) extractFunctionSignature(node *sitter.Node, content []byte) string {
	var signature strings.Builder

	// Get function name
	nameNode := FindChildByType(node, "name")
	if nameNode != nil {
		signature.WriteString(GetNodeText(nameNode, content))
	}

	// Get parameters
	params := FindChildByType(node, "formal_parameters")
	if params != nil {
		signature.WriteString(GetNodeText(params, content))
	} else {
		signature.WriteString("()")
	}

	// Get return type if present
	returnType := FindChildByType(node, "return_type")
	if returnType != nil {
		signature.WriteString(": ")
		signature.WriteString(GetNodeText(returnType, content))
	}

	return signature.String()
}

// extractConcatenatedPath handles string concatenation in include paths
func (pe *PHPExtractor) extractConcatenatedPath(node *sitter.Node, content []byte) string {
	// This is a simplified implementation
	// In practice, you might want to evaluate simple string concatenations
	return GetNodeText(node, content)
}

// traverseNode traverses the AST with a visitor function
func (pe *PHPExtractor) traverseNode(node *sitter.Node, visitor func(*sitter.Node) bool) {
	if node == nil || !visitor(node) {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		pe.traverseNode(node.Child(i), visitor)
	}
}

// PHPAttribute represents a PHP 8.0+ attribute
type PHPAttribute struct {
	Name      string
	Arguments []string
	Location  types.SymbolLocation
}

// extractAttributes extracts PHP 8.0+ attributes from a node
// Returns a slice of attributes found on the declaration
func (pe *PHPExtractor) extractAttributes(node *sitter.Node, content []byte, fileID types.FileID) []PHPAttribute {
	var attributes []PHPAttribute

	// Look for attribute_list as the first child of the declaration
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "attribute_list" {
			// Process each attribute_group within the attribute_list
			for j := uint(0); j < child.ChildCount(); j++ {
				attrGroup := child.Child(j)
				if attrGroup != nil && attrGroup.Kind() == "attribute_group" {
					// Extract each attribute within the group
					for k := uint(0); k < attrGroup.ChildCount(); k++ {
						attrNode := attrGroup.Child(k)
						if attrNode != nil && attrNode.Kind() == "attribute" {
							attr := pe.extractSingleAttribute(attrNode, content, fileID)
							if attr.Name != "" {
								attributes = append(attributes, attr)
							}
						}
					}
				}
			}
		}
	}

	return attributes
}

// extractSingleAttribute extracts a single attribute from an attribute node
func (pe *PHPExtractor) extractSingleAttribute(node *sitter.Node, content []byte, fileID types.FileID) PHPAttribute {
	attr := PHPAttribute{
		Location: GetNodeLocation(node, fileID),
	}

	// Extract attribute name
	nameNode := FindChildByType(node, "name")
	if nameNode != nil {
		attr.Name = GetNodeText(nameNode, content)
	}

	// Extract arguments if present
	argsNode := FindChildByType(node, "arguments")
	if argsNode != nil {
		for i := uint(0); i < argsNode.ChildCount(); i++ {
			argChild := argsNode.Child(i)
			if argChild != nil && argChild.Kind() == "argument" {
				argText := GetNodeText(argChild, content)
				attr.Arguments = append(attr.Arguments, argText)
			}
		}
	}

	return attr
}

// WordPressHook represents a WordPress action or filter hook registration
type WordPressHook struct {
	HookType string // "action", "filter", "shortcode", "rest_route"
	HookName string // The hook name (e.g., "init", "the_content")
	Callback string // The callback function or method
	Priority int    // Hook priority (default 10)
	Location types.SymbolLocation
}

// extractWordPressHooks extracts WordPress hook registrations from the AST
// Detects: add_action, add_filter, add_shortcode, register_rest_route
func (pe *PHPExtractor) extractWordPressHooks(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []WordPressHook {
	var hooks []WordPressHook

	pe.traverseNode(root, func(node *sitter.Node) bool {
		if node.Kind() != "function_call_expression" {
			return true
		}

		// Get function name
		nameNode := FindChildByType(node, "name")
		if nameNode == nil {
			return true
		}

		funcName := GetNodeText(nameNode, content)

		// Check for WordPress hook functions
		var hookType string
		switch funcName {
		case "add_action":
			hookType = "action"
		case "add_filter":
			hookType = "filter"
		case "add_shortcode":
			hookType = "shortcode"
		case "register_rest_route":
			hookType = "rest_route"
		default:
			return true
		}

		// Extract hook details from arguments
		argsNode := FindChildByType(node, "arguments")
		if argsNode == nil {
			return true
		}

		hook := WordPressHook{
			HookType: hookType,
			Priority: 10, // WordPress default priority
			Location: GetNodeLocation(node, fileID),
		}

		// Extract arguments
		argIndex := 0
		for i := uint(0); i < argsNode.ChildCount(); i++ {
			child := argsNode.Child(i)
			if child == nil || child.Kind() != "argument" {
				continue
			}

			switch argIndex {
			case 0:
				// First argument: hook name (or namespace for REST routes)
				hook.HookName = pe.extractStringArgument(child, content)
			case 1:
				// Second argument: callback (or route path for REST routes)
				if hookType == "rest_route" {
					// For REST routes, second arg is the path
					hook.HookName += pe.extractStringArgument(child, content)
				} else {
					hook.Callback = pe.extractCallbackArgument(child, content)
				}
			case 2:
				// Third argument: priority for actions/filters, or options for REST routes
				if hookType == "action" || hookType == "filter" {
					hook.Priority = pe.extractIntArgument(child, content, 10)
				} else if hookType == "rest_route" {
					// For REST routes, extract callback from options array
					hook.Callback = pe.extractRestRouteCallback(child, content)
				}
			}
			argIndex++
		}

		if hook.HookName != "" {
			hooks = append(hooks, hook)

			// Add as a symbol for searchability
			symbolName := fmt.Sprintf("hook:%s:%s", hook.HookType, hook.HookName)
			localID := builder.AddSymbol(
				symbolName,
				types.SymbolKindEvent, // Use Event kind for hooks
				hook.Location,
				nil,
				true,
			)

			if symbol, ok := builder.symbols[localID]; ok {
				symbol.Signature = fmt.Sprintf("%s('%s', %s)", funcName, hook.HookName, hook.Callback)
				symbol.Type = hook.HookType
			}
		}

		return true
	})

	return hooks
}

// extractStringArgument extracts a string value from an argument node
func (pe *PHPExtractor) extractStringArgument(node *sitter.Node, content []byte) string {
	// Look for string node within the argument
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "string" {
			// Extract string content
			contentNode := FindChildByType(child, "string_content")
			if contentNode != nil {
				return GetNodeText(contentNode, content)
			}
			// Fall back to getting the full string minus quotes
			text := GetNodeText(child, content)
			if len(text) >= 2 {
				return text[1 : len(text)-1]
			}
		}
	}
	return ""
}

// extractCallbackArgument extracts a callback from an argument (string or array)
func (pe *PHPExtractor) extractCallbackArgument(node *sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "string":
			// Simple function name callback
			contentNode := FindChildByType(child, "string_content")
			if contentNode != nil {
				return GetNodeText(contentNode, content)
			}
		case "array_creation_expression":
			// Array callback like [$this, 'method'] or array($this, 'method')
			return pe.extractArrayCallback(child, content)
		}
	}
	return ""
}

// extractArrayCallback extracts callback from array notation [$this, 'method']
func (pe *PHPExtractor) extractArrayCallback(node *sitter.Node, content []byte) string {
	var parts []string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "array_element_initializer" {
			// Get the value inside the element
			for j := uint(0); j < child.ChildCount(); j++ {
				elem := child.Child(j)
				if elem == nil {
					continue
				}

				switch elem.Kind() {
				case "variable_name":
					parts = append(parts, GetNodeText(elem, content))
				case "string":
					contentNode := FindChildByType(elem, "string_content")
					if contentNode != nil {
						parts = append(parts, GetNodeText(contentNode, content))
					}
				}
			}
		}
	}

	if len(parts) == 2 {
		return fmt.Sprintf("[%s, '%s']", parts[0], parts[1])
	} else if len(parts) == 1 {
		return parts[0]
	}
	return ""
}

// extractIntArgument extracts an integer value from an argument
func (pe *PHPExtractor) extractIntArgument(node *sitter.Node, content []byte, defaultVal int) int {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "integer" {
			text := GetNodeText(child, content)
			var val int
			if _, err := fmt.Sscanf(text, "%d", &val); err == nil {
				return val
			}
		}
	}
	return defaultVal
}

// extractRestRouteCallback extracts callback from REST route options array
func (pe *PHPExtractor) extractRestRouteCallback(node *sitter.Node, content []byte) string {
	// Look for 'callback' key in the options array
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "array_creation_expression" {
			for j := uint(0); j < child.ChildCount(); j++ {
				elem := child.Child(j)
				if elem == nil || elem.Kind() != "array_element_initializer" {
					continue
				}

				// Check if this is the 'callback' key
				keyNode := FindChildByType(elem, "string")
				if keyNode != nil {
					contentNode := FindChildByType(keyNode, "string_content")
					if contentNode != nil && GetNodeText(contentNode, content) == "callback" {
						// Get the value
						for k := uint(0); k < elem.ChildCount(); k++ {
							valChild := elem.Child(k)
							if valChild != nil && valChild.Kind() == "string" {
								valContentNode := FindChildByType(valChild, "string_content")
								if valContentNode != nil {
									return GetNodeText(valContentNode, content)
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// extractPromotedProperties extracts constructor property promotion parameters
// PHP 8.0+ feature: public function __construct(private string $name) {}
func (pe *PHPExtractor) extractPromotedProperties(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Only process if this is a constructor
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return
	}
	methodName := GetNodeText(nameNode, content)
	if methodName != "__construct" {
		return
	}

	paramsList := FindChildByType(node, "formal_parameters")
	if paramsList == nil {
		return
	}

	// Get class name from scope for full property name
	className := ""
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
	}

	for i := uint(0); i < paramsList.ChildCount(); i++ {
		child := paramsList.Child(i)
		if child != nil && child.Kind() == "property_promotion_parameter" {
			// Extract property name
			varNode := FindChildByType(child, "variable_name")
			if varNode == nil {
				continue
			}
			propName := GetNodeText(varNode, content)
			location := GetNodeLocation(varNode, fileID)

			// Extract visibility
			visibility := "public"
			visNode := FindChildByType(child, "visibility_modifier")
			if visNode != nil {
				visibility = GetNodeText(visNode, content)
			}

			// Check for readonly modifier
			isReadonly := false
			for j := uint(0); j < child.ChildCount(); j++ {
				modChild := child.Child(j)
				if modChild != nil && modChild.Kind() == "readonly_modifier" {
					isReadonly = true
					break
				}
			}

			// Extract type
			propType := ""
			for j := uint(0); j < child.ChildCount(); j++ {
				typeChild := child.Child(j)
				if typeChild != nil {
					kind := typeChild.Kind()
					if kind == "primitive_type" || kind == "named_type" || kind == "nullable_type" || kind == "union_type" || kind == "intersection_type" {
						propType = GetNodeText(typeChild, content)
						break
					}
				}
			}

			// Create full property name
			fullName := propName
			if className != "" {
				fullName = className + "." + propName
			}

			// Add as property symbol (promoted properties become class properties)
			localID := builder.AddSymbol(
				fullName,
				types.SymbolKindProperty,
				location,
				scopeManager.CurrentScope(),
				visibility == "public",
			)

			// Add metadata
			if symbol, ok := builder.symbols[localID]; ok {
				symbol.Type = propType
				metadata := make(map[string]interface{})
				metadata["visibility"] = visibility
				metadata["promoted"] = true
				if isReadonly {
					metadata["readonly"] = true
				}
				// Store metadata in signature for now
				if isReadonly {
					symbol.Signature = fmt.Sprintf("%s readonly %s %s", visibility, propType, propName)
				} else {
					symbol.Signature = fmt.Sprintf("%s %s %s", visibility, propType, propName)
				}
			}
		}
	}
}

// WordPressMetadata represents metadata extracted from WordPress file headers
type WordPressMetadata struct {
	Type        string // "plugin", "theme", "template"
	Name        string
	Description string
	Version     string
	Author      string
	AuthorURI   string
	URI         string
	License     string
	TextDomain  string
	PostTypes   []string // For templates
	Location    types.SymbolLocation
	AllFields   map[string]string
}

// GutenbergBlock represents a Gutenberg block registration
type GutenbergBlock struct {
	BlockName      string // e.g., "myplugin/my-block"
	RenderCallback string
	Attributes     []string
	Location       types.SymbolLocation
}

// extractWordPressMetadata extracts plugin/theme/template metadata from doc comments
func (pe *PHPExtractor) extractWordPressMetadata(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []WordPressMetadata {
	var metadata []WordPressMetadata

	pe.traverseNode(root, func(node *sitter.Node) bool {
		if node.Kind() != "comment" {
			return true
		}

		commentText := GetNodeText(node, content)
		if !strings.HasPrefix(commentText, "/**") {
			return true
		}

		// Parse the doc comment for WordPress metadata fields
		fields := pe.parseWordPressDocComment(commentText)
		if len(fields) == 0 {
			return true
		}

		meta := WordPressMetadata{
			Location:  GetNodeLocation(node, fileID),
			AllFields: fields,
		}

		// Determine metadata type and extract relevant fields
		if name, ok := fields["Plugin Name"]; ok {
			meta.Type = "plugin"
			meta.Name = name
			meta.Description = fields["Description"]
			meta.Version = fields["Version"]
			meta.Author = fields["Author"]
			meta.AuthorURI = fields["Author URI"]
			meta.URI = fields["Plugin URI"]
			meta.License = fields["License"]
			meta.TextDomain = fields["Text Domain"]
		} else if name, ok := fields["Theme Name"]; ok {
			meta.Type = "theme"
			meta.Name = name
			meta.Description = fields["Description"]
			meta.Version = fields["Version"]
			meta.Author = fields["Author"]
			meta.AuthorURI = fields["Author URI"]
			meta.URI = fields["Theme URI"]
			meta.License = fields["License"]
			meta.TextDomain = fields["Text Domain"]
		} else if name, ok := fields["Template Name"]; ok {
			meta.Type = "template"
			meta.Name = name
			if postTypes, ok := fields["Template Post Type"]; ok {
				// Split by comma and trim
				parts := strings.Split(postTypes, ",")
				for _, pt := range parts {
					meta.PostTypes = append(meta.PostTypes, strings.TrimSpace(pt))
				}
			}
		} else {
			// No recognized WordPress metadata
			return true
		}

		if meta.Name != "" {
			metadata = append(metadata, meta)

			// Add as a symbol for searchability
			symbolName := fmt.Sprintf("wp:%s:%s", meta.Type, meta.Name)
			localID := builder.AddSymbol(
				symbolName,
				types.SymbolKindModule, // Use Module kind for plugins/themes
				meta.Location,
				nil,
				true,
			)

			if symbol, ok := builder.symbols[localID]; ok {
				symbol.Type = meta.Type
				if meta.Version != "" {
					symbol.Value = meta.Version
				}
				// Build signature with key metadata
				var sigParts []string
				if meta.Description != "" {
					sigParts = append(sigParts, meta.Description)
				}
				if meta.Author != "" {
					sigParts = append(sigParts, "Author: "+meta.Author)
				}
				if meta.Version != "" {
					sigParts = append(sigParts, "Version: "+meta.Version)
				}
				if len(meta.PostTypes) > 0 {
					sigParts = append(sigParts, "Post Types: "+strings.Join(meta.PostTypes, ", "))
				}
				if len(sigParts) > 0 {
					symbol.Signature = strings.Join(sigParts, " | ")
				}
			}
		}

		return true
	})

	return metadata
}

// parseWordPressDocComment parses a WordPress-style doc comment and extracts key-value pairs
func (pe *PHPExtractor) parseWordPressDocComment(comment string) map[string]string {
	fields := make(map[string]string)

	// Remove /** and */ and split into lines
	comment = strings.TrimPrefix(comment, "/**")
	comment = strings.TrimSuffix(comment, "*/")

	lines := strings.Split(comment, "\n")
	for _, line := range lines {
		// Remove leading asterisk and whitespace
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)

		// Look for "Key: Value" pattern
		if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
			key := strings.TrimSpace(line[:colonIdx])
			value := strings.TrimSpace(line[colonIdx+1:])

			// Only capture known WordPress metadata keys
			knownKeys := []string{
				"Plugin Name", "Plugin URI", "Theme Name", "Theme URI",
				"Template Name", "Template Post Type",
				"Description", "Version", "Author", "Author URI",
				"License", "License URI", "Text Domain", "Domain Path",
				"Network", "Requires at least", "Requires PHP", "WC requires at least",
				"WC tested up to", "Woo",
			}

			for _, known := range knownKeys {
				if key == known {
					fields[key] = value
					break
				}
			}
		}
	}

	return fields
}

// extractGutenbergBlocks extracts Gutenberg block registrations from the AST
// Detects: register_block_type, register_block_type_from_metadata, new WP_Block_Type
func (pe *PHPExtractor) extractGutenbergBlocks(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []GutenbergBlock {
	var blocks []GutenbergBlock

	pe.traverseNode(root, func(node *sitter.Node) bool {
		var block *GutenbergBlock

		switch node.Kind() {
		case "function_call_expression":
			block = pe.extractBlockFromFunctionCall(node, content, fileID)
		case "object_creation_expression":
			block = pe.extractBlockFromObjectCreation(node, content, fileID)
		}

		if block != nil && block.BlockName != "" {
			blocks = append(blocks, *block)

			// Add as a symbol for searchability
			symbolName := "block:" + block.BlockName
			localID := builder.AddSymbol(
				symbolName,
				types.SymbolKindClass, // Use Class kind for blocks
				block.Location,
				nil,
				true,
			)

			if symbol, ok := builder.symbols[localID]; ok {
				symbol.Type = "gutenberg_block"
				if block.RenderCallback != "" {
					symbol.Signature = fmt.Sprintf("register_block_type('%s', render: %s)", block.BlockName, block.RenderCallback)
				} else {
					symbol.Signature = fmt.Sprintf("register_block_type('%s')", block.BlockName)
				}
			}
		}

		return true
	})

	return blocks
}

// extractBlockFromFunctionCall extracts block info from register_block_type calls
func (pe *PHPExtractor) extractBlockFromFunctionCall(node *sitter.Node, content []byte, fileID types.FileID) *GutenbergBlock {
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return nil
	}

	funcName := GetNodeText(nameNode, content)
	if funcName != "register_block_type" && funcName != "register_block_type_from_metadata" {
		return nil
	}

	argsNode := FindChildByType(node, "arguments")
	if argsNode == nil {
		return nil
	}

	block := &GutenbergBlock{
		Location: GetNodeLocation(node, fileID),
	}

	// Extract block name from first argument
	argIndex := 0
	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if child == nil || child.Kind() != "argument" {
			continue
		}

		switch argIndex {
		case 0:
			// First argument: block name (string or __DIR__ concatenation)
			block.BlockName = pe.extractBlockNameArgument(child, content)
		case 1:
			// Second argument: options array (for register_block_type)
			if funcName == "register_block_type" {
				block.RenderCallback = pe.extractBlockRenderCallback(child, content)
			}
		}
		argIndex++
	}

	return block
}

// extractBlockFromObjectCreation extracts block info from new WP_Block_Type() calls
func (pe *PHPExtractor) extractBlockFromObjectCreation(node *sitter.Node, content []byte, fileID types.FileID) *GutenbergBlock {
	// Check if it's creating WP_Block_Type
	nameNode := FindChildByType(node, "name")
	if nameNode == nil {
		return nil
	}

	className := GetNodeText(nameNode, content)
	if className != "WP_Block_Type" {
		return nil
	}

	argsNode := FindChildByType(node, "arguments")
	if argsNode == nil {
		return nil
	}

	block := &GutenbergBlock{
		Location: GetNodeLocation(node, fileID),
	}

	// Extract arguments
	argIndex := 0
	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if child == nil || child.Kind() != "argument" {
			continue
		}

		switch argIndex {
		case 0:
			block.BlockName = pe.extractStringArgument(child, content)
		case 1:
			block.RenderCallback = pe.extractBlockRenderCallback(child, content)
		}
		argIndex++
	}

	return block
}

// extractBlockNameArgument extracts block name from argument (handles string and __DIR__ concatenation)
func (pe *PHPExtractor) extractBlockNameArgument(node *sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "string":
			// Direct string: 'myplugin/my-block'
			contentNode := FindChildByType(child, "string_content")
			if contentNode != nil {
				return GetNodeText(contentNode, content)
			}
		case "binary_expression":
			// __DIR__ . '/blocks/my-block' - extract the path part
			// Look for string in the concatenation
			for j := uint(0); j < child.ChildCount(); j++ {
				binChild := child.Child(j)
				if binChild != nil && binChild.Kind() == "string" {
					contentNode := FindChildByType(binChild, "string_content")
					if contentNode != nil {
						path := GetNodeText(contentNode, content)
						// Return the path - caller can handle __DIR__ context
						return "__DIR__" + path
					}
				}
			}
		}
	}
	return ""
}

// extractBlockRenderCallback extracts render_callback from block options array
func (pe *PHPExtractor) extractBlockRenderCallback(node *sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || child.Kind() != "array_creation_expression" {
			continue
		}

		// Search for 'render_callback' key
		for j := uint(0); j < child.ChildCount(); j++ {
			elem := child.Child(j)
			if elem == nil || elem.Kind() != "array_element_initializer" {
				continue
			}

			// Check if this is the 'render_callback' key
			keyNode := FindChildByType(elem, "string")
			if keyNode == nil {
				continue
			}

			contentNode := FindChildByType(keyNode, "string_content")
			if contentNode == nil || GetNodeText(contentNode, content) != "render_callback" {
				continue
			}

			// Get the value (second string in the element)
			stringCount := 0
			for k := uint(0); k < elem.ChildCount(); k++ {
				valChild := elem.Child(k)
				if valChild != nil && valChild.Kind() == "string" {
					stringCount++
					if stringCount == 2 {
						valContentNode := FindChildByType(valChild, "string_content")
						if valContentNode != nil {
							return GetNodeText(valContentNode, content)
						}
					}
				}
			}
		}
	}
	return ""
}
