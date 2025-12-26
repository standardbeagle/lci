package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Domain definitions - declared at package level to avoid repeated allocations
var termClusterDomains = []struct {
	name     string
	keywords []string
}{
	{"Authentication", []string{"auth", "login", "user", "session", "token", "password", "verify"}},
	{"Database", []string{"db", "sql", "model", "repository", "query", "table", "column"}},
	{"API", []string{"api", "handler", "controller", "endpoint", "route", "request", "response"}},
	{"Payment", []string{"payment", "billing", "checkout", "charge", "invoice", "price", "cart"}},
	{"Config", []string{"config", "setting", "option", "parameter", "env", "variable"}},
	{"Service", []string{"service", "manager", "provider", "factory", "builder"}},
	{"Test", []string{"test", "spec", "mock", "fixture", "assert"}},
	{"Cache", []string{"cache", "memo", "store", "session", "cookie"}},
	{"Logger", []string{"log", "debug", "trace", "error", "warn", "info"}},
	{"File", []string{"file", "stream", "read", "write", "save", "load"}},
	{"DateTime", []string{"date", "time", "datetime", "timestamp", "now", "format"}},
	{"Network", []string{"http", "https", "tcp", "udp", "socket", "server", "client"}},
}

const (
	maxTermsPerCluster = 20 // Cap terms shown per cluster
	maxKeyTerms        = 50 // Cap key terms
	maxTopDomains      = 12 // Cap domains (matches termClusterDomains length)
)

// buildTermClusterAnalysis implements simplified term clustering
// Uses existing symbol index (efficient) instead of universal graph
func (s *Server) buildTermClusterAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*TermClusterAnalysis, error) {
	// Get all symbols from the existing symbol index
	allSymbols, err := s.getAllSymbolsFromIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols: %w", err)
	}

	if len(allSymbols) == 0 {
		return nil, errors.New("no symbols found in index")
	}

	// Extract terms from symbol names - estimate 3 terms per symbol
	estimatedTerms := len(allSymbols) * 3
	if estimatedTerms > 10000 {
		estimatedTerms = 10000 // Cap for very large codebases
	}
	termMap := make(map[string]int, estimatedTerms)

	// Reuse slice for extracted terms to reduce allocations
	var termBuf []string
	for _, symbol := range allSymbols {
		termBuf = extractTermsInto(symbol.Name, termBuf[:0])
		for _, term := range termBuf {
			termMap[term]++
		}
	}

	numDomains := len(termClusterDomains)

	// Create clusters by domain - preallocate with known size
	clusters := make([]TermCluster, 0, numDomains)

	// Preallocate cluster terms slice with cap
	clusterTerms := make([]string, 0, maxTermsPerCluster)

	for i := range termClusterDomains {
		domain := &termClusterDomains[i]
		clusterTerms = clusterTerms[:0] // Reset without reallocating

		for term := range termMap {
			for _, keyword := range domain.keywords {
				// Only match if term contains keyword (not reverse)
				// Require keyword to be at least 3 chars to avoid short matches
				if len(keyword) >= 3 && strings.Contains(term, keyword) {
					clusterTerms = append(clusterTerms, term)
					if len(clusterTerms) >= maxTermsPerCluster {
						break
					}
					break
				}
			}
			if len(clusterTerms) >= maxTermsPerCluster {
				break
			}
		}

		if len(clusterTerms) > 0 {
			// Copy to avoid sharing backing array
			termsCopy := make([]string, len(clusterTerms))
			copy(termsCopy, clusterTerms)

			clusters = append(clusters, TermCluster{
				ClusterID:   len(clusters),
				Domain:      domain.name,
				Terms:       termsCopy,
				Centroid:    termsCopy[0],
				Strength:    0.7,
				MemberCount: len(termsCopy),
			})
		}
	}

	// Build domain models - preallocate
	domainModels := make([]DomainModel, 0, numDomains)
	for i := range termClusterDomains {
		domain := &termClusterDomains[i]
		domainModels = append(domainModels, DomainModel{
			Domain:         domain.name,
			Concepts:       []DomainConcept{{Name: domain.name, Terms: []string{domain.name}}},
			Relationships:  nil, // nil instead of empty slice
			VocabularySize: len(domain.keywords),
		})
	}

	// Vocabulary - reuse termMap size
	vocabulary := make(map[string]float64, len(termMap))
	for term, freq := range termMap {
		vocabulary[term] = float64(freq)
	}

	// Key terms - estimate based on terms with freq > 1
	keyTerms := make([]KeyTerm, 0, min(len(termMap)/2, maxKeyTerms))
	for term, freq := range termMap {
		if freq > 1 {
			keyTerms = append(keyTerms, KeyTerm{
				Term:       term,
				Frequency:  freq,
				TFIDFScore: float64(freq),
				Domain:     classifyTermSimple(term),
				Category:   "general",
			})
			if len(keyTerms) >= maxKeyTerms {
				break
			}
		}
	}

	// Top domains - preallocate with known max
	topDomains := make([]DomainSummary, 0, numDomains)
	for i := range termClusterDomains {
		domain := &termClusterDomains[i]
		domainTermCount := 0
		for term := range termMap {
			for _, keyword := range domain.keywords {
				if strings.Contains(term, keyword) {
					domainTermCount++
					break
				}
			}
		}
		if domainTermCount > 0 {
			topDomains = append(topDomains, DomainSummary{
				Domain:         domain.name,
				TermCount:      domainTermCount,
				ConceptCount:   1,
				Confidence:     0.8,
				Representative: domain.name,
			})
		}
	}

	// Calculate average cluster size
	avgClusterSize := 0.0
	if len(clusters) > 0 {
		avgClusterSize = float64(len(termMap)) / float64(len(clusters))
	}

	return &TermClusterAnalysis{
		Clusters:     clusters,
		DomainModels: domainModels,
		Vocabulary:   vocabulary,
		KeyTerms:     keyTerms,
		TopDomains:   topDomains,
		Metrics: TermClusterMetrics{
			TotalClusters:      len(clusters),
			AverageClusterSize: avgClusterSize,
			Coverage:           0.6,
			Quality:            0.7,
		},
	}, nil
}

