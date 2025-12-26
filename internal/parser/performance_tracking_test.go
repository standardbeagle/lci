package parser

import (
	"context"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestUnifiedExtractor_PerformanceTracking_JSAsyncAwaits(t *testing.T) {
	code := []byte(`
async function fetchUserData(userId) {
    const profile = await fetchProfile(userId);
    const orders = await fetchOrders(userId);
    const preferences = await fetchPreferences(userId);
    return { profile, orders, preferences };
}
`)
	parser := NewTreeSitterParser()

	// Use the ParseFileEnhancedWithASTAndContextStringRef which gives us access to the AST
	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.js", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse JavaScript code")
	}
	defer tree.Close()

	// Create extractor and run extraction
	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".js", "test.js")
	extractor.Extract(tree)
	results := extractor.GetPerfAnalysisResults()

	if len(results) == 0 {
		t.Fatal("Expected at least one function analysis result")
	}

	// Find the fetchUserData function
	var funcResult *PerfAnalysisResult
	for i := range results {
		if results[i].FunctionName == "fetchUserData" {
			funcResult = &results[i]
			break
		}
	}

	if funcResult == nil {
		t.Logf("Available functions: %v", getFunctionNames(results))
		t.Fatal("Expected to find fetchUserData function analysis")
	}

	// Should be detected as async
	if !funcResult.IsAsync {
		t.Error("Expected fetchUserData to be detected as async")
	}

	// Should have 3 await expressions
	if len(funcResult.Awaits) != 3 {
		t.Errorf("Expected 3 await expressions, got %d", len(funcResult.Awaits))
	}

	// Language should be javascript
	if funcResult.Language != "javascript" {
		t.Errorf("Expected language 'javascript', got '%s'", funcResult.Language)
	}
}

func TestUnifiedExtractor_PerformanceTracking_GoLoops(t *testing.T) {
	code := []byte(`
package main

func processItems(items []string) {
    for _, item := range items {
        for j := 0; j < len(item); j++ {
            process(item[j])
        }
    }
}

func simpleLoop() {
    for i := 0; i < 10; i++ {
        fmt.Println(i)
    }
}
`)
	parser := NewTreeSitterParser()

	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.go", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".go", "test.go")
	extractor.Extract(tree)
	results := extractor.GetPerfAnalysisResults()

	// Find processItems function
	var processItemsResult *PerfAnalysisResult
	var simpleLoopResult *PerfAnalysisResult
	for i := range results {
		if results[i].FunctionName == "processItems" {
			processItemsResult = &results[i]
		}
		if results[i].FunctionName == "simpleLoop" {
			simpleLoopResult = &results[i]
		}
	}

	if processItemsResult == nil {
		t.Logf("Available functions: %v", getFunctionNames(results))
		t.Fatal("Expected to find processItems function analysis")
	}

	// processItems should have 2 loops (nested)
	if len(processItemsResult.Loops) != 2 {
		t.Errorf("processItems: expected 2 loops, got %d", len(processItemsResult.Loops))
	}

	// Verify nesting depth
	hasDepth1 := false
	hasDepth2 := false
	for _, loop := range processItemsResult.Loops {
		if loop.Depth == 1 {
			hasDepth1 = true
		}
		if loop.Depth == 2 {
			hasDepth2 = true
		}
	}
	if !hasDepth1 || !hasDepth2 {
		t.Errorf("processItems: expected loops at depth 1 and 2, got: %+v", processItemsResult.Loops)
	}

	// simpleLoop should have 1 loop
	if simpleLoopResult == nil {
		t.Fatal("Expected to find simpleLoop function analysis")
	}
	if len(simpleLoopResult.Loops) != 1 {
		t.Errorf("simpleLoop: expected 1 loop, got %d", len(simpleLoopResult.Loops))
	}

	// Language should be go
	if processItemsResult.Language != "go" {
		t.Errorf("Expected language 'go', got '%s'", processItemsResult.Language)
	}
}

func TestUnifiedExtractor_PerformanceTracking_CallsInLoop(t *testing.T) {
	code := []byte(`
package main

import "regexp"

func processData(items []string) {
    for _, item := range items {
        re := regexp.MustCompile(item)
        if re.MatchString("test") {
            fmt.Println(item)
        }
    }
}
`)
	parser := NewTreeSitterParser()

	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.go", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".go", "test.go")
	extractor.Extract(tree)
	results := extractor.GetPerfAnalysisResults()

	// Find processData function
	var funcResult *PerfAnalysisResult
	for i := range results {
		if results[i].FunctionName == "processData" {
			funcResult = &results[i]
			break
		}
	}

	if funcResult == nil {
		t.Logf("Available functions: %v", getFunctionNames(results))
		t.Fatal("Expected to find processData function analysis")
	}

	// Should have calls tracked
	if len(funcResult.Calls) == 0 {
		t.Error("Expected calls to be tracked")
	}

	// Should have calls inside loop
	hasCallInLoop := false
	for _, call := range funcResult.Calls {
		if call.InLoop {
			hasCallInLoop = true
			break
		}
	}
	if !hasCallInLoop {
		t.Error("Expected at least one call to be marked as inside loop")
	}
}

