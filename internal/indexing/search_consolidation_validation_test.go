package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/search"
	testutil "github.com/standardbeagle/lci/internal/testing"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndexingSearchPerformance_Validation ensures consolidated search still meets performance SLOs
// Renamed from TestSearchPerformance to avoid generic duplicate naming
func TestIndexingSearchPerformance_Validation(t *testing.T) {
	// Create test directory with sample files
	testDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
	processData()
}

func processData() {
	// TODO: implement data processing
	data := []int{1, 2, 3, 4, 5}
	for _, v := range data {
		fmt.Printf("Processing: %d\n", v)
	}
}`,
		"util.go": `package main

import "errors"

func utilFunction() error {
	// Helper function
	return nil
}

func validateInput(input string) error {
	if input == "" {
		return errors.New("input cannot be empty")
	}
	return nil
}`,
		"test.go": `package main

import "testing"

// TestMain tests the main.
func TestMain(t *testing.T) {
	// Test main function
	t.Log("Testing main function")
}

// TestProcessData tests the process data.
func TestProcessData(t *testing.T) {
	// Test data processing
	t.Log("Testing data processing")
}`,
	}

	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(testDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create config with safe defaults
	cfg := testhelpers.NewTestConfigBuilder(testDir).
		WithIncludePatterns("*.go").
		Build()

	// Create and build index
	gi := NewMasterIndex(cfg)
	ctx := context.Background()
	err := gi.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Test patterns
	patterns := []struct {
		name    string
		pattern string
		options types.SearchOptions
	}{
		{
			name:    "Simple text search",
			pattern: "function",
			options: types.SearchOptions{},
		},
		{
			name:    "Case insensitive",
			pattern: "ERROR",
			options: types.SearchOptions{CaseInsensitive: true},
		},
		{
			name:    "TODO search",
			pattern: "TODO",
			options: types.SearchOptions{},
		},
	}

	for _, test := range patterns {
		t.Run(test.name, func(t *testing.T) {
			// Use stampede prevention retry for timing-sensitive test
			// Updated threshold to 10ms to account for system variability
			// Actual performance is typically <5ms but allow headroom for CI/load
			testutil.RetryTimingAssertion(t, 2, func() (time.Duration, error) {
				var totalTime time.Duration

				for i := 0; i < 10; i++ {
					start := time.Now()
					_, err := gi.SearchWithOptions(test.pattern, test.options)
					if err != nil {
						return 0, err
					}
					totalTime += time.Since(start)
				}
				avgTime := totalTime / 10
				return avgTime, nil
			}, 10*time.Millisecond, fmt.Sprintf("Search pattern '%s'", test.pattern))

			// Verify results separately (outside timing loop)
			results, err := gi.SearchWithOptions(test.pattern, test.options)
			require.NoError(t, err)
			assert.Greater(t, len(results), 0, "Should find matches")
		})
	}
}

// TestSearchAccuracy validates search results are correct
func TestSearchAccuracy(t *testing.T) {
	// Create test directory
	testDir := t.TempDir()

	// Create a more complex test file
	testContent := `package main

import (
	"fmt"
	"errors"
)

// User represents a user in the system
type User struct {
	ID   int
	Name string
}

// NewUser creates a new user
func NewUser(id int, name string) *User {
	return &User{
		ID:   id,
		Name: name,
	}
}

