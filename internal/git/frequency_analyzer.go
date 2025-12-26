package git

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// excludedFilePatterns contains patterns for files that should be excluded from churn analysis
// These are typically documentation, generated files, or files that change frequently but
// don't represent meaningful code churn
var excludedFilePatterns = []string{
	// Documentation files
	"CHANGELOG*",
	"HISTORY*",
	"CHANGES*",
	"NEWS*",
	"RELEASE*",
	"*.md",
	"*.rst",
	"*.txt",
	"docs/*",
	"doc/*",
	"documentation/*",

	// Generated/minified files
	"*.min.js",
	"*.min.css",
	"*.bundle.js",
	"*.bundle.css",
	"*.generated.*",
	"*.g.dart",
	"*.freezed.dart",

	// Type definitions (often auto-generated)
	"*.d.ts",
	"index.d.ts",

	// Lock files
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Gemfile.lock",
	"poetry.lock",
	"Cargo.lock",
	"go.sum",
	"composer.lock",

	// Build artifacts and output directories
	"dist/*",
	"build/*",
	"out/*",
	"target/*",
	".next/*",
	"bin/*",
	"obj/*",
	"Debug/*",
	"Release/*",
	"x64/*",
	"x86/*",
	"artifacts/*",
	"output/*",
	"_build/*",
	"__pycache__/*",
	".cache/*",

	// Binary executables and libraries
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
	"*.a",
	"*.lib",
	"*.o",
	"*.obj",
	"*.pyc",
	"*.pyo",
	"*.class",
	"*.jar",
	"*.war",
	"*.ear",
	"*.wasm",

	// Compiled/binary assets
	"*.bin",
	"*.dat",
	"*.db",
	"*.sqlite",
	"*.sqlite3",
	"*.mdb",
	"*.ldb",

	// Images and media (binary, not code)
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.ico",
	"*.svg",
	"*.webp",
	"*.bmp",
	"*.tiff",
	"*.mp3",
	"*.mp4",
	"*.wav",
	"*.avi",
	"*.mov",
	"*.webm",
	"*.ogg",
	"*.flac",
	"*.pdf",

	// Fonts
	"*.woff",
	"*.woff2",
	"*.ttf",
	"*.otf",
	"*.eot",

	// Archives
	"*.zip",
	"*.tar",
	"*.gz",
	"*.tgz",
	"*.bz2",
	"*.xz",
	"*.7z",
	"*.rar",

	// Package files
	"*.nupkg",
	"*.gem",
	"*.egg",
	"*.whl",

	// Vendored dependencies
	"vendor/*",
	"node_modules/*",
	"third_party/*",
	"packages/*",
	"bower_components/*",

	// IDE/editor files
	".idea/*",
	".vscode/*",
	"*.iml",
	"*.suo",
	"*.user",
	"*.userosscache",
	"*.sln.docstates",

	// Config that changes often but isn't code
	".github/*",
	".gitlab-ci.yml",
	".travis.yml",
	"Jenkinsfile",

	// Coverage and test output
	"coverage/*",
	".nyc_output/*",
	"*.coverage",
	"*.lcov",
	"test-results/*",
	"junit.xml",
}

// excludedExactFiles contains exact filenames to exclude
var excludedExactFiles = map[string]bool{
	"CHANGELOG.md":      true,
	"CHANGELOG":         true,
	"HISTORY.md":        true,
	"CHANGES.md":        true,
	"NEWS.md":           true,
	"RELEASES.md":       true,
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"go.sum":            true,
	"Cargo.lock":        true,
	"poetry.lock":       true,
	"composer.lock":     true,
	"Gemfile.lock":      true,
}

// ChurnFilterConfig holds configuration for file filtering in churn analysis
type ChurnFilterConfig struct {
	IncludePatterns       []string
	ExcludePatterns       []string
	SkipDefaultExclusions bool
}

// shouldExcludeFromChurn checks if a file should be excluded from churn analysis
func shouldExcludeFromChurn(filePath string) bool {
	return shouldExcludeFromChurnWithConfig(filePath, ChurnFilterConfig{})
}

