package git

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/parser"
	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"
)

// Analyzer performs git change analysis comparing new code against existing index
type Analyzer struct {
	provider *Provider
	index    *indexing.MasterIndex
	parser   *parser.TreeSitterParser

	// Analysis components (reuse existing infrastructure)
	duplicateDetector *analysis.DuplicateDetector
	fuzzyMatcher      *semantic.FuzzyMatcher
	nameSplitter      *semantic.NameSplitter
}

// NewAnalyzer creates a new git change analyzer
func NewAnalyzer(provider *Provider, index *indexing.MasterIndex) *Analyzer {
	return &Analyzer{
		provider:          provider,
		index:             index,
		parser:            parser.NewTreeSitterParser(),
		duplicateDetector: analysis.NewDuplicateDetector(),
		fuzzyMatcher:      semantic.NewFuzzyMatcher(true, 0.8, "jaro-winkler"),
		nameSplitter:      semantic.NewNameSplitter(),
	}
}

// Analyze performs complete change analysis
func (a *Analyzer) Analyze(ctx context.Context, params AnalysisParams) (*AnalysisReport, error) {
	startTime := time.Now()

	// Get changed files
	changedFiles, err := a.provider.GetChangedFiles(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		return a.emptyReport(params, startTime), nil
	}

	// Parse changed files to extract new/modified symbols
	newSymbols, err := a.parseChangedFiles(ctx, changedFiles, params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse changed files: %w", err)
	}

	// Get existing symbols from index for comparison
	existingSymbols := a.getExistingSymbols()

	// Perform analyses based on focus
	var duplicates []DuplicateFinding
	var namingIssues []NamingFinding

	if params.HasFocus("duplicates") {
		duplicates = a.findDuplicates(ctx, newSymbols, existingSymbols, params)
	}

	if params.HasFocus("naming") {
		namingIssues = a.checkNamingConsistency(ctx, newSymbols, existingSymbols, params)
	}

	// Build report
	report := a.buildReport(changedFiles, newSymbols, duplicates, namingIssues, params, startTime)

	return report, nil
}

// parseChangedFiles extracts symbols from changed files
func (a *Analyzer) parseChangedFiles(ctx context.Context, files []ChangedFile, params AnalysisParams) ([]SymbolInfo, error) {
	var symbols []SymbolInfo

	targetRef := a.provider.GetTargetRef(params)

	for _, file := range files {
		// Skip deleted files
		if file.Status == FileStatusDeleted {
			continue
		}

		// Get file content at target ref
		content, err := a.provider.GetFileContent(ctx, targetRef, file.Path)
		if err != nil {
			// Skip files we can't read
			continue
		}

		// Check if file is a supported source file
		if !a.isSupportedFile(file.Path) {
			continue
		}

		// Parse the file using TreeSitterParser
		_, fileSymbols, _ := a.parser.ParseFile(file.Path, content)

		// Convert to SymbolInfo
		for _, sym := range fileSymbols {
			symbols = append(symbols, SymbolInfo{
				Name:     sym.Name,
				Type:     sym.Type.String(),
				FilePath: file.Path,
				Line:     sym.Line,
				EndLine:  sym.EndLine,
				// Complexity is not available from basic Symbol parsing
				Content: a.extractSymbolContent(content, sym),
			})
		}
	}

	return symbols, nil
}

// isSupportedFile checks if a file is supported for parsing
func (a *Analyzer) isSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	supportedExts := map[string]bool{
		".go":     true,
		".js":     true,
		".jsx":    true,
		".ts":     true,
		".tsx":    true,
		".py":     true,
		".rs":     true,
		".java":   true,
		".c":      true,
		".cpp":    true,
		".cc":     true,
		".h":      true,
		".hpp":    true,
		".cs":     true,
		".php":    true,
		".rb":     true,
		".swift":  true,
		".kt":     true,
		".scala":  true,
		".zig":    true,
		".vue":    true,
		".svelte": true,
	}

	return supportedExts[ext]
}

