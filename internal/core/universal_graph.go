package core

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// UniversalSymbolGraph represents the Universal Symbol Graph that extends beyond
// function calls to include all symbol relationships across languages
type UniversalSymbolGraph struct {
	// Core symbol storage - maps CompositeSymbolID to UniversalSymbolNode
	nodes map[types.CompositeSymbolID]*types.UniversalSymbolNode

	// Relationship indexes for fast queries
	relationshipIndex map[types.RelationshipType]map[types.CompositeSymbolID][]types.CompositeSymbolID

	// Reverse relationship indexes for bidirectional queries
	reverseRelationshipIndex map[types.RelationshipType]map[types.CompositeSymbolID][]types.CompositeSymbolID

	// File co-location index - maps FileID to symbols in that file
	fileIndex map[types.FileID][]types.CompositeSymbolID

	// Name-based lookup index for quick symbol resolution
	nameIndex map[string][]types.CompositeSymbolID

	// Language-based index for cross-language queries
	languageIndex map[string][]types.CompositeSymbolID

	// Usage tracking for hot spot analysis
	usageTracking map[types.CompositeSymbolID]*types.SymbolUsage

	// Statistics for performance monitoring
	stats UniversalGraphStats

	// Memory management
	maxNodes    int
	accessOrder []types.CompositeSymbolID       // For LRU eviction
	accessMap   map[types.CompositeSymbolID]int // Maps to index in accessOrder

	// Thread safety
	mu sync.RWMutex
}

// UniversalGraphStats tracks statistics about the Universal Symbol Graph
type UniversalGraphStats struct {
	TotalNodes        int            `json:"total_nodes"`
	NodesByKind       map[string]int `json:"nodes_by_kind"`
	NodesByLanguage   map[string]int `json:"nodes_by_language"`
	RelationshipCount map[string]int `json:"relationship_count"`
	LastUpdated       time.Time      `json:"last_updated"`
	MemoryUsage       int64          `json:"memory_usage_bytes"`
	QueryCount        int64          `json:"query_count"`
	AvgQueryTime      time.Duration  `json:"avg_query_time"`
}

const (
	// DefaultMaxNodes is the default maximum number of nodes in the Universal Symbol Graph
	DefaultMaxNodes = 100000 // 100k nodes should handle most large codebases
)

// NewUniversalSymbolGraph creates a new Universal Symbol Graph
func NewUniversalSymbolGraph() *UniversalSymbolGraph {
	return NewUniversalSymbolGraphWithConfig(DefaultMaxNodes)
}

// NewUniversalSymbolGraphWithConfig creates a new Universal Symbol Graph with custom configuration
func NewUniversalSymbolGraphWithConfig(maxNodes int) *UniversalSymbolGraph {
	return &UniversalSymbolGraph{
		nodes:                    make(map[types.CompositeSymbolID]*types.UniversalSymbolNode),
		relationshipIndex:        make(map[types.RelationshipType]map[types.CompositeSymbolID][]types.CompositeSymbolID),
		reverseRelationshipIndex: make(map[types.RelationshipType]map[types.CompositeSymbolID][]types.CompositeSymbolID),
		fileIndex:                make(map[types.FileID][]types.CompositeSymbolID),
		nameIndex:                make(map[string][]types.CompositeSymbolID),
		languageIndex:            make(map[string][]types.CompositeSymbolID),
		usageTracking:            make(map[types.CompositeSymbolID]*types.SymbolUsage),
		maxNodes:                 maxNodes,
		accessOrder:              make([]types.CompositeSymbolID, 0, maxNodes),
		accessMap:                make(map[types.CompositeSymbolID]int),
		stats: UniversalGraphStats{
			NodesByKind:       make(map[string]int),
			NodesByLanguage:   make(map[string]int),
			RelationshipCount: make(map[string]int),
			LastUpdated:       time.Now(),
		},
	}
}

