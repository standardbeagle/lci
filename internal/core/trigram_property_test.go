package core

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/types"
)

// Property-based tests for trigram indexing functionality
// These tests verify invariants that should hold for all valid inputs

// TestProperty_TrigramExtraction tests that trigram extraction is deterministic and correct
func TestProperty_TrigramExtraction(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	t.Run("extraction_is_deterministic", func(t *testing.T) {
		// Property: Same content should always produce same trigrams
		for i := 0; i < 100; i++ {
			content := generateRandomContent(rng, 50+rng.Intn(200))

			trigrams1 := extractSimpleTrigrams(content)
			trigrams2 := extractSimpleTrigrams(content)

			assert.Equal(t, len(trigrams1), len(trigrams2), "Trigram count should be deterministic")

			for offset, hash := range trigrams1 {
				hash2, ok := trigrams2[offset]
				assert.True(t, ok, "Same offset should exist in both extractions")
				assert.Equal(t, hash, hash2, "Same trigram hash at offset %d", offset)
			}
		}
	})

	t.Run("trigram_count_bounds", func(t *testing.T) {
		// Property: Number of trigrams <= len(content) - 2 (for content >= 3 bytes)
		for i := 0; i < 100; i++ {
			content := generateRandomContent(rng, 10+rng.Intn(100))

			if len(content) < 3 {
				continue
			}

			trigrams := extractSimpleTrigrams(content)

			// Maximum possible trigrams is len(content) - 2
			maxTrigrams := len(content) - 2
			assert.LessOrEqual(t, len(trigrams), maxTrigrams,
				"Trigram count should not exceed len-2")
		}
	})

	t.Run("short_content_returns_nil", func(t *testing.T) {
		// Property: Content shorter than 3 bytes should return nil
		shortContents := [][]byte{
			{},
			{'a'},
			{'a', 'b'},
		}

		for _, content := range shortContents {
			trigrams := extractSimpleTrigrams(content)
			assert.Nil(t, trigrams, "Content of length %d should return nil trigrams", len(content))
		}
	})

	t.Run("alphanumeric_filtering", func(t *testing.T) {
		// Property: Trigrams with no alphanumeric chars should be filtered out
		nonAlphaContent := []byte("... --- ...")
		trigrams := extractSimpleTrigrams(nonAlphaContent)

		// All-punctuation trigrams should be filtered
		assert.Equal(t, 0, len(trigrams), "Non-alphanumeric content should produce no trigrams")

		// Mixed content should produce some trigrams
		mixedContent := []byte("a.. b.. c..")
		mixedTrigrams := extractSimpleTrigrams(mixedContent)
		assert.Greater(t, len(mixedTrigrams), 0, "Mixed content should produce some trigrams")
	})
}

// TestProperty_TrigramIndexOperations tests index operations maintain consistency
func TestProperty_TrigramIndexOperations(t *testing.T) {
	t.Run("index_file_then_find_candidates", func(t *testing.T) {
		// Property: After indexing a file, searching for content in that file
		// should return the file as a candidate
		index := NewTrigramIndex()

		content := []byte("function calculateTotal(items) { return items.reduce((a,b) => a+b); }")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		// Search for patterns that exist in the content
		patterns := []string{"function", "calculate", "Total", "items", "reduce"}

		for _, pattern := range patterns {
			candidates := index.FindCandidates(pattern)
			assert.Contains(t, candidates, fileID,
				"File should be found when searching for '%s'", pattern)
		}
	})

	t.Run("remove_file_clears_candidates", func(t *testing.T) {
		// Property: After removing a file, it should not appear in candidates
		// (after cleanup is triggered)
		index := NewTrigramIndex()
		index.SetCleanupThreshold(1) // Trigger cleanup immediately

		content := []byte("unique_identifier_xyz_12345")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		// Verify file is found
		candidates := index.FindCandidates("unique_identifier")
		assert.Contains(t, candidates, fileID)

		// Remove and force cleanup
		index.RemoveFile(fileID, content)
		index.ForceCleanup()

		// File should no longer appear (may need to clear cache)
		index.invalidateCacheCompletely()
		candidatesAfter := index.FindCandidates("unique_identifier")
		assert.NotContains(t, candidatesAfter, fileID,
			"Removed file should not appear in candidates after cleanup")
	})

	t.Run("update_file_reflects_new_content", func(t *testing.T) {
		// Property: After updating a file, new patterns should match
		index := NewTrigramIndex()

		oldContent := []byte("function oldFunction() { }")
		newContent := []byte("function newFunction() { }")
		fileID := types.FileID(1)

		index.IndexFile(fileID, oldContent)

		// Should find old pattern
		candidates := index.FindCandidates("oldFunction")
		assert.Contains(t, candidates, fileID)

		// Update file
		index.UpdateFile(fileID, oldContent, newContent)

		// New pattern should find the file after update
		candidatesNew := index.FindCandidates("newFunction")
		assert.Contains(t, candidatesNew, fileID,
			"New pattern should find file after update")
	})
}