// shouldExcludeFromChurnWithConfig checks if a file should be excluded with custom config
func shouldExcludeFromChurnWithConfig(filePath string, config ChurnFilterConfig) bool {
	// Normalize path separators
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
	lowerPath := strings.ToLower(normalizedPath)
	baseName := filepath.Base(filePath)
	lowerBase := strings.ToLower(baseName)

	// If include patterns are specified, file MUST match at least one
	if len(config.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range config.IncludePatterns {
			if matchesGlobPattern(lowerPath, lowerBase, strings.ToLower(pattern)) {
				matched = true
				break
			}
		}
		if !matched {
			return true // Exclude if not matching any include pattern
		}
	}

	// Check custom exclude patterns first (highest priority)
	for _, pattern := range config.ExcludePatterns {
		if matchesGlobPattern(lowerPath, lowerBase, strings.ToLower(pattern)) {
			return true
		}
	}

	// Skip default exclusions if requested
	if config.SkipDefaultExclusions {
		return false
	}

	// Check exact matches (fast path)
	if excludedExactFiles[baseName] {
		return true
	}

	// Check default exclusion patterns
	for _, pattern := range excludedFilePatterns {
		if matchesGlobPattern(lowerPath, lowerBase, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// matchesGlobPattern checks if a file path matches a glob pattern
func matchesGlobPattern(lowerPath, lowerBase, pattern string) bool {
	// Handle glob patterns
	if strings.Contains(pattern, "*") {
		// Check against full path
		if matched, _ := filepath.Match(pattern, lowerPath); matched {
			return true
		}
		// Check against base name
		if matched, _ := filepath.Match(pattern, lowerBase); matched {
			return true
		}
		// Check if pattern matches any path component (e.g., "vendor/*")
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(lowerPath, prefix+"/") || strings.Contains(lowerPath, "/"+prefix+"/") {
				return true
			}
		}
		// Check for ** patterns (match any depth)
		if strings.Contains(pattern, "**") {
			// Simple prefix match for common cases
			// Note: regex pattern matching was removed (unused)
			prefix := strings.Split(pattern, "**")[0]
			if prefix != "" && strings.HasPrefix(lowerPath, prefix) {
				return true
			}
		}
	} else {
		// Exact match
		if lowerBase == pattern || lowerPath == pattern {
			return true
		}
	}
	return false
}

// FrequencyAnalyzer performs change frequency analysis
type FrequencyAnalyzer struct {
	provider *HistoryProvider
	cache    *FrequencyCache
}

// NewFrequencyAnalyzer creates a new frequency analyzer
func NewFrequencyAnalyzer(provider *Provider) *FrequencyAnalyzer {
	return &FrequencyAnalyzer{
		provider: NewHistoryProvider(provider),
		cache:    NewFrequencyCache(10 * time.Minute),
	}
}

// Analyze performs change frequency analysis based on parameters
func (a *FrequencyAnalyzer) Analyze(ctx context.Context, params ChangeFrequencyParams) (*ChangeFrequencyReport, error) {
	startTime := time.Now()

	// Apply defaults
	if params.TopN <= 0 {
		params.TopN = 50
	}
	if params.MinChanges <= 0 {
		params.MinChanges = 2
	}
	if params.MinContributors <= 0 {
		params.MinContributors = 2
	}

	window := params.GetTimeWindow()
	windowDuration := TimeWindowToDuration(window)
	since := time.Now().Add(-windowDuration)

	report := &ChangeFrequencyReport{
		Metadata: ChangeFrequencyMetadata{
			AnalyzedAt:  time.Now(),
			TimeWindow:  string(window),
			WindowStart: since,
			WindowEnd:   time.Now(),
		},
	}

	// Get commit history
	var commits []CommitInfo
	var err error

	if params.FilePath != "" {
		// Single file analysis
		commits, err = a.provider.GetFileHistory(ctx, params.FilePath, since)
	} else if params.FilePattern != "" {
		// Pattern-based analysis
		commits, err = a.provider.GetRepoHistory(ctx, since, params.FilePattern)
	} else {
		// Full repo analysis
		commits, err = a.provider.GetRepoHistory(ctx, since, "")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get commit history: %w", err)
	}

	report.Summary.TotalCommitsAnalyzed = len(commits)

	// Build filter config from params
	filterConfig := ChurnFilterConfig{
		IncludePatterns:       params.IncludePatterns,
		ExcludePatterns:       params.ExcludePatterns,
		SkipDefaultExclusions: params.SkipDefaultExclusions,
	}

	// Aggregate by file
	fileStats := a.aggregateByFile(commits, window, filterConfig)
	report.Summary.TotalFilesAnalyzed = len(fileStats)

	// Apply analysis based on focus areas
	if params.HasFocus(FocusHotspots) {
		report.Hotspots = a.findHotspots(fileStats, params.MinChanges, params.TopN)
		report.Summary.HotspotsFound = len(report.Hotspots)
		if len(report.Hotspots) > 0 {
			report.Summary.HighestChurn = report.Hotspots[0].FilePath
		}
	}

	if params.HasFocus(FocusCollisions) {
		report.Collisions = a.findCollisions(fileStats, params.MinContributors)
		report.Summary.CollisionZones = len(report.Collisions)
	}

	if params.HasFocus(FocusOwnership) {
		report.Ownership = a.calculateOwnership(fileStats)
	}

	// Symbol-level analysis if requested
	if params.GetGranularity() == GranularitySymbol && params.FilePath != "" && params.SymbolName != "" {
		symbolFreq, err := a.analyzeSymbol(ctx, params)
		if err == nil && symbolFreq != nil {
			report.SymbolDetails = []SymbolChangeFrequency{*symbolFreq}
		}
	}

	// Find most active contributor
	report.Summary.MostActiveContributor = a.findMostActiveContributor(commits)

	report.Metadata.ComputeTimeMs = time.Since(startTime).Milliseconds()
	return report, nil
}

// aggregateByFile groups commits by file and calculates metrics
func (a *FrequencyAnalyzer) aggregateByFile(commits []CommitInfo, window TimeWindow, filterConfig ChurnFilterConfig) map[string]*FileChangeFrequency {
	fileStats := make(map[string]*FileChangeFrequency)
	windowDuration := TimeWindowToDuration(window)
	windowDays := windowDuration.Hours() / 24

	// First pass: aggregate file changes
	for _, commit := range commits {
		for _, fc := range commit.FileChanges {
			path := fc.Path
			if path == "" {
				continue
			}

			// Skip documentation, generated, and non-code files (respects custom config)
			if shouldExcludeFromChurnWithConfig(path, filterConfig) {
				continue
			}

			if _, exists := fileStats[path]; !exists {
				fileStats[path] = &FileChangeFrequency{
					FilePath: path,
					Metrics: map[TimeWindow]*FrequencyMetrics{
						window: {},
					},
					Contributors: []ContributorActivity{},
				}
			}

			stats := fileStats[path]
			metrics := stats.Metrics[window]

			metrics.ChangeCount++
			metrics.LinesAdded += fc.LinesAdded
			metrics.LinesDeleted += fc.LinesDeleted

			// Update timestamps
			if metrics.FirstChangeAt.IsZero() || commit.Timestamp.Before(metrics.FirstChangeAt) {
				metrics.FirstChangeAt = commit.Timestamp
			}
			if commit.Timestamp.After(metrics.LastChangeAt) {
				metrics.LastChangeAt = commit.Timestamp
			}
		}

		// Track contributors per file
		for _, fc := range commit.FileChanges {
			path := fc.Path
			if path == "" || fileStats[path] == nil {
				continue
			}

			stats := fileStats[path]
			found := false
			for i, c := range stats.Contributors {
				if c.AuthorEmail == commit.AuthorEmail {
					stats.Contributors[i].ChangeCount++
					stats.Contributors[i].LinesAdded += fc.LinesAdded
					stats.Contributors[i].LinesDeleted += fc.LinesDeleted
					if commit.Timestamp.After(stats.Contributors[i].LastChangeAt) {
						stats.Contributors[i].LastChangeAt = commit.Timestamp
					}
					found = true
					break
				}
			}
			if !found {
				stats.Contributors = append(stats.Contributors, ContributorActivity{
					AuthorName:   commit.AuthorName,
					AuthorEmail:  commit.AuthorEmail,
					ChangeCount:  1,
					LinesAdded:   fc.LinesAdded,
					LinesDeleted: fc.LinesDeleted,
					LastChangeAt: commit.Timestamp,
				})
			}
		}
	}

	// Second pass: calculate derived metrics
	for _, stats := range fileStats {
		metrics := stats.Metrics[window]
		metrics.UniqueAuthors = len(stats.Contributors)
		metrics.ChangeRate = float64(metrics.ChangeCount) / windowDays
		metrics.VolatilityScore = CalculateVolatilityScore(
			metrics.ChangeCount,
			metrics.LinesAdded+metrics.LinesDeleted,
			metrics.UniqueAuthors,
			windowDays,
		)

		// Calculate ownership shares
		totalChanges := metrics.ChangeCount
		for i := range stats.Contributors {
			if totalChanges > 0 {
				stats.Contributors[i].OwnershipShare = float64(stats.Contributors[i].ChangeCount) / float64(totalChanges)
			}
		}

		// Sort contributors by change count
		sort.Slice(stats.Contributors, func(i, j int) bool {
			return stats.Contributors[i].ChangeCount > stats.Contributors[j].ChangeCount
		})
	}

	return fileStats
}

// findHotspots returns the most frequently changed files
func (a *FrequencyAnalyzer) findHotspots(fileStats map[string]*FileChangeFrequency, minChanges, topN int) []FileChangeFrequency {
	var hotspots []FileChangeFrequency

	for _, stats := range fileStats {
		// Get the first window's metrics
		for _, metrics := range stats.Metrics {
			if metrics.ChangeCount >= minChanges {
				hotspots = append(hotspots, *stats)
			}
			break
		}
	}

	// Sort by volatility score
	sort.Slice(hotspots, func(i, j int) bool {
		// Get volatility from first metric
		var vi, vj float64
		for _, m := range hotspots[i].Metrics {
			vi = m.VolatilityScore
			break
		}
		for _, m := range hotspots[j].Metrics {
			vj = m.VolatilityScore
			break
		}
		return vi > vj
	})

	// Limit results
	if len(hotspots) > topN {
		hotspots = hotspots[:topN]
	}

	return hotspots
}

// findCollisions identifies files with multiple active contributors
func (a *FrequencyAnalyzer) findCollisions(fileStats map[string]*FileChangeFrequency, minContributors int) []CollisionZone {
	var collisions []CollisionZone

	for _, stats := range fileStats {
		if len(stats.Contributors) < minContributors {
			continue
		}

		// Count recent changes (last 7 days)
		recentChanges := 0
		recentCutoff := time.Now().Add(-7 * 24 * time.Hour)
		for _, c := range stats.Contributors {
			if c.LastChangeAt.After(recentCutoff) {
				recentChanges += c.ChangeCount
			}
		}

		score := CalculateCollisionScore(stats.Contributors, recentChanges)
		severity := DetermineCollisionSeverity(score)

		zone := CollisionZone{
			EntityType:     "file",
			Path:           stats.FilePath,
			Contributors:   stats.Contributors,
			CollisionScore: score,
			Severity:       severity,
			RecentChanges:  recentChanges,
		}

		// Generate recommendation
		zone.Recommendation = generateCollisionRecommendation(stats, severity)

		collisions = append(collisions, zone)
	}

	// Sort by collision score
	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].CollisionScore > collisions[j].CollisionScore
	})

	return collisions
}