// AddSymbol adds a symbol to the Universal Symbol Graph
func (usg *UniversalSymbolGraph) AddSymbol(node *types.UniversalSymbolNode) error {
	usg.mu.Lock()
	defer usg.mu.Unlock()

	if node == nil {
		return errors.New("cannot add nil symbol node")
	}

	if !node.Identity.ID.IsValid() {
		return fmt.Errorf("invalid symbol ID: %v", node.Identity.ID)
	}

	// Check if we need to evict old nodes
	if len(usg.nodes) >= usg.maxNodes {
		if err := usg.evictLRU(); err != nil {
			return fmt.Errorf("failed to evict LRU node: %w", err)
		}
	}

	// Store the symbol
	usg.nodes[node.Identity.ID] = node

	// Update LRU tracking
	usg.updateLRU(node.Identity.ID)

	// Update indexes
	usg.updateIndexes(node)

	// Update statistics
	usg.updateStats(node, "add")

	return nil
}

// GetSymbol retrieves a symbol by its CompositeSymbolID
func (usg *UniversalSymbolGraph) GetSymbol(id types.CompositeSymbolID) (*types.UniversalSymbolNode, bool) {
	usg.mu.Lock() // Use Lock instead of RLock for LRU updates
	defer usg.mu.Unlock()

	node, exists := usg.nodes[id]
	if exists {
		// Update LRU tracking for accessed symbol
		usg.updateLRU(id)

		// Update query statistics
		usg.stats.QueryCount++
	}

	return node, exists
}

// GetSymbolsByName finds symbols by name
func (usg *UniversalSymbolGraph) GetSymbolsByName(name string) []*types.UniversalSymbolNode {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	ids, exists := usg.nameIndex[name]
	if !exists {
		return nil
	}

	symbols := make([]*types.UniversalSymbolNode, 0, len(ids))
	for _, id := range ids {
		if node, exists := usg.nodes[id]; exists {
			symbols = append(symbols, node)
		}
	}

	return symbols
}

// GetAllSymbols returns all symbols in the Universal Symbol Graph
func (usg *UniversalSymbolGraph) GetAllSymbols() []*types.UniversalSymbolNode {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	symbols := make([]*types.UniversalSymbolNode, 0, len(usg.nodes))
	for _, node := range usg.nodes {
		symbols = append(symbols, node)
	}

	return symbols
}

// GetSymbolsByFile returns all symbols in a specific file
func (usg *UniversalSymbolGraph) GetSymbolsByFile(fileID types.FileID) []*types.UniversalSymbolNode {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	ids, exists := usg.fileIndex[fileID]
	if !exists {
		return nil
	}

	symbols := make([]*types.UniversalSymbolNode, 0, len(ids))
	for _, id := range ids {
		if node, exists := usg.nodes[id]; exists {
			symbols = append(symbols, node)
		}
	}

	return symbols
}

// GetSymbolsByLanguage returns all symbols for a specific language
func (usg *UniversalSymbolGraph) GetSymbolsByLanguage(language string) []*types.UniversalSymbolNode {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	ids, exists := usg.languageIndex[language]
	if !exists {
		return nil
	}

	symbols := make([]*types.UniversalSymbolNode, 0, len(ids))
	for _, id := range ids {
		if node, exists := usg.nodes[id]; exists {
			symbols = append(symbols, node)
		}
	}

	return symbols
}

// GetRelatedSymbols finds symbols related to the given symbol by relationship type
func (usg *UniversalSymbolGraph) GetRelatedSymbols(id types.CompositeSymbolID, relationType types.RelationshipType) ([]*types.UniversalSymbolNode, error) {
	start := time.Now()

	usg.mu.RLock()
	relationMap, exists := usg.relationshipIndex[relationType]
	if !exists {
		usg.mu.RUnlock()
		// Update stats after releasing lock
		usg.mu.Lock()
		usg.stats.QueryCount++
		usg.stats.AvgQueryTime = (usg.stats.AvgQueryTime + time.Since(start)) / 2
		usg.mu.Unlock()
		return nil, fmt.Errorf("relationship type %s not found", relationType.String())
	}

	relatedIds, exists := relationMap[id]
	if !exists {
		usg.mu.RUnlock()
		// Update stats after releasing lock
		usg.mu.Lock()
		usg.stats.QueryCount++
		usg.stats.AvgQueryTime = (usg.stats.AvgQueryTime + time.Since(start)) / 2
		usg.mu.Unlock()
		return []*types.UniversalSymbolNode{}, nil
	}

	symbols := make([]*types.UniversalSymbolNode, 0, len(relatedIds))
	for _, relatedId := range relatedIds {
		if node, exists := usg.nodes[relatedId]; exists {
			symbols = append(symbols, node)
		}
	}
	usg.mu.RUnlock()

	// Update stats after releasing lock
	usg.mu.Lock()
	usg.stats.QueryCount++
	usg.stats.AvgQueryTime = (usg.stats.AvgQueryTime + time.Since(start)) / 2
	usg.mu.Unlock()

	return symbols, nil
}