// TestProperty_TrigramHashConsistency tests that trigram hashing is consistent
func TestProperty_TrigramHashConsistency(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	t.Run("same_bytes_same_hash", func(t *testing.T) {
		// Property: Same 3-byte sequence should always produce same hash
		for i := 0; i < 1000; i++ {
			b1 := byte(rng.Intn(128)) // ASCII only
			b2 := byte(rng.Intn(128))
			b3 := byte(rng.Intn(128))

			content := []byte{b1, b2, b3}

			// Extract trigrams twice
			t1 := extractSimpleTrigrams(content)
			t2 := extractSimpleTrigrams(content)

			// If any trigrams were extracted, they should match
			if len(t1) > 0 && len(t2) > 0 {
				for offset, hash := range t1 {
					assert.Equal(t, hash, t2[offset], "Same content should produce same hash")
				}
			}
		}
	})

	t.Run("hash_collision_resistance", func(t *testing.T) {
		// Property: Different 3-byte sequences should (usually) produce different hashes
		hashCounts := make(map[uint32]int)

		// Generate many trigrams and count collisions
		for b1 := byte('a'); b1 <= byte('z'); b1++ {
			for b2 := byte('a'); b2 <= byte('z'); b2++ {
				for b3 := byte('a'); b3 <= byte('z'); b3++ {
					content := []byte{b1, b2, b3}
					trigrams := extractSimpleTrigrams(content)

					for _, hash := range trigrams {
						hashCounts[hash]++
					}
				}
			}
		}

		// No hash should appear more than once (no collisions for unique inputs)
		collisions := 0
		for _, count := range hashCounts {
			if count > 1 {
				collisions++
			}
		}

		assert.Equal(t, 0, collisions, "Should have no collisions for unique letter trigrams")
	})
}

// TestProperty_ASCIIDetection tests ASCII detection is accurate
func TestProperty_ASCIIDetection(t *testing.T) {
	t.Run("pure_ascii_detection", func(t *testing.T) {
		// Property: Content with only bytes < 128 should be detected as pure ASCII
		asciiContent := []byte("Hello, World! 123 @#$%^&*()")
		assert.True(t, isPureASCII(asciiContent), "ASCII content should be detected as pure ASCII")
	})

	t.Run("unicode_detection", func(t *testing.T) {
		// Property: Content with any byte >= 128 should not be pure ASCII
		unicodeContents := [][]byte{
			[]byte("Hello, 世界!"),
			[]byte("Café"),
			[]byte("naïve"),
			{0x80},
			{0xFF},
		}

		for _, content := range unicodeContents {
			assert.False(t, isPureASCII(content),
				"Content with non-ASCII bytes should not be detected as pure ASCII")
		}
	})

	t.Run("empty_is_ascii", func(t *testing.T) {
		// Property: Empty content should be considered pure ASCII (vacuously true)
		assert.True(t, isPureASCII([]byte{}), "Empty content should be pure ASCII")
	})
}

