package testing

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
)

// TestFilesystem provides an isolated mock filesystem for LCI testing
type TestFilesystem struct {
	*testing.T
	*MockFilesystem
	root             string
	gitignoreContent string
	config           *config.Config
}

// NewTestFilesystem creates an isolated test filesystem with sensible defaults
func NewTestFilesystem(t *testing.T) *TestFilesystem {
	root := "/test/project"

	return &TestFilesystem{
		T:                t,
		MockFilesystem:   NewMockFilesystem(),
		root:             root,
		gitignoreContent: "",
		config: &config.Config{
			Project: config.Project{
				Root: root,
			},
			Index: config.Index{
				MaxFileSize:      1024 * 1024, // 1MB
				RespectGitignore: true,
			},
			Performance: config.Performance{
				MaxMemoryMB:   1000,
				MaxGoroutines: 4,
			},
		},
	}
}

// SetGitignore configures gitignore patterns for testing
func (tfs *TestFilesystem) SetGitignore(patterns ...string) {
	tfs.gitignoreContent = strings.Join(patterns, "\n")
	_ = tfs.CreateFile(filepath.Join(tfs.root, ".gitignore"), []byte(tfs.gitignoreContent))
}

// AddProjectFile adds a file relative to the test project root
func (tfs *TestFilesystem) AddProjectFile(path, content string) {
	fullPath := filepath.Join(tfs.root, path)
	_ = tfs.CreateFile(fullPath, []byte(content))
}

// AddProjectDir adds a directory relative to the test project root
func (tfs *TestFilesystem) AddProjectDir(path string) {
	fullPath := filepath.Join(tfs.root, path)
	_ = tfs.CreateFile(fullPath, []byte{}) // Empty file represents directory
}

// GetConfig returns the test configuration
func (tfs *TestFilesystem) GetConfig() *config.Config {
	return tfs.config
}

// GetGitignoreContent returns the current gitignore content
func (tfs *TestFilesystem) GetGitignoreContent() string {
	return tfs.gitignoreContent
}

// CreateFileService creates a FileService backed by this mock filesystem
func (tfs *TestFilesystem) CreateFileService() *core.FileService {
	return core.NewFileServiceWithOptions(core.FileServiceOptions{
		FileSystem: &TestFilesystemAdapter{TestFilesystem: tfs},
	})
}

// TestFilesystemAdapter adapts TestFilesystem to core.FileSystemInterface
type TestFilesystemAdapter struct {
	*TestFilesystem
}

// Stat implements core.FileSystemInterface
func (tfsa *TestFilesystemAdapter) Stat(path string) (fs.FileInfo, error) {
	return nil, &FileNotFoundError{Path: path}
}

// ReadFile implements core.FileSystemInterface
func (tfsa *TestFilesystemAdapter) ReadFile(path string) ([]byte, error) {
	return tfsa.TestFilesystem.ReadFile(path)
}

// ReadDir implements core.FileSystemInterface
func (tfsa *TestFilesystemAdapter) ReadDir(path string) ([]fs.DirEntry, error) {
	return nil, &FileNotFoundError{Path: path}
}

// Exists implements core.FileSystemInterface
func (tfsa *TestFilesystemAdapter) Exists(path string) bool {
	tfsa.MockFilesystem.mu.RLock()
	defer tfsa.MockFilesystem.mu.RUnlock()

	file, exists := tfsa.MockFilesystem.files[path]
	return exists && file.Exists
}

// IsDir implements core.FileSystemInterface
func (tfsa *TestFilesystemAdapter) IsDir(path string) bool {
	tfsa.MockFilesystem.mu.RLock()
	defer tfsa.MockFilesystem.mu.RUnlock()

	file, exists := tfsa.MockFilesystem.files[path]
	return exists && file.Exists && file.IsDirectory
}

// IsFile implements core.FileSystemInterface
func (tfsa *TestFilesystemAdapter) IsFile(path string) bool {
	tfsa.MockFilesystem.mu.RLock()
	defer tfsa.MockFilesystem.mu.RUnlock()

	file, exists := tfsa.MockFilesystem.files[path]
	return exists && file.Exists && !file.IsDirectory
}

// FileServiceAdapter adapts MockFilesystem to core.FileService interface
type FileServiceAdapter struct {
	*MockFilesystem
}

// NewFileServiceAdapter creates a new adapter
func NewFileServiceAdapter(mfs *MockFilesystem) *FileServiceAdapter {
	return &FileServiceAdapter{MockFilesystem: mfs}
}

// ReadFile implements core.FileService interface
func (fsa *FileServiceAdapter) ReadFile(path string) ([]byte, error) {
	return fsa.MockFilesystem.ReadFile(path)
}

