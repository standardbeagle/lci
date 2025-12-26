package semantic

import (
	"sort"
	"strings"
)

// PhraseMatchResult represents the result of a phrase match operation
type PhraseMatchResult struct {
	Matched       bool     // Whether the phrase matched
	IsExactPhrase bool     // Whether all words matched in order
	Score         float64  // Overall match score (0.0-1.0)
	Target        string   // The target that was matched against
	MatchedWords  []string // Words from query that matched
	Justification string   // Human-readable explanation
}

// PhraseMatcher provides natural language phrase matching for multi-word queries
// It applies the full range of existing matchers (exact, fuzzy, stemming, etc.)
// to each word while preserving phrase-level ranking.
//
// Ranking priority:
// 1. Exact phrase match (all words present in order) - highest score
// 2. All words match but out of order - high score
// 3. Partial word matches with fuzzy matching - medium score
// 4. Single word matches - lower score
type PhraseMatcher struct {
	splitter   *NameSplitter
	fuzzer     *FuzzyMatcher
	stemmer    *Stemmer               // Full Porter2 stemmer for word normalization
	dictionary *TranslationDictionary // For synonym/vocabulary expansion (signin ↔ login)

	// Score weights for different match types
	exactPhraseBonus float64 // Bonus for exact phrase (all words in order)
	allWordsBonus    float64 // Bonus for all words matching (any order)
	wordOrderBonus   float64 // Bonus per word that matches in correct position
	fuzzyPenalty     float64 // Penalty for fuzzy vs exact word match
}

// NewPhraseMatcher creates a new phrase matcher with default configuration
// Uses simplified stemming - for full semantic matching, use NewPhraseMatcherFull
func NewPhraseMatcher(splitter *NameSplitter, fuzzer *FuzzyMatcher) *PhraseMatcher {
	return &PhraseMatcher{
		splitter:         splitter,
		fuzzer:           fuzzer,
		stemmer:          nil,  // Will use simplified stemming
		dictionary:       nil,  // No synonym expansion
		exactPhraseBonus: 0.05, // Bonus for exact phrase (all words in order)
		allWordsBonus:    0.02, // Bonus for all words matching (any order)
		wordOrderBonus:   0.03, // Bonus/penalty per word for order
		fuzzyPenalty:     0.08, // Penalty for fuzzy vs exact word match
	}
}

// NewPhraseMatcherFull creates a phrase matcher with all semantic features:
// - Full Porter2 stemming (running → run)
// - Dictionary synonym expansion (signin ↔ login)
// - Abbreviation expansion (auth → authenticate)
func NewPhraseMatcherFull(splitter *NameSplitter, fuzzer *FuzzyMatcher, stemmer *Stemmer, dict *TranslationDictionary) *PhraseMatcher {
	return &PhraseMatcher{
		splitter:         splitter,
		fuzzer:           fuzzer,
		stemmer:          stemmer,
		dictionary:       dict,
		exactPhraseBonus: 0.05,
		allWordsBonus:    0.02,
		wordOrderBonus:   0.03,
		fuzzyPenalty:     0.08,
	}
}

// Match checks if a multi-word query matches a target symbol name
// Returns detailed match information including score and match type
func (pm *PhraseMatcher) Match(query, target string) PhraseMatchResult {
	// Handle empty inputs
	query = strings.TrimSpace(query)
	target = strings.TrimSpace(target)

	if query == "" || target == "" {
		return PhraseMatchResult{
			Matched: false,
			Target:  target,
		}
	}

	// Split query into words (handles spaces) - lowercase first since query is natural language
	queryLower := strings.ToLower(query)
	queryWords := pm.splitQuery(queryLower)
	if len(queryWords) == 0 {
		return PhraseMatchResult{
			Matched: false,
			Target:  target,
		}
	}

	// Split target BEFORE lowercasing to preserve camelCase boundaries
	// NameSplitter.Split() already returns lowercase words
	targetWords := pm.splitter.Split(target)
	if len(targetWords) == 0 {
		return PhraseMatchResult{
			Matched: false,
			Target:  target,
		}
	}

	// Try to match each query word against target words
	wordMatches := pm.matchWords(queryWords, targetWords)

	// Calculate overall match result
	return pm.calculateResult(query, target, queryWords, targetWords, wordMatches)
}

