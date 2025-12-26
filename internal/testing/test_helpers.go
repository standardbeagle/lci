package testing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Common test patterns and helper functions for LCI testing

// CommonGitignorePatterns returns commonly used gitignore patterns for testing
func CommonGitignorePatterns() []string {
	return []string{
		"node_modules/",
		"*.log",
		"dist/",
		"build/",
		"coverage/",
		".env*",
		"!important.log",
		"!unit.test.js",
	}
}

// NodeProjectFiles returns a map of common Node.js project files
func NodeProjectFiles() map[string]string {
	return map[string]string{
		"src/app.js": `const express = require('express');
const app = express();

function startServer(port) {
    return app.listen(port, () => {
        console.log('Server running on port', port);
    });
}

module.exports = { startServer };`,

		"src/utils.js": `function formatDate(date) {
    return date.toISOString().split('T')[0];
}

function validateEmail(email) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

module.exports = { formatDate, validateEmail };`,

		"package.json": `{
  "name": "test-project",
  "version": "1.0.0",
  "main": "src/app.js",
  "scripts": {
    "start": "node src/app.js",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.0"
  }
}`,

		"tests/app.test.js": `const { startServer } = require('../src/app');

describe('Server', () => {
    test('should start server', () => {
        const server = startServer(3000);
        expect(server).toBeDefined();
    });
});`,
	}
}

// GoProjectFiles returns a map of common Go project files
func GoProjectFiles() map[string]string {
	return map[string]string{
		"main.go": `package main

import (
    "fmt"
    "test-project/internal/config"
    "test-project/pkg/utils"
)

func main() {
    cfg := config.Load()
    result := utils.Process(cfg.Data)
    fmt.Println(result)
}`,

		"internal/config/config.go": `package config

import "encoding/json"

type Config struct {
    Data map[string]interface{}
}

func Load() *Config {
    return &Config{
        Data: map[string]interface{}{
            "timeout": 30,
            "debug":   true,
        },
    }
}`,

		"pkg/utils/utils.go": `package utils

import "test-project/internal/config"

func Process(data map[string]interface{}) string {
    return "processed: " + data["debug"].(string)
}`,

		"go.mod": `module test-project

go 1.21

require (
    // dependencies would go here
)`,

		"utils_test.go": `package utils

import (
    "testing"
    "test-project/internal/config"
)

func TestProcess(t *testing.T) {
    cfg := &config.Config{
        Data: map[string]interface{}{"test": "value"},
    }
    result := Process(cfg.Data)
    if result != "processed: <nil>" {
        t.Errorf("Unexpected result: %s", result)
    }
}`,
	}
}

// CreateTestFile creates a test file with the given content
func CreateTestFile(basePath, filename, content string) error {
	fullPath := filepath.Join(basePath, filename)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// CreateTestDirectory creates a test directory
func CreateTestDirectory(basePath, dirPath string) error {
	fullPath := filepath.Join(basePath, dirPath)
	return os.MkdirAll(fullPath, 0755)
}

// AssertFileExists asserts that a file exists
func AssertFileExists(t *testing.T, filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Expected file %s to exist", filePath)
	}
}

// AssertFileNotExists asserts that a file does not exist
func AssertFileNotExists(t *testing.T, filePath string) {
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("Expected file %s to not exist", filePath)
	}
}

// CreateTempDir creates a temporary directory for testing
func CreateTempDir(t *testing.T) string {
	tempDir := t.TempDir()
	require.NotEmpty(t, tempDir, "TempDir should not be empty")
	return tempDir
}

// TestFile represents a test file with path and content
type TestFile struct {
	Path    string
	Content string
	IsDir   bool
}

// TestFiles represents a collection of test files
type TestFiles []TestFile

// AddFile adds a file to the test collection
func (tf *TestFiles) AddFile(path, content string) {
	*tf = append(*tf, TestFile{Path: path, Content: content, IsDir: false})
}

// AddDirectory adds a directory to the test collection
func (tf *TestFiles) AddDirectory(path string) {
	*tf = append(*tf, TestFile{Path: path, Content: "", IsDir: true})
}

// CreateFiles creates all files in the test collection
func (tf TestFiles) CreateFiles(basePath string) error {
	for _, file := range tf {
		if file.IsDir {
			if err := CreateTestDirectory(basePath, file.Path); err != nil {
				return err
			}
		} else {
			if err := CreateTestFile(basePath, file.Path, file.Content); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetupProjectWithGitignore creates a test project with gitignore patterns
func SetupProjectWithGitignore(t *testing.T, files map[string]string, gitignorePatterns []string) string {
	tempDir := CreateTempDir(t)

	// Create .gitignore file
	if len(gitignorePatterns) > 0 {
		gitignorePath := filepath.Join(tempDir, ".gitignore")
		gitignoreContent := ""
		for _, pattern := range gitignorePatterns {
			gitignoreContent += pattern + "\n"
		}
		require.NoError(t, os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644))
	}

	// Create project files
	for path, content := range files {
		_ = CreateTestFile(tempDir, path, content)
	}

	return tempDir
}
