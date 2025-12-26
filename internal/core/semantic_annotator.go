package core

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/standardbeagle/lci/internal/types"
)

// Global cached patterns for semantic annotations
var (
	globalSemanticPatterns     map[string]*regexp.Regexp
	globalSemanticPatternsOnce sync.Once
)

// SemanticAnnotator extracts and manages semantic annotations from code comments
// Supports structured comment-based metadata for enhanced code understanding
type SemanticAnnotator struct {
	// Annotation extraction patterns
	patterns map[string]*regexp.Regexp

	// Parsed annotations indexed by FileID and SymbolID
	annotations map[types.FileID]map[types.SymbolID]*SemanticAnnotation

	// Global annotation registry for fast lookups
	annotationIndex map[string][]*AnnotatedSymbol
}

// SemanticAnnotation contains structured metadata extracted from comments
type SemanticAnnotation struct {
	// Core metadata
	Labels   []string          `json:"labels"`   // @lci:labels[api,public,critical]
	Category string            `json:"category"` // @lci:category[endpoint]
	Tags     map[string]string `json:"tags"`     // @lci:tag[team=backend,owner=alice]

	// Exclusion directives - exclude from specific analyses
	// @lci:exclude[memory] - exclude from memory pressure analysis
	// @lci:exclude[memory,complexity] - exclude from multiple analyses
	// @lci:exclude[all] - exclude from all optional analyses
	Excludes []string `json:"excludes,omitempty"`

	// Dependency information
	Dependencies []Dependency `json:"dependencies"` // @lci:deps[db:read,service:auth]
	Provides     []string     `json:"provides"`     // @lci:provides[user-auth,session]

	// Metrics and attributes
	Metrics    map[string]interface{} `json:"metrics"`    // @lci:metrics[complexity=high,perf=slow]
	Attributes map[string]interface{} `json:"attributes"` // @lci:attr[timeout=30s,retries=3]

	// Memory analysis control annotations
	// @lci:loop-weight[0.1] - Override loop iteration multiplier (0.0-100.0, default varies by loop type)
	// @lci:loop-bounded[3] - Mark loop as bounded with max iterations (reduces false positives for retry loops)
	// @lci:call-frequency[once-per-request] - Hint about how often this code runs
	//   Values: hot-path, once-per-file, once-per-request, once-per-session, startup-only, cli-output
	// @lci:propagation-weight[0.5] - Override propagation damping factor for this function (0.0-1.0)
	MemoryHints *MemoryAnalysisHints `json:"memory_hints,omitempty"`

	// Propagation configuration
	PropagationRules []PropagationRule `json:"propagation_rules,omitempty"`

	// Source location
	SourceLocation AnnotationLocation `json:"source_location"`

	// Extraction metadata
	RawText     string   `json:"raw_text"`
	ParsedLines []string `json:"parsed_lines"`
	Confidence  float64  `json:"confidence"`
}

// MemoryAnalysisHints contains hints for memory pressure analysis
type MemoryAnalysisHints struct {
	// LoopWeight overrides the default loop iteration multiplier (0.0-100.0)
	// Use lower values for bounded/retry loops, higher for hot iteration loops
	LoopWeight float64 `json:"loop_weight,omitempty"`

	// LoopBounded marks the loop as having a known maximum iteration count
	// This significantly reduces the score for retry loops and similar patterns
	LoopBounded int `json:"loop_bounded,omitempty"`

	// CallFrequency indicates how often this code path is executed
	// Helps prioritize hot paths over CLI output or startup code
	CallFrequency string `json:"call_frequency,omitempty"`

	// PropagationWeight overrides the damping factor for score propagation (0.0-1.0)
	// Lower values reduce how much callee scores propagate to this function
	PropagationWeight float64 `json:"propagation_weight,omitempty"`

	// HasAnnotation tracks whether any memory hints were explicitly set
	HasAnnotation bool `json:"has_annotation,omitempty"`
}

// Dependency represents a dependency declaration
type Dependency struct {
	Type     string            `json:"type"`     // database, service, file, etc.
	Name     string            `json:"name"`     // specific resource name
	Mode     string            `json:"mode"`     // read, write, read-write
	Metadata map[string]string `json:"metadata"` // additional attributes
}

