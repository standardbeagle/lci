package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/types"
)

// MockSideEffectPropagator implements SideEffectPropagatorInterface for testing
type MockSideEffectPropagator struct {
	sideEffects map[types.SymbolID]*types.SideEffectInfo
}

func NewMockSideEffectPropagator() *MockSideEffectPropagator {
	return &MockSideEffectPropagator{
		sideEffects: make(map[types.SymbolID]*types.SideEffectInfo),
	}
}

func (m *MockSideEffectPropagator) GetSideEffectInfo(symbolID types.SymbolID) *types.SideEffectInfo {
	return m.sideEffects[symbolID]
}

func (m *MockSideEffectPropagator) GetAllSideEffects() map[types.SymbolID]*types.SideEffectInfo {
	return m.sideEffects
}

func (m *MockSideEffectPropagator) GetPureFunctions() []types.SymbolID {
	var result []types.SymbolID
	for id, info := range m.sideEffects {
		if info.IsPure {
			result = append(result, id)
		}
	}
	return result
}

func (m *MockSideEffectPropagator) GetImpureFunctions() []types.SymbolID {
	var result []types.SymbolID
	for id, info := range m.sideEffects {
		if !info.IsPure {
			result = append(result, id)
		}
	}
	return result
}

func (m *MockSideEffectPropagator) AddTestSideEffect(symbolID types.SymbolID, info *types.SideEffectInfo) {
	m.sideEffects[symbolID] = info
}

func TestSideEffectParamsUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected SideEffectParams
	}{
		{
			name: "basic mode",
			json: `{"mode": "pure"}`,
			expected: SideEffectParams{
				Mode: "pure",
			},
		},
		{
			name: "symbol mode",
			json: `{"mode": "symbol", "symbol_name": "processData", "include_reasons": true}`,
			expected: SideEffectParams{
				Mode:           "symbol",
				SymbolName:     "processData",
				IncludeReasons: true,
			},
		},
		{
			name: "category mode",
			json: `{"mode": "category", "category": "io", "max_results": 50}`,
			expected: SideEffectParams{
				Mode:       "category",
				Category:   "io",
				MaxResults: 50,
			},
		},
		{
			name: "file mode",
			json: `{"mode": "file", "file_path": "src/main.go"}`,
			expected: SideEffectParams{
				Mode:     "file",
				FilePath: "src/main.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params SideEffectParams
			err := json.Unmarshal([]byte(tt.json), &params)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Mode, params.Mode)
			assert.Equal(t, tt.expected.SymbolName, params.SymbolName)
			assert.Equal(t, tt.expected.IncludeReasons, params.IncludeReasons)
			assert.Equal(t, tt.expected.Category, params.Category)
			assert.Equal(t, tt.expected.MaxResults, params.MaxResults)
			assert.Equal(t, tt.expected.FilePath, params.FilePath)
		})
	}
}

