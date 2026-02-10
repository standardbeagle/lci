package server

import (
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// RPC request/response types for client-server communication

// IndexStatus represents the current status of the index
type IndexStatus struct {
	Ready          bool    `json:"ready"`
	FileCount      int     `json:"file_count"`
	SymbolCount    int     `json:"symbol_count"`
	IndexingActive bool    `json:"indexing_active"`
	Progress       float64 `json:"progress"`
	Error          string  `json:"error,omitempty"`
}

// SearchRequest represents a search request from a client
type SearchRequest struct {
	Pattern    string              `json:"pattern"`
	Options    types.SearchOptions `json:"options"`
	MaxResults int                 `json:"max_results,omitempty"`
}

// SearchResponse contains search results
type SearchResponse struct {
	Results []searchtypes.Result `json:"results"`
	Error   string               `json:"error,omitempty"`
}

// GetSymbolRequest requests symbol information
type GetSymbolRequest struct {
	SymbolID types.SymbolID `json:"symbol_id"`
}

// GetSymbolResponse contains symbol information
type GetSymbolResponse struct {
	Symbol *types.EnhancedSymbol `json:"symbol,omitempty"`
	Error  string                `json:"error,omitempty"`
}

// GetFileInfoRequest requests file information
type GetFileInfoRequest struct {
	FileID types.FileID `json:"file_id"`
}

// GetFileInfoResponse contains file information
type GetFileInfoResponse struct {
	FileInfo *types.FileInfo `json:"file_info,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// ShutdownRequest requests server shutdown
type ShutdownRequest struct {
	Force bool `json:"force,omitempty"`
}

// ShutdownResponse confirms shutdown
type ShutdownResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// PingRequest for health check
type PingRequest struct{}

// PingResponse confirms server is alive
type PingResponse struct {
	Uptime  float64 `json:"uptime_seconds"`
	Version string  `json:"version"`
	BuildID string  `json:"build_id,omitempty"`
}

// ReindexRequest triggers a re-index
type ReindexRequest struct {
	Path string `json:"path,omitempty"` // Empty means use configured root
}

// ReindexResponse confirms re-indexing started
type ReindexResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// StatsRequest requests index statistics
type StatsRequest struct{}

// StatsResponse contains index statistics
type StatsResponse struct {
	FileCount       int     `json:"file_count"`
	SymbolCount     int     `json:"symbol_count"`
	IndexSizeBytes  int64   `json:"index_size_bytes"`
	BuildDurationMs int64   `json:"build_duration_ms"`
	MemoryAllocMB   float64 `json:"memory_alloc_mb"`
	MemoryTotalMB   float64 `json:"memory_total_mb"`
	MemoryHeapMB    float64 `json:"memory_heap_mb"`
	NumGoroutines   int     `json:"num_goroutines"`
	UptimeSeconds   float64 `json:"uptime_seconds"`
	SearchCount     int64   `json:"search_count,omitempty"`
	AvgSearchTimeMs float64 `json:"avg_search_time_ms,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// DefinitionRequest requests symbol definition locations
type DefinitionRequest struct {
	Pattern    string `json:"pattern"`               // Symbol name pattern to search for
	MaxResults int    `json:"max_results,omitempty"` // Maximum number of results to return
}

// DefinitionLocation represents a single definition location
type DefinitionLocation struct {
	Name       string `json:"name"`        // Symbol name
	Type       string `json:"type"`        // Symbol type (function, class, struct, interface, type, method)
	FilePath   string `json:"file_path"`   // Full file path
	Line       int    `json:"line"`        // Line number (1-based)
	Column     int    `json:"column"`      // Column number (0-based)
	Signature  string `json:"signature,omitempty"`   // Function/method signature if available
	DocComment string `json:"doc_comment,omitempty"` // Documentation comment if available
}

// DefinitionResponse contains definition search results
type DefinitionResponse struct {
	Definitions []DefinitionLocation `json:"definitions"`
	Error       string               `json:"error,omitempty"`
}

// ReferencesRequest requests symbol reference locations (usages)
type ReferencesRequest struct {
	Pattern    string `json:"pattern"`               // Symbol name pattern to search for
	MaxResults int    `json:"max_results,omitempty"` // Maximum number of results to return
}

// ReferenceLocation represents a single reference location
type ReferenceLocation struct {
	FilePath string `json:"file_path"` // Full file path
	Line     int    `json:"line"`      // Line number (1-based)
	Column   int    `json:"column"`    // Column number (0-based)
	Context  string `json:"context"`   // Line content or surrounding context
	Match    string `json:"match"`     // The matched text
}

// ReferencesResponse contains reference search results
type ReferencesResponse struct {
	References []ReferenceLocation `json:"references"`
	Error      string              `json:"error,omitempty"`
}

// TreeRequest requests a function call tree
type TreeRequest struct {
	FunctionName string `json:"function_name"`          // Function name to generate tree for
	MaxDepth     int    `json:"max_depth,omitempty"`    // Maximum depth to traverse (0 = unlimited)
	ShowLines    bool   `json:"show_lines,omitempty"`   // Include line numbers
	Compact      bool   `json:"compact,omitempty"`      // Use compact output format
	Exclude      string `json:"exclude,omitempty"`      // Pattern to exclude from tree
	AgentMode    bool   `json:"agent_mode,omitempty"`   // Enable agent-friendly output with safety info
}

// TreeResponse contains the function call tree
type TreeResponse struct {
	Tree  *types.FunctionTree `json:"tree,omitempty"`
	Error string              `json:"error,omitempty"`
}

// GitAnalyzeRequest requests git change analysis
type GitAnalyzeRequest struct {
	Scope               string   `json:"scope"`                 // staged, wip, commit, range
	BaseRef             string   `json:"base_ref,omitempty"`
	TargetRef           string   `json:"target_ref,omitempty"`
	Focus               []string `json:"focus,omitempty"`       // duplicates, naming, metrics
	SimilarityThreshold float64  `json:"similarity_threshold,omitempty"`
	MaxFindings         int      `json:"max_findings,omitempty"`
}

// GitAnalyzeResponse contains the git analysis report
type GitAnalyzeResponse struct {
	Report interface{} `json:"report,omitempty"` // *git.AnalysisReport
	Error  string      `json:"error,omitempty"`
}

// ListSymbolsRequest requests enumeration of symbols with filtering
type ListSymbolsRequest struct {
	Kind          string `json:"kind"`                    // Required: symbol kinds (comma-separated)
	File          string `json:"file,omitempty"`           // Glob pattern for file path filter
	Exported      *bool  `json:"exported,omitempty"`       // Visibility filter
	Name          string `json:"name,omitempty"`           // Substring filter on symbol name
	Receiver      string `json:"receiver,omitempty"`       // Filter methods by receiver type
	MinComplexity *int   `json:"min_complexity,omitempty"` // Min cyclomatic complexity
	MaxComplexity *int   `json:"max_complexity,omitempty"` // Max cyclomatic complexity
	MinParams     *int   `json:"min_params,omitempty"`     // Min parameter count
	MaxParams     *int   `json:"max_params,omitempty"`     // Max parameter count
	Flags         string `json:"flags,omitempty"`          // Comma-separated: async, variadic, generator, method
	Sort          string `json:"sort,omitempty"`           // name, complexity, refs, line, params
	Max           int    `json:"max,omitempty"`            // Max results
	Offset        int    `json:"offset,omitempty"`         // Pagination offset
	Include       string `json:"include,omitempty"`        // Comma-separated extras
}

// ListSymbolsEntry is a symbol in the list response
type ListSymbolsEntry struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	File           string   `json:"file"`
	Line           int      `json:"line"`
	ObjectID       string   `json:"object_id,omitempty"`
	IsExported     bool     `json:"is_exported"`
	Signature      string   `json:"signature,omitempty"`
	Complexity     int      `json:"complexity,omitempty"`
	ParameterCount int      `json:"parameter_count,omitempty"`
	ReceiverType   string   `json:"receiver_type,omitempty"`
	IncomingRefs   int      `json:"incoming_refs,omitempty"`
	OutgoingRefs   int      `json:"outgoing_refs,omitempty"`
	Callers        []string `json:"callers,omitempty"`
	Callees        []string `json:"callees,omitempty"`
}

// ListSymbolsResponse contains enumerated symbols
type ListSymbolsHTTPResponse struct {
	Symbols []ListSymbolsEntry `json:"symbols"`
	Total   int                `json:"total"`
	Showing int                `json:"showing"`
	HasMore bool               `json:"has_more"`
	Error   string             `json:"error,omitempty"`
}

// InspectSymbolRequest requests deep inspection of a symbol
type InspectSymbolRequest struct {
	Name     string `json:"name,omitempty"`
	ID       string `json:"id,omitempty"`
	File     string `json:"file,omitempty"`
	Type     string `json:"type,omitempty"`
	Include  string `json:"include,omitempty"`
	MaxDepth int    `json:"max_depth,omitempty"`
}

// InspectSymbolEntry is a detailed symbol in the inspect response
type InspectSymbolEntry struct {
	Name           string              `json:"name"`
	ObjectID       string              `json:"object_id"`
	Type           string              `json:"type"`
	File           string              `json:"file"`
	Line           int                 `json:"line"`
	IsExported     bool                `json:"is_exported"`
	Signature      string              `json:"signature,omitempty"`
	DocComment     string              `json:"doc_comment,omitempty"`
	Complexity     int                 `json:"complexity,omitempty"`
	ParameterCount int                 `json:"parameter_count,omitempty"`
	ReceiverType   string              `json:"receiver_type,omitempty"`
	FunctionFlags  []string            `json:"function_flags,omitempty"`
	VariableFlags  []string            `json:"variable_flags,omitempty"`
	Callers        []string            `json:"callers,omitempty"`
	Callees        []string            `json:"callees,omitempty"`
	TypeHierarchy  *TypeHierarchyEntry `json:"type_hierarchy,omitempty"`
	ScopeChain     []string            `json:"scope_chain,omitempty"`
	IncomingRefs   int                 `json:"incoming_refs,omitempty"`
	OutgoingRefs   int                 `json:"outgoing_refs,omitempty"`
	Annotations    []string            `json:"annotations,omitempty"`
}

// TypeHierarchyEntry for inspect response
type TypeHierarchyEntry struct {
	Implements    []string `json:"implements,omitempty"`
	ImplementedBy []string `json:"implemented_by,omitempty"`
	Extends       []string `json:"extends,omitempty"`
	ExtendedBy    []string `json:"extended_by,omitempty"`
}

// InspectSymbolHTTPResponse contains detailed symbol inspection results
type InspectSymbolHTTPResponse struct {
	Symbols []InspectSymbolEntry `json:"symbols"`
	Count   int                  `json:"count"`
	Error   string               `json:"error,omitempty"`
}

// BrowseFileRequest requests symbol listing for a specific file
type BrowseFileRequest struct {
	File        string `json:"file,omitempty"`
	FileID      *int   `json:"file_id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Exported    *bool  `json:"exported,omitempty"`
	Sort        string `json:"sort,omitempty"`
	Max         int    `json:"max,omitempty"`
	Include     string `json:"include,omitempty"`
	ShowImports bool   `json:"show_imports,omitempty"`
	ShowStats   bool   `json:"show_stats,omitempty"`
}

// BrowseFileHTTPResponse contains file browsing results
type BrowseFileHTTPResponse struct {
	File    BrowseFileInfoEntry  `json:"file"`
	Symbols []ListSymbolsEntry   `json:"symbols"`
	Total   int                  `json:"total"`
	Imports []string             `json:"imports,omitempty"`
	Stats   *FileStatsEntry      `json:"stats,omitempty"`
	Error   string               `json:"error,omitempty"`
}

// BrowseFileInfoEntry describes the file being browsed
type BrowseFileInfoEntry struct {
	Path     string `json:"path"`
	FileID   int    `json:"file_id"`
	Language string `json:"language,omitempty"`
}

// FileStatsEntry for file-level statistics
type FileStatsEntry struct {
	SymbolCount   int     `json:"symbol_count"`
	FunctionCount int     `json:"function_count"`
	TypeCount     int     `json:"type_count"`
	AvgComplexity float64 `json:"avg_complexity,omitempty"`
	MaxComplexity int     `json:"max_complexity,omitempty"`
	ExportedCount int     `json:"exported_count"`
}
