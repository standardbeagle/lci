package semantic

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/surgebase/porter2"
)

// MatchDetector provides methods to detect matches at a specific scoring layer
type MatchDetector interface {
	// Detect returns (matched bool, score float64, justification string, details map[string]string)
	// queryLower and targetLower are pre-normalized (lowercase) versions for performance
	Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string)
}

// ExactMatcher detects exact string matches (case-insensitive)
type ExactMatcher struct{}

func (em *ExactMatcher) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	if queryLower == targetLower {
		return true, config.ExactWeight, "Query matches symbol name exactly", map[string]string{
			"query":      query,
			"targetName": targetName,
		}
	}
	return false, 0, "", nil
}

// SubstringMatcher detects exact substring containment (case-insensitive)
type SubstringMatcher struct{}

func (sm *SubstringMatcher) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	if strings.Contains(targetLower, queryLower) {
		return true, config.SubstringWeight, "Symbol name contains query as substring", map[string]string{
			"query":      query,
			"targetName": targetName,
		}
	}
	return false, 0, "", nil
}

// AnnotationMatcher detects matches based on @lci: annotations
type AnnotationMatcher struct {
	annotationIndex *AnnotationSearchIndex
}

func NewAnnotationMatcher(index *AnnotationSearchIndex) *AnnotationMatcher {
	return &AnnotationMatcher{
		annotationIndex: index,
	}
}

func (am *AnnotationMatcher) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	if am.annotationIndex == nil {
		return false, 0, "", nil
	}

	// Check if query matches any annotation label (AnnotationSearchIndex doesn't have direct symbol search)
	// For now, we check if the label stats contain the query
	labelStats := am.annotationIndex.GetLabelStats()
	if _, exists := labelStats[queryLower]; exists {
		details := map[string]string{
			"query": query,
			"match": "label",
		}
		justification := "Query matches symbol annotation label: " + query
		return true, config.AnnotationWeight, justification, details
	}

	return false, 0, "", nil
}

// FuzzyMatcherDetector detects matches with typo tolerance using Jaro-Winkler
type FuzzyMatcherDetector struct {
	matcher *FuzzyMatcher
}

func NewFuzzyMatcherDetector(matcher *FuzzyMatcher) *FuzzyMatcherDetector {
	return &FuzzyMatcherDetector{
		matcher: matcher,
	}
}

func (fmd *FuzzyMatcherDetector) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	if fmd.matcher == nil {
		return false, 0, "", nil
	}

	// Use existing FuzzyMatcher from phase 3C with pre-normalized strings
	similarity := fmd.matcher.Similarity(queryLower, targetLower)

	if similarity > config.FuzzyThreshold {
		// Score ranges from 0.7 to 0.8 based on similarity above threshold
		score := config.FuzzyWeight * (0.7 + (similarity-config.FuzzyThreshold)*0.1)
		if score > config.FuzzyWeight {
			score = config.FuzzyWeight
		}

		details := map[string]string{
			"query":      query,
			"targetName": targetName,
			"similarity": formatFloat(similarity),
			"threshold":  formatFloat(config.FuzzyThreshold),
		}

		justification := "Fuzzy match: '" + query + "' resembles '" + targetName + "'"
		return true, score, justification, details
	}

	return false, 0, "", nil
}

// StemmingMatcher detects matches based on word stemming
type StemmingMatcher struct {
	splitter *NameSplitter
	stemmer  *Stemmer
}

func NewStemmingMatcher(splitter *NameSplitter, stemmer *Stemmer) *StemmingMatcher {
	return &StemmingMatcher{
		splitter: splitter,
		stemmer:  stemmer,
	}
}

func (sm *StemmingMatcher) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	// Split both query and target into words (use pre-normalized strings)
	queryWords := sm.splitter.Split(queryLower)
	targetWords := sm.splitter.Split(targetLower)

	if len(queryWords) == 0 || len(targetWords) == 0 {
		return false, 0, "", nil
	}

	// Stem all words
	queryStemmed := make([]string, 0, len(queryWords))
	for _, word := range queryWords {
		if len(word) >= config.StemMinLength {
			queryStemmed = append(queryStemmed, porter2.Stem(word))
		}
	}

	targetStemmed := make([]string, 0, len(targetWords))
	for _, word := range targetWords {
		if len(word) >= config.StemMinLength {
			targetStemmed = append(targetStemmed, porter2.Stem(word))
		}
	}

	if len(queryStemmed) == 0 || len(targetStemmed) == 0 {
		return false, 0, "", nil
	}

	// Check if any query stems match any target stems
	matchedStems := 0
	stemMatches := []string{}
	for _, qstem := range queryStemmed {
		for _, tstem := range targetStemmed {
			if qstem == tstem {
				matchedStems++
				stemMatches = append(stemMatches, qstem)
				break
			}
		}
	}

	if matchedStems > 0 {
		// Score based on proportion of matched stems
		matchRatio := float64(matchedStems) / float64(len(queryStemmed))
		score := config.StemmingWeight * matchRatio

		details := map[string]string{
			"query":        query,
			"targetName":   targetName,
			"matchedStems": strings.Join(stemMatches, ", "),
			"matchCount":   formatInt(matchedStems),
			"totalStems":   formatInt(len(queryStemmed)),
		}

		justification := "Stemming match: " + strings.Join(stemMatches, ", ")
		return true, score, justification, details
	}

	return false, 0, "", nil
}

