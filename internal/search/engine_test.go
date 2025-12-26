package search

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/interfaces"
	"github.com/standardbeagle/lci/internal/types"
)

// mockLineProvider provides a simple LineProvider implementation for tests
// that uses fileInfo.Lines directly
type mockLineProvider struct{}

func (m *mockLineProvider) GetLine(fileInfo *types.FileInfo, lineNum int) string {
	if lineNum < 1 || lineNum > len(fileInfo.Lines) {
		return ""
	}
	return fileInfo.Lines[lineNum-1]
}

func (m *mockLineProvider) GetLineCount(fileInfo *types.FileInfo) int {
	return len(fileInfo.Lines)
}

func (m *mockLineProvider) GetLineRange(fileInfo *types.FileInfo, startLine, endLine int) []string {
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(fileInfo.Lines) {
		endLine = len(fileInfo.Lines)
	}
	if startLine > endLine {
		return nil
	}
	return fileInfo.Lines[startLine-1 : endLine]
}

// MockIndexer provides a mock implementation of the indexer interface for testing
type MockIndexer struct {
	files      map[types.FileID]*types.FileInfo
	symbols    map[types.FileID][]*types.EnhancedSymbol
	content    map[types.FileID][]byte
	nextFileID types.FileID
}

func NewMockIndexer() *MockIndexer {
	return &MockIndexer{
		files:      make(map[types.FileID]*types.FileInfo),
		symbols:    make(map[types.FileID][]*types.EnhancedSymbol),
		content:    make(map[types.FileID][]byte),
		nextFileID: 1,
	}
}

func (m *MockIndexer) AddFile(path string, content string) types.FileID {
	fileID := m.nextFileID
	m.nextFileID++

	lines := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		lines = append(lines, line)
	}

	m.files[fileID] = &types.FileInfo{
		ID:          fileID,
		Path:        path,
		Content:     []byte(content),
		Lines:       lines,
		LineOffsets: types.ComputeLineOffsets([]byte(content)),
	}

	m.content[fileID] = []byte(content)
	return fileID
}

func (m *MockIndexer) GetFileInfo(fileID types.FileID) *types.FileInfo {
	fileInfo := m.files[fileID]
	if fileInfo == nil {
		return nil
	}

	// If we have symbols for this file, include them in the returned FileInfo
	if symbols, ok := m.symbols[fileID]; ok && len(symbols) > 0 {
		// Create a copy to avoid modifying the original
		info := *fileInfo
		info.EnhancedSymbols = symbols
		return &info
	}

	return fileInfo
}

func (m *MockIndexer) GetAllFileIDs() []types.FileID {
	var ids []types.FileID
	for id := range m.files {
		ids = append(ids, id)
	}
	return ids
}

// GetConfig provides an empty config for path resolution
func (m *MockIndexer) GetConfig() *config.Config {
	return &config.Config{
		Project: config.Project{
			Root: "", // Empty root means paths are used as-is
		},
	}
}

func (m *MockIndexer) GetFileContent(fileID types.FileID) ([]byte, bool) {
	content, exists := m.content[fileID]
	return content, exists
}

func (m *MockIndexer) GetFilePath(fileID types.FileID) string {
	if info, exists := m.files[fileID]; exists {
		return info.Path
	}
	return ""
}

func (m *MockIndexer) GetFileLineOffsets(fileID types.FileID) ([]uint32, bool) {
	if info, exists := m.files[fileID]; exists && len(info.LineOffsets) > 0 {
		// Convert []int to []uint32
		offsets := make([]uint32, len(info.LineOffsets))
		for i, offset := range info.LineOffsets {
			offsets[i] = uint32(offset)
		}
		return offsets, true
	}
	// Compute from content if not available
	if content, exists := m.content[fileID]; exists {
		var offsets []uint32
		offsets = append(offsets, 0)
		for i, b := range content {
			if b == '\n' {
				offsets = append(offsets, uint32(i+1))
			}
		}
		return offsets, true
	}
	return nil, false
}

func (m *MockIndexer) FindSymbolsByName(name string) []*types.EnhancedSymbol {
	var results []*types.EnhancedSymbol
	for _, symbols := range m.symbols {
		for _, symbol := range symbols {
			if symbol.Name == name {
				results = append(results, symbol)
			}
		}
	}
	return results
}

