package symbollinker

import (
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// PythonResolver resolves Python import statements
type PythonResolver struct {
	// projectRoot is the root directory of the project being indexed
	projectRoot string

	// pythonPath stores additional Python paths (similar to PYTHONPATH)
	pythonPath []string

	// packagePaths stores package installation paths
	packagePaths []string

	// virtualEnvPath is the path to virtual environment if detected
	virtualEnvPath string

	// sitePackages stores site-packages directory paths
	sitePackages []string

	// filePathToID maps absolute file paths to FileIDs
	filePathToID map[string]types.FileID

	// fileIDToPath maps FileIDs to absolute file paths
	fileIDToPath map[types.FileID]string

	// builtinModules contains Python built-in modules
	builtinModules map[string]bool

	// standardLibModules contains Python standard library modules
	standardLibModules map[string]bool

	// packageInfo stores information about installed packages
	packageInfo map[string]*PythonPackageInfo

	// fileService is the centralized file service for all filesystem operations
	fileService *core.FileService
}

// PythonPackageInfo stores information about a Python package
type PythonPackageInfo struct {
	Name    string
	Version string
	Path    string
	Files   []string
	Modules []string
}

// NewPythonResolver creates a new Python module resolver
func NewPythonResolver(projectRoot string) *PythonResolver {
	return NewPythonResolverWithFileService(projectRoot, core.NewFileService())
}

// NewPythonResolverWithFileService creates a new Python module resolver with a specific FileService
func NewPythonResolverWithFileService(projectRoot string, fileService *core.FileService) *PythonResolver {
	resolver := &PythonResolver{
		projectRoot:        projectRoot,
		pythonPath:         []string{},
		packagePaths:       []string{},
		sitePackages:       []string{},
		filePathToID:       make(map[string]types.FileID),
		fileIDToPath:       make(map[types.FileID]string),
		builtinModules:     initPythonBuiltinModules(),
		standardLibModules: initPythonStandardLibModules(),
		packageInfo:        make(map[string]*PythonPackageInfo),
		fileService:        fileService,
	}

	// Initialize default Python paths
	resolver.initializePythonPaths()

	// Try to detect virtual environment
	resolver.detectVirtualEnv()

	// Scan for packages
	resolver.scanPackages()

	return resolver
}

// RegisterFile registers a file with the resolver
func (pr *PythonResolver) RegisterFile(fileID types.FileID, filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	pr.filePathToID[absPath] = fileID
	pr.fileIDToPath[fileID] = absPath
}

// SetFileRegistry sets the file ID registry for path resolution
func (pr *PythonResolver) SetFileRegistry(registry map[string]types.FileID) {
	pr.filePathToID = registry

	// Build reverse mapping
	pr.fileIDToPath = make(map[types.FileID]string)
	for path, fileID := range registry {
		pr.fileIDToPath[fileID] = path
	}
}

// ResolveImport resolves a Python import statement to a ModuleResolution
func (pr *PythonResolver) ResolveImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: importPath,
	}

	// Handle different types of Python imports:
	// 1. Built-in modules (sys, os, etc.)
	// 2. Standard library modules (json, http, etc.)
	// 3. Relative imports (.module, ..module)
	// 4. Absolute imports (package.module)
	// 5. Installed packages (third-party)

	// Check if it's a built-in module
	if pr.builtinModules[importPath] {
		resolution.ResolvedPath = importPath
		resolution.IsBuiltin = true
		resolution.Resolution = types.ResolutionBuiltin
		return resolution
	}

	// Check if it's a standard library module
	if pr.isStandardLibModule(importPath) {
		resolution.ResolvedPath = importPath
		resolution.IsBuiltin = true
		resolution.Resolution = types.ResolutionBuiltin
		return resolution
	}

	// Handle relative imports
	if strings.HasPrefix(importPath, ".") {
		return pr.resolveRelativeImport(importPath, fromFile)
	}

	// Try to resolve as absolute import
	return pr.resolveAbsoluteImport(importPath, fromFile)
}

// resolveRelativeImport resolves relative imports (.module, ..module)
func (pr *PythonResolver) resolveRelativeImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: importPath,
	}

	fromPath, exists := pr.fileIDToPath[fromFile]
	if !exists {
		resolution.Resolution = types.ResolutionError
		resolution.Error = "source file not found"
		return resolution
	}

	// Count leading dots to determine how many levels up
	dots := 0
	for i, char := range importPath {
		if char == '.' {
			dots++
		} else {
			importPath = importPath[i:] // Remove leading dots
			break
		}
	}

	// Start from the directory containing the source file
	currentDir := filepath.Dir(fromPath)

	// Go up 'dots-1' levels (one dot means current package)
	for i := 1; i < dots; i++ {
		currentDir = filepath.Dir(currentDir)
	}

	// If there's a module name after the dots, resolve it
	if importPath != "" {
		return pr.resolveModuleInDirectory(importPath, currentDir, resolution)
	}

	// If it's just dots (from . import something), resolve to current package
	if initFile := pr.findInitFile(currentDir); initFile != "" {
		return pr.checkResolvedPath(initFile, resolution)
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "relative import target not found"
	return resolution
}

