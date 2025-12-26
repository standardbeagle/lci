package semantic

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// BenchmarkSemanticScorer benchmarks the semantic scorer with various input sizes
func BenchmarkSemanticScorer(b *testing.B) {
	// Initialize components once
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()
	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	// Test cases with different candidate set sizes
	benchCases := []struct {
		name          string
		numCandidates int
		queryType     string
	}{
		{"Small_10_Exact", 10, "exact"},
		{"Small_10_Fuzzy", 10, "fuzzy"},
		{"Medium_100_Exact", 100, "exact"},
		{"Medium_100_Fuzzy", 100, "fuzzy"},
		{"Large_1000_Exact", 1000, "exact"},
		{"Large_1000_Fuzzy", 1000, "fuzzy"},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			// Generate candidates
			candidates := generateCandidates(bc.numCandidates)

			// Generate query based on type
			var query string
			if bc.queryType == "exact" {
				// Use an exact match from candidates
				query = candidates[rand.Intn(len(candidates))]
			} else {
				// Use a fuzzy match (typo)
				base := candidates[rand.Intn(len(candidates))]
				query = introduceTypo(base)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = scorer.ScoreMultiple(query, candidates)
			}

			b.ReportMetric(float64(bc.numCandidates), "candidates")
		})
	}
}

// BenchmarkSemanticScorerConcurrent benchmarks concurrent access
func BenchmarkSemanticScorerConcurrent(b *testing.B) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()
	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	candidates := generateCandidates(100)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			query := candidates[rand.Intn(len(candidates))]
			_ = scorer.ScoreMultiple(query, candidates)
		}
	})
}

// BenchmarkLRUCache benchmarks the LRU cache implementation
func BenchmarkLRUCache(b *testing.B) {
	cacheSize := []int{100, 500, 1000, 5000}

	for _, size := range cacheSize {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			cache := NewLRUCache(size)

			// Pre-populate cache to 80% capacity
			for i := 0; i < int(float64(size)*0.8); i++ {
				key := fmt.Sprintf("query_%d", i)
				value := &normalizedQuery{
					original: key,
					words:    []string{"test", "word"},
					stems:    []string{"test", "word"},
				}
				cache.Set(key, value)
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Mix of reads and writes (80% reads, 20% writes)
			for i := 0; i < b.N; i++ {
				if rand.Float32() < 0.8 {
					// Read operation
					key := fmt.Sprintf("query_%d", rand.Intn(size))
					_, _ = cache.Get(key)
				} else {
					// Write operation
					key := fmt.Sprintf("new_query_%d", i)
					value := &normalizedQuery{
						original: key,
						words:    []string{"new", "test"},
						stems:    []string{"new", "test"},
					}
					cache.Set(key, value)
				}
			}

			b.ReportMetric(float64(cache.Size()), "items_in_cache")
		})
	}
}

// BenchmarkSemanticLayers benchmarks individual matching layers
func BenchmarkSemanticLayers(b *testing.B) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()
	config := DefaultScoreLayers

	query := "getUserName"
	symbol := "getUserNameById"

	layers := []struct {
		name    string
		matcher interface {
			Detect(query, symbol, queryLower, symbolLower string, config ScoreLayers) (bool, float64, string, map[string]string)
		}
	}{
		{"Exact", &ExactMatcher{}},
		{"Fuzzy", NewFuzzyMatcherDetector(fuzzer)},
		{"Stemming", NewStemmingMatcher(splitter, stemmer)},
		{"Abbreviation", NewAbbreviationMatcher(dict, NewNameSplitter())},
		{"NameSplit", NewNameSplitMatcher(splitter)},
	}

	for _, layer := range layers {
		b.Run(layer.name, func(b *testing.B) {
			b.ReportAllocs()
			queryLower := strings.ToLower(query)
			symbolLower := strings.ToLower(symbol)
			for i := 0; i < b.N; i++ {
				_, _, _, _ = layer.matcher.Detect(query, symbol, queryLower, symbolLower, config)
			}
		})
	}
}

// BenchmarkMemoryScaling benchmarks memory usage with increasing candidates
func BenchmarkMemoryScaling(b *testing.B) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()
	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Candidates_%d", size), func(b *testing.B) {
			candidates := generateCandidates(size)
			query := candidates[0]

			// Measure memory allocation
			b.ReportAllocs()

			var totalAllocs uint64
			for i := 0; i < b.N; i++ {
				result := scorer.ScoreMultiple(query, candidates)
				// Force allocation by accessing result
				if len(result) > 0 {
					_ = result[0].Symbol
				}
			}

			// Report bytes per candidate
			if b.N > 0 {
				avgAllocs := totalAllocs / uint64(b.N)
				b.ReportMetric(float64(avgAllocs)/float64(size), "bytes/candidate")
			}
		})
	}
}

// Helper functions

func generateCandidates(n int) []string {
	prefixes := []string{"get", "set", "update", "delete", "create", "find", "search", "validate"}
	entities := []string{"User", "Account", "Product", "Order", "Payment", "Session", "Token", "Config"}
	suffixes := []string{"", "ById", "ByName", "ByEmail", "List", "Count", "Info", "Details"}

	candidates := make([]string, 0, n)
	for i := 0; i < n; i++ {
		prefix := prefixes[rand.Intn(len(prefixes))]
		entity := entities[rand.Intn(len(entities))]
		suffix := suffixes[rand.Intn(len(suffixes))]

		// Mix of camelCase and snake_case
		if rand.Float32() < 0.5 {
			candidates = append(candidates, prefix+entity+suffix)
		} else {
			candidate := strings.ToLower(prefix + "_" + entity)
			if suffix != "" {
				candidate += "_" + strings.ToLower(suffix[2:]) // Remove "By" prefix
			}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func introduceTypo(s string) string {
	if len(s) < 3 {
		return s
	}

	// Random typo type
	typoType := rand.Intn(3)
	runes := []rune(s)

	switch typoType {
	case 0: // Swap adjacent characters
		pos := rand.Intn(len(runes) - 1)
		runes[pos], runes[pos+1] = runes[pos+1], runes[pos]
	case 1: // Delete a character
		pos := rand.Intn(len(runes))
		runes = append(runes[:pos], runes[pos+1:]...)
	case 2: // Replace a character
		pos := rand.Intn(len(runes))
		runes[pos] = rune('a' + rand.Intn(26))
	}

	return string(runes)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