// GetReverseRelatedSymbols finds symbols that have the given relationship TO the specified symbol
func (usg *UniversalSymbolGraph) GetReverseRelatedSymbols(id types.CompositeSymbolID, relationType types.RelationshipType) ([]*types.UniversalSymbolNode, error) {
	start := time.Now()

	usg.mu.RLock()
	reverseRelationMap, exists := usg.reverseRelationshipIndex[relationType]
	if !exists {
		usg.mu.RUnlock()
		// Update stats after releasing lock
		usg.mu.Lock()
		usg.stats.QueryCount++
		usg.stats.AvgQueryTime = (usg.stats.AvgQueryTime + time.Since(start)) / 2
		usg.mu.Unlock()
		return nil, fmt.Errorf("reverse relationship type %s not found", relationType.String())
	}

	relatedIds, exists := reverseRelationMap[id]
	if !exists {
		usg.mu.RUnlock()
		// Update stats after releasing lock
		usg.mu.Lock()
		usg.stats.QueryCount++
		usg.stats.AvgQueryTime = (usg.stats.AvgQueryTime + time.Since(start)) / 2
		usg.mu.Unlock()
		return []*types.UniversalSymbolNode{}, nil
	}

	symbols := make([]*types.UniversalSymbolNode, 0, len(relatedIds))
	for _, relatedId := range relatedIds {
		if node, exists := usg.nodes[relatedId]; exists {
			symbols = append(symbols, node)
		}
	}
	usg.mu.RUnlock()

	// Update stats after releasing lock
	usg.mu.Lock()
	usg.stats.QueryCount++
	usg.stats.AvgQueryTime = (usg.stats.AvgQueryTime + time.Since(start)) / 2
	usg.mu.Unlock()

	return symbols, nil
}

// BuildRelationshipTree builds a tree of relationships starting from a symbol
func (usg *UniversalSymbolGraph) BuildRelationshipTree(rootID types.CompositeSymbolID, relationTypes []types.RelationshipType, maxDepth int) (*RelationshipTree, error) {
	usg.mu.RLock()

	rootNode, exists := usg.nodes[rootID]
	if !exists {
		usg.mu.RUnlock()
		return nil, fmt.Errorf("root symbol not found: %v", rootID)
	}

	tree := &RelationshipTree{
		RootSymbol: rootNode,
		MaxDepth:   maxDepth,
		Relations:  relationTypes,
	}

	// Build the tree recursively (with lock held for entire operation)
	visited := make(map[types.CompositeSymbolID]bool)
	tree.Root = usg.buildRelationshipNodeLocked(rootNode, relationTypes, 0, maxDepth, visited)

	usg.mu.RUnlock()
	return tree, nil
}

// GetFileCoLocatedSymbols returns symbols that are co-located in the same file
func (usg *UniversalSymbolGraph) GetFileCoLocatedSymbols(id types.CompositeSymbolID) ([]*types.UniversalSymbolNode, error) {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	node, exists := usg.nodes[id]
	if !exists {
		return nil, fmt.Errorf("symbol not found: %v", id)
	}

	fileID := node.Identity.Location.FileID
	allSymbolsInFile, exists := usg.fileIndex[fileID]
	if !exists {
		return []*types.UniversalSymbolNode{}, nil
	}

	// Filter out the original symbol itself
	coLocatedSymbols := make([]*types.UniversalSymbolNode, 0)
	for _, symbolID := range allSymbolsInFile {
		if !symbolID.Equals(id) {
			if symbol, exists := usg.nodes[symbolID]; exists {
				coLocatedSymbols = append(coLocatedSymbols, symbol)
			}
		}
	}

	return coLocatedSymbols, nil
}

