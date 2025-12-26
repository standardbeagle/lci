package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestIndex creates a test index with the given files
func createTestIndex(t *testing.T, files map[string]string) (*indexing.MasterIndex, string) {
	t.Helper()
	projectRoot := t.TempDir()

	for filename, content := range files {
		filePath := filepath.Join(projectRoot, filename)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := writeFile(filePath, content); err != nil {
			t.Fatalf("Failed to write %s: %v", filename, err)
		}
	}

	cfg := &config.Config{
		Project: config.Project{
			Root: projectRoot,
		},
		Include: []string{"*.go"},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   500,
			MaxFileCount:     10000,
			RespectGitignore: false,
		},
	}

	masterIndex := indexing.NewMasterIndex(cfg)

	ctx := context.Background()
	if err := masterIndex.IndexDirectory(ctx, projectRoot); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	return masterIndex, projectRoot
}

// createExpansionEngine creates an expansion engine from a master index
func createExpansionEngine(masterIndex *indexing.MasterIndex) *ExpansionEngine {
	return NewExpansionEngine(
		masterIndex.GetRefTracker(),
		masterIndex,
	)
}

// Helper function to write test files
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// =============================================================================
// ParseExpansionDirective Tests
// =============================================================================

func TestParseExpansionDirective(t *testing.T) {
	tests := []struct {
		name          string
		directive     string
		expectedType  string
		expectedDepth int
	}{
		{
			name:          "Simple directive without depth",
			directive:     "callers",
			expectedType:  "callers",
			expectedDepth: 1,
		},
		{
			name:          "Directive with explicit depth 1",
			directive:     "callers:1",
			expectedType:  "callers",
			expectedDepth: 1,
		},
		{
			name:          "Directive with depth 2",
			directive:     "callees:2",
			expectedType:  "callees",
			expectedDepth: 2,
		},
		{
			name:          "Directive with depth 5",
			directive:     "callers:5",
			expectedType:  "callers",
			expectedDepth: 5,
		},
		{
			name:          "Directive with invalid depth (defaults to 1)",
			directive:     "callers:abc",
			expectedType:  "callers",
			expectedDepth: 1,
		},
		{
			name:          "Directive with zero depth (defaults to 1)",
			directive:     "callers:0",
			expectedType:  "callers",
			expectedDepth: 1,
		},
		{
			name:          "Directive with negative depth (defaults to 1)",
			directive:     "callers:-1",
			expectedType:  "callers",
			expectedDepth: 1,
		},
		{
			name:          "Implementations directive",
			directive:     "implementations",
			expectedType:  "implementations",
			expectedDepth: 1,
		},
		{
			name:          "Siblings directive",
			directive:     "siblings",
			expectedType:  "siblings",
			expectedDepth: 1,
		},
		{
			name:          "Tests directive",
			directive:     "tests",
			expectedType:  "tests",
			expectedDepth: 1,
		},
		{
			name:          "Empty directive",
			directive:     "",
			expectedType:  "",
			expectedDepth: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directiveType, depth := ParseExpansionDirective(tt.directive)
			if directiveType != tt.expectedType {
				t.Errorf("Type: got %q, want %q", directiveType, tt.expectedType)
			}
			if depth != tt.expectedDepth {
				t.Errorf("Depth: got %d, want %d", depth, tt.expectedDepth)
			}
		})
	}
}

// =============================================================================
// Callers Expansion Tests - Happy Path
// =============================================================================

func TestExpansionEngineCallers_Basic(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	helper()
}

func helper() {
	compute()
}

func compute() {
	// computation
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	// Find the "compute" function
	symbols := refTracker.FindSymbolsByName("compute")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'compute' function")
	}
	computeSymbol := symbols[0]

	fileInfo := masterIndex.GetFileInfo(computeSymbol.FileID)
	if fileInfo == nil {
		t.Fatal("Could not get file info")
	}

	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "compute",
		L: &types.LineRange{Start: computeSymbol.Line, End: computeSymbol.EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	// Should find "helper" as direct caller
	if len(callers) != 1 {
		t.Errorf("Expected 1 caller, got %d", len(callers))
	}

	if len(callers) > 0 && callers[0].Symbol != "helper" {
		t.Errorf("Expected caller 'helper', got %q", callers[0].Symbol)
	}
}

func TestExpansionEngineCallers_MultipleDepth(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	helper()
}

func helper() {
	compute()
}

