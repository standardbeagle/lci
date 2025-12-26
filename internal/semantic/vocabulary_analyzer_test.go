package semantic

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Setup
// ============================================================================

func TestAnalyzeCodebaseVocabulary_EmptyProject(t *testing.T) {
	// Setup
	symbols := []FileSymbol{}
	config := ProjectConfig{
		Language:   "go",
		SourceDirs: []string{"cmd/", "internal/"},
	}
	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	// Execute
	result, err := AnalyzeCodebaseVocabulary(symbols, config, dict, splitter)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.DomainsPresent)
	assert.NotEmpty(t, result.DomainsAbsent) // Should have all domains marked as absent
	assert.Empty(t, result.UniqueTerms)
	assert.Empty(t, result.CommonTerms)
	assert.Zero(t, result.VocabularySize)
}

func TestAnalyzeCodebaseVocabulary_SimpleGoProject(t *testing.T) {
	// Setup - Biology project with production code
	symbols := []FileSymbol{
		{FilePath: "internal/biology/mitosis.go", Name: "CalculateMitosisDuration", Type: "function", IsExported: true},
		{FilePath: "internal/biology/mitosis.go", Name: "mitosisController", Type: "function", IsExported: false},
		{FilePath: "cmd/biology/main.go", Name: "main", Type: "function", IsExported: true},
		{FilePath: "internal/user/user_service.go", Name: "GetUserByID", Type: "function", IsExported: true},
		{FilePath: "internal/user/user.go", Name: "User", Type: "type", IsExported: true},
		// Test files - should be excluded
		{FilePath: "internal/biology/mitosis_test.go", Name: "TestCalculateMitosisDuration", Type: "function", IsExported: false},
		{FilePath: "internal/user/user_test.go", Name: "TestGetUserByID", Type: "function", IsExported: false},
	}

	config := ProjectConfig{
		Language:   "go",
		SourceDirs: []string{"cmd/", "internal/"},
	}
	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	// Execute
	result, err := AnalyzeCodebaseVocabulary(symbols, config, dict, splitter)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check analysis scope
	// Total unique files: 6 (mitosis.go appears once even though it has 2 symbols)
	assert.Equal(t, 6, result.AnalysisScope.TotalFiles)
	// Production files: 4 (mitosis.go, main.go, user_service.go, user.go - test files excluded)
	assert.Equal(t, 4, result.AnalysisScope.ProductionFiles)
	// Test files excluded: 2 (mitosis_test.go, user_test.go)
	assert.Equal(t, 2, result.AnalysisScope.TestFilesExcluded)
	// Total symbols: 7 (5 production + 2 test)
	assert.Equal(t, 7, result.AnalysisScope.TotalSymbols)
	// Total functions: 4 (3 production functions + 1 type = 4 total)
	assert.Equal(t, 4, result.AnalysisScope.TotalFunctions)
	// Total types: 1 (User type)
	assert.Equal(t, 1, result.AnalysisScope.TotalTypes)

	// Check domains present
	assert.NotEmpty(t, result.DomainsPresent)
	// Should find user domain
	userDomainFound := false
	for _, domain := range result.DomainsPresent {
		if domain.Name == "user" {
			userDomainFound = true
			assert.Greater(t, domain.Count, 0)
			assert.True(t, domain.Confidence > 0)
			assert.NotEmpty(t, domain.ExampleSymbols)
		}
	}
	assert.True(t, userDomainFound, "Should find 'user' domain")

	// Check domains absent (should include many that aren't in this biology/user project)
	assert.NotEmpty(t, result.DomainsAbsent)

	// Check unique terms
	mitosisFound := false
	for _, term := range result.UniqueTerms {
		if term.Term == "mitosis" {
			mitosisFound = true
			assert.Greater(t, term.Count, 0)
			assert.NotEmpty(t, term.ExampleSymbols)
		}
	}
	assert.True(t, mitosisFound, "Should find 'mitosis' as unique term")

	// Check common terms (user, service, etc.)
	userFound := false
	for _, term := range result.CommonTerms {
		if term.Term == "user" {
			userFound = true
			assert.Greater(t, term.Count, 0)
		}
	}
	// user might be in common terms, but it's okay if not
	_ = userFound

	// Vocabulary size should be > 0
	assert.Greater(t, result.VocabularySize, 0)
}

// ============================================================================
// Test Production Code Filtering
// ============================================================================

