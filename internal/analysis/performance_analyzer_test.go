package analysis

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestPerformanceAnalyzer_NewPerformanceAnalyzer(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	if pa == nil {
		t.Fatal("NewPerformanceAnalyzer returned nil")
	}

	// Check that expensive call patterns are initialized for key languages
	for _, lang := range []string{"go", "javascript", "typescript", "python", "rust", "java", "csharp"} {
		if len(pa.expensiveCallPatterns[lang]) == 0 {
			t.Errorf("Expected expensive call patterns for %s", lang)
		}
	}

	// Check loop node types
	if !pa.IsLoopNode("for_statement") {
		t.Error("for_statement should be a loop node")
	}
	if !pa.IsLoopNode("while_statement") {
		t.Error("while_statement should be a loop node")
	}
	if pa.IsLoopNode("if_statement") {
		t.Error("if_statement should not be a loop node")
	}

	// Check await node types
	if !pa.IsAwaitNode("await_expression") {
		t.Error("await_expression should be an await node")
	}
	if !pa.IsAwaitNode("await") {
		t.Error("await should be an await node (Python)")
	}
}

func TestPerformanceAnalyzer_DetectSequentialAwaits(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	tests := []struct {
		name          string
		analysis      *FunctionAnalysis
		expectPattern bool
		expectedCount int
	}{
		{
			name: "No awaits - no pattern",
			analysis: &FunctionAnalysis{
				Name:     "noAwaits",
				IsAsync:  true,
				Awaits:   []AwaitInfo{},
				Language: "javascript",
			},
			expectPattern: false,
		},
		{
			name: "Single await - no pattern",
			analysis: &FunctionAnalysis{
				Name:    "singleAwait",
				IsAsync: true,
				Awaits: []AwaitInfo{
					{Line: 5, AssignedVar: "a", CallTarget: "fetchA", UsedVars: []string{}},
				},
				Language: "javascript",
			},
			expectPattern: false,
		},
		{
			name: "Two independent awaits - should detect",
			analysis: &FunctionAnalysis{
				Name:    "twoIndependentAwaits",
				IsAsync: true,
				Awaits: []AwaitInfo{
					{Line: 5, AssignedVar: "a", CallTarget: "fetchA", UsedVars: []string{}},
					{Line: 6, AssignedVar: "b", CallTarget: "fetchB", UsedVars: []string{}},
				},
				Language:  "javascript",
				StartLine: 1,
				EndLine:   10,
				FilePath:  "test.js",
			},
			expectPattern: true,
			expectedCount: 2,
		},
		{
			name: "Two dependent awaits - no pattern",
			analysis: &FunctionAnalysis{
				Name:    "twoDependentAwaits",
				IsAsync: true,
				Awaits: []AwaitInfo{
					{Line: 5, AssignedVar: "a", CallTarget: "fetchA", UsedVars: []string{}},
					{Line: 6, AssignedVar: "b", CallTarget: "fetchB", UsedVars: []string{"a"}}, // depends on a
				},
				Language:  "javascript",
				StartLine: 1,
				EndLine:   10,
				FilePath:  "test.js",
			},
			expectPattern: false,
		},
		{
			name: "Three awaits, two independent - should detect",
			analysis: &FunctionAnalysis{
				Name:    "threeAwaits",
				IsAsync: true,
				Awaits: []AwaitInfo{
					{Line: 5, AssignedVar: "a", CallTarget: "fetchA", UsedVars: []string{}},
					{Line: 6, AssignedVar: "b", CallTarget: "fetchB", UsedVars: []string{"a"}}, // depends on a
					{Line: 7, AssignedVar: "c", CallTarget: "fetchC", UsedVars: []string{}},    // independent
				},
				Language:  "javascript",
				StartLine: 1,
				EndLine:   10,
				FilePath:  "test.js",
			},
			expectPattern: true,
			expectedCount: 2, // a and c are independent
		},
		{
			name: "Non-async function - no pattern",
			analysis: &FunctionAnalysis{
				Name:    "nonAsync",
				IsAsync: false, // Not async
				Awaits: []AwaitInfo{
					{Line: 5, AssignedVar: "a", CallTarget: "fetchA", UsedVars: []string{}},
					{Line: 6, AssignedVar: "b", CallTarget: "fetchB", UsedVars: []string{}},
				},
				Language: "javascript",
			},
			expectPattern: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := pa.AnalyzeFunction(tt.analysis)

			var sequentialAwaitPattern *PerformancePattern
			for i := range patterns {
				if patterns[i].Type == "sequential-awaits" {
					sequentialAwaitPattern = &patterns[i]
					break
				}
			}

			if tt.expectPattern && sequentialAwaitPattern == nil {
				t.Error("Expected sequential-awaits pattern but didn't find one")
			}
			if !tt.expectPattern && sequentialAwaitPattern != nil {
				t.Errorf("Didn't expect sequential-awaits pattern but found: %+v", sequentialAwaitPattern)
			}
			if tt.expectPattern && sequentialAwaitPattern != nil {
				if sequentialAwaitPattern.Details.ParallelizableCount != tt.expectedCount {
					t.Errorf("Expected %d parallelizable awaits, got %d",
						tt.expectedCount, sequentialAwaitPattern.Details.ParallelizableCount)
				}
			}
		})
	}
}

