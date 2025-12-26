package indexing

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/config"
	lcittesting "github.com/standardbeagle/lci/internal/testing"
)

// TestGitignoreIntegration_MockFilesystem tests gitignore functionality with mock filesystem
func TestGitignoreIntegration_MockFilesystem(t *testing.T) {
	// Create mock test filesystem
	fs := lcittesting.NewTestFilesystem(t)

	// Set up gitignore patterns
	fs.SetGitignore(
		"node_modules/",
		"*.log",
		"dist/",
		"temp/",
		".env",
	)

	// Add files that should be included
	fs.AddProjectFile("src/app.js", "console.log('app started');")
	fs.AddProjectFile("src/utils.js", "function helper() { return 'help'; }")
	fs.AddProjectFile("README.md", "# Project Documentation")
	fs.AddProjectFile("package.json", `{"name": "test-project"}`)

	// Add files that should be excluded
	fs.AddProjectFile("node_modules/react/index.js", "// React library")
	fs.AddProjectFile("debug.log", "2023-01-01: Application started")
	fs.AddProjectFile("dist/bundle.min.js", "minified code")
	fs.AddProjectFile("temp/cache.tmp", "temporary data")
	fs.AddProjectFile(".env", "SECRET=abc123")

	// Note: This test focuses on gitignore pattern matching with mock filesystem
	// Full integration testing would require injecting the mock filesystem into the scanner

	// Test that files are properly filtered
	testFiles := []struct {
		path     string
		expected bool // true = should be indexed
	}{
		{"src/app.js", true},
		{"src/utils.js", true},
		{"README.md", true},
		{"package.json", true},
		{"node_modules/react/index.js", false},
		{"debug.log", false},
		{"dist/bundle.min.js", false},
		{"temp/cache.tmp", false},
		{".env", false},
	}

	for _, tf := range testFiles {
		t.Run(tf.path, func(t *testing.T) {
			// Test gitignore pattern matching directly
			parser := config.NewGitignoreParser()

			// Manually add patterns from the gitignore content
			patterns := strings.Split(fs.GetGitignoreContent(), "\n")
			for _, pattern := range patterns {
				pattern = strings.TrimSpace(pattern)
				if pattern != "" && !strings.HasPrefix(pattern, "#") {
					parser.AddPattern(pattern)
				}
			}

			relativePath := tf.path
			shouldIgnore := parser.ShouldIgnore(relativePath, false)
			expectedToIndex := !shouldIgnore

			assert.Equal(t, tf.expected, expectedToIndex,
				"File %s should be indexed: %v", tf.path, tf.expected)
		})
	}
}

// TestGitignoreIntegration_RealFilesystem tests gitignore functionality with real filesystem
func TestGitignoreIntegration_RealFilesystem(t *testing.T) {
	// Create isolated test environment
	env := lcittesting.NewIsolatedTestEnv(t,
		"node_modules/",
		"*.log",
		"dist/",
		"temp/",
		".env",
	)

	// Create files that should be included
	env.WriteFile("src/app.js", "console.log('app started');")
	env.WriteFile("src/utils.js", "function helper() { return 'help'; }")
	env.WriteFile("README.md", "# Project Documentation")
	env.WriteFile("package.json", `{"name": "test-project"}`)

	// Create files that should be excluded
	env.MkdirAll("node_modules/react")
	env.WriteFile("node_modules/react/index.js", "// React library")
	env.WriteFile("debug.log", "2023-01-01: Application started")
	env.MkdirAll("dist")
	env.WriteFile("dist/bundle.min.js", "minified code")
	env.MkdirAll("temp")
	env.WriteFile("temp/cache.tmp", "temporary data")
	env.WriteFile(".env", "SECRET=abc123")

	// Test gitignore parsing
	parser := config.NewGitignoreParser()
	require.NoError(t, parser.LoadGitignore(env.TempDir()))

	// Test file filtering
	testFiles := []struct {
		path     string
		expected bool // true = should be ignored (not indexed)
	}{
		{"src/app.js", false},
		{"src/utils.js", false},
		{"README.md", false},
		{"package.json", false},
		{"node_modules/react/index.js", true},
		{"debug.log", true},
		{"dist/bundle.min.js", true},
		{"temp/cache.tmp", true},
		{".env", true},
	}

	for _, tf := range testFiles {
		t.Run(tf.path, func(t *testing.T) {
			shouldIgnore := parser.ShouldIgnore(tf.path, false)
			assert.Equal(t, tf.expected, shouldIgnore,
				"File %s should be ignored: %v", tf.path, tf.expected)
		})
	}
}

