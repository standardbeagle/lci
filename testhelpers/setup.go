// Package testhelpers provides shared utilities for testing Lightning Code Index
package testhelpers

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
	"github.com/standardbeagle/lci/internal/config"
)

// createTestConfig creates a configuration optimized for testing
func createTestConfig(tempDir string) *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   50,               // 50MB for tests
			MaxFileCount:     1000,             // Limited for test performance
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
			RespectGitignore: false, // Disabled for tests
			WatchMode:        false, // Disabled for tests
		},
		Performance: config.Performance{
			MaxMemoryMB:   100, // Reduced for tests
			MaxGoroutines: 4,   // Limited for predictable behavior
			DebounceMs:    10,  // Fast debounce for tests
		},
		Search: config.Search{
			DefaultContextLines:    3,
			MaxResults:             50,
			EnableFuzzy:            true,
			MaxContextLines:        20,
			MergeFileResults:       true,
			EnsureCompleteStmt:     true,
			IncludeLeadingComments: true,
		},
		FeatureFlags: config.FeatureFlags{
			EnableMemoryLimits:          true,
			EnableGracefulDegradation:   true,
			EnablePerformanceMonitoring: false, // Disabled for cleaner test output
			EnableDetailedErrorLogging:  false, // Disabled for cleaner test output
			EnableFeatureFlagLogging:    false, // Disabled for cleaner test output
		},
		Include: []string{
			"*.go", "*.js", "*.jsx", "*.ts", "*.tsx", "*.py",
			"*.java", "*.c", "*.cpp", "*.h", "*.hpp", "*.rs",
			"*.md", "*.txt",
		},
		Exclude: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/dist/**",
			"**/build/**",
			"**/target/**",
			"**/*.min.js",
			"**/__pycache__/**",
		},
	}
}

// WaitFor waits for a condition to become true with timeout
// Usage:
//
//	testhelpers.WaitFor(t, func() bool {
//	    return index.IsReady()
//	}, 5*time.Second)
func WaitFor(t *testing.T, condition func() bool, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if condition() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("Condition not met within %v", timeout)
				return
			}
		}
	}
}

// RetryOptions configures retry behavior
type RetryOptions struct {
	MaxAttempts int           // Maximum number of attempts
	BaseDelay   time.Duration // Base delay for exponential backoff
	MaxDelay    time.Duration // Maximum delay between attempts
	Jitter      bool          // Add random jitter to delays
	Timeout     time.Duration // Total timeout for all attempts
}

// RetryWithBackoff retries a function with exponential backoff
// Usage:
//
//	err := testhelpers.RetryWithBackoff(t, testhelpers.RetryOptions{
//	    MaxAttempts: 5,
//	    BaseDelay:   100 * time.Millisecond,
//	    MaxDelay:    2 * time.Second,
//	    Jitter:      true,
//	}, func() error {
//	    return performOperation()
//	})
func RetryWithBackoff(t *testing.T, opts RetryOptions, fn func() error) error {
	t.Helper()

	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.BaseDelay == 0 {
		opts.BaseDelay = 100 * time.Millisecond
	}
	if opts.MaxDelay == 0 {
		opts.MaxDelay = 5 * time.Second
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	var lastErr error
	start := time.Now()

	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		// Check total timeout
		if time.Since(start) > opts.Timeout {
			return fmt.Errorf("timeout after %v (attempt %d/%d): last error: %v",
				time.Since(start), attempt, opts.MaxAttempts, lastErr)
		}

		err := fn()
		if err == nil {
			// Success on attempt 1 doesn't need to log
			if attempt > 1 {
				t.Logf("Succeeded on attempt %d/%d", attempt, opts.MaxAttempts)
			}
			return nil
		}

		lastErr = err

		// Last attempt - return error
		if attempt == opts.MaxAttempts {
			t.Logf("Failed after %d attempts: %v", attempt, err)
			return err
		}

		// Calculate delay with exponential backoff
		delay := time.Duration(1<<uint(attempt-1)) * opts.BaseDelay
		if delay > opts.MaxDelay {
			delay = opts.MaxDelay
		}

		// Add jitter if enabled (10-20% random variation)
		if opts.Jitter {
			jitter := time.Duration(float64(delay) * (0.1 + 0.1*float64(attempt%2)))
			if attempt%2 == 0 {
				delay += jitter
			} else {
				delay -= jitter
			}
		}

		t.Logf("Attempt %d/%d failed: %v, retrying in %v...",
			attempt, opts.MaxAttempts, err, delay)

		// Wait with timeout
		waitCh := make(chan struct{})
		go func() {
			defer close(waitCh)
			time.Sleep(delay)
		}()

		select {
		case <-waitCh:
			// Continue to next attempt
		case <-time.After(opts.Timeout):
			// Total timeout exceeded
			return fmt.Errorf("timeout exceeded while retrying: %v", err)
		}
	}

	return lastErr
}

// WaitForWithJitter waits for a condition with exponential backoff retry
// Usage:
//
//	err := testhelpers.WaitForWithJitter(t, testhelpers.RetryOptions{
//	    MaxAttempts: 5,
//	    BaseDelay:   50 * time.Millisecond,
//	    Jitter:      true,
//	}, func() bool {
//	    return checkResourceCleaned()
//	})
func WaitForWithJitter(t *testing.T, opts RetryOptions, condition func() bool) error {
	return RetryWithBackoff(t, opts, func() error {
		if condition() {
			return nil
		}
		return errors.New("condition not yet met")
	})
}

