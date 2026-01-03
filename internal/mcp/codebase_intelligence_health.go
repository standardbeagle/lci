package mcp

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"time"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/bmatcuk/doublestar/v4"
)

// isInsightExcludedFile returns true if the file should be excluded from
// code insight reports (but still counted in aggregate statistics).
// Uses config-based exclusion patterns which include default test file patterns.
func (s *Server) isInsightExcludedFile(path string) bool {
	// Config-based exclusions (includes all test file patterns by default)
	if s.cfg != nil {
		for _, pattern := range s.cfg.Exclude {
			matched, err := doublestar.Match(pattern, path)
			if err != nil {
				continue // Skip invalid patterns
			}
			if matched {
				return true
			}
		}
	}

	return false
}

// ============================================================================
// Code Smell Detection Thresholds
// ============================================================================

const (
	// Long function thresholds
	longFunctionThreshold    = 50  // Functions > 50 lines trigger warning
	longFunctionHighSeverity = 100 // Functions > 100 lines are high severity

	// High complexity thresholds
	highComplexityThreshold = 10 // CC > 10 triggers warning
	highComplexityHighSev   = 20 // CC > 20 is high severity

	// God class thresholds
	godClassThreshold     = 15 // Classes with > 15 methods
	godClassHighSeverity  = 25 // Classes with > 25 methods

	// Shotgun surgery thresholds
	shotgunSurgeryThreshold = 10 // Symbols with > 10 dependents
	shotgunSurgeryHighSev   = 20 // Symbols with > 20 dependents

	// Risk score thresholds for problematic symbols
	riskScoreCutoff       = 5 // Only show symbols with risk >= 5
	maxDetailedSmells     = 5 // Limit to top 5 smells
	maxProblematicSymbols = 5 // Limit to top 5 problematic symbols
)

// calculateComplexityMetricsFromFiles calculates complexity metrics from file data
func (s *Server) calculateComplexityMetricsFromFiles(allFiles []*types.FileInfo) ComplexityMetrics {
	complexities := make([]float64, 0)
	distribution := make(map[string]int)
	highComplexityFuncs := make([]FunctionInfo, 0)

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
				// Use actual cyclomatic complexity from parsed data
				cc := sym.Complexity
				if cc <= 0 {
					cc = 1 // Minimum complexity is 1
				}
				complexity := float64(cc)
				complexities = append(complexities, complexity)

				// Categorize by cyclomatic complexity ranges using defined thresholds
				if cc <= complexityThresholdLow {
					distribution["low"]++
				} else if cc <= complexityThresholdHigh {
					distribution["medium"]++
				} else {
					distribution["high"]++
					// Track high complexity functions for reporting (exclude test files)
					if len(highComplexityFuncs) < 10 && !s.isInsightExcludedFile(file.Path) {
						objectID := searchtypes.EncodeSymbolID(sym.ID)
						highComplexityFuncs = append(highComplexityFuncs, FunctionInfo{
							ObjectID:   objectID,
							Name:       sym.Name,
							Location:   fmt.Sprintf("%s:%d", file.Path, sym.Line),
							Complexity: complexity,
						})
					}
				}
			}
		}
	}

	// Calculate average and median
	avgComplexity := 0.0
	medianComplexity := 0.0
	if len(complexities) > 0 {
		for _, c := range complexities {
			avgComplexity += c
		}
		avgComplexity /= float64(len(complexities))

		// Simple median (sort and take middle)
		sorted := make([]float64, len(complexities))
		copy(sorted, complexities)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		if len(sorted)%2 == 0 {
			medianComplexity = (sorted[mid-1] + sorted[mid]) / 2
		} else {
			medianComplexity = sorted[mid]
		}
	}

	// Calculate percentiles
	percentiles := map[string]float64{
		"p50": medianComplexity,
		"p75": avgComplexity * 1.2, // Approximation
		"p90": avgComplexity * 1.5, // Approximation
	}

	return ComplexityMetrics{
		AverageCC:           avgComplexity,
		MedianCC:            medianComplexity,
		Percentiles:         percentiles,
		HighComplexityFuncs: highComplexityFuncs,
		Distribution:        distribution,
	}
}

