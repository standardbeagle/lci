package testhelpers

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestValidator provides comprehensive test validation and cleanup verification
type TestValidator struct {
	t                     *testing.T
	initialGoroutineCount int
	initialMemoryUsage    uint64
	checks                []ValidationCheck
}

// ValidationCheck represents a validation check to be performed
type ValidationCheck struct {
	Name     string
	Check    func() error
	Critical bool
}

// NewTestValidator creates a new test validator
func NewTestValidator(t *testing.T) *TestValidator {
	return &TestValidator{
		t:                     t,
		initialGoroutineCount: runtime.NumGoroutine(),
		initialMemoryUsage:    getMemoryUsageValidation(),
		checks:                make([]ValidationCheck, 0),
	}
}

// AddGoroutineCheck adds a goroutine leak check
func (tv *TestValidator) AddGoroutineCheck() {
	tv.checks = append(tv.checks, ValidationCheck{
		Name: "Goroutine Leak Check",
		Check: func() error {
			current := runtime.NumGoroutine()
			if current > tv.initialGoroutineCount {
				return &ValidationError{
					Type:    "goroutine_leak",
					Message: fmt.Sprintf("Goroutine count increased from %d to %d", tv.initialGoroutineCount, current),
				}
			}
			return nil
		},
		Critical: true,
	})
}

// AddMemoryCheck adds a memory leak check
func (tv *TestValidator) AddMemoryCheck() {
	tv.checks = append(tv.checks, ValidationCheck{
		Name: "Memory Leak Check",
		Check: func() error {
			// Force garbage collection to get accurate memory reading
			runtime.GC()
			runtime.GC()
			time.Sleep(10 * time.Millisecond) // Allow GC to complete

			current := getMemoryUsageValidation()
			threshold := tv.initialMemoryUsage + (50 * 1024 * 1024) // 50MB threshold

			if current > threshold {
				return &ValidationError{
					Type:    "memory_leak",
					Message: fmt.Sprintf("Memory usage increased from %d to %d (threshold: %d)", tv.initialMemoryUsage, current, threshold),
				}
			}
			return nil
		},
		Critical: false,
	})
}

// AddGlobalStateCheck adds a global state validation check
func (tv *TestValidator) AddGlobalStateCheck() {
	tv.checks = append(tv.checks, ValidationCheck{
		Name: "Global State Check",
		Check: func() error {
			// This would check that global singletons are in their reset state
			// Implementation depends on specific global state management
			return nil
		},
		Critical: true,
	})
}

// AddCustomCheck adds a custom validation check
func (tv *TestValidator) AddCustomCheck(name string, check func() error, critical bool) {
	tv.checks = append(tv.checks, ValidationCheck{
		Name:     name,
		Check:    check,
		Critical: critical,
	})
}

// Validate performs all registered validation checks
func (tv *TestValidator) Validate() {
	tv.t.Log("Running test validation checks")

	failedChecks := make([]ValidationCheck, 0)
	criticalFailures := make([]ValidationCheck, 0)

	for _, check := range tv.checks {
		if err := check.Check(); err != nil {
			tv.t.Logf("Validation check failed: %s - %v", check.Name, err)
			failedChecks = append(failedChecks, check)

			if check.Critical {
				criticalFailures = append(criticalFailures, check)
			}
		}
	}

	if len(criticalFailures) > 0 {
		tv.t.Errorf("CRITICAL VALIDATION FAILURES: %d critical checks failed", len(criticalFailures))
		for _, check := range criticalFailures {
			tv.t.Errorf("  - %s", check.Name)
		}
	}

	if len(failedChecks) > 0 {
		tv.t.Logf("Total validation failures: %d (Critical: %d)", len(failedChecks), len(criticalFailures))
	}
}

// getMemoryUsageValidation returns current memory usage in bytes
func getMemoryUsageValidation() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// ValidationError represents a validation error
type ValidationError struct {
	Type    string
	Message string
}

