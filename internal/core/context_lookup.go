package core

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// CodeObjectID uniquely identifies a code object (function, class, type, etc.)
type CodeObjectID struct {
	FileID   types.FileID     `json:"file_id"`
	SymbolID string           `json:"symbol_id"` // Unique symbol identifier
	Name     string           `json:"name"`      // Human-readable name
	Type     types.SymbolType `json:"type"`      // Function, Class, Method, etc.
}

// CodeObjectContext provides comprehensive context about a code object
type CodeObjectContext struct {
	// Basic identification
	ObjectID      CodeObjectID         `json:"object_id"`
	Signature     string               `json:"signature"`
	Documentation string               `json:"documentation"`
	Location      types.SymbolLocation `json:"location"`

	// Direct relationships
	DirectRelationships DirectRelationships `json:"direct_relationships"`

	// Variable and data context
	VariableContext VariableContext `json:"variable_context"`

	// Semantic and dependency context
	SemanticContext SemanticContext `json:"semantic_context"`

	// Code structure context
	StructureContext StructureContext `json:"structure_context"`

	// Usage and impact analysis
	UsageAnalysis UsageAnalysis `json:"usage_analysis"`

	// AI-enhanced context
	AIContext AIContext `json:"ai_context"`

	// Metadata
	GeneratedAt    time.Time `json:"generated_at"`
	ContextVersion string    `json:"context_version"`
}

// DirectRelationships contains immediate relationships of the code object
type DirectRelationships struct {
	// Incoming: things that use/reference this object
	IncomingReferences []ObjectReference `json:"incoming_references"`
	CallerFunctions    []ObjectReference `json:"caller_functions"`
	ParentClasses      []ObjectReference `json:"parent_classes"`
	ImplementingTypes  []ObjectReference `json:"implementing_types"`

	// Outgoing: things that this object uses/references
	OutgoingReferences []ObjectReference `json:"outgoing_references"`
	CalledFunctions    []ObjectReference `json:"called_functions"`
	UsedTypes          []ObjectReference `json:"used_types"`
	ImportedModules    []ModuleReference `json:"imported_modules"`

	// Hierarchy
	ParentObjects []ObjectReference `json:"parent_objects"` // Parent functions, classes, namespaces
	ChildObjects  []ObjectReference `json:"child_objects"`  // Member functions, nested types
}

// VariableContext contains information about variables accessible to the object
type VariableContext struct {
	GlobalVariables []VariableInfo `json:"global_variables"`
	UsedGlobals     []VariableInfo `json:"used_globals"`    // Actually used by this object
	ClassVariables  []VariableInfo `json:"class_variables"` // For classes
	LocalVariables  []VariableInfo `json:"local_variables"` // Important locals
	Parameters      []VariableInfo `json:"parameters"`      // Function parameters
	ReturnValues    []VariableInfo `json:"return_values"`   // Return type info
}

// SemanticContext provides semantic meaning and dependencies
type SemanticContext struct {
	EntryPointDependencies []EntryPointRef   `json:"entry_point_dependencies"`
	ServiceDependencies    []ServiceRef      `json:"service_dependencies"`
	PropagationLabels      []PropagationInfo `json:"propagation_labels"`
	CriticalityAnalysis    CriticalityInfo   `json:"criticality_analysis"`
	Purpose                string            `json:"purpose"` // "API handler", "utility function", etc.
	Confidence             float64           `json:"confidence"`
}

// StructureContext provides structural information about the code object
type StructureContext struct {
	FilePath                string            `json:"file_path"`
	Module                  string            `json:"module"`
	Package                 string            `json:"package"`
	Imports                 []ImportInfo      `json:"imports"`
	Exports                 []ExportInfo      `json:"exports"`
	InterfaceImplementation []InterfaceInfo   `json:"interface_implementations"`
	InheritanceChain        []ObjectReference `json:"inheritance_chain"`
	CompositionPattern      string            `json:"composition_pattern"`
}

// UsageAnalysis provides metrics and impact analysis
type UsageAnalysis struct {
	CallFrequency     int64             `json:"call_frequency"` // If available
	FanIn             int               `json:"fan_in"`         // Number of callers
	FanOut            int               `json:"fan_out"`        // Number of calls
	ComplexityMetrics ComplexityMetrics `json:"complexity_metrics"`
	ChangeImpact      ChangeImpactInfo  `json:"change_impact"`
	TestCoverage      TestCoverageInfo  `json:"test_coverage"`
}

