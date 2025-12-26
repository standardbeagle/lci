package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestGraphPropagator_TypeHierarchyPropagation tests that labels propagate through
// implements/extends relationships when IncludeTypeHierarchy is enabled
func TestGraphPropagator_TypeHierarchyPropagation(t *testing.T) {
	// Create a reference tracker with test data
	rt := NewReferenceTrackerForTest()

	// Create test symbols representing an interface and implementations
	symbols := []types.Symbol{
		{Name: "Handler", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
		{Name: "FileHandler", Type: types.SymbolTypeStruct, Line: 20, EndLine: 25},
		{Name: "NetHandler", Type: types.SymbolTypeStruct, Line: 30, EndLine: 35},
		{Name: "BaseService", Type: types.SymbolTypeClass, Line: 40, EndLine: 45},
		{Name: "DerivedService", Type: types.SymbolTypeClass, Line: 50, EndLine: 55},
	}

	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)

	// Get symbol IDs
	var handlerID, fileHandlerID, netHandlerID, baseServiceID, derivedServiceID types.SymbolID
	for _, sym := range enhancedSymbols {
		switch sym.Name {
		case "Handler":
			handlerID = sym.ID
		case "FileHandler":
			fileHandlerID = sym.ID
		case "NetHandler":
			netHandlerID = sym.ID
		case "BaseService":
			baseServiceID = sym.ID
		case "DerivedService":
			derivedServiceID = sym.ID
		}
	}

	// Add implements references
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   fileHandlerID,
		TargetSymbol:   handlerID,
		ReferencedName: "Handler",
		FileID:         1,
		Line:           21,
		Quality:        types.RefQualityAssigned,
	})
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   netHandlerID,
		TargetSymbol:   handlerID,
		ReferencedName: "Handler",
		FileID:         1,
		Line:           31,
		Quality:        types.RefQualityHeuristic,
	})

	// Add extends reference
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeExtends,
		SourceSymbol:   derivedServiceID,
		TargetSymbol:   baseServiceID,
		ReferencedName: "BaseService",
		FileID:         1,
		Line:           51,
	})

	rt.ProcessAllReferences()

	t.Run("getConnectedSymbols_Downstream_WithTypeHierarchy", func(t *testing.T) {
		gp := NewGraphPropagator(nil, rt, nil)

		// Downstream from interface should include implementors
		connected := gp.getConnectedSymbols(handlerID, "downstream", true)

		// Should find both FileHandler and NetHandler
		found := make(map[types.SymbolID]bool)
		for _, id := range connected {
			found[id] = true
		}

		if !found[fileHandlerID] {
			t.Error("expected FileHandler in downstream connections")
		}
		if !found[netHandlerID] {
			t.Error("expected NetHandler in downstream connections")
		}
	})

	t.Run("getConnectedSymbols_Upstream_WithTypeHierarchy", func(t *testing.T) {
		gp := NewGraphPropagator(nil, rt, nil)

		// Upstream from implementation should include interface
		connected := gp.getConnectedSymbols(fileHandlerID, "upstream", true)

		found := make(map[types.SymbolID]bool)
		for _, id := range connected {
			found[id] = true
		}

		if !found[handlerID] {
			t.Error("expected Handler interface in upstream connections")
		}
	})

	t.Run("getConnectedSymbols_Downstream_ExtendsRelationship", func(t *testing.T) {
		gp := NewGraphPropagator(nil, rt, nil)

		// Downstream from base should include derived
		connected := gp.getConnectedSymbols(baseServiceID, "downstream", true)

		found := make(map[types.SymbolID]bool)
		for _, id := range connected {
			found[id] = true
		}

		if !found[derivedServiceID] {
			t.Error("expected DerivedService in downstream connections")
		}
	})

	t.Run("getConnectedSymbols_WithoutTypeHierarchy", func(t *testing.T) {
		gp := NewGraphPropagator(nil, rt, nil)

		// Without type hierarchy, should only get call graph connections (none in this test)
		connected := gp.getConnectedSymbols(handlerID, "downstream", false)

		if len(connected) != 0 {
			t.Errorf("expected 0 connections without type hierarchy, got %d", len(connected))
		}
	})
}