// resolveAbsoluteImport resolves absolute imports (package.module)
func (pr *PythonResolver) resolveAbsoluteImport(importPath string, fromFile types.FileID) types.ModuleResolution {
	resolution := types.ModuleResolution{
		RequestPath: importPath,
	}

	// Try different resolution strategies in order:

	// 1. Try as local project module
	if filePath := pr.resolveProjectModule(importPath, fromFile); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	// 2. Try as installed package
	if filePath := pr.resolveInstalledPackage(importPath); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	// 3. Try in Python path directories
	if filePath := pr.resolveInPythonPath(importPath); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	// 4. Check if it's a known third-party package (external)
	if pr.isKnownThirdPartyPackage(importPath) {
		resolution.ResolvedPath = importPath
		resolution.IsExternal = true
		resolution.Resolution = types.ResolutionExternal
		return resolution
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "module not found"
	return resolution
}

// resolveProjectModule tries to resolve a module within the current project
func (pr *PythonResolver) resolveProjectModule(importPath string, fromFile types.FileID) string {
	parts := strings.Split(importPath, ".")

	// Try to find the module starting from project root
	return pr.findModuleInDirectory(parts, pr.projectRoot)
}

// resolveInstalledPackage tries to resolve a module from installed packages
func (pr *PythonResolver) resolveInstalledPackage(importPath string) string {
	parts := strings.Split(importPath, ".")
	packageName := parts[0]

	// Check if we have information about this package
	if packageInfo, exists := pr.packageInfo[packageName]; exists {
		if len(parts) == 1 {
			// Import the package itself
			return packageInfo.Path
		}

		// Import a submodule
		subModuleParts := parts[1:]
		return pr.findModuleInDirectory(subModuleParts, packageInfo.Path)
	}

	// Try to find in site-packages
	for _, sitePackageDir := range pr.sitePackages {
		if filePath := pr.findModuleInDirectory(parts, sitePackageDir); filePath != "" {
			return filePath
		}
	}

	return ""
}

// resolveInPythonPath tries to resolve a module in Python path directories
func (pr *PythonResolver) resolveInPythonPath(importPath string) string {
	parts := strings.Split(importPath, ".")

	for _, pythonDir := range pr.pythonPath {
		if filePath := pr.findModuleInDirectory(parts, pythonDir); filePath != "" {
			return filePath
		}
	}

	return ""
}

// resolveModuleInDirectory resolves a module within a specific directory
func (pr *PythonResolver) resolveModuleInDirectory(importPath, baseDir string, resolution types.ModuleResolution) types.ModuleResolution {
	parts := strings.Split(importPath, ".")

	if filePath := pr.findModuleInDirectory(parts, baseDir); filePath != "" {
		return pr.checkResolvedPath(filePath, resolution)
	}

	resolution.Resolution = types.ResolutionError
	resolution.Error = "module not found in directory"
	return resolution
}

// findModuleInDirectory finds a module file in a directory structure
func (pr *PythonResolver) findModuleInDirectory(parts []string, baseDir string) string {
	if len(parts) == 0 {
		return ""
	}

	currentDir := baseDir

	// Navigate through package directories
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - look for the actual module
			// Try module.py first
			modulePath := filepath.Join(currentDir, part+".py")
			if pr.fileService.Exists(modulePath) {
				if absPath, err := filepath.Abs(modulePath); err == nil {
					return absPath
				}
			}

			// Try as package directory with __init__.py
			packageDir := filepath.Join(currentDir, part)
			if initFile := pr.findInitFile(packageDir); initFile != "" {
				return initFile
			}
		} else {
			// Intermediate part - must be a package directory
			packageDir := filepath.Join(currentDir, part)
			if !pr.fileService.IsDir(packageDir) {
				return ""
			}

			// Check if it's a valid Python package (has __init__.py)
			if !pr.hasInitFile(packageDir) {
				return ""
			}

			currentDir = packageDir
		}
	}

	return ""
}

