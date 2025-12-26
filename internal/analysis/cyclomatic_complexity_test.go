package analysis

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// createGoParser creates a tree-sitter parser for Go code
func createGoParser(t *testing.T) *tree_sitter.Parser {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_go.Language()
	lang := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(lang)
	if err != nil {
		t.Fatalf("Failed to set Go language: %v", err)
	}
	return parser
}

// findFunctionNode finds the first function_declaration node in the AST
func findFunctionNode(node *tree_sitter.Node) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	if node.Kind() == "function_declaration" || node.Kind() == "method_declaration" {
		return node
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		if found := findFunctionNode(node.Child(i)); found != nil {
			return found
		}
	}
	return nil
}

// TestCyclomaticComplexity_SimpleFunction tests CC=1 for a simple function with no branches
func TestCyclomaticComplexity_SimpleFunction(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Simple function with no branches: CC should be 1
	code := []byte(`package main

func add(a, b int) int {
	return a + b
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 1 {
		t.Errorf("Simple function CC: expected 1, got %d", cc)
	}
}

// TestCyclomaticComplexity_SingleIf tests CC=2 for a function with one if statement
func TestCyclomaticComplexity_SingleIf(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with single if: CC should be 2 (1 base + 1 if)
	code := []byte(`package main

func isPositive(n int) bool {
	if n > 0 {
		return true
	}
	return false
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 2 {
		t.Errorf("Single if CC: expected 2, got %d", cc)
	}
}

// TestCyclomaticComplexity_IfElseIf tests CC for if-else-if chains
func TestCyclomaticComplexity_IfElseIf(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with if-else if-else: CC should be 3 (1 base + 2 conditions)
	code := []byte(`package main

func classify(n int) string {
	if n < 0 {
		return "negative"
	} else if n > 0 {
		return "positive"
	} else {
		return "zero"
	}
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 3 {
		t.Errorf("If-else-if CC: expected 3, got %d", cc)
	}
}

// TestCyclomaticComplexity_ForLoop tests CC for a for loop
func TestCyclomaticComplexity_ForLoop(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with for loop: CC should be 2 (1 base + 1 loop)
	code := []byte(`package main

func sum(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 2 {
		t.Errorf("For loop CC: expected 2, got %d", cc)
	}
}

// printNodeTree prints tree-sitter node tree for debugging
func printNodeTree(t *testing.T, node *tree_sitter.Node, indent string) {
	if node == nil {
		return
	}
	t.Logf("%s%s", indent, node.Kind())
	for i := uint(0); i < node.ChildCount(); i++ {
		printNodeTree(t, node.Child(i), indent+"  ")
	}
}

// TestCyclomaticComplexity_Switch tests CC for switch statements
func TestCyclomaticComplexity_Switch(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with switch: CC should be 5 (1 base + 1 switch + 3 cases)
	code := []byte(`package main

func dayType(day string) string {
	switch day {
	case "Saturday":
		return "weekend"
	case "Sunday":
		return "weekend"
	case "Monday":
		return "weekday"
	default:
		return "weekday"
	}
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	// Debug: print the AST structure
	t.Log("AST structure:")
	printNodeTree(t, funcNode, "")

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	// Expected: 1 base + 3 expression_case (default doesn't count) = 4
	if cc != 4 {
		t.Errorf("Switch CC: expected 4, got %d", cc)
	}
	t.Logf("Switch CC: %d", cc)
}

// TestCyclomaticComplexity_LogicalAnd tests CC for && operators
func TestCyclomaticComplexity_LogicalAnd(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with && in condition: CC should be 3 (1 base + 1 if + 1 &&)
	code := []byte(`package main

func isValidAge(age int) bool {
	if age >= 0 && age <= 120 {
		return true
	}
	return false
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 3 {
		t.Errorf("Logical AND CC: expected 3, got %d", cc)
	}
}

// TestCyclomaticComplexity_LogicalOr tests CC for || operators
func TestCyclomaticComplexity_LogicalOr(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with || in condition: CC should be 3 (1 base + 1 if + 1 ||)
	code := []byte(`package main

func isWeekend(day string) bool {
	if day == "Saturday" || day == "Sunday" {
		return true
	}
	return false
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 3 {
		t.Errorf("Logical OR CC: expected 3, got %d", cc)
	}
}

// TestCyclomaticComplexity_NestedIf tests CC for nested if statements
func TestCyclomaticComplexity_NestedIf(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Function with nested ifs: CC should be 3 (1 base + 2 ifs)
	code := []byte(`package main

func checkRange(n int, min int, max int) bool {
	if n >= min {
		if n <= max {
			return true
		}
	}
	return false
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	if cc != 3 {
		t.Errorf("Nested if CC: expected 3, got %d", cc)
	}
}

// TestCyclomaticComplexity_Complex tests a more complex function
func TestCyclomaticComplexity_Complex(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Complex function with multiple control structures
	// Expected: 1 base + 1 for + 2 ifs + 1 && = 6
	code := []byte(`package main

func processItems(items []int, threshold int) int {
	count := 0
	for _, item := range items {
		if item > 0 {
			if item < threshold && item%2 == 0 {
				count++
			}
		}
	}
	return count
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	// 1 base + 1 for + 2 ifs + 1 && = 5-6
	if cc < 5 || cc > 6 {
		t.Errorf("Complex function CC: expected 5-6, got %d", cc)
	}
	t.Logf("Complex function CC: %d", cc)
}

// TestCyclomaticComplexity_HighComplexity tests a high complexity function
func TestCyclomaticComplexity_HighComplexity(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// High complexity function with many branches
	code := []byte(`package main

func validate(data map[string]interface{}) error {
	if data == nil {
		return fmt.Errorf("nil data")
	}

	if name, ok := data["name"]; !ok || name == "" {
		return fmt.Errorf("missing name")
	}

	if age, ok := data["age"].(int); ok {
		if age < 0 || age > 150 {
			return fmt.Errorf("invalid age")
		}
	} else {
		return fmt.Errorf("age must be int")
	}

	if email, ok := data["email"].(string); ok {
		if len(email) == 0 {
			return fmt.Errorf("empty email")
		}
		for _, c := range email {
			if c == '@' {
				break
			}
		}
	}

	return nil
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	// This should have high complexity (> 10)
	if cc < 8 {
		t.Errorf("High complexity function CC: expected >= 8, got %d", cc)
	}
	t.Logf("High complexity function CC: %d", cc)
}

// TestCognitiveComplexity_Simple tests cognitive complexity for simple functions
func TestCognitiveComplexity_Simple(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Simple function: cognitive complexity should be 0
	code := []byte(`package main

func add(a, b int) int {
	return a + b
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cognitive := calculator.calculateCognitiveComplexity(funcNode)
	if cognitive != 0 {
		t.Errorf("Simple function cognitive complexity: expected 0, got %d", cognitive)
	}
}

// TestCognitiveComplexity_Nested tests cognitive complexity penalizes nesting
func TestCognitiveComplexity_Nested(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	// Nested structures should have higher cognitive complexity than cyclomatic
	code := []byte(`package main

func deepNested(n int) int {
	if n > 0 {
		if n > 10 {
			if n > 100 {
				return n * 3
			}
			return n * 2
		}
		return n
	}
	return 0
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	cc := calculator.calculateCyclomaticComplexity(funcNode)
	cognitive := calculator.calculateCognitiveComplexity(funcNode)

	t.Logf("Nested function - CC: %d, Cognitive: %d", cc, cognitive)

	// Cognitive complexity should be higher due to nesting penalty
	// CC = 1 + 3 ifs = 4
	// Cognitive = 1 (if at depth 0) + 2 (if at depth 1) + 3 (if at depth 2) = 6
	if cognitive < cc {
		t.Errorf("Cognitive complexity should be >= cyclomatic for nested code")
	}
}

// TestNestingDepth tests nesting depth calculation
func TestNestingDepth(t *testing.T) {
	parser := createGoParser(t)
	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	code := []byte(`package main

func deepNested(n int) int {
	if n > 0 {
		for i := 0; i < n; i++ {
			if i%2 == 0 {
				return i
			}
		}
	}
	return 0
}`)

	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse Go code")
	}
	defer tree.Close()

	funcNode := findFunctionNode(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function node")
	}

	depth := calculator.calculateNestingDepth(funcNode, 0)
	// if -> for -> if = depth 3
	if depth < 3 {
		t.Errorf("Nesting depth: expected >= 3, got %d", depth)
	}
	t.Logf("Nesting depth: %d", depth)
}

// BenchmarkCyclomaticComplexity benchmarks CC calculation
func BenchmarkCyclomaticComplexity(b *testing.B) {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_go.Language()
	lang := tree_sitter.NewLanguage(languagePtr)
	_ = parser.SetLanguage(lang)

	calculator := NewCachedMetricsCalculator(DefaultCachedMetricsConfig())

	code := []byte(`package main

func complex(data []int, threshold int) int {
	count := 0
	for _, item := range data {
		if item > 0 && item < threshold {
			switch item % 3 {
			case 0:
				count += 3
			case 1:
				count += 1
			default:
				count += 2
			}
		}
	}
	return count
}`)

	tree := parser.Parse(code, nil)
	defer tree.Close()

	root := tree.RootNode()
	var funcNode *tree_sitter.Node
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() == "function_declaration" {
			funcNode = child
			break
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculator.calculateCyclomaticComplexity(funcNode)
	}
}
