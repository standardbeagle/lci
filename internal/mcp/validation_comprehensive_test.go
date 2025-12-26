package mcp

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidationErrorsUseIsErrorFlag ensures all validation errors properly set IsError=true
// per MCP SDK specification: tool errors should be reported with IsError=true so LLMs can self-correct
func TestValidationErrorsUseIsErrorFlag(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		params      interface{}
		expectedErr string
	}{
		// Search validation errors
		{
			name:        "search_empty_pattern",
			toolName:    "search",
			params:      SearchParams{Pattern: ""},
			expectedErr: "pattern cannot be empty",
		},
		{
			name:        "search_invalid_symbol_type",
			toolName:    "search",
			params:      SearchParams{Pattern: "test", SymbolTypes: "invalidtype"},
			expectedErr: "invalid symbol type 'invalidtype'",
		},
		{
			name:        "search_negative_max_results",
			toolName:    "search",
			params:      SearchParams{Pattern: "test", Max: -1},
			expectedErr: "max cannot be negative",
		},

		// Symbol validation errors
		{
			name:        "definition_empty_symbol",
			toolName:    "definition",
			params:      SymbolParams{Symbol: ""},
			expectedErr: "symbol cannot be empty",
		},

		// Tree validation errors
		{
			name:        "tree_empty_function",
			toolName:    "tree",
			params:      TreeParams{Function: ""},
			expectedErr: "function",
		},
		{
			name:        "tree_invalid_depth",
			toolName:    "tree",
			params:      TreeParams{Function: "main", MaxDepth: 100},
			expectedErr: "value must be between",
		},

		// Object context validation errors
		{
			name:        "context_no_ids",
			toolName:    "get_object_context",
			params:      ObjectContextParams{},
			expectedErr: "missing required 'id' parameter",
		},
		{
			name:        "context_conflict_id_and_name",
			toolName:    "get_object_context",
			params:      ObjectContextParams{ID: "VE", Name: "test"},
			expectedErr: "parameter conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validator directly
			var err error
			switch params := tt.params.(type) {
			case SearchParams:
				err = validateSearchParams(params)
			case SymbolParams:
				err = validateSymbolParams(params)
			case TreeParams:
				err = validateTreeParams(params)
			case ObjectContextParams:
				err = validateObjectContextParams(params)
			default:
				t.Fatalf("Unknown param type: %T", tt.params)
			}

			require.Error(t, err, "Expected validation error but got none")
			assert.Contains(t, err.Error(), tt.expectedErr, "Error message should contain expected text")

			// Verify it's a ValidationError
			validationErr, ok := err.(ValidationError)
			require.True(t, ok, "Error should be ValidationError type")
			assert.NotEmpty(t, validationErr.Code, "ValidationError should have error code")
			assert.NotEmpty(t, validationErr.Field, "ValidationError should specify field")
		})
	}
}

// TestCreateValidationErrorResponseSetsIsError ensures CreateValidationErrorResponse sets IsError=true
func TestCreateValidationErrorResponseSetsIsError(t *testing.T) {
	validationErr := ValidationError{
		Field:   "pattern",
		Message: "pattern cannot be empty",
		Code:    string(ErrCodeRequired),
	}

	result, err := CreateValidationErrorResponse("search", validationErr, nil)
	require.NoError(t, err, "CreateValidationErrorResponse should not return error")
	require.NotNil(t, result, "Result should not be nil")

	// CRITICAL: IsError must be true for tool-level errors per MCP SDK spec
	assert.True(t, result.IsError, "ValidationError responses MUST set IsError=true per MCP SDK specification")

	// Verify content structure
	require.NotEmpty(t, result.Content, "Result should have content")
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "Content should be TextContent")

	var responseData map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &responseData)
	require.NoError(t, err, "Response should be valid JSON")

	assert.False(t, responseData["success"].(bool), "success should be false")
	assert.Contains(t, responseData, "error", "Response should contain error field")
	assert.Contains(t, responseData, "validation_details", "Response should contain validation_details")
}

// TestCreateErrorResponseSetsIsError ensures createErrorResponse sets IsError=true
func TestCreateErrorResponseSetsIsError(t *testing.T) {
	result, err := createErrorResponse("search", assert.AnError)
	require.NoError(t, err, "createErrorResponse should not return error")
	require.NotNil(t, result, "Result should not be nil")

	// CRITICAL: IsError must be true for tool-level errors
	assert.True(t, result.IsError, "Error responses MUST set IsError=true per MCP SDK specification")
}

// TestCreateSmartErrorResponseSetsIsError ensures createSmartErrorResponse sets IsError=true
func TestCreateSmartErrorResponseSetsIsError(t *testing.T) {
	context := map[string]interface{}{"pattern": "test"}
	result, err := createSmartErrorResponse("search", assert.AnError, context)
	require.NoError(t, err, "createSmartErrorResponse should not return error")
	require.NotNil(t, result, "Result should not be nil")

	// CRITICAL: IsError must be true for tool-level errors
	assert.True(t, result.IsError, "Smart error responses MUST set IsError=true per MCP SDK specification")
}

// TestMCPSpecificationCompliance tests that error responses comply with MCP SDK specification
// which states: "Any errors that originate from the tool should be reported inside the result
// object, with isError set to true, not as an MCP protocol-level error response."
func TestMCPSpecificationCompliance(t *testing.T) {
	t.Run("validation_errors_must_have_isError_true", func(t *testing.T) {
		// Create a validation error response
		validationErr := ValidationError{
			Field:   "test_field",
			Message: "test validation error",
			Code:    string(ErrCodeRequired),
		}

		result, err := CreateValidationErrorResponse("test_tool", validationErr, nil)
		require.NoError(t, err, "CreateValidationErrorResponse should not return error")
		require.NotNil(t, result, "Result should not be nil")

		// CRITICAL: Per MCP SDK spec, tool errors must set IsError=true
		assert.True(t, result.IsError,
			"CRITICAL: ValidationError responses MUST set IsError=true per MCP SDK specification.\n"+
				"Quote from SDK: 'Any errors that originate from the tool should be reported inside the result object, with isError set to true'")
	})

	t.Run("general_errors_must_have_isError_true", func(t *testing.T) {
		result, err := createErrorResponse("test_tool", assert.AnError)
		require.NoError(t, err, "createErrorResponse should not return error")
		require.NotNil(t, result, "Result should not be nil")

		assert.True(t, result.IsError,
			"CRITICAL: Error responses MUST set IsError=true per MCP SDK specification")
	})

	t.Run("smart_errors_must_have_isError_true", func(t *testing.T) {
		context := map[string]interface{}{"key": "value"}
		result, err := createSmartErrorResponse("test_tool", assert.AnError, context)
		require.NoError(t, err, "createSmartErrorResponse should not return error")
		require.NotNil(t, result, "Result should not be nil")

		assert.True(t, result.IsError,
			"CRITICAL: Smart error responses MUST set IsError=true per MCP SDK specification")
	})
}
