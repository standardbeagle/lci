package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestSymbolLocationIndex_IndexAndFind tests the symbol location index index and find.
func TestSymbolLocationIndex_IndexAndFind(t *testing.T) {
	index := NewSymbolLocationIndex()

	// Create test symbols
	symbols := []types.Symbol{
		{
			Name:      "TestFunction",
			Type:      types.SymbolTypeFunction,
			Line:      10,
			Column:    5,
			EndLine:   15,
			EndColumn: 10,
		},
		{
			Name:      "TestVariable",
			Type:      types.SymbolTypeVariable,
			Line:      20,
			Column:    8,
			EndLine:   20,
			EndColumn: 20,
		},
		{
			Name:      "TestClass",
			Type:      types.SymbolTypeClass,
			Line:      5,
			Column:    0,
			EndLine:   25,
			EndColumn: 5,
		},
	}

	fileID := types.FileID(1)
	index.IndexFileSymbols(fileID, symbols, nil)

	// Test finding symbols at different positions
	tests := []struct {
		line     int
		column   int
		expected string
		found    bool
	}{
		{10, 5, "TestFunction", true},  // Start of function
		{12, 8, "TestFunction", true},  // Middle of function
		{15, 10, "TestFunction", true}, // End of function
		{20, 10, "TestVariable", true}, // Middle of variable
		{8, 2, "TestClass", true},      // Inside class (should be most specific)
		{12, 8, "TestFunction", true},  // Function should be more specific than class
		{30, 5, "", false},             // Outside all symbols
	}

	for _, test := range tests {
		symbol := index.FindSymbolAtPosition(fileID, test.line, test.column)

		if test.found {
			if symbol == nil {
				t.Errorf("Expected to find symbol at line %d, column %d", test.line, test.column)
			} else if symbol.Name != test.expected {
				t.Errorf("At line %d, column %d: expected symbol %s, got %s",
					test.line, test.column, test.expected, symbol.Name)
			}
		} else {
			if symbol != nil {
				t.Errorf("Expected no symbol at line %d, column %d, but found %s",
					test.line, test.column, symbol.Name)
			}
		}
	}
}

// TestSymbolLocationIndex_MostSpecificSymbol tests the symbol location index most specific symbol.
func TestSymbolLocationIndex_MostSpecificSymbol(t *testing.T) {
	index := NewSymbolLocationIndex()

	// Create nested symbols (class containing a function)
	symbols := []types.Symbol{
		{
			Name:      "OuterClass",
			Type:      types.SymbolTypeClass,
			Line:      1,
			Column:    0,
			EndLine:   20,
			EndColumn: 5,
		},
		{
			Name:      "InnerFunction",
			Type:      types.SymbolTypeFunction,
			Line:      5,
			Column:    4,
			EndLine:   10,
			EndColumn: 8,
		},
	}

	fileID := types.FileID(1)
	index.IndexFileSymbols(fileID, symbols, nil)

	// At position (7, 6) - inside both class and function
	// Should return the more specific (smaller) symbol
	symbol := index.FindSymbolAtPosition(fileID, 7, 6)

	if symbol == nil {
		t.Fatal("Expected to find a symbol")
	}

	if symbol.Name != "InnerFunction" {
		t.Errorf("Expected most specific symbol 'InnerFunction', got '%s'", symbol.Name)
	}
}

// TestSymbolLocationIndex_GetFileSymbols tests the symbol location index get file symbols.
func TestSymbolLocationIndex_GetFileSymbols(t *testing.T) {
	index := NewSymbolLocationIndex()

	symbols := []types.Symbol{
		{Name: "Symbol1", Type: types.SymbolTypeFunction, Line: 1, Column: 0, EndLine: 5, EndColumn: 0},
		{Name: "Symbol2", Type: types.SymbolTypeVariable, Line: 10, Column: 0, EndLine: 10, EndColumn: 10},
		{Name: "Symbol3", Type: types.SymbolTypeClass, Line: 15, Column: 0, EndLine: 20, EndColumn: 0},
	}

	fileID := types.FileID(1)
	index.IndexFileSymbols(fileID, symbols, nil)

	retrievedSymbols := index.GetFileSymbols(fileID)

	if len(retrievedSymbols) != len(symbols) {
		t.Errorf("Expected %d symbols, got %d", len(symbols), len(retrievedSymbols))
	}

	// Create a map for easy lookup
	symbolMap := make(map[string]bool)
	for _, symbol := range retrievedSymbols {
		symbolMap[symbol.Name] = true
	}

	// Check all original symbols are present
	for _, original := range symbols {
		if !symbolMap[original.Name] {
			t.Errorf("Symbol %s not found in retrieved symbols", original.Name)
		}
	}
}

// TestSymbolLocationIndex_RemoveFile tests the symbol location index remove file.
func TestSymbolLocationIndex_RemoveFile(t *testing.T) {
	index := NewSymbolLocationIndex()

	symbols := []types.Symbol{
		{Name: "TestSymbol", Type: types.SymbolTypeFunction, Line: 1, Column: 0, EndLine: 5, EndColumn: 0},
	}

	fileID := types.FileID(1)
	index.IndexFileSymbols(fileID, symbols, nil)

	// Verify symbol exists
	symbol := index.FindSymbolAtPosition(fileID, 3, 2)
	if symbol == nil {
		t.Fatal("Symbol should exist before removal")
	}

	// Remove file
	index.RemoveFile(fileID)

	// Verify symbol is gone
	symbol = index.FindSymbolAtPosition(fileID, 3, 2)
	if symbol != nil {
		t.Fatal("Symbol should not exist after file removal")
	}

	// Verify GetFileSymbols returns empty
	fileSymbols := index.GetFileSymbols(fileID)
	if len(fileSymbols) != 0 {
		t.Errorf("Expected 0 symbols after file removal, got %d", len(fileSymbols))
	}
}