// PropagationRule defines how annotations should propagate through the graph
type PropagationRule struct {
	Attribute   string  `json:"attribute"`           // which attribute to propagate
	Direction   string  `json:"direction"`           // upstream, downstream, bidirectional
	Decay       float64 `json:"decay"`               // strength reduction per hop (0.0-1.0)
	MaxHops     int     `json:"max_hops"`            // maximum propagation distance
	Aggregation string  `json:"aggregation"`         // sum, max, unique, concat
	Condition   string  `json:"condition,omitempty"` // conditional propagation
}

// AnnotatedSymbol represents a symbol with its semantic annotations
type AnnotatedSymbol struct {
	FileID     types.FileID        `json:"file_id"`
	SymbolID   types.SymbolID      `json:"symbol_id"`
	Symbol     types.Symbol        `json:"symbol"`
	Annotation *SemanticAnnotation `json:"annotation"`
	FilePath   string              `json:"file_path"`
}

// AnnotationLocation specifies where an annotation was found
type AnnotationLocation struct {
	FileID    types.FileID   `json:"file_id"`
	SymbolID  types.SymbolID `json:"symbol_id"`
	StartLine int            `json:"start_line"`
	EndLine   int            `json:"end_line"`
	Context   string         `json:"context"`
}

// NewSemanticAnnotator creates a new semantic annotation system with shared cached patterns
func NewSemanticAnnotator() *SemanticAnnotator {
	// Initialize patterns once globally
	globalSemanticPatternsOnce.Do(func() {
		globalSemanticPatterns = map[string]*regexp.Regexp{
			"labels":      regexp.MustCompile(`@lci:labels?\[([^\]]+)\]`),
			"category":    regexp.MustCompile(`@lci:category\[([^\]]+)\]`),
			"deps":        regexp.MustCompile(`@lci:deps?\[([^\]]+)\]`),
			"provides":    regexp.MustCompile(`@lci:provides?\[([^\]]+)\]`),
			"tags":        regexp.MustCompile(`@lci:tags?\[([^\]]+)\]`),
			"metrics":     regexp.MustCompile(`@lci:metrics?\[([^\]]+)\]`),
			"attr":        regexp.MustCompile(`@lci:attr(?:ibutes?)?\[([^\]]+)\]`),
			"propagate":   regexp.MustCompile(`@lci:propagate\[([^\]]+)\]`),
			"exclude":     regexp.MustCompile(`@lci:exclude\[([^\]]+)\]`),
			"block_start": regexp.MustCompile(`@lci:begin`),
			"block_end":   regexp.MustCompile(`@lci:end`),
			// Memory analysis control annotations
			"loop_weight":        regexp.MustCompile(`@lci:loop-weight\[([0-9.]+)\]`),
			"loop_bounded":       regexp.MustCompile(`@lci:loop-bounded\[([0-9]+)\]`),
			"call_frequency":     regexp.MustCompile(`@lci:call-frequency\[([^\]]+)\]`),
			"propagation_weight": regexp.MustCompile(`@lci:propagation-weight\[([0-9.]+)\]`),
		}
	})

	return &SemanticAnnotator{
		patterns:        globalSemanticPatterns,
		annotations:     make(map[types.FileID]map[types.SymbolID]*SemanticAnnotation),
		annotationIndex: make(map[string][]*AnnotatedSymbol),
	}
}

