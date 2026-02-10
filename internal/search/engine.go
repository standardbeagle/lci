package search

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/interfaces"
	"github.com/standardbeagle/lci/internal/regex_analyzer"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"
)

// SearchRankingScoreConstants defines scoring constants for search ranking configuration
const (
	DefaultCodeFileBoost    = 50.0
	DefaultDocFilePenalty   = -20.0
	DefaultConfigFileBoost  = 10.0
	DefaultNonSymbolPenalty = -30.0
	RequireSymbolPenalty    = -1000.0
)

type Engine struct {
	indexer          interfaces.Indexer
	contextExtractor *ContextExtractor
	regexEngine      *regex_analyzer.HybridRegexEngine
	semanticScorer   *semantic.SemanticScorer // Semantic scoring for advanced matching (camelCase, fuzzy, etc.)
}

func (e *Engine) LastError() error {
	// No longer tracking regex errors globally
	return nil
}

// FileProvider is the minimal interface needed by the search engine
type FileProvider interface {
	GetFileInfo(types.FileID) *types.FileInfo
	GetAllFileIDs() []types.FileID
}

// Type aliases for backward compatibility and convenience
type GrepResult = searchtypes.GrepResult
type StandardResult = searchtypes.StandardResult
type Match = searchtypes.Match
type ExtractedContext = searchtypes.ExtractedContext

// Note: The following type aliases have been removed (use the new names instead):
// - Result → GrepResult
// - DetailedResult → StandardResult
// - EnhancedResult → StandardResult

// FileCategory represents the type of file for ranking purposes
type FileCategory int

const (
	FileCategoryCode FileCategory = iota
	FileCategoryDocumentation
	FileCategoryConfig
	FileCategoryTest
	FileCategoryUnknown
)

// codeExtensions defines source code files that get priority boost in search ranking.
// This is more comprehensive than indexing/constants.go:SourceFileExtensions since
// search ranking benefits from recognizing additional languages even if they're
// not fully indexed with semantic parsing.
var codeExtensions = map[string]bool{
	".go": true, ".rs": true, ".py": true, ".js": true, ".jsx": true,
	".ts": true, ".tsx": true, ".java": true, ".c": true, ".cpp": true,
	".cc": true, ".cxx": true, ".h": true, ".hpp": true, ".cs": true,
	".php": true, ".rb": true, ".swift": true, ".kt": true, ".scala": true,
	".lua": true, ".pl": true, ".pm": true, ".r": true, ".jl": true,
	".ex": true, ".exs": true, ".erl": true, ".hrl": true, ".hs": true,
	".clj": true, ".cljs": true, ".elm": true, ".vue": true, ".svelte": true,
	".zig": true, ".nim": true, ".v": true, ".d": true, ".m": true, ".mm": true,
}

// docExtensions - documentation files that get demoted in rankings
var docExtensions = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".rst": true,
	".adoc": true, ".asciidoc": true, ".rdoc": true, ".org": true,
	".wiki": true, ".textile": true, ".pod": true, ".rmd": true,
}

// configExtensions - configuration files with slight boost
var configExtensions = map[string]bool{
	".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".ini": true, ".cfg": true, ".conf": true, ".xml": true,
	".kdl": true, ".env": true, ".properties": true,
}

// classifyFile determines the category of a file based on its path
func classifyFile(path string) FileCategory {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// Check for test files first (by name pattern)
	if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_") {
		return FileCategoryTest
	}

	if codeExtensions[ext] {
		return FileCategoryCode
	}
	if docExtensions[ext] {
		return FileCategoryDocumentation
	}
	if configExtensions[ext] {
		return FileCategoryConfig
	}

	return FileCategoryUnknown
}

// getRankingConfig returns the search ranking configuration, with defaults if not available
func (e *Engine) getRankingConfig() config.SearchRanking {
	// Try to get config from indexer if it implements ConfigProvider
	if e.indexer != nil {
		type configProvider interface {
			GetConfig() *config.Config
		}
		if cp, ok := e.indexer.(configProvider); ok {
			if cfg := cp.GetConfig(); cfg != nil {
				return cfg.Search.Ranking
			}
		}
	}

	// Return defaults
	return config.SearchRanking{
		Enabled:          true,
		CodeFileBoost:    DefaultCodeFileBoost,
		DocFilePenalty:   DefaultDocFilePenalty,
		ConfigFileBoost:  DefaultConfigFileBoost,
		RequireSymbol:    false,
		NonSymbolPenalty: DefaultNonSymbolPenalty,
	}
}

// scoreFileType returns a score adjustment based on file extension category
func (e *Engine) scoreFileType(path string, cfg config.SearchRanking) float64 {
	ext := strings.ToLower(filepath.Ext(path))

	// Check for explicit extension override first
	if cfg.ExtensionWeights != nil {
		if weight, ok := cfg.ExtensionWeights[ext]; ok {
			return weight
		}
	}

	// Apply category-based scoring
	category := classifyFile(path)
	switch category {
	case FileCategoryCode:
		return cfg.CodeFileBoost
	case FileCategoryDocumentation:
		return cfg.DocFilePenalty
	case FileCategoryConfig:
		return cfg.ConfigFileBoost
	case FileCategoryTest:
		return cfg.CodeFileBoost * 0.8 // Slightly lower than production code
	default:
		return 0.0
	}
}

// scoreSymbolPresence returns a score adjustment based on whether a symbol exists at the match
func (e *Engine) scoreSymbolPresence(hasSymbol bool, cfg config.SearchRanking) float64 {
	if !hasSymbol {
		if cfg.RequireSymbol {
			return RequireSymbolPenalty // Effectively filter out
		}
		return cfg.NonSymbolPenalty
	}
	return 0.0
}

// LineProvider provides zero-allocation line access using LineOffsets
type LineProvider interface {
	// GetLine returns the content of a single line (1-based). Returns empty string if out of range.
	GetLine(fileInfo *types.FileInfo, lineNum int) string
	// GetLineCount returns the total number of lines in the file.
	GetLineCount(fileInfo *types.FileInfo) int
	// GetLineRange returns lines from startLine to endLine (inclusive, 1-based).
	GetLineRange(fileInfo *types.FileInfo, startLine, endLine int) []string
}

type ContextExtractor struct {
	maxLines            int
	defaultContextLines int // Default context lines when no block is found
	lineProvider        LineProvider
	// Pre-compiled regexes (lock-free, component-local)
	selfClosingPattern *regexp.Regexp
}

func NewEngine(indexer interfaces.Indexer) *Engine {
	// Use default of 50 lines of context
	// This can be overridden by creating a custom engine with NewEngineWithConfig
	e := &Engine{
		indexer:     indexer,
		regexEngine: regex_analyzer.NewHybridRegexEngine(1000, 1000, indexer),
	}
	e.contextExtractor = NewContextExtractorWithLineProvider(0, 50, e)
	return e
}

// NewEngineWithSemanticScorer creates a new search engine with semantic scoring enabled
func NewEngineWithSemanticScorer(indexer interfaces.Indexer, scorer *semantic.SemanticScorer) *Engine {
	e := &Engine{
		indexer:        indexer,
		regexEngine:    regex_analyzer.NewHybridRegexEngine(1000, 1000, indexer),
		semanticScorer: scorer,
	}
	e.contextExtractor = NewContextExtractorWithLineProvider(0, 50, e)
	return e
}

// NewEngineWithConfig creates a new search engine with custom configuration
func NewEngineWithConfig(indexer interfaces.Indexer, defaultContextLines int) *Engine {
	if defaultContextLines <= 0 {
		defaultContextLines = DefaultContextLines
	}

	e := &Engine{
		indexer:     indexer,
		regexEngine: regex_analyzer.NewHybridRegexEngine(1000, 1000, indexer),
	}
	e.contextExtractor = NewContextExtractorWithLineProvider(0, defaultContextLines, e)
	return e
}

func NewContextExtractor(maxLines int) *ContextExtractor {
	return &ContextExtractor{
		maxLines:            maxLines,
		defaultContextLines: DefaultContextLines,
		selfClosingPattern:  regexp.MustCompile(`<[^>]*\s*/>`),
	}
}

// NewContextExtractorWithLineProvider creates a ContextExtractor with a LineProvider for zero-allocation line access
func NewContextExtractorWithLineProvider(maxLines int, defaultContextLines int, lp LineProvider) *ContextExtractor {
	return &ContextExtractor{
		maxLines:            maxLines,
		defaultContextLines: defaultContextLines,
		lineProvider:        lp,
		selfClosingPattern:  regexp.MustCompile(`<[^>]*\s*/>`),
	}
}

func lineStart(content []byte, offset int) int {
	idx := bytes.LastIndexByte(content[:offset], '\n')
	if idx < 0 {
		return 0
	}
	return idx + 1
}

