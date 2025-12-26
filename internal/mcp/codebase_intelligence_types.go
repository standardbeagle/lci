package mcp

import (
	"encoding/json"
	"time"
)

// ============================================================================
// High-Level Overview Types (Tier 1)
// ============================================================================

// RepositoryMap represents the high-level view of the codebase structure
type RepositoryMap struct {
	// Tier 1: Must-have for agents
	CriticalFunctions []FunctionSignature `json:"critical_functions"`
	ModuleBoundaries  []ModuleBoundary    `json:"module_boundaries"`
	DomainTerms       []DomainTerm        `json:"domain_terms"`
	EntryPoints       []EntryPoint        `json:"entry_points"`

	// Analysis metadata
	AnalyzedAt     time.Time `json:"analyzed_at"`
	TotalFiles     int       `json:"total_files"`
	TotalFunctions int       `json:"total_functions"`
	TotalSymbols   int       `json:"total_symbols"`

	// Navigation guidance
	Note string `json:"note,omitempty"` // Navigation instructions for EntityID usage
}

// FunctionSignature represents a critical function with minimal context
type FunctionSignature struct {
	// ObjectID is a compact base-63 encoded ID for use with get_context drill-down.
	// This is the preferred ID format for API responses as it's shorter than EntityID.
	// Use get_context with {\"id\": \"<object_id>\"} to retrieve full symbol details.
	ObjectID string `json:"oid,omitempty"` // Compact ID for drill-down (base-63 encoded)

	EntityID        string  `json:"entity_id"` // Unified entity ID for drill-down (verbose format)
	Name            string  `json:"name"`
	Module          string  `json:"module"`
	Signature       string  `json:"signature"`
	ImportanceScore float64 `json:"importance_score"` // PageRank-based score
	ReferencedBy    int     `json:"referenced_by"`    // Number of references
	SymbolType      string  `json:"symbol_type"`      // "function", "method", "constructor"
	IsExported      bool    `json:"is_exported"`

	// Location information for context
	FileID   string `json:"file_id"`  // File entity ID
	Location string `json:"location"` // "file.go:line:column"
}

// ModuleBoundary represents a detected module with basic metrics
type ModuleBoundary struct {
	EntityID      string  `json:"entity_id"` // Unified module entity ID
	Name          string  `json:"name"`
	Type          string  `json:"type"` // "core", "application", "library", "test"
	Path          string  `json:"path"`
	CohesionScore float64 `json:"cohesion_score"` // 0.0-1.0
	CouplingScore float64 `json:"coupling_score"` // 0.0-1.0
	Stability     float64 `json:"stability"`      // 0.0-1.0 (Martin metric)
	FileCount     int     `json:"file_count"`
	FunctionCount int     `json:"function_count"`

	// Entity collections for drill-down
	FileIDs     []string `json:"file_ids"`     // All file entity IDs in module
	FunctionIDs []string `json:"function_ids"` // All function entity IDs in module
	ClassIDs    []string `json:"class_ids"`    // All class/struct entity IDs in module

	// Representative examples (first 5 of each type)
	ExampleFunctions []SymbolRef `json:"example_functions"`
	ExampleClasses   []SymbolRef `json:"example_classes"`
}

// SymbolRef represents a symbol reference for quick browsing and drill-down
type SymbolRef struct {
	// ObjectID is a compact base-63 encoded ID for use with get_context drill-down.
	ObjectID string `json:"oid,omitempty"` // Compact ID for drill-down (base-63 encoded)

	EntityID   string `json:"entity_id"` // Unified symbol entity ID (verbose format)
	Name       string `json:"name"`
	SymbolType string `json:"symbol_type"`          // "function", "method", "struct", "interface", etc.
	Location   string `json:"location"`             // "file.go:line:column"
	FileID     string `json:"file_id"`              // File entity ID
	Complexity int    `json:"complexity,omitempty"` // Optional complexity score
}

// DomainTerm represents a domain-specific term cluster
type DomainTerm struct {
	Domain     string   `json:"domain"`
	Terms      []string `json:"terms"`
	Confidence float64  `json:"confidence"` // 0.0-1.0
	Count      int      `json:"count"`      // Occurrences
}

// EntryPoint represents a codebase entry point
type EntryPoint struct {
	// ObjectID is a compact base-63 encoded ID for use with get_context drill-down.
	// This is the preferred ID format for API responses as it's shorter than EntityID.
	ObjectID string `json:"oid,omitempty"` // Compact ID for drill-down (base-63 encoded)

	EntityID   string  `json:"entity_id"` // Unified entry point entity ID (verbose format)
	Name       string  `json:"name"`
	Type       string  `json:"type"` // "main", "api", "handler", "test"
	Location   string  `json:"location"`
	Signature  string  `json:"signature"`
	IsExported bool    `json:"is_exported"`
	FileID     string  `json:"file_id"`    // File entity ID
	Importance float64 `json:"importance"` // Importance score for ranking
}

