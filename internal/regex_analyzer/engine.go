package regex_analyzer

import (
	"regexp"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/interfaces"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// HybridRegexEngine combines our classifier, cache, and trigram extraction
// to provide optimized regex searching with trigram filtering
type HybridRegexEngine struct {
	classifier *RegexClassifier
	cache      *RegexCache
	extractor  *LiteralExtractor
	indexer    interfaces.Indexer // Access to trigram index
}

// NewHybridRegexEngine creates a new hybrid regex engine
func NewHybridRegexEngine(simpleCacheSize, complexCacheSize int, indexer interfaces.Indexer) *HybridRegexEngine {
	return &HybridRegexEngine{
		classifier: NewRegexClassifier(),
		cache:      NewRegexCache(simpleCacheSize, complexCacheSize),
		extractor:  NewLiteralExtractor(),
		indexer:    indexer,
	}
}

// RegexExecutionResult represents the result of a regex search
type RegexExecutionResult struct {
	// Execution path taken
	ExecutionPath ExecutionPath

	// Timing information
	TotalTime  time.Duration
	CacheTime  time.Duration
	FilterTime time.Duration
	SearchTime time.Duration

	// Performance metrics
	CandidatesFiltered int
	CandidatesTotal    int
	MatchesFound       int

	// Cache statistics
	CacheHit    bool
	PatternType string
}

// ExecutionPath indicates which path was taken for regex execution
type ExecutionPath int

const (
	// Execution paths
	PathSimpleTrigramFiltered ExecutionPath = iota
	PathSimpleNoTrigrams
	PathComplexDirect
	PathComplexFiltered
	PathError
)

// SearchWithRegex performs regex search with trigram optimization
func (hre *HybridRegexEngine) SearchWithRegex(
	pattern string,
	caseInsensitive bool,
	getFileContent func(types.FileID) ([]byte, bool),
	candidateFiles []types.FileID,
) ([]searchtypes.Match, *RegexExecutionResult) {

	startTime := time.Now()
	result := &RegexExecutionResult{
		CandidatesTotal: len(candidateFiles),
	}

	// Check cache first
	cacheStart := time.Now()
	simplePattern, complexRegex := hre.cache.GetRegex(pattern, caseInsensitive)
	result.CacheTime = time.Since(cacheStart)
	result.CacheHit = simplePattern != nil || complexRegex != nil

	// Classify and compile if not cached
	if simplePattern == nil && complexRegex == nil {
		classifyStart := time.Now()
		isSimple := hre.classifier.IsSimple(pattern)
		classifyTime := time.Since(classifyStart)

		if isSimple {
			// Parse and cache simple pattern
			simplePattern = hre.parseSimplePattern(pattern, caseInsensitive)
			if simplePattern != nil {
				hre.cache.CacheSimple(simplePattern, caseInsensitive)
			}
		} else {
			// Compile and cache complex pattern
			compiled, err := hre.compileComplexPattern(pattern, caseInsensitive)
			if err == nil {
				hre.cache.CacheComplex(pattern, compiled, caseInsensitive)
				complexRegex = compiled
			}
		}

		// Include classification time in cache time for simplicity
		result.CacheTime += classifyTime
	}

	// Execute based on pattern type
	var matches []searchtypes.Match

	if simplePattern != nil {
		matches, result.ExecutionPath = hre.executeSimplePattern(
			simplePattern, candidateFiles, getFileContent, result)
	} else if complexRegex != nil {
		matches, result.ExecutionPath = hre.executeComplexPattern(
			complexRegex, candidateFiles, getFileContent, result)
	} else {
		// Regex compilation failed - return empty result
		result.ExecutionPath = PathError
		matches = nil
	}

	result.TotalTime = time.Since(startTime)
	result.MatchesFound = len(matches)

	return matches, result
}

// parseSimplePattern parses a simple regex pattern into a SimpleRegexPattern
func (hre *HybridRegexEngine) parseSimplePattern(pattern string, caseInsensitive bool) *SimpleRegexPattern {
	// Try to compile the regex
	// Always use multiline mode so ^ and $ match line boundaries, not just start/end of text
	flags := "(?m)"
	if caseInsensitive {
		flags = "(?mi)"
	}

	compiled, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil
	}

	// Extract literals for trigram filtering
	literals := hre.extractor.ExtractLiterals(pattern)

	return &SimpleRegexPattern{
		Pattern:  pattern,
		Literals: literals,
		Compiled: compiled,
	}
}

