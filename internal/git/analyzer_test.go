package git

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestGetSideEffectCategories(t *testing.T) {
	a := &Analyzer{} // Minimal analyzer for testing helper methods

	tests := []struct {
		name     string
		cats     types.SideEffectCategory
		expected []string
	}{
		{
			name:     "no side effects",
			cats:     types.SideEffectNone,
			expected: nil,
		},
		{
			name:     "single param write",
			cats:     types.SideEffectParamWrite,
			expected: []string{"param-write"},
		},
		{
			name:     "single receiver write",
			cats:     types.SideEffectReceiverWrite,
			expected: []string{"receiver-write"},
		},
		{
			name:     "single global write",
			cats:     types.SideEffectGlobalWrite,
			expected: []string{"global-write"},
		},
		{
			name:     "single io",
			cats:     types.SideEffectIO,
			expected: []string{"io"},
		},
		{
			name:     "single network",
			cats:     types.SideEffectNetwork,
			expected: []string{"network"},
		},
		{
			name:     "single database",
			cats:     types.SideEffectDatabase,
			expected: []string{"database"},
		},
		{
			name:     "multiple write effects",
			cats:     types.SideEffectParamWrite | types.SideEffectReceiverWrite | types.SideEffectGlobalWrite,
			expected: []string{"param-write", "receiver-write", "global-write"},
		},
		{
			name:     "io and network combined",
			cats:     types.SideEffectIO | types.SideEffectNetwork,
			expected: []string{"io", "network"},
		},
		{
			name:     "typical sql writer module",
			cats:     types.SideEffectReceiverWrite | types.SideEffectFieldWrite,
			expected: []string{"receiver-write", "field-write"},
		},
		{
			name:     "typical sql connection module",
			cats:     types.SideEffectDatabase | types.SideEffectExternalCall,
			expected: []string{"database", "external-call"},
		},
		{
			name:     "control flow effects",
			cats:     types.SideEffectThrow | types.SideEffectChannel | types.SideEffectAsync,
			expected: []string{"throw", "channel", "async"},
		},
		{
			name:     "uncertainty markers",
			cats:     types.SideEffectExternalCall | types.SideEffectDynamicCall | types.SideEffectReflection,
			expected: []string{"external-call", "dynamic-call", "reflection"},
		},
		{
			name:     "all write effects",
			cats:     types.SideEffectParamWrite | types.SideEffectReceiverWrite | types.SideEffectGlobalWrite | types.SideEffectClosureWrite | types.SideEffectFieldWrite | types.SideEffectIndirectWrite,
			expected: []string{"param-write", "receiver-write", "global-write", "closure-write", "field-write", "indirect-write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := a.getSideEffectCategories(tt.cats)

			if tt.expected == nil && result != nil {
				t.Errorf("getSideEffectCategories(%v) = %v, want nil", tt.cats, result)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("getSideEffectCategories(%v) = %v (len=%d), want %v (len=%d)",
					tt.cats, result, len(result), tt.expected, len(tt.expected))
				return
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("getSideEffectCategories(%v)[%d] = %v, want %v",
						tt.cats, i, result[i], exp)
				}
			}
		})
	}
}

