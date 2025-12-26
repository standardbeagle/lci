package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestParserComplexityCalculation verifies that the parser calculates cyclomatic complexity
// for functions and methods during the parsing phase
func TestParserComplexityCalculation(t *testing.T) {
	// Create a parser
	parser := NewTreeSitterParser()

	tests := []struct {
		name       string
		code       string
		funcName   string
		expectedCC int
	}{
		{
			name: "simple function",
			code: `package main

func add(a, b int) int {
	return a + b
}`,
			funcName:   "add",
			expectedCC: 1,
		},
		{
			name: "function with if",
			code: `package main

func isPositive(n int) bool {
	if n > 0 {
		return true
	}
	return false
}`,
			funcName:   "isPositive",
			expectedCC: 2,
		},
		{
			name: "function with for loop",
			code: `package main

func sum(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}`,
			funcName:   "sum",
			expectedCC: 2,
		},
		{
			name: "function with switch",
			code: `package main

func classify(n int) string {
	switch {
	case n < 0:
		return "negative"
	case n > 0:
		return "positive"
	default:
		return "zero"
	}
}`,
			funcName:   "classify",
			expectedCC: 3, // 1 base + 2 expression_case (default doesn't count)
		},
		{
			name: "function with logical operators",
			code: `package main

func isValid(x, y int) bool {
	if x > 0 && y > 0 {
		return true
	}
	return false
}`,
			funcName:   "isValid",
			expectedCC: 3, // 1 base + 1 if + 1 &&
		},
		{
			name: "complex function",
			code: `package main

func process(items []int, threshold int) int {
	count := 0
	for _, item := range items {
		if item > 0 {
			if item < threshold || item == 100 {
				count++
			}
		}
	}
	return count
}`,
			funcName:   "process",
			expectedCC: 5, // 1 base + 1 for + 2 ifs + 1 ||
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the code
			_, _, _, enhancedSymbols, _, _ := parser.ParseFileEnhanced("test.go", []byte(tt.code))

			// Find the function symbol
			var funcSymbol *types.EnhancedSymbol
			for i := range enhancedSymbols {
				sym := &enhancedSymbols[i]
				if sym.Name == tt.funcName && (sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod) {
					funcSymbol = sym
					break
				}
			}

			if funcSymbol == nil {
				t.Fatalf("Could not find function %s in parsed symbols", tt.funcName)
			}

			if funcSymbol.Complexity != tt.expectedCC {
				t.Errorf("Function %s: expected CC=%d, got CC=%d", tt.funcName, tt.expectedCC, funcSymbol.Complexity)
			} else {
				t.Logf("Function %s: CC=%d (correct)", tt.funcName, funcSymbol.Complexity)
			}
		})
	}
}

// TestParserComplexityForMethods verifies complexity is calculated for methods too
func TestParserComplexityForMethods(t *testing.T) {
	parser := NewTreeSitterParser()

	code := `package main

type Counter struct {
	value int
}

func (c *Counter) Increment() {
	c.value++
}

func (c *Counter) IncrementBy(n int) {
	if n > 0 {
		c.value += n
	}
}

func (c *Counter) Reset() {
	c.value = 0
}
`

	_, _, _, enhancedSymbols, _, _ := parser.ParseFileEnhanced("counter.go", []byte(code))

	// Check each method's complexity
	expectedCC := map[string]int{
		"Increment":   1, // Simple method
		"IncrementBy": 2, // Has one if
		"Reset":       1, // Simple method
	}

	for name, expected := range expectedCC {
		var found bool
		for i := range enhancedSymbols {
			sym := &enhancedSymbols[i]
			if sym.Name == name && sym.Type == types.SymbolTypeMethod {
				found = true
				if sym.Complexity != expected {
					t.Errorf("Method %s: expected CC=%d, got CC=%d", name, expected, sym.Complexity)
				} else {
					t.Logf("Method %s: CC=%d (correct)", name, sym.Complexity)
				}
				break
			}
		}
		if !found {
			t.Errorf("Method %s not found in parsed symbols", name)
		}
	}
}

// TestNonFunctionSymbolsHaveZeroComplexity verifies that non-function symbols have CC=0
func TestNonFunctionSymbolsHaveZeroComplexity(t *testing.T) {
	parser := NewTreeSitterParser()

	code := `package main

const MaxSize = 100

var counter int

type User struct {
	Name string
	Age  int
}

type Handler interface {
	Handle() error
}
`

	_, _, _, enhancedSymbols, _, _ := parser.ParseFileEnhanced("types.go", []byte(code))

	// All non-function symbols should have complexity 0
	for i := range enhancedSymbols {
		sym := &enhancedSymbols[i]
		if sym.Type != types.SymbolTypeFunction && sym.Type != types.SymbolTypeMethod {
			if sym.Complexity != 0 {
				t.Errorf("Non-function symbol %s (type=%v) has complexity %d, expected 0",
					sym.Name, sym.Type, sym.Complexity)
			}
		}
	}
}
