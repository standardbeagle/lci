package core

import (
	"fmt"
	"sort"
	"strings"
)

// SearchResult represents a search result for LLM optimization
type SearchResult struct {
	File          string  `json:"file"`
	Line          int     `json:"line"`
	Column        int     `json:"column"`
	Score         float64 `json:"score"`
	Match         string  `json:"match"`
	Context       string  `json:"context"`
	SymbolName    string  `json:"symbol_name,omitempty"`
	SymbolType    string  `json:"symbol_type,omitempty"`
	IsDeclaration bool    `json:"is_declaration,omitempty"`
}

// LLMOptimizer provides LLM-specific optimizations and context formatting
// for better AI agent integration and reduced token usage
type LLMOptimizer struct {
	maxContextLength int
	prioritizeRecent bool
}

// NewLLMOptimizer creates a new LLM optimization engine
func NewLLMOptimizer() *LLMOptimizer {
	return &LLMOptimizer{
		maxContextLength: 8000, // Conservative token limit for most LLMs
		prioritizeRecent: true,
	}
}

// OptimizedSearchResult represents search results optimized for LLM consumption
type OptimizedSearchResult struct {
	Summary         string               `json:"summary"`
	KeyFindings     []string             `json:"key_findings"`
	CodeExamples    []CodeExample        `json:"code_examples"`
	Architecture    ArchitectureOverview `json:"architecture"`
	Recommendations []string             `json:"recommendations"`
	TokenEstimate   int                  `json:"token_estimate"`
	SourceFiles     []string             `json:"source_files"`
}

// CodeExample represents a contextually relevant code snippet
type CodeExample struct {
	FilePath    string `json:"file_path"`
	Function    string `json:"function,omitempty"`
	Purpose     string `json:"purpose"`
	Code        string `json:"code"`
	Explanation string `json:"explanation"`
	Relevance   string `json:"relevance"`
	LineRange   string `json:"line_range"`
}

// ArchitectureOverview provides high-level architectural context
type ArchitectureOverview struct {
	Components   []string               `json:"components"`
	Patterns     []string               `json:"patterns"`
	Dependencies []string               `json:"dependencies"`
	Structure    map[string]interface{} `json:"structure"`
	Complexity   string                 `json:"complexity"`
}

// OptimizeForLLM takes raw search results and optimizes them for LLM consumption
func (opt *LLMOptimizer) OptimizeForLLM(query string, results []SearchResult, intent *IntentAnalysisResult, patterns []PatternViolation) *OptimizedSearchResult {
	optimized := &OptimizedSearchResult{
		KeyFindings:     make([]string, 0),
		CodeExamples:    make([]CodeExample, 0),
		Recommendations: make([]string, 0),
		SourceFiles:     make([]string, 0),
	}

	// Generate summary
	optimized.Summary = opt.generateSummary(query, results, intent, patterns)

	// Extract key findings
	optimized.KeyFindings = opt.extractKeyFindings(results, intent, patterns)

	// Select most relevant code examples
	optimized.CodeExamples = opt.selectCodeExamples(results, 3) // Limit to 3 examples

	// Analyze architecture
	optimized.Architecture = opt.analyzeArchitecture(results, intent)

	// Generate recommendations
	optimized.Recommendations = opt.generateRecommendations(intent, patterns)

	// Estimate token usage
	optimized.TokenEstimate = opt.estimateTokens(optimized)

	// Track source files
	for _, result := range results {
		optimized.SourceFiles = append(optimized.SourceFiles, result.File)
	}

	return optimized
}

// generateSummary creates a concise summary for LLM context
func (opt *LLMOptimizer) generateSummary(query string, results []SearchResult, intent *IntentAnalysisResult, patterns []PatternViolation) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Analysis for '%s':\n", query))
	summary.WriteString(fmt.Sprintf("• Found %d relevant code locations\n", len(results)))

	if intent != nil {
		summary.WriteString(fmt.Sprintf("• Primary intent: %s (confidence: %.1f%%)\n",
			intent.PrimaryIntent, intent.IntentScore*100))
		summary.WriteString(fmt.Sprintf("• Domain: %s, Complexity: %s\n",
			intent.Context.Domain, intent.Context.Complexity))
	}

	if len(patterns) > 0 {
		summary.WriteString(fmt.Sprintf("• Found %d architectural issues to address\n", len(patterns)))
	}

	return summary.String()
}