// GetUsageHotSpots returns symbols with high usage patterns
func (usg *UniversalSymbolGraph) GetUsageHotSpots(limit int) []*types.UniversalSymbolNode {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	// Create a slice of symbol-usage pairs for sorting
	type symbolUsagePair struct {
		symbol *types.UniversalSymbolNode
		usage  *types.SymbolUsage
	}

	pairs := make([]symbolUsagePair, 0, len(usg.usageTracking))
	for id, usage := range usg.usageTracking {
		if symbol, exists := usg.nodes[id]; exists {
			pairs = append(pairs, symbolUsagePair{symbol, usage})
		}
	}

	// Sort by reference count descending
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].usage.ReferenceCount > pairs[j].usage.ReferenceCount
	})

	// Return top symbols up to limit
	if limit > len(pairs) {
		limit = len(pairs)
	}

	hotSpots := make([]*types.UniversalSymbolNode, limit)
	for i := 0; i < limit; i++ {
		hotSpots[i] = pairs[i].symbol
	}

	return hotSpots
}

// UpdateUsage updates usage statistics for a symbol
func (usg *UniversalSymbolGraph) UpdateUsage(id types.CompositeSymbolID, usageType string, location types.SymbolLocation) error {
	usg.mu.Lock()
	defer usg.mu.Unlock()

	usage, exists := usg.usageTracking[id]
	if !exists {
		usage = &types.SymbolUsage{
			ReferencingFiles: []types.FileID{},
			HotSpots:         []types.UsageHotSpot{},
			FirstSeen:        time.Now(),
		}
		usg.usageTracking[id] = usage
	}

	// Update counts based on usage type
	switch usageType {
	case "reference":
		usage.ReferenceCount++
	case "import":
		usage.ImportCount++
	case "inheritance":
		usage.InheritanceCount++
	case "call":
		usage.CallCount++
	case "modification":
		usage.ModificationCount++
	}

	// Update last referenced time
	usage.LastReferenced = time.Now()

	// Add referencing file if not already present
	fileID := location.FileID
	found := false
	for _, existingFileID := range usage.ReferencingFiles {
		if existingFileID == fileID {
			found = true
			break
		}
	}
	if !found {
		usage.ReferencingFiles = append(usage.ReferencingFiles, fileID)
	}

	// Update symbol's usage in the node
	if node, exists := usg.nodes[id]; exists {
		node.Usage = *usage
	}

	return nil
}

// RemoveSymbol removes a symbol from the Universal Symbol Graph
func (usg *UniversalSymbolGraph) RemoveSymbol(id types.CompositeSymbolID) error {
	usg.mu.Lock()
	defer usg.mu.Unlock()

	node, exists := usg.nodes[id]
	if !exists {
		return fmt.Errorf("symbol not found: %v", id)
	}

	// Remove from main storage
	delete(usg.nodes, id)

	// Remove from indexes
	usg.removeFromIndexes(node)

	// Remove from usage tracking
	delete(usg.usageTracking, id)

	// Update statistics
	usg.updateStats(node, "remove")

	return nil
}

// Clear clears all symbols and indexes
func (usg *UniversalSymbolGraph) Clear() {
	usg.mu.Lock()
	defer usg.mu.Unlock()

	usg.nodes = make(map[types.CompositeSymbolID]*types.UniversalSymbolNode)
	usg.relationshipIndex = make(map[types.RelationshipType]map[types.CompositeSymbolID][]types.CompositeSymbolID)
	usg.reverseRelationshipIndex = make(map[types.RelationshipType]map[types.CompositeSymbolID][]types.CompositeSymbolID)
	usg.fileIndex = make(map[types.FileID][]types.CompositeSymbolID)
	usg.nameIndex = make(map[string][]types.CompositeSymbolID)
	usg.languageIndex = make(map[string][]types.CompositeSymbolID)
	usg.usageTracking = make(map[types.CompositeSymbolID]*types.SymbolUsage)

	usg.stats = UniversalGraphStats{
		NodesByKind:       make(map[string]int),
		NodesByLanguage:   make(map[string]int),
		RelationshipCount: make(map[string]int),
		LastUpdated:       time.Now(),
	}
}

// GetStats returns statistics about the Universal Symbol Graph
func (usg *UniversalSymbolGraph) GetStats() UniversalGraphStats {
	usg.mu.RLock()
	defer usg.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := usg.stats
	stats.TotalNodes = len(usg.nodes)

	return stats
}

// Private helper methods

