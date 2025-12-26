package core

import (
	"fmt"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// fillAIContext populates AI-enhanced understanding of the code object
func (cle *ContextLookupEngine) fillAIContext(context *CodeObjectContext) error {
	objectID := context.ObjectID

	// Generate natural language summary
	summary, err := cle.generateNaturalLanguageSummary(context)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}
	context.AIContext.NaturalLanguageSummary = summary

	// Find similar objects
	similar, err := cle.findSimilarObjects(objectID)
	if err != nil {
		return fmt.Errorf("failed to find similar objects: %w", err)
	}
	context.AIContext.SimilarObjects = similar

	// Generate refactoring suggestions
	suggestions, err := cle.generateRefactoringSuggestions(context)
	if err != nil {
		return fmt.Errorf("failed to generate refactoring suggestions: %w", err)
	}
	context.AIContext.RefactoringSuggestions = suggestions

	// Detect code smells
	smells, err := cle.detectCodeSmells(context)
	if err != nil {
		return fmt.Errorf("failed to detect code smells: %w", err)
	}
	context.AIContext.CodeSmells = smells

	// Suggest best practices
	practices, err := cle.suggestBestPractices(context)
	if err != nil {
		return fmt.Errorf("failed to suggest best practices: %w", err)
	}
	context.AIContext.BestPractices = practices

	return nil
}

// generateNaturalLanguageSummary creates a human-readable description of the object
func (cle *ContextLookupEngine) generateNaturalLanguageSummary(context *CodeObjectContext) (string, error) {
	objectID := context.ObjectID
	var summary strings.Builder

	// Start with basic description
	summary.WriteString(fmt.Sprintf("This %s `%s`", objectID.Type, objectID.Name))

	// Add purpose information
	if context.SemanticContext.Purpose != "" {
		summary.WriteString(" is a " + context.SemanticContext.Purpose)
	}

	// Add location information
	filePath := cle.fileService.GetPathForFileID(objectID.FileID)
	if filePath != "" {
		summary.WriteString(" located in " + filePath)
	}

	// Add relationship information
	if len(context.DirectRelationships.IncomingReferences) > 0 {
		summary.WriteString(fmt.Sprintf(" that is referenced by %d other objects", len(context.DirectRelationships.IncomingReferences)))
	}

	if len(context.DirectRelationships.CalledFunctions) > 0 {
		summary.WriteString(fmt.Sprintf(" and calls %d other functions", len(context.DirectRelationships.CalledFunctions)))
	}

	// Add complexity information
	if context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity > 5 {
		summary.WriteString(fmt.Sprintf(" with high cyclomatic complexity (%d)", context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity))
	}

	// Add criticality information
	if context.SemanticContext.CriticalityAnalysis.IsCritical {
		summary.WriteString(fmt.Sprintf(" and is marked as %s-critical", context.SemanticContext.CriticalityAnalysis.CriticalityType))
	}

	// Add usage information
	if context.UsageAnalysis.CallFrequency > 0 {
		summary.WriteString(fmt.Sprintf(". It is called approximately %d times", context.UsageAnalysis.CallFrequency))
	}

	// Add test coverage information
	if context.UsageAnalysis.TestCoverage.HasTests {
		summary.WriteString(" and has test coverage")
	} else {
		summary.WriteString(" but lacks test coverage")
	}

	summary.WriteString(".")

	// Add dependency information
	if len(context.SemanticContext.ServiceDependencies) > 0 {
		summary.WriteString(fmt.Sprintf(" It depends on %d external services including ", len(context.SemanticContext.ServiceDependencies)))

		services := context.SemanticContext.ServiceDependencies
		if len(services) > 3 {
			summary.WriteString(services[0].ServiceName + " and others")
		} else {
			for i, service := range services {
				if i > 0 {
					summary.WriteString(", ")
				}
				summary.WriteString(service.ServiceName)
			}
		}
		summary.WriteString(".")
	}

	// Add change impact information
	if context.UsageAnalysis.ChangeImpact.EstimatedImpact > 7 {
		summary.WriteString(" Changes to this object would have high impact on the system.")
	}

	return summary.String(), nil
}

