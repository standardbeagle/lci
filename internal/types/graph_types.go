package types

import (
	"fmt"
	"time"
)

// Universal Symbol Graph Types - Language-agnostic symbol relationship representation

// UniversalSymbolNode represents a node in the Universal Symbol Graph
// This is the core node that extends beyond just function calls to include all symbol relationships
type UniversalSymbolNode struct {
	// Identity - Core identification information
	Identity SymbolIdentity `json:"identity"`

	// Relationships - All types of relationships this symbol has
	Relationships SymbolRelationships `json:"relationships"`

	// Visibility - Access control and scope information
	Visibility SymbolVisibility `json:"visibility"`

	// Usage - Usage statistics and metrics
	Usage SymbolUsage `json:"usage"`

	// Metadata - Additional contextual information
	Metadata SymbolMetadata `json:"metadata"`
}

// SymbolIdentity contains core identification information for a symbol
type SymbolIdentity struct {
	ID        CompositeSymbolID `json:"id"`        // Unique composite ID
	Name      string            `json:"name"`      // Symbol name
	FullName  string            `json:"full_name"` // Fully qualified name
	Kind      SymbolKind        `json:"kind"`      // Type of symbol (function, class, etc.)
	Language  string            `json:"language"`  // Programming language
	Location  SymbolLocation    `json:"location"`  // Where the symbol is defined
	Signature string            `json:"signature"` // Function/method signature
	Type      string            `json:"type"`      // Variable/field type
	Value     string            `json:"value"`     // Constant value if applicable
}

// SymbolRelationships contains all relationship types for a symbol
type SymbolRelationships struct {
	// Structural relationships
	Extends     []CompositeSymbolID `json:"extends"`      // Types this symbol extends
	Implements  []CompositeSymbolID `json:"implements"`   // Interfaces this symbol implements
	Contains    []CompositeSymbolID `json:"contains"`     // Symbols contained within this symbol
	ContainedBy *CompositeSymbolID  `json:"contained_by"` // Parent symbol containing this one

	// Dependency relationships
	Dependencies []SymbolDependency  `json:"dependencies"` // Symbols this depends on
	Dependents   []CompositeSymbolID `json:"dependents"`   // Symbols that depend on this

	// Call relationships (from existing CallGraph)
	CallsTo  []FunctionCall `json:"calls_to"`  // Functions this calls
	CalledBy []FunctionCall `json:"called_by"` // Functions that call this

	// File co-location relationships
	FileCoLocated []CompositeSymbolID `json:"file_co_located"` // Other symbols in same file

	// Cross-language relationships
	CrossLanguage []CrossLanguageLink `json:"cross_language"` // Links to symbols in other languages
}

// SymbolVisibility represents access control and scope information
type SymbolVisibility struct {
	Access      AccessLevel `json:"access"`       // public, private, protected, internal
	Scope       SymbolScope `json:"scope"`        // Scope hierarchy information
	IsExported  bool        `json:"is_exported"`  // Whether symbol is exported/public
	IsExternal  bool        `json:"is_external"`  // Whether symbol is external to project
	IsBuiltin   bool        `json:"is_builtin"`   // Whether symbol is language builtin
	IsGenerated bool        `json:"is_generated"` // Whether symbol is generated code
}

// SymbolUsage represents usage statistics and patterns
type SymbolUsage struct {
	ReferenceCount    int `json:"reference_count"`    // Total references to this symbol
	ImportCount       int `json:"import_count"`       // Times imported
	InheritanceCount  int `json:"inheritance_count"`  // Times extended/implemented
	CallCount         int `json:"call_count"`         // Times called (functions only)
	ModificationCount int `json:"modification_count"` // Times modified (variables only)

	// Reference patterns
	ReferencingFiles []FileID       `json:"referencing_files"` // Files that reference this
	HotSpots         []UsageHotSpot `json:"hot_spots"`         // High-usage locations

	// Temporal usage patterns
	FirstSeen      time.Time `json:"first_seen"`      // When first discovered
	LastModified   time.Time `json:"last_modified"`   // Last modification time
	LastReferenced time.Time `json:"last_referenced"` // Last time referenced
}

