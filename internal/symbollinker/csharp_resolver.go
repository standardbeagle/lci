package symbollinker

import (
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// CSharpResolver resolves C# using statements and assembly references
type CSharpResolver struct {
	// projectRoot is the root directory of the project being indexed
	projectRoot string

	// projectFiles stores .csproj and .sln file paths
	projectFiles []string

	// assemblies stores referenced assemblies and their paths
	assemblies map[string]string

	// globalUsings stores global using statements
	globalUsings []string

	// namespaceToAssembly maps namespaces to their containing assemblies
	namespaceToAssembly map[string]string

	// filePathToID maps absolute file paths to FileIDs
	filePathToID map[string]types.FileID

	// fileIDToPath maps FileIDs to absolute file paths
	fileIDToPath map[types.FileID]string

	// builtinNamespaces contains .NET built-in namespaces
	builtinNamespaces map[string]bool

	// fileService is the centralized file service for all filesystem operations
	fileService *core.FileService
}

// NewCSharpResolver creates a new C# module resolver
func NewCSharpResolver(projectRoot string) *CSharpResolver {
	return NewCSharpResolverWithFileService(projectRoot, core.NewFileService())
}

// NewCSharpResolverWithFileService creates a new C# module resolver with a specific FileService
func NewCSharpResolverWithFileService(projectRoot string, fileService *core.FileService) *CSharpResolver {
	resolver := &CSharpResolver{
		projectRoot:         projectRoot,
		projectFiles:        []string{},
		assemblies:          make(map[string]string),
		globalUsings:        []string{},
		namespaceToAssembly: make(map[string]string),
		filePathToID:        make(map[string]types.FileID),
		fileIDToPath:        make(map[types.FileID]string),
		builtinNamespaces:   initBuiltinDotNetNamespaces(),
		fileService:         fileService,
	}

	// Find project files and parse them
	resolver.findProjectFiles()
	resolver.loadBuiltinNamespaces()

	return resolver
}

// RegisterFile registers a file with the resolver
func (cr *CSharpResolver) RegisterFile(fileID types.FileID, filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	cr.filePathToID[absPath] = fileID
	cr.fileIDToPath[fileID] = absPath
}

// SetFileRegistry sets the file ID registry for path resolution
func (cr *CSharpResolver) SetFileRegistry(registry map[string]types.FileID) {
	cr.filePathToID = registry

	// Build reverse mapping
	cr.fileIDToPath = make(map[types.FileID]string)
	for path, fileID := range registry {
		cr.fileIDToPath[fileID] = path
	}
}

// ResolveImport resolves a C# using statement to a ModuleResolution
func (cr *CSharpResolver) ResolveImport(usingNamespace string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: usingNamespace,
	}

	// Handle different types of C# imports:
	// 1. Namespace using statements (System, System.Collections.Generic)
	// 2. Static using statements (using static System.Math)
	// 3. Alias using statements (using Alias = System.Collections.Generic.List<int>)

	// Check if it's a built-in .NET namespace
	if cr.isBuiltinNamespace(usingNamespace) {
		resolution.ResolvedPath = usingNamespace
		resolution.IsBuiltin = true
		resolution.Resolution = types.ResolutionBuiltin
		return resolution
	}

	// Try to resolve as project namespace
	if filePath := cr.resolveProjectNamespace(usingNamespace, fromFile); filePath != "" {
		return cr.checkResolvedPath(filePath, resolution)
	}

	// Try to resolve as referenced assembly namespace
	if assemblyPath := cr.resolveAssemblyNamespace(usingNamespace); assemblyPath != "" {
		resolution.ResolvedPath = assemblyPath
		resolution.IsExternal = true
		resolution.Resolution = types.ResolutionExternal
		return resolution
	}

	// Check if it matches any known namespace patterns
	if cr.isKnownNamespacePattern(usingNamespace) {
		resolution.ResolvedPath = usingNamespace
		resolution.IsExternal = true
		resolution.Resolution = types.ResolutionExternal
		return resolution
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "namespace not found"
	return resolution
}

// resolveProjectNamespace tries to resolve a namespace within the current project
func (cr *CSharpResolver) resolveProjectNamespace(namespace string, fromFile types.FileID) string {
	// In C#, namespaces often correspond to directory structure
	// Try to find files in directories that match the namespace

	namespaceParts := strings.Split(namespace, ".")

	// Try different directory patterns
	candidateDirs := []string{
		// Direct mapping: Namespace.Subnamespace -> Namespace/Subnamespace
		strings.Join(namespaceParts, string(filepath.Separator)),
		// Flat structure: all in project root
		"",
		// Common patterns
		filepath.Join("src", strings.Join(namespaceParts, string(filepath.Separator))),
		filepath.Join("lib", strings.Join(namespaceParts, string(filepath.Separator))),
	}

	for _, candidateDir := range candidateDirs {
		var searchDir string
		if candidateDir == "" {
			searchDir = cr.projectRoot
		} else {
			searchDir = filepath.Join(cr.projectRoot, candidateDir)
		}

		// Look for C# files in this directory
		if files := cr.findCSharpFilesInDir(searchDir); len(files) > 0 {
			// Return the first file found as the representative
			return files[0]
		}
	}

	return ""
}

// resolveAssemblyNamespace tries to resolve a namespace from referenced assemblies
func (cr *CSharpResolver) resolveAssemblyNamespace(namespace string) string {
	// Check if we have a direct mapping
	if assemblyPath, exists := cr.namespaceToAssembly[namespace]; exists {
		return assemblyPath
	}

	// Try partial namespace matches
	for ns, assemblyPath := range cr.namespaceToAssembly {
		if strings.HasPrefix(namespace, ns) {
			return assemblyPath
		}
	}

	return ""
}

// isBuiltinNamespace checks if a namespace is part of .NET built-ins
func (cr *CSharpResolver) isBuiltinNamespace(namespace string) bool {
	// Check exact match
	if cr.builtinNamespaces[namespace] {
		return true
	}

	// Check if it starts with known built-in prefixes
	builtinPrefixes := []string{
		"System",
		"Microsoft",
		"Windows",
	}

	for _, prefix := range builtinPrefixes {
		if strings.HasPrefix(namespace, prefix+".") || namespace == prefix {
			return true
		}
	}

	return false
}

// isKnownNamespacePattern checks if a namespace follows known patterns
func (cr *CSharpResolver) isKnownNamespacePattern(namespace string) bool {
	// Common third-party namespace patterns
	knownPatterns := []string{
		"Newtonsoft",
		"NUnit",
		"Moq",
		"AutoMapper",
		"FluentValidation",
		"Serilog",
		"NLog",
		"EntityFramework",
		"Dapper",
	}

	for _, pattern := range knownPatterns {
		if strings.HasPrefix(namespace, pattern) {
			return true
		}
	}

	return false
}

// checkResolvedPath checks if a resolved path exists and returns appropriate resolution
func (cr *CSharpResolver) checkResolvedPath(filePath string, resolution types.ModuleResolution) types.ModuleResolution {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		resolution.Resolution = types.ResolutionError
		resolution.Error = "invalid path"
		return resolution
	}

	// Check if file exists in our registry
	if fileID, exists := cr.filePathToID[absPath]; exists {
		resolution.ResolvedPath = absPath
		resolution.FileID = fileID
		resolution.Resolution = types.ResolutionInternal
		return resolution
	}

	// Check if file exists on filesystem
	if cr.fileService.Exists(absPath) {
		resolution.ResolvedPath = absPath
		resolution.IsExternal = true
		resolution.Resolution = types.ResolutionExternal
		return resolution
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "file not found"
	return resolution
}

// findProjectFiles finds .csproj, .sln, and other project files
func (cr *CSharpResolver) findProjectFiles() {
	// Look for project files in the root and subdirectories
	patterns := []string{
		"*.csproj",
		"*.sln",
		"*.fsproj", // F# project
		"*.vbproj", // VB.NET project
	}

	for _, pattern := range patterns {
		if files := cr.findFilesWithPattern(cr.projectRoot, pattern); len(files) > 0 {
			cr.projectFiles = append(cr.projectFiles, files...)
		}
	}

	// Parse found project files
	for _, projectFile := range cr.projectFiles {
		cr.parseProjectFile(projectFile)
	}
}

// findFilesWithPattern finds files matching a pattern
func (cr *CSharpResolver) findFilesWithPattern(dir, pattern string) []string {
	var files []string

	// This is a simplified implementation. In production, use proper file walking.
	// For now, check common locations
	commonPaths := []string{
		filepath.Join(dir, pattern),
	}

	for _, path := range commonPaths {
		matches, _ := filepath.Glob(path)
		files = append(files, matches...)
	}

	return files
}

// findCSharpFilesInDir finds C# files in a directory
func (cr *CSharpResolver) findCSharpFilesInDir(dir string) []string {
	var files []string

	// Check if directory exists
	if !cr.fileService.IsDir(dir) {
		return files
	}

	// Look for .cs files in the directory
	pattern := filepath.Join(dir, "*.cs")
	matches, _ := filepath.Glob(pattern)

	for _, match := range matches {
		if absPath, err := filepath.Abs(match); err == nil {
			files = append(files, absPath)
		}
	}

	return files
}

// parseProjectFile parses a .csproj or .sln file for references
func (cr *CSharpResolver) parseProjectFile(filePath string) {
	// Read project file through ContentStore for consistency
	fileID, err := cr.fileService.LoadFile(filePath)
	if err != nil {
		return
	}

	content, ok := cr.fileService.GetFileContent(fileID)
	if !ok {
		return
	}

	contentStr := string(content)

	// Look for PackageReference elements
	cr.parsePackageReferences(contentStr)

	// Look for ProjectReference elements
	cr.parseProjectReferences(contentStr, filepath.Dir(filePath))

	// Look for global using statements
	cr.parseGlobalUsings(contentStr)
}

// parsePackageReferences parses PackageReference elements from project file
func (cr *CSharpResolver) parsePackageReferences(content string) {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for PackageReference tags
		if strings.Contains(line, "<PackageReference") && strings.Contains(line, "Include=") {
			// Extract package name
			if start := strings.Index(line, "Include=\""); start != -1 {
				start += 9 // len("Include=\"")
				if end := strings.Index(line[start:], "\""); end != -1 {
					packageName := line[start : start+end]
					cr.assemblies[packageName] = "external_package"

					// Map common package namespaces
					cr.mapPackageNamespaces(packageName)
				}
			}
		}
	}
}