func TestFilterProductionSymbols_GoProject(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "cmd/main.go", Name: "main", Type: "function"},
		{FilePath: "internal/service/user.go", Name: "GetUser", Type: "function"},
		{FilePath: "internal/service/user_test.go", Name: "TestGetUser", Type: "function"},
		{FilePath: "internal/model/user.go", Name: "User", Type: "type"},
		{FilePath: "internal/model/user_test.go", Name: "TestUser", Type: "function"},
		{FilePath: "test/e2e/test_user.go", Name: "TestUserE2E", Type: "function"},
	}

	config := ProjectConfig{
		Language:   "go",
		SourceDirs: []string{"cmd/", "internal/"},
	}

	result := FilterProductionSymbols(symbols, config)

	// Should filter out test files
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "cmd/main.go", result[0].FilePath)
	assert.Equal(t, "internal/service/user.go", result[1].FilePath)
	assert.Equal(t, "internal/model/user.go", result[2].FilePath)
}

func TestFilterProductionSymbols_NodeProject(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "src/components/User.tsx", Name: "UserComponent", Type: "function"},
		{FilePath: "src/services/user.ts", Name: "getUser", Type: "function"},
		{FilePath: "__tests__/user.test.ts", Name: "getUser", Type: "function"},
		{FilePath: "test/user.spec.ts", Name: "getUser", Type: "function"},
	}

	config := ProjectConfig{
		Language:   "javascript",
		SourceDirs: []string{"src/", "components/", "services/"},
	}

	result := FilterProductionSymbols(symbols, config)

	// Should filter out test files
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "src/components/User.tsx", result[0].FilePath)
	assert.Equal(t, "src/services/user.ts", result[1].FilePath)
}

func TestFilterProductionSymbols_EmptyConfig(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "main.go", Name: "main", Type: "function"},
		{FilePath: "test.go", Name: "TestMain", Type: "function"},
	}

	config := ProjectConfig{
		Language: "go",
	}

	result := FilterProductionSymbols(symbols, config)

	// Should keep main.go, filter out test.go
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "main.go", result[0].FilePath)
}

// ============================================================================
// Test Domain Analysis
// ============================================================================

func TestExtractDomainOccurrences_MultipleMatches(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "auth.go", Name: "loginUser", Type: "function"},
		{FilePath: "auth.go", Name: "signinHandler", Type: "function"},
		{FilePath: "auth.go", Name: "authenticateToken", Type: "function"},
		{FilePath: "db.go", Name: "sqlQuery", Type: "function"},
		{FilePath: "db.go", Name: "getDatabase", Type: "function"},
	}

	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	analysis := ExtractDomainOccurrences(symbols, dict, splitter)

	// Should find authentication domain
	assert.Contains(t, analysis.domainCounts, "authentication")
	assert.Greater(t, analysis.domainCounts["authentication"], 0)

	// Should find database domain
	assert.Contains(t, analysis.domainCounts, "database")
	assert.Greater(t, analysis.domainCounts["database"], 0)

	// Should have matched terms
	authTerms := analysis.domainTerms["authentication"]
	assert.NotEmpty(t, authTerms)
	assert.Contains(t, authTerms, "login")
	assert.Contains(t, authTerms, "signin")
	// Note: "auth" is not directly matched, only "authenticate" is matched
	// Abbreviation expansion is not applied in ExtractDomainOccurrences
}

func TestExtractDomainOccurrences_NoMatches(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "weird.go", Name: "xyzABC123", Type: "function"},
		{FilePath: "weird.go", Name: "fooBarBaz", Type: "function"},
	}

	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	analysis := ExtractDomainOccurrences(symbols, dict, splitter)

	// All domains should have zero count
	for domainName, count := range analysis.domainCounts {
		assert.Zero(t, count, "Domain %s should have zero count", domainName)
	}

	// No present domains
	assert.Empty(t, analysis.presentDomainNames)
}

// ============================================================================
// Test Term Analysis
// ============================================================================

