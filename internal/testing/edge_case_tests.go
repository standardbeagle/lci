package testing

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EdgeCaseScenario represents a test scenario for edge cases and error paths
type EdgeCaseScenario struct {
	Name        string
	Description string
	SetupFunc   func(t *testing.T) (cleanup func())
	TestFunc    func(t *testing.T)
	ExpectError bool
	ErrorMatch  string // Substring to match in error message
}

// RunEdgeCaseScenarios runs a suite of edge case scenarios
func RunEdgeCaseScenarios(t *testing.T, scenarios []EdgeCaseScenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			if scenario.SetupFunc != nil {
				cleanup := scenario.SetupFunc(t)
				if cleanup != nil {
					defer cleanup()
				}
			}
			scenario.TestFunc(t)
		})
	}
}

// ============================================================================
// File System Edge Cases
// ============================================================================

// FileSystemEdgeCases provides test scenarios for file system edge conditions
type FileSystemEdgeCases struct {
	TempDir string
}

// NewFileSystemEdgeCases creates edge case scenarios in a temp directory
func NewFileSystemEdgeCases(t *testing.T) *FileSystemEdgeCases {
	tempDir := t.TempDir()
	return &FileSystemEdgeCases{TempDir: tempDir}
}

// EmptyFile creates an empty file for testing
func (e *FileSystemEdgeCases) EmptyFile(t *testing.T) string {
	path := filepath.Join(e.TempDir, "empty.go")
	err := os.WriteFile(path, []byte{}, 0644)
	require.NoError(t, err)
	return path
}

// LargeFile creates a file larger than typical limits
func (e *FileSystemEdgeCases) LargeFile(t *testing.T, sizeBytes int) string {
	path := filepath.Join(e.TempDir, "large.go")
	content := bytes.Repeat([]byte("// large content\n"), sizeBytes/17+1)
	err := os.WriteFile(path, content[:sizeBytes], 0644)
	require.NoError(t, err)
	return path
}

// FileWithNullBytes creates a file containing null bytes
func (e *FileSystemEdgeCases) FileWithNullBytes(t *testing.T) string {
	path := filepath.Join(e.TempDir, "nullbytes.go")
	content := []byte("package main\n\x00\x00func main() {}\n")
	err := os.WriteFile(path, content, 0644)
	require.NoError(t, err)
	return path
}

// BinaryFile creates a binary file
func (e *FileSystemEdgeCases) BinaryFile(t *testing.T) string {
	path := filepath.Join(e.TempDir, "binary.bin")
	content := make([]byte, 256)
	for i := range content {
		content[i] = byte(i)
	}
	err := os.WriteFile(path, content, 0644)
	require.NoError(t, err)
	return path
}

