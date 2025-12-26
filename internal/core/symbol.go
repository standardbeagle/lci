package core

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// SymbolIndexStats contains pre-computed, queryable statistics
type SymbolIndexStats struct {
	TotalSymbols   int `json:"total_symbols"`
	TotalFunctions int `json:"total_functions"`
	TotalMethods   int `json:"total_methods"`
	TotalTypes     int `json:"total_types"`
	TotalVariables int `json:"total_variables"`
	FileCount      int `json:"file_count"`

	// Top-N sorted results
	TopSymbols      []SymbolWithScore `json:"top_symbols,omitempty"`
	ExportedSymbols []types.Symbol    `json:"exported_symbols,omitempty"`
	EntryPoints     []types.Symbol    `json:"entry_points,omitempty"`

	// Distribution data
	TypeDistribution map[types.SymbolType]int `json:"type_distribution"`
}

type SymbolWithScore struct {
	Symbol types.Symbol `json:"symbol"`
	Score  int          `json:"score"` // Reference count
}

type SymbolIndex struct {
	definitions map[string][]types.Symbol               // symbol name -> definitions
	references  map[string][]types.Symbol               // symbol name -> references
	metrics     map[types.FileID]map[string]interface{} // fileID -> symbol name -> metrics
	mu          sync.RWMutex

	// Flag to indicate bulk indexing mode (lock-free when true)
	BulkIndexing int32

	// NEW: Pre-computed statistics (read-only, O(1) queries)
	stats SymbolIndexStats

	// NEW: Incremental statistics tracking (for optimization)
	// Track symbol frequency for TopSymbols computation
	symbolFrequency map[string]int

	// Track file count separately to avoid O(n) scan
	fileCount int
}

func NewSymbolIndex() *SymbolIndex {
	return &SymbolIndex{
		definitions: make(map[string][]types.Symbol),
		references:  make(map[string][]types.Symbol),
		metrics:     make(map[types.FileID]map[string]interface{}),
		stats: SymbolIndexStats{
			TypeDistribution: make(map[types.SymbolType]int),
		},
		// NEW: Initialize incremental tracking fields
		symbolFrequency: make(map[string]int),
	}
}

// GetStats returns pre-computed symbol statistics
func (si *SymbolIndex) GetStats() SymbolIndexStats {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.stats
}

// GetEntryPoints returns all entry point symbols (main, init, exported)
func (si *SymbolIndex) GetEntryPoints() []types.Symbol {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if len(si.stats.EntryPoints) == 0 {
		return nil
	}
	// Return copy to avoid race conditions
	entries := make([]types.Symbol, len(si.stats.EntryPoints))
	copy(entries, si.stats.EntryPoints)
	return entries
}

// GetTopSymbols returns most referenced symbols
func (si *SymbolIndex) GetTopSymbols(limit int) []SymbolWithScore {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if limit > len(si.stats.TopSymbols) {
		limit = len(si.stats.TopSymbols)
	}
	if limit == 0 {
		return nil
	}
	symbols := make([]SymbolWithScore, limit)
	copy(symbols, si.stats.TopSymbols[:limit])
	return symbols
}

// GetTypeDistribution returns histogram of symbol types
func (si *SymbolIndex) GetTypeDistribution() map[types.SymbolType]int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	dist := make(map[types.SymbolType]int, len(si.stats.TypeDistribution))
	for k, v := range si.stats.TypeDistribution {
		dist[k] = v
	}
	return dist
}

