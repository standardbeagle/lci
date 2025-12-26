package symbollinker

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// IncrementalEngine extends the SymbolLinkerEngine with incremental update capabilities
type IncrementalEngine struct {
	*SymbolLinkerEngine

	// File tracking for incremental updates
	fileHashes     map[types.FileID][32]byte       // File content hashes
	fileDependents map[types.FileID][]types.FileID // Files that depend on this file
	fileTimestamps map[types.FileID]time.Time      // Last update timestamps

	// Dependency graph for efficient updates
	importGraph map[types.FileID][]types.FileID // FileID -> imported files
	exportedBy  map[types.FileID][]types.FileID // FileID -> files that export to it

	// Update tracking
	pendingUpdates map[types.FileID]UpdateInfo // Files pending update
	updateBatch    []types.FileID              // Current update batch

	// Thread safety for incremental operations
	incrementalMutex sync.RWMutex
}

// UpdateInfo tracks information about a pending file update
type UpdateInfo struct {
	FileID     types.FileID
	NewContent []byte
	NewHash    [32]byte
	Timestamp  time.Time
	UpdateType UpdateType
	AffectedBy []types.FileID // Files that caused this update (for cascading)
}

// UpdateType represents the type of incremental update
type UpdateType int

const (
	UpdateTypeModified UpdateType = iota // File was modified
	UpdateTypeAdded                      // File was added
	UpdateTypeRemoved                    // File was removed
	UpdateTypeCascade                    // Update triggered by dependency change
)

// String returns a string representation of the update type
func (ut UpdateType) String() string {
	switch ut {
	case UpdateTypeModified:
		return "modified"
	case UpdateTypeAdded:
		return "added"
	case UpdateTypeRemoved:
		return "removed"
	case UpdateTypeCascade:
		return "cascade"
	default:
		return "unknown"
	}
}

// UpdateResult contains information about an incremental update
type UpdateResult struct {
	UpdatedFiles   []types.FileID
	AffectedFiles  []types.FileID
	RemovedSymbols []types.CompositeSymbolID
	AddedSymbols   []types.CompositeSymbolID
	ModifiedLinks  int
	UpdateDuration time.Duration
	CascadeDepth   int
}

// NewIncrementalEngine creates a new incremental symbol linker engine
func NewIncrementalEngine(rootPath string) *IncrementalEngine {
	base := NewSymbolLinkerEngine(rootPath)

	return &IncrementalEngine{
		SymbolLinkerEngine: base,
		fileHashes:         make(map[types.FileID][32]byte),
		fileDependents:     make(map[types.FileID][]types.FileID),
		fileTimestamps:     make(map[types.FileID]time.Time),
		importGraph:        make(map[types.FileID][]types.FileID),
		exportedBy:         make(map[types.FileID][]types.FileID),
		pendingUpdates:     make(map[types.FileID]UpdateInfo),
		updateBatch:        make([]types.FileID, 0),
	}
}

// LinkSymbols overrides the base LinkSymbols to also update dependency graph
func (ie *IncrementalEngine) LinkSymbols() error {
	ie.incrementalMutex.Lock()
	defer ie.incrementalMutex.Unlock()

	// Call the base implementation
	if err := ie.SymbolLinkerEngine.LinkSymbols(); err != nil {
		return err
	}

	// Update dependency graph for all files
	for fileID := range ie.symbolTables {
		ie.updateDependencyGraph(fileID)
	}

	return nil
}

// UpdateFile performs an incremental update of a single file
func (ie *IncrementalEngine) UpdateFile(path string, content []byte) (*UpdateResult, error) {
	startTime := time.Now()

	ie.incrementalMutex.Lock()
	defer ie.incrementalMutex.Unlock()

	fileID := ie.GetOrCreateFileID(path)
	newHash := sha256.Sum256(content)

	// Check if file actually changed
	if oldHash, exists := ie.fileHashes[fileID]; exists {
		if oldHash == newHash {
			// File unchanged, return empty result
			return &UpdateResult{
				UpdateDuration: time.Since(startTime),
			}, nil
		}
	}

	// Determine update type
	updateType := UpdateTypeModified
	if _, exists := ie.fileHashes[fileID]; !exists {
		updateType = UpdateTypeAdded
	}

	// Schedule update
	updateInfo := UpdateInfo{
		FileID:     fileID,
		NewContent: content,
		NewHash:    newHash,
		Timestamp:  time.Now(),
		UpdateType: updateType,
	}

	ie.pendingUpdates[fileID] = updateInfo

	// Process the update
	return ie.processPendingUpdates(startTime)
}

