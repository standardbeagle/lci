package parser

import (
	"context"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestSideEffectTracking_EnableAndExtract(t *testing.T) {
	// Test Go code with side effects
	code := []byte(`package main

func pureAdd(a, b int) int {
	return a + b
}

func modifySlice(data []int) {
	data[0] = 100
}
`)

	parser := NewTreeSitterParser()

	// Use the ParseFileEnhancedWithASTAndContextStringRef which gives us access to the AST
	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.go", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".go", "test.go")
	extractor.EnableSideEffectTracking()
	extractor.Extract(tree)

	results := extractor.GetSideEffectResults()
	if results == nil {
		t.Fatal("Side effect results should not be nil when tracking is enabled")
	}

	// Check that we got results for the functions
	if len(results) < 2 {
		t.Errorf("Expected at least 2 function results, got %d", len(results))
	}

	// Verify results exist (exact keys depend on line numbers)
	foundPure := false
	foundParamWrite := false

	for key, info := range results {
		t.Logf("Function %s at %s: IsPure=%v, Categories=%d", info.FunctionName, key, info.IsPure, info.Categories)

		if info.FunctionName == "pureAdd" {
			foundPure = true
			if !info.IsPure {
				t.Errorf("pureAdd should be detected as pure")
			}
		}

		if info.FunctionName == "modifySlice" {
			foundParamWrite = true
			if info.IsPure {
				t.Errorf("modifySlice should be detected as impure (param write)")
			}
			if info.Categories&types.SideEffectParamWrite == 0 {
				t.Errorf("modifySlice should have SideEffectParamWrite flag")
			}
		}
	}

	if !foundPure {
		t.Error("Did not find pureAdd function in results")
	}
	if !foundParamWrite {
		t.Error("Did not find modifySlice function in results")
	}
}

func TestSideEffectTracking_Disabled(t *testing.T) {
	code := []byte(`package main

func add(a, b int) int {
	return a + b
}
`)

	parser := NewTreeSitterParser()

	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.go", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".go", "test.go")
	// Do NOT enable side effect tracking

	extractor.Extract(tree)

	results := extractor.GetSideEffectResults()
	if results != nil && len(results) > 0 {
		t.Error("Side effect results should be empty when tracking is disabled")
	}
}

func TestSideEffectTracking_JavaScript(t *testing.T) {
	code := []byte(`
function pureFunction(x, y) {
	return x + y;
}

let globalState = 0;

function impureFunction() {
	globalState++;
	return globalState;
}

function modifyParam(obj) {
	obj.value = 10;
}
`)

	parser := NewTreeSitterParser()

	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.js", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".js", "test.js")
	extractor.EnableSideEffectTracking()
	extractor.Extract(tree)

	results := extractor.GetSideEffectResults()
	if results == nil {
		t.Fatal("Side effect results should not be nil when tracking is enabled")
	}

	t.Logf("Found %d function results", len(results))

	for key, info := range results {
		t.Logf("Function %s at %s: IsPure=%v, Categories=%d, Confidence=%v",
			info.FunctionName, key, info.IsPure, info.Categories, info.Confidence)
	}
}

func TestSideEffectTracking_Python(t *testing.T) {
	code := []byte(`
def pure_function(x, y):
    return x + y

global_counter = 0

def impure_function():
    global global_counter
    global_counter += 1
    return global_counter

def modify_list(data):
    data[0] = 100
`)

	parser := NewTreeSitterParser()

	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.py", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".py", "test.py")
	extractor.EnableSideEffectTracking()
	extractor.Extract(tree)

	results := extractor.GetSideEffectResults()
	if results == nil {
		t.Fatal("Side effect results should not be nil when tracking is enabled")
	}

	t.Logf("Found %d function results", len(results))

	for key, info := range results {
		t.Logf("Function %s at %s: IsPure=%v, Categories=%d",
			info.FunctionName, key, info.IsPure, info.Categories)
	}
}

func TestLanguageFromExtension(t *testing.T) {
	testCases := []struct {
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
		{".cpp", "cpp"},
		{".c", "c"},
		{".rb", "ruby"},
		{".php", "php"},
		{".kt", "kotlin"},
		{".swift", "swift"},
		{".zig", "zig"},
		{".unknown", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			result := LanguageFromExtension(tc.ext)
			if result != tc.expected {
				t.Errorf("LanguageFromExtension(%q) = %q, want %q", tc.ext, result, tc.expected)
			}
		})
	}
}

func TestSideEffectTracker_BasicUsage(t *testing.T) {
	tracker := NewSideEffectTracker("go")

	// Test function lifecycle
	tracker.BeginFunction("testFunc", "test.go", 1, 10)

	if !tracker.IsInFunction() {
		t.Error("Should be in function after BeginFunction")
	}

	// Add parameters
	tracker.AddParameter("data", 0)
	tracker.AddParameter("config", 1)

	// End function
	info := tracker.EndFunction()

	if info == nil {
		t.Fatal("EndFunction should return SideEffectInfo")
	}

	if tracker.IsInFunction() {
		t.Error("Should not be in function after EndFunction")
	}

	if info.FunctionName != "testFunc" {
		t.Errorf("Expected function name 'testFunc', got '%s'", info.FunctionName)
	}
}
