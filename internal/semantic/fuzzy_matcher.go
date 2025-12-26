package semantic

import (
	"fmt"
	"math"

	"github.com/hbollon/go-edlib"
)

// FuzzyMatcher provides fuzzy string matching using Jaro-Winkler algorithm
// Enables finding similar terms even with typos or variations
type FuzzyMatcher struct {
	// Configuration from TranslationDictionary
	enabled   bool
	threshold float64
	algorithm string // "jaro-winkler", "levenshtein", "cosine"
}

// NewFuzzyMatcher creates a new fuzzy matcher with default configuration
func NewFuzzyMatcher(enabled bool, threshold float64, algorithm string) *FuzzyMatcher {
	if threshold < 0 || threshold > 1 {
		threshold = 0.80 // Default from TranslationDictionary
	}

	if algorithm == "" {
		algorithm = "jaro-winkler"
	}

	return &FuzzyMatcher{
		enabled:   enabled,
		threshold: threshold,
		algorithm: algorithm,
	}
}

// NewFuzzyMatcherFromDict creates a fuzzy matcher from TranslationDictionary config
func NewFuzzyMatcherFromDict(dict *TranslationDictionary) *FuzzyMatcher {
	if dict == nil {
		return NewFuzzyMatcher(false, 0.80, "jaro-winkler")
	}

	return NewFuzzyMatcher(
		dict.FuzzyConfig.Enabled,
		dict.FuzzyConfig.Threshold,
		dict.FuzzyConfig.Algorithm,
	)
}

// IsEnabled checks if fuzzy matching is enabled
func (fm *FuzzyMatcher) IsEnabled() bool {
	return fm.enabled
}

// GetThreshold returns the configured similarity threshold
func (fm *FuzzyMatcher) GetThreshold() float64 {
	return fm.threshold
}

// GetAlgorithm returns the configured algorithm name
func (fm *FuzzyMatcher) GetAlgorithm() string {
	return fm.algorithm
}

// Match checks if two strings are similar within the configured threshold
func (fm *FuzzyMatcher) Match(a, b string) bool {
	if !fm.enabled {
		return a == b
	}

	similarity := fm.Similarity(a, b)
	return similarity >= fm.threshold
}

// Similarity returns the similarity score between two strings (0.0-1.0)
func (fm *FuzzyMatcher) Similarity(a, b string) float64 {
	if !fm.enabled {
		if a == b {
			return 1.0
		}
		return 0.0
	}

	switch fm.algorithm {
	case "jaro-winkler":
		return fm.jaroWinkler(a, b)
	case "levenshtein":
		return fm.levenshteinSimilarity(a, b)
	case "cosine":
		return fm.cosineSimilarity(a, b)
	default:
		return fm.jaroWinkler(a, b)
	}
}

// jaroWinkler calculates Jaro-Winkler similarity using go-edlib
func (fm *FuzzyMatcher) jaroWinkler(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// go-edlib returns distance (lower is better), we need similarity (higher is better)
	// JaroWinkler returns 0-1 directly
	score, err := edlib.StringsSimilarity(a, b, edlib.JaroWinkler)
	if err != nil {
		return 0.0
	}

	return float64(score)
}

// levenshteinSimilarity calculates Levenshtein-based similarity
func (fm *FuzzyMatcher) levenshteinSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// Get Levenshtein distance
	distance, err := edlib.StringsSimilarity(a, b, edlib.Levenshtein)
	if err != nil {
		return 0.0
	}

	// Convert distance to similarity
	// Similarity = 1 - (distance / max_length)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	if maxLen == 0 {
		return 1.0
	}

	// distance is already normalized to 0-1 range by go-edlib
	return 1.0 - float64(distance)
}

// cosineSimilarity calculates cosine similarity based on character bigrams
func (fm *FuzzyMatcher) cosineSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// Get bigrams for both strings
	bigramsA := fm.getBigrams(a)
	bigramsB := fm.getBigrams(b)

	if len(bigramsA) == 0 || len(bigramsB) == 0 {
		return 0.0
	}

	// Calculate intersection and magnitudes
	intersection := 0.0
	for bigram := range bigramsA {
		if bigramsB[bigram] {
			intersection++
		}
	}

	magnitudeA := math.Sqrt(float64(len(bigramsA)))
	magnitudeB := math.Sqrt(float64(len(bigramsB)))

	if magnitudeA == 0 || magnitudeB == 0 {
		return 0.0
	}

	return intersection / (magnitudeA * magnitudeB)
}

