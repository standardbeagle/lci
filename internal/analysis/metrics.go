package analysis

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// BasicSymbolMetrics represents simple metrics calculated for a symbol (Phase 3A)
type BasicSymbolMetrics struct {
	// Core metrics
	Complexity     float64  `json:"complexity"`
	LinesOfCode    int      `json:"lines_of_code"`
	ReferenceCount int      `json:"reference_count"`
	Dependencies   []string `json:"dependencies"`

	// Quality indicators
	Quality   map[string]float64 `json:"quality"`
	RiskScore float64            `json:"risk_score"`
	Tags      []string           `json:"tags"`
}

// BasicMetricsCalculator calculates simple code metrics for symbols (Phase 3A)
type BasicMetricsCalculator struct {
	// Configuration for metrics calculation
	enableAdvanced bool
}

// NewBasicMetricsCalculator creates a new basic metrics calculator
func NewBasicMetricsCalculator(enableAdvanced bool) *BasicMetricsCalculator {
	return &BasicMetricsCalculator{
		enableAdvanced: enableAdvanced,
	}
}

// CalculateBasicSymbolMetrics calculates metrics for a single symbol using its AST node
func (mc *BasicMetricsCalculator) CalculateBasicSymbolMetrics(symbol *types.EnhancedSymbol, astNode *tree_sitter.Node, content []byte) *BasicSymbolMetrics {
	if symbol == nil || astNode == nil {
		return &BasicSymbolMetrics{
			Complexity:     1.0,
			LinesOfCode:    0,
			ReferenceCount: 0,
			Dependencies:   []string{},
			Quality:        make(map[string]float64),
			RiskScore:      0.0,
			Tags:           []string{},
		}
	}

	metrics := &BasicSymbolMetrics{
		Quality:      make(map[string]float64),
		Dependencies: []string{},
		Tags:         []string{},
	}

	// Phase 3A: Simple metrics only
	metrics.ReferenceCount = len(symbol.IncomingRefs) + len(symbol.OutgoingRefs)
	metrics.LinesOfCode = mc.calculateLinesOfCode(astNode)
	metrics.Complexity = mc.calculateBasicComplexity(astNode)

	// Basic quality indicators based on simple metrics
	metrics.Quality["reference_density"] = mc.calculateReferenceDensity(symbol)
	metrics.Quality["size_score"] = mc.calculateSizeScore(metrics.LinesOfCode)

	// Calculate basic risk score
	metrics.RiskScore = mc.calculateBasicRiskScore(metrics)

	// Add basic tags
	metrics.Tags = mc.generateBasicTags(symbol, metrics)

	return metrics
}

// calculateLinesOfCode counts the lines of code in an AST node
func (mc *BasicMetricsCalculator) calculateLinesOfCode(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}

	startLine := int(node.StartPosition().Row)
	endLine := int(node.EndPosition().Row)

	return endLine - startLine + 1
}

// calculateBasicComplexity calculates basic cyclomatic complexity
func (mc *BasicMetricsCalculator) calculateBasicComplexity(node *tree_sitter.Node) float64 {
	if node == nil {
		return 1.0
	}

	complexity := 1.0 // Base complexity

	// Count decision points in the AST
	complexity += mc.countDecisionPoints(node)

	return complexity
}

// countDecisionPoints recursively counts decision points in AST
func (mc *BasicMetricsCalculator) countDecisionPoints(node *tree_sitter.Node) float64 {
	if node == nil {
		return 0.0
	}

	count := 0.0
	nodeType := node.Kind()

	// Count decision points based on node type
	switch nodeType {
	case "if_statement", "else_clause":
		count += 1.0
	case "for_statement", "while_statement", "range_clause":
		count += 1.0
	case "switch_statement", "case_clause":
		count += 1.0
	case "conditional_expression", "logical_expression":
		count += 1.0
	case "try_statement", "catch_clause":
		count += 1.0
	}

	// Recursively count in children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		count += mc.countDecisionPoints(child)
	}

	return count
}

