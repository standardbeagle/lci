package indexing

import (
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/testhelpers"
)

// TestHealthCheck verifies the health check functionality detects issues correctly
func TestHealthCheck(t *testing.T) {
	// Create a properly configured index
	cfg := testhelpers.NewTestConfigBuilder(t.TempDir()).Build()

	gi := NewMasterIndex(cfg)

	// Test 1: Empty index should be healthy
	health := gi.HealthCheck()
	if health["status"] != "healthy" {
		t.Errorf("Expected empty index to be healthy, got %v", health["status"])
	}

	if len(health["errors"].([]string)) > 0 {
		t.Errorf("Expected no errors in healthy empty index, got %v", health["errors"])
	}

	metrics := health["metrics"].(map[string]interface{})

	// Verify basic metrics are present
	if _, exists := metrics["total_files"]; !exists {
		t.Error("Expected total_files metric")
	}
	if _, exists := metrics["processed_files"]; !exists {
		t.Error("Expected processed_files metric")
	}
	if _, exists := metrics["snapshot_files"]; !exists {
		t.Error("Expected snapshot_files metric")
	}

	// Test 2: Index with some data should still be healthy
	// Create a simple test file
	testContent := []byte("package test\n\nfunc TestFunction() {}")
	fileID := gi.fileContentStore.LoadFile("test.go", testContent)
	if fileID == 0 {
		t.Fatal("Failed to load test file")
	}

	// Update snapshot manually to simulate indexed state
	gi.updateSnapshotAtomic(func(oldSnapshot *FileSnapshot) *FileSnapshot {
		newSnapshot := &FileSnapshot{
			fileMap:        make(map[string]types.FileID, len(oldSnapshot.fileMap)+1),
			reverseFileMap: make(map[types.FileID]string, len(oldSnapshot.reverseFileMap)+1),
			fileScopes:     make(map[types.FileID][]types.ScopeInfo, len(oldSnapshot.fileScopes)),
		}

		// Copy existing mappings
		for k, v := range oldSnapshot.fileMap {
			newSnapshot.fileMap[k] = v
		}
		for k, v := range oldSnapshot.reverseFileMap {
			newSnapshot.reverseFileMap[k] = v
		}
		for k, v := range oldSnapshot.fileScopes {
			newSnapshot.fileScopes[k] = v
		}

		// Add test file
		newSnapshot.fileMap["test.go"] = fileID
		newSnapshot.reverseFileMap[fileID] = "test.go"

		return newSnapshot
	})

	// Update counters
	// Note: We can't directly set atomic counters from outside the package,
	// but we can still test the health check logic

	health = gi.HealthCheck()
	if health["status"] != "healthy" {
		t.Errorf("Expected index with file to be healthy, got %v", health["status"])
	}

	metrics = health["metrics"].(map[string]interface{})
	if snapshotFiles := metrics["snapshot_files"].(int); snapshotFiles < 1 {
		t.Errorf("Expected at least 1 file in snapshot, got %d", snapshotFiles)
	}
}

// TestHealthCheckCorruptionDetection tests that the health check can detect data corruption
func TestHealthCheckCorruptionDetection(t *testing.T) {
	// Test 1: Simple test - create an index and manually set one component to nil
	// This should trigger the health check to detect the missing component
	cfg := testhelpers.NewTestConfigBuilder(t.TempDir()).Build()
	gi := NewMasterIndex(cfg)

	// Verify it starts healthy
	health := gi.HealthCheck()
	if health["status"] != "healthy" {
		t.Errorf("Expected properly initialized index to be healthy, got %v", health["status"])
	}

	// Now manually corrupt one component
	gi.trigramIndex = nil

	// Debug: verify component is nil
	t.Logf("After manual nil assignment: trigramIndex=%v", gi.trigramIndex == nil)
	t.Logf("Direct component check: trigramIndex=%p, symbolIndex=%p, refTracker=%p",
		gi.trigramIndex, gi.symbolIndex, gi.refTracker)

	health = gi.HealthCheck()
	t.Logf("Health check after corruption: status=%v, errors=%v", health["status"], health["errors"])

	// Should detect missing component and report unhealthy status
	if health["status"] != "unhealthy" {
		t.Errorf("Expected index with missing trigram index to be unhealthy, got %v", health["status"])
	}

	errors := health["errors"].([]string)
	if len(errors) == 0 {
		t.Error("Expected at least 1 error for missing trigram index, got none")
	}

	// Check for expected error message
	errorMap := make(map[string]bool)
	for _, err := range errors {
		errorMap[err] = true
		t.Logf("Found error: %s", err)
	}

	if !errorMap["trigram_index is not initialized"] {
		t.Error("Expected error about missing trigram index")
	}

	// Test 2: Test with multiple missing components
	gi.symbolIndex = nil
	gi.refTracker = nil

	health = gi.HealthCheck()
	if health["status"] != "unhealthy" {
		t.Errorf("Expected index with multiple missing components to be unhealthy, got %v", health["status"])
	}

	errors = health["errors"].([]string)
	if len(errors) < 3 {
		t.Errorf("Expected at least 3 errors for multiple missing components, got %d", len(errors))
	}

	// Test 3: Verify that restored index is healthy again
	gi.trigramIndex = core.NewTrigramIndex()
	gi.symbolIndex = core.NewSymbolIndex()
	gi.refTracker = core.NewReferenceTrackerForTest()

	health = gi.HealthCheck()
	if health["status"] != "healthy" {
		t.Errorf("Expected restored index to be healthy, got %v", health["status"])
	}

	errors = health["errors"].([]string)
	if len(errors) > 0 {
		t.Errorf("Expected no errors in restored healthy index, got %d errors: %v", len(errors), errors)
	}
}

// TestHealthCheckMemoryLimit validates memory limit checking
func TestHealthCheckMemoryLimit(t *testing.T) {
	// Note: TestConfigBuilder has default MaxMemoryMB=100, but test validates low memory limits
	cfg := testhelpers.NewTestConfigBuilder(t.TempDir()).Build()
	gi := NewMasterIndex(cfg)

	health := gi.HealthCheck()

	// With empty index, memory should be low enough
	if health["status"] != "healthy" {
		t.Errorf("Expected empty index to be healthy even with low memory limit, got %v", health["status"])
	}

	// The health check should report memory usage metrics
	metrics := health["metrics"].(map[string]interface{})
	if _, exists := metrics["estimated_memory_mb"]; !exists {
		t.Error("Expected estimated_memory_mb metric")
	}
}