// ExtractAnnotations processes file content to extract semantic annotations
func (sa *SemanticAnnotator) ExtractAnnotations(fileID types.FileID, filePath string, content string, symbols []types.Symbol) error {
	if sa.annotations[fileID] == nil {
		sa.annotations[fileID] = make(map[types.SymbolID]*SemanticAnnotation)
	}

	lines := strings.Split(content, "\n")

	// Process each symbol and find associated annotations
	for _, symbol := range symbols {
		annotation := sa.extractSymbolAnnotation(fileID, symbol, lines)
		if annotation != nil {
			// Create a unique symbol ID based on file and symbol position
			symbolID := types.SymbolID(fileID)<<32 | types.SymbolID(symbol.Line)<<16 | types.SymbolID(symbol.Column)
			sa.annotations[fileID][symbolID] = annotation

			// Index by labels for fast lookup
			for _, label := range annotation.Labels {
				annotatedSymbol := &AnnotatedSymbol{
					FileID:     fileID,
					SymbolID:   symbolID,
					Symbol:     symbol,
					Annotation: annotation,
					FilePath:   filePath,
				}
				sa.annotationIndex[label] = append(sa.annotationIndex[label], annotatedSymbol)
			}
		}
	}

	return nil
}

// extractSymbolAnnotation finds and parses annotations for a specific symbol
func (sa *SemanticAnnotator) extractSymbolAnnotation(fileID types.FileID, symbol types.Symbol, lines []string) *SemanticAnnotation {
	// Look for annotations in comments before the symbol
	startSearchLine := max(0, symbol.Line-10) // Look up to 10 lines before
	endSearchLine := symbol.Line - 1

	var annotationLines []string
	var rawText strings.Builder

	// Collect potential annotation lines
	for i := startSearchLine; i <= endSearchLine && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if sa.isAnnotationLine(line) {
			annotationLines = append(annotationLines, line)
			rawText.WriteString(line + "\n")
		}
	}

	if len(annotationLines) == 0 {
		return nil
	}

	// Parse the collected annotation lines
	annotation := &SemanticAnnotation{
		Labels:           make([]string, 0),
		Excludes:         make([]string, 0),
		Dependencies:     make([]Dependency, 0),
		Provides:         make([]string, 0),
		Tags:             make(map[string]string),
		Metrics:          make(map[string]interface{}),
		Attributes:       make(map[string]interface{}),
		PropagationRules: make([]PropagationRule, 0),
		SourceLocation: AnnotationLocation{
			FileID:    fileID,
			SymbolID:  types.SymbolID(fileID)<<32 | types.SymbolID(symbol.Line)<<16 | types.SymbolID(symbol.Column),
			StartLine: startSearchLine,
			EndLine:   endSearchLine,
			Context:   symbol.Name,
		},
		RawText:     rawText.String(),
		ParsedLines: annotationLines,
		Confidence:  1.0,
	}

	// Parse each annotation line
	for _, line := range annotationLines {
		sa.parseAnnotationLine(line, annotation)
	}

	// Only return annotation if we found actual content
	if len(annotation.Labels) > 0 || annotation.Category != "" || len(annotation.Dependencies) > 0 ||
		len(annotation.Tags) > 0 || len(annotation.Metrics) > 0 || len(annotation.Attributes) > 0 ||
		len(annotation.Excludes) > 0 || annotation.MemoryHints != nil {
		return annotation
	}

	return nil
}

// isAnnotationLine checks if a line contains LCI annotations
func (sa *SemanticAnnotator) isAnnotationLine(line string) bool {
	// Only accept unified @lci: prefix (legacy/alternative formats not supported)
	return strings.Contains(line, "@lci:")
}

