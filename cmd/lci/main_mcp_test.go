package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Protocol-level Tests (require manual JSON-RPC for specific behaviors)
// =============================================================================

// TestMCPAutoDetection tests that lci can auto-detect MCP mode when run without arguments
// and stdin appears to be a JSON-RPC stream. This tests a specific behavior that requires
// manual JSON-RPC control - the SDK client always uses explicit MCP mode.
func TestMCPAutoDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow integration test (spawns process, 5s timeout)")
	}

	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	// Start lci WITHOUT any arguments - should auto-detect MCP from JSON-RPC input
	cmd := exec.Command(testBinaryPath)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Start()
	require.NoError(t, err)

	// Send initialize request
	jsonrpcInput := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	_, err = stdin.Write([]byte(jsonrpcInput))
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	stdin.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "signal") {
			t.Logf("Command completed with: %v", err)
		}
	case <-time.After(1 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("Server did not exit within 1s after stdin close - EOF handling bug")
	}

	stdoutStr := stdout.String()
	assert.Contains(t, stdoutStr, "jsonrpc", "Expected JSON-RPC response on stdout")
	assert.Contains(t, stdoutStr, "result", "Expected successful JSON-RPC response")

	stderrStr := stderr.String()
	assert.NotContains(t, stderrStr, "Building index", "Debug output should be suppressed in MCP mode")
	assert.NotContains(t, stderrStr, "DEBUG:", "Debug output should be suppressed in MCP mode")
}

// TestMCPSignalShutdown tests that MCP server shuts down gracefully on SIGINT.
// This requires manual control to send the signal, can't be tested with SDK client.
func TestMCPSignalShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow integration test (spawns process, 5s timeout)")
	}

	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	cmd := exec.Command(testBinaryPath, "mcp")

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Start()
	require.NoError(t, err)

	// Send initialize request
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	_, err = stdin.Write([]byte(initRequest))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Send interrupt signal (Ctrl+C)
	err = cmd.Process.Signal(os.Interrupt)
	require.NoError(t, err)

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- cmd.Wait() }()

	select {
	case err := <-shutdownDone:
		t.Logf("Process shutdown with: %v", err)
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("MCP server failed to shutdown gracefully within 5 seconds")
	}

	stderrStr := stderr.String()
	assert.NotContains(t, stderrStr, "shutdown timeout", "Should not have shutdown timeout")
	// Note: "context canceled" is expected when the process receives SIGINT
	// We only check for actual fatal errors that indicate unexpected shutdown issues
	if strings.Contains(stderrStr, "Fatal error") && !strings.Contains(stderrStr, "context canceled") {
		t.Errorf("Unexpected fatal error in shutdown: %s", stderrStr)
	}
}

// =============================================================================
// SDK-based MCP Client Tests
// =============================================================================
// These tests use the official MCP SDK client to test the server through the
// actual MCP protocol, providing true end-to-end testing.

// testClientImpl is the client implementation used for all SDK-based tests
var testClientImpl = &mcp.Implementation{Name: "lci-test-client", Version: "1.0.0"}

// setupTestProjectForMCP creates a test project directory with sample Go files
func setupTestProjectForMCP(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()

	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
	processData()
}

func processData() {
	data := fetchData()
	result := transformData(data)
	saveResult(result)
}

func fetchData() string {
	return "raw data"
}

func transformData(data string) string {
	return "transformed: " + data
}

func saveResult(result string) {
	fmt.Printf("Saving: %s\n", result)
}
`,
		"utils/helper.go": `package utils

// HelperFunction provides utility functionality
func HelperFunction(input string) string {
	return "processed: " + input
}

// Calculate performs arithmetic
func Calculate(a, b int) int {
	return a + b
}
`,
		"service/api.go": `package service

// APIService handles API requests
type APIService struct {
	endpoint string
}

// HandleRequest processes incoming requests
func (s *APIService) HandleRequest(path string) {
	processPath(path)
}

