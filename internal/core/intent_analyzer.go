package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// IntentAnalyzer provides semantic understanding of code purpose and intent
// using pattern recognition and contextual analysis
type IntentAnalyzer struct {
	// Intent classification patterns
	intentPatterns map[string]IntentPattern

	// Anti-pattern detection rules
	antiPatterns map[string]AntiPattern

	// Context-aware analyzers
	contextAnalyzers map[string]ContextAnalyzer
}

// IntentPattern defines patterns for identifying code intent
type IntentPattern struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`   // ui, business-logic, data-access, etc.
	Patterns    []string               `json:"patterns"`   // Regex patterns to match
	Keywords    []string               `json:"keywords"`   // Intent-indicating keywords
	Contexts    []string               `json:"contexts"`   // File/package contexts where this applies
	Confidence  float64                `json:"confidence"` // Base confidence score (0.0-1.0)
	Examples    []string               `json:"examples"`   // Example code snippets
	Metadata    map[string]interface{} `json:"metadata"`
}

// AntiPattern defines problematic code patterns that violate best practices
type AntiPattern struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`    // performance, security, maintainability
	Patterns    []string `json:"patterns"`    // Regex patterns that indicate anti-pattern
	Contexts    []string `json:"contexts"`    // Where this anti-pattern applies
	Severity    string   `json:"severity"`    // critical, high, medium, low
	Remediation string   `json:"remediation"` // How to fix the anti-pattern
	Examples    []string `json:"examples"`    // Examples of the anti-pattern
}

// ContextAnalyzer provides domain-specific analysis capabilities
type ContextAnalyzer struct {
	Domain      string                 `json:"domain"` // ui, api, data, etc.
	Description string                 `json:"description"`
	Patterns    []string               `json:"patterns"`     // File/package patterns this applies to
	IntentRules []IntentRule           `json:"intent_rules"` // Domain-specific intent detection
	Metadata    map[string]interface{} `json:"metadata"`
}

// IntentRule defines domain-specific intent detection logic
type IntentRule struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Trigger     string   `json:"trigger"`    // Pattern that triggers this rule
	Intent      string   `json:"intent"`     // The intent this rule identifies
	Confidence  float64  `json:"confidence"` // Confidence multiplier
	Context     []string `json:"context"`    // Additional context requirements
}

// IntentAnalysisResult contains the results of intent analysis
type IntentAnalysisResult struct {
	FileID        types.FileID           `json:"file_id"`
	FilePath      string                 `json:"file_path"`
	PrimaryIntent string                 `json:"primary_intent"`
	IntentScore   float64                `json:"intent_score"`
	Intents       []DetectedIntent       `json:"intents"`
	AntiPatterns  []DetectedAntiPattern  `json:"anti_patterns"`
	Context       AnalysisContext        `json:"context"`
	Summary       IntentSummary          `json:"summary"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// DetectedIntent represents an identified code intent
type DetectedIntent struct {
	Intent      string                 `json:"intent"`
	Category    string                 `json:"category"`
	Confidence  float64                `json:"confidence"`
	Evidence    []string               `json:"evidence"` // Code patterns that support this intent
	Location    IntentLocation         `json:"location"` // Where in the code this intent was found
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// DetectedAntiPattern represents an identified anti-pattern
type DetectedAntiPattern struct {
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Severity    string                 `json:"severity"`
	Confidence  float64                `json:"confidence"`
	Evidence    []string               `json:"evidence"`
	Location    IntentLocation         `json:"location"`
	Remediation string                 `json:"remediation"`
	Impact      string                 `json:"impact"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// IntentLocation specifies where an intent or anti-pattern was detected
type IntentLocation struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Function  string `json:"function"`
	Symbol    string `json:"symbol"`
	Context   string `json:"context"`
}

// AnalysisContext provides contextual information for intent analysis
type AnalysisContext struct {
	Domain       string   `json:"domain"`       // ui, api, data, etc.
	Framework    string   `json:"framework"`    // detected framework (if any)
	Language     string   `json:"language"`     // programming language
	FileType     string   `json:"file_type"`    // source, test, config, etc.
	Complexity   string   `json:"complexity"`   // low, medium, high
	Dependencies []string `json:"dependencies"` // key dependencies detected
}

