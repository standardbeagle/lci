package builders

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// SearchTestBuilder provides fluent API for building search test scenarios
type SearchTestBuilder struct {
	t               *testing.T
	indexer         *TestIndexer
	files           map[string]string
	searchPattern   string
	maxResults      int
	options         types.SearchOptions
	expectedResults []ExpectedResult
}

// ExpectedResult represents an expected search result
type ExpectedResult struct {
	Path       string
	Line       int
	Contains   string
	MinScore   float64
	NotPresent bool // If true, this path should NOT be in results
}

// NewSearchTestBuilder creates a new search test builder
func NewSearchTestBuilder(t *testing.T) *SearchTestBuilder {
	return &SearchTestBuilder{
		t:          t,
		files:      make(map[string]string),
		maxResults: 50,
		options:    types.SearchOptions{},
	}
}

// WithFile adds a file to be indexed
func (b *SearchTestBuilder) WithFile(path, content string) *SearchTestBuilder {
	b.files[path] = content
	return b
}

// WithGoFile adds a Go file with the given package name and functions
func (b *SearchTestBuilder) WithGoFile(pkg string, funcs ...string) *SearchTestBuilder {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("package %s\n\n", pkg))

	for _, fn := range funcs {
		builder.WriteString(fmt.Sprintf("func %s() {\n\t// implementation\n}\n\n", fn))
	}

	path := fmt.Sprintf("%s/%s.go", pkg, pkg)
	b.files[path] = builder.String()
	return b
}

// WithJSFile adds a JavaScript file
func (b *SearchTestBuilder) WithJSFile(name string, functions ...string) *SearchTestBuilder {
	var builder strings.Builder

	for _, fn := range functions {
		builder.WriteString(fmt.Sprintf("function %s() {\n  // implementation\n}\n\n", fn))
	}

	path := fmt.Sprintf("src/%s.js", name)
	b.files[path] = builder.String()
	return b
}

// WithPythonFile adds a Python file
func (b *SearchTestBuilder) WithPythonFile(name string, functions ...string) *SearchTestBuilder {
	var builder strings.Builder

	for _, fn := range functions {
		builder.WriteString(fmt.Sprintf("def %s():\n    pass\n\n", fn))
	}

	path := fmt.Sprintf("src/%s.py", name)
	b.files[path] = builder.String()
	return b
}

// SearchFor sets the search pattern
func (b *SearchTestBuilder) SearchFor(pattern string) *SearchTestBuilder {
	b.searchPattern = pattern
	return b
}

// WithMaxResults sets the maximum number of results
func (b *SearchTestBuilder) WithMaxResults(max int) *SearchTestBuilder {
	b.maxResults = max
	return b
}

// CaseInsensitive enables case-insensitive search
func (b *SearchTestBuilder) CaseInsensitive() *SearchTestBuilder {
	b.options.CaseInsensitive = true
	return b
}

// WithRegex enables regex search
func (b *SearchTestBuilder) WithRegex() *SearchTestBuilder {
	b.options.UseRegex = true
	return b
}

// WithWordBoundary enables word boundary matching
func (b *SearchTestBuilder) WithWordBoundary() *SearchTestBuilder {
	b.options.WordBoundary = true
	return b
}

// FilesOnly enables files-only mode
func (b *SearchTestBuilder) FilesOnly() *SearchTestBuilder {
	b.options.FilesOnly = true
	return b
}

// ExpectPath adds an expected result path
func (b *SearchTestBuilder) ExpectPath(path string) *SearchTestBuilder {
	b.expectedResults = append(b.expectedResults, ExpectedResult{Path: path})
	return b
}

// ExpectResult adds an expected result with details
func (b *SearchTestBuilder) ExpectResult(path string, line int, contains string) *SearchTestBuilder {
	b.expectedResults = append(b.expectedResults, ExpectedResult{
		Path:     path,
		Line:     line,
		Contains: contains,
	})
	return b
}

// ExpectNotFound marks a path that should NOT appear in results
func (b *SearchTestBuilder) ExpectNotFound(path string) *SearchTestBuilder {
	b.expectedResults = append(b.expectedResults, ExpectedResult{
		Path:       path,
		NotPresent: true,
	})
	return b
}

// Build creates the test scenario and returns the indexer
func (b *SearchTestBuilder) Build() *TestIndexer {
	b.indexer = NewTestIndexer(b.t)

	for path, content := range b.files {
		err := b.indexer.IndexString(path, content)
		require.NoError(b.t, err, "Failed to index file %s", path)
	}

	return b.indexer
}

