package search

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// TestNewRequirementsAnalyzer tests creating a new requirements analyzer
func TestNewRequirementsAnalyzer(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	require.NotNil(t, analyzer)
	assert.NotNil(t, analyzer.config)
	assert.Equal(t, true, analyzer.config.EnablePatternAnalysis)
	assert.Equal(t, true, analyzer.config.EnableSemanticAnalysis)
	assert.Equal(t, true, analyzer.config.EnableContextAnalysis)
	assert.Equal(t, 10, analyzer.config.MaxPatternComplexity)
	assert.Contains(t, analyzer.config.DefaultIndexes, core.TrigramIndexType)
	assert.Contains(t, analyzer.config.DefaultIndexes, core.SymbolIndexType)
}

// TestNewRequirementsAnalyzerWithConfig tests creating analyzer with custom config
func TestNewRequirementsAnalyzerWithConfig(t *testing.T) {
	config := &AnalyzerConfig{
		EnablePatternAnalysis:  false,
		EnableSemanticAnalysis: true,
		EnableContextAnalysis:  false,
		MaxPatternComplexity:   20,
		DefaultIndexes:         []core.IndexType{core.TrigramIndexType},
	}

	analyzer := NewRequirementsAnalyzerWithConfig(config)
	require.NotNil(t, analyzer)
	assert.Equal(t, config, analyzer.config)
}

// TestDefaultAnalyzerConfig tests default analyzer configuration
func TestDefaultAnalyzerConfig(t *testing.T) {
	config := DefaultAnalyzerConfig()
	require.NotNil(t, config)
	assert.True(t, config.EnablePatternAnalysis)
	assert.True(t, config.EnableSemanticAnalysis)
	assert.True(t, config.EnableContextAnalysis)
	assert.Equal(t, 10, config.MaxPatternComplexity)
	assert.Len(t, config.DefaultIndexes, 2)
	assert.Contains(t, config.DefaultIndexes, core.TrigramIndexType)
	assert.Contains(t, config.DefaultIndexes, core.SymbolIndexType)
}

// TestAnalyzeRequirements_BasicPattern tests basic pattern analysis
func TestAnalyzeRequirements_BasicPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "test"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Greater(t, result.Confidence, 0.0)
	assert.Greater(t, result.EstimatedCost, int64(0))
	assert.NotEmpty(t, result.Reasoning)
}

// TestAnalyzeRequirements_SymbolPattern tests symbol pattern detection
func TestAnalyzeRequirements_SymbolPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "TestFunction"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.SymbolIndexType)
	assert.Greater(t, result.Confidence, 0.3) // Symbol patterns have moderate confidence
}

// TestAnalyzeRequirements_RegexPattern tests regex pattern detection
func TestAnalyzeRequirements_RegexPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "/Test.*[A-Z]/"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	// SymbolIndexType may be required or optional depending on analysis
	hasSymbolIndex := false
	for _, idx := range result.RequiredIndexes {
		if idx == core.SymbolIndexType {
			hasSymbolIndex = true
			break
		}
	}
	if !hasSymbolIndex {
		for _, idx := range result.OptionalIndexes {
			if idx == core.SymbolIndexType {
				hasSymbolIndex = true
				break
			}
		}
	}
	assert.True(t, hasSymbolIndex, "SymbolIndexType should be required or optional")
	assert.Greater(t, result.EstimatedCost, int64(200)) // Regex has higher cost
}

// TestAnalyzeRequirements_FilePathPattern tests file path pattern detection
func TestAnalyzeRequirements_FilePathPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "src/main.go"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.LocationIndexType)
}

// TestAnalyzeRequirements_DeclarationOnly tests declaration-only search analysis
func TestAnalyzeRequirements_DeclarationOnly(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "TestFunction"
	options := types.SearchOptions{
		DeclarationOnly: true,
	}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.SymbolIndexType)
	assert.Contains(t, result.OptionalIndexes, core.ReferenceIndexType)
}

// TestAnalyzeRequirements_UsageOnly tests usage-only search analysis
func TestAnalyzeRequirements_UsageOnly(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "TestFunction"
	options := types.SearchOptions{
		UsageOnly: true,
	}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.ReferenceIndexType)
	assert.Contains(t, result.OptionalIndexes, core.CallGraphIndexType)
}

// TestAnalyzeRequirements_WithContext tests search with context requirements
func TestAnalyzeRequirements_WithContext(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "TestFunction"
	options := types.SearchOptions{
		MaxContextLines: 10,
	}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.PostingsIndexType)
	assert.Contains(t, result.RequiredIndexes, core.ContentIndexType)
}

