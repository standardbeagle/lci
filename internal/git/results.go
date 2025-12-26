package git

import "time"

// AnalysisReport is the complete output of git change analysis
type AnalysisReport struct {
	// Summary provides high-level statistics
	Summary ReportSummary `json:"summary"`

	// Duplicates contains duplicate code findings
	Duplicates []DuplicateFinding `json:"duplicates,omitempty"`

	// NamingIssues contains naming consistency findings
	NamingIssues []NamingFinding `json:"naming_issues,omitempty"`

	// Metadata provides analysis context
	Metadata ReportMetadata `json:"metadata"`
}

// ReportSummary provides high-level statistics about the analysis
type ReportSummary struct {
	// FilesChanged is the number of files with changes
	FilesChanged int `json:"files_changed"`

	// SymbolsAdded is the count of new symbols
	SymbolsAdded int `json:"symbols_added"`

	// SymbolsModified is the count of modified symbols
	SymbolsModified int `json:"symbols_modified"`

	// SymbolsDeleted is the count of deleted symbols
	SymbolsDeleted int `json:"symbols_deleted"`

	// DuplicatesFound is the count of duplicate findings
	DuplicatesFound int `json:"duplicates_found"`

	// NamingIssuesFound is the count of naming issues
	NamingIssuesFound int `json:"naming_issues_found"`

	// RiskScore indicates overall risk (0.0 = safe, 1.0 = risky)
	RiskScore float64 `json:"risk_score"`

	// TopRecommendation is the highest priority suggestion
	TopRecommendation string `json:"top_recommendation,omitempty"`
}

// FindingSeverity indicates the importance of a finding
type FindingSeverity string

const (
	// SeverityCritical indicates a must-fix issue
	SeverityCritical FindingSeverity = "critical"
	// SeverityWarning indicates a should-consider issue
	SeverityWarning FindingSeverity = "warning"
	// SeverityInfo indicates a nice-to-know item
	SeverityInfo FindingSeverity = "info"
)

// DuplicateFinding represents detected duplicate code
type DuplicateFinding struct {
	// Severity indicates the importance
	Severity FindingSeverity `json:"severity"`

	// Description provides a human-readable summary
	Description string `json:"description"`

	// NewCode is the location in the changed code
	NewCode CodeLocation `json:"new_code"`

	// ExistingCode is the location in the existing codebase
	ExistingCode CodeLocation `json:"existing_code"`

	// Similarity is the similarity score (0.0-1.0)
	Similarity float64 `json:"similarity"`

	// Type indicates exact, structural, or semantic duplicate
	Type string `json:"type"`

	// Suggestion provides actionable advice
	Suggestion string `json:"suggestion"`
}

// NamingFinding represents a naming consistency issue
type NamingFinding struct {
	// Severity indicates the importance
	Severity FindingSeverity `json:"severity"`

	// Description provides a human-readable summary
	Description string `json:"description"`

	// NewSymbol is the symbol with the naming issue
	NewSymbol SymbolInfo `json:"new_symbol"`

	// SimilarNames are existing symbols with similar names
	SimilarNames []SymbolInfo `json:"similar_names"`

	// IssueType categorizes the naming issue
	IssueType NamingIssueType `json:"issue_type"`

	// Issue provides details about the problem
	Issue string `json:"issue"`

	// Suggestion provides the recommended name
	Suggestion string `json:"suggestion"`
}

// ReportMetadata provides context about the analysis
type ReportMetadata struct {
	// BaseRef is the base reference used
	BaseRef string `json:"base_ref"`

	// TargetRef is the target reference used
	TargetRef string `json:"target_ref"`

	// Scope is the analysis scope
	Scope AnalysisScope `json:"scope"`

	// AnalyzedAt is when the analysis was performed
	AnalyzedAt time.Time `json:"analyzed_at"`

	// AnalysisTimeMs is how long the analysis took
	AnalysisTimeMs int64 `json:"analysis_time_ms"`

	// Truncated indicates if findings were limited
	Truncated bool `json:"truncated,omitempty"`

	// TotalDuplicates is the actual count before truncation
	TotalDuplicates int `json:"total_duplicates,omitempty"`

	// TotalNamingIssues is the actual count before truncation
	TotalNamingIssues int `json:"total_naming_issues,omitempty"`
}

// CalculateRiskScore computes an overall risk score based on findings
func CalculateRiskScore(duplicates []DuplicateFinding, namingIssues []NamingFinding) float64 {
	risk := 0.0

	// Weight duplicates by severity
	for _, dup := range duplicates {
		switch dup.Severity {
		case SeverityCritical:
			risk += 0.15
		case SeverityWarning:
			risk += 0.08
		case SeverityInfo:
			risk += 0.03
		}
	}

	// Weight naming issues by severity
	for _, issue := range namingIssues {
		switch issue.Severity {
		case SeverityCritical:
			risk += 0.10
		case SeverityWarning:
			risk += 0.05
		case SeverityInfo:
			risk += 0.02
		}
	}

	// Cap at 1.0
	if risk > 1.0 {
		risk = 1.0
	}

	return risk
}

// DetermineDuplicateSeverity determines severity based on similarity and size
func DetermineDuplicateSeverity(similarity float64, lineCount int) FindingSeverity {
	switch {
	case similarity >= 0.95 && lineCount >= 20:
		return SeverityCritical
	case similarity >= 0.90 || lineCount >= 30:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

// DetermineNamingSeverity determines severity based on issue type and context
func DetermineNamingSeverity(issueType NamingIssueType, similarity float64) FindingSeverity {
	switch issueType {
	case NamingIssueSimilarExists:
		if similarity >= 0.9 {
			return SeverityWarning
		}
		return SeverityInfo
	case NamingIssueCaseMismatch:
		return SeverityWarning
	case NamingIssueAbbreviation:
		return SeverityInfo
	default:
		return SeverityInfo
	}
}

// GenerateTopRecommendation creates the highest-priority recommendation
func GenerateTopRecommendation(duplicates []DuplicateFinding, namingIssues []NamingFinding) string {
	// Check for critical duplicates first
	for _, dup := range duplicates {
		if dup.Severity == SeverityCritical {
			return dup.Suggestion
		}
	}

	// Then critical naming issues
	for _, issue := range namingIssues {
		if issue.Severity == SeverityCritical {
			return issue.Suggestion
		}
	}

	// Then any duplicate
	if len(duplicates) > 0 {
		return duplicates[0].Suggestion
	}

	// Then any naming issue
	if len(namingIssues) > 0 {
		return namingIssues[0].Suggestion
	}

	return ""
}
