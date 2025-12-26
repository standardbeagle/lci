package types

import (
	"fmt"
	"strings"
	"time"
)

// Common system-wide constants
const (
	// File size limits
	DefaultMaxFileSize = 10 * 1024 * 1024 // 10MB per file - standard limit for indexing
	// Rationale: Prevents memory exhaustion from large
	// generated files while covering 99.9% of source files.
	// Large files are typically binaries or generated code.

	// Memory limits
	DefaultMaxMemoryMB = 100 // 100MB - typical memory limit for lightweight indexing
	// Rationale: Allows indexing to run on resource-constrained
	// environments (CI, containers) while providing good
	// performance for typical codebases.

	// Performance limits
	DefaultMaxFileCount = 10000 // Maximum files to index in a single operation
	// Rationale: Covers most application codebases while
	// preventing runaway indexing of node_modules or
	// vendor directories. Enterprise projects can increase.

	DefaultMaxTotalSizeMB = 500 // Maximum total size for indexed files (MB)
	// Rationale: 5x memory limit provides headroom for
	// index structures while preventing excessive memory
	// use. Allows indexing medium-sized monorepos.

	// Binary detection optimization threshold
	BinaryPreCheckSizeThreshold = 100 * 1024 // 100KB - files above this size get pre-checked for binary content
	// Rationale: Reading first 512 bytes to detect binary files
	// is cheaper than loading the entire file into memory.
	// This prevents wasting memory on large binary files.
	BinaryPreCheckBytes = 512 // Number of bytes to read for binary magic number detection
)

type FileID uint32
type SymbolID uint64

// ContextAttributeType represents different types of context-altering attributes
type ContextAttributeType uint8

const (
	AttrTypeDirective    ContextAttributeType = iota // "use server", "use client", etc.
	AttrTypeUnsafe                                   // unsafe blocks or operations
	AttrTypeLock                                     // lock statements, mutex operations
	AttrTypeDecorator                                // @decorator annotations
	AttrTypePragma                                   // #pragma directives
	AttrTypeIterator                                 // function*/yield keywords
	AttrTypeAsync                                    // async/await
	AttrTypeVolatile                                 // volatile memory access
	AttrTypeDeprecated                               // @deprecated markers
	AttrTypeExperimental                             // @experimental markers
	AttrTypePure                                     // pure/const functions
	AttrTypeNoThrow                                  // nothrow/noexcept
	AttrTypeSideEffect                               // has side effects
	AttrTypeRecursive                                // recursive function
	AttrTypeExported                                 // exported/public
	AttrTypeInline                                   // inline directive
	AttrTypeVirtual                                  // virtual method
	AttrTypeAbstract                                 // abstract method
	AttrTypeStatic                                   // static method
	AttrTypeFinal                                    // final/sealed
	AttrTypeConst                                    // const method
	AttrTypeGenerator                                // generator function
	AttrTypeCoroutine                                // coroutine/async generator
)

func (cat ContextAttributeType) String() string {
	switch cat {
	case AttrTypeDirective:
		return "directive"
	case AttrTypeUnsafe:
		return "unsafe"
	case AttrTypeLock:
		return "lock"
	case AttrTypeDecorator:
		return "decorator"
	case AttrTypePragma:
		return "pragma"
	case AttrTypeIterator:
		return "iterator"
	case AttrTypeAsync:
		return "async"
	case AttrTypeVolatile:
		return "volatile"
	case AttrTypeDeprecated:
		return "deprecated"
	case AttrTypeExperimental:
		return "experimental"
	case AttrTypePure:
		return "pure"
	case AttrTypeNoThrow:
		return "nothrow"
	case AttrTypeSideEffect:
		return "side_effect"
	case AttrTypeRecursive:
		return "recursive"
	case AttrTypeExported:
		return "exported"
	case AttrTypeInline:
		return "inline"
	case AttrTypeVirtual:
		return "virtual"
	case AttrTypeAbstract:
		return "abstract"
	case AttrTypeStatic:
		return "static"
	case AttrTypeFinal:
		return "final"
	case AttrTypeConst:
		return "const"
	case AttrTypeGenerator:
		return "generator"
	case AttrTypeCoroutine:
		return "coroutine"
	default:
		return "unknown"
	}
}

// ContextAttribute represents a context-altering attribute that affects code behavior
type ContextAttribute struct {
	Type  ContextAttributeType `json:"type"`
	Value string               `json:"value"` // e.g., "use server", "@deprecated('Use foo instead')"
	Line  int                  `json:"line"`  // Line where attribute appears
}

// TypeParameter represents a generic type parameter (e.g., T in func Foo[T any]())
type TypeParameter struct {
	Name       string `json:"name"`       // e.g., "T", "K", "V"
	Constraint string `json:"constraint"` // e.g., "any", "comparable", "io.Reader"
}

type Symbol struct {
	Name           string
	Type           SymbolType
	FileID         FileID
	Line           int
	Column         int
	EndLine        int
	EndColumn      int
	Attributes     []ContextAttribute // Context-altering attributes
	TypeParameters []TypeParameter    `json:"type_parameters,omitempty"` // Generic type parameters
	Visibility     SymbolVisibility   `json:"visibility,omitempty"`      // Visibility/export status
}

type SymbolType uint8

const (
	SymbolTypeFunction SymbolType = iota
	SymbolTypeClass
	SymbolTypeMethod
	SymbolTypeVariable
	SymbolTypeConstant
	SymbolTypeInterface
	SymbolTypeType
	// Phase 4: New symbol types
	SymbolTypeStruct
	SymbolTypeModule
	SymbolTypeNamespace
	// Phase 5A: C# specific symbol types
	SymbolTypeProperty
	SymbolTypeEvent
	SymbolTypeDelegate
	SymbolTypeEnum
	SymbolTypeRecord
	SymbolTypeOperator
	SymbolTypeIndexer
	// Phase 5B: Kotlin specific symbol types
	SymbolTypeObject
	SymbolTypeCompanion
	SymbolTypeExtension
	SymbolTypeAnnotation
	// Additional symbol types
	SymbolTypeField
	SymbolTypeEnumMember
	// Rust specific symbol types
	SymbolTypeTrait
	SymbolTypeImpl
	// Constructor
	SymbolTypeConstructor
)

func (st SymbolType) String() string {
	switch st {
	case SymbolTypeFunction:
		return "function"
	case SymbolTypeClass:
		return "class"
	case SymbolTypeMethod:
		return "method"
	case SymbolTypeVariable:
		return "variable"
	case SymbolTypeConstant:
		return "constant"
	case SymbolTypeInterface:
		return "interface"
	case SymbolTypeType:
		return "type"
	case SymbolTypeStruct:
		return "struct"
	case SymbolTypeModule:
		return "module"
	case SymbolTypeNamespace:
		return "namespace"
	// Phase 5A: C# specific symbol types
	case SymbolTypeProperty:
		return "property"
	case SymbolTypeEvent:
		return "event"
	case SymbolTypeDelegate:
		return "delegate"
	case SymbolTypeEnum:
		return "enum"
	case SymbolTypeRecord:
		return "record"
	case SymbolTypeOperator:
		return "operator"
	case SymbolTypeIndexer:
		return "indexer"
	// Phase 5B: Kotlin specific symbol types
	case SymbolTypeObject:
		return "object"
	case SymbolTypeCompanion:
		return "companion"
	case SymbolTypeExtension:
		return "extension"
	case SymbolTypeAnnotation:
		return "annotation"
	case SymbolTypeField:
		return "field"
	case SymbolTypeEnumMember:
		return "enum_member"
	case SymbolTypeTrait:
		return "trait"
	case SymbolTypeImpl:
		return "impl"
	case SymbolTypeConstructor:
		return "constructor"
	default:
		return "unknown"
	}
}

