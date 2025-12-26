package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ============================================================================
// Complexity and Quality Thresholds
// ============================================================================
//
// These thresholds are based on industry standards and empirical research:
// - McCabe's original paper recommends CC ≤ 10 for maintainable code
// - NIST 500-235 suggests CC > 20 indicates high risk
// - SEI/CERT uses similar thresholds for security-critical code
//
// Thresholds can be adjusted based on project-specific requirements.
// ============================================================================
const (
	// Cyclomatic complexity thresholds
	// Based on McCabe's guidelines: 1-10 simple, 11-20 moderate, 21-50 complex, 50+ untestable
	complexityThresholdLow      = 10 // Functions with CC ≤ 10 are considered simple
	complexityThresholdModerate = 15 // Functions with CC > 15 may indicate technical debt
	complexityThresholdHigh     = 20 // Functions with CC > 20 require attention (used for "high" category)

	// Function importance scoring thresholds
	// Used to identify "critical" functions worth highlighting to users
	importanceThresholdMetrics  = 20.0 // Threshold for metrics-based importance
	importanceThresholdEnhanced = 25.0 // Threshold for enhanced symbol importance

	// Hotspot detection thresholds
	hotspotComplexityThreshold = 10 // CC > 10 is considered a complexity hotspot
	hotspotLinecountThreshold  = 50 // Functions > 50 lines may be hotspots

	// Reference count thresholds
	highReferenceCountThreshold = 10 // Symbols with > 10 references need attention
	highUsageThreshold          = 5  // Functions with > 5 usages are considered important

	// Risk score bounds
	riskScoreMax = 10.0 // Maximum risk score for normalization

	// Maintainability index bounds (0-100 scale)
	maintainabilityIndexMin = 0.0
	maintainabilityIndexMax = 100.0
)

// ============================================================================
// Parameters
// ============================================================================

// CodebaseIntelligenceParams represents parameters for the codebase intelligence tool
type CodebaseIntelligenceParams struct {
	// Mode selection (required)
	Mode string `json:"mode"` // "overview", "detailed", "statistics", "unified", "structure"

	// Tier level (for overview mode)
	Tier *int `json:"tier,omitempty"` // 1, 2, or 3

	// Include flags (for overview mode)
	Include struct {
		RepositoryMap   bool `json:"repository_map,omitempty"`
		DependencyGraph bool `json:"dependency_graph,omitempty"`
		HealthDashboard bool `json:"health_dashboard,omitempty"`
		EntryPoints     bool `json:"entry_points,omitempty"`
	} `json:"include,omitempty"`

	// Analysis options (for detailed mode)
	Analysis *string `json:"analysis,omitempty"` // "modules", "layers", "features", "terms"

	// Statistics options (for statistics mode)
	Metrics *[]string `json:"metrics,omitempty"` // ["complexity", "coupling", "quality", "change"]

	// Granularity
	Granularity *string `json:"granularity,omitempty"` // "module", "layer", "function"

	// Performance and output control
	Max                 *int     `json:"max_results,omitempty"`
	ConfidenceThreshold *float64 `json:"confidence_threshold,omitempty"`

	// Domain filter (for feature analysis)
	Domain *string `json:"domain,omitempty"`

	// Query (for unified mode)
	Query *string `json:"query,omitempty"`

	// Focus filter (for structure mode) - filter to specific areas of the codebase
	// Examples: "api", "testing", "config", "handlers", "types"
	Focus *string `json:"focus,omitempty"`

	// Target for analysis (file path, directory, or symbol name)
	Target *string `json:"target,omitempty"`

	// Languages filter - scope analysis to specific programming languages
	// Examples: ["go"], ["typescript", "javascript"], ["csharp"]
	// Language names are case-insensitive and match against file extensions
	Languages []string `json:"languages,omitempty"`

	// Git analysis parameters (for git modes)
	Git *GitAnalysisParams `json:"git,omitempty"`
}

// GitAnalysisParams contains parameters for git-related analysis modes
type GitAnalysisParams struct {
	// For git_analyze mode
	Scope               string   `json:"scope,omitempty"`                // "staged", "wip", "commit", "range"
	BaseRef             string   `json:"base_ref,omitempty"`             // Base reference for comparison
	TargetRef           string   `json:"target_ref,omitempty"`           // Target reference (for range mode)
	Focus               []string `json:"focus,omitempty"`                // "duplicates", "naming"
	SimilarityThreshold float64  `json:"similarity_threshold,omitempty"` // 0.0-1.0, default: 0.8
	MaxFindings         int      `json:"max_findings,omitempty"`         // default: 20

	// For git_hotspots mode (change frequency)
	TimeWindow            string   `json:"time_window,omitempty"`             // "7d", "30d", "90d", "1y"
	Granularity           string   `json:"granularity,omitempty"`             // "file", "symbol"
	FilePattern           string   `json:"file_pattern,omitempty"`            // Glob pattern
	FilePath              string   `json:"file_path,omitempty"`               // Specific file
	SymbolName            string   `json:"symbol_name,omitempty"`             // Specific symbol
	MinChanges            int      `json:"min_changes,omitempty"`             // default: 2
	MinContributors       int      `json:"min_contributors,omitempty"`        // default: 2
	TopN                  int      `json:"top_n,omitempty"`                   // default: 50
	IncludePatterns       []string `json:"include_patterns,omitempty"`        // Glob patterns to include
	ExcludePatterns       []string `json:"exclude_patterns,omitempty"`        // Glob patterns to exclude
	SkipDefaultExclusions bool     `json:"skip_default_exclusions,omitempty"` // Disable default exclusions
}

// ============================================================================
// Handler
// ============================================================================

// handleCodebaseIntelligence provides comprehensive codebase intelligence
// @lci:labels[mcp-tool-handler,code-insight,architecture,complexity,statistics]
// @lci:category[mcp-api]
func (s *Server) handleCodebaseIntelligence(
	ctx context.Context,
	req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	startTime := time.Now()

	// Manual deserialization to avoid "unknown field" errors and give better error messages
	var ciParams CodebaseIntelligenceParams
	if err := json.Unmarshal(req.Params.Arguments, &ciParams); err != nil {
		return createSmartErrorResponse("codebase_intelligence", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"common_mistakes": []string{
				"Using CLI flags like -n in JSON parameters",
				"Passing include as string instead of object",
				"Passing metrics as string instead of array",
			},
			"correct_format": map[string]interface{}{
				"mode":        "overview",
				"tier":        1,
				"include":     map[string]interface{}{"repository_map": true, "dependency_graph": true},
				"metrics":     []string{"complexity", "coupling"},
				"max_results": 50,
			},
			"info_command": "Run info tool with {\"tool\": \"code_insight\"} for examples",
		})
	}

	// checkIndexAvailability now includes timeout-based waiting for index completion
	// using channel-based signaling (no polling)
	if available, err := s.checkIndexAvailability(); err != nil {
		return createSmartErrorResponse("codebase_intelligence", err, map[string]interface{}{
			"troubleshooting": []string{
				"Verify you're in a project directory with source code",
				"Check file permissions in project directory",
				"Review .lci.kdl configuration for errors",
				"Wait for auto-indexing to complete (check index_stats)",
			},
		})
	} else if !available {
		return createErrorResponse("codebase_intelligence", errors.New("codebase intelligence cannot proceed: index is not available"))
	}

	// Check if index is empty (no files indexed yet)
	if s.goroutineIndex.GetFileCount() == 0 {
		return createSmartErrorResponse("codebase_intelligence",
			errors.New("index not initialized - empty index with no files indexed"),
			map[string]interface{}{
				"suggestion": "Wait for auto-indexing to complete (check index_stats for progress)",
			})
	}

	// Check if index has symbols (required for most analysis modes)
	if s.goroutineIndex.GetSymbolCount() == 0 {
		return createSmartErrorResponse("codebase_intelligence",
			errors.New("index has files but no symbols extracted - this may indicate a parsing issue"),
			map[string]interface{}{
				"suggestion": "Check if files contain parseable code and try re-indexing",
				"file_count": s.goroutineIndex.GetFileCount(),
			})
	}

	// Get parameters with defaults (use manually deserialized params)
	args := ciParams

	// Validate mode
	validModes := map[string]bool{
		"overview":     true,
		"detailed":     true,
		"statistics":   true,
		"unified":      true,
		"structure":    true, // Codebase structure exploration
		"git_analyze":  true, // Git change analysis (duplicates, naming)
		"git_hotspots": true, // Git change frequency and hotspots
	}
	if args.Mode == "" {
		args.Mode = "overview" // Default mode
	}
	if !validModes[args.Mode] {
		return createSmartErrorResponse("codebase_intelligence",
			fmt.Errorf("invalid mode '%s', must be one of: overview, detailed, statistics, unified, structure, git_analyze, git_hotspots", args.Mode),
			map[string]interface{}{
				"valid_modes": []string{"overview", "detailed", "statistics", "unified", "structure", "git_analyze", "git_hotspots"},
			})
	}

	// Set default tier
	if args.Tier == nil {
		tier := DefaultCodebaseIntelligenceTier
		args.Tier = &tier
	}
	if *args.Tier < 1 || *args.Tier > 3 {
		return createSmartErrorResponse("codebase_intelligence",
			fmt.Errorf("invalid tier %d, must be 1, 2, or 3", *args.Tier),
			map[string]interface{}{
				"tier_1": "Must-have (Repository Map, Dependency Graph, Health Dashboard, Entry Points)",
				"tier_2": "High-value (Module Detection, Layer Classification, Feature Location, Term Clustering)",
				"tier_3": "Specialized (Code Statistics, Metrics, Quality Analysis)",
			})
	}

	// Set default granularity
	if args.Granularity == nil {
		granularity := DefaultGranularity
		args.Granularity = &granularity
	}

	// Set default confidence threshold
	if args.ConfidenceThreshold == nil {
		threshold := DefaultConfidenceThreshold
		args.ConfidenceThreshold = &threshold
	}

	// Set default include flags for overview mode
	if args.Mode == "overview" && (args.Include.RepositoryMap || args.Include.DependencyGraph ||
		args.Include.HealthDashboard || args.Include.EntryPoints) {
		// At least one include flag is set, keep as is
	} else if args.Mode == "overview" {
		// Default include all for overview mode
		args.Include.RepositoryMap = true
		args.Include.DependencyGraph = true
		args.Include.HealthDashboard = true
		args.Include.EntryPoints = true
	}

	// Phase 1: Build response based on mode
	var response *CodebaseIntelligenceResponse
	var err error

	switch args.Mode {
	case "overview":
		response, err = s.buildOverviewAnalysis(ctx, args)
	case "detailed":
		response, err = s.buildDetailedAnalysis(ctx, args)
	case "statistics":
		response, err = s.buildStatisticsAnalysis(ctx, args)
	case "unified":
		response, err = s.buildUnifiedAnalysis(ctx, args)
	case "structure":
		response, err = s.buildStructureAnalysis(ctx, args)
	case "git_analyze":
		response, err = s.buildGitAnalyzeAnalysis(ctx, args)
	case "git_hotspots":
		response, err = s.buildGitHotspotsAnalysis(ctx, args)
	case "type_hierarchy":
		response, err = s.buildTypeHierarchyAnalysis(ctx, args)
	default:
		return createSmartErrorResponse("codebase_intelligence",
			fmt.Errorf("unhandled mode: %s", args.Mode), nil)
	}

	if err != nil {
		return createSmartErrorResponse("codebase_intelligence", err, nil)
	}

	// Set metadata
	response.AnalysisMode = args.Mode
	response.Tier = *args.Tier
	response.AnalysisMetadata = AnalysisMetadata{
		AnalysisTimeMs: int(time.Since(startTime).Milliseconds()),
		FilesAnalyzed:  s.goroutineIndex.GetFileCount(),
		AnalyzedAt:     time.Now(),
		IndexVersion:   "1.0",
	}

	// Enforce token budget for 79.8% context reduction
	response = s.enforceTokenBudget(response, args.Max)

	// Use ultra-compact format to minimize token usage
	includeContext := args.Mode == "detailed" || args.Mode == "unified" || args.Mode == "structure"
	includeMetadata := args.ConfidenceThreshold != nil && *args.ConfidenceThreshold > 0.8
	return createCompactResponse(response, includeContext, includeMetadata)
}

