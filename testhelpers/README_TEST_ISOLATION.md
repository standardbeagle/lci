# Test Isolation Guide

This guide provides comprehensive patterns for eliminating shared state between tests and ensuring complete test isolation.

## Problem Statement

The Lightning Code Index project had several shared state issues:

1. **Package-level variables** in test files that persist between tests
2. **Global singletons** that maintain state across test runs
3. **Goroutine leaks** from background processes not properly cleaned up
4. **Memory leaks** from uncleaned resources
5. **Race conditions** from concurrent access to shared state

## Solution Overview

We've implemented a comprehensive test isolation system with these components:

- **IsolatedTestRunner**: Complete test isolation with automatic cleanup
- **TestDataBuilder**: Build isolated test data without shared state
- **TestValidator**: Validate that tests don't leak resources
- **SharedStateConverter**: Convert existing shared tests to isolated ones
- **GlobalStateManager**: Reset all global singletons between tests

## Usage Patterns

### Basic Test Isolation

```go
func TestMyFeature_Isolated(t *testing.T) {
    testhelpers.RunIsolatedTest(t, "MyFeature", func(t *testing.T) {
        // Test code here - completely isolated from other tests
        // No shared state, no global variables, no leaks
    })
}
```

### Parallel Test Isolation

```go
func TestMyFeature_ParallelIsolated(t *testing.T) {
    testhelpers.RunParallelIsolatedTest(t, "MyFeature", func(t *testing.T) {
        // This test runs in parallel with complete isolation
        // Safe to run alongside other tests
    })
}
```

### Test Data Builder Pattern

```go
func TestWithTestData(t *testing.T) {
    testhelpers.RunIsolatedTest(t, "TestData", func(t *testing.T) {
        testData := testhelpers.NewTestDataBuilder().
            AddGoFile("main.go",
                testhelpers.PackageDecl{Name: "main"},
                testhelpers.ImportDecl{Imports: []string{"fmt"}},
                testhelpers.GlobalVar{
                    Name:  "config",
                    Type:  "string",
                    Value: `"production"`,
                },
                testhelpers.FunctionDecl{
                    Name: "main",
                    Body: `fmt.Println("Hello, World!")`,
                    Exported: true,
                },
            ).
            AddFile("config.txt", "debug=true").
            Build()

        // Use testData.FileStore, testData.Files, etc.
        // No shared package-level variables needed
    })
}
```

### Comprehensive Test with Validation

```go
func TestComprehensive_Example(t *testing.T) {
    runner := testhelpers.NewComprehensiveTestRunner(t).
        AddTestData(testhelpers.NewTestDataBuilder().
            AddGoFile("test.go", testhelpers.PackageDecl{Name: "main"}).
            Build()).
        AddValidation(testhelpers.ValidationCheck{
            Name: "Custom Check",
            Check: func() error {
                // Custom validation logic
                return nil
            },
            Critical: true,
        }).
        AddCleanup("Custom cleanup", func() error {
            // Custom cleanup logic
            return nil
        })

    runner.Run(func(testData *testhelpers.TestData, t *testing.T) {
        // Test implementation
        // Automatic validation and cleanup will run
    })
}
```

## Converting Existing Tests

### Before (Shared State)

```go
package search

var globalVar = "test"
var globalCounter = 0

func TestSearch_BadSharedState(t *testing.T) {
    globalCounter++

    fileStore := NewFileContentStore()
    content := []byte(`package main

var globalVar = "test"
func main() {}`)

    fileID := fileStore.LoadFile("test.go", content)

    // Test using globalVar and globalCounter
    // BAD: These persist between tests
}
```

### After (Isolated)

```go
package search

func TestSearch_GoodIsolation(t *testing.T) {
    testhelpers.RunIsolatedTest(t, "Search", func(t *testing.T) {
        // Create isolated test data instead of shared variables
        testData := testhelpers.NewTestDataBuilder().
            AddGoFile("test.go",
                testhelpers.PackageDecl{Name: "main"},
                testhelpers.GlobalVar{
                    Name:  "globalVar",
                    Type:  "string",
                    Value: `"test"`,
                },
                testhelpers.FunctionDecl{
                    Name: "main",
                    Exported: true,
                },
            ).
            Build()

        // Use isolated counter instead of shared global
        localCounter := 0
        localCounter++

        // Test using isolated data
        fileID := testData.GetFile("test.go").ID

        // All state is isolated to this test
    })
}
```

## Global State Reset

The system automatically resets these global singletons:

- `GlobalIndexCoordinator` - Main indexing coordinator
- `GlobalMetricsCollector` - Performance metrics collection
- `GlobalErrorHandler` - Error handling state
- `GlobalIndexSystemLogger` - Logging system
- `GlobalCoordinationHelpers` - Coordination utilities
- `GlobalStringRefAllocator` - String reference management
- `GlobalPerformanceOptimizations` - Performance optimization state

