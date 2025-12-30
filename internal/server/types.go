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
