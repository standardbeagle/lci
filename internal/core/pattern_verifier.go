package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// PatternVerifier validates architectural patterns and compliance rules
type PatternVerifier struct {
	// Predefined architectural pattern rules
	architecturalPatterns map[string]ArchitecturalPattern

	// Custom verification rules
	customRules map[string]VerificationRule
}

// ArchitecturalPattern defines common architectural patterns
type ArchitecturalPattern struct {
	Name        string
	Description string
	Scope       []string      // File patterns this applies to
	Rules       []PatternRule // Rules that must be satisfied
}

// PatternRule defines a specific verification rule
type PatternRule struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Pattern        string   `json:"pattern"`          // Regex pattern to match functions/symbols
	MustContain    []string `json:"must_contain"`     // Required patterns that must be present
	MustNotContain []string `json:"must_not_contain"` // Anti-patterns that must be absent
	FileScope      string   `json:"file_scope"`       // File pattern scope for this rule
	Severity       string   `json:"severity"`         // error, warning, info
}

// VerificationRule is a custom user-defined rule
type VerificationRule struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Scope       string        `json:"scope"`  // File pattern scope
	Checks      []PatternRule `json:"checks"` // List of checks to perform
	Enabled     bool          `json:"enabled"`
}

// VerificationResult contains the results of pattern verification
type VerificationResult struct {
	RuleName     string                 `json:"rule_name"`
	Scope        string                 `json:"scope"`
	TotalFiles   int                    `json:"total_files"`
	CheckedFiles int                    `json:"checked_files"`
	Violations   []PatternViolation     `json:"violations"`
	Summary      VerificationSummary    `json:"summary"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// PatternViolation represents a single violation found
type PatternViolation struct {
	File       string                 `json:"file"`
	FileID     types.FileID           `json:"file_id"`
	Rule       string                 `json:"rule"`
	Severity   string                 `json:"severity"`
	Line       int                    `json:"line"`
	Symbol     string                 `json:"symbol"`
	Issue      string                 `json:"issue"`
	Context    string                 `json:"context"`
	Suggestion string                 `json:"suggestion"`
	Evidence   []string               `json:"evidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// VerificationSummary provides aggregated results
type VerificationSummary struct {
	TotalViolations int      `json:"total_violations"`
	ErrorCount      int      `json:"error_count"`
	WarningCount    int      `json:"warning_count"`
	InfoCount       int      `json:"info_count"`
	ComplianceScore float64  `json:"compliance_score"`
	RulesSatisfied  []string `json:"rules_satisfied"`
	RulesViolated   []string `json:"rules_violated"`
	FilesAnalyzed   []string `json:"files_analyzed"`
}

// NewPatternVerifier creates a new pattern verification engine
func NewPatternVerifier() *PatternVerifier {
	pv := &PatternVerifier{
		architecturalPatterns: make(map[string]ArchitecturalPattern),
		customRules:           make(map[string]VerificationRule),
	}

	pv.initializeBuiltInPatterns()
	return pv
}

// initializeBuiltInPatterns sets up common architectural patterns
func (pv *PatternVerifier) initializeBuiltInPatterns() {
	// MVC Pattern Verification
	pv.architecturalPatterns["mvc_separation"] = ArchitecturalPattern{
		Name:        "MVC Separation",
		Description: "Enforce Model-View-Controller separation of concerns",
		Scope:       []string{"**/controllers/**", "**/models/**", "**/views/**"},
		Rules: []PatternRule{
			{
				Name:           "controllers_no_direct_db",
				Description:    "Controllers should not contain direct database operations",
				Pattern:        `(?i)func.*Controller.*\{`,
				MustNotContain: []string{`sql\.`, `db\.Exec`, `db\.Query`, `\.Save\(`, `\.Delete\(`},
				FileScope:      "**/controllers/**",
				Severity:       "error",
			},
			{
				Name:           "models_no_http_logic",
				Description:    "Models should not contain HTTP-specific logic",
				Pattern:        `(?i)(struct.*Model|type.*Model)`,
				MustNotContain: []string{`http\.`, `gin\.`, `echo\.`, `ResponseWriter`, `Request`},
				FileScope:      "**/models/**",
				Severity:       "error",
			},
		},
	}

	// Repository Pattern Verification
	pv.architecturalPatterns["repository_pattern"] = ArchitecturalPattern{
		Name:        "Repository Pattern",
		Description: "Enforce repository pattern for data access",
		Scope:       []string{"**/repositories/**", "**/repos/**"},
		Rules: []PatternRule{
			{
				Name:        "repository_interface_compliance",
				Description: "Repository implementations should implement defined interfaces",
				Pattern:     `(?i)type.*Repository.*struct`,
				MustContain: []string{`interface`},
				FileScope:   "**/repositories/**",
				Severity:    "warning",
			},
			{
				Name:        "repository_error_handling",
				Description: "Repository methods should return errors",
				Pattern:     `(?i)func.*\(.*Repository.*\).*\(.*\)`,
				MustContain: []string{`error`},
				FileScope:   "**/repositories/**",
				Severity:    "error",
			},
		},
	}

	// API Handler Pattern Verification
	pv.architecturalPatterns["api_handler_pattern"] = ArchitecturalPattern{
		Name:        "API Handler Pattern",
		Description: "Enforce consistent API handler implementation",
		Scope:       []string{"**/handlers/**", "**/api/**"},
		Rules: []PatternRule{
			{
				Name:        "handler_error_responses",
				Description: "API handlers should have proper error response handling",
				Pattern:     `(?i)func.*Handler.*\(.*\)`,
				MustContain: []string{`error`, `status`},
				FileScope:   "**/handlers/**",
				Severity:    "error",
			},
			{
				Name:        "handler_input_validation",
				Description: "API handlers should validate input parameters",
				Pattern:     `(?i)func.*Handler.*\(.*\)`,
				MustContain: []string{`Valid`, `Parse`, `Bind`},
				FileScope:   "**/handlers/**",
				Severity:    "warning",
			},
		},
	}

	// Service Layer Pattern Verification
	pv.architecturalPatterns["service_layer_pattern"] = ArchitecturalPattern{
		Name:        "Service Layer Pattern",
		Description: "Enforce service layer patterns and dependency injection",
		Scope:       []string{"**/services/**"},
		Rules: []PatternRule{
			{
				Name:           "service_dependency_injection",
				Description:    "Services should use dependency injection instead of global state",
				Pattern:        `(?i)type.*Service.*struct`,
				MustNotContain: []string{`global\.`, `singleton\.`, `instance\.`},
				FileScope:      "**/services/**",
				Severity:       "warning",
			},
			{
				Name:        "service_interface_definition",
				Description: "Services should define clear interfaces",
				Pattern:     `(?i)type.*Service.*interface`,
				MustContain: []string{`interface`},
				FileScope:   "**/services/**",
				Severity:    "info",
			},
		},
	}

	// Testing Pattern Verification
	pv.architecturalPatterns["testing_patterns"] = ArchitecturalPattern{
		Name:        "Testing Patterns",
		Description: "Enforce testing best practices and patterns",
		Scope:       []string{"**/*_test.go", "**/test/**", "**/tests/**"},
		Rules: []PatternRule{
			{
				Name:           "test_isolation",
				Description:    "Tests should be isolated and not depend on external state",
				Pattern:        `(?i)func.*Test.*\(.*\)`,
				MustNotContain: []string{`time\.Sleep`, `global\.`, `singleton\.`},
				FileScope:      "**/*_test.go",
				Severity:       "warning",
			},
			{
				Name:        "test_naming_convention",
				Description: "Test functions should follow naming conventions",
				Pattern:     `func (Test|Benchmark).*\(`,
				MustContain: []string{`Test`, `Benchmark`},
				FileScope:   "**/*_test.go",
				Severity:    "info",
			},
		},
	}

	// Security Pattern Verification
	pv.architecturalPatterns["security_patterns"] = ArchitecturalPattern{
		Name:        "Security Patterns",
		Description: "Enforce security best practices",
		Scope:       []string{"**/*.go", "**/*.js", "**/*.ts"},
		Rules: []PatternRule{
			{
				Name:        "no_hardcoded_credentials",
				Description: "Code should not contain hardcoded credentials",
				Pattern:     `(?i)(password|secret|token|key)\s*[:=]\s*["'][^"']+["']`,
				FileScope:   "**/*",
				Severity:    "error",
			},
			{
				Name:        "input_sanitization",
				Description: "User input should be properly sanitized",
				Pattern:     `(?i)(input|param|query|body).*\.(get|post|put|delete)`,
				MustContain: []string{`Sanitize`, `Validate`, `Escape`},
				FileScope:   "**/handlers/**",
				Severity:    "error",
			},
		},
	}
}

// VerifyPattern runs pattern verification with the given rule
func (pv *PatternVerifier) VerifyPattern(rule VerificationRule, files map[types.FileID]string, symbols map[types.FileID][]types.Symbol, fileContent map[types.FileID]string) (*VerificationResult, error) {
	result := &VerificationResult{
		RuleName:   rule.Name,
		Scope:      rule.Scope,
		Violations: make([]PatternViolation, 0),
		Metadata:   make(map[string]interface{}),
	}

	// Find files matching the scope
	matchingFiles := pv.findMatchingFiles(rule.Scope, files)
	result.TotalFiles = len(files)
	result.CheckedFiles = len(matchingFiles)

	// Run each check against matching files
	for _, check := range rule.Checks {
		violations := pv.runPatternCheck(check, matchingFiles, symbols, fileContent)
		result.Violations = append(result.Violations, violations...)
	}

	// Generate summary
	result.Summary = pv.generateSummary(result.Violations, matchingFiles)
	result.Metadata["rule_description"] = rule.Description
	result.Metadata["check_count"] = len(rule.Checks)

	return result, nil
}

// VerifyArchitecturalPattern runs verification for a built-in architectural pattern
func (pv *PatternVerifier) VerifyArchitecturalPattern(patternName string, files map[types.FileID]string, symbols map[types.FileID][]types.Symbol, fileContent map[types.FileID]string) (*VerificationResult, error) {
	pattern, exists := pv.architecturalPatterns[patternName]
	if !exists {
		return nil, fmt.Errorf("architectural pattern not found: %s", patternName)
	}

	// Convert to verification rule format
	rule := VerificationRule{
		Name:        pattern.Name,
		Description: pattern.Description,
		Scope:       strings.Join(pattern.Scope, ","),
		Checks:      pattern.Rules,
		Enabled:     true,
	}

	return pv.VerifyPattern(rule, files, symbols, fileContent)
}

// runPatternCheck executes a single pattern check
func (pv *PatternVerifier) runPatternCheck(check PatternRule, files map[types.FileID]string, symbols map[types.FileID][]types.Symbol, fileContent map[types.FileID]string) []PatternViolation {
	var violations []PatternViolation

	// Compile the main pattern regex
	mainPattern, err := regexp.Compile(check.Pattern)
	if err != nil {
		return violations // Skip invalid patterns
	}

	// Process each file
	for fileID, filePath := range files {
		// Check if file matches the check's scope
		if check.FileScope != "" && !pv.matchesGlobPattern(check.FileScope, filePath) {
			continue
		}

		content := fileContent[fileID]
		fileSymbols := symbols[fileID]

		// Find matching symbols/functions
		if mainPattern.MatchString(content) {
			violation := pv.checkPatternViolations(check, fileID, filePath, content, fileSymbols, mainPattern)
			if violation != nil {
				violations = append(violations, *violation)
			}
		}
	}

	return violations
}

// checkPatternViolations checks for must_contain and must_not_contain violations
func (pv *PatternVerifier) checkPatternViolations(check PatternRule, fileID types.FileID, filePath, content string, symbols []types.Symbol, mainPattern *regexp.Regexp) *PatternViolation {
	// Find matches for the main pattern
	matches := mainPattern.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	// For security patterns like hardcoded credentials, the main pattern match itself is a violation
	// when there are no MustContain or MustNotContain requirements
	if len(check.MustContain) == 0 && len(check.MustNotContain) == 0 {
		return &PatternViolation{
			File:       filePath,
			FileID:     fileID,
			Rule:       check.Name,
			Severity:   check.Severity,
			Line:       pv.getLineNumber(content, matches[0][0]),
			Symbol:     pv.extractSymbolName(content, matches[0][0], matches[0][1]),
			Issue:      "Pattern violation: " + check.Description,
			Context:    pv.getContext(content, matches[0][0], matches[0][1]),
			Suggestion: "Fix pattern violation to comply with " + check.Description,
			Evidence:   []string{fmt.Sprintf("Pattern '%s' found in file", check.Pattern)},
			Metadata:   map[string]interface{}{"check_type": "pattern_match"},
		}
	}

	// Check must_contain requirements
	for _, required := range check.MustContain {
		requiredPattern, err := regexp.Compile(required)
		if err != nil {
			continue
		}
		if !requiredPattern.MatchString(content) {
			return &PatternViolation{
				File:       filePath,
				FileID:     fileID,
				Rule:       check.Name,
				Severity:   check.Severity,
				Line:       pv.getLineNumber(content, matches[0][0]),
				Symbol:     pv.extractSymbolName(content, matches[0][0], matches[0][1]),
				Issue:      "Missing required pattern: " + required,
				Context:    pv.getContext(content, matches[0][0], matches[0][1]),
				Suggestion: fmt.Sprintf("Add %s to satisfy %s", required, check.Description),
				Evidence:   []string{fmt.Sprintf("Pattern '%s' not found in file", required)},
				Metadata:   map[string]interface{}{"check_type": "must_contain"},
			}
		}
	}

	// Check must_not_contain restrictions (anti-patterns)
	for _, forbidden := range check.MustNotContain {
		forbiddenPattern, err := regexp.Compile(forbidden)
		if err != nil {
			continue
		}
		if forbiddenPattern.MatchString(content) {
			forbiddenMatches := forbiddenPattern.FindAllStringIndex(content, -1)
			return &PatternViolation{
				File:       filePath,
				FileID:     fileID,
				Rule:       check.Name,
				Severity:   check.Severity,
				Line:       pv.getLineNumber(content, forbiddenMatches[0][0]),
				Symbol:     pv.extractSymbolName(content, forbiddenMatches[0][0], forbiddenMatches[0][1]),
				Issue:      "Forbidden pattern found: " + forbidden,
				Context:    pv.getContext(content, forbiddenMatches[0][0], forbiddenMatches[0][1]),
				Suggestion: fmt.Sprintf("Remove or refactor %s to comply with %s", forbidden, check.Description),
				Evidence:   []string{fmt.Sprintf("Anti-pattern '%s' found in file", forbidden)},
				Metadata:   map[string]interface{}{"check_type": "must_not_contain"},
			}
		}
	}

	return nil
}

// Helper functions

func (pv *PatternVerifier) findMatchingFiles(scope string, files map[types.FileID]string) map[types.FileID]string {
	if scope == "" || scope == "*" {
		return files
	}

	matching := make(map[types.FileID]string)
	patterns := strings.Split(scope, ",")

	for fileID, filePath := range files {
		for _, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)
			if pv.matchesGlobPattern(pattern, filePath) {
				matching[fileID] = filePath
				break
			}
		}
	}

	return matching
}

