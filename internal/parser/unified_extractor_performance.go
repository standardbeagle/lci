package parser

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ============================================================================
// Performance Anti-Pattern Detection
// ============================================================================

// isLoopNode returns true if the node type represents a loop construct
func (ue *UnifiedExtractor) isLoopNode(nodeType string) bool {
	switch nodeType {
	// Go
	case "for_statement", "for_range_statement":
		return true
	// JavaScript/TypeScript
	case "for_in_statement", "for_of_statement":
		return true
	// Python, common
	case "while_statement", "do_while_statement", "do_statement":
		return true
	// Rust
	case "loop_expression", "while_expression", "for_expression":
		return true
	// Java/C#
	case "for_each_statement", "enhanced_for_statement", "foreach_statement":
		return true
	}
	return false
}

// isAwaitNode returns true if the node type represents an await expression
// Note: We only match the expression nodes, not the keyword nodes (which are children)
func (ue *UnifiedExtractor) isAwaitNode(nodeType string) bool {
	switch nodeType {
	case "await_expression": // JS/TS, C#, Rust - the expression containing await
		return true
	}
	return false
}

// isAsyncFunctionNode returns true if the node type is an async function
func (ue *UnifiedExtractor) isAsyncFunctionNode(node *tree_sitter.Node, nodeType string) bool {
	// Check for async keyword in function declarations
	switch nodeType {
	case "function_declaration", "function_definition", "arrow_function",
		"function_expression", "method_definition":
		// Check if any child is "async" keyword
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil {
				childType := ue.getNodeType(child)
				if childType == "async" {
					return true
				}
			}
		}
	// Explicit async node types
	case "async_function_declaration", "async_function_expression",
		"async_arrow_function", "async_method_definition":
		return true
	}
	return false
}

// processPerformanceTracking handles performance anti-pattern detection during traversal
func (ue *UnifiedExtractor) processPerformanceTracking(node *tree_sitter.Node, nodeType string) {
	// Track function entry/exit for performance analysis
	if ue.isFunctionNode(nodeType) {
		ue.startFunctionAnalysis(node, nodeType)
	}

	// Track await expressions
	if ue.isAwaitNode(nodeType) {
		ue.trackAwaitExpression(node, nodeType)
	}

	// Track function calls (for expensive-call-in-loop detection)
	if ue.isCallExpression(nodeType) {
		ue.trackCallExpression(node, nodeType)
	}

	// Track loop information for current function
	if ue.isLoopNode(nodeType) && ue.currentFuncAnalysis != nil {
		startLine := int(node.StartPosition().Row) + 1
		endLine := int(node.EndPosition().Row) + 1
		ue.currentFuncAnalysis.loops = append(ue.currentFuncAnalysis.loops, loopStackEntry{
			nodeType:  nodeType,
			startLine: startLine,
			endLine:   endLine,
			depth:     len(ue.loopStack) + 1, // 1-indexed depth (before push happens later)
		})
	}
}

// isCallExpression returns true if the node type represents a function call
func (ue *UnifiedExtractor) isCallExpression(nodeType string) bool {
	switch nodeType {
	case "call_expression", // Go, JS/TS, C#
		"call",                  // Python
		"invocation_expression", // C#
		"method_invocation":     // Java
		return true
	}
	return false
}

// startFunctionAnalysis initializes tracking for a new function
func (ue *UnifiedExtractor) startFunctionAnalysis(node *tree_sitter.Node, nodeType string) {
	// Save any previous function analysis
	if ue.currentFuncAnalysis != nil {
		ue.finalizeFunctionAnalysis()
	}

	// Extract function name
	funcName := ue.extractFunctionName(node, nodeType)
	startLine := int(node.StartPosition().Row) + 1
	endLine := int(node.EndPosition().Row) + 1

	ue.currentFuncAnalysis = &functionAnalysisState{
		name:      funcName,
		startLine: startLine,
		endLine:   endLine,
		isAsync:   ue.isAsyncFunctionNode(node, nodeType),
		loops:     make([]loopStackEntry, 0, 4),
		awaits:    make([]awaitExprInfo, 0, 4),
		calls:     make([]callInFuncInfo, 0, 8),
	}
}