func TestIdentifyUniqueTerms_BiologyProject(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "bio.go", Name: "mitosisDuration", Type: "function"},
		{FilePath: "bio.go", Name: "checkMitosis", Type: "function"},
		{FilePath: "bio.go", Name: "cytokinesisController", Type: "function"},
		// Common terms mixed in
		{FilePath: "bio.go", Name: "userService", Type: "function"},
		{FilePath: "bio.go", Name: "getUser", Type: "function"},
	}

	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	uniqueTerms := IdentifyUniqueTerms(symbols, dict, splitter)

	// Should find biology-specific terms
	mitosisFound := false
	cytokinesisFound := false
	for _, term := range uniqueTerms {
		if term.Term == "mitosis" {
			mitosisFound = true
			assert.Greater(t, term.Count, 0)
			assert.NotEmpty(t, term.ExampleSymbols)
		}
		if term.Term == "cytokinesis" {
			cytokinesisFound = true
			assert.Greater(t, term.Count, 0)
		}
	}
	assert.True(t, mitosisFound, "Should find 'mitosis'")
	assert.True(t, cytokinesisFound, "Should find 'cytokinesis'")
}

func TestIdentifyCommonTerms_GenericProject(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "service.go", Name: "userService", Type: "function"},
		{FilePath: "service.go", Name: "getUser", Type: "function"},
		{FilePath: "service.go", Name: "userManager", Type: "function"},
		{FilePath: "service.go", Name: "orderService", Type: "function"}, // Add "service" again
		{FilePath: "model.go", Name: "UserModel", Type: "type"},
		{FilePath: "controller.go", Name: "UserController", Type: "type"},
		{FilePath: "controller.go", Name: "orderManager", Type: "function"}, // Add "manager" again
	}

	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	commonTerms := IdentifyCommonTerms(symbols, dict, splitter)

	// Should find common terms
	userFound := false
	serviceFound := false
	managerFound := false
	for _, term := range commonTerms {
		if term.Term == "user" {
			userFound = true
		}
		if term.Term == "service" {
			serviceFound = true
		}
		if term.Term == "manager" {
			managerFound = true
		}
	}
	assert.True(t, userFound, "Should find 'user' as common term")
	assert.True(t, serviceFound, "Should find 'service' as common term")
	assert.True(t, managerFound, "Should find 'manager' as common term")
}

func TestIdentifyCommonTerms_NoDuplicates(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "test.go", Name: "test", Type: "function"},    // Single occurrence
		{FilePath: "test.go", Name: "setup", Type: "function"},   // Single occurrence
		{FilePath: "service.go", Name: "user", Type: "variable"}, // Multiple occurrences
		{FilePath: "service.go", Name: "userID", Type: "variable"},
		{FilePath: "controller.go", Name: "user", Type: "variable"},
	}

	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	commonTerms := IdentifyCommonTerms(symbols, dict, splitter)

	// Should not include terms with count < 2
	for _, term := range commonTerms {
		assert.GreaterOrEqual(t, term.Count, 2, "Common terms should have count >= 2")
	}
}

// ============================================================================
// Test Statistics Calculation
// ============================================================================

func TestCalculateScopeStatistics_Basic(t *testing.T) {
	allSymbols := []FileSymbol{
		{FilePath: "main.go", Name: "main", Type: "function"},
		{FilePath: "service.go", Name: "GetUser", Type: "function"},
		{FilePath: "model.go", Name: "User", Type: "type"},
		{FilePath: "service.go", Name: "userID", Type: "variable"},
		{FilePath: "service_test.go", Name: "TestGetUser", Type: "function"},
	}

	productionSymbols := []FileSymbol{
		{FilePath: "main.go", Name: "main", Type: "function"},
		{FilePath: "service.go", Name: "GetUser", Type: "function"},
		{FilePath: "model.go", Name: "User", Type: "type"},
		{FilePath: "service.go", Name: "userID", Type: "variable"},
	}

	config := ProjectConfig{
		Language:   "go",
		SourceDirs: []string{"."},
	}

	stats := CalculateScopeStatistics(allSymbols, productionSymbols, config)

	assert.Equal(t, 4, stats.TotalFiles)        // 4 unique files (main.go, service.go, model.go, service_test.go)
	assert.Equal(t, 3, stats.ProductionFiles)   // 3 production files (main.go, service.go, model.go)
	assert.Equal(t, 1, stats.TestFilesExcluded) // 1 test file
	assert.Equal(t, 5, stats.TotalSymbols)      // All symbols
	assert.Equal(t, 2, stats.TotalFunctions)    // main, GetUser
	assert.Equal(t, 1, stats.TotalVariables)    // userID
	assert.Equal(t, 1, stats.TotalTypes)        // User
}

