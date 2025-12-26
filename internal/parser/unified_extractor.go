package parser

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// extractorPool is a sync.Pool for reusing UnifiedExtractor instances
// This reduces allocation overhead by ~15% (162MB savings per 1000 files)
var extractorPool = sync.Pool{
	New: func() interface{} {
		return &UnifiedExtractor{
			scopes:              make([]types.ScopeInfo, 0, 32),
			references:          make([]types.Reference, 0, 128),
			declarations:        make(map[string]DeclarationInfo, 32),
			complexity:          make(map[PositionKey]int, 16),
			symbols:             make([]types.Symbol, 0, 64),
			blocks:              make([]types.BlockBoundary, 0, 32),
			imports:             make([]types.Import, 0, 16),
			scopeStack:          make([]scopeStackEntry, 0, 8),
			handledNodes:        make(map[uintptr]bool, 64),
			nodeTypeCache:       make(map[uintptr]string, 256),
			loopStack:           make([]loopStackEntry, 0, 4),
			perfAnalysisResults: make([]PerfAnalysisResult, 0, 8),
			sideEffectResults:   make(map[string]*types.SideEffectInfo),
		}
	},
}

// UnifiedExtractor performs all AST extraction in a single tree walk.
// This consolidates what were previously 6+ separate operations:
// 1. extractNestedScopes - scope hierarchy extraction
// 2. extractReferences - reference extraction (language-specific)
// 3. declaration metadata extraction (signatures, doc comments)
// 4. walkNodeForCyclomatic - complexity calculation per function
// 5. detectContextAttributes - attribute detection
// 6. extractBasicSymbols - symbol, block, and import extraction
//
// Performance impact: ~60% CPU reduction by eliminating redundant CGO calls
// (ts_node_child, ts_node_child_count, ts_node_type were called 6x per node)
type UnifiedExtractor struct {
	// Input data
	content []byte
	fileID  types.FileID
	ext     string
	path    string
	parser  *TreeSitterParser // Reference to parser for helper methods

	// Output collections - pre-allocated for efficiency
	scopes       []types.ScopeInfo
	references   []types.Reference
	declarations map[string]DeclarationInfo
	complexity   map[PositionKey]int

	// Symbol extraction output (consolidated from extractBasicSymbolsStringRef)
	symbols []types.Symbol
	blocks  []types.BlockBoundary
	imports []types.Import

	// Cached data for efficiency
	lines []string // Pre-split content lines for context extraction

	// Context tracking during traversal (replaces parent chain walking)
	scopeStack        []scopeStackEntry // Current scope hierarchy during traversal
	refID             uint64            // Incrementing reference ID
	handledNodes      map[uintptr]bool  // Nodes already processed (for JS context-aware extraction)
	inImportContext   bool              // Track if we're in an import statement
	currentLevel      int               // Current scope nesting level
	inTraitOrImplBody bool              // Track if we're inside a trait or impl body (Rust)
	inClassBody       bool              // Track if we're inside a class body (Python, JS, etc.)

	// Complexity tracking during traversal (replaces separate calculateCyclomaticComplexity walk)
	complexityStack []int // Stack of complexity counts for nested functions
	currentFuncKey  *PositionKey

	// Node type caches - batch Kind() lookups to reduce CGO
	nodeTypeCache map[uintptr]string

	// Performance anti-pattern tracking during traversal
	loopStack           []loopStackEntry       // Track nested loops during traversal
	awaitExpressions    []awaitExprInfo        // Await expressions in current function
	callsInCurrentFunc  []callInFuncInfo       // Function calls in current function
	currentFuncAnalysis *functionAnalysisState // Current function being analyzed
	perfAnalysisResults []PerfAnalysisResult   // Collected performance analysis per function

	// Side effect tracking during traversal
	sideEffectTracker *SideEffectTracker               // Tracks side effects for purity analysis
	sideEffectResults map[string]*types.SideEffectInfo // Results keyed by "file:line"
	enableSideEffects bool                             // Whether to track side effects (opt-in)
}

// loopStackEntry tracks a loop during traversal for performance analysis
type loopStackEntry struct {
	nodeType  string
	startLine int
	endLine   int
	depth     int // Nesting depth (1 = outermost)
}

// awaitExprInfo tracks an await expression for analysis
type awaitExprInfo struct {
	line        int
	assignedVar string   // Variable receiving the await result
	callTarget  string   // Function/method being awaited
	usedVars    []string // Variables referenced in arguments
}

// callInFuncInfo tracks a function call for expensive-call-in-loop detection
type callInFuncInfo struct {
	target    string // Function/method name
	line      int
	inLoop    bool
	loopDepth int // Depth of containing loop (0 if not in loop)
	loopLine  int // Start line of containing loop
}

// functionAnalysisState tracks the current function being analyzed
type functionAnalysisState struct {
	name      string
	startLine int
	endLine   int
	isAsync   bool
	loops     []loopStackEntry
	awaits    []awaitExprInfo
	calls     []callInFuncInfo
}

// PerfAnalysisResult holds performance analysis for a function
type PerfAnalysisResult struct {
	FunctionName string
	StartLine    int
	EndLine      int
	IsAsync      bool
	Language     string
	FilePath     string
	Loops        []LoopInfo
	Awaits       []AwaitInfo
	Calls        []CallInfo
}

// LoopInfo exported for use by performance analyzer
type LoopInfo struct {
	NodeType  string
	StartLine int
	EndLine   int
	Depth     int
}

// AwaitInfo exported for use by performance analyzer
type AwaitInfo struct {
	Line        int
	AssignedVar string
	CallTarget  string
	UsedVars    []string
}

// CallInfo exported for use by performance analyzer
type CallInfo struct {
	Target    string
	Line      int
	InLoop    bool
	LoopDepth int
	LoopLine  int
}

// scopeStackEntry tracks a scope during traversal
type scopeStackEntry struct {
	scopeType types.ScopeType
	name      string
	node      *tree_sitter.Node
	startLine int
	endLine   int
}

