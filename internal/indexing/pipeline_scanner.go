// Context: FileScanner helper methods (filtering, path matching, priority) extracted from pipeline.go.
// External deps: standard library only; operates on FileScanner and config types.
// Prompt-log: See root prompt-log.md for session details (2025-09-05).
package indexing

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// shouldProcessFile determines if a file should be indexed (simplified - main filtering done earlier)
func (fs *FileScanner) shouldProcessFile(path string, info os.FileInfo) bool {
	// Skip directories (should already be filtered out)
	if info.IsDir() {
		return false
	}

	// Fast binary detection by extension (no I/O needed)
	if fs.binaryDetector != nil && fs.binaryDetector.IsBinaryByExtension(path) {
		return false
	}

	// Check gitignore if enabled (more expensive check done after filename filtering)
	if fs.gitignoreParser != nil {
		// Convert absolute path to relative path from project root for gitignore matching
		relativePath, err := filepath.Rel(fs.config.Project.Root, path)
		if err != nil {
			// If we can't get a relative path, log the error and use the original path
			// This is rare but can happen with unusual path configurations
			relativePath = path
		} else {
			// Use forward slashes for gitignore consistency
			relativePath = filepath.ToSlash(relativePath)
		}

		if fs.gitignoreParser.ShouldIgnore(relativePath, info.IsDir()) {
			return false
		}
	}

	// Check file size limits (done after filename filtering to avoid stat calls)
	if info.Size() > int64(fs.config.Index.MaxFileSize) {
		return false
	}

	// Binary pre-check: For files above the threshold, read first bytes to detect binary content
	// This prevents loading large binary files into memory
	if fs.binaryDetector != nil && info.Size() > types.BinaryPreCheckSizeThreshold {
		if isBinary := fs.preCheckBinaryFile(path); isBinary {
			return false
		}
	}

	// If we get here, the file passed all filename filters and should be processed
	return true
}

// preCheckBinaryFile reads the first bytes of a file to detect binary content
// without loading the entire file into memory
func (fs *FileScanner) preCheckBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		// If we can't open the file, skip it
		return true
	}
	defer file.Close()

	// Read only the first bytes for magic number detection
	buffer := make([]byte, types.BinaryPreCheckBytes)
	n, err := io.ReadFull(file, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// If we can't read the file, skip it as potentially binary
		return true
	}

	// Check the bytes we read for binary signatures
	return fs.binaryDetector.IsBinaryByMagicNumber(buffer[:n])
}

// matchesGlobPattern performs glob-style pattern matching with ** support
func (fs *FileScanner) matchesGlobPattern(pattern, path string) bool {
	// Handle ** patterns (match any number of directories)
	if strings.Contains(pattern, "**") {
		return fs.matchDoubleGlob(pattern, path)
	}

	// Use Go's filepath.Match for simple patterns
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	// Handle directory patterns by checking path components
	if strings.Contains(pattern, "/") {
		return fs.matchPathPattern(pattern, path)
	}

	// For simple patterns, check if they match any component
	pathParts := strings.Split(path, "/")
	for _, part := range pathParts {
		if matched, _ := filepath.Match(pattern, part); matched {
			return true
		}
	}

	return false
}

// matchDoubleGlob handles ** patterns
func (fs *FileScanner) matchDoubleGlob(pattern, path string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	// Special case: patterns like **/.*/** (multiple **) mean "match pattern at any depth"
	// This is common for excluding hidden directories
	if len(parts) >= 3 {
		// Extract the middle pattern between ** markers
		// For "**/.*/**", parts = ["", "/.*/"," "]
		// We want to check if any path component matches the middle pattern
		for i := 1; i < len(parts)-1; i++ {
			middle := strings.Trim(parts[i], "/")
			if middle == "" {
				continue
			}

			// Check if any component of the path matches this pattern
			pathParts := strings.Split(path, "/")
			for _, part := range pathParts {
				if matched, _ := filepath.Match(middle, part); matched {
					return true
				}
			}
		}
		return false
	}

	if len(parts) != 2 {
		// Single ** or no **, fall back to simple match
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	prefix := parts[0]
	suffix := parts[1]

	// Remove trailing/leading slashes from prefix/suffix
	prefix = strings.TrimSuffix(prefix, "/")
	suffix = strings.TrimPrefix(suffix, "/")

	// Path must start with prefix (if any)
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	// Handle suffix matching
	if suffix != "" {
		// Special handling for patterns like **/.*/** which should match hidden directories
		// The suffix between ** markers should match ANY directory component in the path
		remainingPath := path
		if prefix != "" {
			remainingPath = strings.TrimPrefix(path, prefix)
			remainingPath = strings.TrimPrefix(remainingPath, "/")
		}

		// Split the suffix to handle patterns like .*/something
		suffixTrimmed := strings.TrimSuffix(suffix, "/")

		// Check if any path component matches the suffix pattern
		pathParts := strings.Split(remainingPath, "/")
		for _, part := range pathParts {
			// Match the suffix pattern against this component
			if matched, _ := filepath.Match(suffixTrimmed, part); matched {
				return true
			}
		}

		// Also check full path matches
		if matched, _ := filepath.Match(suffix, remainingPath); matched {
			return true
		}

		// Check if the entire remaining path ends with the suffix pattern
		if strings.HasSuffix(remainingPath, suffix) {
			return true
		}

		return false
	}

	return true
}

// matchPathPattern handles patterns with directory separators
func (fs *FileScanner) matchPathPattern(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	// Try to match pattern parts against path parts
	return fs.matchParts(patternParts, pathParts)
}

// matchParts matches pattern parts against path parts
func (fs *FileScanner) matchParts(patternParts, pathParts []string) bool {
	if len(patternParts) > len(pathParts) {
		return false
	}

	// Try matching from the end (suffix match)
	pOffset := len(pathParts) - len(patternParts)
	for i, patternPart := range patternParts {
		pathPart := pathParts[pOffset+i]
		if matched, _ := filepath.Match(patternPart, pathPart); !matched {
			return false
		}
	}

	return true
}

// getFilePriority assigns priority to files (higher = more important)
func (fs *FileScanner) getFilePriority(path string) int {
	ext := filepath.Ext(path)

	// Higher priority for common source files
	switch ext {
	case ".go", ".rs", ".py", ".js", ".ts":
		return 10
	case ".java", ".cpp", ".c", ".h":
		return 8
	case ".md", ".txt", ".yaml", ".yml", ".json":
		return 5
	default:
		return 1
	}
}
