package search

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// RequirementsAnalyzer analyzes search queries and options to determine index requirements
type RequirementsAnalyzer struct {
	config      *AnalyzerConfig
	coordinator core.IndexCoordinator

	// Dependency analysis cache
	dependencyCache map[string]*DependencyAnalysis
	cacheMutex      sync.RWMutex
	cacheExpiry     time.Duration

	// Index availability tracking
	availabilityTracker map[core.IndexType]*IndexAvailabilityStatus
	availabilityMutex   sync.RWMutex

	// Pre-compiled regexes (lock-free, component-local)
	symbolRegex *regexp.Regexp
}

// AnalyzerConfig configures the requirements analysis behavior
type AnalyzerConfig struct {
	EnablePatternAnalysis  bool
	EnableSemanticAnalysis bool
	EnableContextAnalysis  bool
	MaxPatternComplexity   int
	DefaultIndexes         []core.IndexType
}

// AnalysisResult contains the analysis result with required index types and confidence
type AnalysisResult struct {
	RequiredIndexes   []core.IndexType
	OptionalIndexes   []core.IndexType
	Confidence        float64
	EstimatedCost     int64
	Reasoning         []string
	OptimizationHints []string
}

// NewRequirementsAnalyzer creates a new requirements analyzer
func NewRequirementsAnalyzer() *RequirementsAnalyzer {
	config := DefaultAnalyzerConfig()
	return &RequirementsAnalyzer{
		config:              config,
		dependencyCache:     make(map[string]*DependencyAnalysis),
		availabilityTracker: make(map[core.IndexType]*IndexAvailabilityStatus),
		cacheExpiry:         5 * time.Minute,
		symbolRegex:         regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`),
	}
}

// NewRequirementsAnalyzerWithConfig creates an analyzer with custom configuration
func NewRequirementsAnalyzerWithConfig(config *AnalyzerConfig) *RequirementsAnalyzer {
	return &RequirementsAnalyzer{
		config:              config,
		dependencyCache:     make(map[string]*DependencyAnalysis),
		availabilityTracker: make(map[core.IndexType]*IndexAvailabilityStatus),
		cacheExpiry:         5 * time.Minute,
		symbolRegex:         regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`),
	}
}

// DefaultAnalyzerConfig returns default analyzer configuration
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		EnablePatternAnalysis:  true,
		EnableSemanticAnalysis: true,
		EnableContextAnalysis:  true,
		MaxPatternComplexity:   10,
		DefaultIndexes: []core.IndexType{
			core.TrigramIndexType,
			core.SymbolIndexType,
		},
	}
}

// AnalyzeRequirements analyzes search requirements based on pattern and options
func (ra *RequirementsAnalyzer) AnalyzeRequirements(pattern string, options types.SearchOptions) *AnalysisResult {
	core.LogCoordinationInfo("Analyzing requirements for pattern: "+pattern, core.ErrorContext{
		OperationType: "requirements_analysis_start",
	})

	result := &AnalysisResult{
		RequiredIndexes:   ra.config.DefaultIndexes,
		OptionalIndexes:   []core.IndexType{},
		Confidence:        0.5,
		EstimatedCost:     100,
		Reasoning:         []string{},
		OptimizationHints: []string{},
	}

	// Always include trigram index for pattern matching
	ra.addRequiredIndex(result, core.TrigramIndexType, "Pattern matching requires trigram index")
	result.RequiredIndexes = ra.removeDuplicates(result.RequiredIndexes)

	// Analyze pattern complexity and type
	if ra.config.EnablePatternAnalysis {
		ra.analyzePattern(result, pattern)
	}

	// Analyze search options for index requirements
	ra.analyzeSearchOptions(result, options)

	// Analyze semantic requirements
	if ra.config.EnableSemanticAnalysis {
		ra.analyzeSemanticRequirements(result, pattern, options)
	}

	// Analyze context requirements
	if ra.config.EnableContextAnalysis {
		ra.analyzeContextRequirements(result, options)
	}

	// Calculate confidence and cost
	ra.calculateMetrics(result, pattern, options)

	// Generate optimization hints
	ra.generateOptimizationHints(result, pattern, options)

	// Log analysis completion
	core.LogCoordinationInfo(fmt.Sprintf("Requirements analysis completed - required: %v, optional: %v, confidence: %.2f, cost: %d",
		result.RequiredIndexes, result.OptionalIndexes, result.Confidence, result.EstimatedCost), core.ErrorContext{
		OperationType: "requirements_analysis_complete",
	})

	return result
}