// findInitFile finds __init__.py in a directory
func (pr *PythonResolver) findInitFile(dir string) string {
	initFile := filepath.Join(dir, "__init__.py")
	if pr.fileService.Exists(initFile) {
		if absPath, err := filepath.Abs(initFile); err == nil {
			return absPath
		}
	}
	return ""
}

// hasInitFile checks if a directory has __init__.py
func (pr *PythonResolver) hasInitFile(dir string) bool {
	return pr.findInitFile(dir) != ""
}

// isStandardLibModule checks if a module is part of Python standard library
func (pr *PythonResolver) isStandardLibModule(module string) bool {
	// Check exact match
	if pr.standardLibModules[module] {
		return true
	}

	// Check if it's a submodule of a standard library package
	parts := strings.Split(module, ".")
	if len(parts) > 1 {
		return pr.standardLibModules[parts[0]]
	}

	return false
}

// isKnownThirdPartyPackage checks if a module is a known third-party package
func (pr *PythonResolver) isKnownThirdPartyPackage(module string) bool {
	// Extract base package name
	parts := strings.Split(module, ".")
	packageName := parts[0]

	// List of common third-party packages
	knownPackages := []string{
		"numpy", "pandas", "matplotlib", "scipy", "sklearn",
		"requests", "flask", "django", "fastapi", "aiohttp",
		"pytest", "click", "pydantic", "sqlalchemy", "alembic",
		"celery", "redis", "boto3", "paramiko", "fabric",
		"pillow", "opencv", "tensorflow", "torch", "keras",
	}

	for _, known := range knownPackages {
		if packageName == known {
			return true
		}
	}

	return false
}

