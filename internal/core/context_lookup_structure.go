package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// fillStructureContext populates structural information about the code object
func (cle *ContextLookupEngine) fillStructureContext(context *CodeObjectContext) error {
	objectID := context.ObjectID

	// Get file and module information
	filePath, module, pkg, err := cle.getFileInfo(objectID)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	context.StructureContext.FilePath = filePath
	context.StructureContext.Module = module
	context.StructureContext.Package = pkg

	// Get imports
	imports, err := cle.getImports(objectID)
	if err != nil {
		return fmt.Errorf("failed to get imports: %w", err)
	}
	context.StructureContext.Imports = imports

	// Get exports
	exports, err := cle.getExports(objectID)
	if err != nil {
		return fmt.Errorf("failed to get exports: %w", err)
	}
	context.StructureContext.Exports = exports

	// Get interface implementations
	interfaces, err := cle.getInterfaceImplementations(objectID)
	if err != nil {
		return fmt.Errorf("failed to get interface implementations: %w", err)
	}
	context.StructureContext.InterfaceImplementation = interfaces

	// Get inheritance chain
	inheritance, err := cle.getInheritanceChain(objectID)
	if err != nil {
		return fmt.Errorf("failed to get inheritance chain: %w", err)
	}
	context.StructureContext.InheritanceChain = inheritance

	// Determine composition pattern
	composition := cle.determineCompositionPattern(objectID)
	context.StructureContext.CompositionPattern = composition

	return nil
}

// getFileInfo extracts file path, module, and package information
func (cle *ContextLookupEngine) getFileInfo(objectID CodeObjectID) (string, string, string, error) {
	// Get file path from file service
	filePath := cle.fileService.GetPathForFileID(objectID.FileID)
	if filePath == "" {
		return "", "", "", fmt.Errorf("file path not found for file ID %d", objectID.FileID)
	}
	module := extractModuleFromPath(filePath)
	pkg := extractPackageFromPath(filePath)

	return filePath, module, pkg, nil
}

// getImports finds all imports used by the file containing the object
func (cle *ContextLookupEngine) getImports(objectID CodeObjectID) ([]ImportInfo, error) {
	var imports []ImportInfo

	// Get all enhanced symbols in the file to find import references
	enhancedSyms := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Collect all import references from all symbols in the file
	importRefs := make(map[string]*ImportInfo) // keyed by module path

	for _, sym := range enhancedSyms {
		// Check outgoing references for imports
		for _, ref := range sym.OutgoingRefs {
			if ref.Type == types.RefTypeImport {
				modulePath := ref.ReferencedName
				if modulePath == "" {
					continue
				}

				// Create or update import info
				if _, exists := importRefs[modulePath]; !exists {
					importRefs[modulePath] = &ImportInfo{
						ModulePath:  modulePath,
						ImportStyle: "default",
						IsUsed:      true, // If there's a reference, it's used
					}
				}
			}
		}
	}

	// Convert map to slice
	for _, imp := range importRefs {
		imports = append(imports, *imp)
	}

	return imports, nil
}

// getExports finds all exports from the file containing the object
func (cle *ContextLookupEngine) getExports(objectID CodeObjectID) ([]ExportInfo, error) {
	var exports []ExportInfo

	// Get all enhanced symbols in the file
	enhancedSyms := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Find all exported symbols
	for _, sym := range enhancedSyms {
		if sym.IsExported {
			exportInfo := ExportInfo{
				Name:        sym.Name,
				Type:        sym.Type.String(),
				ExportStyle: "named", // Default for most languages
			}

			// Check who uses this export by looking at incoming references
			var usedBy []string
			for _, ref := range sym.IncomingRefs {
				if ref.FileID != objectID.FileID {
					// Referenced from another file
					usedBy = append(usedBy, fmt.Sprintf("file:%d", ref.FileID))
				}
			}
			exportInfo.UsedBy = usedBy

			exports = append(exports, exportInfo)
		}
	}

	return exports, nil
}