// analyzePattern analyzes the search pattern for complexity and type
func (ra *RequirementsAnalyzer) analyzePattern(result *AnalysisResult, pattern string) {
	// Check for regex patterns
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) > 2 {
		result.Reasoning = append(result.Reasoning, "Detected regex pattern")
		result.EstimatedCost += 200
		ra.addOptionalIndex(result, core.SymbolIndexType, "Regex patterns benefit from symbol analysis")
		return
	}

	// Check for complex patterns
	complexity := ra.calculatePatternComplexity(pattern)
	if complexity > ra.config.MaxPatternComplexity {
		result.Reasoning = append(result.Reasoning, "Complex pattern detected")
		result.EstimatedCost += 150
		ra.addOptionalIndex(result, core.SymbolIndexType, "Complex patterns benefit from symbol analysis")
	}

	// Check for symbol-like patterns
	if ra.isSymbolPattern(pattern) {
		result.Reasoning = append(result.Reasoning, "Symbol-like pattern detected")
		ra.addRequiredIndex(result, core.SymbolIndexType, "Symbol search requires symbol index")
		result.Confidence += 0.2
	}

	// Check for file path patterns
	if ra.isFilePathPattern(pattern) {
		result.Reasoning = append(result.Reasoning, "File path pattern detected")
		ra.addRequiredIndex(result, core.LocationIndexType, "File path search requires location index")
		result.EstimatedCost += 50
	}

	// Check for content patterns
	if ra.isContentPattern(pattern) {
		result.Reasoning = append(result.Reasoning, "Content pattern detected")
		ra.addOptionalIndex(result, core.ContentIndexType, "Content patterns benefit from full-text search")
		result.EstimatedCost += 100
	}
}

// analyzeSearchOptions analyzes search options for index requirements
func (ra *RequirementsAnalyzer) analyzeSearchOptions(result *AnalysisResult, options types.SearchOptions) {
	// Declaration-only search
	if options.DeclarationOnly {
		result.Reasoning = append(result.Reasoning, "Declaration-only search")
		ra.addRequiredIndex(result, core.SymbolIndexType, "Declaration search requires symbol index")
		ra.addOptionalIndex(result, core.ReferenceIndexType, "Declarations may benefit from reference analysis")
		result.Confidence += 0.1
	}

	// Usage-only search
	if options.UsageOnly {
		result.Reasoning = append(result.Reasoning, "Usage-only search")
		ra.addRequiredIndex(result, core.ReferenceIndexType, "Usage search requires reference index")
		ra.addOptionalIndex(result, core.CallGraphIndexType, "Usage analysis benefits from call graph")
		result.Confidence += 0.1
	}

	// Call graph support (future enhancement)
	// Note: Would require SearchOptions to include call graph traversal depth and filters
	// When implemented, this would add CallGraphIndexType requirement

	// Context lines requirement
	if options.MaxContextLines > 0 {
		result.Reasoning = append(result.Reasoning, "Context lines requested")
		ra.addRequiredIndex(result, core.PostingsIndexType, "Context requires postings index")
		ra.addRequiredIndex(result, core.ContentIndexType, "Context requires content index")
		result.EstimatedCost += int64(options.MaxContextLines * 20)
	}

	// File filtering
	if options.IncludePattern != "" || options.ExcludePattern != "" {
		result.Reasoning = append(result.Reasoning, "File filtering enabled")
		ra.addRequiredIndex(result, core.LocationIndexType, "File filtering requires location index")
		result.EstimatedCost += 30
	}
}

// analyzeSemanticRequirements analyzes semantic search requirements
func (ra *RequirementsAnalyzer) analyzeSemanticRequirements(result *AnalysisResult, pattern string, options types.SearchOptions) {
	// Check for semantic annotations
	if ra.hasSemanticAnnotations(pattern) {
		result.Reasoning = append(result.Reasoning, "Semantic annotations detected")
		ra.addRequiredIndex(result, core.SymbolIndexType, "Semantic search requires symbol index")
		ra.addOptionalIndex(result, core.CallGraphIndexType, "Semantic analysis benefits from call graph")
		result.Confidence += 0.15
	}

	// Check for relationship patterns
	if ra.hasRelationshipPatterns(pattern) {
		result.Reasoning = append(result.Reasoning, "Relationship patterns detected")
		ra.addRequiredIndex(result, core.ReferenceIndexType, "Relationship analysis requires reference index")
		ra.addOptionalIndex(result, core.CallGraphIndexType, "Complex relationships benefit from call graph")
		result.Confidence += 0.1
	}

	// Check for architectural patterns
	if ra.hasArchitecturalPatterns(pattern) {
		result.Reasoning = append(result.Reasoning, "Architectural patterns detected")
		ra.addRequiredIndex(result, core.SymbolIndexType, "Architectural analysis requires symbol index")
		ra.addRequiredIndex(result, core.CallGraphIndexType, "Architectural analysis requires call graph")
		result.Confidence += 0.2
	}
}