// findSimilarObjects finds objects with similar patterns or usage
func (cle *ContextLookupEngine) findSimilarObjects(objectID CodeObjectID) ([]ObjectReference, error) {
	var similar []ObjectReference

	// Find objects with similar names
	nameSimilar := cle.findObjectsWithSimilarName(objectID)
	similar = append(similar, nameSimilar...)

	// Find objects with similar structure
	structureSimilar := cle.findObjectsWithSimilarStructure(objectID)
	similar = append(similar, structureSimilar...)

	// Find objects with similar usage patterns
	usageSimilar := cle.findObjectsWithSimilarUsage(objectID)
	similar = append(similar, usageSimilar...)

	// Deduplicate and limit results
	similar = deduplicateObjectReferences(similar)
	if len(similar) > 5 {
		similar = similar[:5]
	}

	return similar, nil
}

// generateRefactoringSuggestions provides improvement suggestions
func (cle *ContextLookupEngine) generateRefactoringSuggestions(context *CodeObjectContext) ([]string, error) {
	var suggestions []string

	// Analyze complexity
	if context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity > 10 {
		suggestions = append(suggestions, "Consider breaking down this function into smaller functions to reduce cyclomatic complexity")
	}

	if context.UsageAnalysis.ComplexityMetrics.CognitiveComplexity > 15 {
		suggestions = append(suggestions, "High cognitive complexity detected - consider simplifying logic and reducing nesting")
	}

	if context.UsageAnalysis.ComplexityMetrics.ParameterCount > 5 {
		suggestions = append(suggestions, "Too many parameters - consider using a parameter object or configuration struct")
	}

	// Analyze usage patterns
	if context.UsageAnalysis.FanIn > 20 {
		suggestions = append(suggestions, "This function is heavily used - consider adding comprehensive tests and documentation")
	}

	if context.UsageAnalysis.FanOut > 10 {
		suggestions = append(suggestions, "This function calls many other functions - consider applying the facade pattern")
	}

	// Analyze test coverage (note: only detects test file presence, not actual coverage %)
	if !context.UsageAnalysis.TestCoverage.HasTests {
		suggestions = append(suggestions, "Add unit tests to ensure reliability and prevent regressions")
	}

	// Analyze dependencies
	if len(context.SemanticContext.ServiceDependencies) > 3 {
		suggestions = append(suggestions, "Multiple service dependencies detected - consider implementing dependency injection")
	}

	// Analyze code smells
	for _, smell := range context.AIContext.CodeSmells {
		if smell.Severity == "high" || smell.Severity == "critical" {
			suggestions = append(suggestions, fmt.Sprintf("Address %s: %s", smell.Type, smell.Description))
		}
	}

	// Analyze naming
	if !hasGoodNaming(context.ObjectID.Name, context.ObjectID.Type) {
		suggestions = append(suggestions, "Consider using more descriptive names to improve code readability")
	}

	// Analyze documentation
	if context.Documentation == "" {
		suggestions = append(suggestions, "Add documentation to explain the purpose and usage of this object")
	}

	return suggestions, nil
}