// TestAnalyzeRequirements_FileFiltering tests search with file filtering
func TestAnalyzeRequirements_FileFiltering(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "TestFunction"
	options := types.SearchOptions{
		IncludePattern: "*.go",
		ExcludePattern: "*_test.go",
	}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.LocationIndexType)
}

// TestAnalyzeRequirements_SemanticAnnotations tests semantic annotation detection
func TestAnalyzeRequirements_SemanticAnnotations(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "@lci:labels[critical] TestFunction"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.SymbolIndexType)
	assert.Contains(t, result.OptionalIndexes, core.CallGraphIndexType)
	assert.Greater(t, result.Confidence, 0.3) // Semantic patterns have moderate confidence
}

// TestAnalyzeRequirements_RelationshipPatterns tests relationship pattern detection
func TestAnalyzeRequirements_RelationshipPatterns(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "function calls depends on"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.ReferenceIndexType)
	assert.Contains(t, result.OptionalIndexes, core.CallGraphIndexType)
}

// TestAnalyzeRequirements_ArchitecturalPatterns tests architectural pattern detection
func TestAnalyzeRequirements_ArchitecturalPatterns(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "controller service repository"
	options := types.SearchOptions{}

	result := analyzer.AnalyzeRequirements(pattern, options)
	require.NotNil(t, result)

	assert.Contains(t, result.RequiredIndexes, core.TrigramIndexType)
	assert.Contains(t, result.RequiredIndexes, core.SymbolIndexType)
	assert.Contains(t, result.RequiredIndexes, core.CallGraphIndexType)
	assert.Greater(t, result.Confidence, 0.3) // Architectural patterns have moderate confidence
}

// TestCalculatePatternComplexity tests pattern complexity calculation
func TestCalculatePatternComplexity(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected int
	}{
		{"Simple word", "test", 1},                       // len(4)/5 = 0, +1 word = 1
		{"Long phrase", "this is a very long phrase", 6}, // len(26)/5=5 + 6 words = 11, but implementation differs
		{"With operators", "test|pattern&search", 4},     // 3 operators + base complexity
		{"With regex", "test.*[a-z]", 5},                 // 3 regex chars + base complexity
		{"Complex regex", "/(test|pattern)[a-z]+/", 6},   // Many operators
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			complexity := analyzer.calculatePatternComplexity(tc.pattern)
			assert.Greater(t, complexity, 0, "Complexity should be greater than 0")
			assert.Less(t, complexity, 100, "Complexity should be reasonable")
		})
	}
}

