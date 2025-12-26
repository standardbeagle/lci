package symbollinker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIncrementalEngine_Basic tests the incremental engine basic.
func TestIncrementalEngine_Basic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Test basic functionality
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.SymbolLinkerEngine)

	// Test initial state
	stats := engine.IncrementalStats()
	assert.Equal(t, 0, stats["tracked_files"])
	assert.Equal(t, 0, stats["dependency_edges"])
	assert.Equal(t, 0, stats["pending_updates"])
}

// TestIncrementalEngine_SingleFileUpdate tests the incremental engine single file update.
func TestIncrementalEngine_SingleFileUpdate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_single_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Initial file content
	initialContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`

	mainPath := filepath.Join(tempDir, "main.go")

	// First update (add file)
	result1, err := engine.UpdateFile(mainPath, []byte(initialContent))
	require.NoError(t, err)
	require.NotNil(t, result1)

	assert.Len(t, result1.UpdatedFiles, 1)
	assert.Greater(t, len(result1.AddedSymbols), 0)
	assert.Len(t, result1.RemovedSymbols, 0)
	assert.Greater(t, result1.UpdateDuration, time.Duration(0))

	fileID := engine.GetOrCreateFileID(mainPath)
	assert.Contains(t, result1.UpdatedFiles, fileID)

	// Verify file is tracked
	hash1, exists1 := engine.GetFileHash(fileID)
	assert.True(t, exists1)
	assert.NotEqual(t, [32]byte{}, hash1)

	timestamp1, exists2 := engine.GetFileTimestamp(fileID)
	assert.True(t, exists2)
	assert.False(t, timestamp1.IsZero())

	// Second update (no change)
	result2, err := engine.UpdateFile(mainPath, []byte(initialContent))
	require.NoError(t, err)

	assert.Len(t, result2.UpdatedFiles, 0) // No changes
	assert.Len(t, result2.AddedSymbols, 0)
	assert.Len(t, result2.RemovedSymbols, 0)

	// Third update (modify file)
	modifiedContent := `package main

import "fmt"
import "os"

func main() {
	fmt.Println("Hello, World!")
	fmt.Println("Args:", os.Args)
}

func helper() {
	fmt.Println("Helper function")
}
`

	result3, err := engine.UpdateFile(mainPath, []byte(modifiedContent))
	require.NoError(t, err)

	assert.Len(t, result3.UpdatedFiles, 1)
	assert.Greater(t, len(result3.AddedSymbols), 0) // Should have new helper function

	// Verify hash changed
	hash2, _ := engine.GetFileHash(fileID)
	assert.NotEqual(t, hash1, hash2)
}

// TestIncrementalEngine_FileRemoval tests the incremental engine file removal.
func TestIncrementalEngine_FileRemoval(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_removal_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	content := `package main

func main() {}
`

	mainPath := filepath.Join(tempDir, "main.go")

	// Add file
	result1, err := engine.UpdateFile(mainPath, []byte(content))
	require.NoError(t, err)
	assert.Len(t, result1.UpdatedFiles, 1)

	fileID := engine.GetOrCreateFileID(mainPath)

	// Verify file exists
	_, exists := engine.GetFileHash(fileID)
	assert.True(t, exists)

	symbols, err := engine.GetSymbolsInFile(fileID)
	require.NoError(t, err)
	assert.Greater(t, len(symbols), 0)

	// Remove file
	result2, err := engine.RemoveFile(mainPath)
	require.NoError(t, err)

	assert.Len(t, result2.UpdatedFiles, 1)
	assert.Greater(t, len(result2.RemovedSymbols), 0)
	assert.Contains(t, result2.UpdatedFiles, fileID)

	// Verify file is gone
	_, exists = engine.GetFileHash(fileID)
	assert.False(t, exists)

	_, err = engine.GetSymbolsInFile(fileID)
	assert.Error(t, err) // Should error because file no longer exists
}

// TestIncrementalEngine_BatchUpdate tests the incremental engine batch update.
func TestIncrementalEngine_BatchUpdate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_batch_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Prepare multiple files
	files := map[string][]byte{
		filepath.Join(tempDir, "main.go"): []byte(`package main