func (usg *UniversalSymbolGraph) updateIndexes(node *types.UniversalSymbolNode) {
	id := node.Identity.ID

	// Update name index
	name := node.Identity.Name
	if _, exists := usg.nameIndex[name]; !exists {
		usg.nameIndex[name] = []types.CompositeSymbolID{}
	}
	usg.nameIndex[name] = append(usg.nameIndex[name], id)

	// Update file index
	fileID := node.Identity.Location.FileID
	if _, exists := usg.fileIndex[fileID]; !exists {
		usg.fileIndex[fileID] = []types.CompositeSymbolID{}
	}
	usg.fileIndex[fileID] = append(usg.fileIndex[fileID], id)

	// Update language index
	language := node.Identity.Language
	if _, exists := usg.languageIndex[language]; !exists {
		usg.languageIndex[language] = []types.CompositeSymbolID{}
	}
	usg.languageIndex[language] = append(usg.languageIndex[language], id)

	// Update relationship indexes
	usg.updateRelationshipIndexes(id, &node.Relationships)
}

func (usg *UniversalSymbolGraph) updateRelationshipIndexes(id types.CompositeSymbolID, relationships *types.SymbolRelationships) {
	// Update Extends relationships
	usg.updateRelationshipIndex(types.RelationExtends, id, relationships.Extends)

	// Update Implements relationships
	usg.updateRelationshipIndex(types.RelationImplements, id, relationships.Implements)

	// Update Contains relationships
	usg.updateRelationshipIndex(types.RelationContains, id, relationships.Contains)

	// Update Dependencies
	depIds := make([]types.CompositeSymbolID, len(relationships.Dependencies))
	for i, dep := range relationships.Dependencies {
		depIds[i] = dep.Target
	}
	usg.updateRelationshipIndex(types.RelationDependsOn, id, depIds)

	// Update Calls relationships
	callIds := make([]types.CompositeSymbolID, len(relationships.CallsTo))
	for i, call := range relationships.CallsTo {
		callIds[i] = call.Target
	}
	usg.updateRelationshipIndex(types.RelationCalls, id, callIds)

	// Update file co-location relationships
	usg.updateRelationshipIndex(types.RelationFileCoLocated, id, relationships.FileCoLocated)
}

func (usg *UniversalSymbolGraph) updateRelationshipIndex(relationType types.RelationshipType, sourceID types.CompositeSymbolID, targetIDs []types.CompositeSymbolID) {
	// Initialize maps if they don't exist
	if _, exists := usg.relationshipIndex[relationType]; !exists {
		usg.relationshipIndex[relationType] = make(map[types.CompositeSymbolID][]types.CompositeSymbolID)
	}
	if _, exists := usg.reverseRelationshipIndex[relationType]; !exists {
		usg.reverseRelationshipIndex[relationType] = make(map[types.CompositeSymbolID][]types.CompositeSymbolID)
	}

	// Update forward index
	usg.relationshipIndex[relationType][sourceID] = append(usg.relationshipIndex[relationType][sourceID], targetIDs...)

	// Update reverse index
	for _, targetID := range targetIDs {
		usg.reverseRelationshipIndex[relationType][targetID] = append(usg.reverseRelationshipIndex[relationType][targetID], sourceID)
	}
}

func (usg *UniversalSymbolGraph) removeFromIndexes(node *types.UniversalSymbolNode) {
	id := node.Identity.ID

	// Remove from name index
	name := node.Identity.Name
	if ids, exists := usg.nameIndex[name]; exists {
		filtered := []types.CompositeSymbolID{}
		for _, existingID := range ids {
			if !existingID.Equals(id) {
				filtered = append(filtered, existingID)
			}
		}
		if len(filtered) == 0 {
			delete(usg.nameIndex, name)
		} else {
			usg.nameIndex[name] = filtered
		}
	}

	// Remove from file index
	fileID := node.Identity.Location.FileID
	if ids, exists := usg.fileIndex[fileID]; exists {
		filtered := []types.CompositeSymbolID{}
		for _, existingID := range ids {
			if !existingID.Equals(id) {
				filtered = append(filtered, existingID)
			}
		}
		if len(filtered) == 0 {
			delete(usg.fileIndex, fileID)
		} else {
			usg.fileIndex[fileID] = filtered
		}
	}

	// Remove from language index
	language := node.Identity.Language
	if ids, exists := usg.languageIndex[language]; exists {
		filtered := []types.CompositeSymbolID{}
		for _, existingID := range ids {
			if !existingID.Equals(id) {
				filtered = append(filtered, existingID)
			}
		}
		if len(filtered) == 0 {
			delete(usg.languageIndex, language)
		} else {
			usg.languageIndex[language] = filtered
		}
	}

	// Remove from relationship indexes
	usg.removeFromRelationshipIndexes(id)

	// Remove from usage tracking
	delete(usg.usageTracking, id)

	// Update statistics
	usg.updateStats(node, "remove")
}

