package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FilesParams defines parameters for the files search tool
type FilesParams struct {
	// Pattern is the file/path pattern to search for (supports fuzzy matching)
	Pattern string `json:"pattern"`

	// Max limits the number of results (default: 50, max: 200)
	Max int `json:"max,omitempty"`

	// Filter filters by file type or glob pattern (e.g., "go", "*.ts", "src/**/*.js")
	Filter string `json:"filter,omitempty"`

	// Flags for search behavior
	Flags string `json:"flags,omitempty"` // "ci" (case-insensitive), "exact" (exact match only)

	// IncludeHidden includes hidden files/directories (default: false)
	IncludeHidden bool `json:"include_hidden,omitempty"`

	// Directory to search within (relative to project root)
	Directory string `json:"directory,omitempty"`
}

// FileSearchResult represents a single file match result
type FileSearchResult struct {
	Path      string  `json:"path"`                 // File path relative to project root
	Score     float64 `json:"score"`                // Match score (0.0-1.0)
	MatchType string  `json:"match_type,omitempty"` // "exact", "fuzzy", "substring", "path_component"
	FileID    int     `json:"file_id,omitempty"`    // File ID in index
}

// FilesResponse is the response for the files search tool
type FilesResponse struct {
	Results      []FileSearchResult `json:"results"`
	TotalMatches int                `json:"total_matches"`
	SearchTime   string             `json:"search_time,omitempty"`
	Pattern      string             `json:"pattern"`
}

// handleFiles handles the files search tool
// @lci:labels[mcp-tool-handler,file-search,fuzzy-matching,path-search]
// @lci:category[mcp-api]
func (s *Server) handleFiles(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	var params FilesParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createSmartErrorResponse("files", fmt.Errorf("invalid parameters: %w", err), nil)
	}

	// Validate pattern
	if params.Pattern == "" {
		return createSmartErrorResponse("files", errors.New("pattern is required"), map[string]interface{}{
			"example": map[string]string{
				"search_by_name": `{"pattern": "UserController"}`,
				"search_by_path": `{"pattern": "src/components"}`,
				"fuzzy_search":   `{"pattern": "usrctrl"}`,
				"with_filter":    `{"pattern": "handler", "filter": "*.go"}`,
				"in_directory":   `{"pattern": "test", "directory": "internal"}`,
			},
		})
	}

	// Check if index is available
	if available, err := s.checkIndexAvailability(); err != nil {
		return createSmartErrorResponse("files", err, nil)
	} else if !available {
		return nil, errors.New("files search cannot proceed: index is not available")
	}

	// Set defaults
	if params.Max == 0 {
		params.Max = 50
	}
	if params.Max > 200 {
		params.Max = 200
	}

	// Get all files from index
	allFiles := s.goroutineIndex.GetAllFiles()
	if len(allFiles) == 0 {
		return createSmartErrorResponse("files", errors.New("no files in index"), map[string]interface{}{
			"suggestion": "Wait for indexing to complete or check if files were excluded by configuration",
		})
	}

	// Parse flags
	caseInsensitive := strings.Contains(params.Flags, "ci")
	exactOnly := strings.Contains(params.Flags, "exact")

	// Filter files by directory if specified
	filteredFiles := allFiles
	if params.Directory != "" {
		normalizedDir := filepath.Clean(params.Directory)
		filteredFiles = make([]*types.FileInfo, 0)
		for _, file := range allFiles {
			if strings.HasPrefix(file.Path, normalizedDir+"/") || strings.HasPrefix(file.Path, normalizedDir+"\\") {
				filteredFiles = append(filteredFiles, file)
			}
		}
	}

	// Apply filter (file type or glob pattern)
	if params.Filter != "" {
		filteredFiles = s.applyFileFilter(filteredFiles, params.Filter)
	}

	// Apply hidden file filter
	if !params.IncludeHidden {
		filteredFiles = s.filterHiddenFiles(filteredFiles)
	}

	// Aggressive pattern matching: split multi-word patterns and search for each
	// This ensures "user controller" finds files matching "user" OR "controller"
	// with boost for files matching multiple words
	patterns := expandFileSearchPatterns(params.Pattern)

	// Search with all patterns, tracking coverage for scoring boost
	matches := s.matchFilePathsWithCoverage(patterns, filteredFiles, caseInsensitive, exactOnly)

	// Sort by score (descending)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// Limit results
	if len(matches) > params.Max {
		matches = matches[:params.Max]
	}

	// Create response
	response := &FilesResponse{
		Results:      matches,
		TotalMatches: len(matches),
		Pattern:      params.Pattern,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		return createErrorResponse("files", fmt.Errorf("failed to marshal response: %w", err))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil
}

