package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCreateJSONResponse tests the create j s o n response.
func TestCreateJSONResponse(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "simple map",
			data: map[string]interface{}{
				"status": "success",
				"count":  42,
			},
			wantErr: false,
		},
		{
			name: "struct data",
			data: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "test",
				Age:  25,
			},
			wantErr: false,
		},
		{
			name:    "string data",
			data:    "simple string",
			wantErr: false,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := createJSONResponse(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("createJSONResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the result structure
				if result == nil {
					t.Error("createJSONResponse() returned nil result")
					return
				}

				if len(result.Content) != 1 {
					t.Errorf("createJSONResponse() returned %d content items, want 1", len(result.Content))
					return
				}

				textContent, ok := result.Content[0].(*mcp.TextContent)
				if !ok {
					t.Error("createJSONResponse() did not return TextContent")
					return
				}

				// Verify the JSON is valid
				var parsed interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &parsed); err != nil {
					t.Errorf("createJSONResponse() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

// TestCreateErrorResponse tests the create error response.
func TestCreateErrorResponse(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		err       error
	}{
		{
			name:      "simple error",
			operation: "search",
			err:       errors.New("pattern cannot be empty"),
		},
		{
			name:      "validation error",
			operation: "tree",
			err: ValidationError{
				Field:   "function",
				Message: "cannot be empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := createErrorResponse(tt.operation, tt.err)
			if err != nil {
				t.Errorf("createErrorResponse() error = %v, want nil", err)
				return
			}

			if result == nil {
				t.Error("createErrorResponse() returned nil result")
				return
			}

			if len(result.Content) != 1 {
				t.Errorf("createErrorResponse() returned %d content items, want 1", len(result.Content))
				return
			}

			textContent, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Error("createErrorResponse() did not return TextContent")
				return
			}

			// Parse and verify the error response structure
			var errorData map[string]interface{}
			if err := json.Unmarshal([]byte(textContent.Text), &errorData); err != nil {
				t.Errorf("createErrorResponse() returned invalid JSON: %v", err)
				return
			}

			// Verify required fields
			if success, ok := errorData["success"].(bool); !ok || success {
				t.Error("createErrorResponse() success field should be false")
			}

			if operation, ok := errorData["operation"].(string); !ok || operation != tt.operation {
				t.Errorf("createErrorResponse() operation = %v, want %v", operation, tt.operation)
			}

			if errorMsg, ok := errorData["error"].(string); !ok || errorMsg != tt.err.Error() {
				t.Errorf("createErrorResponse() error = %v, want %v", errorMsg, tt.err.Error())
			}
		})
	}
}
