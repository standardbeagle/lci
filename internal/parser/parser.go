package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

type TreeSitterParser struct {
	parsers map[string]*tree_sitter.Parser
	queries map[string]*tree_sitter.Query
	// Phase 5: Lazy loading infrastructure
	parserMutex sync.RWMutex        // Protects parser/query initialization
	lazyInit    map[string]func()   // Language initialization functions
	initialized map[string]bool     // Track which languages are initialized
	langGroups  map[string][]string // Language group mapping for bulk initialization
	// Phase 5C: Community parser framework
	communityRegistry *CommunityParserRegistry // Community parser management
	fileContentStore  *core.FileContentStore   // Per-index file content store
	// Batching optimization: Per-file caches to reduce CGO calls (using FileID for efficiency)
	lineContentCache map[types.FileID][]string       // FileID -> pre-split lines for zero-copy operations
	parentNodeCache  map[uintptr]*tree_sitter.Node   // node ptr -> parent node
	childNodeCache   map[uintptr][]*tree_sitter.Node // node ptr -> cached children
	// Performance optimization: Track if this is a shared instance
	isShared bool
}

type BlockBoundary struct {
	Start int
	End   int
	Type  BlockType
	Name  string
	Depth int
}

type BlockType uint8

const (
	BlockTypeFunction BlockType = iota
	BlockTypeClass
	BlockTypeMethod
	BlockTypeInterface
	BlockTypeStruct
	BlockTypeBlock
)

// VisitContext maintains context during tree traversal to eliminate parent chain walking
type VisitContext struct {
	parentStack       []string         // Stack of current parent node types
	handledFunctions  map[uintptr]bool // Function node IDs already processed by call_expression
	handledProperties map[uintptr]bool // Property node IDs already processed by member_expression
	inImportStatement bool             // Whether we're currently in an import statement
	depth             int              // Current traversal depth for debugging/bounds
}

// NewVisitContext creates a new VisitContext for tree traversal
func NewVisitContext() *VisitContext {
	return &VisitContext{
		parentStack:       make([]string, 0, 10), // Pre-allocate reasonable capacity
		handledFunctions:  make(map[uintptr]bool),
		handledProperties: make(map[uintptr]bool),
		inImportStatement: false,
		depth:             0,
	}
}

// PushParent adds a parent type to the context stack
func (ctx *VisitContext) PushParent(parentType string) {
	ctx.parentStack = append(ctx.parentStack, parentType)
	ctx.depth++

	// Set context flags based on parent type
	switch parentType {
	case "import_statement":
		ctx.inImportStatement = true
	}
}

// PopParent removes the most recent parent type from the context stack
func (ctx *VisitContext) PopParent() {
	if len(ctx.parentStack) > 0 {
		removedParent := ctx.parentStack[len(ctx.parentStack)-1]
		ctx.parentStack = ctx.parentStack[:len(ctx.parentStack)-1]
		ctx.depth--

		// Clear context flags when leaving specific parent types
		switch removedParent {
		case "import_statement":
			ctx.inImportStatement = false
		}
	}
}

// GetImmediateParent returns the current parent type (top of stack)
func (ctx *VisitContext) GetImmediateParent() string {
	if len(ctx.parentStack) == 0 {
		return ""
	}
	return ctx.parentStack[len(ctx.parentStack)-1]
}

// IsInParentType checks if we're currently in any parent of the given type
func (ctx *VisitContext) IsInParentType(parentType string) bool {
	for i := len(ctx.parentStack) - 1; i >= 0; i-- {
		if ctx.parentStack[i] == parentType {
			return true
		}
	}
	return false
}

// MarkFunctionHandled marks a function node as processed by a call_expression
func (ctx *VisitContext) MarkFunctionHandled(nodeID uintptr) {
	ctx.handledFunctions[nodeID] = true
}

// MarkPropertyHandled marks a property node as processed by a member_expression
func (ctx *VisitContext) MarkPropertyHandled(nodeID uintptr) {
	ctx.handledProperties[nodeID] = true
}

// IsFunctionHandled checks if a function node was already processed
func (ctx *VisitContext) IsFunctionHandled(nodeID uintptr) bool {
	return ctx.handledFunctions[nodeID]
}

// IsPropertyHandled checks if a property node was already processed
func (ctx *VisitContext) IsPropertyHandled(nodeID uintptr) bool {
	return ctx.handledProperties[nodeID]
}

// Reset clears the context state for reuse
func (ctx *VisitContext) Reset() {
	ctx.parentStack = ctx.parentStack[:0] // Keep backing array
	ctx.depth = 0

	// Clear maps
	for k := range ctx.handledFunctions {
		delete(ctx.handledFunctions, k)
	}
	for k := range ctx.handledProperties {
		delete(ctx.handledProperties, k)
	}

	ctx.inImportStatement = false
}

func (b BlockType) String() string {
	switch b {
	case BlockTypeFunction:
		return "function"
	case BlockTypeClass:
		return "class"
	case BlockTypeMethod:
		return "method"
	case BlockTypeInterface:
		return "interface"
	case BlockTypeStruct:
		return "struct"
	case BlockTypeBlock:
		return "block"
	default:
		return "unknown"
	}
}

// Language represents the programming language for parser selection
type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageRust       Language = "rust"
	LanguageJava       Language = "java"
	LanguageCpp        Language = "cpp"
	LanguageCSharp     Language = "csharp"
	LanguageZig        Language = "zig"
	LanguagePHP        Language = "php"
)

// parserPoolData encapsulates the pool and initialization for a language
type parserPoolData struct {
	pool sync.Pool
	once sync.Once
	init func(*TreeSitterParser) // Language-specific initialization
}

// Language-specific parser pools for true parallel parsing
// Each language has its own pool to prevent contention
var parserPools = map[Language]*parserPoolData{
	LanguageGo: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".go"}, p.setupGo, "go")
		},
	},
	LanguagePython: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".py"}, p.setupPython, "python")
		},
	},
	LanguageJavaScript: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".js", ".jsx"}, p.setupJavaScript, "javascript")
		},
	},
	LanguageTypeScript: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".ts", ".tsx"}, p.setupTypeScript, "typescript")
		},
	},
	LanguageRust: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".rs"}, p.setupRust, "rust")
		},
	},
	LanguageJava: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".java"}, p.setupJava, "java")
		},
	},
	LanguageCpp: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".cpp", ".cc", ".cxx", ".c", ".h", ".hpp"}, p.setupCpp, "cpp")
		},
	},
	LanguageCSharp: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".cs"}, p.setupCSharp, "csharp")
		},
	},
	LanguageZig: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".zig"}, p.setupZig, "zig")
		},
	},
	LanguagePHP: {
		init: func(p *TreeSitterParser) {
			p.registerLazyInit([]string{".php", ".phtml"}, p.setupPHP, "php")
		},
	},
}

// getParser returns a parser from the language-specific pool
// This enables true parallel parsing of multi-language projects
func getParser(language Language, store *core.FileContentStore) *TreeSitterParser {
	data, exists := parserPools[language]
	if !exists {
		// Fallback: create general parser (will load all grammars - slower)
		p := NewTreeSitterParser()
		if store != nil {
			p.SetFileContentStore(store)
		}
		return p
	}

	// Initialize pool once per language
	data.once.Do(func() {
		data.pool.New = func() any {
			p := NewTreeSitterParser()
			data.init(p)
			return p
		}
	})

	// Get parser from pool and configure with store
	p := data.pool.Get().(*TreeSitterParser)
	if store != nil {
		p.SetFileContentStore(store)
	}
	return p
}