// splitQuery splits a query string into words, handling natural language input
func (pm *PhraseMatcher) splitQuery(query string) []string {
	// Split on whitespace
	parts := strings.Fields(query)

	words := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			words = append(words, part)
		}
	}

	return words
}

// wordMatch represents how a query word matched a target word
type wordMatch struct {
	queryWord      string
	targetWord     string
	targetIndex    int     // Index in target words (-1 if no match)
	matchScore     float64 // Score for this word match (0.0-1.0)
	isExact        bool    // Exact match (no fuzzy)
	isFuzzy        bool    // Fuzzy match
	isStemMatch    bool    // Stem match
	isAbbrevMatch  bool    // Abbreviation match (param -> parameter)
	isSynonymMatch bool    // Dictionary synonym match (signin -> login)
}

// matchWords matches each query word against target words
// Returns a slice of wordMatch results for each query word
func (pm *PhraseMatcher) matchWords(queryWords, targetWords []string) []wordMatch {
	matches := make([]wordMatch, len(queryWords))
	// Use a slice instead of map for small arrays (typically <10 words)
	// This avoids map allocation overhead
	usedTargetIndices := make([]bool, len(targetWords))

	// First pass: find exact matches
	for i, qw := range queryWords {
		matches[i] = wordMatch{
			queryWord:   qw,
			targetIndex: -1,
			matchScore:  0,
		}

		for j, tw := range targetWords {
			if usedTargetIndices[j] {
				continue
			}

			if qw == tw {
				matches[i].targetWord = tw
				matches[i].targetIndex = j
				matches[i].matchScore = 1.0
				matches[i].isExact = true
				usedTargetIndices[j] = true
				break
			}
		}
	}

	// Second pass: find substring matches for unmatched words
	for i := range matches {
		if matches[i].targetIndex >= 0 {
			continue // Already matched
		}

		qw := matches[i].queryWord

		for j, tw := range targetWords {
			if usedTargetIndices[j] {
				continue
			}

			// Check if query word is contained in target word
			if strings.Contains(tw, qw) || strings.Contains(qw, tw) {
				matches[i].targetWord = tw
				matches[i].targetIndex = j
				matches[i].matchScore = 0.95
				matches[i].isExact = true // Substring still counts as "exact" for ranking
				usedTargetIndices[j] = true
				break
			}
		}
	}

	// Third pass: find fuzzy matches for remaining unmatched words
	for i := range matches {
		if matches[i].targetIndex >= 0 {
			continue // Already matched
		}

		qw := matches[i].queryWord
		bestScore := 0.0
		bestIdx := -1
		bestTarget := ""

		for j, tw := range targetWords {
			if usedTargetIndices[j] {
				continue
			}

			// Try fuzzy match
			similarity := pm.fuzzer.Similarity(qw, tw)
			if similarity >= pm.fuzzer.GetThreshold() && similarity > bestScore {
				bestScore = similarity
				bestIdx = j
				bestTarget = tw
			}
		}

		if bestIdx >= 0 {
			matches[i].targetWord = bestTarget
			matches[i].targetIndex = bestIdx
			matches[i].matchScore = bestScore
			matches[i].isFuzzy = true
			usedTargetIndices[bestIdx] = true
		}
	}

	// Fourth pass: check for abbreviation matches (param -> parameter, etc.)
	for i := range matches {
		if matches[i].targetIndex >= 0 {
			continue // Already matched
		}

		qw := matches[i].queryWord

		for j, tw := range targetWords {
			if usedTargetIndices[j] {
				continue
			}

			// Check common abbreviation patterns
			if pm.isAbbreviationMatch(qw, tw) {
				matches[i].targetWord = tw
				matches[i].targetIndex = j
				matches[i].matchScore = 0.85
				matches[i].isAbbrevMatch = true
				usedTargetIndices[j] = true
				break
			}
		}
	}

	// Fifth pass: check stem matches for remaining words
	for i := range matches {
		if matches[i].targetIndex >= 0 {
			continue // Already matched
		}

		qw := matches[i].queryWord
		qwStem := pm.stem(qw)

		for j, tw := range targetWords {
			if usedTargetIndices[j] {
				continue
			}

			twStem := pm.stem(tw)
			if qwStem == twStem && qwStem != "" {
				matches[i].targetWord = tw
				matches[i].targetIndex = j
				matches[i].matchScore = 0.80
				matches[i].isStemMatch = true
				usedTargetIndices[j] = true
				break
			}
		}
	}

	// Sixth pass: check dictionary synonym matches (signin ↔ login, etc.)
	if pm.dictionary != nil {
		for i := range matches {
			if matches[i].targetIndex >= 0 {
				continue // Already matched
			}

			qw := matches[i].queryWord
			// Expand query word to get all synonyms
			synonyms := pm.dictionary.Expand(qw)

			for j, tw := range targetWords {
				if usedTargetIndices[j] {
					continue
				}

				// Check if target word matches any synonym
				for _, syn := range synonyms {
					if syn == tw {
						matches[i].targetWord = tw
						matches[i].targetIndex = j
						matches[i].matchScore = 0.82 // Slightly higher than stem since it's semantic
						matches[i].isSynonymMatch = true
						usedTargetIndices[j] = true
						break
					}
				}
				if matches[i].targetIndex >= 0 {
					break
				}
			}
		}
	}

	return matches
}

