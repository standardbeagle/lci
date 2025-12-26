package core

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/standardbeagle/lci/internal/alloc"
	"github.com/standardbeagle/lci/internal/types"
)

// SearchCacheEntry holds a cached search result with metadata
type SearchCacheEntry struct {
	Results   []types.FileID
	Timestamp time.Time
}

type TrigramIndex struct {
	// Fast ASCII trigrams using simple uint32 bit-shifts
	trigrams map[uint32]*TrigramEntry

	// Unicode trigrams for non-ASCII content
	unicodeTrigrams map[string]*TrigramEntry // String-based Unicode trigrams

	// Slab allocator for efficient FileLocation slice management
	locationAllocator *alloc.SlabAllocator[FileLocation]

	// Single mutex for coordinating updates during indexing pause
	// Note: During normal search operations, GoroutineIndex already blocks updates
	updateMu sync.Mutex

	// Flag to indicate bulk indexing mode (lock-free when true)
	BulkIndexing int32

	// Invalidation list for efficient file removal
	invalidatedFiles map[types.FileID]bool
	cleanupThreshold int // Trigger cleanup when invalidation list reaches this size

	// OPTIMIZED: Search result cache for common patterns
	searchCache      map[string]SearchCacheEntry
	searchCacheMutex sync.Mutex
	searchCacheTTL   time.Duration // Time to live for cached results

	// Track active indexing operations to avoid caching during concurrent indexing
	activeIndexingOps int32

	// NEW: Bucketing configuration for sharded indexing
	bucketCount uint16 // Number of buckets (256 by default)
	bucketMask  uint32 // Mask for bucket calculation (bucketCount - 1)
}

type TrigramEntry struct {
	Locations []FileLocation
}

type FileLocation struct {
	FileID types.FileID
	Offset uint32
}

// SearchLocation represents a pre-computed search match location
type SearchLocation struct {
	FileID types.FileID
	Line   int
	Column int
	Offset int
}

func NewTrigramIndex() *TrigramIndex {
	return &TrigramIndex{
		trigrams:          make(map[uint32]*TrigramEntry),
		unicodeTrigrams:   make(map[string]*TrigramEntry),
		locationAllocator: alloc.NewTrigramSlabAllocator[FileLocation](),
		invalidatedFiles:  make(map[types.FileID]bool),
		cleanupThreshold:  100, // Trigger cleanup after 100 invalidated files
		// OPTIMIZED: Initialize search cache
		searchCache:    make(map[string]SearchCacheEntry),
		searchCacheTTL: 5 * time.Minute, // Cache for 5 minutes
		// NEW: Initialize bucketing (256 buckets by default)
		bucketCount: 256,
		bucketMask:  255, // 256 - 1 = 0xFF
	}
}

func (ti *TrigramIndex) Clear() {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	// Return all location slices to the allocator pools before clearing
	for _, entry := range ti.trigrams {
		ti.locationAllocator.Put(entry.Locations)
	}
	for _, entry := range ti.unicodeTrigrams {
		ti.locationAllocator.Put(entry.Locations)
	}

	ti.trigrams = make(map[uint32]*TrigramEntry)
	ti.unicodeTrigrams = make(map[string]*TrigramEntry)
	ti.invalidatedFiles = make(map[types.FileID]bool)

	// OPTIMIZED: Clear search cache
	ti.searchCache = make(map[string]SearchCacheEntry)

	// Reset allocator statistics
	ti.locationAllocator.ResetStats()
}

// predictTrigramCount estimates the number of trigrams in a file based on content size
// This is used for pre-allocation to reduce GrowSlice operations
// Typical trigram density: ~0.8-1.0 trigram per character for ASCII, ~0.3-0.5 for Unicode
func (ti *TrigramIndex) predictTrigramCount(contentSize int) int {
	if contentSize <= 0 {
		return 0
	}

	// Conservative estimate: 1 trigram per 2 characters on average
	// This gives us some headroom without over-allocating too much
	predicted := contentSize / 2

	// Minimum allocation to avoid too many small reallocations
	if predicted < 8 {
		return 8
	}

	// Cap at 1000 to prevent massive over-allocation for very large files
	if predicted > 1000 {
		return 1000
	}

	return predicted
}

