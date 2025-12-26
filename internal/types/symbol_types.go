package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
)

// CompositeSymbolID is a composite identifier for symbols within the codebase
// It combines a FileID and LocalSymbolID to uniquely identify any symbol
// This will replace the current SymbolID (uint64) once migration is complete
type CompositeSymbolID struct {
	FileID        FileID // 32-bit file identifier
	LocalSymbolID uint32 // 32-bit symbol identifier within the file
}

// NewCompositeSymbolID creates a new CompositeSymbolID
func NewCompositeSymbolID(fileID FileID, localID uint32) CompositeSymbolID {
	return CompositeSymbolID{
		FileID:        fileID,
		LocalSymbolID: localID,
	}
}

// String returns a human-readable string representation for debugging
// This is intentionally fast - use CompactString() for external APIs
func (s CompositeSymbolID) String() string {
	return fmt.Sprintf("Symbol[F:%d,L:%d]", s.FileID, s.LocalSymbolID)
}

// valueToChar converts a base-63 value to its character representation
func valueToChar(val uint64) byte {
	// Use lookup table for O(1) conversion instead of nested conditionals
	// This reduces cyclomatic complexity from 4 to 0
	if val < 26 {
		return byte('A' + val)
	} else if val < 52 {
		return byte('a' + (val - 26))
	} else if val < 62 {
		return byte('0' + (val - 52))
	}
	return '_'
}

// CompactString returns the dense encoded representation for external APIs
func (s CompositeSymbolID) CompactString() string {
	// Use the same compact encoding as DenseObjectID
	combined := uint64(s.FileID) | (uint64(s.LocalSymbolID) << 32)

	if combined == 0 {
		return ""
	}

	var result []byte
	const base = 63

	// Encode using base-63 (A-Za-z0-9_)
	// Extracted loop logic for better testability
	for combined > 0 {
		val := combined % base
		c := valueToChar(val)
		result = append(result, c)
		combined /= base
	}

	// Reverse for correct order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// charToValue converts a character to its base-63 numeric value
func charToValue(c rune) (uint64, error) {
	// Use lookup logic instead of nested conditionals
	// Returns value and error in single call
	if c >= 'A' && c <= 'Z' {
		return uint64(c - 'A'), nil
	}
	if c >= 'a' && c <= 'z' {
		return uint64(c-'a') + 26, nil
	}
	if c >= '0' && c <= '9' {
		return uint64(c-'0') + 52, nil
	}
	if c == '_' {
		return 62, nil
	}
	return 0, fmt.Errorf("invalid character in compact string: %c", c)
}

// ParseCompactString decodes a compact string back to a CompositeSymbolID
func ParseCompactString(compact string) (CompositeSymbolID, error) {
	if compact == "" {
		return CompositeSymbolID{}, errors.New("empty compact string")
	}

	var combined uint64
	const base = 63

	// Decode from base-63
	for _, c := range compact {
		val, err := charToValue(c)
		if err != nil {
			return CompositeSymbolID{}, err
		}
		combined = combined*base + val
	}

	// Extract FileID and LocalSymbolID
	fileID := FileID(combined & 0xFFFFFFFF)
	localSymbolID := uint32(combined >> 32)

	return CompositeSymbolID{
		FileID:        fileID,
		LocalSymbolID: localSymbolID,
	}, nil
}

// Hash returns a hash value for the CompositeSymbolID
func (s CompositeSymbolID) Hash() uint64 {
	h := fnv.New64a()
	h.Write([]byte{
		byte(s.FileID >> 24),
		byte(s.FileID >> 16),
		byte(s.FileID >> 8),
		byte(s.FileID),
		byte(s.LocalSymbolID >> 24),
		byte(s.LocalSymbolID >> 16),
		byte(s.LocalSymbolID >> 8),
		byte(s.LocalSymbolID),
	})
	return h.Sum64()
}

// Equals checks if two CompositeSymbolIDs are equal
func (s CompositeSymbolID) Equals(other CompositeSymbolID) bool {
	return s.FileID == other.FileID && s.LocalSymbolID == other.LocalSymbolID
}

// IsValid checks if the CompositeSymbolID is valid
func (s CompositeSymbolID) IsValid() bool {
	// At least one component must be non-zero for basic validity
	// For complete validation, use ValidateRange() which enforces both components
	return s.FileID != 0 || s.LocalSymbolID != 0
}

// ValidateRange checks if the CompositeSymbolID components are within reasonable ranges
func (s CompositeSymbolID) ValidateRange() error {
	if s.FileID == 0 {
		return errors.New("invalid FileID: cannot be zero")
	}
	if s.LocalSymbolID == 0 {
		return errors.New("invalid LocalSymbolID: cannot be zero")
	}

	// Check for reasonable upper bounds to catch corruption
	const MaxFileID = 1 << 30        // ~1 billion files should be enough
	const MaxLocalSymbolID = 1 << 24 // ~16 million symbols per file should be enough

	if uint32(s.FileID) > MaxFileID {
		return fmt.Errorf("invalid FileID %d: exceeds maximum allowed (%d)", s.FileID, MaxFileID)
	}
	if s.LocalSymbolID > MaxLocalSymbolID {
		return fmt.Errorf("invalid LocalSymbolID %d: exceeds maximum allowed (%d)", s.LocalSymbolID, MaxLocalSymbolID)
	}

	return nil
}

// MarshalJSON implements json.Marshaler for CompositeSymbolID
// Returns the compact encoded string for minimal JSON size
func (s CompositeSymbolID) MarshalJSON() ([]byte, error) {
	// Use compact string encoding for JSON
	return json.Marshal(s.CompactString())
}

// UnmarshalJSON implements json.Unmarshaler for CompositeSymbolID
// Handles both compact string format and legacy object format
func (s *CompositeSymbolID) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a string (compact format)
	var compactStr string
	if err := json.Unmarshal(data, &compactStr); err == nil {
		// Reuse existing ParseCompactString function
		// This eliminates code duplication and maintains consistency
		parsed, err := ParseCompactString(compactStr)
		if err != nil {
			return err
		}
		*s = parsed
		return nil
	}

	// Fall back to legacy object format for backward compatibility
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	if fileID, ok := obj["file_id"].(float64); ok {
		s.FileID = FileID(fileID)
	}

	if localID, ok := obj["local_symbol_id"].(float64); ok {
		s.LocalSymbolID = uint32(localID)
	}

	return nil
}