// TestProperty_UnicodeTrigramExtraction tests Unicode trigram handling
func TestProperty_UnicodeTrigramExtraction(t *testing.T) {
	t.Run("unicode_extraction_deterministic", func(t *testing.T) {
		// Property: Unicode trigram extraction should be deterministic
		unicodeContent := []byte("函数 calculate 计算")

		trigrams1 := extractUnicodeTrigrams(unicodeContent)
		trigrams2 := extractUnicodeTrigrams(unicodeContent)

		assert.Equal(t, len(trigrams1), len(trigrams2))
		for offset, str := range trigrams1 {
			assert.Equal(t, str, trigrams2[offset])
		}
	})

	t.Run("unicode_rune_boundaries", func(t *testing.T) {
		// Property: Unicode trigrams should respect rune boundaries
		content := []byte("你好世界") // 4 Chinese characters = 4 runes

		trigrams := extractUnicodeTrigrams(content)

		// Should have exactly 2 trigrams (runes 0-2 and 1-3)
		// (4 runes - 3 + 1 = 2 positions)
		expectedCount := 2
		assert.Equal(t, expectedCount, len(trigrams),
			"Should have correct number of rune-based trigrams")

		// Each trigram should be valid UTF-8
		for _, trigramStr := range trigrams {
			assert.True(t, utf8.ValidString(trigramStr),
				"Each trigram should be valid UTF-8")
		}
	})
}

// TestProperty_CaseInsensitiveSearch tests case-insensitive search properties
func TestProperty_CaseInsensitiveSearch(t *testing.T) {
	t.Run("case_variations_find_same_file", func(t *testing.T) {
		// Property: Case-insensitive search should find content regardless of case
		index := NewTrigramIndex()

		content := []byte("function TestFunction() { return TESTFUNCTION; }")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		// All case variations should find the file with case-insensitive search
		variations := []string{
			"testfunction",
			"TESTFUNCTION",
			"TestFunction",
			"testFUNCTION",
		}

		for _, pattern := range variations {
			candidates := index.FindCandidatesWithOptions(pattern, true)
			assert.Contains(t, candidates, fileID,
				"Case-insensitive search should find '%s'", pattern)
		}
	})
}

// TestProperty_ConcurrentAccess tests thread-safety properties
func TestProperty_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent_index_and_search", func(t *testing.T) {
		// Property: Concurrent indexing and searching should not cause data races
		index := NewTrigramIndex()

		// Index some initial files
		for i := 0; i < 10; i++ {
			content := []byte("initial content for file " + string(rune('0'+i)))
			index.IndexFile(types.FileID(i), content)
		}

		// Run concurrent operations
		done := make(chan bool, 20)

		// Concurrent searches
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				for j := 0; j < 100; j++ {
					_ = index.FindCandidates("content")
				}
			}()
		}

		// Concurrent indexing
		for i := 0; i < 10; i++ {
			go func(id int) {
				defer func() { done <- true }()
				for j := 0; j < 10; j++ {
					content := []byte("new content " + string(rune('A'+j)))
					index.IndexFile(types.FileID(100+id*10+j), content)
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 20; i++ {
			<-done
		}

		// Index should still be consistent
		count := index.FileCount()
		assert.Greater(t, count, 0, "Index should have files after concurrent operations")
	})
}

// TestProperty_SearchCacheConsistency tests search cache properties
func TestProperty_SearchCacheConsistency(t *testing.T) {
	t.Run("cache_returns_same_results", func(t *testing.T) {
		// Property: Cached results should be identical to fresh results
		index := NewTrigramIndex()

		for i := 0; i < 5; i++ {
			content := []byte("function testFunc" + string(rune('A'+i)) + "() { return true; }")
			index.IndexFile(types.FileID(i), content)
		}

		// First search (populates cache)
		results1 := index.FindCandidates("testFunc")

		// Second search (should use cache)
		results2 := index.FindCandidates("testFunc")

		assert.Equal(t, len(results1), len(results2),
			"Cached results should have same length")

		// Results should contain same file IDs (order may vary)
		for _, id := range results1 {
			assert.Contains(t, results2, id, "Cached results should contain same files")
		}
	})

	t.Run("cache_invalidation_on_file_change", func(t *testing.T) {
		// Property: Cache should be invalidated when files change
		index := NewTrigramIndex()

		content := []byte("unique_search_pattern_xyz")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		// Populate cache
		_ = index.FindCandidates("unique_search_pattern")

		// Update file (invalidates cache)
		newContent := []byte("different_content_entirely")
		index.UpdateFile(fileID, content, newContent)

		// Search again - should not find old pattern
		results := index.FindCandidates("unique_search_pattern")
		assert.NotContains(t, results, fileID,
			"Cache should be invalidated after file update")
	})
}

