// Package analysis provides code analysis utilities including memory allocation analysis.
package analysis

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// TestFilePatterns defines patterns for detecting test files by language
type TestFilePatterns struct {
	// SuffixPatterns maps file extensions to suffix patterns (without extension)
	// e.g., ".go" -> ["_test"] matches "*_test.go"
	SuffixPatterns map[string][]string

	// PrefixPatterns maps file extensions to prefix patterns
	// e.g., ".py" -> ["test_"] matches "test_*.py"
	PrefixPatterns map[string][]string

	// DirectoryPatterns are directory path patterns that indicate test files
	// e.g., "/tests/", "/__tests__/"
	DirectoryPatterns []string

	// CustomMatcher is an optional function for custom test file detection
	// If provided, it's called after built-in patterns; returns true if file is a test
	CustomMatcher func(filePath string) bool
}

// MemoryAnalyzerOptions configures memory analysis behavior
type MemoryAnalyzerOptions struct {
	// IncludeTestFiles controls whether test files are included in the analysis.
	// When false (default), test files receive a 0.1x score penalty, effectively
	// filtering them from top hotspots while still tracking them.
	// When true, test files are scored normally (useful for analyzing test performance).
	IncludeTestFiles bool

	// TestFilePenalty is the multiplier applied to test file scores when
	// IncludeTestFiles is false. Default is 0.1 (10% of normal score).
	TestFilePenalty float64

	// TestFilePatterns configures how test files are detected.
	// If nil, uses DefaultTestFilePatterns().
	TestFilePatterns *TestFilePatterns
}

// DefaultTestFilePatterns returns the default test file detection patterns
func DefaultTestFilePatterns() *TestFilePatterns {
	return &TestFilePatterns{
		SuffixPatterns: map[string][]string{
			".go":   {"_test"},
			".js":   {".test", ".spec", "_test", "_spec"},
			".jsx":  {".test", ".spec", "_test", "_spec"},
			".ts":   {".test", ".spec", "_test", "_spec"},
			".tsx":  {".test", ".spec", "_test", "_spec"},
			".mjs":  {".test", ".spec", "_test", "_spec"},
			".mts":  {".test", ".spec", "_test", "_spec"},
			".py":   {"_test"},
			".java": {"test", "tests", "it"},
			".cs":   {"test", "tests"},
			".rs":   {"_test"},
			".rb":   {"_test", "_spec"},
			".php":  {"test"},
		},
		PrefixPatterns: map[string][]string{
			".py": {"test_"},
		},
		DirectoryPatterns: []string{
			"/tests/",
			"/test/",
			"/__tests__/",
			"/spec/",
			"/specs/",
			// Test fixtures and test data directories
			"/fixtures/",
			"/testdata/",
			"/testutil/",
			"/testing/",
			"/internal/testing/",
			"/mocks/",
			"/mock/",
			"/stubs/",
			"/fakes/",
			// Benchmark directories
			"/benchmarks/",
			"/bench/",
		},
	}
}

// DefaultMemoryAnalyzerOptions returns the default options for memory analysis
func DefaultMemoryAnalyzerOptions() MemoryAnalyzerOptions {
	return MemoryAnalyzerOptions{
		IncludeTestFiles: false,
		TestFilePenalty:  0.1,
		TestFilePatterns: nil, // Will use DefaultTestFilePatterns()
	}
}

// ExclusionChecker interface for checking if symbols should be excluded from analysis
// This allows for dependency injection and testing
type ExclusionChecker interface {
	IsExcludedFromAnalysis(fileID types.FileID, symbolID types.SymbolID, analysisType string) bool
}

// MemoryAnalyzer performs memory allocation analysis using the core GraphPropagator
// for PageRank-style score propagation through the call graph.
// It tracks allocations per branch/function and propagates "memory pressure" scores
// to identify hotspots.
type MemoryAnalyzer struct {
	// Allocation patterns per language
	allocationPatterns map[string][]AllocationPattern

	// Core propagation infrastructure (injected)
	propagator        *core.GraphPropagator
	refTracker        *core.ReferenceTracker
	symbolIndex       *core.SymbolIndex
	semanticAnnotator *core.SemanticAnnotator
	exclusionChecker  ExclusionChecker // For testing - if set, used instead of semanticAnnotator

	// Options for analysis behavior
	options MemoryAnalyzerOptions
}

// AllocationPattern defines a pattern that indicates memory allocation
type AllocationPattern struct {
	Pattern     *regexp.Regexp
	Category    string  // "heap", "stack", "pool", "gc-managed"
	Weight      float64 // Relative allocation cost (1.0 = baseline)
	Description string
}

// BranchAllocation tracks allocations within a code branch
type BranchAllocation struct {
	BranchType     string // "if", "else", "switch_case", "loop_body", "try", "catch"
	StartLine      int
	EndLine        int
	Depth          int     // Nesting depth
	Created        int     // Objects created in this branch
	Dropped        int     // Objects that go out of scope
	Retained       int     // Objects that escape the branch
	Weight         float64 // Weighted allocation score based on patterns
	LoopMultiplier float64 // If in loop, estimated iteration count multiplier
}

// FunctionMemoryProfile tracks memory characteristics of a function
type FunctionMemoryProfile struct {
	Name      string
	FilePath  string
	StartLine int
	EndLine   int
	Language  string
	FileID    types.FileID   // For annotation lookup
	SymbolID  types.SymbolID // For integration with GraphPropagator

	// Branch-level data
	Branches []BranchAllocation

	// Aggregated metrics
	TotalCreated   int
	TotalDropped   int
	TotalRetained  int     // Objects that escape function scope
	WeightedScore  float64 // Weighted allocation score
	LoopAllocScore float64 // Extra score for allocations in loops

	// Call relationships for propagation
	Callees []string // Functions this function calls
	Callers []string // Functions that call this function

	// Memory analysis hints from @lci annotations
	MemoryHints *core.MemoryAnalysisHints
}