// detectCodeSmells identifies potential code quality issues
func (cle *ContextLookupEngine) detectCodeSmells(context *CodeObjectContext) ([]CodeSmell, error) {
	var smells []CodeSmell
	objectID := context.ObjectID

	// Long function/method
	if context.UsageAnalysis.ComplexityMetrics.LineCount > 50 {
		smells = append(smells, CodeSmell{
			Type:        "long-function",
			Description: fmt.Sprintf("Function is %d lines long (consider < 30 lines)", context.UsageAnalysis.ComplexityMetrics.LineCount),
			Severity:    determineSeverity(context.UsageAnalysis.ComplexityMetrics.LineCount, 30, 50),
			Location:    context.Location,
		})
	}

	// High cyclomatic complexity
	if context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity > 10 {
		smells = append(smells, CodeSmell{
			Type:        "high-cyclomatic-complexity",
			Description: fmt.Sprintf("Cyclomatic complexity is %d (consider < 10)", context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity),
			Severity:    determineSeverity(context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity, 10, 15),
			Location:    context.Location,
		})
	}

	// God class (too many responsibilities)
	if objectID.Type == types.SymbolTypeClass && len(context.DirectRelationships.ChildObjects) > 15 {
		smells = append(smells, CodeSmell{
			Type:        "god-class",
			Description: fmt.Sprintf("Class has %d methods/fields (consider splitting responsibilities)", len(context.DirectRelationships.ChildObjects)),
			Severity:    determineSeverity(len(context.DirectRelationships.ChildObjects), 10, 15),
			Location:    context.Location,
		})
	}

	// Feature envy (uses more data from other classes than its own)
	if cle.hasFeatureEnvy(objectID) {
		smells = append(smells, CodeSmell{
			Type:        "feature-envy",
			Description: "Object uses more data from other objects than its own - consider moving methods",
			Severity:    "medium",
			Location:    context.Location,
		})
	}

	// Data clumps (groups of parameters that appear together)
	if cle.hasDataClumps(objectID) {
		smells = append(smells, CodeSmell{
			Type:        "data-clumps",
			Description: "Groups of parameters appear together - consider creating a parameter object",
			Severity:    "medium",
			Location:    context.Location,
		})
	}

	// Inappropriate intimacy (too much knowledge of another class)
	if cle.hasInappropriateIntimacy(objectID) {
		smells = append(smells, CodeSmell{
			Type:        "inappropriate-intimacy",
			Description: "Object has too much knowledge of another class's internals",
			Severity:    "high",
			Location:    context.Location,
		})
	}

	// Shotgun surgery (changing this requires changes in many places)
	if context.UsageAnalysis.ChangeImpact.EstimatedImpact > 8 {
		smells = append(smells, CodeSmell{
			Type:        "shotgun-surgery",
			Description: "Changes to this object require modifications in many different places",
			Severity:    "high",
			Location:    context.Location,
		})
	}

	return smells, nil
}

// suggestBestPractices provides recommendations based on best practices
func (cle *ContextLookupEngine) suggestBestPractices(context *CodeObjectContext) ([]string, error) {
	var practices []string
	objectID := context.ObjectID

	// General best practices
	practices = append(practices, "Follow consistent naming conventions across the codebase")
	practices = append(practices, "Write self-documenting code that minimizes the need for comments")

	// Type-specific practices
	switch objectID.Type {
	case types.SymbolTypeFunction, types.SymbolTypeMethod:
		practices = append(practices, "Keep functions small and focused on a single responsibility")
		practices = append(practices, "Use pure functions when possible to improve testability")
		practices = append(practices, "Validate input parameters at the beginning of functions")

	case types.SymbolTypeClass:
		practices = append(practices, "Design classes with high cohesion and low coupling")
		practices = append(practices, "Prefer composition over inheritance when possible")
		practices = append(practices, "Implement meaningful equals and hashCode methods")
	}

	// Context-specific practices
	if context.SemanticContext.CriticalityAnalysis.IsCritical {
		practices = append(practices, "Add comprehensive error handling and logging for critical code")
		practices = append(practices, "Consider adding circuit breakers for external service calls")
	}

	if len(context.SemanticContext.ServiceDependencies) > 0 {
		practices = append(practices, "Implement proper error handling for external service calls")
		practices = append(practices, "Add timeouts and retry logic for network operations")
	}

	if !context.UsageAnalysis.TestCoverage.HasTests {
		practices = append(practices, "Write tests before fixing bugs to ensure the fix works")
		practices = append(practices, "Use test-driven development for new features")
	}

	// Performance practices
	if context.UsageAnalysis.CallFrequency > 100 {
		practices = append(practices, "Consider performance optimizations for frequently called code")
		practices = append(practices, "Add metrics and monitoring for high-traffic functions")
	}

	if context.UsageAnalysis.ComplexityMetrics.CyclomaticComplexity > 5 {
		practices = append(practices, "Consider extracting complex conditions into well-named boolean functions")
	}

	// Security practices
	if strings.Contains(strings.ToLower(objectID.Name), "auth") ||
		strings.Contains(strings.ToLower(objectID.Name), "password") ||
		strings.Contains(strings.ToLower(objectID.Name), "token") {
		practices = append(practices, "Follow security best practices for authentication/authorization code")
		practices = append(practices, "Never log sensitive information like passwords or tokens")
		practices = append(practices, "Use secure coding practices and validate all inputs")
	}

	return practices, nil
}

