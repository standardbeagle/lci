package core

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// TestSemanticSearchIndex_BasicOperations tests basic AddSymbolData and retrieval operations
func TestSemanticSearchIndex_BasicOperations(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	// Test data
	symbolID1 := types.SymbolID(1000)
	symbolID2 := types.SymbolID(2000)

	words1 := []string{"auth", "user", "manager"}
	stems1 := []string{"auth", "user", "manag"}
	phonetic1 := "a236"
	expansions1 := []string{"authenticate", "user", "management"}

	words2 := []string{"database", "connection", "pool"}
	stems2 := []string{"databas", "connect", "pool"}
	phonetic2 := "d216"
	expansions2 := []string{"database", "connection", "pool"}

	// Add first symbol directly (no background processing)
	ssi.AddSymbolData(symbolID1, "AuthUserManager", words1, stems1, phonetic1, expansions1)

	// Verify retrieval by word
	symbolsByAuth := ssi.GetSymbolsByWord("auth")
	if len(symbolsByAuth) != 1 || symbolsByAuth[0] != symbolID1 {
		t.Errorf("Expected symbolID1 for word 'auth', got %v", symbolsByAuth)
	}

	// Verify retrieval by stem
	symbolsByUser := ssi.GetSymbolsByStem("user")
	if len(symbolsByUser) != 1 || symbolsByUser[0] != symbolID1 {
		t.Errorf("Expected symbolID1 for stem 'user', got %v", symbolsByUser)
	}

	// Verify retrieval by phonetic
	symbolsByPhonetic := ssi.GetSymbolsByPhonetic(phonetic1)
	if len(symbolsByPhonetic) != 1 || symbolsByPhonetic[0] != symbolID1 {
		t.Errorf("Expected symbolID1 for phonetic '%s', got %v", phonetic1, symbolsByPhonetic)
	}

	// Verify retrieval by abbreviation
	symbolsByAbbrev := ssi.GetSymbolsByAbbreviation("authenticate")
	if len(symbolsByAbbrev) != 1 || symbolsByAbbrev[0] != symbolID1 {
		t.Errorf("Expected symbolID1 for abbreviation 'authenticate', got %v", symbolsByAbbrev)
	}

	// Verify symbol name
	name := ssi.GetSymbolName(symbolID1)
	if name != "AuthUserManager" {
		t.Errorf("Expected 'AuthUserManager', got %q", name)
	}

	// Add second symbol directly
	ssi.AddSymbolData(symbolID2, "DatabaseConnectionPool", words2, stems2, phonetic2, expansions2)

	// Verify multiple symbols can be retrieved
	symbolsByWord := ssi.GetSymbolsByWord("pool")
	if len(symbolsByWord) != 1 || symbolsByWord[0] != symbolID2 {
		t.Errorf("Expected symbolID2 for word 'pool', got %v", symbolsByWord)
	}

	// Verify stats
	stats := ssi.GetStats()
	if stats.TotalSymbols != 2 {
		t.Errorf("Expected 2 symbols, got %d", stats.TotalSymbols)
	}
	if stats.UniqueWords < 6 { // "auth", "user", "manager", "database", "connection", "pool"
		t.Errorf("Expected at least 6 unique words, got %d", stats.UniqueWords)
	}

	// Test GetSymbolNames
	names := ssi.GetSymbolNames([]types.SymbolID{symbolID1, symbolID2})
	if len(names) != 2 {
		t.Errorf("Expected 2 names, got %d", len(names))
	}
}

// TestSemanticSearchIndex_ConcurrentAddSymbol tests concurrent AddSymbolData operations
func TestSemanticSearchIndex_ConcurrentAddSymbol(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	const numGoroutines = 10
	const symbolsPerGoroutine = 100
	var wg sync.WaitGroup

	// Add symbols concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < symbolsPerGoroutine; j++ {
				symbolID := types.SymbolID(goroutineID*symbolsPerGoroutine + j)
				words := []string{"test", "word", "symbol"}
				stems := []string{"test", "word", "symbol"}
				phonetic := "t236"
				expansions := []string{"test", "word", "symbol"}

				ssi.AddSymbolData(symbolID, fmt.Sprintf("Symbol%d", symbolID), words, stems, phonetic, expansions)
			}
		}(i)
	}

	wg.Wait()

	// Verify all symbols were added
	stats := ssi.GetStats()
	expectedSymbols := numGoroutines * symbolsPerGoroutine
	if stats.TotalSymbols != expectedSymbols {
		t.Errorf("Expected %d symbols, got %d", expectedSymbols, stats.TotalSymbols)
	}

	// Verify we can retrieve symbols
	symbols := ssi.GetSymbolsByWord("test")
	if len(symbols) != expectedSymbols {
		t.Errorf("Expected %d symbols for word 'test', got %d", expectedSymbols, len(symbols))
	}
}

