package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// getTestSocketPath returns a unique socket path for the given test
func getTestSocketPath(t *testing.T) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("lci-test-%s.sock", t.Name()))
}

// TestServerIntegration_BasicLifecycle tests server start, query, and shutdown
func TestServerIntegration_BasicLifecycle(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create initial test file
	initialContent := `package test

func TestFunction() string {
	return "test"
}
`
	err := os.WriteFile(filepath.Join(testDir, "test.go"), []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create config with explicit file inclusion
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024, // 10MB
		},
	}

	// Create and start server with custom socket path
	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)

	err = srv.Start()
	require.NoError(t, err)

	// Give indexing time to complete
	time.Sleep(2 * time.Second)

	// Create client with custom socket path and connect
	client := NewClientWithSocket(socketPath)
	require.True(t, client.IsServerRunning(), "Server should be running")

	// Wait for index to be ready
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Check status
	status, err := client.GetStatus()
	require.NoError(t, err)
	t.Logf("Index status: Ready=%v, FileCount=%d, SymbolCount=%d", status.Ready, status.FileCount, status.SymbolCount)

	// Perform search
	searchOpts := types.SearchOptions{
		MaxResults: 100,
	}
	results, err := client.Search("TestFunction", searchOpts, 100)
	require.NoError(t, err)
	t.Logf("Search for 'TestFunction' returned %d results", len(results))
	for i, r := range results {
		t.Logf("  Result %d: %s:%d - %s", i, r.Path, r.Line, r.Match)
	}
	assert.GreaterOrEqual(t, len(results), 1, "Should find TestFunction (found %d results)", len(results))

	// Verify result contains our file
	foundTestFile := false
	for _, r := range results {
		if strings.Contains(r.Path, "test.go") {
			foundTestFile = true
			break
		}
	}
	assert.True(t, foundTestFile, "Should find TestFunction in test.go")

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	require.NoError(t, err)

	// Verify server stopped
	assert.False(t, client.IsServerRunning(), "Server should be stopped")
}

// TestServerIntegration_ManualFileUpdate tests manual file updates via IndexFile
func TestServerIntegration_ManualFileUpdate(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create initial test file
	initialContent := `package test

func OriginalFunction() string {
	return "original"
}
`
	testFile := filepath.Join(testDir, "update.go")
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create config and server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)

	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	// Wait for initial index
	time.Sleep(2 * time.Second)

	client := NewClientWithSocket(socketPath)
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Verify original function found
	results, err := client.Search("OriginalFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find OriginalFunction")

	// Update file content
	updatedContent := `package test

func UpdatedFunction() string {
	return "updated"
}
`
	err = os.WriteFile(testFile, []byte(updatedContent), 0644)
	require.NoError(t, err)

	// Manually trigger re-index via server's indexer
	err = srv.indexer.IndexFile(testFile)
	require.NoError(t, err)

	// Small delay for index to propagate
	time.Sleep(100 * time.Millisecond)

	// Search for new function
	results, err = client.Search("UpdatedFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find UpdatedFunction after update")

	// Verify old function NOT found in updated file
	results, err = client.Search("OriginalFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotContains(t, r.Path, "update.go", "Should NOT find OriginalFunction in updated file")
	}
}

// TestServerIntegration_MultipleClients tests multiple clients sharing the same index
func TestServerIntegration_MultipleClients(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create test files
	file1Content := `package test
func File1Function() {}
`
	file2Content := `package test
func File2Function() {}
`
	err := os.WriteFile(filepath.Join(testDir, "file1.go"), []byte(file1Content), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testDir, "file2.go"), []byte(file2Content), 0644)
	require.NoError(t, err)

	// Start server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	time.Sleep(2 * time.Second)

	// Create multiple clients
	client1 := NewClientWithSocket(socketPath)
	client2 := NewClientWithSocket(socketPath)
	client3 := NewClientWithSocket(socketPath)

	// All clients should connect
	assert.True(t, client1.IsServerRunning())
	assert.True(t, client2.IsServerRunning())
	assert.True(t, client3.IsServerRunning())

	// All clients should be able to search
	var wg sync.WaitGroup
	errors := make(chan error, 3)

	searchWithClient := func(client *Client, pattern string) {
		defer wg.Done()
		results, err := client.Search(pattern, types.SearchOptions{}, 100)
		if err != nil {
			errors <- err
			return
		}
		if len(results) < 1 {
			errors <- assert.AnError
			return
		}
	}

	wg.Add(3)
	go searchWithClient(client1, "File1Function")
	go searchWithClient(client2, "File2Function")
	go searchWithClient(client3, "File1Function")

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Client search failed: %v", err)
	}
}

