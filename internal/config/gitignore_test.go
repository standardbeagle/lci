package config

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGitignoreParser_BasicPatterns tests fundamental gitignore pattern matching
func TestGitignoreParser_BasicPatterns(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		isDir    bool
		expected bool
	}{
		{
			name:     "Simple file match",
			pattern:  "README.md",
			path:     "README.md",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Simple file no match",
			pattern:  "README.md",
			path:     "main.js",
			isDir:    false,
			expected: false,
		},
		{
			name:     "Directory pattern matches directory",
			pattern:  "node_modules/",
			path:     "node_modules",
			isDir:    true,
			expected: true,
		},
		{
			name:     "Directory pattern matches files inside",
			pattern:  "node_modules/",
			path:     "node_modules/react/index.js",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Directory pattern no match outside",
			pattern:  "node_modules/",
			path:     "src/main.js",
			isDir:    false,
			expected: false,
		},
		{
			name:     "Absolute pattern match",
			pattern:  "/build",
			path:     "build",
			isDir:    true,
			expected: true,
		},
		{
			name:     "Absolute pattern no match subdirectory",
			pattern:  "/build",
			path:     "public/build",
			isDir:    true,
			expected: false,
		},
		{
			name:     "Wildcard pattern match",
			pattern:  "*.min.js",
			path:     "bundle.min.js",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Wildcard pattern no match",
			pattern:  "*.min.js",
			path:     "bundle.js",
			isDir:    false,
			expected: false,
		},
		{
			name:     "Double wildcard pattern",
			pattern:  "**/*.log",
			path:     "logs/app.log",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Double wildcard deep match",
			pattern:  "**/*.log",
			path:     "logs/2023/01/app.log",
			isDir:    false,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewGitignoreParser()
			// Manually add the pattern to test individual pattern matching
			pattern := parser.parsePattern(tt.pattern)

			result := parser.matchesPattern(pattern, tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result, "Pattern: %s, Path: %s, IsDir: %v", tt.pattern, tt.path, tt.isDir)
		})
	}
}

// TestGitignoreParser_ComplexPatterns tests more complex gitignore scenarios
func TestGitignoreParser_ComplexPatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		isDir    bool
		expected bool
	}{
		{
			name:     "Node modules exclusion",
			patterns: []string{"node_modules/"},
			path:     "node_modules/react/index.js",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Multiple patterns - file excluded",
			patterns: []string{"*.log", "*.tmp", "temp/"},
			path:     "debug.log",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Multiple patterns - file not excluded",
			patterns: []string{"*.log", "*.tmp", "temp/"},
			path:     "src/main.js",
			isDir:    false,
			expected: false,
		},
		{
			name:     "Negation pattern - excluded then included",
			patterns: []string{"*.log", "!important.log"},
			path:     "important.log",
			isDir:    false,
			expected: false, // Negation should include the file
		},
		{
			name:     "Negation pattern - different file still excluded",
			patterns: []string{"*.log", "!important.log"},
			path:     "debug.log",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Complex nested path",
			patterns: []string{"dist/**", "build/**"},
			path:     "dist/static/css/main.css",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Hidden directory exclusion",
			patterns: []string{".git/", ".vscode/"},
			path:     ".git/objects/12/3456",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Test directory exclusion",
			patterns: []string{"coverage/", "test-results/"},
			path:     "coverage/coverage.out",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Environment file patterns",
			patterns: []string{".env*", "!.env.example"},
			path:     ".env.local",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Environment file example not excluded",
			patterns: []string{".env*", "!.env.example"},
			path:     ".env.example",
			isDir:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewGitignoreParser()

			// Load all patterns
			for _, pattern := range tt.patterns {
				parser.patterns = append(parser.patterns, parser.parsePattern(pattern))
			}

			result := parser.ShouldIgnore(tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result, "Patterns: %v, Path: %s, IsDir: %v", tt.patterns, tt.path, tt.isDir)
		})
	}
}

// TestGitignoreParser_LoadFromContent tests parsing from string content
func TestGitignoreParser_LoadFromContent(t *testing.T) {
	content := `# Comments should be ignored

node_modules/
*.log
!important.log
build/
.env*
!.env.example
coverage/

# Test files
test-results/
*.test.js
!unit.test.js
`

	parser := NewGitignoreParser()

	// Simulate loading from content by manually adding parsed patterns
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			parser.patterns = append(parser.patterns, parser.parsePattern(line))
		}
	}

	tests := []struct {
		path     string
		isDir    bool
		expected bool
	}{
		{"node_modules/react/index.js", false, true},
		{"debug.log", false, true},
		{"important.log", false, false}, // Should be included by negation
		{"build/bundle.js", false, true},
		{".env.local", false, true},
		{".env.example", false, false}, // Should be included by negation
		{"coverage/coverage.out", false, true},
		{"test-results/junit.xml", false, true},
		{"unit.test.js", false, false}, // Should be included by negation
		{"integration.test.js", false, true},
		{"src/main.js", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := parser.ShouldIgnore(tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result, "Path: %s, IsDir: %v", tt.path, tt.isDir)
		})
	}
}