// DependencyGraph represents module and symbol dependencies
type DependencyGraph struct {
	Nodes                []DependencyNode     `json:"nodes"`
	Edges                []DependencyEdge     `json:"edges"`
	CircularDependencies []CircularDependency `json:"circular_dependencies"`
	LayerViolations      []LayerViolation     `json:"layer_violations"`
	CouplingHotspots     []CouplingHotspot    `json:"coupling_hotspots"`
	HighestCentrality    []string             `json:"highest_centrality"` // Entity IDs of most central nodes
	AnalysisMetadata     AnalysisMetadata     `json:"analysis_metadata"`
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	EntityID   string  `json:"entity_id"` // Unified entity ID for drill-down
	Name       string  `json:"name"`
	Type       string  `json:"type"`       // "module", "symbol"
	Centrality float64 `json:"centrality"` // Betweenness centrality score
}

// DependencyEdge represents a dependency relationship
type DependencyEdge struct {
	FromEntityID string  `json:"from_entity_id"` // Source entity ID
	ToEntityID   string  `json:"to_entity_id"`   // Target entity ID
	Weight       float64 `json:"weight"`         // Coupling strength (0.0-1.0)
	Type         string  `json:"type"`           // "import", "call", "reference"
}

// CircularDependency represents a circular dependency
type CircularDependency struct {
	ModuleEntityIDs []string `json:"module_entity_ids"` // Module entity IDs involved in cycle
	Severity        string   `json:"severity"`          // "high", "medium", "low"
	Description     string   `json:"description"`       // Human-readable description
}

// LayerViolation represents an architectural layer violation
type LayerViolation struct {
	FromEntityID string `json:"from_entity_id"` // Source module/entity ID
	ToEntityID   string `json:"to_entity_id"`   // Target module/entity ID
	Severity     string `json:"severity"`
	Description  string `json:"description"`
	Example      string `json:"example,omitempty"`
}

// CouplingHotspot represents a high-coupling module
type CouplingHotspot struct {
	ModuleEntityID    string   `json:"module_entity_id"` // Module entity ID
	ModuleName        string   `json:"module_name"`
	CouplingScore     float64  `json:"coupling_score"`
	AffectedCount     int      `json:"affected_count"`
	AffectedEntityIDs []string `json:"affected_entity_ids"` // Entity IDs of affected modules
}

// HealthDashboard represents codebase health metrics
type HealthDashboard struct {
	OverallScore     float64              `json:"overall_score"` // 0.0-10.0
	Grade            string               `json:"grade"`         // A, B, C, D, F
	Complexity       ComplexityMetrics    `json:"complexity"`
	TechnicalDebt    TechnicalDebtMetrics `json:"technical_debt"`
	Hotspots         []Hotspot            `json:"hotspots"`
	AnalysisMetadata AnalysisMetadata     `json:"analysis_metadata"`

	// Detailed quality issues for actionable insights
	DetailedSmells     []CodeSmellEntry    `json:"detailed_smells,omitempty"`     // Top code smells above severity cutoff
	ProblematicSymbols []ProblematicSymbol `json:"problematic_symbols,omitempty"` // High-risk symbols above cutoff
	SmellCounts        map[string]int      `json:"smell_counts,omitempty"`        // Count by smell type

	// Performance anti-patterns detected during AST analysis
	PerformancePatterns *PerformanceAnalysis `json:"performance_patterns,omitempty"`

	// Memory allocation analysis with PageRank-style propagation
	MemoryAnalysis *MemoryPressureAnalysis `json:"memory_analysis,omitempty"`

	// Function purity summary (side effect analysis overview)
	// Use side_effects tool for detailed queries (pure/impure lists, by-category filtering)
	PuritySummary *PuritySummary `json:"purity_summary,omitempty"`
}

// PuritySummary provides an overview of function purity in the codebase
// For detailed queries, use the side_effects tool with modes: symbol, file, pure, impure, category
type PuritySummary struct {
	TotalFunctions  int     `json:"total_funcs"`  // Total functions analyzed
	PureFunctions   int     `json:"pure_funcs"`   // Functions with no side effects
	ImpureFunctions int     `json:"impure_funcs"` // Functions with side effects
	PurityRatio     float64 `json:"purity_ratio"` // 0.0-1.0, higher is better
	Grade           string  `json:"grade"`        // A (>80%), B (>60%), C (>40%), D (>20%), F

	// Category breakdown (count of functions with each effect type)
	WithParamWrites   int `json:"with_param_writes,omitempty"`   // Mutate parameters
	WithGlobalWrites  int `json:"with_global_writes,omitempty"`  // Write to globals
	WithIOEffects     int `json:"with_io_effects,omitempty"`     // File/network/database I/O
	WithThrows        int `json:"with_throws,omitempty"`         // Can throw/panic
	WithExternalCalls int `json:"with_external_calls,omitempty"` // Call unknown functions

	// Navigation hint for detailed analysis
	DetailedQuery string `json:"detailed_query,omitempty"` // Example query for side_effects tool
}

// MemoryPressureAnalysis contains memory allocation analysis results
type MemoryPressureAnalysis struct {
	Scores   []MemoryScore   `json:"scores"`   // Ranked functions by memory pressure
	Summary  MemorySummary   `json:"summary"`  // Aggregate statistics
	Hotspots []MemoryHotspot `json:"hotspots"` // High pressure locations
}