// matchFilePaths performs fuzzy matching on file paths
func (s *Server) matchFilePaths(pattern string, files []*types.FileInfo, caseInsensitive bool, exactOnly bool) []FileSearchResult {
	results := make([]FileSearchResult, 0)

	// Normalize pattern for matching
	normalizedPattern := pattern
	if caseInsensitive {
		normalizedPattern = strings.ToLower(pattern)
	}

	// Create fuzzy matcher for path components
	splitter := semantic.NewNameSplitter()
	fuzzer := semantic.NewFuzzyMatcher(true, 0.7, "levenshtein") // enabled, threshold 0.7, levenshtein algorithm
	phraseMatcher := semantic.NewPhraseMatcher(splitter, fuzzer)

	for _, file := range files {
		filePath := file.Path
		if caseInsensitive {
			filePath = strings.ToLower(file.Path)
		}

		// Extract filename without extension for better matching
		filename := filepath.Base(file.Path)
		filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))

		var score float64
		var matchType string

		// 1. Exact full path match (highest priority)
		if filePath == normalizedPattern {
			score = 1.0
			matchType = "exact"
		} else if !exactOnly {
			// 2. Exact filename match
			if caseInsensitive {
				if strings.ToLower(filename) == normalizedPattern {
					score = 0.95
					matchType = "exact_filename"
				} else if strings.ToLower(filenameNoExt) == normalizedPattern {
					score = 0.93
					matchType = "exact_filename_noext"
				}
			} else {
				if filename == normalizedPattern {
					score = 0.95
					matchType = "exact_filename"
				} else if filenameNoExt == normalizedPattern {
					score = 0.93
					matchType = "exact_filename_noext"
				}
			}

			// 3. Contains pattern (substring match)
			if score == 0 && strings.Contains(filePath, normalizedPattern) {
				// Score based on how early in the path the match appears
				index := strings.Index(filePath, normalizedPattern)
				score = 0.8 - (float64(index) / float64(len(filePath)) * 0.2)
				matchType = "substring"
			}

			// 4. Fuzzy match on filename
			if score == 0 {
				fuzzyResult := phraseMatcher.Match(normalizedPattern, filenameNoExt)
				if fuzzyResult.Matched {
					score = fuzzyResult.Score * 0.7 // Scale down fuzzy matches
					matchType = "fuzzy"
				}
			}

			// 5. Path component match (pattern matches any directory/file name in path)
			if score == 0 {
				components := strings.Split(file.Path, string(filepath.Separator))
				for _, component := range components {
					componentNormalized := component
					if caseInsensitive {
						componentNormalized = strings.ToLower(component)
					}
					if strings.Contains(componentNormalized, normalizedPattern) {
						score = 0.6
						matchType = "path_component"
						break
					}
				}
			}
		}

		// Only include matches with score > 0
		if score > 0 {
			result := FileSearchResult{
				Path:      file.Path,
				Score:     score,
				MatchType: matchType,
				FileID:    int(file.ID),
			}
			results = append(results, result)
		}
	}

	return results
}

