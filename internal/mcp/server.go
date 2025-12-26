package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// UnknownField represents an unknown field that was passed but not recognized
type UnknownField struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// UnmarshalJSON implements custom unmarshaling that accepts unknown fields
// and handles field name aliases for backward compatibility
func (s *SearchParams) UnmarshalJSON(data []byte) error {
	type Alias SearchParams // Type alias to avoid recursion

	// Define known fields (both new and legacy names)
	knownFields := map[string]struct{}{
		// New consolidated field names
		"pattern": {}, "output": {}, "max": {}, "filter": {},
		"flags": {}, "include": {}, "symbol_types": {},
		"patterns": {}, "max_per_file": {}, "semantic": {},
		"warnings": {},

		// Legacy field names (for backward compatibility)
		"output_size": {}, "output_mode": {}, "max_line_count": {},
		"max_results": {}, "type": {}, "max_count_per_file": {},
		"include_breadcrumbs": {}, "include_safety": {}, "include_references": {},
		"include_dependencies": {}, "include_ids": {}, "case_insensitive": {},
		"use_regex": {}, "exclude": {}, "declaration_only": {},
		"usage_only": {}, "exported_only": {}, "exclude_tests": {},
		"exclude_comments": {}, "invert_match": {}, "count_per_file": {},
		"files_only": {}, "word_boundary": {}, "disable_semantic": {},
	}

	raw, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}

	// Handle field name aliases - normalize legacy names to new names
	normalizedData := make(map[string]json.RawMessage)
	for key, value := range raw {
		switch key {
		case "type":
			normalizedData["filter"] = value
		case "max_results":
			normalizedData["max"] = value
		case "output_size":
			// Parse output_size and convert to new output format
			var outputSize string
			json.Unmarshal(value, &outputSize)
			// Convert "single-line" to "line", "context" to "ctx", etc.
			normalizedOutput := convertOutputSizeToOutput(outputSize)
			if normalizedOutput != "" {
				normalizedData["output"] = json.RawMessage(`"` + normalizedOutput + `"`)
			}
		case "output_mode":
			normalizedData["output"] = value
		case "max_line_count":
			// Parse max_line_count and convert to output format like "ctx:5"
			var maxLineCount int
			json.Unmarshal(value, &maxLineCount)
			if maxLineCount > 0 {
				ctxFormat := fmt.Sprintf("ctx:%d", maxLineCount)
				normalizedData["output"] = json.RawMessage(`"` + ctxFormat + `"`)
			}
		case "include_breadcrumbs":
			normalizedData["include"] = mergeIncludeField(normalizedData, value, "breadcrumbs")
		case "include_safety":
			normalizedData["include"] = mergeIncludeField(normalizedData, value, "safety")
		case "include_references":
			normalizedData["include"] = mergeIncludeField(normalizedData, value, "refs")
		case "include_dependencies":
			normalizedData["include"] = mergeIncludeField(normalizedData, value, "deps")
		case "include_ids":
			normalizedData["include"] = mergeIncludeField(normalizedData, value, "ids")
		case "case_insensitive":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "ci")
		case "use_regex":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "rx")
		case "exclude_tests":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "nt")
		case "exclude_comments":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "nc")
		case "invert_match":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "iv")
		case "word_boundary":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "wb")
		case "declaration_only":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "dl")
		case "usage_only":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "ul")
		case "exported_only":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "eo")
		case "files_only":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "fo")
		case "count_per_file":
			normalizedData["flags"] = mergeFlagField(normalizedData, value, "cf")
		case "disable_semantic":
			normalizedData["semantic"] = value
		default:
			normalizedData[key] = value
		}
	}

	// Encode normalized data
	normalizedJSON, _ := json.Marshal(normalizedData)

	// Decode into alias type (won't call UnmarshalJSON again)
	aux := (*Alias)(s)
	if err := json.Unmarshal(normalizedJSON, aux); err != nil {
		return err
	}

	// Set warnings manually
	s.Warnings = warnings

	return nil
}

// convertOutputSize converts legacy output_size values to new format
func convertOutputSizeToOutput(outputSize string) string {
	switch strings.ToLower(outputSize) {
	case "single-line", "single", "line":
		return "line"
	case "context":
		return "ctx"
	case "full":
		return "full"
	case "files", "files_with_matches":
		return "files"
	case "count":
		return "count"
	default:
		return ""
	}
}

// mergeIncludeField merges a boolean include field into the include string
func mergeIncludeField(normalizedData map[string]json.RawMessage, value json.RawMessage, fieldName string) json.RawMessage {
	var includeValue bool
	json.Unmarshal(value, &includeValue)

	if !includeValue {
		return normalizedData["include"]
	}

	// Get existing include value
	var existingInclude string
	if includeData, ok := normalizedData["include"]; ok {
		json.Unmarshal(includeData, &existingInclude)
	}

	// Add field if not already present
	includeFields := strings.Split(existingInclude, ",")
	hasField := false
	for _, field := range includeFields {
		if strings.TrimSpace(field) == fieldName {
			hasField = true
			break
		}
	}

	if !hasField {
		if existingInclude == "" {
			existingInclude = fieldName
		} else {
			existingInclude = existingInclude + "," + fieldName
		}
	}

	return json.RawMessage(`"` + existingInclude + `"`)
}

// mergeFlagField merges a boolean flag into the flags string
func mergeFlagField(normalizedData map[string]json.RawMessage, value json.RawMessage, flagName string) json.RawMessage {
	var flagValue bool
	json.Unmarshal(value, &flagValue)

	if !flagValue {
		return normalizedData["flags"]
	}

	// Get existing flags value
	var existingFlags string
	if flagsData, ok := normalizedData["flags"]; ok {
		json.Unmarshal(flagsData, &existingFlags)
	}

	// Add flag if not already present
	flagList := strings.Split(existingFlags, ",")
	hasFlag := false
	for _, flag := range flagList {
		if strings.TrimSpace(flag) == flagName {
			hasFlag = true
			break
		}
	}

	if !hasFlag {
		if existingFlags == "" {
			existingFlags = flagName
		} else {
			existingFlags = existingFlags + "," + flagName
		}
	}

	return json.RawMessage(`"` + existingFlags + `"`)
}

// Output defines the level of detail in search results
type Output string

const (
	OutputSingleLine Output = "single-line" // Only the matched line
	OutputContext    Output = "context"     // Matched line with some context
	OutputFull       Output = "full"        // Complete context with all metadata
)

// SearchParams - Lean but LLM-friendly parameter structure
// Supports both old verbose format AND new concise format via custom unmarshaler
// Unknown fields are tracked in Warnings array to help LLMs learn better format
type SearchParams struct {
	// Core parameters
	Pattern string `json:"pattern"`       // Required: search pattern
	Max     int    `json:"max,omitempty"` // Max results (default: 100)

	// Output control - flexible format
	// Accepts: "line", "ctx:3", "ctx", "full", "files", "count"
	Output string `json:"output,omitempty"`

	// File filters consolidated
	// Examples: "go,*.py" (types and patterns), "*.go" (glob)
	Filter string `json:"filter,omitempty"`

	// Search modifiers (comma-separated, ultra-short)
	// Flags: "ci" (case-insensitive), "rx" (regex), "iv" (invert),
	//        "wb" (word-boundary), "nt" (no-tests), "nc" (no-comments)
	Flags string `json:"flags,omitempty"`

	// Context add-ons (comma-separated)
	// Examples: "breadcrumbs,safety,refs,deps,ids"
	Include string `json:"include,omitempty"`

	// Symbol type filter (comma-separated, backwards compat)
	// Examples: "func,class,var,type,method"
	SymbolTypes string `json:"symbol_types,omitempty"`

	// Multiple patterns (OR logic)
	Patterns string `json:"patterns,omitempty"`

	// Max per file (grep -m equivalent)
	MaxPerFile int `json:"max_per_file,omitempty"`

	// Semantic search toggle (default: true - enabled)
	Semantic bool `json:"semantic,omitempty"`

	// Language filter - scope search to specific programming languages
	// Examples: ["go"], ["typescript", "javascript"], ["csharp"]
	// Language names are case-insensitive and support aliases (e.g., 'ts' for TypeScript)
	Languages []string `json:"languages,omitempty"`

	// LLM-friendly: Track unknown fields for learning
	Warnings []UnknownField `json:"warnings,omitempty"`
}