// extractFunctionName extracts the name from a function node
func (ue *UnifiedExtractor) extractFunctionName(node *tree_sitter.Node, nodeType string) string {
	// Try common field names for function names
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		return string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}
	if nameNode := node.ChildByFieldName("declarator"); nameNode != nil {
		// C/C++ style
		if idNode := nameNode.ChildByFieldName("declarator"); idNode != nil {
			return string(ue.content[idNode.StartByte():idNode.EndByte()])
		}
		return string(ue.content[nameNode.StartByte():nameNode.EndByte()])
	}

	// For arrow functions assigned to variables, check parent
	if nodeType == "arrow_function" || nodeType == "function_expression" {
		if parent := node.Parent(); parent != nil {
			parentType := ue.getNodeType(parent)
			if parentType == "variable_declarator" {
				if nameNode := parent.ChildByFieldName("name"); nameNode != nil {
					return string(ue.content[nameNode.StartByte():nameNode.EndByte()])
				}
			}
		}
	}

	// Anonymous function
	return "<anonymous>"
}

// trackAwaitExpression records an await expression for analysis
func (ue *UnifiedExtractor) trackAwaitExpression(node *tree_sitter.Node, nodeType string) {
	if ue.currentFuncAnalysis == nil {
		return
	}

	line := int(node.StartPosition().Row) + 1

	// Try to extract the call target (what is being awaited)
	callTarget := ue.extractAwaitTarget(node)

	// Try to find the assigned variable (if this await is in an assignment)
	assignedVar := ue.findAssignedVariable(node)

	// Extract variables used in the await arguments
	usedVars := ue.extractUsedVariables(node)

	ue.currentFuncAnalysis.awaits = append(ue.currentFuncAnalysis.awaits, awaitExprInfo{
		line:        line,
		assignedVar: assignedVar,
		callTarget:  callTarget,
		usedVars:    usedVars,
	})
}

// extractAwaitTarget extracts the function/method being awaited
func (ue *UnifiedExtractor) extractAwaitTarget(node *tree_sitter.Node) string {
	// The awaited expression is usually the first child or a field
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := ue.getNodeType(child)

		// Look for call expression inside await
		if childType == "call_expression" || childType == "call" {
			if funcNode := child.ChildByFieldName("function"); funcNode != nil {
				return string(ue.content[funcNode.StartByte():funcNode.EndByte()])
			}
			// Try first child as function name
			if child.ChildCount() > 0 {
				firstChild := child.Child(0)
				if firstChild != nil {
					return string(ue.content[firstChild.StartByte():firstChild.EndByte()])
				}
			}
		}

		// Member expression (method call)
		if childType == "member_expression" || childType == "attribute" {
			return string(ue.content[child.StartByte():child.EndByte()])
		}

		// Direct identifier
		if childType == "identifier" {
			return string(ue.content[child.StartByte():child.EndByte()])
		}
	}
	return "<unknown>"
}

// findAssignedVariable finds the variable an await result is assigned to
func (ue *UnifiedExtractor) findAssignedVariable(node *tree_sitter.Node) string {
	// Walk up to find assignment or variable declaration
	parent := node.Parent()
	for parent != nil {
		parentType := ue.getNodeType(parent)

		// Variable declaration: const x = await foo()
		if parentType == "variable_declarator" || parentType == "lexical_declaration" {
			if nameNode := parent.ChildByFieldName("name"); nameNode != nil {
				return string(ue.content[nameNode.StartByte():nameNode.EndByte()])
			}
			// Check first child for identifier
			if parent.ChildCount() > 0 {
				firstChild := parent.Child(0)
				if firstChild != nil && ue.getNodeType(firstChild) == "identifier" {
					return string(ue.content[firstChild.StartByte():firstChild.EndByte()])
				}
			}
		}

		// Assignment expression: x = await foo()
		if parentType == "assignment_expression" || parentType == "assignment" {
			if leftNode := parent.ChildByFieldName("left"); leftNode != nil {
				if ue.getNodeType(leftNode) == "identifier" {
					return string(ue.content[leftNode.StartByte():leftNode.EndByte()])
				}
			}
		}

		// Short variable declaration in Go: x := <-ch (not await, but similar pattern)
		if parentType == "short_var_declaration" {
			if leftNode := parent.ChildByFieldName("left"); leftNode != nil {
				return string(ue.content[leftNode.StartByte():leftNode.EndByte()])
			}
		}

		// Don't go too far up
		if parentType == "function_declaration" || parentType == "method_definition" ||
			parentType == "program" || parentType == "block" {
			break
		}

		parent = parent.Parent()
	}
	return ""
}

// extractUsedVariables extracts variable identifiers used in node's descendants
func (ue *UnifiedExtractor) extractUsedVariables(node *tree_sitter.Node) []string {
	var vars []string
	seen := make(map[string]bool)

	var walk func(n *tree_sitter.Node)
	walk = func(n *tree_sitter.Node) {
		if n == nil {
			return
		}
		nodeType := ue.getNodeType(n)

		// Collect identifiers (excluding function names and keywords)
		if nodeType == "identifier" {
			name := string(ue.content[n.StartByte():n.EndByte()])
			// Skip common non-variable identifiers
			if !seen[name] && !isCommonKeyword(name) {
				seen[name] = true
				vars = append(vars, name)
			}
		}

		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}

	// Walk the arguments of the await expression
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			childType := ue.getNodeType(child)
			// Look for arguments in call expressions
			if childType == "call_expression" || childType == "call" {
				if argsNode := child.ChildByFieldName("arguments"); argsNode != nil {
					walk(argsNode)
				}
			}
		}
	}

	return vars
}