// isAbbreviationMatch checks if one word is an abbreviation of another
func (pm *PhraseMatcher) isAbbreviationMatch(short, long string) bool {
	// Common abbreviations
	abbrevs := map[string][]string{
		"param":  {"parameter", "parameters"},
		"params": {"parameters", "parameter"},
		"arg":    {"argument", "arguments"},
		"args":   {"arguments", "argument"},
		"config": {"configuration", "configure"},
		"cfg":    {"configuration", "config", "configure"},
		"msg":    {"message", "messages"},
		"err":    {"error", "errors"},
		"req":    {"request", "requests"},
		"resp":   {"response", "responses"},
		"res":    {"response", "result", "resource"},
		"ctx":    {"context"},
		"conn":   {"connection", "connect"},
		"db":     {"database"},
		"auth":   {"authentication", "authorize", "authorization"},
		"init":   {"initialize", "initialization"},
		"info":   {"information"},
		"mgr":    {"manager"},
		"srv":    {"server", "service"},
		"svc":    {"service"},
		"util":   {"utility", "utilities"},
		"utils":  {"utilities", "utility"},
		"func":   {"function"},
		"fn":     {"function"},
		"var":    {"variable"},
		"val":    {"value"},
		"num":    {"number"},
		"str":    {"string"},
		"int":    {"integer"},
		"buf":    {"buffer"},
		"len":    {"length"},
		"idx":    {"index"},
		"ptr":    {"pointer"},
		"src":    {"source"},
		"dst":    {"destination"},
		"dir":    {"directory"},
		"tmp":    {"temporary", "temp"},
		"temp":   {"temporary"},
		"max":    {"maximum"},
		"min":    {"minimum"},
		"avg":    {"average"},
		"cnt":    {"count"},
		"async":  {"asynchronous"},
		"sync":   {"synchronous", "synchronize"},
		"doc":    {"document", "documentation"},
		"docs":   {"documents", "documentation"},
		"impl":   {"implementation", "implement"},
		"exec":   {"execute", "execution"},
		"cmd":    {"command"},
		"opt":    {"option", "optional"},
		"opts":   {"options"},
		"attr":   {"attribute", "attributes"},
		"attrs":  {"attributes"},
		"prop":   {"property"},
		"props":  {"properties"},
		"elem":   {"element"},
		"obj":    {"object"},
		"ref":    {"reference"},
		"refs":   {"references"},
		"prev":   {"previous"},
		"curr":   {"current"},
		"next":   {"next"},
		"desc":   {"description", "descending"},
		"asc":    {"ascending"},
	}

	// Check if short is abbreviation of long
	if expansions, ok := abbrevs[short]; ok {
		for _, exp := range expansions {
			if exp == long || strings.HasPrefix(long, exp) || strings.HasPrefix(exp, long) {
				return true
			}
		}
	}

	// Check if long is abbreviation of short (reverse lookup)
	if expansions, ok := abbrevs[long]; ok {
		for _, exp := range expansions {
			if exp == short || strings.HasPrefix(short, exp) || strings.HasPrefix(exp, short) {
				return true
			}
		}
	}

	// Check if short is a prefix of long (at least 3 chars)
	if len(short) >= 3 && len(long) > len(short) {
		if strings.HasPrefix(long, short) {
			return true
		}
	}

	return false
}