type Import struct {
	Path   string
	FileID FileID
	Line   int
}

type BlockBoundary struct {
	Start int
	End   int
	Type  BlockType
	Name  string
	Depth int
}

type BlockType uint8

const (
	BlockTypeFunction BlockType = iota
	BlockTypeClass
	BlockTypeMethod
	BlockTypeInterface
	BlockTypeStruct
	BlockTypeVariable
	BlockTypeBlock
	// Additional block types
	BlockTypeEnum
	BlockTypeTrait
	BlockTypeImpl
	BlockTypeModule
	BlockTypeNamespace
	BlockTypeConstructor
	BlockTypeOther
)

func (b BlockType) String() string {
	switch b {
	case BlockTypeFunction:
		return "function"
	case BlockTypeClass:
		return "class"
	case BlockTypeMethod:
		return "method"
	case BlockTypeInterface:
		return "interface"
	case BlockTypeStruct:
		return "struct"
	case BlockTypeVariable:
		return "variable"
	case BlockTypeBlock:
		return "block"
	case BlockTypeEnum:
		return "enum"
	case BlockTypeTrait:
		return "trait"
	case BlockTypeImpl:
		return "impl"
	case BlockTypeModule:
		return "module"
	case BlockTypeNamespace:
		return "namespace"
	case BlockTypeConstructor:
		return "constructor"
	case BlockTypeOther:
		return "other"
	default:
		return "unknown"
	}
}

type FileInfo struct {
	ID   FileID
	Path string
	// Content is now managed by FileContentStore - access via ID
	LastModified    time.Time
	Checksum        uint64
	FastHash        uint64   // xxhash for quick equality checks (~0.5ns)
	ContentHash     [32]byte // Pre-computed SHA256 hash for cache optimization
	Blocks          []BlockBoundary
	EnhancedSymbols []*EnhancedSymbol // All symbols with relational data (pointers avoid copy overhead)
	Imports         []Import
	References      []Reference           // All references in this file
	ScopeHierarchy  []ScopeInfo           // Complete scope chain for file
	CharMask        EnhancedCharacterMask // Enhanced character mask with ASCII bitmask + Unicode bloom filter

	// Performance optimization: Precomputed line boundaries for O(1) line access
	// LineOffsets[i] contains the byte offset of the start of line i+1
	// Example: LineOffsets = [0, 10, 25] means line 1 starts at 0, line 2 at 10, line 3 at 25
	LineOffsets []int `json:"-"` // Computed during indexing

	// Performance analysis data collected during AST parsing
	// Used for detecting performance anti-patterns (sequential awaits, expensive calls in loops, etc.)
	PerfData []FunctionPerfData `json:"perf_data,omitempty"`

	// Deprecated: Remove after migration to FileContentStore
	Content []byte   `json:"-"` // Will be removed
	Lines   []string `json:"-"` // Will be removed
}

// BloomFilter represents a bloom filter for Unicode characters
type BloomFilter struct {
	bits [1024]uint64 // 64KB bit array (1024 * 64 bits = 65536 bits)
	size uint64
}

// NewBloomFilter creates a new bloom filter
func NewBloomFilter() BloomFilter {
	return BloomFilter{size: 65536}
}

// Add adds a rune to the bloom filter using multiple hash functions
func (bf *BloomFilter) Add(r rune) {
	h1 := bf.hash1(uint32(r))
	h2 := bf.hash2(uint32(r))
	h3 := bf.hash3(uint32(r))

	bf.setBit(h1)
	bf.setBit(h2)
	bf.setBit(h3)
}

// Contains checks if a rune might be in the bloom filter
func (bf *BloomFilter) Contains(r rune) bool {
	h1 := bf.hash1(uint32(r))
	h2 := bf.hash2(uint32(r))
	h3 := bf.hash3(uint32(r))

	return bf.getBit(h1) && bf.getBit(h2) && bf.getBit(h3)
}

func (bf *BloomFilter) setBit(pos uint64) {
	pos = pos % bf.size
	wordIndex := pos / 64
	bitIndex := pos % 64
	bf.bits[wordIndex] |= 1 << bitIndex
}

func (bf *BloomFilter) getBit(pos uint64) bool {
	pos = pos % bf.size
	wordIndex := pos / 64
	bitIndex := pos % 64
	return bf.bits[wordIndex]&(1<<bitIndex) != 0
}

// Simple hash functions for bloom filter
func (bf *BloomFilter) hash1(val uint32) uint64 {
	// FNV-1a hash variant
	return uint64((val * 0x01000193) ^ 0x811c9dc5)
}

func (bf *BloomFilter) hash2(val uint32) uint64 {
	// Simple multiplicative hash
	return uint64(val * 2654435761)
}

func (bf *BloomFilter) hash3(val uint32) uint64 {
	// Another variant
	return uint64((val ^ 0x55555555) * 0x9e3779b9)
}

// EnhancedCharacterMask combines ASCII bitmask with Unicode bloom filter
type EnhancedCharacterMask struct {
	ASCIIMask     [4]uint64   // Fast bitmask for ASCII (0-255)
	UnicodeFilter BloomFilter // Bloom filter for Unicode characters (>255)
	HasUnicode    bool        // Optimization flag
}

// ExtractCharMask creates an enhanced character mask supporting Unicode
func ExtractCharMask(content []byte) EnhancedCharacterMask {
	mask := EnhancedCharacterMask{
		UnicodeFilter: NewBloomFilter(),
		HasUnicode:    false,
	}

	// Process content as UTF-8 runes
	for _, r := range string(content) {
		if r <= 255 {
			// ASCII/Latin-1 range - use bit mask for fast access
			wordIndex := r / 64
			bitIndex := r % 64
			mask.ASCIIMask[wordIndex] |= 1 << bitIndex
		} else {
			// Unicode range - use bloom filter
			mask.UnicodeFilter.Add(r)
			mask.HasUnicode = true
		}
	}

	return mask
}

// HasAllChars checks if the mask contains all characters from the pattern
func (mask EnhancedCharacterMask) HasAllChars(pattern []byte) bool {
	for _, r := range string(pattern) {
		if r <= 255 {
			// Check ASCII bitmask
			wordIndex := r / 64
			bitIndex := r % 64
			if mask.ASCIIMask[wordIndex]&(1<<bitIndex) == 0 {
				return false
			}
		} else {
			// Check Unicode bloom filter
			if !mask.UnicodeFilter.Contains(r) {
				return false
			}
		}
	}

	return true
}

// HasAllCharsIgnoreCase checks if the mask contains all characters from the pattern (case insensitive)
func (mask EnhancedCharacterMask) HasAllCharsIgnoreCase(pattern []byte) bool {
	for _, r := range string(pattern) {
		if r <= 255 {
			// ASCII range - check both cases using bitmask
			hasLower := false
			hasUpper := false

			if r >= 'A' && r <= 'Z' {
				// Check uppercase
				wordIndex := r / 64
				bitIndex := r % 64
				hasUpper = mask.ASCIIMask[wordIndex]&(1<<bitIndex) != 0

				// Check lowercase equivalent
				lowerR := r + 32
				wordIndex = lowerR / 64
				bitIndex = lowerR % 64
				hasLower = mask.ASCIIMask[wordIndex]&(1<<bitIndex) != 0
			} else if r >= 'a' && r <= 'z' {
				// Check lowercase
				wordIndex := r / 64
				bitIndex := r % 64
				hasLower = mask.ASCIIMask[wordIndex]&(1<<bitIndex) != 0

				// Check uppercase equivalent
				upperR := r - 32
				wordIndex = upperR / 64
				bitIndex = upperR % 64
				hasUpper = mask.ASCIIMask[wordIndex]&(1<<bitIndex) != 0
			} else {
				// Non-alphabetic ASCII character, check exact match
				wordIndex := r / 64
				bitIndex := r % 64
				if mask.ASCIIMask[wordIndex]&(1<<bitIndex) == 0 {
					return false
				}
				continue
			}

			// For alphabetic characters, we need either case to be present
			if !hasLower && !hasUpper {
				return false
			}
		} else {
			// Unicode range - check bloom filter (bloom filter doesn't distinguish case easily)
			// For Unicode case handling, we'd need more sophisticated logic
			if !mask.UnicodeFilter.Contains(r) {
				return false
			}
		}
	}

	return true
}

