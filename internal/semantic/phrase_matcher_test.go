package semantic

import (
	"testing"
)

// TestPhraseMatcher_ExactPhraseMatch tests that exact phrase matches rank highest
func TestPhraseMatcher_ExactPhraseMatch(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	tests := []struct {
		name      string
		query     string
		target    string
		wantMatch bool
		wantExact bool
		minScore  float64
	}{
		{
			name:      "exact phrase in target",
			query:     "HTTP client",
			target:    "HTTPClient",
			wantMatch: true,
			wantExact: true,
			minScore:  0.90, // Exact phrase should score high
		},
		{
			name:      "exact phrase with underscore",
			query:     "http client",
			target:    "http_client",
			wantMatch: true,
			wantExact: true,
			minScore:  0.90,
		},
		{
			name:      "words present but out of order",
			query:     "client http",
			target:    "HTTPClient",
			wantMatch: true,
			wantExact: false, // Not exact because order differs
			minScore:  0.70,
		},
		{
			name:      "partial word match",
			query:     "HTTP request",
			target:    "HTTPClientRequest",
			wantMatch: true,
			wantExact: true, // Both words present in order
			minScore:  0.85,
		},
		{
			name:      "no match",
			query:     "database query",
			target:    "HTTPClient",
			wantMatch: false,
			wantExact: false,
			minScore:  0,
		},
		{
			name:      "single word query",
			query:     "client",
			target:    "HTTPClient",
			wantMatch: true,
			wantExact: true,
			minScore:  0.80,
		},
		{
			name:      "fuzzy phrase match",
			query:     "HTTP clent", // typo in "client"
			target:    "HTTPClient",
			wantMatch: true,
			wantExact: false,
			minScore:  0.70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)

			if result.Matched != tt.wantMatch {
				t.Errorf("Match() matched = %v, want %v", result.Matched, tt.wantMatch)
			}

			if tt.wantMatch {
				if result.IsExactPhrase != tt.wantExact {
					t.Errorf("Match() isExactPhrase = %v, want %v", result.IsExactPhrase, tt.wantExact)
				}

				if result.Score < tt.minScore {
					t.Errorf("Match() score = %v, want >= %v", result.Score, tt.minScore)
				}
			}
		})
	}
}

// TestPhraseMatcher_MultiWordRanking tests that multi-word queries rank results appropriately
func TestPhraseMatcher_MultiWordRanking(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	query := "HTTP client"
	targets := []string{
		"HTTPClient",        // Exact phrase match
		"HttpClientRequest", // Contains phrase with extra word
		"ClientHTTP",        // Words out of order
		"HTTPConnection",    // Only one word matches
		"DatabaseClient",    // Only one word matches (different one)
		"SomethingElse",     // No match
	}

	results := pm.MatchMultiple(query, targets)

	// Verify ordering: exact phrase > partial phrase > single word match > no match
	if len(results) < 2 {
		t.Fatalf("Expected at least 2 results, got %d", len(results))
	}

	// HTTPClient should rank first (exact phrase)
	if results[0].Target != "HTTPClient" {
		t.Errorf("Expected HTTPClient to rank first, got %s", results[0].Target)
	}

	// Verify scores are in descending order
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("Results not in descending score order: %s (%.2f) > %s (%.2f)",
				results[i].Target, results[i].Score,
				results[i-1].Target, results[i-1].Score)
		}
	}
}

// TestPhraseMatcher_EmptyInput tests edge cases with empty input
func TestPhraseMatcher_EmptyInput(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	tests := []struct {
		name   string
		query  string
		target string
	}{
		{"empty query", "", "HTTPClient"},
		{"empty target", "HTTP client", ""},
		{"both empty", "", ""},
		{"whitespace query", "   ", "HTTPClient"},
		{"whitespace target", "HTTP client", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)
			if result.Matched {
				t.Errorf("Match() matched = true for empty/whitespace input, want false")
			}
		})
	}
}

// TestPhraseMatcher_CaseInsensitive tests case-insensitive matching
func TestPhraseMatcher_CaseInsensitive(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	tests := []struct {
		name   string
		query  string
		target string
	}{
		{"lowercase query, PascalCase target", "http client", "HTTPClient"},
		{"UPPERCASE query, lowercase target", "HTTP CLIENT", "httpclient"},
		{"mixed case query", "Http ClIeNt", "HTTPClient"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)
			if !result.Matched {
				t.Errorf("Match() matched = false, want true for case-insensitive match")
			}
		})
	}
}