// stem returns the stem of a word
// Uses full Porter2 stemmer if available, otherwise falls back to simplified stemming
func (pm *PhraseMatcher) stem(word string) string {
	// Use full Porter2 stemmer if available
	if pm.stemmer != nil && pm.stemmer.IsEnabled() {
		return pm.stemmer.Stem(word)
	}

	// Fallback to simplified stemming
	return pm.simpleStem(word)
}

// simpleStem provides basic suffix removal for when full stemmer is not available
func (pm *PhraseMatcher) simpleStem(word string) string {
	if len(word) < 4 {
		return word
	}

	// Remove common suffixes
	suffixes := []string{
		"ing", "tion", "sion", "ment", "ness", "able", "ible",
		"ful", "less", "ous", "ive", "ity", "er", "or", "ly",
		"ed", "es", "s",
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) && len(word) > len(suffix)+2 {
			return strings.TrimSuffix(word, suffix)
		}
	}

	return word
}

// calculateResult computes the final match result from word matches
func (pm *PhraseMatcher) calculateResult(
	query, target string,
	queryWords, targetWords []string,
	wordMatches []wordMatch,
) PhraseMatchResult {
	// Count matched words and track match quality
	matchedCount := 0
	matchedWords := make([]string, 0, len(queryWords))
	totalWordScore := 0.0
	fuzzyCount := 0
	allExactMatches := true // True if all matched words are exact (not fuzzy/stem/abbrev)

	for _, wm := range wordMatches {
		if wm.targetIndex >= 0 {
			matchedCount++
			matchedWords = append(matchedWords, wm.queryWord)
			totalWordScore += wm.matchScore
			if wm.isFuzzy {
				fuzzyCount++
				allExactMatches = false
			}
			if wm.isStemMatch || wm.isAbbrevMatch || wm.isSynonymMatch {
				allExactMatches = false
			}
		}
	}

	// No matches at all
	if matchedCount == 0 {
		return PhraseMatchResult{
			Matched: false,
			Target:  target,
		}
	}

	// Calculate base score from word matches
	// Scale avgWordScore to leave room for bonuses (max base = 0.85)
	avgWordScore := (totalWordScore / float64(len(queryWords))) * 0.85

	// Check if all query words matched
	allWordsMatched := matchedCount == len(queryWords)

	// Check if words are in order (for phrase matching)
	inOrder := pm.areWordsInOrder(wordMatches)

	// Calculate final score
	score := avgWordScore

	// Apply bonuses for phrase matching
	if allWordsMatched && inOrder {
		// Exact phrase match - highest bonus
		score += pm.exactPhraseBonus
	} else if allWordsMatched {
		// All words match but not in order - smaller bonus
		score += pm.allWordsBonus
	}

	// Bonus for word order preservation (only if in order)
	if inOrder && matchedCount > 1 {
		score += pm.wordOrderBonus * float64(matchedCount-1)
	}

	// Penalty for out-of-order matches
	if !inOrder && matchedCount > 1 {
		score -= pm.wordOrderBonus * float64(matchedCount) // Penalty proportional to word count
	}

	// Penalty for fuzzy matches
	if fuzzyCount > 0 {
		score -= pm.fuzzyPenalty * float64(fuzzyCount) / float64(matchedCount)
	}

	// Clamp score to valid range [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0 {
		score = 0
	}

	// IsExactPhrase is only true if ALL conditions are met:
	// 1. All query words matched
	// 2. Words are in correct order
	// 3. All matches were exact (no fuzzy/stem/abbrev)
	isExactPhrase := allWordsMatched && inOrder && allExactMatches

	// Build justification
	justification := pm.buildJustification(queryWords, wordMatches, allWordsMatched, inOrder)

	return PhraseMatchResult{
		Matched:       true,
		IsExactPhrase: isExactPhrase,
		Score:         score,
		Target:        target,
		MatchedWords:  matchedWords,
		Justification: justification,
	}
}

// areWordsInOrder checks if matched words appear in the same order in target
func (pm *PhraseMatcher) areWordsInOrder(wordMatches []wordMatch) bool {
	lastIdx := -1
	for _, wm := range wordMatches {
		if wm.targetIndex < 0 {
			continue // Skip unmatched
		}
		if wm.targetIndex <= lastIdx {
			return false
		}
		lastIdx = wm.targetIndex
	}
	return true
}