// getFromCache retrieves a cached search result if it exists and is not expired
func (ti *TrigramIndex) getFromCache(pattern string) ([]types.FileID, bool) {
	ti.searchCacheMutex.Lock()
	defer ti.searchCacheMutex.Unlock()

	entry, exists := ti.searchCache[pattern]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.Timestamp) > ti.searchCacheTTL {
		// Expired, remove it
		delete(ti.searchCache, pattern)
		return nil, false
	}

	// Return a copy to avoid external modification
	result := make([]types.FileID, len(entry.Results))
	copy(result, entry.Results)
	return result, true
}

// setCache stores a search result in the cache
func (ti *TrigramIndex) setCache(pattern string, results []types.FileID) {
	ti.searchCacheMutex.Lock()
	defer ti.searchCacheMutex.Unlock()

	// Limit cache size to prevent unbounded growth
	if len(ti.searchCache) > 1000 {
		// Simple eviction: delete 10% of oldest entries
		// In a production system, you'd want a proper LRU cache
		countToEvict := len(ti.searchCache) / 10
		for pattern := range ti.searchCache {
			delete(ti.searchCache, pattern)
			countToEvict--
			if countToEvict <= 0 {
				break
			}
		}
	}

	// Store result with timestamp
	entry := SearchCacheEntry{
		Results:   results,
		Timestamp: time.Now(),
	}
	ti.searchCache[pattern] = entry
}

// invalidateCacheForFile removes cached search results that might be affected by a file removal or update
// This is called when files are removed or updated to ensure cache consistency
func (ti *TrigramIndex) invalidateCacheForFile(fileID types.FileID) {
	ti.searchCacheMutex.Lock()
	defer ti.searchCacheMutex.Unlock()

	// We need to check all cached results to see if they contain the file being removed/updated
	// If a cached result contains the file, we should remove it from the cache
	// However, checking every cached result is expensive

	// For simplicity and correctness, we clear the entire cache
	// This ensures stale results don't contain removed/updated files
	// Trade-off: reduced cache effectiveness vs. correctness
	ti.searchCache = make(map[string]SearchCacheEntry)
}

// invalidateCacheCompletely clears all cached search results
// Used when a large number of files are being modified/removed
func (ti *TrigramIndex) invalidateCacheCompletely() {
	ti.searchCacheMutex.Lock()
	defer ti.searchCacheMutex.Unlock()
	ti.searchCache = make(map[string]SearchCacheEntry)
}

func (ti *TrigramIndex) IndexFile(fileID types.FileID, content []byte) {
	// Increment active indexing operations
	atomic.AddInt32(&ti.activeIndexingOps, 1)
	defer atomic.AddInt32(&ti.activeIndexingOps, -1)

	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	// Clear any invalidation for this file since we're indexing it
	delete(ti.invalidatedFiles, fileID)

	// Extract trigrams based on content type
	if isPureASCII(content) {
		// Fast path for ASCII content - use simple uint32 bit-shifts
		trigrams := extractSimpleTrigrams(content)
		for offset, trigramHash := range trigrams {
			entry, exists := ti.trigrams[trigramHash]
			if !exists {
				entry = &TrigramEntry{}
				ti.trigrams[trigramHash] = entry
			}

			entry.Locations = append(entry.Locations, FileLocation{
				FileID: fileID,
				Offset: uint32(offset),
			})
		}
	} else {
		// Unicode path - use string-based trigrams
		trigrams := extractUnicodeTrigrams(content)
		for offset, trigramStr := range trigrams {
			entry, exists := ti.unicodeTrigrams[trigramStr]
			if !exists {
				entry = &TrigramEntry{}
				ti.unicodeTrigrams[trigramStr] = entry
			}

			entry.Locations = append(entry.Locations, FileLocation{
				FileID: fileID,
				Offset: uint32(offset),
			})
		}
	}
}

