package parser

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// ============================================================================
// TYPE RELATIONSHIP EXTRACTION
// Extracts implements/extends/embeds relationships for type hierarchy tracking
// ============================================================================

// processTypeRelationships extracts type relationships (implements, extends, embeds)
// This runs in the same single-pass visitor as other extraction
func (ue *UnifiedExtractor) processTypeRelationships(node *tree_sitter.Node, nodeType string) {
	switch ue.ext {
	case ".go":
		ue.processGoTypeRelationships(node, nodeType)
	case ".ts", ".tsx", ".js", ".jsx":
		ue.processJSTypeRelationships(node, nodeType)
	case ".py":
		ue.processPythonTypeRelationships(node, nodeType)
	case ".rs":
		ue.processRustTypeRelationships(node, nodeType)
	case ".java":
		ue.processJavaTypeRelationships(node, nodeType)
	case ".cs":
		ue.processCSharpTypeRelationships(node, nodeType)
	case ".php", ".phtml":
		ue.processPhpTypeRelationships(node, nodeType)
	}
}

// processGoTypeRelationships handles Go interface embedding, struct embedding, and interface usage detection
// Go patterns:
// - Interface embedding: type ReadWriter interface { Reader; Writer }
// - Struct embedding: type MyStruct struct { BaseStruct; name string }
// - Interface assignment: var w Writer = &File{} (quality: assigned)
// - Type assertion: x.(Writer) (quality: cast)
// - Interface return: func New() Writer { return &File{} } (quality: returned)
func (ue *UnifiedExtractor) processGoTypeRelationships(node *tree_sitter.Node, nodeType string) {
	switch nodeType {
	case "type_declaration":
		// Find the type_spec to get type name and underlying type
		for i := uint(0); i < node.ChildCount(); i++ {
			typeSpec := node.Child(i)
			if ue.getNodeType(typeSpec) != "type_spec" {
				continue
			}

			// Get the type name
			var typeName string
			var underlyingType *tree_sitter.Node

			for j := uint(0); j < typeSpec.ChildCount(); j++ {
				child := typeSpec.Child(j)
				childType := ue.getNodeType(child)
				switch childType {
				case "type_identifier":
					typeName = string(ue.content[child.StartByte():child.EndByte()])
				case "interface_type":
					underlyingType = child
				case "struct_type":
					underlyingType = child
				}
			}

			if typeName == "" || underlyingType == nil {
				continue
			}

			underlyingNodeType := ue.getNodeType(underlyingType)

			if underlyingNodeType == "interface_type" {
				// Extract embedded interfaces
				ue.extractGoEmbeddedInterfaces(node, typeName, underlyingType)
			} else if underlyingNodeType == "struct_type" {
				// Extract embedded structs
				ue.extractGoEmbeddedStructs(node, typeName, underlyingType)
			}
		}

	case "var_declaration":
		// Detect interface assignments: var w Writer = &File{}
		ue.extractGoInterfaceAssignment(node)

	case "type_assertion_expression":
		// Detect type casts: x.(Writer)
		ue.extractGoTypeCast(node)

	case "return_statement":
		// Detect interface returns: return &File{} in func that returns Writer
		ue.extractGoInterfaceReturn(node)
	}
}

// extractGoEmbeddedInterfaces extracts embedded interfaces from an interface_type
func (ue *UnifiedExtractor) extractGoEmbeddedInterfaces(sourceNode *tree_sitter.Node, typeName string, interfaceType *tree_sitter.Node) {
	// Look for type_elem children which represent embedded interfaces
	for i := uint(0); i < interfaceType.ChildCount(); i++ {
		child := interfaceType.Child(i)
		childType := ue.getNodeType(child)

		if childType == "type_elem" {
			// type_elem contains type_identifier for embedded interface
			for j := uint(0); j < child.ChildCount(); j++ {
				typeIdent := child.Child(j)
				if ue.getNodeType(typeIdent) == "type_identifier" {
					embeddedName := string(ue.content[typeIdent.StartByte():typeIdent.EndByte()])
					// Create RefTypeExtends reference (interface embedding = extends)
					ref := ue.createTypeRelationshipRef(typeIdent, types.RefTypeExtends, embeddedName, typeName)
					ue.references = append(ue.references, ref)
				}
			}
		}
	}
}

