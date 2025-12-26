package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// addCallRelationship is a test helper to add a call relationship
func addCallRelationship(rt *ReferenceTracker, caller, callee types.SymbolID) {
	ref := types.Reference{
		SourceSymbol: caller,
		TargetSymbol: callee,
		Type:         types.RefTypeCall,
		Quality:      "test",
	}
	rt.AddTestReference(ref)
}

func TestSideEffectPropagator_BasicPropagation(t *testing.T) {
	// Create a mock reference tracker with call relationships
	refTracker := NewReferenceTrackerForTest()

	// Symbol IDs for our test functions
	funcA := types.SymbolID(1001) // calls funcB
	funcB := types.SymbolID(1002) // has side effects (writes to global)
	funcC := types.SymbolID(1003) // pure function

	// Set up call relationships: A -> B, A -> C
	addCallRelationship(refTracker, funcA, funcB)
	addCallRelationship(refTracker, funcA, funcC)

	// Create propagator
	sep := NewSideEffectPropagator(refTracker, nil, nil)

	// Add side effect info
	// funcB has global write
	infoB := types.NewSideEffectInfo()
	infoB.FunctionName = "funcB"
	infoB.Categories = types.SideEffectGlobalWrite
	infoB.Confidence = types.ConfidenceHigh
	infoB.IsPure = false
	sep.AddLocalSideEffect(funcB, infoB)

	// funcC is pure
	infoC := types.NewSideEffectInfo()
	infoC.FunctionName = "funcC"
	infoC.Categories = types.SideEffectNone
	infoC.Confidence = types.ConfidenceHigh
	infoC.IsPure = true
	sep.AddLocalSideEffect(funcC, infoC)

	// funcA has no local side effects
	infoA := types.NewSideEffectInfo()
	infoA.FunctionName = "funcA"
	infoA.Categories = types.SideEffectNone
	infoA.Confidence = types.ConfidenceHigh
	infoA.IsPure = true
	sep.AddLocalSideEffect(funcA, infoA)

	// Run propagation
	err := sep.Propagate()
	if err != nil {
		t.Fatalf("Propagation failed: %v", err)
	}

	// Check results
	resultA := sep.GetSideEffectInfo(funcA)
	if resultA == nil {
		t.Fatal("Expected result for funcA")
	}

	// funcA should now have transitive global write from funcB
	if resultA.TransitiveCategories&types.SideEffectGlobalWrite == 0 {
		t.Error("funcA should have transitive SideEffectGlobalWrite from funcB")
	}

	// funcA should be impure due to calling funcB
	if resultA.IsPure {
		t.Error("funcA should be impure after propagation (calls impure funcB)")
	}

	// funcB should be unchanged
	resultB := sep.GetSideEffectInfo(funcB)
	if resultB == nil {
		t.Fatal("Expected result for funcB")
	}
	if resultB.IsPure {
		t.Error("funcB should remain impure")
	}

	// funcC should be unchanged
	resultC := sep.GetSideEffectInfo(funcC)
	if resultC == nil {
		t.Fatal("Expected result for funcC")
	}
	if !resultC.IsPure {
		t.Error("funcC should remain pure")
	}

	t.Logf("Propagation completed in %d iterations", sep.GetIterationCount())
}