// NewUnifiedExtractor creates a new unified extractor for single-pass AST processing
func NewUnifiedExtractor(parser *TreeSitterParser, content []byte, fileID types.FileID, ext, path string) *UnifiedExtractor {
	return &UnifiedExtractor{
		content:       content,
		fileID:        fileID,
		ext:           ext,
		path:          path,
		parser:        parser,
		scopes:        make([]types.ScopeInfo, 0, 32),       // Pre-allocate for typical file
		references:    make([]types.Reference, 0, 128),      // Pre-allocate for typical file
		declarations:  make(map[string]DeclarationInfo, 32), // Pre-allocate
		complexity:    make(map[PositionKey]int, 16),        // Pre-allocate for functions
		symbols:       make([]types.Symbol, 0, 64),          // Pre-allocate for typical file
		blocks:        make([]types.BlockBoundary, 0, 32),   // Pre-allocate for typical file
		imports:       make([]types.Import, 0, 16),          // Pre-allocate for typical file
		lines:         nil,                                  // Lazy init on first reference
		scopeStack:    make([]scopeStackEntry, 0, 8),        // Max nesting depth rarely >8
		refID:         1,
		handledNodes:  make(map[uintptr]bool, 64),
		nodeTypeCache: make(map[uintptr]string, 256), // Cache node types
		// Performance analysis tracking
		loopStack:           make([]loopStackEntry, 0, 4),     // Track loop nesting
		perfAnalysisResults: make([]PerfAnalysisResult, 0, 8), // Results per function
		// Side effect tracking (disabled by default, enable with EnableSideEffectTracking)
		sideEffectResults: make(map[string]*types.SideEffectInfo),
		enableSideEffects: false,
	}
}

// GetPooledExtractor gets an extractor from the pool and initializes it for use
// This reduces allocation overhead by reusing pre-allocated slices and maps
func GetPooledExtractor(parser *TreeSitterParser, content []byte, fileID types.FileID, ext, path string) *UnifiedExtractor {
	ue := extractorPool.Get().(*UnifiedExtractor)
	ue.content = content
	ue.fileID = fileID
	ue.ext = ext
	ue.path = path
	ue.parser = parser
	ue.refID = 1
	return ue
}

// ReleaseExtractor returns an extractor to the pool after resetting its state
// The slices are cleared but retain their capacity for reuse
func ReleaseExtractor(ue *UnifiedExtractor) {
	if ue == nil {
		return
	}
	ue.Reset()
	extractorPool.Put(ue)
}

// Reset clears the extractor state while retaining allocated slice/map capacity
// This enables efficient reuse via sync.Pool
func (ue *UnifiedExtractor) Reset() {
	// Clear input references (allow GC of content)
	ue.content = nil
	ue.fileID = 0
	ue.ext = ""
	ue.path = ""
	ue.parser = nil

	// Clear slices but keep capacity
	ue.scopes = ue.scopes[:0]
	ue.references = ue.references[:0]
	ue.symbols = ue.symbols[:0]
	ue.blocks = ue.blocks[:0]
	ue.imports = ue.imports[:0]
	ue.scopeStack = ue.scopeStack[:0]
	ue.loopStack = ue.loopStack[:0]
	ue.perfAnalysisResults = ue.perfAnalysisResults[:0]
	ue.complexityStack = ue.complexityStack[:0]
	ue.awaitExpressions = ue.awaitExpressions[:0]
	ue.callsInCurrentFunc = ue.callsInCurrentFunc[:0]

	// Clear maps by deleting all keys (faster than reallocating for small maps)
	for k := range ue.declarations {
		delete(ue.declarations, k)
	}
	for k := range ue.complexity {
		delete(ue.complexity, k)
	}
	for k := range ue.handledNodes {
		delete(ue.handledNodes, k)
	}
	for k := range ue.nodeTypeCache {
		delete(ue.nodeTypeCache, k)
	}
	for k := range ue.sideEffectResults {
		delete(ue.sideEffectResults, k)
	}

	// Reset cached data
	ue.lines = nil

	// Reset state flags
	ue.refID = 1
	ue.inImportContext = false
	ue.currentLevel = 0
	ue.inTraitOrImplBody = false
	ue.inClassBody = false
	ue.currentFuncKey = nil
	ue.currentFuncAnalysis = nil
	ue.sideEffectTracker = nil
	ue.enableSideEffects = false
}

// EnableSideEffectTracking enables side effect analysis for this extractor.
// This must be called before Extract() to enable tracking.
// Side effect tracking adds ~10-15% overhead but provides purity analysis.
func (ue *UnifiedExtractor) EnableSideEffectTracking() {
	ue.enableSideEffects = true
	language := LanguageFromExtension(ue.ext)
	ue.sideEffectTracker = NewSideEffectTracker(language)
}

// GetSideEffectResults returns the side effect analysis results.
// Returns nil if side effect tracking was not enabled.
func (ue *UnifiedExtractor) GetSideEffectResults() map[string]*types.SideEffectInfo {
	if !ue.enableSideEffects {
		return nil
	}
	return ue.sideEffectResults
}

// getLines returns pre-split lines, lazily initializing on first call
// Uses manual line splitting to avoid strings.Split allocations
func (ue *UnifiedExtractor) getLines() []string {
	if ue.lines == nil {
		content := ue.content
		if len(content) == 0 {
			ue.lines = []string{}
			return ue.lines
		}

		// Count lines first to pre-allocate
		lineCount := 1
		for _, b := range content {
			if b == '\n' {
				lineCount++
			}
		}

		ue.lines = make([]string, 0, lineCount)
		start := 0
		for i, b := range content {
			if b == '\n' {
				ue.lines = append(ue.lines, string(content[start:i]))
				start = i + 1
			}
		}
		// Add final line if content doesn't end with newline
		if start < len(content) {
			ue.lines = append(ue.lines, string(content[start:]))
		}
	}
	return ue.lines
}

// Extract performs the unified single-pass extraction
// Returns without processing if tree is nil (which indicates a parsing error).
// This is an expected condition for unparseable files, not an error.
func (ue *UnifiedExtractor) Extract(tree *tree_sitter.Tree) {
	if tree == nil || tree.RootNode() == nil {
		// Silent return is intentional - nil tree indicates parsing failed upstream
		// (e.g., unsupported language, syntax errors). The caller handles these cases.
		return
	}

	root := tree.RootNode()

	// Add file-level scope
	fileScope := types.ScopeInfo{
		Type:      types.ScopeTypeFile,
		Name:      filepath.Base(ue.path),
		FullPath:  ue.path,
		StartLine: 0,
		EndLine:   int(root.EndPosition().Row) + 1,
		Level:     0,
		Language:  ue.parser.getLanguageFromExt(ue.ext),
	}
	ue.scopes = append(ue.scopes, fileScope)

	// Add folder-level scope
	dir := filepath.Dir(ue.path)
	if dir != "." && dir != "/" {
		folderScope := types.ScopeInfo{
			Type:      types.ScopeTypeFolder,
			Name:      filepath.Base(dir),
			FullPath:  dir,
			StartLine: 0,
			EndLine:   0,
			Level:     -1,
			Language:  "",
		}
		ue.scopes = append([]types.ScopeInfo{folderScope}, ue.scopes...)
	}

	// Single-pass traversal
	ue.currentLevel = 1
	ue.visitNode(root)
}

