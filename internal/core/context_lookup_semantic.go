package core

import (
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// LocationKey is a zero-allocation key type for FileID:Line lookups
// Replaces fmt.Sprintf("%d:%d") in hot paths (50-100MB savings per query)
type LocationKey struct {
	FileID types.FileID
	Line   int
}

// ObjectLocationKey is a zero-allocation key type for Name:FileID lookups
type ObjectLocationKey struct {
	Name   string
	FileID types.FileID
}

// ContextKey is a zero-allocation key type for Name:FileID:Context lookups
type ContextKey struct {
	Name    string
	FileID  types.FileID
	Context string
}

// fillSemanticContext populates semantic meaning and dependency information
func (cle *ContextLookupEngine) fillSemanticContext(context *CodeObjectContext) error {
	objectID := context.ObjectID

	// Get entry point dependencies
	entryPoints, err := cle.getEntryPointDependencies(objectID)
	if err != nil {
		return fmt.Errorf("failed to get entry point dependencies: %w", err)
	}
	context.SemanticContext.EntryPointDependencies = entryPoints

	// Get service dependencies
	services, err := cle.getServiceDependencies(objectID)
	if err != nil {
		return fmt.Errorf("failed to get service dependencies: %w", err)
	}
	context.SemanticContext.ServiceDependencies = services

	// Get propagation labels from LCI annotations
	propagationLabels, err := cle.getPropagationLabels(objectID)
	if err != nil {
		return fmt.Errorf("failed to get propagation labels: %w", err)
	}
	context.SemanticContext.PropagationLabels = propagationLabels

	// Analyze criticality
	criticality, err := cle.analyzeCriticality(objectID)
	if err != nil {
		return fmt.Errorf("failed to analyze criticality: %w", err)
	}
	context.SemanticContext.CriticalityAnalysis = criticality

	// Determine purpose
	purpose, confidence := cle.determinePurpose(objectID)
	context.SemanticContext.Purpose = purpose
	context.SemanticContext.Confidence = confidence

	return nil
}

// getEntryPointDependencies finds all entry points that depend on this object
func (cle *ContextLookupEngine) getEntryPointDependencies(objectID CodeObjectID) ([]EntryPointRef, error) {
	var entryPoints []EntryPointRef

	// Use graph propagator to find entry points
	if cle.graphPropagator != nil {
		// Get all entry points in the codebase
		allEntryPoints := cle.findAllEntryPoints()

		// Check which entry points can reach this object
		for _, ep := range allEntryPoints {
			if cle.canEntryPointReachObject(ep, objectID) {
				entryRef := EntryPointRef{
					EntryPointID: ep,
					Type:         cle.determineEntryPointType(ep),
					Path:         cle.getEntryPointPath(ep),
					Confidence:   0.8,
				}
				entryPoints = append(entryPoints, entryRef)
			}
		}
	}

	// Sort by confidence
	sortEntryPoints(entryPoints)
	return entryPoints, nil
}

// getServiceDependencies identifies external services this object depends on
func (cle *ContextLookupEngine) getServiceDependencies(objectID CodeObjectID) ([]ServiceRef, error) {
	var services []ServiceRef

	// Analyze the object's content to find service calls
	// Using ReferenceTracker.GetEnhancedSymbol() with indexed data
	// Stubbed to return empty result until implemented with indexed data
	// Look for common service patterns
	servicePatterns := []struct {
		pattern     string
		serviceType string
		operation   string
	}{
		{`\.get\(.*\)|\.post\(.*\)|\.put\(.*\)|\.delete\(.*\)`, "http", "api_call"},
		{`\.query\(.*\)|\.execute\(.*\)|\.fetch\(.*\)`, "database", "query"},
		{`\.send\(.*\)|\.publish\(.*\)|\.emit\(.*\)`, "message_queue", "message"},
		{`\.connect\(.*\)|\.dial\(.*\)|\.listen\(.*\)`, "network", "connection"},
		{`\.readFile\(.*\)|\.writeFile\(.*\)|\.open\(.*\)`, "filesystem", "io"},
	}

	// Service dependency detection using ReferenceTracker indexed data
	// For now, analyze outgoing references for service call patterns

	// Find the symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol != nil {
		// Analyze outgoing call references for service patterns
		for _, ref := range targetSymbol.OutgoingRefs {
			if ref.Type == types.RefTypeCall {
				callName := ref.ReferencedName

				// Check against service patterns
				for _, servicePattern := range servicePatterns {
					// Simple pattern matching on call name
					if strings.Contains(callName, "connect") ||
						strings.Contains(callName, "dial") ||
						strings.Contains(callName, "query") ||
						strings.Contains(callName, "execute") {
						serviceRef := ServiceRef{
							ServiceName:    callName,
							OperationType:  servicePattern.operation,
							DependencyType: "direct",
							Confidence:     0.6, // Lower confidence without full AST analysis
						}
						services = append(services, serviceRef)
						break
					}
				}
			}
		}
	}

	// Deduplicate services
	services = deduplicateServices(services)
	return services, nil
}