// MemoryPressureScore represents the final propagated score for a function
type MemoryPressureScore struct {
	FunctionName     string
	FilePath         string
	Line             int
	SymbolID         types.SymbolID
	DirectScore      float64 // Score from direct allocations
	PropagatedScore  float64 // Score propagated from callees via GraphPropagator
	TotalScore       float64 // Combined score
	HeapPressure     float64 // Estimated heap allocation pressure
	LoopPressure     float64 // Pressure from loop allocations
	BranchComplexity float64 // Memory complexity from branching
	RetentionRisk    float64 // Risk of memory retention/leaks
	Percentile       float64 // Where this function ranks (0-100)
	Severity         string  // "critical", "high", "medium", "low"
	IsTestFile       bool    // Whether this function is in a test file
	IsExcluded       bool    // Whether this function is excluded via @lci:exclude[memory]

	// Call frequency context from @lci:call-frequency annotation
	// Values: "hot-path", "once-per-file", "once-per-request", "once-per-session",
	//         "startup-only", "cli-output", "test-only", "rare", or "" (unknown)
	CallFrequency string

	// Whether this function has bounded loops (reduces false positives for retry loops)
	HasBoundedLoops bool
}

// MemoryAnalysisResult contains the full analysis results
type MemoryAnalysisResult struct {
	Functions []FunctionMemoryProfile
	Scores    []MemoryPressureScore
	Summary   MemorySummary
	Hotspots  []MemoryHotspot
}

// MemorySummary provides aggregate statistics
type MemorySummary struct {
	TotalFunctions   int
	TotalAllocations int
	AvgAllocPerFunc  float64
	MaxAllocInFunc   int
	LoopAllocCount   int // Allocations inside loops
	BranchAllocCount int // Allocations in conditional branches
	CriticalCount    int
	HighCount        int
	MediumCount      int
	LowCount         int
	ExcludedCount    int // Functions excluded via @lci:exclude[memory]
}

// MemoryHotspot identifies a specific location with high memory pressure
type MemoryHotspot struct {
	FunctionName string
	FilePath     string
	Line         int
	Score        float64
	Reason       string
	Suggestion   string
}

// NewMemoryAnalyzer creates a new memory analyzer with default settings
func NewMemoryAnalyzer() *MemoryAnalyzer {
	return NewMemoryAnalyzerWithOptions(DefaultMemoryAnalyzerOptions())
}

// NewMemoryAnalyzerWithOptions creates a new memory analyzer with custom options
func NewMemoryAnalyzerWithOptions(options MemoryAnalyzerOptions) *MemoryAnalyzer {
	ma := &MemoryAnalyzer{
		allocationPatterns: make(map[string][]AllocationPattern),
		options:            options,
	}
	ma.initializeAllocationPatterns()
	return ma
}

// NewMemoryAnalyzerWithPropagator creates a memory analyzer that uses the core GraphPropagator
// for accurate call graph based score propagation
func NewMemoryAnalyzerWithPropagator(propagator *core.GraphPropagator, refTracker *core.ReferenceTracker, symbolIndex *core.SymbolIndex) *MemoryAnalyzer {
	return NewMemoryAnalyzerWithPropagatorAndOptions(propagator, refTracker, symbolIndex, DefaultMemoryAnalyzerOptions())
}

// NewMemoryAnalyzerWithPropagatorAndOptions creates a memory analyzer with propagator and custom options
func NewMemoryAnalyzerWithPropagatorAndOptions(propagator *core.GraphPropagator, refTracker *core.ReferenceTracker, symbolIndex *core.SymbolIndex, options MemoryAnalyzerOptions) *MemoryAnalyzer {
	ma := NewMemoryAnalyzerWithOptions(options)
	ma.propagator = propagator
	ma.refTracker = refTracker
	ma.symbolIndex = symbolIndex
	return ma
}

// NewMemoryAnalyzerFull creates a memory analyzer with all dependencies including semantic annotator
// for @lci:exclude annotation support
func NewMemoryAnalyzerFull(propagator *core.GraphPropagator, refTracker *core.ReferenceTracker, symbolIndex *core.SymbolIndex, semanticAnnotator *core.SemanticAnnotator, options MemoryAnalyzerOptions) *MemoryAnalyzer {
	ma := NewMemoryAnalyzerWithPropagatorAndOptions(propagator, refTracker, symbolIndex, options)
	ma.semanticAnnotator = semanticAnnotator
	return ma
}

// SetSemanticAnnotator sets the semantic annotator for @lci:exclude support
func (ma *MemoryAnalyzer) SetSemanticAnnotator(annotator *core.SemanticAnnotator) {
	ma.semanticAnnotator = annotator
}

// isExcludedFromMemoryAnalysis checks if a symbol should be excluded from memory analysis
// via @lci:exclude[memory] or @lci:exclude[all] annotations
func (ma *MemoryAnalyzer) isExcludedFromMemoryAnalysis(fileID types.FileID, symbolID types.SymbolID) bool {
	// Use exclusionChecker if available (for testing)
	if ma.exclusionChecker != nil {
		return ma.exclusionChecker.IsExcludedFromAnalysis(fileID, symbolID, "memory")
	}
	// Fall back to semanticAnnotator
	if ma.semanticAnnotator == nil {
		return false
	}
	return ma.semanticAnnotator.IsExcludedFromAnalysis(fileID, symbolID, "memory")
}