// analyzeContextRequirements analyzes context-related requirements
func (ra *RequirementsAnalyzer) analyzeContextRequirements(result *AnalysisResult, options types.SearchOptions) {
	// Analyze context requirements
	if options.MaxContextLines > 5 {
		result.Reasoning = append(result.Reasoning, "Extensive context requested")
		ra.addRequiredIndex(result, core.ContentIndexType, "Extensive context requires content index")
		result.EstimatedCost += 100
	}

	// Analyze file-based context
	if options.IncludePattern != "" {
		result.Reasoning = append(result.Reasoning, "File-based context requested")
		ra.addOptionalIndex(result, core.LocationIndexType, "File context benefits from location index")
	}

	// Relationship context analysis (future enhancement)
	// Note: Would require SearchOptions to include relationship traversal options
	// When implemented, would add ReferenceIndexType and SymbolIndexType requirements
}

// calculatePatternComplexity calculates the complexity of a search pattern
func (ra *RequirementsAnalyzer) calculatePatternComplexity(pattern string) int {
	complexity := 0

	// Base complexity from length
	complexity += len(pattern) / 5

	// Regex complexity
	if strings.Contains(pattern, "(") || strings.Contains(pattern, "[") || strings.Contains(pattern, "*") {
		complexity += 3
	}

	// Operator complexity
	operators := []string{"|", "&", "^", "$", ".", "+"}
	for _, op := range operators {
		complexity += strings.Count(pattern, op)
	}

	// Word complexity
	words := strings.Fields(pattern)
	complexity += len(words)

	return complexity
}

// isSymbolPattern checks if pattern looks like a symbol search
func (ra *RequirementsAnalyzer) isSymbolPattern(pattern string) bool {
	// Remove regex delimiters
	cleanPattern := strings.Trim(pattern, "/")

	// Use pre-compiled regex (lock-free, component-local)
	return ra.symbolRegex.MatchString(cleanPattern)
}

// isFilePathPattern checks if pattern looks like a file path
func (ra *RequirementsAnalyzer) isFilePathPattern(pattern string) bool {
	return strings.Contains(pattern, "/") ||
		strings.Contains(pattern, "\\") ||
		strings.HasSuffix(pattern, ".go") ||
		strings.HasSuffix(pattern, ".js") ||
		strings.HasSuffix(pattern, ".py") ||
		strings.HasSuffix(pattern, ".rs") ||
		strings.HasSuffix(pattern, ".cpp") ||
		strings.HasSuffix(pattern, ".java")
}

// isContentPattern checks if pattern looks like content search
func (ra *RequirementsAnalyzer) isContentPattern(pattern string) bool {
	contentIndicators := []string{" ", "\"", "'", "{", "}", "(", ")", ";", ","}
	for _, indicator := range contentIndicators {
		if strings.Contains(pattern, indicator) {
			return true
		}
	}
	return false
}

// hasSemanticAnnotations checks if pattern contains semantic annotation references
func (ra *RequirementsAnalyzer) hasSemanticAnnotations(pattern string) bool {
	semanticKeywords := []string{"@lci:", "labels", "category", "depends", "critical", "bug", "security"}
	for _, keyword := range semanticKeywords {
		if strings.Contains(pattern, keyword) {
			return true
		}
	}
	return false
}

// hasRelationshipPatterns checks if pattern contains relationship indicators
func (ra *RequirementsAnalyzer) hasRelationshipPatterns(pattern string) bool {
	relationshipKeywords := []string{"calls", "uses", "depends", "implements", "extends", "overrides"}
	for _, keyword := range relationshipKeywords {
		if strings.Contains(pattern, keyword) {
			return true
		}
	}
	return false
}

// hasArchitecturalPatterns checks if pattern contains architectural indicators
func (ra *RequirementsAnalyzer) hasArchitecturalPatterns(pattern string) bool {
	architecturalKeywords := []string{"controller", "service", "repository", "model", "view", "handler", "middleware"}
	for _, keyword := range architecturalKeywords {
		if strings.Contains(pattern, keyword) {
			return true
		}
	}
	return false
}

// calculateMetrics calculates confidence and cost metrics
func (ra *RequirementsAnalyzer) calculateMetrics(result *AnalysisResult, pattern string, options types.SearchOptions) {
	// Base confidence
	confidence := 0.3

	// Pattern clarity bonus
	if len(pattern) > 3 {
		confidence += 0.1
	}

	// Specific options bonus
	if options.DeclarationOnly || options.UsageOnly {
		confidence += 0.2
	}

	// Required indexes confidence boost
	requiredCount := len(result.RequiredIndexes)
	if requiredCount > 2 {
		confidence += 0.1
	}

	// Cap confidence at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	result.Confidence = confidence

	// Calculate estimated cost based on indexes and complexity
	baseCost := int64(100)
	for _, indexType := range result.RequiredIndexes {
		baseCost += ra.getIndexCost(indexType)
	}
	for _, indexType := range result.OptionalIndexes {
		baseCost += ra.getIndexCost(indexType) / 2 // Optional indexes cost less
	}

	result.EstimatedCost = baseCost
}

