package indexing

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

func (mi *MasterIndex) FindCandidateFiles(pattern string, caseInsensitive bool) []types.FileID {
	return mi.trigramIndex.FindCandidatesWithOptions(pattern, caseInsensitive)
}

// Simple search result types (temporary until search package is fixed)
type SimpleResult struct {
	FileID  types.FileID  `json:"file_id"`
	Path    string        `json:"path"`
	Line    int           `json:"line"`
	Column  int           `json:"column"`
	Context SimpleContext `json:"context"`
	Score   float64       `json:"score"`
}

type SimpleContext struct {
	Lines     []string `json:"lines"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
	BlockType string   `json:"block_type"`
	BlockName string   `json:"block_name"`
}

type SimpleEnhancedResult struct {
	Result         SimpleResult             `json:"result"`
	RelationalData *types.RelationalContext `json:"relational_data,omitempty"`
}

// extractContextFromLines extracts context using pre-split lines - no string operations!

// scoreLocation scores a match location for relevance

// getCandidateFiles returns candidate files for search based on pattern and options

// SearchStats was removed - statistics are now gathered via SearchWithOptions

// GetSymbolLocationIndex returns the symbol location index for fast position-based lookups
func (mi *MasterIndex) GetSymbolLocationIndex() *core.SymbolLocationIndex {
	return mi.symbolLocationIndex
}

// GetFilePath returns the file path for a given FileID
func (mi *MasterIndex) GetFilePath(fileID types.FileID) string {
	snapshot := mi.fileSnapshot.Load()
	return snapshot.reverseFileMap[fileID]
}

// SearchWithOptions performs a search with the given options
// @lci:labels[search,semantic-search,query-engine]
// @lci:category[search]
func (mi *MasterIndex) SearchWithOptions(pattern string, options types.SearchOptions) ([]searchtypes.Result, error) {
	// Check memory pressure before proceeding
	if mi.isMemoryPressureDetected() {
		return []searchtypes.Result{}, errors.New("memory pressure detected - indexing temporarily suspended")
	}

	// Validate inputs and options
	if err := mi.validateSearchInput(pattern, &options); err != nil {
		return nil, err
	}

	// Check index state, filtering out deleted files
	allFiles := mi.GetAllFileIDsFiltered()
	if len(allFiles) == 0 {
		err := errors.New("no files indexed - index appears to be empty")
		debug.LogIndexing("Warning: %v\n", err)
		return nil, err
	}

	// Validate core components
	if err := mi.validateSearchComponents(); err != nil {
		return nil, err
	}

	debug.LogIndexing("Search: pattern='%s' (%d candidates, max_results=%d)\n",
		pattern, len(allFiles), options.MaxResults)

	// Parse query syntax and filter candidates
	contentPattern, candidates := mi.parseQuerySyntax(pattern, allFiles)

	// Use injected search engine with semantic scoring, or create default engine
	engine := mi.searchEngine
	if engine == nil {
		engine = search.NewEngine(mi)
	}

	// Delegate to search engine
	results := engine.SearchWithOptions(contentPattern, candidates, options)

	// Record metrics
	atomic.AddInt64(&mi.searchCount, 1)

	return results, nil
}

// validateSearchInput validates search pattern and options
func (mi *MasterIndex) validateSearchInput(pattern string, options *types.SearchOptions) error {
	if pattern == "" {
		err := errors.New("search pattern cannot be empty")
		debug.LogIndexing("ERROR: %v\n", err)
		return err
	}

	if len(pattern) > 1000 {
		err := fmt.Errorf("search pattern too long: %d characters (max 1000)", len(pattern))
		debug.LogIndexing("ERROR: %v\n", err)
		return err
	}

	if options.MaxResults < 0 {
		err := fmt.Errorf("max results cannot be negative: %d", options.MaxResults)
		debug.LogIndexing("ERROR: %v\n", err)
		return err
	}

	if options.MaxResults == 0 {
		options.MaxResults = 100 // Default limit
	}

	return nil
}

// validateSearchComponents checks that required index components are initialized
func (mi *MasterIndex) validateSearchComponents() error {
	if mi.trigramIndex == nil {
		err := errors.New("trigram index not initialized")
		debug.LogIndexing("ERROR: %v\n", err)
		return err
	}

	if mi.symbolLocationIndex == nil {
		debug.LogIndexing("Warning: symbol location index not initialized - function context extraction may fail\n")
	}

	if mi.refTracker == nil {
		err := errors.New("reference tracker not initialized")
		debug.LogIndexing("ERROR: %v\n", err)
		return err
	}

	return nil
}

// parseQuerySyntax parses combined query syntax (path:, dir:, ext:) and returns content pattern and filtered candidates
func (mi *MasterIndex) parseQuerySyntax(pattern string, allFiles []types.FileID) (string, []types.FileID) {
	contentPattern := pattern
	candidates := allFiles

	// Check if pattern contains special syntax
	if !strings.Contains(pattern, "path:") && !strings.Contains(pattern, "dir:") && !strings.Contains(pattern, "ext:") {
		return contentPattern, candidates
	}

	// Parse fields
	fields := strings.Fields(pattern)
	var dirs, exts []string
	var glob string
	var contentParts []string

	for _, f := range fields {
		if strings.HasPrefix(f, "path:") {
			glob = strings.TrimPrefix(f, "path:")
		} else if strings.HasPrefix(f, "dir:") {
			dirs = append(dirs, strings.TrimPrefix(f, "dir:"))
		} else if strings.HasPrefix(f, "ext:") {
			e := strings.TrimPrefix(f, "ext:")
			if !strings.HasPrefix(e, ".") {
				e = "." + e
			}
			exts = append(exts, e)
		} else {
			contentParts = append(contentParts, f)
		}
	}

	if len(contentParts) > 0 {
		contentPattern = strings.Join(contentParts, " ")
	}

	// Build file search options and filter candidates
	fsOpts := mi.buildFileSearchOptions(glob, dirs, exts)
	if fsOpts.Pattern != "" || len(fsOpts.Directories) > 0 || len(fsOpts.Extensions) > 0 {
		candidates = mi.filterCandidatesByFileSearch(fsOpts, allFiles)
	}

	return contentPattern, candidates
}

// buildFileSearchOptions creates FileSearchOptions from parsed query fields
func (mi *MasterIndex) buildFileSearchOptions(glob string, dirs, exts []string) types.FileSearchOptions {
	fsOpts := types.FileSearchOptions{MaxResults: 50000}

	if glob != "" {
		fsOpts.Pattern = glob
		fsOpts.Type = "glob"
	}
	if len(dirs) > 0 {
		fsOpts.Directories = dirs
	}
	if len(exts) > 0 {
		fsOpts.Extensions = exts
	}

	return fsOpts
}

// filterCandidatesByFileSearch filters file candidates using file search options
func (mi *MasterIndex) filterCandidatesByFileSearch(fsOpts types.FileSearchOptions, allFiles []types.FileID) []types.FileID {
	results, err := mi.fileSearchEngine.SearchFiles(fsOpts)
	if err != nil || len(results) == 0 {
		return allFiles
	}

	candSet := make(map[types.FileID]struct{}, len(results))
	for _, r := range results {
		candSet[r.FileID] = struct{}{}
	}

	// Intersect with allFiles
	var filtered []types.FileID
	for _, id := range allFiles {
		if _, ok := candSet[id]; ok {
			filtered = append(filtered, id)
		}
	}

	return filtered
}

// SearchDetailedWithOptions performs a detailed search with the given options
func (mi *MasterIndex) SearchDetailedWithOptions(pattern string, options types.SearchOptions) ([]searchtypes.DetailedResult, error) {
	// Check memory pressure before proceeding
	if mi.isMemoryPressureDetected() {
		return []searchtypes.DetailedResult{}, errors.New("memory pressure detected - indexing temporarily suspended")
	}

	// Validate input
	if pattern == "" {
		return nil, errors.New("search pattern cannot be empty")
	}

	// Use injected search engine with semantic scoring, or create default engine
	engine := mi.searchEngine
	if engine == nil {
		engine = search.NewEngine(mi)
	}

	// Get all file IDs as candidates, filtering out deleted files
	allFiles := mi.GetAllFileIDsFiltered()
	if len(allFiles) == 0 {
		return nil, errors.New("no files indexed")
	}

	// Delegate to search engine
	results := engine.SearchDetailedWithOptions(pattern, allFiles, options)

	// Record metrics
	atomic.AddInt64(&mi.searchCount, 1)

	return results, nil
}

// Search performs a basic search with a maximum context lines
func (mi *MasterIndex) Search(pattern string, maxContextLines int) ([]searchtypes.Result, error) {
	// Use SearchWithOptions with basic options
	options := types.SearchOptions{
		MaxContextLines: maxContextLines,
	}
	return mi.SearchWithOptions(pattern, options)
}

// SearchDefinitions searches for symbol definitions (declarations)
func (mi *MasterIndex) SearchDefinitions(pattern string) ([]searchtypes.Result, error) {
	// Check memory pressure before proceeding
	if mi.isMemoryPressureDetected() {
		return []searchtypes.Result{}, errors.New("memory pressure detected - indexing temporarily suspended")
	}

	// Use SearchWithOptions with DeclarationOnly flag
	options := types.SearchOptions{
		DeclarationOnly: true,
		MaxContextLines: 5,
	}
	return mi.SearchWithOptions(pattern, options)
}

// SearchReferences searches for symbol references (usages)
func (mi *MasterIndex) SearchReferences(symbol string) ([]searchtypes.Result, error) {
	// Check memory pressure before proceeding
	if mi.isMemoryPressureDetected() {
		return []searchtypes.Result{}, errors.New("memory pressure detected - indexing temporarily suspended")
	}

	// Use SearchWithOptions with UsageOnly flag
	options := types.SearchOptions{
		UsageOnly:       true,
		MaxContextLines: 5,
	}
	return mi.SearchWithOptions(symbol, options)
}