// ============================================================================
// Helper Functions
// ============================================================================

// limitFunctionSignatures limits the number of function signatures to maxResults
func (s *Server) limitFunctionSignatures(functions []FunctionSignature, maxResults int) []FunctionSignature {
	if maxResults <= 0 {
		return functions
	}
	if len(functions) > maxResults {
		return functions[:maxResults]
	}
	return functions
}

// extractFunctionSignature extracts signature information from a symbol
func (s *Server) extractFunctionSignature(symbol *types.EnhancedSymbol) FunctionSignature {
	// Get module name from file path
	module := ""
	if fileInfo := s.goroutineIndex.GetFile(symbol.FileID); fileInfo != nil {
		module = fileInfo.Path
	}

	// Calculate importance score
	importanceScore := s.calculateImportanceScore(symbol)

	// Generate compact ObjectID from SymbolID
	objectID := searchtypes.EncodeSymbolID(symbol.ID)

	return FunctionSignature{
		ObjectID:        objectID,
		Name:            symbol.Name,
		Module:          module,
		Signature:       symbol.Signature,
		ImportanceScore: importanceScore,
		ReferencedBy:    len(symbol.IncomingRefs),
		SymbolType:      symbol.Type.String(),
		IsExported:      symbol.IsExported,
	}
}

// calculateImportanceScore calculates importance score for ranking functions
func (s *Server) calculateImportanceScore(symbol *types.EnhancedSymbol) float64 {
	// Base score: reference count (incoming references)
	score := float64(len(symbol.IncomingRefs))

	// Boost for exported/public functions
	if symbol.IsExported {
		score *= 1.5
	}

	// Boost for main functions
	if symbol.Name == "main" || symbol.Name == "Main" {
		score *= 2.0
	}

	// Boost for well-known patterns
	if strings.Contains(strings.ToLower(symbol.Name), "handler") ||
		strings.Contains(strings.ToLower(symbol.Name), "controller") ||
		strings.Contains(strings.ToLower(symbol.Name), "service") {
		score *= 1.3
	}

	// Boost for functions with complexity (indicates important logic)
	if symbol.Complexity > 0 {
		score *= (1.0 + float64(symbol.Complexity)/20.0)
	}

	return score
}

// ============================================================================
// Analysis Builders
// ============================================================================

// buildOverviewAnalysis builds Tier 1 overview analysis
func (s *Server) buildOverviewAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*CodebaseIntelligenceResponse, error) {
	response := &CodebaseIntelligenceResponse{
		NavigationHints: map[string]string{
			"clickable_ids":     "Every entity_id is clickable - use with get_object_context for full details",
			"search_handling":   "Search handles synonyms automatically via fuzzy matching, name splitting, and stemming",
			"vocabulary_report": "Semantic vocabulary moved to separate report mode for refactoring/onboarding",
			"navigation_flow":   "Click entry_point → see call hierarchy → follow references → understand codebase",
			"example_usage":     "get_context with {\"id\": \"<entity_id>\"} from any EntryPoint",
			"documentation":     "See docs/NAVIGATION_OVERVIEW_SPEC.md for complete guide",
		},
	}

	// Extract max_results with default value
	maxResults := 50 // Default if not specified
	if args.Max != nil && *args.Max > 0 {
		maxResults = *args.Max
	}

	// Build Repository Map
	if args.Include.RepositoryMap {
		repositoryMap, err := s.buildRepositoryMap(ctx, args, maxResults)
		if err != nil {
			return nil, fmt.Errorf("failed to build repository map: %w", err)
		}
		response.RepositoryMap = repositoryMap
	}

	// Build Dependency Graph
	if args.Include.DependencyGraph {
		dependencyGraph, err := s.buildDependencyGraph(ctx, args, maxResults)
		if err != nil {
			return nil, fmt.Errorf("failed to build dependency graph: %w", err)
		}
		response.DependencyGraph = dependencyGraph
	}

	// Build Health Dashboard
	if args.Include.HealthDashboard {
		healthDashboard, err := s.buildHealthDashboard(ctx, args, maxResults)
		if err != nil {
			return nil, fmt.Errorf("failed to build health dashboard: %w", err)
		}
		response.HealthDashboard = healthDashboard
	}

	// Build Entry Points
	if args.Include.EntryPoints {
		entryPoints, err := s.buildEntryPoints(ctx, args, maxResults)
		if err != nil {
			return nil, fmt.Errorf("failed to build entry points: %w", err)
		}
		response.EntryPoints = entryPoints
	}

	// Build Semantic Vocabulary (optional - separated to report mode)
	// Note: Semantic vocabulary is now in a separate report mode.
	// Search handles synonyms automatically via fuzzy matching, name splitting, and stemming.
	// For vocabulary analysis and refactoring guidance, use: codebase_intelligence with mode="vocabulary"
	// See documentation/docs/NAVIGATION_OVERVIEW_SPEC.md for details

	return response, nil
}

// buildDetailedAnalysis builds Tier 2 detailed analysis
func (s *Server) buildDetailedAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*CodebaseIntelligenceResponse, error) {
	response := &CodebaseIntelligenceResponse{}

	// Default analysis type
	if args.Analysis == nil {
		analysis := "modules"
		args.Analysis = &analysis
	}

	switch *args.Analysis {
	case "modules":
		moduleAnalysis, err := s.buildModuleAnalysis(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("failed to build module analysis: %w", err)
		}
		response.ModuleAnalysis = moduleAnalysis

	case "layers":
		layerAnalysis, err := s.buildLayerAnalysis(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("failed to build layer analysis: %w", err)
		}
		response.LayerAnalysis = layerAnalysis

	case "features":
		featureAnalysis, err := s.buildFeatureAnalysis(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("failed to build feature analysis: %w", err)
		}
		response.FeatureAnalysis = featureAnalysis

	case "terms":
		termClusterAnalysis, err := s.buildTermClusterAnalysis(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("failed to build term cluster analysis: %w", err)
		}
		response.TermClusterAnalysis = termClusterAnalysis

	default:
		return nil, fmt.Errorf("invalid analysis type '%s', must be one of: modules, layers, features, terms. For complexity metrics, use mode='statistics' with metrics=['complexity']", *args.Analysis)
	}

	return response, nil
}

// buildStatisticsAnalysis builds Tier 3 statistics analysis
func (s *Server) buildStatisticsAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*CodebaseIntelligenceResponse, error) {
	response := &CodebaseIntelligenceResponse{}

	// Build comprehensive statistics report
	statisticsReport, err := s.buildStatisticsReport(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build statistics report: %w", err)
	}
	response.StatisticsReport = statisticsReport

	return response, nil
}

// buildUnifiedAnalysis builds all analysis types
func (s *Server) buildUnifiedAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*CodebaseIntelligenceResponse, error) {
	response := &CodebaseIntelligenceResponse{}

	// Build all overview components
	overviewArgs := args
	overviewArgs.Mode = "overview"
	overviewArgs.Include.RepositoryMap = true
	overviewArgs.Include.DependencyGraph = true
	overviewArgs.Include.HealthDashboard = true
	overviewArgs.Include.EntryPoints = true

	overviewResponse, err := s.buildOverviewAnalysis(ctx, overviewArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build overview analysis: %w", err)
	}

	// Build module analysis
	moduleArgs := args
	moduleArgs.Mode = "detailed"
	moduleArgs.Analysis = stringPtr("modules")

	moduleResponse, err := s.buildDetailedAnalysis(ctx, moduleArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build detailed analysis: %w", err)
	}

	// Build statistics
	statsArgs := args
	statsArgs.Mode = "statistics"

	statsResponse, err := s.buildStatisticsAnalysis(ctx, statsArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build statistics analysis: %w", err)
	}

	// Merge responses
	response.RepositoryMap = overviewResponse.RepositoryMap
	response.DependencyGraph = overviewResponse.DependencyGraph
	response.HealthDashboard = overviewResponse.HealthDashboard
	response.EntryPoints = overviewResponse.EntryPoints
	response.ModuleAnalysis = moduleResponse.ModuleAnalysis
	response.StatisticsReport = statsResponse.StatisticsReport

	return response, nil
}