// extractGoEmbeddedStructs extracts embedded structs from a struct_type
func (ue *UnifiedExtractor) extractGoEmbeddedStructs(sourceNode *tree_sitter.Node, typeName string, structType *tree_sitter.Node) {
	// Look for field_declaration_list → field_declaration
	for i := uint(0); i < structType.ChildCount(); i++ {
		child := structType.Child(i)
		if ue.getNodeType(child) != "field_declaration_list" {
			continue
		}

		// Iterate through field declarations
		for j := uint(0); j < child.ChildCount(); j++ {
			fieldDecl := child.Child(j)
			if ue.getNodeType(fieldDecl) != "field_declaration" {
				continue
			}

			// An embedded struct field has only a type_identifier, no field_identifier
			hasFieldName := false
			var embeddedTypeNode *tree_sitter.Node

			for k := uint(0); k < fieldDecl.ChildCount(); k++ {
				fieldChild := fieldDecl.Child(k)
				fieldChildType := ue.getNodeType(fieldChild)
				if fieldChildType == "field_identifier" {
					hasFieldName = true
					break
				}
				if fieldChildType == "type_identifier" {
					embeddedTypeNode = fieldChild
				}
			}

			// Only type_identifier without field_identifier = embedded struct
			if !hasFieldName && embeddedTypeNode != nil {
				embeddedName := string(ue.content[embeddedTypeNode.StartByte():embeddedTypeNode.EndByte()])
				// Create RefTypeExtends reference (struct embedding = extends)
				ref := ue.createTypeRelationshipRef(embeddedTypeNode, types.RefTypeExtends, embeddedName, typeName)
				ue.references = append(ue.references, ref)
			}
		}
	}
}

// processJSTypeRelationships handles JavaScript/TypeScript extends and implements
// TypeScript patterns:
// - class Child extends Parent implements Interface1, Interface2 {}
// - interface Extended extends Base {}
func (ue *UnifiedExtractor) processJSTypeRelationships(node *tree_sitter.Node, nodeType string) {
	switch nodeType {
	case "class_declaration":
		// Get class name
		var className string
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			className = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
		}
		if className == "" {
			return
		}

		// Look for class_heritage which contains extends_clause and implements_clause
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if ue.getNodeType(child) != "class_heritage" {
				continue
			}

			// Process class_heritage children
			for j := uint(0); j < child.ChildCount(); j++ {
				heritageChild := child.Child(j)
				heritageType := ue.getNodeType(heritageChild)

				switch heritageType {
				case "extends_clause":
					// extends_clause contains the parent class
					for k := uint(0); k < heritageChild.ChildCount(); k++ {
						extChild := heritageChild.Child(k)
						extChildType := ue.getNodeType(extChild)
						if extChildType == "identifier" || extChildType == "type_identifier" {
							parentName := string(ue.content[extChild.StartByte():extChild.EndByte()])
							ref := ue.createTypeRelationshipRef(extChild, types.RefTypeExtends, parentName, className)
							ue.references = append(ue.references, ref)
						}
					}

				case "implements_clause":
					// implements_clause contains implemented interfaces
					for k := uint(0); k < heritageChild.ChildCount(); k++ {
						implChild := heritageChild.Child(k)
						implChildType := ue.getNodeType(implChild)
						if implChildType == "type_identifier" {
							ifaceName := string(ue.content[implChild.StartByte():implChild.EndByte()])
							ref := ue.createTypeRelationshipRef(implChild, types.RefTypeImplements, ifaceName, className)
							ue.references = append(ue.references, ref)
						}
					}
				}
			}
		}

	case "interface_declaration":
		// Get interface name
		var ifaceName string
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			ifaceName = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
		}
		if ifaceName == "" {
			return
		}

		// Look for extends_type_clause
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if ue.getNodeType(child) != "extends_type_clause" {
				continue
			}

			// Process extended interfaces
			for j := uint(0); j < child.ChildCount(); j++ {
				extChild := child.Child(j)
				if ue.getNodeType(extChild) == "type_identifier" {
					parentIface := string(ue.content[extChild.StartByte():extChild.EndByte()])
					ref := ue.createTypeRelationshipRef(extChild, types.RefTypeExtends, parentIface, ifaceName)
					ue.references = append(ue.references, ref)
				}
			}
		}
	}
}