// IntentSummary provides high-level analysis summary
type IntentSummary struct {
	TotalIntents     int      `json:"total_intents"`
	PrimaryCategory  string   `json:"primary_category"`
	ConfidenceScore  float64  `json:"confidence_score"`
	AntiPatternCount int      `json:"anti_pattern_count"`
	ComplexityScore  float64  `json:"complexity_score"`
	Recommendations  []string `json:"recommendations"`
}

// IntentAnalysisOptions is imported from types package

// NewIntentAnalyzer creates a new intent analysis engine
func NewIntentAnalyzer() *IntentAnalyzer {
	ia := &IntentAnalyzer{
		intentPatterns:   make(map[string]IntentPattern),
		antiPatterns:     make(map[string]AntiPattern),
		contextAnalyzers: make(map[string]ContextAnalyzer),
	}

	ia.initializeBuiltInPatterns()
	return ia
}

// initializeBuiltInPatterns sets up common intent patterns and anti-patterns
func (ia *IntentAnalyzer) initializeBuiltInPatterns() {
	// UI/View Intent Patterns
	ia.intentPatterns["size_management"] = IntentPattern{
		Name:        "Size Management",
		Description: "Code that handles component sizing and layout",
		Category:    "ui",
		Patterns: []string{
			`(?i)(height|width|size).*\s*[:=]`,
			`(?i)func.*(Render|View|Layout)`,
			`(?i)(SetSize|GetSize|Resize)`,
		},
		Keywords:   []string{"height", "width", "size", "dimensions", "layout", "render"},
		Contexts:   []string{"**/ui/**", "**/view/**", "**/component/**"},
		Confidence: 0.8,
		Examples:   []string{"func (v *View) Render() { v.width = 80 }"},
	}

	ia.intentPatterns["event_handling"] = IntentPattern{
		Name:        "Event Handling",
		Description: "Code that processes user input and events",
		Category:    "ui",
		Patterns: []string{
			`(?i)func.*(Handle|Process|On)[A-Z]`,
			`(?i)(KeyPress|MouseClick|Input)`,
			`(?i)(Event|Handler|Listener)`,
		},
		Keywords:   []string{"event", "handler", "input", "key", "mouse", "click"},
		Contexts:   []string{"**/handler/**", "**/event/**", "**/ui/**"},
		Confidence: 0.9,
		Examples:   []string{"func HandleKeyPress(key Key) { ... }"},
	}

	ia.intentPatterns["data_transformation"] = IntentPattern{
		Name:        "Data Transformation",
		Description: "Code that converts or processes data between formats",
		Category:    "business-logic",
		Patterns: []string{
			`(?i)func.*(Transform|Convert|Parse|Format)`,
			`(?i)(Marshal|Unmarshal|Serialize)`,
			`(?i)(Encode|Decode|Map)`,
		},
		Keywords:   []string{"transform", "convert", "parse", "format", "map", "serialize"},
		Contexts:   []string{"**/transform/**", "**/convert/**", "**/mapper/**"},
		Confidence: 0.85,
		Examples:   []string{"func TransformUserData(raw []byte) User { ... }"},
	}

	ia.intentPatterns["configuration"] = IntentPattern{
		Name:        "Configuration Management",
		Description: "Code that manages application settings and configuration",
		Category:    "configuration",
		Patterns: []string{
			`(?i)(Config|Setting|Option)`,
			`(?i)func.*(Load|Save|Get|Set)Config`,
			`(?i)(Default|Override|Env)`,
		},
		Keywords:   []string{"config", "setting", "option", "default", "env", "load", "save"},
		Contexts:   []string{"**/config/**", "**/settings/**", "**/env/**"},
		Confidence: 0.75,
		Examples:   []string{"func LoadConfig(path string) *Config { ... }"},
	}

	// Anti-Patterns
	ia.antiPatterns["hardcoded_dimensions"] = AntiPattern{
		Name:        "Hardcoded Dimensions",
		Description: "UI components with hardcoded size values instead of responsive design",
		Category:    "maintainability",
		Patterns: []string{
			`(?i)(height|width)\s*[:=]\s*\d+`,
			`(?i)(SetSize|Resize)\(\d+,\s*\d+\)`,
			`(?i)(80|120|24|40)\s*(//.*terminal|//.*size)`,
		},
		Contexts:    []string{"**/ui/**", "**/view/**", "**/component/**"},
		Severity:    "medium",
		Remediation: "Use responsive design patterns, pass dimensions as parameters",
		Examples:    []string{"width := 80 // BAD: hardcoded terminal width"},
	}

	ia.antiPatterns["global_state_mutation"] = AntiPattern{
		Name:        "Global State Mutation",
		Description: "Direct mutation of global state without proper coordination",
		Category:    "architecture",
		Patterns: []string{
			`(?i)global\.[A-Z][a-zA-Z]*\s*=`,
			`(?i)(GlobalState|AppState)\.[A-Z][a-zA-Z]*\s*=`,
			`(?i)var\s+[A-Z][a-zA-Z]*\s*=.*//.*global`,
		},
		Contexts:    []string{"**/*.go"},
		Severity:    "high",
		Remediation: "Use dependency injection or state management patterns",
		Examples:    []string{"global.Config = newConfig // BAD: direct global mutation"},
	}

	ia.antiPatterns["missing_error_handling"] = AntiPattern{
		Name:        "Missing Error Handling",
		Description: "Functions that can fail but don't return or handle errors properly",
		Category:    "reliability",
		Patterns: []string{
			`(?i)func.*\([^)]*\)\s*[^{]*\{[^}]*\}`, // Single line functions
			`(?i)(Parse|Convert|Load|Save|Get|Set).*\([^)]*\)\s*[^{]*\{`,
		},
		Contexts:    []string{"**/*.go"},
		Severity:    "high",
		Remediation: "Add proper error handling and return error values",
		Examples:    []string{"func ParseInt(s string) int { i, _ := strconv.Atoi(s); return i }"},
	}

	// Context Analyzers
	ia.contextAnalyzers["bubble_tea"] = ContextAnalyzer{
		Domain:      "ui",
		Description: "Bubble Tea TUI application analysis",
		Patterns:    []string{"**/tui/**", "**/ui/**", "**/*view*.go"},
		IntentRules: []IntentRule{
			{
				Name:        "view_component",
				Description: "Detects Bubble Tea view components",
				Trigger:     `(?i)func.*View\(\).*string`,
				Intent:      "ui_rendering",
				Confidence:  0.9,
				Context:     []string{"bubble_tea", "tui"},
			},
			{
				Name:        "model_update",
				Description: "Detects Bubble Tea model update logic",
				Trigger:     `(?i)func.*Update\(.*tea\.Msg\)`,
				Intent:      "state_management",
				Confidence:  0.95,
				Context:     []string{"bubble_tea", "tui"},
			},
		},
	}
}

