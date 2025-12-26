// Testing infrastructure for FileContentStore - provides builder API for creating
// mock file systems in memory without disk I/O.
//
// This enables fast, deterministic testing of indexing pipelines and dependent services.
package core

import (
	"github.com/standardbeagle/lci/internal/types"
	"time"
)

// MockFile represents a single file in an in-memory file system for testing.
// Files are ordered - the array position determines processing order.
type MockFile struct {
	// Path is the virtual file path (e.g., "src/main.go")
	Path string

	// Content is the file content as bytes
	Content []byte

	// Language is optional language hint for the parser
	// If empty, will be inferred from file extension
	Language string

	// ModTime is optional modification time for the file
	// If zero, uses current time
	ModTime time.Time
}

// MockFileSystem represents an ordered collection of files for testing.
// The order of Files determines processing order in the indexer.
type MockFileSystem struct {
	// Files is the ordered array of mock files
	Files []MockFile

	// RootPath is the virtual root directory (default: "/test")
	RootPath string
}

// FileContentStoreBuilder provides a fluent API for building test file systems.
//
// Example usage:
//
//	builder := NewFileContentStoreBuilder().
//		WithFile("main.go", []byte("package main\n\nfunc main() {}")).
//		WithFile("util.go", []byte("package util\n\nfunc Helper() {}"))
//
//	store := builder.Build()
//	fileIDs := builder.GetFileIDs()
type FileContentStoreBuilder struct {
	files    []MockFile
	rootPath string
	store    *FileContentStore
	fileIDs  []types.FileID
}

// NewFileContentStoreBuilder creates a new builder for test file systems
func NewFileContentStoreBuilder() *FileContentStoreBuilder {
	return &FileContentStoreBuilder{
		files:    make([]MockFile, 0),
		rootPath: "/test",
	}
}

// NewMockFileSystem creates a new mock file system from a map of files.
// The order is deterministic but based on map iteration, so prefer
// NewFileContentStoreBuilder for explicit ordering.
func NewMockFileSystem(files map[string]string) *MockFileSystem {
	mockFiles := make([]MockFile, 0, len(files))
	for path, content := range files {
		mockFiles = append(mockFiles, MockFile{
			Path:    path,
			Content: []byte(content),
		})
	}
	return &MockFileSystem{
		Files:    mockFiles,
		RootPath: "/test",
	}
}

// WithFile adds a file to the builder with string content
func (b *FileContentStoreBuilder) WithFile(path string, content string) *FileContentStoreBuilder {
	b.files = append(b.files, MockFile{
		Path:    path,
		Content: []byte(content),
	})
	return b
}

// WithFileBytes adds a file to the builder with byte content
func (b *FileContentStoreBuilder) WithFileBytes(path string, content []byte) *FileContentStoreBuilder {
	b.files = append(b.files, MockFile{
		Path:    path,
		Content: content,
	})
	return b
}

// WithLanguage sets the language hint for the last added file
func (b *FileContentStoreBuilder) WithLanguage(lang string) *FileContentStoreBuilder {
	if len(b.files) > 0 {
		b.files[len(b.files)-1].Language = lang
	}
	return b
}

// WithModTime sets the modification time for the last added file
func (b *FileContentStoreBuilder) WithModTime(modTime time.Time) *FileContentStoreBuilder {
	if len(b.files) > 0 {
		b.files[len(b.files)-1].ModTime = modTime
	}
	return b
}

// WithRootPath sets the virtual root directory path
func (b *FileContentStoreBuilder) WithRootPath(root string) *FileContentStoreBuilder {
	b.rootPath = root
	return b
}

// Build creates a FileContentStore and loads all files into it.
// Returns the store with all files loaded. Use GetFileIDs() to get the FileID mappings.
func (b *FileContentStoreBuilder) Build() *FileContentStore {
	store := NewFileContentStore()
	b.store = store
	b.fileIDs = make([]types.FileID, len(b.files))

	for i, file := range b.files {
		fileID := store.LoadFile(file.Path, file.Content)
		b.fileIDs[i] = fileID
	}

	return store
}

// GetFileIDs returns the FileIDs for all files in the order they were added.
// Must call Build() first.
func (b *FileContentStoreBuilder) GetFileIDs() []types.FileID {
	return b.fileIDs
}

// GetStore returns the built FileContentStore.
// Must call Build() first.
func (b *FileContentStoreBuilder) GetStore() *FileContentStore {
	return b.store
}

// GetFileID returns the FileID for a specific file by path.
// Must call Build() first. Returns 0 if file not found.
func (b *FileContentStoreBuilder) GetFileID(path string) types.FileID {
	for i, file := range b.files {
		if file.Path == path {
			return b.fileIDs[i]
		}
	}
	return 0
}

// GetMockFileSystem returns the MockFileSystem representation
func (b *FileContentStoreBuilder) GetMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		Files:    b.files,
		RootPath: b.rootPath,
	}
}

// LoadIntoStore loads the mock file system into an existing FileContentStore.
// Returns the FileIDs for all loaded files in order.
func (mfs *MockFileSystem) LoadIntoStore(store *FileContentStore) []types.FileID {
	fileIDs := make([]types.FileID, len(mfs.Files))
	for i, file := range mfs.Files {
		fileIDs[i] = store.LoadFile(file.Path, file.Content)
	}
	return fileIDs
}

// LoadBatch loads multiple files into the store using the batch API for efficiency.
// Returns the FileIDs for all loaded files in order.
func (mfs *MockFileSystem) LoadBatch(store *FileContentStore) []types.FileID {
	if len(mfs.Files) == 0 {
		return nil
	}

	// Prepare batch data
	batchData := make([]struct {
		Path    string
		Content []byte
	}, len(mfs.Files))

	for i, file := range mfs.Files {
		batchData[i].Path = file.Path
		batchData[i].Content = file.Content
	}

	return store.BatchLoadFiles(batchData)
}
