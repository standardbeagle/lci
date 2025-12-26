package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"github.com/standardbeagle/lci/internal/types"
	"strings"
)

// buildFeatureAnalysis implements the Feature Location Engine
// Uses existing symbol names and directory structure (efficient) instead of universal graph
func (s *Server) buildFeatureAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*FeatureAnalysis, error) {
	// Get all symbols from the index
	allSymbols, err := s.getAllSymbolsFromIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols: %w", err)
	}

	if len(allSymbols) == 0 {
		return nil, errors.New("no symbols found in index")
	}

	// Detect features using symbol names and file paths
	features := s.detectFeatures(allSymbols)

	// Convert to FeatureAnalysis format
	featureList := make([]Feature, 0)
	for _, feat := range features {
		components := make([]ComponentInfo, 0)
		for _, comp := range feat.Components {
			components = append(components, ComponentInfo{
				Name:         comp.Name,
				Type:         comp.Type,
				Location:     comp.Location,
				Complexity:   comp.Complexity,
				Dependencies: comp.Dependencies,
			})
		}

		featureList = append(featureList, Feature{
			Name:          feat.Name,
			PrimaryModule: feat.Type, // Use Type as PrimaryModule for now
			Components:    components,
			APIs:          []APIEndpoint{},
			Tests:         []TestInfo{},
			Confidence:    0.8, // Default confidence
		})
	}

	// Calculate feature metrics
	featureMetrics := s.calculateFeatureMetrics(features)

	featureAnalysis := &FeatureAnalysis{
		Features:         featureList,
		FeatureMap:       make(map[string]string),
		CrossFeatureDeps: []FeatureDependency{},
		OrphanComponents: []ComponentInfo{},
		Metrics:          featureMetrics,
	}

	return featureAnalysis, nil
}

// FeatureInfo represents a detected feature
type FeatureInfo struct {
	Name         string
	Type         string
	Components   []FeatureComponentInfo
	Cohesion     float64
	Complexity   float64
	Completeness float64
}

// FeatureComponentInfo represents a component within a feature
type FeatureComponentInfo struct {
	Name         string
	Type         string
	Location     string
	Complexity   float64
	Dependencies []string
}

// detectFeatures detects features based on symbol names and directory structure
func (s *Server) detectFeatures(symbols []*types.Symbol) []FeatureInfo {
	// Group symbols by feature patterns
	featureGroups := make(map[string][]*types.Symbol)

	// Common feature patterns
	featurePatterns := []string{
		"user", "auth", "login", "register", "account", "profile",
		"order", "cart", "checkout", "payment", "billing", "invoice",
		"product", "catalog", "inventory", "stock", "price",
		"notification", "email", "sms", "push", "alert",
		"report", "analytics", "dashboard", "metric", "statistic",
		"search", "filter", "query", "sort", "pagination",
		"upload", "download", "file", "image", "document",
		"config", "setting", "preference", "option",
		"api", "endpoint", "service", "controller",
		"database", "db", "sql", "cache", "session",
	}

	// Group symbols by feature
	for _, symbol := range symbols {
		symbolName := strings.ToLower(symbol.Name)
		filePath := s.goroutineIndex.GetFilePath(symbol.FileID)

		// Find matching feature
		matchedFeature := ""
		for _, pattern := range featurePatterns {
			if strings.Contains(symbolName, pattern) {
				matchedFeature = pattern
				break
			}
		}

		// If no match, try directory-based feature detection
		if matchedFeature == "" {
			dir := filepath.Dir(filePath)
			dirBase := strings.ToLower(filepath.Base(dir))
			for _, pattern := range featurePatterns {
				if strings.Contains(dirBase, pattern) {
					matchedFeature = pattern
					break
				}
			}
		}

		// Default to "general" if no pattern matches
		if matchedFeature == "" {
			matchedFeature = "general"
		}

		featureGroups[matchedFeature] = append(featureGroups[matchedFeature], symbol)
	}

	// Convert to FeatureInfo
	features := make([]FeatureInfo, 0)
	for featureName, syms := range featureGroups {
		if len(syms) == 0 {
			continue
		}

		// Create components from symbols
		components := s.createComponentsFromSymbols(syms)

		// Calculate feature metrics
		cohesion := calculateFeatureCohesion(syms)
		complexity := calculateFeatureComplexity(syms)
		completeness := calculateFeatureCompleteness(syms, featureName)

		// Classify feature type
		featureType := classifyFeatureType(featureName)

		features = append(features, FeatureInfo{
			Name:         featureName,
			Type:         featureType,
			Components:   components,
			Cohesion:     cohesion,
			Complexity:   complexity,
			Completeness: completeness,
		})
	}

	return features
}