// SymbolReference represents a reference to a symbol from another location
type SymbolReference struct {
	Symbol     CompositeSymbolID // The symbol being referenced
	Location   SymbolLocation    // Where the reference occurs
	IsExternal bool              // True if the symbol is external to the indexed folder
	ImportPath string            // Import path if applicable (e.g., "fmt", "react")
}

// SymbolLocation represents a location in the codebase
type SymbolLocation struct {
	FileID FileID
	Line   int
	Column int
	Offset int // Byte offset in file
}

// SymbolScope represents the scope hierarchy of a symbol
type SymbolScope struct {
	Type     SymbolScopeType
	Name     string // Name of the scope (e.g., function name, class name)
	Parent   *SymbolScope
	StartPos int // Start position in file
	EndPos   int // End position in file
}

// SymbolScopeType represents different types of scopes for symbols
// This is distinct from the existing ScopeType used elsewhere
type SymbolScopeType int

const (
	ScopeGlobal SymbolScopeType = iota
	ScopeModule
	ScopePackage
	ScopeClass
	ScopeFunction
	ScopeMethod
	ScopeBlock
	ScopeNamespace
	ScopeInterface
)

// String returns a string representation of the scope type
func (st SymbolScopeType) String() string {
	switch st {
	case ScopeGlobal:
		return "global"
	case ScopeModule:
		return "module"
	case ScopePackage:
		return "package"
	case ScopeClass:
		return "class"
	case ScopeFunction:
		return "function"
	case ScopeMethod:
		return "method"
	case ScopeBlock:
		return "block"
	case ScopeNamespace:
		return "namespace"
	case ScopeInterface:
		return "interface"
	default:
		return "unknown"
	}
}

// SymbolKind represents the kind of symbol
type SymbolKind int

const (
	SymbolKindUnknown SymbolKind = iota
	SymbolKindPackage
	SymbolKindImport
	SymbolKindType
	SymbolKindInterface
	SymbolKindStruct
	SymbolKindClass
	SymbolKindFunction
	SymbolKindMethod
	SymbolKindConstructor
	SymbolKindVariable
	SymbolKindConstant
	SymbolKindField
	SymbolKindProperty
	SymbolKindParameter
	SymbolKindLabel
	SymbolKindModule
	SymbolKindNamespace
	SymbolKindEnum
	SymbolKindEnumMember
	SymbolKindTrait
	SymbolKindEvent
	SymbolKindDelegate
	SymbolKindRecord
	SymbolKindAttribute
)

// symbolKindStrings provides O(1) lookup for symbol kind names
// This eliminates the need for a large switch statement
var symbolKindStrings = map[SymbolKind]string{
	SymbolKindPackage:     "package",
	SymbolKindImport:      "import",
	SymbolKindType:        "type",
	SymbolKindInterface:   "interface",
	SymbolKindStruct:      "struct",
	SymbolKindClass:       "class",
	SymbolKindFunction:    "function",
	SymbolKindMethod:      "method",
	SymbolKindConstructor: "constructor",
	SymbolKindVariable:    "variable",
	SymbolKindConstant:    "constant",
	SymbolKindField:       "field",
	SymbolKindProperty:    "property",
	SymbolKindParameter:   "parameter",
	SymbolKindLabel:       "label",
	SymbolKindModule:      "module",
	SymbolKindNamespace:   "namespace",
	SymbolKindEnum:        "enum",
	SymbolKindEnumMember:  "enum_member",
	SymbolKindTrait:       "trait",
	SymbolKindEvent:       "event",
	SymbolKindDelegate:    "delegate",
	SymbolKindRecord:      "record",
	SymbolKindAttribute:   "attribute",
}

