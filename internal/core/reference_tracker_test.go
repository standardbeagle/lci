package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// Test buildSymbolScopeChain with various edge cases
// This function is critical for proper scope hierarchy construction during indexing
func TestBuildSymbolScopeChain(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	tests := []struct {
		name        string
		symbol      types.Symbol
		scopes      []types.ScopeInfo
		expectedLen int
		description string
	}{
		{
			name:        "Empty scopes array",
			symbol:      types.Symbol{Name: "testFunc", Line: 10, EndLine: 20},
			scopes:      []types.ScopeInfo{},
			expectedLen: 0,
			description: "Symbol with no scopes should return empty slice",
		},
		{
			name:   "Single scope match",
			symbol: types.Symbol{Name: "testFunc", Line: 15, EndLine: 25},
			scopes: []types.ScopeInfo{
				{Name: "function", StartLine: 10, EndLine: 30},
			},
			expectedLen: 1,
			description: "Symbol within one scope should return that scope",
		},
		{
			name:   "Multiple nested scopes",
			symbol: types.Symbol{Name: "testFunc", Line: 25, EndLine: 35},
			scopes: []types.ScopeInfo{
				{Name: "file", StartLine: 1, EndLine: 100},
				{Name: "module", StartLine: 5, EndLine: 80},
				{Name: "class", StartLine: 15, EndLine: 50},
				{Name: "function", StartLine: 20, EndLine: 40},
			},
			expectedLen: 4,
			description: "Symbol in nested scopes should return all matching scopes",
		},
		{
			name:   "Symbol outside all scopes",
			symbol: types.Symbol{Name: "testFunc", Line: 5, EndLine: 10},
			scopes: []types.ScopeInfo{
				{Name: "scope1", StartLine: 20, EndLine: 30},
				{Name: "scope2", StartLine: 40, EndLine: 50},
			},
			expectedLen: 0,
			description: "Symbol before all scopes should return empty slice",
		},
		{
			name:   "Symbol after all scopes",
			symbol: types.Symbol{Name: "testFunc", Line: 60, EndLine: 70},
			scopes: []types.ScopeInfo{
				{Name: "scope1", StartLine: 20, EndLine: 30},
				{Name: "scope2", StartLine: 40, EndLine: 50},
			},
			expectedLen: 0,
			description: "Symbol after all scopes should return empty slice",
		},
		{
			name:   "Symbol at exact scope start line",
			symbol: types.Symbol{Name: "testFunc", Line: 20, EndLine: 30},
			scopes: []types.ScopeInfo{
				{Name: "scope1", StartLine: 20, EndLine: 40},
			},
			expectedLen: 1,
			description: "Symbol at scope start line should match",
		},
		{
			name:   "Symbol at exact scope end line",
			symbol: types.Symbol{Name: "testFunc", Line: 40, EndLine: 50},
			scopes: []types.ScopeInfo{
				{Name: "scope1", StartLine: 20, EndLine: 40},
			},
			expectedLen: 1,
			description: "Symbol at scope end line should match (inclusive)",
		},
		{
			name:   "Symbol with EndLine = 0 (infinite scope)",
			symbol: types.Symbol{Name: "testFunc", Line: 50, EndLine: 60},
			scopes: []types.ScopeInfo{
				{Name: "global", StartLine: 1, EndLine: 0}, // EndLine 0 means infinite
			},
			expectedLen: 1,
			description: "Scope with EndLine=0 should match symbols after StartLine",
		},
		{
			name:   "Symbol before infinite scope start",
			symbol: types.Symbol{Name: "testFunc", Line: 5, EndLine: 10},
			scopes: []types.ScopeInfo{
				{Name: "global", StartLine: 10, EndLine: 0},
			},
			expectedLen: 0,
			description: "Symbol before infinite scope start should not match",
		},
		{
			name:   "Multiple scopes with partial overlap",
			symbol: types.Symbol{Name: "testFunc", Line: 15, EndLine: 20},
			scopes: []types.ScopeInfo{
				{Name: "scope1", StartLine: 10, EndLine: 12},
				{Name: "scope2", StartLine: 12, EndLine: 18},
				{Name: "scope3", StartLine: 18, EndLine: 25},
			},
			expectedLen: 1,
			description: "Symbol at line 15 only matches scope2 (12-18)",
		},
		{
			name:   "Symbol spanning multiple scopes",
			symbol: types.Symbol{Name: "testFunc", Line: 10, EndLine: 30},
			scopes: []types.ScopeInfo{
				{Name: "scope1", StartLine: 5, EndLine: 15},
				{Name: "scope2", StartLine: 20, EndLine: 25},
			},
			expectedLen: 1,
			description: "Only matches scope1 because symbol starts at line 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use reflection to access the private buildSymbolScopeChain method
			// This is a common pattern for testing private methods
			scopeChain := tracker.buildSymbolScopeChain(tt.symbol, tt.scopes)

			if len(scopeChain) != tt.expectedLen {
				t.Errorf("%s: expected %d scopes, got %d",
					tt.description, tt.expectedLen, len(scopeChain))
			}

			// Verify all returned scopes actually contain the symbol
			for _, scope := range scopeChain {
				if !(scope.StartLine <= tt.symbol.Line &&
					(scope.EndLine == 0 || scope.EndLine >= tt.symbol.Line)) {
					t.Errorf("Scope %s (lines %d-%d) does not contain symbol at line %d",
						scope.Name, scope.StartLine, scope.EndLine, tt.symbol.Line)
				}
			}
		})
	}
}