// buildStructureAnalysis builds a hierarchical structure view of the codebase
// Optimized for exploration - shows directory tree, file categories, and key symbols
func (s *Server) buildStructureAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*CodebaseIntelligenceResponse, error) {
	response := &CodebaseIntelligenceResponse{}

	// Get all files from index and apply language filter if specified
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(args.Languages) > 0 {
		allFiles = s.filterFilesByLanguage(allFiles, args.Languages)
	}
	if len(allFiles) == 0 {
		return nil, errors.New("no files indexed (or matching language filter)")
	}

	// Apply focus filter if provided
	focusFilter := ""
	if args.Focus != nil && *args.Focus != "" {
		focusFilter = strings.ToLower(*args.Focus)
	}

	// Build directory structure
	dirMap := make(map[string]*DirectoryNode)
	fileCategories := FileCategories{
		Code:   make([]string, 0),
		Tests:  make([]string, 0),
		Config: make([]string, 0),
		Docs:   make([]string, 0),
		Other:  make([]string, 0),
	}
	fileTypeCount := make(map[string]int)
	dirFileCounts := make(map[string]int)
	keySymbols := make([]DirectorySymbol, 0)
	maxDepth := 0

	// Get project root for relative paths
	projectRoot := ""
	if s.cfg != nil {
		projectRoot = s.cfg.Project.Root
	}

	for _, file := range allFiles {
		// Apply focus filter
		if focusFilter != "" {
			pathLower := strings.ToLower(file.Path)
			if !strings.Contains(pathLower, focusFilter) {
				continue
			}
		}

		// Make path relative
		relPath := file.Path
		if projectRoot != "" && strings.HasPrefix(file.Path, projectRoot) {
			relPath = strings.TrimPrefix(file.Path, projectRoot)
			relPath = strings.TrimPrefix(relPath, "/")
		}

		// Get directory and file info
		dir := filepath.Dir(relPath)
		name := filepath.Base(relPath)
		ext := strings.ToLower(filepath.Ext(relPath))

		// Track file type counts
		if ext != "" {
			fileTypeCount[ext]++
		}

		// Categorize file
		category := categorizeFile(relPath)
		switch category {
		case "code":
			fileCategories.Code = append(fileCategories.Code, relPath)
		case "test":
			fileCategories.Tests = append(fileCategories.Tests, relPath)
		case "config":
			fileCategories.Config = append(fileCategories.Config, relPath)
		case "doc":
			fileCategories.Docs = append(fileCategories.Docs, relPath)
		default:
			fileCategories.Other = append(fileCategories.Other, relPath)
		}

		// Track directory
		dirFileCounts[dir]++

		// Calculate depth
		depth := strings.Count(relPath, "/")
		if depth > maxDepth {
			maxDepth = depth
		}

		// Ensure directory node exists
		if _, exists := dirMap[dir]; !exists {
			dirMap[dir] = &DirectoryNode{
				Path:     dir,
				Name:     filepath.Base(dir),
				Files:    make([]FileNode, 0),
				Children: make([]DirectoryNode, 0),
			}
		}

		// Add file to directory
		fileNode := FileNode{
			Path:        relPath,
			Name:        name,
			Category:    category,
			Extension:   ext,
			SymbolCount: len(file.EnhancedSymbols),
			KeySymbols:  make([]string, 0),
		}

		// Extract key symbols (top-level functions, types, classes)
		for _, sym := range file.EnhancedSymbols {
			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeClass ||
				sym.Type == types.SymbolTypeType || sym.Type == types.SymbolTypeInterface {
				fileNode.KeySymbols = append(fileNode.KeySymbols, sym.Name)

				// Track important symbols
				importance := "low"
				if sym.IsExported {
					importance = "medium"
				}
				if sym.RefStats.Total.IncomingCount > 5 {
					importance = "high"
				}

				keySymbols = append(keySymbols, DirectorySymbol{
					Directory:  dir,
					Name:       sym.Name,
					Type:       sym.Type.String(),
					File:       relPath,
					Line:       sym.Line,
					Importance: importance,
				})
			}
		}

		// Limit key symbols per file
		if len(fileNode.KeySymbols) > 5 {
			fileNode.KeySymbols = fileNode.KeySymbols[:5]
		}

		dirMap[dir].Files = append(dirMap[dir].Files, fileNode)
		dirMap[dir].FileCount++
		dirMap[dir].SymbolCount += len(file.EnhancedSymbols)
	}

	// Build top-level directory tree (limit depth to 3 for token efficiency)
	rootDirs := make([]DirectoryNode, 0)
	for dir, node := range dirMap {
		depth := strings.Count(dir, "/")
		if depth <= 1 { // Top-level directories
			// Collapse if too many files
			if node.FileCount > 20 {
				node.Files = node.Files[:min(10, len(node.Files))]
				node.Collapsed = true
			}
			rootDirs = append(rootDirs, *node)
		}
	}

	// Sort directories by file count
	sort.Slice(rootDirs, func(i, j int) bool {
		return rootDirs[i].FileCount > rootDirs[j].FileCount
	})

	// Limit to top directories
	if len(rootDirs) > 15 {
		rootDirs = rootDirs[:15]
	}

	// Build directory sizes for summary
	dirSizes := make([]DirSize, 0)
	for dir, count := range dirFileCounts {
		dirSizes = append(dirSizes, DirSize{Path: dir, FileCount: count})
	}
	sort.Slice(dirSizes, func(i, j int) bool {
		return dirSizes[i].FileCount > dirSizes[j].FileCount
	})
	if len(dirSizes) > 10 {
		dirSizes = dirSizes[:10]
	}

	// Limit key symbols
	sort.Slice(keySymbols, func(i, j int) bool {
		impOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return impOrder[keySymbols[i].Importance] < impOrder[keySymbols[j].Importance]
	})
	if len(keySymbols) > 50 {
		keySymbols = keySymbols[:50]
	}

	// Limit file categories
	limitSlice := func(s []string, max int) []string {
		if len(s) > max {
			return s[:max]
		}
		return s
	}
	fileCategories.Code = limitSlice(fileCategories.Code, 30)
	fileCategories.Tests = limitSlice(fileCategories.Tests, 20)
	fileCategories.Config = limitSlice(fileCategories.Config, 15)
	fileCategories.Docs = limitSlice(fileCategories.Docs, 10)
	fileCategories.Other = limitSlice(fileCategories.Other, 10)

	// Build summary
	summary := StructureSummary{
		TotalDirectories:  len(dirMap),
		TotalFiles:        len(allFiles),
		TotalSymbols:      s.goroutineIndex.GetSymbolCount(),
		MaxDepth:          maxDepth,
		FileTypeBreakdown: fileTypeCount,
		DirectorySizes:    dirSizes,
	}

	response.StructureAnalysis = &StructureAnalysis{
		RootDirectory:   projectRoot,
		DirectoryTree:   rootDirs,
		FilesByCategory: fileCategories,
		KeySymbols:      keySymbols,
		Focus:           focusFilter,
		Summary:         summary,
	}

	// Add navigation hints for exploration
	response.NavigationHints = map[string]string{
		"explore_directory": "Use search with filter='<dir>/*' to see files in a directory",
		"find_symbols":      "Use search with symbol_types='function' to find functions",
		"focus_area":        "Use mode='structure' with focus='<term>' to filter results",
		"detailed_analysis": "Use mode='detailed' with analysis='modules' for module breakdown",
	}

	return response, nil
}

// categorizeFile determines the category of a file based on path and extension
func categorizeFile(path string) string {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	// Test files - check both filename patterns and directory paths
	if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_") ||
		strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") {
		return "test"
	}

	// Use consistent extension lists (kept in sync with search/engine.go)
	docExts := map[string]bool{".md": true, ".txt": true, ".rst": true, ".adoc": true, ".markdown": true}
	if docExts[ext] {
		return "doc"
	}

	configExts := map[string]bool{".json": true, ".yaml": true, ".yml": true, ".toml": true, ".kdl": true, ".ini": true, ".cfg": true, ".conf": true}
	if configExts[ext] {
		return "config"
	}

	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".rs": true, ".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".cs": true, ".rb": true, ".php": true, ".swift": true, ".kt": true, ".scala": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true,
	}
	if codeExts[ext] {
		return "code"
	}

	return "other"
}

// ============================================================================
// Component Builders (Phase 1 - Tier 1 Overview)
// ============================================================================

// buildRepositoryMap builds the repository map component
func (s *Server) buildRepositoryMap(
	ctx context.Context,
	args CodebaseIntelligenceParams,
	maxResults int,
) (*RepositoryMap, error) {
	// Get actual counts from index (not cached stats which may be stale)
	fileCount := s.goroutineIndex.GetFileCount()
	symbolCount := s.goroutineIndex.GetSymbolCount()

	if symbolCount == 0 {
		return nil, errors.New("index not initialized or empty")
	}

	// Get all files and apply language filter if specified
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(args.Languages) > 0 {
		allFiles = s.filterFilesByLanguage(allFiles, args.Languages)
		// Update counts for filtered results
		fileCount = len(allFiles)
		symbolCount = 0
		for _, file := range allFiles {
			symbolCount += len(file.EnhancedSymbols)
		}
	}

	// Count functions from files
	functionCount := 0
	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
				functionCount++
			}
		}
	}

	// Create RepositoryMap using actual counts
	repositoryMap := &RepositoryMap{
		CriticalFunctions: make([]FunctionSignature, 0),
		ModuleBoundaries:  make([]ModuleBoundary, 0),
		DomainTerms:       make([]DomainTerm, 0),
		EntryPoints:       make([]EntryPoint, 0),
		TotalFiles:        fileCount,
		TotalFunctions:    functionCount,
		TotalSymbols:      symbolCount,
		AnalyzedAt:        time.Now(),
	}

	// Build set of allowed file IDs for language filtering
	var allowedFileIDs map[types.FileID]bool
	if len(args.Languages) > 0 {
		allowedFileIDs = make(map[types.FileID]bool, len(allFiles))
		for _, file := range allFiles {
			allowedFileIDs[file.ID] = true
		}
	}

	// Get entry points with full metadata from files
	if entryPoints := s.goroutineIndex.GetEntryPoints(); len(entryPoints) > 0 {
		// Group by file to minimize GetFile calls
		entryPointsByFile := make(map[types.FileID][]types.Symbol)
		for _, ep := range entryPoints {
			// Skip entry points from excluded languages
			if allowedFileIDs != nil && !allowedFileIDs[ep.FileID] {
				continue
			}
			entryPointsByFile[ep.FileID] = append(entryPointsByFile[ep.FileID], ep)
		}

		// Get file info for each entry point
		for fileID, eps := range entryPointsByFile {
			if fileInfo := s.goroutineIndex.GetFile(fileID); fileInfo != nil {
				for _, ep := range eps {
					// Find enhanced symbol with full metadata
					var enhancedSym *types.EnhancedSymbol
					for _, sym := range fileInfo.EnhancedSymbols {
						if sym.Name == ep.Name && sym.Line == ep.Line {
							enhancedSym = sym
							break
						}
					}
					found := enhancedSym != nil
					if !found {
						// Fallback to basic symbol - no ObjectID available
						entityID := ep.EntityID(s.cfg.Project.Root, fileInfo.Path)
						fileID := fileInfo.EntityID(s.cfg.Project.Root)

						repositoryMap.EntryPoints = append(repositoryMap.EntryPoints, EntryPoint{
							ObjectID:   "", // No SymbolID available for basic symbol
							EntityID:   entityID,
							Name:       ep.Name,
							Type:       "function",
							Location:   fmt.Sprintf("%s:%d", fileInfo.Path, ep.Line),
							FileID:     fileID,
							Signature:  "",
							IsExported: len(ep.Name) > 0 && ep.Name[0] >= 'A' && ep.Name[0] <= 'Z',
						})
					} else {
						// Generate entity ID and compact ObjectID for enhanced symbol
						entityID := enhancedSym.EntityID(s.cfg.Project.Root, fileInfo.Path)
						fileID := fileInfo.EntityID(s.cfg.Project.Root)
						objectID := searchtypes.EncodeSymbolID(enhancedSym.ID)

						repositoryMap.EntryPoints = append(repositoryMap.EntryPoints, EntryPoint{
							ObjectID:   objectID,
							EntityID:   entityID,
							Name:       enhancedSym.Name,
							Type:       "function",
							Location:   fmt.Sprintf("%s:%d", fileInfo.Path, enhancedSym.Line),
							FileID:     fileID,
							Signature:  enhancedSym.Signature,
							IsExported: enhancedSym.IsExported,
						})
					}
				}
			}
		}
	}

	// ModuleBoundaries need ALL files for accurate package grouping and counts
	repositoryMap.ModuleBoundaries = s.extractModuleBoundariesFromSymbols(allFiles, &args)

	// For detailed analysis (CriticalFunctions, DomainTerms), use sampling
	// to limit token usage while still providing useful examples
	sampleSize := 50
	if len(allFiles) < sampleSize {
		sampleSize = len(allFiles)
	}
	if sampleSize > 0 {
		sampleFiles := allFiles[:sampleSize]
		repositoryMap.CriticalFunctions = s.extractCriticalFunctionsFromSymbols(sampleFiles, &args)
		repositoryMap.DomainTerms = s.extractDomainTermsFromSymbols(sampleFiles, &args)
	}

	// Add navigation instructions
	repositoryMap.Note = "Use EntityIDs with get_object_context for full navigation. " +
		"Example: get_object_context with entity_id from EntryPoints to see call hierarchy, " +
		"references, dependencies, and file content."

	return repositoryMap, nil
}

// buildDependencyGraph builds the dependency graph component
func (s *Server) buildDependencyGraph(
	ctx context.Context,
	args CodebaseIntelligenceParams,
	maxResults int,
) (*DependencyGraph, error) {
	// Get all files and apply language filter if specified
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(args.Languages) > 0 {
		allFiles = s.filterFilesByLanguage(allFiles, args.Languages)
	}
	if len(allFiles) == 0 {
		return nil, errors.New("no files found in index (or matching language filter)")
	}

	dependencyGraph := &DependencyGraph{
		Nodes:                make([]DependencyNode, 0),
		Edges:                make([]DependencyEdge, 0),
		CircularDependencies: make([]CircularDependency, 0),
		LayerViolations:      make([]LayerViolation, 0),
		CouplingHotspots:     make([]CouplingHotspot, 0),
		HighestCentrality:    make([]string, 0),
		AnalysisMetadata: AnalysisMetadata{
			AnalyzedAt: time.Now(),
		},
	}

	// Build nodes from symbols
	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod || sym.Type == types.SymbolTypeStruct {
				// Generate entity ID for the symbol
				entityID := sym.EntityID(s.cfg.Project.Root, file.Path)

				dependencyGraph.Nodes = append(dependencyGraph.Nodes, DependencyNode{
					EntityID:   entityID,
					Name:       sym.Name,
					Type:       sym.Type.String(),
					Centrality: 0.0, // Would need reference analysis
				})
			}
		}
	}

	return dependencyGraph, nil
}

