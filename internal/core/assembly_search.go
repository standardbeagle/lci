package core

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

// AssemblySearchEngine performs reverse string assembly search
type AssemblySearchEngine struct {
	trigramIndex      *TrigramIndex
	symbolIndex       *SymbolIndex
	fileProvider      FileProvider // Interface to avoid import cycle
	minFragmentLength int          // Default: 4 chars
	minCoverage       float64      // Default: 0.7 (70%)
	maxResults        int          // Default: 20
}

// FileProvider interface for accessing file information
type FileProvider interface {
	GetFileInfo(fileID types.FileID) *types.FileInfo
}

// NewAssemblySearchEngine creates a new assembly search engine
func NewAssemblySearchEngine(
	trigramIndex *TrigramIndex,
	symbolIndex *SymbolIndex,
	fileProvider FileProvider,
) *AssemblySearchEngine {
	return &AssemblySearchEngine{
		trigramIndex:      trigramIndex,
		symbolIndex:       symbolIndex,
		fileProvider:      fileProvider,
		minFragmentLength: 4,
		minCoverage:       0.7,
		maxResults:        20,
	}
}

// AssemblyResult represents a potential string assembly match
type AssemblyResult struct {
	Fragments []Fragment           // The fragments that could assemble the string
	Coverage  float64              // Percentage of target string covered
	Pattern   string               // Assembly pattern detected (concat, format, template)
	Score     float64              // Overall confidence score
	FileID    uint32               // File containing the assembly
	Location  types.SymbolLocation // Primary location of assembly
	Context   string               // Code snippet showing assembly
	GroupID   int                  // For grouping related results
}

// Fragment represents a piece of the target string found in code
type Fragment struct {
	Text          string               // The fragment text
	FileID        uint32               // File containing fragment
	Location      types.SymbolLocation // Location in file
	InStringLit   bool                 // Is this found in a string literal?
	SymbolContext string               // Function/class containing fragment
	Confidence    float64              // Confidence this is part of assembly
}

// ProximityScore represents how close fragments are in code
type ProximityScore struct {
	SameLine     bool
	SameFunction bool
	SameFile     bool
	CallDistance int // Number of function calls between fragments
	Score        float64
}

// SearchParams configures assembly search behavior
type AssemblySearchParams struct {
	Pattern           string   // The target string to search for
	MinCoverage       float64  // Minimum coverage threshold (0.0-1.0)
	MinFragmentLength int      // Minimum fragment size in characters
	MaxResults        int      // Maximum number of results to return
	Languages         []string // Language filters (empty = all)
}

// Search performs assembly search for dynamically built strings
func (ase *AssemblySearchEngine) Search(params AssemblySearchParams) ([]AssemblyResult, error) {
	startTime := time.Now()

	// Apply defaults
	if params.MinCoverage == 0 {
		params.MinCoverage = ase.minCoverage
	}
	if params.MinFragmentLength == 0 {
		params.MinFragmentLength = ase.minFragmentLength
	}
	if params.MaxResults == 0 {
		params.MaxResults = ase.maxResults
	}

	// Step 1: Fragment the search string
	fragments := ase.fragmentString(params.Pattern, params.MinFragmentLength)
	if len(fragments) == 0 {
		return nil, errors.New("no viable fragments found in pattern")
	}

	// Step 2: Find each fragment in the codebase
	fragmentMatches := make(map[string][]Fragment)
	for _, frag := range fragments {
		matches := ase.findFragmentInCode(frag)
		if len(matches) > 0 {
			fragmentMatches[frag] = matches
		}
	}

	// Step 3: Analyze fragment combinations for assembly
	assemblies := ase.analyzeAssemblies(fragmentMatches, params.Pattern, params.MinCoverage)

	// Step 4: Score and rank results
	ase.scoreAssemblies(assemblies)
	sort.Slice(assemblies, func(i, j int) bool {
		return assemblies[i].Score > assemblies[j].Score
	})

	// Step 5: Limit results
	if len(assemblies) > params.MaxResults {
		assemblies = assemblies[:params.MaxResults]
	}

	// Log performance
	elapsed := time.Since(startTime)
	if elapsed > 500*time.Millisecond {
		debug.Printf("Assembly search took %v (target: <500ms)\n", elapsed)
	}

	return assemblies, nil
}

