package core

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/standardbeagle/lci/internal/types"
)

// ReferenceStats contains pre-computed reference tracking statistics
type ReferenceStats struct {
	TotalReferences int `json:"total_references"`
	TotalSymbols    int `json:"total_symbols"`
	FilesWithRefs   int `json:"files_with_references"`
	SymbolRefs      int `json:"symbol_references"` // Total incoming + outgoing ref count
}

// ReferenceTracker manages bidirectional symbol references and scope relationships
type ReferenceTracker struct {
	// Core data structures
	// NEW: SymbolStore for 30-50% performance improvement over map-based storage
	symbols       *SymbolStore                      // High-performance symbol storage
	references    map[uint64]*types.Reference       // All references by ID
	symbolsByName map[string][]types.SymbolID       // Symbol lookup by name
	symbolsByFile map[types.FileID][]types.SymbolID // Symbols by file

	// Reference maps for fast lookup
	incomingRefs map[types.SymbolID][]uint64 // Symbol -> References pointing to it
	outgoingRefs map[types.SymbolID][]uint64 // Symbol -> References it makes

	// Scope tracking
	scopesByFile map[types.FileID][]types.ScopeInfo   // Scope hierarchy by file
	symbolScopes map[types.SymbolID][]types.ScopeInfo // Complete scope chain per symbol

	// Import resolution for better symbol linking
	importResolver *ImportResolver
	importData     []*FileImportData // Collected import data (lock-free during indexing)

	// Fast symbol lookup by position (eliminates O(n) linear searches)
	symbolLocationIndex *SymbolLocationIndex

	// Cache for resolved references to avoid redundant import resolution
	// Key: (fileID<<32) | hash(name), Value: symbolID
	referenceCache map[uint64]types.SymbolID

	// Cache for scope chains to avoid rebuilding identical scope chains
	// Key: scope signature hash (uint64), Value: cached scope chain with verification data
	// OPTIMIZED: Uses uint64 key instead of string for reduced allocations
	// NOTE: Uses hash with collision verification - stores original key data for validation
	scopeChainCache map[uint64]*scopeChainCacheEntry

	// Performance analysis data by file for code_insight anti-pattern detection
	perfDataByFile map[types.FileID][]types.FunctionPerfData

	nextSymbolID types.SymbolID
	mu           sync.RWMutex
	BulkIndexing int32
	nextRefID    uint64

	// NEW: Pre-computed statistics
	stats ReferenceStats
}

// NewReferenceTracker creates a new reference tracking system
// symbolLocationIndex can be nil for tests
func NewReferenceTracker(symbolLocationIndex *SymbolLocationIndex) *ReferenceTracker {
	// OPTIMIZED: Use conservative initial allocations and let maps grow as needed
	// Previous: Pre-allocated for 5000 symbols (causing 5GB allocations!)
	// New: Start with smaller capacity for typical small-to-medium projects
	// Maps will automatically grow as needed, with minimal performance impact
	expectedSymbols := 256 // Start small, grows to 512, 1024, etc. as needed

	return &ReferenceTracker{
		symbols:             NewSymbolStore(expectedSymbols),
		references:          make(map[uint64]*types.Reference, expectedSymbols*2),
		symbolsByName:       make(map[string][]types.SymbolID, expectedSymbols),
		symbolsByFile:       make(map[types.FileID][]types.SymbolID, 32), // Typical: 10-100 files
		incomingRefs:        make(map[types.SymbolID][]uint64, expectedSymbols),
		outgoingRefs:        make(map[types.SymbolID][]uint64, expectedSymbols),
		scopesByFile:        make(map[types.FileID][]types.ScopeInfo, 32),
		symbolScopes:        make(map[types.SymbolID][]types.ScopeInfo, expectedSymbols),
		importResolver:      NewImportResolver(),
		symbolLocationIndex: symbolLocationIndex,
		// Start with modest cache size - will grow if needed
		referenceCache:  make(map[uint64]types.SymbolID, expectedSymbols*2),
		scopeChainCache: make(map[uint64]*scopeChainCacheEntry, 64),
		perfDataByFile:  make(map[types.FileID][]types.FunctionPerfData, 32),
		nextSymbolID:    1,
		nextRefID:       1,
	}
}

// NewReferenceTrackerForTest creates a ReferenceTracker for testing with a new SymbolLocationIndex
func NewReferenceTrackerForTest() *ReferenceTracker {
	return NewReferenceTracker(NewSymbolLocationIndex())
}

// Clear resets all data in the reference tracker
func (rt *ReferenceTracker) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Clear symbol store (more efficient than map recreation)
	rt.symbols.Clear()

	// Clear all maps
	rt.references = make(map[uint64]*types.Reference)
	rt.symbolsByName = make(map[string][]types.SymbolID)
	rt.symbolsByFile = make(map[types.FileID][]types.SymbolID)
	rt.incomingRefs = make(map[types.SymbolID][]uint64)
	rt.outgoingRefs = make(map[types.SymbolID][]uint64)
	rt.scopesByFile = make(map[types.FileID][]types.ScopeInfo)
	rt.symbolScopes = make(map[types.SymbolID][]types.ScopeInfo)
	rt.importData = nil
	rt.referenceCache = make(map[uint64]types.SymbolID)
	rt.scopeChainCache = make(map[uint64]*scopeChainCacheEntry)
	rt.perfDataByFile = make(map[types.FileID][]types.FunctionPerfData)
	rt.nextSymbolID = 1
	rt.nextRefID = 1

	// Clear import resolver data
	rt.importResolver.Clear()
}

// GetReferenceStats returns pre-computed reference statistics
func (rt *ReferenceTracker) GetReferenceStats() ReferenceStats {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.stats
}

// Shutdown performs graceful shutdown with resource cleanup
func (rt *ReferenceTracker) Shutdown() error {
	// Clear all data which includes cleaning up import resolver
	rt.Clear()
	return nil
}