// parseAnnotationLine extracts structured data from a single annotation line
func (sa *SemanticAnnotator) parseAnnotationLine(line string, annotation *SemanticAnnotation) {
	// Parse labels
	if matches := sa.patterns["labels"].FindStringSubmatch(line); len(matches) > 1 {
		labels := sa.parseCommaSeparated(matches[1])
		annotation.Labels = append(annotation.Labels, labels...)
	}

	// Parse category
	if matches := sa.patterns["category"].FindStringSubmatch(line); len(matches) > 1 {
		annotation.Category = strings.TrimSpace(matches[1])
	}

	// Parse dependencies
	if matches := sa.patterns["deps"].FindStringSubmatch(line); len(matches) > 1 {
		deps := sa.parseDependencies(matches[1])
		annotation.Dependencies = append(annotation.Dependencies, deps...)
	}

	// Parse provides
	if matches := sa.patterns["provides"].FindStringSubmatch(line); len(matches) > 1 {
		provides := sa.parseCommaSeparated(matches[1])
		annotation.Provides = append(annotation.Provides, provides...)
	}

	// Parse tags (key=value pairs)
	if matches := sa.patterns["tags"].FindStringSubmatch(line); len(matches) > 1 {
		tags := sa.parseKeyValuePairs(matches[1])
		for k, v := range tags {
			annotation.Tags[k] = v
		}
	}

	// Parse metrics
	if matches := sa.patterns["metrics"].FindStringSubmatch(line); len(matches) > 1 {
		metrics := sa.parseKeyValuePairs(matches[1])
		for k, v := range metrics {
			annotation.Metrics[k] = sa.parseValue(v)
		}
	}

	// Parse attributes
	if matches := sa.patterns["attr"].FindStringSubmatch(line); len(matches) > 1 {
		attrs := sa.parseKeyValuePairs(matches[1])
		for k, v := range attrs {
			annotation.Attributes[k] = sa.parseValue(v)
		}
	}

	// Parse propagation rules
	if matches := sa.patterns["propagate"].FindStringSubmatch(line); len(matches) > 1 {
		rule := sa.parsePropagationRule(matches[1])
		if rule != nil {
			annotation.PropagationRules = append(annotation.PropagationRules, *rule)
		}
	}

	// Parse exclusion directives
	// @lci:exclude[memory] - exclude from memory pressure analysis
	// @lci:exclude[memory,complexity] - exclude from multiple analyses
	// @lci:exclude[all] - exclude from all optional analyses
	if matches := sa.patterns["exclude"].FindStringSubmatch(line); len(matches) > 1 {
		excludes := sa.parseCommaSeparated(matches[1])
		annotation.Excludes = append(annotation.Excludes, excludes...)
	}

	// Parse memory analysis hints
	sa.parseMemoryHints(line, annotation)
}

// parseMemoryHints extracts memory analysis control annotations
func (sa *SemanticAnnotator) parseMemoryHints(line string, annotation *SemanticAnnotation) {
	// Initialize MemoryHints if any memory-related annotation is found
	ensureMemoryHints := func() {
		if annotation.MemoryHints == nil {
			annotation.MemoryHints = &MemoryAnalysisHints{}
		}
		annotation.MemoryHints.HasAnnotation = true
	}

	// @lci:loop-weight[0.1] - Override loop iteration multiplier
	if matches := sa.patterns["loop_weight"].FindStringSubmatch(line); len(matches) > 1 {
		if weight, err := strconv.ParseFloat(matches[1], 64); err == nil {
			ensureMemoryHints()
			annotation.MemoryHints.LoopWeight = weight
		}
	}

	// @lci:loop-bounded[3] - Mark loop as bounded with max iterations
	if matches := sa.patterns["loop_bounded"].FindStringSubmatch(line); len(matches) > 1 {
		if bounded, err := strconv.Atoi(matches[1]); err == nil {
			ensureMemoryHints()
			annotation.MemoryHints.LoopBounded = bounded
		}
	}

	// @lci:call-frequency[hot-path] - Hint about execution frequency
	if matches := sa.patterns["call_frequency"].FindStringSubmatch(line); len(matches) > 1 {
		freq := strings.TrimSpace(matches[1])
		if isValidCallFrequency(freq) {
			ensureMemoryHints()
			annotation.MemoryHints.CallFrequency = freq
		}
	}

	// @lci:propagation-weight[0.5] - Override propagation damping factor
	if matches := sa.patterns["propagation_weight"].FindStringSubmatch(line); len(matches) > 1 {
		if weight, err := strconv.ParseFloat(matches[1], 64); err == nil {
			// Clamp to valid range [0.0, 1.0]
			if weight < 0 {
				weight = 0
			} else if weight > 1 {
				weight = 1
			}
			ensureMemoryHints()
			annotation.MemoryHints.PropagationWeight = weight
		}
	}
}