// SearchResponse represents the new flexible search response
type SearchResponse struct {
	Results      []CompactSearchResult `json:"results"`
	TotalMatches int                   `json:"total_matches"` // Total matches found before limit applied
	Showing      int                   `json:"showing"`       // Number of results in this response
	MaxResults   int                   `json:"max_results"`   // Limit that was applied (default: 50, max: 100)
}

// FilesOnlyResponse returns just unique file paths - minimal token usage
type FilesOnlyResponse struct {
	Files        []string `json:"files"`
	TotalMatches int      `json:"total_matches"`
	UniqueFiles  int      `json:"unique_files"`
}

// CountOnlyResponse returns just the count - absolute minimal output
type CountOnlyResponse struct {
	TotalMatches int `json:"total_matches"`
	UniqueFiles  int `json:"unique_files"`
}

// CompactSearchResult represents a minimal search result with unique IDs
type CompactSearchResult struct {
	// Unique identifiers
	ResultID string `json:"result_id"` // Unique ID for this result (for context lookup)
	ObjectID string `json:"object_id"` // CompositeSymbolID for the matched symbol

	// Basic location information
	File   string  `json:"file"`
	Line   int     `json:"line"`
	Column int     `json:"column"`
	Match  string  `json:"match"`
	Score  float64 `json:"score"`

	// Symbol information
	SymbolType string `json:"symbol_type,omitempty"` // "function", "class", "variable", "type", etc.
	SymbolName string `json:"symbol_name,omitempty"` // Name of the symbol
	IsExported bool   `json:"is_exported,omitempty"` // Whether symbol is public/exported

	// Grep feature fields
	FileMatchCount int `json:"file_match_count,omitempty"` // Total matches in this file (for count_per_file mode)

	// Context (controlled by Output and MaxLineCount)
	ContextLines []string `json:"context_lines,omitempty"` // Only included if Output != "single-line"

	// Optional context elements (only included if requested)
	Breadcrumbs  []ScopeBreadcrumb `json:"breadcrumbs,omitempty"`  // Included if IncludeBreadcrumbs=true
	Safety       *SafetyInfo       `json:"safety,omitempty"`       // Included if IncludeSafety=true
	References   *ReferenceInfo    `json:"references,omitempty"`   // Included if IncludeReferences=true
	Dependencies []string          `json:"dependencies,omitempty"` // Included if IncludeDependencies=true
}

// SafetyInfo contains AI safety annotations for a result
type SafetyInfo struct {
	EditSafety      string  `json:"edit_safety"`                // "safe", "caution", "dangerous"
	SafetyReason    string  `json:"safety_reason,omitempty"`    // Explanation of safety level
	ComplexityScore float64 `json:"complexity_score,omitempty"` // Complexity score
}

// ReferenceInfo contains reference statistics for a result
type ReferenceInfo struct {
	IncomingCount int `json:"incoming_count"` // Number of references to this symbol
	OutgoingCount int `json:"outgoing_count"` // Number of references from this symbol
}

// ContextResponse represents a response containing multiple object contexts
type ContextResponse struct {
	Contexts []ObjectContext `json:"contexts"`
	Count    int             `json:"count"`
}

// ObjectContext represents a simplified context object for compact formatting
type ObjectContext struct {
	FilePath   string   `json:"file_path"`
	Line       int      `json:"line"`
	ObjectID   string   `json:"object_id"`
	SymbolType string   `json:"symbol_type"`
	SymbolName string   `json:"symbol_name"`
	IsExported bool     `json:"is_exported"`
	Signature  string   `json:"signature,omitempty"`
	Definition string   `json:"definition,omitempty"`
	Context    []string `json:"context,omitempty"`
}