// fragmentString splits the target string into searchable fragments
// It automatically detects HTML/JSX content and uses appropriate extraction
func (ase *AssemblySearchEngine) fragmentString(pattern string, minLength int) []string {
	// Check if pattern looks like HTML/JSX
	if strings.Contains(pattern, "<") && strings.Contains(pattern, ">") {
		// Use HTML-aware extraction
		return ase.extractHTMLFragments(pattern, minLength)
	}

	// Regular text extraction
	var fragments []string

	// Strategy 1: Split by common separators (including slashes for paths)
	separators := []string{": ", " - ", ", ", " | ", "/", " ", ".", "!", "?"}

	remaining := pattern
	for _, sep := range separators {
		parts := strings.Split(remaining, sep)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if len(part) >= minLength {
				fragments = append(fragments, part)
			}
		}
	}

	// Strategy 2: Look for likely static string boundaries
	// Common patterns: "Error", "Warning", "Failed", "Success", etc.
	staticPrefixes := []string{"Error", "Warning", "Failed", "Success", "Invalid", "Missing"}
	for _, prefix := range staticPrefixes {
		if strings.Contains(pattern, prefix) {
			idx := strings.Index(pattern, prefix)
			if idx >= 0 {
				// Extract prefix and some context
				endIdx := idx + len(prefix)
				for endIdx < len(pattern) && pattern[endIdx] != ' ' && pattern[endIdx] != ':' {
					endIdx++
				}
				if endIdx-idx >= minLength {
					fragments = append(fragments, pattern[idx:endIdx])
				}
			}
		}
	}

	// Deduplicate fragments
	seen := make(map[string]bool)
	unique := []string{}
	for _, frag := range fragments {
		if !seen[frag] {
			seen[frag] = true
			unique = append(unique, frag)
		}
	}

	return unique
}

