package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// Client connects to a remote IndexServer
type Client struct {
	httpClient *http.Client
	socketPath string
}

// NewClient creates a new client connection to the index server using the default socket path
func NewClient() *Client {
	return NewClientWithSocket(GetSocketPath())
}

// NewClientWithSocket creates a new client connection to the index server with a custom socket path
func NewClientWithSocket(socketPath string) *Client {
	// Create HTTP client that uses Unix socket
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	return &Client{
		httpClient: httpClient,
		socketPath: socketPath,
	}
}

// IsServerRunning checks if the server is accessible
func (c *Client) IsServerRunning() bool {
	_, err := c.Ping()
	return err == nil
}

// Ping sends a health check to the server
func (c *Client) Ping() (*PingResponse, error) {
	resp, err := c.httpClient.Post("http://unix/ping", "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var pingResp PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &pingResp, nil
}

// GetStatus retrieves the current index status
func (c *Client) GetStatus() (*IndexStatus, error) {
	resp, err := c.httpClient.Get("http://unix/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var status IndexStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	return &status, nil
}

// Search performs a search query on the remote index
func (c *Client) Search(pattern string, options types.SearchOptions, maxResults int) ([]searchtypes.Result, error) {
	req := SearchRequest{
		Pattern:    pattern,
		Options:    options,
		MaxResults: maxResults,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if searchResp.Error != "" {
		return nil, fmt.Errorf("search error: %s", searchResp.Error)
	}

	return searchResp.Results, nil
}

// GetSymbol retrieves symbol information
func (c *Client) GetSymbol(symbolID types.SymbolID) (*types.EnhancedSymbol, error) {
	req := GetSymbolRequest{
		SymbolID: symbolID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/symbol", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var symbolResp GetSymbolResponse
	if err := json.NewDecoder(resp.Body).Decode(&symbolResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if symbolResp.Error != "" {
		return nil, fmt.Errorf("symbol error: %s", symbolResp.Error)
	}

	return symbolResp.Symbol, nil
}

// GetFileInfo retrieves file information
func (c *Client) GetFileInfo(fileID types.FileID) (*types.FileInfo, error) {
	req := GetFileInfoRequest{
		FileID: fileID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/fileinfo", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var fileInfoResp GetFileInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileInfoResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if fileInfoResp.Error != "" {
		return nil, fmt.Errorf("file info error: %s", fileInfoResp.Error)
	}

	return fileInfoResp.FileInfo, nil
}

// Shutdown requests the server to shut down
func (c *Client) Shutdown(force bool) error {
	req := ShutdownRequest{
		Force: force,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/shutdown", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to shutdown: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	var shutdownResp ShutdownResponse
	if err := json.NewDecoder(resp.Body).Decode(&shutdownResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !shutdownResp.Success {
		return fmt.Errorf("shutdown failed: %s", shutdownResp.Message)
	}

	return nil
}

// Reindex triggers a re-index of the project
func (c *Client) Reindex(path string) error {
	req := ReindexRequest{
		Path: path,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/reindex", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to reindex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	var reindexResp ReindexResponse
	if err := json.NewDecoder(resp.Body).Decode(&reindexResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !reindexResp.Success {
		return fmt.Errorf("reindex failed: %s", reindexResp.Message)
	}

	return nil
}

// GetDefinition searches for symbol definitions by name pattern
func (c *Client) GetDefinition(pattern string, maxResults int) ([]DefinitionLocation, error) {
	req := DefinitionRequest{
		Pattern:    pattern,
		MaxResults: maxResults,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/definition", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to get definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var defResp DefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&defResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if defResp.Error != "" {
		return nil, fmt.Errorf("definition error: %s", defResp.Error)
	}

	return defResp.Definitions, nil
}

// GetReferences searches for symbol references (usages) by name pattern
func (c *Client) GetReferences(pattern string, maxResults int) ([]ReferenceLocation, error) {
	req := ReferencesRequest{
		Pattern:    pattern,
		MaxResults: maxResults,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/references", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to get references: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var refResp ReferencesResponse
	if err := json.NewDecoder(resp.Body).Decode(&refResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if refResp.Error != "" {
		return nil, fmt.Errorf("references error: %s", refResp.Error)
	}

	return refResp.References, nil
}

// GetTree generates a function call hierarchy tree
func (c *Client) GetTree(functionName string, maxDepth int, showLines, compact, agentMode bool, exclude string) (*types.FunctionTree, error) {
	req := TreeRequest{
		FunctionName: functionName,
		MaxDepth:     maxDepth,
		ShowLines:    showLines,
		Compact:      compact,
		AgentMode:    agentMode,
		Exclude:      exclude,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/tree", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var treeResp TreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&treeResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if treeResp.Error != "" {
		return nil, fmt.Errorf("tree error: %s", treeResp.Error)
	}

	return treeResp.Tree, nil
}

// GetStats retrieves index statistics from the server
func (c *Client) GetStats() (*StatsResponse, error) {
	resp, err := c.httpClient.Post("http://unix/stats", "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var statsResp StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if statsResp.Error != "" {
		return nil, fmt.Errorf("stats error: %s", statsResp.Error)
	}

	return &statsResp, nil
}

// WaitForReady waits until the index is ready or timeout
func (c *Client) WaitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for index to be ready")
		case <-ticker.C:
			status, err := c.GetStatus()
			if err != nil {
				continue
			}
			if status.Ready {
				return nil
			}
		}
	}
}

// GitAnalyze performs git change analysis
func (c *Client) GitAnalyze(req GitAnalyzeRequest) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/git-analyze", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze git changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var gitResp GitAnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&gitResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if gitResp.Error != "" {
		return nil, fmt.Errorf("git analyze error: %s", gitResp.Error)
	}

	return gitResp.Report, nil
}

// ListSymbols enumerates and filters symbols in the index
func (c *Client) ListSymbols(req ListSymbolsRequest) (*ListSymbolsHTTPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/list-symbols", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to list symbols: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var result ListSymbolsHTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("list symbols error: %s", result.Error)
	}

	return &result, nil
}

// InspectSymbol provides deep inspection of a symbol
func (c *Client) InspectSymbol(req InspectSymbolRequest) (*InspectSymbolHTTPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/inspect-symbol", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to inspect symbol: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var result InspectSymbolHTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("inspect symbol error: %s", result.Error)
	}

	return &result, nil
}

// BrowseFile lists all symbols in a specific file
func (c *Client) BrowseFile(req BrowseFileRequest) (*BrowseFileHTTPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/browse-file", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to browse file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var result BrowseFileHTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("browse file error: %s", result.Error)
	}

	return &result, nil
}