// MemoryScore represents a function's memory pressure score
type MemoryScore struct {
	Function        string  `json:"func"`          // Function name
	Location        string  `json:"loc"`           // "file.go:line"
	TotalScore      float64 `json:"score"`         // Combined score
	DirectScore     float64 `json:"direct"`        // From direct allocations
	PropagatedScore float64 `json:"propagated"`    // From PageRank propagation
	LoopPressure    float64 `json:"loop_pressure"` // Allocations in loops
	Severity        string  `json:"sev"`           // "critical", "high", "medium", "low"
	Percentile      float64 `json:"percentile"`    // Ranking 0-100
}

// MemorySummary provides aggregate memory statistics
type MemorySummary struct {
	TotalFunctions   int     `json:"total_funcs"`
	TotalAllocations int     `json:"total_allocs"`
	AvgAllocPerFunc  float64 `json:"avg_alloc"`
	LoopAllocCount   int     `json:"loop_allocs"` // Allocations in loops
	CriticalCount    int     `json:"critical"`
	HighCount        int     `json:"high"`
	MediumCount      int     `json:"medium"`
	LowCount         int     `json:"low"`
}

// MemoryHotspot identifies a high memory pressure location
type MemoryHotspot struct {
	Function   string  `json:"func"`
	Location   string  `json:"loc"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	Suggestion string  `json:"suggestion"`
}

// CodeSmellEntry represents a detected code smell with drill-down capability
type CodeSmellEntry struct {
	Type        string `json:"type"`   // "long-function", "high-complexity", "deep-nesting", "god-class", "shotgun-surgery"
	Symbol      string `json:"symbol"` // Symbol name
	ObjectID    string `json:"oid"`    // For drill-down via get_context
	Location    string `json:"loc"`    // "file.go:line"
	Severity    string `json:"sev"`    // "high", "medium"
	Description string `json:"desc"`   // Human-readable description
}

// ProblematicSymbol represents a symbol with quality issues and drill-down capability
type ProblematicSymbol struct {
	ObjectID  string   `json:"oid"`            // For drill-down via get_context
	Name      string   `json:"name"`           // Symbol name
	Location  string   `json:"loc"`            // "file.go:line"
	RiskScore int      `json:"risk"`           // 0-10 risk score
	Tags      []string `json:"tags,omitempty"` // ["HIGH_COMPLEXITY", "DEEP_NESTING", ...]
}

// ComplexityMetrics represents complexity distribution
type ComplexityMetrics struct {
	AverageCC           float64            `json:"average_cyclomatic_complexity"`
	MedianCC            float64            `json:"median_cyclomatic_complexity"`
	Percentiles         map[string]float64 `json:"percentiles"` // P50, P75, P90, P95, P99
	HighComplexityFuncs []FunctionInfo     `json:"high_complexity_functions"`
	Distribution        map[string]int     `json:"distribution"` // Count by CC range
}

// TechnicalDebtMetrics represents technical debt analysis
type TechnicalDebtMetrics struct {
	Ratio      float64  `json:"ratio"`      // 0.0-1.0
	Grade      string   `json:"grade"`      // A, B, C, D, F
	Estimate   string   `json:"estimate"`   // e.g., "2 weeks"
	Components []string `json:"components"` // Major debt contributors
}

// Hotspot represents a code hotspot (high complexity area)
type Hotspot struct {
	Location   string  `json:"location"`
	Complexity float64 `json:"complexity"` // Cyclomatic complexity
	RiskScore  float64 `json:"risk_score"` // Risk based on complexity and size
}

// AnalysisMetadata represents metadata about the analysis
type AnalysisMetadata struct {
	AnalysisTimeMs int       `json:"analysis_time_ms"`
	FilesAnalyzed  int       `json:"files_analyzed"`
	AnalyzedAt     time.Time `json:"analyzed_at"`
	IndexVersion   string    `json:"index_version"`
}

// ============================================================================
// Detailed Analysis Types (Tier 2)
// ============================================================================

// ModuleAnalysis represents detailed module analysis
type ModuleAnalysis struct {
	Modules           []ModuleBoundary      `json:"modules"`
	DetectionStrategy string                `json:"detection_strategy"`
	ModuleTypes       map[string]int        `json:"module_types"`
	LayerDistribution map[string][]string   `json:"layer_distribution"`
	Violations        []Violation           `json:"violations"`
	Metrics           ModuleAnalysisMetrics `json:"metrics"`
}

// ModuleAnalysisMetrics represents module analysis metrics
type ModuleAnalysisMetrics struct {
	TotalModules       int     `json:"total_modules"`
	AverageCohesion    float64 `json:"average_cohesion"`
	AverageCoupling    float64 `json:"average_coupling"`
	ArchitecturalScore float64 `json:"architectural_score"`
}

// LayerAnalysis represents detailed layer analysis
type LayerAnalysis struct {
	Layers           []ArchitecturalLayer `json:"layers"`
	ViolationCount   int                  `json:"violation_count"`
	LayerMetrics     []LayerMetrics       `json:"layer_metrics"`
	DependencyMatrix [][]float64          `json:"dependency_matrix"`
	Patterns         []LayerPattern       `json:"patterns"`
}

// ArchitecturalLayer represents a detected architectural layer
type ArchitecturalLayer struct {
	Name           string       `json:"name"`
	Modules        []string     `json:"modules"`
	Depth          int          `json:"depth"` // Layer depth (1 = top level)
	ComponentTypes []string     `json:"component_types"`
	Metrics        LayerMetrics `json:"metrics"`
}

// LayerMetrics represents metrics for a layer
type LayerMetrics struct {
	ModuleCount     int     `json:"module_count"`
	CohesionScore   float64 `json:"cohesion_score"`
	CouplingScore   float64 `json:"coupling_score"`
	Maintainability float64 `json:"maintainability"`
	Complexity      float64 `json:"complexity"`
}

// LayerPattern represents detected architectural patterns
type LayerPattern struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"`
	Violations  []string `json:"violations"`
}

