package symbollinker

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

// PHPResolver resolves PHP include/require statements and namespace imports
type PHPResolver struct {
	// projectRoot is the root directory of the project being indexed
	projectRoot string

	// composerJsonPath is the path to composer.json if it exists
	composerJsonPath string

	// autoloadConfig stores PSR-4 and PSR-0 autoload configuration
	autoloadConfig *PHPAutoloadConfig

	// filePathToID maps absolute file paths to FileIDs
	filePathToID map[string]types.FileID

	// fileIDToPath maps FileIDs to absolute file paths
	fileIDToPath map[types.FileID]string

	// includePaths stores additional include paths
	includePaths []string

	// fileService is the centralized file service for all filesystem operations
	fileService *core.FileService
}

// PHPAutoloadConfig stores PHP autoload configuration
type PHPAutoloadConfig struct {
	PSR4     map[string]string // namespace prefix -> directory path
	PSR0     map[string]string // namespace prefix -> directory path
	ClassMap map[string]string // class name -> file path
	Files    []string          // files to include
}

// NewPHPResolver creates a new PHP module resolver
func NewPHPResolver(projectRoot string) *PHPResolver {
	return NewPHPResolverWithFileService(projectRoot, core.NewFileService())
}

// NewPHPResolverWithFileService creates a new PHP module resolver with a specific FileService
func NewPHPResolverWithFileService(projectRoot string, fileService *core.FileService) *PHPResolver {
	resolver := &PHPResolver{
		projectRoot:  projectRoot,
		filePathToID: make(map[string]types.FileID),
		fileIDToPath: make(map[types.FileID]string),
		includePaths: []string{"."},
		autoloadConfig: &PHPAutoloadConfig{
			PSR4:     make(map[string]string),
			PSR0:     make(map[string]string),
			ClassMap: make(map[string]string),
			Files:    []string{},
		},
		fileService: fileService,
	}

	// Try to find and parse composer.json
	resolver.findComposerJson()

	return resolver
}

// RegisterFile registers a file with the resolver
func (pr *PHPResolver) RegisterFile(fileID types.FileID, filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	pr.filePathToID[absPath] = fileID
	pr.fileIDToPath[fileID] = absPath
}

// SetFileRegistry sets the file ID registry for path resolution
func (pr *PHPResolver) SetFileRegistry(registry map[string]types.FileID) {
	pr.filePathToID = registry

	// Build reverse mapping
	pr.fileIDToPath = make(map[types.FileID]string)
	for path, fileID := range registry {
		pr.fileIDToPath[fileID] = path
	}
}

// ResolveImport resolves a PHP include/require or namespace import to a ModuleResolution
func (pr *PHPResolver) ResolveImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	// Handle different types of PHP imports:
	// 1. File includes/requires (relative or absolute paths)
	// 2. Namespace use statements (fully qualified class names)

	if pr.isFilePath(importPath) {
		// This is a file include/require
		return pr.resolveFileInclude(importPath, fromFile)
	} else {
		// This is a namespace/class import
		return pr.resolveNamespaceImport(importPath, fromFile)
	}
}

// resolveFileInclude resolves file include/require statements
func (pr *PHPResolver) resolveFileInclude(includePath string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: includePath,
	}

	fromPath, exists := pr.fileIDToPath[fromFile]
	if !exists {
		resolution.Resolution = types.ResolutionError
		resolution.Error = "source file not found"
		return resolution
	}

	var candidatePaths []string

	// If path is absolute, use it directly
	if filepath.IsAbs(includePath) {
		candidatePaths = []string{includePath}
	} else {
		// Try relative to the current file
		currentDir := filepath.Dir(fromPath)
		relativePath := filepath.Join(currentDir, includePath)
		candidatePaths = append(candidatePaths, relativePath)

		// Try relative to project root
		projectPath := filepath.Join(pr.projectRoot, includePath)
		candidatePaths = append(candidatePaths, projectPath)

		// Try include paths
		for _, includeDir := range pr.includePaths {
			if !filepath.IsAbs(includeDir) {
				includeDir = filepath.Join(pr.projectRoot, includeDir)
			}
			includeDirPath := filepath.Join(includeDir, includePath)
			candidatePaths = append(candidatePaths, includeDirPath)
		}
	}

	// Check each candidate path
	for _, candidatePath := range candidatePaths {
		absPath, err := filepath.Abs(candidatePath)
		if err != nil {
			continue
		}

		// Check if file exists in our registry
		if fileID, exists := pr.filePathToID[absPath]; exists {
			resolution.ResolvedPath = absPath
			resolution.FileID = fileID
			resolution.Resolution = types.ResolutionInternal
			return resolution
		}

		// Check if file exists on filesystem
		if pr.fileService.Exists(absPath) {
			resolution.ResolvedPath = absPath
			resolution.IsExternal = true
			resolution.Resolution = types.ResolutionExternal
			return resolution
		}
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "file not found"
	return resolution
}