// removeFromRelationshipIndexes removes a symbol from all relationship indexes
func (usg *UniversalSymbolGraph) removeFromRelationshipIndexes(id types.CompositeSymbolID) {
	// Remove from all forward relationship indexes
	for _, index := range usg.relationshipIndex {
		// Remove this symbol as a source
		delete(index, id)

		// Remove this symbol as a target from other symbols' relationships
		for sourceID, targets := range index {
			filtered := []types.CompositeSymbolID{}
			for _, targetID := range targets {
				if !targetID.Equals(id) {
					filtered = append(filtered, targetID)
				}
			}
			if len(filtered) == 0 {
				delete(index, sourceID)
			} else {
				index[sourceID] = filtered
			}
		}
	}

	// Remove from all reverse relationship indexes
	for _, index := range usg.reverseRelationshipIndex {
		// Remove this symbol as a target
		delete(index, id)

		// Remove this symbol as a source from other symbols' reverse relationships
		for targetID, sources := range index {
			filtered := []types.CompositeSymbolID{}
			for _, sourceID := range sources {
				if !sourceID.Equals(id) {
					filtered = append(filtered, sourceID)
				}
			}
			if len(filtered) == 0 {
				delete(index, targetID)
			} else {
				index[targetID] = filtered
			}
		}
	}
}

// updateLRU updates the LRU access tracking for a symbol
func (usg *UniversalSymbolGraph) updateLRU(id types.CompositeSymbolID) {
	// If already in access order, move to front
	if idx, exists := usg.accessMap[id]; exists {
		// Remove from current position
		usg.accessOrder = append(usg.accessOrder[:idx], usg.accessOrder[idx+1:]...)
		// Update indices in accessMap for shifted elements
		for i := idx; i < len(usg.accessOrder); i++ {
			usg.accessMap[usg.accessOrder[i]] = i
		}
	}

	// Add to front (most recently used)
	usg.accessOrder = append([]types.CompositeSymbolID{id}, usg.accessOrder...)
	usg.accessMap[id] = 0

	// Update indices for all other elements
	for i := 1; i < len(usg.accessOrder); i++ {
		usg.accessMap[usg.accessOrder[i]] = i
	}
}

// evictLRU removes the least recently used symbol to make room for new ones
func (usg *UniversalSymbolGraph) evictLRU() error {
	if len(usg.accessOrder) == 0 {
		return errors.New("no symbols to evict")
	}

	// Get least recently used (last in order)
	lruID := usg.accessOrder[len(usg.accessOrder)-1]

	// Remove it
	return usg.removeSymbolInternal(lruID)
}

// removeSymbolInternal is the internal removal method used by evictLRU
func (usg *UniversalSymbolGraph) removeSymbolInternal(id types.CompositeSymbolID) error {
	node, exists := usg.nodes[id]
	if !exists {
		return fmt.Errorf("symbol %v not found", id)
	}

	// Remove from main storage
	delete(usg.nodes, id)

	// Remove from LRU tracking
	if idx, exists := usg.accessMap[id]; exists {
		usg.accessOrder = append(usg.accessOrder[:idx], usg.accessOrder[idx+1:]...)
		delete(usg.accessMap, id)
		// Update indices for shifted elements
		for i := idx; i < len(usg.accessOrder); i++ {
			usg.accessMap[usg.accessOrder[i]] = i
		}
	}

	// Remove from name index
	name := node.Identity.Name
	if ids, exists := usg.nameIndex[name]; exists {
		filtered := []types.CompositeSymbolID{}
		for _, existingID := range ids {
			if !existingID.Equals(id) {
				filtered = append(filtered, existingID)
			}
		}
		if len(filtered) == 0 {
			delete(usg.nameIndex, name)
		} else {
			usg.nameIndex[name] = filtered
		}
	}

	// Remove from file index
	fileID := node.Identity.Location.FileID
	if ids, exists := usg.fileIndex[fileID]; exists {
		filtered := []types.CompositeSymbolID{}
		for _, existingID := range ids {
			if !existingID.Equals(id) {
				filtered = append(filtered, existingID)
			}
		}
		if len(filtered) == 0 {
			delete(usg.fileIndex, fileID)
		} else {
			usg.fileIndex[fileID] = filtered
		}
	}

	// Remove from language index
	language := node.Identity.Language
	if ids, exists := usg.languageIndex[language]; exists {
		filtered := []types.CompositeSymbolID{}
		for _, existingID := range ids {
			if !existingID.Equals(id) {
				filtered = append(filtered, existingID)
			}
		}
		if len(filtered) == 0 {
			delete(usg.languageIndex, language)
		} else {
			usg.languageIndex[language] = filtered
		}
	}

	// Remove from relationship indexes
	usg.removeFromRelationshipIndexes(id)

	// Remove from usage tracking
	delete(usg.usageTracking, id)

	// Update statistics
	usg.updateStats(node, "remove")

	return nil
}