func TestSideEffectPropagator_TransitivePropagation(t *testing.T) {
	// Test that side effects propagate through multiple levels
	// A -> B -> C (C has side effects, should propagate to B and A)

	refTracker := NewReferenceTrackerForTest()

	funcA := types.SymbolID(1001)
	funcB := types.SymbolID(1002)
	funcC := types.SymbolID(1003)

	// A calls B, B calls C
	addCallRelationship(refTracker, funcA, funcB)
	addCallRelationship(refTracker, funcB, funcC)

	sep := NewSideEffectPropagator(refTracker, nil, nil)

	// Only C has local side effects
	infoC := types.NewSideEffectInfo()
	infoC.FunctionName = "funcC"
	infoC.Categories = types.SideEffectIO
	infoC.Confidence = types.ConfidenceHigh
	infoC.IsPure = false
	sep.AddLocalSideEffect(funcC, infoC)

	// B and A are locally pure
	for _, id := range []types.SymbolID{funcA, funcB} {
		info := types.NewSideEffectInfo()
		info.Categories = types.SideEffectNone
		info.IsPure = true
		sep.AddLocalSideEffect(id, info)
	}

	// Run propagation
	sep.Propagate()

	// Check B got C's effects
	resultB := sep.GetSideEffectInfo(funcB)
	if resultB.TransitiveCategories&types.SideEffectIO == 0 {
		t.Error("funcB should have transitive I/O effect from funcC")
	}
	if resultB.IsPure {
		t.Error("funcB should be impure (calls C)")
	}

	// Check A got B's propagated effects (which came from C)
	resultA := sep.GetSideEffectInfo(funcA)
	if resultA.TransitiveCategories&types.SideEffectIO == 0 {
		t.Error("funcA should have transitive I/O effect from funcC via funcB")
	}
	if resultA.IsPure {
		t.Error("funcA should be impure (calls B which calls C)")
	}
}

func TestSideEffectPropagator_DiamondPattern(t *testing.T) {
	// Test diamond pattern: A -> B, A -> C, B -> D, C -> D
	// D has side effects - should propagate to A through both paths

	refTracker := NewReferenceTrackerForTest()

	funcA := types.SymbolID(1001)
	funcB := types.SymbolID(1002)
	funcC := types.SymbolID(1003)
	funcD := types.SymbolID(1004)

	addCallRelationship(refTracker, funcA, funcB)
	addCallRelationship(refTracker, funcA, funcC)
	addCallRelationship(refTracker, funcB, funcD)
	addCallRelationship(refTracker, funcC, funcD)

	sep := NewSideEffectPropagator(refTracker, nil, nil)

	// Only D has side effects
	infoD := types.NewSideEffectInfo()
	infoD.FunctionName = "funcD"
	infoD.Categories = types.SideEffectParamWrite
	infoD.Confidence = types.ConfidenceHigh
	infoD.IsPure = false
	sep.AddLocalSideEffect(funcD, infoD)

	// Others are locally pure
	for _, id := range []types.SymbolID{funcA, funcB, funcC} {
		info := types.NewSideEffectInfo()
		info.Categories = types.SideEffectNone
		info.IsPure = true
		sep.AddLocalSideEffect(id, info)
	}

	sep.Propagate()

	// All should be impure now
	for _, id := range []types.SymbolID{funcA, funcB, funcC} {
		info := sep.GetSideEffectInfo(id)
		if info.IsPure {
			t.Errorf("Symbol %d should be impure after propagation", id)
		}
		if info.TransitiveCategories&types.SideEffectParamWrite == 0 {
			t.Errorf("Symbol %d should have transitive param write", id)
		}
	}
}

func TestSideEffectPropagator_CyclicCallGraph(t *testing.T) {
	// Test cycle: A -> B -> A (with max iterations protection)

	refTracker := NewReferenceTrackerForTest()

	funcA := types.SymbolID(1001)
	funcB := types.SymbolID(1002)

	addCallRelationship(refTracker, funcA, funcB)
	addCallRelationship(refTracker, funcB, funcA)

	config := DefaultSideEffectPropagationConfig()
	config.MaxIterations = 10 // Limit iterations for cycle

	sep := NewSideEffectPropagator(refTracker, nil, config)

	// A has side effects
	infoA := types.NewSideEffectInfo()
	infoA.FunctionName = "funcA"
	infoA.Categories = types.SideEffectGlobalWrite
	infoA.Confidence = types.ConfidenceHigh
	infoA.IsPure = false
	sep.AddLocalSideEffect(funcA, infoA)

	// B is locally pure
	infoB := types.NewSideEffectInfo()
	infoB.FunctionName = "funcB"
	infoB.Categories = types.SideEffectNone
	infoB.IsPure = true
	sep.AddLocalSideEffect(funcB, infoB)

	// Should not hang due to cycle
	sep.Propagate()

	// B should get A's effects
	resultB := sep.GetSideEffectInfo(funcB)
	if resultB.TransitiveCategories&types.SideEffectGlobalWrite == 0 {
		t.Error("funcB should have transitive global write from funcA")
	}

	t.Logf("Cyclic graph propagation completed in %d iterations", sep.GetIterationCount())
}