// getPropagationLabels extracts LCI @lci: annotation labels that propagate to/from this object
func (cle *ContextLookupEngine) getPropagationLabels(objectID CodeObjectID) ([]PropagationInfo, error) {
	var labels []PropagationInfo

	// Use graph propagator to get REAL propagated labels
	if cle.graphPropagator != nil && cle.symbolIndex != nil {
		// Find the symbol to get its numeric SymbolID
		defResults := cle.symbolIndex.FindDefinitions(objectID.Name)
		for _, result := range defResults {
			if result.FileID == objectID.FileID {
				// Calculate numeric SymbolID from result location
				numericSymbolID := types.SymbolID(result.FileID)<<32 | types.SymbolID(result.Line)<<16 | types.SymbolID(result.Column)

				// Get actual propagated labels from graph propagator
				propagatedLabels := cle.graphPropagator.GetPropagatedLabels(numericSymbolID)

				for _, pLabel := range propagatedLabels {
					// Determine propagation direction based on hops
					direction := "bidirectional"
					if pLabel.Hops > 0 {
						// Propagated from somewhere else
						direction = "upstream" // Received from source
					}

					propInfo := PropagationInfo{
						Label: pLabel.Label,
						Source: CodeObjectID{
							FileID:   types.FileID(pLabel.Source >> 32),
							SymbolID: fmt.Sprintf("%d", pLabel.Source),
							Name:     "", // Would need additional lookup
						},
						Strength:  pLabel.Strength,
						Direction: direction,
					}
					labels = append(labels, propInfo)
				}
				break // Found the symbol, stop searching
			}
		}
	}

	// Also look for direct @lci: annotations on this object
	directAnnotations := cle.getDirectLCIAnnotations(objectID)
	for _, annotation := range directAnnotations {
		propInfo := PropagationInfo{
			Label:     annotation.Label,
			Source:    objectID, // Source is the object itself
			Strength:  1.0,      // Direct annotations have full strength
			Direction: "bidirectional",
		}
		labels = append(labels, propInfo)
	}

	return labels, nil
}

