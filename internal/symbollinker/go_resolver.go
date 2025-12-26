package symbollinker

import (
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// GoResolver resolves Go module and package imports
type GoResolver struct {
	// projectRoot is the root directory of the project being indexed
	projectRoot string

	// goModPath is the path to go.mod if it exists
	goModPath string

	// moduleName is the module name from go.mod
	moduleName string

	// filePathToID maps absolute file paths to FileIDs
	filePathToID map[string]types.FileID

	// fileIDToPath maps FileIDs to absolute file paths
	fileIDToPath map[types.FileID]string

	// standardPackages contains Go standard library packages
	standardPackages map[string]bool

	// fileService is the centralized file service for all filesystem operations
	fileService *core.FileService
}

// NewGoResolver creates a new Go module resolver
func NewGoResolver(projectRoot string) *GoResolver {
	return NewGoResolverWithFileService(projectRoot, core.NewFileService())
}

// NewGoResolverWithFileService creates a new Go module resolver with a specific FileService
func NewGoResolverWithFileService(projectRoot string, fileService *core.FileService) *GoResolver {
	resolver := &GoResolver{
		projectRoot:      projectRoot,
		filePathToID:     make(map[string]types.FileID),
		fileIDToPath:     make(map[types.FileID]string),
		standardPackages: initStandardPackages(),
		fileService:      fileService,
	}

	// Try to find and parse go.mod
	resolver.findGoMod()

	return resolver
}

// RegisterFile registers a file with the resolver
func (gr *GoResolver) RegisterFile(fileID types.FileID, filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	gr.filePathToID[absPath] = fileID
	gr.fileIDToPath[fileID] = absPath
}

// SetFileRegistry sets the file ID registry for path resolution
func (gr *GoResolver) SetFileRegistry(registry map[string]types.FileID) {
	gr.filePathToID = registry

	// Build reverse mapping
	gr.fileIDToPath = make(map[types.FileID]string)
	for path, fileID := range registry {
		gr.fileIDToPath[fileID] = path
	}
}

// ResolveImport resolves an import path to a ModuleResolution
func (gr *GoResolver) ResolveImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: importPath,
	}

	// Check if it's a standard library package
	if gr.isStandardPackage(importPath) {
		resolution.ResolvedPath = importPath
		resolution.IsBuiltin = true
		resolution.Resolution = types.ResolutionBuiltin
		return resolution
	}

	// Check if it's a relative import (starts with ./ or ../)
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		return gr.resolveRelativeImport(importPath, fromFile)
	}

	// Check if it's a module import (matches our module name)
	if gr.moduleName != "" && strings.HasPrefix(importPath, gr.moduleName) {
		return gr.resolveModuleImport(importPath)
	}

	// Check vendor directory
	if vendorPath := gr.resolveVendorImport(importPath); vendorPath != "" {
		resolution.ResolvedPath = vendorPath
		if fileID, ok := gr.filePathToID[vendorPath]; ok {
			resolution.FileID = fileID
			resolution.Resolution = types.ResolutionModule
		} else {
			resolution.IsExternal = true
			resolution.Resolution = types.ResolutionExternal
		}
		return resolution
	}

	// External package (not in our project)
	resolution.IsExternal = true
	resolution.Resolution = types.ResolutionExternal
	resolution.ResolvedPath = importPath

	return resolution
}