// checkResolvedPath checks if a resolved path exists and returns appropriate resolution
func (pr *PythonResolver) checkResolvedPath(filePath string, resolution types.ModuleResolution) types.ModuleResolution {
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

// initializePythonPaths initializes default Python paths
func (pr *PythonResolver) initializePythonPaths() {
	// Add project root to Python path
	pr.pythonPath = append(pr.pythonPath, pr.projectRoot)

	// Add common Python directories
	commonDirs := []string{
		"src",
		"lib",
		"modules",
	}

	for _, dir := range commonDirs {
		fullPath := filepath.Join(pr.projectRoot, dir)
		if pr.fileService.IsDir(fullPath) {
			pr.pythonPath = append(pr.pythonPath, fullPath)
		}
	}
}

// detectVirtualEnv tries to detect if we're in a virtual environment
func (pr *PythonResolver) detectVirtualEnv() {
	// Look for common virtual environment indicators
	venvPaths := []string{
		"venv",
		".venv",
		"env",
		".env",
		"virtualenv",
	}

	for _, venvPath := range venvPaths {
		fullPath := filepath.Join(pr.projectRoot, venvPath)
		if pr.fileService.IsDir(fullPath) {
			pr.virtualEnvPath = fullPath

			// Add site-packages to our search paths
			sitePackagesPath := filepath.Join(fullPath, "lib", "python*", "site-packages")
			pr.sitePackages = append(pr.sitePackages, sitePackagesPath)

			// Also check Lib/site-packages for Windows
			windowsSitePackages := filepath.Join(fullPath, "Lib", "site-packages")
			if pr.fileService.IsDir(windowsSitePackages) {
				pr.sitePackages = append(pr.sitePackages, windowsSitePackages)
			}

			break
		}
	}
}

// scanPackages scans for installed Python packages
func (pr *PythonResolver) scanPackages() {
	// Look for requirements.txt or setup.py
	requirementsFile := filepath.Join(pr.projectRoot, "requirements.txt")
	if pr.fileService.Exists(requirementsFile) {
		pr.parseRequirements(requirementsFile)
	}

	setupFile := filepath.Join(pr.projectRoot, "setup.py")
	if pr.fileService.Exists(setupFile) {
		pr.parseSetupPy(setupFile)
	}

	pyprojectFile := filepath.Join(pr.projectRoot, "pyproject.toml")
	if pr.fileService.Exists(pyprojectFile) {
		pr.parsePyprojectToml(pyprojectFile)
	}
}

// parseRequirements parses requirements.txt file
func (pr *PythonResolver) parseRequirements(filePath string) {
	// Read requirements.txt through ContentStore for consistency
	fileID, err := pr.fileService.LoadFile(filePath)
	if err != nil {
		return
	}

	content, ok := pr.fileService.GetFileContent(fileID)
	if !ok {
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract package name (before ==, >=, etc.)
		packageName := line
		for _, op := range []string{"==", ">=", "<=", ">", "<", "!=", "~="} {
			if idx := strings.Index(line, op); idx != -1 {
				packageName = strings.TrimSpace(line[:idx])
				break
			}
		}

		if packageName != "" {
			pr.packageInfo[packageName] = &PythonPackageInfo{
				Name: packageName,
				Path: "", // Will be resolved later if needed
			}
		}
	}
}

// parseSetupPy parses setup.py file for dependencies
func (pr *PythonResolver) parseSetupPy(filePath string) {
	// Read setup.py through ContentStore for consistency
	fileID, err := pr.fileService.LoadFile(filePath)
	if err != nil {
		return
	}

	content, ok := pr.fileService.GetFileContent(fileID)
	if !ok {
		return
	}

	contentStr := string(content)

	// Look for install_requires
	if strings.Contains(contentStr, "install_requires") {
		// This is a simplified parser - in production use proper Python parsing
		lines := strings.Split(contentStr, "\n")
		inRequires := false

		for _, line := range lines {
			line = strings.TrimSpace(line)

			if strings.Contains(line, "install_requires") {
				inRequires = true
				continue
			}

			if inRequires {
				if strings.Contains(line, "]") {
					break
				}

				if strings.Contains(line, "\"") || strings.Contains(line, "'") {
					// Extract package name from quoted string
					var packageName string
					if start := strings.Index(line, "\""); start != -1 {
						end := strings.Index(line[start+1:], "\"")
						if end != -1 {
							packageName = line[start+1 : start+1+end]
						}
					} else if start := strings.Index(line, "'"); start != -1 {
						end := strings.Index(line[start+1:], "'")
						if end != -1 {
							packageName = line[start+1 : start+1+end]
						}
					}

					if packageName != "" {
						// Remove version specifications
						for _, op := range []string{"==", ">=", "<=", ">", "<", "!="} {
							if idx := strings.Index(packageName, op); idx != -1 {
								packageName = strings.TrimSpace(packageName[:idx])
								break
							}
						}

						pr.packageInfo[packageName] = &PythonPackageInfo{
							Name: packageName,
							Path: "",
						}
					}
				}
			}
		}
	}
}

// parsePyprojectToml parses pyproject.toml file for dependencies
func (pr *PythonResolver) parsePyprojectToml(filePath string) {
	// Read pyproject.toml through ContentStore for consistency
	fileID, err := pr.fileService.LoadFile(filePath)
	if err != nil {
		return
	}

	content, ok := pr.fileService.GetFileContent(fileID)
	if !ok {
		return
	}

	// This is a very simplified TOML parser
	// In production, use a proper TOML parsing library
	lines := strings.Split(string(content), "\n")
	inDependencies := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "dependencies = [") {
			inDependencies = true
			continue
		}

		if inDependencies {
			if strings.Contains(line, "]") {
				break
			}

			if strings.Contains(line, "\"") {
				// Extract package name
				start := strings.Index(line, "\"")
				end := strings.Index(line[start+1:], "\"")
				if start != -1 && end != -1 {
					packageName := line[start+1 : start+1+end]

					// Remove version specifications
					for _, op := range []string{"==", ">=", "<=", ">", "<", "!=", "~="} {
						if idx := strings.Index(packageName, op); idx != -1 {
							packageName = strings.TrimSpace(packageName[:idx])
							break
						}
					}

					if packageName != "" {
						pr.packageInfo[packageName] = &PythonPackageInfo{
							Name: packageName,
							Path: "",
						}
					}
				}
			}
		}
	}
}

// initPythonBuiltinModules initializes the map of Python built-in modules
func initPythonBuiltinModules() map[string]bool {
	builtins := map[string]bool{
		"builtins":    true,
		"sys":         true,
		"os":          true,
		"math":        true,
		"random":      true,
		"time":        true,
		"datetime":    true,
		"itertools":   true,
		"functools":   true,
		"collections": true,
		"operator":    true,
		"copy":        true,
		"pickle":      true,
		"struct":      true,
		"array":       true,
		"weakref":     true,
		"types":       true,
		"gc":          true,
	}

	return builtins
}

