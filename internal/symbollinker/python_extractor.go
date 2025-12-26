package symbollinker

import (
	"errors"
	"strings"

	"github.com/standardbeagle/lci/internal/types"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// PythonExtractor extracts symbols from Python source code
type PythonExtractor struct {
	*BaseExtractor
}

// NewPythonExtractor creates a new Python symbol extractor
func NewPythonExtractor() *PythonExtractor {
	return &PythonExtractor{
		BaseExtractor: NewBaseExtractor("python", []string{".py", ".pyw", ".pyi", ".pyx"}),
	}
}

// ExtractSymbols extracts all symbols from a Python AST
func (pe *PythonExtractor) ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error) {
	if tree == nil {
		return nil, errors.New("tree is nil")
	}

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("root node is nil")
	}

	builder := NewSymbolTableBuilder(fileID, "python")
	scopeManager := NewScopeManager()

	// Track imports for resolution
	imports := pe.extractImports(root, content, builder, fileID)

	// Add imports to the builder
	for _, imp := range imports {
		builder.AddImport(imp)
	}

	// Extract symbols with scope tracking
	pe.extractSymbolsFromNode(root, content, builder, scopeManager, fileID)

	// Build and return the symbol table
	table := builder.Build()

	return table, nil
}

// extractImports extracts all import statements
func (pe *PythonExtractor) extractImports(root *sitter.Node, content []byte, builder *SymbolTableBuilder, fileID types.FileID) []types.ImportInfo {
	var imports []types.ImportInfo

	pe.traverseNode(root, func(node *sitter.Node) bool {
		nodeKind := node.Kind()

		switch nodeKind {
		case "import_statement":
			pe.extractImportStatement(node, content, &imports, fileID)

		case "import_from_statement":
			pe.extractImportFromStatement(node, content, &imports, fileID)
		}

		return true // Continue traversal
	})

	return imports
}

// extractImportStatement extracts simple import statements (import module)
func (pe *PythonExtractor) extractImportStatement(node *sitter.Node, content []byte, imports *[]types.ImportInfo, fileID types.FileID) {
	// Handle: import module1, module2 as alias
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "dotted_name" || child.Kind() == "identifier" {
			imp := types.ImportInfo{
				Location:   GetNodeLocation(child, fileID),
				ImportPath: GetNodeText(child, content),
			}

			// Check for alias (import module as alias)
			if i+1 < node.ChildCount() {
				next := node.Child(i + 1)
				if next != nil && GetNodeText(next, content) == "as" && i+2 < node.ChildCount() {
					aliasNode := node.Child(i + 2)
					if aliasNode != nil {
						imp.Alias = GetNodeText(aliasNode, content)
					}
				}
			}

			// If no alias, use the module name (last part)
			if imp.Alias == "" && imp.ImportPath != "" {
				parts := strings.Split(imp.ImportPath, ".")
				imp.Alias = parts[len(parts)-1]
			}

			*imports = append(*imports, imp)
		} else if child.Kind() == "aliased_import" {
			// Handle aliased imports within the statement
			pe.extractAliasedImport(child, content, imports, fileID)
		}
	}
}

// extractImportFromStatement extracts from-import statements (from module import name)
func (pe *PythonExtractor) extractImportFromStatement(node *sitter.Node, content []byte, imports *[]types.ImportInfo, fileID types.FileID) {
	// Handle: from module import name1, name2 as alias
	// Handle: from .relative import name
	// Handle: from module import *

	var modulePath string
	var importedNames []string
	var isWildcard bool

	// Find the module path
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "dotted_name", "identifier":
			if modulePath == "" {
				modulePath = GetNodeText(child, content)
			}
		case "relative_import":
			// Handle relative imports (.module, ..module)
			modulePath = GetNodeText(child, content)
		case "wildcard_import":
			// from module import *
			isWildcard = true
		case "import_list":
			// Extract imported names from list
			pe.extractImportList(child, content, &importedNames, imports, fileID)
		}
	}

	// Create import entry
	if modulePath != "" {
		imp := types.ImportInfo{
			Location:      GetNodeLocation(node, fileID),
			ImportPath:    modulePath,
			ImportedNames: importedNames,
			IsNamespace:   isWildcard,
		}

		*imports = append(*imports, imp)
	}
}