// compileComplexPattern compiles a complex regex pattern
func (hre *HybridRegexEngine) compileComplexPattern(pattern string, caseInsensitive bool) (*regexp.Regexp, error) {
	// Always use multiline mode so ^ and $ match line boundaries, not just start/end of text
	flags := "(?m)"
	if caseInsensitive {
		flags = "(?mi)"
	}

	return regexp.Compile(flags + pattern)
}

// executeSimplePattern executes a simple pattern with trigram filtering
func (hre *HybridRegexEngine) executeSimplePattern(
	pattern *SimpleRegexPattern,
	candidateFiles []types.FileID,
	getFileContent func(types.FileID) ([]byte, bool),
	result *RegexExecutionResult,
) ([]searchtypes.Match, ExecutionPath) {

	filterStart := time.Now()

	// Filter candidates using trigrams if we have literals
	var filteredCandidates []types.FileID
	if len(pattern.Literals) > 0 {
		filteredCandidates = hre.filterCandidatesByLiterals(
			pattern.Literals, candidateFiles, getFileContent)
		result.CandidatesFiltered = len(filteredCandidates)
	}

	filterTime := time.Since(filterStart)

	// If no trigram filtering possible or no candidates after filtering, use all candidates
	if len(filteredCandidates) == 0 {
		filteredCandidates = candidateFiles
		result.ExecutionPath = PathSimpleNoTrigrams
	} else {
		result.ExecutionPath = PathSimpleTrigramFiltered
	}

	// Execute regex search on filtered candidates
	searchStart := time.Now()
	matches, _ := hre.executeRegexOnCandidates(
		pattern.Compiled, filteredCandidates, getFileContent)
	result.SearchTime = time.Since(searchStart)
	result.FilterTime += filterTime

	return matches, result.ExecutionPath
}

// executeComplexPattern executes a complex pattern without trigram filtering
func (hre *HybridRegexEngine) executeComplexPattern(
	regex *regexp.Regexp,
	candidateFiles []types.FileID,
	getFileContent func(types.FileID) ([]byte, bool),
	result *RegexExecutionResult,
) ([]searchtypes.Match, ExecutionPath) {

	// For complex patterns, we don't do trigram filtering
	// We could potentially extract some literals from complex patterns in the future
	result.ExecutionPath = PathComplexDirect

	searchStart := time.Now()
	matches, _ := hre.executeRegexOnCandidates(regex, candidateFiles, getFileContent)
	result.SearchTime = time.Since(searchStart)

	return matches, result.ExecutionPath
}

// filterCandidatesByLiterals filters candidates using trigram-like literal matching
func (hre *HybridRegexEngine) filterCandidatesByLiterals(
	literals []string,
	candidateFiles []types.FileID,
	getFileContent func(types.FileID) ([]byte, bool),
) []types.FileID {

	if len(literals) == 0 || len(candidateFiles) == 0 {
		return nil
	}

	// Try to use the trigram index for massive performance improvement
	if trigramIndex := hre.getTrigramIndex(); trigramIndex != nil {
		return hre.filterCandidatesWithTrigramIndex(literals, candidateFiles, trigramIndex)
	}

	// Fallback to linear search if trigram index not available
	return hre.filterCandidatesLinear(literals, candidateFiles, getFileContent)
}

// getTrigramIndex attempts to get the trigram index from the indexer
func (hre *HybridRegexEngine) getTrigramIndex() *core.TrigramIndex {
	// Check if indexer has trigram index access
	if hre.indexer == nil {
		return nil
	}
	type trigramAccessor interface{ GetTrigramIndex() *core.TrigramIndex }
	if ta, ok := any(hre.indexer).(trigramAccessor); ok {
		return ta.GetTrigramIndex()
	}
	return nil
}

// filterCandidatesWithTrigramIndex uses the actual trigram index for fast filtering
func (hre *HybridRegexEngine) filterCandidatesWithTrigramIndex(
	literals []string,
	candidateFiles []types.FileID,
	trigramIndex *core.TrigramIndex,
) []types.FileID {

	// Create a set of initial candidates for intersection
	candidateSet := make(map[types.FileID]bool)
	for _, fileID := range candidateFiles {
		candidateSet[fileID] = true
	}

	// Collect results from all literals
	var allCandidates []types.FileID
	seen := make(map[types.FileID]int)

	for _, literal := range literals {
		if len(literal) < 3 {
			// For short literals, we can't use trigram index effectively
			continue
		}

		// Use trigram index to find files containing this literal
		literalCandidates := trigramIndex.FindCandidates(literal)

		for _, fileID := range literalCandidates {
			// Only include files that were in our original candidate set
			if candidateSet[fileID] {
				seen[fileID]++
				if seen[fileID] == 1 {
					allCandidates = append(allCandidates, fileID)
				}
			}
		}
	}

	// If no trigram matches, fall back to linear search for remaining literals
	if len(allCandidates) == 0 {
		return nil
	}

	return allCandidates
}

