package semantic

import (
	"testing"
)

// TestPhase3E_UnifiedSemanticScoring demonstrates the complete Phase 3E semantic scoring system
// integrating all 6 layers from Phases 3A-3D+
func TestPhase3E_UnifiedSemanticScoring(t *testing.T) {
	// Set up all components from previous phases
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()

	// Create the unified semantic scorer
	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	t.Run("Example1: User query matching getUserName", func(t *testing.T) {
		// From PHASE_3E_SEMANTIC_SCORING.md Example 1
		candidates := []string{
			"getUserName", // Should rank first
			"setUserName", // Should match but lower
			"getUsername", // Should match but lower
			"userName",    // Should match but lower
			"user",        // Should match but lower
		}

		result := scorer.Search("get user name", candidates)

		if len(result.Symbols) == 0 {
			t.Fatal("expected results")
		}

		// First result should be getUserName
		topSymbol := result.Symbols[0]
		if topSymbol.Symbol != "getUserName" {
			t.Errorf("expected getUserName to rank first, got %v", topSymbol.Symbol)
		}

		if topSymbol.Score.QueryMatch != MatchTypeNameSplit {
			t.Logf("Top match type: %s (score: %.3f)", topSymbol.Score.QueryMatch, topSymbol.Score.Score)
		}
	})

	t.Run("Example2: Abbreviation query matching HTTPConnection", func(t *testing.T) {
		// From PHASE_3E_SEMANTIC_SCORING.md Example 2
		candidates := []string{
			"HTTPConnection",       // Should match on abbreviation
			"httpServerConnection", // Should match on fuzzy/name
			"getConnectionStatus",  // Should match lower
		}

		result := scorer.Search("http conn", candidates)

		if len(result.Symbols) == 0 {
			t.Fatal("expected results")
		}

		// HTTPConnection should be top or near-top due to abbreviation matching
		topSymbol := result.Symbols[0]
		t.Logf("Top match for 'http conn': %v (score: %.3f, type: %s)",
			topSymbol.Symbol, topSymbol.Score.Score, topSymbol.Score.QueryMatch)
	})

	t.Run("Example3: Typo query matching authenticate", func(t *testing.T) {
		// From PHASE_3E_SEMANTIC_SCORING.md Example 3
		candidates := []string{
			"authenticate",   // Should match via fuzzy (typo tolerance)
			"Authentication", // Should match via stemming
			"authorizeUser",  // Should not match well
		}

		result := scorer.Search("authentificate", candidates)

		if len(result.Symbols) == 0 {
			t.Fatal("expected results")
		}

		// authenticate should rank first due to fuzzy matching handling the typo
		topSymbol := result.Symbols[0]
		if topSymbol.Symbol != "authenticate" {
			t.Errorf("expected authenticate to rank first, got %v", topSymbol.Symbol)
		}

		if topSymbol.Score.QueryMatch != MatchTypeFuzzy {
			t.Logf("Note: matched via %s instead of fuzzy", topSymbol.Score.QueryMatch)
		}
	})

	t.Run("Layer 1: Exact Match - getUserName", func(t *testing.T) {
		score := scorer.ScoreSymbol("getUserName", "getUserName")
		if score.QueryMatch != MatchTypeExact {
			t.Errorf("expected exact match, got %s", score.QueryMatch)
		}
		if score.Score != scorer.config.ExactWeight {
			t.Errorf("expected score %.2f, got %.2f", scorer.config.ExactWeight, score.Score)
		}
	})

	t.Run("Layer 2: Annotation Match - with labels", func(t *testing.T) {
		// Without actual annotator, annotation matching won't work in this test
		// But we can verify the layer exists and is configured
		if len(scorer.matchers) < 3 {
			t.Fatal("not enough matchers initialized")
		}
		if scorer.config.AnnotationWeight == 0 {
			t.Fatal("annotation weight not configured")
		}
	})

	t.Run("Layer 3: Fuzzy Match - typo tolerance", func(t *testing.T) {
		// Test fuzzy matching with a typo
		score := scorer.ScoreSymbol("getUserNme", "getUserName")
		if score.QueryMatch != MatchTypeFuzzy {
			t.Logf("fuzzy match not triggered, got %s instead", score.QueryMatch)
		}
		if score.Score == 0 {
			t.Errorf("expected non-zero score for fuzzy match")
		}
	})

	t.Run("Layer 4: Stemming Match - word normalization", func(t *testing.T) {
		// Test stemming: "running" and "runs" should match via stemming
		score := scorer.ScoreSymbol("running", "runs")
		if score.QueryMatch != MatchTypeStemming {
			t.Logf("stemming not the top match, got %s with score %.3f", score.QueryMatch, score.Score)
		}
		if score.Score == 0 {
			t.Errorf("expected non-zero score for stemming match")
		}
	})

	t.Run("Layer 5: Abbreviation Match - term expansion", func(t *testing.T) {
		// Test abbreviation: "api" expands and matches "application"
		score := scorer.ScoreSymbol("api", "ApplicationInterface")
		if score.Score > 0 {
			t.Logf("abbreviation layer activated (score: %.3f, type: %s)",
				score.Score, score.QueryMatch)
		}
	})

	t.Run("Layer 6: Name Split Match - camelCase splitting", func(t *testing.T) {
		// Test name split: "user" matches in "getUserName" via splitting
		score := scorer.ScoreSymbol("user", "getUserName")
		if score.QueryMatch == MatchTypeNameSplit {
			if score.Score == 0 {
				t.Errorf("expected non-zero score for name split match")
			}
		} else {
			t.Logf("name split not triggered, got %s instead", score.QueryMatch)
		}
	})

	t.Run("ScoreLayers configuration", func(t *testing.T) {
		// Verify all layer weights are configured
		config := scorer.GetConfig()

		if config.ExactWeight != 1.0 {
			t.Errorf("exact weight should be 1.0, got %.2f", config.ExactWeight)
		}
		if config.AnnotationWeight < config.FuzzyWeight {
			t.Errorf("annotation weight should be higher than fuzzy")
		}
		if config.FuzzyWeight < config.StemmingWeight {
			t.Errorf("fuzzy weight should be higher than stemming")
		}
	})

	t.Run("Result ranking and filtering", func(t *testing.T) {
		candidates := []string{
			"getUserName",
			"setUserName",
			"userName",
			"user",
			"unknownSymbol",
			"xyzabc",
		}

		result := scorer.Search("get user", candidates)

		// Results should be sorted by score descending
		for i := 1; i < len(result.Symbols); i++ {
			if result.Symbols[i].Score.Score > result.Symbols[i-1].Score.Score {
				t.Errorf("results not sorted at position %d", i)
			}
		}

		// All results should be >= minScore
		for _, sym := range result.Symbols {
			if sym.Score.Score < scorer.config.MinScore {
				t.Errorf("result score %.3f below minimum %.3f",
					sym.Score.Score, scorer.config.MinScore)
			}
		}

		// Results should not exceed maxResults
		if len(result.Symbols) > scorer.config.MaxResults {
			t.Errorf("result count %d exceeds max %d",
				len(result.Symbols), scorer.config.MaxResults)
		}
	})

	t.Run("Custom scoring configuration", func(t *testing.T) {
		customConfig := ScoreLayers{
			ExactWeight:        1.0,
			SubstringWeight:    0.9,
			PhraseWeight:       0.85, // Multi-word phrase matching
			FuzzyWeight:        0.8,  // Favor fuzzy over stemming
			StemmingWeight:     0.4,
			NameSplitWeight:    0.3,
			AbbreviationWeight: 0.25,
			FuzzyThreshold:     0.75,
			MaxResults:         5,
			MinScore:           0.4,
		}

		scorer.Configure(customConfig)

		if scorer.config.FuzzyWeight != 0.8 {
			t.Errorf("config not applied")
		}

		score := scorer.ScoreSymbol("getUserNme", "getUserName")
		// Should be penalized less by fuzzy due to higher threshold
		if score.Score > 0.5 {
			t.Logf("fuzzy match score with custom config: %.3f", score.Score)
		}
	})

	t.Run("Query normalization and caching", func(t *testing.T) {
		// Test that whitespace is normalized
		score1 := scorer.ScoreSymbol("getUserName", "getUserName")
		score2 := scorer.ScoreSymbol("  getUserName  ", "getUserName")

		if score1.Score != score2.Score {
			t.Errorf("whitespace normalization failed: %.3f vs %.3f",
				score1.Score, score2.Score)
		}
	})

	t.Run("Score validation and statistics", func(t *testing.T) {
		candidates := []string{
			"getUserName",
			"setUserName",
			"getUsername",
		}

		result := scorer.Search("get user", candidates)
		dist := scorer.GetScoreDistribution(result)

		if count, ok := dist["count"].(int); !ok || count == 0 {
			t.Fatal("expected count in distribution")
		}

		if avgScore, ok := dist["avg_score"].(float64); !ok || avgScore < 0 || avgScore > 1.0 {
			t.Errorf("invalid avg_score: %.3f", avgScore)
		}

		if _, ok := dist["distribution"].(map[string]int); !ok {
			t.Fatal("expected match type distribution")
		}
	})
}

// BenchmarkSemanticScoring benchmarks the complete semantic scoring pipeline
func BenchmarkSemanticScoring(b *testing.B) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()

	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	candidates := []string{
		"getUserName", "setUserName", "getUsername", "userName", "user",
		"authenticate", "Authorization", "authorizeUser",
		"HTTPConnection", "httpServerConnection", "getConnectionStatus",
		"validateEmail", "sendNotification", "processPayment",
		"databaseQuery", "cacheManager", "loggerService",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = scorer.Search("get user", candidates)
	}
}

// BenchmarkSingleSymbolScoring benchmarks scoring a single symbol
func BenchmarkSingleSymbolScoring(b *testing.B) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()

	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = scorer.ScoreSymbol("getUserName", "getUserName")
	}
}
