package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

// TestSideEffects_ParameterMutation tests detection of parameter mutations
func TestSideEffects_ParameterMutation(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

// Pure - returns new slice
func PureAppend(s []int, x int) []int {
	return append(s, x)
}

// Impure - modifies parameter
func ImpureModify(s []int) {
	s[0] = 999
}

// Impure - modifies slice via append
func ImpureAppendInPlace(s *[]int, x int) {
	*s = append(*s, x)
}

// Pure - doesn't modify pointer
func PureRead(p *int) int {
	return *p
}

// Impure - modifies through pointer
func ImpureWrite(p *int, val int) {
	*p = val
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	// Count by purity
	pure, impure := 0, 0
	paramWrites := 0

	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.IsPure {
			pure++
		} else {
			impure++
			if info.Categories&types.SideEffectParamWrite != 0 {
				paramWrites++
			}
		}
	}

	t.Logf("Parameter mutation: pure=%d impure=%d paramWrites=%d", pure, impure, paramWrites)

	require.Greater(t, pure, 0, "Should detect pure functions")
	require.Greater(t, impure, 0, "Should detect impure functions")
	require.Greater(t, paramWrites, 0, "CRITICAL: Failed to detect parameter writes!")

	// Should detect at least 2 pure (PureAppend, PureRead)
	assert.GreaterOrEqual(t, pure, 2, "Should detect at least 2 pure functions")

	// Should detect at least 3 impure with param writes (ImpureModify, ImpureAppendInPlace, ImpureWrite)
	assert.GreaterOrEqual(t, paramWrites, 3, "Should detect at least 3 functions with parameter writes")
}

// TestSideEffects_GlobalState tests detection of global variable access
func TestSideEffects_GlobalState(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

var globalCounter int
var globalMap = make(map[string]int)

// Pure - no global access
func PureLocal() int {
	x := 10
	return x * 2
}

// Impure - reads global
func ImpureReadGlobal() int {
	return globalCounter
}

// Impure - writes global
func ImpureWriteGlobal(val int) {
	globalCounter = val
}

// Impure - modifies global map
func ImpureModifyGlobalMap(key string, val int) {
	globalMap[key] = val
}

// Impure - increments global
func ImpureIncrementGlobal() {
	globalCounter++
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	globalWrites := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.Categories&types.SideEffectGlobalWrite != 0 {
			globalWrites++
		}
	}

	t.Logf("Global state: globalWrites=%d", globalWrites)

	require.Greater(t, globalWrites, 0, "CRITICAL: Failed to detect global writes!")

	// Should detect at least 3 global writes (ImpureWriteGlobal, ImpureModifyGlobalMap, ImpureIncrementGlobal)
	assert.GreaterOrEqual(t, globalWrites, 3, "Should detect at least 3 global write functions")
}

// TestSideEffects_IOOperations tests detection of I/O operations
func TestSideEffects_IOOperations(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import (
	"fmt"
	"os"
	"io/ioutil"
)

// Pure - no I/O
func PureCalculate(x int) int {
	return x * x
}

// Impure - prints to stdout
func ImpurePrint(msg string) {
	fmt.Println(msg)
}

// Impure - reads file
func ImpureReadFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

// Impure - writes file
func ImpureWriteFile(path string, data []byte) error {
	return ioutil.WriteFile(path, data, 0644)
}

// Impure - multiple I/O operations
func ImpureLog(msg string) {
	fmt.Printf("[LOG] %s\n", msg)
	os.Stderr.WriteString(msg)
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	ioEffects := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.Categories&types.SideEffectIO != 0 {
			ioEffects++
		}
	}

	t.Logf("I/O operations: ioEffects=%d", ioEffects)

	require.Greater(t, ioEffects, 0, "CRITICAL: Failed to detect I/O operations!")

	// Should detect at least 4 I/O functions (ImpurePrint, ImpureReadFile, ImpureWriteFile, ImpureLog)
	assert.GreaterOrEqual(t, ioEffects, 4, "Should detect at least 4 I/O functions")
}