// IndexFileWithTrigrams indexes a file using pre-computed simple trigrams from the processor
// This is more efficient than IndexFile because trigrams are computed in parallel processors
// Thread-safe: Uses mutex to protect all map operations
// OPTIMIZED: Pre-calculates total trigram count to minimize GrowSlice operations
func (ti *TrigramIndex) IndexFileWithTrigrams(fileID types.FileID, trigrams map[uint32][]uint32) {
	// Always acquire lock to protect all map operations (trigrams, invalidatedFiles)
	// This ensures thread-safety for both bulk and incremental indexing
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	// Clear any invalidation for this file since we're indexing it
	delete(ti.invalidatedFiles, fileID)

	// OPTIMIZATION: First pass - calculate total trigram count for better pre-allocation
	// This helps us predict how much to allocate for each trigram
	var totalLocations int
	for _, offsets := range trigrams {
		totalLocations += len(offsets)
	}

	// If no trigrams, nothing to do
	if totalLocations == 0 {
		return
	}

	// Second pass - index with pre-calculated capacity
	// Note: avgLocations calculation was removed (was unused - allocation guidance not implemented)

	for trigramHash, offsets := range trigrams {
		entry, exists := ti.trigrams[trigramHash]
		if !exists {
			entry = &TrigramEntry{}
			ti.trigrams[trigramHash] = entry
		}

		// OPTIMIZED: Pre-calculate final size to avoid repeated GrowSlice
		existingLen := len(entry.Locations)
		additionalLen := len(offsets)
		finalSize := existingLen + additionalLen

		// Only grow if necessary
		if cap(entry.Locations) < finalSize {
			// Calculate target capacity with headroom (50% more to avoid frequent growth)
			targetCapacity := finalSize * 3 / 2
			// Use slab allocator to get a slice with better capacity
			newSlice := ti.locationAllocator.Get(targetCapacity)
			// Copy existing data
			if existingLen > 0 {
				newSlice = append(newSlice, entry.Locations...)
			}
			// Return old slice to pool
			ti.locationAllocator.Put(entry.Locations)
			entry.Locations = newSlice[:existingLen]
		}

		// Add all offsets for this trigram
		entry.Locations = append(entry.Locations, offsetsToFileLocations(fileID, offsets)...)
	}
}

// offsetsToFileLocations converts offset slice to FileLocation slice without intermediate allocations
func offsetsToFileLocations(fileID types.FileID, offsets []uint32) []FileLocation {
	locations := make([]FileLocation, len(offsets))
	for i, offset := range offsets {
		locations[i] = FileLocation{
			FileID: fileID,
			Offset: offset,
		}
	}
	return locations
}

func (ti *TrigramIndex) UpdateFile(fileID types.FileID, oldContent, newContent []byte) {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	// Clear any invalidation for this file since we're updating it
	delete(ti.invalidatedFiles, fileID)

	// Handle different content types for old and new content
	oldIsASCII := isPureASCII(oldContent)
	newIsASCII := isPureASCII(newContent)

	if oldIsASCII {
		// Remove old ASCII trigrams
		oldTrigrams := extractSimpleTrigrams(oldContent)
		for _, trigramHash := range oldTrigrams {
			if entry, exists := ti.trigrams[trigramHash]; exists {
				entry.Locations = removeFileLocations(entry.Locations, fileID)
				if len(entry.Locations) == 0 {
					delete(ti.trigrams, trigramHash)
				}
			}
		}
	} else {
		// Remove old Unicode trigrams
		oldTrigrams := extractUnicodeTrigrams(oldContent)
		for _, trigramStr := range oldTrigrams {
			if entry, exists := ti.unicodeTrigrams[trigramStr]; exists {
				entry.Locations = removeFileLocations(entry.Locations, fileID)
				if len(entry.Locations) == 0 {
					delete(ti.unicodeTrigrams, trigramStr)
				}
			}
		}
	}

	if newIsASCII {
		// Add new ASCII trigrams
		newTrigrams := extractSimpleTrigrams(newContent)
		for offset, trigramHash := range newTrigrams {
			entry, exists := ti.trigrams[trigramHash]
			if !exists {
				entry = &TrigramEntry{}
				ti.trigrams[trigramHash] = entry
			}

			entry.Locations = append(entry.Locations, FileLocation{
				FileID: fileID,
				Offset: uint32(offset),
			})
		}
	} else {
		// Add new Unicode trigrams
		newTrigrams := extractUnicodeTrigrams(newContent)
		for offset, trigramStr := range newTrigrams {
			entry, exists := ti.unicodeTrigrams[trigramStr]
			if !exists {
				entry = &TrigramEntry{}
				ti.unicodeTrigrams[trigramStr] = entry
			}

			entry.Locations = append(entry.Locations, FileLocation{
				FileID: fileID,
				Offset: uint32(offset),
			})
		}
	}

	// OPTIMIZED: Invalidate search cache since file content has changed
	// This ensures stale cached results don't contain outdated trigrams
	ti.invalidateCacheForFile(fileID)
}