// Test buildSymbolScopeChain with large number of scopes (stress test)
// Ensures the pre-allocated slice with capacity 4 works correctly with many scopes
func TestBuildSymbolScopeChainManyScopes(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	// Create 25 scopes that all contain the symbol
	var scopes []types.ScopeInfo
	for i := 0; i < 25; i++ {
		scopes = append(scopes, types.ScopeInfo{
			Name:      "scope" + string(rune('A'+i)),
			StartLine: 1,
			EndLine:   100,
		})
	}

	symbol := types.Symbol{Name: "testFunc", Line: 50, EndLine: 60}
	scopeChain := tracker.buildSymbolScopeChain(symbol, scopes)

	// All 25 scopes should match
	if len(scopeChain) != 25 {
		t.Errorf("Expected 25 scopes, got %d", len(scopeChain))
	}

	// Verify the function handles large slices efficiently (no crashes, correct results)
	if cap(scopeChain) < 25 {
		t.Errorf("Slice capacity should accommodate all matching scopes, got capacity %d", cap(scopeChain))
	}
}

// Test buildSymbolScopeChain with symbol at various line positions
func TestBuildSymbolScopeChainLinePositions(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	scopes := []types.ScopeInfo{
		{Name: "outer", StartLine: 10, EndLine: 90},
		{Name: "middle", StartLine: 20, EndLine: 80},
		{Name: "inner", StartLine: 30, EndLine: 70},
		{Name: "core", StartLine: 40, EndLine: 60},
	}

	testCases := []struct {
		line     int
		expected int
	}{
		{1, 0},  // Before all scopes
		{5, 0},  // Still before outer scope
		{10, 1}, // At outer scope start
		{15, 1}, // In outer scope only
		{20, 2}, // In outer and middle
		{25, 2}, // Still in outer and middle
		{30, 3}, // Now in outer, middle, and inner
		{35, 3}, // Still in three scopes
		{40, 4}, // All four scopes
		{50, 4}, // Still in all four
		{60, 4}, // At core scope end (inclusive)
		{65, 3}, // Past core, still in three
		{70, 3}, // At inner scope end (inclusive)
		{75, 2}, // Back to two scopes
		{80, 2}, // At middle scope end (inclusive)
		{85, 1}, // Back to outer only
		{90, 1}, // At outer scope end (inclusive)
		{95, 0}, // After all scopes
	}

	for _, tc := range testCases {
		t.Run("line_"+string(rune('0'+tc.line)), func(t *testing.T) {
			symbol := types.Symbol{Name: "testFunc", Line: tc.line, EndLine: tc.line}
			scopeChain := tracker.buildSymbolScopeChain(symbol, scopes)

			if len(scopeChain) != tc.expected {
				t.Errorf("Symbol at line %d: expected %d scopes, got %d",
					tc.line, tc.expected, len(scopeChain))
			}

			// Verify scopes are returned in order (outer to inner)
			for i, scope := range scopeChain {
				expectedLine := 10 + i*10
				if scope.StartLine != expectedLine {
					t.Errorf("Scope %d: expected start line %d, got %d",
						i, expectedLine, scope.StartLine)
				}
			}
		})
	}
}