// getInterfaceImplementations finds interfaces that this object implements
func (cle *ContextLookupEngine) getInterfaceImplementations(objectID CodeObjectID) ([]InterfaceInfo, error) {
	var interfaces []InterfaceInfo

	// Only applicable to classes and structs
	if objectID.Type != types.SymbolTypeClass {
		return interfaces, nil
	}

	// Find the symbol's enhanced data
	enhancedSyms := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSym *types.EnhancedSymbol
	for _, sym := range enhancedSyms {
		if sym != nil && sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSym = sym
			break
		}
	}

	if targetSym == nil {
		return interfaces, nil
	}

	// Look for inheritance references to interfaces
	for _, ref := range targetSym.OutgoingRefs {
		if ref.Type == types.RefTypeInheritance {
			// This is a parent class/interface reference
			interfaceInfo := InterfaceInfo{
				InterfaceID: CodeObjectID{
					FileID:   ref.FileID,
					Name:     ref.ReferencedName,
					Type:     types.SymbolTypeInterface,
					SymbolID: fmt.Sprintf("%d", ref.TargetSymbol),
				},
				IsFullyImplemented: true,       // Assume implemented (would need deeper analysis)
				Methods:            []string{}, // Would need to query interface methods
			}
			interfaces = append(interfaces, interfaceInfo)
		}
	}

	return interfaces, nil
}

// getInheritanceChain finds the inheritance hierarchy for the object
func (cle *ContextLookupEngine) getInheritanceChain(objectID CodeObjectID) ([]ObjectReference, error) {
	var chain []ObjectReference

	// Only applicable to classes and structs
	if objectID.Type != types.SymbolTypeClass {
		return chain, nil
	}

	// Find the symbol's enhanced data
	enhancedSyms := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSym *types.EnhancedSymbol
	for _, sym := range enhancedSyms {
		if sym != nil && sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSym = sym
			break
		}
	}

	if targetSym == nil {
		return chain, nil
	}

	// Build inheritance chain from OutgoingRefs with RefTypeInheritance
	for _, ref := range targetSym.OutgoingRefs {
		if ref.Type == types.RefTypeInheritance {
			parentRef := ObjectReference{
				ObjectID: CodeObjectID{
					FileID:   ref.FileID,
					Name:     ref.ReferencedName,
					Type:     types.SymbolTypeClass,
					SymbolID: fmt.Sprintf("%d", ref.TargetSymbol),
				},
				Location: types.SymbolLocation{
					FileID: ref.FileID,
					Line:   ref.Line,
					Column: ref.Column,
				},
				Context:    "inheritance",
				Confidence: 0.9,
			}
			chain = append(chain, parentRef)
		}
	}

	return chain, nil
}

// determineCompositionPattern analyzes how the object is composed
func (cle *ContextLookupEngine) determineCompositionPattern(objectID CodeObjectID) string {
	// Use indexed data to determine composition pattern
	// Look for composition indicators from enhanced symbol data
	if cle.hasDependencyInjection(objectID) {
		return "dependency-injection"
	}
	if cle.hasCompositionPattern(objectID) {
		return "composition"
	}
	if cle.hasInheritancePattern(objectID) {
		return "inheritance"
	}
	if cle.hasFactoryPattern(objectID) {
		return "factory"
	}
	if cle.hasSingletonPattern(objectID) {
		return "singleton"
	}

	return "simple"
}

// Helper functions

func extractModuleFromPath(filePath string) string {
	// Extract module name from file path
	// This would typically be the root directory or go module name
	dir := filepath.Dir(filePath)
	if strings.Contains(dir, "src") {
		parts := strings.Split(dir, "src")
		if len(parts) > 1 {
			return strings.TrimPrefix(parts[1], "/")
		}
	}
	return filepath.Base(dir)
}

func extractPackageFromPath(filePath string) string {
	// Extract package name from file path
	// For Go, this would be the directory containing the file
	return filepath.Base(filepath.Dir(filePath))
}

func getImportQuery(language string) string {
	switch language {
	case "go":
		return `
			(import_declaration
				path: (interpreted_string_literal) @import_path
				name: (identifier)? @import_name)
		`
	case "javascript", "typescript":
		return `
			[
				(import_statement
					source: (string) @import_path
					import_kind: "type")
				(import_statement
					source: (string) @import_path
					(import_specifier name: (identifier) @import_name))
				(import_statement
					source: (string) @import_path
					(namespace_import (identifier) @alias))
				(import_statement
					source: (string) @import_path
					(named_imports (import_specifier
						name: (identifier) @import_name
						alias: (identifier)? @alias)))
			]
		`
	case "python":
		return `
			(import_statement name: (dotted_name) @import_path)
			(import_from_statement
				module_name: (dotted_name) @import_path
				name: (dotted_name) @import_name)
		`
	default:
		return ""
	}
}