// TrigramType indicates whether a trigram is byte-based or rune-based
type TrigramType uint8

const (
	ByteTrigram TrigramType = iota
	RuneTrigram
)

// EnhancedTrigram represents either a byte-based or rune-based trigram
type EnhancedTrigram struct {
	Type     TrigramType
	ByteHash uint32 // For byte-based trigrams (ASCII content)
	RuneStr  string // For rune-based trigrams (Unicode content)
	RuneHash uint64 // Hash of the rune string for indexing
}

// Hash returns a consistent hash for the trigram regardless of type
func (et EnhancedTrigram) Hash() uint64 {
	if et.Type == ByteTrigram {
		return uint64(et.ByteHash)
	}
	return et.RuneHash
}

// String returns a string representation of the trigram
func (et EnhancedTrigram) String() string {
	if et.Type == ByteTrigram {
		// Convert byte hash back to string for display
		b1 := byte(et.ByteHash >> 16)
		b2 := byte(et.ByteHash >> 8)
		b3 := byte(et.ByteHash)
		return string([]byte{b1, b2, b3})
	}
	return et.RuneStr
}

// NewByteTrigram creates a byte-based trigram
func NewByteTrigram(b1, b2, b3 byte) EnhancedTrigram {
	hash := uint32(b1)<<16 | uint32(b2)<<8 | uint32(b3)
	return EnhancedTrigram{
		Type:     ByteTrigram,
		ByteHash: hash,
	}
}

// NewRuneTrigram creates a rune-based trigram
func NewRuneTrigram(runes string) EnhancedTrigram {
	// Simple hash for rune string
	hash := uint64(0)
	for _, r := range runes {
		hash = hash*31 + uint64(r)
	}
	return EnhancedTrigram{
		Type:     RuneTrigram,
		RuneStr:  runes,
		RuneHash: hash,
	}
}

// Relational Data Types for Enhanced Symbol Tracking

// ReferenceType represents the kind of reference relationship
type ReferenceType uint8

const (
	RefTypeImport ReferenceType = iota
	RefTypeCall
	RefTypeInheritance
	RefTypeAssignment
	RefTypeDeclaration
	RefTypeParameter
	RefTypeReturn
	RefTypeTypeAnnotation
	RefTypeImplements
	RefTypeExtends
	RefTypeUsage
)

func (rt ReferenceType) String() string {
	switch rt {
	case RefTypeImport:
		return "import"
	case RefTypeCall:
		return "call"
	case RefTypeInheritance:
		return "inheritance"
	case RefTypeAssignment:
		return "assignment"
	case RefTypeDeclaration:
		return "declaration"
	case RefTypeParameter:
		return "parameter"
	case RefTypeReturn:
		return "return"
	case RefTypeTypeAnnotation:
		return "type_annotation"
	case RefTypeImplements:
		return "implements"
	case RefTypeExtends:
		return "extends"
	case RefTypeUsage:
		return "usage"
	default:
		return "unknown"
	}
}

// RefStrength represents the coupling strength of a reference
type RefStrength uint8

const (
	RefStrengthTight      RefStrength = iota // Direct dependency
	RefStrengthLoose                         // Indirect usage
	RefStrengthTransitive                    // Through other symbols
)

func (rs RefStrength) String() string {
	switch rs {
	case RefStrengthTight:
		return "tight"
	case RefStrengthLoose:
		return "loose"
	case RefStrengthTransitive:
		return "transitive"
	default:
		return "unknown"
	}
}

// RefQuality represents the confidence level of a reference relationship.
// Higher quality values indicate more certain evidence of the relationship.
const (
	// RefQualityPrecise indicates explicit syntax declaration (e.g., "implements" keyword)
	RefQualityPrecise = "precise"

	// RefQualityAssigned indicates a concrete type was assigned to an interface-typed variable
	// e.g., var w Writer = &File{} - proves File implements Writer
	RefQualityAssigned = "assigned"

	// RefQualityReturned indicates a concrete type was returned from a function with interface return type
	// e.g., func New() Writer { return &File{} } - proves File implements Writer
	RefQualityReturned = "returned"

	// RefQualityCast indicates a type assertion to an interface was found
	// e.g., x.(Writer) - suggests the value implements Writer
	RefQualityCast = "cast"

	// RefQualityHeuristic indicates method signature matching only (no explicit usage evidence)
	// e.g., File has Write() method matching Writer interface - inferred relationship
	RefQualityHeuristic = "heuristic"
)

// RefQualityRank returns a numeric ranking for quality comparison (higher = more confident)
func RefQualityRank(quality string) int {
	switch quality {
	case RefQualityPrecise:
		return 100
	case RefQualityAssigned:
		return 95
	case RefQualityReturned:
		return 90
	case RefQualityCast:
		return 85
	case RefQualityHeuristic:
		return 50
	default:
		return 0
	}
}

// ScopeType represents different levels of code organization
type ScopeType uint8

const (
	ScopeTypeFolder ScopeType = iota
	ScopeTypeFile
	ScopeTypePackage
	ScopeTypeNamespace
	ScopeTypeClass
	ScopeTypeInterface
	ScopeTypeFunction
	ScopeTypeMethod
	ScopeTypeVariable
	ScopeTypeBlock
	ScopeTypeStruct
)

// VariableType represents different classifications of variables
type VariableType uint8

const (
	VariableTypeGlobal    VariableType = iota // Package/global scope
	VariableTypeLocal                         // Function local scope
	VariableTypeParameter                     // Function parameter
	VariableTypeField                         // Class/struct field
	VariableTypeMember                        // Class member (method/property)
	VariableTypeConstant                      // Constant value
)

func (vt VariableType) String() string {
	switch vt {
	case VariableTypeGlobal:
		return "global"
	case VariableTypeLocal:
		return "local"
	case VariableTypeParameter:
		return "parameter"
	case VariableTypeField:
		return "field"
	case VariableTypeMember:
		return "member"
	case VariableTypeConstant:
		return "constant"
	default:
		return "unknown"
	}
}

func (st ScopeType) String() string {
	switch st {
	case ScopeTypeFolder:
		return "folder"
	case ScopeTypeFile:
		return "file"
	case ScopeTypeNamespace:
		return "namespace"
	case ScopeTypeClass:
		return "class"
	case ScopeTypeInterface:
		return "interface"
	case ScopeTypeFunction:
		return "function"
	case ScopeTypeMethod:
		return "method"
	case ScopeTypeVariable:
		return "variable"
	case ScopeTypeBlock:
		return "block"
	default:
		return "unknown"
	}
}

// ScopeInfo represents contextual scope information
type ScopeInfo struct {
	Type       ScopeType          `json:"type"`
	Name       string             `json:"name"`
	FullPath   string             `json:"full_path"`
	StartLine  int                `json:"start_line"`
	EndLine    int                `json:"end_line"`
	Level      int                `json:"level"`
	Language   string             `json:"language"`
	Attributes []ContextAttribute `json:"attributes,omitempty"` // Context attributes for this scope
}

