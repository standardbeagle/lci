package symbollinker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/types"
)

// TestSymbolLinkerEngine_Basic tests the symbol linker engine basic.
func TestSymbolLinkerEngine_Basic(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "linker_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Test basic functionality
	assert.NotNil(t, engine)
	assert.Equal(t, tempDir, engine.rootPath)
	assert.Len(t, engine.extractors, 6) // Go, JS, TS, PHP, Python, C#

	// Test file ID creation
	testFile := filepath.Join(tempDir, "test.go")
	fileID1 := engine.GetOrCreateFileID(testFile)
	fileID2 := engine.GetOrCreateFileID(testFile)

	assert.Equal(t, fileID1, fileID2) // Same file should get same ID
	assert.Equal(t, testFile, engine.GetFilePath(fileID1))
}

// TestSymbolLinkerEngine_GoLinking tests the symbol linker engine go linking.
func TestSymbolLinkerEngine_GoLinking(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "linker_go_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Create Go files
	mainGo := `package main

import "fmt"
import "./utils"

func main() {
	fmt.Println("Hello")
	result := utils.Add(1, 2)
	fmt.Println(result)
}
`

	utilsGo := `package utils

func Add(a, b int) int {
	return a + b
}

func Multiply(x, y int) int {
	return x * y
}

var GlobalVar = 42
`

	// Write test files
	mainPath := filepath.Join(tempDir, "main.go")
	utilsDir := filepath.Join(tempDir, "utils")
	utilsPath := filepath.Join(utilsDir, "utils.go")

	require.NoError(t, os.WriteFile(mainPath, []byte(mainGo), 0644))
	require.NoError(t, os.MkdirAll(utilsDir, 0755))
	require.NoError(t, os.WriteFile(utilsPath, []byte(utilsGo), 0644))

	// Index files
	err = engine.IndexFile(mainPath, []byte(mainGo))
	require.NoError(t, err)

	err = engine.IndexFile(utilsPath, []byte(utilsGo))
	require.NoError(t, err)

	// Link symbols
	err = engine.LinkSymbols()
	require.NoError(t, err)

	// Verify file registration
	mainFileID := engine.GetOrCreateFileID(mainPath)
	utilsFileID := engine.GetOrCreateFileID(utilsPath)

	assert.NotEqual(t, mainFileID, utilsFileID)

	// Check imports for main.go
	imports, err := engine.GetFileImports(mainFileID)
	require.NoError(t, err)
	assert.Len(t, imports, 2) // fmt and ./utils

	// Check symbols in utils.go
	utilsSymbols, err := engine.GetSymbolsInFile(utilsFileID)
	require.NoError(t, err)
	assert.Greater(t, len(utilsSymbols), 0)

	// Verify specific symbols exist
	hasAdd := false
	hasMultiply := false
	hasGlobalVar := false

	for _, symbol := range utilsSymbols {
		switch symbol.Name {
		case "Add":
			hasAdd = true
			assert.Equal(t, types.SymbolKindFunction, symbol.Kind)
		case "Multiply":
			hasMultiply = true
			assert.Equal(t, types.SymbolKindFunction, symbol.Kind)
		case "GlobalVar":
			hasGlobalVar = true
			assert.Equal(t, types.SymbolKindVariable, symbol.Kind)
		}
	}

	assert.True(t, hasAdd, "Should find Add function")
	assert.True(t, hasMultiply, "Should find Multiply function")
	assert.True(t, hasGlobalVar, "Should find GlobalVar")

	// Check engine stats
	stats := engine.Stats()
	assert.Equal(t, 2, stats["files"])
	assert.Greater(t, stats["symbols"], 0)
	assert.Equal(t, 2, stats["import_links"])
}

