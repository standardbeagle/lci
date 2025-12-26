package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// FileSearchEngine provides file path search capabilities using only indexes and content store
type FileSearchEngine struct {
	mu        sync.RWMutex
	pathIndex *types.FilePathIndex
}

// NewFileSearchEngine creates a new file search engine
func NewFileSearchEngine() *FileSearchEngine {
	return &FileSearchEngine{
		pathIndex: types.NewFilePathIndex(),
	}
}

// Clear resets the file search engine's path index
func (fse *FileSearchEngine) Clear() {
	fse.mu.Lock()
	defer fse.mu.Unlock()
	fse.pathIndex = types.NewFilePathIndex()
}

// IndexFile adds a file path to the search index
func (fse *FileSearchEngine) IndexFile(fileID types.FileID, filePath string) {
	fse.mu.Lock()
	defer fse.mu.Unlock()

	// Clean and normalize the path
	cleanPath := filepath.Clean(filePath)
	fse.pathIndex.FullPaths[fileID] = cleanPath

	// Index path segments by depth
	segments := strings.Split(cleanPath, string(filepath.Separator))
	for depth, segment := range segments {
		if segment == "" {
			continue // Skip empty segments
		}

		if fse.pathIndex.PathSegments[depth] == nil {
			fse.pathIndex.PathSegments[depth] = make(map[string][]types.FileID)
		}

		fse.pathIndex.PathSegments[depth][segment] = append(
			fse.pathIndex.PathSegments[depth][segment],
			fileID,
		)
	}

	// Index file extension
	ext := filepath.Ext(cleanPath)
	if ext != "" {
		fse.pathIndex.Extensions[ext] = append(fse.pathIndex.Extensions[ext], fileID)
	}

	// Index directory
	dir := filepath.Dir(cleanPath)
	if dir != "." {
		fse.pathIndex.Directories[dir] = append(fse.pathIndex.Directories[dir], fileID)
	}

	// Index base filename
	baseName := filepath.Base(cleanPath)
	fse.pathIndex.BaseNames[baseName] = append(fse.pathIndex.BaseNames[baseName], fileID)

	fse.pathIndex.TotalFiles++
}

// SearchFiles searches for files matching the given options using only the index
func (fse *FileSearchEngine) SearchFiles(options types.FileSearchOptions) ([]types.FileSearchResult, error) {
	fse.mu.RLock()
	defer fse.mu.RUnlock()

	startTime := time.Now()

	// Set defaults
	if options.MaxResults == 0 {
		options.MaxResults = 100
	}
	if options.Type == "" {
		options.Type = "glob"
	}

	var results []types.FileSearchResult
	var err error

	switch options.Type {
	case "glob":
		results, err = fse.searchByGlob(options)
	case "regex":
		results, err = fse.searchByRegex(options)
	case "exact":
		results, err = fse.searchByExact(options)
	default:
		return nil, fmt.Errorf("unsupported search type: %s", options.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("file search failed: %w", err)
	}

	// Apply exclusion patterns
	if len(options.Exclude) > 0 {
		results = fse.applyExclusions(results, options.Exclude)
	}

	// Apply extension filtering
	if len(options.Extensions) > 0 {
		results = fse.filterByExtensions(results, options.Extensions)
	}

	// Apply directory filtering
	if len(options.Directories) > 0 {
		results = fse.filterByDirectories(results, options.Directories)
	}

	// Limit results
	if len(results) > options.MaxResults {
		results = results[:options.MaxResults]
	}

	// Sort results by path for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	searchTime := time.Since(startTime)
	_ = searchTime // For future metrics/logging

	return results, nil
}

// searchByGlob searches files using glob pattern matching
func (fse *FileSearchEngine) searchByGlob(options types.FileSearchOptions) ([]types.FileSearchResult, error) {
	pattern := options.Pattern
	
	// Handle simple cases first
	if !strings.ContainsAny(pattern, "*?[]{}") {
		// No glob metacharacters - treat as exact match
		return fse.searchByExact(types.FileSearchOptions{
			Pattern:    pattern,
			Type:       "exact",
			MaxResults: options.MaxResults,
		})
	}
	
	var candidateFiles []types.FileID
	
	// Strategy: Use path segments to narrow down candidates efficiently
	segments := strings.Split(pattern, string(filepath.Separator))
	
	// Find the first non-wildcard segment to start filtering
	firstConcreteSegment := -1
	for i, segment := range segments {
		if !strings.ContainsAny(segment, "*?[]{}") && segment != "" {
			firstConcreteSegment = i
			break
		}
	}
	
	if firstConcreteSegment >= 0 {
		// Use the concrete segment to get candidate files
		segment := segments[firstConcreteSegment]
		if fileIDs, exists := fse.pathIndex.PathSegments[firstConcreteSegment][segment]; exists {
			candidateFiles = fileIDs
		}
	} else {
		// All segments are wildcards - need to check all files
		candidateFiles = make([]types.FileID, 0, fse.pathIndex.TotalFiles)
		for fileID := range fse.pathIndex.FullPaths {
			candidateFiles = append(candidateFiles, fileID)
		}
	}
	
	// Filter candidates using filepath.Match
	var results []types.FileSearchResult
	for _, fileID := range candidateFiles {
		filePath := fse.pathIndex.FullPaths[fileID]

		// For patterns without path separators, match against filename only
		// For patterns with path separators, match against full path
		var matchTarget string
		if strings.ContainsRune(pattern, filepath.Separator) || strings.Contains(pattern, "/") {
			matchTarget = filePath
		} else {
			matchTarget = filepath.Base(filePath)
		}

		matched, err := filepath.Match(pattern, matchTarget)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern '%s': %w", pattern, err)
		}

		if matched {
			results = append(results, fse.createResult(fileID, filePath, "glob pattern match"))
		}
	}
	
	return results, nil
}

