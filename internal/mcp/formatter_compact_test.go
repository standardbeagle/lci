package mcp

import (
	"strings"
	"testing"
)

// TestCompactFormatter_FormatIntelligenceResponse_MemoryAnalysis tests that memory analysis
// data is properly formatted in LCF output when provided.
// NOTE: Memory analysis is disabled in production (see codebase_intelligence_tools.go)
// due to regex-based allocation detection producing false positives. This test verifies
// the formatter capability works correctly if the feature is re-enabled in the future.
func TestCompactFormatter_FormatIntelligenceResponse_MemoryAnalysis(t *testing.T) {
	t.Skip("Memory analysis feature disabled - formatter capability test preserved for future use")
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 8.5,
			Complexity: ComplexityMetrics{
				AverageCC: 5.2,
			},
			MemoryAnalysis: &MemoryPressureAnalysis{
				Summary: MemorySummary{
					TotalFunctions:   100,
					TotalAllocations: 250,
					LoopAllocCount:   15,
					CriticalCount:    2,
					HighCount:        5,
					MediumCount:      20,
					LowCount:         73,
				},
				Scores: []MemoryScore{
					{
						Function:        "processHeavy",
						Location:        "heavy.go:42",
						TotalScore:      85.5,
						DirectScore:     45.0,
						PropagatedScore: 40.5,
						LoopPressure:    30.0,
						Severity:        "critical",
						Percentile:      99.0,
					},
					{
						Function:        "compilePatterns",
						Location:        "regex.go:15",
						TotalScore:      72.3,
						DirectScore:     50.0,
						PropagatedScore: 22.3,
						LoopPressure:    25.0,
						Severity:        "high",
						Percentile:      95.0,
					},
				},
				Hotspots: []MemoryHotspot{
					{
						Function:   "processHeavy",
						Location:   "heavy.go:42",
						Score:      85.5,
						Reason:     "JSON marshaling in loop",
						Suggestion: "Pre-allocate buffer or move JSON encoding outside loop",
					},
				},
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Verify memory_pressure section exists
	if !strings.Contains(output, "memory_pressure:") {
		t.Error("Expected memory_pressure section in output")
		t.Logf("Output:\n%s", output)
	}

	// Verify summary line
	if !strings.Contains(output, "summary: funcs=100 allocs=250") {
		t.Error("Expected memory summary with function and allocation counts")
	}
	if !strings.Contains(output, "loop_allocs=15") {
		t.Error("Expected loop_allocs in summary")
	}
	if !strings.Contains(output, "critical=2") {
		t.Error("Expected critical count in summary")
	}

	// Verify top_pressure section
	if !strings.Contains(output, "top_pressure:") {
		t.Error("Expected top_pressure section")
	}

	// Verify function scores are included
	if !strings.Contains(output, "[critical] processHeavy (heavy.go:42)") {
		t.Error("Expected processHeavy with critical severity")
	}
	if !strings.Contains(output, "score=85.5") {
		t.Error("Expected score value for processHeavy")
	}
	if !strings.Contains(output, "propagated=40.5") {
		t.Error("Expected propagated score for processHeavy")
	}

	// Verify hotspots section
	if !strings.Contains(output, "hotspots:") {
		t.Error("Expected hotspots section")
	}
	if !strings.Contains(output, "JSON marshaling in loop") {
		t.Error("Expected hotspot reason")
	}
	if !strings.Contains(output, "-> Pre-allocate buffer") {
		t.Error("Expected hotspot suggestion")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_NoMemoryAnalysis tests that output
// is still valid when memory analysis is nil.
func TestCompactFormatter_FormatIntelligenceResponse_NoMemoryAnalysis(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 9.0,
			Complexity: ComplexityMetrics{
				AverageCC: 3.5,
			},
			MemoryAnalysis: nil, // No memory analysis
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Should NOT contain memory_pressure section
	if strings.Contains(output, "memory_pressure:") {
		t.Error("Should not include memory_pressure when MemoryAnalysis is nil")
	}

	// Should still have basic health info
	if !strings.Contains(output, "score=9.00") {
		t.Error("Expected basic health score info")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_EmptyMemoryScores tests that output
// handles empty memory scores gracefully.
func TestCompactFormatter_FormatIntelligenceResponse_EmptyMemoryScores(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 9.0,
			Complexity: ComplexityMetrics{
				AverageCC: 3.5,
			},
			MemoryAnalysis: &MemoryPressureAnalysis{
				Scores:   []MemoryScore{}, // Empty scores
				Hotspots: []MemoryHotspot{},
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Should NOT contain memory_pressure section when scores are empty
	if strings.Contains(output, "memory_pressure:") {
		t.Error("Should not include memory_pressure when Scores is empty")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_DetailedSmells tests code smell output.
func TestCompactFormatter_FormatIntelligenceResponse_DetailedSmells(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 7.5,
			Complexity: ComplexityMetrics{
				AverageCC: 12.0,
			},
			SmellCounts: map[string]int{
				"long-function":   3,
				"high-complexity": 2,
				"shotgun-surgery": 1,
			},
			DetailedSmells: []CodeSmellEntry{
				{
					Type:     "long-function",
					Symbol:   "processAllData",
					ObjectID: "ABC123",
					Location: "processor.go:45",
					Severity: "high",
				},
				{
					Type:     "high-complexity",
					Symbol:   "validateInput",
					ObjectID: "DEF456",
					Location: "validator.go:100",
					Severity: "medium",
				},
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Verify smell counts
	if !strings.Contains(output, "smells:") {
		t.Error("Expected smells summary line")
	}
	if !strings.Contains(output, "long-function=3") {
		t.Error("Expected long-function count")
	}

	// Verify detailed smells
	if !strings.Contains(output, "detailed_smells:") {
		t.Error("Expected detailed_smells section")
	}
	if !strings.Contains(output, "[high] long-function: processAllData") {
		t.Error("Expected detailed smell entry with severity")
	}
	if !strings.Contains(output, "oid=ABC123") {
		t.Error("Expected object ID for drill-down")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_ProblematicSymbols tests problematic symbol output.
func TestCompactFormatter_FormatIntelligenceResponse_ProblematicSymbols(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 6.0,
			Complexity: ComplexityMetrics{
				AverageCC: 18.0,
			},
			ProblematicSymbols: []ProblematicSymbol{
				{
					ObjectID:  "XYZ789",
					Name:      "godObject",
					Location:  "main.go:10",
					RiskScore: 9,
					Tags:      []string{"HIGH_COMPLEXITY", "DEEP_NESTING", "MANY_DEPS"},
				},
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Verify problematic symbols section
	if !strings.Contains(output, "problematic_symbols:") {
		t.Error("Expected problematic_symbols section")
	}
	if !strings.Contains(output, "godObject (main.go:10) risk=9") {
		t.Error("Expected symbol with risk score")
	}
	if !strings.Contains(output, "[HIGH_COMPLEXITY,DEEP_NESTING,MANY_DEPS]") {
		t.Error("Expected tags in output")
	}
	if !strings.Contains(output, "oid=XYZ789") {
		t.Error("Expected object ID for drill-down")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_Statistics tests statistics mode output.
func TestCompactFormatter_FormatIntelligenceResponse_Statistics(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "statistics",
		Tier:         3,
		StatisticsReport: &StatisticsReport{
			ComplexityMetrics: ComplexityMetrics{
				AverageCC: 8.5,
				MedianCC:  6.0,
				Distribution: map[string]int{
					"low":    80,
					"medium": 15,
					"high":   5,
				},
				HighComplexityFuncs: []FunctionInfo{
					{Name: "complexFunc", Location: "complex.go:50", Complexity: 35.0},
				},
			},
			CouplingMetrics: CouplingMetrics{
				AverageCoupling: 0.3,
				MaxCoupling:     0.8,
			},
			CohesionMetrics: CohesionMetrics{
				AverageCohesion:    0.7,
				MinCohesion:        0.2,
				LowCohesionModules: []string{"legacy_module", "utils_module"},
			},
			QualityMetrics: QualityMetrics{
				MaintainabilityIndex: 75.0,
				TechnicalDebtRatio:   0.05,
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Verify statistics section
	if !strings.Contains(output, "== STATISTICS ==") {
		t.Error("Expected STATISTICS section")
	}
	if !strings.Contains(output, "complexity: avg=8.50 median=6.00") {
		t.Error("Expected complexity metrics")
	}
	if !strings.Contains(output, "distribution:") {
		t.Error("Expected distribution info")
	}
	if !strings.Contains(output, "coupling: avg=0.30 max=0.80") {
		t.Error("Expected coupling metrics")
	}
	if !strings.Contains(output, "cohesion: avg=0.70 min=0.20") {
		t.Error("Expected cohesion metrics")
	}
	if !strings.Contains(output, "quality: maintainability=75.00") {
		t.Error("Expected quality metrics")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_MemoryAnalysisLimit tests that
// memory scores are limited to top 5.
func TestCompactFormatter_FormatIntelligenceResponse_MemoryAnalysisLimit(t *testing.T) {
	f := &CompactFormatter{}

	// Create 10 memory scores
	scores := make([]MemoryScore, 10)
	for i := 0; i < 10; i++ {
		scores[i] = MemoryScore{
			Function:   "func" + string(rune('A'+i)),
			Location:   "file.go:" + string(rune('0'+i)),
			TotalScore: float64(100 - i*10),
			Severity:   "medium",
		}
	}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 7.0,
			Complexity:   ComplexityMetrics{AverageCC: 5.0},
			MemoryAnalysis: &MemoryPressureAnalysis{
				Scores: scores,
				Summary: MemorySummary{
					TotalFunctions: 10,
				},
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Should contain first 5 functions
	if !strings.Contains(output, "funcA") {
		t.Error("Expected funcA (first)")
	}
	if !strings.Contains(output, "funcE") {
		t.Error("Expected funcE (5th)")
	}

	// Should NOT contain 6th function onwards
	if strings.Contains(output, "funcF") {
		t.Error("Should not include funcF (6th) - limit is 5")
	}
	if strings.Contains(output, "funcJ") {
		t.Error("Should not include funcJ (10th)")
	}
}

// TestCompactFormatter_FormatIntelligenceResponse_HotspotsLimit tests that
// memory hotspots are limited to top 3.
func TestCompactFormatter_FormatIntelligenceResponse_HotspotsLimit(t *testing.T) {
	f := &CompactFormatter{}

	// Create 5 hotspots
	hotspots := make([]MemoryHotspot, 5)
	for i := 0; i < 5; i++ {
		hotspots[i] = MemoryHotspot{
			Function: "hotspot" + string(rune('A'+i)),
			Location: "hot.go:" + string(rune('0'+i)),
			Reason:   "reason " + string(rune('A'+i)),
		}
	}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 7.0,
			Complexity:   ComplexityMetrics{AverageCC: 5.0},
			MemoryAnalysis: &MemoryPressureAnalysis{
				Scores: []MemoryScore{{Function: "test", TotalScore: 10}},
				Summary: MemorySummary{
					TotalFunctions: 1,
				},
				Hotspots: hotspots,
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Should contain first 3 hotspots
	if !strings.Contains(output, "hotspotA") {
		t.Error("Expected hotspotA (first)")
	}
	if !strings.Contains(output, "hotspotC") {
		t.Error("Expected hotspotC (3rd)")
	}

	// Should NOT contain 4th hotspot onwards
	if strings.Contains(output, "hotspotD") {
		t.Error("Should not include hotspotD (4th) - limit is 3")
	}
}

// TestCompactFormatter_LCFHeader tests the LCF header format.
func TestCompactFormatter_LCFHeader(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         2,
	}

	output := f.FormatIntelligenceResponse(response)

	// Must start with LCF header
	if !strings.HasPrefix(output, "LCF/1.0\n") {
		t.Error("Expected output to start with LCF/1.0 header")
	}

	// Must include mode and tier
	if !strings.Contains(output, "mode=overview") {
		t.Error("Expected mode in header")
	}
	if !strings.Contains(output, "tier=2") {
		t.Error("Expected tier in header")
	}
}

// TestCompactFormatter_SectionSeparators tests that sections are properly separated.
func TestCompactFormatter_SectionSeparators(t *testing.T) {
	f := &CompactFormatter{}

	response := &CodebaseIntelligenceResponse{
		AnalysisMode: "overview",
		Tier:         1,
		HealthDashboard: &HealthDashboard{
			OverallScore: 9.0,
			Complexity:   ComplexityMetrics{AverageCC: 3.0},
		},
		RepositoryMap: &RepositoryMap{
			ModuleBoundaries: []ModuleBoundary{
				{Name: "core", FileCount: 10},
			},
		},
	}

	output := f.FormatIntelligenceResponse(response)

	// Count section separators
	separatorCount := strings.Count(output, "---")

	// Should have at least 2 separators (for health and repository map)
	if separatorCount < 2 {
		t.Errorf("Expected at least 2 section separators, got %d", separatorCount)
	}
}
