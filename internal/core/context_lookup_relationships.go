package core

import (
	"fmt"
	"math"
	"sync/atomic"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// fillBasicInfo populates basic identification information
func (cle *ContextLookupEngine) fillBasicInfo(ctx *CodeObjectContext) error {
	// Use ReferenceTracker to get EnhancedSymbol with all metadata
	enhancedSyms := cle.refTracker.FindSymbolsByName(ctx.ObjectID.Name)
	var targetSym *types.EnhancedSymbol
	for _, sym := range enhancedSyms {
		if sym != nil && sym.FileID == ctx.ObjectID.FileID && sym.Type == ctx.ObjectID.Type {
			targetSym = sym
			break
		}
	}

	if targetSym == nil {
		return fmt.Errorf("symbol not found in reference tracker: %s", ctx.ObjectID.Name)
	}

	// Set basic information from EnhancedSymbol
	ctx.Location = types.SymbolLocation{
		FileID: targetSym.FileID,
		Line:   targetSym.Line,
		Column: targetSym.Column,
	}

	// Get signature and documentation from indexed data
	ctx.Signature = targetSym.Signature
	ctx.Documentation = targetSym.DocComment

	return nil
}

// fillDirectRelationships populates immediate relationships
func (cle *ContextLookupEngine) fillDirectRelationships(context *CodeObjectContext) error {
	objectID := context.ObjectID

	// Get incoming references (who uses this object)
	incomingRefs, err := cle.getIncomingReferences(objectID)
	if err != nil {
		return fmt.Errorf("failed to get incoming references: %w", err)
	}
	context.DirectRelationships.IncomingReferences = incomingRefs

	// Get outgoing references (what this object uses)
	outgoingRefs, err := cle.getOutgoingReferences(objectID)
	if err != nil {
		return fmt.Errorf("failed to get outgoing references: %w", err)
	}
	context.DirectRelationships.OutgoingReferences = outgoingRefs

	// Get caller relationships
	callerFunctions, err := cle.getCallerFunctions(objectID)
	if err != nil {
		return fmt.Errorf("failed to get caller functions: %w", err)
	}
	context.DirectRelationships.CallerFunctions = callerFunctions

	// Get called functions
	calledFunctions, err := cle.getCalledFunctions(objectID)
	if err != nil {
		return fmt.Errorf("failed to get called functions: %w", err)
	}
	context.DirectRelationships.CalledFunctions = calledFunctions

	// Get parent hierarchy
	parentObjects, err := cle.getParentObjects(objectID)
	if err != nil {
		return fmt.Errorf("failed to get parent objects: %w", err)
	}
	context.DirectRelationships.ParentObjects = parentObjects

	// Get child objects
	childObjects, err := cle.getChildObjects(objectID)
	if err != nil {
		return fmt.Errorf("failed to get child objects: %w", err)
	}
	context.DirectRelationships.ChildObjects = childObjects

	// Get class relationships
	if objectID.Type == types.SymbolTypeClass || objectID.Type == types.SymbolTypeMethod {
		parentClasses, err := cle.getParentClasses(objectID)
		if err != nil {
			return fmt.Errorf("failed to get parent classes: %w", err)
		}
		context.DirectRelationships.ParentClasses = parentClasses

		implementingTypes, err := cle.getImplementingTypes(objectID)
		if err != nil {
			return fmt.Errorf("failed to get implementing types: %w", err)
		}
		context.DirectRelationships.ImplementingTypes = implementingTypes
	}

	// Get type relationships
	usedTypes, err := cle.getUsedTypes(objectID)
	if err != nil {
		return fmt.Errorf("failed to get used types: %w", err)
	}
	context.DirectRelationships.UsedTypes = usedTypes

	// Get imported modules
	importedModules, err := cle.getImportedModules(objectID)
	if err != nil {
		return fmt.Errorf("failed to get imported modules: %w", err)
	}
	context.DirectRelationships.ImportedModules = importedModules

	return nil
}

// getIncomingReferences finds all objects that reference the target object
func (cle *ContextLookupEngine) getIncomingReferences(objectID CodeObjectID) ([]ObjectReference, error) {
	var references []ObjectReference

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return references, nil // No symbols found, return empty list
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
		return references, nil // Symbol not found in specified file
	}

	// Get incoming references from ReferenceTracker
	incomingRefs := cle.refTracker.GetSymbolReferences(targetSymbol.ID, "incoming")

	for _, ref := range incomingRefs {
		// Get the source symbol (the one making the reference)
		sourceSymbol := cle.refTracker.GetEnhancedSymbol(ref.SourceSymbol)
		if sourceSymbol == nil {
			continue
		}

		refObj := ObjectReference{
			ObjectID: CodeObjectID{
				FileID:   sourceSymbol.FileID,
				Name:     sourceSymbol.Name,
				Type:     sourceSymbol.Type,
				SymbolID: fmt.Sprintf("%d", sourceSymbol.ID),
			},
			Location: types.SymbolLocation{
				FileID: ref.FileID,
				Line:   ref.Line,
				Column: ref.Column,
			},
			Context:    fmt.Sprintf("%d", ref.Type),
			Confidence: 0.9, // High confidence for indexed references
		}

		references = append(references, refObj)
	}

	// Sort by confidence and filter
	SortByConfidence(references)
	bits := atomic.LoadInt64(&cle.confidenceThreshold)
	threshold := math.Float64frombits(uint64(bits))
	return FilterHighConfidence(references, threshold), nil
}