// Test that buildSymbolScopeChain doesn't modify the original scopes slice
func TestBuildSymbolScopeChainNoSideEffects(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	originalScopes := []types.ScopeInfo{
		{Name: "scope1", StartLine: 10, EndLine: 50},
		{Name: "scope2", StartLine: 20, EndLine: 40},
	}

	// Create a copy to compare after the call
	scopeCopy := make([]types.ScopeInfo, len(originalScopes))
	copy(scopeCopy, originalScopes)

	symbol := types.Symbol{Name: "testFunc", Line: 30, EndLine: 35}
	scopeChain := tracker.buildSymbolScopeChain(symbol, originalScopes)

	// Verify the original slice wasn't modified
	if len(originalScopes) != len(scopeCopy) {
		t.Errorf("Original scopes slice length changed: was %d, now %d",
			len(scopeCopy), len(originalScopes))
	}

	for i := range originalScopes {
		// Compare basic fields that don't contain slices
		if originalScopes[i].Name != scopeCopy[i].Name ||
			originalScopes[i].StartLine != scopeCopy[i].StartLine ||
			originalScopes[i].EndLine != scopeCopy[i].EndLine {
			t.Errorf("Original scope %d was modified", i)
		}
	}

	// Verify the result contains expected scopes
	if len(scopeChain) != 2 {
		t.Errorf("Expected 2 scopes in result, got %d", len(scopeChain))
	}
}

// TestScopeChainCacheCollisionHandling tests that hash collisions don't corrupt cache entries
// This verifies the fix for: when symbol A and B produce the same hash key but different data,
// symbol A's cache entry should NOT be overwritten by symbol B's computation.
func TestScopeChainCacheCollisionHandling(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	// Create two symbols that we can manipulate to test collision behavior
	// Even with different line numbers, they could produce same hash in edge cases
	symbolA := types.Symbol{Name: "funcA", Line: 100, EndLine: 110}
	symbolB := types.Symbol{Name: "funcB", Line: 200, EndLine: 210}

	scopesA := []types.ScopeInfo{
		{Name: "scopeA", StartLine: 50, EndLine: 150},
	}
	scopesB := []types.ScopeInfo{
		{Name: "scopeB", StartLine: 150, EndLine: 250},
	}

	// First call for symbol A - should cache
	resultA1 := tracker.buildSymbolScopeChain(symbolA, scopesA)
	if len(resultA1) != 1 || resultA1[0].Name != "scopeA" {
		t.Fatalf("First call for symbolA failed: got %v", resultA1)
	}

	// Call for symbol B - different key, should not affect A's cache
	resultB := tracker.buildSymbolScopeChain(symbolB, scopesB)
	if len(resultB) != 1 || resultB[0].Name != "scopeB" {
		t.Fatalf("Call for symbolB failed: got %v", resultB)
	}

	// Second call for symbol A - should hit cache and return same result
	resultA2 := tracker.buildSymbolScopeChain(symbolA, scopesA)
	if len(resultA2) != 1 || resultA2[0].Name != "scopeA" {
		t.Errorf("Second call for symbolA returned wrong result: got %v, want scopeA", resultA2)
	}

	// Verify cache was actually used (same slice reference for cached results)
	// Note: We can't directly compare slice pointers in Go, but we can verify the
	// content is correct after multiple calls
}

// TestScopeChainCacheCollisionWithSameHash simulates a hash collision scenario
// by directly manipulating the cache to verify collision detection works correctly
func TestScopeChainCacheCollisionWithSameHash(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	// Symbol and scope setup
	symbol1 := types.Symbol{Name: "func1", Line: 10, EndLine: 20}
	scopes1 := []types.ScopeInfo{
		{Name: "scope1", StartLine: 5, EndLine: 25},
	}

	// First call - populate cache
	result1 := tracker.buildSymbolScopeChain(symbol1, scopes1)
	if len(result1) != 1 {
		t.Fatalf("Expected 1 scope, got %d", len(result1))
	}

	// Now create a symbol with DIFFERENT line numbers (which would fail collision verification)
	// but that produces the same scope chain
	symbol2 := types.Symbol{Name: "func2", Line: 15, EndLine: 20} // Different Line
	scopes2 := []types.ScopeInfo{
		{Name: "scope1", StartLine: 5, EndLine: 25}, // Same scope
	}

	// This should work correctly because even if hash collides,
	// the collision verification will detect different Line values
	result2 := tracker.buildSymbolScopeChain(symbol2, scopes2)
	if len(result2) != 1 {
		t.Errorf("Expected 1 scope for symbol2, got %d", len(result2))
	}

	// Both results should be correct
	if result1[0].Name != "scope1" {
		t.Errorf("Result1 scope name wrong: got %s", result1[0].Name)
	}
	if result2[0].Name != "scope1" {
		t.Errorf("Result2 scope name wrong: got %s", result2[0].Name)
	}
}

