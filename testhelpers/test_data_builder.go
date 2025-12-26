package testhelpers

import (
	"fmt"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// TestDataBuilder provides isolated test data creation without shared state
type TestDataBuilder struct {
	fileStore    *core.FileContentStore
	files        []*TestFile
	symbols      []*types.EnhancedSymbol
	fileContents map[string][]byte
}

// TestFile represents a test file with isolated content
type TestFile struct {
	Name            string
	Content         []byte
	ID              types.FileID
	EnhancedSymbols []*types.EnhancedSymbol
}

// NewTestDataBuilder creates a new test data builder
func NewTestDataBuilder() *TestDataBuilder {
	return &TestDataBuilder{
		fileStore:    core.NewFileContentStore(),
		files:        make([]*TestFile, 0),
		symbols:      make([]*types.EnhancedSymbol, 0),
		fileContents: make(map[string][]byte),
	}
}

// AddFile adds a file with the given name and content
func (tdb *TestDataBuilder) AddFile(name, content string) *TestDataBuilder {
	contentBytes := []byte(content)
	fileID := tdb.fileStore.LoadFile(name, contentBytes)

	testFile := &TestFile{
		Name:    name,
		Content: contentBytes,
		ID:      fileID,
	}

	tdb.files = append(tdb.files, testFile)
	tdb.fileContents[name] = contentBytes

	return tdb
}

// AddFileWithSymbols adds a file with symbol information
func (tdb *TestDataBuilder) AddFileWithSymbols(name, content string, symbols []*types.EnhancedSymbol) *TestDataBuilder {
	tdb.AddFile(name, content)
	if len(tdb.files) > 0 {
		tdb.files[len(tdb.files)-1].EnhancedSymbols = symbols
		tdb.symbols = append(tdb.symbols, symbols...)
	}
	return tdb
}

// AddGoFile creates a Go file with common patterns
func (tdb *TestDataBuilder) AddGoFile(name string, elements ...GoFileElement) *TestDataBuilder {
	var builder strings.Builder

	for _, element := range elements {
		builder.WriteString(element.Generate())
		builder.WriteString("\n")
	}

	content := builder.String()
	return tdb.AddFile(name, content)
}

// Build creates the final test data
func (tdb *TestDataBuilder) Build() *IsolatedTestData {
	// Create FileInfo for each file
	fileInfos := make([]*types.FileInfo, 0, len(tdb.files))
	for _, file := range tdb.files {
		fileInfo := &types.FileInfo{
			ID:              file.ID,
			Content:         file.Content,
			EnhancedSymbols: file.EnhancedSymbols,
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	return &IsolatedTestData{
		FileStore:    tdb.fileStore,
		Files:        tdb.files,
		FileInfos:    fileInfos,
		AllSymbols:   tdb.symbols,
		FileContents: tdb.fileContents,
	}
}

// Close cleans up the FileContentStore to prevent goroutine leaks
func (tdb *TestDataBuilder) Close() {
	if tdb.fileStore != nil {
		tdb.fileStore.Close()
		tdb.fileStore = nil
	}
}

// IsolatedTestData represents isolated test data
type IsolatedTestData struct {
	FileStore    *core.FileContentStore
	Files        []*TestFile
	FileInfos    []*types.FileInfo
	AllSymbols   []*types.EnhancedSymbol
	FileContents map[string][]byte
}

// GetFile returns a file by name
func (itd *IsolatedTestData) GetFile(name string) *TestFile {
	for _, file := range itd.Files {
		if file.Name == name {
			return file
		}
	}
	return nil
}

// GetFileInfo returns FileInfo for a file by name
func (itd *IsolatedTestData) GetFileInfo(name string) *types.FileInfo {
	for _, fileInfo := range itd.FileInfos {
		// Find the corresponding file to get the name
		for _, file := range itd.Files {
			if file.ID == fileInfo.ID && file.Name == name {
				return fileInfo
			}
		}
	}
	return nil
}

// Close cleans up the FileContentStore
func (itd *IsolatedTestData) Close() {
	if itd.FileStore != nil {
		itd.FileStore.Close()
		itd.FileStore = nil
	}
}

// GoFileElement represents an element in a Go file
type GoFileElement interface {
	Generate() string
}

// ConstElement represents a constant declaration
type ConstElement struct {
	Name  string
	Type  string
	Value string
}

func (c ConstElement) Generate() string {
	return fmt.Sprintf("const %s %s = %s", c.Name, c.Type, c.Value)
}

// TypeElement represents a type declaration
type TypeElement struct {
	Name string
	Def  string
}

func (te TypeElement) Generate() string {
	return fmt.Sprintf("type %s %s", te.Name, te.Def)
}

// StringElement represents a raw string element
type StringElement string

func (s StringElement) Generate() string {
	return string(s)
}

// PackageDecl creates a package declaration
type PackageDecl struct {
	Name string
}

func (p PackageDecl) Generate() string {
	return fmt.Sprintf("package %s", p.Name)
}

// ImportDecl creates an import declaration
type ImportDecl struct {
	Imports []string
}

func (i ImportDecl) Generate() string {
	if len(i.Imports) == 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString("import (")
	for _, imp := range i.Imports {
		builder.WriteString(fmt.Sprintf("\n\t\"%s\"", imp))
	}
	builder.WriteString("\n)")
	return builder.String()
}

// GlobalVar creates a global variable declaration
type GlobalVar struct {
	Name  string
	Type  string
	Value string
}

func (gv GlobalVar) Generate() string {
	return fmt.Sprintf("var %s %s = %s", gv.Name, gv.Type, gv.Value)
}

// FunctionDecl creates a function declaration
type FunctionDecl struct {
	Name       string
	Parameters []string
	ReturnType string
	Body       string
	Exported   bool
}

func (f FunctionDecl) Generate() string {
	name := f.Name
	if !f.Exported {
		name = strings.ToLower(name[:1]) + name[1:]
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("func %s(", name))

	if len(f.Parameters) > 0 {
		builder.WriteString(strings.Join(f.Parameters, ", "))
	}

	builder.WriteString(")")

	if f.ReturnType != "" {
		builder.WriteString(" " + f.ReturnType)
	}

	builder.WriteString(" {")
	if f.Body != "" {
		builder.WriteString("\n")
		builder.WriteString(f.Body)
		builder.WriteString("\n")
	}
	builder.WriteString("}")

	return builder.String()
}

// Helper functions for common test patterns

// SimpleGoFile creates a simple Go file with basic elements
func SimpleGoFile(name, packageName string) *TestDataBuilder {
	return NewTestDataBuilder().AddGoFile(name,
		PackageDecl{Name: packageName},
		ImportDecl{Imports: []string{"fmt"}},
		FunctionDecl{
			Name:     "main",
			Body:     `fmt.Println("Hello, World!")`,
			Exported: true,
		},
	)
}

// GoFileWithGlobals creates a Go file with global variables
func GoFileWithGlobals(name, packageName string, globals []GlobalVar) *TestDataBuilder {
	elements := []GoFileElement{
		PackageDecl{Name: packageName},
	}

	// Convert GlobalVar to GoFileElement
	for _, gv := range globals {
		elements = append(elements, gv)
	}

	return NewTestDataBuilder().AddGoFile(name, elements...)
}

// GoFileWithFunctions creates a Go file with multiple functions
func GoFileWithFunctions(name, packageName string, functions []FunctionDecl) *TestDataBuilder {
	elements := []GoFileElement{
		PackageDecl{Name: packageName},
		ImportDecl{Imports: []string{"fmt"}},
	}

	// Convert FunctionDecl to GoFileElement
	for _, fd := range functions {
		elements = append(elements, fd)
	}

	return NewTestDataBuilder().AddGoFile(name, elements...)
}