// extractKeyFindings identifies the most important insights
func (opt *LLMOptimizer) extractKeyFindings(results []SearchResult, intent *IntentAnalysisResult, patterns []PatternViolation) []string {
	findings := make([]string, 0)

	// Group results by file to identify patterns
	fileGroups := make(map[string][]SearchResult)
	for _, result := range results {
		fileGroups[result.File] = append(fileGroups[result.File], result)
	}

	// Add findings based on file analysis
	if len(fileGroups) == 1 {
		findings = append(findings, "Implementation concentrated in single file - potential for modularization")
	} else if len(fileGroups) > 5 {
		findings = append(findings, fmt.Sprintf("Implementation spread across %d files - indicates complex feature", len(fileGroups)))
	}

	// Add intent-based findings
	if intent != nil {
		for _, detectedIntent := range intent.Intents {
			if detectedIntent.Confidence > 0.7 {
				findings = append(findings, fmt.Sprintf("High-confidence %s pattern detected", detectedIntent.Category))
			}
		}

		if len(intent.AntiPatterns) > 0 {
			findings = append(findings, fmt.Sprintf("%d anti-patterns detected requiring attention", len(intent.AntiPatterns)))
		}
	}

	// Add pattern findings
	severityCount := make(map[string]int)
	for _, pattern := range patterns {
		severityCount[pattern.Severity]++
	}

	for severity, count := range severityCount {
		if count > 0 {
			findings = append(findings, fmt.Sprintf("%d %s-level architectural issues found", count, severity))
		}
	}

	return findings
}

// selectCodeExamples chooses the most relevant code snippets
func (opt *LLMOptimizer) selectCodeExamples(results []SearchResult, maxExamples int) []CodeExample {
	examples := make([]CodeExample, 0)

	// Sort results by relevance (score)
	sortedResults := make([]SearchResult, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].Score > sortedResults[j].Score
	})

	// Select top examples, avoiding duplicates from same file
	seenFiles := make(map[string]bool)
	for _, result := range sortedResults {
		if len(examples) >= maxExamples {
			break
		}

		// Skip if we already have an example from this file
		if seenFiles[result.File] {
			continue
		}
		seenFiles[result.File] = true

		// Create focused code example
		example := CodeExample{
			FilePath:    result.File,
			Purpose:     opt.inferPurpose(result),
			Code:        opt.extractRelevantCode(result),
			Explanation: opt.generateExplanation(result),
			Relevance:   fmt.Sprintf("Score: %.1f", result.Score),
			LineRange:   fmt.Sprintf("Lines %d-%d", result.Line, result.Line+5),
		}

		// Try to identify function context
		if result.SymbolName != "" {
			example.Function = result.SymbolName
		}

		examples = append(examples, example)
	}

	return examples
}

// analyzeArchitecture provides architectural context
func (opt *LLMOptimizer) analyzeArchitecture(results []SearchResult, intent *IntentAnalysisResult) ArchitectureOverview {
	overview := ArchitectureOverview{
		Components:   make([]string, 0),
		Patterns:     make([]string, 0),
		Dependencies: make([]string, 0),
		Structure:    make(map[string]interface{}),
		Complexity:   "medium",
	}

	// Analyze file paths to infer architecture
	pathComponents := make(map[string]int)
	for _, result := range results {
		parts := strings.Split(result.File, "/")
		for _, part := range parts {
			if part != "" && part != ".go" && part != ".js" && part != ".ts" {
				pathComponents[part]++
			}
		}
	}

	// Extract main components
	for component, count := range pathComponents {
		if count > 1 { // Only include components that appear multiple times
			overview.Components = append(overview.Components, component)
		}
	}

	// Add intent-based patterns
	if intent != nil {
		for _, detectedIntent := range intent.Intents {
			overview.Patterns = append(overview.Patterns, detectedIntent.Category)
		}
		overview.Complexity = intent.Context.Complexity
	}

	// Create structure map
	overview.Structure["total_files"] = len(results)
	overview.Structure["main_components"] = len(overview.Components)
	overview.Structure["detected_patterns"] = len(overview.Patterns)

	return overview
}

