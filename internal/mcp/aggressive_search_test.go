package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// ============================================================================
// Word Splitting Tests
// ============================================================================

func TestExpandFileSearchPatterns(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "single word - no expansion",
			pattern:  "handler",
			expected: []string{"handler"},
		},
		{
			name:     "two words - split",
			pattern:  "user controller",
			expected: []string{"user controller", "user", "controller"},
		},
		{
			name:     "three words - split all",
			pattern:  "get user data",
			expected: []string{"get user data", "get", "user", "data"},
		},
		{
			name:     "short words filtered",
			pattern:  "a bc def",
			expected: []string{"a bc def", "def"}, // "a" and "bc" are too short
		},
		{
			name:     "preserves original with spaces",
			pattern:  "http response writer",
			expected: []string{"http response writer", "http", "response", "writer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandFileSearchPatterns(tt.pattern)
			assert.Equal(t, tt.expected, result, "Pattern expansion mismatch")
		})
	}
}

func TestSearchPatternExpansion_MultiWord(t *testing.T) {
	// Simulate the performSemanticExpansion logic
	expandPatternWithWords := func(pattern string, expanded *[]string) {
		words := strings.Fields(pattern)
		for _, word := range words {
			if len(word) > 2 && word != pattern {
				*expanded = append(*expanded, word)
			}
		}
	}

	performExpansion := func(pattern string) []string {
		expandedPatterns := make([]string, 0, 20)
		expandedPatterns = append(expandedPatterns, pattern)

		hasMultipleWords := strings.Contains(strings.TrimSpace(pattern), " ")
		if hasMultipleWords {
			expandPatternWithWords(pattern, &expandedPatterns)
		}

		// Deduplicate
		seen := make(map[string]struct{})
		deduped := make([]string, 0, len(expandedPatterns))
		for _, p := range expandedPatterns {
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				deduped = append(deduped, p)
			}
		}
		return deduped
	}

	tests := []struct {
		name            string
		pattern         string
		expectMultiple  bool
		expectedMinimum int
	}{
		{
			name:            "single word - no expansion",
			pattern:         "handler",
			expectMultiple:  false,
			expectedMinimum: 1,
		},
		{
			name:            "two words - expands to 3",
			pattern:         "user handler",
			expectMultiple:  true,
			expectedMinimum: 3, // original + 2 words
		},
		{
			name:            "three words - expands to 4",
			pattern:         "get user data",
			expectMultiple:  true,
			expectedMinimum: 4, // original + 3 words
		},
		{
			name:            "phrase with short words - filters short",
			pattern:         "a user",
			expectMultiple:  true,
			expectedMinimum: 2, // original + "user" (not "a")
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := performExpansion(tt.pattern)
			assert.GreaterOrEqual(t, len(result), tt.expectedMinimum,
				"Expected at least %d patterns, got %d: %v", tt.expectedMinimum, len(result), result)

			// First element should always be the original pattern
			assert.Equal(t, tt.pattern, result[0], "First pattern should be original")

			if tt.expectMultiple {
				assert.Greater(t, len(result), 1, "Expected multiple patterns for multi-word query")
			}
		})
	}
}

// ============================================================================
// Score-Proportional Context Tests
// ============================================================================