// getIndexCost returns the estimated cost for using a specific index type
func (ra *RequirementsAnalyzer) getIndexCost(indexType core.IndexType) int64 {
	costs := map[core.IndexType]int64{
		core.TrigramIndexType:   50,  // Fast, low cost
		core.SymbolIndexType:    100, // Medium cost
		core.ReferenceIndexType: 120, // Medium-high cost
		core.CallGraphIndexType: 200, // High cost
		core.PostingsIndexType:  80,  // Low-medium cost
		core.LocationIndexType:  60,  // Low cost
		core.ContentIndexType:   150, // Medium-high cost
	}

	if cost, exists := costs[indexType]; exists {
		return cost
	}
	return 100 // Default cost
}

// generateOptimizationHints generates optimization hints for the search
func (ra *RequirementsAnalyzer) generateOptimizationHints(result *AnalysisResult, pattern string, options types.SearchOptions) {
	// Pattern optimization hints
	if ra.calculatePatternComplexity(pattern) > ra.config.MaxPatternComplexity {
		result.OptimizationHints = append(result.OptimizationHints, "Consider simplifying the search pattern for better performance")
	}

	// Index optimization hints
	if len(result.RequiredIndexes) > 4 {
		result.OptimizationHints = append(result.OptimizationHints, "Consider using more specific search options to reduce required indexes")
	}

	// Context optimization hints
	if options.MaxContextLines > 10 {
		result.OptimizationHints = append(result.OptimizationHints, "Large context requests may impact performance")
	}

	// Future feature: Add depth optimization hints when SearchOptions supports depth parameters

	// Alternative approach hints
	if result.Confidence < 0.5 {
		result.OptimizationHints = append(result.OptimizationHints, "Consider using more specific search terms for better results")
	}
}

// addRequiredIndex adds a required index type if not already present
func (ra *RequirementsAnalyzer) addRequiredIndex(result *AnalysisResult, indexType core.IndexType, reason string) {
	if !ra.containsIndex(result.RequiredIndexes, indexType) {
		result.RequiredIndexes = append(result.RequiredIndexes, indexType)
		result.Reasoning = append(result.Reasoning, reason)
	}
}

// addOptionalIndex adds an optional index type if not already present
func (ra *RequirementsAnalyzer) addOptionalIndex(result *AnalysisResult, indexType core.IndexType, reason string) {
	if !ra.containsIndex(result.OptionalIndexes, indexType) && !ra.containsIndex(result.RequiredIndexes, indexType) {
		result.OptionalIndexes = append(result.OptionalIndexes, indexType)
		result.Reasoning = append(result.Reasoning, reason)
	}
}

// containsIndex checks if an index type is present in the slice
func (ra *RequirementsAnalyzer) containsIndex(indexes []core.IndexType, indexType core.IndexType) bool {
	for _, idx := range indexes {
		if idx == indexType {
			return true
		}
	}
	return false
}

// removeDuplicates removes duplicate index types while preserving order
func (ra *RequirementsAnalyzer) removeDuplicates(indexes []core.IndexType) []core.IndexType {
	if len(indexes) == 0 {
		return []core.IndexType{} // Return empty slice instead of nil
	}

	seen := make(map[core.IndexType]bool)
	var result []core.IndexType

	for _, indexType := range indexes {
		if !seen[indexType] {
			seen[indexType] = true
			result = append(result, indexType)
		}
	}

	return result
}

// GetRequiredIndexes returns the required index types from analysis result
func (result *AnalysisResult) GetRequiredIndexes() []core.IndexType {
	return result.RequiredIndexes
}

// GetAllIndexes returns both required and optional index types
func (ra *RequirementsAnalyzer) GetAllIndexes(result *AnalysisResult) []core.IndexType {
	allIndexes := append([]core.IndexType{}, result.RequiredIndexes...)
	allIndexes = append(allIndexes, result.OptionalIndexes...)
	return ra.removeDuplicates(allIndexes)
}

// ShouldUseIndex checks if an index type should be used for the search
func (ra *RequirementsAnalyzer) ShouldUseIndex(result *AnalysisResult, indexType core.IndexType) bool {
	return ra.containsIndex(result.RequiredIndexes, indexType) || ra.containsIndex(result.OptionalIndexes, indexType)
}

// GetEstimatedSearchTime returns estimated search time in milliseconds
func (ra *RequirementsAnalyzer) GetEstimatedSearchTime(result *AnalysisResult) int64 {
	// Base time plus cost-based estimation
	baseTime := int64(5)                                 // 5ms base
	searchTime := baseTime + (result.EstimatedCost / 20) // Cost to time conversion
	return searchTime
}

// T046: Search Dependency Analysis and Resolution

// IndexAvailabilityStatus tracks the availability status of an index
type IndexAvailabilityStatus struct {
	IndexType      core.IndexType
	IsAvailable    bool
	IsIndexing     bool
	LastChecked    time.Time
	Error          error
	EstimatedReady time.Time
	QueuedPosition int
}

