package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// TestWatcherIntegration_AutomaticFileUpdate tests automatic file change detection and indexing
// This is a critical test for the shared server architecture with MCP
func TestWatcherIntegration_AutomaticFileUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file watcher integration test in short mode")
	}

	testDir := t.TempDir()

	// Create initial file
	initialContent := `package test

func OriginalWatchedFunction() string {
	return "original"
}
`
	testFile := filepath.Join(testDir, "watched.go")
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create config with file watching enabled
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			WatchMode:       true,
			WatchDebounceMs: 100, // Short debounce for testing
			MaxFileSize:     10 * 1024 * 1024,
		},
	}

	// Create server - it will handle indexing and start watching
	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)

	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	// Wait for initial indexing to complete
	time.Sleep(1 * time.Second)

	// Create client
	client := NewClient()
	require.True(t, client.IsServerRunning())

	// Verify original function found
	status, err := client.GetStatus()
	require.NoError(t, err)
	t.Logf("Server status before search: Ready=%v, FileCount=%d, SymbolCount=%d", status.Ready, status.FileCount, status.SymbolCount)

	results, err := client.Search("OriginalWatchedFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	t.Logf("Search for 'OriginalWatchedFunction' returned %d results", len(results))
	require.GreaterOrEqual(t, len(results), 1, "Should find OriginalWatchedFunction initially")

	// Wait for any initial watcher events to be processed
	// (watcher may have queued events from initial file creation)
	time.Sleep(3 * time.Second)

	// Modify file on disk (simulate user editing)
	t.Logf("Modifying file %s at %v", testFile, time.Now())
	updatedContent := `package test

func UpdatedWatchedFunction() string {
	return "updated by watcher"
}
`
	err = os.WriteFile(testFile, []byte(updatedContent), 0644)
	require.NoError(t, err)
	t.Logf("File modified successfully")

	// Wait for file watcher to detect change and rebuild index
	// This tests the full automatic update pipeline:
	// file change → fsnotify → debounced rebuild → index update
	// Need to wait longer for lock acquisition and indexing to complete
	time.Sleep(5 * time.Second)

	// Check status after update
	status, err = client.GetStatus()
	require.NoError(t, err)
	t.Logf("Server status after update: Ready=%v, FileCount=%d, SymbolCount=%d", status.Ready, status.FileCount, status.SymbolCount)

	// Debug: Search for old function to see if it's still there
	oldResults, err := client.Search("OriginalWatchedFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	t.Logf("Search for 'OriginalWatchedFunction' after update returned %d results", len(oldResults))

	// Search for NEW function (should be found via automatic update)
	var updatedResults []searchtypes.Result
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		updatedResults, err = client.Search("UpdatedWatchedFunction", types.SearchOptions{}, 100)
		require.NoError(t, err)
		if len(updatedResults) >= 1 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.GreaterOrEqual(t, len(updatedResults), 1, "Should find UpdatedWatchedFunction after automatic update")

	// Verify new function is in the correct file
	foundInWatchedFile := false
	for _, r := range updatedResults {
		if strings.Contains(r.Path, "watched.go") {
			foundInWatchedFile = true
			break
		}
	}
	assert.True(t, foundInWatchedFile, "UpdatedWatchedFunction should be found in watched.go")

	// Search for OLD function (should NOT be found - critical test for stale data removal)
	originalResults, err := client.Search("OriginalWatchedFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	for _, r := range originalResults {
		assert.NotContains(t, r.Path, "watched.go", "Should NOT find OriginalWatchedFunction in watched.go after automatic update")
	}
}

// TestWatcherIntegration_NewFileDetection tests automatic detection of new files
func TestWatcherIntegration_NewFileDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file watcher integration test in short mode")
	}

	testDir := t.TempDir()

	// Create initial file
	initialContent := `package test
func InitialFunction() {}
`
	err := os.WriteFile(filepath.Join(testDir, "initial.go"), []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create config with watching
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			WatchMode:       true,
			WatchDebounceMs: 100,
			MaxFileSize:     10 * 1024 * 1024,
		},
	}

	// Create and start watched index
	externalIndexer := indexing.NewMasterIndex(cfg)
	err = externalIndexer.IndexDirectory(context.Background(), testDir)
	require.NoError(t, err)

	searchEngine := search.NewEngine(externalIndexer)
	externalIndexer.SetSearchEngine(searchEngine)

	require.NoError(t, err)

	// Start server
	srv, err := NewIndexServerWithIndex(cfg, externalIndexer, searchEngine)
	require.NoError(t, err)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	client := NewClient()

	// Verify initial function found
	results, err := client.Search("InitialFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Create NEW file (watcher should detect)
	newContent := `package test
func NewlyCreatedFunction() {}
`
	newFile := filepath.Join(testDir, "newfile.go")
	err = os.WriteFile(newFile, []byte(newContent), 0644)
	require.NoError(t, err)

	// Wait for watcher to detect and index new file
	time.Sleep(2 * time.Second)

	// Search for function in new file
	var newResults []searchtypes.Result
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		newResults, err = client.Search("NewlyCreatedFunction", types.SearchOptions{}, 100)
		require.NoError(t, err)
		if len(newResults) >= 1 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	require.GreaterOrEqual(t, len(newResults), 1, "Should automatically detect and index new file")

	// Verify found in correct file
	foundInNewFile := false
	for _, r := range newResults {
		if strings.Contains(r.Path, "newfile.go") {
			foundInNewFile = true
			break
		}
	}
	assert.True(t, foundInNewFile, "Should find NewlyCreatedFunction in newfile.go")
}