// extractImportList extracts names from an import list
func (pe *PythonExtractor) extractImportList(listNode *sitter.Node, content []byte, importedNames *[]string, imports *[]types.ImportInfo, fileID types.FileID) {
	for i := uint(0); i < listNode.ChildCount(); i++ {
		child := listNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "identifier":
			name := GetNodeText(child, content)
			if name != "," {
				*importedNames = append(*importedNames, name)
			}
		case "aliased_import":
			pe.extractAliasedImport(child, content, imports, fileID)
		}
	}
}

// extractAliasedImport extracts aliased import (name as alias)
func (pe *PythonExtractor) extractAliasedImport(aliasNode *sitter.Node, content []byte, imports *[]types.ImportInfo, fileID types.FileID) {
	var name, alias string

	for i := uint(0); i < aliasNode.ChildCount(); i++ {
		child := aliasNode.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "identifier" || child.Kind() == "dotted_name" {
			text := GetNodeText(child, content)
			if text != "as" {
				if name == "" {
					name = text
				} else {
					alias = text
				}
			}
		}
	}

	if name != "" {
		imp := types.ImportInfo{
			Location:      GetNodeLocation(aliasNode, fileID),
			ImportPath:    name,
			Alias:         alias,
			ImportedNames: []string{name},
		}
		*imports = append(*imports, imp)
	}
}

// extractSymbolsFromNode recursively extracts symbols from an AST node
func (pe *PythonExtractor) extractSymbolsFromNode(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()

	switch nodeKind {
	case "function_definition":
		pe.extractFunction(node, content, builder, scopeManager, fileID, false)
		return

	case "async_function_definition":
		pe.extractFunction(node, content, builder, scopeManager, fileID, true)
		return

	case "class_definition":
		pe.extractClass(node, content, builder, scopeManager, fileID)
		return

	case "assignment":
		pe.extractAssignment(node, content, builder, scopeManager, fileID)
		return

	case "augmented_assignment":
		pe.extractAugmentedAssignment(node, content, builder, scopeManager, fileID)
		return

	case "global_statement":
		pe.extractGlobalStatement(node, content, builder, scopeManager, fileID)
		return

	case "nonlocal_statement":
		pe.extractNonlocalStatement(node, content, builder, scopeManager, fileID)
		return

	case "for_statement", "while_statement", "if_statement", "with_statement", "try_statement":
		// These create new block scopes
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, nodeKind, startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			pe.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID)
		}

		scopeManager.PopScope()
		return

	case "block":
		// Enter block scope
		startPos := int(node.StartByte())
		endPos := int(node.EndByte())
		scopeManager.PushScope(types.ScopeBlock, "", startPos, endPos)

		// Process children
		for i := uint(0); i < node.ChildCount(); i++ {
			pe.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID)
		}

		// Exit block scope
		scopeManager.PopScope()
		return
	}

	// Process children for other node types
	for i := uint(0); i < node.ChildCount(); i++ {
		pe.extractSymbolsFromNode(node.Child(i), content, builder, scopeManager, fileID)
	}
}

