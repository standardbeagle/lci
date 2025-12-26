package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kdl "github.com/sblinch/kdl-go"
	"github.com/sblinch/kdl-go/document"
)

// PropagationConfigManager handles loading, validation, and management of propagation configurations
type PropagationConfigManager struct {
	configs       map[string]*PropagationConfig
	defaultConfig *PropagationConfig
	configDir     string
	fileService   *FileService
}

// ConfigTemplate represents a reusable configuration template
type ConfigTemplate struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Config      *PropagationConfig     `json:"config"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Extends     string                 `json:"extends,omitempty"`
}

// BuiltInTemplates provides common configuration templates
var BuiltInTemplates = map[string]*ConfigTemplate{
	"web-application": {
		Name:        "Web Application",
		Description: "Configuration optimized for web application analysis with correct propagation semantics",
		Version:     "2.0",
		Config: &PropagationConfig{
			MaxIterations:        15,
			ConvergenceThreshold: 0.001,
			DefaultDecay:         0.8,
			LabelRules: []LabelPropagationRule{
				{
					Label:                "critical-bug",
					Direction:            "upstream",
					Mode:                 ModeReachability, // All callers are equally affected by bugs
					MaxHops:              0,                // Unlimited - bugs propagate to all callers
					Priority:             3,
					Conditions:           []string{"critical"},
					IncludeTypeHierarchy: true, // Propagate through interface implementations
				},
				{
					Label:                "security-vulnerability",
					Direction:            "upstream",
					Mode:                 ModeReachability, // Security issues affect all callers
					MaxHops:              0,
					Priority:             3,
					Conditions:           []string{"security"},
					IncludeTypeHierarchy: true, // Security issues propagate through type hierarchy
				},
				{
					Label:                "authentication",
					Direction:            "upstream",
					Mode:                 ModeReachability, // Auth dependencies are binary
					MaxHops:              0,
					Priority:             3,
					Conditions:           []string{"security"},
					IncludeTypeHierarchy: true, // Auth propagates through implementations
				},
				{
					Label:                "database-calls",
					Direction:            "upstream",
					Mode:                 ModeAccumulation, // DB call counts accumulate
					MaxHops:              0,
					Priority:             2,
					IncludeTypeHierarchy: true, // Include DB calls from implementations
				},
				{
					Label:                "api-calls",
					Direction:            "upstream",
					Mode:                 ModeAccumulation, // API call counts accumulate
					MaxHops:              0,
					Priority:             2,
					IncludeTypeHierarchy: true, // Include API calls from implementations
				},
				{
					Label:                "api-endpoint",
					Direction:            "downstream",
					Mode:                 ModeReachability, // Endpoint reachability is binary
					MaxHops:              10,
					Priority:             2,
					Conditions:           []string{"public"},
					IncludeTypeHierarchy: true, // Endpoints flow to implementations
				},
				{
					Label:                "checkout-flow",
					Direction:            "upstream",
					Mode:                 ModeReachability, // Checkout involvement is binary
					MaxHops:              0,
					Boost:                1.2,
					Priority:             3,
					Conditions:           []string{"payment", "critical"},
					IncludeTypeHierarchy: true, // Checkout propagates through implementations
				},
				{
					Label:                "ui-search-relevance",
					Direction:            "bidirectional",
					Mode:                 ModeDecay, // UI ranking heuristic only
					Decay:                0.7,
					MaxHops:              6,
					MinStrength:          0.1,
					Priority:             1,
					IncludeTypeHierarchy: false, // UI relevance only follows call graph
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
					DependencyType: "external-api",
					Direction:      "upstream",
					Aggregation:    "unique",
					WeightFunction: "linear",
					MaxDepth:       4,
					Threshold:      0.15,
				},
				{
					DependencyType: "cache",
					Direction:      "upstream",
					Aggregation:    "max",
					WeightFunction: "exponential",
					MaxDepth:       3,
					Threshold:      0.2,
				},
			},
			CustomRules: []CustomPropagationRule{
				{
					Name:        "critical-path-boost",
					Description: "Boost propagation for critical business paths",
					Trigger:     "has_label(checkout) OR has_label(payment)",
					Action:      "multiply_strength(1.3)",
					Parameters:  map[string]interface{}{"boost_factor": 1.3},
					Priority:    1,
				},
			},
			AnalysisConfig: AnalysisConfig{
				DetectEntryPoints:   true,
				CalculateDepth:      true,
				FindCriticalPaths:   true,
				EntryPointLabels:    []string{"api-endpoint", "handler", "controller", "route"},
				CriticalLabels:      []string{"checkout", "payment", "authentication", "security"},
				HighImpactThreshold: 0.6,
			},
		},
	},

	"microservices": {
		Name:        "Microservices Architecture",
		Description: "Configuration for microservices-based systems with proper dependency tracking",
		Version:     "2.0",
		Config: &PropagationConfig{
			MaxIterations:        12,
			ConvergenceThreshold: 0.002,
			DefaultDecay:         0.75,
			LabelRules: []LabelPropagationRule{
				{
					Label:                "service-boundary",
					Direction:            "bidirectional",
					Mode:                 ModeReachability, // Service dependencies are binary
					MaxHops:              2,                // Stop at immediate service boundaries
					Priority:             3,
					IncludeTypeHierarchy: true, // Include interface implementations at service boundaries
				},
				{
					Label:                "cross-service-call",
					Direction:            "upstream",
					Mode:                 ModeAccumulation, // Count service calls
					MaxHops:              8,
					Priority:             2,
					IncludeTypeHierarchy: true, // Include service calls through interfaces
				},
				{
					Label:                "distributed-transaction",
					Direction:            "upstream",
					Mode:                 ModeReachability, // Transaction involvement is binary
					MaxHops:              0,
					Priority:             3,
					IncludeTypeHierarchy: true, // Transactions propagate through implementations
				},
				{
					Label:                "public-api",
					Direction:            "downstream",
					Mode:                 ModeReachability, // Public API exposure is binary
					MaxHops:              4,
					Priority:             2,
					IncludeTypeHierarchy: true, // Public APIs flow to implementations
				},
				{
					Label:                "service-importance",
					Direction:            "upstream",
					Mode:                 ModeMax, // Take maximum importance level
					MaxHops:              0,
					Priority:             2,
					IncludeTypeHierarchy: true, // Include importance from implementations
				},
			},
			DependencyRules: []DependencyPropagationRule{
				{
					DependencyType: "service",
					Direction:      "upstream",
					Aggregation:    "unique",
					WeightFunction: "linear",
					MaxDepth:       6,
					Threshold:      0.1,
				},
				{
					DependencyType: "database",
					Direction:      "upstream",
					Aggregation:    "sum",
					WeightFunction: "exponential",
					MaxDepth:       3,
					Threshold:      0.2,
				},
				{
					DependencyType: "message-queue",
					Direction:      "bidirectional",
					Aggregation:    "unique",
					WeightFunction: "linear",
					MaxDepth:       5,
					Threshold:      0.15,
				},
			},
			AnalysisConfig: AnalysisConfig{
				DetectEntryPoints:   true,
				CalculateDepth:      true,
				FindCriticalPaths:   true,
				EntryPointLabels:    []string{"api-gateway", "service-entry", "public-endpoint"},
				CriticalLabels:      []string{"cross-service", "data-consistency", "transaction"},
				HighImpactThreshold: 0.5,
			},
		},
	},

	"library-analysis": {
		Name:        "Library/SDK Analysis",
		Description: "Configuration for analyzing libraries and SDKs with API stability tracking",
		Version:     "2.0",
		Config: &PropagationConfig{
			MaxIterations:        8,
			ConvergenceThreshold: 0.005,
			DefaultDecay:         0.9,
			LabelRules: []LabelPropagationRule{
				{
					Label:                "public-api",
					Direction:            "downstream",
					Mode:                 ModeReachability, // Public API reachability is binary
					MaxHops:              10,
					Priority:             2,
					IncludeTypeHierarchy: true, // Public APIs propagate to implementations
				},
				{
					Label:                "breaking-change",
					Direction:            "upstream",
					Mode:                 ModeReachability, // Breaking changes affect all callers
					MaxHops:              0,
					Priority:             3,
					IncludeTypeHierarchy: true, // Breaking changes in implementations affect interfaces
				},
				{
					Label:                "deprecated",
					Direction:            "upstream",
					Mode:                 ModeReachability, // Deprecation affects all callers
					MaxHops:              5,
					Priority:             3,
					IncludeTypeHierarchy: true, // Deprecation propagates through type hierarchy
				},
				{
					Label:                "internal-only",
					Direction:            "bidirectional",
					Mode:                 ModeReachability, // Internal usage is binary
					MaxHops:              3,
					Priority:             1,
					IncludeTypeHierarchy: false, // Internal tracking only follows call graph
				},
				{
					Label:                "complexity-score",
					Direction:            "upstream",
					Mode:                 ModeAccumulation, // Complexity accumulates
					MaxHops:              0,
					Priority:             2,
					IncludeTypeHierarchy: true, // Complexity includes implementations
				},
			},
			DependencyRules: []DependencyPropagationRule{
				{
					DependencyType: "external-dependency",
					Direction:      "upstream",
					Aggregation:    "unique",
					WeightFunction: "linear",
					MaxDepth:       4,
					Threshold:      0.1,
				},
			},
			AnalysisConfig: AnalysisConfig{
				DetectEntryPoints:   true,
				CalculateDepth:      false,
				FindCriticalPaths:   false,
				EntryPointLabels:    []string{"public-api", "exported"},
				CriticalLabels:      []string{"breaking-change", "deprecated"},
				HighImpactThreshold: 0.8,
			},
		},
	},
}

// NewPropagationConfigManager creates a new configuration manager
func NewPropagationConfigManager(configDir string) *PropagationConfigManager {
	return NewPropagationConfigManagerWithFileService(configDir, NewFileService())
}

// NewPropagationConfigManagerWithFileService creates a new configuration manager with a specific FileService
func NewPropagationConfigManagerWithFileService(configDir string, fileService *FileService) *PropagationConfigManager {
	return &PropagationConfigManager{
		configs:       make(map[string]*PropagationConfig),
		defaultConfig: getDefaultPropagationConfig(),
		configDir:     configDir,
		fileService:   fileService,
	}
}

// LoadConfig loads a configuration from file
func (pcm *PropagationConfigManager) LoadConfig(name string) (*PropagationConfig, error) {
	// Check if already loaded
	if config, exists := pcm.configs[name]; exists {
		return config, nil
	}

	// Try to load from file
	configPath := filepath.Join(pcm.configDir, name+".kdl")

	if pcm.fileExists(configPath) {
		config, err := pcm.loadConfigFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config %s: %w", name, err)
		}
		pcm.configs[name] = config
		return config, nil
	}

	// Check built-in templates
	if template, exists := BuiltInTemplates[name]; exists {
		config := template.Config
		pcm.configs[name] = config
		return config, nil
	}

	return nil, fmt.Errorf("configuration %s not found", name)
}

// LoadConfigFromTemplate loads and customizes a configuration template
func (pcm *PropagationConfigManager) LoadConfigFromTemplate(templateName string, variables map[string]interface{}) (*PropagationConfig, error) {
	template, exists := BuiltInTemplates[templateName]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateName)
	}

	// Clone the template config
	configData, err := json.Marshal(template.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal template config: %w", err)
	}

	var config PropagationConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template config: %w", err)
	}

	// Apply variables if provided
	if len(variables) > 0 {
		if err := pcm.applyVariables(&config, variables); err != nil {
			return nil, fmt.Errorf("failed to apply variables: %w", err)
		}
	}

	return &config, nil
}

// SaveConfig saves a configuration to file
func (pcm *PropagationConfigManager) SaveConfig(name string, config *PropagationConfig, format string) error {
	if pcm.configDir == "" {
		return errors.New("no config directory specified")
	}

	// Ensure config directory exists using FileService
	if err := pcm.fileService.MkdirAll(pcm.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ignore requested format; always save as KDL
	filename := filepath.Join(pcm.configDir, name+".kdl")
	data, err := marshalPropagationConfigKDL(config)
	if err != nil {
		return fmt.Errorf("failed to marshal KDL: %w", err)
	}
	if err := pcm.fileService.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	pcm.configs[name] = config
	return nil
}

// ValidateConfig validates a configuration for consistency and correctness
func (pcm *PropagationConfigManager) ValidateConfig(config *PropagationConfig) error {
	// Validate global settings
	if config.MaxIterations <= 0 {
		return errors.New("max_iterations must be positive")
	}
	if config.ConvergenceThreshold <= 0 || config.ConvergenceThreshold >= 1 {
		return errors.New("convergence_threshold must be between 0 and 1")
	}
	if config.DefaultDecay <= 0 || config.DefaultDecay > 1 {
		return errors.New("default_decay must be between 0 and 1")
	}

	// Validate label rules
	for i, rule := range config.LabelRules {
		if err := pcm.validateLabelRule(rule); err != nil {
			return fmt.Errorf("label rule %d: %w", i, err)
		}
	}

	// Validate dependency rules
	for i, rule := range config.DependencyRules {
		if err := pcm.validateDependencyRule(rule); err != nil {
			return fmt.Errorf("dependency rule %d: %w", i, err)
		}
	}

	// Validate custom rules
	for i, rule := range config.CustomRules {
		if err := pcm.validateCustomRule(rule); err != nil {
			return fmt.Errorf("custom rule %d: %w", i, err)
		}
	}

	return nil
}

// validateLabelRule validates a single label propagation rule
func (pcm *PropagationConfigManager) validateLabelRule(rule LabelPropagationRule) error {
	if rule.Label == "" {
		return errors.New("label cannot be empty")
	}

	validDirections := map[string]bool{
		"upstream": true, "downstream": true, "bidirectional": true,
	}
	if !validDirections[rule.Direction] {
		return fmt.Errorf("invalid direction: %s", rule.Direction)
	}

	if rule.Decay <= 0 || rule.Decay > 1 {
		return errors.New("decay must be between 0 and 1")
	}

	if rule.MaxHops <= 0 {
		return errors.New("max_hops must be positive")
	}

	if rule.MinStrength < 0 || rule.MinStrength > 1 {
		return errors.New("min_strength must be between 0 and 1")
	}

	return nil
}

// validateDependencyRule validates a single dependency propagation rule
func (pcm *PropagationConfigManager) validateDependencyRule(rule DependencyPropagationRule) error {
	if rule.DependencyType == "" {
		return errors.New("dependency_type cannot be empty")
	}

	validDirections := map[string]bool{
		"upstream": true, "downstream": true, "bidirectional": true,
	}
	if !validDirections[rule.Direction] {
		return fmt.Errorf("invalid direction: %s", rule.Direction)
	}

	validAggregations := map[string]bool{
		"sum": true, "max": true, "unique": true, "concat": true, "weighted_sum": true,
	}
	if !validAggregations[rule.Aggregation] {
		return fmt.Errorf("invalid aggregation: %s", rule.Aggregation)
	}

	validWeightFunctions := map[string]bool{
		"linear": true, "exponential": true, "log": true,
	}
	if rule.WeightFunction != "" && !validWeightFunctions[rule.WeightFunction] {
		return fmt.Errorf("invalid weight_function: %s", rule.WeightFunction)
	}

	if rule.MaxDepth <= 0 {
		return errors.New("max_depth must be positive")
	}

	if rule.Threshold < 0 || rule.Threshold > 1 {
		return errors.New("threshold must be between 0 and 1")
	}

	return nil
}

// validateCustomRule validates a single custom propagation rule
func (pcm *PropagationConfigManager) validateCustomRule(rule CustomPropagationRule) error {
	if rule.Name == "" {
		return errors.New("name cannot be empty")
	}

	if rule.Trigger == "" {
		return errors.New("trigger cannot be empty")
	}

	if rule.Action == "" {
		return errors.New("action cannot be empty")
	}

	return nil
}

// MergeConfigs merges multiple configurations with priority handling
func (pcm *PropagationConfigManager) MergeConfigs(configs ...*PropagationConfig) *PropagationConfig {
	if len(configs) == 0 {
		return pcm.defaultConfig
	}

	if len(configs) == 1 {
		return configs[0]
	}

	// Start with first config
	merged := &PropagationConfig{
		MaxIterations:        configs[0].MaxIterations,
		ConvergenceThreshold: configs[0].ConvergenceThreshold,
		DefaultDecay:         configs[0].DefaultDecay,
		LabelRules:           make([]LabelPropagationRule, 0),
		DependencyRules:      make([]DependencyPropagationRule, 0),
		CustomRules:          make([]CustomPropagationRule, 0),
		AnalysisConfig:       configs[0].AnalysisConfig,
	}

	// Collect all rules with priority handling
	labelRules := make([]LabelPropagationRule, 0)
	dependencyRules := make([]DependencyPropagationRule, 0)
	customRules := make([]CustomPropagationRule, 0)

	for _, config := range configs {
		labelRules = append(labelRules, config.LabelRules...)
		dependencyRules = append(dependencyRules, config.DependencyRules...)
		customRules = append(customRules, config.CustomRules...)

		// Use highest values for global settings
		if config.MaxIterations > merged.MaxIterations {
			merged.MaxIterations = config.MaxIterations
		}
		if config.ConvergenceThreshold < merged.ConvergenceThreshold {
			merged.ConvergenceThreshold = config.ConvergenceThreshold
		}
	}

	// Sort rules by priority (higher priority first)
	pcm.sortRulesByPriority(labelRules, dependencyRules, customRules)

	merged.LabelRules = labelRules
	merged.DependencyRules = dependencyRules
	merged.CustomRules = customRules

	return merged
}

// GetAvailableTemplates returns a list of available configuration templates
func (pcm *PropagationConfigManager) GetAvailableTemplates() []string {
	templates := make([]string, 0, len(BuiltInTemplates))
	for name := range BuiltInTemplates {
		templates = append(templates, name)
	}
	return templates
}

// GetTemplateInfo returns information about a specific template
func (pcm *PropagationConfigManager) GetTemplateInfo(templateName string) (*ConfigTemplate, error) {
	template, exists := BuiltInTemplates[templateName]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateName)
	}
	return template, nil
}

// Helper functions

func (pcm *PropagationConfigManager) loadConfigFromFile(filename string) (*PropagationConfig, error) {
	data, err := pcm.fileService.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}
	var config PropagationConfig
	if err := unmarshalPropagationConfigKDL(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal KDL config from %s: %w", filename, err)
	}
	if err := pcm.ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", filename, err)
	}
	return &config, nil
}

func (pcm *PropagationConfigManager) applyVariables(config *PropagationConfig, variables map[string]interface{}) error {
	// Convert config to JSON for variable substitution
	configData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config for variable substitution: %w", err)
	}

	configStr := string(configData)

	// Simple variable substitution (could be enhanced with templating engine)
	for key, value := range variables {
		placeholder := fmt.Sprintf("${%s}", key)
		valueStr := fmt.Sprintf("%v", value)
		configStr = fmt.Sprintf(configStr, placeholder, valueStr)
	}

	// Parse back to config
	if err := json.Unmarshal([]byte(configStr), config); err != nil {
		return fmt.Errorf("failed to unmarshal config after variable substitution: %w", err)
	}
	return nil
}

func (pcm *PropagationConfigManager) sortRulesByPriority(labelRules []LabelPropagationRule, dependencyRules []DependencyPropagationRule, customRules []CustomPropagationRule) {
	// Sort label rules by priority (descending)
	for i := 0; i < len(labelRules)-1; i++ {
		for j := i + 1; j < len(labelRules); j++ {
			if labelRules[i].Priority < labelRules[j].Priority {
				labelRules[i], labelRules[j] = labelRules[j], labelRules[i]
			}
		}
	}

	// Sort custom rules by priority (descending)
	for i := 0; i < len(customRules)-1; i++ {
		for j := i + 1; j < len(customRules); j++ {
			if customRules[i].Priority < customRules[j].Priority {
				customRules[i], customRules[j] = customRules[j], customRules[i]
			}
		}
	}
}

// fileExists checks if a file exists using FileService when available
func (pcm *PropagationConfigManager) fileExists(filename string) bool {
	// Use FileService when available
	if pcm.fileService != nil {
		return pcm.fileService.Exists(filename)
	}

	// Fallback to direct filesystem access when FileService is not available
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}

// marshalPropagationConfigKDL converts a PropagationConfig into KDL document bytes
func marshalPropagationConfigKDL(cfg *PropagationConfig) ([]byte, error) {
	var b strings.Builder
	b.WriteString("config {\n")
	b.WriteString(fmt.Sprintf("  max_iterations %d\n", cfg.MaxIterations))
	b.WriteString(fmt.Sprintf("  convergence_threshold %f\n", cfg.ConvergenceThreshold))
	b.WriteString(fmt.Sprintf("  default_decay %f\n", cfg.DefaultDecay))

	// Label rules
	for _, r := range cfg.LabelRules {
		b.WriteString("  label_rule ")
		b.WriteString(fmt.Sprintf("label=\"%s\" direction=\"%s\" decay=%f max_hops=%d min_strength=%f priority=%d",
			r.Label, r.Direction, r.Decay, r.MaxHops, r.MinStrength, r.Priority))
		if r.Boost != 0 {
			b.WriteString(fmt.Sprintf(" boost=%f", r.Boost))
		}
		if len(r.Conditions) > 0 {
			b.WriteString(fmt.Sprintf(" conditions=\"%s\"", strings.Join(r.Conditions, ",")))
		}
		if r.IncludeTypeHierarchy {
			b.WriteString(" include_type_hierarchy=true")
		}
		b.WriteString("\n")
	}

	for _, r := range cfg.DependencyRules {
		b.WriteString("  dependency_rule ")
		b.WriteString(fmt.Sprintf("dependency_type=\"%s\" direction=\"%s\" aggregation=\"%s\" max_depth=%d threshold=%f",
			r.DependencyType, r.Direction, r.Aggregation, r.MaxDepth, r.Threshold))
		if r.WeightFunction != "" {
			b.WriteString(fmt.Sprintf(" weight_function=\"%s\"", r.WeightFunction))
		}
		b.WriteString("\n")
	}

	for _, r := range cfg.CustomRules {
		b.WriteString("  custom_rule ")
		b.WriteString(fmt.Sprintf("name=\"%s\" trigger=\"%s\" action=\"%s\" priority=%d",
			r.Name, escapeKDLString(r.Trigger), escapeKDLString(r.Action), r.Priority))
		if r.Description != "" {
			b.WriteString(fmt.Sprintf(" description=\"%s\"", escapeKDLString(r.Description)))
		}
		b.WriteString("\n")
	}

	b.WriteString("  analysis detect_entry_points=")
	b.WriteString(boolToKDL(cfg.AnalysisConfig.DetectEntryPoints))
	b.WriteString(" calculate_depth=")
	b.WriteString(boolToKDL(cfg.AnalysisConfig.CalculateDepth))
	b.WriteString(" find_critical_paths=")
	b.WriteString(boolToKDL(cfg.AnalysisConfig.FindCriticalPaths))
	b.WriteString(fmt.Sprintf(" high_impact_threshold=%f\n", cfg.AnalysisConfig.HighImpactThreshold))

	if len(cfg.AnalysisConfig.EntryPointLabels) > 0 {
		b.WriteString("  entry_point_labels ")
		for _, l := range cfg.AnalysisConfig.EntryPointLabels {
			b.WriteString(fmt.Sprintf("\"%s\" ", l))
		}
		b.WriteString("\n")
	}
	if len(cfg.AnalysisConfig.CriticalLabels) > 0 {
		b.WriteString("  critical_labels ")
		for _, l := range cfg.AnalysisConfig.CriticalLabels {
			b.WriteString(fmt.Sprintf("\"%s\" ", l))
		}
		b.WriteString("\n")
	}

	b.WriteString("}\n")
	return []byte(b.String()), nil
}

// unmarshalPropagationConfigKDL parses a minimal KDL form into PropagationConfig
func unmarshalPropagationConfigKDL(data []byte, cfg *PropagationConfig) error {
	doc, err := kdl.Parse(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	// start from defaults
	*cfg = *getDefaultPropagationConfig()
	for _, n := range doc.Nodes {
		if nodeName(n) == "config" {
			for _, cn := range n.Children { // children entries
				name := nodeName(cn)
				switch name {
				case "max_iterations":
					if iv, ok := firstIntArg(cn); ok {
						cfg.MaxIterations = iv
					}
				case "convergence_threshold":
					if fv, ok := firstFloatArg(cn); ok {
						cfg.ConvergenceThreshold = fv
					}
				case "default_decay":
					if fv, ok := firstFloatArg(cn); ok {
						cfg.DefaultDecay = fv
					}
				case "label_rule":
					lr := LabelPropagationRule{}
					if s, ok := propString(cn, "label"); ok {
						lr.Label = s
					}
					if s, ok := propString(cn, "direction"); ok {
						lr.Direction = s
					}
					if fv, ok := propFloat(cn, "decay"); ok {
						lr.Decay = fv
					}
					if iv, ok := propInt(cn, "max_hops"); ok {
						lr.MaxHops = iv
					}
					if fv, ok := propFloat(cn, "min_strength"); ok {
						lr.MinStrength = fv
					}
					if fv, ok := propFloat(cn, "boost"); ok {
						lr.Boost = fv
					}
					if s, ok := propString(cn, "conditions"); ok && s != "" {
						lr.Conditions = strings.Split(s, ",")
					}
					if iv, ok := propInt(cn, "priority"); ok {
						lr.Priority = iv
					}
					if bv, ok := propBool(cn, "include_type_hierarchy"); ok {
						lr.IncludeTypeHierarchy = bv
					}
					cfg.LabelRules = append(cfg.LabelRules, lr)
				case "dependency_rule":
					dr := DependencyPropagationRule{}
					if s, ok := propString(cn, "dependency_type"); ok {
						dr.DependencyType = s
					}
					if s, ok := propString(cn, "direction"); ok {
						dr.Direction = s
					}
					if s, ok := propString(cn, "aggregation"); ok {
						dr.Aggregation = s
					}
					if s, ok := propString(cn, "weight_function"); ok {
						dr.WeightFunction = s
					}
					if iv, ok := propInt(cn, "max_depth"); ok {
						dr.MaxDepth = iv
					}
					if fv, ok := propFloat(cn, "threshold"); ok {
						dr.Threshold = fv
					}
					cfg.DependencyRules = append(cfg.DependencyRules, dr)
				case "custom_rule":
					cr := CustomPropagationRule{}
					if s, ok := propString(cn, "name"); ok {
						cr.Name = s
					}
					if s, ok := propString(cn, "description"); ok {
						cr.Description = s
					}
					if s, ok := propString(cn, "trigger"); ok {
						cr.Trigger = s
					}
					if s, ok := propString(cn, "action"); ok {
						cr.Action = s
					}
					if iv, ok := propInt(cn, "priority"); ok {
						cr.Priority = iv
					}
					cfg.CustomRules = append(cfg.CustomRules, cr)
				case "analysis":
					if bv, ok := propBool(cn, "detect_entry_points"); ok {
						cfg.AnalysisConfig.DetectEntryPoints = bv
					}
					if bv, ok := propBool(cn, "calculate_depth"); ok {
						cfg.AnalysisConfig.CalculateDepth = bv
					}
					if bv, ok := propBool(cn, "find_critical_paths"); ok {
						cfg.AnalysisConfig.FindCriticalPaths = bv
					}
					if fv, ok := propFloat(cn, "high_impact_threshold"); ok {
						cfg.AnalysisConfig.HighImpactThreshold = fv
					}
				case "entry_point_labels":
					cfg.AnalysisConfig.EntryPointLabels = collectStringArgs(cn)
				case "critical_labels":
					cfg.AnalysisConfig.CriticalLabels = collectStringArgs(cn)
				}
			}
		}
	}
	return nil
}

// Helper utilities using kdl-go document model
func nodeName(n *document.Node) string {
	if n == nil || n.Name == nil {
		return ""
	}
	return n.Name.NodeNameString()
}
func firstIntArg(n *document.Node) (int, bool) {
	if len(n.Arguments) == 0 {
		return 0, false
	}
	switch v := n.Arguments[0].Value.(type) {
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
func firstFloatArg(n *document.Node) (float64, bool) {
	if len(n.Arguments) == 0 {
		return 0, false
	}
	switch v := n.Arguments[0].Value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}
func propString(n *document.Node, key string) (string, bool) {
	if n.Properties == nil {
		return "", false
	}
	if v, ok := n.Properties[key]; ok {
		if s, ok2 := v.Value.(string); ok2 {
			return s, true
		}
	}
	return "", false
}
func propInt(n *document.Node, key string) (int, bool) {
	if n.Properties == nil {
		return 0, false
	}
	if v, ok := n.Properties[key]; ok {
		switch val := v.Value.(type) {
		case int64:
			return int(val), true
		case float64:
			return int(val), true
		}
	}
	return 0, false
}
func propFloat(n *document.Node, key string) (float64, bool) {
	if n.Properties == nil {
		return 0, false
	}
	if v, ok := n.Properties[key]; ok {
		switch val := v.Value.(type) {
		case float64:
			return val, true
		case int64:
			return float64(val), true
		}
	}
	return 0, false
}
func propBool(n *document.Node, key string) (bool, bool) {
	if n.Properties == nil {
		return false, false
	}
	if v, ok := n.Properties[key]; ok {
		if b, ok2 := v.Value.(bool); ok2 {
			return b, true
		}
	}
	return false, false
}
func collectStringArgs(n *document.Node) []string {
	if n == nil {
		return nil
	}
	out := make([]string, 0, len(n.Arguments))
	for _, a := range n.Arguments {
		if s, ok := a.Value.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
func boolToKDL(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func escapeKDLString(s string) string { return strings.ReplaceAll(s, "\"", "\\\"") }