// generateCollisionRecommendation creates actionable advice
func generateCollisionRecommendation(stats *FileChangeFrequency, severity CollisionSeverity) string {
	switch severity {
	case SeverityCritical:
		if len(stats.Contributors) > 0 {
			return fmt.Sprintf("High collision risk. Primary owner: %s (%.0f%%). Coordinate before making changes.",
				stats.Contributors[0].AuthorName, stats.Contributors[0].OwnershipShare*100)
		}
		return "High collision risk. Multiple developers actively editing. Coordinate changes."
	case SeverityWarning:
		return "Moderate collision risk. Consider notifying recent contributors."
	default:
		return "Low collision risk, but multiple contributors. Be aware of potential conflicts."
	}
}

// calculateOwnership groups files by directory and calculates module ownership
func (a *FrequencyAnalyzer) calculateOwnership(fileStats map[string]*FileChangeFrequency) []ModuleOwnership {
	moduleStats := make(map[string]*ModuleOwnership)

	for _, stats := range fileStats {
		// Extract module/directory path
		modulePath := extractModulePath(stats.FilePath)
		if modulePath == "" {
			modulePath = "."
		}

		if _, exists := moduleStats[modulePath]; !exists {
			moduleStats[modulePath] = &ModuleOwnership{
				ModulePath: modulePath,
			}
		}

		module := moduleStats[modulePath]
		module.FileCount++

		// Aggregate changes
		for _, metrics := range stats.Metrics {
			module.TotalChanges += metrics.ChangeCount
			break
		}
	}

	// Aggregate contributors per module
	moduleContributors := make(map[string]map[string]*ContributorActivity)
	for path, stats := range fileStats {
		modulePath := extractModulePath(path)
		if modulePath == "" {
			modulePath = "."
		}

		if moduleContributors[modulePath] == nil {
			moduleContributors[modulePath] = make(map[string]*ContributorActivity)
		}

		for _, c := range stats.Contributors {
			if existing, ok := moduleContributors[modulePath][c.AuthorEmail]; ok {
				existing.ChangeCount += c.ChangeCount
				existing.LinesAdded += c.LinesAdded
				existing.LinesDeleted += c.LinesDeleted
				if c.LastChangeAt.After(existing.LastChangeAt) {
					existing.LastChangeAt = c.LastChangeAt
				}
			} else {
				copy := c
				moduleContributors[modulePath][c.AuthorEmail] = &copy
			}
		}
	}

	// Find primary and secondary owners
	for modulePath, module := range moduleStats {
		contributors := moduleContributors[modulePath]
		var contribList []ContributorActivity
		for _, c := range contributors {
			contribList = append(contribList, *c)
		}

		// Calculate ownership shares
		for i := range contribList {
			if module.TotalChanges > 0 {
				contribList[i].OwnershipShare = float64(contribList[i].ChangeCount) / float64(module.TotalChanges)
			}
		}

		// Sort by change count
		sort.Slice(contribList, func(i, j int) bool {
			return contribList[i].ChangeCount > contribList[j].ChangeCount
		})

		if len(contribList) > 0 {
			module.PrimaryOwner = contribList[0]
		}
		if len(contribList) > 1 {
			// Secondary owners are those with >10% ownership
			for _, c := range contribList[1:] {
				if c.OwnershipShare >= 0.1 {
					module.SecondaryOwners = append(module.SecondaryOwners, c)
				}
			}
		}
	}

	// Convert to slice
	var ownership []ModuleOwnership
	for _, m := range moduleStats {
		ownership = append(ownership, *m)
	}

	// Sort by total changes
	sort.Slice(ownership, func(i, j int) bool {
		return ownership[i].TotalChanges > ownership[j].TotalChanges
	})

	return ownership
}