// buildJustification creates a human-readable explanation of the match
// Optimized to reduce string allocations by using strings.Builder
func (pm *PhraseMatcher) buildJustification(
	queryWords []string,
	wordMatches []wordMatch,
	allMatched, inOrder bool,
) string {
	// Pre-allocate builder with estimated capacity to avoid reallocations
	// Estimate: prefix (~25) + per word (~20 chars each) + separators
	var b strings.Builder
	b.Grow(25 + len(wordMatches)*25)

	// Write prefix directly
	if allMatched && inOrder {
		b.WriteString("Exact phrase match: ")
	} else if allMatched {
		b.WriteString("All words match (unordered): ")
	} else {
		b.WriteString("Partial phrase match: ")
	}

	// Write word matches without intermediate slice allocation
	first := true
	for _, wm := range wordMatches {
		if !first {
			b.WriteString(", ")
		}
		first = false

		if wm.targetIndex < 0 {
			b.WriteString(wm.queryWord)
			b.WriteString(" (no match)")
		} else if wm.isExact {
			b.WriteString(wm.queryWord)
			b.WriteString(" → ")
			b.WriteString(wm.targetWord)
		} else if wm.isFuzzy {
			b.WriteString(wm.queryWord)
			b.WriteString(" ≈ ")
			b.WriteString(wm.targetWord)
			b.WriteString(" (fuzzy)")
		} else if wm.isStemMatch {
			b.WriteString(wm.queryWord)
			b.WriteString(" ~ ")
			b.WriteString(wm.targetWord)
			b.WriteString(" (stem)")
		} else if wm.isSynonymMatch {
			b.WriteString(wm.queryWord)
			b.WriteString(" ↔ ")
			b.WriteString(wm.targetWord)
			b.WriteString(" (synonym)")
		} else if wm.isAbbrevMatch {
			b.WriteString(wm.queryWord)
			b.WriteString(" = ")
			b.WriteString(wm.targetWord)
			b.WriteString(" (abbrev)")
		}
	}

	return b.String()
}

// MatchMultiple matches a query against multiple targets and returns sorted results
func (pm *PhraseMatcher) MatchMultiple(query string, targets []string) []PhraseMatchResult {
	results := make([]PhraseMatchResult, 0, len(targets))

	for _, target := range targets {
		result := pm.Match(query, target)
		if result.Matched {
			results = append(results, result)
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		// Tiebreaker: prefer exact phrase matches
		if results[i].IsExactPhrase != results[j].IsExactPhrase {
			return results[i].IsExactPhrase
		}
		// Tiebreaker: shorter target name
		return len(results[i].Target) < len(results[j].Target)
	})

	return results
}

// PhraseMatcherDetector adapts PhraseMatcher to the Matcher interface for SemanticScorer
// It detects multi-word phrase matches in symbol names
type PhraseMatcherDetector struct {
	matcher *PhraseMatcher
}

// NewPhraseMatcherDetector creates a new phrase matcher detector
func NewPhraseMatcherDetector(matcher *PhraseMatcher) *PhraseMatcherDetector {
	return &PhraseMatcherDetector{
		matcher: matcher,
	}
}

// Detect implements the Matcher interface
// Returns (matched, score, justification, details)
func (pmd *PhraseMatcherDetector) Detect(query, symbolName, queryLower, symbolLower string, config ScoreLayers) (bool, float64, string, map[string]string) {
	if pmd.matcher == nil {
		return false, 0, "", nil
	}

	// Only use phrase matching for multi-word queries
	// Single word queries are handled by other matchers more efficiently
	if !strings.Contains(strings.TrimSpace(query), " ") {
		return false, 0, "", nil
	}

	// Use the original query and symbolName to preserve case information for splitting
	result := pmd.matcher.Match(query, symbolName)

	if !result.Matched {
		return false, 0, "", nil
	}

	// Convert PhraseMatcher result to Matcher interface format
	// Scale score according to config weights
	score := result.Score * config.PhraseWeight // Use PhraseWeight for phrase matching

	// Build details map without fmt.Sprintf to reduce allocations
	isExactStr := "false"
	if result.IsExactPhrase {
		isExactStr = "true"
	}

	details := map[string]string{
		"query":         query,
		"symbolName":    symbolName,
		"matchedWords":  strings.Join(result.MatchedWords, ", "),
		"isExactPhrase": isExactStr,
	}

	return true, score, result.Justification, details
}

// IsMultiWordQuery returns true if the query contains multiple words (spaces)
func IsMultiWordQuery(query string) bool {
	return strings.Contains(strings.TrimSpace(query), " ")
}
