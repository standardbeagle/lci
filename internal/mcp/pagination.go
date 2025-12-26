package mcp

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/standardbeagle/lci/internal/search"
)

// TokenEstimator provides token counting functionality for pagination
type TokenEstimator struct {
	// Rough approximation: 1 token ≈ 4 characters for English text
	// JSON overhead and structure adds ~20% more tokens
	averageCharsPerToken float64
	jsonOverheadFactor   float64
}

// NewTokenEstimator creates a new token estimator with default values
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		averageCharsPerToken: 4.0,
		jsonOverheadFactor:   1.2, // 20% overhead for JSON structure
	}
}

// EstimateTokens estimates the token count for a search result
func (te *TokenEstimator) EstimateTokens(result interface{}) int {
	// Convert to JSON to get accurate size estimation
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		// Fallback to string length estimation
		return int(float64(len(toString(result))) / te.averageCharsPerToken * te.jsonOverheadFactor)
	}

	// Account for JSON structure and convert to estimated tokens
	charCount := len(jsonBytes)
	estimatedTokens := int(float64(charCount) / te.averageCharsPerToken * te.jsonOverheadFactor)

	return estimatedTokens
}

// EstimateResultTokens estimates tokens for a search result
func (te *TokenEstimator) EstimateResultTokens(result search.GrepResult) int {
	// Base estimation: file path + line number + content
	tokens := 0

	// File path (typically 10-50 tokens)
	tokens += int(float64(len(result.Path)) / te.averageCharsPerToken)

	// Line number and metadata (5-10 tokens)
	tokens += 10

	// Content lines (major contributor)
	for _, line := range result.Context.Lines {
		tokens += int(float64(len(line)) / te.averageCharsPerToken)
	}

	// Context metadata
	tokens += int(float64(len(result.Context.BlockType)) / te.averageCharsPerToken)

	// JSON structure overhead
	tokens = int(float64(tokens) * te.jsonOverheadFactor)

	return tokens
}

// EstimateStandardResultTokens estimates tokens for a standard search result
func (te *TokenEstimator) EstimateStandardResultTokens(result search.StandardResult) int {
	// Start with base result estimation
	tokens := te.EstimateResultTokens(result.Result)

	// Enhanced results add metadata but let's be more conservative
	// Instead of 2.5x, use 1.5x to allow more results through
	tokens = int(float64(tokens) * 1.5)

	return tokens
}