func (m *MockIndexer) GetSymbolReferences(symbolID types.SymbolID) []types.Reference {
	return []types.Reference{}
}

func (m *MockIndexer) GetFileEnhancedSymbols(fileID types.FileID) []*types.EnhancedSymbol {
	return m.symbols[fileID]
}

func (m *MockIndexer) GetSymbolAtLine(fileID types.FileID, line int) *types.Symbol {
	symbols, exists := m.symbols[fileID]
	if !exists {
		return nil
	}

	for _, symbol := range symbols {
		if symbol.Line == line {
			return &symbol.Symbol
		}
	}
	return nil
}

func (m *MockIndexer) GetEnhancedSymbol(symbolID types.SymbolID) *types.EnhancedSymbol {
	for _, symbols := range m.symbols {
		for _, symbol := range symbols {
			if symbol.ID == symbolID {
				return symbol
			}
		}
	}
	return nil
}

func (m *MockIndexer) GetEnhancedSymbolAtLine(fileID types.FileID, line int) *types.EnhancedSymbol {
	symbol := m.GetSymbolAtLine(fileID, line)
	if symbol == nil {
		return nil
	}
	return &types.EnhancedSymbol{
		Symbol: *symbol,
		ID:     types.SymbolID(symbol.Line),
	}
}

func (m *MockIndexer) GetFileScopeInfo(fileID types.FileID) []types.ScopeInfo {
	return []types.ScopeInfo{}
}

func (m *MockIndexer) GetFileBlockBoundaries(fileID types.FileID) []types.BlockBoundary {
	// Return empty block boundaries for testing
	return []types.BlockBoundary{}
}

func (m *MockIndexer) GetFileReferences(fileID types.FileID) []types.Reference {
	return []types.Reference{}
}

func (m *MockIndexer) GetFileImports(fileID types.FileID) []types.Import {
	return []types.Import{}
}

func (m *MockIndexer) GetFileCount() int {
	return len(m.files)
}

func (m *MockIndexer) GetFileLineCount(fileID types.FileID) int {
	fileInfo := m.files[fileID]
	if fileInfo == nil {
		return 0
	}
	return len(fileInfo.Lines)
}

func (m *MockIndexer) GetFileLine(fileID types.FileID, lineNum int) (string, bool) {
	fileInfo := m.files[fileID]
	if fileInfo == nil || lineNum <= 0 || lineNum > len(fileInfo.Lines) {
		return "", false
	}
	return fileInfo.Lines[lineNum-1], true
}

func (m *MockIndexer) GetFileLines(fileID types.FileID, startLine, endLine int) []string {
	fileInfo := m.files[fileID]
	if fileInfo == nil {
		return nil
	}

	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(fileInfo.Lines) {
		endLine = len(fileInfo.Lines)
	}
	if startLine > endLine {
		return nil
	}

	return fileInfo.Lines[startLine-1 : endLine]
}

func (m *MockIndexer) GetFileSymbols(fileID types.FileID) []types.Symbol {
	// Extract base symbols from enhanced symbols
	var result []types.Symbol
	for _, sym := range m.symbols[fileID] {
		result = append(result, sym.Symbol)
	}
	return result
}

func (m *MockIndexer) GetIndexStats() interfaces.IndexStats {
	return interfaces.IndexStats{
		FileCount:      len(m.files),
		SymbolCount:    0,
		ReferenceCount: 0,
		ImportCount:    0,
		TotalSizeBytes: 0,
		IndexTimeMs:    0,
	}
}

func (m *MockIndexer) IndexDirectory(ctx context.Context, rootPath string) error {
	// Mock implementation - do nothing
	return nil
}

// TestNewEngine tests creating a new search engine
func TestNewEngine(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	require.NotNil(t, engine)
	assert.Equal(t, indexer, engine.indexer)
	assert.NotNil(t, engine.contextExtractor)
	assert.NotNil(t, engine.regexEngine)
}

// TestNewEngineWithConfig tests creating a search engine with custom configuration
func TestNewEngineWithConfig(t *testing.T) {
	indexer := NewMockIndexer()
	defaultContextLines := 25
	engine := NewEngineWithConfig(indexer, defaultContextLines)

	require.NotNil(t, engine)
	assert.Equal(t, indexer, engine.indexer)
	assert.Equal(t, defaultContextLines, engine.contextExtractor.defaultContextLines)
}