// RemoveFile performs an incremental removal of a file
func (ie *IncrementalEngine) RemoveFile(path string) (*UpdateResult, error) {
	startTime := time.Now()

	ie.incrementalMutex.Lock()
	defer ie.incrementalMutex.Unlock()

	fileID, exists := ie.fileRegistry[path]
	if !exists {
		// File not in registry, nothing to remove
		return &UpdateResult{
			UpdateDuration: time.Since(startTime),
		}, nil
	}

	// Schedule removal
	updateInfo := UpdateInfo{
		FileID:     fileID,
		Timestamp:  time.Now(),
		UpdateType: UpdateTypeRemoved,
	}

	ie.pendingUpdates[fileID] = updateInfo

	// Process the update
	return ie.processPendingUpdates(startTime)
}

// BatchUpdate performs incremental updates on multiple files
func (ie *IncrementalEngine) BatchUpdate(updates map[string][]byte) (*UpdateResult, error) {
	startTime := time.Now()

	ie.incrementalMutex.Lock()
	defer ie.incrementalMutex.Unlock()

	// Schedule all updates
	for path, content := range updates {
		fileID := ie.GetOrCreateFileID(path)
		newHash := sha256.Sum256(content)

		// Check if file actually changed
		if oldHash, exists := ie.fileHashes[fileID]; exists {
			if oldHash == newHash {
				continue // Skip unchanged files
			}
		}

		updateType := UpdateTypeModified
		if _, exists := ie.fileHashes[fileID]; !exists {
			updateType = UpdateTypeAdded
		}

		updateInfo := UpdateInfo{
			FileID:     fileID,
			NewContent: content,
			NewHash:    newHash,
			Timestamp:  time.Now(),
			UpdateType: updateType,
		}

		ie.pendingUpdates[fileID] = updateInfo
	}

	// Process all updates
	return ie.processPendingUpdates(startTime)
}

// processPendingUpdates processes all pending updates and their cascading effects
func (ie *IncrementalEngine) processPendingUpdates(startTime time.Time) (*UpdateResult, error) {
	result := &UpdateResult{
		UpdatedFiles:   make([]types.FileID, 0),
		AffectedFiles:  make([]types.FileID, 0),
		RemovedSymbols: make([]types.CompositeSymbolID, 0),
		AddedSymbols:   make([]types.CompositeSymbolID, 0),
	}

	cascadeDepth := 0

	// Process updates in cascading waves
	for len(ie.pendingUpdates) > 0 {
		currentBatch := make([]types.FileID, 0, len(ie.pendingUpdates))
		for fileID := range ie.pendingUpdates {
			currentBatch = append(currentBatch, fileID)
		}

		// Process current batch
		cascadeUpdates := make(map[types.FileID]UpdateInfo)

		for _, fileID := range currentBatch {
			updateInfo := ie.pendingUpdates[fileID]
			delete(ie.pendingUpdates, fileID)

			if err := ie.processFileUpdate(updateInfo, result); err != nil {
				return result, fmt.Errorf("failed to process update for file %d: %w", fileID, err)
			}

			// Find files that need cascade updates
			ie.findCascadeUpdates(fileID, updateInfo, cascadeUpdates)
		}

		// Add cascade updates to pending
		for fileID, updateInfo := range cascadeUpdates {
			ie.pendingUpdates[fileID] = updateInfo
		}

		cascadeDepth++
		if cascadeDepth > 10 {
			return result, errors.New("cascade update depth exceeded limit (possible circular dependency)")
		}
	}

	result.CascadeDepth = cascadeDepth
	result.UpdateDuration = time.Since(startTime)

	return result, nil
}

// processFileUpdate processes a single file update
func (ie *IncrementalEngine) processFileUpdate(updateInfo UpdateInfo, result *UpdateResult) error {
	switch updateInfo.UpdateType {
	case UpdateTypeAdded, UpdateTypeModified:
		return ie.processFileAddOrModify(updateInfo, result)
	case UpdateTypeRemoved:
		return ie.processFileRemoval(updateInfo, result)
	case UpdateTypeCascade:
		return ie.processFileCascade(updateInfo, result)
	default:
		return fmt.Errorf("unknown update type: %v", updateInfo.UpdateType)
	}
}

