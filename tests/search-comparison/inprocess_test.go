package searchcomparison

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// inProcessIndex wraps a MasterIndex for in-process search testing,
// eliminating dependency on external lci daemon servers.
type inProcessIndex struct {
	idx         *indexing.MasterIndex
	cfg         *config.Config
	projectRoot string
}

// setupInProcessIndex creates an in-process index for the given fixture directory.
func setupInProcessIndex(t *testing.T, absFixtureDir string) *inProcessIndex {
	t.Helper()

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: absFixtureDir,
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   500,
			MaxFileCount:     10000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:   200,
			MaxGoroutines: 4,
		},
		Search: config.Search{
			MaxResults:         1000,
			MaxContextLines:    0,
			EnableFuzzy:        false,
			MergeFileResults:   false,
			EnsureCompleteStmt: false,
		},
		Include: []string{"**/*"},
		Exclude: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/.git/**",
		},
	}

	idx := indexing.NewMasterIndex(cfg)
	err := idx.IndexDirectory(context.Background(), absFixtureDir)
	require.NoError(t, err, "Failed to build in-process index for %s", absFixtureDir)

	return &inProcessIndex{
		idx:         idx,
		cfg:         cfg,
		projectRoot: absFixtureDir,
	}
}

// Close releases the index resources.
func (ipi *inProcessIndex) Close() {
	if ipi.idx != nil {
		ipi.idx.Close()
	}
}

// search performs a case-sensitive search and converts results to SearchResults.
func (ipi *inProcessIndex) search(t *testing.T, pattern string) SearchResults {
	t.Helper()
	return ipi.searchWithOptions(t, pattern, false)
}

// searchCaseInsensitive performs a case-insensitive search.
func (ipi *inProcessIndex) searchCaseInsensitive(t *testing.T, pattern string) SearchResults {
	t.Helper()
	return ipi.searchWithOptions(t, pattern, true)
}

func (ipi *inProcessIndex) searchWithOptions(t *testing.T, pattern string, caseInsensitive bool) SearchResults {
	t.Helper()

	if pattern == "" {
		return SearchResults{}
	}

	opts := types.SearchOptions{
		MaxResults:       1000,
		CaseInsensitive:  caseInsensitive,
		MergeFileResults: false,
	}

	results, err := ipi.idx.SearchWithOptions(pattern, opts)
	if err != nil {
		// Validation errors (e.g. empty pattern) → empty results, matching grep behavior
		t.Logf("in-process search error for %q: %v", pattern, err)
		return SearchResults{}
	}

	return convertGrepResults(results, ipi.projectRoot)
}

// convertGrepResults converts searchtypes.GrepResult slice to SearchResults,
// stripping the project root prefix to produce relative paths matching grep output.
func convertGrepResults(grepResults []searchtypes.Result, projectRoot string) SearchResults {
	prefix := projectRoot
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	out := make(SearchResults, 0, len(grepResults))
	for _, gr := range grepResults {
		filePath := gr.Path
		if strings.HasPrefix(filePath, prefix) {
			filePath = strings.TrimPrefix(filePath, prefix)
		}

		content := gr.Match
		// If we have context lines, extract the matching line
		if len(gr.Context.Lines) > 0 {
			lineIdx := gr.Line - gr.Context.StartLine
			if lineIdx >= 0 && lineIdx < len(gr.Context.Lines) {
				content = strings.TrimSpace(gr.Context.Lines[lineIdx])
			}
		}

		out = append(out, SearchResult{
			FilePath: filePath,
			Line:     gr.Line,
			Content:  content,
		})
	}
	return out
}

// getOrCreateIndex returns a cached in-process index for the directory,
// creating and indexing it on first access. Safe to call from multiple tests
// since Go test runs subtests serially within a single test function.
func getOrCreateIndex(t *testing.T, absFixtureDir string) *inProcessIndex {
	t.Helper()

	if idx, ok := indexCache[absFixtureDir]; ok {
		return idx
	}

	idx := setupInProcessIndex(t, absFixtureDir)
	indexCache[absFixtureDir] = idx
	return idx
}