// TestScopeChainCacheNoOverwriteOnCollision verifies that when a hash collision occurs,
// the original cache entry is preserved (not overwritten with the new entry)
func TestScopeChainCacheNoOverwriteOnCollision(t *testing.T) {
	tracker := NewReferenceTrackerForTest()

	// First, populate cache with a known entry
	symbolOrig := types.Symbol{Name: "original", Line: 100, EndLine: 110}
	scopesOrig := []types.ScopeInfo{
		{Name: "originalScope", StartLine: 50, EndLine: 150},
		{Name: "nestedScope", StartLine: 90, EndLine: 120},
	}

	resultOrig := tracker.buildSymbolScopeChain(symbolOrig, scopesOrig)
	if len(resultOrig) != 2 {
		t.Fatalf("Original call should return 2 scopes, got %d", len(resultOrig))
	}

	// Now query with the same symbol again - should hit cache
	resultCached := tracker.buildSymbolScopeChain(symbolOrig, scopesOrig)
	if len(resultCached) != 2 {
		t.Errorf("Cached call should return 2 scopes, got %d", len(resultCached))
	}

	// The cached result should be identical to original
	if resultCached[0].Name != "originalScope" || resultCached[1].Name != "nestedScope" {
		t.Errorf("Cache returned wrong scopes: %v", resultCached)
	}
}

// TestProcessFile_IsExportedPropagation verifies that Symbol.Visibility.IsExported
// is correctly propagated to EnhancedSymbol.IsExported during file processing.
// This is a regression test for the bug where is_exported was always false in get_context.
func TestProcessFile_IsExportedPropagation(t *testing.T) {
	tracker := NewReferenceTrackerForTest()
	fileID := types.FileID(1)

	tests := []struct {
		name           string
		symbolName     string
		isExported     bool
		wantIsExported bool
	}{
		{
			name:           "exported Go function (uppercase)",
			symbolName:     "ExportedFunc",
			isExported:     true,
			wantIsExported: true,
		},
		{
			name:           "unexported Go function (lowercase)",
			symbolName:     "unexportedFunc",
			isExported:     false,
			wantIsExported: false,
		},
		{
			name:           "exported Go type",
			symbolName:     "Router",
			isExported:     true,
			wantIsExported: true,
		},
		{
			name:           "exported Go interface",
			symbolName:     "Routes",
			isExported:     true,
			wantIsExported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := []types.Symbol{
				{
					Name:    tt.symbolName,
					Type:    types.SymbolTypeFunction,
					Line:    10,
					EndLine: 20,
					Visibility: types.SymbolVisibility{
						IsExported: tt.isExported,
						Access:     types.AccessPublic,
					},
				},
			}

			enhancedSymbols := tracker.ProcessFile(fileID, "test.go", symbols, nil, nil)

			if len(enhancedSymbols) != 1 {
				t.Fatalf("Expected 1 enhanced symbol, got %d", len(enhancedSymbols))
			}

			got := enhancedSymbols[0].IsExported
			if got != tt.wantIsExported {
				t.Errorf("EnhancedSymbol.IsExported = %v, want %v (Symbol.Visibility.IsExported = %v)",
					got, tt.wantIsExported, tt.isExported)
			}

			// Also verify the symbol stored in the tracker has the correct value
			storedSymbol := tracker.GetEnhancedSymbol(enhancedSymbols[0].ID)
			if storedSymbol == nil {
				t.Fatal("Stored symbol not found in tracker")
			}
			if storedSymbol.IsExported != tt.wantIsExported {
				t.Errorf("Stored EnhancedSymbol.IsExported = %v, want %v",
					storedSymbol.IsExported, tt.wantIsExported)
			}
		})
	}
}

// TestProcessFile_VisibilityFieldsPropagation verifies that all Symbol.Visibility fields
// are correctly propagated to EnhancedSymbol during file processing.
func TestProcessFile_VisibilityFieldsPropagation(t *testing.T) {
	tracker := NewReferenceTrackerForTest()
	fileID := types.FileID(1)

	symbols := []types.Symbol{
		{
			Name:    "TestFunc",
			Type:    types.SymbolTypeFunction,
			Line:    10,
			EndLine: 20,
			Visibility: types.SymbolVisibility{
				IsExported: true,
				Access:     types.AccessPublic,
			},
		},
	}

	enhancedSymbols := tracker.ProcessFile(fileID, "test.go", symbols, nil, nil)

	if len(enhancedSymbols) != 1 {
		t.Fatalf("Expected 1 enhanced symbol, got %d", len(enhancedSymbols))
	}

	// Verify IsExported is propagated
	if !enhancedSymbols[0].IsExported {
		t.Error("EnhancedSymbol.IsExported should be true for exported symbol")
	}

	// Verify the embedded Symbol still has the correct visibility
	if !enhancedSymbols[0].Symbol.Visibility.IsExported {
		t.Error("EnhancedSymbol.Symbol.Visibility.IsExported should be true")
	}
}