// AIContext provides AI-enhanced understanding
type AIContext struct {
	NaturalLanguageSummary string            `json:"natural_language_summary"`
	SimilarObjects         []ObjectReference `json:"similar_objects"`
	RefactoringSuggestions []string          `json:"refactoring_suggestions"`
	CodeSmells             []CodeSmell       `json:"code_smells"`
	BestPractices          []string          `json:"best_practices"`
}

// Supporting types

type ObjectReference struct {
	ObjectID   CodeObjectID         `json:"object_id"`
	Location   types.SymbolLocation `json:"location"`
	Context    string               `json:"context"` // How it's referenced
	Confidence float64              `json:"confidence"`
}

type ModuleReference struct {
	ModulePath  string   `json:"module_path"`
	ImportStyle string   `json:"import_style"` // "direct", "aliased", "wildcard"
	UsedItems   []string `json:"used_items"`
}

type VariableInfo struct {
	Name      string               `json:"name"`
	Type      string               `json:"type"`
	Location  types.SymbolLocation `json:"location"`
	IsUsed    bool                 `json:"is_used"`
	UseCount  int                  `json:"use_count"`
	Scope     string               `json:"scope"` // "global", "class", "local"
	IsMutable bool                 `json:"is_mutable"`
}

type EntryPointRef struct {
	EntryPointID CodeObjectID `json:"entry_point_id"`
	Type         string       `json:"type"` // "HTTP endpoint", "main function", etc.
	Path         string       `json:"path"` // URL path, command name, etc.
	Confidence   float64      `json:"confidence"`
}

type ServiceRef struct {
	ServiceName    string  `json:"service_name"`
	OperationType  string  `json:"operation_type"`  // "database", "http", "message_queue"
	DependencyType string  `json:"dependency_type"` // "direct", "indirect", "transitive"
	Confidence     float64 `json:"confidence"`
}

type PropagationInfo struct {
	Label     string       `json:"label"`     // "critical-bug", "performance-cost", etc.
	Source    CodeObjectID `json:"source"`    // Where it originates
	Strength  float64      `json:"strength"`  // Propagation strength
	Direction string       `json:"direction"` // "upstream", "downstream", "bidirectional"
}

type CriticalityInfo struct {
	IsCritical         bool     `json:"is_critical"`
	CriticalityType    string   `json:"criticality_type"` // "security", "performance", "business-logic"
	ImpactScore        float64  `json:"impact_score"`
	AffectedComponents []string `json:"affected_components"`
}

type ComplexityMetrics struct {
	CyclomaticComplexity int `json:"cyclomatic_complexity"`
	CognitiveComplexity  int `json:"cognitive_complexity"`
	LineCount            int `json:"line_count"`
	ParameterCount       int `json:"parameter_count"`
	NestingDepth         int `json:"nesting_depth"`
}

type ChangeImpactInfo struct {
	BreakingChangeRisk  string   `json:"breaking_change_risk"` // "low", "medium", "high"
	DependentComponents []string `json:"dependent_components"`
	EstimatedImpact     int      `json:"estimated_impact"` // 1-10 scale
	RequiresTests       bool     `json:"requires_tests"`
}

// TestCoverageInfo provides test file discovery (not coverage measurement)
// Note: Actual coverage percentage requires specialized tools like 'go test -cover'
type TestCoverageInfo struct {
	HasTests      bool     `json:"has_tests"`
	TestFilePaths []string `json:"test_file_paths"`
}

type ImportInfo struct {
	ModulePath  string `json:"module_path"`
	ImportName  string `json:"import_name"`
	ImportStyle string `json:"import_style"`
	IsUsed      bool   `json:"is_used"`
}

type ExportInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	ExportStyle string   `json:"export_style"`
	UsedBy      []string `json:"used_by"`
}

type InterfaceInfo struct {
	InterfaceID        CodeObjectID `json:"interface_id"`
	Methods            []string     `json:"methods"`
	IsFullyImplemented bool         `json:"is_fully_implemented"`
}

type CodeSmell struct {
	Type        string               `json:"type"` // "long-function", "god-class", etc."
	Description string               `json:"description"`
	Severity    string               `json:"severity"` // "low", "medium", "high", "critical"`
	Location    types.SymbolLocation `json:"location"`
}

