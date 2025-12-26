package semantic

import (
	"sort"
	"strings"
	"time"
)

// SemanticScorer provides unified scoring across all semantic search layers
type SemanticScorer struct {
	// Configuration
	config ScoreLayers

	// Matchers in priority order
	matchers []Matcher

	// Shared components
	splitter   *NameSplitter
	stemmer    *Stemmer
	fuzzer     *FuzzyMatcher
	dictionary *TranslationDictionary
	annIndex   *AnnotationSearchIndex

	// Query normalization cache (bounded LRU)
	queryCache *LRUCache
}

// Matcher interface for match detection
type Matcher interface {
	Detect(query, symbolName, queryLower, symbolLower string, config ScoreLayers) (bool, float64, string, map[string]string)
}

// normalizedQuery caches pre-processed query information
type normalizedQuery struct {
	original string
	words    []string
	stems    []string
}

// NewSemanticScorer creates a new semantic scorer with all layers
func NewSemanticScorer(
	splitter *NameSplitter,
	stemmer *Stemmer,
	fuzzer *FuzzyMatcher,
	dict *TranslationDictionary,
	annIndex *AnnotationSearchIndex,
) *SemanticScorer {
	// Create phrase matcher with full semantic features:
	// - Porter2 stemming (running → run)
	// - Dictionary synonym expansion (signin ↔ login)
	// - Abbreviation expansion (auth → authenticate)
	phraseMatcher := NewPhraseMatcherFull(splitter, fuzzer, stemmer, dict)

	matchers := []Matcher{
		&ExactMatcher{},
		&SubstringMatcher{},
		NewPhraseMatcherDetector(phraseMatcher), // Multi-word phrase matching (early for high priority)
		NewAnnotationMatcher(annIndex),
		NewFuzzyMatcherDetector(fuzzer),
		NewStemmingMatcher(splitter, stemmer),
		NewNameSplitMatcher(splitter),
		NewAbbreviationMatcher(dict, splitter),
	}

	return &SemanticScorer{
		config:     DefaultScoreLayers,
		matchers:   matchers,
		splitter:   splitter,
		stemmer:    stemmer,
		fuzzer:     fuzzer,
		dictionary: dict,
		annIndex:   annIndex,
		queryCache: NewLRUCache(1000),
	}
}

// Configure updates the scoring configuration
func (ss *SemanticScorer) Configure(layers ScoreLayers) {
	ss.config = layers
}

// GetConfig returns the current configuration
func (ss *SemanticScorer) GetConfig() ScoreLayers {
	return ss.config
}

// ScoreSymbol scores a single symbol against a query
func (ss *SemanticScorer) ScoreSymbol(query string, symbolName string) SemanticScore {
	// Validate and normalize inputs
	if validationResult := ss.validateInputs(query, symbolName); validationResult != nil {
		return *validationResult
	}

	queryLower, symbolLower := ss.normalizeInputs(&query, &symbolName)

	// Run all matchers and keep the best result
	// This ensures higher-quality matches from later matchers aren't shadowed
	// by lower-quality matches from earlier ones
	var bestResult *SemanticScore
	for i, matcher := range ss.matchers {
		if result := ss.tryMatch(matcher, i, query, symbolName, queryLower, symbolLower); result != nil {
			if bestResult == nil || result.Score > bestResult.Score {
				bestResult = result
			}
		}
	}

	if bestResult != nil {
		return *bestResult
	}

	// No match found
	return SemanticScore{
		Score:         0,
		QueryMatch:    MatchTypeNone,
		Confidence:    0,
		Justification: "No semantic match found",
		MatchDetails:  map[string]string{},
	}
}

// tryMatch attempts a match with a specific matcher
func (ss *SemanticScorer) tryMatch(matcher Matcher, index int, query, symbolName, queryLower, symbolLower string) *SemanticScore {
	matched, score, justification, details := matcher.Detect(query, symbolName, queryLower, symbolLower, ss.config)

	if matched {
		return &SemanticScore{
			Score:         score,
			QueryMatch:    MatchType(indexToMatchType(index)),
			Confidence:    matchTypeToConfidence(indexToMatchType(index)),
			Justification: justification,
			MatchDetails:  details,
		}
	}

	return nil
}

