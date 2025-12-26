package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// TestUniversalSymbolGraph_BasicOperations tests core graph operations
func TestUniversalSymbolGraph_BasicOperations(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Create test symbols
	symbol1 := createTestSymbol("TestFunction", types.SymbolKindFunction, "go", 1, 1)
	symbol2 := createTestSymbol("TestClass", types.SymbolKindClass, "javascript", 2, 2)
	symbol3 := createTestSymbol("TestMethod", types.SymbolKindMethod, "go", 1, 3)

	// Test AddSymbol
	err := usg.AddSymbol(symbol1)
	if err != nil {
		t.Fatalf("Failed to add symbol1: %v", err)
	}

	err = usg.AddSymbol(symbol2)
	if err != nil {
		t.Fatalf("Failed to add symbol2: %v", err)
	}

	err = usg.AddSymbol(symbol3)
	if err != nil {
		t.Fatalf("Failed to add symbol3: %v", err)
	}

	// Test GetSymbol
	retrievedSymbol, exists := usg.GetSymbol(symbol1.Identity.ID)
	if !exists {
		t.Fatal("Symbol1 should exist")
	}
	if retrievedSymbol.Identity.Name != "TestFunction" {
		t.Errorf("Expected TestFunction, got %s", retrievedSymbol.Identity.Name)
	}

	// Test GetSymbolsByName
	symbols := usg.GetSymbolsByName("TestFunction")
	if len(symbols) != 1 {
		t.Errorf("Expected 1 symbol with name TestFunction, got %d", len(symbols))
	}

	// Test GetSymbolsByFile
	fileSymbols := usg.GetSymbolsByFile(types.FileID(1))
	if len(fileSymbols) != 2 { // symbol1 and symbol3 are in file 1
		t.Errorf("Expected 2 symbols in file 1, got %d", len(fileSymbols))
	}

	// Test GetSymbolsByLanguage
	goSymbols := usg.GetSymbolsByLanguage("go")
	if len(goSymbols) != 2 { // symbol1 and symbol3 are Go
		t.Errorf("Expected 2 Go symbols, got %d", len(goSymbols))
	}

	jsSymbols := usg.GetSymbolsByLanguage("javascript")
	if len(jsSymbols) != 1 { // symbol2 is JavaScript
		t.Errorf("Expected 1 JavaScript symbol, got %d", len(jsSymbols))
	}

	// Test stats
	stats := usg.GetStats()
	if stats.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", stats.TotalNodes)
	}
	if stats.NodesByLanguage["go"] != 2 {
		t.Errorf("Expected 2 Go nodes, got %d", stats.NodesByLanguage["go"])
	}
	if stats.NodesByLanguage["javascript"] != 1 {
		t.Errorf("Expected 1 JavaScript node, got %d", stats.NodesByLanguage["javascript"])
	}
}

