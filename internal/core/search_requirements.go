package core

import (
	"errors"
	"strings"
)

// SearchRequirements defines which index types are required for a specific search operation
type SearchRequirements struct {
	NeedsTrigrams   bool `json:"needsTrigrams"`
	NeedsSymbols    bool `json:"needsSymbols"`
	NeedsReferences bool `json:"needsReferences"`
	NeedsCallGraph  bool `json:"needsCallGraph"`
	NeedsPostings   bool `json:"needsPostings"`
	NeedsLocations  bool `json:"needsLocations"`
	NeedsContent    bool `json:"needsContent"`
}

// NewSearchRequirements creates a new SearchRequirements with all values set to false
func NewSearchRequirements() SearchRequirements {
	return SearchRequirements{}
}

// NewTrigramOnlyRequirements creates SearchRequirements that only needs trigram index
func NewTrigramOnlyRequirements() SearchRequirements {
	return SearchRequirements{
		NeedsTrigrams: true,
	}
}

// NewSymbolOnlyRequirements creates SearchRequirements that only needs symbol index
func NewSymbolOnlyRequirements() SearchRequirements {
	return SearchRequirements{
		NeedsSymbols: true,
	}
}

// NewComprehensiveRequirements creates SearchRequirements that needs all indexes
func NewComprehensiveRequirements() SearchRequirements {
	return SearchRequirements{
		NeedsTrigrams:   true,
		NeedsSymbols:    true,
		NeedsReferences: true,
		NeedsCallGraph:  true,
		NeedsPostings:   true,
		NeedsLocations:  true,
		NeedsContent:    true,
	}
}

// HasAnyRequirements returns true if at least one requirement is set to true
func (sr SearchRequirements) HasAnyRequirements() bool {
	return sr.NeedsTrigrams || sr.NeedsSymbols || sr.NeedsReferences ||
		sr.NeedsCallGraph || sr.NeedsPostings || sr.NeedsLocations || sr.NeedsContent
}

// GetRequiredIndexTypes returns a slice of all required index types
func (sr SearchRequirements) GetRequiredIndexTypes() []IndexType {
	var required []IndexType

	if sr.NeedsTrigrams {
		required = append(required, TrigramIndexType)
	}
	if sr.NeedsSymbols {
		required = append(required, SymbolIndexType)
	}
	if sr.NeedsReferences {
		required = append(required, ReferenceIndexType)
	}
	if sr.NeedsCallGraph {
		required = append(required, CallGraphIndexType)
	}
	if sr.NeedsPostings {
		required = append(required, PostingsIndexType)
	}
	if sr.NeedsLocations {
		required = append(required, LocationIndexType)
	}
	if sr.NeedsContent {
		required = append(required, ContentIndexType)
	}

	return required
}

// RequiresIndex returns true if the specified index type is required
func (sr SearchRequirements) RequiresIndex(indexType IndexType) bool {
	switch indexType {
	case TrigramIndexType:
		return sr.NeedsTrigrams
	case SymbolIndexType:
		return sr.NeedsSymbols
	case ReferenceIndexType:
		return sr.NeedsReferences
	case CallGraphIndexType:
		return sr.NeedsCallGraph
	case PostingsIndexType:
		return sr.NeedsPostings
	case LocationIndexType:
		return sr.NeedsLocations
	case ContentIndexType:
		return sr.NeedsContent
	default:
		return false
	}
}

// Merge combines two SearchRequirements, returning a new requirements that includes all requirements from both
func (sr SearchRequirements) Merge(other SearchRequirements) SearchRequirements {
	return SearchRequirements{
		NeedsTrigrams:   sr.NeedsTrigrams || other.NeedsTrigrams,
		NeedsSymbols:    sr.NeedsSymbols || other.NeedsSymbols,
		NeedsReferences: sr.NeedsReferences || other.NeedsReferences,
		NeedsCallGraph:  sr.NeedsCallGraph || other.NeedsCallGraph,
		NeedsPostings:   sr.NeedsPostings || other.NeedsPostings,
		NeedsLocations:  sr.NeedsLocations || other.NeedsLocations,
		NeedsContent:    sr.NeedsContent || other.NeedsContent,
	}
}

