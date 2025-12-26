package analysis

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestSideEffectAnalyzer_PureFunction(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// Simulate analyzing a pure function
	// func add(a, b int) int { return a + b }
	sa.BeginFunction("add", "test.go", 1, 3)
	sa.AddParameter("a", 0)
	sa.AddParameter("b", 1)

	// Only reads of parameters
	sa.RecordAccess("a", nil, types.AccessRead, 2, 10)
	sa.RecordAccess("b", nil, types.AccessRead, 2, 14)

	info := sa.EndFunction()

	if !info.IsPure {
		t.Errorf("expected pure function, got impure: %v", info.ImpurityReasons)
	}
	if info.Categories != types.SideEffectNone {
		t.Errorf("expected no side effects, got: %s", info.Categories.String())
	}
	if info.AccessPattern.Pattern != types.PatternPure {
		t.Errorf("expected pure pattern, got: %s", info.AccessPattern.Pattern.String())
	}
	if info.Confidence != types.ConfidenceHigh {
		t.Errorf("expected high confidence, got: %s", info.Confidence.String())
	}
}

func TestSideEffectAnalyzer_ParameterWrite(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// Simulate: func modify(slice []int) { slice[0] = 42 }
	sa.BeginFunction("modify", "test.go", 1, 3)
	sa.AddParameter("slice", 0)

	// Write to parameter
	sa.RecordAccess("slice", []string{"0"}, types.AccessWrite, 2, 5)

	info := sa.EndFunction()

	if info.IsPure {
		t.Error("expected impure function (parameter write)")
	}
	if info.Categories&types.SideEffectParamWrite == 0 {
		t.Errorf("expected SideEffectParamWrite, got: %s", info.Categories.String())
	}
	if len(info.ParameterWrites) != 1 {
		t.Errorf("expected 1 parameter write, got %d", len(info.ParameterWrites))
	}
	if info.ParameterWrites[0].ParameterName != "slice" {
		t.Errorf("expected parameter name 'slice', got '%s'", info.ParameterWrites[0].ParameterName)
	}
	if info.AccessPattern.ParameterWrites != 1 {
		t.Errorf("expected 1 parameter write in pattern, got %d", info.AccessPattern.ParameterWrites)
	}
}

func TestSideEffectAnalyzer_GlobalWrite(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// Simulate: func increment() { counter++ }
	// where counter is a global variable
	sa.BeginFunction("increment", "test.go", 1, 3)

	// Write to global (not a parameter or local)
	sa.RecordAccess("counter", nil, types.AccessWrite, 2, 5)

	info := sa.EndFunction()

	if info.IsPure {
		t.Error("expected impure function (global write)")
	}
	if info.Categories&types.SideEffectGlobalWrite == 0 {
		t.Errorf("expected SideEffectGlobalWrite, got: %s", info.Categories.String())
	}
	if info.AccessPattern.GlobalWrites != 1 {
		t.Errorf("expected 1 global write in pattern, got %d", info.AccessPattern.GlobalWrites)
	}
}

func TestSideEffectAnalyzer_ReceiverWrite(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// Simulate: func (u *User) SetName(name string) { u.Name = name }
	sa.BeginFunction("SetName", "test.go", 1, 3)
	sa.SetReceiver("u", "*User")
	sa.AddParameter("name", 0)

	// Read parameter
	sa.RecordAccess("name", nil, types.AccessRead, 2, 20)
	// Write to receiver field
	sa.RecordAccess("u", []string{"Name"}, types.AccessWrite, 2, 5)

	info := sa.EndFunction()

	if info.IsPure {
		t.Error("expected impure function (receiver write)")
	}
	if info.Categories&types.SideEffectReceiverWrite == 0 {
		t.Errorf("expected SideEffectReceiverWrite, got: %s", info.Categories.String())
	}
	if info.AccessPattern.ReceiverWrites != 1 {
		t.Errorf("expected 1 receiver write in pattern, got %d", info.AccessPattern.ReceiverWrites)
	}
}

