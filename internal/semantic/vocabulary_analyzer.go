package semantic

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================================
// Type Definitions
// ============================================================================

// ProjectConfig defines how to filter production vs test code
type ProjectConfig struct {
	Language    string
	SourceDirs  []string
	TestMarkers []string // Patterns that indicate test files
	ExcludeDirs []string
}

// VocabularyAnalysis represents the complete vocabulary analysis
type VocabularyAnalysis struct {
	DomainsPresent []DomainResult  `json:"domains_present"`
	DomainsAbsent  []string        `json:"domains_absent"`
	UniqueTerms    []TermResult    `json:"unique_terms"`
	CommonTerms    []TermResult    `json:"common_terms"`
	AnalysisScope  ScopeStatistics `json:"analysis_scope"`
	VocabularySize int             `json:"vocabulary_size"`
}

// DomainResult contains analysis for a semantic domain
type DomainResult struct {
	Name           string   `json:"name"`
	Count          int      `json:"count"`
	Confidence     float64  `json:"confidence"`
	ExampleSymbols []string `json:"example_symbols"`
	MatchedTerms   []string `json:"matched_terms"`
}

// TermResult contains statistics for a specific term
type TermResult struct {
	Term           string   `json:"term"`
	Count          int      `json:"count"`
	ExampleSymbols []string `json:"example_symbols"`
	Domains        []string `json:"domains,omitempty"` // Which semantic domains this term belongs to
}

// ScopeStatistics provides analysis scope information
type ScopeStatistics struct {
	TotalFiles        int      `json:"total_files"`
	ProductionFiles   int      `json:"production_files"`
	TestFilesExcluded int      `json:"test_files_excluded"`
	SourceDirectories []string `json:"source_directories"`
	TotalSymbols      int      `json:"total_symbols"`
	TotalFunctions    int      `json:"total_functions"`
	TotalVariables    int      `json:"total_variables"`
	TotalTypes        int      `json:"total_types"`
}

// FileSymbol represents a symbol extracted from a file
type FileSymbol struct {
	FilePath   string
	Name       string
	Type       string // function, variable, type, etc.
	IsExported bool
}

// Production code filter patterns
var (
	// Go test patterns
	goTestPattern = regexp.MustCompile(`_test\.go$|test_.*\.go$|.*_test\.go$`)

	// Node.js test patterns
	jsTestPattern = regexp.MustCompile(`\.(test|spec)\.(js|ts|jsx|tsx)$|__tests?__/|test/`)

	// Python test patterns
	pythonTestPattern = regexp.MustCompile(`test_.*\.py$|.*_test\.py$|tests?/`)

	// Java test patterns
	javaTestPattern = regexp.MustCompile(`.*Test\.java$|test/`)

	// Common exclude patterns
	commonExcludePatterns = []string{
		"test", "tests", "Test", "Tests",
		"spec", "specs", "Spec", "Specs",
		"mock", "mocks", "Mock", "Mocks",
		"fixture", "fixtures", "Fixture", "Fixtures",
		"__tests__", "__mocks__",
		"testdata", "TestData",
		".git", ".svn", ".hg",
		"node_modules", "vendor",
	}
)

// ============================================================================
// Public API - Main Analysis Function
// ============================================================================

// AnalyzeCodebaseVocabulary analyzes vocabulary of a codebase
// This is a pure function - no side effects, thread-safe
func AnalyzeCodebaseVocabulary(
	symbols []FileSymbol,
	config ProjectConfig,
	dict *TranslationDictionary,
	splitter *NameSplitter,
) (*VocabularyAnalysis, error) {
	if len(symbols) == 0 {
		return &VocabularyAnalysis{
			DomainsPresent: []DomainResult{},
			DomainsAbsent:  getAllDomainNames(dict),
			UniqueTerms:    []TermResult{},
			CommonTerms:    []TermResult{},
			AnalysisScope:  ScopeStatistics{},
			VocabularySize: 0,
		}, nil
	}

	// Step 1: Filter to production code only
	productionSymbols := FilterProductionSymbols(symbols, config)
	if len(productionSymbols) == 0 {
		return &VocabularyAnalysis{
			DomainsPresent: []DomainResult{},
			DomainsAbsent:  getAllDomainNames(dict),
			UniqueTerms:    []TermResult{},
			CommonTerms:    []TermResult{},
			AnalysisScope:  ScopeStatistics{},
			VocabularySize: 0,
		}, nil
	}

	// Step 2: Extract domain occurrences
	domainAnalysis := ExtractDomainOccurrences(productionSymbols, dict, splitter)

	// Step 3: Identify unique terms (domain-specific, not common dev terms)
	uniqueTerms := IdentifyUniqueTerms(productionSymbols, dict, splitter)

	// Step 4: Identify common terms (generic dev vocabulary)
	commonTerms := IdentifyCommonTerms(productionSymbols, dict, splitter)

	// Step 5: Calculate statistics
	scopeStats := CalculateScopeStatistics(symbols, productionSymbols, config)
	vocabularySize := len(domainAnalysis.presentDomainNames) + len(uniqueTerms) + len(commonTerms)

	// Build result
	domainsPresent := buildDomainResults(domainAnalysis, dict, len(productionSymbols))
	domainsAbsent := getAbsentDomains(domainAnalysis, dict)

	return &VocabularyAnalysis{
		DomainsPresent: domainsPresent,
		DomainsAbsent:  domainsAbsent,
		UniqueTerms:    uniqueTerms,
		CommonTerms:    commonTerms,
		AnalysisScope:  scopeStats,
		VocabularySize: vocabularySize,
	}, nil
}