// searchByRegex searches files using regular expression matching
func (fse *FileSearchEngine) searchByRegex(options types.FileSearchOptions) ([]types.FileSearchResult, error) {
	regex, err := regexp.Compile(options.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern '%s': %w", options.Pattern, err)
	}
	
	var results []types.FileSearchResult
	for fileID, filePath := range fse.pathIndex.FullPaths {
		if regex.MatchString(filePath) {
			results = append(results, fse.createResult(fileID, filePath, "regex pattern match"))
		}
	}
	
	return results, nil
}

// searchByExact searches for files with exact path matches
func (fse *FileSearchEngine) searchByExact(options types.FileSearchOptions) ([]types.FileSearchResult, error) {
	pattern := options.Pattern
	
	var results []types.FileSearchResult
	for fileID, filePath := range fse.pathIndex.FullPaths {
		if filePath == pattern {
			results = append(results, fse.createResult(fileID, filePath, "exact path match"))
		}
	}
	
	return results, nil
}

// createResult creates a FileSearchResult from file information
func (fse *FileSearchEngine) createResult(fileID types.FileID, filePath string, matchReason string) types.FileSearchResult {
	extension := filepath.Ext(filePath)
	return types.FileSearchResult{
		FileID:      fileID,
		Path:        filePath,
		Directory:   filepath.Dir(filePath),
		BaseName:    filepath.Base(filePath),
		Extension:   extension,
		Language:    fse.getLanguageFromExtension(extension),
		MatchReason: matchReason,
	}
}

// getLanguageFromExtension determines the programming language from file extension
func (fse *FileSearchEngine) getLanguageFromExtension(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "c"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return "text"
	}
}

// applyExclusions filters out results matching exclusion patterns
func (fse *FileSearchEngine) applyExclusions(results []types.FileSearchResult, exclusions []string) []types.FileSearchResult {
	var filtered []types.FileSearchResult
	
	for _, result := range results {
		excluded := false
		for _, exclusion := range exclusions {
			matched, err := filepath.Match(exclusion, result.Path)
			if err == nil && matched {
				excluded = true
				break
			}
		}
		
		if !excluded {
			filtered = append(filtered, result)
		}
	}
	
	return filtered
}

// filterByExtensions filters results to only include specified extensions
func (fse *FileSearchEngine) filterByExtensions(results []types.FileSearchResult, extensions []string) []types.FileSearchResult {
	extSet := make(map[string]bool)
	for _, ext := range extensions {
		extSet[ext] = true
	}
	
	var filtered []types.FileSearchResult
	for _, result := range results {
		if extSet[result.Extension] {
			filtered = append(filtered, result)
		}
	}
	
	return filtered
}

// filterByDirectories filters results to only include files in specified directories
func (fse *FileSearchEngine) filterByDirectories(results []types.FileSearchResult, directories []string) []types.FileSearchResult {
	var filtered []types.FileSearchResult
	
	for _, result := range results {
		for _, dir := range directories {
			matched, err := filepath.Match(dir, result.Directory)
			if err == nil && matched {
				filtered = append(filtered, result)
				break
			}
		}
	}
	
	return filtered
}

// GetStats returns statistics about the file path index
func (fse *FileSearchEngine) GetStats() map[string]interface{} {
	fse.mu.RLock()
	defer fse.mu.RUnlock()

	stats := make(map[string]interface{})

	stats["total_files"] = fse.pathIndex.TotalFiles
	stats["path_segments"] = len(fse.pathIndex.PathSegments)
	stats["extensions"] = len(fse.pathIndex.Extensions)
	stats["directories"] = len(fse.pathIndex.Directories)
	stats["base_names"] = len(fse.pathIndex.BaseNames)

	// Extension breakdown
	extStats := make(map[string]int)
	for ext, fileIDs := range fse.pathIndex.Extensions {
		extStats[ext] = len(fileIDs)
	}
	stats["extension_counts"] = extStats

	// Directory breakdown (top level only)
	topDirStats := make(map[string]int)
	for dir, fileIDs := range fse.pathIndex.Directories {
		topLevel := strings.Split(dir, string(filepath.Separator))[0]
		topDirStats[topLevel] += len(fileIDs)
	}
	stats["top_directory_counts"] = topDirStats

	return stats
}

// Reset clears all indexed data
func (fse *FileSearchEngine) Reset() {
	fse.mu.Lock()
	defer fse.mu.Unlock()
	fse.pathIndex = types.NewFilePathIndex()
}

// GetIndex returns the underlying path index (for testing and debugging)
func (fse *FileSearchEngine) GetIndex() *types.FilePathIndex {
	fse.mu.RLock()
	defer fse.mu.RUnlock()
	return fse.pathIndex
}