// TestUniversalSymbolGraph_Relationships tests relationship functionality
func TestUniversalSymbolGraph_Relationships(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Create test symbols with relationships
	parentClass := createTestSymbol("ParentClass", types.SymbolKindClass, "go", 1, 1)
	childClass := createTestSymbol("ChildClass", types.SymbolKindClass, "go", 1, 2)
	method := createTestSymbol("method", types.SymbolKindMethod, "go", 1, 3)

	// Set up relationships: ChildClass extends ParentClass, and contains method
	childClass.Relationships.Extends = []types.CompositeSymbolID{parentClass.Identity.ID}
	childClass.Relationships.Contains = []types.CompositeSymbolID{method.Identity.ID}
	method.Relationships.ContainedBy = &childClass.Identity.ID

	// Add symbols to graph
	_ = usg.AddSymbol(parentClass)
	_ = usg.AddSymbol(childClass)
	_ = usg.AddSymbol(method)

	// Test GetRelatedSymbols - extends relationship
	extendedSymbols, err := usg.GetRelatedSymbols(childClass.Identity.ID, types.RelationExtends)
	if err != nil {
		t.Fatalf("Failed to get extended symbols: %v", err)
	}
	if len(extendedSymbols) != 1 {
		t.Errorf("Expected 1 extended symbol, got %d", len(extendedSymbols))
	}
	if extendedSymbols[0].Identity.Name != "ParentClass" {
		t.Errorf("Expected ParentClass, got %s", extendedSymbols[0].Identity.Name)
	}

	// Test GetRelatedSymbols - contains relationship
	containedSymbols, err := usg.GetRelatedSymbols(childClass.Identity.ID, types.RelationContains)
	if err != nil {
		t.Fatalf("Failed to get contained symbols: %v", err)
	}
	if len(containedSymbols) != 1 {
		t.Errorf("Expected 1 contained symbol, got %d", len(containedSymbols))
	}
	if containedSymbols[0].Identity.Name != "method" {
		t.Errorf("Expected method, got %s", containedSymbols[0].Identity.Name)
	}

	// Test GetReverseRelatedSymbols - what extends ParentClass
	extendingSymbols, err := usg.GetReverseRelatedSymbols(parentClass.Identity.ID, types.RelationExtends)
	if err != nil {
		t.Fatalf("Failed to get extending symbols: %v", err)
	}
	if len(extendingSymbols) != 1 {
		t.Errorf("Expected 1 extending symbol, got %d", len(extendingSymbols))
	}
	if extendingSymbols[0].Identity.Name != "ChildClass" {
		t.Errorf("Expected ChildClass, got %s", extendingSymbols[0].Identity.Name)
	}
}

// TestUniversalSymbolGraph_FileCoLocation tests file co-location functionality
func TestUniversalSymbolGraph_FileCoLocation(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Create multiple symbols in the same file
	symbol1 := createTestSymbol("Function1", types.SymbolKindFunction, "go", 1, 1)
	symbol2 := createTestSymbol("Function2", types.SymbolKindFunction, "go", 1, 2)
	symbol3 := createTestSymbol("Class1", types.SymbolKindClass, "go", 1, 3)
	symbol4 := createTestSymbol("Function3", types.SymbolKindFunction, "go", 2, 4) // different file

	_ = usg.AddSymbol(symbol1)
	_ = usg.AddSymbol(symbol2)
	_ = usg.AddSymbol(symbol3)
	_ = usg.AddSymbol(symbol4)

	// Test GetFileCoLocatedSymbols
	coLocated, err := usg.GetFileCoLocatedSymbols(symbol1.Identity.ID)
	if err != nil {
		t.Fatalf("Failed to get co-located symbols: %v", err)
	}

	// Should return 2 other symbols in the same file (symbol2 and symbol3)
	if len(coLocated) != 2 {
		t.Errorf("Expected 2 co-located symbols, got %d", len(coLocated))
	}

	// Verify the co-located symbols are the expected ones
	names := make(map[string]bool)
	for _, sym := range coLocated {
		names[sym.Identity.Name] = true
	}
	if !names["Function2"] || !names["Class1"] {
		t.Errorf("Expected Function2 and Class1 in co-located symbols, got %v", names)
	}
}

// TestUniversalSymbolGraph_UsageTracking tests usage tracking functionality
func TestUniversalSymbolGraph_UsageTracking(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Create test symbol
	symbol := createTestSymbol("PopularFunction", types.SymbolKindFunction, "go", 1, 1)
	_ = usg.AddSymbol(symbol)

	// Track usage
	location := types.SymbolLocation{FileID: types.FileID(2), Line: 10, Column: 5}
	err := usg.UpdateUsage(symbol.Identity.ID, "reference", location)
	if err != nil {
		t.Fatalf("Failed to update usage: %v", err)
	}

	// Update usage multiple times
	for i := 0; i < 5; i++ {
		location.Line = 10 + i
		_ = usg.UpdateUsage(symbol.Identity.ID, "call", location)
	}

	// Check usage was tracked
	updatedSymbol, exists := usg.GetSymbol(symbol.Identity.ID)
	if !exists {
		t.Fatal("Symbol should still exist")
	}
	if updatedSymbol.Usage.ReferenceCount != 1 {
		t.Errorf("Expected 1 reference, got %d", updatedSymbol.Usage.ReferenceCount)
	}
	if updatedSymbol.Usage.CallCount != 5 {
		t.Errorf("Expected 5 calls, got %d", updatedSymbol.Usage.CallCount)
	}

	// Test GetUsageHotSpots
	hotSpots := usg.GetUsageHotSpots(10)
	if len(hotSpots) != 1 {
		t.Errorf("Expected 1 hot spot, got %d", len(hotSpots))
	}
	if hotSpots[0].Identity.Name != "PopularFunction" {
		t.Errorf("Expected PopularFunction as hot spot, got %s", hotSpots[0].Identity.Name)
	}
}

