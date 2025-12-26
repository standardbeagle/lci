package core

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/fnv"
	"github.com/standardbeagle/lci/internal/types"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
)

// LineRef represents a reference to a specific line within a file
type LineRef struct {
	FileID  types.FileID
	LineNum uint32 // 0-based line number
}

// FileContent holds the actual content and pre-computed line information
type FileContent struct {
	FileID      types.FileID
	Content     []byte       // The actual file content
	LineOffsets []uint32     // Byte offsets for start of each line
	FastHash    uint64       // xxhash for quick equality checks (~0.5ns)
	ContentHash [32]byte     // Pre-computed SHA256 hash for cache optimization
	RefCount    atomic.Int32 // Reference counting for cleanup
}

// FileContentSnapshot represents concurrent file content data using sync.Map
// for safe concurrent read/write access without copy-on-write overhead.
type FileContentSnapshot struct {
	files       sync.Map       // map[types.FileID]*FileContent
	pathToID    sync.Map       // map[string]types.FileID
	accessOrder []types.FileID // For LRU tracking (protected by single-writer)
}

// UpdateType represents the type of update operation
type UpdateType int

const (
	UpdateTypeLoad UpdateType = iota
	UpdateTypeBatch
	UpdateTypeInvalidate
	UpdateTypeInvalidateByID
	UpdateTypeClear
)

// ContentUpdate represents a file content update request
type ContentUpdate struct {
	Type      UpdateType
	Path      string
	Content   []byte
	FileID    types.FileID
	BatchData []struct {
		Path    string
		Content []byte
	}
	Response chan UpdateResult
}

// UpdateResult represents the result of an update operation
type UpdateResult struct {
	FileID  types.FileID
	FileIDs []types.FileID // For batch operations
	Success bool
	Error   error
}

// FileContentStore manages all file content centrally with concurrent read/write access.
// Uses sync.Map for O(1) concurrent operations without copy-on-write overhead.
//
// ARCHITECTURE:
//   - Concurrent reads via sync.Map (no locks, no copying)
//   - Writes serialized through dedicated goroutine for consistency
//   - O(1) per-file operations (no O(n) map copying)
//
// PERFORMANCE:
//   - Lock-free reads with atomic.Load (near-zero overhead)
//   - Updates serialized through channel (maintains consistency)
//   - Memory-efficient shallow copies for snapshots
type FileContentStore struct {
	// Immutable snapshot for lock-free reads
	snapshot atomic.Value // *FileContentSnapshot

	// Single-writer update channel
	updateChan chan *ContentUpdate
	closeChan  chan struct{}
	closeOnce  sync.Once     // Ensure Close() is only called once
	closed     atomic.Bool   // Track if store is closed
	doneChan   chan struct{} // Channel to wait for goroutine to finish

	// Memory management (atomic)
	currentMemory  atomic.Int64
	maxMemoryBytes int64

	// FileID generation (atomic)
	nextID atomic.Uint32
}

// NewFileContentStore creates a new lock-free file content store
func NewFileContentStore() *FileContentStore {
	return NewFileContentStoreWithLimit(500 * 1024 * 1024) // Default 500MB limit
}

// NewFileContentStoreWithLimit creates a new lock-free file content store with memory limit
func NewFileContentStoreWithLimit(maxMemoryBytes int64) *FileContentStore {
	store := &FileContentStore{
		updateChan:     make(chan *ContentUpdate, 100), // Buffered for performance
		closeChan:      make(chan struct{}),
		doneChan:       make(chan struct{}), // For waiting on goroutine
		maxMemoryBytes: maxMemoryBytes,
	}

	// Initialize with empty snapshot (sync.Map zero-values are ready to use)
	store.snapshot.Store(&FileContentSnapshot{
		accessOrder: make([]types.FileID, 0),
	})

	// Start update processor goroutine
	go store.processUpdates()

	return store
}

// Close shuts down the update processor goroutine
// This is safe to call multiple times due to sync.Once
func (fcs *FileContentStore) Close() {
	fcs.closeOnce.Do(func() {
		// Mark as closed to prevent new operations
		fcs.closed.Store(true)
		// Signal the processUpdates goroutine to stop
		close(fcs.closeChan)
		// Wait for the goroutine to finish draining
		<-fcs.doneChan
	})
}