// indexToMatchType converts matcher index to MatchType
func indexToMatchType(index int) MatchType {
	matchTypes := []MatchType{
		MatchTypeExact,
		MatchTypeSubstring,
		MatchTypePhrase, // Multi-word phrase matching
		MatchTypeAnnotation,
		MatchTypeFuzzy,
		MatchTypeStemming,
		MatchTypeNameSplit,
		MatchTypeAbbreviation,
	}

	if index >= 0 && index < len(matchTypes) {
		return matchTypes[index]
	}
	return MatchTypeNone
}

// matchTypeToConfidence returns confidence level for a match type
func matchTypeToConfidence(matchType MatchType) float64 {
	confidences := map[MatchType]float64{
		MatchTypeExact:        1.0,
		MatchTypeSubstring:    0.95,
		MatchTypePhrase:       0.92, // High confidence for phrase matches
		MatchTypeAnnotation:   0.90,
		MatchTypeFuzzy:        0.80,
		MatchTypeStemming:     0.70,
		MatchTypeNameSplit:    0.60,
		MatchTypeAbbreviation: 0.50,
	}

	return confidences[matchType]
}

// validateInputs checks for empty inputs
func (ss *SemanticScorer) validateInputs(query, symbolName string) *SemanticScore {
	if query == "" || symbolName == "" {
		return &SemanticScore{
			Score:         0,
			QueryMatch:    MatchTypeNone,
			Confidence:    0,
			Justification: "Empty query or symbol name",
			MatchDetails:  map[string]string{},
		}
	}
	return nil
}

// normalizeInputs trims and lowercases the query and symbol name
func (ss *SemanticScorer) normalizeInputs(query, symbolName *string) (string, string) {
	*query = strings.TrimSpace(*query)
	*symbolName = strings.TrimSpace(*symbolName)
	return strings.ToLower(*query), strings.ToLower(*symbolName)
}

// ScoreMultiple scores multiple symbols and returns them ranked by score
func (ss *SemanticScorer) ScoreMultiple(query string, symbolNames []string) []ScoredSymbol {
	if len(symbolNames) == 0 {
		return []ScoredSymbol{}
	}

	// Score all symbols
	scoredSymbols := make([]ScoredSymbol, 0, len(symbolNames))
	for _, name := range symbolNames {
		score := ss.ScoreSymbol(query, name)

		// Only include results above minimum threshold
		if score.Score >= ss.config.MinScore {
			scoredSymbols = append(scoredSymbols, ScoredSymbol{
				Symbol: name,
				Score:  score,
				Rank:   0, // Will be assigned after sorting
			})
		}
	}

	// Sort by score descending
	sort.Slice(scoredSymbols, func(i, j int) bool {
		if scoredSymbols[i].Score.Score != scoredSymbols[j].Score.Score {
			return scoredSymbols[i].Score.Score > scoredSymbols[j].Score.Score
		}
		// Tiebreaker: sort by confidence descending
		return scoredSymbols[i].Score.Confidence > scoredSymbols[j].Score.Confidence
	})

	// Assign ranks and limit results
	maxResults := ss.config.MaxResults
	if maxResults == 0 {
		maxResults = 10 // Default if not configured
	}

	if len(scoredSymbols) > maxResults {
		scoredSymbols = scoredSymbols[:maxResults]
	}

	// Assign final ranks
	for i := range scoredSymbols {
		scoredSymbols[i].Rank = i + 1
	}

	return scoredSymbols
}

// Search performs a complete semantic search and returns ranked results
func (ss *SemanticScorer) Search(query string, candidates []string) SearchResult {
	startTime := time.Now()

	result := SearchResult{
		Query:                query,
		CandidatesConsidered: len(candidates),
		Symbols:              ss.ScoreMultiple(query, candidates),
	}

	result.ResultsReturned = len(result.Symbols)
	result.ExecutionTime = time.Since(startTime).Nanoseconds()

	return result
}

