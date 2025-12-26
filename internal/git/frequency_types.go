// Package git provides git integration for analyzing code changes.
// This file contains types for change frequency analysis, collision detection,
// and anti-pattern identification.
package git

import (
	"math"
	"time"
)

// TimeWindow represents fixed analysis periods for change frequency
type TimeWindow string

const (
	// Window7Days analyzes the last 7 days of history
	Window7Days TimeWindow = "7d"
	// Window30Days analyzes the last 30 days of history
	Window30Days TimeWindow = "30d"
	// Window90Days analyzes the last 90 days of history
	Window90Days TimeWindow = "90d"
	// Window1Year analyzes the last year of history
	Window1Year TimeWindow = "1y"
)

// TimeWindowToDuration converts a TimeWindow to a time.Duration
func TimeWindowToDuration(tw TimeWindow) time.Duration {
	switch tw {
	case Window7Days:
		return 7 * 24 * time.Hour
	case Window30Days:
		return 30 * 24 * time.Hour
	case Window90Days:
		return 90 * 24 * time.Hour
	case Window1Year:
		return 365 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour // Default to 30 days
	}
}

// ParseTimeWindow parses a string to TimeWindow
func ParseTimeWindow(s string) TimeWindow {
	switch s {
	case "7d", "7days", "week":
		return Window7Days
	case "30d", "30days", "month":
		return Window30Days
	case "90d", "90days", "quarter":
		return Window90Days
	case "1y", "1year", "year", "365d":
		return Window1Year
	default:
		return Window30Days
	}
}

// FrequencyGranularity defines the level of analysis detail
type FrequencyGranularity string

const (
	// GranularityFile analyzes at file level (cheapest)
	GranularityFile FrequencyGranularity = "file"
	// GranularitySymbol analyzes at symbol level (moderate cost)
	GranularitySymbol FrequencyGranularity = "symbol"
)

// FrequencyFocus defines what aspects to analyze
type FrequencyFocus string

const (
	// FocusHotspots identifies frequently changing areas
	FocusHotspots FrequencyFocus = "hotspots"
	// FocusCollisions identifies multi-developer edit zones
	FocusCollisions FrequencyFocus = "collisions"
	// FocusPatterns identifies conflict-prone anti-patterns
	FocusPatterns FrequencyFocus = "patterns"
	// FocusOwnership analyzes code ownership
	FocusOwnership FrequencyFocus = "ownership"
	// FocusAll enables all analysis types
	FocusAll FrequencyFocus = "all"
)

// FrequencyMetrics provides change statistics for any granularity level
type FrequencyMetrics struct {
	// ChangeCount is the number of commits touching this entity
	ChangeCount int `json:"change_count"`

	// LinesAdded is the total lines added in the time window
	LinesAdded int `json:"lines_added"`

	// LinesDeleted is the total lines deleted in the time window
	LinesDeleted int `json:"lines_deleted"`

	// UniqueAuthors is the number of distinct contributors
	UniqueAuthors int `json:"unique_authors"`

	// FirstChangeAt is the earliest change in the window
	FirstChangeAt time.Time `json:"first_change_at,omitempty"`

	// LastChangeAt is the most recent change in the window
	LastChangeAt time.Time `json:"last_change_at,omitempty"`

	// ChangeRate is the number of changes per day
	ChangeRate float64 `json:"change_rate"`

	// VolatilityScore is a 0-1 score indicating how volatile this entity is
	// Higher values indicate more frequent, larger changes by multiple authors
	VolatilityScore float64 `json:"volatility_score"`
}

// CalculateVolatilityScore computes a volatility score from metrics
func CalculateVolatilityScore(changeCount, linesChanged, uniqueAuthors int, windowDays float64) float64 {
	if windowDays <= 0 {
		windowDays = 30
	}

	// Factor 1: Change frequency (40% weight)
	// Normalize: 1 change/day = 1.0
	changeRate := float64(changeCount) / windowDays
	normalizedChangeRate := math.Min(changeRate/1.0, 1.0)

	// Factor 2: Churn rate (40% weight)
	// Normalize: 100 lines changed/day = 1.0
	churnRate := float64(linesChanged) / windowDays
	normalizedChurnRate := math.Min(churnRate/100.0, 1.0)

	// Factor 3: Author diversity (20% weight)
	// Normalize: 5+ authors = 1.0
	normalizedAuthorDiv := math.Min(float64(uniqueAuthors)/5.0, 1.0)

	return 0.4*normalizedChangeRate + 0.4*normalizedChurnRate + 0.2*normalizedAuthorDiv
}

