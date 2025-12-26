package display

import (
	"fmt"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// TreeFormatter formats function trees for display
type TreeFormatter struct {
	options FormatterOptions
}

// FormatterOptions controls tree formatting
type FormatterOptions struct {
	Format      string // "text", "json", "compact"
	ShowLines   bool   // Show line numbers
	ShowMetrics bool   // Show complexity metrics
	AgentMode   bool   // Show agent-specific annotations
	MaxDepth    int    // Maximum depth to display
	Indent      string // Indentation string
}

// NewTreeFormatter creates a new tree formatter
func NewTreeFormatter(options FormatterOptions) *TreeFormatter {
	if options.Indent == "" {
		options.Indent = "  "
	}
	return &TreeFormatter{options: options}
}

// Format formats a function tree for display
func (tf *TreeFormatter) Format(tree *types.FunctionTree) string {
	if tree == nil || tree.Root == nil {
		return "No tree data available"
	}

	switch tf.options.Format {
	case "json":
		return tf.formatJSON(tree)
	case "compact":
		return tf.formatCompact(tree)
	default:
		return tf.formatText(tree)
	}
}

// formatText formats tree as ASCII art
func (tf *TreeFormatter) formatText(tree *types.FunctionTree) string {
	var sb strings.Builder

	// Header
	if tf.options.AgentMode {
		sb.WriteString(fmt.Sprintf("🔍 Function Tree Analysis: %s\n", tree.RootFunction))
		sb.WriteString(fmt.Sprintf("📊 Total Nodes: %d | Max Depth: %d\n", tree.TotalNodes, tree.MaxDepth))
		sb.WriteString("⚠️  Risk Levels: 🟢 Low (0-3) | 🟡 Medium (4-6) | 🔴 High (7-10)\n")
		sb.WriteString("─────────────────────────────────────────────────────────────\n")
	} else {
		sb.WriteString(fmt.Sprintf("Function tree for '%s'\n", tree.RootFunction))
		sb.WriteString(fmt.Sprintf("Total nodes: %d, Max depth: %d\n", tree.TotalNodes, tree.MaxDepth))
		sb.WriteString("\n")
	}

	// Tree content
	tf.formatNode(&sb, tree.Root, "", true, true)

	return sb.String()
}

// formatNode recursively formats a tree node
func (tf *TreeFormatter) formatNode(sb *strings.Builder, node *types.TreeNode, prefix string, isLast bool, isRoot bool) {
	if node == nil {
		return
	}

	// Skip if beyond max depth
	if tf.options.MaxDepth > 0 && node.Depth > tf.options.MaxDepth {
		return
	}

	// Tree branch characters
	var branch string
	if isRoot {
		branch = "→ "
	} else if isLast {
		branch = "└─→ "
	} else {
		branch = "├─→ "
	}

	// Node name with optional decorations
	nodeName := node.Name
	if tf.options.AgentMode {
		nodeName = tf.decorateNodeName(node)
	}

	// Write the node line
	sb.WriteString(prefix)
	sb.WriteString(branch)
	sb.WriteString(nodeName)

	// Add location info if requested
	if tf.options.ShowLines && node.Line > 0 {
		sb.WriteString(fmt.Sprintf(" [%s:%d]", node.FilePath, node.Line))
	}

	// Add depth info
	sb.WriteString(fmt.Sprintf(" (depth=%d)", node.Depth))

	// Add agent mode annotations
	if tf.options.AgentMode {
		sb.WriteString(tf.getAgentAnnotations(node))
	}

	sb.WriteString("\n")

	// Process children
	childCount := len(node.Children)
	for i, child := range node.Children {
		isLastChild := i == childCount-1

		// Calculate child prefix
		var childPrefix string
		if isRoot {
			childPrefix = prefix + "  "
		} else if isLast {
			childPrefix = prefix + "  "
		} else {
			childPrefix = prefix + "│ "
		}

		tf.formatNode(sb, child, childPrefix, isLastChild, false)
	}
}

// decorateNodeName adds visual indicators based on node properties
func (tf *TreeFormatter) decorateNodeName(node *types.TreeNode) string {
	name := node.Name

	// Add risk indicator
	if node.EditRiskScore >= 7 {
		name = "🔴 " + name
	} else if node.EditRiskScore >= 4 {
		name = "🟡 " + name
	} else {
		name = "🟢 " + name
	}

	// Add node type indicator
	switch node.NodeType {
	case types.NodeTypeExternal:
		name += " 🔗"
	case types.NodeTypeRecursive:
		name += " 🔄"
	}

	return name
}

// getAgentAnnotations returns agent-specific annotations for a node
func (tf *TreeFormatter) getAgentAnnotations(node *types.TreeNode) string {
	var annotations []string

	// Add stability tags
	for _, tag := range node.StabilityTags {
		switch tag {
		case "CORE":
			annotations = append(annotations, "⚡CORE")
		case "PUBLIC_API":
			annotations = append(annotations, "🌐PUBLIC")
		case "DEPRECATED":
			annotations = append(annotations, "⚠️DEPRECATED")
		}
	}

	// Add dependency metrics
	if node.DependentCount > 0 {
		annotations = append(annotations, fmt.Sprintf("👥%d deps", node.DependentCount))
	}

	// Add risk warning
	if node.EditRiskScore >= 7 {
		annotations = append(annotations, "⚠️HIGH-RISK")
	}

	if len(annotations) > 0 {
		return " [" + strings.Join(annotations, " ") + "]"
	}
	return ""
}

// formatCompact formats tree in a compact single-line format
func (tf *TreeFormatter) formatCompact(tree *types.FunctionTree) string {
	if tree == nil || tree.Root == nil {
		return ""
	}

	var parts []string
	tf.collectCompactParts(tree.Root, &parts)
	return strings.Join(parts, " → ")
}

// collectCompactParts collects node names for compact format
func (tf *TreeFormatter) collectCompactParts(node *types.TreeNode, parts *[]string) {
	if node == nil {
		return
	}

	*parts = append(*parts, node.Name)

	// Only follow first child for linear representation
	if len(node.Children) > 0 {
		tf.collectCompactParts(node.Children[0], parts)
		if len(node.Children) > 1 {
			*parts = append(*parts, fmt.Sprintf("(+%d more)", len(node.Children)-1))
		}
	}
}

// formatJSON formats tree as JSON (placeholder - would use json.Marshal in real implementation)
func (tf *TreeFormatter) formatJSON(tree *types.FunctionTree) string {
	// In a real implementation, this would use json.Marshal
	return fmt.Sprintf(`{
  "root_function": "%s",
  "total_nodes": %d,
  "max_depth": %d,
  "tree": { ... }
}`, tree.RootFunction, tree.TotalNodes, tree.MaxDepth)
}