// ContextLookupEngine provides the main interface for code object context lookup
type ContextLookupEngine struct {
	// Core components
	symbolIndex *SymbolIndex
	// callGraph removed - RefTracker handles call relationships
	trigramIndex      *TrigramIndex
	fileService       *FileService
	graphPropagator   *GraphPropagator
	semanticAnnotator *SemanticAnnotator
	componentDetector *ComponentDetector
	refTracker        *ReferenceTracker

	// Configuration with atomic access (no mutex needed)
	maxContextDepth     int32
	includeAIText       int32 // bool as atomic int32 (0/1)
	confidenceThreshold int64 // float64 as atomic int64 for math.Float64 operations
}

// NewContextLookupEngine creates a new context lookup engine
func NewContextLookupEngine(
	symbolIndex *SymbolIndex,
	trigramIndex *TrigramIndex,
	fileService *FileService,
	graphPropagator *GraphPropagator,
	semanticAnnotator *SemanticAnnotator,
	componentDetector *ComponentDetector,
	refTracker *ReferenceTracker,
) *ContextLookupEngine {
	return &ContextLookupEngine{
		symbolIndex: symbolIndex,
		// callGraph removed - RefTracker handles call relationships
		trigramIndex:        trigramIndex,
		fileService:         fileService,
		graphPropagator:     graphPropagator,
		semanticAnnotator:   semanticAnnotator,
		componentDetector:   componentDetector,
		refTracker:          refTracker,
		maxContextDepth:     5,
		includeAIText:       1,                            // true as int32
		confidenceThreshold: int64(math.Float64bits(0.3)), // 0.3 as int64
	}
}

// GetContext returns comprehensive context for a code object
func (cle *ContextLookupEngine) GetContext(objectID CodeObjectID) (*CodeObjectContext, error) {
	// Validate engine
	if cle == nil {
		return nil, errors.New("context lookup engine is nil")
	}

	// Validate required components
	if cle.symbolIndex == nil {
		return nil, errors.New("symbol index not initialized")
	}
	// AST store removed - using metadata index instead
	if cle.refTracker == nil {
		return nil, errors.New("reference tracker not initialized")
	}

	// Validate object ID
	if !objectID.IsValid() {
		return nil, fmt.Errorf("invalid object ID: %+v", objectID)
	}

	context := &CodeObjectContext{
		ObjectID:       objectID,
		GeneratedAt:    time.Now(),
		ContextVersion: "1.0.0",
	}

	// Fill in basic information
	if err := cle.fillBasicInfo(context); err != nil {
		return nil, fmt.Errorf("failed to get basic info: %w", err)
	}

	// Fill direct relationships
	if err := cle.fillDirectRelationships(context); err != nil {
		return nil, fmt.Errorf("failed to get direct relationships: %w", err)
	}

	// Fill variable context
	if err := cle.fillVariableContext(context); err != nil {
		return nil, fmt.Errorf("failed to get variable context: %w", err)
	}

	// Fill semantic context
	if err := cle.fillSemanticContext(context); err != nil {
		return nil, fmt.Errorf("failed to get semantic context: %w", err)
	}

	// Fill structure context
	if err := cle.fillStructureContext(context); err != nil {
		return nil, fmt.Errorf("failed to get structure context: %w", err)
	}

	// Fill usage analysis
	if err := cle.fillUsageAnalysis(context); err != nil {
		return nil, fmt.Errorf("failed to get usage analysis: %w", err)
	}

	// Fill AI context if enabled
	if atomic.LoadInt32(&cle.includeAIText) != 0 {
		if err := cle.fillAIContext(context); err != nil {
			return nil, fmt.Errorf("failed to get AI context: %w", err)
		}
	}

	return context, nil
}

// Helper methods for validation and filtering

func (ref ObjectReference) IsValid() bool {
	return ref.ObjectID.Name != "" && ref.ObjectID.FileID != 0
}

func (obj CodeObjectID) String() string {
	return fmt.Sprintf("%s:%s:%d", obj.Type, obj.Name, obj.FileID)
}

func (obj CodeObjectID) IsValid() bool {
	return obj.Name != "" && obj.FileID != 0 && obj.SymbolID != ""
}

// Filter high-confidence references
func FilterHighConfidence(refs []ObjectReference, threshold float64) []ObjectReference {
	var filtered []ObjectReference
	for _, ref := range refs {
		if ref.Confidence >= threshold {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

// Sort references by confidence (descending)
func SortByConfidence(refs []ObjectReference) {
	for i := 0; i < len(refs)-1; i++ {
		for j := i + 1; j < len(refs); j++ {
			if refs[i].Confidence < refs[j].Confidence {
				refs[i], refs[j] = refs[j], refs[i]
			}
		}
	}
}