// identifyHotspotsFromFiles identifies code hotspots from file data
func (s *Server) identifyHotspotsFromFiles(allFiles []*types.FileInfo) []Hotspot {
	hotspots := make([]Hotspot, 0)

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
				cc := sym.Complexity
				if cc <= 0 {
					cc = 1
				}

				lineCount := sym.EndLine - sym.Line
				if lineCount <= 0 {
					lineCount = 1
				}

				// Hotspot criteria: high complexity OR large function
				if cc > hotspotComplexityThreshold || lineCount > hotspotLinecountThreshold {
					riskScore := float64(cc)*0.7 + float64(lineCount)*0.03
					if riskScore > riskScoreMax {
						riskScore = riskScoreMax
					}

					hotspots = append(hotspots, Hotspot{
						Location:   fmt.Sprintf("%s:%s:%d", file.Path, sym.Name, sym.Line),
						Complexity: float64(cc),
						RiskScore:  riskScore,
					})
				}
			}
		}
	}

	// Sort by risk score (highest first)
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].RiskScore > hotspots[j].RiskScore
	})

	return hotspots
}

// calculateComplexityMetricsForNodes calculates complexity metrics from universal symbol nodes
func (s *Server) calculateComplexityMetricsForNodes(allSymbols []*types.UniversalSymbolNode) ComplexityMetrics {
	complexities := make([]float64, 0)
	distribution := make(map[string]int)

	for _, node := range allSymbols {
		if node.Identity.Kind == types.SymbolKindFunction || node.Identity.Kind == types.SymbolKindMethod {
			lineCount := node.Identity.Location.Line
			if lineCount <= 0 {
				lineCount = 5
			}
			complexity := float64(lineCount)
			complexities = append(complexities, complexity)

			if complexity <= float64(complexityThresholdLow) {
				distribution["low"]++
			} else if complexity <= float64(complexityThresholdHigh) {
				distribution["medium"]++
			} else {
				distribution["high"]++
			}
		}
	}

	avgComplexity := 0.0
	if len(complexities) > 0 {
		for _, c := range complexities {
			avgComplexity += c
		}
		avgComplexity /= float64(len(complexities))
	}

	return ComplexityMetrics{
		AverageCC:           avgComplexity,
		MedianCC:            avgComplexity,
		Percentiles:         map[string]float64{"p50": avgComplexity, "p75": avgComplexity * 1.2},
		HighComplexityFuncs: make([]FunctionInfo, 0),
		Distribution:        distribution,
	}
}

// identifyHotspots identifies code hotspots from universal symbol nodes
func (s *Server) identifyHotspots(allSymbols []*types.UniversalSymbolNode) []Hotspot {
	hotspots := make([]Hotspot, 0)

	for _, node := range allSymbols {
		if node.Identity.Kind == types.SymbolKindFunction || node.Identity.Kind == types.SymbolKindMethod {
			totalUsage := node.Usage.ReferenceCount + node.Usage.CallCount
			if totalUsage > highUsageThreshold {
				riskScore := float64(totalUsage) * 0.5
				if riskScore > riskScoreMax {
					riskScore = riskScoreMax
				}

				hotspots = append(hotspots, Hotspot{
					Location:   fmt.Sprintf("%s:%s", node.Identity.ID.String(), node.Identity.Name),
					Complexity: float64(totalUsage),
					RiskScore:  riskScore,
				})
			}
		}
	}

	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].RiskScore > hotspots[j].RiskScore
	})

	return hotspots
}

// calculateOverallHealthScore calculates overall health score (0-10)
func (s *Server) calculateOverallHealthScore(complexity ComplexityMetrics, totalFiles int) float64 {
	score := 10.0

	totalFunctions := complexity.Distribution["low"] + complexity.Distribution["medium"] + complexity.Distribution["high"]
	if totalFunctions == 0 {
		totalFunctions = 1
	}

	highComplexityRatio := float64(complexity.Distribution["high"]) / float64(totalFunctions)
	score -= highComplexityRatio * 4.0

	mediumComplexityRatio := float64(complexity.Distribution["medium"]) / float64(totalFunctions)
	score -= mediumComplexityRatio * 1.5

	if complexity.AverageCC > complexityThresholdLow {
		deduction := (complexity.AverageCC - complexityThresholdLow) * 0.15
		if deduction > 3.0 {
			deduction = 3.0
		}
		score -= deduction
	}

	lowComplexityRatio := float64(complexity.Distribution["low"]) / float64(totalFunctions)
	if lowComplexityRatio > 0.8 {
		score += 1.0
	} else if lowComplexityRatio > 0.6 {
		score += 0.5
	}

	if score < 0 {
		score = 0
	}
	if score > riskScoreMax {
		score = riskScoreMax
	}

	return score
}


