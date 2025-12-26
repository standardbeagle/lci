package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// GitignoreParser handles parsing and matching .gitignore files
type GitignoreParser struct {
	patterns []GitignorePattern

	// Performance optimization: regex compilation cache
	regexCache sync.Map
}

type GitignorePattern struct {
	Pattern   string
	Negate    bool
	Directory bool
	Absolute  bool

	// Performance optimization fields
	patternType PatternType
	compiled    *regexp.Regexp
	prefix      string // Fast prefix matching for simple patterns
	suffix      string // Fast suffix matching for simple patterns
}

// PatternType represents the type of pattern for optimization
type PatternType int

const (
	PatternExact PatternType = iota
	PatternPrefix
	PatternSuffix
	PatternContains
	PatternWildcard
	PatternComplex
)

// NewGitignoreParser creates a new gitignore parser
func NewGitignoreParser() *GitignoreParser {
	return &GitignoreParser{
		patterns: make([]GitignorePattern, 0),
	}
}

// LoadGitignore loads patterns from a .gitignore file
func (gp *GitignoreParser) LoadGitignore(rootPath string) error {
	gitignorePath := filepath.Join(rootPath, ".gitignore")

	file, err := os.Open(gitignorePath)
	if err != nil {
		// .gitignore file doesn't exist, which is fine
		return nil
	}
	defer file.Close()

	return gp.scanAndParsePatterns(file)
}

// scanAndParsePatterns scans a file and parses each line as a pattern
func (gp *GitignoreParser) scanAndParsePatterns(file *os.File) error {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if gp.shouldSkipLine(line) {
			continue
		}

		pattern := gp.parsePattern(line)
		gp.patterns = append(gp.patterns, pattern)
	}

	return scanner.Err()
}

// shouldSkipLine checks if a line should be skipped (empty or comment)
func (gp *GitignoreParser) shouldSkipLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "#")
}

// AddPattern adds a single pattern to the parser (for testing)
func (gp *GitignoreParser) AddPattern(line string) {
	pattern := gp.parsePattern(line)
	gp.patterns = append(gp.patterns, pattern)
}

// parsePattern parses a single gitignore pattern line with performance optimization
func (gp *GitignoreParser) parsePattern(line string) GitignorePattern {
	pattern := GitignorePattern{}

	// Extract pattern modifiers (negation, directory, absolute)
	line = gp.extractPatternModifiers(&pattern, line)

	// Store the cleaned pattern
	pattern.Pattern = line

	// Analyze and optimize pattern for fast matching
	pattern.patternType, pattern.prefix, pattern.suffix, pattern.compiled = gp.analyzePattern(line)

	return pattern
}

// extractPatternModifiers extracts and processes pattern modifiers (!, /, leading /)
// Returns the cleaned pattern string
func (gp *GitignoreParser) extractPatternModifiers(pattern *GitignorePattern, line string) string {
	// Handle negation (!)
	if strings.HasPrefix(line, "!") {
		pattern.Negate = true
		line = line[1:]
	}

	// Handle directory-only patterns (ending with /)
	if strings.HasSuffix(line, "/") {
		pattern.Directory = true
		line = strings.TrimSuffix(line, "/")
	}

	// Handle absolute patterns (starting with /)
	if strings.HasPrefix(line, "/") {
		pattern.Absolute = true
		line = line[1:]
	}

	return line
}

// analyzePattern determines pattern type and pre-compiles for performance
// Refactored to reduce cyclomatic complexity from 38 to <10
func (gp *GitignoreParser) analyzePattern(pattern string) (PatternType, string, string, *regexp.Regexp) {
	// Fast path: exact match (no wildcards)
	if !strings.ContainsAny(pattern, "*?[") {
		return PatternExact, pattern, pattern, nil
	}

	// Try to optimize simple wildcard patterns
	if patternType, prefix, suffix := gp.trySimplePatternOptimization(pattern); patternType != PatternWildcard {
		return patternType, prefix, suffix, nil
	}

	// Complex pattern - compile and cache regex
	return gp.compileAndCachePattern(pattern)
}

