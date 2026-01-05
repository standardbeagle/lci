package mcp

import (
	"fmt"
	"strings"
)

// CompactFormatter implements semistructured LCF (LCI Compact Format) response formatting
type CompactFormatter struct {
	IncludeContext     bool
	IncludeMetadata    bool
	IncludeBreadcrumbs bool
}

// FormatSearchResponse converts SearchResponse to ultra-compact LCF format
func (f *CompactFormatter) FormatSearchResponse(response *SearchResponse) string {
	var lines []string

	// Header with pagination info
	lines = append(lines, "LCF/1.0")
	lines = append(lines, fmt.Sprintf("total=%d showing=%d max=%d", response.TotalMatches, response.Showing, response.MaxResults))

	// Results (one per line, compact format)
	for _, result := range response.Results {
		lines = append(lines, f.formatCompactSearchResult(result))
	}

	return strings.Join(lines, "\n")
}

// FormatFilesOnlyResponse formats files-only output - minimal token usage
// Format: one file path per line, no metadata
func (f *CompactFormatter) FormatFilesOnlyResponse(response *FilesOnlyResponse) string {
	var lines []string

	// Minimal header
	lines = append(lines, "LCF/1.0 mode=files")
	lines = append(lines, fmt.Sprintf("total=%d files=%d", response.TotalMatches, response.UniqueFiles))

	// Just file paths, one per line
	for _, file := range response.Files {
		lines = append(lines, file)
	}

	return strings.Join(lines, "\n")
}

// FormatCountOnlyResponse formats count-only output - absolute minimal
func (f *CompactFormatter) FormatCountOnlyResponse(response *CountOnlyResponse) string {
	return fmt.Sprintf("LCF/1.0 mode=count\ntotal=%d files=%d", response.TotalMatches, response.UniqueFiles)
}

// formatCompactSearchResult formats a single search result in ultra-compact format
func (f *CompactFormatter) formatCompactSearchResult(result CompactSearchResult) string {
	// Single-line ultra-compact format: file:line:col o=oid s=score [t=type] [n=name] [e] match
	dataParts := []string{
		fmt.Sprintf("%s:%d:%d", result.File, result.Line, result.Column),
		"o=" + result.ObjectID,
		fmt.Sprintf("s=%.0f", result.Score),
	}

	// Symbol info (optional)
	if result.SymbolType != "" {
		dataParts = append(dataParts, "t="+result.SymbolType)
	}
	if result.SymbolName != "" {
		dataParts = append(dataParts, "n="+result.SymbolName)
	}
	if result.IsExported {
		dataParts = append(dataParts, "e=1")
	}
	if result.FileMatchCount > 0 {
		dataParts = append(dataParts, fmt.Sprintf("m=%d", result.FileMatchCount))
	}

	line := strings.Join(dataParts, " ")

	// Add match text (truncated if very long to save tokens)
	match := result.Match
	if len(match) > 100 {
		match = match[:97] + "..."
	}
	line += " " + match

	// Context lines (only if IncludeContext and not too many)
	if f.IncludeContext && len(result.ContextLines) > 0 && len(result.ContextLines) <= 2 {
		for _, ctx := range result.ContextLines {
			line += "\n> " + ctx
		}
	}

	// Metadata (very compact)
	if f.IncludeMetadata {
		meta := f.formatMetadata(result)
		if meta != "" {
			line += "\n" + meta
		}
	}

	return line
}

// formatMetadata formats optional metadata
func (f *CompactFormatter) formatMetadata(result CompactSearchResult) string {
	var parts []string

	if f.IncludeBreadcrumbs && len(result.Breadcrumbs) > 0 {
		bcParts := make([]string, len(result.Breadcrumbs))
		for i, bc := range result.Breadcrumbs {
			bcParts[i] = bc.Name
		}
		parts = append(parts, "bc="+strings.Join(bcParts, "."))
	}

	if result.Safety != nil {
		parts = append(parts, "safety="+result.Safety.EditSafety)
		if result.Safety.ComplexityScore > 0 {
			parts = append(parts, fmt.Sprintf("complexity=%.2f", result.Safety.ComplexityScore))
		}
	}

	if result.References != nil {
		parts = append(parts, fmt.Sprintf("refs=%d,%d", result.References.IncomingCount, result.References.OutgoingCount))
	}

	if len(result.Dependencies) > 0 {
		parts = append(parts, fmt.Sprintf("deps=%d", len(result.Dependencies)))
	}

	if len(parts) == 0 {
		return ""
	}

	return "@" + strings.Join(parts, " ")
}