// Reference represents a relationship between symbols
type Reference struct {
	ID             uint64        `json:"id"`
	SourceSymbol   SymbolID      `json:"source_symbol"`
	TargetSymbol   SymbolID      `json:"target_symbol"`
	FileID         FileID        `json:"file_id"`
	Line           int           `json:"line"`
	Column         int           `json:"column"`
	Type           ReferenceType `json:"type"`
	ContextLines   []StringRef   `json:"-"`             // Line references for context (not serialized)
	ScopeContext   []ScopeInfo   `json:"scope_context"` // Scope breadcrumb at reference point
	Strength       RefStrength   `json:"strength"`
	ReferencedName string        `json:"referenced_name"`          // Actual symbol name being referenced (from Tree-sitter AST)
	Quality        string        `json:"quality,omitempty"`        // RefQuality* constants: precise, assigned, returned, cast, heuristic
	Resolved       *bool         `json:"resolved,omitempty"`       // For include/import resolution (nil if not applicable)
	Ambiguous      bool          `json:"ambiguous,omitempty"`      // Multiple possible targets
	Candidates     []string      `json:"candidates,omitempty"`     // Candidate file paths / symbols for ambiguous include
	FailureReason  string        `json:"failure_reason,omitempty"` // If Resolved=false

	// Deprecated
	Context []string `json:"context"` // Will be removed
}

// RefStrengthStats provides breakdown by coupling strength
type RefStrengthStats struct {
	Tight      int `json:"tight"`
	Loose      int `json:"loose"`
	Transitive int `json:"transitive"`
}

// RefCount tracks reference statistics
type RefCount struct {
	IncomingCount int              `json:"incoming_count"`
	OutgoingCount int              `json:"outgoing_count"`
	IncomingFiles []FileID         `json:"incoming_files"`
	OutgoingFiles []FileID         `json:"outgoing_files"`
	ByType        map[string]int   `json:"by_type"`
	Strength      RefStrengthStats `json:"strength"`
}

// RefStats provides reference statistics at multiple scope levels
type RefStats struct {
	FolderLevel   RefCount `json:"folder_level"`
	FileLevel     RefCount `json:"file_level"`
	ClassLevel    RefCount `json:"class_level"`
	FunctionLevel RefCount `json:"function_level"`
	VariableLevel RefCount `json:"variable_level"`
	Total         RefCount `json:"total"`
}

// EnhancedSymbol extends Symbol with relational information
type EnhancedSymbol struct {
	Symbol                   // Base symbol information
	ID           SymbolID    `json:"id"`
	IncomingRefs []Reference `json:"incoming_refs"`     // Symbols that reference this one
	OutgoingRefs []Reference `json:"outgoing_refs"`     // Symbols this one references
	ScopeChain   []ScopeInfo `json:"scope_chain"`       // Complete scope hierarchy
	RefStats     RefStats    `json:"ref_stats"`         // Aggregated reference statistics
	Metrics      interface{} `json:"metrics,omitempty"` // Code quality metrics (SymbolMetrics from analysis package)

	// Enhanced metadata for context analysis - MEMORY-CONSCIOUS VERSION
	// Use string fields but with clear documentation about memory implications
	TypeInfo    string   `json:"type_info,omitempty"`   // Type annotation (e.g., "string", "int", "*User")
	IsMutable   bool     `json:"is_mutable"`            // Whether the symbol is mutable (for variables)
	IsExported  bool     `json:"is_exported"`           // Whether the symbol is public/exported
	Annotations []string `json:"annotations,omitempty"` // Annotations like @lci:labels[tag], etc.
	DocComment  string   `json:"doc_comment,omitempty"` // Documentation comment
	Signature   string   `json:"signature,omitempty"`   // Function/method signature
	Complexity  int      `json:"complexity,omitempty"`  // Cyclomatic complexity for functions

	// Variable-specific metadata - COMPACT representation using bitfields
	VariableType  VariableType `json:"variable_type,omitempty"`  // Classification: global, local, parameter, field
	VariableFlags uint8        `json:"variable_flags,omitempty"` // Bitfield: bit0=const, bit1=static, bit2=pointer, bit3=array, bit4=channel, bit5=interface

	// Function-specific metadata - COMPACT representation
	ParameterCount uint8  `json:"parameter_count,omitempty"` // Number of parameters (0-255)
	FunctionFlags  uint8  `json:"function_flags,omitempty"`  // Bitfield: bit0=async, bit1=generator, bit2=method, bit3=variadic
	ReceiverType   string `json:"receiver_type,omitempty"`   // Method receiver type
}

// Bitfield constants for EnhancedSymbol metadata optimization
// Variable flag bit positions
const (
	VariableFlagConst     = 1 << iota // bit 0: constant variable
	VariableFlagStatic                // bit 1: static member
	VariableFlagPointer               // bit 2: pointer type
	VariableFlagArray                 // bit 3: array/slice type
	VariableFlagChannel               // bit 4: channel type
	VariableFlagInterface             // bit 5: interface type
)

// Function flag bit positions
const (
	FunctionFlagAsync     = 1 << iota // bit 0: async function
	FunctionFlagGenerator             // bit 1: generator function
	FunctionFlagMethod                // bit 2: method vs function
	FunctionFlagVariadic              // bit 3: variadic parameters
)

// Helper methods for variable flag access
func (es *EnhancedSymbol) IsConst() bool {
	return (es.VariableFlags & VariableFlagConst) != 0
}

func (es *EnhancedSymbol) IsStatic() bool {
	return (es.VariableFlags & VariableFlagStatic) != 0
}

func (es *EnhancedSymbol) IsPointer() bool {
	return (es.VariableFlags & VariableFlagPointer) != 0
}

func (es *EnhancedSymbol) IsArray() bool {
	return (es.VariableFlags & VariableFlagArray) != 0
}

func (es *EnhancedSymbol) IsChannel() bool {
	return (es.VariableFlags & VariableFlagChannel) != 0
}

func (es *EnhancedSymbol) IsInterface() bool {
	return (es.VariableFlags & VariableFlagInterface) != 0
}

// Helper methods for function flag access
func (es *EnhancedSymbol) IsAsyncFunc() bool {
	return (es.FunctionFlags & FunctionFlagAsync) != 0
}

func (es *EnhancedSymbol) IsGeneratorFunc() bool {
	return (es.FunctionFlags & FunctionFlagGenerator) != 0
}

func (es *EnhancedSymbol) IsMethodFunc() bool {
	return (es.FunctionFlags & FunctionFlagMethod) != 0
}

func (es *EnhancedSymbol) IsVariadicFunc() bool {
	return (es.FunctionFlags & FunctionFlagVariadic) != 0
}

// EntityID generates a stable, unique identifier for this symbol.
//
// Format: symbol:<type>_<name>:<filename>:<line>:<column>
// Examples:
//
//	symbol:func_main:main.go:71:0           -> Function main
//	symbol:var_config:main.go:25:5          -> Variable config
//	symbol:struct_MyStruct:types.go:16:27   -> Struct MyStruct
//
// This identifier is stable across runs and can be used for navigation,
// cross-references, and drill-down operations. Use with get_object_context
// tool for full symbol details including calls, references, and metrics.
//
// Returns empty string if validation fails - callers should check result
// before using in production code.
func (s *Symbol) EntityID(rootPath string, filePath string) string {
	// BOUNCER PATTERN: Validate inputs at the door
	if s == nil {
		return ""
	}
	// Name is required - symbols without names are invalid
	if s.Name == "" {
		return ""
	}
	// File path is required for locating the entity
	if filePath == "" {
		return ""
	}
	// Line must be positive for valid location
	if s.Line <= 0 {
		return ""
	}

	// Convert symbol type to entity format (e.g., "function" -> "func")
	// FAIL-FAST: Unknown symbol types return empty
	symbolType := s.normalizeTypeForEntityID()
	if symbolType == "" {
		return ""
	}

	filename := extractFilename(filePath)

	return fmt.Sprintf("symbol:%s_%s:%s:%d:%d", symbolType, s.Name, filename, s.Line, s.Column)
}

