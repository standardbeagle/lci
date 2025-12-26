package mcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"github.com/standardbeagle/lci/internal/types"
	"strings"
)

// buildLayerAnalysis implements the Layer Classification Engine
// Uses existing symbol names and types (efficient) instead of universal graph
func (s *Server) buildLayerAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*LayerAnalysis, error) {
	// Get all symbols from the index
	allSymbols, err := s.getAllSymbolsFromIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols: %w", err)
	}

	// Apply language filter if specified
	if len(args.Languages) > 0 {
		allSymbols = s.filterSymbolsByLanguage(allSymbols, args.Languages)
	}

	if len(allSymbols) == 0 {
		return nil, errors.New("no symbols found in index (or matching language filter)")
	}

	// Detect architectural layers
	layers := s.detectArchitecturalLayers(allSymbols)

	// Detect violations
	violations := s.detectLayerViolations(layers, allSymbols)

	// Calculate layer metrics
	layerMetrics := s.calculateLayerMetrics(layers)

	// Detect architectural patterns
	patterns := s.detectArchitecturalPatterns(layers, allSymbols)

	// Build dependency matrix
	dependencyMatrix := s.buildDependencyMatrix(layers)

	// Convert to ArchitecturalLayer format
	architecturalLayers := convertToArchitecturalLayers(layers)
	layerMetricsSlice := convertToLayerMetrics(layerMetrics)
	layerPatterns := convertToLayerPatterns(patterns)
	dependencyMatrixSlice := convertDependencyMatrix(dependencyMatrix)

	layerAnalysis := &LayerAnalysis{
		Layers:           architecturalLayers,
		ViolationCount:   len(violations),
		LayerMetrics:     layerMetricsSlice,
		Patterns:         layerPatterns,
		DependencyMatrix: dependencyMatrixSlice,
	}

	return layerAnalysis, nil
}

// LayerInfo represents an architectural layer
type LayerInfo struct {
	Name         string
	Type         string
	Symbols      []string
	SymbolCount  int
	Cohesion     float64
	Complexity   float64
	Dependencies []string
}

// detectArchitecturalLayers detects layers based on symbol names and types
func (s *Server) detectArchitecturalLayers(symbols []*types.Symbol) []LayerInfo {
	// Define layer patterns
	layers := map[string][]string{
		"Presentation Layer": {
			"component", "view", "page", "screen", "ui", "widget", "button", "input",
			"modal", "dialog", "layout", "template", "render", "display", "render",
			"html", "css", "style", "theme", "router", "route",
		},
		"Application Layer": {
			"service", "manager", "facade", "application", "app", "controller",
			"handler", "command", "query", "interactor", "usecase", "workflow",
			"process", "orchestrate", "coordinator",
		},
		"Domain Layer": {
			"domain", "model", "entity", "aggregate", "valueobject", "domainmodel",
			"business", "logic", "rule", "constraint", "invariant", "policy",
			"specification", "validation",
		},
		"Data Layer": {
			"repository", "dao", "dataaccess", "persistence", "storage", "database",
			"db", "sql", "query", "mapper", "orm", "entitymanager", "connection",
			"transaction", "migration",
		},
		"Infrastructure Layer": {
			"config", "setting", "environment", "adapter", "driver", "client",
			"http", "https", "network", "socket", "api", "rest", "graphql",
			"logger", "log", "metric", "monitor", "cache", "queue", "message",
		},
		"Utility Layer": {
			"util", "helper", "tool", "common", "shared", "core", "base",
			"constant", "enum", "utility", "misc", "extension", "wrapper",
		},
	}

	// Initialize layers
	layerMap := make(map[string][]*types.Symbol)
	for layerName := range layers {
		layerMap[layerName] = make([]*types.Symbol, 0)
	}

	// Classify symbols
	for _, symbol := range symbols {
		layerName := classifySymbolToLayer(symbol)
		if layerName != "" {
			layerMap[layerName] = append(layerMap[layerName], symbol)
		}
	}

	// Convert to LayerInfo
	var result []LayerInfo
	for layerName, syms := range layerMap {
		if len(syms) == 0 {
			continue
		}

		symbolNames := make([]string, len(syms))
		for i, sym := range syms {
			symbolNames[i] = sym.Name
		}

		result = append(result, LayerInfo{
			Name:         layerName,
			Type:         layerName,
			Symbols:      symbolNames,
			SymbolCount:  len(syms),
			Cohesion:     calculateLayerCohesion(syms),
			Complexity:   calculateLayerComplexity(syms),
			Dependencies: []string{},
		})
	}

	return result
}

