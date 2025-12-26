// Binary file detection utility for early rejection of non-text files
// Prevents tree-sitter from attempting to parse binary data as source code
package indexing

import (
	"bytes"
	"path/filepath"
	"strings"
)

// BinaryDetector handles detection of binary files that should not be indexed
type BinaryDetector struct {
	binaryExtensions map[string]bool
}

// NewBinaryDetector creates a new binary file detector with comprehensive extension database
func NewBinaryDetector() *BinaryDetector {
	extensions := map[string]bool{
		// Font files
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".otf":   true,
		".eot":   true,

		// Image files
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".bmp":  true,
		".ico":  true,
		".webp": true,
		".svg":  false, // SVG is text-based XML
		".tiff": true,
		".tif":  true,

		// Archive files
		".zip": true,
		".tar": true,
		".gz":  true,
		".bz2": true,
		".xz":  true,
		".7z":  true,
		".rar": true,
		".jar": true,
		".war": true,
		".ear": true,

		// Binary executables
		".exe":   true,
		".dll":   true,
		".so":    true,
		".dylib": true,
		".a":     true,
		".o":     true,
		".obj":   true,
		".bin":   true,

		// Media files
		".mp3":  true,
		".mp4":  true,
		".avi":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".wav":  true,
		".flac": true,
		".ogg":  true,

		// Document files (binary formats)
		".pdf":  true,
		".doc":  true,
		".docx": true,
		".xls":  true,
		".xlsx": true,
		".ppt":  true,
		".pptx": true,

		// Database files
		".db":      true,
		".sqlite":  true,
		".sqlite3": true,

		// Compiled/minified assets
		".min.js":  false, // JavaScript, but minified (allow)
		".min.css": false, // CSS, but minified (allow)
		".map":     false, // Source maps are JSON (allow)

		// Protocol buffers
		".proto": false, // Text-based (allow)

		// Other binary formats
		".pyc":    true, // Python bytecode
		".pyo":    true, // Python optimized bytecode
		".class":  true, // Java bytecode
		".pickle": true,
		".pkl":    true,
	}

	return &BinaryDetector{
		binaryExtensions: extensions,
	}
}

// IsBinaryByExtension checks if a file is binary based on its extension
func (bd *BinaryDetector) IsBinaryByExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}

	// Handle compound extensions like .min.js
	if strings.Contains(path, ".min.") {
		if strings.HasSuffix(path, ".min.js") || strings.HasSuffix(path, ".min.css") {
			return false // Minified text files are still text
		}
	}

	isBinary, exists := bd.binaryExtensions[ext]
	return exists && isBinary
}

// IsBinaryByMagicNumber checks if content is binary using magic number detection
// This is a fast heuristic check for the first 512 bytes
func (bd *BinaryDetector) IsBinaryByMagicNumber(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check first 512 bytes (standard for file type detection)
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}
	sample := content[:checkLen]

	// Common binary file signatures (magic numbers)
	if bytes.HasPrefix(sample, []byte{0x1F, 0x8B}) {
		return true // gzip
	}
	if bytes.HasPrefix(sample, []byte{0x50, 0x4B, 0x03, 0x04}) ||
		bytes.HasPrefix(sample, []byte{0x50, 0x4B, 0x05, 0x06}) {
		return true // ZIP
	}
	if bytes.HasPrefix(sample, []byte{0x89, 0x50, 0x4E, 0x47}) {
		return true // PNG
	}
	if bytes.HasPrefix(sample, []byte{0xFF, 0xD8, 0xFF}) {
		return true // JPEG
	}
	if bytes.HasPrefix(sample, []byte{0x47, 0x49, 0x46, 0x38}) {
		return true // GIF
	}
	if bytes.HasPrefix(sample, []byte{0x25, 0x50, 0x44, 0x46}) {
		return true // PDF
	}
	if bytes.HasPrefix(sample, []byte{0x7F, 0x45, 0x4C, 0x46}) {
		return true // ELF (Linux executable)
	}
	if bytes.HasPrefix(sample, []byte{0x4D, 0x5A}) {
		return true // DOS/Windows executable
	}
	if bytes.HasPrefix(sample, []byte{0xCA, 0xFE, 0xBA, 0xBE}) {
		return true // Mach-O (macOS executable)
	}
	if bytes.HasPrefix(sample, []byte{0x77, 0x4F, 0x46, 0x46}) ||
		bytes.HasPrefix(sample, []byte{0x77, 0x4F, 0x46, 0x32}) {
		return true // WOFF/WOFF2 fonts
	}

	// Heuristic: Check for null bytes and high proportion of non-printable characters
	// Binary files typically have null bytes in first 512 bytes
	nullBytes := 0
	nonPrintable := 0

	for _, b := range sample {
		if b == 0 {
			nullBytes++
		}
		// Count bytes that are not printable ASCII and not common whitespace
		if b < 0x20 && b != 0x09 && b != 0x0A && b != 0x0D {
			nonPrintable++
		}
		// High bytes (>= 0x80) might be UTF-8 or binary
		// Don't count them as non-printable to avoid false positives on UTF-8 text
	}

	// If more than 1% null bytes, very likely binary
	if nullBytes > len(sample)/100 {
		return true
	}

	// If more than 30% non-printable characters (excluding UTF-8), likely binary
	if nonPrintable > len(sample)*30/100 {
		return true
	}

	return false
}

// IsBinary combines extension and magic number checks for robust detection
func (bd *BinaryDetector) IsBinary(path string, content []byte) bool {
	// Fast path: check extension first (no I/O needed)
	if bd.IsBinaryByExtension(path) {
		return true
	}

	// Slow path: check content if extension is ambiguous or unknown
	if len(content) > 0 {
		return bd.IsBinaryByMagicNumber(content)
	}

	return false
}