func (ti *TrigramIndex) RemoveFile(fileID types.FileID, content []byte) {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	// Fast invalidation approach: mark file as invalid instead of expensive removal
	ti.invalidatedFiles[fileID] = true
	invalidationCount := len(ti.invalidatedFiles)

	// Trigger background cleanup if threshold reached
	if invalidationCount >= ti.cleanupThreshold {
		// Note: Background cleanup needs its own locking
		go ti.performCleanup()
	}

	// OPTIMIZED: Invalidate search cache since file is being removed
	// This ensures stale cached results don't contain references to the removed file
	ti.invalidateCacheForFile(fileID)
}

func (ti *TrigramIndex) FindCandidates(pattern string) []types.FileID {
	return ti.FindCandidatesWithOptions(pattern, false)
}

// FileCount returns the number of unique files in the index
// Note: During search operations, updates are already blocked by GoroutineIndex
func (ti *TrigramIndex) FileCount() int {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	uniqueFiles := make(map[types.FileID]bool)

	// Count files from ASCII trigrams
	for _, entry := range ti.trigrams {
		for _, location := range entry.Locations {
			// Only count files that haven't been invalidated
			if !ti.invalidatedFiles[location.FileID] {
				uniqueFiles[location.FileID] = true
			}
		}
	}

	// Count files from Unicode trigrams
	for _, entry := range ti.unicodeTrigrams {
		for _, location := range entry.Locations {
			// Only count files that haven't been invalidated
			if !ti.invalidatedFiles[location.FileID] {
				uniqueFiles[location.FileID] = true
			}
		}
	}

	return len(uniqueFiles)
}

// FindMatchLocations returns exact match locations for a pattern using trigram index
func (ti *TrigramIndex) FindMatchLocations(pattern string, caseInsensitive bool, fileProvider func(types.FileID) *types.FileInfo) []SearchLocation {
	if len(pattern) < 3 {
		return nil
	}

	searchPattern := pattern
	if caseInsensitive {
		searchPattern = strings.ToLower(pattern)
	}

	// Get candidate files first
	candidateFiles := ti.FindCandidatesWithOptions(pattern, caseInsensitive)
	if len(candidateFiles) == 0 {
		return nil
	}

	var locations []SearchLocation
	patternBytes := []byte(searchPattern)

	// For each candidate file, find exact matches using stored trigrams
	for _, fileID := range candidateFiles {
		fileInfo := fileProvider(fileID)
		if fileInfo == nil {
			continue
		}

		// Find all exact matches in this file
		content := fileInfo.Content
		if caseInsensitive {
			// Use bytes.ToLower to avoid double conversion ([]byte → string → []byte)
			content = bytes.ToLower(fileInfo.Content)
		}

		// Find exact pattern matches (use bytes.Index to avoid allocations)
		offset := 0
		for {
			idx := bytes.Index(content[offset:], patternBytes)
			if idx < 0 {
				break
			}

			matchOffset := offset + idx
			line, column := ti.offsetToLineColumn(fileInfo.LineOffsets, matchOffset)

			locations = append(locations, SearchLocation{
				FileID: fileID,
				Line:   line,
				Column: column,
				Offset: matchOffset,
			})

			offset = matchOffset + 1
		}
	}

	return locations
}