// calculateReferenceDensity calculates how well-connected a symbol is
func (mc *BasicMetricsCalculator) calculateReferenceDensity(symbol *types.EnhancedSymbol) float64 {
	if symbol == nil {
		return 0.0
	}

	incomingRefs := len(symbol.IncomingRefs)
	outgoingRefs := len(symbol.OutgoingRefs)

	// Normalize reference density (higher is more connected)
	totalRefs := float64(incomingRefs + outgoingRefs)
	if totalRefs == 0 {
		return 0.0
	}

	// Simple density calculation - can be enhanced in Phase 3B
	return totalRefs / 10.0 // Normalize to 0-1 range approximately
}

// calculateSizeScore calculates a quality score based on symbol size
func (mc *BasicMetricsCalculator) calculateSizeScore(linesOfCode int) float64 {
	// Ideal size ranges - these are configurable
	if linesOfCode <= 20 {
		return 1.0 // Optimal size
	} else if linesOfCode <= 50 {
		return 0.8 // Good size
	} else if linesOfCode <= 100 {
		return 0.6 // Acceptable size
	} else if linesOfCode <= 200 {
		return 0.4 // Large but manageable
	} else {
		return 0.2 // Too large
	}
}

// calculateBasicRiskScore calculates a simple risk score based on basic metrics
func (mc *BasicMetricsCalculator) calculateBasicRiskScore(metrics *BasicSymbolMetrics) float64 {
	risk := 0.0

	// High complexity increases risk
	if metrics.Complexity > 10 {
		risk += 0.3
	} else if metrics.Complexity > 5 {
		risk += 0.1
	}

	// Large size increases risk
	if metrics.LinesOfCode > 100 {
		risk += 0.3
	} else if metrics.LinesOfCode > 50 {
		risk += 0.1
	}

	// Low reference count may indicate unused code
	if metrics.ReferenceCount == 0 {
		risk += 0.2
	}

	// Very high reference count may indicate high coupling
	if metrics.ReferenceCount > 20 {
		risk += 0.2
	}

	// Cap risk score at 1.0
	if risk > 1.0 {
		risk = 1.0
	}

	return risk
}

// generateBasicTags generates basic tags based on metrics
func (mc *BasicMetricsCalculator) generateBasicTags(symbol *types.EnhancedSymbol, metrics *BasicSymbolMetrics) []string {
	var tags []string

	// Size-based tags
	if metrics.LinesOfCode > 100 {
		tags = append(tags, "large")
	} else if metrics.LinesOfCode < 10 {
		tags = append(tags, "small")
	}

	// Complexity-based tags
	if metrics.Complexity > 10 {
		tags = append(tags, "complex")
	} else if metrics.Complexity == 1 {
		tags = append(tags, "simple")
	}

	// Reference-based tags
	if metrics.ReferenceCount == 0 {
		tags = append(tags, "unused")
	} else if metrics.ReferenceCount > 10 {
		tags = append(tags, "highly-coupled")
	}

	// Risk-based tags
	if metrics.RiskScore > 0.7 {
		tags = append(tags, "high-risk")
	} else if metrics.RiskScore < 0.3 {
		tags = append(tags, "low-risk")
	}

	// Symbol type-based tags
	switch symbol.Type {
	case types.SymbolTypeFunction:
		tags = append(tags, "function")
	case types.SymbolTypeMethod:
		tags = append(tags, "method")
	case types.SymbolTypeClass:
		tags = append(tags, "class")
	case types.SymbolTypeInterface:
		tags = append(tags, "interface")
	}

	return tags
}

// ConvertToMainMetrics converts BasicSymbolMetrics to main branch SymbolMetrics format
func (basic *BasicSymbolMetrics) ConvertToMainMetrics() *SymbolMetrics {
	return &SymbolMetrics{
		Quality: CodeQualityMetrics{
			CyclomaticComplexity: int(basic.Complexity),
			LinesOfCode:          basic.LinesOfCode,
		},
		Dependencies: DependencyMetrics{
			IncomingDependencies: basic.ReferenceCount / 2, // Rough split
			OutgoingDependencies: basic.ReferenceCount / 2,
		},
		Tags:      basic.Tags,
		RiskScore: int(basic.RiskScore * 10), // Convert to 0-10 scale
	}
}