// createComponentsFromSymbols creates components from symbols
func (s *Server) createComponentsFromSymbols(symbols []*types.Symbol) []FeatureComponentInfo {
	components := make([]FeatureComponentInfo, 0)

	for _, symbol := range symbols {
		filePath := s.goroutineIndex.GetFilePath(symbol.FileID)

		componentType := classifyComponentType(symbol)
		complexity := calculateComponentComplexity(symbol)

		components = append(components, FeatureComponentInfo{
			Name:         symbol.Name,
			Type:         componentType,
			Location:     filePath,
			Complexity:   complexity,
			Dependencies: []string{}, // Would use call graph in real implementation
		})
	}

	return components
}

// classifyComponentType classifies a component based on symbol type and name
func classifyComponentType(symbol *types.Symbol) string {
	name := strings.ToLower(symbol.Name)

	switch symbol.Type {
	case types.SymbolTypeFunction:
		if strings.Contains(name, "handler") || strings.Contains(name, "controller") {
			return "Controller"
		}
		if strings.Contains(name, "service") || strings.Contains(name, "manager") {
			return "Service"
		}
		if strings.Contains(name, "repository") || strings.Contains(name, "dao") {
			return "Repository"
		}
		if strings.Contains(name, "model") || strings.Contains(name, "entity") {
			return "Model"
		}
		if strings.Contains(name, "util") || strings.Contains(name, "helper") {
			return "Utility"
		}
		return "Function"
	case types.SymbolTypeClass:
		if strings.Contains(name, "service") {
			return "Service"
		}
		if strings.Contains(name, "controller") {
			return "Controller"
		}
		if strings.Contains(name, "model") {
			return "Model"
		}
		if strings.Contains(name, "repository") {
			return "Repository"
		}
		return "Class"
	case types.SymbolTypeInterface:
		return "Interface"
	case types.SymbolTypeVariable:
		return "Variable"
	case types.SymbolTypeConstant:
		return "Constant"
	}

	return "Symbol"
}

// calculateComponentComplexity calculates complexity of a component
func calculateComponentComplexity(symbol *types.Symbol) float64 {
	// Simple heuristic based on symbol name length
	// In reality, we'd analyze cyclomatic complexity
	return float64(len(symbol.Name)) / 10.0
}

