package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTokenEstimator tests creating a new token estimator
func TestNewTokenEstimator(t *testing.T) {
	estimator := NewTokenEstimator()
	require.NotNil(t, estimator)
	assert.Equal(t, 4.0, estimator.averageCharsPerToken)
	assert.Equal(t, 1.2, estimator.jsonOverheadFactor)
}

// TestTokenEstimator_EstimateTokens tests token estimation with different inputs
func TestTokenEstimator_EstimateTokens(t *testing.T) {
	estimator := NewTokenEstimator()

	// Test with simple string
	tokens := estimator.EstimateTokens("hello world")
	assert.Greater(t, tokens, 0, "Should estimate positive tokens")
	assert.Less(t, tokens, 100, "Should estimate reasonable tokens for short string")

	// Test with struct
	testStruct := struct {
		Name  string
		Value int
	}{
		Name:  "test",
		Value: 42,
	}
	tokens = estimator.EstimateTokens(testStruct)
	assert.Greater(t, tokens, 0, "Should estimate positive tokens for struct")
}

// TestTokenEstimator_EstimateResultTokens tests token estimation for mock results
func TestTokenEstimator_EstimateResultTokens(t *testing.T) {
	estimator := NewTokenEstimator()

	// Create a mock result structure for testing
	mockResult := struct {
		Path    string   `json:"path"`
		Context []string `json:"context"`
	}{
		Path:    "test.go",
		Context: []string{"func main() {", "	fmt.Println(\"hello\")", "}"},
	}

	tokens := estimator.EstimateTokens(mockResult)
	assert.Greater(t, tokens, 0, "Should estimate positive tokens for result")
	assert.Greater(t, tokens, 10, "Should estimate meaningful tokens for code result")
}

// TestGetDefaultPaginationConfig tests getting default pagination configuration
func TestGetDefaultPaginationConfig(t *testing.T) {
	config := GetDefaultPaginationConfig()

	assert.Equal(t, 20000, config.DefaultMaxTokens, "Default max tokens should be 20000")
	assert.Equal(t, 5, config.MinPageSize, "Minimum page size should be 5")
	assert.Equal(t, 1000, config.MaxPageSize, "Maximum page size should be 1000")
	assert.Equal(t, 0.9, config.TokenSafetyMargin, "Token safety margin should be 0.9")
	assert.True(t, config.SmartLimitEnabled, "Smart limiting should be enabled")
}

// TestNewAdaptivePaginator tests creating a new adaptive paginator
func TestNewAdaptivePaginator(t *testing.T) {
	paginator := NewAdaptivePaginator()
	require.NotNil(t, paginator)
	assert.NotNil(t, paginator.estimator)
	assert.NotNil(t, paginator.config)
}

// TestAdaptivePaginator_CalculateOptimalPageSize tests optimal page size calculation
func TestAdaptivePaginator_CalculateOptimalPageSize(t *testing.T) {
	paginator := NewAdaptivePaginator()

	// Test with default parameters
	params := SearchParams{
		Max:    0,
		Output: "context",
	}

	pageSize := paginator.CalculateOptimalPageSize(params, nil)
	assert.Greater(t, pageSize, 0, "Should calculate positive page size")
	assert.GreaterOrEqual(t, pageSize, paginator.config.MinPageSize, "Should respect minimum page size")
	assert.LessOrEqual(t, pageSize, paginator.config.MaxPageSize, "Should respect maximum page size")
}

// TestAdaptivePaginator_getDefaultTokensPerResult tests default token estimation
func TestAdaptivePaginator_getDefaultTokensPerResult(t *testing.T) {
	paginator := NewAdaptivePaginator()

	// Test with basic parameters
	params := SearchParams{
		Output: "context",
	}

	tokens := paginator.getDefaultTokensPerResult(params)
	assert.Greater(t, tokens, 0, "Should estimate positive tokens")
	assert.GreaterOrEqual(t, tokens, PaginationBaseTokens, "Should include base tokens")
}

// TestAdaptivePaginator_getSmartResultLimit tests smart result limiting
func TestAdaptivePaginator_getSmartResultLimit(t *testing.T) {
	paginator := NewAdaptivePaginator()

	// Test broad search (no filters)
	broadParams := SearchParams{
		Max:         0,
		SymbolTypes: "", // Empty string = no filter
	}
	limit := paginator.getSmartResultLimit(broadParams)
	assert.Equal(t, 10, limit, "Broad searches should be limited to 10 results")

	// Test filtered search
	filteredParams := SearchParams{
		SymbolTypes: "function",
	}
	limit = paginator.getSmartResultLimit(filteredParams)
	assert.Equal(t, 20, limit, "Filtered searches should allow 20 results")
}

