package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/standardbeagle/lci/internal/types"
)

// TestNewTreeFormatter tests the new tree formatter.
func TestNewTreeFormatter(t *testing.T) {
	// Test with default options
	formatter := NewTreeFormatter(FormatterOptions{})
	assert.NotNil(t, formatter)
	assert.Equal(t, "  ", formatter.options.Indent)

	// Test with custom options
	options := FormatterOptions{
		Format:      "text",
		ShowLines:   true,
		ShowMetrics: true,
		AgentMode:   true,
		MaxDepth:    5,
		Indent:      "\t",
	}
	formatter = NewTreeFormatter(options)
	assert.Equal(t, options, formatter.options)
}

// TestTreeFormatter_Format_NilTree tests the tree formatter format nil tree.
func TestTreeFormatter_Format_NilTree(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{})

	// Test nil tree
	output := formatter.Format(nil)
	assert.Equal(t, "No tree data available", output)

	// Test tree with nil root
	tree := &types.FunctionTree{Root: nil}
	output = formatter.Format(tree)
	assert.Equal(t, "No tree data available", output)
}

// TestTreeFormatter_Format_SimpleTree tests the tree formatter format simple tree.
func TestTreeFormatter_Format_SimpleTree(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format: "text",
	})

	// Create simple tree
	tree := &types.FunctionTree{
		RootFunction: "main",
		TotalNodes:   2,
		MaxDepth:     1,
		Root: &types.TreeNode{
			Name:     "main",
			FilePath: "/main.go",
			Line:     10,
			Depth:    0,
			NodeType: types.NodeTypeFunction,
			Children: []*types.TreeNode{
				{
					Name:     "helper",
					FilePath: "/helper.go",
					Line:     20,
					Depth:    1,
					NodeType: types.NodeTypeFunction,
					Children: []*types.TreeNode{},
				},
			},
		},
	}

	output := formatter.Format(tree)

	// Verify output contains expected elements
	assert.Contains(t, output, "Function tree for 'main'")
	assert.Contains(t, output, "Total nodes: 2")
	assert.Contains(t, output, "→ main")
	assert.Contains(t, output, "└─→ helper")
}

// TestTreeFormatter_Format_WithLines tests the tree formatter format with lines.
func TestTreeFormatter_Format_WithLines(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format:    "text",
		ShowLines: true,
	})

	tree := createTestTree()
	output := formatter.Format(tree)

	// Should show file paths and line numbers
	assert.Contains(t, output, "[/main.go:10]")
	assert.Contains(t, output, "[/helper.go:20]")
}

// TestTreeFormatter_Format_AgentMode tests the tree formatter format agent mode.
func TestTreeFormatter_Format_AgentMode(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format:    "text",
		AgentMode: true,
	})

	// Create tree with risk indicators
	tree := &types.FunctionTree{
		RootFunction: "main",
		TotalNodes:   2,
		MaxDepth:     1,
		Root: &types.TreeNode{
			Name:           "main",
			FilePath:       "/main.go",
			Line:           10,
			Depth:          0,
			NodeType:       types.NodeTypeFunction,
			EditRiskScore:  8,
			DependentCount: 15,
			StabilityTags:  []string{"CORE", "PUBLIC_API"},
			Children:       []*types.TreeNode{},
		},
	}

	output := formatter.Format(tree)

	// Should show agent mode indicators
	assert.Contains(t, output, "🔍 Function Tree Analysis")
	assert.Contains(t, output, "📊 Total Nodes")
	assert.Contains(t, output, "⚠️  Risk Levels")
	assert.Contains(t, output, "🔴") // High risk indicator
	assert.Contains(t, output, "⚡CORE")
	assert.Contains(t, output, "🌐PUBLIC")
	assert.Contains(t, output, "👥15 deps")
	assert.Contains(t, output, "⚠️HIGH-RISK")
}

// TestTreeFormatter_Format_MaxDepth tests the tree formatter format max depth.
func TestTreeFormatter_Format_MaxDepth(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format:   "text",
		MaxDepth: 1,
	})

	// Create deep tree
	tree := &types.FunctionTree{
		RootFunction: "main",
		Root: &types.TreeNode{
			Name:  "main",
			Depth: 0,
			Children: []*types.TreeNode{
				{
					Name:  "level1",
					Depth: 1,
					Children: []*types.TreeNode{
						{
							Name:  "level2",
							Depth: 2,
							Children: []*types.TreeNode{
								{
									Name:     "level3",
									Depth:    3,
									Children: []*types.TreeNode{},
								},
							},
						},
					},
				},
			},
		},
	}

	output := formatter.Format(tree)

	// Should include up to depth 1
	assert.Contains(t, output, "main")
	assert.Contains(t, output, "level1")
	// Should not include deeper levels
	assert.NotContains(t, output, "level2")
	assert.NotContains(t, output, "level3")
}

// TestTreeFormatter_Format_CompactMode tests the tree formatter format compact mode.
func TestTreeFormatter_Format_CompactMode(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format: "compact",
	})

	tree := createTestTree()
	output := formatter.Format(tree)

	// Compact format should be single line with arrows
	assert.Contains(t, output, "main → helper")
	assert.NotContains(t, output, "\n")
}

