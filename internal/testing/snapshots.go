package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// SnapshotMode determines how snapshots are handled
type SnapshotMode string

const (
	// SnapshotModeCompare compares against existing snapshots
	SnapshotModeCompare SnapshotMode = "compare"
	// SnapshotModeUpdate updates snapshots with current values
	SnapshotModeUpdate SnapshotMode = "update"
)

// GetSnapshotMode returns the current snapshot mode from environment
func GetSnapshotMode() SnapshotMode {
	if os.Getenv("UPDATE_SNAPSHOTS") == "true" {
		return SnapshotModeUpdate
	}
	return SnapshotModeCompare
}

// ReferenceSnapshot represents a snapshot of symbol reference data
type ReferenceSnapshot struct {
	SymbolName    string `json:"symbol_name"`
	IncomingCount int    `json:"incoming_count"`
	OutgoingCount int    `json:"outgoing_count"`
	HasID         bool   `json:"has_id"`
	HasFileID     bool   `json:"has_file_id"`
	IncomingRefs  int    `json:"incoming_refs_length"`
	OutgoingRefs  int    `json:"outgoing_refs_length"`
}

// ProjectSnapshot represents reference data for an entire project
type ProjectSnapshot struct {
	ProjectName string                       `json:"project_name"`
	Language    string                       `json:"language"`
	FileCount   int                          `json:"file_count"`
	SymbolCount int                          `json:"symbol_count"`
	References  map[string]ReferenceSnapshot `json:"references"`
}

// SearchTest represents a search test case
type SearchTest struct {
	Name    string      `json:"name"`
	Pattern string      `json:"pattern"`
	Options interface{} `json:"options"` // types.SearchOptions
}

// SearchTestResult represents the result of a search test
type SearchTestResult struct {
	Pattern             string      `json:"pattern"`
	Options             interface{} `json:"options"`
	BasicResultCount    int         `json:"basic_result_count"`
	EnhancedResultCount int         `json:"enhanced_result_count"`
	FirstResultLine     int         `json:"first_result_line"`
	FirstResultPath     string      `json:"first_result_path"`
	HasRelationalData   bool        `json:"has_relational_data"`
	SymbolTypesFound    []string    `json:"symbol_types_found"`
}

// SearchBehaviorSnapshot represents comprehensive search behavior test results
type SearchBehaviorSnapshot struct {
	ProjectName string                      `json:"project_name"`
	Language    string                      `json:"language"`
	FileCount   int                         `json:"file_count"`
	SymbolCount int                         `json:"symbol_count"`
	SearchTests map[string]SearchTestResult `json:"search_tests"`
}

// PerformanceTestResult represents the result of a performance test
type PerformanceTestResult struct {
	Pattern        string      `json:"pattern"`
	Options        interface{} `json:"options"`
	AvgResultCount float64     `json:"avg_result_count"`
	Iterations     int         `json:"iterations"`
}

// SearchPerformanceSnapshot represents performance test results
type SearchPerformanceSnapshot struct {
	ProjectName      string                           `json:"project_name"`
	FileCount        int                              `json:"file_count"`
	SymbolCount      int                              `json:"symbol_count"`
	IndexSizeMB      float64                          `json:"index_size_mb"`
	PerformanceTests map[string]PerformanceTestResult `json:"performance_tests"`
}

// SnapshotPath returns the path for a snapshot file
func SnapshotPath(testName string) string {
	return filepath.Join("testdata", "snapshots", testName+".json")
}

// LoadSnapshot loads a snapshot from disk
func LoadSnapshot(testName string) (*ProjectSnapshot, error) {
	path := SnapshotPath(testName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var snapshot ProjectSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("invalid snapshot format: %w", err)
	}

	return &snapshot, nil
}