// FeatureAnalysis represents feature location analysis
type FeatureAnalysis struct {
	Features         []Feature              `json:"features"`
	FeatureMap       map[string]string      `json:"feature_map"` // feature_name -> primary_module
	CrossFeatureDeps []FeatureDependency    `json:"cross_feature_deps"`
	OrphanComponents []ComponentInfo        `json:"orphan_components"`
	Metrics          FeatureAnalysisMetrics `json:"metrics"`
}

// Feature represents a detected feature
type Feature struct {
	Name          string          `json:"name"`
	PrimaryModule string          `json:"primary_module"`
	Components    []ComponentInfo `json:"components"`
	APIs          []APIEndpoint   `json:"apis"`
	Tests         []TestInfo      `json:"tests"`
	Confidence    float64         `json:"confidence"`
}

// ComponentInfo represents a code component
type ComponentInfo struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "class", "interface", "service", etc.
	Location     string   `json:"location"`
	Complexity   float64  `json:"complexity"`
	Dependencies []string `json:"dependencies"`
}

// APIEndpoint represents an API endpoint
type APIEndpoint struct {
	Method   string `json:"method"` // GET, POST, etc.
	Path     string `json:"path"`
	Handler  string `json:"handler"`
	Module   string `json:"module"`
	Exported bool   `json:"exported"`
}

// TestInfo represents a test
type TestInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "unit", "integration", "e2e"
	Location string `json:"location"`
	Feature  string `json:"feature"`
}

// FeatureDependency represents cross-feature dependencies
type FeatureDependency struct {
	FeatureA string  `json:"feature_a"`
	FeatureB string  `json:"feature_b"`
	Type     string  `json:"type"`     // "data", "control", "temporal"
	Strength float64 `json:"strength"` // 0.0-1.0
}

// FeatureAnalysisMetrics represents feature analysis metrics
type FeatureAnalysisMetrics struct {
	TotalFeatures     int     `json:"total_features"`
	AverageComponents float64 `json:"average_components"`
	CouplingScore     float64 `json:"coupling_score"`
	ModularityScore   float64 `json:"modularity_score"`
}

// TermClusterAnalysis represents term clustering analysis
type TermClusterAnalysis struct {
	Clusters     []TermCluster      `json:"clusters"`
	DomainModels []DomainModel      `json:"domain_models"`
	Vocabulary   map[string]float64 `json:"vocabulary"`
	KeyTerms     []KeyTerm          `json:"key_terms"`
	TopDomains   []DomainSummary    `json:"top_domains"`
	Metrics      TermClusterMetrics `json:"metrics"`
}

// TermCluster represents a cluster of related terms
type TermCluster struct {
	ClusterID   int      `json:"cluster_id"`
	Domain      string   `json:"domain"`
	Terms       []string `json:"terms"`
	Centroid    string   `json:"centroid"` // Most representative term
	Strength    float64  `json:"strength"` // 0.0-1.0
	MemberCount int      `json:"member_count"`
}

// DomainModel represents a domain model
type DomainModel struct {
	Domain         string               `json:"domain"`
	Concepts       []DomainConcept      `json:"concepts"`
	Relationships  []DomainRelationship `json:"relationships"`
	VocabularySize int                  `json:"vocabulary_size"`
}

// DomainConcept represents a concept in a domain
type DomainConcept struct {
	Name       string   `json:"name"`
	Terms      []string `json:"terms"`
	Definition string   `json:"definition,omitempty"`
	Confidence float64  `json:"confidence"`
}

// DomainRelationship represents relationships between concepts
type DomainRelationship struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Type     string  `json:"type"` // "is-a", "part-of", "uses", "depends-on"
	Strength float64 `json:"strength"`
}

// KeyTerm represents a significant term
type KeyTerm struct {
	Term       string  `json:"term"`
	Frequency  int     `json:"frequency"`
	TFIDFScore float64 `json:"tfidf_score"`
	Domain     string  `json:"domain"`
	Category   string  `json:"category"` // "type", "function", "concept"
}

// DomainSummary represents a domain summary
type DomainSummary struct {
	Domain         string  `json:"domain"`
	TermCount      int     `json:"term_count"`
	ConceptCount   int     `json:"concept_count"`
	Confidence     float64 `json:"confidence"`
	Representative string  `json:"representative"` // Most common term
}

// TermClusterMetrics represents clustering metrics
type TermClusterMetrics struct {
	TotalClusters      int     `json:"total_clusters"`
	AverageClusterSize float64 `json:"average_cluster_size"`
	Coverage           float64 `json:"coverage"` // % of terms clustered
	Quality            float64 `json:"quality"`  // 0.0-1.0
}