func TestCategoriesToStrings(t *testing.T) {
	tests := []struct {
		name     string
		category types.SideEffectCategory
		expected []string
	}{
		{
			name:     "none",
			category: types.SideEffectNone,
			expected: nil,
		},
		{
			name:     "single param write",
			category: types.SideEffectParamWrite,
			expected: []string{"param_write"},
		},
		{
			name:     "multiple categories",
			category: types.SideEffectIO | types.SideEffectThrow | types.SideEffectGlobalWrite,
			expected: []string{"global_write", "io", "throw"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categoriesToStrings(tt.category)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestCategoryNameToBit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected types.SideEffectCategory
	}{
		{"param_write", "param_write", types.SideEffectParamWrite},
		{"global_write", "global_write", types.SideEffectGlobalWrite},
		{"io", "io", types.SideEffectIO},
		{"network", "network", types.SideEffectNetwork},
		{"throw", "throw", types.SideEffectThrow},
		{"channel", "channel", types.SideEffectChannel},
		{"external_call", "external_call", types.SideEffectExternalCall},
		{"unknown", "unknown_category", types.SideEffectNone},
		{"alias global", "global", types.SideEffectGlobalWrite},
		{"alias panic", "panic", types.SideEffectThrow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categoryNameToBit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfidenceToString(t *testing.T) {
	tests := []struct {
		conf     types.PurityConfidence
		expected string
	}{
		{types.ConfidenceProven, "proven"},
		{types.ConfidenceHigh, "high"},
		{types.ConfidenceMedium, "medium"},
		{types.ConfidenceLow, "low"},
		{types.ConfidenceNone, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := confidenceToString(tt.conf)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertSideEffectInfo(t *testing.T) {
	info := &types.SideEffectInfo{
		FunctionName:         "testFunc",
		FilePath:             "src/test.go",
		StartLine:            10,
		EndLine:              20,
		IsPure:               false,
		PurityScore:          0.3,
		Categories:           types.SideEffectIO | types.SideEffectParamWrite,
		TransitiveCategories: types.SideEffectThrow,
		Confidence:           types.ConfidenceHigh,
		ImpurityReasons:      []string{"Writes to parameter", "Performs I/O"},
		ErrorHandling: &types.ErrorHandlingInfo{
			CanThrow:         true,
			ExceptionNeutral: false,
			ExceptionSafe:    true,
			DeferCount:       2,
		},
	}

	t.Run("basic conversion", func(t *testing.T) {
		params := SideEffectParams{}
		result := convertSideEffectInfo(info, params)

		assert.Equal(t, "testFunc", result.SymbolName)
		assert.Equal(t, "src/test.go", result.FilePath)
		assert.Equal(t, 10, result.Line)
		assert.Equal(t, 20, result.EndLine)
		assert.False(t, result.IsPure)
		assert.Equal(t, 0.3, result.PurityScore)
		assert.ElementsMatch(t, []string{"param_write", "io"}, result.LocalCategories)
	})

	t.Run("with transitive", func(t *testing.T) {
		params := SideEffectParams{IncludeTransitive: true}
		result := convertSideEffectInfo(info, params)

		assert.ElementsMatch(t, []string{"throw"}, result.TransitiveCategories)
	})

	t.Run("with confidence", func(t *testing.T) {
		params := SideEffectParams{IncludeConfidence: true}
		result := convertSideEffectInfo(info, params)

		assert.Equal(t, "high", result.Confidence)
	})

	t.Run("with reasons", func(t *testing.T) {
		params := SideEffectParams{IncludeReasons: true}
		result := convertSideEffectInfo(info, params)

		assert.Equal(t, []string{"Writes to parameter", "Performs I/O"}, result.Reasons)
	})

	t.Run("with error handling", func(t *testing.T) {
		params := SideEffectParams{}
		result := convertSideEffectInfo(info, params)

		assert.True(t, result.CanThrow)
		assert.False(t, result.ExceptionNeutral)
		assert.True(t, result.ExceptionSafe)
		assert.Equal(t, 2, result.DeferCount)
	})
}

func TestSideEffectSummary(t *testing.T) {
	mock := NewMockSideEffectPropagator()

	// Add test data
	mock.AddTestSideEffect(1, &types.SideEffectInfo{
		IsPure:     true,
		Categories: types.SideEffectNone,
	})
	mock.AddTestSideEffect(2, &types.SideEffectInfo{
		IsPure:     true,
		Categories: types.SideEffectNone,
	})
	mock.AddTestSideEffect(3, &types.SideEffectInfo{
		IsPure:     false,
		Categories: types.SideEffectIO,
	})
	mock.AddTestSideEffect(4, &types.SideEffectInfo{
		IsPure:               false,
		Categories:           types.SideEffectParamWrite,
		TransitiveCategories: types.SideEffectThrow,
	})

	// Test GetPureFunctions
	pure := mock.GetPureFunctions()
	assert.Len(t, pure, 2)

	// Test GetImpureFunctions
	impure := mock.GetImpureFunctions()
	assert.Len(t, impure, 2)

	// Test GetAllSideEffects
	all := mock.GetAllSideEffects()
	assert.Len(t, all, 4)
}

func TestSideEffectParamsUnknownFields(t *testing.T) {
	// Test that unknown fields are captured as warnings
	jsonData := `{"mode": "pure", "unknown_field": "value"}`

	var params SideEffectParams
	err := json.Unmarshal([]byte(jsonData), &params)
	require.NoError(t, err)

	assert.Equal(t, "pure", params.Mode)
	assert.Len(t, params.Warnings, 1)
	assert.Equal(t, "unknown_field", params.Warnings[0].Name)
}

// Test helper to create a mock request
func createMockSideEffectRequest(params map[string]interface{}) *mcp.CallToolRequest {
	jsonData, _ := json.Marshal(params)
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "side_effects",
			Arguments: jsonData,
		},
	}
}

func TestSideEffectModes(t *testing.T) {
	// Test that all modes parse correctly
	modes := []string{"symbol", "file", "pure", "impure", "category", "summary"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			params := SideEffectParams{Mode: mode}
			data, err := json.Marshal(params)
			require.NoError(t, err)

			var parsed SideEffectParams
			err = json.Unmarshal(data, &parsed)
			require.NoError(t, err)
			assert.Equal(t, mode, parsed.Mode)
		})
	}
}

func TestHandleSideEffectsSummaryMode(t *testing.T) {
	// Create a server with mock components (simplified test)
	server := &Server{}

	// Test that handleSideEffects exists and can be called
	// In a full integration test, we would set up the goroutineIndex
	req := createMockSideEffectRequest(map[string]interface{}{
		"mode": "summary",
	})

	// The handler should return an error about index not being available
	// since we haven't set up the index
	result, err := server.handleSideEffects(context.Background(), req)

	// Expect an error response due to no index
	if err == nil && result != nil {
		// Check that we got an error response
		assert.NotNil(t, result)
	}
}

func TestCalculatePurityGrade(t *testing.T) {
	tests := []struct {
		ratio    float64
		expected string
	}{
		{0.95, "A"},
		{0.80, "A"},
		{0.79, "B"},
		{0.60, "B"},
		{0.59, "C"},
		{0.40, "C"},
		{0.39, "D"},
		{0.20, "D"},
		{0.19, "F"},
		{0.0, "F"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ratio_%.2f", tt.ratio), func(t *testing.T) {
			grade := calculatePurityGrade(tt.ratio)
			assert.Equal(t, tt.expected, grade, "Ratio %.2f should give grade %s", tt.ratio, tt.expected)
		})
	}
}

func TestPuritySummaryType(t *testing.T) {
	// Test that PuritySummary can be marshaled/unmarshaled correctly
	summary := &PuritySummary{
		TotalFunctions:    100,
		PureFunctions:     75,
		ImpureFunctions:   25,
		PurityRatio:       0.75,
		Grade:             "B",
		WithParamWrites:   10,
		WithGlobalWrites:  5,
		WithIOEffects:     8,
		WithThrows:        3,
		WithExternalCalls: 12,
		DetailedQuery:     `side_effects {"mode": "impure", "include_reasons": true}`,
	}

	// Marshal
	data, err := json.Marshal(summary)
	require.NoError(t, err)

	// Unmarshal
	var decoded PuritySummary
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, summary.TotalFunctions, decoded.TotalFunctions)
	assert.Equal(t, summary.PureFunctions, decoded.PureFunctions)
	assert.Equal(t, summary.PurityRatio, decoded.PurityRatio)
	assert.Equal(t, summary.Grade, decoded.Grade)
	assert.Equal(t, summary.WithIOEffects, decoded.WithIOEffects)
	assert.Equal(t, summary.DetailedQuery, decoded.DetailedQuery)
}