// Legacy parser pool for backward compatibility
// Delegates to language-specific pools
var (
	parserPool     sync.Pool
	parserPoolOnce sync.Once
)

// GetSharedParser returns a parser instance from the pool
// This avoids the overhead of creating parsers while maintaining thread safety
func GetSharedParser() *TreeSitterParser {
	parserPoolOnce.Do(func() {
		parserPool.New = func() any {
			p := NewTreeSitterParser()
			// Register all languages for backward compatibility
			p.registerLazyInit([]string{".js", ".jsx"}, p.setupJavaScript, "javascript")
			p.registerLazyInit([]string{".ts", ".tsx"}, p.setupTypeScript, "typescript")
			p.registerLazyInit([]string{".go"}, p.setupGo, "go")
			p.registerLazyInit([]string{".py"}, p.setupPython, "python")
			p.registerLazyInit([]string{".rs"}, p.setupRust, "rust")
			p.registerLazyInit([]string{".java"}, p.setupJava, "java")
			p.registerLazyInit([]string{".cpp", ".cc", ".cxx", ".c", ".h", ".hpp"}, p.setupCpp, "cpp")
			p.registerLazyInit([]string{".cs"}, p.setupCSharp, "csharp")
			p.registerLazyInit([]string{".zig"}, p.setupZig, "zig")
			p.registerLazyInit([]string{".php", ".phtml"}, p.setupPHP, "php")
			return p
		}
	})

	p := parserPool.Get().(*TreeSitterParser)
	return p
}

// GetSharedParserWithStore returns a parser from the pool configured with a FileContentStore
// Call ReleaseParser when done to return it to the pool
func GetSharedParserWithStore(store *core.FileContentStore) *TreeSitterParser {
	p := GetSharedParser()
	p.SetFileContentStore(store)
	return p
}

// ReleaseParser returns a parser to the pool for reuse
// This must be called when done with the parser
func ReleaseParser(p *TreeSitterParser) {
	if p != nil {
		parserPool.Put(p)
	}
}

// GetLanguageFromExtension returns the language name for a given file extension
func GetLanguageFromExtension(ext string) string {
	// Map extensions to language names for parser cache optimization
	switch ext {
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx", ".c", ".h", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".kt", ".kts":
		return "kotlin"
	case ".zig":
		return "zig"
	default:
		return ""
	}
}

// NewProjectSpecificParser creates a parser optimized for a specific project
// It only loads language parsers for extensions actually present in the project
func NewProjectSpecificParser(store *core.FileContentStore, projectRoot string) *TreeSitterParser {
	// Scan the project to find which languages are actually used
	usedExtensions := scanProjectForExtensions(projectRoot)

	// Create parser with only the needed language support
	p := NewTreeSitterParser()
	p.SetFileContentStore(store)

	// Only register the languages actually used in this project
	p.registerLazyInitForExtensions(usedExtensions)

	return p
}

// scanProjectForExtensions scans the project directory to find which file extensions are used
// This allows us to only load the parsers we actually need
func scanProjectForExtensions(projectRoot string) map[string]bool {
	extensions := make(map[string]bool)

	// Walk the project directory and collect all file extensions
	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != "" && len(ext) <= 5 { // Reasonable extension length
			extensions[ext] = true
		}

		return nil
	})

	return extensions
}

// registerLazyInitForExtensions registers only the language parsers for the given extensions
func (p *TreeSitterParser) registerLazyInitForExtensions(usedExtensions map[string]bool) {
	// Only register parsers for extensions that actually exist in the project
	if usedExtensions[".go"] {
		p.registerLazyInit([]string{".go"}, p.setupGo, "go")
	}
	if usedExtensions[".js"] || usedExtensions[".jsx"] {
		p.registerLazyInit([]string{".js", ".jsx"}, p.setupJavaScript, "javascript")
	}
	if usedExtensions[".ts"] || usedExtensions[".tsx"] {
		p.registerLazyInit([]string{".ts", ".tsx"}, p.setupTypeScript, "typescript")
	}
	if usedExtensions[".py"] {
		p.registerLazyInit([]string{".py"}, p.setupPython, "python")
	}
	if usedExtensions[".rs"] {
		p.registerLazyInit([]string{".rs"}, p.setupRust, "rust")
	}
	if usedExtensions[".java"] {
		p.registerLazyInit([]string{".java"}, p.setupJava, "java")
	}
	if usedExtensions[".cpp"] || usedExtensions[".cc"] || usedExtensions[".cxx"] ||
		usedExtensions[".c"] || usedExtensions[".h"] || usedExtensions[".hpp"] {
		p.registerLazyInit([]string{".cpp", ".cc", ".cxx", ".c", ".h", ".hpp"}, p.setupCpp, "cpp")
	}
	if usedExtensions[".cs"] {
		p.registerLazyInit([]string{".cs"}, p.setupCSharp, "csharp")
	}
	if usedExtensions[".zig"] {
		p.registerLazyInit([]string{".zig"}, p.setupZig, "zig")
	}
	if usedExtensions[".php"] || usedExtensions[".phtml"] {
		p.registerLazyInit([]string{".php", ".phtml"}, p.setupPHP, "php")
	}
}

// GetParserForLanguage returns a parser instance for the specified language
func GetParserForLanguage(language string, store *core.FileContentStore) *TreeSitterParser {
	// Convert string to Language type
	lang := Language(language)
	return getParser(lang, store)
}

// ReleaseParserToPool returns a parser to its language-specific pool
// Call this when done with a language-specific parser
func ReleaseParserToPool(p *TreeSitterParser, language Language) {
	if p == nil {
		return
	}

	data, exists := parserPools[language]
	if !exists {
		// Language not in our pool map, just put in general pool
		parserPool.Put(p)
		return
	}

	data.pool.Put(p)
}

func NewTreeSitterParser() *TreeSitterParser {
	p := &TreeSitterParser{
		parsers:           make(map[string]*tree_sitter.Parser),
		queries:           make(map[string]*tree_sitter.Query),
		lazyInit:          make(map[string]func()),
		initialized:       make(map[string]bool),
		langGroups:        make(map[string][]string),
		communityRegistry: NewCommunityParserRegistry(), // Phase 5C: Community parser support
		// Batching optimization: Initialize caches
		lineContentCache: make(map[types.FileID][]string),
		parentNodeCache:  make(map[uintptr]*tree_sitter.Node),
		childNodeCache:   make(map[uintptr][]*tree_sitter.Node),
	}

	// Phase 5: Register lazy initialization functions - only for languages we have new bindings for
	p.registerLazyInit([]string{".js", ".jsx"}, p.setupJavaScript, "javascript")
	p.registerLazyInit([]string{".ts", ".tsx"}, p.setupTypeScript, "typescript")
	p.registerLazyInit([]string{".go"}, p.setupGo, "go")
	p.registerLazyInit([]string{".py"}, p.setupPython, "python")
	p.registerLazyInit([]string{".rs"}, p.setupRust, "rust")
	p.registerLazyInit([]string{".java"}, p.setupJava, "java")
	p.registerLazyInit([]string{".cpp", ".cc", ".cxx", ".c", ".h", ".hpp"}, p.setupCpp, "cpp")
	p.registerLazyInit([]string{".cs"}, p.setupCSharp, "csharp")
	// Phase 5C: Zig community parser support
	p.registerLazyInit([]string{".zig"}, p.setupZig, "zig")
	// PHP support
	p.registerLazyInit([]string{".php", ".phtml"}, p.setupPHP, "php")

	// Phase 5C: Setup community parsers
	p.setupCommunityParsers()

	return p
}