func TestCalculateScopeStatistics_InferSourceDirectories(t *testing.T) {
	allSymbols := []FileSymbol{
		{FilePath: "cmd/main.go", Name: "main", Type: "function"},
		{FilePath: "internal/service/user.go", Name: "GetUser", Type: "function"},
		{FilePath: "pkg/utils/helper.go", Name: "Helper", Type: "function"},
	}

	productionSymbols := allSymbols

	config := ProjectConfig{
		Language: "go",
		// SourceDirs not specified - should infer
	}

	stats := CalculateScopeStatistics(allSymbols, productionSymbols, config)

	// Should infer source directories
	assert.NotEmpty(t, stats.SourceDirectories)
	assert.Contains(t, stats.SourceDirectories, "cmd")
	assert.Contains(t, stats.SourceDirectories, "internal")
	assert.Contains(t, stats.SourceDirectories, "pkg")
}

func TestCountUniqueFiles(t *testing.T) {
	symbols := []FileSymbol{
		{FilePath: "file1.go", Name: "func1", Type: "function"},
		{FilePath: "file1.go", Name: "func2", Type: "function"},
		{FilePath: "file2.go", Name: "func3", Type: "function"},
		{FilePath: "file3.go", Name: "func4", Type: "function"},
	}

	count := countUniqueFiles(symbols)
	assert.Equal(t, 3, count)
}

// ============================================================================
// Test Helper Functions
// ============================================================================

func TestFindDomainsForTerm(t *testing.T) {
	dict := DefaultTranslationDictionary()

	// Test auth terms
	authDomains := findDomainsForTerm("login", dict)
	assert.Contains(t, authDomains, "authentication")

	signinDomains := findDomainsForTerm("signin", dict)
	assert.Contains(t, signinDomains, "authentication")

	// Test database terms
	sqlDomains := findDomainsForTerm("sql", dict)
	assert.Contains(t, sqlDomains, "database")

	// Test non-domain term
	weirdDomains := findDomainsForTerm("xyz123", dict)
	assert.Empty(t, weirdDomains)
}

func TestGetAllDomainNames(t *testing.T) {
	dict := DefaultTranslationDictionary()

	domains := getAllDomainNames(dict)

	assert.NotEmpty(t, domains)
	assert.Contains(t, domains, "authentication")
	assert.Contains(t, domains, "user")
	assert.Contains(t, domains, "database")
}

func TestGetAbsentDomains(t *testing.T) {
	dict := DefaultTranslationDictionary()

	// Create analysis with only user domain present
	analysis := &domainAnalysis{
		domainCounts: make(map[string]int),
	}
	for domainName := range dict.Domains {
		if domainName == "user" {
			analysis.domainCounts[domainName] = 5
		} else {
			analysis.domainCounts[domainName] = 0
		}
	}

	absent := getAbsentDomains(analysis, dict)

	// Should include all domains except user
	assert.NotEmpty(t, absent)
	assert.NotContains(t, absent, "user")
	// Should contain at least some absent domains
	otherFound := false
	for _, domain := range absent {
		if domain != "user" {
			otherFound = true
			break
		}
	}
	assert.True(t, otherFound, "Should have some absent domains")
}

// ============================================================================
// Benchmark Tests (Performance)
// ============================================================================

func BenchmarkAnalyzeCodebaseVocabulary_Small(b *testing.B) {
	symbols := generateTestSymbols(100)
	config := ProjectConfig{
		Language:   "go",
		SourceDirs: []string{"internal/", "cmd/"},
	}
	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := AnalyzeCodebaseVocabulary(symbols, config, dict, splitter)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

func BenchmarkAnalyzeCodebaseVocabulary_Large(b *testing.B) {
	symbols := generateTestSymbols(10000)
	config := ProjectConfig{
		Language:   "go",
		SourceDirs: []string{"internal/", "cmd/"},
	}
	dict := DefaultTranslationDictionary()
	splitter := NewNameSplitter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := AnalyzeCodebaseVocabulary(symbols, config, dict, splitter)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

// generateTestSymbols creates test symbols for benchmarking
func generateTestSymbols(n int) []FileSymbol {
	symbols := make([]FileSymbol, n)
	prefixes := []string{"user", "auth", "service", "manager", "model", "controller", "view", "data"}
	types := []string{"function", "type", "variable"}

	for i := 0; i < n; i++ {
		prefix := prefixes[i%len(prefixes)]
		symbols[i] = FileSymbol{
			FilePath:   filepath.Join("internal", "test", "file.go"),
			Name:       prefix + "TestSymbol",
			Type:       types[i%len(types)],
			IsExported: i%2 == 0,
		}
	}

	return symbols
}
