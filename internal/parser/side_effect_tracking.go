package parser

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/types"
)

// SideEffectTracker integrates side effect analysis into AST extraction.
// It hooks into the UnifiedExtractor to track accesses and function calls
// during the AST traversal, enabling accurate side effect detection.
type SideEffectTracker struct {
	analyzer *analysis.SideEffectAnalyzer
	language string

	// Current function context
	inFunction      bool
	currentFuncName string
	currentFuncFile string
	parameterNames  map[string]int // parameter name -> index
	receiverName    string
	localVars       map[string]int // local variable name -> declaration line

	// Results
	results map[string]*types.SideEffectInfo // keyed by "file:line"
}

// NewSideEffectTracker creates a new tracker for the given language
func NewSideEffectTracker(language string) *SideEffectTracker {
	return &SideEffectTracker{
		analyzer:       analysis.NewSideEffectAnalyzer(language, nil),
		language:       language,
		parameterNames: make(map[string]int),
		localVars:      make(map[string]int),
		results:        make(map[string]*types.SideEffectInfo),
	}
}

// BeginFunction starts tracking a new function
func (st *SideEffectTracker) BeginFunction(name, file string, startLine, endLine int) {
	// End any previous function first
	if st.inFunction {
		st.EndFunction()
	}

	st.inFunction = true
	st.currentFuncName = name
	st.currentFuncFile = file
	st.parameterNames = make(map[string]int)
	st.localVars = make(map[string]int)
	st.receiverName = ""

	st.analyzer.BeginFunction(name, file, startLine, endLine)
}

// EndFunction completes tracking of the current function
func (st *SideEffectTracker) EndFunction() *types.SideEffectInfo {
	if !st.inFunction {
		return nil
	}

	info := st.analyzer.EndFunction()
	if info != nil {
		key := st.currentFuncFile + ":" + itoa(info.ErrorHandling.DeferCount) // Use a unique key
		st.results[key] = info
	}

	st.inFunction = false
	st.currentFuncName = ""
	st.parameterNames = make(map[string]int)
	st.localVars = make(map[string]int)
	st.receiverName = ""

	return info
}

// AddParameter registers a function parameter
func (st *SideEffectTracker) AddParameter(name string, index int) {
	if st.inFunction {
		st.parameterNames[name] = index
		st.analyzer.AddParameter(name, index)
	}
}

// SetReceiver sets the method receiver
func (st *SideEffectTracker) SetReceiver(name, receiverType string) {
	if st.inFunction {
		st.receiverName = name
		st.analyzer.SetReceiver(name, receiverType)
	}
}

// AddLocalVariable registers a local variable
func (st *SideEffectTracker) AddLocalVariable(name string, line int) {
	if st.inFunction {
		st.localVars[name] = line
		st.analyzer.AddLocalVariable(name, line)
	}
}

// ProcessAssignment handles an assignment expression/statement
func (st *SideEffectTracker) ProcessAssignment(node *tree_sitter.Node, content []byte, getNodeType func(*tree_sitter.Node) string) {
	if !st.inFunction || node == nil {
		return
	}

	nodeType := getNodeType(node)

	var leftNode *tree_sitter.Node
	var line, column int

	switch nodeType {
	case "assignment_expression", "assignment_statement", "assignment":
		leftNode = node.ChildByFieldName("left")
		if leftNode == nil && node.ChildCount() > 0 {
			leftNode = node.Child(0)
		}

	case "augmented_assignment_expression", "augmented_assignment", "compound_assignment_expr":
		// +=, -=, etc. are both read and write
		leftNode = node.ChildByFieldName("left")
		if leftNode == nil && node.ChildCount() > 0 {
			leftNode = node.Child(0)
		}
		// Record the read first
		if leftNode != nil {
			identifier, fieldPath := st.extractTarget(leftNode, content, getNodeType)
			if identifier != "" {
				line = int(leftNode.StartPosition().Row) + 1
				column = int(leftNode.StartPosition().Column) + 1
				st.analyzer.RecordAccess(identifier, fieldPath, types.AccessRead, line, column)
			}
		}

	case "short_var_declaration":
		// Go's := creates new variables AND assigns
		// The left side is a new variable, not a write to existing
		leftNode = node.ChildByFieldName("left")
		if leftNode != nil {
			// Register as local variable, not as write
			varName := string(content[leftNode.StartByte():leftNode.EndByte()])
			line = int(leftNode.StartPosition().Row) + 1
			st.AddLocalVariable(varName, line)
		}
		return // Don't process as assignment

	case "update_expression":
		// i++, i--, ++i, --i (JavaScript, TypeScript, C-like languages)
		if node.ChildCount() > 0 {
			// Find the identifier (could be first or second child)
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child != nil && getNodeType(child) == "identifier" {
					leftNode = child
					break
				}
			}
		}

	case "inc_statement", "dec_statement":
		// Go's i++ and i-- statements
		// These have the identifier as the first child
		if node.ChildCount() > 0 {
			leftNode = node.Child(0)
		}
	}

	if leftNode == nil {
		return
	}

	line = int(leftNode.StartPosition().Row) + 1
	column = int(leftNode.StartPosition().Column) + 1

	identifier, fieldPath := st.extractTarget(leftNode, content, getNodeType)
	if identifier != "" {
		st.analyzer.RecordAccess(identifier, fieldPath, types.AccessWrite, line, column)
	}
}

