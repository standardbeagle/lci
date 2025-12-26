package symbollinker

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/standardbeagle/lci/internal/types"
)

// SymbolLinkerEngine is the main engine for cross-file symbol linking
type SymbolLinkerEngine struct {
	rootPath string

	// Extractors and resolvers
	extractors     map[string]SymbolExtractor // language -> extractor
	goResolver     *GoResolver
	jsResolver     *JSResolver
	phpResolver    *PHPResolver
	csharpResolver *CSharpResolver
	pythonResolver *PythonResolver

	// Symbol storage
	symbolTables    map[types.FileID]*types.SymbolTable // FileID -> SymbolTable
	fileRegistry    map[string]types.FileID             // Path -> FileID
	reverseRegistry map[types.FileID]string             // FileID -> Path
	nextFileID      types.FileID

	// Cross-file links
	symbolLinks map[types.CompositeSymbolID]*SymbolLink // Symbol -> Link info
	importLinks map[types.FileID][]*ImportLink          // FileID -> Import links

	// Thread safety
	mutex sync.RWMutex
}

// SymbolLink represents a cross-file symbol relationship
type SymbolLink struct {
	Symbol         types.CompositeSymbolID
	DefinitionFile types.FileID
	References     []*SymbolReference
	ImportedBy     []types.FileID // Files that import this symbol
	ExportedBy     types.FileID   // File that exports this symbol (if any)
	IsExternal     bool           // True if symbol is external to project
	Resolution     types.ModuleResolution
}

// SymbolReference represents a reference to a symbol
type SymbolReference struct {
	FromFile   types.FileID
	FromSymbol types.CompositeSymbolID // Symbol that makes the reference
	Location   types.SymbolLocation
	ImportPath string // Original import path
}

// ImportLink represents a resolved import relationship
type ImportLink struct {
	FromFile        types.FileID
	ImportPath      string
	ResolvedFile    types.FileID
	ImportedSymbols []string
	Resolution      types.ModuleResolution
	IsExternal      bool
}

// NewSymbolLinkerEngine creates a new symbol linker engine
func NewSymbolLinkerEngine(rootPath string) *SymbolLinkerEngine {
	engine := &SymbolLinkerEngine{
		rootPath:        rootPath,
		extractors:      make(map[string]SymbolExtractor),
		symbolTables:    make(map[types.FileID]*types.SymbolTable),
		fileRegistry:    make(map[string]types.FileID),
		reverseRegistry: make(map[types.FileID]string),
		nextFileID:      1,
		symbolLinks:     make(map[types.CompositeSymbolID]*SymbolLink),
		importLinks:     make(map[types.FileID][]*ImportLink),
	}

	// Initialize resolvers
	engine.goResolver = NewGoResolver(rootPath)
	engine.jsResolver = NewJSResolver(rootPath)
	engine.phpResolver = NewPHPResolver(rootPath)
	engine.csharpResolver = NewCSharpResolver(rootPath)
	engine.pythonResolver = NewPythonResolver(rootPath)

	// Register default extractors
	engine.RegisterExtractor(NewGoExtractor())
	engine.RegisterExtractor(NewJSExtractor())
	engine.RegisterExtractor(NewTSExtractor())
	engine.RegisterExtractor(NewPHPExtractor())
	engine.RegisterExtractor(NewCSharpExtractor())
	engine.RegisterExtractor(NewPythonExtractor())

	return engine
}

// RegisterExtractor registers a symbol extractor for a language
func (engine *SymbolLinkerEngine) RegisterExtractor(extractor SymbolExtractor) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	engine.extractors[extractor.GetLanguage()] = extractor
}

// GetOrCreateFileID gets or creates a FileID for a path
func (engine *SymbolLinkerEngine) GetOrCreateFileID(path string) types.FileID {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	if fileID, exists := engine.fileRegistry[absPath]; exists {
		return fileID
	}

	fileID := engine.nextFileID
	engine.nextFileID++

	engine.fileRegistry[absPath] = fileID
	engine.reverseRegistry[fileID] = absPath

	return fileID
}