// applyFileFilter filters files by type or glob pattern
func (s *Server) applyFileFilter(files []*types.FileInfo, filter string) []*types.FileInfo {
	filtered := make([]*types.FileInfo, 0)

	// Check if filter is a language name (e.g., "go", "python", "javascript")
	isLanguageFilter := !strings.Contains(filter, "*") && !strings.Contains(filter, ".")

	for _, file := range files {
		if isLanguageFilter {
			// Match by file extension (since FileInfo doesn't have Language field)
			ext := filepath.Ext(file.Path)
			if ext != "" {
				ext = ext[1:] // Remove leading dot
				if strings.EqualFold(ext, filter) {
					filtered = append(filtered, file)
				}
			}
		} else {
			// Match by glob pattern
			matched, err := filepath.Match(filter, filepath.Base(file.Path))
			if err == nil && matched {
				filtered = append(filtered, file)
			}
			// Also try matching full path for patterns like "src/**/*.js"
			if !matched {
				matched, err := filepath.Match(filter, file.Path)
				if err == nil && matched {
					filtered = append(filtered, file)
				}
			}
		}
	}

	return filtered
}

// expandFileSearchPatterns expands a pattern into multiple search terms
// Multi-word patterns are split to enable aggressive matching
// e.g., "user controller" → ["user controller", "user", "controller"]
func expandFileSearchPatterns(pattern string) []string {
	patterns := []string{pattern} // Always include original pattern first

	// Split on spaces for multi-word queries
	words := strings.Fields(pattern)
	if len(words) > 1 {
		for _, word := range words {
			if len(word) > 2 { // Skip very short words
				patterns = append(patterns, word)
			}
		}
	}

	return patterns
}

// matchFilePathsWithCoverage performs matching with all patterns and boosts scores for multi-pattern matches
func (s *Server) matchFilePathsWithCoverage(patterns []string, files []*types.FileInfo, caseInsensitive bool, exactOnly bool) []FileSearchResult {
	// Track best match per file and pattern coverage
	type fileMatch struct {
		result       FileSearchResult
		patternCount int
	}
	fileMatches := make(map[string]*fileMatch) // keyed by file path

	for _, pattern := range patterns {
		matches := s.matchFilePaths(pattern, files, caseInsensitive, exactOnly)
		for _, match := range matches {
			if existing, exists := fileMatches[match.Path]; exists {
				// File already matched by another pattern - boost coverage
				existing.patternCount++
				// Keep the higher score
				if match.Score > existing.result.Score {
					existing.result = match
				}
			} else {
				fileMatches[match.Path] = &fileMatch{
					result:       match,
					patternCount: 1,
				}
			}
		}
	}

	// Apply coverage boost and collect results
	results := make([]FileSearchResult, 0, len(fileMatches))
	for _, fm := range fileMatches {
		// Boost score for files matching multiple patterns (15% per additional pattern, max 50%)
		if fm.patternCount > 1 {
			boost := float64(fm.patternCount-1) * 0.15
			if boost > 0.5 {
				boost = 0.5
			}
			fm.result.Score *= (1.0 + boost)
			// Cap at 1.0 for normalized scores
			if fm.result.Score > 1.0 {
				fm.result.Score = 1.0
			}
		}
		results = append(results, fm.result)
	}

	return results
}

// filterHiddenFiles removes hidden files and directories from the list
func (s *Server) filterHiddenFiles(files []*types.FileInfo) []*types.FileInfo {
	filtered := make([]*types.FileInfo, 0)

	for _, file := range files {
		// Check if any path component starts with a dot
		isHidden := false
		components := strings.Split(file.Path, string(filepath.Separator))
		for _, component := range components {
			if strings.HasPrefix(component, ".") && component != "." && component != ".." {
				isHidden = true
				break
			}
		}

		if !isHidden {
			filtered = append(filtered, file)
		}
	}

	return filtered
}
