package analysis

import (
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// Mock implementations for testing

type mockSymbolLinker struct {
	references map[types.CompositeSymbolID][]ResolvedReference
}

func (msl *mockSymbolLinker) ResolveReferences(symbolID types.CompositeSymbolID) ([]ResolvedReference, error) {
	refs, exists := msl.references[symbolID]
	if !exists {
		return []ResolvedReference{}, nil
	}
	return refs, nil
}

func (msl *mockSymbolLinker) ResolveDependencies(symbolID types.CompositeSymbolID) ([]types.SymbolDependency, error) {
	return []types.SymbolDependency{}, nil
}

func (msl *mockSymbolLinker) ResolveImports(fileID types.FileID) ([]ImportResolution, error) {
	return []ImportResolution{}, nil
}

type mockFileService struct {
	files map[types.FileID]mockFile
}

type mockFile struct {
	content  string
	path     string
	language string
	size     int64
}

func (mfs *mockFileService) GetFileContent(fileID types.FileID) (string, error) {
	file, exists := mfs.files[fileID]
	if !exists {
		return "", nil
	}
	return file.content, nil
}

func (mfs *mockFileService) GetFilePath(fileID types.FileID) (string, error) {
	file, exists := mfs.files[fileID]
	if !exists {
		return "", nil
	}
	return file.path, nil
}

func (mfs *mockFileService) GetFileInfo(fileID types.FileID) (FileInfo, error) {
	file, exists := mfs.files[fileID]
	if !exists {
		return FileInfo{}, nil
	}

	return FileInfo{
		Path:         file.path,
		Language:     file.language,
		Size:         file.size,
		LastModified: time.Now(),
		IsGenerated:  false,
	}, nil
}

// Test functions

func TestRelationshipAnalyzer_Creation(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{files: make(map[types.FileID]mockFile)}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	if analyzer == nil {
		t.Fatal("Failed to create RelationshipAnalyzer")
	}

	if analyzer.universalGraph != universalGraph {
		t.Error("UniversalGraph not properly assigned")
	}

	if analyzer.symbolLinker != symbolLinker {
		t.Error("SymbolLinker not properly assigned")
	}

	if analyzer.fileService != fileService {
		t.Error("FileService not properly assigned")
	}

	// Check that language analyzers were initialized
	if len(analyzer.languageAnalyzers) == 0 {
		t.Error("No language analyzers were initialized")
	}
}

// TestRelationshipAnalyzer_AnalyzeGoFile tests the relationship analyzer analyze go file.
func TestRelationshipAnalyzer_AnalyzeGoFile(t *testing.T) {
	// Setup test data
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			1: {
				content: `package main

import "fmt"

type User struct {
	Name string
	Age  int
}

func (u *User) GetName() string {
	return u.Name
}

func main() {
	user := &User{Name: "John", Age: 30}
	fmt.Println(user.GetName())
}`,
				path:     "main.go",
				language: "go",
				size:     200,
			},
		},
	}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Analyze the file
	err := analyzer.AnalyzeFile(types.FileID(1))
	if err != nil {
		t.Fatalf("Failed to analyze Go file: %v", err)
	}

	// Check that symbols were extracted and added to the universal graph
	stats := analyzer.GetAnalysisStats()
	if stats.TotalSymbols == 0 {
		t.Error("No symbols were extracted from Go file")
	}

	// Check that Go symbols were detected
	if stats.SymbolsByLanguage["go"] == 0 {
		t.Error("No Go symbols were detected")
	}

	// Verify specific symbols exist
	symbols := universalGraph.GetSymbolsByFile(types.FileID(1))
	if len(symbols) == 0 {
		t.Error("No symbols found in universal graph for the file")
	}

	// Check for expected symbol types
	symbolNames := make(map[string]bool)
	for _, symbol := range symbols {
		symbolNames[symbol.Identity.Name] = true
	}

	expectedSymbols := []string{"main", "User", "GetName"}
	for _, expected := range expectedSymbols {
		if !symbolNames[expected] {
			t.Errorf("Expected symbol '%s' not found", expected)
		}
	}
}

