package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// FileService provides centralized file system operations and content management.
// It is the ONLY component that should directly interact with the filesystem.
// All other components should use this service for file operations.
type FileService struct {
	contentStore *FileContentStore

	// Filesystem abstraction layer
	fileSystem FileSystemInterface

	// File system cache and metadata
	mu             sync.RWMutex
	fileInfo       map[string]FileMetadata
	fileIDToPath   map[types.FileID]string // Reverse mapping for O(1) FileID -> path lookup
	directoryCache map[string][]string     // directory -> list of files

	// Configuration
	maxFileSizeBytes int64
	ignoreDotFiles   bool
	ignorePatterns   []string
}

// FileSystemInterface abstracts filesystem operations for testing and flexibility
type FileSystemInterface interface {
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Exists(path string) bool
	IsDir(path string) bool
	IsFile(path string) bool
}

// FileMetadata contains cached filesystem metadata
type FileMetadata struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	Exists  bool
	FileID  types.FileID // Only set for loaded files
}

// RealFileSystem implements FileSystemInterface using the actual filesystem
type RealFileSystem struct{}

func (rfs *RealFileSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (rfs *RealFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (rfs *RealFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (rfs *RealFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (rfs *RealFileSystem) IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (rfs *RealFileSystem) IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// NewFileService creates a new centralized file service
func NewFileService() *FileService {
	return NewFileServiceWithOptions(FileServiceOptions{})
}

// FileServiceOptions configures the file service
type FileServiceOptions struct {
	ContentStore     *FileContentStore
	FileSystem       FileSystemInterface
	MaxFileSizeBytes int64
	IgnoreDotFiles   bool
	IgnorePatterns   []string
}

// NewFileServiceWithOptions creates a file service with custom configuration
func NewFileServiceWithOptions(opts FileServiceOptions) *FileService {
	contentStore := opts.ContentStore
	if contentStore == nil {
		contentStore = NewFileContentStore()
	}

	fileSystem := opts.FileSystem
	if fileSystem == nil {
		fileSystem = &RealFileSystem{}
	}

	maxSize := opts.MaxFileSizeBytes
	if maxSize == 0 {
		maxSize = 10 * 1024 * 1024 // Default 10MB limit
	}

	return &FileService{
		contentStore:     contentStore,
		fileSystem:       fileSystem,
		fileInfo:         make(map[string]FileMetadata),
		fileIDToPath:     make(map[types.FileID]string),
		directoryCache:   make(map[string][]string),
		maxFileSizeBytes: maxSize,
		ignoreDotFiles:   opts.IgnoreDotFiles,
		ignorePatterns:   opts.IgnorePatterns,
	}
}

// File Loading and Content Management

// LoadFile loads a file's content and returns its FileID
func (fs *FileService) LoadFile(path string) (types.FileID, error) {
	// Check file metadata first
	metadata, err := fs.getFileMetadata(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get file metadata for %s: %w", path, err)
	}

	if !metadata.Exists {
		return 0, fmt.Errorf("file does not exist: %s", path)
	}

	if metadata.IsDir {
		// Silently skip directories (including symlinks to directories)
		// This prevents errors when file watchers encounter directory symlinks
		return 0, nil
	}

	if metadata.Size > fs.maxFileSizeBytes {
		return 0, fmt.Errorf("file too large (%d bytes, limit %d): %s",
			metadata.Size, fs.maxFileSizeBytes, path)
	}

	// Read file content
	content, err := fs.fileSystem.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Load into content store
	fileID := fs.contentStore.LoadFile(path, content)

	// Update metadata with FileID and reverse mapping
	fs.mu.Lock()
	if meta, exists := fs.fileInfo[path]; exists {
		meta.FileID = fileID
		fs.fileInfo[path] = meta
	}
	fs.fileIDToPath[fileID] = path // O(1) reverse lookup
	fs.mu.Unlock()

	return fileID, nil
}

// LoadFileFromMemory loads file content from memory (for testing or streaming)
func (fs *FileService) LoadFileFromMemory(path string, content []byte) types.FileID {
	fileID := fs.contentStore.LoadFile(path, content)

	// Create synthetic metadata and reverse mapping
	fs.mu.Lock()
	fs.fileInfo[path] = FileMetadata{
		Path:    path,
		Size:    int64(len(content)),
		ModTime: time.Now(),
		IsDir:   false,
		Exists:  true,
		FileID:  fileID,
	}
	fs.fileIDToPath[fileID] = path // O(1) reverse lookup
	fs.mu.Unlock()

	return fileID
}

// UpdateFileContent updates existing file content or loads new file
func (fs *FileService) UpdateFileContent(path string, content []byte) (types.FileID, error) {
	// Check if this is a real file or in-memory update
	if fs.fileSystem.Exists(path) {
		// Real file - verify it matches what we expect
		diskContent, err := fs.fileSystem.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("failed to read file from disk %s: %w", path, err)
		}

		// Use disk content to ensure consistency
		content = diskContent
	}

	fileID := fs.contentStore.LoadFile(path, content)

	// Update metadata
	fs.mu.Lock()
	fs.fileInfo[path] = FileMetadata{
		Path:    path,
		Size:    int64(len(content)),
		ModTime: time.Now(),
		IsDir:   false,
		Exists:  true,
		FileID:  fileID,
	}
	fs.mu.Unlock()

	return fileID, nil
}

// GetFileContent returns the content for a FileID
func (fs *FileService) GetFileContent(fileID types.FileID) ([]byte, bool) {
	return fs.contentStore.GetContent(fileID)
}

// ReadFile reads the content of a file directly from disk
func (fs *FileService) ReadFile(path string) ([]byte, error) {
	return fs.fileSystem.ReadFile(path)
}

// WriteFile writes content to a file
func (fs *FileService) WriteFile(path string, content []byte, perm fs.FileMode) error {
	// Use direct filesystem access for writing since this is configuration management
	if _, ok := fs.fileSystem.(*RealFileSystem); ok {
		return os.WriteFile(path, content, perm)
	}
	return errors.New("write operations not supported on this filesystem interface")
}

// MkdirAll creates all directories in the path
func (fs *FileService) MkdirAll(path string, perm fs.FileMode) error {
	// Use direct filesystem access for directory creation
	if _, ok := fs.fileSystem.(*RealFileSystem); ok {
		return os.MkdirAll(path, perm)
	}
	return errors.New("directory creation not supported on this filesystem interface")
}

// File System Operations

// Exists checks if a file or directory exists
func (fs *FileService) Exists(path string) bool {
	// Check cache first
	fs.mu.RLock()
	if metadata, exists := fs.fileInfo[path]; exists {
		fs.mu.RUnlock()
		return metadata.Exists
	}
	fs.mu.RUnlock()

	// Check filesystem
	exists := fs.fileSystem.Exists(path)

	// Cache the result
	fs.cacheFileMetadata(path, exists)

	return exists
}

// IsDir checks if a path is a directory
func (fs *FileService) IsDir(path string) bool {
	metadata, err := fs.getFileMetadata(path)
	if err != nil {
		return false
	}
	return metadata.IsDir
}

// IsFile checks if a path is a regular file
func (fs *FileService) IsFile(path string) bool {
	metadata, err := fs.getFileMetadata(path)
	if err != nil {
		return false
	}
	return metadata.Exists && !metadata.IsDir
}

// GetFileSize returns the size of a file in bytes
func (fs *FileService) GetFileSize(path string) (int64, error) {
	metadata, err := fs.getFileMetadata(path)
	if err != nil {
		return 0, err
	}

	if !metadata.Exists {
		return 0, fmt.Errorf("file does not exist: %s", path)
	}

	return metadata.Size, nil
}

// GetModTime returns the modification time of a file
func (fs *FileService) GetModTime(path string) (time.Time, error) {
	metadata, err := fs.getFileMetadata(path)
	if err != nil {
		return time.Time{}, err
	}

	if !metadata.Exists {
		return time.Time{}, fmt.Errorf("file does not exist: %s", path)
	}

	return metadata.ModTime, nil
}

// Directory Operations

// ListFiles returns all files in a directory (non-recursive)
func (fs *FileService) ListFiles(dirPath string) ([]string, error) {
	// Check if it's a directory
	if !fs.IsDir(dirPath) {
		return nil, fmt.Errorf("not a directory: %s", dirPath)
	}

	// Check cache first
	fs.mu.RLock()
	if cached, exists := fs.directoryCache[dirPath]; exists {
		fs.mu.RUnlock()
		return cached, nil
	}
	fs.mu.RUnlock()

	// Read directory
	entries, err := fs.fileSystem.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()

		// Apply ignore filters
		if fs.shouldIgnoreFile(name) {
			continue
		}

		fullPath := filepath.Join(dirPath, name)
		if !entry.IsDir() {
			files = append(files, fullPath)
		}
	}

	// Cache the result
	fs.mu.Lock()
	fs.directoryCache[dirPath] = files
	fs.mu.Unlock()

	return files, nil
}

// ListGoFiles returns all Go files in a directory
func (fs *FileService) ListGoFiles(dirPath string) ([]string, error) {
	allFiles, err := fs.ListFiles(dirPath)
	if err != nil {
		return nil, err
	}

	var goFiles []string
	for _, file := range allFiles {
		if strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") {
			goFiles = append(goFiles, file)
		}
	}

	return goFiles, nil
}

// ListJSFiles returns all JavaScript/TypeScript files in a directory
func (fs *FileService) ListJSFiles(dirPath string) ([]string, error) {
	allFiles, err := fs.ListFiles(dirPath)
	if err != nil {
		return nil, err
	}

	var jsFiles []string
	for _, file := range allFiles {
		ext := filepath.Ext(file)
		if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" ||
			ext == ".mjs" || ext == ".cjs" {
			jsFiles = append(jsFiles, file)
		}
	}

	return jsFiles, nil
}

// FindFilesWithExtensions finds all files with given extensions in a directory
func (fs *FileService) FindFilesWithExtensions(dirPath string, extensions []string) ([]string, error) {
	allFiles, err := fs.ListFiles(dirPath)
	if err != nil {
		return nil, err
	}

	// Create extension map for fast lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[ext] = true
	}

	var matchingFiles []string
	for _, file := range allFiles {
		if extMap[filepath.Ext(file)] {
			matchingFiles = append(matchingFiles, file)
		}
	}

	return matchingFiles, nil
}