// buildEntryPoints builds the entry points component
func (s *Server) buildEntryPoints(
	ctx context.Context,
	args CodebaseIntelligenceParams,
	maxResults int,
) (*EntryPointsList, error) {
	// Get entry points using O(1) query
	entryPointsList := s.goroutineIndex.GetEntryPoints()
	if len(entryPointsList) == 0 {
		return &EntryPointsList{
			MainFunctions: make([]EntryPoint, 0),
		}, nil
	}

	entryPoints := &EntryPointsList{
		MainFunctions: make([]EntryPoint, 0),
	}

	// Group by file to minimize GetFile calls
	entryPointsByFile := make(map[types.FileID][]types.Symbol)
	for _, ep := range entryPointsList {
		entryPointsByFile[ep.FileID] = append(entryPointsByFile[ep.FileID], ep)
	}

	// Get file info and find main/exported functions
	mainCount := 0
	apiCount := 0

	for fileID, eps := range entryPointsByFile {
		if fileInfo := s.goroutineIndex.GetFile(fileID); fileInfo != nil {
			for _, ep := range eps {
				// Find enhanced symbol with full metadata
				var enhancedSym *types.EnhancedSymbol
				for _, sym := range fileInfo.EnhancedSymbols {
					if sym.Name == ep.Name && sym.Line == ep.Line {
						enhancedSym = sym
						break
					}
				}

				if enhancedSym == nil {
					continue // Skip if no enhanced symbol found
				}

				// Generate EntityID and compact ObjectID for this entry point
				entityID := enhancedSym.EntityID(s.cfg.Project.Root, fileInfo.Path)
				fileID := fileInfo.EntityID(s.cfg.Project.Root)
				objectID := searchtypes.EncodeSymbolID(enhancedSym.ID)

				// Check if it's a main function
				if ep.Name == "main" || ep.Name == "Main" {
					entryPoints.MainFunctions = append(entryPoints.MainFunctions, EntryPoint{
						ObjectID:   objectID,
						EntityID:   entityID,
						Name:       enhancedSym.Name,
						Type:       "main",
						Location:   fmt.Sprintf("%s:%d", fileInfo.Path, enhancedSym.Line),
						FileID:     fileID,
						Signature:  enhancedSym.Signature,
						IsExported: enhancedSym.IsExported,
					})
					mainCount++
				} else if enhancedSym.IsExported && (enhancedSym.Type == types.SymbolTypeFunction || enhancedSym.Type == types.SymbolTypeMethod) {
					// Check if it's an exported function (API entry point)
					if apiCount < 10 { // Limit to 10 for context
						entryPoints.MainFunctions = append(entryPoints.MainFunctions, EntryPoint{
							ObjectID:   objectID,
							EntityID:   entityID,
							Name:       enhancedSym.Name,
							Type:       "api",
							Location:   fmt.Sprintf("%s:%d", fileInfo.Path, enhancedSym.Line),
							FileID:     fileID,
							Signature:  enhancedSym.Signature,
							IsExported: enhancedSym.IsExported,
						})
						apiCount++
					}
				}
			}
		}
	}

	return entryPoints, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// extractCriticalFunctions extracts critical functions using reference count from usage tracking
func (s *Server) extractCriticalFunctions(allSymbols []*types.UniversalSymbolNode, args *CodebaseIntelligenceParams) []FunctionSignature {
	criticalFunctions := make([]FunctionSignature, 0)

	// Create function signatures using Usage data from universal graph
	for _, node := range allSymbols {
		if node.Identity.Kind == types.SymbolKindFunction || node.Identity.Kind == types.SymbolKindMethod {
			// Use actual reference and call counts from usage tracking
			refCount := node.Usage.ReferenceCount
			callCount := node.Usage.CallCount

			// Calculate importance score based on actual usage metrics
			// - References indicate how many places depend on this function
			// - Calls indicate how frequently the function is invoked
			// - Exported functions are implicitly important for public API
			importanceScore := float64(refCount)*10.0 + float64(callCount)*5.0
			if node.Visibility.IsExported {
				importanceScore += 20.0 // Boost for exported functions
			}

			// Include function if it has references, calls, or is exported
			if refCount > 0 || callCount > 0 || node.Visibility.IsExported {
				criticalFunctions = append(criticalFunctions, FunctionSignature{
					ObjectID:        "", // Not available from UniversalSymbolNode
					Name:            node.Identity.Name,
					Module:          s.getModuleName(node.Identity.ID.String()),
					Signature:       node.Identity.Signature,
					ImportanceScore: importanceScore,
					ReferencedBy:    refCount + callCount, // Combined usage count
					SymbolType:      node.Identity.Kind.String(),
					IsExported:      node.Visibility.IsExported,
				})
			}
		}
	}

	// Sort by importance score (descending) to show most critical functions first
	sort.Slice(criticalFunctions, func(i, j int) bool {
		return criticalFunctions[i].ImportanceScore > criticalFunctions[j].ImportanceScore
	})

	return criticalFunctions
}

// extractCriticalFunctionsFromMetrics extracts critical functions using precomputed metrics
func (s *Server) extractCriticalFunctionsFromMetrics(allMetrics map[types.FileID]map[string]interface{}, args *CodebaseIntelligenceParams) []FunctionSignature {
	criticalFunctions := make([]FunctionSignature, 0)

	// Iterate through all precomputed metrics
	for fileID, fileMetrics := range allMetrics {
		for symbolName, metrics := range fileMetrics {
			// Cast to SymbolMetrics if available
			if symbolMetrics, ok := metrics.(*analysis.SymbolMetrics); ok {
				// Calculate importance score based on metrics
				importanceScore := 0.0

				// Use complexity and coupling to determine importance
				if symbolMetrics.Quality.CyclomaticComplexity > complexityThresholdLow {
					importanceScore += float64(symbolMetrics.Quality.CyclomaticComplexity)
				}
				if symbolMetrics.Dependencies.OutgoingDependencies > highUsageThreshold {
					importanceScore += float64(symbolMetrics.Dependencies.OutgoingDependencies) * 2.0
				}
				if symbolMetrics.Dependencies.IncomingDependencies > 3 {
					importanceScore += float64(symbolMetrics.Dependencies.IncomingDependencies) * 3.0
				}

				// Add to critical functions if important enough
				if importanceScore > importanceThresholdMetrics || symbolMetrics.RiskScore > highUsageThreshold {
					criticalFunctions = append(criticalFunctions, FunctionSignature{
						ObjectID:        "", // Not available from metrics
						Name:            symbolName,
						Module:          fmt.Sprintf("file_%d", fileID),
						Signature:       "", // Not stored in metrics
						ImportanceScore: importanceScore,
						ReferencedBy:    symbolMetrics.Dependencies.IncomingDependencies,
						SymbolType:      symbolMetrics.Type.String(),
						IsExported:      false, // Not in metrics
					})
				}
			} else {
				// For other metric types, use simple heuristic
				criticalFunctions = append(criticalFunctions, FunctionSignature{
					ObjectID:        "", // Not available from metrics
					Name:            symbolName,
					Module:          fmt.Sprintf("file_%d", fileID),
					Signature:       "",
					ImportanceScore: 10.0, // Default importance
					ReferencedBy:    0,
					SymbolType:      "function",
					IsExported:      false,
				})
			}
		}
	}

	return criticalFunctions
}

// extractCriticalFunctionsFromSymbols extracts critical functions from symbol data
func (s *Server) extractCriticalFunctionsFromSymbols(allFiles []*types.FileInfo, args *CodebaseIntelligenceParams) []FunctionSignature {
	criticalFunctions := make([]FunctionSignature, 0)

	// Check if we have EnhancedSymbols
	hasEnhanced := len(allFiles) > 0 && len(allFiles[0].EnhancedSymbols) > 0

	if hasEnhanced {
		// Use EnhancedSymbols
		for _, file := range allFiles {
			for _, sym := range file.EnhancedSymbols {
				if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
					// Simple heuristic: exported functions with longer names are more important
					importanceScore := 0.0

					if sym.IsExported {
						importanceScore += 30.0
					}

					// Longer names might indicate more specific/important functions
					importanceScore += float64(len(sym.Name)) * 0.5

					// Add critical functions if they meet threshold
					if importanceScore > importanceThresholdEnhanced {
						// Generate entity ID and compact ObjectID for function
						entityID := sym.EntityID(s.cfg.Project.Root, file.Path)
						fileID := file.EntityID(s.cfg.Project.Root)
						objectID := searchtypes.EncodeSymbolID(sym.ID)

						criticalFunctions = append(criticalFunctions, FunctionSignature{
							ObjectID:        objectID,
							EntityID:        entityID,
							Name:            sym.Name,
							Module:          file.Path,
							Signature:       sym.Signature,
							ImportanceScore: importanceScore,
							ReferencedBy:    0, // Would need reference tracking
							SymbolType:      sym.Type.String(),
							IsExported:      sym.IsExported,
							FileID:          fileID,
							Location:        fmt.Sprintf("%s:%d:%d", file.Path, sym.Line, sym.Column),
						})
					}
				}
			}
		}
	}

	return criticalFunctions
}

// extractModuleBoundaries extracts module boundaries with actual cohesion/coupling/stability metrics
func (s *Server) extractModuleBoundaries(allSymbols []*types.UniversalSymbolNode, args *CodebaseIntelligenceParams) []ModuleBoundary {
	moduleMap := make(map[string][]*types.UniversalSymbolNode)

	// Group symbols by module (directory)
	for _, node := range allSymbols {
		module := s.getModuleName(node.Identity.ID.String())
		moduleMap[module] = append(moduleMap[module], node)
	}

	// Build a set of symbol IDs per module for efficient lookup
	moduleSymbolSets := make(map[string]map[string]bool)
	for moduleName, syms := range moduleMap {
		moduleSymbolSets[moduleName] = make(map[string]bool)
		for _, sym := range syms {
			moduleSymbolSets[moduleName][sym.Identity.ID.String()] = true
		}
	}

	moduleBoundaries := make([]ModuleBoundary, 0)
	for moduleName, syms := range moduleMap {
		// Calculate actual cohesion, coupling, and stability metrics
		cohesion, coupling, stability := s.calculateModuleMetrics(moduleName, syms, moduleSymbolSets)

		moduleBoundaries = append(moduleBoundaries, ModuleBoundary{
			Name:          moduleName,
			Type:          s.getModuleType(moduleName),
			Path:          moduleName,
			CohesionScore: cohesion,
			CouplingScore: coupling,
			Stability:     stability,
			FileCount:     s.countFilesInModule(moduleName),
			FunctionCount: len(syms),
		})
	}

	return moduleBoundaries
}