// offsetToLineColumn converts byte offset to line/column using LineOffsets
// LineOffsets[i] contains the byte offset of the start of line i+1
func (ti *TrigramIndex) offsetToLineColumn(lineOffsets []int, offset int) (int, int) {
	if len(lineOffsets) == 0 {
		return 1, 1
	}

	// Binary search for the line containing this offset
	// LineOffsets is sorted in ascending order
	low, high := 0, len(lineOffsets)-1
	for low < high {
		mid := (low + high + 1) / 2
		if lineOffsets[mid] <= offset {
			low = mid
		} else {
			high = mid - 1
		}
	}

	lineNum := low + 1 // Convert 0-indexed to 1-indexed
	lineStart := lineOffsets[low]
	column := offset - lineStart + 1

	// Ensure column is at least 1
	if column < 1 {
		column = 1
	}
	return lineNum, column
}

// fileIDCount is a temporary struct for batching map operations
type fileIDCount struct {
	fileID types.FileID
	count  int
}

func (ti *TrigramIndex) FindCandidatesWithOptions(pattern string, caseInsensitive bool) []types.FileID {
	if len(pattern) < 3 {
		return nil
	}

	searchPattern := pattern
	if caseInsensitive {
		searchPattern = strings.ToLower(pattern)
	}

	// OPTIMIZED: Check cache first, but skip if indexing operations are active
	// During active indexing, the index is being built and cache might have stale/incomplete results
	if atomic.LoadInt32(&ti.activeIndexingOps) == 0 {
		if cached, found := ti.getFromCache(searchPattern); found {
			return cached
		}
	}

	// Acquire read lock to protect map access during search
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	totalTrigrams := 0

	// Check if pattern is ASCII to determine which trigram system to use
	patternBytes := []byte(searchPattern)
	if isPureASCII(patternBytes) {
		// Use fast ASCII trigrams
		allPatternTrigrams := extractSimpleTrigrams(patternBytes)
		totalTrigrams = len(allPatternTrigrams)

		// OPTIMIZED: Batch map operations to reduce contention
		// Use temporary slice to accumulate counts, then aggregate
		var tempCounts []fileIDCount
		tempCounts = make([]fileIDCount, 0, 100) // Pre-allocate for common cases

		for _, trigramHash := range allPatternTrigrams {
			if entry, exists := ti.trigrams[trigramHash]; exists {
				for _, loc := range entry.Locations {
					tempCounts = append(tempCounts, fileIDCount{fileID: loc.FileID, count: 1})
				}
			}
		}

		// Aggregate counts in a single map write pass
		fileTrigramCounts := make(map[types.FileID]int)
		for _, fc := range tempCounts {
			fileTrigramCounts[fc.fileID] += fc.count
		}

		return ti.filterAndReturnCandidates(fileTrigramCounts, totalTrigrams, searchPattern)
	} else {
		// Use Unicode trigrams
		allPatternTrigrams := extractUnicodeTrigrams(patternBytes)
		totalTrigrams = len(allPatternTrigrams)

		// OPTIMIZED: Batch map operations for Unicode trigrams too
		var tempCounts []fileIDCount
		tempCounts = make([]fileIDCount, 0, 100)

		for _, trigramStr := range allPatternTrigrams {
			if entry, exists := ti.unicodeTrigrams[trigramStr]; exists {
				for _, loc := range entry.Locations {
					tempCounts = append(tempCounts, fileIDCount{fileID: loc.FileID, count: 1})
				}
			}
		}

		// Aggregate counts
		fileTrigramCounts := make(map[types.FileID]int)
		for _, fc := range tempCounts {
			fileTrigramCounts[fc.fileID] += fc.count
		}

		return ti.filterAndReturnCandidates(fileTrigramCounts, totalTrigrams, searchPattern)
	}
}

// filterAndReturnCandidates filters candidates based on match count and invalidated files
func (ti *TrigramIndex) filterAndReturnCandidates(fileTrigramCounts map[types.FileID]int, totalTrigrams int, pattern string) []types.FileID {
	if totalTrigrams == 0 {
		return nil
	}

	// Return files that contain a reasonable number of the trigrams
	// Tighten thresholds to reduce false candidates and improve search speed
	minRequiredMatches := 1
	if totalTrigrams > 6 {
		minRequiredMatches = totalTrigrams / 2 // Require at least half of trigrams
	} else if totalTrigrams > 3 {
		minRequiredMatches = 3 // Require at least 3 trigrams for mid-length patterns
	}

	// Filter out invalidated files before returning
	var candidates []types.FileID
	for fileID, matchCount := range fileTrigramCounts {
		if matchCount >= minRequiredMatches && !ti.invalidatedFiles[fileID] {
			candidates = append(candidates, fileID)
		}
	}

	// OPTIMIZED: Cache the results for future lookups
	// Only cache if there are results and no active indexing operations
	if len(candidates) > 0 && atomic.LoadInt32(&ti.activeIndexingOps) == 0 {
		ti.setCache(pattern, candidates)
	}

	return candidates
}