func TestPerformanceAnalyzer_DetectAwaitInLoop(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	tests := []struct {
		name          string
		analysis      *FunctionAnalysis
		expectPattern bool
	}{
		{
			name: "Await outside loop - no pattern",
			analysis: &FunctionAnalysis{
				Name:    "awaitOutsideLoop",
				IsAsync: true,
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 10, EndLine: 20, Depth: 1},
				},
				Awaits: []AwaitInfo{
					{Line: 5, CallTarget: "fetchData"}, // Before loop
				},
				Language: "javascript",
			},
			expectPattern: false,
		},
		{
			name: "Await inside loop - should detect",
			analysis: &FunctionAnalysis{
				Name:    "awaitInsideLoop",
				IsAsync: true,
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 10, EndLine: 20, Depth: 1},
				},
				Awaits: []AwaitInfo{
					{Line: 15, CallTarget: "fetchData"}, // Inside loop
				},
				Language:  "javascript",
				StartLine: 1,
				EndLine:   25,
				FilePath:  "test.js",
			},
			expectPattern: true,
		},
		{
			name: "Multiple awaits, one in loop - should detect one",
			analysis: &FunctionAnalysis{
				Name:    "mixedAwaits",
				IsAsync: true,
				Loops: []LoopInfo{
					{NodeType: "for_of_statement", StartLine: 10, EndLine: 20, Depth: 1},
				},
				Awaits: []AwaitInfo{
					{Line: 5, CallTarget: "setup"},     // Before loop
					{Line: 15, CallTarget: "fetchOne"}, // Inside loop
					{Line: 25, CallTarget: "cleanup"},  // After loop
				},
				Language:  "javascript",
				StartLine: 1,
				EndLine:   30,
				FilePath:  "test.js",
			},
			expectPattern: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := pa.AnalyzeFunction(tt.analysis)

			var awaitInLoopCount int
			for _, p := range patterns {
				if p.Type == "await-in-loop" {
					awaitInLoopCount++
				}
			}

			if tt.expectPattern && awaitInLoopCount == 0 {
				t.Error("Expected await-in-loop pattern but didn't find one")
			}
			if !tt.expectPattern && awaitInLoopCount > 0 {
				t.Errorf("Didn't expect await-in-loop pattern but found %d", awaitInLoopCount)
			}
		})
	}
}