// bytesToLine counts the number of lines from the beginning of content up to offset
func bytesToLine(content []byte, offset int) int {
	if offset >= len(content) {
		offset = len(content) - 1
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}

// New helper functions using indexer interface instead of direct content access
func (e *Engine) byteOffsetToLine(fileID types.FileID, offset int) int {
	content, ok := e.indexer.GetFileContent(fileID)
	if !ok {
		return 1
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}

func (e *Engine) lineStartOffset(fileID types.FileID, offset int) int {
	content, ok := e.indexer.GetFileContent(fileID)
	if !ok {
		return 0
	}
	idx := bytes.LastIndexByte(content[:offset], '\n')
	if idx < 0 {
		return 0
	}
	return idx + 1
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

func findAllMatches(content, pattern []byte) []Match {
	// Default literal matching excludes comment-only lines to identify primary code matches
	return findAllMatchesWithOptions(content, pattern, types.SearchOptions{CaseInsensitive: false, ExcludeComments: true})
}

func findAllMatchesWithOptions(content, pattern []byte, options types.SearchOptions) []Match {
	// Content filtering will be integrated with AST in future iteration
	// For now, maintain existing search behavior

	// Word boundary support (grep -w)
	// Convert to regex with word boundaries if enabled
	if options.WordBoundary && !options.UseRegex {
		// Use regex engine with \b word boundaries
		regexPattern := `\b` + regexp.QuoteMeta(string(pattern)) + `\b`
		opts := options
		opts.UseRegex = true
		m, err := findRegexMatchesLegacy(content, regexPattern, opts)
		if err != nil {
			return nil
		}
		return m
	}

	if options.UseRegex {
		m, err := findRegexMatchesLegacy(content, string(pattern), options)
		if err != nil {
			return nil
		}
		return m
	}

	// Literal string search (original behavior)
	return findLiteralMatches(content, pattern, options)
}

func findRegexMatchesLegacy(content []byte, pattern string, options types.SearchOptions) ([]Match, error) {
	var matches []Match

	// Compile regex with appropriate flags
	// Always use multiline mode so ^ and $ match line boundaries, not just start/end of text
	flags := "(?m)"
	if options.CaseInsensitive {
		flags = "(?mi)"
	}

	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %v", err)
	}

	// Find all matches
	allMatches := re.FindAllIndex(content, -1)
	for _, match := range allMatches {
		matches = append(matches, Match{
			Start: match[0],
			End:   match[1],
			Exact: false, // Regex matches are not exact word matches by default
		})
	}

	return matches, nil
}

// searchWithHybridRegex uses the hybrid regex engine for multi-file regex searches
func (e *Engine) searchWithHybridRegex(pattern string, candidates []types.FileID, options types.SearchOptions) []GrepResult {
	regexMatches, _ := e.regexEngine.SearchWithRegex(
		pattern,
		options.CaseInsensitive,
		e.indexer.GetFileContent,
		candidates,
	)

	var allResults []GrepResult

	// Track matches per file for MaxCountPerFile support
	fileMatchCounts := make(map[types.FileID]int)

	// Hybrid engine returns matches without file attribution; we must map by scanning each file.
	for _, match := range regexMatches {
		// Apply MaxCountPerFile limit if set
		if options.MaxCountPerFile > 0 {
			if fileMatchCounts[match.FileID] >= options.MaxCountPerFile {
				continue // Skip this match, already hit limit for this file
			}
		}

		// PERFORMANCE: Use lightweight accessors instead of GetFileInfo
		content, ok := e.indexer.GetFileContent(match.FileID)
		if !ok {
			continue
		}

		offsets, ok := e.indexer.GetFileLineOffsets(match.FileID)
		if !ok {
			continue
		}

		path := e.indexer.GetFilePath(match.FileID)

		// Binary search for line
		line := 1
		col := 0
		if len(offsets) > 0 {
			l, r := 0, len(offsets)-1
			for l <= r {
				m := (l + r) / 2
				if int(offsets[m]) <= match.Start {
					l = m + 1
				} else {
					r = m - 1
				}
			}
			line = r + 1
			col = match.Start - int(offsets[line-1])
		}
		if match.End > len(content) {
			continue
		}
		matchText := string(content[match.Start:match.End])
		res := GrepResult{FileID: match.FileID, Path: path, Line: line, Column: col, Match: matchText, Score: 1.0}
		res.Context = e.extractSimpleContext(content, match.Start, match.End)
		allResults = append(allResults, res)
		fileMatchCounts[match.FileID]++
	}

	// Apply semantic filtering if requested
	allResults = e.applySemanticFilteringToGrepResults(allResults, pattern, options)

	return allResults
}

// applySemanticFilteringToGrepResults applies semantic filtering to GrepResult slices
// This is used for regex search results that bypass the normal per-file processing
// ZERO-ALLOC OPTIMIZED: Filters in-place when possible, uses linear symbol scan to avoid map allocation
func (e *Engine) applySemanticFilteringToGrepResults(results []GrepResult, pattern string, options types.SearchOptions) []GrepResult {
	// Early exit if no semantic filtering requested
	if len(options.SymbolTypes) == 0 && !options.DeclarationOnly && !options.UsageOnly &&
		!options.ExportedOnly && !options.ExcludeTests && !options.ExcludeComments &&
		!options.MutableOnly && !options.GlobalOnly {
		return results
	}

	if len(results) == 0 {
		return results
	}

	// Filter in-place to avoid allocation - reuse input slice
	writeIdx := 0
	var lastFileID types.FileID
	var lastFileInfo *types.FileInfo

	for _, result := range results {
		var fileInfo *types.FileInfo

		// Check if this is the same file as last iteration (common case)
		if result.FileID == lastFileID && lastFileInfo != nil {
			fileInfo = lastFileInfo
		} else {
			fi := e.indexer.GetFileInfo(result.FileID)
			if fi == nil {
				if len(options.SymbolTypes) > 0 {
					continue // Skip - need symbol info for SymbolTypes filter
				}
				// Keep result for other filters
				results[writeIdx] = result
				writeIdx++
				continue
			}
			fileInfo = fi
			lastFileID = result.FileID
			lastFileInfo = fileInfo
		}

		// Check if this result passes semantic filtering (zero-alloc linear scan)
		if e.passesSemanticFilterForGrepResultZeroAlloc(fileInfo, result, pattern, options) {
			results[writeIdx] = result
			writeIdx++
		}
	}

	return results[:writeIdx]
}

// passesSemanticFilterForGrepResultZeroAlloc checks semantic filtering with zero map allocations
// Uses linear scan through symbols - typically faster for small symbol counts due to cache locality
func (e *Engine) passesSemanticFilterForGrepResultZeroAlloc(fileInfo *types.FileInfo, result GrepResult, pattern string, options types.SearchOptions) bool {
	line := result.Line

	// Check if in comment (if ExcludeComments is enabled)
	if options.ExcludeComments && e.isInComment(fileInfo, line) {
		return false
	}

	// Find symbol at this location using linear scan (avoids map allocation)
	var matchingSymbol *types.EnhancedSymbol
	var firstSymbolOnLine *types.EnhancedSymbol

	for _, symbol := range fileInfo.EnhancedSymbols {
		if symbol.Line != line {
			continue
		}

		// Track first symbol on line for fallback
		if firstSymbolOnLine == nil {
			firstSymbolOnLine = symbol
		}

		// Check if symbol matches pattern
		var matches bool
		if e.semanticScorer != nil {
			score := e.semanticScorer.ScoreSymbol(pattern, symbol.Name)
			matches = score.Score >= e.semanticScorer.GetConfig().MinScore
		} else {
			matches = strings.Contains(symbol.Name, pattern) || strings.Contains(pattern, symbol.Name)
		}

		if matches {
			matchingSymbol = symbol
			break
		}
	}

	// Use first symbol on line if no pattern match found
	if matchingSymbol == nil {
		matchingSymbol = firstSymbolOnLine
	}

	// Apply symbol-based filters
	if matchingSymbol != nil {
		if len(options.SymbolTypes) > 0 {
			if !contains(options.SymbolTypes, matchingSymbol.Type.String()) {
				return false
			}
		}

		if options.DeclarationOnly {
			return true
		}

		if options.ExportedOnly && !e.isExportedEnhancedSymbol(matchingSymbol, fileInfo) {
			return false
		}

		if options.MutableOnly && !e.isMutableSymbol(matchingSymbol, fileInfo, line) {
			return false
		}

		if options.GlobalOnly && !e.isGlobalSymbol(matchingSymbol, fileInfo) {
			return false
		}
	} else {
		// No symbol found - reject if SymbolTypes specified
		if len(options.SymbolTypes) > 0 {
			return false
		}

		if options.DeclarationOnly {
			return false
		}

		if options.UsageOnly {
			return true
		}
	}

	return true
}

// extractSimpleContext extracts basic context around a match
// Note: This is a fallback for when FileInfo.LineOffsets is not available
// Prefer using extractSimpleContextWithOffsets for better performance
func (e *Engine) extractSimpleContext(content []byte, start, end int) ExtractedContext {
	// Find the line containing the match using byte position
	matchLine := bytesToLine(content, start)

	// Get a few lines around the match
	contextLines := 3
	startLine := matchLine - contextLines
	if startLine < 1 {
		startLine = 1
	}

	// Estimate end line (we don't have total line count without splitting)
	// This is inefficient but necessary without LineOffsets
	endLine := matchLine + contextLines

	// Extract context lines by counting newlines
	var contextLinesSlice []string
	lineNum := 0
	lineStart := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' || i == len(content)-1 {
			lineNum++
			lineEnd := i
			if i == len(content)-1 && content[i] != '\n' {
				lineEnd = i + 1
			}

			if lineNum >= startLine && lineNum <= endLine {
				contextLinesSlice = append(contextLinesSlice, string(content[lineStart:lineEnd]))
			}

			if lineNum > endLine {
				break
			}

			lineStart = i + 1
		}
	}

	return ExtractedContext{
		Lines:      contextLinesSlice,
		StartLine:  startLine,
		EndLine:    startLine + len(contextLinesSlice) - 1,
		IsComplete: true,
	}
}

func findLiteralMatches(content, pattern []byte, options types.SearchOptions) []Match {
	var matches []Match

	// Early return for empty pattern or content
	if len(pattern) == 0 || len(content) == 0 {
		return matches
	}

	offset := 0

	searchContent := content
	searchPattern := pattern

	if options.CaseInsensitive {
		searchContent = bytes.ToLower(content)
		searchPattern = bytes.ToLower(pattern)
	}

	for {
		// Check bounds before slicing
		if offset >= len(searchContent) {
			break
		}

		idx := bytes.Index(searchContent[offset:], searchPattern)
		if idx < 0 {
			break
		}

		start := offset + idx
		end := start + len(pattern)

		// Optionally skip matches on comment-only lines when requested
		if options.ExcludeComments {
			// Determine the start of the current line in original content
			lineStartIdx := bytes.LastIndex(content[:start], []byte("\n")) + 1
			lineEndIdx := bytes.Index(content[start:], []byte("\n"))
			if lineEndIdx < 0 {
				lineEndIdx = len(content) - start
			}
			line := content[lineStartIdx : start+lineEndIdx]
			trimmed := bytes.TrimLeft(line, " \t")
			if bytes.HasPrefix(trimmed, []byte("//")) {
				// Advance offset past this position and continue
				offset = start + 1
				continue
			}
		}

		// Check if exact word match (use original content for word boundary check)
		exact := true
		if start > 0 && isWordChar(content[start-1]) {
			exact = false
		}
		if end < len(content) && isWordChar(content[end]) {
			exact = false
		}

		matches = append(matches, Match{
			Start: start,
			End:   end,
			Exact: exact,
		})

		offset = start + 1
	}

	return matches
}

func (e *Engine) Search(pattern string, candidates []types.FileID, maxContextLines int) []GrepResult {
	return e.SearchWithOptions(pattern, candidates, types.SearchOptions{
		MaxContextLines: maxContextLines,
	})
}

// ====== Helper functions for SearchWithOptions - reducing complexity through decomposition ======

// validateAndPreparePatterns validates search patterns and returns the patterns to use
func (e *Engine) validateAndPreparePatterns(pattern string, options types.SearchOptions) ([]string, bool) {
	searchPatterns := []string{pattern}
	if len(options.Patterns) > 0 {
		searchPatterns = options.Patterns
	}

	// Validate at least one pattern
	if len(searchPatterns) == 0 || (len(searchPatterns) == 1 && searchPatterns[0] == "") {
		return nil, false
	}

	return searchPatterns, true
}