// Violation represents a detected violation
type Violation struct {
	Type            string `json:"type"` // "layer", "dependency", "architectural"
	Severity        string `json:"severity"`
	Description     string `json:"description"`
	Location        string `json:"location"`                    // Human-readable location
	EntityID        string `json:"entity_id"`                   // Entity ID where violation occurs
	RelatedEntityID string `json:"related_entity_id,omitempty"` // Related entity ID if applicable
	Suggestion      string `json:"suggestion,omitempty"`
}

// FunctionInfo represents function information
type FunctionInfo struct {
	// ObjectID is a compact base-63 encoded ID for use with get_context drill-down.
	ObjectID string `json:"oid,omitempty"` // Compact ID for drill-down (base-63 encoded)

	EntityID   string  `json:"entity_id"` // Unified function entity ID (verbose format)
	Name       string  `json:"name"`
	Location   string  `json:"location"` // "file.go:line:column"
	FileID     string  `json:"file_id"`  // File entity ID
	Complexity float64 `json:"complexity"`
	LineCount  int     `json:"line_count"`
}

// ============================================================================
// Code Statistics Types (Tier 3)
// ============================================================================

// StatisticsReport represents comprehensive code statistics
type StatisticsReport struct {
	ComplexityMetrics ComplexityMetrics `json:"complexity_metrics,omitempty"`
	CouplingMetrics   CouplingMetrics   `json:"coupling_metrics,omitempty"`
	CohesionMetrics   CohesionMetrics   `json:"cohesion_metrics,omitempty"`
	ChangeMetrics     ChangeMetrics     `json:"change_metrics,omitempty"`
	QualityMetrics    QualityMetrics    `json:"quality_metrics,omitempty"`

	// Distributions
	ComplexityDistribution map[string]int `json:"complexity_distribution,omitempty"`
	CouplingDistribution   map[string]int `json:"coupling_distribution,omitempty"`
	LayerSizeDistribution  map[string]int `json:"layer_size_distribution,omitempty"`

	// Comparisons
	AgainstIndustryBenchmarks IndustryComparison   `json:"industry_benchmarks,omitempty"`
	HistoricalComparison      HistoricalComparison `json:"historical_comparison,omitempty"`

	// Git analysis (for git modes)
	GitAnalysis json.RawMessage `json:"git_analysis,omitempty"` // git_analyze mode results
	GitHotspots json.RawMessage `json:"git_hotspots,omitempty"` // git_hotspots mode results

	AnalysisMetadata AnalysisMetadata `json:"analysis_metadata,omitempty"`
}

// CouplingMetrics represents coupling analysis
type CouplingMetrics struct {
	AfferentCoupling map[string]int     `json:"afferent_coupling"` // Ca
	EfferentCoupling map[string]int     `json:"efferent_coupling"` // Ce
	Instability      map[string]float64 `json:"instability"`       // I = Ce/(Ca+Ce)
	Abstractness     map[string]float64 `json:"abstractness"`      // A = Abstract/Total
	Distance         map[string]float64 `json:"distance"`          // D = |A+I-1|

	// Module-level
	ModuleCoupling map[string]float64 `json:"module_coupling"`
	LayerCoupling  map[string]float64 `json:"layer_coupling"`

	// Metrics
	AverageCoupling      float64              `json:"average_coupling"`
	MaxCoupling          float64              `json:"max_coupling"`
	HighCouplingModules  []string             `json:"high_coupling_modules"`
	CircularDependencies []CircularDependency `json:"circular_dependencies"`
}

// CohesionMetrics represents cohesion analysis
type CohesionMetrics struct {
	// Module cohesion
	RelationalCohesion map[string]float64 `json:"relational_cohesion"`
	FunctionalCohesion map[string]float64 `json:"functional_cohesion"`
	SequentialCohesion map[string]float64 `json:"sequential_cohesion"`

	// Layer cohesion
	LayerCohesion map[string]float64 `json:"layer_cohesion"`

	// Metrics
	AverageCohesion    float64  `json:"average_cohesion"`
	MinCohesion        float64  `json:"min_cohesion"`
	LowCohesionModules []string `json:"low_cohesion_modules"`
}

// ChangeMetrics represents change/risk analysis
// Note: Git-based metrics (churn, frequency) require git integration - not currently implemented
type ChangeMetrics struct {
	Hotspots    []Hotspot    `json:"hotspots"`     // Complexity-based hotspots
	RiskFactors []RiskFactor `json:"risk_factors"` // Static analysis risk factors
}

// QualityMetrics represents quality analysis
type QualityMetrics struct {
	TechnicalDebtRatio     float64 `json:"technical_debt_ratio"`
	CodeSmells             int     `json:"code_smells"`
	ArchitectureViolations int     `json:"architecture_violations"`
	DuplicationRatio       float64 `json:"duplication_ratio"`
	CommentCoverage        float64 `json:"comment_coverage"`

	// Scores
	MaintainabilityIndex float64 `json:"maintainability_index"`
	ReliabilityScore     float64 `json:"reliability_score"`
	SecurityScore        float64 `json:"security_score"`
	PerformanceScore     float64 `json:"performance_score"`

	// Grading
	Grade             string         `json:"grade"` // A, B, C, D, F
	GradeDistribution map[string]int `json:"grade_distribution"`
}