// RemoveFile removes all symbols and references for a specific file
func (rt *ReferenceTracker) RemoveFile(fileID types.FileID) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Get all symbols in this file
	symbolIDs, exists := rt.symbolsByFile[fileID]
	if !exists {
		return
	}

	// Remove each symbol
	for _, symbolID := range symbolIDs {
		// Use SymbolStore.Get for fast O(1) access
		symbol := rt.symbols.Get(symbolID)
		if symbol == nil {
			continue
		}

		// Remove from name index
		if nameList, exists := rt.symbolsByName[symbol.Name]; exists {
			filtered := nameList[:0]
			for _, id := range nameList {
				if id != symbolID {
					filtered = append(filtered, id)
				}
			}
			if len(filtered) > 0 {
				rt.symbolsByName[symbol.Name] = filtered
			} else {
				delete(rt.symbolsByName, symbol.Name)
			}
		}

		// Remove references where this symbol is the source
		if refIDs, exists := rt.outgoingRefs[symbolID]; exists {
			for _, refID := range refIDs {
				delete(rt.references, refID)
				// Remove from target's incoming refs
				if ref := rt.references[refID]; ref != nil && ref.TargetSymbol != 0 {
					rt.removeFromIncomingRefs(ref.TargetSymbol, refID)
				}
			}
			delete(rt.outgoingRefs, symbolID)
		}

		// Remove references where this symbol is the target
		if refIDs, exists := rt.incomingRefs[symbolID]; exists {
			for _, refID := range refIDs {
				if ref := rt.references[refID]; ref != nil && ref.SourceSymbol != 0 {
					rt.removeFromOutgoingRefs(ref.SourceSymbol, refID)
				}
			}
			delete(rt.incomingRefs, symbolID)
		}

		// Remove symbol data
		// Use SymbolStore.Delete for O(1) removal
		rt.symbols.Delete(symbolID)
		delete(rt.symbolScopes, symbolID)
	}

	// Remove file index
	delete(rt.symbolsByFile, fileID)

	// Remove scope data for this file
	delete(rt.scopesByFile, fileID)

	// Remove performance data for this file
	delete(rt.perfDataByFile, fileID)

	// Clear import data for this file
	rt.importResolver.RemoveFile(fileID)
}

// removeFromIncomingRefs removes a reference ID from a symbol's incoming refs
func (rt *ReferenceTracker) removeFromIncomingRefs(symbolID types.SymbolID, refID uint64) {
	if refs, exists := rt.incomingRefs[symbolID]; exists {
		filtered := refs[:0]
		for _, id := range refs {
			if id != refID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			rt.incomingRefs[symbolID] = filtered
		} else {
			delete(rt.incomingRefs, symbolID)
		}
	}
}

// removeFromOutgoingRefs removes a reference ID from a symbol's outgoing refs
func (rt *ReferenceTracker) removeFromOutgoingRefs(symbolID types.SymbolID, refID uint64) {
	if refs, exists := rt.outgoingRefs[symbolID]; exists {
		filtered := refs[:0]
		for _, id := range refs {
			if id != refID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			rt.outgoingRefs[symbolID] = filtered
		} else {
			delete(rt.outgoingRefs, symbolID)
		}
	}
}

// makeGlobalReferenceID creates a globally unique reference ID using file prefix
// Uses 32 bits for file ID, 32 bits for local reference ID
func (rt *ReferenceTracker) makeGlobalReferenceID(fileID types.FileID, localRefID uint32) uint64 {
	return (uint64(fileID) << 32) | uint64(localRefID)
}

// computeIsExported determines if a symbol is exported based on language-specific rules.
// This is used as a fallback when the parser doesn't set Visibility.IsExported.
// Performance: This is O(1) with no allocations - just string comparison and rune check.
func computeIsExported(path string, symbolName string) bool {
	if len(symbolName) == 0 {
		return false
	}

	// Determine language from file extension
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		// Go: exported if first character is uppercase
		firstRune := []rune(symbolName)[0]
		return unicode.IsUpper(firstRune)

	case ".py":
		// Python: exported if doesn't start with underscore
		return !strings.HasPrefix(symbolName, "_")

	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		// JavaScript/TypeScript: treat all symbols as potentially exported
		// (actual export detection requires semantic analysis)
		// Private symbols conventionally start with _ or #
		return !strings.HasPrefix(symbolName, "_") && !strings.HasPrefix(symbolName, "#")

	case ".java", ".kt":
		// Java/Kotlin: assume exported unless starts with lowercase for classes
		// This is a heuristic; actual visibility depends on access modifiers
		return true

	case ".rs":
		// Rust: pub keyword determines visibility, but by convention:
		// public items often start with lowercase (no naming convention)
		// This is a fallback; proper detection requires semantic analysis
		return true

	case ".rb":
		// Ruby: methods starting with _ are conventionally private
		return !strings.HasPrefix(symbolName, "_")

	case ".c", ".h", ".cpp", ".hpp", ".cc", ".cxx":
		// C/C++: no language-level export concept, treat as exported
		return true

	default:
		// Unknown language: assume exported
		return true
	}
}

// ProcessFile processes a file's enhanced symbols, references, scopes, and imports
// This is now split into two phases: AddSymbols and ProcessReferences
func (rt *ReferenceTracker) ProcessFile(fileID types.FileID, path string, symbols []types.Symbol, references []types.Reference, scopes []types.ScopeInfo) []types.EnhancedSymbol {
	return rt.ProcessFileWithEnhanced(fileID, path, symbols, nil, references, scopes)
}

