package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// TestIndexFailureHandling tests that search operations properly handle index failures
// with helpful error messages
func TestIndexFailureHandling(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Project: config.Project{
			Name: "test-project",
			Root: t.TempDir(),
		},
		Index: config.Index{
			MaxFileSize:      1048576,
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			FollowSymlinks:   false,
			RespectGitignore: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:   512,
			MaxGoroutines: 4,
		},
		Include: []string{"**/*.go"},
		Exclude: []string{"vendor/**", "node_modules/**"},
	}

	// Create MasterIndex
	gi := indexing.NewMasterIndex(cfg)
	defer gi.Close()

	// Create MCP server
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test 1: Check that index is available when properly initialized
	t.Run("IndexAvailableAfterInitialization", func(t *testing.T) {
		available, err := server.checkIndexAvailability()
		if err != nil {
			t.Fatalf("Expected no error for initialized index, got: %v", err)
		}
		if !available {
			t.Error("Expected index to be available")
		}
	})

	// Test 2: Check that search fails gracefully when index is nil
	t.Run("SearchFailsWithNilIndex", func(t *testing.T) {
		// Temporarily set goroutineIndex to nil to simulate uninitialized state
		originalIndex := server.goroutineIndex
		server.goroutineIndex = nil

		// Try to search
		_, err := server.Search(context.Background(), SearchParams{
			Pattern: "test",
		})

		// Restore the index
		server.goroutineIndex = originalIndex

		// Verify error message is helpful
		if err == nil {
			t.Error("Expected error when index is nil")
		} else {
			errorMsg := err.Error()
			// Check that error message contains helpful guidance
			if !stringContains(errorMsg, "index not initialized") && !stringContains(errorMsg, "project directory") {
				t.Errorf("Error message should contain helpful guidance, got: %s", errorMsg)
			}
			t.Logf("Got expected error with guidance: %s", errorMsg)
		}
	})

	// Test 3: Check that search fails gracefully when indexing failed
	t.Run("SearchFailsWithFailedIndexing", func(t *testing.T) {
		// Simulate failed indexing state
		server.indexingMutex.Lock()
		server.indexingState.Status = "failed"
		server.indexingState.ErrorMessage = "permission denied"
		server.indexingState.CanCancel = false
		server.indexingMutex.Unlock()

		// Try to search
		_, err := server.Search(context.Background(), SearchParams{
			Pattern: "test",
		})

		// Reset state
		server.indexingMutex.Lock()
		server.indexingState.Status = "idle"
		server.indexingState.ErrorMessage = ""
		server.indexingState.CanCancel = false
		server.indexingMutex.Unlock()

		// Verify error message is helpful
		if err == nil {
			t.Error("Expected error when indexing failed")
		} else {
			errorMsg := err.Error()
			// Check that error message contains troubleshooting guidance
			if !stringContains(errorMsg, "indexing failed") {
				t.Errorf("Error message should mention indexing failure, got: %s", errorMsg)
			}
			if !stringContains(errorMsg, "permission") {
				t.Errorf("Error message should mention the original error, got: %s", errorMsg)
			}
			if !stringContains(errorMsg, "Troubleshooting") {
				t.Errorf("Error message should include troubleshooting steps, got: %s", errorMsg)
			}
			t.Logf("Got expected error with troubleshooting: %s", errorMsg)
		}
	})

	// Test 4: Check that get_object_context fails gracefully when index is unavailable
	t.Run("GetObjectContextFailsWithNoIndex", func(t *testing.T) {
		// Temporarily set goroutineIndex to nil
		originalIndex := server.goroutineIndex
		server.goroutineIndex = nil

		// Create mock parameters
		paramsBytes, _ := json.Marshal(ObjectContextParams{
			ID: "VE", // Use concise object ID format
		})

		// Try to get object context
		result, _ := server.handleGetObjectContext(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
			Arguments: paramsBytes,
		}})

		// Restore the index
		server.goroutineIndex = originalIndex

		// Verify error response - handlers return result with error info, not Go error
		// The result should contain error content even if the returned error is nil
		if result == nil {
			t.Error("Expected result when index is unavailable")
		} else {
			t.Logf("Got result: %v", result)
			// Check if the result contains an error
			if result.Content == nil || len(result.Content) == 0 {
				t.Error("Expected error content in result when index is unavailable")
			} else {
				t.Logf("Got expected error content in result")
			}
		}
	})

	// Test 5: Check that checkIndexAvailability returns false for failed state
	t.Run("CheckIndexAvailabilityDetectsFailedState", func(t *testing.T) {
		// Set up failed state
		server.indexingMutex.Lock()
		server.indexingState.Status = "failed"
		server.indexingState.ErrorMessage = "disk full"
		server.indexingMutex.Unlock()

		// Check availability
		available, err := server.checkIndexAvailability()

		// Reset state
		server.indexingMutex.Lock()
		server.indexingState.Status = "idle"
		server.indexingState.ErrorMessage = ""
		server.indexingMutex.Unlock()

		// Verify
		if available {
			t.Error("Expected available to be false for failed state")
		}
		if err == nil {
			t.Error("Expected error for failed state")
		} else {
			errorMsg := err.Error()
			if !stringContains(errorMsg, "disk full") {
				t.Errorf("Error should include original error message, got: %s", errorMsg)
			}
			if !stringContains(errorMsg, "Troubleshooting") {
				t.Errorf("Error should include troubleshooting steps, got: %s", errorMsg)
			}
		}
	})
}