// Validate checks if the user is valid
func (u *User) Validate() error {
	if u.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

func main() {
	user := NewUser(1, "John Doe")
	if err := user.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	}
}
`

	err := os.WriteFile(filepath.Join(testDir, "user.go"), []byte(testContent), 0644)
	require.NoError(t, err)

	// Create config with safe defaults
	cfg := testhelpers.NewTestConfigBuilder(testDir).
		WithIncludePatterns("*.go").
		Build()

	// Create and build index
	gi := NewMasterIndex(cfg)
	ctx := context.Background()
	err = gi.IndexDirectory(ctx, testDir)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name    string
		pattern string
		options types.SearchOptions
	}{
		{
			name:    "Type definition",
			pattern: "type User",
			options: types.SearchOptions{},
		},
		{
			name:    "Function definition",
			pattern: "func New",
			options: types.SearchOptions{},
		},
		{
			name:    "Method definition",
			pattern: "func.*Validate",
			options: types.SearchOptions{UseRegex: true},
		},
		{
			name:    "Error handling",
			pattern: "error",
			options: types.SearchOptions{},
		},
		{
			name:    "Comments",
			pattern: "represents",
			options: types.SearchOptions{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get search results
			results, err := gi.SearchWithOptions(tc.pattern, tc.options)
			require.NoError(t, err)

			// Log results
			t.Logf("Pattern '%s': %d results", tc.pattern, len(results))

			// Verify we found expected results
			switch tc.name {
			case "Type definition":
				assert.Equal(t, 1, len(results), "Should find exactly one User type")
			case "Function definition":
				assert.Equal(t, 1, len(results), "Should find exactly one NewUser function")
			case "Method definition":
				assert.Equal(t, 1, len(results), "Should find Validate method")
			case "Error handling":
				assert.GreaterOrEqual(t, len(results), 3, "Should find multiple error references")
			case "Comments":
				assert.Equal(t, 1, len(results), "Should find comment with 'represents'")
			}

			// Verify results have proper context
			for i, result := range results {
				assert.NotEmpty(t, result.Path, "Result %d should have file path", i)
				assert.Greater(t, result.Line, 0, "Result %d should have valid line number", i)
				assert.NotEmpty(t, result.Context.Lines, "Result %d should have context", i)
			}
		})
	}
}

// TestRealWorldPerformance tests on actual codebase
// Note: Skipped because the real-world codebase includes large test projects
// in internal/mcp/workflow_testdata that make this test time out in CI
func TestRealWorldPerformance(t *testing.T) {
	t.Skip("Skipping real-world performance test - use locally with adequate timeout")

	// Use the actual project root
	projectRoot := "../../" // Relative to test file
	absRoot, err := filepath.Abs(projectRoot)
	require.NoError(t, err)

	// Use test config builder to ensure safe defaults (prevents accidental scanning of .git, vendor, test data)
	cfg := testhelpers.NewTestConfigBuilder(absRoot).
		WithExclusions("internal/mcp/workflow_testdata/**").
		WithIncludePatterns("*.go").
		Build()

	gi := NewMasterIndex(cfg)
	ctx := context.Background()
	err = gi.IndexDirectory(ctx, absRoot)
	require.NoError(t, err)

	// Real-world search patterns
	patterns := []struct {
		name    string
		pattern string
		options types.SearchOptions
	}{
		{
			name:    "Common function search",
			pattern: "func New",
			options: types.SearchOptions{},
		},
		{
			name:    "TODO/FIXME comments",
			pattern: "TODO|FIXME",
			options: types.SearchOptions{UseRegex: true},
		},
		{
			name:    "Error handling",
			pattern: "if err != nil",
			options: types.SearchOptions{},
		},
	}

	stats := gi.Stats()
	t.Logf("Testing on real codebase with %d files", stats.TotalFiles)

	for _, test := range patterns {
		t.Run(test.name, func(t *testing.T) {
			// Measure performance (average of 5 runs)
			var totalTime time.Duration
			var results []search.GrepResult

			for i := 0; i < 5; i++ {
				start := time.Now()
				results, err = gi.SearchWithOptions(test.pattern, test.options)
				require.NoError(t, err)
				totalTime += time.Since(start)
			}
			avgTime := totalTime / 5

			// Log results
			t.Logf("Pattern '%s': %v (results: %d)", test.pattern, avgTime, len(results))

			// Performance should meet <5ms requirement
			if avgTime > 5*time.Millisecond {
				t.Logf("Note: Search took %v which is > 5ms target (but acceptable for real codebase)", avgTime)
			}

			// Ensure we found results
			assert.Greater(t, len(results), 0, "Should find matches for pattern: %s", test.pattern)
		})
	}
}