func TestSideEffectAnalyzer_ClosureWrite(t *testing.T) {
	sa := NewSideEffectAnalyzer("javascript", nil)

	// Simulate a closure that writes to captured variable
	// In real usage, the outer function would add the variable to outer scope,
	// then the inner function starts fresh but has access to outer scope.
	// Here we simulate by: BeginFunction for inner, add outer scope manually, then access.
	sa.BeginFunction("innerClosure", "test.js", 4, 8)

	// Simulate that "count" was declared in enclosing scope
	// (In real integration, this would come from analyzing the enclosing function)
	sa.currentFunc.OuterScopes = append(sa.currentFunc.OuterScopes, map[string]int{
		"count": 2, // count was declared on line 2 in outer scope
	})

	// Inner function writes to outer variable (closure capture)
	sa.RecordAccess("count", nil, types.AccessWrite, 5, 10)

	info := sa.EndFunction()

	if info.IsPure {
		t.Error("expected impure function (closure write)")
	}
	if info.Categories&types.SideEffectClosureWrite == 0 {
		t.Errorf("expected SideEffectClosureWrite, got: %s", info.Categories.String())
	}
	if info.AccessPattern.ClosureWrites != 1 {
		t.Errorf("expected 1 closure write in pattern, got %d", info.AccessPattern.ClosureWrites)
	}
}

func TestSideEffectAnalyzer_ExternalCall(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("doSomething", "test.go", 1, 5)

	// Call to unknown function
	sa.RecordFunctionCall("unknownFunc", "somePackage", false, 2, 5)

	info := sa.EndFunction()

	// Two-phase purity: unknown functions go into UnresolvedCalls, not ExternalCalls
	// Phase 1: Function is internally pure but has unresolved calls -> Level 2 (InternallyPure)
	if info.IsPure {
		t.Error("expected not pure (has unresolved calls)")
	}
	if info.PurityLevel != types.PurityLevelInternallyPure {
		t.Errorf("expected PurityLevelInternallyPure, got: %s", info.PurityLevel.String())
	}
	if len(info.UnresolvedCalls) != 1 {
		t.Errorf("expected 1 unresolved call, got %d", len(info.UnresolvedCalls))
	}
	// Should NOT be marked as external call (that's for true I/O operations)
	if info.Categories&types.SideEffectExternalCall != 0 {
		t.Errorf("unexpected SideEffectExternalCall, got: %s", info.Categories.String())
	}
}

func TestSideEffectAnalyzer_KnownPureCall(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("processString", "test.go", 1, 5)
	sa.AddParameter("s", 0)

	// Read parameter
	sa.RecordAccess("s", nil, types.AccessRead, 2, 20)
	// Call to known pure function
	sa.RecordFunctionCall("ToLower", "strings", false, 2, 10)

	info := sa.EndFunction()

	if !info.IsPure {
		t.Errorf("expected pure function (only calls known pure functions), got impure: %v", info.ImpurityReasons)
	}
	if len(info.ExternalCalls) != 0 {
		t.Errorf("expected 0 external calls for known pure function, got %d", len(info.ExternalCalls))
	}
}

func TestSideEffectAnalyzer_KnownIOCall(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("printMessage", "test.go", 1, 5)
	sa.AddParameter("msg", 0)

	// Call to known IO function
	sa.RecordFunctionCall("Println", "fmt", false, 2, 5)

	info := sa.EndFunction()

	if info.IsPure {
		t.Error("expected impure function (calls I/O function)")
	}
	if info.Categories&types.SideEffectIO == 0 {
		t.Errorf("expected SideEffectIO, got: %s", info.Categories.String())
	}
}

func TestSideEffectAnalyzer_Throw(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("mustSucceed", "test.go", 1, 5)
	sa.RecordThrow("panic", 3, 5)

	info := sa.EndFunction()

	if info.Categories&types.SideEffectThrow == 0 {
		t.Errorf("expected SideEffectThrow, got: %s", info.Categories.String())
	}
	if len(info.ThrowSites) != 1 {
		t.Errorf("expected 1 throw site, got %d", len(info.ThrowSites))
	}
	if !info.ErrorHandling.CanThrow {
		t.Error("expected CanThrow to be true")
	}
}

func TestSideEffectAnalyzer_DynamicCall(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("callInterface", "test.go", 1, 5)
	sa.RecordDynamicCall("interface method call", 2, 5)

	info := sa.EndFunction()

	if info.IsPure {
		t.Error("expected impure function (dynamic call)")
	}
	if info.Categories&types.SideEffectDynamicCall == 0 {
		t.Errorf("expected SideEffectDynamicCall, got: %s", info.Categories.String())
	}
}

func TestSideEffectAnalyzer_Defer(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("safeClose", "test.go", 1, 5)
	sa.RecordDefer()
	sa.RecordDefer()

	info := sa.EndFunction()

	if info.ErrorHandling.DeferCount != 2 {
		t.Errorf("expected 2 defers, got %d", info.ErrorHandling.DeferCount)
	}
	if !info.ErrorHandling.ExceptionSafe {
		t.Error("expected ExceptionSafe to be true with defers")
	}
}

