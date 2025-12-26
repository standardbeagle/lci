package core

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// fillVariableContext populates variable and data context information
func (cle *ContextLookupEngine) fillVariableContext(context *CodeObjectContext) error {
	objectID := context.ObjectID

	// Get global variables accessible to this object
	globalVars, err := cle.getGlobalVariables(objectID)
	if err != nil {
		return fmt.Errorf("failed to get global variables: %w", err)
	}
	context.VariableContext.GlobalVariables = globalVars

	// Get which global variables are actually used by this object
	usedGlobals, err := cle.getUsedGlobalVariables(objectID)
	if err != nil {
		return fmt.Errorf("failed to get used global variables: %w", err)
	}
	context.VariableContext.UsedGlobals = usedGlobals

	// Get class variables (for classes)
	if objectID.Type == types.SymbolTypeClass {
		classVars, err := cle.getClassVariables(objectID)
		if err != nil {
			return fmt.Errorf("failed to get class variables: %w", err)
		}
		context.VariableContext.ClassVariables = classVars
	}

	// Get local variables
	localVars, err := cle.getLocalVariables(objectID)
	if err != nil {
		return fmt.Errorf("failed to get local variables: %w", err)
	}
	context.VariableContext.LocalVariables = localVars

	// Get function parameters
	if objectID.Type == types.SymbolTypeFunction || objectID.Type == types.SymbolTypeMethod {
		params, err := cle.getFunctionParameters(objectID)
		if err != nil {
			return fmt.Errorf("failed to get function parameters: %w", err)
		}
		context.VariableContext.Parameters = params

		// Get return value information
		returnVals, err := cle.getReturnValues(objectID)
		if err != nil {
			return fmt.Errorf("failed to get return values: %w", err)
		}
		context.VariableContext.ReturnValues = returnVals
	}

	return nil
}

// getGlobalVariables finds all global variables accessible to the object
func (cle *ContextLookupEngine) getGlobalVariables(objectID CodeObjectID) ([]VariableInfo, error) {
	var globals []VariableInfo

	// Get all file-level symbols with global scope
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)
	for _, sym := range fileSymbols {
		// Only include top-level variables (global scope)
		if sym.Type == types.SymbolTypeVariable || sym.Type == types.SymbolTypeConstant {
			// Check if it's actually global (no parent scope beyond file/package)
			isGlobal := true
			for _, scope := range sym.ScopeChain {
				if scope.Type != types.ScopeTypeFile && scope.Type != types.ScopeTypePackage && scope.Type != types.ScopeTypeNamespace {
					isGlobal = false
					break
				}
			}

			if isGlobal {
				varInfo := VariableInfo{
					Name: sym.Name,
					Type: sym.TypeInfo,
					Location: types.SymbolLocation{
						FileID: sym.FileID,
						Line:   sym.Line,
						Column: sym.Column,
					},
					IsUsed:    len(sym.IncomingRefs) > 0,
					UseCount:  len(sym.IncomingRefs),
					Scope:     "global",
					IsMutable: sym.IsMutable,
				}
				globals = append(globals, varInfo)
			}
		}
	}

	return globals, nil
}

// getUsedGlobalVariables finds which global variables are actually used by the object
func (cle *ContextLookupEngine) getUsedGlobalVariables(objectID CodeObjectID) ([]VariableInfo, error) {
	var usedGlobals []VariableInfo

	// First get all accessible globals
	allGlobals, err := cle.getGlobalVariables(objectID)
	if err != nil {
		return nil, err
	}

	// Find the target symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		// If we can't find the symbol, return only globals that have incoming refs
		for _, global := range allGlobals {
			if global.IsUsed {
				usedGlobals = append(usedGlobals, global)
			}
		}
		return usedGlobals, nil
	}

	// Check which globals are referenced by this symbol
	globalNameSet := make(map[string]VariableInfo)
	for _, global := range allGlobals {
		globalNameSet[global.Name] = global
	}

	// Look through outgoing references to find global variable usage
	usedNames := make(map[string]bool)
	for _, ref := range targetSymbol.OutgoingRefs {
		if globalInfo, exists := globalNameSet[ref.ReferencedName]; exists {
			// This global is used by the target symbol
			if !usedNames[globalInfo.Name] {
				usedGlobals = append(usedGlobals, globalInfo)
				usedNames[globalInfo.Name] = true
			}
		}
	}

	return usedGlobals, nil
}

