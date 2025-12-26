package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

// TestGetFilePreservesNewlines verifies GetFile doesn't strip newlines
func TestGetFilePreservesNewlines(t *testing.T) {
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

	// Create index
	cfg := &config.Config{
		Project: config.Project{
			Root: absFixtureDir,
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Search: config.Search{
			MaxResults: 1000,
		},
	}

	gi := NewMasterIndex(cfg)
	ctx := context.Background()
	err = gi.IndexDirectory(ctx, absFixtureDir)
	if err != nil {
		t.Fatalf("Failed to index: %v", err)
	}

	// Find auth.rs file ID
	var authFileID types.FileID
	fileIDs := gi.GetAllFileIDs()
	for _, fileID := range fileIDs {
		fileInfo := gi.GetFile(fileID)
		if fileInfo != nil && filepath.Base(fileInfo.Path) == "auth.rs" {
			authFileID = fileID
			break
		}
	}

	if authFileID == 0 {
		t.Fatal("auth.rs not found in index")
	}

	// First, check FileContentStore directly
	storeContent, ok := gi.fileContentStore.GetContent(authFileID)
	if !ok {
		t.Fatal("Content not in FileContentStore")
	}

	storeNewlines := 0
	for _, b := range storeContent {
		if b == '\n' {
			storeNewlines++
		}
	}
	t.Logf("FileContentStore.GetContent: %d newlines, %d bytes", storeNewlines, len(storeContent))

	// Get file info
	fileInfo := gi.GetFile(authFileID)
	if fileInfo == nil {
		t.Fatal("FileInfo is nil")
	}

	t.Logf("FileInfo.Path: %s", fileInfo.Path)
	t.Logf("FileInfo.Content length: %d", len(fileInfo.Content))
	t.Logf("FileInfo.Lines length: %d", len(fileInfo.Lines))
	t.Logf("Are Content pointers equal? %v", &storeContent[0] == &fileInfo.Content[0])

	// Count newlines in FileInfo.Content
	fileInfoNewlines := 0
	for i, b := range fileInfo.Content {
		if b == '\n' {
			fileInfoNewlines++
			if fileInfoNewlines <= 3 || fileInfoNewlines >= 40 {
				t.Logf("Newline %d at byte %d", fileInfoNewlines, i)
			}
		}
	}
	t.Logf("FileInfo.Content: %d newlines", fileInfoNewlines)

	// Verify lengths match
	if len(directContent) != len(fileInfo.Content) {
		t.Errorf("LENGTH MISMATCH: direct=%d fileInfo=%d", len(directContent), len(fileInfo.Content))
	}

	// Verify newline counts match
	if directNewlines != fileInfoNewlines {
		t.Errorf("NEWLINE MISMATCH: direct=%d fileInfo=%d", directNewlines, fileInfoNewlines)

		// Debug: Check last bytes
		t.Logf("Direct last 30 bytes: %q", string(directContent[len(directContent)-30:]))
		t.Logf("FileInfo last 30 bytes: %q", string(fileInfo.Content[len(fileInfo.Content)-30:]))
	}
}