// trySimplePatternOptimization attempts to optimize simple wildcard patterns
// Returns PatternWildcard if optimization is not possible
func (gp *GitignoreParser) trySimplePatternOptimization(pattern string) (PatternType, string, string) {
	// Only optimize simple asterisk patterns (no ? or [])
	if !gp.isSimpleAsteriskPattern(pattern) {
		return PatternWildcard, "", ""
	}

	// Try suffix optimization (e.g., "*.log" -> ".log" suffix)
	if suffix, ok := gp.extractSuffixPattern(pattern); ok {
		return PatternSuffix, "", suffix
	}

	// Try prefix optimization (e.g., "test*" -> "test" prefix)
	if prefix, ok := gp.extractPrefixPattern(pattern); ok {
		return PatternPrefix, prefix, ""
	}

	return PatternWildcard, "", ""
}

// isSimpleAsteriskPattern checks if pattern only contains asterisks (no ? or [])
func (gp *GitignoreParser) isSimpleAsteriskPattern(pattern string) bool {
	return strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") && !strings.Contains(pattern, "[")
}

// extractSuffixPattern extracts suffix from patterns like "*.log"
// Returns the suffix and true if extraction succeeded
func (gp *GitignoreParser) extractSuffixPattern(pattern string) (string, bool) {
	if strings.HasPrefix(pattern, "*") && !strings.Contains(pattern[1:], "*") {
		return pattern[1:], true
	}
	return "", false
}

// extractPrefixPattern extracts prefix from patterns like "test*"
// Returns the prefix and true if extraction succeeded
func (gp *GitignoreParser) extractPrefixPattern(pattern string) (string, bool) {
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		return pattern[:len(pattern)-1], true
	}
	return "", false
}

// compileAndCachePattern compiles complex patterns to regex and caches them
func (gp *GitignoreParser) compileAndCachePattern(pattern string) (PatternType, string, string, *regexp.Regexp) {
	regexPattern := gp.globToRegex(pattern)

	// Check cache first
	if cached, ok := gp.regexCache.Load(regexPattern); ok {
		return PatternComplex, "", "", cached.(*regexp.Regexp)
	}

	// Compile and cache
	compiled, err := regexp.Compile(regexPattern)
	if err != nil {
		// Fallback to filepath.Match for invalid patterns
		return PatternWildcard, "", "", nil
	}

	gp.regexCache.Store(regexPattern, compiled)
	return PatternComplex, "", "", compiled
}

// globToRegex converts a glob pattern to a regex pattern
func (gp *GitignoreParser) globToRegex(pattern string) string {
	regex := regexp.QuoteMeta(pattern)

	// Replace escaped wildcards with regex equivalents
	regex = strings.ReplaceAll(regex, `\*`, `.*`)
	regex = strings.ReplaceAll(regex, `\?`, `.`)

	// Handle character classes
	regex = strings.ReplaceAll(regex, `\[`, `[`)
	regex = strings.ReplaceAll(regex, `\]`, `]`)

	// Anchors for full match
	return "^" + regex + "$"
}