// normalizeTypeForEntityID converts SymbolType to entity ID format.
// Returns empty string for invalid/unknown types.
func (s *Symbol) normalizeTypeForEntityID() string {
	switch s.Type {
	case SymbolTypeFunction:
		return "func"
	case SymbolTypeClass:
		return "class"
	case SymbolTypeMethod:
		return "method"
	case SymbolTypeStruct:
		return "struct"
	case SymbolTypeInterface:
		return "interface"
	case SymbolTypeVariable:
		return "var"
	case SymbolTypeConstant:
		return "const"
	case SymbolTypeEnum:
		return "enum"
	case SymbolTypeType:
		return "type"
	case SymbolTypeProperty:
		return "property"
	case SymbolTypeField:
		return "field"
	case SymbolTypeModule:
		return "module"
	case SymbolTypeNamespace:
		return "namespace"
	case SymbolTypeOperator:
		return "operator"
	default:
		return ""
	}
}

// extractFilename extracts just the filename from a full path.
// Handles both Unix and Windows path separators.
func extractFilename(filePath string) string {
	if filePath == "" {
		return ""
	}
	// Find last separator (either / or \)
	lastSep := -1
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' || filePath[i] == '\\' {
			lastSep = i
			break
		}
	}
	if lastSep >= 0 && lastSep < len(filePath)-1 {
		return filePath[lastSep+1:]
	}
	return filePath
}

// EntityID generates a unified entity ID for this enhanced symbol
func (es *EnhancedSymbol) EntityID(rootPath string, filePath string) string {
	return es.Symbol.EntityID(rootPath, filePath)
}

// Direct access methods for string fields (no content parameter needed)
func (es *EnhancedSymbol) GetTypeInfo() string {
	return es.TypeInfo
}

func (es *EnhancedSymbol) GetDocComment() string {
	return es.DocComment
}

func (es *EnhancedSymbol) GetSignature() string {
	return es.Signature
}

func (es *EnhancedSymbol) GetReceiverType() string {
	return es.ReceiverType
}

func (es *EnhancedSymbol) GetAnnotations() []string {
	return es.Annotations
}

// RelationType represents how symbols are related
type RelationType uint8

const (
	RelationParent   RelationType = iota // Contains this symbol
	RelationChild                        // Contained by this symbol
	RelationSibling                      // Same scope level
	RelationCaller                       // Calls this symbol
	RelationCallee                       // Called by this symbol
	RelationImporter                     // Imports this symbol
	RelationImportee                     // Imported by this symbol
)

func (rt RelationType) String() string {
	switch rt {
	case RelationParent:
		return "parent"
	case RelationChild:
		return "child"
	case RelationSibling:
		return "sibling"
	case RelationCaller:
		return "caller"
	case RelationCallee:
		return "callee"
	case RelationImporter:
		return "importer"
	case RelationImportee:
		return "importee"
	default:
		return "unknown"
	}
}

// RelatedSymbol represents a symbol related to the current one
type RelatedSymbol struct {
	Symbol   EnhancedSymbol `json:"symbol"`
	Relation RelationType   `json:"relation"`
	Strength RefStrength    `json:"strength"`
	Distance int            `json:"distance"`            // Degrees of separation
	FileName string         `json:"file_name,omitempty"` // File name for display
}

// RelationalContext provides comprehensive relational information for search results
type RelationalContext struct {
	Symbol         EnhancedSymbol  `json:"symbol"`
	RefStats       RefStats        `json:"ref_stats"`
	Breadcrumbs    []ScopeInfo     `json:"breadcrumbs"`      // Scope chain outside search context
	RelatedSymbols []RelatedSymbol `json:"related_symbols"`  // Nearby related symbols
	Function       FunctionContext `json:"function_context"` // Enclosing function/method context
}

// FunctionContext represents the enclosing function for a match
type FunctionContext struct {
	Name        string `json:"name"`
	Qualified   string `json:"qualified"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Known       bool   `json:"known"`    // False if no enclosing function
	Inferred    bool   `json:"inferred"` // True if resolved heuristically, not from scope hierarchy
	Description string `json:"description,omitempty"`
}

// Function Call Tree Types

// NodeType represents different types of code nodes in function trees
type NodeType uint8

const (
	NodeTypeFunction NodeType = iota
	NodeTypeLoop
	NodeTypeCondition
	NodeTypeSwitch
	NodeTypeAsync
	NodeTypeNetwork
	NodeTypeDatabase
	NodeTypeCPUIntensive
	NodeTypeFileIO
	NodeTypeExternal
	NodeTypeRecursive
)

func (nt NodeType) String() string {
	switch nt {
	case NodeTypeFunction:
		return "function"
	case NodeTypeLoop:
		return "loop"
	case NodeTypeCondition:
		return "condition"
	case NodeTypeSwitch:
		return "switch"
	case NodeTypeAsync:
		return "async"
	case NodeTypeNetwork:
		return "network"
	case NodeTypeDatabase:
		return "database"
	case NodeTypeCPUIntensive:
		return "cpu_intensive"
	case NodeTypeFileIO:
		return "file_io"
	case NodeTypeExternal:
		return "external"
	case NodeTypeRecursive:
		return "recursive"
	default:
		return "unknown"
	}
}

// NodeAnnotation represents architectural patterns and characteristics
type NodeAnnotation struct {
	Type        NodeType               `json:"type"`
	Emoji       string                 `json:"emoji"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // iterations, probability, timing, etc.
}

// TreeNode represents a node in the function call hierarchy
type TreeNode struct {
	Name        string           `json:"name"`
	FilePath    string           `json:"file_path"`
	Line        int              `json:"line"`
	Depth       int              `json:"depth"`
	NodeType    NodeType         `json:"node_type"`
	Annotations []NodeAnnotation `json:"annotations"`
	Children    []*TreeNode      `json:"children"`
	Parent      *TreeNode        `json:"-"` // Exclude from JSON to avoid circular references

	// Edit safety and stability indicators for coding agents
	EditRiskScore   int      `json:"edit_risk_score"`  // 0-10 risk of breaking when editing this function
	StabilityTags   []string `json:"stability_tags"`   // CORE, PUBLIC_API, RECURSIVE, SIDE_EFFECTS, FRAGILE
	DependencyCount int      `json:"dependency_count"` // Number of functions this depends on
	DependentCount  int      `json:"dependent_count"`  // Number of functions that depend on this
	ImpactRadius    int      `json:"impact_radius"`    // How many files could be affected by changes
	SafetyNotes     []string `json:"safety_notes"`     // Human-readable safety warnings
}

// FunctionTree represents the complete call hierarchy of a function
type FunctionTree struct {
	RootFunction string      `json:"root_function"`
	Root         *TreeNode   `json:"root"`
	MaxDepth     int         `json:"max_depth"`
	TotalNodes   int         `json:"total_nodes"`
	Options      TreeOptions `json:"options"`
}