// NoRetry is a convenience function for WaitFor without retry
func NoRetry() RetryOptions {
	return RetryOptions{
		MaxAttempts: 1,
		Timeout:     1 * time.Minute,
	}
}

// WaitForCleanup waits for background operations to complete
// Used in tests that spawn goroutines to ensure proper cleanup
func WaitForCleanup(t *testing.T, timeout time.Duration) {
	t.Helper()

	// Give goroutines time to cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify no goroutine leaks
	if err := goleak.Find(goleak.IgnoreCurrent()); err != nil {
		t.Errorf("Goroutine leak detected: %v", err)
	}
}

// MarkFlaky marks a test as flaky with a reason
// Usage: testhelpers.MarkFlaky(t, "Race condition in cleanup")
func MarkFlaky(t *testing.T, reason string) {
	t.Helper()
	t.Logf("FLAKY TEST: %s", reason)

	// In CI, this could be used to mark tests for separate execution
	// For now, just log the reason
}

// AssertNoLeaks verifies no goroutine leaks occurred during the test
func AssertNoLeaks(t *testing.T) {
	t.Helper()

	// Ignore goroutines started by the test runtime
	ignore := goleak.IgnoreCurrent()

	if err := goleak.Find(ignore); err != nil {
		t.Errorf("Goroutine leak detected: %v", err)
	}
}

// SkipIfShort skips the test if -short flag is provided
func SkipIfShort(t *testing.T, reason string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping in short mode: %s", reason)
	}
}

// SkipInCI skips the test if running in CI environment
func SkipInCI(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skipf("Skipping in CI: %s", reason)
	}
}

// TestData provides access to common test data patterns
var TestData = struct {
	GoSimple   string
	GoComplex  string
	Javascript string
	Python     string
	MultiLang  map[string]string
}{
	GoSimple: `package main

import "fmt"

func hello() {
	fmt.Println("Hello, World!")
}

func main() {
	hello()
}`,

	GoComplex: `package service

import (
	"context"
	"errors"
	"time"
)

type Service struct {
	timeout time.Duration
}

func NewService(timeout time.Duration) *Service {
	return &Service{timeout: timeout}
}

func (s *Service) Process(ctx context.Context, data string) (string, error) {
	if data == "" {
		return "", errors.New("empty data")
	}

	select {
	case <-time.After(s.timeout):
		return "", errors.New("timeout")
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "processed: " + data, nil
	}
}`,

	Javascript: `function calculateSum(a, b) {
	return a + b;
}

class Calculator {
	constructor() {
		this.result = 0;
	}

	add(value) {
		this.result += value;
		return this;
	}

	multiply(value) {
		this.result *= value;
		return this;
	}

	getResult() {
		return this.result;
	}
}

const calculator = new Calculator();
export { calculateSum, Calculator };`,

	Python: `def calculate_fibonacci(n):
    """Calculate the nth Fibonacci number."""
    if n <= 1:
        return n
    return calculate_fibonacci(n-1) + calculate_fibonacci(n-2)

class DataProcessor:
    def __init__(self, name):
        self.name = name
        self.data = []

    def add_data(self, item):
        self.data.append(item)

    def process_data(self):
        return [item.upper() for item in self.data if isinstance(item, str)]

    def __str__(self):
        return f"DataProcessor({self.name})"

if __name__ == "__main__":
    processor = DataProcessor("test")
    processor.add_data("hello")
    processor.add_data("world")
    print(processor.process_data())`,
}

// GetMultiLangProject returns a test project with multiple language files
func GetMultiLangProject() map[string]string {
	return map[string]string{
		"main.go":            TestData.GoSimple,
		"service/service.go": TestData.GoComplex,
		"calc.js":            TestData.Javascript,
		"process.py":         TestData.Python,
		"README.md":          "# Test Project\n\nThis is a test project with multiple languages.",
	}
}

// PopulateTestIndexes properly indexes symbols in both SymbolIndex and ReferenceTracker
// This is required for tests that manually create components instead of using SetupTestIndex
// Usage:
//
//	import "github.com/standardbeagle/lci/internal/core"
//	import "github.com/standardbeagle/lci/internal/types"
//
//	symbols := []types.Symbol{...}
//	testhelpers.PopulateTestIndexes(
//		symbolIndex,
//		refTracker,
//		fileID,
//		"test.go",
//		symbols,
//	)
//
// This ensures that ContextLookupEngine.GetContext() can find symbols via ReferenceTracker.
// The function uses reflection to work with the core types without requiring direct imports here.
func PopulateTestIndexes(symbolIndex, refTracker, fileID, path interface{}, symbols interface{}) {
	// Call IndexSymbols on SymbolIndex
	if si, ok := symbolIndex.(interface {
		IndexSymbols(fileID interface{}, symbols interface{})
	}); ok {
		si.IndexSymbols(fileID, symbols)
	}

	// Call ProcessFile on ReferenceTracker to populate EnhancedSymbols
	if rt, ok := refTracker.(interface {
		ProcessFile(fileID interface{}, path string, symbols interface{}, references interface{}, scopes interface{}) interface{}
	}); ok {
		// Pass empty references and scopes - tests typically don't set these
		rt.ProcessFile(fileID, path.(string), symbols, []interface{}{}, []interface{}{})
	}
}