// AnalyzeIntent performs intent analysis on the given files
func (ia *IntentAnalyzer) AnalyzeIntent(options types.IntentAnalysisOptions, files map[types.FileID]string, symbols map[types.FileID][]types.Symbol, fileContent map[types.FileID]string) ([]*IntentAnalysisResult, error) {
	var results []*IntentAnalysisResult

	// Filter files based on scope
	targetFiles := ia.filterFilesByScope(options.Scope, files)

	// Analyze each file
	for fileID, filePath := range targetFiles {
		content := fileContent[fileID]
		fileSymbols := symbols[fileID]

		result := ia.analyzeFile(fileID, filePath, content, fileSymbols, options)

		// Apply confidence threshold
		if result.IntentScore >= options.MinConfidence {
			results = append(results, result)
		}
	}

	// Sort by confidence score
	sort.Slice(results, func(i, j int) bool {
		return results[i].IntentScore > results[j].IntentScore
	})

	// Apply max results limit
	if options.MaxResults > 0 && len(results) > options.MaxResults {
		results = results[:options.MaxResults]
	}

	return results, nil
}

// analyzeFile performs intent analysis on a single file
func (ia *IntentAnalyzer) analyzeFile(fileID types.FileID, filePath, content string, symbols []types.Symbol, options types.IntentAnalysisOptions) *IntentAnalysisResult {
	result := &IntentAnalysisResult{
		FileID:       fileID,
		FilePath:     filePath,
		Intents:      make([]DetectedIntent, 0),
		AntiPatterns: make([]DetectedAntiPattern, 0),
		Context:      ia.analyzeContext(filePath, content, symbols),
		Metadata:     make(map[string]interface{}),
	}

	// Detect intents
	detectedIntents := ia.detectIntents(content, symbols, options)
	result.Intents = detectedIntents

	// Determine primary intent
	if len(detectedIntents) > 0 {
		// Sort by confidence and take the highest
		sort.Slice(detectedIntents, func(i, j int) bool {
			return detectedIntents[i].Confidence > detectedIntents[j].Confidence
		})
		result.PrimaryIntent = detectedIntents[0].Intent
		result.IntentScore = detectedIntents[0].Confidence
	}

	// Detect anti-patterns
	if len(options.AntiPatterns) == 0 || contains(options.AntiPatterns, "*") {
		result.AntiPatterns = ia.detectAntiPatterns(content, symbols, options)
	} else {
		result.AntiPatterns = ia.detectSpecificAntiPatterns(content, symbols, options.AntiPatterns, options)
	}

	// Generate summary
	result.Summary = ia.generateSummary(result)

	return result
}

