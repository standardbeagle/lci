package semantic

import (
	"testing"
)

// TestExactMatch tests exact case-insensitive matching
func TestExactMatch(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	tests := []struct {
		name        string
		query       string
		symbol      string
		expectMatch bool
		expectScore float64
	}{
		{"exact lowercase", "getuser", "getuser", true, 1.0},
		{"exact uppercase", "GETUSER", "GETUSER", true, 1.0},
		{"exact mixed case", "GetUser", "getuser", true, 1.0},
		{"whitespace normalization", " getuser ", "getuser", true, 1.0},
		{"no match", "getuser", "setuser", false, 0.0},
		{"partial match not exact", "get", "getuser", false, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.ScoreSymbol(tt.query, tt.symbol)
			if tt.expectMatch && score.Score != tt.expectScore {
				t.Errorf("expected score %.2f, got %.2f", tt.expectScore, score.Score)
			}
			if !tt.expectMatch && score.Score > 0 && score.QueryMatch != MatchTypeNameSplit {
				// Name split might still match, but exact shouldn't
				if score.QueryMatch == MatchTypeExact {
					t.Errorf("expected no exact match, got score %.2f", score.Score)
				}
			}
		})
	}
}

// TestFuzzyMatch tests typo tolerance matching
func TestFuzzyMatch(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	tests := []struct {
		name        string
		query       string
		symbol      string
		expectMatch bool
	}{
		{"typo: one char off", "getUserName", "getUserNane", true},
		{"typo: transposition", "getUser", "gteUser", true},
		{"similar words", "authenticate", "authenticat", true},
		{"very different", "xyz", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.ScoreSymbol(tt.query, tt.symbol)
			if tt.expectMatch && score.Score == 0 {
				t.Errorf("expected match, got score 0")
			}
			if !tt.expectMatch && score.Score >= scorer.config.FuzzyThreshold {
				t.Errorf("expected no fuzzy match, got score %.2f", score.Score)
			}
		})
	}
}

// TestStemming tests word normalization matching
func TestStemming(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)
	splitter := NewNameSplitter()
	scorer := NewSemanticScorer(
		splitter,
		stemmer,
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	tests := []struct {
		name        string
		query       string
		symbol      string
		expectMatch bool
	}{
		{"stemming match: auth", "authenticate", "authentication", true},
		{"stemming match: run", "running", "runs", true},
		{"stemming match: connect", "connection", "connected", true},
		{"no stemming match", "xyz", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.ScoreSymbol(tt.query, tt.symbol)
			if tt.expectMatch && score.Score == 0 {
				t.Errorf("expected stemming match, got score 0")
			}
			if !tt.expectMatch && score.Score >= scorer.config.MinScore {
				t.Errorf("expected no match, got score %.2f", score.Score)
			}
		})
	}
}

// TestNameSplitting tests camelCase and other naming convention splitting
func TestNameSplitting(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	tests := []struct {
		name        string
		query       string
		symbol      string
		expectMatch bool
	}{
		{"name split: camelCase", "user", "getUserName", true},
		{"name split: camelCase multi-word", "name", "getUserName", true},
		{"name split: snake_case", "user", "get_user_name", true},
		{"name split: multiple words", "get user", "getUserName", true},
		{"no name split match", "xyz", "getUserName", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.ScoreSymbol(tt.query, tt.symbol)
			if tt.expectMatch && score.Score == 0 {
				t.Errorf("expected name split match, got score 0 (type: %s)", score.QueryMatch)
			}
			if !tt.expectMatch && score.Score >= scorer.config.MinScore && score.QueryMatch == MatchTypeNameSplit {
				t.Errorf("expected no match, got score %.2f", score.Score)
			}
		})
	}
}

// TestScoreMultiple tests ranking multiple symbols
func TestScoreMultiple(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	symbols := []string{
		"getUserName",
		"setUserName",
		"getUsername",
		"userName",
		"user",
	}

	results := scorer.ScoreMultiple("get user name", symbols)

	if len(results) == 0 {
		t.Fatal("expected results, got empty list")
	}

	// Check that results are sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score.Score > results[i-1].Score.Score {
			t.Errorf("results not sorted: %.2f > %.2f", results[i].Score.Score, results[i-1].Score.Score)
		}
	}

	// Check that ranks are assigned correctly
	for i, result := range results {
		expectedRank := i + 1
		if result.Rank != expectedRank {
			t.Errorf("rank mismatch: expected %d, got %d", expectedRank, result.Rank)
		}
	}
}

// TestSearchResult tests full search operation
func TestSearchResult(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	candidates := []string{
		"getUserName",
		"setUserName",
		"getUsername",
		"userName",
		"user",
		"xyz",
		"abc",
	}

	result := scorer.Search("get user", candidates)

	if result.Query != "get user" {
		t.Errorf("query mismatch: expected 'get user', got '%s'", result.Query)
	}

	if result.CandidatesConsidered != len(candidates) {
		t.Errorf("candidates mismatch: expected %d, got %d", len(candidates), result.CandidatesConsidered)
	}

	if result.ResultsReturned == 0 {
		t.Fatal("expected results, got empty list")
	}

	if result.ExecutionTime <= 0 {
		t.Errorf("execution time should be positive, got %d", result.ExecutionTime)
	}
}

