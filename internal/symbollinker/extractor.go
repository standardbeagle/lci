package symbollinker

import (
	"fmt"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// SymbolExtractor is the interface for language-specific symbol extractors
type SymbolExtractor interface {
	// ExtractSymbols extracts all symbols from the given AST
	ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error)

	// GetLanguage returns the language this extractor handles
	GetLanguage() string

	// CanHandle checks if this extractor can handle the given file
	CanHandle(filepath string) bool
}

// BaseExtractor provides common functionality for all language extractors
type BaseExtractor struct {
	language string
	fileExts []string
	mu       sync.RWMutex
}

// NewBaseExtractor creates a new base extractor
func NewBaseExtractor(language string, fileExts []string) *BaseExtractor {
	return &BaseExtractor{
		language: language,
		fileExts: fileExts,
	}
}

// GetLanguage returns the language name
func (b *BaseExtractor) GetLanguage() string {
	return b.language
}

// CanHandle checks if this extractor can handle the given file
func (b *BaseExtractor) CanHandle(filepath string) bool {
	for _, ext := range b.fileExts {
		if hasExtension(filepath, ext) {
			return true
		}
	}
	return false
}

// hasExtension checks if a filepath has the given extension
func hasExtension(filepath, ext string) bool {
	if len(filepath) < len(ext) {
		return false
	}
	return filepath[len(filepath)-len(ext):] == ext
}

// ScopeManager manages the scope hierarchy during AST traversal
type ScopeManager struct {
	currentScope *types.SymbolScope
	scopeStack   []*types.SymbolScope
}

// NewScopeManager creates a new scope manager
func NewScopeManager() *ScopeManager {
	globalScope := &types.SymbolScope{
		Type:     types.ScopeGlobal,
		Name:     "global",
		Parent:   nil,
		StartPos: 0,
		EndPos:   -1,
	}

	return &ScopeManager{
		currentScope: globalScope,
		scopeStack:   []*types.SymbolScope{globalScope},
	}
}

// PushScope enters a new scope
func (sm *ScopeManager) PushScope(scopeType types.SymbolScopeType, name string, startPos, endPos int) {
	newScope := &types.SymbolScope{
		Type:     scopeType,
		Name:     name,
		Parent:   sm.currentScope,
		StartPos: startPos,
		EndPos:   endPos,
	}

	sm.scopeStack = append(sm.scopeStack, newScope)
	sm.currentScope = newScope
}

// PopScope exits the current scope
func (sm *ScopeManager) PopScope() {
	if len(sm.scopeStack) > 1 {
		sm.scopeStack = sm.scopeStack[:len(sm.scopeStack)-1]
		sm.currentScope = sm.scopeStack[len(sm.scopeStack)-1]
	}
}

// CurrentScope returns the current scope
func (sm *ScopeManager) CurrentScope() *types.SymbolScope {
	return sm.currentScope
}

// IsInScope checks if a position is within the current scope
func (sm *ScopeManager) IsInScope(pos int) bool {
	return pos >= sm.currentScope.StartPos &&
		(sm.currentScope.EndPos == -1 || pos <= sm.currentScope.EndPos)
}

// GetScopeAtPosition returns the most specific scope at the given position
func (sm *ScopeManager) GetScopeAtPosition(pos int) *types.SymbolScope {
	// Walk backwards through scope stack to find the most specific scope
	for i := len(sm.scopeStack) - 1; i >= 0; i-- {
		scope := sm.scopeStack[i]
		if pos >= scope.StartPos && (scope.EndPos == -1 || pos <= scope.EndPos) {
			return scope
		}
	}
	return sm.scopeStack[0] // Global scope as fallback
}

// ASTTraversal provides utilities for traversing ASTs
type ASTTraversal struct {
	visitFunc func(node *sitter.Node, depth int) bool
}