// Cache Management

// InvalidateFile removes file from caches
func (fs *FileService) InvalidateFile(path string) {
	fs.contentStore.InvalidateFile(path)

	fs.mu.Lock()
	delete(fs.fileInfo, path)

	// Invalidate directory cache for parent directory
	dir := filepath.Dir(path)
	delete(fs.directoryCache, dir)
	fs.mu.Unlock()
}

// InvalidateDirectory removes directory from caches
func (fs *FileService) InvalidateDirectory(dirPath string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Remove directory cache
	delete(fs.directoryCache, dirPath)

	// Remove cached file metadata for files in this directory
	for path := range fs.fileInfo {
		if strings.HasPrefix(path, dirPath+string(filepath.Separator)) {
			delete(fs.fileInfo, path)
		}
	}
}

// ClearCache clears all caches
func (fs *FileService) ClearCache() {
	fs.contentStore.Clear()

	fs.mu.Lock()
	fs.fileInfo = make(map[string]FileMetadata)
	fs.directoryCache = make(map[string][]string)
	fs.mu.Unlock()
}

// Content Store Access

// GetContentStore returns the underlying content store for direct access when needed
func (fs *FileService) GetContentStore() *FileContentStore {
	return fs.contentStore
}

// Path Resolution

// GetFileIDForPath returns the FileID for a path, or 0 if not loaded
func (fs *FileService) GetFileIDForPath(path string) types.FileID {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if metadata, exists := fs.fileInfo[path]; exists {
		return metadata.FileID
	}
	return 0
}