func TestSideEffectAnalyzer_ChannelOp(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("sendMessage", "test.go", 1, 5)
	sa.RecordChannelOp(2)

	info := sa.EndFunction()

	if info.Categories&types.SideEffectChannel == 0 {
		t.Errorf("expected SideEffectChannel, got: %s", info.Categories.String())
	}
}

// Access Pattern Tests

func TestAccessPattern_ReadThenWrite(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// func transform(user *User) { name := user.Name; user.Name = name + "!" }
	sa.BeginFunction("transform", "test.go", 1, 5)
	sa.AddParameter("user", 0)

	// Read first
	sa.RecordAccess("user", []string{"Name"}, types.AccessRead, 2, 15)
	// Then write
	sa.RecordAccess("user", []string{"Name"}, types.AccessWrite, 3, 5)

	info := sa.EndFunction()

	if info.AccessPattern.Pattern != types.PatternReadThenWrite {
		t.Errorf("expected ReadThenWrite pattern, got: %s", info.AccessPattern.Pattern.String())
	}
	if info.AccessPattern.Pattern.IsClean() == false {
		t.Error("expected ReadThenWrite to be considered clean")
	}
}

func TestAccessPattern_WriteThenRead(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// func suspicious(data *Data) { data.Value = 0; total := data.Value + data.Other }
	sa.BeginFunction("suspicious", "test.go", 1, 5)
	sa.AddParameter("data", 0)

	// Write first
	sa.RecordAccess("data", []string{"Value"}, types.AccessWrite, 2, 5)
	// Then read same target
	sa.RecordAccess("data", []string{"Value"}, types.AccessRead, 3, 15)

	info := sa.EndFunction()

	targetPattern := info.AccessPattern.TargetPatterns["param:data.Value"]
	if targetPattern == nil {
		t.Fatal("expected target pattern for param:data.Value")
	}
	if targetPattern.Pattern != types.PatternWriteThenRead {
		t.Errorf("expected WriteThenRead pattern for target, got: %s", targetPattern.Pattern.String())
	}
	if targetPattern.Sequence != "WR" {
		t.Errorf("expected sequence 'WR', got '%s'", targetPattern.Sequence)
	}

	// Should have a violation
	hasViolation := false
	for _, v := range info.AccessPattern.Violations {
		if v.Type == types.ViolationWriteBeforeRead {
			hasViolation = true
			break
		}
	}
	if !hasViolation {
		t.Error("expected ViolationWriteBeforeRead")
	}
}

func TestAccessPattern_Interleaved(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// func interleaved(state *State) { a := state.X; state.Y = a; b := state.Z; state.W = b }
	// But on same target: state.X read, write, read
	sa.BeginFunction("interleaved", "test.go", 1, 5)
	sa.AddParameter("state", 0)

	sa.RecordAccess("state", []string{"X"}, types.AccessRead, 2, 10)
	sa.RecordAccess("state", []string{"X"}, types.AccessWrite, 3, 5)
	sa.RecordAccess("state", []string{"X"}, types.AccessRead, 4, 10)

	info := sa.EndFunction()

	targetPattern := info.AccessPattern.TargetPatterns["param:state.X"]
	if targetPattern == nil {
		t.Fatal("expected target pattern for param:state.X")
	}
	if targetPattern.Pattern != types.PatternInterleaved {
		t.Errorf("expected Interleaved pattern, got: %s", targetPattern.Pattern.String())
	}
	if targetPattern.Sequence != "RWR" {
		t.Errorf("expected sequence 'RWR', got '%s'", targetPattern.Sequence)
	}

	// Should have a violation
	hasViolation := false
	for _, v := range info.AccessPattern.Violations {
		if v.Type == types.ViolationInterleavedAccess {
			hasViolation = true
			break
		}
	}
	if !hasViolation {
		t.Error("expected ViolationInterleavedAccess")
	}
}

func TestAccessPattern_WriteOnly(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// func init(obj *Object) { obj.A = 1; obj.B = 2; obj.C = 3 }
	sa.BeginFunction("init", "test.go", 1, 5)
	sa.AddParameter("obj", 0)

	sa.RecordAccess("obj", []string{"A"}, types.AccessWrite, 2, 5)
	sa.RecordAccess("obj", []string{"B"}, types.AccessWrite, 3, 5)
	sa.RecordAccess("obj", []string{"C"}, types.AccessWrite, 4, 5)

	info := sa.EndFunction()

	if info.AccessPattern.Pattern != types.PatternWriteOnly {
		t.Errorf("expected WriteOnly pattern, got: %s", info.AccessPattern.Pattern.String())
	}
	if info.AccessPattern.TotalWrites != 3 {
		t.Errorf("expected 3 total writes, got %d", info.AccessPattern.TotalWrites)
	}
	if info.AccessPattern.TotalReads != 0 {
		t.Errorf("expected 0 total reads, got %d", info.AccessPattern.TotalReads)
	}
}

