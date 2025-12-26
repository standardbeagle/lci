package semantic

import "fmt"

// MatchType represents the type of semantic match found
type MatchType string

const (
	MatchTypeExact        MatchType = "exact"
	MatchTypeSubstring    MatchType = "substring"
	MatchTypePhrase       MatchType = "phrase" // Multi-word phrase match
	MatchTypeAnnotation   MatchType = "annotation"
	MatchTypeFuzzy        MatchType = "fuzzy"
	MatchTypeStemming     MatchType = "stemming"
	MatchTypeAbbreviation MatchType = "abbreviation"
	MatchTypeNameSplit    MatchType = "name_split"
	MatchTypeNone         MatchType = "no_match"
)

// SemanticScore represents the scoring result for a single symbol match
type SemanticScore struct {
	// Final combined score (0.0-1.0)
	Score float64

	// Type of match that achieved the highest score
	QueryMatch MatchType

	// Confidence in the match (0.0-1.0)
	Confidence float64

	// Human-readable explanation of why this match occurred
	Justification string

	// Layer-specific details for debugging and transparency
	MatchDetails map[string]string
}

// ScoredSymbol represents a symbol with its computed semantic score
type ScoredSymbol struct {
	// The symbol being scored
	Symbol interface{} // Typically a *core.Symbol or similar

	// The semantic score for this symbol
	Score SemanticScore

	// Rank in the result set (1-based, 1 = highest score)
	Rank int
}

// SearchResult represents the complete result of a semantic search
type SearchResult struct {
	// Query that was executed
	Query string

	// Symbols sorted by score descending
	Symbols []ScoredSymbol

	// Total number of candidates before filtering
	CandidatesConsidered int

	// Number of results after min score filtering
	ResultsReturned int

	// Time to execute search (nanoseconds)
	ExecutionTime int64
}

// ScoreLayers contains configuration for each scoring layer
type ScoreLayers struct {
	// Individual layer weights
	ExactWeight        float64
	SubstringWeight    float64 // New: substring containment matching
	PhraseWeight       float64 // Multi-word phrase matching
	AnnotationWeight   float64
	FuzzyWeight        float64
	StemmingWeight     float64
	AbbreviationWeight float64
	NameSplitWeight    float64

	// Matching thresholds
	FuzzyThreshold float64
	StemMinLength  int

	// Result configuration
	MaxResults int
	MinScore   float64
}

// DefaultScoreLayers provides reasonable defaults for semantic scoring
var DefaultScoreLayers = ScoreLayers{
	// Weights reflect match quality with WIDE spread for clear hierarchy
	ExactWeight:        1.0,  // Layer 1: Exact matches (query == symbol)
	SubstringWeight:    0.9,  // Layer 2: Substring containment (query IN symbol)
	PhraseWeight:       0.88, // Layer 3: Multi-word phrase matching
	AnnotationWeight:   0.85, // Layer 4: Developer intent is highly reliable
	FuzzyWeight:        0.70, // Layer 5: Typos are common, good signal
	StemmingWeight:     0.55, // Layer 6: Word forms are informative
	NameSplitWeight:    0.40, // Layer 7: camelCase/snake_case splitting
	AbbreviationWeight: 0.25, // Layer 8: Abbreviations (gui→user, udp→user)

	// Fuzzy matching threshold
	FuzzyThreshold: 0.7,

	// Minimum word length to stem
	StemMinLength: 3,

	// Result limits
	MaxResults: 10,
	MinScore:   0.2, // Lower threshold to include abbreviation matches
}

// String returns a human-readable representation of a SemanticScore
func (s SemanticScore) String() string {
	return fmt.Sprintf("SemanticScore{Score: %.3f, Match: %s, Confidence: %.3f}",
		s.Score, s.QueryMatch, s.Confidence)
}

// IsValidScore checks if a score is within acceptable bounds
func (s SemanticScore) IsValidScore() bool {
	return s.Score >= 0.0 && s.Score <= 1.0 &&
		s.Confidence >= 0.0 && s.Confidence <= 1.0
}

// String returns a human-readable representation of a ScoredSymbol
func (ss ScoredSymbol) String() string {
	return fmt.Sprintf("ScoredSymbol{Rank: %d, Score: %.3f}",
		ss.Rank, ss.Score.Score)
}

// String returns a human-readable summary of SearchResult
func (sr SearchResult) String() string {
	return fmt.Sprintf("SearchResult{Query: %q, Results: %d/%d candidates, Time: %dμs}",
		sr.Query, sr.ResultsReturned, sr.CandidatesConsidered,
		sr.ExecutionTime/1000)
}

// DebugString returns detailed information about a score for debugging
func (s SemanticScore) DebugString() string {
	details := fmt.Sprintf("Score: %.3f, Match: %s, Confidence: %.3f\nJustification: %s\n",
		s.Score, s.QueryMatch, s.Confidence, s.Justification)

	if len(s.MatchDetails) > 0 {
		details += "Details:\n"
		for key, value := range s.MatchDetails {
			details += fmt.Sprintf("  %s: %s\n", key, value)
		}
	}

	return details
}