// processPythonTypeRelationships handles Python class inheritance
// Python pattern:
// - class Child(Parent1, Parent2):
func (ue *UnifiedExtractor) processPythonTypeRelationships(node *tree_sitter.Node, nodeType string) {
	if nodeType != "class_definition" {
		return
	}

	// Get class name
	var className string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		className = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}
	if className == "" {
		return
	}

	// Look for argument_list containing base classes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) != "argument_list" {
			continue
		}

		// Each identifier in the argument_list is a base class
		for j := uint(0); j < child.ChildCount(); j++ {
			argChild := child.Child(j)
			if ue.getNodeType(argChild) == "identifier" {
				baseName := string(ue.content[argChild.StartByte():argChild.EndByte()])
				// Python uses RefTypeExtends for all inheritance
				ref := ue.createTypeRelationshipRef(argChild, types.RefTypeExtends, baseName, className)
				ue.references = append(ue.references, ref)
			}
		}
	}
}

// processRustTypeRelationships handles Rust trait implementations
// Rust pattern:
// - impl Trait for Type {}
func (ue *UnifiedExtractor) processRustTypeRelationships(node *tree_sitter.Node, nodeType string) {
	if nodeType != "impl_item" {
		return
	}

	// Look for pattern: impl TraitName for TypeName
	// Structure: impl_item → type_identifier (trait) → "for" → type_identifier (type)
	var traitName string
	var typeName string
	var traitNode *tree_sitter.Node
	sawFor := false

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)

		switch childType {
		case "type_identifier":
			if !sawFor {
				// First type_identifier is the trait
				traitName = string(ue.content[child.StartByte():child.EndByte()])
				traitNode = child
			} else {
				// Second type_identifier (after "for") is the implementing type
				typeName = string(ue.content[child.StartByte():child.EndByte()])
			}
		case "for":
			sawFor = true
		}
	}

	// Only create reference if we have both trait and type (impl Trait for Type)
	if traitName != "" && typeName != "" && traitNode != nil {
		ref := ue.createTypeRelationshipRef(traitNode, types.RefTypeImplements, traitName, typeName)
		ue.references = append(ue.references, ref)
	}
}

// processJavaTypeRelationships handles Java extends and implements
// Java patterns:
// - class Child extends Parent implements Interface1, Interface2 {}
func (ue *UnifiedExtractor) processJavaTypeRelationships(node *tree_sitter.Node, nodeType string) {
	if nodeType != "class_declaration" && nodeType != "interface_declaration" {
		return
	}

	// Get class/interface name
	var typeName string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		typeName = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}
	if typeName == "" {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)

		switch childType {
		case "superclass":
			// superclass → "extends" → type_identifier
			for j := uint(0); j < child.ChildCount(); j++ {
				superChild := child.Child(j)
				if ue.getNodeType(superChild) == "type_identifier" {
					parentName := string(ue.content[superChild.StartByte():superChild.EndByte()])
					ref := ue.createTypeRelationshipRef(superChild, types.RefTypeExtends, parentName, typeName)
					ue.references = append(ue.references, ref)
				}
			}

		case "super_interfaces":
			// super_interfaces → "implements" → type_list → type_identifier*
			for j := uint(0); j < child.ChildCount(); j++ {
				superChild := child.Child(j)
				if ue.getNodeType(superChild) == "type_list" {
					// Extract each interface from type_list
					for k := uint(0); k < superChild.ChildCount(); k++ {
						listChild := superChild.Child(k)
						if ue.getNodeType(listChild) == "type_identifier" {
							ifaceName := string(ue.content[listChild.StartByte():listChild.EndByte()])
							ref := ue.createTypeRelationshipRef(listChild, types.RefTypeImplements, ifaceName, typeName)
							ue.references = append(ue.references, ref)
						}
					}
				}
			}
		}
	}
}

