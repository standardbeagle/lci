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
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
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

	// TODO: Add GetSymbol method to MasterIndex
	// For now, return not implemented
	response := GetSymbolResponse{
		Symbol: nil,
		Error:  "GetSymbol not yet implemented",
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

	// TODO: Add GetFileInfo method to MasterIndex
	// For now, return not implemented
	response := GetFileInfoResponse{
		FileInfo: nil,
		Error:    "GetFileInfo not yet implemented",
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