// isValidCallFrequency checks if the call frequency value is recognized
func isValidCallFrequency(freq string) bool {
	validFrequencies := map[string]bool{
		"hot-path":         true, // Called very frequently (inner loops, critical paths)
		"once-per-file":    true, // Called once per file being processed
		"once-per-request": true, // Called once per API/HTTP request
		"once-per-session": true, // Called once per user session
		"startup-only":     true, // Called only during initialization
		"cli-output":       true, // CLI display code, runs once per command
		"test-only":        true, // Only runs during testing
		"rare":             true, // Rarely executed (error paths, fallbacks)
	}
	return validFrequencies[freq]
}

// Helper parsing functions

func (sa *SemanticAnnotator) parseCommaSeparated(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (sa *SemanticAnnotator) parseKeyValuePairs(input string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(input, ",")

	for _, pair := range pairs {
		if parts := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}

	return result
}

func (sa *SemanticAnnotator) parseDependencies(input string) []Dependency {
	var deps []Dependency
	parts := sa.parseCommaSeparated(input)

	for _, part := range parts {
		dep := Dependency{
			Metadata: make(map[string]string),
		}

		// Parse format: type:name:mode or type:name
		segments := strings.Split(part, ":")
		if len(segments) >= 2 {
			dep.Type = strings.TrimSpace(segments[0])
			dep.Name = strings.TrimSpace(segments[1])
			if len(segments) >= 3 {
				dep.Mode = strings.TrimSpace(segments[2])
			} else {
				dep.Mode = "read-write" // default
			}
		} else {
			// Simple format: assume it's a service dependency
			dep.Type = "service"
			dep.Name = strings.TrimSpace(part)
			dep.Mode = "read-write"
		}

		deps = append(deps, dep)
	}

	return deps
}

func (sa *SemanticAnnotator) parsePropagationRule(input string) *PropagationRule {
	rule := &PropagationRule{
		Decay:       0.8,      // default decay
		MaxHops:     3,        // default max hops
		Aggregation: "unique", // default aggregation
	}

	// Parse comma-separated key=value pairs
	pairs := sa.parseKeyValuePairs(input)

	for key, value := range pairs {
		switch key {
		case "attribute", "attr":
			rule.Attribute = value
		case "direction", "dir":
			rule.Direction = value
		case "decay":
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				rule.Decay = f
			}
		case "max_hops", "hops":
			if i, err := strconv.Atoi(value); err == nil {
				rule.MaxHops = i
			}
		case "aggregation", "agg":
			rule.Aggregation = value
		case "condition", "cond":
			rule.Condition = value
		}
	}

	// Validate required fields
	if rule.Attribute != "" && rule.Direction != "" {
		return rule
	}

	return nil
}

func (sa *SemanticAnnotator) parseValue(input string) interface{} {
	// Try to parse as number
	if i, err := strconv.Atoi(input); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(input, 64); err == nil {
		return f
	}

	// Try to parse as boolean
	if b, err := strconv.ParseBool(input); err == nil {
		return b
	}

	// Try to parse as JSON
	var jsonValue interface{}
	if err := json.Unmarshal([]byte(input), &jsonValue); err == nil {
		return jsonValue
	}

	// Default to string
	return input
}

// Query functions

// GetAnnotation retrieves annotation for a specific symbol
func (sa *SemanticAnnotator) GetAnnotation(fileID types.FileID, symbolID types.SymbolID) *SemanticAnnotation {
	if fileAnnotations, exists := sa.annotations[fileID]; exists {
		return fileAnnotations[symbolID]
	}
	return nil
}

// GetMemoryHints retrieves memory analysis hints for a specific symbol
// Returns nil if no memory hints are defined for the symbol
func (sa *SemanticAnnotator) GetMemoryHints(fileID types.FileID, symbolID types.SymbolID) *MemoryAnalysisHints {
	annotation := sa.GetAnnotation(fileID, symbolID)
	if annotation == nil {
		return nil
	}
	return annotation.MemoryHints
}

// GetSymbolsByLabel finds all symbols with a specific label
func (sa *SemanticAnnotator) GetSymbolsByLabel(label string) []*AnnotatedSymbol {
	return sa.annotationIndex[label]
}

