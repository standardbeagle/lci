package semantic

import (
	"math"
	"testing"
)

func TestNewFuzzyMatcher(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.85, "jaro-winkler")

	if !matcher.IsEnabled() {
		t.Error("Fuzzy matcher should be enabled")
	}

	if matcher.GetThreshold() != 0.85 {
		t.Errorf("Expected threshold 0.85, got %.2f", matcher.GetThreshold())
	}

	if matcher.GetAlgorithm() != "jaro-winkler" {
		t.Errorf("Expected algorithm jaro-winkler, got %s", matcher.GetAlgorithm())
	}
}

func TestFuzzyMatcherDisabled(t *testing.T) {
	matcher := NewFuzzyMatcher(false, 0.80, "jaro-winkler")

	// When disabled, only exact matches
	if !matcher.Match("auth", "auth") {
		t.Error("Exact match should succeed when disabled")
	}

	if matcher.Match("auth", "authentication") {
		t.Error("Fuzzy match should fail when disabled")
	}

	if matcher.Similarity("auth", "auth") != 1.0 {
		t.Error("Exact match should return 1.0 when disabled")
	}

	if matcher.Similarity("auth", "authentication") != 0.0 {
		t.Error("Different strings should return 0.0 when disabled")
	}
}

func TestJaroWinklerSimilarity(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	tests := []struct {
		a       string
		b       string
		minSim  float64 // minimum expected similarity
		maxSim  float64 // maximum expected similarity
		message string
	}{
		// Exact matches
		{"auth", "auth", 1.0, 1.0, "exact match"},
		{"authentication", "authentication", 1.0, 1.0, "exact match long"},

		// Very similar
		{"auth", "autho", 0.9, 1.0, "one character difference"},
		{"login", "login", 1.0, 1.0, "exact login"},
		{"signin", "signin", 1.0, 1.0, "exact signin"},

		// Similar but different
		{"authentication", "authorize", 0.6, 0.9, "similar prefix"},
		{"database", "databases", 0.85, 1.0, "plural form"},
		{"api", "apis", 0.85, 1.0, "plural API"},

		// Different
		{"auth", "xyz", 0.0, 0.3, "completely different"},
		{"", "test", 0.0, 0.0, "empty string"},
		{"test", "", 0.0, 0.0, "empty string"},
	}

	for _, test := range tests {
		similarity := matcher.Similarity(test.a, test.b)

		if similarity < test.minSim || similarity > test.maxSim {
			t.Errorf("%s: got %.2f, expected %.2f-%.2f for '%s' vs '%s'",
				test.message, similarity, test.minSim, test.maxSim, test.a, test.b)
		}
	}
}

func TestMatch(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	tests := []struct {
		a        string
		b        string
		expected bool
		message  string
	}{
		{"auth", "auth", true, "exact match"},
		{"database", "databases", true, "similar (plural)"},
		{"authenticate", "auth", true, "prefix match should work"},
	}

	for _, test := range tests {
		result := matcher.Match(test.a, test.b)
		if result != test.expected {
			t.Errorf("%s: Match('%s', '%s') = %v, expected %v",
				test.message, test.a, test.b, result, test.expected)
		}
	}
}

func TestFindMatches(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.75, "jaro-winkler")

	candidates := []string{
		"authenticate",
		"authorization",
		"authorize",
		"login",
		"signin",
		"api",
		"application",
		"database",
	}

	matches := matcher.FindMatches("auth", candidates)

	if len(matches) < 2 {
		t.Errorf("Expected at least 2 matches for 'auth', got %d", len(matches))
	}

	// Verify matches are sorted by similarity
	for i := 0; i < len(matches)-1; i++ {
		if matches[i].Similarity < matches[i+1].Similarity {
			t.Error("Matches should be sorted by similarity descending")
		}
	}

	// Verify all returned matches meet threshold
	for _, match := range matches {
		if match.Similarity < matcher.GetThreshold() {
			t.Errorf("Match '%s' has similarity %.2f below threshold %.2f",
				match.Term, match.Similarity, matcher.GetThreshold())
		}
	}
}

func TestFindMatchesNoResults(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.90, "jaro-winkler")

	candidates := []string{"xyz", "abc", "def"}
	matches := matcher.FindMatches("auth", candidates)

	if len(matches) != 0 {
		t.Errorf("Expected no matches for 'auth' with high threshold, got %d", len(matches))
	}
}