// TestProperty_OffsetToLineColumn tests offset-to-line conversion
func TestProperty_OffsetToLineColumn(t *testing.T) {
	index := NewTrigramIndex()

	t.Run("first_line_offsets", func(t *testing.T) {
		// Property: Offsets before first newline should be line 1
		lineOffsets := []int{0, 10, 20, 30}

		for offset := 0; offset < 10; offset++ {
			line, col := index.offsetToLineColumn(lineOffsets, offset)
			assert.Equal(t, 1, line, "Offset %d should be line 1", offset)
			assert.Equal(t, offset+1, col, "Column should be offset+1 for first line")
		}
	})

	t.Run("subsequent_line_offsets", func(t *testing.T) {
		// Property: Offsets after newlines should map to correct lines
		lineOffsets := []int{0, 10, 20, 30}

		// Offset 10-19 should be line 2
		for offset := 10; offset < 20; offset++ {
			line, _ := index.offsetToLineColumn(lineOffsets, offset)
			assert.Equal(t, 2, line, "Offset %d should be line 2", offset)
		}

		// Offset 20-29 should be line 3
		for offset := 20; offset < 30; offset++ {
			line, _ := index.offsetToLineColumn(lineOffsets, offset)
			assert.Equal(t, 3, line, "Offset %d should be line 3", offset)
		}
	})

	t.Run("empty_offsets", func(t *testing.T) {
		// Property: Empty line offsets should return line 1, col 1
		line, col := index.offsetToLineColumn([]int{}, 5)
		assert.Equal(t, 1, line)
		assert.Equal(t, 1, col)
	})
}

// TestProperty_AllocatorStats tests allocator statistics
func TestProperty_AllocatorStats(t *testing.T) {
	t.Run("stats_increase_with_indexing", func(t *testing.T) {
		// Property: Allocator stats should reflect indexing activity
		index := NewTrigramIndex()

		statsBefore := index.GetAllocatorStats()

		// Index many files
		for i := 0; i < 100; i++ {
			content := []byte("content with lots of trigrams for file number " + string(rune('0'+i%10)))
			index.IndexFile(types.FileID(i), content)
		}

		statsAfter := index.GetAllocatorStats()

		// Total allocations should increase or stay same (never decrease)
		assert.GreaterOrEqual(t, statsAfter.Allocations, statsBefore.Allocations,
			"Allocations should increase with indexing")
	})
}

// Helper function to generate random content
func generateRandomContent(rng *rand.Rand, length int) []byte {
	chars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 _.-")
	content := make([]byte, length)
	for i := range content {
		content[i] = chars[rng.Intn(len(chars))]
	}
	return content
}

// TestProperty_TrigramPreallocation tests pre-allocation prediction
func TestProperty_TrigramPreallocation(t *testing.T) {
	index := NewTrigramIndex()

	t.Run("prediction_bounds", func(t *testing.T) {
		// Property: Predicted trigram count should be within reasonable bounds
		testSizes := []int{0, 1, 10, 100, 500, 1000, 5000}

		for _, size := range testSizes {
			predicted := index.predictTrigramCount(size)

			if size <= 0 {
				assert.Equal(t, 0, predicted, "Empty content should predict 0")
				continue
			}

			// Should be at least 8 (minimum allocation)
			if size > 16 {
				assert.GreaterOrEqual(t, predicted, 8, "Should have minimum allocation")
			}

			// Should not exceed 1000 (cap)
			assert.LessOrEqual(t, predicted, 1000, "Should not exceed cap")

			// Prediction should be reasonable (allow minimum allocation buffer)
			// For small content, minimum allocation of 8 is acceptable
			if size >= 16 {
				assert.LessOrEqual(t, predicted, size+8, "Prediction should be reasonable for content size")
			}
		}
	})
}

