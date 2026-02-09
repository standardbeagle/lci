package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/git"
	"github.com/standardbeagle/lci/internal/idcodec"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/internal/version"
)

// IndexServer manages a persistent index that can be shared between CLI and MCP
type IndexServer struct {
	indexer        *indexing.MasterIndex
	searchEngine   *search.Engine
	cfg            *config.Config
	listener       net.Listener
	server         *http.Server
	startTime      time.Time
	shutdownChan   chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	running        bool
	indexingActive bool
	socketPath     string // Custom socket path (empty uses default)
}

// NewIndexServer creates a new persistent index server
func NewIndexServer(cfg *config.Config) (*IndexServer, error) {
	// Create the master index
	indexer := indexing.NewMasterIndex(cfg)

	// Create search engine - will be initialized after indexing
	var searchEngine *search.Engine

	return &IndexServer{
		indexer:        indexer,
		searchEngine:   searchEngine,
		cfg:            cfg,
		startTime:      time.Now(),
		shutdownChan:   make(chan struct{}),
		indexingActive: false,
	}, nil
}

// NewIndexServerWithIndex creates a new persistent index server with an existing MasterIndex
// This is used when the index is managed externally (e.g., by MCP)
func NewIndexServerWithIndex(cfg *config.Config, indexer *indexing.MasterIndex, searchEngine *search.Engine) (*IndexServer, error) {
	return &IndexServer{
		indexer:        indexer,
		searchEngine:   searchEngine,
		cfg:            cfg,
		startTime:      time.Now(),
		shutdownChan:   make(chan struct{}),
		indexingActive: false, // External caller manages indexing
	}, nil
}

// GetSocketPath returns the default path to the Unix socket (for backwards compatibility)
func GetSocketPath() string {
	// Use temp directory for socket
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "lci-server.sock")
}

// GetSocketPathForRoot returns a project-specific socket path based on the root directory
// This allows multiple servers to run for different projects simultaneously
func GetSocketPathForRoot(root string) string {
	if root == "" {
		return GetSocketPath()
	}
	// Create a hash of the absolute path to generate a unique socket name
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return GetSocketPath()
	}
	// Use a simple hash to create a unique but deterministic socket name
	hash := uint32(0)
	for _, c := range absRoot {
		hash = hash*31 + uint32(c)
	}
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, fmt.Sprintf("lci-server-%08x.sock", hash))
}

// SetSocketPath sets a custom socket path for this server (used for testing)
func (s *IndexServer) SetSocketPath(path string) {
	s.socketPath = path
}

// GetServerSocketPath returns the socket path this server is using
func (s *IndexServer) GetServerSocketPath() string {
	if s.socketPath != "" {
		return s.socketPath
	}
	return GetSocketPath()
}

// Start begins listening for client connections
func (s *IndexServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Remove existing socket if present
	socketPath := s.GetServerSocketPath()
	os.Remove(socketPath)

	// Create Unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	s.listener = listener

	// Make socket accessible to user
	os.Chmod(socketPath, 0600)

	// Create HTTP server for RPC
	mux := http.NewServeMux()
	s.registerHandlers(mux)

	s.server = &http.Server{
		Handler: mux,
	}

	// Start indexing in background only if search engine not already set
	// (When using NewIndexServerWithIndex, the index is managed externally)
	s.mu.Lock()
	hasSearchEngine := s.searchEngine != nil
	s.mu.Unlock()

	if !hasSearchEngine {
		go func() {
			s.mu.Lock()
			s.indexingActive = true
			s.mu.Unlock()

			debug.LogMCP("Starting indexing of %s...", s.cfg.Project.Root)
			if err := s.indexer.IndexDirectory(context.Background(), s.cfg.Project.Root); err != nil {
				debug.LogMCP("Indexing error: %v", err)
			} else {
				debug.LogMCP("Indexing completed successfully")
			}

			// Create search engine after indexing completes
			s.mu.Lock()
			s.searchEngine = search.NewEngine(s.indexer)
			s.indexer.SetSearchEngine(s.searchEngine)
			s.indexingActive = false
			s.mu.Unlock()

			debug.LogMCP("Index ready for queries")
		}()
	} else {
		debug.LogMCP("Using externally managed index (ready immediately)")
	}

	// Start serving
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			debug.LogMCP("Server error: %v", err)
		}
	}()

	debug.LogMCP("Index server started on %s (pid: %d)", socketPath, os.Getpid())
	debug.LogMCP("Project root: %s", s.cfg.Project.Root)

	return nil
}