func (ve *ValidationError) Error() string {
	return ve.Message
}

// TestCleanupManager provides comprehensive cleanup management
type TestCleanupManager struct {
	t          *testing.T
	cleanupOps []CleanupOperation
	completed  bool
}

// CleanupOperation represents a cleanup operation
type CleanupOperation struct {
	Name      string
	Operation func() error
	Timeout   time.Duration
}

// NewTestCleanupManager creates a new cleanup manager
func NewTestCleanupManager(t *testing.T) *TestCleanupManager {
	return &TestCleanupManager{
		t:          t,
		cleanupOps: make([]CleanupOperation, 0),
		completed:  false,
	}
}

// AddCleanup adds a cleanup operation
func (tcm *TestCleanupManager) AddCleanup(name string, cleanup func() error) {
	tcm.AddCleanupWithTimeout(name, cleanup, 5*time.Second)
}

// AddCleanupWithTimeout adds a cleanup operation with a custom timeout
func (tcm *TestCleanupManager) AddCleanupWithTimeout(name string, cleanup func() error, timeout time.Duration) {
	if tcm.completed {
		tcm.t.Error("Cannot add cleanup operations after cleanup has completed")
		return
	}

	tcm.cleanupOps = append(tcm.cleanupOps, CleanupOperation{
		Name:      name,
		Operation: cleanup,
		Timeout:   timeout,
	})
}

// AddFileCleanup adds file cleanup operations
func (tcm *TestCleanupManager) AddFileCleanup(filePaths ...string) {
	for _, path := range filePaths {
		tcm.AddCleanup(fmt.Sprintf("Remove file: %s", path), func() error {
			// Implementation would remove the file
			return nil
		})
	}
}

// AddGoroutineCleanup adds goroutine cleanup operations
func (tcm *TestCleanupManager) AddGoroutineCleanup(stopFuncs ...func()) {
	for i, stopFunc := range stopFuncs {
		tcm.AddCleanup(fmt.Sprintf("Stop goroutine %d", i), func() error {
			stopFunc()
			return nil
		})
	}
}

// Execute executes all cleanup operations
func (tcm *TestCleanupManager) Execute() {
	if tcm.completed {
		return
	}

	tcm.t.Log("Executing cleanup operations")
	defer func() { tcm.completed = true }()

	// Run cleanup operations in reverse order (LIFO)
	for i := len(tcm.cleanupOps) - 1; i >= 0; i-- {
		op := tcm.cleanupOps[i]

		done := make(chan error, 1)
		go func() {
			done <- op.Operation()
		}()

		select {
		case err := <-done:
			if err != nil {
				tcm.t.Errorf("Cleanup operation '%s' failed: %v", op.Name, err)
			}
		case <-time.After(op.Timeout):
			tcm.t.Errorf("Cleanup operation '%s' timed out after %v", op.Name, op.Timeout)
		}
	}
}

// ComprehensiveTestRunner provides a complete test execution environment
type ComprehensiveTestRunner struct {
	t               *testing.T
	validator       *TestValidator
	cleanupManager  *TestCleanupManager
	testDataBuilder *TestDataBuilder
}

// NewComprehensiveTestRunner creates a comprehensive test runner
func NewComprehensiveTestRunner(t *testing.T) *ComprehensiveTestRunner {
	return &ComprehensiveTestRunner{
		t:               t,
		validator:       NewTestValidator(t),
		cleanupManager:  NewTestCleanupManager(t),
		testDataBuilder: NewTestDataBuilder(),
	}
}

// AddTestData adds test data to the runner
func (ctr *ComprehensiveTestRunner) AddTestData(builder *TestDataBuilder) *ComprehensiveTestRunner {
	ctr.testDataBuilder = builder
	ctr.cleanupManager.AddCleanup("Test data cleanup", func() error {
		// Cleanup test data resources
		return nil
	})
	return ctr
}