// resolveRelativeImport resolves relative imports like "./package" or "../package"
func (gr *GoResolver) resolveRelativeImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: importPath,
	}

	// Get the directory of the importing file
	fromPath, ok := gr.fileIDToPath[fromFile]
	if !ok {
		resolution.Resolution = types.ResolutionNotFound
		return resolution
	}

	fromDir := filepath.Dir(fromPath)

	// Resolve the path relative to the importing file
	targetPath := filepath.Join(fromDir, importPath)
	targetPath = filepath.Clean(targetPath)

	// Check if it's a directory with Go files
	if gr.fileService.IsDir(targetPath) {
		// Look for Go files in the directory
		goFiles := gr.findGoFiles(targetPath)
		if len(goFiles) > 0 {
			resolution.ResolvedPath = targetPath
			// Use the first Go file as the representative
			if fileID, ok := gr.filePathToID[goFiles[0]]; ok {
				resolution.FileID = fileID
				resolution.Resolution = types.ResolutionDirectory
			} else {
				resolution.IsExternal = true
				resolution.Resolution = types.ResolutionExternal
			}
			return resolution
		}
	}

	// Try adding .go extension
	goFile := targetPath + ".go"
	if gr.fileService.Exists(goFile) {
		resolution.ResolvedPath = goFile
		if fileID, ok := gr.filePathToID[goFile]; ok {
			resolution.FileID = fileID
			resolution.Resolution = types.ResolutionFile
		} else {
			resolution.IsExternal = true
			resolution.Resolution = types.ResolutionExternal
		}
		return resolution
	}

	resolution.Resolution = types.ResolutionNotFound
	return resolution
}

// resolveModuleImport resolves imports that start with the module name
func (gr *GoResolver) resolveModuleImport(importPath string) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: importPath,
	}

	// Remove the module name prefix to get the relative path
	relativePath := strings.TrimPrefix(importPath, gr.moduleName)
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Resolve relative to project root
	targetPath := filepath.Join(gr.projectRoot, relativePath)
	targetPath = filepath.Clean(targetPath)

	// Check if it's a directory with Go files
	if gr.fileService.IsDir(targetPath) {
		goFiles := gr.findGoFiles(targetPath)
		if len(goFiles) > 0 {
			resolution.ResolvedPath = targetPath
			// Use the first Go file as the representative
			if fileID, ok := gr.filePathToID[goFiles[0]]; ok {
				resolution.FileID = fileID
				resolution.Resolution = types.ResolutionModule
			} else {
				// Still in our module but not indexed
				resolution.Resolution = types.ResolutionDirectory
			}
			return resolution
		}
	}

	// Check for internal packages
	if strings.Contains(relativePath, "/internal/") || strings.HasPrefix(relativePath, "internal/") {
		// Internal packages have special visibility rules
		// For now, we'll just mark them as found if they exist
		if gr.fileService.Exists(targetPath) {
			resolution.ResolvedPath = targetPath
			resolution.Resolution = types.ResolutionModule
			// Note: We should check if the importing package is allowed to import this internal package
			return resolution
		}
	}

	resolution.Resolution = types.ResolutionNotFound
	return resolution
}

// resolveVendorImport checks the vendor directory for an import
func (gr *GoResolver) resolveVendorImport(importPath string) string {
	vendorPath := filepath.Join(gr.projectRoot, "vendor", importPath)
	if gr.fileService.IsDir(vendorPath) {
		return vendorPath
	}
	return ""
}

// findGoMod finds and parses the go.mod file
func (gr *GoResolver) findGoMod() {
	// Start from project root and look for go.mod
	current := gr.projectRoot

	for {
		goModPath := filepath.Join(current, "go.mod")
		if gr.fileService.Exists(goModPath) {
			gr.goModPath = goModPath
			gr.parseGoMod()
			return
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root
			break
		}

		// Don't go above the project root
		if !strings.HasPrefix(parent, gr.projectRoot) && parent != gr.projectRoot {
			break
		}

		current = parent
	}
}

// parseGoMod parses the go.mod file to extract the module name
func (gr *GoResolver) parseGoMod() {
	if gr.goModPath == "" {
		return
	}

	// Try to load from FileService first
	if fileID := gr.fileService.GetFileIDForPath(gr.goModPath); fileID != 0 {
		if content, ok := gr.fileService.GetFileContent(fileID); ok {
			gr.parseGoModContent(content)
			return
		}
	}

	// Fallback to loading through FileService
	fileID, err := gr.fileService.LoadFile(gr.goModPath)
	if err != nil {
		return
	}

	content, ok := gr.fileService.GetFileContent(fileID)
	if !ok {
		return
	}

	gr.parseGoModContent(content)
}

