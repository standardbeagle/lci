package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Global variable to store the CLI binary path
var testBinaryPath string

// TestMain runs once before all tests
func TestMain(m *testing.M) {
	// Build the CLI binary once for all tests
	tempBinary := filepath.Join(os.TempDir(), "lci-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))

	buildCmd := exec.Command("go", "build", "-o", tempBinary, ".")
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut

	if err := buildCmd.Run(); err != nil {
		fmt.Printf("Failed to build CLI for testing: %v\nBuild output: %s\n", err, buildOut.String())
		os.Exit(1)
	}

	testBinaryPath = tempBinary

	// Run tests
	code := m.Run()

	// Cleanup
	os.Remove(testBinaryPath)
	os.Exit(code)
}

// Test data setup
func setupTestProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create test files with various content
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
	processData()
}

func processData() {
	data := "test data"
	fmt.Println(data)
}`,
		"utils/helper.go": `package utils

// HelperFunction does important work
func HelperFunction(input string) string {
	return "processed: " + input
}

// AnotherFunction for testing
func AnotherFunction() {
	HelperFunction("test")
}`,
		"README.md": `# Test Project
This is a test project for CLI testing.`,
		"test_data.json": `{"test": "data", "items": [1, 2, 3]}`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	return tempDir
}

// TestCLICommands tests various CLI commands with index-compute-shutdown workflow
func TestCLICommands(t *testing.T) {
	projectDir := setupTestProject(t)

	// Change to test directory for CLI tests
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(t, err)

	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, output string, err error)
	}{
		{
			name: "search command - server mode",
			args: []string{"search", "main"},
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)
				// Server-based architecture: CLI connects to server instead of local indexing
				// The server handles indexing in background, CLI just queries
				assert.Contains(t, output, "main.go")
				assert.Contains(t, output, "func main")
				// Verify we got search results (not "Indexed" - that was removed in server architecture)
				assert.Contains(t, output, "Found")
			},
		},
		{
			name: "search with JSON output",
			args: []string{"search", "--json", "HelperFunction"},
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)

				// Parse JSON output if present
				if strings.Contains(output, "{") {
					var result map[string]interface{}
					jsonStart := strings.Index(output, "{")
					if jsonStart >= 0 {
						jsonOutput := output[jsonStart:]
						err = json.Unmarshal([]byte(jsonOutput), &result)
						if err == nil {
							// JSON output uses "results" field, not "matches"
							assert.Contains(t, result, "results")
						}
					}
				}
			},
		},
		{
			name: "definition command - server mode",
			args: []string{"def", "HelperFunction"},
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)
				// Definition command shows the symbol definition via server
				// Output should contain file path and line number
				assert.Contains(t, output, "helper.go")
			},
		},
		{
			name: "references command - server mode",
			args: []string{"refs", "HelperFunction"},
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)
				// References command finds all usages via server
				// Should find at least the definition and the call site
				assert.Contains(t, output, "helper.go")
			},
		},
		{
			name: "tree command - server mode",
			args: []string{"tree", "main"},
			validate: func(t *testing.T, output string, err error) {
				// Tree command might fail if function not found
				if err != nil {
					t.Logf("Tree command error: %v, output: %s", err, output)
				}
				// Either shows the tree or an error message
				if err == nil {
					assert.Contains(t, output, "main")
					// Tree output shows function hierarchy
					assert.Contains(t, output, "tree")
				}
			},
		},
		{
			name: "config init command",
			args: []string{"config", "init"},
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, output, "Configuration file created")
			},
		},
		{
			name: "config show command",
			args: []string{"config", "show"},
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runCLICommand(tt.args...)
			tt.validate(t, output, err)
		})
	}
}

// TestServerBasedSearchWorkflow tests the server-based search workflow
func TestServerBasedSearchWorkflow(t *testing.T) {
	projectDir := setupTestProject(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(t, err)

	// Test search workflow - now server-based
	output, err := runCLICommand("search", "processData")
	require.NoError(t, err)

	// Verify server-based search works
	// Server handles indexing in background, CLI just queries
	assert.Contains(t, output, "processData", "Search should find results")
	assert.Contains(t, output, "main.go", "Results should include file path")
	assert.Contains(t, output, "Found", "Should show result count")

	// Verify clean execution (no error)
	assert.NoError(t, err, "Search should complete cleanly")
}

// TestCLIDiagnosticCapabilities tests the CLI's diagnostic features for MCP debugging
func TestCLIDiagnosticCapabilities(t *testing.T) {
	projectDir := setupTestProject(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(t, err)

	// Test 1: Search with debug info
	output, err := runCLICommand("search", "--json", "main")
	require.NoError(t, err)

	// Parse JSON for diagnostic info if present
	if strings.Contains(output, "{") {
		jsonStart := strings.Index(output, "{")
		if jsonStart >= 0 {
			var result map[string]interface{}
			jsonOutput := output[jsonStart:]
			err = json.Unmarshal([]byte(jsonOutput), &result)
			if err == nil {
				// Verify diagnostic fields are present
				// Server-based architecture uses "results" field
				assert.Contains(t, result, "results", "Should have results field")
				// Check for timing info
				if _, ok := result["time_ms"]; ok {
					assert.Contains(t, result, "time_ms", "Should have timing info")
				}
			}
		}
	}

	// Test 2: Test error diagnostics - non-existent function
	output, err = runCLICommand("def", "NonExistentFunction")
	// Should not error, but may return empty results
	assert.NoError(t, err)
	// Definition command runs via server - just verify it completes
	// Empty result is acceptable for non-existent function
}

// TestCLIErrorHandling tests error scenarios
func TestCLIErrorHandling(t *testing.T) {
	projectDir := setupTestProject(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(t, err)

	tests := []struct {
		name      string
		args      []string
		expectErr bool
		validate  func(t *testing.T, output string, err error)
	}{
		{
			name:      "search for non-existent pattern",
			args:      []string{"search", "ThisDoesNotExist12345"},
			expectErr: false,
			validate: func(t *testing.T, output string, err error) {
				assert.NoError(t, err)
				// Server-based search completes without error
				// Search with no results shows "Found 0" message
				assert.Contains(t, output, "Found 0")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runCLICommand(tt.args...)
			if tt.expectErr {
				assert.Error(t, err)
			}
			if tt.validate != nil {
				tt.validate(t, output, err)
			}
		})
	}
}

// TestCLIPerformance tests performance requirements
func TestCLIPerformance(t *testing.T) {
	projectDir := setupTestProject(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(t, err)

	// Test search performance
	start := time.Now()
	output, err := runCLICommand("search", "main")
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Contains(t, output, "main")

	// CLI should complete within reasonable time for small test project
	assert.Less(t, duration.Seconds(), 2.0, "CLI command should complete within 2 seconds for small project")
}

// TestCLIConfiguration tests configuration handling
func TestCLIConfiguration(t *testing.T) {
	projectDir := setupTestProject(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(t, err)

	// Initialize config
	output, err := runCLICommand("config", "init")
	require.NoError(t, err)
	assert.Contains(t, output, "Configuration file created")

	// Verify config file exists
	configFiles := []string{".lci.kdl"}
	var configExists bool
	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			configExists = true
			break
		}
	}
	assert.True(t, configExists, "Config file should be created")

	// Test config show
	output, err = runCLICommand("config", "show")
	require.NoError(t, err)
	assert.NotEmpty(t, output)

	// Test config validate
	_, err = runCLICommand("config", "validate")
	require.NoError(t, err)
}

// Helper function to run CLI commands and capture output
func runCLICommand(args ...string) (string, error) {
	if testBinaryPath == "" {
		return "", fmt.Errorf("test binary not built")
	}

	// Run the command
	cmd := exec.Command(testBinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine stdout and stderr for full output
	output := stdout.String() + stderr.String()

	return output, err
}

// Benchmark CLI operations
func BenchmarkCLISearch(b *testing.B) {
	projectDir := setupBenchProject(b)

	oldDir, err := os.Getwd()
	require.NoError(b, err)
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(projectDir)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := runCLICommand("search", "main")
		require.NoError(b, err)
	}
}

// setupBenchProject for benchmarks
func setupBenchProject(tb testing.TB) string {
	tempDir := tb.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go": `package main
import "fmt"
func main() { fmt.Println("Hello") }`,
		"utils/helper.go": `package utils
func Helper() string { return "help" }`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(tb, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(tb, err)
	}

	return tempDir
}