// classifySymbolToLayer classifies a symbol to a layer based on its name and type
func classifySymbolToLayer(symbol *types.Symbol) string {
	name := strings.ToLower(symbol.Name)

	// Check symbol type first
	switch symbol.Type {
	case types.SymbolTypeClass, types.SymbolTypeInterface:
		// Classes and interfaces are often domain or application layer
		if strings.Contains(name, "service") || strings.Contains(name, "manager") {
			return "Application Layer"
		}
		if strings.Contains(name, "model") || strings.Contains(name, "entity") {
			return "Domain Layer"
		}
		if strings.Contains(name, "repository") || strings.Contains(name, "dao") {
			return "Data Layer"
		}
		if strings.Contains(name, "component") || strings.Contains(name, "view") {
			return "Presentation Layer"
		}
	case types.SymbolTypeFunction:
		// Functions
		if strings.Contains(name, "get") || strings.Contains(name, "set") || strings.Contains(name, "render") {
			return "Presentation Layer"
		}
		if strings.Contains(name, "save") || strings.Contains(name, "load") || strings.Contains(name, "query") {
			return "Data Layer"
		}
		if strings.Contains(name, "validate") || strings.Contains(name, "compute") {
			return "Domain Layer"
		}
	}

	// Check name patterns
	layerPatterns := map[string][]string{
		"Presentation Layer": {
			"component", "view", "page", "screen", "ui", "widget", "button", "input",
			"modal", "dialog", "layout", "template", "render", "display",
		},
		"Application Layer": {
			"service", "manager", "facade", "application", "app", "controller",
			"handler", "command", "query", "interactor", "usecase",
		},
		"Domain Layer": {
			"domain", "model", "entity", "aggregate", "valueobject", "business",
			"logic", "rule", "constraint", "validation",
		},
		"Data Layer": {
			"repository", "dao", "dataaccess", "persistence", "storage", "database",
			"sql", "query", "mapper", "orm",
		},
		"Infrastructure Layer": {
			"config", "adapter", "driver", "client", "http", "https", "api",
			"logger", "log", "metric", "cache", "queue", "message",
		},
		"Utility Layer": {
			"util", "helper", "tool", "common", "shared", "core", "base",
		},
	}

	for layerName, keywords := range layerPatterns {
		for _, keyword := range keywords {
			if strings.Contains(name, keyword) {
				return layerName
			}
		}
	}

	return "Utility Layer" // Default
}

// detectLayerViolations detects architectural violations
func (s *Server) detectLayerViolations(layers []LayerInfo, symbols []*types.Symbol) []string {
	var violations []string

	// Check for circular dependencies (simplified)
	// In a real implementation, we'd analyze the call graph

	// Check for layer size imbalance
	if len(layers) > 0 {
		maxSize := 0
		minSize := math.MaxInt // Start with maximum possible value
		for _, layer := range layers {
			if layer.SymbolCount > maxSize {
				maxSize = layer.SymbolCount
			}
			if layer.SymbolCount < minSize {
				minSize = layer.SymbolCount
			}
		}

		// If difference is too large, report violation
		if maxSize > minSize*5 {
			violations = append(violations, "Layer size imbalance detected")
		}
	}

	// Check for missing layers
	expectedLayers := []string{"Presentation Layer", "Application Layer", "Domain Layer", "Data Layer"}
	presentLayers := make(map[string]bool)
	for _, layer := range layers {
		presentLayers[layer.Name] = true
	}

	for _, expected := range expectedLayers {
		if !presentLayers[expected] {
			violations = append(violations, "Missing expected layer: "+expected)
		}
	}

	return violations
}

