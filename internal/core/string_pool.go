package core

import (
	"sync"
	"sync/atomic"
)

// StringRange represents a range within a pooled string
type StringRange struct {
	PoolID uint32 // ID of the string pool entry
	Start  uint32 // Start offset in the pooled string
	Length uint32 // Length of the substring
}

// StringPool provides centralized string storage with efficient range-based access
type StringPool struct {
	mu      sync.RWMutex
	strings map[uint32]string // Map of pool ID to actual string content
	nextID  atomic.Uint32     // Atomic counter for pool IDs

	// Reverse lookup for deduplication
	lookup map[string]uint32
}

// NewStringPool creates a new string pool
func NewStringPool() *StringPool {
	return &StringPool{
		strings: make(map[uint32]string),
		lookup:  make(map[string]uint32),
	}
}

// Intern adds a string to the pool and returns its ID
// If the string already exists, returns the existing ID
func (sp *StringPool) Intern(s string) uint32 {
	// Fast path: check if already interned
	sp.mu.RLock()
	if id, exists := sp.lookup[s]; exists {
		sp.mu.RUnlock()
		return id
	}
	sp.mu.RUnlock()

	// Slow path: add to pool
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Double-check after acquiring write lock
	if id, exists := sp.lookup[s]; exists {
		return id
	}

	id := sp.nextID.Add(1)
	sp.strings[id] = s
	sp.lookup[s] = id

	return id
}

// InternRange adds a string to the pool and returns a StringRange for the full string
func (sp *StringPool) InternRange(s string) StringRange {
	id := sp.Intern(s)
	return StringRange{
		PoolID: id,
		Start:  0,
		Length: uint32(len(s)),
	}
}

// GetString retrieves the full string for a given pool ID
func (sp *StringPool) GetString(id uint32) (string, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	s, ok := sp.strings[id]
	return s, ok
}

// GetRangeString retrieves the substring specified by a StringRange
func (sp *StringPool) GetRangeString(r StringRange) (string, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	s, ok := sp.strings[r.PoolID]
	if !ok {
		return "", false
	}

	// Bounds checking
	if r.Start >= uint32(len(s)) {
		return "", false
	}

	end := r.Start + r.Length
	if end > uint32(len(s)) {
		end = uint32(len(s))
	}

	return s[r.Start:end], true
}

// CreateSubrange creates a new StringRange that is a subset of an existing range
func (sp *StringPool) CreateSubrange(parent StringRange, start, length uint32) StringRange {
	return StringRange{
		PoolID: parent.PoolID,
		Start:  parent.Start + start,
		Length: length,
	}
}

// FileStringPool manages strings for a specific file with line information
type FileStringPool struct {
	pool       *StringPool
	fileID     uint32        // Pool ID for the entire file content
	lineRanges []StringRange // Pre-computed ranges for each line
}

// NewFileStringPool creates a new file-specific string pool
func NewFileStringPool(pool *StringPool, content string) *FileStringPool {
	fileID := pool.Intern(content)

	// Pre-compute line ranges
	var lineRanges []StringRange
	start := uint32(0)

	for i, ch := range content {
		if ch == '\n' {
			lineRanges = append(lineRanges, StringRange{
				PoolID: fileID,
				Start:  start,
				Length: uint32(i) - start,
			})
			start = uint32(i + 1)
		}
	}

	// Don't forget the last line if it doesn't end with newline
	if start < uint32(len(content)) {
		lineRanges = append(lineRanges, StringRange{
			PoolID: fileID,
			Start:  start,
			Length: uint32(len(content)) - start,
		})
	}

	return &FileStringPool{
		pool:       pool,
		fileID:     fileID,
		lineRanges: lineRanges,
	}
}

// GetLine returns the StringRange for a specific line (0-indexed)
func (fsp *FileStringPool) GetLine(lineNum int) (StringRange, bool) {
	if lineNum < 0 || lineNum >= len(fsp.lineRanges) {
		return StringRange{}, false
	}
	return fsp.lineRanges[lineNum], true
}

// GetLines returns StringRanges for a range of lines
func (fsp *FileStringPool) GetLines(start, end int) []StringRange {
	if start < 0 {
		start = 0
	}
	if end > len(fsp.lineRanges) {
		end = len(fsp.lineRanges)
	}
	if start >= end {
		return nil
	}

	return fsp.lineRanges[start:end]
}

// GetLineCount returns the total number of lines
func (fsp *FileStringPool) GetLineCount() int {
	return len(fsp.lineRanges)
}

// GetContextLines returns StringRanges for lines around a given line with specified context
func (fsp *FileStringPool) GetContextLines(lineNum, contextBefore, contextAfter int) []StringRange {
	start := lineNum - contextBefore
	end := lineNum + contextAfter + 1
	return fsp.GetLines(start, end)
}