// TestSymbolLocationIndex_Clear tests the symbol location index clear.
func TestSymbolLocationIndex_Clear(t *testing.T) {
	index := NewSymbolLocationIndex()

	symbols := []types.Symbol{
		{Name: "TestSymbol", Type: types.SymbolTypeFunction, Line: 1, Column: 0, EndLine: 5, EndColumn: 0},
	}

	fileID1 := types.FileID(1)
	fileID2 := types.FileID(2)

	index.IndexFileSymbols(fileID1, symbols, nil)
	index.IndexFileSymbols(fileID2, symbols, nil)

	// Verify data exists
	stats := index.GetStats()
	if stats["indexed_files"].(int) != 2 {
		t.Fatal("Expected 2 indexed files before clear")
	}

	// Clear and verify
	index.Clear()

	stats = index.GetStats()
	if stats["indexed_files"].(int) != 0 {
		t.Error("Expected 0 indexed files after clear")
	}

	if stats["total_symbols"].(int) != 0 {
		t.Error("Expected 0 total symbols after clear")
	}
}

// TestSymbolLocationIndex_GetStats tests the symbol location index get stats.
func TestSymbolLocationIndex_GetStats(t *testing.T) {
	index := NewSymbolLocationIndex()

	// File 1 with 2 symbols
	symbols1 := []types.Symbol{
		{Name: "Symbol1", Type: types.SymbolTypeFunction, Line: 1, Column: 0, EndLine: 5, EndColumn: 0},
		{Name: "Symbol2", Type: types.SymbolTypeVariable, Line: 10, Column: 0, EndLine: 10, EndColumn: 10},
	}

	// File 2 with 1 symbol
	symbols2 := []types.Symbol{
		{Name: "Symbol3", Type: types.SymbolTypeClass, Line: 1, Column: 0, EndLine: 10, EndColumn: 0},
	}

	index.IndexFileSymbols(types.FileID(1), symbols1, nil)
	index.IndexFileSymbols(types.FileID(2), symbols2, nil)

	stats := index.GetStats()

	if stats["indexed_files"].(int) != 2 {
		t.Errorf("Expected 2 indexed files, got %v", stats["indexed_files"])
	}

	if stats["total_symbols"].(int) != 3 {
		t.Errorf("Expected 3 total symbols, got %v", stats["total_symbols"])
	}

	expectedAvg := 3.0 / 2.0
	if stats["avg_per_file"].(float64) != expectedAvg {
		t.Errorf("Expected avg_per_file %.2f, got %v", expectedAvg, stats["avg_per_file"])
	}
}

// TestSymbolLocationIndex_EdgeCases tests the symbol location index edge cases.
func TestSymbolLocationIndex_EdgeCases(t *testing.T) {
	index := NewSymbolLocationIndex()

	// Test with empty symbol list
	index.IndexFileSymbols(types.FileID(1), []types.Symbol{}, nil)

	symbol := index.FindSymbolAtPosition(types.FileID(1), 5, 5)
	if symbol != nil {
		t.Error("Expected no symbol for empty file")
	}

	// Test with non-existent file
	symbol = index.FindSymbolAtPosition(types.FileID(999), 5, 5)
	if symbol != nil {
		t.Error("Expected no symbol for non-existent file")
	}

	// Test zero-width symbol
	zeroWidthSymbol := []types.Symbol{
		{Name: "ZeroWidth", Type: types.SymbolTypeVariable, Line: 5, Column: 10, EndLine: 5, EndColumn: 10},
	}

	index.IndexFileSymbols(types.FileID(2), zeroWidthSymbol, nil)

	// Should find at exact position
	symbol = index.FindSymbolAtPosition(types.FileID(2), 5, 10)
	if symbol == nil || symbol.Name != "ZeroWidth" {
		t.Error("Should find zero-width symbol at exact position")
	}

	// Should not find at adjacent position
	symbol = index.FindSymbolAtPosition(types.FileID(2), 5, 11)
	if symbol != nil {
		t.Error("Should not find zero-width symbol at adjacent position")
	}
}

// TestSymbolLocationIndex_ConcurrentAccess tests the symbol location index concurrent access.
func TestSymbolLocationIndex_ConcurrentAccess(t *testing.T) {
	index := NewSymbolLocationIndex()

	symbols := []types.Symbol{
		{Name: "ConcurrentSymbol", Type: types.SymbolTypeFunction, Line: 1, Column: 0, EndLine: 5, EndColumn: 0},
	}

	fileID := types.FileID(1)
	index.IndexFileSymbols(fileID, symbols, nil)

	// Test concurrent reads (should not panic)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Multiple read operations
			for j := 0; j < 100; j++ {
				index.FindSymbolAtPosition(fileID, 3, 2)
				index.GetFileSymbols(fileID)
				index.GetStats()
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify data is still intact
	symbol := index.FindSymbolAtPosition(fileID, 3, 2)
	if symbol == nil || symbol.Name != "ConcurrentSymbol" {
		t.Error("Data corruption after concurrent access")
	}
}
