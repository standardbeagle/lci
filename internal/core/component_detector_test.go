package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestComponentDetector_DetectComponents tests the component detector detect components.
func TestComponentDetector_DetectComponents(t *testing.T) {
	detector := NewComponentDetector()

	// Test data: file mappings and symbols
	files := map[types.FileID]string{
		1: "main.go",
		2: "internal/handlers/user_handler.go",
		3: "internal/services/user_service.go",
		4: "internal/models/user.go",
		5: "internal/config/config.go",
		6: "internal/utils/helper.go",
		7: "test/user_test.go",
	}

	symbols := map[types.FileID][]types.Symbol{
		1: {
			{Name: "main", Type: types.SymbolTypeFunction, Line: 10},
		},
		2: {
			{Name: "UserHandler", Type: types.SymbolTypeStruct, Line: 8},
			{Name: "HandleGetUser", Type: types.SymbolTypeFunction, Line: 15},
		},
		3: {
			{Name: "UserService", Type: types.SymbolTypeStruct, Line: 5},
			{Name: "CreateUser", Type: types.SymbolTypeFunction, Line: 12},
		},
		4: {
			{Name: "User", Type: types.SymbolTypeStruct, Line: 5},
		},
		5: {
			{Name: "Config", Type: types.SymbolTypeStruct, Line: 8},
		},
		6: {
			{Name: "FormatString", Type: types.SymbolTypeFunction, Line: 10},
		},
		7: {
			{Name: "TestUserCreation", Type: types.SymbolTypeFunction, Line: 12},
		},
	}

	// Test options
	options := types.ComponentSearchOptions{
		MinConfidence: 0.3,
		MaxResults:    100,
		IncludeTests:  true,
	}

	// Run detection
	components, err := detector.DetectComponents(files, symbols, options)
	if err != nil {
		t.Fatalf("DetectComponents failed: %v", err)
	}

	if len(components) == 0 {
		t.Fatal("Expected components to be detected, got none")
	}

	// Verify expected component types are detected
	componentTypes := make(map[types.ComponentType]int)
	for _, comp := range components {
		componentTypes[comp.Type]++
		t.Logf("Detected: %s (%s) - Confidence: %.2f - File: %s",
			comp.Name, comp.Type.String(), comp.Confidence, comp.FilePath)
	}

	// Check that we detected some expected component types
	if componentTypes[types.ComponentTypeEntryPoint] == 0 {
		t.Error("Expected to detect entry point components")
	}

	if componentTypes[types.ComponentTypeAPIHandler] == 0 {
		t.Error("Expected to detect API handler components")
	}

	if componentTypes[types.ComponentTypeService] == 0 {
		t.Error("Expected to detect service components")
	}

	if componentTypes[types.ComponentTypeDataModel] == 0 {
		t.Error("Expected to detect data model components")
	}

	if componentTypes[types.ComponentTypeConfiguration] == 0 {
		t.Error("Expected to detect configuration components")
	}

	if componentTypes[types.ComponentTypeUtility] == 0 {
		t.Error("Expected to detect utility components")
	}

	if componentTypes[types.ComponentTypeTest] == 0 {
		t.Error("Expected to detect test components")
	}

	t.Logf("Successfully detected %d components across %d types",
		len(components), len(componentTypes))
}

// TestComponentDetector_FilterByType tests the component detector filter by type.
func TestComponentDetector_FilterByType(t *testing.T) {
	detector := NewComponentDetector()

	files := map[types.FileID]string{
		1: "main.go",
		2: "handler.go",
		3: "service.go",
	}

	symbols := map[types.FileID][]types.Symbol{
		1: {{Name: "main", Type: types.SymbolTypeFunction, Line: 10}},
		2: {{Name: "HandleRequest", Type: types.SymbolTypeFunction, Line: 15}},
		3: {{Name: "ProcessData", Type: types.SymbolTypeFunction, Line: 12}},
	}

	// Test filtering by specific types
	options := types.ComponentSearchOptions{
		Types:         []types.ComponentType{types.ComponentTypeEntryPoint},
		MinConfidence: 0.3,
		MaxResults:    100,
	}

	components, err := detector.DetectComponents(files, symbols, options)
	if err != nil {
		t.Fatalf("DetectComponents failed: %v", err)
	}

	// Should only get entry point components
	for _, comp := range components {
		if comp.Type != types.ComponentTypeEntryPoint {
			t.Errorf("Expected only entry-point components, got %s", comp.Type.String())
		}
	}

	t.Logf("Successfully filtered to %d entry-point components", len(components))
}

// TestComponentDetector_LanguageFilter tests the component detector language filter.
func TestComponentDetector_LanguageFilter(t *testing.T) {
	detector := NewComponentDetector()

	files := map[types.FileID]string{
		1: "main.go",
		2: "app.js",
		3: "server.py",
	}

	symbols := map[types.FileID][]types.Symbol{
		1: {{Name: "main", Type: types.SymbolTypeFunction, Line: 10}},
		2: {{Name: "startApp", Type: types.SymbolTypeFunction, Line: 5}},
		3: {{Name: "run_server", Type: types.SymbolTypeFunction, Line: 8}},
	}

	// Test Go language filter
	options := types.ComponentSearchOptions{
		Language:      "go",
		MinConfidence: 0.3,
		MaxResults:    100,
	}

	components, err := detector.DetectComponents(files, symbols, options)
	if err != nil {
		t.Fatalf("DetectComponents failed: %v", err)
	}

	// Should only get Go components
	for _, comp := range components {
		if comp.Language != "go" {
			t.Errorf("Expected only Go components, got %s component", comp.Language)
		}
	}

	t.Logf("Successfully filtered to %d Go components", len(components))
}