func (usg *UniversalSymbolGraph) updateStats(node *types.UniversalSymbolNode, operation string) {
	kind := node.Identity.Kind.String()
	language := node.Identity.Language

	switch operation {
	case "add":
		usg.stats.NodesByKind[kind]++
		usg.stats.NodesByLanguage[language]++
	case "remove":
		usg.stats.NodesByKind[kind]--
		usg.stats.NodesByLanguage[language]--
		if usg.stats.NodesByKind[kind] <= 0 {
			delete(usg.stats.NodesByKind, kind)
		}
		if usg.stats.NodesByLanguage[language] <= 0 {
			delete(usg.stats.NodesByLanguage, language)
		}
	}

	usg.stats.LastUpdated = time.Now()
}

// buildRelationshipNodeLocked builds relationship nodes assuming the lock is already held
func (usg *UniversalSymbolGraph) buildRelationshipNodeLocked(node *types.UniversalSymbolNode, relationTypes []types.RelationshipType, depth int, maxDepth int, visited map[types.CompositeSymbolID]bool) *RelationshipTreeNode {
	if depth > maxDepth || visited[node.Identity.ID] {
		return nil
	}

	visited[node.Identity.ID] = true
	defer func() { visited[node.Identity.ID] = false }()

	treeNode := &RelationshipTreeNode{
		Symbol:   node,
		Depth:    depth,
		Children: make(map[types.RelationshipType][]*RelationshipTreeNode),
	}

	// Build children for each requested relationship type
	for _, relationType := range relationTypes {
		// Use internal method that doesn't acquire locks
		if relatedSymbols := usg.getRelatedSymbolsLocked(node.Identity.ID, relationType); relatedSymbols != nil {
			for _, relatedSymbol := range relatedSymbols {
				if childNode := usg.buildRelationshipNodeLocked(relatedSymbol, relationTypes, depth+1, maxDepth, visited); childNode != nil {
					treeNode.Children[relationType] = append(treeNode.Children[relationType], childNode)
				}
			}
		}
	}

	return treeNode
}

// getRelatedSymbolsLocked is an internal method that assumes the lock is already held
func (usg *UniversalSymbolGraph) getRelatedSymbolsLocked(id types.CompositeSymbolID, relationType types.RelationshipType) []*types.UniversalSymbolNode {
	relationMap, exists := usg.relationshipIndex[relationType]
	if !exists {
		return nil
	}

	relatedIds, exists := relationMap[id]
	if !exists {
		return []*types.UniversalSymbolNode{}
	}

	symbols := make([]*types.UniversalSymbolNode, 0, len(relatedIds))
	for _, relatedId := range relatedIds {
		if node, exists := usg.nodes[relatedId]; exists {
			symbols = append(symbols, node)
		}
	}

	return symbols
}

// RelationshipTree represents a tree of symbol relationships
type RelationshipTree struct {
	RootSymbol *types.UniversalSymbolNode `json:"root_symbol"`
	Root       *RelationshipTreeNode      `json:"root"`
	MaxDepth   int                        `json:"max_depth"`
	Relations  []types.RelationshipType   `json:"relations"`
}

// RelationshipTreeNode represents a node in a relationship tree
type RelationshipTreeNode struct {
	Symbol   *types.UniversalSymbolNode                         `json:"symbol"`
	Depth    int                                                `json:"depth"`
	Children map[types.RelationshipType][]*RelationshipTreeNode `json:"children"`
}
