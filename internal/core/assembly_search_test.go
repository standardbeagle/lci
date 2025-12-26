package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/standardbeagle/lci/internal/types"
)

// TestAssemblySearchEngine_FragmentString tests the assembly search engine fragment string.
func TestAssemblySearchEngine_FragmentString(t *testing.T) {
	ase := &AssemblySearchEngine{
		minFragmentLength: 4,
	}

	tests := []struct {
		name      string
		pattern   string
		minLength int
		expected  []string
	}{
		{
			name:      "Error message with colon separator",
			pattern:   "Error: user not found",
			minLength: 4,
			expected:  []string{"Error", "user", "found", "user not found"},
		},
		{
			name:      "Log message with multiple parts",
			pattern:   "User alice logged in successfully at 2024-08-14",
			minLength: 4,
			expected:  []string{"User", "alice", "logged", "successfully", "2024-08-14"},
		},
		{
			name:      "API path with segments",
			pattern:   "/api/v1/users/123/profile",
			minLength: 4,
			expected:  []string{"users", "profile"},
		},
		{
			name:      "SQL query pattern",
			pattern:   "SELECT * FROM users WHERE id = 42",
			minLength: 4,
			expected:  []string{"SELECT", "FROM", "users", "WHERE"},
		},
		{
			name:      "Short fragments filtered",
			pattern:   "a b c test",
			minLength: 4,
			expected:  []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragments := ase.fragmentString(tt.pattern, tt.minLength)

			// Check that we get reasonable fragments
			assert.NotEmpty(t, fragments, "Should find some fragments")

			// Check minimum length constraint
			for _, frag := range fragments {
				assert.GreaterOrEqual(t, len(frag), tt.minLength,
					"Fragment '%s' should meet minimum length", frag)
			}

			// Check for expected key fragments
			for _, expected := range tt.expected {
				found := false
				for _, frag := range fragments {
					if frag == expected {
						found = true
						break
					}
				}
				if len(expected) >= tt.minLength {
					assert.True(t, found, "Should find fragment '%s'", expected)
				}
			}
		})
	}
}

// TestAssemblySearchEngine_CalculateCoverage tests the assembly search engine calculate coverage.
func TestAssemblySearchEngine_CalculateCoverage(t *testing.T) {
	ase := &AssemblySearchEngine{}

	tests := []struct {
		name      string
		fragments []Fragment
		target    string
		expected  float64
	}{
		{
			name: "Full coverage",
			fragments: []Fragment{
				{Text: "Error: "},
				{Text: "user not found"},
			},
			target:   "Error: user not found",
			expected: 1.0,
		},
		{
			name: "Partial coverage",
			fragments: []Fragment{
				{Text: "Error"},
				{Text: "not found"},
			},
			target:   "Error: user not found",
			expected: float64(len("Error")+len("not found")) / float64(len("Error: user not found")),
		},
		{
			name: "No coverage",
			fragments: []Fragment{
				{Text: "Warning"},
				{Text: "system"},
			},
			target:   "Error: user not found",
			expected: 0.0,
		},
		{
			name: "Overlapping fragments",
			fragments: []Fragment{
				{Text: "Error message"},
				{Text: "message: critical"},
			},
			target:   "Error message: critical",
			expected: 1.0, // Should not over-count
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coverage := ase.calculateCoverage(tt.fragments, tt.target)
			assert.InDelta(t, tt.expected, coverage, 0.01,
				"Coverage should be approximately %.2f", tt.expected)
		})
	}
}