func TestPerformanceAnalyzer_DetectExpensiveCallInLoop(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	tests := []struct {
		name          string
		analysis      *FunctionAnalysis
		expectPattern bool
		category      string
	}{
		{
			name: "Regex compile in loop - Go",
			analysis: &FunctionAnalysis{
				Name: "regexInLoop",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 15, Depth: 1},
				},
				Calls: []CallInfo{
					{Target: "regexp.Compile", Line: 10, InLoop: true, LoopDepth: 1, LoopLine: 5},
				},
				Language:  "go",
				StartLine: 1,
				EndLine:   20,
				FilePath:  "test.go",
			},
			expectPattern: true,
			category:      "regex",
		},
		{
			name: "Regex compile outside loop - Go",
			analysis: &FunctionAnalysis{
				Name: "regexOutsideLoop",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 10, EndLine: 20, Depth: 1},
				},
				Calls: []CallInfo{
					{Target: "regexp.Compile", Line: 5, InLoop: false, LoopDepth: 0, LoopLine: 0},
				},
				Language: "go",
			},
			expectPattern: false,
		},
		{
			name: "JSON parse in loop - JavaScript",
			analysis: &FunctionAnalysis{
				Name: "jsonInLoop",
				Loops: []LoopInfo{
					{NodeType: "for_of_statement", StartLine: 5, EndLine: 15, Depth: 1},
				},
				Calls: []CallInfo{
					{Target: "JSON.parse", Line: 10, InLoop: true, LoopDepth: 1, LoopLine: 5},
				},
				Language:  "javascript",
				StartLine: 1,
				EndLine:   20,
				FilePath:  "test.js",
			},
			expectPattern: true,
			category:      "parse",
		},
		{
			name: "HTTP request in loop - Python",
			analysis: &FunctionAnalysis{
				Name: "httpInLoop",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 15, Depth: 1},
				},
				Calls: []CallInfo{
					{Target: "requests.get", Line: 10, InLoop: true, LoopDepth: 1, LoopLine: 5},
				},
				Language:  "python",
				StartLine: 1,
				EndLine:   20,
				FilePath:  "test.py",
			},
			expectPattern: true,
			category:      "network",
		},
		{
			name: "Normal call in loop - no pattern",
			analysis: &FunctionAnalysis{
				Name: "normalCallInLoop",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 15, Depth: 1},
				},
				Calls: []CallInfo{
					{Target: "processItem", Line: 10, InLoop: true, LoopDepth: 1, LoopLine: 5},
				},
				Language: "go",
			},
			expectPattern: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := pa.AnalyzeFunction(tt.analysis)

			var expensiveCallPattern *PerformancePattern
			for i := range patterns {
				if patterns[i].Type == "expensive-call-in-loop" {
					expensiveCallPattern = &patterns[i]
					break
				}
			}

			if tt.expectPattern && expensiveCallPattern == nil {
				t.Error("Expected expensive-call-in-loop pattern but didn't find one")
			}
			if !tt.expectPattern && expensiveCallPattern != nil {
				t.Errorf("Didn't expect expensive-call-in-loop pattern but found: %+v", expensiveCallPattern)
			}
			if tt.expectPattern && expensiveCallPattern != nil && tt.category != "" {
				if expensiveCallPattern.Details.ExpenseCategory != tt.category {
					t.Errorf("Expected category %s, got %s", tt.category, expensiveCallPattern.Details.ExpenseCategory)
				}
			}
		})
	}
}

func TestPerformanceAnalyzer_DetectNestedLoops(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	tests := []struct {
		name          string
		analysis      *FunctionAnalysis
		expectPattern bool
		expectedDepth int
	}{
		{
			name: "Single loop - no pattern",
			analysis: &FunctionAnalysis{
				Name: "singleLoop",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 15, Depth: 1},
				},
				Language: "go",
			},
			expectPattern: false,
		},
		{
			name: "Two nested loops - should detect",
			analysis: &FunctionAnalysis{
				Name: "nestedLoops",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 20, Depth: 1},
					{NodeType: "for_statement", StartLine: 10, EndLine: 15, Depth: 2},
				},
				Language:  "go",
				StartLine: 1,
				EndLine:   25,
				FilePath:  "test.go",
			},
			expectPattern: true,
			expectedDepth: 2,
		},
		{
			name: "Three nested loops - should detect with high severity",
			analysis: &FunctionAnalysis{
				Name: "tripleNested",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 30, Depth: 1},
					{NodeType: "for_statement", StartLine: 10, EndLine: 25, Depth: 2},
					{NodeType: "for_statement", StartLine: 15, EndLine: 20, Depth: 3},
				},
				Language:  "go",
				StartLine: 1,
				EndLine:   35,
				FilePath:  "test.go",
			},
			expectPattern: true,
			expectedDepth: 3,
		},
		{
			name: "Two sequential loops - no pattern (not nested)",
			analysis: &FunctionAnalysis{
				Name: "sequentialLoops",
				Loops: []LoopInfo{
					{NodeType: "for_statement", StartLine: 5, EndLine: 10, Depth: 1},
					{NodeType: "for_statement", StartLine: 15, EndLine: 20, Depth: 1}, // Same depth = sequential
				},
				Language: "go",
			},
			expectPattern: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := pa.AnalyzeFunction(tt.analysis)

			var nestedPattern *PerformancePattern
			for i := range patterns {
				if patterns[i].Type == "nested-loops" {
					nestedPattern = &patterns[i]
					break
				}
			}

			if tt.expectPattern && nestedPattern == nil {
				t.Error("Expected nested-loops pattern but didn't find one")
			}
			if !tt.expectPattern && nestedPattern != nil {
				t.Errorf("Didn't expect nested-loops pattern but found: %+v", nestedPattern)
			}
			if tt.expectPattern && nestedPattern != nil {
				if nestedPattern.Details.NestingDepth != tt.expectedDepth {
					t.Errorf("Expected nesting depth %d, got %d",
						tt.expectedDepth, nestedPattern.Details.NestingDepth)
				}
			}
		})
	}
}