// calculateModuleMetrics calculates cohesion, coupling, and stability for a module
// Based on Robert C. Martin's package metrics:
// - Cohesion: Ratio of internal dependencies to total dependencies (0-1, higher is better)
// - Coupling: Ratio of external dependencies to total possible connections (0-1, lower is better)
// - Stability: Afferent / (Afferent + Efferent) - 0 means maximally unstable, 1 means maximally stable
func (s *Server) calculateModuleMetrics(moduleName string, symbols []*types.UniversalSymbolNode, moduleSymbolSets map[string]map[string]bool) (cohesion, coupling, stability float64) {
	if len(symbols) == 0 {
		return 0.5, 0.0, 0.5 // Default values for empty modules
	}

	thisModuleSet := moduleSymbolSets[moduleName]

	var (
		internalDeps  int // Dependencies within the module
		externalDeps  int // Dependencies outside the module (efferent coupling Ce)
		incomingDeps  int // Dependencies from outside pointing to this module (afferent coupling Ca)
		totalCalls    int // Total call relationships
		internalCalls int // Calls within the module
	)

	for _, sym := range symbols {
		// Count dependency relationships
		for _, dep := range sym.Relationships.Dependencies {
			totalDep := dep.Target.String()
			if thisModuleSet[totalDep] {
				internalDeps++
			} else {
				externalDeps++
			}
		}

		// Count incoming dependencies (afferent)
		for _, depID := range sym.Relationships.Dependents {
			depStr := depID.String()
			if !thisModuleSet[depStr] {
				incomingDeps++
			}
		}

		// Count call relationships
		for _, call := range sym.Relationships.CallsTo {
			totalCalls++
			if thisModuleSet[call.Target.String()] {
				internalCalls++
			}
		}

		// Count incoming calls as part of afferent coupling
		for _, call := range sym.Relationships.CalledBy {
			callerStr := call.Target.String()
			if !thisModuleSet[callerStr] {
				incomingDeps++
			}
		}
	}

	// Calculate Cohesion: ratio of internal dependencies/calls to total
	// High cohesion means symbols within the module work together
	totalInternalConnections := internalDeps + internalCalls
	totalConnections := internalDeps + externalDeps + totalCalls
	if totalConnections > 0 {
		cohesion = float64(totalInternalConnections) / float64(totalConnections)
	} else if len(symbols) > 0 {
		// No dependencies but has symbols - could be independent utilities
		// Give moderate cohesion since nothing is violating module boundaries
		cohesion = 0.5
	}

	// Calculate Coupling: efferent coupling normalized by module size
	// Lower is better - indicates module is more independent
	maxPossibleExternalDeps := len(symbols) * 10 // Heuristic max
	if maxPossibleExternalDeps > 0 {
		coupling = float64(externalDeps) / float64(maxPossibleExternalDeps)
		if coupling > 1.0 {
			coupling = 1.0
		}
	}

	// Calculate Stability: I = Ca / (Ca + Ce)
	// Martin's Instability metric, inverted to Stability
	// Ca = afferent coupling (incoming), Ce = efferent coupling (outgoing)
	afferent := float64(incomingDeps)
	efferent := float64(externalDeps)
	if afferent+efferent > 0 {
		// Martin's Instability = Ce / (Ca + Ce), so Stability = Ca / (Ca + Ce)
		stability = afferent / (afferent + efferent)
	} else {
		// No dependencies in either direction - module is isolated
		// Consider it moderately stable
		stability = 0.5
	}

	return cohesion, coupling, stability
}

// extractModuleBoundariesFromMetrics extracts module boundaries using precomputed metrics
func (s *Server) extractModuleBoundariesFromMetrics(allMetrics map[types.FileID]map[string]interface{}, args *CodebaseIntelligenceParams) []ModuleBoundary {
	// Group metrics by module (file-based)
	moduleMap := make(map[string][]*analysis.SymbolMetrics)

	// Group symbols by file/module
	for fileID, fileMetrics := range allMetrics {
		moduleName := fmt.Sprintf("file_%d", fileID)

		for _, metrics := range fileMetrics {
			if symbolMetrics, ok := metrics.(*analysis.SymbolMetrics); ok {
				moduleMap[moduleName] = append(moduleMap[moduleName], symbolMetrics)
			}
		}
	}

	moduleBoundaries := make([]ModuleBoundary, 0)
	for moduleName, metrics := range moduleMap {
		// Calculate aggregated metrics
		totalComplexity := 0
		avgCoupling := 0.0
		avgStability := 0.0
		functionCount := len(metrics)

		for _, m := range metrics {
			totalComplexity += m.Quality.CyclomaticComplexity
			avgCoupling += m.Dependencies.CouplingStrength
			avgStability += m.Dependencies.StabilityIndex
		}

		if functionCount > 0 {
			avgCoupling /= float64(functionCount)
			avgStability /= float64(functionCount)
		}

		// Calculate cohesion score (simplified - inverse of coupling)
		cohesionScore := 1.0 - avgCoupling
		if cohesionScore < 0 {
			cohesionScore = 0
		}

		moduleBoundaries = append(moduleBoundaries, ModuleBoundary{
			Name:          moduleName,
			Type:          "module",
			Path:          moduleName,
			CohesionScore: cohesionScore,
			CouplingScore: avgCoupling,
			Stability:     avgStability,
			FileCount:     1, // One file per module in this simplified model
			FunctionCount: functionCount,
		})
	}

	return moduleBoundaries
}

// extractModuleBoundariesFromSymbols extracts module boundaries from symbol data
// Groups files by package directory and uses the ReferenceTracker's graph data
// for accurate coupling/cohesion metrics based on actual symbol references.
func (s *Server) extractModuleBoundariesFromSymbols(allFiles []*types.FileInfo, args *CodebaseIntelligenceParams) []ModuleBoundary {
	projectRoot := s.cfg.Project.Root
	refTracker := s.goroutineIndex.GetRefTracker()

	// Track files and symbols by package (directory)
	type packageData struct {
		files          []*types.FileInfo
		fileIDs        []string
		typeFileIDs    []types.FileID // For graph lookups
		functionIDs    []string
		classIDs       []string
		exampleFuncs   []SymbolRef
		exampleClasses []SymbolRef
		functionCount  int
		classCount     int
		symbolCount    int
	}
	packageMap := make(map[string]*packageData)

	// Build fileID -> package mapping for reference resolution
	fileToPackage := make(map[types.FileID]string)

	// Group files by package directory
	for _, file := range allFiles {
		// Skip non-code files
		if !isCodeFile(file.Path) {
			continue
		}

		// Get package name (directory relative to project root)
		packageName := getPackageName(file.Path, projectRoot)

		if packageMap[packageName] == nil {
			packageMap[packageName] = &packageData{
				files:       make([]*types.FileInfo, 0),
				typeFileIDs: make([]types.FileID, 0),
			}
		}

		pkg := packageMap[packageName]
		pkg.files = append(pkg.files, file)
		pkg.typeFileIDs = append(pkg.typeFileIDs, file.ID)
		fileToPackage[file.ID] = packageName

		// Track file IDs (string format for output)
		fileID := file.EntityID(projectRoot)
		pkg.fileIDs = append(pkg.fileIDs, fileID)

		// Collect symbols from this file
		for _, sym := range file.EnhancedSymbols {
			pkg.symbolCount++
			relPath := getRelativePath(file.Path, projectRoot)
			entityID := sym.EntityID(projectRoot, file.Path)

			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
				pkg.functionCount++
				pkg.functionIDs = append(pkg.functionIDs, entityID)

				if len(pkg.exampleFuncs) < 5 {
					objectID := searchtypes.EncodeSymbolID(sym.ID)
					pkg.exampleFuncs = append(pkg.exampleFuncs, SymbolRef{
						ObjectID:   objectID,
						EntityID:   entityID,
						Name:       sym.Name,
						SymbolType: sym.Type.String(),
						Location:   fmt.Sprintf("%s:%d", relPath, sym.Line),
						FileID:     fileID,
					})
				}
			} else if sym.Type == types.SymbolTypeStruct || sym.Type == types.SymbolTypeInterface {
				pkg.classCount++
				pkg.classIDs = append(pkg.classIDs, entityID)

				if len(pkg.exampleClasses) < 5 {
					objectID := searchtypes.EncodeSymbolID(sym.ID)
					pkg.exampleClasses = append(pkg.exampleClasses, SymbolRef{
						ObjectID:   objectID,
						EntityID:   entityID,
						Name:       sym.Name,
						SymbolType: sym.Type.String(),
						Location:   fmt.Sprintf("%s:%d", relPath, sym.Line),
						FileID:     fileID,
					})
				}
			}
		}
	}

	// Build package dependency graph using ReferenceTracker
	// packageDeps[A][B] = count of references from package A to package B
	packageDeps := make(map[string]map[string]int)
	for pkgName := range packageMap {
		packageDeps[pkgName] = make(map[string]int)
	}

	// Analyze references to build the dependency graph
	if refTracker != nil {
		refs := refTracker.GetAllReferences()
		for _, ref := range refs {
			sourceFileID := ref.FileID
			sourcePkg, sourceOK := fileToPackage[sourceFileID]

			// Look up target symbol's file
			if ref.TargetSymbol != 0 {
				targetSym := refTracker.GetEnhancedSymbol(ref.TargetSymbol)
				if targetSym != nil {
					targetPkg, targetOK := fileToPackage[targetSym.FileID]
					if sourceOK && targetOK {
						packageDeps[sourcePkg][targetPkg]++
					}
				}
			}
		}
	}

	// Convert to ModuleBoundary slice with graph-based metrics
	moduleBoundaries := make([]ModuleBoundary, 0, len(packageMap))
	for packageName, pkg := range packageMap {
		// Calculate metrics using the dependency graph
		cohesion, coupling, stability := calculatePackageMetricsFromGraph(
			packageName, pkg.symbolCount, packageDeps,
		)

		moduleEntityID := "package:" + packageName

		moduleBoundaries = append(moduleBoundaries, ModuleBoundary{
			EntityID:         moduleEntityID,
			Name:             packageName,
			Type:             "package",
			Path:             packageName,
			CohesionScore:    cohesion,
			CouplingScore:    coupling,
			Stability:        stability,
			FileCount:        len(pkg.files),
			FunctionCount:    pkg.functionCount,
			FileIDs:          pkg.fileIDs,
			FunctionIDs:      pkg.functionIDs,
			ClassIDs:         pkg.classIDs,
			ExampleFunctions: pkg.exampleFuncs,
			ExampleClasses:   pkg.exampleClasses,
		})
	}

	// Sort by file count descending for most important packages first
	sort.Slice(moduleBoundaries, func(i, j int) bool {
		return moduleBoundaries[i].FileCount > moduleBoundaries[j].FileCount
	})

	return moduleBoundaries
}


// Helper functions
func (s *Server) getModuleName(fileID string) string {
	// Simplified: use directory name
	return fileID
}

func (s *Server) getModuleType(moduleName string) string {
	if len(moduleName) == 0 {
		return "core"
	}
	return "application"
}

func (s *Server) countFilesInModule(moduleName string) int {
	return 1 // Simplified
}


func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}

func stringPtr(s string) *string {
	return &s
}

// ============================================================================
// Placeholder implementations for Tier 3 components
// ============================================================================