// isPureASCII checks if content contains only ASCII characters
func isPureASCII(content []byte) bool {
	for _, b := range content {
		if b > 127 {
			return false
		}
	}
	return true
}

// hasAlphaNumASCII checks if a byte sequence has at least one alphanumeric character
func hasAlphaNumASCII(content []byte, start, length int) bool {
	for i := start; i < start+length && i < len(content); i++ {
		b := content[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') || b == '_' {
			return true
		}
	}
	return false
}

// hasAlphaNumUnicode checks if a rune sequence has at least one alphanumeric character
func hasAlphaNumUnicode(runes []rune, start, length int) bool {
	for i := start; i < start+length && i < len(runes); i++ {
		if unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_' {
			return true
		}
	}
	return false
}

// extractSimpleTrigrams creates simple trigrams using direct uint32 bit-shifts for ASCII content
// This is much more memory-efficient than EnhancedTrigram objects
func extractSimpleTrigrams(content []byte) map[int]uint32 {
	if len(content) < 3 {
		return nil
	}

	trigrams := make(map[int]uint32)

	for i := 0; i <= len(content)-3; i++ {
		// Skip trigrams that are all non-alphanumeric (like "   " or "...")
		// But include trigrams with at least one alphanumeric character
		hasAlpha := isAlphaNum(content[i]) || isAlphaNum(content[i+1]) || isAlphaNum(content[i+2])
		if !hasAlpha {
			continue
		}

		// Direct bit-shift: (byte1 << 16) | (byte2 << 8) | byte3
		trigram := uint32(content[i])<<16 | uint32(content[i+1])<<8 | uint32(content[i+2])
		trigrams[i] = trigram
	}

	return trigrams
}

// extractUnicodeTrigrams creates string-based trigrams for Unicode content
// Less optimized than ASCII but handles full Unicode properly
func extractUnicodeTrigrams(content []byte) map[int]string {
	if len(content) < 3 {
		return nil
	}

	trigrams := make(map[int]string)
	runes := []rune(string(content))

	for i := 0; i <= len(runes)-3; i++ {
		// Include Unicode trigrams with at least one alphanumeric character
		hasAlpha := hasAlphaNumUnicode(runes, i, 3)
		if !hasAlpha {
			continue
		}

		// Create string trigram for Unicode content
		trigramStr := string(runes[i : i+3])
		// Convert back to byte offset for consistent indexing
		byteOffset := len(string(runes[:i]))
		trigrams[byteOffset] = trigramStr
	}

	return trigrams
}

// extractEnhancedTrigrams creates enhanced trigrams based on content type
func extractEnhancedTrigrams(content []byte) map[int]types.EnhancedTrigram {
	if len(content) < 3 {
		return nil
	}

	trigrams := make(map[int]types.EnhancedTrigram)

	// Check if content is pure ASCII for optimization
	if utf8.Valid(content) && isPureASCII(content) {
		// Use fast byte-based trigrams for ASCII content
		for i := 0; i <= len(content)-3; i++ {
			if hasAlphaNumASCII(content, i, 3) {
				trigram := types.NewByteTrigram(content[i], content[i+1], content[i+2])
				trigrams[i] = trigram
			}
		}
	} else {
		// Use rune-based trigrams for Unicode content
		runes := []rune(string(content))
		for i := 0; i <= len(runes)-3; i++ {
			if hasAlphaNumUnicode(runes, i, 3) {
				runeStr := string(runes[i : i+3])
				trigram := types.NewRuneTrigram(runeStr)
				trigrams[i] = trigram
			}
		}
	}

	return trigrams
}