// TestPhraseMatcher_RealWorldQueries tests queries from the original issue
func TestPhraseMatcher_RealWorldQueries(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	// These are the queries that failed in the original issue
	tests := []struct {
		name      string
		query     string
		target    string
		wantMatch bool
	}{
		{
			name:      "HTTP client query matches HTTPClient",
			query:     "HTTP client",
			target:    "HTTPClient",
			wantMatch: true,
		},
		{
			name:      "http request matches HttpRequest",
			query:     "http request",
			target:    "HttpRequest",
			wantMatch: true,
		},
		{
			name:      "error handling matches ErrorHandler",
			query:     "error handling",
			target:    "ErrorHandler",
			wantMatch: true, // Should match due to stemming (handling -> handle)
		},
		{
			name:      "API error matches ApiError",
			query:     "API error",
			target:    "ApiError",
			wantMatch: true,
		},
		{
			name:      "URL parameter matches URLParam",
			query:     "URL parameter",
			target:    "URLParam",
			wantMatch: true, // Should match due to abbreviation (parameter -> param)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)
			if result.Matched != tt.wantMatch {
				t.Errorf("Match(%q, %q) matched = %v, want %v",
					tt.query, tt.target, result.Matched, tt.wantMatch)
			}
		})
	}
}

// TestPhraseMatcher_FuzzyWordMatch tests fuzzy matching on individual words
func TestPhraseMatcher_FuzzyWordMatch(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	tests := []struct {
		name      string
		query     string
		target    string
		wantMatch bool
		desc      string
	}{
		{
			name:      "typo in first word",
			query:     "HTPP client",
			target:    "HTTPClient",
			wantMatch: true,
			desc:      "HTPP should fuzzy match HTTP",
		},
		{
			name:      "typo in second word",
			query:     "HTTP clinet",
			target:    "HTTPClient",
			wantMatch: true,
			desc:      "clinet should fuzzy match client",
		},
		{
			name:      "both words have typos",
			query:     "HTPP clinet",
			target:    "HTTPClient",
			wantMatch: true,
			desc:      "Both words should fuzzy match",
		},
		{
			name:      "completely different words",
			query:     "database query",
			target:    "HTTPClient",
			wantMatch: false,
			desc:      "No fuzzy match possible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)
			if result.Matched != tt.wantMatch {
				t.Errorf("Match(%q, %q) matched = %v, want %v (%s)",
					tt.query, tt.target, result.Matched, tt.wantMatch, tt.desc)
			}
		})
	}
}

// TestPhraseMatcher_WordOrderPreference tests that in-order matches score higher
func TestPhraseMatcher_WordOrderPreference(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	query := "HTTP client"

	// In-order match should score higher than out-of-order
	inOrderResult := pm.Match(query, "HTTPClient")
	outOfOrderResult := pm.Match(query, "ClientHTTP")

	if !inOrderResult.Matched || !outOfOrderResult.Matched {
		t.Fatal("Expected both to match")
	}

	if inOrderResult.Score <= outOfOrderResult.Score {
		t.Errorf("In-order match should score higher: in-order=%.2f, out-of-order=%.2f",
			inOrderResult.Score, outOfOrderResult.Score)
	}
}

// BenchmarkPhraseMatcher benchmarks phrase matching performance
func BenchmarkPhraseMatcher(b *testing.B) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	query := "HTTP client request"
	target := "HTTPClientRequest"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.Match(query, target)
	}
}