func (pv *PatternVerifier) matchesGlobPattern(pattern, path string) bool {
	// Simple glob pattern matching
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Check if pattern contains ** for recursive matching
	if strings.Contains(pattern, "**") {
		// Convert ** pattern to regex
		regexPattern := strings.ReplaceAll(pattern, "**", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "*", "[^/]*")
		regex, err := regexp.Compile(regexPattern)
		if err == nil {
			return regex.MatchString(path)
		}
	}

	return false
}

func (pv *PatternVerifier) getLineNumber(content string, position int) int {
	lines := strings.Split(content[:position], "\n")
	return len(lines)
}

func (pv *PatternVerifier) extractSymbolName(content string, start, end int) string {
	// Extract text around the match to find symbol name
	context := content[start:end]

	// Try to extract function/struct/type name
	if strings.Contains(context, "func") {
		parts := strings.Fields(context)
		for i, part := range parts {
			if part == "func" && i+1 < len(parts) {
				name := parts[i+1]
				if idx := strings.Index(name, "("); idx > 0 {
					name = name[:idx]
				}
				return name
			}
		}
	}

	return strings.TrimSpace(context)
}

func (pv *PatternVerifier) getContext(content string, start, end int) string {
	// Get surrounding context (3 lines before and after)
	lines := strings.Split(content, "\n")
	lineNum := pv.getLineNumber(content, start) - 1

	contextStart := lineNum - 3
	if contextStart < 0 {
		contextStart = 0
	}

	contextEnd := lineNum + 4
	if contextEnd > len(lines) {
		contextEnd = len(lines)
	}

	return strings.Join(lines[contextStart:contextEnd], "\n")
}