// TestRelationshipAnalyzer_AnalyzeJavaScriptFile tests the relationship analyzer analyze java script file.
func TestRelationshipAnalyzer_AnalyzeJavaScriptFile(t *testing.T) {
	// Setup test data
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			2: {
				content: `import React from 'react';

export class UserComponent extends React.Component {
	constructor(props) {
		super(props);
		this.state = { name: '' };
	}
	
	async fetchUser() {
		const response = await fetch('/api/user');
		return response.json();
	}
	
	render() {
		return <div>{this.state.name}</div>;
	}
}

export function createUser(name) {
	return { name, id: Math.random() };
}`,
				path:     "UserComponent.js",
				language: "javascript",
				size:     300,
			},
		},
	}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Analyze the file
	err := analyzer.AnalyzeFile(types.FileID(2))
	if err != nil {
		t.Fatalf("Failed to analyze JavaScript file: %v", err)
	}

	// Check that symbols were extracted
	stats := analyzer.GetAnalysisStats()
	if stats.TotalSymbols == 0 {
		t.Error("No symbols were extracted from JavaScript file")
	}

	// Check that JavaScript symbols were detected
	if stats.SymbolsByLanguage["javascript"] == 0 {
		t.Error("No JavaScript symbols were detected")
	}

	// Verify specific symbols exist
	symbols := universalGraph.GetSymbolsByFile(types.FileID(2))
	if len(symbols) == 0 {
		t.Error("No symbols found in universal graph for the file")
	}

	// Check for expected symbol types
	symbolNames := make(map[string]bool)
	for _, symbol := range symbols {
		symbolNames[symbol.Identity.Name] = true
		t.Logf("Found symbol: %s (kind: %s)", symbol.Identity.Name, symbol.Identity.Kind.String())
	}

	expectedSymbols := []string{"UserComponent", "createUser"}
	for _, expected := range expectedSymbols {
		if !symbolNames[expected] {
			t.Errorf("Expected symbol '%s' not found. Available symbols: %v", expected, symbolNames)
		}
	}
}

// TestRelationshipAnalyzer_AnalyzeProject tests the relationship analyzer analyze project.
func TestRelationshipAnalyzer_AnalyzeProject(t *testing.T) {
	// Setup test data with multiple files
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			1: {
				content: `package main
func main() {
	println("Hello")
}`,
				path:     "main.go",
				language: "go",
				size:     50,
			},
			2: {
				content: `export function helper() {
	return "helper";
}`,
				path:     "helper.js",
				language: "javascript",
				size:     40,
			},
		},
	}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Analyze the project
	fileIDs := []types.FileID{1, 2}
	err := analyzer.AnalyzeProject(fileIDs)
	if err != nil {
		t.Fatalf("Failed to analyze project: %v", err)
	}

	// Check that symbols from both files were extracted
	stats := analyzer.GetAnalysisStats()
	if stats.TotalSymbols == 0 {
		t.Error("No symbols were extracted from project")
	}

	// Should have symbols from both languages
	if stats.SymbolsByLanguage["go"] == 0 {
		t.Error("No Go symbols found in project analysis")
	}
	if stats.SymbolsByLanguage["javascript"] == 0 {
		t.Error("No JavaScript symbols found in project analysis")
	}

	// Check file co-location analysis (handle duplicates)
	goSymbols := universalGraph.GetSymbolsByFile(types.FileID(1))
	if len(goSymbols) > 1 {
		// Get unique symbols by name to handle potential duplicates
		uniqueGoSymbols := make(map[string]*types.UniversalSymbolNode)
		for _, symbol := range goSymbols {
			uniqueGoSymbols[symbol.Identity.Name] = symbol
		}

		if len(uniqueGoSymbols) > 1 {
			// Check if file co-location relationships were created
			for _, symbol := range uniqueGoSymbols {
				expectedCoLocated := len(uniqueGoSymbols) - 1
				if len(symbol.Relationships.FileCoLocated) != expectedCoLocated {
					t.Errorf("File co-location relationships not properly created. Symbol %s has %d co-located, expected %d",
						symbol.Identity.Name, len(symbol.Relationships.FileCoLocated), expectedCoLocated)
					break
				}
			}
		}
	}
}