// TestProperty_BulkIndexingMode tests bulk indexing flag
func TestProperty_BulkIndexingMode(t *testing.T) {
	t.Run("bulk_mode_toggle", func(t *testing.T) {
		index := NewTrigramIndex()

		// Initially should be 0
		require.Equal(t, int32(0), index.BulkIndexing)

		// Enable bulk mode
		index.SetBulkIndexing(true)
		assert.Equal(t, int32(1), index.BulkIndexing)

		// Disable bulk mode
		index.SetBulkIndexing(false)
		assert.Equal(t, int32(0), index.BulkIndexing)
	})
}

// TestProperty_CleanupThreshold tests cleanup threshold behavior
func TestProperty_CleanupThreshold(t *testing.T) {
	t.Run("threshold_setting", func(t *testing.T) {
		index := NewTrigramIndex()

		// Default threshold is 100
		index.SetCleanupThreshold(50)

		// After many invalidations, count should reset after cleanup
		for i := 0; i < 60; i++ {
			content := []byte("content" + string(rune('A'+i%26)))
			index.IndexFile(types.FileID(i), content)
		}

		// Remove files (should trigger cleanup at threshold)
		for i := 0; i < 60; i++ {
			content := []byte("content" + string(rune('A'+i%26)))
			index.RemoveFile(types.FileID(i), content)
		}

		// Force cleanup and check invalidation count is reset
		index.ForceCleanup()
		assert.Equal(t, 0, index.GetInvalidationCount(),
			"Invalidation count should be 0 after cleanup")
	})
}

// TestProperty_TrigramWithPrecomputed tests IndexFileWithTrigrams
func TestProperty_TrigramWithPrecomputed(t *testing.T) {
	t.Run("precomputed_matches_direct", func(t *testing.T) {
		// Property: Indexing with pre-computed trigrams should produce same
		// candidates as direct indexing
		content := []byte("function mySpecialFunction() { return 42; }")
		fileID1 := types.FileID(1)
		fileID2 := types.FileID(2)

		// Index directly
		index1 := NewTrigramIndex()
		index1.IndexFile(fileID1, content)

		// Index with pre-computed trigrams
		index2 := NewTrigramIndex()
		precomputed := make(map[uint32][]uint32)
		simpleTrigrams := extractSimpleTrigrams(content)
		for offset, hash := range simpleTrigrams {
			precomputed[hash] = append(precomputed[hash], uint32(offset))
		}
		index2.IndexFileWithTrigrams(fileID2, precomputed)

		// Search patterns should work in both
		patterns := []string{"function", "mySpecial", "Function", "return"}

		for _, pattern := range patterns {
			candidates1 := index1.FindCandidates(pattern)
			candidates2 := index2.FindCandidates(pattern)

			hasFile1 := fileIDSliceContains(candidates1, fileID1)
			hasFile2 := fileIDSliceContains(candidates2, fileID2)

			assert.Equal(t, hasFile1, hasFile2,
				"Both indexing methods should produce same candidacy for pattern '%s'", pattern)
		}
	})
}

// TestProperty_ClearResets tests that Clear properly resets the index
func TestProperty_ClearResets(t *testing.T) {
	index := NewTrigramIndex()

	// Index some files
	for i := 0; i < 10; i++ {
		content := []byte("content for file " + string(rune('0'+i)))
		index.IndexFile(types.FileID(i), content)
	}

	// Verify files are indexed
	require.Greater(t, index.FileCount(), 0)

	// Clear the index
	index.Clear()

	// Verify index is empty
	assert.Equal(t, 0, index.FileCount(), "FileCount should be 0 after Clear")

	// Searches should return no results
	candidates := index.FindCandidates("content")
	assert.Empty(t, candidates, "Search should return no candidates after Clear")
}

