package core

import (
	"strings"
	"sync"
	"time"
	
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// UnifiedSearchResult combines regular and assembly search results
type UnifiedSearchResult struct {
	RegularResults   []searchtypes.Result
	AssemblyResults  []AssemblyResult
	TotalTime        time.Duration
	AssemblyTriggered bool
}

// IntegratedSearch performs both regular and assembly search when appropriate
type IntegratedSearch struct {
	regularEngine   SearchEngine
	assemblyEngine  *AssemblySearchEngine
	enableAssembly  bool
}

// NewIntegratedSearch creates a search that combines regular and assembly search
func NewIntegratedSearch(regular SearchEngine, assembly *AssemblySearchEngine) *IntegratedSearch {
	return &IntegratedSearch{
		regularEngine:  regular,
		assemblyEngine: assembly,
		enableAssembly: true,
	}
}

// Search performs integrated search with automatic assembly search detection
func (is *IntegratedSearch) Search(pattern string, options types.SearchOptions) (*UnifiedSearchResult, error) {
	start := time.Now()
	result := &UnifiedSearchResult{}
	
	// Determine if assembly search should run
	shouldRunAssembly := is.enableAssembly && is.isAssemblyCandidate(pattern)
	result.AssemblyTriggered = shouldRunAssembly
	
	// Run searches in parallel
	var wg sync.WaitGroup
	var regularErr, assemblyErr error
	
	// Always run regular search
	wg.Add(1)
	go func() {
		defer wg.Done()
		result.RegularResults, regularErr = is.regularEngine.Search(pattern, options)
	}()
	
	// Conditionally run assembly search
	if shouldRunAssembly {
		wg.Add(1)
		go func() {
			defer wg.Done()
			params := AssemblySearchParams{
				Pattern:           pattern,
				MinCoverage:       0.6, // Lower threshold for integrated search
				MinFragmentLength: 3,
				MaxResults:        20,
			}
			result.AssemblyResults, assemblyErr = is.assemblyEngine.Search(params)
		}()
	}
	
	wg.Wait()
	
	// Return first error if any
	if regularErr != nil {
		return nil, regularErr
	}
	if assemblyErr != nil {
		return nil, assemblyErr
	}
	
	result.TotalTime = time.Since(start)
	return result, nil
}

// isAssemblyCandidate determines if a pattern should trigger assembly search
func (is *IntegratedSearch) isAssemblyCandidate(pattern string) bool {
	// Don't run for very short patterns
	if len(pattern) < 8 {
		return false
	}
	
	// Strong indicators for assembly search
	if strings.Contains(pattern, "<") && strings.Contains(pattern, ">") {
		return true // HTML/JSX content
	}
	
	// Error messages and log patterns
	errorPrefixes := []string{"Error:", "Warning:", "Failed", "Invalid", "Missing"}
	for _, prefix := range errorPrefixes {
		if strings.HasPrefix(pattern, prefix) {
			return true
		}
	}
	
	// Multi-segment patterns (likely concatenated)
	wordCount := len(strings.Fields(pattern))
	if wordCount >= 4 {
		return true // Phrases with multiple words
	}
	
	// Path-like patterns
	if strings.Count(pattern, "/") >= 3 {
		return true // API paths, URLs
	}
	
	// SQL-like patterns
	sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "FROM", "WHERE"}
	upperPattern := strings.ToUpper(pattern)
	for _, keyword := range sqlKeywords {
		if strings.Contains(upperPattern, keyword) {
			return true
		}
	}
	
	return false
}

// SearchWithFallback runs regular search first, then assembly if results are poor
func (is *IntegratedSearch) SearchWithFallback(pattern string, options types.SearchOptions) (*UnifiedSearchResult, error) {
	start := time.Now()
	
	// Run regular search first
	regularResults, err := is.regularEngine.Search(pattern, options)
	if err != nil {
		return nil, err
	}
	
	result := &UnifiedSearchResult{
		RegularResults: regularResults,
		TotalTime:      time.Since(start),
	}
	
	// If regular search found good results, return them
	if len(regularResults) >= 5 {
		return result, nil
	}
	
	// Poor results - try assembly search as fallback
	if is.enableAssembly {
		params := AssemblySearchParams{
			Pattern:           pattern,
			MinCoverage:       0.5, // Lower threshold for fallback
			MinFragmentLength: 3,
			MaxResults:        30,
		}
		
		result.AssemblyResults, err = is.assemblyEngine.Search(params)
		if err == nil {
			result.AssemblyTriggered = true
			result.TotalTime = time.Since(start)
		}
	}
	
	return result, nil
}

// FormatUnifiedResults creates a human-readable format for combined results
func FormatUnifiedResults(result *UnifiedSearchResult) string {
	var output strings.Builder
	
	// Regular results
	if len(result.RegularResults) > 0 {
		output.WriteString("=== Direct Matches ===\n")
		for i, r := range result.RegularResults {
			if i >= 10 {
				output.WriteString("... and ")
				output.WriteString(string(rune(len(result.RegularResults) - 10)))
				output.WriteString(" more\n")
				break
			}
			output.WriteString(formatRegularResult(r))
		}
	}
	
	// Assembly results  
	if len(result.AssemblyResults) > 0 {
		output.WriteString("\n=== Possible String Assembly ===\n")
		for i, a := range result.AssemblyResults {
			if i >= 5 {
				output.WriteString("... and ")
				output.WriteString(string(rune(len(result.AssemblyResults) - 5)))
				output.WriteString(" more assembly patterns\n")
				break
			}
			output.WriteString(formatAssemblyResult(a))
		}
	}
	
	if len(result.RegularResults) == 0 && len(result.AssemblyResults) == 0 {
		output.WriteString("No matches found\n")
		if !result.AssemblyTriggered {
			output.WriteString("(Assembly search not triggered for this pattern)\n")
		}
	}
	
	output.WriteString("\nSearch completed in ")
	output.WriteString(result.TotalTime.String())
	
	return output.String()
}

func formatRegularResult(r searchtypes.Result) string {
	// Simple format for regular results
	return r.Path + ":" + string(rune(r.Line)) + " " + r.Match + "\n"
}

func formatAssemblyResult(a AssemblyResult) string {
	// Format assembly results with coverage info
	return "Group " + string(rune(a.GroupID)) + " (coverage: " + 
	       string(rune(int(a.Coverage*100))) + "%) - " + 
	       string(rune(len(a.Fragments))) + " fragments found\n"
}

// SearchEngine interface for regular search
type SearchEngine interface {
	Search(pattern string, options types.SearchOptions) ([]searchtypes.Result, error)
}