// prepareCandidates prepares the candidate files for searching
func (e *Engine) prepareCandidates(pattern string, candidates []types.FileID, options types.SearchOptions) []types.FileID {
	// Prefer indexed candidate pruning when available (non-regex, length>=3)
	if len(candidates) == 0 {
		if !options.UseRegex && len(pattern) >= 3 {
			// Try to use indexer's candidate finder if available
			type candidateProvider interface {
				FindCandidateFiles(string, bool) []types.FileID
			}
			if cp, ok := any(e.indexer).(candidateProvider); ok {
				candidates = cp.FindCandidateFiles(pattern, options.CaseInsensitive)
			}
		}
		if len(candidates) == 0 {
			candidates = e.indexer.GetAllFileIDs()
			if len(candidates) == 0 {
				return nil
			}
		}
	}

	// Apply path-based include/exclude filters early to reduce candidates
	candidates = e.filterIncludedFiles(candidates, options.IncludePattern)
	candidates = e.filterExcludedFiles(candidates, options.ExcludePattern)

	return candidates
}

// getEffectiveResultCap determines the effective result cap for the search
func (e *Engine) getEffectiveResultCap(candidates []types.FileID, options types.SearchOptions) int {
	effectiveCap := options.MaxResults
	if effectiveCap <= 0 {
		if len(candidates) >= 400 {
			effectiveCap = 25
		} else {
			effectiveCap = 0 // no cap
		}
	}
	return effectiveCap
}

// processInvertedMatch processes inverted match results for a file
func (e *Engine) processInvertedMatch(fileID types.FileID, path string, content []byte,
	patternBytes []byte, options types.SearchOptions, effectiveCap int, allResults *[]GrepResult) bool {

	if !options.InvertMatch {
		return false
	}

	// Find all matches
	matches := findAllMatchesWithOptions(content, patternBytes, options)

	// Build set of matching line numbers
	matchingLines := make(map[int]bool)
	for _, match := range matches {
		line := bytesToLine(content, match.Start)
		matchingLines[line] = true
	}

	lineCount := e.indexer.GetFileLineCount(fileID)

	// Create results for non-matching lines
	for lineNum := 1; lineNum <= lineCount; lineNum++ {
		// Skip matching lines (we want lines that DON'T match)
		if matchingLines[lineNum] {
			continue
		}

		// Get single line on demand (no allocation for unused lines)
		lineText, ok := e.indexer.GetFileLine(fileID, lineNum)
		if !ok {
			continue
		}

		// Create result for non-matching line
		result := GrepResult{
			FileID: fileID,
			Path:   path,
			Line:   lineNum,
			Column: 0,
			Match:  lineText,
			Score:  1.0,
			Context: ExtractedContext{
				StartLine:    lineNum,
				EndLine:      lineNum,
				Lines:        []string{lineText},
				MatchedLines: []int{lineNum},
				MatchCount:   1,
			},
		}
		*allResults = append(*allResults, result)

		// Apply result cap for inverted match too
		if effectiveCap > 0 && len(*allResults) >= effectiveCap {
			return true // Signal early termination
		}
	}

	return true // Processed inverted match
}

// applyMatchLimits applies per-file match limits
func (e *Engine) applyMatchLimits(matches []Match, options types.SearchOptions) []Match {
	// Apply MaxCountPerFile if set (grep -m)
	if options.MaxCountPerFile > 0 && len(matches) > options.MaxCountPerFile {
		return matches[:options.MaxCountPerFile]
	}

	// Limit matches per file for performance (default behavior)
	const maxMatchesPerFile = 100
	if len(matches) > maxMatchesPerFile && !options.DeclarationOnly {
		return matches[:maxMatchesPerFile]
	}

	return matches
}

// prepareFileInfo prepares FileInfo based on what's needed for processing
func (e *Engine) prepareFileInfo(fileID types.FileID, path string, content []byte, options types.SearchOptions) (*types.FileInfo, bool) {
	needsBlocks := options.FullFunction || options.EnsureCompleteStmt ||
		options.MaxContextLines == 0 || options.MaxContextLines > 10

	var fileInfo *types.FileInfo
	if needsBlocks {
		fileInfo = e.indexer.GetFileInfo(fileID)
		if fileInfo == nil {
			log.Printf("WARNING: GetFileInfo returned nil for fileID %d", fileID)
			return nil, false
		}
	} else {
		// Minimal FileInfo for semantic filtering only
		fileInfo = &types.FileInfo{
			ID:              fileID,
			Path:            path,
			Content:         content,
			EnhancedSymbols: e.indexer.GetFileEnhancedSymbols(fileID),
			LineToSymbols:   e.indexer.GetFileLineToSymbols(fileID), // Pre-computed for O(1) semantic filtering
		}
	}

	return fileInfo, true
}

// shouldMergeResults determines if results should be merged
func (e *Engine) shouldMergeResults(matches []Match, options types.SearchOptions) bool {
	return options.MergeFileResults && len(matches) > 1 &&
		!options.DeclarationOnly && !options.UsageOnly &&
		len(options.SymbolTypes) == 0 && !options.ExportedOnly &&
		!options.MutableOnly && !options.GlobalOnly
}

// processFile processes a single file for matches - extracted to reduce complexity
func (e *Engine) processFile(fileID types.FileID, patternBytes []byte, pattern string,
	options types.SearchOptions, effectiveCap int, allResults *[]GrepResult) {

	// Get file content
	content, ok := e.indexer.GetFileContent(fileID)
	if !ok {
		log.Printf("WARNING: file with ID %d not found in index - skipping (possible concurrent modification)", fileID)
		return
	}

	path := e.indexer.GetFilePath(fileID)

	// Handle inverted match
	if e.processInvertedMatch(fileID, path, content, patternBytes, options, effectiveCap, allResults) {
		return // Inverted match processed, skip normal processing
	}

	// Find all matches
	matches := findAllMatchesWithOptions(content, patternBytes, options)
	if len(matches) == 0 {
		return
	}

	// Apply match limits
	matches = e.applyMatchLimits(matches, options)

	// Prepare file info
	fileInfo, ok := e.prepareFileInfo(fileID, path, content, options)
	if !ok {
		return
	}

	// Apply semantic filtering
	matches = e.applySemanticFiltering(fileInfo, matches, pattern, options)
	if len(matches) == 0 {
		return
	}

	// Process and append results
	if e.shouldMergeResults(matches, options) {
		results := e.mergeFileResults(fileInfo, matches, pattern, options)
		*allResults = append(*allResults, results...)
	} else {
		results := e.processIndividualMatches(fileInfo, matches, pattern, options)
		*allResults = append(*allResults, results...)
	}
}

// processIndividualMatches processes matches individually with deduplication
func (e *Engine) processIndividualMatches(fileInfo *types.FileInfo, matches []Match,
	pattern string, options types.SearchOptions) []GrepResult {

	var results []GrepResult
	seenLines := make(map[int]bool)

	for _, match := range matches {
		line := bytesToLine(fileInfo.Content, match.Start)

		// Skip if we've already processed this line
		if seenLines[line] {
			continue
		}
		seenLines[line] = true

		// Extract context
		needsEnhancedContext := options.FullFunction || options.EnsureCompleteStmt ||
			options.MaxContextLines == 0 || options.MaxContextLines > 10

		var context ExtractedContext
		if needsEnhancedContext {
			context = e.extractEnhancedContext(fileInfo, line, options)
		} else {
			context = e.extractSimpleContextWithOptions(fileInfo.ID, line, options)
		}

		// Extract the actual matched text
		matchText := ""
		if match.End > match.Start && match.End <= len(fileInfo.Content) {
			matchText = string(fileInfo.Content[match.Start:match.End])
		}

		// Create result
		result := GrepResult{
			FileID:  fileInfo.ID,
			Path:    fileInfo.Path,
			Line:    line,
			Column:  match.Start - lineStart(fileInfo.Content, match.Start),
			Match:   matchText,
			Context: context,
			Score:   e.scoreMatch(fileInfo, match, pattern, line),
		}

		results = append(results, result)
	}

	return results
}

// extractSimpleContextWithOptions extracts simple context around a match with search options
func (e *Engine) extractSimpleContextWithOptions(fileID types.FileID, line int, options types.SearchOptions) ExtractedContext {
	contextLines := 3
	if options.MaxContextLines > 0 && options.MaxContextLines < contextLines {
		contextLines = options.MaxContextLines
	}

	startLine := line - contextLines
	if startLine < 1 {
		startLine = 1
	}

	lineCount := e.indexer.GetFileLineCount(fileID)
	endLine := line + contextLines
	if endLine > lineCount {
		endLine = lineCount
	}

	// PERFORMANCE: Extract only the lines we need
	contextLinesSlice := e.indexer.GetFileLines(fileID, startLine, endLine)

	return ExtractedContext{
		StartLine:    startLine,
		EndLine:      endLine,
		Lines:        contextLinesSlice,
		MatchedLines: []int{line},
		MatchCount:   1,
	}
}

// extractEnhancedContext extracts enhanced context with function boundaries
func (e *Engine) extractEnhancedContext(fileInfo *types.FileInfo, line int, options types.SearchOptions) ExtractedContext {
	context := e.contextExtractor.ExtractWithSearchOptions(fileInfo, line, options)

	// Normalize function context: drop leading package line and trailing blank if present
	if options.FullFunction && len(context.Lines) > 0 {
		first := strings.TrimSpace(context.Lines[0])
		if strings.HasPrefix(first, "package ") {
			context.Lines = context.Lines[1:]
			context.StartLine++
		}
		if n := len(context.Lines); n > 0 && strings.TrimSpace(context.Lines[n-1]) == "" {
			context.Lines = context.Lines[:n-1]
			context.EndLine--
		}
	}

	// Clamp to containing function boundaries to avoid extra lines
	if options.FullFunction {
		for i := range fileInfo.Blocks {
			b := &fileInfo.Blocks[i]
			if (b.Type == types.BlockTypeFunction || b.Type == types.BlockTypeMethod) &&
				b.Start+1 <= line && b.End+1 >= line {
				funcStart := b.Start + 1
				funcEnd := b.End + 1

				if context.StartLine < funcStart {
					delta := funcStart - context.StartLine
					if delta < len(context.Lines) {
						context.Lines = context.Lines[delta:]
					}
					context.StartLine = funcStart
				}

				if context.EndLine > funcEnd {
					trim := context.EndLine - funcEnd
					if trim < len(context.Lines) {
						context.Lines = context.Lines[:len(context.Lines)-trim]
					}
					context.EndLine = funcEnd
				}
				break
			}
		}
	}

	// Add match tracking info
	context.MatchedLines = []int{line}
	context.MatchCount = 1

	// Enforce 100-line window for centered contexts when no explicit max provided
	if !options.FullFunction && options.MaxContextLines == 0 && len(context.Lines) > 100 {
		context.Lines = context.Lines[:100]
		context.EndLine = context.StartLine + 100 - 1
	}

	return context
}

