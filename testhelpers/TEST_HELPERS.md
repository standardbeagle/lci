# Test Helpers Documentation

This directory contains comprehensive testing utilities for the Lightning Code Index project. These helpers provide debugging, performance monitoring, flaky test detection, and test isolation capabilities.

## Overview

The test helpers follow the project's philosophy of **no mocking the index** - all tests use real indexing on actual code to ensure accurate behavior.

## Core Components

### 1. Debug and Diagnostics (`debug.go`)

Provides comprehensive debugging utilities for test execution with memory tracking, goroutine monitoring, and operation tracing.

#### Quick Start

```go
func TestMyFeature(t *testing.T) {
    helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugVerbose)
    defer helper.Close()

    helper.Logf("Starting test")
    helper.CaptureState("test_state")
    helper.StartOperation("my_operation")

    // Run your test code
    result := performOperation()

    helper.EndOperation("my_operation", nil)
}
```

#### Debug Levels

- `DebugNone` - Minimal overhead, no logging
- `DebugBasic` - Log operations and errors
- `DebugVerbose` - Include state capture and memory stats
- `DebugTrace` - Full tracing with timing information

#### Key Features

- **State Capture**: Track memory usage, goroutine count, and timing
- **Operation Tracking**: Wrap operations with Start/End markers
- **Memory Trends**: Analyze memory allocation patterns
- **Log Rotation**: Automatic log rotation when size limits are reached
- **Structured Output**: JSON reports for post-test analysis

#### Example: Tracing an Operation

```go
err := helper.TraceOperation("indexing", func() error {
    return index.Build(context.Background())
})
if err != nil {
    helper.FailWithDebug("Indexing failed: %v", err)
}
```

### 2. Flaky Test Detection (`flaky_detector.go`)

Automatically detects and tracks flaky tests with pattern recognition and failure rate analysis.

#### Key Features

- **Pattern Detection**: Recognizes common flaky test patterns
- **Failure Rate Tracking**: Calculates flakiness percentage over time
- **Occurrence Logging**: Records each flaky test instance
- **Status Management**: Track active, isolated, fixed, and ignored flaky tests

#### Usage

```go
func TestWithFlakyDetection(t *testing.T) {
    detector := testhelpers.NewFlakyTestDetector(testhelpers.FlakyDetectorConfig{
        MinimumRuns:      10,
        FlakinessThreshold: 0.3, // 30% failure rate
    })
    defer detector.Close()

    // Run your test multiple times
    for i := 0; i < 10; i++ {
        result := performTest()
        detector.RecordResult(result)
    }

    // Check for flaky behavior
    flakyTests := detector.GetFlakyTests()
    if len(flakyTests) > 0 {
        t.Errorf("Detected %d flaky tests", len(flakyTests))
    }
}
```

### 3. Performance Monitoring (`monitor.go`)

Tracks test execution performance and provides optimization suggestions.

#### Key Features

- **Performance Metrics**: Duration, memory, goroutines
- **Slow Test Detection**: Configurable thresholds
- **Optimization Rules**: Actionable suggestions for improvement
- **Baseline Comparison**: Track performance over time
- **Detailed Reporting**: JSON output for CI integration

#### Usage

```go
func TestWithPerformanceMonitoring(t *testing.T) {
    monitor := testhelpers.NewTestPerformanceMonitor(testhelpers.MonitorConfig{
        EnableDetailedLogging: true,
        HistoryRetentionDays:  30,
    })
    defer monitor.Close()

    helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugVerbose)

    // Your test code
    startTime := time.Now()
    result := performComplexOperation()
    duration := time.Since(startTime)

    // Record performance
    monitor.RecordTest(t.Name(), duration, getMemoryStats(), goroutineDelta)
    helper.CaptureState("test_end")

    // Get optimization suggestions
    suggestions := monitor.GetOptimizationSuggestions()
    for _, suggestion := range suggestions {
        t.Logf("Optimization: %s", suggestion)
    }
}
```

### 4. Test Setup and Utilities (`setup.go`)

Common test configuration and utilities for consistent test environments.

#### Test Configuration