// getOutgoingReferences finds all objects referenced by the target object
func (cle *ContextLookupEngine) getOutgoingReferences(objectID CodeObjectID) ([]ObjectReference, error) {
	var references []ObjectReference

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return references, nil // No symbols found, return empty list
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
		return references, nil // Symbol not found in specified file
	}

	// Get outgoing references from ReferenceTracker
	outgoingRefs := cle.refTracker.GetSymbolReferences(targetSymbol.ID, "outgoing")

	for _, ref := range outgoingRefs {
		// Get the target symbol (the one being referenced)
		targetRefSymbol := cle.refTracker.GetEnhancedSymbol(ref.TargetSymbol)
		if targetRefSymbol == nil {
			continue
		}

		refObj := ObjectReference{
			ObjectID: CodeObjectID{
				FileID:   targetRefSymbol.FileID,
				Name:     targetRefSymbol.Name,
				Type:     targetRefSymbol.Type,
				SymbolID: fmt.Sprintf("%d", targetRefSymbol.ID),
			},
			Location: types.SymbolLocation{
				FileID: ref.FileID,
				Line:   ref.Line,
				Column: ref.Column,
			},
			Context:    fmt.Sprintf("%d", ref.Type),
			Confidence: 0.9, // High confidence for indexed references
		}

		references = append(references, refObj)
	}

	// Deduplicate and sort
	references = deduplicateReferences(references)
	SortByConfidence(references)
	bits := atomic.LoadInt64(&cle.confidenceThreshold)
	threshold := math.Float64frombits(uint64(bits))
	return FilterHighConfidence(references, threshold), nil
}

// getCallerFunctions finds all functions that call the target function
func (cle *ContextLookupEngine) getCallerFunctions(objectID CodeObjectID) ([]ObjectReference, error) {
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return nil, nil
	}

	var callers []ObjectReference

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return callers, nil
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
		return callers, nil
	}

	// Get incoming references (who calls this function)
	incomingRefs := cle.refTracker.GetSymbolReferences(targetSymbol.ID, "incoming")

	for _, ref := range incomingRefs {
		// Filter to only call references
		if ref.Type != types.RefTypeCall {
			continue
		}

		// Get the source symbol (the caller)
		sourceSymbol := cle.refTracker.GetEnhancedSymbol(ref.SourceSymbol)
		if sourceSymbol == nil {
			continue
		}

		// Only include functions and methods as callers
		if sourceSymbol.Type != types.SymbolTypeFunction && sourceSymbol.Type != types.SymbolTypeMethod {
			continue
		}

		callerObj := ObjectReference{
			ObjectID: CodeObjectID{
				FileID:   sourceSymbol.FileID,
				Name:     sourceSymbol.Name,
				Type:     sourceSymbol.Type,
				SymbolID: fmt.Sprintf("%d", sourceSymbol.ID),
			},
			Location: types.SymbolLocation{
				FileID: sourceSymbol.FileID,
				Line:   sourceSymbol.Line,
				Column: sourceSymbol.Column,
			},
			Context:    "function_call",
			Confidence: 0.95, // Very high confidence for indexed call graph
		}
		callers = append(callers, callerObj)
	}

	SortByConfidence(callers)
	bits := atomic.LoadInt64(&cle.confidenceThreshold)
	threshold := math.Float64frombits(uint64(bits))
	return FilterHighConfidence(callers, threshold), nil
}

