package parser

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/standardbeagle/lci/internal/types"
)

func (p *TreeSitterParser) parseFunction(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	return p.parseFunctionStringRef(node, content, 0, captureName, nil)
}

// parseFunctionStringRef parses a function using StringRef for zero-copy operations
func (p *TreeSitterParser) parseFunctionStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	// First try to get name from captured names (for languages like C++)
	if nameRef, exists := capturedNames["function.name"]; exists {
		name = nameRef.String(content)
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		// Fallback to field-based name extraction (for languages like JS/Go)
		start := int(nameNode.StartByte())
		length := int(nameNode.EndByte()) - start
		nameRef := types.NewStringRef(fileID, content, start, length)
		name = nameRef.String(content)
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeFunction,
		Name:  name,
	}

	// Detect context-altering attributes
	attributes := p.detectContextAttributesStringRef(node, content, fileID)

	symbol := types.Symbol{
		Name:       name,
		Type:       types.SymbolTypeFunction,
		Line:       int(startPoint.Row) + 1,
		Column:     int(startPoint.Column) + 1,
		EndLine:    int(endPoint.Row) + 1,
		EndColumn:  int(endPoint.Column) + 1,
		Attributes: attributes,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseMethod(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	// First try to get name from captured names (for languages like C++)
	if capturedName, exists := capturedNames["method.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		// Fallback to field-based name extraction (for languages like JS/Go)
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeMethod,
		Name:  name,
	}

	// Detect context-altering attributes
	attributes := p.detectContextAttributes(node, content)

	symbol := types.Symbol{
		Name:       name,
		Type:       types.SymbolTypeMethod,
		Line:       int(startPoint.Row) + 1,
		Column:     int(startPoint.Column) + 1,
		EndLine:    int(endPoint.Row) + 1,
		EndColumn:  int(endPoint.Column) + 1,
		Attributes: attributes,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseVariable(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	// First try to get name from captured names
	if capturedName, exists := capturedNames["variable.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		// Fallback to field-based name extraction
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeVariable,
		Name:  name,
	}

	// Detect context-altering attributes
	attributes := p.detectContextAttributes(node, content)

	symbol := types.Symbol{
		Name:       name,
		Type:       types.SymbolTypeVariable,
		Line:       int(startPoint.Row) + 1,
		Column:     int(startPoint.Column) + 1,
		EndLine:    int(endPoint.Row) + 1,
		EndColumn:  int(endPoint.Column) + 1,
		Attributes: attributes,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseClass(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	// First try to get name from captured names (for languages like C++)
	if capturedName, exists := capturedNames["class.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		// Fallback to field-based name extraction (for languages like JS/Go)
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeClass,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseInterface(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeInterface,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeInterface,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseTypeAlias(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string

	// For Go type_declaration, the name is in type_spec -> type_identifier
	if node.Kind() == "type_declaration" {
		// Find the type_spec child
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "type_spec" {
				// Find the type_identifier within type_spec
				for j := uint(0); j < child.ChildCount(); j++ {
					grandchild := child.Child(j)
					if grandchild.Kind() == "type_identifier" {
						name = string(content[grandchild.StartByte():grandchild.EndByte()])
						break
					}
				}
				break
			}
		}
	} else {
		// For other languages, try to find name field directly
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name = string(content[nameNode.StartByte():nameNode.EndByte()])
		}
	}

	// Will be updated with correct type below
	var block types.BlockBoundary

	// Determine the correct symbol type for Go type declarations
	symbolType := types.SymbolTypeType    // Default to type
	blockType := types.BlockTypeInterface // Default block type

	// For Go type declarations, check if it's a struct
	if node.Kind() == "type_declaration" {
		// Look for struct_type in the type_spec
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "type_spec" {
				for j := uint(0); j < child.ChildCount(); j++ {
					grandchild := child.Child(j)
					if grandchild.Kind() == "struct_type" {
						symbolType = types.SymbolTypeStruct
						blockType = types.BlockTypeStruct
						break
					} else if grandchild.Kind() == "interface_type" {
						symbolType = types.SymbolTypeInterface
						blockType = types.BlockTypeInterface
						break
					}
				}
				break
			}
		}
	}

	// Create the block with the determined type
	block = types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  blockType,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      symbolType,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseImport(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) *types.Import {
	// For Go imports - look for path field
	if pathNode := node.ChildByFieldName("path"); pathNode != nil {
		path := string(content[pathNode.StartByte():pathNode.EndByte()])
		// Remove quotes
		path = strings.Trim(path, `"'`)

		return &types.Import{
			Path: path,
			Line: int(node.StartPosition().Row) + 1,
		}
	}

	// For JavaScript/TypeScript imports - look for source field
	if sourceNode := node.ChildByFieldName("source"); sourceNode != nil {
		source := string(content[sourceNode.StartByte():sourceNode.EndByte()])
		// Remove quotes
		source = strings.Trim(source, `"'`)

		return &types.Import{
			Path: source,
			Line: int(node.StartPosition().Row) + 1,
		}
	}

	// For Python imports
	if node.Kind() == "import_statement" || node.Kind() == "import_from_statement" {
		importText := string(content[node.StartByte():node.EndByte()])
		return &types.Import{
			Path: importText,
			Line: int(node.StartPosition().Row) + 1,
		}
	}

	return nil
}

// Phase 4: New language parsing methods

func (p *TreeSitterParser) parseStruct(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	} else if capturedName, exists := capturedNames["struct.name"]; exists {
		// Fallback to captured name for community parsers like Zig
		name = capturedName
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeStruct,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeStruct,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseImpl(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("type"); nameNode != nil {
		name = "impl " + string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Use class type for impl blocks
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeClass,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseTrait(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeInterface,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeInterface,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseModule(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Use class type for modules
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeModule,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseNamespace(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Use class type for namespaces
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeNamespace,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseConstructor(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeMethod,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeMethod,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parsePackage(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) *types.Import {
	// Java package declaration
	packageText := string(content[node.StartByte():node.EndByte()])
	return &types.Import{
		Path: packageText,
		Line: int(node.StartPosition().Row) + 1,
	}
}

func (p *TreeSitterParser) parseInclude(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) *types.Import {
	// C/C++ include directive
	if pathNode := node.ChildByFieldName("path"); pathNode != nil {
		path := string(content[pathNode.StartByte():pathNode.EndByte()])
		// Remove quotes and angle brackets
		path = strings.Trim(path, `"'<>`)

		return &types.Import{
			Path: path,
			Line: int(node.StartPosition().Row) + 1,
		}
	}
	return nil
}

// Phase 5A: Additional C# parsing methods

func (p *TreeSitterParser) parseRecord(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["record.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Records are similar to classes
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeClass, // Use class type for records
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseProperty(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["property.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{} // Properties don't create blocks

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeProperty,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseEvent(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["event.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{} // Events don't create blocks

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeEvent,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseDelegate(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["delegate.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{} // Delegates don't create blocks

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeType, // Use type for delegates
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseEnum(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["enum.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Use class type for enums
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeEnum,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseField(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["field.name"]; exists {
		name = capturedName
	}

	block := types.BlockBoundary{} // Fields don't create blocks

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeField,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseEnumMember(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["enum_member.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{} // Enum members don't create blocks

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeEnumMember,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

// Phase 5B: Additional Kotlin parsing methods

func (p *TreeSitterParser) parseObject(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["object.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Objects are similar to classes
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeObject,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseCompanionObject(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["companion.name"]; exists {
		name = capturedName
	} else {
		// Companion objects might not have a name
		name = "Companion"
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeCompanion,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseEnumEntry(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["enum_entry.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	block := types.BlockBoundary{} // Enum entries don't create blocks

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeEnumMember,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	return block, symbol
}

// extractRelationalData extracts enhanced symbols with references and scope information
func (p *TreeSitterParser) extractRelationalData(tree *tree_sitter.Tree, content []byte, ext, path string) ([]types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	enhanced, refs, scopes, _ := p.extractRelationalDataStringRef(tree, content, 0, ext, path)
	return enhanced, refs, scopes
}

// extractRelationalDataStringRef extracts enhanced symbols with references and scope information using StringRef
// OPTIMIZATION: Uses UnifiedExtractor for single-pass AST traversal, consolidating what were previously
// separate tree walks for scopes, references, declarations, and type relationships.
// This reduces CGO overhead by ~50% by eliminating redundant ts_node_* calls.
// Returns the UnifiedExtractor as DeclarationLookup for declaration signature/doc comment lookup
func (p *TreeSitterParser) extractRelationalDataStringRef(tree *tree_sitter.Tree, content []byte, fileID types.FileID, ext, path string) ([]types.EnhancedSymbol, []types.Reference, []types.ScopeInfo, DeclarationLookup) {
	var enhancedSymbols []types.EnhancedSymbol

	// Use UnifiedExtractor for single-pass extraction of scopes, references, declarations, and type relationships
	// This consolidates buildScopeHierarchy + extractReferences + declarations + extractTypeRelationships into one walk
	extractor := NewUnifiedExtractor(p, content, fileID, ext, path)
	extractor.Extract(tree)

	// Get results from the unified extraction
	scopeInfo, references, _, _ := extractor.GetResults()

	// Enhanced symbols are now built in the calling function using buildEnhancedSymbols
	// This separation allows for better modularity and testing
	// Return the extractor as DeclarationLookup for signature/doc comment lookup

	return enhancedSymbols, references, scopeInfo, extractor
}

// detectContextAttributesStringRef detects context-altering attributes using StringRef
func (p *TreeSitterParser) detectContextAttributesStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID) []types.ContextAttribute {
	// For now, delegate to the existing method - can be optimized further with StringRef in future iterations
	return p.detectContextAttributes(node, content)
}

// Simplified StringRef-based parsing methods
// REFACTORED: Replaced 30+ duplicate wrapper methods with generic adapter

// parseMethodStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseMethodStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseMethod(node, content, captureName, sc)
		})
}

// parseVariableStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseVariableStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseVariable(node, content, captureName, sc)
		})
}

// parseClassStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseClassStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseClass(node, content, captureName, sc)
		})
}

// parseInterfaceStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseInterfaceStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseInterface(node, content, captureName, sc)
		})
}

// parseTypeAliasStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseTypeAliasStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseTypeAlias(node, content, captureName, sc)
		})
}

// parseImportStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseImportStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) *types.Import {
	return p.parseImportWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) *types.Import {
			return p.parseImport(node, content, captureName, sc)
		})
}

// parseStructStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseStructStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseStruct(node, content, captureName, sc)
		})
}

// parseImplStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseImplStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseImpl(node, content, captureName, sc)
		})
}

// parseTraitStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseTraitStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseTrait(node, content, captureName, sc)
		})
}

// parseModuleStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseModuleStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseModule(node, content, captureName, sc)
		})
}

// parseNamespaceStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseNamespaceStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseNamespace(node, content, captureName, sc)
		})
}

// parseConstructorStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseConstructorStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseConstructor(node, content, captureName, sc)
		})
}

// parsePackageStringRef delegates to the generic adapter
func (p *TreeSitterParser) parsePackageStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) *types.Import {
	return p.parseImportWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) *types.Import {
			return p.parsePackage(node, content, captureName, sc)
		})
}

// parseIncludeStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseIncludeStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) *types.Import {
	return p.parseImportWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) *types.Import {
			return p.parseInclude(node, content, captureName, sc)
		})
}

// parseUsingStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseUsingStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) *types.Import {
	return p.parseImportWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) *types.Import {
			return p.parseUsing(node, content, captureName, sc)
		})
}

// parseRecordStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseRecordStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseRecord(node, content, captureName, sc)
		})
}

// parsePropertyStringRef delegates to the generic adapter
func (p *TreeSitterParser) parsePropertyStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseProperty(node, content, captureName, sc)
		})
}

// parseEventStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseEventStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseEvent(node, content, captureName, sc)
		})
}

// parseDelegateStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseDelegateStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseDelegate(node, content, captureName, sc)
		})
}

// parseEnumStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseEnumStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseEnum(node, content, captureName, sc)
		})
}

// parseFieldStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseFieldStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseField(node, content, captureName, sc)
		})
}

// parseEnumMemberStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseEnumMemberStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseEnumMember(node, content, captureName, sc)
		})
}

// parseObjectStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseObjectStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseObject(node, content, captureName, sc)
		})
}

// parseCompanionObjectStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseCompanionObjectStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseCompanionObject(node, content, captureName, sc)
		})
}

// parseAnnotationStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseAnnotationStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseAnnotation(node, content, captureName, sc)
		})
}

// parseTemplateStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseTemplateStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseTemplate(node, content, captureName, sc)
		})
}

// parseMacroStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseMacroStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseMacro(node, content, captureName, sc)
		})
}

// parseEnumEntryStringRef delegates to the generic adapter
func (p *TreeSitterParser) parseEnumEntryStringRef(node *tree_sitter.Node, content []byte, fileID types.FileID, captureName string, capturedNames map[string]types.StringRef) (types.BlockBoundary, types.Symbol) {
	return p.parseWithStringRefAdapter(node, content, captureName, capturedNames,
		func(sc map[string]string) (types.BlockBoundary, types.Symbol) {
			return p.parseEnumEntry(node, content, captureName, sc)
		})
}

// Generic adapter for parsing methods that return (BlockBoundary, Symbol)
func (p *TreeSitterParser) parseWithStringRefAdapter(
	node *tree_sitter.Node,
	content []byte,
	captureName string,
	capturedNames map[string]types.StringRef,
	delegateFunc func(map[string]string) (types.BlockBoundary, types.Symbol),
) (types.BlockBoundary, types.Symbol) {
	stringCapturedNames := convertStringRefToStringMap(capturedNames, content)
	return delegateFunc(stringCapturedNames)
}

// Generic adapter for parsing methods that return *Import
func (p *TreeSitterParser) parseImportWithStringRefAdapter(
	node *tree_sitter.Node,
	content []byte,
	captureName string,
	capturedNames map[string]types.StringRef,
	delegateFunc func(map[string]string) *types.Import,
) *types.Import {
	stringCapturedNames := convertStringRefToStringMap(capturedNames, content)
	return delegateFunc(stringCapturedNames)
}

// Helper function to convert StringRef map to string map
func convertStringRefToStringMap(stringRefs map[string]types.StringRef, content []byte) map[string]string {
	if stringRefs == nil {
		return nil
	}
	result := make(map[string]string, len(stringRefs))
	for k, v := range stringRefs {
		result[k] = v.String(content)
	}
	return result
}