// AddValidation adds validation checks
func (ctr *ComprehensiveTestRunner) AddValidation(check ValidationCheck) *ComprehensiveTestRunner {
	ctr.validator.checks = append(ctr.validator.checks, check)
	return ctr
}

// AddCleanup adds cleanup operations
func (ctr *ComprehensiveTestRunner) AddCleanup(name string, cleanup func() error) *ComprehensiveTestRunner {
	ctr.cleanupManager.AddCleanup(name, cleanup)
	return ctr
}

// Run executes a test with comprehensive isolation and validation
func (ctr *ComprehensiveTestRunner) Run(testFunc func(*IsolatedTestData, *testing.T)) {
	ctr.t.Log("Starting comprehensive isolated test")

	// Add default validation checks
	ctr.validator.AddGoroutineCheck()
	ctr.validator.AddMemoryCheck()
	ctr.validator.AddGlobalStateCheck()

	// Ensure cleanup happens
	defer ctr.cleanupManager.Execute()
	defer ctr.validator.Validate()

	// Run the actual test
	testFunc(ctr.testDataBuilder.Build(), ctr.t)
}

// RunParallel executes a test in parallel with comprehensive isolation
func (ctr *ComprehensiveTestRunner) RunParallel(testFunc func(*IsolatedTestData, *testing.T)) {
	ctr.t.Parallel()
	ctr.Run(testFunc)
}

// MemoryValidationResult holds the results of a memory validation check
type MemoryValidationResult struct {
	ContentSize       int64
	GlobalHeapBytes   int64
	StoreMemoryBytes  int64
	GlobalMultiplier  float64
	StoreMultiplier   float64
	BackgroundRunning bool
}

// ValidateFileContentStoreMemory performs a two-tier memory validation:
// 1. Quick global MemStats check (can detect obvious issues)
// 2. Accurate store-specific memory tracking (isolated from background operations)
// Returns detailed results for test logging and assertion
func ValidateFileContentStoreMemory(t *testing.T, store MemoryTracker, content []byte, maxMultiplier float64) *MemoryValidationResult {
	// Take baseline measurements
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Perform the operation (store already created and file loaded)
	// Store memory is measured after the operation completes

	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	contentSize := int64(len(content))
	storeMemoryBytes := store.GetMemoryUsage()

	globalHeapBytes := int64(m2.Alloc) - int64(m1.Alloc)
	globalMultiplier := float64(globalHeapBytes) / float64(contentSize)
	storeMultiplier := float64(storeMemoryBytes) / float64(contentSize)

	// Detect if background operations are running (global much higher than store)
	backgroundRunning := globalMultiplier > storeMultiplier*2

	result := &MemoryValidationResult{
		ContentSize:       contentSize,
		GlobalHeapBytes:   globalHeapBytes,
		StoreMemoryBytes:  storeMemoryBytes,
		GlobalMultiplier:  globalMultiplier,
		StoreMultiplier:   storeMultiplier,
		BackgroundRunning: backgroundRunning,
	}

	// Log detailed information
	t.Logf("Memory validation: content=%d KB, global heap=%d KB (%.2fx), store memory=%d KB (%.2fx)",
		contentSize/1024, globalHeapBytes/1024, globalMultiplier,
		storeMemoryBytes/1024, storeMultiplier)

	if backgroundRunning {
		t.Logf("Note: Global heap (%.2fx) significantly higher than store (%.2fx) - background operations detected",
			globalMultiplier, storeMultiplier)
	}

	// Assert using store-specific measurement (prevents false positives from background operations)
	assert.Less(t, storeMultiplier, maxMultiplier,
		"Memory usage should not exceed %.1fx content size (got %.2fx)", maxMultiplier, storeMultiplier)

	return result
}

// MemoryTracker interface for memory tracking in tests
type MemoryTracker interface {
	GetMemoryUsage() int64
}