// getCalledFunctions finds all functions called by the target function
func (cle *ContextLookupEngine) getCalledFunctions(objectID CodeObjectID) ([]ObjectReference, error) {
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return nil, nil
	}

	var called []ObjectReference

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return called, nil
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
		return called, nil
	}

	// Get outgoing references (what this function calls)
	outgoingRefs := cle.refTracker.GetSymbolReferences(targetSymbol.ID, "outgoing")

	for _, ref := range outgoingRefs {
		// Filter to only call references
		if ref.Type != types.RefTypeCall {
			continue
		}

		// Get the target symbol (the callee)
		targetRefSymbol := cle.refTracker.GetEnhancedSymbol(ref.TargetSymbol)
		if targetRefSymbol == nil {
			continue
		}

		// Only include functions and methods as callees
		if targetRefSymbol.Type != types.SymbolTypeFunction && targetRefSymbol.Type != types.SymbolTypeMethod {
			continue
		}

		calledObj := ObjectReference{
			ObjectID: CodeObjectID{
				FileID:   targetRefSymbol.FileID,
				Name:     targetRefSymbol.Name,
				Type:     targetRefSymbol.Type,
				SymbolID: fmt.Sprintf("%d", targetRefSymbol.ID),
			},
			Location: types.SymbolLocation{
				FileID: ref.FileID,
				Line:   ref.Line,
				Column: ref.Column,
			},
			Context:    "function_call",
			Confidence: 0.95, // Very high confidence for indexed call graph
		}
		called = append(called, calledObj)
	}

	SortByConfidence(called)
	bits := atomic.LoadInt64(&cle.confidenceThreshold)
	threshold := math.Float64frombits(uint64(bits))
	return FilterHighConfidence(called, threshold), nil
}

// getParentObjects finds parent objects (namespaces, classes, functions)
func (cle *ContextLookupEngine) getParentObjects(objectID CodeObjectID) ([]ObjectReference, error) {
	var parents []ObjectReference

	// Find the symbol by name in the reference tracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return parents, nil
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
		return parents, nil
	}

	// Get the scope chain for this symbol - it already contains parent scopes!
	// The scope chain is ordered from innermost to outermost
	scopeChain := targetSymbol.ScopeChain

	// Convert scopes to ObjectReferences
	// Skip the first scope if it's the symbol itself
	for i, scope := range scopeChain {
		// Skip if this is the symbol's own scope
		if i == 0 && scope.Name == targetSymbol.Name {
			continue
		}

		// Only include named scopes (skip anonymous blocks)
		if scope.Name == "" {
			continue
		}

		parentObj := ObjectReference{
			ObjectID: CodeObjectID{
				FileID:   objectID.FileID,
				Name:     scope.Name,
				Type:     convertScopeTypeToSymbolType(scope.Type),
				SymbolID: fmt.Sprintf("scope_%s", scope.Name),
			},
			Location: types.SymbolLocation{
				FileID: objectID.FileID,
				Line:   scope.StartLine,
				Column: 0, // ScopeInfo doesn't have column info
			},
			Context:    scope.Type.String(),
			Confidence: 0.95, // High confidence for indexed scope data
		}
		parents = append(parents, parentObj)
	}

	return parents, nil
}

// convertScopeTypeToSymbolType converts ScopeType to SymbolType
func convertScopeTypeToSymbolType(scopeType types.ScopeType) types.SymbolType {
	switch scopeType {
	case types.ScopeTypeFile, types.ScopeTypeFolder:
		return types.SymbolTypeModule
	case types.ScopeTypeNamespace:
		return types.SymbolTypeModule
	case types.ScopeTypeClass:
		return types.SymbolTypeClass
	case types.ScopeTypeInterface:
		return types.SymbolTypeInterface
	case types.ScopeTypeFunction, types.ScopeTypeMethod:
		return types.SymbolTypeFunction
	default:
		return types.SymbolTypeVariable
	}
}

// getChildObjects finds child objects (member functions, nested types)
func (cle *ContextLookupEngine) getChildObjects(objectID CodeObjectID) ([]ObjectReference, error) {
	var children []ObjectReference

	// For classes, find methods and fields
	if objectID.Type == types.SymbolTypeClass {
		methods, err := cle.getClassMethods(objectID)
		if err == nil {
			children = append(children, methods...)
		}

		fields, err := cle.getClassFields(objectID)
		if err == nil {
			children = append(children, fields...)
		}
	}

	// For namespaces/modules, find contained objects
	if objectID.Type == types.SymbolTypeModule {
		contained, err := cle.getModuleContents(objectID)
		if err == nil {
			children = append(children, contained...)
		}
	}

	SortByConfidence(children)
	return children, nil
}

// Helper functions

func (cle *ContextLookupEngine) extractSignature(ast *sitter.Tree, content []byte, symbol *types.Symbol) string {
	// This would extract the function/class signature from the AST
	// For now, return a simplified version
	return fmt.Sprintf("%s %s()", symbol.Type, symbol.Name)
}

func (cle *ContextLookupEngine) extractDocumentation(ast *sitter.Tree, content []byte, symbol *types.Symbol) string {
	// This would extract documentation comments from the AST
	// For now, return empty string
	return ""
}

func (cle *ContextLookupEngine) getReferenceContext(fileID types.FileID, line int, symbolName string) string {
	// This would analyze the context around the reference
	// For now, return a generic context
	return "direct_reference"
}

func determineSymbolType(captureName string) types.SymbolType {
	switch captureName {
	case "call_id":
		return types.SymbolTypeFunction
	case "type_id":
		return types.SymbolTypeType
	case "field_id":
		return types.SymbolTypeField
	default:
		return types.SymbolTypeVariable
	}
}