// processCSharpTypeRelationships handles C# base list
// C# pattern:
// - class Child : Parent, Interface1, Interface2 {}
// Note: C# uses a single base_list for both class inheritance and interface implementation
func (ue *UnifiedExtractor) processCSharpTypeRelationships(node *tree_sitter.Node, nodeType string) {
	if nodeType != "class_declaration" && nodeType != "struct_declaration" && nodeType != "interface_declaration" {
		return
	}

	// Get type name
	var typeName string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		typeName = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}
	if typeName == "" {
		return
	}

	// Look for base_list
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) != "base_list" {
			continue
		}

		// In C#, the first identifier after ":" could be a class (extends) or interface (implements)
		// Without type resolution, we treat the first as extends and rest as implements
		// This is a heuristic that works for typical single-inheritance patterns
		isFirst := true
		for j := uint(0); j < child.ChildCount(); j++ {
			baseChild := child.Child(j)
			baseChildType := ue.getNodeType(baseChild)

			if baseChildType == "identifier" || baseChildType == "qualified_name" || baseChildType == "generic_name" {
				baseName := string(ue.content[baseChild.StartByte():baseChild.EndByte()])

				// For interfaces, all bases are extends (interface extension)
				// For classes, first is extends (class inheritance), rest are implements
				var refType types.ReferenceType
				if nodeType == "interface_declaration" {
					refType = types.RefTypeExtends
				} else if isFirst {
					refType = types.RefTypeExtends
					isFirst = false
				} else {
					refType = types.RefTypeImplements
				}

				ref := ue.createTypeRelationshipRef(baseChild, refType, baseName, typeName)
				ue.references = append(ue.references, ref)
			}
		}
	}
}

// processPhpTypeRelationships handles PHP extends, implements, and trait usage
// PHP patterns:
// - class Child extends Parent implements Interface1, Interface2 {}
// - interface Extended extends BaseInterface {}
// - use TraitName; (inside class body)
func (ue *UnifiedExtractor) processPhpTypeRelationships(node *tree_sitter.Node, nodeType string) {
	switch nodeType {
	case "class_declaration":
		ue.extractPhpClassRelationships(node)
	case "interface_declaration":
		ue.extractPhpInterfaceRelationships(node)
	case "use_declaration":
		// Trait usage: use TraitName;
		ue.extractPhpTraitUsage(node)
	}
}

// extractPhpClassRelationships extracts extends and implements from PHP classes
func (ue *UnifiedExtractor) extractPhpClassRelationships(node *tree_sitter.Node) {
	// Get class name
	var className string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		className = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}
	if className == "" {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)

		switch childType {
		case "base_clause":
			// extends Parent - PHP uses base_clause for class inheritance
			for j := uint(0); j < child.ChildCount(); j++ {
				baseChild := child.Child(j)
				baseChildType := ue.getNodeType(baseChild)
				if baseChildType == "name" || baseChildType == "qualified_name" {
					parentName := string(ue.content[baseChild.StartByte():baseChild.EndByte()])
					ref := ue.createTypeRelationshipRef(baseChild, types.RefTypeExtends, parentName, className)
					ue.references = append(ue.references, ref)
				}
			}

		case "class_interface_clause":
			// implements Interface1, Interface2
			for j := uint(0); j < child.ChildCount(); j++ {
				ifaceChild := child.Child(j)
				ifaceChildType := ue.getNodeType(ifaceChild)
				if ifaceChildType == "name" || ifaceChildType == "qualified_name" {
					ifaceName := string(ue.content[ifaceChild.StartByte():ifaceChild.EndByte()])
					ref := ue.createTypeRelationshipRef(ifaceChild, types.RefTypeImplements, ifaceName, className)
					ue.references = append(ue.references, ref)
				}
			}
		}
	}
}

// extractPhpInterfaceRelationships extracts extends from PHP interfaces
func (ue *UnifiedExtractor) extractPhpInterfaceRelationships(node *tree_sitter.Node) {
	// Get interface name
	var interfaceName string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		interfaceName = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}
	if interfaceName == "" {
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)

		if childType == "base_clause" {
			// interface extends BaseInterface1, BaseInterface2
			for j := uint(0); j < child.ChildCount(); j++ {
				baseChild := child.Child(j)
				baseChildType := ue.getNodeType(baseChild)
				if baseChildType == "name" || baseChildType == "qualified_name" {
					baseName := string(ue.content[baseChild.StartByte():baseChild.EndByte()])
					ref := ue.createTypeRelationshipRef(baseChild, types.RefTypeExtends, baseName, interfaceName)
					ue.references = append(ue.references, ref)
				}
			}
		}
	}
}