// extractModulePath extracts the directory/module path from a file path
func extractModulePath(filePath string) string {
	// Get directory path up to 2 levels
	parts := splitPath(filePath)
	if len(parts) <= 1 {
		return ""
	}

	// Return first 2 directory levels
	depth := 2
	if len(parts)-1 < depth {
		depth = len(parts) - 1
	}

	return joinPath(parts[:depth])
}

// splitPath splits a path into components
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' || c == '\\' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// joinPath joins path components
func joinPath(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "/"
		}
		result += p
	}
	return result
}

// analyzeSymbol performs symbol-level analysis
func (a *FrequencyAnalyzer) analyzeSymbol(ctx context.Context, params ChangeFrequencyParams) (*SymbolChangeFrequency, error) {
	// Note: This requires symbol location data which should be passed in params
	// For now, return nil - full implementation would integrate with the index
	return nil, nil
}

// findMostActiveContributor identifies the top contributor across all commits
func (a *FrequencyAnalyzer) findMostActiveContributor(commits []CommitInfo) string {
	authorCounts := make(map[string]int)
	authorNames := make(map[string]string)

	for _, commit := range commits {
		authorCounts[commit.AuthorEmail]++
		authorNames[commit.AuthorEmail] = commit.AuthorName
	}

	maxCount := 0
	maxEmail := ""
	for email, count := range authorCounts {
		if count > maxCount {
			maxCount = count
			maxEmail = email
		}
	}

	if maxEmail != "" {
		return authorNames[maxEmail]
	}
	return ""
}

