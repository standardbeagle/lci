package core

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// fillUsageAnalysis populates usage metrics and impact analysis
func (cle *ContextLookupEngine) fillUsageAnalysis(context *CodeObjectContext) error {
	objectID := context.ObjectID

	// Calculate call frequency
	callFreq := cle.calculateCallFrequency(objectID)
	context.UsageAnalysis.CallFrequency = callFreq

	// Calculate fan-in (number of callers)
	fanIn := cle.calculateFanIn(objectID)
	context.UsageAnalysis.FanIn = fanIn

	// Calculate fan-out (number of calls)
	fanOut := cle.calculateFanOut(objectID)
	context.UsageAnalysis.FanOut = fanOut

	// Calculate complexity metrics
	complexity, err := cle.calculateComplexityMetrics(objectID)
	if err != nil {
		return fmt.Errorf("failed to calculate complexity metrics: %w", err)
	}
	context.UsageAnalysis.ComplexityMetrics = complexity

	// Analyze change impact
	impact, err := cle.analyzeChangeImpact(objectID)
	if err != nil {
		return fmt.Errorf("failed to analyze change impact: %w", err)
	}
	context.UsageAnalysis.ChangeImpact = impact

	// Get test coverage
	coverage, err := cle.getTestCoverage(objectID)
	if err != nil {
		return fmt.Errorf("failed to get test coverage: %w", err)
	}
	context.UsageAnalysis.TestCoverage = coverage

	return nil
}

// calculateCallFrequency estimates how often the object is called
func (cle *ContextLookupEngine) calculateCallFrequency(objectID CodeObjectID) int64 {
	// This would ideally use runtime profiling data or call frequency tracking
	// For now, we'll estimate based on usage patterns

	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Get all references to this function
	refs := cle.symbolIndex.FindReferences(objectID.Name)

	// Count actual calls (not just references)
	callCount := int64(0)
	for _, ref := range refs {
		if cle.isFunctionCall(ref.FileID, ref.Line, objectID.Name) {
			callCount++
		}
	}

	// Apply heuristics based on context
	callCount = cle.adjustCallFrequency(objectID, callCount)

	return callCount
}

// calculateFanIn counts the number of unique callers
func (cle *ContextLookupEngine) calculateFanIn(objectID CodeObjectID) int {
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Get callers from call graph or symbol references
	callers := make(map[ObjectLocationKey]bool) // Use struct key (reduces allocations)

	refs := cle.symbolIndex.FindReferences(objectID.Name)
	for _, ref := range refs {
		// Find the containing function
		caller := cle.findContainingFunction(ref.FileID, ref.Line, ref.Column)
		if caller != nil {
			callerKey := ObjectLocationKey{Name: caller.Name, FileID: caller.FileID}
			callers[callerKey] = true
		}
	}

	return len(callers)
}

// calculateFanOut counts the number of unique functions called by this object
func (cle *ContextLookupEngine) calculateFanOut(objectID CodeObjectID) int {
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Get called functions from earlier analysis
	called, err := cle.getCalledFunctions(objectID)
	if err != nil {
		return 0
	}

	// Count unique functions
	unique := make(map[ObjectLocationKey]bool) // Use struct key (reduces allocations)
	for _, call := range called {
		key := ObjectLocationKey{Name: call.ObjectID.Name, FileID: call.ObjectID.FileID}
		unique[key] = true
	}

	return len(unique)
}

// calculateComplexityMetrics computes various complexity measures
func (cle *ContextLookupEngine) calculateComplexityMetrics(objectID CodeObjectID) (ComplexityMetrics, error) {
	metrics := ComplexityMetrics{}

	// Calculate cyclomatic complexity using EnhancedSymbol.Complexity from indexed data
	cyclomatic := cle.calculateCyclomaticComplexity(objectID)
	metrics.CyclomaticComplexity = cyclomatic

	// Calculate cognitive complexity
	cognitive := cle.calculateCognitiveComplexity(objectID)
	metrics.CognitiveComplexity = cognitive

	// Count lines of code
	lineCount := cle.countLinesOfCode(objectID)
	metrics.LineCount = lineCount

	// Count parameters
	paramCount := cle.countParameters(objectID)
	metrics.ParameterCount = paramCount

	// Calculate nesting depth
	nestingDepth := cle.calculateNestingDepth(objectID)
	metrics.NestingDepth = nestingDepth

	return metrics, nil
}