func (cle *ContextLookupEngine) isWithinObjectScope(match QueryMatch, objectID CodeObjectID) bool {
	// Using EnhancedSymbol bounds from indexed data
	// For now, return false (conservative - may miss some matches)
	return false
}

// findObjectBounds finds the start and end row (0-indexed) of a function or method
func (cle *ContextLookupEngine) findObjectBounds(node *sitter.Node, content []byte, name string, symbolType types.SymbolType) (startRow, endRow int) {
	if node == nil {
		return -1, -1
	}

	nodeKind := node.Kind()

	// Check if this is the function/method we're looking for
	if nodeKind == "function_declaration" || nodeKind == "method_declaration" {
		// Find the name child
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			childKind := child.Kind()
			if childKind == "identifier" || childKind == "field_identifier" {
				nameText := string(content[child.StartByte():child.EndByte()])
				if nameText == name {
					// Found it! Return the bounds of this node
					return int(node.StartPosition().Row), int(node.EndPosition().Row)
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		startRow, endRow := cle.findObjectBounds(child, content, name, symbolType)
		if startRow != -1 {
			return startRow, endRow
		}
	}

	return -1, -1
}

func (cle *ContextLookupEngine) findContainingFunction(fileID types.FileID, line int, column int) *types.Symbol {
	// This would find the function containing the given location
	// For now, return nil as placeholder
	return nil
}

func (cle *ContextLookupEngine) findContainingScopes(ast *sitter.Tree, content []byte, location types.SymbolLocation) []struct {
	Name     string
	Type     types.SymbolType
	Location types.SymbolLocation
	SymbolID string
} {
	// This would find all containing scopes (function, class, namespace)
	// For now, return empty slice
	return []struct {
		Name     string
		Type     types.SymbolType
		Location types.SymbolLocation
		SymbolID string
	}{}
}

func deduplicateReferences(refs []ObjectReference) []ObjectReference {
	seen := make(map[ContextKey]bool) // Use struct key (reduces allocations)
	var unique []ObjectReference

	for _, ref := range refs {
		key := ContextKey{Name: ref.ObjectID.Name, FileID: ref.ObjectID.FileID, Context: ref.Context}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ref)
		}
	}

	return unique
}

// Placeholder implementations for class/module methods

func (cle *ContextLookupEngine) getParentClasses(objectID CodeObjectID) ([]ObjectReference, error) {
	// Find parent classes (embedded types in Go) for a struct
	var parents []ObjectReference

	// Only structs can have parent classes (embedded types)
	if objectID.Type != types.SymbolTypeStruct && objectID.Type != types.SymbolTypeClass {
		return parents, nil
	}

	// Find the symbol in ReferenceTracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return parents, nil
	}

	// Get inheritance relationships from OutgoingRefs
	for _, ref := range targetSymbol.OutgoingRefs {
		if ref.Type == types.RefTypeInheritance {
			parentObj := ObjectReference{
				ObjectID: CodeObjectID{
					FileID:   ref.FileID,
					Name:     ref.ReferencedName,
					Type:     types.SymbolTypeStruct, // Could also be interface
					SymbolID: fmt.Sprintf("%d", ref.TargetSymbol),
				},
				Location: types.SymbolLocation{
					FileID: ref.FileID,
					Line:   ref.Line,
					Column: ref.Column,
				},
				Confidence: 0.95,
				Context:    "inheritance",
			}
			parents = append(parents, parentObj)
		}
	}

	return parents, nil
}