// processUpdates handles all mutations in a single goroutine
func (fcs *FileContentStore) processUpdates() {
	defer close(fcs.doneChan)

	for {
		select {
		case update := <-fcs.updateChan:
			fcs.handleUpdate(update)
		case <-fcs.closeChan:
			// Drain any remaining updates before exiting
			for {
				select {
				case update := <-fcs.updateChan:
					// Send error response to unblock waiting goroutines
					update.Response <- UpdateResult{
						Success: false,
						Error:   errors.New("store is closing"),
					}
				default:
					return
				}
			}
		}
	}
}

// handleUpdate processes a single update request
func (fcs *FileContentStore) handleUpdate(update *ContentUpdate) {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)

	switch update.Type {
	case UpdateTypeLoad:
		newSnapshot, fileID := fcs.applyLoadUpdate(snapshot, update.Path, update.Content)
		// Enforce memory limit after adding content
		fcs.enforceMemoryLimit(newSnapshot)
		fcs.snapshot.Store(newSnapshot)
		update.Response <- UpdateResult{FileID: fileID, Success: true}

	case UpdateTypeBatch:
		newSnapshot, fileIDs := fcs.applyBatchUpdate(snapshot, update.BatchData)
		// Enforce memory limit after adding content
		fcs.enforceMemoryLimit(newSnapshot)
		fcs.snapshot.Store(newSnapshot)
		update.Response <- UpdateResult{FileIDs: fileIDs, Success: true}

	case UpdateTypeInvalidate:
		newSnapshot := fcs.applyInvalidateUpdate(snapshot, update.Path)
		fcs.snapshot.Store(newSnapshot)
		update.Response <- UpdateResult{Success: true}

	case UpdateTypeInvalidateByID:
		newSnapshot := fcs.applyInvalidateByIDUpdate(snapshot, update.FileID)
		fcs.snapshot.Store(newSnapshot)
		update.Response <- UpdateResult{Success: true}

	case UpdateTypeClear:
		// Create fresh snapshot with empty sync.Maps
		newSnapshot := &FileContentSnapshot{
			accessOrder: make([]types.FileID, 0),
		}
		fcs.snapshot.Store(newSnapshot)
		fcs.currentMemory.Store(0)
		fcs.nextID.Store(0)
		update.Response <- UpdateResult{Success: true}
	}
}

// applyLoadUpdate adds/updates a file using sync.Map (O(1), no copying)
func (fcs *FileContentStore) applyLoadUpdate(snapshot *FileContentSnapshot, path string, content []byte) (*FileContentSnapshot, types.FileID) {
	// Compute hashes and line offsets
	fastHash := xxhash.Sum64(content)
	lineOffsets := computeLineOffsets(content)

	// Check if file exists and content unchanged
	if idVal, exists := snapshot.pathToID.Load(path); exists {
		id := idVal.(types.FileID)
		if fcVal, ok := snapshot.files.Load(id); ok {
			fc := fcVal.(*FileContent)
			if fc.FastHash == fastHash {
				// Content unchanged
				return snapshot, id
			}
		}
	}

	// Determine FileID (existing or new)
	var fileID types.FileID
	if idVal, exists := snapshot.pathToID.Load(path); exists {
		fileID = idVal.(types.FileID)
		// Update existing file - track memory delta
		if fcVal, ok := snapshot.files.Load(fileID); ok {
			fc := fcVal.(*FileContent)
			oldSize := int64(len(fc.Content) + len(fc.LineOffsets)*4 + 64)
			newSize := int64(len(content) + len(lineOffsets)*4 + 64)
			fcs.currentMemory.Add(newSize - oldSize)
		}
	} else {
		// New file
		fileID = types.FileID(fcs.nextID.Add(1))
		newSize := int64(len(content) + len(lineOffsets)*4 + 64)
		fcs.currentMemory.Add(newSize)
	}

	// Create file content
	contentHash := sha256.Sum256(content)
	fc := &FileContent{
		FileID:      fileID,
		Content:     content,
		LineOffsets: lineOffsets,
		FastHash:    fastHash,
		ContentHash: contentHash,
	}
	fc.RefCount.Store(1)

	// Store in sync.Map (concurrent-safe, O(1))
	snapshot.files.Store(fileID, fc)
	snapshot.pathToID.Store(path, fileID)

	// Append to LRU (protected by single-writer)
	snapshot.accessOrder = append(snapshot.accessOrder, fileID)

	return snapshot, fileID
}

