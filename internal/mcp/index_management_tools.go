package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/git"
	"github.com/standardbeagle/lci/internal/types"
)

// ============================================================================
// Index Status Types
// ============================================================================

// IndexStatusParams defines parameters for index_stats queries
type IndexStatusParams struct {
	// Mode: "summary", "detailed", "progress", "health"
	Mode string `json:"mode,omitempty"`

	// Include specific sections
	IncludeMemory     bool `json:"include_memory,omitempty"`
	IncludeWatchMode  bool `json:"include_watch_mode,omitempty"`
	IncludeComponents bool `json:"include_components,omitempty"`
}

// IndexStatusResponse contains comprehensive index status
type IndexStatusResponse struct {
	// Core status
	Status      string    `json:"status"` // "ready", "indexing", "error", "initializing"
	Timestamp   time.Time `json:"timestamp"`
	ServerReady bool      `json:"server_ready"`

	// Basic stats
	FileCount      int   `json:"file_count"`
	SymbolCount    int   `json:"symbol_count"`
	ReferenceCount int   `json:"reference_count"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
	IndexTimeMs    int64 `json:"index_time_ms"`

	// Indexing progress (if indexing)
	Progress *IndexingProgressInfo `json:"progress,omitempty"`

	// Component health
	ComponentHealth *ComponentHealthInfo `json:"component_health,omitempty"`

	// Memory usage
	MemoryUsage *MemoryUsageInfo `json:"memory_usage,omitempty"`

	// Watch mode status
	WatchMode *WatchModeInfo `json:"watch_mode,omitempty"`

	// Issues and warnings
	Issues   []string `json:"issues,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// IndexingProgressInfo contains indexing progress details
type IndexingProgressInfo struct {
	IsIndexing        bool    `json:"is_indexing"`
	IsScanning        bool    `json:"is_scanning"`
	TotalFiles        int     `json:"total_files"`
	FilesProcessed    int     `json:"files_processed"`
	FilesSkipped      int     `json:"files_skipped"`
	ScanningProgress  float64 `json:"scanning_progress"`
	IndexingProgress  float64 `json:"indexing_progress"`
	OverallProgress   float64 `json:"overall_progress"`
	EstimatedTimeLeft string  `json:"estimated_time_left,omitempty"`
}

// ComponentHealthInfo contains health status of each index component
type ComponentHealthInfo struct {
	SymbolIndexReady     bool `json:"symbol_index_ready"`
	TrigramIndexReady    bool `json:"trigram_index_ready"`
	RefTrackerReady      bool `json:"ref_tracker_ready"`
	CallGraphPopulated   bool `json:"call_graph_populated"`
	SideEffectsReady     bool `json:"side_effects_ready"`
	SemanticIndexReady   bool `json:"semantic_index_ready"`
	FileContentStoreReady bool `json:"file_content_store_ready"`

	// Detailed stats per component
	SymbolStats    map[string]interface{} `json:"symbol_stats,omitempty"`
	ReferenceStats map[string]interface{} `json:"reference_stats,omitempty"`
}

// MemoryUsageInfo contains memory statistics
type MemoryUsageInfo struct {
	HeapAllocMB     float64 `json:"heap_alloc_mb"`
	HeapSysMB       float64 `json:"heap_sys_mb"`
	TotalAllocMB    float64 `json:"total_alloc_mb"`
	NumGC           uint32  `json:"num_gc"`
	EstimatedIndexMB float64 `json:"estimated_index_mb"`
	PressureLevel   string  `json:"pressure_level"` // "low", "medium", "high"
}

// WatchModeInfo contains file watcher status
type WatchModeInfo struct {
	Enabled         bool      `json:"enabled"`
	Active          bool      `json:"active"`
	EventsProcessed int64     `json:"events_processed"`
	ErrorCount      int       `json:"error_count"`
	LastEventTime   time.Time `json:"last_event_time,omitempty"`
}

// ============================================================================
// Debug Info Types
// ============================================================================

// DebugInfoParams defines parameters for debug_info queries
type DebugInfoParams struct {
	// Mode: "overview", "symbols", "references", "types", "files"
	Mode string `json:"mode,omitempty"`

	// Filters
	FileID   int    `json:"file_id,omitempty"`
	FilePath string `json:"file_path,omitempty"`

	// Options
	MaxResults int  `json:"max_results,omitempty"`
	Verbose    bool `json:"verbose,omitempty"`
}

// DebugInfoResponse contains debug information
type DebugInfoResponse struct {
	Mode      string    `json:"mode"`
	Timestamp time.Time `json:"timestamp"`

	// Overview stats
	Overview *DebugOverview `json:"overview,omitempty"`

	// Detailed breakdowns
	SymbolsByType    map[string]int      `json:"symbols_by_type,omitempty"`
	SymbolsByFile    map[string]int      `json:"symbols_by_file,omitempty"`
	FilesByLanguage  map[string]int      `json:"files_by_language,omitempty"`
	TopReferencedSymbols []SymbolRefCount `json:"top_referenced_symbols,omitempty"`

	// Specific file info
	FileInfo *FileDebugInfo `json:"file_info,omitempty"`
}

// DebugOverview provides high-level debug statistics
type DebugOverview struct {
	TotalFiles        int            `json:"total_files"`
	TotalSymbols      int            `json:"total_symbols"`
	TotalReferences   int            `json:"total_references"`
	UniqueLanguages   int            `json:"unique_languages"`
	AvgSymbolsPerFile float64        `json:"avg_symbols_per_file"`
	AvgRefsPerSymbol  float64        `json:"avg_refs_per_symbol"`
	LanguageBreakdown map[string]int `json:"language_breakdown"`
	TypeBreakdown     map[string]int `json:"type_breakdown"`
}

// SymbolRefCount tracks symbol reference counts
type SymbolRefCount struct {
	SymbolName  string `json:"symbol_name"`
	SymbolType  string `json:"symbol_type"`
	FilePath    string `json:"file_path"`
	IncomingRefs int   `json:"incoming_refs"`
	OutgoingRefs int   `json:"outgoing_refs"`
}

// FileDebugInfo contains debug info for a specific file
type FileDebugInfo struct {
	FileID      types.FileID `json:"file_id"`
	FilePath    string       `json:"file_path"`
	Language    string       `json:"language"`
	SymbolCount int          `json:"symbol_count"`
	LineCount   int          `json:"line_count"`
	Symbols     []SymbolDebugInfo `json:"symbols,omitempty"`
}

// SymbolDebugInfo contains debug info for a symbol
type SymbolDebugInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Line        int    `json:"line"`
	IsExported  bool   `json:"is_exported"`
	IncomingRefs int   `json:"incoming_refs"`
	OutgoingRefs int   `json:"outgoing_refs"`
}