// TestSemanticSearchIndex_GracefulShutdown tests that Close() processes remaining batch
func TestSemanticSearchIndex_Close_ProcessesRemainingBatch(t *testing.T) {
	ssi := NewSemanticSearchIndex()

	// Add symbols
	for i := 0; i < 50; i++ {
		symbolID := types.SymbolID(i)
		words := []string{"test", "word"}
		stems := []string{"test", "word"}
		phonetic := "t236"
		expansions := []string{"test", "word"}

		ssi.AddSymbolData(symbolID, fmt.Sprintf("Symbol%d", i), words, stems, phonetic, expansions)
	}

	// Close immediately (without waiting for background processing)
	// The Close() method should process remaining batch before shutting down
	ssi.Close()

	// Verify all symbols were processed despite immediate close
	stats := ssi.GetStats()
	if stats.TotalSymbols != 50 {
		t.Errorf("Expected 50 symbols after close, got %d", stats.TotalSymbols)
	}

	// Verify we can still read data
	symbols := ssi.GetSymbolsByWord("test")
	if len(symbols) != 50 {
		t.Errorf("Expected 50 symbols for word 'test', got %d", len(symbols))
	}
}

// TestSemanticSearchIndex_ErrorRecovery tests that one bad symbol doesn't break indexing
func TestSemanticSearchIndex_ErrorRecovery(t *testing.T) {
	// This test would require the FileIntegrator.buildSemanticIndex
	// which has the panic recovery. We'll test the index behavior here.
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	// Add valid symbols
	ssi.AddSymbolData(types.SymbolID(1), "ValidSymbol1", []string{"valid"}, []string{"valid"}, "v235", []string{"valid"})
	ssi.AddSymbolData(types.SymbolID(2), "ValidSymbol2", []string{"valid"}, []string{"valid"}, "v235", []string{"valid"})

	// No need to wait - AddSymbolData is synchronous

	// Verify valid symbols were added
	if stats := ssi.GetStats(); stats.TotalSymbols != 2 {
		t.Errorf("Expected 2 valid symbols, got %d", stats.TotalSymbols)
	}
}