// TestGitignoreIntegration_ComplexScenarios tests complex gitignore scenarios
func TestGitignoreIntegration_ComplexScenarios(t *testing.T) {
	t.Run("Mock Filesystem", func(t *testing.T) {
		testComplexGitignoreScenarios_Mock(t)
	})

	t.Run("Real Filesystem", func(t *testing.T) {
		testComplexGitignoreScenarios_Real(t)
	})
}

func testComplexGitignoreScenarios_Mock(t *testing.T) {
	fs := lcittesting.NewTestFilesystem(t)

	// Complex gitignore with negations and nested patterns
	fs.SetGitignore(
		"# Dependencies",
		"node_modules/",
		"",
		"# Build outputs",
		"dist/",
		"build/",
		"*.min.js",
		"",
		"# Logs",
		"*.log",
		"logs/",
		"!logs/important.log",
		"",
		"# Environment",
		".env*",
		"!.env.example",
		"",
		"# Temporary files",
		"temp/",
		"*.tmp",
		"",
		"# Test coverage",
		"coverage/",
		"*.test.js",
		"!unit.test.js",
	)

	// Create complex project structure
	fs.CreateNodeProject() // Uses helper to create realistic structure

	// Complex gitignore with negations and nested patterns (set after project creation)
	fs.SetGitignore(
		"# Dependencies",
		"node_modules/",
		"",
		"# Build outputs",
		"dist/",
		"build/",
		"*.min.js",
		"",
		"# Logs",
		"*.log",
		"logs/",
		"!logs/important.log",
		"",
		"# Environment",
		".env*",
		"!.env.example",
		"",
		"# Temporary files",
		"temp/",
		"*.tmp",
		"",
		"# Test coverage",
		"coverage/",
		"*.test.js",
		"!unit.test.js",
	)

	// Add files that should be handled specially
	fs.AddProjectFile("logs/important.log", "Important log file")
	fs.AddProjectFile("logs/debug.log", "Debug log file")
	fs.AddProjectFile("unit.test.js", "Unit test file")
	fs.AddProjectFile("integration.test.js", "Integration test file")
	fs.AddProjectFile("docs/README.md", "Documentation")

	// Test gitignore parsing
	parser := config.NewGitignoreParser()

	// Manually add patterns from the gitignore content
	patterns := strings.Split(fs.GetGitignoreContent(), "\n")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" && !strings.HasPrefix(pattern, "#") {
			parser.AddPattern(pattern)
		}
	}

	testCases := []struct {
		path     string
		expected bool // should be ignored
		reason   string
	}{
		{"src/app.js", false, "Source file should be included"},
		{"node_modules/express/index.js", true, "Dependencies should be excluded"},
		{"dist/bundle.js", true, "Build output should be excluded"},
		{"build/output.js", true, "Build output should be excluded"},
		{"app.min.js", true, "Minified files should be excluded"},
		{"debug.log", true, "Log files should be excluded"},
		{"logs/debug.log", true, "Log directory should be excluded"},
		{"logs/important.log", false, "Negated important log should be included"},
		{".env.local", true, "Environment files should be excluded"},
		{".env.example", false, "Negated env example should be included"},
		{"temp/cache.tmp", true, "Temp files should be excluded"},
		{"coverage/coverage.out", true, "Coverage reports should be excluded"},
		{"integration.test.js", true, "Test files should be excluded"},
		{"unit.test.js", false, "Negated unit test should be included"},
		{"docs/README.md", false, "Documentation should be included"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			shouldIgnore := parser.ShouldIgnore(tc.path, false)
			assert.Equal(t, tc.expected, shouldIgnore,
				"File %s should be ignored: %v (%s)", tc.path, tc.expected, tc.reason)
		})
	}
}