// ============================================================================
// Handler Functions
// ============================================================================

// handleIndexStats handles the index_stats tool requests
// @lci:labels[mcp-tool-handler,index-management,status,diagnostics]
// @lci:category[mcp-api]
func (s *Server) handleIndexStats(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params IndexStatusParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createSmartErrorResponse("index_stats", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"correct_format": map[string]interface{}{
				"mode":              "summary",
				"include_memory":    true,
				"include_components": true,
			},
			"available_modes": []string{"summary", "detailed", "progress", "health"},
		})
	}

	// Set default mode
	if params.Mode == "" {
		params.Mode = "summary"
	}

	response := &IndexStatusResponse{
		Timestamp:   time.Now(),
		ServerReady: s.goroutineIndex != nil,
	}

	// Determine overall status
	if s.goroutineIndex == nil {
		response.Status = "initializing"
		response.Issues = append(response.Issues, "Index not yet initialized")
		return createJSONResponse(response)
	}

	// Check if indexing is in progress
	err := s.goroutineIndex.CheckIndexingComplete()
	if err != nil {
		response.Status = "indexing"
		// Get progress info
		progress := s.goroutineIndex.GetProgress()
		response.Progress = &IndexingProgressInfo{
			IsIndexing:       true,
			IsScanning:       progress.IsScanning,
			TotalFiles:       progress.TotalFiles,
			FilesProcessed:   progress.FilesProcessed,
			FilesSkipped:     0, // Not tracked separately
			ScanningProgress: progress.ScanningProgress,
			IndexingProgress: progress.IndexingProgress,
		}
		// Calculate overall progress
		if progress.IsScanning {
			response.Progress.OverallProgress = progress.ScanningProgress * 0.1
		} else {
			response.Progress.OverallProgress = 10.0 + (progress.IndexingProgress * 0.9)
		}
	} else {
		response.Status = "ready"
	}

	// Get basic stats
	stats := s.goroutineIndex.GetIndexStats()
	response.FileCount = stats.FileCount
	response.SymbolCount = stats.SymbolCount
	response.ReferenceCount = stats.ReferenceCount
	response.TotalSizeBytes = stats.TotalSizeBytes
	response.IndexTimeMs = stats.IndexTimeMs

	// Include component health for detailed/health modes
	if params.Mode == "detailed" || params.Mode == "health" || params.IncludeComponents {
		response.ComponentHealth = s.getComponentHealth()
	}

	// Include memory info
	if params.Mode == "detailed" || params.IncludeMemory {
		response.MemoryUsage = s.getMemoryUsage()
	}

	// Include watch mode info
	if params.Mode == "detailed" || params.IncludeWatchMode {
		response.WatchMode = s.getWatchModeInfo()
	}

	// Collect any issues/warnings
	if response.ComponentHealth != nil {
		if !response.ComponentHealth.CallGraphPopulated {
			response.Warnings = append(response.Warnings, "Call graph is empty - relationship queries may return no data")
		}
		if !response.ComponentHealth.SideEffectsReady {
			response.Warnings = append(response.Warnings, "Side effects analysis not available")
		}
	}

	return createJSONResponse(response)
}