func (si *SymbolIndex) IndexSymbols(fileID types.FileID, symbols []types.Symbol) {
	// Only acquire lock if not in bulk indexing mode (multiple callers)
	// During indexing, FileIntegrator is the only writer (lock-free)
	if atomic.LoadInt32(&si.BulkIndexing) == 0 {
		si.mu.Lock()
		defer si.mu.Unlock()
	}

	definitionCount := 0
	referenceCount := 0

	for _, symbol := range symbols {
		// Index as definition if it's a declaration
		if isDefinition(symbol.Type) {
			si.definitions[symbol.Name] = append(si.definitions[symbol.Name], symbol)
			definitionCount++
		} else {
			// Otherwise it's a reference
			si.references[symbol.Name] = append(si.references[symbol.Name], symbol)
			referenceCount++
		}
	}

	// Store metrics for this file
	if si.metrics[fileID] == nil {
		si.metrics[fileID] = make(map[string]interface{})
	}

	// OPTIMIZED: Update statistics incrementally instead of full recomputation
	// This is O(n) for the current file only, not O(total_files)
	si.UpdateStatsIncremental(fileID, symbols)
}

func (si *SymbolIndex) UpdateSymbols(fileID types.FileID, oldSymbols, newSymbols []types.Symbol) {
	si.mu.Lock()
	defer si.mu.Unlock()

	// Remove old symbols
	for _, symbol := range oldSymbols {
		if isDefinition(symbol.Type) {
			si.definitions[symbol.Name] = removeSymbolFromSlice(si.definitions[symbol.Name], fileID)
			if len(si.definitions[symbol.Name]) == 0 {
				delete(si.definitions, symbol.Name)
			}
		} else {
			si.references[symbol.Name] = removeSymbolFromSlice(si.references[symbol.Name], fileID)
			if len(si.references[symbol.Name]) == 0 {
				delete(si.references, symbol.Name)
			}
		}
	}

	// Add new symbols
	for _, symbol := range newSymbols {
		if isDefinition(symbol.Type) {
			si.definitions[symbol.Name] = append(si.definitions[symbol.Name], symbol)
		} else {
			si.references[symbol.Name] = append(si.references[symbol.Name], symbol)
		}
	}

	// For updates, we need to recompute stats (more complex than incremental)
	// This is less common than initial indexing, so full recomputation is acceptable
	// OPTIMIZED: Use incremental updates for both remove and add
	// Remove old stats
	if len(oldSymbols) > 0 {
		si.updateStatsForRemoval(fileID, oldSymbols)
	}
	// Add new stats
	if len(newSymbols) > 0 {
		si.UpdateStatsIncremental(fileID, newSymbols)
	}
}

func (si *SymbolIndex) RemoveSymbols(fileID types.FileID, symbols []types.Symbol) {
	si.mu.Lock()
	defer si.mu.Unlock()

	for _, symbol := range symbols {
		if isDefinition(symbol.Type) {
			si.definitions[symbol.Name] = removeSymbolFromSlice(si.definitions[symbol.Name], fileID)
			if len(si.definitions[symbol.Name]) == 0 {
				delete(si.definitions, symbol.Name)
			}
		} else {
			si.references[symbol.Name] = removeSymbolFromSlice(si.references[symbol.Name], fileID)
			if len(si.references[symbol.Name]) == 0 {
				delete(si.references, symbol.Name)
			}
		}
	}

	// OPTIMIZED: Update statistics incrementally for removals
	if len(symbols) > 0 {
		si.updateStatsForRemoval(fileID, symbols)
	}
}