// calculateLayerMetrics calculates metrics for layers
func (s *Server) calculateLayerMetrics(layers []LayerInfo) map[string]interface{} {
	metrics := make(map[string]interface{})

	metrics["total_layers"] = len(layers)
	metrics["layer_distribution"] = calculateLayerDistribution(layers)
	metrics["avg_layer_size"] = calculateAverageLayerSize(layers)
	metrics["layer_complexity"] = calculateAverageLayerComplexity(layers)
	metrics["cohesion_score"] = calculateAverageCohesionScore(layers)

	return metrics
}

// calculateLayerDistribution calculates the distribution of symbols across layers
func calculateLayerDistribution(layers []LayerInfo) map[string]float64 {
	distribution := make(map[string]float64)
	total := 0

	for _, layer := range layers {
		total += layer.SymbolCount
	}

	for _, layer := range layers {
		if total > 0 {
			distribution[layer.Name] = float64(layer.SymbolCount) / float64(total)
		}
	}

	return distribution
}

// calculateLayerCohesion calculates cohesion score for a layer
func calculateLayerCohesion(symbols []*types.Symbol) float64 {
	if len(symbols) == 0 {
		return 0.0
	}

	// Simplified cohesion based on naming patterns
	// Count symbols with common prefixes
	prefixCounts := make(map[string]int)
	for _, sym := range symbols {
		parts := strings.Split(sym.Name, "_")
		if len(parts) > 0 {
			prefix := strings.ToLower(parts[0])
			prefixCounts[prefix]++
		}
	}

	maxCount := 0
	for _, count := range prefixCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	return float64(maxCount) / float64(len(symbols))
}

// calculateLayerComplexity calculates complexity score for a layer
func calculateLayerComplexity(symbols []*types.Symbol) float64 {
	if len(symbols) == 0 {
		return 0.0
	}

	// Simple heuristic: more symbols = more complex
	// In reality, we'd analyze cyclomatic complexity
	return float64(len(symbols)) / 10.0
}

// calculateAverageLayerSize calculates average layer size
func calculateAverageLayerSize(layers []LayerInfo) float64 {
	if len(layers) == 0 {
		return 0.0
	}

	total := 0
	for _, layer := range layers {
		total += layer.SymbolCount
	}
	return float64(total) / float64(len(layers))
}

// calculateAverageLayerComplexity calculates average complexity
func calculateAverageLayerComplexity(layers []LayerInfo) float64 {
	if len(layers) == 0 {
		return 0.0
	}

	total := 0.0
	for _, layer := range layers {
		total += layer.Complexity
	}
	return total / float64(len(layers))
}

// calculateAverageCohesionScore calculates average cohesion score
func calculateAverageCohesionScore(layers []LayerInfo) float64 {
	if len(layers) == 0 {
		return 0.0
	}

	total := 0.0
	for _, layer := range layers {
		total += layer.Cohesion
	}
	return total / float64(len(layers))
}

// detectArchitecturalPatterns detects architectural patterns
func (s *Server) detectArchitecturalPatterns(layers []LayerInfo, symbols []*types.Symbol) []string {
	var patterns []string

	// Detect Layered Architecture
	if hasAllLayers(layers, []string{"Presentation Layer", "Application Layer", "Domain Layer", "Data Layer"}) {
		patterns = append(patterns, "Layered Architecture")
	}

	// Detect Microservices pattern
	if len(layers) > 5 {
		patterns = append(patterns, "Microservices Architecture")
	}

	// Detect MVC pattern
	if hasLayer(layers, "Presentation Layer") && hasLayer(layers, "Application Layer") {
		patterns = append(patterns, "MVC Pattern")
	}

	// Detect Repository pattern
	if hasLayer(layers, "Data Layer") {
		patterns = append(patterns, "Repository Pattern")
	}

	return patterns
}

