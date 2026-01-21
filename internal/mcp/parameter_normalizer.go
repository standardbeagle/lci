package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ParameterAlias defines a parameter alias mapping
type ParameterAlias struct {
	From   string
	To     string
	Tool   string // empty means applies to all tools
	Reason string // explanation for the alias
}

// Common parameter aliases for LCI MCP tools
var commonAliases = []ParameterAlias{
	// Search parameter aliases
	{"query", "pattern", "search", "Use 'pattern' instead of 'query' for search"},
	{"search_term", "pattern", "search", "Use 'pattern' instead of 'search_term'"},
	{"search_query", "pattern", "search", "Use 'pattern' instead of 'search_query'"},
	{"text", "pattern", "search", "Use 'pattern' instead of 'text'"},
	{"q", "pattern", "search", "Use 'pattern' instead of 'q'"},

	// get_context parameter aliases
	{"symbol_id", "id", "get_context", "Use 'id' instead of 'symbol_id'"},
	{"object_id", "id", "get_context", "Use 'id' instead of 'object_id'"},
	{"object_ids", "id", "get_context", "Use 'id' with comma-separated values instead of 'object_ids' array"},
	{"result_id", "id", "get_context", "Use 'id' from search results instead of 'result_id'"},
	{"oid", "id", "get_context", "Use 'id' instead of 'oid'"},
}

// normalizeParameters renames aliased parameters and returns warnings
func normalizeParameters(rawJSON json.RawMessage, tool string) (json.RawMessage, []string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	if len(raw) == 0 {
		return rawJSON, nil, nil
	}

	warnings := []string{}
	normalized := make(map[string]interface{})

	// Apply aliases for the specific tool
	for key, value := range raw {
		normalizedKey := key

		for _, alias := range commonAliases {
			if alias.Tool == tool || alias.Tool == "" {
				if strings.EqualFold(key, alias.From) {
					normalizedKey = alias.To
					warnings = append(warnings, fmt.Sprintf("Parameter '%s' is deprecated, use '%s' instead. %s", alias.From, alias.To, alias.Reason))
					break
				}
			}
		}

		normalized[normalizedKey] = value
	}

	result, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal normalized parameters: %w", err)
	}

	return result, warnings, nil
}

// extractObjectIDFromCodeInsight extracts object IDs from code_insight output format
// Handles formats like: "oid=ABC", "shotgun-surgery: FuncName (file.go:123) oid=ABC"
func extractObjectIDFromCodeInsight(input string) []string {
	var ids []string

	// Pattern 1: Direct oid= format (e.g., "oid=ABC" or "ABC")
	directPattern := regexp.MustCompile(`\boid=([A-Za-z0-9_]+)\b`)
	if matches := directPattern.FindAllStringSubmatch(input, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				ids = append(ids, match[1])
			}
		}
	}

	// Pattern 2: Standalone base-63 ID (2-6 chars, typical LCI object ID format)
	// Only match if it's likely an object ID (not just any word)
	standalonePattern := regexp.MustCompile(`\b([A-Za-z0-9_]{2,6})\b`)
	if matches := standalonePattern.FindAllStringSubmatch(input, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				candidate := match[1]
				// Avoid common words that aren't object IDs
				if !isCommonWord(candidate) {
					ids = append(ids, candidate)
				}
			}
		}
	}

	return ids
}

// isCommonWord checks if a string is a common word that's unlikely to be an object ID
func isCommonWord(s string) bool {
	lower := strings.ToLower(s)
	commonWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"was": true, "one": true, "our": true, "out": true, "has": true,
		"oid": true, "line": true, "file": true, "type": true, "mode": true,
		"tool": true, "info": true, "help": true, "test": true, "name": true,
	}
	return commonWords[lower]
}

// parseIDList parses comma-separated IDs or extracts IDs from code_insight format
func parseIDList(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	// Check if input contains code_insight format markers
	if strings.Contains(input, "oid=") {
		return extractObjectIDFromCodeInsight(input)
	}

	// Standard comma-separated list
	parts := strings.Split(input, ",")
	var ids []string
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// AutoSearchParams represents parameters for automatic search when symbol/path are provided
type AutoSearchParams struct {
	Symbol string
	Path   string
	Filter string // optional file filter
}

// extractAutoSearchParams extracts symbol and path from get_context parameters
// Returns the parameters for auto-search if both symbol and path are found
func extractAutoSearchParams(raw map[string]interface{}) *AutoSearchParams {
	var symbol, path, filter string

	// Look for symbol/name parameter
	if val, ok := raw["symbol"].(string); ok && val != "" {
		symbol = val
	}
	if val, ok := raw["name"].(string); ok && val != "" && symbol == "" {
		symbol = val
	}

	// Look for path parameter
	if val, ok := raw["path"].(string); ok && val != "" {
		path = val
	}
	if val, ok := raw["file"].(string); ok && val != "" && path == "" {
		path = val
	}
	if val, ok := raw["file_path"].(string); ok && val != "" && path == "" {
		path = val
	}

	// Look for filter
	if val, ok := raw["filter"].(string); ok {
		filter = val
	}

	// Only return auto-search params if we have both symbol and path
	if symbol != "" && path != "" {
		return &AutoSearchParams{
			Symbol: symbol,
			Path:   path,
			Filter: filter,
		}
	}

	return nil
}

// NormalizeSearchParams normalizes search parameters with alias support
func NormalizeSearchParams(rawJSON json.RawMessage) (json.RawMessage, []string, error) {
	return normalizeParameters(rawJSON, "search")
}

// NormalizeContextParams normalizes get_context parameters with alias support
func NormalizeContextParams(rawJSON json.RawMessage) (json.RawMessage, []string, error) {
	return normalizeParameters(rawJSON, "get_context")
}