// getClassVariables finds class fields and properties
func (cle *ContextLookupEngine) getClassVariables(objectID CodeObjectID) ([]VariableInfo, error) {
	var classVars []VariableInfo

	// Only process class/struct types
	if objectID.Type != types.SymbolTypeClass && objectID.Type != types.SymbolTypeStruct {
		return classVars, nil
	}

	// Find the class/struct symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return classVars, nil
	}

	// Get all symbols in the file and find fields belonging to this class
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)
	for _, sym := range fileSymbols {
		if sym.Type != types.SymbolTypeField && sym.Type != types.SymbolTypeVariable {
			continue
		}

		// Check if this field belongs to the target class (within its line range)
		if sym.Line >= targetSymbol.Line && sym.EndLine <= targetSymbol.EndLine {
			// Also check scope chain to ensure it's within the class
			belongsToClass := false
			for _, scope := range sym.ScopeChain {
				if scope.Name == objectID.Name && (scope.Type == types.ScopeTypeClass || scope.Type == types.ScopeTypeStruct) {
					belongsToClass = true
					break
				}
			}

			if belongsToClass {
				varInfo := VariableInfo{
					Name: sym.Name,
					Type: sym.TypeInfo,
					Location: types.SymbolLocation{
						FileID: sym.FileID,
						Line:   sym.Line,
						Column: sym.Column,
					},
					IsUsed:    len(sym.IncomingRefs) > 0,
					UseCount:  len(sym.IncomingRefs),
					Scope:     "class",
					IsMutable: sym.IsMutable,
				}
				classVars = append(classVars, varInfo)
			}
		}
	}

	return classVars, nil
}

// getLocalVariables finds important local variables within the object
func (cle *ContextLookupEngine) getLocalVariables(objectID CodeObjectID) ([]VariableInfo, error) {
	var localVars []VariableInfo

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return localVars, nil
	}

	// Find the symbol that matches our file
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return localVars, nil
	}

	// Get all symbols in the same file
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Filter for variables that are within this symbol's scope
	for _, sym := range fileSymbols {
		// Only include variables (not parameters, which are handled separately)
		if sym.Type != types.SymbolTypeVariable {
			continue
		}

		// Check if this variable is within the target symbol's scope
		// Variables are local if they're defined within the function's line range
		if objectID.Type == types.SymbolTypeFunction || objectID.Type == types.SymbolTypeMethod {
			// Check if variable is between target symbol's start and end lines
			if sym.Line >= targetSymbol.Line && sym.EndLine <= targetSymbol.EndLine {
				// Count how many times this variable is referenced
				refs := cle.refTracker.GetSymbolReferences(sym.ID, "both")
				useCount := len(refs)

				varInfo := VariableInfo{
					Name: sym.Name,
					Type: sym.Type.String(), // Use symbol type for type info
					Location: types.SymbolLocation{
						FileID: sym.FileID,
						Line:   sym.Line,
						Column: sym.Column,
					},
					IsUsed:    useCount > 0,
					UseCount:  useCount,
					Scope:     "local",
					IsMutable: true, // Most local variables are mutable
				}
				localVars = append(localVars, varInfo)
			}
		}
	}

	return localVars, nil
}

// getFunctionParameters extracts function parameter information
func (cle *ContextLookupEngine) getFunctionParameters(objectID CodeObjectID) ([]VariableInfo, error) {
	var params []VariableInfo

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return params, nil
	}

	// Find the symbol that matches our file
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return params, nil
	}

	// Get all symbols in the same file
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Look for parameters - they are variables defined on the same line as the function
	// Parameters typically appear on the function declaration line or in its scope chain
	for _, sym := range fileSymbols {
		// Parameters might be marked as variables at the function line
		if sym.Type == types.SymbolTypeVariable {
			// Check if this variable is a parameter by checking if it's on the function's declaration line
			// or if it's in the function's scope chain with parameter scope
			isParameter := false

			// Method 1: Check if variable is on the same line as function (for some languages)
			if sym.Line == targetSymbol.Line {
				isParameter = true
			}

			// Method 2: Check scope chain for parameter scope
			for _, scope := range sym.ScopeChain {
				if scope.Type == types.ScopeTypeVariable && scope.Name == sym.Name {
					// This might indicate a parameter
					isParameter = true
					break
				}
			}

			if isParameter {
				// Count usage in function body
				refs := cle.refTracker.GetSymbolReferences(sym.ID, "both")
				useCount := len(refs)

				varInfo := VariableInfo{
					Name: sym.Name,
					Type: sym.Type.String(),
					Location: types.SymbolLocation{
						FileID: sym.FileID,
						Line:   sym.Line,
						Column: sym.Column,
					},
					IsUsed:    useCount > 0,
					UseCount:  useCount,
					Scope:     "parameter",
					IsMutable: false, // Parameters are typically immutable
				}
				params = append(params, varInfo)
			}
		}
	}

	return params, nil
}

// getReturnValues extracts return type information
func (cle *ContextLookupEngine) getReturnValues(objectID CodeObjectID) ([]VariableInfo, error) {
	// Implementing return value extraction using indexed data
	// For now, return empty list - return type analysis is complex and not critical
	// Could potentially extract from function signature in Symbol or analyze return statements
	return []VariableInfo{}, nil
}