// analyzeCriticality determines the criticality of the object
func (cle *ContextLookupEngine) analyzeCriticality(objectID CodeObjectID) (CriticalityInfo, error) {
	criticality := CriticalityInfo{
		IsCritical: false,
	}

	// Check propagation labels for criticality indicators
	labels, err := cle.getPropagationLabels(objectID)
	if err != nil {
		return criticality, err
	}

	for _, label := range labels {
		if strings.Contains(label.Label, "critical") || strings.Contains(label.Label, "security") {
			criticality.IsCritical = true
			criticality.CriticalityType = determineCriticalityType(label.Label)
			criticality.ImpactScore = calculateImpactScore(label.Strength, label.Direction)
			break
		}
	}

	// If not marked as critical, analyze usage patterns
	if !criticality.IsCritical {
		if cle.isSecurityCritical(objectID) {
			criticality.IsCritical = true
			criticality.CriticalityType = "security"
			criticality.ImpactScore = 8.0
		} else if cle.isPerformanceCritical(objectID) {
			criticality.IsCritical = true
			criticality.CriticalityType = "performance"
			criticality.ImpactScore = 6.0
		} else if cle.isBusinessLogicCritical(objectID) {
			criticality.IsCritical = true
			criticality.CriticalityType = "business-logic"
			criticality.ImpactScore = 7.0
		}
	}

	// Get affected components
	if criticality.IsCritical {
		criticality.AffectedComponents = cle.getAffectedComponents(objectID)
	}

	return criticality, nil
}

// determinePurpose analyzes the object to determine its purpose
func (cle *ContextLookupEngine) determinePurpose(objectID CodeObjectID) (string, float64) {
	// Analyze name patterns
	name := strings.ToLower(objectID.Name)

	// Check for common patterns
	if strings.Contains(name, "handler") || strings.Contains(name, "controller") {
		return "API handler", 0.9
	}
	if strings.Contains(name, "middleware") {
		return "middleware", 0.9
	}
	if strings.Contains(name, "util") || strings.Contains(name, "helper") {
		return "utility function", 0.8
	}
	if strings.Contains(name, "config") || strings.Contains(name, "settings") {
		return "configuration", 0.9
	}
	if strings.Contains(name, "test") || strings.Contains(name, "spec") {
		return "test", 0.9
	}
	if strings.Contains(name, "model") || strings.Contains(name, "entity") {
		return "data model", 0.8
	}
	if strings.Contains(name, "service") {
		return "service layer", 0.8
	}
	if strings.Contains(name, "repo") || strings.Contains(name, "repository") {
		return "data access", 0.8
	}

	// Analyze call patterns for more insight
	if cle.makesAPICalls(objectID) {
		return "API client", 0.7
	}
	if cle.accessesDatabase(objectID) {
		return "data access", 0.7
	}
	if cle.processesBusinessLogic(objectID) {
		return "business logic", 0.6
	}

	// Default purpose based on type
	switch objectID.Type {
	case types.SymbolTypeFunction:
		return "function", 0.5
	case types.SymbolTypeClass:
		return "class", 0.5
	case types.SymbolTypeMethod:
		return "method", 0.5
	case types.SymbolTypeInterface:
		return "interface", 0.5
	default:
		return "unknown", 0.3
	}
}

// Helper functions