// ====== End of helper functions ======

// SearchWithOptions performs a search with configurable options
// Refactored to reduce cyclomatic complexity from 52 to ~8
func (e *Engine) SearchWithOptions(pattern string, candidates []types.FileID, options types.SearchOptions) []GrepResult {
	// Step 1: Validate and prepare patterns
	searchPatterns, valid := e.validateAndPreparePatterns(pattern, options)
	if !valid {
		return nil
	}

	// Step 2: Handle multiple patterns case
	if len(searchPatterns) > 1 {
		return e.searchMultiplePatterns(searchPatterns, candidates, options)
	}

	// Step 3: Single pattern validation
	if pattern == "" {
		return nil
	}

	// Step 4: Prepare candidates
	candidates = e.prepareCandidates(pattern, candidates, options)
	if candidates == nil {
		return nil
	}

	// Step 4.1: Filter out deleted/invalidated files
	type deletedFileFilter interface {
		FilterDeletedFiles([]types.FileID) []types.FileID
	}
	if dff, ok := any(e.indexer).(deletedFileFilter); ok {
		candidates = dff.FilterDeletedFiles(candidates)
		if len(candidates) == 0 {
			return nil
		}
	}

	// Step 4.5: Optimize literal patterns (8x faster than regex!)
	// If pattern is marked as regex but contains no regex metacharacters, use literal search
	if options.UseRegex && isLiteralPattern(pattern) {
		options.UseRegex = false // Use fast literal search instead
	}

	// Step 5: Handle regex search (use cached regex engine for all regex searches)
	// Note: InvertMatch requires per-file processing, so skip hybrid engine for that case
	if options.UseRegex && !options.InvertMatch && len(candidates) >= 1 {
		return e.searchWithHybridRegex(pattern, candidates, options)
	}

	// Step 6: Determine effective result cap
	effectiveCap := e.getEffectiveResultCap(candidates, options)

	// Step 7: Process each file
	var allResults []GrepResult
	patternBytes := []byte(pattern)

	for _, fileID := range candidates {
		// Early termination check
		if effectiveCap > 0 && len(allResults) >= effectiveCap {
			break
		}

		// Process single file
		e.processFile(fileID, patternBytes, pattern, options, effectiveCap, &allResults)
	}

	// Step 8: Handle special output modes (Count Per File)
	if options.CountPerFile {
		fileCounts := make(map[types.FileID]int)
		for _, result := range allResults {
			fileCounts[result.FileID]++
		}

		// Convert to results showing counts
		var fileResults []GrepResult
		for fileID, count := range fileCounts {
			path := e.indexer.GetFilePath(fileID)
			fileResults = append(fileResults, GrepResult{
				FileID:         fileID,
				Path:           path,
				Line:           0, // No specific line in count mode
				Column:         0,
				Match:          strconv.Itoa(count),
				Score:          float64(count),
				FileMatchCount: count, // Store count in proper field
			})
		}

		// Sort by path for consistent output
		sort.Slice(fileResults, func(i, j int) bool {
			return fileResults[i].Path < fileResults[j].Path
		})

		return fileResults
	}

	// Step 9: Handle Files Only mode (grep -l)
	if options.FilesOnly {
		seenFiles := make(map[types.FileID]bool)
		var fileResults []GrepResult

		for _, result := range allResults {
			if !seenFiles[result.FileID] {
				seenFiles[result.FileID] = true
				// Return minimal result with just filename
				fileResults = append(fileResults, GrepResult{
					FileID:  result.FileID,
					Path:    result.Path,
					Line:    0,                  // No line number in files-only mode
					Score:   1.0,                // Default score
					Context: ExtractedContext{}, // No context
				})
			}
		}

		// Sort by path for consistent output
		sort.Slice(fileResults, func(i, j int) bool {
			return fileResults[i].Path < fileResults[j].Path
		})

		return fileResults
	}

	// Step 10: Sort results by score (normal mode)
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	return allResults
}

// SearchDetailed performs a detailed search with relational data
func (e *Engine) SearchDetailed(pattern string, candidates []types.FileID, maxContextLines int) []StandardResult {
	return e.SearchDetailedWithOptions(pattern, candidates, types.SearchOptions{
		MaxContextLines: maxContextLines,
	})
}

func (e *Engine) SearchDetailedWithOptions(pattern string, candidates []types.FileID, options types.SearchOptions) []StandardResult {
	// Get basic search results first
	results := e.SearchWithOptions(pattern, candidates, options)

	// Create caches to avoid redundant lookups
	fileScopesCache := make(map[types.FileID][]types.ScopeInfo)
	fileInfoCache := make(map[types.FileID]*types.FileInfo)
	symbolCache := make(map[types.SymbolID]*types.EnhancedSymbol)

	// Enhance each result with relational data
	var detailedResults []StandardResult
	for _, result := range results {
		// Provide same quality of results for everything
		detailed := e.enhanceResult(result, pattern, fileScopesCache, fileInfoCache, symbolCache)
		detailedResults = append(detailedResults, detailed)
	}

	// Populate dense object IDs if enabled
	if options.IncludeObjectIDs {
		searchtypes.PopulateDenseObjectIDs(detailedResults)
	}

	return detailedResults
}

// enhanceResult adds relational data with caching to avoid redundant lookups
func (e *Engine) enhanceResult(
	result GrepResult,
	pattern string,
	fileScopesCache map[types.FileID][]types.ScopeInfo,
	fileInfoCache map[types.FileID]*types.FileInfo,
	symbolCache map[types.SymbolID]*types.EnhancedSymbol,
) StandardResult {
	detailed := StandardResult{
		Result: result,
	}

	// Get file information (cached)
	fileInfo, exists := fileInfoCache[result.FileID]
	if !exists {
		fileInfo = e.indexer.GetFileInfo(result.FileID)
		if fileInfo != nil {
			fileInfoCache[result.FileID] = fileInfo
		}
	}
	if fileInfo == nil {
		// File not found - return basic result without enhancement
		log.Printf("WARNING: file with ID %d not found while enhancing result - skipping enhancement", result.FileID)
		return detailed
	}

	// Find the enhanced symbol at this location using optimized lookup
	matchedSymbol := e.indexer.GetEnhancedSymbolAtLine(result.FileID, result.Line)

	// Build relational context
	relationalContext := &types.RelationalContext{}

	if matchedSymbol != nil {
		// Use the actual enhanced symbol with reference counts
		relationalContext.Symbol = *matchedSymbol
		relationalContext.RefStats = matchedSymbol.RefStats
	} else {
		// For non-symbol matches (like comments), show zero references
		relationalContext.RefStats = types.RefStats{
			Total: types.RefCount{
				IncomingCount: 0,
				OutgoingCount: 0,
			},
		}
	}

	// Get scope hierarchy (cached)
	allScopes, exists := fileScopesCache[result.FileID]
	if !exists {
		allScopes = e.indexer.GetFileScopeInfo(result.FileID)
		fileScopesCache[result.FileID] = allScopes
	}

	// Build scope breadcrumbs (only for scopes containing this result)
	var breadcrumbs []types.ScopeInfo
	for _, scope := range allScopes {
		if scope.StartLine <= result.Line && (scope.EndLine == 0 || scope.EndLine >= result.Line) {
			enrichedScope := scope

			// Find enhanced symbol for this scope using optimized lookup
			scopeSymbol := e.indexer.GetEnhancedSymbolAtLine(result.FileID, scope.StartLine)
			if scopeSymbol != nil && scopeSymbol.Name == scope.Name {
				// Add reference count attributes from the enhanced symbol
				refAttr := types.ContextAttribute{
					Type:  types.AttrTypeExported,
					Value: fmt.Sprintf("%d↑ %d↓", scopeSymbol.RefStats.Total.IncomingCount, scopeSymbol.RefStats.Total.OutgoingCount),
				}
				enrichedScope.Attributes = append(enrichedScope.Attributes, refAttr)
			}
			breadcrumbs = append(breadcrumbs, enrichedScope)
		}
	}
	relationalContext.Breadcrumbs = breadcrumbs

	// Find related symbols (limit lookups with caching)
	if matchedSymbol != nil {
		var relatedSymbols []types.RelatedSymbol

		// Process incoming refs (limit to 3)
		refCount := 0
		for _, ref := range matchedSymbol.IncomingRefs {
			if refCount >= 3 {
				break
			}
			if ref.SourceSymbol != 0 {
				// Check cache first
				relatedSymbol, exists := symbolCache[ref.SourceSymbol]
				if !exists {
					relatedSymbol = e.indexer.GetEnhancedSymbol(ref.SourceSymbol)
					if relatedSymbol != nil {
						symbolCache[ref.SourceSymbol] = relatedSymbol
					}
				}

				if relatedSymbol != nil {
					// Get file info from cache
					relatedFileInfo, exists := fileInfoCache[relatedSymbol.FileID]
					if !exists {
						relatedFileInfo = e.indexer.GetFileInfo(relatedSymbol.FileID)
						if relatedFileInfo != nil {
							fileInfoCache[relatedSymbol.FileID] = relatedFileInfo
						}
					}

					var fileName string
					if relatedFileInfo != nil {
						fileName = filepath.Base(relatedFileInfo.Path)
					}

					relatedSymbols = append(relatedSymbols, types.RelatedSymbol{
						Symbol:   *relatedSymbol,
						Relation: types.RelationCaller,
						Strength: ref.Strength,
						Distance: 1,
						FileName: fileName,
					})
					refCount++
				}
			}
		}

		// Process outgoing refs (limit to 2)
		refCount = 0
		for _, ref := range matchedSymbol.OutgoingRefs {
			if refCount >= 2 {
				break
			}
			if ref.TargetSymbol != 0 {
				// Check cache first
				relatedSymbol, exists := symbolCache[ref.TargetSymbol]
				if !exists {
					relatedSymbol = e.indexer.GetEnhancedSymbol(ref.TargetSymbol)
					if relatedSymbol != nil {
						symbolCache[ref.TargetSymbol] = relatedSymbol
					}
				}

				if relatedSymbol != nil {
					// Get file info from cache
					relatedFileInfo, exists := fileInfoCache[relatedSymbol.FileID]
					if !exists {
						relatedFileInfo = e.indexer.GetFileInfo(relatedSymbol.FileID)
						if relatedFileInfo != nil {
							fileInfoCache[relatedSymbol.FileID] = relatedFileInfo
						}
					}

					var fileName string
					if relatedFileInfo != nil {
						fileName = filepath.Base(relatedFileInfo.Path)
					}

					relatedSymbols = append(relatedSymbols, types.RelatedSymbol{
						Symbol:   *relatedSymbol,
						Relation: types.RelationCallee,
						Strength: ref.Strength,
						Distance: 1,
						FileName: fileName,
					})
					refCount++
				}
			}
		}

		relationalContext.RelatedSymbols = relatedSymbols
	}

	detailed.RelationalData = relationalContext
	return detailed
}

