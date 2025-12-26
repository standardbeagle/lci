package indexing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/standardbeagle/lci/internal/core"
)

// ProjectInitializer handles project root detection and initialization logic
// This is shared between CLI and MCP to avoid code duplication
type ProjectInitializer struct {
	fileService *core.FileService
}

// NewProjectInitializer creates a new project initializer with the default FileService
func NewProjectInitializer() *ProjectInitializer {
	return &ProjectInitializer{
		fileService: core.NewFileService(),
	}
}

// NewProjectInitializerWithFileService creates a project initializer with a custom FileService
// This is primarily used for testing with mock filesystems
func NewProjectInitializerWithFileService(fs *core.FileService) *ProjectInitializer {
	return &ProjectInitializer{
		fileService: fs,
	}
}

// DetectProjectRoot checks if a path is likely a project root
// Returns (isProjectRoot, detectionMarker)
func (pi *ProjectInitializer) DetectProjectRoot(path string) (bool, string) {
	if path == "" {
		return false, ""
	}

	// Validate path is a directory
	if !pi.fileService.IsDir(path) {
		return false, ""
	}

	// LCI config markers - highest priority (explicit user configuration)
	// These take precedence over all other markers to ensure the index
	// respects user-defined project boundaries and exclusion patterns
	for _, marker := range LCIConfigMarkers {
		markerPath := filepath.Join(path, marker)
		if pi.fileService.Exists(markerPath) {
			return true, marker
		}
	}

	// Check primary markers
	for _, marker := range PrimaryProjectMarkers {
		markerPath := filepath.Join(path, marker)
		if pi.fileService.Exists(markerPath) {
			return true, marker
		}
	}

	// Check secondary markers
	for _, marker := range SecondaryProjectMarkers {
		markerPath := filepath.Join(path, marker)
		if pi.fileService.Exists(markerPath) {
			return true, marker
		}
	}

	// Tertiary fallback - check for source code directories
	sourceDirCount := 0
	for _, dir := range SourceDirectoryNames {
		dirPath := filepath.Join(path, dir)
		if pi.fileService.IsDir(dirPath) {
			if pi.hasSourceFiles(dirPath) {
				sourceDirCount++
			}
		}
	}

	// If we have multiple source directories, consider it a project root
	if sourceDirCount >= SourceDirectoryThreshold {
		return true, "source_structure"
	}

	// Final fallback - check for any source files in current directory
	if pi.hasSourceFiles(path) {
		return true, "source_files"
	}

	return false, ""
}

// hasSourceFiles checks if a directory contains source code files
func (pi *ProjectInitializer) hasSourceFiles(dirPath string) bool {
	if dirPath == "" {
		return false
	}

	files, err := pi.fileService.ListFiles(dirPath)
	if err != nil {
		return false
	}

	// Check for source files using centralized extension map
	for _, file := range files {
		ext := filepath.Ext(file)
		if SourceFileExtensions[ext] {
			return true
		}
	}

	return false
}

// FindProjectRoot walks up the directory tree from startPath to find the project root.
//
// Priority order:
// 1. LCI config files (.lci.kdl, .lciconfig) - searched all the way up first
// 2. Other project markers (.git, go.mod, etc.) - only if no LCI config found
//
// This ensures that if a parent directory has an LCI config with exclusion
// patterns, those patterns are respected even when starting from a subdirectory
// that might have its own .git or other markers.
func (pi *ProjectInitializer) FindProjectRoot(startPath string) (string, string, error) {
	if startPath == "" {
		return "", "", errors.New("startPath cannot be empty")
	}

	// First pass: Look for LCI config files all the way up the tree
	// This ensures parent LCI configs take precedence over child .git directories
	currentPath := startPath
	for {
		for _, marker := range LCIConfigMarkers {
			markerPath := filepath.Join(currentPath, marker)
			if pi.fileService.Exists(markerPath) {
				return currentPath, marker, nil
			}
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached root directory
			break
		}
		currentPath = parentPath
	}

	// Second pass: No LCI config found, look for other project markers
	// Start from the original path and stop at first marker found
	if isRoot, marker := pi.DetectProjectRoot(startPath); isRoot {
		return startPath, marker, nil
	}

	currentPath = startPath
	for {
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached root directory
			break
		}

		if isRoot, marker := pi.DetectProjectRoot(parentPath); isRoot {
			return parentPath, marker, nil
		}

		currentPath = parentPath
	}

	// No project root found
	return "", "", fmt.Errorf("no project root detected from path: %s", startPath)
}

// GetProjectRoot detects the project root starting from a given path
// and walking up the directory tree until a project marker is found.
// This is a convenience function that uses the default FileService.
// For testing, use NewProjectInitializerWithFileService and call FindProjectRoot.
func GetProjectRoot(startPath string) (string, string, error) {
	if startPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		startPath = cwd
	}

	initializer := NewProjectInitializer()
	return initializer.FindProjectRoot(startPath)
}