// NewASTTraversal creates a new AST traversal helper
func NewASTTraversal(visitFunc func(node *sitter.Node, depth int) bool) *ASTTraversal {
	return &ASTTraversal{
		visitFunc: visitFunc,
	}
}

// Traverse walks the AST depth-first
func (at *ASTTraversal) Traverse(node *sitter.Node, depth int) {
	if node == nil {
		return
	}

	// Visit current node
	if !at.visitFunc(node, depth) {
		return // Stop traversal if visit returns false
	}

	// Visit children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		at.Traverse(child, depth+1)
	}
}

// SymbolTableBuilder helps build symbol tables
type SymbolTableBuilder struct {
	fileID        types.FileID
	language      string
	symbols       map[uint32]*types.EnhancedSymbolInfo
	symbolsByName map[string][]uint32
	imports       []types.ImportInfo
	exports       []types.ExportInfo
	nextLocalID   uint32
	mu            sync.Mutex
}

// NewSymbolTableBuilder creates a new symbol table builder
func NewSymbolTableBuilder(fileID types.FileID, language string) *SymbolTableBuilder {
	return &SymbolTableBuilder{
		fileID:        fileID,
		language:      language,
		symbols:       make(map[uint32]*types.EnhancedSymbolInfo),
		symbolsByName: make(map[string][]uint32),
		imports:       []types.ImportInfo{},
		exports:       []types.ExportInfo{},
		nextLocalID:   1,
	}
}

// AddSymbol adds a symbol to the table
func (stb *SymbolTableBuilder) AddSymbol(name string, kind types.SymbolKind, location types.SymbolLocation, scope *types.SymbolScope, isExported bool) uint32 {
	stb.mu.Lock()
	defer stb.mu.Unlock()

	localID := stb.nextLocalID
	stb.nextLocalID++

	symbol := &types.EnhancedSymbolInfo{
		ID:         types.NewCompositeSymbolID(stb.fileID, localID),
		Name:       name,
		Kind:       kind,
		Location:   location,
		Scope:      scope,
		IsExported: isExported,
		Language:   stb.language,
	}

	stb.symbols[localID] = symbol
	stb.symbolsByName[name] = append(stb.symbolsByName[name], localID)

	return localID
}

// AddImport adds an import to the table
func (stb *SymbolTableBuilder) AddImport(importInfo types.ImportInfo) {
	stb.mu.Lock()
	defer stb.mu.Unlock()

	if importInfo.LocalID == 0 {
		importInfo.LocalID = stb.nextLocalID
		stb.nextLocalID++
	}

	stb.imports = append(stb.imports, importInfo)
}

// AddExport adds an export to the table
func (stb *SymbolTableBuilder) AddExport(exportInfo types.ExportInfo) {
	stb.mu.Lock()
	defer stb.mu.Unlock()

	if exportInfo.LocalID == 0 {
		exportInfo.LocalID = stb.nextLocalID
		stb.nextLocalID++
	}

	stb.exports = append(stb.exports, exportInfo)
}

// Build creates the final symbol table
func (stb *SymbolTableBuilder) Build() *types.SymbolTable {
	stb.mu.Lock()
	defer stb.mu.Unlock()

	return &types.SymbolTable{
		FileID:        stb.fileID,
		Language:      stb.language,
		Symbols:       stb.symbols,
		Imports:       stb.imports,
		Exports:       stb.exports,
		SymbolsByName: stb.symbolsByName,
		NextLocalID:   stb.nextLocalID,
	}
}

// GetNodeText extracts text content from an AST node
func GetNodeText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	start := node.StartByte()
	end := node.EndByte()

	if start > uint(len(content)) || end > uint(len(content)) || start > end {
		return ""
	}

	return string(content[start:end])
}