// extractByteTrigrams creates byte-based trigrams for ASCII content
func extractByteTrigrams(content []byte) map[int]types.EnhancedTrigram {
	if len(content) < 3 {
		return nil
	}

	trigrams := make(map[int]types.EnhancedTrigram)
	for i := 0; i <= len(content)-3; i++ {
		if hasAlphaNumASCII(content, i, 3) {
			trigram := types.NewByteTrigram(content[i], content[i+1], content[i+2])
			trigrams[i] = trigram
		}
	}
	return trigrams
}

// extractRuneTrigrams creates rune-based trigrams for Unicode content
func extractRuneTrigrams(content []byte) map[int]types.EnhancedTrigram {
	if len(content) < 3 {
		return nil
	}

	trigrams := make(map[int]types.EnhancedTrigram)
	runes := []rune(string(content))
	for i := 0; i <= len(runes)-3; i++ {
		if hasAlphaNumUnicode(runes, i, 3) {
			runeStr := string(runes[i : i+3])
			trigram := types.NewRuneTrigram(runeStr)
			trigrams[i] = trigram
		}
	}
	return trigrams
}

// performCleanup performs background cleanup of invalidated files
func (ti *TrigramIndex) performCleanup() {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()

	// Get list of invalidated files and clear the list
	invalidatedFiles := make(map[types.FileID]bool)
	for fileID := range ti.invalidatedFiles {
		invalidatedFiles[fileID] = true
	}
	// Clear the invalidation list
	ti.invalidatedFiles = make(map[types.FileID]bool)

	if len(invalidatedFiles) == 0 {
		return
	}

	// Cleanup ASCII trigrams
	for trigramHash, entry := range ti.trigrams {
		// Filter out invalidated files from each trigram entry
		filteredLocations := entry.Locations[:0]
		for _, loc := range entry.Locations {
			if !invalidatedFiles[loc.FileID] {
				filteredLocations = append(filteredLocations, loc)
			}
		}

		if len(filteredLocations) == 0 {
			// No valid locations left, remove the trigram
			delete(ti.trigrams, trigramHash)
		} else {
			// Update the entry with filtered locations
			entry.Locations = filteredLocations
		}
	}

	// Cleanup Unicode trigrams
	for trigramStr, entry := range ti.unicodeTrigrams {
		// Filter out invalidated files from each trigram entry
		filteredLocations := entry.Locations[:0]
		for _, loc := range entry.Locations {
			if !invalidatedFiles[loc.FileID] {
				filteredLocations = append(filteredLocations, loc)
			}
		}

		if len(filteredLocations) == 0 {
			// No valid locations left, remove the trigram
			delete(ti.unicodeTrigrams, trigramStr)
		} else {
			// Update the entry with filtered locations
			entry.Locations = filteredLocations
		}
	}
}

// GetInvalidationCount returns the current number of invalidated files
func (ti *TrigramIndex) GetInvalidationCount() int {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()
	return len(ti.invalidatedFiles)
}

// SetCleanupThreshold sets the threshold for triggering background cleanup
func (ti *TrigramIndex) SetCleanupThreshold(threshold int) {
	ti.updateMu.Lock()
	defer ti.updateMu.Unlock()
	ti.cleanupThreshold = threshold
}

// ForceCleanup immediately performs cleanup of invalidated files
func (ti *TrigramIndex) ForceCleanup() {
	ti.performCleanup()
}

// SetBulkIndexing sets the bulk indexing mode
func (ti *TrigramIndex) SetBulkIndexing(enabled bool) {
	if enabled {
		atomic.StoreInt32(&ti.BulkIndexing, 1)
	} else {
		atomic.StoreInt32(&ti.BulkIndexing, 0)
	}
}

// GetAllocatorStats returns statistics from the slab allocator for monitoring
func (ti *TrigramIndex) GetAllocatorStats() alloc.AllocatorStats {
	return ti.locationAllocator.GetStats()
}

func removeFileLocations(locations []FileLocation, fileID types.FileID) []FileLocation {
	filtered := locations[:0]
	for _, loc := range locations {
		if loc.FileID != fileID {
			filtered = append(filtered, loc)
		}
	}
	return filtered
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
