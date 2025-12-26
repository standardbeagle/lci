package core

// Context-free unit: Minimal postings index mapping lowercased ASCII tokens (len>=3)
// to files and the first byte offset per file. Purpose: accelerate common literal
// word searches (e.g., "function") by avoiding per-file scans at query time.
// External deps: none; used by GoroutineIndex and search.Engine fast path.
// Prompt log: Initial implementation to meet <50ms search for 500/1000 files.

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/standardbeagle/lci/internal/types"
)

// PostingsIndex stores token -> (fileID -> firstOffset).
// Offsets are byte positions in original file content.
// For memory efficiency we only store the first offset per (token,file).
// Note: This index is ASCII-focused and best-effort; non-ASCII tokens are ignored.
type PostingsIndex struct {
	mu          sync.RWMutex
	tokens      map[string]map[types.FileID]int // token -> fileID -> firstOffset
	reverseKeys map[types.FileID][]string       // fileID -> tokens present (for fast removal)

	// Flag to indicate bulk indexing mode (lock-free when true)
	BulkIndexing int32
}

func NewPostingsIndex() *PostingsIndex {
	return &PostingsIndex{
		tokens:      make(map[string]map[types.FileID]int),
		reverseKeys: make(map[types.FileID][]string),
	}
}

// IndexFile tokenizes ASCII word tokens (a-z,A-Z,0-9,_) and records the first
// occurrence offset per token for this file. Skips tokens shorter than 3.
func (pi *PostingsIndex) IndexFile(fileID types.FileID, content []byte) {
	if len(content) == 0 {
		return
	}

	// Build a local set to avoid duplicate inserts and track first offset.
	tokensForFile := make(map[string]int)
	// Fast ASCII scan; treat letters and underscores/digits as part of a token.
	start := -1
	for i, b := range content {
		if isTokenChar(b) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			pi.maybeAddTokenAt(tokensForFile, content[start:i], start)
			start = -1
		}
	}
	if start >= 0 {
		pi.maybeAddTokenAt(tokensForFile, content[start:], start)
	}

	if len(tokensForFile) == 0 {
		return
	}

	// Only acquire lock if not in bulk indexing mode (multiple callers)
	// During indexing, FileIntegrator is the only writer (lock-free)
	if atomic.LoadInt32(&pi.BulkIndexing) == 0 {
		pi.mu.Lock()
		defer pi.mu.Unlock()
	}

	// Record reverse mapping first for efficient removal
	keys := make([]string, 0, len(tokensForFile))
	for tok := range tokensForFile {
		keys = append(keys, tok)
	}
	pi.reverseKeys[fileID] = keys

	for tok, off := range tokensForFile {
		m, ok := pi.tokens[tok]
		if !ok {
			m = make(map[types.FileID]int)
			pi.tokens[tok] = m
		}
		// Only store the first offset for this token/file
		if _, exists := m[fileID]; !exists {
			m[fileID] = off
		}
	}
}
func (pi *PostingsIndex) maybeAddTokenAt(dst map[string]int, raw []byte, absStart int) {
	if len(raw) < 3 {
		return
	}
	lower := bytes.ToLower(raw)
	if !isAllASCII(lower) {
		return
	}
	lower = bytes.TrimFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	})
	if len(lower) < 3 {
		return
	}
	tok := string(lower)
	if _, exists := dst[tok]; !exists {
		dst[tok] = absStart
	}
}

func (pi *PostingsIndex) maybeAddToken(dst map[string]int, raw []byte) {
	if len(raw) < 3 {
		return
	}
	// Lowercase for case-insensitive matching; skip non-ASCII quickly
	lower := bytes.ToLower(raw)
	if !isAllASCII(lower) {
		return
	}
	// Trim leading/trailing non-letter/digit/underscore just in case
	lower = bytes.TrimFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	})
	if len(lower) < 3 {
		return
	}
	tok := string(lower)
	// Record only the first offset for this token in this file
	if _, exists := dst[tok]; !exists {
		// Find first occurrence offset of tok in raw content segment's vicinity
		// To avoid costly global search, use the start of the provided slice, which
		// corresponds to the first occurrence position.
		dst[tok] = 0 // placeholder; will be corrected by caller via absolute index
	}
}

// RemoveFile removes all postings for a given fileID.
func (pi *PostingsIndex) RemoveFile(fileID types.FileID) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	keys := pi.reverseKeys[fileID]
	for _, tok := range keys {
		if m, ok := pi.tokens[tok]; ok {
			delete(m, fileID)
			if len(m) == 0 {
				delete(pi.tokens, tok)
			}
		}
	}
	delete(pi.reverseKeys, fileID)
}

// Find returns candidate files and first offsets for a token. If caseInsensitive
// is true, the token is lowercased before lookup.
func (pi *PostingsIndex) Find(token string, caseInsensitive bool) (files []types.FileID, firstOffsets map[types.FileID]int) {
	if len(token) < 3 {
		return nil, nil
	}
	tok := token
	if caseInsensitive {
		tok = strings.ToLower(token)
	}
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	m, ok := pi.tokens[tok]
	if !ok || len(m) == 0 {
		return nil, nil
	}
	firstOffsets = make(map[types.FileID]int, len(m))
	files = make([]types.FileID, 0, len(m))
	for fid, off := range m {
		files = append(files, fid)
		firstOffsets[fid] = off
	}
	return files, firstOffsets
}

// Helper: ASCII token char
func isTokenChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func isAllASCII(b []byte) bool {
	for _, c := range b {
		if c > 0x7F {
			return false
		}
	}
	return true
}