// FormatContextResponse converts ContextResponse to ultra-compact LCF format
func (f *CompactFormatter) FormatContextResponse(response *ContextResponse) string {
	var lines []string

	// Minimal header - just version and count
	lines = append(lines, "LCF/1.0")
	lines = append(lines, fmt.Sprintf("c=%d", response.Count))

	// Contexts (one per line, no separators to save tokens)
	for _, ctx := range response.Contexts {
		lines = append(lines, f.formatObjectContext(ctx))
	}

	return strings.Join(lines, "\n")
}

// formatObjectContext formats a single object context in ultra-compact format
func (f *CompactFormatter) formatObjectContext(ctx ObjectContext) string {
	var parts []string

	// Single-line ultra-compact format: path:line o=oid t=type n=name [e] [s=sig]
	parts = append(parts, fmt.Sprintf("%s:%d", ctx.FilePath, ctx.Line))

	// Core fields with single-letter codes
	dataParts := []string{
		"o=" + ctx.ObjectID,
		"t=" + ctx.SymbolType,
	}

	if ctx.SymbolName != "" {
		dataParts = append(dataParts, "n="+ctx.SymbolName)
	}
	if ctx.IsExported {
		dataParts = append(dataParts, "e=1")
	}
	if ctx.Signature != "" {
		dataParts = append(dataParts, "s="+ctx.Signature)
	}

	parts = append(parts, strings.Join(dataParts, " "))

	// Definition on same line if short, otherwise omit (redundant with name/sig)
	if ctx.Definition != "" && (len(ctx.Definition) < 40 || strings.Contains(ctx.Definition, ctx.SymbolName)) {
		parts[1] += " d=" + ctx.Definition
	}

	// Context lines (only if IncludeContext is true and not too verbose)
	if f.IncludeContext && len(ctx.Context) > 0 && len(ctx.Context) <= 2 {
		for _, c := range ctx.Context {
			parts = append(parts, "> "+c)
		}
	}

	return strings.Join(parts, "\n")
}

// FormatIntelligenceResponse converts CodebaseIntelligenceResponse to LCF format
// @lci:call-frequency[once-per-request]
func (f *CompactFormatter) FormatIntelligenceResponse(response *CodebaseIntelligenceResponse) string {
	// Pre-allocate lines slice with estimated capacity to avoid repeated reallocations
	// Typical response has ~50-100 lines depending on content
	lines := make([]string, 0, 128)

	// Header (always present)
	lines = append(lines, f.formatHeaderSection(response)...)

	// Optional sections - each returns empty slice if nil/empty
	lines = append(lines, f.formatRepositoryMapSection(response.RepositoryMap)...)
	lines = append(lines, f.formatDependencyGraphSection(response.DependencyGraph)...)
	lines = append(lines, f.formatHealthDashboardSection(response.HealthDashboard)...)
	lines = append(lines, f.formatEntryPointsSection(response.RepositoryMap)...)
	lines = append(lines, f.formatModuleAnalysisSection(response.ModuleAnalysis)...)
	lines = append(lines, f.formatTermClusterSection(response.TermClusterAnalysis)...)
	lines = append(lines, f.formatStructureAnalysisSection(response.StructureAnalysis)...)

	// Layer analysis (minimal - inline due to small size)
	if response.LayerAnalysis != nil {
		lines = append(lines, "== LAYERS ==")
		lines = append(lines, fmt.Sprintf("violations=%d", response.LayerAnalysis.ViolationCount))
		lines = append(lines, "---")
	}

	lines = append(lines, f.formatStatisticsSection(response.StatisticsReport)...)

	// Analysis metadata (conditional)
	if f.IncludeMetadata {
		lines = append(lines, "== METADATA ==")
		am := response.AnalysisMetadata
		lines = append(lines, fmt.Sprintf("time=%dms files=%d at=%s",
			am.AnalysisTimeMs, am.FilesAnalyzed, am.AnalyzedAt))
		lines = append(lines, "---")
	}

	return strings.Join(lines, "\n")
}

// formatHeaderSection formats the LCF header with version, mode, tier, and token estimate
func (f *CompactFormatter) formatHeaderSection(response *CodebaseIntelligenceResponse) []string {
	return []string{
		"LCF/1.0",
		"mode=" + response.AnalysisMode,
		fmt.Sprintf("tier=%d", response.Tier),
		fmt.Sprintf("tokens=%d", estimateLCFTokenCount(response)),
		"---",
	}
}