// applyBatchUpdate adds multiple files using sync.Map (O(k) for k files, no copying)
func (fcs *FileContentStore) applyBatchUpdate(snapshot *FileContentSnapshot, files []struct {
	Path    string
	Content []byte
}) (*FileContentSnapshot, []types.FileID) {
	fileIDs := make([]types.FileID, len(files))
	totalMemoryDelta := int64(0)

	for i, file := range files {
		fastHash := xxhash.Sum64(file.Content)
		lineOffsets := computeLineOffsets(file.Content)

		// Check if file exists and content unchanged
		if idVal, exists := snapshot.pathToID.Load(file.Path); exists {
			id := idVal.(types.FileID)
			if fcVal, ok := snapshot.files.Load(id); ok {
				fc := fcVal.(*FileContent)
				if fc.FastHash == fastHash {
					fileIDs[i] = id
					continue
				}
				// Content changed
				oldSize := int64(len(fc.Content) + len(fc.LineOffsets)*4 + 64)
				newSize := int64(len(file.Content) + len(lineOffsets)*4 + 64)
				totalMemoryDelta += newSize - oldSize
			}
			fileIDs[i] = id
		} else {
			// New file
			fileIDs[i] = types.FileID(fcs.nextID.Add(1))
			newSize := int64(len(file.Content) + len(lineOffsets)*4 + 64)
			totalMemoryDelta += newSize
		}

		// Create file content
		contentHash := sha256.Sum256(file.Content)
		fc := &FileContent{
			FileID:      fileIDs[i],
			Content:     file.Content,
			LineOffsets: lineOffsets,
			FastHash:    fastHash,
			ContentHash: contentHash,
		}
		fc.RefCount.Store(1)

		// Store in sync.Map
		snapshot.files.Store(fileIDs[i], fc)
		snapshot.pathToID.Store(file.Path, fileIDs[i])
		snapshot.accessOrder = append(snapshot.accessOrder, fileIDs[i])
	}

	if totalMemoryDelta != 0 {
		fcs.currentMemory.Add(totalMemoryDelta)
	}

	return snapshot, fileIDs
}

// applyInvalidateUpdate removes a file using sync.Map (O(1), no copying)
func (fcs *FileContentStore) applyInvalidateUpdate(snapshot *FileContentSnapshot, path string) *FileContentSnapshot {
	idVal, exists := snapshot.pathToID.Load(path)
	if !exists {
		return snapshot // No change needed
	}
	id := idVal.(types.FileID)

	// Update memory tracking before deletion
	if fcVal, ok := snapshot.files.Load(id); ok {
		fc := fcVal.(*FileContent)
		fileSize := int64(len(fc.Content) + len(fc.LineOffsets)*4 + 64)
		fcs.currentMemory.Add(-fileSize)
	}

	// Delete from sync.Maps (O(1))
	snapshot.files.Delete(id)
	snapshot.pathToID.Delete(path)

	// Update access order (protected by single-writer)
	newAccessOrder := make([]types.FileID, 0, len(snapshot.accessOrder))
	for _, fileID := range snapshot.accessOrder {
		if fileID != id {
			newAccessOrder = append(newAccessOrder, fileID)
		}
	}
	snapshot.accessOrder = newAccessOrder

	return snapshot
}

// applyInvalidateByIDUpdate removes a file by ID using sync.Map (O(n) for path lookup)
func (fcs *FileContentStore) applyInvalidateByIDUpdate(snapshot *FileContentSnapshot, fileID types.FileID) *FileContentSnapshot {
	if _, exists := snapshot.files.Load(fileID); !exists {
		return snapshot // No change needed
	}

	// Find path for this FileID (O(n) - sync.Map doesn't support reverse lookup)
	var pathToRemove string
	snapshot.pathToID.Range(func(key, value interface{}) bool {
		if value.(types.FileID) == fileID {
			pathToRemove = key.(string)
			return false // Stop iteration
		}
		return true // Continue iteration
	})

	if pathToRemove == "" {
		return snapshot
	}

	return fcs.applyInvalidateUpdate(snapshot, pathToRemove)
}