// extractPhpTraitUsage extracts trait usage from PHP classes
func (ue *UnifiedExtractor) extractPhpTraitUsage(node *tree_sitter.Node) {
	// Find the containing class to get context
	// For now, extract trait names as RefTypeExtends (traits are a form of composition)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)
		if childType == "name" || childType == "qualified_name" {
			traitName := string(ue.content[child.StartByte():child.EndByte()])
			// Use RefTypeExtends for traits as they extend functionality
			ref := ue.createTypeRelationshipRef(child, types.RefTypeExtends, traitName, "")
			ue.references = append(ue.references, ref)
		}
	}
}

// createTypeRelationshipRef creates a Reference for type relationships
// targetName is the base/interface being extended/implemented
// sourceName is the type that extends/implements
func (ue *UnifiedExtractor) createTypeRelationshipRef(node *tree_sitter.Node, refType types.ReferenceType, targetName, sourceName string) types.Reference {
	return ue.createTypeRelationshipRefWithQuality(node, refType, targetName, sourceName, "")
}

// createTypeRelationshipRefWithQuality creates a Reference for type relationships with explicit quality
// targetName is the base/interface being extended/implemented
// sourceName is the type that extends/implements
// quality indicates the confidence level (RefQuality* constants, empty for default)
func (ue *UnifiedExtractor) createTypeRelationshipRefWithQuality(node *tree_sitter.Node, refType types.ReferenceType, targetName, sourceName, quality string) types.Reference {
	startPoint := node.StartPosition()

	// Extract context lines around the reference
	lines := ue.getLines()
	contextStart := max(0, int(startPoint.Row)-1)
	contextEnd := min(len(lines), int(startPoint.Row)+2)
	context := lines[contextStart:contextEnd]

	ref := types.Reference{
		ID:             ue.refID,
		SourceSymbol:   0, // Will be resolved during symbol linking
		TargetSymbol:   0, // Will be resolved during symbol linking
		FileID:         0,
		Line:           int(startPoint.Row) + 1,
		Column:         int(startPoint.Column) + 1,
		Type:           refType,
		Context:        context,
		ScopeContext:   []types.ScopeInfo{},
		Strength:       types.RefStrengthTight, // Type relationships are tight coupling
		ReferencedName: targetName,
		Quality:        quality,
	}
	ue.refID++
	return ref
}

// extractGoInterfaceAssignment detects when a concrete type is assigned to an interface-typed variable
// Pattern: var w Writer = &File{} or w := &File{} where w is typed as Writer
// This creates a RefTypeImplements reference with Quality="assigned"
func (ue *UnifiedExtractor) extractGoInterfaceAssignment(node *tree_sitter.Node) {
	// var_declaration contains var_spec children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) != "var_spec" {
			continue
		}

		// var_spec has: identifier, type_identifier (optional), expression_list
		var varType string
		var concreteType string
		var typeNode *tree_sitter.Node

		for j := uint(0); j < child.ChildCount(); j++ {
			specChild := child.Child(j)
			specChildType := ue.getNodeType(specChild)

			switch specChildType {
			case "type_identifier":
				// This is the declared type (potentially an interface)
				varType = string(ue.content[specChild.StartByte():specChild.EndByte()])
				typeNode = specChild
			case "expression_list":
				// Look for concrete type in the initializer
				concreteType = ue.extractConcreteTypeFromExpr(specChild)
			}
		}

		// If we have both a type annotation and a concrete type, create the reference
		if varType != "" && concreteType != "" && typeNode != nil {
			ref := ue.createTypeRelationshipRefWithQuality(
				typeNode,
				types.RefTypeImplements,
				varType,      // target: the interface
				concreteType, // source: the concrete type
				types.RefQualityAssigned,
			)
			ue.references = append(ue.references, ref)
		}
	}
}