// handleDebugInfo handles the debug_info tool requests
// @lci:labels[mcp-tool-handler,debug,diagnostics,symbols]
// @lci:category[mcp-api]
func (s *Server) handleDebugInfo(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params DebugInfoParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createSmartErrorResponse("debug_info", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"correct_format": map[string]interface{}{
				"mode":        "overview",
				"max_results": 20,
			},
			"available_modes": []string{"overview", "symbols", "references", "types", "files"},
		})
	}

	// Set defaults
	if params.Mode == "" {
		params.Mode = "overview"
	}
	if params.MaxResults == 0 {
		params.MaxResults = 20
	}

	// Check index availability
	if s.goroutineIndex == nil {
		return createErrorResponse("debug_info", fmt.Errorf("index not initialized"))
	}

	response := &DebugInfoResponse{
		Mode:      params.Mode,
		Timestamp: time.Now(),
	}

	switch params.Mode {
	case "overview":
		response.Overview = s.getDebugOverview()
	case "symbols":
		response.SymbolsByType = s.getSymbolsByType()
	case "references":
		response.TopReferencedSymbols = s.getTopReferencedSymbols(params.MaxResults)
	case "types":
		response.SymbolsByType = s.getSymbolsByType()
	case "files":
		response.FilesByLanguage = s.getFilesByLanguage()
		if params.FileID > 0 || params.FilePath != "" {
			response.FileInfo = s.getFileDebugInfo(types.FileID(params.FileID), params.FilePath, params.Verbose)
		}
	default:
		return createErrorResponse("debug_info", fmt.Errorf("unknown mode: %s", params.Mode))
	}

	return createJSONResponse(response)
}

// ============================================================================
// Helper Functions
// ============================================================================

