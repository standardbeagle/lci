package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// createJSONResponse creates a standardized JSON response for MCP tools
func createJSONResponse(data interface{}) (*mcp.CallToolResult, error) {
	content, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(content)},
		},
	}, nil
}

// createCompactResponse creates a standardized compact response for MCP tools using LCF format
func createCompactResponse(data interface{}, includeContext bool, includeMetadata bool) (*mcp.CallToolResult, error) {
	var text string

	formatter := &CompactFormatter{
		IncludeContext:     includeContext,
		IncludeMetadata:    includeMetadata,
		IncludeBreadcrumbs: includeMetadata,
	}

	switch resp := data.(type) {
	case *SearchResponse:
		text = formatter.FormatSearchResponse(resp)
	case *FilesOnlyResponse:
		text = formatter.FormatFilesOnlyResponse(resp)
	case *CountOnlyResponse:
		text = formatter.FormatCountOnlyResponse(resp)
	case *ContextResponse:
		text = formatter.FormatContextResponse(resp)
	case *CodebaseIntelligenceResponse:
		text = formatter.FormatIntelligenceResponse(resp)
	case *SemanticAnnotationsResponse:
		text = formatter.FormatAnnotationsResponse(resp)
	default:
		// Fallback to JSON for unknown types
		content, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %v", err)
		}
		text = string(content)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}, nil
}

// createErrorResponse creates a standardized error response for MCP tools
func createErrorResponse(operation string, err error) (*mcp.CallToolResult, error) {
	errorData := map[string]interface{}{
		"success":   false,
		"error":     err.Error(),
		"operation": operation,
	}

	response, marshalErr := createJSONResponse(errorData)
	if marshalErr != nil {
		return nil, marshalErr
	}

	// CRITICAL: Set IsError=true per MCP SDK specification
	// "Any errors that originate from the tool should be reported inside the result
	// object, with isError set to true, not as an MCP protocol-level error response.
	// Otherwise, the LLM would not be able to see that an error occurred and self-correct."
	response.IsError = true

	return response, nil
}

// createSmartErrorResponse creates an enhanced error response with context-aware suggestions
func createSmartErrorResponse(operation string, err error, context map[string]interface{}) (*mcp.CallToolResult, error) {
	errorData := map[string]interface{}{
		"success":   false,
		"error":     err.Error(),
		"operation": operation,
	}

	// Add suggestions based on the error type and context
	suggestions := generateErrorSuggestions(operation, err, context)
	if len(suggestions) > 0 {
		errorData["suggestions"] = suggestions
	}

	// Add help information
	if help := getOperationHelp(operation); help != "" {
		errorData["help"] = help
	}

	// Add related operations
	if related := getRelatedOperations(operation); len(related) > 0 {
		errorData["related_operations"] = related
	}

	// Add context information if provided
	if len(context) > 0 {
		errorData["context"] = context
	}

	response, marshalErr := createJSONResponse(errorData)
	if marshalErr != nil {
		return nil, marshalErr
	}

	// CRITICAL: Set IsError=true per MCP SDK specification
	response.IsError = true

	return response, nil
}

