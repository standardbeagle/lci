package indexing

import (
	"testing"
)

func TestBinaryDetector_IsBinaryByExtension(t *testing.T) {
	bd := NewBinaryDetector()

	tests := []struct {
		path   string
		binary bool
	}{
		// Binary files
		{"/path/to/font.woff2", true},
		{"/path/to/font.woff", true},
		{"/path/to/font.ttf", true},
		{"/path/to/image.png", true},
		{"/path/to/image.jpg", true},
		{"/path/to/archive.zip", true},
		{"/path/to/binary.exe", true},
		{"/path/to/library.so", true},
		{"/path/to/doc.pdf", true},
		{"/path/to/bytecode.pyc", true},
		{"/path/to/db.sqlite", true},

		// Text files
		{"/path/to/source.go", false},
		{"/path/to/source.js", false},
		{"/path/to/source.py", false},
		{"/path/to/image.svg", false}, // SVG is XML text
		{"/path/to/config.json", false},
		{"/path/to/readme.md", false},
		{"/path/to/source.min.js", false},  // Minified JS is still text
		{"/path/to/source.min.css", false}, // Minified CSS is still text
		{"/path/to/source.map", false},     // Source maps are JSON

		// Test case insensitivity
		{"/path/to/image.PNG", true},  // Uppercase PNG
		{"/path/to/image.Jpg", true},  // Mixed case JPG
		{"/path/to/font.WOFF2", true}, // Uppercase WOFF2
		{"/path/to/source.GO", false}, // Uppercase GO source
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := bd.IsBinaryByExtension(tt.path)
			if got != tt.binary {
				t.Errorf("IsBinaryByExtension(%q) = %v, want %v", tt.path, got, tt.binary)
			}
		})
	}
}

func TestBinaryDetector_IsBinaryByMagicNumber(t *testing.T) {
	bd := NewBinaryDetector()

	tests := []struct {
		name    string
		content []byte
		binary  bool
	}{
		{
			name:    "PNG file",
			content: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			binary:  true,
		},
		{
			name:    "JPEG file",
			content: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46},
			binary:  true,
		},
		{
			name:    "GIF file",
			content: []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
			binary:  true,
		},
		{
			name:    "ZIP file",
			content: []byte{0x50, 0x4B, 0x03, 0x04},
			binary:  true,
		},
		{
			name:    "PDF file",
			content: []byte{0x25, 0x50, 0x44, 0x46, 0x2D},
			binary:  true,
		},
		{
			name:    "gzip file",
			content: []byte{0x1F, 0x8B, 0x08},
			binary:  true,
		},
		{
			name:    "ELF executable",
			content: []byte{0x7F, 0x45, 0x4C, 0x46, 0x02, 0x01, 0x01},
			binary:  true,
		},
		{
			name:    "Windows executable",
			content: []byte{0x4D, 0x5A, 0x90, 0x00},
			binary:  true,
		},
		{
			name:    "WOFF font",
			content: []byte{0x77, 0x4F, 0x46, 0x46},
			binary:  true,
		},
		{
			name:    "WOFF2 font",
			content: []byte{0x77, 0x4F, 0x46, 0x32},
			binary:  true,
		},
		{
			name:    "Plain text Go source",
			content: []byte("package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}\n"),
			binary:  false,
		},
		{
			name:    "JSON file",
			content: []byte("{\"key\": \"value\", \"number\": 42}"),
			binary:  false,
		},
		{
			name:    "UTF-8 text",
			content: []byte("Hello, 世界! 🚀 Unicode works fine."),
			binary:  false,
		},
		{
			name:    "Empty file",
			content: []byte{},
			binary:  false,
		},
		{
			name: "Binary with null bytes",
			content: []byte{
				'h', 'e', 'l', 'l', 'o', 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			binary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bd.IsBinaryByMagicNumber(tt.content)
			if got != tt.binary {
				t.Errorf("IsBinaryByMagicNumber() = %v, want %v", got, tt.binary)
			}
		})
	}
}

func TestBinaryDetector_IsBinary(t *testing.T) {
	bd := NewBinaryDetector()

	tests := []struct {
		name    string
		path    string
		content []byte
		binary  bool
	}{
		{
			name:    "PNG file by extension",
			path:    "/path/to/image.png",
			content: []byte("actually text, but PNG extension"),
			binary:  true, // Rejected by extension before content check
		},
		{
			name:    "Binary file no extension",
			path:    "/path/to/noext",
			content: []byte{0x50, 0x4B, 0x03, 0x04}, // ZIP magic
			binary:  true,                           // Detected by magic number
		},
		{
			name:    "Text file",
			path:    "/path/to/file.go",
			content: []byte("package main\n"),
			binary:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bd.IsBinary(tt.path, tt.content)
			if got != tt.binary {
				t.Errorf("IsBinary() = %v, want %v", got, tt.binary)
			}
		})
	}
}
