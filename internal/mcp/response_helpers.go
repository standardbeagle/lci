package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addWarningsToResponse adds warning messages to an MCP response
// This modifies the response to include a "warnings" field in the JSON metadata
// while preserving all existing response content
func addWarningsToResponse(result *mcp.CallToolResult, warnings []string) {
	if result == nil || len(warnings) == 0 {
		return
	}

	// Try to add warnings to existing content
	if len(result.Content) > 0 {
		// Check if the first content is TextContent
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			// Parse the existing JSON
			var responseData map[string]interface{}
			if err := json.Unmarshal([]byte(textContent.Text), &responseData); err == nil {
				// Add warnings field
				responseData["warnings"] = warnings

				// Re-encode with warnings
				if updatedJSON, err := json.Marshal(responseData); err == nil {
					result.Content[0] = &mcp.TextContent{
						Text: string(updatedJSON),
					}
					return
				}
			}

			// If we couldn't parse as JSON, append warnings as text
			warningText := "\n\nWarnings:\n"
			for _, warning := range warnings {
				warningText += fmt.Sprintf("- %s\n", warning)
			}
			textContent.Text += warningText
		}
	}
}

// createResponseWithWarnings creates an MCP response with warnings included
func createResponseWithWarnings(data interface{}, warnings []string) (*mcp.CallToolResult, error) {
	// First create the base response
	response, err := createJSONResponse(data)
	if err != nil {
		return nil, err
	}

	// Add warnings if present
	if len(warnings) > 0 {
		addWarningsToResponse(response, warnings)
	}

	return response, nil
}

// createJSONResponseWithMetadata creates a JSON response with additional metadata fields
func createJSONResponseWithMetadata(data interface{}, metadata map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert data to map if it isn't already
	var responseMap map[string]interface{}

	// Try to convert data to JSON and back to map
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, &responseMap); err != nil {
		// If it's not a map-like structure, wrap it
		responseMap = map[string]interface{}{
			"data": data,
		}
	}

	// Add metadata fields
	for key, value := range metadata {
		responseMap[key] = value
	}

	// Create the final response
	return createJSONResponse(responseMap)
}

// wrapErrorWithWarnings creates an error response that includes warnings
func wrapErrorWithWarnings(toolName string, err error, warnings []string) *mcp.CallToolResult {
	errorData := map[string]interface{}{
		"error": map[string]interface{}{
			"tool":    toolName,
			"message": err.Error(),
		},
	}

	if len(warnings) > 0 {
		errorData["warnings"] = warnings
	}

	result, _ := createJSONResponse(errorData)
	return result
}
