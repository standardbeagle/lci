package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/standardbeagle/lci/internal/types"
)

// FileLoader provides high-level file loading operations using the FileService.
// It handles batch loading, filtering, and discovery of code files.
type FileLoader struct {
	fileService *FileService

	// Configuration
	supportedExtensions map[string]bool
	excludePatterns     []string
	maxConcurrentLoads  int
}

// FileLoaderOptions configures the file loader
type FileLoaderOptions struct {
	FileService         *FileService
	SupportedExtensions []string
	ExcludePatterns     []string
	MaxConcurrentLoads  int
}

// NewFileLoader creates a new file loader
func NewFileLoader(opts FileLoaderOptions) *FileLoader {
	fileService := opts.FileService
	if fileService == nil {
		fileService = NewFileService()
	}

	// Default supported extensions for code files
	extensions := opts.SupportedExtensions
	if len(extensions) == 0 {
		extensions = []string{
			".go", ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs",
			".py", ".java", ".cpp", ".c", ".h", ".hpp",
			".rs", ".rb", ".php", ".cs", ".swift", ".kt",
			".scala", ".clj", ".hs", ".ml", ".elm", ".dart",
			".json", ".yaml", ".yml", ".toml", ".md",
		}
	}

	// Convert to map for fast lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[ext] = true
	}

	maxConcurrent := opts.MaxConcurrentLoads
	if maxConcurrent == 0 {
		maxConcurrent = 10 // Default concurrent loading limit
	}

	excludePatterns := opts.ExcludePatterns
	if len(excludePatterns) == 0 {
		excludePatterns = []string{
			"node_modules/*", "vendor/*", ".git/*",
			"*.test.*", "*_test.*", "*.min.*",
			"dist/*", "build/*", "target/*", ".next/*",
		}
	}

	return &FileLoader{
		fileService:         fileService,
		supportedExtensions: extMap,
		excludePatterns:     excludePatterns,
		maxConcurrentLoads:  maxConcurrent,
	}
}

// LoadResult contains the result of loading a file
type LoadResult struct {
	Path   string
	FileID types.FileID
	Error  error
}

// BatchLoadResult contains the results of batch loading
type BatchLoadResult struct {
	Loaded     []LoadResult
	Failed     []LoadResult
	Skipped    []string
	TotalFiles int
}

// LoadFile loads a single file and returns its FileID
func (fl *FileLoader) LoadFile(path string) (types.FileID, error) {
	// Check if file should be loaded
	if !fl.shouldLoadFile(path) {
		return 0, fmt.Errorf("file excluded by filters: %s", path)
	}

	return fl.fileService.LoadFile(path)
}

// LoadFiles loads multiple files concurrently
func (fl *FileLoader) LoadFiles(paths []string) *BatchLoadResult {
	result := &BatchLoadResult{
		TotalFiles: len(paths),
	}

	// Filter paths first
	var validPaths []string
	for _, path := range paths {
		if fl.shouldLoadFile(path) {
			validPaths = append(validPaths, path)
		} else {
			result.Skipped = append(result.Skipped, path)
		}
	}

	// Load files concurrently
	loadChan := make(chan string, len(validPaths))
	resultChan := make(chan LoadResult, len(validPaths))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < fl.maxConcurrentLoads && i < len(validPaths); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range loadChan {
				fileID, err := fl.fileService.LoadFile(path)
				resultChan <- LoadResult{
					Path:   path,
					FileID: fileID,
					Error:  err,
				}
			}
		}()
	}

	// Send work to workers
	for _, path := range validPaths {
		loadChan <- path
	}
	close(loadChan)

	// Wait for completion
	wg.Wait()
	close(resultChan)

	// Collect results
	for loadResult := range resultChan {
		if loadResult.Error != nil {
			result.Failed = append(result.Failed, loadResult)
		} else {
			result.Loaded = append(result.Loaded, loadResult)
		}
	}

	return result
}

// DiscoverAndLoadFiles discovers all code files in a directory tree and loads them
func (fl *FileLoader) DiscoverAndLoadFiles(rootPath string) (*BatchLoadResult, error) {
	if !fl.fileService.IsDir(rootPath) {
		return nil, fmt.Errorf("not a directory: %s", rootPath)
	}

	// Discover files
	files, err := fl.discoverFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover files in %s: %w", rootPath, err)
	}

	// Load discovered files
	return fl.LoadFiles(files), nil
}

// DiscoverGoFiles discovers all Go files in a directory tree
func (fl *FileLoader) DiscoverGoFiles(rootPath string) ([]string, error) {
	return fl.discoverFilesWithExtensions(rootPath, []string{".go"})
}

// DiscoverJSFiles discovers all JavaScript/TypeScript files in a directory tree
func (fl *FileLoader) DiscoverJSFiles(rootPath string) ([]string, error) {
	return fl.discoverFilesWithExtensions(rootPath, []string{
		".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs",
	})
}

// LoadGoFiles discovers and loads all Go files in a directory tree
func (fl *FileLoader) LoadGoFiles(rootPath string) (*BatchLoadResult, error) {
	files, err := fl.DiscoverGoFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover Go files in %s: %w", rootPath, err)
	}
	return fl.LoadFiles(files), nil
}

// LoadJSFiles discovers and loads all JavaScript/TypeScript files in a directory tree
func (fl *FileLoader) LoadJSFiles(rootPath string) (*BatchLoadResult, error) {
	files, err := fl.DiscoverJSFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover JS/TS files in %s: %w", rootPath, err)
	}
	return fl.LoadFiles(files), nil
}