// String returns a string representation of the symbol kind
func (sk SymbolKind) String() string {
	if name, ok := symbolKindStrings[sk]; ok {
		return name
	}
	return "unknown"
}

// EnhancedSymbolInfo represents detailed information about a symbol
type EnhancedSymbolInfo struct {
	ID         CompositeSymbolID
	Name       string
	Kind       SymbolKind
	Location   SymbolLocation
	Scope      *SymbolScope
	IsExported bool
	IsExternal bool

	// Language-specific metadata
	Language       string
	Signature      string             // Function/method signature
	Type           string             // Variable/field type
	Value          string             // Constant value if applicable
	TypeParameters []TypeParameter    `json:"type_parameters,omitempty"` // Generic type parameters
	Attributes     []ContextAttribute `json:"attributes,omitempty"`      // Decorators, attributes, annotations

	// Relationships
	ParentSymbol    *CompositeSymbolID  // For methods, fields, etc.
	ImplementsIDs   []CompositeSymbolID // Interfaces this type implements
	ReferencedByIDs []CompositeSymbolID // Symbols that reference this one
	ReferencesIDs   []CompositeSymbolID // Symbols this one references
}

// SymbolTable represents a collection of symbols for a file
type SymbolTable struct {
	FileID   FileID
	Language string
	Symbols  map[uint32]*EnhancedSymbolInfo // LocalSymbolID -> Symbol
	Imports  []ImportInfo
	Exports  []ExportInfo

	// Scope tree
	RootScope *SymbolScope

	// Quick lookups
	SymbolsByName map[string][]uint32 // Name -> LocalSymbolIDs
	NextLocalID   uint32              // Next available LocalSymbolID
}

// ImportInfo represents an import statement
type ImportInfo struct {
	LocalID       uint32   // Local symbol ID for this import
	ImportPath    string   // What is being imported
	Alias         string   // Import alias if any
	ImportedNames []string // Specific names imported (for named imports)
	IsDefault     bool     // True for default imports
	IsNamespace   bool     // True for namespace imports (import * as X)
	IsTypeOnly    bool     // True for TypeScript type-only imports
	Location      SymbolLocation
}

// ExportInfo represents an export statement
type ExportInfo struct {
	LocalID      uint32 // Local symbol ID for this export
	ExportedName string // Name as exported
	LocalName    string // Local name (might be different)
	IsDefault    bool   // True for default export
	IsTypeOnly   bool   // True for TypeScript type-only exports
	IsReExport   bool   // True if re-exporting from another module
	SourcePath   string // Source module for re-exports
	Location     SymbolLocation
}

// ModuleResolution represents how a module/package is resolved
type ModuleResolution struct {
	RequestPath  string // Original import/require path
	ResolvedPath string // Resolved file path
	FileID       FileID // Resolved file ID
	IsExternal   bool   // True if outside indexed folder
	IsBuiltin    bool   // True for language builtins
	Resolution   ResolutionType
	Error        string // Error message if resolution failed
}

// ResolutionType represents how a module was resolved
type ResolutionType int

const (
	ResolutionUnknown   ResolutionType = iota
	ResolutionFile                     // Direct file resolution
	ResolutionDirectory                // Directory with index file
	ResolutionPackage                  // Package.json resolution
	ResolutionModule                   // Go module resolution
	ResolutionBuiltin                  // Language builtin
	ResolutionExternal                 // External dependency
	ResolutionNotFound                 // Could not resolve
	ResolutionInternal                 // Internal to project
	ResolutionError                    // Resolution failed
)

// resolutionTypeStrings provides O(1) lookup for resolution type names
var resolutionTypeStrings = map[ResolutionType]string{
	ResolutionFile:      "file",
	ResolutionDirectory: "directory",
	ResolutionPackage:   "package",
	ResolutionModule:    "module",
	ResolutionBuiltin:   "builtin",
	ResolutionExternal:  "external",
	ResolutionNotFound:  "not_found",
	ResolutionInternal:  "internal",
	ResolutionError:     "error",
}

// String returns a string representation of the resolution type
func (rt ResolutionType) String() string {
	if name, ok := resolutionTypeStrings[rt]; ok {
		return name
	}
	return "unknown"
}
