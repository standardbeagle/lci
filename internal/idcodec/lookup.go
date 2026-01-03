package idcodec

import (
	"errors"
	"fmt"

	"github.com/standardbeagle/lci/internal/types"
)

// LookupErrorReason indicates why a symbol lookup failed.
type LookupErrorReason int

const (
	// ReasonNotFound indicates the symbol ID does not exist in the index.
	ReasonNotFound LookupErrorReason = iota
	// ReasonDeletedFile indicates the symbol's file has been deleted.
	ReasonDeletedFile
	// ReasonInvalidID indicates the provided ID was malformed.
	ReasonInvalidID
)

func (r LookupErrorReason) String() string {
	switch r {
	case ReasonNotFound:
		return "not found"
	case ReasonDeletedFile:
		return "file deleted"
	case ReasonInvalidID:
		return "invalid ID"
	default:
		return "unknown"
	}
}

// LookupError provides context about why a symbol lookup failed.
type LookupError struct {
	SymbolID types.SymbolID
	Reason   LookupErrorReason
	Detail   string // Optional additional detail
}

func (e *LookupError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("symbol lookup failed for %d: %s (%s)", e.SymbolID, e.Reason, e.Detail)
	}
	return fmt.Sprintf("symbol lookup failed for %d: %s", e.SymbolID, e.Reason)
}

// Is implements errors.Is for LookupError.
func (e *LookupError) Is(target error) bool {
	var le *LookupError
	if errors.As(target, &le) {
		return e.Reason == le.Reason
	}
	return false
}

// Sentinel errors for use with errors.Is
var (
	ErrSymbolNotFound    = &LookupError{Reason: ReasonNotFound}
	ErrSymbolFileDeleted = &LookupError{Reason: ReasonDeletedFile}
	ErrSymbolInvalidID   = &LookupError{Reason: ReasonInvalidID}
)

// NewNotFoundError creates a not-found error for a specific symbol ID.
func NewNotFoundError(id types.SymbolID) *LookupError {
	return &LookupError{SymbolID: id, Reason: ReasonNotFound}
}

// NewDeletedFileError creates a deleted-file error for a specific symbol ID.
func NewDeletedFileError(id types.SymbolID, filePath string) *LookupError {
	return &LookupError{SymbolID: id, Reason: ReasonDeletedFile, Detail: filePath}
}

// NewInvalidIDError creates an invalid-ID error.
func NewInvalidIDError(id types.SymbolID, detail string) *LookupError {
	return &LookupError{SymbolID: id, Reason: ReasonInvalidID, Detail: detail}
}

// SymbolGetter is the interface for symbol lookup.
// Implemented by ReferenceTracker.
type SymbolGetter interface {
	GetEnhancedSymbol(symbolID types.SymbolID) *types.EnhancedSymbol
}

// DeletedFileChecker is the interface for checking if a file is deleted.
type DeletedFileChecker interface {
	IsDeleted(fileID types.FileID) bool
}

// SymbolLookup provides safe symbol lookup with proper error handling.
// It wraps a SymbolGetter to provide typed errors.
type SymbolLookup struct {
	getter         SymbolGetter
	deletedChecker DeletedFileChecker
}

// NewSymbolLookup creates a new SymbolLookup.
func NewSymbolLookup(getter SymbolGetter, deletedChecker DeletedFileChecker) *SymbolLookup {
	return &SymbolLookup{
		getter:         getter,
		deletedChecker: deletedChecker,
	}
}

// Get looks up a symbol by ID with proper error handling.
// Returns (*EnhancedSymbol, nil) on success.
// Returns (nil, *LookupError) with specific reason on failure.
func (l *SymbolLookup) Get(id types.SymbolID) (*types.EnhancedSymbol, error) {
	if l.getter == nil {
		return nil, NewInvalidIDError(id, "symbol getter is nil")
	}

	symbol := l.getter.GetEnhancedSymbol(id)
	if symbol == nil {
		// Try to determine why - is it deleted or just not found?
		// Note: GetEnhancedSymbol already filters deleted files, so we return NotFound
		return nil, NewNotFoundError(id)
	}

	return symbol, nil
}

// GetOrNil looks up a symbol by ID, returning nil without error if not found.
// Use this when you expect some lookups to fail (e.g., in batch operations).
func (l *SymbolLookup) GetOrNil(id types.SymbolID) *types.EnhancedSymbol {
	if l.getter == nil {
		return nil
	}
	return l.getter.GetEnhancedSymbol(id)
}

// MustGet looks up a symbol by ID, panicking if not found.
// Use this only when you're certain the symbol exists.
func (l *SymbolLookup) MustGet(id types.SymbolID) *types.EnhancedSymbol {
	symbol, err := l.Get(id)
	if err != nil {
		panic(fmt.Sprintf("idcodec.MustGet: %v", err))
	}
	return symbol
}

// LookupResult holds a symbol lookup result with possible error.
type LookupResult struct {
	ID     types.SymbolID
	Symbol *types.EnhancedSymbol
	Error  error
}

// GetMultiple looks up multiple symbols, returning results for all.
// Continues on errors, returning partial results.
func (l *SymbolLookup) GetMultiple(ids []types.SymbolID) []LookupResult {
	results := make([]LookupResult, len(ids))
	for i, id := range ids {
		symbol, err := l.Get(id)
		results[i] = LookupResult{ID: id, Symbol: symbol, Error: err}
	}
	return results
}

// GetMultipleValid looks up multiple symbols, returning only valid results.
// Silently skips errors.
func (l *SymbolLookup) GetMultipleValid(ids []types.SymbolID) []*types.EnhancedSymbol {
	var results []*types.EnhancedSymbol
	for _, id := range ids {
		if symbol := l.GetOrNil(id); symbol != nil {
			results = append(results, symbol)
		}
	}
	return results
}

// DecodeAndGet decodes a base-63 string and looks up the symbol.
// Combines DecodeSymbolID and Get into a single operation.
func (l *SymbolLookup) DecodeAndGet(encoded string) (*types.EnhancedSymbol, error) {
	id, err := DecodeSymbolID(encoded)
	if err != nil {
		return nil, &LookupError{Reason: ReasonInvalidID, Detail: err.Error()}
	}
	return l.Get(id)
}