// initPythonStandardLibModules initializes the map of Python standard library modules
func initPythonStandardLibModules() map[string]bool {
	stdlib := map[string]bool{
		// Text processing
		"string": true, "re": true, "difflib": true, "textwrap": true,
		"unicodedata": true, "stringprep": true, "readline": true, "rlcompleter": true,

		// Binary data
		"struct": true, "codecs": true,

		// Data types
		"datetime": true, "calendar": true, "collections": true, "heapq": true,
		"bisect": true, "array": true, "weakref": true, "types": true,
		"copy": true, "pprint": true, "reprlib": true, "enum": true,

		// Numeric
		"numbers": true, "math": true, "cmath": true, "decimal": true,
		"fractions": true, "random": true, "statistics": true,

		// Functional programming
		"itertools": true, "functools": true, "operator": true,

		// File and directory
		"pathlib": true, "fileinput": true, "stat": true, "filecmp": true,
		"tempfile": true, "glob": true, "fnmatch": true, "linecache": true,
		"shutil": true,

		// Data persistence
		"pickle": true, "copyreg": true, "shelve": true, "marshal": true,
		"dbm": true, "sqlite3": true,

		// Data compression
		"zlib": true, "gzip": true, "bz2": true, "lzma": true, "zipfile": true,
		"tarfile": true,

		// File formats
		"csv": true, "configparser": true, "netrc": true, "xdrlib": true,
		"plistlib": true,

		// Cryptographic
		"hashlib": true, "hmac": true, "secrets": true,

		// OS interface
		"os": true, "io": true, "time": true, "argparse": true, "getopt": true,
		"logging": true, "getpass": true, "curses": true, "platform": true,
		"errno": true, "ctypes": true,

		// Concurrent execution
		"threading": true, "multiprocessing": true, "concurrent": true,
		"subprocess": true, "sched": true, "queue": true, "contextvars": true,
		"asyncio": true,

		// Networking
		"socket": true, "ssl": true, "select": true, "selectors": true,
		"asyncore": true, "asynchat": true, "signal": true, "mmap": true,

		// Internet data handling
		"email": true, "json": true, "mailcap": true, "mailbox": true,
		"mimetypes": true, "base64": true, "binhex": true, "binascii": true,
		"quopri": true, "uu": true,

		// HTML and XML
		"html": true, "xml": true,

		// Internet protocols
		"urllib": true, "http": true, "ftplib": true, "poplib": true,
		"imaplib": true, "nntplib": true, "smtplib": true, "smtpd": true,
		"telnetlib": true, "uuid": true, "socketserver": true, "xmlrpc": true,

		// Multimedia
		"audioop": true, "aifc": true, "sunau": true, "wave": true,
		"chunk": true, "colorsys": true, "imghdr": true, "sndhdr": true,
		"ossaudiodev": true,

		// Internationalization
		"gettext": true, "locale": true,

		// Development tools
		"typing": true, "pydoc": true, "doctest": true, "unittest": true,
		"test": true, "lib2to3": true,

		// Debugging and profiling
		"bdb": true, "faulthandler": true, "pdb": true, "profile": true,
		"cProfile": true, "pstats": true, "timeit": true, "trace": true,
		"tracemalloc": true,

		// Runtime services
		"sys": true, "sysconfig": true, "builtins": true, "warnings": true,
		"dataclasses": true, "contextlib": true, "abc": true, "atexit": true,
		"traceback": true, "gc": true, "inspect": true, "site": true,

		// Custom interpreters
		"code": true, "codeop": true,

		// Importing modules
		"zipimport": true, "pkgutil": true, "modulefinder": true, "runpy": true,
		"importlib": true,

		// Language services
		"parser": true, "ast": true, "symtable": true, "symbol": true,
		"token": true, "keyword": true, "tokenize": true, "tabnanny": true,
		"py_compile": true, "compileall": true, "dis": true, "pickletools": true,
	}

	return stdlib
}

// AddPythonPath adds a directory to Python path
func (pr *PythonResolver) AddPythonPath(path string) {
	pr.pythonPath = append(pr.pythonPath, path)
}

// AddPackageInfo adds package information
func (pr *PythonResolver) AddPackageInfo(name string, info *PythonPackageInfo) {
	pr.packageInfo[name] = info
}

// GetVirtualEnvPath returns the detected virtual environment path
func (pr *PythonResolver) GetVirtualEnvPath() string {
	return pr.virtualEnvPath
}

// GetPackageInfo returns information about installed packages
func (pr *PythonResolver) GetPackageInfo() map[string]*PythonPackageInfo {
	return pr.packageInfo
}

// Stats returns statistics about the Python resolver
func (pr *PythonResolver) Stats() map[string]interface{} {
	return map[string]interface{}{
		"project_root":     pr.projectRoot,
		"python_paths":     len(pr.pythonPath),
		"site_packages":    len(pr.sitePackages),
		"virtual_env":      pr.virtualEnvPath != "",
		"builtin_modules":  len(pr.builtinModules),
		"stdlib_modules":   len(pr.standardLibModules),
		"known_packages":   len(pr.packageInfo),
		"registered_files": len(pr.filePathToID),
	}
}