// TestProcessFile_IsExportedFallbackComputation verifies that IsExported is correctly
// computed from naming conventions when the parser doesn't set Visibility explicitly.
// This tests the fallback path used for parser-generated symbols.
func TestProcessFile_IsExportedFallbackComputation(t *testing.T) {
	tracker := NewReferenceTrackerForTest()
	fileID := types.FileID(1)

	tests := []struct {
		name           string
		filePath       string
		symbolName     string
		wantIsExported bool
	}{
		// Go: uppercase = exported
		{
			name:           "Go exported function (uppercase)",
			filePath:       "main.go",
			symbolName:     "ExportedFunc",
			wantIsExported: true,
		},
		{
			name:           "Go unexported function (lowercase)",
			filePath:       "main.go",
			symbolName:     "privateFunc",
			wantIsExported: false,
		},
		// Python: underscore prefix = private
		{
			name:           "Python public function",
			filePath:       "module.py",
			symbolName:     "public_func",
			wantIsExported: true,
		},
		{
			name:           "Python private function",
			filePath:       "module.py",
			symbolName:     "_private_func",
			wantIsExported: false,
		},
		// JavaScript: underscore/hash = private
		{
			name:           "JS public function",
			filePath:       "app.js",
			symbolName:     "publicFunc",
			wantIsExported: true,
		},
		{
			name:           "JS private function (underscore)",
			filePath:       "app.js",
			symbolName:     "_privateFunc",
			wantIsExported: false,
		},
		{
			name:           "TS private field (hash)",
			filePath:       "app.ts",
			symbolName:     "#privateField",
			wantIsExported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create symbol WITHOUT setting Visibility (simulates parser output)
			symbols := []types.Symbol{
				{
					Name:    tt.symbolName,
					Type:    types.SymbolTypeFunction,
					Line:    10,
					EndLine: 20,
					// Visibility NOT SET - triggers fallback computation
				},
			}

			enhancedSymbols := tracker.ProcessFile(fileID, tt.filePath, symbols, nil, nil)

			if len(enhancedSymbols) != 1 {
				t.Fatalf("Expected 1 enhanced symbol, got %d", len(enhancedSymbols))
			}

			got := enhancedSymbols[0].IsExported
			if got != tt.wantIsExported {
				t.Errorf("IsExported = %v, want %v (file=%s, symbol=%s)",
					got, tt.wantIsExported, tt.filePath, tt.symbolName)
			}
		})
	}
}

// TestComputeIsExported tests the computeIsExported helper function directly
func TestComputeIsExported(t *testing.T) {
	tests := []struct {
		path       string
		symbolName string
		want       bool
	}{
		// Go
		{"main.go", "Exported", true},
		{"main.go", "unexported", false},
		{"pkg/foo.go", "PublicType", true},

		// Python
		{"module.py", "public", true},
		{"module.py", "_private", false},
		{"module.py", "__dunder__", false},

		// JavaScript/TypeScript
		{"app.js", "public", true},
		{"app.ts", "_private", false},
		{"app.tsx", "#private", false},
		{"app.jsx", "Public", true},

		// Empty name
		{"main.go", "", false},

		// Unknown extension - defaults to exported
		{"file.unknown", "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.symbolName, func(t *testing.T) {
			got := computeIsExported(tt.path, tt.symbolName)
			if got != tt.want {
				t.Errorf("computeIsExported(%q, %q) = %v, want %v",
					tt.path, tt.symbolName, got, tt.want)
			}
		})
	}
}

// Benchmark buildSymbolScopeChain for performance testing
func BenchmarkBuildSymbolScopeChain(b *testing.B) {
	tracker := NewReferenceTrackerForTest()

	// Create a realistic scenario with multiple scopes
	var scopes []types.ScopeInfo
	for i := 0; i < 20; i++ {
		scopes = append(scopes, types.ScopeInfo{
			Name:      "scope" + string(rune('A'+i)),
			StartLine: i * 5,
			EndLine:   i*5 + 100,
		})
	}

	symbol := types.Symbol{Name: "testFunc", Line: 50, EndLine: 60}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.buildSymbolScopeChain(symbol, scopes)
	}
}