// DependencyAnalysis represents the analysis of dependencies for search requirements
type DependencyAnalysis struct {
	RequiredIndexes  []core.IndexType
	DependencyGraph  map[core.IndexType][]core.IndexType
	ValidationErrors []error
	AnalysisTime     time.Time
	CacheExpiry      time.Time
}

// DependencyResolution represents how dependencies are resolved based on availability
type DependencyResolution struct {
	Strategy             ResolutionStrategy
	ResolvedRequirements core.SearchRequirements
	AvailableIndexes     []core.IndexType
	UnavailableIndexes   []core.IndexType
	IsDegraded           bool
	IsBlocked            bool
	FallbackReason       string
	OptimizedOrder       []core.IndexType
	ResolutionTime       time.Duration
}

// ResolutionStrategy represents different approaches to dependency resolution
type ResolutionStrategy string

const (
	StrategyDirect   ResolutionStrategy = "direct"   // All required indexes available
	StrategyFallback ResolutionStrategy = "fallback" // Use alternative indexes
	StrategyPartial  ResolutionStrategy = "partial"  // Use available subset
	StrategyBlocked  ResolutionStrategy = "blocked"  // Critical dependencies unavailable
)

// TransitiveResolution handles complex dependency chains
type TransitiveResolution struct {
	DirectDependencies      []core.IndexType
	TransitiveDependencies  []core.IndexType
	UnavailableDependencies []core.IndexType
	HasOptimization         bool
	OptimizedOrder          []core.IndexType
	CircularDependencies    []core.IndexType
	ResolutionScore         float64
}

// DependencyCacheMetrics tracks cache performance
type DependencyCacheMetrics struct {
	HitCount    int64
	MissCount   int64
	HitRate     float64
	TotalSize   int
	LastCleanup time.Time
}

// AnalyzeSearchDependencies performs comprehensive dependency analysis for search requirements
func (ra *RequirementsAnalyzer) AnalyzeSearchDependencies(requirements core.SearchRequirements) *DependencyAnalysis {
	// Generate cache key
	cacheKey := ra.generateCacheKey(requirements)

	// Check cache first
	ra.cacheMutex.RLock()
	if cached, exists := ra.dependencyCache[cacheKey]; exists && time.Now().Before(cached.CacheExpiry) {
		ra.cacheMutex.RUnlock()
		return cached
	}
	ra.cacheMutex.RUnlock()

	// Perform new analysis
	analysis := &DependencyAnalysis{
		RequiredIndexes:  requirements.GetRequiredIndexTypes(),
		DependencyGraph:  ra.buildDependencyGraph(requirements),
		ValidationErrors: []error{},
		AnalysisTime:     time.Now(),
		CacheExpiry:      time.Now().Add(ra.cacheExpiry),
	}

	// Validate dependencies
	if err := ra.validateDependencies(analysis); err != nil {
		analysis.ValidationErrors = append(analysis.ValidationErrors, err)
	}

	// Cache the result
	ra.cacheMutex.Lock()
	ra.dependencyCache[cacheKey] = analysis
	ra.cacheMutex.Unlock()

	// Cleanup expired cache entries periodically
	go ra.cleanupExpiredCache()

	return analysis
}

// Validate validates the dependency analysis
func (da *DependencyAnalysis) Validate() error {
	if len(da.ValidationErrors) > 0 {
		return fmt.Errorf("dependency validation failed: %v", da.ValidationErrors)
	}
	return nil
}

// GetDependencyGraph returns the dependency graph
func (da *DependencyAnalysis) GetDependencyGraph() map[core.IndexType][]core.IndexType {
	return da.DependencyGraph
}

// ResolveSearchDependencies resolves dependencies based on current index availability
func (ra *RequirementsAnalyzer) ResolveSearchDependencies(requirements core.SearchRequirements) *DependencyResolution {
	startTime := time.Now()

	// Get current index availability
	availability := ra.getCurrentIndexAvailability()

	// Analyze required dependencies
	analysis := ra.AnalyzeSearchDependencies(requirements)

	// Determine resolution strategy
	strategy := ra.determineResolutionStrategy(analysis, availability)

	// Create resolution
	resolution := &DependencyResolution{
		Strategy:       strategy,
		ResolutionTime: time.Since(startTime),
	}

	switch strategy {
	case StrategyDirect:
		resolution.ResolvedRequirements = requirements
		resolution.AvailableIndexes = analysis.RequiredIndexes
		resolution.IsDegraded = false
		resolution.IsBlocked = false

	case StrategyFallback:
		resolution.ResolvedRequirements = ra.createFallbackRequirements(analysis, availability)
		resolution.AvailableIndexes = ra.getAvailableIndexes(analysis.RequiredIndexes, availability)
		resolution.UnavailableIndexes = ra.getUnavailableIndexes(analysis.RequiredIndexes, availability)
		resolution.IsDegraded = true
		resolution.FallbackReason = "Some required indexes unavailable, using alternatives"

	case StrategyPartial:
		resolution.ResolvedRequirements = ra.createPartialRequirements(analysis, availability)
		resolution.AvailableIndexes = ra.getAvailableIndexes(analysis.RequiredIndexes, availability)
		resolution.UnavailableIndexes = ra.getUnavailableIndexes(analysis.RequiredIndexes, availability)
		resolution.IsDegraded = true
		resolution.FallbackReason = "Using available subset of required indexes"

	case StrategyBlocked:
		resolution.ResolvedRequirements = core.NewSearchRequirements() // Empty requirements
		resolution.AvailableIndexes = []core.IndexType{}
		resolution.UnavailableIndexes = analysis.RequiredIndexes
		resolution.IsDegraded = false
		resolution.IsBlocked = true
		resolution.FallbackReason = "Critical dependencies unavailable"
	}

	// Optimize access order
	resolution.OptimizedOrder = ra.optimizeAccessOrder(resolution.AvailableIndexes, analysis.DependencyGraph)

	return resolution
}