func TestAccessPattern_MutateParameter_Violation(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("mutate", "test.go", 1, 3)
	sa.AddParameter("data", 0)

	sa.RecordAccess("data", []string{"Field"}, types.AccessWrite, 2, 5)

	info := sa.EndFunction()

	// Should have MutateParameter violation
	hasViolation := false
	for _, v := range info.AccessPattern.Violations {
		if v.Type == types.ViolationMutateParameter {
			hasViolation = true
			if v.Severity < 0.8 {
				t.Errorf("expected high severity for parameter mutation, got %f", v.Severity)
			}
			break
		}
	}
	if !hasViolation {
		t.Error("expected ViolationMutateParameter")
	}
}

// Confidence Tests

func TestConfidence_HighForPure(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("pureFunc", "test.go", 1, 3)
	sa.AddParameter("x", 0)
	sa.RecordAccess("x", nil, types.AccessRead, 2, 5)

	info := sa.EndFunction()

	if info.Confidence != types.ConfidenceHigh {
		t.Errorf("expected high confidence for pure function, got: %s", info.Confidence.String())
	}
}

func TestConfidence_LowForManyExternalCalls(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	sa.BeginFunction("manyUnknown", "test.go", 1, 20)

	// Many unknown calls
	for i := 0; i < 10; i++ {
		sa.RecordFunctionCall("unknown", "pkg", false, i+2, 5)
	}

	info := sa.EndFunction()

	// Two-phase purity: unresolved calls don't lower confidence
	// They're recorded for Phase 2 resolution
	// Confidence should be high because we successfully analyzed the function
	if info.Confidence != types.ConfidenceHigh {
		t.Errorf("expected high confidence (unresolved calls don't affect Phase 1 confidence), got: %s", info.Confidence.String())
	}
	if len(info.UnresolvedCalls) != 10 {
		t.Errorf("expected 10 unresolved calls, got %d", len(info.UnresolvedCalls))
	}
}

// Known Functions Tests

func TestKnownPureFunctions(t *testing.T) {
	testCases := []struct {
		language string
		function string
		isPure   bool
	}{
		{"go", "strings.ToLower", true},
		{"go", "math.Sqrt", true},
		{"go", "len", true},
		{"go", "fmt.Println", false},
		{"go", "os.ReadFile", false},
		{"javascript", "Math.abs", true},
		{"javascript", "console.log", false},
		{"javascript", "Array.prototype.push", false},
		{"python", "len", true},
		{"python", "print", false},
	}

	for _, tc := range testCases {
		isPure, conf := CheckFunctionPurity(tc.language, tc.function)
		if tc.isPure {
			if !isPure {
				t.Errorf("%s:%s should be pure", tc.language, tc.function)
			}
			if conf != types.ConfidenceProven {
				t.Errorf("%s:%s should have proven confidence", tc.language, tc.function)
			}
		} else {
			if isPure && conf == types.ConfidenceProven {
				t.Errorf("%s:%s should not be proven pure", tc.language, tc.function)
			}
		}
	}
}

func TestGetKnownSideEffects(t *testing.T) {
	testCases := []struct {
		language string
		function string
		expected types.SideEffectCategory
	}{
		{"go", "strings.ToLower", types.SideEffectNone},
		{"go", "fmt.Println", types.SideEffectIO},
		{"go", "os.ReadFile", types.SideEffectIO},
		{"go", "http.Get", types.SideEffectNetwork},
		{"go", "panic", types.SideEffectThrow},
		{"javascript", "console.log", types.SideEffectIO},
		{"javascript", "fetch", types.SideEffectNetwork | types.SideEffectAsync},
		{"javascript", "Array.prototype.push", types.SideEffectParamWrite},
	}

	for _, tc := range testCases {
		effects := GetKnownSideEffects(tc.language, tc.function)
		if effects != tc.expected && tc.expected != types.SideEffectUncertain {
			t.Errorf("%s:%s: expected %s, got %s", tc.language, tc.function, tc.expected.String(), effects.String())
		}
	}
}