// TestServerIntegration_ConcurrentSearches tests concurrent searches from multiple clients
func TestServerIntegration_ConcurrentSearches(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create test file with multiple functions
	content := `package test

func Function1() {}
func Function2() {}
func Function3() {}
func Function4() {}
func Function5() {}
`
	err := os.WriteFile(filepath.Join(testDir, "concurrent.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Start server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	time.Sleep(2 * time.Second)

	client := NewClientWithSocket(socketPath)
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Perform 50 concurrent searches
	var wg sync.WaitGroup
	searchCount := 50
	var successCount int32 // Use atomic operations instead of mutex

	for i := 0; i < searchCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// Use efficient string formatting instead of rune conversion
			funcNum := (index % 5) + 1
			pattern := "Function" + strconv.Itoa(funcNum)
			results, err := client.Search(pattern, types.SearchOptions{}, 100)
			if err == nil && len(results) >= 1 {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// All searches should succeed
	assert.Equal(t, int32(searchCount), successCount, "All concurrent searches should succeed")
}

// TestServerIntegration_ExternalIndexWithMCP tests using an externally managed index (MCP scenario)
func TestServerIntegration_ExternalIndexWithMCP(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create test file
	content := `package test
func ExternalFunction() string {
	return "external"
}
`
	err := os.WriteFile(filepath.Join(testDir, "external.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Create external index (simulating MCP's index)
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	externalIndexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	// Index the directory externally
	err = externalIndexer.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Create search engine
	searchEngine := search.NewEngine(externalIndexer)
	externalIndexer.SetSearchEngine(searchEngine)

	// Create server with external index
	srv, err := NewIndexServerWithIndex(cfg, externalIndexer, searchEngine)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)

	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	// Index should be immediately ready (no background indexing)
	client := NewClientWithSocket(socketPath)
	assert.True(t, client.IsServerRunning())

	// Search should work immediately
	results, err := client.Search("ExternalFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find ExternalFunction from external index")
}

// TestServerIntegration_FileAddition tests adding a new file to an existing index
func TestServerIntegration_FileAddition(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create initial file
	initialContent := `package test
func InitialFunction() {}
`
	err := os.WriteFile(filepath.Join(testDir, "initial.go"), []byte(initialContent), 0644)
	require.NoError(t, err)

	// Start server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	time.Sleep(2 * time.Second)

	client := NewClientWithSocket(socketPath)
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Verify initial function found
	results, err := client.Search("InitialFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Add new file
	newContent := `package test
func NewlyAddedFunction() {}
`
	newFile := filepath.Join(testDir, "newfile.go")
	err = os.WriteFile(newFile, []byte(newContent), 0644)
	require.NoError(t, err)

	// Trigger reindex to pick up the new file
	err = client.Reindex("")
	require.NoError(t, err)

	// Wait for reindex to complete
	time.Sleep(2 * time.Second)

	// Search for new function
	results, err = client.Search("NewlyAddedFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find newly added function")

	// Original function should still be searchable
	results, err = client.Search("InitialFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Original function should still be found")
}

// TestServerIntegration_FileDeletion tests removing a file from the index
func TestServerIntegration_FileDeletion(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create two files
	keep := `package test
func KeepFunction() {}
`
	delete := `package test
func DeleteFunction() {}
`
	keepFile := filepath.Join(testDir, "keep.go")
	deleteFile := filepath.Join(testDir, "delete.go")

	err := os.WriteFile(keepFile, []byte(keep), 0644)
	require.NoError(t, err)
	err = os.WriteFile(deleteFile, []byte(delete), 0644)
	require.NoError(t, err)

	// Start server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	time.Sleep(2 * time.Second)

	client := NewClientWithSocket(socketPath)
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Verify both functions found
	results, err := client.Search("KeepFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	results, err = client.Search("DeleteFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Delete file and remove from index
	err = os.Remove(deleteFile)
	require.NoError(t, err)

	err = srv.indexer.RemoveFile(deleteFile)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Deleted function should NOT be found
	results, err = client.Search("DeleteFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotContains(t, r.Path, "delete.go", "Should NOT find deleted function")
	}

	// Kept function should still be found
	results, err = client.Search("KeepFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Kept function should still be found")
}

// TestServerIntegration_ReindexCommand tests the reindex endpoint
func TestServerIntegration_ReindexCommand(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create initial file
	content := `package test
func OriginalFunction() {}
`
	testFile := filepath.Join(testDir, "reindex.go")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Start server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	time.Sleep(2 * time.Second)

	client := NewClientWithSocket(socketPath)
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Verify original function found
	results, err := client.Search("OriginalFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Update file
	newContent := `package test
func ReindexedFunction() {}
`
	err = os.WriteFile(testFile, []byte(newContent), 0644)
	require.NoError(t, err)

	// Trigger reindex via client
	err = client.Reindex("")
	require.NoError(t, err)

	// Wait for reindex to complete
	time.Sleep(3 * time.Second)
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// New function should be found
	results, err = client.Search("ReindexedFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find new function after reindex")

	// Old function should NOT be found
	results, err = client.Search("OriginalFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotContains(t, r.Path, "reindex.go", "Should NOT find old function after reindex")
	}
}

// TestServerIntegration_StatusEndpoint tests the status endpoint
func TestServerIntegration_StatusEndpoint(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	// Create test file
	content := `package test
func StatusTestFunction() {}
`
	err := os.WriteFile(filepath.Join(testDir, "status.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Start server
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	client := NewClientWithSocket(socketPath)

	// Initial status might show indexing active
	status, err := client.GetStatus()
	require.NoError(t, err)
	assert.NotNil(t, status)

	// Wait for indexing to complete
	err = client.WaitForReady(10 * time.Second)
	require.NoError(t, err)

	// Status should show ready
	status, err = client.GetStatus()
	require.NoError(t, err)
	assert.True(t, status.Ready, "Index should be ready")
	assert.False(t, status.IndexingActive, "Indexing should not be active")
}

// TestServerIntegration_PingEndpoint tests server health check
func TestServerIntegration_PingEndpoint(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			MaxFileSize: 10 * 1024 * 1024,
		},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	client := NewClientWithSocket(socketPath)

	// Ping should succeed
	ping, err := client.Ping()
	require.NoError(t, err)
	assert.NotNil(t, ping)
	assert.GreaterOrEqual(t, ping.Uptime, 0.0, "Uptime should be non-negative")
	assert.NotEmpty(t, ping.Version, "Version should be set")
}