## Validation Features

### Goroutine Leak Detection

Automatically detects if tests leave goroutines running:

```go
validator := testhelpers.NewTestValidator(t)
validator.AddGoroutineCheck()
validator.Validate() // Fails if goroutines leaked
```

### Memory Leak Detection

Monitors memory usage during tests:

```go
validator := testhelpers.NewTestValidator(t)
validator.AddMemoryCheck()
validator.Validate() // Fails if memory usage too high
```

### Custom Validation

Add domain-specific validation:

```go
validator.AddCustomCheck("Index integrity", func() error {
    // Check that index structures are clean
    return nil
}, true) // Critical = true
```

## Best Practices

### 1. Always Use Isolation

```go
// GOOD
func TestFeature(t *testing.T) {
    testhelpers.RunIsolatedTest(t, "Feature", func(t *testing.T) {
        // Test implementation
    })
}

// BAD - Shared state possible
func TestFeature(t *testing.T) {
    // Test implementation without isolation
}
```

### 2. Never Use Package-Level Variables in Tests

```go
// BAD - Shared between tests
var testConfig = Config{Debug: true}

func TestA(t *testing.T) {
    testConfig.Debug = false
}

func TestB(t *testing.T) {
    // testConfig.Debug might be false from TestA
}

// GOOD - Isolated data
func TestA(t *testing.T) {
    testhelpers.RunIsolatedTest(t, "A", func(t *testing.T) {
        config := Config{Debug: true}
        // Use local config
    })
}

func TestB(t *testing.T) {
    testhelpers.RunIsolatedTest(t, "B", func(t *testing.T) {
        config := Config{Debug: true}
        // Fresh config, independent of TestA
    })
}
```

### 3. Clean Up Resources

```go
func TestWithResources(t *testing.T) {
    runner := testhelpers.NewComprehensiveTestRunner(t)

    runner.AddCleanup("Stop background processor", func() error {
        return stopBackgroundProcessor()
    })

    runner.Run(func(testData *testhelpers.TestData, t *testing.T) {
        // Start background processor
        startBackgroundProcessor()

        // Test implementation
        // Cleanup happens automatically
    })
}
```

### 4. Use Parallel Tests Safely

```go
func TestParallelFeature1(t *testing.T) {
    testhelpers.RunParallelIsolatedTest(t, "ParallelFeature1", func(t *testing.T) {
        // Safe to run in parallel
    })
}

func TestParallelFeature2(t *testing.T) {
    testhelpers.RunParallelIsolatedTest(t, "ParallelFeature2", func(t *testing.T) {
        // Safe to run in parallel with TestParallelFeature1
    })
}
```

## Migration Strategy

### Phase 1: Critical Tests
Start with tests that have known race conditions or use shared state heavily.

### Phase 2: High-Traffic Tests
Convert tests that run frequently in CI/CD pipelines.

### Phase 3: All Tests
Eventually migrate all tests to use isolation patterns.

### Automation Tools

Use the SharedStateConverter to help migrate existing tests:

```go
converter := testhelpers.NewSharedStateConverter()

// Generate isolated version of shared content
isolatedBuilder := converter.ConvertSharedStateFile(sharedGoCode, "main")

// Get template for converting test
template := converter.GenerateIsolatedTestTemplate("TestOldName", []string{"sharedVar1", "sharedVar2"})
```

## Performance Considerations

- **Isolation Overhead**: ~1-2ms per test for global state reset
- **Memory Usage**: Slight increase due to isolated test data
- **Parallel Execution**: Safe and recommended for faster test runs
- **Cleanup Time**: ~5-10ms per test for comprehensive cleanup

The overhead is minimal compared to the benefits of reliable, isolated tests.

## Troubleshooting

### Tests Failing with Isolation

1. **Hidden Dependencies**: Tests may depend on shared state they shouldn't
2. **Order Dependencies**: Tests relying on specific execution order
3. **Global Side Effects**: Tests modifying global state without cleanup

### Solutions

1. **Review Test Logic**: Ensure tests are truly independent
2. **Use TestDataBuilder**: Create explicit test data instead of shared state
3. **Add Custom Validation**: Detect specific state leaks
4. **Enable Debug Logging**: Use validator logging to track issues

### Performance Issues

1. **Too Many Validations**: Remove non-critical validation checks
2. **Large Test Data**: Use efficient test data builders
3. **Slow Cleanup**: Optimize or timeout slow cleanup operations

This isolation system ensures that tests are reliable, independent, and free from race conditions while maintaining good performance characteristics.