// TestRelationshipAnalyzer_FileCoLocationAnalysis tests the relationship analyzer file co location analysis.
func TestRelationshipAnalyzer_FileCoLocationAnalysis(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			1: {
				content: `package test

func Function1() {}
func Function2() {}
func Function3() {}`,
				path:     "test.go",
				language: "go",
				size:     80,
			},
		},
	}
	config := DefaultAnalysisConfig()
	config.AnalyzeFileCoLocation = true

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Analyze the file
	err := analyzer.AnalyzeFile(types.FileID(1))
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	// Get symbols and check co-location relationships
	symbols := universalGraph.GetSymbolsByFile(types.FileID(1))
	if len(symbols) < 3 {
		t.Fatalf("Expected at least 3 symbols, got %d", len(symbols))
	}

	// Count unique symbols by name to handle potential duplicates
	uniqueSymbols := make(map[string]*types.UniversalSymbolNode)
	for _, symbol := range symbols {
		uniqueSymbols[symbol.Identity.Name] = symbol
	}

	uniqueSymbolList := make([]*types.UniversalSymbolNode, 0, len(uniqueSymbols))
	for _, symbol := range uniqueSymbols {
		uniqueSymbolList = append(uniqueSymbolList, symbol)
	}

	t.Logf("Total symbols found: %d (unique: %d)", len(symbols), len(uniqueSymbolList))
	for i, symbol := range uniqueSymbolList {
		t.Logf("Unique Symbol %d: %s (kind: %s)", i, symbol.Identity.Name, symbol.Identity.Kind.String())
	}

	if len(uniqueSymbolList) < 3 {
		t.Fatalf("Expected at least 3 unique symbols, got %d", len(uniqueSymbolList))
	}

	// Each symbol should have co-location relationships with the others
	for _, symbol := range uniqueSymbolList {
		expectedCoLocated := len(uniqueSymbolList) - 1 // All others except itself
		if len(symbol.Relationships.FileCoLocated) != expectedCoLocated {
			t.Errorf("Symbol %s has %d co-located symbols, expected %d (unique symbols: %d)",
				symbol.Identity.Name, len(symbol.Relationships.FileCoLocated), expectedCoLocated, len(uniqueSymbolList))
		}
	}
}

// TestRelationshipAnalyzer_ConcurrentAnalysis tests the relationship analyzer concurrent analysis.
func TestRelationshipAnalyzer_ConcurrentAnalysis(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}

	// Create multiple files for concurrent testing
	files := make(map[types.FileID]mockFile)
	var fileIDs []types.FileID

	for i := 1; i <= 5; i++ {
		fileID := types.FileID(i)
		fileIDs = append(fileIDs, fileID)
		files[fileID] = mockFile{
			content: `package main
func Function` + string(rune('0'+i)) + `() {
	println("Function ` + string(rune('0'+i)) + `")
}`,
			path:     "file" + string(rune('0'+i)) + ".go",
			language: "go",
			size:     60,
		}
	}

	fileService := &mockFileService{files: files}
	config := DefaultAnalysisConfig()
	config.ConcurrentAnalysis = true
	config.MaxConcurrentFiles = 3

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Analyze project concurrently
	err := analyzer.AnalyzeProject(fileIDs)
	if err != nil {
		t.Fatalf("Failed to analyze project concurrently: %v", err)
	}

	// Check that all files were processed
	stats := analyzer.GetAnalysisStats()
	if stats.SymbolsByLanguage["go"] < 5 {
		t.Errorf("Expected at least 5 Go symbols, got %d", stats.SymbolsByLanguage["go"])
	}
}

// TestRelationshipAnalyzer_UnsupportedLanguage tests the relationship analyzer unsupported language.
func TestRelationshipAnalyzer_UnsupportedLanguage(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			1: {
				content:  "print('Hello World')",
				path:     "test.py", // Python - unsupported
				language: "python",
				size:     20,
			},
		},
	}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Should not error on unsupported language, but should skip processing
	err := analyzer.AnalyzeFile(types.FileID(1))
	if err != nil {
		t.Fatalf("Should not error on unsupported language: %v", err)
	}

	// Should have no symbols since language is unsupported
	stats := analyzer.GetAnalysisStats()
	if stats.TotalSymbols != 0 {
		t.Error("Should have no symbols for unsupported language")
	}
}