// analyzeChangeImpact assesses the impact of changing this object
func (cle *ContextLookupEngine) analyzeChangeImpact(objectID CodeObjectID) (ChangeImpactInfo, error) {
	impact := ChangeImpactInfo{}

	// Analyze dependencies
	fanIn := cle.calculateFanIn(objectID)
	fanOut := cle.calculateFanOut(objectID)

	// Determine breaking change risk
	impact.BreakingChangeRisk = cle.assessBreakingChangeRisk(objectID, fanIn, fanOut)

	// Find dependent components
	impact.DependentComponents = cle.findDependentComponents(objectID)

	// Estimate impact score
	impact.EstimatedImpact = cle.calculateImpactScore(objectID, fanIn, fanOut)

	// Determine if tests are required
	impact.RequiresTests = cle.requiresTestsForChange(objectID)

	return impact, nil
}

// getTestCoverage finds test files related to the object (not coverage measurement)
// Note: Actual coverage percentage requires specialized tools like 'go test -cover'
func (cle *ContextLookupEngine) getTestCoverage(objectID CodeObjectID) (TestCoverageInfo, error) {
	coverage := TestCoverageInfo{}

	// Find test files related to this object
	testFiles := cle.findTestFiles(objectID)
	if len(testFiles) > 0 {
		coverage.HasTests = true
		coverage.TestFilePaths = testFiles
	}

	return coverage, nil
}

// Helper functions for complexity calculation

func (cle *ContextLookupEngine) calculateCyclomaticComplexity(objectID CodeObjectID) int {
	// Cyclomatic complexity = 1 + number of decision points
	// Decision points include: if, for, case, &&, ||

	// Only calculate for functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Use EnhancedSymbol.Complexity from indexed data
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			return sym.Complexity
		}
	}
	return 1 // Default complexity
}

// findFunctionNode finds the AST node for a specific function or method
func (cle *ContextLookupEngine) findFunctionNode(node *sitter.Node, content []byte, name string, symbolType types.SymbolType) *sitter.Node {
	if node == nil {
		return nil
	}

	nodeKind := node.Kind()

	// Check if this is the function/method we're looking for
	if nodeKind == "function_declaration" || nodeKind == "method_declaration" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			childKind := child.Kind()
			if childKind == "identifier" || childKind == "field_identifier" {
				nameText := string(content[child.StartByte():child.EndByte()])
				if nameText == name {
					return node
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if result := cle.findFunctionNode(child, content, name, symbolType); result != nil {
			return result
		}
	}

	return nil
}

// countDecisionPoints recursively counts decision points in an AST subtree
func (cle *ContextLookupEngine) countDecisionPoints(node *sitter.Node, content []byte) int {
	if node == nil {
		return 0
	}

	count := 0
	nodeKind := node.Kind()

	// Decision points in Go:
	// - if_statement
	// - for_statement (includes while-style for loops and range)
	// - expression_case in switch statements (each case)
	// - binary_expression with && or ||
	switch nodeKind {
	case "if_statement":
		count++
	case "for_statement":
		count++
	case "expression_case":
		// Each case in a switch adds 1 to complexity
		count++
	case "binary_expression":
		// Check if it's && or ||
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "&&" || child.Kind() == "||" {
				count++
				break
			}
		}
	}

	// Recursively count in children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		count += cle.countDecisionPoints(child, content)
	}

	return count
}

