package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SnapshotManager handles snapshot testing for workflow tests
type SnapshotManager struct {
	T           *testing.T
	SnapshotDir string
	UpdateMode  bool
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(t *testing.T, category string) *SnapshotManager {
	t.Helper()

	// Get snapshot directory
	cwd, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	snapshotDir := filepath.Join(cwd, "workflow_testdata", "snapshots", category)

	// Create snapshot directory if it doesn't exist
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatalf("Failed to create snapshot directory: %v", err)
	}

	// Check if we're in update mode
	updateMode := os.Getenv("UPDATE_SNAPSHOTS") == "1"

	return &SnapshotManager{
		T:           t,
		SnapshotDir: snapshotDir,
		UpdateMode:  updateMode,
	}
}

// ValidateSnapshot compares data against a golden file snapshot
func (sm *SnapshotManager) ValidateSnapshot(name string, data interface{}) {
	sm.T.Helper()

	// Convert data to pretty-printed JSON
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	require.NoError(sm.T, err, "Failed to marshal data for snapshot '%s'", name)

	snapshotPath := filepath.Join(sm.SnapshotDir, name+".json")

	if sm.UpdateMode {
		// Update mode: write new snapshot
		err := os.WriteFile(snapshotPath, jsonBytes, 0644)
		require.NoError(sm.T, err, "Failed to write snapshot '%s'", name)
		sm.T.Logf("Updated snapshot: %s", name)
		return
	}

	// Normal mode: compare against existing snapshot
	expectedBytes, err := os.ReadFile(snapshotPath)
	if os.IsNotExist(err) {
		// Snapshot doesn't exist - write it and fail the test
		actualPath := filepath.Join(sm.SnapshotDir, name+".actual.json")
		_ = os.WriteFile(actualPath, jsonBytes, 0644)
		sm.T.Fatalf("Snapshot '%s' does not exist. Created .actual.json file. Review and run with UPDATE_SNAPSHOTS=1 to accept.", name)
		return
	}
	require.NoError(sm.T, err, "Failed to read snapshot '%s'", name)

	// Compare
	if !bytes.Equal(jsonBytes, expectedBytes) {
		// Write actual output for debugging
		actualPath := filepath.Join(sm.SnapshotDir, name+".actual.json")
		_ = os.WriteFile(actualPath, jsonBytes, 0644)

		// Write diff for easy comparison
		diffPath := filepath.Join(sm.SnapshotDir, name+".diff")
		diff := generateSimpleDiff(string(expectedBytes), string(jsonBytes))
		_ = os.WriteFile(diffPath, []byte(diff), 0644)

		sm.T.Errorf("Snapshot mismatch for '%s'.\n  Expected: %s\n  Actual: %s\n  Diff: %s\n\nRun with UPDATE_SNAPSHOTS=1 to update.",
			name, snapshotPath, actualPath, diffPath)
	}
}

// ValidateStructure validates the structure/schema without exact content matching
func (sm *SnapshotManager) ValidateStructure(name string, data interface{}, requiredFields []string) {
	sm.T.Helper()

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		sm.T.Fatalf("Data for snapshot '%s' is not a map", name)
	}

	for _, field := range requiredFields {
		if _, exists := dataMap[field]; !exists {
			sm.T.Errorf("Required field '%s' missing from snapshot '%s'", field, name)
		}
	}
}

// generateSimpleDiff generates a simple line-by-line diff
func generateSimpleDiff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString("--- Expected\n")
	diff.WriteString("+++ Actual\n")

	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			if expLine != "" {
				diff.WriteString(fmt.Sprintf("- %s\n", expLine))
			}
			if actLine != "" {
				diff.WriteString(fmt.Sprintf("+ %s\n", actLine))
			}
		}
	}

	return diff.String()
}

// CompareSnapshots compares two snapshot files and returns whether they match
func CompareSnapshots(t *testing.T, snapshot1, snapshot2 string) bool {
	t.Helper()

	data1, err := os.ReadFile(snapshot1)
	if err != nil {
		return false
	}

	data2, err := os.ReadFile(snapshot2)
	if err != nil {
		return false
	}

	return bytes.Equal(data1, data2)
}

// SnapshotTestCase represents a test case with snapshot validation
type SnapshotTestCase struct {
	Name           string
	SearchPattern  string
	SearchOptions  SearchOptions
	ContextOptions ContextOptions
	SnapshotName   string
	// Assertions to run before snapshot validation
	PreSnapshotAssertions func(*testing.T, map[string]interface{})
}

// RunSnapshotTestCases runs a suite of snapshot test cases
func RunSnapshotTestCases(t *testing.T, ctx *WorkflowTestContext, category string, cases []SnapshotTestCase) {
	t.Helper()

	snapshots := NewSnapshotManager(t, category)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Execute search
			searchOpts := tc.SearchOptions
			searchOpts.Pattern = tc.SearchPattern
			searchResult := ctx.Search(tc.Name, searchOpts)

			if len(searchResult.Results) == 0 {
				t.Skipf("No search results found for pattern '%s'", tc.SearchPattern)
				return
			}

			// Get context for first result
			contextResult := ctx.GetObjectContext(tc.Name, 0, tc.ContextOptions)

			// Run custom assertions
			if tc.PreSnapshotAssertions != nil {
				tc.PreSnapshotAssertions(t, contextResult)
			}

			// Validate snapshot
			snapshotName := tc.SnapshotName
			if snapshotName == "" {
				// Generate snapshot name from test name
				snapshotName = strings.ReplaceAll(tc.Name, " ", "_")
				snapshotName = strings.ToLower(snapshotName)
			}

			snapshots.ValidateSnapshot(snapshotName, contextResult)
		})
	}
}