// findStructNode finds a struct declaration node by name
func findStructNode(node *sitter.Node, structName string, source []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	// Check if this is a type_declaration with matching struct name
	if node.Kind() == "type_declaration" {
		// Look for type_spec child
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "type_spec" {
				// Check if the name matches
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					name := string(source[nameNode.StartByte():nameNode.EndByte()])
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
		result := findStructNode(child, structName, source)
		if result != nil {
			return result
		}
	}

	return nil
}

// extractEmbeddedTypes extracts embedded type names from a struct_type node
func extractEmbeddedTypes(structNode *sitter.Node, source []byte) []string {
	var embedded []string

	if structNode == nil || structNode.Kind() != "struct_type" {
		return embedded
	}

	// Look for field_declaration_list
	var fieldListNode *sitter.Node
	for i := uint(0); i < structNode.ChildCount(); i++ {
		child := structNode.Child(i)
		if child.Kind() == "field_declaration_list" {
			fieldListNode = child
			break
		}
	}

	if fieldListNode == nil {
		return embedded
	}

	// Iterate through field declarations
	for i := uint(0); i < fieldListNode.ChildCount(); i++ {
		child := fieldListNode.Child(i)
		if child.Kind() == "field_declaration" {
			// Check if this is an embedded field
			// Embedded fields have type_identifier as direct child, not field_identifier
			hasFieldName := false
			var typeName string

			for j := uint(0); j < child.ChildCount(); j++ {
				subChild := child.Child(j)
				if subChild.Kind() == "field_identifier" {
					hasFieldName = true
					break
				} else if subChild.Kind() == "type_identifier" {
					typeName = string(source[subChild.StartByte():subChild.EndByte()])
				}
			}

			// If no field_identifier but has type_identifier, it's embedded
			if !hasFieldName && typeName != "" {
				embedded = append(embedded, typeName)
			}
		}
	}

	return embedded
}

func (cle *ContextLookupEngine) getImplementingTypes(objectID CodeObjectID) ([]ObjectReference, error) {
	// Find all types that implement the given interface
	var implementations []ObjectReference

	// Only interfaces can have implementing types
	if objectID.Type != types.SymbolTypeInterface {
		return implementations, nil
	}

	// Find the interface symbol in ReferenceTracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == types.SymbolTypeInterface {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return implementations, nil
	}

	// Get incoming inheritance references (types that implement this interface)
	incomingRefs := cle.refTracker.GetSymbolReferences(targetSymbol.ID, "incoming")

	for _, ref := range incomingRefs {
		if ref.Type == types.RefTypeInheritance {
			// Get the implementing type
			implSymbol := cle.refTracker.GetEnhancedSymbol(ref.SourceSymbol)
			if implSymbol == nil {
				continue
			}

			implObj := ObjectReference{
				ObjectID: CodeObjectID{
					FileID:   implSymbol.FileID,
					Name:     implSymbol.Name,
					Type:     implSymbol.Type,
					SymbolID: fmt.Sprintf("%d", implSymbol.ID),
				},
				Location: types.SymbolLocation{
					FileID: implSymbol.FileID,
					Line:   implSymbol.Line,
					Column: implSymbol.Column,
				},
				Confidence: 0.95,
				Context:    "implements",
			}
			implementations = append(implementations, implObj)
		}
	}

	return implementations, nil
}

// findInterfaceMethods extracts method names from an interface
func findInterfaceMethods(node *sitter.Node, interfaceName string, source []byte) []string {
	var methods []string

	if node == nil {
		return methods
	}

	// Find interface definition
	if node.Kind() == "type_declaration" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					name := string(source[nameNode.StartByte():nameNode.EndByte()])
					if name == interfaceName {
						// Found the interface, extract methods
						typeNode := child.ChildByFieldName("type")
						if typeNode != nil && typeNode.Kind() == "interface_type" {
							methods = extractInterfaceMethodNames(typeNode, source)
							return methods
						}
					}
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		result := findInterfaceMethods(child, interfaceName, source)
		if len(result) > 0 {
			return result
		}
	}

	return methods
}

// extractInterfaceMethodNames extracts method names from interface_type node
func extractInterfaceMethodNames(interfaceNode *sitter.Node, source []byte) []string {
	var methods []string

	// Interface methods are method_elem nodes directly under interface_type
	for i := uint(0); i < interfaceNode.ChildCount(); i++ {
		child := interfaceNode.Child(i)
		if child.Kind() == "method_elem" {
			// First child is field_identifier with the method name
			for j := uint(0); j < child.ChildCount(); j++ {
				subChild := child.Child(j)
				if subChild.Kind() == "field_identifier" {
					methodName := string(source[subChild.StartByte():subChild.EndByte()])
					methods = append(methods, methodName)
					break
				}
			}
		}
	}

	return methods
}

// findAllStructs finds all struct type names in the AST
func findAllStructs(node *sitter.Node, source []byte) []string {
	var structs []string

	if node == nil {
		return structs
	}

	if node.Kind() == "type_declaration" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				typeNode := child.ChildByFieldName("type")
				if nameNode != nil && typeNode != nil && typeNode.Kind() == "struct_type" {
					structName := string(source[nameNode.StartByte():nameNode.EndByte()])
					structs = append(structs, structName)
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		structs = append(structs, findAllStructs(child, source)...)
	}

	return structs
}