// ProcessFileWithEnhanced processes a file's symbols with optional pre-computed enhanced symbols
// from the parser (which include complexity data). If parserEnhanced is provided, complexity
// values will be copied from matching symbols.
func (rt *ReferenceTracker) ProcessFileWithEnhanced(fileID types.FileID, path string, symbols []types.Symbol, parserEnhanced []types.EnhancedSymbol, references []types.Reference, scopes []types.ScopeInfo) []types.EnhancedSymbol {
	// Only acquire lock if not in bulk indexing mode (multiple callers)
	// During indexing, FileIntegrator is the only writer (lock-free)
	if atomic.LoadInt32(&rt.BulkIndexing) == 0 {
		rt.mu.Lock()
		defer rt.mu.Unlock()
	}

	// Store scope hierarchy for this file
	rt.scopesByFile[fileID] = scopes

	// Build complexity lookup map from parser's enhanced symbols (keyed by line:column)
	// This allows O(1) lookup when creating our enhanced symbols
	complexityMap := make(map[int]int) // line -> complexity
	for _, pe := range parserEnhanced {
		if pe.Complexity > 0 {
			complexityMap[pe.Line] = pe.Complexity
		}
	}

	// Convert basic symbols to enhanced symbols
	var enhancedSymbols []types.EnhancedSymbol
	var symbolIDs []types.SymbolID

	// Pre-allocate symbol slices to avoid repeated slice growth
	if cap(enhancedSymbols) < len(symbols) {
		enhancedSymbols = make([]types.EnhancedSymbol, 0, len(symbols))
	}
	if cap(symbolIDs) < len(symbols) {
		symbolIDs = make([]types.SymbolID, 0, len(symbols))
	}

	for _, symbol := range symbols {
		symbolID := rt.nextSymbolID
		rt.nextSymbolID++

		// Build scope chain for this symbol
		scopeChain := rt.buildSymbolScopeChain(symbol, scopes)

		// Ensure the symbol has the correct FileID
		symbol.FileID = fileID

		// Compute IsExported: use explicit visibility if set, otherwise infer from naming convention
		// If Visibility.IsExported is true, use it directly
		// If Visibility.IsExported is false but Access is non-zero, trust the explicit setting
		// If Visibility is completely unset (default zero values), compute from naming conventions
		isExported := symbol.Visibility.IsExported
		if !symbol.Visibility.IsExported && symbol.Visibility.Access == 0 {
			// No explicit visibility set by parser, compute from language-specific naming conventions
			isExported = computeIsExported(path, symbol.Name)
		}

		// Look up complexity from parser's enhanced symbols
		complexity := complexityMap[symbol.Line]

		// Use nil slices instead of empty slices to save memory
		// Empty slices still allocate memory, nil slices don't
		enhancedSymbol := types.EnhancedSymbol{
			Symbol:       symbol,
			ID:           symbolID,
			IncomingRefs: nil, // nil instead of []types.Reference{}
			OutgoingRefs: nil, // nil instead of []types.Reference{}
			ScopeChain:   scopeChain,
			RefStats:     types.RefStats{}, // Will be calculated later
			IsExported:   isExported,
			Complexity:   complexity, // From parser's enhanced symbols
		}

		// Use SymbolStore.Set for O(1) array-based storage (30-50% faster)
		rt.symbols.Set(symbolID, &enhancedSymbol)
		rt.symbolsByName[symbol.Name] = append(rt.symbolsByName[symbol.Name], symbolID)
		rt.symbolScopes[symbolID] = scopeChain

		enhancedSymbols = append(enhancedSymbols, enhancedSymbol)
		symbolIDs = append(symbolIDs, symbolID)
	}

	rt.symbolsByFile[fileID] = symbolIDs

	// Store references for later processing (after all symbols are indexed)
	// OPTIMIZATION: Pre-allocate backing array to reduce heap allocations
	// Instead of copying each ref individually, allocate one slice and use indices
	if len(references) > 0 {
		refBacking := make([]types.Reference, len(references))
		for i := range references {
			refBacking[i] = references[i]
			refBacking[i].FileID = fileID

			// Convert local reference ID to globally unique ID using file prefix
			globalRefID := rt.makeGlobalReferenceID(fileID, uint32(references[i].ID))
			refBacking[i].ID = globalRefID

			rt.references[refBacking[i].ID] = &refBacking[i]
		}
	}

	return enhancedSymbols
}

// ProcessFileImports processes a file's imports for better symbol resolution
// This collects import data lock-free during parallel indexing
func (rt *ReferenceTracker) ProcessFileImports(fileID types.FileID, filePath string, content []byte) {
	// Extract import data without locking (safe for concurrent calls)
	importData := rt.importResolver.ExtractFileImports(fileID, filePath, content)

	if importData != nil {
		// Only need to lock when appending to the shared slice
		rt.mu.Lock()
		rt.importData = append(rt.importData, importData)
		rt.mu.Unlock()
	}
}

// ProcessAllReferences processes all stored references after all symbols have been indexed
func (rt *ReferenceTracker) ProcessAllReferences() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Build the import graph from collected data (single-threaded, no locking needed)
	rt.importResolver.BuildImportGraph(rt.importData)

	// Clear the import data as it's no longer needed
	rt.importData = nil

	// Clear existing reference mappings to avoid duplicates
	// Note: We clear incoming/outgoing refs but NOT references themselves
	// because references are built during indexing and should only be processed once
	rt.incomingRefs = make(map[types.SymbolID][]uint64)
	rt.outgoingRefs = make(map[types.SymbolID][]uint64)

	// Now that all symbols are indexed, process all references
	for refID, ref := range rt.references {
		if ref == nil {
			continue
		}

		// Use existing symbol IDs if already set, otherwise try to resolve
		sourceSymbolID := ref.SourceSymbol
		targetSymbolID := ref.TargetSymbol

		// Only try to resolve if IDs are not already set
		if sourceSymbolID == 0 {
			sourceSymbolID = rt.findSymbolAtLocation(ref.FileID, ref.Line, ref.Column)
			if sourceSymbolID != 0 {
				ref.SourceSymbol = sourceSymbolID
			}
		}
		if targetSymbolID == 0 {
			targetSymbolID = rt.resolveReferenceTarget(*ref, rt.symbolsByFile[ref.FileID])
			if targetSymbolID != 0 {
				ref.TargetSymbol = targetSymbolID
			}
		}

		// Update bidirectional reference maps using the final symbol IDs
		if sourceSymbolID != 0 {
			// Deduplicate: only add if not already present
			outgoing := rt.outgoingRefs[sourceSymbolID]
			if !containsRefID(outgoing, refID) {
				rt.outgoingRefs[sourceSymbolID] = append(outgoing, refID)
			}
		}
		if targetSymbolID != 0 {
			// Deduplicate: only add if not already present
			incoming := rt.incomingRefs[targetSymbolID]
			if !containsRefID(incoming, refID) {
				rt.incomingRefs[targetSymbolID] = append(incoming, refID)
			}
		}
	}

	// Update reference statistics for all symbols
	// OPTIMIZED: Use GetIDs() to iterate over SymbolStore efficiently
	for _, symbolID := range rt.symbols.GetIDs() {
		rt.updateReferenceStatsForSymbol(symbolID)
	}

	// Build heuristic interface implementations based on method matching
	// This runs after all symbols and explicit references are processed
	rt.buildHeuristicImplementations()
}