// formatRepositoryMapSection formats the module boundaries section
func (f *CompactFormatter) formatRepositoryMapSection(repoMap *RepositoryMap) []string {
	if repoMap == nil || len(repoMap.ModuleBoundaries) == 0 {
		return nil
	}

	lines := []string{"== REPOSITORY MAP =="}
	for _, module := range repoMap.ModuleBoundaries {
		lines = append(lines, fmt.Sprintf("module=%s files=%d", module.Name, module.FileCount))
		if f.IncludeMetadata {
			limit := 5
			if len(module.FileIDs) < limit {
				limit = len(module.FileIDs)
			}
			for _, fileID := range module.FileIDs[:limit] {
				lines = append(lines, "  - "+fileID)
			}
		}
	}
	lines = append(lines, "---")
	return lines
}

// formatDependencyGraphSection formats the dependency edges section
func (f *CompactFormatter) formatDependencyGraphSection(graph *DependencyGraph) []string {
	if graph == nil || len(graph.Edges) == 0 {
		return nil
	}

	lines := []string{"== DEPENDENCIES =="}
	lines = append(lines, fmt.Sprintf("total=%d", len(graph.Edges)))

	if f.IncludeMetadata {
		for _, edge := range graph.Edges {
			lines = append(lines, fmt.Sprintf("%s -> %s", edge.FromEntityID, edge.ToEntityID))
		}
	}
	lines = append(lines, "---")
	return lines
}

// formatHealthDashboardSection formats the health dashboard with all sub-sections
func (f *CompactFormatter) formatHealthDashboardSection(hd *HealthDashboard) []string {
	if hd == nil {
		return nil
	}

	lines := []string{"== HEALTH =="}
	lines = append(lines, fmt.Sprintf("score=%.2f", hd.OverallScore))
	lines = append(lines, fmt.Sprintf("complexity=%.2f", hd.Complexity.AverageCC))

	// Smell counts summary
	if smellLine := f.formatSmellCountsSummary(hd.SmellCounts); smellLine != "" {
		lines = append(lines, smellLine)
	}

	// Detailed code smells
	lines = append(lines, f.formatDetailedSmells(hd.DetailedSmells)...)

	// Problematic symbols
	lines = append(lines, f.formatProblematicSymbols(hd.ProblematicSymbols)...)

	// Memory pressure analysis
	lines = append(lines, f.formatMemoryPressureAnalysis(hd.MemoryAnalysis)...)

	// Purity summary
	lines = append(lines, f.formatPuritySummary(hd.PuritySummary)...)

	lines = append(lines, "---")
	return lines
}

// formatSmellCountsSummary formats the smell type counts as a single line
func (f *CompactFormatter) formatSmellCountsSummary(smellCounts map[string]int) string {
	if len(smellCounts) == 0 {
		return ""
	}

	var smellParts []string
	for smellType, count := range smellCounts {
		smellParts = append(smellParts, fmt.Sprintf("%s=%d", smellType, count))
	}
	return "smells: " + strings.Join(smellParts, " ")
}

// formatDetailedSmells formats the top detailed code smells
func (f *CompactFormatter) formatDetailedSmells(smells []CodeSmellEntry) []string {
	if len(smells) == 0 {
		return nil
	}

	lines := []string{"detailed_smells:"}
	for _, smell := range smells {
		lines = append(lines, fmt.Sprintf("  [%s] %s: %s (%s) oid=%s",
			smell.Severity, smell.Type, smell.Symbol, smell.Location, smell.ObjectID))
	}
	return lines
}

// formatProblematicSymbols formats high-risk symbols with tags
func (f *CompactFormatter) formatProblematicSymbols(symbols []ProblematicSymbol) []string {
	if len(symbols) == 0 {
		return nil
	}

	lines := []string{"problematic_symbols:"}
	for _, ps := range symbols {
		tagStr := ""
		if len(ps.Tags) > 0 {
			tagStr = " [" + strings.Join(ps.Tags, ",") + "]"
		}
		lines = append(lines, fmt.Sprintf("  %s (%s) risk=%d%s oid=%s",
			ps.Name, ps.Location, ps.RiskScore, tagStr, ps.ObjectID))
	}
	return lines
}

