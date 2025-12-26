// Package interfaces defines the core interfaces used throughout the Lightning Code Index system.
// These interfaces provide abstraction boundaries between different components, enabling
// modularity and testability.
package interfaces

import (
	"context"
	"github.com/standardbeagle/lci/internal/types"
)

// Indexer defines the interface for code indexing operations.
// Implementations of this interface are responsible for building and maintaining
// the code index, including symbol extraction, reference tracking, and file content management.
type Indexer interface {
	// Core indexing operations
	IndexDirectory(ctx context.Context, rootPath string) error
	GetFileInfo(fileID types.FileID) *types.FileInfo
	GetAllFileIDs() []types.FileID
	GetFileCount() int

	// Symbol operations
	GetFileSymbols(fileID types.FileID) []types.Symbol
	GetFileEnhancedSymbols(fileID types.FileID) []*types.EnhancedSymbol
	GetSymbolAtLine(fileID types.FileID, line int) *types.Symbol
	GetEnhancedSymbolAtLine(fileID types.FileID, line int) *types.EnhancedSymbol
	FindSymbolsByName(name string) []*types.EnhancedSymbol
	GetEnhancedSymbol(symbolID types.SymbolID) *types.EnhancedSymbol

	// Reference operations
	GetFileReferences(fileID types.FileID) []types.Reference
	GetSymbolReferences(symbolID types.SymbolID) []types.Reference

	// Import operations
	GetFileImports(fileID types.FileID) []types.Import

	// Scope operations
	GetFileScopeInfo(fileID types.FileID) []types.ScopeInfo

	// Block boundary operations
	GetFileBlockBoundaries(fileID types.FileID) []types.BlockBoundary

	// File content operations (using FileContentStore)
	GetFileContent(fileID types.FileID) ([]byte, bool)
	GetFilePath(fileID types.FileID) string
	GetFileLineOffsets(fileID types.FileID) ([]uint32, bool)
	GetFileLineCount(fileID types.FileID) int
	GetFileLine(fileID types.FileID, lineNum int) (string, bool)
	GetFileLines(fileID types.FileID, startLine, endLine int) []string

	// Statistics
	GetIndexStats() IndexStats
}

// IndexStats represents indexing statistics.
// These metrics provide insight into the size and complexity of the indexed codebase,
// as well as performance characteristics of the indexing operation.
type IndexStats struct {
	FileCount      int   // Total number of files indexed
	SymbolCount    int   // Total number of symbols extracted
	ReferenceCount int   // Total number of symbol references found
	ImportCount    int   // Total number of import statements processed
	TotalSizeBytes int64 // Combined size of all indexed files in bytes
	IndexTimeMs    int64 // Time taken to build the index in milliseconds
}

// FileProvider defines the minimal interface for providing file content.
// This interface is used by components that only need basic file access,
// such as the search engine.
type FileProvider interface {
	GetFileInfo(fileID types.FileID) *types.FileInfo
	GetAllFileIDs() []types.FileID
}

// SymbolProvider defines the interface for providing symbol information.
// This interface is used by components that need to query and analyze
// code symbols without requiring full indexing capabilities.
type SymbolProvider interface {
	GetFileSymbols(fileID types.FileID) []types.Symbol
	GetFileEnhancedSymbols(fileID types.FileID) []*types.EnhancedSymbol
	GetSymbolAtLine(fileID types.FileID, line int) *types.Symbol
	FindSymbolsByName(name string) []*types.EnhancedSymbol
}

// ReferenceProvider defines the interface for providing reference information.
// This interface is used by components that need to track and analyze
// symbol usage across the codebase.
type ReferenceProvider interface {
	GetFileReferences(fileID types.FileID) []types.Reference
	GetSymbolReferences(symbolID types.SymbolID) []types.Reference
}