// TestSymbolLinkerEngine_JSLinking tests the symbol linker engine j s linking.
func TestSymbolLinkerEngine_JSLinking(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "linker_js_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Create JS files
	indexJS := `import { formatDate, parseJSON } from './utils.js';
import config from './config.js';

export function processData(data) {
	const parsed = parseJSON(data);
	return {
		...parsed,
		timestamp: formatDate(new Date()),
		environment: config.env
	};
}

export default {
	process: processData
};
`

	utilsJS := `export function formatDate(date) {
	return date.toISOString();
}

export function parseJSON(text) {
	try {
		return JSON.parse(text);
	} catch (e) {
		return null;
	}
}

export const API_URL = 'http://localhost:3000';
`

	configJS := `const config = {
	env: 'development',
	debug: true
};

export default config;
`

	// Write test files
	indexPath := filepath.Join(tempDir, "index.js")
	utilsPath := filepath.Join(tempDir, "utils.js")
	configPath := filepath.Join(tempDir, "config.js")

	require.NoError(t, os.WriteFile(indexPath, []byte(indexJS), 0644))
	require.NoError(t, os.WriteFile(utilsPath, []byte(utilsJS), 0644))
	require.NoError(t, os.WriteFile(configPath, []byte(configJS), 0644))

	// Index files
	err = engine.IndexFile(indexPath, []byte(indexJS))
	require.NoError(t, err)

	err = engine.IndexFile(utilsPath, []byte(utilsJS))
	require.NoError(t, err)

	err = engine.IndexFile(configPath, []byte(configJS))
	require.NoError(t, err)

	// Link symbols
	err = engine.LinkSymbols()
	require.NoError(t, err)

	// Check imports for index.js
	indexFileID := engine.GetOrCreateFileID(indexPath)
	imports, err := engine.GetFileImports(indexFileID)
	require.NoError(t, err)
	assert.Len(t, imports, 2) // utils.js and config.js

	// Check utils symbols
	utilsFileID := engine.GetOrCreateFileID(utilsPath)
	utilsSymbols, err := engine.GetSymbolsInFile(utilsFileID)
	require.NoError(t, err)

	hasFormatDate := false
	hasParseJSON := false
	hasAPIURL := false

	for _, symbol := range utilsSymbols {
		switch symbol.Name {
		case "formatDate":
			hasFormatDate = true
			assert.Equal(t, types.SymbolKindFunction, symbol.Kind)
		case "parseJSON":
			hasParseJSON = true
			assert.Equal(t, types.SymbolKindFunction, symbol.Kind)
		case "API_URL":
			hasAPIURL = true
			assert.Equal(t, types.SymbolKindConstant, symbol.Kind)
		}
	}

	assert.True(t, hasFormatDate, "Should find formatDate function")
	assert.True(t, hasParseJSON, "Should find parseJSON function")
	assert.True(t, hasAPIURL, "Should find API_URL constant")

	// Check engine stats
	stats := engine.Stats()
	assert.Equal(t, 3, stats["files"])
	assert.Greater(t, stats["symbols"], 0)
	assert.Equal(t, 2, stats["import_links"])
}

// TestSymbolLinkerEngine_TypeScriptLinking tests the symbol linker engine type script linking.
func TestSymbolLinkerEngine_TypeScriptLinking(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "linker_ts_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Create TypeScript files
	indexTS := `import { User, UserService } from './types';
import { API_BASE_URL } from './config';

export class AppService {
	private userService: UserService;
	
	constructor() {
		this.userService = new UserService(API_BASE_URL);
	}
	
	async getUser(id: string): Promise<User> {
		return this.userService.fetchUser(id);
	}
}
`

	typesTS := `export interface User {
	id: string;
	name: string;
	email: string;
}

export class UserService {
	constructor(private baseUrl: string) {}
	
	async fetchUser(id: string): Promise<User> {
		const response = await fetch(` + "`${this.baseUrl}/users/${id}`" + `);
		return response.json();
	}
}

export type UserRole = 'admin' | 'user' | 'guest';
`

	configTS := `export const API_BASE_URL = 'https://api.example.com';
export const VERSION = '1.0.0';
`

	// Write test files
	indexPath := filepath.Join(tempDir, "index.ts")
	typesPath := filepath.Join(tempDir, "types.ts")
	configPath := filepath.Join(tempDir, "config.ts")

	require.NoError(t, os.WriteFile(indexPath, []byte(indexTS), 0644))
	require.NoError(t, os.WriteFile(typesPath, []byte(typesTS), 0644))
	require.NoError(t, os.WriteFile(configPath, []byte(configTS), 0644))

	// Index files
	err = engine.IndexFile(indexPath, []byte(indexTS))
	require.NoError(t, err)

	err = engine.IndexFile(typesPath, []byte(typesTS))
	require.NoError(t, err)

	err = engine.IndexFile(configPath, []byte(configTS))
	require.NoError(t, err)

	// Link symbols
	err = engine.LinkSymbols()
	require.NoError(t, err)

	// Check imports for index.ts
	indexFileID := engine.GetOrCreateFileID(indexPath)
	imports, err := engine.GetFileImports(indexFileID)
	require.NoError(t, err)
	assert.Len(t, imports, 2) // types and config

	// Check types symbols
	typesFileID := engine.GetOrCreateFileID(typesPath)
	typesSymbols, err := engine.GetSymbolsInFile(typesFileID)
	require.NoError(t, err)

	hasUser := false
	hasUserRole := false
	hasConstructor := false
	hasUserService := false

	for _, symbol := range typesSymbols {
		switch symbol.Name {
		case "User":
			hasUser = true
			assert.Equal(t, types.SymbolKindInterface, symbol.Kind)
		case "UserRole":
			hasUserRole = true
			assert.Equal(t, types.SymbolKindType, symbol.Kind)
		case "UserService":
			hasUserService = true
			assert.Equal(t, types.SymbolKindClass, symbol.Kind)
		case "constructor":
			hasConstructor = true
		}
	}

	assert.True(t, hasUser, "Should find User interface")
	assert.True(t, hasUserRole, "Should find UserRole type")
	assert.True(t, hasUserService, "Should find UserService class")
	assert.True(t, hasConstructor, "Should find constructor method")

	// Check engine stats
	stats := engine.Stats()
	assert.Equal(t, 3, stats["files"])
	assert.Greater(t, stats["symbols"], 0)
	assert.Equal(t, 2, stats["import_links"])
}