// TreeOptions configures tree generation behavior
type TreeOptions struct {
	MaxDepth       int    `json:"max_depth"`
	ShowLines      bool   `json:"show_lines"`
	Compact        bool   `json:"compact"`
	ExcludePattern string `json:"exclude_pattern"`
	AgentMode      bool   `json:"agent_mode"`
}

// String renders the tree as a formatted string with emoji annotations
func (tree *FunctionTree) String() string {
	if tree.Root == nil {
		return "No function tree available\n"
	}
	return tree.renderNode(tree.Root, "", true)
}

// renderNode recursively renders tree nodes with proper indentation and emoji
func (tree *FunctionTree) renderNode(node *TreeNode, prefix string, isLast bool) string {
	if node == nil {
		return ""
	}

	// Determine tree characters
	connector, branch := tree.getTreeConnector(node, isLast)

	// Build node line
	nodeLine := tree.buildNodeLine(node, prefix, connector)

	// Process children
	result := nodeLine + "\n"
	children := node.Children

	for i, child := range children {
		isLastChild := i == len(children)-1
		result += tree.renderNode(child, prefix+branch, isLastChild)
	}

	return result
}

// getTreeConnector determines the tree connector characters
func (tree *FunctionTree) getTreeConnector(node *TreeNode, isLast bool) (string, string) {
	if node.Parent == nil {
		return "", ""
	}
	if isLast {
		return "└─", "  "
	}
	return "├─", "│ "
}

// buildNodeLine constructs the node display line
func (tree *FunctionTree) buildNodeLine(node *TreeNode, prefix, connector string) string {
	nodeLine := prefix + connector

	// Add emoji based on mode
	if tree.Options.AgentMode {
		nodeLine += tree.getAgentModeEmoji(node)
	} else {
		nodeLine += tree.getHumanModeEmoji(node)
	}

	// Add name and metadata
	nodeLine += node.Name
	nodeLine += tree.formatMetadata(node)

	return nodeLine
}

// getAgentModeEmoji returns emoji based on edit risk score
func (tree *FunctionTree) getAgentModeEmoji(node *TreeNode) string {
	switch {
	case node.EditRiskScore >= 8:
		return "🚨 " // High risk - critical dependencies, public APIs
	case node.EditRiskScore >= 6:
		return "⚠️  " // Medium risk - moderate dependencies
	case node.EditRiskScore >= 4:
		return "🔶 " // Low-medium risk - some dependencies
	case node.EditRiskScore >= 2:
		return "🟡 " // Low risk - minimal dependencies
	default:
		return "🟢 " // Safe to edit
	}
}

// getHumanModeEmoji returns emoji based on node type
func (tree *FunctionTree) getHumanModeEmoji(node *TreeNode) string {
	switch node.NodeType {
	case NodeTypeFunction:
		return "→ "
	case NodeTypeLoop:
		return "🔁 "
	case NodeTypeCondition:
		return "❓ "
	case NodeTypeSwitch:
		return "🔀 "
	case NodeTypeAsync:
		return "⚡ "
	case NodeTypeNetwork, NodeTypeDatabase, NodeTypeExternal:
		return "⚠️  "
	default:
		return "→ "
	}
}

// formatMetadata formats node metadata based on mode
func (tree *FunctionTree) formatMetadata(node *TreeNode) string {
	if tree.Options.AgentMode {
		return tree.formatAgentModeMetadata(node)
	}
	return tree.formatHumanModeMetadata(node)
}

// formatAgentModeMetadata formats dense dependency data
func (tree *FunctionTree) formatAgentModeMetadata(node *TreeNode) string {
	nodeLine := ""
	if len(node.StabilityTags) > 0 {
		nodeLine += fmt.Sprintf(" [%s]", strings.Join(node.StabilityTags, ","))
	}
	if node.DependencyCount > 0 || node.DependentCount > 0 {
		nodeLine += fmt.Sprintf(" deps=%d←%d→", node.DependencyCount, node.DependentCount)
	}
	if node.ImpactRadius > 1 {
		nodeLine += fmt.Sprintf(" impact=%d", node.ImpactRadius)
	}
	return nodeLine
}

// formatHumanModeMetadata formats readable safety warnings
func (tree *FunctionTree) formatHumanModeMetadata(node *TreeNode) string {
	nodeLine := ""
	for _, note := range node.SafetyNotes {
		nodeLine += " ⚠️ " + note
	}
	return nodeLine
}

// SearchOptions defines options for searching code
type SearchOptions struct {
	// Basic search options
	CaseInsensitive    bool
	MaxContextLines    int
	MergeFileResults   bool   // Merge multiple results from same file
	EnsureCompleteStmt bool   // Ensure complete statements with comments
	ExcludePattern     string // Regex pattern to exclude files
	IncludePattern     string // Regex pattern to include files (whitelist)

	// Result control
	MaxResults int // Optional cap for number of results to return (0 = no cap)

	// Regex support
	UseRegex bool // Enable regex pattern matching

	// Semantic search filters for AI agents
	SymbolTypes     []string // Filter by symbol types: "function", "variable", "class", "type", "constant"
	DeclarationOnly bool     // Only show symbol definitions, not usages
	UsageOnly       bool     // Only show symbol usages, not definitions
	ExportedOnly    bool     // Only show public/exported symbols
	ExcludeTests    bool     // Exclude test files and test functions
	ExcludeComments bool     // Exclude matches in comments (deprecated: use CodeOnly instead)
	MutableOnly     bool     // Only mutable variables (var, not const)
	GlobalOnly      bool     // Only global/package-level symbols

	// Content-specific filters (powered by AST)
	CommentsOnly    bool // Search only in comments
	CodeOnly        bool // Search only in code (excludes comments and strings)
	StringsOnly     bool // Search only in string literals
	TemplateStrings bool // Include template strings (sql``, gql``, etc.) when StringsOnly is true

	// Context extraction control
	FullFunction     bool // Show complete function body for matches inside functions
	MaxFunctionLines int  // Max lines for function context (0 = unlimited)
	ContextPadding   int  // Extra lines around match when not showing full function

	// Grep-like features (P0 - Critical for LLM use cases)
	InvertMatch     bool     // Inverted match (grep -v): show lines that DON'T match pattern
	Patterns        []string // Multiple patterns with OR logic (grep -e pattern1 -e pattern2)
	CountPerFile    bool     // Return match count per file (grep -c)
	FilesOnly       bool     // Return only filenames with matches (grep -l)
	WordBoundary    bool     // Match whole words only (grep -w)
	MaxCountPerFile int      // Max matches per file (grep -m), 0 = unlimited

	// Additional indexing-specific option
	Verbose bool // Show verbose output
	// Context search option
	IncludeObjectIDs bool // Include compact object IDs for context search (default: true)
}

// SearchStats represents statistical information about search results
type SearchStats struct {
	Pattern          string `json:"pattern"`
	TotalMatches     int    `json:"total_matches"`
	FilesWithMatches int    `json:"files_with_matches"`

	// Distribution across codebase
	FileDistribution map[string]int `json:"file_distribution"` // filepath -> match count
	DirDistribution  map[string]int `json:"dir_distribution"`  // directory -> match count

	// Symbol context
	SymbolTypes     map[string]int `json:"symbol_types"` // "function", "variable", etc -> count
	DefinitionCount int            `json:"definition_count"`
	UsageCount      int            `json:"usage_count"`
	CommentMatches  int            `json:"comment_matches"`

	// Code quality indicators
	TestFileMatches int `json:"test_file_matches"`
	ExportedSymbols int `json:"exported_symbols"`

	// Performance hints
	HotSpots     []HotSpot `json:"hot_spots"` // Files with most matches
	SearchTimeMs int64     `json:"search_time_ms"`
}