// TestNewContextExtractor tests creating a new context extractor
func TestNewContextExtractor(t *testing.T) {
	maxLines := 100
	extractor := NewContextExtractor(maxLines)

	require.NotNil(t, extractor)
	assert.Equal(t, maxLines, extractor.maxLines)
	assert.Equal(t, DefaultContextLines, extractor.defaultContextLines)
}

// TestEngine_Search_Basic tests basic search functionality
func TestEngine_Search_Basic(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	// Add test files
	fileID1 := indexer.AddFile("test1.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	fileID2 := indexer.AddFile("test2.go", "package main\n\nfunc helper() {\n\treturn 42\n}")

	// Test search
	candidates := []types.FileID{fileID1, fileID2}
	results := engine.Search("func", candidates, 3)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 2) // Should find "func" in both files

	// Verify results
	for _, result := range results {
		assert.NotEmpty(t, result.Path)
		assert.Greater(t, result.Line, 0)
		assert.Contains(t, result.Match, "func")
		assert.NotEmpty(t, result.Context.Lines)
	}
}

// TestEngine_Search_EmptyPattern tests empty pattern handling
func TestEngine_Search_EmptyPattern(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {}")
	candidates := []types.FileID{fileID}

	// Empty pattern should return no results
	results := engine.Search("", candidates, 3)
	assert.Empty(t, results)
}

// TestEngine_SearchWithOptions tests search with custom options
func TestEngine_SearchWithOptions(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	candidates := []types.FileID{fileID}

	// Test with case insensitive option
	options := types.SearchOptions{
		CaseInsensitive: true,
		MaxContextLines: 10,
	}
	results := engine.SearchWithOptions("FUNC", candidates, options)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 1)
}

// TestEngine_SearchDetailed tests detailed search functionality
func TestEngine_SearchDetailed(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	candidates := []types.FileID{fileID}

	results := engine.SearchDetailed("func", candidates, 5)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 1)

	// Verify detailed results have relational data
	for _, result := range results {
		assert.Equal(t, "func", result.Result.Match)
		assert.NotNil(t, result.RelationalData)
	}
}

// TestEngine_SearchRegex tests regex search functionality
func TestEngine_SearchRegex(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	candidates := []types.FileID{fileID}

	// Test regex pattern
	options := types.SearchOptions{
		UseRegex:        true,
		CaseInsensitive: false,
	}
	results := engine.SearchWithOptions("f.*n.", candidates, options)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 1)
}

// TestEngine_SearchInverted tests inverted search functionality
func TestEngine_SearchInverted(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n// comment line\n}")
	candidates := []types.FileID{fileID}

	// Test inverted search (lines that don't match)
	options := types.SearchOptions{
		InvertMatch: true,
	}
	results := engine.SearchWithOptions("func", candidates, options)

	require.NotEmpty(t, results)
	// Should find lines that don't contain "func"
	assert.GreaterOrEqual(t, len(results), 3) // package line, comment line, etc.
}

// TestEngine_SearchFilesOnly tests files-only search functionality
func TestEngine_SearchFilesOnly(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID1 := indexer.AddFile("test1.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	fileID2 := indexer.AddFile("test2.go", "package main\n\nfunc helper() {\n\treturn 42\n}")
	candidates := []types.FileID{fileID1, fileID2}

	// Test files-only mode
	options := types.SearchOptions{
		FilesOnly: true,
	}
	results := engine.SearchWithOptions("func", candidates, options)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 2)

	// Results should have no line numbers in files-only mode
	for _, result := range results {
		assert.Equal(t, 0, result.Line)
		assert.Empty(t, result.Context.Lines)
	}
}

// TestEngine_SearchCountPerFile tests count-per-file search functionality
func TestEngine_SearchCountPerFile(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID1 := indexer.AddFile("test1.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	fileID2 := indexer.AddFile("test2.go", "package main\n\nfunc helper() {\n\treturn 42\n}")
	candidates := []types.FileID{fileID1, fileID2}

	// Test count-per-file mode
	options := types.SearchOptions{
		CountPerFile: true,
	}
	results := engine.SearchWithOptions("func", candidates, options)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 2)

	// Results should have file match counts instead of line numbers
	for _, result := range results {
		assert.Equal(t, 0, result.Line) // No specific line in count mode
		assert.Greater(t, result.FileMatchCount, 0)
	}
}