func (cle *ContextLookupEngine) calculateCognitiveComplexity(objectID CodeObjectID) int {
	// Cognitive complexity is a more human-friendly measure
	// It accounts for nesting depth and cognitive load
	//
	// Key differences from cyclomatic:
	// - Base complexity is 0 (not 1)
	// - Increments for nesting level
	// - Does NOT increment for else (continuation of if)
	// - Increments for each logical operator
	// - Increments for recursion

	// Only calculate for functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Use EnhancedSymbol.Complexity from indexed data
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Cognitive typically similar to cyclomatic for simple functions
			return sym.Complexity
		}
	}
	return 0 // Default cognitive complexity
}

// countCognitiveComplexity recursively counts cognitive complexity with nesting depth
func (cle *ContextLookupEngine) countCognitiveComplexity(node *sitter.Node, content []byte, funcName string, nestingLevel int) int {
	if node == nil {
		return 0
	}

	complexity := 0
	nodeKind := node.Kind()

	// Decision points that increase complexity and nesting
	switch nodeKind {
	case "if_statement":
		// +1 for the if, plus nesting level
		complexity += 1 + nestingLevel

		// Check if this is inside an "else" clause (else if pattern)
		// In Go AST, "else if" appears as: if_statement -> else -> if_statement
		// We need to identify when we're processing the nested if to avoid double-counting

		// Process children with special handling for else clauses
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			childKind := child.Kind()

			// Check if this is an else keyword followed by an if (else if pattern)
			if childKind == "else" {
				// Look ahead to see if there's an if_statement (else if)
				if i+1 < node.ChildCount() {
					nextChild := node.Child(i + 1)
					if nextChild != nil && nextChild.Kind() == "if_statement" {
						// This is "else if" - process the if at current nesting level
						complexity += cle.countCognitiveComplexity(nextChild, content, funcName, nestingLevel)
						i++ // Skip the next if_statement since we just processed it
						continue
					}
				}
				// Regular else - process its block at increased nesting
				if i+1 < node.ChildCount() {
					nextChild := node.Child(i + 1)
					if nextChild != nil && nextChild.Kind() == "block" {
						complexity += cle.countCognitiveComplexity(nextChild, content, funcName, nestingLevel+1)
						i++ // Skip the block since we just processed it
						continue
					}
				}
			} else if childKind == "block" {
				// The main if body - process with increased nesting
				complexity += cle.countCognitiveComplexity(child, content, funcName, nestingLevel+1)
			} else if childKind != "if" && childKind != "(" && childKind != ")" {
				// Other children (condition, etc.) - process at current nesting
				complexity += cle.countCognitiveComplexity(child, content, funcName, nestingLevel)
			}
		}
		return complexity

	case "for_statement":
		// +1 for the for loop, plus nesting level
		complexity += 1 + nestingLevel
		// Process children with increased nesting
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			complexity += cle.countCognitiveComplexity(child, content, funcName, nestingLevel+1)
		}
		return complexity

	case "expression_case":
		// Each case in a switch: +1 (no nesting penalty for cases)
		complexity += 1
		// Process children with current nesting level (cases don't increase nesting)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			complexity += cle.countCognitiveComplexity(child, content, funcName, nestingLevel)
		}
		return complexity

	case "binary_expression":
		// Check if it's && or ||
		hasLogicalOp := false
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "&&" || child.Kind() == "||" {
				complexity += 1 // +1 for each logical operator
				hasLogicalOp = true
			}
		}

		// If we found logical operators, also process the children
		if hasLogicalOp {
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child != nil && child.Kind() != "&&" && child.Kind() != "||" {
					complexity += cle.countCognitiveComplexity(child, content, funcName, nestingLevel)
				}
			}
			return complexity
		}

	case "call_expression":
		// Check for recursion
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Kind() == "identifier" {
				nameText := string(content[child.StartByte():child.EndByte()])
				if nameText == funcName {
					complexity += 1 // +1 for recursion
					break
				}
			}
		}
	}

	// Recursively process children at current nesting level
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		complexity += cle.countCognitiveComplexity(child, content, funcName, nestingLevel)
	}

	return complexity
}