func (e *Engine) scoreMatch(file *types.FileInfo, match Match, pattern string, line int) float64 {
	score := 10.0

	// HUGE boost for exact matches - these are what the user is looking for
	if match.Exact {
		score += 100.0
	}

	// Extract the line using precomputed offsets (O(1) instead of O(n) string split)
	lineBytes := types.GetLineFromOffsets(file.Content, file.LineOffsets, line)
	currentLine := strings.TrimSpace(string(lineBytes))

	// MASSIVE bonus for the actual definition - the thing being searched for
	if strings.HasPrefix(currentLine, "func ") && strings.Contains(currentLine, pattern) {
		score += 1000.0
	}

	// Symbol definition bonus - definitions should always rank highest
	hasSymbolOnLine := false
	for _, symbol := range file.EnhancedSymbols {
		if symbol.Line == line {
			hasSymbolOnLine = true
			score += 500.0 // Major bonus for any symbol definition
			if symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeClass {
				score += 200.0 // Extra for functions/classes
			}
			break
		}
	}

	// Property weight propagation using reference graph
	// Boost results based on symbol importance derived from reference counts
	// This implements a simplified PageRank-like scoring system

	// Boost non-comment code lines
	if !strings.HasPrefix(currentLine, "//") {
		score += 1.0

		// Additional boost based on reference graph connectivity
		// Symbols with more references are more "important"
		if e.indexer != nil {
			type refCounter interface {
				GetSymbolAtLine(types.FileID, int) *types.EnhancedSymbol
			}
			if rc, ok := any(e.indexer).(refCounter); ok {
				fileID := e.getFileIDFromPath(file.Path)
				if sym := rc.GetSymbolAtLine(fileID, line); sym != nil {
					// Weight based on incoming references (how many use this symbol)
					incomingBoost := float64(sym.RefStats.Total.IncomingCount) * 0.1
					// Weight based on outgoing references (symbol complexity/connectivity)
					outgoingBoost := float64(sym.RefStats.Total.OutgoingCount) * 0.05
					// Cap the boost to prevent dominance
					totalBoost := math.Min(incomingBoost+outgoingBoost, 10.0)
					score += totalBoost
				}
			}
		}
	}

	// Penalize comment lines - they shouldn't rank high unless it's the only match
	if strings.HasPrefix(currentLine, "//") {
		score -= 5.0
	}

	// File path relevance
	if strings.Contains(strings.ToLower(file.Path), strings.ToLower(pattern)) {
		score += 3.0
	}

	// Prefer shorter paths (likely more important files)
	pathDepth := strings.Count(file.Path, string(filepath.Separator))
	score -= float64(pathDepth) * 0.5

	// Test file filtering should be done at index/config level, not in search engine
	// Files can be excluded via Include/Exclude patterns in config

	// Apply file type and symbol presence ranking from config
	rankingCfg := e.getRankingConfig()
	if rankingCfg.Enabled {
		// Boost code files, penalize documentation files
		score += e.scoreFileType(file.Path, rankingCfg)
		// Penalize matches without symbol definitions
		score += e.scoreSymbolPresence(hasSymbolOnLine, rankingCfg)
	}

	return score
}

// applySemanticFiltering filters matches based on semantic criteria
// OPTIMIZED: Uses pre-computed LineToSymbols from indexing to avoid O(matches*symbols) complexity
// Eliminates 1.1GB allocation by reusing index computed during map phase
func (e *Engine) applySemanticFiltering(fileInfo *types.FileInfo, matches []Match, pattern string, options types.SearchOptions) []Match {
	if len(options.SymbolTypes) == 0 && !options.DeclarationOnly && !options.UsageOnly &&
		!options.ExportedOnly && !options.ExcludeTests && !options.ExcludeComments &&
		!options.MutableOnly && !options.GlobalOnly {
		return matches // No semantic filtering requested
	}

	// Use pre-computed index if available (from map phase), otherwise build on-demand as fallback
	lineToSymbols := fileInfo.LineToSymbols
	if lineToSymbols == nil && len(fileInfo.EnhancedSymbols) > 0 {
		// Fallback: build index on-demand (for files indexed before this optimization)
		lineToSymbols = make(map[int][]int, len(fileInfo.EnhancedSymbols))
		for i, symbol := range fileInfo.EnhancedSymbols {
			lineToSymbols[symbol.Line] = append(lineToSymbols[symbol.Line], i)
		}
	}

	// Pre-allocate result slice with reasonable capacity
	filteredMatches := make([]Match, 0, len(matches)/2+1)

	for _, match := range matches {
		if e.passesSemanticFilter(fileInfo, match, pattern, options, lineToSymbols) {
			filteredMatches = append(filteredMatches, match)
		}
	}

	return filteredMatches
}

// passesSemanticFilter checks if a match passes semantic filtering using pre-built line index
func (e *Engine) passesSemanticFilter(fileInfo *types.FileInfo, match Match, pattern string, options types.SearchOptions, lineToSymbols map[int][]int) bool {
	line := bytesToLine(fileInfo.Content, match.Start)

	// Check if in comment (if ExcludeComments is enabled)
	if options.ExcludeComments && e.isInComment(fileInfo, line) {
		return false
	}

	// Find symbol at this location using the pre-built index
	var matchingSymbol *types.EnhancedSymbol

	// Get symbols on this line from index (O(1) lookup instead of O(n) scan)
	symbolIndices := lineToSymbols[line]

	// Try to find a symbol that matches the pattern on this line using semantic scoring
	for _, idx := range symbolIndices {
		symbol := fileInfo.EnhancedSymbols[idx]
		// Use semantic scoring for advanced matching (camelCase, fuzzy, stemming, etc.)
		var matches bool
		if e.semanticScorer != nil {
			// Use semantic scoring layers (name splitting, fuzzy matching, etc.)
			score := e.semanticScorer.ScoreSymbol(pattern, symbol.Name)
			// Use the semantic scorer's configured MinScore threshold
			matches = score.Score >= e.semanticScorer.GetConfig().MinScore
		} else {
			// Fallback to simple string containment for backward compatibility
			matches = strings.Contains(symbol.Name, pattern) || strings.Contains(pattern, symbol.Name)
		}

		if matches {
			matchingSymbol = symbol
			break
		}
	}

	// If no match found, use first symbol on this line (for DeclarationOnly/UsageOnly cases)
	if matchingSymbol == nil && len(symbolIndices) > 0 {
		matchingSymbol = fileInfo.EnhancedSymbols[symbolIndices[0]]
	}

	// Apply symbol-based filters
	if matchingSymbol != nil {
		// Filter by symbol types
		if len(options.SymbolTypes) > 0 {
			// Use the String() method to get proper string representation
			if !contains(options.SymbolTypes, matchingSymbol.Type.String()) {
				return false
			}
		}

		// Declaration only filter
		if options.DeclarationOnly {
			// Only show symbols at their definition location
			return true
		}

		// Exported only filter
		if options.ExportedOnly && !e.isExportedEnhancedSymbol(matchingSymbol, fileInfo) {
			return false
		}

		// Mutable only filter
		if options.MutableOnly && !e.isMutableSymbol(matchingSymbol, fileInfo, line) {
			return false
		}

		// Global only filter
		if options.GlobalOnly && !e.isGlobalSymbol(matchingSymbol, fileInfo) {
			return false
		}
	} else {
		// No symbol found at this location - this is likely a usage
		// If SymbolTypes is specified, we need a symbol to filter, so reject
		if len(options.SymbolTypes) > 0 {
			return false
		}

		if options.DeclarationOnly {
			return false // Only want declarations, this is usage
		}

		// For usage-only filter, allow if we couldn't find a symbol (likely usage)
		if options.UsageOnly {
			return true
		}
	}

	return true
}

// GetLine returns a single line from file content using LineOffsets for O(1) access.
// Returns empty string if line number is out of range. Line numbers are 1-based.
// Implements LineProvider interface.
func (e *Engine) GetLine(fileInfo *types.FileInfo, lineNum int) string {
	if lineNum < 1 {
		return ""
	}

	// Use LineOffsets from FileInfo if available
	if len(fileInfo.LineOffsets) > 0 {
		lineBytes := types.GetLineFromOffsets(fileInfo.Content, fileInfo.LineOffsets, lineNum)
		if lineBytes != nil {
			return string(lineBytes)
		}
		return ""
	}

	// Use LineOffsets from indexer (FileContentStore) - returns []uint32
	if offsets, ok := e.indexer.GetFileLineOffsets(fileInfo.ID); ok && len(offsets) > 0 {
		content := fileInfo.Content
		if len(content) == 0 {
			if contentBytes, ok := e.indexer.GetFileContent(fileInfo.ID); ok {
				content = contentBytes
			}
		}
		if len(content) > 0 {
			lineIdx := lineNum - 1
			if lineIdx >= 0 && lineIdx < len(offsets) {
				start := int(offsets[lineIdx])
				end := len(content)
				if lineIdx+1 < len(offsets) {
					end = int(offsets[lineIdx+1])
					if end > start && content[end-1] == '\n' {
						end--
					}
				}
				if start < len(content) && start <= end {
					return string(content[start:end])
				}
			}
		}
		return ""
	}

	// Fallback: scan content for the requested line
	content := fileInfo.Content
	if len(content) == 0 {
		return ""
	}

	currentLine := 1
	start := 0
	for i, b := range content {
		if b == '\n' {
			if currentLine == lineNum {
				return string(content[start:i])
			}
			currentLine++
			start = i + 1
		}
	}
	// Handle last line (no trailing newline)
	if currentLine == lineNum && start < len(content) {
		return string(content[start:])
	}
	return ""
}

// GetLineCount returns the number of lines in the file content.
// Implements LineProvider interface.
func (e *Engine) GetLineCount(fileInfo *types.FileInfo) int {
	// Use LineOffsets from FileInfo if available
	if len(fileInfo.LineOffsets) > 0 {
		return len(fileInfo.LineOffsets)
	}

	// Use LineOffsets from indexer
	if offsets, ok := e.indexer.GetFileLineOffsets(fileInfo.ID); ok && len(offsets) > 0 {
		return len(offsets)
	}

	// Fallback: count newlines
	content := fileInfo.Content
	if len(content) == 0 {
		return 0
	}
	count := 1
	for _, b := range content {
		if b == '\n' {
			count++
		}
	}
	return count
}