func getExportQuery(language string) string {
	switch language {
	case "go":
		return `
			(exported_declaration
				name: (identifier) @export_name)
		`
	case "javascript", "typescript":
		return `
			[
				(export_statement declaration: (function_declaration name: (identifier) @export_name))
				(export_statement declaration: (class_declaration name: (identifier) @export_name))
				(export_statement declaration: (lexical_declaration (variable_declarator name: (identifier) @export_name)))
				(export_specifier name: (identifier) @export_name)
				(export_default_declaration (identifier) @export_name)
			]
		`
	case "python":
		return `
			(expression_statement (assignment left: (identifier) @export_name))
		`
	default:
		return ""
	}
}

func getInterfaceImplementationQuery(language string) string {
	switch language {
	case "go":
		return `
			(type_spec
				name: (type_identifier) @type_name
				type: (struct_type)
				(#match? @type_name ".*"))
		`
	case "java":
		return `
			(class_declaration
				name: (identifier) @class_name
				superclass: (type_identifier) @parent_class
				interfaces: (super_interfaces (type_list (type_identifier) @interface_name)))
		`
	default:
		return ""
	}
}

func getInheritanceQuery(language string) string {
	switch language {
	case "java":
		return `
			(class_declaration
				superclass: (type_identifier) @parent_class)
		`
	case "cpp":
		return `
			(class_specifier
				name: (type_identifier) @class_name
				base_class_clause: (base_class_list (type_identifier) @parent_class))
		`
	default:
		return ""
	}
}

func determineExportType(captureName string) string {
	if strings.Contains(captureName, "function") {
		return "function"
	}
	if strings.Contains(captureName, "class") {
		return "class"
	}
	if strings.Contains(captureName, "variable") {
		return "variable"
	}
	return "unknown"
}

func determineExportStyle(captureName string) string {
	if strings.Contains(captureName, "default") {
		return "default"
	}
	if strings.Contains(captureName, "named") {
		return "named"
	}
	return "default"
}

func (cle *ContextLookupEngine) isImportUsed(fileID types.FileID, modulePath string) bool {
	// Using ReferenceTracker indexed data to check if import has outgoing references
	// For now, conservatively return true (assume imports are used)
	// This prevents false positives about unused imports

	// Check if there are any import references for this module path
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(fileID)
	for _, sym := range fileSymbols {
		for _, ref := range sym.OutgoingRefs {
			if ref.Type == types.RefTypeImport && ref.ReferencedName == modulePath {
				// Found an import - check if there are any calls to symbols from this module
				// If we can't determine, assume it's used (conservative)
				return true
			}
		}
	}

	// No import found, or unable to determine usage - conservatively return true
	return true
}