```go
// Create test-optimized config
config := testhelpers.createTestConfig(tempDir)

// Wait for condition with timeout
testhelpers.WaitFor(t, func() bool {
    return index.IsReady()
}, 5*time.Second)

// Verify no goroutine leaks
testhelpers.AssertNoLeaks(t)

// Mark test as flaky
testhelpers.MarkFlaky(t, "Race condition in cleanup")
```

#### Test Data

Pre-defined test fixtures for multiple programming languages:

```go
// Use built-in test data
files := testhelpers.GetMultiLangProject()
for path, content := range files {
    writeTestFile(path, content)
}

// Populate test indexes (when not using SetupTestIndex)
testhelpers.PopulateTestIndexes(
    symbolIndex,
    refTracker,
    fileID,
    "test.go",
    symbols,
)
```

### 5. Test Isolation (`test_isolation.go`, `isolated_test_runner.go`)

Ensures tests don't interfere with each other through proper cleanup and isolation.

#### Features

- **Isolated Test Execution**: Each test runs in its own context
- **Global State Reset**: Clean up state between tests
- **Resource Management**: Proper cleanup of resources
- **Test Runner Integration**: Works with Go's testing package

### 6. Baselines and Validation (`baselines.go`, `test_validation.go`)

Maintain performance baselines and validate test results against expected behavior.

#### Usage

```go
// Compare against baseline
baseline := testhelpers.GetBaseline("search_performance")
current := measureSearchPerformance()
testhelpers.ValidatePerformance(baseline, current, 0.1) // 10% tolerance
```

### 7. Event Capture and Monitoring (`events.go`, `monitor.go`)

Capture and analyze events during test execution for debugging and analysis.

### 8. Config Builder (`config_builder.go`)

Build test configurations with different options for various test scenarios.

### 9. Shared State Converter (`shared_state_converter.go`)

Convert between different state representations for testing different code paths.

## Testing Philosophy

### Integration-First Testing

All tests use real indexing on actual code:

```go
// ✅ GOOD: Real indexing
func TestSearch(t *testing.T) {
    index := setupTestIndex(t)  // Uses real files
    results := index.Search("test")
    assert.NotEmpty(t, results)
}

// ❌ BAD: Mocking the index
func TestSearchMock(t *testing.T) {
    mockIndex := new(MockIndex)
    results := mockIndex.Search("test")
    // This doesn't test real behavior
}
```

### Test Organization

Tests follow this pattern:

```go
func TestFeature(t *testing.T) {
    // 1. Setup with debug helper
    helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugVerbose)
    defer helper.Close()

    // 2. Create test environment
    config := testhelpers.createTestConfig(tempDir)
    index := indexing.NewMasterIndex(config)

    // 3. Run test with monitoring
    helper.StartOperation("test_operation")
    defer helper.EndOperation("test_operation", nil)

    // 4. Execute test logic
    result := performTest(index)
    assert.NotNil(t, result)

    // 5. Verify cleanup
    testhelpers.AssertNoLeaks(t)
}
```

## Debug Levels and When to Use Them

| Level | Use Case | Overhead |
|-------|----------|----------|
| `DebugNone` | Performance benchmarks, short tests | Minimal |
| `DebugBasic` | Quick debugging, error tracking | Low |
| `DebugVerbose` | General development, failing tests | Medium |
| `DebugTrace` | Deep investigation, timing analysis | High |

## Best Practices

### 1. Always Use Cleanup Functions

```go
func TestWithCleanup(t *testing.T) {
    helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugVerbose)
    defer helper.Close()  // Ensures logs are saved and resources cleaned

    // Test code here
}
```

### 2. Capture State at Key Points

```go
helper.CaptureState("setup_complete")
// ... perform operation ...
helper.CaptureState("operation_complete")
// ... verify results ...
helper.CaptureState("verification_complete")
```

### 3. Use TraceOperation for Complex Operations

```go
err := helper.TraceOperation("build_index", func() error {
    return index.Build(context.Background())
})
if err != nil {
    helper.FailWithDebug("Build failed: %v", err)
}
```

### 4. Check for Leaks

```go
// Always check for goroutine leaks in concurrent tests
defer testhelpers.AssertNoLeaks(t)
```

### 5. Use Appropriate Test Data