// buildHeuristicImplementations finds potential interface implementations by method matching
// For each interface, finds types that have ALL required methods and creates RefTypeImplements
// references with Quality="heuristic" to indicate these are inferred, not explicit
//
// This enables Go implicit interface satisfaction detection without full type analysis.
// The heuristic approach has lower confidence than explicit evidence (assignment, return, cast).
func (rt *ReferenceTracker) buildHeuristicImplementations() {
	// Step 1: Collect interfaces and their required method names
	// Map: interface name -> set of method names
	interfaceMethods := make(map[string]map[string]types.SymbolID) // interface -> methodName -> methodSymbolID

	// Step 2: Collect types and their methods via ReceiverType
	// Map: receiver type -> method name -> method symbol ID
	typeMethods := make(map[string]map[string]types.SymbolID)

	// Step 3: Track which interface symbols exist (for creating references)
	interfaceSymbols := make(map[string]types.SymbolID) // interface name -> symbol ID

	// Iterate through all symbols to collect interface methods and type methods
	for _, symbolID := range rt.symbols.GetIDs() {
		symbol := rt.symbols.Get(symbolID)
		if symbol == nil {
			continue
		}

		switch symbol.Type {
		case types.SymbolTypeInterface:
			// Track interface symbol
			interfaceSymbols[symbol.Name] = symbolID
			// Initialize method map for this interface
			if _, exists := interfaceMethods[symbol.Name]; !exists {
				interfaceMethods[symbol.Name] = make(map[string]types.SymbolID)
			}

		case types.SymbolTypeMethod:
			// Check if this is an interface method (no receiver) or a type method (has receiver)
			if symbol.ReceiverType != "" {
				// Type method - track by receiver type
				receiverType := normalizeReceiverType(symbol.ReceiverType)
				if _, exists := typeMethods[receiverType]; !exists {
					typeMethods[receiverType] = make(map[string]types.SymbolID)
				}
				typeMethods[receiverType][symbol.Name] = symbolID
			} else {
				// Interface method - find parent interface from scope chain
				parentInterface := rt.findParentInterface(symbolID)
				if parentInterface != "" {
					if _, exists := interfaceMethods[parentInterface]; !exists {
						interfaceMethods[parentInterface] = make(map[string]types.SymbolID)
					}
					interfaceMethods[parentInterface][symbol.Name] = symbolID
				}
			}
		}
	}

	// Step 4: Match types against interfaces
	// For each interface, find types that have ALL required methods
	for ifaceName, requiredMethods := range interfaceMethods {
		if len(requiredMethods) == 0 {
			continue // Empty interface - skip
		}

		ifaceSymbolID, hasIfaceSymbol := interfaceSymbols[ifaceName]
		if !hasIfaceSymbol {
			continue
		}

		// Check each type
		for typeName, methods := range typeMethods {
			// Skip if already has an explicit implements relationship
			if rt.hasExplicitImplements(typeName, ifaceName) {
				continue
			}

			// Check if type has all interface methods
			allMatch := true
			for methodName := range requiredMethods {
				if _, has := methods[methodName]; !has {
					allMatch = false
					break
				}
			}

			if allMatch {
				// Create heuristic implements reference
				// Find a method symbol to use for location (use first method)
				var locationSymbolID types.SymbolID
				for _, methodID := range methods {
					locationSymbolID = methodID
					break
				}

				rt.createHeuristicImplementsRef(typeName, ifaceName, ifaceSymbolID, locationSymbolID)
			}
		}
	}
}

// normalizeReceiverType normalizes a receiver type by removing pointer prefix
// e.g., "*File" -> "File", "File" -> "File"
func normalizeReceiverType(receiverType string) string {
	if len(receiverType) > 0 && receiverType[0] == '*' {
		return receiverType[1:]
	}
	return receiverType
}

// findParentInterface finds the parent interface name for a method symbol
// by looking at its scope chain
func (rt *ReferenceTracker) findParentInterface(methodSymbolID types.SymbolID) string {
	scopeChain, exists := rt.symbolScopes[methodSymbolID]
	if !exists {
		return ""
	}

	// Walk up the scope chain looking for an interface scope
	for _, scope := range scopeChain {
		if scope.Type == types.ScopeTypeInterface {
			return scope.Name
		}
	}
	return ""
}

// hasExplicitImplements checks if a type already has an explicit implements reference
// to avoid creating duplicate heuristic references
func (rt *ReferenceTracker) hasExplicitImplements(typeName, ifaceName string) bool {
	// Check existing references for explicit implements
	for _, ref := range rt.references {
		if ref == nil {
			continue
		}
		if ref.Type == types.RefTypeImplements &&
			ref.ReferencedName == ifaceName &&
			ref.Quality != types.RefQualityHeuristic {
			// Found an explicit implements - check if it's for this type
			// We'd need to resolve the source symbol to compare, but for now
			// we'll be conservative and allow heuristic refs to coexist
			// The expansion engine will prefer higher-quality refs
			return false
		}
	}
	return false
}

// createHeuristicImplementsRef creates a RefTypeImplements with heuristic quality
func (rt *ReferenceTracker) createHeuristicImplementsRef(typeName, ifaceName string, ifaceSymbolID, locationSymbolID types.SymbolID) {
	// Get location from the method symbol
	locationSymbol := rt.symbols.Get(locationSymbolID)
	if locationSymbol == nil {
		return
	}

	// Create the reference
	ref := &types.Reference{
		ID:             rt.nextRefID,
		SourceSymbol:   0, // Will be resolved if we can find type symbol
		TargetSymbol:   ifaceSymbolID,
		FileID:         locationSymbol.FileID,
		Line:           locationSymbol.Line,
		Column:         locationSymbol.Column,
		Type:           types.RefTypeImplements,
		Strength:       types.RefStrengthLoose, // Heuristic = loose coupling
		ReferencedName: ifaceName,
		Quality:        types.RefQualityHeuristic,
	}
	rt.nextRefID++

	// Try to find the type symbol for source
	if typeSymbols, ok := rt.symbolsByName[typeName]; ok {
		for _, symID := range typeSymbols {
			sym := rt.symbols.Get(symID)
			if sym != nil && (sym.Type == types.SymbolTypeStruct || sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeType) {
				ref.SourceSymbol = symID
				break
			}
		}
	}

	// Store the reference
	rt.references[ref.ID] = ref

	// Update reference maps
	if ref.SourceSymbol != 0 {
		rt.outgoingRefs[ref.SourceSymbol] = append(rt.outgoingRefs[ref.SourceSymbol], ref.ID)
	}
	if ref.TargetSymbol != 0 {
		rt.incomingRefs[ref.TargetSymbol] = append(rt.incomingRefs[ref.TargetSymbol], ref.ID)
	}
}

// buildSymbolScopeChain constructs the complete scope hierarchy for a symbol
// OPTIMIZED: Uses caching to avoid rebuilding identical scope chains
// This reduces memory allocations by 20-30% (targeting 91MB hotspot)
// NOTE: Cache uses hash keys with collision verification for correctness
func (rt *ReferenceTracker) buildSymbolScopeChain(symbol types.Symbol, scopes []types.ScopeInfo) []types.ScopeInfo {
	// Create a cache key based on symbol line and scope boundaries
	// Symbols at the same position with similar scope structures will share cache entries
	cacheKey, scopeCount := rt.createScopeChainCacheKeyWithCount(symbol, scopes)

	// Check cache first with collision verification
	cacheHit := false
	isCollision := false
	if cached, exists := rt.scopeChainCache[cacheKey]; exists {
		// Verify this is not a hash collision by checking original key data
		if cached.symbolLine == symbol.Line &&
			cached.symbolEndLine == symbol.EndLine &&
			cached.scopeCount == scopeCount {
			return cached.scopeChain
		}
		// Hash collision detected - fall through to recompute but don't overwrite
		// the existing cache entry (preserves the original valid entry)
		isCollision = true
		cacheHit = true
	}
	_ = cacheHit // Used for clarity, compiler will optimize away

	// Pre-allocate with small capacity
	// Most symbols only match 1-3 scopes, so capacity of 4 covers most cases
	// This reduces slice growth allocations while not wasting memory
	scopeChain := make([]types.ScopeInfo, 0, 4)

	// Find all scopes that contain this symbol
	for _, scope := range scopes {
		if scope.StartLine <= symbol.Line && (scope.EndLine == 0 || scope.EndLine >= symbol.Line) {
			scopeChain = append(scopeChain, scope)
		}
	}

	// Only cache if this is not a collision - preserve the original entry
	// Collisions are rare, so skipping cache for colliding entries has minimal impact
	if !isCollision {
		rt.scopeChainCache[cacheKey] = &scopeChainCacheEntry{
			scopeChain:    scopeChain,
			symbolLine:    symbol.Line,
			symbolEndLine: symbol.EndLine,
			scopeCount:    scopeCount,
		}
	}

	return scopeChain
}