// GetResults returns all extracted data (legacy signature for backward compatibility)
func (ue *UnifiedExtractor) GetResults() ([]types.ScopeInfo, []types.Reference, map[string]DeclarationInfo, map[PositionKey]int) {
	return ue.scopes, ue.references, ue.declarations, ue.complexity
}

// GetAllResults returns all extracted data including symbols, blocks, and imports
func (ue *UnifiedExtractor) GetAllResults() ([]types.Symbol, []types.BlockBoundary, []types.Import, []types.ScopeInfo, []types.Reference, map[string]DeclarationInfo, map[PositionKey]int) {
	return ue.symbols, ue.blocks, ue.imports, ue.scopes, ue.references, ue.declarations, ue.complexity
}

// visitNode is the core single-pass visitor that extracts all data types
func (ue *UnifiedExtractor) visitNode(node *tree_sitter.Node) {
	if node == nil {
		return
	}

	// Batch CGO: Get node type once and cache it
	nodeType := ue.getNodeType(node)

	// === COMPLEXITY TRACKING (integrated into single pass) ===
	// Track function entry for complexity calculation
	var funcKey *PositionKey
	var sideEffectFuncStarted bool
	if ue.isFunctionNode(nodeType) {
		startPoint := node.StartPosition()
		endPoint := node.EndPosition()
		key := PositionKey{Line: int(startPoint.Row) + 1, Column: int(startPoint.Column) + 1}
		funcKey = &key
		// Push new complexity counter (base complexity = 1)
		ue.complexityStack = append(ue.complexityStack, 1)

		// === SIDE EFFECT TRACKING: Function entry ===
		if ue.enableSideEffects && ue.sideEffectTracker != nil {
			funcName := ue.extractFunctionName(node, nodeType)
			ue.sideEffectTracker.BeginFunction(funcName, ue.path, int(startPoint.Row)+1, int(endPoint.Row)+1)
			ue.extractFunctionParameters(node, nodeType)
			sideEffectFuncStarted = true
		}
	}

	// Count decision points for current function's complexity
	if len(ue.complexityStack) > 0 {
		ue.countComplexityPoint(node, nodeType)
	}

	// === SYMBOL/BLOCK/IMPORT EXTRACTION (from extractBasicSymbolsStringRef) ===
	// This replaces the separate query-based pass with visitor-based extraction
	ue.processSymbolNode(node, nodeType)

	// === SCOPE EXTRACTION (from extractNestedScopes) ===
	scopeEntry := ue.processScopeNode(node, nodeType)
	if scopeEntry != nil {
		ue.scopeStack = append(ue.scopeStack, *scopeEntry)
		ue.currentLevel++
	}

	// === DECLARATION EXTRACTION ===
	ue.processDeclarationNode(node, nodeType)

	// === REFERENCE EXTRACTION (language-specific) ===
	ue.processReferenceNode(node, nodeType)

	// === TYPE RELATIONSHIP EXTRACTION (implements, extends, embeds) ===
	ue.processTypeRelationships(node, nodeType)

	// === PERFORMANCE ANALYSIS TRACKING ===
	ue.processPerformanceTracking(node, nodeType)

	// === SIDE EFFECT TRACKING (if enabled) ===
	if ue.enableSideEffects && ue.sideEffectTracker != nil && ue.sideEffectTracker.IsInFunction() {
		ue.processSideEffectNode(node, nodeType)
	}

	// Track if we're entering an import statement context
	wasInImportContext := ue.inImportContext
	if nodeType == "import_statement" {
		ue.inImportContext = true
	}

	// Track if we're entering a trait or impl body (Rust)
	wasInTraitOrImpl := ue.inTraitOrImplBody
	if nodeType == "trait_item" || nodeType == "impl_item" {
		ue.inTraitOrImplBody = true
	}

	// Track if we're entering a class body (Python, JS, etc.)
	wasInClassBody := ue.inClassBody
	if nodeType == "class_definition" || nodeType == "class_declaration" || nodeType == "class_body" {
		ue.inClassBody = true
	}

	// Track loop entry for performance analysis
	var loopEntry *loopStackEntry
	if ue.isLoopNode(nodeType) {
		startLine := int(node.StartPosition().Row) + 1
		endLine := int(node.EndPosition().Row) + 1
		loopEntry = &loopStackEntry{
			nodeType:  nodeType,
			startLine: startLine,
			endLine:   endLine,
			depth:     len(ue.loopStack) + 1,
		}
		ue.loopStack = append(ue.loopStack, *loopEntry)
	}

	// === RECURSE INTO CHILDREN ===
	// Batch CGO: Get child count once
	childCount := node.ChildCount()
	for i := uint(0); i < childCount; i++ {
		child := node.Child(i)
		ue.visitNode(child)
	}

	// Reset import context after visiting import statement children
	if nodeType == "import_statement" {
		ue.inImportContext = wasInImportContext
	}

	// Reset trait/impl context
	if nodeType == "trait_item" || nodeType == "impl_item" {
		ue.inTraitOrImplBody = wasInTraitOrImpl
	}

	// Reset class body context
	if nodeType == "class_definition" || nodeType == "class_declaration" || nodeType == "class_body" {
		ue.inClassBody = wasInClassBody
	}

	// Pop loop stack if we pushed one
	if loopEntry != nil {
		ue.loopStack = ue.loopStack[:len(ue.loopStack)-1]
	}

	// Pop scope if we pushed one
	if scopeEntry != nil {
		ue.scopeStack = ue.scopeStack[:len(ue.scopeStack)-1]
		ue.currentLevel--
	}

	// === FINALIZE COMPLEXITY for function exit ===
	if funcKey != nil && len(ue.complexityStack) > 0 {
		// Pop and store the complexity for this function
		complexity := ue.complexityStack[len(ue.complexityStack)-1]
		ue.complexityStack = ue.complexityStack[:len(ue.complexityStack)-1]
		ue.complexity[*funcKey] = complexity
	}

	// === FINALIZE SIDE EFFECT TRACKING for function exit ===
	if sideEffectFuncStarted && ue.enableSideEffects && ue.sideEffectTracker != nil {
		info := ue.sideEffectTracker.EndFunction()
		if info != nil {
			key := fmt.Sprintf("%s:%d", ue.path, funcKey.Line)
			ue.sideEffectResults[key] = info
		}
	}
}

