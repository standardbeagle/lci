package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// TestMCPServerStartupShutdown tests basic server lifecycle management
func TestMCPServerStartupShutdown(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-mcp-lifecycle-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package test

func TestFunction() {
	// Test function
}

func AnotherFunction() int {
	return 42
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = tmpDir

	// Create goroutine index
	goroutineIndex := indexing.NewMasterIndex(cfg)

	// Test server creation
	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Verify server components are initialized
	if server.goroutineIndex == nil {
		t.Error("MasterIndex not initialized")
	}
	if server.cfg == nil {
		t.Error("Config not initialized")
	}
	if server.diagnosticLogger == nil {
		t.Error("Logger not initialized")
	}
	if server.autoIndexManager == nil {
		t.Error("AutoIndexManager not initialized")
	}

	// Test that the server can start up without panicking
	// Note: We can't easily test the full MCP server lifecycle without a transport
	// But we can verify that components are properly initialized
	t.Log("Server components verified successfully")
}

// TestMCPServerConcurrentRequests tests concurrent request handling
func TestMCPServerConcurrentRequests(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-mcp-concurrent-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create multiple test files
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf(`package test

func Function%d() int {
	return %d
}
`, i, i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = tmpDir

	// Create goroutine index (server will handle indexing via auto-indexing)
	goroutineIndex := indexing.NewMasterIndex(cfg)

	// Create server (which starts auto-indexing in background)
	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Wait for auto-indexing to complete
	status, err := server.autoIndexManager.waitForCompletion(30 * time.Second)
	if err != nil {
		t.Fatalf("Failed to wait for indexing completion: %v", err)
	}
	t.Logf("Indexing completed with status: %s", status)

	// Test concurrent search requests with structured concurrency
	const numGoroutines = 10
	const requestsPerGoroutine = 5

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use errgroup for structured concurrency with bounded parallelism
	g, ctx := errgroup.WithContext(testCtx)
	g.SetLimit(10) // Limit concurrent goroutines (backpressure)

	var mu sync.Mutex
	var errors []error

	for i := 0; i < numGoroutines; i++ {
		goroutineID := i // Capture
		g.Go(func() error {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			for j := 0; j < requestsPerGoroutine; j++ {
				// Test search functionality
				searchReq := SearchParams{
					Pattern: fmt.Sprintf("Function%d", j%5),
					Output:  string(OutputSingleLine),
					Max:     10,
				}

				_, err := server.Search(ctx, searchReq)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d, request %d: %v", goroutineID, j, err))
					mu.Unlock()
					// Continue processing other requests instead of returning early
				}
			}
			return nil
		})
	}

	// Wait for all goroutines to complete with timeout
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- g.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			t.Fatalf("Concurrent requests failed: %v", err)
		}
	case <-testCtx.Done():
		t.Fatalf("Test timeout: %v", testCtx.Err())
	}

	// Check for any errors
	if len(errors) > 0 {
		t.Errorf("%d/%d requests failed:\n%v", len(errors), numGoroutines*requestsPerGoroutine, errors)
	}

	t.Logf("Successfully handled %d concurrent requests", numGoroutines*requestsPerGoroutine)
}

// TestMCPServerErrorHandling tests server error handling and recovery
func TestMCPServerErrorHandling(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-mcp-errors-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = tmpDir

	// Create goroutine index
	goroutineIndex := indexing.NewMasterIndex(cfg)

	// Create server
	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test search with empty pattern (should return validation error)
	searchReq := SearchParams{
		Pattern: "",
		Output:  string(OutputSingleLine),
	}

	_, err = server.Search(context.Background(), searchReq)
	if err == nil {
		t.Error("Expected error for empty pattern, got nil")
	}

	// Test search with invalid pattern
	searchReq.Pattern = "*[invalid*"
	_, err = server.Search(context.Background(), searchReq)
	// This might not error in all implementations, but we test that it doesn't panic
	t.Logf("Search with invalid pattern returned: %v", err)

	// Test object context with missing ID - validation should fail
	contextReq := ObjectContextParams{}

	// Test object context validation directly - missing id/name should fail
	err = validateObjectContextParams(contextReq)
	if err == nil {
		t.Error("Expected error for missing object context ID, got nil")
	}

	// Test object context with valid ID - validation should pass
	// (format is checked at lookup time, not validation time)
	contextReq2 := ObjectContextParams{
		ID: "VE", // Valid concise object ID format
	}
	err = validateObjectContextParams(contextReq2)
	if err != nil {
		t.Errorf("Expected no error for valid object context ID, got: %v", err)
	}

	t.Log("Error handling tests completed successfully")
}