// SetFileContentStore sets the file content store for this parser instance
func (p *TreeSitterParser) SetFileContentStore(store *core.FileContentStore) {
	p.fileContentStore = store
}

// setupQueryForLanguage provides consistent error handling for tree-sitter query setup
func (p *TreeSitterParser) setupQueryForLanguage(language string, extensions []string, queryStr string, tsLanguage *tree_sitter.Language) error {
	query, _ := tree_sitter.NewQuery(tsLanguage, queryStr)
	// Note: tree-sitter Go bindings don't return errors reliably, so we check query directly
	if query == nil {
		return fmt.Errorf("failed to setup %s query: query creation returned nil", language)
	}

	// Register query for all extensions
	for _, ext := range extensions {
		p.queries[ext] = query
	}

	return nil
}

// Phase 5: registerLazyInit registers a lazy initialization function for multiple extensions
func (p *TreeSitterParser) registerLazyInit(extensions []string, initFunc func(), langGroup string) {
	for _, ext := range extensions {
		p.lazyInit[ext] = initFunc
	}
	p.langGroups[langGroup] = extensions
}

// Phase 5: ensureParserInitialized initializes parser on first use (30% memory reduction)
func (p *TreeSitterParser) ensureParserInitialized(ext string) bool {
	// Fast path: already initialized (read-only check)
	p.parserMutex.RLock()
	if p.initialized[ext] {
		p.parserMutex.RUnlock()
		return true
	}
	// Don't release read lock yet to prevent race condition

	// Check if we have an init function before acquiring write lock
	initFunc, hasInitFunc := p.lazyInit[ext]
	p.parserMutex.RUnlock()

	if !hasInitFunc {
		return false
	}

	// Slow path: need to initialize (acquire write lock)
	p.parserMutex.Lock()
	defer p.parserMutex.Unlock()

	// Double-check after acquiring write lock (another goroutine might have initialized)
	if p.initialized[ext] {
		return true
	}

	// Safe to initialize - we hold the write lock
	initFunc()

	// Mark all related extensions as initialized by finding the language group
	for _, extensions := range p.langGroups {
		for _, groupExt := range extensions {
			if groupExt == ext {
				// Found the language group, mark all extensions in this group as initialized
				for _, relatedExt := range extensions {
					p.initialized[relatedExt] = true
				}
				return true
			}
		}
	}

	// Fallback: mark just this extension as initialized
	p.initialized[ext] = true
	return true
}

// Phase 5: GetLanguageFromExtension returns language name for cache optimization
func (p *TreeSitterParser) GetLanguageFromExtension(ext string) string {
	// Phase 5: Map extensions to language names for parser cache optimization
	switch ext {
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx", ".c", ".h", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".cs": // Phase 5A: C# language support
		return "csharp"
	case ".kt", ".kts": // Phase 5B: Kotlin language support
		return "kotlin"
	case ".zig": // Phase 5C: Zig language support
		return "zig"
	default:
		return "unknown"
	}
}

// GetSupportedLanguages returns a list of all supported languages
func (p *TreeSitterParser) GetSupportedLanguages() []string {
	p.parserMutex.RLock()
	defer p.parserMutex.RUnlock()

	languages := make([]string, 0, len(p.langGroups))
	for lang := range p.langGroups {
		languages = append(languages, lang)
	}
	return languages
}

func (p *TreeSitterParser) ParseFile(path string, content []byte) ([]types.BlockBoundary, []types.Symbol, []types.Import) {
	blocks, symbols, imports, _, _, _ := p.ParseFileEnhanced(path, content)
	return blocks, symbols, imports
}

// ParseFileWithContext parses a file with context cancellation support
func (p *TreeSitterParser) ParseFileWithContext(ctx context.Context, path string, content []byte) ([]types.BlockBoundary, []types.Symbol, []types.Import) {
	blocks, symbols, imports, _, _, _ := p.ParseFileEnhancedWithContext(ctx, path, content)
	return blocks, symbols, imports
}

// ParseFileEnhanced extracts enhanced symbols with relational data (Phase 5: with lazy loading)
func (p *TreeSitterParser) ParseFileEnhanced(path string, content []byte) ([]types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	return p.ParseFileEnhancedWithContext(context.Background(), path, content)
}

// ParseFileEnhancedWithAST extracts enhanced symbols and returns the AST for storage
func (p *TreeSitterParser) ParseFileEnhancedWithAST(path string, content []byte) (*tree_sitter.Tree, []types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	return p.ParseFileEnhancedWithASTAndContext(context.Background(), path, content)
}

// ParseFileEnhancedWithASTAndContext extracts enhanced symbols and returns the AST with context cancellation support
func (p *TreeSitterParser) ParseFileEnhancedWithASTAndContext(ctx context.Context, path string, content []byte) (*tree_sitter.Tree, []types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	return p.ParseFileEnhancedWithASTAndContextStringRef(ctx, path, content, 0)
}

// DeclarationLookup is an interface for looking up declaration metadata.
// UnifiedExtractor implements this interface.
type DeclarationLookup interface {
	// Lookup retrieves signature and doc comment for a symbol at given position (1-based line/column)
	Lookup(line, column int) (signature string, docComment string)
}

// DeclarationInfo stores extracted metadata for a declaration
type DeclarationInfo struct {
	Signature  string
	DocComment string
}

// isDeclarationNode checks if a node type represents a declaration
func isDeclarationNode(nodeType string) bool {
	declarationTypes := []string{
		"function_declaration",
		"method_declaration",
		"function_definition",
		"method_definition",
		"class_declaration",
		"class_definition",
		"type_declaration",
		"interface_declaration",
		"struct_declaration",
		"enum_declaration",
		"const_declaration",
		"var_declaration",
	}

	for _, declType := range declarationTypes {
		if nodeType == declType {
			return true
		}
	}
	return false
}

// ParseFileEnhancedWithASTAndContextStringRef extracts enhanced symbols using StringRef for zero-copy operations
func (p *TreeSitterParser) ParseFileEnhancedWithASTAndContextStringRef(ctx context.Context, path string, content []byte, fileID types.FileID) (*tree_sitter.Tree, []types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	ext := filepath.Ext(path)

	// Phase 5: Ensure parser is initialized on first use (30% memory reduction)
	if !p.ensureParserInitialized(ext) {
		return nil, nil, nil, nil, nil, nil, nil
	}

	parser, ok := p.parsers[ext]
	if !ok {
		return nil, nil, nil, nil, nil, nil, nil
	}

	// Add protection against tree-sitter crashes with proper error logging
	defer func() {
		if r := recover(); r != nil {
			// CRITICAL: Log parser panics instead of silently swallowing them
			// This helps identify problematic files and parsing issues
			debug.LogIndexing("TREE-SITTER PANIC in file %s: %v", path, r)
			// Don't return nil - let the calling code handle the parsing failure
		}
	}()

	// CRITICAL: Tree-sitter C library mutates input buffers via CGO
	// Make defensive copy to protect FileContentStore immutability
	// This is the ONLY place content should be copied (copy-on-parse pattern)
	parserBuffer := make([]byte, len(content))
	copy(parserBuffer, content)

	tree := parser.Parse(parserBuffer, nil)
	if tree == nil {
		return nil, nil, nil, nil, nil, nil, nil
	}

	// UNIFIED SINGLE-PASS EXTRACTION
	// Uses UnifiedExtractor to extract ALL data in one tree walk:
	// - Symbols, blocks, imports (previously in extractBasicSymbolsStringRef)
	// - Scopes, references, declarations, type relationships
	// - Complexity metrics
	// Performance impact: ~60% CGO reduction by eliminating redundant ts_node_* calls
	extractor := NewUnifiedExtractor(p, parserBuffer, fileID, ext, path)
	extractor.Extract(tree)

	// Get all results from unified extraction
	symbols, blocks, imports, scopeInfo, references, _, complexityMap := extractor.GetAllResults()

	// Build enhanced symbols from unified extraction data
	// Declaration lookup is provided by UnifiedExtractor
	enhancedSymbols := p.buildEnhancedSymbols(parserBuffer, symbols, references, scopeInfo, extractor, complexityMap)

	// Return the AST along with all extracted data
	return tree, blocks, symbols, imports, enhancedSymbols, references, scopeInfo
}