func (cle *ContextLookupEngine) countLinesOfCode(objectID CodeObjectID) int {
	// Count non-empty, non-comment lines within the function/method body

	// Only count for functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Use EnhancedSymbol bounds from indexed data
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Return line count based on symbol bounds from indexed data
			if sym.EndLine > sym.Line {
				return sym.EndLine - sym.Line
			}
		}
	}
	return 0
}

// findFunctionBody finds the block statement (body) of a function/method
func (cle *ContextLookupEngine) findFunctionBody(funcNode *sitter.Node) *sitter.Node {
	if funcNode == nil {
		return nil
	}

	// Look for the block child (function body)
	for i := uint(0); i < funcNode.ChildCount(); i++ {
		child := funcNode.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "block" {
			return child
		}
	}

	return nil
}

// countNonEmptyLines counts non-empty, non-comment lines in a range
func (cle *ContextLookupEngine) countNonEmptyLines(content []byte, startLine, endLine int) int {
	lines := strings.Split(string(content), "\n")
	count := 0

	// Skip the first and last lines (opening and closing braces)
	// Start from startLine+1 to skip the opening brace
	// End at endLine-1 to skip the closing brace
	for lineNum := startLine + 1; lineNum < endLine && lineNum < len(lines); lineNum++ {
		line := strings.TrimSpace(lines[lineNum])

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comment-only lines
		if strings.HasPrefix(line, "//") {
			continue
		}

		// Skip block comment lines
		if strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*/") || (strings.HasPrefix(line, "*") && !strings.Contains(line, "=")) {
			continue
		}

		// This is a code line (may include nested braces, which we do want to count)
		count++
	}

	return count
}

func (cle *ContextLookupEngine) countParameters(objectID CodeObjectID) int {
	// Count function parameters
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Use ParameterCount from EnhancedSymbol indexed data
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Use directly indexed ParameterCount field (most accurate)
			if sym.ParameterCount > 0 {
				return int(sym.ParameterCount)
			}
			// Fallback: parse Signature from indexed data if ParameterCount not set
			if sym.Signature != "" {
				commas := strings.Count(sym.Signature, ",")
				if strings.Contains(sym.Signature, "(") {
					return commas + 1
				}
			}
		}
	}
	return 0
}

// findAndCountParameters recursively searches for a function/method and counts its parameters
func (cle *ContextLookupEngine) findAndCountParameters(node *sitter.Node, functionName string, content []byte) int {
	if node == nil {
		return 0
	}

	nodeKind := node.Kind()

	// Check if this is a function_declaration or method_declaration
	if nodeKind == "function_declaration" || nodeKind == "method_declaration" {
		// Find the name child node
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			childKind := child.Kind()

			// For function_declaration, name is "identifier"
			// For method_declaration, name is "field_identifier"
			if childKind == "identifier" || childKind == "field_identifier" {
				nameText := string(content[child.StartByte():child.EndByte()])
				if nameText == functionName {
					// Found the function! Now find and count parameters
					return cle.countParametersInNode(node, content)
				}
			}
		}
	}

	// Recursively search children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if count := cle.findAndCountParameters(child, functionName, content); count > 0 {
			return count
		}
	}

	return 0
}

// countParametersInNode counts parameters in a function/method declaration node
func (cle *ContextLookupEngine) countParametersInNode(node *sitter.Node, content []byte) int {
	// For method_declaration, there are TWO parameter_list nodes:
	// 1. The receiver (comes first)
	// 2. The actual parameters (comes second)
	// We want the second one.

	isMethod := node.Kind() == "method_declaration"
	paramListCount := 0

	// Find the parameter_list child (second one for methods)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "parameter_list" {
			paramListCount++

			// For methods, skip the first parameter_list (receiver)
			if isMethod && paramListCount == 1 {
				continue
			}

			// Count parameter_declaration children
			count := 0
			for j := uint(0); j < child.ChildCount(); j++ {
				paramChild := child.Child(j)
				if paramChild == nil {
					continue
				}

				if paramChild.Kind() == "parameter_declaration" || paramChild.Kind() == "variadic_parameter_declaration" {
					// Each parameter_declaration might have multiple identifiers
					// e.g., "x, y, z int" is one parameter_declaration with 3 identifiers
					count += cle.countIdentifiersInParameter(paramChild, content)
				}
			}
			return count
		}
	}

	return 0
}

