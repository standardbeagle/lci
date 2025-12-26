package analysis

import (
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestMemoryAnalyzer_BasicAnalysis(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:      "allocateHeavily",
			FilePath:  "test.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 2, EndLine: 8, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
				{Target: "append(slice, item)", Line: 4, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "simpleFunc",
			FilePath:  "test.go",
			StartLine: 12,
			EndLine:   15,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "fmt.Println(x)", Line: 13, InLoop: false},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should have 2 functions
	if len(result.Functions) != 2 {
		t.Errorf("Expected 2 functions, got %d", len(result.Functions))
	}

	// Should have 2 scores
	if len(result.Scores) != 2 {
		t.Errorf("Expected 2 scores, got %d", len(result.Scores))
	}

	// allocateHeavily should have higher score than simpleFunc
	var heavyScore, simpleScore float64
	for _, score := range result.Scores {
		if score.FunctionName == "allocateHeavily" {
			heavyScore = score.TotalScore
		}
		if score.FunctionName == "simpleFunc" {
			simpleScore = score.TotalScore
		}
	}

	if heavyScore <= simpleScore {
		t.Errorf("Expected allocateHeavily (%.2f) to have higher score than simpleFunc (%.2f)",
			heavyScore, simpleScore)
	}

	// Summary should reflect the analysis
	if result.Summary.TotalFunctions != 2 {
		t.Errorf("Expected 2 total functions in summary, got %d", result.Summary.TotalFunctions)
	}

	t.Logf("Memory Analysis Results:")
	t.Logf("  Total Functions: %d", result.Summary.TotalFunctions)
	t.Logf("  Total Allocations: %d", result.Summary.TotalAllocations)
	t.Logf("  Critical: %d, High: %d, Medium: %d, Low: %d",
		result.Summary.CriticalCount, result.Summary.HighCount,
		result.Summary.MediumCount, result.Summary.LowCount)

	for _, score := range result.Scores {
		t.Logf("  %s: total=%.2f (direct=%.2f, propagated=%.2f) [%s]",
			score.FunctionName, score.TotalScore, score.DirectScore,
			score.PropagatedScore, score.Severity)
	}
}

func TestMemoryAnalyzer_PageRankPropagation(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create a call chain: A -> B -> C (C allocates heavily)
	functions := []*FunctionAnalysis{
		{
			Name:      "funcA",
			FilePath:  "test.go",
			StartLine: 1,
			EndLine:   5,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "funcB", Line: 2, InLoop: false},
			},
		},
		{
			Name:      "funcB",
			FilePath:  "test.go",
			StartLine: 7,
			EndLine:   12,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "funcC", Line: 8, InLoop: false},
			},
		},
		{
			Name:      "funcC",
			FilePath:  "test.go",
			StartLine: 14,
			EndLine:   25,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 15, EndLine: 23, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 16, InLoop: true, LoopDepth: 1},
				{Target: "json.Marshal(data)", Line: 17, InLoop: true, LoopDepth: 1},
				{Target: "regexp.MustCompile(pattern)", Line: 18, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	// funcC should have highest direct score
	var scoreA, scoreB, scoreC float64
	for _, score := range result.Scores {
		switch score.FunctionName {
		case "funcA":
			scoreA = score.TotalScore
		case "funcB":
			scoreB = score.TotalScore
		case "funcC":
			scoreC = score.TotalScore
		}
	}

	t.Logf("Scores: A=%.2f, B=%.2f, C=%.2f", scoreA, scoreB, scoreC)

	// C should have highest score (direct allocations)
	if scoreC < scoreA || scoreC < scoreB {
		t.Errorf("Expected funcC to have highest score")
	}
}