// TestTreeFormatter_Format_CompactMode_MultipleChildren tests the tree formatter format compact mode multiple children.
func TestTreeFormatter_Format_CompactMode_MultipleChildren(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format: "compact",
	})

	tree := &types.FunctionTree{
		Root: &types.TreeNode{
			Name: "main",
			Children: []*types.TreeNode{
				{Name: "child1", Children: []*types.TreeNode{}},
				{Name: "child2", Children: []*types.TreeNode{}},
				{Name: "child3", Children: []*types.TreeNode{}},
			},
		},
	}

	output := formatter.Format(tree)

	// Should show first child and indicate more
	assert.Contains(t, output, "main → child1")
	assert.Contains(t, output, "(+2 more)")
}

// TestTreeFormatter_Format_JSONMode tests the tree formatter format j s o n mode.
func TestTreeFormatter_Format_JSONMode(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format: "json",
	})

	tree := createTestTree()
	output := formatter.Format(tree)

	// JSON format should contain expected fields
	assert.Contains(t, output, `"root_function": "main"`)
	assert.Contains(t, output, `"total_nodes": 2`)
	assert.Contains(t, output, `"max_depth": 1`)
	assert.Contains(t, output, `"tree": { ... }`)
}

// TestTreeFormatter_NodeTypeIndicators tests the tree formatter node type indicators.
func TestTreeFormatter_NodeTypeIndicators(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format:    "text",
		AgentMode: true,
	})

	tree := &types.FunctionTree{
		RootFunction: "main",
		Root: &types.TreeNode{
			Name:     "main",
			NodeType: types.NodeTypeFunction,
			Children: []*types.TreeNode{
				{
					Name:     "external_lib",
					NodeType: types.NodeTypeExternal,
					Children: []*types.TreeNode{},
				},
				{
					Name:     "recursive_func",
					NodeType: types.NodeTypeRecursive,
					Children: []*types.TreeNode{},
				},
			},
		},
	}

	output := formatter.Format(tree)

	// Should show node type indicators
	assert.Contains(t, output, "external_lib 🔗")
	assert.Contains(t, output, "recursive_func 🔄")
}

// TestTreeFormatter_ComplexTree tests the tree formatter complex tree.
func TestTreeFormatter_ComplexTree(t *testing.T) {
	formatter := NewTreeFormatter(FormatterOptions{
		Format:      "text",
		ShowLines:   true,
		ShowMetrics: true,
		AgentMode:   true,
	})

	// Create complex tree with various features
	tree := &types.FunctionTree{
		RootFunction: "processData",
		TotalNodes:   5,
		MaxDepth:     3,
		Root: &types.TreeNode{
			Name:           "processData",
			FilePath:       "/processor.go",
			Line:           100,
			Depth:          0,
			NodeType:       types.NodeTypeFunction,
			EditRiskScore:  6,
			DependentCount: 8,
			StabilityTags:  []string{"PUBLIC_API"},
			Children: []*types.TreeNode{
				{
					Name:           "validateInput",
					FilePath:       "/validator.go",
					Line:           50,
					Depth:          1,
					NodeType:       types.NodeTypeFunction,
					EditRiskScore:  3,
					DependentCount: 2,
					Children:       []*types.TreeNode{},
				},
				{
					Name:           "transformData",
					FilePath:       "/transformer.go",
					Line:           200,
					Depth:          1,
					NodeType:       types.NodeTypeFunction,
					EditRiskScore:  5,
					DependentCount: 4,
					Children: []*types.TreeNode{
						{
							Name:          "applyRules",
							FilePath:      "/rules.go",
							Line:          150,
							Depth:         2,
							NodeType:      types.NodeTypeFunction,
							EditRiskScore: 2,
							Children:      []*types.TreeNode{},
						},
					},
				},
			},
		},
	}

	output := formatter.Format(tree)

	// Verify complex tree rendering
	assert.Contains(t, output, "processData")
	assert.Contains(t, output, "validateInput")
	assert.Contains(t, output, "transformData")
	assert.Contains(t, output, "applyRules")

	// Verify risk indicators
	assert.Contains(t, output, "🟡") // Medium risk
	assert.Contains(t, output, "🟢") // Low risk

	// Verify file paths
	assert.Contains(t, output, "[/processor.go:100]")
	assert.Contains(t, output, "[/validator.go:50]")

	// Verify tree structure
	assert.Contains(t, output, "├─→")
	assert.Contains(t, output, "└─→")
}

// Helper function to create a simple test tree
func createTestTree() *types.FunctionTree {
	return &types.FunctionTree{
		RootFunction: "main",
		TotalNodes:   2,
		MaxDepth:     1,
		Root: &types.TreeNode{
			Name:     "main",
			FilePath: "/main.go",
			Line:     10,
			Depth:    0,
			NodeType: types.NodeTypeFunction,
			Children: []*types.TreeNode{
				{
					Name:     "helper",
					FilePath: "/helper.go",
					Line:     20,
					Depth:    1,
					NodeType: types.NodeTypeFunction,
					Children: []*types.TreeNode{},
				},
			},
		},
	}
}