func (cle *ContextLookupEngine) findAllEntryPoints() []CodeObjectID {
	var entryPoints []CodeObjectID
	seen := make(map[LocationKey]bool) // Use struct key (50-100MB savings)

	if cle.symbolIndex == nil {
		return entryPoints
	}

	// Pattern 1: Find all "main" functions
	mainResults := cle.symbolIndex.FindDefinitions("main")
	for _, result := range mainResults {
		// The Match field contains the symbol name
		symbolName := "main" // We searched for "main"
		key := LocationKey{FileID: result.FileID, Line: result.Line}
		if !seen[key] {
			seen[key] = true
			// Store string representation for SymbolID (only when needed)
			symbolID := fmt.Sprintf("%d:%d", result.FileID, result.Line)
			entryPoints = append(entryPoints, CodeObjectID{
				FileID:   result.FileID,
				SymbolID: symbolID,
				Name:     symbolName,
				Type:     types.SymbolTypeFunction,
			})
		}
	}

	// Pattern 2: Find HTTP handlers (functions with "Handler", "handler", "Handle", "handle" in name)
	// Note: FindDefinitions does fuzzy matching, so searching for "handler" will find "HandleUserRequest" etc.
	handlerPatterns := []string{"handler", "Handler", "endpoint", "Endpoint"}
	for _, pattern := range handlerPatterns {
		results := cle.symbolIndex.FindDefinitions(pattern)
		for _, result := range results {
			key := LocationKey{FileID: result.FileID, Line: result.Line}
			if !seen[key] {
				seen[key] = true
				// Use the Match field which contains the matched text
				symbolName := result.Match
				if symbolName == "" {
					symbolName = pattern // Fallback to pattern
				}
				// Store string representation for SymbolID (only when needed)
				symbolID := fmt.Sprintf("%d:%d", result.FileID, result.Line)
				entryPoints = append(entryPoints, CodeObjectID{
					FileID:   result.FileID,
					SymbolID: symbolID,
					Name:     symbolName,
					Type:     types.SymbolTypeFunction,
				})
			}
		}
	}

	// Pattern 3: Functions annotated as entry points via @lci:category[entry-point]
	if cle.semanticAnnotator != nil {
		entryPointSymbols := cle.semanticAnnotator.GetSymbolsByCategory("entry-point")
		for _, annotated := range entryPointSymbols {
			key := LocationKey{FileID: annotated.FileID, Line: annotated.Symbol.Line}
			if !seen[key] {
				seen[key] = true
				// Store string representation for SymbolID (only when needed)
				symbolID := fmt.Sprintf("%d:%d", annotated.FileID, annotated.Symbol.Line)
				entryPoints = append(entryPoints, CodeObjectID{
					FileID:   annotated.FileID,
					SymbolID: symbolID,
					Name:     annotated.Symbol.Name,
					Type:     annotated.Symbol.Type,
				})
			}
		}
	}

	return entryPoints
}

func (cle *ContextLookupEngine) canEntryPointReachObject(entryPoint CodeObjectID, target CodeObjectID) bool {
	if cle.refTracker == nil {
		return false
	}

	// RefTracker uses function names for lookups
	entryName := entryPoint.Name
	targetName := target.Name

	// Find the entry point symbol
	entrySymbols := cle.refTracker.FindSymbolsByName(entryName)
	if len(entrySymbols) == 0 {
		// Symbol not found - this could mean the function doesn't exist or wasn't indexed
		// This is not necessarily an error, just means we can't determine reachability
		return false
	}

	// BFS from entry point to target using function names
	visited := make(map[string]bool)
	queue := []string{entryName}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == targetName {
			return true // Found path!
		}

		if visited[current] {
			continue
		}
		visited[current] = true

		// Get all callees from this function using RefTracker
		currentSymbols := cle.refTracker.FindSymbolsByName(current)
		if len(currentSymbols) > 0 {
			callees := cle.refTracker.GetCalleeNames(currentSymbols[0].ID)
			for _, callee := range callees {
				if !visited[callee] {
					queue = append(queue, callee)
				}
			}
		}
	}

	return false
}

func (cle *ContextLookupEngine) determineEntryPointType(entryPoint CodeObjectID) string {
	name := strings.ToLower(entryPoint.Name)

	if name == "main" || name == "_main" {
		return "main function"
	}
	if strings.Contains(name, "handler") || strings.Contains(name, "serve") {
		return "HTTP endpoint"
	}
	if strings.Contains(name, "cmd") || strings.Contains(name, "command") {
		return "CLI command"
	}
	if strings.Contains(name, "test") {
		return "test entry point"
	}

	return "unknown entry point"
}

func (cle *ContextLookupEngine) getEntryPointPath(entryPoint CodeObjectID) string {
	// Extract path information for HTTP endpoints, CLI commands, etc.
	// Look for comment annotations above the function like:
	// - "GET /api/users"
	// - "POST /api/users/create"
	// - "Command: migrate-db"
	// - "@route PUT /api/users/:id"

	// Using EnhancedSymbol.DocComment from indexed data to extract path annotations
	// For now, try to extract from DocComment if available
	symbols := cle.refTracker.FindSymbolsByName(entryPoint.Name)
	for _, sym := range symbols {
		if sym.FileID == entryPoint.FileID && sym.Type == entryPoint.Type {
			// Parse DocComment for route/path information
			return extractPathFromDocComment(sym.DocComment)
		}
	}

	return ""
}