// Helper functions

func (cle *ContextLookupEngine) getLanguageFromPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".java":
		return "java"
	default:
		return "unknown"
	}
}

func (cle *ContextLookupEngine) getLocalVariableQuery(language string) string {
	switch language {
	case ".go":
		return `
			[
				(short_var_declaration left: (expression_list (identifier) @var_name))
				(var_declaration (var_spec name: (identifier) @var_name))
				(assignment_statement left: (expression_list (identifier) @var_name))
			]
		`
	case ".js", ".jsx", ".ts", ".tsx":
		return `
			[
				(variable_declaration (variable_declarator name: (identifier) @var_name))
				(lexical_declaration (variable_declarator name: (identifier) @var_name))
			]
		`
	case ".py":
		return `
			(assignment left: (identifier) @var_name)
		`
	default:
		return ""
	}
}

func (cle *ContextLookupEngine) getGlobalVariableQuery(language string) string {
	switch language {
	case ".go":
		return `
			[
				(var_declaration (var_spec name: (identifier) @global_var))
				(const_declaration (const_spec name: (identifier) @const))
			]
		`
	case ".js", ".jsx", ".ts", ".tsx":
		return `
			[
				(variable_declaration (variable_declarator name: (identifier) @global_var))
				(lexical_declaration (variable_declarator name: (identifier) @global_var))
			]
		`
	case ".py":
		return `
			(assignment left: (identifier) @global_var)
		`
	default:
		return ""
	}
}

func (cle *ContextLookupEngine) getClassVariableQuery(language string) string {
	switch language {
	case ".go":
		return `
			(field_declaration name: (field_identifier) @field)
		`
	case ".js", ".jsx", ".ts", ".tsx":
		return `
			(property_definition name: (property_identifier) @property)
		`
	case ".py":
		return `
			(class_definition body: (expression_statement (assignment left: (identifier) @field)))
		`
	default:
		return ""
	}
}

func (cle *ContextLookupEngine) isImportantLocalVariable(varName string, match QueryMatch) bool {
	// Filter out common non-important variables
	unimportant := []string{"i", "j", "k", "x", "y", "temp", "result", "err", "error"}
	for _, name := range unimportant {
		if varName == name {
			return false
		}
	}

	// Consider variables with meaningful names as important
	return len(varName) > 2 && !strings.HasPrefix(varName, "_")
}

func (cle *ContextLookupEngine) isWithinClassScope(match QueryMatch, objectID CodeObjectID) bool {
	// Using indexed data instead of AST parsing
	// This function requires checking if a QueryMatch (from old AST query system)
	// is within a class scope. Since we've migrated away from AST queries,
	// we need to refactor the QueryMatch-based API.

	// Only check structs/classes
	if objectID.Type != types.SymbolTypeStruct && objectID.Type != types.SymbolTypeClass {
		return false
	}

	matchLine := int(match.StartPoint.Row)

	// Find the class/struct symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Check if match line is within the symbol's range
			if matchLine >= sym.Line && matchLine <= sym.EndLine {
				return true
			}

			// Also check if it's within any method of this class
			fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)
			for _, methodSym := range fileSymbols {
				if methodSym.Type == types.SymbolTypeMethod {
					// Check if method belongs to this class
					for _, scope := range methodSym.ScopeChain {
						if scope.Name == objectID.Name && (scope.Type == types.ScopeTypeClass || scope.Type == types.ScopeTypeStruct) {
							// Check if match is within this method
							if matchLine >= methodSym.Line && matchLine <= methodSym.EndLine {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// isWithinStructMethod checks if a line is within a method of the given struct
func (cle *ContextLookupEngine) isWithinStructMethod(node *sitter.Node, content []byte, structName string, targetLine int) bool {
	if node == nil {
		return false
	}

	// Check if this is a method_declaration with matching receiver
	if node.Kind() == "method_declaration" {
		receiverNode := node.ChildByFieldName("receiver")
		if receiverNode != nil {
			// Extract receiver type from parameter_list
			for i := uint(0); i < receiverNode.ChildCount(); i++ {
				child := receiverNode.Child(i)
				if child != nil && child.Kind() == "parameter_declaration" {
					typeNode := child.ChildByFieldName("type")
					if typeNode != nil {
						receiverType := cle.extractReceiverTypeName(typeNode, content)
						if receiverType == structName {
							// Check if target line is within this method
							methodStart := int(node.StartPosition().Row)
							methodEnd := int(node.EndPosition().Row)
							if targetLine >= methodStart && targetLine <= methodEnd {
								return true
							}
						}
					}
				}
			}
		}
	}

	// Recursively check children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if cle.isWithinStructMethod(child, content, structName, targetLine) {
			return true
		}
	}

	return false
}

// extractReceiverTypeName extracts the type name from a receiver type node
func (cle *ContextLookupEngine) extractReceiverTypeName(typeNode *sitter.Node, content []byte) string {
	if typeNode == nil {
		return ""
	}

	kind := typeNode.Kind()

	switch kind {
	case "type_identifier":
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	case "pointer_type":
		// For *User, extract User
		for i := uint(0); i < typeNode.ChildCount(); i++ {
			child := typeNode.Child(i)
			if child != nil && child.Kind() == "type_identifier" {
				return string(content[child.StartByte():child.EndByte()])
			}
		}
	}

	return ""
}

func (cle *ContextLookupEngine) isWithinFunctionScope(match QueryMatch, objectID CodeObjectID) bool {
	// Using indexed data instead of AST parsing
	// This function requires checking if a QueryMatch (from old AST query system)
	// is within a function scope. Since we've migrated away from AST queries,
	// we need to refactor the QueryMatch-based API.

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return false
	}

	matchLine := int(match.StartPoint.Row)

	// Find the function symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Check if match line is within function bounds (signature + body)
			return matchLine >= sym.Line && matchLine <= sym.EndLine
		}
	}

	return false
}