// RemoveFileSymbols removes all symbols for a specific file
func (si *SymbolIndex) RemoveFileSymbols(fileID types.FileID) {
	si.mu.Lock()
	defer si.mu.Unlock()

	// Collect all symbols being removed for incremental stats update
	var removedSymbols []types.Symbol

	// Remove from definitions
	for name, symbols := range si.definitions {
		filtered := removeSymbolFromSlice(symbols, fileID)
		if len(filtered) < len(symbols) {
			// Some symbols were removed, collect them
			for _, sym := range symbols {
				if !containsFileID(filtered, fileID, sym) {
					removedSymbols = append(removedSymbols, sym)
				}
			}
		}
		si.definitions[name] = filtered
		if len(filtered) == 0 {
			delete(si.definitions, name)
		}
	}

	// Remove from references
	for name, symbols := range si.references {
		filtered := removeSymbolFromSlice(symbols, fileID)
		if len(filtered) < len(symbols) {
			// Some symbols were removed, collect them
			for _, sym := range symbols {
				if !containsFileID(filtered, fileID, sym) {
					removedSymbols = append(removedSymbols, sym)
				}
			}
		}
		si.references[name] = filtered
		if len(filtered) == 0 {
			delete(si.references, name)
		}
	}

	// Remove metrics
	delete(si.metrics, fileID)

	// Decrement file count
	if si.fileCount > 0 {
		si.fileCount--
	}

	// OPTIMIZED: Update stats incrementally
	if len(removedSymbols) > 0 {
		si.updateStatsForRemoval(fileID, removedSymbols)
	}
}

// containsFileID checks if a symbol list contains a symbol with the given fileID
func containsFileID(symbols []types.Symbol, fileID types.FileID, symbol types.Symbol) bool {
	for _, s := range symbols {
		if s.FileID == fileID && s.Name == symbol.Name && s.Type == symbol.Type {
			return true
		}
	}
	return false
}

func (si *SymbolIndex) FindDefinitions(symbolName string) []searchtypes.Result {
	si.mu.RLock()
	defer si.mu.RUnlock()

	var results []searchtypes.Result

	// Exact match
	if defs, ok := si.definitions[symbolName]; ok {
		for _, def := range defs {
			results = append(results, symbolToResult(def))
		}
	}

	// Fuzzy match if no exact results
	if len(results) == 0 {
		lowerSymbol := strings.ToLower(symbolName)
		for name, defs := range si.definitions {
			if strings.Contains(strings.ToLower(name), lowerSymbol) {
				for _, def := range defs {
					results = append(results, symbolToResult(def))
				}
			}
		}
	}

	return results
}

func (si *SymbolIndex) FindReferences(symbolName string) []searchtypes.Result {
	si.mu.RLock()
	defer si.mu.RUnlock()

	var results []searchtypes.Result

	// Include definitions as references
	if defs, ok := si.definitions[symbolName]; ok {
		for _, def := range defs {
			results = append(results, symbolToResult(def))
		}
	}

	// Add references
	if refs, ok := si.references[symbolName]; ok {
		for _, ref := range refs {
			results = append(results, symbolToResult(ref))
		}
	}

	return results
}

func isDefinition(symbolType types.SymbolType) bool {
	switch symbolType {
	case types.SymbolTypeFunction, types.SymbolTypeClass, types.SymbolTypeMethod,
		types.SymbolTypeConstant, types.SymbolTypeInterface, types.SymbolTypeType:
		return true
	default:
		return false
	}
}

func removeSymbolFromSlice(symbols []types.Symbol, fileID types.FileID) []types.Symbol {
	filtered := symbols[:0]
	for _, symbol := range symbols {
		if symbol.FileID != fileID {
			filtered = append(filtered, symbol)
		}
	}
	return filtered
}

// StoreSymbolMetrics stores metrics for a symbol
func (si *SymbolIndex) StoreSymbolMetrics(fileID types.FileID, symbolName string, metrics interface{}) {
	si.mu.Lock()
	defer si.mu.Unlock()

	if si.metrics[fileID] == nil {
		si.metrics[fileID] = make(map[string]interface{})
	}
	si.metrics[fileID][symbolName] = metrics
}

// GetSymbolMetrics retrieves metrics for a symbol
func (si *SymbolIndex) GetSymbolMetrics(fileID types.FileID, symbolName string) interface{} {
	si.mu.RLock()
	defer si.mu.RUnlock()

	if fileMetrics, ok := si.metrics[fileID]; ok {
		return fileMetrics[symbolName]
	}
	return nil
}