// SideEffectCategory Tests

func TestSideEffectCategory_HasWriteEffects(t *testing.T) {
	testCases := []struct {
		category types.SideEffectCategory
		expected bool
	}{
		{types.SideEffectNone, false},
		{types.SideEffectParamWrite, true},
		{types.SideEffectReceiverWrite, true},
		{types.SideEffectGlobalWrite, true},
		{types.SideEffectClosureWrite, true},
		{types.SideEffectIO, false},
		{types.SideEffectThrow, false},
		{types.SideEffectParamWrite | types.SideEffectIO, true},
	}

	for _, tc := range testCases {
		result := tc.category.HasWriteEffects()
		if result != tc.expected {
			t.Errorf("%s.HasWriteEffects(): expected %v, got %v", tc.category.String(), tc.expected, result)
		}
	}
}

func TestSideEffectCategory_HasUncertainty(t *testing.T) {
	testCases := []struct {
		category types.SideEffectCategory
		expected bool
	}{
		{types.SideEffectNone, false},
		{types.SideEffectParamWrite, false},
		{types.SideEffectExternalCall, true},
		{types.SideEffectDynamicCall, true},
		{types.SideEffectReflection, true},
		{types.SideEffectUncertain, true},
		{types.SideEffectParamWrite | types.SideEffectUncertain, true},
	}

	for _, tc := range testCases {
		result := tc.category.HasUncertainty()
		if result != tc.expected {
			t.Errorf("%s.HasUncertainty(): expected %v, got %v", tc.category.String(), tc.expected, result)
		}
	}
}

func TestSideEffectCategory_IsPure(t *testing.T) {
	testCases := []struct {
		category types.SideEffectCategory
		expected bool
	}{
		{types.SideEffectNone, true},
		{types.SideEffectParamWrite, false},
		{types.SideEffectIO, false},
		{types.SideEffectExternalCall, false},
	}

	for _, tc := range testCases {
		result := tc.category.IsPure()
		if result != tc.expected {
			t.Errorf("%s.IsPure(): expected %v, got %v", tc.category.String(), tc.expected, result)
		}
	}
}

// Integration test: Full function analysis workflow
func TestFullAnalysisWorkflow(t *testing.T) {
	sa := NewSideEffectAnalyzer("go", nil)

	// Analyze a realistic function:
	// func ProcessUser(user *User, logger *Logger) error {
	//     name := user.Name          // read param
	//     logger.Info("processing")  // external call (unknown)
	//     user.Status = "processed"  // write param
	//     return nil
	// }
	sa.BeginFunction("ProcessUser", "handler.go", 10, 16)
	sa.AddParameter("user", 0)
	sa.AddParameter("logger", 1)

	// Read param
	sa.RecordAccess("user", []string{"Name"}, types.AccessRead, 11, 15)
	// External call
	sa.RecordFunctionCall("Info", "logger", true, 12, 5)
	// Write param
	sa.RecordAccess("user", []string{"Status"}, types.AccessWrite, 13, 5)

	info := sa.EndFunction()

	// Should be impure (param write)
	if info.IsPure {
		t.Error("expected impure function")
	}

	// Should have param write (but NOT external call - logger.Info is unresolved, not external)
	if info.Categories&types.SideEffectParamWrite == 0 {
		t.Error("expected SideEffectParamWrite")
	}
	// Two-phase purity: unknown methods go into UnresolvedCalls, not ExternalCalls
	if info.Categories&types.SideEffectExternalCall != 0 {
		t.Error("unexpected SideEffectExternalCall (logger.Info is unresolved, not external)")
	}

	// Check access pattern
	if info.AccessPattern == nil {
		t.Fatal("expected access pattern")
	}
	if info.AccessPattern.ParameterWrites != 1 {
		t.Errorf("expected 1 parameter write, got %d", info.AccessPattern.ParameterWrites)
	}

	// Should have unresolved call recorded (not external call)
	if len(info.UnresolvedCalls) != 1 {
		t.Errorf("expected 1 unresolved call, got %d", len(info.UnresolvedCalls))
	}
	if len(info.ExternalCalls) != 0 {
		t.Errorf("expected 0 external calls (logger.Info is unresolved), got %d", len(info.ExternalCalls))
	}

	// Should have impurity reason for param write only
	// (unresolved calls don't add impurity reasons in Phase 1)
	if len(info.ImpurityReasons) < 1 {
		t.Errorf("expected at least 1 impurity reason, got %d: %v", len(info.ImpurityReasons), info.ImpurityReasons)
	}
}
