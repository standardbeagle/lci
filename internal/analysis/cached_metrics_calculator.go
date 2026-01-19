package analysis

import (
	"fmt"
	"math"
	"time"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/cache"
	"github.com/standardbeagle/lci/internal/types"
)

// CachedMetricsCalculator wraps MetricsCalculator with intelligent caching
type CachedMetricsCalculator struct {
	*MetricsCalculator                     // Embed the original calculator
	cache              *cache.MetricsCache // Intelligent caching layer

	// Configuration
	enableCaching bool

	// Statistics for monitoring
	cacheHits   int64
	cacheMisses int64
}

// CachedMetricsConfig provides configuration for cached metrics calculator
type CachedMetricsConfig struct {
	EnableCaching bool              `json:"enable_caching"`
	CacheConfig   cache.CacheConfig `json:"cache_config"`
	MetricsConfig MetricsConfig     `json:"metrics_config"`
}

// MetricsConfig provides configuration for the underlying metrics calculator
type MetricsConfig struct {
	EnableHalstead          bool `json:"enable_halstead"`
	EnableMaintainability   bool `json:"enable_maintainability"`
	EnableDependencyMetrics bool `json:"enable_dependency_metrics"`
}

// DefaultCachedMetricsConfig returns a production-ready configuration
func DefaultCachedMetricsConfig() CachedMetricsConfig {
	return CachedMetricsConfig{
		EnableCaching: true,
		CacheConfig:   cache.DefaultCacheConfig(),
		MetricsConfig: MetricsConfig{
			EnableHalstead:          true,
			EnableMaintainability:   true,
			EnableDependencyMetrics: true,
		},
	}
}

// NewCachedMetricsCalculator creates a new cached metrics calculator
func NewCachedMetricsCalculator(config CachedMetricsConfig) *CachedMetricsCalculator {
	var metricsCache *cache.MetricsCache
	if config.EnableCaching {
		metricsCache = cache.NewMetricsCache(config.CacheConfig)
	}

	// Create underlying metrics calculator
	metricsCalculator := &MetricsCalculator{
		duplicateDetector: NewDuplicateDetector(),
		metricsCache:      make(map[types.SymbolID]*SymbolMetrics),
		fileMetrics:       make(map[types.FileID]*FileMetrics),
	}

	return &CachedMetricsCalculator{
		MetricsCalculator: metricsCalculator,
		cache:             metricsCache,
		enableCaching:     config.EnableCaching,
	}
}

// CalculateSymbolMetrics calculates metrics for an enhanced symbol with intelligent caching
func (c *CachedMetricsCalculator) CalculateSymbolMetrics(
	symbol *types.EnhancedSymbol,
	node *tree_sitter.Node,
	content []byte,
) (*SymbolMetrics, error) {
	symbolName := symbol.Name

	// Try cache first if enabled
	if c.enableCaching && c.cache != nil {
		if cached := c.cache.Get(content, int(symbol.FileID), symbolName); cached != nil {
			c.cacheHits++
			if metrics, ok := cached.(*SymbolMetrics); ok {
				return metrics, nil
			}
		}
		c.cacheMisses++
	}

	// Create basic metrics structure
	metrics := &SymbolMetrics{
		SymbolID:     symbol.ID,
		Name:         symbolName,
		Type:         symbol.Type,
		FileID:       symbol.FileID,
		CalculatedAt: time.Now().Unix(),
	}

	// Calculate code quality metrics
	qualityMetrics, err := c.calculateCodeQualityMetrics(content, node)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate quality metrics: %w", err)
	}
	metrics.Quality = *qualityMetrics

	// Calculate dependency metrics if requested
	depMetrics, err := c.calculateDependencyMetrics(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate dependency metrics: %w", err)
	}
	metrics.Dependencies = *depMetrics

	// Store in cache if enabled
	if c.enableCaching && c.cache != nil {
		c.cache.Put(content, int(symbol.FileID), symbolName, metrics)
	}

	return metrics, nil
}