// processFileAddOrModify processes adding or modifying a file
func (ie *IncrementalEngine) processFileAddOrModify(updateInfo UpdateInfo, result *UpdateResult) error {
	fileID := updateInfo.FileID
	path := ie.GetFilePath(fileID)

	// Store old symbols for comparison
	oldSymbols := make(map[uint32]*types.EnhancedSymbolInfo)
	if symbolTable := ie.symbolTables[fileID]; symbolTable != nil {
		for localID, symbol := range symbolTable.Symbols {
			oldSymbols[localID] = symbol
		}
	}

	// Re-index the file
	if err := ie.IndexFile(path, updateInfo.NewContent); err != nil {
		return fmt.Errorf("failed to re-index file: %w", err)
	}

	// Update file tracking
	ie.fileHashes[fileID] = updateInfo.NewHash
	ie.fileTimestamps[fileID] = updateInfo.Timestamp

	// Compare symbols to track changes
	newSymbolTable := ie.symbolTables[fileID]
	if newSymbolTable != nil {
		// Find added symbols
		for localID := range newSymbolTable.Symbols {
			if _, existed := oldSymbols[localID]; !existed {
				compositeID := types.NewCompositeSymbolID(fileID, localID)
				result.AddedSymbols = append(result.AddedSymbols, compositeID)
			}
		}

		// Find removed symbols
		for localID := range oldSymbols {
			if _, exists := newSymbolTable.Symbols[localID]; !exists {
				compositeID := types.NewCompositeSymbolID(fileID, localID)
				result.RemovedSymbols = append(result.RemovedSymbols, compositeID)

				// Clean up symbol links
				delete(ie.symbolLinks, compositeID)
			}
		}
	}

	// Update dependency graph
	ie.updateDependencyGraph(fileID)

	result.UpdatedFiles = append(result.UpdatedFiles, fileID)

	return nil
}

// processFileRemoval processes removing a file
func (ie *IncrementalEngine) processFileRemoval(updateInfo UpdateInfo, result *UpdateResult) error {
	fileID := updateInfo.FileID

	// Remove all symbols for this file
	if symbolTable := ie.symbolTables[fileID]; symbolTable != nil {
		for localID := range symbolTable.Symbols {
			compositeID := types.NewCompositeSymbolID(fileID, localID)
			result.RemovedSymbols = append(result.RemovedSymbols, compositeID)
			delete(ie.symbolLinks, compositeID)
		}
	}

	// Clean up file data
	delete(ie.symbolTables, fileID)
	delete(ie.fileHashes, fileID)
	delete(ie.fileTimestamps, fileID)
	delete(ie.importLinks, fileID)

	// Clean up dependency graph
	delete(ie.importGraph, fileID)
	delete(ie.exportedBy, fileID)
	delete(ie.fileDependents, fileID)

	// Remove from other files' dependency lists
	for otherFileID, dependents := range ie.fileDependents {
		ie.fileDependents[otherFileID] = ie.removeFileFromSlice(dependents, fileID)
	}

	for otherFileID, imports := range ie.importGraph {
		ie.importGraph[otherFileID] = ie.removeFileFromSlice(imports, fileID)
	}

	for otherFileID, exports := range ie.exportedBy {
		ie.exportedBy[otherFileID] = ie.removeFileFromSlice(exports, fileID)
	}

	result.UpdatedFiles = append(result.UpdatedFiles, fileID)

	return nil
}

// processFileCascade processes a cascade update (re-linking without re-indexing)
func (ie *IncrementalEngine) processFileCascade(updateInfo UpdateInfo, result *UpdateResult) error {
	fileID := updateInfo.FileID

	// Re-process the file's imports and exports without re-indexing
	if symbolTable := ie.symbolTables[fileID]; symbolTable != nil {
		if err := ie.processFileLinks(fileID, symbolTable); err != nil {
			return fmt.Errorf("failed to re-process file links: %w", err)
		}
	}

	// Update dependency graph
	ie.updateDependencyGraph(fileID)

	result.AffectedFiles = append(result.AffectedFiles, fileID)
	result.ModifiedLinks++

	return nil
}