func TestHasSignificantSideEffects(t *testing.T) {
	a := &Analyzer{} // Minimal analyzer for testing helper methods

	tests := []struct {
		name     string
		effects  []string
		expected bool
	}{
		{
			name:     "empty effects",
			effects:  []string{},
			expected: false,
		},
		{
			name:     "nil effects",
			effects:  nil,
			expected: false,
		},
		{
			name:     "only receiver write - not significant",
			effects:  []string{"receiver-write"},
			expected: false,
		},
		{
			name:     "only field write - not significant",
			effects:  []string{"field-write"},
			expected: false,
		},
		{
			name:     "only param write - not significant",
			effects:  []string{"param-write"},
			expected: false,
		},
		{
			name:     "io is significant",
			effects:  []string{"io"},
			expected: true,
		},
		{
			name:     "network is significant",
			effects:  []string{"network"},
			expected: true,
		},
		{
			name:     "database is significant",
			effects:  []string{"database"},
			expected: true,
		},
		{
			name:     "global-write is significant",
			effects:  []string{"global-write"},
			expected: true,
		},
		{
			name:     "sql writer pattern - receiver only - not significant",
			effects:  []string{"receiver-write", "field-write"},
			expected: false,
		},
		{
			name:     "sql connection pattern - database - significant",
			effects:  []string{"database", "external-call"},
			expected: true,
		},
		{
			name:     "mixed with io",
			effects:  []string{"receiver-write", "io"},
			expected: true,
		},
		{
			name:     "external-call alone - not significant",
			effects:  []string{"external-call"},
			expected: false,
		},
		{
			name:     "throw alone - not significant",
			effects:  []string{"throw"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := a.hasSignificantSideEffects(tt.effects)
			if result != tt.expected {
				t.Errorf("hasSignificantSideEffects(%v) = %v, want %v",
					tt.effects, result, tt.expected)
			}
		})
	}
}

func TestCheckFunctionMetrics_PurityLoss(t *testing.T) {
	a := &Analyzer{} // Minimal analyzer for testing

	// Test purity loss detection
	existingSymbols := []SymbolInfo{
		{
			Name:        "processData",
			Type:        "function",
			FilePath:    "handler.go",
			Line:        10,
			Complexity:  5,
			LinesOfCode: 20,
			IsPure:      true,
			SideEffects: nil,
		},
	}

	newSymbols := []SymbolInfo{
		{
			Name:        "processData",
			Type:        "function",
			FilePath:    "handler.go",
			Line:        10,
			Complexity:  6,
			LinesOfCode: 25,
			IsPure:      false,
			SideEffects: []string{"io", "network"},
		},
	}

	params := DefaultAnalysisParams()
	findings := a.checkFunctionMetrics(nil, newSymbols, existingSymbols, params)

	// Should find purity loss
	var purityLossFinding *MetricsFinding
	for i := range findings {
		if findings[i].IssueType == MetricsIssuePurityLost {
			purityLossFinding = &findings[i]
			break
		}
	}

	if purityLossFinding == nil {
		t.Fatal("Expected to find purity loss finding, but none found")
	}

	if purityLossFinding.Severity != SeverityWarning {
		t.Errorf("Purity loss severity = %v, want %v", purityLossFinding.Severity, SeverityWarning)
	}

	if purityLossFinding.OldMetrics == nil || !purityLossFinding.OldMetrics.IsPure {
		t.Error("OldMetrics should show IsPure=true")
	}

	if purityLossFinding.NewMetrics == nil || purityLossFinding.NewMetrics.IsPure {
		t.Error("NewMetrics should show IsPure=false")
	}

	if len(purityLossFinding.NewMetrics.SideEffects) != 2 {
		t.Errorf("NewMetrics.SideEffects = %v, want [io, network]", purityLossFinding.NewMetrics.SideEffects)
	}
}

func TestCheckFunctionMetrics_NewImpureFunction(t *testing.T) {
	a := &Analyzer{} // Minimal analyzer for testing

	// Test new impure function detection (no existing symbol)
	existingSymbols := []SymbolInfo{}

	newSymbols := []SymbolInfo{
		{
			Name:        "fetchFromAPI",
			Type:        "function",
			FilePath:    "api.go",
			Line:        10,
			Complexity:  3,
			LinesOfCode: 15,
			IsPure:      false,
			SideEffects: []string{"network", "external-call"},
		},
	}

	params := DefaultAnalysisParams()
	findings := a.checkFunctionMetrics(nil, newSymbols, existingSymbols, params)

	// Should find impure function
	var impureFinding *MetricsFinding
	for i := range findings {
		if findings[i].IssueType == MetricsIssueImpureFunction {
			impureFinding = &findings[i]
			break
		}
	}

	if impureFinding == nil {
		t.Fatal("Expected to find impure function finding, but none found")
	}

	if impureFinding.Severity != SeverityInfo {
		t.Errorf("Impure function severity = %v, want %v", impureFinding.Severity, SeverityInfo)
	}
}

