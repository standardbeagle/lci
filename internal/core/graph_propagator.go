package core

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// GraphPropagator implements multi-mode label and dependency propagation
// through the symbol call graph supporting reachability, accumulation, decay, and max modes
type GraphPropagator struct {
	// Core graph structure
	// callGraph removed - RefTracker handles call relationships
	annotator *SemanticAnnotator

	// Symbol resolution infrastructure
	refTracker  *ReferenceTracker
	symbolIndex *SymbolIndex

	// Propagation configuration
	config *PropagationConfig

	// Propagation state
	propagationState map[PropagationKey]*PropagationValue
	iterationCount   int
	converged        bool

	// Results cache
	propagatedLabels map[types.SymbolID][]PropagatedLabel
	propagatedDeps   map[types.SymbolID][]PropagatedDependency

	// Analysis results
	entryPoints     map[string][]*AnnotatedSymbol // Entry points by label
	dependencyDepth map[types.SymbolID]int        // Max dependency depth per symbol
	criticalPaths   []CriticalPath                // High-impact propagation paths
}

// PropagationMode defines how properties propagate through the call graph
type PropagationMode string

const (
	// ModeReachability: Binary reachability - all reachable symbols get full strength (1.0)
	// Use for: Bug propagation, security vulnerabilities, dependency existence
	// Semantics: If function A calls function B with @lci:critical, A is also critical (not 0.7× critical)
	ModeReachability PropagationMode = "reachability"

	// ModeAccumulation: Strength accumulates upward through call graph
	// Use for: Performance costs, database call counts, resource usage
	// Semantics: If A calls B (1 DB call) and C (2 DB calls), A has 3 DB calls total
	ModeAccumulation PropagationMode = "accumulation"

	// ModeDecay: Strength decays per hop (PageRank-style)
	// Use for: UI relevance ranking, attention priority for search results
	// Semantics: Direct callers 100% relevant, 2-hop 80% relevant, 5-hop 33% relevant
	// WARNING: This is primarily a UI heuristic, not a semantic property of code
	ModeDecay PropagationMode = "decay"

	// ModeMax: Take maximum strength along any path
	// Use for: Risk assessment, criticality levels, priority propagation
	// Semantics: If A calls B (priority=3) and C (priority=5), A gets priority=5
	ModeMax PropagationMode = "max"
)

// PropagationConfig defines rules for how attributes propagate through the graph
type PropagationConfig struct {
	// Global settings
	MaxIterations        int     `json:"max_iterations"`        // Maximum propagation iterations
	ConvergenceThreshold float64 `json:"convergence_threshold"` // Convergence threshold
	DefaultDecay         float64 `json:"default_decay"`         // Default decay rate (only used in ModeDecay)

	// Label propagation rules
	LabelRules []LabelPropagationRule `json:"label_rules"`

	// Dependency propagation rules
	DependencyRules []DependencyPropagationRule `json:"dependency_rules"`

	// Custom propagation functions
	CustomRules []CustomPropagationRule `json:"custom_rules"`

	// Analysis configuration
	AnalysisConfig AnalysisConfig `json:"analysis_config"`
}

// LabelPropagationRule defines how specific labels propagate
type LabelPropagationRule struct {
	Label                string          `json:"label"`                  // Label to propagate
	Direction            string          `json:"direction"`              // upstream, downstream, bidirectional
	Mode                 PropagationMode `json:"mode"`                   // Propagation mode (reachability, accumulation, decay, max)
	Decay                float64         `json:"decay"`                  // Strength reduction per hop (only used in ModeDecay)
	MaxHops              int             `json:"max_hops"`               // Maximum propagation distance (0 = unlimited)
	MinStrength          float64         `json:"min_strength"`           // Minimum strength to continue (only used in ModeDecay)
	Boost                float64         `json:"boost"`                  // Strength boost for certain conditions
	Conditions           []string        `json:"conditions"`             // Conditional propagation rules
	Priority             int             `json:"priority"`               // Rule priority (higher = more important)
	IncludeTypeHierarchy bool            `json:"include_type_hierarchy"` // Include implements/extends relationships in propagation
}

// DependencyPropagationRule defines how dependencies aggregate
type DependencyPropagationRule struct {
	DependencyType string  `json:"dependency_type"` // Type of dependency to track
	Direction      string  `json:"direction"`       // upstream, downstream
	Aggregation    string  `json:"aggregation"`     // sum, max, unique, concat, weighted_sum
	WeightFunction string  `json:"weight_function"` // linear, exponential, log
	MaxDepth       int     `json:"max_depth"`       // Maximum aggregation depth
	Threshold      float64 `json:"threshold"`       // Minimum threshold for inclusion
}

// CustomPropagationRule allows custom propagation logic
type CustomPropagationRule struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Trigger     string                 `json:"trigger"` // Trigger condition
	Action      string                 `json:"action"`  // Action to take
	Parameters  map[string]interface{} `json:"parameters"`
	Priority    int                    `json:"priority"`
}

// AnalysisConfig controls high-level analysis features
type AnalysisConfig struct {
	DetectEntryPoints   bool     `json:"detect_entry_points"`
	CalculateDepth      bool     `json:"calculate_depth"`
	FindCriticalPaths   bool     `json:"find_critical_paths"`
	EntryPointLabels    []string `json:"entry_point_labels"`    // Labels that indicate entry points
	CriticalLabels      []string `json:"critical_labels"`       // Labels that indicate critical functionality
	HighImpactThreshold float64  `json:"high_impact_threshold"` // Threshold for high-impact classification
}

// PropagationKey uniquely identifies a propagation value
type PropagationKey struct {
	SymbolID  types.SymbolID `json:"symbol_id"`
	Attribute string         `json:"attribute"` // Label name or dependency type
	Type      string         `json:"type"`      // "label" or "dependency"
}

// PropagationValue stores the propagated value and metadata
type PropagationValue struct {
	Strength    float64                `json:"strength"`     // Current propagation strength
	Source      types.SymbolID         `json:"source"`       // Original source symbol
	Hops        int                    `json:"hops"`         // Number of hops from source
	Path        []types.SymbolID       `json:"path"`         // Propagation path
	Metadata    map[string]interface{} `json:"metadata"`     // Additional metadata
	LastUpdated int                    `json:"last_updated"` // Iteration when last updated
}

// PropagatedLabel represents a label that has been propagated to a symbol
type PropagatedLabel struct {
	Label      string                 `json:"label"`
	Strength   float64                `json:"strength"`
	Source     types.SymbolID         `json:"source"`
	Path       []types.SymbolID       `json:"path"`
	Hops       int                    `json:"hops"`
	Metadata   map[string]interface{} `json:"metadata"`
	Confidence float64                `json:"confidence"`
}

// PropagatedDependency represents an aggregated dependency
type PropagatedDependency struct {
	Type     string                 `json:"type"`
	Count    int                    `json:"count"`
	Sources  []types.SymbolID       `json:"sources"`
	Depth    int                    `json:"depth"`
	Weight   float64                `json:"weight"`
	Details  []DependencyDetail     `json:"details"`
	Metadata map[string]interface{} `json:"metadata"`
}