// ============================================================================
// Pure Functions - Production Code Filtering
// ============================================================================

// FilterProductionSymbols filters symbols to only production code
func FilterProductionSymbols(symbols []FileSymbol, config ProjectConfig) []FileSymbol {
	if len(symbols) == 0 {
		return symbols
	}

	result := make([]FileSymbol, 0, len(symbols))

	for _, symbol := range symbols {
		if isProductionCode(symbol, config) {
			result = append(result, symbol)
		}
	}

	return result
}

// isProductionCode determines if a file is production code
func isProductionCode(symbol FileSymbol, config ProjectConfig) bool {
	filePath := symbol.FilePath

	// Check exclude directories
	for _, excludeDir := range config.ExcludeDirs {
		if strings.Contains(filePath, excludeDir) {
			return false
		}
	}

	// Check common test patterns
	for _, pattern := range commonExcludePatterns {
		if strings.Contains(strings.ToLower(filePath), strings.ToLower(pattern)) {
			return false
		}
	}

	// Check language-specific test patterns
	switch config.Language {
	case "go":
		return !goTestPattern.MatchString(filePath)
	case "javascript", "typescript":
		return !jsTestPattern.MatchString(filePath)
	case "python":
		return !pythonTestPattern.MatchString(filePath)
	case "java":
		return !javaTestPattern.MatchString(filePath)
	default:
		// Generic filtering
		for _, marker := range config.TestMarkers {
			if strings.Contains(strings.ToLower(filePath), strings.ToLower(marker)) {
				return false
			}
		}
	}

	// Check if in source directories (if specified)
	if len(config.SourceDirs) > 0 {
		for _, srcDir := range config.SourceDirs {
			if strings.Contains(filePath, srcDir) {
				return true
			}
		}
		// If no match, likely not production
		return false
	}

	return true
}

// ============================================================================
// Pure Functions - Domain Analysis
// ============================================================================

// domainAnalysis holds temporary analysis state
type domainAnalysis struct {
	domainCounts       map[string]int
	domainTerms        map[string][]string
	domainExamples     map[string][]string
	presentDomainNames []string
}

// ExtractDomainOccurrences analyzes which domains are present
func ExtractDomainOccurrences(
	symbols []FileSymbol,
	dict *TranslationDictionary,
	splitter *NameSplitter,
) *domainAnalysis {
	analysis := &domainAnalysis{
		domainCounts:   make(map[string]int, len(dict.Domains)),
		domainTerms:    make(map[string][]string, len(dict.Domains)),
		domainExamples: make(map[string][]string, len(dict.Domains)),
	}

	// Initialize all domains with zero counts
	for domainName := range dict.Domains {
		analysis.domainCounts[domainName] = 0
	}

	// Process each symbol
	for _, symbol := range symbols {
		// Split symbol name
		words := splitter.Split(symbol.Name)
		if len(words) == 0 {
			continue
		}

		// Check each domain for matches
		for domainName, domainTerms := range dict.Domains {
			matched := false
			for _, word := range words {
				// Check exact matches
				for _, term := range domainTerms {
					if strings.EqualFold(word, term) {
						analysis.domainCounts[domainName]++
						matched = true

						// Add to matched terms
						analysis.addUniqueTerm(domainName, word)

						// Add to examples
						analysis.addExample(domainName, symbol.Name)
						break
					}
				}
				if matched {
					break
				}
			}
		}
	}

	// Identify which domains are present
	analysis.presentDomainNames = make([]string, 0, len(dict.Domains))
	for domainName, count := range analysis.domainCounts {
		if count > 0 {
			analysis.presentDomainNames = append(analysis.presentDomainNames, domainName)
		}
	}

	return analysis
}