// GetFilePath gets the file path for a FileID
func (engine *SymbolLinkerEngine) GetFilePath(fileID types.FileID) string {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	return engine.reverseRegistry[fileID]
}

// IndexFile extracts symbols from a file and adds them to the engine
func (engine *SymbolLinkerEngine) IndexFile(path string, content []byte) error {
	// Get file ID
	fileID := engine.GetOrCreateFileID(path)

	// Find appropriate extractor
	extractor := engine.findExtractor(path)
	if extractor == nil {
		return fmt.Errorf("no extractor found for file: %s", path)
	}

	// Parse with tree-sitter
	tree, err := engine.parseFile(content, extractor.GetLanguage())
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", path, err)
	}
	defer tree.Close()

	// Extract symbols
	symbolTable, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		return fmt.Errorf("failed to extract symbols from %s: %w", path, err)
	}

	// Store symbol table
	engine.mutex.Lock()
	engine.symbolTables[fileID] = symbolTable
	engine.mutex.Unlock()

	return nil
}

// LinkSymbols performs cross-file symbol linking for all indexed files
func (engine *SymbolLinkerEngine) LinkSymbols() error {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	// Update resolver registries
	engine.goResolver.SetFileRegistry(engine.fileRegistry)
	engine.jsResolver.SetFileRegistry(engine.fileRegistry)
	engine.phpResolver.SetFileRegistry(engine.fileRegistry)
	engine.csharpResolver.SetFileRegistry(engine.fileRegistry)
	engine.pythonResolver.SetFileRegistry(engine.fileRegistry)

	// Clear existing links
	engine.symbolLinks = make(map[types.CompositeSymbolID]*SymbolLink)
	engine.importLinks = make(map[types.FileID][]*ImportLink)

	// Process each file
	for fileID, symbolTable := range engine.symbolTables {
		if err := engine.processFileLinks(fileID, symbolTable); err != nil {
			return fmt.Errorf("failed to process links for file %d: %w", fileID, err)
		}
	}

	return nil
}

// processFileLinks processes all symbol links for a single file
func (engine *SymbolLinkerEngine) processFileLinks(fileID types.FileID, symbolTable *types.SymbolTable) error {
	filePath := engine.reverseRegistry[fileID]

	// Process imports
	for _, importInfo := range symbolTable.Imports {
		if err := engine.processImport(fileID, filePath, importInfo, symbolTable.Language); err != nil {
			// Continue processing other imports on error
			continue
		}
	}

	// Process exports
	for _, exportInfo := range symbolTable.Exports {
		if err := engine.processExport(fileID, exportInfo); err != nil {
			// Continue processing other exports on error
			continue
		}
	}

	// Create symbol links for all symbols in this file
	for localID, symbol := range symbolTable.Symbols {
		compositeID := types.NewCompositeSymbolID(fileID, localID)

		// Create or update symbol link
		if _, exists := engine.symbolLinks[compositeID]; !exists {
			engine.symbolLinks[compositeID] = &SymbolLink{
				Symbol:         compositeID,
				DefinitionFile: fileID,
				References:     make([]*SymbolReference, 0),
				ImportedBy:     make([]types.FileID, 0),
				ExportedBy:     0, // Will be set if exported
				IsExternal:     false,
			}
		}

		// Mark as exported if it appears in exports
		for _, exportInfo := range symbolTable.Exports {
			if exportInfo.LocalName == symbol.Name || exportInfo.ExportedName == symbol.Name {
				engine.symbolLinks[compositeID].ExportedBy = fileID
				break
			}
		}
	}

	return nil
}