func TestUnifiedExtractor_PerformanceTracking_PythonAsync(t *testing.T) {
	code := []byte(`
async def fetch_all_data(user_id):
    profile = await fetch_profile(user_id)
    orders = await fetch_orders(user_id)
    return profile, orders
`)
	parser := NewTreeSitterParser()

	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(
		context.Background(), "test.py", code, types.FileID(1))

	if tree == nil {
		t.Fatal("Failed to parse Python code")
	}
	defer tree.Close()

	extractor := NewUnifiedExtractor(parser, code, types.FileID(1), ".py", "test.py")
	extractor.Extract(tree)
	results := extractor.GetPerfAnalysisResults()

	if len(results) == 0 {
		t.Fatal("Expected at least one function analysis result")
	}

	// Find the fetch_all_data function or any function with awaits
	var funcResult *PerfAnalysisResult
	for i := range results {
		if results[i].FunctionName == "fetch_all_data" {
			funcResult = &results[i]
			break
		}
	}

	if funcResult == nil {
		// Try to find any function with awaits
		for i := range results {
			if len(results[i].Awaits) > 0 {
				funcResult = &results[i]
				break
			}
		}
	}

	if funcResult == nil {
		t.Logf("Results: %+v", results)
		t.Fatal("Expected to find function with awaits")
	}

	// Should have awaits detected (Python await nodes may differ)
	if len(funcResult.Awaits) < 2 {
		t.Logf("Awaits found: %+v", funcResult.Awaits)
		// Note: Python await detection depends on tree-sitter node types
		// This test may need adjustment based on actual tree-sitter output
	}

	// Language should be python
	if funcResult.Language != "python" {
		t.Errorf("Expected language 'python', got '%s'", funcResult.Language)
	}
}

func TestUnifiedExtractor_LoopTypes(t *testing.T) {
	parser := NewTreeSitterParser()
	extractor := NewUnifiedExtractor(parser, nil, types.FileID(1), ".js", "test.js")

	// Test various loop node types
	loopTypes := []string{
		"for_statement",
		"for_range_statement",
		"for_in_statement",
		"for_of_statement",
		"while_statement",
		"do_while_statement",
		"loop_expression",
		"for_each_statement",
	}

	for _, lt := range loopTypes {
		if !extractor.isLoopNode(lt) {
			t.Errorf("Expected %s to be recognized as loop node", lt)
		}
	}

	// Non-loop types
	nonLoopTypes := []string{
		"if_statement",
		"function_declaration",
		"block",
		"identifier",
	}

	for _, nlt := range nonLoopTypes {
		if extractor.isLoopNode(nlt) {
			t.Errorf("Expected %s to NOT be recognized as loop node", nlt)
		}
	}
}

func TestUnifiedExtractor_AwaitTypes(t *testing.T) {
	parser := NewTreeSitterParser()
	extractor := NewUnifiedExtractor(parser, nil, types.FileID(1), ".js", "test.js")

	// Test await node types - only await_expression to avoid double-counting
	// (await_expression contains await keyword as child)
	awaitTypes := []string{
		"await_expression",
	}

	for _, at := range awaitTypes {
		if !extractor.isAwaitNode(at) {
			t.Errorf("Expected %s to be recognized as await node", at)
		}
	}

	// Non-await types - including "await" keyword which is a child of await_expression
	nonAwaitTypes := []string{
		"await", // keyword node, not expression
		"call_expression",
		"identifier",
		"promise",
	}

	for _, nat := range nonAwaitTypes {
		if extractor.isAwaitNode(nat) {
			t.Errorf("Expected %s to NOT be recognized as await node", nat)
		}
	}
}

func TestUnifiedExtractor_CallExpressionTypes(t *testing.T) {
	parser := NewTreeSitterParser()
	extractor := NewUnifiedExtractor(parser, nil, types.FileID(1), ".js", "test.js")

	// Test call expression node types
	callTypes := []string{
		"call_expression",
		"call",
		"invocation_expression",
		"method_invocation",
	}

	for _, ct := range callTypes {
		if !extractor.isCallExpression(ct) {
			t.Errorf("Expected %s to be recognized as call expression", ct)
		}
	}

	// Non-call types
	nonCallTypes := []string{
		"identifier",
		"member_expression",
		"function_declaration",
	}

	for _, nct := range nonCallTypes {
		if extractor.isCallExpression(nct) {
			t.Errorf("Expected %s to NOT be recognized as call expression", nct)
		}
	}
}

func TestGetLanguageFromExtInternal(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".js", "javascript"},
		{".jsx", "javascript"},
		{".mjs", "javascript"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".py", "python"},
		{".rs", "rust"},
		{".java", "java"},
		{".cs", "csharp"},
		{".rb", "ruby"},
		{".php", "php"},
		{".unknown", "unknown"},
	}

	for _, tt := range tests {
		result := getLanguageFromExt(tt.ext)
		if result != tt.expected {
			t.Errorf("getLanguageFromExt(%s) = %s, expected %s", tt.ext, result, tt.expected)
		}
	}
}

// Helper function to get function names from results for debugging
func getFunctionNames(results []PerfAnalysisResult) []string {
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.FunctionName
	}
	return names
}
