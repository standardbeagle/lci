package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchParamsAcceptsUnknownFields tests that unknown fields are accepted and tracked as warnings
func TestSearchParamsAcceptsUnknownFields(t *testing.T) {
	tests := []struct {
		name             string
		jsonData         string
		wantPattern      string
		wantType         string
		wantWarningCount int
		wantWarningNames []string
	}{
		{
			name:             "accepts output_mode field (known, no warning)",
			jsonData:         `{"pattern": "func", "output_mode": "files_with_matches", "type": "go"}`,
			wantPattern:      "func",
			wantType:         "go",
			wantWarningCount: 0,
			wantWarningNames: []string{},
		},
		{
			name:             "tracks unknown fields as warnings",
			jsonData:         `{"pattern": "test", "unknown_field1": "value1", "unknown_field2": "value2"}`,
			wantPattern:      "test",
			wantWarningCount: 2,
			wantWarningNames: []string{"unknown_field1", "unknown_field2"},
		},
		{
			name:             "tracks multiple unknown fields",
			jsonData:         `{"pattern": "test", "max_results": 10, "foo": "bar", "baz": 123, "qux": true}`,
			wantPattern:      "test",
			wantWarningCount: 3,
			wantWarningNames: []string{"foo", "baz", "qux"},
		},
		{
			name:             "no warnings when all fields are known",
			jsonData:         `{"pattern": "test", "max_results": 10, "type": "go", "case_insensitive": true}`,
			wantPattern:      "test",
			wantWarningCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params SearchParams
			err := json.Unmarshal([]byte(tt.jsonData), &params)

			require.NoError(t, err, "Should not error on unknown fields")
			assert.Equal(t, tt.wantPattern, params.Pattern)
			if tt.wantType != "" {
				assert.Equal(t, tt.wantType, params.Filter)
			}

			// Verify warnings
			if tt.wantWarningCount > 0 {
				assert.Len(t, params.Warnings, tt.wantWarningCount, "Should have expected number of warnings")
				for _, name := range tt.wantWarningNames {
					found := false
					for _, w := range params.Warnings {
						if w.Name == name {
							found = true
							break
						}
					}
					assert.True(t, found, fmt.Sprintf("Should have warning for field '%s'", name))
				}
			} else {
				assert.Empty(t, params.Warnings, "Should have no warnings")
			}
		})
	}
}

// TestSearchParamsOutputModeAndTypeFields tests the explicit OutputMode and Type fields
func TestSearchParamsOutputModeAndTypeFields(t *testing.T) {
	tests := []struct {
		name       string
		jsonData   string
		wantType   string
		wantOutput string
	}{
		{
			name:       "type field",
			jsonData:   `{"pattern": "test", "type": "go"}`,
			wantType:   "go",
			wantOutput: "",
		},
		{
			name:       "output_mode field",
			jsonData:   `{"pattern": "test", "output_mode": "files_with_matches"}`,
			wantOutput: "files_with_matches",
		},
		{
			name:       "both type and output_mode",
			jsonData:   `{"pattern": "test", "type": "go", "output_mode": "content"}`,
			wantType:   "go",
			wantOutput: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params SearchParams
			err := json.Unmarshal([]byte(tt.jsonData), &params)

			require.NoError(t, err)
			assert.Equal(t, "test", params.Pattern)
			assert.Equal(t, tt.wantType, params.Filter)
			assert.Equal(t, tt.wantOutput, params.Output)
		})
	}
}

// TestSearchParamsUnknownFieldsWithValidation tests that validation still works with unknown fields
func TestSearchParamsUnknownFieldsWithValidation(t *testing.T) {
	// Test that we can validate params even with unknown fields
	jsonData := `{"pattern": "", "unknown_field": "ignored", "another_unknown": 123}`
	var params SearchParams
	err := json.Unmarshal([]byte(jsonData), &params)

	require.NoError(t, err, "Should unmarshal despite unknown fields")
	assert.Equal(t, "", params.Pattern)

	// Now test validation catches empty pattern
	validationErr := validateSearchParams(params)
	assert.Error(t, validationErr, "Should fail validation for empty pattern")
	assert.Contains(t, validationErr.Error(), "pattern cannot be empty")
}