// resolveNamespaceImport resolves PHP namespace/class imports
func (pr *PHPResolver) resolveNamespaceImport(className string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: className,
	}

	// Normalize class name (remove leading backslash)
	normalizedClassName := strings.TrimPrefix(className, "\\")

	// Try PSR-4 autoloading first
	if filePath := pr.resolvePSR4(normalizedClassName); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	// Try PSR-0 autoloading
	if filePath := pr.resolvePSR0(normalizedClassName); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	// Try class map
	if filePath, exists := pr.autoloadConfig.ClassMap[normalizedClassName]; exists {
		return pr.checkResolvedPath(filePath, resolution)
	}

	// Try simple file-based resolution (class name to file name)
	if filePath := pr.resolveSimpleClassFile(normalizedClassName, fromFile); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "class not found"
	return resolution
}

// resolvePSR4 resolves a class using PSR-4 autoloading rules
func (pr *PHPResolver) resolvePSR4(className string) string {
	for namespacePrefix, baseDir := range pr.autoloadConfig.PSR4 {
		if strings.HasPrefix(className, namespacePrefix) {
			// Remove the namespace prefix and convert to file path
			relativePath := strings.TrimPrefix(className, namespacePrefix)
			relativePath = strings.TrimPrefix(relativePath, "\\")

			// Convert namespace separators to directory separators
			filePath := strings.ReplaceAll(relativePath, "\\", string(filepath.Separator))
			filePath += ".php"

			// Combine with base directory
			if !filepath.IsAbs(baseDir) {
				baseDir = filepath.Join(pr.projectRoot, baseDir)
			}

			fullPath := filepath.Join(baseDir, filePath)
			return fullPath
		}
	}

	return ""
}

// resolvePSR0 resolves a class using PSR-0 autoloading rules
func (pr *PHPResolver) resolvePSR0(className string) string {
	for namespacePrefix, baseDir := range pr.autoloadConfig.PSR0 {
		if strings.HasPrefix(className, namespacePrefix) {
			// PSR-0 includes the full namespace in the path
			filePath := strings.ReplaceAll(className, "\\", string(filepath.Separator))
			filePath += ".php"

			// Combine with base directory
			if !filepath.IsAbs(baseDir) {
				baseDir = filepath.Join(pr.projectRoot, baseDir)
			}

			fullPath := filepath.Join(baseDir, filePath)
			return fullPath
		}
	}

	return ""
}

// resolveSimpleClassFile tries to resolve a class by converting class name to file name
func (pr *PHPResolver) resolveSimpleClassFile(className string, fromFile types.FileID) string {
	// Try various common patterns:
	// 1. ClassName.php
	// 2. class.ClassName.php
	// 3. ClassName.class.php

	// Extract just the class name (last part after \)
	parts := strings.Split(className, "\\")
	simpleClassName := parts[len(parts)-1]

	fromPath, exists := pr.fileIDToPath[fromFile]
	if !exists {
		return ""
	}

	currentDir := filepath.Dir(fromPath)

	candidates := []string{
		simpleClassName + ".php",
		"class." + simpleClassName + ".php",
		simpleClassName + ".class.php",
		strings.ToLower(simpleClassName) + ".php",
	}

	for _, candidate := range candidates {
		candidatePath := filepath.Join(currentDir, candidate)
		if pr.fileService.Exists(candidatePath) {
			absPath, _ := filepath.Abs(candidatePath)
			return absPath
		}
	}

	return ""
}

// checkResolvedPath checks if a resolved path exists and returns appropriate resolution
func (pr *PHPResolver) checkResolvedPath(filePath string, resolution types.ModuleResolution) types.ModuleResolution {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		resolution.Resolution = types.ResolutionError
		resolution.Error = "invalid path"
		return resolution
	}

	// Check if file exists in our registry
	if fileID, exists := pr.filePathToID[absPath]; exists {
		resolution.ResolvedPath = absPath
		resolution.FileID = fileID
		resolution.Resolution = types.ResolutionInternal
		return resolution
	}

	// Check if file exists on filesystem
	if pr.fileService.Exists(absPath) {
		resolution.ResolvedPath = absPath
		resolution.IsExternal = true
		resolution.Resolution = types.ResolutionExternal
		return resolution
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "file not found"
	return resolution
}

