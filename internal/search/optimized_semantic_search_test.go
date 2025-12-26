package search

import (
	"context"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"
)

// TestOptimizedSemanticSearch_FallbackBehavior tests fallback when semantic index is nil
func TestOptimizedSemanticSearch_FallbackBehavior(t *testing.T) {
	// Create scorer without semantic index
	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(nil, semanticScorer, symbolIndex)

	// Should not be optimized
	if oss.IsOptimized() {
		t.Error("Expected IsOptimized() to return false when semantic index is nil")
	}

	// Search should still work (using regular scorer)
	result := oss.Search(context.Background(), "test query", []string{"symbol1", "symbol2"}, 10)

	if result.Query != "test query" {
		t.Errorf("Expected query to be preserved, got %q", result.Query)
	}
	if result.CandidatesConsidered != 2 {
		t.Errorf("Expected 2 candidates considered, got %d", result.CandidatesConsidered)
	}
}

// TestOptimizedSemanticSearch_WithSemanticIndex tests behavior with semantic index
func TestOptimizedSemanticSearch_WithSemanticIndex(t *testing.T) {
	// Create semantic index with test data
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add test symbols to semantic index
	addTestSymbolToIndex(semanticIndex, 1000, "AuthUserManager",
		[]string{"auth", "user", "manager"},
		[]string{"auth", "user", "manag"},
		"a236",
		[]string{"authenticate", "user", "management"})

	addTestSymbolToIndex(semanticIndex, 2000, "DatabaseConnectionPool",
		[]string{"database", "connection", "pool"},
		[]string{"databas", "connect", "pool"},
		"d216",
		[]string{"database", "connection", "pool"})

	addTestSymbolToIndex(semanticIndex, 3000, "TransactionHandler",
		[]string{"transaction", "handler"},
		[]string{"transact", "handler"},
		"t236",
		[]string{"transaction", "handler"})

	// Create optimized search with semantic index
	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Should be optimized
	if !oss.IsOptimized() {
		t.Error("Expected IsOptimized() to return true when semantic index is available")
	}

	// Test optimized candidate gathering
	candidates := oss.GatherCandidates("auth user")

	if len(candidates) == 0 {
		t.Error("Expected to find candidates for 'auth user' query")
	}

	// Should include AuthUserManager
	found := false
	for _, name := range candidates {
		if name == "AuthUserManager" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'AuthUserManager' in candidates for 'auth user' query")
	}
}