// TestGitignoreParser_EdgeCases tests edge cases and special scenarios
func TestGitignoreParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		isDir    bool
		expected bool
	}{
		{
			name:     "Empty pattern",
			patterns: []string{""},
			path:     "any-file.txt",
			isDir:    false,
			expected: false,
		},
		{
			name:     "Pattern with only slash",
			patterns: []string{"/"},
			path:     "any-file.txt",
			isDir:    false,
			expected: false,
		},
		{
			name:     "Pattern with dots",
			patterns: []string{".DS_Store"},
			path:     ".DS_Store",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Pattern with special characters",
			patterns: []string{"*.tmp?"},
			path:     "temp.tmp1",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Very deep nesting",
			patterns: []string{"deep/nested/structure/"},
			path:     "deep/nested/structure/file.txt",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Directory with spaces",
			patterns: []string{"my folder/"},
			path:     "my folder/file.txt",
			isDir:    false,
			expected: true,
		},
		{
			name:     "Case sensitivity test",
			patterns: []string{"README.md"},
			path:     "readme.md",
			isDir:    false,
			expected: false, // Gitignore is case-sensitive on most systems
		},
		{
			name:     "Unicode characters",
			patterns: []string{"*.日志"},
			path:     "application.日志",
			isDir:    false,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewGitignoreParser()

			// Load all patterns
			for _, pattern := range tt.patterns {
				parser.patterns = append(parser.patterns, parser.parsePattern(pattern))
			}

			result := parser.ShouldIgnore(tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result, "Patterns: %v, Path: %s, IsDir: %v", tt.patterns, tt.path, tt.isDir)
		})
	}
}

// TestGitignoreParser_GetExclusionPatterns tests conversion to LCI patterns
func TestGitignoreParser_GetExclusionPatterns(t *testing.T) {
	parser := NewGitignoreParser()

	// Add some test patterns
	testPatterns := []string{
		"node_modules/",
		"*.log",
		"dist/",
		".DS_Store",
		"!important.log",
	}

	for _, pattern := range testPatterns {
		parser.patterns = append(parser.patterns, parser.parsePattern(pattern))
	}

	exclusions := parser.GetExclusionPatterns()

	// Should not include negation patterns
	for _, exclusion := range exclusions {
		assert.False(t, strings.HasPrefix(exclusion, "!"), "Exclusion should not include negation: %s", exclusion)
	}

	// Should have converted patterns appropriately
	expectedExclusions := []string{
		"**/node_modules/**",
		"**/*.log",
		"**/dist/**",
		"**/.DS_Store",
	}

	// Check that expected patterns are present (order may vary)
	patternMap := make(map[string]bool)
	for _, pattern := range exclusions {
		patternMap[pattern] = true
	}

	for _, expected := range expectedExclusions {
		assert.True(t, patternMap[expected], "Expected exclusion pattern not found: %s", expected)
	}
}

// TestGitignoreParser_Performance tests performance with large pattern sets
func TestGitignoreParser_Performance(t *testing.T) {
	parser := NewGitignoreParser()

	// Add a realistic number of patterns (not too many)
	for i := 0; i < 100; i++ {
		pattern := fmt.Sprintf("*.test%d", i)
		parser.patterns = append(parser.patterns, parser.parsePattern(pattern))
	}

	// Test lookup performance
	start := time.Now()

	// Test multiple lookups
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("file.test%d", i%100) // Cycle through the patterns
		parser.ShouldIgnore(path, false)
	}

	duration := time.Since(start)

	// Should complete quickly (less than 500ms for 1000 lookups)
	// Using 500ms to accommodate slower CI environments
	assert.Less(t, duration, 500*time.Millisecond, "Gitignore lookup should be fast")
}

// TestGitignoreParser_NegationPriority tests that later negations override earlier patterns
func TestGitignoreParser_NegationPriority(t *testing.T) {
	parser := NewGitignoreParser()

	// Add patterns where negation should override
	patterns := []string{
		"*.log",
		"!important.log",
		"!debug.log",
	}

	for _, pattern := range patterns {
		parser.patterns = append(parser.patterns, parser.parsePattern(pattern))
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"app.log", true},       // Should be excluded
		{"important.log", false}, // Should be included by negation
		{"debug.log", false},     // Should be included by negation
		{"error.log", true},      // Should be excluded
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := parser.ShouldIgnore(tt.path, false)
			assert.Equal(t, tt.expected, result, "Path: %s", tt.path)
		})
	}
}

// BenchmarkGitignoreParsing benchmarks gitignore parsing performance
func BenchmarkGitignoreParsing(b *testing.B) {
	content := `
# Common gitignore patterns
node_modules/
*.log
dist/
build/
coverage/
*.tmp
.DS_Store
.vscode/
.idea/
*.swp
*.swo
*~

# Environment files
.env*
!.env.example

# Test files
*.test.js
*.spec.js
test-results/
coverage/

# OS specific
Thumbs.db
ehthumbs.db
Desktop.ini
`

	patterns := strings.Split(content, "\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewGitignoreParser()
		parsedPatterns := make([]GitignorePattern, 0)
		for _, line := range patterns {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				parsedPatterns = append(parsedPatterns, parser.parsePattern(line))
			}
		}
		// Prevent compiler optimization from removing the benchmark work
		if len(parsedPatterns) == 0 {
			b.StopTimer()
			b.Log("unexpected: no patterns parsed")
			b.StartTimer()
		}
	}
}

// BenchmarkGitignoreLookup benchmarks gitignore lookup performance
func BenchmarkGitignoreLookup(b *testing.B) {
	parser := NewGitignoreParser()

	// Add common patterns
	patterns := []string{
		"node_modules/",
		"*.log",
		"dist/",
		"*.tmp",
		"coverage/",
		".DS_Store",
		"*.swp",
	}

	for _, pattern := range patterns {
		parser.patterns = append(parser.patterns, parser.parsePattern(pattern))
	}

	testPaths := []string{
		"src/main.js",
		"node_modules/react/index.js",
		"debug.log",
		"dist/bundle.js",
		"temp.tmp",
		"coverage/coverage.out",
		".DS_Store",
		"README.md",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			parser.ShouldIgnore(path, false)
		}
	}
}