// HotSpot represents a file with high match density
type HotSpot struct {
	File       string `json:"file"`
	MatchCount int    `json:"match_count"`
	FirstLine  int    `json:"first_line"`
	LastLine   int    `json:"last_line"`
}

// MultiSearchStats represents statistics for multiple search patterns
type MultiSearchStats struct {
	Patterns []string                `json:"patterns"`
	Results  map[string]*SearchStats `json:"results"`

	// Cross-pattern analysis
	CommonFiles       []string                  `json:"common_files"`  // Files matching all patterns
	CoOccurrence      map[string]map[string]int `json:"co_occurrence"` // Pattern pairs -> count
	TotalSearchTimeMs int64                     `json:"total_search_time_ms"`
}

// Definition represents a symbol definition location
type Definition struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// FilePathIndex supports efficient file path search with glob pattern matching
type FilePathIndex struct {
	// Path segments indexed by depth for efficient glob matching
	// Example: "internal/tui/views/main.go" -> segments[0]["internal"], segments[1]["tui"], segments[2]["views"]
	PathSegments map[int]map[string][]FileID `json:"path_segments"` // depth -> segment -> FileIDs

	// Full paths for exact matching and fallback
	FullPaths map[FileID]string `json:"full_paths"` // FileID -> full path

	// File extensions for filtering
	Extensions map[string][]FileID `json:"extensions"` // extension (e.g., ".go") -> FileIDs

	// Directory hierarchy for efficient directory-based searches
	Directories map[string][]FileID `json:"directories"` // directory path -> FileIDs

	// Base filenames for pattern matching
	BaseNames map[string][]FileID `json:"base_names"` // filename without path -> FileIDs

	// File count for statistics
	TotalFiles int `json:"total_files"`
}

// NewFilePathIndex creates a new empty file path index
func NewFilePathIndex() *FilePathIndex {
	return &FilePathIndex{
		PathSegments: make(map[int]map[string][]FileID),
		FullPaths:    make(map[FileID]string),
		Extensions:   make(map[string][]FileID),
		Directories:  make(map[string][]FileID),
		BaseNames:    make(map[string][]FileID),
		TotalFiles:   0,
	}
}

// FileSearchOptions defines options for file path searching
type FileSearchOptions struct {
	Pattern     string   `json:"pattern"`     // Glob pattern like "internal/tui/*view*.go"
	Type        string   `json:"type"`        // "glob", "regex", or "exact"
	Exclude     []string `json:"exclude"`     // Patterns to exclude
	MaxResults  int      `json:"max_results"` // Maximum results to return (default: 100)
	Extensions  []string `json:"extensions"`  // File extensions to include (e.g., [".go", ".ts"])
	Directories []string `json:"directories"` // Directory patterns to search in
}

// FileSearchResult represents a file found by path search
type FileSearchResult struct {
	FileID      FileID `json:"file_id"`
	Path        string `json:"path"`
	Directory   string `json:"directory"`
	BaseName    string `json:"base_name"`
	Extension   string `json:"extension"`
	Language    string `json:"language"`     // Programming language based on extension
	MatchReason string `json:"match_reason"` // How the pattern matched
}

// ComponentType represents different types of code components for semantic analysis
type ComponentType int

const (
	ComponentTypeUnknown        ComponentType = iota
	ComponentTypeEntryPoint                   // main functions, init blocks, program entry points
	ComponentTypeAPIHandler                   // HTTP handlers, REST endpoints, GraphQL resolvers
	ComponentTypeViewController               // UI components, renderers, views, templates
	ComponentTypeController                   // State management, business logic controllers
	ComponentTypeDataModel                    // Structs, interfaces, schemas, data types
	ComponentTypeConfiguration                // Config files, settings, environment handling
	ComponentTypeTest                         // Test files, test functions, test utilities
	ComponentTypeUtility                      // Helper functions, utilities, shared code
	ComponentTypeService                      // Business logic services, application services
	ComponentTypeRepository                   // Data access layer, repositories, DAOs
	ComponentTypeMiddleware                   // Middleware, interceptors, filters
	ComponentTypeRouter                       // Routing configuration, URL mapping
	ComponentTypeValidator                    // Input validation, data validation
	ComponentTypeSerializer                   // JSON/XML serialization, data transformation
	ComponentTypeDatabase                     // Database migrations, models, queries
	ComponentTypeAuth                         // Authentication, authorization, security
	ComponentTypeLogging                      // Logging utilities, audit trails
	ComponentTypeMetrics                      // Monitoring, metrics, observability
	ComponentTypeWorker                       // Background workers, job processors
	ComponentTypeEvent                        // Event handling, messaging, pub/sub
)

// String returns a string representation of the component type
func (ct ComponentType) String() string {
	switch ct {
	case ComponentTypeEntryPoint:
		return "entry-point"
	case ComponentTypeAPIHandler:
		return "api-handler"
	case ComponentTypeViewController:
		return "view-component"
	case ComponentTypeController:
		return "controller"
	case ComponentTypeDataModel:
		return "data-model"
	case ComponentTypeConfiguration:
		return "configuration"
	case ComponentTypeTest:
		return "test"
	case ComponentTypeUtility:
		return "utility"
	case ComponentTypeService:
		return "service"
	case ComponentTypeRepository:
		return "repository"
	case ComponentTypeMiddleware:
		return "middleware"
	case ComponentTypeRouter:
		return "router"
	case ComponentTypeValidator:
		return "validator"
	case ComponentTypeSerializer:
		return "serializer"
	case ComponentTypeDatabase:
		return "database"
	case ComponentTypeAuth:
		return "auth"
	case ComponentTypeLogging:
		return "logging"
	case ComponentTypeMetrics:
		return "metrics"
	case ComponentTypeWorker:
		return "worker"
	case ComponentTypeEvent:
		return "event"
	default:
		return "unknown"
	}
}

// ComponentInfo represents a detected code component with semantic information
type ComponentInfo struct {
	Type        ComponentType `json:"type"`
	Name        string        `json:"name"`
	FilePath    string        `json:"file_path"`
	FileID      FileID        `json:"file_id"`
	Language    string        `json:"language"`
	Description string        `json:"description"`

	// Symbol information
	Symbols      []Symbol `json:"symbols"`
	Dependencies []string `json:"dependencies"` // Import paths, dependencies

	// Metadata for classification
	Patterns   []string               `json:"patterns"`   // Naming patterns that matched
	Confidence float64                `json:"confidence"` // Classification confidence (0-1)
	Evidence   []string               `json:"evidence"`   // Evidence for classification
	Metadata   map[string]interface{} `json:"metadata"`   // Additional component-specific data
}

// ComponentSearchOptions defines options for finding components
type ComponentSearchOptions struct {
	Types         []ComponentType `json:"types"`          // Component types to find
	Language      string          `json:"language"`       // Specific language filter
	MinConfidence float64         `json:"min_confidence"` // Minimum classification confidence (default: 0.5)
	IncludeTests  bool            `json:"include_tests"`  // Include test components
	MaxResults    int             `json:"max_results"`    // Maximum results to return (default: 100)
}

// PatternVerificationOptions defines options for pattern verification
type PatternVerificationOptions struct {
	Pattern    string   `json:"pattern"`     // Pattern name (built-in) or custom rule
	Scope      string   `json:"scope"`       // File pattern scope for verification
	Severity   []string `json:"severity"`    // Filter by severity levels (error, warning, info)
	MaxResults int      `json:"max_results"` // Maximum violations to return
	ReportMode string   `json:"report_mode"` // 'violations' or 'summary' or 'detailed'
}

