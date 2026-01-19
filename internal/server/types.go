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