// registerHandlers sets up RPC endpoints
func (s *IndexServer) registerHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/symbol", s.handleGetSymbol)
	mux.HandleFunc("/fileinfo", s.handleGetFileInfo)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/reindex", s.handleReindex)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/definition", s.handleDefinition)
	mux.HandleFunc("/references", s.handleReferences)
	mux.HandleFunc("/tree", s.handleTree)
	mux.HandleFunc("/git-analyze", s.handleGitAnalyze)
	mux.HandleFunc("/list-symbols", s.handleListSymbols)
	mux.HandleFunc("/inspect-symbol", s.handleInspectSymbol)
	mux.HandleFunc("/browse-file", s.handleBrowseFile)
}

// handleStatus returns the current index status
func (s *IndexServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	indexingActive := s.indexingActive
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	// Get index statistics
	fileCount := 0
	symbolCount := 0
	if ready {
		fileCount = s.indexer.GetFileCount()
		symbolCount = s.indexer.GetSymbolCount()
	}

	status := IndexStatus{
		Ready:          ready,
		FileCount:      fileCount,
		SymbolCount:    symbolCount,
		IndexingActive: indexingActive,
		Progress:       1.0, // TODO: Add real progress tracking
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleSearch performs a search query
func (s *IndexServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	engine := s.searchEngine
	s.mu.RUnlock()

	if engine == nil {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Perform search using engine
	results := engine.SearchWithOptions(req.Pattern, nil, req.Options)

	// Limit results if requested
	if req.MaxResults > 0 && len(results) > req.MaxResults {
		results = results[:req.MaxResults]
	}

	response := SearchResponse{
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetSymbol retrieves symbol information
func (s *IndexServer) handleGetSymbol(w http.ResponseWriter, r *http.Request) {
	var req GetSymbolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	symbol := s.indexer.GetEnhancedSymbol(req.SymbolID)
	if symbol == nil {
		http.Error(w, fmt.Sprintf("symbol %d not found", req.SymbolID), http.StatusNotFound)
		return
	}

	response := GetSymbolResponse{
		Symbol: symbol,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetFileInfo retrieves file information
func (s *IndexServer) handleGetFileInfo(w http.ResponseWriter, r *http.Request) {
	var req GetFileInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	fileInfo := s.indexer.GetFileInfo(req.FileID)
	if fileInfo == nil {
		http.Error(w, fmt.Sprintf("file %d not found", req.FileID), http.StatusNotFound)
		return
	}

	response := GetFileInfoResponse{
		FileInfo: fileInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleShutdown gracefully shuts down the server
func (s *IndexServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	var req ShutdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req = ShutdownRequest{}
	}

	response := ShutdownResponse{
		Success: true,
		Message: "Server shutting down",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	// Trigger shutdown after response is sent
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(s.shutdownChan)
	}()
}

// handlePing responds to health check requests
func (s *IndexServer) handlePing(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime).Seconds()
	response := PingResponse{
		Uptime:  uptime,
		Version: version.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStats returns index statistics including file count, symbol count, and memory usage
func (s *IndexServer) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Get index stats from the indexer
	indexStats := s.indexer.Stats()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Extract search statistics from AdditionalStats
	var searchCount int64
	var avgSearchTimeMs float64
	if indexStats.AdditionalStats != nil {
		if sc, ok := indexStats.AdditionalStats["search_count"].(int64); ok {
			searchCount = sc
		}
		if ast, ok := indexStats.AdditionalStats["avg_search_time_ms"].(float64); ok {
			avgSearchTimeMs = ast
		}
	}

	response := StatsResponse{
		FileCount:       indexStats.TotalFiles,
		SymbolCount:     indexStats.TotalSymbols,
		IndexSizeBytes:  indexStats.IndexSize,
		BuildDurationMs: indexStats.BuildDuration.Milliseconds(),
		MemoryAllocMB:   float64(memStats.Alloc) / 1024 / 1024,
		MemoryTotalMB:   float64(memStats.TotalAlloc) / 1024 / 1024,
		MemoryHeapMB:    float64(memStats.HeapAlloc) / 1024 / 1024,
		NumGoroutines:   runtime.NumGoroutine(),
		UptimeSeconds:   time.Since(s.startTime).Seconds(),
		SearchCount:     searchCount,
		AvgSearchTimeMs: avgSearchTimeMs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDefinition searches for symbol definitions by name pattern
func (s *IndexServer) handleDefinition(w http.ResponseWriter, r *http.Request) {
	var req DefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	engine := s.searchEngine
	s.mu.RUnlock()

	if engine == nil {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Build search options for definition search
	searchOpts := types.SearchOptions{
		SymbolTypes: []string{"function", "class", "struct", "interface", "type", "method"},
		DeclarationOnly: true, // Only find definitions, not usages
	}

	// Perform search using engine
	results := engine.SearchWithOptions(req.Pattern, nil, searchOpts)

	// Limit results if requested
	if req.MaxResults > 0 && len(results) > req.MaxResults {
		results = results[:req.MaxResults]
	}

	// Convert results to DefinitionLocation structs
	definitions := make([]DefinitionLocation, 0, len(results))
	for _, result := range results {
		// Prefer BlockName as it's the actual symbol name from the index
		// Fall back to Match if BlockName is empty
		name := result.Context.BlockName
		if name == "" {
			name = result.Match
		}

		def := DefinitionLocation{
			Name:     name,
			Type:     result.Context.BlockType,
			FilePath: result.Path,
			Line:     result.Line,
			Column:   result.Column,
		}

		definitions = append(definitions, def)
	}

	response := DefinitionResponse{
		Definitions: definitions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTree generates a function call hierarchy tree
func (s *IndexServer) handleTree(w http.ResponseWriter, r *http.Request) {
	var req TreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Build tree options from request
	treeOpts := types.TreeOptions{
		MaxDepth:       req.MaxDepth,
		ShowLines:      req.ShowLines,
		Compact:        req.Compact,
		ExcludePattern: req.Exclude,
		AgentMode:      req.AgentMode,
	}

	// Generate the function tree using the indexer
	tree, err := s.indexer.GenerateFunctionTree(req.FunctionName, treeOpts)
	if err != nil {
		response := TreeResponse{
			Error: fmt.Sprintf("failed to generate tree: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := TreeResponse{
		Tree: tree,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReferences searches for symbol references (usages, not definitions)
func (s *IndexServer) handleReferences(w http.ResponseWriter, r *http.Request) {
	var req ReferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	engine := s.searchEngine
	s.mu.RUnlock()

	if engine == nil {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Build search options for reference search
	// Unlike definition search, we don't set DeclarationOnly=true
	// This will find all usages of the symbol, not just declarations
	searchOpts := types.SearchOptions{
		DeclarationOnly: false, // Get all usages, not just declarations
	}

	// Perform search using engine
	results := engine.SearchWithOptions(req.Pattern, nil, searchOpts)

	// Limit results if requested
	if req.MaxResults > 0 && len(results) > req.MaxResults {
		results = results[:req.MaxResults]
	}

	// Convert results to ReferenceLocation structs
	references := make([]ReferenceLocation, 0, len(results))
	for _, result := range results {
		// Get context from the Lines slice
		contextStr := ""
		if len(result.Context.Lines) > 0 {
			// Find the line containing the match (relative index)
			lineIdx := result.Line - result.Context.StartLine
			if lineIdx >= 0 && lineIdx < len(result.Context.Lines) {
				contextStr = result.Context.Lines[lineIdx]
			} else {
				// Fallback to first line if index is out of range
				contextStr = result.Context.Lines[0]
			}
		}

		ref := ReferenceLocation{
			FilePath: result.Path,
			Line:     result.Line,
			Column:   result.Column,
			Context:  contextStr,
			Match:    result.Match,
		}
		references = append(references, ref)
	}

	response := ReferencesResponse{
		References: references,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReindex triggers a re-index of the project
func (s *IndexServer) handleReindex(w http.ResponseWriter, r *http.Request) {
	var req ReindexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req = ReindexRequest{}
	}

	rootPath := req.Path
	if rootPath == "" {
		rootPath = s.cfg.Project.Root
	}

	// Start re-indexing in background
	go func() {
		s.mu.Lock()
		s.indexingActive = true
		s.searchEngine = nil // Invalidate during reindex
		s.mu.Unlock()

		debug.LogMCP("Re-indexing %s...", rootPath)
		if err := s.indexer.IndexDirectory(context.Background(), rootPath); err != nil {
			debug.LogMCP("Re-indexing error: %v", err)
		} else {
			debug.LogMCP("Re-indexing completed successfully")
		}

		// Recreate search engine
		s.mu.Lock()
		s.searchEngine = search.NewEngine(s.indexer)
		s.indexer.SetSearchEngine(s.searchEngine)
		s.indexingActive = false
		s.mu.Unlock()

		debug.LogMCP("Index ready after reindex")
	}()

	response := ReindexResponse{
		Success: true,
		Message: fmt.Sprintf("Re-indexing started for %s", rootPath),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Wait blocks until the server is shut down
func (s *IndexServer) Wait() {
	<-s.shutdownChan
}

// Shutdown gracefully shuts down the server
func (s *IndexServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	// Shutdown HTTP server
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
	}

	// Wait for goroutines
	s.wg.Wait()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Remove socket file
	os.Remove(s.GetServerSocketPath())

	debug.LogMCP("Index server shut down cleanly")
	runtime.GC() // Force GC to release memory

	return nil
}

// GetMemoryStats returns current memory usage
func (s *IndexServer) GetMemoryStats() runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}

// handleGitAnalyze performs git change analysis
func (s *IndexServer) handleGitAnalyze(w http.ResponseWriter, r *http.Request) {
	var req GitAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Convert scope string to AnalysisScope
	var scope git.AnalysisScope
	switch req.Scope {
	case "staged":
		scope = git.ScopeStaged
	case "wip":
		scope = git.ScopeWIP
	case "commit":
		scope = git.ScopeCommit
	case "range":
		scope = git.ScopeRange
	default:
		response := GitAnalyzeResponse{Error: fmt.Sprintf("invalid scope: %s", req.Scope)}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create git provider
	gitProvider, err := git.NewProvider(s.cfg.Project.Root)
	if err != nil {
		response := GitAnalyzeResponse{Error: fmt.Sprintf("failed to create git provider: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create analyzer
	analyzer := git.NewAnalyzer(gitProvider, s.indexer)

	// Build params
	params := git.DefaultAnalysisParams()
	params.Scope = scope
	if req.BaseRef != "" {
		params.BaseRef = req.BaseRef
	}
	if req.TargetRef != "" {
		params.TargetRef = req.TargetRef
	}
	if len(req.Focus) > 0 {
		params.Focus = req.Focus
	}
	if req.SimilarityThreshold > 0 {
		params.SimilarityThreshold = req.SimilarityThreshold
	}
	if req.MaxFindings > 0 {
		params.MaxFindings = req.MaxFindings
	}

	// Run analysis
	report, err := analyzer.Analyze(context.Background(), params)
	if err != nil {
		response := GitAnalyzeResponse{Error: fmt.Sprintf("analysis failed: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := GitAnalyzeResponse{Report: report}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleListSymbols enumerates and filters symbols in the index
func (s *IndexServer) handleListSymbols(w http.ResponseWriter, r *http.Request) {
	var req ListSymbolsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Delegate to the indexer's symbol enumeration
	allFileIDs := s.indexer.GetAllFileIDsFiltered()
	kinds := parseHTTPSymbolKinds(req.Kind)
	maxResults := req.Max
	if maxResults <= 0 {
		maxResults = 50
	}
	if maxResults > 500 {
		maxResults = 500
	}

	tracker := s.indexer.GetRefTracker()

	var allEntries []ListSymbolsEntry
	for _, fileID := range allFileIDs {
		filePath := s.indexer.GetFilePath(fileID)
		if filePath == "" {
			continue
		}
		if req.File != "" {
			matched, _ := filepath.Match(req.File, filePath)
			if !matched {
				matched, _ = filepath.Match(req.File, filepath.Base(filePath))
			}
			if !matched {
				continue
			}
		}
		symbols := s.indexer.GetFileEnhancedSymbols(fileID)
		for _, sym := range symbols {
			if !matchesHTTPListFilters(sym, kinds, req) {
				continue
			}
			entry := buildHTTPSymbolEntry(sym, filePath, tracker)
			allEntries = append(allEntries, entry)
		}
	}

	total := len(allEntries)

	// Apply offset/limit
	if req.Offset > 0 && req.Offset < len(allEntries) {
		allEntries = allEntries[req.Offset:]
	} else if req.Offset >= len(allEntries) {
		allEntries = nil
	}
	if len(allEntries) > maxResults {
		allEntries = allEntries[:maxResults]
	}

	resp := ListSymbolsHTTPResponse{
		Symbols: allEntries,
		Total:   total,
		Showing: len(allEntries),
		HasMore: total > req.Offset+len(allEntries),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleInspectSymbol provides deep inspection of a symbol
func (s *IndexServer) handleInspectSymbol(w http.ResponseWriter, r *http.Request) {
	var req InspectSymbolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	var matched []*types.EnhancedSymbol

	if req.ID != "" {
		symbolID, err := idcodec.DecodeSymbolID(req.ID)
		if err == nil {
			tracker := s.indexer.GetRefTracker()
			if tracker != nil {
				if sym := tracker.GetEnhancedSymbol(symbolID); sym != nil {
					matched = append(matched, sym)
				}
			}
		}
	}

	if req.Name != "" && len(matched) == 0 {
		matched = s.indexer.FindSymbolsByName(req.Name)
	}

	// Apply disambiguators
	if req.File != "" || req.Type != "" {
		filtered := matched[:0]
		for _, sym := range matched {
			filePath := s.indexer.GetFilePath(sym.FileID)
			if req.File != "" {
				m, _ := filepath.Match(req.File, filePath)
				if !m {
					m, _ = filepath.Match(req.File, filepath.Base(filePath))
				}
				if !m {
					continue
				}
			}
			if req.Type != "" {
				expectedKinds := parseHTTPSymbolKinds(req.Type)
				if expectedKinds != nil && !expectedKinds[sym.Symbol.Type] {
					continue
				}
			}
			filtered = append(filtered, sym)
		}
		matched = filtered
	}

	tracker := s.indexer.GetRefTracker()
	results := make([]InspectSymbolEntry, len(matched))
	for i, sym := range matched {
		results[i] = buildHTTPInspectEntry(sym, s.indexer.GetFilePath(sym.FileID), tracker, s.indexer)
	}

	resp := InspectSymbolHTTPResponse{
		Symbols: results,
		Count:   len(results),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleBrowseFile lists all symbols in a specific file
func (s *IndexServer) handleBrowseFile(w http.ResponseWriter, r *http.Request) {
	var req BrowseFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	ready := s.searchEngine != nil
	s.mu.RUnlock()

	if !ready {
		http.Error(w, "index not ready - still indexing", http.StatusServiceUnavailable)
		return
	}

	// Find the target file
	var targetFileID types.FileID
	var targetFilePath string
	found := false

	if req.FileID != nil {
		targetFileID = types.FileID(*req.FileID)
		targetFilePath = s.indexer.GetFilePath(targetFileID)
		if targetFilePath != "" {
			found = true
		}
	}

	if !found && req.File != "" {
		allFileIDs := s.indexer.GetAllFileIDsFiltered()
		for _, fid := range allFileIDs {
			fp := s.indexer.GetFilePath(fid)
			if fp == "" {
				continue
			}
			if fp == req.File || strings.HasSuffix(fp, "/"+req.File) || strings.HasSuffix(fp, "\\"+req.File) {
				targetFileID = fid
				targetFilePath = fp
				found = true
				break
			}
			if m, _ := filepath.Match(req.File, fp); m {
				targetFileID = fid
				targetFilePath = fp
				found = true
				break
			}
		}
	}

	if !found {
		resp := BrowseFileHTTPResponse{Error: fmt.Sprintf("file not found: %s", req.File)}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	kinds := parseHTTPSymbolKinds(req.Kind)
	tracker := s.indexer.GetRefTracker()
	maxResults := req.Max
	if maxResults <= 0 {
		maxResults = 100
	}

	symbols := s.indexer.GetFileEnhancedSymbols(targetFileID)
	var entries []ListSymbolsEntry
	for _, sym := range symbols {
		if kinds != nil && !kinds[sym.Symbol.Type] {
			continue
		}
		if req.Exported != nil {
			if *req.Exported && !sym.IsExported {
				continue
			}
			if !*req.Exported && sym.IsExported {
				continue
			}
		}
		entries = append(entries, buildHTTPSymbolEntry(sym, targetFilePath, tracker))
	}

	total := len(entries)
	if len(entries) > maxResults {
		entries = entries[:maxResults]
	}

	// Detect language
	lang := httpLanguageFromPath(targetFilePath)

	resp := BrowseFileHTTPResponse{
		File: BrowseFileInfoEntry{
			Path:     targetFilePath,
			FileID:   int(targetFileID),
			Language: lang,
		},
		Symbols: entries,
		Total:   total,
	}

	if req.ShowImports {
		fileInfo := s.indexer.GetFile(targetFileID)
		if fileInfo != nil {
			imports := make([]string, len(fileInfo.Imports))
			for i, imp := range fileInfo.Imports {
				imports[i] = imp.Path
			}
			resp.Imports = imports
		}
	}

	if req.ShowStats {
		stats := computeHTTPFileStats(symbols)
		resp.Stats = stats
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ========== HTTP helpers ==========

func parseHTTPSymbolKinds(kindStr string) map[types.SymbolType]bool {
	if kindStr == "" || kindStr == "all" {
		return nil
	}
	kinds := make(map[types.SymbolType]bool)
	for _, k := range strings.Split(kindStr, ",") {
		k = strings.TrimSpace(strings.ToLower(k))
		switch k {
		case "func", "fn", "function":
			kinds[types.SymbolTypeFunction] = true
		case "type":
			kinds[types.SymbolTypeType] = true
		case "struct":
			kinds[types.SymbolTypeStruct] = true
		case "interface", "iface":
			kinds[types.SymbolTypeInterface] = true
		case "method":
			kinds[types.SymbolTypeMethod] = true
		case "class", "cls":
			kinds[types.SymbolTypeClass] = true
		case "enum":
			kinds[types.SymbolTypeEnum] = true
		case "variable", "var":
			kinds[types.SymbolTypeVariable] = true
		case "constant", "const":
			kinds[types.SymbolTypeConstant] = true
		case "field":
			kinds[types.SymbolTypeField] = true
		}
	}
	if len(kinds) == 0 {
		return nil
	}
	return kinds
}

func matchesHTTPListFilters(sym *types.EnhancedSymbol, kinds map[types.SymbolType]bool, req ListSymbolsRequest) bool {
	if kinds != nil && !kinds[sym.Symbol.Type] {
		return false
	}
	if req.Exported != nil {
		if *req.Exported && !sym.IsExported {
			return false
		}
		if !*req.Exported && sym.IsExported {
			return false
		}
	}
	if req.Name != "" && !strings.Contains(strings.ToLower(sym.Symbol.Name), strings.ToLower(req.Name)) {
		return false
	}
	if req.Receiver != "" && !strings.EqualFold(sym.ReceiverType, req.Receiver) {
		return false
	}
	if req.MinComplexity != nil && sym.Complexity < *req.MinComplexity {
		return false
	}
	if req.MaxComplexity != nil && sym.Complexity > *req.MaxComplexity {
		return false
	}
	if req.MinParams != nil && int(sym.ParameterCount) < *req.MinParams {
		return false
	}
	if req.MaxParams != nil && int(sym.ParameterCount) > *req.MaxParams {
		return false
	}
	return true
}

func buildHTTPSymbolEntry(sym *types.EnhancedSymbol, filePath string, tracker *core.ReferenceTracker) ListSymbolsEntry {
	entry := ListSymbolsEntry{
		Name:           sym.Symbol.Name,
		Type:           sym.Symbol.Type.String(),
		File:           filePath,
		Line:           sym.Symbol.Line,
		ObjectID:       searchtypes.EncodeSymbolID(sym.ID),
		IsExported:     sym.IsExported,
		Signature:      sym.Signature,
		Complexity:     sym.Complexity,
		ParameterCount: int(sym.ParameterCount),
		ReceiverType:   sym.ReceiverType,
		IncomingRefs:   len(sym.IncomingRefs),
		OutgoingRefs:   len(sym.OutgoingRefs),
	}
	return entry
}

func buildHTTPInspectEntry(sym *types.EnhancedSymbol, filePath string, tracker *core.ReferenceTracker, indexer *indexing.MasterIndex) InspectSymbolEntry {
	entry := InspectSymbolEntry{
		Name:           sym.Symbol.Name,
		ObjectID:       searchtypes.EncodeSymbolID(sym.ID),
		Type:           sym.Symbol.Type.String(),
		File:           filePath,
		Line:           sym.Symbol.Line,
		IsExported:     sym.IsExported,
		Signature:      sym.Signature,
		DocComment:     sym.DocComment,
		Complexity:     sym.Complexity,
		ParameterCount: int(sym.ParameterCount),
		ReceiverType:   sym.ReceiverType,
		IncomingRefs:   len(sym.IncomingRefs),
		OutgoingRefs:   len(sym.OutgoingRefs),
		Annotations:    sym.Annotations,
	}

	if tracker != nil {
		entry.Callers = tracker.GetCallerNames(sym.ID)
		entry.Callees = tracker.GetCalleeNames(sym.ID)

		rels := tracker.GetTypeRelationships(sym.ID)
		if rels != nil && rels.HasTypeRelationships() {
			th := &TypeHierarchyEntry{}
			for _, id := range rels.Implements {
				if s := tracker.GetEnhancedSymbol(id); s != nil {
					th.Implements = append(th.Implements, s.Symbol.Name)
				}
			}
			for _, id := range rels.ImplementedBy {
				if s := tracker.GetEnhancedSymbol(id); s != nil {
					th.ImplementedBy = append(th.ImplementedBy, s.Symbol.Name)
				}
			}
			for _, id := range rels.Extends {
				if s := tracker.GetEnhancedSymbol(id); s != nil {
					th.Extends = append(th.Extends, s.Symbol.Name)
				}
			}
			for _, id := range rels.ExtendedBy {
				if s := tracker.GetEnhancedSymbol(id); s != nil {
					th.ExtendedBy = append(th.ExtendedBy, s.Symbol.Name)
				}
			}
			entry.TypeHierarchy = th
		}
	}

	if len(sym.ScopeChain) > 0 {
		chain := make([]string, len(sym.ScopeChain))
		for i, sc := range sym.ScopeChain {
			chain[i] = sc.Name
		}
		entry.ScopeChain = chain
	}

	// Decode flags
	if sym.FunctionFlags != 0 {
		var flags []string
		if sym.FunctionFlags&types.FunctionFlagAsync != 0 {
			flags = append(flags, "async")
		}
		if sym.FunctionFlags&types.FunctionFlagGenerator != 0 {
			flags = append(flags, "generator")
		}
		if sym.FunctionFlags&types.FunctionFlagMethod != 0 {
			flags = append(flags, "method")
		}
		if sym.FunctionFlags&types.FunctionFlagVariadic != 0 {
			flags = append(flags, "variadic")
		}
		entry.FunctionFlags = flags
	}
	if sym.VariableFlags != 0 {
		var flags []string
		if sym.VariableFlags&types.VariableFlagConst != 0 {
			flags = append(flags, "const")
		}
		if sym.VariableFlags&types.VariableFlagStatic != 0 {
			flags = append(flags, "static")
		}
		if sym.VariableFlags&types.VariableFlagPointer != 0 {
			flags = append(flags, "pointer")
		}
		entry.VariableFlags = flags
	}

	return entry
}

func computeHTTPFileStats(symbols []*types.EnhancedSymbol) *FileStatsEntry {
	stats := &FileStatsEntry{
		SymbolCount: len(symbols),
	}
	totalComplexity := 0
	complexityCount := 0
	for _, sym := range symbols {
		if sym.IsExported {
			stats.ExportedCount++
		}
		switch sym.Symbol.Type {
		case types.SymbolTypeFunction, types.SymbolTypeMethod:
			stats.FunctionCount++
			if sym.Complexity > 0 {
				totalComplexity += sym.Complexity
				complexityCount++
				if sym.Complexity > stats.MaxComplexity {
					stats.MaxComplexity = sym.Complexity
				}
			}
		case types.SymbolTypeType, types.SymbolTypeStruct, types.SymbolTypeInterface,
			types.SymbolTypeClass, types.SymbolTypeEnum:
			stats.TypeCount++
		}
	}
	if complexityCount > 0 {
		stats.AvgComplexity = float64(totalComplexity) / float64(complexityCount)
	}
	return stats
}

func httpLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	default:
		return ""
	}
}