// ProcessRead handles a variable/field read
func (st *SideEffectTracker) ProcessRead(node *tree_sitter.Node, content []byte, getNodeType func(*tree_sitter.Node) string) {
	if !st.inFunction || node == nil {
		return
	}

	line := int(node.StartPosition().Row) + 1
	column := int(node.StartPosition().Column) + 1

	identifier, fieldPath := st.extractTarget(node, content, getNodeType)
	if identifier != "" {
		st.analyzer.RecordAccess(identifier, fieldPath, types.AccessRead, line, column)
	}
}

// ProcessFunctionCall handles a function call
func (st *SideEffectTracker) ProcessFunctionCall(node *tree_sitter.Node, content []byte, getNodeType func(*tree_sitter.Node) string) {
	if !st.inFunction || node == nil {
		return
	}

	line := int(node.StartPosition().Row) + 1
	column := int(node.StartPosition().Column) + 1

	funcName, qualifier, isMethod, isDynamic := st.extractCallInfo(node, content, getNodeType)

	if isDynamic {
		st.analyzer.RecordDynamicCall(funcName, line, column)
	} else if funcName != "" {
		st.analyzer.RecordFunctionCall(funcName, qualifier, isMethod, line, column)
	}
}

// ProcessThrow handles a throw/panic/raise statement
func (st *SideEffectTracker) ProcessThrow(node *tree_sitter.Node, throwType string) {
	if !st.inFunction || node == nil {
		return
	}

	line := int(node.StartPosition().Row) + 1
	column := int(node.StartPosition().Column) + 1

	st.analyzer.RecordThrow(throwType, line, column)
}

// ProcessDefer handles a defer statement (Go)
func (st *SideEffectTracker) ProcessDefer() {
	if st.inFunction {
		st.analyzer.RecordDefer()
	}
}

// ProcessTryFinally handles a try-finally block
func (st *SideEffectTracker) ProcessTryFinally() {
	if st.inFunction {
		st.analyzer.RecordTryFinally()
	}
}

// ProcessChannelOp handles a channel send/receive (Go)
func (st *SideEffectTracker) ProcessChannelOp(line int) {
	if st.inFunction {
		st.analyzer.RecordChannelOp(line)
	}
}

// extractTarget extracts the identifier and field path from an expression
func (st *SideEffectTracker) extractTarget(node *tree_sitter.Node, content []byte, getNodeType func(*tree_sitter.Node) string) (identifier string, fieldPath []string) {
	if node == nil {
		return "", nil
	}

	nodeType := getNodeType(node)

	switch nodeType {
	case "identifier", "shorthand_property_identifier":
		return string(content[node.StartByte():node.EndByte()]), nil

	case "member_expression", "field_expression", "attribute":
		// obj.field or obj.field.subfield
		return st.extractMemberExpression(node, content, getNodeType)

	case "subscript_expression", "index_expression":
		// arr[i] - treat as field access with "[index]" path
		if objectNode := node.ChildByFieldName("object"); objectNode != nil {
			id, path := st.extractTarget(objectNode, content, getNodeType)
			return id, append(path, "[index]")
		}
		if node.ChildCount() > 0 {
			id, path := st.extractTarget(node.Child(0), content, getNodeType)
			return id, append(path, "[index]")
		}

	case "this", "self":
		return nodeType, nil

	case "expression_list":
		// Go multiple assignment - just take first for now
		if node.ChildCount() > 0 {
			return st.extractTarget(node.Child(0), content, getNodeType)
		}

	case "pointer_expression", "unary_expression", "parenthesized_expression":
		// *ptr or (expr) - unwrap
		// Note: Go uses "unary_expression" for pointer dereferences (*p)
		if node.ChildCount() > 0 {
			return st.extractTarget(node.Child(node.ChildCount()-1), content, getNodeType)
		}

	case "selector_expression":
		// Go's obj.Field
		return st.extractMemberExpression(node, content, getNodeType)
	}

	return "", nil
}

