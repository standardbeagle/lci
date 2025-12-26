package semantic

import (
	"fmt"
	"strings"

	"github.com/surgebase/porter2"
)

// Stemmer provides word normalization through stemming algorithms
// Enables finding similar words in different forms (authenticate, authentication, authenticating)
type Stemmer struct {
	enabled    bool
	algorithm  string
	minLength  int
	exclusions map[string]bool // Words to never stem
}

// NewStemmer creates a new stemmer with configuration
func NewStemmer(enabled bool, algorithm string, minLength int, exclusions map[string]bool) *Stemmer {
	if algorithm == "" {
		algorithm = "porter2"
	}

	if minLength < 0 {
		minLength = 3
	}

	if exclusions == nil {
		exclusions = make(map[string]bool)
	}

	return &Stemmer{
		enabled:    enabled,
		algorithm:  algorithm,
		minLength:  minLength,
		exclusions: exclusions,
	}
}

// NewStemmerFromDict creates a stemmer from TranslationDictionary config
func NewStemmerFromDict(dict *TranslationDictionary) *Stemmer {
	if dict == nil {
		return NewStemmer(false, "porter2", 3, make(map[string]bool))
	}

	return NewStemmer(
		dict.StemmingConfig.Enabled,
		dict.StemmingConfig.Algorithm,
		dict.StemmingConfig.MinLength,
		dict.StemmingConfig.Exclusions,
	)
}

// IsEnabled checks if stemming is enabled
func (s *Stemmer) IsEnabled() bool {
	return s.enabled
}

// GetAlgorithm returns the configured algorithm
func (s *Stemmer) GetAlgorithm() string {
	return s.algorithm
}

// GetMinLength returns the minimum word length for stemming
func (s *Stemmer) GetMinLength() int {
	return s.minLength
}

// GetExclusions returns the exclusion list
func (s *Stemmer) GetExclusions() map[string]bool {
	exclusions := make(map[string]bool)
	for k, v := range s.exclusions {
		exclusions[k] = v
	}
	return exclusions
}

// Stem returns the stem of a word, or the original word if stemming is disabled/excluded
func (s *Stemmer) Stem(word string) string {
	if !s.enabled {
		return word
	}

	// Check exclusions
	if s.exclusions[strings.ToLower(word)] {
		return word
	}

	// Check minimum length
	if len(word) < s.minLength {
		return word
	}

	// Apply stemming algorithm
	switch s.algorithm {
	case "porter2":
		return porter2.Stem(word)
	case "none":
		return word
	default:
		return porter2.Stem(word)
	}
}

// StemAll applies stemming to multiple words
func (s *Stemmer) StemAll(words []string) []string {
	if !s.enabled {
		return words
	}

	result := make([]string, 0, len(words))
	for _, word := range words {
		result = append(result, s.Stem(word))
	}

	return result
}

// StemAndGroup groups words by their stem
// Useful for finding all variations of a word
func (s *Stemmer) StemAndGroup(words []string) map[string][]string {
	groups := make(map[string][]string)

	for _, word := range words {
		stem := s.Stem(word)
		groups[stem] = append(groups[stem], word)
	}

	return groups
}

// GetVariations finds all variations of a word
// For example: "running" → returns ["run", "running", "runs", "runner"]
// Only returns variations that stem to the same value
func (s *Stemmer) GetVariations(word string, candidates []string) []string {
	if !s.enabled {
		return []string{word}
	}

	stem := s.Stem(word)
	var variations []string

	for _, candidate := range candidates {
		if s.Stem(candidate) == stem {
			variations = append(variations, candidate)
		}
	}

	return variations
}

// NormalizeTerms converts a list of terms to their normalized stems
// Useful for creating a searchable vocabulary
func (s *Stemmer) NormalizeTerms(terms []string) map[string]bool {
	normalized := make(map[string]bool)

	for _, term := range terms {
		stem := s.Stem(term)
		normalized[stem] = true
	}

	return normalized
}

// Enable turns on stemming
func (s *Stemmer) Enable() {
	s.enabled = true
}

// Disable turns off stemming
func (s *Stemmer) Disable() {
	s.enabled = false
}