// SetExclusionChecker sets a custom exclusion checker (primarily for testing)
func (ma *MemoryAnalyzer) SetExclusionChecker(checker ExclusionChecker) {
	ma.exclusionChecker = checker
}

// SetOptions updates the analyzer options
func (ma *MemoryAnalyzer) SetOptions(options MemoryAnalyzerOptions) {
	ma.options = options
}

// GetOptions returns the current analyzer options
func (ma *MemoryAnalyzer) GetOptions() MemoryAnalyzerOptions {
	return ma.options
}

// getTestFilePatterns returns the configured test file patterns or defaults
func (ma *MemoryAnalyzer) getTestFilePatterns() *TestFilePatterns {
	if ma.options.TestFilePatterns != nil {
		return ma.options.TestFilePatterns
	}
	return DefaultTestFilePatterns()
}

// isTestFile detects whether a file path represents a test file using configured patterns.
func (ma *MemoryAnalyzer) isTestFile(filePath string) bool {
	patterns := ma.getTestFilePatterns()
	return isTestFileWithPatterns(filePath, patterns)
}

// isTestFileWithPatterns detects whether a file path represents a test file using provided patterns.
// This is the core detection logic used by both the method and standalone function.
func isTestFileWithPatterns(filePath string, patterns *TestFilePatterns) bool {
	// Normalize path separators
	normalizedPath := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))
	baseName := filepath.Base(normalizedPath)

	// Check for test directories
	if patterns != nil && len(patterns.DirectoryPatterns) > 0 {
		for _, testDir := range patterns.DirectoryPatterns {
			if strings.Contains("/"+normalizedPath, testDir) {
				return true
			}
		}
	}

	// Check for common test file patterns by extension
	ext := strings.ToLower(filepath.Ext(baseName))
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	if patterns != nil {
		// Check suffix patterns
		if suffixes, ok := patterns.SuffixPatterns[ext]; ok {
			for _, suffix := range suffixes {
				if strings.HasSuffix(nameWithoutExt, suffix) {
					return true
				}
			}
		}

		// Check prefix patterns
		if prefixes, ok := patterns.PrefixPatterns[ext]; ok {
			for _, prefix := range prefixes {
				if strings.HasPrefix(nameWithoutExt, prefix) {
					return true
				}
			}
		}

		// Check custom matcher
		if patterns.CustomMatcher != nil {
			if patterns.CustomMatcher(filePath) {
				return true
			}
		}
	}

	return false
}

// isTestFile is a standalone function that uses default patterns for test file detection.
// This is provided for backward compatibility and testing.
func isTestFile(filePath string) bool {
	return isTestFileWithPatterns(filePath, DefaultTestFilePatterns())
}

