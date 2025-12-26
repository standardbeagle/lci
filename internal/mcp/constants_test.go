package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSearchDefaultMax tests the default max results for search operations
func TestSearchDefaultMax(t *testing.T) {
	assert.Equal(t, 100, SearchDefaultMax, "Search default max results should be 100")
	assert.Greater(t, SearchDefaultMax, 0, "Search default max results should be positive")
	assert.Less(t, SearchDefaultMax, 1000, "Search default max results should be reasonable")
}

// TestSearchDefaultContextLines tests the default context lines for search operations
func TestSearchDefaultContextLines(t *testing.T) {
	assert.Equal(t, 5, SearchDefaultContextLines, "Search default context lines should be 5")
	assert.Greater(t, SearchDefaultContextLines, 0, "Search default context lines should be positive")
	assert.Less(t, SearchDefaultContextLines, 20, "Search default context lines should be reasonable")
}

// TestGrepDefaultMax tests the default max results for grep operations
func TestGrepDefaultMax(t *testing.T) {
	assert.Equal(t, 500, GrepDefaultMax, "Grep default max results should be 500")
	assert.Greater(t, GrepDefaultMax, SearchDefaultMax, "Grep should allow more results than search")
	assert.Less(t, GrepDefaultMax, 5000, "Grep default max results should be reasonable")
}

// TestGrepDefaultContextLines tests the default context lines for grep operations
func TestGrepDefaultContextLines(t *testing.T) {
	assert.Equal(t, 3, GrepDefaultContextLines, "Grep default context lines should be 3")
	assert.Greater(t, GrepDefaultContextLines, 0, "Grep default context lines should be positive")
	assert.Less(t, GrepDefaultContextLines, SearchDefaultContextLines, "Grep should use less context than search")
}

// TestIndexCompareDefaultMaxLines tests the default max lines for index comparison
func TestIndexCompareDefaultMaxLines(t *testing.T) {
	assert.Equal(t, 3, IndexCompareDefaultMaxLines, "Index compare default max lines should be 3")
	assert.Greater(t, IndexCompareDefaultMaxLines, 0, "Index compare default max lines should be positive")
	assert.Equal(t, IndexCompareDefaultMaxLines, GrepDefaultContextLines, "Index compare should match grep context")
}

// TestPaginationDefaultContextLines tests the default context lines for pagination
func TestPaginationDefaultContextLines(t *testing.T) {
	assert.Equal(t, 3, PaginationDefaultContextLines, "Pagination default context lines should be 3")
	assert.Greater(t, PaginationDefaultContextLines, 0, "Pagination default context lines should be positive")
	assert.Equal(t, PaginationDefaultContextLines, GrepDefaultContextLines, "Pagination should match grep context")
}

// TestPaginationBaseTokens tests the base token count for pagination
func TestPaginationBaseTokens(t *testing.T) {
	assert.Equal(t, 50, PaginationBaseTokens, "Pagination base tokens should be 50")
	assert.Greater(t, PaginationBaseTokens, 0, "Pagination base tokens should be positive")
	assert.Less(t, PaginationBaseTokens, 200, "Pagination base tokens should be reasonable")
}

// TestPaginationMetadataTokens tests the metadata token reserve for pagination
func TestPaginationMetadataTokens(t *testing.T) {
	assert.Equal(t, 100, PaginationMetadataTokens, "Pagination metadata tokens should be 100")
	assert.Greater(t, PaginationMetadataTokens, 0, "Pagination metadata tokens should be positive")
	assert.Greater(t, PaginationMetadataTokens, PaginationBaseTokens, "Metadata tokens should exceed base tokens")
}

// TestConstantRelationships tests that constants maintain sensible relationships
func TestConstantRelationships(t *testing.T) {
	// Grep should allow more results than search (optimized for speed)
	assert.Greater(t, GrepDefaultMax, SearchDefaultMax,
		"Grep max results should be greater than search max results")

	// Grep should use less context than search (for speed)
	assert.Less(t, GrepDefaultContextLines, SearchDefaultContextLines,
		"Grep context lines should be less than search context lines")

	// Pagination should match grep for consistency
	assert.Equal(t, PaginationDefaultContextLines, GrepDefaultContextLines,
		"Pagination context should match grep context")

	// Index compare should match grep context
	assert.Equal(t, IndexCompareDefaultMaxLines, GrepDefaultContextLines,
		"Index compare context should match grep context")

	// Total token budget should be reasonable
	totalTokens := PaginationBaseTokens + PaginationMetadataTokens
	assert.Greater(t, totalTokens, 100, "Total token budget should be meaningful")
	assert.Less(t, totalTokens, 500, "Total token budget should be reasonable")
}

// TestConstantValuesRationale tests that constant values are well-chosen
func TestConstantValuesRationale(t *testing.T) {
	// Test that search defaults are reasonable for AI consumption
	assert.GreaterOrEqual(t, SearchDefaultMax, 50, "Search should provide at least 50 results")
	assert.LessOrEqual(t, SearchDefaultMax, 200, "Search should not exceed 200 results")

	assert.GreaterOrEqual(t, SearchDefaultContextLines, 3, "Search should provide at least 3 context lines")
	assert.LessOrEqual(t, SearchDefaultContextLines, 10, "Search should not exceed 10 context lines")

	// Test that grep defaults are optimized for speed
	assert.GreaterOrEqual(t, GrepDefaultMax, 200, "Grep should provide at least 200 results")
	assert.LessOrEqual(t, GrepDefaultMax, 1000, "Grep should not exceed 1000 results")

	assert.GreaterOrEqual(t, GrepDefaultContextLines, 1, "Grep should provide at least 1 context line")
	assert.LessOrEqual(t, GrepDefaultContextLines, 5, "Grep should not exceed 5 context lines")

	// Test that pagination tokens are well-balanced
	assert.Greater(t, PaginationMetadataTokens, PaginationBaseTokens,
		"Metadata tokens should be greater than base tokens")
	assert.Less(t, PaginationMetadataTokens, PaginationBaseTokens*5,
		"Metadata tokens should not be excessively larger than base tokens")
}

// BenchmarkConstantAccess benchmarks constant access performance
func BenchmarkConstantAccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = SearchDefaultMax
		_ = SearchDefaultContextLines
		_ = GrepDefaultMax
		_ = GrepDefaultContextLines
		_ = IndexCompareDefaultMaxLines
		_ = PaginationDefaultContextLines
		_ = PaginationBaseTokens
		_ = PaginationMetadataTokens
	}
}