// calculateCodeQualityMetrics calculates comprehensive code quality metrics
func (c *CachedMetricsCalculator) calculateCodeQualityMetrics(content []byte, node *tree_sitter.Node) (*CodeQualityMetrics, error) {
	metrics := &CodeQualityMetrics{}

	// Calculate basic complexity metrics
	metrics.CyclomaticComplexity = c.calculateCyclomaticComplexity(node)
	metrics.CognitiveComplexity = c.calculateCognitiveComplexity(node)
	metrics.NestingDepth = c.calculateNestingDepth(node, 0)

	// Calculate lines of code metrics
	metrics.LinesOfCode = c.calculateLinesOfCode(node)
	metrics.LinesOfComments = c.calculateLinesOfComments(node)

	// Calculate advanced metrics (Halstead, maintainability)
	halsteadMetrics := c.calculateHalsteadMetrics(node)
	metrics.HalsteadVolume = halsteadMetrics.Volume
	metrics.HalsteadDifficulty = halsteadMetrics.Difficulty

	// Calculate maintainability index
	metrics.MaintainabilityIndex = c.calculateMaintainabilityIndex(metrics)

	return metrics, nil
}

// calculateDependencyMetrics calculates dependency-related metrics
func (c *CachedMetricsCalculator) calculateDependencyMetrics(symbol *types.EnhancedSymbol) (*DependencyMetrics, error) {
	// Placeholder implementation - would integrate with dependency tracking
	return &DependencyMetrics{
		IncomingDependencies:   0,
		OutgoingDependencies:   0,
		TransitiveDependencies: 0,
		CouplingStrength:       0.0,
		CohesionStrength:       1.0,
		DepthInCallTree:        0,
		HasCircularDeps:        false,
		StabilityIndex:         1.0,
		InstabilityIndex:       0.0,
	}, nil
}

// Halstead metrics calculation
type HalsteadMetrics struct {
	Volume     float64
	Difficulty float64
	Effort     float64
	Time       float64
	Bugs       float64
}

// calculateHalsteadMetrics calculates Halstead complexity metrics
func (c *CachedMetricsCalculator) calculateHalsteadMetrics(node *tree_sitter.Node) HalsteadMetrics {
	if node == nil {
		return HalsteadMetrics{}
	}

	operators := make(map[string]int)
	operands := make(map[string]int)

	c.collectHalsteadOperators(node, operators, operands)

	n1 := len(operators)         // Unique operators
	n2 := len(operands)          // Unique operands
	N1 := c.sumValues(operators) // Total operators
	N2 := c.sumValues(operands)  // Total operands

	if n1 == 0 || n2 == 0 {
		return HalsteadMetrics{}
	}

	vocabulary := float64(n1 + n2)
	length := float64(N1 + N2)
	volume := length * logBase2(vocabulary)
	difficulty := (float64(n1) / 2.0) * (float64(N2) / float64(n2))
	effort := difficulty * volume
	time := effort / 18.0   // Halstead's constant
	bugs := volume / 3000.0 // Halstead's constant

	return HalsteadMetrics{
		Volume:     volume,
		Difficulty: difficulty,
		Effort:     effort,
		Time:       time,
		Bugs:       bugs,
	}
}

// calculateMaintainabilityIndex calculates the maintainability index
func (c *CachedMetricsCalculator) calculateMaintainabilityIndex(metrics *CodeQualityMetrics) float64 {
	// Microsoft's maintainability index formula
	loc := float64(metrics.LinesOfCode)
	cc := float64(metrics.CyclomaticComplexity)
	hv := metrics.HalsteadVolume

	if loc == 0 {
		return 100.0
	}

	// Maintainability Index = 171 - 5.2 * ln(HV) - 0.23 * CC - 16.2 * ln(LOC)
	mi := 171.0 - 5.2*logNatural(hv) - 0.23*cc - 16.2*logNatural(loc)

	// Normalize to 0-100 scale
	if mi < 0 {
		return 0
	}
	if mi > 100 {
		return 100
	}

	return mi
}

// Helper functions for complexity calculations
func (c *CachedMetricsCalculator) calculateCyclomaticComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 1
	}

	// Cyclomatic complexity = edges - nodes + 2
	// Start with 1 (base complexity)
	complexity := 1

	// Add 1 for each decision point
	c.walkNodeForCyclomatic(node, &complexity)

	return complexity
}