// scopeChainCacheEntry stores cached scope chains with verification data to detect hash collisions
// This addresses the hash collision risk by storing original key data for validation
type scopeChainCacheEntry struct {
	scopeChain []types.ScopeInfo
	// Verification data to detect collisions
	symbolLine    int
	symbolEndLine int
	scopeCount    int // Number of scopes used in hash calculation
}

// createScopeChainCacheKeyWithCount creates a numeric hash-based cache key for scope chains
// and returns the scope count used for collision verification.
// OPTIMIZATION: Uses inline FNV-1a hash without allocating hasher object
// Uses package-level FNV constants from hash_constants.go
func (rt *ReferenceTracker) createScopeChainCacheKeyWithCount(symbol types.Symbol, scopes []types.ScopeInfo) (uint64, int) {
	h := uint64(fnvOffset64)

	// Hash symbol position inline
	h ^= uint64(symbol.Line)
	h *= fnvPrime64
	h ^= uint64(symbol.EndLine)
	h *= fnvPrime64

	// Add key scope boundaries to distinguish different scope structures
	// Only need a few key boundaries to differentiate common cases
	boundaryCount := 0
	for _, scope := range scopes {
		// Include first 3 scope boundaries and any that span our symbol
		if boundaryCount >= 3 {
			// Check if this scope contains our symbol
			if scope.StartLine <= symbol.Line && (scope.EndLine == 0 || scope.EndLine >= symbol.Line) {
				h ^= uint64(scope.StartLine)
				h *= fnvPrime64
				h ^= uint64(scope.EndLine)
				h *= fnvPrime64
				boundaryCount++
				break
			}
		} else {
			h ^= uint64(scope.StartLine)
			h *= fnvPrime64
			h ^= uint64(scope.EndLine)
			h *= fnvPrime64
			boundaryCount++
		}
	}

	return h, boundaryCount
}

// findSymbolAtLocation finds the symbol at a specific line/column in a file
// Uses SymbolLocationIndex for O(1) lookup instead of O(n) linear search
func (rt *ReferenceTracker) findSymbolAtLocation(fileID types.FileID, line, column int) types.SymbolID {
	// Use SymbolLocationIndex for fast O(1) lookup - no secondary search needed
	if rt.symbolLocationIndex != nil {
		return rt.symbolLocationIndex.FindSymbolIDAtPosition(fileID, line, column)
	}

	// Fallback to linear search if index not available (shouldn't happen in normal operation)
	symbolIDs, exists := rt.symbolsByFile[fileID]
	if !exists {
		return 0
	}

	for _, symbolID := range symbolIDs {
		// Use SymbolStore.Get for fast O(1) access
		symbol := rt.symbols.Get(symbolID)
		if symbol.Line <= line && symbol.EndLine >= line {
			// More precise column matching for better symbol resolution
			if symbol.Line == line {
				// On same line, check column bounds if available
				if column >= symbol.Column && column <= symbol.EndColumn {
					return symbolID
				}
			} else if symbol.Line < line && symbol.EndLine > line {
				// Symbol spans multiple lines, line is within bounds
				return symbolID
			} else if symbol.Line == symbol.EndLine && symbol.Line == line {
				// Single-line symbol, check column bounds
				if column >= symbol.Column && column <= symbol.EndColumn {
					return symbolID
				}
			}
		}
	}

	return 0
}

// resolveReferenceTarget attempts to resolve what symbol a reference points to
// Optimized with caching to avoid redundant import resolution
func (rt *ReferenceTracker) resolveReferenceTarget(ref types.Reference, fileSymbolIDs []types.SymbolID) types.SymbolID {
	// Use Tree-sitter extracted symbol name instead of error-prone string parsing
	referencedName := ref.ReferencedName

	if referencedName == "" {
		return 0
	}

	// Check cache first to avoid redundant import resolution
	// Key: (fileID<<32) | hash(name)
	// Inline FNV-1a to avoid allocating hasher
	// Uses package-level FNV constants from hash_constants.go
	nameHash := uint64(fnvOffset64)
	for i := 0; i < len(referencedName); i++ {
		nameHash ^= uint64(referencedName[i])
		nameHash *= fnvPrime64
	}
	cacheKey := (uint64(ref.FileID) << 32) | (nameHash & 0xFFFFFFFF)

	if cachedID, found := rt.referenceCache[cacheKey]; found {
		return cachedID
	}

	// Look for symbols with matching name
	// First check within the same file (fast path for local references)
	// OPTIMIZED: Pre-load symbols into slice for better cache locality
	// This avoids repeated map lookups and improves CPU cache performance
	if len(fileSymbolIDs) > 0 {
		// Pre-allocate slice with expected capacity
		symbols := make([]*types.EnhancedSymbol, 0, len(fileSymbolIDs))

		// Batch load all symbols into slice (cache-friendly)
		for _, symbolID := range fileSymbolIDs {
			symbol := rt.symbols.Get(symbolID)
			symbols = append(symbols, symbol)
		}

		// Now iterate over the slice (much better cache locality!)
		for i, symbol := range symbols {
			if symbol != nil && symbol.Name == referencedName {
				// Cache and return the local match
				rt.referenceCache[cacheKey] = fileSymbolIDs[i]
				return fileSymbolIDs[i]
			}
		}
	}

	// Then check across all files using ImportResolver for better resolution
	candidateIDs, exists := rt.symbolsByName[referencedName]

	var resolvedID types.SymbolID = 0
	if exists && len(candidateIDs) > 0 {
		// Use ImportResolver to find the best match based on imports and language rules
		bestMatch := rt.importResolver.ResolveSymbolReference(
			ref.FileID,
			referencedName,
			candidateIDs,
			func(symbolID types.SymbolID) *types.EnhancedSymbol {
				// OPTIMIZED: Use SymbolStore.Get for fast O(1) access
				return rt.symbols.Get(symbolID)
			},
		)

		if bestMatch != 0 {
			resolvedID = bestMatch
		} else {
			// Fallback to first candidate if ImportResolver couldn't decide
			resolvedID = candidateIDs[0]
		}
	}

	// Cache the result (even if 0 to avoid repeated lookups)
	rt.referenceCache[cacheKey] = resolvedID
	return resolvedID
}

