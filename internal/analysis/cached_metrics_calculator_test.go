package analysis

import (
	"math"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/cache"
	"github.com/standardbeagle/lci/internal/types"
)

// TestCachedMetricsCalculator_Creation tests the cached metrics calculator creation.
func TestCachedMetricsCalculator_Creation(t *testing.T) {
	config := DefaultCachedMetricsConfig()
	calculator := NewCachedMetricsCalculator(config)

	if calculator == nil {
		t.Fatal("NewCachedMetricsCalculator returned nil")
	}

	if !calculator.enableCaching {
		t.Error("Expected caching to be enabled")
	}

	if calculator.cache == nil {
		t.Error("Expected cache to be initialized")
	}
}

// TestCachedMetricsCalculator_CacheDisabled tests the cached metrics calculator cache disabled.
func TestCachedMetricsCalculator_CacheDisabled(t *testing.T) {
	config := CachedMetricsConfig{
		EnableCaching: false,
		CacheConfig:   cache.DefaultCacheConfig(),
		MetricsConfig: MetricsConfig{
			EnableHalstead:          true,
			EnableMaintainability:   true,
			EnableDependencyMetrics: true,
		},
	}

	calculator := NewCachedMetricsCalculator(config)

	if calculator.enableCaching {
		t.Error("Expected caching to be disabled")
	}

	if calculator.cache != nil {
		t.Error("Expected cache to be nil when disabled")
	}
}

// TestCachedMetricsCalculator_DefaultConfig tests the cached metrics calculator default config.
func TestCachedMetricsCalculator_DefaultConfig(t *testing.T) {
	config := DefaultCachedMetricsConfig()

	if !config.EnableCaching {
		t.Error("Expected caching to be enabled by default")
	}

	if config.CacheConfig.MaxContentEntries != 400 {
		t.Errorf("Expected default cache size 400, got %d", config.CacheConfig.MaxContentEntries)
	}

	if !config.MetricsConfig.EnableHalstead {
		t.Error("Expected Halstead metrics to be enabled by default")
	}

	if !config.MetricsConfig.EnableMaintainability {
		t.Error("Expected maintainability metrics to be enabled by default")
	}
}

// TestCachedMetricsCalculator_CacheIntegration tests the cached metrics calculator cache integration.
func TestCachedMetricsCalculator_CacheIntegration(t *testing.T) {
	config := DefaultCachedMetricsConfig()
	config.CacheConfig.MaxContentEntries = 10 // Small cache for testing
	config.CacheConfig.MaxSymbolEntries = 10

	calculator := NewCachedMetricsCalculator(config)

	// Test cache statistics
	initialStats := calculator.GetCacheStats()
	if initialStats.TotalRequests != 0 {
		t.Error("Expected 0 initial cache requests")
	}

	// Test cache info
	info := calculator.GetCacheInfo()
	if info.MaxEntries != 10 {
		t.Errorf("Expected max entries 10, got %d", info.MaxEntries)
	}

	if info.Status != "excellent" && info.Status != "good" && info.Status != "fair" && info.Status != "poor" {
		t.Errorf("Unexpected cache status: %s", info.Status)
	}
}

// TestCachedMetricsCalculator_CacheOperations tests the cached metrics calculator cache operations.
func TestCachedMetricsCalculator_CacheOperations(t *testing.T) {
	config := DefaultCachedMetricsConfig()
	calculator := NewCachedMetricsCalculator(config)

	// Test cache clearing
	calculator.ClearCache()
	stats := calculator.GetCacheStats()
	if stats.TotalEntries != 0 {
		t.Error("Expected cache to be empty after clear")
	}

	// Test cache size updates
	calculator.SetCacheMaxEntries(50)
	info := calculator.GetCacheInfo()
	if info.MaxEntries != 50 {
		t.Errorf("Expected max entries 50 after update, got %d", info.MaxEntries)
	}

	// Test TTL updates
	newTTL := 30 * time.Minute
	calculator.UpdateCacheTTL(newTTL)
	info = calculator.GetCacheInfo()
	if info.TTL != newTTL {
		t.Errorf("Expected TTL %v after update, got %v", newTTL, info.TTL)
	}
}

// TestCachedMetricsCalculator_HalsteadMetrics tests the cached metrics calculator halstead metrics.
func TestCachedMetricsCalculator_HalsteadMetrics(t *testing.T) {
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Test with empty/nil node
	metrics := calculator.calculateHalsteadMetrics(nil)
	if metrics.Volume != 0 || metrics.Difficulty != 0 {
		t.Error("Expected zero metrics for nil node")
	}
}

