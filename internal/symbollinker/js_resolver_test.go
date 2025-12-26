package symbollinker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/types"
)

// TestJSResolver_RelativeImports tests the j s resolver relative imports.
func TestJSResolver_RelativeImports(t *testing.T) {
	// Create temporary test directory structure
	tempDir, err := os.MkdirTemp("", "js_resolver_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	createTestFile(t, filepath.Join(tempDir, "index.js"), "// main file")
	createTestFile(t, filepath.Join(tempDir, "utils.js"), "// utils")
	createTestFile(t, filepath.Join(tempDir, "lib", "helper.js"), "// helper")
	createTestFile(t, filepath.Join(tempDir, "lib", "index.js"), "// lib index")

	resolver := NewJSResolver(tempDir)

	// Set up file registry
	fileRegistry := map[string]types.FileID{
		filepath.Join(tempDir, "index.js"):      1,
		filepath.Join(tempDir, "utils.js"):      2,
		filepath.Join(tempDir, "lib/helper.js"): 3,
		filepath.Join(tempDir, "lib/index.js"):  4,
	}
	resolver.SetFileRegistry(fileRegistry)

	tests := []struct {
		name         string
		importPath   string
		fromFile     types.FileID
		expectedFile string
		expectedRes  types.ResolutionType
	}{
		{
			name:         "Relative file import",
			importPath:   "./utils",
			fromFile:     1, // index.js
			expectedFile: "utils.js",
			expectedRes:  types.ResolutionFile,
		},
		{
			name:         "Relative directory import",
			importPath:   "./lib",
			fromFile:     1, // index.js
			expectedFile: "lib/index.js",
			expectedRes:  types.ResolutionDirectory,
		},
		{
			name:         "Parent directory import",
			importPath:   "../utils",
			fromFile:     3, // lib/helper.js
			expectedFile: "utils.js",
			expectedRes:  types.ResolutionFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.ResolveImport(tt.importPath, tt.fromFile)

			assert.Equal(t, tt.expectedRes, result.Resolution)
			if tt.expectedFile != "" {
				expectedPath := filepath.Join(tempDir, tt.expectedFile)
				assert.Equal(t, expectedPath, result.ResolvedPath)
				assert.NotEqual(t, types.FileID(0), result.FileID)
			}
		})
	}
}

// TestJSResolver_BuiltinModules tests the j s resolver builtin modules.
func TestJSResolver_BuiltinModules(t *testing.T) {
	resolver := NewJSResolver("/tmp")

	// Set up file registry for builtin test
	fileRegistry := map[string]types.FileID{
		"/tmp/index.js": 1,
	}
	resolver.SetFileRegistry(fileRegistry)

	tests := []struct {
		name       string
		importPath string
	}{
		{"Node.js fs", "fs"},
		{"Node.js path", "path"},
		{"Node.js prefixed", "node:fs"},
		{"Node.js crypto", "crypto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.ResolveImport(tt.importPath, types.FileID(1))

			assert.Equal(t, types.ResolutionBuiltin, result.Resolution)
			assert.True(t, result.IsBuiltin)
			assert.True(t, result.IsExternal)
		})
	}
}

// TestJSResolver_PackageImports tests the j s resolver package imports.
func TestJSResolver_PackageImports(t *testing.T) {
	// Create temporary test directory structure with node_modules
	tempDir, err := os.MkdirTemp("", "js_resolver_package_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create project structure
	nodeModules := filepath.Join(tempDir, "node_modules")
	err = os.MkdirAll(nodeModules, 0755)
	require.NoError(t, err)

	// Create a test package
	lodashDir := filepath.Join(nodeModules, "lodash")
	err = os.MkdirAll(lodashDir, 0755)
	require.NoError(t, err)

	// Create package.json for lodash
	packageJSON := `{
		"name": "lodash",
		"version": "4.17.21",
		"main": "lodash.js"
	}`
	createTestFile(t, filepath.Join(lodashDir, "package.json"), packageJSON)
	createTestFile(t, filepath.Join(lodashDir, "lodash.js"), "// lodash implementation")

	// Create a scoped package
	typesDir := filepath.Join(nodeModules, "@types")
	nodeTypesDir := filepath.Join(typesDir, "node")
	err = os.MkdirAll(nodeTypesDir, 0755)
	require.NoError(t, err)

	typesPackageJSON := `{
		"name": "@types/node",
		"version": "18.0.0",
		"main": "index.d.ts"
	}`
	createTestFile(t, filepath.Join(nodeTypesDir, "package.json"), typesPackageJSON)
	createTestFile(t, filepath.Join(nodeTypesDir, "index.d.ts"), "// Node.js types")

	// Create main project file
	createTestFile(t, filepath.Join(tempDir, "index.js"), "// main file")

	resolver := NewJSResolver(tempDir)

	// Set up file registry
	fileRegistry := map[string]types.FileID{
		filepath.Join(tempDir, "index.js"): 1,
	}
	resolver.SetFileRegistry(fileRegistry)

	tests := []struct {
		name         string
		importPath   string
		expectedFile string
		isExternal   bool
	}{
		{
			name:         "Regular package",
			importPath:   "lodash",
			expectedFile: filepath.Join(lodashDir, "lodash.js"),
			isExternal:   true,
		},
		{
			name:         "Scoped package",
			importPath:   "@types/node",
			expectedFile: filepath.Join(nodeTypesDir, "index.d.ts"),
			isExternal:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.ResolveImport(tt.importPath, types.FileID(1))

			assert.Equal(t, types.ResolutionPackage, result.Resolution)
			assert.Equal(t, tt.expectedFile, result.ResolvedPath)
			assert.Equal(t, tt.isExternal, result.IsExternal)
		})
	}
}