// initializeAllocationPatterns sets up language-specific allocation detection
func (ma *MemoryAnalyzer) initializeAllocationPatterns() {
	// Go allocation patterns
	ma.allocationPatterns["go"] = []AllocationPattern{
		{mustCompileRegex(`make\s*\(`), "heap", 1.0, "make() allocates on heap"},
		{mustCompileRegex(`new\s*\(`), "heap", 1.0, "new() allocates on heap"},
		{mustCompileRegex(`&\w+\{`), "heap", 0.8, "Composite literal address may escape to heap"},
		{mustCompileRegex(`append\s*\(`), "heap", 0.5, "append may reallocate"},
		{mustCompileRegex(`\[\]byte\s*\(`), "heap", 0.7, "byte slice conversion allocates"},
		{mustCompileRegex(`string\s*\(`), "heap", 0.7, "string conversion allocates"},
		{mustCompileRegex(`fmt\.Sprintf`), "heap", 0.8, "Sprintf allocates strings"},
		{mustCompileRegex(`json\.(Marshal|Unmarshal)`), "heap", 1.5, "JSON serialization allocates heavily"},
		{mustCompileRegex(`regexp\.(Compile|MustCompile)`), "heap", 2.0, "Regex compilation is expensive"},
	}

	// JavaScript/TypeScript allocation patterns
	ma.allocationPatterns["javascript"] = []AllocationPattern{
		{mustCompileRegex(`new\s+\w+`), "gc-managed", 1.0, "new creates object"},
		{mustCompileRegex(`\{\s*\}`), "gc-managed", 0.3, "Object literal"},
		{mustCompileRegex(`\[\s*\]`), "gc-managed", 0.3, "Array literal"},
		{mustCompileRegex(`Array\s*\(`), "gc-managed", 0.5, "Array constructor"},
		{mustCompileRegex(`Object\.(create|assign|keys|values|entries)`), "gc-managed", 0.5, "Object methods create new objects"},
		{mustCompileRegex(`\.map\s*\(`), "gc-managed", 0.8, "map creates new array"},
		{mustCompileRegex(`\.filter\s*\(`), "gc-managed", 0.6, "filter creates new array"},
		{mustCompileRegex(`\.slice\s*\(`), "gc-managed", 0.5, "slice creates copy"},
		{mustCompileRegex(`\.concat\s*\(`), "gc-managed", 0.7, "concat creates new array"},
		{mustCompileRegex(`JSON\.(parse|stringify)`), "gc-managed", 1.2, "JSON operations allocate"},
		{mustCompileRegex(`new\s+RegExp`), "gc-managed", 1.5, "Regex object creation"},
		{mustCompileRegex(`\.split\s*\(`), "gc-managed", 0.8, "split creates array of strings"},
	}
	ma.allocationPatterns["typescript"] = ma.allocationPatterns["javascript"]

	// Python allocation patterns
	ma.allocationPatterns["python"] = []AllocationPattern{
		{mustCompileRegex(`\[\s*\]`), "gc-managed", 0.3, "List literal"},
		{mustCompileRegex(`\{\s*\}`), "gc-managed", 0.3, "Dict literal"},
		{mustCompileRegex(`\(\s*\)`), "gc-managed", 0.2, "Tuple literal"},
		{mustCompileRegex(`list\s*\(`), "gc-managed", 0.5, "list() creates new list"},
		{mustCompileRegex(`dict\s*\(`), "gc-managed", 0.5, "dict() creates new dict"},
		{mustCompileRegex(`set\s*\(`), "gc-managed", 0.5, "set() creates new set"},
		{mustCompileRegex(`\w+\s*\(`), "gc-managed", 0.4, "Function call may return new object"},
		{mustCompileRegex(`copy\.(copy|deepcopy)`), "gc-managed", 1.5, "Copying objects"},
		{mustCompileRegex(`json\.(loads|dumps)`), "gc-managed", 1.2, "JSON operations"},
		{mustCompileRegex(`re\.(compile|match|search)`), "gc-managed", 1.0, "Regex operations"},
	}

	// Rust allocation patterns
	ma.allocationPatterns["rust"] = []AllocationPattern{
		{mustCompileRegex(`Box::new`), "heap", 1.0, "Box allocates on heap"},
		{mustCompileRegex(`Vec::new|vec!\[`), "heap", 0.8, "Vec allocates on heap"},
		{mustCompileRegex(`String::new|String::from|\.to_string\(\)`), "heap", 0.7, "String allocates"},
		{mustCompileRegex(`Rc::new`), "heap", 1.0, "Rc allocates with refcount"},
		{mustCompileRegex(`Arc::new`), "heap", 1.2, "Arc allocates with atomic refcount"},
		{mustCompileRegex(`HashMap::new|HashSet::new`), "heap", 1.0, "Hash collections allocate"},
		{mustCompileRegex(`\.clone\(\)`), "heap", 0.8, "clone() may allocate"},
		{mustCompileRegex(`\.to_vec\(\)`), "heap", 0.8, "to_vec() allocates"},
		{mustCompileRegex(`\.collect\(\)`), "heap", 0.6, "collect() allocates"},
	}

	// Java allocation patterns
	ma.allocationPatterns["java"] = []AllocationPattern{
		{mustCompileRegex(`new\s+\w+`), "heap", 1.0, "new allocates object"},
		{mustCompileRegex(`Arrays\.copyOf`), "heap", 0.8, "Array copy"},
		{mustCompileRegex(`\.clone\(\)`), "heap", 0.8, "clone() allocates"},
		{mustCompileRegex(`StringBuilder|StringBuffer`), "heap", 0.6, "String builder"},
		{mustCompileRegex(`new\s+ArrayList|new\s+HashMap|new\s+HashSet`), "heap", 1.0, "Collection creation"},
		{mustCompileRegex(`\.stream\(\).*\.collect\(`), "heap", 1.2, "Stream collection"},
		{mustCompileRegex(`String\.format`), "heap", 0.7, "String formatting"},
		{mustCompileRegex(`Pattern\.compile`), "heap", 1.5, "Regex compilation"},
	}

	// C# allocation patterns
	ma.allocationPatterns["csharp"] = []AllocationPattern{
		{mustCompileRegex(`new\s+\w+`), "heap", 1.0, "new allocates object"},
		{mustCompileRegex(`\.ToArray\(\)`), "heap", 0.8, "ToArray allocates"},
		{mustCompileRegex(`\.ToList\(\)`), "heap", 0.8, "ToList allocates"},
		{mustCompileRegex(`string\.Format`), "heap", 0.7, "String formatting"},
		{mustCompileRegex(`\$"`), "heap", 0.6, "String interpolation"},
		{mustCompileRegex(`new\s+List<|new\s+Dictionary<`), "heap", 1.0, "Collection creation"},
		{mustCompileRegex(`\.Select\(.*\)\.ToList\(`), "heap", 1.2, "LINQ with materialization"},
		{mustCompileRegex(`Regex\.Match|new\s+Regex`), "heap", 1.5, "Regex operations"},
	}
}

// AnalyzeFunctions analyzes a set of functions for memory allocation patterns
func (ma *MemoryAnalyzer) AnalyzeFunctions(functions []*FunctionAnalysis) *MemoryAnalysisResult {
	result := &MemoryAnalysisResult{
		Functions: make([]FunctionMemoryProfile, 0, len(functions)),
		Scores:    make([]MemoryPressureScore, 0, len(functions)),
	}

	// Phase 1: Build function profiles with direct allocation counts
	profiles := make(map[string]*FunctionMemoryProfile)
	symbolProfiles := make(map[types.SymbolID]*FunctionMemoryProfile)
	for _, fn := range functions {
		profile := ma.buildFunctionProfile(fn)
		result.Functions = append(result.Functions, *profile)
		profiles[fn.Name] = profile
		if profile.SymbolID != 0 {
			symbolProfiles[profile.SymbolID] = profile
		}
	}

	// Phase 2: Build call graph relationships
	ma.buildCallGraph(profiles, functions)

	// Phase 3: Run propagation
	var propagatedScores map[types.SymbolID]float64
	if ma.propagator != nil && ma.refTracker != nil {
		// Use core GraphPropagator for accurate call graph propagation
		propagatedScores = ma.runGraphPropagation(symbolProfiles)
	} else {
		// Fallback to standalone propagation for testing
		propagatedScores = ma.runStandalonePropagation(profiles)
	}

	// Phase 4: Generate final scores and rankings
	result.Scores = ma.generateScores(profiles, propagatedScores)

	// Phase 5: Identify hotspots
	result.Hotspots = ma.identifyHotspots(result.Scores)

	// Phase 6: Build summary
	result.Summary = ma.buildSummary(result)

	return result
}