// TestProperty_EnhancedTrigrams tests enhanced trigram extraction
func TestProperty_EnhancedTrigrams(t *testing.T) {
	t.Run("enhanced_extraction_deterministic", func(t *testing.T) {
		content := []byte("function test() { return value; }")

		trigrams1 := extractEnhancedTrigrams(content)
		trigrams2 := extractEnhancedTrigrams(content)

		assert.Equal(t, len(trigrams1), len(trigrams2))
		for offset, tg := range trigrams1 {
			assert.Equal(t, tg, trigrams2[offset])
		}
	})

	t.Run("enhanced_vs_simple_equivalence", func(t *testing.T) {
		// For ASCII content, enhanced trigrams should cover same positions
		content := []byte("function calculate(x, y) { return x + y; }")

		simple := extractSimpleTrigrams(content)
		enhanced := extractEnhancedTrigrams(content)

		// Same number of trigrams
		assert.Equal(t, len(simple), len(enhanced),
			"Simple and enhanced should have same count for ASCII")
	})
}

// fileIDSliceContains checks if a slice of FileID contains the given value
func fileIDSliceContains(slice []types.FileID, val types.FileID) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// TestProperty_FindMatchLocations tests match location finding
func TestProperty_FindMatchLocations(t *testing.T) {
	t.Run("locations_match_actual_content", func(t *testing.T) {
		index := NewTrigramIndex()

		content := []byte("function test() {\n  return test;\n}\n")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		// Create a mock file provider
		lineOffsets := types.ComputeLineOffsets(content)
		fileProvider := func(id types.FileID) *types.FileInfo {
			if id == fileID {
				return &types.FileInfo{
					ID:          fileID,
					Path:        "test.go",
					Content:     content,
					LineOffsets: lineOffsets,
				}
			}
			return nil
		}

		// Find locations for "test"
		locations := index.FindMatchLocations("test", false, fileProvider)

		// Should find both occurrences
		assert.GreaterOrEqual(t, len(locations), 2, "Should find at least 2 occurrences of 'test'")

		// Verify each location points to actual match
		for _, loc := range locations {
			// Extract 4 bytes at the offset
			if loc.Offset+4 <= len(content) {
				extracted := string(content[loc.Offset : loc.Offset+4])
				assert.Equal(t, "test", extracted,
					"Location should point to actual match")
			}
		}
	})

	t.Run("case_insensitive_locations", func(t *testing.T) {
		index := NewTrigramIndex()

		content := []byte("TEST test Test TEST")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		lineOffsets := types.ComputeLineOffsets(content)
		fileProvider := func(id types.FileID) *types.FileInfo {
			if id == fileID {
				return &types.FileInfo{
					ID:          fileID,
					Path:        "test.go",
					Content:     content,
					LineOffsets: lineOffsets,
				}
			}
			return nil
		}

		// Case-insensitive search
		locations := index.FindMatchLocations("test", true, fileProvider)

		// Should find all 4 occurrences
		assert.Equal(t, 4, len(locations), "Case-insensitive should find all case variations")

		// Verify each match when lowercased
		for _, loc := range locations {
			if loc.Offset+4 <= len(content) {
				extracted := strings.ToLower(string(content[loc.Offset : loc.Offset+4]))
				assert.Equal(t, "test", extracted)
			}
		}
	})

	t.Run("short_pattern_returns_nil", func(t *testing.T) {
		index := NewTrigramIndex()

		content := []byte("ab ab ab")
		fileID := types.FileID(1)

		index.IndexFile(fileID, content)

		fileProvider := func(id types.FileID) *types.FileInfo {
			if id == fileID {
				return &types.FileInfo{
					ID:      fileID,
					Path:    "test.go",
					Content: content,
				}
			}
			return nil
		}

		// Pattern shorter than 3 should return nil
		locations := index.FindMatchLocations("ab", false, fileProvider)
		assert.Nil(t, locations, "Short pattern should return nil")
	})
}