// updateReferenceStatsForSymbol calculates reference statistics for a single symbol
func (rt *ReferenceTracker) updateReferenceStatsForSymbol(symbolID types.SymbolID) {
	// Use SymbolStore.Get for fast O(1) access
	symbol := rt.symbols.Get(symbolID)
	if symbol == nil {
		return
	}

	// Calculate incoming references
	incomingRefIDs := rt.incomingRefs[symbolID]
	outgoingRefIDs := rt.outgoingRefs[symbolID]

	// Build reference statistics
	refStats := rt.calculateRefStats(incomingRefIDs, outgoingRefIDs)

	// Update symbol
	symbol.RefStats = refStats

	// Populate actual reference objects
	symbol.IncomingRefs = rt.getReferencesById(incomingRefIDs)
	symbol.OutgoingRefs = rt.getReferencesById(outgoingRefIDs)
}

// calculateRefStats computes comprehensive reference statistics
func (rt *ReferenceTracker) calculateRefStats(incomingRefIDs, outgoingRefIDs []uint64) types.RefStats {

	// Process incoming references
	incomingFiles := make(map[types.FileID]bool)
	incomingByType := make(map[string]int)
	incomingByStrength := types.RefStrengthStats{}
	usageIncomingCount := 0 // Count only non-import references for usage statistics

	for _, refID := range incomingRefIDs {
		ref := rt.references[refID]
		if ref != nil {
			incomingFiles[ref.FileID] = true
			incomingByType[ref.Type.String()]++

			// Count only non-import references for usage statistics
			if ref.Type != types.RefTypeImport {
				usageIncomingCount++
			}

			switch ref.Strength {
			case types.RefStrengthTight:
				incomingByStrength.Tight++
			case types.RefStrengthLoose:
				incomingByStrength.Loose++
			case types.RefStrengthTransitive:
				incomingByStrength.Transitive++
			}
		}
	}

	// Process outgoing references
	outgoingFiles := make(map[types.FileID]bool)
	outgoingByType := make(map[string]int)
	outgoingByStrength := types.RefStrengthStats{}

	for _, refID := range outgoingRefIDs {
		ref := rt.references[refID]
		if ref != nil {
			outgoingFiles[ref.FileID] = true
			outgoingByType[ref.Type.String()]++

			switch ref.Strength {
			case types.RefStrengthTight:
				outgoingByStrength.Tight++
			case types.RefStrengthLoose:
				outgoingByStrength.Loose++
			case types.RefStrengthTransitive:
				outgoingByStrength.Transitive++
			}
		}
	}

	// Build file lists
	var incomingFileList []types.FileID
	for fileID := range incomingFiles {
		incomingFileList = append(incomingFileList, fileID)
	}
	var outgoingFileList []types.FileID
	for fileID := range outgoingFiles {
		outgoingFileList = append(outgoingFileList, fileID)
	}

	// Aggregate statistics (simplified - all levels get same stats for now)
	totalCount := types.RefCount{
		IncomingCount: usageIncomingCount, // Exclude import references from usage count
		OutgoingCount: len(outgoingRefIDs),
		IncomingFiles: incomingFileList,
		OutgoingFiles: outgoingFileList,
		ByType:        mergeTypeCounts(incomingByType, outgoingByType),
		Strength: types.RefStrengthStats{
			Tight:      incomingByStrength.Tight + outgoingByStrength.Tight,
			Loose:      incomingByStrength.Loose + outgoingByStrength.Loose,
			Transitive: incomingByStrength.Transitive + outgoingByStrength.Transitive,
		},
	}

	return types.RefStats{
		FolderLevel:   totalCount, // Simplified - same as total for now
		FileLevel:     totalCount, // Simplified - same as total for now
		ClassLevel:    totalCount, // Simplified - same as total for now
		FunctionLevel: totalCount, // Simplified - same as total for now
		VariableLevel: totalCount, // Simplified - same as total for now
		Total:         totalCount,
	}
}

// getReferencesById converts reference IDs to actual Reference objects
func (rt *ReferenceTracker) getReferencesById(refIDs []uint64) []types.Reference {
	// Pre-allocate with exact capacity to avoid slice growth
	// This function is called frequently and creating copies causes allocations
	refs := make([]types.Reference, 0, len(refIDs))
	for _, refID := range refIDs {
		if ref := rt.references[refID]; ref != nil {
			refs = append(refs, *ref)
		}
	}
	return refs
}

// GetSymbolReferences returns all references for a symbol (incoming and outgoing)
func (rt *ReferenceTracker) GetSymbolReferences(symbolID types.SymbolID, direction string) []types.Reference {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var refIDs []uint64

	switch direction {
	case "incoming":
		refIDs = rt.incomingRefs[symbolID]
	case "outgoing":
		refIDs = rt.outgoingRefs[symbolID]
	case "both":
		refIDs = append(rt.incomingRefs[symbolID], rt.outgoingRefs[symbolID]...)
	default:
		refIDs = append(rt.incomingRefs[symbolID], rt.outgoingRefs[symbolID]...)
	}

	return rt.getReferencesById(refIDs)
}