// isFilePath checks if an import path looks like a file path
func (pr *PHPResolver) isFilePath(importPath string) bool {
	// File paths usually contain:
	// - Forward slashes for directory separators
	// - File extensions (.php, etc.)
	// - Relative path indicators (./, ../)
	//
	// Note: Backslashes in PHP are namespace separators, not file paths

	return strings.Contains(importPath, "/") ||
		strings.HasPrefix(importPath, "./") ||
		strings.HasPrefix(importPath, "../") ||
		strings.HasSuffix(importPath, ".php") ||
		strings.HasSuffix(importPath, ".phar") ||
		strings.HasSuffix(importPath, ".inc")
}

// findComposerJson finds and parses composer.json for autoload configuration
func (pr *PHPResolver) findComposerJson() {
	composerPath := filepath.Join(pr.projectRoot, "composer.json")

	if !pr.fileService.Exists(composerPath) {
		return
	}

	pr.composerJsonPath = composerPath

	// Read and parse composer.json through ContentStore for consistency
	fileID, err := pr.fileService.LoadFile(composerPath)
	if err != nil {
		return
	}

	content, ok := pr.fileService.GetFileContent(fileID)
	if !ok {
		return
	}

	// Simple JSON parsing for autoload section
	// In a real implementation, you'd use proper JSON parsing
	pr.parseComposerAutoload(string(content))
}

// parseComposerAutoload parses autoload configuration from composer.json
func (pr *PHPResolver) parseComposerAutoload(jsonContent string) {
	// Parse composer.json content using proper JSON parsing
	var composer struct {
		Autoload struct {
			PSR4     map[string]string `json:"psr-4"`
			PSR0     map[string]string `json:"psr-0"`
			ClassMap []string          `json:"classmap"`
			Files    []string          `json:"files"`
		} `json:"autoload"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &composer); err != nil {
		// CRITICAL: Log JSON parsing errors instead of silently ignoring them
		// This helps identify corrupted composer.json files
		debug.LogIndexing("Failed to parse composer.json: %v", err)
		return
	}

	// Copy PSR-4 mappings
	for namespace, directory := range composer.Autoload.PSR4 {
		pr.autoloadConfig.PSR4[namespace] = directory
	}

	// Copy PSR-0 mappings
	for namespace, directory := range composer.Autoload.PSR0 {
		pr.autoloadConfig.PSR0[namespace] = directory
	}

	// Copy files
	pr.autoloadConfig.Files = append(pr.autoloadConfig.Files, composer.Autoload.Files...)
}

// AddIncludePath adds an additional include path for file resolution
func (pr *PHPResolver) AddIncludePath(path string) {
	pr.includePaths = append(pr.includePaths, path)
}

// SetPSR4Mapping sets a PSR-4 namespace mapping
func (pr *PHPResolver) SetPSR4Mapping(namespace, directory string) {
	pr.autoloadConfig.PSR4[namespace] = directory
}

// SetPSR0Mapping sets a PSR-0 namespace mapping
func (pr *PHPResolver) SetPSR0Mapping(namespace, directory string) {
	pr.autoloadConfig.PSR0[namespace] = directory
}

// AddClassMapping adds a direct class-to-file mapping
func (pr *PHPResolver) AddClassMapping(className, filePath string) {
	pr.autoloadConfig.ClassMap[className] = filePath
}

// GetAutoloadConfig returns the current autoload configuration
func (pr *PHPResolver) GetAutoloadConfig() *PHPAutoloadConfig {
	return pr.autoloadConfig
}

// Stats returns statistics about the PHP resolver
func (pr *PHPResolver) Stats() map[string]interface{} {
	return map[string]interface{}{
		"project_root":     pr.projectRoot,
		"composer_json":    pr.composerJsonPath != "",
		"psr4_mappings":    len(pr.autoloadConfig.PSR4),
		"psr0_mappings":    len(pr.autoloadConfig.PSR0),
		"class_mappings":   len(pr.autoloadConfig.ClassMap),
		"include_paths":    len(pr.includePaths),
		"registered_files": len(pr.filePathToID),
	}
}