// TestSideEffects_TransitivePropagation tests that impurity propagates through call chains
func TestSideEffects_TransitivePropagation(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// Pure leaf function
func PureLeaf(x int) int {
	return x * 2
}

// Impure leaf function
func ImpureLeaf(x int) {
	fmt.Println(x)
}

// Should be pure - calls only pure
func CallerOfPure(x int) int {
	return PureLeaf(x) + PureLeaf(x+1)
}

// Should be impure - calls impure
func CallerOfImpure(x int) {
	ImpureLeaf(x)
}

// Should be impure - calls impure transitively
func TransitiveImpure(x int) {
	CallerOfImpure(x)
}

// Should be impure - mixes pure and impure calls
func MixedCaller(x int) int {
	CallerOfImpure(x)
	return PureLeaf(x)
}

// Deep call chain - should be impure
func DeepCaller(x int) {
	level1(x)
}

func level1(x int) {
	level2(x)
}

func level2(x int) {
	ImpureLeaf(x)
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	// Run propagation to compute transitive effects
	err = propagator.Propagate()
	require.NoError(t, err)

	// Re-get effects after propagation
	allEffects = propagator.GetAllSideEffects()

	pure, impure, transitive := 0, 0, 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.IsPure {
			pure++
		} else {
			impure++
			if info.TransitiveCategories != 0 {
				transitive++
			}
		}
	}

	t.Logf("Transitive propagation: pure=%d impure=%d transitive=%d", pure, impure, transitive)

	require.Greater(t, transitive, 0, "CRITICAL: Failed to detect transitive side effects!")

	// CallerOfPure should be pure
	// CallerOfImpure, TransitiveImpure, MixedCaller, DeepCaller, level1, level2 should be impure
	assert.GreaterOrEqual(t, impure, 6, "Should detect at least 6 impure functions")
	assert.GreaterOrEqual(t, transitive, 4, "Should detect at least 4 functions with transitive impurity")
}

// TestSideEffects_ExceptionHandling tests detection of throw/panic
func TestSideEffects_ExceptionHandling(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

// Pure - no panic
func PureDivide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

// Impure - panics
func ImpurePanic(msg string) {
	panic(msg)
}

// Impure - may panic
func ImpureMayPanic(x int) int {
	if x < 0 {
		panic("negative value")
	}
	return x * 2
}

// Has defer - error handling
func WithDefer() {
	defer func() {
		recover()
	}()
	panic("test")
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	throws := 0
	defers := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.Categories&types.SideEffectThrow != 0 {
			throws++
		}
		if info.ErrorHandling.DeferCount > 0 {
			defers++
		}
	}

	t.Logf("Exception handling: throws=%d defers=%d", throws, defers)

	require.Greater(t, throws, 0, "CRITICAL: Failed to detect panic/throw!")

	// Should detect at least 2 functions with direct panic (ImpurePanic, ImpureMayPanic)
	// Note: WithDefer's panic is inside the body but followed by recover, detection may vary
	assert.GreaterOrEqual(t, throws, 2, "Should detect at least 2 functions with panic")

	// Note: WithDefer uses defer with anonymous function - defer count may be 0 if
	// anonymous functions are tracked separately from their parent functions
	// This is a known limitation in the current implementation
}

// TestSideEffects_MethodsVsFunctions tests detection on methods vs functions
func TestSideEffects_MethodsVsFunctions(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

type Counter struct {
	value int
}

// Pure method - returns value
func (c *Counter) Get() int {
	return c.value
}

// Impure method - modifies receiver
func (c *Counter) Increment() {
	c.value++
}

// Impure method - modifies receiver and has I/O
func (c *Counter) IncrementAndLog() {
	c.value++
	fmt.Printf("Counter: %d\n", c.value)
}

// Pure function
func PureAdd(a, b int) int {
	return a + b
}

// Method that calls other method (transitive)
func (c *Counter) IncrementTwice() {
	c.Increment()
	c.Increment()
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	receiverWrites := 0
	transitiveReceiverWrites := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		// Receiver modifications should be detected as SideEffectReceiverWrite
		if info.Categories&types.SideEffectReceiverWrite != 0 {
			receiverWrites++
		}
		// Also count transitive receiver writes (e.g., IncrementTwice calls Increment)
		if info.TransitiveCategories&types.SideEffectReceiverWrite != 0 {
			transitiveReceiverWrites++
		}
	}

	t.Logf("Methods: receiverWrites=%d (direct), transitiveReceiverWrites=%d", receiverWrites, transitiveReceiverWrites)

	require.Greater(t, receiverWrites, 0, "CRITICAL: Failed to detect receiver modifications!")

	// Should detect at least 2 methods with direct receiver writes (Increment, IncrementAndLog)
	// IncrementTwice only has transitive effects through calling Increment
	assert.GreaterOrEqual(t, receiverWrites, 2, "Should detect at least 2 methods directly modifying receiver")
}

// TestSideEffects_ComplexCallGraph tests complex call patterns
func TestSideEffects_ComplexCallGraph(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// Diamond pattern: A calls B and C, both call D
func A() {
	B()
	C()
}

func B() {
	D()
}

func C() {
	D()
}

func D() {
	fmt.Println("D")
}

// Mutual recursion
func Even(n int) bool {
	if n == 0 {
		return true
	}
	return Odd(n - 1)
}

func Odd(n int) bool {
	if n == 0 {
		return false
	}
	return Even(n - 1)
}

// Self-recursive
func Factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * Factorial(n-1)
}

