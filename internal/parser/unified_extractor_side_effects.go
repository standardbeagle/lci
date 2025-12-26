package parser

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// === SIDE EFFECT TRACKING HELPER METHODS ===

// extractFunctionParameters extracts and registers function parameters for side effect tracking
func (ue *UnifiedExtractor) extractFunctionParameters(node *tree_sitter.Node, nodeType string) {
	if ue.sideEffectTracker == nil {
		return
	}

	// Check for method receiver (Go)
	if nodeType == "method_declaration" || nodeType == "function_declaration" {
		if receiverNode := node.ChildByFieldName("receiver"); receiverNode != nil {
			// Go receiver: (r *ReceiverType)
			// Extract receiver name and type
			receiverName, receiverType := ue.extractGoReceiver(receiverNode)
			if receiverName != "" {
				ue.sideEffectTracker.SetReceiver(receiverName, receiverType)
			}
		}
	}

	// Extract parameters
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		paramsNode = node.ChildByFieldName("parameter_list")
	}
	if paramsNode == nil {
		paramsNode = node.ChildByFieldName("formal_parameters")
	}

	if paramsNode == nil {
		return
	}

	paramIndex := 0
	for i := uint(0); i < paramsNode.ChildCount(); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		childType := ue.getNodeType(child)

		switch childType {
		case "identifier":
			// Simple parameter: func(x)
			name := string(ue.content[child.StartByte():child.EndByte()])
			ue.sideEffectTracker.AddParameter(name, paramIndex)
			paramIndex++

		case "parameter_declaration", "required_parameter", "optional_parameter",
			"formal_parameter", "simple_parameter":
			// Named parameter with type
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := string(ue.content[nameNode.StartByte():nameNode.EndByte()])
				ue.sideEffectTracker.AddParameter(name, paramIndex)
				paramIndex++
			} else {
				// Try first identifier child
				for j := uint(0); j < child.ChildCount(); j++ {
					paramChild := child.Child(j)
					if paramChild != nil && ue.getNodeType(paramChild) == "identifier" {
						name := string(ue.content[paramChild.StartByte():paramChild.EndByte()])
						ue.sideEffectTracker.AddParameter(name, paramIndex)
						paramIndex++
						break
					}
				}
			}

		case "typed_parameter", "typed_default_parameter":
			// Python: def foo(x: int)
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := string(ue.content[nameNode.StartByte():nameNode.EndByte()])
				ue.sideEffectTracker.AddParameter(name, paramIndex)
				paramIndex++
			}

		case "default_parameter":
			// Python/JS: def foo(x=1) or function foo(x=1)
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := string(ue.content[nameNode.StartByte():nameNode.EndByte()])
				ue.sideEffectTracker.AddParameter(name, paramIndex)
				paramIndex++
			} else if child.ChildCount() > 0 {
				first := child.Child(0)
				if first != nil && ue.getNodeType(first) == "identifier" {
					name := string(ue.content[first.StartByte():first.EndByte()])
					ue.sideEffectTracker.AddParameter(name, paramIndex)
					paramIndex++
				}
			}

		case "rest_parameter", "spread_parameter", "variadic_parameter":
			// ...args or *args
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := string(ue.content[nameNode.StartByte():nameNode.EndByte()])
				ue.sideEffectTracker.AddParameter(name, paramIndex)
				paramIndex++
			}

		case "self_parameter", "self":
			// Python self, Rust self
			ue.sideEffectTracker.SetReceiver("self", "")

		case "this":
			// JavaScript this
			ue.sideEffectTracker.SetReceiver("this", "")
		}
	}
}

// extractGoReceiver extracts receiver name and type from a Go method receiver
func (ue *UnifiedExtractor) extractGoReceiver(receiverNode *tree_sitter.Node) (name, receiverType string) {
	// Receiver is typically: (r ReceiverType) or (r *ReceiverType)
	for i := uint(0); i < receiverNode.ChildCount(); i++ {
		child := receiverNode.Child(i)
		if child == nil {
			continue
		}

		childType := ue.getNodeType(child)

		switch childType {
		case "parameter_declaration":
			// Extract name and type from declaration
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name = string(ue.content[nameNode.StartByte():nameNode.EndByte()])
			}
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				receiverType = string(ue.content[typeNode.StartByte():typeNode.EndByte()])
			}
		case "identifier":
			if name == "" {
				name = string(ue.content[child.StartByte():child.EndByte()])
			} else if receiverType == "" {
				receiverType = string(ue.content[child.StartByte():child.EndByte()])
			}
		case "type_identifier", "pointer_type":
			receiverType = string(ue.content[child.StartByte():child.EndByte()])
		}
	}
	return
}