// GetSymbolsByCategory finds all symbols in a specific category
func (sa *SemanticAnnotator) GetSymbolsByCategory(category string) []*AnnotatedSymbol {
	var result []*AnnotatedSymbol

	for fileID, fileAnnotations := range sa.annotations {
		for symbolID, annotation := range fileAnnotations {
			if annotation.Category == category {
				// Find the symbol info (this would need symbol lookup)
				annotatedSymbol := &AnnotatedSymbol{
					FileID:     fileID,
					SymbolID:   symbolID,
					Annotation: annotation,
				}
				result = append(result, annotatedSymbol)
			}
		}
	}

	return result
}

// GetDependencyGraph builds a dependency graph from annotations
func (sa *SemanticAnnotator) GetDependencyGraph() map[types.SymbolID][]Dependency {
	graph := make(map[types.SymbolID][]Dependency)

	for _, fileAnnotations := range sa.annotations {
		for symbolID, annotation := range fileAnnotations {
			if len(annotation.Dependencies) > 0 {
				graph[symbolID] = annotation.Dependencies
			}
		}
	}

	return graph
}

// IsExcludedFromAnalysis checks if a symbol should be excluded from a specific analysis type.
// Supported analysis types: "memory", "complexity", "duplicates", "all"
// Returns true if the symbol has @lci:exclude[analysisType] or @lci:exclude[all]
func (sa *SemanticAnnotator) IsExcludedFromAnalysis(fileID types.FileID, symbolID types.SymbolID, analysisType string) bool {
	annotation := sa.GetAnnotation(fileID, symbolID)
	if annotation == nil {
		return false
	}
	return IsExcludedFromAnalysisByAnnotation(annotation, analysisType)
}

// IsExcludedFromAnalysisByAnnotation checks if an annotation excludes a specific analysis type.
// This is a standalone function for use when the annotation is already available.
func IsExcludedFromAnalysisByAnnotation(annotation *SemanticAnnotation, analysisType string) bool {
	if annotation == nil || len(annotation.Excludes) == 0 {
		return false
	}

	analysisType = strings.ToLower(analysisType)
	for _, exclude := range annotation.Excludes {
		exclude = strings.ToLower(strings.TrimSpace(exclude))
		if exclude == "all" || exclude == analysisType {
			return true
		}
	}
	return false
}

// GetExcludedSymbols returns all symbols excluded from a specific analysis type
func (sa *SemanticAnnotator) GetExcludedSymbols(analysisType string) []*AnnotatedSymbol {
	var excluded []*AnnotatedSymbol

	for fileID, fileAnnotations := range sa.annotations {
		for symbolID, annotation := range fileAnnotations {
			if IsExcludedFromAnalysisByAnnotation(annotation, analysisType) {
				excluded = append(excluded, &AnnotatedSymbol{
					FileID:     fileID,
					SymbolID:   symbolID,
					Annotation: annotation,
				})
			}
		}
	}

	return excluded
}

// Statistics and reporting

// GetAnnotationStats provides statistics about extracted annotations
func (sa *SemanticAnnotator) GetAnnotationStats() map[string]interface{} {
	stats := make(map[string]interface{})

	totalAnnotations := 0
	labelCount := make(map[string]int)
	categoryCount := make(map[string]int)
	dependencyTypes := make(map[string]int)

	for _, fileAnnotations := range sa.annotations {
		for _, annotation := range fileAnnotations {
			totalAnnotations++

			// Count labels
			for _, label := range annotation.Labels {
				labelCount[label]++
			}

			// Count categories
			if annotation.Category != "" {
				categoryCount[annotation.Category]++
			}

			// Count dependency types
			for _, dep := range annotation.Dependencies {
				dependencyTypes[dep.Type]++
			}
		}
	}

	stats["total_annotations"] = totalAnnotations
	stats["unique_labels"] = len(labelCount)
	stats["unique_categories"] = len(categoryCount)
	stats["dependency_types"] = len(dependencyTypes)
	stats["label_distribution"] = labelCount
	stats["category_distribution"] = categoryCount
	stats["dependency_type_distribution"] = dependencyTypes

	return stats
}

// Utility functions

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