// GetNodeLocation gets the location information for an AST node
func GetNodeLocation(node *sitter.Node, fileID types.FileID) types.SymbolLocation {
	if node == nil {
		return types.SymbolLocation{}
	}

	startPos := node.StartPosition()

	return types.SymbolLocation{
		FileID: fileID,
		Line:   int(startPos.Row) + 1,    // Tree-sitter uses 0-based lines
		Column: int(startPos.Column) + 1, // Tree-sitter uses 0-based columns
		Offset: int(node.StartByte()),
	}
}

// FindChildByType finds the first child node of the given type
func FindChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	if node == nil {
		return nil
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == nodeType {
			return child
		}
	}

	return nil
}

// FindChildrenByType finds all child nodes of the given type
func FindChildrenByType(node *sitter.Node, nodeType string) []*sitter.Node {
	if node == nil {
		return nil
	}

	var children []*sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == nodeType {
			children = append(children, child)
		}
	}

	return children
}

// IsExportedSymbol determines if a symbol should be exported based on language rules
type VisibilityChecker func(symbolName string, node *sitter.Node, content []byte) bool

// CommonVisibilityRules provides common visibility checking patterns
var CommonVisibilityRules = struct {
	// GoCapitalization checks if a Go symbol is exported (starts with capital letter)
	GoCapitalization VisibilityChecker

	// JavaScriptExport checks for JavaScript export keywords
	JavaScriptExport VisibilityChecker

	// PythonUnderscore checks Python naming conventions
	PythonUnderscore VisibilityChecker
}{
	GoCapitalization: func(symbolName string, node *sitter.Node, content []byte) bool {
		return len(symbolName) > 0 && symbolName[0] >= 'A' && symbolName[0] <= 'Z'
	},

	JavaScriptExport: func(symbolName string, node *sitter.Node, content []byte) bool {
		// Check if the node or its parent has an export keyword
		if node == nil {
			return false
		}

		// Check parent for export statement
		parent := node.Parent()
		if parent != nil {
			parentKind := parent.Kind()
			if parentKind == "export_statement" || parentKind == "export_default_declaration" {
				return true
			}
		}

		return false
	},

	PythonUnderscore: func(symbolName string, node *sitter.Node, content []byte) bool {
		// Python convention: names starting with _ are private
		return len(symbolName) > 0 && symbolName[0] != '_'
	},
}

// ExtractorRegistry manages all registered symbol extractors
type ExtractorRegistry struct {
	extractors map[string]SymbolExtractor
	mu         sync.RWMutex
}

// NewExtractorRegistry creates a new extractor registry
func NewExtractorRegistry() *ExtractorRegistry {
	return &ExtractorRegistry{
		extractors: make(map[string]SymbolExtractor),
	}
}

// Register registers a new symbol extractor
func (er *ExtractorRegistry) Register(language string, extractor SymbolExtractor) {
	er.mu.Lock()
	defer er.mu.Unlock()

	er.extractors[language] = extractor
}

// GetExtractor gets the extractor for a given language
func (er *ExtractorRegistry) GetExtractor(language string) (SymbolExtractor, error) {
	er.mu.RLock()
	defer er.mu.RUnlock()

	extractor, ok := er.extractors[language]
	if !ok {
		return nil, fmt.Errorf("no extractor registered for language: %s", language)
	}

	return extractor, nil
}

// GetExtractorForFile gets the appropriate extractor for a file
func (er *ExtractorRegistry) GetExtractorForFile(filepath string) (SymbolExtractor, error) {
	er.mu.RLock()
	defer er.mu.RUnlock()

	for _, extractor := range er.extractors {
		if extractor.CanHandle(filepath) {
			return extractor, nil
		}
	}

	return nil, fmt.Errorf("no extractor found for file: %s", filepath)
}

// GetSupportedLanguages returns all supported languages
func (er *ExtractorRegistry) GetSupportedLanguages() []string {
	er.mu.RLock()
	defer er.mu.RUnlock()

	languages := make([]string, 0, len(er.extractors))
	for lang := range er.extractors {
		languages = append(languages, lang)
	}

	return languages
}