// parseProjectReferences parses ProjectReference elements from project file
func (cr *CSharpResolver) parseProjectReferences(content, baseDir string) {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for ProjectReference tags
		if strings.Contains(line, "<ProjectReference") && strings.Contains(line, "Include=") {
			// Extract project path
			if start := strings.Index(line, "Include=\""); start != -1 {
				start += 9 // len("Include=\"")
				if end := strings.Index(line[start:], "\""); end != -1 {
					projectPath := line[start : start+end]

					// Resolve relative path
					if !filepath.IsAbs(projectPath) {
						projectPath = filepath.Join(baseDir, projectPath)
					}

					if absPath, err := filepath.Abs(projectPath); err == nil {
						cr.assemblies[filepath.Base(projectPath)] = absPath
					}
				}
			}
		}
	}
}

// parseGlobalUsings parses global using statements from project file
func (cr *CSharpResolver) parseGlobalUsings(content string) {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for global using statements in .csproj
		if strings.Contains(line, "<Using") && strings.Contains(line, "Include=") {
			if start := strings.Index(line, "Include=\""); start != -1 {
				start += 9
				if end := strings.Index(line[start:], "\""); end != -1 {
					usingNamespace := line[start : start+end]
					cr.globalUsings = append(cr.globalUsings, usingNamespace)
				}
			}
		}
	}
}