// DependencyDetail provides detailed information about a specific dependency instance
type DependencyDetail struct {
	Name     string           `json:"name"`
	Source   types.SymbolID   `json:"source"`
	Path     []types.SymbolID `json:"path"`
	Mode     string           `json:"mode"`
	Weight   float64          `json:"weight"`
	Distance int              `json:"distance"`
}

// CriticalPath represents a high-impact propagation path
type CriticalPath struct {
	Path        []types.SymbolID       `json:"path"`
	Labels      []string               `json:"labels"`
	TotalImpact float64                `json:"total_impact"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewGraphPropagator creates a new graph propagation engine
func NewGraphPropagator(annotator *SemanticAnnotator, refTracker *ReferenceTracker, symbolIndex *SymbolIndex) *GraphPropagator {
	return &GraphPropagator{
		annotator:        annotator,
		refTracker:       refTracker,
		symbolIndex:      symbolIndex,
		config:           getDefaultPropagationConfig(),
		propagationState: make(map[PropagationKey]*PropagationValue),
		propagatedLabels: make(map[types.SymbolID][]PropagatedLabel),
		propagatedDeps:   make(map[types.SymbolID][]PropagatedDependency),
		entryPoints:      make(map[string][]*AnnotatedSymbol),
		dependencyDepth:  make(map[types.SymbolID]int),
		criticalPaths:    make([]CriticalPath, 0),
	}
}

// SetConfig updates the propagation configuration
func (gp *GraphPropagator) SetConfig(config *PropagationConfig) {
	gp.config = config
}

// PropagateAll runs the complete propagation algorithm
func (gp *GraphPropagator) PropagateAll() error {
	// Initialize propagation state from annotations
	if err := gp.initializePropagationState(); err != nil {
		return fmt.Errorf("failed to initialize propagation state: %w", err)
	}

	// Run iterative propagation until convergence
	for gp.iterationCount = 0; gp.iterationCount < gp.config.MaxIterations && !gp.converged; gp.iterationCount++ {
		if err := gp.runPropagationIteration(); err != nil {
			return fmt.Errorf("propagation iteration %d failed: %w", gp.iterationCount, err)
		}

		// Check for convergence
		gp.checkConvergence()
	}

	// Build final results
	if err := gp.buildResults(); err != nil {
		return fmt.Errorf("failed to build propagation results: %w", err)
	}

	// Run analysis
	if err := gp.runAnalysis(); err != nil {
		return fmt.Errorf("failed to run propagation analysis: %w", err)
	}

	return nil
}

// initializePropagationState seeds the propagation with direct annotations
func (gp *GraphPropagator) initializePropagationState() error {
	// Get all annotated symbols
	stats := gp.annotator.GetAnnotationStats()
	labelDistribution, ok := stats["label_distribution"].(map[string]int)
	if !ok {
		return errors.New("invalid annotation stats format")
	}

	// Initialize from direct annotations
	for label := range labelDistribution {
		symbols := gp.annotator.GetSymbolsByLabel(label)
		for _, symbol := range symbols {
			key := PropagationKey{
				SymbolID:  symbol.SymbolID,
				Attribute: label,
				Type:      "label",
			}

			// Determine initial strength - check if annotation has numeric tag value
			strength := 1.0 // Default full strength
			if symbol.Annotation != nil && symbol.Annotation.Tags != nil {
				// Look for numeric tags like "priority", "level", "weight", etc.
				for tagKey, tagValue := range symbol.Annotation.Tags {
					if tagKey == "priority" || tagKey == "level" || tagKey == "weight" || tagKey == "value" {
						if numValue, err := strconv.ParseFloat(tagValue, 64); err == nil {
							strength = numValue
							break
						}
					}
				}
			}

			gp.propagationState[key] = &PropagationValue{
				Strength:    strength,
				Source:      symbol.SymbolID,
				Hops:        0,
				Path:        []types.SymbolID{symbol.SymbolID},
				Metadata:    make(map[string]interface{}),
				LastUpdated: 0,
			}
		}
	}

	// Initialize dependency propagation
	depGraph := gp.annotator.GetDependencyGraph()
	for symbolID, deps := range depGraph {
		for _, dep := range deps {
			key := PropagationKey{
				SymbolID:  symbolID,
				Attribute: dep.Type,
				Type:      "dependency",
			}

			gp.propagationState[key] = &PropagationValue{
				Strength: 1.0,
				Source:   symbolID,
				Hops:     0,
				Path:     []types.SymbolID{symbolID},
				Metadata: map[string]interface{}{
					"dependency_name": dep.Name,
					"dependency_mode": dep.Mode,
				},
				LastUpdated: 0,
			}
		}
	}

	return nil
}

// runPropagationIteration executes one iteration of the propagation algorithm
func (gp *GraphPropagator) runPropagationIteration() error {
	newState := make(map[PropagationKey]*PropagationValue)

	// Copy existing state
	for key, value := range gp.propagationState {
		newState[key] = &PropagationValue{
			Strength:    value.Strength,
			Source:      value.Source,
			Hops:        value.Hops,
			Path:        append([]types.SymbolID{}, value.Path...),
			Metadata:    value.Metadata,
			LastUpdated: value.LastUpdated,
		}
	}

	// Propagate labels
	for _, rule := range gp.config.LabelRules {
		if err := gp.propagateLabel(rule, newState); err != nil {
			return fmt.Errorf("failed to propagate label %s: %w", rule.Label, err)
		}
	}

	// Propagate dependencies
	for _, rule := range gp.config.DependencyRules {
		if err := gp.propagateDependency(rule, newState); err != nil {
			return fmt.Errorf("failed to propagate dependency %s: %w", rule.DependencyType, err)
		}
	}

	// Apply custom rules
	for _, rule := range gp.config.CustomRules {
		if err := gp.applyCustomRule(rule, newState); err != nil {
			return fmt.Errorf("failed to apply custom rule %s: %w", rule.Name, err)
		}
	}

	// Update state
	gp.propagationState = newState
	return nil
}

// propagateLabel propagates a specific label according to its rule and mode
// When IncludeTypeHierarchy is enabled, propagation follows both call relationships
// and type hierarchy (implements/extends) relationships.
func (gp *GraphPropagator) propagateLabel(rule LabelPropagationRule, newState map[PropagationKey]*PropagationValue) error {
	// For ModeAccumulation, we need to process all symbols at once to sum correctly
	if rule.Mode == ModeAccumulation {
		return gp.propagateLabelAccumulation(rule, newState)
	}

	// For other modes, use standard iterative propagation
	// Find all current instances of this label
	for key, value := range gp.propagationState {
		if key.Type == "label" && key.Attribute == rule.Label {
			// Skip if we've reached max hops (0 = unlimited)
			if rule.MaxHops > 0 && value.Hops >= rule.MaxHops {
				continue
			}

			// Skip if strength is too low (only relevant for ModeDecay)
			if rule.Mode == ModeDecay && value.Strength < rule.MinStrength {
				continue
			}

			// Get connected symbols based on direction and type hierarchy setting
			// When IncludeTypeHierarchy is true, also follow implements/extends relationships
			targets := gp.getConnectedSymbols(key.SymbolID, rule.Direction, rule.IncludeTypeHierarchy)

			// Propagate to targets
			for _, targetID := range targets {
				targetKey := PropagationKey{
					SymbolID:  targetID,
					Attribute: rule.Label,
					Type:      "label",
				}

				// Calculate new strength based on propagation mode
				newStrength := gp.calculatePropagatedStrength(rule, value, targetID)

				// Determine if we should update based on mode
				shouldUpdate := false
				if existing, exists := newState[targetKey]; exists {
					switch rule.Mode {
					case ModeReachability:
						// Already reachable, no need to update
						shouldUpdate = false
					case ModeDecay:
						// Keep stronger value
						shouldUpdate = newStrength > existing.Strength
					case ModeMax:
						// Keep maximum
						shouldUpdate = newStrength > existing.Strength
					}
				} else {
					shouldUpdate = true
				}

				if shouldUpdate {
					newPath := append(value.Path, targetID)
					newState[targetKey] = &PropagationValue{
						Strength:    newStrength,
						Source:      value.Source,
						Hops:        value.Hops + 1,
						Path:        newPath,
						Metadata:    value.Metadata,
						LastUpdated: gp.iterationCount,
					}
				}
			}
		}
	}

	return nil
}

// propagateLabelAccumulation handles accumulation mode separately to sum all incoming values correctly
// When IncludeTypeHierarchy is enabled, also considers implements/extends relationships.
func (gp *GraphPropagator) propagateLabelAccumulation(rule LabelPropagationRule, newState map[PropagationKey]*PropagationValue) error {
	// For each symbol that could receive accumulated values
	processed := make(map[types.SymbolID]bool)

	for key, value := range gp.propagationState {
		if key.Type == "label" && key.Attribute == rule.Label {
			// Skip if we've reached max hops
			if rule.MaxHops > 0 && value.Hops >= rule.MaxHops {
				continue
			}

			// Get targets based on direction and type hierarchy setting
			targets := gp.getConnectedSymbols(key.SymbolID, rule.Direction, rule.IncludeTypeHierarchy)

			// For each target, sum up all incoming values
			for _, targetID := range targets {
				if processed[targetID] {
					continue // Already calculated sum for this symbol
				}

				targetKey := PropagationKey{
					SymbolID:  targetID,
					Attribute: rule.Label,
					Type:      "label",
				}

				// Calculate sum of all incoming values for this target
				totalStrength := 0.0
				maxHops := 0
				var sources []types.SymbolID

				// Get all related symbols in the reverse direction
				// For upstream propagation, sum from all downstream symbols (callees, implementors, derived)
				// For downstream propagation, sum from all upstream symbols (callers, interfaces, base)
				var reverseDirection string
				switch rule.Direction {
				case "upstream":
					reverseDirection = "downstream"
				case "downstream":
					reverseDirection = "upstream"
				default:
					reverseDirection = "bidirectional"
				}
				related := gp.getConnectedSymbols(targetID, reverseDirection, rule.IncludeTypeHierarchy)

				for _, relatedID := range related {
					relatedKey := PropagationKey{
						SymbolID:  relatedID,
						Attribute: rule.Label,
						Type:      "label",
					}
					if relatedValue, exists := gp.propagationState[relatedKey]; exists {
						totalStrength += relatedValue.Strength
						sources = append(sources, relatedID)
						if relatedValue.Hops > maxHops {
							maxHops = relatedValue.Hops
						}
					}
				}

				// Only update if we have incoming values
				if totalStrength > 0 {
					newState[targetKey] = &PropagationValue{
						Strength:    totalStrength,
						Source:      targetID, // Self as source for accumulated value
						Hops:        maxHops + 1,
						Path:        append(sources, targetID),
						Metadata:    map[string]interface{}{"sources": sources},
						LastUpdated: gp.iterationCount,
					}
					processed[targetID] = true
				}
			}
		}
	}

	return nil
}

// calculatePropagatedStrength computes strength based on propagation mode
func (gp *GraphPropagator) calculatePropagatedStrength(rule LabelPropagationRule, value *PropagationValue, targetID types.SymbolID) float64 {
	baseStrength := value.Strength

	switch rule.Mode {
	case ModeReachability:
		// Binary reachability: always full strength
		return 1.0

	case ModeAccumulation:
		// Accumulate: return the source strength (will be summed in propagateLabel)
		strength := value.Strength
		if rule.Boost > 0 && gp.shouldBoost(targetID, rule.Conditions) {
			strength *= rule.Boost
		}
		return strength

	case ModeDecay:
		// Decay: reduce by decay factor per hop
		strength := baseStrength * rule.Decay
		if rule.Boost > 0 && gp.shouldBoost(targetID, rule.Conditions) {
			strength *= rule.Boost
		}
		return strength

	case ModeMax:
		// Max: keep the original strength (will be compared in propagateLabel)
		strength := baseStrength
		if rule.Boost > 0 && gp.shouldBoost(targetID, rule.Conditions) {
			strength *= rule.Boost
		}
		return strength

	default:
		// Default to decay mode
		return baseStrength * rule.Decay
	}
}

// propagateDependency aggregates dependencies according to rules
func (gp *GraphPropagator) propagateDependency(rule DependencyPropagationRule, newState map[PropagationKey]*PropagationValue) error {
	// Find all current instances of this dependency type
	for key, value := range gp.propagationState {
		if key.Type == "dependency" && key.Attribute == rule.DependencyType {
			// Skip if we've reached max depth
			if value.Hops >= rule.MaxDepth {
				continue
			}

			// Get connected symbols based on direction
			var targets []types.SymbolID
			switch rule.Direction {
			case "upstream":
				targets = gp.getCallers(key.SymbolID)
			case "downstream":
				targets = gp.getCallees(key.SymbolID)
			}

			// Aggregate to targets
			for _, targetID := range targets {
				targetKey := PropagationKey{
					SymbolID:  targetID,
					Attribute: rule.DependencyType,
					Type:      "dependency",
				}

				// Calculate weight based on function
				weight := gp.calculateWeight(rule.WeightFunction, value.Hops+1)

				// Skip if below threshold
				if weight < rule.Threshold {
					continue
				}

				// Aggregate with existing value
				if existing, exists := newState[targetKey]; exists {
					gp.aggregateDependencyValues(existing, value, rule.Aggregation, weight)
				} else {
					newPath := append(value.Path, targetID)
					newState[targetKey] = &PropagationValue{
						Strength:    weight,
						Source:      value.Source,
						Hops:        value.Hops + 1,
						Path:        newPath,
						Metadata:    value.Metadata,
						LastUpdated: gp.iterationCount,
					}
				}
			}
		}
	}

	return nil
}

// Helper functions for graph navigation
func (gp *GraphPropagator) getCallees(symbolID types.SymbolID) []types.SymbolID {
	if gp.refTracker != nil {
		return gp.refTracker.GetCalleeSymbols(symbolID)
	}
	return []types.SymbolID{}
}

func (gp *GraphPropagator) getCallers(symbolID types.SymbolID) []types.SymbolID {
	if gp.refTracker != nil {
		return gp.refTracker.GetCallerSymbols(symbolID)
	}
	return []types.SymbolID{}
}

// getImplementors returns types that implement an interface.
// Used for propagating attributes from an interface to concrete implementations.
func (gp *GraphPropagator) getImplementors(symbolID types.SymbolID) []types.SymbolID {
	if gp.refTracker != nil {
		return gp.refTracker.GetImplementors(symbolID)
	}
	return []types.SymbolID{}
}

// getImplementedInterfaces returns interfaces that a type implements.
// Used for propagating attributes from a concrete type up to its interfaces.
func (gp *GraphPropagator) getImplementedInterfaces(symbolID types.SymbolID) []types.SymbolID {
	if gp.refTracker != nil {
		return gp.refTracker.GetImplementedInterfaces(symbolID)
	}
	return []types.SymbolID{}
}

// getDerivedTypes returns types that extend/inherit from a base type.
// Used for propagating attributes from a base type to derived types.
func (gp *GraphPropagator) getDerivedTypes(symbolID types.SymbolID) []types.SymbolID {
	if gp.refTracker != nil {
		return gp.refTracker.GetDerivedTypes(symbolID)
	}
	return []types.SymbolID{}
}

// getBaseTypes returns base types that a type extends/inherits from.
// Used for propagating attributes from a derived type to base types.
func (gp *GraphPropagator) getBaseTypes(symbolID types.SymbolID) []types.SymbolID {
	if gp.refTracker != nil {
		return gp.refTracker.GetBaseTypes(symbolID)
	}
	return []types.SymbolID{}
}

// getImplementorsWithQuality returns implementors sorted by quality (highest first).
// Allows preferring implementations with stronger evidence over heuristic matches.
func (gp *GraphPropagator) getImplementorsWithQuality(symbolID types.SymbolID) []ImplementorWithQuality {
	if gp.refTracker != nil {
		return gp.refTracker.GetImplementorsWithQuality(symbolID)
	}
	return nil
}

// getConnectedSymbols returns all symbols connected to the given symbol
// through the specified relationship types, combining call graph with type hierarchy.
// This enables attribute propagation through both call relationships AND type relationships.
//
// direction: "upstream", "downstream", or "bidirectional"
// includeTypeHierarchy: when true, includes implements/extends relationships
func (gp *GraphPropagator) getConnectedSymbols(symbolID types.SymbolID, direction string, includeTypeHierarchy bool) []types.SymbolID {
	var targets []types.SymbolID
	seen := make(map[types.SymbolID]bool)

	addUnique := func(ids []types.SymbolID) {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				targets = append(targets, id)
			}
		}
	}

	switch direction {
	case "downstream":
		// Downstream: where attributes flow to (callees + implementors + derived types)
		addUnique(gp.getCallees(symbolID))
		if includeTypeHierarchy {
			// For interface methods, propagate to implementor methods
			addUnique(gp.getImplementors(symbolID))
			// For base type symbols, propagate to derived type symbols
			addUnique(gp.getDerivedTypes(symbolID))
		}

	case "upstream":
		// Upstream: where attributes flow from (callers + interfaces + base types)
		addUnique(gp.getCallers(symbolID))
		if includeTypeHierarchy {
			// For implementor methods, propagate to interface methods
			addUnique(gp.getImplementedInterfaces(symbolID))
			// For derived type symbols, propagate to base type symbols
			addUnique(gp.getBaseTypes(symbolID))
		}

	case "bidirectional":
		// Both directions
		addUnique(gp.getCallees(symbolID))
		addUnique(gp.getCallers(symbolID))
		if includeTypeHierarchy {
			addUnique(gp.getImplementors(symbolID))
			addUnique(gp.getImplementedInterfaces(symbolID))
			addUnique(gp.getDerivedTypes(symbolID))
			addUnique(gp.getBaseTypes(symbolID))
		}
	}

	return targets
}

// shouldBoost determines if a boost should be applied based on conditions
func (gp *GraphPropagator) shouldBoost(symbolID types.SymbolID, conditions []string) bool {
	if len(conditions) == 0 || gp.annotator == nil {
		return false
	}

	// Get symbol information
	symbol := gp.refTracker.GetEnhancedSymbol(symbolID)
	if symbol == nil {
		return false
	}

	// Get annotations for this symbol
	fileID := types.FileID(symbolID >> 32) // Extract FileID from composite SymbolID
	annotation := gp.annotator.GetAnnotation(fileID, symbolID)

	if annotation == nil {
		return false
	}

	// Check if any condition matches
	for _, condition := range conditions {
		if gp.matchesCondition(symbol, annotation, condition) {
			return true
		}
	}

	return false
}

// matchesCondition checks if a symbol matches a specific condition
func (gp *GraphPropagator) matchesCondition(symbol *types.EnhancedSymbol, annotation *SemanticAnnotation, condition string) bool {
	// Check labels
	for _, label := range annotation.Labels {
		if strings.Contains(strings.ToLower(label), strings.ToLower(condition)) {
			return true
		}
	}

	// Check category
	if strings.Contains(strings.ToLower(annotation.Category), strings.ToLower(condition)) {
		return true
	}

	// Check tags
	if value, exists := annotation.Tags[condition]; exists && value != "" {
		return true
	}

	// Check symbol type/name patterns
	switch condition {
	case "public":
		return gp.isPublicSymbol(symbol)
	case "private", "internal":
		return gp.isPrivateSymbol(symbol)
	case "function":
		return symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeMethod
	case "class", "struct":
		return symbol.Type == types.SymbolTypeClass || symbol.Type == types.SymbolTypeStruct || symbol.Type == types.SymbolTypeType
	case "critical":
		return gp.isCriticalSymbol(annotation)
	case "payment":
		return gp.isPaymentRelated(symbol, annotation)
	case "security":
		return gp.isSecurityRelated(symbol, annotation)
	}

	return false
}

// Helper functions for condition checking
func (gp *GraphPropagator) isPublicSymbol(symbol *types.EnhancedSymbol) bool {
	// In Go, exported symbols start with uppercase letter
	if len(symbol.Name) > 0 {
		firstChar := rune(symbol.Name[0])
		return firstChar >= 'A' && firstChar <= 'Z'
	}
	return false
}

func (gp *GraphPropagator) isPrivateSymbol(symbol *types.EnhancedSymbol) bool {
	return !gp.isPublicSymbol(symbol)
}

func (gp *GraphPropagator) isCriticalSymbol(annotation *SemanticAnnotation) bool {
	criticalKeywords := []string{"critical", "important", "security", "payment", "auth", "checkout"}

	for _, label := range annotation.Labels {
		for _, keyword := range criticalKeywords {
			if strings.Contains(strings.ToLower(label), keyword) {
				return true
			}
		}
	}

	return strings.Contains(strings.ToLower(annotation.Category), "critical")
}

func (gp *GraphPropagator) isPaymentRelated(symbol *types.EnhancedSymbol, annotation *SemanticAnnotation) bool {
	paymentKeywords := []string{"payment", "pay", "checkout", "billing", "invoice", "transaction"}

	// Check symbol name
	symbolName := strings.ToLower(symbol.Name)
	for _, keyword := range paymentKeywords {
		if strings.Contains(symbolName, keyword) {
			return true
		}
	}

	// Check annotations
	for _, label := range annotation.Labels {
		for _, keyword := range paymentKeywords {
			if strings.Contains(strings.ToLower(label), keyword) {
				return true
			}
		}
	}

	return false
}

func (gp *GraphPropagator) isSecurityRelated(symbol *types.EnhancedSymbol, annotation *SemanticAnnotation) bool {
	securityKeywords := []string{"auth", "security", "token", "validate", "verify", "encrypt", "decrypt", "permission", "access"}

	// Check symbol name
	symbolName := strings.ToLower(symbol.Name)
	for _, keyword := range securityKeywords {
		if strings.Contains(symbolName, keyword) {
			return true
		}
	}

	// Check annotations
	for _, label := range annotation.Labels {
		for _, keyword := range securityKeywords {
			if strings.Contains(strings.ToLower(label), keyword) {
				return true
			}
		}
	}

	return false
}

// calculateWeight computes weight based on distance and function type
func (gp *GraphPropagator) calculateWeight(weightFunction string, hops int) float64 {
	switch weightFunction {
	case "linear":
		return math.Max(0, 1.0-float64(hops)*0.2)
	case "exponential":
		return math.Pow(0.8, float64(hops))
	case "log":
		if hops == 0 {
			return 1.0
		}
		return 1.0 / math.Log(float64(hops)+1)
	default:
		return math.Pow(0.8, float64(hops)) // Default exponential decay
	}
}

// aggregateDependencyValues combines dependency values using the specified aggregation method
func (gp *GraphPropagator) aggregateDependencyValues(existing, new *PropagationValue, aggregation string, weight float64) {
	switch aggregation {
	case "sum":
		existing.Strength += new.Strength * weight
	case "max":
		if new.Strength*weight > existing.Strength {
			existing.Strength = new.Strength * weight
			existing.Source = new.Source
			existing.Path = new.Path
		}
	case "weighted_sum":
		existing.Strength += weight
	case "unique":
		// For unique aggregation, we keep track in metadata
		if existing.Metadata == nil {
			existing.Metadata = make(map[string]interface{})
		}
		sources, ok := existing.Metadata["unique_sources"].([]types.SymbolID)
		if !ok {
			sources = []types.SymbolID{existing.Source}
		}
		// Add new source if not already present
		found := false
		for _, source := range sources {
			if source == new.Source {
				found = true
				break
			}
		}
		if !found {
			sources = append(sources, new.Source)
		}
		existing.Metadata["unique_sources"] = sources
		existing.Strength = float64(len(sources))
	}
}

// applyCustomRule applies a custom propagation rule
func (gp *GraphPropagator) applyCustomRule(rule CustomPropagationRule, newState map[PropagationKey]*PropagationValue) error {
	// Evaluate trigger condition for all symbols in current state
	for key, value := range gp.propagationState {
		if gp.evaluateCustomTrigger(rule.Trigger, key, value) {
			// Apply the custom action
			if err := gp.executeCustomAction(rule.Action, rule.Parameters, key, value, newState); err != nil {
				return fmt.Errorf("failed to execute custom action for rule %s: %w", rule.Name, err)
			}
		}
	}

	return nil
}

// evaluateCustomTrigger evaluates a trigger condition for a specific symbol/value
func (gp *GraphPropagator) evaluateCustomTrigger(trigger string, key PropagationKey, value *PropagationValue) bool {
	// Parse simple trigger expressions
	trigger = strings.TrimSpace(trigger)

	// Handle OR conditions
	if strings.Contains(trigger, " OR ") {
		parts := strings.Split(trigger, " OR ")
		for _, part := range parts {
			if gp.evaluateSingleTrigger(strings.TrimSpace(part), key, value) {
				return true
			}
		}
		return false
	}

	// Handle AND conditions
	if strings.Contains(trigger, " AND ") {
		parts := strings.Split(trigger, " AND ")
		for _, part := range parts {
			if !gp.evaluateSingleTrigger(strings.TrimSpace(part), key, value) {
				return false
			}
		}
		return true
	}

	// Single condition
	return gp.evaluateSingleTrigger(trigger, key, value)
}

// evaluateSingleTrigger evaluates a single trigger condition
func (gp *GraphPropagator) evaluateSingleTrigger(trigger string, key PropagationKey, value *PropagationValue) bool {
	// Parse function-style conditions: has_label(checkout)
	if strings.HasPrefix(trigger, "has_label(") && strings.HasSuffix(trigger, ")") {
		labelName := strings.Trim(trigger[10:len(trigger)-1], " \"'")
		return gp.symbolHasLabel(key.SymbolID, labelName)
	}

	// Parse function-style conditions: has_dependency(database)
	if strings.HasPrefix(trigger, "has_dependency(") && strings.HasSuffix(trigger, ")") {
		depType := strings.Trim(trigger[15:len(trigger)-1], " \"'")
		return gp.symbolHasDependency(key.SymbolID, depType)
	}

	// Parse strength conditions: strength > 0.5
	if strings.Contains(trigger, "strength") {
		return gp.evaluateStrengthCondition(trigger, value.Strength)
	}

	// Parse hop conditions: hops < 3
	if strings.Contains(trigger, "hops") {
		return gp.evaluateHopCondition(trigger, value.Hops)
	}

	// Parse attribute type conditions: type == "label"
	if strings.Contains(trigger, "type") {
		return gp.evaluateTypeCondition(trigger, key.Type)
	}

	return false
}

// Helper functions for trigger evaluation
func (gp *GraphPropagator) symbolHasLabel(symbolID types.SymbolID, labelName string) bool {
	if gp.annotator == nil {
		return false
	}

	// Extract FileID from composite SymbolID
	fileID := types.FileID(symbolID >> 32)
	annotation := gp.annotator.GetAnnotation(fileID, symbolID)

	if annotation == nil {
		return false
	}

	for _, label := range annotation.Labels {
		if strings.EqualFold(label, labelName) {
			return true
		}
	}

	return false
}

func (gp *GraphPropagator) symbolHasDependency(symbolID types.SymbolID, depType string) bool {
	if gp.annotator == nil {
		return false
	}

	// Extract FileID from composite SymbolID
	fileID := types.FileID(symbolID >> 32)
	annotation := gp.annotator.GetAnnotation(fileID, symbolID)

	if annotation == nil {
		return false
	}

	for _, dep := range annotation.Dependencies {
		if strings.EqualFold(dep.Type, depType) {
			return true
		}
	}

	return false
}

func (gp *GraphPropagator) evaluateStrengthCondition(condition string, strength float64) bool {
	// Parse conditions like "strength > 0.5", "strength >= 0.8"
	if strings.Contains(condition, ">=") {
		parts := strings.Split(condition, ">=")
		if len(parts) == 2 {
			if threshold, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return strength >= threshold
			}
		}
	} else if strings.Contains(condition, "<=") {
		parts := strings.Split(condition, "<=")
		if len(parts) == 2 {
			if threshold, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return strength <= threshold
			}
		}
	} else if strings.Contains(condition, ">") {
		parts := strings.Split(condition, ">")
		if len(parts) == 2 {
			if threshold, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return strength > threshold
			}
		}
	} else if strings.Contains(condition, "<") {
		parts := strings.Split(condition, "<")
		if len(parts) == 2 {
			if threshold, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return strength < threshold
			}
		}
	}

	return false
}

func (gp *GraphPropagator) evaluateHopCondition(condition string, hops int) bool {
	// Parse conditions like "hops < 3", "hops >= 2"
	if strings.Contains(condition, ">=") {
		parts := strings.Split(condition, ">=")
		if len(parts) == 2 {
			if threshold, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				return hops >= threshold
			}
		}
	} else if strings.Contains(condition, "<=") {
		parts := strings.Split(condition, "<=")
		if len(parts) == 2 {
			if threshold, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				return hops <= threshold
			}
		}
	} else if strings.Contains(condition, ">") {
		parts := strings.Split(condition, ">")
		if len(parts) == 2 {
			if threshold, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				return hops > threshold
			}
		}
	} else if strings.Contains(condition, "<") {
		parts := strings.Split(condition, "<")
		if len(parts) == 2 {
			if threshold, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				return hops < threshold
			}
		}
	}

	return false
}

func (gp *GraphPropagator) evaluateTypeCondition(condition string, attrType string) bool {
	// Parse conditions like "type == \"label\"", "type != \"dependency\""
	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) == 2 {
			expectedType := strings.Trim(strings.TrimSpace(parts[1]), " \"'")
			return strings.EqualFold(attrType, expectedType)
		}
	} else if strings.Contains(condition, "!=") {
		parts := strings.Split(condition, "!=")
		if len(parts) == 2 {
			expectedType := strings.Trim(strings.TrimSpace(parts[1]), " \"'")
			return !strings.EqualFold(attrType, expectedType)
		}
	}

	return false
}

// executeCustomAction executes a custom action on a propagation value
func (gp *GraphPropagator) executeCustomAction(action string, parameters map[string]interface{}, key PropagationKey, value *PropagationValue, newState map[PropagationKey]*PropagationValue) error {
	action = strings.TrimSpace(action)

	// Handle strength multiplication: multiply_strength(1.3)
	if strings.HasPrefix(action, "multiply_strength(") && strings.HasSuffix(action, ")") {
		factorStr := strings.Trim(action[18:len(action)-1], " \"'")
		if factor, err := strconv.ParseFloat(factorStr, 64); err == nil {
			// Update the value in newState
			if existing, exists := newState[key]; exists {
				existing.Strength *= factor
			}
			return nil
		}
		return fmt.Errorf("invalid factor in multiply_strength: %s", factorStr)
	}

	// Handle strength addition: add_strength(0.2)
	if strings.HasPrefix(action, "add_strength(") && strings.HasSuffix(action, ")") {
		amountStr := strings.Trim(action[13:len(action)-1], " \"'")
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			// Update the value in newState
			if existing, exists := newState[key]; exists {
				existing.Strength += amount
			}
			return nil
		}
		return fmt.Errorf("invalid amount in add_strength: %s", amountStr)
	}

	// Handle strength setting: set_strength(0.8)
	if strings.HasPrefix(action, "set_strength(") && strings.HasSuffix(action, ")") {
		valueStr := strings.Trim(action[13:len(action)-1], " \"'")
		if newStrength, err := strconv.ParseFloat(valueStr, 64); err == nil {
			// Update the value in newState
			if existing, exists := newState[key]; exists {
				existing.Strength = newStrength
			}
			return nil
		}
		return fmt.Errorf("invalid value in set_strength: %s", valueStr)
	}

	// Handle decay modification: multiply_decay(0.9)
	if strings.HasPrefix(action, "multiply_decay(") && strings.HasSuffix(action, ")") {
		// This would affect future propagation steps
		// For now, we apply it as a strength multiplier
		factorStr := strings.Trim(action[15:len(action)-1], " \"'")
		if factor, err := strconv.ParseFloat(factorStr, 64); err == nil {
			if existing, exists := newState[key]; exists {
				existing.Strength *= factor
			}
			return nil
		}
		return fmt.Errorf("invalid factor in multiply_decay: %s", factorStr)
	}

	return fmt.Errorf("unknown custom action: %s", action)
}

// checkConvergence determines if the propagation has converged
func (gp *GraphPropagator) checkConvergence() {
	// Simple convergence check based on total strength change
	// More sophisticated implementations could track per-symbol changes
	totalChange := 0.0
	stateCount := 0

	for _, value := range gp.propagationState {
		if value.LastUpdated == gp.iterationCount {
			totalChange += value.Strength
		}
		stateCount++
	}

	averageChange := totalChange / float64(stateCount)
	gp.converged = averageChange < gp.config.ConvergenceThreshold
}

// buildResults constructs the final propagation results
func (gp *GraphPropagator) buildResults() error {
	// Build propagated labels
	for key, value := range gp.propagationState {
		if key.Type == "label" {
			label := PropagatedLabel{
				Label:      key.Attribute,
				Strength:   value.Strength,
				Source:     value.Source,
				Path:       value.Path,
				Hops:       value.Hops,
				Metadata:   value.Metadata,
				Confidence: gp.calculateConfidence(value),
			}
			gp.propagatedLabels[key.SymbolID] = append(gp.propagatedLabels[key.SymbolID], label)
		} else if key.Type == "dependency" {
			dep := gp.buildPropagatedDependency(key, value)
			gp.propagatedDeps[key.SymbolID] = append(gp.propagatedDeps[key.SymbolID], *dep)
		}
	}

	// Sort results by strength
	for symbolID := range gp.propagatedLabels {
		sort.Slice(gp.propagatedLabels[symbolID], func(i, j int) bool {
			return gp.propagatedLabels[symbolID][i].Strength > gp.propagatedLabels[symbolID][j].Strength
		})
	}

	return nil
}

// buildPropagatedDependency constructs a PropagatedDependency from propagation value
func (gp *GraphPropagator) buildPropagatedDependency(key PropagationKey, value *PropagationValue) *PropagatedDependency {
	dep := &PropagatedDependency{
		Type:     key.Attribute,
		Count:    int(value.Strength),
		Sources:  []types.SymbolID{value.Source},
		Depth:    value.Hops,
		Weight:   value.Strength,
		Details:  make([]DependencyDetail, 0),
		Metadata: value.Metadata,
	}

	// Handle unique sources if present
	if uniqueSources, ok := value.Metadata["unique_sources"].([]types.SymbolID); ok {
		dep.Sources = uniqueSources
		dep.Count = len(uniqueSources)
	}

	return dep
}

// calculateConfidence computes confidence score for a propagated value
func (gp *GraphPropagator) calculateConfidence(value *PropagationValue) float64 {
	// Base confidence on strength and path length
	pathPenalty := math.Pow(0.9, float64(value.Hops))
	return value.Strength * pathPenalty
}

// runAnalysis performs high-level analysis of propagation results
func (gp *GraphPropagator) runAnalysis() error {
	if gp.config.AnalysisConfig.DetectEntryPoints {
		gp.detectEntryPoints()
	}

	if gp.config.AnalysisConfig.CalculateDepth {
		gp.calculateDependencyDepth()
	}

	if gp.config.AnalysisConfig.FindCriticalPaths {
		gp.findCriticalPaths()
	}

	return nil
}

// detectEntryPoints identifies symbols that serve as system entry points
func (gp *GraphPropagator) detectEntryPoints() {
	for _, label := range gp.config.AnalysisConfig.EntryPointLabels {
		symbols := gp.annotator.GetSymbolsByLabel(label)
		gp.entryPoints[label] = symbols
	}
}

// calculateDependencyDepth computes maximum dependency depth for each symbol
func (gp *GraphPropagator) calculateDependencyDepth() {
	for symbolID, deps := range gp.propagatedDeps {
		maxDepth := 0
		for _, dep := range deps {
			if dep.Depth > maxDepth {
				maxDepth = dep.Depth
			}
		}
		gp.dependencyDepth[symbolID] = maxDepth
	}
}

// findCriticalPaths identifies high-impact propagation paths
func (gp *GraphPropagator) findCriticalPaths() {
	threshold := gp.config.AnalysisConfig.HighImpactThreshold

	for symbolID, labels := range gp.propagatedLabels {
		for _, label := range labels {
			if label.Strength > threshold && gp.isCriticalLabel(label.Label) {
				path := CriticalPath{
					Path:        label.Path,
					Labels:      []string{label.Label},
					TotalImpact: label.Strength,
					Description: "Critical path for " + label.Label,
					Metadata: map[string]interface{}{
						"symbol_id":  symbolID,
						"confidence": label.Confidence,
					},
				}
				gp.criticalPaths = append(gp.criticalPaths, path)
			}
		}
	}

	// Sort by impact
	sort.Slice(gp.criticalPaths, func(i, j int) bool {
		return gp.criticalPaths[i].TotalImpact > gp.criticalPaths[j].TotalImpact
	})
}

// isCriticalLabel checks if a label is considered critical
func (gp *GraphPropagator) isCriticalLabel(label string) bool {
	for _, criticalLabel := range gp.config.AnalysisConfig.CriticalLabels {
		if strings.Contains(strings.ToLower(label), strings.ToLower(criticalLabel)) {
			return true
		}
	}
	return false
}

// Query and access functions

// GetPropagatedLabels returns all propagated labels for a symbol
func (gp *GraphPropagator) GetPropagatedLabels(symbolID types.SymbolID) []PropagatedLabel {
	return gp.propagatedLabels[symbolID]
}

// GetSymbolsWithLabel returns all symbols that have a specific propagated label
// with at least the specified minimum strength
func (gp *GraphPropagator) GetSymbolsWithLabel(label string, minStrength float64) []*AnnotatedSymbol {
	var results []*AnnotatedSymbol

	// Iterate through all propagated labels
	for symbolID, labels := range gp.propagatedLabels {
		for _, pLabel := range labels {
			if pLabel.Label == label && pLabel.Strength >= minStrength {
				// Extract FileID from SymbolID
				fileID := types.FileID(symbolID >> 32)
				line := int((symbolID >> 16) & 0xFFFF)
				column := int(symbolID & 0xFFFF)

				// Create a basic AnnotatedSymbol
				annotatedSymbol := &AnnotatedSymbol{
					Symbol: types.Symbol{
						FileID: fileID,
						Line:   line,
						Column: column,
					},
					SymbolID: symbolID,
					FileID:   fileID,
					FilePath: "", // Would need file service to get path
					// Annotation will be filled if we can find it
				}
				results = append(results, annotatedSymbol)
				break // Only add each symbol once
			}
		}
	}

	return results
}

// GetPropagationPath returns the propagation path showing how a label reached a target symbol
// Returns the sequence of symbol IDs from source to target, or nil if no path exists
func (gp *GraphPropagator) GetPropagationPath(targetID types.SymbolID, label string) []types.SymbolID {
	// Check the propagation state for this symbol and label
	key := PropagationKey{
		SymbolID:  targetID,
		Attribute: label,
		Type:      "label",
	}

	if value, exists := gp.propagationState[key]; exists {
		// The Path field in PropagationValue already contains the propagation path
		return value.Path
	}

	return nil
}

// GetPropagatedDependencies returns all propagated dependencies for a symbol
func (gp *GraphPropagator) GetPropagatedDependencies(symbolID types.SymbolID) []PropagatedDependency {
	return gp.propagatedDeps[symbolID]
}

// GetEntryPoints returns identified entry points by label
func (gp *GraphPropagator) GetEntryPoints(label string) []*AnnotatedSymbol {
	return gp.entryPoints[label]
}

// GetCriticalPaths returns identified critical paths
func (gp *GraphPropagator) GetCriticalPaths() []CriticalPath {
	return gp.criticalPaths
}

// GetDependencyDepth returns the maximum dependency depth for a symbol
func (gp *GraphPropagator) GetDependencyDepth(symbolID types.SymbolID) int {
	return gp.dependencyDepth[symbolID]
}

// GetPropagationStats provides statistics about the propagation results
func (gp *GraphPropagator) GetPropagationStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["iterations_run"] = gp.iterationCount
	stats["converged"] = gp.converged
	stats["total_propagated_labels"] = len(gp.propagationState)
	stats["symbols_with_propagated_labels"] = len(gp.propagatedLabels)
	stats["symbols_with_propagated_deps"] = len(gp.propagatedDeps)
	stats["entry_points"] = len(gp.entryPoints)
	stats["critical_paths"] = len(gp.criticalPaths)

	// Calculate average propagation depths
	totalDepth := 0
	for _, depth := range gp.dependencyDepth {
		totalDepth += depth
	}
	if len(gp.dependencyDepth) > 0 {
		stats["average_dependency_depth"] = float64(totalDepth) / float64(len(gp.dependencyDepth))
	}

	return stats
}

// ============================================================================
// INTERFACE CALL ATTRIBUTION
// Attaches attributes to concrete implementations when propagating through
// interface method calls, using code analysis with heuristic fallback.
// ============================================================================

// InterfaceCallAttribution represents an attribution of an interface call
// to specific concrete implementations with confidence levels.
type InterfaceCallAttribution struct {
	InterfaceSymbolID types.SymbolID              `json:"interface_symbol_id"`
	Implementations   []ImplementationAttribution `json:"implementations"`
	AttributionMethod string                      `json:"attribution_method"` // "code_analysis", "heuristic", "combined"
}

// ImplementationAttribution represents attribution to a specific implementation
type ImplementationAttribution struct {
	SymbolID   types.SymbolID `json:"symbol_id"`
	Confidence float64        `json:"confidence"` // 0.0-1.0, higher = more confident
	Quality    string         `json:"quality"`    // Quality from reference (assigned, returned, cast, heuristic)
	Evidence   string         `json:"evidence"`   // Description of why this implementation was chosen
}

// GetInterfaceCallImplementations returns potential concrete implementations
// for an interface method call, using code analysis first and falling back
// to heuristics when explicit evidence is insufficient.
//
// This method implements a tiered attribution strategy:
//  1. Code Analysis: Use explicit implements references with quality ranking
//     (assigned > returned > cast > heuristic)
//  2. Heuristic Fallback: If no high-quality matches, use method signature matching
//  3. Combined: Return all candidates with appropriate confidence scores
func (gp *GraphPropagator) GetInterfaceCallImplementations(interfaceSymbolID types.SymbolID) *InterfaceCallAttribution {
	result := &InterfaceCallAttribution{
		InterfaceSymbolID: interfaceSymbolID,
		Implementations:   make([]ImplementationAttribution, 0),
		AttributionMethod: "none",
	}

	if gp.refTracker == nil {
		return result
	}

	// First, try to get implementors with quality ranking (code analysis)
	implementorsWithQuality := gp.getImplementorsWithQuality(interfaceSymbolID)

	if len(implementorsWithQuality) > 0 {
		// Check if we have high-quality matches (explicit code evidence)
		hasHighQuality := false
		for _, impl := range implementorsWithQuality {
			// Assigned, returned, and cast are considered high quality
			if impl.Quality == types.RefQualityAssigned ||
				impl.Quality == types.RefQualityReturned ||
				impl.Quality == types.RefQualityCast {
				hasHighQuality = true
				break
			}
		}

		if hasHighQuality {
			// Use code analysis results
			result.AttributionMethod = "code_analysis"
			for _, impl := range implementorsWithQuality {
				confidence := gp.calculateImplementationConfidence(impl.Quality, impl.Rank)
				result.Implementations = append(result.Implementations, ImplementationAttribution{
					SymbolID:   impl.SymbolID,
					Confidence: confidence,
					Quality:    impl.Quality,
					Evidence:   fmt.Sprintf("Explicit %s evidence from code analysis", impl.Quality),
				})
			}
			return result
		}

		// We have heuristic matches - use them but mark as heuristic
		result.AttributionMethod = "heuristic"
		for _, impl := range implementorsWithQuality {
			confidence := gp.calculateImplementationConfidence(impl.Quality, impl.Rank)
			result.Implementations = append(result.Implementations, ImplementationAttribution{
				SymbolID:   impl.SymbolID,
				Confidence: confidence,
				Quality:    impl.Quality,
				Evidence:   "Method signature matching (heuristic)",
			})
		}
		return result
	}

	// No implementors found - return empty result
	return result
}

// calculateImplementationConfidence converts quality ranking to a confidence score
func (gp *GraphPropagator) calculateImplementationConfidence(quality string, rank int) float64 {
	// Map quality to base confidence
	// Explicit code evidence gets high confidence
	// Heuristic matches get lower confidence
	switch quality {
	case types.RefQualityAssigned:
		return 0.95 // Very high - direct assignment to interface type
	case types.RefQualityReturned:
		return 0.90 // High - returned as interface type
	case types.RefQualityCast:
		return 0.85 // High - explicit type assertion/cast
	case types.RefQualityHeuristic:
		return 0.50 // Medium - method signature matching only
	default:
		return 0.30 // Low - unknown quality
	}
}

// PropagateWithInterfaceAttribution propagates a label through an interface call
// to concrete implementations, adjusting strength based on attribution confidence.
// This allows labels to flow through interface boundaries with appropriate uncertainty.
func (gp *GraphPropagator) PropagateWithInterfaceAttribution(
	rule LabelPropagationRule,
	sourceValue *PropagationValue,
	interfaceSymbolID types.SymbolID,
	newState map[PropagationKey]*PropagationValue,
) error {
	attribution := gp.GetInterfaceCallImplementations(interfaceSymbolID)

	if len(attribution.Implementations) == 0 {
		// No implementations found - cannot propagate
		return nil
	}

	for _, impl := range attribution.Implementations {
		targetKey := PropagationKey{
			SymbolID:  impl.SymbolID,
			Attribute: rule.Label,
			Type:      "label",
		}

		// Calculate new strength based on propagation mode and attribution confidence
		baseStrength := gp.calculatePropagatedStrength(rule, sourceValue, impl.SymbolID)

		// Adjust strength by confidence for heuristic attributions
		// Code analysis attributions keep full strength
		adjustedStrength := baseStrength
		if attribution.AttributionMethod == "heuristic" {
			adjustedStrength = baseStrength * impl.Confidence
		}

		// Determine if we should update based on mode
		shouldUpdate := false
		if existing, exists := newState[targetKey]; exists {
			switch rule.Mode {
			case ModeReachability:
				shouldUpdate = false // Already reachable
			case ModeDecay, ModeMax:
				shouldUpdate = adjustedStrength > existing.Strength
			case ModeAccumulation:
				shouldUpdate = true // Always accumulate
			}
		} else {
			shouldUpdate = adjustedStrength > 0
		}

		if shouldUpdate {
			newPath := append(sourceValue.Path, impl.SymbolID)
			metadata := make(map[string]interface{})
			for k, v := range sourceValue.Metadata {
				metadata[k] = v
			}
			metadata["attribution_method"] = attribution.AttributionMethod
			metadata["attribution_confidence"] = impl.Confidence
			metadata["attribution_quality"] = impl.Quality

			newState[targetKey] = &PropagationValue{
				Strength:    adjustedStrength,
				Source:      sourceValue.Source,
				Hops:        sourceValue.Hops + 1,
				Path:        newPath,
				Metadata:    metadata,
				LastUpdated: gp.iterationCount,
			}
		}
	}

	return nil
}

// getDefaultPropagationConfig returns a sensible default configuration
func getDefaultPropagationConfig() *PropagationConfig {
	return &PropagationConfig{
		MaxIterations:        10,
		ConvergenceThreshold: 0.001,
		DefaultDecay:         0.8, // Only used when Mode is not specified
		LabelRules: []LabelPropagationRule{
			{
				Label:                "critical",
				Direction:            "upstream",
				Mode:                 ModeReachability, // Bug criticality doesn't decay
				MaxHops:              0,                // Unlimited - all callers are affected
				Priority:             3,
				IncludeTypeHierarchy: true, // Critical bugs propagate through interface implementations
			},
			{
				Label:                "security",
				Direction:            "upstream",
				Mode:                 ModeReachability, // Security issues affect all callers
				MaxHops:              0,
				Priority:             3,
				IncludeTypeHierarchy: true, // Security issues propagate through type hierarchy
			},
			{
				Label:                "database-call",
				Direction:            "upstream",
				Mode:                 ModeAccumulation, // DB calls accumulate upward
				MaxHops:              0,
				Priority:             2,
				IncludeTypeHierarchy: true, // DB call counts include implementations
			},
			{
				Label:                "api-endpoint",
				Direction:            "downstream",
				Mode:                 ModeReachability, // API reachability is binary
				MaxHops:              10,
				Priority:             2,
				IncludeTypeHierarchy: true, // API endpoints flow to implementations
			},
			{
				Label:                "ui-relevance",
				Direction:            "bidirectional",
				Mode:                 ModeDecay, // UI ranking uses decay
				Decay:                0.7,
				MaxHops:              5,
				MinStrength:          0.15,
				Priority:             1,
				IncludeTypeHierarchy: false, // UI relevance only follows call graph
			},
			{
				Label:                "memory-allocation",
				Direction:            "upstream",
				Mode:                 ModeAccumulation, // Memory pressure accumulates through call chains
				MaxHops:              0,                // Unlimited - propagate to all callers
				Priority:             2,
				IncludeTypeHierarchy: true, // Memory usage propagates through implementations
			},
		},
		DependencyRules: []DependencyPropagationRule{
			{
				DependencyType: "database",
				Direction:      "upstream",
				Aggregation:    "sum",
				WeightFunction: "exponential",
				MaxDepth:       5,
				Threshold:      0.1,
			},
			{
				DependencyType: "service",
				Direction:      "upstream",
				Aggregation:    "unique",
				WeightFunction: "linear",
				MaxDepth:       4,
				Threshold:      0.2,
			},
		},
		CustomRules: []CustomPropagationRule{},
		AnalysisConfig: AnalysisConfig{
			DetectEntryPoints:   true,
			CalculateDepth:      true,
			FindCriticalPaths:   true,
			EntryPointLabels:    []string{"api", "endpoint", "handler", "main"},
			CriticalLabels:      []string{"checkout", "payment", "security", "auth"},
			HighImpactThreshold: 0.7,
		},
	}
}