// processImport processes a single import statement
func (engine *SymbolLinkerEngine) processImport(fileID types.FileID, filePath string, importInfo types.ImportInfo, language string) error {
	// Resolve the import
	var resolution types.ModuleResolution

	switch language {
	case "go":
		resolution = engine.goResolver.ResolveImport(importInfo.ImportPath, fileID)
	case "javascript", "typescript":
		resolution = engine.jsResolver.ResolveImport(importInfo.ImportPath, fileID)
	case "php":
		resolution = engine.phpResolver.ResolveImport(importInfo.ImportPath, fileID)
	case "csharp":
		resolution = engine.csharpResolver.ResolveImport(importInfo.ImportPath, fileID)
	case "python":
		resolution = engine.pythonResolver.ResolveImport(importInfo.ImportPath, fileID)
	default:
		return fmt.Errorf("unsupported language for import resolution: %s", language)
	}

	// Create import link
	importLink := &ImportLink{
		FromFile:        fileID,
		ImportPath:      importInfo.ImportPath,
		ResolvedFile:    resolution.FileID,
		ImportedSymbols: importInfo.ImportedNames,
		Resolution:      resolution,
		IsExternal:      resolution.IsExternal,
	}

	engine.importLinks[fileID] = append(engine.importLinks[fileID], importLink)

	// If the import resolves to an internal file, create cross-file references
	if !resolution.IsExternal && resolution.FileID != 0 {
		targetSymbolTable := engine.symbolTables[resolution.FileID]
		if targetSymbolTable != nil {
			engine.createImportReferences(fileID, importInfo, resolution.FileID, targetSymbolTable)
		}
	}

	return nil
}

// createImportReferences creates cross-file symbol references for an import
func (engine *SymbolLinkerEngine) createImportReferences(fromFile types.FileID, importInfo types.ImportInfo, targetFile types.FileID, targetTable *types.SymbolTable) {
	// Handle different import patterns
	if len(importInfo.ImportedNames) > 0 {
		// Named imports: import { foo, bar } from './module'
		for _, importedName := range importInfo.ImportedNames {
			engine.linkImportedSymbol(fromFile, importedName, targetFile, targetTable, importInfo.ImportPath)
		}
	} else if importInfo.IsNamespace {
		// Namespace import: import * as utils from './utils'
		// Link to all exported symbols
		for _, exportInfo := range targetTable.Exports {
			engine.linkImportedSymbol(fromFile, exportInfo.ExportedName, targetFile, targetTable, importInfo.ImportPath)
		}
	} else if importInfo.IsDefault {
		// Default import: import Component from './Component'
		// Look for default export
		for _, exportInfo := range targetTable.Exports {
			if exportInfo.IsDefault {
				engine.linkImportedSymbol(fromFile, exportInfo.LocalName, targetFile, targetTable, importInfo.ImportPath)
				break
			}
		}
	}
}

// linkImportedSymbol creates a link between an imported symbol and its definition
func (engine *SymbolLinkerEngine) linkImportedSymbol(fromFile types.FileID, symbolName string, targetFile types.FileID, targetTable *types.SymbolTable, importPath string) {
	// Find the symbol in the target file
	if localIDs, exists := targetTable.SymbolsByName[symbolName]; exists {
		for _, localID := range localIDs {
			compositeID := types.NewCompositeSymbolID(targetFile, localID)

			// Get or create symbol link
			link, exists := engine.symbolLinks[compositeID]
			if !exists {
				link = &SymbolLink{
					Symbol:         compositeID,
					DefinitionFile: targetFile,
					References:     make([]*SymbolReference, 0),
					ImportedBy:     make([]types.FileID, 0),
					ExportedBy:     0,
					IsExternal:     false,
				}
				engine.symbolLinks[compositeID] = link
			}

			// Add import reference
			if !engine.containsFileID(link.ImportedBy, fromFile) {
				link.ImportedBy = append(link.ImportedBy, fromFile)
			}

			// Create symbol reference
			reference := &SymbolReference{
				FromFile:   fromFile,
				FromSymbol: types.NewCompositeSymbolID(fromFile, 0), // Import statement itself
				ImportPath: importPath,
				// Location would need to be tracked from import statement
			}

			link.References = append(link.References, reference)
		}
	}
}

