package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileContentStoreBuilder_Basic tests the basic builder functionality
func TestFileContentStoreBuilder_Basic(t *testing.T) {
	builder := NewFileContentStoreBuilder().
		WithFile("main.go", "package main\n\nfunc main() {}").
		WithFile("util.go", "package util\n\nfunc Helper() {}")

	store := builder.Build()
	require.NotNil(t, store, "Store should be created")
	defer store.Close()

	fileIDs := builder.GetFileIDs()
	require.Len(t, fileIDs, 2, "Should have 2 files")
	assert.NotZero(t, fileIDs[0], "First file should have valid ID")
	assert.NotZero(t, fileIDs[1], "Second file should have valid ID")

	// Verify content can be retrieved
	content1, ok := store.GetContent(fileIDs[0])
	require.True(t, ok, "Should retrieve first file content")
	assert.Equal(t, "package main\n\nfunc main() {}", string(content1))

	content2, ok := store.GetContent(fileIDs[1])
	require.True(t, ok, "Should retrieve second file content")
	assert.Equal(t, "package util\n\nfunc Helper() {}", string(content2))
}

// TestFileContentStoreBuilder_GetFileID tests looking up FileID by path
func TestFileContentStoreBuilder_GetFileID(t *testing.T) {
	builder := NewFileContentStoreBuilder().
		WithFile("src/main.go", "package main").
		WithFile("src/util.go", "package util")

	store := builder.Build()
	defer store.Close()

	mainID := builder.GetFileID("src/main.go")
	utilID := builder.GetFileID("src/util.go")
	notFoundID := builder.GetFileID("nonexistent.go")

	assert.NotZero(t, mainID, "main.go should have valid ID")
	assert.NotZero(t, utilID, "util.go should have valid ID")
	assert.Zero(t, notFoundID, "nonexistent.go should return 0")
	assert.NotEqual(t, mainID, utilID, "Different files should have different IDs")
}

// TestFileContentStoreBuilder_WithBytes tests adding binary content
func TestFileContentStoreBuilder_WithBytes(t *testing.T) {
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF}

	builder := NewFileContentStoreBuilder().
		WithFileBytes("data.bin", binaryData)

	store := builder.Build()
	defer store.Close()
	fileIDs := builder.GetFileIDs()

	require.Len(t, fileIDs, 1)

	content, ok := store.GetContent(fileIDs[0])
	require.True(t, ok)
	assert.Equal(t, binaryData, content)
}

// TestFileContentStoreBuilder_Ordering tests that file order is preserved
func TestFileContentStoreBuilder_Ordering(t *testing.T) {
	builder := NewFileContentStoreBuilder().
		WithFile("a.go", "package a").
		WithFile("b.go", "package b").
		WithFile("c.go", "package c")

	store := builder.Build()
	defer store.Close()
	fileIDs := builder.GetFileIDs()

	require.Len(t, fileIDs, 3)

	// Verify order is preserved
	assert.Equal(t, builder.GetFileID("a.go"), fileIDs[0])
	assert.Equal(t, builder.GetFileID("b.go"), fileIDs[1])
	assert.Equal(t, builder.GetFileID("c.go"), fileIDs[2])
}

// TestMockFileSystem_LoadIntoStore tests loading a mock file system
func TestMockFileSystem_LoadIntoStore(t *testing.T) {
	mfs := &MockFileSystem{
		Files: []MockFile{
			{Path: "main.go", Content: []byte("package main")},
			{Path: "util.go", Content: []byte("package util")},
		},
		RootPath: "/test",
	}

	store := NewFileContentStore()
	defer store.Close()
	fileIDs := mfs.LoadIntoStore(store)

	require.Len(t, fileIDs, 2)
	assert.NotZero(t, fileIDs[0])
	assert.NotZero(t, fileIDs[1])

	content1, ok := store.GetContent(fileIDs[0])
	require.True(t, ok)
	assert.Equal(t, "package main", string(content1))
}

// TestMockFileSystem_LoadBatch tests batch loading
func TestMockFileSystem_LoadBatch(t *testing.T) {
	mfs := &MockFileSystem{
		Files: []MockFile{
			{Path: "a.go", Content: []byte("package a")},
			{Path: "b.go", Content: []byte("package b")},
			{Path: "c.go", Content: []byte("package c")},
		},
	}

	store := NewFileContentStore()
	defer store.Close()
	fileIDs := mfs.LoadBatch(store)

	require.Len(t, fileIDs, 3)
	assert.NotZero(t, fileIDs[0])
	assert.NotZero(t, fileIDs[1])
	assert.NotZero(t, fileIDs[2])

	// Verify all files loaded correctly
	for i, file := range mfs.Files {
		content, ok := store.GetContent(fileIDs[i])
		require.True(t, ok, "File %s should be retrievable", file.Path)
		assert.Equal(t, string(file.Content), string(content))
	}
}

// TestNewMockFileSystem_FromMap tests creating from a map
func TestNewMockFileSystem_FromMap(t *testing.T) {
	files := map[string]string{
		"main.go": "package main",
		"util.go": "package util",
	}

	mfs := NewMockFileSystem(files)
	require.Len(t, mfs.Files, 2)
	assert.Equal(t, "/test", mfs.RootPath)

	// Verify all files are present
	paths := make(map[string]bool)
	for _, f := range mfs.Files {
		paths[f.Path] = true
	}
	assert.True(t, paths["main.go"])
	assert.True(t, paths["util.go"])
}

// TestFileContentStoreBuilder_EmptyStore tests building an empty store
func TestFileContentStoreBuilder_EmptyStore(t *testing.T) {
	builder := NewFileContentStoreBuilder()
	store := builder.Build()
	defer store.Close()

	require.NotNil(t, store)
	assert.Empty(t, builder.GetFileIDs())
}

// TestFileContentStoreBuilder_GetMockFileSystem tests converting to MockFileSystem
func TestFileContentStoreBuilder_GetMockFileSystem(t *testing.T) {
	builder := NewFileContentStoreBuilder().
		WithFile("main.go", "package main").
		WithRootPath("/custom")

	store := builder.Build()
	defer store.Close()
	mfs := builder.GetMockFileSystem()

	require.NotNil(t, mfs)
	assert.Equal(t, "/custom", mfs.RootPath)
	assert.Len(t, mfs.Files, 1)
	assert.Equal(t, "main.go", mfs.Files[0].Path)
}