// detectIntents identifies code intents in the given content
func (ia *IntentAnalyzer) detectIntents(content string, symbols []types.Symbol, options types.IntentAnalysisOptions) []DetectedIntent {
	var intents []DetectedIntent

	// If specific intent requested, only analyze that one
	if options.Intent != "" {
		if pattern, exists := ia.intentPatterns[options.Intent]; exists {
			if intent := ia.matchIntentPattern(pattern, content, symbols); intent != nil {
				intents = append(intents, *intent)
			}
		}
		return intents
	}

	// Analyze all intent patterns
	for _, pattern := range ia.intentPatterns {
		if intent := ia.matchIntentPattern(pattern, content, symbols); intent != nil {
			intents = append(intents, *intent)
		}
	}

	return intents
}

// matchIntentPattern checks if content matches an intent pattern
func (ia *IntentAnalyzer) matchIntentPattern(pattern IntentPattern, content string, symbols []types.Symbol) *DetectedIntent {
	var evidence []string
	var totalConfidence float64
	var matches int

	// Check regex patterns
	for _, patternRegex := range pattern.Patterns {
		if regex, err := regexp.Compile(patternRegex); err == nil {
			if matchStrings := regex.FindAllString(content, -1); len(matchStrings) > 0 {
				evidence = append(evidence, matchStrings...)
				totalConfidence += pattern.Confidence
				matches++
			}
		}
	}

	// Check keywords
	for _, keyword := range pattern.Keywords {
		if strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
			evidence = append(evidence, "keyword: "+keyword)
			totalConfidence += 0.1 // Small confidence boost for keywords
			matches++
		}
	}

	// Must have at least one match to be considered
	if matches == 0 {
		return nil
	}

	// Calculate final confidence (average with boost for multiple matches)
	confidence := totalConfidence / float64(len(pattern.Patterns)+len(pattern.Keywords))
	if matches > 1 {
		confidence *= 1.2 // Boost for multiple evidence points
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return &DetectedIntent{
		Intent:      pattern.Name,
		Category:    pattern.Category,
		Confidence:  confidence,
		Evidence:    evidence,
		Description: pattern.Description,
		Location:    ia.findIntentLocation(content, evidence),
		Metadata: map[string]interface{}{
			"matches":       matches,
			"pattern_count": len(pattern.Patterns),
			"keyword_count": len(pattern.Keywords),
		},
	}
}

// detectAntiPatterns identifies anti-patterns in the given content
func (ia *IntentAnalyzer) detectAntiPatterns(content string, symbols []types.Symbol, options types.IntentAnalysisOptions) []DetectedAntiPattern {
	var antiPatterns []DetectedAntiPattern

	for _, pattern := range ia.antiPatterns {
		if ap := ia.matchAntiPattern(pattern, content, symbols); ap != nil {
			antiPatterns = append(antiPatterns, *ap)
		}
	}

	return antiPatterns
}

// detectSpecificAntiPatterns detects only specified anti-patterns
func (ia *IntentAnalyzer) detectSpecificAntiPatterns(content string, symbols []types.Symbol, patternNames []string, options types.IntentAnalysisOptions) []DetectedAntiPattern {
	var antiPatterns []DetectedAntiPattern

	for _, name := range patternNames {
		if pattern, exists := ia.antiPatterns[name]; exists {
			if ap := ia.matchAntiPattern(pattern, content, symbols); ap != nil {
				antiPatterns = append(antiPatterns, *ap)
			}
		}
	}

	return antiPatterns
}