func (s *Server) getComponentHealth() *ComponentHealthInfo {
	health := &ComponentHealthInfo{}

	// Check symbol index
	symbolIndex := s.goroutineIndex.GetSymbolIndex()
	health.SymbolIndexReady = symbolIndex != nil
	if symbolIndex != nil {
		stats := symbolIndex.GetStats()
		health.SymbolStats = map[string]interface{}{
			"total_symbols":  stats.TotalSymbols,
			"exported_count": len(stats.ExportedSymbols),
			"function_count": stats.TotalFunctions,
			"type_count":     stats.TotalTypes,
		}
	}

	// Check trigram index
	trigramIndex := s.goroutineIndex.GetTrigramIndex()
	health.TrigramIndexReady = trigramIndex != nil

	// Check reference tracker
	refTracker := s.goroutineIndex.GetRefTracker()
	health.RefTrackerReady = refTracker != nil
	if refTracker != nil {
		health.CallGraphPopulated = refTracker.HasRelationships()
		refStats := refTracker.GetRelationshipStats()
		health.ReferenceStats = map[string]interface{}{
			"total_symbols":           refStats["total_symbols"],
			"total_references":        refStats["total_references"],
			"symbols_with_incoming":   refStats["symbols_with_incoming_refs"],
			"symbols_with_outgoing":   refStats["symbols_with_outgoing_refs"],
		}
	}

	// Check side effects propagator
	sideEffects := s.goroutineIndex.GetSideEffectPropagator()
	health.SideEffectsReady = sideEffects != nil

	// Check semantic index
	semanticIndex := s.goroutineIndex.GetSemanticSearchIndex()
	health.SemanticIndexReady = semanticIndex != nil

	// Check file content store
	contentStore := s.goroutineIndex.GetFileContentStore()
	health.FileContentStoreReady = contentStore != nil

	return health
}

func (s *Server) getMemoryUsage() *MemoryUsageInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pressureInfo := s.goroutineIndex.GetMemoryPressureInfo()
	pressureLevel := "low"
	if level, ok := pressureInfo["pressure_level"].(string); ok {
		pressureLevel = level
	}

	estimatedIndex := float64(0)
	if estimate, ok := pressureInfo["estimated_usage_mb"].(float64); ok {
		estimatedIndex = estimate
	}

	return &MemoryUsageInfo{
		HeapAllocMB:      float64(m.HeapAlloc) / 1024 / 1024,
		HeapSysMB:        float64(m.HeapSys) / 1024 / 1024,
		TotalAllocMB:     float64(m.TotalAlloc) / 1024 / 1024,
		NumGC:            m.NumGC,
		EstimatedIndexMB: estimatedIndex,
		PressureLevel:    pressureLevel,
	}
}

func (s *Server) getWatchModeInfo() *WatchModeInfo {
	stats := s.goroutineIndex.GetStats()
	watchMode, ok := stats["watch_mode"].(map[string]interface{})
	if !ok {
		return &WatchModeInfo{Enabled: false}
	}

	info := &WatchModeInfo{}
	if enabled, ok := watchMode["enabled"].(bool); ok {
		info.Enabled = enabled
	}
	if active, ok := watchMode["active"].(bool); ok {
		info.Active = active
	}
	if events, ok := watchMode["events_processed"].(int64); ok {
		info.EventsProcessed = events
	}
	if errors, ok := watchMode["error_count"].(int); ok {
		info.ErrorCount = errors
	}
	if lastEvent, ok := watchMode["last_event"].(time.Time); ok {
		info.LastEventTime = lastEvent
	}

	return info
}

func (s *Server) getDebugOverview() *DebugOverview {
	stats := s.goroutineIndex.GetIndexStats()
	typeDistribution := s.goroutineIndex.GetTypeDistribution()

	// Convert type distribution to string keys
	typeBreakdown := make(map[string]int)
	for t, count := range typeDistribution {
		typeBreakdown[t.String()] = count
	}

	// Get language breakdown from files
	languageBreakdown := s.getFilesByLanguage()

	avgSymbolsPerFile := float64(0)
	if stats.FileCount > 0 {
		avgSymbolsPerFile = float64(stats.SymbolCount) / float64(stats.FileCount)
	}

	avgRefsPerSymbol := float64(0)
	if stats.SymbolCount > 0 {
		avgRefsPerSymbol = float64(stats.ReferenceCount) / float64(stats.SymbolCount)
	}

	return &DebugOverview{
		TotalFiles:        stats.FileCount,
		TotalSymbols:      stats.SymbolCount,
		TotalReferences:   stats.ReferenceCount,
		UniqueLanguages:   len(languageBreakdown),
		AvgSymbolsPerFile: avgSymbolsPerFile,
		AvgRefsPerSymbol:  avgRefsPerSymbol,
		LanguageBreakdown: languageBreakdown,
		TypeBreakdown:     typeBreakdown,
	}
}