// FileWithLongLines creates a file with very long lines
func (e *FileSystemEdgeCases) FileWithLongLines(t *testing.T, lineLength int) string {
	path := filepath.Join(e.TempDir, "longlines.go")
	line := "// " + strings.Repeat("x", lineLength) + "\n"
	content := line + "package main\nfunc main() {}\n"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

// FileWithManyLines creates a file with many lines
func (e *FileSystemEdgeCases) FileWithManyLines(t *testing.T, lineCount int) string {
	path := filepath.Join(e.TempDir, "manylines.go")
	var builder strings.Builder
	builder.WriteString("package main\n\n")
	for i := 0; i < lineCount; i++ {
		builder.WriteString("// line\n")
	}
	builder.WriteString("func main() {}\n")
	err := os.WriteFile(path, []byte(builder.String()), 0644)
	require.NoError(t, err)
	return path
}

// SymbolicLink creates a symlink (if supported by the OS)
func (e *FileSystemEdgeCases) SymbolicLink(t *testing.T, target string) string {
	linkPath := filepath.Join(e.TempDir, "symlink.go")
	err := os.Symlink(target, linkPath)
	if err != nil {
		t.Skip("Symlinks not supported on this system")
	}
	return linkPath
}

// DeepDirectory creates deeply nested directories
func (e *FileSystemEdgeCases) DeepDirectory(t *testing.T, depth int) string {
	parts := make([]string, depth)
	for i := range parts {
		parts[i] = "d"
	}
	dirPath := filepath.Join(e.TempDir, filepath.Join(parts...))
	err := os.MkdirAll(dirPath, 0755)
	require.NoError(t, err)

	filePath := filepath.Join(dirPath, "deep.go")
	err = os.WriteFile(filePath, []byte("package deep\n"), 0644)
	require.NoError(t, err)

	return dirPath
}

// ReadOnlyFile creates a read-only file
func (e *FileSystemEdgeCases) ReadOnlyFile(t *testing.T) string {
	path := filepath.Join(e.TempDir, "readonly.go")
	err := os.WriteFile(path, []byte("package main\n"), 0444)
	require.NoError(t, err)
	return path
}

// ============================================================================
// Unicode and Encoding Edge Cases
// ============================================================================

// UnicodeEdgeCases provides test scenarios for Unicode edge conditions
type UnicodeEdgeCases struct {
	TempDir string
}

// NewUnicodeEdgeCases creates Unicode edge case scenarios
func NewUnicodeEdgeCases(t *testing.T) *UnicodeEdgeCases {
	return &UnicodeEdgeCases{TempDir: t.TempDir()}
}

// GetTestCases returns various Unicode test strings
func (u *UnicodeEdgeCases) GetTestCases() map[string][]byte {
	return map[string][]byte{
		// Basic multilingual plane
		"chinese":  []byte("package main // 中文注释"),
		"japanese": []byte("package main // 日本語コメント"),
		"korean":   []byte("package main // 한글 주석"),
		"arabic":   []byte("package main // تعليق عربي"),
		"hebrew":   []byte("package main // הערה בעברית"),
		"cyrillic": []byte("package main // Кириллица"),
		"greek":    []byte("package main // Ελληνικά"),

		// Special characters
		"emoji":      []byte("package main // 🚀 emoji test 🎉"),
		"math":       []byte("package main // ∑∏∫∂√∞≈≠≤≥"),
		"combining":  []byte("package main // café résumé naïve"),
		"diacritics": []byte("package main // àáâãäå èéêë"),

		// Edge cases
		"zero_width": []byte("package main // zero\u200Bwidth"),
		"bom":        []byte("\xEF\xBB\xBFpackage main // BOM"),
		"rtl":        []byte("package main // \u202Brtl\u202C"),

		// Invalid UTF-8 sequences (should be handled gracefully)
		"invalid_continuation": []byte("package main // \x80\x81\x82"),
		"truncated":            []byte("package main // \xC2"),     // Truncated 2-byte
		"overlong":             []byte("package main // \xC0\xAF"), // Overlong encoding
	}
}

// CreateUnicodeFile creates a file with Unicode content
func (u *UnicodeEdgeCases) CreateUnicodeFile(t *testing.T, name string, content []byte) string {
	path := filepath.Join(u.TempDir, name+".go")
	err := os.WriteFile(path, content, 0644)
	require.NoError(t, err)
	return path
}

// ============================================================================
// Concurrent Access Edge Cases
// ============================================================================

// ConcurrentEdgeCases provides test scenarios for concurrency edge conditions
type ConcurrentEdgeCases struct {
	sync.Mutex
	Operations []string
}

// NewConcurrentEdgeCases creates concurrent edge case scenarios
func NewConcurrentEdgeCases() *ConcurrentEdgeCases {
	return &ConcurrentEdgeCases{
		Operations: make([]string, 0, 1000),
	}
}

// RecordOperation records an operation with thread safety
func (c *ConcurrentEdgeCases) RecordOperation(op string) {
	c.Lock()
	defer c.Unlock()
	c.Operations = append(c.Operations, op)
}

// RunConcurrentOperations runs operations concurrently and records them
func (c *ConcurrentEdgeCases) RunConcurrentOperations(t *testing.T, ops []func(), timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(ops))

	for i, op := range ops {
		wg.Add(1)
		go func(idx int, operation func()) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- errors.New("panic in concurrent operation")
				}
			}()

			c.RecordOperation("start-" + string(rune('A'+idx)))
			operation()
			c.RecordOperation("end-" + string(rune('A'+idx)))
		}(i, op)
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errChan)
		for err := range errChan {
			if err != nil {
				return err
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ============================================================================
// Memory and Resource Edge Cases
// ============================================================================

// MemoryPressureScenario tests behavior under memory pressure
type MemoryPressureScenario struct {
	Name          string
	AllocateMB    int
	Operation     func(t *testing.T)
	ShouldSucceed bool
}

// RunWithMemoryPressure runs an operation while allocating memory
func RunWithMemoryPressure(t *testing.T, allocMB int, op func(t *testing.T)) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	// Allocate memory to create pressure
	pressure := make([][]byte, allocMB)
	for i := range pressure {
		pressure[i] = make([]byte, 1024*1024) // 1MB per slice
	}

	// Run the operation
	op(t)

	// Keep reference to prevent GC
	_ = pressure
}

// ============================================================================
// Timeout and Cancellation Edge Cases
// ============================================================================

// TimeoutScenario represents a test scenario for timeout handling
type TimeoutScenario struct {
	Name      string
	Timeout   time.Duration
	Operation func(ctx context.Context) error
	ExpectErr error
}

// RunTimeoutScenario runs an operation with a timeout
func RunTimeoutScenario(t *testing.T, scenario TimeoutScenario) error {
	ctx, cancel := context.WithTimeout(context.Background(), scenario.Timeout)
	defer cancel()

	err := scenario.Operation(ctx)

	if scenario.ExpectErr != nil {
		assert.True(t, errors.Is(err, scenario.ExpectErr) || err == scenario.ExpectErr,
			"Expected error %v, got %v", scenario.ExpectErr, err)
	}

	return err
}

// ============================================================================
// Input Validation Edge Cases
// ============================================================================

// InputValidationTestCase represents a test case for input validation
type InputValidationTestCase struct {
	Name        string
	Input       interface{}
	Validator   func(interface{}) error
	ExpectValid bool
	ErrorMatch  string
}

// RunInputValidationTests runs a suite of input validation tests
func RunInputValidationTests(t *testing.T, cases []InputValidationTestCase) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.Validator(tc.Input)

			if tc.ExpectValid {
				assert.NoError(t, err, "Expected input to be valid")
			} else {
				assert.Error(t, err, "Expected input to be invalid")
				if tc.ErrorMatch != "" {
					assert.Contains(t, err.Error(), tc.ErrorMatch)
				}
			}
		})
	}
}

