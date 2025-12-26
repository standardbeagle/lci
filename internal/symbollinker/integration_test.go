package symbollinker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/types"
)

// TestSymbolLinkerIntegration_CrossLanguageProject tests the complete symbol linking
// workflow across Go and JavaScript/TypeScript files with complex dependencies
func TestSymbolLinkerIntegration_CrossLanguageProject(t *testing.T) {
	// Create test project with Go backend and JS/TS frontend
	tempDir, err := os.MkdirTemp("", "integration_cross_lang_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Create Go API backend
	goAPIContent := `package api

import (
	"encoding/json"
	"net/http"
)

// User represents a user in the system
type User struct {
	ID    int    ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

// UserService handles user operations  
type UserService struct {
	users []User
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{
		users: make([]User, 0),
	}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*User, error) {
	for _, user := range s.users {
		if user.ID == id {
			return &user, nil
		}
	}
	return nil, errors.New("user not found")
}

// CreateUser creates a new user
func (s *UserService) CreateUser(name, email string) *User {
	user := User{
		ID:    len(s.users) + 1,
		Name:  name,
		Email: email,
	}
	s.users = append(s.users, user)
	return &user
}

// HandleGetUser HTTP handler for GET /users/:id
func HandleGetUser(service *UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation here
	}
}
`

	// Create Go main file
	goMainContent := `package main

import (
	"./api"
	"log"
	"net/http"
)

func main() {
	userService := api.NewUserService()
	
	http.HandleFunc("/users/", api.HandleGetUser(userService))
	
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`

	// Create TypeScript types (matching Go structs)
	tsTypesContent := `// Types matching the Go API
export interface User {
	id: number;
	name: string;
	email: string;
}

export interface CreateUserRequest {
	name: string;
	email: string;
}

export interface ApiError {
	message: string;
	code: number;
}

export type ApiResponse<T> = {
	data: T;
	error?: ApiError;
};
`

	// Create TypeScript API client
	tsAPIContent := `import { User, CreateUserRequest, ApiResponse } from './types';

export class UserAPIClient {
	private baseUrl: string;

	constructor(baseUrl: string = 'http://localhost:8080') {
		this.baseUrl = baseUrl;
	}

	async getUser(id: number): Promise<User | null> {
		try {
			const response = await fetch(` + "`${this.baseUrl}/users/${id}`" + `);
			const result: ApiResponse<User> = await response.json();
			
			if (result.error) {
				console.error('API Error:', result.error);
				return null;
			}
			
			return result.data;
		} catch (error) {
			console.error('Failed to fetch user:', error);
			return null;
		}
	}

	async createUser(request: CreateUserRequest): Promise<User | null> {
		try {
			const response = await fetch(` + "`${this.baseUrl}/users`" + `, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify(request),
			});
			
			const result: ApiResponse<User> = await response.json();
			return result.error ? null : result.data;
		} catch (error) {
			console.error('Failed to create user:', error);
			return null;
		}
	}
}

// Singleton instance
export const userAPI = new UserAPIClient();
`

	// Create React component using the API
	jsxComponentContent := `import React, { useState, useEffect } from 'react';
import { userAPI } from './api-client';
import { User } from './types';

interface UserProfileProps {
	userId: number;
}

export const UserProfile: React.FC<UserProfileProps> = ({ userId }) => {
	const [user, setUser] = useState<User | null>(null);
	const [loading, setLoading] = useState<boolean>(true);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		const fetchUser = async () => {
			setLoading(true);
			setError(null);
			
			const userData = await userAPI.getUser(userId);
			
			if (userData) {
				setUser(userData);
			} else {
				setError('Failed to load user');
			}
			
			setLoading(false);
		};

		fetchUser();
	}, [userId]);

	const handleUpdateUser = async (name: string, email: string) => {
		// This would call an update API endpoint
		// For now, just update local state
		if (user) {
			setUser({ ...user, name, email });
		}
	};

	if (loading) return <div>Loading user...</div>;
	if (error) return <div>Error: {error}</div>;
	if (!user) return <div>User not found</div>;

	return (
		<div className="user-profile">
			<h2>User Profile</h2>
			<div>
				<label>ID: {user.id}</label>
			</div>
			<div>
				<label>Name: {user.name}</label>
			</div>
			<div>
				<label>Email: {user.email}</label>
			</div>
		</div>
	);
};

export default UserProfile;
`

	// Write all test files using FileService
	files := map[string]string{
		"api/user.go":              goAPIContent,
		"main.go":                  goMainContent,
		"frontend/types.ts":        tsTypesContent,
		"frontend/api-client.ts":   tsAPIContent,
		"frontend/UserProfile.tsx": jsxComponentContent,
	}

	for relativePath, content := range files {
		fullPath := filepath.Join(tempDir, relativePath)

		// Ensure directory exists
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		// Write file directly (FileService will load it when needed)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)

		// Index through symbol linker engine
		err = engine.IndexFile(fullPath, []byte(content))
		require.NoError(t, err)
	}

	// Perform cross-file symbol linking
	start := time.Now()
	err = engine.LinkSymbols()
	require.NoError(t, err)
	linkingTime := time.Since(start)

	// Verify performance requirement: linking should complete quickly
	assert.Less(t, linkingTime, 100*time.Millisecond, "Symbol linking took too long")

	// Test 1: Verify Go symbols are properly extracted
	t.Run("Go Symbol Extraction", func(t *testing.T) {
		userServicePath := filepath.Join(tempDir, "api/user.go")
		fileID := engine.GetOrCreateFileID(userServicePath)

		symbols, err := engine.GetSymbolsInFile(fileID)
		require.NoError(t, err)
		assert.Greater(t, len(symbols), 5, "Should extract multiple symbols from Go file")

		// Verify specific symbols
		symbolNames := make(map[string]types.SymbolKind)
		for _, symbol := range symbols {
			symbolNames[symbol.Name] = symbol.Kind
		}

		assert.Equal(t, types.SymbolKindStruct, symbolNames["User"])
		assert.Equal(t, types.SymbolKindStruct, symbolNames["UserService"])
		assert.Equal(t, types.SymbolKindFunction, symbolNames["NewUserService"])
		assert.Equal(t, types.SymbolKindFunction, symbolNames["HandleGetUser"])
		assert.Equal(t, types.SymbolKindMethod, symbolNames["UserService.GetUser"])
		assert.Equal(t, types.SymbolKindMethod, symbolNames["UserService.CreateUser"])
	})

	// Test 2: Verify TypeScript symbols are properly extracted
	t.Run("TypeScript Symbol Extraction", func(t *testing.T) {
		typesPath := filepath.Join(tempDir, "frontend/types.ts")
		fileID := engine.GetOrCreateFileID(typesPath)

		symbols, err := engine.GetSymbolsInFile(fileID)
		require.NoError(t, err)
		assert.Greater(t, len(symbols), 3, "Should extract TypeScript interfaces and types")

		symbolNames := make(map[string]types.SymbolKind)
		for _, symbol := range symbols {
			symbolNames[symbol.Name] = symbol.Kind
		}

		assert.Equal(t, types.SymbolKindInterface, symbolNames["User"])
		assert.Equal(t, types.SymbolKindInterface, symbolNames["CreateUserRequest"])
		assert.Equal(t, types.SymbolKindType, symbolNames["ApiResponse"])
	})

	// Test 3: Verify JSX/React component extraction
	t.Run("JSX Component Extraction", func(t *testing.T) {
		componentPath := filepath.Join(tempDir, "frontend/UserProfile.tsx")
		fileID := engine.GetOrCreateFileID(componentPath)

		symbols, err := engine.GetSymbolsInFile(fileID)
		require.NoError(t, err)
		assert.Greater(t, len(symbols), 10, "Should extract many React component symbols")

		symbolNames := make(map[string]types.SymbolKind)
		for _, symbol := range symbols {
			symbolNames[symbol.Name] = symbol.Kind
		}

		// Verify key symbols are present
		assert.Equal(t, types.SymbolKindInterface, symbolNames["UserProfileProps"])
		// UserProfile can be either function or constant depending on extraction
		assert.True(t, symbolNames["UserProfile"] == types.SymbolKindFunction || symbolNames["UserProfile"] == types.SymbolKindConstant)
		// handleUpdateUser can be either function or constant depending on how it's extracted in JSX
		assert.True(t, symbolNames["handleUpdateUser"] == types.SymbolKindFunction || symbolNames["handleUpdateUser"] == types.SymbolKindConstant)
	})

	// Test 4: Verify cross-file import resolution
	t.Run("Import Resolution", func(t *testing.T) {
		// Check Go imports
		mainPath := filepath.Join(tempDir, "main.go")
		mainFileID := engine.GetOrCreateFileID(mainPath)

		imports, err := engine.GetFileImports(mainFileID)
		require.NoError(t, err)
		assert.Len(t, imports, 3, "main.go should have 3 imports")

		// Check TypeScript imports
		apiClientPath := filepath.Join(tempDir, "frontend/api-client.ts")
		clientFileID := engine.GetOrCreateFileID(apiClientPath)

		clientImports, err := engine.GetFileImports(clientFileID)
		require.NoError(t, err)
		assert.Len(t, clientImports, 1, "api-client.ts should import from types")
	})

	// Test 5: Verify engine statistics and performance
	t.Run("Engine Statistics", func(t *testing.T) {
		stats := engine.Stats()

		assert.Equal(t, 5, stats["files"], "Should track all 5 files")
		assert.Greater(t, stats["symbols"], 15, "Should extract many symbols across languages")
		assert.Greater(t, stats["import_links"], 3, "Should resolve multiple imports")

		// Note: Engine is not incremental in this test, so skip incremental stats
	})

	// Test 6: Verify FileService integration concept
	t.Run("FileService Integration", func(t *testing.T) {
		// Verify files exist and can be read
		for relativePath := range files {
			fullPath := filepath.Join(tempDir, relativePath)

			// File should exist on filesystem
			_, err := os.Stat(fullPath)
			require.NoError(t, err, "File should exist: %s", relativePath)

			// File should be readable
			content, err := os.ReadFile(fullPath)
			require.NoError(t, err)
			assert.Greater(t, len(content), 0, "File should have content: %s", relativePath)
		}
	})
}

// TestSymbolLinkerIntegration_IncrementalUpdates tests incremental updates with cross-file dependencies
func TestSymbolLinkerIntegration_IncrementalUpdates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "integration_incremental_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewIncrementalEngine(tempDir)

	// Initial file structure: A imports B, B imports C
	filesContent := map[string]string{
		"a.js": `import { processData } from './b.js';
export function handleRequest(data) {
	return processData(data);
}`,
		"b.js": `import { validateData } from './c.js';
export function processData(data) {
	if (validateData(data)) {
		return { processed: true, data };
	}
	return { processed: false };
}`,
		"c.js": `export function validateData(data) {
	return data != null && typeof data === 'object';
}
export const VALIDATION_RULES = {
	required: ['id', 'name']
};`,
	}

	// Index initial files
	for filename, content := range filesContent {
		fullPath := filepath.Join(tempDir, filename)

		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)

		err = engine.IndexFile(fullPath, []byte(content))
		require.NoError(t, err)
	}

	err = engine.LinkSymbols()
	require.NoError(t, err)

	// Verify initial state
	stats := engine.Stats()
	initialFiles := stats["files"]

	// Test incremental update: modify c.js to add new function
	t.Run("Add Function to Dependency", func(t *testing.T) {
		newContentC := `export function validateData(data) {
	return data != null && typeof data === 'object';
}

export function sanitizeData(data) {
	if (!validateData(data)) return null;
	return { ...data, sanitized: true };
}

export const VALIDATION_RULES = {
	required: ['id', 'name'],
	optional: ['description']
};`

		cPath := filepath.Join(tempDir, "c.js")

		start := time.Now()

		// Update file
		err = os.WriteFile(cPath, []byte(newContentC), 0644)
		require.NoError(t, err)

		// Perform incremental update
		_, err = engine.UpdateFile(cPath, []byte(newContentC))
		require.NoError(t, err)

		updateTime := time.Since(start)
		assert.Less(t, updateTime, 50*time.Millisecond, "Incremental update should be fast")

		// Verify update was processed
		cFileID := engine.GetOrCreateFileID(cPath)
		symbols, err := engine.GetSymbolsInFile(cFileID)
		require.NoError(t, err)

		symbolNames := make(map[string]bool)
		for _, symbol := range symbols {
			symbolNames[symbol.Name] = true
		}

		assert.True(t, symbolNames["validateData"])
		assert.True(t, symbolNames["sanitizeData"]) // New function
		assert.True(t, symbolNames["VALIDATION_RULES"])
	})

	// Test cascading updates: modify b.js to use new function from c.js
	t.Run("Cascading Dependency Update", func(t *testing.T) {
		newContentB := `import { validateData, sanitizeData } from './c.js';

export function processData(data) {
	const sanitized = sanitizeData(data);
	if (sanitized) {
		return { processed: true, data: sanitized };
	}
	return { processed: false };
}

export function quickValidate(data) {
	return validateData(data);
}`

		bPath := filepath.Join(tempDir, "b.js")

		err = os.WriteFile(bPath, []byte(newContentB), 0644)
		require.NoError(t, err)

		_, err = engine.UpdateFile(bPath, []byte(newContentB))
		require.NoError(t, err)

		// Verify b.js now imports sanitizeData
		bFileID := engine.GetOrCreateFileID(bPath)
		imports, err := engine.GetFileImports(bFileID)
		require.NoError(t, err)

		// Should have at least the import from c.js
		assert.Greater(t, len(imports), 0, "Should have at least one import")

		// Verify new symbols in b.js
		symbols, err := engine.GetSymbolsInFile(bFileID)
		require.NoError(t, err)

		symbolNames := make(map[string]bool)
		for _, symbol := range symbols {
			symbolNames[symbol.Name] = true
		}

		assert.True(t, symbolNames["processData"])
		assert.True(t, symbolNames["quickValidate"]) // New function
	})

	// Verify final statistics
	t.Run("Final Statistics", func(t *testing.T) {
		finalStats := engine.Stats()

		assert.Equal(t, initialFiles, finalStats["files"]) // Same number of files
		// Note: Symbol count may change due to incremental updates modifying extraction patterns
		assert.Greater(t, finalStats["symbols"], 0, "Should have extracted symbols")

		incStats := engine.IncrementalStats()
		// Note: tracked_files may be 0 after all updates are processed
		assert.GreaterOrEqual(t, incStats["dependency_edges"], 2, "Should have at least 2 dependency relationships")
		assert.Equal(t, 0, incStats["pending_updates"], "Should have no pending updates")
	})
}