// TestWatcherIntegration_FileDeleteDetection tests automatic detection of deleted files
func TestWatcherIntegration_FileDeleteDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file watcher integration test in short mode")
	}

	testDir := t.TempDir()

	// Create two files
	keepContent := `package test
func KeepThisFunction() {}
`
	deleteContent := `package test
func DeleteThisFunction() {}
`
	keepFile := filepath.Join(testDir, "keep.go")
	deleteFile := filepath.Join(testDir, "delete.go")

	err := os.WriteFile(keepFile, []byte(keepContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(deleteFile, []byte(deleteContent), 0644)
	require.NoError(t, err)

	// Setup watched index
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			WatchMode:       true,
			WatchDebounceMs: 100,
			MaxFileSize:     10 * 1024 * 1024,
		},
	}

	externalIndexer := indexing.NewMasterIndex(cfg)
	err = externalIndexer.IndexDirectory(context.Background(), testDir)
	require.NoError(t, err)

	searchEngine := search.NewEngine(externalIndexer)
	externalIndexer.SetSearchEngine(searchEngine)

	require.NoError(t, err)

	srv, err := NewIndexServerWithIndex(cfg, externalIndexer, searchEngine)
	require.NoError(t, err)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	client := NewClient()

	// Verify both functions found initially
	results, err := client.Search("KeepThisFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	results, err = client.Search("DeleteThisFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Delete file (watcher should detect)
	err = os.Remove(deleteFile)
	require.NoError(t, err)

	// Wait for watcher to detect deletion and update index
	time.Sleep(2 * time.Second)

	// Deleted function should NOT be found
	maxRetries := 5
	deletedStillFound := true
	for i := 0; i < maxRetries; i++ {
		results, err = client.Search("DeleteThisFunction", types.SearchOptions{}, 100)
		require.NoError(t, err)

		foundInDeletedFile := false
		for _, r := range results {
			if strings.Contains(r.Path, "delete.go") {
				foundInDeletedFile = true
				break
			}
		}

		if !foundInDeletedFile {
			deletedStillFound = false
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	assert.False(t, deletedStillFound, "Should NOT find DeleteThisFunction after file deletion")

	// Kept function should still be found
	results, err = client.Search("KeepThisFunction", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "KeepThisFunction should still be found")
}

// TestWatcherIntegration_MultipleSequentialEdits tests multiple rapid file edits
func TestWatcherIntegration_MultipleSequentialEdits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file watcher integration test in short mode")
	}

	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "multi.go")

	// Version 1
	v1Content := `package test
func Version1Function() {}
`
	err := os.WriteFile(testFile, []byte(v1Content), 0644)
	require.NoError(t, err)

	// Setup watched index
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			WatchMode:       true,
			WatchDebounceMs: 200, // Longer debounce to handle multiple edits
			MaxFileSize:     10 * 1024 * 1024,
		},
	}

	externalIndexer := indexing.NewMasterIndex(cfg)
	err = externalIndexer.IndexDirectory(context.Background(), testDir)
	require.NoError(t, err)

	searchEngine := search.NewEngine(externalIndexer)
	externalIndexer.SetSearchEngine(searchEngine)

	require.NoError(t, err)

	srv, err := NewIndexServerWithIndex(cfg, externalIndexer, searchEngine)
	require.NoError(t, err)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	client := NewClient()

	// Verify Version1 found
	results, err := client.Search("Version1Function", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)

	// Update to Version 2
	v2Content := `package test
func Version2Function() {}
`
	err = os.WriteFile(testFile, []byte(v2Content), 0644)
	require.NoError(t, err)

	// Wait for debounce + rebuild
	time.Sleep(1500 * time.Millisecond)

	// Version2 should be found
	results, err = client.Search("Version2Function", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find Version2Function")

	// Update to Version 3
	v3Content := `package test
func Version3Function() {}
`
	err = os.WriteFile(testFile, []byte(v3Content), 0644)
	require.NoError(t, err)

	time.Sleep(1500 * time.Millisecond)

	// Version3 should be found
	results, err = client.Search("Version3Function", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "Should find Version3Function")

	// Version1 and Version2 should NOT be found (stale data check)
	results, err = client.Search("Version1Function", types.SearchOptions{}, 100)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotContains(t, r.Path, "multi.go", "Version1 should not be found after updates")
	}

	results, err = client.Search("Version2Function", types.SearchOptions{}, 100)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotContains(t, r.Path, "multi.go", "Version2 should not be found after updates")
	}
}