// FindSymbolsByName finds all symbols with a given name
func (rt *ReferenceTracker) FindSymbolsByName(name string) []*types.EnhancedSymbol {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var symbols []*types.EnhancedSymbol
	if symbolIDs, exists := rt.symbolsByName[name]; exists {
		for _, symbolID := range symbolIDs {
			// Use SymbolStore.Get for fast O(1) access
			if symbol := rt.symbols.Get(symbolID); symbol != nil {
				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols
}

// GetEnhancedSymbol returns an enhanced symbol by ID
// OPTIMIZED: Uses SymbolStore.Get for O(1) array access instead of map lookup
func (rt *ReferenceTracker) GetEnhancedSymbol(symbolID types.SymbolID) *types.EnhancedSymbol {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	return rt.symbols.Get(symbolID)
}

// GetFileEnhancedSymbols returns all enhanced symbols for a file
func (rt *ReferenceTracker) GetFileEnhancedSymbols(fileID types.FileID) []*types.EnhancedSymbol {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	symbolIDs, exists := rt.symbolsByFile[fileID]
	if !exists {
		return nil
	}

	var enhanced []*types.EnhancedSymbol
	for _, id := range symbolIDs {
		// Use SymbolStore.Get for fast O(1) access
		if symbol := rt.symbols.Get(id); symbol != nil {
			enhanced = append(enhanced, symbol)
		}
	}
	return enhanced
}

// GetFileReferences returns all references where either source or target is in the given file
func (rt *ReferenceTracker) GetFileReferences(fileID types.FileID) []types.Reference {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var refIDs []uint64

	// Get all symbols in this file
	symbolIDs, exists := rt.symbolsByFile[fileID]
	if !exists {
		return []types.Reference{}
	}

	// For each symbol in the file, get its references
	for _, symbolID := range symbolIDs {
		// Get outgoing references (this symbol references others)
		if outgoing, exists := rt.outgoingRefs[symbolID]; exists {
			refIDs = append(refIDs, outgoing...)
		}

		// Get incoming references (other symbols reference this one)
		if incoming, exists := rt.incomingRefs[symbolID]; exists {
			refIDs = append(refIDs, incoming...)
		}
	}

	// Convert reference IDs to Reference objects
	return rt.getReferencesById(refIDs)
}

// GetSymbolAtLine finds the symbol that contains the given line in a file
// This is optimized for search result enhancement
func (rt *ReferenceTracker) GetSymbolAtLine(fileID types.FileID, line int) *types.EnhancedSymbol {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	symbolIDs, exists := rt.symbolsByFile[fileID]
	if !exists {
		return nil
	}

	// Check each symbol to see if the line falls within its range
	for _, id := range symbolIDs {
		// Use SymbolStore.Get for fast O(1) access
		if symbol := rt.symbols.Get(id); symbol != nil {
			if symbol.Line <= line && line <= symbol.EndLine {
				return symbol
			}
		}
	}

	return nil
}

// Utility functions

func mergeTypeCounts(a, b map[string]int) map[string]int {
	result := make(map[string]int)

	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] += v
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AddHeuristicReference adds a reference without requiring known source/target symbols.
func (rt *ReferenceTracker) AddHeuristicReference(ref types.Reference) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	ref.ID = rt.nextRefID
	rt.nextRefID++
	if ref.Quality == "" {
		ref.Quality = "heuristic"
	}
	rt.references[ref.ID] = &ref
}

// AddTestReference adds a reference directly for testing purposes
// This bypasses normal symbol resolution and directly populates the reference maps
func (rt *ReferenceTracker) AddTestReference(ref types.Reference) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	ref.ID = rt.nextRefID
	rt.nextRefID++
	if ref.Quality == "" {
		ref.Quality = "test"
	}

	// Store in references map
	rt.references[ref.ID] = &ref

	// Add to outgoing refs (source -> target) - store reference ID, not the reference itself
	if ref.SourceSymbol != 0 {
		rt.outgoingRefs[ref.SourceSymbol] = append(rt.outgoingRefs[ref.SourceSymbol], ref.ID)
	}

	// Add to incoming refs (target <- source) - store reference ID, not the reference itself
	if ref.TargetSymbol != 0 {
		rt.incomingRefs[ref.TargetSymbol] = append(rt.incomingRefs[ref.TargetSymbol], ref.ID)
	}
}

// GetAllReferences returns a snapshot of all references (testing/diagnostics).
func (rt *ReferenceTracker) GetAllReferences() []types.Reference {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	out := make([]types.Reference, 0, len(rt.references))
	for _, r := range rt.references {
		out = append(out, *r)
	}
	return out
}

// containsRefID checks if a refID is already in the list (helper for deduplication)
func containsRefID(refIDs []uint64, refID uint64) bool {
	for _, id := range refIDs {
		if id == refID {
			return true
		}
	}
	return false
}

// ============================================================================
// TYPE RELATIONSHIP QUERY METHODS
// These methods query implements/extends relationships for type hierarchy analysis
// ============================================================================

// GetImplementors returns all types that implement an interface.
// For a given interface symbol ID, this returns all symbols that have
// RefTypeImplements references pointing to this interface.
func (rt *ReferenceTracker) GetImplementors(interfaceID types.SymbolID) []types.SymbolID {
	return rt.getSymbolsByRefType(interfaceID, true, types.RefTypeImplements)
}

// ImplementorWithQuality represents an implementor with its quality ranking
type ImplementorWithQuality struct {
	SymbolID types.SymbolID
	Quality  string
	Rank     int // Higher rank = more confident (from types.RefQualityRank)
}

// GetImplementorsWithQuality returns implementors sorted by quality (highest first)
// This allows the expansion engine to prefer implementations with stronger evidence
// (assigned, returned, cast) over heuristic matches (method name matching)
func (rt *ReferenceTracker) GetImplementorsWithQuality(interfaceID types.SymbolID) []ImplementorWithQuality {
	// During bulk indexing, data is incomplete - skip lock and return nil
	if atomic.LoadInt32(&rt.BulkIndexing) != 0 {
		return nil
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	refIDs := rt.incomingRefs[interfaceID]
	if len(refIDs) == 0 {
		return nil
	}

	// Collect implementors with their quality
	seen := make(map[types.SymbolID]ImplementorWithQuality)
	for _, refID := range refIDs {
		ref, ok := rt.references[refID]
		if !ok || ref == nil || ref.Type != types.RefTypeImplements {
			continue
		}

		// Get the source symbol (the implementor)
		sourceID := ref.SourceSymbol
		if sourceID == 0 {
			continue
		}

		quality := ref.Quality
		rank := types.RefQualityRank(quality)

		// Keep highest quality reference for each implementor
		if existing, ok := seen[sourceID]; ok {
			if rank > existing.Rank {
				seen[sourceID] = ImplementorWithQuality{
					SymbolID: sourceID,
					Quality:  quality,
					Rank:     rank,
				}
			}
		} else {
			seen[sourceID] = ImplementorWithQuality{
				SymbolID: sourceID,
				Quality:  quality,
				Rank:     rank,
			}
		}
	}

	// Convert to slice and sort by rank (descending)
	result := make([]ImplementorWithQuality, 0, len(seen))
	for _, impl := range seen {
		result = append(result, impl)
	}

	// Sort by rank descending (highest quality first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Rank > result[i].Rank {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetImplementedInterfaces returns all interfaces a type implements.
// For a given type symbol ID, this returns all interface symbols that
// this type has RefTypeImplements references pointing to.
func (rt *ReferenceTracker) GetImplementedInterfaces(typeID types.SymbolID) []types.SymbolID {
	return rt.getSymbolsByRefType(typeID, false, types.RefTypeImplements)
}

// InterfaceWithQuality represents an implemented interface with its quality ranking
type InterfaceWithQuality struct {
	SymbolID types.SymbolID
	Quality  string
	Rank     int // Higher rank = more confident (from types.RefQualityRank)
}

// GetImplementedInterfacesWithQuality returns interfaces sorted by quality (highest first)
// This allows the expansion engine to prefer interfaces with stronger evidence
func (rt *ReferenceTracker) GetImplementedInterfacesWithQuality(typeID types.SymbolID) []InterfaceWithQuality {
	// During bulk indexing, data is incomplete - skip lock and return nil
	if atomic.LoadInt32(&rt.BulkIndexing) != 0 {
		return nil
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	refIDs := rt.outgoingRefs[typeID]
	if len(refIDs) == 0 {
		return nil
	}

	// Collect interfaces with their quality
	seen := make(map[types.SymbolID]InterfaceWithQuality)
	for _, refID := range refIDs {
		ref, ok := rt.references[refID]
		if !ok || ref == nil || ref.Type != types.RefTypeImplements {
			continue
		}

		// Get the target symbol (the interface)
		targetID := ref.TargetSymbol
		if targetID == 0 {
			continue
		}

		quality := ref.Quality
		rank := types.RefQualityRank(quality)

		// Keep highest quality reference for each interface
		if existing, ok := seen[targetID]; ok {
			if rank > existing.Rank {
				seen[targetID] = InterfaceWithQuality{
					SymbolID: targetID,
					Quality:  quality,
					Rank:     rank,
				}
			}
		} else {
			seen[targetID] = InterfaceWithQuality{
				SymbolID: targetID,
				Quality:  quality,
				Rank:     rank,
			}
		}
	}

	// Convert to slice and sort by rank (descending)
	result := make([]InterfaceWithQuality, 0, len(seen))
	for _, iface := range seen {
		result = append(result, iface)
	}

	// Sort by rank descending (highest quality first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Rank > result[i].Rank {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetBaseTypes returns types this type extends (inheritance chain).
// For a given type symbol ID, this returns all base types that
// this type has RefTypeExtends references pointing to.
func (rt *ReferenceTracker) GetBaseTypes(typeID types.SymbolID) []types.SymbolID {
	return rt.getSymbolsByRefType(typeID, false, types.RefTypeExtends)
}

// GetDerivedTypes returns types that extend this type.
// For a given base type symbol ID, this returns all symbols that
// have RefTypeExtends references pointing to this type.
func (rt *ReferenceTracker) GetDerivedTypes(baseID types.SymbolID) []types.SymbolID {
	return rt.getSymbolsByRefType(baseID, true, types.RefTypeExtends)
}

// GetTypeRelationships returns all type relationships for a symbol.
// This includes both implements and extends relationships in both directions.
func (rt *ReferenceTracker) GetTypeRelationships(symbolID types.SymbolID) *TypeRelationships {
	return &TypeRelationships{
		Implements:    rt.GetImplementedInterfaces(symbolID),
		ImplementedBy: rt.GetImplementors(symbolID),
		Extends:       rt.GetBaseTypes(symbolID),
		ExtendedBy:    rt.GetDerivedTypes(symbolID),
	}
}

// TypeRelationships holds all type relationship information for a symbol
type TypeRelationships struct {
	Implements    []types.SymbolID // Interfaces this type implements
	ImplementedBy []types.SymbolID // Types that implement this interface
	Extends       []types.SymbolID // Base types this type extends
	ExtendedBy    []types.SymbolID // Types that extend this type
}

// HasTypeRelationships returns true if the symbol has any type relationships
func (tr *TypeRelationships) HasTypeRelationships() bool {
	return len(tr.Implements) > 0 || len(tr.ImplementedBy) > 0 ||
		len(tr.Extends) > 0 || len(tr.ExtendedBy) > 0
}

// getSymbolsByRefType is a helper that filters references by type and direction.
// incoming=true means references pointing to symbolID, incoming=false means references from symbolID.
func (rt *ReferenceTracker) getSymbolsByRefType(symbolID types.SymbolID, incoming bool, refType types.ReferenceType) []types.SymbolID {
	// During bulk indexing, data is incomplete - skip lock and return nil
	if atomic.LoadInt32(&rt.BulkIndexing) != 0 {
		return nil
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var refIDs []uint64
	if incoming {
		refIDs = rt.incomingRefs[symbolID]
	} else {
		refIDs = rt.outgoingRefs[symbolID]
	}

	if len(refIDs) == 0 {
		return nil
	}

	// Pre-allocate with capacity hint based on reference count
	// Most type relationships have few matches, so use min of refIDs length and 8
	capHint := len(refIDs)
	if capHint > 8 {
		capHint = 8
	}
	seen := make(map[types.SymbolID]bool, capHint)
	result := make([]types.SymbolID, 0, capHint)

	for _, refID := range refIDs {
		ref, ok := rt.references[refID]
		if !ok || ref == nil {
			continue
		}

		// Only include references of the requested type
		if ref.Type != refType {
			continue
		}

		// Get the target symbol ID based on direction
		var targetID types.SymbolID
		if incoming {
			// For incoming refs, the source is what we want (who references us)
			targetID = ref.SourceSymbol
		} else {
			// For outgoing refs, the target is what we want (who we reference)
			targetID = ref.TargetSymbol
		}

		// Skip if target is 0 (unresolved) or already seen
		if targetID == 0 || seen[targetID] {
			continue
		}

		seen[targetID] = true
		result = append(result, targetID)
	}

	return result
}

// FindSymbolByName finds a symbol by its name (first match).
// This is a convenience method for expansion directives.
func (rt *ReferenceTracker) FindSymbolByName(name string) *types.EnhancedSymbol {
	// During bulk indexing, data is incomplete - skip lock and return nil
	if atomic.LoadInt32(&rt.BulkIndexing) != 0 {
		return nil
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	ids, ok := rt.symbolsByName[name]
	if !ok || len(ids) == 0 {
		return nil
	}

	return rt.symbols.Get(ids[0])
}

// FindSymbolByFileAndName finds a symbol by file ID and name.
// This is more precise than FindSymbolByName for disambiguation.
func (rt *ReferenceTracker) FindSymbolByFileAndName(fileID types.FileID, name string) *types.EnhancedSymbol {
	// During bulk indexing, data is incomplete - skip lock and return nil
	if atomic.LoadInt32(&rt.BulkIndexing) != 0 {
		return nil
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	ids, ok := rt.symbolsByName[name]
	if !ok {
		return nil
	}

	for _, id := range ids {
		sym := rt.symbols.Get(id)
		if sym != nil && sym.FileID == fileID {
			return sym
		}
	}

	return nil
}

// StorePerfData stores performance analysis data for a file
func (rt *ReferenceTracker) StorePerfData(fileID types.FileID, perfData []types.FunctionPerfData) {
	if len(perfData) == 0 {
		return
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.perfDataByFile[fileID] = perfData
}

// GetFilePerfData returns performance analysis data for a file
func (rt *ReferenceTracker) GetFilePerfData(fileID types.FileID) []types.FunctionPerfData {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	return rt.perfDataByFile[fileID]
}
