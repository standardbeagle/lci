package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"
)

// ============================================================================
// Domain Vocabulary Analysis
// ============================================================================

// Domain classification patterns with match strength
var domainPatterns = map[string]struct {
	keywords     []string
	exactWeight  float64 // Weight for exact matches
	prefixWeight float64 // Weight for prefix/suffix matches
}{
	"Authentication": {
		keywords:     []string{"auth", "login", "logout", "user", "password", "token", "session", "oauth", "jwt", "credential"},
		exactWeight:  1.0,
		prefixWeight: 0.7,
	},
	"Database": {
		keywords:     []string{"db", "database", "query", "sql", "table", "schema", "migrate", "transaction", "cursor"},
		exactWeight:  1.0,
		prefixWeight: 0.8,
	},
	"HTTP/API": {
		keywords:     []string{"http", "api", "rest", "endpoint", "request", "response", "handler", "route", "server", "client"},
		exactWeight:  1.0,
		prefixWeight: 0.8,
	},
	"Parsing": {
		keywords:     []string{"parse", "parser", "lexer", "token", "ast", "syntax", "grammar", "node"},
		exactWeight:  1.0,
		prefixWeight: 0.75,
	},
	"Testing": {
		keywords:     []string{"test", "mock", "stub", "assert", "expect", "benchmark", "fixture"},
		exactWeight:  1.0,
		prefixWeight: 0.85,
	},
	"Indexing": {
		keywords:     []string{"index", "search", "trigram", "symbol", "reference", "cache"},
		exactWeight:  1.0,
		prefixWeight: 0.7,
	},
	"Configuration": {
		keywords:     []string{"config", "setting", "option", "env", "parameter", "flag"},
		exactWeight:  1.0,
		prefixWeight: 0.8,
	},
	"Error Handling": {
		keywords:     []string{"error", "err", "exception", "panic", "recover", "fail", "invalid"},
		exactWeight:  1.0,
		prefixWeight: 0.65,
	},
	"Concurrency": {
		keywords:     []string{"goroutine", "channel", "mutex", "lock", "sync", "async", "concurrent", "parallel", "worker"},
		exactWeight:  1.0,
		prefixWeight: 0.85,
	},
}

// extractTermsFromName extracts terms from a symbol name
func (s *Server) extractTermsFromName(name string) []string {
	terms := make([]string, 0)
	if name != "" {
		terms = append(terms, name)
	}
	return terms
}

// classifyTermDomainWithStrength returns domain and match strength (0.0-1.0)
func (s *Server) classifyTermDomainWithStrength(term string) (domain string, strength float64) {
	termLower := strings.ToLower(term)
	bestDomain := ""
	bestStrength := 0.0

	for d, pattern := range domainPatterns {
		for _, keyword := range pattern.keywords {
			if termLower == keyword {
				if pattern.exactWeight > bestStrength ||
					(pattern.exactWeight == bestStrength && d < bestDomain) {
					bestDomain = d
					bestStrength = pattern.exactWeight
				}
			} else if strings.Contains(termLower, keyword) {
				matchStrength := pattern.prefixWeight
				if strings.HasPrefix(termLower, keyword) || strings.HasSuffix(termLower, keyword) {
					matchStrength = pattern.prefixWeight + (pattern.exactWeight-pattern.prefixWeight)*0.3
				}
				if matchStrength > bestStrength ||
					(matchStrength == bestStrength && d < bestDomain) {
					bestDomain = d
					bestStrength = matchStrength
				}
			}
		}
	}

	return bestDomain, bestStrength
}

// classifyTermDomain is a simplified wrapper for backward compatibility
func (s *Server) classifyTermDomain(term string) string {
	domain, _ := s.classifyTermDomainWithStrength(term)
	return domain
}