// TestUniversalSymbolGraph_BuildRelationshipTree tests tree building functionality
func TestUniversalSymbolGraph_BuildRelationshipTree(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Create a hierarchy: GrandParent -> Parent -> Child
	grandParent := createTestSymbol("GrandParent", types.SymbolKindClass, "go", 1, 1)
	parent := createTestSymbol("Parent", types.SymbolKindClass, "go", 1, 2)
	child := createTestSymbol("Child", types.SymbolKindClass, "go", 1, 3)

	// Set up relationships
	parent.Relationships.Extends = []types.CompositeSymbolID{grandParent.Identity.ID}
	child.Relationships.Extends = []types.CompositeSymbolID{parent.Identity.ID}

	_ = usg.AddSymbol(grandParent)
	_ = usg.AddSymbol(parent)
	_ = usg.AddSymbol(child)

	// Build relationship tree
	tree, err := usg.BuildRelationshipTree(child.Identity.ID, []types.RelationshipType{types.RelationExtends}, 3)
	if err != nil {
		t.Fatalf("Failed to build relationship tree: %v", err)
	}

	if tree.RootSymbol.Identity.Name != "Child" {
		t.Errorf("Expected Child as root, got %s", tree.RootSymbol.Identity.Name)
	}

	// Verify tree structure
	if tree.Root == nil {
		t.Fatal("Tree root should not be nil")
	}
	if tree.Root.Symbol.Identity.Name != "Child" {
		t.Errorf("Expected Child in tree root, got %s", tree.Root.Symbol.Identity.Name)
	}

	// Check if Parent is in the children
	extendsChildren := tree.Root.Children[types.RelationExtends]
	if len(extendsChildren) != 1 {
		t.Errorf("Expected 1 extends child, got %d", len(extendsChildren))
	}
	if extendsChildren[0].Symbol.Identity.Name != "Parent" {
		t.Errorf("Expected Parent as extends child, got %s", extendsChildren[0].Symbol.Identity.Name)
	}
}

// TestUniversalSymbolGraph_RemoveSymbol tests symbol removal
func TestUniversalSymbolGraph_RemoveSymbol(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Add symbols
	symbol1 := createTestSymbol("ToRemove", types.SymbolKindFunction, "go", 1, 1)
	symbol2 := createTestSymbol("ToKeep", types.SymbolKindFunction, "go", 1, 2)

	_ = usg.AddSymbol(symbol1)
	_ = usg.AddSymbol(symbol2)

	// Verify symbols exist
	if _, exists := usg.GetSymbol(symbol1.Identity.ID); !exists {
		t.Fatal("Symbol1 should exist before removal")
	}
	if _, exists := usg.GetSymbol(symbol2.Identity.ID); !exists {
		t.Fatal("Symbol2 should exist before removal")
	}

	// Remove symbol1
	err := usg.RemoveSymbol(symbol1.Identity.ID)
	if err != nil {
		t.Fatalf("Failed to remove symbol: %v", err)
	}

	// Verify symbol1 is gone but symbol2 remains
	if _, exists := usg.GetSymbol(symbol1.Identity.ID); exists {
		t.Fatal("Symbol1 should not exist after removal")
	}
	if _, exists := usg.GetSymbol(symbol2.Identity.ID); !exists {
		t.Fatal("Symbol2 should still exist after removing symbol1")
	}

	// Check stats updated
	stats := usg.GetStats()
	if stats.TotalNodes != 1 {
		t.Errorf("Expected 1 node after removal, got %d", stats.TotalNodes)
	}
}

// TestUniversalSymbolGraph_CallGraphIntegration tests integration with existing CallGraph
// DISABLED: Requires CallGraph type which was removed from core
func TestUniversalSymbolGraph_CallGraphIntegration(t *testing.T) {
	t.Skip("Disabled: CallGraph type removed from core")
}