// GetLineRange returns lines from startLine to endLine (inclusive, 1-based).
// Implements LineProvider interface.
func (e *Engine) GetLineRange(fileInfo *types.FileInfo, startLine, endLine int) []string {
	if startLine < 1 {
		startLine = 1
	}
	lineCount := e.GetLineCount(fileInfo)
	if endLine > lineCount {
		endLine = lineCount
	}
	if startLine > endLine {
		return nil
	}

	result := make([]string, 0, endLine-startLine+1)
	for i := startLine; i <= endLine; i++ {
		result = append(result, e.GetLine(fileInfo, i))
	}
	return result
}

// getLineRef returns a ZeroAllocStringRef for the given 1-based line number,
// avoiding the string allocation that GetLine performs. Uses the same resolution
// order as GetLine: FileInfo.LineOffsets → indexer offsets → linear scan.
func (e *Engine) getLineRef(fileInfo *types.FileInfo, lineNum int) types.ZeroAllocStringRef {
	if lineNum < 1 {
		return types.EmptyZeroAllocStringRef
	}

	// Helper to build a ref from content + start/end byte offsets
	makeRef := func(content []byte, start, end int) types.ZeroAllocStringRef {
		if start >= len(content) || start > end {
			return types.EmptyZeroAllocStringRef
		}
		if end > len(content) {
			end = len(content)
		}
		return types.ZeroAllocStringRef{
			Data:   content,
			FileID: fileInfo.ID,
			Offset: uint32(start),
			Length: uint32(end - start),
			Hash:   types.ComputeHash(content[start:end]),
		}
	}

	// Path 1: FileInfo.LineOffsets ([]int, 0-based index, 1-based lineNum)
	if len(fileInfo.LineOffsets) > 0 {
		lineIdx := lineNum - 1
		if lineIdx < 0 || lineIdx >= len(fileInfo.LineOffsets) {
			return types.EmptyZeroAllocStringRef
		}
		start := fileInfo.LineOffsets[lineIdx]
		end := len(fileInfo.Content)
		if lineIdx+1 < len(fileInfo.LineOffsets) {
			end = fileInfo.LineOffsets[lineIdx+1]
			if end > start && fileInfo.Content[end-1] == '\n' {
				end--
			}
		}
		return makeRef(fileInfo.Content, start, end)
	}

	// Path 2: indexer offsets ([]uint32)
	if offsets, ok := e.indexer.GetFileLineOffsets(fileInfo.ID); ok && len(offsets) > 0 {
		content := fileInfo.Content
		if len(content) == 0 {
			if contentBytes, ok := e.indexer.GetFileContent(fileInfo.ID); ok {
				content = contentBytes
			}
		}
		if len(content) > 0 {
			lineIdx := lineNum - 1
			if lineIdx >= 0 && lineIdx < len(offsets) {
				start := int(offsets[lineIdx])
				end := len(content)
				if lineIdx+1 < len(offsets) {
					end = int(offsets[lineIdx+1])
					if end > start && content[end-1] == '\n' {
						end--
					}
				}
				return makeRef(content, start, end)
			}
		}
		return types.EmptyZeroAllocStringRef
	}

	// Path 3: linear scan fallback
	content := fileInfo.Content
	if len(content) == 0 {
		return types.EmptyZeroAllocStringRef
	}
	currentLine := 1
	start := 0
	for i, b := range content {
		if b == '\n' {
			if currentLine == lineNum {
				return makeRef(content, start, i)
			}
			currentLine++
			start = i + 1
		}
	}
	if currentLine == lineNum && start < len(content) {
		return makeRef(content, start, len(content))
	}
	return types.EmptyZeroAllocStringRef
}

func (e *Engine) isInComment(fileInfo *types.FileInfo, line int) bool {
	lineRef := e.getLineRef(fileInfo, line)
	if lineRef.IsEmpty() {
		return false
	}

	trimmed := lineRef.TrimSpace()
	return trimmed.HasAnyPrefix("//", "#", "/*") || trimmed.Contains("*/")
}

func (e *Engine) isExportedEnhancedSymbol(symbol *types.EnhancedSymbol, fileInfo *types.FileInfo) bool {
	// Use pre-computed IsExported flag if available
	if symbol.IsExported {
		return true
	}

	// Check if symbol name starts with uppercase (Go convention)
	if len(symbol.Name) > 0 && symbol.Name[0] >= 'A' && symbol.Name[0] <= 'Z' {
		return true
	}

	// Check for export keywords
	if line := symbol.Line; line > 0 {
		lineRef := e.getLineRef(fileInfo, line)
		if !lineRef.IsEmpty() {
			return lineRef.ContainsAny("export ", "public ", "pub ")
		}
	}

	return false
}

func (e *Engine) isMutableSymbol(symbol *types.EnhancedSymbol, fileInfo *types.FileInfo, line int) bool {
	// Use pre-computed IsMutable flag if available
	if symbol.IsMutable {
		return true
	}

	if symbol.Type != types.SymbolTypeVariable {
		return false // Only variables can be mutable
	}

	if line > 0 {
		lineRef := e.getLineRef(fileInfo, line)
		if !lineRef.IsEmpty() {
			// Check for mutable variable declarations
			return lineRef.ContainsAny("var ", "let ") ||
				(!lineRef.ContainsAny("const ", "final "))
		}
	}

	return false
}

func (e *Engine) isGlobalSymbol(symbol *types.EnhancedSymbol, fileInfo *types.FileInfo) bool {
	// Check the VariableType field first
	if symbol.VariableType == types.VariableTypeGlobal {
		return true
	}
	// Simple heuristic: global symbols are typically at the top level (less indentation)
	if line := symbol.Line; line > 0 {
		lineRef := e.getLineRef(fileInfo, line)
		if !lineRef.IsEmpty() {
			trimmedLeft := lineRef.TrimLeft(" \t")
			// No indentation = top level (no leading whitespace removed)
			return lineRef.Len() == trimmedLeft.Len()
		}
	}

	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// mergeFileResults combines multiple matches from the same file into fewer, more comprehensive results
// Algorithm:
// 1. Every line has a +- 2 line window
// 2. The first result in a file is expanded to the function that contains it (if it is a function)
// 3. Any results in that range are removed
// 4. Overlapping regions are joined iteratively until every match has been merged or checked
func (e *Engine) mergeFileResults(fileInfo *types.FileInfo, matches []Match, pattern string, options types.SearchOptions) []GrepResult {
	if len(matches) == 0 {
		return nil
	}

	// Step 1: Convert matches to line-based results with initial +/- 2 line windows
	type LineRange struct {
		StartLine   int
		EndLine     int
		MatchLine   int     // Original match line
		MatchColumn int     // Original match column
		Score       float64 // Match score
		IsFunction  bool    // Whether this range represents a function
		AllMatches  []Match // Track ALL matches in this range
	}

	var ranges []LineRange
	for _, match := range matches {
		line := bytesToLine(fileInfo.Content, match.Start)
		column := match.Start - lineStart(fileInfo.Content, match.Start)
		score := e.scoreMatch(fileInfo, match, pattern, line)

		// Initial range is match line +/- 2
		startLine := line - 2
		if startLine < 1 {
			startLine = 1
		}
		lineCount := e.indexer.GetFileLineCount(fileInfo.ID)
		endLine := line + 2
		if endLine > lineCount {
			endLine = lineCount
		}

		// Try to expand to containing function for any match inside a function
		isFunction := false
		// Try to find the symbol at this location
		symbol := e.indexer.GetSymbolAtLine(fileInfo.ID, line)
		if symbol != nil && (symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeMethod) {
			// Expand if the match is anywhere within the function (including declaration line)
			if line >= symbol.Line && line <= symbol.EndLine {
				// Expand to function boundaries
				startLine = symbol.Line
				endLine = symbol.EndLine
				isFunction = true
			}
		}

		ranges = append(ranges, LineRange{
			StartLine:   startLine,
			EndLine:     endLine,
			MatchLine:   line,
			MatchColumn: column,
			Score:       score,
			IsFunction:  isFunction,
			AllMatches:  []Match{match}, // Start with this match
		})
	}

	// Step 2: Remove any ranges that are completely contained within function ranges
	// but merge their matches into the function range
	functionRanges := make(map[int]*LineRange)
	for i := range ranges {
		if ranges[i].IsFunction {
			functionRanges[i] = &ranges[i]
		}
	}

	var filteredRanges []LineRange
	for i, r := range ranges {
		contained := false
		// Check if this range is contained within any function range
		for j, fr := range functionRanges {
			if i != j && !r.IsFunction &&
				r.StartLine >= fr.StartLine && r.EndLine <= fr.EndLine {
				// Merge the matches from the contained range into the function range
				fr.AllMatches = append(fr.AllMatches, r.AllMatches...)
				contained = true
				break
			}
		}
		if !contained {
			filteredRanges = append(filteredRanges, r)
		}
	}
	ranges = filteredRanges

	// Sort ranges by start line for proper merging
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].StartLine < ranges[j].StartLine
	})

	// Step 3: Iteratively merge overlapping ranges
	for {
		merged := false
		var newRanges []LineRange

		for i := 0; i < len(ranges); i++ {
			if i > 0 {
				prevRange := &newRanges[len(newRanges)-1]
				currRange := ranges[i]

				// Check if ranges overlap or are adjacent (within 1 line)
				if currRange.StartLine <= prevRange.EndLine+1 {
					// Merge ranges
					if currRange.EndLine > prevRange.EndLine {
						prevRange.EndLine = currRange.EndLine
					}
					// Keep the better score and position
					if currRange.Score > prevRange.Score {
						prevRange.MatchLine = currRange.MatchLine
						prevRange.MatchColumn = currRange.MatchColumn
						prevRange.Score = currRange.Score
					}
					// If either is a function, the merged range is a function
					if currRange.IsFunction {
						prevRange.IsFunction = true
					}
					// Merge all matches from both ranges
					prevRange.AllMatches = append(prevRange.AllMatches, currRange.AllMatches...)
					merged = true
					continue
				}
			}
			newRanges = append(newRanges, ranges[i])
		}

		ranges = newRanges
		if !merged {
			break
		}
	}

	// Step 4: Convert ranges back to Results with proper context extraction
	var results []GrepResult
	for _, r := range ranges {
		// Calculate which lines have matches and total match count
		matchedLines := make(map[int]bool)
		for _, match := range r.AllMatches {
			line := bytesToLine(fileInfo.Content, match.Start)
			matchedLines[line] = true
		}

		// Convert map to sorted slice
		var matchedLinesList []int
		for line := range matchedLines {
			matchedLinesList = append(matchedLinesList, line)
		}
		sort.Ints(matchedLinesList)

		// Extract context for the full range
		context := ExtractedContext{
			StartLine:    r.StartLine,
			EndLine:      r.EndLine,
			Lines:        make([]string, 0, r.EndLine-r.StartLine+1),
			MatchedLines: matchedLinesList,
			MatchCount:   len(r.AllMatches),
		}

		// Build the context lines using lightweight accessor
		lineCount := e.indexer.GetFileLineCount(fileInfo.ID)
		endLine := r.EndLine
		if endLine > lineCount {
			endLine = lineCount
		}
		context.Lines = e.indexer.GetFileLines(fileInfo.ID, r.StartLine, endLine)

		// Extract the match text from the original line
		matchText := ""
		if r.MatchLine > 0 && r.MatchLine <= lineCount {
			line, ok := e.indexer.GetFileLine(fileInfo.ID, r.MatchLine)
			if ok {
				// Find the pattern in the line to extract the exact match
				if idx := strings.Index(strings.ToLower(line), strings.ToLower(pattern)); idx >= 0 {
					// Extract the actual match preserving original case
					matchText = line[idx : idx+len(pattern)]
				}
			}
		}

		result := GrepResult{
			FileID:  fileInfo.ID,
			Path:    fileInfo.Path,
			Line:    r.MatchLine,
			Column:  r.MatchColumn,
			Match:   matchText,
			Context: context,
			Score:   r.Score,
		}

		results = append(results, result)
	}

	return results
}