// matchAntiPattern checks if content matches an anti-pattern
func (ia *IntentAnalyzer) matchAntiPattern(pattern AntiPattern, content string, symbols []types.Symbol) *DetectedAntiPattern {
	var evidence []string

	for _, patternRegex := range pattern.Patterns {
		if regex, err := regexp.Compile(patternRegex); err == nil {
			if matches := regex.FindAllString(content, -1); len(matches) > 0 {
				evidence = append(evidence, matches...)
			}
		}
	}

	if len(evidence) == 0 {
		return nil
	}

	return &DetectedAntiPattern{
		Name:        pattern.Name,
		Category:    pattern.Category,
		Severity:    pattern.Severity,
		Confidence:  0.8, // Base confidence for anti-pattern detection
		Evidence:    evidence,
		Location:    ia.findIntentLocation(content, evidence),
		Remediation: pattern.Remediation,
		Impact:      fmt.Sprintf("%s impact on code %s", pattern.Severity, pattern.Category),
		Metadata: map[string]interface{}{
			"pattern_count":  len(pattern.Patterns),
			"evidence_count": len(evidence),
		},
	}
}

// Helper functions

func (ia *IntentAnalyzer) filterFilesByScope(scope string, files map[types.FileID]string) map[types.FileID]string {
	if scope == "" || scope == "*" {
		return files
	}

	filtered := make(map[types.FileID]string)
	for fileID, filePath := range files {
		if ia.matchesScope(scope, filePath) {
			filtered[fileID] = filePath
		}
	}

	return filtered
}

func (ia *IntentAnalyzer) matchesScope(scope, filePath string) bool {
	// Simple glob pattern matching (could be enhanced with filepath.Match)
	if strings.Contains(scope, "*") {
		pattern := strings.ReplaceAll(scope, "*", ".*")
		if regex, err := regexp.Compile(pattern); err == nil {
			return regex.MatchString(filePath)
		}
	}
	return strings.Contains(filePath, scope)
}

func (ia *IntentAnalyzer) analyzeContext(filePath, content string, symbols []types.Symbol) AnalysisContext {
	context := AnalysisContext{
		Language: "go", // Default to Go for now
		FileType: "source",
	}

	// Detect domain based on file path
	if strings.Contains(filePath, "ui") || strings.Contains(filePath, "view") || strings.Contains(filePath, "tui") {
		context.Domain = "ui"
	} else if strings.Contains(filePath, "api") || strings.Contains(filePath, "handler") {
		context.Domain = "api"
	} else if strings.Contains(filePath, "config") {
		context.Domain = "configuration"
	} else {
		context.Domain = "business-logic"
	}

	// Detect test files
	if strings.Contains(filePath, "_test.go") {
		context.FileType = "test"
	}

	// Basic complexity assessment
	lines := strings.Split(content, "\n")
	if len(lines) < 50 {
		context.Complexity = "low"
	} else if len(lines) < 200 {
		context.Complexity = "medium"
	} else {
		context.Complexity = "high"
	}

	return context
}

func (ia *IntentAnalyzer) findIntentLocation(content string, evidence []string) IntentLocation {
	location := IntentLocation{}

	if len(evidence) > 0 {
		// Find the first evidence in content
		evidenceText := evidence[0]
		lines := strings.Split(content, "\n")

		for i, line := range lines {
			if strings.Contains(line, evidenceText) {
				location.StartLine = i + 1
				location.EndLine = i + 1
				location.Context = strings.TrimSpace(line)
				break
			}
		}
	}

	return location
}

func (ia *IntentAnalyzer) generateSummary(result *IntentAnalysisResult) IntentSummary {
	summary := IntentSummary{
		TotalIntents:     len(result.Intents),
		AntiPatternCount: len(result.AntiPatterns),
		ConfidenceScore:  result.IntentScore,
	}

	// Determine primary category
	if len(result.Intents) > 0 {
		summary.PrimaryCategory = result.Intents[0].Category
	}

	// Generate recommendations
	if len(result.AntiPatterns) > 0 {
		summary.Recommendations = append(summary.Recommendations, "Address detected anti-patterns to improve code quality")
	}
	if result.Context.Complexity == "high" {
		summary.Recommendations = append(summary.Recommendations, "Consider refactoring to reduce complexity")
	}

	// Calculate complexity score based on various factors
	summary.ComplexityScore = ia.calculateComplexityScore(result)

	return summary
}

func (ia *IntentAnalyzer) calculateComplexityScore(result *IntentAnalysisResult) float64 {
	score := 0.5 // Base complexity

	// Anti-patterns increase complexity
	score += float64(len(result.AntiPatterns)) * 0.2

	// Multiple intents might indicate complexity
	if len(result.Intents) > 3 {
		score += 0.3
	}

	// Context complexity
	switch result.Context.Complexity {
	case "low":
		score += 0.1
	case "medium":
		score += 0.3
	case "high":
		score += 0.5
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// Utility functions - use contains from context_lookup_usage.go