// generateErrorSuggestions generates context-aware suggestions for common errors
func generateErrorSuggestions(operation string, err error, context map[string]interface{}) []string {
	var suggestions []string
	errorMsg := err.Error()

	switch operation {
	case "search":
		if errorMsg == "pattern is required" {
			suggestions = append(suggestions, "Provide a search pattern like 'func main' or 'class User'")
			suggestions = append(suggestions, "Use wildcards: 'test*' or regex patterns with use_regex=true")
		} else if contains(errorMsg, "search failed") {
			suggestions = append(suggestions, "Try a simpler pattern first to test connectivity")
			if pattern, ok := context["pattern"].(string); ok && len(pattern) > 50 {
				suggestions = append(suggestions, "Pattern may be too complex - try breaking it into smaller parts")
			}
		} else if pattern, ok := context["pattern"].(string); ok {
			// Enhanced regex error detection based on Brummer feedback
			regexChars := []string{"|", "+", "*", "?", "^", "$", "[", "]", "{", "}", "(", ")"}
			hasRegexChars := false
			detectedChar := ""
			for _, char := range regexChars {
				if strings.Contains(pattern, char) {
					hasRegexChars = true
					detectedChar = char
					break
				}
			}

			if hasRegexChars {
				useRegex, regexSet := context["use_regex"].(bool)
				if !regexSet || !useRegex {
					suggestions = append(suggestions, fmt.Sprintf("Pattern contains '%s' - add \"use_regex\": true for alternation/regex patterns", detectedChar))
					suggestions = append(suggestions, fmt.Sprintf("Example: {\"pattern\": \"%s\", \"use_regex\": true}", pattern))
					if detectedChar == "|" {
						suggestions = append(suggestions, "The '|' operator requires regex mode for alternation (OR logic)")
					}
				}
			}

			// Specific Go pattern suggestions
			if strings.Contains(pattern, "func") || strings.Contains(pattern, "method") {
				suggestions = append(suggestions, "For Go functions, try patterns like: 'func.*handleProxy' or exact names like 'handleProxyCommand'")
				suggestions = append(suggestions, "Use '\\.' prefix to find method calls: '\\.RegisterURL\\(' (with use_regex=true)")
			}
		}

	case "find_files", "file_search":
		if errorMsg == "pattern is required" {
			suggestions = append(suggestions, "Use glob patterns like '*.go' or 'internal/**/*.ts'")
			suggestions = append(suggestions, "Use regex patterns with pattern_type='regex'")
			suggestions = append(suggestions, "Use prefix patterns with pattern_type='prefix'")
		} else if contains(errorMsg, "unsupported pattern type") {
			suggestions = append(suggestions, "Supported pattern types: 'glob' (default), 'regex', 'prefix', 'suffix'")
			suggestions = append(suggestions, "Example: {\"pattern\": \"*.go\", \"pattern_type\": \"glob\"}")
		}

	case "definition", "references":
		if errorMsg == "symbol is required" {
			suggestions = append(suggestions, "Provide a symbol name like 'main', 'User', or 'handleRequest'")
			suggestions = append(suggestions, "Try partial matches - the search will find similar symbols")
		} else if contains(errorMsg, "not found") {
			suggestions = append(suggestions, "Try searching first to find available symbols: search tool")
			suggestions = append(suggestions, "Check if the symbol exists with a broader search pattern")
		}

	case "tree":
		if contains(errorMsg, "function parameter is required") || contains(errorMsg, "not found") {
			suggestions = append(suggestions, "Use definition tool first to find exact function names")
			suggestions = append(suggestions, "Try search tool to discover available functions")
			suggestions = append(suggestions, "Function names are case-sensitive and should match exactly")
		}

	case "validate_pattern":
		if contains(errorMsg, "Invalid regex") {
			suggestions = append(suggestions, "Use online regex validators to test complex patterns")
			suggestions = append(suggestions, "Escape special characters with backslashes")
			suggestions = append(suggestions, "Try pattern_type='glob' for simpler file patterns")
		}
	}

	// Generic suggestions based on indexing state
	if contains(errorMsg, "indexing not complete") || contains(errorMsg, "no files indexed") {
		suggestions = append(suggestions, "Wait for auto-indexing to complete (this happens automatically)")
		suggestions = append(suggestions, "Check index_stats to see current indexing status")
	}

	return suggestions
}

// getOperationHelp provides helpful information about each operation
func getOperationHelp(operation string) string {
	helpMap := map[string]string{
		"search":           "Search for patterns in code content. Supports literal text, regex patterns, and symbol filtering.",
		"find_files":       "Like 'find' or 'fd' - searches file paths, not content, on an in-memory index.",
		"definition":       "Find where symbols are defined. Provide exact symbol names for best results.",
		"references":       "Find all places where symbols are used. Helps with refactoring and impact analysis.",
		"tree":             "Show function call hierarchies. Useful for understanding code dependencies.",
		"validate_pattern": "Test patterns before using them in searches. Helps avoid syntax errors.",
		"find_components":  "Discover architectural components in your codebase. Helps understand project structure.",
	}
	return helpMap[operation]
}

// getRelatedOperations suggests related operations that might be helpful
func getRelatedOperations(operation string) []string {
	relatedMap := map[string][]string{
		"search":           {"find_files", "definition", "references"},
		"find_files":       {"search", "code_insight"},
		"definition":       {"references", "search", "tree"},
		"references":       {"definition", "search", "tree"},
		"tree":             {"definition", "references", "search"},
		"validate_pattern": {"search", "find_files"},
	}
	return relatedMap[operation]
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createPreviewResponse creates a response with a preview summary for large result sets
func createPreviewResponse(data interface{}, totalCount int, threshold int) (*mcp.CallToolResult, error) {
	if totalCount <= threshold {
		// Small result set, return as normal
		return createJSONResponse(data)
	}

	// Large result set, add preview information
	responseData := map[string]interface{}{
		"preview":     true,
		"total_count": totalCount,
		"threshold":   threshold,
		"message":     fmt.Sprintf("Large result set (%d items). Showing preview - use pagination or filters to get specific results.", totalCount),
		"data":        data,
	}

	// Add performance warning if extremely large
	if totalCount > threshold*10 {
		responseData["performance_warning"] = "Very large result set may impact performance. Consider using more specific search criteria."
	}

	return createJSONResponse(responseData)
}

// createProgressResponse creates a response showing operation progress
func createProgressResponse(operation string, progress map[string]interface{}) (*mcp.CallToolResult, error) {
	progressData := map[string]interface{}{
		"operation":   operation,
		"in_progress": true,
		"progress":    progress,
		"timestamp":   strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	return createJSONResponse(progressData)
}
