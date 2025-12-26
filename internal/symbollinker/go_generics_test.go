package symbollinker

import (
	"testing"

	"github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// TestGoGenerics_BasicGenericFunction tests extraction of generic functions
func TestGoGenerics_BasicGenericFunction(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `package main

// Min returns the minimum of two values
func Min[T comparable](a, b T) T {
	if a < b {
		return a
	}
	return b
}
`
	fileID := types.FileID(1)
	extractor := NewGoExtractor()
	tree := parseGoCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the Min function from symbols map
	var minFunc *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "Min" {
			minFunc = sym
			break
		}
	}
	if minFunc == nil {
		t.Fatal("Min function not found")
	}

	// Verify it's a function
	if minFunc.Kind != types.SymbolKindFunction {
		t.Errorf("Expected SymbolKindFunction, got %v", minFunc.Kind)
	}

	// Verify type parameters are captured
	if minFunc.TypeParameters == nil || len(minFunc.TypeParameters) == 0 {
		t.Error("Expected type parameters to be extracted")
	}

	// Should have one type parameter: T
	if len(minFunc.TypeParameters) != 1 {
		t.Errorf("Expected 1 type parameter, got %d", len(minFunc.TypeParameters))
	}

	if minFunc.TypeParameters[0].Name != "T" {
		t.Errorf("Expected type parameter name 'T', got %s", minFunc.TypeParameters[0].Name)
	}

	if minFunc.TypeParameters[0].Constraint != "comparable" {
		t.Errorf("Expected constraint 'comparable', got %s", minFunc.TypeParameters[0].Constraint)
	}
}

// TestGoGenerics_GenericStruct tests extraction of generic struct types
func TestGoGenerics_GenericStruct(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `package main

// Stack is a generic stack data structure
type Stack[T any] struct {
	items []T
	count int
}

func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
	s.count++
}

func (s *Stack[T]) Pop() (T, bool) {
	if s.count == 0 {
		var zero T
		return zero, false
	}
	item := s.items[s.count-1]
	s.items = s.items[:s.count-1]
	s.count--
	return item, true
}
`
	fileID := types.FileID(1)
	extractor := NewGoExtractor()
	tree := parseGoCode(t, code)
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the Stack type
	stackType := findSymbolByName(table, "Stack")
	if stackType == nil {
		t.Fatal("Stack type not found")
	}

	// Verify it's a struct
	if stackType.Kind != types.SymbolKindStruct {
		t.Errorf("Expected SymbolKindStruct, got %v", stackType.Kind)
	}

	// Verify type parameters
	if stackType.TypeParameters == nil || len(stackType.TypeParameters) == 0 {
		t.Error("Expected type parameters to be extracted")
	}

	if len(stackType.TypeParameters) != 1 {
		t.Errorf("Expected 1 type parameter, got %d", len(stackType.TypeParameters))
	}

	if stackType.TypeParameters[0].Name != "T" {
		t.Errorf("Expected type parameter name 'T', got %s", stackType.TypeParameters[0].Name)
	}

	if stackType.TypeParameters[0].Constraint != "any" {
		t.Errorf("Expected constraint 'any', got %s", stackType.TypeParameters[0].Constraint)
	}

	// Find the Push method
	pushMethod := findSymbolByName(table, "Stack.Push")
	if pushMethod == nil {
		t.Fatal("Push method not found")
	}

	// Verify Push method has type parameter reference
	if pushMethod.TypeParameters == nil || len(pushMethod.TypeParameters) == 0 {
		t.Error("Expected Push method to reference type parameters from receiver")
	}

	// Find the Pop method
	popMethod := findSymbolByName(table, "Stack.Pop")
	if popMethod == nil {
		t.Fatal("Pop method not found")
	}

	// Verify return type includes generic type
	if popMethod.Signature == "" {
		t.Error("Expected method signature to be captured")
	}
}