// enforceMemoryLimit performs LRU eviction using sync.Map if needed
func (fcs *FileContentStore) enforceMemoryLimit(snapshot *FileContentSnapshot) *FileContentSnapshot {
	if fcs.maxMemoryBytes <= 0 {
		return snapshot
	}

	currentMemory := fcs.currentMemory.Load()
	if currentMemory <= fcs.maxMemoryBytes {
		return snapshot
	}

	// Evict oldest files until under limit (using accessOrder for LRU)
	evictedIDs := make(map[types.FileID]bool)
	for i := 0; i < len(snapshot.accessOrder) && currentMemory > fcs.maxMemoryBytes; i++ {
		fileID := snapshot.accessOrder[i]

		// Find path for this fileID
		var pathToRemove string
		snapshot.pathToID.Range(func(key, value interface{}) bool {
			if value.(types.FileID) == fileID {
				pathToRemove = key.(string)
				return false
			}
			return true
		})

		if pathToRemove != "" {
			if fcVal, ok := snapshot.files.Load(fileID); ok {
				fc := fcVal.(*FileContent)
				fileSize := int64(len(fc.Content) + len(fc.LineOffsets)*4 + 64)
				currentMemory -= fileSize
				fcs.currentMemory.Add(-fileSize)
			}
			snapshot.files.Delete(fileID)
			snapshot.pathToID.Delete(pathToRemove)
			evictedIDs[fileID] = true
		}
	}

	// Update access order to remove evicted files
	if len(evictedIDs) > 0 {
		newAccessOrder := make([]types.FileID, 0, len(snapshot.accessOrder)-len(evictedIDs))
		for _, fileID := range snapshot.accessOrder {
			if !evictedIDs[fileID] {
				newAccessOrder = append(newAccessOrder, fileID)
			}
		}
		snapshot.accessOrder = newAccessOrder
	}

	return snapshot
}

// ==================== PUBLIC API (Lock-Free Read Operations) ====================

// GetContent returns the full content for a file (LOCK-FREE)
func (fcs *FileContentStore) GetContent(fileID types.FileID) ([]byte, bool) {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	if fcVal, ok := snapshot.files.Load(fileID); ok {
		return fcVal.(*FileContent).Content, true
	}
	return nil, false
}

// GetLineOffsets returns the precomputed line offsets for a file (LOCK-FREE)
func (fcs *FileContentStore) GetLineOffsets(fileID types.FileID) ([]uint32, bool) {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	if fcVal, ok := snapshot.files.Load(fileID); ok {
		return fcVal.(*FileContent).LineOffsets, true
	}
	return nil, false
}

// GetString materializes a string from a types.StringRef (LOCK-FREE)
func (fcs *FileContentStore) GetString(ref types.StringRef) (string, error) {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	fcVal, ok := snapshot.files.Load(ref.FileID)
	if !ok {
		return "", fmt.Errorf("StringRef with FileID %d not found", ref.FileID)
	}
	fc := fcVal.(*FileContent)

	end := ref.Offset + ref.Length
	if ref.Offset >= uint32(len(fc.Content)) || end > uint32(len(fc.Content)) {
		return "", fmt.Errorf("StringRef bounds invalid - FileID:%d Offset:%d Length:%d ContentLen:%d",
			ref.FileID, ref.Offset, ref.Length, len(fc.Content))
	}

	return string(fc.Content[ref.Offset:end]), nil
}

// GetBytes returns the byte slice for a types.StringRef (LOCK-FREE)
func (fcs *FileContentStore) GetBytes(ref types.StringRef) ([]byte, error) {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	fcVal, ok := snapshot.files.Load(ref.FileID)
	if !ok {
		return nil, fmt.Errorf("StringRef with FileID %d not found", ref.FileID)
	}
	fc := fcVal.(*FileContent)

	end := ref.Offset + ref.Length
	if ref.Offset >= uint32(len(fc.Content)) || end > uint32(len(fc.Content)) {
		return nil, fmt.Errorf("StringRef bounds invalid - FileID:%d Offset:%d Length:%d ContentLen:%d",
			ref.FileID, ref.Offset, ref.Length, len(fc.Content))
	}

	return fc.Content[ref.Offset:end], nil
}