// calculateTechnicalDebtRatioFromFiles calculates technical debt as a percentage
func (s *Server) calculateTechnicalDebtRatioFromFiles(allFiles []*types.FileInfo) float64 {
	totalSymbols := 0
	debtSymbols := 0

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			totalSymbols++
			if sym.Complexity > complexityThresholdModerate || len(sym.IncomingRefs) > highReferenceCountThreshold {
				debtSymbols++
			}
		}
	}

	if totalSymbols == 0 {
		return 0.0
	}

	return float64(debtSymbols) / float64(totalSymbols)
}


// estimateDebtRemediationTime estimates remediation time
func (s *Server) estimateDebtRemediationTime(ratio float64) string {
	switch {
	case ratio < 0.05:
		return "1 day"
	case ratio < 0.10:
		return "1 week"
	case ratio < 0.20:
		return "2 weeks"
	case ratio < 0.30:
		return "1 month"
	default:
		return "3+ months"
	}
}

// identifyDebtComponents identifies components with high debt
func (s *Server) identifyDebtComponents(allFiles []*types.FileInfo) []string {
	var components []string
	debtByFile := make(map[string]int)

	for _, file := range allFiles {
		debtCount := 0
		for _, sym := range file.EnhancedSymbols {
			if sym.Complexity > complexityThresholdModerate || len(sym.IncomingRefs) > highReferenceCountThreshold {
				debtCount++
			}
		}
		if debtCount > highUsageThreshold {
			debtByFile[file.Path] = debtCount
		}
	}

	type fileDebt struct {
		path  string
		count int
	}
	var fileDebts []fileDebt
	for path, count := range debtByFile {
		fileDebts = append(fileDebts, fileDebt{path: path, count: count})
	}
	sort.Slice(fileDebts, func(i, j int) bool {
		return fileDebts[i].count > fileDebts[j].count
	})

	for _, fd := range fileDebts {
		if len(components) < 5 {
			components = append(components, fmt.Sprintf("%s (%d issues)", fd.path, fd.count))
		}
	}

	return components
}

// buildPuritySummary builds a summary of function purity from side effect analysis
func (s *Server) buildPuritySummary() *PuritySummary {
	propagator := s.goroutineIndex.GetSideEffectPropagator()
	if propagator == nil {
		return nil
	}

	allEffects := propagator.GetAllSideEffects()
	if len(allEffects) == 0 {
		return nil
	}

	summary := &PuritySummary{
		TotalFunctions: len(allEffects),
	}

	for _, info := range allEffects {
		if info == nil {
			continue
		}

		// Skip test files from purity reporting
		if s.isInsightExcludedFile(info.FilePath) {
			continue
		}

		if info.IsPure {
			summary.PureFunctions++
		} else {
			summary.ImpureFunctions++
		}

		combined := info.Categories | info.TransitiveCategories
		if combined&types.SideEffectParamWrite != 0 {
			summary.WithParamWrites++
		}
		if combined&types.SideEffectGlobalWrite != 0 {
			summary.WithGlobalWrites++
		}
		if combined&(types.SideEffectIO|types.SideEffectNetwork|types.SideEffectDatabase) != 0 {
			summary.WithIOEffects++
		}
		if combined&types.SideEffectThrow != 0 {
			summary.WithThrows++
		}
		if combined&types.SideEffectExternalCall != 0 {
			summary.WithExternalCalls++
		}
	}

	if summary.TotalFunctions > 0 {
		summary.PurityRatio = float64(summary.PureFunctions) / float64(summary.TotalFunctions)
	}
	summary.DetailedQuery = `side_effects {"mode": "impure", "include_reasons": true}`

	return summary
}