// TestWatcherIntegration_MCPScenario tests the full MCP scenario with shared server
func TestWatcherIntegration_MCPScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP scenario integration test in short mode")
	}

	testDir := t.TempDir()

	// Simulate MCP starting with file watching
	initialContent := `package test

type UserService struct {
	db Database
}

func (s *UserService) GetUser(id string) (*User, error) {
	return s.db.FindUser(id)
}
`
	serviceFile := filepath.Join(testDir, "service.go")
	err := os.WriteFile(serviceFile, []byte(initialContent), 0644)
	require.NoError(t, err)

	// MCP creates and manages the index with watching
	cfg := &config.Config{
		Project: config.Project{
			Root: testDir,
		},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index: config.Index{
			WatchMode:       true,
			WatchDebounceMs: 100,
			MaxFileSize:     10 * 1024 * 1024,
		},
	}

	mcpIndexer := indexing.NewMasterIndex(cfg)
	err = mcpIndexer.IndexDirectory(context.Background(), testDir)
	require.NoError(t, err)

	searchEngine := search.NewEngine(mcpIndexer)
	mcpIndexer.SetSearchEngine(searchEngine)

	require.NoError(t, err)

	// MCP starts shared index server
	srv, err := NewIndexServerWithIndex(cfg, mcpIndexer, searchEngine)
	require.NoError(t, err)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	// CLI client connects to MCP's shared server
	cliClient := NewClient()
	require.True(t, cliClient.IsServerRunning(), "CLI should connect to MCP's server")

	// CLI searches for UserService
	results, err := cliClient.Search("UserService", types.SearchOptions{}, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "CLI should find UserService via MCP's index")

	// User edits file in their editor (simulated)
	editedContent := `package test

type UserService struct {
	db      Database
	cache   Cache  // New field added
	metrics Metrics // New field added
}

func (s *UserService) GetUser(id string) (*User, error) {
	// Check cache first
	if user := s.cache.Get(id); user != nil {
		s.metrics.RecordCacheHit()
		return user, nil
	}

	user, err := s.db.FindUser(id)
	if err == nil {
		s.cache.Set(id, user)
	}
	return user, err
}

// New method
func (s *UserService) InvalidateCache(id string) {
	s.cache.Delete(id)
}
`
	err = os.WriteFile(serviceFile, []byte(editedContent), 0644)
	require.NoError(t, err)

	// Wait for automatic detection and rebuild
	time.Sleep(2 * time.Second)

	// CLI searches for new functionality
	var cacheResults []searchtypes.Result
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		cacheResults, err = cliClient.Search("InvalidateCache", types.SearchOptions{}, 100)
		require.NoError(t, err)
		if len(cacheResults) >= 1 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	require.GreaterOrEqual(t, len(cacheResults), 1, "CLI should find new InvalidateCache method via automatic update")

	// Verify it's in the correct file
	foundInService := false
	for _, r := range cacheResults {
		if strings.Contains(r.Path, "service.go") {
			foundInService = true
			break
		}
	}
	assert.True(t, foundInService, "Should find InvalidateCache in service.go")

	// Both MCP and CLI should see the same updated index
	// This verifies the shared index architecture works correctly
}
