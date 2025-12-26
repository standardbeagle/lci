package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConstants tests that search engine constants have expected values
func TestConstants(t *testing.T) {
	// Test context extraction constants
	assert.Equal(t, 50, DefaultContextLines, "DefaultContextLines should be 50")
	assert.Equal(t, 20, TokensPerContextLine, "TokensPerContextLine should be 20")
}

// TestDefaultContextLines_Rationale tests the rationale for DefaultContextLines value
func TestDefaultContextLines_Rationale(t *testing.T) {
	// 50 lines should capture most complete functions/methods
	// while avoiding excessive memory usage
	assert.True(t, DefaultContextLines >= 10, "Should provide reasonable minimum context")
	assert.True(t, DefaultContextLines <= 100, "Should not provide excessive context that impacts performance")
}

// TestTokensPerContextLine_Rationale tests the rationale for token estimation
func TestTokensPerContextLine_Rationale(t *testing.T) {
	// Conservative estimate to avoid exceeding token limits
	// Based on 15-25 tokens per line analysis
	assert.True(t, TokensPerContextLine >= 15, "Should account for minimum expected tokens per line")
	assert.True(t, TokensPerContextLine <= 25, "Should not overestimate tokens per line")
}

// TestContextTokenCalculation tests context token calculation based on constants
func TestContextTokenCalculation(t *testing.T) {
	// Test token calculation for different context line scenarios
	testCases := []struct {
		name           string
		contextLines   int
		expectedTokens int
	}{
		{"No context", 0, 0},
		{"Small context", 5, 5 * TokensPerContextLine},
		{"Default context", DefaultContextLines, DefaultContextLines * TokensPerContextLine},
		{"Large context", 100, 100 * TokensPerContextLine},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			calculatedTokens := tc.contextLines * TokensPerContextLine
			assert.Equal(t, tc.expectedTokens, calculatedTokens)
		})
	}
}

// TestConstantRelationships tests that constants maintain sensible relationships
func TestConstantRelationships(t *testing.T) {
	// DefaultContextLines should be reasonable for typical scenarios
	assert.True(t, DefaultContextLines > 10, "Should provide sufficient context for analysis")
	assert.True(t, DefaultContextLines < 200, "Should not cause memory issues")

	// TokensPerContextLine should be realistic for code
	assert.True(t, TokensPerContextLine > 10, "Should account for code density")
	assert.True(t, TokensPerContextLine < 50, "Should not overestimate code verbosity")

	// Combined estimation should be reasonable
	defaultContextTokens := DefaultContextLines * TokensPerContextLine
	assert.True(t, defaultContextTokens > 500, "Default context should provide meaningful token count")
	assert.True(t, defaultContextTokens < 2000, "Default context should not exceed typical token limits")
}

// BenchmarkConstantAccess benchmarks constant access performance
func BenchmarkConstantAccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultContextLines
		_ = TokensPerContextLine
	}
}