// GetAllMetrics returns all stored metrics for debugging
func (si *SymbolIndex) GetAllMetrics() map[types.FileID]map[string]interface{} {
	si.mu.RLock()
	defer si.mu.RUnlock()

	// Return a copy for debugging
	result := make(map[types.FileID]map[string]interface{})
	for fileID, fileMetrics := range si.metrics {
		result[fileID] = make(map[string]interface{})
		for symbolName, metrics := range fileMetrics {
			result[fileID][symbolName] = metrics
		}
	}
	return result
}

// GetFileMetrics retrieves all metrics for a file
func (si *SymbolIndex) GetFileMetrics(fileID types.FileID) map[string]interface{} {
	si.mu.RLock()
	defer si.mu.RUnlock()

	result := make(map[string]interface{})
	if fileMetrics, ok := si.metrics[fileID]; ok {
		for name, metrics := range fileMetrics {
			result[name] = metrics
		}
	}
	return result
}

// RemoveFileMetrics removes all metrics for a file
func (si *SymbolIndex) RemoveFileMetrics(fileID types.FileID) {
	si.mu.Lock()
	defer si.mu.Unlock()

	delete(si.metrics, fileID)
}

// Count returns the total number of symbols (definitions + references)
func (si *SymbolIndex) Count() int {
	si.mu.RLock()
	defer si.mu.RUnlock()

	count := 0
	for _, symbols := range si.definitions {
		count += len(symbols)
	}
	for _, symbols := range si.references {
		count += len(symbols)
	}
	return count
}

// DefinitionCount returns the number of symbol definitions
func (si *SymbolIndex) DefinitionCount() int {
	si.mu.RLock()
	defer si.mu.RUnlock()

	count := 0
	for _, symbols := range si.definitions {
		count += len(symbols)
	}
	return count
}

func symbolToResult(symbol types.Symbol) searchtypes.Result {
	// This is a simplified conversion - in real implementation,
	// we'd need to fetch the file content and extract context
	return searchtypes.Result{
		FileID: symbol.FileID,
		Line:   symbol.Line,
		Column: symbol.Column,
		Score:  10.0, // Symbol matches get high score
	}
}

// GetAllSymbolNames returns all unique symbol names from definitions
// This is efficient since the symbol index is small (deduped) compared to content
func (si *SymbolIndex) GetAllSymbolNames() []string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	names := make([]string, 0, len(si.definitions))
	for name := range si.definitions {
		names = append(names, name)
	}
	return names
}

// GetAllDefinitions returns all symbol definitions
// Used for semantic search which scores all symbols
func (si *SymbolIndex) GetAllDefinitions() map[string][]types.Symbol {
	si.mu.RLock()
	defer si.mu.RUnlock()

	// Return a copy to avoid concurrent modification issues
	result := make(map[string][]types.Symbol, len(si.definitions))
	for name, symbols := range si.definitions {
		symbolsCopy := make([]types.Symbol, len(symbols))
		copy(symbolsCopy, symbols)
		result[name] = symbolsCopy
	}
	return result
}

// GetAllFileIDs returns all FileIDs that have symbols indexed
func (si *SymbolIndex) GetAllFileIDs() []types.FileID {
	si.mu.RLock()
	defer si.mu.RUnlock()

	fileIDs := make([]types.FileID, 0, len(si.metrics))
	for fileID := range si.metrics {
		fileIDs = append(fileIDs, fileID)
	}
	return fileIDs
}

// GetFilePath returns the file path for a FileID
func (si *SymbolIndex) GetFilePath(fileID types.FileID) (string, bool) {
	si.mu.RLock()
	defer si.mu.RUnlock()

	// The SymbolIndex doesn't store file paths
	// This is a placeholder for compatibility
	return "", false
}