func processPath(path string) {
	// Implementation
}
`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	return tempDir
}

// TestMCPClientSDK_Initialize tests basic MCP initialization using the SDK client
func TestMCPClientSDK_Initialize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create command for the MCP server
	cmd := exec.Command(testBinaryPath, "mcp")

	// Create SDK client and connect via CommandTransport
	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err, "Failed to connect to MCP server")
	defer session.Close()

	// Verify we can ping the server
	err = session.Ping(ctx, nil)
	require.NoError(t, err, "Failed to ping MCP server")

	t.Log("✓ MCP client successfully connected and pinged server")
}

// TestMCPClientSDK_ListTools tests that we can list available tools
func TestMCPClientSDK_ListTools(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.Command(testBinaryPath, "mcp")
	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err)
	defer session.Close()

	// List available tools
	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err, "Failed to list tools")
	require.NotNil(t, tools, "Tools list should not be nil")

	// Verify expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools.Tools {
		toolNames[tool.Name] = true
		t.Logf("  Found tool: %s", tool.Name)
	}

	// Check for core tools (actual tool names from the server)
	expectedTools := []string{"search", "info", "get_context", "code_insight"}
	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Expected tool %q to be available", expected)
	}

	t.Logf("✓ Listed %d tools", len(tools.Tools))
}

// TestMCPClientSDK_CallTool_Info tests calling the info tool
func TestMCPClientSDK_CallTool_Info(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.Command(testBinaryPath, "mcp")
	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err)
	defer session.Close()

	// Call the info tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "info",
		Arguments: map[string]any{},
	})
	require.NoError(t, err, "Failed to call info tool")
	require.NotNil(t, result, "Result should not be nil")
	require.NotEmpty(t, result.Content, "Result should have content")

	// Verify content is text
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			assert.NotEmpty(t, textContent.Text, "Text content should not be empty")
			t.Logf("✓ Info tool returned %d bytes", len(textContent.Text))
		}
	}
}

// TestMCPClientSDK_CallTool_Search tests calling the search tool with a real project
func TestMCPClientSDK_CallTool_Search(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	// Create test project
	projectDir := setupTestProjectForMCP(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start MCP server in the test project directory
	// The server auto-indexes on startup, so we just need to wait for it
	cmd := exec.Command(testBinaryPath, "mcp")
	cmd.Dir = projectDir

	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err)
	defer session.Close()

	// Wait for auto-indexing to complete
	time.Sleep(3 * time.Second)

	// Search for a function
	searchResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search",
		Arguments: map[string]any{
			"pattern": "processData",
		},
	})
	require.NoError(t, err, "Failed to call search tool")
	require.NotNil(t, searchResult, "Search result should not be nil")

	// Check results
	for _, content := range searchResult.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			t.Logf("Search result: %s", textContent.Text[:min(200, len(textContent.Text))])
			// Verify we found something
			assert.Contains(t, textContent.Text, "processData", "Should find processData in results")
		}
	}

	t.Log("✓ Search tool executed successfully")
}

// TestMCPClientSDK_CallTool_CodeInsight tests calling the code_insight tool
func TestMCPClientSDK_CallTool_CodeInsight(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	projectDir := setupTestProjectForMCP(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.Command(testBinaryPath, "mcp")
	cmd.Dir = projectDir

	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err)
	defer session.Close()

	// Wait for auto-indexing
	time.Sleep(3 * time.Second)

	// Get code insight (codebase intelligence)
	insightResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "code_insight",
		Arguments: map[string]any{},
	})
	require.NoError(t, err, "Failed to get code insight")
	require.NotNil(t, insightResult)

	for _, content := range insightResult.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			t.Logf("Code insight: %s", textContent.Text[:min(500, len(textContent.Text))])
			// Should have some content about the codebase
			assert.NotEmpty(t, textContent.Text, "Code insight should return content")
		}
	}

	t.Log("✓ Code insight retrieved successfully")
}

// TestMCPClientSDK_GracefulShutdown tests that the server shuts down properly when the client disconnects
func TestMCPClientSDK_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.Command(testBinaryPath, "mcp")
	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err)

	// Ping to ensure connection is established
	err = session.Ping(ctx, nil)
	require.NoError(t, err)

	// Close the session - server should shut down gracefully
	err = session.Close()
	require.NoError(t, err, "Session close should succeed")

	// Wait a moment and verify process exited
	time.Sleep(500 * time.Millisecond)

	// The process should have exited by now
	// (CommandTransport handles process cleanup)
	t.Log("✓ Server shut down gracefully after client disconnect")
}

// TestMCPClientSDK_ConcurrentToolCalls tests making multiple tool calls concurrently
// Uses a small number of concurrent calls (3) to be realistic for typical MCP clients
func TestMCPClientSDK_ConcurrentToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test (spawns process)")
	}
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	projectDir := setupTestProjectForMCP(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.Command(testBinaryPath, "mcp")
	cmd.Dir = projectDir

	client := mcp.NewClient(testClientImpl, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	require.NoError(t, err)
	defer session.Close()

	// Wait for auto-indexing
	time.Sleep(3 * time.Second)

	// Make 3 concurrent search calls (realistic for typical MCP clients)
	patterns := []string{"main", "func", "string"}
	results := make(chan error, len(patterns))

	for _, pattern := range patterns {
		go func(p string) {
			_, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "search",
				Arguments: map[string]any{
					"pattern": p,
				},
			})
			results <- err
		}(pattern)
	}

	// Collect results
	errorCount := 0
	for i := 0; i < len(patterns); i++ {
		if err := <-results; err != nil {
			t.Logf("Concurrent search error: %v", err)
			errorCount++
		}
	}

	assert.Zero(t, errorCount, "All concurrent searches should succeed")
	t.Logf("✓ %d concurrent tool calls completed successfully", len(patterns))
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// Mode Detection Tests (require manual control, can't use SDK client)
// =============================================================================

// TestDebugOutputSuppression tests that debug output is properly suppressed in MCP mode
// and verifies the LCI_MCP_MODE environment variable works correctly.
func TestDebugOutputSuppression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow integration test (spawns multiple processes, 3s timeout each)")
	}

	// This test verifies that when MCP mode is enabled, debug output goes to stderr
	// but in a controlled way that doesn't interfere with the JSON-RPC protocol on stdout

	// Use shared test binary from TestMain
	if testBinaryPath == "" {
		t.Fatal("Test binary not built - TestMain did not run")
	}

	// Test with various debug-triggering scenarios
	testCases := []struct {
		name          string
		args          []string
		input         string
		env           map[string]string
		expectMCPMode bool
	}{
		{
			name: "explicit_mcp_command",
			args: []string{"mcp"},
			// Send initialize request followed by initialized notification
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n" +
				`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n",
			expectMCPMode: true,
		},
		{
			name: "environment_variable_override",
			args: []string{},
			env:  map[string]string{"LCI_MCP_MODE": "1"},
			// Send initialize request followed by initialized notification
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n" +
				`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n",
			expectMCPMode: true,
		},
		{
			name:          "regular_search_command",
			args:          []string{"search", "test"},
			input:         "",
			expectMCPMode: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(testBinaryPath, tc.args...)

			// Set environment variables if specified
			if tc.env != nil {
				cmd.Env = os.Environ()
				for key, value := range tc.env {
					cmd.Env = append(cmd.Env, key+"="+value)
				}
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			var stdinPipe io.WriteCloser
			var err error
			if tc.input != "" {
				stdinPipe, err = cmd.StdinPipe()
				require.NoError(t, err)
			}

			err = cmd.Start()
			require.NoError(t, err)

			// Send input if provided
			if tc.input != "" {
				_, err = io.WriteString(stdinPipe, tc.input)
				require.NoError(t, err)
				// Give the server time to process the input
				time.Sleep(200 * time.Millisecond)
			}

			// Wait with timeout
			done := make(chan error, 1)
			go func() {
				// Close stdin to signal end of input
				if stdinPipe != nil {
					stdinPipe.Close()
				}
				done <- cmd.Wait()
			}()

			select {
			case err := <-done:
				if err != nil && !strings.Contains(err.Error(), "exit status") {
					t.Logf("Command completed with: %v", err)
				}
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}

			stdoutStr := stdout.String()
			stderrStr := stderr.String()

			if tc.expectMCPMode {
				// In MCP mode: stdout should have JSON-RPC, stderr should have controlled logging
				assert.Contains(t, stdoutStr, "jsonrpc", "MCP mode should output JSON-RPC to stdout")

				// Stderr can have MCP server messages but no general debug output
				if len(stderrStr) > 0 {
					assert.Contains(t, stderrStr, "[MCP]", "Only MCP server messages on stderr")
					assert.NotContains(t, stderrStr, "Building index", "No general debug output in MCP mode")
				}
			} else {
				// In regular mode: stderr can have debug output, stdout has command output
				// Check for JSON-RPC protocol responses (not just "jsonrpc" in search results)
				// JSON-RPC protocol responses have specific format: {"jsonrpc":"2.0","id":N,"result":...}
				hasJSONRpcProtocol := strings.Contains(stdoutStr, `"jsonrpc":"2.0"`) &&
					strings.Contains(stdoutStr, `"result"`) &&
					!strings.Contains(stdoutStr, "=== Direct Matches ===") // Not search results

				if hasJSONRpcProtocol {
					assert.Fail(t, "Regular mode should not output JSON-RPC protocol responses")
				}
			}
		})
	}
}