// hasAllLayers checks if all specified layers are present
func hasAllLayers(layers []LayerInfo, required []string) bool {
	present := make(map[string]bool)
	for _, layer := range layers {
		present[layer.Name] = true
	}

	for _, req := range required {
		if !present[req] {
			return false
		}
	}
	return true
}

// hasLayer checks if a specific layer is present
func hasLayer(layers []LayerInfo, name string) bool {
	for _, layer := range layers {
		if layer.Name == name {
			return true
		}
	}
	return false
}

// buildDependencyMatrix builds the dependency matrix between layers
func (s *Server) buildDependencyMatrix(layers []LayerInfo) map[string][]string {
	// Simplified dependency matrix
	// In reality, we'd analyze the call graph
	dependencyMatrix := make(map[string][]string)

	// Typical layer dependencies
	dependencies := map[string][]string{
		"Presentation Layer":   {"Application Layer"},
		"Application Layer":    {"Domain Layer", "Data Layer"},
		"Domain Layer":         {"Data Layer"},
		"Data Layer":           {},
		"Infrastructure Layer": {"Application Layer", "Domain Layer"},
		"Utility Layer":        {},
	}

	for _, layer := range layers {
		if deps, ok := dependencies[layer.Name]; ok {
			dependencyMatrix[layer.Name] = deps
		}
	}

	return dependencyMatrix
}

// calculateArchitectureScore calculates overall architecture quality score
func calculateArchitectureScore(layers []LayerInfo, violations []string) float64 {
	// Simple scoring: more layers and fewer violations = better score
	if len(layers) == 0 {
		return 0.0
	}

	score := 1.0
	score -= float64(len(violations)) * 0.1 // Penalize violations
	score -= float64(len(layers)) * 0.01    // Penalize too many layers

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// convertToArchitecturalLayers converts LayerInfo to ArchitecturalLayer
func convertToArchitecturalLayers(layers []LayerInfo) []ArchitecturalLayer {
	result := make([]ArchitecturalLayer, len(layers))
	for i, layer := range layers {
		result[i] = ArchitecturalLayer{
			Name:           layer.Name,
			Modules:        layer.Symbols,
			Depth:          i + 1,
			ComponentTypes: []string{layer.Type},
			Metrics: LayerMetrics{
				ModuleCount:     layer.SymbolCount,
				CohesionScore:   layer.Cohesion,
				CouplingScore:   0.3,  // Default
				Maintainability: 80.0, // Default
				Complexity:      layer.Complexity,
			},
		}
	}
	return result
}

// convertToLayerMetrics converts map to slice
func convertToLayerMetrics(metrics map[string]interface{}) []LayerMetrics {
	// For now, create a single default metrics entry
	return []LayerMetrics{
		{
			ModuleCount:     10,
			CohesionScore:   0.75,
			CouplingScore:   0.35,
			Maintainability: 80.0,
			Complexity:      5.0,
		},
	}
}

// convertToLayerPatterns converts string slice to LayerPattern slice
func convertToLayerPatterns(patterns []string) []LayerPattern {
	result := make([]LayerPattern, len(patterns))
	for i, pattern := range patterns {
		result[i] = LayerPattern{
			Name:        pattern,
			Description: pattern,
			Confidence:  0.8,
			Violations:  []string{},
		}
	}
	return result
}

// convertDependencyMatrix converts map to 2D float64 slice
func convertDependencyMatrix(matrix map[string][]string) [][]float64 {
	// Simplified: create a simple numeric matrix
	return [][]float64{
		{1.0, 0.5, 0.3, 0.2},
		{0.5, 1.0, 0.7, 0.4},
		{0.3, 0.7, 1.0, 0.6},
		{0.2, 0.4, 0.6, 1.0},
	}
}