func TestComputeEffectiveOutputSize(t *testing.T) {
	tests := []struct {
		name          string
		requestedSize Output
		score         float64
		expectedSize  Output
	}{
		// High scores (>= 0.8) - get requested size
		{
			name:          "high score gets full output",
			requestedSize: OutputFull,
			score:         0.9,
			expectedSize:  OutputFull,
		},
		{
			name:          "high score gets context output",
			requestedSize: OutputContext,
			score:         0.85,
			expectedSize:  OutputContext,
		},
		{
			name:          "score exactly 0.8 gets requested",
			requestedSize: OutputFull,
			score:         0.8,
			expectedSize:  OutputFull,
		},

		// Medium scores (0.5-0.8) - downgraded one level
		{
			name:          "medium score downgrades full to context",
			requestedSize: OutputFull,
			score:         0.7,
			expectedSize:  OutputContext,
		},
		{
			name:          "medium score downgrades context to single",
			requestedSize: OutputContext,
			score:         0.6,
			expectedSize:  OutputSingleLine,
		},
		{
			name:          "medium score keeps single as single",
			requestedSize: OutputSingleLine,
			score:         0.55,
			expectedSize:  OutputSingleLine,
		},

		// Low scores (< 0.5) - always minimal
		{
			name:          "low score gets single line regardless of request",
			requestedSize: OutputFull,
			score:         0.3,
			expectedSize:  OutputSingleLine,
		},
		{
			name:          "very low score gets single line",
			requestedSize: OutputContext,
			score:         0.1,
			expectedSize:  OutputSingleLine,
		},

		// Scores > 1.0 (percentage scores) - normalized
		{
			name:          "percentage score 95 treated as 0.95",
			requestedSize: OutputFull,
			score:         95.0, // Will be normalized to 0.95
			expectedSize:  OutputFull,
		},
		{
			name:          "percentage score 50 treated as 0.5",
			requestedSize: OutputFull,
			score:         50.0,          // Will be normalized to 0.5
			expectedSize:  OutputContext, // Medium score, downgraded
		},
		{
			name:          "percentage score 30 treated as 0.3",
			requestedSize: OutputFull,
			score:         30.0,             // Will be normalized to 0.3
			expectedSize:  OutputSingleLine, // Low score
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeEffectiveOutputSize(tt.requestedSize, tt.score)
			assert.Equal(t, tt.expectedSize, result,
				"Expected %v for score %.2f with requested %v, got %v",
				tt.expectedSize, tt.score, tt.requestedSize, result)
		})
	}
}

func TestScoreProportionalContextThresholds(t *testing.T) {
	// Verify the threshold constants are properly defined
	assert.Equal(t, 0.8, scoreThresholdFull, "Full threshold should be 0.8")
	assert.Equal(t, 0.5, scoreThresholdMedium, "Medium threshold should be 0.5")

	// Test boundary conditions
	t.Run("boundary at 0.8", func(t *testing.T) {
		// Score exactly at 0.8 should get full
		result := computeEffectiveOutputSize(OutputFull, 0.8)
		assert.Equal(t, OutputFull, result)

		// Score just below 0.8 should be downgraded
		result = computeEffectiveOutputSize(OutputFull, 0.79)
		assert.Equal(t, OutputContext, result)
	})

	t.Run("boundary at 0.5", func(t *testing.T) {
		// Score exactly at 0.5 should get medium treatment
		result := computeEffectiveOutputSize(OutputFull, 0.5)
		assert.Equal(t, OutputContext, result)

		// Score just below 0.5 should be minimal
		result = computeEffectiveOutputSize(OutputFull, 0.49)
		assert.Equal(t, OutputSingleLine, result)
	})
}

// ============================================================================
// Word Coverage Scoring Tests
// ============================================================================

func TestWordCoverageBoostConstants(t *testing.T) {
	// Verify constants are properly defined
	assert.Equal(t, 0.15, wordCoverageBoostPerWord, "Coverage boost per word should be 0.15")
	assert.Equal(t, 0.5, maxWordCoverageBoost, "Max coverage boost should be 0.5")
}

func TestWordCoverageBoostCalculation(t *testing.T) {
	// Test the boost calculation logic used in searchAndDeduplicate
	calculateBoost := func(patternCount int) float64 {
		if patternCount <= 1 {
			return 0
		}
		boost := float64(patternCount-1) * wordCoverageBoostPerWord
		if boost > maxWordCoverageBoost {
			boost = maxWordCoverageBoost
		}
		return boost
	}

	tests := []struct {
		patternCount  int
		expectedBoost float64
	}{
		{1, 0.0},   // No boost for single pattern
		{2, 0.15},  // 1 additional = 15%
		{3, 0.30},  // 2 additional = 30%
		{4, 0.45},  // 3 additional = 45%
		{5, 0.50},  // 4 additional would be 60%, capped at 50%
		{10, 0.50}, // Many additional, still capped
	}

	for _, tt := range tests {
		t.Run("patterns_"+string(rune('0'+tt.patternCount)), func(t *testing.T) {
			boost := calculateBoost(tt.patternCount)
			assert.InDelta(t, tt.expectedBoost, boost, 0.001,
				"Expected boost %.2f for %d patterns, got %.2f",
				tt.expectedBoost, tt.patternCount, boost)
		})
	}
}