// TestAssemblySearchEngine_HasSignificantWords tests the assembly search engine has significant words.
func TestAssemblySearchEngine_HasSignificantWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Contains error",
			text:     "Error occurred",
			expected: true,
		},
		{
			name:     "Contains warning",
			text:     "Warning: low memory",
			expected: true,
		},
		{
			name:     "Contains database",
			text:     "database connection",
			expected: true,
		},
		{
			name:     "No significant words",
			text:     "foo bar baz",
			expected: false,
		},
		{
			name:     "Case insensitive",
			text:     "ERROR MESSAGE",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSignificantWords(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAssemblySearchEngine_CalculateProximityBoost tests the assembly search engine calculate proximity boost.
func TestAssemblySearchEngine_CalculateProximityBoost(t *testing.T) {
	ase := &AssemblySearchEngine{}

	tests := []struct {
		name      string
		fragments []Fragment
		expected  float64
		desc      string
	}{
		{
			name: "Same function",
			fragments: []Fragment{
				{Text: "Error", SymbolContext: "handleRequest", Location: types.SymbolLocation{Line: 10}},
				{Text: "not found", SymbolContext: "handleRequest", Location: types.SymbolLocation{Line: 11}},
			},
			expected: 2.0,
			desc:     "Fragments in same function should get high boost",
		},
		{
			name: "Close lines",
			fragments: []Fragment{
				{Text: "Error", Location: types.SymbolLocation{Line: 10}},
				{Text: "not found", Location: types.SymbolLocation{Line: 12}},
			},
			expected: 1.5,
			desc:     "Fragments on nearby lines should get medium boost",
		},
		{
			name: "Same region",
			fragments: []Fragment{
				{Text: "Error", Location: types.SymbolLocation{Line: 10}},
				{Text: "not found", Location: types.SymbolLocation{Line: 25}},
			},
			expected: 1.2,
			desc:     "Fragments in same region should get small boost",
		},
		{
			name: "Distant fragments",
			fragments: []Fragment{
				{Text: "Error", Location: types.SymbolLocation{Line: 10}},
				{Text: "not found", Location: types.SymbolLocation{Line: 100}},
			},
			expected: 1.0,
			desc:     "Distant fragments should get no boost",
		},
		{
			name: "Single fragment",
			fragments: []Fragment{
				{Text: "Error message"},
			},
			expected: 1.0,
			desc:     "Single fragment should get no proximity boost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boost := ase.calculateProximityBoost(tt.fragments)
			assert.Equal(t, tt.expected, boost, tt.desc)
		})
	}
}

// TestAssemblySearchEngine_DetectPattern tests the assembly search engine detect pattern.
func TestAssemblySearchEngine_DetectPattern(t *testing.T) {
	ase := &AssemblySearchEngine{}

	tests := []struct {
		name      string
		fragments []Fragment
		expected  string
	}{
		{
			name: "Single literal",
			fragments: []Fragment{
				{Text: "Error: not found"},
			},
			expected: "literal",
		},
		{
			name: "Consecutive lines concatenation",
			fragments: []Fragment{
				{Text: "Error:", Location: types.SymbolLocation{Line: 10}},
				{Text: "user", Location: types.SymbolLocation{Line: 11}},
				{Text: "not found", Location: types.SymbolLocation{Line: 12}},
			},
			expected: "concat",
		},
		{
			name: "Non-consecutive fragments",
			fragments: []Fragment{
				{Text: "Error", Location: types.SymbolLocation{Line: 10}},
				{Text: "not found", Location: types.SymbolLocation{Line: 20}},
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := ase.detectPattern(tt.fragments)
			assert.Equal(t, tt.expected, pattern)
		})
	}
}

// TestAssemblySearchEngine_ScoreAssemblies tests the assembly search engine score assemblies.
func TestAssemblySearchEngine_ScoreAssemblies(t *testing.T) {
	ase := &AssemblySearchEngine{}

	results := []AssemblyResult{
		{
			Coverage: 0.9,
			Pattern:  "concat",
			Fragments: []Fragment{
				{Text: "Error", SymbolContext: "handleError"},
				{Text: "not found", SymbolContext: "handleError"},
			},
		},
		{
			Coverage: 0.7,
			Pattern:  "format",
			Fragments: []Fragment{
				{Text: "User"},
				{Text: "logged in"},
			},
		},
		{
			Coverage: 0.8,
			Pattern:  "unknown",
			Fragments: []Fragment{
				{Text: "Part1"},
				{Text: "Part2"},
				{Text: "Part3"},
				{Text: "Part4"},
				{Text: "Part5"},
				{Text: "Part6"}, // Too many fragments
			},
		},
	}

	ase.scoreAssemblies(results)

	// First result should score highest (high coverage, concat pattern, same function)
	assert.Greater(t, results[0].Score, results[1].Score,
		"High coverage concat in same function should score highest")

	// Format pattern should get boost
	assert.Greater(t, results[1].Score, 0.0,
		"Format pattern should have positive score")

	// Too many fragments should be penalized
	assert.Less(t, results[2].Score, 100.0,
		"Too many fragments should be penalized")
}

// Integration test with mock data
func TestAssemblySearchEngine_Search_Integration(t *testing.T) {
	// This would require mocking the trigram index and other dependencies
	// For now, we'll skip the integration test as it requires full setup
	t.Skip("Integration test requires full index setup")
}

// Benchmark fragment discovery
func BenchmarkFragmentString(b *testing.B) {
	ase := &AssemblySearchEngine{
		minFragmentLength: 4,
	}

	pattern := "Error: Failed to connect to database server at localhost:5432 - connection timeout after 30 seconds"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ase.fragmentString(pattern, 4)
	}
}

// Benchmark coverage calculation
func BenchmarkCalculateCoverage(b *testing.B) {
	ase := &AssemblySearchEngine{}

	fragments := []Fragment{
		{Text: "Error"},
		{Text: "Failed to connect"},
		{Text: "database server"},
		{Text: "connection timeout"},
	}
	target := "Error: Failed to connect to database server at localhost:5432 - connection timeout"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ase.calculateCoverage(fragments, target)
	}
}
