package semantic

import (
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// Helper function to create mock annotated symbols
func createMockAnnotation(labels []string, category string, deps []core.Dependency) *core.SemanticAnnotation {
	return &core.SemanticAnnotation{
		Labels:       labels,
		Category:     category,
		Dependencies: deps,
		Tags:         make(map[string]string),
		Metrics:      make(map[string]interface{}),
		Attributes:   make(map[string]interface{}),
	}
}

func createMockAnnotatedSymbol(fileID types.FileID, symbolID types.SymbolID, name string, annotation *core.SemanticAnnotation) *core.AnnotatedSymbol {
	return &core.AnnotatedSymbol{
		FileID: fileID,
		SymbolID: symbolID,
		Symbol: types.Symbol{
			Name: name,
			Type: types.SymbolTypeFunction,
			Line: 1,
			Column: 0,
		},
		Annotation: annotation,
		FilePath: "test.go",
	}
}

func TestNewAnnotationSearchIndex(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	if idx == nil {
		t.Fatal("NewAnnotationSearchIndex returned nil")
	}

	if idx.labelIndex == nil {
		t.Error("labelIndex should be initialized")
	}

	if idx.categoryIndex == nil {
		t.Error("categoryIndex should be initialized")
	}
}

func TestGetSymbolsByLabel(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data directly
	symbolID := types.SymbolID(1)
	idx.labelIndex["critical"] = []types.SymbolID{symbolID}
	idx.symbolLabelMap[symbolID] = []string{"critical"}

	results := idx.GetSymbolsByLabel("critical")

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0] != symbolID {
		t.Errorf("Expected symbolID %d, got %d", symbolID, results[0])
	}
}

func TestGetSymbolsByCategory(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data directly
	symbolID := types.SymbolID(2)
	idx.categoryIndex["authentication"] = []types.SymbolID{symbolID}
	idx.symbolCategoryMap[symbolID] = "authentication"

	results := idx.GetSymbolsByCategory("authentication")

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0] != symbolID {
		t.Errorf("Expected symbolID %d, got %d", symbolID, results[0])
	}
}

func TestGetSymbolsByTag(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data directly
	symbolID := types.SymbolID(3)
	tagKey := "team"
	tagValue := "backend"
	fullKey := tagKey + ":" + tagValue

	if idx.tagIndex[fullKey] == nil {
		idx.tagIndex[fullKey] = make(map[string][]types.SymbolID)
	}
	idx.tagIndex[fullKey][tagKey] = []types.SymbolID{symbolID}

	results := idx.GetSymbolsByTag(tagKey, tagValue)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0] != symbolID {
		t.Errorf("Expected symbolID %d, got %d", symbolID, results[0])
	}
}

func TestGetSymbolsDependingOn(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data directly
	symbolID := types.SymbolID(4)
	deps := []core.Dependency{
		{
			Type: "database",
			Name: "users",
			Mode: "read",
		},
	}
	idx.symbolDepsMap[symbolID] = deps

	results := idx.GetSymbolsDependingOn("database", "users")

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0] != symbolID {
		t.Errorf("Expected symbolID %d, got %d", symbolID, results[0])
	}
}

func TestExpandQueryWithAnnotations(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	symbolID := types.SymbolID(5)
	idx.labelIndex["auth"] = []types.SymbolID{symbolID}

	expanded := idx.ExpandQueryWithAnnotations("login", []string{"auth"}, nil)

	// Should contain at least the original pattern
	if len(expanded) == 0 {
		t.Error("Expected at least 1 expansion")
	}

	// Original pattern should be first
	if expanded[0] != "login" {
		t.Errorf("Expected first expansion to be 'login', got '%s'", expanded[0])
	}
}

func TestGetLabelsForSymbol(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	symbolID := types.SymbolID(6)
	labels := []string{"critical", "security"}
	idx.symbolLabelMap[symbolID] = labels

	results := idx.GetLabelsForSymbol(symbolID)

	if len(results) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(results))
	}

	for i, label := range labels {
		if results[i] != label {
			t.Errorf("Expected label '%s', got '%s'", label, results[i])
		}
	}
}

func TestGetCategoryForSymbol(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	symbolID := types.SymbolID(7)
	category := "endpoint"
	idx.symbolCategoryMap[symbolID] = category

	result := idx.GetCategoryForSymbol(symbolID)

	if result != category {
		t.Errorf("Expected category '%s', got '%s'", category, result)
	}
}

func TestGetDependenciesForSymbol(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	symbolID := types.SymbolID(8)
	deps := []core.Dependency{
		{
			Type: "service",
			Name: "auth-service",
			Mode: "read-write",
		},
	}
	idx.symbolDepsMap[symbolID] = deps

	results := idx.GetDependenciesForSymbol(symbolID)

	if len(results) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(results))
	}

	if results[0].Type != "service" {
		t.Errorf("Expected type 'service', got '%s'", results[0].Type)
	}
}

func TestGetLabelStats(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	idx.labelIndex["critical"] = []types.SymbolID{1, 2}
	idx.labelIndex["security"] = []types.SymbolID{3}

	stats := idx.GetLabelStats()

	if len(stats) != 2 {
		t.Errorf("Expected 2 label stats, got %d", len(stats))
	}

	if stats["critical"] != 2 {
		t.Errorf("Expected 'critical' count 2, got %d", stats["critical"])
	}

	if stats["security"] != 1 {
		t.Errorf("Expected 'security' count 1, got %d", stats["security"])
	}
}