// SearchWithMinScore performs search and returns all results above minimum score
func (ss *SemanticScorer) SearchWithMinScore(query string, candidates []string, minScore float64) SearchResult {
	oldMinScore := ss.config.MinScore
	ss.config.MinScore = minScore
	defer func() {
		ss.config.MinScore = oldMinScore
	}()

	return ss.Search(query, candidates)
}

// SearchWithMaxResults performs search with custom result limit
func (ss *SemanticScorer) SearchWithMaxResults(query string, candidates []string, maxResults int) SearchResult {
	oldMaxResults := ss.config.MaxResults
	ss.config.MaxResults = maxResults
	defer func() {
		ss.config.MaxResults = oldMaxResults
	}()

	return ss.Search(query, candidates)
}

// GetScoreDistribution returns statistics about score distribution in a result set
func (ss *SemanticScorer) GetScoreDistribution(result SearchResult) map[string]interface{} {
	// Handle empty result set
	if ss.isEmptyResult(result) {
		return ss.emptyDistribution()
	}

	// Calculate statistics
	stats := ss.calculateDistributionStats(result.Symbols)

	// Convert distribution to string keys for JSON compatibility
	dist := ss.convertDistributionToStringKeys(stats.distribution)

	return map[string]interface{}{
		"count":        len(result.Symbols),
		"avg_score":    stats.avgScore,
		"median_score": stats.medianScore,
		"min_score":    stats.minScore,
		"max_score":    stats.maxScore,
		"distribution": dist,
	}
}

// isEmptyResult checks if the result set is empty
func (ss *SemanticScorer) isEmptyResult(result SearchResult) bool {
	return len(result.Symbols) == 0
}

// emptyDistribution returns statistics for an empty result set
func (ss *SemanticScorer) emptyDistribution() map[string]interface{} {
	return map[string]interface{}{
		"count":        0,
		"avg_score":    0,
		"median_score": 0,
		"min_score":    0,
		"max_score":    0,
		"distribution": map[string]int{},
	}
}

// distributionStats holds calculated statistics
type distributionStats struct {
	minScore     float64
	maxScore     float64
	avgScore     float64
	medianScore  float64
	distribution map[MatchType]int
}

// calculateDistributionStats computes statistics from scored symbols
func (ss *SemanticScorer) calculateDistributionStats(symbols []ScoredSymbol) distributionStats {
	var sum float64
	minScore := symbols[0].Score.Score
	maxScore := symbols[0].Score.Score
	distribution := make(map[MatchType]int)

	for _, sym := range symbols {
		score := sym.Score.Score
		sum += score
		minScore = ss.updateMin(minScore, score)
		maxScore = ss.updateMax(maxScore, score)
		distribution[sym.Score.QueryMatch]++
	}

	avgScore := sum / float64(len(symbols))
	medianScore := symbols[len(symbols)/2].Score.Score

	return distributionStats{
		minScore:     minScore,
		maxScore:     maxScore,
		avgScore:     avgScore,
		medianScore:  medianScore,
		distribution: distribution,
	}
}

// updateMin updates minimum value if needed
func (ss *SemanticScorer) updateMin(currentMin, value float64) float64 {
	if value < currentMin {
		return value
	}
	return currentMin
}

// updateMax updates maximum value if needed
func (ss *SemanticScorer) updateMax(currentMax, value float64) float64 {
	if value > currentMax {
		return value
	}
	return currentMax
}

// convertDistributionToStringKeys converts MatchType map to string map
func (ss *SemanticScorer) convertDistributionToStringKeys(distribution map[MatchType]int) map[string]int {
	dist := make(map[string]int)
	for matchType, count := range distribution {
		dist[string(matchType)] = count
	}
	return dist
}

// IsValidScore checks if a score meets quality criteria
func (ss *SemanticScorer) IsValidScore(score SemanticScore) bool {
	return score.IsValidScore() && score.Score >= ss.config.MinScore
}

// ClearCache clears the query normalization cache
func (ss *SemanticScorer) ClearCache() {
	ss.queryCache.Clear()
}