// extractHTMLFragments extracts fragments from HTML/JSX content
func (ase *AssemblySearchEngine) extractHTMLFragments(pattern string, minLength int) []string {
	fragments := make(map[string]bool)

	// Regular expressions for HTML/JSX patterns
	tagRegex := regexp.MustCompile(`</?(\w+)[^>]*>`)
	attrRegex := regexp.MustCompile(`(\w+(?:-\w+)*)=["'{]([^"'}]*)["'}]`)
	jsxExprRegex := regexp.MustCompile(`\{([^}]+)\}`)
	classNameRegex := regexp.MustCompile(`className=["']([^"']+)["']`)
	dataAttrRegex := regexp.MustCompile(`(data-\w+|aria-\w+)=["']([^"']+)["']`)

	// Extract tag names (even single-letter tags are important in HTML)
	for _, match := range tagRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 && match[1] != "" {
			fragments[match[1]] = true
		}
	}

	// Extract attributes and their values
	for _, match := range attrRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 && len(match[1]) >= minLength {
			fragments[match[1]] = true
		}
		if len(match) > 2 && len(match[2]) >= minLength {
			// Split attribute values by common separators
			parts := strings.FieldsFunc(match[2], func(r rune) bool {
				return r == '-' || r == '_' || r == ' ' || r == '/'
			})
			for _, part := range parts {
				if len(part) >= minLength {
					fragments[part] = true
				}
			}
		}
	}

	// Extract JSX expressions
	for _, match := range jsxExprRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 {
			expr := match[1]
			// Handle object property access like user.name
			if strings.Contains(expr, ".") {
				parts := strings.Split(expr, ".")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if len(part) >= minLength {
						fragments[part] = true
					}
				}
			}
			// Also include the full expression if it's not too long
			if len(expr) >= minLength && len(expr) <= 30 {
				fragments[expr] = true
			}
		}
	}

	// Extract className values specifically
	for _, match := range classNameRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 {
			// Split by spaces for multiple classes
			classes := strings.Fields(match[1])
			for _, class := range classes {
				if len(class) >= minLength {
					fragments[class] = true
				}
			}
		}
	}

	// Extract data and aria attributes
	for _, match := range dataAttrRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 && len(match[1]) >= minLength {
			fragments[match[1]] = true
		}
		if len(match) > 2 && len(match[2]) >= minLength {
			// Split compound values
			parts := strings.FieldsFunc(match[2], func(r rune) bool {
				return r == '-' || r == ' '
			})
			for _, part := range parts {
				if len(part) >= minLength {
					fragments[part] = true
				}
			}
		}
	}

	// Extract text content between tags
	textRegex := regexp.MustCompile(`>([^<>{]+)<`)
	for _, match := range textRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			// Split text content by spaces and punctuation
			words := strings.FieldsFunc(text, func(r rune) bool {
				return r == ' ' || r == ':' || r == ',' || r == '.' || r == '(' || r == ')' || r == '@'
			})
			for _, word := range words {
				word = strings.TrimSpace(word)
				if len(word) >= minLength {
					fragments[word] = true
				}
			}
		}
	}

	// Also extract text that appears before the first tag or after the last tag
	leadingTextRegex := regexp.MustCompile(`^([^<]+)<`)
	if match := leadingTextRegex.FindStringSubmatch(pattern); len(match) > 1 {
		words := strings.Fields(match[1])
		for _, word := range words {
			if len(word) >= minLength {
				fragments[word] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(fragments))
	for frag := range fragments {
		result = append(result, frag)
	}

	return result
}

// findFragmentInCode searches for a fragment using the trigram index
func (ase *AssemblySearchEngine) findFragmentInCode(fragment string) []Fragment {
	var results []Fragment

	// Use trigram index to find match locations
	matches := ase.trigramIndex.FindMatchLocations(fragment, false, ase.fileProvider.GetFileInfo)

	for _, match := range matches {
		// Create fragment result
		frag := Fragment{
			Text:   fragment,
			FileID: uint32(match.FileID),
			Location: types.SymbolLocation{
				FileID: match.FileID,
				Line:   match.Line,
				Column: match.Column,
			},
			InStringLit: true, // Assume in string literal for now
			Confidence:  ase.calculateFragmentConfidence(fragment, match),
		}

		// Get symbol context if available
		if ase.symbolIndex != nil {
			// For now, we'll leave symbol context empty as the method doesn't exist
			// This can be enhanced later
			frag.SymbolContext = ""
		}

		results = append(results, frag)
	}

	return results
}

// calculateFragmentConfidence scores how likely a fragment is part of string assembly
func (ase *AssemblySearchEngine) calculateFragmentConfidence(fragment string, match SearchLocation) float64 {
	confidence := 1.0

	// Longer fragments are more confident matches
	lengthScore := math.Min(float64(len(fragment))/20.0, 1.0)
	confidence *= (0.5 + 0.5*lengthScore)

	// Fragments with meaningful words score higher
	if hasSignificantWords(fragment) {
		confidence *= 1.2
	}

	// Very short fragments score lower
	if len(fragment) < 6 {
		confidence *= 0.7
	}

	return math.Min(confidence, 1.0)
}

// hasSignificantWords checks if fragment contains meaningful content
func hasSignificantWords(text string) bool {
	significantWords := []string{
		"error", "warning", "success", "failed", "invalid",
		"missing", "required", "user", "system", "database",
		"file", "connection", "timeout", "permission", "access",
	}

	lower := strings.ToLower(text)
	for _, word := range significantWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// analyzeAssemblies finds fragment combinations that could build the target string
func (ase *AssemblySearchEngine) analyzeAssemblies(
	fragmentMatches map[string][]Fragment,
	targetString string,
	minCoverage float64,
) []AssemblyResult {
	var results []AssemblyResult

	// For MVP, use simple proximity-based grouping
	// Group fragments that appear in the same file/function
	fileGroups := make(map[uint32][]Fragment)

	for _, fragments := range fragmentMatches {
		for _, frag := range fragments {
			fileGroups[frag.FileID] = append(fileGroups[frag.FileID], frag)
		}
	}

	// Create assembly results for each file group
	groupID := 0
	for fileID, fragments := range fileGroups {
		coverage := ase.calculateCoverage(fragments, targetString)
		if coverage >= minCoverage {
			result := AssemblyResult{
				Fragments: fragments,
				Coverage:  coverage,
				Pattern:   ase.detectPattern(fragments),
				FileID:    fileID,
				GroupID:   groupID,
			}

			// Set primary location as the first fragment
			if len(fragments) > 0 {
				result.Location = fragments[0].Location
			}

			results = append(results, result)
			groupID++
		}
	}

	return results
}

// calculateCoverage determines what percentage of target string is covered
func (ase *AssemblySearchEngine) calculateCoverage(fragments []Fragment, target string) float64 {
	covered := 0
	for _, frag := range fragments {
		if strings.Contains(target, frag.Text) {
			covered += len(frag.Text)
		}
	}

	// Avoid over-counting overlapping fragments
	if covered > len(target) {
		covered = len(target)
	}

	return float64(covered) / float64(len(target))
}

// detectPattern identifies the string assembly pattern
func (ase *AssemblySearchEngine) detectPattern(fragments []Fragment) string {
	// For MVP, return basic pattern detection
	// Future feature: Implement AST-based pattern detection for more accurate analysis

	if len(fragments) == 1 {
		return "literal"
	}

	// Check if fragments are on consecutive lines (likely concatenation)
	consecutiveLines := true
	for i := 1; i < len(fragments); i++ {
		if fragments[i].Location.Line-fragments[i-1].Location.Line > 2 {
			consecutiveLines = false
			break
		}
	}

	if consecutiveLines {
		return "concat"
	}

	return "unknown"
}

// scoreAssemblies calculates final scores for assembly results
func (ase *AssemblySearchEngine) scoreAssemblies(results []AssemblyResult) {
	for i := range results {
		result := &results[i]

		// Base score from coverage
		score := result.Coverage * 100

		// Boost for proximity
		proximityBoost := ase.calculateProximityBoost(result.Fragments)
		score *= proximityBoost

		// Boost for known patterns
		if result.Pattern == "concat" {
			score *= 1.2
		} else if result.Pattern == "format" {
			score *= 1.3
		}

		// Penalty for too many fragments (likely noise)
		if len(result.Fragments) > 5 {
			score *= 0.8
		}

		result.Score = score
	}
}

// calculateProximityBoost scores based on how close fragments are
func (ase *AssemblySearchEngine) calculateProximityBoost(fragments []Fragment) float64 {
	if len(fragments) <= 1 {
		return 1.0
	}

	// Check if all fragments are in same function
	sameFunction := true
	firstContext := fragments[0].SymbolContext
	for _, frag := range fragments[1:] {
		if frag.SymbolContext != firstContext || firstContext == "" {
			sameFunction = false
			break
		}
	}

	if sameFunction {
		return 2.0 // High boost for same function
	}

	// Check if fragments are close in line numbers
	maxLineDiff := 0
	for i := 1; i < len(fragments); i++ {
		diff := int(math.Abs(float64(fragments[i].Location.Line - fragments[i-1].Location.Line)))
		if diff > maxLineDiff {
			maxLineDiff = diff
		}
	}

	if maxLineDiff <= 5 {
		return 1.5 // Medium boost for nearby lines
	} else if maxLineDiff <= 20 {
		return 1.2 // Small boost for same region
	}

	return 1.0 // No boost for distant fragments
}