// extractMemberExpression extracts the base identifier and field path from a member expression
func (st *SideEffectTracker) extractMemberExpression(node *tree_sitter.Node, content []byte, getNodeType func(*tree_sitter.Node) string) (identifier string, fieldPath []string) {
	// Try standard field names first
	objectNode := node.ChildByFieldName("object")
	propertyNode := node.ChildByFieldName("property")

	// Fallback for languages with different field names
	if objectNode == nil {
		objectNode = node.ChildByFieldName("operand")
	}
	if propertyNode == nil {
		propertyNode = node.ChildByFieldName("field")
	}
	if propertyNode == nil {
		propertyNode = node.ChildByFieldName("attribute")
	}

	// Last resort: positional children
	if objectNode == nil && node.ChildCount() > 0 {
		objectNode = node.Child(0)
	}
	if propertyNode == nil && node.ChildCount() > 1 {
		// Skip operator (.) and get field
		for i := uint(1); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil {
				childType := getNodeType(child)
				if childType == "identifier" || childType == "property_identifier" || childType == "field_identifier" {
					propertyNode = child
					break
				}
			}
		}
	}

	if objectNode == nil {
		return "", nil
	}

	// Recursively extract from object
	baseID, basePath := st.extractTarget(objectNode, content, getNodeType)

	// Add this field to path
	if propertyNode != nil {
		fieldName := string(content[propertyNode.StartByte():propertyNode.EndByte()])
		return baseID, append(basePath, fieldName)
	}

	return baseID, basePath
}

// extractCallInfo extracts function call information
func (st *SideEffectTracker) extractCallInfo(node *tree_sitter.Node, content []byte, getNodeType func(*tree_sitter.Node) string) (funcName, qualifier string, isMethod, isDynamic bool) {
	// Try "function" field first (Go, JS/TS)
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil {
		funcNode = node.ChildByFieldName("name")
	}
	if funcNode == nil && node.ChildCount() > 0 {
		funcNode = node.Child(0)
	}

	if funcNode == nil {
		return "", "", false, true // Can't determine - dynamic
	}

	funcNodeType := getNodeType(funcNode)

	switch funcNodeType {
	case "identifier":
		// Simple function call: foo()
		return string(content[funcNode.StartByte():funcNode.EndByte()]), "", false, false

	case "member_expression", "selector_expression", "field_expression", "attribute":
		// Method call: obj.method() or pkg.Function()
		objectNode := funcNode.ChildByFieldName("object")
		if objectNode == nil {
			objectNode = funcNode.ChildByFieldName("operand")
		}
		propertyNode := funcNode.ChildByFieldName("property")
		if propertyNode == nil {
			propertyNode = funcNode.ChildByFieldName("field")
		}

		if objectNode == nil && funcNode.ChildCount() > 0 {
			objectNode = funcNode.Child(0)
		}
		if propertyNode == nil && funcNode.ChildCount() > 1 {
			for i := uint(1); i < funcNode.ChildCount(); i++ {
				child := funcNode.Child(i)
				if child != nil {
					childType := getNodeType(child)
					if childType == "identifier" || childType == "property_identifier" || childType == "field_identifier" {
						propertyNode = child
						break
					}
				}
			}
		}

		if propertyNode != nil {
			methodName := string(content[propertyNode.StartByte():propertyNode.EndByte()])

			if objectNode != nil {
				objectType := getNodeType(objectNode)
				objectText := string(content[objectNode.StartByte():objectNode.EndByte()])

				// Check if it's a package/module qualifier or object method
				if objectType == "identifier" {
					// Could be pkg.Func or obj.method
					// If the first letter is uppercase (Go) or it's a known package, treat as qualifier
					if len(objectText) > 0 && objectText[0] >= 'A' && objectText[0] <= 'Z' {
						// Go package (e.g., strings.ToLower)
						return methodName, objectText, false, false
					}
					// Check if it's a parameter/local/receiver
					if _, isParam := st.parameterNames[objectText]; isParam {
						return methodName, objectText, true, false
					}
					if objectText == st.receiverName || objectText == "this" || objectText == "self" {
						return methodName, objectText, true, false
					}
					if _, isLocal := st.localVars[objectText]; isLocal {
						return methodName, objectText, true, false
					}
					// Unknown - could be global object, treat as method on unknown
					return methodName, objectText, true, false
				}

				// Complex expression as receiver - dynamic
				return methodName, "", true, true
			}

			return methodName, "", false, false
		}

		// Can't parse - dynamic
		return "", "", false, true

	case "parenthesized_expression", "call_expression":
		// (getFunc())() or func()() - definitely dynamic
		return "dynamic_call", "", false, true

	default:
		// Unknown pattern - assume dynamic
		return funcNodeType, "", false, true
	}
}

// GetResults returns all analysis results
func (st *SideEffectTracker) GetResults() map[string]*types.SideEffectInfo {
	return st.results
}

// GetAnalyzer returns the underlying analyzer (for direct access if needed)
func (st *SideEffectTracker) GetAnalyzer() *analysis.SideEffectAnalyzer {
	return st.analyzer
}

// IsInFunction returns whether we're currently tracking a function
func (st *SideEffectTracker) IsInFunction() bool {
	return st.inFunction
}

// Helper function - same as in analyzer to avoid import cycle
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	neg := i < 0
	if neg {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}

// LanguageFromExtension maps file extension to language name
func LanguageFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py", ".pyw":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".cpp", ".cc", ".cxx", ".c++", ".hpp", ".hxx", ".h":
		return "cpp"
	case ".c":
		return "c"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".zig":
		return "zig"
	default:
		return "unknown"
	}
}