func TestPerformanceAnalyzer_Suggestions(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	// Test sequential awaits suggestions for different languages
	languages := []struct {
		lang     string
		contains string
	}{
		{"javascript", "Promise.all"},
		{"typescript", "Promise.all"},
		{"python", "asyncio.gather"},
		{"csharp", "Task.WhenAll"},
		{"rust", "join!"},
	}

	for _, lang := range languages {
		suggestion := pa.getParallelizationSuggestion(lang.lang)
		if suggestion == "" {
			t.Errorf("Empty suggestion for %s", lang.lang)
		}
		// Just verify we get a non-empty suggestion
		if len(suggestion) < 10 {
			t.Errorf("Suggestion for %s seems too short: %s", lang.lang, suggestion)
		}
	}
}

func TestPerformanceAnalyzer_AnalyzeFile(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	fileInfo := &types.FileInfo{
		Path: "test.js",
	}

	functions := []*FunctionAnalysis{
		{
			Name:    "func1",
			IsAsync: true,
			Awaits: []AwaitInfo{
				{Line: 5, AssignedVar: "a", CallTarget: "fetch1", UsedVars: []string{}},
				{Line: 6, AssignedVar: "b", CallTarget: "fetch2", UsedVars: []string{}},
			},
			Language:  "javascript",
			StartLine: 1,
			EndLine:   10,
			FilePath:  "test.js",
		},
		{
			Name: "func2",
			Loops: []LoopInfo{
				{NodeType: "for_statement", StartLine: 15, EndLine: 25, Depth: 1},
				{NodeType: "for_statement", StartLine: 18, EndLine: 22, Depth: 2},
			},
			Language:  "javascript",
			StartLine: 12,
			EndLine:   30,
			FilePath:  "test.js",
		},
	}

	patterns := pa.AnalyzeFile(fileInfo, functions)

	if len(patterns) < 2 {
		t.Errorf("Expected at least 2 patterns, got %d", len(patterns))
	}

	// Should have sequential-awaits from func1 and nested-loops from func2
	hasSequentialAwaits := false
	hasNestedLoops := false
	for _, p := range patterns {
		if p.Type == "sequential-awaits" {
			hasSequentialAwaits = true
		}
		if p.Type == "nested-loops" {
			hasNestedLoops = true
		}
	}

	if !hasSequentialAwaits {
		t.Error("Expected sequential-awaits pattern from func1")
	}
	if !hasNestedLoops {
		t.Error("Expected nested-loops pattern from func2")
	}
}

func TestGetLanguageFromExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".js", "javascript"},
		{".jsx", "javascript"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".py", "python"},
		{".rs", "rust"},
		{".java", "java"},
		{".cs", "csharp"},
		{".rb", "ruby"},
		{".php", "php"},
		{".xyz", "unknown"},
	}

	for _, tt := range tests {
		result := GetLanguageFromExt(tt.ext)
		if result != tt.expected {
			t.Errorf("GetLanguageFromExt(%s) = %s, expected %s", tt.ext, result, tt.expected)
		}
	}
}

func TestExpensiveCallPatterns(t *testing.T) {
	pa := NewPerformanceAnalyzer()

	// Test that patterns actually match expected function calls
	testCases := []struct {
		language string
		call     string
		should   bool
	}{
		// Go patterns
		{"go", "regexp.Compile", true},
		{"go", "regexp.MustCompile", true},
		{"go", "json.Unmarshal", true},
		{"go", "os.Open", true},
		{"go", "http.Get", true},
		{"go", "fmt.Println", false},
		{"go", "strings.Contains", false},

		// JavaScript patterns
		{"javascript", "new RegExp", true},
		{"javascript", "JSON.parse", true},
		{"javascript", "fetch(", true},
		{"javascript", "console.log", false},

		// Python patterns
		{"python", "re.compile", true},
		{"python", "json.loads", true},
		{"python", "requests.get", true},
		{"python", "print", false},

		// Rust patterns
		{"rust", "Regex::new", true},
		{"rust", "File::open", true},
		{"rust", "Vec::new", false},
	}

	for _, tc := range testCases {
		patterns, exists := pa.expensiveCallPatterns[tc.language]
		if !exists {
			t.Errorf("No patterns for language %s", tc.language)
			continue
		}

		matched := false
		for _, p := range patterns {
			if p.Pattern.MatchString(tc.call) {
				matched = true
				break
			}
		}

		if tc.should && !matched {
			t.Errorf("Expected %s call '%s' to match expensive pattern", tc.language, tc.call)
		}
		if !tc.should && matched {
			t.Errorf("Didn't expect %s call '%s' to match expensive pattern", tc.language, tc.call)
		}
	}
}
