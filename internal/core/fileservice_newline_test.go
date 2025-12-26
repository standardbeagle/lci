package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileServicePreservesNewlines verifies FileService doesn't strip newlines
func TestFileServicePreservesNewlines(t *testing.T) {
	fixtureDir := "../../tests/search-comparison/fixtures/rust-sample"
	absFixtureDir, _ := filepath.Abs(fixtureDir)
	authPath := filepath.Join(absFixtureDir, "src/auth.rs")

	// Skip test if file doesn't exist
	if _, err := os.Stat(authPath); os.IsNotExist(err) {
		t.Skip("Test fixture not found")
	}

	// Read file directly to establish baseline
	directContent, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("Failed to read file directly: %v", err)
	}

	directNewlines := 0
	for _, b := range directContent {
		if b == '\n' {
			directNewlines++
		}
	}
	t.Logf("Direct os.ReadFile: %d newlines, %d bytes", directNewlines, len(directContent))

	// Create FileService
	contentStore := NewFileContentStore()
	defer contentStore.Close()
	fileService := NewFileServiceWithOptions(FileServiceOptions{
		ContentStore:     contentStore,
		FileSystem:       &RealFileSystem{},
		MaxFileSizeBytes: 10 * 1024 * 1024,
	})

	// Load through FileService
	fileID, err := fileService.LoadFile(authPath)
	if err != nil {
		t.Fatalf("Error loading file: %v", err)
	}

	// Get content back from store
	storedContent, ok := contentStore.GetContent(fileID)
	if !ok {
		t.Fatal("Failed to get content from store")
	}

	storedNewlines := 0
	for _, b := range storedContent {
		if b == '\n' {
			storedNewlines++
		}
	}
	t.Logf("FileContentStore: %d newlines, %d bytes", storedNewlines, len(storedContent))

	// Verify lengths match
	if len(directContent) != len(storedContent) {
		t.Errorf("LENGTH MISMATCH: direct=%d stored=%d", len(directContent), len(storedContent))
	}

	// Verify newline counts match
	if directNewlines != storedNewlines {
		t.Errorf("NEWLINE MISMATCH: direct=%d stored=%d", directNewlines, storedNewlines)
	}

	// Verify byte-for-byte equality
	for i := range directContent {
		if i >= len(storedContent) {
			t.Errorf("Stored content is shorter at position %d", i)
			break
		}
		if directContent[i] != storedContent[i] {
			t.Errorf("BYTE MISMATCH at position %d: direct=%q stored=%q", i, directContent[i], storedContent[i])
			break
		}
	}
}