// SymbolMetadata contains additional contextual information
type SymbolMetadata struct {
	// Documentation and comments
	Documentation []string `json:"documentation"` // Doc comments
	Comments      []string `json:"comments"`      // Inline comments

	// Attributes and annotations
	Attributes  []ContextAttribute `json:"attributes"`  // Context attributes (async, deprecated, etc.)
	Annotations []SymbolAnnotation `json:"annotations"` // Language-specific annotations

	// Quality metrics
	ComplexityScore int `json:"complexity_score"` // Cyclomatic/cognitive complexity
	CouplingScore   int `json:"coupling_score"`   // How tightly coupled this symbol is
	CohesionScore   int `json:"cohesion_score"`   // How cohesive this symbol is

	// AI navigation hints
	EditRiskScore int      `json:"edit_risk_score"` // Risk of editing this symbol (0-10)
	StabilityTags []string `json:"stability_tags"`  // CORE, PUBLIC_API, RECURSIVE, etc.
	SafetyNotes   []string `json:"safety_notes"`    // Human-readable safety warnings
}

// RelationshipType represents different types of symbol relationships
type RelationshipType uint8

const (
	// Structural relationships
	RelationExtends RelationshipType = iota
	RelationImplements
	RelationContains
	RelationContainedBy

	// Dependency relationships
	RelationDependsOn
	RelationDependedOnBy

	// Call relationships
	RelationCalls
	RelationCalledBy

	// Reference relationships
	RelationReferences
	RelationReferencedBy

	// Import relationships
	RelationImports
	RelationImportedBy

	// File co-location
	RelationFileCoLocated

	// Type relationships
	RelationTypeOf
	RelationHasType

	// Inheritance chain
	RelationParentType
	RelationChildType

	// Cross-language
	RelationCrossLanguage
)

// String returns string representation of relationship type
func (rt RelationshipType) String() string {
	switch rt {
	case RelationExtends:
		return "extends"
	case RelationImplements:
		return "implements"
	case RelationContains:
		return "contains"
	case RelationContainedBy:
		return "contained_by"
	case RelationDependsOn:
		return "depends_on"
	case RelationDependedOnBy:
		return "depended_on_by"
	case RelationCalls:
		return "calls"
	case RelationCalledBy:
		return "called_by"
	case RelationReferences:
		return "references"
	case RelationReferencedBy:
		return "referenced_by"
	case RelationImports:
		return "imports"
	case RelationImportedBy:
		return "imported_by"
	case RelationFileCoLocated:
		return "file_co_located"
	case RelationTypeOf:
		return "type_of"
	case RelationHasType:
		return "has_type"
	case RelationParentType:
		return "parent_type"
	case RelationChildType:
		return "child_type"
	case RelationCrossLanguage:
		return "cross_language"
	default:
		return "unknown"
	}
}

// ParseRelationshipType parses a string into a RelationshipType
func ParseRelationshipType(s string) RelationshipType {
	switch s {
	case "extends":
		return RelationExtends
	case "implements":
		return RelationImplements
	case "contains":
		return RelationContains
	case "contained_by":
		return RelationContainedBy
	case "depends_on":
		return RelationDependsOn
	case "depended_on_by":
		return RelationDependedOnBy
	case "calls":
		return RelationCalls
	case "called_by":
		return RelationCalledBy
	case "references":
		return RelationReferences
	case "referenced_by":
		return RelationReferencedBy
	case "imports":
		return RelationImports
	case "imported_by":
		return RelationImportedBy
	case "file_co_located":
		return RelationFileCoLocated
	case "type_of":
		return RelationTypeOf
	case "has_type":
		return RelationHasType
	case "parent_type":
		return RelationParentType
	case "child_type":
		return RelationChildType
	case "cross_language":
		return RelationCrossLanguage
	default:
		return RelationExtends // Default fallback
	}
}

// SymbolDependency represents a dependency relationship with additional context
type SymbolDependency struct {
	Target        CompositeSymbolID  `json:"target"`         // What this symbol depends on
	Type          DependencyType     `json:"type"`           // Type of dependency
	Strength      DependencyStrength `json:"strength"`       // How strong the dependency is
	Context       string             `json:"context"`        // Context where dependency occurs
	ImportPath    string             `json:"import_path"`    // Import path if applicable
	IsOptional    bool               `json:"is_optional"`    // Whether dependency is optional
	IsConditional bool               `json:"is_conditional"` // Whether dependency is conditional
}

// FunctionCall extends the existing CallEdge with more context
type FunctionCall struct {
	Target      CompositeSymbolID `json:"target"`       // Function being called
	CallType    CallType          `json:"call_type"`    // Type of call
	Location    SymbolLocation    `json:"location"`     // Where call occurs
	Context     string            `json:"context"`      // Context of the call
	IsAsync     bool              `json:"is_async"`     // Whether call is asynchronous
	IsRecursive bool              `json:"is_recursive"` // Whether call is recursive
	Arguments   []CallArgument    `json:"arguments"`    // Arguments passed (if available)
}