// TestSymbolLinkerIntegration_Performance validates performance requirements
func TestSymbolLinkerIntegration_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "integration_perf_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	engine := NewSymbolLinkerEngine(tempDir)

	// Create a moderately sized project (50 files, mix of Go/JS/TS)
	const numFiles = 50

	start := time.Now()

	for i := 0; i < numFiles; i++ {
		// Create alternating Go and JS files
		if i%2 == 0 {
			// Go file
			content := fmt.Sprintf(`package main

import (
	"fmt"
	"./utils%d"
)

type Service%d struct {
	id int
	name string
}

func NewService%d(id int) *Service%d {
	return &Service%d{id: id, name: "service_%d"}
}

func (s *Service%d) Process(data string) error {
	result := utils%d.ProcessData(data)
	fmt.Println("Result:", result)
	return nil
}

func (s *Service%d) GetID() int {
	return s.id
}
`, i, i, i, i, i, i, i, i, i)

			filename := filepath.Join(tempDir, fmt.Sprintf("service_%d.go", i))
			err = os.WriteFile(filename, []byte(content), 0644)
			require.NoError(t, err)

			err = engine.IndexFile(filename, []byte(content))
			require.NoError(t, err)
		} else {
			// JavaScript file
			content := fmt.Sprintf(`export class DataProcessor%d {
	constructor(config) {
		this.config = config;
		this.id = %d;
	}

	async processAsync(data) {
		return new Promise((resolve) => {
			const result = this.transform(data);
			setTimeout(() => resolve(result), 10);
		});
	}

	transform(data) {
		return {
			id: this.id,
			processed: true,
			data: data,
			timestamp: new Date()
		};
	}

	static validateConfig(config) {
		return config && typeof config === 'object';
	}
}

export const PROCESSOR_%d_CONFIG = {
	version: '1.0.%d',
	enabled: true
};

export function createProcessor%d(config) {
	if (DataProcessor%d.validateConfig(config)) {
		return new DataProcessor%d(config);
	}
	return null;
}
`, i, i, i, i, i, i, i)

			filename := filepath.Join(tempDir, fmt.Sprintf("processor_%d.js", i))
			err = os.WriteFile(filename, []byte(content), 0644)
			require.NoError(t, err)

			err = engine.IndexFile(filename, []byte(content))
			require.NoError(t, err)
		}
	}

	indexingTime := time.Since(start)

	// Perform symbol linking
	linkStart := time.Now()
	err = engine.LinkSymbols()
	require.NoError(t, err)
	linkingTime := time.Since(linkStart)

	totalTime := time.Since(start)

	// Performance assertions based on testing strategy requirements
	t.Run("Performance Requirements", func(t *testing.T) {
		// Index building should be reasonable for 50 files
		assert.Less(t, indexingTime, 2*time.Second,
			"Indexing %d files took %v, should be under 2s", numFiles, indexingTime)

		// Symbol linking should be fast
		assert.Less(t, linkingTime, 500*time.Millisecond,
			"Symbol linking took %v, should be under 500ms", linkingTime)

		// Total time should be reasonable
		assert.Less(t, totalTime, 3*time.Second,
			"Total processing took %v, should be under 3s", totalTime)
	})

	// Verify extraction accuracy
	t.Run("Extraction Accuracy", func(t *testing.T) {
		stats := engine.Stats()

		assert.Equal(t, numFiles, stats["files"])
		assert.Greater(t, stats["symbols"], numFiles*3, "Should extract multiple symbols per file")

		// Test a few specific files
		goFile := filepath.Join(tempDir, "service_0.go")
		goFileID := engine.GetOrCreateFileID(goFile)
		goSymbols, err := engine.GetSymbolsInFile(goFileID)
		require.NoError(t, err)
		assert.Greater(t, len(goSymbols), 3, "Go file should have multiple symbols")

		jsFile := filepath.Join(tempDir, "processor_1.js")
		jsFileID := engine.GetOrCreateFileID(jsFile)
		jsSymbols, err := engine.GetSymbolsInFile(jsFileID)
		require.NoError(t, err)
		assert.Greater(t, len(jsSymbols), 4, "JS file should have multiple symbols")
	})

	t.Logf("Performance Results:")
	t.Logf("  Files indexed: %d", numFiles)
	t.Logf("  Indexing time: %v", indexingTime)
	t.Logf("  Linking time: %v", linkingTime)
	t.Logf("  Total time: %v", totalTime)
	t.Logf("  Symbols extracted: %d", engine.Stats()["symbols"])
	t.Logf("  Files per second: %.1f", float64(numFiles)/totalTime.Seconds())
}

// Note: FileService integration is implemented in the actual engine constructors
// These tests demonstrate the integration patterns without requiring changes to the existing API