func (c *CachedMetricsCalculator) walkNodeForCyclomatic(node *tree_sitter.Node, complexity *int) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Decision points that increase cyclomatic complexity
	// Supports multiple languages: Go, JavaScript, Python, etc.
	switch nodeType {
	// If statements (all languages)
	case "if_statement", "if_expression":
		*complexity++

	// For/while loops (all languages)
	case "for_statement", "for_range_statement", "for_in_statement":
		*complexity++
	case "while_statement", "do_while_statement":
		*complexity++

	// Switch statements (generic and language-specific)
	case "switch_statement", "switch_expression":
		// Generic switch - don't count the switch itself, just the cases
	case "expression_switch_statement", "type_switch_statement": // Go
		// Don't count the switch itself, just the cases

	// Case clauses (all add a path)
	case "case_clause", "case_statement": // Generic
		*complexity++
	case "expression_case", "type_case": // Go specific
		*complexity++
	case "default_case": // Go default case
		// Default case doesn't add complexity (it's the fallthrough path)

	// Ternary/conditional expressions
	case "conditional_expression", "ternary_expression":
		*complexity++

	// Logical operators in binary expressions
	case "binary_expression":
		// Check for logical operators (&&, ||, and, or)
		if node.ChildCount() >= 3 {
			operatorNode := node.Child(1)
			if operatorNode != nil {
				operator := operatorNode.Kind()
				if operator == "&&" || operator == "||" || operator == "and" || operator == "or" {
					*complexity++
				}
			}
		}

	// Exception handling
	case "catch_clause", "except_clause":
		*complexity++
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		c.walkNodeForCyclomatic(child, complexity)
	}
}

func (c *CachedMetricsCalculator) calculateCognitiveComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}

	complexity := 0
	c.calculateCognitiveComplexityRecursive(node, 0, &complexity)
	return complexity
}

func (c *CachedMetricsCalculator) calculateCognitiveComplexityRecursive(node *tree_sitter.Node, nestingLevel int, complexity *int) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Increment complexity based on node type and nesting
	switch nodeType {
	case "if_statement", "if_expression":
		*complexity += 1 + nestingLevel
		nestingLevel++
	case "else_clause", "elif_clause", "else_if_clause":
		*complexity++ // else/elif adds 1 regardless of nesting
	case "switch_statement", "switch_expression":
		*complexity += 1 + nestingLevel
		nestingLevel++
	case "for_statement", "for_range_statement", "for_in_statement", "while_statement", "do_while_statement":
		*complexity += 1 + nestingLevel
		nestingLevel++
	case "break_statement", "continue_statement":
		*complexity++ // Jumps add 1
	case "catch_clause", "except_clause":
		*complexity += 1 + nestingLevel
		nestingLevel++
	case "binary_expression":
		// Check for logical operators
		if node.ChildCount() >= 3 {
			operatorNode := node.Child(1)
			if operatorNode != nil {
				operator := operatorNode.Kind()
				if operator == "&&" || operator == "||" || operator == "and" || operator == "or" {
					*complexity++
				}
			}
		}
	case "conditional_expression", "ternary_expression":
		*complexity += 1 + nestingLevel
	case "function_declaration", "method_declaration", "lambda_expression", "arrow_function":
		// Nested functions increase nesting
		nestingLevel++
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		c.calculateCognitiveComplexityRecursive(child, nestingLevel, complexity)
	}
}

func (c *CachedMetricsCalculator) calculateNestingDepth(node *tree_sitter.Node, currentDepth int) int {
	if node == nil {
		return currentDepth
	}

	maxDepth := currentDepth
	nodeType := node.Kind()

	// Check if this node increases nesting
	increasesNesting := false
	switch nodeType {
	case "if_statement", "if_expression",
		"for_statement", "for_range_statement", "for_in_statement",
		"while_statement", "do_while_statement",
		"switch_statement", "switch_expression",
		"function_declaration", "method_declaration",
		"lambda_expression", "arrow_function",
		"try_statement", "catch_clause":
		increasesNesting = true
		currentDepth++
	}

	// Check all children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childDepth := c.calculateNestingDepth(child, currentDepth)
		if childDepth > maxDepth {
			maxDepth = childDepth
		}
	}

	// Reset depth if we increased it (maintaining invariant for recursive calls)
	if increasesNesting {
		currentDepth--
		_ = currentDepth // Mark as intentionally unused after decrement
	}

	return maxDepth
}