// GetPathForFileID returns the path for a FileID, or empty string if not found
func (fs *FileService) GetPathForFileID(fileID types.FileID) string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	for path, metadata := range fs.fileInfo {
		if metadata.FileID == fileID {
			return path
		}
	}
	return ""
}

// GetAllLoadedFiles returns a map of path -> FileID for all loaded files
func (fs *FileService) GetAllLoadedFiles() map[string]types.FileID {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	result := make(map[string]types.FileID)
	for path, metadata := range fs.fileInfo {
		if metadata.FileID != 0 {
			result[path] = metadata.FileID
		}
	}
	return result
}

// GetFilePath returns the file path for a FileID using O(1) reverse lookup
func (fs *FileService) GetFilePath(fileID types.FileID) (string, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Use reverse mapping for O(1) lookup instead of O(n) iteration
	if path, exists := fs.fileIDToPath[fileID]; exists {
		return path, true
	}
	return "", false
}

// Private helper methods

func (fs *FileService) getFileMetadata(path string) (FileMetadata, error) {
	// Check cache first
	fs.mu.RLock()
	if metadata, exists := fs.fileInfo[path]; exists {
		fs.mu.RUnlock()
		return metadata, nil
	}
	fs.mu.RUnlock()

	// Get from filesystem
	info, err := fs.fileSystem.Stat(path)
	var metadata FileMetadata

	if err != nil {
		if os.IsNotExist(err) {
			metadata = FileMetadata{
				Path:   path,
				Exists: false,
			}
		} else {
			return FileMetadata{}, err
		}
	} else {
		metadata = FileMetadata{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			Exists:  true,
		}
	}

	// Cache the result
	fs.mu.Lock()
	fs.fileInfo[path] = metadata
	fs.mu.Unlock()

	return metadata, nil
}

func (fs *FileService) cacheFileMetadata(path string, exists bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, already := fs.fileInfo[path]; !already {
		fs.fileInfo[path] = FileMetadata{
			Path:   path,
			Exists: exists,
		}
	}
}

func (fs *FileService) shouldIgnoreFile(name string) bool {
	// Ignore dot files if configured
	if fs.ignoreDotFiles && strings.HasPrefix(name, ".") {
		return true
	}

	// Check ignore patterns
	for _, pattern := range fs.ignorePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}

	return false
}

// Close releases resources held by the FileService
func (fs *FileService) Close() {
	if fs.contentStore != nil {
		fs.contentStore.Close()
	}
}

// NOTE: EstimateDirectorySize function has been REMOVED.
//
// REASON: The function used broken pattern matching for exclusion patterns.
// Pattern "**/.git/**" would NOT match root ".git" directory because it used
// naive string stripping (TrimSuffix "/*") instead of proper glob matching.
//
// REPLACEMENT: Use indexing.FileScanner.CountFiles() for accurate file counting
// with proper glob pattern matching that matches the actual indexing behavior.
//
// MIGRATION:
//   Old: estimate, err := fileService.EstimateDirectorySize(root, maxMB, include, exclude)
//   New: scanner := indexing.NewFileScanner(cfg, 100)
//        fileCount, totalBytes, err := scanner.CountFiles(ctx, root)
