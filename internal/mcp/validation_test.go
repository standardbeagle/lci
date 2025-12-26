package mcp

import (
	"testing"
)

// TestValidateSearchParams tests the validate search params.
func TestValidateSearchParams(t *testing.T) {
	tests := []struct {
		name    string
		params  SearchParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid search params",
			params: SearchParams{
				Pattern: "test",
			},
			wantErr: false,
		},
		{
			name: "empty pattern",
			params: SearchParams{
				Pattern: "",
			},
			wantErr: true,
			errMsg:  "validation error for field 'pattern': pattern cannot be empty",
		},
		{
			name: "whitespace only pattern",
			params: SearchParams{
				Pattern: "   ",
			},
			wantErr: true,
			errMsg:  "validation error for field 'pattern': pattern cannot be empty",
		},
		{
			name: "negative max results",
			params: SearchParams{
				Pattern: "test",
				Max:     -1,
			},
			wantErr: true,
			errMsg:  "validation error for field 'max': max cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSearchParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateSearchParams() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestValidateSymbolParams tests the validate symbol params.
func TestValidateSymbolParams(t *testing.T) {
	tests := []struct {
		name    string
		params  SymbolParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid symbol params",
			params: SymbolParams{
				Symbol: "TestFunction",
			},
			wantErr: false,
		},
		{
			name:    "empty symbol",
			params:  SymbolParams{Symbol: ""},
			wantErr: true,
			errMsg:  "validation error for field 'symbol': symbol cannot be empty",
		},
		{
			name:    "whitespace only symbol",
			params:  SymbolParams{Symbol: "   "},
			wantErr: true,
			errMsg:  "validation error for field 'symbol': symbol cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSymbolParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSymbolParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateSymbolParams() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestValidateObjectContextParams tests the validate object context params.
func TestValidateObjectContextParams(t *testing.T) {
	tests := []struct {
		name    string
		params  ObjectContextParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid params with single id",
			params: ObjectContextParams{
				ID: "VE",
			},
			wantErr: false,
		},
		{
			name: "valid params with multiple ids",
			params: ObjectContextParams{
				ID: "VE,tG,Ab",
			},
			wantErr: false,
		},
		{
			name: "valid params with name",
			params: ObjectContextParams{
				Name: "getUserName",
			},
			wantErr: false,
		},
		{
			name:    "missing id and name",
			params:  ObjectContextParams{},
			wantErr: true,
			errMsg:  "validation error for field 'id': missing required 'id' parameter with object ID from search results",
		},
		{
			name: "conflict - both id and name provided",
			params: ObjectContextParams{
				ID:   "VE",
				Name: "getUserName",
			},
			wantErr: true,
			errMsg:  "validation error for field 'id,name': parameter conflict: use either 'id' OR 'name', not both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateObjectContextParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateObjectContextParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateObjectContextParams() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestIsValidObjectID tests the is valid object ID helper function.
// Now tests ONLY base-63 character set validation (A-Za-z0-9_)
func TestIsValidObjectID(t *testing.T) {
	tests := []struct {
		name     string
		objectID string
		expected bool
	}{
		// Valid base-63 IDs
		{
			name:     "valid base-63 ID - alphanumeric",
			objectID: "ABC123",
			expected: true,
		},
		{
			name:     "valid base-63 ID - with underscore",
			objectID: "test_function_123",
			expected: true,
		},
		{
			name:     "valid base-63 ID - all letters",
			objectID: "testFunction",
			expected: true,
		},
		{
			name:     "valid base-63 ID - all numbers",
			objectID: "123456",
			expected: true,
		},
		{
			name:     "valid base-63 ID - mixed case",
			objectID: "AbC123XyZ",
			expected: true,
		},
		{
			name:     "valid base-63 ID - starts with underscore",
			objectID: "_privateFunction",
			expected: true,
		},

		// Invalid - contains non-base-63 characters
		{
			name:     "invalid - empty string",
			objectID: "",
			expected: false,
		},
		{
			name:     "invalid - contains colon",
			objectID: "symbol:123",
			expected: false,
		},
		{
			name:     "invalid - contains plus",
			objectID: "file:123+line:789",
			expected: false,
		},
		{
			name:     "invalid - contains hyphen",
			objectID: "invalid-format",
			expected: false,
		},
		{
			name:     "invalid - contains space",
			objectID: "AB CD",
			expected: false,
		},
		{
			name:     "invalid - contains special chars",
			objectID: "ABC!@#",
			expected: false,
		},
		{
			name:     "invalid - only separators",
			objectID: "+:+:",
			expected: false,
		},
		{
			name:     "invalid - contains dot",
			objectID: "test.function",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidObjectID(tt.objectID)
			if result != tt.expected {
				t.Errorf("isValidObjectID(%q) = %v, want %v", tt.objectID, result, tt.expected)
			}
		})
	}
}

// TestRequestValidator tests the enhanced request validator framework
func TestRequestValidator(t *testing.T) {
	validator := NewRequestValidator()

	// Add validation rules
	validator.AddRule("name", RequiredRule())
	validator.AddRule("name", MinLengthRule(2))
	validator.AddRule("name", MaxLengthRule(50))
	validator.AddRule("age", RangeRule(0, 150))
	validator.AddRule("email", RegexRule(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, "invalid email format"))

	type TestRequest struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	tests := []struct {
		name        string
		request     TestRequest
		expectValid bool
		errorFields []string
	}{
		{
			name: "valid request",
			request: TestRequest{
				Name:  "John Doe",
				Age:   30,
				Email: "john@example.com",
			},
			expectValid: true,
		},
		{
			name: "missing required name",
			request: TestRequest{
				Name:  "",
				Age:   30,
				Email: "john@example.com",
			},
			expectValid: false,
			errorFields: []string{"name"},
		},
		{
			name: "name too short",
			request: TestRequest{
				Name:  "J",
				Age:   30,
				Email: "john@example.com",
			},
			expectValid: false,
			errorFields: []string{"name"},
		},
		{
			name: "name too long",
			request: TestRequest{
				Name:  string(make([]byte, 51)), // 51 characters
				Age:   30,
				Email: "john@example.com",
			},
			expectValid: false,
			errorFields: []string{"name"},
		},
		{
			name: "age out of range",
			request: TestRequest{
				Name:  "John Doe",
				Age:   200,
				Email: "john@example.com",
			},
			expectValid: false,
			errorFields: []string{"age"},
		},
		{
			name: "invalid email format",
			request: TestRequest{
				Name:  "John Doe",
				Age:   30,
				Email: "invalid-email",
			},
			expectValid: false,
			errorFields: []string{"email"},
		},
		{
			name: "multiple validation errors",
			request: TestRequest{
				Name:  "",
				Age:   200,
				Email: "invalid-email",
			},
			expectValid: false,
			errorFields: []string{"name", "age", "email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.request)

			if tt.expectValid {
				if !result.Valid {
					t.Errorf("Expected validation to pass, but got errors: %v", result.Errors)
				}
				if len(result.Errors) != 0 {
					t.Errorf("Expected no errors, but got %d errors: %v", len(result.Errors), result.Errors)
				}
			} else {
				if result.Valid {
					t.Errorf("Expected validation to fail, but it passed")
				}
				if len(result.Errors) == 0 {
					t.Errorf("Expected validation errors, but got none")
				}

				// Check that expected error fields are present
				errorFieldMap := make(map[string]bool)
				for _, err := range result.Errors {
					errorFieldMap[err.Field] = true
				}

				for _, expectedField := range tt.errorFields {
					if !errorFieldMap[expectedField] {
						t.Errorf("Expected error for field '%s', but didn't find it in errors: %v", expectedField, result.Errors)
					}
				}
			}
		})
	}
}