// ObjectContextParams represents parameters for the get_object_context tool
type ObjectContextParams struct {
	// Primary: Use 'id' with concise object IDs from search results (e.g., "VE" or "VE,tG,Ab")
	ID string `json:"id,omitempty"` // Concise object ID(s) - comma-separated for multiple

	// Alternative identification by name
	Name   string `json:"name,omitempty"`    // Symbol name for direct lookup
	FileID int    `json:"file_id,omitempty"` // File ID to narrow lookup scope (optional with name)
	Line   int    `json:"line,omitempty"`    // Line number (optional, helps with disambiguation)
	Column int    `json:"column,omitempty"`  // Column number (optional)

	// Mode parameter for consolidated tool (from context_lookup)
	Mode string `json:"mode,omitempty"` // "full", "quick", "relationships", "semantic", "usage", "variables" (default: "full")

	// Context options (all true by default for full context)
	IncludeFullSymbol     bool `json:"include_full_symbol,omitempty"`     // Complete symbol information
	IncludeCallHierarchy  bool `json:"include_call_hierarchy,omitempty"`  // Call graph data
	IncludeAllReferences  bool `json:"include_all_references,omitempty"`  // All reference locations
	IncludeDependencies   bool `json:"include_dependencies,omitempty"`    // Dependency analysis
	IncludeFileContext    bool `json:"include_file_context,omitempty"`    // File-level context
	IncludeQualityMetrics bool `json:"include_quality_metrics,omitempty"` // Code quality information

	// Additional context options (from context_lookup)
	MaxDepth            int     `json:"max_depth,omitempty"`            // Maximum depth for relationship analysis (default: 5)
	IncludeAIText       bool    `json:"include_ai_text,omitempty"`      // Include AI-generated descriptions (default: true)
	ConfidenceThreshold float64 `json:"confidence_threshold,omitempty"` // Minimum confidence for results (default: 0.3)

	// Filters (from context_lookup)
	ExcludeTestFiles bool     `json:"exclude_test_files,omitempty"` // Exclude test files from analysis
	IncludeSections  []string `json:"include_sections,omitempty"`   // Sections to include
	ExcludeSections  []string `json:"exclude_sections,omitempty"`   // Sections to exclude

	// Captures unknown fields as warnings
	Warnings []UnknownField `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling that accepts unknown fields
func (o *ObjectContextParams) UnmarshalJSON(data []byte) error {
	type Alias ObjectContextParams // Type alias to avoid recursion

	// Define known fields
	knownFields := map[string]struct{}{
		"id": {}, "file_id": {}, "name": {}, "line": {}, "column": {}, "mode": {},
		"include_full_symbol": {}, "include_call_hierarchy": {},
		"include_all_references": {}, "include_dependencies": {},
		"include_file_context": {}, "include_quality_metrics": {},
		"max_depth": {}, "include_ai_text": {}, "confidence_threshold": {},
		"exclude_test_files": {}, "include_sections": {}, "exclude_sections": {},
	}

	// Collect unknown fields via shared helper
	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}

	// Now unmarshal into the actual struct
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*o = ObjectContextParams(alias)
	o.Warnings = warnings
	return nil
}

// CodeContext provides essential context about current code location for AI editing
type CodeContext struct {
	// Current Location Context
	CurrentFile     string `json:"current_file"`
	CurrentSymbol   string `json:"current_symbol"`
	CurrentLine     int    `json:"current_line"`
	CurrentFunction string `json:"current_function"`

	// Structural Breadcrumbs (containing scopes)
	ContainingScopes []ScopeBreadcrumb `json:"containing_scopes"`

	// Language-Specific Compiler Context
	CompilerFlags CompilerContext `json:"compiler_flags"`

	// Import/Export Context
	ImportContext ImportContext `json:"import_context"`

	// Dependency Impact Analysis
	DependencyImpact DependencyImpact `json:"dependency_impact"`

	// Code Quality Context
	QualityMetrics CodeQualityMetrics `json:"quality_metrics"`

	// Last Updated
	LastUpdated time.Time `json:"last_updated"`
}

// ScopeBreadcrumb represents a containing scope (namespace, class, function, block)
type ScopeBreadcrumb struct {
	ScopeType  string `json:"scope_type"` // "namespace", "class", "function", "method", "block"
	Name       string `json:"name"`       // scope name
	StartLine  int    `json:"start_line"` // where scope starts
	EndLine    int    `json:"end_line"`   // where scope ends
	Language   string `json:"language"`   // programming language
	Visibility string `json:"visibility"` // "public", "private", "protected", "internal"
}

// CompilerContext contains language-specific compiler flags and directives
type CompilerContext struct {
	Language      string            `json:"language"`
	StrictMode    bool              `json:"strict_mode"`    // "use strict" in JS, strict mode in others
	ServerSide    bool              `json:"server_side"`    // "use server" in Next.js, server-side context
	BuildTags     []string          `json:"build_tags"`     // Go build tags
	CompilerFlags []string          `json:"compiler_flags"` // Language-specific compiler directives
	ModuleType    string            `json:"module_type"`    // "esm", "commonjs", "umd", "go module", etc.
	Encoding      string            `json:"encoding"`       // File encoding (Python, etc.)
	Pragmas       map[string]string `json:"pragmas"`        // JSX pragma, other language pragmas
}

// ImportContext contains information about imports and exports for current file
type ImportContext struct {
	Imports         []ImportInfo `json:"imports"`           // All imports in current file
	Exports         []ExportInfo `json:"exports"`           // All exports in current file
	ModulePath      string       `json:"module_path"`       // Current module/package path
	HasCircularDeps bool         `json:"has_circular_deps"` // Circular dependency warning
	UnusedImports   []string     `json:"unused_imports"`    // Potentially unused imports
}

// ImportInfo represents a single import statement
type ImportInfo struct {
	ImportPath    string   `json:"import_path"`    // Path being imported
	ImportedNames []string `json:"imported_names"` // Names being imported
	ImportAlias   string   `json:"import_alias"`   // Alias if any
	ImportType    string   `json:"import_type"`    // "default", "named", "namespace", "side-effect"
	IsTypeOnly    bool     `json:"is_type_only"`   // TypeScript type-only import
	Line          int      `json:"line"`           // Line number of import
}

// ExportInfo represents a single export statement
type ExportInfo struct {
	ExportedName string `json:"exported_name"` // Name being exported
	ExportType   string `json:"export_type"`   // "default", "named", "re-export"
	OriginalName string `json:"original_name"` // Original name if renamed
	IsTypeOnly   bool   `json:"is_type_only"`  // TypeScript type-only export
	Line         int    `json:"line"`          // Line number of export
}

// DependencyImpact analyzes how changes to current code affect other parts
type DependencyImpact struct {
	IncomingRefs    int            `json:"incoming_refs"`     // How many places reference this
	OutgoingRefs    int            `json:"outgoing_refs"`     // How many places this references
	ImportedByFiles []string       `json:"imported_by_files"` // Files that import this
	BreakingRisk    string         `json:"breaking_risk"`     // "low", "medium", "high"
	UsageHotspots   []UsageHotspot `json:"usage_hotspots"`    // High-usage areas
}

// UsageHotspot represents a high-usage area that could be affected by changes
type UsageHotspot struct {
	FilePath     string `json:"file_path"`
	FunctionName string `json:"function_name"`
	UsageCount   int    `json:"usage_count"`
	UsageType    string `json:"usage_type"` // "call", "import", "inheritance", "reference"
}

// CodeQualityMetrics provides code quality context for the current location
type CodeQualityMetrics struct {
	CyclomaticComplexity int       `json:"cyclomatic_complexity"`
	CognitiveComplexity  int       `json:"cognitive_complexity"`
	LinesOfCode          int       `json:"lines_of_code"`
	TechnicalDebtScore   float64   `json:"technical_debt_score"`
	LastModified         time.Time `json:"last_modified"`
	HasDocumentation     bool      `json:"has_documentation"`
	HasTests             bool      `json:"has_tests"`
	SecurityAnnotations  []string  `json:"security_annotations"` // Security-related notes
}

// NavigationPattern represents common navigation sequences
type NavigationPattern struct {
	Pattern    []string  `json:"pattern"`     // sequence of operations
	Frequency  int       `json:"frequency"`   // how often this pattern occurs
	LastSeen   time.Time `json:"last_seen"`   // when last observed
	Confidence float64   `json:"confidence"`  // confidence score 0-1
	NextLikely []string  `json:"next_likely"` // most likely next operations
}

// PredictiveContext extends navigation context with pattern analysis
type PredictiveContext struct {
	CommonPatterns    []NavigationPattern `json:"common_patterns"`
	SessionFrequency  map[string]int      `json:"session_frequency"`   // operation frequency in current session
	SymbolAffinities  map[string][]string `json:"symbol_affinities"`   // symbols often explored together
	TimeBasedPatterns map[string]float64  `json:"time_based_patterns"` // operations by time of day
}

// AsyncIndexingState tracks the state of background indexing operations
type AsyncIndexingState struct {
	Status          string                   `json:"status"`   // "estimating", "indexing", "completed", "cancelled", "failed"
	Progress        float64                  `json:"progress"` // 0.0 to 1.0
	StartTime       time.Time                `json:"start_time"`
	RootPath        string                   `json:"root_path"`
	IndexedFiles    int                      `json:"indexed_files"`
	TotalFiles      int                      `json:"total_files"`
	CurrentFile     string                   `json:"current_file"`
	EstimatedSizeMB int64                    `json:"estimated_size_mb"`
	ErrorMessage    string                   `json:"error_message,omitempty"`
	CanCancel       bool                     `json:"can_cancel"`
	SessionID       string                   `json:"session_id"`
	UpdateChannel   chan AsyncIndexingUpdate `json:"-"`
	CancelChannel   chan bool                `json:"-"`
	CompletionTime  *time.Time               `json:"completion_time,omitempty"`
}

// AsyncIndexingUpdate represents a progress update during async indexing
type AsyncIndexingUpdate struct {
	Type            string    `json:"type"` // "progress", "completion", "error", "cancellation"
	Progress        float64   `json:"progress"`
	IndexedFiles    int       `json:"indexed_files"`
	TotalFiles      int       `json:"total_files"`
	CurrentFile     string    `json:"current_file"`
	Message         string    `json:"message"`
	ElapsedTime     float64   `json:"elapsed_time_s"`
	EstimatedRemain float64   `json:"estimated_remain_s"`
	Timestamp       time.Time `json:"timestamp"`
}

type Server struct {
	goroutineIndex          *indexing.MasterIndex // Direct access to MasterIndex
	ownsIndex               bool                  // True if server created the index and should close it
	cfg                     *config.Config
	server                  *mcp.Server
	diagnosticLogger        *DiagnosticLogger               // CRITICAL: File-based logging only (no stdout/stderr)
	codeContext             *CodeContext                    // Current code location context for AI editing
	predictiveContext       *PredictiveContext              // Predictive navigation patterns
	contextLookupEngine     *core.ContextLookupEngine       // Code object context lookup
	searchCoordinator       *search.SearchCoordinator       // Concurrent search coordination
	semanticScorer          *semantic.SemanticScorer        // Unified semantic scoring engine (Phase 3E)
	optimizedSemanticSearch *search.OptimizedSemanticSearch // Optimized semantic search with pre-computed indexes

	// Code insight analysis components
	componentDetector      *core.ComponentDetector           // Component detection and classification
	metricsCalculator      *analysis.CachedMetricsCalculator // Code metrics calculation
	relationshipAnalyzer   *analysis.RelationshipAnalyzer    // Relationship analysis
	analysisComponentsOnce sync.Once                         // Ensures thread-safe initialization of analysis components

	// Auto-indexing manager with proper synchronization
	autoIndexManager *AutoIndexingManager

	// Profiling and metrics
	profilingMetrics *ProfilingMetrics

	// Health monitoring

	// Async indexing state
	indexingState *AsyncIndexingState
	indexingMutex sync.RWMutex
}

// FileSearchParams for intuitive file discovery with path patterns
type FileSearchParams struct {
	Pattern      string   `json:"pattern"`                 // Glob pattern: "internal/tui/*view*.go"
	PatternType  string   `json:"pattern_type,omitempty"`  // "glob" (default), "regex", "prefix", "suffix"
	Exclude      []string `json:"exclude,omitempty"`       // ["**/node_modules/**", "**/vendor/**"]
	Extensions   []string `json:"extensions,omitempty"`    // [".go", ".js", ".ts"] - shorthand filter
	Max          int      `json:"max_results,omitempty"`   // Default: 100
	ShowContent  bool     `json:"show_content,omitempty"`  // Include first few lines as preview
	SortBy       string   `json:"sort_by,omitempty"`       // "name" (default), "size", "modified", "type"
	IncludeStats bool     `json:"include_stats,omitempty"` // Include file size, modification time
}

type SymbolParams struct {
	Symbol string `json:"symbol"`
}

type TreeParams struct {
	Function  string `json:"function"`
	MaxDepth  int    `json:"max_depth,omitempty"`
	ShowLines bool   `json:"show_lines,omitempty"`
	Compact   bool   `json:"compact,omitempty"`
	Exclude   string `json:"exclude,omitempty"`
	Agent     bool   `json:"agent,omitempty"`
}

type RelationParams struct {
	Symbol        string   `json:"symbol"`
	MaxDepth      int      `json:"max_depth,omitempty"`
	RelationTypes []string `json:"relation_types,omitempty"`
	ShowContext   bool     `json:"show_context,omitempty"`
	IncludeUsage  bool     `json:"include_usage,omitempty"`
}

// FindComponentsParams for semantic component discovery
// UnifiedFileParams consolidates file_search, find_important_files, and find_components functionality
type UnifiedFileParams struct {
	// Search mode options
	ImportantFiles bool `json:"important_files,omitempty"` // Find key project files (package.json, Dockerfile, etc.)
	Components     bool `json:"components,omitempty"`      // Discover semantic components and entry points

	// File search options (when neither mode is set, defaults to pattern search)
	Pattern     string   `json:"pattern,omitempty"`      // Glob pattern: "internal/tui/*view*.go"
	PatternType string   `json:"pattern_type,omitempty"` // "glob" (default), "regex", "prefix", "suffix"
	Exclude     []string `json:"exclude,omitempty"`      // ["**/node_modules/**", "**/vendor/**"]
	Extensions  []string `json:"extensions,omitempty"`   // [".go", ".js", ".ts"] - shorthand filter

	// Component discovery options (when components=true)
	Types         []string `json:"types,omitempty"`          // Component types: ["entry-point", "api-handler", "view-component", etc.]
	Language      string   `json:"language,omitempty"`       // Specific language filter: "go", "javascript", "typescript", etc.
	MinConfidence float64  `json:"min_confidence,omitempty"` // Minimum classification confidence (0.0-1.0, default: 0.5)
	IncludeTests  bool     `json:"include_tests,omitempty"`  // Include test components (default: false)

	// Output options
	Max          int    `json:"max_results,omitempty"`   // Default: 100
	ShowContent  bool   `json:"show_content,omitempty"`  // Include first few lines as preview
	SortBy       string `json:"sort_by,omitempty"`       // "name" (default), "size", "modified", "type"
	IncludeStats bool   `json:"include_stats,omitempty"` // Include file size, modification time
}

// UnifiedSymbolParams consolidates definition, references, tree, and relations functionality
type UnifiedSymbolParams struct {
	// Operation mode options
	Definition bool `json:"definition,omitempty"` // Find symbol definitions
	References bool `json:"references,omitempty"` // Find symbol references/usages
	Tree       bool `json:"tree,omitempty"`       // Show call hierarchy tree
	Relations  bool `json:"relations,omitempty"`  // Show symbol relationships

	// Target specification (required)
	Symbol   string `json:"symbol,omitempty"`   // Symbol name for definition/references/relations
	Function string `json:"function,omitempty"` // Function name for tree

	// Tree-specific options
	MaxDepth  int    `json:"max_depth,omitempty"`  // Maximum depth for tree/relations
	ShowLines bool   `json:"show_lines,omitempty"` // Show line numbers in tree
	Compact   bool   `json:"compact,omitempty"`    // Compact tree display
	Exclude   string `json:"exclude,omitempty"`    // Exclude patterns from tree
	Agent     bool   `json:"agent,omitempty"`      // Agent mode for tree

	// Relations-specific options
	RelationTypes []string `json:"relation_types,omitempty"` // Types of relations to show
	ShowContext   bool     `json:"show_context,omitempty"`   // Show context for relations
	IncludeUsage  bool     `json:"include_usage,omitempty"`  // Include usage information
}

// UnifiedAnalysisParams consolidates analyze_intent, verify_pattern, and code_context functionality
type UnifiedAnalysisParams struct {
	// Operation mode options
	Intent  bool `json:"intent,omitempty"`  // Analyze semantic code intent with LLM-assisted pattern recognition
	Pattern bool `json:"pattern,omitempty"` // Verify architectural patterns and compliance rules
	Context bool `json:"context,omitempty"` // Analyze code context with scopes, imports, and dependencies

	// Intent analysis options
	IntentType      string   `json:"intent_type,omitempty"`      // Specific intent to search for (e.g., "size_management", "event_handling")
	DomainContext   string   `json:"domain_context,omitempty"`   // Domain context ("ui", "api", "business-logic", "configuration")
	AntiPatterns    []string `json:"anti_patterns,omitempty"`    // Specific anti-patterns to detect or ["*"] for all
	MinConfidence   float64  `json:"min_confidence,omitempty"`   // Minimum confidence threshold (0.0-1.0, default: 0.5)
	IncludeEvidence bool     `json:"include_evidence,omitempty"` // Include code evidence in results (default: true)

	// Pattern verification options
	PatternName string   `json:"pattern_name,omitempty"` // Built-in pattern name (e.g., "mvc_separation", "repository_pattern")
	Severity    []string `json:"severity,omitempty"`     // Filter by severity levels: ["error", "warning", "info"]
	ReportMode  string   `json:"report_mode,omitempty"`  // 'violations', 'summary', or 'detailed' (default: 'detailed')

	// Code context analysis options
	FilePath           string `json:"file_path,omitempty"`            // File to analyze
	Line               int    `json:"line,omitempty"`                 // Specific line number
	Symbol             string `json:"symbol,omitempty"`               // Specific symbol to contextualize
	ShowScopes         bool   `json:"show_scopes,omitempty"`          // Show containing scopes
	ShowImports        bool   `json:"show_imports,omitempty"`         // Show import/export context
	ShowDependencies   bool   `json:"show_dependencies,omitempty"`    // Show dependency impact
	ShowQualityMetrics bool   `json:"show_quality_metrics,omitempty"` // Show code quality metrics
	ShowCompilerFlags  bool   `json:"show_compiler_flags,omitempty"`  // Show compiler context

	// Common options
	Scope string `json:"scope,omitempty"`       // File pattern scope for analysis (e.g., "internal/handlers/**")
	Max   int    `json:"max_results,omitempty"` // Maximum results to return (default varies by operation)
}

// UnifiedTreeSitterParams removed - tree-sitter query functionality has been removed

type FindComponentsParams struct {
	Types         []string `json:"types,omitempty"`          // Component types to find: ["entry-point", "api-handler", "view-component", "controller", "data-model", "configuration", "test", "utility", "service", "repository"]
	Language      string   `json:"language,omitempty"`       // Specific language filter: "go", "javascript", "typescript", etc.
	MinConfidence float64  `json:"min_confidence,omitempty"` // Minimum classification confidence (0.0-1.0, default: 0.5)
	IncludeTests  bool     `json:"include_tests,omitempty"`  // Include test components (default: false)
	Max           int      `json:"max_results,omitempty"`    // Maximum results to return (default: 100)
}

// ValidatePatternParams for pattern verification and validation
type ValidatePatternParams struct {
	Pattern     string `json:"pattern"`                // Pattern to validate
	PatternType string `json:"pattern_type,omitempty"` // "regex", "glob", "literal" (default: "literal")
	TestFile    string `json:"test_file,omitempty"`    // Optional file path to test pattern against
}

// VerifyPatternParams for architectural pattern verification
type VerifyPatternParams struct {
	Pattern    string   `json:"pattern"`               // Built-in pattern name (e.g., "mvc_separation", "repository_pattern")
	Scope      string   `json:"scope,omitempty"`       // File pattern scope for verification (e.g., "internal/handlers/**")
	Severity   []string `json:"severity,omitempty"`    // Filter by severity levels: ["error", "warning", "info"]
	Max        int      `json:"max_results,omitempty"` // Maximum violations to return (default: 100)
	ReportMode string   `json:"report_mode,omitempty"` // 'violations', 'summary', or 'detailed' (default: 'detailed')
}

// AnalyzeIntentParams for semantic code intent analysis
type AnalyzeIntentParams struct {
	Intent          string   `json:"intent,omitempty"`           // Specific intent to search for (e.g., "size_management", "event_handling")
	Scope           string   `json:"scope,omitempty"`            // File scope to analyze (e.g., "**/ui/**", "**/handlers/**")
	Context         string   `json:"context,omitempty"`          // Domain context ("ui", "api", "business-logic", "configuration")
	AntiPatterns    []string `json:"anti_patterns,omitempty"`    // Specific anti-patterns to detect or ["*"] for all
	MinConfidence   float64  `json:"min_confidence,omitempty"`   // Minimum confidence threshold (0.0-1.0, default: 0.5)
	IncludeEvidence bool     `json:"include_evidence,omitempty"` // Include code evidence in results (default: true)
	Max             int      `json:"max_results,omitempty"`      // Maximum results to return (default: 50)
}

type InfoParams struct {
	Tool     string         `json:"tool,omitempty"`
	Warnings []UnknownField `json:"-"` // Captures unknown fields
}

// UnmarshalJSON implements custom unmarshaling that accepts unknown fields
func (i *InfoParams) UnmarshalJSON(data []byte) error {
	type Alias InfoParams // Type alias to avoid recursion

	knownFields := map[string]struct{}{
		"tool": {},
	}

	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}

	// Now unmarshal into the actual struct
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*i = InfoParams(alias)
	i.Warnings = warnings
	return nil
}

type CodeContextParams struct {
	FilePath           string `json:"file_path,omitempty"`            // File to analyze
	Line               int    `json:"line,omitempty"`                 // Specific line number
	Symbol             string `json:"symbol,omitempty"`               // Specific symbol to contextualize
	ShowScopes         bool   `json:"show_scopes,omitempty"`          // Show containing scopes
	ShowImports        bool   `json:"show_imports,omitempty"`         // Show import/export context
	ShowDependencies   bool   `json:"show_dependencies,omitempty"`    // Show dependency impact
	ShowQualityMetrics bool   `json:"show_quality_metrics,omitempty"` // Show code quality metrics
	ShowCompilerFlags  bool   `json:"show_compiler_flags,omitempty"`  // Show compiler context
}

type PredictiveNavigationParams struct {
	AnalyzePatterns  bool `json:"analyze_patterns,omitempty"`
	SuggestNextSteps bool `json:"suggest_next_steps,omitempty"`
	MaxSuggestions   int  `json:"max_suggestions,omitempty"`
	IncludeReasons   bool `json:"include_reasons,omitempty"`
}

type IndexSwitchParams struct {
	Implementation string `json:"implementation"`
}

type IndexCompareParams struct {
	Pattern  string `json:"pattern"`
	MaxLines int    `json:"max_lines,omitempty"`
}

// NewServer creates a new MCP server using MasterIndex directly
func NewServer(goroutineIndex *indexing.MasterIndex, cfg *config.Config) (*Server, error) {
	// CRITICAL: Use file-based logging for MCP to keep stdio clean
	diagnosticLogger := NewDiagnosticLogger(true)

	// ALWAYS ensure we have an index - create one if not provided
	ownsIndex := false
	if goroutineIndex == nil {
		diagnosticLogger.Printf("Creating new MasterIndex")
		goroutineIndex = indexing.NewMasterIndex(cfg)
		ownsIndex = true // Server created it, so server owns it
	}
	diagnosticLogger.Printf("MCP server initialized with MasterIndex")

	// Create index coordinator for concurrent search operations
	indexCoordinator := core.NewIndexCoordinator()

	s := &Server{
		goroutineIndex:    goroutineIndex, // Always non-nil
		ownsIndex:         ownsIndex,
		cfg:               cfg,
		diagnosticLogger:  diagnosticLogger,
		searchCoordinator: search.NewSearchCoordinator(indexCoordinator),
		codeContext: &CodeContext{
			ContainingScopes: make([]ScopeBreadcrumb, 0),
			CompilerFlags: CompilerContext{
				CompilerFlags: make([]string, 0),
				BuildTags:     make([]string, 0),
				Pragmas:       make(map[string]string),
			},
			ImportContext: ImportContext{
				Imports:       make([]ImportInfo, 0),
				Exports:       make([]ExportInfo, 0),
				UnusedImports: make([]string, 0),
			},
			DependencyImpact: DependencyImpact{
				ImportedByFiles: make([]string, 0),
				UsageHotspots:   make([]UsageHotspot, 0),
			},
			QualityMetrics: CodeQualityMetrics{
				SecurityAnnotations: make([]string, 0),
			},
			LastUpdated: time.Now(),
		},
		predictiveContext: &PredictiveContext{
			CommonPatterns:    make([]NavigationPattern, 0),
			SessionFrequency:  make(map[string]int),
			SymbolAffinities:  make(map[string][]string),
			TimeBasedPatterns: make(map[string]float64),
		},
		indexingState: &AsyncIndexingState{
			Status:        "idle",
			CanCancel:     false,
			UpdateChannel: make(chan AsyncIndexingUpdate, 100),
			CancelChannel: make(chan bool, 1),
		},
	}

	// Initialize entity ID generator for consistent IDs across all tools
	// Determine project root with proper fallback logic
	projectRoot := s.determineProjectRoot(cfg)
	// Entity ID generation is now handled by methods on core types
	diagnosticLogger.Printf("Project root configured: %s", projectRoot)

	// Initialize auto-indexing manager with race-condition protection
	s.autoIndexManager = NewAutoIndexingManager(s)

	// Initialize semantic scorer (moved from lazy initialization in handlers.go)
	// This removes sync.Once from the search path for better performance
	splitter := semantic.NewNameSplitter()
	stemmer := semantic.NewStemmer(true, "porter2", cfg.SemanticScoring.StemMinLength, nil)
	fuzzer := semantic.NewFuzzyMatcher(true, cfg.SemanticScoring.FuzzyThreshold, "jaro-winkler")
	dict := semantic.DefaultTranslationDictionary()
	s.semanticScorer = semantic.NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	// Configure semantic scorer with values from config
	scoreLayers := semantic.ScoreLayers{
		ExactWeight:        cfg.SemanticScoring.ExactWeight,
		SubstringWeight:    cfg.SemanticScoring.SubstringWeight,
		AnnotationWeight:   cfg.SemanticScoring.AnnotationWeight,
		FuzzyWeight:        cfg.SemanticScoring.FuzzyWeight,
		StemmingWeight:     cfg.SemanticScoring.StemmingWeight,
		NameSplitWeight:    cfg.SemanticScoring.NameSplitWeight,
		AbbreviationWeight: cfg.SemanticScoring.AbbreviationWeight,
		FuzzyThreshold:     cfg.SemanticScoring.FuzzyThreshold,
		StemMinLength:      cfg.SemanticScoring.StemMinLength,
		MaxResults:         cfg.SemanticScoring.MaxResults,
		MinScore:           cfg.SemanticScoring.MinScore,
	}
	s.semanticScorer.Configure(scoreLayers)

	// Create search engine with semantic scoring for advanced matching (camelCase, fuzzy, etc.)
	searchEngine := search.NewEngineWithSemanticScorer(goroutineIndex, s.semanticScorer)
	goroutineIndex.SetSearchEngine(searchEngine)
	diagnosticLogger.Printf("Search engine initialized with semantic scoring")

	// Create MCP server with correct API
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "lci-mcp-server",
		Version: "0.1.0",
	}, nil)

	s.server = server
	s.registerMasterIndexTools() // Use different tool registration

	// Start auto-indexing synchronously (it will manage its own goroutines internally)
	// This ensures the index is created and indexing is started before the server is ready
	s.startAutoIndexing()

	return s, nil
}

// startAutoIndexing uses the AutoIndexingManager to ensure proper state management,
// progress tracking, and cancellation support during the indexing process.
// CRITICAL: This runs in BOTH production and test environments to ensure consistency.
// The divergence between MCP server and workflow tests was a bug.
func (s *Server) startAutoIndexing() {
	// If config is nil, we can't perform auto-indexing (e.g., in some tests)
	if s.cfg == nil {
		s.diagnosticLogger.Printf("Config is nil, skipping auto-indexing")
		return
	}

	// Check if we're in a project root
	rootPath := s.cfg.Project.Root
	if rootPath == "" || rootPath == "." {
		// Try to detect the current working directory
		if cwd, err := os.Getwd(); err == nil {
			rootPath = cwd
		} else {
			s.diagnosticLogger.Printf("Failed to get current working directory: %v", err)
			rootPath = "."
		}
	}

	// Validate root path
	if rootPath == "" {
		s.diagnosticLogger.Printf("Empty root path, skipping auto-indexing")
		return
	}

	// Use shared ProjectInitializer to check if we're in a project
	initializer := indexing.NewProjectInitializer()
	isProjectRoot, marker := initializer.DetectProjectRoot(rootPath)
	if !isProjectRoot {
		s.diagnosticLogger.Printf("Not in a detectable project root (path: %s), skipping auto-indexing", rootPath)
		return
	}

	s.diagnosticLogger.Printf("Detected project root at %s (found %s), starting auto-indexing via manager", rootPath, marker)

	// Use the AutoIndexingManager for proper state management and error handling
	if err := s.autoIndexManager.startAutoIndexing(rootPath, s.cfg); err != nil {
		s.diagnosticLogger.Printf("Failed to start auto-indexing manager: %v", err)
		// Update server state to reflect the error
		s.indexingMutex.Lock()
		s.indexingState.Status = "failed"
		s.indexingState.ErrorMessage = err.Error()
		s.indexingState.CanCancel = false
		s.indexingMutex.Unlock()

		// Send notification to MCP client about index failure
		s.sendIndexFailureNotification(err, rootPath)
	}
}

// checkIndexAvailability checks if the index is ready for operations.
// If indexing is in progress, it waits up to the configured timeout for completion.
// Returns true if index is available, false otherwise with error details.
func (s *Server) checkIndexAvailability() (bool, error) {
	// Check if index is initialized
	if s.goroutineIndex == nil {
		return false, errors.New("index not initialized - server may be starting up. Try: 1) Check that you're in a project directory with source code, 2) Wait for auto-indexing to complete (check index_stats)")
	}

	// Check initial indexing state
	s.indexingMutex.RLock()
	status := s.indexingState.Status
	errorMsg := s.indexingState.ErrorMessage
	s.indexingMutex.RUnlock()

	// Handle failed state immediately
	if status == "failed" {
		return false, fmt.Errorf("indexing failed: %s. Troubleshooting: 1) Check file permissions in project directory, 2) Verify .lci.kdl configuration is valid, 3) Ensure project contains supported file types (.go, .js, .ts, .py, .rs, .java), 4) Check diagnostic logs for details", errorMsg)
	}

	// If no indexing has been done yet, return immediately (auto-indexing will start)
	if status == "idle" {
		s.diagnosticLogger.Printf("Index is idle, background indexing will start automatically")
		return true, nil
	}

	// If indexing is complete, return immediately
	if status == "completed" {
		return true, nil
	}

	// Indexing is in progress - wait for completion using autoIndexManager signals
	if status == "indexing" || status == "estimating" {
		if s.autoIndexManager == nil {
			return false, errors.New("indexing in progress but auto-index manager unavailable")
		}

		// Determine timeout from config or use default
		timeoutSeconds := DefaultIndexingTimeout
		if s.cfg != nil && s.cfg.Performance.IndexingTimeoutSec > 0 {
			timeoutSeconds = s.cfg.Performance.IndexingTimeoutSec
		}
		timeout := time.Duration(timeoutSeconds) * time.Second

		s.diagnosticLogger.Printf("Indexing in progress, waiting up to %v for completion", timeout)

		// Wait for completion using channel-based signaling (no polling)
		finalStatus, err := s.autoIndexManager.waitForCompletion(timeout)
		if err != nil {
			return false, fmt.Errorf("indexing timeout after %v - codebase indexing is taking longer than expected. For large projects, check index_stats for progress", timeout)
		}

		// Check final status after waiting
		if finalStatus == "failed" {
			s.indexingMutex.RLock()
			errorMsg = s.indexingState.ErrorMessage
			s.indexingMutex.RUnlock()
			return false, fmt.Errorf("indexing failed: %s", errorMsg)
		}

		if finalStatus == "completed" {
			return true, nil
		}

		// Unexpected final status
		return false, fmt.Errorf("unexpected indexing status after wait: %s", finalStatus)
	}

	// Unknown status - treat as available
	return true, nil
}

// sendIndexFailureNotification sends an error notification to MCP clients about indexing failure
func (s *Server) sendIndexFailureNotification(err error, rootPath string) {
	// Note: We don't have access to ServerSession here, but we can log the error
	// The handlers will detect the failed state and return helpful errors to clients
	s.diagnosticLogger.Printf("INDEX FAILURE NOTIFICATION: %v", err)
	s.diagnosticLogger.Printf("To resolve indexing issues: 1) Verify you're in a project directory with source code, 2) Check file permissions, 3) Review .lci.kdl configuration")
}

// ========== Helper functions for consolidated parameter parsing ==========

// parseBoolFlag checks if a flag is present in a comma-separated flag string
func parseBoolFlag(flags, target string) bool {
	if flags == "" {
		return false
	}
	flagList := parseCommaSeparated(flags)
	for _, flag := range flagList {
		if flag == target {
			return true
		}
	}
	return false
}

// parseCommaSeparated splits a comma-separated string into a slice
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	// Split by comma and trim spaces
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseContextLines extracts context line count from output format
// Accepts: "line" (0), "ctx" (3), "ctx:5" (5), "full" (default), etc.
func parseContextLines(output string) int {
	if output == "" || output == "full" {
		return SearchDefaultContextLines
	}
	if output == "line" {
		return 0
	}
	// Handle "ctx:N" format
	if strings.HasPrefix(output, "ctx") {
		parts := strings.Split(output, ":")
		if len(parts) == 2 {
			if count, err := strconv.Atoi(parts[1]); err == nil {
				return count
			}
		}
		return SearchDefaultContextLines // Default if can't parse
	}
	return 0 // For "files", "count" etc.
}

// Search adapts SearchParams to MasterIndex.SearchWithOptions
func (s *Server) Search(ctx context.Context, req SearchParams) (*SearchResponse, error) {
	// Check if index is available before attempting search
	if available, err := s.checkIndexAvailability(); err != nil {
		return nil, err
	} else if !available {
		return nil, errors.New("search cannot proceed: index is not available")
	}

	// Convert SearchParams to SearchOptions with consolidated format parsing
	options := types.SearchOptions{
		// Parse flags from comma-separated string
		CaseInsensitive:  parseBoolFlag(req.Flags, "ci"),
		UseRegex:         parseBoolFlag(req.Flags, "rx"),
		DeclarationOnly:  false, // Not in new format
		ExcludeTests:     parseBoolFlag(req.Flags, "nt"),
		ExcludeComments:  parseBoolFlag(req.Flags, "nc"),
		SymbolTypes:      parseCommaSeparated(req.SymbolTypes),
		IncludeObjectIDs: true, // Default true in new format

		// Parse output format
		MaxContextLines: parseContextLines(req.Output),
		CountPerFile:    req.Output == "count",
		FilesOnly:       req.Output == "files",

		// Grep-like features from flags
		InvertMatch:     parseBoolFlag(req.Flags, "iv"),
		WordBoundary:    parseBoolFlag(req.Flags, "wb"),
		MaxCountPerFile: req.MaxPerFile,
	}

	// Parse Patterns field (comma-separated)
	if req.Patterns != "" {
		options.Patterns = parseCommaSeparated(req.Patterns)
	}

	// Perform detailed search to get full semantic data
	detailedResults, err := s.goroutineIndex.SearchDetailedWithOptions(req.Pattern, options)
	if err != nil {
		return nil, err
	}

	// Convert detailed results to SearchResponse format with full semantic data
	searchResults := make([]CompactSearchResult, len(detailedResults))
	for i, detailed := range detailedResults {
		result := detailed.Result
		searchResult := CompactSearchResult{
			// Basic location information
			File:   result.Path,
			Line:   result.Line,
			Column: result.Column,
			Score:  result.Score,
			Match:  result.Match,

			// Grep feature fields
			FileMatchCount: result.FileMatchCount,

			// Context lines (only if we have context)
			ContextLines: result.Context.Lines,
		}

		// Add semantic symbol information if available
		if detailed.RelationalData != nil {
			symbol := &detailed.RelationalData.Symbol
			searchResult.SymbolType = symbol.Type.String()
			searchResult.SymbolName = symbol.Name
			// Note: Signature and Kind fields don't exist in current Symbol struct
			// searchResult.SymbolSignature = symbol.Signature
			// searchResult.SymbolKind = string(symbol.Kind)

			// Add reference statistics
			searchResult.References = &ReferenceInfo{
				IncomingCount: detailed.RelationalData.RefStats.Total.IncomingCount,
				OutgoingCount: detailed.RelationalData.RefStats.Total.OutgoingCount,
			}

			// Add scope information as breadcrumbs if available
			if len(detailed.RelationalData.Breadcrumbs) > 0 {
				// Convert breadcrumbs to our format
				breadcrumbs := make([]ScopeBreadcrumb, len(detailed.RelationalData.Breadcrumbs))
				for i, crumb := range detailed.RelationalData.Breadcrumbs {
					breadcrumbs[i] = ScopeBreadcrumb{
						Name:      crumb.Name,
						ScopeType: crumb.Type.String(),
					}
				}
				searchResult.Breadcrumbs = breadcrumbs
			}
		}

		// Language information not needed for CompactSearchResult in prototype

		searchResults[i] = searchResult
	}

	return &SearchResponse{
		Results:      searchResults,
		TotalMatches: len(detailedResults),
	}, nil
}

// registerMasterIndexTools registers tools that work with MasterIndex directly
func (s *Server) registerMasterIndexTools() {
	// For now, delegate to the existing registration - we'll update handlers later
	s.registerTools()
}

// RegisterCodeInsightTools registers the code insight tools with the MCP server
// @lci:labels[mcp-registration,tool-setup]
// @lci:category[mcp-api]
func (s *Server) RegisterCodeInsightTools() {
	// This method is called by tests to ensure tools are properly registered
	// The actual registration happens in registerTools() above
}

// registerTools registers all MCP tools with the server
// @lci:labels[mcp-registration,tool-setup,entry-point]
// @lci:category[mcp-api]
func (s *Server) registerTools() {
	// Meta tools - always register these first
	s.server.AddTool(&mcp.Tool{
		Name:        "info",
		Description: "🔍 Get detailed help and examples for any tool - start here! Use 'info' for overview or 'info <tool>' for specifics. Use 'info version' for server version info.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"tool": {
					Type:        "string",
					Description: "Tool name to get information about (e.g., 'search', 'get_context', 'version')",
				},
			},
		},
	}, s.handleInfo)

	// Powerful search and analysis tools
	s.server.AddTool(&mcp.Tool{
		Name:        "search",
		Description: "Sub-millisecond in-memory semantic code search. Use instead of grep, rg, find. Note: Uses JSON parameters, not CLI flags like -n. See 'info search' for parameter details.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"pattern": {
					Type:        "string",
					Description: "Search pattern",
				},
				"max": {
					Type:        "integer",
					Description: "Maximum results",
				},
				"output": {
					Type:        "string",
					Description: "Output format",
				},
				"filter": {
					Type:        "string",
					Description: "File filter",
				},
				"flags": {
					Type:        "string",
					Description: "Search flags",
				},
				"include": {
					Type:        "string",
					Description: "Include options",
				},
				"symbol_types": {
					Type:        "string",
					Description: "Symbol types to filter results (comma-separated). Valid types: function, class, method, variable, constant, interface, type, struct, module, namespace, property, event, delegate, enum, record, operator, indexer, object, companion, extension, annotation, field, enum_member. Aliases: func->function, var->variable, const->constant, cls->class, meth->method, iface->interface, def->function (Python), fn->function (Rust), trait->interface (Rust). Prefix and fuzzy matching supported with warnings.",
				},
				"patterns": {
					Type:        "string",
					Description: "Multiple patterns",
				},
				"max_per_file": {
					Type:        "integer",
					Description: "Max per file",
				},
				"semantic": {
					Type:        "boolean",
					Description: "Enable semantic",
				},
				"languages": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Filter by programming languages (e.g., [\"go\"], [\"typescript\", \"javascript\"], [\"csharp\"]). Case-insensitive with aliases (e.g., 'ts' for TypeScript, 'cs' for C#).",
				},
			},
			Required: []string{"pattern"},
		},
	}, s.handleNewSearch)

	s.server.AddTool(&mcp.Tool{
		Name:        "get_context",
		Description: "📋 Get detailed context for specific code objects. Use the 'id' parameter with object IDs from search results. See 'info get_context' for examples.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"id": {
					Type:        "string",
					Description: "Concise object ID(s) from search results (e.g., \"VE\" or \"VE,tG\" for multiple)",
				},
				"name": {
					Type:        "string",
					Description: "Symbol name for direct lookup (alternative to id)",
				},
				"file_id": {
					Type:        "integer",
					Description: "File ID to narrow name lookup scope",
				},
				"line": {
					Type:        "integer",
					Description: "Line number",
				},
				"column": {
					Type:        "integer",
					Description: "Column number",
				},
				"mode": {
					Type:        "string",
					Description: "Lookup mode",
				},
				"include_full_symbol": {
					Type:        "boolean",
					Description: "Include full symbol info",
				},
				"include_call_hierarchy": {
					Type:        "boolean",
					Description: "Include call hierarchy",
				},
				"include_all_references": {
					Type:        "boolean",
					Description: "Include references",
				},
				"include_dependencies": {
					Type:        "boolean",
					Description: "Include dependencies",
				},
				"include_file_context": {
					Type:        "boolean",
					Description: "Include file context",
				},
				"include_quality_metrics": {
					Type:        "boolean",
					Description: "Include quality metrics",
				},
				"max_depth": {
					Type:        "integer",
					Description: "Max depth",
				},
				"include_ai_text": {
					Type:        "boolean",
					Description: "Include AI text",
				},
				"confidence_threshold": {
					Type:        "number",
					Description: "Confidence threshold",
				},
				"exclude_test_files": {
					Type:        "boolean",
					Description: "Exclude test files",
				},
				"include_sections": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Include sections",
				},
				"exclude_sections": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Exclude sections",
				},
			},
		},
	}, s.handleGetObjectContext)

	s.server.AddTool(&mcp.Tool{
		Name:        "semantic_annotations",
		Description: "🏷️  Query symbols by semantic labels or categories. Supports both direct @lci: annotations and propagated labels through call graphs. See 'info semantic_annotations'.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"label": {
					Type:        "string",
					Description: "Semantic label to search for",
				},
				"category": {
					Type:        "string",
					Description: "Semantic category",
				},
				"min_strength": {
					Type:        "number",
					Description: "Minimum label strength",
				},
				"include_direct": {
					Type:        "boolean",
					Description: "Include direct annotations",
				},
				"include_propagated": {
					Type:        "boolean",
					Description: "Include propagated labels",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum results (keep small to avoid token overload)",
				},
			},
		},
	}, s.handleSemanticAnnotations)

	// Side effect analysis tool
	s.server.AddTool(&mcp.Tool{
		Name:        "side_effects",
		Description: "🔬 Query function purity and side effects. Detects writes to parameters, globals, closures, I/O operations, and exception handling. Supports transitive analysis through call graphs. See 'info side_effects'.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"mode": {
					Type:        "string",
					Description: "Query mode: symbol, file, pure, impure, category, summary",
				},
				"symbol_id": {
					Type:        "string",
					Description: "Symbol ID for symbol mode",
				},
				"symbol_name": {
					Type:        "string",
					Description: "Symbol name for symbol mode",
				},
				"file_path": {
					Type:        "string",
					Description: "File path for file mode",
				},
				"file_id": {
					Type:        "integer",
					Description: "File ID for file mode",
				},
				"category": {
					Type:        "string",
					Description: "Side effect category: param_write, global_write, io, network, throw, channel, external_call",
				},
				"include_reasons": {
					Type:        "boolean",
					Description: "Include reasons for impurity",
				},
				"include_transitive": {
					Type:        "boolean",
					Description: "Include transitive side effects from callees",
				},
				"include_confidence": {
					Type:        "boolean",
					Description: "Include confidence levels",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum results (keep small to avoid token overload)",
				},
			},
		},
	}, s.handleSideEffects)

	// Component-level code analysis
	s.server.AddTool(&mcp.Tool{
		Name:        "code_insight",
		Description: "🎯 Comprehensive codebase intelligence system for AI agents. Provides high-level overview (79.8% context reduction), detailed analysis (2-4x accuracy improvement), code statistics, and git analysis. Modes: overview, detailed, statistics, unified, structure, git_analyze, git_hotspots. See 'info code_insight'.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"mode": {
					Type:        "string",
					Description: "Analysis mode",
				},
				"tier": {
					Type:        "integer",
					Description: "Analysis tier",
				},
				"analysis": {
					Type:        "string",
					Description: "Type of analysis",
				},
				"metrics": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Metrics to include",
				},
				"target": {
					Type:        "string",
					Description: "Target to analyze",
				},
				"focus": {
					Type:        "string",
					Description: "Analysis focus",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum results (keep small to avoid token overload)",
				},
				"languages": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Filter by programming languages (e.g., [\"go\"], [\"typescript\", \"javascript\"], [\"csharp\"]). Case-insensitive with aliases (e.g., 'ts' for TypeScript, 'cs' for C#).",
				},
			},
		},
	}, s.handleCodebaseIntelligence)

	// File search tool with fuzzy matching
	s.server.AddTool(&mcp.Tool{
		Name:        "find_files",
		Description: "📁 Like 'find' or 'fd' - searches file paths, not content, on an in-memory index. Supports fuzzy matching, glob patterns, and filters. See 'info find_files'.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"pattern": {
					Type:        "string",
					Description: "File/path pattern to search for (supports fuzzy matching)",
				},
				"max": {
					Type:        "integer",
					Description: "Maximum results (default: 50, max: 200)",
				},
				"filter": {
					Type:        "string",
					Description: "Filter by file type or glob pattern (e.g., 'go', '*.ts', 'src/**/*.js')",
				},
				"flags": {
					Type:        "string",
					Description: "Search flags: 'ci' (case-insensitive), 'exact' (exact match only)",
				},
				"include_hidden": {
					Type:        "boolean",
					Description: "Include hidden files/directories (default: false)",
				},
				"directory": {
					Type:        "string",
					Description: "Directory to search within (relative to project root)",
				},
			},
			Required: []string{"pattern"},
		},
	}, s.handleFiles)

	// Git analysis functionality has been integrated into code_insight tool
	// Use code_insight with mode="git_analyze" or mode="git_hotspots"

	// Semantic tools removed - semantic search is now built into the 'search' tool by default
	// Configuration moved to .lci.kdl file under 'semantic_scoring' section

	// Context manifest tool for agent handoff and context compression
	s.server.AddTool(&mcp.Tool{
		Name:        "context",
		Description: "🎯 Capture and hydrate code context manifests for efficient agent handoff. Save compact symbol references (2-5KB manifest), load to get instant full context with source code + call graphs. Eliminates redundant exploration across agent sessions. Operations: 'save' to create manifest, 'load' to hydrate. See 'info context'.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"operation": {
					Type:        "string",
					Description: "Operation: 'save' to create manifest, 'load' to hydrate context",
				},
				// Save parameters
				"refs": {
					Type:        "array",
					Description: "Code references to save (for 'save' operation)",
					Items: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							"f": {Type: "string", Description: "File path (required)"},
							"s": {Type: "string", Description: "Symbol name (optional)"},
							"l": {
								Type:        "object",
								Description: "Line range {start, end} (optional)",
								Properties: map[string]*jsonschema.Schema{
									"start": {Type: "integer", Description: "Start line (1-indexed)"},
									"end":   {Type: "integer", Description: "End line (1-indexed)"},
								},
							},
							"x": {
								Type:        "array",
								Items:       &jsonschema.Schema{Type: "string"},
								Description: "Expansion directives: 'callers', 'callees:2', 'implementations', etc.",
							},
							"role": {Type: "string", Description: "Semantic role: 'modify', 'contract', 'pattern', 'boundary'"},
							"note": {Type: "string", Description: "Architect annotation (free-form text)"},
						},
						Required: []string{"f"},
					},
				},
				"to_file": {
					Type:        "string",
					Description: "Write manifest to file path (relative to project root)",
				},
				"to_string": {
					Type:        "boolean",
					Description: "Return manifest as JSON string instead of writing to file",
				},
				"append": {
					Type:        "boolean",
					Description: "Append to existing manifest (default: false)",
				},
				"task": {
					Type:        "string",
					Description: "Task description/directive (free-form text)",
				},
				// Load parameters
				"from_file": {
					Type:        "string",
					Description: "Load manifest from file path (for 'load' operation)",
				},
				"from_string": {
					Type:        "string",
					Description: "Load manifest from inline JSON string",
				},
				"filter": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Only include these roles (e.g., ['modify', 'contract'])",
				},
				"exclude": {
					Type:        "array",
					Items:       &jsonschema.Schema{Type: "string"},
					Description: "Exclude these roles",
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'full' (default), 'signatures', 'outline'",
				},
				"max_tokens": {
					Type:        "integer",
					Description: "Approximate token limit for hydrated context (0 = no limit)",
				},
			},
			Required: []string{"operation"},
		},
	}, s.handleContext)

}

// determineProjectRoot determines the project root path with proper fallback logic.
// Uses configuration if available, otherwise falls back to current working directory.
// FAIL-FAST: Returns empty string if no valid path can be determined.
func (s *Server) determineProjectRoot(cfg *config.Config) string {
	// First priority: Use config if available and valid
	if cfg != nil && cfg.Project.Root != "" {
		return cfg.Project.Root
	}

	// Second priority: Use current working directory
	// FAIL-FAST: os.Getwd can fail, handle the error
	cwd, err := os.Getwd()
	if err == nil && cwd != "" {
		return cwd
	}

	// Final fallback: Use "." as current directory
	// This ensures we always return a valid (if not ideal) path
	s.diagnosticLogger.Printf("Warning: cannot determine project root, using current directory")
	return "."
}

// recoverFromPanic provides panic recovery middleware for MCP operations
func (s *Server) recoverFromPanic(operation string, handler func() (*mcp.CallToolResult, error)) (*mcp.CallToolResult, error) {
	defer func() {
		if r := recover(); r != nil {
			panicStack := debug.Stack()

			s.diagnosticLogger.Printf("PANIC RECOVERED in %s: %v", operation, r)
			s.diagnosticLogger.Printf("Stack trace: %s", panicStack)

			// Log additional context about server state
			s.diagnosticLogger.Printf("Server state during panic - goroutineIndex: %v, searchCoordinator: %v",
				s.goroutineIndex != nil, s.searchCoordinator != nil)

			// Log memory statistics for debugging
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			s.diagnosticLogger.Printf("Memory stats - Alloc: %d KB, TotalAlloc: %d KB, Sys: %d KB, NumGC: %d",
				m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC)

			// Log recovery attempt using standard logger
			s.diagnosticLogger.Printf("Recovery completed for operation: %s", operation)
		}
	}()

	// Execute the handler with panic protection
	result, err := handler()
	if err != nil {
		// Log the error with context
		s.diagnosticLogger.Printf("Error in %s: %v", operation, err)

		// Return a user-friendly error response
		return createSmartErrorResponse(operation, err, map[string]interface{}{
			"operation": operation,
			"timestamp": time.Now().Format(time.RFC3339),
			"server_info": map[string]interface{}{
				"index_available":              s.goroutineIndex != nil,
				"search_coordinator_available": s.searchCoordinator != nil,
			},
		})
	}

	return result, nil
}

func (s *Server) Start(ctx context.Context) error {
	// Use dedicated logger instead of modifying global state
	s.diagnosticLogger.Printf("Starting MCP server with stdio transport")

	// Start pprof HTTP server for profiling (optional, controlled by env var)
	if pprofPort := os.Getenv("LCI_PPROF_PORT"); pprofPort != "" {
		go func() {
			s.diagnosticLogger.Printf("Starting pprof server on http://localhost:%s/debug/pprof/", pprofPort)
			if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
				s.diagnosticLogger.Printf("pprof server error: %v", err)
			}
		}()
	}

	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// Shutdown gracefully shuts down the server and its components
func (s *Server) Shutdown(ctx context.Context) error {
	s.diagnosticLogger.Printf("Shutting down MCP server...")

	// Shutdown auto-indexing manager
	if s.autoIndexManager != nil {
		s.autoIndexManager.Close()
		s.diagnosticLogger.Printf("Auto-indexing manager shutdown complete")
	}

	// Shutdown context lookup engine if it exists
	if s.contextLookupEngine != nil {
		// ContextLookupEngine doesn't have an explicit shutdown method, but it uses TrigramIndex
		// which will be shut down by the MasterIndex
		s.diagnosticLogger.Printf("Context lookup engine shutdown complete")
	}

	// Shutdown goroutine index (this should handle TrigramIndex shutdown)
	if s.goroutineIndex != nil {
		if err := s.goroutineIndex.Close(); err != nil {
			s.diagnosticLogger.Printf("Error shutting down goroutine index: %v", err)
		} else {
			s.diagnosticLogger.Printf("Goroutine index shutdown complete")
		}
	}

	s.diagnosticLogger.Printf("MCP server shutdown complete")

	// Close diagnostic logger to flush file
	if s.diagnosticLogger != nil {
		s.diagnosticLogger.Close()
	}

	return nil
}

// GetHandlerForTesting returns a handler function for testing purposes
func (s *Server) GetHandlerForTesting(toolName string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch toolName {
	case "search":
		return s.handleNewSearch
	case "get_object_context":
		return s.handleGetObjectContext
	default:
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, _ := createErrorResponse("GetHandlerForTesting", fmt.Errorf("unknown tool: %s", toolName))
			return result, nil
		}
	}
}

// Close releases resources held by the Server
func (s *Server) Close() error {
	// Only close the index if we created it
	if s.ownsIndex && s.goroutineIndex != nil {
		return s.goroutineIndex.Close()
	}
	return nil
}