// processExport processes a single export statement
func (engine *SymbolLinkerEngine) processExport(fileID types.FileID, exportInfo types.ExportInfo) error {
	// Mark local symbols as exported
	if symbolTable := engine.symbolTables[fileID]; symbolTable != nil {
		if localIDs, exists := symbolTable.SymbolsByName[exportInfo.LocalName]; exists {
			for _, localID := range localIDs {
				compositeID := types.NewCompositeSymbolID(fileID, localID)
				if link, exists := engine.symbolLinks[compositeID]; exists {
					link.ExportedBy = fileID
				}
			}
		}
	}

	// Handle re-exports
	if exportInfo.IsReExport && exportInfo.SourcePath != "" {
		// This is a re-export: export { foo } from './other'
		// We would need to resolve the source and create appropriate links
		// For now, we'll skip re-export linking as it's complex
	}

	return nil
}

// GetSymbolDefinition gets the definition location of a symbol
func (engine *SymbolLinkerEngine) GetSymbolDefinition(symbolID types.CompositeSymbolID) (*types.SymbolLocation, error) {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	symbolTable := engine.symbolTables[symbolID.FileID]
	if symbolTable == nil {
		return nil, fmt.Errorf("no symbol table for file %d", symbolID.FileID)
	}

	symbol := symbolTable.Symbols[symbolID.LocalSymbolID]
	if symbol == nil {
		return nil, fmt.Errorf("symbol %d not found in file %d", symbolID.LocalSymbolID, symbolID.FileID)
	}

	return &symbol.Location, nil
}

// GetSymbolReferences gets all references to a symbol
func (engine *SymbolLinkerEngine) GetSymbolReferences(symbolID types.CompositeSymbolID) ([]*SymbolReference, error) {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	link := engine.symbolLinks[symbolID]
	if link == nil {
		return nil, fmt.Errorf("no link found for symbol %s", symbolID.String())
	}

	return link.References, nil
}

// GetFileImports gets all imports for a file
func (engine *SymbolLinkerEngine) GetFileImports(fileID types.FileID) ([]*ImportLink, error) {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	imports := engine.importLinks[fileID]
	if imports == nil {
		return []*ImportLink{}, nil
	}

	return imports, nil
}

// GetSymbolsInFile gets all symbols defined in a file
func (engine *SymbolLinkerEngine) GetSymbolsInFile(fileID types.FileID) (map[uint32]*types.EnhancedSymbolInfo, error) {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	symbolTable := engine.symbolTables[fileID]
	if symbolTable == nil {
		return nil, fmt.Errorf("no symbol table for file %d", fileID)
	}

	return symbolTable.Symbols, nil
}

// Helper methods

// findExtractor finds the appropriate extractor for a file
func (engine *SymbolLinkerEngine) findExtractor(path string) SymbolExtractor {
	for _, extractor := range engine.extractors {
		if extractor.CanHandle(path) {
			return extractor
		}
	}
	return nil
}

// parseFile parses a file using the appropriate tree-sitter parser
func (engine *SymbolLinkerEngine) parseFile(content []byte, language string) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	defer parser.Close()

	switch language {
	case "go":
		_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_go.Language()))
	case "javascript":
		_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_javascript.Language()))
	case "typescript":
		_ = parser.SetLanguage(sitter.NewLanguage(typescript.LanguageTypescript()))
	case "python":
		_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_python.Language()))
	case "csharp":
		_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_csharp.Language()))
	case "php":
		_ = parser.SetLanguage(sitter.NewLanguage(tree_sitter_php.LanguagePHP()))
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, errors.New("failed to parse content")
	}

	return tree, nil
}

// containsFileID checks if a slice contains a FileID
func (engine *SymbolLinkerEngine) containsFileID(slice []types.FileID, item types.FileID) bool {
	for _, id := range slice {
		if id == item {
			return true
		}
	}
	return false
}

// Stats returns statistics about the symbol linker engine
func (engine *SymbolLinkerEngine) Stats() map[string]int {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	stats := map[string]int{
		"files":        len(engine.symbolTables),
		"symbols":      len(engine.symbolLinks),
		"import_links": 0,
		"extractors":   len(engine.extractors),
	}

	for _, links := range engine.importLinks {
		stats["import_links"] += len(links)
	}

	return stats
}