// nodeTypeCacheMaxSize is the maximum number of entries in the node type cache.
// This prevents memory accumulation for very large ASTs while maintaining performance
// benefits for typical files. 10k entries covers most files while bounded at ~800KB memory.
const nodeTypeCacheMaxSize = 10000

// getNodeType returns cached node type to reduce CGO calls.
// The cache is bounded to prevent memory accumulation for very large ASTs.
func (ue *UnifiedExtractor) getNodeType(node *tree_sitter.Node) string {
	id := node.Id()
	if nodeType, exists := ue.nodeTypeCache[id]; exists {
		return nodeType
	}
	nodeType := node.Kind()

	// Bound cache size to prevent memory accumulation for very large ASTs
	// When limit is reached, skip caching but still return the type
	// This is rare (>10k unique nodes) and the cost is just extra CGO calls
	if len(ue.nodeTypeCache) < nodeTypeCacheMaxSize {
		ue.nodeTypeCache[id] = nodeType
	}
	return nodeType
}

// isFunctionNode returns true if the node type represents a function definition
func (ue *UnifiedExtractor) isFunctionNode(nodeType string) bool {
	switch nodeType {
	case "function_declaration", "function_definition", "function_item",
		"method_definition", "method_declaration", "arrow_function",
		"function_expression", "generator_function", "generator_function_declaration",
		"func_literal", "constructor_definition":
		return true
	}
	return false
}

// countComplexityPoint increments complexity counter for decision points
// This is called during traversal instead of a separate walk
func (ue *UnifiedExtractor) countComplexityPoint(node *tree_sitter.Node, nodeType string) {
	stackLen := len(ue.complexityStack)
	if stackLen == 0 {
		return
	}

	switch nodeType {
	// If statements (all languages)
	case "if_statement", "if_expression":
		ue.complexityStack[stackLen-1]++

	// For/while loops (all languages)
	case "for_statement", "for_range_statement", "for_in_statement",
		"while_statement", "do_while_statement":
		ue.complexityStack[stackLen-1]++

	// Case clauses (all add a path)
	case "case_clause", "case_statement", "expression_case", "type_case":
		ue.complexityStack[stackLen-1]++

	// Ternary/conditional expressions
	case "conditional_expression", "ternary_expression":
		ue.complexityStack[stackLen-1]++

	// Exception handling
	case "catch_clause", "except_clause":
		ue.complexityStack[stackLen-1]++

	// Logical operators in binary expressions
	case "binary_expression":
		if node.ChildCount() >= 3 {
			operatorNode := node.Child(1)
			if operatorNode != nil {
				operator := ue.getNodeType(operatorNode)
				if operator == "&&" || operator == "||" || operator == "and" || operator == "or" {
					ue.complexityStack[stackLen-1]++
				}
			}
		}
	}
}

// processScopeNode extracts scope information from a node
// Returns a scopeStackEntry if this node creates a new scope, nil otherwise
func (ue *UnifiedExtractor) processScopeNode(node *tree_sitter.Node, nodeType string) *scopeStackEntry {
	var scopeType types.ScopeType
	var name string

	switch nodeType {
	case "class_declaration", "class_definition":
		scopeType = types.ScopeTypeClass
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
		}

	case "type_declaration":
		// Go struct/type declarations
		scopeType = types.ScopeTypeClass
		name = ue.extractGoTypeName(node)

	case "function_declaration", "function_definition":
		scopeType = types.ScopeTypeFunction
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
		}
		// NOTE: Complexity is computed in extractBasicSymbolsStringRef (query-based)
		// to avoid duplicate tree walks. Do NOT compute here.

	case "method_definition", "method_declaration":
		scopeType = types.ScopeTypeMethod
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
		}
		// NOTE: Complexity is computed in extractBasicSymbolsStringRef (query-based)
		// to avoid duplicate tree walks. Do NOT compute here.

	case "interface_declaration":
		scopeType = types.ScopeTypeInterface
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
		}

	case "block_statement", "compound_statement":
		scopeType = types.ScopeTypeBlock
		name = "block"

	default:
		return nil
	}

	if name == "" && scopeType != types.ScopeTypeBlock {
		return nil
	}

	// Create scope entry
	startLine := int(node.StartPosition().Row) + 1
	endLine := int(node.EndPosition().Row) + 1

	// Detect attributes for this scope
	attributes := ue.parser.detectContextAttributes(node, ue.content)

	scope := types.ScopeInfo{
		Type:       scopeType,
		Name:       name,
		FullPath:   ue.buildFullQualifiedName(name),
		StartLine:  startLine,
		EndLine:    endLine,
		Level:      ue.currentLevel,
		Language:   "",
		Attributes: attributes,
	}
	ue.scopes = append(ue.scopes, scope)

	return &scopeStackEntry{
		scopeType: scopeType,
		name:      name,
		node:      node,
		startLine: startLine,
		endLine:   endLine,
	}
}

// extractGoTypeName extracts the name from a Go type_declaration
func (ue *UnifiedExtractor) extractGoTypeName(node *tree_sitter.Node) string {
	childCount := node.ChildCount()
	for j := uint(0); j < childCount; j++ {
		typeSpecChild := node.Child(j)
		if ue.getNodeType(typeSpecChild) == "type_spec" {
			specChildCount := typeSpecChild.ChildCount()
			for k := uint(0); k < specChildCount; k++ {
				typeIdChild := typeSpecChild.Child(k)
				if ue.getNodeType(typeIdChild) == "type_identifier" {
					return string(ue.content[typeIdChild.StartByte():typeIdChild.EndByte()])
				}
			}
		}
	}
	return ""
}

// buildFullQualifiedName builds the full qualified name from current scope stack
func (ue *UnifiedExtractor) buildFullQualifiedName(name string) string {
	if len(ue.scopeStack) == 0 {
		return name
	}

	var parts []string
	for _, entry := range ue.scopeStack {
		if entry.name != "" && entry.name != "block" {
			parts = append(parts, entry.name)
		}
	}
	parts = append(parts, name)
	return strings.Join(parts, ".")
}