// TestSymbolLinkerEngine_ExtractorRegistration tests the symbol linker engine extractor registration.
func TestSymbolLinkerEngine_ExtractorRegistration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "linker_extractor_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Test default extractors (Go, JS, TypeScript, PHP, Python, C#)
	assert.Len(t, engine.extractors, 6)

	// Test extractor finding
	goExtractor := engine.findExtractor("test.go")
	assert.NotNil(t, goExtractor)
	assert.Equal(t, "go", goExtractor.GetLanguage())

	jsExtractor := engine.findExtractor("test.js")
	assert.NotNil(t, jsExtractor)
	assert.Equal(t, "javascript", jsExtractor.GetLanguage())

	tsExtractor := engine.findExtractor("test.ts")
	assert.NotNil(t, tsExtractor)
	assert.Equal(t, "typescript", tsExtractor.GetLanguage())

	unknownExtractor := engine.findExtractor("test.unknown")
	assert.Nil(t, unknownExtractor)
}

// TestSymbolLinkerEngine_FileIDManagement tests the symbol linker engine file i d management.
func TestSymbolLinkerEngine_FileIDManagement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "linker_fileid_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Test file ID creation and retrieval
	file1 := "/tmp/test1.go"
	file2 := "/tmp/test2.go"

	id1 := engine.GetOrCreateFileID(file1)
	id2 := engine.GetOrCreateFileID(file2)
	id1_again := engine.GetOrCreateFileID(file1)

	// IDs should be different for different files
	assert.NotEqual(t, id1, id2)
	// Same file should get same ID
	assert.Equal(t, id1, id1_again)

	// Path retrieval should work
	assert.Equal(t, file1, engine.GetFilePath(id1))
	assert.Equal(t, file2, engine.GetFilePath(id2))

	// Non-existent file ID should return empty string
	assert.Equal(t, "", engine.GetFilePath(types.FileID(999)))
}

// TestSymbolLinkerEngine_ErrorHandling tests the symbol linker engine error handling.
func TestSymbolLinkerEngine_ErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "linker_error_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Test indexing unsupported file type
	err = engine.IndexFile("/tmp/test.unknown", []byte("content"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no extractor found")

	// Test getting definition for non-existent symbol
	fakeSymbolID := types.NewCompositeSymbolID(types.FileID(999), 1)
	_, err = engine.GetSymbolDefinition(fakeSymbolID)
	assert.Error(t, err)

	// Test getting references for non-existent symbol
	_, err = engine.GetSymbolReferences(fakeSymbolID)
	assert.Error(t, err)

	// Test getting symbols for non-existent file
	_, err = engine.GetSymbolsInFile(types.FileID(999))
	assert.Error(t, err)

	// Test getting imports for non-existent file (should not error, return empty)
	imports, err := engine.GetFileImports(types.FileID(999))
	assert.NoError(t, err)
	assert.Len(t, imports, 0)
}
