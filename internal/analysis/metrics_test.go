package analysis

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestMetricsCalculator_CalculateSymbolMetrics tests the metrics calculator calculate symbol metrics.
func TestMetricsCalculator_CalculateSymbolMetrics(t *testing.T) {
	// Create a duplicate detector for the metrics calculator
	duplicateDetector := NewDuplicateDetector()
	calculator := NewMetricsCalculator(duplicateDetector)

	// Test symbol with basic properties
	symbol := &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:      "testFunction",
			Type:      types.SymbolTypeFunction,
			FileID:    1,
			Line:      1,
			Column:    0,
			EndLine:   10,
			EndColumn: 0,
		},
	}

	// Mock content representing a simple function
	content := `function testFunction() {
    if (condition) {
        doSomething();
    } else {
        doSomethingElse();
    }
    return result;
}`

	// Calculate metrics
	metrics := calculator.CalculateSymbolMetrics(symbol, nil, []byte(content))

	// Verify metrics were calculated
	if metrics == nil {
		t.Fatal("Expected metrics to be calculated, got nil")
	}

	// Verify basic structure
	if metrics.Name != "testFunction" {
		t.Errorf("Expected Name to be 'testFunction', got %s", metrics.Name)
	}

	if metrics.Type != types.SymbolTypeFunction {
		t.Errorf("Expected Type to be SymbolTypeFunction, got %v", metrics.Type)
	}

	if metrics.FileID != 1 {
		t.Errorf("Expected FileID to be 1, got %d", metrics.FileID)
	}

	// Check that quality metrics structure exists
	// Note: Actual calculation values depend on AST parsing implementation

	// Check risk score is within bounds
	if metrics.RiskScore < 0 || metrics.RiskScore > 10 {
		t.Errorf("Expected RiskScore to be between 0 and 10, got %d", metrics.RiskScore)
	}
}

// TestMetricsCalculator_Structure tests the metrics calculator structure.
func TestMetricsCalculator_Structure(t *testing.T) {
	duplicateDetector := NewDuplicateDetector()
	calculator := NewMetricsCalculator(duplicateDetector)

	if calculator == nil {
		t.Fatal("Expected NewMetricsCalculator to return non-nil calculator")
	}

	if calculator.duplicateDetector != duplicateDetector {
		t.Error("Expected metrics calculator to store the provided duplicate detector")
	}
}
