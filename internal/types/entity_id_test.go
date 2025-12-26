package types

import (
	"strings"
	"testing"
)

func TestSymbolEntityID(t *testing.T) {
	tests := []struct {
		name     string
		symbol   Symbol
		rootPath string
		filePath string
		expected string
	}{
		{
			name: "Function symbol",
			symbol: Symbol{
				Name:   "CalculateMetrics",
				Type:   SymbolTypeFunction,
				Line:   45,
				Column: 18,
			},
			rootPath: "/home/beagle/work/lightning-docs/lightning-code-index",
			filePath: "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/cached_metrics_calculator.go",
			expected: "symbol:func_CalculateMetrics:cached_metrics_calculator.go:45:18",
		},
		{
			name: "Struct symbol",
			symbol: Symbol{
				Name:   "CachedMetricsCalculator",
				Type:   SymbolTypeStruct,
				Line:   16,
				Column: 27,
			},
			rootPath: "/home/beagle/work/lightning-docs/lightning-code-index",
			filePath: "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/cached_metrics_calculator.go",
			expected: "symbol:struct_CachedMetricsCalculator:cached_metrics_calculator.go:16:27",
		},
		{
			name: "Variable symbol",
			symbol: Symbol{
				Name:   "config",
				Type:   SymbolTypeVariable,
				Line:   25,
				Column: 5,
			},
			rootPath: "/home/beagle/work/lightning-docs/lightning-code-index",
			filePath: "/home/beagle/work/lightning-docs/lightning-code-index/cmd/main.go",
			expected: "symbol:var_config:main.go:25:5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.symbol.EntityID(tt.rootPath, tt.filePath)
			if result != tt.expected {
				t.Errorf("EntityID() = %v, want %v", result, tt.expected)
			}

			// Verify the ID format follows our schema
			parts := strings.Split(result, ":")
			if len(parts) != 5 {
				t.Errorf("EntityID should have 5 parts, got %d", len(parts))
			}
			if parts[0] != "symbol" {
				t.Errorf("First part should be 'symbol', got %s", parts[0])
			}
		})
	}
}

func TestEnhancedSymbolEntityID(t *testing.T) {
	enhancedSym := EnhancedSymbol{
		Symbol: Symbol{
			Name:   "ProcessData",
			Type:   SymbolTypeFunction,
			Line:   100,
			Column: 15,
		},
		ID: 12345, // This should not affect EntityID generation
	}

	expected := "symbol:func_ProcessData:processor.go:100:15"
	result := enhancedSym.EntityID("/home/beagle/work/lightning-docs/lightning-code-index", "/home/beagle/work/lightning-docs/lightning-code-index/internal/processor.go")

	if result != expected {
		t.Errorf("EnhancedSymbol.EntityID() = %v, want %v", result, expected)
	}
}

func TestFileInfoEntityID(t *testing.T) {
	tests := []struct {
		name     string
		fileInfo FileInfo
		rootPath string
		expected string
	}{
		{
			name: "Simple file",
			fileInfo: FileInfo{
				Path: "/home/beagle/work/lightning-docs/lightning-code-index/internal/core/main.go",
			},
			rootPath: "/home/beagle/work/lightning-docs/lightning-code-index",
			expected: "file:main.go:internal/core/main.go",
		},
		{
			name: "Windows path",
			fileInfo: FileInfo{
				Path: "C:\\Users\\dev\\project\\src\\main.go",
			},
			rootPath: "C:\\Users\\dev\\project",
			expected: "file:main.go:src/main.go",
		},
		{
			name: "File at root",
			fileInfo: FileInfo{
				Path: "/home/beagle/work/lightning-docs/lightning-code-index/main.go",
			},
			rootPath: "/home/beagle/work/lightning-docs/lightning-code-index",
			expected: "file:main.go:main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fileInfo.EntityID(tt.rootPath)
			if result != tt.expected {
				t.Errorf("FileInfo.EntityID() = %v, want %v", result, tt.expected)
			}

			// Verify the ID format follows our schema
			parts := strings.Split(result, ":")
			if len(parts) != 3 {
				t.Errorf("FileInfo EntityID should have 3 parts, got %d", len(parts))
			}
			if parts[0] != "file" {
				t.Errorf("First part should be 'file', got %s", parts[0])
			}
		})
	}
}

func TestReferenceEntityID(t *testing.T) {
	ref := Reference{
		ReferencedName: "CalculateMetrics",
		Type:           RefTypeCall,
		Line:           123,
		Column:         45,
	}

	expected := "reference:call_CalculateMetrics:test.go:123:45"
	result := ref.EntityID("/home/beagle/work/lightning-docs/lightning-code-index", "/home/beagle/work/lightning-docs/lightning-code-index/internal/test.go")

	if result != expected {
		t.Errorf("Reference.EntityID() = %v, want %v", result, expected)
	}
}

func TestEntityIDConsistency(t *testing.T) {
	// Test that the same inputs always produce the same ID
	symbol := Symbol{
		Name:   "TestFunction",
		Type:   SymbolTypeFunction,
		Line:   50,
		Column: 10,
	}

	rootPath := "/home/beagle/work/lightning-docs/lightning-code-index"
	filePath := "/home/beagle/work/lightning-docs/lightning-code-index/internal/test.go"

	id1 := symbol.EntityID(rootPath, filePath)
	id2 := symbol.EntityID(rootPath, filePath)

	if id1 != id2 {
		t.Errorf("EntityID generation is not consistent: %s != %s", id1, id2)
	}

	// Test that different inputs produce different IDs
	differentSymbol := Symbol{
		Name:   "DifferentFunction",
		Type:   SymbolTypeFunction,
		Line:   50,
		Column: 10,
	}

	id3 := differentSymbol.EntityID(rootPath, filePath)
	if id1 == id3 {
		t.Errorf("Different symbols produced same ID: %s", id1)
	}
}