// mapPackageNamespaces maps common NuGet packages to their namespaces
func (cr *CSharpResolver) mapPackageNamespaces(packageName string) {
	// Common package to namespace mappings
	packageMappings := map[string][]string{
		"Newtonsoft.Json":               {"Newtonsoft.Json"},
		"Microsoft.EntityFrameworkCore": {"Microsoft.EntityFrameworkCore"},
		"AutoMapper":                    {"AutoMapper"},
		"FluentValidation":              {"FluentValidation"},
		"Serilog":                       {"Serilog"},
		"NUnit":                         {"NUnit.Framework"},
		"xunit":                         {"Xunit"},
		"Moq":                           {"Moq"},
		"Dapper":                        {"Dapper"},
	}

	if namespaces, exists := packageMappings[packageName]; exists {
		for _, namespace := range namespaces {
			cr.namespaceToAssembly[namespace] = packageName
		}
	}
}

// loadBuiltinNamespaces loads common .NET built-in namespaces
func (cr *CSharpResolver) loadBuiltinNamespaces() {
	// Add common System namespaces that might not be in the static list
	systemNamespaces := []string{
		"System.Threading.Tasks",
		"System.Collections.Concurrent",
		"System.Text.Json",
		"System.Net.Http",
		"System.IO.Pipelines",
		"Microsoft.Extensions.DependencyInjection",
		"Microsoft.Extensions.Logging",
		"Microsoft.Extensions.Configuration",
		"Microsoft.AspNetCore",
	}

	for _, ns := range systemNamespaces {
		cr.builtinNamespaces[ns] = true
	}
}