// extractFunction extracts a function definition
func (pe *PythonExtractor) extractFunction(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, isAsync bool) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	funcName := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Determine visibility based on naming convention
	isExported := CommonVisibilityRules.PythonUnderscore(funcName, node, content)

	// Check for decorators
	decorators := pe.extractDecorators(node, content)

	// Get class name from scope if this is a method
	className := ""
	kind := types.SymbolKindFunction
	if scopeManager.CurrentScope().Type == types.ScopeClass {
		className = scopeManager.CurrentScope().Name
		kind = types.SymbolKindMethod
	}

	fullName := funcName
	if className != "" {
		fullName = className + "." + funcName
	}

	// Add function/method symbol
	localID := builder.AddSymbol(
		fullName,
		kind,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		metadata := make(map[string]interface{})
		if isAsync {
			metadata["async"] = true
		}
		if len(decorators) > 0 {
			metadata["decorators"] = decorators
		}

		// Check for special methods
		if strings.HasPrefix(funcName, "__") && strings.HasSuffix(funcName, "__") {
			metadata["dunder"] = true
		}

		// Check for property decorators
		if pe.hasPropertyDecorator(decorators) {
			symbol.Type = "property"
			symbol.Kind = types.SymbolKindProperty
		} else if pe.hasStaticMethodDecorator(decorators) {
			metadata["staticmethod"] = true
		} else if pe.hasClassMethodDecorator(decorators) {
			metadata["classmethod"] = true
		}

		signature := pe.extractFunctionSignature(node, content)
		symbol.Signature = signature
	}

	// Enter function scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeType := types.ScopeFunction
	if kind == types.SymbolKindMethod {
		scopeType = types.ScopeMethod
	}
	scopeManager.PushScope(scopeType, fullName, startPos, endPos)

	// Extract parameters
	pe.extractParameters(node, content, builder, scopeManager, fileID)

	// Process function body
	body := FindChildByType(node, "block")
	if body != nil {
		// Special handling for __init__ methods - extract instance attributes
		if funcName == "__init__" && className != "" {
			pe.extractInstanceAttributes(body, content, builder, scopeManager, fileID, className)
		}

		for i := uint(0); i < body.ChildCount(); i++ {
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID)
		}
	}

	// Exit function scope
	scopeManager.PopScope()
}

// extractClass extracts a class definition
func (pe *PythonExtractor) extractClass(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	nameNode := FindChildByType(node, "identifier")
	if nameNode == nil {
		return
	}

	className := GetNodeText(nameNode, content)
	location := GetNodeLocation(nameNode, fileID)

	// Determine visibility based on naming convention
	isExported := CommonVisibilityRules.PythonUnderscore(className, node, content)

	// Check for decorators
	decorators := pe.extractDecorators(node, content)

	// Determine symbol kind based on inheritance
	symbolKind := types.SymbolKindClass
	baseClasses := pe.extractBaseClasses(node, content)

	// Check if this is an enum
	if pe.isEnumClass(baseClasses) {
		symbolKind = types.SymbolKindEnum
	}

	// Add class symbol
	localID := builder.AddSymbol(
		className,
		symbolKind,
		location,
		scopeManager.CurrentScope(),
		isExported,
	)

	// Add metadata
	if symbol, ok := builder.symbols[localID]; ok {
		if len(decorators) > 0 {
			metadata := make(map[string]interface{})
			metadata["decorators"] = decorators
		}

		// Check for dataclass decorator
		if pe.hasDataclassDecorator(decorators) {
			symbol.Type = "dataclass"
		} else if symbolKind == types.SymbolKindEnum {
			symbol.Type = "enum"
		}

		// Store base classes in the type field if it's an enum
		if symbolKind == types.SymbolKindEnum && len(baseClasses) > 0 {
			symbol.Type = "enum(" + strings.Join(baseClasses, ",") + ")"
		}
	}

	// Enter class scope
	startPos := int(node.StartByte())
	endPos := int(node.EndByte())
	scopeManager.PushScope(types.ScopeClass, className, startPos, endPos)

	// Extract class body
	body := FindChildByType(node, "block")
	if body != nil {
		// First, extract class variables (assignments at class level)
		pe.extractClassVariables(body, content, builder, scopeManager, fileID, className)

		for i := uint(0); i < body.ChildCount(); i++ {
			pe.extractSymbolsFromNode(body.Child(i), content, builder, scopeManager, fileID)
		}
	}

	// Exit class scope
	scopeManager.PopScope()
}