// ParseFileWithPerfData parses a file and returns performance analysis data alongside standard results.
// This method extracts loops, awaits, and calls during AST traversal for performance anti-pattern detection.
func (p *TreeSitterParser) ParseFileWithPerfData(ctx context.Context, path string, content []byte, fileID types.FileID) (*tree_sitter.Tree, []types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo, []types.FunctionPerfData) {
	ext := filepath.Ext(path)

	// Phase 5: Ensure parser is initialized on first use (30% memory reduction)
	if !p.ensureParserInitialized(ext) {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}

	parser, ok := p.parsers[ext]
	if !ok {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}

	// Add protection against tree-sitter crashes with proper error logging
	defer func() {
		if r := recover(); r != nil {
			debug.LogIndexing("TREE-SITTER PANIC in file %s: %v", path, r)
		}
	}()

	// CRITICAL: Tree-sitter C library mutates input buffers via CGO
	parserBuffer := make([]byte, len(content))
	copy(parserBuffer, content)

	tree := parser.Parse(parserBuffer, nil)
	if tree == nil {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}

	// UNIFIED SINGLE-PASS EXTRACTION with performance tracking
	extractor := NewUnifiedExtractor(p, parserBuffer, fileID, ext, path)
	extractor.Extract(tree)

	// Get all results from unified extraction
	symbols, blocks, imports, scopeInfo, references, _, complexityMap := extractor.GetAllResults()

	// Get performance analysis data
	perfResults := extractor.GetPerfAnalysisResults()

	// Convert to types.FunctionPerfData
	perfData := convertPerfResultsToTypes(perfResults)

	// Build enhanced symbols from unified extraction data
	enhancedSymbols := p.buildEnhancedSymbols(parserBuffer, symbols, references, scopeInfo, extractor, complexityMap)

	return tree, blocks, symbols, imports, enhancedSymbols, references, scopeInfo, perfData
}

// ParseFileWithSideEffects parses a file and extracts all data including side effect analysis.
// This is the most comprehensive parsing method, enabling function purity detection.
func (p *TreeSitterParser) ParseFileWithSideEffects(ctx context.Context, path string, content []byte, fileID types.FileID) (
	tree *tree_sitter.Tree,
	blocks []types.BlockBoundary,
	symbols []types.Symbol,
	imports []types.Import,
	enhancedSymbols []types.EnhancedSymbol,
	references []types.Reference,
	scopeInfo []types.ScopeInfo,
	perfData []types.FunctionPerfData,
	sideEffects map[string]*types.SideEffectInfo,
) {
	ext := filepath.Ext(path)

	// Ensure parser is initialized
	if !p.ensureParserInitialized(ext) {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}

	parser, ok := p.parsers[ext]
	if !ok {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}

	// Add protection against tree-sitter crashes
	defer func() {
		if r := recover(); r != nil {
			debug.LogIndexing("TREE-SITTER PANIC in file %s: %v", path, r)
		}
	}()

	// CRITICAL: Tree-sitter C library mutates input buffers via CGO
	parserBuffer := make([]byte, len(content))
	copy(parserBuffer, content)

	tree = parser.Parse(parserBuffer, nil)
	if tree == nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}

	// UNIFIED SINGLE-PASS EXTRACTION with side effect tracking enabled
	extractor := NewUnifiedExtractor(p, parserBuffer, fileID, ext, path)
	extractor.EnableSideEffectTracking() // Enable side effect analysis
	extractor.Extract(tree)

	// Get all results from unified extraction
	symbols, blocks, imports, scopeInfo, references, _, complexityMap := extractor.GetAllResults()

	// Get performance analysis data
	perfResults := extractor.GetPerfAnalysisResults()
	perfData = convertPerfResultsToTypes(perfResults)

	// Get side effect analysis results
	sideEffects = extractor.GetSideEffectResults()

	// Build enhanced symbols from unified extraction data
	enhancedSymbols = p.buildEnhancedSymbols(parserBuffer, symbols, references, scopeInfo, extractor, complexityMap)

	return tree, blocks, symbols, imports, enhancedSymbols, references, scopeInfo, perfData, sideEffects
}

// convertPerfResultsToTypes converts parser.PerfAnalysisResult to types.FunctionPerfData
func convertPerfResultsToTypes(results []PerfAnalysisResult) []types.FunctionPerfData {
	perfData := make([]types.FunctionPerfData, len(results))
	for i, r := range results {
		// Convert loops
		loops := make([]types.LoopData, len(r.Loops))
		for j, l := range r.Loops {
			loops[j] = types.LoopData{
				NodeType:  l.NodeType,
				StartLine: l.StartLine,
				EndLine:   l.EndLine,
				Depth:     l.Depth,
			}
		}

		// Convert awaits
		awaits := make([]types.AwaitData, len(r.Awaits))
		for j, a := range r.Awaits {
			awaits[j] = types.AwaitData{
				Line:        a.Line,
				AssignedVar: a.AssignedVar,
				CallTarget:  a.CallTarget,
				UsedVars:    a.UsedVars,
			}
		}

		// Convert calls
		calls := make([]types.CallData, len(r.Calls))
		for j, c := range r.Calls {
			calls[j] = types.CallData{
				Target:    c.Target,
				Line:      c.Line,
				InLoop:    c.InLoop,
				LoopDepth: c.LoopDepth,
				LoopLine:  c.LoopLine,
			}
		}

		perfData[i] = types.FunctionPerfData{
			Name:      r.FunctionName,
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			IsAsync:   r.IsAsync,
			Language:  r.Language,
			Loops:     loops,
			Awaits:    awaits,
			Calls:     calls,
		}
	}
	return perfData
}

// ParseFileEnhancedWithContext extracts enhanced symbols with context cancellation support
func (p *TreeSitterParser) ParseFileEnhancedWithContext(ctx context.Context, path string, content []byte) ([]types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	ext := filepath.Ext(path)

	// Phase 5: Ensure parser is initialized on first use (30% memory reduction)
	if !p.ensureParserInitialized(ext) {
		return nil, nil, nil, nil, nil, nil
	}

	parser, ok := p.parsers[ext]
	if !ok {
		return nil, nil, nil, nil, nil, nil
	}

	// Add protection against tree-sitter crashes with proper error logging
	defer func() {
		if r := recover(); r != nil {
			// CRITICAL: Log parser panics instead of silently swallowing them
			// This helps identify problematic files and parsing issues
			debug.LogIndexing("TREE-SITTER PANIC in file %s: %v", path, r)
			// Don't return nil - let the calling code handle the parsing failure
		}
	}()

	// CRITICAL: Tree-sitter C library mutates input buffers via CGO
	// Make defensive copy to protect FileContentStore immutability
	// This is the ONLY place content should be copied (copy-on-parse pattern)
	parserBuffer := make([]byte, len(content))
	copy(parserBuffer, content)

	tree := parser.Parse(parserBuffer, nil)
	if tree == nil {
		return nil, nil, nil, nil, nil, nil
	}
	defer tree.Close()

	// UNIFIED SINGLE-PASS EXTRACTION
	// Uses UnifiedExtractor to extract ALL data in one tree walk:
	// - Symbols, blocks, imports (previously in extractBasicSymbolsStringRef)
	// - Scopes, references, declarations, type relationships
	// - Complexity metrics
	// Performance impact: ~60% CGO reduction by eliminating redundant ts_node_* calls
	extractor := NewUnifiedExtractor(p, parserBuffer, 0, ext, path)
	extractor.Extract(tree)

	// Get all results from unified extraction
	symbols, blocks, imports, scopeInfo, references, _, complexityMap := extractor.GetAllResults()

	// Build enhanced symbols using unified extraction data
	// Declaration lookup is provided by UnifiedExtractor
	enhancedSymbols := p.buildEnhancedSymbols(parserBuffer, symbols, references, scopeInfo, extractor, complexityMap)

	return blocks, symbols, imports, enhancedSymbols, references, scopeInfo
}