// FileExists implements core.FileService interface
func (fsa *FileServiceAdapter) FileExists(path string) bool {
	fsa.MockFilesystem.mu.RLock()
	defer fsa.MockFilesystem.mu.RUnlock()

	file, exists := fsa.MockFilesystem.files[path]
	return exists && file.Exists && !file.IsDirectory
}

// IsDirectory implements core.FileService interface
func (fsa *FileServiceAdapter) IsDirectory(path string) bool {
	fsa.MockFilesystem.mu.RLock()
	defer fsa.MockFilesystem.mu.RUnlock()

	file, exists := fsa.MockFilesystem.files[path]
	return exists && file.Exists && file.IsDirectory
}

// GetModTime implements core.FileService interface
func (fsa *FileServiceAdapter) GetModTime(path string) (int64, error) {
	fsa.MockFilesystem.mu.RLock()
	defer fsa.MockFilesystem.mu.RUnlock()

	file, exists := fsa.MockFilesystem.files[path]
	if !exists || !file.Exists {
		return 0, &FileNotFoundError{Path: path}
	}

	return file.ModTime.Unix(), nil
}

// GetSize implements core.FileService interface
func (fsa *FileServiceAdapter) GetSize(path string) (int64, error) {
	fsa.MockFilesystem.mu.RLock()
	defer fsa.MockFilesystem.mu.RUnlock()

	file, exists := fsa.MockFilesystem.files[path]
	if !exists || !file.Exists {
		return 0, &FileNotFoundError{Path: path}
	}

	return file.Size, nil
}

// FileNotFoundError represents a file not found error
type FileNotFoundError struct {
	Path string
}

func (e *FileNotFoundError) Error() string {
	return "file not found: " + e.Path
}

// Helper methods for common test scenarios using project generators

// CreateNodeProject creates a typical Node.js project structure using the NodeProjectGenerator
func (tfs *TestFilesystem) CreateNodeProject() {
	// Set gitignore patterns from generator
	patterns := []string{
		"node_modules/",
		"dist/",
		"build/",
		"*.log",
		".DS_Store",
	}
	tfs.SetGitignore(patterns...)

	// Create project files using the generator's file templates
	for path, content := range NodeProjectFiles() {
		tfs.AddProjectFile(path, content)
	}
}

// CreateGoProject creates a typical Go project structure using the GoProjectGenerator
func (tfs *TestFilesystem) CreateGoProject() {
	// Set gitignore patterns from generator
	patterns := []string{
		"vendor/",
		"*.exe",
		"*.exe~",
		"*.dll",
		"*.so",
		"*.dylib",
		"*.test",
		"*.out",
	}
	tfs.SetGitignore(patterns...)

	// Create project files using the generator's file templates
	for path, content := range GoProjectFiles() {
		tfs.AddProjectFile(path, content)
	}
}

// CreateWebProject creates a modern web project with Next.js-like structure using the WebProjectGenerator
func (tfs *TestFilesystem) CreateWebProject() {
	// Set gitignore patterns from generator
	patterns := []string{
		".next/",
		"node_modules/",
		"out/",
		"dist/",
		"build/",
		".env.local",
		"*.log",
	}
	tfs.SetGitignore(patterns...)

	// Create project files using the generator's file templates
	for path, content := range webProjectFiles() {
		tfs.AddProjectFile(path, content)
	}
}

// CreateComplexProject creates a project with various edge cases for testing gitignore functionality
func (tfs *TestFilesystem) CreateComplexProject() {
	tfs.SetGitignore(
		"node_modules/",
		"*.min.js",
		"*.log",
		"temp/",
		".env",
	)

	// Various file types and patterns
	tfs.AddProjectFile("src/main.js", "console.log('main');")
	tfs.AddProjectFile("src/utils.js", "export function helper() { return 'helper'; }")
	tfs.AddProjectFile("src/app.min.js", "console.log('minified');")   // Should be excluded
	tfs.AddProjectFile("debug.log", "2023-01-01: Application started") // Should be excluded
	tfs.AddProjectFile("temp/cache.tmp", "temporary cache data")       // Should be excluded
	tfs.AddProjectFile(".env", "SECRET_KEY=abc123")                    // Should be excluded

	// Nested directories
	tfs.AddProjectFile("src/components/Header.jsx", "export function Header() { return <header/>; }")
	tfs.AddProjectFile("src/components/Footer.jsx", "export function Footer() { return <footer/>; }")
	tfs.AddProjectFile("src/utils/helpers/date.js", "export function formatDate() { return new Date(); }")
	tfs.AddProjectFile("src/utils/helpers/string.js", "export function capitalize(str) { return str.toUpperCase(); }")

	// Files that should be included
	tfs.AddProjectFile("README.md", "# Project Documentation")
	tfs.AddProjectFile("LICENSE", "MIT License")
	tfs.AddProjectFile("config.json", `{"timeout": 5000, "retries": 3}`)
}