// extractAssignment extracts variable assignments
func (pe *PythonExtractor) extractAssignment(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// Handle different assignment patterns:
	// x = value
	// x, y = values
	// x.attr = value (skip - this is attribute assignment)
	// x[key] = value (skip - this is subscript assignment)

	leftSide := node.Child(0)
	if leftSide == nil {
		return
	}

	switch leftSide.Kind() {
	case "identifier":
		// Simple assignment: x = value
		varName := GetNodeText(leftSide, content)
		location := GetNodeLocation(leftSide, fileID)
		isExported := CommonVisibilityRules.PythonUnderscore(varName, node, content)

		// Determine symbol kind based on context and name
		kind := types.SymbolKindVariable
		currentScope := scopeManager.CurrentScope()

		// Check if we're in an enum class
		if currentScope.Type == types.ScopeClass && pe.isInEnumClass(builder, currentScope) {
			// In enum classes, uppercase assignments are enum members
			if isUppercaseIdentifier(varName) {
				kind = types.SymbolKindEnumMember
			}
		} else if isUppercaseIdentifier(varName) {
			kind = types.SymbolKindConstant
		}

		// Check for type aliases (assignments to type expressions)
		rightSide := node.Child(2) // Skip the '=' operator
		if pe.isTypeAlias(rightSide, content) {
			kind = types.SymbolKindType
		}

		// Check for lambda assignments
		if pe.isLambdaAssignment(rightSide, content) {
			kind = types.SymbolKindFunction
		}

		builder.AddSymbol(
			varName,
			kind,
			location,
			currentScope,
			isExported,
		)

	case "pattern_list", "tuple_pattern":
		// Tuple unpacking: x, y = values
		pe.extractTupleAssignment(leftSide, content, builder, scopeManager, fileID)

	case "list_pattern":
		// List unpacking: [x, y] = values
		pe.extractListAssignment(leftSide, content, builder, scopeManager, fileID)
	}
}

// extractAugmentedAssignment extracts augmented assignments (+=, -=, etc.)
func (pe *PythonExtractor) extractAugmentedAssignment(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// For augmented assignment, we only care about the left side if it's a new variable
	leftSide := node.Child(0)
	if leftSide != nil && leftSide.Kind() == "identifier" {
		varName := GetNodeText(leftSide, content)
		location := GetNodeLocation(leftSide, fileID)
		isExported := CommonVisibilityRules.PythonUnderscore(varName, node, content)

		builder.AddSymbol(
			varName,
			types.SymbolKindVariable,
			location,
			scopeManager.CurrentScope(),
			isExported,
		)
	}
}

// extractGlobalStatement extracts global variable declarations
func (pe *PythonExtractor) extractGlobalStatement(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// global var1, var2, ...
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			varName := GetNodeText(child, content)
			if varName != "global" && varName != "," {
				location := GetNodeLocation(child, fileID)
				isExported := CommonVisibilityRules.PythonUnderscore(varName, node, content)

				// Mark as global variable
				localID := builder.AddSymbol(
					varName,
					types.SymbolKindVariable,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)

				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Type = "global"
				}
			}
		}
	}
}

// extractNonlocalStatement extracts nonlocal variable declarations
func (pe *PythonExtractor) extractNonlocalStatement(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	// nonlocal var1, var2, ...
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			varName := GetNodeText(child, content)
			if varName != "nonlocal" && varName != "," {
				location := GetNodeLocation(child, fileID)

				// Mark as nonlocal variable
				localID := builder.AddSymbol(
					varName,
					types.SymbolKindVariable,
					location,
					scopeManager.CurrentScope(),
					false, // nonlocal variables are not exported
				)

				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Type = "nonlocal"
				}
			}
		}
	}
}

// extractTupleAssignment extracts variables from tuple unpacking
func (pe *PythonExtractor) extractTupleAssignment(tupleNode *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	for i := uint(0); i < tupleNode.ChildCount(); i++ {
		child := tupleNode.Child(i)
		if child != nil && child.Kind() == "identifier" {
			varName := GetNodeText(child, content)
			if varName != "," {
				location := GetNodeLocation(child, fileID)
				isExported := CommonVisibilityRules.PythonUnderscore(varName, child, content)

				builder.AddSymbol(
					varName,
					types.SymbolKindVariable,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)
			}
		}
	}
}

// extractListAssignment extracts variables from list unpacking
func (pe *PythonExtractor) extractListAssignment(listNode *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	for i := uint(0); i < listNode.ChildCount(); i++ {
		child := listNode.Child(i)
		if child != nil && child.Kind() == "identifier" {
			varName := GetNodeText(child, content)
			if varName != "," && varName != "[" && varName != "]" {
				location := GetNodeLocation(child, fileID)
				isExported := CommonVisibilityRules.PythonUnderscore(varName, child, content)

				builder.AddSymbol(
					varName,
					types.SymbolKindVariable,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)
			}
		}
	}
}

// Helper methods