func TestMemoryAnalyzer_JavaScriptPatterns(t *testing.T) {
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:      "processData",
			FilePath:  "app.js",
			StartLine: 1,
			EndLine:   15,
			Language:  "javascript",
			Loops: []LoopInfo{
				{NodeType: "for_of_statement", StartLine: 2, EndLine: 10, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "items.map(transform)", Line: 3, InLoop: true, LoopDepth: 1},
				{Target: "JSON.parse(data)", Line: 4, InLoop: true, LoopDepth: 1},
				{Target: "new RegExp(pattern)", Line: 5, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if len(result.Scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(result.Scores))
	}

	score := result.Scores[0]
	t.Logf("JavaScript function score: %.2f (loop pressure: %.2f)", score.TotalScore, score.LoopPressure)

	// Should have significant loop pressure
	if score.LoopPressure == 0 {
		t.Error("Expected non-zero loop pressure for allocations in loop")
	}
}

func TestMemoryAnalyzer_AnalyzeFromPerfData(t *testing.T) {
	ma := NewMemoryAnalyzer()

	files := []*types.FileInfo{
		{
			Path: "test.go",
			PerfData: []types.FunctionPerfData{
				{
					Name:      "processItems",
					StartLine: 1,
					EndLine:   20,
					Language:  "go",
					Loops: []types.LoopData{
						{NodeType: "for_range_statement", StartLine: 3, EndLine: 15, Depth: 1},
					},
					Calls: []types.CallData{
						{Target: "make([]byte, size)", Line: 5, InLoop: true, LoopDepth: 1},
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
		t.Errorf("Expected 1 score, got %d", len(result.Scores))
	}

	if result.Summary.TotalFunctions != 1 {
		t.Errorf("Expected 1 function, got %d", result.Summary.TotalFunctions)
	}

	t.Logf("PerfData analysis: %d functions, %d allocations",
		result.Summary.TotalFunctions, result.Summary.TotalAllocations)
}

func TestMemoryAnalyzer_HotspotIdentification(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create functions with varying allocation levels
	functions := make([]*FunctionAnalysis, 10)
	for i := 0; i < 10; i++ {
		calls := make([]CallInfo, i+1)
		for j := 0; j <= i; j++ {
			calls[j] = CallInfo{
				Target:    "make([]byte, 1024)",
				Line:      j + 2,
				InLoop:    i > 5, // Higher indexed functions have loop allocations
				LoopDepth: 1,
			}
		}

		var loops []LoopInfo
		if i > 5 {
			loops = []LoopInfo{
				{NodeType: "for_statement", StartLine: 1, EndLine: i + 3, Depth: 1},
			}
		}

		functions[i] = &FunctionAnalysis{
			Name:      "func" + string(rune('A'+i)),
			FilePath:  "test.go",
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Language:  "go",
			Loops:     loops,
			Calls:     calls,
		}
	}

	result := ma.AnalyzeFunctions(functions)

	t.Logf("Hotspots identified: %d", len(result.Hotspots))
	for _, hs := range result.Hotspots {
		t.Logf("  %s: score=%.2f reason=%s", hs.FunctionName, hs.Score, hs.Reason)
	}

	// Should have identified some hotspots (high allocation functions)
	if len(result.Hotspots) == 0 && result.Summary.CriticalCount+result.Summary.HighCount > 0 {
		t.Error("Expected hotspots to be identified for high/critical functions")
	}
}

func TestMemoryAnalyzer_EmptyInput(t *testing.T) {
	ma := NewMemoryAnalyzer()

	result := ma.AnalyzeFunctions([]*FunctionAnalysis{})

	if result == nil {
		t.Fatal("Expected non-nil result even for empty input")
	}

	if len(result.Functions) != 0 {
		t.Errorf("Expected 0 functions, got %d", len(result.Functions))
	}

	if result.Summary.TotalFunctions != 0 {
		t.Errorf("Expected 0 total functions, got %d", result.Summary.TotalFunctions)
	}
}

func TestAllocationPatterns(t *testing.T) {
	ma := NewMemoryAnalyzer()

	tests := []struct {
		language string
		pattern  string
		expected bool
	}{
		{"go", "make([]byte, 100)", true},
		{"go", "new(MyStruct)", true},
		{"go", "json.Marshal(data)", true},
		{"javascript", "new Object()", true},
		{"javascript", "items.map(fn)", true},
		{"javascript", "JSON.parse(str)", true},
		{"python", "list(items)", true},
		{"rust", "Box::new(value)", true},
		{"rust", "Vec::new()", true},
	}

	for _, tt := range tests {
		patterns := ma.allocationPatterns[tt.language]
		found := false
		for _, p := range patterns {
			if p.Pattern.MatchString(tt.pattern) {
				found = true
				break
			}
		}
		if found != tt.expected {
			t.Errorf("Language %s, pattern %q: expected match=%v, got %v",
				tt.language, tt.pattern, tt.expected, found)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Go test files
		{"memory_analyzer_test.go", true},
		{"internal/analysis/memory_analyzer_test.go", true},
		{"memory_analyzer.go", false},

		// JavaScript/TypeScript test files
		{"component.test.js", true},
		{"component.spec.js", true},
		{"component.test.ts", true},
		{"component.spec.tsx", true},
		{"component_test.js", true},
		{"component.js", false},
		{"src/__tests__/component.js", true},

		// Python test files
		{"test_memory.py", true},
		{"memory_test.py", true},
		{"memory.py", false},
		{"tests/test_helper.py", true},

		// Java test files
		{"MemoryAnalyzerTest.java", true},
		{"MemoryAnalyzerTests.java", true},
		{"MemoryAnalyzerIT.java", true},
		{"MemoryAnalyzer.java", false},
		{"src/test/java/MemoryAnalyzer.java", true},

		// C# test files
		{"MemoryAnalyzerTest.cs", true},
		{"MemoryAnalyzerTests.cs", true},
		{"MemoryAnalyzer.cs", false},

		// Rust test files
		{"tests/integration.rs", true},
		{"memory_test.rs", true},
		{"memory.rs", false},

		// Ruby test files
		{"memory_test.rb", true},
		{"memory_spec.rb", true},
		{"memory.rb", false},

		// PHP test files
		{"MemoryAnalyzerTest.php", true},
		{"MemoryAnalyzer.php", false},

		// Test directories
		{"/project/tests/helper.go", true},
		{"/project/test/helper.go", true},
		{"/project/__tests__/helper.js", true},
		{"/project/spec/helper.rb", true},
		{"/project/src/helper.go", false},

		// NEW: Fixtures and testdata directories (previously not detected)
		{"/project/fixtures/sample_data.go", true},
		{"/project/internal/testing/fixtures/large_file.go", true},
		{"/project/testdata/sample.go", true},
		{"/project/internal/testutil/helpers.go", true},
		{"/project/mocks/mock_service.go", true},
		{"/project/stubs/stub_client.go", true},
		{"/project/internal/testing/helpers.go", true},

		// Benchmark directories
		{"/project/benchmarks/perf.go", true},
		{"/project/bench/memory.go", true},
	}

	for _, tt := range tests {
		got := isTestFile(tt.path)
		if got != tt.expected {
			t.Errorf("isTestFile(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func TestMemoryAnalyzer_TestFilePenalty(t *testing.T) {
	// Create functions with identical profiles, one in test file, one in production
	functions := []*FunctionAnalysis{
		{
			Name:      "productionFunc",
			FilePath:  "memory.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 2, EndLine: 8, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "testFunc",
			FilePath:  "memory_test.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 2, EndLine: 8, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	// Test with default options (test files penalized)
	ma := NewMemoryAnalyzer()
	result := ma.AnalyzeFunctions(functions)

	var prodScore, testScore float64
	var prodIsTest, testIsTest bool
	for _, score := range result.Scores {
		if score.FunctionName == "productionFunc" {
			prodScore = score.TotalScore
			prodIsTest = score.IsTestFile
		}
		if score.FunctionName == "testFunc" {
			testScore = score.TotalScore
			testIsTest = score.IsTestFile
		}
	}

	// Verify IsTestFile detection
	if prodIsTest {
		t.Error("productionFunc should not be marked as test file")
	}
	if !testIsTest {
		t.Error("testFunc should be marked as test file")
	}

	// Test file should have much lower score due to penalty
	expectedRatio := 0.1 // Default penalty
	actualRatio := testScore / prodScore
	if actualRatio > expectedRatio*1.1 || actualRatio < expectedRatio*0.9 {
		t.Errorf("Test file score ratio = %.2f, expected ~%.2f (prod=%.2f, test=%.2f)",
			actualRatio, expectedRatio, prodScore, testScore)
	}

	t.Logf("Default options: prod=%.2f, test=%.2f (ratio=%.2f)", prodScore, testScore, actualRatio)
}

func TestMemoryAnalyzer_IncludeTestFiles(t *testing.T) {
	functions := []*FunctionAnalysis{
		{
			Name:      "productionFunc",
			FilePath:  "memory.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "testFunc",
			FilePath:  "memory_test.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	// Test with IncludeTestFiles=true (no penalty)
	options := MemoryAnalyzerOptions{
		IncludeTestFiles: true,
		TestFilePenalty:  0.1, // Won't be applied
	}
	ma := NewMemoryAnalyzerWithOptions(options)
	result := ma.AnalyzeFunctions(functions)

	var prodScore, testScore float64
	for _, score := range result.Scores {
		if score.FunctionName == "productionFunc" {
			prodScore = score.TotalScore
		}
		if score.FunctionName == "testFunc" {
			testScore = score.TotalScore
		}
	}

	// With IncludeTestFiles=true, scores should be equal
	if prodScore != testScore {
		t.Errorf("With IncludeTestFiles=true, scores should be equal: prod=%.2f, test=%.2f",
			prodScore, testScore)
	}

	t.Logf("IncludeTestFiles=true: prod=%.2f, test=%.2f", prodScore, testScore)
}

func TestMemoryAnalyzer_CustomTestFilePenalty(t *testing.T) {
	functions := []*FunctionAnalysis{
		{
			Name:      "productionFunc",
			FilePath:  "memory.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "testFunc",
			FilePath:  "memory_test.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	// Test with custom penalty (0.5 = 50%)
	options := MemoryAnalyzerOptions{
		IncludeTestFiles: false,
		TestFilePenalty:  0.5,
	}
	ma := NewMemoryAnalyzerWithOptions(options)
	result := ma.AnalyzeFunctions(functions)

	var prodScore, testScore float64
	for _, score := range result.Scores {
		if score.FunctionName == "productionFunc" {
			prodScore = score.TotalScore
		}
		if score.FunctionName == "testFunc" {
			testScore = score.TotalScore
		}
	}

	// Test file should have 50% of production score
	expectedRatio := 0.5
	actualRatio := testScore / prodScore
	if actualRatio > expectedRatio*1.1 || actualRatio < expectedRatio*0.9 {
		t.Errorf("Custom penalty: ratio = %.2f, expected ~%.2f (prod=%.2f, test=%.2f)",
			actualRatio, expectedRatio, prodScore, testScore)
	}

	t.Logf("Custom penalty (0.5): prod=%.2f, test=%.2f (ratio=%.2f)", prodScore, testScore, actualRatio)
}

func TestMemoryAnalyzer_SetOptions(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Verify default options
	opts := ma.GetOptions()
	if opts.IncludeTestFiles {
		t.Error("Default should have IncludeTestFiles=false")
	}
	if opts.TestFilePenalty != 0.1 {
		t.Errorf("Default TestFilePenalty should be 0.1, got %.2f", opts.TestFilePenalty)
	}

	// Set new options
	ma.SetOptions(MemoryAnalyzerOptions{
		IncludeTestFiles: true,
		TestFilePenalty:  0.5,
	})

	opts = ma.GetOptions()
	if !opts.IncludeTestFiles {
		t.Error("SetOptions should have set IncludeTestFiles=true")
	}
	if opts.TestFilePenalty != 0.5 {
		t.Errorf("SetOptions should have set TestFilePenalty=0.5, got %.2f", opts.TestFilePenalty)
	}
}

func TestMemoryAnalyzer_CustomTestFilePatterns(t *testing.T) {
	// Create custom patterns that include benchmark files
	customPatterns := &TestFilePatterns{
		SuffixPatterns: map[string][]string{
			".go": {"_test", "_bench", "_benchmark"},
		},
		PrefixPatterns: map[string][]string{},
		DirectoryPatterns: []string{
			"/tests/",
			"/benchmarks/",
		},
	}

	functions := []*FunctionAnalysis{
		{
			Name:      "productionFunc",
			FilePath:  "memory.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "testFunc",
			FilePath:  "memory_test.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "benchFunc",
			FilePath:  "memory_bench.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "benchmarkFunc",
			FilePath:  "benchmarks/perf.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	options := MemoryAnalyzerOptions{
		IncludeTestFiles: false,
		TestFilePenalty:  0.1,
		TestFilePatterns: customPatterns,
	}
	ma := NewMemoryAnalyzerWithOptions(options)
	result := ma.AnalyzeFunctions(functions)

	var prodScore, testScore, benchScore, benchmarkScore float64
	var testIsTest, benchIsTest, benchmarkIsTest bool

	for _, score := range result.Scores {
		switch score.FunctionName {
		case "productionFunc":
			prodScore = score.TotalScore
		case "testFunc":
			testScore = score.TotalScore
			testIsTest = score.IsTestFile
		case "benchFunc":
			benchScore = score.TotalScore
			benchIsTest = score.IsTestFile
		case "benchmarkFunc":
			benchmarkScore = score.TotalScore
			benchmarkIsTest = score.IsTestFile
		}
	}

	// Verify detection
	if !testIsTest {
		t.Error("testFunc should be detected as test file")
	}
	if !benchIsTest {
		t.Error("benchFunc should be detected as test file (custom _bench pattern)")
	}
	if !benchmarkIsTest {
		t.Error("benchmarkFunc should be detected as test file (custom /benchmarks/ directory)")
	}

	// All test/bench files should have penalty applied
	expectedRatio := 0.1
	for name, score := range map[string]float64{"testFunc": testScore, "benchFunc": benchScore, "benchmarkFunc": benchmarkScore} {
		ratio := score / prodScore
		if ratio > expectedRatio*1.1 || ratio < expectedRatio*0.9 {
			t.Errorf("%s ratio = %.2f, expected ~%.2f", name, ratio, expectedRatio)
		}
	}

	t.Logf("Custom patterns: prod=%.2f, test=%.2f, bench=%.2f, benchmark=%.2f",
		prodScore, testScore, benchScore, benchmarkScore)
}

func TestMemoryAnalyzer_CustomMatcher(t *testing.T) {
	// Create patterns with a custom matcher for special files
	customPatterns := &TestFilePatterns{
		SuffixPatterns:    DefaultTestFilePatterns().SuffixPatterns,
		PrefixPatterns:    DefaultTestFilePatterns().PrefixPatterns,
		DirectoryPatterns: DefaultTestFilePatterns().DirectoryPatterns,
		CustomMatcher: func(filePath string) bool {
			// Match any file with "mock" or "stub" in the name
			lower := strings.ToLower(filePath)
			return strings.Contains(lower, "mock") || strings.Contains(lower, "stub")
		},
	}

	functions := []*FunctionAnalysis{
		{
			Name:      "productionFunc",
			FilePath:  "service.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "mockFunc",
			FilePath:  "mock_service.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "stubFunc",
			FilePath:  "database_stub.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	options := MemoryAnalyzerOptions{
		IncludeTestFiles: false,
		TestFilePenalty:  0.1,
		TestFilePatterns: customPatterns,
	}
	ma := NewMemoryAnalyzerWithOptions(options)
	result := ma.AnalyzeFunctions(functions)

	var mockIsTest, stubIsTest bool
	for _, score := range result.Scores {
		switch score.FunctionName {
		case "mockFunc":
			mockIsTest = score.IsTestFile
		case "stubFunc":
			stubIsTest = score.IsTestFile
		}
	}

	if !mockIsTest {
		t.Error("mockFunc should be detected as test file via custom matcher")
	}
	if !stubIsTest {
		t.Error("stubFunc should be detected as test file via custom matcher")
	}

	t.Logf("Custom matcher: mockIsTest=%v, stubIsTest=%v", mockIsTest, stubIsTest)
}

func TestDefaultTestFilePatterns(t *testing.T) {
	patterns := DefaultTestFilePatterns()

	// Verify all expected extensions have patterns
	expectedExtensions := []string{".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".java", ".cs", ".rs", ".rb", ".php"}
	for _, ext := range expectedExtensions {
		if _, ok := patterns.SuffixPatterns[ext]; !ok {
			t.Errorf("Missing suffix patterns for extension %s", ext)
		}
	}

	// Verify Python has both prefix and suffix patterns
	if _, ok := patterns.PrefixPatterns[".py"]; !ok {
		t.Error("Python should have prefix patterns (test_)")
	}

	// Verify directory patterns
	expectedDirs := []string{"/tests/", "/test/", "/__tests__/"}
	for _, dir := range expectedDirs {
		found := false
		for _, d := range patterns.DirectoryPatterns {
			if d == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing directory pattern: %s", dir)
		}
	}
}

func TestIsTestFileWithPatterns(t *testing.T) {
	// Test with nil patterns (should return false for everything)
	if isTestFileWithPatterns("anything_test.go", nil) {
		t.Error("nil patterns should not match anything")
	}

	// Test with empty patterns
	emptyPatterns := &TestFilePatterns{}
	if isTestFileWithPatterns("anything_test.go", emptyPatterns) {
		t.Error("empty patterns should not match anything")
	}

	// Test with minimal patterns
	minimalPatterns := &TestFilePatterns{
		SuffixPatterns: map[string][]string{
			".go": {"_test"},
		},
	}
	if !isTestFileWithPatterns("foo_test.go", minimalPatterns) {
		t.Error("minimal patterns should match foo_test.go")
	}
	if isTestFileWithPatterns("foo.go", minimalPatterns) {
		t.Error("minimal patterns should not match foo.go")
	}
}

func TestMemoryAnalyzer_ExcludedFunctions(t *testing.T) {
	// Test that excluded functions get zero score and "excluded" severity
	ma := NewMemoryAnalyzer()

	// Create a mock exclusion checker
	mockChecker := &mockExclusionChecker{
		excludedSymbols: map[types.SymbolID]bool{
			types.SymbolID(1001): true, // excludedFunc
		},
	}
	ma.SetExclusionChecker(mockChecker)

	functions := []*FunctionAnalysis{
		{
			Name:      "normalFunc",
			FilePath:  "service.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			SymbolID:  types.SymbolID(1000),
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "excludedFunc",
			FilePath:  "benchmark.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			SymbolID:  types.SymbolID(1001),
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	var normalScore, excludedScore *MemoryPressureScore
	for i := range result.Scores {
		if result.Scores[i].FunctionName == "normalFunc" {
			normalScore = &result.Scores[i]
		}
		if result.Scores[i].FunctionName == "excludedFunc" {
			excludedScore = &result.Scores[i]
		}
	}

	if normalScore == nil {
		t.Fatal("normalFunc not found in scores")
	}
	if excludedScore == nil {
		t.Fatal("excludedFunc not found in scores")
	}

	// Normal function should have positive score
	if normalScore.TotalScore <= 0 {
		t.Error("normalFunc should have positive score")
	}
	if normalScore.IsExcluded {
		t.Error("normalFunc should not be marked as excluded")
	}
	if normalScore.Severity == "excluded" {
		t.Error("normalFunc should not have 'excluded' severity")
	}

	// Excluded function should have zero score and "excluded" severity
	if excludedScore.TotalScore != 0 {
		t.Errorf("excludedFunc should have zero score, got %.2f", excludedScore.TotalScore)
	}
	if !excludedScore.IsExcluded {
		t.Error("excludedFunc should be marked as excluded")
	}
	if excludedScore.Severity != "excluded" {
		t.Errorf("excludedFunc should have 'excluded' severity, got %s", excludedScore.Severity)
	}

	// Summary should count excluded functions
	if result.Summary.ExcludedCount != 1 {
		t.Errorf("Summary.ExcludedCount should be 1, got %d", result.Summary.ExcludedCount)
	}

	t.Logf("Exclusion test: normalScore=%.2f, excludedScore=%.2f, ExcludedCount=%d",
		normalScore.TotalScore, excludedScore.TotalScore, result.Summary.ExcludedCount)
}

func TestMemoryAnalyzer_ExcludedNotInHotspots(t *testing.T) {
	// Test that excluded high-allocation functions don't appear in hotspots
	ma := NewMemoryAnalyzer()

	// Create a mock exclusion checker that excludes high-allocation functions
	mockChecker := &mockExclusionChecker{
		excludedSymbols: map[types.SymbolID]bool{
			types.SymbolID(1001): true, // highAllocExcluded
		},
	}
	ma.SetExclusionChecker(mockChecker)

	// Create functions with varying allocation levels
	// Higher indexed functions have more allocations
	functions := make([]*FunctionAnalysis, 10)
	for i := 0; i < 10; i++ {
		calls := make([]CallInfo, i+5) // More calls = more allocations
		for j := 0; j < len(calls); j++ {
			calls[j] = CallInfo{
				Target:    "make([]byte, 1024)",
				Line:      j + 2,
				InLoop:    true,
				LoopDepth: 1,
			}
		}

		symbolID := types.SymbolID(1000 + i)
		// Make one of the high-allocation functions excluded (func with i=1)
		if i == 1 {
			symbolID = types.SymbolID(1001) // This one is excluded
		}

		functions[i] = &FunctionAnalysis{
			Name:      "func" + string(rune('A'+i)),
			FilePath:  "test.go",
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Language:  "go",
			SymbolID:  symbolID,
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: i * 10, EndLine: i*10 + 3, Depth: 1},
			},
			Calls: calls,
		}
	}

	result := ma.AnalyzeFunctions(functions)

	// Check that the excluded function is not in hotspots
	for _, hs := range result.Hotspots {
		if hs.FunctionName == "funcB" { // funcB has SymbolID 1001 (excluded)
			t.Error("Excluded function funcB should not appear in hotspots")
		}
	}

	// Verify the excluded function has IsExcluded=true
	for _, score := range result.Scores {
		if score.FunctionName == "funcB" {
			if !score.IsExcluded {
				t.Error("funcB should be marked as excluded")
			}
			break
		}
	}

	t.Logf("Hotspots count: %d (excluded func should not be included)", len(result.Hotspots))
}

func TestMemoryAnalyzer_NoAnnotator(t *testing.T) {
	// Test that analysis works correctly without a semantic annotator
	ma := NewMemoryAnalyzer()
	// Don't set semantic annotator - should work without it

	functions := []*FunctionAnalysis{
		{
			Name:      "someFunc",
			FilePath:  "service.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			SymbolID:  types.SymbolID(1000),
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result.Scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(result.Scores))
	}

	// Without annotator, nothing should be excluded
	if result.Scores[0].IsExcluded {
		t.Error("Without annotator, function should not be excluded")
	}

	if result.Summary.ExcludedCount != 0 {
		t.Errorf("Without annotator, ExcludedCount should be 0, got %d", result.Summary.ExcludedCount)
	}
}

func TestMemoryAnalyzer_SetExclusionChecker(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Initially no exclusion checker
	if ma.exclusionChecker != nil {
		t.Error("Initially exclusionChecker should be nil")
	}

	// Set checker
	mockChecker := &mockExclusionChecker{}
	ma.SetExclusionChecker(mockChecker)

	if ma.exclusionChecker == nil {
		t.Error("After SetExclusionChecker, exclusionChecker should not be nil")
	}
}

// mockExclusionChecker is a test double that implements ExclusionChecker interface
type mockExclusionChecker struct {
	excludedSymbols map[types.SymbolID]bool
}

func (m *mockExclusionChecker) IsExcludedFromAnalysis(fileID types.FileID, symbolID types.SymbolID, analysisType string) bool {
	if m.excludedSymbols == nil {
		return false
	}
	return m.excludedSymbols[symbolID] && analysisType == "memory"
}

func TestGetCallFrequencyMultiplier(t *testing.T) {
	tests := []struct {
		frequency string
		expected  float64
	}{
		{"hot-path", 1.0},
		{"once-per-file", 0.8},
		{"once-per-request", 0.7},
		{"once-per-session", 0.5},
		{"startup-only", 0.3},
		{"cli-output", 0.2},
		{"test-only", 0.1},
		{"rare", 0.1},
		{"", 1.0},            // Unknown defaults to hot
		{"unknown", 1.0},     // Unknown value defaults to hot
		{"some-random", 1.0}, // Unrecognized value defaults to hot
	}

	for _, tt := range tests {
		got := getCallFrequencyMultiplier(tt.frequency)
		if got != tt.expected {
			t.Errorf("getCallFrequencyMultiplier(%q) = %.2f, want %.2f", tt.frequency, got, tt.expected)
		}
	}
}

func TestMemoryAnalyzer_CallFrequencyScoring(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Create identical functions with same allocations
	// We'll manually check that call frequency info is preserved in the score
	functions := []*FunctionAnalysis{
		{
			Name:      "hotPathFunc",
			FilePath:  "service.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 2, EndLine: 8, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
		{
			Name:      "cliOutputFunc",
			FilePath:  "cli.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 2, EndLine: 8, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "make([]byte, 1024)", Line: 3, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	// Both functions should have scores (since no annotations set CallFrequency)
	// This test verifies the structure is correct
	if len(result.Scores) != 2 {
		t.Fatalf("Expected 2 scores, got %d", len(result.Scores))
	}

	// Verify CallFrequency and HasBoundedLoops fields exist in score
	for _, score := range result.Scores {
		// CallFrequency should be empty (no annotations)
		if score.CallFrequency != "" {
			t.Errorf("%s should have empty CallFrequency without annotations, got %q",
				score.FunctionName, score.CallFrequency)
		}
		// HasBoundedLoops should be false (no annotations)
		if score.HasBoundedLoops {
			t.Errorf("%s should have HasBoundedLoops=false without annotations",
				score.FunctionName)
		}
	}

	t.Logf("Call frequency scoring test passed - structure verified")
}

func TestMemoryAnalyzer_BoundedLoopDetection(t *testing.T) {
	// Test that HasBoundedLoops is properly tracked in scores
	// Note: Without semantic annotator, MemoryHints will be nil,
	// so this tests the default behavior
	ma := NewMemoryAnalyzer()

	functions := []*FunctionAnalysis{
		{
			Name:      "retryFunc",
			FilePath:  "client.go",
			StartLine: 1,
			EndLine:   20,
			Language:  "go",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 2, EndLine: 15, Depth: 1},
			},
			Calls: []CallInfo{
				{Target: "doRequest()", Line: 5, InLoop: true, LoopDepth: 1},
			},
		},
	}

	result := ma.AnalyzeFunctions(functions)

	if len(result.Scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(result.Scores))
	}

	score := result.Scores[0]
	// Without annotations, HasBoundedLoops should be false
	if score.HasBoundedLoops {
		t.Error("Without annotations, HasBoundedLoops should be false")
	}

	t.Logf("Bounded loop detection: HasBoundedLoops=%v", score.HasBoundedLoops)
}

func TestEstimateLoopIterations(t *testing.T) {
	tests := []struct {
		loopType string
		expected float64
	}{
		{"for_statement", 10.0},
		{"for_range_statement", 10.0},
		{"while_statement", 20.0},
		{"do_while_statement", 20.0},
		{"for_in_statement", 10.0},
		{"for_of_statement", 10.0},
		{"for_each_statement", 10.0},
		{"unknown_loop_type", 10.0}, // Default
	}

	for _, tt := range tests {
		got := estimateLoopIterations(tt.loopType)
		if got != tt.expected {
			t.Errorf("estimateLoopIterations(%q) = %.1f, want %.1f", tt.loopType, got, tt.expected)
		}
	}
}

func TestMemoryAnalyzer_LoopWeightCalculation(t *testing.T) {
	ma := NewMemoryAnalyzer()

	// Test getEffectiveLoopWeight with no hints
	weight := ma.getEffectiveLoopWeight(nil, nil)
	if weight != 10.0 {
		t.Errorf("getEffectiveLoopWeight(nil, nil) = %.1f, want 10.0", weight)
	}

	// Test with loops but no hints
	loops := []LoopInfo{
		{NodeType: "while_statement", StartLine: 1, EndLine: 10, Depth: 1},
	}
	weight = ma.getEffectiveLoopWeight(nil, loops)
	if weight != 20.0 { // while_statement has 20.0 multiplier
		t.Errorf("getEffectiveLoopWeight(nil, loops) = %.1f, want 20.0", weight)
	}

	t.Logf("Loop weight calculation verified")
}