// TestSearchParamsComplexUnknownFields tests complex scenarios with many unknown fields
func TestSearchParamsComplexUnknownFields(t *testing.T) {
	jsonData := `{
		"pattern": "complex.*pattern",
		"output_mode": "files_with_matches",
		"type": "go",
		"max_results": 100,
		"include_dependencies": true,
		"unknown_array": ["item1", "item2"],
		"unknown_object": {"nested": "value"},
		"unknown_number": 42,
		"unknown_boolean": true
	}`

	var params SearchParams
	err := json.Unmarshal([]byte(jsonData), &params)

	require.NoError(t, err)
	assert.Equal(t, "complex.*pattern", params.Pattern)
	assert.Equal(t, "files_with_matches", params.Output)
	assert.Equal(t, "go", params.Filter)
	assert.Equal(t, 100, params.Max)
	// Note: IncludeDependencies is not in new format - would be in Include field as "deps"

	// Verify it doesn't panic or error
	assert.NotNil(t, &params)
}

// TestMCPCompatibleSearchParams tests specific MCP request patterns
func TestMCPCompatibleSearchParams(t *testing.T) {
	// Simulate the exact MCP calls mentioned in the issue
	mcpCall1 := `{
		"pattern": "func \\(.*\\) \\w+\\(.*\\) \\{",
		"output_mode": "files_with_matches",
		"type": "go"
	}`

	mcpCall2 := `{
		"type": "go",
		"pattern": "if.*else if.*else if",
		"output_mode": "files_with_matches"
	}`

	for i, jsonData := range []string{mcpCall1, mcpCall2} {
		t.Run(fmt.Sprintf("MCP_call_%d", i+1), func(t *testing.T) {
			var params SearchParams
			err := json.Unmarshal([]byte(jsonData), &params)

			require.NoError(t, err, "Should handle MCP request parameters")
			assert.NotEmpty(t, params.Pattern)
			assert.Equal(t, "files_with_matches", params.Output)
			assert.Equal(t, "go", params.Filter)
			// output and filter are known fields, so no warnings expected
			assert.Empty(t, params.Warnings, "Should not have warnings for known fields")
		})
	}
}

// TestMCPUnknownFieldsAsWarnings tests that unknown fields in MCP calls are captured as warnings
func TestMCPUnknownFieldsAsWarnings(t *testing.T) {
	// Simulate MCP call with unknown fields
	mcpCallWithUnknown := `{
		"pattern": "test",
		"output_mode": "files_with_matches",
		"type": "go",
		"unknown_param1": "value1",
		"unknown_param2": 123
	}`

	var params SearchParams
	err := json.Unmarshal([]byte(mcpCallWithUnknown), &params)

	require.NoError(t, err)
	assert.NotEmpty(t, params.Pattern)
	assert.Equal(t, "files_with_matches", params.Output)
	assert.Equal(t, "go", params.Filter)

	// Verify unknown fields are captured as warnings (order not guaranteed)
	require.Len(t, params.Warnings, 2, "Should have warnings for unknown fields")

	// Check that both unknown fields are present (order doesn't matter)
	var foundParam1, foundParam2 bool
	for _, warning := range params.Warnings {
		if warning.Name == "unknown_param1" {
			foundParam1 = true
			assert.Equal(t, "value1", warning.Value)
		} else if warning.Name == "unknown_param2" {
			foundParam2 = true
			assert.Equal(t, float64(123), warning.Value)
		}
	}
	assert.True(t, foundParam1, "Should have warning for unknown_param1")
	assert.True(t, foundParam2, "Should have warning for unknown_param2")
}

// BenchmarkSearchParamsUnmarshalUnknownFields benchmarks unmarshaling with unknown fields
func BenchmarkSearchParamsUnmarshalUnknownFields(b *testing.B) {
	jsonData := `{"pattern": "test", "unknown1": "value", "unknown2": 123, "unknown3": true}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var params SearchParams
		_ = json.Unmarshal([]byte(jsonData), &params)
	}
}