// New FileID-based methods using FileContentStore

// ParseFileFromStore parses a file using FileID and FileContentStore
func (p *TreeSitterParser) ParseFileFromStore(path string, fileID types.FileID) ([]types.BlockBoundary, []types.Symbol, []types.Import) {
	blocks, symbols, imports, _, _, _ := p.ParseFileEnhancedFromStore(path, fileID)
	return blocks, symbols, imports
}

// ParseFileEnhancedFromStore extracts enhanced symbols using FileID and FileContentStore
func (p *TreeSitterParser) ParseFileEnhancedFromStore(path string, fileID types.FileID) ([]types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	return p.ParseFileEnhancedFromStoreWithContext(context.Background(), path, fileID)
}

// ParseFileEnhancedFromStoreWithContext extracts enhanced symbols with context cancellation support using FileContentStore
func (p *TreeSitterParser) ParseFileEnhancedFromStoreWithContext(ctx context.Context, path string, fileID types.FileID) ([]types.BlockBoundary, []types.Symbol, []types.Import, []types.EnhancedSymbol, []types.Reference, []types.ScopeInfo) {
	// Get content from FileContentStore
	var content []byte
	var ok bool

	if p.fileContentStore == nil {
		// Parser not properly initialized - return empty results
		return nil, nil, nil, nil, nil, nil
	}

	content, ok = p.fileContentStore.GetContent(fileID)

	if !ok {
		return nil, nil, nil, nil, nil, nil
	}

	// Use existing content-based method
	return p.ParseFileEnhancedWithContext(ctx, path, content)
}

// extractBasicSymbolsStringRef extracts symbols using StringRef for zero-copy operations
// Also returns complexity map computed during extraction (avoids separate tree walk)
// @lci:call-frequency[once-per-file]
func (p *TreeSitterParser) extractBasicSymbolsStringRef(tree *tree_sitter.Tree, content []byte, fileID types.FileID, ext string) ([]types.BlockBoundary, []types.Symbol, []types.Import, map[PositionKey]int) {
	query := p.queries[ext]
	if query == nil {
		// No query available for this file extension, return empty results
		return nil, nil, nil, nil
	}
	qc := tree_sitter.NewQueryCursor()
	defer qc.Close()
	queryMatches := qc.Matches(query, tree.RootNode(), content)

	var blocks []types.BlockBoundary
	var symbols []types.Symbol
	var imports []types.Import
	complexityMap := make(map[PositionKey]int) // OPTIMIZATION: Build during extraction

	captureNames := query.CaptureNames()

	// Pre-allocate capturedNames map outside loop and reuse it
	// Typical captures have 2-4 .name entries, so capacity of 4 is sufficient
	capturedNames := make(map[string]types.StringRef, 4)

	for {
		match := queryMatches.Next()
		if match == nil {
			break
		}

		// Clear the map for reuse (avoids allocation per iteration)
		for k := range capturedNames {
			delete(capturedNames, k)
		}

		// Collect captured names from this match for name resolution using StringRef
		for _, c := range match.Captures {
			captureName := captureNames[c.Index]
			if strings.Contains(captureName, ".name") {
				// Extract the captured name as StringRef for zero-copy operations
				start := int(c.Node.StartByte())
				length := int(c.Node.EndByte()) - start
				nameRef := types.NewStringRef(fileID, content, start, length)
				capturedNames[captureName] = nameRef
			}
		}

		for _, c := range match.Captures {
			node := c.Node
			captureName := captureNames[c.Index]

			// Only process main captures, not sub-captures like .name
			switch captureName {
			case "function":
				block, symbol := p.parseFunctionStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)
				// OPTIMIZATION: Compute complexity during extraction (already have the node)
				key := PositionKey{Line: symbol.Line, Column: symbol.Column}
				complexityMap[key] = calculateCyclomaticComplexity(&node)

			case "method":
				block, symbol := p.parseMethodStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)
				// OPTIMIZATION: Compute complexity during extraction (already have the node)
				key := PositionKey{Line: symbol.Line, Column: symbol.Column}
				complexityMap[key] = calculateCyclomaticComplexity(&node)

			case "variable":
				block, symbol := p.parseVariableStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "class":
				block, symbol := p.parseClassStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "interface":
				block, symbol := p.parseInterfaceStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "type":
				block, symbol := p.parseTypeAliasStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "import":
				imp := p.parseImportStringRef(&node, content, fileID, captureName, capturedNames)
				if imp != nil {
					imports = append(imports, *imp)
				}

			// Phase 4: New language constructs
			case "struct":
				block, symbol := p.parseStructStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "impl":
				block, symbol := p.parseImplStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "trait":
				block, symbol := p.parseTraitStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "module":
				block, symbol := p.parseModuleStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "namespace":
				block, symbol := p.parseNamespaceStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "constructor":
				block, symbol := p.parseConstructorStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "package":
				imp := p.parsePackageStringRef(&node, content, fileID, captureName, capturedNames)
				if imp != nil {
					imports = append(imports, *imp)
				}

			case "include":
				imp := p.parseIncludeStringRef(&node, content, fileID, captureName, capturedNames)
				if imp != nil {
					imports = append(imports, *imp)
				}

			// C# specific construct - using directive
			case "using":
				imp := p.parseUsingStringRef(&node, content, fileID, captureName, capturedNames)
				if imp != nil {
					imports = append(imports, *imp)
				}

			// Phase 5A: Additional C# constructs
			case "record":
				block, symbol := p.parseRecordStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "property":
				_, symbol := p.parsePropertyStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "event":
				_, symbol := p.parseEventStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "delegate":
				_, symbol := p.parseDelegateStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "enum":
				block, symbol := p.parseEnumStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "field":
				_, symbol := p.parseFieldStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "enum_member":
				_, symbol := p.parseEnumMemberStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			// Phase 5B: Additional Kotlin constructs
			case "object":
				block, symbol := p.parseObjectStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "companion":
				block, symbol := p.parseCompanionObjectStringRef(&node, content, fileID, captureName, capturedNames)
				blocks = append(blocks, block)
				symbols = append(symbols, symbol)

			case "typealias":
				_, symbol := p.parseTypeAliasStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "enum_entry":
				_, symbol := p.parseEnumEntryStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			// Additional language constructs
			case "annotation":
				_, symbol := p.parseAnnotationStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "template":
				_, symbol := p.parseTemplateStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)

			case "macro":
				_, symbol := p.parseMacroStringRef(&node, content, fileID, captureName, capturedNames)
				symbols = append(symbols, symbol)
			}
		}
	}

	return blocks, symbols, imports, complexityMap
}