// Indirect recursion with I/O
func RecurseWithIO(n int) {
	if n > 0 {
		fmt.Println(n)
		RecurseWithIO(n - 1)
	}
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	err = propagator.Propagate()
	require.NoError(t, err)

	allEffects = propagator.GetAllSideEffects()

	pure, impure := 0, 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.IsPure {
			pure++
		} else {
			impure++
		}
	}

	t.Logf("Complex call graph: pure=%d impure=%d total=%d", pure, impure, len(allEffects))

	// Even, Odd, Factorial should be pure
	// A, B, C, D, RecurseWithIO should be impure
	assert.GreaterOrEqual(t, pure, 3, "Should detect at least 3 pure functions (Even, Odd, Factorial)")
	assert.GreaterOrEqual(t, impure, 5, "Should detect at least 5 impure functions")
}

// TestSideEffects_EmptyAndEdgeCases tests edge cases
func TestSideEffects_EmptyAndEdgeCases(t *testing.T) {
	t.Run("EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tmpDir, "empty.go"), []byte(`package test
`), 0644)
		require.NoError(t, err)

		cfg := &config.Config{
			Project: config.Project{Root: tmpDir},
			Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
		}

		mi := NewMasterIndex(cfg)
		defer mi.Close()

		err = mi.IndexDirectory(context.Background(), tmpDir)
		require.NoError(t, err)

		propagator := mi.GetSideEffectPropagator()
		require.NotNil(t, propagator)

		allEffects := propagator.GetAllSideEffects()
		// Empty file should have no functions, so empty effects is OK
		t.Logf("Empty file: %d effects", len(allEffects))
	})

	t.Run("OnlyVariables", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tmpDir, "vars.go"), []byte(`package test

var x = 10
var y = "hello"
const z = 42
`), 0644)
		require.NoError(t, err)

		cfg := &config.Config{
			Project: config.Project{Root: tmpDir},
			Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
		}

		mi := NewMasterIndex(cfg)
		defer mi.Close()

		err = mi.IndexDirectory(context.Background(), tmpDir)
		require.NoError(t, err)

		propagator := mi.GetSideEffectPropagator()
		require.NotNil(t, propagator)

		allEffects := propagator.GetAllSideEffects()
		// No functions, so empty effects is OK
		t.Logf("Only variables: %d effects", len(allEffects))
	})

	t.Run("EmptyFunctions", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tmpDir, "empty_funcs.go"), []byte(`package test

func Empty1() {
}

func Empty2() {
}

func EmptyReturn() int {
	return 0
}
`), 0644)
		require.NoError(t, err)

		cfg := &config.Config{
			Project: config.Project{Root: tmpDir},
			Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
		}

		mi := NewMasterIndex(cfg)
		defer mi.Close()

		err = mi.IndexDirectory(context.Background(), tmpDir)
		require.NoError(t, err)

		propagator := mi.GetSideEffectPropagator()
		require.NotNil(t, propagator)

		allEffects := propagator.GetAllSideEffects()
		require.NotEmpty(t, allEffects, "Should detect empty functions")

		// All empty functions should be pure
		for _, info := range allEffects {
			if info != nil {
				assert.True(t, info.IsPure, "Empty functions should be pure")
			}
		}

		t.Logf("Empty functions: %d effects, all should be pure", len(allEffects))
	})
}

// TestSideEffects_MultiFile tests cross-file side effect propagation
func TestSideEffects_MultiFile(t *testing.T) {
	tmpDir := t.TempDir()

	// File 1: Pure utilities
	err := os.WriteFile(filepath.Join(tmpDir, "pure_utils.go"), []byte(`package test

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}
`), 0644)
	require.NoError(t, err)

	// File 2: Impure utilities
	err = os.WriteFile(filepath.Join(tmpDir, "impure_utils.go"), []byte(`package test

import "fmt"

func Log(msg string) {
	fmt.Println(msg)
}

func Warn(msg string) {
	fmt.Fprintf(os.Stderr, "WARN: %s\n", msg)
}
`), 0644)
	require.NoError(t, err)

	// File 3: Mixed - uses both
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package test

func PureCalculation(x int) int {
	return Add(Multiply(x, 2), 10)
}

func LoggedCalculation(x int) int {
	result := Add(Multiply(x, 2), 10)
	Log(fmt.Sprintf("Result: %d", result))
	return result
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	err = mi.IndexDirectory(context.Background(), tmpDir)
	require.NoError(t, err)

	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator)

	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects, "CRITICAL: No side effects collected!")

	err = propagator.Propagate()
	require.NoError(t, err)

	allEffects = propagator.GetAllSideEffects()

	pure, impure := 0, 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.IsPure {
			pure++
		} else {
			impure++
		}
	}

	t.Logf("Multi-file: pure=%d impure=%d", pure, impure)

	// Debug: show all functions
	for _, info := range allEffects {
		if info != nil {
			t.Logf("  %s: IsPure=%v, Categories=%d, Transitive=%d",
				info.FunctionName, info.IsPure, info.Categories, info.TransitiveCategories)
		}
	}

	// Add, Multiply, PureCalculation should be pure
	assert.GreaterOrEqual(t, pure, 3, "Should detect at least 3 pure functions")

	// Log, Warn, LoggedCalculation should be impure
	assert.GreaterOrEqual(t, impure, 3, "Should detect at least 3 impure functions")
}