// TestGraphPropagator_InterfaceCallAttribution tests the interface call attribution
// with code analysis and heuristic fallback
func TestGraphPropagator_InterfaceCallAttribution(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	// Create interface and implementations with different quality levels
	symbols := []types.Symbol{
		{Name: "Repository", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
		{Name: "PostgresRepo", Type: types.SymbolTypeStruct, Line: 20, EndLine: 25},
		{Name: "MongoRepo", Type: types.SymbolTypeStruct, Line: 30, EndLine: 35},
		{Name: "MockRepo", Type: types.SymbolTypeStruct, Line: 40, EndLine: 45},
	}

	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)

	var repoID, postgresID, mongoID, mockID types.SymbolID
	for _, sym := range enhancedSymbols {
		switch sym.Name {
		case "Repository":
			repoID = sym.ID
		case "PostgresRepo":
			postgresID = sym.ID
		case "MongoRepo":
			mongoID = sym.ID
		case "MockRepo":
			mockID = sym.ID
		}
	}

	// PostgresRepo: assigned (high quality - explicit interface assignment)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   postgresID,
		TargetSymbol:   repoID,
		ReferencedName: "Repository",
		FileID:         1,
		Line:           21,
		Quality:        types.RefQualityAssigned,
	})

	// MongoRepo: returned (medium quality - returned as interface)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   mongoID,
		TargetSymbol:   repoID,
		ReferencedName: "Repository",
		FileID:         1,
		Line:           31,
		Quality:        types.RefQualityReturned,
	})

	// MockRepo: heuristic (low quality - method signature matching only)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   mockID,
		TargetSymbol:   repoID,
		ReferencedName: "Repository",
		FileID:         1,
		Line:           41,
		Quality:        types.RefQualityHeuristic,
	})

	rt.ProcessAllReferences()

	gp := NewGraphPropagator(nil, rt, nil)

	t.Run("GetInterfaceCallImplementations_CodeAnalysis", func(t *testing.T) {
		attribution := gp.GetInterfaceCallImplementations(repoID)

		if attribution.AttributionMethod != "code_analysis" {
			t.Errorf("expected attribution method 'code_analysis', got %q", attribution.AttributionMethod)
		}

		if len(attribution.Implementations) != 3 {
			t.Errorf("expected 3 implementations, got %d", len(attribution.Implementations))
		}

		// Check confidence levels - code analysis should have high confidence
		for _, impl := range attribution.Implementations {
			if impl.Quality == types.RefQualityAssigned && impl.Confidence < 0.9 {
				t.Errorf("expected high confidence for assigned quality, got %f", impl.Confidence)
			}
			if impl.Quality == types.RefQualityReturned && impl.Confidence < 0.85 {
				t.Errorf("expected medium-high confidence for returned quality, got %f", impl.Confidence)
			}
		}
	})

	t.Run("CalculateImplementationConfidence", func(t *testing.T) {
		tests := []struct {
			quality string
			minConf float64
			maxConf float64
		}{
			{types.RefQualityAssigned, 0.90, 1.0},
			{types.RefQualityReturned, 0.85, 0.95},
			{types.RefQualityCast, 0.80, 0.90},
			{types.RefQualityHeuristic, 0.45, 0.55},
			{"unknown", 0.20, 0.40},
		}

		for _, tt := range tests {
			confidence := gp.calculateImplementationConfidence(tt.quality, 0)
			if confidence < tt.minConf || confidence > tt.maxConf {
				t.Errorf("confidence for %q: got %f, expected between %f and %f",
					tt.quality, confidence, tt.minConf, tt.maxConf)
			}
		}
	})
}

// TestGraphPropagator_HeuristicOnlyAttribution tests fallback to heuristic
// when no high-quality code analysis evidence exists
func TestGraphPropagator_HeuristicOnlyAttribution(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	symbols := []types.Symbol{
		{Name: "Logger", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
		{Name: "FileLogger", Type: types.SymbolTypeStruct, Line: 20, EndLine: 25},
		{Name: "ConsoleLogger", Type: types.SymbolTypeStruct, Line: 30, EndLine: 35},
	}

	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)

	var loggerID, fileLoggerID, consoleLoggerID types.SymbolID
	for _, sym := range enhancedSymbols {
		switch sym.Name {
		case "Logger":
			loggerID = sym.ID
		case "FileLogger":
			fileLoggerID = sym.ID
		case "ConsoleLogger":
			consoleLoggerID = sym.ID
		}
	}

	// Both implementations only have heuristic quality (method matching)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   fileLoggerID,
		TargetSymbol:   loggerID,
		ReferencedName: "Logger",
		FileID:         1,
		Line:           21,
		Quality:        types.RefQualityHeuristic,
	})
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   consoleLoggerID,
		TargetSymbol:   loggerID,
		ReferencedName: "Logger",
		FileID:         1,
		Line:           31,
		Quality:        types.RefQualityHeuristic,
	})

	rt.ProcessAllReferences()

	gp := NewGraphPropagator(nil, rt, nil)

	attribution := gp.GetInterfaceCallImplementations(loggerID)

	// Should fall back to heuristic mode
	if attribution.AttributionMethod != "heuristic" {
		t.Errorf("expected attribution method 'heuristic', got %q", attribution.AttributionMethod)
	}

	if len(attribution.Implementations) != 2 {
		t.Errorf("expected 2 implementations, got %d", len(attribution.Implementations))
	}

	// Heuristic attributions should have lower confidence
	for _, impl := range attribution.Implementations {
		if impl.Confidence > 0.6 {
			t.Errorf("expected lower confidence for heuristic attribution, got %f", impl.Confidence)
		}
	}
}

// TestLabelPropagationRule_IncludeTypeHierarchy tests the new field in rules
func TestLabelPropagationRule_IncludeTypeHierarchy(t *testing.T) {
	config := getDefaultPropagationConfig()

	// Check that critical and security rules have type hierarchy enabled
	for _, rule := range config.LabelRules {
		switch rule.Label {
		case "critical", "security", "database-call", "api-endpoint", "memory-allocation":
			if !rule.IncludeTypeHierarchy {
				t.Errorf("expected IncludeTypeHierarchy=true for %q rule", rule.Label)
			}
		case "ui-relevance":
			if rule.IncludeTypeHierarchy {
				t.Errorf("expected IncludeTypeHierarchy=false for %q rule", rule.Label)
			}
		}
	}
}

// TestGraphPropagator_NoImplementors tests behavior when interface has no implementors
func TestGraphPropagator_NoImplementors(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	symbols := []types.Symbol{
		{Name: "UnimplementedInterface", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
	}

	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)
	ifaceID := enhancedSymbols[0].ID

	rt.ProcessAllReferences()

	gp := NewGraphPropagator(nil, rt, nil)

	attribution := gp.GetInterfaceCallImplementations(ifaceID)

	if attribution.AttributionMethod != "none" {
		t.Errorf("expected attribution method 'none', got %q", attribution.AttributionMethod)
	}

	if len(attribution.Implementations) != 0 {
		t.Errorf("expected 0 implementations, got %d", len(attribution.Implementations))
	}
}