// searchMultiplePatterns searches for multiple patterns with OR logic (grep -e pattern1 -e pattern2)
func (e *Engine) searchMultiplePatterns(patterns []string, candidates []types.FileID, options types.SearchOptions) []GrepResult {
	if len(candidates) == 0 {
		candidates = e.indexer.GetAllFileIDs()
		if len(candidates) == 0 {
			return nil
		}
	}

	// Apply path-based filters
	candidates = e.filterIncludedFiles(candidates, options.IncludePattern)
	candidates = e.filterExcludedFiles(candidates, options.ExcludePattern)

	var allResults []GrepResult
	matchedLines := make(map[types.FileID]map[int]bool) // Track matched lines to avoid duplicates

	// Search each pattern across all files
	for _, pat := range patterns {
		if pat == "" {
			continue
		}

		patBytes := []byte(pat)

		for _, fileID := range candidates {
			fileInfo := e.indexer.GetFileInfo(fileID)
			if fileInfo == nil {
				continue
			}

			// Initialize line tracking for this file
			if matchedLines[fileID] == nil {
				matchedLines[fileID] = make(map[int]bool)
			}

			// Find all matches for this pattern
			matches := findAllMatchesWithOptions(fileInfo.Content, patBytes, options)

			// Apply semantic filtering
			matches = e.applySemanticFiltering(fileInfo, matches, pat, options)

			// Convert matches to results, avoiding duplicate lines
			for _, match := range matches {
				line := bytesToLine(fileInfo.Content, match.Start)

				// Skip if we already have a result for this line
				if matchedLines[fileID][line] {
					continue
				}
				matchedLines[fileID][line] = true

				column := match.Start - lineStart(fileInfo.Content, match.Start)
				score := e.scoreMatch(fileInfo, match, pat, line)

				// Extract context
				context := e.extractSimpleContext(fileInfo.Content, match.Start, match.End)

				result := GrepResult{
					FileID:  fileID,
					Path:    fileInfo.Path,
					Line:    line,
					Column:  column,
					Match:   string(fileInfo.Content[match.Start:match.End]),
					Context: context,
					Score:   score,
				}

				allResults = append(allResults, result)
			}
		}
	}

	// Sort results by file and line
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].FileID != allResults[j].FileID {
			return allResults[i].FileID < allResults[j].FileID
		}
		return allResults[i].Line < allResults[j].Line
	})

	return allResults
}

// findContainingBlock finds the smallest block containing the given line
func (e *Engine) findContainingBlock(fileInfo *types.FileInfo, matchLine int) *types.BlockBoundary {
	var containingBlock *types.BlockBoundary

	for i := range fileInfo.Blocks {
		block := &fileInfo.Blocks[i]
		// Tree-sitter lines are 0-based, our lines are 1-based
		if block.Start+1 <= matchLine && block.End+1 >= matchLine {
			if containingBlock == nil ||
				(block.End-block.Start) < (containingBlock.End-containingBlock.Start) {
				containingBlock = block
			}
		}
	}

	return containingBlock
}

// createMergedResult creates a single result representing multiple matches within the same block
func (e *Engine) createMergedResult(fileInfo *types.FileInfo, block *types.BlockBoundary, matches []Match, pattern string, options types.SearchOptions) GrepResult {
	// Find the best match (highest scoring) to use as the primary result
	var bestMatch Match
	var bestLine int
	bestScore := -1.0

	for _, match := range matches {
		line := bytesToLine(fileInfo.Content, match.Start)
		score := e.scoreMatch(fileInfo, match, pattern, line)
		if score > bestScore {
			bestScore = score
			bestMatch = match
			bestLine = line
		}
	}

	// Extract the complete block with enhanced context using the best match line
	context := e.contextExtractor.extractBlockContextEnhanced(fileInfo, bestLine, options.EnsureCompleteStmt)

	// Calculate combined score (average of all matches with bonus for multiple matches)
	totalScore := 0.0
	for _, match := range matches {
		line := bytesToLine(fileInfo.Content, match.Start)
		totalScore += e.scoreMatch(fileInfo, match, pattern, line)
	}
	avgScore := totalScore / float64(len(matches))
	// Bonus for multiple matches in same context
	bonusScore := avgScore + float64(len(matches)-1)*2.0

	// Extract the matched text from the best match
	matchText := string(fileInfo.Content[bestMatch.Start:bestMatch.End])

	return GrepResult{
		FileID:  fileInfo.ID,
		Path:    fileInfo.Path,
		Line:    bestLine,
		Column:  bestMatch.Start - lineStart(fileInfo.Content, bestMatch.Start),
		Match:   matchText,
		Context: context,
		Score:   bonusScore,
	}
}

// extractBlockContextEnhanced provides enhanced block context extraction with complete statement support

// isSimpleStatement determines if a match is a simple one-liner that doesn't need full block context
func (e *Engine) isSimpleStatement(fileInfo *types.FileInfo, matchLine int, block *types.BlockBoundary) bool {
	lineContent := e.GetLine(fileInfo, matchLine)
	if lineContent == "" {
		return true
	}

	line := strings.TrimSpace(lineContent)

	// Function declarations should NEVER be simple - always show full function body
	if strings.Contains(line, "func ") {
		return false
	}

	// Simple variable declarations, type declarations, imports
	if strings.Contains(line, "type ") && !strings.Contains(line, "{") {
		return true
	}
	if strings.Contains(line, "var ") {
		return true
	}
	if strings.Contains(line, "const ") {
		return true
	}
	if strings.HasPrefix(line, "import ") {
		return true
	}

	// Simple struct field declarations
	if block != nil && (block.Type == types.BlockTypeStruct || block.Type == types.BlockTypeInterface) {
		if !strings.Contains(line, "func") {
			return true
		}
	}

	// Single-line comments
	if strings.HasPrefix(line, "//") {
		return true
	}

	// Simple assignments or function calls on one line (shorter lines are more likely to be simple)
	if !strings.Contains(line, "{") && len(line) < 80 {
		return true
	}

	return false
}

// calculateBasicRefStats provides basic reference statistics by counting pattern occurrences
func (e *Engine) calculateBasicRefStats(pattern string) types.RefStats {
	// This is a simplified reference counting system
	// In a full implementation, this would use the actual reference tracking system

	// For now, just count how many files contain this pattern
	allFiles := e.getAllFileIDs()
	incomingCount := 0

	for _, fileID := range allFiles {
		fileInfo := e.indexer.GetFileInfo(fileID)
		if fileInfo == nil {
			// File not found - skip it and continue
			log.Printf("WARNING: file with ID %d not found while calculating ref stats - skipping", fileID)
			continue
		}

		// Simple string search to count references
		content := string(fileInfo.Content)
		if strings.Contains(strings.ToLower(content), strings.ToLower(pattern)) {
			incomingCount++
		}
	}

	// Create basic stats
	return types.RefStats{
		Total: types.RefCount{
			IncomingCount: max(0, incomingCount-1), // Subtract 1 to not count the definition file
			OutgoingCount: 0,                       // Would need more sophisticated analysis
		},
	}
}

// filterIncludedFiles keeps only files that match the include pattern
// Supports both glob patterns (*.go, internal/mcp/**/*.go) and regex patterns
func (e *Engine) filterIncludedFiles(candidates []types.FileID, includePattern string) []types.FileID {
	if includePattern == "" {
		return candidates
	}

	// Convert glob pattern to regex if it looks like a glob (contains *, ?, or [])
	pattern := includePattern
	if strings.ContainsAny(includePattern, "*?[]") {
		pattern = globToRegex(includePattern)
	}

	// Compile the pattern once for efficiency
	re, err := regexp.Compile(pattern)
	if err != nil {
		// If pattern is invalid, return nothing for safety (conservative)
		return nil
	}

	// Get project root for relative path calculation
	projectRoot := getProjectRoot(e.indexer)

	var filtered []types.FileID
	for _, fileID := range candidates {
		fileInfo := e.indexer.GetFileInfo(fileID)
		if fileInfo == nil {
			// File not found - skip it
			log.Printf("WARNING: file with ID %d not found while filtering included files - skipping", fileID)
			continue
		}

		// Try to match against relative path first, fall back to absolute path
		relativePath, err := filepath.Rel(projectRoot, fileInfo.Path)
		if err != nil {
			relativePath = fileInfo.Path
		}

		// Match against both relative and absolute paths for flexibility
		if re.MatchString(relativePath) || re.MatchString(fileInfo.Path) {
			filtered = append(filtered, fileID)
		}
	}
	return filtered
}

// filterExcludedFiles removes files that match the exclude pattern
// Supports both glob patterns (*.go, internal/mcp/**/*.go) and regex patterns
func (e *Engine) filterExcludedFiles(candidates []types.FileID, excludePattern string) []types.FileID {
	if excludePattern == "" {
		return candidates
	}

	// Convert glob pattern to regex if it looks like a glob (contains *, ?, or [])
	pattern := excludePattern
	if strings.ContainsAny(excludePattern, "*?[]") {
		pattern = globToRegex(excludePattern)
	}

	// Compile the pattern once for efficiency
	re, err := regexp.Compile(pattern)
	if err != nil {
		// If pattern is invalid, include all files (don't exclude due to bad pattern)
		return candidates
	}

	// Get project root for relative path calculation
	projectRoot := getProjectRoot(e.indexer)

	var filtered []types.FileID
	for _, fileID := range candidates {
		fileInfo := e.indexer.GetFileInfo(fileID)
		if fileInfo == nil {
			// File not found - skip it
			log.Printf("WARNING: file with ID %d not found while filtering excluded files - skipping", fileID)
			continue
		}

		// Try to match against relative path first, fall back to absolute path
		relativePath, err := filepath.Rel(projectRoot, fileInfo.Path)
		if err != nil {
			relativePath = fileInfo.Path
		}

		// Exclude if pattern matches against either relative or absolute paths
		if !re.MatchString(relativePath) && !re.MatchString(fileInfo.Path) {
			filtered = append(filtered, fileID)
		}
	}

	return filtered
}