// extractSymbolContent extracts the content of a symbol from file content
// Uses zero-copy byte scanning instead of strings.Split
func (a *Analyzer) extractSymbolContent(content []byte, sym types.Symbol) string {
	if sym.Line <= 0 || len(content) == 0 {
		return ""
	}

	// Find line offsets using byte scanning
	startLine := sym.Line - 1 // Convert to 0-based
	endLine := sym.EndLine - 1
	if endLine < startLine {
		endLine = startLine
	}

	// Scan for line boundaries
	lineNum := 0
	lineStart := 0
	var startOffset, endOffset int
	foundStart := false

	for i := 0; i <= len(content); i++ {
		isEnd := i == len(content) || content[i] == '\n'
		if isEnd {
			if lineNum == startLine {
				startOffset = lineStart
				foundStart = true
			}
			if lineNum == endLine {
				endOffset = i
				break
			}
			lineNum++
			if i < len(content) {
				lineStart = i + 1
			}
		}
	}

	if !foundStart {
		return ""
	}
	if endOffset == 0 {
		endOffset = len(content)
	}

	return string(content[startOffset:endOffset])
}

// getExistingSymbols retrieves symbols from the existing index
func (a *Analyzer) getExistingSymbols() []SymbolInfo {
	var symbols []SymbolInfo

	// Get all files from index
	allFiles := a.index.GetAllFiles()

	for _, fileInfo := range allFiles {
		for _, sym := range fileInfo.EnhancedSymbols {
			symbols = append(symbols, SymbolInfo{
				Name:       sym.Name,
				Type:       sym.Type.String(),
				FilePath:   fileInfo.Path,
				Line:       sym.Line,
				EndLine:    sym.EndLine,
				Complexity: sym.Complexity,
			})
		}
	}

	return symbols
}

// findDuplicates detects duplicate code between new and existing symbols
func (a *Analyzer) findDuplicates(ctx context.Context, newSymbols, existingSymbols []SymbolInfo, params AnalysisParams) []DuplicateFinding {
	var findings []DuplicateFinding

	threshold := params.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.8
	}

	// Build a map of existing symbol content hashes for exact duplicate detection
	existingHashes := make(map[string][]SymbolInfo)
	for _, sym := range existingSymbols {
		if sym.Content != "" {
			hash := a.normalizeContent(sym.Content)
			existingHashes[hash] = append(existingHashes[hash], sym)
		}
	}

	// Check each new symbol for duplicates
	for _, newSym := range newSymbols {
		if newSym.Content == "" {
			continue
		}

		// Only check functions and methods for duplicates
		if newSym.Type != "function" && newSym.Type != "method" {
			continue
		}

		newHash := a.normalizeContent(newSym.Content)

		// Check for exact duplicates
		if existing, ok := existingHashes[newHash]; ok {
			for _, existSym := range existing {
				// Skip self-matches (same file, same line)
				if existSym.FilePath == newSym.FilePath && existSym.Line == newSym.Line {
					continue
				}

				finding := DuplicateFinding{
					Severity:    DetermineDuplicateSeverity(1.0, newSym.EndLine-newSym.Line),
					Description: fmt.Sprintf("Exact duplicate of %s in %s", existSym.Name, filepath.Base(existSym.FilePath)),
					NewCode: CodeLocation{
						FilePath:   newSym.FilePath,
						StartLine:  newSym.Line,
						EndLine:    newSym.EndLine,
						SymbolName: newSym.Name,
					},
					ExistingCode: CodeLocation{
						FilePath:   existSym.FilePath,
						StartLine:  existSym.Line,
						EndLine:    existSym.EndLine,
						SymbolName: existSym.Name,
					},
					Similarity: 1.0,
					Type:       "exact",
					Suggestion: fmt.Sprintf("Extract common code into a shared function, used by both %s and %s", newSym.Name, existSym.Name),
				}
				findings = append(findings, finding)
			}
		}

		// Check for structural duplicates using token similarity
		for _, existSym := range existingSymbols {
			if existSym.Content == "" {
				continue
			}

			// Skip self-matches
			if existSym.FilePath == newSym.FilePath && existSym.Line == newSym.Line {
				continue
			}

			// Skip if already found as exact duplicate
			if a.normalizeContent(existSym.Content) == newHash {
				continue
			}

			// Calculate structural similarity
			similarity := a.calculateStructuralSimilarity(newSym.Content, existSym.Content)
			if similarity >= threshold {
				finding := DuplicateFinding{
					Severity:    DetermineDuplicateSeverity(similarity, newSym.EndLine-newSym.Line),
					Description: fmt.Sprintf("Structurally similar to %s in %s (%.0f%% similar)", existSym.Name, filepath.Base(existSym.FilePath), similarity*100),
					NewCode: CodeLocation{
						FilePath:   newSym.FilePath,
						StartLine:  newSym.Line,
						EndLine:    newSym.EndLine,
						SymbolName: newSym.Name,
					},
					ExistingCode: CodeLocation{
						FilePath:   existSym.FilePath,
						StartLine:  existSym.Line,
						EndLine:    existSym.EndLine,
						SymbolName: existSym.Name,
					},
					Similarity: similarity,
					Type:       "structural",
					Suggestion: "Consider parameterizing the common structure to reduce duplication",
				}
				findings = append(findings, finding)
			}
		}
	}

	// Sort by similarity descending
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].Similarity > findings[j].Similarity
	})

	// Limit findings
	maxFindings := params.MaxFindings
	if maxFindings == 0 {
		maxFindings = 20
	}
	if len(findings) > maxFindings {
		findings = findings[:maxFindings]
	}

	return findings
}

