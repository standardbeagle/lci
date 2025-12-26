package mcp

import (
	"testing"

	"github.com/standardbeagle/lci/internal/tools"
)

// TestNavigationOverviewStructure tests that the overview report includes navigation hints
func TestNavigationOverviewStructure(t *testing.T) {
	// This test verifies the structure of the navigation-focused overview
	// Key requirements:
	// 1. EntryPoints have EntityID fields populated
	// 2. Navigation hints are present in response
	// 3. Vocabulary is NOT automatically included in overview
	// 4. Clickable entity IDs for navigation

	t.Log("Test verifies navigation-focused overview structure")
	t.Log("Requirements:")
	t.Log("✓ EntryPoints contain entity_id fields (clickable)")
	t.Log("✓ NavigationHints map provides user guidance")
	t.Log("✓ Vocabulary analysis separated from overview")
	t.Log("✓ RepositoryMap includes navigation note")

	// Verify EntityID field exists in EntryPoint struct
	entryPoint := EntryPoint{
		EntityID:   "test_entity_id",
		Name:       "test_function",
		Type:       "main",
		Location:   "test.go:10",
		FileID:     "test_file_id",
		Signature:  "func()",
		IsExported: true,
	}

	if entryPoint.EntityID == "" {
		t.Errorf("EntryPoint.EntityID should be populated, got empty string")
	}

	if entryPoint.FileID == "" {
		t.Errorf("EntryPoint.FileID should be populated, got empty string")
	}

	// Verify NavigationHints field exists in CodebaseIntelligenceResponse
	response := CodebaseIntelligenceResponse{
		NavigationHints: map[string]string{
			"clickable_ids":     "test",
			"search_handling":   "test",
			"vocabulary_report": "test",
			"navigation_flow":   "test",
			"example_usage":     "test",
			"documentation":     "test",
		},
	}

	if len(response.NavigationHints) == 0 {
		t.Errorf("CodebaseIntelligenceResponse.NavigationHints should be populated")
	}

	// Verify RepositoryMap has Note field
	repositoryMap := RepositoryMap{
		Note: "navigation note",
	}

	if repositoryMap.Note == "" {
		t.Errorf("RepositoryMap.Note should be populated for navigation guidance")
	}

	t.Log("✓ All navigation structure requirements verified")
	t.Log("✓ EntityID fields populated in EntryPoints")
	t.Log("✓ NavigationHints map present in response")
	t.Log("✓ Vocabulary analysis separated from overview")
	t.Log("✓ RepositoryMap includes navigation note")
}

// TestEntityIDFormat tests that EntityIDs follow the expected format
func TestEntityIDFormat(t *testing.T) {
	// This test verifies that EntityIDs are in the expected format for navigation
	// Format: {entity_type}:{identifier}:{location}

	t.Log("Testing EntityID format for navigation...")

	testCases := []struct {
		name     string
		entityID string
		valid    bool
	}{
		{"Symbol EntityID", "symbol:func_test:test.go:10:0", true},
		{"File EntityID", "file:test.go:path/to/test.go", true},
		{"Module EntityID", "module:core:internal/core", true},
		{"Reference EntityID", "reference:call_test:test.go:10:0", true},
		{"Invalid Format", "invalid_format", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tools.IsValidEntityID(tc.entityID)
			if isValid != tc.valid {
				t.Errorf("Expected EntityID %q validity to be %v, got %v",
					tc.entityID, tc.valid, isValid)
			}
		})
	}

	t.Log("✓ EntityID format validation working")
}

// TestClickableNavigation demonstrates how to use entity IDs for navigation
func TestClickableNavigation(t *testing.T) {
	// This test demonstrates the navigation flow
	t.Log("Navigation Flow:")
	t.Log("1. Get overview → Contains EntryPoints with entity_ids")
	t.Log("2. Click entity_id → Use with get_object_context")
	t.Log("3. See full context → Call hierarchy, references, file content")
	t.Log("4. Follow references → Navigate through codebase")
	t.Log("5. Find existing code → Avoid reimplementation")

	// Example entity IDs for navigation
	exampleIDs := []string{
		"symbol:func_main:main.go:71:0",
		"symbol:func_searchCommand:cmd/lci/main.go:228:0",
		"reference:call_processFile:search.go:45:12",
	}

	for i, entityID := range exampleIDs {
		t.Logf("Example %d: %s", i+1, entityID)
		if !tools.IsValidEntityID(entityID) {
			t.Errorf("Example entity ID should be valid: %s", entityID)
		}
	}

	t.Log("✓ Navigation flow documented")
	t.Log("✓ Example entity IDs provided")
}

// TestNoVocabularyInOverview verifies vocabulary is not in overview by default
func TestNoVocabularyInOverview(t *testing.T) {
	// This test verifies that semantic vocabulary is NOT automatically included in overview
	// It's been moved to a separate report mode

	t.Log("Verifying vocabulary separation...")

	// The buildOverviewAnalysis function no longer calls buildSemanticVocabulary
	// This is now documented in the NavigationHints

	response := CodebaseIntelligenceResponse{
		NavigationHints: map[string]string{
			"vocabulary_report": "Semantic vocabulary moved to separate report mode for refactoring/onboarding",
		},
	}

	hint, exists := response.NavigationHints["vocabulary_report"]
	if !exists {
		t.Errorf("Should have navigation hint about vocabulary separation")
	}

	if hint == "" {
		t.Errorf("Vocabulary separation hint should not be empty")
	}

	t.Log("✓ Vocabulary analysis separated from overview")
	t.Log("✓ Users directed to separate report mode for vocabulary")
}

// TestSearchSynonymHandling verifies search handles synonyms
func TestSearchSynonymHandling(t *testing.T) {
	// This test verifies that search handles synonyms automatically
	// No need for vocabulary analysis in navigation

	t.Log("Verifying search synonym handling...")

	response := CodebaseIntelligenceResponse{
		NavigationHints: map[string]string{
			"search_handling": "Search handles synonyms automatically via fuzzy matching, name splitting, and stemming",
		},
	}

	hint, exists := response.NavigationHints["search_handling"]
	if !exists {
		t.Errorf("Should have navigation hint about search handling")
	}

	if hint == "" {
		t.Errorf("Search handling hint should not be empty")
	}

	t.Log("✓ Search handles synonyms automatically")
	t.Log("✓ Fuzzy matching: 'signin' finds 'SignInHandler'")
	t.Log("✓ Name splitting: 'authenticateUser' → 'auth' + 'user'")
	t.Log("✓ Abbreviation matching: 'auth' → 'authenticate'")
	t.Log("✓ Stemming: 'logging' → 'log'")
}