func (s *Server) getSymbolsByType() map[string]int {
	typeDistribution := s.goroutineIndex.GetTypeDistribution()
	result := make(map[string]int)
	for t, count := range typeDistribution {
		result[t.String()] = count
	}
	return result
}

func (s *Server) getFilesByLanguage() map[string]int {
	result := make(map[string]int)
	files := s.goroutineIndex.GetAllFiles()
	for _, f := range files {
		if f != nil {
			lang := analysis.GetLanguageFromPath(f.Path)
			if lang == "" {
				lang = "unknown"
			}
			result[lang]++
		}
	}
	return result
}

func (s *Server) getTopReferencedSymbols(maxResults int) []SymbolRefCount {
	refTracker := s.goroutineIndex.GetRefTracker()
	if refTracker == nil {
		return nil
	}

	// Get symbols with most references
	topSymbols := s.goroutineIndex.GetTopSymbols(maxResults)
	result := make([]SymbolRefCount, 0, len(topSymbols))

	for _, sym := range topSymbols {
		filePath := s.goroutineIndex.GetFilePath(sym.Symbol.FileID)
		// Score contains the total reference count
		// For detailed incoming/outgoing breakdown, we'd need to query the ref tracker
		// but for top symbols we just use the score as incoming (it's typically computed from incoming refs)
		result = append(result, SymbolRefCount{
			SymbolName:   sym.Symbol.Name,
			SymbolType:   sym.Symbol.Type.String(),
			FilePath:     filePath,
			IncomingRefs: sym.Score, // Score is the reference count
			OutgoingRefs: 0,          // Not tracked in TopSymbols, would need separate query
		})
	}

	return result
}

func (s *Server) getFileDebugInfo(fileID types.FileID, filePath string, verbose bool) *FileDebugInfo {
	// If filePath provided, find fileID
	if filePath != "" && fileID == 0 {
		files := s.goroutineIndex.GetAllFiles()
		for _, f := range files {
			if f != nil && f.Path == filePath {
				fileID = f.ID
				break
			}
		}
	}

	if fileID == 0 {
		return nil
	}

	fileInfo := s.goroutineIndex.GetFile(fileID)
	if fileInfo == nil {
		return nil
	}

	result := &FileDebugInfo{
		FileID:      fileID,
		FilePath:    fileInfo.Path,
		Language:    analysis.GetLanguageFromPath(fileInfo.Path),
		SymbolCount: len(fileInfo.EnhancedSymbols),
		LineCount:   s.goroutineIndex.GetFileLineCount(fileID),
	}

	if verbose {
		refTracker := s.goroutineIndex.GetRefTracker()
		symbols := s.goroutineIndex.GetFileEnhancedSymbols(fileID)
		result.Symbols = make([]SymbolDebugInfo, 0, len(symbols))

		for _, sym := range symbols {
			if sym == nil {
				continue
			}
			incoming := 0
			outgoing := 0
			if refTracker != nil {
				refs := refTracker.GetSymbolReferences(sym.ID, "incoming")
				incoming = len(refs)
				refs = refTracker.GetSymbolReferences(sym.ID, "outgoing")
				outgoing = len(refs)
			}
			result.Symbols = append(result.Symbols, SymbolDebugInfo{
				Name:         sym.Name,
				Type:         sym.Type.String(),
				Line:         sym.Line,
				IsExported:   sym.IsExported,
				IncomingRefs: incoming,
				OutgoingRefs: outgoing,
			})
		}
	}

	return result
}

// ========== Git Analysis Tool ==========