// RiskFactor represents a risk factor
type RiskFactor struct {
	Type        string  `json:"type"` // "complexity", "coupling", "change", "debt"
	Description string  `json:"description"`
	Location    string  `json:"location"`
	Score       float64 `json:"score"`  // 0.0-1.0
	Impact      string  `json:"impact"` // "high", "medium", "low"
}

// IndustryComparison represents comparison with industry benchmarks
type IndustryComparison struct {
	ComplexityPercentile float64 `json:"complexity_percentile"` // 0.0-100.0
	CouplingPercentile   float64 `json:"coupling_percentile"`
	CohesionPercentile   float64 `json:"cohesion_percentile"`
	QualityPercentile    float64 `json:"quality_percentile"`
	IndustryStandard     string  `json:"industry_standard"`
	ComparisonDate       string  `json:"comparison_date"`
}

// HistoricalComparison represents comparison with historical data
type HistoricalComparison struct {
	PreviousVersion  string             `json:"previous_version"`
	CurrentVersion   string             `json:"current_version"`
	ChangePercentage float64            `json:"change_percentage"`
	Metrics          map[string]float64 `json:"metrics"`
	Trends           map[string]string  `json:"trends"` // "improving", "degrading", "stable"
}

// EntryPointsList represents a list of entry points (renamed to avoid conflict with EntryPoint type)
type EntryPointsList struct {
	MainFunctions []EntryPoint `json:"main_functions"`
}

// ============================================================================
// Unified Response Type
// ============================================================================

// CodebaseIntelligenceResponse represents a unified response
type CodebaseIntelligenceResponse struct {
	// Mode-specific data
	RepositoryMap   *RepositoryMap   `json:"repository_map,omitempty"`
	DependencyGraph *DependencyGraph `json:"dependency_graph,omitempty"`
	HealthDashboard *HealthDashboard `json:"health_dashboard,omitempty"`
	EntryPoints     *EntryPointsList `json:"entry_points,omitempty"`

	// Detailed analysis
	ModuleAnalysis      *ModuleAnalysis      `json:"module_analysis,omitempty"`
	LayerAnalysis       *LayerAnalysis       `json:"layer_analysis,omitempty"`
	FeatureAnalysis     *FeatureAnalysis     `json:"feature_analysis,omitempty"`
	TermClusterAnalysis *TermClusterAnalysis `json:"term_cluster_analysis,omitempty"`

	// Structure analysis (for exploration mode)
	StructureAnalysis *StructureAnalysis `json:"structure_analysis,omitempty"`

	// Type hierarchy analysis (for type_hierarchy mode)
	TypeHierarchyAnalysis *TypeHierarchyAnalysis `json:"type_hierarchy_analysis,omitempty"`

	// Vocabulary analysis (semantic)
	SemanticVocabulary *SemanticVocabulary `json:"semantic_vocabulary,omitempty"`

	// Statistics
	StatisticsReport *StatisticsReport `json:"statistics_report,omitempty"`

	// Metadata
	AnalysisMode     string           `json:"analysis_mode"` // "overview", "detailed", "statistics", "unified", "structure"
	Tier             int              `json:"tier"`          // 1, 2, 3
	AnalysisMetadata AnalysisMetadata `json:"analysis_metadata"`

	// Navigation guidance
	NavigationHints map[string]string `json:"navigation_hints,omitempty"` // User guidance for navigation
}

// SemanticVocabulary represents the vocabulary analysis of a codebase
type SemanticVocabulary struct {
	DomainsPresent []SemanticDomain `json:"domains_present"`
	DomainsAbsent  []string         `json:"domains_absent"`
	UniqueTerms    []SemanticTerm   `json:"unique_terms"`
	CommonTerms    []SemanticTerm   `json:"common_terms"`
	AnalysisScope  VocabularyScope  `json:"analysis_scope"`
	VocabularySize int              `json:"vocabulary_size"`
}

// SemanticDomain represents a semantic domain found in the codebase
type SemanticDomain struct {
	Name           string   `json:"name"`
	Count          int      `json:"count"`
	Confidence     float64  `json:"confidence"`
	ExampleSymbols []string `json:"example_symbols"`
	MatchedTerms   []string `json:"matched_terms"`
}

// SemanticTerm represents a vocabulary term
type SemanticTerm struct {
	Term           string   `json:"term"`
	Count          int      `json:"count"`
	ExampleSymbols []string `json:"example_symbols"`
	Domains        []string `json:"domains,omitempty"`
}

// VocabularyScope provides scope information for the vocabulary analysis
type VocabularyScope struct {
	TotalFiles        int      `json:"total_files"`
	ProductionFiles   int      `json:"production_files"`
	TestFilesExcluded int      `json:"test_files_excluded"`
	SourceDirectories []string `json:"source_directories"`
	TotalSymbols      int      `json:"total_symbols"`
	TotalFunctions    int      `json:"total_functions"`
	TotalVariables    int      `json:"total_variables"`
	TotalTypes        int      `json:"total_types"`
}

// ============================================================================
// Structure Analysis Types (for exploration-focused mode)
// ============================================================================

// StructureAnalysis provides a hierarchical view of the codebase for exploration
type StructureAnalysis struct {
	RootDirectory   string            `json:"root_directory"`
	DirectoryTree   []DirectoryNode   `json:"directory_tree"`
	FilesByCategory FileCategories    `json:"files_by_category"`
	KeySymbols      []DirectorySymbol `json:"key_symbols"`
	Focus           string            `json:"focus,omitempty"`
	Summary         StructureSummary  `json:"summary"`
}