func testComplexGitignoreScenarios_Real(t *testing.T) {
	env := lcittesting.NewIsolatedTestEnv(t,
		"# Dependencies",
		"node_modules/",
		"",
		"# Build outputs",
		"dist/",
		"build/",
		"*.min.js",
		"",
		"# Logs",
		"*.log",
		"logs/",
		"!logs/important.log",
		"",
		"# Environment",
		".env*",
		"!.env.example",
		"",
		"# Temporary files",
		"temp/",
		"*.tmp",
		"",
		"# Test coverage",
		"coverage/",
		"*.test.js",
		"!unit.test.js",
	)

	// Create complex project structure
	env.CreateRealNodeProject()

	// Add files that should be handled specially
	env.WriteFile("logs/important.log", "Important log file")
	env.WriteFile("logs/debug.log", "Debug log file")
	env.WriteFile("unit.test.js", "Unit test file")
	env.WriteFile("integration.test.js", "Integration test file")
	env.WriteFile("docs/README.md", "Documentation")

	// Test gitignore parsing
	parser := config.NewGitignoreParser()
	require.NoError(t, parser.LoadGitignore(env.TempDir()))

	testCases := []struct {
		path     string
		expected bool // should be ignored
		reason   string
	}{
		{"src/app.js", false, "Source file should be included"},
		{"node_modules/express/index.js", true, "Dependencies should be excluded"},
		{"dist/bundle.js", true, "Build output should be excluded"},
		{"build/output.js", true, "Build output should be excluded"},
		{"app.min.js", true, "Minified files should be excluded"},
		{"debug.log", true, "Log files should be excluded"},
		{"logs/debug.log", true, "Log directory should be excluded"},
		{"logs/important.log", false, "Negated important log should be included"},
		{".env.local", true, "Environment files should be excluded"},
		{".env.example", false, "Negated env example should be included"},
		{"temp/cache.tmp", true, "Temp files should be excluded"},
		{"coverage/coverage.out", true, "Coverage reports should be excluded"},
		{"integration.test.js", true, "Test files should be excluded"},
		{"unit.test.js", false, "Negated unit test should be included"},
		{"docs/README.md", false, "Documentation should be included"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			shouldIgnore := parser.ShouldIgnore(tc.path, false)
			assert.Equal(t, tc.expected, shouldIgnore,
				"File %s should be ignored: %v (%s)", tc.path, tc.expected, tc.reason)
		})
	}
}

// TestGitignoreIntegration_Performance tests performance with realistic file counts
func TestGitignoreIntegration_Performance(t *testing.T) {
	t.Run("Mock Filesystem Performance", func(t *testing.T) {
		testGitignorePerformance_Mock(t)
	})

	t.Run("Real Filesystem Performance", func(t *testing.T) {
		testGitignorePerformance_Real(t)
	})
}