// formatMemoryPressureAnalysis formats memory pressure with PageRank scores and hotspots
func (f *CompactFormatter) formatMemoryPressureAnalysis(ma *MemoryPressureAnalysis) []string {
	if ma == nil || len(ma.Scores) == 0 {
		return nil
	}

	lines := []string{"memory_pressure:"}
	lines = append(lines, fmt.Sprintf("  summary: funcs=%d allocs=%d loop_allocs=%d critical=%d high=%d medium=%d low=%d",
		ma.Summary.TotalFunctions,
		ma.Summary.TotalAllocations,
		ma.Summary.LoopAllocCount,
		ma.Summary.CriticalCount,
		ma.Summary.HighCount,
		ma.Summary.MediumCount,
		ma.Summary.LowCount))

	// Top memory pressure functions (limit to 5)
	lines = append(lines, "  top_pressure:")
	limit := 5
	if len(ma.Scores) < limit {
		limit = len(ma.Scores)
	}
	for i := 0; i < limit; i++ {
		s := ma.Scores[i]
		lines = append(lines, fmt.Sprintf("    [%s] %s (%s) score=%.1f direct=%.1f propagated=%.1f loop=%.1f",
			s.Severity, s.Function, s.Location, s.TotalScore, s.DirectScore, s.PropagatedScore, s.LoopPressure))
	}

	// Hotspots with actionable suggestions
	if len(ma.Hotspots) > 0 {
		lines = append(lines, "  hotspots:")
		hotspotLimit := 3
		if len(ma.Hotspots) < hotspotLimit {
			hotspotLimit = len(ma.Hotspots)
		}
		for i := 0; i < hotspotLimit; i++ {
			h := ma.Hotspots[i]
			lines = append(lines, fmt.Sprintf("    %s (%s): %s", h.Function, h.Location, h.Reason))
			if h.Suggestion != "" {
				lines = append(lines, "      -> "+h.Suggestion)
			}
		}
	}

	return lines
}

// formatPuritySummary formats the purity/side-effect summary
func (f *CompactFormatter) formatPuritySummary(ps *PuritySummary) []string {
	if ps == nil {
		return nil
	}

	lines := []string{"purity:"}
	lines = append(lines, fmt.Sprintf("  total=%d pure=%d impure=%d ratio=%.2f",
		ps.TotalFunctions, ps.PureFunctions, ps.ImpureFunctions, ps.PurityRatio))
	if ps.WithIOEffects > 0 || ps.WithGlobalWrites > 0 || ps.WithParamWrites > 0 {
		lines = append(lines, fmt.Sprintf("  effects: io=%d global_writes=%d param_writes=%d throws=%d",
			ps.WithIOEffects, ps.WithGlobalWrites, ps.WithParamWrites, ps.WithThrows))
	}
	if ps.DetailedQuery != "" {
		lines = append(lines, "  query: "+ps.DetailedQuery)
	}
	return lines
}

// formatEntryPointsSection formats entry points from the repository map
func (f *CompactFormatter) formatEntryPointsSection(repoMap *RepositoryMap) []string {
	if repoMap == nil || len(repoMap.EntryPoints) == 0 {
		return nil
	}

	lines := []string{"== ENTRY POINTS =="}
	for _, ep := range repoMap.EntryPoints {
		lines = append(lines, fmt.Sprintf("%s: %s", ep.Type, ep.Name))
	}
	lines = append(lines, "---")
	return lines
}

// formatModuleAnalysisSection formats module analysis with metrics
func (f *CompactFormatter) formatModuleAnalysisSection(ma *ModuleAnalysis) []string {
	if ma == nil || len(ma.Modules) == 0 {
		return nil
	}

	lines := []string{"== MODULES =="}
	lines = append(lines, fmt.Sprintf("total=%d cohesion=%.2f coupling=%.2f",
		ma.Metrics.TotalModules, ma.Metrics.AverageCohesion, ma.Metrics.AverageCoupling))

	// Show top modules by file count (limit to 10)
	modules := ma.Modules
	if len(modules) > 10 {
		modules = modules[:10]
	}
	for _, mod := range modules {
		lines = append(lines, fmt.Sprintf("  %s: type=%s files=%d funcs=%d cohesion=%.2f",
			mod.Name, mod.Type, mod.FileCount, mod.FunctionCount, mod.CohesionScore))
	}
	if len(ma.Modules) > 10 {
		lines = append(lines, fmt.Sprintf("  ... and %d more modules", len(ma.Modules)-10))
	}
	lines = append(lines, "---")
	return lines
}