func (c *CachedMetricsCalculator) calculateLinesOfCode(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}
	startRow := int(node.StartPosition().Row)
	endRow := int(node.EndPosition().Row)
	return endRow - startRow + 1
}

func (c *CachedMetricsCalculator) calculateLinesOfComments(node *tree_sitter.Node) int {
	// Simplified implementation - would need language-specific comment detection
	return 0
}

// Helper functions for Halstead metrics
func (c *CachedMetricsCalculator) collectHalsteadOperators(node *tree_sitter.Node, operators, operands map[string]int) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Check if this node represents an operator or operand
	if c.isOperator(nodeType) {
		// For operators, we want to capture the actual operator text
		if nodeType == "binary_expression" || nodeType == "unary_expression" {
			// The operator is usually the middle child in binary expressions
			if nodeType == "binary_expression" && node.ChildCount() >= 3 {
				operatorNode := node.Child(1)
				if operatorNode != nil {
					operatorText := operatorNode.Kind()
					operators[operatorText]++
				}
			} else if nodeType == "unary_expression" && node.ChildCount() >= 1 {
				// First child is usually the operator
				operatorNode := node.Child(0)
				if operatorNode != nil {
					operatorText := operatorNode.Kind()
					operators[operatorText]++
				}
			}
		} else {
			// For other operator types, use the node type itself
			operators[nodeType]++
		}
	} else if c.isOperand(nodeType) {
		// For operands, increment the count
		// In a real implementation, we might want to get the actual text
		operands[nodeType]++
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		c.collectHalsteadOperators(child, operators, operands)
	}
}

func (c *CachedMetricsCalculator) isOperator(nodeType string) bool {
	operators := map[string]bool{
		"binary_expression": true, "unary_expression": true, "assignment": true,
		"call_expression": true, "if_statement": true, "while_statement": true,
		"for_statement": true, "return_statement": true,
	}
	return operators[nodeType]
}

func (c *CachedMetricsCalculator) isOperand(nodeType string) bool {
	operands := map[string]bool{
		"identifier": true, "number": true, "string": true,
		"true": true, "false": true, "null": true,
	}
	return operands[nodeType]
}

func (c *CachedMetricsCalculator) sumValues(m map[string]int) int {
	sum := 0
	for _, v := range m {
		sum += v
	}
	return sum
}

// Math helper functions
func logBase2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Log2(x)
}

func logNatural(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Log(x)
}

// GetCacheStats returns caching performance statistics
func (c *CachedMetricsCalculator) GetCacheStats() cache.CacheStats {
	if c.cache != nil {
		return c.cache.Stats()
	}
	return cache.CacheStats{}
}

// GetCacheInfo returns detailed cache information
func (c *CachedMetricsCalculator) GetCacheInfo() cache.CacheInfo {
	if c.cache != nil {
		return c.cache.GetCacheInfo()
	}
	return cache.CacheInfo{}
}

// ClearCache clears all cached metrics
func (c *CachedMetricsCalculator) ClearCache() {
	if c.cache != nil {
		c.cache.Clear()
	}
}

// SetCacheMaxEntries updates the cache size limit
func (c *CachedMetricsCalculator) SetCacheMaxEntries(maxEntries int) {
	if c.cache != nil {
		c.cache.SetMaxEntries(maxEntries)
	}
}

// UpdateCacheTTL updates the cache time-to-live
func (c *CachedMetricsCalculator) UpdateCacheTTL(ttl time.Duration) {
	if c.cache != nil {
		c.cache.UpdateTTL(ttl)
	}
}

// CalculateCyclomaticComplexity calculates cyclomatic complexity for a node
// Exported for use by git change analyzer
func (c *CachedMetricsCalculator) CalculateCyclomaticComplexity(node *tree_sitter.Node) int {
	return c.calculateCyclomaticComplexity(node)
}

// CalculateNestingDepth calculates the maximum nesting depth for a node
// Exported for use by git change analyzer
func (c *CachedMetricsCalculator) CalculateNestingDepth(node *tree_sitter.Node, currentDepth int) int {
	return c.calculateNestingDepth(node, currentDepth)
}

// CalculateLinesOfCode calculates the lines of code for a node
// Exported for use by git change analyzer
func (c *CachedMetricsCalculator) CalculateLinesOfCode(node *tree_sitter.Node) int {
	return c.calculateLinesOfCode(node)
}