func compute() {
	// computation
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("compute")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'compute' function")
	}
	computeSymbol := symbols[0]

	fileInfo := masterIndex.GetFileInfo(computeSymbol.FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "compute",
		L: &types.LineRange{Start: computeSymbol.Line, End: computeSymbol.EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	// With depth 2, should find both helper and main
	callers, err := engine.expandCallers(ctx, ref, 2, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	// Should find at least "helper" as direct caller
	// main should be in helper's expanded callers
	if len(callers) == 0 {
		t.Error("Expected at least 1 caller")
	}

	foundHelper := false
	for _, caller := range callers {
		if caller.Symbol == "helper" {
			foundHelper = true
			// Check nested callers
			if nested, ok := caller.Expanded["callers"]; ok {
				foundMain := false
				for _, nestedCaller := range nested {
					if nestedCaller.Symbol == "main" {
						foundMain = true
					}
				}
				if !foundMain {
					t.Error("Expected 'main' in nested callers of 'helper'")
				}
			}
		}
	}

	if !foundHelper {
		t.Error("Expected 'helper' as direct caller")
	}
}

func TestExpansionEngineCallers_CrossFile(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	helper()
}
`,
		"helper.go": `package main

func helper() {
	compute()
}
`,
		"compute.go": `package main

func compute() {
	// computation
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("compute")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'compute' function")
	}
	computeSymbol := symbols[0]

	fileInfo := masterIndex.GetFileInfo(computeSymbol.FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "compute",
		L: &types.LineRange{Start: computeSymbol.Line, End: computeSymbol.EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	// Should find "helper" from different file
	foundHelper := false
	for _, caller := range callers {
		if caller.Symbol == "helper" {
			foundHelper = true
			// Verify it's from helper.go
			if !filepath.IsAbs(caller.File) {
				t.Logf("Caller file: %s", caller.File)
			}
		}
	}

	if !foundHelper {
		t.Error("Expected 'helper' as caller from different file")
	}
}

// =============================================================================
// Callers Expansion Tests - Sad Path
// =============================================================================

func TestExpansionEngineCallers_SymbolNotFound(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	// nothing
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "nonexistent",
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return empty result, not error
	if len(callers) != 0 {
		t.Errorf("Expected 0 callers for nonexistent symbol, got %d", len(callers))
	}
}

func TestExpansionEngineCallers_NoCallers(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	// nothing calls this from within the indexed code
}

func isolated() {
	// never called
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("isolated")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'isolated' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "isolated",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(callers) != 0 {
		t.Errorf("Expected 0 callers for isolated function, got %d", len(callers))
	}
}

func TestExpansionEngineCallers_EmptySymbolName(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "", // Empty symbol name
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(callers) != 0 {
		t.Errorf("Expected 0 callers for empty symbol name, got %d", len(callers))
	}
}

func TestExpansionEngineCallers_NilRefTracker(t *testing.T) {
	engine := NewExpansionEngine(nil, nil)

	ref := types.ContextRef{
		F: "main.go",
		S: "main",
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 1, visited, 10000, ".", types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(callers) != 0 {
		t.Errorf("Expected 0 callers with nil refTracker, got %d", len(callers))
	}
}

// =============================================================================
// Callers Expansion Tests - Edge Cases
// =============================================================================

func TestExpansionEngineCallers_CycleDetection(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func a() {
	b()
}

func b() {
	a()  // mutual recursion
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("a")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'a' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "a",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	// With high depth, should not infinite loop
	callers, err := engine.expandCallers(ctx, ref, 10, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	// Should find 'b' as caller but not loop infinitely
	t.Logf("Found %d callers with cycle detection", len(callers))
}

func TestExpansionEngineCallers_SelfRecursion(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func recursive(n int) int {
	if n <= 1 {
		return n
	}
	return recursive(n-1) + recursive(n-2)
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("recursive")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'recursive' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "recursive",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callers, err := engine.expandCallers(ctx, ref, 5, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	// Self-recursive function should be handled without infinite loop
	t.Logf("Self-recursive function returned %d callers", len(callers))
}

func TestExpansionEngineCallers_TokenBudget(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func caller1() { target() }
func caller2() { target() }
func caller3() { target() }
func target() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("target")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'target' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "target",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()

	// Test with very small token budget (should limit results)
	visited := make(map[string]struct{})
	callersLimited, err := engine.expandCallers(ctx, ref, 1, visited, 10, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	// Test with large token budget (should return all)
	visited2 := make(map[string]struct{})
	callersAll, err := engine.expandCallers(ctx, ref, 1, visited2, 100000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallers failed: %v", err)
	}

	if len(callersAll) < len(callersLimited) {
		t.Error("Higher token budget should return at least as many results")
	}

	t.Logf("Limited budget returned %d callers, full budget returned %d callers",
		len(callersLimited), len(callersAll))
}

// =============================================================================
// Callees Expansion Tests - Happy Path
// =============================================================================

func TestExpansionEngineCallees_Basic(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	helper()
	compute()
}

func helper() {}
func compute() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("main")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'main' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "main",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callees, err := engine.expandCallees(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallees failed: %v", err)
	}

	// Should find both helper and compute
	if len(callees) < 2 {
		t.Errorf("Expected at least 2 callees, got %d", len(callees))
	}

	foundHelper := false
	foundCompute := false
	for _, callee := range callees {
		if callee.Symbol == "helper" {
			foundHelper = true
		}
		if callee.Symbol == "compute" {
			foundCompute = true
		}
	}

	if !foundHelper {
		t.Error("Expected to find 'helper' as callee")
	}
	if !foundCompute {
		t.Error("Expected to find 'compute' as callee")
	}
}

func TestExpansionEngineCallees_MultipleDepth(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	a()
}

func a() {
	b()
}

func b() {
	c()
}

func c() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("main")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'main' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "main",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	// With depth 3, should traverse main -> a -> b -> c
	callees, err := engine.expandCallees(ctx, ref, 3, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandCallees failed: %v", err)
	}

	// Should find 'a' as direct callee
	foundA := false
	for _, callee := range callees {
		if callee.Symbol == "a" {
			foundA = true
			// Check nested callees
			if nested, ok := callee.Expanded["callees"]; ok {
				foundB := false
				for _, nestedCallee := range nested {
					if nestedCallee.Symbol == "b" {
						foundB = true
					}
				}
				if !foundB {
					t.Error("Expected 'b' in nested callees of 'a'")
				}
			}
		}
	}

	if !foundA {
		t.Error("Expected 'a' as direct callee")
	}
}

// =============================================================================
// Callees Expansion Tests - Sad Path
// =============================================================================

func TestExpansionEngineCallees_NoCallees(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func leaf() {
	// no function calls
	x := 1 + 2
	_ = x
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("leaf")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'leaf' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "leaf",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callees, err := engine.expandCallees(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(callees) != 0 {
		t.Errorf("Expected 0 callees for leaf function, got %d", len(callees))
	}
}

func TestExpansionEngineCallees_EmptySymbolName(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "", // Empty symbol name
	}

	ctx := context.Background()
	visited := make(map[string]struct{})

	callees, err := engine.expandCallees(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(callees) != 0 {
		t.Errorf("Expected 0 callees for empty symbol name, got %d", len(callees))
	}
}

// =============================================================================
// Siblings Expansion Tests - Happy Path
// =============================================================================

func TestExpansionEngineSiblings_Basic(t *testing.T) {
	files := map[string]string{
		"calculator.go": `package main

type Calculator struct {
	value int
}

func (c *Calculator) Add(n int) {
	c.value += n
}

func (c *Calculator) Subtract(n int) {
	c.value -= n
}

func (c *Calculator) Multiply(n int) {
	c.value *= n
}

func (c *Calculator) GetValue() int {
	return c.value
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Add")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Add' method")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Add",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	siblings, err := engine.expandSiblings(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandSiblings failed: %v", err)
	}

	// Should find Subtract, Multiply, and GetValue (but not Add itself)
	expectedSiblings := map[string]bool{
		"Subtract": false,
		"Multiply": false,
		"GetValue": false,
	}

	for _, sibling := range siblings {
		if _, ok := expectedSiblings[sibling.Symbol]; ok {
			expectedSiblings[sibling.Symbol] = true
		}
		if sibling.Symbol == "Add" {
			t.Error("Siblings should not include the original method")
		}
	}

	for name, found := range expectedSiblings {
		if !found {
			t.Errorf("Expected to find sibling method '%s'", name)
		}
	}
}

// =============================================================================
// Siblings Expansion Tests - Sad Path
// =============================================================================

func TestExpansionEngineSiblings_NotAMethod(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func regularFunction() {
	// not a method
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("regularFunction")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'regularFunction'")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "regularFunction",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	siblings, err := engine.expandSiblings(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Regular functions have no siblings
	if len(siblings) != 0 {
		t.Errorf("Expected 0 siblings for regular function, got %d", len(siblings))
	}
}

func TestExpansionEngineSiblings_SingleMethod(t *testing.T) {
	files := map[string]string{
		"single.go": `package main

type Single struct{}

func (s *Single) OnlyMethod() {
	// this is the only method
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("OnlyMethod")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'OnlyMethod'")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "OnlyMethod",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	siblings, err := engine.expandSiblings(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Single method has no siblings
	if len(siblings) != 0 {
		t.Errorf("Expected 0 siblings for single method, got %d", len(siblings))
	}
}

func TestExpansionEngineSiblings_EmptySymbolName(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

type T struct{}
func (t *T) Method() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "", // Empty symbol name
	}

	ctx := context.Background()
	siblings, err := engine.expandSiblings(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(siblings) != 0 {
		t.Errorf("Expected 0 siblings for empty symbol name, got %d", len(siblings))
	}
}

// =============================================================================
// Siblings Expansion Tests - Edge Cases
// =============================================================================

func TestExpansionEngineSiblings_MixedMethodsAndFunctions(t *testing.T) {
	files := map[string]string{
		"mixed.go": `package main

type MyType struct{}

func (m *MyType) Method1() {}
func (m *MyType) Method2() {}

func regularFunc() {}

type OtherType struct{}
func (o *OtherType) OtherMethod() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Method1")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Method1'")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Method1",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	siblings, err := engine.expandSiblings(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandSiblings failed: %v", err)
	}

	// Due to fallback behavior (ReceiverType is empty), we include all methods in same file
	// Should NOT include regularFunc (not a method)
	for _, sibling := range siblings {
		if sibling.Symbol == "regularFunc" {
			t.Error("Siblings should not include regular functions")
		}
		if sibling.Symbol == "Method1" {
			t.Error("Siblings should not include self")
		}
	}

	t.Logf("Found %d siblings for Method1", len(siblings))
}

func TestExpansionEngineSiblings_TokenBudget(t *testing.T) {
	files := map[string]string{
		"many_methods.go": `package main

type LargeType struct{}

func (l *LargeType) Method1() { /* some code */ }
func (l *LargeType) Method2() { /* some code */ }
func (l *LargeType) Method3() { /* some code */ }
func (l *LargeType) Method4() { /* some code */ }
func (l *LargeType) Method5() { /* some code */ }
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Method1")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Method1'")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Method1",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()

	// Test with small token budget
	siblingsLimited, err := engine.expandSiblings(ctx, ref, 5, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test with large token budget
	siblingsAll, err := engine.expandSiblings(ctx, ref, 100000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(siblingsAll) < len(siblingsLimited) {
		t.Error("Higher token budget should return at least as many results")
	}

	t.Logf("Limited budget returned %d siblings, full budget returned %d siblings",
		len(siblingsLimited), len(siblingsAll))
}

// =============================================================================
// HydrateReference Tests
// =============================================================================

func TestHydrateReference_WithLineRange(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func hello() {
	println("hello world")
}

func goodbye() {
	println("goodbye")
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "hello",
		L: &types.LineRange{Start: 3, End: 5},
	}

	ctx := context.Background()
	hydrated, tokens, err := engine.HydrateReference(ctx, ref, types.FormatFull, projectRoot)
	if err != nil {
		t.Fatalf("HydrateReference failed: %v", err)
	}

	if hydrated.Source == "" {
		t.Error("Expected non-empty source")
	}

	if tokens <= 0 {
		t.Error("Expected positive token count")
	}

	if hydrated.Symbol != "hello" {
		t.Errorf("Expected symbol 'hello', got %q", hydrated.Symbol)
	}
}

func TestHydrateReference_FileNotFound(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "nonexistent.go",
		S: "foo",
		L: &types.LineRange{Start: 1, End: 5},
	}

	ctx := context.Background()
	_, _, err := engine.HydrateReference(ctx, ref, types.FormatFull, projectRoot)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestHydrateReference_NoSymbolOrLineRange(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		// No symbol name and no line range
	}

	ctx := context.Background()
	_, _, err := engine.HydrateReference(ctx, ref, types.FormatFull, projectRoot)
	if err == nil {
		t.Error("Expected error when neither symbol nor line range provided")
	}
}

func TestHydrateReference_InvalidLineRange(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		L: &types.LineRange{Start: 1000, End: 2000}, // Way beyond file
	}

	ctx := context.Background()
	_, _, err := engine.HydrateReference(ctx, ref, types.FormatFull, projectRoot)
	if err == nil {
		t.Error("Expected error for invalid line range")
	}
}

// =============================================================================
// ApplyExpansions Tests
// =============================================================================

func TestApplyExpansions_MultipleDirectives(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() {
	helper()
}

func helper() {
	compute()
}

func compute() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("helper")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'helper'")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "helper",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
		X: []string{"callers", "callees"},
	}

	ctx := context.Background()
	hydratedRef := &types.HydratedRef{
		File:     ref.F,
		Symbol:   ref.S,
		Source:   "func helper() { compute() }",
		Expanded: make(map[string][]types.HydratedRef),
	}

	tokens, err := engine.ApplyExpansions(ctx, ref, hydratedRef, types.FormatFull, 10000, projectRoot)
	if err != nil {
		t.Fatalf("ApplyExpansions failed: %v", err)
	}

	// Should have both callers and callees expanded
	if _, ok := hydratedRef.Expanded["callers"]; !ok {
		t.Error("Expected 'callers' in expanded refs")
	}
	if _, ok := hydratedRef.Expanded["callees"]; !ok {
		t.Error("Expected 'callees' in expanded refs")
	}

	if tokens <= 0 {
		t.Error("Expected positive token count")
	}
}

func TestApplyExpansions_NoDirectives(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "main",
		X: []string{}, // No directives
	}

	ctx := context.Background()
	hydratedRef := &types.HydratedRef{
		Source:   "func main() {}",
		Expanded: make(map[string][]types.HydratedRef),
	}

	tokens, err := engine.ApplyExpansions(ctx, ref, hydratedRef, types.FormatFull, 10000, projectRoot)
	if err != nil {
		t.Fatalf("ApplyExpansions failed: %v", err)
	}

	if tokens != 0 {
		t.Errorf("Expected 0 tokens for no directives, got %d", tokens)
	}

	if len(hydratedRef.Expanded) != 0 {
		t.Error("Expected no expanded refs")
	}
}

func TestApplyExpansions_UnknownDirective(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "main",
		X: []string{"unknown_directive"},
	}

	ctx := context.Background()
	hydratedRef := &types.HydratedRef{
		Source:   "func main() {}",
		Expanded: make(map[string][]types.HydratedRef),
	}

	// Should not error on unknown directive, just skip it
	_, err := engine.ApplyExpansions(ctx, ref, hydratedRef, types.FormatFull, 10000, projectRoot)
	if err != nil {
		t.Fatalf("Unexpected error for unknown directive: %v", err)
	}
}

func TestApplyExpansions_TokenBudgetExceeded(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func main() { a(); b(); c(); d(); e() }
func a() {}
func b() {}
func c() {}
func d() {}
func e() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "main",
		X: []string{"callees"},
	}

	ctx := context.Background()
	hydratedRef := &types.HydratedRef{
		Source:   "func main() { a(); b(); c(); d(); e() }",
		Expanded: make(map[string][]types.HydratedRef),
	}

	// Very small token budget
	tokens, err := engine.ApplyExpansions(ctx, ref, hydratedRef, types.FormatFull, 1, projectRoot)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should stop early due to budget
	t.Logf("With budget=1, used %d tokens", tokens)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestExpansionEngine_RealWorldScenario(t *testing.T) {
	// Simulate a more realistic codebase structure
	files := map[string]string{
		"cmd/main.go": `package main

import "myapp/internal/service"

func main() {
	svc := service.NewService()
	svc.Run()
}
`,
		"internal/service/service.go": `package service

type Service struct {
	repo *Repository
}

func NewService() *Service {
	return &Service{repo: NewRepository()}
}

func (s *Service) Run() {
	s.repo.Save("data")
}

func (s *Service) Stop() {
	// cleanup
}
`,
		"internal/service/repository.go": `package service

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Save(data string) {
	// save to database
}

func (r *Repository) Load(id string) string {
	return "loaded"
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	ctx := context.Background()

	// Test 1: Find callers of Save
	t.Run("Find callers of Save", func(t *testing.T) {
		refTracker := masterIndex.GetRefTracker()
		symbols := refTracker.FindSymbolsByName("Save")
		if len(symbols) == 0 {
			t.Skip("Could not find 'Save' method")
		}

		fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
		ref := types.ContextRef{
			F: fileInfo.Path,
			S: "Save",
			L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
		}

		visited := make(map[string]struct{})
		callers, err := engine.expandCallers(ctx, ref, 2, visited, 10000, projectRoot, types.FormatFull)
		if err != nil {
			t.Fatalf("expandCallers failed: %v", err)
		}

		t.Logf("Found %d callers of Save", len(callers))
		for _, caller := range callers {
			t.Logf("  - %s in %s", caller.Symbol, caller.File)
		}
	})

	// Test 2: Find siblings of Run method
	t.Run("Find siblings of Run", func(t *testing.T) {
		refTracker := masterIndex.GetRefTracker()
		symbols := refTracker.FindSymbolsByName("Run")
		if len(symbols) == 0 {
			t.Skip("Could not find 'Run' method")
		}

		fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
		ref := types.ContextRef{
			F: fileInfo.Path,
			S: "Run",
			L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
		}

		siblings, err := engine.expandSiblings(ctx, ref, 10000, projectRoot, types.FormatFull)
		if err != nil {
			t.Fatalf("expandSiblings failed: %v", err)
		}

		t.Logf("Found %d siblings of Run", len(siblings))
		for _, sibling := range siblings {
			t.Logf("  - %s", sibling.Symbol)
		}
	})

	// Test 3: Find callees of NewService
	t.Run("Find callees of NewService", func(t *testing.T) {
		refTracker := masterIndex.GetRefTracker()
		symbols := refTracker.FindSymbolsByName("NewService")
		if len(symbols) == 0 {
			t.Skip("Could not find 'NewService' function")
		}

		fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
		ref := types.ContextRef{
			F: fileInfo.Path,
			S: "NewService",
			L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
		}

		visited := make(map[string]struct{})
		callees, err := engine.expandCallees(ctx, ref, 1, visited, 10000, projectRoot, types.FormatFull)
		if err != nil {
			t.Fatalf("expandCallees failed: %v", err)
		}

		t.Logf("Found %d callees of NewService", len(callees))
		for _, callee := range callees {
			t.Logf("  - %s", callee.Symbol)
		}
	})
}

// =============================================================================
// Tests Expansion Tests - Happy Path
// =============================================================================

func TestExpandTests_ExactNameMatch(t *testing.T) {
	files := map[string]string{
		"calculator.go": `package main

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}
`,
		"calculator_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}
}

func TestSubtract(t *testing.T) {
	result := Subtract(5, 3)
	if result != 2 {
		t.Errorf("Expected 2, got %d", result)
	}
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Add")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Add' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Add",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	tests, err := engine.expandTests(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandTests failed: %v", err)
	}

	// Should find TestAdd
	foundTestAdd := false
	for _, test := range tests {
		t.Logf("Found test: %s", test.Symbol)
		if test.Symbol == "TestAdd" {
			foundTestAdd = true
		}
	}

	if !foundTestAdd {
		t.Error("Expected to find 'TestAdd' test function")
	}
}

func TestExpandTests_CallerBased(t *testing.T) {
	files := map[string]string{
		"utils.go": `package main

func Helper() string {
	return "helper"
}
`,
		"utils_test.go": `package main

import "testing"

func TestHelperFunction(t *testing.T) {
	result := Helper()
	if result != "helper" {
		t.Errorf("Expected 'helper', got %s", result)
	}
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Helper")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Helper' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Helper",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	tests, err := engine.expandTests(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandTests failed: %v", err)
	}

	// Should find TestHelperFunction (caller-based discovery)
	t.Logf("Found %d tests for Helper", len(tests))
	for _, test := range tests {
		t.Logf("  - %s", test.Symbol)
	}
}

// =============================================================================
// Tests Expansion Tests - Sad Path
// =============================================================================

func TestExpandTests_NoTestsExist(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func Untested() {
	// no tests exist
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Untested")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Untested' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Untested",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	tests, err := engine.expandTests(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tests) != 0 {
		t.Errorf("Expected 0 tests, got %d", len(tests))
	}
}

func TestExpandTests_EmptySymbolName(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "", // Empty symbol name
	}

	ctx := context.Background()
	tests, err := engine.expandTests(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if tests != nil && len(tests) != 0 {
		t.Errorf("Expected nil or empty tests for empty symbol name")
	}
}

func TestExpandTests_NilRefTracker(t *testing.T) {
	engine := NewExpansionEngine(nil, nil)

	ref := types.ContextRef{
		F: "main.go",
		S: "SomeFunc",
	}

	ctx := context.Background()
	tests, err := engine.expandTests(ctx, ref, 10000, ".", types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if tests != nil && len(tests) != 0 {
		t.Errorf("Expected nil or empty tests with nil refTracker")
	}
}

// =============================================================================
// TypeDeps Expansion Tests - Happy Path
// =============================================================================

func TestExpandTypeDeps_BasicFunction(t *testing.T) {
	files := map[string]string{
		"types.go": `package main

type User struct {
	Name string
	Age  int
}

type Config struct {
	Timeout int
}
`,
		"service.go": `package main

func ProcessUser(user *User, cfg Config) *User {
	return user
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("ProcessUser")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'ProcessUser' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "ProcessUser",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	typeDeps, err := engine.expandTypeDeps(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandTypeDeps failed: %v", err)
	}

	t.Logf("Found %d type dependencies", len(typeDeps))
	for _, dep := range typeDeps {
		t.Logf("  - %s", dep.Symbol)
	}

	// Should find User and Config types
	foundUser := false
	foundConfig := false
	for _, dep := range typeDeps {
		if dep.Symbol == "User" {
			foundUser = true
		}
		if dep.Symbol == "Config" {
			foundConfig = true
		}
	}

	if !foundUser {
		t.Error("Expected to find 'User' type dependency")
	}
	if !foundConfig {
		t.Error("Expected to find 'Config' type dependency")
	}
}

func TestExpandTypeDeps_Method(t *testing.T) {
	files := map[string]string{
		"types.go": `package main

type Request struct {
	Data string
}

type Response struct {
	Result string
}

type Handler struct{}

func (h *Handler) Process(req *Request) *Response {
	return &Response{Result: req.Data}
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Process")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Process' method")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Process",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	typeDeps, err := engine.expandTypeDeps(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("expandTypeDeps failed: %v", err)
	}

	t.Logf("Found %d type dependencies for method", len(typeDeps))
	for _, dep := range typeDeps {
		t.Logf("  - %s", dep.Symbol)
	}

	// Should find Handler (receiver), Request, and Response
	foundHandler := false
	foundRequest := false
	foundResponse := false
	for _, dep := range typeDeps {
		if dep.Symbol == "Handler" {
			foundHandler = true
		}
		if dep.Symbol == "Request" {
			foundRequest = true
		}
		if dep.Symbol == "Response" {
			foundResponse = true
		}
	}

	if !foundHandler {
		t.Error("Expected to find 'Handler' type (receiver)")
	}
	if !foundRequest {
		t.Error("Expected to find 'Request' type (parameter)")
	}
	if !foundResponse {
		t.Error("Expected to find 'Response' type (return)")
	}
}

// =============================================================================
// TypeDeps Expansion Tests - Sad Path
// =============================================================================

func TestExpandTypeDeps_BuiltinTypesOnly(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

func Add(a, b int) int {
	return a + b
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Add")
	if len(symbols) == 0 {
		t.Fatal("Could not find 'Add' function")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Add",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	typeDeps, err := engine.expandTypeDeps(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should find no type deps (int is builtin)
	if len(typeDeps) != 0 {
		t.Errorf("Expected 0 type deps for builtin types only, got %d", len(typeDeps))
	}
}

func TestExpandTypeDeps_SymbolNotFound(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "NonexistentFunc",
	}

	ctx := context.Background()
	typeDeps, err := engine.expandTypeDeps(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(typeDeps) != 0 {
		t.Errorf("Expected 0 type deps for nonexistent symbol, got %d", len(typeDeps))
	}
}

func TestExpandTypeDeps_EmptySymbolName(t *testing.T) {
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)

	ref := types.ContextRef{
		F: "main.go",
		S: "", // Empty symbol name
	}

	ctx := context.Background()
	typeDeps, err := engine.expandTypeDeps(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(typeDeps) != 0 {
		t.Errorf("Expected 0 type deps for empty symbol name, got %d", len(typeDeps))
	}
}

// =============================================================================
// Signature Parsing Helper Tests
// =============================================================================

func TestExtractTypeNamesFromSignature(t *testing.T) {
	engine := &ExpansionEngine{}

	tests := []struct {
		name      string
		signature string
		expected  []string
	}{
		{
			name:      "Simple function with params",
			signature: "func Process(user *User, cfg Config) error",
			expected:  []string{"User", "Config"},
		},
		{
			name:      "Method with receiver",
			signature: "func (h *Handler) Process(req Request) Response",
			expected:  []string{"Handler", "Request", "Response"},
		},
		{
			name:      "Multiple return types",
			signature: "func Parse(data string) (*Result, error)",
			expected:  []string{"Result"},
		},
		{
			name:      "Builtin types only",
			signature: "func Add(a, b int) int",
			expected:  []string{},
		},
		{
			name:      "Slice parameter",
			signature: "func Process(items []Item) []Result",
			expected:  []string{"Item", "Result"},
		},
		{
			name:      "Empty function",
			signature: "func Empty()",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.extractTypeNamesFromSignature(tt.signature)

			// Filter out builtins and empty strings
			var filtered []string
			for _, r := range result {
				if r != "" && !isBuiltinType(r) {
					filtered = append(filtered, r)
				}
			}

			// Check if expected types are present
			for _, exp := range tt.expected {
				found := false
				for _, r := range filtered {
					if r == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected type '%s' not found in result %v", exp, filtered)
				}
			}
		})
	}
}

func TestExtractBaseType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "User"},
		{"*User", "User"},
		{"[]User", "User"},
		{"*[]User", "User"},
		{"context.Context", "Context"},
		{"*pkg.Type", "Type"},
		{"error", "error"},
		{"interface{}", ""},
		{"any", ""},
		{"...string", "string"},
		{"chan int", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractBaseType(tt.input)
			if result != tt.expected {
				t.Errorf("extractBaseType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsBuiltinType(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"int", true},
		{"string", true},
		{"bool", true},
		{"error", true},
		{"any", true},
		{"User", false},
		{"Config", false},
		{"Handler", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBuiltinType(tt.name)
			if result != tt.expected {
				t.Errorf("isBuiltinType(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestSplitParams(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a int, b int", []string{"a int", " b int"}},
		{"user *User, cfg Config", []string{"user *User", " cfg Config"}},
		{"fn func(int, int) int, x int", []string{"fn func(int, int) int", " x int"}},
		{"", []string{}},
		{"single int", []string{"single int"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitParams(tt.input)

			// Handle empty case
			if tt.input == "" {
				if len(result) != 0 {
					t.Errorf("splitParams(%q) = %v, want empty", tt.input, result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("splitParams(%q) returned %d parts, want %d", tt.input, len(result), len(tt.expected))
			}
		})
	}
}

func TestFindMatchingParen(t *testing.T) {
	tests := []struct {
		input    string
		start    int
		expected int
	}{
		{"(a, b)", 0, 5},
		{"(a, (b, c), d)", 0, 13},
		{"((nested))", 0, 9},
		{"no parens", 0, -1},
		{"(unclosed", 0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := findMatchingParen(tt.input, tt.start)
			if result != tt.expected {
				t.Errorf("findMatchingParen(%q, %d) = %d, want %d", tt.input, tt.start, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Implementations/Interface Expansion Tests (Not Supported)
// =============================================================================

func TestExpandImplementations_NotSupported(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

type Reader interface {
	Read() string
}

type FileReader struct{}

func (f *FileReader) Read() string {
	return "file data"
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Reader")
	if len(symbols) == 0 {
		t.Skip("Could not find 'Reader' interface")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Reader",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	impls, err := engine.expandImplementations(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Currently not supported - should return empty
	// This test documents the current behavior
	t.Logf("expandImplementations returned %d results (currently not supported)", len(impls))
}

func TestExpandInterface_NotSupported(t *testing.T) {
	files := map[string]string{
		"main.go": `package main

type Greeter interface {
	Greet() string
}

type Person struct{}

func (p *Person) Greet() string {
	return "Hello"
}
`,
	}

	masterIndex, projectRoot := createTestIndex(t, files)
	defer masterIndex.Close()

	engine := createExpansionEngine(masterIndex)
	refTracker := masterIndex.GetRefTracker()

	symbols := refTracker.FindSymbolsByName("Person")
	if len(symbols) == 0 {
		t.Skip("Could not find 'Person' struct")
	}

	fileInfo := masterIndex.GetFileInfo(symbols[0].FileID)
	ref := types.ContextRef{
		F: fileInfo.Path,
		S: "Person",
		L: &types.LineRange{Start: symbols[0].Line, End: symbols[0].EndLine},
	}

	ctx := context.Background()
	ifaces, err := engine.expandInterface(ctx, ref, 10000, projectRoot, types.FormatFull)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Currently not supported - should return empty
	// This test documents the current behavior
	t.Logf("expandInterface returned %d results (currently not supported)", len(ifaces))
}