// calculateDetailedCodeSmells detects 5 types of code smells with severity levels
func (s *Server) calculateDetailedCodeSmells(allFiles []*types.FileInfo) []CodeSmellEntry {
	var smells []CodeSmellEntry

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			basePath := filepath.Base(file.Path)

			// 1. Long function (>50 lines)
			lineCount := sym.EndLine - sym.Line
			if lineCount > longFunctionThreshold {
				severity := "medium"
				if lineCount > longFunctionHighSeverity {
					severity = "high"
				}
				smells = append(smells, CodeSmellEntry{
					Type:        "long-function",
					Symbol:      sym.Name,
					ObjectID:    searchtypes.EncodeSymbolID(sym.ID),
					Location:    fmt.Sprintf("%s:%d", basePath, sym.Line),
					Severity:    severity,
					Description: fmt.Sprintf("%d lines (recommend < 30)", lineCount),
				})
			}

			// 2. High cyclomatic complexity (>10)
			if sym.Complexity > highComplexityThreshold {
				severity := "medium"
				if sym.Complexity > highComplexityHighSev {
					severity = "high"
				}
				smells = append(smells, CodeSmellEntry{
					Type:        "high-complexity",
					Symbol:      sym.Name,
					ObjectID:    searchtypes.EncodeSymbolID(sym.ID),
					Location:    fmt.Sprintf("%s:%d", basePath, sym.Line),
					Severity:    severity,
					Description: fmt.Sprintf("CC=%d (recommend < 10)", sym.Complexity),
				})
			}

			// 3. God class (>15 methods)
			if sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeStruct {
				methodCount := s.countChildMethods(file, sym)
				if methodCount > godClassThreshold {
					severity := "medium"
					if methodCount > godClassHighSeverity {
						severity = "high"
					}
					smells = append(smells, CodeSmellEntry{
						Type:        "god-class",
						Symbol:      sym.Name,
						ObjectID:    searchtypes.EncodeSymbolID(sym.ID),
						Location:    fmt.Sprintf("%s:%d", basePath, sym.Line),
						Severity:    severity,
						Description: fmt.Sprintf("%d methods (consider splitting)", methodCount),
					})
				}
			}

			// 4. Shotgun surgery (high impact = many dependents)
			impactCount := len(sym.IncomingRefs)
			if impactCount > shotgunSurgeryThreshold {
				severity := "medium"
				if impactCount > shotgunSurgeryHighSev {
					severity = "high"
				}
				smells = append(smells, CodeSmellEntry{
					Type:        "shotgun-surgery",
					Symbol:      sym.Name,
					ObjectID:    searchtypes.EncodeSymbolID(sym.ID),
					Location:    fmt.Sprintf("%s:%d", basePath, sym.Line),
					Severity:    severity,
					Description: fmt.Sprintf("changes affect %d locations", impactCount),
				})
			}
		}
	}

	smells = sortAndLimitSmells(smells, maxDetailedSmells)
	return smells
}

// countChildMethods counts methods belonging to a class/struct
func (s *Server) countChildMethods(file *types.FileInfo, parent *types.EnhancedSymbol) int {
	count := 0
	for _, sym := range file.EnhancedSymbols {
		if sym.Type == types.SymbolTypeMethod &&
			sym.Line > parent.Line && sym.EndLine <= parent.EndLine {
			count++
		}
	}
	return count
}

// sortAndLimitSmells sorts smells by severity and limits to maxCount
func sortAndLimitSmells(smells []CodeSmellEntry, maxCount int) []CodeSmellEntry {
	sort.Slice(smells, func(i, j int) bool {
		return severityRank(smells[i].Severity) > severityRank(smells[j].Severity)
	})

	if len(smells) > maxCount {
		smells = smells[:maxCount]
	}
	return smells
}

// severityRank returns numeric rank for severity comparison
func severityRank(sev string) int {
	switch sev {
	case "high":
		return 2
	case "medium":
		return 1
	default:
		return 0
	}
}

// identifyProblematicSymbols finds symbols with quality issues above risk cutoff
func (s *Server) identifyProblematicSymbols(allFiles []*types.FileInfo) []ProblematicSymbol {
	var problematic []ProblematicSymbol

	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			tags, riskScore := s.calculateSymbolRiskAndTags(sym)

			if riskScore >= riskScoreCutoff {
				basePath := filepath.Base(file.Path)
				problematic = append(problematic, ProblematicSymbol{
					ObjectID:  searchtypes.EncodeSymbolID(sym.ID),
					Name:      sym.Name,
					Location:  fmt.Sprintf("%s:%d", basePath, sym.Line),
					RiskScore: riskScore,
					Tags:      tags,
				})
			}
		}
	}

	sort.Slice(problematic, func(i, j int) bool {
		return problematic[i].RiskScore > problematic[j].RiskScore
	})

	if len(problematic) > maxProblematicSymbols {
		problematic = problematic[:maxProblematicSymbols]
	}
	return problematic
}

// calculateSymbolRiskAndTags calculates risk score and quality tags for a symbol
func (s *Server) calculateSymbolRiskAndTags(sym *types.EnhancedSymbol) ([]string, int) {
	var tags []string
	riskScore := 0

	if sym.Complexity > 15 {
		tags = append(tags, "HIGH_COMPLEXITY")
		riskScore += 3
	}

	lineCount := sym.EndLine - sym.Line
	if lineCount > 100 {
		tags = append(tags, "LARGE_FUNCTION")
		riskScore += 2
	}

	if len(sym.IncomingRefs) > 15 {
		tags = append(tags, "HIGH_COUPLING")
		riskScore += 2
	}

	if len(sym.OutgoingRefs) > 15 {
		tags = append(tags, "MANY_DEPENDENCIES")
		riskScore += 2
	}

	if riskScore > 10 {
		riskScore = 10
	}

	return tags, riskScore
}