// filterCandidatesLinear performs linear search as fallback
func (hre *HybridRegexEngine) filterCandidatesLinear(
	literals []string,
	candidateFiles []types.FileID,
	getFileContent func(types.FileID) ([]byte, bool),
) []types.FileID {

	var filtered []types.FileID
	seen := make(map[types.FileID]bool)

	for _, fileID := range candidateFiles {
		if seen[fileID] {
			continue
		}

		// PERFORMANCE: Get content directly (zero-copy)
		content, ok := getFileContent(fileID)
		if !ok {
			continue
		}

		// Check if file content contains any of the literals
		if hre.containsAnyLiteral(content, literals) {
			filtered = append(filtered, fileID)
			seen[fileID] = true
		}
	}

	return filtered
}

// containsAnyLiteral checks if content contains any of the specified literals
func (hre *HybridRegexEngine) containsAnyLiteral(content []byte, literals []string) bool {
	contentStr := string(content)
	for _, literal := range literals {
		if strings.Contains(contentStr, literal) {
			return true
		}
	}
	return false
}

// executeRegexOnCandidates executes regex search on specific candidate files
func (hre *HybridRegexEngine) executeRegexOnCandidates(
	regex *regexp.Regexp,
	candidateFiles []types.FileID,
	getFileContent func(types.FileID) ([]byte, bool),
) ([]searchtypes.Match, error) {

	var allMatches []searchtypes.Match

	for _, fileID := range candidateFiles {
		// PERFORMANCE: Get content directly (zero-copy) instead of full FileInfo
		content, ok := getFileContent(fileID)
		if !ok {
			continue
		}

		// Find all matches in this file
		allIndices := regex.FindAllIndex(content, -1)

		// Convert to per-file Match with FileID
		for _, match := range allIndices {
			allMatches = append(allMatches, searchtypes.Match{
				Start:  match[0],
				End:    match[1],
				Exact:  false,
				FileID: fileID,
			})
		}
	}

	return allMatches, nil
}

// fallbackToLiteralSearch performs simple literal search as fallback
func (hre *HybridRegexEngine) fallbackToLiteralSearch(pattern string, content []byte) []searchtypes.Match {
	// Very simple literal search fallback
	var matches []searchtypes.Match

	patternBytes := []byte(pattern)
	start := 0
	for {
		idx := indexOf(content[start:], patternBytes)
		if idx < 0 {
			break
		}

		matchStart := start + idx
		matchEnd := matchStart + len(patternBytes)

		matches = append(matches, searchtypes.Match{
			Start: matchStart,
			End:   matchEnd,
			Exact: true, // Literal matches are exact
		})

		start = matchStart + 1
	}

	return matches
}

// Simple bytes.IndexOf implementation
func indexOf(data, pattern []byte) int {
	for i := 0; i <= len(data)-len(pattern); i++ {
		if len(pattern) == 0 {
			return i
		}
		match := true
		for j := 0; j < len(pattern); j++ {
			if i+j >= len(data) || data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// GetCacheStats returns cache performance statistics
func (hre *HybridRegexEngine) GetCacheStats() CacheStats {
	return hre.cache.GetStats()
}

// ClearCache clears all cached patterns
func (hre *HybridRegexEngine) ClearCache() {
	hre.cache.Clear()
}

// GetCacheSize returns current cache sizes
func (hre *HybridRegexEngine) GetCacheSize() (simple, complex int) {
	return hre.cache.GetSize()
}

// GetHitRatio returns cache hit ratios
func (hre *HybridRegexEngine) GetHitRatio() (simple, complex float64) {
	return hre.cache.GetHitRatio(), hre.cache.GetComplexHitRatio()
}

// GetMostAccessedPatterns returns the most frequently accessed simple patterns
func (hre *HybridRegexEngine) GetMostAccessedPatterns(limit int) []*SimpleRegexPattern {
	return hre.cache.GetMostAccessedSimple(limit)
}

// ExtractLiterals extracts literal strings from a regex pattern suitable for trigram filtering
func (hre *HybridRegexEngine) ExtractLiterals(pattern string) []string {
	return hre.extractor.ExtractLiterals(pattern)
}