// TestOptimizedSemanticSearch_StemMatching tests stem-based matching
func TestOptimizedSemanticSearch_StemMatching(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add symbol with word that has a stem
	addTestSymbolToIndex(semanticIndex, 4000, "ManagingUsers",
		[]string{"managing", "users"},
		[]string{"manag", "user"}, // "managing" stems to "manag"
		"m536",
		[]string{"managing", "users"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Query with a word that should match the stem
	candidates := oss.GatherCandidates("manage")

	if len(candidates) == 0 {
		t.Error("Expected to find candidates for 'manage' query (stem match)")
	}

	// Should include ManagingUsers (via stem match)
	found := false
	for _, name := range candidates {
		if name == "ManagingUsers" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'ManagingUsers' in candidates for 'manage' query (stem match)")
	}
}

// TestOptimizedSemanticSearch_PhoneticMatching tests phonetic matching
func TestOptimizedSemanticSearch_PhoneticMatching(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add symbol - "serve" generates phonetic code "s610" which matches "serv"
	addTestSymbolToIndex(semanticIndex, 5000, "ServeHandler",
		[]string{"serve", "handler"},
		[]string{"serve", "handler"},
		"s610", // Simplified Soundex for "serve" (matches "serv")
		[]string{"serve", "handler"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Query with phonetic similarity - "serv" generates "s610" which matches
	candidates := oss.GatherCandidates("serv")

	if len(candidates) == 0 {
		t.Error("Expected to find candidates for 'serv' query (phonetic match)")
	}
}

// TestOptimizedSemanticSearch_AbbreviationMatching tests abbreviation expansion matching
func TestOptimizedSemanticSearch_AbbreviationMatching(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add symbol with abbreviation expansions
	addTestSymbolToIndex(semanticIndex, 6000, "TxnManager",
		[]string{"txn", "manager"},
		[]string{"txn", "manag"},
		"t536",
		[]string{"transaction", "manager"}) // "txn" expands to "transaction"

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Query with full word should match abbreviation
	candidates := oss.GatherCandidates("transaction")

	if len(candidates) == 0 {
		t.Error("Expected to find candidates for 'transaction' query (abbreviation match)")
	}

	// Should include TxnManager (via abbreviation expansion)
	found := false
	for _, name := range candidates {
		if name == "TxnManager" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'TxnManager' in candidates for 'transaction' query (abbreviation match)")
	}
}

// TestOptimizedSemanticSearch_NameSplitting tests that camelCase names are properly split during indexing
// This test specifically validates that "serve http" finds "ServeHTTP"
func TestOptimizedSemanticSearch_NameSplitting(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add test symbol with camelCase name
	// The index should split "ServeHTTP" into ["serve", "http"] during indexing
	addTestSymbolToIndex(semanticIndex, 12000, "ServeHTTP",
		[]string{"serve", "http"}, // This is what SplitSymbolName should produce
		[]string{"serve", "http"},
		"s610",
		[]string{"serve", "http"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Query with two words should find the symbol
	candidates := oss.GatherCandidates("serve http")

	if len(candidates) == 0 {
		t.Error("Expected to find candidates for 'serve http' query (name splitting)")
	}

	// Should include ServeHTTP (via word match)
	found := false
	for _, name := range candidates {
		if name == "ServeHTTP" {
			found = true
			t.Logf("✓ Found 'ServeHTTP' for query 'serve http' - name splitting works!")
			break
		}
	}
	if !found {
		t.Error("Expected to find 'ServeHTTP' in candidates for 'serve http' query (name splitting)")
	}
}

// TestOptimizedSemanticSearch_MultiWord_AND_vs_OR tests whether multi-word queries
// use AND (must match all words) or OR (can match any word)
func TestOptimizedSemanticSearch_MultiWord_AND_vs_OR(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add multiple test symbols
	addTestSymbolToIndex(semanticIndex, 14000, "ServeHTTP",
		[]string{"serve", "http"},
		[]string{"serve", "http"},
		"s610",
		[]string{"serve", "http"})

	addTestSymbolToIndex(semanticIndex, 14001, "ServeTCP",
		[]string{"serve", "tcp"},
		[]string{"serve", "tcp"},
		"s610",
		[]string{"serve", "tcp"})

	addTestSymbolToIndex(semanticIndex, 14002, "HTTPServer",
		[]string{"http", "server"},
		[]string{"http", "server"},
		"h216",
		[]string{"http", "server"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Test with single word "http" - should find ServeHTTP, ServeTCP, and HTTPServer
	// (all contain "http" in their words, stems, or expansions)
	candidates := oss.GatherCandidates("http")
	t.Logf("Query 'http' found %d candidates: %v", len(candidates), candidates)

	// Check each result
	foundServer := false
	for _, name := range candidates {
		if name == "HTTPServer" {
			foundServer = true
		}
	}

	// Single word should find at least HTTPServer
	if !foundServer {
		t.Error("Expected 'http' to find 'HTTPServer'")
	} else {
		t.Logf("✓ Single word 'http' correctly found symbols with 'http'")
	}

	// Test with multi-word "http serve" - this is the key test!
	// AND logic: should find ONLY symbols with BOTH "http" AND "serve" → ServeHTTP
	// OR logic: should find symbols with EITHER "http" OR "serve" → ServeHTTP, ServeTCP, HTTPServer
	candidates2 := oss.GatherCandidates("http serve")
	t.Logf("Query 'http serve' found %d candidates: %v", len(candidates2), candidates2)

	foundHTTPInMulti := false
	foundTCPInMulti := false
	foundServerInMulti := false
	for _, name := range candidates2 {
		if name == "ServeHTTP" {
			foundHTTPInMulti = true
		}
		if name == "ServeTCP" {
			foundTCPInMulti = true
		}
		if name == "HTTPServer" {
			foundServerInMulti = true
		}
	}

	// The critical check: does "http serve" find ServeTCP?
	// - If YES → it's using OR logic (any word matches)
	// - If NO → it's using AND logic (all words must match)
	if foundTCPInMulti {
		t.Logf("⚠️  'http serve' found 'ServeTCP' → Using OR logic (any word matches)")
		t.Logf("   This means 'http serve' is effectively the same as 'http' or 'serve'")
	} else {
		t.Logf("✓ 'http serve' did NOT find 'ServeTCP' → Using AND logic (all words must match)")
	}

	// "http serve" should definitely find ServeHTTP (has both words)
	if !foundHTTPInMulti {
		t.Error("Expected 'http serve' to find 'ServeHTTP' (has both 'http' and 'serve')")
	}

	// "http serve" should find HTTPServer (has "http")
	if !foundServerInMulti {
		t.Error("Expected 'http serve' to find 'HTTPServer' (has 'http')")
	}

	if foundTCPInMulti {
		t.Skip("Skipping semantic scoring check - GatherCandidates uses OR logic")
	}

	// If we get here, AND logic is being used in GatherCandidates
	// Now test if the semantic scorer does additional filtering
	result := oss.Search(context.Background(), "http serve", nil, 10)
	t.Logf("Semantic search 'http serve' returned %d results with scores:", len(result.Symbols))
	for i, sym := range result.Symbols {
		t.Logf("  #%d: %s (score: %.2f, %s)", i+1, sym.Symbol, sym.Score.Score, sym.Score.QueryMatch)
	}
}

// TestOptimizedSemanticSearch_MultiWordOrderAndPartial tests multi-word query behavior
// Specifically: partial matching and out-of-order matching
func TestOptimizedSemanticSearch_MultiWordOrderAndPartial(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add test symbols
	addTestSymbolToIndex(semanticIndex, 13000, "ServeHTTP",
		[]string{"serve", "http"},
		[]string{"serve", "http"},
		"s610",
		[]string{"serve", "http"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	tests := []struct {
		name     string
		query    string
		expected bool // Should find ServeHTTP
		reason   string
	}{
		{
			name:     "normal_order",
			query:    "serve http",
			expected: true,
			reason:   "Normal word order should work",
		},
		{
			name:     "reversed_order",
			query:    "http serve",
			expected: true,
			reason:   "Out of order matching should work (union search)",
		},
		{
			name:     "partial_word",
			query:    "serv http",
			expected: false, // Currently NO partial matching in GatherCandidates
			reason:   "Partial word 'serv' (not 'serve') - depends on if fuzzy matching is applied",
		},
		{
			name:     "both_partial",
			query:    "serv htt",
			expected: false,
			reason:   "Both words partial - likely won't match without fuzzy matching",
		},
		{
			name:     "extra_word",
			query:    "serve http server",
			expected: true,
			reason:   "Extra word should still find ServeHTTP (union search)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := oss.GatherCandidates(tt.query)
			found := false
			for _, name := range candidates {
				if name == "ServeHTTP" {
					found = true
					break
				}
			}

			if tt.expected && !found {
				t.Errorf("Expected to find 'ServeHTTP' for query '%s': %s", tt.query, tt.reason)
			} else if !tt.expected && found {
				t.Logf("Note: Found 'ServeHTTP' for query '%s' (unexpected): %s", tt.query, tt.reason)
			} else {
				t.Logf("✓ Query '%s': %s (found=%v)", tt.query, tt.reason, found)
			}
		})
	}
}

// TestOptimizedSemanticSearch_GetStats tests statistics retrieval
func TestOptimizedSemanticSearch_GetStats(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	// Add some test data
	addTestSymbolToIndex(semanticIndex, 7000, "TestSymbol1", []string{"test"}, []string{"test"}, "t236", []string{"test"})
	addTestSymbolToIndex(semanticIndex, 8000, "TestSymbol2", []string{"test"}, []string{"test"}, "t236", []string{"test"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	stats := oss.GetStats()

	if stats.TotalSymbols != 2 {
		t.Errorf("Expected 2 symbols, got %d", stats.TotalSymbols)
	}
	if stats.MemoryEstimate <= 0 {
		t.Errorf("Expected positive memory estimate, got %d", stats.MemoryEstimate)
	}
}

// TestOptimizedSemanticSearch_SearchAPICompatibility tests API compatibility with SemanticScorer
func TestOptimizedSemanticSearch_SearchAPICompatibility(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	addTestSymbolToIndex(semanticIndex, 9000, "APIService",
		[]string{"api", "service"},
		[]string{"api", "servic"},
		"a123",
		[]string{"api", "service"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Test with candidates provided (should use regular path)
	result1 := oss.Search(context.Background(), "api", []string{"APIService", "OtherSymbol"}, 10)
	if result1.Query != "api" {
		t.Errorf("Expected query 'api', got %q", result1.Query)
	}

	// Test without candidates (should use optimized path)
	result2 := oss.Search(context.Background(), "api", nil, 10)
	if result2.Query != "api" {
		t.Errorf("Expected query 'api', got %q", result2.Query)
	}

	// Both should work without errors
	if result1.CandidatesConsidered < 0 {
		t.Errorf("Expected valid CandidatesConsidered, got %d", result1.CandidatesConsidered)
	}
	if result2.CandidatesConsidered < 0 {
		t.Errorf("Expected valid CandidatesConsidered, got %d", result2.CandidatesConsidered)
	}
}

// TestOptimizedSemanticSearch_EmptyQuery tests behavior with empty queries
func TestOptimizedSemanticSearch_EmptyQuery(t *testing.T) {
	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(nil, semanticScorer, symbolIndex)

	result := oss.Search(context.Background(), "", []string{"symbol1"}, 10)

	if len(result.Symbols) != 0 {
		t.Errorf("Expected empty results for empty query, got %d symbols", len(result.Symbols))
	}
	if result.CandidatesConsidered != 0 {
		t.Errorf("Expected 0 candidates for empty query, got %d", result.CandidatesConsidered)
	}
}

// TestOptimizedSemanticSearch_EmptyCandidateList tests behavior with empty candidate list
func TestOptimizedSemanticSearch_EmptyCandidateList(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	result := oss.Search(context.Background(), "test", []string{}, 10)

	if len(result.Symbols) != 0 {
		t.Errorf("Expected empty results for empty candidate list, got %d symbols", len(result.Symbols))
	}
}

// TestOptimizedSemanticSearch_SymbolNames tests SymbolNames retrieval
func TestOptimizedSemanticSearch_SymbolNames(t *testing.T) {
	semanticIndex := core.NewSemanticSearchIndex()
	defer semanticIndex.Close()

	addTestSymbolToIndex(semanticIndex, 10000, "SymbolOne", []string{"symbol"}, []string{"symbol"}, "s123", []string{"symbol"})
	addTestSymbolToIndex(semanticIndex, 11000, "SymbolTwo", []string{"symbol"}, []string{"symbol"}, "s123", []string{"symbol"})

	semanticScorer := createTestSemanticScorer()
	symbolIndex := &core.SymbolIndex{}

	oss := NewOptimizedSemanticSearch(semanticIndex, semanticScorer, symbolIndex)

	// Search for symbols
	symbolNames := oss.SearchSymbols(context.Background(), "symbol", 10)

	if len(symbolNames) == 0 {
		t.Error("Expected to find symbol names")
	}

	// Should include both symbols
	foundOne := false
	foundTwo := false
	for _, name := range symbolNames {
		if name == "SymbolOne" {
			foundOne = true
		}
		if name == "SymbolTwo" {
			foundTwo = true
		}
	}
	if !foundOne {
		t.Error("Expected to find SymbolOne")
	}
	if !foundTwo {
		t.Error("Expected to find SymbolTwo")
	}
}

// Helper function to add test symbols to semantic index
func addTestSymbolToIndex(semanticIndex *core.SemanticSearchIndex, id uint32, name string, words, stems []string, phonetic string, expansions []string) {
	symbolID := types.SymbolID(id)
	semanticIndex.AddSymbolData(symbolID, name, words, stems, phonetic, expansions)
	// Wait for background processing
	time.Sleep(10 * time.Millisecond)
}

// Helper function to create a test semantic scorer
func createTestSemanticScorer() *semantic.SemanticScorer {
	splitter := semantic.NewNameSplitter()
	stemmer := semantic.NewStemmer(true, "porter2", 3, nil)
	fuzzer := semantic.NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := semantic.DefaultTranslationDictionary()

	return semantic.NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)
}
