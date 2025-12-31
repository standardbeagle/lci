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

// GetSocketPath returns the path to the Unix socket for this server
func GetSocketPath() string {
	// Use temp directory for socket
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "lci-server.sock")
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
	socketPath := GetSocketPath()
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
	os.Remove(GetSocketPath())

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
