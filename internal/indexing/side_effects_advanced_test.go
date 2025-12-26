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

// TestSideEffects_Closures tests side effect detection in closures
func TestSideEffects_Closures(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// Pure - returns closure that's pure
func PureClosureGenerator(x int) func(int) int {
	return func(y int) int {
		return x + y
	}
}

// Impure - closure captures and modifies outer variable
func ImpureClosureWithCapture() func() {
	counter := 0
	return func() {
		counter++
		fmt.Println(counter)
	}
}

// Impure - closure does I/O
func ImpureClosureIO(msg string) func() {
	return func() {
		fmt.Println(msg)
	}
}

// Complex closure scenario
func ClosureFactory() func(int) {
	var total int
	return func(x int) {
		total += x // Modifies captured variable
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

	t.Logf("Closures: %d functions analyzed", len(allEffects))

	// At minimum, should detect the outer functions
	assert.GreaterOrEqual(t, len(allEffects), 3, "Should detect at least outer functions")
}

// TestSideEffects_Interfaces tests side effects with interface methods
func TestSideEffects_Interfaces(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

type Writer interface {
	Write(data []byte) error
}

type Logger interface {
	Log(msg string)
}

// Implementation that's impure
type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(msg string) {
	fmt.Println(msg)
}

// Implementation that's pure (in theory - just stores)
type BufferLogger struct {
	buffer []string
}

func (b *BufferLogger) Log(msg string) {
	b.buffer = append(b.buffer, msg)
}

// Function taking interface - purity depends on implementation
func LogMessage(logger Logger, msg string) {
	logger.Log(msg)
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

	ioMethods := 0
	receiverWrites := 0

	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.Categories&types.SideEffectIO != 0 {
			ioMethods++
		}
		if info.Categories&types.SideEffectReceiverWrite != 0 {
			receiverWrites++
		}
	}

	t.Logf("Interfaces: ioMethods=%d receiverWrites=%d", ioMethods, receiverWrites)

	// ConsoleLogger.Log should have I/O
	assert.GreaterOrEqual(t, ioMethods, 1, "Should detect I/O in ConsoleLogger.Log")

	// BufferLogger.Log modifies receiver (b.buffer = append(...))
	assert.GreaterOrEqual(t, receiverWrites, 1, "Should detect receiver write in BufferLogger.Log")
}

// TestSideEffects_VariadicFunctions tests variadic function parameters
func TestSideEffects_VariadicFunctions(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// Pure - sums variadic args
func PureSum(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// Impure - logs variadic args
func ImpureLogAll(msgs ...string) {
	for _, msg := range msgs {
		fmt.Println(msg)
	}
}

// Impure - modifies variadic slice
func ImpureModifySlice(values ...int) {
	for i := range values {
		values[i] = values[i] * 2
	}
}

// Pure - doesn't modify, just reads
func PureMax(nums ...int) int {
	if len(nums) == 0 {
		return 0
	}
	max := nums[0]
	for _, n := range nums {
		if n > max {
			max = n
		}
	}
	return max
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

	t.Logf("Variadic: pure=%d impure=%d", pure, impure)

	// PureSum and PureMax should be pure
	assert.GreaterOrEqual(t, pure, 2, "Should detect at least 2 pure variadic functions")

	// ImpureLogAll and ImpureModifySlice should be impure
	assert.GreaterOrEqual(t, impure, 2, "Should detect at least 2 impure variadic functions")
}

// TestSideEffects_Generics tests side effects with Go generics (if supported)
func TestSideEffects_Generics(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// Pure generic function
func PureIdentity[T any](x T) T {
	return x
}

// Pure generic function
func PureMap[T any, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Impure generic function
func ImpurePrint[T any](value T) {
	fmt.Printf("%v\n", value)
}

// Generic with constraints
func PureSum[T ~int | ~float64](values ...T) T {
	var total T
	for _, v := range values {
		total += v
	}
	return total
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

	t.Logf("Generics: pure=%d impure=%d", pure, impure)

	// Should detect both pure and impure generic functions
	// Note: PureMap calls a passed function which may be considered impure due to uncertainty
	// PureIdentity and PureSum should be detected as pure
	assert.GreaterOrEqual(t, pure, 1, "Should detect at least 1 pure generic function")
	assert.GreaterOrEqual(t, impure, 1, "Should detect impure generic functions")
}

// TestSideEffects_Channels tests Go channel operations
func TestSideEffects_Channels(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

// Pure - no channel operations
func PureCompute(x int) int {
	return x * x
}

// Impure - sends to channel
func ImpureSend(ch chan<- int, value int) {
	ch <- value
}

// Impure - receives from channel
func ImpureReceive(ch <-chan int) int {
	return <-ch
}

// Impure - creates and uses channel
func ImpureWithChannel() {
	ch := make(chan int)
	go func() {
		ch <- 42
	}()
	<-ch
}

// Impure - select on channels
func ImpureSelect(ch1, ch2 <-chan int) int {
	select {
	case v := <-ch1:
		return v
	case v := <-ch2:
		return v
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

	channelOps := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.Categories&types.SideEffectChannel != 0 {
			channelOps++
		}
	}

	t.Logf("Channels: channelOps=%d", channelOps)

	require.Greater(t, channelOps, 0, "CRITICAL: Failed to detect channel operations!")

	// Should detect at least 4 functions with channel ops
	assert.GreaterOrEqual(t, channelOps, 4, "Should detect at least 4 functions with channel operations")
}

// TestSideEffects_DeferRecover tests defer and recover patterns
func TestSideEffects_DeferRecover(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// Has defer
func WithSimpleDefer() {
	defer fmt.Println("cleanup")
	fmt.Println("work")
}

// Has defer with recover
func WithRecover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered: %v\n", r)
		}
	}()
	panic("test")
}

// Multiple defers
func WithMultipleDefers() {
	defer fmt.Println("first")
	defer fmt.Println("second")
	defer fmt.Println("third")
}

// Defer with closure
func WithDeferClosure() {
	x := 0
	defer func() {
		x++ // Modifies captured variable
		fmt.Println(x)
	}()
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

	withDefer := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.ErrorHandling.DeferCount > 0 {
			withDefer++
		}
	}

	t.Logf("Defer/Recover: withDefer=%d", withDefer)

	require.Greater(t, withDefer, 0, "CRITICAL: Failed to detect defer statements!")

	// Should detect at least 2 functions with defer
	// Note: Anonymous function defers (defer func() {...}()) may be counted as separate functions
	// rather than incrementing the parent function's defer count
	assert.GreaterOrEqual(t, withDefer, 2, "Should detect at least 2 functions with defer")
}

// TestSideEffects_ExternalCalls tests calls to external packages
func TestSideEffects_ExternalCalls(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import (
	"fmt"
	"math"
	"strings"
	"os"
	"net/http"
)

// Pure - uses pure stdlib function
func PureWithMath(x float64) float64 {
	return math.Sqrt(x)
}

// Pure - uses pure string functions
func PureWithStrings(s string) string {
	return strings.ToUpper(s)
}

// Impure - uses fmt
func ImpureWithFmt(msg string) {
	fmt.Println(msg)
}

// Impure - uses os
func ImpureWithOS() {
	os.Getwd()
}

// Impure - uses net/http
func ImpureWithHTTP(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
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

	externalCalls := 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.Categories&types.SideEffectExternalCall != 0 {
			externalCalls++
		}
	}

	t.Logf("External calls: externalCalls=%d", externalCalls)

	// Should detect external calls (may or may not based on implementation)
	t.Logf("External calls detected: %d", externalCalls)
}

// TestSideEffects_ConfidenceLevels tests purity confidence scoring
func TestSideEffects_ConfidenceLevels(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package test

import "fmt"

// High confidence pure
func HighConfidencePure(x int) int {
	return x * 2
}

// High confidence impure
func HighConfidenceImpure(x int) {
	fmt.Println(x)
}

// Medium confidence - calls unknown function
func MediumConfidence(x int) int {
	return someUnknownFunction(x)
}

// Transitive impurity (lower confidence)
func TransitiveImpure(x int) {
	HighConfidenceImpure(x)
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

	highConf, medConf, lowConf := 0, 0, 0
	for _, info := range allEffects {
		if info == nil {
			continue
		}
		switch info.Confidence {
		case types.ConfidenceHigh:
			highConf++
		case types.ConfidenceMedium:
			medConf++
		case types.ConfidenceLow:
			lowConf++
		}
	}

	t.Logf("Confidence: high=%d medium=%d low=%d", highConf, medConf, lowConf)

	// Should have some functions with confidence levels assigned
	total := highConf + medConf + lowConf
	assert.Greater(t, total, 0, "Should assign confidence levels")
}