// ResolveTransitiveDependencies resolves complex dependency chains
func (ra *RequirementsAnalyzer) ResolveTransitiveDependencies(requirements core.SearchRequirements) *TransitiveResolution {
	analysis := ra.AnalyzeSearchDependencies(requirements)
	availability := ra.getCurrentIndexAvailability()

	resolution := &TransitiveResolution{
		DirectDependencies:      analysis.RequiredIndexes,
		TransitiveDependencies:  []core.IndexType{},
		UnavailableDependencies: []core.IndexType{},
		HasOptimization:         true,
		OptimizedOrder:          []core.IndexType{},
		CircularDependencies:    []core.IndexType{},
		ResolutionScore:         1.0,
	}

	// Find all transitive dependencies
	visited := make(map[core.IndexType]bool)
	var visit func(indexType core.IndexType)
	visit = func(indexType core.IndexType) {
		if visited[indexType] {
			return
		}
		visited[indexType] = true

		deps := analysis.DependencyGraph[indexType]
		for _, dep := range deps {
			if !ra.containsIndex(resolution.DirectDependencies, dep) {
				resolution.TransitiveDependencies = append(resolution.TransitiveDependencies, dep)
			}
			visit(dep)
		}
	}

	for _, indexType := range analysis.RequiredIndexes {
		visit(indexType)
	}

	// Check for unavailable dependencies
	allDeps := append(resolution.DirectDependencies, resolution.TransitiveDependencies...)
	for _, dep := range allDeps {
		if status, exists := availability[dep]; !exists || !status.IsAvailable {
			resolution.UnavailableDependencies = append(resolution.UnavailableDependencies, dep)
		}
	}

	// Detect circular dependencies
	resolution.CircularDependencies = ra.detectCircularDependencies(analysis.DependencyGraph)

	// Create optimized access order
	resolution.OptimizedOrder = ra.createOptimizedAccessOrder(resolution.DirectDependencies, resolution.TransitiveDependencies, analysis.DependencyGraph)

	// Calculate resolution score
	availableCount := len(allDeps) - len(resolution.UnavailableDependencies)
	resolution.ResolutionScore = float64(availableCount) / float64(len(allDeps))

	return resolution
}

// GetResolutionHistory returns resolution history for a set of requirements (mock implementation)
func (ra *RequirementsAnalyzer) GetResolutionHistory(requirements core.SearchRequirements) []*DependencyResolution {
	// Mock implementation - in real system would track history
	return []*DependencyResolution{}
}

// SetIndexAvailability updates the availability status of indexes
func (ra *RequirementsAnalyzer) SetIndexAvailability(availability map[core.IndexType]bool) {
	ra.availabilityMutex.Lock()
	defer ra.availabilityMutex.Unlock()

	now := time.Now()
	for indexType, isAvailable := range availability {
		status := &IndexAvailabilityStatus{
			IndexType:   indexType,
			IsAvailable: isAvailable,
			LastChecked: now,
		}

		if ra.coordinator != nil {
			indexStatus := ra.coordinator.GetIndexStatus(indexType)
			status.IsIndexing = indexStatus.IsIndexing
			if status.IsIndexing {
				status.EstimatedReady = now.Add(ra.estimateIndexingTime(indexType))
			}
		}

		ra.availabilityTracker[indexType] = status
	}
}

// SimulateIndexingStateChange simulates an index state change (for testing)
func (ra *RequirementsAnalyzer) SimulateIndexingStateChange(indexType core.IndexType, isIndexing bool) {
	ra.availabilityMutex.Lock()
	defer ra.availabilityMutex.Unlock()

	if status, exists := ra.availabilityTracker[indexType]; exists {
		status.IsIndexing = isIndexing
		status.LastChecked = time.Now()
		if isIndexing {
			status.IsAvailable = false
			status.EstimatedReady = time.Now().Add(ra.estimateIndexingTime(indexType))
		} else {
			status.IsAvailable = true
			status.EstimatedReady = time.Time{}
		}
	}
}