// TestCheckIndexAvailability_StateSyncOnStart verifies that when autoIndexManager
// starts indexing, the server's indexingState.Status is immediately updated so that
// checkIndexAvailability() does not see stale "idle" status and allow requests through
// to an empty index. This was the root cause of the ABC-Bench empty results bug.
func TestCheckIndexAvailability_StateSyncOnStart(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{
			Name: "test-project",
			Root: t.TempDir(),
		},
		Index: config.Index{
			MaxFileSize:    1048576,
			MaxTotalSizeMB: 100,
			MaxFileCount:   1000,
		},
		Performance: config.Performance{
			MaxMemoryMB:   512,
			MaxGoroutines: 4,
		},
		Include: []string{"**/*.go"},
	}

	gi := indexing.NewMasterIndex(cfg)
	defer gi.Close()

	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	t.Run("ServerStatusSyncedAfterAutoIndexStart", func(t *testing.T) {
		// Simulate the state as if no auto-indexing ran yet (reset to idle).
		server.indexingMutex.Lock()
		server.indexingState.Status = "idle"
		server.indexingMutex.Unlock()

		// Create a fresh autoIndexManager and start it.
		// The root won't have a project marker, but what matters is that
		// startAutoIndexing sets the manager status and syncs to the server.
		manager := NewAutoIndexingManager(server)
		server.autoIndexManager = manager

		// Manually trigger the manager's startup sequence (same as startAutoIndexing
		// but without actually launching the goroutine, to test the sync step).
		atomic.StoreInt32(&manager.running, 1)
		manager.mu.Lock()
		manager.status = "estimating"
		manager.mu.Unlock()
		manager.syncWithServerState()

		// Verify: server's indexingState should now reflect "estimating", not "idle"
		server.indexingMutex.RLock()
		status := server.indexingState.Status
		server.indexingMutex.RUnlock()

		if status != "estimating" {
			t.Errorf("Expected server indexingState.Status to be %q after sync, got %q", "estimating", status)
		}

		// Reset
		atomic.StoreInt32(&manager.running, 0)
		manager.Close()
	})
}

// TestCheckIndexAvailability_ManagerAuthoritativeOverStaleServer verifies that
// checkIndexAvailability() uses the autoIndexManager as the authoritative source
// when the server's synced status is stale. This catches the case where the manager
// has moved to "indexing" but the server still shows "idle".
func TestCheckIndexAvailability_ManagerAuthoritativeOverStaleServer(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{
			Name: "test-project",
			Root: t.TempDir(),
		},
		Index: config.Index{
			MaxFileSize:    1048576,
			MaxTotalSizeMB: 100,
			MaxFileCount:   1000,
		},
		Performance: config.Performance{
			MaxMemoryMB:         512,
			MaxGoroutines:       4,
			IndexingTimeoutSec:  2, // Short timeout for test
		},
		Include: []string{"**/*.go"},
	}

	gi := indexing.NewMasterIndex(cfg)
	defer gi.Close()

	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	t.Run("StaleIdleWithRunningManager", func(t *testing.T) {
		// Set server state to "idle" (stale) while manager is actually running
		server.indexingMutex.Lock()
		server.indexingState.Status = "idle"
		server.indexingMutex.Unlock()

		manager := NewAutoIndexingManager(server)
		server.autoIndexManager = manager

		// Simulate manager in "indexing" state and running
		atomic.StoreInt32(&manager.running, 1)
		manager.mu.Lock()
		manager.status = "indexing"
		manager.mu.Unlock()

		// Complete the indexing immediately so waitForCompletion doesn't block
		go func() {
			time.Sleep(10 * time.Millisecond)
			manager.updateStatus("completed")
			atomic.StoreInt32(&manager.running, 0)
		}()

		// checkIndexAvailability should detect the mismatch, use the manager's
		// "indexing" status, and wait for completion rather than returning immediately
		available, err := server.checkIndexAvailability()
		if err != nil {
			t.Fatalf("Expected no error after completion, got: %v", err)
		}
		if !available {
			t.Error("Expected index to be available after completion")
		}

		manager.Close()
	})
}