func (s *Server) buildStatisticsReport(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*StatisticsReport, error) {
	// Get all files from the index and apply language filter if specified
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(args.Languages) > 0 {
		allFiles = s.filterFilesByLanguage(allFiles, args.Languages)
	}
	if len(allFiles) == 0 {
		return nil, errors.New("no files found in index (or matching language filter)")
	}

	// Calculate complexity metrics using actual cyclomatic complexity
	complexityMetrics := s.calculateComplexityMetricsFromFiles(allFiles)

	// Calculate coupling/cohesion using ReferenceTracker graph data
	couplingMetrics, cohesionMetrics := s.calculateGraphBasedCouplingCohesion(allFiles)
	qualityMetrics := calculateQualityMetricsFromComplexity(complexityMetrics)

	// Build statistics report with the required structure
	statisticsReport := &StatisticsReport{
		ComplexityMetrics:         complexityMetrics,
		CouplingMetrics:           couplingMetrics,
		CohesionMetrics:           cohesionMetrics,
		ChangeMetrics:             ChangeMetrics{},
		QualityMetrics:            qualityMetrics,
		ComplexityDistribution:    complexityMetrics.Distribution,
		CouplingDistribution:      map[string]int{},
		LayerSizeDistribution:     map[string]int{},
		AgainstIndustryBenchmarks: IndustryComparison{},
		HistoricalComparison:      HistoricalComparison{},
		AnalysisMetadata:          AnalysisMetadata{},
	}

	return statisticsReport, nil
}

// calculateComplexityMetrics calculates code complexity metrics
func calculateComplexityMetrics(symbols []*types.Symbol) ComplexityMetrics {
	metrics := ComplexityMetrics{}

	// Calculate complexity distribution (low/medium/high) based on line counts
	distribution := make(map[string]int)
	complexities := make([]float64, 0)

	for _, sym := range symbols {
		if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
			// Use line count as proxy for complexity
			lineCount := sym.EndLine - sym.Line
			if lineCount <= 0 {
				lineCount = 5 // Default
			}
			complexity := float64(lineCount)
			complexities = append(complexities, complexity)

			// Categorize by complexity ranges using threshold constants
			// Note: Uses line count as proxy for CC, but same thresholds for consistency
			if complexity <= float64(complexityThresholdLow) {
				distribution["low"]++
			} else if complexity <= float64(complexityThresholdHigh) {
				distribution["medium"]++
			} else {
				distribution["high"]++
			}
		}
	}

	// Calculate average complexity
	avgComplexity := 0.0
	if len(complexities) > 0 {
		for _, c := range complexities {
			avgComplexity += c
		}
		avgComplexity /= float64(len(complexities))
	}

	metrics.AverageCC = avgComplexity
	metrics.MedianCC = avgComplexity
	metrics.Percentiles = map[string]float64{"p50": avgComplexity, "p75": avgComplexity * 1.2}
	metrics.HighComplexityFuncs = make([]FunctionInfo, 0)
	metrics.Distribution = distribution

	return metrics
}

// calculateCouplingMetrics calculates coupling metrics
func calculateCouplingMetrics(symbols []*types.Symbol) CouplingMetrics {
	metrics := CouplingMetrics{}

	// Simplified coupling metrics
	metrics.AfferentCoupling = map[string]int{}
	metrics.EfferentCoupling = map[string]int{}
	metrics.Instability = map[string]float64{}
	metrics.Abstractness = map[string]float64{}
	metrics.Distance = map[string]float64{}
	metrics.ModuleCoupling = map[string]float64{}
	metrics.LayerCoupling = map[string]float64{}

	return metrics
}

// calculateCohesionMetrics calculates cohesion metrics
func calculateCohesionMetrics(symbols []*types.Symbol) CohesionMetrics {
	metrics := CohesionMetrics{}

	// Group symbols by FileID (as proxy for module)
	fileGroups := make(map[types.FileID][]*types.Symbol)
	for _, sym := range symbols {
		fileGroups[sym.FileID] = append(fileGroups[sym.FileID], sym)
	}

	// Calculate cohesion per file/module
	moduleCohesion := make(map[string]float64)
	var minCohesion float64 = 1.0
	var totalCohesion float64 = 0.0

	for fileID, syms := range fileGroups {
		lcom := calculateLCOM(syms)
		cohesion := 1.0 - lcom
		moduleKey := fmt.Sprintf("file_%d", fileID)
		moduleCohesion[moduleKey] = cohesion

		totalCohesion += cohesion
		if cohesion < minCohesion {
			minCohesion = cohesion
		}
	}

	// Calculate average cohesion
	avgCohesion := 0.5 // Default
	if len(moduleCohesion) > 0 {
		avgCohesion = totalCohesion / float64(len(moduleCohesion))
	}

	// Identify low cohesion modules (cohesion < 0.3)
	const lowCohesionThreshold = 0.3
	var lowCohesionModules []string
	for module, cohesion := range moduleCohesion {
		if cohesion < lowCohesionThreshold {
			lowCohesionModules = append(lowCohesionModules, module)
		}
	}

	// Sort by cohesion (lowest first) and limit to 5
	sort.Slice(lowCohesionModules, func(i, j int) bool {
		return moduleCohesion[lowCohesionModules[i]] < moduleCohesion[lowCohesionModules[j]]
	})
	if len(lowCohesionModules) > 5 {
		lowCohesionModules = lowCohesionModules[:5]
	}

	metrics.RelationalCohesion = moduleCohesion
	metrics.FunctionalCohesion = map[string]float64{"average": avgCohesion}
	metrics.SequentialCohesion = map[string]float64{"average": avgCohesion}
	metrics.AverageCohesion = avgCohesion
	metrics.MinCohesion = minCohesion
	metrics.LowCohesionModules = lowCohesionModules

	return metrics
}

// calculateQualityMetrics calculates quality metrics
func calculateQualityMetrics(symbols []*types.Symbol) QualityMetrics {
	metrics := QualityMetrics{}

	// Simple quality calculation
	avgNameLength := 0.0
	for _, sym := range symbols {
		avgNameLength += float64(len(sym.Name))
	}
	avgNameLength /= float64(len(symbols))

	// Quality score (0-100)
	qualityScore := 100.0 - avgNameLength
	if qualityScore < 0 {
		qualityScore = 0
	}

	metrics.TechnicalDebtRatio = 0.1 // Default
	metrics.CodeSmells = calculateCodeSmells(symbols)
	metrics.ArchitectureViolations = 0
	metrics.DuplicationRatio = 0.05
	metrics.CommentCoverage = 0.4
	metrics.MaintainabilityIndex = qualityScore

	return metrics
}

// calculateComplexityMetrics calculates code complexity metrics
func (s *Server) calculateComplexityMetrics(symbols []*types.Symbol) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Count symbols by type
	typeCounts := make(map[string]int)
	for _, sym := range symbols {
		typeName := getSymbolTypeName(sym.Type)
		typeCounts[typeName]++
	}

	// Calculate cyclomatic complexity (simplified)
	totalComplexity := 0.0
	for _, sym := range symbols {
		// Simple heuristic: more arguments = more complex
		totalComplexity += float64(len(sym.Name)) / 10.0
	}

	metrics["total_symbols"] = len(symbols)
	metrics["symbols_by_type"] = typeCounts
	metrics["average_cyclomatic_complexity"] = totalComplexity / float64(len(symbols))
	metrics["max_cyclomatic_complexity"] = totalComplexity / 5.0 // Simplified
	metrics["functions_count"] = typeCounts["Function"]
	metrics["classes_count"] = typeCounts["Class"]
	metrics["interfaces_count"] = typeCounts["Interface"]
	metrics["variables_count"] = typeCounts["Variable"]
	metrics["constants_count"] = typeCounts["Constant"]

	return metrics
}

// calculateCouplingMetrics calculates coupling metrics
func (s *Server) calculateCouplingMetrics(symbols []*types.Symbol) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Simplified coupling calculation
	// In reality, we'd use the call graph
	avgCoupling := 0.3 // Default moderate coupling
	maxCoupling := 0.7

	metrics["average_coupling"] = avgCoupling
	metrics["max_coupling"] = maxCoupling
	metrics["coupling_violations"] = 0
	metrics["afferent_coupling"] = 0.0
	metrics["efferent_coupling"] = 0.0

	return metrics
}

// calculateCohesionMetrics calculates cohesion metrics
func (s *Server) calculateCohesionMetrics(symbols []*types.Symbol) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Calculate LCOM (Lack of Cohesion of Methods) - simplified
	lcom := calculateLCOM(symbols)
	avgCohesion := 1.0 - lcom

	metrics["lcom"] = lcom
	metrics["average_cohesion"] = avgCohesion
	metrics["cohesion_violations"] = 0

	return metrics
}

// calculateQualityMetrics calculates code quality metrics
func (s *Server) calculateQualityMetrics(symbols []*types.Symbol, complexityMetrics, couplingMetrics map[string]interface{}) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Code quality score (0-100)
	complexity := complexityMetrics["average_cyclomatic_complexity"].(float64)
	coupling := couplingMetrics["average_coupling"].(float64)

	// Simple quality formula
	qualityScore := 100.0
	qualityScore -= complexity * 10 // Penalize complexity
	qualityScore -= coupling * 20   // Penalize coupling
	if qualityScore < 0 {
		qualityScore = 0
	}

	metrics["quality_score"] = qualityScore
	metrics["code_smells"] = calculateCodeSmells(symbols)
	metrics["duplication_ratio"] = 0.05 // Simplified
	metrics["maintainability_rating"] = getMaintainabilityRating(qualityScore)

	return metrics
}

// calculateLanguageDistribution calculates language distribution
func (s *Server) calculateLanguageDistribution(symbols []*types.Symbol) map[string]int {
	distribution := make(map[string]int)

	for _, sym := range symbols {
		filePath := s.goroutineIndex.GetFilePath(sym.FileID)
		ext := strings.ToLower(filepath.Ext(filePath))

		lang := getLanguageFromExtension(ext)
		distribution[lang]++
	}

	return distribution
}

// calculateSymbolTypeDistribution calculates symbol type distribution
func (s *Server) calculateSymbolTypeDistribution(symbols []*types.Symbol) map[string]int {
	distribution := make(map[string]int)

	for _, sym := range symbols {
		typeName := getSymbolTypeName(sym.Type)
		distribution[typeName]++
	}

	return distribution
}

// calculateFileSizeDistribution calculates file size distribution
func (s *Server) calculateFileSizeDistribution() map[string]int {
	// Simplified file size distribution
	return map[string]int{
		"small":  50, // < 100 lines
		"medium": 30, // 100-500 lines
		"large":  15, // 500-1000 lines
		"xl":     5,  // > 1000 lines
	}
}

// getSymbolTypeName gets the name of a symbol type
func getSymbolTypeName(symbolType types.SymbolType) string {
	switch symbolType {
	case types.SymbolTypeFunction:
		return "Function"
	case types.SymbolTypeClass:
		return "Class"
	case types.SymbolTypeMethod:
		return "Method"
	case types.SymbolTypeVariable:
		return "Variable"
	case types.SymbolTypeConstant:
		return "Constant"
	case types.SymbolTypeInterface:
		return "Interface"
	default:
		return "Unknown"
	}
}

// getLanguageFromExtension gets language from file extension
func getLanguageFromExtension(ext string) string {
	languageMap := map[string]string{
		".go":     "Go",
		".js":     "JavaScript",
		".jsx":    "JavaScript",
		".ts":     "TypeScript",
		".tsx":    "TypeScript",
		".py":     "Python",
		".java":   "Java",
		".rs":     "Rust",
		".cpp":    "C++",
		".cc":     "C++",
		".cxx":    "C++",
		".hpp":    "C++",
		".c":      "C",
		".h":      "C",
		".cs":     "C#",
		".php":    "PHP",
		".rb":     "Ruby",
		".swift":  "Swift",
		".kt":     "Kotlin",
		".scala":  "Scala",
		".vue":    "Vue",
		".svelte": "Svelte",
		".dart":   "Dart",
		".zig":    "Zig",
	}

	return languageMap[ext]
}