// TestPhraseMatcher_Porter2Stemming tests that full Porter2 stemming works when enabled
func TestPhraseMatcher_Porter2Stemming(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	stemmer := NewStemmer(true, "porter2", 3, nil)

	// Create phrase matcher with full Porter2 stemmer (nil dictionary for stemming-only test)
	pm := NewPhraseMatcherFull(splitter, fuzzer, stemmer, nil)

	tests := []struct {
		name      string
		query     string
		target    string
		wantMatch bool
		desc      string
	}{
		{
			name:      "running matches run (Porter2)",
			query:     "running tests",
			target:    "runTests",
			wantMatch: true,
			desc:      "Porter2 stems 'running' to 'run'",
		},
		{
			name:      "authentication matches authenticate (Porter2)",
			query:     "user authentication",
			target:    "authenticateUser",
			wantMatch: true,
			desc:      "Porter2 stems 'authentication' to 'authent'",
		},
		{
			name:      "processing matches process (Porter2)",
			query:     "data processing",
			target:    "processData",
			wantMatch: true,
			desc:      "Porter2 stems 'processing' to 'process'",
		},
		{
			name:      "validation matches validate (Porter2)",
			query:     "input validation",
			target:    "validateInput",
			wantMatch: true,
			desc:      "Porter2 stems 'validation' to 'valid'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)
			if result.Matched != tt.wantMatch {
				t.Errorf("Match(%q, %q) matched = %v, want %v (%s)",
					tt.query, tt.target, result.Matched, tt.wantMatch, tt.desc)
			}
			if tt.wantMatch && result.Matched {
				t.Logf("Match(%q, %q): score=%.3f, justification=%s",
					tt.query, tt.target, result.Score, result.Justification)
			}
		})
	}
}

// TestPhraseMatcher_StemmerFallback tests that simplified stemming works when stemmer is nil
func TestPhraseMatcher_StemmerFallback(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")

	// Create phrase matcher WITHOUT stemmer (uses simplified fallback)
	pm := NewPhraseMatcher(splitter, fuzzer)

	// "handling" -> "handl" via simplified stemmer (removes "ing")
	result := pm.Match("error handling", "handleError")

	if !result.Matched {
		t.Errorf("Expected simplified stemmer to match 'handling' -> 'handle'")
	}
	t.Logf("Simplified stem match: score=%.3f, justification=%s",
		result.Score, result.Justification)
}

// TestPhraseMatcher_SynonymMatching tests dictionary synonym expansion (signin ↔ login)
func TestPhraseMatcher_SynonymMatching(t *testing.T) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	stemmer := NewStemmer(true, "porter2", 3, nil)
	dict := DefaultTranslationDictionary()

	// Create phrase matcher with full semantic features
	pm := NewPhraseMatcherFull(splitter, fuzzer, stemmer, dict)

	tests := []struct {
		name      string
		query     string
		target    string
		wantMatch bool
		desc      string
	}{
		{
			name:      "signin matches login (synonym)",
			query:     "user signin",
			target:    "userLogin",
			wantMatch: true,
			desc:      "signin and login are synonyms in authentication domain",
		},
		{
			name:      "login matches signin (synonym)",
			query:     "user login",
			target:    "userSignin",
			wantMatch: true,
			desc:      "login and signin are synonyms (bidirectional)",
		},
		{
			name:      "auth matches authenticate (abbreviation)",
			query:     "user auth",
			target:    "authenticateUser",
			wantMatch: true,
			desc:      "auth is an abbreviation for authenticate",
		},
		{
			name:      "store matches persist (synonym)",
			query:     "data store",
			target:    "persistData",
			wantMatch: true,
			desc:      "store and persist are synonyms in persistence domain",
		},
		{
			name:      "verify matches authenticate (synonym)",
			query:     "user verify",
			target:    "authenticateUser",
			wantMatch: true,
			desc:      "verify and authenticate are in same domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.Match(tt.query, tt.target)
			if result.Matched != tt.wantMatch {
				t.Errorf("Match(%q, %q) matched = %v, want %v (%s)",
					tt.query, tt.target, result.Matched, tt.wantMatch, tt.desc)
			}
			if tt.wantMatch && result.Matched {
				t.Logf("Match(%q, %q): score=%.3f, justification=%s",
					tt.query, tt.target, result.Score, result.Justification)
			}
		})
	}
}

// BenchmarkPhraseMatcher_MultipleTargets benchmarks matching against multiple targets
func BenchmarkPhraseMatcher_MultipleTargets(b *testing.B) {
	splitter := NewNameSplitter()
	fuzzer := NewFuzzyMatcher(true, 0.80, "jaro-winkler")
	pm := NewPhraseMatcher(splitter, fuzzer)

	query := "HTTP client"
	targets := []string{
		"HTTPClient",
		"HttpClientRequest",
		"ClientHTTP",
		"HTTPConnection",
		"DatabaseClient",
		"SomethingElse",
		"AsyncHTTPClient",
		"HTTPClientFactory",
		"SimpleClient",
		"HTTPTransport",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.MatchMultiple(query, targets)
	}
}