// extractDecorators extracts decorator names from a function or class
func (pe *PythonExtractor) extractDecorators(node *sitter.Node, content []byte) []string {
	var decorators []string

	// Check if this node is inside a decorated_definition
	parent := node.Parent()
	if parent != nil && parent.Kind() == "decorated_definition" {
		// Extract decorators from the decorated_definition parent
		for i := uint(0); i < parent.ChildCount(); i++ {
			child := parent.Child(i)
			if child == node {
				break // Stop when we reach the function/class definition
			}

			if child != nil && child.Kind() == "decorator" {
				decoratorName := pe.extractDecoratorName(child, content)
				if decoratorName != "" {
					decorators = append(decorators, decoratorName)
				}
			}
		}
	}

	return decorators
}

// extractDecoratorsFromDecoratedDef extracts decorators from a decorated definition
func (pe *PythonExtractor) extractDecoratorsFromDecoratedDef(decoratedDef *sitter.Node, content []byte, decorators *[]string) {
	for i := uint(0); i < decoratedDef.ChildCount(); i++ {
		child := decoratedDef.Child(i)
		if child != nil && child.Kind() == "decorator" {
			decoratorName := pe.extractDecoratorName(child, content)
			if decoratorName != "" {
				*decorators = append(*decorators, decoratorName)
			}
		}
	}
}

// extractDecoratorName extracts the name from a decorator node
func (pe *PythonExtractor) extractDecoratorName(decoratorNode *sitter.Node, content []byte) string {
	// Decorators can be: @decorator, @decorator(), @module.decorator, etc.
	for i := uint(0); i < decoratorNode.ChildCount(); i++ {
		child := decoratorNode.Child(i)
		if child != nil {
			switch child.Kind() {
			case "identifier":
				return GetNodeText(child, content)
			case "attribute":
				return GetNodeText(child, content)
			case "call":
				// For @decorator() form, get the function being called
				funcNode := FindChildByType(child, "identifier")
				if funcNode == nil {
					funcNode = FindChildByType(child, "attribute")
				}
				if funcNode != nil {
					return GetNodeText(funcNode, content)
				}
			}
		}
	}
	return ""
}

// hasPropertyDecorator checks if decorators contain @property
func (pe *PythonExtractor) hasPropertyDecorator(decorators []string) bool {
	for _, decorator := range decorators {
		if decorator == "property" {
			return true
		}
	}
	return false
}

// hasStaticMethodDecorator checks if decorators contain @staticmethod
func (pe *PythonExtractor) hasStaticMethodDecorator(decorators []string) bool {
	for _, decorator := range decorators {
		if decorator == "staticmethod" {
			return true
		}
	}
	return false
}

// hasClassMethodDecorator checks if decorators contain @classmethod
func (pe *PythonExtractor) hasClassMethodDecorator(decorators []string) bool {
	for _, decorator := range decorators {
		if decorator == "classmethod" {
			return true
		}
	}
	return false
}

// hasDataclassDecorator checks if decorators contain @dataclass
func (pe *PythonExtractor) hasDataclassDecorator(decorators []string) bool {
	for _, decorator := range decorators {
		if decorator == "dataclass" || strings.Contains(decorator, "dataclass") {
			return true
		}
	}
	return false
}