// Intersection returns a new SearchRequirements that includes only requirements present in both
func (sr SearchRequirements) Intersection(other SearchRequirements) SearchRequirements {
	return SearchRequirements{
		NeedsTrigrams:   sr.NeedsTrigrams && other.NeedsTrigrams,
		NeedsSymbols:    sr.NeedsSymbols && other.NeedsSymbols,
		NeedsReferences: sr.NeedsReferences && other.NeedsReferences,
		NeedsCallGraph:  sr.NeedsCallGraph && other.NeedsCallGraph,
		NeedsPostings:   sr.NeedsPostings && other.NeedsPostings,
		NeedsLocations:  sr.NeedsLocations && other.NeedsLocations,
		NeedsContent:    sr.NeedsContent && other.NeedsContent,
	}
}

// IsSubset returns true if this SearchRequirements is a subset of the other requirements
func (sr SearchRequirements) IsSubset(other SearchRequirements) bool {
	return (!sr.NeedsTrigrams || other.NeedsTrigrams) &&
		(!sr.NeedsSymbols || other.NeedsSymbols) &&
		(!sr.NeedsReferences || other.NeedsReferences) &&
		(!sr.NeedsCallGraph || other.NeedsCallGraph) &&
		(!sr.NeedsPostings || other.NeedsPostings) &&
		(!sr.NeedsLocations || other.NeedsLocations) &&
		(!sr.NeedsContent || other.NeedsContent)
}

// GetComplexity returns a score representing the complexity of the search requirements
// Higher values indicate more complex searches that require more indexes
func (sr SearchRequirements) GetComplexity() int {
	complexity := 0
	if sr.NeedsTrigrams {
		complexity += 1
	}
	if sr.NeedsSymbols {
		complexity += 3
	}
	if sr.NeedsReferences {
		complexity += 2
	}
	if sr.NeedsCallGraph {
		complexity += 4
	}
	if sr.NeedsPostings {
		complexity += 1
	}
	if sr.NeedsLocations {
		complexity += 1
	}
	if sr.NeedsContent {
		complexity += 2
	}
	return complexity
}

// Validate validates the search requirements and returns an error if invalid
func (sr SearchRequirements) Validate() error {
	if !sr.HasAnyRequirements() {
		return errors.New("search requirements must specify at least one required index type")
	}

	// Note: Symbol searches typically benefit from trigram filtering, but this is not enforced
	// Note: Reference searches typically need symbol information, but this is not enforced

	if sr.NeedsCallGraph && !sr.NeedsSymbols {
		return errors.New("call graph searches require symbol information")
	}

	return nil
}

// String returns a human-readable string representation of the requirements
func (sr SearchRequirements) String() string {
	var requirements []string

	if sr.NeedsTrigrams {
		requirements = append(requirements, "Trigrams")
	}
	if sr.NeedsSymbols {
		requirements = append(requirements, "Symbols")
	}
	if sr.NeedsReferences {
		requirements = append(requirements, "References")
	}
	if sr.NeedsCallGraph {
		requirements = append(requirements, "CallGraph")
	}
	if sr.NeedsPostings {
		requirements = append(requirements, "Postings")
	}
	if sr.NeedsLocations {
		requirements = append(requirements, "Locations")
	}
	if sr.NeedsContent {
		requirements = append(requirements, "Content")
	}

	if len(requirements) == 0 {
		return "NoRequirements"
	}

	return strings.Join(requirements, "+")
}

// RequirementsBuilder provides a fluent interface for building SearchRequirements
type RequirementsBuilder struct {
	requirements SearchRequirements
}

// NewRequirementsBuilder creates a new RequirementsBuilder
func NewRequirementsBuilder() *RequirementsBuilder {
	return &RequirementsBuilder{
		requirements: NewSearchRequirements(),
	}
}