// generateRecommendations provides actionable insights
func (opt *LLMOptimizer) generateRecommendations(intent *IntentAnalysisResult, patterns []PatternViolation) []string {
	recommendations := make([]string, 0)

	// Intent-based recommendations
	if intent != nil {
		if intent.Summary.ComplexityScore > 0.7 {
			recommendations = append(recommendations, "Consider refactoring to reduce complexity")
		}

		if len(intent.AntiPatterns) > 0 {
			recommendations = append(recommendations, "Address identified anti-patterns to improve maintainability")
		}

		for _, recommendation := range intent.Summary.Recommendations {
			recommendations = append(recommendations, recommendation)
		}
	}

	// Pattern-based recommendations
	errorPatterns := 0
	for _, pattern := range patterns {
		if pattern.Severity == "error" {
			errorPatterns++
		}
	}

	if errorPatterns > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Fix %d critical architectural violations", errorPatterns))
	}

	// General recommendations
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Code structure looks good - consider adding tests if not present")
	}

	return recommendations
}

// Helper methods for code analysis
func (opt *LLMOptimizer) inferPurpose(result SearchResult) string {
	file := strings.ToLower(result.File)

	if strings.Contains(file, "test") {
		return "Testing"
	} else if strings.Contains(file, "handler") {
		return "Request handling"
	} else if strings.Contains(file, "service") {
		return "Business logic"
	} else if strings.Contains(file, "model") {
		return "Data model"
	} else if strings.Contains(file, "controller") {
		return "Flow control"
	} else if strings.Contains(file, "config") {
		return "Configuration"
	}

	return "Core functionality"
}

func (opt *LLMOptimizer) extractRelevantCode(result SearchResult) string {
	// Extract a focused snippet around the match
	lines := strings.Split(result.Context, "\n")
	if len(lines) <= 10 {
		return result.Context
	}

	// Take middle portion to show context
	start := len(lines)/2 - 3
	end := start + 6

	if start < 0 {
		start = 0
		end = 6
	}
	if end > len(lines) {
		end = len(lines)
		start = end - 6
	}
	if start < 0 {
		start = 0
	}

	return strings.Join(lines[start:end], "\n")
}

func (opt *LLMOptimizer) generateExplanation(result SearchResult) string {
	explanation := "Matches search criteria"

	if result.SymbolType != "" {
		explanation += " as " + result.SymbolType
	}

	if result.IsDeclaration {
		explanation += " (declaration)"
	} else {
		explanation += " (usage)"
	}

	return explanation
}

func (opt *LLMOptimizer) estimateTokens(optimized *OptimizedSearchResult) int {
	// Rough token estimation (1 token ≈ 4 characters for English)
	total := 0

	total += len(optimized.Summary) / 4
	total += len(strings.Join(optimized.KeyFindings, " ")) / 4
	total += len(strings.Join(optimized.Recommendations, " ")) / 4

	for _, example := range optimized.CodeExamples {
		total += len(example.Code) / 4
		total += len(example.Explanation) / 4
	}

	return total
}

// OptimizeForContext reduces content to fit within token limits
func (opt *LLMOptimizer) OptimizeForContext(optimized *OptimizedSearchResult, maxTokens int) *OptimizedSearchResult {
	if optimized.TokenEstimate <= maxTokens {
		return optimized
	}

	// Create a copy
	result := *optimized

	// Progressively reduce content
	if len(result.CodeExamples) > 2 {
		result.CodeExamples = result.CodeExamples[:2]
	}

	if len(result.KeyFindings) > 5 {
		result.KeyFindings = result.KeyFindings[:5]
	}

	if len(result.Recommendations) > 3 {
		result.Recommendations = result.Recommendations[:3]
	}

	// Truncate code examples if still too long
	for i := range result.CodeExamples {
		if len(result.CodeExamples[i].Code) > 500 {
			result.CodeExamples[i].Code = result.CodeExamples[i].Code[:500] + "...[truncated]"
		}
	}

	// Recalculate token estimate
	result.TokenEstimate = opt.estimateTokens(&result)

	return &result
}