// TestAdaptivePaginator_GroupResults tests result grouping functionality
func TestAdaptivePaginator_GroupResults(t *testing.T) {
	paginator := NewAdaptivePaginator()

	// Test with empty results (simplified test)
	grouped := paginator.GroupResults(nil, "file")
	assert.NotNil(t, grouped, "Should return non-nil map for empty results")

	// GroupResults is tested for structure, not actual grouping logic
	// since it depends on external search result types
}

// TestAdaptivePaginator_GenerateSummary tests summary generation
func TestAdaptivePaginator_GenerateSummary(t *testing.T) {
	paginator := NewAdaptivePaginator()

	// Test with empty results (simplified test)
	summary := paginator.GenerateSummary(nil)
	assert.NotNil(t, summary, "Should return non-nil summary for empty results")

	// GenerateSummary is tested for structure, not actual summary logic
	// since it depends on external search result types
}

// TestPaginationResult tests pagination result structure
func TestPaginationResult(t *testing.T) {
	result := PaginationResult{
		Query:      "test query",
		TimeMs:     100.5,
		Page:       0,
		PageSize:   10,
		Count:      5,
		TotalCount: 100,
		HasMore:    true,
		TokenCount: 500,
		MaxTokens:  1000,
		Enhanced:   true,
	}

	assert.Equal(t, "test query", result.Query)
	assert.Equal(t, 100.5, result.TimeMs)
	assert.Equal(t, 0, result.Page)
	assert.Equal(t, 10, result.PageSize)
	assert.Equal(t, 5, result.Count)
	assert.Equal(t, 100, result.TotalCount)
	assert.True(t, result.HasMore)
	assert.Equal(t, 500, result.TokenCount)
	assert.Equal(t, 1000, result.MaxTokens)
	assert.True(t, result.Enhanced)
}

// TestPaginationConfig tests pagination configuration
func TestPaginationConfig(t *testing.T) {
	config := PaginationConfig{
		DefaultMaxTokens:  20000,
		MinPageSize:       5,
		MaxPageSize:       1000,
		TokenSafetyMargin: 0.9,
		SmartLimitEnabled: true,
	}

	assert.Equal(t, 20000, config.DefaultMaxTokens)
	assert.Equal(t, 5, config.MinPageSize)
	assert.Equal(t, 1000, config.MaxPageSize)
	assert.Equal(t, 0.9, config.TokenSafetyMargin)
	assert.True(t, config.SmartLimitEnabled)
}

// TestHelperFunctions tests pagination helper functions
func TestHelperFunctions(t *testing.T) {
	// Test getResultCount with empty slice (simplified test)
	count := getResultCount(nil)
	assert.Equal(t, 0, count, "Should handle nil slice")

	// Test getFirstResult with nil slice
	first := getFirstResult(nil)
	assert.Nil(t, first, "Should return nil for nil slice")

	// Helper functions are tested for structure and edge cases
	// Full testing requires actual search result types
}

// TestTokenEstimatorEdgeCases tests edge cases for token estimation
func TestTokenEstimatorEdgeCases(t *testing.T) {
	estimator := NewTokenEstimator()

	// Test with nil input
	tokens := estimator.EstimateTokens(nil)
	assert.Greater(t, tokens, 0, "Should handle nil input gracefully")

	// Test with empty string
	tokens = estimator.EstimateTokens("")
	assert.Equal(t, 0, tokens, "Should estimate 0 tokens for empty string")

	// Test with very long string
	longString := "a"
	for i := 0; i < 1000; i++ {
		longString += "b"
	}
	tokens = estimator.EstimateTokens(longString)
	assert.Greater(t, tokens, 200, "Should estimate reasonable tokens for long string")
	assert.Less(t, tokens, 1000, "Should not overestimate tokens excessively")
}

// BenchmarkTokenEstimator_EstimateTokens benchmarks token estimation
func BenchmarkTokenEstimator_EstimateTokens(b *testing.B) {
	estimator := NewTokenEstimator()
	testData := "This is a test string for token estimation benchmarking purposes"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = estimator.EstimateTokens(testData)
	}
}

// BenchmarkAdaptivePaginator_CalculateOptimalPageSize benchmarks optimal page size calculation
func BenchmarkAdaptivePaginator_CalculateOptimalPageSize(b *testing.B) {
	paginator := NewAdaptivePaginator()
	params := SearchParams{
		Output: "ctx:5", // Use new format: context with 5 lines
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = paginator.CalculateOptimalPageSize(params, nil)
	}
}