// TestJSResolver_FileExtensions tests the j s resolver file extensions.
func TestJSResolver_FileExtensions(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "js_resolver_ext_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create files with different extensions
	createTestFile(t, filepath.Join(tempDir, "main.js"), "// main.js")
	createTestFile(t, filepath.Join(tempDir, "component.jsx"), "// component.jsx")
	createTestFile(t, filepath.Join(tempDir, "types.ts"), "// types.ts")
	createTestFile(t, filepath.Join(tempDir, "app.tsx"), "// app.tsx")
	createTestFile(t, filepath.Join(tempDir, "module.mjs"), "// module.mjs")
	createTestFile(t, filepath.Join(tempDir, "config.cjs"), "// config.cjs")
	createTestFile(t, filepath.Join(tempDir, "data.json"), `{"key": "value"}`)
	createTestFile(t, filepath.Join(tempDir, "index.js"), "// index.js")

	resolver := NewJSResolver(tempDir)

	// Set up file registry
	fileRegistry := map[string]types.FileID{
		filepath.Join(tempDir, "index.js"):      1,
		filepath.Join(tempDir, "main.js"):       2,
		filepath.Join(tempDir, "component.jsx"): 3,
		filepath.Join(tempDir, "types.ts"):      4,
		filepath.Join(tempDir, "app.tsx"):       5,
		filepath.Join(tempDir, "module.mjs"):    6,
		filepath.Join(tempDir, "config.cjs"):    7,
		filepath.Join(tempDir, "data.json"):     8,
	}
	resolver.SetFileRegistry(fileRegistry)

	tests := []struct {
		name         string
		importPath   string
		expectedFile string
	}{
		{"Import JS without extension", "./main", "main.js"},
		{"Import JSX without extension", "./component", "component.jsx"},
		{"Import TS without extension", "./types", "types.ts"},
		{"Import TSX without extension", "./app", "app.tsx"},
		{"Import MJS without extension", "./module", "module.mjs"},
		{"Import CJS without extension", "./config", "config.cjs"},
		{"Import JSON without extension", "./data", "data.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.ResolveImport(tt.importPath, types.FileID(1))

			assert.Equal(t, types.ResolutionFile, result.Resolution)
			expectedPath := filepath.Join(tempDir, tt.expectedFile)
			assert.Equal(t, expectedPath, result.ResolvedPath)
		})
	}
}

// TestJSResolver_PackageJSONFields tests the j s resolver package j s o n fields.
func TestJSResolver_PackageJSONFields(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "js_resolver_fields_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test different package.json configurations
	tests := []struct {
		name        string
		packageJSON string
		expected    string
	}{
		{
			name: "ESM package with module field",
			packageJSON: `{
				"name": "test-pkg",
				"type": "module",
				"main": "dist/index.cjs",
				"module": "dist/index.mjs"
			}`,
			expected: "dist/index.mjs",
		},
		{
			name: "CommonJS package with main",
			packageJSON: `{
				"name": "test-pkg",
				"main": "lib/index.js"
			}`,
			expected: "lib/index.js",
		},
		{
			name: "TypeScript package with types",
			packageJSON: `{
				"name": "test-pkg",
				"main": "dist/index.js",
				"types": "dist/index.d.ts"
			}`,
			expected: "dist/index.d.ts",
		},
		{
			name: "Package with typings field",
			packageJSON: `{
				"name": "test-pkg",
				"main": "dist/index.js",
				"typings": "types/index.d.ts"
			}`,
			expected: "types/index.d.ts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create package directory
			packageDir := filepath.Join(tempDir, "test-pkg")
			err := os.MkdirAll(packageDir, 0755)
			require.NoError(t, err)

			// Create package.json
			createTestFile(t, filepath.Join(packageDir, "package.json"), tt.packageJSON)

			// Create expected file
			expectedPath := filepath.Join(packageDir, tt.expected)
			err = os.MkdirAll(filepath.Dir(expectedPath), 0755)
			require.NoError(t, err)
			createTestFile(t, expectedPath, "// test content")

			resolver := NewJSResolver(tempDir)
			pkg := resolver.loadPackageJSON(packageDir)
			require.NotNil(t, pkg)

			entry := resolver.resolvePackageEntry(pkg, "", packageDir)
			assert.Equal(t, tt.expected, entry)
		})
	}
}

// TestJSResolver_NotFound tests the j s resolver not found.
func TestJSResolver_NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "js_resolver_notfound_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	createTestFile(t, filepath.Join(tempDir, "index.js"), "// index.js")

	resolver := NewJSResolver(tempDir)
	fileRegistry := map[string]types.FileID{
		filepath.Join(tempDir, "index.js"): 1,
	}
	resolver.SetFileRegistry(fileRegistry)

	tests := []string{
		"./nonexistent",
		"./nonexistent/file",
		"../outside",
		"nonexistent-package",
	}

	for _, importPath := range tests {
		t.Run(importPath, func(t *testing.T) {
			result := resolver.ResolveImport(importPath, types.FileID(1))
			assert.Equal(t, types.ResolutionNotFound, result.Resolution)
		})
	}
}

// Helper function to create test files
func createTestFile(t *testing.T, path, content string) {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}