// extractParameters extracts function parameters
func (pe *PythonExtractor) extractParameters(node *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID) {
	paramsList := FindChildByType(node, "parameters")
	if paramsList == nil {
		return
	}

	for i := uint(0); i < paramsList.ChildCount(); i++ {
		child := paramsList.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "identifier":
			paramName := GetNodeText(child, content)
			if paramName != "," && paramName != "(" && paramName != ")" {
				location := GetNodeLocation(child, fileID)

				builder.AddSymbol(
					paramName,
					types.SymbolKindParameter,
					location,
					scopeManager.CurrentScope(),
					false,
				)
			}

		case "default_parameter":
			// Parameter with default value
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

				// Extract default value
				if symbol, ok := builder.symbols[localID]; ok {
					// Find the default value (after =)
					for j := uint(0); j < child.ChildCount(); j++ {
						valueChild := child.Child(j)
						if valueChild != nil && valueChild != nameNode && GetNodeText(valueChild, content) != "=" {
							symbol.Value = GetNodeText(valueChild, content)
							break
						}
					}
				}
			}

		case "typed_parameter":
			// Parameter with type annotation
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

				// Extract type annotation
				typeNode := FindChildByType(child, "type")
				if typeNode != nil && localID > 0 {
					if symbol, ok := builder.symbols[localID]; ok {
						symbol.Type = GetNodeText(typeNode, content)
					}
				}
			}

		case "list_splat_pattern":
			// *args parameter
			nameNode := FindChildByType(child, "identifier")
			if nameNode != nil {
				paramName := "*" + GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				localID := builder.AddSymbol(
					paramName,
					types.SymbolKindParameter,
					location,
					scopeManager.CurrentScope(),
					false,
				)

				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Type = "variadic"
				}
			}

		case "dictionary_splat_pattern":
			// **kwargs parameter
			nameNode := FindChildByType(child, "identifier")
			if nameNode != nil {
				paramName := "**" + GetNodeText(nameNode, content)
				location := GetNodeLocation(nameNode, fileID)

				localID := builder.AddSymbol(
					paramName,
					types.SymbolKindParameter,
					location,
					scopeManager.CurrentScope(),
					false,
				)

				if symbol, ok := builder.symbols[localID]; ok {
					symbol.Type = "keyword variadic"
				}
			}
		}
	}
}

// extractFunctionSignature extracts a function's signature
func (pe *PythonExtractor) extractFunctionSignature(node *sitter.Node, content []byte) string {
	var signature strings.Builder

	// Get function name
	nameNode := FindChildByType(node, "identifier")
	if nameNode != nil {
		signature.WriteString(GetNodeText(nameNode, content))
	}

	// Get parameters
	params := FindChildByType(node, "parameters")
	if params != nil {
		signature.WriteString(GetNodeText(params, content))
	} else {
		signature.WriteString("()")
	}

	// Get return type annotation if present
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "type" {
			signature.WriteString(" -> ")
			signature.WriteString(GetNodeText(child, content))
			break
		}
	}

	return signature.String()
}

// isUppercaseIdentifier checks if an identifier is all uppercase (constant convention)
func isUppercaseIdentifier(name string) bool {
	if name == "" {
		return false
	}

	hasLetter := false
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
	}

	return hasLetter
}

// traverseNode traverses the AST with a visitor function
func (pe *PythonExtractor) traverseNode(node *sitter.Node, visitor func(*sitter.Node) bool) {
	if node == nil || !visitor(node) {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		pe.traverseNode(node.Child(i), visitor)
	}
}

// extractBaseClasses extracts base class names from class definition
func (pe *PythonExtractor) extractBaseClasses(node *sitter.Node, content []byte) []string {
	var baseClasses []string

	argumentList := FindChildByType(node, "argument_list")
	if argumentList != nil {
		for i := uint(0); i < argumentList.ChildCount(); i++ {
			child := argumentList.Child(i)
			if child != nil {
				switch child.Kind() {
				case "identifier":
					baseClasses = append(baseClasses, GetNodeText(child, content))
				case "attribute":
					// Handle module.ClassName inheritance
					baseClasses = append(baseClasses, GetNodeText(child, content))
				}
			}
		}
	}

	return baseClasses
}

// isEnumClass checks if a class inherits from Enum or IntEnum
func (pe *PythonExtractor) isEnumClass(baseClasses []string) bool {
	for _, baseClass := range baseClasses {
		// Check for direct enum inheritance
		if baseClass == "Enum" || baseClass == "IntEnum" || baseClass == "Flag" || baseClass == "IntFlag" {
			return true
		}
		// Check for module-qualified enum inheritance
		if strings.HasSuffix(baseClass, ".Enum") || strings.HasSuffix(baseClass, ".IntEnum") {
			return true
		}
	}
	return false
}

// isInEnumClass checks if the current scope is inside an enum class
func (pe *PythonExtractor) isInEnumClass(builder *SymbolTableBuilder, scope *types.SymbolScope) bool {
	if scope.Type != types.ScopeClass {
		return false
	}

	// Find the class symbol in the builder
	for _, symbol := range builder.symbols {
		if symbol.Name == scope.Name && symbol.Kind == types.SymbolKindEnum {
			return true
		}
	}

	return false
}