// extractPathFromDocComment extracts route/path information from doc comments
func extractPathFromDocComment(docComment string) string {
	if docComment == "" {
		return ""
	}

	// Split by lines
	lines := strings.Split(docComment, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for HTTP method patterns (GET, POST, PUT, DELETE, PATCH, WS)
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "WS"}
		for _, method := range methods {
			if strings.HasPrefix(line, method+" ") {
				return line
			}
		}

		// Check for @route annotation
		if strings.HasPrefix(line, "@route ") {
			return strings.TrimPrefix(line, "@route ")
		}

		// Check for Command: pattern
		if strings.HasPrefix(line, "Command:") {
			cmd := strings.TrimPrefix(line, "Command:")
			return strings.TrimSpace(cmd)
		}
	}

	return ""
}

func extractServiceName(methodCall string) string {
	// Extract service name from method call
	// e.g., "database.query()" -> "database"
	parts := strings.Split(methodCall, ".")
	if len(parts) >= 2 {
		return parts[0]
	}
	return "unknown"
}

func deduplicateServices(services []ServiceRef) []ServiceRef {
	seen := make(map[string]bool)
	var unique []ServiceRef

	for _, service := range services {
		key := fmt.Sprintf("%s:%s", service.ServiceName, service.OperationType)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, service)
		}
	}

	return unique
}

func sortEntryPoints(entryPoints []EntryPointRef) {
	// Sort by confidence (descending)
	for i := 0; i < len(entryPoints)-1; i++ {
		for j := i + 1; j < len(entryPoints); j++ {
			if entryPoints[i].Confidence < entryPoints[j].Confidence {
				entryPoints[i], entryPoints[j] = entryPoints[j], entryPoints[i]
			}
		}
	}
}

func (cle *ContextLookupEngine) getDirectLCIAnnotations(objectID CodeObjectID) []struct {
	Label string
} {
	// Use SemanticAnnotator to get actual annotations
	if cle.semanticAnnotator == nil {
		return []struct{ Label string }{}
	}

	// Get annotation stats to find all labels
	stats := cle.semanticAnnotator.GetAnnotationStats()
	labelDistribution, ok := stats["label_distribution"].(map[string]int)
	if !ok {
		return []struct{ Label string }{}
	}

	var result []struct{ Label string }

	// Iterate through all labels to find ones that match our symbol
	for label := range labelDistribution {
		symbols := cle.semanticAnnotator.GetSymbolsByLabel(label)
		for _, annotatedSymbol := range symbols {
			if annotatedSymbol.Symbol.Name == objectID.Name && annotatedSymbol.FileID == objectID.FileID {
				// Found matching symbol with this label
				result = append(result, struct{ Label string }{Label: label})
				break
			}
		}
	}

	return result
}

func determineCriticalityType(label string) string {
	if strings.Contains(label, "security") {
		return "security"
	}
	if strings.Contains(label, "performance") {
		return "performance"
	}
	if strings.Contains(label, "bug") {
		return "correctness"
	}
	return "general"
}

func calculateImpactScore(strength float64, direction string) float64 {
	score := strength * 10.0 // Convert to 1-10 scale

	if direction == "bidirectional" {
		score *= 1.2 // Higher impact for bidirectional
	}

	if score > 10.0 {
		score = 10.0
	}

	return score
}

func (cle *ContextLookupEngine) isSecurityCritical(objectID CodeObjectID) bool {
	// Check if object handles authentication, authorization, encryption, etc.
	name := strings.ToLower(objectID.Name)
	securityKeywords := []string{"auth", "password", "token", "encrypt", "decrypt", "hash", "validate", "sanitize"}

	for _, keyword := range securityKeywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}

	return false
}