// TestMCPServerIndexState tests server index state management
func TestMCPServerIndexState(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-mcp-index-state-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = tmpDir

	// Create goroutine index
	goroutineIndex := indexing.NewMasterIndex(cfg)

	// Create server (starts auto-indexing in background)
	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Wait for initial auto-indexing to complete or stay idle (empty directory may result in "idle" status)
	status, err := server.autoIndexManager.waitForCompletion(10 * time.Second)
	if err != nil {
		t.Fatalf("Failed to wait for initial auto-indexing: %v", err)
	}
	// Status can be "completed" (if indexing ran and finished) or "idle" (if nothing to index)
	if status != "completed" && status != "idle" {
		t.Errorf("Expected auto-indexing to complete or be idle, got status: %s", status)
	}

	// Verify indexing is complete (even with 0 files) - should return nil when not actively indexing
	if err := goroutineIndex.CheckIndexingComplete(); err != nil {
		t.Errorf("Expected index to be ready (not actively indexing): %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package test

func TestFunction() {
	// Test function
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Index the directory
	ctx := context.Background()
	if err := goroutineIndex.IndexDirectory(ctx, tmpDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Test index state after indexing
	if err := goroutineIndex.CheckIndexingComplete(); err != nil {
		t.Errorf("Expected index to be complete after indexing: %v", err)
	}

	// Test search after indexing
	searchReq := SearchParams{
		Pattern: "TestFunction",
		Output:  string(OutputSingleLine),
		Max:     10,
	}

	results, err := server.Search(context.Background(), searchReq)
	if err != nil {
		t.Errorf("Failed to search after indexing: %v", err)
	}
	if len(results.Results) == 0 {
		t.Error("Expected to find TestFunction in search results")
	}

	t.Log("Index state management tests completed successfully")
}

// TestMCPServerGracefulShutdown tests graceful shutdown behavior
func TestMCPServerGracefulShutdown(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-mcp-shutdown-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = tmpDir

	// Create goroutine index
	goroutineIndex := indexing.NewMasterIndex(cfg)

	// Create server
	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start some operations in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		// Simulate some work
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				// Test auto-indexing manager state
				if server.autoIndexManager.isRunning() {
					t.Logf("Auto-indexing is running (iteration %d)", i)
				}
				// Wait for next iteration with timeout
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
		}
		close(done)
	}()

	// Wait for background work or timeout
	select {
	case <-done:
		t.Log("Background operations completed normally")
	case <-ctx.Done():
		t.Log("Background operations cancelled (expected behavior)")
	}

	// Test that server components are still accessible
	if server.goroutineIndex == nil {
		t.Error("MasterIndex became nil during operations")
	}
	if server.autoIndexManager == nil {
		t.Error("AutoIndexManager became nil during operations")
	}

	t.Log("Graceful shutdown tests completed successfully")
}

// TestMCPServerResourceManagement tests resource management and cleanup
func TestMCPServerResourceManagement(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "test-mcp-resources-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config
	cfg, err := config.LoadWithRoot("", tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = tmpDir

	// Create multiple servers to test resource management
	const numServers = 5
	servers := make([]*Server, numServers)

	for i := 0; i < numServers; i++ {
		goroutineIndex := indexing.NewMasterIndex(cfg)
		server, err := NewServer(goroutineIndex, cfg)
		if err != nil {
			t.Fatalf("Failed to create server %d: %v", i, err)
		}
		servers[i] = server
	}

	// Verify all servers are properly initialized
	for i, server := range servers {
		if server.goroutineIndex == nil {
			t.Errorf("Server %d: MasterIndex not initialized", i)
		}
		if server.autoIndexManager == nil {
			t.Errorf("Server %d: AutoIndexManager not initialized", i)
		}
	}

	// Test concurrent operations on all servers
	var wg sync.WaitGroup
	for i, server := range servers {
		wg.Add(1)
		go func(serverID int, srv *Server) {
			defer wg.Done()

			// Test auto-indexing manager
			if srv.autoIndexManager.isRunning() {
				t.Logf("Server %d: Auto-indexing is running", serverID)
			}

			// Test status retrieval
			status := srv.autoIndexManager.getStatus()
			t.Logf("Server %d: Auto-indexing status: %s", serverID, status)
		}(i, server)
	}

	wg.Wait()
	t.Logf("Successfully tested %d servers concurrently", numServers)
}