func testGitignorePerformance_Mock(t *testing.T) {
	fs := lcittesting.NewTestFilesystem(t)

	// Set up realistic gitignore patterns
	fs.SetGitignore(
		"node_modules/",
		"*.log",
		"dist/",
		"build/",
		"*.min.js",
		"*.tmp",
		"coverage/",
		"test-results/",
		".DS_Store",
		"*.swp",
	)

	// Create many files (1000 files total)
	fileCount := 1000
	for i := 0; i < fileCount; i++ {
		if i%10 == 0 {
			// 10% should be excluded (node_modules files)
			fs.AddProjectFile(fmt.Sprintf("node_modules/lib%d/index.js", i), fmt.Sprintf("Library %d", i))
		} else if i%20 == 0 {
			// 5% should be excluded (log files)
			fs.AddProjectFile(fmt.Sprintf("logs/debug%d.log", i), fmt.Sprintf("Log %d", i))
		} else if i%30 == 0 {
			// 3% should be excluded (minified files)
			fs.AddProjectFile(fmt.Sprintf("app%d.min.js", i), fmt.Sprintf("Minified %d", i))
		} else {
			// 82% should be included (source files)
			fs.AddProjectFile(fmt.Sprintf("src/file%d.js", i), fmt.Sprintf("Source %d", i))
		}
	}

	// Test gitignore parsing performance
	parser := config.NewGitignoreParser()
	require.NoError(t, parser.LoadGitignore(fs.GetConfig().Project.Root))

	// Measure lookup performance
	start := time.Now()

	lookupCount := 1000
	for i := 0; i < lookupCount; i++ {
		// Test a mix of different file types
		if i%3 == 0 {
			parser.ShouldIgnore(fmt.Sprintf("src/file%d.js", i%100), false)
		} else if i%3 == 1 {
			parser.ShouldIgnore(fmt.Sprintf("node_modules/lib%d/index.js", i%10), false)
		} else {
			parser.ShouldIgnore(fmt.Sprintf("logs/debug%d.log", i%20), false)
		}
	}

	duration := time.Since(start)
	avgTime := duration / time.Duration(lookupCount)

	// Should be very fast (less than 0.1ms per lookup on average)
	assert.Less(t, avgTime, 100*time.Microsecond,
		"Average lookup time should be fast: %v", avgTime)
}

func testGitignorePerformance_Real(t *testing.T) {
	env := lcittesting.NewIsolatedTestEnv(t,
		"node_modules/",
		"*.log",
		"dist/",
		"build/",
		"*.min.js",
		"*.tmp",
		"coverage/",
		"test-results/",
		".DS_Store",
		"*.swp",
	)

	// Create many files (100 files for real filesystem to avoid slow test)
	fileCount := 100
	for i := 0; i < fileCount; i++ {
		if i%10 == 0 {
			// 10% should be excluded (node_modules files)
			env.MkdirAll("node_modules/express")
			env.WriteFile(fmt.Sprintf("node_modules/express/lib%d.js", i), fmt.Sprintf("Library %d", i))
		} else if i%20 == 0 {
			// 5% should be excluded (log files)
			env.WriteFile(fmt.Sprintf("logs/debug%d.log", i), fmt.Sprintf("Log %d", i))
		} else if i%30 == 0 {
			// 3% should be excluded (minified files)
			env.WriteFile(fmt.Sprintf("app%d.min.js", i), fmt.Sprintf("Minified %d", i))
		} else {
			// 82% should be included (source files)
			env.WriteFile(fmt.Sprintf("src/file%d.js", i), fmt.Sprintf("Source %d", i))
		}
	}

	// Test gitignore parsing performance
	parser := config.NewGitignoreParser()
	require.NoError(t, parser.LoadGitignore(env.TempDir()))

	// Measure lookup performance
	start := time.Now()

	lookupCount := 1000
	for i := 0; i < lookupCount; i++ {
		// Test a mix of different file types
		if i%3 == 0 {
			parser.ShouldIgnore(fmt.Sprintf("src/file%d.js", i%100), false)
		} else if i%3 == 1 {
			parser.ShouldIgnore(fmt.Sprintf("node_modules/express/lib%d.js", i%10), false)
		} else {
			parser.ShouldIgnore(fmt.Sprintf("logs/debug%d.log", i%20), false)
		}
	}

	duration := time.Since(start)
	avgTime := duration / time.Duration(lookupCount)

	// Should be very fast (less than 0.1ms per lookup on average)
	assert.Less(t, avgTime, 100*time.Microsecond,
		"Average lookup time should be fast: %v", avgTime)
}