// SaveSnapshot saves a snapshot to disk
func SaveSnapshot(testName string, snapshot *ProjectSnapshot) error {
	path := SnapshotPath(testName)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// CompareSnapshots compares two snapshots and reports differences
func CompareSnapshots(t *testing.T, expected, actual *ProjectSnapshot) {
	t.Helper()

	if expected.Language != actual.Language {
		t.Errorf("Language mismatch: expected %s, got %s", expected.Language, actual.Language)
	}

	if expected.FileCount != actual.FileCount {
		t.Errorf("FileCount mismatch: expected %d, got %d", expected.FileCount, actual.FileCount)
	}

	if expected.SymbolCount != actual.SymbolCount {
		t.Errorf("SymbolCount mismatch: expected %d, got %d", expected.SymbolCount, actual.SymbolCount)
	}

	// Check each expected reference
	for symbolName, expectedRef := range expected.References {
		actualRef, found := actual.References[symbolName]
		if !found {
			t.Errorf("Symbol %s not found in actual results", symbolName)
			continue
		}

		if expectedRef.IncomingCount != actualRef.IncomingCount {
			t.Errorf("Symbol %s: IncomingCount mismatch: expected %d, got %d",
				symbolName, expectedRef.IncomingCount, actualRef.IncomingCount)
		}

		if expectedRef.OutgoingCount != actualRef.OutgoingCount {
			t.Errorf("Symbol %s: OutgoingCount mismatch: expected %d, got %d",
				symbolName, expectedRef.OutgoingCount, actualRef.OutgoingCount)
		}

		if expectedRef.HasID != actualRef.HasID {
			t.Errorf("Symbol %s: HasID mismatch: expected %v, got %v",
				symbolName, expectedRef.HasID, actualRef.HasID)
		}

		if expectedRef.HasFileID != actualRef.HasFileID {
			t.Errorf("Symbol %s: HasFileID mismatch: expected %v, got %v",
				symbolName, expectedRef.HasFileID, actualRef.HasFileID)
		}
	}

	// Check for unexpected symbols
	for symbolName := range actual.References {
		if _, found := expected.References[symbolName]; !found {
			t.Errorf("Unexpected symbol %s in actual results", symbolName)
		}
	}
}

// AssertSnapshot compares or updates a snapshot based on mode
func AssertSnapshot(t *testing.T, testName string, actual interface{}) {
	t.Helper()

	mode := GetSnapshotMode()
	path := SnapshotPath(testName)

	if mode == SnapshotModeUpdate {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create snapshot directory: %v", err)
		}

		data, err := json.MarshalIndent(actual, "", "  ")
		if err != nil {
			t.Fatalf("Failed to marshal snapshot: %v", err)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("Failed to save snapshot: %v", err)
		}

		t.Logf("Updated snapshot for %s", testName)
		return
	}

	// Compare mode
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("Snapshot not found for %s. Run with UPDATE_SNAPSHOTS=true to create it.", testName)
		}
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	// Unmarshal into same type as actual
	switch actual := actual.(type) {
	case *ProjectSnapshot:
		var expected ProjectSnapshot
		if err := json.Unmarshal(data, &expected); err != nil {
			t.Fatalf("Invalid snapshot format: %v", err)
		}
		CompareSnapshots(t, &expected, actual)
	case *SearchBehaviorSnapshot:
		var expected SearchBehaviorSnapshot
		if err := json.Unmarshal(data, &expected); err != nil {
			t.Fatalf("Invalid snapshot format: %v", err)
		}
		CompareSearchBehaviorSnapshots(t, &expected, actual)
	case *SearchPerformanceSnapshot:
		var expected SearchPerformanceSnapshot
		if err := json.Unmarshal(data, &expected); err != nil {
			t.Fatalf("Invalid snapshot format: %v", err)
		}
		CompareSearchPerformanceSnapshots(t, &expected, actual)
	default:
		t.Fatalf("Unsupported snapshot type: %T", actual)
	}
}

// CompareSearchBehaviorSnapshots compares two search behavior snapshots
func CompareSearchBehaviorSnapshots(t *testing.T, expected, actual *SearchBehaviorSnapshot) {
	t.Helper()

	if expected.Language != actual.Language {
		t.Errorf("Language mismatch: expected %s, got %s", expected.Language, actual.Language)
	}

	if expected.FileCount != actual.FileCount {
		t.Errorf("FileCount mismatch: expected %d, got %d", expected.FileCount, actual.FileCount)
	}

	if expected.SymbolCount != actual.SymbolCount {
		t.Errorf("SymbolCount mismatch: expected %d, got %d", expected.SymbolCount, actual.SymbolCount)
	}

	// Compare search tests
	for testName, expectedResult := range expected.SearchTests {
		actualResult, found := actual.SearchTests[testName]
		if !found {
			t.Errorf("Search test %s not found in actual results", testName)
			continue
		}

		if expectedResult.Pattern != actualResult.Pattern {
			t.Errorf("Test %s: Pattern mismatch: expected %s, got %s",
				testName, expectedResult.Pattern, actualResult.Pattern)
		}

		if expectedResult.BasicResultCount != actualResult.BasicResultCount {
			t.Errorf("Test %s: BasicResultCount mismatch: expected %d, got %d",
				testName, expectedResult.BasicResultCount, actualResult.BasicResultCount)
		}

		if expectedResult.EnhancedResultCount != actualResult.EnhancedResultCount {
			t.Errorf("Test %s: EnhancedResultCount mismatch: expected %d, got %d",
				testName, expectedResult.EnhancedResultCount, actualResult.EnhancedResultCount)
		}

		if expectedResult.HasRelationalData != actualResult.HasRelationalData {
			t.Errorf("Test %s: HasRelationalData mismatch: expected %v, got %v",
				testName, expectedResult.HasRelationalData, actualResult.HasRelationalData)
		}
	}

	// Check for unexpected tests
	for testName := range actual.SearchTests {
		if _, found := expected.SearchTests[testName]; !found {
			t.Errorf("Unexpected search test %s in actual results", testName)
		}
	}
}

// CompareSearchPerformanceSnapshots compares two search performance snapshots
func CompareSearchPerformanceSnapshots(t *testing.T, expected, actual *SearchPerformanceSnapshot) {
	t.Helper()

	if expected.FileCount != actual.FileCount {
		t.Errorf("FileCount mismatch: expected %d, got %d", expected.FileCount, actual.FileCount)
	}

	if expected.SymbolCount != actual.SymbolCount {
		t.Errorf("SymbolCount mismatch: expected %d, got %d", expected.SymbolCount, actual.SymbolCount)
	}

	// Compare performance tests - allow some variance in performance numbers
	for testName, expectedResult := range expected.PerformanceTests {
		actualResult, found := actual.PerformanceTests[testName]
		if !found {
			t.Errorf("Performance test %s not found in actual results", testName)
			continue
		}

		if expectedResult.Pattern != actualResult.Pattern {
			t.Errorf("Test %s: Pattern mismatch: expected %s, got %s",
				testName, expectedResult.Pattern, actualResult.Pattern)
		}

		// Allow 20% variance in result counts for performance tests
		expectedCount := expectedResult.AvgResultCount
		actualCount := actualResult.AvgResultCount
		variance := 0.20

		if actualCount < expectedCount*(1-variance) || actualCount > expectedCount*(1+variance) {
			t.Errorf("Test %s: AvgResultCount variance too high: expected ~%.1f, got %.1f",
				testName, expectedCount, actualCount)
		}
	}
}