// extractTerms extracts terms from a symbol name by splitting on camelCase and snake_case
// Handles acronyms properly: "parseHTTPResponse" -> ["parse", "http", "response"]
func extractTerms(name string) []string {
	// Estimate capacity: typical symbol has 2-4 terms
	terms := make([]string, 0, 4)
	return extractTermsInto(name, terms)
}

// extractTermsInto extracts terms into a provided slice to reduce allocations
// The slice is appended to and returned (may be reallocated if capacity exceeded)
func extractTermsInto(name string, terms []string) []string {
	if len(name) == 0 {
		return terms
	}

	// First split on underscores, hyphens, and dots
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})

	// Reuse a single builder across all parts
	var current strings.Builder
	current.Grow(32) // Preallocate for typical word length

	for _, part := range parts {
		// Split camelCase with proper acronym handling
		// "getUserName" -> ["get", "User", "Name"]
		// "parseHTTPResponse" -> ["parse", "HTTP", "Response"]
		// "URLParam" -> ["URL", "Param"]
		runes := []rune(part)

		for i := 0; i < len(runes); i++ {
			r := runes[i]
			isUpper := r >= 'A' && r <= 'Z'

			if i == 0 {
				current.WriteRune(r)
				continue
			}

			prevUpper := runes[i-1] >= 'A' && runes[i-1] <= 'Z'

			// Determine if we should start a new word
			startNewWord := false

			if isUpper && !prevUpper {
				// Transition from lowercase to uppercase: "get|User"
				startNewWord = true
			} else if isUpper && prevUpper && i+1 < len(runes) {
				// Check if next char is lowercase: "HTT|P|Response" -> split before 'R'
				nextLower := runes[i+1] >= 'a' && runes[i+1] <= 'z'
				if nextLower {
					startNewWord = true
				}
			}

			if startNewWord && current.Len() > 0 {
				word := strings.ToLower(current.String())
				if len(word) >= 3 {
					terms = append(terms, word)
				}
				current.Reset()
			}

			current.WriteRune(r)
		}

		// Emit final word from this part
		if current.Len() > 0 {
			word := strings.ToLower(current.String())
			if len(word) >= 3 {
				terms = append(terms, word)
			}
			current.Reset()
		}
	}

	return terms
}

// classifyDomains is declared at package level to avoid allocations on each call
// Aligned with termClusterDomains for consistency in term classification
var classifyDomains = []struct {
	name     string
	keywords []string
}{
	// Check exact/longer matches first to avoid substring conflicts
	// e.g., "session" should match "auth" not "file" (which has "io")
	{"auth", []string{"authentication", "authorize", "session", "login", "token", "auth", "user", "password", "verify"}},
	{"db", []string{"database", "sql", "model", "data", "query", "repository", "table", "column"}},
	{"api", []string{"http", "request", "response", "api", "endpoint", "handler", "controller", "route"}},
	{"test", []string{"test", "spec", "mock", "assert", "fixture"}},
	{"file", []string{"file", "read", "write", "stream", "save", "load"}},
	{"cache", []string{"cache", "store", "memo"}},
	{"config", []string{"config", "setting", "option", "parameter", "env"}},
	{"service", []string{"service", "manager", "provider", "factory", "builder"}},
	{"log", []string{"log", "debug", "trace", "error", "warn", "info"}},
}

// classifyTermSimple classifies a term to a domain
// Uses ordered domain matching to ensure deterministic results
func classifyTermSimple(term string) string {
	lower := strings.ToLower(term)

	for i := range classifyDomains {
		domain := &classifyDomains[i]
		for _, keyword := range domain.keywords {
			if strings.Contains(lower, keyword) {
				return domain.name
			}
		}
	}

	return "general"
}