// countSmellsByType aggregates smell counts by type
func (s *Server) countSmellsByType(smells []CodeSmellEntry) map[string]int {
	counts := make(map[string]int)
	for _, smell := range smells {
		counts[smell.Type]++
	}
	return counts
}

// getMaintainabilityRating gets maintainability rating from quality score
func getMaintainabilityRating(score float64) string {
	switch {
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

// calculateMaintainabilityIndex calculates maintainability index
func calculateMaintainabilityIndex(complexityMetrics, qualityMetrics map[string]interface{}) float64 {
	complexity := complexityMetrics["average_cyclomatic_complexity"].(float64)
	quality := qualityMetrics["quality_score"].(float64)

	mi := (171.0 - 5.2*complexity - 0.23*complexity - 16.2*math.Log10(complexity+1)) * (quality / 100.0)
	return mi
}

// calculateTechnicalDebtRatio calculates technical debt ratio
func calculateTechnicalDebtRatio(complexityMetrics, qualityMetrics map[string]interface{}) float64 {
	complexity := complexityMetrics["average_cyclomatic_complexity"].(float64)
	quality := qualityMetrics["quality_score"].(float64)

	td := (complexity / 10.0) * (1.0 - quality/100.0)
	return td
}

// calculateQualityMetricsFromComplexity derives quality metrics from complexity
func calculateQualityMetricsFromComplexity(complexity ComplexityMetrics) QualityMetrics {
	mi := maintainabilityIndexMax - (complexity.AverageCC * 2.0)
	if mi < maintainabilityIndexMin {
		mi = maintainabilityIndexMin
	}
	if mi > maintainabilityIndexMax {
		mi = maintainabilityIndexMax
	}

	totalFuncs := complexity.Distribution["low"] + complexity.Distribution["medium"] + complexity.Distribution["high"]
	debtRatio := 0.0
	if totalFuncs > 0 {
		debtRatio = float64(complexity.Distribution["high"]) / float64(totalFuncs)
	}

	return QualityMetrics{
		MaintainabilityIndex: mi,
		TechnicalDebtRatio:   debtRatio,
	}
}

// buildHealthDashboard builds the health dashboard component
func (s *Server) buildHealthDashboard(
	ctx context.Context,
	args CodebaseIntelligenceParams,
	maxResults int,
) (*HealthDashboard, error) {
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(args.Languages) > 0 {
		allFiles = s.filterFilesByLanguage(allFiles, args.Languages)
	}
	if len(allFiles) == 0 {
		return nil, fmt.Errorf("no files found in index (or matching language filter)")
	}

	complexityMetrics := s.calculateComplexityMetricsFromFiles(allFiles)

	allHotspots := s.identifyHotspotsFromFiles(allFiles)
	hotspots := allHotspots
	if maxResults > 0 && len(allHotspots) > maxResults {
		hotspots = allHotspots[:maxResults]
	}

	overallScore := s.calculateOverallHealthScore(complexityMetrics, len(allFiles))

	debtRatio := s.calculateTechnicalDebtRatioFromFiles(allFiles)
	detailedSmells := s.calculateDetailedCodeSmells(allFiles)
	problematicSymbols := s.identifyProblematicSymbols(allFiles)
	smellCounts := s.countSmellsByType(detailedSmells)
	performanceAnalysis := s.analyzePerformancePatterns(allFiles, maxResults)
	puritySummary := s.buildPuritySummary()

	healthDashboard := &HealthDashboard{
		OverallScore: overallScore,
		Complexity:   complexityMetrics,
		TechnicalDebt: TechnicalDebtMetrics{
			Ratio:      debtRatio,
			Estimate:   s.estimateDebtRemediationTime(debtRatio),
			Components: s.identifyDebtComponents(allFiles),
		},
		Hotspots: hotspots,
		AnalysisMetadata: AnalysisMetadata{
			AnalyzedAt: time.Now(),
		},
		DetailedSmells:      detailedSmells,
		ProblematicSymbols:  problematicSymbols,
		SmellCounts:         smellCounts,
		PerformancePatterns: performanceAnalysis,
		MemoryAnalysis:      nil,
		PuritySummary:       puritySummary,
	}

	return healthDashboard, nil
}