// initBuiltinDotNetNamespaces initializes the map of built-in .NET namespaces
func initBuiltinDotNetNamespaces() map[string]bool {
	builtins := map[string]bool{
		// Core System namespaces
		"System":                     true,
		"System.Collections":         true,
		"System.Collections.Generic": true,
		"System.IO":                  true,
		"System.Text":                true,
		"System.Threading":           true,
		"System.Linq":                true,
		"System.Net":                 true,
		"System.Reflection":          true,
		"System.Runtime":             true,
		"System.ComponentModel":      true,
		"System.Globalization":       true,
		"System.Security":            true,
		"System.Diagnostics":         true,
		"System.Configuration":       true,

		// Microsoft namespaces
		"Microsoft.Extensions":          true,
		"Microsoft.AspNetCore":          true,
		"Microsoft.EntityFrameworkCore": true,

		// Windows-specific
		"Windows":                  true,
		"Windows.UI":               true,
		"Windows.ApplicationModel": true,
	}

	return builtins
}

// AddAssembly adds an assembly reference
func (cr *CSharpResolver) AddAssembly(name, path string) {
	cr.assemblies[name] = path
}

// AddNamespaceMapping adds a namespace to assembly mapping
func (cr *CSharpResolver) AddNamespaceMapping(namespace, assembly string) {
	cr.namespaceToAssembly[namespace] = assembly
}

// AddGlobalUsing adds a global using statement
func (cr *CSharpResolver) AddGlobalUsing(usingNamespace string) {
	cr.globalUsings = append(cr.globalUsings, usingNamespace)
}

// GetGlobalUsings returns all global using statements
func (cr *CSharpResolver) GetGlobalUsings() []string {
	return cr.globalUsings
}

// Stats returns statistics about the C# resolver
func (cr *CSharpResolver) Stats() map[string]interface{} {
	return map[string]interface{}{
		"project_root":       cr.projectRoot,
		"project_files":      len(cr.projectFiles),
		"assemblies":         len(cr.assemblies),
		"global_usings":      len(cr.globalUsings),
		"namespace_mappings": len(cr.namespaceToAssembly),
		"builtin_namespaces": len(cr.builtinNamespaces),
		"registered_files":   len(cr.filePathToID),
	}
}