// IntentAnalysisOptions configures intent analysis behavior for semantic code understanding
type IntentAnalysisOptions struct {
	Intent          string   `json:"intent"`           // Specific intent to search for (e.g., "size_management", "event_handling")
	Scope           string   `json:"scope"`            // File scope to analyze (glob patterns like "**/ui/**")
	Context         string   `json:"context"`          // Domain context ("ui", "api", "business-logic", "configuration")
	AntiPatterns    []string `json:"anti_patterns"`    // Specific anti-patterns to detect or "*" for all
	MinConfidence   float64  `json:"min_confidence"`   // Minimum confidence threshold (0.0-1.0, default: 0.5)
	IncludeEvidence bool     `json:"include_evidence"` // Include code evidence in results (default: true)
	MaxResults      int      `json:"max_results"`      // Maximum results to return (default: 50)
}

// EntityID generates a stable, unique identifier for this file.
//
// Format: file:<filename>:<relative_path>
// Examples:
//
//	file:main.go:cmd/lci/main.go
//	file:types.go:internal/types/types.go
//
// This identifier is stable across runs and can be used for navigation,
// file lookups, and cross-references. Use with get_object_context
// tool for full file details including symbols, references, and content.
//
// Returns empty string if validation fails - callers should check result
// before using in production code.
func (fi *FileInfo) EntityID(rootPath string) string {
	// BOUNCER PATTERN: Validate inputs at the door
	if fi == nil {
		return ""
	}
	// Path is required - files must have a path
	if fi.Path == "" {
		return ""
	}

	// Make relative path from rootPath
	relPath := fi.makeRelativePath(rootPath)

	// Extract filename from full path
	filename := extractFilename(fi.Path)

	// Normalize path separators to forward slashes for consistency
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	return fmt.Sprintf("file:%s:%s", filename, relPath)
}

// makeRelativePath creates a relative path from the repository root.
// Returns empty string if inputs are invalid.
func (fi *FileInfo) makeRelativePath(rootPath string) string {
	if rootPath == "" {
		return fi.Path
	}
	relPath := strings.TrimPrefix(fi.Path, rootPath)
	relPath = strings.TrimPrefix(relPath, "/")
	relPath = strings.TrimPrefix(relPath, "\\")
	return relPath
}

// EntityID generates a stable, unique identifier for this reference.
//
// Format: reference:<type>_<symbol_name>:<filename>:<line>:<column>
// Examples:
//
//	reference:call_CalculateMetrics:test.go:123:45
//	reference:use_config:main.go:45:12
//
// This identifier is stable across runs and can be used for navigation,
// reference lookups, and call graph analysis. Use with get_object_context
// tool for full reference details including source and target entities.
//
// Returns empty string if validation fails - callers should check result
// before using in production code.
func (r *Reference) EntityID(rootPath string, filePath string) string {
	// BOUNCER PATTERN: Validate inputs at the door
	if r == nil {
		return ""
	}
	// ReferencedName is required - references must point to something
	if r.ReferencedName == "" {
		return ""
	}
	// File path is required for locating the reference
	if filePath == "" {
		return ""
	}
	// Line must be positive for valid location
	if r.Line <= 0 {
		return ""
	}

	// Convert reference type to entity format
	refType := r.normalizeTypeForEntityID()
	if refType == "" {
		return ""
	}

	filename := extractFilename(filePath)

	return fmt.Sprintf("reference:%s_%s:%s:%d:%d", refType, r.ReferencedName, filename, r.Line, r.Column)
}

// normalizeTypeForEntityID converts RefType to entity ID format.
// Returns empty string for invalid/unknown types.
func (r *Reference) normalizeTypeForEntityID() string {
	switch r.Type {
	case RefTypeCall:
		return "call"
	case RefTypeUsage:
		return "use"
	case RefTypeImport:
		return "import"
	case RefTypeInheritance:
		return "inherit"
	case RefTypeAssignment:
		return "assign"
	case RefTypeDeclaration:
		return "declare"
	case RefTypeParameter:
		return "param"
	case RefTypeReturn:
		return "return"
	case RefTypeTypeAnnotation:
		return "type"
	case RefTypeImplements:
		return "impl"
	case RefTypeExtends:
		return "extend"
	default:
		return ""
	}
}

// ComputeLineOffsets computes byte offsets for the start of each line in the content.
// This enables O(1) line extraction instead of O(n) string splitting.
// LineOffsets[i] contains the byte offset where line i+1 starts.
// Example: for content "hello\nworld\n", returns [0, 6, 12]
func ComputeLineOffsets(content []byte) []int {
	if len(content) == 0 {
		return []int{0}
	}

	// OPTIMIZATION: Two-pass approach to eliminate append() reallocations
	// Pass 1: Count newlines to determine exact size needed
	newlineCount := 0
	for _, b := range content {
		if b == '\n' {
			newlineCount++
		}
	}

	// Pass 2: Pre-allocate with exact capacity and populate
	offsets := make([]int, 1, newlineCount+1) // +1 for initial offset at 0
	offsets[0] = 0                            // Line 1 always starts at byte 0
	for i, b := range content {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// GetLineFromOffsets extracts a single line using precomputed offsets.
// Returns the line content (without trailing newline) or empty slice if line is out of range.
func GetLineFromOffsets(content []byte, lineOffsets []int, lineNum int) []byte {
	if lineNum < 1 || len(lineOffsets) == 0 {
		return nil
	}

	lineIdx := lineNum - 1 // Convert 1-based to 0-based
	if lineIdx >= len(lineOffsets) {
		return nil
	}

	start := lineOffsets[lineIdx]
	end := len(content)

	// Find end of line (next line's start, minus newline char)
	if lineIdx+1 < len(lineOffsets) {
		end = lineOffsets[lineIdx+1]
		// Exclude the newline character
		if end > start && content[end-1] == '\n' {
			end--
		}
	}

	if start >= len(content) {
		return nil
	}
	if end > len(content) {
		end = len(content)
	}

	return content[start:end]
}

// ============================================================================
// Performance Analysis Types
// ============================================================================
// These types store performance-related data collected during AST parsing
// for use in performance anti-pattern detection.

// FunctionPerfData holds performance analysis data for a single function
// Collected during AST parsing and stored in FileInfo for later analysis
type FunctionPerfData struct {
	Name      string      `json:"name"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	IsAsync   bool        `json:"is_async"`
	Language  string      `json:"language"`
	Loops     []LoopData  `json:"loops,omitempty"`
	Awaits    []AwaitData `json:"awaits,omitempty"`
	Calls     []CallData  `json:"calls,omitempty"`
}

// LoopData tracks information about a loop for performance analysis
type LoopData struct {
	NodeType  string `json:"node_type"` // "for_statement", "while_statement", etc.
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Depth     int    `json:"depth"` // Nesting depth (1 = top-level loop)
}

// AwaitData tracks information about an await expression
type AwaitData struct {
	Line        int      `json:"line"`
	AssignedVar string   `json:"assigned_var,omitempty"` // Variable receiving the result
	CallTarget  string   `json:"call_target,omitempty"`  // Function/method being awaited
	UsedVars    []string `json:"used_vars,omitempty"`    // Variables referenced in arguments
}

// CallData tracks information about a function call for performance analysis
type CallData struct {
	Target    string `json:"target"` // Function/method name being called
	Line      int    `json:"line"`
	InLoop    bool   `json:"in_loop"`    // Whether this call is inside a loop
	LoopDepth int    `json:"loop_depth"` // Depth of containing loop (0 if not in loop)
	LoopLine  int    `json:"loop_line"`  // Line of containing loop
}