// buildFunctionProfile creates a memory profile for a single function
func (ma *MemoryAnalyzer) buildFunctionProfile(fn *FunctionAnalysis) *FunctionMemoryProfile {
	profile := &FunctionMemoryProfile{
		Name:      fn.Name,
		FilePath:  fn.FilePath,
		StartLine: fn.StartLine,
		EndLine:   fn.EndLine,
		Language:  fn.Language,
		SymbolID:  fn.SymbolID,
		Branches:  make([]BranchAllocation, 0),
		Callees:   make([]string, 0),
	}

	// Fetch memory hints from semantic annotations if available
	if ma.semanticAnnotator != nil && fn.SymbolID != 0 {
		profile.MemoryHints = ma.semanticAnnotator.GetMemoryHints(profile.FileID, fn.SymbolID)
	}

	patterns := ma.allocationPatterns[fn.Language]
	if patterns == nil {
		patterns = ma.allocationPatterns["javascript"] // Default fallback
	}

	// Determine effective loop weight multiplier
	// Use annotation override if provided, otherwise use heuristics
	effectiveLoopWeight := ma.getEffectiveLoopWeight(profile.MemoryHints, fn.Loops)

	// Analyze loops as branches with multiplier
	for _, loop := range fn.Loops {
		branch := BranchAllocation{
			BranchType:     loop.NodeType,
			StartLine:      loop.StartLine,
			EndLine:        loop.EndLine,
			Depth:          loop.Depth,
			LoopMultiplier: ma.estimateLoopIterationsWithHints(loop.NodeType, profile.MemoryHints),
		}
		profile.Branches = append(profile.Branches, branch)
	}

	// Track calls for call graph
	for _, call := range fn.Calls {
		profile.Callees = append(profile.Callees, call.Target)

		// If call is in loop, count it as potential allocation
		if call.InLoop {
			profile.LoopAllocScore += 0.5 // Base score for call in loop
		}
	}

	// Estimate allocations from calls
	for _, call := range fn.Calls {
		weight := ma.estimateAllocationWeight(call.Target, patterns)
		if call.InLoop {
			// Use effective loop weight instead of fixed 10.0
			weight *= effectiveLoopWeight
			profile.LoopAllocScore += weight
		}
		profile.WeightedScore += weight
		profile.TotalCreated++
	}

	return profile
}

// getEffectiveLoopWeight determines the loop iteration multiplier to use
func (ma *MemoryAnalyzer) getEffectiveLoopWeight(hints *core.MemoryAnalysisHints, loops []LoopInfo) float64 {
	// If we have an explicit loop weight annotation, use it
	if hints != nil && hints.LoopWeight > 0 {
		return hints.LoopWeight
	}

	// If we have a loop-bounded annotation, use that value
	if hints != nil && hints.LoopBounded > 0 {
		return float64(hints.LoopBounded)
	}

	// Default heuristic based on loop types
	if len(loops) == 0 {
		return 10.0 // Default for general calls
	}

	// Use the maximum loop multiplier from all loops
	maxMultiplier := 1.0
	for _, loop := range loops {
		mult := estimateLoopIterations(loop.NodeType)
		if mult > maxMultiplier {
			maxMultiplier = mult
		}
	}
	return maxMultiplier
}

// estimateLoopIterationsWithHints estimates loop iterations considering annotations
func (ma *MemoryAnalyzer) estimateLoopIterationsWithHints(loopType string, hints *core.MemoryAnalysisHints) float64 {
	// If explicit loop-bounded annotation, use it
	if hints != nil && hints.LoopBounded > 0 {
		return float64(hints.LoopBounded)
	}

	// If explicit loop-weight annotation, use it
	if hints != nil && hints.LoopWeight > 0 {
		return hints.LoopWeight
	}

	// Fall back to heuristics
	return estimateLoopIterations(loopType)
}

// estimateLoopIterations provides a heuristic estimate of loop iterations
func estimateLoopIterations(loopType string) float64 {
	switch loopType {
	case "for_statement", "for_range_statement":
		return 10.0 // Conservative estimate
	case "while_statement", "do_while_statement":
		return 20.0 // Potentially unbounded
	case "for_in_statement", "for_of_statement", "for_each_statement":
		return 10.0 // Collection iteration
	default:
		return 10.0
	}
}

// estimateAllocationWeight checks if a call matches allocation patterns
func (ma *MemoryAnalyzer) estimateAllocationWeight(callTarget string, patterns []AllocationPattern) float64 {
	for _, p := range patterns {
		if p.Pattern.MatchString(callTarget) {
			return p.Weight
		}
	}
	return 0.1 // Small base weight for any call
}

// buildCallGraph establishes caller/callee relationships
func (ma *MemoryAnalyzer) buildCallGraph(profiles map[string]*FunctionMemoryProfile, functions []*FunctionAnalysis) {
	// Build reverse mapping (callers)
	for _, profile := range profiles {
		for _, calleeName := range profile.Callees {
			if calleeProfile, exists := profiles[calleeName]; exists {
				calleeProfile.Callers = append(calleeProfile.Callers, profile.Name)
			}
		}
	}
}