// TestRelationshipAnalyzer_ConfigUpdate tests the relationship analyzer config update.
func TestRelationshipAnalyzer_ConfigUpdate(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{files: make(map[types.FileID]mockFile)}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Initial config
	if !analyzer.config.AnalyzeExtends {
		t.Error("AnalyzeExtends should be enabled by default")
	}

	// Update config
	newConfig := DefaultAnalysisConfig()
	newConfig.AnalyzeExtends = false
	newConfig.EnabledLanguages = []string{"go"} // Only Go

	err := analyzer.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Check config was updated
	if analyzer.config.AnalyzeExtends {
		t.Error("AnalyzeExtends should be disabled after config update")
	}

	if len(analyzer.config.EnabledLanguages) != 1 || analyzer.config.EnabledLanguages[0] != "go" {
		t.Error("EnabledLanguages not properly updated")
	}
}

// TestRelationshipAnalyzer_Stats tests the relationship analyzer stats.
func TestRelationshipAnalyzer_Stats(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			1: {
				content: `package main
func TestFunction() {}
type TestType struct {}`,
				path:     "test.go",
				language: "go",
				size:     50,
			},
		},
	}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Initial stats should be empty
	initialStats := analyzer.GetAnalysisStats()
	if initialStats.TotalSymbols != 0 {
		t.Error("Initial stats should show zero symbols")
	}

	// Analyze file
	err := analyzer.AnalyzeFile(types.FileID(1))
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	// Stats should now show symbols
	finalStats := analyzer.GetAnalysisStats()
	if finalStats.TotalSymbols == 0 {
		t.Error("Final stats should show symbols after analysis")
	}

	if len(finalStats.EnabledAnalyzers) == 0 {
		t.Error("Should have enabled analyzers in stats")
	}

	// Check that Go analyzer is listed
	hasGoAnalyzer := false
	for _, analyzer := range finalStats.EnabledAnalyzers {
		if analyzer == "go" {
			hasGoAnalyzer = true
			break
		}
	}
	if !hasGoAnalyzer {
		t.Error("Go analyzer should be listed in enabled analyzers")
	}
}

// TestRelationshipAnalyzer_Clear tests the relationship analyzer clear.
func TestRelationshipAnalyzer_Clear(t *testing.T) {
	universalGraph := core.NewUniversalSymbolGraph()
	symbolLinker := &mockSymbolLinker{references: make(map[types.CompositeSymbolID][]ResolvedReference)}
	fileService := &mockFileService{
		files: map[types.FileID]mockFile{
			1: {
				content: `package main
func TestFunction() {}`,
				path:     "test.go",
				language: "go",
				size:     30,
			},
		},
	}
	config := DefaultAnalysisConfig()

	analyzer := NewRelationshipAnalyzer(universalGraph, symbolLinker, fileService, config)

	// Analyze file
	err := analyzer.AnalyzeFile(types.FileID(1))
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	// Should have symbols
	stats := analyzer.GetAnalysisStats()
	if stats.TotalSymbols == 0 {
		t.Error("Should have symbols before clear")
	}

	// Clear analyzer
	analyzer.Clear()

	// Should have no symbols after clear
	clearedStats := analyzer.GetAnalysisStats()
	if clearedStats.TotalSymbols != 0 {
		t.Error("Should have no symbols after clear")
	}
}

// TestDefaultAnalysisConfig tests the default analysis config.
func TestDefaultAnalysisConfig(t *testing.T) {
	config := DefaultAnalysisConfig()

	// Check that default values are reasonable
	if len(config.EnabledLanguages) == 0 {
		t.Error("Default config should have enabled languages")
	}

	if !config.AnalyzeExtends {
		t.Error("Default config should analyze extends relationships")
	}

	if !config.ConcurrentAnalysis {
		t.Error("Default config should enable concurrent analysis")
	}

	if config.MaxConcurrentFiles <= 0 {
		t.Error("Default config should have positive max concurrent files")
	}

	// Check that sensible limits are set
	if config.MaxCallDepth <= 0 || config.MaxCallDepth > 100 {
		t.Error("Default max call depth should be reasonable")
	}

	if config.MaxReferences <= 0 {
		t.Error("Default max references should be positive")
	}
}