// getBigrams extracts all 2-character subsequences from a string
func (fm *FuzzyMatcher) getBigrams(s string) map[string]bool {
	bigrams := make(map[string]bool)

	if len(s) < 2 {
		bigrams[s] = true
		return bigrams
	}

	for i := 0; i < len(s)-1; i++ {
		bigram := s[i : i+2]
		bigrams[bigram] = true
	}

	return bigrams
}

// FindMatches finds all strings from a list that are similar to target
// Returns matches sorted by similarity score (highest first)
func (fm *FuzzyMatcher) FindMatches(target string, candidates []string) []FuzzyMatch {
	var matches []FuzzyMatch

	for _, candidate := range candidates {
		similarity := fm.Similarity(target, candidate)
		if similarity >= fm.threshold {
			matches = append(matches, FuzzyMatch{
				Term:       candidate,
				Similarity: similarity,
			})
		}
	}

	// Sort by similarity descending
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Similarity > matches[i].Similarity {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

// FuzzyMatch represents a fuzzy match result
type FuzzyMatch struct {
	Term       string
	Similarity float64
}

// FuzzyMatchesWithWeights finds matches and returns with combined weights
type FuzzyMatchesWithWeights struct {
	Matches []FuzzyMatch
	Weights map[string]float64 // term → combined weight
}

// FindMatchesWithWeights combines multiple weighted candidates with fuzzy matching
// Useful for combining abbreviation dictionary with fuzzy matches
func (fm *FuzzyMatcher) FindMatchesWithWeights(target string, candidates []string, weights map[string]float64) *FuzzyMatchesWithWeights {
	fuzzyMatches := fm.FindMatches(target, candidates)

	result := &FuzzyMatchesWithWeights{
		Matches: make([]FuzzyMatch, 0),
		Weights: make(map[string]float64),
	}

	for _, match := range fuzzyMatches {
		// Combine fuzzy similarity with precomputed weight
		weight := 0.0
		if w, ok := weights[match.Term]; ok {
			weight = w
		}

		// Combined score: 70% fuzzy similarity + 30% weight
		combinedScore := 0.7*match.Similarity + 0.3*weight

		result.Matches = append(result.Matches, FuzzyMatch{
			Term:       match.Term,
			Similarity: combinedScore,
		})

		result.Weights[match.Term] = combinedScore
	}

	return result
}

// ValidateConfig validates fuzzy matcher configuration
func (fm *FuzzyMatcher) ValidateConfig() error {
	if fm.threshold < 0 || fm.threshold > 1 {
		return fmt.Errorf("invalid threshold: %.2f (must be 0-1)", fm.threshold)
	}

	validAlgorithms := map[string]bool{
		"jaro-winkler": true,
		"levenshtein":  true,
		"cosine":       true,
	}

	if !validAlgorithms[fm.algorithm] {
		return fmt.Errorf("invalid algorithm: %s (must be jaro-winkler, levenshtein, or cosine)", fm.algorithm)
	}

	return nil
}

// SetThreshold updates the similarity threshold
func (fm *FuzzyMatcher) SetThreshold(threshold float64) error {
	if threshold < 0 || threshold > 1 {
		return fmt.Errorf("invalid threshold: %.2f (must be 0-1)", threshold)
	}
	fm.threshold = threshold
	return nil
}

// SetAlgorithm changes the matching algorithm
func (fm *FuzzyMatcher) SetAlgorithm(algorithm string) error {
	validAlgorithms := map[string]bool{
		"jaro-winkler": true,
		"levenshtein":  true,
		"cosine":       true,
	}

	if !validAlgorithms[algorithm] {
		return fmt.Errorf("invalid algorithm: %s (must be jaro-winkler, levenshtein, or cosine)", algorithm)
	}

	fm.algorithm = algorithm
	return nil
}

// Enable turns on fuzzy matching
func (fm *FuzzyMatcher) Enable() {
	fm.enabled = true
}

// Disable turns off fuzzy matching
func (fm *FuzzyMatcher) Disable() {
	fm.enabled = false
}