// extractReferencedSymbolName extracts the name of the referenced symbol from an AST node
// Used by UnifiedExtractor to populate ReferencedName in references
func (p *TreeSitterParser) extractReferencedSymbolName(node *tree_sitter.Node, content []byte) string {
	return p.extractReferencedSymbolNameWithType(node, content, node.Kind())
}

// extractReferencedSymbolNameWithType extracts the name using a pre-fetched node type
// This avoids redundant CGO calls when the node type is already known
func (p *TreeSitterParser) extractReferencedSymbolNameWithType(node *tree_sitter.Node, content []byte, nodeType string) string {
	switch nodeType {
	case "identifier", "field_identifier", "type_identifier", "property_identifier":
		// Simple identifier - return the text directly
		return string(content[node.StartByte():node.EndByte()])

	case "selector_expression":
		// Go: pkg.Function or obj.method
		// Get the field (right side) which is the actual referenced symbol
		if fieldNode := node.ChildByFieldName("field"); fieldNode != nil {
			return string(content[fieldNode.StartByte():fieldNode.EndByte()])
		}

	case "member_expression":
		// JavaScript: obj.method
		// Get the property (right side)
		if propertyNode := node.ChildByFieldName("property"); propertyNode != nil {
			return string(content[propertyNode.StartByte():propertyNode.EndByte()])
		}

	case "call_expression":
		// Function call - extract the function name
		if functionNode := node.ChildByFieldName("function"); functionNode != nil {
			return p.extractReferencedSymbolName(functionNode, content)
		}

	case "attribute":
		// Python: obj.attr
		if attrNode := node.ChildByFieldName("attribute"); attrNode != nil {
			return string(content[attrNode.StartByte():attrNode.EndByte()])
		}
	}

	// Fallback: return the full node text (will be cleaned up later)
	fullText := string(content[node.StartByte():node.EndByte()])

	// For qualified names like "types.NewRuneTrigram", extract the last part
	// Using LastIndex instead of Split to avoid allocation
	if lastDot := strings.LastIndex(fullText, "."); lastDot >= 0 {
		return fullText[lastDot+1:] // Return "NewRuneTrigram" from "types.NewRuneTrigram"
	}

	return fullText
}

// getLanguageFromExt returns the language name for a file extension
func (p *TreeSitterParser) getLanguageFromExt(ext string) string {
	switch ext {
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx", ".c":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	default:
		return "unknown"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectContextAttributes detects context-altering attributes that affect code behavior
func (p *TreeSitterParser) detectContextAttributes(node *tree_sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	// Check for preceding string literals and decorators
	attributes = append(attributes, p.checkParentDirectives(node, content)...)

	// Check the function node itself for various attributes
	attributes = append(attributes, p.checkNodeTypeAttributes(node, content)...)

	// Check function modifiers
	attributes = append(attributes, p.checkChildModifiers(node, content)...)

	return attributes
}

// getLineContentCache returns cached line-split content for a file
// Uses core.SplitLinesWithCapacity for pre-allocated, zero-reallocation line splitting
func (p *TreeSitterParser) getLineContentCache(fileID types.FileID, content []byte) []string {
	if lines, exists := p.lineContentCache[fileID]; exists {
		return lines
	}
	// Use optimized line splitting with pre-counted capacity
	lines := core.SplitLinesWithCapacity(content)
	p.lineContentCache[fileID] = lines
	return lines
}

// getParentNodeCached returns cached parent node to avoid repeated CGO Parent() calls
func (p *TreeSitterParser) getParentNodeCached(node *tree_sitter.Node) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	nodePtr := node.Id()
	if parent, exists := p.parentNodeCache[nodePtr]; exists {
		return parent
	}
	parent := node.Parent()
	p.parentNodeCache[nodePtr] = parent
	return parent
}

// getChildNodesCached returns cached children array to avoid repeated CGO Child() calls
func (p *TreeSitterParser) getChildNodesCached(node *tree_sitter.Node) []*tree_sitter.Node {
	if node == nil {
		return nil
	}
	nodePtr := node.Id()
	if children, exists := p.childNodeCache[nodePtr]; exists {
		return children
	}
	childCount := int(node.ChildCount())
	children := make([]*tree_sitter.Node, childCount)
	for i := 0; i < childCount; i++ {
		children[i] = node.Child(uint(i))
	}
	p.childNodeCache[nodePtr] = children
	return children
}

// clearParsingCache clears per-file parsing caches to free memory
func (p *TreeSitterParser) clearParsingCache(fileID types.FileID) {
	delete(p.lineContentCache, fileID)
	// Note: Node caches will be garbage collected when nodes are no longer referenced
}

// checkParentDirectives checks for directives and decorators in parent/sibling nodes
func (p *TreeSitterParser) checkParentDirectives(node *tree_sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	// Use cached parent to avoid CGO Parent() call
	parent := p.getParentNodeCached(node)
	if parent == nil {
		return attributes
	}

	// Use cached children to avoid multiple CGO Child() calls
	children := p.getChildNodesCached(parent)
	for i, child := range children {
		if child == node {
			// Look at previous siblings (max 5 back)
			maxBack := i - 5
			if maxBack < 0 {
				maxBack = 0
			}
			for j := i - 1; j >= maxBack; j-- {
				sibling := children[j]
				siblingType := sibling.Kind()

				// Check for string literals like "use server", "use client"
				if p.isStringLiteral(siblingType) {
					text := string(content[sibling.StartByte():sibling.EndByte()])
					text = strings.Trim(text, `"';`)

					if p.isDirective(text) {
						attributes = append(attributes, types.ContextAttribute{
							Type:  types.AttrTypeDirective,
							Value: text,
							Line:  int(sibling.StartPosition().Row) + 1,
						})
					}
				}

				// Check for decorators
				if p.isDecorator(siblingType) {
					decoratorText := string(content[sibling.StartByte():sibling.EndByte()])
					attributes = append(attributes, types.ContextAttribute{
						Type:  types.AttrTypeDecorator,
						Value: decoratorText,
						Line:  int(sibling.StartPosition().Row) + 1,
					})
				}
			}
			break
		}
	}

	return attributes
}

// isStringLiteral checks if node type is a string literal
func (p *TreeSitterParser) isStringLiteral(nodeType string) bool {
	return nodeType == "string" || nodeType == "string_literal" || nodeType == "expression_statement"
}

// isDirective checks if text is a known directive
func (p *TreeSitterParser) isDirective(text string) bool {
	directives := map[string]bool{
		"use server": true,
		"use client": true,
		"use strict": true,
	}
	return directives[text]
}

// isDecorator checks if node type is a decorator
func (p *TreeSitterParser) isDecorator(nodeType string) bool {
	return nodeType == "decorator" || nodeType == "annotation"
}

// checkNodeTypeAttributes checks for node-type specific attributes
func (p *TreeSitterParser) checkNodeTypeAttributes(node *tree_sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	nodeType := node.Kind()
	line := int(node.StartPosition().Row) + 1

	// Check for async functions
	if p.isAsyncNode(nodeType) {
		attributes = append(attributes, types.ContextAttribute{
			Type:  types.AttrTypeAsync,
			Value: "async",
			Line:  line,
		})
	}

	// Check for generator functions
	if p.isGeneratorNode(nodeType) {
		attributes = append(attributes, types.ContextAttribute{
			Type:  types.AttrTypeGenerator,
			Value: "function*",
			Line:  line,
		})
	}

	// Also check if the node text contains function* pattern
	nodeText := string(content[node.StartByte():node.EndByte()])
	if strings.Contains(nodeText, "function*") && !p.hasAttribute(attributes, types.AttrTypeGenerator) {
		attributes = append(attributes, types.ContextAttribute{
			Type:  types.AttrTypeGenerator,
			Value: "function*",
			Line:  line,
		})
	}

	return attributes
}

// isAsyncNode checks if node type indicates async
func (p *TreeSitterParser) isAsyncNode(nodeType string) bool {
	return nodeType == "async_function" || nodeType == "async_arrow_function" || nodeType == "async_method"
}

// isGeneratorNode checks if node type indicates generator
func (p *TreeSitterParser) isGeneratorNode(nodeType string) bool {
	return nodeType == "generator_function" || nodeType == "generator_function_declaration"
}

// hasAttribute checks if attributes already contain a specific type
func (p *TreeSitterParser) hasAttribute(attributes []types.ContextAttribute, attrType types.ContextAttributeType) bool {
	for _, attr := range attributes {
		if attr.Type == attrType {
			return true
		}
	}
	return false
}

// checkChildModifiers checks for modifiers in child nodes
func (p *TreeSitterParser) checkChildModifiers(node *tree_sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := child.Kind()
		childText := string(content[child.StartByte():child.EndByte()])
		line := int(child.StartPosition().Row) + 1

		// Check for various modifiers
		if modifierAttr := p.getModifierAttribute(childText, line); modifierAttr != nil {
			attributes = append(attributes, *modifierAttr)
		}

		// Check for async modifier
		if childType == "async" || childText == "async" {
			attributes = append(attributes, types.ContextAttribute{
				Type:  types.AttrTypeAsync,
				Value: "async",
				Line:  line,
			})
		}

		// Check for generator (function* or yield)
		if childType == "generator" || strings.Contains(childText, "yield") {
			attributes = append(attributes, types.ContextAttribute{
				Type:  types.AttrTypeIterator,
				Value: "generator",
				Line:  line,
			})
		}

		// Check for modifiers node
		if childType == "modifiers" {
			attributes = append(attributes, p.checkModifiersNode(child, content)...)
		}
	}

	return attributes
}