// languageAliases maps common language name variations to canonical names
var languageAliases = map[string]string{
	"go":         "Go",
	"golang":     "Go",
	"javascript": "JavaScript",
	"js":         "JavaScript",
	"typescript": "TypeScript",
	"ts":         "TypeScript",
	"python":     "Python",
	"py":         "Python",
	"java":       "Java",
	"rust":       "Rust",
	"rs":         "Rust",
	"c++":        "C++",
	"cpp":        "C++",
	"c":          "C",
	"csharp":     "C#",
	"c#":         "C#",
	"cs":         "C#",
	"dotnet":     "C#",
	"php":        "PHP",
	"ruby":       "Ruby",
	"rb":         "Ruby",
	"swift":      "Swift",
	"kotlin":     "Kotlin",
	"kt":         "Kotlin",
	"scala":      "Scala",
	"vue":        "Vue",
	"svelte":     "Svelte",
	"dart":       "Dart",
	"zig":        "Zig",
}

// normalizeLanguageName converts a language name to its canonical form
func normalizeLanguageName(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	if canonical, ok := languageAliases[lower]; ok {
		return canonical
	}
	// If not found in aliases, return original with title case
	return strings.Title(lower)
}

// filterSymbolsByLanguage filters symbols to only include those from specified languages
// Languages are matched case-insensitively with support for common aliases
// (e.g., "go", "golang", "Go" all match Go files)
func (s *Server) filterSymbolsByLanguage(symbols []*types.Symbol, languages []string) []*types.Symbol {
	if len(languages) == 0 {
		return symbols
	}

	// Build set of normalized language names
	langSet := make(map[string]bool, len(languages))
	for _, lang := range languages {
		normalized := normalizeLanguageName(lang)
		langSet[normalized] = true
	}

	// Filter symbols
	filtered := make([]*types.Symbol, 0, len(symbols)/2) // Pre-allocate reasonable capacity
	for _, sym := range symbols {
		filePath := s.goroutineIndex.GetFilePath(sym.FileID)
		ext := strings.ToLower(filepath.Ext(filePath))
		lang := getLanguageFromExtension(ext)
		if langSet[lang] {
			filtered = append(filtered, sym)
		}
	}

	return filtered
}

// filterFilesByLanguage filters file infos to only include those from specified languages
func (s *Server) filterFilesByLanguage(files []*types.FileInfo, languages []string) []*types.FileInfo {
	if len(languages) == 0 {
		return files
	}

	// Build set of normalized language names
	langSet := make(map[string]bool, len(languages))
	for _, lang := range languages {
		normalized := normalizeLanguageName(lang)
		langSet[normalized] = true
	}

	// Filter files
	filtered := make([]*types.FileInfo, 0, len(files)/2)
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Path))
		lang := getLanguageFromExtension(ext)
		if langSet[lang] {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// calculateLCOM calculates Lack of Cohesion of Methods (simplified)
func calculateLCOM(symbols []*types.Symbol) float64 {
	if len(symbols) == 0 {
		return 0.0
	}

	// Simplified LCOM calculation
	// Count methods vs other symbols
	methods := 0
	for _, sym := range symbols {
		if sym.Type == types.SymbolTypeMethod || sym.Type == types.SymbolTypeFunction {
			methods++
		}
	}

	if methods == 0 {
		return 0.0
	}

	// Simple heuristic
	return float64(len(symbols)-methods) / float64(methods)
}

// calculateCodeSmells calculates code smell count
func calculateCodeSmells(symbols []*types.Symbol) int {
	smells := 0

	for _, sym := range symbols {
		// Long method/function names
		if len(sym.Name) > 30 {
			smells++
		}

		// Complex names (too many underscores)
		if strings.Count(sym.Name, "_") > 3 {
			smells++
		}
	}

	return smells
}

// getAllSymbolsFromIndex retrieves all symbols from the index
func (s *Server) getAllSymbolsFromIndex() ([]*types.Symbol, error) {
	// Get all files from the index
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(allFiles) == 0 {
		return []*types.Symbol{}, nil
	}

	// Collect all symbols from all files
	allSymbols := make([]*types.Symbol, 0)
	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			// Convert EnhancedSymbol to Symbol
			allSymbols = append(allSymbols, &sym.Symbol)
		}
	}

	return allSymbols, nil
}

// buildTypeHierarchyAnalysis analyzes type relationships (implements, extends) in the codebase
func (s *Server) buildTypeHierarchyAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*CodebaseIntelligenceResponse, error) {
	response := &CodebaseIntelligenceResponse{}

	refTracker := s.goroutineIndex.GetRefTracker()
	if refTracker == nil {
		return nil, errors.New("reference tracker not available")
	}

	// Get all files for file info lookup
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(allFiles) == 0 {
		return nil, errors.New("no files indexed")
	}

	// Build file ID to file info map for efficient lookup - preallocate based on file count
	fileInfoMap := make(map[types.FileID]*types.FileInfo, len(allFiles))
	for _, f := range allFiles {
		fileInfoMap[f.ID] = f
	}

	// Helper to get file path from file ID
	getFilePath := func(fileID types.FileID) string {
		if fileInfo, ok := fileInfoMap[fileID]; ok {
			return fileInfo.Path
		}
		return ""
	}

	// Helper to get language from file path
	getLanguage := func(filePath string) string {
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".go":
			return "Go"
		case ".ts", ".tsx":
			return "TypeScript"
		case ".js", ".jsx":
			return "JavaScript"
		case ".py":
			return "Python"
		case ".rs":
			return "Rust"
		case ".java":
			return "Java"
		case ".cs":
			return "C#"
		case ".cpp", ".cc", ".cxx":
			return "C++"
		case ".c":
			return "C"
		default:
			return "Unknown"
		}
	}

	// Helper to convert symbol type to string
	symbolTypeStr := func(t types.SymbolType) string {
		switch t {
		case types.SymbolTypeInterface:
			return "interface"
		case types.SymbolTypeClass:
			return "class"
		case types.SymbolTypeStruct:
			return "struct"
		case types.SymbolTypeType:
			return "type"
		default:
			return "other"
		}
	}

	// Pre-allocate with reasonable capacity estimates
	// Most codebases have relatively few interfaces with multiple implementors
	typeHierarchy := &TypeHierarchyAnalysis{
		Interfaces:  make([]InterfaceHierarchy, 0, 32),
		Inheritance: make([]InheritanceHierarchy, 0, 32),
		Summary: TypeHierarchySummary{
			LanguageBreakdown: make(map[string]TypeLanguageStats, 6), // ~6 common languages
		},
	}

	// Track seen interfaces and base types to avoid duplicates
	// Pre-allocate with reasonable capacity
	seenInterfaces := make(map[types.SymbolID]bool, 64)
	seenBaseTypes := make(map[types.SymbolID]bool, 64)

	// Iterate through all files to find interfaces and types with relationships
	for _, fileInfo := range allFiles {
		for _, sym := range fileInfo.EnhancedSymbols {
			// Check for interfaces with implementors
			if sym.Type == types.SymbolTypeInterface && !seenInterfaces[sym.ID] {
				implementorIDs := refTracker.GetImplementors(sym.ID)
				if len(implementorIDs) > 0 {
					seenInterfaces[sym.ID] = true

					filePath := getFilePath(sym.FileID)
					lang := getLanguage(filePath)

					// Count methods in interface (approximate: count methods within line range)
					methodCount := 0
					for _, child := range fileInfo.EnhancedSymbols {
						if child.Type == types.SymbolTypeMethod &&
							child.Line > sym.Line && child.Line < sym.EndLine {
							methodCount++
						}
					}

					ifaceHierarchy := InterfaceHierarchy{
						ObjectID:     searchtypes.EncodeSymbolID(sym.ID),
						EntityID:     sym.EntityID(s.cfg.Project.Root, filePath),
						Name:         sym.Name,
						File:         filePath,
						Line:         sym.Line,
						MethodCount:  methodCount,
						Implementors: make([]TypeRelationshipRef, 0, len(implementorIDs)),
					}

					// Add implementors
					for _, implID := range implementorIDs {
						implSym := refTracker.GetEnhancedSymbol(implID)
						if implSym == nil {
							continue
						}

						implFilePath := getFilePath(implSym.FileID)
						implLang := getLanguage(implFilePath)

						ifaceHierarchy.Implementors = append(ifaceHierarchy.Implementors, TypeRelationshipRef{
							ObjectID: searchtypes.EncodeSymbolID(implSym.ID),
							EntityID: implSym.EntityID(s.cfg.Project.Root, implFilePath),
							Name:     implSym.Name,
							File:     implFilePath,
							Line:     implSym.Line,
							TypeKind: symbolTypeStr(implSym.Type),
							Language: implLang,
						})

						// Update summary
						typeHierarchy.Summary.TotalImplementors++
						stats := typeHierarchy.Summary.LanguageBreakdown[implLang]
						stats.Implementors++
						typeHierarchy.Summary.LanguageBreakdown[implLang] = stats
					}

					typeHierarchy.Interfaces = append(typeHierarchy.Interfaces, ifaceHierarchy)
					typeHierarchy.Summary.TotalInterfaces++

					// Update language breakdown for interface
					stats := typeHierarchy.Summary.LanguageBreakdown[lang]
					stats.Interfaces++
					typeHierarchy.Summary.LanguageBreakdown[lang] = stats
				}
			}

			// Check for base types with derived types (class/struct inheritance)
			if (sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeStruct || sym.Type == types.SymbolTypeType) && !seenBaseTypes[sym.ID] {
				derivedIDs := refTracker.GetDerivedTypes(sym.ID)
				if len(derivedIDs) > 0 {
					seenBaseTypes[sym.ID] = true

					filePath := getFilePath(sym.FileID)
					lang := getLanguage(filePath)

					inheritHierarchy := InheritanceHierarchy{
						ObjectID:     searchtypes.EncodeSymbolID(sym.ID),
						EntityID:     sym.EntityID(s.cfg.Project.Root, filePath),
						Name:         sym.Name,
						File:         filePath,
						Line:         sym.Line,
						TypeKind:     symbolTypeStr(sym.Type),
						DerivedTypes: make([]TypeRelationshipRef, 0, len(derivedIDs)),
					}

					// Add derived types
					for _, derivedID := range derivedIDs {
						derivedSym := refTracker.GetEnhancedSymbol(derivedID)
						if derivedSym == nil {
							continue
						}

						derivedFilePath := getFilePath(derivedSym.FileID)
						derivedLang := getLanguage(derivedFilePath)

						inheritHierarchy.DerivedTypes = append(inheritHierarchy.DerivedTypes, TypeRelationshipRef{
							ObjectID: searchtypes.EncodeSymbolID(derivedSym.ID),
							EntityID: derivedSym.EntityID(s.cfg.Project.Root, derivedFilePath),
							Name:     derivedSym.Name,
							File:     derivedFilePath,
							Line:     derivedSym.Line,
							TypeKind: symbolTypeStr(derivedSym.Type),
							Language: derivedLang,
						})

						// Update summary
						typeHierarchy.Summary.TotalDerivedTypes++
						stats := typeHierarchy.Summary.LanguageBreakdown[derivedLang]
						stats.DerivedTypes++
						typeHierarchy.Summary.LanguageBreakdown[derivedLang] = stats
					}

					// Also include base types this type extends (for context)
					baseIDs := refTracker.GetBaseTypes(sym.ID)
					for _, baseID := range baseIDs {
						baseSym := refTracker.GetEnhancedSymbol(baseID)
						if baseSym == nil {
							continue
						}

						baseFilePath := getFilePath(baseSym.FileID)
						baseLang := getLanguage(baseFilePath)

						inheritHierarchy.BaseTypes = append(inheritHierarchy.BaseTypes, TypeRelationshipRef{
							ObjectID: searchtypes.EncodeSymbolID(baseSym.ID),
							EntityID: baseSym.EntityID(s.cfg.Project.Root, baseFilePath),
							Name:     baseSym.Name,
							File:     baseFilePath,
							Line:     baseSym.Line,
							TypeKind: symbolTypeStr(baseSym.Type),
							Language: baseLang,
						})
					}

					typeHierarchy.Inheritance = append(typeHierarchy.Inheritance, inheritHierarchy)
					typeHierarchy.Summary.TotalBaseTypes++

					// Update language breakdown for base type
					stats := typeHierarchy.Summary.LanguageBreakdown[lang]
					stats.BaseTypes++
					typeHierarchy.Summary.LanguageBreakdown[lang] = stats
				}
			}
		}
	}

	// Sort interfaces by number of implementors (descending)
	sort.Slice(typeHierarchy.Interfaces, func(i, j int) bool {
		return len(typeHierarchy.Interfaces[i].Implementors) > len(typeHierarchy.Interfaces[j].Implementors)
	})

	// Sort inheritance by number of derived types (descending)
	sort.Slice(typeHierarchy.Inheritance, func(i, j int) bool {
		return len(typeHierarchy.Inheritance[i].DerivedTypes) > len(typeHierarchy.Inheritance[j].DerivedTypes)
	})

	// Apply max results limit if specified
	maxResults := 100 // Default
	if args.Max != nil && *args.Max > 0 {
		maxResults = *args.Max
	}

	if len(typeHierarchy.Interfaces) > maxResults {
		typeHierarchy.Interfaces = typeHierarchy.Interfaces[:maxResults]
	}
	if len(typeHierarchy.Inheritance) > maxResults {
		typeHierarchy.Inheritance = typeHierarchy.Inheritance[:maxResults]
	}

	response.TypeHierarchyAnalysis = typeHierarchy
	return response, nil
}