// DirectoryNode represents a directory in the tree structure
type DirectoryNode struct {
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	FileCount   int             `json:"file_count"`
	SymbolCount int             `json:"symbol_count"`
	Children    []DirectoryNode `json:"children,omitempty"`
	Files       []FileNode      `json:"files,omitempty"`
	Collapsed   bool            `json:"collapsed"` // For pagination/depth control
}

// FileNode represents a file with key information
type FileNode struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Category    string   `json:"category"` // code, test, config, doc
	Extension   string   `json:"extension"`
	SymbolCount int      `json:"symbol_count"`
	KeySymbols  []string `json:"key_symbols,omitempty"` // Top-level symbols
}

// FileCategories groups files by their type
type FileCategories struct {
	Code   []string `json:"code"`
	Tests  []string `json:"tests"`
	Config []string `json:"config"`
	Docs   []string `json:"docs"`
	Other  []string `json:"other"`
}

// DirectorySymbol represents a key symbol in a directory
type DirectorySymbol struct {
	Directory  string `json:"directory"`
	Name       string `json:"name"`
	Type       string `json:"type"` // function, class, type, etc.
	File       string `json:"file"`
	Line       int    `json:"line"`
	Importance string `json:"importance"` // high, medium, low
}

// StructureSummary provides high-level statistics about the structure
type StructureSummary struct {
	TotalDirectories  int            `json:"total_directories"`
	TotalFiles        int            `json:"total_files"`
	TotalSymbols      int            `json:"total_symbols"`
	MaxDepth          int            `json:"max_depth"`
	FileTypeBreakdown map[string]int `json:"file_type_breakdown"` // extension -> count
	DirectorySizes    []DirSize      `json:"directory_sizes"`     // Top directories by size
}

// DirSize represents a directory's size information
type DirSize struct {
	Path      string `json:"path"`
	FileCount int    `json:"file_count"`
}

// ============================================================================
// Type Hierarchy Analysis Types
// ============================================================================

// TypeHierarchyAnalysis provides analysis of type relationships in the codebase
type TypeHierarchyAnalysis struct {
	// Interfaces and their implementors
	Interfaces []InterfaceHierarchy `json:"interfaces"`

	// Base types and their derived types (class inheritance)
	Inheritance []InheritanceHierarchy `json:"inheritance"`

	// Summary statistics
	Summary TypeHierarchySummary `json:"summary"`
}

// InterfaceHierarchy represents an interface and all types that implement it
type InterfaceHierarchy struct {
	// Interface information
	ObjectID    string `json:"oid,omitempty"` // Compact ID for drill-down
	EntityID    string `json:"entity_id"`     // Unified entity ID
	Name        string `json:"name"`          // Interface name
	File        string `json:"file"`          // File location
	Line        int    `json:"line"`          // Line number
	MethodCount int    `json:"method_count"`  // Number of methods in interface

	// Types implementing this interface
	Implementors []TypeRelationshipRef `json:"implementors"`
}

// InheritanceHierarchy represents a base type and all types that extend it
type InheritanceHierarchy struct {
	// Base type information
	ObjectID string `json:"oid,omitempty"` // Compact ID for drill-down
	EntityID string `json:"entity_id"`     // Unified entity ID
	Name     string `json:"name"`          // Base type name
	File     string `json:"file"`          // File location
	Line     int    `json:"line"`          // Line number
	TypeKind string `json:"type_kind"`     // "class", "struct", "interface", etc.

	// Types extending this base type
	DerivedTypes []TypeRelationshipRef `json:"derived_types"`

	// Base types this type extends (for showing full inheritance chain)
	BaseTypes []TypeRelationshipRef `json:"base_types,omitempty"`
}

// TypeRelationshipRef represents a reference in a type relationship
type TypeRelationshipRef struct {
	ObjectID string `json:"oid,omitempty"` // Compact ID for drill-down
	EntityID string `json:"entity_id"`     // Unified entity ID
	Name     string `json:"name"`          // Type name
	File     string `json:"file"`          // File location
	Line     int    `json:"line"`          // Line number
	TypeKind string `json:"type_kind"`     // "class", "struct", "interface", etc.
	Language string `json:"language"`      // Programming language
}

// TypeHierarchySummary provides summary statistics for type hierarchy analysis
type TypeHierarchySummary struct {
	TotalInterfaces   int `json:"total_interfaces"`
	TotalImplementors int `json:"total_implementors"`
	TotalBaseTypes    int `json:"total_base_types"`
	TotalDerivedTypes int `json:"total_derived_types"`

	// Distribution by language
	LanguageBreakdown map[string]TypeLanguageStats `json:"language_breakdown"`
}

// TypeLanguageStats provides type hierarchy stats per language
type TypeLanguageStats struct {
	Interfaces   int `json:"interfaces"`
	Implementors int `json:"implementors"`
	BaseTypes    int `json:"base_types"`
	DerivedTypes int `json:"derived_types"`
}

// ============================================================================
// Performance Anti-Pattern Detection Types (Phase 1)
// ============================================================================