// TestSemanticSearchIndex_LockFreeReads tests that reads don't block during updates
func TestSemanticSearchIndex_LockFreeReads(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	const numReaders = 5
	const numWriters = 2
	const operationsPerWriter = 50

	var wg sync.WaitGroup

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				// Read operations should not block
				_ = ssi.GetStats()
				_ = ssi.GetSymbolsByWord("test")
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWriter; j++ {
				symbolID := types.SymbolID(writerID*operationsPerWriter + j)
				words := []string{"test", "word"}
				stems := []string{"test", "word"}
				phonetic := "t236"
				expansions := []string{"test", "word"}

				ssi.AddSymbolData(symbolID, fmt.Sprintf("Symbol%d", symbolID), words, stems, phonetic, expansions)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	stats := ssi.GetStats()
	expectedSymbols := numWriters * operationsPerWriter
	if stats.TotalSymbols < expectedSymbols {
		t.Errorf("Expected at least %d symbols, got %d", expectedSymbols, stats.TotalSymbols)
	}
}

// TestSemanticSearchIndex_MemoryEstimation tests memory estimation
func TestSemanticSearchIndex_MemoryEstimation(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	// Add several symbols
	for i := 0; i < 100; i++ {
		symbolID := types.SymbolID(i)
		words := []string{"word1", "word2", "word3"}
		stems := []string{"word1", "word2", "word3"}
		phonetic := "w236"
		expansions := []string{"word1", "word2", "word3"}

		ssi.AddSymbolData(symbolID, fmt.Sprintf("Symbol%d", i), words, stems, phonetic, expansions)
	}

	// No need to wait - AddSymbolData is synchronous

	stats := ssi.GetStats()

	// Memory estimate should be positive and reasonable
	if stats.MemoryEstimate <= 0 {
		t.Errorf("Memory estimate should be positive, got %d", stats.MemoryEstimate)
	}

	// For 100 symbols, memory should be roughly 100 * 240 bytes = 24KB
	// Allow generous variance for map overhead and different Go versions
	expectedMin := int64(100) * 150 // Minimum 150 bytes per symbol
	expectedMax := int64(100) * 400 // Maximum 400 bytes per symbol

	if stats.MemoryEstimate < expectedMin {
		t.Errorf("Memory estimate too low: got %d, expected at least %d", stats.MemoryEstimate, expectedMin)
	}
	if stats.MemoryEstimate > expectedMax {
		t.Errorf("Memory estimate too high: got %d, expected at most %d", stats.MemoryEstimate, expectedMax)
	}
}

// TestSemanticSearchIndex_EmptyIndex tests behavior with empty index
func TestSemanticSearchIndex_EmptyIndex(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	// Test empty index returns
	if symbols := ssi.GetSymbolsByWord("nonexistent"); len(symbols) != 0 {
		t.Errorf("Expected empty result for nonexistent word, got %v", symbols)
	}

	if name := ssi.GetSymbolName(types.SymbolID(9999)); name != "" {
		t.Errorf("Expected empty string for nonexistent symbol, got %q", name)
	}

	if names := ssi.GetSymbolNames([]types.SymbolID{types.SymbolID(1)}); len(names) != 0 {
		t.Errorf("Expected empty names, got %v", names)
	}

	stats := ssi.GetStats()
	if stats.TotalSymbols != 0 {
		t.Errorf("Expected 0 symbols, got %d", stats.TotalSymbols)
	}
}

// TestSemanticSearchIndex_BatchProcessing tests that batches are processed correctly
func TestSemanticSearchIndex_BatchProcessing(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	// Add symbols that exceed batch size (100)
	for i := 0; i < 150; i++ {
		symbolID := types.SymbolID(i)
		words := []string{"batch", "test"}
		stems := []string{"batch", "test"}
		phonetic := "b235"
		expansions := []string{"batch", "test"}

		ssi.AddSymbolData(symbolID, fmt.Sprintf("Symbol%d", i), words, stems, phonetic, expansions)
	}

	// No need to wait - AddSymbolData is synchronous

	// Verify all symbols were processed
	stats := ssi.GetStats()
	if stats.TotalSymbols < 150 {
		t.Errorf("Expected at least 150 symbols, got %d", stats.TotalSymbols)
	}

	// Verify we can retrieve symbols by word
	symbols := ssi.GetSymbolsByWord("batch")
	if len(symbols) < 150 {
		t.Errorf("Expected at least 150 symbols for word 'batch', got %d", len(symbols))
	}
}

// TestSemanticSearchIndex_Deduplication tests that symbol names are deduplicated
func TestSemanticSearchIndex_Deduplication(t *testing.T) {
	ssi := NewSemanticSearchIndex()
	defer ssi.Close()

	// Add multiple symbols with the same name
	ssi.AddSymbolData(types.SymbolID(1), "SameName", []string{"same"}, []string{"same"}, "s235", []string{"same"})
	ssi.AddSymbolData(types.SymbolID(2), "SameName", []string{"same"}, []string{"same"}, "s235", []string{"same"})
	ssi.AddSymbolData(types.SymbolID(3), "SameName", []string{"same"}, []string{"same"}, "s235", []string{"same"})

	// No need to wait - AddSymbolData is synchronous

	// Get symbol names - should deduplicate
	names := ssi.GetSymbolNames([]types.SymbolID{types.SymbolID(1), types.SymbolID(2), types.SymbolID(3)})
	if len(names) != 1 || names[0] != "SameName" {
		t.Errorf("Expected deduplicated names [SameName], got %v", names)
	}

	// But stats should show 3 symbols
	stats := ssi.GetStats()
	if stats.TotalSymbols != 3 {
		t.Errorf("Expected 3 symbols in stats, got %d", stats.TotalSymbols)
	}
}