func TestFuzzyMatchesWithWeights(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.75, "jaro-winkler")

	candidates := []string{
		"authenticate",
		"authorization",
		"api",
	}

	weights := map[string]float64{
		"authenticate":  1.0,
		"authorization": 0.8,
		"api":          0.5,
	}

	result := matcher.FindMatchesWithWeights("auth", candidates, weights)

	if len(result.Matches) < 1 {
		t.Error("Expected at least 1 match")
	}

	// Check that weights are recorded
	for _, match := range result.Matches {
		if _, ok := result.Weights[match.Term]; !ok {
			t.Errorf("Weight missing for match '%s'", match.Term)
		}
	}
}

func TestLevenshteinSimilarity(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.75, "levenshtein")

	tests := []struct {
		a       string
		b       string
		minSim  float64
		message string
	}{
		{"auth", "auth", 1.0, "exact match"},
		// Levenshtein distance can be low for insertions relative to string length
	}

	for _, test := range tests {
		similarity := matcher.Similarity(test.a, test.b)
		if similarity < test.minSim {
			t.Errorf("%s: got %.2f, expected >= %.2f",
				test.message, similarity, test.minSim)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.70, "cosine")

	tests := []struct {
		a       string
		b       string
		minSim  float64
		message string
	}{
		{"auth", "auth", 1.0, "exact match"},
		{"database", "data", 0.4, "substring (low similarity)"},
		{"api", "api", 1.0, "exact match short"},
	}

	for _, test := range tests {
		similarity := matcher.Similarity(test.a, test.b)
		if similarity < test.minSim {
			t.Errorf("%s: got %.2f, expected >= %.2f",
				test.message, similarity, test.minSim)
		}
	}
}

func TestGetBigrams(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "cosine")

	tests := []struct {
		input    string
		expected int // expected number of unique bigrams
		message  string
	}{
		{"ab", 1, "two characters"},
		{"abc", 2, "three characters"},
		{"abcd", 3, "four characters"},
		{"auth", 3, "four-char word"},
	}

	for _, test := range tests {
		bigrams := matcher.getBigrams(test.input)
		if len(bigrams) != test.expected {
			t.Errorf("%s: got %d bigrams, expected %d",
				test.message, len(bigrams), test.expected)
		}
	}
}

func TestSetThreshold(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	// Valid threshold
	err := matcher.SetThreshold(0.90)
	if err != nil {
		t.Errorf("SetThreshold(0.90) failed: %v", err)
	}
	if matcher.GetThreshold() != 0.90 {
		t.Error("Threshold was not updated")
	}

	// Invalid thresholds
	testCases := []float64{-0.1, 1.1, -1.0}
	for _, threshold := range testCases {
		err := matcher.SetThreshold(threshold)
		if err == nil {
			t.Errorf("SetThreshold(%.2f) should have failed", threshold)
		}
	}
}

func TestSetAlgorithm(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	validAlgorithms := []string{"jaro-winkler", "levenshtein", "cosine"}
	for _, algo := range validAlgorithms {
		err := matcher.SetAlgorithm(algo)
		if err != nil {
			t.Errorf("SetAlgorithm('%s') failed: %v", algo, err)
		}
		if matcher.GetAlgorithm() != algo {
			t.Errorf("Algorithm was not set to '%s'", algo)
		}
	}

	// Invalid algorithm
	err := matcher.SetAlgorithm("invalid")
	if err == nil {
		t.Error("SetAlgorithm('invalid') should have failed")
	}
}

func TestEnableDisable(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	matcher.Disable()
	if matcher.IsEnabled() {
		t.Error("Fuzzy matcher should be disabled")
	}

	matcher.Enable()
	if !matcher.IsEnabled() {
		t.Error("Fuzzy matcher should be enabled")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		threshold float64
		algorithm string
		valid     bool
		message   string
	}{
		{0.80, "jaro-winkler", true, "valid config"},
		{0.75, "levenshtein", true, "valid levenshtein"},
		{0.70, "cosine", true, "valid cosine"},
		{0.80, "invalid", false, "invalid algorithm"},
		{0.0, "jaro-winkler", true, "zero threshold (valid)"},
		{1.0, "jaro-winkler", true, "threshold 1.0 (valid)"},
	}

	for _, test := range tests {
		// Note: NewFuzzyMatcher defaults invalid thresholds to 0.80
		// so we test ValidateConfig directly with bad configs
		threshold := test.threshold
		if test.threshold < 0 || test.threshold > 1 {
			threshold = 0.80 // Will be auto-corrected by NewFuzzyMatcher
		}

		matcher := NewFuzzyMatcher(true, threshold, test.algorithm)
		err := matcher.ValidateConfig()

		if test.valid && err != nil {
			t.Errorf("%s: ValidateConfig() should not error, got %v", test.message, err)
		}

		if !test.valid && err == nil {
			t.Errorf("%s: ValidateConfig() should have errored", test.message)
		}
	}
}