// ContributorActivity tracks an author's contributions to an entity
type ContributorActivity struct {
	// AuthorName is the git author name
	AuthorName string `json:"author_name"`

	// AuthorEmail is the git author email
	AuthorEmail string `json:"author_email"`

	// ChangeCount is the number of commits by this author
	ChangeCount int `json:"change_count"`

	// LinesAdded is total lines added by this author
	LinesAdded int `json:"lines_added"`

	// LinesDeleted is total lines deleted by this author
	LinesDeleted int `json:"lines_deleted"`

	// OwnershipShare is the fraction of changes by this author (0-1)
	OwnershipShare float64 `json:"ownership_share"`

	// LastChangeAt is when this author last modified the entity
	LastChangeAt time.Time `json:"last_change_at"`
}

// FileChangeFrequency tracks how often a file changes over time
type FileChangeFrequency struct {
	// FilePath is the path to the file
	FilePath string `json:"file_path"`

	// Metrics contains frequency data per time window
	Metrics map[TimeWindow]*FrequencyMetrics `json:"metrics"`

	// Contributors lists authors and their activity
	Contributors []ContributorActivity `json:"contributors"`

	// AntiPatterns lists detected conflict-prone patterns
	AntiPatterns []AntiPattern `json:"anti_patterns,omitempty"`

	// LineCount is the current line count of the file
	LineCount int `json:"line_count,omitempty"`
}

// SymbolChangeFrequency tracks how often a symbol changes
type SymbolChangeFrequency struct {
	// SymbolName is the name of the function/class/etc
	SymbolName string `json:"symbol_name"`

	// SymbolType is the kind of symbol (function, class, method, etc)
	SymbolType string `json:"symbol_type"`

	// FilePath is the file containing the symbol
	FilePath string `json:"file_path"`

	// StartLine is the current starting line of the symbol
	StartLine int `json:"start_line"`

	// EndLine is the current ending line of the symbol
	EndLine int `json:"end_line"`

	// Metrics contains frequency data per time window
	Metrics map[TimeWindow]*FrequencyMetrics `json:"metrics"`

	// Contributors lists authors and their activity on this symbol
	Contributors []ContributorActivity `json:"contributors"`
}

// CollisionSeverity is an alias for FindingSeverity to maintain consistent naming
// Uses the same values as FindingSeverity: "critical", "warning", "info"
type CollisionSeverity = FindingSeverity

// CollisionZone identifies areas where multiple developers are working
type CollisionZone struct {
	// EntityType is "file" or "symbol"
	EntityType string `json:"entity_type"`

	// Path is the file path
	Path string `json:"path"`

	// SymbolName is the symbol name (if EntityType is "symbol")
	SymbolName string `json:"symbol_name,omitempty"`

	// Contributors lists the developers working on this area
	Contributors []ContributorActivity `json:"contributors"`

	// CollisionScore is a 0-1 score indicating collision risk
	CollisionScore float64 `json:"collision_score"`

	// Severity categorizes the risk level
	Severity CollisionSeverity `json:"severity"`

	// Recommendation provides actionable guidance
	Recommendation string `json:"recommendation"`

	// RecentChanges is the number of changes in the last 7 days
	RecentChanges int `json:"recent_changes"`
}

// CalculateCollisionScore computes a collision risk score
func CalculateCollisionScore(contributors []ContributorActivity, recentChanges int) float64 {
	if len(contributors) < 2 {
		return 0.0
	}

	// Factor 1: Number of contributors (40% weight)
	// 5+ contributors = max score
	authorFactor := math.Min(float64(len(contributors)-1)/4.0, 1.0) * 0.4

	// Factor 2: Recent activity (40% weight)
	// 10+ recent changes = max score
	recencyFactor := math.Min(float64(recentChanges)/10.0, 1.0) * 0.4

	// Factor 3: Activity concentration (20% weight)
	// If activity is spread across many authors, higher risk
	if len(contributors) >= 2 {
		// Check if top 2 authors have similar ownership
		if contributors[0].OwnershipShare > 0 && len(contributors) >= 2 {
			ratio := contributors[1].OwnershipShare / contributors[0].OwnershipShare
			// Higher ratio = more even distribution = higher collision risk
			return authorFactor + recencyFactor + (ratio * 0.2)
		}
	}

	return authorFactor + recencyFactor
}