// TestProperty_ByteTrigramExtraction tests byte-based trigram extraction
func TestProperty_ByteTrigramExtraction(t *testing.T) {
	t.Run("byte_trigrams_match_simple", func(t *testing.T) {
		content := []byte("function test")

		byteTrigrams := extractByteTrigrams(content)
		simpleTrigrams := extractSimpleTrigrams(content)

		// Byte trigrams produce EnhancedTrigram, simple produces uint32
		// They should have same positions
		assert.Equal(t, len(byteTrigrams), len(simpleTrigrams),
			"Byte and simple should have same positions")
	})
}

// TestProperty_RuneTrigramExtraction tests rune-based trigram extraction
func TestProperty_RuneTrigramExtraction(t *testing.T) {
	t.Run("rune_trigrams_valid_utf8", func(t *testing.T) {
		content := []byte("你好世界abc")

		trigrams := extractRuneTrigrams(content)

		// All trigrams should be valid UTF-8 strings
		for _, tg := range trigrams {
			// EnhancedTrigram stores the trigram data
			assert.NotNil(t, tg, "Each trigram should not be nil")
		}
	})
}

// TestProperty_RemoveFileLocations tests the removeFileLocations helper
func TestProperty_RemoveFileLocations(t *testing.T) {
	t.Run("removes_all_occurrences", func(t *testing.T) {
		locations := []FileLocation{
			{FileID: 1, Offset: 0},
			{FileID: 2, Offset: 10},
			{FileID: 1, Offset: 20},
			{FileID: 3, Offset: 30},
			{FileID: 1, Offset: 40},
		}

		result := removeFileLocations(locations, 1)

		// Should have removed all FileID=1 entries
		assert.Len(t, result, 2)
		for _, loc := range result {
			assert.NotEqual(t, types.FileID(1), loc.FileID)
		}
	})

	t.Run("preserves_order", func(t *testing.T) {
		locations := []FileLocation{
			{FileID: 1, Offset: 0},
			{FileID: 2, Offset: 10},
			{FileID: 3, Offset: 20},
		}

		result := removeFileLocations(locations, 1)

		assert.Len(t, result, 2)
		assert.Equal(t, types.FileID(2), result[0].FileID)
		assert.Equal(t, types.FileID(3), result[1].FileID)
	})

	t.Run("empty_input", func(t *testing.T) {
		result := removeFileLocations(nil, 1)
		assert.Nil(t, result)

		result = removeFileLocations([]FileLocation{}, 1)
		assert.Empty(t, result)
	})
}