// normalizeContent normalizes code content for comparison
// Uses zero-copy iteration instead of strings.Split
func (a *Analyzer) normalizeContent(content string) string {
	// Remove whitespace and normalize using zero-copy iteration
	var normalized []string
	remaining := content
	for len(remaining) > 0 {
		var line string
		if idx := strings.IndexByte(remaining, '\n'); idx >= 0 {
			line = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			line = remaining
			remaining = ""
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		normalized = append(normalized, line)
	}

	return strings.Join(normalized, "\n")
}

// calculateStructuralSimilarity calculates similarity between two code blocks
func (a *Analyzer) calculateStructuralSimilarity(content1, content2 string) float64 {
	// Tokenize both contents
	tokens1 := a.tokenize(content1)
	tokens2 := a.tokenize(content2)

	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0
	}

	// Calculate Jaccard similarity
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, t := range tokens1 {
		set1[t] = true
	}
	for _, t := range tokens2 {
		set2[t] = true
	}

	intersection := 0
	for t := range set1 {
		if set2[t] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// tokenize breaks code into tokens
func (a *Analyzer) tokenize(content string) []string {
	var tokens []string
	var current strings.Builder

	for _, ch := range content {
		if a.isDelimiter(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			if !a.isWhitespace(ch) {
				tokens = append(tokens, string(ch))
			}
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func (a *Analyzer) isDelimiter(ch rune) bool {
	return strings.ContainsRune("(){}[];,.<>+-*/=!&|^~?:", ch) || a.isWhitespace(ch)
}

func (a *Analyzer) isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// checkNamingConsistency detects naming issues between new and existing symbols
func (a *Analyzer) checkNamingConsistency(ctx context.Context, newSymbols, existingSymbols []SymbolInfo, params AnalysisParams) []NamingFinding {
	var findings []NamingFinding

	threshold := params.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.8
	}

	// Build existing name index by type
	existingByType := make(map[string][]SymbolInfo)
	for _, sym := range existingSymbols {
		existingByType[sym.Type] = append(existingByType[sym.Type], sym)
	}

	for _, newSym := range newSymbols {
		// 1. Check case style consistency using language-specific rules
		if caseStyleFinding := a.checkCaseStyleLanguageAware(newSym); caseStyleFinding != nil {
			findings = append(findings, *caseStyleFinding)
		}

		// 2. Check for similar existing names
		sameType := existingByType[newSym.Type]
		similarFinding := a.findSimilarNames(newSym, sameType, threshold)
		if similarFinding != nil {
			findings = append(findings, *similarFinding)
		}

		// 3. Check for abbreviation inconsistencies
		abbrevFinding := a.checkAbbreviations(newSym, existingSymbols)
		if abbrevFinding != nil {
			findings = append(findings, *abbrevFinding)
		}
	}

	// Sort by severity
	sort.Slice(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
	})

	// Limit findings
	maxFindings := params.MaxFindings
	if maxFindings == 0 {
		maxFindings = 20
	}
	if len(findings) > maxFindings {
		findings = findings[:maxFindings]
	}

	return findings
}

func severityRank(s FindingSeverity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// detectDominantCaseStyles determines the most common case style per symbol type
func (a *Analyzer) detectDominantCaseStyles(symbols []SymbolInfo) map[string]CaseStyle {
	styleCount := make(map[string]map[CaseStyle]int)

	for _, sym := range symbols {
		style := DetectCaseStyle(sym.Name)
		if style == CaseStyleUnknown {
			continue
		}

		if styleCount[sym.Type] == nil {
			styleCount[sym.Type] = make(map[CaseStyle]int)
		}
		styleCount[sym.Type][style]++
	}

	// Find dominant style for each type
	dominant := make(map[string]CaseStyle)
	for symType, counts := range styleCount {
		maxCount := 0
		var maxStyle CaseStyle
		for style, count := range counts {
			if count > maxCount {
				maxCount = count
				maxStyle = style
			}
		}
		if maxCount >= 3 { // Need at least 3 examples to establish a pattern
			dominant[symType] = maxStyle
		}
	}

	return dominant
}

// checkCaseStyleLanguageAware checks if symbol follows language-specific naming conventions
func (a *Analyzer) checkCaseStyleLanguageAware(sym SymbolInfo) *NamingFinding {
	// Get language from file path
	lang := GetLanguageFromPath(sym.FilePath)
	if lang == LangUnknown {
		return nil // Skip unknown languages
	}

	// Get symbol kind
	kind := SymbolTypeToKind(sym.Type)
	if kind == KindUnknownKind {
		return nil // Skip unknown symbol kinds
	}

	// Check actual case style
	actualStyle := DetectCaseStyle(sym.Name)
	if actualStyle == CaseStyleUnknown {
		return nil // Can't determine style (single word, etc.)
	}

	// Check if style is valid for this language/kind
	if IsValidCaseStyle(lang, kind, actualStyle) {
		return nil // Style is valid for this language
	}

	// Style violates language conventions
	expectedStyles := GetExpectedStyles(lang, kind)
	if len(expectedStyles) == 0 {
		return nil // No rules for this kind
	}

	// Format expected styles for the message
	expectedStr := formatExpectedStyles(expectedStyles)

	return &NamingFinding{
		Severity:   SeverityWarning,
		NewSymbol:  sym,
		IssueType:  NamingIssueCaseMismatch,
		Issue:      fmt.Sprintf("Uses %s but %s convention for %s is %s", actualStyle, lang, sym.Type, expectedStr),
		Suggestion: fmt.Sprintf("Consider renaming to use %s style to match %s conventions", expectedStr, lang),
	}
}

// formatExpectedStyles formats a slice of case styles for display
func formatExpectedStyles(styles []CaseStyle) string {
	if len(styles) == 0 {
		return ""
	}
	if len(styles) == 1 {
		return string(styles[0])
	}

	// Format as "style1 or style2" for two styles
	// or "style1, style2, or style3" for more
	strs := make([]string, len(styles))
	for i, s := range styles {
		strs[i] = string(s)
	}

	if len(strs) == 2 {
		return strs[0] + " or " + strs[1]
	}

	return strings.Join(strs[:len(strs)-1], ", ") + ", or " + strs[len(strs)-1]
}

// checkCaseStyle checks if symbol follows the dominant case style (legacy, unused)
func (a *Analyzer) checkCaseStyle(sym SymbolInfo, dominantStyles map[string]CaseStyle) *NamingFinding {
	dominantStyle, ok := dominantStyles[sym.Type]
	if !ok {
		return nil
	}

	actualStyle := DetectCaseStyle(sym.Name)
	if actualStyle == CaseStyleUnknown || actualStyle == dominantStyle {
		return nil
	}

	return &NamingFinding{
		Severity:   SeverityWarning,
		NewSymbol:  sym,
		IssueType:  NamingIssueCaseMismatch,
		Issue:      fmt.Sprintf("Uses %s but codebase uses %s for %s", actualStyle, dominantStyle, sym.Type),
		Suggestion: fmt.Sprintf("Consider renaming to use %s style to match codebase conventions", dominantStyle),
	}
}

// findSimilarNames finds existing symbols with similar names
func (a *Analyzer) findSimilarNames(newSym SymbolInfo, existing []SymbolInfo, threshold float64) *NamingFinding {
	var similar []SymbolInfo

	newLower := strings.ToLower(newSym.Name)

	for _, sym := range existing {
		// Skip exact matches
		if sym.Name == newSym.Name {
			continue
		}

		existLower := strings.ToLower(sym.Name)
		similarity := a.fuzzyMatcher.Similarity(newLower, existLower)

		if similarity >= threshold {
			similar = append(similar, sym)
		}
	}

	if len(similar) == 0 {
		return nil
	}

	// Sort by similarity (we'd need to recalculate, so just take first few)
	if len(similar) > 3 {
		similar = similar[:3]
	}

	return &NamingFinding{
		Severity:     DetermineNamingSeverity(NamingIssueSimilarExists, threshold),
		NewSymbol:    newSym,
		SimilarNames: similar,
		IssueType:    NamingIssueSimilarExists,
		Issue:        "Similar names already exist: " + formatSimilarNames(similar),
		Suggestion:   fmt.Sprintf("Consider using existing name '%s' or differentiate more clearly", similar[0].Name),
	}
}

// formatSimilarNames formats a list of similar names for display
func formatSimilarNames(symbols []SymbolInfo) string {
	names := make([]string, len(symbols))
	for i, s := range symbols {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

// checkAbbreviations checks for abbreviation inconsistencies
func (a *Analyzer) checkAbbreviations(newSym SymbolInfo, existing []SymbolInfo) *NamingFinding {
	// Split new symbol name into words
	newWords := a.nameSplitter.Split(newSym.Name)
	if len(newWords) == 0 {
		return nil
	}

	// Common abbreviation mappings to check
	abbrevMap := map[string][]string{
		"usr":   {"user"},
		"msg":   {"message"},
		"req":   {"request"},
		"res":   {"response", "result"},
		"resp":  {"response"},
		"btn":   {"button"},
		"img":   {"image"},
		"err":   {"error"},
		"ctx":   {"context"},
		"cfg":   {"config", "configuration"},
		"db":    {"database"},
		"str":   {"string"},
		"num":   {"number"},
		"idx":   {"index"},
		"len":   {"length"},
		"val":   {"value"},
		"ptr":   {"pointer"},
		"src":   {"source"},
		"dst":   {"destination", "dest"},
		"tmp":   {"temp", "temporary"},
		"auth":  {"authentication", "authorization"},
		"info":  {"information"},
		"init":  {"initialize", "initialization"},
		"param": {"parameter"},
		"args":  {"arguments"},
	}

	// Check if new symbol uses an abbreviation
	for _, word := range newWords {
		wordLower := strings.ToLower(word)

		// Check if this is a known abbreviation
		expansions, isAbbrev := abbrevMap[wordLower]
		if !isAbbrev {
			continue
		}

		// Check if existing code uses full form
		for _, existing := range existing {
			existingWords := a.nameSplitter.Split(existing.Name)
			for _, existWord := range existingWords {
				existWordLower := strings.ToLower(existWord)
				for _, expansion := range expansions {
					if existWordLower == expansion {
						return &NamingFinding{
							Severity:     SeverityInfo,
							NewSymbol:    newSym,
							SimilarNames: []SymbolInfo{existing},
							IssueType:    NamingIssueAbbreviation,
							Issue:        fmt.Sprintf("Uses abbreviation '%s' but codebase uses '%s'", word, existWord),
							Suggestion:   fmt.Sprintf("Consider using '%s' instead of '%s' for consistency", existWord, word),
						}
					}
				}
			}
		}

		// Also check reverse: if new uses full form but existing uses abbreviation
		for abbrev, exps := range abbrevMap {
			for _, exp := range exps {
				if wordLower == exp {
					// New symbol uses full form, check if existing uses abbreviation
					for _, existing := range existing {
						existingWords := a.nameSplitter.Split(existing.Name)
						for _, existWord := range existingWords {
							if strings.ToLower(existWord) == abbrev {
								return &NamingFinding{
									Severity:     SeverityInfo,
									NewSymbol:    newSym,
									SimilarNames: []SymbolInfo{existing},
									IssueType:    NamingIssueAbbreviation,
									Issue:        fmt.Sprintf("Uses full form '%s' but codebase uses abbreviation '%s'", word, existWord),
									Suggestion:   fmt.Sprintf("Consider using '%s' instead of '%s' for consistency", existWord, word),
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// buildReport constructs the final analysis report
func (a *Analyzer) buildReport(files []ChangedFile, symbols []SymbolInfo, duplicates []DuplicateFinding, namingIssues []NamingFinding, params AnalysisParams, startTime time.Time) *AnalysisReport {
	// Count symbols by change type
	symbolsAdded := 0
	for _, file := range files {
		if file.Status == FileStatusAdded {
			for _, sym := range symbols {
				if sym.FilePath == file.Path {
					symbolsAdded++
				}
			}
		}
	}
	symbolsModified := len(symbols) - symbolsAdded

	// Calculate risk score
	riskScore := CalculateRiskScore(duplicates, namingIssues)

	// Generate top recommendation
	topRec := GenerateTopRecommendation(duplicates, namingIssues)

	baseRef, _ := a.provider.GetBaseRef(context.Background(), params)
	targetRef := a.provider.GetTargetRef(params)

	return &AnalysisReport{
		Summary: ReportSummary{
			FilesChanged:      len(files),
			SymbolsAdded:      symbolsAdded,
			SymbolsModified:   symbolsModified,
			DuplicatesFound:   len(duplicates),
			NamingIssuesFound: len(namingIssues),
			RiskScore:         riskScore,
			TopRecommendation: topRec,
		},
		Duplicates:   duplicates,
		NamingIssues: namingIssues,
		Metadata: ReportMetadata{
			BaseRef:        baseRef,
			TargetRef:      targetRef,
			Scope:          params.Scope,
			AnalyzedAt:     time.Now(),
			AnalysisTimeMs: time.Since(startTime).Milliseconds(),
		},
	}
}

// emptyReport creates an empty report when there are no changes
func (a *Analyzer) emptyReport(params AnalysisParams, startTime time.Time) *AnalysisReport {
	baseRef, _ := a.provider.GetBaseRef(context.Background(), params)
	targetRef := a.provider.GetTargetRef(params)

	return &AnalysisReport{
		Summary: ReportSummary{
			TopRecommendation: "No changes to analyze",
		},
		Metadata: ReportMetadata{
			BaseRef:        baseRef,
			TargetRef:      targetRef,
			Scope:          params.Scope,
			AnalyzedAt:     time.Now(),
			AnalysisTimeMs: time.Since(startTime).Milliseconds(),
		},
	}
}