// GetLine returns a types.StringRef for a specific line (LOCK-FREE)
func (fcs *FileContentStore) GetLine(fileID types.FileID, lineNum int) (types.StringRef, bool) {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	fcVal, ok := snapshot.files.Load(fileID)
	if !ok {
		return types.StringRef{}, false
	}
	fc := fcVal.(*FileContent)

	if lineNum < 0 || lineNum >= len(fc.LineOffsets) {
		return types.StringRef{}, false
	}

	start := fc.LineOffsets[lineNum]
	var end uint32

	if lineNum+1 < len(fc.LineOffsets) {
		end = fc.LineOffsets[lineNum+1]
		// Remove newline character
		if end > start && fc.Content[end-1] == '\n' {
			end--
		}
	} else {
		end = uint32(len(fc.Content))
	}

	length := end - start
	var hash uint64
	if length > 0 {
		hash = computeHash(fc.Content[start:end])
	}

	return types.StringRef{
		FileID: fileID,
		Offset: start,
		Length: length,
		Hash:   hash,
	}, true
}

// GetLineCount returns the number of lines in a file (LOCK-FREE)
func (fcs *FileContentStore) GetLineCount(fileID types.FileID) int {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	if fcVal, ok := snapshot.files.Load(fileID); ok {
		return len(fcVal.(*FileContent).LineOffsets)
	}
	return 0
}

// GetContentHash returns the pre-computed SHA256 hash for a file (LOCK-FREE)
func (fcs *FileContentStore) GetContentHash(fileID types.FileID) [32]byte {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	if fcVal, ok := snapshot.files.Load(fileID); ok {
		return fcVal.(*FileContent).ContentHash
	}
	return [32]byte{}
}

// GetFastHash returns the pre-computed xxhash for a file (LOCK-FREE)
func (fcs *FileContentStore) GetFastHash(fileID types.FileID) uint64 {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	if fcVal, ok := snapshot.files.Load(fileID); ok {
		return fcVal.(*FileContent).FastHash
	}
	return 0
}