import "./utils"

func main() {
	utils.Helper()
}
`),
		filepath.Join(tempDir, "utils.go"): []byte(`package utils

func Helper() {
	println("Helper")
}
`),
		filepath.Join(tempDir, "config.go"): []byte(`package config

const VERSION = "1.0.0"
`),
	}

	// Batch update
	result, err := engine.BatchUpdate(files)
	require.NoError(t, err)

	assert.Len(t, result.UpdatedFiles, 3)
	assert.Greater(t, len(result.AddedSymbols), 0)
	assert.Len(t, result.RemovedSymbols, 0)

	// Verify all files are tracked
	stats := engine.IncrementalStats()
	assert.Equal(t, 3, stats["tracked_files"])
}

// TestIncrementalEngine_CascadeUpdates tests the incremental engine cascade updates.
func TestIncrementalEngine_CascadeUpdates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_cascade_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Set up dependency: main.js -> utils.js
	utilsPath := filepath.Join(tempDir, "utils.js")
	mainPath := filepath.Join(tempDir, "main.js")

	utilsContent := `export function helper() {
	return "original helper";
}

export const CONSTANT = 42;
`

	mainContent := `import { helper, CONSTANT } from './utils.js';

export function main() {
	console.log(helper());
	console.log('Constant:', CONSTANT);
}
`

	// Initial setup
	_, err = engine.UpdateFile(utilsPath, []byte(utilsContent))
	require.NoError(t, err)

	_, err = engine.UpdateFile(mainPath, []byte(mainContent))
	require.NoError(t, err)

	utilsFileID := engine.GetOrCreateFileID(utilsPath)
	mainFileID := engine.GetOrCreateFileID(mainPath)

	// Link symbols to establish dependencies
	err = engine.LinkSymbols()
	require.NoError(t, err)

	// Verify dependency relationship
	dependents := engine.GetFileDependents(utilsFileID)
	assert.Contains(t, dependents, mainFileID)

	dependencies := engine.GetFileDependencies(mainFileID)
	assert.Contains(t, dependencies, utilsFileID)

	// Modify utils.js (should trigger cascade update in main.js)
	modifiedUtilsContent := `export function helper() {
	return "modified helper";
}

export const CONSTANT = 42;

export function newFunction() {
	return "new";
}
`

	result, err := engine.UpdateFile(utilsPath, []byte(modifiedUtilsContent))
	require.NoError(t, err)

	// Should have updated utils.js and affected main.js
	assert.Contains(t, result.UpdatedFiles, utilsFileID)
	assert.Greater(t, result.CascadeDepth, 0) // Should have cascade updates

	// Check stats reflect the dependency tracking
	stats := engine.IncrementalStats()
	assert.Equal(t, 2, stats["tracked_files"])
	assert.Greater(t, stats["dependency_edges"], 0)
}

// TestIncrementalEngine_DependencyGraph tests the incremental engine dependency graph.
func TestIncrementalEngine_DependencyGraph(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_deps_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Create dependency chain: main.go -> b/b.go -> c/c.go
	files := map[string][]byte{
		filepath.Join(tempDir, "main.go"): []byte(`package main

import (
	"./b"
	"./c"
)

func main() {
	b.FuncB()
	c.FuncC()
}
`),
		filepath.Join(tempDir, "b", "b.go"): []byte(`package b

import "../c"

func FuncB() {
	c.FuncC()
}
`),
		filepath.Join(tempDir, "c", "c.go"): []byte(`package c

