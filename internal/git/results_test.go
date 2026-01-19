package git

import (
	"testing"
)

func TestDetermineDuplicateSeverity(t *testing.T) {
	tests := []struct {
		name       string
		similarity float64
		lineCount  int
		expected   FindingSeverity
	}{
		{"exact match large", 1.0, 25, SeverityCritical},
		{"high similarity large", 0.95, 20, SeverityCritical},
		{"high similarity medium", 0.92, 15, SeverityWarning},
		{"medium similarity large", 0.85, 35, SeverityWarning},
		{"low similarity small", 0.85, 10, SeverityInfo},
		{"threshold similarity", 0.8, 5, SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineDuplicateSeverity(tt.similarity, tt.lineCount)
			if result != tt.expected {
				t.Errorf("DetermineDuplicateSeverity(%v, %v) = %v, want %v",
					tt.similarity, tt.lineCount, result, tt.expected)
			}
		})
	}
}

func TestDetermineNamingSeverity(t *testing.T) {
	tests := []struct {
		name       string
		issueType  NamingIssueType
		similarity float64
		expected   FindingSeverity
	}{
		{"case mismatch", NamingIssueCaseMismatch, 0.0, SeverityWarning},
		{"similar exists high similarity", NamingIssueSimilarExists, 0.95, SeverityWarning},
		{"similar exists low similarity", NamingIssueSimilarExists, 0.8, SeverityInfo},
		{"abbreviation", NamingIssueAbbreviation, 0.0, SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineNamingSeverity(tt.issueType, tt.similarity)
			if result != tt.expected {
				t.Errorf("DetermineNamingSeverity(%v, %v) = %v, want %v",
					tt.issueType, tt.similarity, result, tt.expected)
			}
		})
	}
}

func TestCalculateRiskScore(t *testing.T) {
	tests := []struct {
		name         string
		duplicates   []DuplicateFinding
		namingIssues []NamingFinding
		minExpected  float64
		maxExpected  float64
	}{
		{
			name:         "no issues",
			duplicates:   nil,
			namingIssues: nil,
			minExpected:  0.0,
			maxExpected:  0.0,
		},
		{
			name: "one critical duplicate",
			duplicates: []DuplicateFinding{
				{Severity: SeverityCritical},
			},
			namingIssues: nil,
			minExpected:  0.14,
			maxExpected:  0.16,
		},
		{
			name:       "one warning naming issue",
			duplicates: nil,
			namingIssues: []NamingFinding{
				{Severity: SeverityWarning},
			},
			minExpected: 0.04,
			maxExpected: 0.06,
		},
		{
			name: "mixed issues",
			duplicates: []DuplicateFinding{
				{Severity: SeverityWarning},
				{Severity: SeverityInfo},
			},
			namingIssues: []NamingFinding{
				{Severity: SeverityWarning},
			},
			minExpected: 0.15,
			maxExpected: 0.20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRiskScore(tt.duplicates, tt.namingIssues, nil)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("CalculateRiskScore() = %v, want between %v and %v",
					result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestSeverityValues(t *testing.T) {
	if SeverityCritical != "critical" {
		t.Errorf("SeverityCritical = %v, want %v", SeverityCritical, "critical")
	}
	if SeverityWarning != "warning" {
		t.Errorf("SeverityWarning = %v, want %v", SeverityWarning, "warning")
	}
	if SeverityInfo != "info" {
		t.Errorf("SeverityInfo = %v, want %v", SeverityInfo, "info")
	}
}

func TestGenerateTopRecommendation(t *testing.T) {
	tests := []struct {
		name         string
		duplicates   []DuplicateFinding
		namingIssues []NamingFinding
		expected     string
	}{
		{
			name:         "no issues",
			duplicates:   nil,
			namingIssues: nil,
			expected:     "",
		},
		{
			name: "critical duplicate",
			duplicates: []DuplicateFinding{
				{Severity: SeverityCritical, Suggestion: "Extract to shared function"},
			},
			namingIssues: nil,
			expected:     "Extract to shared function",
		},
		{
			name: "critical naming over warning duplicate",
			duplicates: []DuplicateFinding{
				{Severity: SeverityWarning, Suggestion: "Consider refactoring"},
			},
			namingIssues: []NamingFinding{
				{Severity: SeverityCritical, Suggestion: "Rename to match convention"},
			},
			expected: "Rename to match convention",
		},
		{
			name: "warning duplicate first when no critical",
			duplicates: []DuplicateFinding{
				{Severity: SeverityWarning, Suggestion: "First suggestion"},
				{Severity: SeverityInfo, Suggestion: "Second suggestion"},
			},
			namingIssues: nil,
			expected:     "First suggestion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTopRecommendation(tt.duplicates, tt.namingIssues, nil)
			if result != tt.expected {
				t.Errorf("GenerateTopRecommendation() = %v, want %v", result, tt.expected)
			}
		})
	}
}