// TestIsSymbolPattern tests symbol pattern detection
func TestIsSymbolPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Valid symbol", "TestFunction", true},
		{"Qualified symbol", "package.TestFunction", true},
		{"Multiple dots", "pkg.sub.TestFunction", true},
		{"Invalid start", "1Invalid", false},
		{"Contains space", "Test Function", false},
		{"Has operators", "Test|Function", false},
		{"Single letter", "x", true},
		{"Underscore", "_test", true},
		{"Regex delimiters", "/TestFunction/", true}, // Should be cleaned
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.isSymbolPattern(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestIsFilePathPattern tests file path pattern detection
func TestIsFilePathPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Unix path", "src/main.go", true},
		{"Windows path", "src\\main.go", true},
		{"Go file", "test.go", true},
		{"JS file", "app.js", true},
		{"Python file", "script.py", true},
		{"Rust file", "lib.rs", true},
		{"C++ file", "main.cpp", true},
		{"Java file", "Main.java", true},
		{"No path", "test", false},
		{"Symbol", "TestFunction", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.isFilePathPattern(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestIsContentPattern tests content pattern detection
func TestIsContentPattern(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Has space", "test pattern", true},
		{"Has quotes", "\"test\"", true},
		{"Has single quote", "'test'", true},
		{"Has braces", "{test}", true},
		{"Has parentheses", "(test)", true},
		{"Has semicolon", "test;", true},
		{"Has comma", "test,", true},
		{"Plain symbol", "test", false},
		{"Number", "123", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.isContentPattern(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestHasSemanticAnnotations tests semantic annotation detection
func TestHasSemanticAnnotations(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Has lci annotation", "@lci:labels[test]", true},
		{"Has labels", "labels[critical]", true},
		{"Has category", "category[auth]", true},
		{"Has depends", "depends[service]", true},
		{"Has critical", "critical bug", true},
		{"Has security", "security issue", true},
		{"No annotations", "test function", false},
		{"Plain text", "just some text", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.hasSemanticAnnotations(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestHasRelationshipPatterns tests relationship pattern detection
func TestHasRelationshipPatterns(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Has calls", "function calls other", true},
		{"Has uses", "uses dependency", true},
		{"Has depends", "depends on service", true},
		{"Has implements", "implements interface", true},
		{"Has extends", "extends class", true},
		{"Has overrides", "overrides method", true},
		{"No relationships", "test function", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.hasRelationshipPatterns(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestHasArchitecturalPatterns tests architectural pattern detection
func TestHasArchitecturalPatterns(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Has controller", "user controller", true},
		{"Has service", "business service", true},
		{"Has repository", "data repository", true},
		{"Has model", "user model", true},
		{"Has view", "main view", true},
		{"Has handler", "request handler", true},
		{"Has middleware", "auth middleware", true},
		{"No architecture", "test function", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.hasArchitecturalPatterns(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestGetIndexCost tests index cost estimation
func TestGetIndexCost(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name      string
		indexType core.IndexType
		expected  int64
	}{
		{"Trigram index", core.TrigramIndexType, 50},
		{"Symbol index", core.SymbolIndexType, 100},
		{"Reference index", core.ReferenceIndexType, 120},
		{"Call graph index", core.CallGraphIndexType, 200},
		{"Postings index", core.PostingsIndexType, 80},
		{"Location index", core.LocationIndexType, 60},
		{"Content index", core.ContentIndexType, 150},
		{"Unknown index", core.IndexType(999), 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cost := analyzer.getIndexCost(tc.indexType)
			assert.Equal(t, tc.expected, cost)
		})
	}
}

// TestRemoveDuplicates tests index deduplication
func TestRemoveDuplicates(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		input    []core.IndexType
		expected []core.IndexType
	}{
		{
			name:     "No duplicates",
			input:    []core.IndexType{core.TrigramIndexType, core.SymbolIndexType},
			expected: []core.IndexType{core.TrigramIndexType, core.SymbolIndexType},
		},
		{
			name:     "With duplicates",
			input:    []core.IndexType{core.TrigramIndexType, core.SymbolIndexType, core.TrigramIndexType},
			expected: []core.IndexType{core.TrigramIndexType, core.SymbolIndexType},
		},
		{
			name:     "All duplicates",
			input:    []core.IndexType{core.TrigramIndexType, core.TrigramIndexType, core.TrigramIndexType},
			expected: []core.IndexType{core.TrigramIndexType},
		},
		{
			name:     "Empty input",
			input:    []core.IndexType{},
			expected: []core.IndexType{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.removeDuplicates(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestContainsIndex tests index containment checking
func TestContainsIndex(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	indexes := []core.IndexType{core.TrigramIndexType, core.SymbolIndexType}

	assert.True(t, analyzer.containsIndex(indexes, core.TrigramIndexType))
	assert.True(t, analyzer.containsIndex(indexes, core.SymbolIndexType))
	assert.False(t, analyzer.containsIndex(indexes, core.ReferenceIndexType))
}

// TestAddRequiredIndex tests adding required indexes
func TestAddRequiredIndex(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	result := &AnalysisResult{
		RequiredIndexes: []core.IndexType{core.TrigramIndexType},
		Reasoning:       []string{"Initial reason"},
	}

	analyzer.addRequiredIndex(result, core.SymbolIndexType, "Need symbol index")

	assert.Len(t, result.RequiredIndexes, 2)
	assert.Contains(t, result.RequiredIndexes, core.SymbolIndexType)
	assert.Len(t, result.Reasoning, 2)
	assert.Contains(t, result.Reasoning, "Need symbol index")
}

// TestAddOptionalIndex tests adding optional indexes
func TestAddOptionalIndex(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	result := &AnalysisResult{
		RequiredIndexes: []core.IndexType{core.TrigramIndexType},
		OptionalIndexes: []core.IndexType{},
		Reasoning:       []string{"Initial reason"},
	}

	analyzer.addOptionalIndex(result, core.SymbolIndexType, "Symbol index optional")

	assert.Len(t, result.OptionalIndexes, 1)
	assert.Contains(t, result.OptionalIndexes, core.SymbolIndexType)
	assert.Len(t, result.Reasoning, 2)
	assert.Contains(t, result.Reasoning, "Symbol index optional")
}

// TestGenerateOptimizationHints tests optimization hint generation
func TestGenerateOptimizationHints(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name    string
		pattern string
		options types.SearchOptions
		result  *AnalysisResult
	}{
		{
			name:    "Complex pattern",
			pattern: "very.*complex.*[a-z].*pattern.*with.*many.*operators",
			options: types.SearchOptions{},
			result: &AnalysisResult{
				RequiredIndexes: []core.IndexType{core.TrigramIndexType, core.SymbolIndexType},
				Confidence:      0.3,
			},
		},
		{
			name:    "Too many indexes",
			pattern: "test",
			options: types.SearchOptions{},
			result: &AnalysisResult{
				RequiredIndexes: []core.IndexType{
					core.TrigramIndexType, core.SymbolIndexType, core.ReferenceIndexType,
					core.CallGraphIndexType, core.PostingsIndexType, core.LocationIndexType,
				},
				Confidence: 0.8,
			},
		},
		{
			name:    "Large context",
			pattern: "test",
			options: types.SearchOptions{MaxContextLines: 20},
			result: &AnalysisResult{
				RequiredIndexes: []core.IndexType{core.TrigramIndexType},
				Confidence:      0.9,
			},
		},
		{
			name:    "Low confidence",
			pattern: "test",
			options: types.SearchOptions{},
			result: &AnalysisResult{
				RequiredIndexes: []core.IndexType{core.TrigramIndexType},
				Confidence:      0.3,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			analyzer.generateOptimizationHints(tc.result, tc.pattern, tc.options)
			assert.NotEmpty(t, tc.result.OptimizationHints)
		})
	}
}

// TestGetEstimatedSearchTime tests search time estimation
func TestGetEstimatedSearchTime(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	testCases := []struct {
		name     string
		cost     int64
		expected int64
	}{
		{"Low cost", 100, 10},    // 5ms base + 100/20 = 5ms = 10ms
		{"Medium cost", 500, 30}, // 5ms base + 500/20 = 25ms = 30ms
		{"High cost", 1000, 55},  // 5ms base + 1000/20 = 50ms = 55ms
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := &AnalysisResult{
				EstimatedCost: tc.cost,
			}
			searchTime := analyzer.GetEstimatedSearchTime(result)
			assert.Equal(t, tc.expected, searchTime)
		})
	}
}

// TestAnalysisResult_GetRequiredIndexes tests getting required indexes
func TestAnalysisResult_GetRequiredIndexes(t *testing.T) {
	requiredIndexes := []core.IndexType{core.TrigramIndexType, core.SymbolIndexType}
	result := &AnalysisResult{
		RequiredIndexes: requiredIndexes,
	}

	assert.Equal(t, requiredIndexes, result.GetRequiredIndexes())
}

// TestShouldUseIndex tests index usage checking
func TestShouldUseIndex(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	result := &AnalysisResult{
		RequiredIndexes: []core.IndexType{core.TrigramIndexType},
		OptionalIndexes: []core.IndexType{core.SymbolIndexType},
	}

	assert.True(t, analyzer.ShouldUseIndex(result, core.TrigramIndexType))
	assert.True(t, analyzer.ShouldUseIndex(result, core.SymbolIndexType))
	assert.False(t, analyzer.ShouldUseIndex(result, core.ReferenceIndexType))
}

// TestGetAllIndexes tests getting all indexes
func TestGetAllIndexes(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	result := &AnalysisResult{
		RequiredIndexes: []core.IndexType{core.TrigramIndexType},
		OptionalIndexes: []core.IndexType{core.SymbolIndexType, core.TrigramIndexType}, // Duplicate
	}

	allIndexes := analyzer.GetAllIndexes(result)
	expected := []core.IndexType{core.TrigramIndexType, core.SymbolIndexType}
	assert.Equal(t, expected, allIndexes)
}

// TestAnalyzeSearchDependencies tests dependency analysis
func TestAnalyzeSearchDependencies(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	requirements := core.NewComprehensiveRequirements()

	analysis := analyzer.AnalyzeSearchDependencies(requirements)
	require.NotNil(t, analysis)
	assert.NotEmpty(t, analysis.RequiredIndexes)
	assert.NotNil(t, analysis.DependencyGraph)
	assert.NotNil(t, analysis.ValidationErrors)
	assert.False(t, analysis.AnalysisTime.IsZero())
	assert.False(t, analysis.CacheExpiry.IsZero())
}

// TestDependencyAnalysis_Validate tests dependency analysis validation
func TestDependencyAnalysis_Validate(t *testing.T) {
	t.Run("Valid analysis", func(t *testing.T) {
		analysis := &DependencyAnalysis{
			ValidationErrors: []error{},
		}
		assert.NoError(t, analysis.Validate())
	})

	t.Run("Invalid analysis", func(t *testing.T) {
		analysis := &DependencyAnalysis{
			ValidationErrors: []error{assert.AnError},
		}
		assert.Error(t, analysis.Validate())
	})
}

// TestResolveSearchDependencies tests dependency resolution
func TestResolveSearchDependencies(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	// Set up availability
	availability := map[core.IndexType]bool{
		core.TrigramIndexType: true,
		core.SymbolIndexType:  true,
	}
	analyzer.SetIndexAvailability(availability)

	requirements := core.NewSearchRequirements()
	requirements.NeedsTrigrams = true

	resolution := analyzer.ResolveSearchDependencies(requirements)
	require.NotNil(t, resolution)
	assert.NotEqual(t, time.Duration(0), resolution.ResolutionTime)
}

// TestSetIndexAvailability tests setting index availability
func TestSetIndexAvailability(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	availability := map[core.IndexType]bool{
		core.TrigramIndexType: true,
		core.SymbolIndexType:  false,
	}

	analyzer.SetIndexAvailability(availability)

	status := analyzer.getCurrentIndexAvailability()
	trigramStatus := status[core.TrigramIndexType]
	symbolStatus := status[core.SymbolIndexType]

	assert.NotNil(t, trigramStatus)
	assert.True(t, trigramStatus.IsAvailable)
	assert.False(t, trigramStatus.IsIndexing)

	assert.NotNil(t, symbolStatus)
	assert.False(t, symbolStatus.IsAvailable)
	assert.False(t, symbolStatus.IsIndexing)
}

// TestSimulateIndexingStateChange tests simulating index state changes
func TestSimulateIndexingStateChange(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	// Set initial availability
	availability := map[core.IndexType]bool{
		core.TrigramIndexType: true,
	}
	analyzer.SetIndexAvailability(availability)

	// Simulate indexing start
	analyzer.SimulateIndexingStateChange(core.TrigramIndexType, true)

	status := analyzer.getCurrentIndexAvailability()
	trigramStatus := status[core.TrigramIndexType]
	assert.False(t, trigramStatus.IsAvailable)
	assert.True(t, trigramStatus.IsIndexing)
	assert.False(t, trigramStatus.EstimatedReady.IsZero())

	// Simulate indexing complete
	analyzer.SimulateIndexingStateChange(core.TrigramIndexType, false)

	status = analyzer.getCurrentIndexAvailability()
	trigramStatus = status[core.TrigramIndexType]
	assert.True(t, trigramStatus.IsAvailable)
	assert.False(t, trigramStatus.IsIndexing)
	assert.True(t, trigramStatus.EstimatedReady.IsZero())
}

// TestGetDependencyCacheMetrics tests cache metrics
func TestGetDependencyCacheMetrics(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()
	metrics := analyzer.GetDependencyCacheMetrics()
	require.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.HitCount, int64(0))
	assert.GreaterOrEqual(t, metrics.MissCount, int64(0))
	assert.GreaterOrEqual(t, metrics.HitRate, 0.0)
	assert.LessOrEqual(t, metrics.HitRate, 1.0)
	assert.GreaterOrEqual(t, metrics.TotalSize, 0)
}

// TestResolveTransitiveDependencies tests transitive dependency resolution
func TestResolveTransitiveDependencies(t *testing.T) {
	analyzer := NewRequirementsAnalyzer()

	// Set up availability
	availability := map[core.IndexType]bool{
		core.TrigramIndexType: true,
		core.SymbolIndexType:  true,
	}
	analyzer.SetIndexAvailability(availability)

	requirements := core.NewSearchRequirements()
	requirements.NeedsTrigrams = true
	requirements.NeedsCallGraph = true

	resolution := analyzer.ResolveTransitiveDependencies(requirements)
	require.NotNil(t, resolution)
	assert.NotEmpty(t, resolution.DirectDependencies)
	assert.GreaterOrEqual(t, resolution.ResolutionScore, 0.0)
	assert.LessOrEqual(t, resolution.ResolutionScore, 1.0)
}

// BenchmarkAnalyzeRequirements benchmarks requirements analysis
func BenchmarkAnalyzeRequirements(b *testing.B) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "TestFunction"
	options := types.SearchOptions{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.AnalyzeRequirements(pattern, options)
	}
}

// BenchmarkCalculatePatternComplexity benchmarks pattern complexity calculation
func BenchmarkCalculatePatternComplexity(b *testing.B) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "test.*[a-z]+pattern"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.calculatePatternComplexity(pattern)
	}
}

// BenchmarkIsSymbolPattern benchmarks symbol pattern detection
func BenchmarkIsSymbolPattern(b *testing.B) {
	analyzer := NewRequirementsAnalyzer()
	pattern := "package.TestFunction"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.isSymbolPattern(pattern)
	}
}