// parseGoModContent parses the go.mod content to extract the module name
func (gr *GoResolver) parseGoModContent(content []byte) {
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			gr.moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			// Remove any version suffix
			if idx := strings.Index(gr.moduleName, " "); idx > 0 {
				gr.moduleName = gr.moduleName[:idx]
			}
			break
		}
	}
}

// findGoFiles finds all Go files in a directory
func (gr *GoResolver) findGoFiles(dir string) []string {
	var goFiles []string

	// First check file registry for in-memory files
	for path := range gr.filePathToID {
		if strings.HasPrefix(path, dir+"/") && strings.HasSuffix(path, ".go") {
			// Skip test files
			if !strings.HasSuffix(path, "_test.go") {
				goFiles = append(goFiles, path)
			}
		}
	}

	// If we found files in registry, return them
	if len(goFiles) > 0 {
		return goFiles
	}

	// Fallback to FileService for actual files
	dirGoFiles, err := gr.fileService.ListGoFiles(dir)
	if err != nil {
		return goFiles
	}

	goFiles = append(goFiles, dirGoFiles...)

	return goFiles
}

// isStandardPackage checks if an import path is a Go standard library package
func (gr *GoResolver) isStandardPackage(importPath string) bool {
	// Remove any subpackages
	basePkg := importPath
	if idx := strings.Index(importPath, "/"); idx > 0 {
		basePkg = importPath[:idx]
	}

	return gr.standardPackages[basePkg]
}

// GetModuleName returns the module name from go.mod
func (gr *GoResolver) GetModuleName() string {
	return gr.moduleName
}

// GetProjectRoot returns the project root directory
func (gr *GoResolver) GetProjectRoot() string {
	return gr.projectRoot
}

// ResolvePackagePath resolves a package path to a directory
func (gr *GoResolver) ResolvePackagePath(pkgPath string) string {
	// If it's a standard package, return as-is
	if gr.isStandardPackage(pkgPath) {
		return pkgPath
	}

	// If it's our module, resolve relative to project root
	if gr.moduleName != "" && strings.HasPrefix(pkgPath, gr.moduleName) {
		relativePath := strings.TrimPrefix(pkgPath, gr.moduleName)
		relativePath = strings.TrimPrefix(relativePath, "/")
		return filepath.Join(gr.projectRoot, relativePath)
	}

	// Check vendor directory
	vendorPath := filepath.Join(gr.projectRoot, "vendor", pkgPath)
	if gr.fileService.Exists(vendorPath) {
		return vendorPath
	}

	// Return the package path as-is (external)
	return pkgPath
}

// initStandardPackages initializes the set of Go standard library packages
func initStandardPackages() map[string]bool {
	return map[string]bool{
		"archive":   true,
		"bufio":     true,
		"builtin":   true,
		"bytes":     true,
		"compress":  true,
		"container": true,
		"context":   true,
		"crypto":    true,
		"database":  true,
		"debug":     true,
		"embed":     true,
		"encoding":  true,
		"errors":    true,
		"expvar":    true,
		"flag":      true,
		"fmt":       true,
		"go":        true,
		"hash":      true,
		"html":      true,
		"image":     true,
		"index":     true,
		"io":        true,
		"log":       true,
		"maps":      true,
		"math":      true,
		"mime":      true,
		"net":       true,
		"os":        true,
		"path":      true,
		"plugin":    true,
		"reflect":   true,
		"regexp":    true,
		"runtime":   true,
		"slices":    true,
		"sort":      true,
		"strconv":   true,
		"strings":   true,
		"sync":      true,
		"syscall":   true,
		"testing":   true,
		"text":      true,
		"time":      true,
		"unicode":   true,
		"unsafe":    true,

		// Special packages
		"C":        true, // CGO
		"internal": true, // Internal packages
	}
}