// calculateDomainConfidence calculates confidence based on term frequency, count, and match strength
func (s *Server) calculateDomainConfidence(matchStrength float64, termCount int, totalFrequency int, totalTerms int) float64 {
	confidence := matchStrength * 0.4

	termCountFactor := 0.0
	if termCount > 0 {
		termCountFactor = math.Min(1.0, math.Log10(float64(termCount)+1)/math.Log10(11))
	}
	confidence += termCountFactor * 0.25

	freqFactor := 0.0
	if totalFrequency > 0 {
		freqFactor = math.Min(1.0, math.Log10(float64(totalFrequency)+1)/math.Log10(101))
	}
	confidence += freqFactor * 0.2

	specificityFactor := 0.0
	if totalTerms > 0 {
		ratio := float64(termCount) / float64(totalTerms)
		specificityFactor = math.Min(1.0, ratio*10)
	}
	confidence += specificityFactor * 0.15

	if confidence < 0.1 {
		confidence = 0.1
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// extractDomainTermsFromSymbols extracts domain terms from symbol data
func (s *Server) extractDomainTermsFromSymbols(allFiles []*types.FileInfo, args *CodebaseIntelligenceParams) []DomainTerm {
	termFrequency := make(map[string]int)

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			terms := s.extractTermsFromName(sym.Name)
			for _, term := range terms {
				termFrequency[term]++
			}
		}
	}

	type domainTermInfo struct {
		terms         []string
		totalFreq     int
		avgStrength   float64
		strengthCount int
	}
	domainInfo := make(map[string]*domainTermInfo)

	for term, freq := range termFrequency {
		domain, strength := s.classifyTermDomainWithStrength(term)
		if domain != "" {
			if domainInfo[domain] == nil {
				domainInfo[domain] = &domainTermInfo{terms: make([]string, 0)}
			}
			domainInfo[domain].terms = append(domainInfo[domain].terms, term)
			domainInfo[domain].totalFreq += freq
			domainInfo[domain].avgStrength += strength
			domainInfo[domain].strengthCount++
		}
	}

	totalTerms := len(termFrequency)
	domainTerms := make([]DomainTerm, 0)
	for domain, info := range domainInfo {
		if len(info.terms) > 0 {
			avgStrength := info.avgStrength / float64(info.strengthCount)
			confidence := s.calculateDomainConfidence(avgStrength, len(info.terms), info.totalFreq, totalTerms)

			domainTerms = append(domainTerms, DomainTerm{
				Domain:     domain,
				Terms:      info.terms,
				Confidence: confidence,
				Count:      len(info.terms),
			})
		}
	}

	return domainTerms
}

// extractDomainTermsFromMetrics extracts domain-specific terms from precomputed metrics
func (s *Server) extractDomainTermsFromMetrics(allMetrics map[types.FileID]map[string]interface{}, args *CodebaseIntelligenceParams) []DomainTerm {
	termFrequency := make(map[string]int)

	for _, fileMetrics := range allMetrics {
		for symbolName := range fileMetrics {
			terms := s.extractTermsFromName(symbolName)
			for _, term := range terms {
				termFrequency[term]++
			}
		}
	}

	type domainTermInfo struct {
		terms         []string
		totalFreq     int
		avgStrength   float64
		strengthCount int
	}
	domainInfo := make(map[string]*domainTermInfo)

	for term, freq := range termFrequency {
		domain, strength := s.classifyTermDomainWithStrength(term)
		if domain != "" {
			if domainInfo[domain] == nil {
				domainInfo[domain] = &domainTermInfo{terms: make([]string, 0)}
			}
			domainInfo[domain].terms = append(domainInfo[domain].terms, term)
			domainInfo[domain].totalFreq += freq
			domainInfo[domain].avgStrength += strength
			domainInfo[domain].strengthCount++
		}
	}

	totalTerms := len(termFrequency)
	domainTerms := make([]DomainTerm, 0)
	for domain, info := range domainInfo {
		if len(info.terms) > 0 {
			avgStrength := info.avgStrength / float64(info.strengthCount)
			confidence := s.calculateDomainConfidence(avgStrength, len(info.terms), info.totalFreq, totalTerms)

			domainTerms = append(domainTerms, DomainTerm{
				Domain:     domain,
				Terms:      info.terms,
				Confidence: confidence,
				Count:      len(info.terms),
			})
		}
	}

	return domainTerms
}

// extractDomainTerms extracts domain-specific terms from universal symbol nodes
func (s *Server) extractDomainTerms(allSymbols []*types.UniversalSymbolNode, args *CodebaseIntelligenceParams) []DomainTerm {
	termFrequency := make(map[string]int)

	for _, node := range allSymbols {
		terms := s.extractTermsFromName(node.Identity.Name)
		for _, term := range terms {
			termFrequency[term]++
		}
	}

	type domainTermInfo struct {
		terms         []string
		totalFreq     int
		avgStrength   float64
		strengthCount int
	}
	domainInfo := make(map[string]*domainTermInfo)

	for term, freq := range termFrequency {
		domain, strength := s.classifyTermDomainWithStrength(term)
		if domain != "" {
			if domainInfo[domain] == nil {
				domainInfo[domain] = &domainTermInfo{terms: make([]string, 0)}
			}
			domainInfo[domain].terms = append(domainInfo[domain].terms, term)
			domainInfo[domain].totalFreq += freq
			domainInfo[domain].avgStrength += strength
			domainInfo[domain].strengthCount++
		}
	}

	totalTerms := len(termFrequency)
	domainTerms := make([]DomainTerm, 0)
	for domain, info := range domainInfo {
		if len(info.terms) > 0 {
			avgStrength := info.avgStrength / float64(info.strengthCount)
			confidence := s.calculateDomainConfidence(avgStrength, len(info.terms), info.totalFreq, totalTerms)

			domainTerms = append(domainTerms, DomainTerm{
				Domain:     domain,
				Terms:      info.terms,
				Confidence: confidence,
				Count:      len(info.terms),
			})
		}
	}

	return domainTerms
}

