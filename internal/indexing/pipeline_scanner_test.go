package indexing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

func TestFileScanner_PreCheckBinaryFile(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Helper to create large text content
	createLargeText := func(sizeKB int) []byte {
		var content []byte
		for i := 0; i < sizeKB; i++ {
			content = append(content, []byte("// This is a comment line with some text content to fill up space\n")...)
			content = append(content, []byte("func example"+string(rune('0'+i%10))+"() { return nil }\n")...)
		}
		return content
	}

	// Create test files
	tests := []struct {
		name       string
		content    []byte
		wantBinary bool
	}{
		{
			name:       "large_png_file",
			content:    append([]byte{0x89, 0x50, 0x4E, 0x47}, make([]byte, 200*1024)...), // PNG magic + 200KB
			wantBinary: true,
		},
		{
			name:       "large_text_file",
			content:    []byte("package main\n\nfunc main() {\n\t" + string(createLargeText(200)) + "\n}"),
			wantBinary: false,
		},
		{
			name:       "large_zip_file",
			content:    append([]byte{0x50, 0x4B, 0x03, 0x04}, make([]byte, 150*1024)...), // ZIP magic + 150KB
			wantBinary: true,
		},
		{
			name:       "large_source_with_comments",
			content:    []byte("// Source file\n" + string(createLargeText(120)) + "\npackage test\n"),
			wantBinary: false,
		},
	}

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
		Index: config.Index{
			MaxFileSize: types.DefaultMaxFileSize,
		},
	}

	scanner := NewFileScanner(cfg, 100)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(filePath, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer os.Remove(filePath)

			// Test pre-check
			got := scanner.preCheckBinaryFile(filePath)
			if got != tt.wantBinary {
				t.Errorf("preCheckBinaryFile(%q) = %v, want %v", tt.name, got, tt.wantBinary)
			}
		})
	}
}

func TestFileScanner_ShouldProcessFile_BinaryPreCheck(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper to create large text content
	createLargeText := func(sizeKB int) []byte {
		var content []byte
		for i := 0; i < sizeKB; i++ {
			content = append(content, []byte("// This is a comment line with some text content\n")...)
			content = append(content, []byte("func example() { return nil }\n")...)
		}
		return content
	}

	tests := []struct {
		name          string
		fileName      string
		content       []byte
		expectProcess bool
		description   string
	}{
		{
			name:          "small_binary_below_threshold",
			fileName:      "small.txt",                    // Use .txt extension so it's not filtered by extension
			content:       []byte{0x89, 0x50, 0x4E, 0x47}, // PNG magic, 4 bytes
			expectProcess: true,                           // Below threshold, won't be pre-checked (caught later in processor)
			description:   "Small binary files below threshold are not pre-checked",
		},
		{
			name:          "large_binary_above_threshold",
			fileName:      "large.dat",
			content:       append([]byte{0x50, 0x4B, 0x03, 0x04}, make([]byte, 200*1024)...), // ZIP magic + 200KB
			expectProcess: false,
			description:   "Large binary files above threshold are rejected during enumeration",
		},
		{
			name:          "large_text_above_threshold",
			fileName:      "large.go",
			content:       []byte("package main\n\n" + string(createLargeText(150)) + "\nfunc main() {}\n"),
			expectProcess: true,
			description:   "Large text files above threshold pass pre-check",
		},
		{
			name:          "medium_text_at_threshold",
			fileName:      "medium.go",
			content:       []byte("package main\n\n" + string(createLargeText(100)) + "\nfunc main() {}\n"),
			expectProcess: true,
			description:   "Text files at threshold boundary are processed",
		},
		{
			name:          "binary_by_extension",
			fileName:      "image.png",
			content:       []byte("actually text content"), // Extension check happens first
			expectProcess: false,
			description:   "Binary extension check happens before size pre-check",
		},
	}

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
		Index: config.Index{
			MaxFileSize:      types.DefaultMaxFileSize,
			RespectGitignore: false,
		},
		Include: []string{"**/*"}, // Include everything for testing
		Exclude: []string{},
	}

	scanner := NewFileScanner(cfg, 100)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpDir, tt.fileName)
			if err := os.WriteFile(filePath, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer os.Remove(filePath)

			// Get file info
			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("Failed to stat test file: %v", err)
			}

			// Test shouldProcessFile
			got := scanner.shouldProcessFile(filePath, info)
			if got != tt.expectProcess {
				t.Errorf("%s: shouldProcessFile() = %v, want %v\nDescription: %s",
					tt.name, got, tt.expectProcess, tt.description)
			}
		})
	}
}

func TestFileScanner_PreCheckNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
	}

	scanner := NewFileScanner(cfg, 100)

	// Test that non-existent file is treated as binary (skipped)
	nonExistentPath := filepath.Join(tmpDir, "does_not_exist.txt")
	isBinary := scanner.preCheckBinaryFile(nonExistentPath)
	if !isBinary {
		t.Error("preCheckBinaryFile() should return true for non-existent files")
	}
}

func TestFileScanner_PreCheckEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}
	defer os.Remove(filePath)

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
	}

	scanner := NewFileScanner(cfg, 100)

	// Empty files should not be detected as binary
	isBinary := scanner.preCheckBinaryFile(filePath)
	if isBinary {
		t.Error("preCheckBinaryFile() should return false for empty files")
	}
}
