package tools

import (
	"strings"
	"testing"
)

func TestEntityIDGenerator(t *testing.T) {
	rootPath := "/home/beagle/work/lightning-docs/lightning-code-index"
	gen := NewEntityIDGenerator(rootPath)

	tests := []struct {
		name        string
		testFunc    func() string
		expected    string
		shouldParse bool
	}{
		{
			name: "Module ID generation",
			testFunc: func() string {
				return gen.GetModuleID("core", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core")
			},
			expected:    "module:core:internal/core",
			shouldParse: true,
		},
		{
			name: "File ID generation",
			testFunc: func() string {
				return gen.GetFileID("/home/beagle/work/lightning-docs/lightning-code-index/internal/core/cached_metrics_calculator.go")
			},
			expected:    "file:cached_metrics_calculator.go:internal/core/cached_metrics_calculator.go",
			shouldParse: true,
		},
		{
			name: "Function symbol ID generation",
			testFunc: func() string {
				return gen.GetSymbolID(SymbolTypeFunction, "CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/cached_metrics_calculator.go", 45, 18)
			},
			expected:    "symbol:func_CalculateMetrics:cached_metrics_calculator.go:45:18",
			shouldParse: true,
		},
		{
			name: "Struct symbol ID generation",
			testFunc: func() string {
				return gen.GetSymbolID(SymbolTypeStruct, "CachedMetricsCalculator", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/cached_metrics_calculator.go", 16, 27)
			},
			expected:    "symbol:struct_CachedMetricsCalculator:cached_metrics_calculator.go:16:27",
			shouldParse: true,
		},
		{
			name: "Reference ID generation",
			testFunc: func() string {
				return gen.GetReferenceID(ReferenceTypeCall, "func_CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/test.go", 123, 45)
			},
			expected:    "reference:call_func_CalculateMetrics:test.go:123:45",
			shouldParse: true,
		},
		{
			name: "Callsite ID generation",
			testFunc: func() string {
				return gen.GetCallsiteID("CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/test.go", 123, 45)
			},
			expected:    "reference:call_CalculateMetrics:test.go:123:45",
			shouldParse: true,
		},
		{
			name: "Usage ID generation",
			testFunc: func() string {
				return gen.GetUsageID("config", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/main.go", 45, 12)
			},
			expected:    "reference:use_config:main.go:45:12",
			shouldParse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc()

			if result != tt.expected {
				t.Errorf("Expected: %s, Got: %s", tt.expected, result)
			}

			if tt.shouldParse {
				if !IsValidEntityID(result) {
					t.Errorf("Generated ID is not valid: %s", result)
				}

				entityType, identifier, location, err := ParseEntityID(result)
				if err != nil {
					t.Errorf("Failed to parse generated ID: %v", err)
				}

				if entityType == "" || identifier == "" || location == "" {
					t.Errorf("Parsed ID has empty components: type=%s, id=%s, loc=%s",
						entityType, identifier, location)
				}
			}
		})
	}
}

func TestSanitizeForID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CalculateMetrics", "CalculateMetrics"},
		{"calculate_metrics", "calculate_metrics"},
		{"calculate-metrics", "calculate_metrics"},
		{"calculate.metrics", "calculate_metrics"},
		{"Calculate@Metrics", "CalculateMetrics"},
		{"123Function", "_123Function"},
		{"", "unnamed"},
		{"!!!", "unnamed"},
		{"func_name_123", "func_name_123"},
		{"My-Class.Name", "My_Class_Name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeForID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeForID(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractSymbolInfo(t *testing.T) {
	tests := []struct {
		name        string
		symbolID    string
		expType     string
		expName     string
		expFile     string
		expLine     int
		expColumn   int
		expectError bool
	}{
		{
			name:        "Valid function symbol ID",
			symbolID:    "symbol:func_CalculateMetrics:cached_metrics_calculator.go:45:18",
			expType:     "func",
			expName:     "CalculateMetrics",
			expFile:     "cached_metrics_calculator.go",
			expLine:     45,
			expColumn:   18,
			expectError: false,
		},
		{
			name:        "Valid struct symbol ID",
			symbolID:    "symbol:struct_CachedMetricsCalculator:cached_metrics_calculator.go:16:27",
			expType:     "struct",
			expName:     "CachedMetricsCalculator",
			expFile:     "cached_metrics_calculator.go",
			expLine:     16,
			expColumn:   27,
			expectError: false,
		},
		{
			name:        "Invalid symbol ID format",
			symbolID:    "invalid_symbol_id",
			expectError: true,
		},
		{
			name:        "Missing location info",
			symbolID:    "symbol:func_CalculateMetrics:file.go",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbolType, name, file, line, column, err := ExtractSymbolInfo(tt.symbolID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if symbolType != tt.expType {
				t.Errorf("Expected type %s, got %s", tt.expType, symbolType)
			}
			if name != tt.expName {
				t.Errorf("Expected name %s, got %s", tt.expName, name)
			}
			if file != tt.expFile {
				t.Errorf("Expected file %s, got %s", tt.expFile, file)
			}
			if line != tt.expLine {
				t.Errorf("Expected line %d, got %d", tt.expLine, line)
			}
			if column != tt.expColumn {
				t.Errorf("Expected column %d, got %d", tt.expColumn, column)
			}
		})
	}
}

func TestIsValidEntityID(t *testing.T) {
	tests := []struct {
		name     string
		entityID string
		expected bool
	}{
		{"Valid module ID", "module:core:internal/core", true},
		{"Valid file ID", "file:main.go:cmd/main.go", true},
		{"Valid symbol ID", "symbol:func_main:main.go:15:3", true},
		{"Valid reference ID", "reference:call_main:test.go:45:12", true},
		{"Invalid format - missing parts", "module:core", false},
		{"Invalid format - too many parts", "module:core:internal/core:extra", false},
		{"Invalid entity type", "invalid:some_id:path", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidEntityID(tt.entityID)
			if result != tt.expected {
				t.Errorf("IsValidEntityID(%q) = %v, expected %v", tt.entityID, result, tt.expected)
			}
		})
	}
}

func TestMakeRelativePath(t *testing.T) {
	gen := NewEntityIDGenerator("/home/beagle/work/lightning-docs/lightning-code-index")

	tests := []struct {
		absPath  string
		expected string
	}{
		{"/home/beagle/work/lightning-docs/lightning-code-index/internal/core/main.go", "internal/core/main.go"},
		{"/home/beagle/work/lightning-docs/lightning-code-index/cmd/lci/main.go", "cmd/lci/main.go"},
		{"/home/beagle/work/lightning-docs/lightning-code-index/internal/types/types.go", "internal/types/types.go"},
		{"/home/beagle/work/lightning-docs/lightning-code-index", ""}, // Root path
	}

	for _, tt := range tests {
		t.Run(tt.absPath, func(t *testing.T) {
			result := gen.makeRelativePath(tt.absPath)
			if result != tt.expected {
				t.Errorf("makeRelativePath(%q) = %q, expected %q", tt.absPath, result, tt.expected)
			}
		})
	}
}

func TestIDConsistency(t *testing.T) {
	gen := NewEntityIDGenerator("/home/beagle/work/lightning-docs/lightning-code-index")

	// Test that the same inputs always produce the same ID
	id1 := gen.GetSymbolID(SymbolTypeFunction, "CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/file.go", 45, 18)
	id2 := gen.GetSymbolID(SymbolTypeFunction, "CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/file.go", 45, 18)

	if id1 != id2 {
		t.Errorf("ID generation is not consistent: %s != %s", id1, id2)
	}

	// Test that IDs are deterministic and unique for different inputs
	id3 := gen.GetSymbolID(SymbolTypeFunction, "DifferentFunction", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/file.go", 45, 18)
	if id1 == id3 {
		t.Errorf("Different inputs produced same ID: %s", id1)
	}

	id4 := gen.GetSymbolID(SymbolTypeFunction, "CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/file.go", 46, 18)
	if id1 == id4 {
		t.Errorf("Different line numbers produced same ID: %s", id1)
	}
}

func TestIDUniqueness(t *testing.T) {
	gen := NewEntityIDGenerator("/home/beagle/work/lightning-docs/lightning-code-index")

	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	testCases := []struct {
		fn   func() string
		desc string
	}{
		{
			fn: func() string {
				return gen.GetModuleID("core", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core")
			},
			desc: "Module ID",
		},
		{
			fn: func() string {
				return gen.GetFileID("/home/beagle/work/lightning-docs/lightning-code-index/internal/core/main.go")
			},
			desc: "File ID",
		},
		{
			fn: func() string {
				return gen.GetSymbolID(SymbolTypeFunction, "TestFunc", "/home/beagle/work/lightning-docs/lightning-code-index/test.go", 1, 1)
			},
			desc: "Function Symbol ID",
		},
		{
			fn: func() string {
				return gen.GetSymbolID(SymbolTypeStruct, "TestStruct", "/home/beagle/work/lightning-docs/lightning-code-index/test.go", 1, 1)
			},
			desc: "Struct Symbol ID",
		},
		{
			fn: func() string {
				return gen.GetReferenceID(ReferenceTypeCall, "TestFunc", "/home/beagle/work/lightning-docs/lightning-code-index/caller.go", 10, 5)
			},
			desc: "Reference ID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			id := tc.fn()
			if ids[id] {
				t.Errorf("Duplicate ID generated for %s: %s", tc.desc, id)
			}
			ids[id] = true
		})
	}
}

func TestSpecialCharactersInNames(t *testing.T) {
	gen := NewEntityIDGenerator("/home/beagle/work/lightning-docs/lightning-code-index")

	specialNames := []string{
		"function-name",
		"function.name",
		"function name",
		"function@name",
		"function#name",
		"function$name",
		"function%name",
		"123function",
		"function-name-with-many-parts",
		"CamelCaseFunction",
		"snake_case_function",
		"functionWithNumbers123",
	}

	for _, name := range specialNames {
		t.Run(name, func(t *testing.T) {
			id := gen.GetSymbolID(SymbolTypeFunction, name, "/home/beagle/work/lightning-docs/lightning-code-index/test.go", 1, 1)

			// Should not contain special characters (except underscores)
			if strings.ContainsAny(id, "-@#$% ") {
				t.Errorf("ID contains special characters: %s", id)
			}

			// Should be valid
			if !IsValidEntityID(id) {
				t.Errorf("Generated ID is not valid: %s", id)
			}

			// Should be parseable
			symbolType, parsedName, _, _, _, err := ExtractSymbolInfo(id)
			if err != nil {
				t.Errorf("Failed to parse generated ID: %v", err)
				return
			}

			if symbolType != SymbolTypeFunction {
				t.Errorf("Wrong symbol type extracted: %s", symbolType)
			}

			// Parsed name might be different due to sanitization, but should be valid
			if parsedName == "" {
				t.Errorf("Parsed name is empty")
			}
		})
	}
}

func BenchmarkEntityIDGeneration(b *testing.B) {
	gen := NewEntityIDGenerator("/home/beagle/work/lightning-docs/lightning-code-index")

	b.Run("GetModuleID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GetModuleID("core", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core")
		}
	})

	b.Run("GetFileID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GetFileID("/home/beagle/work/lightning-docs/lightning-code-index/internal/core/main.go")
		}
	})

	b.Run("GetSymbolID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GetSymbolID(SymbolTypeFunction, "CalculateMetrics", "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/main.go", 45, 18)
		}
	})

	b.Run("ExtractSymbolInfo", func(b *testing.B) {
		symbolID := "symbol:func_CalculateMetrics:main.go:45:18"
		for i := 0; i < b.N; i++ {
			_, _, _, _, _, _ = ExtractSymbolInfo(symbolID)
		}
	})
}