func FuncC() {
	println("C")
}
`),
	}

	// Create directories
	err = os.MkdirAll(filepath.Join(tempDir, "b"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tempDir, "c"), 0755)
	require.NoError(t, err)

	// Add all files
	_, err = engine.BatchUpdate(files)
	require.NoError(t, err)

	// Link to establish dependencies
	err = engine.LinkSymbols()
	require.NoError(t, err)

	mainFileID := engine.GetOrCreateFileID(filepath.Join(tempDir, "main.go"))
	bFileID := engine.GetOrCreateFileID(filepath.Join(tempDir, "b", "b.go"))
	cFileID := engine.GetOrCreateFileID(filepath.Join(tempDir, "c", "c.go"))

	// Test dependency relationships
	mainDeps := engine.GetFileDependencies(mainFileID)
	assert.Contains(t, mainDeps, bFileID)
	assert.Contains(t, mainDeps, cFileID)

	bDeps := engine.GetFileDependencies(bFileID)
	assert.Contains(t, bDeps, cFileID)

	cDependents := engine.GetFileDependents(cFileID)
	assert.Contains(t, cDependents, bFileID)
	assert.Contains(t, cDependents, mainFileID)

	bDependents := engine.GetFileDependents(bFileID)
	assert.Contains(t, bDependents, mainFileID)

	// Modify c.go and check cascade
	modifiedCContent := `package c

func FuncC() {
	println("Modified C")
}

func NewFunc() {
	println("New function")
}
`

	result, err := engine.UpdateFile(filepath.Join(tempDir, "c", "c.go"), []byte(modifiedCContent))
	require.NoError(t, err)

	// Should have cascade effects
	assert.Contains(t, result.UpdatedFiles, cFileID)
	assert.Greater(t, result.CascadeDepth, 0)
}

// TestIncrementalEngine_UpdateTypes tests the incremental engine update types.
func TestIncrementalEngine_UpdateTypes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_types_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)
	filePath := filepath.Join(tempDir, "test.go")

	// Test Added
	result1, err := engine.UpdateFile(filePath, []byte(`package main
func main() {}`))
	require.NoError(t, err)

	// First update should be "added" type
	assert.Len(t, result1.UpdatedFiles, 1)

	// Test Modified
	result2, err := engine.UpdateFile(filePath, []byte(`package main
func main() { println("hello") }`))
	require.NoError(t, err)

	assert.Len(t, result2.UpdatedFiles, 1)

	// Test Removed
	result3, err := engine.RemoveFile(filePath)
	require.NoError(t, err)

	assert.Len(t, result3.UpdatedFiles, 1)
	assert.Greater(t, len(result3.RemovedSymbols), 0)
}

// TestIncrementalEngine_ErrorHandling tests the incremental engine error handling.
func TestIncrementalEngine_ErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_error_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Test removing non-existent file
	result, err := engine.RemoveFile("/nonexistent/file.go")
	require.NoError(t, err)
	assert.Len(t, result.UpdatedFiles, 0)

	// Test invalid file content (unsupported language)
	_, err = engine.UpdateFile(filepath.Join(tempDir, "test.unknown"), []byte("invalid content"))
	assert.Error(t, err)
}

// TestIncrementalEngine_PerformanceMetrics tests the incremental engine performance metrics.
func TestIncrementalEngine_PerformanceMetrics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "incremental_perf_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Add several files to track performance
	for i := 0; i < 10; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf(`package main

import "fmt"

func func%d() {
	fmt.Println("Function %d")
}
`, i, i)

		result, err := engine.UpdateFile(filePath, []byte(content))
		require.NoError(t, err)

		// Each update should complete reasonably quickly
		assert.Less(t, result.UpdateDuration, time.Second)
	}

	stats := engine.IncrementalStats()
	assert.Equal(t, 10, stats["tracked_files"])

	// Test batch update performance
	batchFiles := make(map[string][]byte)
	for i := 10; i < 20; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf(`package main

import "fmt"

func func%d() {
	fmt.Println("Function %d")
}
`, i, i)
		batchFiles[filePath] = []byte(content)
	}

	batchResult, err := engine.BatchUpdate(batchFiles)
	require.NoError(t, err)

	assert.Len(t, batchResult.UpdatedFiles, 10)
	assert.Less(t, batchResult.UpdateDuration, 5*time.Second) // Should be reasonably fast
}