// countIdentifiersInParameter counts identifiers in a parameter_declaration node
func (cle *ContextLookupEngine) countIdentifiersInParameter(node *sitter.Node, content []byte) int {
	// For grouped parameters like "x, y, z int", count all identifiers
	count := 0
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Kind() == "identifier" {
			count++
		} else if child.Kind() == "variadic_argument" {
			// Variadic parameter counts as one
			count++
		}
	}

	// If no identifiers found, it might be an unnamed parameter (just type)
	if count == 0 {
		count = 1
	}

	return count
}

// countParametersFromText counts parameters in a parameter list string
// Handles cases like:
//   - "()" -> 0
//   - "(x int)" -> 1
//   - "(x int, y string)" -> 2
//   - "(x, y, z int)" -> 3
//   - "(args ...string)" -> 1
func (cle *ContextLookupEngine) countParametersFromText(paramsText string) int {
	// Remove outer parentheses and trim
	paramsText = strings.TrimSpace(paramsText)
	if paramsText == "" || paramsText == "()" {
		return 0
	}

	// Remove leading/trailing parens
	paramsText = strings.TrimPrefix(paramsText, "(")
	paramsText = strings.TrimSuffix(paramsText, ")")
	paramsText = strings.TrimSpace(paramsText)

	if paramsText == "" {
		return 0
	}

	// Count commas to estimate parameters, but this is tricky due to:
	// - "x, y, z int" has 2 commas but 3 params
	// - "x int, y string" has 1 comma and 2 params
	// Better approach: count identifier sequences before type keywords

	// For now, use a simpler heuristic:
	// Split by comma and count segments, but handle grouped params
	segments := strings.Split(paramsText, ",")

	count := 0
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		// Count identifiers in this segment
		// Each word before the first type indicator is an identifier
		parts := strings.Fields(segment)
		if len(parts) == 0 {
			continue
		}

		// Check if this segment has multiple identifiers (e.g., "x y z int")
		// Count non-type words before we hit a type
		identCount := 0
		for _, part := range parts {
			// Stop counting when we hit a type keyword or variadic operator
			if isGoTypeKeyword(part) || strings.HasPrefix(part, "...") {
				break
			}
			// Skip receiver indicators
			if part == "*" || part == "[" || part == "]" {
				continue
			}
			identCount++
		}

		if identCount == 0 {
			// No identifiers means this might be just a type (unnamed param)
			identCount = 1
		}

		count += identCount
	}

	return count
}

// isGoTypeKeyword checks if a word is a Go type keyword
func isGoTypeKeyword(word string) bool {
	typeKeywords := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"complex64": true, "complex128": true,
		"bool": true, "byte": true, "rune": true,
		"string": true, "error": true,
		"interface": true, "struct": true, "map": true, "chan": true,
		"func": true,
	}
	return typeKeywords[word]
}

func (cle *ContextLookupEngine) calculateNestingDepth(objectID CodeObjectID) int {
	// Find the maximum nesting depth within the object
	// Nesting structures include: if, for, switch, select

	// Only calculate for functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Use EnhancedSymbol.Complexity as proxy for nesting depth
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Nesting depth correlates with complexity
			// Use complexity / 2 as rough estimate
			if sym.Complexity > 1 {
				return sym.Complexity / 2
			}
		}
	}
	return 0
}