// isCommonKeyword returns true if the name is a common keyword to skip
func isCommonKeyword(name string) bool {
	switch name {
	case "await", "async", "return", "const", "let", "var", "function",
		"true", "false", "null", "undefined", "this", "self", "None", "True", "False":
		return true
	}
	return false
}

// trackCallExpression records a function call for analysis
func (ue *UnifiedExtractor) trackCallExpression(node *tree_sitter.Node, nodeType string) {
	if ue.currentFuncAnalysis == nil {
		return
	}

	line := int(node.StartPosition().Row) + 1

	// Extract call target
	target := ue.extractCallTarget(node)

	// Determine if we're inside a loop
	inLoop := len(ue.loopStack) > 0
	loopDepth := len(ue.loopStack)
	loopLine := 0
	if inLoop {
		loopLine = ue.loopStack[0].startLine // Outermost loop
	}

	ue.currentFuncAnalysis.calls = append(ue.currentFuncAnalysis.calls, callInFuncInfo{
		target:    target,
		line:      line,
		inLoop:    inLoop,
		loopDepth: loopDepth,
		loopLine:  loopLine,
	})
}

// extractCallTarget extracts the function/method name from a call expression
func (ue *UnifiedExtractor) extractCallTarget(node *tree_sitter.Node) string {
	// Try "function" field (JS/TS, Go)
	if funcNode := node.ChildByFieldName("function"); funcNode != nil {
		return string(ue.content[funcNode.StartByte():funcNode.EndByte()])
	}

	// Try first child (Python, others)
	if node.ChildCount() > 0 {
		firstChild := node.Child(0)
		if firstChild != nil {
			return string(ue.content[firstChild.StartByte():firstChild.EndByte()])
		}
	}

	return "<unknown>"
}

// finalizeFunctionAnalysis saves the current function's analysis
func (ue *UnifiedExtractor) finalizeFunctionAnalysis() {
	if ue.currentFuncAnalysis == nil {
		return
	}

	// Convert internal types to exported types
	loops := make([]LoopInfo, len(ue.currentFuncAnalysis.loops))
	for i, l := range ue.currentFuncAnalysis.loops {
		loops[i] = LoopInfo{
			NodeType:  l.nodeType,
			StartLine: l.startLine,
			EndLine:   l.endLine,
			Depth:     l.depth,
		}
	}

	awaits := make([]AwaitInfo, len(ue.currentFuncAnalysis.awaits))
	for i, a := range ue.currentFuncAnalysis.awaits {
		awaits[i] = AwaitInfo{
			Line:        a.line,
			AssignedVar: a.assignedVar,
			CallTarget:  a.callTarget,
			UsedVars:    a.usedVars,
		}
	}

	calls := make([]CallInfo, len(ue.currentFuncAnalysis.calls))
	for i, c := range ue.currentFuncAnalysis.calls {
		calls[i] = CallInfo{
			Target:    c.target,
			Line:      c.line,
			InLoop:    c.inLoop,
			LoopDepth: c.loopDepth,
			LoopLine:  c.loopLine,
		}
	}

	// Determine language from extension
	language := getLanguageFromExt(ue.ext)

	result := PerfAnalysisResult{
		FunctionName: ue.currentFuncAnalysis.name,
		StartLine:    ue.currentFuncAnalysis.startLine,
		EndLine:      ue.currentFuncAnalysis.endLine,
		IsAsync:      ue.currentFuncAnalysis.isAsync,
		Language:     language,
		FilePath:     ue.path,
		Loops:        loops,
		Awaits:       awaits,
		Calls:        calls,
	}

	ue.perfAnalysisResults = append(ue.perfAnalysisResults, result)
	ue.currentFuncAnalysis = nil
}

// getLanguageFromExt returns language name from file extension
func getLanguageFromExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return "unknown"
	}
}

// GetPerfAnalysisResults returns the collected performance analysis data
func (ue *UnifiedExtractor) GetPerfAnalysisResults() []PerfAnalysisResult {
	// Finalize any in-progress function analysis
	if ue.currentFuncAnalysis != nil {
		ue.finalizeFunctionAnalysis()
	}
	return ue.perfAnalysisResults
}