// findMethodsForType finds all method names for a given type
func findMethodsForType(node *sitter.Node, typeName string, source []byte) []string {
	var methods []string

	if node == nil {
		return methods
	}

	if node.Kind() == "method_declaration" {
		// Check if the receiver matches our type
		// Receiver is the first parameter_list in a method_declaration
		receiverType := extractReceiverTypeFromMethod(node, source)
		if receiverType == typeName {
			// Extract method name (field_identifier after first parameter_list)
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child.Kind() == "field_identifier" {
					methodName := string(source[child.StartByte():child.EndByte()])
					methods = append(methods, methodName)
					break
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		methods = append(methods, findMethodsForType(child, typeName, source)...)
	}

	return methods
}

// extractReceiverTypeFromMethod extracts the receiver type from a method_declaration node
func extractReceiverTypeFromMethod(methodNode *sitter.Node, source []byte) string {
	// The first parameter_list in a method_declaration is the receiver
	for i := uint(0); i < methodNode.ChildCount(); i++ {
		child := methodNode.Child(i)
		if child.Kind() == "parameter_list" {
			// This is the receiver parameter list
			for j := uint(0); j < child.ChildCount(); j++ {
				param := child.Child(j)
				if param.Kind() == "parameter_declaration" {
					// Find the type (skip the identifier)
					for k := uint(0); k < param.ChildCount(); k++ {
						typeNode := param.Child(k)
						if typeNode.Kind() == "pointer_type" {
							// Extract underlying type from pointer
							for m := uint(0); m < typeNode.ChildCount(); m++ {
								subType := typeNode.Child(m)
								if subType.Kind() == "type_identifier" {
									return string(source[subType.StartByte():subType.EndByte()])
								}
							}
						} else if typeNode.Kind() == "type_identifier" {
							return string(source[typeNode.StartByte():typeNode.EndByte()])
						}
					}
				}
			}
			// Only check the first parameter_list (the receiver)
			break
		}
	}
	return ""
}

// implementsInterface checks if a type implements all methods of an interface
func implementsInterface(interfaceMethods []string, typeMethods []string) bool {
	// Convert type methods to a set for fast lookup
	methodSet := make(map[string]bool)
	for _, method := range typeMethods {
		methodSet[method] = true
	}

	// Check if all interface methods are implemented
	for _, method := range interfaceMethods {
		if !methodSet[method] {
			return false
		}
	}

	return true
}

func (cle *ContextLookupEngine) getUsedTypes(objectID CodeObjectID) ([]ObjectReference, error) {
	// Find types used by a function (parameter types, return types, variable types)
	var usedTypes []ObjectReference

	// Find the symbol in ReferenceTracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return usedTypes, nil
	}

	// Get type references from OutgoingRefs
	seen := make(map[string]bool)
	for _, ref := range targetSymbol.OutgoingRefs {
		// Look for type references (not function calls)
		if ref.Type == types.RefTypeDeclaration || ref.Type == types.RefTypeAssignment {
			typeName := ref.ReferencedName
			if typeName == "" || seen[typeName] {
				continue
			}
			seen[typeName] = true

			// Try to resolve the type symbol
			typeSymbol := cle.refTracker.GetEnhancedSymbol(ref.TargetSymbol)
			var typeType types.SymbolType
			if typeSymbol != nil {
				typeType = typeSymbol.Type
			} else {
				typeType = types.SymbolTypeType
			}

			typeObj := ObjectReference{
				ObjectID: CodeObjectID{
					FileID:   objectID.FileID,
					Name:     typeName,
					Type:     typeType,
					SymbolID: fmt.Sprintf("%d", ref.TargetSymbol),
				},
				Location: types.SymbolLocation{
					FileID: ref.FileID,
					Line:   ref.Line,
					Column: ref.Column,
				},
				Confidence: 0.8,
				Context:    "type_reference",
			}
			usedTypes = append(usedTypes, typeObj)
		}
	}

	return usedTypes, nil
}

// findTypesUsedByFunction extracts custom type names from function parameters and body
func findTypesUsedByFunction(node *sitter.Node, functionName string, source []byte) []string {
	var typeNames []string

	if node == nil {
		return typeNames
	}

	// Find function_declaration with matching name
	if node.Kind() == "function_declaration" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			name := string(source[nameNode.StartByte():nameNode.EndByte()])
			if name == functionName {
				// Extract types from parameters
				paramsNode := node.ChildByFieldName("parameters")
				if paramsNode != nil {
					typeNames = append(typeNames, extractTypesFromParameterList(paramsNode, source)...)
				}
				return typeNames
			}
		}
	}

	// Recursively search
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		result := findTypesUsedByFunction(child, functionName, source)
		if len(result) > 0 {
			return result
		}
	}

	return typeNames
}

// extractTypesFromParameterList extracts custom type names from parameters
func extractTypesFromParameterList(paramListNode *sitter.Node, source []byte) []string {
	var types []string

	for i := uint(0); i < paramListNode.ChildCount(); i++ {
		child := paramListNode.Child(i)
		if child.Kind() == "parameter_declaration" {
			// Find type_identifier children (custom types)
			for j := uint(0); j < child.ChildCount(); j++ {
				typeNode := child.Child(j)
				if typeNode.Kind() == "type_identifier" {
					typeName := string(source[typeNode.StartByte():typeNode.EndByte()])
					// Filter out built-in types
					if !isBuiltInType(typeName) {
						types = append(types, typeName)
					}
				}
			}
		}
	}

	return types
}