// WithTrigrams adds trigram requirement
func (rb *RequirementsBuilder) WithTrigrams() *RequirementsBuilder {
	rb.requirements.NeedsTrigrams = true
	return rb
}

// WithSymbols adds symbol requirement
func (rb *RequirementsBuilder) WithSymbols() *RequirementsBuilder {
	rb.requirements.NeedsSymbols = true
	return rb
}

// WithReferences adds reference requirement
func (rb *RequirementsBuilder) WithReferences() *RequirementsBuilder {
	rb.requirements.NeedsReferences = true
	return rb
}

// WithCallGraph adds call graph requirement
func (rb *RequirementsBuilder) WithCallGraph() *RequirementsBuilder {
	rb.requirements.NeedsCallGraph = true
	return rb
}

// WithPostings adds postings requirement
func (rb *RequirementsBuilder) WithPostings() *RequirementsBuilder {
	rb.requirements.NeedsPostings = true
	return rb
}

// WithLocations adds location requirement
func (rb *RequirementsBuilder) WithLocations() *RequirementsBuilder {
	rb.requirements.NeedsLocations = true
	return rb
}

// WithContent adds content requirement
func (rb *RequirementsBuilder) WithContent() *RequirementsBuilder {
	rb.requirements.NeedsContent = true
	return rb
}

// WithAll adds all index requirements
func (rb *RequirementsBuilder) WithAll() *RequirementsBuilder {
	return rb.WithTrigrams().WithSymbols().WithReferences().
		WithCallGraph().WithPostings().WithLocations().WithContent()
}

// WithIndexType adds requirement for a specific index type
func (rb *RequirementsBuilder) WithIndexType(indexType IndexType) *RequirementsBuilder {
	switch indexType {
	case TrigramIndexType:
		return rb.WithTrigrams()
	case SymbolIndexType:
		return rb.WithSymbols()
	case ReferenceIndexType:
		return rb.WithReferences()
	case CallGraphIndexType:
		return rb.WithCallGraph()
	case PostingsIndexType:
		return rb.WithPostings()
	case LocationIndexType:
		return rb.WithLocations()
	case ContentIndexType:
		return rb.WithContent()
	default:
		return rb
	}
}

// Build returns the constructed SearchRequirements
func (rb *RequirementsBuilder) Build() SearchRequirements {
	return rb.requirements
}

// AnalyzePattern determines search requirements based on a search pattern
func AnalyzePattern(pattern string) SearchRequirements {
	builder := NewRequirementsBuilder()

	// Basic trigram search always enabled
	builder.WithTrigrams()

	// Analyze pattern for more complex requirements
	pattern = strings.ToLower(pattern)

	// Look for symbol-like patterns
	if strings.Contains(pattern, "func") || strings.Contains(pattern, "function") ||
		strings.Contains(pattern, "class") || strings.Contains(pattern, "struct") ||
		strings.Contains(pattern, "interface") || strings.Contains(pattern, "method") {
		builder.WithSymbols()
	}

	// Look for reference-like patterns
	if strings.Contains(pattern, "ref") || strings.Contains(pattern, "reference") ||
		strings.Contains(pattern, "call") || strings.Contains(pattern, "usage") {
		builder.WithReferences()
	}

	// Look for call graph patterns
	if strings.Contains(pattern, "callgraph") || strings.Contains(pattern, "hierarchy") ||
		strings.Contains(pattern, "tree") || strings.Contains(pattern, "parent") ||
		strings.Contains(pattern, "child") {
		builder.WithCallGraph()
	}

	// Look for content search patterns
	if strings.Contains(pattern, "content") || strings.Contains(pattern, "full") ||
		strings.Contains(pattern, "text") {
		builder.WithContent()
	}

	return builder.Build()
}

// AnalyzeSearchOptions determines search requirements based on search options
// This would be implemented based on the actual search options structure
// For now, it returns comprehensive requirements
func AnalyzeSearchOptions(options interface{}) SearchRequirements {
	// This is a placeholder implementation
	// In a real implementation, this would analyze the search options
	// to determine which indexes are needed
	return NewComprehensiveRequirements()
}