// findImportAlias finds the import declaration and returns the name it's imported as
// Returns: alias name, or package name, or "_" for underscore imports, or "." for dot imports
func (cle *ContextLookupEngine) findImportAlias(node *sitter.Node, content []byte, modulePath string) string {
	if node == nil {
		return ""
	}

	nodeKind := node.Kind()

	// Look for import_spec nodes
	if nodeKind == "import_spec" {
		// Get the path (string literal)
		var pathNode *sitter.Node
		var nameNode *sitter.Node

		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "interpreted_string_literal" {
				pathNode = child
			} else if child.Kind() == "package_identifier" || child.Kind() == "dot" || child.Kind() == "blank_identifier" {
				nameNode = child
			}
		}

		if pathNode != nil {
			// Extract the import path (remove quotes)
			pathText := string(content[pathNode.StartByte():pathNode.EndByte()])
			pathText = strings.Trim(pathText, "\"")

			// Check if this is the import we're looking for
			if pathText == modulePath {
				// Found it! Now determine the name
				if nameNode != nil {
					// Explicit alias (including "_" and ".")
					return string(content[nameNode.StartByte():nameNode.EndByte()])
				} else {
					// No alias, use the last part of the path
					parts := strings.Split(modulePath, "/")
					return parts[len(parts)-1]
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if alias := cle.findImportAlias(child, content, modulePath); alias != "" {
			return alias
		}
	}

	return ""
}

// hasPackageReference checks if the package name is referenced anywhere in the code
func (cle *ContextLookupEngine) hasPackageReference(node *sitter.Node, content []byte, packageName string) bool {
	if node == nil {
		return false
	}

	nodeKind := node.Kind()

	// Look for selector_expression: packageName.Something
	if nodeKind == "selector_expression" {
		// Selector has two parts: operand and field
		// We want to check if operand is an identifier matching packageName
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "identifier" {
				idText := string(content[child.StartByte():child.EndByte()])
				if idText == packageName {
					return true // Found a reference!
				}
				break // Only check the first identifier (the package part)
			}
		}
	}

	// Also check for qualified_type: packageName.TypeName (in type declarations)
	if nodeKind == "qualified_type" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "package_identifier" {
				idText := string(content[child.StartByte():child.EndByte()])
				if idText == packageName {
					return true
				}
				break
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if cle.hasPackageReference(child, content, packageName) {
			return true
		}
	}

	return false
}

func (cle *ContextLookupEngine) findExportUsage(fileID types.FileID, exportName string) []string {
	// Finding all files that use this export using indexed data
	// This would search across the codebase for imports and usage
	return []string{} // Placeholder
}

func (cle *ContextLookupEngine) isInterfaceFullyImplemented(objectID CodeObjectID, interfaceID CodeObjectID) bool {
	// Check if the object fully implements all interface methods

	// Using ReferenceTracker indexed inheritance relationships
	// For now, check if there's an inheritance reference between these types

	// Find the object symbol
	objectSymbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, objSym := range objectSymbols {
		if objSym.FileID == objectID.FileID && objSym.Type == objectID.Type {
			// Check outgoing inheritance references
			for _, ref := range objSym.OutgoingRefs {
				if ref.Type == types.RefTypeInheritance && ref.ReferencedName == interfaceID.Name {
					// Found inheritance relationship - assume fully implemented
					return true
				}
			}
		}
	}

	// No inheritance relationship found
	return false
}

// findInterfaceNode finds an interface declaration by name
func (cle *ContextLookupEngine) findInterfaceNode(node *sitter.Node, content []byte, interfaceName string) *sitter.Node {
	if node == nil {
		return nil
	}

	// Check if this is a type_declaration with an interface
	if node.Kind() == "type_declaration" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					name := string(content[nameNode.StartByte():nameNode.EndByte()])
					if name == interfaceName {
						typeNode := child.ChildByFieldName("type")
						if typeNode != nil && typeNode.Kind() == "interface_type" {
							return typeNode
						}
					}
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if result := cle.findInterfaceNode(child, content, interfaceName); result != nil {
			return result
		}
	}

	return nil
}

// extractInterfaceMethods extracts method names from an interface
func (cle *ContextLookupEngine) extractInterfaceMethods(interfaceNode *sitter.Node, content []byte) []string {
	var methods []string

	if interfaceNode == nil {
		return methods
	}

	// Look for method_spec nodes within the interface
	for i := uint(0); i < interfaceNode.ChildCount(); i++ {
		child := interfaceNode.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "method_spec" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				methodName := string(content[nameNode.StartByte():nameNode.EndByte()])
				methods = append(methods, methodName)
			}
		}
	}

	return methods
}

// getObjectMethods returns all methods implemented by an object
func (cle *ContextLookupEngine) getObjectMethods(objectID CodeObjectID) []string {
	var methods []string

	// Using ReferenceTracker indexed data to find methods
	// For now, find all method symbols in the file that belong to this object

	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)
	for _, sym := range fileSymbols {
		if sym.Type != types.SymbolTypeMethod {
			continue
		}

		// Check if this method belongs to our object via scope chain
		for _, scope := range sym.ScopeChain {
			if scope.Name == objectID.Name {
				methods = append(methods, sym.Name)
				break
			}
		}
	}

	return methods
}