// GitAnalysisRequest represents the git analysis MCP tool request
type GitAnalysisRequest struct {
	Scope               string   `json:"scope"`                  // staged, wip, commit, range
	BaseRef             string   `json:"base_ref,omitempty"`     // For commit/range scope
	TargetRef           string   `json:"target_ref,omitempty"`   // For range scope
	Focus               []string `json:"focus,omitempty"`        // duplicates, naming, metrics
	SimilarityThreshold float64  `json:"similarity_threshold,omitempty"`
	MaxFindings         int      `json:"max_findings,omitempty"`
}

// GitAnalysisResponse represents the git analysis MCP tool response
type GitAnalysisResponse struct {
	Summary       *git.ReportSummary     `json:"summary"`
	Duplicates    []git.DuplicateFinding `json:"duplicates,omitempty"`
	NamingIssues  []git.NamingFinding    `json:"naming_issues,omitempty"`
	MetricsIssues []git.MetricsFinding   `json:"metrics_issues,omitempty"`
	Metadata      *git.ReportMetadata    `json:"metadata"`
	Error         string                 `json:"error,omitempty"`
}

// handleGitAnalysis handles the git_analysis MCP tool
func (s *Server) handleGitAnalysis(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	var params GitAnalysisRequest
	paramsJSON, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return createErrorResponse("git_analysis", fmt.Errorf("failed to marshal params: %v", err))
	}
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return createErrorResponse("git_analysis", fmt.Errorf("failed to parse params: %v", err))
	}

	// Default scope to "staged" if not specified
	if params.Scope == "" {
		params.Scope = "staged"
	}

	// Convert scope string to AnalysisScope
	var scope git.AnalysisScope
	switch params.Scope {
	case "staged":
		scope = git.ScopeStaged
	case "wip":
		scope = git.ScopeWIP
	case "commit":
		scope = git.ScopeCommit
	case "range":
		scope = git.ScopeRange
	default:
		return createErrorResponse("git_analysis", fmt.Errorf("invalid scope: %s (must be staged, wip, commit, or range)", params.Scope))
	}

	// Check for required args
	if scope == git.ScopeRange && params.BaseRef == "" {
		return createErrorResponse("git_analysis", fmt.Errorf("base_ref is required for range scope"))
	}

	// Get project root from config
	var projectRoot string
	if s.cfg != nil && s.cfg.Project.Root != "" {
		projectRoot = s.cfg.Project.Root
	}
	if projectRoot == "" {
		return createErrorResponse("git_analysis", fmt.Errorf("project root not configured"))
	}

	// Create git provider
	gitProvider, err := git.NewProvider(projectRoot)
	if err != nil {
		return createErrorResponse("git_analysis", fmt.Errorf("failed to create git provider: %v", err))
	}

	// Create analyzer with indexer
	analyzer := git.NewAnalyzer(gitProvider, s.goroutineIndex)

	// Build params
	analysisParams := git.DefaultAnalysisParams()
	analysisParams.Scope = scope
	if params.BaseRef != "" {
		analysisParams.BaseRef = params.BaseRef
	}
	if params.TargetRef != "" {
		analysisParams.TargetRef = params.TargetRef
	}
	if len(params.Focus) > 0 {
		analysisParams.Focus = params.Focus
	}
	if params.SimilarityThreshold > 0 {
		analysisParams.SimilarityThreshold = params.SimilarityThreshold
	}
	if params.MaxFindings > 0 {
		analysisParams.MaxFindings = params.MaxFindings
	}

	// Run analysis
	report, err := analyzer.Analyze(ctx, analysisParams)
	if err != nil {
		return createErrorResponse("git_analysis", fmt.Errorf("analysis failed: %v", err))
	}

	// Build response
	response := GitAnalysisResponse{
		Summary:       &report.Summary,
		Duplicates:    report.Duplicates,
		NamingIssues:  report.NamingIssues,
		MetricsIssues: report.MetricsIssues,
		Metadata:      &report.Metadata,
	}

	return createJSONResponse(response)
}