// isTypeAlias checks if the right side of an assignment is a type expression
func (pe *PythonExtractor) isTypeAlias(rightSide *sitter.Node, content []byte) bool {
	if rightSide == nil {
		return false
	}

	rightText := GetNodeText(rightSide, content)

	// Common type alias patterns in Python
	typePatterns := []string{
		"Dict[", "List[", "Set[", "Tuple[", "Optional[", "Union[",
		"Callable[", "Type[", "ClassVar[", "Final[", "Literal[",
		"Protocol", "TypeVar", "Generic[", "Any", "NoReturn",
	}

	for _, pattern := range typePatterns {
		if strings.Contains(rightText, pattern) {
			return true
		}
	}

	// Also check for simple type references (ClassName, module.ClassName)
	if rightSide.Kind() == "identifier" || rightSide.Kind() == "attribute" {
		// If it's a capitalized identifier, might be a type alias
		parts := strings.Split(rightText, ".")
		lastPart := parts[len(parts)-1]
		if len(lastPart) > 0 && lastPart[0] >= 'A' && lastPart[0] <= 'Z' {
			return true
		}
	}

	return false
}

// isLambdaAssignment checks if the right side of an assignment is a lambda expression
func (pe *PythonExtractor) isLambdaAssignment(rightSide *sitter.Node, content []byte) bool {
	if rightSide == nil {
		return false
	}

	return rightSide.Kind() == "lambda"
}

// extractInstanceAttributes extracts instance attributes from __init__ method
func (pe *PythonExtractor) extractInstanceAttributes(body *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, className string) {
	pe.traverseNode(body, func(node *sitter.Node) bool {
		if node.Kind() == "assignment" {
			// Look for self.attribute = value assignments
			leftSide := node.Child(0)
			if leftSide != nil && leftSide.Kind() == "attribute" {
				// Check if it starts with "self."
				objectNode := leftSide.Child(0)
				if objectNode != nil && objectNode.Kind() == "identifier" {
					objectName := GetNodeText(objectNode, content)
					if objectName == "self" {
						// Extract the attribute name (it's the last identifier in the attribute)
						attrNode := leftSide.Child(2) // self.name -> child 2 is "name"
						if attrNode != nil && attrNode.Kind() == "identifier" {
							attrName := GetNodeText(attrNode, content)
							location := GetNodeLocation(attrNode, fileID)
							isExported := CommonVisibilityRules.PythonUnderscore(attrName, node, content)

							fullName := className + "." + attrName
							builder.AddSymbol(
								fullName,
								types.SymbolKindAttribute,
								location,
								scopeManager.CurrentScope().Parent, // Use class scope, not method scope
								isExported,
							)
						}
					}
				}
			}
		}
		return true // Continue traversal
	})
}

// extractClassVariables extracts class variables (assignments at class level)
func (pe *PythonExtractor) extractClassVariables(body *sitter.Node, content []byte, builder *SymbolTableBuilder, scopeManager *ScopeManager, fileID types.FileID, className string) {
	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}

		var assignmentNode *sitter.Node

		if child.Kind() == "assignment" {
			assignmentNode = child
		} else if child.Kind() == "expression_statement" {
			// Check if the expression statement contains an assignment
			for j := uint(0); j < child.ChildCount(); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Kind() == "assignment" {
					assignmentNode = grandchild
					break
				}
			}
		}

		if assignmentNode != nil {
			// Look for simple identifier assignments at class level
			leftSide := assignmentNode.Child(0)
			if leftSide != nil && leftSide.Kind() == "identifier" {
				varName := GetNodeText(leftSide, content)
				location := GetNodeLocation(leftSide, fileID)
				isExported := CommonVisibilityRules.PythonUnderscore(varName, assignmentNode, content)

				fullName := className + "." + varName

				// Determine symbol kind based on context
				kind := types.SymbolKindAttribute
				if isUppercaseIdentifier(varName) {
					// Check if we're in an enum class
					if pe.isInEnumClass(builder, scopeManager.CurrentScope()) {
						kind = types.SymbolKindEnumMember
					} else {
						kind = types.SymbolKindConstant
					}
				}

				builder.AddSymbol(
					fullName,
					kind,
					location,
					scopeManager.CurrentScope(),
					isExported,
				)
			}
		}
	}
}