// runGraphPropagation uses the core GraphPropagator for accurate call graph propagation
func (ma *MemoryAnalyzer) runGraphPropagation(symbolProfiles map[types.SymbolID]*FunctionMemoryProfile) map[types.SymbolID]float64 {
	scores := make(map[types.SymbolID]float64)

	// Initialize direct scores
	for symbolID, profile := range symbolProfiles {
		scores[symbolID] = profile.WeightedScore + profile.LoopAllocScore
	}

	// Use GraphPropagator's getCallers/getCallees for accurate relationships
	// Run accumulation-style propagation (memory costs accumulate upward)
	defaultDampingFactor := 0.85
	maxIterations := 20
	convergenceThreshold := 0.001

	for iter := 0; iter < maxIterations; iter++ {
		prevScores := make(map[types.SymbolID]float64)
		for k, v := range scores {
			prevScores[k] = v
		}

		maxDiff := 0.0
		for symbolID, profile := range symbolProfiles {
			// Get actual callees from ReferenceTracker
			callees := ma.refTracker.GetCalleeSymbols(symbolID)

			// Use per-function propagation weight if specified, otherwise use default
			dampingFactor := defaultDampingFactor
			if profile.MemoryHints != nil && profile.MemoryHints.PropagationWeight > 0 {
				dampingFactor = profile.MemoryHints.PropagationWeight
			}

			// Reduce propagation weight for bounded loops (retry patterns, etc.)
			if profile.MemoryHints != nil && profile.MemoryHints.LoopBounded > 0 {
				// Bounded loops are less of a concern - reduce propagation
				boundedReduction := math.Min(1.0, float64(profile.MemoryHints.LoopBounded)/10.0)
				dampingFactor *= boundedReduction
			}

			// Accumulate scores from callees
			accumulatedScore := profile.WeightedScore + profile.LoopAllocScore
			for _, calleeID := range callees {
				if calleeScore, exists := prevScores[calleeID]; exists {
					accumulatedScore += dampingFactor * calleeScore
				}
			}

			diff := math.Abs(accumulatedScore - prevScores[symbolID])
			if diff > maxDiff {
				maxDiff = diff
			}
			scores[symbolID] = accumulatedScore
		}

		if maxDiff < convergenceThreshold {
			break
		}
	}

	return scores
}

// runStandalonePropagation performs propagation without GraphPropagator (for testing)
func (ma *MemoryAnalyzer) runStandalonePropagation(profiles map[string]*FunctionMemoryProfile) map[types.SymbolID]float64 {
	n := len(profiles)
	if n == 0 {
		return make(map[types.SymbolID]float64)
	}

	// Build name-to-score mapping for string-based call graph
	nameScores := make(map[string]float64)
	prevScores := make(map[string]float64)

	for name, profile := range profiles {
		nameScores[name] = profile.WeightedScore + profile.LoopAllocScore
		prevScores[name] = nameScores[name]
	}

	// Normalize and run propagation
	d := 0.85
	baseScore := (1.0 - d) / float64(n)

	for iter := 0; iter < 100; iter++ {
		for name, score := range nameScores {
			prevScores[name] = score
		}

		for name, profile := range profiles {
			incomingScore := 0.0

			// Sum contributions from callees (memory pressure flows from callees to callers)
			for _, calleeName := range profile.Callees {
				if calleeProfile, exists := profiles[calleeName]; exists {
					outDegree := len(calleeProfile.Callers)
					if outDegree > 0 {
						incomingScore += prevScores[calleeName] / float64(outDegree)
					}
				}
			}

			nameScores[name] = baseScore + d*incomingScore
		}

		// Check convergence
		maxDiff := 0.0
		for name := range nameScores {
			diff := math.Abs(nameScores[name] - prevScores[name])
			if diff > maxDiff {
				maxDiff = diff
			}
		}
		if maxDiff < 0.0001 {
			break
		}
	}

	// Convert to SymbolID-based scores
	scores := make(map[types.SymbolID]float64)
	for name, profile := range profiles {
		if profile.SymbolID != 0 {
			scores[profile.SymbolID] = nameScores[name] * 100 // Scale up
		} else {
			// Create a pseudo-ID from name hash for consistency
			pseudoID := types.SymbolID(hashString(name))
			scores[pseudoID] = nameScores[name] * 100
		}
	}

	return scores
}

// hashString creates a simple hash for string-based lookup
func hashString(s string) uint64 {
	h := uint64(0)
	for _, c := range s {
		h = h*31 + uint64(c)
	}
	return h
}