// getModifierAttribute maps modifier text to ContextAttribute
func (p *TreeSitterParser) getModifierAttribute(modText string, line int) *types.ContextAttribute {
	modifiers := map[string]types.ContextAttributeType{
		"unsafe":   types.AttrTypeUnsafe,
		"static":   types.AttrTypeStatic,
		"const":    types.AttrTypeConst,
		"inline":   types.AttrTypeInline,
		"virtual":  types.AttrTypeVirtual,
		"abstract": types.AttrTypeAbstract,
		"final":    types.AttrTypeFinal,
		"sealed":   types.AttrTypeFinal,
		"export":   types.AttrTypeExported,
		"pub":      types.AttrTypeExported,
		"public":   types.AttrTypeExported,
	}

	if attrType, ok := modifiers[modText]; ok {
		return &types.ContextAttribute{
			Type:  attrType,
			Value: modText,
			Line:  line,
		}
	}

	return nil
}

// checkModifiersNode checks for modifiers in a modifiers node
func (p *TreeSitterParser) checkModifiersNode(modifiersNode *tree_sitter.Node, content []byte) []types.ContextAttribute {
	var attributes []types.ContextAttribute

	for j := uint(0); j < modifiersNode.ChildCount(); j++ {
		modifier := modifiersNode.Child(j)
		modText := string(content[modifier.StartByte():modifier.EndByte()])
		line := int(modifier.StartPosition().Row) + 1

		if modifierAttr := p.getModifierAttribute(modText, line); modifierAttr != nil {
			attributes = append(attributes, *modifierAttr)
		}
	}

	return attributes
}

// PositionKey is a zero-allocation key type for line:column lookups
// Replaces fmt.Sprintf("%d:%d") which allocates ~500KB per 1000 symbols
type PositionKey struct {
	Line   int
	Column int
}

// buildEnhancedSymbols creates EnhancedSymbols from basic symbols with reference data
// Populates Signature and DocComment fields from AST during parsing
//
// REFACTORED: Split into focused helper functions for better maintainability
//
// Performance optimizations (see docs/performance/optimization-opportunities.md):
//   - Uses PositionKey struct instead of fmt.Sprintf (500MB savings)
//   - Pre-allocates enhancedSymbols slice (275MB savings)
//   - Pre-allocates scopeChain slice (75MB savings)
//   - Lazy-initializes ByType map (10MB savings)
//
// buildEnhancedSymbols builds enhanced symbols using a pre-computed complexity map
// Complexity map is pre-computed during symbol extraction, avoiding extra tree walk
// declLookup implements DeclarationLookup for signature/doc comment retrieval
func (p *TreeSitterParser) buildEnhancedSymbols(content []byte, symbols []types.Symbol, references []types.Reference, scopeInfo []types.ScopeInfo, declLookup DeclarationLookup, complexityMap map[PositionKey]int) []types.EnhancedSymbol {
	// Pre-allocate slice with known capacity (275MB savings)
	enhancedSymbols := make([]types.EnhancedSymbol, 0, len(symbols))

	// Build reference maps for quick lookup
	incomingRefs, outgoingRefs := buildReferenceMaps(references)

	// Build symbol map for position-based lookups
	symbolMap := buildSymbolMap(symbols)

	// Create scope chain cache to avoid rebuilding identical chains (554MB → ~100MB savings)
	// Estimated capacity: typical files have 50-200 unique line positions with symbols
	scopeChainCache := make(map[int][]types.ScopeInfo, len(symbols)/3)

	// Create enhanced symbols using the helper functions
	for i, symbol := range symbols {
		enhancedSymbol := p.buildSingleEnhancedSymbol(
			i,
			symbol,
			symbolMap,
			incomingRefs,
			outgoingRefs,
			scopeInfo,
			declLookup,
			complexityMap,
			scopeChainCache,
		)
		enhancedSymbols = append(enhancedSymbols, enhancedSymbol)
	}

	return enhancedSymbols
}

// buildReferenceMaps creates position-keyed maps for incoming and outgoing references
func buildReferenceMaps(references []types.Reference) (map[PositionKey][]types.Reference, map[PositionKey][]types.Reference) {
	incomingRefs := make(map[PositionKey][]types.Reference)
	outgoingRefs := make(map[PositionKey][]types.Reference)

	for _, ref := range references {
		// Map references from their source location (where the reference appears)
		sourceKey := PositionKey{Line: ref.Line, Column: ref.Column}
		outgoingRefs[sourceKey] = append(outgoingRefs[sourceKey], ref)

		// For incoming references, we would need target symbol location
		// This is simplified for now - incoming refs would need symbol resolution
	}

	return incomingRefs, outgoingRefs
}

// buildSymbolMap creates a position-keyed map for quick symbol lookup
func buildSymbolMap(symbols []types.Symbol) map[PositionKey]*types.Symbol {
	symbolMap := make(map[PositionKey]*types.Symbol, len(symbols))
	for i := range symbols {
		symbol := &symbols[i]
		key := PositionKey{Line: symbol.Line, Column: symbol.Column}
		symbolMap[key] = symbol
	}
	return symbolMap
}