// findMaxNestingDepth recursively finds the maximum nesting depth
func (cle *ContextLookupEngine) findMaxNestingDepth(node *sitter.Node, currentDepth int) int {
	if node == nil {
		return currentDepth
	}

	nodeKind := node.Kind()
	maxDepth := currentDepth

	// Check if this node increases nesting depth
	isNestingNode := false
	switch nodeKind {
	case "if_statement", "for_statement", "switch_statement", "select_statement", "expression_case":
		isNestingNode = true
	}

	// If this is a nesting node, increase depth for children
	newDepth := currentDepth
	if isNestingNode {
		newDepth = currentDepth + 1
		if newDepth > maxDepth {
			maxDepth = newDepth
		}
	}

	// Recursively check all children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		childMaxDepth := cle.findMaxNestingDepth(child, newDepth)
		if childMaxDepth > maxDepth {
			maxDepth = childMaxDepth
		}
	}

	return maxDepth
}

// Helper functions for impact analysis

func (cle *ContextLookupEngine) adjustCallFrequency(objectID CodeObjectID, baseCount int64) int64 {
	// Apply heuristics based on object characteristics
	name := strings.ToLower(objectID.Name)

	// Entry points and handlers are called more frequently
	if strings.Contains(name, "main") || strings.Contains(name, "handler") || strings.Contains(name, "serve") {
		return baseCount * 10
	}

	// Utilities and helpers are called moderately
	if strings.Contains(name, "util") || strings.Contains(name, "helper") {
		return baseCount * 3
	}

	// Test functions are called less frequently
	if strings.Contains(name, "test") || strings.Contains(name, "spec") {
		return baseCount / 2
	}

	return baseCount
}

func (cle *ContextLookupEngine) isFunctionCall(fileID types.FileID, line int, functionName string) bool {
	// Check if a reference at the given line is actually a function call
	// vs other uses like assignment, type declaration, etc.

	// Check RefType in references from indexed data - RefTypeCall indicates function call
	fileSymbols := cle.refTracker.GetFileEnhancedSymbols(fileID)
	for _, sym := range fileSymbols {
		for _, ref := range sym.OutgoingRefs {
			if ref.Line == line && ref.ReferencedName == functionName {
				// Use indexed RefType to determine if this is a call reference
				return ref.Type == types.RefTypeCall
			}
		}
	}
	return false // Conservative: assume not a call if uncertain
}

// isCallAtLine checks if the function name at the given line is used as a call
func (cle *ContextLookupEngine) isCallAtLine(node *sitter.Node, content []byte, targetLine int, functionName string) bool {
	if node == nil {
		return false
	}

	// Check if this node is on the target line
	nodeStartLine := int(node.StartPosition().Row)
	nodeEndLine := int(node.EndPosition().Row)

	if nodeStartLine <= targetLine && targetLine <= nodeEndLine {
		// Check if this is a call_expression containing our function name
		if node.Kind() == "call_expression" {
			// Check if the function being called matches our name
			if cle.callExpressionMatchesName(node, content, functionName) {
				return true
			}
		}

		// Recursively check children
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if cle.isCallAtLine(child, content, targetLine, functionName) {
				return true
			}
		}
	}

	return false
}

// callExpressionMatchesName checks if a call_expression calls the specified function
func (cle *ContextLookupEngine) callExpressionMatchesName(callNode *sitter.Node, content []byte, functionName string) bool {
	// The first child of call_expression is the function being called
	// It could be an identifier, selector_expression, etc.
	if callNode.ChildCount() == 0 {
		return false
	}

	funcNode := callNode.Child(0)
	if funcNode == nil {
		return false
	}

	// Check different patterns
	switch funcNode.Kind() {
	case "identifier":
		// Direct function call: functionName()
		name := string(content[funcNode.StartByte():funcNode.EndByte()])
		return name == functionName

	case "selector_expression":
		// Method call: obj.functionName()
		// The selector (method name) is typically the last identifier
		for i := uint(0); i < funcNode.ChildCount(); i++ {
			child := funcNode.Child(i)
			if child != nil && child.Kind() == "field_identifier" {
				name := string(content[child.StartByte():child.EndByte()])
				if name == functionName {
					return true
				}
			}
		}
	}

	return false
}