// processSideEffectNode processes a node for side effect tracking
func (ue *UnifiedExtractor) processSideEffectNode(node *tree_sitter.Node, nodeType string) {
	if ue.sideEffectTracker == nil || !ue.sideEffectTracker.IsInFunction() {
		return
	}

	switch nodeType {
	// === ASSIGNMENTS ===
	case "assignment_expression", "assignment_statement", "assignment",
		"augmented_assignment_expression", "augmented_assignment", "compound_assignment_expr",
		"update_expression",
		"inc_statement", "dec_statement": // Go's ++ and -- statements
		ue.sideEffectTracker.ProcessAssignment(node, ue.content, ue.getNodeType)

	case "short_var_declaration":
		// Go's := declares new variables
		ue.processGoShortVarDecl(node)

	// === FUNCTION CALLS ===
	case "call_expression", "function_call", "method_call", "invocation_expression":
		ue.sideEffectTracker.ProcessFunctionCall(node, ue.content, ue.getNodeType)

	// === THROWS/PANICS ===
	case "throw_statement":
		ue.sideEffectTracker.ProcessThrow(node, "throw")

	case "raise_statement":
		ue.sideEffectTracker.ProcessThrow(node, "raise")

	case "panic_call":
		// Go's panic() - detected as call_expression, but we can catch it here too
		ue.sideEffectTracker.ProcessThrow(node, "panic")

	// === DEFER ===
	case "defer_statement":
		ue.sideEffectTracker.ProcessDefer()

	// === TRY-FINALLY ===
	case "try_statement":
		// Check if it has a finally clause
		if finallyNode := node.ChildByFieldName("finalizer"); finallyNode != nil {
			ue.sideEffectTracker.ProcessTryFinally()
		}

	case "try_expression":
		// Rust try expression
		// May have finally-like cleanup

	// === CHANNEL OPERATIONS (Go) ===
	case "send_statement", "receive_expression":
		line := int(node.StartPosition().Row) + 1
		ue.sideEffectTracker.ProcessChannelOp(line)

	case "select_statement":
		// Go's select statement for channel operations
		// The select itself indicates channel usage
		line := int(node.StartPosition().Row) + 1
		ue.sideEffectTracker.ProcessChannelOp(line)

	case "unary_expression":
		// Check if this is a channel receive (<-ch) vs other unary ops (*p, !x, etc.)
		if node.ChildCount() >= 2 {
			firstChild := node.Child(0)
			if firstChild != nil {
				op := string(ue.content[firstChild.StartByte():firstChild.EndByte()])
				if op == "<-" {
					// This is a channel receive operation
					line := int(node.StartPosition().Row) + 1
					ue.sideEffectTracker.ProcessChannelOp(line)
				}
			}
		}

	// === VARIABLE READS (for tracking access patterns) ===
	case "identifier":
		// Only process if not part of a larger expression that we already handle
		parent := node.Parent()
		if parent != nil {
			parentType := ue.getNodeType(parent)
			// Skip if parent is assignment LHS or call function name
			if parentType == "assignment_expression" || parentType == "assignment_statement" ||
				parentType == "call_expression" || parentType == "member_expression" ||
				parentType == "short_var_declaration" {
				return
			}
		}
		ue.sideEffectTracker.ProcessRead(node, ue.content, ue.getNodeType)

	// === VARIABLE DECLARATIONS ===
	case "variable_declarator":
		// Register as local variable if has initializer
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name := string(ue.content[nameNode.StartByte():nameNode.EndByte()])
			line := int(nameNode.StartPosition().Row) + 1
			ue.sideEffectTracker.AddLocalVariable(name, line)
		}

	case "lexical_declaration", "const_declaration":
		// Let/const in JS, const in Go
		ue.processVariableDeclarations(node)
	}
}

// processGoShortVarDecl processes Go's := declaration
func (ue *UnifiedExtractor) processGoShortVarDecl(node *tree_sitter.Node) {
	// Left side is new variable(s)
	leftNode := node.ChildByFieldName("left")
	if leftNode == nil {
		return
	}

	leftType := ue.getNodeType(leftNode)
	line := int(leftNode.StartPosition().Row) + 1

	if leftType == "identifier" {
		name := string(ue.content[leftNode.StartByte():leftNode.EndByte()])
		ue.sideEffectTracker.AddLocalVariable(name, line)
	} else if leftType == "expression_list" {
		// Multiple declaration: a, b := ...
		for i := uint(0); i < leftNode.ChildCount(); i++ {
			child := leftNode.Child(i)
			if child != nil && ue.getNodeType(child) == "identifier" {
				name := string(ue.content[child.StartByte():child.EndByte()])
				ue.sideEffectTracker.AddLocalVariable(name, line)
			}
		}
	}
}

// processVariableDeclarations processes variable declarations to register local variables
func (ue *UnifiedExtractor) processVariableDeclarations(node *tree_sitter.Node) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := ue.getNodeType(child)
		if childType == "variable_declarator" {
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := string(ue.content[nameNode.StartByte():nameNode.EndByte()])
				line := int(nameNode.StartPosition().Row) + 1
				ue.sideEffectTracker.AddLocalVariable(name, line)
			}
		}
	}
}