// extractGoTypeCast detects type assertions: x.(Writer)
// This creates a RefTypeImplements reference with Quality="cast"
func (ue *UnifiedExtractor) extractGoTypeCast(node *tree_sitter.Node) {
	// type_assertion_expression: identifier . ( type_identifier )
	// Look for the type_identifier which is the interface being cast to
	var interfaceType string
	var typeNode *tree_sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) == "type_identifier" {
			interfaceType = string(ue.content[child.StartByte():child.EndByte()])
			typeNode = child
			break
		}
	}

	if interfaceType != "" && typeNode != nil {
		// For type assertion, we don't know the concrete type at parse time
		// We only know the interface being tested - create a partial reference
		// The concrete type would need to be resolved through data flow analysis
		ref := ue.createTypeRelationshipRefWithQuality(
			typeNode,
			types.RefTypeImplements,
			interfaceType, // target: the interface being cast to
			"",            // source: unknown at parse time
			types.RefQualityCast,
		)
		ue.references = append(ue.references, ref)
	}
}

// extractGoInterfaceReturn detects when a concrete type is returned from a function with interface return type
// Pattern: func New() Writer { return &File{} }
// This creates a RefTypeImplements reference with Quality="returned"
func (ue *UnifiedExtractor) extractGoInterfaceReturn(node *tree_sitter.Node) {
	// Need to find the enclosing function and check its return type
	// Walk up the scope stack to find the function
	var funcReturnType string

	for i := len(ue.scopeStack) - 1; i >= 0; i-- {
		scope := ue.scopeStack[i]
		if scope.scopeType == types.ScopeTypeFunction || scope.scopeType == types.ScopeTypeMethod {
			// Found the enclosing function, extract its return type
			funcReturnType = ue.extractGoFunctionReturnType(scope.node)
			break
		}
	}

	if funcReturnType == "" {
		return
	}

	// Get the concrete type(s) being returned
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) == "expression_list" {
			concreteType := ue.extractConcreteTypeFromExpr(child)
			if concreteType != "" {
				ref := ue.createTypeRelationshipRefWithQuality(
					node, // Use the return_statement node for location
					types.RefTypeImplements,
					funcReturnType, // target: the interface return type
					concreteType,   // source: the concrete type being returned
					types.RefQualityReturned,
				)
				ue.references = append(ue.references, ref)
			}
		}
	}
}

// extractConcreteTypeFromExpr extracts a concrete type name from an expression
// Handles: &File{}, File{}, NewFile(), etc.
func (ue *UnifiedExtractor) extractConcreteTypeFromExpr(exprList *tree_sitter.Node) string {
	for i := uint(0); i < exprList.ChildCount(); i++ {
		child := exprList.Child(i)
		childType := ue.getNodeType(child)

		switch childType {
		case "unary_expression":
			// &File{} - pointer to composite literal
			for j := uint(0); j < child.ChildCount(); j++ {
				unaryChild := child.Child(j)
				if ue.getNodeType(unaryChild) == "composite_literal" {
					return ue.extractTypeFromCompositeLiteral(unaryChild)
				}
			}
		case "composite_literal":
			// File{} - direct composite literal
			return ue.extractTypeFromCompositeLiteral(child)
		case "call_expression":
			// NewFile() - constructor call, harder to determine type
			// For now, skip these as we'd need more analysis
			return ""
		}
	}
	return ""
}

// extractTypeFromCompositeLiteral extracts the type name from a composite literal
func (ue *UnifiedExtractor) extractTypeFromCompositeLiteral(node *tree_sitter.Node) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)
		if childType == "type_identifier" {
			return string(ue.content[child.StartByte():child.EndByte()])
		}
		// Handle qualified types like pkg.Type
		if childType == "qualified_type" {
			return string(ue.content[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// extractGoFunctionReturnType extracts the return type from a function/method declaration
func (ue *UnifiedExtractor) extractGoFunctionReturnType(funcNode *tree_sitter.Node) string {
	if funcNode == nil {
		return ""
	}

	// function_declaration children include: func, identifier, parameter_list, type_identifier/result, block
	// Look for a type_identifier that comes after the parameter lists
	foundParams := false
	for i := uint(0); i < funcNode.ChildCount(); i++ {
		child := funcNode.Child(i)
		childType := ue.getNodeType(child)

		if childType == "parameter_list" {
			foundParams = true
			continue
		}

		// After we've seen parameter_list, the next type_identifier is the return type
		if foundParams && childType == "type_identifier" {
			return string(ue.content[child.StartByte():child.EndByte()])
		}

		// Handle multiple return types in parentheses - for now just skip
		// as interface return with multiple values is less common
		if foundParams && childType == "block" {
			break
		}
	}
	return ""
}