// generateScores creates final MemoryPressureScore objects
func (ma *MemoryAnalyzer) generateScores(profiles map[string]*FunctionMemoryProfile, propagatedScores map[types.SymbolID]float64) []MemoryPressureScore {
	scores := make([]MemoryPressureScore, 0, len(profiles))

	// Collect all scores for percentile calculation (use adjusted scores)
	// Only include non-excluded functions in percentile calculation
	allScores := make([]float64, 0, len(profiles))

	for name, profile := range profiles {
		directScore := profile.WeightedScore + profile.LoopAllocScore

		// Get propagated score
		var propagatedScore float64
		if profile.SymbolID != 0 {
			propagatedScore = propagatedScores[profile.SymbolID]
		} else {
			pseudoID := types.SymbolID(hashString(name))
			propagatedScore = propagatedScores[pseudoID]
		}

		totalScore := directScore + propagatedScore

		// Detect if this is a test file (using configured patterns)
		testFile := ma.isTestFile(profile.FilePath)

		// Check if excluded via @lci:exclude[memory] annotation
		excluded := ma.isExcludedFromMemoryAnalysis(profile.FileID, profile.SymbolID)

		// Apply test file penalty if not including test files at full weight
		adjustedTotalScore := totalScore
		if testFile && !ma.options.IncludeTestFiles {
			adjustedTotalScore = totalScore * ma.options.TestFilePenalty
		}

		// Excluded functions get zero score (won't appear in hotspots)
		if excluded {
			adjustedTotalScore = 0
		}

		// Extract call frequency and bounded loop info from hints
		callFrequency := ""
		hasBoundedLoops := false
		if profile.MemoryHints != nil {
			callFrequency = profile.MemoryHints.CallFrequency
			hasBoundedLoops = profile.MemoryHints.LoopBounded > 0
		}

		// Apply call frequency penalty to reduce score for low-frequency code paths
		frequencyMultiplier := getCallFrequencyMultiplier(callFrequency)
		adjustedTotalScore *= frequencyMultiplier

		score := MemoryPressureScore{
			FunctionName:     name,
			FilePath:         profile.FilePath,
			Line:             profile.StartLine,
			SymbolID:         profile.SymbolID,
			DirectScore:      directScore,
			PropagatedScore:  propagatedScore,
			TotalScore:       adjustedTotalScore,
			HeapPressure:     profile.WeightedScore,
			LoopPressure:     profile.LoopAllocScore,
			BranchComplexity: float64(len(profile.Branches)) * 0.5,
			RetentionRisk:    float64(profile.TotalRetained) * 2.0,
			IsTestFile:       testFile,
			IsExcluded:       excluded,
			CallFrequency:    callFrequency,
			HasBoundedLoops:  hasBoundedLoops,
		}

		// Only include non-excluded functions in percentile calculation
		if !excluded {
			allScores = append(allScores, adjustedTotalScore)
		}
		scores = append(scores, score)
	}

	// Calculate percentiles and severity
	for i := range scores {
		if scores[i].IsExcluded {
			scores[i].Percentile = 0
			scores[i].Severity = "excluded"
		} else {
			scores[i].Percentile = calculatePercentile(scores[i].TotalScore, allScores)
			scores[i].Severity = getSeverityFromPercentile(scores[i].Percentile)
		}
	}

	// Sort by total score descending
	sortScoresByTotal(scores)

	return scores
}

// calculatePercentile computes what percentile a score falls into
func calculatePercentile(score float64, allScores []float64) float64 {
	if len(allScores) == 0 {
		return 0
	}
	count := 0
	for _, s := range allScores {
		if score >= s {
			count++
		}
	}
	return float64(count) / float64(len(allScores)) * 100
}

// getSeverityFromPercentile maps percentile to severity
func getSeverityFromPercentile(percentile float64) string {
	if percentile >= 95 {
		return "critical"
	}
	if percentile >= 85 {
		return "high"
	}
	if percentile >= 70 {
		return "medium"
	}
	return "low"
}

// getCallFrequencyMultiplier returns a score multiplier based on call frequency
// Hot paths get full weight (1.0), while CLI output and rare code paths get reduced weight
func getCallFrequencyMultiplier(frequency string) float64 {
	switch frequency {
	case "hot-path":
		return 1.0 // Full weight for hot paths
	case "once-per-file":
		return 0.8 // Moderately important
	case "once-per-request":
		return 0.7 // Server code, per-request
	case "once-per-session":
		return 0.5 // Session-level code
	case "startup-only":
		return 0.3 // Only runs at startup
	case "cli-output":
		return 0.2 // CLI display code, runs once per command
	case "test-only":
		return 0.1 // Test code
	case "rare":
		return 0.1 // Rarely executed
	default:
		return 1.0 // Unknown frequency = assume it could be hot
	}
}

// sortScoresByTotal sorts scores by TotalScore descending
func sortScoresByTotal(scores []MemoryPressureScore) {
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].TotalScore > scores[i].TotalScore {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
}

// identifyHotspots finds the most significant memory pressure points
// Functions with @lci:exclude[memory] are skipped
func (ma *MemoryAnalyzer) identifyHotspots(scores []MemoryPressureScore) []MemoryHotspot {
	hotspots := make([]MemoryHotspot, 0)

	for _, score := range scores {
		// Skip excluded functions
		if score.IsExcluded {
			continue
		}

		if score.Severity == "critical" || score.Severity == "high" {
			hotspot := MemoryHotspot{
				FunctionName: score.FunctionName,
				FilePath:     score.FilePath,
				Line:         score.Line,
				Score:        score.TotalScore,
			}

			// Add call frequency context prefix if available
			contextPrefix := ""
			if score.CallFrequency != "" {
				contextPrefix = fmt.Sprintf("[%s] ", score.CallFrequency)
			}

			// Determine primary reason with enhanced context
			if score.HasBoundedLoops {
				// Bounded loops are less concerning
				hotspot.Reason = contextPrefix + "Allocations in bounded loop"
				hotspot.Suggestion = "Loop is bounded; verify iteration count matches @lci:loop-bounded annotation"
			} else if score.LoopPressure > score.HeapPressure {
				hotspot.Reason = contextPrefix + "High allocation rate inside loops"
				if score.CallFrequency == "cli-output" {
					hotspot.Suggestion = "CLI output code - consider adding @lci:call-frequency[cli-output] if not already present"
				} else {
					hotspot.Suggestion = "Consider pre-allocating or moving allocations outside loops"
				}
			} else if score.PropagatedScore > score.DirectScore {
				hotspot.Reason = contextPrefix + "Cascading memory pressure from callees"
				hotspot.Suggestion = "Review callee functions for optimization opportunities"
			} else {
				hotspot.Reason = contextPrefix + "High direct allocation rate"
				hotspot.Suggestion = "Consider object pooling or reducing allocations"
			}

			hotspots = append(hotspots, hotspot)
		}
	}

	return hotspots
}