// ShouldIgnore checks if a path should be ignored based on gitignore patterns
func (gp *GitignoreParser) ShouldIgnore(path string, isDir bool) bool {
	// Convert to forward slashes for consistent matching
	path = filepath.ToSlash(path)

	ignored := false

	for _, pattern := range gp.patterns {
		if gp.matchesPattern(pattern, path, isDir) {
			if pattern.Negate {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}

// matchesPattern checks if a pattern matches a given path with optimized matching
func (gp *GitignoreParser) matchesPattern(pattern GitignorePattern, path string, isDir bool) bool {
	// Directory-only patterns should match directories AND files/subdirectories within them
	if pattern.Directory {
		// If the path itself is a directory and matches the directory pattern, return true
		if isDir {
			return gp.matchDirectoryPatternOptimized(pattern, path)
		}
		// If the path is a file, check if it's inside a directory that matches the pattern
		return gp.matchInsideDirectoryPatternOptimized(pattern, path)
	}

	// Handle absolute vs relative patterns
	if pattern.Absolute {
		// Absolute pattern - match from root (exact match only for gitignore)
		return gp.fastMatchPattern(pattern, path)
	} else {
		// Relative pattern - match any component or full path
		pathParts := strings.Split(path, "/")

		// Try matching the full path
		if gp.fastMatchPattern(pattern, path) {
			return true
		}

		// Try matching against any suffix of the path
		for i := 0; i < len(pathParts); i++ {
			suffix := strings.Join(pathParts[i:], "/")
			if gp.fastMatchPattern(pattern, suffix) {
				return true
			}
		}
	}

	return false
}

// fastMatchPattern performs optimized pattern matching based on pattern type
func (gp *GitignoreParser) fastMatchPattern(pattern GitignorePattern, path string) bool {
	switch pattern.patternType {
	case PatternExact:
		return pattern.Pattern == path

	case PatternPrefix:
		return strings.HasPrefix(path, pattern.prefix)

	case PatternSuffix:
		return strings.HasSuffix(path, pattern.suffix)

	case PatternComplex:
		return pattern.compiled.MatchString(path)

	case PatternWildcard:
		// Fallback to filepath.Match for complex wildcard patterns
		if matched, _ := filepath.Match(pattern.Pattern, path); matched {
			return true
		}

	default:
		// Try exact match as fallback
		return pattern.Pattern == path
	}

	return false
}

// matchDirectoryPatternOptimized checks if a directory path matches a gitignore directory pattern
func (gp *GitignoreParser) matchDirectoryPatternOptimized(pattern GitignorePattern, path string) bool {
	// Use optimized pattern matching
	if gp.fastMatchPattern(pattern, path) {
		return true
	}

	// Pattern with /** should match subdirectories too
	if strings.HasSuffix(pattern.Pattern, "/**") {
		basePattern := strings.TrimSuffix(pattern.Pattern, "/**")
		if path == basePattern || strings.HasPrefix(path, basePattern+"/") {
			return true
		}
	}

	return false
}

// matchInsideDirectoryPatternOptimized checks if a file path is inside a directory that matches a gitignore directory pattern
func (gp *GitignoreParser) matchInsideDirectoryPatternOptimized(pattern GitignorePattern, path string) bool {
	// Fast path: direct prefix match
	if strings.HasPrefix(path, pattern.Pattern+"/") {
		return true
	}

	// Use optimized pattern matching for more complex cases
	return gp.fastMatchPattern(pattern, path)
}

// GetExclusionPatterns returns gitignore patterns as exclusion patterns for LCI
func (gp *GitignoreParser) GetExclusionPatterns() []string {
	var exclusions []string

	for _, pattern := range gp.patterns {
		if pattern.Negate {
			// Skip negation patterns for now (complex to implement)
			continue
		}

		// Convert gitignore pattern to LCI exclusion pattern
		lciPattern := gp.convertToLCIPattern(pattern)
		if lciPattern != "" {
			exclusions = append(exclusions, lciPattern)
		}
	}

	return exclusions
}

// convertToLCIPattern converts a gitignore pattern to an LCI exclusion pattern
func (gp *GitignoreParser) convertToLCIPattern(pattern GitignorePattern) string {
	p := pattern.Pattern

	// Handle directory patterns
	if pattern.Directory {
		if pattern.Absolute {
			return p + "/**"
		} else {
			return "**/" + p + "/**"
		}
	}

	// Handle file patterns
	if pattern.Absolute {
		return p
	} else {
		// Make it match anywhere in the tree
		if strings.Contains(p, "/") {
			return "**/" + p
		} else {
			return "**/" + p
		}
	}
}
