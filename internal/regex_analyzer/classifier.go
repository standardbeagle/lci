package regex_analyzer

import (
	"regexp"
	"strings"
)

// RegexClassifier determines if a regex pattern is simple enough for trigram optimization
type RegexClassifier struct {
	// Pre-compiled patterns that indicate complexity
	complexPatterns []*regexp.Regexp
}

// NewRegexClassifier creates a new classifier with default complexity patterns
func NewRegexClassifier() *RegexClassifier {
	complexPatterns := []*regexp.Regexp{
		// Lookaheads and lookbehinds
		regexp.MustCompile(`\(\?[=!]`), // (?= or (?!
		regexp.MustCompile(`\(\?<`),    // (?< (lookbehind or named group)

		// Backreferences
		regexp.MustCompile(`\\\d+`), // \1, \2, etc.

		// Conditional groups
		regexp.MustCompile(`\(\?\(`), // (?( condition )

		// Atomic groups
		regexp.MustCompile(`\(\?>`), // (?>

		// Inline modifiers
		regexp.MustCompile(`\(\?[imsx-]+:`), // (?i: , (?m: , etc.

		// Possessive quantifiers
		regexp.MustCompile(`[*+?]\+`), // *+, ++, ?+

		// Recursive patterns
		regexp.MustCompile(`\(\?R\)|\(\?0\)`), // (?R) or (?0)

		// Subroutine calls
		regexp.MustCompile(`\(\?&\w+\)`), // (?&name)

		// Conditional groups with alternatives
		regexp.MustCompile(`\(\?\(\?[=!][^)]*\)`), // (?(!condition)

		// Mode modifiers
		regexp.MustCompile(`\(\?[^)]*\)`), // Any (?...) that's not captured by above
	}

	return &RegexClassifier{
		complexPatterns: complexPatterns,
	}
}

// IsSimple determines if a regex pattern is simple enough for trigram optimization
func (rc *RegexClassifier) IsSimple(pattern string) bool {
	if pattern == "" {
		return false
	}

	// Check for complex constructs
	for _, complexRegex := range rc.complexPatterns {
		if complexRegex.MatchString(pattern) {
			return false
		}
	}

	// Additional heuristic checks
	return rc.isPatternStructurallySimple(pattern)
}

// isPatternStructurallySimple performs structural analysis of the pattern
func (rc *RegexClassifier) isPatternStructurallySimple(pattern string) bool {
	// Check balanced parentheses and other structures
	if !rc.isBalanced(pattern) {
		return false
	}

	// Check for deeply nested structures (performance concern)
	nestingDepth := rc.calculateNestingDepth(pattern)
	if nestingDepth > 5 { // Arbitrary threshold
		return false
	}

	// Check for extremely long alternations (performance concern)
	if rc.hasLongAlternations(pattern) {
		return false
	}

	return true
}

// isBalanced checks if the pattern has balanced parentheses, brackets, and braces
func (rc *RegexClassifier) isBalanced(pattern string) bool {
	parens := 0
	brackets := 0
	braces := 0
	inCharClass := false
	escaped := false

	for i, r := range pattern {
		if escaped {
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '[':
			if !inCharClass {
				inCharClass = true
			}
		case ']':
			if inCharClass && i > 0 && pattern[i-1] != '\\' {
				inCharClass = false
			}
		case '(':
			if !inCharClass {
				parens++
			}
		case ')':
			if !inCharClass {
				parens--
				if parens < 0 {
					return false
				}
			}
		case '{':
			if !inCharClass {
				braces++
			}
		case '}':
			if !inCharClass {
				braces--
				if braces < 0 {
					return false
				}
			}
		}
	}

	return parens == 0 && brackets == 0 && braces == 0
}

// calculateNestingDepth calculates the maximum nesting depth of groups
func (rc *RegexClassifier) calculateNestingDepth(pattern string) int {
	maxDepth := 0
	currentDepth := 0
	inCharClass := false
	escaped := false

	for i, r := range pattern {
		if escaped {
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '[':
			if !inCharClass {
				inCharClass = true
			}
		case ']':
			if inCharClass && i > 0 && pattern[i-1] != '\\' {
				inCharClass = false
			}
		case '(':
			if !inCharClass {
				currentDepth++
				if currentDepth > maxDepth {
					maxDepth = currentDepth
				}
			}
		case ')':
			if !inCharClass && currentDepth > 0 {
				currentDepth--
			}
		}
	}

	return maxDepth
}