// PaginationResult represents paginated search results
type PaginationResult struct {
	Query      string      `json:"query"`
	TimeMs     float64     `json:"time_ms"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Count      int         `json:"count"`       // Results in this page
	TotalCount int         `json:"total_count"` // Total available results (-1 if unknown)
	HasMore    bool        `json:"has_more"`
	TokenCount int         `json:"token_count"` // Actual tokens used in response
	MaxTokens  int         `json:"max_tokens"`  // Token budget
	Enhanced   bool        `json:"enhanced"`
	Results    interface{} `json:"results"` // []search.GrepResult or []search.StandardResult

	// Pagination guidance for next requests
	SuggestedPageSize int  `json:"suggested_page_size,omitempty"` // Optimal page size for next request
	AutoTruncated     bool `json:"auto_truncated,omitempty"`      // Whether results were auto-truncated to prevent token limit
	NextPage          *int `json:"next_page,omitempty"`           // Next page number (null if no more)
	PrevPage          *int `json:"prev_page,omitempty"`           // Previous page number (null if first page)

	// Result organization features
	GroupedResults map[string]interface{} `json:"grouped_results,omitempty"` // Results grouped by file/symbol/etc
	Summary        map[string]interface{} `json:"summary,omitempty"`         // Summary statistics by category
}

// PaginationConfig holds configuration for adaptive pagination
type PaginationConfig struct {
	DefaultMaxTokens  int     // Default token budget (20000)
	MinPageSize       int     // Minimum results per page (5)
	MaxPageSize       int     // Maximum results per page (1000)
	TokenSafetyMargin float64 // Safety margin for token estimation (0.9 = 90%)
	SmartLimitEnabled bool    // Enable smart result limiting based on query type
}

// GetDefaultPaginationConfig returns the default pagination configuration
func GetDefaultPaginationConfig() PaginationConfig {
	return PaginationConfig{
		DefaultMaxTokens:  20000,
		MinPageSize:       5,
		MaxPageSize:       1000,
		TokenSafetyMargin: 0.9,
		SmartLimitEnabled: true,
	}
}

// AdaptivePaginator handles intelligent pagination based on token limits
type AdaptivePaginator struct {
	estimator *TokenEstimator
	config    PaginationConfig
}

// NewAdaptivePaginator creates a new adaptive paginator
func NewAdaptivePaginator() *AdaptivePaginator {
	return &AdaptivePaginator{
		estimator: NewTokenEstimator(),
		config:    GetDefaultPaginationConfig(),
	}
}

// CalculateOptimalPageSize determines the best page size for given parameters
func (ap *AdaptivePaginator) CalculateOptimalPageSize(params SearchParams, sampleResult interface{}) int {
	// Use a reasonable default token limit for the new prototype
	maxTokens := 8000 // Default for prototype
	if params.Max > 0 && params.Max < 100 {
		maxTokens = 2000 // Smaller for targeted searches
	}

	// Estimate tokens per result using sample
	var tokensPerResult int
	if sampleResult != nil {
		tokensPerResult = ap.estimator.EstimateTokens(sampleResult)
	} else {
		// Use default estimates based on search type
		tokensPerResult = ap.getDefaultTokensPerResult(params)
	}

	// Calculate optimal page size with safety margin
	availableTokens := int(float64(maxTokens) * ap.config.TokenSafetyMargin)

	// Reserve tokens for response metadata (query, pagination info, etc.)
	metadataTokens := 100
	availableTokens -= metadataTokens

	optimalPageSize := availableTokens / tokensPerResult

	// Apply bounds
	if optimalPageSize < ap.config.MinPageSize {
		optimalPageSize = ap.config.MinPageSize
	}
	if optimalPageSize > ap.config.MaxPageSize {
		optimalPageSize = ap.config.MaxPageSize
	}

	// Apply smart limiting based on query characteristics
	if ap.config.SmartLimitEnabled {
		smartLimit := ap.getSmartResultLimit(params)
		if optimalPageSize > smartLimit {
			optimalPageSize = smartLimit
		}
	}

	return optimalPageSize
}

// getDefaultTokensPerResult returns estimated tokens per result based on search parameters
func (ap *AdaptivePaginator) getDefaultTokensPerResult(params SearchParams) int {
	baseTokens := PaginationBaseTokens

	// Parse context lines from Output field
	contextLines := 0
	if params.Output != "" {
		if strings.HasPrefix(params.Output, "ctx:") {
			if parts := strings.Split(params.Output, ":"); len(parts) == 2 {
				if count, err := strconv.Atoi(parts[1]); err == nil {
					contextLines = count
				}
			}
		} else if params.Output == "ctx" {
			contextLines = PaginationDefaultContextLines
		} else if params.Output == "full" {
			contextLines = 10
		}
	}
	if contextLines == 0 {
		contextLines = PaginationDefaultContextLines
	}
	baseTokens += contextLines * search.TokensPerContextLine

	// Adjust based on output size from new SearchParams
	switch params.Output {
	case "full":
		baseTokens = int(float64(baseTokens) * 2.5) // Full results are larger
	case "ctx", "ctx:3", "ctx:5":
		baseTokens = int(float64(baseTokens) * 1.5) // Context results are moderately larger
	case "line", "files", "count":
		// Keep base tokens as is (minimal)
	}

	// Adjust based on semantic filtering (more metadata)
	// Note: DeclarationOnly and ExportedOnly are not in new format
	if params.SymbolTypes != "" {
		baseTokens = int(float64(baseTokens) * 1.3) // 30% more for semantic data
	}

	return baseTokens
}

// getSmartResultLimit returns intelligent result limits based on query type
func (ap *AdaptivePaginator) getSmartResultLimit(params SearchParams) int {
	// For MCP usage, be much more aggressive about limiting results
	// to ensure we always fit within token budgets

	// Broad searches without filters should be very limited for MCP
	if params.SymbolTypes == "" {
		return 10 // Very limited for broad searches
	}

	// Filtered searches can show moderate results
	return 20 // Moderate for filtered searches
}

// ApplyPagination applies pagination to search results with token-aware truncation
func (ap *AdaptivePaginator) ApplyPagination(results interface{}, params SearchParams, totalCount int, query string, timeMs float64) PaginationResult {
	// Use reasonable defaults for new prototype structure
	maxTokens := 8000
	if params.Output == "single-line" {
		maxTokens = 4000
	} else if params.Output == "full" {
		maxTokens = 12000
	}

	// Default to first page for simplified prototype
	page := 0
	if page < 0 {
		page = 0
	}

	var pageSize int
	var paginatedResults interface{}
	var currentTokenCount int
	var hasMore bool

	// Handle different result types
	switch r := results.(type) {
	case []search.GrepResult:
		pageSize, paginatedResults, currentTokenCount, hasMore = ap.paginateGrepResults(r, params, maxTokens)
	case []search.StandardResult:
		pageSize, paginatedResults, currentTokenCount, hasMore = ap.paginateStandardResults(r, params, maxTokens)
	case []CompactSearchResult:
		pageSize, paginatedResults, currentTokenCount, hasMore = ap.paginateSearchResults(r, params, maxTokens)
	default:
		// Fallback for unknown types
		pageSize = ap.CalculateOptimalPageSize(params, nil)
		paginatedResults = results
		currentTokenCount = ap.estimator.EstimateTokens(results)
		hasMore = false
	}

	// Simplified total count calculation for prototype
	if totalCount == -1 {
		// Don't calculate total by default for performance
		totalCount = -1
	}

	// Build pagination result
	result := PaginationResult{
		Query:      query,
		TimeMs:     timeMs,
		Page:       page,
		PageSize:   pageSize,
		Count:      getResultCount(paginatedResults),
		TotalCount: totalCount,
		HasMore:    hasMore,
		TokenCount: currentTokenCount,
		MaxTokens:  maxTokens,
		Enhanced:   params.Output != "single-line", // Enhanced unless single-line
		Results:    paginatedResults,
	}

	// Add pagination guidance
	if hasMore {
		nextPage := page + 1
		result.NextPage = &nextPage
	}
	if page > 0 {
		prevPage := page - 1
		result.PrevPage = &prevPage
	}

	// Suggest optimal page size for next requests
	result.SuggestedPageSize = ap.CalculateOptimalPageSize(params, getFirstResult(paginatedResults))

	return result
}

// paginateGrepResults handles pagination for grep search results
func (ap *AdaptivePaginator) paginateGrepResults(results []search.GrepResult, params SearchParams, maxTokens int) (int, []search.GrepResult, int, bool) {
	if len(results) == 0 {
		return 0, results, 0, false
	}

	// Auto-determine page size based on token budget for prototype
	pageSize := params.Max
	if pageSize == 0 {
		sampleResult := results[0]
		pageSize = ap.CalculateOptimalPageSize(params, sampleResult)
	}

	// Calculate start and end indices for simplified pagination
	start := 0 // Default to start for simplified prototype
	if start >= len(results) {
		return pageSize, []search.GrepResult{}, 0, false
	}

	end := start + pageSize
	hasMore := end < len(results)
	if end > len(results) {
		end = len(results)
	}

	// Extract page of results
	pageResults := results[start:end]

	// Apply token-aware truncation for prototype (always enabled)
	truncatedResults, tokenTruncated := ap.truncateGrepResultsByTokens(pageResults, maxTokens)
	pageResults = truncatedResults
	hasMore = hasMore || tokenTruncated

	// Calculate actual token count
	tokenCount := ap.calculateGrepResultsTokenCount(pageResults)

	return pageSize, pageResults, tokenCount, hasMore
}

// paginateStandardResults handles pagination for standard search results
func (ap *AdaptivePaginator) paginateStandardResults(results []search.StandardResult, params SearchParams, maxTokens int) (int, []search.StandardResult, int, bool) {
	if len(results) == 0 {
		return 0, results, 0, false
	}

	// Auto-determine page size based on token budget for prototype
	pageSize := params.Max
	if pageSize == 0 {
		sampleResult := results[0]
		pageSize = ap.CalculateOptimalPageSize(params, sampleResult)
	}

	// Calculate start and end indices for simplified pagination
	start := 0 // Default to start for simplified prototype
	if start >= len(results) {
		return pageSize, []search.StandardResult{}, 0, false
	}

	end := start + pageSize
	hasMore := end < len(results)
	if end > len(results) {
		end = len(results)
	}

	// Extract page of results
	pageResults := results[start:end]

	// Apply token-aware truncation for prototype (always enabled)
	truncatedResults, tokenTruncated := ap.truncateStandardResultsByTokens(pageResults, maxTokens)
	pageResults = truncatedResults
	hasMore = hasMore || tokenTruncated

	// Calculate actual token count
	tokenCount := ap.calculateStandardResultsTokenCount(pageResults)

	return pageSize, pageResults, tokenCount, hasMore
}

// AutoPaginateWithPrecheck performs intelligent pre-pagination to prevent token limit errors
func (ap *AdaptivePaginator) AutoPaginateWithPrecheck(results interface{}, params SearchParams, totalCount int, query string, timeMs float64) PaginationResult {
	// Use reasonable defaults for prototype
	maxTokens := 8000
	if params.Output == "single-line" {
		maxTokens = 4000
	} else if params.Output == "full" {
		maxTokens = 12000
	}

	// Estimate total response size including metadata
	metadataTokens := 200 // Base metadata tokens
	availableTokens := maxTokens - metadataTokens

	// Auto-reduce page size if current request would exceed limit
	switch v := results.(type) {
	case []search.StandardResult:
		if len(v) > 0 {
			// Estimate tokens per result based on first result
			sampleTokens := ap.estimator.EstimateResultTokens(search.GrepResult{
				Path:    v[0].Result.Path,
				Context: v[0].Result.Context,
			})

			// Calculate safe page size
			safePageSize := availableTokens / sampleTokens
			if safePageSize < len(v) && safePageSize > 0 {
				// Auto-truncate to safe size
				v = v[:safePageSize]
				results = v

				// For prototype, just apply pagination with current results
				paginationResult := ap.ApplyPagination(results, params, totalCount, query, timeMs)

				// Add auto-pagination metadata
				paginationResult.AutoTruncated = true
				paginationResult.SuggestedPageSize = safePageSize
				return paginationResult
			}
		}
	case []search.GrepResult:
		if len(v) > 0 {
			sampleTokens := ap.estimator.EstimateResultTokens(v[0])
			safePageSize := availableTokens / sampleTokens
			if safePageSize < len(v) && safePageSize > 0 {
				v = v[:safePageSize]
				results = v

				// For prototype, just apply pagination with current results
				paginationResult := ap.ApplyPagination(results, params, totalCount, query, timeMs)

				paginationResult.AutoTruncated = true
				paginationResult.SuggestedPageSize = safePageSize
				return paginationResult
			}
		}
	}

	// If no auto-truncation needed, proceed with normal pagination
	return ap.ApplyPagination(results, params, totalCount, query, timeMs)
}

// GroupResults groups search results by file, symbol type, or pattern for better organization
func (ap *AdaptivePaginator) GroupResults(results interface{}, groupBy string) map[string]interface{} {
	grouped := make(map[string]interface{})

	switch v := results.(type) {
	case []search.StandardResult:
		switch groupBy {
		case "file":
			byFile := make(map[string][]search.StandardResult)
			for _, result := range v {
				byFile[result.Result.Path] = append(byFile[result.Result.Path], result)
			}
			for file, fileResults := range byFile {
				grouped[file] = map[string]interface{}{
					"count":   len(fileResults),
					"results": fileResults,
				}
			}
		case "symbol_type":
			bySymbolType := make(map[string][]search.StandardResult)
			for _, result := range v {
				symbolType := "unknown" // StandardResult doesn't have SymbolType directly
				if result.RelationalData != nil {
					symbolType = result.RelationalData.Symbol.Type.String()
				}
				bySymbolType[symbolType] = append(bySymbolType[symbolType], result)
			}
			for symbolType, typeResults := range bySymbolType {
				grouped[symbolType] = map[string]interface{}{
					"count":   len(typeResults),
					"results": typeResults,
				}
			}
		case "directory":
			byDirectory := make(map[string][]search.StandardResult)
			for _, result := range v {
				dir := filepath.Dir(result.Result.Path)
				if dir == "." {
					dir = "root"
				}
				byDirectory[dir] = append(byDirectory[dir], result)
			}
			for dir, dirResults := range byDirectory {
				grouped[dir] = map[string]interface{}{
					"count":   len(dirResults),
					"results": dirResults,
				}
			}
		}
	case []search.GrepResult:
		switch groupBy {
		case "file":
			byFile := make(map[string][]search.GrepResult)
			for _, result := range v {
				byFile[result.Path] = append(byFile[result.Path], result)
			}
			for file, fileResults := range byFile {
				grouped[file] = map[string]interface{}{
					"count":   len(fileResults),
					"results": fileResults,
				}
			}
		case "directory":
			byDirectory := make(map[string][]search.GrepResult)
			for _, result := range v {
				dir := filepath.Dir(result.Path)
				if dir == "." {
					dir = "root"
				}
				byDirectory[dir] = append(byDirectory[dir], result)
			}
			for dir, dirResults := range byDirectory {
				grouped[dir] = map[string]interface{}{
					"count":   len(dirResults),
					"results": dirResults,
				}
			}
		}
	}

	return grouped
}

// GenerateSummary creates summary statistics for search results
func (ap *AdaptivePaginator) GenerateSummary(results interface{}) map[string]interface{} {
	summary := make(map[string]interface{})

	switch v := results.(type) {
	case []search.StandardResult:
		fileCount := make(map[string]bool)
		symbolTypes := make(map[string]int)
		directories := make(map[string]int)

		for _, result := range v {
			fileCount[result.Result.Path] = true
			symbolType := "unknown"
			if result.RelationalData != nil {
				symbolType = result.RelationalData.Symbol.Type.String()
			}
			symbolTypes[symbolType]++
			dir := filepath.Dir(result.Result.Path)
			directories[dir]++
		}

		summary["total_matches"] = len(v)
		summary["unique_files"] = len(fileCount)
		summary["symbol_types"] = symbolTypes
		summary["directories"] = directories

	case []search.GrepResult:
		fileCount := make(map[string]bool)
		directories := make(map[string]int)

		for _, result := range v {
			fileCount[result.Path] = true
			dir := filepath.Dir(result.Path)
			directories[dir]++
		}

		summary["total_matches"] = len(v)
		summary["unique_files"] = len(fileCount)
		summary["directories"] = directories
	}

	return summary
}

// truncateGrepResultsByTokens truncates grep results based on token limit
func (ap *AdaptivePaginator) truncateGrepResultsByTokens(results []search.GrepResult, maxTokens int) ([]search.GrepResult, bool) {
	tokenCount := PaginationMetadataTokens
	truncatedResults := make([]search.GrepResult, 0, len(results))

	for _, result := range results {
		resultTokens := ap.estimator.EstimateResultTokens(result)
		if tokenCount+resultTokens > maxTokens {
			return truncatedResults, true // Truncated
		}
		truncatedResults = append(truncatedResults, result)
		tokenCount += resultTokens
	}

	return truncatedResults, false // Not truncated
}

// truncateStandardResultsByTokens truncates standard results based on token limit
func (ap *AdaptivePaginator) truncateStandardResultsByTokens(results []search.StandardResult, maxTokens int) ([]search.StandardResult, bool) {
	tokenCount := PaginationMetadataTokens
	truncatedResults := make([]search.StandardResult, 0, len(results))

	// Guarantee we return at least 3 results unless there are fewer available
	// This helps ensure MCP responses are useful even if token estimation is off
	minResults := 3
	if len(results) < minResults {
		minResults = len(results)
	}

	for i, result := range results {
		resultTokens := ap.estimator.EstimateStandardResultTokens(result)

		// If we haven't reached minimum results, always include regardless of tokens
		if i < minResults {
			truncatedResults = append(truncatedResults, result)
			tokenCount += resultTokens
			continue
		}

		// After minimum, check token limit
		if tokenCount+resultTokens > maxTokens {
			return truncatedResults, true // Truncated
		}
		truncatedResults = append(truncatedResults, result)
		tokenCount += resultTokens
	}

	return truncatedResults, false // Not truncated
}

// paginateSearchResults handles pagination for MCP SearchResult type
func (ap *AdaptivePaginator) paginateSearchResults(results []CompactSearchResult, params SearchParams, maxTokens int) (int, []CompactSearchResult, int, bool) {
	if len(results) == 0 {
		return 0, results, 0, false
	}

	// Auto-determine page size based on token budget for prototype
	pageSize := params.Max
	if pageSize == 0 {
		sampleResult := results[0]
		pageSize = ap.CalculateOptimalPageSize(params, sampleResult)
	}

	// Calculate start and end indices for simplified pagination
	start := 0 // Default to start for simplified prototype
	if start >= len(results) {
		return pageSize, []CompactSearchResult{}, 0, false
	}

	end := start + pageSize
	hasMore := end < len(results)
	if end > len(results) {
		end = len(results)
	}

	// Extract page of results
	pageResults := results[start:end]

	// Apply token-aware truncation for prototype (always enabled)
	truncatedResults, tokenTruncated := ap.truncateSearchResultsByTokens(pageResults, maxTokens)
	pageResults = truncatedResults
	hasMore = hasMore || tokenTruncated

	// Calculate actual token count
	tokenCount := ap.calculateSearchResultsTokenCount(pageResults)

	return pageSize, pageResults, tokenCount, hasMore
}

// truncateSearchResultsByTokens truncates SearchResult based on token limit
func (ap *AdaptivePaginator) truncateSearchResultsByTokens(results []CompactSearchResult, maxTokens int) ([]CompactSearchResult, bool) {
	tokenCount := PaginationMetadataTokens
	truncatedResults := make([]CompactSearchResult, 0, len(results))

	// Guarantee we return at least 3 results unless there are fewer available
	// This helps ensure MCP responses are useful even if token estimation is off
	minResults := 3
	if len(results) < minResults {
		minResults = len(results)
	}

	for i, result := range results {
		resultTokens := ap.estimator.EstimateTokens(result)

		// If we haven't reached minimum results, always include regardless of tokens
		if i < minResults {
			truncatedResults = append(truncatedResults, result)
			tokenCount += resultTokens
			continue
		}

		// After minimum, check token limit
		if tokenCount+resultTokens > maxTokens {
			return truncatedResults, true // Truncated
		}
		truncatedResults = append(truncatedResults, result)
		tokenCount += resultTokens
	}

	return truncatedResults, false // Not truncated
}

// calculateSearchResultsTokenCount calculates actual token count for SearchResult
func (ap *AdaptivePaginator) calculateSearchResultsTokenCount(results []CompactSearchResult) int {
	totalTokens := PaginationMetadataTokens
	for _, result := range results {
		totalTokens += ap.estimator.EstimateTokens(result)
	}
	return totalTokens
}

// calculateGrepResultsTokenCount calculates actual token count for grep results
func (ap *AdaptivePaginator) calculateGrepResultsTokenCount(results []search.GrepResult) int {
	totalTokens := PaginationMetadataTokens
	for _, result := range results {
		totalTokens += ap.estimator.EstimateResultTokens(result)
	}
	return totalTokens
}

// calculateStandardResultsTokenCount calculates actual token count for standard results
func (ap *AdaptivePaginator) calculateStandardResultsTokenCount(results []search.StandardResult) int {
	totalTokens := PaginationMetadataTokens
	for _, result := range results {
		totalTokens += ap.estimator.EstimateStandardResultTokens(result)
	}
	return totalTokens
}

// Helper functions

func getResultCount(results interface{}) int {
	switch r := results.(type) {
	case []search.GrepResult:
		return len(r)
	case []search.StandardResult:
		return len(r)
	case []CompactSearchResult: // Add support for MCP SearchResult type
		return len(r)
	default:
		return 0
	}
}

func getFirstResult(results interface{}) interface{} {
	switch r := results.(type) {
	case []search.GrepResult:
		if len(r) > 0 {
			return r[0]
		}
	case []search.StandardResult:
		if len(r) > 0 {
			return r[0]
		}
	case []CompactSearchResult:
		if len(r) > 0 {
			return r[0]
		}
	}
	return nil
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
		return ""
	}
}