// updateStatsLocked recomputes statistics from current index state
// Call with lock held
func (si *SymbolIndex) updateStatsLocked() {
	// Reset stats
	si.stats = SymbolIndexStats{
		TypeDistribution: make(map[types.SymbolType]int),
	}

	// Collect all symbols
	var allSymbols []types.Symbol
	for _, symbols := range si.definitions {
		allSymbols = append(allSymbols, symbols...)
	}

	// Also include symbols from references (they might be defined elsewhere)
	for _, symbols := range si.references {
		allSymbols = append(allSymbols, symbols...)
	}

	if len(allSymbols) == 0 {
		return
	}

	si.stats.FileCount = len(si.metrics)
	symbolMap := make(map[string][]types.Symbol) // name -> symbols for scoring

	// Aggregate statistics
	for _, symbol := range allSymbols {
		// Count by type
		si.stats.TypeDistribution[symbol.Type]++
		si.stats.TotalSymbols++

		// Count by category
		switch symbol.Type {
		case types.SymbolTypeFunction:
			si.stats.TotalFunctions++
		case types.SymbolTypeMethod:
			si.stats.TotalMethods++
		case types.SymbolTypeType:
			si.stats.TotalTypes++
		case types.SymbolTypeVariable:
			si.stats.TotalVariables++
		}

		// Identify entry points
		if isEntryPoint(symbol) {
			si.stats.EntryPoints = append(si.stats.EntryPoints, symbol)
		}

		// Identify exported symbols
		if isExported(symbol.Name) {
			si.stats.ExportedSymbols = append(si.stats.ExportedSymbols, symbol)
		}

		// Track for top symbols by name
		symbolMap[symbol.Name] = append(symbolMap[symbol.Name], symbol)
	}

	// Build TopSymbols by frequency (number of references to each name)
	si.stats.TopSymbols = si.buildTopSymbolsFromMap(symbolMap, 100)
}

// isEntryPoint checks if symbol is an entry point
func isEntryPoint(symbol types.Symbol) bool {
	name := strings.ToLower(symbol.Name)
	// Go entry points
	if name == "main" || name == "init" {
		return true
	}
	// Exported functions (proxy for important functions)
	return isExported(symbol.Name) &&
		(symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeMethod)
}