// Private helper methods

func (ra *RequirementsAnalyzer) generateCacheKey(requirements core.SearchRequirements) string {
	// Create a simple cache key based on required indexes
	indexes := requirements.GetRequiredIndexTypes()
	key := fmt.Sprintf("req_%v", indexes)
	return key
}

func (ra *RequirementsAnalyzer) buildDependencyGraph(requirements core.SearchRequirements) map[core.IndexType][]core.IndexType {
	graph := make(map[core.IndexType][]core.IndexType)
	requiredIndexes := requirements.GetRequiredIndexTypes()

	// Define known dependencies
	dependencies := map[core.IndexType][]core.IndexType{
		core.CallGraphIndexType: {core.SymbolIndexType},
		core.ReferenceIndexType: {core.SymbolIndexType},
		core.PostingsIndexType:  {core.TrigramIndexType},
	}

	for _, indexType := range requiredIndexes {
		if deps, exists := dependencies[indexType]; exists {
			graph[indexType] = deps
		} else {
			graph[indexType] = []core.IndexType{}
		}
	}

	return graph
}

func (ra *RequirementsAnalyzer) validateDependencies(analysis *DependencyAnalysis) error {
	// Check for circular dependencies
	if circular := ra.detectCircularDependencies(analysis.DependencyGraph); len(circular) > 0 {
		return fmt.Errorf("circular dependencies detected: %v", circular)
	}

	// Check for invalid dependency combinations
	for indexType, deps := range analysis.DependencyGraph {
		for _, dep := range deps {
			if !ra.containsIndex(analysis.RequiredIndexes, dep) {
				return fmt.Errorf("index %s depends on %s which is not in required indexes", indexType.String(), dep.String())
			}
		}
	}

	return nil
}

func (ra *RequirementsAnalyzer) detectCircularDependencies(graph map[core.IndexType][]core.IndexType) []core.IndexType {
	visited := make(map[core.IndexType]bool)
	recursionStack := make(map[core.IndexType]bool)
	circular := []core.IndexType{}

	var visit func(indexType core.IndexType) bool
	visit = func(indexType core.IndexType) bool {
		if recursionStack[indexType] {
			circular = append(circular, indexType)
			return true
		}
		if visited[indexType] {
			return false
		}

		visited[indexType] = true
		recursionStack[indexType] = true

		for _, dep := range graph[indexType] {
			if visit(dep) {
				return true
			}
		}

		recursionStack[indexType] = false
		return false
	}

	for indexType := range graph {
		if visit(indexType) {
			break
		}
	}

	return circular
}

func (ra *RequirementsAnalyzer) getCurrentIndexAvailability() map[core.IndexType]*IndexAvailabilityStatus {
	ra.availabilityMutex.RLock()
	defer ra.availabilityMutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[core.IndexType]*IndexAvailabilityStatus)
	for indexType, status := range ra.availabilityTracker {
		statusCopy := *status
		result[indexType] = &statusCopy
	}
	return result
}

func (ra *RequirementsAnalyzer) determineResolutionStrategy(analysis *DependencyAnalysis, availability map[core.IndexType]*IndexAvailabilityStatus) ResolutionStrategy {
	requiredCount := len(analysis.RequiredIndexes)
	availableCount := 0

	for _, indexType := range analysis.RequiredIndexes {
		if status, exists := availability[indexType]; exists && status.IsAvailable && !status.IsIndexing {
			availableCount++
		}
	}

	// All required indexes available
	if availableCount == requiredCount {
		return StrategyDirect
	}

	// No critical indexes available
	criticalIndexes := []core.IndexType{core.TrigramIndexType, core.SymbolIndexType}
	availableCritical := 0
	for _, critical := range criticalIndexes {
		if ra.containsIndex(analysis.RequiredIndexes, critical) {
			if status, exists := availability[critical]; exists && status.IsAvailable && !status.IsIndexing {
				availableCritical++
			}
		}
	}

	if availableCritical == 0 && len(criticalIndexes) > 0 {
		return StrategyBlocked
	}

	// Some indexes available
	if availableCount > 0 {
		if availableCount >= requiredCount/2 {
			return StrategyPartial
		} else {
			return StrategyFallback
		}
	}

	return StrategyBlocked
}

func (ra *RequirementsAnalyzer) createFallbackRequirements(analysis *DependencyAnalysis, availability map[core.IndexType]*IndexAvailabilityStatus) core.SearchRequirements {
	// Create requirements with available indexes only
	available := ra.getAvailableIndexes(analysis.RequiredIndexes, availability)

	// Create a simple requirements object (implementation depends on SearchRequirements interface)
	// This is a mock implementation
	requirements := core.NewSearchRequirements()
	for _, indexType := range available {
		// This would need to be implemented based on the actual SearchRequirements interface
		_ = indexType // Avoid unused variable warning
	}

	return requirements
}