// TestMinScoreFiltering tests that results below min score are filtered
func TestMinScoreFiltering(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	scorer.config.MinScore = 0.5

	candidates := []string{
		"getUserName", // Should match well
		"setUserName", // Should match well
		"xyzAbcDef",   // Should score low
		"abcDefGhi",   // Should score low
	}

	results := scorer.ScoreMultiple("get user", candidates)

	// All results should be >= MinScore
	for _, result := range results {
		if result.Score.Score < scorer.config.MinScore {
			t.Errorf("result score %.2f below minimum %.2f", result.Score.Score, scorer.config.MinScore)
		}
	}
}

// TestMaxResults tests that results are limited
func TestMaxResults(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	scorer.config.MaxResults = 3

	candidates := []string{
		"getUser", "getUserName", "getUserID", "getUserEmail",
		"getUserPhone", "getUserAddress", "getUserRole",
	}

	results := scorer.ScoreMultiple("get user", candidates)

	if len(results) > scorer.config.MaxResults {
		t.Errorf("results count %d exceeds max %d", len(results), scorer.config.MaxResults)
	}
}

// TestScoreDistribution tests score distribution statistics
func TestScoreDistribution(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	candidates := []string{
		"getUserName",
		"setUserName",
		"getUserID",
		"userName",
		"user",
	}

	result := scorer.Search("get user", candidates)
	dist := scorer.GetScoreDistribution(result)

	if count, ok := dist["count"]; !ok || count.(int) == 0 {
		t.Fatal("expected non-zero count in distribution")
	}

	if avgScore, ok := dist["avg_score"]; !ok {
		t.Fatal("expected avg_score in distribution")
	} else if avgScore.(float64) < 0 || avgScore.(float64) > 1.0 {
		t.Errorf("invalid avg_score: %.2f", avgScore.(float64))
	}
}

// TestConfigureWeights tests custom weight configuration
func TestConfigureWeights(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	customConfig := ScoreLayers{
		ExactWeight:     1.0,
		FuzzyWeight:     0.5, // Lower fuzzy weight
		StemmingWeight:  0.3, // Lower stemming weight
		NameSplitWeight: 0.2,
		FuzzyThreshold:  0.75,
		MaxResults:      5,
		MinScore:        0.2,
	}

	scorer.Configure(customConfig)

	if scorer.config.FuzzyWeight != 0.5 {
		t.Errorf("expected fuzzy weight 0.5, got %.2f", scorer.config.FuzzyWeight)
	}
}

// TestEmptyInputs tests handling of empty inputs
func TestEmptyInputs(t *testing.T) {
	scorer := NewSemanticScorer(
		NewNameSplitter(),
		NewStemmer(true, "porter2", 3, nil),
		NewFuzzyMatcher(true, 0.7, "jaro-winkler"),
		DefaultTranslationDictionary(),
		nil,
	)

	tests := []struct {
		name   string
		query  string
		symbol string
	}{
		{"empty query", "", "getUserName"},
		{"empty symbol", "getUser", ""},
		{"both empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.ScoreSymbol(tt.query, tt.symbol)
			if score.Score != 0 || score.QueryMatch != MatchTypeNone {
				t.Errorf("expected no match for empty inputs, got score %.2f", score.Score)
			}
		})
	}
}

// TestValidScore tests score validation
func TestValidScore(t *testing.T) {
	validScore := SemanticScore{
		Score:      0.75,
		Confidence: 0.9,
	}

	if !validScore.IsValidScore() {
		t.Error("expected valid score")
	}

	invalidScores := []SemanticScore{
		{Score: 1.5, Confidence: 0.9},  // Score > 1.0
		{Score: -0.1, Confidence: 0.9}, // Score < 0.0
		{Score: 0.5, Confidence: 1.5},  // Confidence > 1.0
		{Score: 0.5, Confidence: -0.1}, // Confidence < 0.0
	}

	for _, score := range invalidScores {
		if score.IsValidScore() {
			t.Errorf("expected invalid score to fail validation: %+v", score)
		}
	}
}

// TestStringRepresentations tests string representations for debugging
func TestStringRepresentations(t *testing.T) {
	score := SemanticScore{
		Score:         0.85,
		QueryMatch:    MatchTypeExact,
		Confidence:    0.95,
		Justification: "Test match",
		MatchDetails: map[string]string{
			"query": "test",
		},
	}

	// Test String()
	s := score.String()
	if s == "" {
		t.Error("expected non-empty String() output")
	}

	// Test DebugString()
	debug := score.DebugString()
	if !containsSubstring(debug, "0.850") && !containsSubstring(debug, "0.85") {
		t.Errorf("debug string should contain score: %s", debug)
	}
}

// TestMatchTypeValues tests all match type constants
func TestMatchTypeValues(t *testing.T) {
	types := []MatchType{
		MatchTypeExact,
		MatchTypeSubstring,
		MatchTypePhrase,
		MatchTypeAnnotation,
		MatchTypeFuzzy,
		MatchTypeStemming,
		MatchTypeAbbreviation,
		MatchTypeNameSplit,
		MatchTypeNone,
	}

	for _, mt := range types {
		if mt == "" {
			t.Errorf("match type should not be empty")
		}
	}
}

// Helper function
func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