func (pv *PatternVerifier) generateSummary(violations []PatternViolation, analyzedFiles map[types.FileID]string) VerificationSummary {
	summary := VerificationSummary{
		TotalViolations: len(violations),
		RulesSatisfied:  make([]string, 0),
		RulesViolated:   make([]string, 0),
		FilesAnalyzed:   make([]string, 0),
	}

	violatedRules := make(map[string]bool)

	for _, violation := range violations {
		switch violation.Severity {
		case "error":
			summary.ErrorCount++
		case "warning":
			summary.WarningCount++
		case "info":
			summary.InfoCount++
		}

		violatedRules[violation.Rule] = true
	}

	for rule := range violatedRules {
		summary.RulesViolated = append(summary.RulesViolated, rule)
	}

	for _, filePath := range analyzedFiles {
		summary.FilesAnalyzed = append(summary.FilesAnalyzed, filePath)
	}

	// Calculate compliance score (percentage of files without violations)
	if len(analyzedFiles) > 0 {
		filesWithViolations := make(map[string]bool)
		for _, violation := range violations {
			filesWithViolations[violation.File] = true
		}

		cleanFiles := len(analyzedFiles) - len(filesWithViolations)
		summary.ComplianceScore = float64(cleanFiles) / float64(len(analyzedFiles)) * 100.0
	}

	return summary
}

// GetAvailablePatterns returns list of built-in architectural patterns
func (pv *PatternVerifier) GetAvailablePatterns() []string {
	patterns := make([]string, 0, len(pv.architecturalPatterns))
	for name := range pv.architecturalPatterns {
		patterns = append(patterns, name)
	}
	return patterns
}

// GetPatternDetails returns details about a specific pattern
func (pv *PatternVerifier) GetPatternDetails(patternName string) (*ArchitecturalPattern, error) {
	pattern, exists := pv.architecturalPatterns[patternName]
	if !exists {
		return nil, fmt.Errorf("pattern not found: %s", patternName)
	}
	return &pattern, nil
}