// findCascadeUpdates finds files that need cascade updates due to a file change
func (ie *IncrementalEngine) findCascadeUpdates(changedFileID types.FileID, updateInfo UpdateInfo, cascadeUpdates map[types.FileID]UpdateInfo) {
	// Files that import from the changed file need cascade updates
	if dependents, exists := ie.fileDependents[changedFileID]; exists {
		for _, dependentFileID := range dependents {
			// Skip if already scheduled for update
			if _, alreadyScheduled := ie.pendingUpdates[dependentFileID]; alreadyScheduled {
				continue
			}
			if _, alreadyCascading := cascadeUpdates[dependentFileID]; alreadyCascading {
				continue
			}

			cascadeUpdate := UpdateInfo{
				FileID:     dependentFileID,
				Timestamp:  time.Now(),
				UpdateType: UpdateTypeCascade,
				AffectedBy: []types.FileID{changedFileID},
			}
			cascadeUpdates[dependentFileID] = cascadeUpdate
		}
	}
}

// updateDependencyGraph updates the dependency graph for a file
func (ie *IncrementalEngine) updateDependencyGraph(fileID types.FileID) {
	// Clear existing dependencies
	if oldImports, exists := ie.importGraph[fileID]; exists {
		for _, importedFileID := range oldImports {
			ie.fileDependents[importedFileID] = ie.removeFileFromSlice(ie.fileDependents[importedFileID], fileID)
		}
	}

	// Build new dependencies
	newImports := make([]types.FileID, 0)
	if importLinks := ie.importLinks[fileID]; importLinks != nil {
		for _, importLink := range importLinks {
			if importLink.ResolvedFile != 0 && !importLink.IsExternal {
				newImports = append(newImports, importLink.ResolvedFile)

				// Add this file as dependent of the imported file
				if ie.fileDependents[importLink.ResolvedFile] == nil {
					ie.fileDependents[importLink.ResolvedFile] = make([]types.FileID, 0)
				}
				if !ie.containsFileID(ie.fileDependents[importLink.ResolvedFile], fileID) {
					ie.fileDependents[importLink.ResolvedFile] = append(ie.fileDependents[importLink.ResolvedFile], fileID)
				}
			}
		}
	}

	ie.importGraph[fileID] = newImports
}

// Helper methods

// removeFileFromSlice removes a FileID from a slice
func (ie *IncrementalEngine) removeFileFromSlice(slice []types.FileID, item types.FileID) []types.FileID {
	result := make([]types.FileID, 0, len(slice))
	for _, id := range slice {
		if id != item {
			result = append(result, id)
		}
	}
	return result
}

// containsFileID checks if a slice contains a specific FileID
func (ie *IncrementalEngine) containsFileID(slice []types.FileID, item types.FileID) bool {
	for _, id := range slice {
		if id == item {
			return true
		}
	}
	return false
}

// GetFileDependents returns files that depend on the given file
func (ie *IncrementalEngine) GetFileDependents(fileID types.FileID) []types.FileID {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	if dependents := ie.fileDependents[fileID]; dependents != nil {
		result := make([]types.FileID, len(dependents))
		copy(result, dependents)
		return result
	}

	return []types.FileID{}
}

// GetFileDependencies returns files that the given file depends on
func (ie *IncrementalEngine) GetFileDependencies(fileID types.FileID) []types.FileID {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	if imports := ie.importGraph[fileID]; imports != nil {
		result := make([]types.FileID, len(imports))
		copy(result, imports)
		return result
	}

	return []types.FileID{}
}

// GetFileHash returns the content hash of a file
func (ie *IncrementalEngine) GetFileHash(fileID types.FileID) ([32]byte, bool) {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	hash, exists := ie.fileHashes[fileID]
	return hash, exists
}

// GetFileTimestamp returns the last update timestamp of a file
func (ie *IncrementalEngine) GetFileTimestamp(fileID types.FileID) (time.Time, bool) {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	timestamp, exists := ie.fileTimestamps[fileID]
	return timestamp, exists
}

// IncrementalStats returns statistics about incremental update capabilities
func (ie *IncrementalEngine) IncrementalStats() map[string]interface{} {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	totalDependencies := 0
	for _, deps := range ie.fileDependents {
		totalDependencies += len(deps)
	}

	return map[string]interface{}{
		"tracked_files":      len(ie.fileHashes),
		"dependency_edges":   totalDependencies,
		"pending_updates":    len(ie.pendingUpdates),
		"average_dependents": float64(totalDependencies) / float64(max(len(ie.fileDependents), 1)),
	}
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