// processSymbolNode extracts symbol, block, and import information from AST nodes
// This consolidates the query-based extraction into the visitor pass
func (ue *UnifiedExtractor) processSymbolNode(node *tree_sitter.Node, nodeType string) {
	switch nodeType {
	// === FUNCTIONS ===
	case "function_declaration", "generator_function_declaration", "func_literal":
		ue.extractFunction(node, nodeType)

	case "function_definition":
		// Python function_definition: if inside class, it's a method; otherwise function
		if ue.inClassBody {
			ue.extractPythonMethod(node)
		} else {
			ue.extractFunction(node, nodeType)
		}

	case "function_item":
		// Rust function_item: if inside trait/impl, it's a method; otherwise function
		if ue.inTraitOrImplBody {
			ue.extractRustMethod(node)
		} else {
			ue.extractFunction(node, nodeType)
		}

	case "method_definition", "method_declaration":
		ue.extractMethod(node, nodeType)

	case "arrow_function", "function_expression", "generator_function":
		// Only extract if parent is variable_declarator (named arrow function)
		// Anonymous functions are captured by their container
		parent := node.Parent()
		if parent != nil && ue.getNodeType(parent) == "variable_declarator" {
			// Extract BOTH as function AND mark that we also want a variable
			// This dual nature is required by the JavaScript classification tests
			ue.extractArrowFunctionDualNature(node, parent)
		}

	// === CLASSES ===
	case "class_declaration", "class_definition", "class_specifier":
		ue.extractClass(node, nodeType)

	// === INTERFACES ===
	case "interface_declaration":
		ue.extractInterface(node)

	// === TYPES ===
	case "type_declaration":
		ue.extractTypeDeclaration(node)

	case "type_alias_declaration":
		ue.extractTypeAlias(node)

	// === STRUCTS (Go, Rust, C#, etc.) ===
	case "struct_item", "struct_expression", "struct_declaration":
		ue.extractStruct(node, nodeType)

	// === ENUMS ===
	case "enum_declaration", "enum_item":
		ue.extractEnum(node, nodeType)

	// === TRAITS/IMPLS (Rust) ===
	case "trait_item":
		ue.extractTrait(node)

	case "impl_item":
		ue.extractImpl(node)

	// === MODULES/NAMESPACES ===
	case "module", "mod_item":
		ue.extractModule(node, nodeType)

	case "namespace_declaration":
		ue.extractNamespace(node)

	// === VARIABLES ===
	case "variable_declarator":
		// Only extract if not already handled as arrow function
		if !ue.isArrowFunctionDeclarator(node) {
			ue.extractVariable(node)
		}

	case "short_var_declaration", "var_declaration", "const_declaration":
		ue.extractGoVariable(node, nodeType)

	// === IMPORTS ===
	case "import_statement":
		// Could be JavaScript/TypeScript or Python import
		if ue.ext == ".py" {
			ue.extractPythonImport(node)
		} else {
			ue.extractJSImport(node, nodeType)
		}

	case "import_from_statement":
		// Python-specific import
		ue.extractPythonImport(node)

	case "import_spec", "import_declaration":
		ue.extractGoImport(node, nodeType)

	// === CONSTRUCTORS ===
	case "constructor_definition":
		ue.extractConstructor(node)

	// === PROPERTIES/FIELDS ===
	case "property_definition", "public_field_definition":
		ue.extractProperty(node, nodeType)

	case "field_declaration":
		ue.extractField(node)

	// === C# SPECIFIC ===
	case "record_declaration":
		ue.extractRecord(node)

	case "delegate_declaration":
		ue.extractDelegate(node)

	case "property_declaration":
		ue.extractPropertyDeclaration(node)

	case "event_declaration":
		ue.extractEvent(node)

	case "using_directive":
		ue.extractUsingDirective(node)

	// === KOTLIN SPECIFIC ===
	case "object_declaration":
		ue.extractKotlinObject(node)

	case "companion_object":
		ue.extractCompanionObject(node)

	// === ZIG SPECIFIC ===
	case "container_decl", "VarDecl":
		ue.extractZigDecl(node, nodeType)

	case "variable_declaration":
		// Zig: const Point = struct { ... } -> variable_declaration with struct_declaration child
		if ue.ext == ".zig" {
			ue.extractZigVariableDeclaration(node)
		}

		// Note: Python class_definition is handled by extractClass() above
	}
}

// isArrowFunctionDeclarator checks if a variable_declarator contains an arrow function
func (ue *UnifiedExtractor) isArrowFunctionDeclarator(node *tree_sitter.Node) bool {
	if valueNode := node.ChildByFieldName("value"); valueNode != nil {
		valueType := ue.getNodeType(valueNode)
		return valueType == "arrow_function" || valueType == "function_expression" || valueType == "generator_function"
	}
	return false
}