// formatTermClusterSection formats domain term clusters
func (f *CompactFormatter) formatTermClusterSection(tca *TermClusterAnalysis) []string {
	if tca == nil || len(tca.Clusters) == 0 {
		return nil
	}

	lines := []string{"== DOMAIN TERMS =="}
	lines = append(lines, fmt.Sprintf("clusters=%d vocabulary=%d",
		tca.Metrics.TotalClusters, len(tca.Vocabulary)))

	// Show top domain clusters (limit to 5)
	clusters := tca.Clusters
	if len(clusters) > 5 {
		clusters = clusters[:5]
	}
	for _, c := range clusters {
		// Limit terms shown per cluster
		terms := c.Terms
		if len(terms) > 5 {
			terms = terms[:5]
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", c.Domain, strings.Join(terms, ", ")))
	}

	// Show key terms (limit to 10)
	if len(tca.KeyTerms) > 0 {
		lines = append(lines, "key_terms:")
		keyTerms := tca.KeyTerms
		if len(keyTerms) > 10 {
			keyTerms = keyTerms[:10]
		}
		for _, kt := range keyTerms {
			lines = append(lines, fmt.Sprintf("  %s: freq=%d domain=%s", kt.Term, kt.Frequency, kt.Domain))
		}
	}
	lines = append(lines, "---")
	return lines
}

// formatStructureAnalysisSection formats project structure analysis
func (f *CompactFormatter) formatStructureAnalysisSection(sa *StructureAnalysis) []string {
	if sa == nil {
		return nil
	}

	lines := []string{"== STRUCTURE =="}

	// Summary
	lines = append(lines, fmt.Sprintf("dirs=%d files=%d symbols=%d depth=%d",
		sa.Summary.TotalDirectories, sa.Summary.TotalFiles,
		sa.Summary.TotalSymbols, sa.Summary.MaxDepth))

	// File type breakdown
	if len(sa.Summary.FileTypeBreakdown) > 0 {
		typeCount := len(sa.Summary.FileTypeBreakdown)
		if typeCount > 5 {
			typeCount = 5
		}
		typeParts := make([]string, 0, typeCount)
		for ext, count := range sa.Summary.FileTypeBreakdown {
			typeParts = append(typeParts, fmt.Sprintf("%s=%d", ext, count))
			if len(typeParts) >= 5 {
				break
			}
		}
		lines = append(lines, "types: "+strings.Join(typeParts, " "))
	}

	// File categories
	lines = append(lines, fmt.Sprintf("categories: code=%d tests=%d config=%d docs=%d",
		len(sa.FilesByCategory.Code), len(sa.FilesByCategory.Tests),
		len(sa.FilesByCategory.Config), len(sa.FilesByCategory.Docs)))

	// Top directories by size
	if len(sa.Summary.DirectorySizes) > 0 {
		lines = append(lines, "top_dirs:")
		dirSizes := sa.Summary.DirectorySizes
		if len(dirSizes) > 5 {
			dirSizes = dirSizes[:5]
		}
		for _, ds := range dirSizes {
			lines = append(lines, fmt.Sprintf("  %s: %d files", ds.Path, ds.FileCount))
		}
	}

	// Key symbols (high importance only, limit to 10)
	if len(sa.KeySymbols) > 0 {
		highCount := 0
		for _, sym := range sa.KeySymbols {
			if sym.Importance == "high" {
				highCount++
				if highCount >= 10 {
					break
				}
			}
		}
		if highCount > 0 {
			lines = append(lines, "key_symbols:")
			shown := 0
			for _, sym := range sa.KeySymbols {
				if sym.Importance == "high" {
					lines = append(lines, fmt.Sprintf("  %s (%s) in %s", sym.Name, sym.Type, sym.File))
					shown++
					if shown >= 10 {
						break
					}
				}
			}
		}
	}

	lines = append(lines, "---")
	return lines
}

// formatStatisticsSection formats the statistics report
func (f *CompactFormatter) formatStatisticsSection(sr *StatisticsReport) []string {
	if sr == nil {
		return nil
	}

	lines := []string{"== STATISTICS =="}

	// Complexity metrics
	lines = append(lines, fmt.Sprintf("complexity: avg=%.2f median=%.2f",
		sr.ComplexityMetrics.AverageCC, sr.ComplexityMetrics.MedianCC))

	// Show complexity distribution if available
	if len(sr.ComplexityMetrics.Distribution) > 0 {
		var distParts []string
		for typeName, count := range sr.ComplexityMetrics.Distribution {
			distParts = append(distParts, fmt.Sprintf("%s=%d", typeName, count))
		}
		lines = append(lines, "  distribution: "+strings.Join(distParts, " "))
	}

	// Coupling metrics
	lines = append(lines, fmt.Sprintf("coupling: avg=%.2f max=%.2f",
		sr.CouplingMetrics.AverageCoupling, sr.CouplingMetrics.MaxCoupling))

	// Cohesion metrics
	lines = append(lines, fmt.Sprintf("cohesion: avg=%.2f min=%.2f",
		sr.CohesionMetrics.AverageCohesion, sr.CohesionMetrics.MinCohesion))

	// Quality metrics
	lines = append(lines, fmt.Sprintf("quality: maintainability=%.2f debt=%.2f purity=%.2f",
		sr.QualityMetrics.MaintainabilityIndex, sr.QualityMetrics.TechnicalDebtRatio, sr.QualityMetrics.PurityRatio))

	// High complexity functions (limit to top 3)
	if len(sr.ComplexityMetrics.HighComplexityFuncs) > 0 {
		lines = append(lines, "  high_complexity:")
		limit := 3
		if len(sr.ComplexityMetrics.HighComplexityFuncs) < limit {
			limit = len(sr.ComplexityMetrics.HighComplexityFuncs)
		}
		for i := 0; i < limit; i++ {
			fn := sr.ComplexityMetrics.HighComplexityFuncs[i]
			lines = append(lines, fmt.Sprintf("    %s (%s) cc=%.1f", fn.Name, fn.Location, fn.Complexity))
		}
	}

	// Low cohesion modules (limit to top 3)
	if len(sr.CohesionMetrics.LowCohesionModules) > 0 {
		limit := 3
		if len(sr.CohesionMetrics.LowCohesionModules) < limit {
			limit = len(sr.CohesionMetrics.LowCohesionModules)
		}
		lines = append(lines, "  low_cohesion: "+strings.Join(sr.CohesionMetrics.LowCohesionModules[:limit], ", "))
	}

	// Circular dependencies
	if len(sr.CouplingMetrics.CircularDependencies) > 0 {
		lines = append(lines, fmt.Sprintf("  circular_deps: %d found", len(sr.CouplingMetrics.CircularDependencies)))
	}

	lines = append(lines, "---")
	return lines
}

// FormatAnnotationsResponse converts SemanticAnnotationsResponse to LCF format
func (f *CompactFormatter) FormatAnnotationsResponse(response *SemanticAnnotationsResponse) string {
	// Pre-allocate lines slice with estimated capacity
	lines := make([]string, 0, len(response.Annotations)*2+8)

	// Header
	lines = append(lines, "LCF/1.0")
	lines = append(lines, fmt.Sprintf("annotations=%d", len(response.Annotations)))
	lines = append(lines, "---")

	// Annotations
	for _, ann := range response.Annotations {
		lines = append(lines, fmt.Sprintf("%s:%d", ann.FilePath, ann.Line))

		dataParts := []string{
			"oid=" + ann.SymbolID,
			"name=" + ann.SymbolName,
		}

		if ann.Category != "" {
			dataParts = append(dataParts, "cat="+ann.Category)
		}

		lines = append(lines, strings.Join(dataParts, " "))

		if len(ann.DirectLabels) > 0 {
			lines = append(lines, "labels="+strings.Join(ann.DirectLabels, ","))
		}

		if len(ann.PropagatedLabels) > 0 {
			propLabels := make([]string, len(ann.PropagatedLabels))
			for i, pl := range ann.PropagatedLabels {
				propLabels[i] = pl.Label
			}
			lines = append(lines, "propagated="+strings.Join(propLabels, ","))
		}

		lines = append(lines, "---")
	}

	return strings.Join(lines, "\n")
}

// estimateLCFTokenCount provides an estimate of tokens for LCF format
func estimateLCFTokenCount(response *CodebaseIntelligenceResponse) int {
	estimate := 0

	if response.RepositoryMap != nil {
		estimate += len(response.RepositoryMap.ModuleBoundaries) * 20
	}

	if response.DependencyGraph != nil {
		estimate += len(response.DependencyGraph.Edges) * 15
	}

	if response.HealthDashboard != nil {
		estimate += 50
	}

	if response.RepositoryMap != nil && response.RepositoryMap.EntryPoints != nil {
		estimate += len(response.RepositoryMap.EntryPoints) * 15
	}

	if response.StatisticsReport != nil {
		estimate += 50
	}

	estimate += 20 // AnalysisMetadata

	return estimate
}