// hasLongAlternations checks if the pattern has very long alternation lists
// Uses zero-copy iteration instead of strings.Split
func (rc *RegexClassifier) hasLongAlternations(pattern string) bool {
	// Simple heuristic: count alternations in top-level groups
	alternationCount := strings.Count(pattern, "|")
	if alternationCount > 20 { // Arbitrary threshold
		return true
	}

	// Check for very long alternation sequences using zero-copy iteration
	remaining := pattern
	for len(remaining) > 0 {
		var part string
		if idx := strings.IndexByte(remaining, '|'); idx >= 0 {
			part = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			part = remaining
			remaining = ""
		}
		// If any alternation part is extremely long, consider it complex
		if len(part) > 1000 { // Arbitrary threshold
			return true
		}
	}

	return false
}

// LiteralExtractor extracts literal strings from regex patterns for trigram filtering
type LiteralExtractor struct {
	// Patterns for extracting literals
	literalPattern     *regexp.Regexp
	alternationPattern *regexp.Regexp
}

// NewLiteralExtractor creates a new literal extractor
func NewLiteralExtractor() *LiteralExtractor {
	// Pattern to find literal sequences (must contain at least one alphanumeric)
	// This extracts clean word-like sequences suitable for trigrams
	literalPattern := regexp.MustCompile(`[a-zA-Z0-9_]{3,}`)

	// Pattern to find alternations - extract word-like alternatives
	alternationPattern := regexp.MustCompile(`\(([a-zA-Z0-9_]+(?:\|[a-zA-Z0-9_]+)*)\)`)

	return &LiteralExtractor{
		literalPattern:     literalPattern,
		alternationPattern: alternationPattern,
	}
}

// ExtractLiterals extracts literal strings suitable for trigram filtering
func (le *LiteralExtractor) ExtractLiterals(pattern string) []string {
	var literals []string
	seen := make(map[string]bool)

	// Extract from alternations first (highest priority)
	alternationLiterals := le.extractFromAlternations(pattern)
	for _, lit := range alternationLiterals {
		if len(lit) >= 3 && le.hasAlphanumeric(lit) && !seen[lit] {
			literals = append(literals, lit)
			seen[lit] = true
		}
	}

	// Extract other literals
	generalLiterals := le.extractGeneralLiterals(pattern)
	for _, lit := range generalLiterals {
		if len(lit) >= 3 && le.hasAlphanumeric(lit) && !seen[lit] {
			literals = append(literals, lit)
			seen[lit] = true
		}
	}

	return literals
}

// hasAlphanumeric checks if a string contains at least one alphanumeric character
func (le *LiteralExtractor) hasAlphanumeric(literal string) bool {
	for _, r := range literal {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return true
		}
	}
	return false
}

// extractFromAlternations extracts literals from alternation groups
// Uses zero-copy iteration instead of strings.Split
func (le *LiteralExtractor) extractFromAlternations(pattern string) []string {
	var literals []string

	// Find alternation groups
	matches := le.alternationPattern.FindAllStringSubmatch(pattern, -1)
	for _, match := range matches {
		if len(match) > 1 {
			alternationContent := match[1]
			// Zero-copy iteration over alternations
			remaining := alternationContent
			for len(remaining) > 0 {
				var alt string
				if idx := strings.IndexByte(remaining, '|'); idx >= 0 {
					alt = remaining[:idx]
					remaining = remaining[idx+1:]
				} else {
					alt = remaining
					remaining = ""
				}
				// Our alternation pattern already extracts clean sequences
				if len(alt) >= 3 {
					literals = append(literals, alt)
				}
			}
		}
	}

	return literals
}

// extractGeneralLiterals extracts literal sequences from the pattern
func (le *LiteralExtractor) extractGeneralLiterals(pattern string) []string {
	var literals []string

	// Find literal sequences using the clean pattern
	matches := le.literalPattern.FindAllString(pattern, -1)
	for _, match := range matches {
		// Our pattern already extracts clean sequences
		if len(match) >= 3 {
			literals = append(literals, match)
		}
	}

	return literals
}