// UpdateFile updates or loads a file with new content
func (fl *FileLoader) UpdateFile(path string, content []byte) (types.FileID, error) {
	if !fl.shouldLoadFile(path) {
		return 0, fmt.Errorf("file excluded by filters: %s", path)
	}

	return fl.fileService.UpdateFileContent(path, content)
}

// GetFileRegistry returns a registry of all loaded files for use with resolvers
func (fl *FileLoader) GetFileRegistry() map[string]types.FileID {
	return fl.fileService.GetAllLoadedFiles()
}

// Private methods

func (fl *FileLoader) discoverFiles(rootPath string) ([]string, error) {
	var files []string

	err := fl.walkDirectory(rootPath, func(path string, isDir bool) error {
		if !isDir && fl.shouldLoadFile(path) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to discover files in %s: %w", rootPath, err)
	}

	return files, nil
}

func (fl *FileLoader) discoverFilesWithExtensions(rootPath string, extensions []string) ([]string, error) {
	// Create extension map for fast lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[ext] = true
	}

	var files []string

	err := fl.walkDirectory(rootPath, func(path string, isDir bool) error {
		if !isDir && extMap[filepath.Ext(path)] && fl.shouldLoadFile(path) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to discover files with extensions in %s: %w", rootPath, err)
	}

	return files, nil
}

func (fl *FileLoader) walkDirectory(rootPath string, visitor func(path string, isDir bool) error) error {
	return fl.walkDirectoryWithVisited(rootPath, visitor, make(map[string]bool))
}

func (fl *FileLoader) walkDirectoryWithVisited(rootPath string, visitor func(path string, isDir bool) error, visitedDirs map[string]bool) error {
	// Optimize: Only check for symlinks if this might be one
	info, err := os.Lstat(rootPath)
	if err != nil {
		return nil // Skip paths that can't be accessed
	}

	// If it's a symlink, check for cycles by resolving the real path
	var realPath string
	if info.Mode()&os.ModeSymlink != 0 {
		realPath, err = filepath.EvalSymlinks(rootPath)
		if err != nil {
			return nil // Skip symlinks that can't be resolved
		}
	} else {
		// For regular directories, use the path as-is (no expensive resolution)
		realPath = rootPath
	}

	// Check if we've already visited this real directory
	if visitedDirs[realPath] {
		return nil // Skip to prevent cycle
	}
	visitedDirs[realPath] = true

	// Get directory contents
	entries, err := fl.fileService.fileSystem.ReadDir(rootPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", rootPath, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(rootPath, name)

		// Skip ignored directories and files
		if fl.shouldIgnorePath(fullPath) {
			continue
		}

		isDir := entry.IsDir()

		// Visit this path
		if err := visitor(fullPath, isDir); err != nil {
			return fmt.Errorf("visitor failed for path %s: %w", fullPath, err)
		}

		// Recurse into subdirectories
		if isDir {
			if err := fl.walkDirectoryWithVisited(fullPath, visitor, visitedDirs); err != nil {
				return fmt.Errorf("failed to walk subdirectory %s: %w", fullPath, err)
			}
		}
	}

	return nil
}

func (fl *FileLoader) shouldLoadFile(path string) bool {
	// Check if path should be ignored
	if fl.shouldIgnorePath(path) {
		return false
	}

	// Check if file exists
	if !fl.fileService.Exists(path) {
		return false
	}

	// Check if it's a regular file
	if !fl.fileService.IsFile(path) {
		return false
	}

	// Check extension
	ext := filepath.Ext(path)
	return fl.supportedExtensions[ext]
}

func (fl *FileLoader) shouldIgnorePath(path string) bool {
	// Normalize path for pattern matching
	normalizedPath := filepath.ToSlash(path)

	// Check each exclude pattern
	for _, pattern := range fl.excludePatterns {
		// Simple pattern matching - could be enhanced with full glob support
		if strings.Contains(normalizedPath, strings.TrimSuffix(pattern, "/*")) {
			return true
		}

		// Check exact match
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}

		// Check path component match
		pathComponents := strings.Split(normalizedPath, "/")
		for _, component := range pathComponents {
			if matched, _ := filepath.Match(pattern, component); matched {
				return true
			}
		}
	}

	return false
}

// Statistics and Monitoring

// GetLoadStats returns statistics about loaded files
func (fl *FileLoader) GetLoadStats() map[string]interface{} {
	registry := fl.GetFileRegistry()

	// Count files by extension
	extCount := make(map[string]int)
	for path := range registry {
		ext := filepath.Ext(path)
		extCount[ext]++
	}

	return map[string]interface{}{
		"total_files":        len(registry),
		"files_by_extension": extCount,
		"content_store_stats": map[string]interface{}{
			"file_count":   fl.fileService.contentStore.GetFileCount(),
			"memory_usage": fl.fileService.contentStore.GetMemoryUsage(),
		},
	}
}

// Configuration Helpers

// AddSupportedExtension adds a file extension to the supported list
func (fl *FileLoader) AddSupportedExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	fl.supportedExtensions[ext] = true
}

// RemoveSupportedExtension removes a file extension from the supported list
func (fl *FileLoader) RemoveSupportedExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	delete(fl.supportedExtensions, ext)
}

// AddExcludePattern adds an exclude pattern
func (fl *FileLoader) AddExcludePattern(pattern string) {
	fl.excludePatterns = append(fl.excludePatterns, pattern)
}

// SetMaxConcurrentLoads sets the maximum number of concurrent file loads
func (fl *FileLoader) SetMaxConcurrentLoads(max int) {
	fl.maxConcurrentLoads = max
}

// Close releases resources held by the FileLoader
func (fl *FileLoader) Close() {
	if fl.fileService != nil {
		fl.fileService.Close()
	}
}