// TestCheckIndexAvailability_AllStatuses verifies that every possible status
// from autoIndexManager is handled explicitly (no catch-all "treat as available").
func TestCheckIndexAvailability_AllStatuses(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{
			Name: "test-project",
			Root: t.TempDir(),
		},
		Index: config.Index{
			MaxFileSize:    1048576,
			MaxTotalSizeMB: 100,
			MaxFileCount:   1000,
		},
		Performance: config.Performance{
			MaxMemoryMB:         512,
			MaxGoroutines:       4,
			IndexingTimeoutSec:  1,
		},
		Include: []string{"**/*.go"},
	}

	gi := indexing.NewMasterIndex(cfg)
	defer gi.Close()

	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	tests := []struct {
		name           string
		status         string
		expectReady    bool
		expectWait     bool // true if checkIndexAvailability will try to wait
		expectErrPart  string
	}{
		{"idle", "idle", true, false, ""},
		{"completed", "completed", true, false, ""},
		{"cancelled", "cancelled", true, false, ""},
		{"failed", "failed", false, false, "indexing failed"},
		{"estimating_waits", "estimating", false, true, ""},
		{"waiting_waits", "waiting", false, true, ""},
		{"indexing_waits", "indexing", false, true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up fresh manager for each test
			manager := NewAutoIndexingManager(server)
			server.autoIndexManager = manager

			// Set server status directly
			server.indexingMutex.Lock()
			server.indexingState.Status = tc.status
			server.indexingState.ErrorMessage = ""
			if tc.status == "failed" {
				server.indexingState.ErrorMessage = "test error"
			}
			server.indexingMutex.Unlock()

			if tc.expectWait {
				// For in-progress statuses: manager is not running so
				// waitForCompletion returns the current status immediately.
				// The result depends on what status the manager is in.
				// Since manager is idle/not running, waitForCompletion returns immediately.
				available, err := server.checkIndexAvailability()

				// When the manager isn't running, waitForCompletion returns the
				// manager's current status (idle), which doesn't match "completed",
				// so we get "unexpected indexing status after wait: idle".
				if available {
					t.Errorf("Expected not-immediately-available for in-progress status %q", tc.status)
				}
				if err == nil {
					t.Errorf("Expected error for unresolved in-progress status %q", tc.status)
				}
			} else if tc.expectReady {
				available, err := server.checkIndexAvailability()
				if err != nil {
					t.Errorf("Expected no error for status %q, got: %v", tc.status, err)
				}
				if !available {
					t.Errorf("Expected available for status %q", tc.status)
				}
			} else {
				// Expect failure (e.g., "failed" status)
				available, err := server.checkIndexAvailability()
				if available {
					t.Errorf("Expected not-available for status %q", tc.status)
				}
				if err == nil {
					t.Errorf("Expected error for status %q", tc.status)
				} else if tc.expectErrPart != "" && !strings.Contains(err.Error(), tc.expectErrPart) {
					t.Errorf("Expected error containing %q, got: %v", tc.expectErrPart, err)
				}
			}

			manager.Close()
		})
	}
}

// TestAutoIndexManager_SyncOnStart verifies that startAutoIndexing() syncs the
// server's indexingState before launching the goroutine.
func TestAutoIndexManager_SyncOnStart(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{
			Name: "test-project",
			Root: t.TempDir(),
		},
		Index: config.Index{
			MaxFileSize:    1048576,
			MaxTotalSizeMB: 100,
			MaxFileCount:   1000,
		},
		Performance: config.Performance{
			MaxMemoryMB:   512,
			MaxGoroutines: 4,
		},
		Include: []string{"**/*.go"},
	}

	gi := indexing.NewMasterIndex(cfg)
	defer gi.Close()

	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Reset server state to idle
	server.indexingMutex.Lock()
	server.indexingState.Status = "idle"
	server.indexingMutex.Unlock()

	manager := NewAutoIndexingManager(server)
	server.autoIndexManager = manager

	// Start auto-indexing (root dir has no project marker, so the goroutine
	// will set status to "indexing" then complete quickly)
	err = manager.startAutoIndexing(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("startAutoIndexing failed: %v", err)
	}

	// IMMEDIATELY after startAutoIndexing returns (before goroutine runs),
	// the server's status should NOT be "idle" anymore.
	server.indexingMutex.RLock()
	status := server.indexingState.Status
	server.indexingMutex.RUnlock()

	if status == "idle" {
		t.Errorf("Server indexingState.Status is still %q immediately after startAutoIndexing; expected %q",
			status, "estimating")
	}
	if status != "estimating" {
		t.Errorf("Expected server indexingState.Status to be %q, got %q", "estimating", status)
	}

	// Wait for the goroutine to finish
	_, _ = manager.waitForCompletion(5 * time.Second)
	manager.Close()
}

// Helper function to check if a string contains a substring (case-sensitive)
// Uses the one from response.go
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		findSubstring(s, substr) >= 0)
}

// Simple substring search
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