// buildTopSymbolsFromMap converts name->symbols map to sorted slice
func (si *SymbolIndex) buildTopSymbolsFromMap(symbolMap map[string][]types.Symbol, limit int) []SymbolWithScore {
	if len(symbolMap) == 0 {
		return nil
	}

	// Convert to slice with scores
	symbols := make([]SymbolWithScore, 0, len(symbolMap))
	for _, symbolsList := range symbolMap {
		// Score is the number of times this symbol name appears
		score := len(symbolsList)
		// Use the first symbol with this name
		if len(symbolsList) > 0 {
			symbols = append(symbols, SymbolWithScore{
				Symbol: symbolsList[0], // Representative symbol
				Score:  score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Score > symbols[j].Score
	})

	// Limit
	if len(symbols) > limit {
		symbols = symbols[:limit]
	}

	return symbols
}

// UpdateStatsIncremental updates statistics incrementally without full recomputation
// This is much faster than updateStatsLocked and should be called for each file during indexing
// Call with lock held
func (si *SymbolIndex) UpdateStatsIncremental(fileID types.FileID, symbols []types.Symbol) {
	// Update file count (add if new file)
	if _, exists := si.metrics[fileID]; !exists {
		si.fileCount++
	}

	// Incrementally update counters
	for _, symbol := range symbols {
		// Count by type
		si.stats.TypeDistribution[symbol.Type]++
		si.stats.TotalSymbols++

		// Count by category
		switch symbol.Type {
		case types.SymbolTypeFunction:
			si.stats.TotalFunctions++
		case types.SymbolTypeMethod:
			si.stats.TotalMethods++
		case types.SymbolTypeType:
			si.stats.TotalTypes++
		case types.SymbolTypeVariable:
			si.stats.TotalVariables++
		}

		// Track entry points and exported symbols incrementally
		if isEntryPoint(symbol) {
			si.stats.EntryPoints = append(si.stats.EntryPoints, symbol)
		}

		if isExported(symbol.Name) {
			si.stats.ExportedSymbols = append(si.stats.ExportedSymbols, symbol)
		}

		// Increment symbol frequency (for TopSymbols)
		si.symbolFrequency[symbol.Name]++
	}

	// Update file count in stats
	si.stats.FileCount = si.fileCount
}

// FinalizeStats performs final optimization of statistics
// Should be called once after all files have been indexed
// Builds TopSymbols from the frequency map (expensive operation, done once)
func (si *SymbolIndex) FinalizeStats() {
	si.mu.Lock()
	defer si.mu.Unlock()

	// Build TopSymbols by frequency
	// Convert symbolFrequency map to []SymbolWithScore for sorting
	if len(si.symbolFrequency) > 0 {
		topSymbols := make([]SymbolWithScore, 0, len(si.symbolFrequency))
		for name, freq := range si.symbolFrequency {
			// Get a representative symbol for this name
			var representative types.Symbol
			if defs, ok := si.definitions[name]; ok && len(defs) > 0 {
				representative = defs[0]
			} else if refs, ok := si.references[name]; ok && len(refs) > 0 {
				representative = refs[0]
			} else {
				continue
			}

			topSymbols = append(topSymbols, SymbolWithScore{
				Symbol: representative,
				Score:  freq,
			})
		}

		// Sort by score descending
		sort.Slice(topSymbols, func(i, j int) bool {
			return topSymbols[i].Score > topSymbols[j].Score
		})

		// Limit to top 100
		if len(topSymbols) > 100 {
			topSymbols = topSymbols[:100]
		}

		si.stats.TopSymbols = topSymbols
	}
}

// ClearAll removes all symbols and resets statistics
// Call with lock held
func (si *SymbolIndex) ClearAll() {
	// Clear all data structures
	si.definitions = make(map[string][]types.Symbol)
	si.references = make(map[string][]types.Symbol)
	si.metrics = make(map[types.FileID]map[string]interface{})

	// Reset incremental tracking
	si.symbolFrequency = make(map[string]int)
	si.fileCount = 0

	// Reset statistics
	si.stats = SymbolIndexStats{
		TypeDistribution: make(map[types.SymbolType]int),
	}
}

// updateStatsForRemoval decrements statistics for removed symbols
// Call with lock held
func (si *SymbolIndex) updateStatsForRemoval(fileID types.FileID, symbols []types.Symbol) {
	// Decrement counters
	for _, symbol := range symbols {
		// Decrement type counts
		if count, ok := si.stats.TypeDistribution[symbol.Type]; ok && count > 0 {
			si.stats.TypeDistribution[symbol.Type]--
		}
		si.stats.TotalSymbols--

		// Decrement category counts
		switch symbol.Type {
		case types.SymbolTypeFunction:
			if si.stats.TotalFunctions > 0 {
				si.stats.TotalFunctions--
			}
		case types.SymbolTypeMethod:
			if si.stats.TotalMethods > 0 {
				si.stats.TotalMethods--
			}
		case types.SymbolTypeType:
			if si.stats.TotalTypes > 0 {
				si.stats.TotalTypes--
			}
		case types.SymbolTypeVariable:
			if si.stats.TotalVariables > 0 {
				si.stats.TotalVariables--
			}
		}

		// Decrement symbol frequency
		if count, ok := si.symbolFrequency[symbol.Name]; ok {
			if count <= 1 {
				delete(si.symbolFrequency, symbol.Name)
			} else {
				si.symbolFrequency[symbol.Name] = count - 1
			}
		}
	}

	// Note: We don't remove from EntryPoints/ExportedSymbols here
	// as that would require scanning those slices
	// They'll be cleaned up when FinalizeStats is called
}
