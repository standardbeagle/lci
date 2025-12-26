package symbollinker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// JSResolver handles JavaScript/TypeScript module resolution
// It supports both ESM (ES6 modules) and CommonJS resolution patterns
type JSResolver struct {
	rootPath     string
	packageJSONs map[string]*PackageJSON // Cache of package.json files
	fileRegistry map[string]types.FileID // Path to FileID mapping
}

// PackageJSON represents a package.json file structure
type PackageJSON struct {
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	Main            string                 `json:"main"`
	Module          string                 `json:"module"`
	Types           string                 `json:"types"`
	Typings         string                 `json:"typings"`
	Exports         map[string]interface{} `json:"exports"`
	Dependencies    map[string]string      `json:"dependencies"`
	DevDependencies map[string]string      `json:"devDependencies"`
	Type            string                 `json:"type"` // "module" for ESM, "commonjs" or empty for CommonJS
}

// NewJSResolver creates a new JavaScript module resolver
func NewJSResolver(rootPath string) *JSResolver {
	return &JSResolver{
		rootPath:     rootPath,
		packageJSONs: make(map[string]*PackageJSON),
		fileRegistry: make(map[string]types.FileID),
	}
}

// SetFileRegistry sets the file ID registry for path resolution
func (jr *JSResolver) SetFileRegistry(registry map[string]types.FileID) {
	jr.fileRegistry = registry
}

// ResolveImport resolves a JavaScript/TypeScript import path
func (jr *JSResolver) ResolveImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	// Get the directory of the importing file
	fromPath := jr.getFilePathFromID(fromFile)
	if fromPath == "" {
		return types.ModuleResolution{
			RequestPath: importPath,
			Resolution:  types.ResolutionNotFound,
		}
	}

	fromDir := filepath.Dir(fromPath)

	// Handle different import patterns
	switch {
	case isRelativeImport(importPath):
		return jr.resolveRelativeImport(importPath, fromDir)
	case isAbsoluteImport(importPath):
		return jr.resolveAbsoluteImport(importPath, fromDir)
	case isBuiltinModule(importPath):
		return types.ModuleResolution{
			RequestPath: importPath,
			IsBuiltin:   true,
			IsExternal:  true,
			Resolution:  types.ResolutionBuiltin,
		}
	default:
		// This is likely a package import, but we need to check if it exists
		// If no node_modules found, it's external
		return jr.resolvePackageImport(importPath, fromDir)
	}
}

// resolveRelativeImport handles relative imports (./xxx, ../xxx)
func (jr *JSResolver) resolveRelativeImport(importPath, fromDir string) types.ModuleResolution {
	targetPath := filepath.Join(fromDir, importPath)
	targetPath = filepath.Clean(targetPath)

	// Try direct file resolution with extensions
	if resolved := jr.tryResolveFile(targetPath); resolved.Resolution != types.ResolutionNotFound {
		return resolved
	}

	// Try directory resolution (index files)
	if resolved := jr.tryResolveDirectory(targetPath); resolved.Resolution != types.ResolutionNotFound {
		return resolved
	}

	return types.ModuleResolution{
		RequestPath: importPath,
		Resolution:  types.ResolutionNotFound,
	}
}

// resolveAbsoluteImport handles absolute imports from project root
func (jr *JSResolver) resolveAbsoluteImport(importPath, fromDir string) types.ModuleResolution {
	// Remove leading slash
	cleanPath := strings.TrimPrefix(importPath, "/")
	targetPath := filepath.Join(jr.rootPath, cleanPath)

	// Try direct file resolution
	if resolved := jr.tryResolveFile(targetPath); resolved.Resolution != types.ResolutionNotFound {
		return resolved
	}

	// Try directory resolution
	if resolved := jr.tryResolveDirectory(targetPath); resolved.Resolution != types.ResolutionNotFound {
		return resolved
	}

	return types.ModuleResolution{
		RequestPath: importPath,
		Resolution:  types.ResolutionNotFound,
	}
}