// AnalyzeFile performs focused analysis on a single file
func (a *FrequencyAnalyzer) AnalyzeFile(ctx context.Context, filePath string, window TimeWindow) (*FileChangeFrequency, error) {
	since := time.Now().Add(-TimeWindowToDuration(window))

	commits, err := a.provider.GetFileHistory(ctx, filePath, since)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return &FileChangeFrequency{
			FilePath: filePath,
			Metrics:  map[TimeWindow]*FrequencyMetrics{window: {}},
		}, nil
	}

	return a.provider.AggregateFileStats(commits, filePath, window), nil
}

// GetCollisionRisk checks collision risk for a specific file
func (a *FrequencyAnalyzer) GetCollisionRisk(ctx context.Context, filePath string) (*CollisionZone, error) {
	freq, err := a.AnalyzeFile(ctx, filePath, Window30Days)
	if err != nil {
		return nil, err
	}

	if freq == nil || len(freq.Contributors) < 2 {
		return nil, nil
	}

	// Count recent changes
	recentChanges := 0
	recentCutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, c := range freq.Contributors {
		if c.LastChangeAt.After(recentCutoff) {
			recentChanges += c.ChangeCount
		}
	}

	score := CalculateCollisionScore(freq.Contributors, recentChanges)
	severity := DetermineCollisionSeverity(score)

	return &CollisionZone{
		EntityType:     "file",
		Path:           filePath,
		Contributors:   freq.Contributors,
		CollisionScore: score,
		Severity:       severity,
		RecentChanges:  recentChanges,
		Recommendation: generateCollisionRecommendation(freq, severity),
	}, nil
}