// TestEngine_SearchMultiplePatterns tests multiple pattern search
func TestEngine_SearchMultiplePatterns(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	candidates := []types.FileID{fileID}

	// Test multiple patterns
	options := types.SearchOptions{
		Patterns: []string{"func", "package"},
	}
	results := engine.SearchWithOptions("", candidates, options)

	require.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 2) // Should find both patterns
}

// TestEngine_SearchWithSemanticFiltering tests semantic filtering
func TestEngine_SearchWithSemanticFiltering(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")

	// Add symbol information
	symbols := []*types.EnhancedSymbol{
		{Symbol: types.Symbol{Name: "main", Type: types.SymbolTypeFunction, Line: 3}},
	}
	indexer.symbols[fileID] = symbols

	candidates := []types.FileID{fileID}

	// Test with symbol type filter
	options := types.SearchOptions{
		SymbolTypes: []string{"function"},
	}
	results := engine.SearchWithOptions("main", candidates, options)

	require.NotEmpty(t, results)
}

// TestEngine_SearchStats tests search statistics functionality
func TestEngine_SearchStats(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID1 := indexer.AddFile("test1.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	fileID2 := indexer.AddFile("test2.go", "package main\n\nfunc helper() {\n\treturn 42\n}")
	candidates := []types.FileID{fileID1, fileID2}

	stats, err := engine.SearchStats("func", candidates, types.SearchOptions{})

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, "func", stats.Pattern)
	assert.Greater(t, stats.FilesWithMatches, 0)
	assert.Greater(t, stats.TotalMatches, 0)
	assert.NotEmpty(t, stats.FileDistribution)
	assert.NotEmpty(t, stats.DirDistribution)
}

// TestEngine_MultiSearchStats tests multi-pattern search statistics
func TestEngine_MultiSearchStats(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")
	candidates := []types.FileID{fileID}

	patterns := []string{"func", "package"}
	stats, err := engine.MultiSearchStats(patterns, candidates, types.SearchOptions{})

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, patterns, stats.Patterns)
	assert.Len(t, stats.Results, 2)                             // One result per pattern
	assert.GreaterOrEqual(t, stats.TotalSearchTimeMs, int64(0)) // Fast operations can complete in 0ms
}

// TestContextExtractor_Extract tests context extraction
func TestContextExtractor_Extract(t *testing.T) {
	extractor := NewContextExtractorWithLineProvider(100, DefaultContextLines, &mockLineProvider{})

	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n\nfunc helper() {\n\treturn 42\n}"),
		Lines: []string{
			"package main",
			"",
			"func main() {",
			"\tfmt.Println(\"hello\")",
			"}",
			"",
			"func helper() {",
			"\treturn 42",
			"}",
		},
	}

	matchLine := 3
	maxContextLines := 5

	context := extractor.Extract(fileInfo, matchLine, maxContextLines)

	require.NotNil(t, context)
	assert.Equal(t, 1, context.StartLine) // Should start around line 3
	assert.GreaterOrEqual(t, context.EndLine, matchLine)
	assert.NotEmpty(t, context.Lines)
}

// TestContextExtractor_ExtractWithBlockContext tests block context extraction
func TestContextExtractor_ExtractWithBlockContext(t *testing.T) {
	extractor := NewContextExtractorWithLineProvider(100, DefaultContextLines, &mockLineProvider{})

	// Create file info with blocks
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"),
		Lines: []string{
			"package main",
			"",
			"func main() {",
			"\tfmt.Println(\"hello\")",
			"}",
		},
		Blocks: []types.BlockBoundary{
			{
				Type:  types.BlockTypeFunction,
				Name:  "main",
				Start: 2, // 0-based
				End:   4,
			},
		},
	}

	matchLine := 3

	// Test with maxContextLines = 0 to get block context
	context := extractor.ExtractWithOptions(fileInfo, matchLine, 0, false)

	require.NotNil(t, context)
	assert.Equal(t, "function", context.BlockType)
	assert.Equal(t, "main", context.BlockName)
	assert.Equal(t, 3, context.StartLine) // Should start at function line
	assert.Equal(t, 5, context.EndLine)   // Should end at function end
}