// DetermineCollisionSeverity maps a collision score to severity
func DetermineCollisionSeverity(score float64) FindingSeverity {
	switch {
	case score >= 0.7:
		return SeverityCritical
	case score >= 0.4:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

// AntiPatternType categorizes conflict-prone code patterns
type AntiPatternType string

const (
	// PatternRegistrationFunction is a large function with many sequential registrations
	PatternRegistrationFunction AntiPatternType = "registration_function"

	// PatternEnumAggregation is a file with many enum/const definitions
	PatternEnumAggregation AntiPatternType = "enum_aggregation"

	// PatternGodObject is a large file/class with high contributor count
	PatternGodObject AntiPatternType = "god_object"

	// PatternBarrelFile is an index/re-export file
	PatternBarrelFile AntiPatternType = "barrel_file"

	// PatternSwitchFactory is a large switch/case statement
	PatternSwitchFactory AntiPatternType = "switch_factory"

	// PatternConfigAggregation is a config struct/file with many fields
	PatternConfigAggregation AntiPatternType = "config_aggregation"
)

// AntiPatternSeverity indicates how problematic a pattern is
type AntiPatternSeverity string

const (
	// AntiPatternSeverityHigh indicates a pattern causing frequent conflicts
	AntiPatternSeverityHigh AntiPatternSeverity = "high"
	// AntiPatternSeverityMedium indicates a pattern that may cause conflicts
	AntiPatternSeverityMedium AntiPatternSeverity = "medium"
	// AntiPatternSeverityLow indicates a minor concern
	AntiPatternSeverityLow AntiPatternSeverity = "low"
)

// AntiPattern represents a detected conflict-prone code pattern
type AntiPattern struct {
	// Type categorizes the anti-pattern
	Type AntiPatternType `json:"type"`

	// Description explains what was detected
	Description string `json:"description"`

	// Location identifies where in the file (line range or function name)
	Location string `json:"location"`

	// Severity indicates how problematic this is
	Severity AntiPatternSeverity `json:"severity"`

	// Suggestion provides a decoupling recommendation
	Suggestion string `json:"suggestion"`

	// Metrics provides supporting data
	Metrics map[string]int `json:"metrics,omitempty"`
}

// ChangeFrequencyParams configures an on-demand frequency analysis
type ChangeFrequencyParams struct {
	// TimeWindow specifies the analysis period (default: "30d")
	TimeWindow string `json:"time_window,omitempty"`

	// Granularity specifies analysis level: "file" or "symbol" (default: "file")
	Granularity string `json:"granularity,omitempty"`

	// Focus specifies what to analyze: "hotspots", "collisions", "patterns", "ownership", "all"
	Focus []string `json:"focus,omitempty"`

	// FilePattern is a glob pattern to filter files (e.g., "internal/**/*.go")
	FilePattern string `json:"file_pattern,omitempty"`

	// FilePath is a specific file to analyze
	FilePath string `json:"file_path,omitempty"`

	// SymbolName is a specific symbol to analyze (requires FilePath)
	SymbolName string `json:"symbol_name,omitempty"`

	// MinChanges filters out entities with fewer changes (default: 2)
	MinChanges int `json:"min_changes,omitempty"`

	// MinContributors filters for collision detection (default: 2)
	MinContributors int `json:"min_contributors,omitempty"`

	// TopN limits the number of results (default: 50)
	TopN int `json:"top_n,omitempty"`

	// IncludePatterns is a list of glob patterns to include (overrides default exclusions)
	// If set, only files matching these patterns will be analyzed
	// Example: ["*.go", "*.py", "src/**/*"]
	IncludePatterns []string `json:"include_patterns,omitempty"`

	// ExcludePatterns is a list of additional glob patterns to exclude
	// These are added to the default exclusion list (docs, generated files, etc.)
	// Example: ["*_test.go", "mocks/*", "testdata/*"]
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// SkipDefaultExclusions disables the default file exclusions
	// (CHANGELOG, *.min.js, lock files, vendor/node_modules, etc.)
	// Set to true to analyze all files including generated/documentation
	SkipDefaultExclusions bool `json:"skip_default_exclusions,omitempty"`
}

// DefaultChangeFrequencyParams returns default parameters
func DefaultChangeFrequencyParams() ChangeFrequencyParams {
	return ChangeFrequencyParams{
		TimeWindow:      string(Window30Days),
		Granularity:     string(GranularityFile),
		Focus:           []string{string(FocusAll)},
		MinChanges:      2,
		MinContributors: 2,
		TopN:            50,
	}
}

// HasFocus checks if a specific focus area is enabled
func (p *ChangeFrequencyParams) HasFocus(focus FrequencyFocus) bool {
	if len(p.Focus) == 0 {
		return true // Default: all enabled
	}
	for _, f := range p.Focus {
		if FrequencyFocus(f) == focus || FrequencyFocus(f) == FocusAll {
			return true
		}
	}
	return false
}

// GetTimeWindow returns the parsed time window
func (p *ChangeFrequencyParams) GetTimeWindow() TimeWindow {
	if p.TimeWindow == "" {
		return Window30Days
	}
	return ParseTimeWindow(p.TimeWindow)
}

// GetGranularity returns the parsed granularity
func (p *ChangeFrequencyParams) GetGranularity() FrequencyGranularity {
	if p.Granularity == "" {
		return GranularityFile
	}
	return FrequencyGranularity(p.Granularity)
}

// ChangeFrequencyReport contains the analysis results
type ChangeFrequencyReport struct {
	// Summary provides high-level statistics
	Summary ChangeFrequencySummary `json:"summary"`

	// Hotspots lists the most frequently changed files/symbols
	Hotspots []FileChangeFrequency `json:"hotspots,omitempty"`

	// Collisions lists areas with multiple active developers
	Collisions []CollisionZone `json:"collisions,omitempty"`

	// AntiPatterns lists detected conflict-prone patterns
	AntiPatterns []AntiPattern `json:"anti_patterns,omitempty"`

	// SymbolDetails provides symbol-level analysis when requested
	SymbolDetails []SymbolChangeFrequency `json:"symbol_details,omitempty"`

	// Ownership provides module-level ownership information
	Ownership []ModuleOwnership `json:"ownership,omitempty"`

	// Metadata contains analysis context
	Metadata ChangeFrequencyMetadata `json:"metadata"`
}

// ChangeFrequencySummary provides high-level statistics
type ChangeFrequencySummary struct {
	// TotalFilesAnalyzed is the number of files examined
	TotalFilesAnalyzed int `json:"total_files_analyzed"`

	// TotalCommitsAnalyzed is the number of commits in the window
	TotalCommitsAnalyzed int `json:"total_commits_analyzed"`

	// HotspotsFound is the number of high-activity areas found
	HotspotsFound int `json:"hotspots_found"`

	// CollisionZones is the number of multi-developer areas found
	CollisionZones int `json:"collision_zones"`

	// AntiPatternsFound is the number of conflict-prone patterns detected
	AntiPatternsFound int `json:"anti_patterns_found"`

	// HighestChurn identifies the most volatile file
	HighestChurn string `json:"highest_churn,omitempty"`

	// MostActiveContributor identifies the top contributor
	MostActiveContributor string `json:"most_active_contributor,omitempty"`
}

// ModuleOwnership provides ownership information at directory/module level
type ModuleOwnership struct {
	// ModulePath is the directory path
	ModulePath string `json:"module_path"`

	// PrimaryOwner is the main contributor
	PrimaryOwner ContributorActivity `json:"primary_owner"`

	// SecondaryOwners are other significant contributors
	SecondaryOwners []ContributorActivity `json:"secondary_owners,omitempty"`

	// TotalChanges is the total change count in this module
	TotalChanges int `json:"total_changes"`

	// FileCount is the number of files in this module
	FileCount int `json:"file_count"`
}

// ChangeFrequencyMetadata contains analysis context
type ChangeFrequencyMetadata struct {
	// AnalyzedAt is when the analysis was performed
	AnalyzedAt time.Time `json:"analyzed_at"`

	// TimeWindow is the analysis period
	TimeWindow string `json:"time_window"`

	// WindowStart is the start of the analysis period
	WindowStart time.Time `json:"window_start"`

	// WindowEnd is the end of the analysis period
	WindowEnd time.Time `json:"window_end"`

	// CommitRange describes the analyzed commits
	CommitRange string `json:"commit_range,omitempty"`

	// ComputeTimeMs is how long the analysis took
	ComputeTimeMs int64 `json:"compute_time_ms"`

	// FromCache indicates if results were served from cache
	FromCache bool `json:"from_cache"`
}

// CommitInfo represents a parsed git commit
type CommitInfo struct {
	// Hash is the commit SHA
	Hash string `json:"hash"`

	// AuthorName is the commit author name
	AuthorName string `json:"author_name"`

	// AuthorEmail is the commit author email
	AuthorEmail string `json:"author_email"`

	// Timestamp is when the commit was made
	Timestamp time.Time `json:"timestamp"`

	// Message is the commit message (first line)
	Message string `json:"message,omitempty"`

	// FileChanges lists files changed in this commit
	FileChanges []FileChange `json:"file_changes,omitempty"`
}

// FileChange represents a file modification in a commit
type FileChange struct {
	// Path is the file path
	Path string `json:"path"`

	// OldPath is the previous path (for renames)
	OldPath string `json:"old_path,omitempty"`

	// LinesAdded is the number of lines added
	LinesAdded int `json:"lines_added"`

	// LinesDeleted is the number of lines deleted
	LinesDeleted int `json:"lines_deleted"`

	// Status is the change type (A/M/D/R)
	Status string `json:"status"`
}