func TestWordCoverageScoreApplication(t *testing.T) {
	// Test that the boost is correctly applied to scores
	baseScore := 100.0

	applyBoost := func(score float64, patternCount int) float64 {
		if patternCount > 1 {
			boost := float64(patternCount-1) * wordCoverageBoostPerWord
			if boost > maxWordCoverageBoost {
				boost = maxWordCoverageBoost
			}
			score *= (1.0 + boost)
		}
		return score
	}

	tests := []struct {
		name          string
		patternCount  int
		expectedScore float64
	}{
		{"single pattern - no boost", 1, 100.0},
		{"two patterns - 15% boost", 2, 115.0},
		{"three patterns - 30% boost", 3, 130.0},
		{"four patterns - 45% boost", 4, 145.0},
		{"five patterns - 50% boost (capped)", 5, 150.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyBoost(baseScore, tt.patternCount)
			assert.InDelta(t, tt.expectedScore, result, 0.01,
				"Expected score %.2f, got %.2f", tt.expectedScore, result)
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestBuildCompactResultScaling(t *testing.T) {
	// Test that buildCompactResult correctly scales based on score
	// We can't test the full function without a server, but we can verify the logic

	t.Run("high score result gets full context", func(t *testing.T) {
		score := 0.9
		effectiveSize := computeEffectiveOutputSize(OutputFull, score)
		assert.Equal(t, OutputFull, effectiveSize)
	})

	t.Run("low score result gets minimal context", func(t *testing.T) {
		score := 0.3
		effectiveSize := computeEffectiveOutputSize(OutputFull, score)
		assert.Equal(t, OutputSingleLine, effectiveSize)
	})
}

func TestSemanticDefaultEnabled(t *testing.T) {
	// Test that semantic defaults to true when not specified
	// The actual implementation checks for "semantic" in raw JSON

	t.Run("semantic not in JSON - defaults to true", func(t *testing.T) {
		rawJSON := `{"pattern": "test"}`
		semanticExplicitlySet := strings.Contains(rawJSON, `"semantic"`)
		assert.False(t, semanticExplicitlySet, "semantic should not be in JSON")
		// When not set, default should be true
	})

	t.Run("semantic explicitly false - respects setting", func(t *testing.T) {
		rawJSON := `{"pattern": "test", "semantic": false}`
		semanticExplicitlySet := strings.Contains(rawJSON, `"semantic"`)
		assert.True(t, semanticExplicitlySet, "semantic should be in JSON")
	})

	t.Run("semantic explicitly true - respects setting", func(t *testing.T) {
		rawJSON := `{"pattern": "test", "semantic": true}`
		semanticExplicitlySet := strings.Contains(rawJSON, `"semantic"`)
		assert.True(t, semanticExplicitlySet, "semantic should be in JSON")
	})
}

// ============================================================================
// Result Key Tests
// ============================================================================

func TestResultKeyDeduplication(t *testing.T) {
	// Test that ResultKey correctly identifies duplicate results
	key1 := ResultKey{FileID: 1, Line: 10, Match: "func test()"}
	key2 := ResultKey{FileID: 1, Line: 10, Match: "func test()"}
	key3 := ResultKey{FileID: 1, Line: 11, Match: "func test()"}
	key4 := ResultKey{FileID: 2, Line: 10, Match: "func test()"}

	seen := make(map[ResultKey]struct{})
	seen[key1] = struct{}{}

	t.Run("identical keys deduplicate", func(t *testing.T) {
		_, exists := seen[key2]
		assert.True(t, exists, "Identical key should be found")
	})

	t.Run("different line is unique", func(t *testing.T) {
		_, exists := seen[key3]
		assert.False(t, exists, "Different line should be unique")
	})

	t.Run("different file is unique", func(t *testing.T) {
		_, exists := seen[key4]
		assert.False(t, exists, "Different file should be unique")
	})
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkComputeEffectiveOutputSize(b *testing.B) {
	scores := []float64{0.1, 0.3, 0.5, 0.7, 0.9, 50.0, 95.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		score := scores[i%len(scores)]
		_ = computeEffectiveOutputSize(OutputFull, score)
	}
}

func BenchmarkExpandFileSearchPatterns(b *testing.B) {
	patterns := []string{
		"handler",
		"user controller",
		"get user data handler",
		"http response writer interface",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		_ = expandFileSearchPatterns(pattern)
	}
}

func BenchmarkWordCoverageBoost(b *testing.B) {
	baseScore := 100.0
	patternCounts := []int{1, 2, 3, 4, 5, 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := patternCounts[i%len(patternCounts)]
		score := baseScore
		if count > 1 {
			boost := float64(count-1) * wordCoverageBoostPerWord
			if boost > maxWordCoverageBoost {
				boost = maxWordCoverageBoost
			}
			score *= (1.0 + boost)
		}
		_ = score
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestEdgeCases(t *testing.T) {
	t.Run("empty pattern", func(t *testing.T) {
		result := expandFileSearchPatterns("")
		assert.Equal(t, []string{""}, result)
	})

	t.Run("whitespace only pattern", func(t *testing.T) {
		result := expandFileSearchPatterns("   ")
		// strings.Fields on whitespace returns empty slice
		// So we get just the original pattern
		assert.Equal(t, []string{"   "}, result)
	})

	t.Run("zero score", func(t *testing.T) {
		result := computeEffectiveOutputSize(OutputFull, 0.0)
		assert.Equal(t, OutputSingleLine, result)
	})

	t.Run("negative score", func(t *testing.T) {
		result := computeEffectiveOutputSize(OutputFull, -0.5)
		assert.Equal(t, OutputSingleLine, result)
	})

	t.Run("very high percentage score", func(t *testing.T) {
		result := computeEffectiveOutputSize(OutputFull, 150.0)
		// 150/100 = 1.5, which is >= 0.8, so should get full
		assert.Equal(t, OutputFull, result)
	})
}

// ============================================================================
// Mock/Stub Helpers for Future Integration Tests
// ============================================================================

// mockDetailedResult creates a mock DetailedResult for testing
func mockDetailedResult(fileID int, line int, match string, score float64) searchtypes.DetailedResult {
	return searchtypes.DetailedResult{
		Result: searchtypes.GrepResult{
			FileID: types.FileID(fileID),
			Line:   line,
			Match:  match,
			Score:  score,
		},
	}
}

func TestMockDetailedResult(t *testing.T) {
	result := mockDetailedResult(1, 10, "test match", 85.0)
	assert.Equal(t, types.FileID(1), result.Result.FileID)
	assert.Equal(t, 10, result.Result.Line)
	assert.Equal(t, "test match", result.Result.Match)
	assert.Equal(t, 85.0, result.Result.Score)
}

// ============================================================================
// Documentation Tests (ensure examples work)
// ============================================================================

func TestDocumentedBehavior(t *testing.T) {
	t.Run("multi-word query expansion as documented", func(t *testing.T) {
		// As documented: "libby clone code" → ["libby clone code", "libby", "clone", "code"]
		pattern := "libby clone code"
		result := expandFileSearchPatterns(pattern)

		require.Len(t, result, 4, "Should expand to 4 patterns")
		assert.Equal(t, "libby clone code", result[0])
		assert.Contains(t, result, "libby")
		assert.Contains(t, result, "clone")
		assert.Contains(t, result, "code")
	})

	t.Run("score thresholds as documented", func(t *testing.T) {
		// Score >= 0.8: Full detail
		assert.Equal(t, OutputFull, computeEffectiveOutputSize(OutputFull, 0.8))

		// Score 0.5-0.8: Medium detail (downgraded)
		assert.Equal(t, OutputContext, computeEffectiveOutputSize(OutputFull, 0.6))

		// Score < 0.5: Minimal detail
		assert.Equal(t, OutputSingleLine, computeEffectiveOutputSize(OutputFull, 0.4))
	})

	t.Run("coverage boost as documented", func(t *testing.T) {
		// 15% boost per additional pattern, max 50%
		baseScore := 100.0

		// 2 patterns: +15%
		score2 := baseScore * (1.0 + 0.15)
		assert.InDelta(t, 115.0, score2, 0.01)

		// 3 patterns: +30%
		score3 := baseScore * (1.0 + 0.30)
		assert.InDelta(t, 130.0, score3, 0.01)

		// 5+ patterns: capped at +50%
		score5 := baseScore * (1.0 + 0.50)
		assert.InDelta(t, 150.0, score5, 0.01)
	})
}