// AbbreviationMatcher detects matches based on abbreviation expansion
type AbbreviationMatcher struct {
	dictionary *TranslationDictionary
	splitter   *NameSplitter // Shared splitter for cache reuse
}

func NewAbbreviationMatcher(dict *TranslationDictionary, splitter *NameSplitter) *AbbreviationMatcher {
	return &AbbreviationMatcher{
		dictionary: dict,
		splitter:   splitter, // Use shared splitter for cache efficiency
	}
}

func (am *AbbreviationMatcher) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	if am.dictionary == nil {
		return false, 0, "", nil
	}

	// Direction 1: Forward expansion - query expands to target
	// Example: "auth" → ["authenticate", "authorization"] matches "AuthenticateUser"
	// Note: Expand() already returns lowercase strings from buildReverseIndexes()
	expandedQuery := am.dictionary.Expand(queryLower)
	forwardMatches := []string{}
	for _, exp := range expandedQuery {
		// exp is already lowercase from dictionary
		if strings.Contains(targetLower, exp) {
			forwardMatches = append(forwardMatches, exp)
		}
	}

	// Direction 2: Reverse expansion - target expands to query
	// Example: "TransactionManager" → ["transaction", "manager"] → "transaction" expands to ["txn"]
	// Use NameSplitter to properly handle camelCase, snake_case, etc.
	targetWords := am.splitter.Split(targetLower)
	reverseMatches := []string{}
	for _, word := range targetWords {
		expandedTarget := am.dictionary.Expand(word)
		for _, exp := range expandedTarget {
			// exp is already lowercase from dictionary
			if strings.Contains(queryLower, exp) && exp != word {
				reverseMatches = append(reverseMatches, word+" → "+exp)
			}
		}
	}

	// Combine both directions
	allMatches := append(forwardMatches, reverseMatches...)

	if len(allMatches) > 0 {
		// Score based on match quality
		matchRatio := float64(len(allMatches)) / float64(len(expandedQuery)+len(targetWords))
		score := config.AbbreviationWeight * matchRatio

		// Build justification showing which direction matched
		var justification string
		if len(forwardMatches) > 0 && len(reverseMatches) > 0 {
			justification = "Bidirectional abbreviation match: " + strings.Join(allMatches, ", ")
		} else if len(forwardMatches) > 0 {
			justification = "Abbreviation expansion: '" + query + "' → " + strings.Join(forwardMatches, ", ")
		} else {
			justification = "Reverse abbreviation match: " + strings.Join(reverseMatches, ", ")
		}

		details := map[string]string{
			"query":          query,
			"targetName":     targetName,
			"matches":        strings.Join(allMatches, ", "),
			"forwardMatches": formatInt(len(forwardMatches)),
			"reverseMatches": formatInt(len(reverseMatches)),
		}

		return true, score, justification, details
	}

	return false, 0, "", nil
}

// NameSplitMatcher detects matches based on name component splitting
type NameSplitMatcher struct {
	splitter *NameSplitter
}

func NewNameSplitMatcher(splitter *NameSplitter) *NameSplitMatcher {
	return &NameSplitMatcher{
		splitter: splitter,
	}
}

func (nsm *NameSplitMatcher) Detect(query string, targetName string, queryLower string, targetLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	// Split both query and target into words (use pre-normalized strings)
	queryWords := nsm.splitter.Split(queryLower)
	targetWords := nsm.splitter.Split(targetLower)

	if len(queryWords) == 0 || len(targetWords) == 0 {
		return false, 0, "", nil
	}

	// Check how many query words appear in target words
	matchedWords := 0
	matchedList := []string{}
	for _, qword := range queryWords {
		for _, tword := range targetWords {
			if qword == tword {
				matchedWords++
				matchedList = append(matchedList, qword)
				break
			}
		}
	}

	if matchedWords > 0 {
		// Score based on proportion of matched words
		matchRatio := float64(matchedWords) / float64(len(queryWords))
		score := config.NameSplitWeight * matchRatio

		details := map[string]string{
			"query":        query,
			"targetName":   targetName,
			"queryWords":   strings.Join(queryWords, ", "),
			"targetWords":  strings.Join(targetWords, ", "),
			"matchedWords": strings.Join(matchedList, ", "),
			"matchCount":   formatInt(matchedWords),
		}

		justification := "Name split match: " + strings.Join(matchedList, ", ")
		return true, score, justification, details
	}

	return false, 0, "", nil
}

// Helper functions for formatting

func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
}

func formatInt(i int) string {
	return strconv.Itoa(i)
}
