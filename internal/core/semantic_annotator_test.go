package core

import (
	"github.com/standardbeagle/lci/internal/types"
	"testing"
)

// TestSemanticAnnotator_BasicExtraction ensures single-line annotations are parsed correctly
func TestSemanticAnnotator_BasicExtraction(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(1)
	content := `// @lci:labels[api,public]\n// @lci:category[endpoint]\n// @lci:deps[db:users:read]\nfunc GetUser(id int) {}`
	symbols := []types.Symbol{{Name: "GetUser", Line: 4, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "user.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	ann := sa.GetAnnotation(fileID, symbolID)
	if ann == nil {
		t.Fatalf("expected annotation for symbol")
	}
	if len(ann.Labels) != 2 || ann.Category != "endpoint" {
		t.Fatalf("unexpected annotation %#v", ann)
	}
	if len(ann.Dependencies) != 1 || ann.Dependencies[0].Type != "db" {
		t.Fatalf("dep parse failed: %#v", ann.Dependencies)
	}
}

// TestSemanticAnnotator_MetricsAndAttributes verifies parsing of numeric, bool and JSON values
func TestSemanticAnnotator_MetricsAndAttributes(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(2)
	content := `// @lci:metrics[complexity=5,perf=0.87,hot=true]\n// @lci:attr[retries=3,meta={"tier":"gold"}]\nfunc Process() {}`
	symbols := []types.Symbol{{Name: "Process", Line: 3, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "proc.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	ann := sa.GetAnnotation(fileID, symbolID)
	if ann == nil {
		t.Fatalf("expected annotation")
	}
	if _, ok := ann.Metrics["complexity"]; !ok {
		t.Fatalf("missing metric complexity")
	}
	if v, ok := ann.Attributes["retries"].(int); !ok || v != 3 {
		t.Fatalf("expected retries=3 got %#v", ann.Attributes["retries"])
	}
}

// TestSemanticAnnotator_PropagationRule validates parsing of propagation rule syntax
func TestSemanticAnnotator_PropagationRule(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(3)
	content := `// @lci:labels[perf]
// @lci:propagate[attribute=latency,direction=downstream,decay=0.5,max_hops=5,aggregation=max]
func Handle() {}`
	symbols := []types.Symbol{{Name: "Handle", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "handle.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	ann := sa.GetAnnotation(fileID, symbolID)
	if ann == nil || len(ann.PropagationRules) != 1 {
		t.Fatalf("expected one propagation rule")
	}
	rule := ann.PropagationRules[0]
	if rule.Attribute != "latency" || rule.Direction != "downstream" || rule.MaxHops != 5 {
		t.Fatalf("unexpected rule %#v", rule)
	}
}

// TestSemanticAnnotator_DependencyGraphGeneration ensures dependency graph creation from annotations
func TestSemanticAnnotator_DependencyGraphGeneration(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(4)
	content := `// @lci:labels[svc]
// @lci:deps[service:auth:read,db:users:write]
func AuthUser() {}`
	symbols := []types.Symbol{{Name: "AuthUser", Line: 3, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "auth.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	graph := sa.GetDependencyGraph()
	if len(graph) != 1 {
		t.Fatalf("expected 1 entry in dependency graph, got %d", len(graph))
	}
	for _, deps := range graph {
		if len(deps) != 2 {
			t.Fatalf("expected 2 deps got %d", len(deps))
		}
	}
}

// TestSemanticAnnotator_ExcludeAnnotation verifies parsing of @lci:exclude directives
func TestSemanticAnnotator_ExcludeAnnotation(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(5)
	content := `// @lci:labels[benchmark]
// @lci:exclude[memory]
func BenchmarkHeavyAllocation() {}`
	symbols := []types.Symbol{{Name: "BenchmarkHeavyAllocation", Line: 3, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "bench.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	ann := sa.GetAnnotation(fileID, symbolID)
	if ann == nil {
		t.Fatalf("expected annotation for symbol")
	}
	if len(ann.Excludes) != 1 || ann.Excludes[0] != "memory" {
		t.Fatalf("expected Excludes=[memory], got %v", ann.Excludes)
	}
}

// TestSemanticAnnotator_ExcludeMultiple verifies parsing multiple exclusion types
func TestSemanticAnnotator_ExcludeMultiple(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(6)
	content := `// @lci:exclude[memory,complexity,duplicates]
func TestHelper() {}`
	symbols := []types.Symbol{{Name: "TestHelper", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "helper.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	ann := sa.GetAnnotation(fileID, symbolID)
	if ann == nil {
		t.Fatalf("expected annotation for symbol")
	}
	if len(ann.Excludes) != 3 {
		t.Fatalf("expected 3 excludes, got %d: %v", len(ann.Excludes), ann.Excludes)
	}
	expectedExcludes := map[string]bool{"memory": true, "complexity": true, "duplicates": true}
	for _, ex := range ann.Excludes {
		if !expectedExcludes[ex] {
			t.Fatalf("unexpected exclude: %s", ex)
		}
	}
}

// TestSemanticAnnotator_ExcludeAll verifies @lci:exclude[all] directive
func TestSemanticAnnotator_ExcludeAll(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(7)
	content := `// @lci:exclude[all]
func GeneratedCode() {}`
	symbols := []types.Symbol{{Name: "GeneratedCode", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "generated.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	ann := sa.GetAnnotation(fileID, symbolID)
	if ann == nil {
		t.Fatalf("expected annotation for symbol")
	}
	if len(ann.Excludes) != 1 || ann.Excludes[0] != "all" {
		t.Fatalf("expected Excludes=[all], got %v", ann.Excludes)
	}
}

// TestSemanticAnnotator_IsExcludedFromAnalysis tests the exclusion check function
func TestSemanticAnnotator_IsExcludedFromAnalysis(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(8)
	// Use actual newlines - symbols need to be far apart (>10 lines) to avoid annotation bleeding
	// The extractor looks up to 10 lines before each symbol for annotations
	content := "// @lci:exclude[memory]\nfunc ExcludedFromMemory() {}\n\n\n\n\n\n\n\n\n\n\n// @lci:exclude[all]\nfunc ExcludedFromAll() {}\n\n\n\n\n\n\n\n\n\n\n// @lci:labels[normal]\nfunc NormalFunction() {}"
	symbols := []types.Symbol{
		{Name: "ExcludedFromMemory", Line: 2, Column: 1},
		{Name: "ExcludedFromAll", Line: 14, Column: 1}, // 12 lines after previous
		{Name: "NormalFunction", Line: 26, Column: 1},  // 12 lines after previous
	}
	if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}

	memoryExcludedID := types.SymbolID(fileID)<<32 | types.SymbolID(2)<<16 | types.SymbolID(1)
	allExcludedID := types.SymbolID(fileID)<<32 | types.SymbolID(14)<<16 | types.SymbolID(1)
	normalID := types.SymbolID(fileID)<<32 | types.SymbolID(26)<<16 | types.SymbolID(1)

	// Test memory exclusion
	if !sa.IsExcludedFromAnalysis(fileID, memoryExcludedID, "memory") {
		t.Error("ExcludedFromMemory should be excluded from memory analysis")
	}
	if sa.IsExcludedFromAnalysis(fileID, memoryExcludedID, "complexity") {
		t.Error("ExcludedFromMemory should NOT be excluded from complexity analysis")
	}

	// Test all exclusion
	if !sa.IsExcludedFromAnalysis(fileID, allExcludedID, "memory") {
		t.Error("ExcludedFromAll should be excluded from memory analysis")
	}
	if !sa.IsExcludedFromAnalysis(fileID, allExcludedID, "complexity") {
		t.Error("ExcludedFromAll should be excluded from complexity analysis")
	}
	if !sa.IsExcludedFromAnalysis(fileID, allExcludedID, "anything") {
		t.Error("ExcludedFromAll should be excluded from any analysis")
	}

	// Test normal function (has labels but no excludes)
	if sa.IsExcludedFromAnalysis(fileID, normalID, "memory") {
		t.Error("NormalFunction should NOT be excluded from memory analysis")
	}
}

// TestIsExcludedFromAnalysisByAnnotation tests the standalone function
func TestIsExcludedFromAnalysisByAnnotation(t *testing.T) {
	tests := []struct {
		name         string
		excludes     []string
		analysisType string
		expected     bool
	}{
		{"nil annotation", nil, "memory", false},
		{"empty excludes", []string{}, "memory", false},
		{"exact match", []string{"memory"}, "memory", true},
		{"no match", []string{"complexity"}, "memory", false},
		{"all excludes everything", []string{"all"}, "memory", true},
		{"all excludes complexity", []string{"all"}, "complexity", true},
		{"case insensitive match", []string{"MEMORY"}, "memory", true},
		{"case insensitive all", []string{"ALL"}, "memory", true},
		{"multiple with match", []string{"complexity", "memory"}, "memory", true},
		{"multiple without match", []string{"complexity", "duplicates"}, "memory", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ann *SemanticAnnotation
			if tt.excludes != nil {
				ann = &SemanticAnnotation{Excludes: tt.excludes}
			}
			result := IsExcludedFromAnalysisByAnnotation(ann, tt.analysisType)
			if result != tt.expected {
				t.Errorf("IsExcludedFromAnalysisByAnnotation(%v, %q) = %v, want %v",
					tt.excludes, tt.analysisType, result, tt.expected)
			}
		})
	}
}

// TestSemanticAnnotator_GetExcludedSymbols tests retrieving all excluded symbols
func TestSemanticAnnotator_GetExcludedSymbols(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(9)
	// Space symbols far apart (>10 lines) to avoid annotation bleeding
	content := "// @lci:exclude[memory]\nfunc Excluded1() {}\n\n\n\n\n\n\n\n\n\n\n// @lci:exclude[memory,complexity]\nfunc Excluded2() {}\n\n\n\n\n\n\n\n\n\n\nfunc Normal() {}\n\n\n\n\n\n\n\n\n\n\n// @lci:exclude[complexity]\nfunc ExcludedComplexityOnly() {}"
	symbols := []types.Symbol{
		{Name: "Excluded1", Line: 2, Column: 1},
		{Name: "Excluded2", Line: 14, Column: 1},
		{Name: "Normal", Line: 26, Column: 1},
		{Name: "ExcludedComplexityOnly", Line: 38, Column: 1},
	}
	if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}

	// Get symbols excluded from memory analysis (Excluded1 and Excluded2)
	memoryExcluded := sa.GetExcludedSymbols("memory")
	if len(memoryExcluded) != 2 {
		t.Errorf("expected 2 symbols excluded from memory, got %d", len(memoryExcluded))
	}

	// Get symbols excluded from complexity analysis (Excluded2 and ExcludedComplexityOnly)
	complexityExcluded := sa.GetExcludedSymbols("complexity")
	if len(complexityExcluded) != 2 {
		t.Errorf("expected 2 symbols excluded from complexity, got %d", len(complexityExcluded))
	}
}

// TestSemanticAnnotator_MemoryHintsLoopWeight tests parsing of @lci:loop-weight annotation
func TestSemanticAnnotator_MemoryHintsLoopWeight(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(20)
	content := `// @lci:loop-weight[3.5]
func RetryLoop() {}`
	symbols := []types.Symbol{{Name: "RetryLoop", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "retry.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	hints := sa.GetMemoryHints(fileID, symbolID)
	if hints == nil {
		t.Fatal("expected memory hints")
	}
	if hints.LoopWeight != 3.5 {
		t.Errorf("expected LoopWeight=3.5, got %.2f", hints.LoopWeight)
	}
	if !hints.HasAnnotation {
		t.Error("expected HasAnnotation=true")
	}
}

// TestSemanticAnnotator_MemoryHintsLoopBounded tests parsing of @lci:loop-bounded annotation
func TestSemanticAnnotator_MemoryHintsLoopBounded(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(21)
	content := `// @lci:loop-bounded[5]
func RetryWithMax() {}`
	symbols := []types.Symbol{{Name: "RetryWithMax", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "retry.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	hints := sa.GetMemoryHints(fileID, symbolID)
	if hints == nil {
		t.Fatal("expected memory hints")
	}
	if hints.LoopBounded != 5 {
		t.Errorf("expected LoopBounded=5, got %d", hints.LoopBounded)
	}
}

// TestSemanticAnnotator_MemoryHintsCallFrequency tests parsing of @lci:call-frequency annotation
func TestSemanticAnnotator_MemoryHintsCallFrequency(t *testing.T) {
	validFrequencies := []string{
		"hot-path", "once-per-file", "once-per-request",
		"once-per-session", "startup-only", "cli-output",
		"test-only", "rare",
	}

	for i, freq := range validFrequencies {
		t.Run(freq, func(t *testing.T) {
			sa := NewSemanticAnnotator()
			fileID := types.FileID(30 + i)
			content := "// @lci:call-frequency[" + freq + "]\nfunc Func() {}"
			symbols := []types.Symbol{{Name: "Func", Line: 2, Column: 1}}
			if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
				t.Fatalf("extract: %v", err)
			}
			symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
			hints := sa.GetMemoryHints(fileID, symbolID)
			if hints == nil {
				t.Fatal("expected memory hints")
			}
			if hints.CallFrequency != freq {
				t.Errorf("expected CallFrequency=%q, got %q", freq, hints.CallFrequency)
			}
		})
	}
}

// TestSemanticAnnotator_MemoryHintsCallFrequencyInvalid tests that invalid call-frequency values are ignored
func TestSemanticAnnotator_MemoryHintsCallFrequencyInvalid(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(50)
	content := `// @lci:call-frequency[invalid-value]
func Func() {}`
	symbols := []types.Symbol{{Name: "Func", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	hints := sa.GetMemoryHints(fileID, symbolID)
	// Should have nil hints because only invalid annotation was present
	if hints != nil {
		t.Errorf("expected nil hints for invalid call-frequency, got %+v", hints)
	}
}

// TestSemanticAnnotator_MemoryHintsPropagationWeight tests parsing of @lci:propagation-weight annotation
func TestSemanticAnnotator_MemoryHintsPropagationWeight(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.5", 0.5},
		{"0.0", 0.0},
		{"1.0", 1.0},
		{"0.85", 0.85},
	}

	for i, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sa := NewSemanticAnnotator()
			fileID := types.FileID(60 + i)
			content := "// @lci:propagation-weight[" + tt.input + "]\nfunc Func() {}"
			symbols := []types.Symbol{{Name: "Func", Line: 2, Column: 1}}
			if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
				t.Fatalf("extract: %v", err)
			}
			symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
			hints := sa.GetMemoryHints(fileID, symbolID)
			if hints == nil {
				t.Fatal("expected memory hints")
			}
			if hints.PropagationWeight != tt.expected {
				t.Errorf("expected PropagationWeight=%.2f, got %.2f", tt.expected, hints.PropagationWeight)
			}
		})
	}
}

// TestSemanticAnnotator_MemoryHintsPropagationWeightClamping tests that out-of-range values are clamped
func TestSemanticAnnotator_MemoryHintsPropagationWeightClamping(t *testing.T) {
	// Test values that ARE valid syntax but need clamping (values > 1)
	tests := []struct {
		input    string
		expected float64
	}{
		{"1.5", 1.0}, // >1 clamped to 1
		{"100", 1.0}, // Way over clamped to 1
	}

	for i, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sa := NewSemanticAnnotator()
			fileID := types.FileID(70 + i)
			content := "// @lci:propagation-weight[" + tt.input + "]\nfunc Func() {}"
			symbols := []types.Symbol{{Name: "Func", Line: 2, Column: 1}}
			if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
				t.Fatalf("extract: %v", err)
			}
			symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
			hints := sa.GetMemoryHints(fileID, symbolID)
			if hints == nil {
				t.Fatal("expected memory hints")
			}
			if hints.PropagationWeight != tt.expected {
				t.Errorf("expected PropagationWeight=%.2f, got %.2f", tt.expected, hints.PropagationWeight)
			}
		})
	}
}

// TestSemanticAnnotator_MemoryHintsInvalidSyntax tests that invalid annotation syntax is ignored
func TestSemanticAnnotator_MemoryHintsInvalidSyntax(t *testing.T) {
	// Negative numbers are not valid syntax (regex doesn't match them)
	sa := NewSemanticAnnotator()
	fileID := types.FileID(75)
	content := "// @lci:propagation-weight[-0.5]\nfunc Func() {}"
	symbols := []types.Symbol{{Name: "Func", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "test.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	hints := sa.GetMemoryHints(fileID, symbolID)
	// Invalid syntax should not create memory hints
	if hints != nil {
		t.Error("expected nil hints for invalid syntax")
	}
}

// TestSemanticAnnotator_MemoryHintsCombined tests multiple memory hints on one symbol
func TestSemanticAnnotator_MemoryHintsCombined(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(80)
	content := `// @lci:loop-bounded[3]
// @lci:call-frequency[cli-output]
// @lci:propagation-weight[0.2]
func DisplayResults() {}`
	symbols := []types.Symbol{{Name: "DisplayResults", Line: 4, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "display.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	hints := sa.GetMemoryHints(fileID, symbolID)
	if hints == nil {
		t.Fatal("expected memory hints")
	}
	if hints.LoopBounded != 3 {
		t.Errorf("expected LoopBounded=3, got %d", hints.LoopBounded)
	}
	if hints.CallFrequency != "cli-output" {
		t.Errorf("expected CallFrequency=cli-output, got %q", hints.CallFrequency)
	}
	if hints.PropagationWeight != 0.2 {
		t.Errorf("expected PropagationWeight=0.2, got %.2f", hints.PropagationWeight)
	}
	if !hints.HasAnnotation {
		t.Error("expected HasAnnotation=true")
	}
}

// TestSemanticAnnotator_GetMemoryHintsNil tests that GetMemoryHints returns nil for symbols without hints
func TestSemanticAnnotator_GetMemoryHintsNil(t *testing.T) {
	sa := NewSemanticAnnotator()
	fileID := types.FileID(90)
	content := `// @lci:labels[api]
func NormalFunc() {}`
	symbols := []types.Symbol{{Name: "NormalFunc", Line: 2, Column: 1}}
	if err := sa.ExtractAnnotations(fileID, "normal.go", content, symbols); err != nil {
		t.Fatalf("extract: %v", err)
	}
	symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbols[0].Line)<<16 | types.SymbolID(symbols[0].Column)
	hints := sa.GetMemoryHints(fileID, symbolID)
	if hints != nil {
		t.Errorf("expected nil hints for symbol without memory annotations, got %+v", hints)
	}
}

// TestIsValidCallFrequency tests the call frequency validation function
func TestIsValidCallFrequency(t *testing.T) {
	valid := []string{
		"hot-path", "once-per-file", "once-per-request",
		"once-per-session", "startup-only", "cli-output",
		"test-only", "rare",
	}
	invalid := []string{
		"", "unknown", "hot", "cold", "sometimes",
		"HOT-PATH", // Case sensitive
	}

	for _, freq := range valid {
		if !isValidCallFrequency(freq) {
			t.Errorf("isValidCallFrequency(%q) should be true", freq)
		}
	}

	for _, freq := range invalid {
		if isValidCallFrequency(freq) {
			t.Errorf("isValidCallFrequency(%q) should be false", freq)
		}
	}
}