// Helper functions

func (cle *ContextLookupEngine) findObjectsWithSimilarName(objectID CodeObjectID) []ObjectReference {
	var similar []ObjectReference

	// Simple name similarity based on prefixes and suffixes
	name := objectID.Name
	prefixes := []string{"get", "set", "handle", "process", "validate", "create", "update", "delete"}
	suffixes := []string{"er", "or", "Handler", "Manager", "Service", "Util"}

	// Look for objects with similar patterns
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			// Find other objects with same prefix
			similar = append(similar, cle.findObjectsWithPrefix(prefix)...)
		}
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			// Find other objects with same suffix
			similar = append(similar, cle.findObjectsWithSuffix(suffix)...)
		}
	}

	return similar
}

func (cle *ContextLookupEngine) findObjectsWithSimilarStructure(objectID CodeObjectID) []ObjectReference {
	// Find objects with similar parameter counts, complexity, etc.
	var similar []ObjectReference

	// This would compare structural characteristics
	// For now, return empty slice
	return similar
}

func (cle *ContextLookupEngine) findObjectsWithSimilarUsage(objectID CodeObjectID) []ObjectReference {
	// Find objects with similar call patterns and dependencies
	var similar []ObjectReference

	// This would analyze usage patterns
	// For now, return empty slice
	return similar
}

func (cle *ContextLookupEngine) findObjectsWithPrefix(prefix string) []ObjectReference {
	// Find objects that start with the given prefix
	// This would search the symbol index
	return []ObjectReference{}
}

func (cle *ContextLookupEngine) findObjectsWithSuffix(suffix string) []ObjectReference {
	// Find objects that end with the given suffix
	// This would search the symbol index
	return []ObjectReference{}
}

func deduplicateObjectReferences(refs []ObjectReference) []ObjectReference {
	seen := make(map[ObjectLocationKey]bool) // Use struct key (reduces allocations)
	var unique []ObjectReference

	for _, ref := range refs {
		key := ObjectLocationKey{Name: ref.ObjectID.Name, FileID: ref.ObjectID.FileID}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ref)
		}
	}

	return unique
}

func hasGoodNaming(name string, symbolType types.SymbolType) bool {
	// Check if name follows common conventions
	if len(name) < 3 {
		return false
	}

	// Avoid generic names
	genericNames := []string{"temp", "data", "info", "item", "obj", "val"}
	for _, generic := range genericNames {
		if name == generic {
			return false
		}
	}

	// Check for meaningful names
	return !strings.Contains(name, "temp") && !strings.Contains(name, "tmp")
}

func determineSeverity(value, mediumThreshold, highThreshold int) string {
	if value >= highThreshold {
		return "critical"
	}
	if value >= mediumThreshold {
		return "high"
	}
	return "medium"
}

func (cle *ContextLookupEngine) hasFeatureEnvy(objectID CodeObjectID) bool {
	// Check if object uses more data from other classes than its own
	// This would analyze field access patterns
	return false
}

func (cle *ContextLookupEngine) hasDataClumps(objectID CodeObjectID) bool {
	// Check if the same group of parameters appear together multiple times
	// This would analyze parameter patterns across functions
	return false
}

func (cle *ContextLookupEngine) hasInappropriateIntimacy(objectID CodeObjectID) bool {
	// Check if object has too much knowledge of another class's internals
	// This would analyze access patterns and coupling
	return false
}