func TestSideEffectPropagator_GetPureFunctions(t *testing.T) {
	refTracker := NewReferenceTrackerForTest()

	pure1 := types.SymbolID(1001)
	pure2 := types.SymbolID(1002)
	impure := types.SymbolID(1003)

	sep := NewSideEffectPropagator(refTracker, nil, nil)

	// Add pure functions
	for _, id := range []types.SymbolID{pure1, pure2} {
		info := types.NewSideEffectInfo()
		info.Categories = types.SideEffectNone
		info.IsPure = true
		sep.AddLocalSideEffect(id, info)
	}

	// Add impure function
	infoImpure := types.NewSideEffectInfo()
	infoImpure.Categories = types.SideEffectIO
	infoImpure.IsPure = false
	sep.AddLocalSideEffect(impure, infoImpure)

	pure := sep.GetPureFunctions()
	if len(pure) != 2 {
		t.Errorf("Expected 2 pure functions, got %d", len(pure))
	}

	impureFuncs := sep.GetImpureFunctions()
	if len(impureFuncs) != 1 {
		t.Errorf("Expected 1 impure function, got %d", len(impureFuncs))
	}
}

func TestSideEffectPropagator_PurityReport(t *testing.T) {
	sep := NewSideEffectPropagator(nil, nil, nil)

	funcID := types.SymbolID(1001)

	info := types.NewSideEffectInfo()
	info.FunctionName = "processData"
	info.Categories = types.SideEffectParamWrite | types.SideEffectIO
	info.Confidence = types.ConfidenceHigh
	info.IsPure = false
	info.ComputePurityScore()

	sep.AddLocalSideEffect(funcID, info)

	report := sep.GetPurityReport(funcID)
	if report == nil {
		t.Fatal("Expected purity report")
	}

	if report.FunctionName != "processData" {
		t.Errorf("Expected function name 'processData', got '%s'", report.FunctionName)
	}

	if report.IsPure {
		t.Error("Report should show function as impure")
	}

	if len(report.Reasons) < 2 {
		t.Errorf("Expected at least 2 reasons for impurity, got %d", len(report.Reasons))
	}

	t.Logf("Purity report: %+v", report)
}

func TestDefaultSideEffectPropagationConfig(t *testing.T) {
	config := DefaultSideEffectPropagationConfig()

	if config.MaxIterations <= 0 {
		t.Error("MaxIterations should be positive")
	}

	if config.ConfidenceDecay <= 0 || config.ConfidenceDecay >= 1 {
		t.Error("ConfidenceDecay should be between 0 and 1")
	}

	if config.MinConfidence < 0 || config.MinConfidence >= 1 {
		t.Error("MinConfidence should be between 0 and 1")
	}
}

func TestSideEffectPropagator_ConfigurablePropagation(t *testing.T) {
	refTracker := NewReferenceTrackerForTest()

	funcA := types.SymbolID(1001)
	funcB := types.SymbolID(1002) // has I/O

	addCallRelationship(refTracker, funcA, funcB)

	// Configure to NOT propagate I/O
	config := DefaultSideEffectPropagationConfig()
	config.PropagateIO = false

	sep := NewSideEffectPropagator(refTracker, nil, config)

	// B has I/O effect
	infoB := types.NewSideEffectInfo()
	infoB.Categories = types.SideEffectIO
	infoB.IsPure = false
	sep.AddLocalSideEffect(funcB, infoB)

	// A is locally pure
	infoA := types.NewSideEffectInfo()
	infoA.Categories = types.SideEffectNone
	infoA.IsPure = true
	sep.AddLocalSideEffect(funcA, infoA)

	sep.Propagate()

	// A should NOT get I/O effect (propagation disabled)
	resultA := sep.GetSideEffectInfo(funcA)
	if resultA.TransitiveCategories&types.SideEffectIO != 0 {
		t.Error("funcA should not have I/O effect when I/O propagation is disabled")
	}
}