// TestEngine_ScoreMatch tests match scoring functionality
func TestEngine_ScoreMatch(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"),
		Lines: []string{
			"package main",
			"",
			"func main() {",
			"\tfmt.Println(\"hello\")",
			"}",
		},
		EnhancedSymbols: []*types.EnhancedSymbol{
			{Symbol: types.Symbol{Name: "main", Type: types.SymbolTypeFunction, FileID: 1, Line: 3}},
		},
	}

	pattern := "main"
	match := Match{
		Start: 18, // Position of "main" in "func main"
		End:   22,
		Exact: true,
	}
	line := 3

	score := engine.scoreMatch(fileInfo, match, pattern, line)

	assert.Greater(t, score, 500.0) // Should get high score for exact function definition match
}

// TestClassifyFile tests file classification by extension and name patterns
func TestClassifyFile(t *testing.T) {
	tests := []struct {
		path     string
		expected FileCategory
	}{
		// Code files
		{"main.go", FileCategoryCode},
		{"src/utils.ts", FileCategoryCode},
		{"lib/parser.py", FileCategoryCode},
		{"internal/search.rs", FileCategoryCode},
		{"components/Button.tsx", FileCategoryCode},
		{"handlers.js", FileCategoryCode},

		// Documentation files
		{"README.md", FileCategoryDocumentation},
		{"docs/guide.txt", FileCategoryDocumentation},
		{"docs/api.rst", FileCategoryDocumentation},
		{"CHANGELOG.markdown", FileCategoryDocumentation},

		// Config files
		{"config.yaml", FileCategoryConfig},
		{"settings.json", FileCategoryConfig},
		{"Cargo.toml", FileCategoryConfig},
		{".lci.kdl", FileCategoryConfig},

		// Test files (identified by name pattern, not extension)
		{"main_test.go", FileCategoryTest},
		{"parser.test.ts", FileCategoryTest},
		{"utils.spec.js", FileCategoryTest},
		{"test_helpers.py", FileCategoryTest},

		// Unknown
		{"Makefile", FileCategoryUnknown},
		{"data.bin", FileCategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := classifyFile(tt.path)
			assert.Equal(t, tt.expected, result, "file %s should be classified as %d", tt.path, tt.expected)
		})
	}
}

// TestEngine_ScoreFileType tests file type scoring
func TestEngine_ScoreFileType(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	cfg := config.SearchRanking{
		Enabled:         true,
		CodeFileBoost:   50.0,
		DocFilePenalty:  -20.0,
		ConfigFileBoost: 10.0,
	}

	tests := []struct {
		path          string
		expectedScore float64
	}{
		{"main.go", 50.0},            // Code gets boost
		{"utils.ts", 50.0},           // Code gets boost
		{"README.md", -20.0},         // Doc gets penalty
		{"config.yaml", 10.0},        // Config gets small boost
		{"main_test.go", 50.0 * 0.8}, // Test files get 80% of code boost
		{"Makefile", 0.0},            // Unknown gets no adjustment
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			score := engine.scoreFileType(tt.path, cfg)
			assert.InDelta(t, tt.expectedScore, score, 0.001, "file %s should have score %f", tt.path, tt.expectedScore)
		})
	}
}

// TestEngine_ScoreSymbolPresence tests symbol presence scoring
func TestEngine_ScoreSymbolPresence(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	t.Run("with symbol - no penalty", func(t *testing.T) {
		cfg := config.SearchRanking{
			Enabled:          true,
			NonSymbolPenalty: -30.0,
		}
		score := engine.scoreSymbolPresence(true, cfg)
		assert.Equal(t, 0.0, score)
	})

	t.Run("without symbol - gets penalty", func(t *testing.T) {
		cfg := config.SearchRanking{
			Enabled:          true,
			NonSymbolPenalty: -30.0,
		}
		score := engine.scoreSymbolPresence(false, cfg)
		assert.Equal(t, -30.0, score)
	})

	t.Run("without symbol but require_symbol - severe penalty", func(t *testing.T) {
		cfg := config.SearchRanking{
			Enabled:          true,
			RequireSymbol:    true,
			NonSymbolPenalty: -30.0,
		}
		score := engine.scoreSymbolPresence(false, cfg)
		assert.Equal(t, -1000.0, score)
	})
}

