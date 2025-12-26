package testhelpers

import (
	"context"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestIsolationManager provides comprehensive test isolation
type TestIsolationManager struct {
	mu           sync.Mutex
	activeTest   string
	startTime    time.Time
	globalLocks  map[string]*sync.RWMutex
	cleanupFuncs []func()
}

var (
	globalIsolationManager = &TestIsolationManager{
		globalLocks:  make(map[string]*sync.RWMutex),
		cleanupFuncs: make([]func(), 0),
	}
)

// IsolateTest creates a new isolated test environment
func IsolateTest(t *testing.T, testName string, testFunc func(t *testing.T)) {
	globalIsolationManager.mu.Lock()
	defer globalIsolationManager.mu.Unlock()

	// Check if another test is running
	if globalIsolationManager.activeTest != "" {
		t.Errorf("Test isolation violation: test '%s' is already running", globalIsolationManager.activeTest)
		return
	}

	globalIsolationManager.activeTest = testName
	globalIsolationManager.startTime = time.Now()

	// Register cleanup
	defer func() {
		globalIsolationManager.activeTest = ""

		// Run all cleanup functions
		for _, cleanup := range globalIsolationManager.cleanupFuncs {
			cleanup()
		}
		globalIsolationManager.cleanupFuncs = globalIsolationManager.cleanupFuncs[:0]

		// Verify no goroutine leaks
		goleak.VerifyNone(t)
	}()

	// Run the actual test
	testFunc(t)
}

// RegisterCleanup adds a cleanup function to be called after test completion
func RegisterCleanup(cleanup func()) {
	globalIsolationManager.mu.Lock()
	defer globalIsolationManager.mu.Unlock()
	globalIsolationManager.cleanupFuncs = append(globalIsolationManager.cleanupFuncs, cleanup)
}

// GetGlobalLock returns a named global lock for critical sections
func GetGlobalLock(name string) *sync.RWMutex {
	globalIsolationManager.mu.Lock()
	defer globalIsolationManager.mu.Unlock()

	if _, exists := globalIsolationManager.globalLocks[name]; !exists {
		globalIsolationManager.globalLocks[name] = &sync.RWMutex{}
	}
	return globalIsolationManager.globalLocks[name]
}

// DisablePerformanceDemo disables the performance demo that interferes with tests
func DisablePerformanceDemo() {
	// Set environment variable to disable demo
	os.Setenv("DISABLE_PERFORMANCE_DEMO", "true")
	RegisterCleanup(func() {
		os.Unsetenv("DISABLE_PERFORMANCE_DEMO")
	})
}

// IsolatedTestConfig provides configuration for isolated tests
type IsolatedTestConfig struct {
	MaxGoroutines      int
	TestTimeout        time.Duration
	EnableRaceDetector bool
	DisablePerformance bool
	IsolateMemory      bool
}

// DefaultIsolationConfig returns sensible defaults for test isolation
func DefaultIsolationConfig() IsolatedTestConfig {
	return IsolatedTestConfig{
		MaxGoroutines:      4, // Reduce concurrency in tests
		TestTimeout:        30 * time.Second,
		EnableRaceDetector: true,
		DisablePerformance: true,
		IsolateMemory:      true,
	}
}

// ConfigureTestEnvironment sets up the test environment according to config
func ConfigureTestEnvironment(t *testing.T, config IsolatedTestConfig) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
	RegisterCleanup(cancel)

	if config.DisablePerformance {
		DisablePerformanceDemo()
	}

	// Add any additional environment setup here
	if config.IsolateMemory {
		// Force garbage collection before/after tests
		RegisterCleanup(func() {
			runtime.GC()
		})
	}

	return ctx
}

// ConcurrentSafeTest runs a test with proper concurrent safety
func ConcurrentSafeTest(t *testing.T, testName string, testFunc func(t *testing.T)) {
	IsolateTest(t, testName, func(t *testing.T) {
		// Use a unique subdirectory for each test
		testDir := t.TempDir()

		// Change to test directory if needed
		originalDir, _ := os.Getwd()
		_ = os.Chdir(testDir)
		defer func() { _ = os.Chdir(originalDir) }()

		// Set test-specific environment
		os.Setenv("TEST_NAME", testName)
		os.Setenv("TEST_ISOLATED", "true")

		defer func() {
			os.Unsetenv("TEST_NAME")
			os.Unsetenv("TEST_ISOLATED")
		}()

		testFunc(t)
	})
}
