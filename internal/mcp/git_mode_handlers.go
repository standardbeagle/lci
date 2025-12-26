package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/standardbeagle/lci/internal/git"
)

// buildGitAnalyzeAnalysis performs git change analysis for duplicates and naming consistency
// @lci:labels[git-analyze,duplicate-detection,naming-consistency]
// @lci:category[code-insight]
func (s *Server) buildGitAnalyzeAnalysis(ctx context.Context, args CodebaseIntelligenceParams) (*CodebaseIntelligenceResponse, error) {
	// Validate git params
	if args.Git == nil {
		args.Git = &GitAnalysisParams{}
	}

	// Determine project root
	projectRoot := s.determineProjectRoot(s.cfg)

	// Create git provider
	gitProvider, err := git.NewProvider(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create git provider: %w (ensure you're in a git repository)", err)
	}

	// Convert to git.AnalysisParams
	gitParams := git.DefaultAnalysisParams()

	// Set scope
	if args.Git.Scope != "" {
		switch args.Git.Scope {
		case "staged":
			gitParams.Scope = git.ScopeStaged
		case "wip":
			gitParams.Scope = git.ScopeWIP
		case "commit":
			gitParams.Scope = git.ScopeCommit
		case "range":
			gitParams.Scope = git.ScopeRange
		default:
			gitParams.Scope = git.ScopeStaged // default
		}
	}

	// Set refs
	if args.Git.BaseRef != "" {
		gitParams.BaseRef = args.Git.BaseRef
	}
	if args.Git.TargetRef != "" {
		gitParams.TargetRef = args.Git.TargetRef
	}

	// Set focus
	if len(args.Git.Focus) > 0 {
		gitParams.Focus = args.Git.Focus
	}

	// Set thresholds
	if args.Git.SimilarityThreshold > 0 {
		gitParams.SimilarityThreshold = args.Git.SimilarityThreshold
	}
	if args.Git.MaxFindings > 0 {
		gitParams.MaxFindings = args.Git.MaxFindings
	}

	// Create analyzer and run analysis
	analyzer := git.NewAnalyzer(gitProvider, s.goroutineIndex)
	report, err := analyzer.Analyze(ctx, gitParams)
	if err != nil {
		return nil, fmt.Errorf("git analysis failed: %w", err)
	}

	// Convert git.AnalysisReport to JSON for embedding in StatisticsReport
	reportJSON, err := json.Marshal(map[string]interface{}{
		"summary":       report.Summary,
		"duplicates":    report.Duplicates,
		"naming_issues": report.NamingIssues,
		"metadata":      report.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal git report: %w", err)
	}

	// Create response using StatisticsReport
	response := &CodebaseIntelligenceResponse{
		StatisticsReport: &StatisticsReport{
			GitAnalysis: reportJSON,
		},
	}

	return response, nil
}

// buildGitHotspotsAnalysis performs git change frequency analysis and hotspot detection
// @lci:labels[git-hotspots,change-frequency,collision-detection]
// @lci:category[code-insight]
func (s *Server) buildGitHotspotsAnalysis(ctx context.Context, args CodebaseIntelligenceParams) (*CodebaseIntelligenceResponse, error) {
	// Validate git params
	if args.Git == nil {
		args.Git = &GitAnalysisParams{}
	}

	// Determine project root
	projectRoot := s.determineProjectRoot(s.cfg)

	// Create git provider
	gitProvider, err := git.NewProvider(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create git provider: %w (ensure you're in a git repository)", err)
	}

	// Create frequency analyzer
	analyzer := git.NewFrequencyAnalyzer(gitProvider)

	// Convert to git.ChangeFrequencyParams
	freqParams := git.ChangeFrequencyParams{
		TimeWindow:  "30d",  // default
		Granularity: "file", // default
	}

	// Set time window
	if args.Git.TimeWindow != "" {
		freqParams.TimeWindow = args.Git.TimeWindow
	}

	// Set granularity
	if args.Git.Granularity != "" {
		freqParams.Granularity = args.Git.Granularity
	}

	// Set focus
	if len(args.Git.Focus) > 0 {
		freqParams.Focus = args.Git.Focus
	}

	// Set file pattern
	if args.Git.FilePattern != "" {
		freqParams.FilePattern = args.Git.FilePattern
	}

	// Set file path
	if args.Git.FilePath != "" {
		freqParams.FilePath = args.Git.FilePath
	}

	// Set symbol name
	if args.Git.SymbolName != "" {
		freqParams.SymbolName = args.Git.SymbolName
	}

	// Set thresholds
	if args.Git.MinChanges > 0 {
		freqParams.MinChanges = args.Git.MinChanges
	} else {
		freqParams.MinChanges = 2 // default
	}

	if args.Git.MinContributors > 0 {
		freqParams.MinContributors = args.Git.MinContributors
	} else {
		freqParams.MinContributors = 2 // default
	}

	if args.Git.TopN > 0 {
		freqParams.TopN = args.Git.TopN
	} else {
		freqParams.TopN = 50 // default
	}

	// Set patterns
	if len(args.Git.IncludePatterns) > 0 {
		freqParams.IncludePatterns = args.Git.IncludePatterns
	}
	if len(args.Git.ExcludePatterns) > 0 {
		freqParams.ExcludePatterns = args.Git.ExcludePatterns
	}
	if args.Git.SkipDefaultExclusions {
		freqParams.SkipDefaultExclusions = args.Git.SkipDefaultExclusions
	}

	// Run analysis
	report, err := analyzer.Analyze(ctx, freqParams)
	if err != nil {
		return nil, fmt.Errorf("git frequency analysis failed: %w", err)
	}

	// Convert git.ChangeFrequencyReport to JSON for embedding in StatisticsReport
	reportJSON, err := json.Marshal(map[string]interface{}{
		"summary":       report.Summary,
		"hotspots":      report.Hotspots,
		"collisions":    report.Collisions,
		"anti_patterns": report.AntiPatterns,
		"ownership":     report.Ownership,
		"metadata":      report.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal frequency report: %w", err)
	}

	// Create response using StatisticsReport
	response := &CodebaseIntelligenceResponse{
		StatisticsReport: &StatisticsReport{
			GitHotspots: reportJSON,
		},
	}

	return response, nil
}

// shouldIncludeFocus checks if a specific focus area should be included
func shouldIncludeFocus(focus []string, target string) bool {
	if len(focus) == 0 {
		return true // include all by default
	}
	for _, f := range focus {
		if f == target || f == "all" {
			return true
		}
	}
	return false
}