// resolvePackageImport handles package imports (lodash, react, @types/node)
func (jr *JSResolver) resolvePackageImport(importPath, fromDir string) types.ModuleResolution {
	// Find nearest node_modules
	nodeModulesPath := jr.findNodeModules(fromDir)
	if nodeModulesPath == "" {
		return types.ModuleResolution{
			RequestPath: importPath,
			IsExternal:  true,
			Resolution:  types.ResolutionNotFound, // Changed from ResolutionExternal to ResolutionNotFound
		}
	}

	// Handle scoped packages (@scope/package)
	var packageDir string
	if strings.HasPrefix(importPath, "@") {
		parts := strings.SplitN(importPath, "/", 3)
		if len(parts) >= 2 {
			packageDir = filepath.Join(nodeModulesPath, parts[0], parts[1])
		}
	} else {
		// Regular package
		parts := strings.SplitN(importPath, "/", 2)
		packageDir = filepath.Join(nodeModulesPath, parts[0])
	}

	if packageDir == "" {
		return types.ModuleResolution{
			RequestPath: importPath,
			IsExternal:  true,
			Resolution:  types.ResolutionExternal,
		}
	}

	// Check if package exists
	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		return types.ModuleResolution{
			RequestPath: importPath,
			IsExternal:  true,
			Resolution:  types.ResolutionNotFound, // Changed from ResolutionExternal
		}
	}

	// Load package.json
	packageJSON := jr.loadPackageJSON(packageDir)
	if packageJSON == nil {
		// No package.json, try index files
		if resolved := jr.tryResolveDirectory(packageDir); resolved.Resolution != types.ResolutionNotFound {
			resolved.IsExternal = true // Packages are always external
			return resolved
		}

		return types.ModuleResolution{
			RequestPath: importPath,
			IsExternal:  true,
			Resolution:  types.ResolutionNotFound, // Changed from ResolutionExternal
		}
	}

	// Resolve using package.json fields
	entryPoint := jr.resolvePackageEntry(packageJSON, importPath, packageDir)
	if entryPoint == "" {
		return types.ModuleResolution{
			RequestPath: importPath,
			IsExternal:  true,
			Resolution:  types.ResolutionExternal,
		}
	}

	resolvedPath := filepath.Join(packageDir, entryPoint)
	fileID := jr.getFileID(resolvedPath)

	return types.ModuleResolution{
		RequestPath:  importPath,
		ResolvedPath: resolvedPath,
		FileID:       fileID,
		IsExternal:   true, // Packages are always external
		Resolution:   types.ResolutionPackage,
	}
}

// tryResolveFile attempts to resolve a file with various extensions
func (jr *JSResolver) tryResolveFile(basePath string) types.ModuleResolution {
	extensions := []string{"", ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs", ".json", ".d.ts"}

	for _, ext := range extensions {
		fullPath := basePath + ext

		// First check if file exists in our registry (for in-memory files)
		if fileID := jr.getFileID(fullPath); fileID != 0 {
			return types.ModuleResolution{
				RequestPath:  basePath,
				ResolvedPath: fullPath,
				FileID:       fileID,
				IsExternal:   !jr.isWithinProject(fullPath),
				Resolution:   types.ResolutionFile,
			}
		}

		// Fallback to filesystem check for actual files
		if stat, err := os.Stat(fullPath); err == nil {
			// Make sure it's actually a file, not a directory
			if !stat.IsDir() {
				fileID := jr.getFileID(fullPath)
				return types.ModuleResolution{
					RequestPath:  basePath,
					ResolvedPath: fullPath,
					FileID:       fileID,
					IsExternal:   !jr.isWithinProject(fullPath),
					Resolution:   types.ResolutionFile,
				}
			}
		}
	}

	return types.ModuleResolution{
		RequestPath: basePath,
		Resolution:  types.ResolutionNotFound,
	}
}

// tryResolveDirectory attempts to resolve a directory by looking for index files
func (jr *JSResolver) tryResolveDirectory(dirPath string) types.ModuleResolution {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return types.ModuleResolution{
			RequestPath: dirPath,
			Resolution:  types.ResolutionNotFound,
		}
	}

	// Check for package.json in directory
	packageJSON := jr.loadPackageJSON(dirPath)
	if packageJSON != nil {
		entryPoint := jr.resolvePackageEntry(packageJSON, "", dirPath)
		if entryPoint != "" {
			resolvedPath := filepath.Join(dirPath, entryPoint)
			if _, err := os.Stat(resolvedPath); err == nil {
				fileID := jr.getFileID(resolvedPath)
				return types.ModuleResolution{
					RequestPath:  dirPath,
					ResolvedPath: resolvedPath,
					FileID:       fileID,
					IsExternal:   !jr.isWithinProject(resolvedPath),
					Resolution:   types.ResolutionDirectory,
				}
			}
		}
	}

	// Try index files
	indexFiles := []string{"index.js", "index.ts", "index.jsx", "index.tsx", "index.mjs", "index.cjs", "index.json", "index.d.ts"}

	for _, indexFile := range indexFiles {
		indexPath := filepath.Join(dirPath, indexFile)
		if _, err := os.Stat(indexPath); err == nil {
			fileID := jr.getFileID(indexPath)
			return types.ModuleResolution{
				RequestPath:  dirPath,
				ResolvedPath: indexPath,
				FileID:       fileID,
				IsExternal:   !jr.isWithinProject(indexPath),
				Resolution:   types.ResolutionDirectory,
			}
		}
	}

	return types.ModuleResolution{
		RequestPath: dirPath,
		Resolution:  types.ResolutionNotFound,
	}
}

