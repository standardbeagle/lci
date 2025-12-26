package searchtypes

import (
	"github.com/standardbeagle/lci/internal/types"
	"testing"
)

// Helper to create a test result with relational data
func makeTestResult(fileID types.FileID, localSymbolID uint32) StandardResult {
	return StandardResult{
		Result: GrepResult{
			FileID: fileID,
			Path:   "test.go",
		},
		RelationalData: &types.RelationalContext{
			Symbol: types.EnhancedSymbol{
				// Pack FileID and LocalSymbolID into SymbolID
				// Format: FileID in upper 32 bits, LocalSymbolID in next 32 bits
				ID: types.SymbolID(uint64(fileID)<<32 | uint64(localSymbolID)),
				Symbol: types.Symbol{
					Name: "TestFunction",
				},
			},
		},
	}
}

func TestPopulateDenseObjectIDs(t *testing.T) {
	t.Run("populates compact IDs for valid results", func(t *testing.T) {
		// Create test results with relational data
		results := []StandardResult{
			makeTestResult(types.FileID(123), 456),
			makeTestResult(types.FileID(789), 654),
		}

		// Populate dense object IDs
		PopulateDenseObjectIDs(results)

		// Verify first result has compact ID
		if results[0].ObjectID == "" {
			t.Error("Expected ObjectID to be populated for first result")
		}

		// Verify second result has compact ID
		if results[1].ObjectID == "" {
			t.Error("Expected ObjectID to be populated for second result")
		}

		// Verify IDs are different
		if results[0].ObjectID == results[1].ObjectID {
			t.Error("Expected different ObjectIDs for different symbols")
		}

		// Verify IDs are compact (should be much shorter than hex)
		if len(results[0].ObjectID) > 12 {
			t.Errorf("ObjectID too long: %s (len=%d), expected < 12 chars", results[0].ObjectID, len(results[0].ObjectID))
		}

		// Verify IDs only contain valid characters (A-Za-z0-9_)
		for i, result := range results {
			for _, c := range result.ObjectID {
				valid := (c >= 'A' && c <= 'Z') ||
					(c >= 'a' && c <= 'z') ||
					(c >= '0' && c <= '9') ||
					c == '_'
				if !valid {
					t.Errorf("Result %d: Invalid character '%c' in ObjectID: %s", i, c, result.ObjectID)
				}
			}
		}

		t.Logf("Generated ObjectIDs: %s, %s", results[0].ObjectID, results[1].ObjectID)
	})

	t.Run("skips results without relational data", func(t *testing.T) {
		results := []StandardResult{
			{
				Result: GrepResult{
					FileID: types.FileID(123),
					Path:   "test.go",
				},
				RelationalData: nil, // No relational data
			},
		}

		PopulateDenseObjectIDs(results)

		if results[0].ObjectID != "" {
			t.Errorf("Expected empty ObjectID for result without relational data, got: %s", results[0].ObjectID)
		}
	})

	t.Run("skips results with zero local symbol ID", func(t *testing.T) {
		results := []StandardResult{
			makeTestResult(types.FileID(123), 0), // Zero local symbol ID
		}

		PopulateDenseObjectIDs(results)

		if results[0].ObjectID != "" {
			t.Errorf("Expected empty ObjectID for zero symbol ID, got: %s", results[0].ObjectID)
		}
	})

	t.Run("generates unique IDs for different file IDs", func(t *testing.T) {
		results := []StandardResult{
			makeTestResult(types.FileID(100), 50),
			makeTestResult(types.FileID(200), 50), // Same local ID, different file
		}

		PopulateDenseObjectIDs(results)

		if results[0].ObjectID == results[1].ObjectID {
			t.Error("Expected different ObjectIDs for different file IDs")
		}

		t.Logf("Different files, same local ID: %s vs %s", results[0].ObjectID, results[1].ObjectID)
	})

	t.Run("generates unique IDs for different local symbol IDs", func(t *testing.T) {
		results := []StandardResult{
			makeTestResult(types.FileID(100), 50),
			makeTestResult(types.FileID(100), 60), // Same file, different local ID
		}

		PopulateDenseObjectIDs(results)

		if results[0].ObjectID == results[1].ObjectID {
			t.Error("Expected different ObjectIDs for different local symbol IDs")
		}

		t.Logf("Same file, different local IDs: %s vs %s", results[0].ObjectID, results[1].ObjectID)
	})

	t.Run("consistent encoding for same inputs", func(t *testing.T) {
		// Test that same inputs produce same output consistently
		fileID := types.FileID(12345)
		localSymbolID := uint32(67890)

		results1 := []StandardResult{makeTestResult(fileID, localSymbolID)}
		results2 := []StandardResult{makeTestResult(fileID, localSymbolID)}

		PopulateDenseObjectIDs(results1)
		PopulateDenseObjectIDs(results2)

		if results1[0].ObjectID != results2[0].ObjectID {
			t.Errorf("Inconsistent encoding: got %s and %s for same input",
				results1[0].ObjectID, results2[0].ObjectID)
		}

		t.Logf("Consistent encoding: FileID=%d, LocalSymbolID=%d -> %s",
			fileID, localSymbolID, results1[0].ObjectID)
	})

	t.Run("compact encoding efficiency", func(t *testing.T) {
		// Test various ID ranges to ensure compact encoding
		testCases := []struct {
			fileID        types.FileID
			localSymbolID uint32
			maxLen        int
			name          string
		}{
			{1, 1, 7, "tiny IDs"},
			{100, 200, 8, "small IDs"},
			{1000, 5000, 9, "medium IDs"},
			{65535, 65535, 10, "max uint16"},
			{1000000, 1000000, 11, "large IDs"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				results := []StandardResult{
					makeTestResult(tc.fileID, tc.localSymbolID),
				}

				PopulateDenseObjectIDs(results)

				if len(results[0].ObjectID) > tc.maxLen {
					t.Errorf("ObjectID too long: %s (len=%d), expected <= %d",
						results[0].ObjectID, len(results[0].ObjectID), tc.maxLen)
				}

				t.Logf("FileID=%d, LocalSymbolID=%d -> %s (len=%d)",
					tc.fileID, tc.localSymbolID, results[0].ObjectID, len(results[0].ObjectID))
			})
		}
	})
}

func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()

	// Test that defaults are reasonable
	if opts.MaxResults <= 0 {
		t.Error("Expected positive MaxResults")
	}

	if opts.MaxContextLines < 0 {
		t.Error("Expected non-negative MaxContextLines")
	}

	// Test that IncludeObjectIDs is set properly
	t.Logf("Default IncludeObjectIDs: %v", opts.IncludeObjectIDs)
}

func BenchmarkPopulateDenseObjectIDs(b *testing.B) {
	// Create a realistic set of results
	results := make([]StandardResult, 100)
	for i := range results {
		results[i] = makeTestResult(types.FileID(i+1), uint32(i+1))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Copy results to avoid mutations affecting timing
		testResults := make([]StandardResult, len(results))
		copy(testResults, results)
		PopulateDenseObjectIDs(testResults)
	}
}