// TestUniversalSymbolGraph_ErrorHandling tests error conditions
func TestUniversalSymbolGraph_ErrorHandling(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Test adding nil symbol
	err := usg.AddSymbol(nil)
	if err == nil {
		t.Error("Adding nil symbol should return error")
	}

	// Test adding symbol with invalid ID
	invalidSymbol := createTestSymbol("Invalid", types.SymbolKindFunction, "go", 1, 1)
	invalidSymbol.Identity.ID = types.CompositeSymbolID{} // Zero value, invalid
	err = usg.AddSymbol(invalidSymbol)
	if err == nil {
		t.Error("Adding symbol with invalid ID should return error")
	}

	// Test getting relationships for non-existent symbol
	nonExistentID := types.CompositeSymbolID{FileID: 999, LocalSymbolID: 999}
	_, err = usg.GetRelatedSymbols(nonExistentID, types.RelationExtends)
	if err == nil {
		t.Error("Getting relationships for non-existent symbol should return error")
	}

	// Test removing non-existent symbol
	err = usg.RemoveSymbol(nonExistentID)
	if err == nil {
		t.Error("Removing non-existent symbol should return error")
	}
}

// TestUniversalSymbolGraph_ConcurrentAccess tests thread safety
func TestUniversalSymbolGraph_ConcurrentAccess(t *testing.T) {
	usg := NewUniversalSymbolGraph()

	// Test concurrent symbol addition
	done := make(chan bool)

	// Launch multiple goroutines that add symbols concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			// Use unique FileID and LocalSymbolID to avoid conflicts
			symbol := createTestSymbol(fmt.Sprintf("Function%d", id), types.SymbolKindFunction, "go", types.FileID(100+id), uint32(200+id))
			_ = usg.AddSymbol(symbol)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all symbols were added
	stats := usg.GetStats()
	if stats.TotalNodes != 10 {
		t.Errorf("Expected 10 nodes after concurrent addition, got %d", stats.TotalNodes)
	}
}

// Helper function to create test symbols
func createTestSymbol(name string, kind types.SymbolKind, language string, fileID types.FileID, localSymbolID uint32) *types.UniversalSymbolNode {
	return &types.UniversalSymbolNode{
		Identity: types.SymbolIdentity{
			ID: types.CompositeSymbolID{
				FileID:        fileID,
				LocalSymbolID: localSymbolID,
			},
			Name:     name,
			FullName: name,
			Kind:     kind,
			Language: language,
			Location: types.SymbolLocation{
				FileID: fileID,
				Line:   1,
				Column: 1,
			},
		},
		Relationships: types.SymbolRelationships{
			Extends:       []types.CompositeSymbolID{},
			Implements:    []types.CompositeSymbolID{},
			Contains:      []types.CompositeSymbolID{},
			Dependencies:  []types.SymbolDependency{},
			Dependents:    []types.CompositeSymbolID{},
			CallsTo:       []types.FunctionCall{},
			CalledBy:      []types.FunctionCall{},
			FileCoLocated: []types.CompositeSymbolID{},
			CrossLanguage: []types.CrossLanguageLink{},
		},
		Visibility: types.SymbolVisibility{
			Access:      types.AccessPublic,
			IsExported:  true,
			IsExternal:  false,
			IsBuiltin:   false,
			IsGenerated: false,
		},
		Usage: types.SymbolUsage{
			ReferencingFiles: []types.FileID{},
			HotSpots:         []types.UsageHotSpot{},
			FirstSeen:        time.Now(),
			LastModified:     time.Now(),
			LastReferenced:   time.Now(),
		},
		Metadata: types.SymbolMetadata{
			Documentation:   []string{},
			Comments:        []string{},
			Attributes:      []types.ContextAttribute{},
			Annotations:     []types.SymbolAnnotation{},
			ComplexityScore: 1,
			CouplingScore:   1,
			CohesionScore:   8,
			EditRiskScore:   2,
			StabilityTags:   []string{},
			SafetyNotes:     []string{},
		},
	}
}
