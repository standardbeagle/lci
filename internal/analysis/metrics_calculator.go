package analysis

import (
	"math"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// MetricsCalculator provides comprehensive code quality and dependency metrics
type MetricsCalculator struct {
	// Reuse complexity calculation from duplicate detector
	duplicateDetector *DuplicateDetector

	// Reference tracker for dependency analysis (optional)
	refTracker ReferenceProvider

	// Cache for expensive calculations
	metricsCache map[types.SymbolID]*SymbolMetrics
	fileMetrics  map[types.FileID]*FileMetrics

	mu sync.RWMutex
}

// ReferenceProvider provides access to symbol reference data
type ReferenceProvider interface {
	GetEnhancedSymbol(symbolID types.SymbolID) *types.EnhancedSymbol
}

// CodeQualityMetrics represents various code quality measurements
type CodeQualityMetrics struct {
	CyclomaticComplexity int     `json:"cyclomatic_complexity"`
	CognitiveComplexity  int     `json:"cognitive_complexity"`
	NestingDepth         int     `json:"nesting_depth"`
	LinesOfCode          int     `json:"lines_of_code"`
	LinesOfComments      int     `json:"lines_of_comments"`
	HalsteadVolume       float64 `json:"halstead_volume"`
	HalsteadDifficulty   float64 `json:"halstead_difficulty"`
	MaintainabilityIndex float64 `json:"maintainability_index"`
}

// DependencyMetrics represents dependency-related measurements
type DependencyMetrics struct {
	IncomingDependencies   int     `json:"incoming_dependencies"`
	OutgoingDependencies   int     `json:"outgoing_dependencies"`
	TransitiveDependencies int     `json:"transitive_dependencies"`
	CouplingStrength       float64 `json:"coupling_strength"` // 0-1 scale
	CohesionStrength       float64 `json:"cohesion_strength"` // 0-1 scale
	DepthInCallTree        int     `json:"depth_in_call_tree"`
	HasCircularDeps        bool    `json:"has_circular_deps"`
	StabilityIndex         float64 `json:"stability_index"`   // incoming/(incoming+outgoing)
	InstabilityIndex       float64 `json:"instability_index"` // outgoing/(incoming+outgoing)
}

// ArchitectureMetrics represents higher-level architectural measurements
type ArchitectureMetrics struct {
	ModuleStability      float64 `json:"module_stability"`       // incoming/(incoming+outgoing)
	AbstractionLevel     float64 `json:"abstraction_level"`      // interfaces/total_types
	DistanceFromMainSeq  float64 `json:"distance_main_sequence"` // |A + I - 1|
	ComponentCohesion    float64 `json:"component_cohesion"`
	InterfaceSegregation float64 `json:"interface_segregation"`
	DependencyInversion  float64 `json:"dependency_inversion"`
}

// SymbolMetrics combines all metrics for a symbol
type SymbolMetrics struct {
	SymbolID types.SymbolID   `json:"symbol_id"`
	Name     string           `json:"name"`
	Type     types.SymbolType `json:"type"`
	FileID   types.FileID     `json:"file_id"`
	FilePath string           `json:"file_path"`

	// Core metrics
	Quality      CodeQualityMetrics  `json:"quality"`
	Dependencies DependencyMetrics   `json:"dependencies"`
	Architecture ArchitectureMetrics `json:"architecture"`

	// Additional context
	ScopeChain []types.ScopeInfo `json:"scope_chain"`
	Tags       []string          `json:"tags"`       // HIGH_COMPLEXITY, PUBLIC_API, etc.
	RiskScore  int               `json:"risk_score"` // 0-10 risk assessment

	// Timestamps for caching
	CalculatedAt int64 `json:"calculated_at"`
	IsStale      bool  `json:"is_stale"`
}

// FileMetrics aggregates metrics for an entire file
type FileMetrics struct {
	FileID   types.FileID `json:"file_id"`
	FilePath string       `json:"file_path"`

	// Aggregated metrics
	TotalComplexity   int     `json:"total_complexity"`
	AverageComplexity float64 `json:"average_complexity"`
	MaxComplexity     int     `json:"max_complexity"`
	TotalLines        int     `json:"total_lines"`
	CodeLines         int     `json:"code_lines"`
	CommentLines      int     `json:"comment_lines"`

	// Dependency metrics
	InternalDependencies int `json:"internal_dependencies"`
	ExternalDependencies int `json:"external_dependencies"`
	Dependents           int `json:"dependents"`

	// Symbol breakdown
	Functions  int `json:"functions"`
	Classes    int `json:"classes"`
	Interfaces int `json:"interfaces"`
	Variables  int `json:"variables"`

	// Quality indicators
	OverallQualityScore float64 `json:"overall_quality_score"`
	TechnicalDebt       float64 `json:"technical_debt"`
	MaintainabilityRank string  `json:"maintainability_rank"` // A, B, C, D, F

	// Hot spots
	MostComplexSymbol *SymbolMetrics   `json:"most_complex_symbol,omitempty"`
	HighRiskSymbols   []*SymbolMetrics `json:"high_risk_symbols,omitempty"`
}

// MetricsFilter provides filtering options for metrics queries
type MetricsFilter struct {
	MinComplexity    int      `json:"min_complexity,omitempty"`
	MaxComplexity    int      `json:"max_complexity,omitempty"`
	MinDependencies  int      `json:"min_dependencies,omitempty"`
	MaxDependencies  int      `json:"max_dependencies,omitempty"`
	SymbolTypes      []string `json:"symbol_types,omitempty"`
	CouplingStrength string   `json:"coupling_strength,omitempty"` // tight, loose, any
	HasCircularDeps  *bool    `json:"has_circular_deps,omitempty"`
	MinRiskScore     int      `json:"min_risk_score,omitempty"`
	MaxRiskScore     int      `json:"max_risk_score,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	FilePaths        []string `json:"file_paths,omitempty"`
}

// NewMetricsCalculator creates a new metrics calculator
func NewMetricsCalculator(duplicateDetector *DuplicateDetector) *MetricsCalculator {
	return &MetricsCalculator{
		duplicateDetector: duplicateDetector,
		refTracker:        nil, // Optional - can be set with SetRefTracker
		metricsCache:      make(map[types.SymbolID]*SymbolMetrics),
		fileMetrics:       make(map[types.FileID]*FileMetrics),
	}
}

// SetRefTracker sets the reference provider for advanced dependency analysis
func (mc *MetricsCalculator) SetRefTracker(refTracker ReferenceProvider) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.refTracker = refTracker
}

// CalculateSymbolMetrics computes comprehensive metrics for a symbol
func (mc *MetricsCalculator) CalculateSymbolMetrics(symbol *types.EnhancedSymbol, node *tree_sitter.Node, content []byte) *SymbolMetrics {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check cache first
	if cached, exists := mc.metricsCache[symbol.ID]; exists && !cached.IsStale {
		return cached
	}

	metrics := &SymbolMetrics{
		SymbolID:   symbol.ID,
		Name:       symbol.Name,
		Type:       symbol.Type,
		FileID:     symbol.FileID,
		ScopeChain: symbol.ScopeChain,
	}

	// Calculate quality metrics
	metrics.Quality = mc.calculateQualityMetrics(symbol, node, content)

	// Calculate dependency metrics
	metrics.Dependencies = mc.calculateDependencyMetrics(symbol)

	// Calculate architecture metrics
	metrics.Architecture = mc.calculateArchitectureMetrics(symbol)

	// Calculate risk score and tags
	metrics.RiskScore = mc.calculateRiskScore(metrics)
	metrics.Tags = mc.generateTags(metrics)

	// Cache the result
	mc.metricsCache[symbol.ID] = metrics

	return metrics
}

// calculateQualityMetrics computes code quality metrics
func (mc *MetricsCalculator) calculateQualityMetrics(symbol *types.EnhancedSymbol, node *tree_sitter.Node, content []byte) CodeQualityMetrics {
	quality := CodeQualityMetrics{}

	if node != nil {
		// Reuse cyclomatic complexity from duplicate detector
		quality.CyclomaticComplexity = mc.duplicateDetector.calculateComplexity(node)

		// Calculate cognitive complexity (weights different structures differently)
		quality.CognitiveComplexity = mc.calculateCognitiveComplexity(node)

		// Calculate nesting depth
		quality.NestingDepth = mc.calculateNestingDepth(node)

		// Calculate lines of code
		quality.LinesOfCode = symbol.EndLine - symbol.Line + 1

		// Calculate Halstead metrics
		operators, operands := mc.extractHalsteadMetrics(node, content)
		quality.HalsteadVolume = mc.calculateHalsteadVolume(operators, operands)
		quality.HalsteadDifficulty = mc.calculateHalsteadDifficulty(operators, operands)

		// Calculate maintainability index
		quality.MaintainabilityIndex = mc.calculateMaintainabilityIndex(quality)
	}

	return quality
}

// calculateCognitiveComplexity implements cognitive complexity rules
func (mc *MetricsCalculator) calculateCognitiveComplexity(node *tree_sitter.Node) int {
	complexity := 0
	nesting := 0

	mc.walkForCognitiveComplexity(node, &complexity, &nesting)

	return complexity
}

// walkForCognitiveComplexity implements cognitive complexity calculation
func (mc *MetricsCalculator) walkForCognitiveComplexity(node *tree_sitter.Node, complexity *int, nesting *int) {
	nodeType := node.Kind()

	// Cognitive complexity weights by nesting and type
	switch nodeType {
	case "if_statement":
		*complexity += 1 + *nesting
	case "else_clause":
		*complexity += 1
	case "switch_statement", "match_statement":
		*complexity += 1 + *nesting
	case "for_statement", "while_statement", "do_statement":
		*complexity += 1 + *nesting
		*nesting++ // Increase nesting for children
	case "catch_clause", "except_clause":
		*complexity += 1 + *nesting
	case "conditional_expression":
		*complexity += 1
	case "goto_statement":
		*complexity += 1 + *nesting
	case "lambda_expression", "arrow_function":
		*complexity += 1 // Don't increase nesting for lambdas
	}

	// Recursively process children
	childNesting := *nesting
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		mc.walkForCognitiveComplexity(child, complexity, &childNesting)
	}
}

// calculateNestingDepth finds the maximum nesting depth
func (mc *MetricsCalculator) calculateNestingDepth(node *tree_sitter.Node) int {
	maxDepth := 0
	currentDepth := 0

	mc.walkForNestingDepth(node, &maxDepth, &currentDepth)

	return maxDepth
}

// walkForNestingDepth calculates maximum nesting depth
func (mc *MetricsCalculator) walkForNestingDepth(node *tree_sitter.Node, maxDepth *int, currentDepth *int) {
	nodeType := node.Kind()

	// Check if this is a nesting construct
	isNesting := false
	switch nodeType {
	case "if_statement", "for_statement", "while_statement", "switch_statement",
		"try_statement", "function_declaration", "method_definition",
		"class_declaration", "block_statement", "compound_statement":
		isNesting = true
	}

	if isNesting {
		*currentDepth++
		if *currentDepth > *maxDepth {
			*maxDepth = *currentDepth
		}
	}

	// Recursively process children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		mc.walkForNestingDepth(child, maxDepth, currentDepth)
	}

	if isNesting {
		*currentDepth--
	}
}

// calculateDependencyMetrics computes dependency-related metrics
func (mc *MetricsCalculator) calculateDependencyMetrics(symbol *types.EnhancedSymbol) DependencyMetrics {
	deps := DependencyMetrics{
		IncomingDependencies: len(symbol.IncomingRefs),
		OutgoingDependencies: len(symbol.OutgoingRefs),
	}

	// Calculate coupling strength (0-1 scale)
	totalRefs := deps.IncomingDependencies + deps.OutgoingDependencies
	if totalRefs > 0 {
		// Higher coupling = more connections relative to total symbols
		deps.CouplingStrength = float64(totalRefs) / 100.0 // Normalize to reasonable scale
		if deps.CouplingStrength > 1.0 {
			deps.CouplingStrength = 1.0
		}
	}

	// Calculate stability (Martin's stability metric)
	if totalRefs > 0 {
		deps.StabilityIndex = float64(deps.IncomingDependencies) / float64(totalRefs)
		deps.InstabilityIndex = float64(deps.OutgoingDependencies) / float64(totalRefs)
	}

	// Calculate transitive dependencies using DFS through reference graph
	deps.TransitiveDependencies = mc.calculateTransitiveDeps(symbol)

	// Detect circular dependencies by checking if any transitive dependency points back
	deps.HasCircularDeps = mc.detectCircularDeps(symbol)

	// Calculate cohesion based on shared references among outgoing dependencies
	deps.CohesionStrength = mc.calculateCohesion(symbol)

	return deps
}

// calculateArchitectureMetrics computes higher-level architectural metrics
func (mc *MetricsCalculator) calculateArchitectureMetrics(symbol *types.EnhancedSymbol) ArchitectureMetrics {
	arch := ArchitectureMetrics{}

	// Module stability (same as dependency stability for now)
	totalRefs := len(symbol.IncomingRefs) + len(symbol.OutgoingRefs)
	if totalRefs > 0 {
		arch.ModuleStability = float64(len(symbol.IncomingRefs)) / float64(totalRefs)
	}

	// Calculate abstraction level based on symbol type and references
	arch.AbstractionLevel = mc.calculateAbstractionLevel(symbol)

	// Calculate distance from main sequence (Martin's metric: D = |A + I - 1|)
	// Where A = abstractness and I = instability
	if totalRefs > 0 {
		instability := float64(len(symbol.OutgoingRefs)) / float64(totalRefs)
		arch.DistanceFromMainSeq = abs(arch.AbstractionLevel + instability - 1.0)
	}

	return arch
}

// extractHalsteadMetrics identifies operators and operands for Halstead complexity
func (mc *MetricsCalculator) extractHalsteadMetrics(node *tree_sitter.Node, content []byte) (map[string]int, map[string]int) {
	operators := make(map[string]int)
	operands := make(map[string]int)

	mc.walkForHalsteadMetrics(node, content, operators, operands)

	return operators, operands
}

// walkForHalsteadMetrics recursively extracts Halstead operators and operands
func (mc *MetricsCalculator) walkForHalsteadMetrics(node *tree_sitter.Node, content []byte, operators map[string]int, operands map[string]int) {
	nodeType := node.Kind()

	// Classify node as operator or operand
	switch nodeType {
	case "binary_expression", "unary_expression", "assignment_expression":
		// Extract the actual operator
		if int(node.EndByte()) <= len(content) {
			op := string(content[node.StartByte():node.EndByte()])
			operators[op]++
		}
	case "identifier", "number", "string", "boolean":
		// Extract the actual operand
		if int(node.EndByte()) <= len(content) {
			operand := string(content[node.StartByte():node.EndByte()])
			operands[operand]++
		}
	case "if", "else", "for", "while", "switch", "case", "return", "break", "continue":
		operators[nodeType]++
	}

	// Recursively process children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		mc.walkForHalsteadMetrics(child, content, operators, operands)
	}
}

// calculateHalsteadVolume computes Halstead volume metric
func (mc *MetricsCalculator) calculateHalsteadVolume(operators, operands map[string]int) float64 {
	n1 := len(operators) // Number of distinct operators
	n2 := len(operands)  // Number of distinct operands
	N1 := 0              // Total number of operators
	N2 := 0              // Total number of operands

	for _, count := range operators {
		N1 += count
	}
	for _, count := range operands {
		N2 += count
	}

	vocabulary := n1 + n2
	length := N1 + N2

	if vocabulary == 0 {
		return 0
	}

	return float64(length) * math.Log2(float64(vocabulary))
}

// calculateHalsteadDifficulty computes Halstead difficulty metric
func (mc *MetricsCalculator) calculateHalsteadDifficulty(operators, operands map[string]int) float64 {
	n1 := len(operators)
	n2 := len(operands)
	N2 := 0

	for _, count := range operands {
		N2 += count
	}

	if n2 == 0 || N2 == 0 {
		return 0
	}

	return (float64(n1) / 2.0) * (float64(N2) / float64(n2))
}

// calculateMaintainabilityIndex computes the maintainability index
func (mc *MetricsCalculator) calculateMaintainabilityIndex(quality CodeQualityMetrics) float64 {
	// Microsoft's maintainability index formula (simplified)
	// MI = 171 - 5.2 * ln(HalsteadVolume) - 0.23 * CyclomaticComplexity - 16.2 * ln(LinesOfCode)

	if quality.HalsteadVolume <= 0 || quality.LinesOfCode <= 0 {
		return 0
	}

	mi := 171.0 - 5.2*math.Log(quality.HalsteadVolume) - 0.23*float64(quality.CyclomaticComplexity) - 16.2*math.Log(float64(quality.LinesOfCode))

	// Normalize to 0-100 scale
	if mi < 0 {
		mi = 0
	}
	if mi > 100 {
		mi = 100
	}

	return mi
}

// calculateRiskScore computes an overall risk score (0-10)
func (mc *MetricsCalculator) calculateRiskScore(metrics *SymbolMetrics) int {
	score := 0

	// Complexity contribution (0-4 points)
	if metrics.Quality.CyclomaticComplexity > 20 {
		score += 4
	} else if metrics.Quality.CyclomaticComplexity > 15 {
		score += 3
	} else if metrics.Quality.CyclomaticComplexity > 10 {
		score += 2
	} else if metrics.Quality.CyclomaticComplexity > 5 {
		score += 1
	}

	// Dependency contribution (0-3 points)
	totalDeps := metrics.Dependencies.IncomingDependencies + metrics.Dependencies.OutgoingDependencies
	if totalDeps > 20 {
		score += 3
	} else if totalDeps > 10 {
		score += 2
	} else if totalDeps > 5 {
		score += 1
	}

	// Maintainability contribution (0-3 points)
	if metrics.Quality.MaintainabilityIndex < 10 {
		score += 3
	} else if metrics.Quality.MaintainabilityIndex < 25 {
		score += 2
	} else if metrics.Quality.MaintainabilityIndex < 50 {
		score += 1
	}

	if score > 10 {
		score = 10
	}

	return score
}

// generateTags creates descriptive tags based on metrics
func (mc *MetricsCalculator) generateTags(metrics *SymbolMetrics) []string {
	var tags []string

	// Complexity tags
	if metrics.Quality.CyclomaticComplexity > 15 {
		tags = append(tags, "HIGH_COMPLEXITY")
	}
	if metrics.Quality.CognitiveComplexity > 25 {
		tags = append(tags, "HIGH_COGNITIVE_LOAD")
	}
	if metrics.Quality.NestingDepth > 5 {
		tags = append(tags, "DEEP_NESTING")
	}

	// Dependency tags
	if metrics.Dependencies.IncomingDependencies > 10 {
		tags = append(tags, "HIGHLY_COUPLED")
	}
	if metrics.Dependencies.OutgoingDependencies > 15 {
		tags = append(tags, "MANY_DEPENDENCIES")
	}
	if metrics.Dependencies.HasCircularDeps {
		tags = append(tags, "CIRCULAR_DEPS")
	}

	// Architecture tags
	if metrics.Type == types.SymbolTypeInterface {
		tags = append(tags, "INTERFACE")
	}
	if len(metrics.ScopeChain) == 1 { // Global scope
		tags = append(tags, "GLOBAL")
	}

	// Risk tags
	if metrics.RiskScore >= 8 {
		tags = append(tags, "HIGH_RISK")
	} else if metrics.RiskScore >= 5 {
		tags = append(tags, "MEDIUM_RISK")
	}

	// Quality tags
	if metrics.Quality.MaintainabilityIndex < 10 {
		tags = append(tags, "HARD_TO_MAINTAIN")
	}
	if metrics.Quality.MaintainabilityIndex > 85 {
		tags = append(tags, "WELL_MAINTAINED")
	}

	return tags
}

// FilterSymbolsByMetrics filters symbols based on metrics criteria
func (mc *MetricsCalculator) FilterSymbolsByMetrics(symbols []*types.EnhancedSymbol, filter MetricsFilter) []*SymbolMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var filtered []*SymbolMetrics

	for _, symbol := range symbols {
		// Get metrics for symbol (may need to calculate if not cached)
		metrics, exists := mc.metricsCache[symbol.ID]
		if !exists {
			continue // Skip if no metrics available
		}

		// Apply filters
		if !mc.matchesFilter(metrics, filter) {
			continue
		}

		filtered = append(filtered, metrics)
	}

	return filtered
}

// matchesFilter checks if metrics match the given filter criteria
func (mc *MetricsCalculator) matchesFilter(metrics *SymbolMetrics, filter MetricsFilter) bool {
	// Complexity filters
	if filter.MinComplexity > 0 && metrics.Quality.CyclomaticComplexity < filter.MinComplexity {
		return false
	}
	if filter.MaxComplexity > 0 && metrics.Quality.CyclomaticComplexity > filter.MaxComplexity {
		return false
	}

	// Dependency filters
	totalDeps := metrics.Dependencies.IncomingDependencies + metrics.Dependencies.OutgoingDependencies
	if filter.MinDependencies > 0 && totalDeps < filter.MinDependencies {
		return false
	}
	if filter.MaxDependencies > 0 && totalDeps > filter.MaxDependencies {
		return false
	}

	// Coupling strength filter
	switch filter.CouplingStrength {
	case "tight":
		if metrics.Dependencies.CouplingStrength < 0.7 {
			return false
		}
	case "loose":
		if metrics.Dependencies.CouplingStrength > 0.3 {
			return false
		}
	}

	// Circular dependency filter
	if filter.HasCircularDeps != nil && metrics.Dependencies.HasCircularDeps != *filter.HasCircularDeps {
		return false
	}

	// Risk score filters
	if filter.MinRiskScore > 0 && metrics.RiskScore < filter.MinRiskScore {
		return false
	}
	if filter.MaxRiskScore > 0 && metrics.RiskScore > filter.MaxRiskScore {
		return false
	}

	// Symbol type filter
	if len(filter.SymbolTypes) > 0 {
		typeMatch := false
		symbolTypeStr := metrics.Type.String()
		for _, filterType := range filter.SymbolTypes {
			if symbolTypeStr == filterType {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}

	// Tags filter
	if len(filter.Tags) > 0 {
		for _, filterTag := range filter.Tags {
			tagMatch := false
			for _, tag := range metrics.Tags {
				if tag == filterTag {
					tagMatch = true
					break
				}
			}
			if !tagMatch {
				return false
			}
		}
	}

	return true
}

// InvalidateCache marks all cached metrics as stale
func (mc *MetricsCalculator) InvalidateCache() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, metrics := range mc.metricsCache {
		metrics.IsStale = true
	}
}

// GetCacheStats returns cache statistics
func (mc *MetricsCalculator) GetCacheStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	staleCount := 0
	for _, metrics := range mc.metricsCache {
		if metrics.IsStale {
			staleCount++
		}
	}

	return map[string]interface{}{
		"total_cached":   len(mc.metricsCache),
		"stale_entries":  staleCount,
		"cache_hit_rate": float64(len(mc.metricsCache)-staleCount) / float64(len(mc.metricsCache)),
		"file_metrics":   len(mc.fileMetrics),
	}
}

// calculateTransitiveDeps computes transitive dependencies using DFS
func (mc *MetricsCalculator) calculateTransitiveDeps(symbol *types.EnhancedSymbol) int {
	visited := make(map[types.SymbolID]bool)
	return mc.dfsTransitiveDeps(symbol.ID, visited)
}

// dfsTransitiveDeps performs depth-first search to count transitive dependencies
func (mc *MetricsCalculator) dfsTransitiveDeps(symbolID types.SymbolID, visited map[types.SymbolID]bool) int {
	if visited[symbolID] {
		return 0
	}
	visited[symbolID] = true

	count := 0
	// Get the symbol from reference tracker
	if mc.refTracker != nil {
		if sym := mc.refTracker.GetEnhancedSymbol(symbolID); sym != nil {
			for _, ref := range sym.OutgoingRefs {
				if ref.TargetSymbol != 0 && !visited[ref.TargetSymbol] {
					count++ // Count this dependency
					count += mc.dfsTransitiveDeps(ref.TargetSymbol, visited)
				}
			}
		}
	}
	return count
}

// detectCircularDeps detects if this symbol is part of a circular dependency
func (mc *MetricsCalculator) detectCircularDeps(symbol *types.EnhancedSymbol) bool {
	visited := make(map[types.SymbolID]bool)
	recursionStack := make(map[types.SymbolID]bool)
	circularCount := 0

	mc.dfsDetectCycles(symbol.ID, visited, recursionStack, &circularCount)
	return circularCount > 0
}

// dfsDetectCycles uses DFS with recursion stack to detect cycles
func (mc *MetricsCalculator) dfsDetectCycles(symbolID types.SymbolID, visited, recursionStack map[types.SymbolID]bool, count *int) {
	visited[symbolID] = true
	recursionStack[symbolID] = true

	if mc.refTracker != nil {
		if sym := mc.refTracker.GetEnhancedSymbol(symbolID); sym != nil {
			for _, ref := range sym.OutgoingRefs {
				targetID := ref.TargetSymbol
				if targetID == 0 {
					continue
				}

				if !visited[targetID] {
					mc.dfsDetectCycles(targetID, visited, recursionStack, count)
				} else if recursionStack[targetID] {
					// Found a cycle!
					*count++
				}
			}
		}
	}

	recursionStack[symbolID] = false
}

// calculateCohesion measures how related the outgoing dependencies are
func (mc *MetricsCalculator) calculateCohesion(symbol *types.EnhancedSymbol) float64 {
	if len(symbol.OutgoingRefs) < 2 {
		return 1.0 // Perfect cohesion for 0 or 1 dependency
	}

	// Count shared references between dependencies
	sharedRefs := 0
	totalPairs := 0

	for i, ref1 := range symbol.OutgoingRefs {
		for j := i + 1; j < len(symbol.OutgoingRefs); j++ {
			ref2 := symbol.OutgoingRefs[j]
			totalPairs++

			// Check if these two dependencies share references
			if mc.haveSharedReferences(ref1.TargetSymbol, ref2.TargetSymbol) {
				sharedRefs++
			}
		}
	}

	if totalPairs == 0 {
		return 1.0
	}

	return float64(sharedRefs) / float64(totalPairs)
}

// haveSharedReferences checks if two symbols reference common dependencies
func (mc *MetricsCalculator) haveSharedReferences(id1, id2 types.SymbolID) bool {
	if mc.refTracker == nil || id1 == 0 || id2 == 0 {
		return false
	}

	sym1 := mc.refTracker.GetEnhancedSymbol(id1)
	sym2 := mc.refTracker.GetEnhancedSymbol(id2)

	if sym1 == nil || sym2 == nil {
		return false
	}

	// Build set of sym1's dependencies
	deps1 := make(map[types.SymbolID]bool)
	for _, ref := range sym1.OutgoingRefs {
		if ref.TargetSymbol != 0 {
			deps1[ref.TargetSymbol] = true
		}
	}

	// Check if sym2 has any matching dependencies
	for _, ref := range sym2.OutgoingRefs {
		if ref.TargetSymbol != 0 && deps1[ref.TargetSymbol] {
			return true
		}
	}

	return false
}

// calculateAbstractionLevel determines how abstract a symbol is
func (mc *MetricsCalculator) calculateAbstractionLevel(symbol *types.EnhancedSymbol) float64 {
	// Interfaces and abstract types are highly abstract
	if symbol.Type == types.SymbolTypeInterface {
		return 1.0
	}

	// Classes/types can be partially abstract
	if symbol.Type == types.SymbolTypeClass || symbol.Type == types.SymbolTypeType {
		// Check if it has many incoming references (used as abstraction)
		// vs outgoing (uses concrete implementations)
		totalRefs := len(symbol.IncomingRefs) + len(symbol.OutgoingRefs)
		if totalRefs > 0 {
			return float64(len(symbol.IncomingRefs)) / float64(totalRefs)
		}
		return 0.5
	}

	// Functions and variables are concrete
	return 0.0
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
