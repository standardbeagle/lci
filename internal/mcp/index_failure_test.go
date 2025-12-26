package mcp

import (
	"context"
	"encoding/json"
	"testing"

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