// getAllFileIDs gets all file IDs (helper method)
func (e *Engine) getAllFileIDs() []types.FileID {
	return e.indexer.GetAllFileIDs()
}

// globToRegex converts a glob pattern to a regular expression
// Supports: * (any chars), ** (any dirs), ? (single char), [abc] (char class)
func globToRegex(glob string) string {
	// Escape special regex characters except glob wildcards
	pattern := regexp.QuoteMeta(glob)

	// Convert glob patterns to regex
	// Handle ** first (matches any directories)
	pattern = strings.ReplaceAll(pattern, `\*\*`, `.*`)
	// Then single * (matches within a path segment)
	pattern = strings.ReplaceAll(pattern, `\*`, `[^/]*`)
	// ? matches any single character except path separator
	pattern = strings.ReplaceAll(pattern, `\?`, `[^/]`)
	// Restore character classes
	pattern = strings.ReplaceAll(pattern, `\[`, `[`)
	pattern = strings.ReplaceAll(pattern, `\]`, `]`)

	// Anchor the pattern to match the full path
	return "^" + pattern + "$"
}

// getProjectRoot extracts the project root from the indexer
// Falls back to current directory if not available
func getProjectRoot(indexer interfaces.Indexer) string {
	// Try to cast to MasterIndex to access config
	type configProvider interface {
		GetConfig() *config.Config
	}

	if cp, ok := indexer.(configProvider); ok {
		if cfg := cp.GetConfig(); cfg != nil {
			return cfg.Project.Root
		}
	}

	// Fallback: use current directory
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}

	return "."
}

// File filtering should be done at index/config level using Include/Exclude patterns
// This allows users to customize what files are indexed based on their project needs

// SearchStats gathers statistical information about search results without returning the actual matches
func (e *Engine) SearchStats(pattern string, candidates []types.FileID, options types.SearchOptions) (*types.SearchStats, error) {
	startTime := time.Now()

	stats := &types.SearchStats{
		Pattern:          pattern,
		FileDistribution: make(map[string]int),
		DirDistribution:  make(map[string]int),
		SymbolTypes:      make(map[string]int),
		HotSpots:         make([]types.HotSpot, 0, 10), // Pre-allocate for typical hot spot count
	}

	// Use the existing Search method to get results
	results := e.SearchWithOptions(pattern, candidates, options)

	// Track hot spots
	type fileStats struct {
		path      string
		matches   int
		firstLine int
		lastLine  int
	}
	fileStatsMap := make(map[types.FileID]*fileStats)

	// Process results to gather statistics
	for _, result := range results {
		fileInfo := e.indexer.GetFileInfo(result.FileID)
		if fileInfo == nil {
			// File not found - skip this result in stats
			log.Printf("WARNING: file with ID %d not found while calculating search stats - skipping", result.FileID)
			continue
		}

		// Update file and directory counts
		if _, exists := stats.FileDistribution[fileInfo.Path]; !exists {
			stats.FilesWithMatches++
		}
		stats.TotalMatches++
		stats.FileDistribution[fileInfo.Path]++

		dir := filepath.Dir(fileInfo.Path)
		stats.DirDistribution[dir]++

		// Track file stats for hot spots
		if fs, exists := fileStatsMap[result.FileID]; exists {
			fs.matches++
			if result.Line < fs.firstLine {
				fs.firstLine = result.Line
			}
			if result.Line > fs.lastLine {
				fs.lastLine = result.Line
			}
		} else {
			fileStatsMap[result.FileID] = &fileStats{
				path:      fileInfo.Path,
				matches:   1,
				firstLine: result.Line,
				lastLine:  result.Line,
			}
		}

		// Count test file matches
		if strings.Contains(fileInfo.Path, "_test.go") || strings.Contains(fileInfo.Path, ".test.") ||
			strings.Contains(fileInfo.Path, "/test/") || strings.Contains(fileInfo.Path, "/tests/") {
			stats.TestFileMatches++
		}

		// Check if match is in a comment
		if e.isInComment(fileInfo, result.Line) {
			stats.CommentMatches++
		}

		// Find symbol context
		symbol := e.findContainingSymbol(fileInfo, result.Line)
		if symbol != nil {
			// Map symbol type to string
			symbolTypeStr := "unknown"
			switch symbol.Type {
			case types.SymbolTypeFunction:
				symbolTypeStr = "function"
			case types.SymbolTypeClass:
				symbolTypeStr = "class"
			case types.SymbolTypeVariable:
				symbolTypeStr = "variable"
			case types.SymbolTypeConstant:
				symbolTypeStr = "constant"
			case types.SymbolTypeType:
				symbolTypeStr = "type"
			case types.SymbolTypeInterface:
				symbolTypeStr = "interface"
			case types.SymbolTypeMethod:
				symbolTypeStr = "method"
			}
			stats.SymbolTypes[symbolTypeStr]++

			// Check if it's likely a definition based on heuristics
			// Simple heuristic: if the symbol name appears at the beginning of the line
			// after trimming whitespace, it's likely a definition
			if result.Line > 0 {
				lineText := strings.TrimSpace(e.GetLine(fileInfo, result.Line))
				if strings.HasPrefix(lineText, symbol.Name) {
					stats.DefinitionCount++
				} else {
					stats.UsageCount++
				}
			}

			// Check if symbol is exported (simple heuristic - starts with uppercase)
			if len(symbol.Name) > 0 && symbol.Name[0] >= 'A' && symbol.Name[0] <= 'Z' {
				stats.ExportedSymbols++
			}
		}
	}

	// Identify hot spots (top 5 files by match count)
	var hotFiles []*fileStats
	for _, fs := range fileStatsMap {
		hotFiles = append(hotFiles, fs)
	}
	sort.Slice(hotFiles, func(i, j int) bool {
		return hotFiles[i].matches > hotFiles[j].matches
	})

	maxHotSpots := 5
	if len(hotFiles) < maxHotSpots {
		maxHotSpots = len(hotFiles)
	}

	for i := 0; i < maxHotSpots; i++ {
		fs := hotFiles[i]
		stats.HotSpots = append(stats.HotSpots, types.HotSpot{
			File:       fs.path,
			MatchCount: fs.matches,
			FirstLine:  fs.firstLine,
			LastLine:   fs.lastLine,
		})
	}

	stats.SearchTimeMs = time.Since(startTime).Milliseconds()
	return stats, nil
}

// MultiSearchStats gathers statistics for multiple search patterns
func (e *Engine) MultiSearchStats(patterns []string, candidates []types.FileID, options types.SearchOptions) (*types.MultiSearchStats, error) {
	startTime := time.Now()

	multiStats := &types.MultiSearchStats{
		Patterns:     patterns,
		Results:      make(map[string]*types.SearchStats),
		CoOccurrence: make(map[string]map[string]int),
	}

	// Track which files match which patterns
	filePatternMap := make(map[string][]string) // file -> patterns that match

	// Gather stats for each pattern
	for _, pattern := range patterns {
		stats, err := e.SearchStats(pattern, candidates, options)
		if err != nil {
			return nil, fmt.Errorf("error searching for pattern %q: %w", pattern, err)
		}
		multiStats.Results[pattern] = stats

		// Track files for this pattern
		for file := range stats.FileDistribution {
			filePatternMap[file] = append(filePatternMap[file], pattern)
		}
	}

	// Find common files (matching all patterns)
	for file, matchedPatterns := range filePatternMap {
		if len(matchedPatterns) == len(patterns) {
			multiStats.CommonFiles = append(multiStats.CommonFiles, file)
		}

		// Track co-occurrence
		for i := 0; i < len(matchedPatterns); i++ {
			p1 := matchedPatterns[i]
			if multiStats.CoOccurrence[p1] == nil {
				multiStats.CoOccurrence[p1] = make(map[string]int)
			}
			for j := i + 1; j < len(matchedPatterns); j++ {
				p2 := matchedPatterns[j]
				multiStats.CoOccurrence[p1][p2]++
				if multiStats.CoOccurrence[p2] == nil {
					multiStats.CoOccurrence[p2] = make(map[string]int)
				}
				multiStats.CoOccurrence[p2][p1]++
			}
		}
	}

	sort.Strings(multiStats.CommonFiles)
	multiStats.TotalSearchTimeMs = time.Since(startTime).Milliseconds()

	return multiStats, nil
}

// Helper methods for SearchStats
func (e *Engine) findContainingSymbol(fileInfo *types.FileInfo, line int) *types.Symbol {
	// Get all symbols for this file
	symbols := e.indexer.GetFileEnhancedSymbols(fileInfo.ID)

	// Find the symbol that contains this line
	for _, enhancedSym := range symbols {
		sym := enhancedSym.Symbol
		// Check if line is within symbol's scope
		// This is a simplified check - ideally we'd use proper scope boundaries
		if sym.Line <= line && line <= sym.Line+10 { // Rough estimate
			return &sym
		}
	}

	return nil
}

// getFileIDFromPath converts a file path to FileID using the indexer
func (e *Engine) getFileIDFromPath(path string) types.FileID {
	if e.indexer == nil {
		return types.FileID(0)
	}

	// Try to find the file in all indexed files
	allFiles := e.indexer.GetAllFileIDs()
	for _, fileID := range allFiles {
		// PERFORMANCE: Use lightweight GetFilePath instead of GetFileInfo
		filePath := e.indexer.GetFilePath(fileID)
		if filePath == path {
			return fileID
		}
	}
	return types.FileID(0)
}

// isLiteralPattern checks if a pattern contains no regex metacharacters
// Returns true if the pattern can be safely searched as a literal string (8x faster than regex!)
func isLiteralPattern(pattern string) bool {
	// Check for common regex metacharacters
	// Note: We could use a regex here but that defeats the purpose :)
	for _, ch := range pattern {
		switch ch {
		case '.', '*', '+', '?', '[', ']', '(', ')', '|', '^', '$', '\\', '{', '}':
			return false // Found regex metacharacter
		}
	}
	return true // No metacharacters found - safe to use literal search
}