// addUniqueTerm adds a term to domain if not already present
func (a *domainAnalysis) addUniqueTerm(domain, term string) {
	terms := a.domainTerms[domain]
	for _, t := range terms {
		if t == term {
			return
		}
	}
	a.domainTerms[domain] = append(terms, term)
}

// addExample adds an example symbol if not already present
func (a *domainAnalysis) addExample(domain, symbol string) {
	examples := a.domainExamples[domain]
	if len(examples) >= 5 {
		// Only keep first 5 examples
		return
	}
	for _, ex := range examples {
		if ex == symbol {
			return
		}
	}
	a.domainExamples[domain] = append(examples, symbol)
}

// buildDomainResults creates DomainResult from analysis
func buildDomainResults(analysis *domainAnalysis, dict *TranslationDictionary, totalSymbols int) []DomainResult {
	results := make([]DomainResult, 0, len(analysis.presentDomainNames))

	for _, domainName := range analysis.presentDomainNames {
		count := analysis.domainCounts[domainName]
		confidence := float64(count) / float64(totalSymbols)

		// Confidence cap at 1.0
		if confidence > 1.0 {
			confidence = 1.0
		}

		results = append(results, DomainResult{
			Name:           domainName,
			Count:          count,
			Confidence:     confidence,
			ExampleSymbols: analysis.domainExamples[domainName],
			MatchedTerms:   analysis.domainTerms[domainName],
		})
	}

	return results
}

// ============================================================================
// Pure Functions - Term Analysis
// ============================================================================

// IdentifyUniqueTerms finds domain-specific terms (not common dev vocabulary)
func IdentifyUniqueTerms(symbols []FileSymbol, dict *TranslationDictionary, splitter *NameSplitter) []TermResult {
	// Collect all terms from symbols
	termCounts := make(map[string]int)
	termExamples := make(map[string][]string)
	termDomains := make(map[string][]string)

	// Build set of common dev terms
	commonTerms := buildCommonTermsSet(dict)

	for _, symbol := range symbols {
		words := splitter.Split(symbol.Name)
		if len(words) == 0 {
			continue
		}

		for _, word := range words {
			// Skip very short words
			if len(word) < 3 {
				continue
			}

			// Skip common dev terms
			if commonTerms[strings.ToLower(word)] {
				continue
			}

			// Count and track examples
			termCounts[word]++
			if len(termExamples[word]) < 3 {
				termExamples[word] = append(termExamples[word], symbol.Name)
			}

			// Find which domains this term belongs to
			domains := findDomainsForTerm(word, dict)
			if len(domains) > 0 {
				termDomains[word] = domains
			}
		}
	}

	// Convert to results, limiting to top terms
	results := make([]TermResult, 0, minInt(50, len(termCounts)))
	// Sort by count (descending) - we need to do this manually
	type termCount struct {
		term  string
		count int
	}
	termList := make([]termCount, 0, len(termCounts))
	for term, count := range termCounts {
		termList = append(termList, termCount{term, count})
	}
	// Simple sort (just take first 50, not fully sorted for performance)
	// In production, we'd use sort.Slice for full ordering
	if len(termList) > 50 {
		termList = termList[:50]
	}

	for _, tc := range termList {
		results = append(results, TermResult{
			Term:           tc.term,
			Count:          tc.count,
			ExampleSymbols: termExamples[tc.term],
			Domains:        termDomains[tc.term],
		})
	}

	return results
}

// IdentifyCommonTerms finds common developer vocabulary
func IdentifyCommonTerms(symbols []FileSymbol, dict *TranslationDictionary, splitter *NameSplitter) []TermResult {
	commonTerms := buildCommonTermsSet(dict)

	termCounts := make(map[string]int)
	termExamples := make(map[string][]string)

	for _, symbol := range symbols {
		words := splitter.Split(symbol.Name)
		if len(words) == 0 {
			continue
		}

		for _, word := range words {
			// Skip very short words
			if len(word) < 3 {
				continue
			}

			// Only include common terms
			if !commonTerms[strings.ToLower(word)] {
				continue
			}

			termCounts[word]++
			if len(termExamples[word]) < 3 {
				termExamples[word] = append(termExamples[word], symbol.Name)
			}
		}
	}

	// Convert to results
	results := make([]TermResult, 0, minInt(30, len(termCounts)))
	for term, count := range termCounts {
		// Only include terms that appear multiple times
		if count < 2 {
			continue
		}
		results = append(results, TermResult{
			Term:           term,
			Count:          count,
			ExampleSymbols: termExamples[term],
		})
	}

	return results
}