// isBuiltInType checks if a type name is a Go built-in type
func isBuiltInType(typeName string) bool {
	builtins := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"string": true, "bool": true, "byte": true, "rune": true,
		"error": true,
	}
	return builtins[typeName]
}

func (cle *ContextLookupEngine) getImportedModules(objectID CodeObjectID) ([]ModuleReference, error) {
	// Find all imports in the file containing this object
	var modules []ModuleReference

	// Get all symbols in the file
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Track unique import paths
	seen := make(map[string]bool)

	// Look for import references in file-level symbols
	for _, sym := range fileSymbols {
		for _, ref := range sym.OutgoingRefs {
			if ref.Type == types.RefTypeImport {
				modulePath := ref.ReferencedName
				if modulePath == "" || seen[modulePath] {
					continue
				}
				seen[modulePath] = true

				modules = append(modules, ModuleReference{
					ModulePath:  modulePath,
					ImportStyle: "direct",
				})
			}
		}
	}

	return modules, nil
}

// extractImports extracts import paths from the AST
func extractImports(node *sitter.Node, source []byte) []string {
	var imports []string

	if node == nil {
		return imports
	}

	// import_declaration contains import_spec or import_spec_list
	if node.Kind() == "import_declaration" {
		// Look for import_spec children
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "import_spec" || child.Kind() == "import_spec_list" {
				imports = append(imports, extractImportFromSpec(child, source)...)
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		imports = append(imports, extractImports(child, source)...)
	}

	return imports
}

// extractImportFromSpec extracts import path from import_spec or import_spec_list
func extractImportFromSpec(specNode *sitter.Node, source []byte) []string {
	var imports []string

	if specNode.Kind() == "import_spec" {
		// Find interpreted_string_literal (the import path)
		for i := uint(0); i < specNode.ChildCount(); i++ {
			child := specNode.Child(i)
			if child.Kind() == "interpreted_string_literal" {
				// Remove quotes from import path
				importPath := string(source[child.StartByte():child.EndByte()])
				importPath = importPath[1 : len(importPath)-1] // Strip quotes
				imports = append(imports, importPath)
			}
		}
	} else if specNode.Kind() == "import_spec_list" {
		// Recursively extract from each import_spec
		for i := uint(0); i < specNode.ChildCount(); i++ {
			child := specNode.Child(i)
			imports = append(imports, extractImportFromSpec(child, source)...)
		}
	}

	return imports
}

func (cle *ContextLookupEngine) getClassMethods(objectID CodeObjectID) ([]ObjectReference, error) {
	// Find all methods for a class/struct
	var methods []ObjectReference

	// Get all symbols in the file
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Find methods that belong to this class
	for _, sym := range fileSymbols {
		if sym.Type != types.SymbolTypeMethod {
			continue
		}

		// Check if this method belongs to our class
		// Methods have the class name in their ScopeChain
		belongsToClass := false
		for _, scope := range sym.ScopeChain {
			if scope.Name == objectID.Name && (scope.Type == types.ScopeTypeClass || scope.Type == types.ScopeTypeInterface) {
				belongsToClass = true
				break
			}
		}

		if belongsToClass {
			methodRef := ObjectReference{
				ObjectID: CodeObjectID{
					FileID:   sym.FileID,
					Name:     sym.Name,
					Type:     types.SymbolTypeMethod,
					SymbolID: fmt.Sprintf("%d", sym.ID),
				},
				Location: types.SymbolLocation{
					FileID: sym.FileID,
					Line:   sym.Line,
					Column: sym.Column,
				},
				Confidence: 0.95,
				Context:    "method",
			}
			methods = append(methods, methodRef)
		}
	}

	return methods, nil
}

func (cle *ContextLookupEngine) getClassFields(objectID CodeObjectID) ([]ObjectReference, error) {
	// Find all fields for a class/struct
	var fields []ObjectReference

	// First, find the class symbol to get its bounds
	classSymbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var classSymbol *types.EnhancedSymbol
	for _, sym := range classSymbols {
		if sym.FileID == objectID.FileID && (sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeStruct) {
			classSymbol = sym
			break
		}
	}

	if classSymbol == nil {
		return fields, nil
	}

	// Get all symbols in the file
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Find fields (variables) that are within the class bounds
	for _, sym := range fileSymbols {
		if sym.Type != types.SymbolTypeVariable && sym.Type != types.SymbolTypeField {
			continue
		}

		// Check if field is within class bounds
		if sym.Line >= classSymbol.Line && sym.EndLine <= classSymbol.EndLine {
			// Also check scope chain to ensure it's a direct member
			isDirectMember := false
			for _, scope := range sym.ScopeChain {
				if scope.Name == objectID.Name {
					isDirectMember = true
					break
				}
			}

			if isDirectMember {
				fieldRef := ObjectReference{
					ObjectID: CodeObjectID{
						FileID:   sym.FileID,
						Name:     sym.Name,
						Type:     types.SymbolTypeField,
						SymbolID: fmt.Sprintf("%d", sym.ID),
					},
					Location: types.SymbolLocation{
						FileID: sym.FileID,
						Line:   sym.Line,
						Column: sym.Column,
					},
					Confidence: 0.95,
					Context:    "field",
				}
				fields = append(fields, fieldRef)
			}
		}
	}

	return fields, nil
}

// extractStructFields extracts field names from a struct_type node (not embedded types)
func extractStructFields(structNode *sitter.Node, source []byte) []string {
	var fields []string

	if structNode == nil || structNode.Kind() != "struct_type" {
		return fields
	}

	// Look for field_declaration_list
	var fieldListNode *sitter.Node
	for i := uint(0); i < structNode.ChildCount(); i++ {
		child := structNode.Child(i)
		if child.Kind() == "field_declaration_list" {
			fieldListNode = child
			break
		}
	}

	if fieldListNode == nil {
		return fields
	}

	// Iterate through field declarations
	for i := uint(0); i < fieldListNode.ChildCount(); i++ {
		child := fieldListNode.Child(i)
		if child.Kind() == "field_declaration" {
			// Extract field_identifier (skip embedded types which have no field_identifier)
			for j := uint(0); j < child.ChildCount(); j++ {
				subChild := child.Child(j)
				if subChild.Kind() == "field_identifier" {
					fieldName := string(source[subChild.StartByte():subChild.EndByte()])
					fields = append(fields, fieldName)
				}
			}
		}
	}

	return fields
}

func (cle *ContextLookupEngine) getModuleContents(objectID CodeObjectID) ([]ObjectReference, error) {
	// Find all exported symbols in a module/package
	var contents []ObjectReference

	// Get all symbols in the file
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(objectID.FileID)

	// Find all exported symbols (top-level symbols with IsExported = true)
	for _, sym := range fileSymbols {
		// Only include exported symbols
		if !sym.IsExported {
			continue
		}

		// Only include top-level symbols (those with minimal scope chain)
		isTopLevel := len(sym.ScopeChain) <= 1 // File-level or module-level only

		if isTopLevel {
			contentRef := ObjectReference{
				ObjectID: CodeObjectID{
					FileID:   sym.FileID,
					Name:     sym.Name,
					Type:     sym.Type,
					SymbolID: fmt.Sprintf("%d", sym.ID),
				},
				Location: types.SymbolLocation{
					FileID: sym.FileID,
					Line:   sym.Line,
					Column: sym.Column,
				},
				Confidence: 0.95,
				Context:    "export",
			}
			contents = append(contents, contentRef)
		}
	}

	return contents, nil
}

// findExportedSymbols finds all exported (capitalized) top-level symbols
func findExportedSymbols(node *sitter.Node, source []byte) map[string]types.SymbolType {
	exports := make(map[string]types.SymbolType)

	if node == nil {
		return exports
	}

	// Look for top-level declarations
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)

		switch child.Kind() {
		case "function_declaration":
			// Check if function is exported
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				name := string(source[nameNode.StartByte():nameNode.EndByte()])
				if isExported(name) {
					exports[name] = types.SymbolTypeFunction
				}
			}

		case "type_declaration":
			// Check if type is exported
			for j := uint(0); j < child.ChildCount(); j++ {
				typeSpec := child.Child(j)
				if typeSpec.Kind() == "type_spec" {
					nameNode := typeSpec.ChildByFieldName("name")
					if nameNode != nil {
						name := string(source[nameNode.StartByte():nameNode.EndByte()])
						if isExported(name) {
							// Determine if it's a struct, interface, or other type
							typeNode := typeSpec.ChildByFieldName("type")
							if typeNode != nil {
								switch typeNode.Kind() {
								case "struct_type":
									exports[name] = types.SymbolTypeStruct
								case "interface_type":
									exports[name] = types.SymbolTypeInterface
								default:
									exports[name] = types.SymbolTypeClass // Generic type
								}
							}
						}
					}
				}
			}

		case "const_declaration", "var_declaration":
			// Check if constant/variable is exported
			for j := uint(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Kind() == "const_spec" || spec.Kind() == "var_spec" {
					// Find identifier list
					for k := uint(0); k < spec.ChildCount(); k++ {
						idList := spec.Child(k)
						if idList.Kind() == "identifier_list" || idList.Kind() == "identifier" {
							// Extract identifier
							for m := uint(0); m < idList.ChildCount(); m++ {
								id := idList.Child(m)
								if id.Kind() == "identifier" {
									name := string(source[id.StartByte():id.EndByte()])
									if isExported(name) {
										exports[name] = types.SymbolTypeVariable
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return exports
}

// isExported checks if a Go identifier is exported (starts with uppercase)
func isExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	// In Go, exported names start with an uppercase letter
	return name[0] >= 'A' && name[0] <= 'Z'
}
