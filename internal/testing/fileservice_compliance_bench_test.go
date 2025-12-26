package testing

import (
	"testing"
)

// BenchmarkFileServiceCompliance benchmarks the FileService compliance checker performance
func BenchmarkFileServiceCompliance(b *testing.B) {
	checker := NewFileServiceComplianceChecker()
	checker.IgnoreTestFiles = true
	checker.IgnoreToolFiles = true

	// Test critical directories
	criticalDirs := []string{
		"../../internal/indexing",
		"../../internal/core",
		"../../internal/search",
		"../../internal/mcp",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, dir := range criticalDirs {
			err := checker.CheckDirectory(dir)
			if err != nil {
				b.Fatalf("Failed to check directory %s: %v", dir, err)
			}
		}
	}
}

// BenchmarkFileServiceComplianceWithoutCache benchmarks performance without caching
func BenchmarkFileServiceComplianceWithoutCache(b *testing.B) {
	checker := NewFileServiceComplianceChecker()
	checker.IgnoreTestFiles = true
	checker.IgnoreToolFiles = true

	// Disable caching by clearing cache for each iteration
	criticalDirs := []string{
		"../../internal/indexing",
		"../../internal/core",
		"../../internal/search",
		"../../internal/mcp",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Clear cache to simulate old behavior
		checker.ClearCache()

		for _, dir := range criticalDirs {
			err := checker.CheckDirectory(dir)
			if err != nil {
				b.Fatalf("Failed to check directory %s: %v", dir, err)
			}
		}
	}
}

// BenchmarkFileServiceComplianceSingleFile benchmarks single file checking performance
func BenchmarkFileServiceComplianceSingleFile(b *testing.B) {
	checker := NewFileServiceComplianceChecker()
	checker.IgnoreTestFiles = true
	checker.IgnoreToolFiles = true

	// Test a representative file
	testFile := "../../internal/indexing/goroutine_index.go"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := checker.CheckFile(testFile)
		if err != nil {
			b.Fatalf("Failed to check file: %v", err)
		}
	}
}

// BenchmarkFileServicePatternMatching benchmarks just the pattern matching performance
func BenchmarkFileServicePatternMatching(b *testing.B) {
	checker := NewFileServiceComplianceChecker()

	// Pre-load cache with a test file
	testFile := "../../internal/indexing/goroutine_index.go"
	_, _ = checker.CheckFile(testFile)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Test pattern matching on cached content
		for _, pattern := range checker.ForbiddenPatterns {
			if lines, ok := checker.fileContentCache[testFile]; ok {
				for lineNum, line := range lines {
					if pattern.MatchString(line) {
						_ = lineNum // Match found - benchmark doesn't need output
					}
				}
			}
		}
	}
}