func (cle *ContextLookupEngine) isPerformanceCritical(objectID CodeObjectID) bool {
	// Check if object is in hot paths (frequently called)
	// A function is performance critical if it has many callers (indicating high frequency usage)

	if cle.refTracker == nil {
		return false
	}

	// Find the symbol and get all callers using RefTracker
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return false
	}

	callers := cle.refTracker.GetCallerNames(symbols[0].ID)

	// Threshold: 100+ callers indicates a hot path
	// This is a heuristic that could be tuned based on project size
	const hotPathThreshold = 100

	return len(callers) >= hotPathThreshold
}

func (cle *ContextLookupEngine) isBusinessLogicCritical(objectID CodeObjectID) bool {
	// Check if object contains core business rules
	// Business logic is critical if it contains calculations, validations, or authorization
	// and has business-related naming

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return false
	}

	// Check for business-related keywords in the name
	name := strings.ToLower(objectID.Name)
	businessKeywords := []string{
		"payment", "price", "discount", "authorize", "validate", "order",
		"calculate", "process", "approve", "charge", "refund", "invoice",
		"bill", "transaction", "apply", "check", "verify",
	}

	hasBusinessNaming := false
	for _, keyword := range businessKeywords {
		if strings.Contains(name, keyword) {
			hasBusinessNaming = true
			break
		}
	}

	if !hasBusinessNaming {
		return false
	}

	// Check if it actually contains business logic (not just a simple getter/setter)
	return cle.processesBusinessLogic(objectID)
}

func (cle *ContextLookupEngine) getAffectedComponents(objectID CodeObjectID) []string {
	// Find all components that would be affected if this object changes
	// Components are inferred from caller function names (e.g., HandleRequest -> "handler")

	var components []string
	seen := make(map[string]bool)

	// Get all functions that call this object using RefTracker
	if cle.refTracker == nil {
		return components
	}

	// Find the symbol
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	if len(symbols) == 0 {
		return components
	}

	callers := cle.refTracker.GetCallerNames(symbols[0].ID)

	for _, callerName := range callers {
		// Extract component name from caller function name
		component := extractComponentName(callerName)
		if component != "" && !seen[component] {
			seen[component] = true
			components = append(components, component)
		}
	}

	return components
}

// extractComponentName extracts component name from function name
// E.g., "HandleRequest" -> "handler", "ServiceProcess" -> "service"
func extractComponentName(functionName string) string {
	lower := strings.ToLower(functionName)

	// Common component patterns - more specific patterns first
	componentPatterns := []string{
		"handler", "handle",
		"repository", "repo",
		"service",
		"controller",
		"api",
		"web",
		"database", "db",
		"cache",
		"util", "helper",
	}

	// Find the LAST match (most specific) in the function name
	// For "HandleAPI", we want "api" not "handle"
	var bestMatch string
	var bestPos int = -1

	for _, pattern := range componentPatterns {
		pos := strings.Index(lower, pattern)
		if pos > bestPos {
			bestPos = pos
			bestMatch = pattern
		}
	}

	return bestMatch
}

func (cle *ContextLookupEngine) makesAPICalls(objectID CodeObjectID) bool {
	// Check if object makes HTTP API calls by looking for http package usage

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return false
	}

	// Using ReferenceTracker indexed data to check for http/net package calls
	// For now, check outgoing references for http-related calls
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Check outgoing references for http/net calls
			for _, ref := range sym.OutgoingRefs {
				if ref.Type == types.RefTypeCall {
					callName := strings.ToLower(ref.ReferencedName)
					// Simple heuristic: check for http-related call names
					if strings.Contains(callName, "http") ||
						strings.Contains(callName, "get") ||
						strings.Contains(callName, "post") ||
						strings.Contains(callName, "request") {
						return true
					}
				}
			}
		}
	}

	return false
}