// loadPackageJSON loads and caches a package.json file
func (jr *JSResolver) loadPackageJSON(dir string) *PackageJSON {
	packagePath := filepath.Join(dir, "package.json")

	// Check cache
	if pkg, exists := jr.packageJSONs[packagePath]; exists {
		return pkg
	}

	// Load from file
	data, err := os.ReadFile(packagePath)
	if err != nil {
		jr.packageJSONs[packagePath] = nil // Cache negative result
		return nil
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		jr.packageJSONs[packagePath] = nil // Cache negative result
		return nil
	}

	jr.packageJSONs[packagePath] = &pkg
	return &pkg
}

// resolvePackageEntry determines the entry point for a package
func (jr *JSResolver) resolvePackageEntry(pkg *PackageJSON, importPath, packageDir string) string {
	// Handle exports field (modern Node.js)
	if pkg.Exports != nil {
		if entry := jr.resolveExportsField(pkg.Exports, importPath); entry != "" {
			return entry
		}
	}

	// Handle TypeScript types
	if pkg.Types != "" {
		return pkg.Types
	}
	if pkg.Typings != "" {
		return pkg.Typings
	}

	// Handle ESM vs CommonJS
	if pkg.Type == "module" {
		// ESM package
		if pkg.Module != "" {
			return pkg.Module
		}
		if pkg.Main != "" {
			return pkg.Main
		}
	} else {
		// CommonJS package (default)
		if pkg.Main != "" {
			return pkg.Main
		}
	}

	// Default fallback
	return "index.js"
}

// resolveExportsField handles the complex exports field resolution
func (jr *JSResolver) resolveExportsField(exports map[string]interface{}, importPath string) string {
	// This is a simplified implementation of Node.js exports resolution
	// The full spec is quite complex and handles conditional exports

	if entry, ok := exports["."].(string); ok {
		return entry
	}

	// Try direct string export
	if len(exports) == 1 {
		for _, value := range exports {
			if str, ok := value.(string); ok {
				return str
			}
		}
	}

	return ""
}

// findNodeModules finds the nearest node_modules directory
func (jr *JSResolver) findNodeModules(fromDir string) string {
	dir := fromDir

	for {
		nodeModules := filepath.Join(dir, "node_modules")
		if _, err := os.Stat(nodeModules); err == nil {
			return nodeModules
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		dir = parent
	}

	return ""
}

// Helper functions

// isRelativeImport checks if an import is relative (./ or ../)
func isRelativeImport(importPath string) bool {
	return strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../")
}

// isAbsoluteImport checks if an import is absolute (starts with /)
func isAbsoluteImport(importPath string) bool {
	return strings.HasPrefix(importPath, "/")
}

// isBuiltinModule checks if a module is a Node.js builtin
func isBuiltinModule(importPath string) bool {
	builtins := []string{
		"assert", "buffer", "child_process", "cluster", "crypto", "dgram",
		"dns", "domain", "events", "fs", "http", "https", "net", "os",
		"path", "punycode", "querystring", "readline", "repl", "stream",
		"string_decoder", "tls", "tty", "url", "util", "vm", "zlib",
		"constants", "module", "process", "timers", "console",

		// Node.js prefixed imports
		"node:assert", "node:buffer", "node:child_process", "node:cluster",
		"node:crypto", "node:dgram", "node:dns", "node:domain", "node:events",
		"node:fs", "node:http", "node:https", "node:net", "node:os", "node:path",
		"node:punycode", "node:querystring", "node:readline", "node:repl",
		"node:stream", "node:string_decoder", "node:tls", "node:tty", "node:url",
		"node:util", "node:vm", "node:zlib", "node:constants", "node:module",
		"node:process", "node:timers", "node:console",
	}

	for _, builtin := range builtins {
		if importPath == builtin {
			return true
		}
	}

	return false
}

// getFileID gets or creates a FileID for a path
func (jr *JSResolver) getFileID(path string) types.FileID {
	if fileID, exists := jr.fileRegistry[path]; exists {
		return fileID
	}
	return 0 // Unknown file
}

// getFilePathFromID gets the file path for a FileID
func (jr *JSResolver) getFilePathFromID(fileID types.FileID) string {
	for path, id := range jr.fileRegistry {
		if id == fileID {
			return path
		}
	}
	return ""
}

// isWithinProject checks if a path is within the project root
func (jr *JSResolver) isWithinProject(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absRoot, err := filepath.Abs(jr.rootPath)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..") && !strings.HasPrefix(rel, "/")
}
