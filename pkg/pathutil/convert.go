// Package pathutil provides utilities for converting between absolute and relative paths.
//
// Architecture Pattern:
// Lightning Code Index uses absolute paths internally for consistency and to avoid ambiguity.
// However, user-facing output should use relative paths for readability and portability.
// This package provides the conversion layer between internal (absolute) and external (relative) representations.
package pathutil

import (
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/searchtypes"
)

// ToRelative converts an absolute path to relative based on a root directory.
// Falls back to the original path if conversion fails or path is already relative.
//
// Examples:
//   - ToRelative("/home/user/project/src/main.go", "/home/user/project") → "src/main.go"
//   - ToRelative("/other/location/file.go", "/home/user/project") → "/other/location/file.go" (outside root)
//   - ToRelative("src/main.go", "/home/user/project") → "src/main.go" (already relative)
func ToRelative(absPath, rootDir string) string {
	// Handle empty inputs
	if absPath == "" || rootDir == "" {
		return absPath
	}

	// If path is already relative, return as-is
	if !filepath.IsAbs(absPath) {
		return absPath
	}

	// Clean both paths to normalize separators and remove redundant elements
	absPath = filepath.Clean(absPath)
	rootDir = filepath.Clean(rootDir)

	// Try to make relative
	relPath, err := filepath.Rel(rootDir, absPath)
	if err != nil {
		// Conversion failed (e.g., different drives on Windows) - return absolute
		return absPath
	}

	// If the relative path starts with ".." it means the file is outside the root
	// In this case, return the absolute path as it's clearer
	if strings.HasPrefix(relPath, "..") {
		return absPath
	}

	return relPath
}

// ToRelativeGrepResults converts paths in GrepResult slice from absolute to relative.
// Creates a new slice without modifying the original results.
//
// This function is designed for use at output boundaries where results are displayed to users:
//   - CLI grep output
//   - JSON serialization
//   - Test result parsing
func ToRelativeGrepResults(results []searchtypes.GrepResult, rootDir string) []searchtypes.GrepResult {
	if len(results) == 0 {
		return results
	}

	// Create a copy to avoid modifying the original
	converted := make([]searchtypes.GrepResult, len(results))
	copy(converted, results)

	// Convert paths
	for i := range converted {
		converted[i].Path = ToRelative(converted[i].Path, rootDir)
	}

	return converted
}

// ToRelativeStandardResults converts paths in StandardResult slice from absolute to relative.
// Creates a new slice without modifying the original results.
//
// This function is designed for use at output boundaries where results are displayed to users:
//   - CLI standard search output
//   - JSON serialization
//   - MCP server responses
func ToRelativeStandardResults(results []searchtypes.StandardResult, rootDir string) []searchtypes.StandardResult {
	if len(results) == 0 {
		return results
	}

	// Create a copy to avoid modifying the original
	converted := make([]searchtypes.StandardResult, len(results))
	copy(converted, results)

	// Convert paths in the nested Result field
	for i := range converted {
		converted[i].Result.Path = ToRelative(converted[i].Result.Path, rootDir)
	}

	return converted
}