// TestGoGenerics_MultipleTypeParameters tests functions with multiple type parameters
func TestGoGenerics_MultipleTypeParameters(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `package main

// Map transforms a slice using a function
func Map[T any, U any](items []T, fn func(T) U) []U {
	result := make([]U, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}
	return result
}
`
	fileID := types.FileID(1)
	extractor := NewGoExtractor()
	tree := parseGoCode(t, code)
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the Map function
	mapFunc := findSymbolByName(table, "Map")
	if mapFunc == nil {
		t.Fatal("Map function not found")
	}

	// Should have two type parameters
	if len(mapFunc.TypeParameters) != 2 {
		t.Errorf("Expected 2 type parameters, got %d", len(mapFunc.TypeParameters))
	}

	// Verify both type parameters
	expectedParams := []struct {
		name       string
		constraint string
	}{
		{"T", "any"},
		{"U", "any"},
	}

	for i, expected := range expectedParams {
		if i >= len(mapFunc.TypeParameters) {
			t.Errorf("Missing type parameter at index %d", i)
			continue
		}
		param := mapFunc.TypeParameters[i]
		if param.Name != expected.name {
			t.Errorf("Type parameter %d: expected name %s, got %s", i, expected.name, param.Name)
		}
		if param.Constraint != expected.constraint {
			t.Errorf("Type parameter %d: expected constraint %s, got %s", i, expected.constraint, param.Constraint)
		}
	}
}

// TestGoGenerics_ConstraintInterfaces tests type parameters with interface constraints
func TestGoGenerics_ConstraintInterfaces(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `package main

import "fmt"

// Stringer is anything that can convert to string
type Stringer interface {
	String() string
}

// Print prints any value that implements Stringer
func Print[T Stringer](value T) {
	fmt.Println(value.String())
}

// Ordered represents types that can be ordered
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string
}

// Max returns the maximum of two ordered values
func Max[T Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}
`
	fileID := types.FileID(1)
	extractor := NewGoExtractor()
	tree := parseGoCode(t, code)
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the Print function
	printFunc := findSymbolByName(table, "Print")
	if printFunc == nil {
		t.Fatal("Print function not found")
	}

	if len(printFunc.TypeParameters) != 1 {
		t.Fatalf("Expected 1 type parameter, got %d", len(printFunc.TypeParameters))
	}

	if printFunc.TypeParameters[0].Constraint != "Stringer" {
		t.Errorf("Expected constraint 'Stringer', got %s", printFunc.TypeParameters[0].Constraint)
	}

	// Find the Max function
	maxFunc := findSymbolByName(table, "Max")
	if maxFunc == nil {
		t.Fatal("Max function not found")
	}

	if len(maxFunc.TypeParameters) != 1 {
		t.Fatalf("Expected 1 type parameter, got %d", len(maxFunc.TypeParameters))
	}

	if maxFunc.TypeParameters[0].Constraint != "Ordered" {
		t.Errorf("Expected constraint 'Ordered', got %s", maxFunc.TypeParameters[0].Constraint)
	}

	// Verify Ordered interface with type union is extracted
	orderedInterface := findSymbolByName(table, "Ordered")
	if orderedInterface == nil {
		t.Fatal("Ordered interface not found")
	}

	if orderedInterface.Kind != types.SymbolKindInterface {
		t.Errorf("Expected SymbolKindInterface, got %v", orderedInterface.Kind)
	}
}

// TestGoGenerics_GenericInterface tests generic interface types
func TestGoGenerics_GenericInterface(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `package main

// Container is a generic container interface
type Container[T any] interface {
	Add(item T)
	Get(index int) (T, bool)
	Remove(index int) bool
	Size() int
}
`
	fileID := types.FileID(1)
	extractor := NewGoExtractor()
	tree := parseGoCode(t, code)
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the Container interface
	container := findSymbolByName(table, "Container")
	if container == nil {
		t.Fatal("Container interface not found")
	}

	if container.Kind != types.SymbolKindInterface {
		t.Errorf("Expected SymbolKindInterface, got %v", container.Kind)
	}

	// Verify type parameters
	if len(container.TypeParameters) != 1 {
		t.Errorf("Expected 1 type parameter, got %d", len(container.TypeParameters))
	}

	if container.TypeParameters[0].Name != "T" {
		t.Errorf("Expected type parameter name 'T', got %s", container.TypeParameters[0].Name)
	}

	// Verify interface methods reference the type parameter
	methods := []string{"Container.Add", "Container.Get", "Container.Remove", "Container.Size"}
	for _, methodName := range methods {
		method := findSymbolByName(table, methodName)
		if method == nil {
			t.Errorf("Method %s not found", methodName)
		}
	}
}

// Helper functions

func parseGoCode(t *testing.T, code string) *tree_sitter.Tree {
	// This would use tree-sitter to parse the code
	// For now, returning nil - will be implemented when we add tree-sitter support
	t.Helper()
	return nil
}

func findSymbolByName(table *types.SymbolTable, name string) *types.EnhancedSymbolInfo {
	for _, symbol := range table.Symbols {
		if symbol.Name == name {
			return symbol
		}
	}
	return nil
}