// TestCachedMetricsCalculator_MaintainabilityIndex tests the cached metrics calculator maintainability index.
func TestCachedMetricsCalculator_MaintainabilityIndex(t *testing.T) {
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Test with simple metrics
	metrics := &CodeQualityMetrics{
		CyclomaticComplexity: 1,
		LinesOfCode:          10,
		HalsteadVolume:       50.0,
	}

	mi := calculator.calculateMaintainabilityIndex(metrics)

	// Maintainability index should be between 0 and 100
	if mi < 0 || mi > 100 {
		t.Errorf("Maintainability index out of range: %.2f", mi)
	}

	// Test with zero lines of code
	metrics.LinesOfCode = 0
	mi = calculator.calculateMaintainabilityIndex(metrics)
	if mi != 100.0 {
		t.Errorf("Expected maintainability index 100 for zero LOC, got %.2f", mi)
	}
}

// TestCachedMetricsCalculator_ComplexityCalculations tests the cached metrics calculator complexity calculations.
func TestCachedMetricsCalculator_ComplexityCalculations(t *testing.T) {
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Test cyclomatic complexity with nil node
	complexity := calculator.calculateCyclomaticComplexity(nil)
	if complexity != 1 { // Base complexity
		t.Errorf("Expected base complexity 1 for nil node, got %d", complexity)
	}

	// Test cognitive complexity with nil node
	cognitiveComplexity := calculator.calculateCognitiveComplexity(nil)
	if cognitiveComplexity != 0 {
		t.Errorf("Expected cognitive complexity 0 for nil node, got %d", cognitiveComplexity)
	}

	// Test nesting depth with nil node
	nestingDepth := calculator.calculateNestingDepth(nil, 0)
	if nestingDepth != 0 {
		t.Errorf("Expected nesting depth 0 for nil node, got %d", nestingDepth)
	}
}

// TestCachedMetricsCalculator_DependencyMetrics tests the cached metrics calculator dependency metrics.
func TestCachedMetricsCalculator_DependencyMetrics(t *testing.T) {
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Create a test enhanced symbol
	symbol := &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:   "testFunction",
			Type:   types.SymbolTypeFunction,
			FileID: types.FileID(1),
			Line:   10,
			Column: 5,
		},
		ID: types.SymbolID(123),
	}

	// Test dependency metrics calculation
	depMetrics, err := calculator.calculateDependencyMetrics(symbol)
	if err != nil {
		t.Fatalf("Failed to calculate dependency metrics: %v", err)
	}

	// Verify default values (placeholder implementation)
	if depMetrics.IncomingDependencies != 0 {
		t.Errorf("Expected 0 incoming dependencies, got %d", depMetrics.IncomingDependencies)
	}

	if depMetrics.StabilityIndex != 1.0 {
		t.Errorf("Expected stability index 1.0, got %.2f", depMetrics.StabilityIndex)
	}

	if depMetrics.HasCircularDeps {
		t.Error("Expected no circular dependencies in placeholder implementation")
	}
}

// TestCachedMetricsCalculator_HalsteadHelpers tests the cached metrics calculator halstead helpers.
func TestCachedMetricsCalculator_HalsteadHelpers(t *testing.T) {
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Test operator detection
	if !calculator.isOperator("binary_expression") {
		t.Error("Expected binary_expression to be classified as operator")
	}

	if calculator.isOperator("unknown_type") {
		t.Error("Expected unknown_type to not be classified as operator")
	}

	// Test operand detection
	if !calculator.isOperand("identifier") {
		t.Error("Expected identifier to be classified as operand")
	}

	if calculator.isOperand("unknown_type") {
		t.Error("Expected unknown_type to not be classified as operand")
	}

	// Test sum values
	testMap := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	sum := calculator.sumValues(testMap)
	if sum != 6 {
		t.Errorf("Expected sum 6, got %d", sum)
	}
}

// TestCachedMetricsCalculator_MathHelpers tests the cached metrics calculator math helpers.
func TestCachedMetricsCalculator_MathHelpers(t *testing.T) {
	// Test logBase2
	result := logBase2(8.0)
	expected := 3.0
	if result != expected {
		t.Errorf("Expected log2(8) = %.1f, got %.1f", expected, result)
	}

	// Test with zero/negative values
	result = logBase2(0.0)
	if result != 0.0 {
		t.Errorf("Expected log2(0) = 0, got %.1f", result)
	}

	result = logBase2(-1.0)
	if result != 0.0 {
		t.Errorf("Expected log2(-1) = 0, got %.1f", result)
	}

	// Test logNatural
	result = logNatural(math.E)
	if math.Abs(result-1.0) > 0.0001 {
		t.Errorf("Expected ln(e) = 1, got %.4f", result)
	}

	// Test with zero/negative values
	result = logNatural(0.0)
	if result != 0.0 {
		t.Errorf("Expected ln(0) = 0, got %.1f", result)
	}
}