func (ra *RequirementsAnalyzer) createPartialRequirements(analysis *DependencyAnalysis, availability map[core.IndexType]*IndexAvailabilityStatus) core.SearchRequirements {
	// Create requirements with available indexes that form a coherent subset
	available := ra.getAvailableIndexes(analysis.RequiredIndexes, availability)

	// Prefer to include trigram and symbol indexes if available
	prioritized := []core.IndexType{core.TrigramIndexType, core.SymbolIndexType}
	finalIndexes := []core.IndexType{}

	for _, priority := range prioritized {
		if ra.containsIndex(available, priority) {
			finalIndexes = append(finalIndexes, priority)
		}
	}

	// Add other available indexes
	for _, indexType := range available {
		if !ra.containsIndex(finalIndexes, indexType) {
			finalIndexes = append(finalIndexes, indexType)
		}
	}

	// Create requirements object (mock implementation)
	requirements := core.NewSearchRequirements()
	for _, indexType := range finalIndexes {
		// This would need to be implemented based on the actual SearchRequirements interface
		_ = indexType // Avoid unused variable warning
	}

	return requirements
}

func (ra *RequirementsAnalyzer) getAvailableIndexes(indexes []core.IndexType, availability map[core.IndexType]*IndexAvailabilityStatus) []core.IndexType {
	available := []core.IndexType{}
	for _, indexType := range indexes {
		if status, exists := availability[indexType]; exists && status.IsAvailable && !status.IsIndexing {
			available = append(available, indexType)
		}
	}
	return available
}

func (ra *RequirementsAnalyzer) getUnavailableIndexes(indexes []core.IndexType, availability map[core.IndexType]*IndexAvailabilityStatus) []core.IndexType {
	unavailable := []core.IndexType{}
	for _, indexType := range indexes {
		if status, exists := availability[indexType]; !exists || !status.IsAvailable || status.IsIndexing {
			unavailable = append(unavailable, indexType)
		}
	}
	return unavailable
}

func (ra *RequirementsAnalyzer) optimizeAccessOrder(indexes []core.IndexType, dependencies map[core.IndexType][]core.IndexType) []core.IndexType {
	// Simple topological sort to optimize access order
	ordered := []core.IndexType{}
	visited := make(map[core.IndexType]bool)

	var visit func(indexType core.IndexType)
	visit = func(indexType core.IndexType) {
		if visited[indexType] {
			return
		}
		visited[indexType] = true

		// Visit dependencies first
		for _, dep := range dependencies[indexType] {
			if ra.containsIndex(indexes, dep) {
				visit(dep)
			}
		}

		ordered = append(ordered, indexType)
	}

	for _, indexType := range indexes {
		visit(indexType)
	}

	return ordered
}

func (ra *RequirementsAnalyzer) createOptimizedAccessOrder(direct, transitive []core.IndexType, dependencies map[core.IndexType][]core.IndexType) []core.IndexType {
	allIndexes := append(direct, transitive...)
	return ra.optimizeAccessOrder(allIndexes, dependencies)
}

func (ra *RequirementsAnalyzer) estimateIndexingTime(indexType core.IndexType) time.Duration {
	// Estimate indexing time based on index type
	switch indexType {
	case core.TrigramIndexType:
		return 1 * time.Second
	case core.SymbolIndexType:
		return 3 * time.Second
	case core.CallGraphIndexType:
		return 5 * time.Second
	case core.ReferenceIndexType:
		return 4 * time.Second
	case core.PostingsIndexType:
		return 2 * time.Second
	case core.LocationIndexType:
		return 1 * time.Second
	case core.ContentIndexType:
		return 6 * time.Second
	default:
		return 3 * time.Second
	}
}

func (ra *RequirementsAnalyzer) cleanupExpiredCache() {
	ra.cacheMutex.Lock()
	defer ra.cacheMutex.Unlock()

	now := time.Now()
	for key, analysis := range ra.dependencyCache {
		if now.After(analysis.CacheExpiry) {
			delete(ra.dependencyCache, key)
		}
	}
}

// GetDependencyCacheMetrics returns cache performance metrics
func (ra *RequirementsAnalyzer) GetDependencyCacheMetrics() *DependencyCacheMetrics {
	ra.cacheMutex.RLock()
	defer ra.cacheMutex.RUnlock()

	cacheSize := len(ra.dependencyCache)

	// Calculate hit rate
	totalRequests := int64(0)
	hitCount := int64(0)

	// For now, we don't track individual cache stats, just cache size
	// In a real implementation, we'd track hits/misses per entry
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(hitCount) / float64(totalRequests)
	}

	return &DependencyCacheMetrics{
		HitCount:    hitCount,
		MissCount:   totalRequests - hitCount,
		HitRate:     hitRate,
		TotalSize:   cacheSize,
		LastCleanup: time.Now(),
	}
}