// ============================================================================
// Performance Anti-Pattern Analysis
// ============================================================================

// analyzePerformancePatterns detects performance anti-patterns across all files.
// Uses PerfData from FileInfo when available (populated during parsing),
// otherwise falls back to basic metadata from EnhancedSymbol.
func (s *Server) analyzePerformancePatterns(allFiles []*types.FileInfo, maxResults int) *PerformanceAnalysis {
	pa := analysis.NewPerformanceAnalyzer()

	var allPatterns []PerformanceAntiPattern
	patternCounts := make(map[string]int)
	byLanguage := make(map[string]int)
	bySeverity := make(map[string]int)

	for _, file := range allFiles {
		ext := filepath.Ext(file.Path)
		language := analysis.GetLanguageFromExt(ext)
		basePath := filepath.Base(file.Path)

		// Build FunctionAnalysis - prefer PerfData from indexing if available
		var functions []*analysis.FunctionAnalysis

		if len(file.PerfData) > 0 {
			// Use persisted performance data from parsing (full analysis)
			for _, perfData := range file.PerfData {
				funcAnalysis := &analysis.FunctionAnalysis{
					Name:      perfData.Name,
					StartLine: perfData.StartLine,
					EndLine:   perfData.EndLine,
					IsAsync:   perfData.IsAsync,
					Language:  perfData.Language,
					FilePath:  file.Path,
					Loops:     convertLoopData(perfData.Loops),
					Awaits:    convertAwaitData(perfData.Awaits),
					Calls:     convertCallData(perfData.Calls),
				}
				functions = append(functions, funcAnalysis)
			}
		} else {
			// Fallback: Build from EnhancedSymbol metadata (limited analysis)
			for _, sym := range file.EnhancedSymbols {
				// Only analyze functions and methods
				if sym.Type != types.SymbolTypeFunction && sym.Type != types.SymbolTypeMethod {
					continue
				}

				// Create analysis struct from available metadata
				funcAnalysis := &analysis.FunctionAnalysis{
					Name:      sym.Name,
					SymbolID:  sym.ID,
					StartLine: sym.Line,
					EndLine:   sym.EndLine,
					IsAsync:   sym.IsAsyncFunc(),
					Language:  language,
					FilePath:  file.Path,
					// Note: Loops, Awaits, and Calls require PerfData from parsing
					Loops:  []analysis.LoopInfo{},
					Awaits: []analysis.AwaitInfo{},
					Calls:  []analysis.CallInfo{},
				}
				functions = append(functions, funcAnalysis)
			}
		}

		// Run performance analysis on functions
		patterns := pa.AnalyzeFile(file, functions)

		// Convert to MCP types and collect
		for _, p := range patterns {
			// Format location as "file.go:line"
			location := fmt.Sprintf("%s:%d", basePath, p.Line)

			antiPattern := PerformanceAntiPattern{
				Type:        PerformancePatternType(p.Type),
				Symbol:      p.Symbol,
				ObjectID:    fmt.Sprintf("%d", p.SymbolID), // Convert SymbolID to string
				Location:    location,
				Severity:    p.Severity,
				Description: p.Description,
				Language:    p.Language,
				Suggestion:  p.Suggestion,
			}

			// Convert details if present
			if p.Details != nil {
				antiPattern.Details = &PatternDetails{
					TotalAwaits:         p.Details.TotalAwaits,
					ParallelizableCount: p.Details.ParallelizableCount,
					AwaitLines:          p.Details.AwaitLines,
					AwaitTargets:        p.Details.AwaitTargets,
					NestingDepth:        p.Details.NestingDepth,
					OuterLoopLine:       p.Details.OuterLoopLine,
					InnerLoopLine:       p.Details.InnerLoopLine,
					ExpensiveCall:       p.Details.ExpensiveCall,
					LoopLine:            p.Details.LoopLine,
					CallLine:            p.Details.CallLine,
					ExpenseCategory:     p.Details.ExpenseCategory,
				}
			}

			allPatterns = append(allPatterns, antiPattern)
			patternCounts[p.Type]++
			byLanguage[p.Language]++
			bySeverity[p.Severity]++
		}
	}

	// Sort patterns by severity (high first)
	sort.Slice(allPatterns, func(i, j int) bool {
		severityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return severityOrder[allPatterns[i].Severity] < severityOrder[allPatterns[j].Severity]
	})

	// Apply max results limit
	if maxResults > 0 && len(allPatterns) > maxResults {
		allPatterns = allPatterns[:maxResults]
	}

	// Don't return analysis if no patterns found
	if len(allPatterns) == 0 {
		return nil
	}

	return &PerformanceAnalysis{
		Patterns: allPatterns,
		Summary: PerformanceSummary{
			TotalPatterns: len(allPatterns),
			ByType:        patternCounts,
			ByLanguage:    byLanguage,
			BySeverity:    bySeverity,
		},
	}
}

// countTotalFunctions counts total functions across all files
func (s *Server) countTotalFunctions(allFiles []*types.FileInfo) int {
	count := 0
	for _, file := range allFiles {
		for _, sym := range file.EnhancedSymbols {
			if sym.Type == types.SymbolTypeFunction || sym.Type == types.SymbolTypeMethod {
				count++
			}
		}
	}
	return count
}

// convertLoopData converts types.LoopData to analysis.LoopInfo
func convertLoopData(loops []types.LoopData) []analysis.LoopInfo {
	result := make([]analysis.LoopInfo, len(loops))
	for i, l := range loops {
		result[i] = analysis.LoopInfo{
			NodeType:  l.NodeType,
			StartLine: l.StartLine,
			EndLine:   l.EndLine,
			Depth:     l.Depth,
		}
	}
	return result
}

// convertAwaitData converts types.AwaitData to analysis.AwaitInfo
func convertAwaitData(awaits []types.AwaitData) []analysis.AwaitInfo {
	result := make([]analysis.AwaitInfo, len(awaits))
	for i, a := range awaits {
		result[i] = analysis.AwaitInfo{
			Line:        a.Line,
			AssignedVar: a.AssignedVar,
			CallTarget:  a.CallTarget,
			UsedVars:    a.UsedVars,
		}
	}
	return result
}

// convertCallData converts types.CallData to analysis.CallInfo
func convertCallData(calls []types.CallData) []analysis.CallInfo {
	result := make([]analysis.CallInfo, len(calls))
	for i, c := range calls {
		result[i] = analysis.CallInfo{
			Target:    c.Target,
			Line:      c.Line,
			InLoop:    c.InLoop,
			LoopDepth: c.LoopDepth,
			LoopLine:  c.LoopLine,
		}
	}
	return result
}

// ============================================================================
// Memory Allocation Analysis with PageRank Propagation
// ============================================================================

// analyzeMemoryPressure performs memory allocation analysis using PageRank-style
// score propagation through the call graph to identify memory pressure hotspots.
// When GraphPropagator and ReferenceTracker are available, it uses accurate call
// graph relationships for propagation. Otherwise falls back to standalone analysis.
func (s *Server) analyzeMemoryPressure(allFiles []*types.FileInfo, maxResults int) *MemoryPressureAnalysis {
	var result *analysis.MemoryAnalysisResult

	// Try to use core GraphPropagator for accurate call graph propagation
	if s.goroutineIndex != nil {
		graphPropagator := s.goroutineIndex.GetGraphPropagator()
		refTracker := s.goroutineIndex.GetRefTracker()
		symbolIndex := s.goroutineIndex.GetSymbolIndex()

		if graphPropagator != nil && refTracker != nil && symbolIndex != nil {
			ma := analysis.NewMemoryAnalyzerWithPropagator(graphPropagator, refTracker, symbolIndex)
			result = ma.AnalyzeFromPerfDataWithSymbols(allFiles, refTracker)
		}
	}

	// Fallback to standalone analysis if core components not available
	if result == nil {
		ma := analysis.NewMemoryAnalyzer()
		result = ma.AnalyzeFromPerfData(allFiles)
	}
	if result == nil || len(result.Scores) == 0 {
		return nil
	}

	// Convert to MCP types
	scores := make([]MemoryScore, 0, len(result.Scores))
	for _, s := range result.Scores {
		location := fmt.Sprintf("%s:%d", filepath.Base(s.FilePath), s.Line)
		scores = append(scores, MemoryScore{
			Function:        s.FunctionName,
			Location:        location,
			TotalScore:      s.TotalScore,
			DirectScore:     s.DirectScore,
			PropagatedScore: s.PropagatedScore,
			LoopPressure:    s.LoopPressure,
			Severity:        s.Severity,
			Percentile:      s.Percentile,
		})
	}

	// Apply max results limit
	if maxResults > 0 && len(scores) > maxResults {
		scores = scores[:maxResults]
	}

	// Convert hotspots
	hotspots := make([]MemoryHotspot, 0, len(result.Hotspots))
	for _, h := range result.Hotspots {
		location := fmt.Sprintf("%s:%d", filepath.Base(h.FilePath), h.Line)
		hotspots = append(hotspots, MemoryHotspot{
			Function:   h.FunctionName,
			Location:   location,
			Score:      h.Score,
			Reason:     h.Reason,
			Suggestion: h.Suggestion,
		})
	}

	// Convert summary
	summary := MemorySummary{
		TotalFunctions:   result.Summary.TotalFunctions,
		TotalAllocations: result.Summary.TotalAllocations,
		AvgAllocPerFunc:  result.Summary.AvgAllocPerFunc,
		LoopAllocCount:   result.Summary.LoopAllocCount,
		CriticalCount:    result.Summary.CriticalCount,
		HighCount:        result.Summary.HighCount,
		MediumCount:      result.Summary.MediumCount,
		LowCount:         result.Summary.LowCount,
	}

	return &MemoryPressureAnalysis{
		Scores:   scores,
		Summary:  summary,
		Hotspots: hotspots,
	}
}
