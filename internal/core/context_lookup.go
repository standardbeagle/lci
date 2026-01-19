package core

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// LookupError represents a specific failure during context lookup
type LookupError struct {
	Code    string // Machine-readable error code
	Message string // Human-readable description
	Field   string // Which field/section failed
	Fatal   bool   // If true, entire lookup should fail
}

func (e *LookupError) Error() string {
	return fmt.Sprintf("[%s] %s (field: %s)", e.Code, e.Message, e.Field)
}

// Standard lookup error codes
var (
	ErrSymbolNotFound      = &LookupError{Code: "SYMBOL_NOT_FOUND", Message: "symbol not found in index", Field: "object_id", Fatal: true}
	ErrSymbolNotInFile     = &LookupError{Code: "SYMBOL_NOT_IN_FILE", Message: "symbol exists but not in specified file", Field: "object_id", Fatal: true}
	ErrRefTrackerNil       = &LookupError{Code: "REF_TRACKER_NIL", Message: "reference tracker not initialized - call graph unavailable", Field: "relationships", Fatal: false}
	ErrCallGraphEmpty      = &LookupError{Code: "CALL_GRAPH_EMPTY", Message: "call graph index is empty - relationships not indexed", Field: "relationships", Fatal: false}
	ErrNotAFunction        = &LookupError{Code: "NOT_A_FUNCTION", Message: "caller/callee queries only apply to functions and methods", Field: "caller_functions", Fatal: false}
	ErrSymbolIndexNil      = &LookupError{Code: "SYMBOL_INDEX_NIL", Message: "symbol index not initialized", Field: "basic_info", Fatal: true}
	ErrFileServiceNil      = &LookupError{Code: "FILE_SERVICE_NIL", Message: "file service not initialized", Field: "structure", Fatal: false}
)

// LookupDiagnostics tracks all issues encountered during context lookup
type LookupDiagnostics struct {
	Errors   []LookupError `json:"errors,omitempty"`   // Non-fatal errors encountered
	Warnings []string      `json:"warnings,omitempty"` // Warnings about partial data
	// Index health indicators
	SymbolIndexReady    bool `json:"symbol_index_ready"`
	RefTrackerReady     bool `json:"ref_tracker_ready"`
	CallGraphPopulated  bool `json:"call_graph_populated"`
	SideEffectsReady    bool `json:"side_effects_ready"`
	// Counts for verification
	RelationshipsFound int `json:"relationships_found"`
	SymbolsSearched    int `json:"symbols_searched"`
}

// HasFatalError returns true if any fatal error occurred
func (d *LookupDiagnostics) HasFatalError() bool {
	for _, e := range d.Errors {
		if e.Fatal {
			return true
		}
	}
	return false
}

// AddError adds a lookup error to diagnostics
func (d *LookupDiagnostics) AddError(err *LookupError) {
	if err != nil {
		d.Errors = append(d.Errors, *err)
	}
}

// AddWarning adds a warning message
func (d *LookupDiagnostics) AddWarning(msg string) {
	d.Warnings = append(d.Warnings, msg)
}

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

	// Diagnostics - reports any issues encountered during lookup
	Diagnostics *LookupDiagnostics `json:"diagnostics,omitempty"`
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
// Errors are reported in Diagnostics field; only fatal errors return error
func (cle *ContextLookupEngine) GetContext(objectID CodeObjectID) (*CodeObjectContext, error) {
	// Initialize diagnostics to track all issues
	diag := &LookupDiagnostics{}

	// Validate engine
	if cle == nil {
		return nil, errors.New("context lookup engine is nil")
	}

	// Check and report component availability
	diag.SymbolIndexReady = cle.symbolIndex != nil
	diag.RefTrackerReady = cle.refTracker != nil

	// Validate required components - these are fatal
	if cle.symbolIndex == nil {
		diag.AddError(ErrSymbolIndexNil)
		return nil, fmt.Errorf("%w: symbol index not initialized", ErrSymbolIndexNil)
	}
	if cle.refTracker == nil {
		diag.AddError(ErrRefTrackerNil)
		return nil, fmt.Errorf("%w: reference tracker not initialized", ErrRefTrackerNil)
	}

	// Check if call graph has data
	diag.CallGraphPopulated = cle.refTracker.HasRelationships()
	if !diag.CallGraphPopulated {
		diag.AddError(ErrCallGraphEmpty)
		diag.AddWarning("Call graph is empty - relationship queries will return no data. Ensure full indexing is complete.")
	}

	// Check side effects availability
	diag.SideEffectsReady = cle.graphPropagator != nil

	// Validate object ID
	if !objectID.IsValid() {
		return nil, fmt.Errorf("invalid object ID: %+v", objectID)
	}

	context := &CodeObjectContext{
		ObjectID:       objectID,
		GeneratedAt:    time.Now(),
		ContextVersion: "1.1.0", // Updated version with diagnostics
		Diagnostics:    diag,
	}

	// Fill in basic information - fatal on failure
	if err := cle.fillBasicInfo(context); err != nil {
		diag.AddError(&LookupError{Code: "BASIC_INFO_FAILED", Message: err.Error(), Field: "basic_info", Fatal: true})
		return nil, fmt.Errorf("failed to get basic info: %w", err)
	}

	// Fill direct relationships - continue on failure, report in diagnostics
	if err := cle.fillDirectRelationships(context); err != nil {
		diag.AddError(&LookupError{Code: "RELATIONSHIPS_FAILED", Message: err.Error(), Field: "direct_relationships", Fatal: false})
		diag.AddWarning(fmt.Sprintf("Relationship lookup failed: %v", err))
	}

	// Count relationships found for diagnostics
	diag.RelationshipsFound = len(context.DirectRelationships.CallerFunctions) +
		len(context.DirectRelationships.CalledFunctions) +
		len(context.DirectRelationships.IncomingReferences) +
		len(context.DirectRelationships.OutgoingReferences)

	// Fill variable context - continue on failure
	if err := cle.fillVariableContext(context); err != nil {
		diag.AddError(&LookupError{Code: "VARIABLES_FAILED", Message: err.Error(), Field: "variable_context", Fatal: false})
	}

	// Fill semantic context - continue on failure
	if err := cle.fillSemanticContext(context); err != nil {
		diag.AddError(&LookupError{Code: "SEMANTIC_FAILED", Message: err.Error(), Field: "semantic_context", Fatal: false})
	}

	// Fill structure context - continue on failure
	if err := cle.fillStructureContext(context); err != nil {
		diag.AddError(&LookupError{Code: "STRUCTURE_FAILED", Message: err.Error(), Field: "structure_context", Fatal: false})
	}

	// Fill usage analysis - continue on failure
	if err := cle.fillUsageAnalysis(context); err != nil {
		diag.AddError(&LookupError{Code: "USAGE_FAILED", Message: err.Error(), Field: "usage_analysis", Fatal: false})
	}

	// Fill AI context if enabled - continue on failure
	if atomic.LoadInt32(&cle.includeAIText) != 0 {
		if err := cle.fillAIContext(context); err != nil {
			diag.AddError(&LookupError{Code: "AI_CONTEXT_FAILED", Message: err.Error(), Field: "ai_context", Fatal: false})
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