func (cle *ContextLookupEngine) assessBreakingChangeRisk(objectID CodeObjectID, fanIn, fanOut int) string {
	// High risk if many callers or complex interface
	if fanIn > 10 || fanOut > 10 {
		return "high"
	}

	// Medium risk if moderate usage
	if fanIn > 3 || fanOut > 3 {
		return "medium"
	}

	// Low risk for simple, rarely used functions
	return "low"
}

func (cle *ContextLookupEngine) findDependentComponents(objectID CodeObjectID) []string {
	// Find all components that depend on this object
	dependents := []string{}

	// Get all callers and their containing modules
	callers, err := cle.getCallerFunctions(objectID)
	if err != nil {
		return dependents
	}

	for _, caller := range callers {
		module := cle.extractModuleFromObjectID(caller.ObjectID)
		if !contains(dependents, module) {
			dependents = append(dependents, module)
		}
	}

	return dependents
}

func (cle *ContextLookupEngine) calculateImpactScore(objectID CodeObjectID, fanIn, fanOut int) int {
	// Calculate impact score on 1-10 scale
	score := 1

	// Base score from usage
	score += int(math.Min(float64(fanIn)/2.0, 5.0))
	score += int(math.Min(float64(fanOut)/2.0, 3.0))

	// Adjust for complexity
	complexity, _ := cle.calculateComplexityMetrics(objectID)
	if complexity.CyclomaticComplexity > 10 {
		score += 2
	}

	// Adjust for public API
	if cle.isPublicAPI(objectID) {
		score += 2
	}

	// Cap at 10
	if score > 10 {
		score = 10
	}

	return score
}

func (cle *ContextLookupEngine) requiresTestsForChange(objectID CodeObjectID) bool {
	// Changes to most objects should have tests
	return true
}

func (cle *ContextLookupEngine) findTestFiles(objectID CodeObjectID) []string {
	var testFiles []string

	// Look for test files with similar names
	baseName := strings.TrimSuffix(objectID.Name, "_test")
	testPatterns := []string{
		baseName + "_test.go",
		baseName + "_test.js",
		baseName + ".test.ts",
		baseName + "_spec.py",
		"test_" + baseName + ".go",
	}

	// This would search the file system for matching test files
	// For now, return empty slice. testPatterns would be used in real implementation.
	_ = testPatterns
	return testFiles
}

func (cle *ContextLookupEngine) getObjectEndLine(objectID CodeObjectID) int {
	// Find the end line of the object in the file

	// Only works for functions and methods
	if objectID.Type != types.SymbolTypeFunction && objectID.Type != types.SymbolTypeMethod {
		return 0
	}

	// Use EnhancedSymbol.EndLine from indexed data
	symbols := cle.refTracker.FindSymbolsByName(objectID.Name)
	for _, sym := range symbols {
		if sym.FileID == objectID.FileID && sym.Type == objectID.Type {
			// Return EndLine directly from indexed symbol data
			return sym.EndLine
		}
	}
	return 0
}

func (cle *ContextLookupEngine) calculateMatchNestingDepth(match QueryMatch) int {
	// Calculate the nesting depth of a match
	// This would analyze the AST hierarchy
	return 1 // Placeholder
}

func (cle *ContextLookupEngine) isPublicAPI(objectID CodeObjectID) bool {
	// Check if the object is part of the public API
	// In Go, exported (public) names start with an uppercase letter
	// Unexported (private) names start with lowercase letter or underscore

	name := objectID.Name
	if name == "" {
		return false
	}

	// Names starting with underscore are private by convention
	if strings.HasPrefix(name, "_") {
		return false
	}

	// Check if first character is uppercase (exported/public)
	firstChar := rune(name[0])
	return unicode.IsUpper(firstChar)
}

func (cle *ContextLookupEngine) extractModuleFromObjectID(objectID CodeObjectID) string {
	// Extract module name from object ID
	filePath := cle.fileService.GetPathForFileID(objectID.FileID)
	if filePath != "" {
		return extractModuleFromPath(filePath)
	}
	return "unknown"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