// findMethodsForStruct finds all methods for a given struct
func (cle *ContextLookupEngine) findMethodsForStruct(node *sitter.Node, content []byte, structName string) []string {
	var methods []string

	if node == nil {
		return methods
	}

	// Check if this is a method_declaration
	if node.Kind() == "method_declaration" {
		receiverNode := node.ChildByFieldName("receiver")
		if receiverNode != nil {
			// Check if receiver type matches our struct
			for i := uint(0); i < receiverNode.ChildCount(); i++ {
				child := receiverNode.Child(i)
				if child != nil && child.Kind() == "parameter_declaration" {
					typeNode := child.ChildByFieldName("type")
					if typeNode != nil {
						receiverType := cle.extractReceiverTypeName(typeNode, content)
						if receiverType == structName {
							// Get method name
							nameNode := node.ChildByFieldName("name")
							if nameNode != nil {
								methodName := string(content[nameNode.StartByte():nameNode.EndByte()])
								methods = append(methods, methodName)
							}
						}
					}
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childMethods := cle.findMethodsForStruct(child, content, structName)
		methods = append(methods, childMethods...)
	}

	return methods
}

func deduplicateImports(imports []ImportInfo) []ImportInfo {
	seen := make(map[string]bool)
	var unique []ImportInfo

	for _, imp := range imports {
		key := imp.ModulePath + ":" + imp.ImportName
		if !seen[key] {
			seen[key] = true
			unique = append(unique, imp)
		}
	}

	return unique
}

// Pattern detection functions

func (cle *ContextLookupEngine) hasDependencyInjection(objectID CodeObjectID) bool {
	// Check for constructor injection patterns
	// DI is indicated by:
	// - Constructor function (New* naming)
	// - Parameters that look like dependencies (capitalized types)
	// - Parameters assigned to struct fields

	// Only check functions
	if objectID.Type != types.SymbolTypeFunction {
		return false
	}

	// Check if it's a constructor function
	if !strings.HasPrefix(objectID.Name, "New") {
		return false
	}

	// Get parameter count
	paramCount := cle.countParameters(objectID)

	// If it has 1+ parameters, it likely uses DI
	// (assuming New* functions with parameters inject dependencies)
	return paramCount >= 1
}

func (cle *ContextLookupEngine) hasCompositionPattern(objectID CodeObjectID) bool {
	// Check for composition over inheritance patterns
	// In Go, composition is indicated by struct fields with non-primitive types
	// (i.e., capitalized type names indicating custom types)

	// Only check structs
	if objectID.Type != types.SymbolTypeStruct {
		return false
	}

	// Using ReferenceTracker indexed data to check struct fields
	// For now, check if struct has any field members (composition indicator)

	// Find the struct symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == types.SymbolTypeStruct {
			// Get file symbols to find fields within the struct
			fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

			// Count fields within this struct's bounds
			fieldCount := 0
			for _, fieldSym := range fileSymbols {
				if fieldSym.Type == types.SymbolTypeField || fieldSym.Type == types.SymbolTypeVariable {
					// Check if within struct bounds
					if fieldSym.Line >= sym.Line && fieldSym.EndLine <= sym.EndLine {
						fieldCount++
					}
				}
			}

			// If has fields, likely uses composition
			return fieldCount > 0
		}
	}

	return false
}

// findStructNode finds a struct declaration by name
func (cle *ContextLookupEngine) findStructNode(node *sitter.Node, content []byte, structName string) *sitter.Node {
	if node == nil {
		return nil
	}

	// Check if this is a type_declaration with a struct
	if node.Kind() == "type_declaration" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			// Look for type_spec
			if child.Kind() == "type_spec" {
				// Check if name matches
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					name := string(content[nameNode.StartByte():nameNode.EndByte()])
					if name == structName {
						// Check if type is struct_type
						typeNode := child.ChildByFieldName("type")
						if typeNode != nil && typeNode.Kind() == "struct_type" {
							return typeNode
						}
					}
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if result := cle.findStructNode(child, content, structName); result != nil {
			return result
		}
	}

	return nil
}

// hasComposedFields checks if a struct has fields with custom (capitalized) types
func (cle *ContextLookupEngine) hasComposedFields(structNode *sitter.Node, content []byte) bool {
	if structNode == nil {
		return false
	}

	// Look for field_declaration_list
	for i := uint(0); i < structNode.ChildCount(); i++ {
		child := structNode.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "field_declaration_list" {
			// Check each field
			for j := uint(0); j < child.ChildCount(); j++ {
				field := child.Child(j)
				if field == nil {
					continue
				}

				if field.Kind() == "field_declaration" {
					// Get the type of the field
					typeNode := field.ChildByFieldName("type")
					if typeNode != nil {
						typeName := cle.extractTypeName(typeNode, content)
						if typeName != "" && unicode.IsUpper(rune(typeName[0])) {
							// Capitalized type = custom type = composition
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// extractTypeName extracts the type name from a type node
func (cle *ContextLookupEngine) extractTypeName(typeNode *sitter.Node, content []byte) string {
	if typeNode == nil {
		return ""
	}

	kind := typeNode.Kind()

	// Handle different type node kinds
	switch kind {
	case "type_identifier":
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	case "pointer_type":
		// For pointer types like *Repository, extract the base type
		for i := uint(0); i < typeNode.ChildCount(); i++ {
			child := typeNode.Child(i)
			if child != nil && child.Kind() == "type_identifier" {
				return string(content[child.StartByte():child.EndByte()])
			}
		}
	case "qualified_type":
		// For qualified types like pkg.Type, extract the type
		for i := uint(0); i < typeNode.ChildCount(); i++ {
			child := typeNode.Child(i)
			if child != nil && child.Kind() == "type_identifier" {
				return string(content[child.StartByte():child.EndByte()])
			}
		}
	}

	return ""
}

func (cle *ContextLookupEngine) hasInheritancePattern(objectID CodeObjectID) bool {
	// Check for inheritance patterns
	// In Go, this is struct embedding (anonymous fields)
	// e.g., type Derived struct { Base; field int }

	// Only check structs
	if objectID.Type != types.SymbolTypeStruct {
		return false
	}

	// Using ReferenceTracker indexed inheritance references
	// For now, check if struct has outgoing inheritance references

	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == types.SymbolTypeStruct {
			// Check for inheritance (embedding) references
			for _, ref := range sym.OutgoingRefs {
				if ref.Type == types.RefTypeInheritance {
					return true
				}
			}
		}
	}

	return false
}

// hasEmbeddedFields checks if a struct has embedded (anonymous) fields
func (cle *ContextLookupEngine) hasEmbeddedFields(structNode *sitter.Node, content []byte) bool {
	if structNode == nil {
		return false
	}

	// Look for field_declaration_list
	for i := uint(0); i < structNode.ChildCount(); i++ {
		child := structNode.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "field_declaration_list" {
			// Check each field
			for j := uint(0); j < child.ChildCount(); j++ {
				field := child.Child(j)
				if field == nil {
					continue
				}

				if field.Kind() == "field_declaration" {
					// Check if this is an embedded field (no name, just type)
					// Embedded fields have no "name" field, only "type"
					nameNode := field.ChildByFieldName("name")
					typeNode := field.ChildByFieldName("type")

					if nameNode == nil && typeNode != nil {
						// This is an embedded field (inheritance-like pattern)
						return true
					}
				}
			}
		}
	}

	return false
}

func (cle *ContextLookupEngine) hasFactoryPattern(objectID CodeObjectID) bool {
	// Check for factory pattern implementation
	// Factory pattern is indicated by:
	// - Function name starting with Create*, Make*, or Build*
	// - Returns a pointer or interface type

	// Only check functions
	if objectID.Type != types.SymbolTypeFunction {
		return false
	}

	// Check for factory naming patterns
	factoryPrefixes := []string{"Create", "Make", "Build"}
	hasFactoryNaming := false
	for _, prefix := range factoryPrefixes {
		if strings.HasPrefix(objectID.Name, prefix) {
			hasFactoryNaming = true
			break
		}
	}

	return hasFactoryNaming
}

func (cle *ContextLookupEngine) hasSingletonPattern(objectID CodeObjectID) bool {
	// Check for singleton pattern implementation
	// Singleton pattern is indicated by:
	// - Function name matching Instance, GetInstance, Shared, or GetSingleton
	// - Typically returns a pointer type
	// - No parameters (accessor function)

	// Only check functions
	if objectID.Type != types.SymbolTypeFunction {
		return false
	}

	// Check for singleton naming patterns
	singletonNames := []string{"Instance", "GetInstance", "Shared", "GetShared", "GetSingleton", "Singleton"}
	for _, name := range singletonNames {
		if objectID.Name == name {
			return true
		}
	}

	return false
}

// Fallback generic implementations

func (cle *ContextLookupEngine) getImportsGeneric(objectID CodeObjectID) ([]ImportInfo, error) {
	// Generic import detection using heuristics
	return []ImportInfo{}, nil
}

func (cle *ContextLookupEngine) getExportsGeneric(objectID CodeObjectID) ([]ExportInfo, error) {
	// Generic export detection using heuristics
	return []ExportInfo{}, nil
}

func (cle *ContextLookupEngine) getInterfaceImplementationsGeneric(objectID CodeObjectID) ([]InterfaceInfo, error) {
	// Generic interface detection using heuristics
	return []InterfaceInfo{}, nil
}

func (cle *ContextLookupEngine) getInheritanceChainGeneric(objectID CodeObjectID) ([]ObjectReference, error) {
	// Generic inheritance detection using heuristics
	return []ObjectReference{}, nil
}