// GetFileCount returns the number of files in the store (LOCK-FREE)
func (fcs *FileContentStore) GetFileCount() int {
	snapshot := fcs.snapshot.Load().(*FileContentSnapshot)
	count := 0
	snapshot.files.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetMemoryUsage returns the current memory usage (LOCK-FREE)
func (fcs *FileContentStore) GetMemoryUsage() int64 {
	return fcs.currentMemory.Load()
}

// GetFileVersion is deprecated and always returns 0, false
// The Version field has been removed from FileContent as it was not being used
// for optimistic locking as originally intended.
func (fcs *FileContentStore) GetFileVersion(fileID types.FileID) (uint64, bool) {
	return 0, false
}

// SlowEqual performs full content comparison when hashes match
func (fcs *FileContentStore) SlowEqual(ref1, ref2 types.StringRef) (bool, error) {
	// Already checked hash and length equality
	if ref1.FileID == ref2.FileID && ref1.Offset == ref2.Offset {
		return true, nil
	}

	b1, err1 := fcs.GetBytes(ref1)
	if err1 != nil {
		return false, fmt.Errorf("failed to get bytes for ref1: %w", err1)
	}

	b2, err2 := fcs.GetBytes(ref2)
	if err2 != nil {
		return false, fmt.Errorf("failed to get bytes for ref2: %w", err2)
	}

	return string(b1) == string(b2), nil
}

// GetLines returns types.StringRefs for a range of lines
func (fcs *FileContentStore) GetLines(fileID types.FileID, startLine, endLine int) []types.StringRef {
	var refs []types.StringRef
	for i := startLine; i < endLine; i++ {
		if ref, ok := fcs.GetLine(fileID, i); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

// GetContextLines returns types.StringRefs for lines around a given line
func (fcs *FileContentStore) GetContextLines(fileID types.FileID, lineNum, before, after int) []types.StringRef {
	start := lineNum - before
	end := lineNum + after + 1
	return fcs.GetLines(fileID, start, end)
}

// CreateSubstring creates a types.StringRef for a substring within another types.StringRef
func (fcs *FileContentStore) CreateSubstring(parent types.StringRef, offset, length uint32) types.StringRef {
	if offset >= parent.Length {
		return types.EmptyStringRef
	}
	if offset+length > parent.Length {
		length = parent.Length - offset
	}

	ref := types.StringRef{
		FileID: parent.FileID,
		Offset: parent.Offset + offset,
		Length: length,
	}

	// Compute hash if we have content
	if bytes, err := fcs.GetBytes(ref); err == nil && len(bytes) > 0 {
		ref.Hash = computeHash(bytes)
	}

	return ref
}

// CreateStringRef creates a types.StringRef with computed hash
func (fcs *FileContentStore) CreateStringRef(fileID types.FileID, start, length uint32) types.StringRef {
	ref := types.StringRef{
		FileID: fileID,
		Offset: start,
		Length: length,
	}

	// Compute hash
	if bytes, err := fcs.GetBytes(ref); err == nil && len(bytes) > 0 {
		ref.Hash = computeHash(bytes)
	}

	return ref
}

// ==================== PUBLIC API (Write Operations via Channel) ====================

// LoadFile loads a file's content into the store and returns its ID
func (fcs *FileContentStore) LoadFile(path string, content []byte) types.FileID {
	// Check if store is closed
	if fcs.closed.Load() {
		return 0 // Return invalid FileID
	}

	update := &ContentUpdate{
		Type:     UpdateTypeLoad,
		Path:     path,
		Content:  content,
		Response: make(chan UpdateResult, 1),
	}

	// Send the update - this will block if channel is full
	fcs.updateChan <- update

	// Wait for response
	result := <-update.Response
	return result.FileID
}

// BatchLoadFiles loads multiple files with a single update
func (fcs *FileContentStore) BatchLoadFiles(files []struct {
	Path    string
	Content []byte
}) []types.FileID {
	if len(files) == 0 {
		return nil
	}

	update := &ContentUpdate{
		Type:      UpdateTypeBatch,
		BatchData: files,
		Response:  make(chan UpdateResult, 1),
	}
	fcs.updateChan <- update
	result := <-update.Response
	return result.FileIDs
}

// InvalidateFile removes a file from the store
func (fcs *FileContentStore) InvalidateFile(path string) {
	update := &ContentUpdate{
		Type:     UpdateTypeInvalidate,
		Path:     path,
		Response: make(chan UpdateResult, 1),
	}
	fcs.updateChan <- update
	<-update.Response
}

// InvalidateFileByID removes a file from the store by its FileID
func (fcs *FileContentStore) InvalidateFileByID(fileID types.FileID) {
	update := &ContentUpdate{
		Type:     UpdateTypeInvalidateByID,
		FileID:   fileID,
		Response: make(chan UpdateResult, 1),
	}
	fcs.updateChan <- update
	<-update.Response
}

// Clear removes all files from the store
func (fcs *FileContentStore) Clear() {
	// Check if store is closed
	if fcs.closed.Load() {
		return // Store is closed, nothing to clear
	}

	update := &ContentUpdate{
		Type:     UpdateTypeClear,
		Response: make(chan UpdateResult, 1),
	}

	// Send the update
	fcs.updateChan <- update

	// Wait for response with timeout to avoid hanging
	select {
	case <-update.Response:
		// Success
	case <-time.After(100 * time.Millisecond):
		// Timeout - store might be closing
	}
}

// ==================== HELPER FUNCTIONS ====================

// computeLineOffsets computes byte offsets for each line in the content
func computeLineOffsets(content []byte) []uint32 {
	if len(content) == 0 {
		return nil
	}

	// Estimate number of lines to pre-allocate with better capacity
	estimatedLines := len(content)/80 + 2 // +2 for first line and margin
	if estimatedLines > 1000 {
		estimatedLines = 1000 // Cap pre-allocation to avoid over-allocation
	}

	offsets := make([]uint32, 1, estimatedLines)
	offsets[0] = 0 // First line starts at offset 0

	for i, b := range content {
		if b == '\n' && i+1 < len(content) {
			offsets = append(offsets, uint32(i+1))
		}
	}

	return offsets
}

// computeHash computes the FNV-1a hash for the given byte slice
func computeHash(data []byte) uint64 {
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}