// calculateFeatureCohesion calculates feature cohesion score
func calculateFeatureCohesion(symbols []*types.Symbol) float64 {
	if len(symbols) == 0 {
		return 0.0
	}

	// Cohesion based on common prefixes in feature
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

// calculateFeatureComplexity calculates feature complexity
func calculateFeatureComplexity(symbols []*types.Symbol) float64 {
	if len(symbols) == 0 {
		return 0.0
	}

	// Simple complexity: more symbols = more complex
	return float64(len(symbols)) / 10.0
}

// calculateFeatureCompleteness calculates feature completeness
func calculateFeatureCompleteness(symbols []*types.Symbol, featureName string) float64 {
	if len(symbols) == 0 {
		return 0.0
	}

	// Completeness based on having all required component types
	requiredTypes := []string{"Controller", "Service", "Repository", "Model"}
	presentTypes := make(map[string]bool)

	for _, sym := range symbols {
		compType := classifyComponentType(sym)
		presentTypes[compType] = true
	}

	matches := 0
	for _, reqType := range requiredTypes {
		if presentTypes[reqType] {
			matches++
		}
	}

	return float64(matches) / float64(len(requiredTypes))
}

// classifyFeatureType classifies a feature by its name
func classifyFeatureType(featureName string) string {
	lower := strings.ToLower(featureName)

	if containsAny(lower, []string{"user", "auth", "login", "register", "account", "profile"}) {
		return "User Management"
	}
	if containsAny(lower, []string{"order", "cart", "checkout", "payment", "billing", "invoice"}) {
		return "E-commerce"
	}
	if containsAny(lower, []string{"product", "catalog", "inventory", "stock", "price"}) {
		return "Product Management"
	}
	if containsAny(lower, []string{"notification", "email", "sms", "push", "alert"}) {
		return "Communication"
	}
	if containsAny(lower, []string{"report", "analytics", "dashboard", "metric", "statistic"}) {
		return "Reporting"
	}
	if containsAny(lower, []string{"search", "filter", "query", "sort"}) {
		return "Search"
	}
	if containsAny(lower, []string{"upload", "download", "file", "image", "document"}) {
		return "File Management"
	}
	if containsAny(lower, []string{"config", "setting", "preference", "option"}) {
		return "Configuration"
	}
	if containsAny(lower, []string{"api", "endpoint", "service", "controller"}) {
		return "API"
	}
	if containsAny(lower, []string{"database", "db", "sql", "cache", "session"}) {
		return "Data Management"
	}

	return "General Feature"
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// calculateFeatureMetrics calculates metrics for features
func (s *Server) calculateFeatureMetrics(features []FeatureInfo) FeatureAnalysisMetrics {
	metrics := FeatureAnalysisMetrics{
		TotalFeatures:     len(features),
		AverageComponents: calculateAverageFeatureSize(features),
		CouplingScore:     calculateAverageCohesion(features),
		ModularityScore:   calculateAverageComplexity(features),
	}

	return metrics
}

// calculateAverageFeatureSize calculates average feature size
func calculateAverageFeatureSize(features []FeatureInfo) float64 {
	if len(features) == 0 {
		return 0.0
	}

	total := 0
	for _, f := range features {
		total += len(f.Components)
	}
	return float64(total) / float64(len(features))
}

// calculateAverageCohesion calculates average cohesion
func calculateAverageCohesion(features []FeatureInfo) float64 {
	if len(features) == 0 {
		return 0.0
	}

	total := 0.0
	for _, f := range features {
		total += f.Cohesion
	}
	return total / float64(len(features))
}

// calculateAverageComplexity calculates average complexity
func calculateAverageComplexity(features []FeatureInfo) float64 {
	if len(features) == 0 {
		return 0.0
	}

	total := 0.0
	for _, f := range features {
		total += f.Complexity
	}
	return total / float64(len(features))
}

// calculateFeatureDistribution calculates feature distribution
func calculateFeatureDistribution(features []FeatureInfo) map[string]int {
	distribution := make(map[string]int)

	for _, f := range features {
		distribution[f.Type]++
	}

	return distribution
}

// analyzeFeatureArchitecture analyzes feature architecture
func analyzeFeatureArchitecture(features []FeatureInfo) string {
	if len(features) == 0 {
		return "No features detected"
	}

	// Count feature types
	typeCounts := make(map[string]int)
	for _, f := range features {
		typeCounts[f.Type]++
	}

	// Find dominant type
	maxCount := 0
	dominantType := ""
	for ftype, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			dominantType = ftype
		}
	}

	return fmt.Sprintf("Dominant architecture: %s (%d features)", dominantType, maxCount)
}