// SetMinLength updates the minimum length for stemming
func (s *Stemmer) SetMinLength(length int) error {
	if length < 0 {
		return fmt.Errorf("invalid min length: %d (must be >= 0)", length)
	}
	s.minLength = length
	return nil
}

// AddExclusion adds a word to the exclusion list
func (s *Stemmer) AddExclusion(word string) {
	s.exclusions[strings.ToLower(word)] = true
}

// RemoveExclusion removes a word from the exclusion list
func (s *Stemmer) RemoveExclusion(word string) {
	delete(s.exclusions, strings.ToLower(word))
}

// IsExcluded checks if a word is in the exclusion list
func (s *Stemmer) IsExcluded(word string) bool {
	return s.exclusions[strings.ToLower(word)]
}

// ValidateConfig validates the stemmer configuration
func (s *Stemmer) ValidateConfig() error {
	if s.minLength < 0 {
		return fmt.Errorf("invalid min length: %d (must be >= 0)", s.minLength)
	}

	validAlgorithms := map[string]bool{
		"porter2": true,
		"none":    true,
	}

	if !validAlgorithms[s.algorithm] {
		return fmt.Errorf("invalid algorithm: %s (must be porter2 or none)", s.algorithm)
	}

	return nil
}

// Porter2Stemmer represents statistics about Porter2 stemming
type Porter2Stemmer struct {
	inputWords   []string
	stems        []string
	uniqueStems  int
	compressionRatio float64
}

// AnalyzeStemming provides statistics about how stemming affects a word list
func (s *Stemmer) AnalyzeStemming(words []string) *Porter2Stemmer {
	stems := s.StemAll(words)

	// Count unique stems
	uniqueSet := make(map[string]bool)
	for _, stem := range stems {
		uniqueSet[stem] = true
	}

	compressionRatio := float64(len(uniqueSet)) / float64(len(words))
	if len(words) == 0 {
		compressionRatio = 0
	}

	return &Porter2Stemmer{
		inputWords:       words,
		stems:            stems,
		uniqueStems:      len(uniqueSet),
		compressionRatio: compressionRatio,
	}
}

// GetStats returns statistics from stemming analysis
func (ps *Porter2Stemmer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"input_words":        len(ps.inputWords),
		"unique_stems":       ps.uniqueStems,
		"compression_ratio":  ps.compressionRatio,
		"compression_pct":    ps.compressionRatio * 100.0,
	}
}

// StemmerChain applies multiple stemming operations in sequence
// Useful for combining different normalization strategies
type StemmerChain struct {
	stemmers []*Stemmer
}

// NewStemmerChain creates a new stemmer chain
func NewStemmerChain(stemmers ...*Stemmer) *StemmerChain {
	return &StemmerChain{
		stemmers: stemmers,
	}
}

// Process applies all stemmers in sequence
func (sc *StemmerChain) Process(word string) string {
	result := word
	for _, stemmer := range sc.stemmers {
		result = stemmer.Stem(result)
	}
	return result
}

// ProcessAll applies the chain to multiple words
func (sc *StemmerChain) ProcessAll(words []string) []string {
	result := make([]string, 0, len(words))
	for _, word := range words {
		result = append(result, sc.Process(word))
	}
	return result
}

// Porter2Examples provides reference examples of Porter2 stemming
var Porter2Examples = map[string]string{
	"running":         "run",
	"runs":            "run",
	"runner":          "runner",  // Note: not "run" due to suffix rules
	"authentication":  "authent",  // Aggressive but correct per Porter2
	"authenticate":    "authent",
	"authenticated":   "authent",
	"authenticating":  "authent",
	"database":        "databas",
	"databases":       "databas",
	"search":          "search",
	"searching":       "search",
	"searches":        "search",
	"function":        "function",
	"functions":       "function",
	"functional":      "function",
	"server":          "server",
	"servers":         "server",
	"serving":         "server",
	"process":         "process",
	"processes":       "process",
	"processing":      "process",
	"compute":         "comput",
	"computing":       "comput",
	"computation":     "comput",
	"service":         "servic",
	"services":        "servic",
	"serviceable":     "servic",
	"api":             "api",       // Excluded from stemming
	"http":            "http",      // Excluded from stemming
	"authorization":   "author",
	"authorize":       "author",
	"authorized":      "author",
}