// PerformanceAntiPattern represents a detected performance issue in code
// These are cross-language patterns detectable via AST analysis
type PerformanceAntiPattern struct {
	Type        PerformancePatternType `json:"type"`              // Pattern type enum
	Symbol      string                 `json:"symbol"`            // Containing function/method name
	ObjectID    string                 `json:"oid"`               // For drill-down via get_context
	Location    string                 `json:"loc"`               // "file.go:line"
	Severity    string                 `json:"sev"`               // "high", "medium", "low"
	Description string                 `json:"desc"`              // Human-readable description
	Language    string                 `json:"lang"`              // Language where detected
	Suggestion  string                 `json:"suggestion"`        // Recommended fix
	Details     *PatternDetails        `json:"details,omitempty"` // Optional detailed info
}

// PerformancePatternType represents the type of performance anti-pattern
type PerformancePatternType string

const (
	// PatternSequentialAwaits - Multiple awaits that could be parallelized
	// Languages: JS/TS, Python, C#, Rust
	PatternSequentialAwaits PerformancePatternType = "sequential-awaits"

	// PatternAwaitInLoop - Await inside a loop causing sequential execution
	// Languages: JS/TS, Python, C#, Rust
	PatternAwaitInLoop PerformancePatternType = "await-in-loop"

	// PatternExpensiveCallInLoop - Known expensive operation inside loop
	// Languages: All (with language-specific patterns)
	PatternExpensiveCallInLoop PerformancePatternType = "expensive-call-in-loop"

	// PatternNestedLoops - Nested loops indicating potential O(n²) complexity
	// Languages: All
	PatternNestedLoops PerformancePatternType = "nested-loops"

	// PatternStringConcatInLoop - String concatenation in loop (use StringBuilder)
	// Languages: All
	PatternStringConcatInLoop PerformancePatternType = "string-concat-in-loop"

	// PatternDeferInLoop - Go-specific: defer inside loop
	// Languages: Go only
	PatternDeferInLoop PerformancePatternType = "defer-in-loop"

	// PatternUnbufferedChannel - Unbuffered channel creation (potential blocking)
	// Languages: Go, Rust
	PatternUnbufferedChannel PerformancePatternType = "unbuffered-channel"

	// PatternMapWithoutCapacity - Map/dict created without capacity hint
	// Languages: Go, Java, C#, Rust
	PatternMapWithoutCapacity PerformancePatternType = "map-without-capacity"
)

// PatternDetails provides additional context for specific pattern types
type PatternDetails struct {
	// For sequential-awaits pattern
	TotalAwaits         int      `json:"total_awaits,omitempty"`   // Total await count in function
	ParallelizableCount int      `json:"parallelizable,omitempty"` // How many could be concurrent
	AwaitLines          []int    `json:"await_lines,omitempty"`    // Lines of parallelizable awaits
	AwaitTargets        []string `json:"await_targets,omitempty"`  // Function names being awaited

	// For nested-loops pattern
	NestingDepth  int `json:"nesting_depth,omitempty"`   // Depth of loop nesting
	OuterLoopLine int `json:"outer_loop_line,omitempty"` // Line of outermost loop
	InnerLoopLine int `json:"inner_loop_line,omitempty"` // Line of innermost loop

	// For expensive-call-in-loop pattern
	ExpensiveCall   string `json:"expensive_call,omitempty"`   // The expensive function name
	LoopLine        int    `json:"loop_line,omitempty"`        // Line of containing loop
	CallLine        int    `json:"call_line,omitempty"`        // Line of the expensive call
	ExpenseCategory string `json:"expense_category,omitempty"` // "regex", "io", "network", "parse"
}

// PerformanceAnalysis contains all detected performance anti-patterns
type PerformanceAnalysis struct {
	Patterns         []PerformanceAntiPattern `json:"patterns"`
	Summary          PerformanceSummary       `json:"summary"`
	AnalysisMetadata AnalysisMetadata         `json:"analysis_metadata"`
}

// PerformanceSummary provides aggregate statistics on performance issues
type PerformanceSummary struct {
	TotalPatterns     int            `json:"total_patterns"`
	BySeverity        map[string]int `json:"by_severity"`         // high, medium, low counts
	ByType            map[string]int `json:"by_type"`             // counts by pattern type
	ByLanguage        map[string]int `json:"by_language"`         // counts by language
	MostAffectedFiles []string       `json:"most_affected_files"` // Top 5 files with issues
}

// ExpensiveCallPattern defines a known expensive operation for a language
type ExpensiveCallPattern struct {
	Language    string `json:"language"`
	Pattern     string `json:"pattern"`  // Regex or exact match
	Category    string `json:"category"` // "regex", "io", "network", "parse", "reflection"
	Description string `json:"description"`
	Severity    string `json:"severity"` // Default severity for this pattern
}

// LanguageAsyncConfig defines async/await syntax for each language
type LanguageAsyncConfig struct {
	AwaitKeyword      string `json:"await_keyword"`      // "await" for most, ".await" for Rust
	AsyncKeyword      string `json:"async_keyword"`      // "async"
	ParallelConstruct string `json:"parallel_construct"` // "Promise.all", "asyncio.gather", etc.
	ASTAwaitNode      string `json:"ast_await_node"`     // Tree-sitter node type
	ASTAsyncNode      string `json:"ast_async_node"`     // Tree-sitter node type for async functions
}