// containsHTTPCalls checks if a node contains HTTP API calls
func (cle *ContextLookupEngine) containsHTTPCalls(node *sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}

	nodeKind := node.Kind()

	// Check if this is a selector_expression like http.Get, http.Post, etc.
	// or qualified_type like http.Client
	if nodeKind == "selector_expression" || nodeKind == "qualified_type" {
		// Check if it's accessing the http package
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "identifier" || child.Kind() == "package_identifier" {
				name := string(content[child.StartByte():child.EndByte()])
				// Check for http package
				if name == "http" {
					return true
				}
			}
		}
	}

	// Recursively check children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if cle.containsHTTPCalls(child, content) {
			return true
		}
	}

	return false
}

func (cle *ContextLookupEngine) accessesDatabase(objectID CodeObjectID) bool {
	// Check if object performs database operations by looking for database package usage

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return false
	}

	// Using ReferenceTracker indexed data to check for database package calls
	// For now, check outgoing references for database-related calls
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Check outgoing references for database calls
			for _, ref := range sym.OutgoingRefs {
				if ref.Type == types.RefTypeCall {
					callName := strings.ToLower(ref.ReferencedName)
					// Simple heuristic: check for database-related call names
					if strings.Contains(callName, "sql") ||
						strings.Contains(callName, "query") ||
						strings.Contains(callName, "exec") ||
						strings.Contains(callName, "db") ||
						strings.Contains(callName, "database") {
						return true
					}
				}
			}
		}
	}

	return false
}

// containsDatabaseAccess checks if a node contains database access patterns
func (cle *ContextLookupEngine) containsDatabaseAccess(node *sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}

	nodeKind := node.Kind()

	// Check if this is a selector_expression or qualified_type with database packages
	if nodeKind == "selector_expression" || nodeKind == "qualified_type" {
		// Check if it's accessing database-related packages
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "identifier" || child.Kind() == "package_identifier" {
				name := string(content[child.StartByte():child.EndByte()])
				// Check for common database packages
				// sql, gorm, mongo, redis, etc.
				if name == "sql" || name == "gorm" || name == "mongo" || name == "redis" {
					return true
				}
			}
		}
	}

	// Recursively check children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if cle.containsDatabaseAccess(child, content) {
			return true
		}
	}

	return false
}

func (cle *ContextLookupEngine) processesBusinessLogic(objectID CodeObjectID) bool {
	// Check if object contains business logic patterns
	// Business logic is indicated by conditional statements, loops, or calculations
	// Simple getters/setters are NOT business logic

	// Only check functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return false
	}

	// Use EnhancedSymbol.Complexity as heuristic
	// For now, use complexity as proxy for business logic
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Complexity > 5 suggests business logic (conditionals, loops, etc.)
			// Simple getters/setters typically have complexity 1-2
			if sym.Complexity > 5 {
				return true
			}

			// Also check if it has multiple outgoing calls (orchestration)
			callCount := 0
			for _, ref := range sym.OutgoingRefs {
				if ref.Type == types.RefTypeCall {
					callCount++
				}
			}
			// Multiple calls suggest orchestration/business logic
			if callCount > 3 {
				return true
			}
		}
	}

	return false
}

// containsBusinessLogicPatterns checks if a node contains business logic patterns
func (cle *ContextLookupEngine) containsBusinessLogicPatterns(node *sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}

	nodeKind := node.Kind()

	// Check for control flow structures (strong indicators of business logic)
	if nodeKind == "if_statement" || nodeKind == "for_statement" ||
		nodeKind == "expression_switch_statement" || nodeKind == "type_switch_statement" {
		return true
	}

	// Recursively check children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if cle.containsBusinessLogicPatterns(child, content) {
			return true
		}
	}

	return false
}