// CrossLanguageLink represents a relationship between symbols in different languages
type CrossLanguageLink struct {
	Target      CompositeSymbolID `json:"target"`      // Symbol in other language
	LinkType    CrossLinkType     `json:"link_type"`   // Type of cross-language link
	Language    string            `json:"language"`    // Target language
	Bridge      string            `json:"bridge"`      // How languages are bridged (FFI, API, etc.)
	Confidence  float64           `json:"confidence"`  // Confidence in the link (0-1)
	Description string            `json:"description"` // Human-readable description
}

// UsageHotSpot represents a location with high symbol usage
type UsageHotSpot struct {
	FileID     FileID  `json:"file_id"`     // File with high usage
	StartLine  int     `json:"start_line"`  // Start of hot spot region
	EndLine    int     `json:"end_line"`    // End of hot spot region
	UsageCount int     `json:"usage_count"` // Number of usages in region
	UsageType  string  `json:"usage_type"`  // Type of usage (calls, references, etc.)
	Intensity  float64 `json:"intensity"`   // Usage intensity score
}

// SymbolAnnotation represents language-specific annotations
type SymbolAnnotation struct {
	Type       AnnotationType `json:"type"`       // Type of annotation
	Value      string         `json:"value"`      // Annotation value
	Parameters []string       `json:"parameters"` // Annotation parameters
	Location   SymbolLocation `json:"location"`   // Where annotation appears
}

// AccessLevel represents different access control levels
type AccessLevel uint8

const (
	AccessUnknown AccessLevel = iota
	AccessPublic
	AccessPrivate
	AccessProtected
	AccessInternal
	AccessPackage
)

// String returns string representation of access level
func (al AccessLevel) String() string {
	switch al {
	case AccessPublic:
		return "public"
	case AccessPrivate:
		return "private"
	case AccessProtected:
		return "protected"
	case AccessInternal:
		return "internal"
	case AccessPackage:
		return "package"
	default:
		return "unknown"
	}
}

// DependencyType represents different types of dependencies
type DependencyType uint8

const (
	DependencyUnknown DependencyType = iota
	DependencyImport
	DependencyInheritance
	DependencyComposition
	DependencyAggregation
	DependencyAssociation
	DependencyUsage
	DependencyConfiguration
	DependencyRuntime
)

// String returns string representation of dependency type
func (dt DependencyType) String() string {
	switch dt {
	case DependencyImport:
		return "import"
	case DependencyInheritance:
		return "inheritance"
	case DependencyComposition:
		return "composition"
	case DependencyAggregation:
		return "aggregation"
	case DependencyAssociation:
		return "association"
	case DependencyUsage:
		return "usage"
	case DependencyConfiguration:
		return "configuration"
	case DependencyRuntime:
		return "runtime"
	default:
		return "unknown"
	}
}

// DependencyStrength represents the strength of a dependency
type DependencyStrength uint8

const (
	DependencyWeak DependencyStrength = iota
	DependencyModerate
	DependencyStrong
	DependencyCritical
)