// TestEngine_ScoreMatchWithRanking tests that scoring integrates ranking config
func TestEngine_ScoreMatchWithRanking(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	// Create two files - code and doc - with same content
	codeFile := &types.FileInfo{
		ID:      1,
		Path:    "main.go",
		Content: []byte("package main\n\nfunc main() {\n}"),
		Lines:   []string{"package main", "", "func main() {", "}"},
		EnhancedSymbols: []*types.EnhancedSymbol{
			{Symbol: types.Symbol{Name: "main", Type: types.SymbolTypeFunction, FileID: 1, Line: 3}},
		},
	}

	docFile := &types.FileInfo{
		ID:              2,
		Path:            "README.md",
		Content:         []byte("# main\n\nThis is the main function"),
		Lines:           []string{"# main", "", "This is the main function"},
		EnhancedSymbols: []*types.EnhancedSymbol{}, // No symbols in markdown
	}

	pattern := "main"
	match := Match{Start: 0, End: 4, Exact: true}

	codeScore := engine.scoreMatch(codeFile, match, pattern, 3)
	docScore := engine.scoreMatch(docFile, match, pattern, 1)

	// Code file with symbol definition should score MUCH higher than doc file
	assert.Greater(t, codeScore, docScore, "code file should rank higher than doc file")
	assert.Greater(t, codeScore-docScore, 100.0, "difference should be substantial")
}

// TestEngine_FilterFiles tests file filtering functionality
func TestEngine_FilterFiles(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	// Add files with different patterns
	fileID1 := indexer.AddFile("src/main.go", "package main")
	fileID2 := indexer.AddFile("src/helper.go", "package main")
	fileID3 := indexer.AddFile("test/main_test.go", "package main")

	candidates := []types.FileID{fileID1, fileID2, fileID3}

	// Test include pattern (use glob syntax, not regex)
	included := engine.filterIncludedFiles(candidates, "src/*.go")
	assert.Len(t, included, 2) // Should include main.go and helper.go

	// Test exclude pattern (use glob syntax, not regex)
	// Use **/* to match files in any directory
	excluded := engine.filterExcludedFiles(candidates, "**/*_test.go")
	assert.Len(t, excluded, 2) // Should exclude test file
}

// TestRegexErrorHandling tests regex error handling
func TestRegexErrorHandling(t *testing.T) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	fileID := indexer.AddFile("test.go", "package main")
	candidates := []types.FileID{fileID}

	// Test invalid regex pattern
	options := types.SearchOptions{
		UseRegex: true,
	}
	results := engine.SearchWithOptions("[invalid regex", candidates, options)

	// Should handle invalid regex gracefully by returning empty results
	assert.Empty(t, results)

	// Error may or may not be recorded depending on implementation
	// The important thing is that it doesn't crash and returns empty results
	_ = engine.LastError() // Clear any error state
}

// BenchmarkEngine_Search benchmarks basic search performance
func BenchmarkEngine_Search(b *testing.B) {
	indexer := NewMockIndexer()
	engine := NewEngine(indexer)

	// Add multiple files
	for i := 0; i < 100; i++ {
		indexer.AddFile(fmt.Sprintf("file%d.go", i), fmt.Sprintf("package main\n\nfunc func%d() {\n\treturn %d\n}", i, i))
	}

	var candidates []types.FileID
	for fileID := range indexer.files {
		candidates = append(candidates, fileID)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Search("func", candidates, 3)
	}
}

// BenchmarkContextExtractor_Extract benchmarks context extraction performance
func BenchmarkContextExtractor_Extract(b *testing.B) {
	extractor := NewContextExtractorWithLineProvider(100, DefaultContextLines, &mockLineProvider{})

	content := []byte(strings.Repeat("line of content\n", 1000))
	lines := strings.Split(string(content), "\n")
	fileInfo := &types.FileInfo{
		ID:      1,
		Path:    "test.go",
		Content: content,
		Lines:   lines,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractor.Extract(fileInfo, 500, 10)
	}
}