// buildSingleEnhancedSymbol creates a single enhanced symbol using pre-computed complexity map
// Uses O(1) complexity lookup from map populated during symbol extraction
func (p *TreeSitterParser) buildSingleEnhancedSymbol(
	idx int,
	symbol types.Symbol,
	symbolMap map[PositionKey]*types.Symbol,
	incomingRefs map[PositionKey][]types.Reference,
	outgoingRefs map[PositionKey][]types.Reference,
	scopeInfo []types.ScopeInfo,
	declLookup DeclarationLookup,
	complexityMap map[PositionKey]int,
	scopeChainCache map[int][]types.ScopeInfo,
) types.EnhancedSymbol {
	symbolKey := PositionKey{Line: symbol.Line, Column: symbol.Column}

	// Build scope chain for this symbol (with caching)
	scopeChain := buildScopeChainForSymbol(symbol, scopeInfo, scopeChainCache)

	// Calculate reference statistics
	incoming := incomingRefs[symbolKey]
	outgoing := outgoingRefs[symbolKey]

	// Build reference statistics
	refStats := buildReferenceStatistics(incoming, outgoing)

	// Extract signature and doc comment using declaration lookup
	signature, docComment := declLookup.Lookup(symbol.Line, symbol.Column)

	// Look up pre-computed cyclomatic complexity for functions and methods (O(1) lookup)
	complexity := 0
	if symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeMethod {
		complexity = complexityMap[symbolKey] // Default to 0 if not found
	}

	return types.EnhancedSymbol{
		Symbol:       symbol,
		ID:           types.SymbolID(idx + 1), // Simple ID assignment
		IncomingRefs: incoming,
		OutgoingRefs: outgoing,
		ScopeChain:   scopeChain,
		RefStats:     refStats,
		Metrics:      nil, // Metrics will be calculated by analysis package if needed

		// Populate metadata fields during parsing
		Signature:  signature,  // From single-pass extraction
		DocComment: docComment, // From single-pass extraction
		IsExported: p.isSymbolExported(symbol.Name),
		Complexity: complexity, // Cyclomatic complexity for functions/methods
	}
}

// calculateCyclomaticComplexity calculates cyclomatic complexity for a function node
// This is a simplified inline version to avoid circular dependencies with analysis package
func calculateCyclomaticComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 1 // Base complexity
	}
	complexity := 1
	walkNodeForCyclomatic(node, &complexity)
	return complexity
}

// walkNodeForCyclomatic recursively walks the AST counting decision points
func walkNodeForCyclomatic(node *tree_sitter.Node, complexity *int) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Decision points that increase cyclomatic complexity
	switch nodeType {
	// If statements (all languages)
	case "if_statement", "if_expression":
		*complexity++

	// For/while loops (all languages)
	case "for_statement", "for_range_statement", "for_in_statement":
		*complexity++
	case "while_statement", "do_while_statement":
		*complexity++

	// Case clauses (all add a path)
	case "case_clause", "case_statement": // Generic
		*complexity++
	case "expression_case", "type_case": // Go specific
		*complexity++

	// Ternary/conditional expressions
	case "conditional_expression", "ternary_expression":
		*complexity++

	// Logical operators in binary expressions
	case "binary_expression":
		if node.ChildCount() >= 3 {
			operatorNode := node.Child(1)
			if operatorNode != nil {
				operator := operatorNode.Kind()
				if operator == "&&" || operator == "||" || operator == "and" || operator == "or" {
					*complexity++
				}
			}
		}

	// Exception handling
	case "catch_clause", "except_clause":
		*complexity++
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		walkNodeForCyclomatic(child, complexity)
	}
}

// buildScopeChainForSymbol finds all scopes that contain the symbol
// Uses cache to avoid rebuilding identical scope chains (554MB → ~100MB savings)
func buildScopeChainForSymbol(symbol types.Symbol, scopeInfo []types.ScopeInfo, cache map[int][]types.ScopeInfo) []types.ScopeInfo {
	// Create cache key from symbol line
	// Symbols on the same line share the same scope chain
	line := symbol.Line

	// Check cache first
	if cached, found := cache[line]; found {
		return cached
	}

	// Pre-allocate with max depth estimate: 75MB savings
	scopeChain := make([]types.ScopeInfo, 0, 8) // Max nesting depth rarely >8
	for _, scope := range scopeInfo {
		// Check if symbol is within this scope
		if line >= scope.StartLine && line <= scope.EndLine {
			scopeChain = append(scopeChain, scope)
		}
	}

	// Cache the result for reuse
	cache[line] = scopeChain
	return scopeChain
}

// buildReferenceStatistics creates RefStats from incoming and outgoing references
func buildReferenceStatistics(incoming []types.Reference, outgoing []types.Reference) types.RefStats {
	// Extract unique file IDs from references
	incomingFileIDs := extractUniqueFileIDs(incoming)
	outgoingFileIDs := extractUniqueFileIDs(outgoing)

	// Lazy-initialize ByType map only if references exist (10MB savings)
	var byType map[string]int
	if len(incoming) > 0 || len(outgoing) > 0 {
		byType = make(map[string]int, 4) // Typical: import, call, field, method
	}

	// Build RefStats with proper structure
	return types.RefStats{
		Total: types.RefCount{
			IncomingCount: len(incoming),
			OutgoingCount: len(outgoing),
			IncomingFiles: incomingFileIDs,
			OutgoingFiles: outgoingFileIDs,
			ByType:        byType,
			Strength:      types.RefStrengthStats{}, // Default empty
		},
	}
}

// extractUniqueFileIDs extracts unique file IDs from a list of references
func extractUniqueFileIDs(refs []types.Reference) []types.FileID {
	fileIDMap := make(map[types.FileID]bool)
	for _, ref := range refs {
		fileIDMap[ref.FileID] = true
	}

	result := make([]types.FileID, 0, len(fileIDMap))
	for fileID := range fileIDMap {
		result = append(result, fileID)
	}
	return result
}

// Additional parsing methods for new language constructs

func (p *TreeSitterParser) parseAnnotation(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["annotation.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeClass, // Annotations are like special classes
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseTemplate(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// For templates, extract the whole declaration as the name
	name := string(content[node.StartByte():node.EndByte()])
	if len(name) > 50 { // Truncate long template declarations
		name = name[:50] + "..."
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeType, // Templates are type-like
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass, // Templates are type-like, use Class
		Name:  name,
	}

	return block, symbol
}

func (p *TreeSitterParser) parseMacro(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) (types.BlockBoundary, types.Symbol) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if capturedName, exists := capturedNames["macro.name"]; exists {
		name = capturedName
	} else if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(content[nameNode.StartByte():nameNode.EndByte()])
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeFunction, // Macros are function-like
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeFunction,
		Name:  name,
	}

	return block, symbol
}

// parseUsing parses C# using directives
func (p *TreeSitterParser) parseUsing(node *tree_sitter.Node, content []byte, captureName string, capturedNames map[string]string) *types.Import {
	startPoint := node.StartPosition()

	var path string
	if capturedName, exists := capturedNames["using.name"]; exists {
		path = capturedName
	}

	if path == "" {
		return nil
	}

	return &types.Import{
		Path: path,
		Line: int(startPoint.Row) + 1,
	}
}

// isSymbolExported determines if a symbol is exported based on naming conventions
func (p *TreeSitterParser) isSymbolExported(name string) bool {
	if name == "" {
		return false
	}

	// Go convention: exported if starts with uppercase
	firstChar := rune(name[0])
	if firstChar >= 'A' && firstChar <= 'Z' {
		return true
	}

	// TypeScript/JavaScript: not prefixed with _ or #
	if strings.HasPrefix(name, "_") || strings.HasPrefix(name, "#") {
		return false
	}

	// Python: not prefixed with _
	if strings.HasPrefix(name, "_") {
		return false
	}

	// Default: assume exported
	return true
}