// extractFunction extracts a function declaration as a symbol
func (ue *UnifiedExtractor) extractFunction(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	// C++ function_definition has name in declarator->declarator
	if name == "" {
		if declaratorNode := node.ChildByFieldName("declarator"); declaratorNode != nil {
			// function_declarator has the actual identifier
			if innerDecl := declaratorNode.ChildByFieldName("declarator"); innerDecl != nil {
				name = string(ue.content[innerDecl.StartByte():innerDecl.EndByte()])
			}
		}
	}

	if name == "" && nodeType != "func_literal" && nodeType != "arrow_function" {
		return // Skip anonymous functions that aren't arrow functions
	}

	// Note: Complexity is computed during visitNode traversal, not here
	// This eliminates the duplicate tree walk that was happening before

	// Detect attributes
	attributes := ue.parser.detectContextAttributes(node, ue.content)

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeFunction,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:       name,
		Type:       types.SymbolTypeFunction,
		Line:       int(startPoint.Row) + 1,
		Column:     int(startPoint.Column) + 1,
		EndLine:    int(endPoint.Row) + 1,
		EndColumn:  int(endPoint.Column) + 1,
		Attributes: attributes,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractMethod extracts a method definition as a symbol
func (ue *UnifiedExtractor) extractMethod(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	// Note: Complexity is computed during visitNode traversal, not here

	// Detect attributes
	attributes := ue.parser.detectContextAttributes(node, ue.content)

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeMethod,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:       name,
		Type:       types.SymbolTypeMethod,
		Line:       int(startPoint.Row) + 1,
		Column:     int(startPoint.Column) + 1,
		EndLine:    int(endPoint.Row) + 1,
		EndColumn:  int(endPoint.Column) + 1,
		Attributes: attributes,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractPythonMethod extracts a Python function_definition that's inside a class as a method
func (ue *UnifiedExtractor) extractPythonMethod(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	// Note: Complexity is computed during visitNode traversal, not here

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeMethod,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeMethod,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractRustMethod extracts a Rust function_item that's inside a trait/impl as a method
func (ue *UnifiedExtractor) extractRustMethod(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	// Note: Complexity is computed during visitNode traversal, not here

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeMethod,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeMethod,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractArrowFunctionDualNature extracts an arrow function from a variable declarator
// It creates BOTH a function symbol AND a variable symbol, matching the dual-nature
// behavior expected by JavaScript code where const arrowFunc = () => {} is both
// a function (callable) and a variable (referenceable).
func (ue *UnifiedExtractor) extractArrowFunctionDualNature(funcNode, declaratorNode *tree_sitter.Node) {
	startPoint := declaratorNode.StartPosition()
	endPoint := funcNode.EndPosition()

	var name string
	if nameNode := declaratorNode.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	// Note: Complexity is computed during visitNode traversal when visiting funcNode

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeFunction,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	// Create the function symbol
	funcSymbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeFunction,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, funcSymbol)

	// Also create a variable symbol for the dual-nature
	// This matches the query-based behavior where both captures would match
	varSymbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeVariable,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, varSymbol)
}

// extractClass extracts a class declaration as a symbol
func (ue *UnifiedExtractor) extractClass(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeClass,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractInterface extracts an interface declaration as a symbol
func (ue *UnifiedExtractor) extractInterface(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeInterface,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeInterface,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractTypeDeclaration extracts a Go type declaration
func (ue *UnifiedExtractor) extractTypeDeclaration(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// Go type declarations have type_spec children
	var name string
	var symbolType types.SymbolType = types.SymbolTypeType
	var blockType = types.BlockTypeOther

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) == "type_spec" {
			// Get name from type_spec
			for j := uint(0); j < child.ChildCount(); j++ {
				specChild := child.Child(j)
				specChildType := ue.getNodeType(specChild)
				if specChildType == "type_identifier" {
					name = string(ue.content[specChild.StartByte():specChild.EndByte()])
				} else if specChildType == "struct_type" {
					symbolType = types.SymbolTypeStruct
					blockType = types.BlockTypeStruct
				} else if specChildType == "interface_type" {
					symbolType = types.SymbolTypeInterface
					blockType = types.BlockTypeInterface
				}
			}
			break
		}
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  blockType,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      symbolType,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractTypeAlias extracts a TypeScript type alias
func (ue *UnifiedExtractor) extractTypeAlias(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeOther,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeType,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractStruct extracts a struct declaration (Rust)
func (ue *UnifiedExtractor) extractStruct(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeStruct,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeStruct,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractEnum extracts an enum declaration
func (ue *UnifiedExtractor) extractEnum(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeEnum,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeEnum,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractTrait extracts a Rust trait as its own distinct type
func (ue *UnifiedExtractor) extractTrait(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeTrait,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeTrait,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractImpl extracts a Rust impl block
func (ue *UnifiedExtractor) extractImpl(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// For impl blocks, construct name from the type being implemented
	var name string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)
		if childType == "type_identifier" || childType == "generic_type" {
			name = string(ue.content[child.StartByte():child.EndByte()])
			break
		}
	}

	if name == "" {
		name = "impl"
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeImpl,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeImpl,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractModule extracts a module declaration
func (ue *UnifiedExtractor) extractModule(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeModule,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeModule,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractNamespace extracts a namespace declaration
func (ue *UnifiedExtractor) extractNamespace(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeNamespace,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeNamespace,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractVariable extracts a variable declaration
func (ue *UnifiedExtractor) extractVariable(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeVariable,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractGoVariable extracts Go variable/const declarations
func (ue *UnifiedExtractor) extractGoVariable(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// Go var/const declarations have var_spec or const_spec children
	specType := "var_spec"
	if nodeType == "const_declaration" {
		specType = "const_spec"
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if ue.getNodeType(child) == specType {
			// Get identifiers from spec
			for j := uint(0); j < child.ChildCount(); j++ {
				specChild := child.Child(j)
				if ue.getNodeType(specChild) == "identifier" {
					name := string(ue.content[specChild.StartByte():specChild.EndByte()])

					symType := types.SymbolTypeVariable
					if nodeType == "const_declaration" {
						symType = types.SymbolTypeConstant
					}

					symbol := types.Symbol{
						Name:      name,
						Type:      symType,
						Line:      int(startPoint.Row) + 1,
						Column:    int(startPoint.Column) + 1,
						EndLine:   int(endPoint.Row) + 1,
						EndColumn: int(endPoint.Column) + 1,
					}
					ue.symbols = append(ue.symbols, symbol)
				}
			}
		}
	}
}

// extractJSImport extracts JavaScript/TypeScript imports
func (ue *UnifiedExtractor) extractJSImport(node *tree_sitter.Node, nodeType string) {
	// Extract import source
	if sourceNode := node.ChildByFieldName("source"); sourceNode != nil {
		source := string(ue.content[sourceNode.StartByte():sourceNode.EndByte()])
		// Remove quotes
		source = strings.Trim(source, `"'`)

		imp := types.Import{
			Path: source,
		}
		ue.imports = append(ue.imports, imp)
	}
}

// extractPythonImport extracts Python import statements
func (ue *UnifiedExtractor) extractPythonImport(node *tree_sitter.Node) {
	// Extract the full import text as the path
	importText := string(ue.content[node.StartByte():node.EndByte()])

	imp := types.Import{
		Path: importText,
	}
	ue.imports = append(ue.imports, imp)
}

// extractGoImport extracts Go imports
func (ue *UnifiedExtractor) extractGoImport(node *tree_sitter.Node, nodeType string) {
	if nodeType == "import_spec" {
		if pathNode := node.ChildByFieldName("path"); pathNode != nil {
			path := string(ue.content[pathNode.StartByte():pathNode.EndByte()])
			// Remove quotes
			path = strings.Trim(path, `"`)

			imp := types.Import{
				Path: path,
			}
			ue.imports = append(ue.imports, imp)
		}
	}
}

// extractConstructor extracts a constructor
func (ue *UnifiedExtractor) extractConstructor(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// Note: Complexity is computed during visitNode traversal, not here

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeConstructor,
		Name:  "constructor",
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      "constructor",
		Type:      types.SymbolTypeConstructor,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractProperty extracts a class property
func (ue *UnifiedExtractor) extractProperty(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeProperty,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractField extracts a struct field
func (ue *UnifiedExtractor) extractField(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	// Try field_identifier first (Go), then name field
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)
		if childType == "field_identifier" || childType == "identifier" {
			name = string(ue.content[child.StartByte():child.EndByte()])
			break
		}
	}

	if name == "" {
		return
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeField,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractRecord extracts a C# record declaration
func (ue *UnifiedExtractor) extractRecord(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeRecord,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractDelegate extracts a C# delegate declaration
func (ue *UnifiedExtractor) extractDelegate(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeDelegate,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractPropertyDeclaration extracts a C# property declaration
func (ue *UnifiedExtractor) extractPropertyDeclaration(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeProperty,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractEvent extracts a C# event declaration
func (ue *UnifiedExtractor) extractEvent(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeEvent,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractUsingDirective extracts C# using directives as imports
func (ue *UnifiedExtractor) extractUsingDirective(node *tree_sitter.Node) {
	// Extract the namespace being imported
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)
		if childType == "qualified_name" || childType == "identifier" {
			path := string(ue.content[child.StartByte():child.EndByte()])
			imp := types.Import{
				Path: path,
			}
			ue.imports = append(ue.imports, imp)
			return
		}
	}
}

// extractKotlinObject extracts a Kotlin object declaration
func (ue *UnifiedExtractor) extractKotlinObject(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeObject,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractCompanionObject extracts a Kotlin companion object
func (ue *UnifiedExtractor) extractCompanionObject(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  "companion",
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      "companion",
		Type:      types.SymbolTypeCompanion,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractZigDecl extracts Zig struct/type declarations
func (ue *UnifiedExtractor) extractZigDecl(node *tree_sitter.Node, nodeType string) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	// Determine if this is a struct or type based on content
	symbolType := types.SymbolTypeType
	blockType := types.BlockTypeOther

	// Check for struct keyword in the declaration
	nodeText := string(ue.content[node.StartByte():node.EndByte()])
	if strings.Contains(nodeText, "struct") {
		symbolType = types.SymbolTypeStruct
		blockType = types.BlockTypeStruct
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  blockType,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      symbolType,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractZigVariableDeclaration extracts Zig variable declarations that contain structs/unions
// In Zig, structs are declared as: const Point = struct { ... }
// This is a variable_declaration with identifier and struct_declaration child
func (ue *UnifiedExtractor) extractZigVariableDeclaration(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	// Look for the identifier (name) child
	var name string
	var hasStruct bool
	var hasUnion bool

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childType := ue.getNodeType(child)

		if childType == "identifier" {
			name = string(ue.content[child.StartByte():child.EndByte()])
		} else if childType == "struct_declaration" {
			hasStruct = true
		} else if childType == "union_declaration" {
			hasUnion = true
		}
	}

	if name == "" {
		return
	}

	// Determine symbol type based on content
	var symbolType types.SymbolType
	var blockType types.BlockType

	if hasStruct {
		symbolType = types.SymbolTypeStruct
		blockType = types.BlockTypeStruct
	} else if hasUnion {
		// Zig unions are similar to enums with associated data
		symbolType = types.SymbolTypeType
		blockType = types.BlockTypeOther
	} else {
		// Regular variable - skip as it's handled elsewhere
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  blockType,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      symbolType,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// extractPythonClass extracts a Python class definition
func (ue *UnifiedExtractor) extractPythonClass(node *tree_sitter.Node) {
	startPoint := node.StartPosition()
	endPoint := node.EndPosition()

	var name string
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	if name == "" {
		return
	}

	block := types.BlockBoundary{
		Start: int(startPoint.Row),
		End:   int(endPoint.Row),
		Type:  types.BlockTypeClass,
		Name:  name,
	}
	ue.blocks = append(ue.blocks, block)

	symbol := types.Symbol{
		Name:      name,
		Type:      types.SymbolTypeClass,
		Line:      int(startPoint.Row) + 1,
		Column:    int(startPoint.Column) + 1,
		EndLine:   int(endPoint.Row) + 1,
		EndColumn: int(endPoint.Column) + 1,
	}
	ue.symbols = append(ue.symbols, symbol)
}

// processDeclarationNode extracts declaration metadata
func (ue *UnifiedExtractor) processDeclarationNode(node *tree_sitter.Node, nodeType string) {
	if !isDeclarationNode(nodeType) {
		return
	}

	// Extract signature and doc comment for this declaration
	signature := ue.extractSignatureFromNode(node, nodeType)
	docComment := ue.extractDocCommentBeforeNode(node)

	// Store by position for O(1) lookup
	start := node.StartPosition()
	key := fmt.Sprintf("%d:%d", start.Row, start.Column)
	ue.declarations[key] = DeclarationInfo{
		Signature:  signature,
		DocComment: docComment,
	}
}

// extractSignatureFromNode extracts the signature from a declaration node
func (ue *UnifiedExtractor) extractSignatureFromNode(node *tree_sitter.Node, nodeType string) string {
	// For function declarations, extract the signature (name + params + return type)
	switch nodeType {
	case "function_declaration", "method_declaration":
		// Get the first line up to the body
		startByte := node.StartByte()
		endByte := node.EndByte()

		// Find the body node to exclude it
		if bodyNode := node.ChildByFieldName("body"); bodyNode != nil {
			endByte = bodyNode.StartByte()
		}

		if startByte < uint(len(ue.content)) && endByte <= uint(len(ue.content)) {
			sig := strings.TrimSpace(string(ue.content[startByte:endByte]))
			// Remove trailing { if present
			sig = strings.TrimSuffix(sig, "{")
			sig = strings.TrimSpace(sig)
			return sig
		}
	}
	return ""
}

// extractDocCommentBeforeNode extracts documentation comment before a node
// OPTIMIZATION: Use PrevSibling() instead of scanning all children - reduces CGO calls from O(n) to O(1)
func (ue *UnifiedExtractor) extractDocCommentBeforeNode(node *tree_sitter.Node) string {
	// Use PrevSibling() which is O(1) instead of searching through all children
	prevSibling := node.PrevSibling()
	if prevSibling == nil {
		return ""
	}

	prevType := ue.getNodeType(prevSibling)

	if prevType == "comment" || prevType == "line_comment" || prevType == "block_comment" {
		return string(ue.content[prevSibling.StartByte():prevSibling.EndByte()])
	}

	return ""
}

// processReferenceNode extracts references from a node (language-specific)
func (ue *UnifiedExtractor) processReferenceNode(node *tree_sitter.Node, nodeType string) {
	switch ue.ext {
	case ".go":
		ue.processGoReference(node, nodeType)
	case ".js", ".jsx", ".ts", ".tsx":
		ue.processJSReference(node, nodeType)
	case ".py":
		ue.processPythonReference(node, nodeType)
	}
}

// processGoReference extracts Go language references
func (ue *UnifiedExtractor) processGoReference(node *tree_sitter.Node, nodeType string) {
	switch nodeType {
	case "call_expression":
		if functionNode := node.ChildByFieldName("function"); functionNode != nil {
			ref := ue.createReference(functionNode, types.RefTypeCall, types.RefStrengthTight)
			ue.references = append(ue.references, ref)
		}

	case "selector_expression":
		if fieldNode := node.ChildByFieldName("field"); fieldNode != nil {
			// Field node is always field_identifier
			ref := ue.createReferenceWithType(fieldNode, "field_identifier", types.RefTypeUsage, types.RefStrengthLoose)
			ue.references = append(ue.references, ref)
		}

	case "type_identifier", "field_identifier":
		// Node type is already known - use optimized version
		ref := ue.createReferenceWithType(node, nodeType, types.RefTypeUsage, types.RefStrengthLoose)
		ue.references = append(ue.references, ref)
	}
}

// processJSReference extracts JavaScript/TypeScript references with context awareness
func (ue *UnifiedExtractor) processJSReference(node *tree_sitter.Node, nodeType string) {
	nodeID := node.Id()

	switch nodeType {
	case "call_expression":
		if functionNode := node.ChildByFieldName("function"); functionNode != nil {
			ue.handledNodes[functionNode.Id()] = true
			ref := ue.createReference(functionNode, types.RefTypeCall, types.RefStrengthTight)
			ue.references = append(ue.references, ref)
		}

	case "member_expression":
		if propertyNode := node.ChildByFieldName("property"); propertyNode != nil {
			ue.handledNodes[propertyNode.Id()] = true
			// Property node is always property_identifier
			ref := ue.createReferenceWithType(propertyNode, "property_identifier", types.RefTypeUsage, types.RefStrengthLoose)
			ue.references = append(ue.references, ref)
		}

	case "import_statement":
		// inImportContext is set/reset in visitNode to properly cover all children
		if sourceNode := node.ChildByFieldName("source"); sourceNode != nil {
			ref := ue.createReference(sourceNode, types.RefTypeImport, types.RefStrengthTight)
			ue.references = append(ue.references, ref)
		}

	case "identifier":
		// Skip if already handled or in import context
		if ue.handledNodes[nodeID] || ue.inImportContext {
			return
		}
		// Node type is already known - use optimized version
		ref := ue.createReferenceWithType(node, nodeType, types.RefTypeUsage, types.RefStrengthLoose)
		ue.references = append(ue.references, ref)
	}
}

// processPythonReference extracts Python language references
func (ue *UnifiedExtractor) processPythonReference(node *tree_sitter.Node, nodeType string) {
	switch nodeType {
	case "call":
		if functionNode := node.ChildByFieldName("function"); functionNode != nil {
			ref := ue.createReference(functionNode, types.RefTypeCall, types.RefStrengthTight)
			ue.references = append(ue.references, ref)
		}

	case "attribute":
		if attrNode := node.ChildByFieldName("attribute"); attrNode != nil {
			// Attribute node is always identifier
			ref := ue.createReferenceWithType(attrNode, "identifier", types.RefTypeUsage, types.RefStrengthLoose)
			ue.references = append(ue.references, ref)
		}

	case "identifier":
		// Node type is already known - use optimized version
		ref := ue.createReferenceWithType(node, nodeType, types.RefTypeUsage, types.RefStrengthLoose)
		ue.references = append(ue.references, ref)
	}
}

// createReference creates a Reference struct from a tree-sitter node
func (ue *UnifiedExtractor) createReference(node *tree_sitter.Node, refType types.ReferenceType, strength types.RefStrength) types.Reference {
	// Get node type once and pass it through
	nodeType := ue.getNodeType(node)
	return ue.createReferenceWithType(node, nodeType, refType, strength)
}

// createReferenceWithType creates a Reference struct using a pre-fetched node type
// This avoids redundant CGO calls when the node type is already known
func (ue *UnifiedExtractor) createReferenceWithType(node *tree_sitter.Node, nodeType string, refType types.ReferenceType, strength types.RefStrength) types.Reference {
	startPoint := node.StartPosition()

	// Extract context lines around the reference (lazy init lines)
	lines := ue.getLines()
	contextStart := max(0, int(startPoint.Row)-1)
	contextEnd := min(len(lines), int(startPoint.Row)+2)
	context := lines[contextStart:contextEnd]

	// Extract the actual referenced symbol name from the AST node
	// Use the optimized version with pre-fetched node type
	referencedName := ue.parser.extractReferencedSymbolNameWithType(node, ue.content, nodeType)

	ref := types.Reference{
		ID:             ue.refID,
		SourceSymbol:   0,
		TargetSymbol:   0,
		FileID:         0,
		Line:           int(startPoint.Row) + 1,
		Column:         int(startPoint.Column) + 1,
		Type:           refType,
		Context:        context,
		ScopeContext:   []types.ScopeInfo{},
		Strength:       strength,
		ReferencedName: referencedName,
	}
	ue.refID++
	return ref
}

// LookupDeclaration retrieves signature and doc comment for a symbol at given position
func (ue *UnifiedExtractor) LookupDeclaration(line, column int) (string, string) {
	key := fmt.Sprintf("%d:%d", line-1, column-1) // Convert 1-based to 0-based
	if info, exists := ue.declarations[key]; exists {
		return info.Signature, info.DocComment
	}
	return "", ""
}

// Lookup implements DeclarationLookup interface for use with buildEnhancedSymbols
// This is an alias for LookupDeclaration to match the interface
func (ue *UnifiedExtractor) Lookup(line, column int) (string, string) {
	return ue.LookupDeclaration(line, column)
}