func TestCheckFunctionMetrics_NewFunctionReceiverWriteOnly(t *testing.T) {
	a := &Analyzer{} // Minimal analyzer for testing

	// Test new function with only receiver-write (should NOT be flagged)
	existingSymbols := []SymbolInfo{}

	newSymbols := []SymbolInfo{
		{
			Name:        "SetValue",
			Type:        "method",
			FilePath:    "state.go",
			Line:        10,
			Complexity:  2,
			LinesOfCode: 5,
			IsPure:      false,
			SideEffects: []string{"receiver-write"},
		},
	}

	params := DefaultAnalysisParams()
	findings := a.checkFunctionMetrics(nil, newSymbols, existingSymbols, params)

	// Should NOT find impure function finding (receiver-write is not significant)
	for _, finding := range findings {
		if finding.IssueType == MetricsIssueImpureFunction {
			t.Errorf("Should not flag receiver-write only as impure function, but found: %v", finding)
		}
	}
}

func TestCheckFunctionMetrics_PureToReceiverWrite(t *testing.T) {
	a := &Analyzer{} // Minimal analyzer for testing

	// Test pure function that gains receiver-write (should flag purity loss)
	existingSymbols := []SymbolInfo{
		{
			Name:        "calculate",
			Type:        "method",
			FilePath:    "calc.go",
			Line:        10,
			Complexity:  3,
			LinesOfCode: 10,
			IsPure:      true,
			SideEffects: nil,
		},
	}

	newSymbols := []SymbolInfo{
		{
			Name:        "calculate",
			Type:        "method",
			FilePath:    "calc.go",
			Line:        10,
			Complexity:  4,
			LinesOfCode: 12,
			IsPure:      false,
			SideEffects: []string{"receiver-write"},
		},
	}

	params := DefaultAnalysisParams()
	findings := a.checkFunctionMetrics(nil, newSymbols, existingSymbols, params)

	// Should find purity loss (even for receiver-write, since it was pure before)
	var purityLossFinding *MetricsFinding
	for i := range findings {
		if findings[i].IssueType == MetricsIssuePurityLost {
			purityLossFinding = &findings[i]
			break
		}
	}

	if purityLossFinding == nil {
		t.Fatal("Expected to find purity loss finding for pure->receiver-write change")
	}
}

func TestDetermineMetricsSeverity_PurityTypes(t *testing.T) {
	thresholds := DefaultMetricsThresholds()

	tests := []struct {
		name      string
		issueType MetricsIssueType
		expected  FindingSeverity
	}{
		{
			name:      "purity lost is warning",
			issueType: MetricsIssuePurityLost,
			expected:  SeverityWarning,
		},
		{
			name:      "impure function is info",
			issueType: MetricsIssueImpureFunction,
			expected:  SeverityInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := SymbolMetrics{} // Metrics don't affect purity severity
			result := DetermineMetricsSeverity(tt.issueType, metrics, thresholds)
			if result != tt.expected {
				t.Errorf("DetermineMetricsSeverity(%v) = %v, want %v",
					tt.issueType, result, tt.expected)
			}
		})
	}
}