func TestGetCategoryStats(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	idx.categoryIndex["endpoint"] = []types.SymbolID{1, 2, 3}
	idx.categoryIndex["middleware"] = []types.SymbolID{4}

	stats := idx.GetCategoryStats()

	if len(stats) != 2 {
		t.Errorf("Expected 2 category stats, got %d", len(stats))
	}

	if stats["endpoint"] != 3 {
		t.Errorf("Expected 'endpoint' count 3, got %d", stats["endpoint"])
	}

	if stats["middleware"] != 1 {
		t.Errorf("Expected 'middleware' count 1, got %d", stats["middleware"])
	}
}

func TestGetTotalAnnotatedSymbols(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	idx.symbolLabelMap[1] = []string{"critical"}
	idx.symbolLabelMap[2] = []string{"security"}
	idx.symbolLabelMap[3] = []string{"performance"}

	total := idx.GetTotalAnnotatedSymbols()

	if total != 3 {
		t.Errorf("Expected 3 annotated symbols, got %d", total)
	}
}

func TestGetAnnotationCoverage(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	idx.symbolLabelMap[1] = []string{"critical"}
	idx.symbolLabelMap[2] = []string{"security"}

	coverage := idx.GetAnnotationCoverage(10)

	if coverage != 20.0 {
		t.Errorf("Expected coverage 20.0%%, got %.1f%%", coverage)
	}
}

func TestAnnotationCoverageEdgeCases(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Empty index with zero total symbols
	coverage := idx.GetAnnotationCoverage(0)
	if coverage != 0.0 {
		t.Errorf("Expected coverage 0.0%% for zero total symbols, got %.1f%%", coverage)
	}

	// Empty index
	coverage = idx.GetAnnotationCoverage(100)
	if coverage != 0.0 {
		t.Errorf("Expected coverage 0.0%% for empty index, got %.1f%%", coverage)
	}
}

func TestSearchQueryBuilder(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	idx.labelIndex["critical"] = []types.SymbolID{1, 2}
	idx.categoryIndex["endpoint"] = []types.SymbolID{2, 3}

	builder := NewSearchQueryBuilder(idx)
	builder.WithLabel("critical").WithCategory("endpoint")

	results := builder.Execute()

	// Should find symbols that have both label AND category (intersection)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0] != types.SymbolID(2) {
		t.Errorf("Expected symbolID 2, got %d", results[0])
	}
}

func TestSearchQueryBuilderWithTag(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	fullKey := "team:backend"
	if idx.tagIndex[fullKey] == nil {
		idx.tagIndex[fullKey] = make(map[string][]types.SymbolID)
	}
	idx.tagIndex[fullKey]["team"] = []types.SymbolID{1, 2}

	builder := NewSearchQueryBuilder(idx)
	builder.WithTag("team", "backend")

	results := builder.Execute()

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestSearchQueryBuilderWithDependency(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	idx.symbolDepsMap[1] = []core.Dependency{
		{Type: "database", Name: "users"},
	}

	builder := NewSearchQueryBuilder(idx)
	builder.WithDependency("database", "users")

	results := builder.Execute()

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0] != types.SymbolID(1) {
		t.Errorf("Expected symbolID 1, got %d", results[0])
	}
}

func TestSearchQueryBuilderMultipleFilters(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data: symbol 5 has label, category, AND dependency
	idx.labelIndex["critical"] = []types.SymbolID{5}
	idx.categoryIndex["endpoint"] = []types.SymbolID{5}
	idx.symbolDepsMap[5] = []core.Dependency{
		{Type: "database", Name: "users"},
	}

	builder := NewSearchQueryBuilder(idx)
	builder.WithLabel("critical").
		WithCategory("endpoint").
		WithDependency("database", "users")

	results := builder.Execute()

	if len(results) != 1 {
		t.Errorf("Expected 1 result (AND logic), got %d", len(results))
	}

	if results[0] != types.SymbolID(5) {
		t.Errorf("Expected symbolID 5, got %d", results[0])
	}
}

func TestSearchQueryBuilderEmptyFilters(t *testing.T) {
	idx := NewAnnotationSearchIndex(nil)

	builder := NewSearchQueryBuilder(idx)
	// Don't add any filters

	results := builder.Execute()

	if results != nil {
		t.Errorf("Expected nil for empty filters, got %v", results)
	}
}

func BenchmarkGetSymbolsByLabel(b *testing.B) {
	idx := NewAnnotationSearchIndex(nil)

	// Add 1000 symbols for different labels
	for i := 0; i < 1000; i++ {
		label := "critical"
		if i%2 == 0 {
			label = "security"
		}
		idx.labelIndex[label] = append(idx.labelIndex[label], types.SymbolID(i))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = idx.GetSymbolsByLabel("critical")
	}
}

func BenchmarkSearchQueryBuilder(b *testing.B) {
	idx := NewAnnotationSearchIndex(nil)

	// Add test data
	for i := 1; i <= 100; i++ {
		if i%2 == 0 {
			idx.labelIndex["critical"] = append(idx.labelIndex["critical"], types.SymbolID(i))
		}
		if i%3 == 0 {
			idx.categoryIndex["endpoint"] = append(idx.categoryIndex["endpoint"], types.SymbolID(i))
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		builder := NewSearchQueryBuilder(idx)
		builder.WithLabel("critical").WithCategory("endpoint")
		_ = builder.Execute()
	}
}