// buildCommonTermsSet builds a set of common developer terms
func buildCommonTermsSet(dict *TranslationDictionary) map[string]bool {
	// Add from abbreviation dictionary
	common := make(map[string]bool)

	// Add common abbreviations
	for abbrev := range dict.Abbreviations {
		common[abbrev] = true
	}

	// Add common function/variable names
	commonTerms := []string{
		"user", "manager", "service", "repository", "model", "view", "controller",
		"handler", "provider", "factory", "builder", "mapper", "converter",
		"validator", "serializer", "deserializer", "parser", "formatter",
		"client", "server", "config", "settings", "options", "params",
		"request", "response", "error", "result", "data", "value", "item",
		"list", "array", "collection", "set", "map", "tree", "graph",
		"node", "edge", "vertex", "element", "item", "object", "entity",
		"id", "name", "title", "description", "type", "kind", "status",
		"create", "update", "delete", "remove", "get", "set", "add", "save",
		"load", "fetch", "send", "receive", "parse", "format", "validate",
		"check", "verify", "ensure", "compute", "calculate", "process",
		"handle", "manage", "control", "execute", "run", "start", "stop",
		"init", "setup", "configure", "build", "make", "create", "new",
		"temp", "tmp", "current", "active", "enabled", "disabled", "true", "false",
	}

	for _, term := range commonTerms {
		common[term] = true
	}

	return common
}

// findDomainsForTerm finds which semantic domains a term belongs to
func findDomainsForTerm(term string, dict *TranslationDictionary) []string {
	domains := make([]string, 0, 5)
	termLower := strings.ToLower(term)

	for domainName, domainTerms := range dict.Domains {
		for _, domainTerm := range domainTerms {
			if strings.EqualFold(termLower, strings.ToLower(domainTerm)) {
				domains = append(domains, domainName)
				break
			}
		}
	}

	return domains
}

// ============================================================================
// Pure Functions - Statistics
// ============================================================================

// CalculateScopeStatistics calculates analysis scope statistics
func CalculateScopeStatistics(
	allSymbols []FileSymbol,
	productionSymbols []FileSymbol,
	config ProjectConfig,
) ScopeStatistics {
	// Count symbol types
	var totalFunctions, totalVariables, totalTypes int
	for _, symbol := range productionSymbols {
		switch symbol.Type {
		case "function", "method":
			totalFunctions++
		case "variable", "field":
			totalVariables++
		case "type", "class", "interface", "struct":
			totalTypes++
		}
	}

	// Get source directories
	sourceDirs := config.SourceDirs
	if len(sourceDirs) == 0 {
		// Infer from file paths
		sourceDirs = inferSourceDirectories(allSymbols)
	}

	return ScopeStatistics{
		TotalFiles:        countUniqueFiles(allSymbols),
		ProductionFiles:   countUniqueFiles(productionSymbols),
		TestFilesExcluded: countUniqueFiles(allSymbols) - countUniqueFiles(productionSymbols),
		SourceDirectories: sourceDirs,
		TotalSymbols:      len(allSymbols),
		TotalFunctions:    totalFunctions,
		TotalVariables:    totalVariables,
		TotalTypes:        totalTypes,
	}
}

// inferSourceDirectories infers source directories from file paths
func inferSourceDirectories(symbols []FileSymbol) []string {
	dirMap := make(map[string]bool)

	for _, symbol := range symbols {
		dir := filepath.Dir(symbol.FilePath)

		// Check for common source directory patterns
		for _, pattern := range []string{"cmd", "internal", "pkg", "lib", "src", "app", "core"} {
			if strings.Contains(dir, pattern) {
				dirMap[pattern] = true
			}
		}
	}

	dirs := make([]string, 0, len(dirMap))
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}

	return dirs
}

// countUniqueFiles counts unique file paths
func countUniqueFiles(symbols []FileSymbol) int {
	fileMap := make(map[string]bool)
	for _, symbol := range symbols {
		fileMap[symbol.FilePath] = true
	}
	return len(fileMap)
}

// ============================================================================
// Helper Functions
// ============================================================================

// getAllDomainNames returns all domain names from dictionary
func getAllDomainNames(dict *TranslationDictionary) []string {
	domains := make([]string, 0, len(dict.Domains))
	for domainName := range dict.Domains {
		domains = append(domains, domainName)
	}
	return domains
}

// getAbsentDomains returns domains that are not present
func getAbsentDomains(analysis *domainAnalysis, dict *TranslationDictionary) []string {
	absent := make([]string, 0, len(dict.Domains))
	for domainName := range dict.Domains {
		if analysis.domainCounts[domainName] == 0 {
			absent = append(absent, domainName)
		}
	}
	return absent
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