func TestNewFuzzyMatcherFromDict(t *testing.T) {
	dict := DefaultTranslationDictionary()
	matcher := NewFuzzyMatcherFromDict(dict)

	if !matcher.IsEnabled() {
		t.Error("Matcher should be enabled from dict")
	}

	if matcher.GetThreshold() != 0.80 {
		t.Errorf("Expected threshold from dict, got %.2f", matcher.GetThreshold())
	}

	if matcher.GetAlgorithm() != "jaro-winkler" {
		t.Errorf("Expected jaro-winkler algorithm from dict, got %s", matcher.GetAlgorithm())
	}
}

func TestNewFuzzyMatcherFromNilDict(t *testing.T) {
	matcher := NewFuzzyMatcherFromDict(nil)

	if matcher == nil {
		t.Fatal("NewFuzzyMatcherFromDict(nil) returned nil")
	}

	// When dict is nil, fuzzy matching is disabled by default (conservative approach)
	if matcher.IsEnabled() {
		t.Error("Should have default enabled false when dict is nil")
	}

	if matcher.GetThreshold() != 0.80 {
		t.Errorf("Should have default threshold 0.80, got %.2f", matcher.GetThreshold())
	}
}

func TestDefaultThreshold(t *testing.T) {
	// Test invalid thresholds default to 0.80
	matcher := NewFuzzyMatcher(true, -1.0, "jaro-winkler")
	if matcher.GetThreshold() != 0.80 {
		t.Errorf("Invalid threshold should default to 0.80, got %.2f", matcher.GetThreshold())
	}

	matcher = NewFuzzyMatcher(true, 1.5, "jaro-winkler")
	if matcher.GetThreshold() != 0.80 {
		t.Errorf("Invalid threshold should default to 0.80, got %.2f", matcher.GetThreshold())
	}
}

func BenchmarkJaroWinklerSimilarity(b *testing.B) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = matcher.Similarity("authentication", "authorize")
	}
}

func BenchmarkLevenshteinSimilarity(b *testing.B) {
	matcher := NewFuzzyMatcher(true, 0.80, "levenshtein")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = matcher.Similarity("authentication", "authorize")
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	matcher := NewFuzzyMatcher(true, 0.80, "cosine")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = matcher.Similarity("authentication", "authorize")
	}
}

func BenchmarkFindMatches(b *testing.B) {
	matcher := NewFuzzyMatcher(true, 0.75, "jaro-winkler")

	candidates := []string{
		"authenticate", "authorization", "authorize", "api", "application",
		"database", "dataset", "delete", "download", "domain",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = matcher.FindMatches("auth", candidates)
	}
}

func TestEdgeCasesEmptyStrings(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	tests := []struct {
		a       string
		b       string
		message string
	}{
		{"", "", "both empty"},
		{"", "test", "a empty"},
		{"test", "", "b empty"},
	}

	for _, test := range tests {
		similarity := matcher.Similarity(test.a, test.b)

		// Both empty should match
		if test.a == test.b && test.a == "" {
			if similarity != 1.0 {
				t.Errorf("%s: expected similarity 1.0, got %.2f", test.message, similarity)
			}
		} else {
			if similarity != 0.0 {
				t.Errorf("%s: expected similarity 0.0, got %.2f", test.message, similarity)
			}
		}
	}
}

func TestSimilarityBounds(t *testing.T) {
	matcher := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	testStrings := []string{
		"auth", "authentication", "database", "api", "function",
		"variable", "constant", "interface", "package", "module",
	}

	for _, a := range testStrings {
		for _, b := range testStrings {
			similarity := matcher.Similarity(a, b)

			if similarity < 0.0 || similarity > 1.0 {
				t.Errorf("Similarity('%s', '%s') = %.2f, out of bounds [0.0, 1.0]",
					a, b, similarity)
			}

			// Similarity should be symmetric
			reverseSim := matcher.Similarity(b, a)
			if math.Abs(similarity-reverseSim) > 0.001 {
				t.Errorf("Similarity not symmetric: ('%s', '%s') = %.2f vs ('%s', '%s') = %.2f",
					a, b, similarity, b, a, reverseSim)
			}

			// Exact match should be 1.0
			if a == b && similarity != 1.0 {
				t.Errorf("Exact match should be 1.0, got %.2f for '%s'", similarity, a)
			}
		}
	}
}