func TestSideEffectsJSONSerialization(t *testing.T) {
	// Test that side effects serialize correctly in JSON output
	sym := SymbolInfo{
		Name:        "processData",
		Type:        "function",
		FilePath:    "handler.go",
		Line:        42,
		IsPure:      false,
		SideEffects: []string{"io", "network", "param-write"},
	}

	data, err := json.Marshal(sym)
	if err != nil {
		t.Fatalf("Failed to marshal SymbolInfo: %v", err)
	}

	jsonStr := string(data)

	// Verify side_effects array is present (is_pure:false omitted due to omitempty)
	if !strings.Contains(jsonStr, `"side_effects"`) {
		t.Errorf("Expected side_effects array in JSON output, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"io"`) {
		t.Error("Expected 'io' in side_effects array")
	}
	if !strings.Contains(jsonStr, `"network"`) {
		t.Error("Expected 'network' in side_effects array")
	}
	if !strings.Contains(jsonStr, `"param-write"`) {
		t.Error("Expected 'param-write' in side_effects array")
	}

	// Test that is_pure:true IS serialized (not omitted)
	pureSym := SymbolInfo{
		Name:        "pureFunc",
		Type:        "function",
		FilePath:    "pure.go",
		Line:        10,
		IsPure:      true,
		SideEffects: nil,
	}

	data, err = json.Marshal(pureSym)
	if err != nil {
		t.Fatalf("Failed to marshal pure SymbolInfo: %v", err)
	}

	jsonStr = string(data)
	if !strings.Contains(jsonStr, `"is_pure":true`) {
		t.Errorf("Expected is_pure:true for pure function, got: %s", jsonStr)
	}

	// Verify metrics also serialize correctly
	metrics := SymbolMetrics{
		Complexity:   5,
		LinesOfCode:  20,
		NestingDepth: 3,
		IsPure:       false,
		SideEffects:  []string{"database", "global-write"},
	}

	data, err = json.Marshal(metrics)
	if err != nil {
		t.Fatalf("Failed to marshal SymbolMetrics: %v", err)
	}

	jsonStr = string(data)
	if !strings.Contains(jsonStr, `"database"`) {
		t.Error("Expected 'database' in SymbolMetrics side_effects")
	}
	if !strings.Contains(jsonStr, `"global-write"`) {
		t.Error("Expected 'global-write' in SymbolMetrics side_effects")
	}

	// Verify round-trip unmarshaling preserves side effects
	var unmarshalled SymbolInfo
	if err := json.Unmarshal(data, &unmarshalled); err == nil {
		// This will fail because we're unmarshaling metrics into SymbolInfo
		// Let's test proper unmarshaling
	}

	var unmarshalledMetrics SymbolMetrics
	if err := json.Unmarshal(data, &unmarshalledMetrics); err != nil {
		t.Fatalf("Failed to unmarshal SymbolMetrics: %v", err)
	}
	if len(unmarshalledMetrics.SideEffects) != 2 {
		t.Errorf("Expected 2 side effects after unmarshal, got %d", len(unmarshalledMetrics.SideEffects))
	}
}

func TestMetricsFindingWithSideEffects(t *testing.T) {
	finding := MetricsFinding{
		Severity:    SeverityWarning,
		Description: "Function 'processData' lost purity: [io, network]",
		Symbol: SymbolInfo{
			Name:        "processData",
			Type:        "function",
			FilePath:    "handler.go",
			Line:        42,
			IsPure:      false,
			SideEffects: []string{"io", "network"},
		},
		IssueType:  MetricsIssuePurityLost,
		Issue:      "Previously pure function now has side effects: [io, network]",
		Suggestion: "Consider keeping pure functions pure or extracting the impure operations",
		OldMetrics: &SymbolMetrics{
			IsPure:      true,
			SideEffects: nil,
		},
		NewMetrics: &SymbolMetrics{
			IsPure:      false,
			SideEffects: []string{"io", "network"},
		},
	}

	data, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("Failed to marshal MetricsFinding: %v", err)
	}

	jsonStr := string(data)

	// Verify structure
	if !strings.Contains(jsonStr, `"issue_type":"purity_lost"`) {
		t.Errorf("Expected issue_type 'purity_lost' in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"severity":"warning"`) {
		t.Error("Expected severity 'warning' in JSON")
	}

	// Verify old/new metrics show purity change
	var unmarshalled MetricsFinding
	if err := json.Unmarshal(data, &unmarshalled); err != nil {
		t.Fatalf("Failed to unmarshal MetricsFinding: %v", err)
	}

	if unmarshalled.OldMetrics == nil || !unmarshalled.OldMetrics.IsPure {
		t.Error("OldMetrics should have IsPure=true")
	}
	if unmarshalled.NewMetrics == nil || unmarshalled.NewMetrics.IsPure {
		t.Error("NewMetrics should have IsPure=false")
	}
	if len(unmarshalled.NewMetrics.SideEffects) != 2 {
		t.Errorf("NewMetrics.SideEffects should have 2 items, got %d", len(unmarshalled.NewMetrics.SideEffects))
	}
}
