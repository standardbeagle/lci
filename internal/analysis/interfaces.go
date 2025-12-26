package analysis

import (
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// SymbolLinkerInterface defines the interface for symbol linking functionality
type SymbolLinkerInterface interface {
	// ResolveReferences resolves symbol references for a given symbol
	ResolveReferences(symbolID types.CompositeSymbolID) ([]ResolvedReference, error)

	// ResolveDependencies resolves dependencies for a symbol
	ResolveDependencies(symbolID types.CompositeSymbolID) ([]types.SymbolDependency, error)

	// ResolveImports resolves import statements for a file
	ResolveImports(fileID types.FileID) ([]ImportResolution, error)
}

// FileServiceInterface defines the interface for file service operations
type FileServiceInterface interface {
	// GetFileContent returns the content of a file
	GetFileContent(fileID types.FileID) (string, error)

	// GetFilePath returns the path of a file
	GetFilePath(fileID types.FileID) (string, error)

	// GetFileInfo returns information about a file
	GetFileInfo(fileID types.FileID) (FileInfo, error)
}

// LanguageAnalyzer defines the interface for language-specific analysis
type LanguageAnalyzer interface {
	// ExtractSymbols extracts all symbols from file content
	ExtractSymbols(fileID types.FileID, content, filePath string) ([]*types.UniversalSymbolNode, error)

	// AnalyzeExtends analyzes inheritance/extension relationships
	AnalyzeExtends(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error)

	// AnalyzeImplements analyzes interface implementation relationships
	AnalyzeImplements(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error)

	// AnalyzeContains analyzes containment relationships (methods in classes, etc.)
	AnalyzeContains(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error)

	// AnalyzeDependencies analyzes dependency relationships
	AnalyzeDependencies(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.SymbolDependency, error)

	// AnalyzeCalls analyzes function call relationships
	AnalyzeCalls(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.FunctionCall, error)

	// GetLanguageName returns the name of the language this analyzer handles
	GetLanguageName() string
}

// ResolvedReference represents a resolved symbol reference
type ResolvedReference struct {
	Target         types.CompositeSymbolID `json:"target"`          // Target symbol ID
	SourceLocation types.SymbolLocation    `json:"source_location"` // Where the reference occurs
	ImportPath     string                  `json:"import_path"`     // Import path if applicable
	Confidence     float64                 `json:"confidence"`      // Confidence in resolution (0-1)
	RefType        ReferenceType           `json:"ref_type"`        // Type of reference
}

// ImportResolution represents a resolved import
type ImportResolution struct {
	ImportPath   string                    `json:"import_path"`   // The import path
	ResolvedPath string                    `json:"resolved_path"` // Resolved file path
	Symbols      []types.CompositeSymbolID `json:"symbols"`       // Imported symbols
	ImportType   ImportType                `json:"import_type"`   // Type of import
	Location     types.SymbolLocation      `json:"location"`      // Location of import statement
}

// FileInfo represents information about a file
type FileInfo struct {
	Path         string    `json:"path"`
	Language     string    `json:"language"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	IsGenerated  bool      `json:"is_generated"`
}

// AnalysisStats represents statistics from relationship analysis
type AnalysisStats struct {
	TotalSymbols      int            `json:"total_symbols"`
	SymbolsByLanguage map[string]int `json:"symbols_by_language"`
	SymbolsByKind     map[string]int `json:"symbols_by_kind"`
	RelationshipCount map[string]int `json:"relationship_count"`
	QueryCount        int64          `json:"query_count"`
	AvgQueryTime      time.Duration  `json:"avg_query_time"`
	LastUpdated       time.Time      `json:"last_updated"`
	MemoryUsage       int64          `json:"memory_usage"`
	EnabledAnalyzers  []string       `json:"enabled_analyzers"`
}

// ReferenceType represents different types of symbol references
type ReferenceType uint8

const (
	RefTypeUnknown        ReferenceType = iota
	RefTypeCall                         // Function call
	RefTypeAccess                       // Variable/field access
	RefTypeType                         // Type reference
	RefTypeInheritance                  // Inheritance reference
	RefTypeImplementation               // Interface implementation
	RefTypeImport                       // Import reference
	RefTypeAnnotation                   // Annotation reference
)

// String returns string representation of reference type
func (rt ReferenceType) String() string {
	switch rt {
	case RefTypeCall:
		return "call"
	case RefTypeAccess:
		return "access"
	case RefTypeType:
		return "type"
	case RefTypeInheritance:
		return "inheritance"
	case RefTypeImplementation:
		return "implementation"
	case RefTypeImport:
		return "import"
	case RefTypeAnnotation:
		return "annotation"
	default:
		return "unknown"
	}
}

// ImportType represents different types of imports
type ImportType uint8

const (
	ImportTypeUnknown   ImportType = iota
	ImportTypeNamed                // import { foo } from 'bar'
	ImportTypeDefault              // import foo from 'bar'
	ImportTypeNamespace            // import * as foo from 'bar'
	ImportTypeModule               // import 'bar'
	ImportTypeRequire              // const foo = require('bar')
	ImportTypePackage              // import "package" (Go)
	ImportTypeDynamic              // import('bar') or require.resolve('bar')
)

// String returns string representation of import type
func (it ImportType) String() string {
	switch it {
	case ImportTypeNamed:
		return "named"
	case ImportTypeDefault:
		return "default"
	case ImportTypeNamespace:
		return "namespace"
	case ImportTypeModule:
		return "module"
	case ImportTypeRequire:
		return "require"
	case ImportTypePackage:
		return "package"
	case ImportTypeDynamic:
		return "dynamic"
	default:
		return "unknown"
	}
}