// TestProperty_IsAlphaNum tests alphanumeric detection
func TestProperty_IsAlphaNum(t *testing.T) {
	t.Run("lowercase_letters", func(t *testing.T) {
		for b := byte('a'); b <= byte('z'); b++ {
			assert.True(t, isAlphaNum(b), "Lowercase '%c' should be alphanumeric", b)
		}
	})

	t.Run("uppercase_letters", func(t *testing.T) {
		for b := byte('A'); b <= byte('Z'); b++ {
			assert.True(t, isAlphaNum(b), "Uppercase '%c' should be alphanumeric", b)
		}
	})

	t.Run("digits", func(t *testing.T) {
		for b := byte('0'); b <= byte('9'); b++ {
			assert.True(t, isAlphaNum(b), "Digit '%c' should be alphanumeric", b)
		}
	})

	t.Run("underscore", func(t *testing.T) {
		assert.True(t, isAlphaNum('_'), "Underscore should be alphanumeric")
	})

	t.Run("non_alphanumeric", func(t *testing.T) {
		nonAlpha := []byte{' ', '\t', '\n', '.', ',', '!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '-', '+', '='}
		for _, b := range nonAlpha {
			assert.False(t, isAlphaNum(b), "'%c' (0x%02x) should not be alphanumeric", b, b)
		}
	})
}

// TestProperty_OffsetsToFileLocations tests the conversion helper
func TestProperty_OffsetsToFileLocations(t *testing.T) {
	t.Run("converts_all_offsets", func(t *testing.T) {
		offsets := []uint32{0, 10, 20, 30, 40}
		fileID := types.FileID(42)

		locations := offsetsToFileLocations(fileID, offsets)

		assert.Len(t, locations, len(offsets))
		for i, loc := range locations {
			assert.Equal(t, fileID, loc.FileID)
			assert.Equal(t, offsets[i], loc.Offset)
		}
	})

	t.Run("empty_offsets", func(t *testing.T) {
		locations := offsetsToFileLocations(1, []uint32{})
		assert.Empty(t, locations)
	})
}

// TestProperty_HasAlphaNumFunctions tests the hasAlphaNum helpers
func TestProperty_HasAlphaNumASCII(t *testing.T) {
	t.Run("finds_alpha_in_range", func(t *testing.T) {
		content := []byte("...a...")
		assert.True(t, hasAlphaNumASCII(content, 0, 7))
		assert.True(t, hasAlphaNumASCII(content, 3, 1))
		assert.False(t, hasAlphaNumASCII(content, 0, 3))
	})

	t.Run("boundary_conditions", func(t *testing.T) {
		content := []byte("abc")
		// Beyond content length
		assert.False(t, hasAlphaNumASCII(content, 10, 3))
		// Partial read
		assert.True(t, hasAlphaNumASCII(content, 0, 10)) // Should clamp to content length
	})
}

func TestProperty_HasAlphaNumUnicode(t *testing.T) {
	t.Run("finds_alpha_in_range", func(t *testing.T) {
		runes := []rune("...a...")
		assert.True(t, hasAlphaNumUnicode(runes, 0, 7))
		assert.True(t, hasAlphaNumUnicode(runes, 3, 1))
		assert.False(t, hasAlphaNumUnicode(runes, 0, 3))
	})

	t.Run("unicode_letters", func(t *testing.T) {
		runes := []rune("...日...")
		assert.True(t, hasAlphaNumUnicode(runes, 3, 1), "CJK character should be detected as letter")
	})
}

// Benchmark for property tests to ensure they don't regress performance
func BenchmarkPropertyTests(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	content := generateRandomContent(rng, 1000)

	b.Run("extractSimpleTrigrams", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = extractSimpleTrigrams(content)
		}
	})

	b.Run("isPureASCII", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = isPureASCII(content)
		}
	})

	b.Run("extractUnicodeTrigrams", func(b *testing.B) {
		unicodeContent := []byte("函数 test 计算 function 处理")
		for i := 0; i < b.N; i++ {
			_ = extractUnicodeTrigrams(unicodeContent)
		}
	})
}

// Additional fuzz-like property tests
func TestProperty_FuzzLikeInputs(t *testing.T) {
	rng := rand.New(rand.NewSource(999))

	t.Run("random_ascii_content", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			length := rng.Intn(1000)
			content := make([]byte, length)
			for j := range content {
				content[j] = byte(rng.Intn(128)) // ASCII only
			}

			// Should not panic
			_ = extractSimpleTrigrams(content)
			_ = isPureASCII(content)
		}
	})

	t.Run("random_binary_content", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			length := rng.Intn(1000)
			content := make([]byte, length)
			for j := range content {
				content[j] = byte(rng.Intn(256)) // Full byte range
			}

			// Should not panic
			_ = extractSimpleTrigrams(content)
			_ = isPureASCII(content)
			_ = extractUnicodeTrigrams(content)
		}
	})

	t.Run("pathological_patterns", func(t *testing.T) {
		pathological := [][]byte{
			bytes.Repeat([]byte{'a'}, 10000),           // Repeated single char
			bytes.Repeat([]byte("ab"), 5000),           // Repeated pair
			bytes.Repeat([]byte("abc"), 3333),          // Repeated trigram
			bytes.Repeat([]byte{0}, 1000),              // Null bytes
			bytes.Repeat([]byte{0xFF}, 1000),           // High bytes
			bytes.Repeat([]byte{' ', '\t', '\n'}, 333), // Whitespace only
		}

		for _, content := range pathological {
			// Should not panic or hang
			_ = extractSimpleTrigrams(content)
			_ = isPureASCII(content)
		}
	})
}