func (cle *ContextLookupEngine) isWithinFunctionSignature(match QueryMatch, objectID CodeObjectID) bool {
	// Using indexed data instead of AST parsing
	// This function requires precise AST information about signature vs body boundaries.
	// Currently we only have Line (signature start) and EndLine (body end) in EnhancedSymbol.
	// For now, we assume signature is only on the first line of the function.

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return false
	}

	matchLine := int(match.StartPoint.Row)

	// Find the function symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Conservative: assume signature is only on the first line
			// In most languages, signature is on the line where function is declared
			return matchLine == sym.Line
		}
	}

	return false
}

func (cle *ContextLookupEngine) inferReturnTypes(objectID CodeObjectID) []string {
	// Using indexed data instead of AST parsing
	// Return types should ideally be captured in EnhancedSymbol.TypeInfo or Signature.
	// For now, we can try to parse them from the Signature field if available.

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return []string{}
	}

	// Find the function symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Try to parse return types from TypeInfo or Signature
			// This is a basic heuristic - actual parsing would be language-specific
			if sym.TypeInfo != "" {
				return []string{sym.TypeInfo}
			}

			// For Go, signature might look like: "func(int, string) (bool, error)"
			// We would need language-specific parsing here
			// For now, return empty to avoid incorrect parsing
			return []string{}
		}
	}

	return []string{}
}

// extractReturnTypes extracts return type declarations from a function node
func (cle *ContextLookupEngine) extractReturnTypes(funcNode *sitter.Node, content []byte) []string {
	if funcNode == nil {
		return []string{}
	}

	var returnTypes []string

	// Look for result field (return types)
	resultNode := funcNode.ChildByFieldName("result")
	if resultNode == nil {
		// No return type
		return []string{}
	}

	// Handle different result node types
	switch resultNode.Kind() {
	case "parameter_list":
		// Multiple return values like (string, error) or named returns (result int, err error)
		for i := uint(0); i < resultNode.ChildCount(); i++ {
			child := resultNode.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "parameter_declaration" {
				// Get the type from parameter_declaration
				typeNode := child.ChildByFieldName("type")
				if typeNode != nil {
					typeName := cle.extractTypeString(typeNode, content)
					if typeName != "" {
						returnTypes = append(returnTypes, typeName)
					}
				}
			}
		}
	default:
		// Single return type like string or *User
		typeName := cle.extractTypeString(resultNode, content)
		if typeName != "" {
			returnTypes = append(returnTypes, typeName)
		}
	}

	return returnTypes
}

// extractTypeString extracts the type as a string from a type node
func (cle *ContextLookupEngine) extractTypeString(typeNode *sitter.Node, content []byte) string {
	if typeNode == nil {
		return ""
	}

	kind := typeNode.Kind()

	switch kind {
	case "type_identifier":
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	case "pointer_type":
		// For *User, include the asterisk
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	case "qualified_type":
		// For io.Reader, include the full qualified name
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	default:
		// For any other type, just return the text representation
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	}
}

// Fallback generic implementations

func (cle *ContextLookupEngine) getGlobalVariablesGeneric(objectID CodeObjectID) ([]VariableInfo, error) {
	// Generic approach using symbol index and simple heuristics
	// This would be a language-agnostic fallback
	return []VariableInfo{}, nil
}

func (cle *ContextLookupEngine) getClassVariablesGeneric(objectID CodeObjectID) ([]VariableInfo, error) {
	// Generic class variable detection
	return []VariableInfo{}, nil
}
