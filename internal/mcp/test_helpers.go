package mcp

// MCP In-Process Testing
// =======================
// This file provides in-process testing utilities for MCP tools.
//
// 📚 DOCUMENTATION: /docs/testing/MCP-TESTING.md
//
// Key Feature:
// - CallTool(): Direct method invocation for MCP tools (bypasses stdio transport)
//
// Why In-Process Testing?
// - Fast: No stdio overhead (~1-5ms vs ~50-100ms per call)
// - Reliable: No process communication issues
// - Debuggable: Direct stack traces
// - Synchronous: No async complexity
//
// Usage:
//
//	server, _ := mcp.NewServer(indexer, cfg)
//	resultJSON, err := server.CallTool("search", map[string]interface{}{
//	    "pattern": "ServeHTTP",
//	})
//
// See: /docs/testing/MCP-TESTING.md for complete testing patterns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CallTool is a test helper method to simulate MCP tool calls
func (s *Server) CallTool(toolName string, params map[string]interface{}) (string, error) {
	ctx := context.Background()

	// Convert params to JSON for proper typing
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal params: %w", err)
	}

	// Create a CallToolRequest with the arguments
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      toolName,
			Arguments: paramsJSON,
		},
	}

	var result *mcp.CallToolResult

	switch toolName {
	case "search":
		result, err = s.handleNewSearch(ctx, req)

	case "get_object_context":
		result, err = s.handleGetObjectContext(ctx, req)

	case "semantic_annotations":
		result, err = s.handleSemanticAnnotations(ctx, req)

	case "side_effects":
		result, err = s.handleSideEffects(ctx, req)

	case "codebase_intelligence", "code_insight":
		result, err = s.handleCodebaseIntelligence(ctx, req)

	case "definition":
		// Not implemented in prototype - return error
		return "", errors.New("definition tool not implemented in prototype")

	case "references":
		// Not implemented in prototype - return error
		return "", errors.New("references tool not implemented in prototype")

	case "tree":
		// Not implemented in prototype - return error
		return "", errors.New("tree tool not implemented in prototype")

	case "version":
		// Version is now accessed via info tool
		req.Params.Arguments = []byte(`{"tool": "version"}`)
		result, err = s.handleInfo(ctx, req)

	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	if err != nil {
		return "", err
	}

	// Convert result to JSON string
	if result != nil && len(result.Content) > 0 {
		// The result is in Content[0] for MCP
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			// Check if this is an error response
			var response map[string]interface{}
			if json.Unmarshal([]byte(textContent.Text), &response) == nil {
				if success, ok := response["success"].(bool); ok && !success {
					// This is an error response - return it as a Go error for test validation
					if errorMsg, ok := response["error"].(string); ok {
						// Include full response context for debugging
						errorDetails := "MCP error: " + errorMsg
						if suggestion, ok := response["suggestion"].(string); ok && suggestion != "" {
							errorDetails += "\nSuggestion: " + suggestion
						}
						if context, ok := response["context"].(map[string]interface{}); ok && len(context) > 0 {
							contextJSON, _ := json.MarshalIndent(context, "", "  ")
							errorDetails += "\nContext: " + string(contextJSON)
						}
						return "", fmt.Errorf("%s", errorDetails)
					}
				}
			}
			return textContent.Text, nil
		}
	}

	return "", nil
}