// ============================================================================
// Error Recovery Edge Cases
// ============================================================================

// RecoveryScenario tests that system can recover from errors
type RecoveryScenario struct {
	Name       string
	CauseError func() error
	Recover    func() error
	Verify     func(t *testing.T) bool
}

// RunRecoveryScenario tests error recovery
func RunRecoveryScenario(t *testing.T, scenario RecoveryScenario) {
	// Cause the error
	err := scenario.CauseError()
	require.Error(t, err, "Should cause an error")

	// Attempt recovery
	err = scenario.Recover()
	require.NoError(t, err, "Recovery should succeed")

	// Verify system is in good state
	assert.True(t, scenario.Verify(t), "System should be in valid state after recovery")
}

// ============================================================================
// Boundary Value Edge Cases
// ============================================================================

// BoundaryValueTestCase represents a test case for boundary values
type BoundaryValueTestCase struct {
	Name        string
	Value       int
	LowerBound  int
	UpperBound  int
	TestFunc    func(value int) error
	ExpectValid bool
}

// RunBoundaryValueTests runs tests for boundary conditions
func RunBoundaryValueTests(t *testing.T, cases []BoundaryValueTestCase) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.TestFunc(tc.Value)

			if tc.ExpectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// GenerateBoundaryValues generates test values around a boundary
func GenerateBoundaryValues(boundary int) []int {
	return []int{
		boundary - 2,
		boundary - 1,
		boundary,
		boundary + 1,
		boundary + 2,
	}
}

// ============================================================================
// Path Edge Cases
// ============================================================================

// PathEdgeCases provides test scenarios for path handling
type PathEdgeCases struct{}

// GetEdgeCasePaths returns paths that should be handled gracefully
func (p PathEdgeCases) GetEdgeCasePaths() []string {
	return []string{
		"",                       // Empty path
		".",                      // Current directory
		"..",                     // Parent directory
		"/",                      // Root (Unix)
		"//",                     // Double slash
		"./file.go",              // Relative with dot
		"../file.go",             // Relative with parent
		"path/./file.go",         // Path with dot
		"path/../file.go",        // Path with parent
		"path//file.go",          // Double slash in path
		strings.Repeat("a", 256), // Long filename
		"file\x00name.go",        // Null byte in name
		"file\nname.go",          // Newline in name
		"file\tname.go",          // Tab in name
		" leading.go",            // Leading space
		"trailing.go ",           // Trailing space
		"file name.go",           // Space in name
		"file?name.go",           // Question mark
		"file*name.go",           // Asterisk
		"CON.go",                 // Windows reserved name
		"NUL.go",                 // Windows reserved name
		"file.go.go",             // Double extension
		".hidden",                // Hidden file
		"...three",               // Multiple dots
		"中文.go",                  // Unicode filename
		"ファイル.go",                // Japanese filename
	}
}

// ============================================================================
// Nil and Zero Value Edge Cases
// ============================================================================

// TestNilSafety tests that functions handle nil inputs gracefully
func TestNilSafety(t *testing.T, funcs map[string]func() error) {
	for name, fn := range funcs {
		t.Run(name, func(t *testing.T) {
			// Should not panic
			var panicked bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
					}
				}()
				_ = fn()
			}()

			assert.False(t, panicked, "Function should not panic on nil input")
		})
	}
}

// ============================================================================
// Race Condition Detection Helpers
// ============================================================================

// DetectRaceCondition attempts to trigger race conditions
func DetectRaceCondition(t *testing.T, concurrentOps int, operation func(id int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < concurrentOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start // Wait for start signal
			operation(id)
		}(i)
	}

	// Start all goroutines simultaneously
	close(start)
	wg.Wait()
}

// ============================================================================
// Helper Assertions
// ============================================================================

// AssertEventuallyTrue waits for a condition to become true
func AssertEventuallyTrue(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// AssertNeverTrue ensures a condition never becomes true within a timeout
func AssertNeverTrue(t *testing.T, duration time.Duration, condition func() bool, msg string) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if condition() {
			t.Fatal(msg)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// AssertResourceCleaned verifies a resource was properly cleaned up
func AssertResourceCleaned(t *testing.T, checkFunc func() bool, resourceName string) {
	AssertEventuallyTrue(t, 5*time.Second, checkFunc,
		"Resource "+resourceName+" was not properly cleaned up")
}