```go
// Use built-in test data for consistency
files := testhelpers.GetMultiLangProject()

// Or create temporary files for integration tests
tempDir := t.TempDir()
writeTestFiles(tempDir, testhelpers.GetMultiLangProject())
```

## Performance Considerations

### Memory Tracking

Debug helpers track memory usage:

- **Alloc**: Currently allocated memory
- **Sys**: Total memory obtained from OS
- **NumGC**: Number of garbage collections

Monitor trends rather than absolute values:

```go
if trend.alloc_change > 10*1024*1024 { // 10MB increase
    t.Errorf("Memory leak detected: +%d bytes", trend.alloc_change)
}
```

### Test Execution Speed

- Use `DebugNone` for benchmarks
- Use `DebugBasic` for quick tests
- Use `DebugVerbose` for failing tests
- Use `DebugTrace` only when needed

## Common Patterns

### Pattern 1: Full Featured Test

```go
func TestFullFeatured(t *testing.T) {
    // Setup
    helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugVerbose)
    defer helper.Close()

    config := testhelpers.createTestConfig(t.TempDir())
    index := indexing.NewMasterIndex(config)

    // Execute
    helper.StartOperation("indexing")
    err := index.Build(context.Background())
    helper.EndOperation("indexing", err)
    if err != nil {
        t.Fatalf("Failed to build index: %v", err)
    }

    // Verify
    results := index.Search("test")
    assert.NotEmpty(t, results)

    // Cleanup
    testhelpers.AssertNoLeaks(t)
}
```

### Pattern 2: Performance Test

```go
func BenchmarkSearch(b *testing.B) {
    config := testhelpers.createTestConfig(tempDir)
    index := indexing.NewMasterIndex(config)
    index.Build(context.Background())

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        results := index.Search("test")
        _ = results
    }
}
```

### Pattern 3: Flaky Test

```go
func TestFlakyOperation(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping flaky test in short mode")
    }

    for i := 0; i < 10; i++ {
        result := performPotentiallyFlakyOperation()
        if !result {
            testhelpers.MarkFlaky(t, "Intermittent failure in operation")
            t.Logf("Flaky test detected on attempt %d", i+1)
        }
    }
}
```

## CI Integration

### Generate Reports

Debug helpers can generate JSON reports for CI:

```bash
# Run tests with verbose debugging
go test -v ./... 2>&1 | tee test-output.log

# Analyze generated reports
testhelpers.AnalyzeDebugLogs("test-logs/")
```

### Performance Baselines

```go
// Save baseline
testhelpers.SaveBaseline("search_performance.json", baselineData)

// Load baseline in CI
baseline := testhelpers.LoadBaseline("search_performance.json")
current := measurePerformance()
testhelpers.ValidatePerformance(baseline, current, 0.05) // 5% tolerance
```

## Troubleshooting

### Issue: Tests Pass Locally But Fail in CI

1. Enable detailed logging:
   ```go
   helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugTrace)
   ```

2. Check for race conditions:
   ```bash
   go test -race ./...
   ```

3. Verify test isolation:
   ```go
   testhelpers.AssertNoLeaks(t)
   ```

### Issue: High Memory Usage in Tests

1. Monitor memory trends:
   ```go
   helper.CaptureState("start")
   // ... test code ...
   helper.CaptureState("end")
   ```

2. Check for memory leaks:
   ```bash
   go test -memprofile=mem.prof -cpuprofile=cpu.prof
   go tool pprof mem.prof
   ```

### Issue: Slow Test Execution

1. Use appropriate debug level:
   ```go
   // For fast tests
   helper := testhelpers.NewTestDebugHelper(t, testhelpers.DebugNone)
   ```

2. Reduce test scope:
   ```go
   // Use smaller test datasets
   config := testhelpers.createTestConfig(tempDir)
   config.Index.MaxFileCount = 100 // Reduce from 1000
   ```

## See Also

- [Go Testing Package Documentation](https://golang.org/pkg/testing/)
- [Go Leak Detection](https://github.com/uber-go/goleak)
- [Project Testing Strategy](../docs/testing-strategy.md)
- [Performance Guidelines](../docs/performance.md)