// String returns string representation of dependency strength
func (ds DependencyStrength) String() string {
	switch ds {
	case DependencyWeak:
		return "weak"
	case DependencyModerate:
		return "moderate"
	case DependencyStrong:
		return "strong"
	case DependencyCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// CallType represents different types of function calls
type CallType uint8

const (
	CallDirect CallType = iota
	CallMethod
	CallCallback
	CallDynamic
	CallRecursive
	CallVirtual
	CallInterface
	CallAsync
	CallDeferred
)

// String returns string representation of call type
func (ct CallType) String() string {
	switch ct {
	case CallDirect:
		return "direct"
	case CallMethod:
		return "method"
	case CallCallback:
		return "callback"
	case CallDynamic:
		return "dynamic"
	case CallRecursive:
		return "recursive"
	case CallVirtual:
		return "virtual"
	case CallInterface:
		return "interface"
	case CallAsync:
		return "async"
	case CallDeferred:
		return "deferred"
	default:
		return "unknown"
	}
}

// CrossLinkType represents different types of cross-language links
type CrossLinkType uint8

const (
	CrossLinkFFI        CrossLinkType = iota // Foreign Function Interface
	CrossLinkAPI                             // REST API, GraphQL, etc.
	CrossLinkRPC                             // Remote Procedure Call
	CrossLinkMessage                         // Message passing
	CrossLinkSharedData                      // Shared data structures
	CrossLinkConfig                          // Configuration files
	CrossLinkBuild                           // Build system integration
)

// String returns string representation of cross-link type
func (clt CrossLinkType) String() string {
	switch clt {
	case CrossLinkFFI:
		return "ffi"
	case CrossLinkAPI:
		return "api"
	case CrossLinkRPC:
		return "rpc"
	case CrossLinkMessage:
		return "message"
	case CrossLinkSharedData:
		return "shared_data"
	case CrossLinkConfig:
		return "config"
	case CrossLinkBuild:
		return "build"
	default:
		return "unknown"
	}
}

// AnnotationType represents different types of annotations
type AnnotationType uint8

const (
	AnnotationDecorator AnnotationType = iota // @decorator
	AnnotationAttribute                       // [Attribute]
	AnnotationPragma                          // #pragma
	AnnotationDirective                       // "use strict"
	AnnotationComment                         // // @annotation
	AnnotationGeneric                         // Generic language annotation
)

// String returns string representation of annotation type
func (at AnnotationType) String() string {
	switch at {
	case AnnotationDecorator:
		return "decorator"
	case AnnotationAttribute:
		return "attribute"
	case AnnotationPragma:
		return "pragma"
	case AnnotationDirective:
		return "directive"
	case AnnotationComment:
		return "comment"
	case AnnotationGeneric:
		return "generic"
	default:
		return "unknown"
	}
}

// CallArgument represents an argument in a function call
type CallArgument struct {
	Name      string `json:"name"`       // Argument name (if available)
	Type      string `json:"type"`       // Argument type (if available)
	Value     string `json:"value"`      // Argument value (if literal)
	IsLiteral bool   `json:"is_literal"` // Whether argument is a literal value
}

// String returns a string representation of the UniversalSymbolNode
func (usn *UniversalSymbolNode) String() string {
	return fmt.Sprintf("UniversalSymbolNode[%s %s:%d]",
		usn.Identity.Kind.String(),
		usn.Identity.Name,
		usn.Identity.Location.FileID)
}

// GetRelationshipCount returns the total number of relationships for this symbol
func (usn *UniversalSymbolNode) GetRelationshipCount() int {
	r := &usn.Relationships
	return len(r.Extends) + len(r.Implements) + len(r.Contains) +
		len(r.Dependencies) + len(r.Dependents) + len(r.CallsTo) +
		len(r.CalledBy) + len(r.FileCoLocated) + len(r.CrossLanguage)
}

// GetComplexityIndicator returns a simple complexity indicator based on relationships and metadata
func (usn *UniversalSymbolNode) GetComplexityIndicator() string {
	relationCount := usn.GetRelationshipCount()
	complexityScore := usn.Metadata.ComplexityScore

	switch {
	case relationCount > 20 || complexityScore > 15:
		return "very_high"
	case relationCount > 10 || complexityScore > 10:
		return "high"
	case relationCount > 5 || complexityScore > 5:
		return "medium"
	case relationCount > 0 || complexityScore > 0:
		return "low"
	default:
		return "minimal"
	}
}

// IsEditSafe returns whether this symbol is safe for AI agents to edit
func (usn *UniversalSymbolNode) IsEditSafe() bool {
	// Consider edit-safe if risk score is low and no critical stability tags
	if usn.Metadata.EditRiskScore > 5 {
		return false
	}

	for _, tag := range usn.Metadata.StabilityTags {
		if tag == "CORE" || tag == "PUBLIC_API" || tag == "CRITICAL" {
			return false
		}
	}

	return true
}

// GetNavigationSuggestions returns suggested next actions for AI navigation
func (usn *UniversalSymbolNode) GetNavigationSuggestions() []string {
	suggestions := []string{}
	r := &usn.Relationships

	// Suggest based on relationships
	if len(r.Extends) > 0 {
		suggestions = append(suggestions, "View parent types")
	}
	if len(r.Implements) > 0 {
		suggestions = append(suggestions, "View implemented interfaces")
	}
	if len(r.Contains) > 0 {
		suggestions = append(suggestions, "Explore contained symbols")
	}
	if len(r.CallsTo) > 0 {
		suggestions = append(suggestions, "Analyze called functions")
	}
	if len(r.CalledBy) > 0 {
		suggestions = append(suggestions, "View calling functions")
	}
	if len(r.Dependencies) > 0 {
		suggestions = append(suggestions, "Review dependencies")
	}

	// Suggest based on symbol type
	switch usn.Identity.Kind {
	case SymbolKindInterface:
		suggestions = append(suggestions, "Find implementations")
	case SymbolKindFunction, SymbolKindMethod:
		suggestions = append(suggestions, "Analyze call hierarchy")
	case SymbolKindClass, SymbolKindStruct:
		suggestions = append(suggestions, "Explore members")
	}

	return suggestions
}
