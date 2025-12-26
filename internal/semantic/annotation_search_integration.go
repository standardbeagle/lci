package semantic

import (
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// AnnotationSearchIndex provides semantic search capabilities through @lci: annotations
// Integrates annotation metadata with the search engine to enable label-based and
// category-based symbol discovery
type AnnotationSearchIndex struct {
	// Annotations from the SemanticAnnotator
	annotator *core.SemanticAnnotator

	// Quick lookup indexes built from annotations
	labelIndex    map[string][]types.SymbolID    // label → [symbolID]
	categoryIndex map[string][]types.SymbolID    // category → [symbolID]
	tagIndex      map[string]map[string][]types.SymbolID // tag:value → [symbolID]

	// Reverse mapping for efficient expansion
	symbolLabelMap    map[types.SymbolID][]string         // symbolID → [label]
	symbolCategoryMap map[types.SymbolID]string           // symbolID → category
	symbolDepsMap     map[types.SymbolID][]core.Dependency // symbolID → [Dependency]
}

// NewAnnotationSearchIndex creates a new annotation search index
func NewAnnotationSearchIndex(annotator *core.SemanticAnnotator) *AnnotationSearchIndex {
	if annotator == nil {
		return &AnnotationSearchIndex{
			labelIndex:        make(map[string][]types.SymbolID),
			categoryIndex:     make(map[string][]types.SymbolID),
			tagIndex:          make(map[string]map[string][]types.SymbolID),
			symbolLabelMap:    make(map[types.SymbolID][]string),
			symbolCategoryMap: make(map[types.SymbolID]string),
			symbolDepsMap:     make(map[types.SymbolID][]core.Dependency),
		}
	}

	idx := &AnnotationSearchIndex{
		annotator:         annotator,
		labelIndex:        make(map[string][]types.SymbolID),
		categoryIndex:     make(map[string][]types.SymbolID),
		tagIndex:          make(map[string]map[string][]types.SymbolID),
		symbolLabelMap:    make(map[types.SymbolID][]string),
		symbolCategoryMap: make(map[types.SymbolID]string),
		symbolDepsMap:     make(map[types.SymbolID][]core.Dependency),
	}

	// Build indexes from annotator
	idx.rebuildIndexes()

	return idx
}

// rebuildIndexes reconstructs all lookup tables from the annotator
func (asi *AnnotationSearchIndex) rebuildIndexes() {
	if asi.annotator == nil {
		return
	}

	// Clear existing indexes
	asi.labelIndex = make(map[string][]types.SymbolID)
	asi.categoryIndex = make(map[string][]types.SymbolID)
	asi.tagIndex = make(map[string]map[string][]types.SymbolID)
	asi.symbolLabelMap = make(map[types.SymbolID][]string)
	asi.symbolCategoryMap = make(map[types.SymbolID]string)
	asi.symbolDepsMap = make(map[types.SymbolID][]core.Dependency)

	// Get all annotated symbols via GetSymbolsByLabel
	// We need to iterate through annotation stats to find all labels
	stats := asi.annotator.GetAnnotationStats()
	labelDist, ok := stats["label_distribution"].(map[string]int)
	if !ok {
		return
	}

	// For each label, get all symbols and build indexes
	for label := range labelDist {
		symbols := asi.annotator.GetSymbolsByLabel(label)
		for _, symbol := range symbols {
			if symbol == nil {
				continue
			}

			symbolID := symbol.SymbolID

			// Build label index
			asi.labelIndex[label] = append(asi.labelIndex[label], symbolID)

			// Build symbol → label mapping
			asi.symbolLabelMap[symbolID] = append(asi.symbolLabelMap[symbolID], label)

			// Build category index if annotation has category
			if symbol.Annotation != nil && symbol.Annotation.Category != "" {
				category := symbol.Annotation.Category
				asi.categoryIndex[category] = append(asi.categoryIndex[category], symbolID)
				asi.symbolCategoryMap[symbolID] = category

				// Build tag indexes
				for tagKey, tagValue := range symbol.Annotation.Tags {
					tagFullKey := tagKey + ":" + tagValue
					if asi.tagIndex[tagFullKey] == nil {
						asi.tagIndex[tagFullKey] = make(map[string][]types.SymbolID)
					}
					asi.tagIndex[tagFullKey][tagKey] = append(asi.tagIndex[tagFullKey][tagKey], symbolID)
				}

				// Store dependencies
				if len(symbol.Annotation.Dependencies) > 0 {
					asi.symbolDepsMap[symbolID] = symbol.Annotation.Dependencies
				}
			}
		}
	}
}

// GetSymbolsByLabel finds all symbols with a specific label
func (asi *AnnotationSearchIndex) GetSymbolsByLabel(label string) []types.SymbolID {
	return asi.labelIndex[label]
}

// GetSymbolsByCategory finds all symbols in a specific category
func (asi *AnnotationSearchIndex) GetSymbolsByCategory(category string) []types.SymbolID {
	return asi.categoryIndex[category]
}

// GetSymbolsByTag finds symbols with a specific tag key=value pair
func (asi *AnnotationSearchIndex) GetSymbolsByTag(tagKey, tagValue string) []types.SymbolID {
	fullKey := tagKey + ":" + tagValue
	if m, ok := asi.tagIndex[fullKey]; ok {
		return m[tagKey]
	}
	return nil
}

// GetSymbolsDependingOn finds symbols that depend on a specific resource
func (asi *AnnotationSearchIndex) GetSymbolsDependingOn(depType, depName string) []types.SymbolID {
	var result []types.SymbolID

	for symbolID, deps := range asi.symbolDepsMap {
		for _, dep := range deps {
			if dep.Type == depType && dep.Name == depName {
				result = append(result, symbolID)
				break
			}
		}
	}

	return result
}

// ExpandQueryWithAnnotations expands a search query using annotation metadata
// Returns the original pattern plus any symbol names found by label/category
func (asi *AnnotationSearchIndex) ExpandQueryWithAnnotations(pattern string, labels []string, categories []string) []string {
	expansions := []string{pattern} // Always include original

	// Expand by labels
	for _, label := range labels {
		symbolIDs := asi.GetSymbolsByLabel(label)
		// Note: We'd need symbol lookup to get actual names
		// For now, just track that we found matching symbols
		_ = symbolIDs
	}

	// Expand by categories
	for _, category := range categories {
		symbolIDs := asi.GetSymbolsByCategory(category)
		_ = symbolIDs
	}

	// Remove duplicates and return
	seen := make(map[string]bool)
	var unique []string
	for _, exp := range expansions {
		if !seen[exp] {
			unique = append(unique, exp)
			seen[exp] = true
		}
	}

	return unique
}

// GetAnnotation retrieves annotation for a specific symbol
func (asi *AnnotationSearchIndex) GetAnnotation(symbolID types.SymbolID) *core.SemanticAnnotation {
	if asi.annotator == nil {
		return nil
	}

	// We'd need to extract fileID from symbolID or maintain separate mapping
	// For now, return nil - full implementation requires indexer integration
	return nil
}

// GetLabelsForSymbol gets all labels assigned to a symbol
func (asi *AnnotationSearchIndex) GetLabelsForSymbol(symbolID types.SymbolID) []string {
	return asi.symbolLabelMap[symbolID]
}

// GetCategoryForSymbol gets the category assigned to a symbol
func (asi *AnnotationSearchIndex) GetCategoryForSymbol(symbolID types.SymbolID) string {
	return asi.symbolCategoryMap[symbolID]
}

// GetDependenciesForSymbol gets the dependencies declared by a symbol
func (asi *AnnotationSearchIndex) GetDependenciesForSymbol(symbolID types.SymbolID) []core.Dependency {
	return asi.symbolDepsMap[symbolID]
}

// Statistics and analysis

// GetLabelStats returns statistics about label usage
func (asi *AnnotationSearchIndex) GetLabelStats() map[string]int {
	stats := make(map[string]int)
	for label, symbolIDs := range asi.labelIndex {
		stats[label] = len(symbolIDs)
	}
	return stats
}

// GetCategoryStats returns statistics about category usage
func (asi *AnnotationSearchIndex) GetCategoryStats() map[string]int {
	stats := make(map[string]int)
	for category, symbolIDs := range asi.categoryIndex {
		stats[category] = len(symbolIDs)
	}
	return stats
}

// GetTotalAnnotatedSymbols returns the total number of annotated symbols
func (asi *AnnotationSearchIndex) GetTotalAnnotatedSymbols() int {
	return len(asi.symbolLabelMap)
}

// GetAnnotationCoverage returns the percentage of symbols that have annotations
// Requires knowledge of total symbols in index (passed as parameter)
func (asi *AnnotationSearchIndex) GetAnnotationCoverage(totalSymbols int) float64 {
	if totalSymbols == 0 {
		return 0.0
	}
	return float64(asi.GetTotalAnnotatedSymbols()) / float64(totalSymbols) * 100.0
}

// SearchQueryBuilder helps construct complex queries using annotations
type SearchQueryBuilder struct {
	index     *AnnotationSearchIndex
	labels    []string
	categories []string
	tags      map[string]string
	deps      map[string]string // depType:depName
}

// NewSearchQueryBuilder creates a new query builder
func NewSearchQueryBuilder(index *AnnotationSearchIndex) *SearchQueryBuilder {
	return &SearchQueryBuilder{
		index:      index,
		labels:     make([]string, 0),
		categories: make([]string, 0),
		tags:       make(map[string]string),
		deps:       make(map[string]string),
	}
}

// WithLabel adds a label filter
func (sqb *SearchQueryBuilder) WithLabel(label string) *SearchQueryBuilder {
	sqb.labels = append(sqb.labels, label)
	return sqb
}

// WithCategory adds a category filter
func (sqb *SearchQueryBuilder) WithCategory(category string) *SearchQueryBuilder {
	sqb.categories = append(sqb.categories, category)
	return sqb
}

// WithTag adds a tag filter (key=value)
func (sqb *SearchQueryBuilder) WithTag(key, value string) *SearchQueryBuilder {
	sqb.tags[key] = value
	return sqb
}

// WithDependency adds a dependency filter (type:name)
func (sqb *SearchQueryBuilder) WithDependency(depType, depName string) *SearchQueryBuilder {
	depKey := depType + ":" + depName
	sqb.deps[depKey] = depKey
	return sqb
}

// Execute runs the query and returns matching symbol IDs
func (sqb *SearchQueryBuilder) Execute() []types.SymbolID {
	resultSets := make([]map[types.SymbolID]bool, 0)

	// Collect symbols from each filter
	for _, label := range sqb.labels {
		symbolSet := make(map[types.SymbolID]bool)
		for _, sid := range sqb.index.GetSymbolsByLabel(label) {
			symbolSet[sid] = true
		}
		resultSets = append(resultSets, symbolSet)
	}

	for _, category := range sqb.categories {
		symbolSet := make(map[types.SymbolID]bool)
		for _, sid := range sqb.index.GetSymbolsByCategory(category) {
			symbolSet[sid] = true
		}
		resultSets = append(resultSets, symbolSet)
	}

	for tagKey, tagValue := range sqb.tags {
		symbolSet := make(map[types.SymbolID]bool)
		for _, sid := range sqb.index.GetSymbolsByTag(tagKey, tagValue) {
			symbolSet[sid] = true
		}
		resultSets = append(resultSets, symbolSet)
	}

	for depKey := range sqb.deps {
		// Parse depKey as type:name
		parts := make([]string, 0)
		for i := 0; i < len(depKey); i++ {
			if depKey[i] == ':' {
				parts = append(parts, depKey[:i])
				parts = append(parts, depKey[i+1:])
				break
			}
		}
		if len(parts) == 2 {
			symbolSet := make(map[types.SymbolID]bool)
			for _, sid := range sqb.index.GetSymbolsDependingOn(parts[0], parts[1]) {
				symbolSet[sid] = true
			}
			resultSets = append(resultSets, symbolSet)
		}
	}

	// Intersect all result sets (AND logic)
	if len(resultSets) == 0 {
		return nil
	}

	result := make([]types.SymbolID, 0)
	for symbolID := range resultSets[0] {
		inAll := true
		for i := 1; i < len(resultSets); i++ {
			if !resultSets[i][symbolID] {
				inAll = false
				break
			}
		}
		if inAll {
			result = append(result, symbolID)
		}
	}

	return result
}