// buildSummary creates aggregate statistics
func (ma *MemoryAnalyzer) buildSummary(result *MemoryAnalysisResult) MemorySummary {
	summary := MemorySummary{
		TotalFunctions: len(result.Functions),
	}

	maxAlloc := 0
	totalAlloc := 0

	for _, fn := range result.Functions {
		totalAlloc += fn.TotalCreated
		if fn.TotalCreated > maxAlloc {
			maxAlloc = fn.TotalCreated
		}

		for _, branch := range fn.Branches {
			if strings.Contains(branch.BranchType, "for") || strings.Contains(branch.BranchType, "while") {
				summary.LoopAllocCount += branch.Created
			} else {
				summary.BranchAllocCount += branch.Created
			}
		}
	}

	summary.TotalAllocations = totalAlloc
	summary.MaxAllocInFunc = maxAlloc
	if summary.TotalFunctions > 0 {
		summary.AvgAllocPerFunc = float64(totalAlloc) / float64(summary.TotalFunctions)
	}

	for _, score := range result.Scores {
		switch score.Severity {
		case "critical":
			summary.CriticalCount++
		case "high":
			summary.HighCount++
		case "medium":
			summary.MediumCount++
		case "low":
			summary.LowCount++
		case "excluded":
			summary.ExcludedCount++
		}
	}

	return summary
}

// AnalyzeFromPerfData creates FunctionAnalysis objects from PerfData and analyzes them
func (ma *MemoryAnalyzer) AnalyzeFromPerfData(files []*types.FileInfo) *MemoryAnalysisResult {
	var functions []*FunctionAnalysis

	for _, file := range files {
		if len(file.PerfData) == 0 {
			continue
		}

		for _, pd := range file.PerfData {
			fn := &FunctionAnalysis{
				Name:      pd.Name,
				FilePath:  file.Path,
				StartLine: pd.StartLine,
				EndLine:   pd.EndLine,
				IsAsync:   pd.IsAsync,
				Language:  pd.Language,
				Loops:     convertLoopsFromTypes(pd.Loops),
				Awaits:    convertAwaitsFromTypes(pd.Awaits),
				Calls:     convertCallsFromTypes(pd.Calls),
			}
			functions = append(functions, fn)
		}
	}

	return ma.AnalyzeFunctions(functions)
}

// AnalyzeFromPerfDataWithSymbols creates FunctionAnalysis with resolved SymbolIDs
// This enables accurate call graph propagation via ReferenceTracker
func (ma *MemoryAnalyzer) AnalyzeFromPerfDataWithSymbols(files []*types.FileInfo, refTracker *core.ReferenceTracker) *MemoryAnalysisResult {
	var functions []*FunctionAnalysis

	for _, file := range files {
		if len(file.PerfData) == 0 {
			continue
		}

		for _, pd := range file.PerfData {
			fn := &FunctionAnalysis{
				Name:      pd.Name,
				FilePath:  file.Path,
				StartLine: pd.StartLine,
				EndLine:   pd.EndLine,
				IsAsync:   pd.IsAsync,
				Language:  pd.Language,
				Loops:     convertLoopsFromTypes(pd.Loops),
				Awaits:    convertAwaitsFromTypes(pd.Awaits),
				Calls:     convertCallsFromTypes(pd.Calls),
			}

			// Resolve SymbolID from ReferenceTracker if available
			if refTracker != nil {
				if sym := refTracker.FindSymbolByFileAndName(file.ID, pd.Name); sym != nil {
					fn.SymbolID = sym.ID
				}
			}

			functions = append(functions, fn)
		}
	}

	return ma.AnalyzeFunctions(functions)
}

// Helper functions to convert types
func convertLoopsFromTypes(loops []types.LoopData) []LoopInfo {
	result := make([]LoopInfo, len(loops))
	for i, l := range loops {
		result[i] = LoopInfo{
			NodeType:  l.NodeType,
			StartLine: l.StartLine,
			EndLine:   l.EndLine,
			Depth:     l.Depth,
		}
	}
	return result
}

func convertAwaitsFromTypes(awaits []types.AwaitData) []AwaitInfo {
	result := make([]AwaitInfo, len(awaits))
	for i, a := range awaits {
		result[i] = AwaitInfo{
			Line:        a.Line,
			AssignedVar: a.AssignedVar,
			CallTarget:  a.CallTarget,
			UsedVars:    a.UsedVars,
		}
	}
	return result
}

func convertCallsFromTypes(calls []types.CallData) []CallInfo {
	result := make([]CallInfo, len(calls))
	for i, c := range calls {
		result[i] = CallInfo{
			Target:    c.Target,
			Line:      c.Line,
			InLoop:    c.InLoop,
			LoopDepth: c.LoopDepth,
			LoopLine:  c.LoopLine,
		}
	}
	return result
}

// GetMemoryAllocationLabelRule returns a LabelPropagationRule for memory allocation tracking
// This can be added to GraphPropagator's config for integrated analysis
func GetMemoryAllocationLabelRule() core.LabelPropagationRule {
	return core.LabelPropagationRule{
		Label:     "memory-allocation",
		Direction: "upstream",            // Memory pressure flows from callees to callers
		Mode:      core.ModeAccumulation, // Accumulate allocation costs
		MaxHops:   0,                     // Unlimited propagation
		Priority:  2,
	}
}