// buildSemanticVocabulary builds semantic vocabulary analysis
func (s *Server) buildSemanticVocabulary(
	ctx context.Context,
	args CodebaseIntelligenceParams,
	maxResults int,
) (*SemanticVocabulary, error) {
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(allFiles) == 0 {
		return &SemanticVocabulary{
			DomainsPresent: []SemanticDomain{},
			DomainsAbsent:  []string{},
			UniqueTerms:    []SemanticTerm{},
			CommonTerms:    []SemanticTerm{},
			AnalysisScope:  VocabularyScope{},
			VocabularySize: 0,
		}, nil
	}

	symbols := convertFilesToSymbols(allFiles)
	config := createProjectConfig(&args)

	dict := semantic.DefaultTranslationDictionary()
	splitter := semantic.NewNameSplitter()

	analysis, err := semantic.AnalyzeCodebaseVocabulary(symbols, config, dict, splitter)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze vocabulary: %w", err)
	}

	return convertVocabularyAnalysis(analysis), nil
}

// convertFilesToSymbols converts FileInfo to FileSymbol for semantic analysis
func convertFilesToSymbols(allFiles []*types.FileInfo) []semantic.FileSymbol {
	symbols := make([]semantic.FileSymbol, 0, len(allFiles)*10)

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			symbols = append(symbols, semantic.FileSymbol{
				FilePath:   file.Path,
				Name:       sym.Name,
				Type:       sym.Type.String(),
				IsExported: sym.IsExported,
			})
		}
	}

	return symbols
}

// createProjectConfig creates ProjectConfig from CodebaseIntelligenceParams
func createProjectConfig(args *CodebaseIntelligenceParams) semantic.ProjectConfig {
	return semantic.ProjectConfig{
		Language: "go",
		SourceDirs: []string{
			"cmd/",
			"internal/",
			"pkg/",
			"lib/",
		},
		TestMarkers: []string{
			"test",
			"spec",
			"_test",
			"Test",
			"Spec",
		},
		ExcludeDirs: []string{
			"vendor",
			"node_modules",
			".git",
			"workflow_testdata",
		},
	}
}

// convertVocabularyAnalysis converts semantic.VocabularyAnalysis to mcp.SemanticVocabulary
func convertVocabularyAnalysis(analysis *semantic.VocabularyAnalysis) *SemanticVocabulary {
	domainsPresent := make([]SemanticDomain, 0, len(analysis.DomainsPresent))
	for _, domain := range analysis.DomainsPresent {
		domainsPresent = append(domainsPresent, SemanticDomain{
			Name:           domain.Name,
			Count:          domain.Count,
			Confidence:     domain.Confidence,
			ExampleSymbols: domain.ExampleSymbols,
			MatchedTerms:   domain.MatchedTerms,
		})
	}

	uniqueTerms := make([]SemanticTerm, 0, len(analysis.UniqueTerms))
	for _, term := range analysis.UniqueTerms {
		uniqueTerms = append(uniqueTerms, SemanticTerm{
			Term:           term.Term,
			Count:          term.Count,
			ExampleSymbols: term.ExampleSymbols,
			Domains:        term.Domains,
		})
	}

	commonTerms := make([]SemanticTerm, 0, len(analysis.CommonTerms))
	for _, term := range analysis.CommonTerms {
		commonTerms = append(commonTerms, SemanticTerm{
			Term:           term.Term,
			Count:          term.Count,
			ExampleSymbols: term.ExampleSymbols,
		})
	}

	scope := VocabularyScope{
		TotalFiles:        analysis.AnalysisScope.TotalFiles,
		ProductionFiles:   analysis.AnalysisScope.ProductionFiles,
		TestFilesExcluded: analysis.AnalysisScope.TestFilesExcluded,
		SourceDirectories: analysis.AnalysisScope.SourceDirectories,
		TotalSymbols:      analysis.AnalysisScope.TotalSymbols,
		TotalFunctions:    analysis.AnalysisScope.TotalFunctions,
		TotalVariables:    analysis.AnalysisScope.TotalVariables,
		TotalTypes:        analysis.AnalysisScope.TotalTypes,
	}

	return &SemanticVocabulary{
		DomainsPresent: domainsPresent,
		DomainsAbsent:  analysis.DomainsAbsent,
		UniqueTerms:    uniqueTerms,
		CommonTerms:    commonTerms,
		AnalysisScope:  scope,
		VocabularySize: analysis.VocabularySize,
	}
}