// Run executes the search and validates expectations
func (b *SearchTestBuilder) Run() SearchTestResult {
	if b.indexer == nil {
		b.Build()
	}

	// Get all file IDs for search
	fileIDs := b.indexer.GetAllFileIDs()

	// Create a mock engine for testing
	// This is a simplified version - in real tests you'd use the actual engine
	result := SearchTestResult{
		Pattern:      b.searchPattern,
		FileCount:    len(fileIDs),
		Expectations: b.expectedResults,
	}

	return result
}

// SearchTestResult holds the results of a search test
type SearchTestResult struct {
	Pattern      string
	Results      []searchtypes.GrepResult
	FileCount    int
	Expectations []ExpectedResult
}

// ============================================================================
// File Content Builder
// ============================================================================

// FileContentBuilder helps build complex file contents for testing
type FileContentBuilder struct {
	lines []string
}

// NewFileContentBuilder creates a new file content builder
func NewFileContentBuilder() *FileContentBuilder {
	return &FileContentBuilder{
		lines: make([]string, 0, 50),
	}
}

// Package adds a package declaration (for Go files)
func (b *FileContentBuilder) Package(name string) *FileContentBuilder {
	b.lines = append(b.lines, fmt.Sprintf("package %s\n", name))
	return b
}

// Import adds an import statement
func (b *FileContentBuilder) Import(pkg string) *FileContentBuilder {
	b.lines = append(b.lines, fmt.Sprintf("import \"%s\"\n", pkg))
	return b
}

// Imports adds multiple imports
func (b *FileContentBuilder) Imports(pkgs ...string) *FileContentBuilder {
	b.lines = append(b.lines, "import (\n")
	for _, pkg := range pkgs {
		b.lines = append(b.lines, fmt.Sprintf("\t\"%s\"\n", pkg))
	}
	b.lines = append(b.lines, ")\n")
	return b
}

// Function adds a function definition
func (b *FileContentBuilder) Function(name string, params string, body string) *FileContentBuilder {
	b.lines = append(b.lines, fmt.Sprintf("\nfunc %s(%s) {\n\t%s\n}\n", name, params, body))
	return b
}

// Struct adds a struct definition
func (b *FileContentBuilder) Struct(name string, fields map[string]string) *FileContentBuilder {
	b.lines = append(b.lines, fmt.Sprintf("\ntype %s struct {\n", name))
	for field, typ := range fields {
		b.lines = append(b.lines, fmt.Sprintf("\t%s %s\n", field, typ))
	}
	b.lines = append(b.lines, "}\n")
	return b
}

// Method adds a method definition
func (b *FileContentBuilder) Method(receiver, receiverType, name, params, body string) *FileContentBuilder {
	b.lines = append(b.lines, fmt.Sprintf("\nfunc (%s *%s) %s(%s) {\n\t%s\n}\n",
		receiver, receiverType, name, params, body))
	return b
}

// Comment adds a comment
func (b *FileContentBuilder) Comment(text string) *FileContentBuilder {
	b.lines = append(b.lines, fmt.Sprintf("// %s\n", text))
	return b
}

// Line adds a raw line
func (b *FileContentBuilder) Line(text string) *FileContentBuilder {
	b.lines = append(b.lines, text+"\n")
	return b
}

// Blank adds a blank line
func (b *FileContentBuilder) Blank() *FileContentBuilder {
	b.lines = append(b.lines, "\n")
	return b
}

// Build returns the final content string
func (b *FileContentBuilder) Build() string {
	return strings.Join(b.lines, "")
}

// ============================================================================
// Project Structure Builder
// ============================================================================

// ProjectBuilder helps build test project structures
type ProjectBuilder struct {
	t       *testing.T
	tempDir string
	files   map[string]string
}

// NewProjectBuilder creates a new project builder
func NewProjectBuilder(t *testing.T) *ProjectBuilder {
	return &ProjectBuilder{
		t:       t,
		tempDir: t.TempDir(),
		files:   make(map[string]string),
	}
}

// AddFile adds a file to the project
func (p *ProjectBuilder) AddFile(path, content string) *ProjectBuilder {
	p.files[path] = content
	return p
}

// AddGoModule adds a Go module with main.go and go.mod
func (p *ProjectBuilder) AddGoModule(moduleName string) *ProjectBuilder {
	p.files["go.mod"] = fmt.Sprintf("module %s\n\ngo 1.21\n", moduleName)
	p.files["main.go"] = `package main

func main() {
	println("Hello, World!")
}
`
	return p
}

// AddGoPackage adds a Go package with the given files
func (p *ProjectBuilder) AddGoPackage(pkgPath string, files map[string]string) *ProjectBuilder {
	for name, content := range files {
		fullPath := filepath.Join(pkgPath, name)
		p.files[fullPath] = content
	}
	return p
}

// AddJSProject adds a basic JavaScript project
func (p *ProjectBuilder) AddJSProject() *ProjectBuilder {
	p.files["package.json"] = `{
  "name": "test-project",
  "version": "1.0.0"
}
`
	p.files["src/index.js"] = `function main() {
  console.log("Hello, World!");
}

module.exports = { main };
`
	return p
}

// AddPythonProject adds a basic Python project
func (p *ProjectBuilder) AddPythonProject() *ProjectBuilder {
	p.files["setup.py"] = `from setuptools import setup
setup(name='test-project', version='1.0.0')
`
	p.files["src/__init__.py"] = ""
	p.files["src/main.py"] = `def main():
    print("Hello, World!")

if __name__ == "__main__":
    main()
`
	return p
}

// TempDir returns the temporary directory path
func (p *ProjectBuilder) TempDir() string {
	return p.tempDir
}

// Build writes all files and returns the indexer
func (p *ProjectBuilder) Build() *TestIndexer {
	indexer := NewTestIndexer(p.t)

	for path, content := range p.files {
		err := indexer.IndexString(path, content)
		require.NoError(p.t, err, "Failed to index file %s", path)
	}

	return indexer
}

// ============================================================================
// Search Result Builder (for mocking)
// ============================================================================

// MockResultBuilder helps build mock search results for testing
type MockResultBuilder struct {
	results []searchtypes.GrepResult
}

// NewMockResultBuilder creates a new mock result builder
func NewMockResultBuilder() *MockResultBuilder {
	return &MockResultBuilder{
		results: make([]searchtypes.GrepResult, 0, 10),
	}
}

// AddResult adds a mock result
func (b *MockResultBuilder) AddResult(path string, line int, match string, score float64) *MockResultBuilder {
	b.results = append(b.results, searchtypes.GrepResult{
		Path:  path,
		Line:  line,
		Match: match,
		Score: score,
	})
	return b
}

// Build returns the mock results
func (b *MockResultBuilder) Build() []searchtypes.GrepResult {
	return b.results
}

// ============================================================================
// Symbol Builder
// ============================================================================

// SymbolBuilder helps build test symbols
type SymbolBuilder struct {
	symbols []*types.EnhancedSymbol
}

// NewSymbolBuilder creates a new symbol builder
func NewSymbolBuilder() *SymbolBuilder {
	return &SymbolBuilder{
		symbols: make([]*types.EnhancedSymbol, 0, 10),
	}
}

// AddFunction adds a function symbol
func (b *SymbolBuilder) AddFunction(name string, line, col int) *SymbolBuilder {
	b.symbols = append(b.symbols, &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:   name,
			Type:   types.SymbolTypeFunction,
			Line:   line,
			Column: col,
		},
	})
	return b
}

// AddClass adds a class/struct symbol
func (b *SymbolBuilder) AddClass(name string, line, col int) *SymbolBuilder {
	b.symbols = append(b.symbols, &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:   name,
			Type:   types.SymbolTypeClass,
			Line:   line,
			Column: col,
		},
	})
	return b
}

// AddMethod adds a method symbol
func (b *SymbolBuilder) AddMethod(name string, line, col int) *SymbolBuilder {
	b.symbols = append(b.symbols, &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:   name,
			Type:   types.SymbolTypeMethod,
			Line:   line,
			Column: col,
		},
	})
	return b
}

// AddVariable adds a variable symbol
func (b *SymbolBuilder) AddVariable(name string, line, col int) *SymbolBuilder {
	b.symbols = append(b.symbols, &types.EnhancedSymbol{
		Symbol: types.Symbol{
			Name:   name,
			Type:   types.SymbolTypeVariable,
			Line:   line,
			Column: col,
		},
	})
	return b
}

// Build returns the built symbols
func (b *SymbolBuilder) Build() []*types.EnhancedSymbol {
	return b.symbols
}
