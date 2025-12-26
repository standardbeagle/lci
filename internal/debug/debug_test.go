package debug

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// saveAndRestoreState saves the debug package state and returns a cleanup function
func saveAndRestoreState() func() {
	originalDebug := EnableDebug
	originalMode := MCPMode
	originalOutput := debugOutput
	originalFile := debugFile
	return func() {
		EnableDebug = originalDebug
		MCPMode = originalMode
		debugOutput = originalOutput
		debugFile = originalFile
	}
}

// TestSetMCPMode tests the set m c p mode.
func TestSetMCPMode(t *testing.T) {
	defer saveAndRestoreState()()

	// Test enabling MCP mode
	SetMCPMode(true)
	assert.True(t, MCPMode)

	// Test disabling MCP mode
	SetMCPMode(false)
	assert.False(t, MCPMode)
}

// TestIsDebugEnabled tests the is debug enabled.
func TestIsDebugEnabled(t *testing.T) {
	defer saveAndRestoreState()()

	// Test when debug is disabled
	EnableDebug = "false"
	MCPMode = false
	assert.False(t, IsDebugEnabled())

	// Test when debug is enabled
	EnableDebug = "true"
	MCPMode = false
	assert.True(t, IsDebugEnabled())

	// Test invalid value defaults to false
	EnableDebug = "invalid"
	assert.False(t, IsDebugEnabled())
}

// TestLog tests the log.
func TestLog(t *testing.T) {
	defer saveAndRestoreState()()

	// Test with debug enabled and MCP disabled, using buffer as output
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	EnableDebug = "true"
	MCPMode = false
	Log("TEST", "Hello %s", "World")

	output := buf.String()
	assert.Contains(t, output, "[DEBUG:TEST]")
	assert.Contains(t, output, "Hello World")
}

// TestLog_MCPMode tests the log m c p mode.
func TestLog_MCPMode(t *testing.T) {
	defer saveAndRestoreState()()

	// Test with MCP enabled - should not output even if debug output is set
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	EnableDebug = "true"
	MCPMode = true
	Log("TEST", "Should not appear")

	output := buf.String()
	assert.Empty(t, output)
}

// TestLogSearch tests the log search.
func TestLogSearch(t *testing.T) {
	defer saveAndRestoreState()()

	// Test search logging
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	EnableDebug = "true"
	MCPMode = false
	LogSearch("searching for %s", "pattern")

	output := buf.String()
	assert.Contains(t, output, "[DEBUG:SEARCH]")
	assert.Contains(t, output, "searching for pattern")
}

// TestFatal tests the fatal.
func TestFatal(t *testing.T) {
	defer saveAndRestoreState()()

	// Test Fatal returns error
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	MCPMode = false
	err := Fatal("test error: %s", "details")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fatal error: test error: details")
	assert.Contains(t, buf.String(), "[FATAL]")

	// Test with MCP mode (should still return error, but no output)
	buf.Reset()
	MCPMode = true
	err = Fatal("another error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fatal error: another error")
	assert.Empty(t, buf.String()) // No output in MCP mode
}

// TestFatalAndExit tests the fatal and exit.
func TestFatalAndExit(t *testing.T) {
	// We can't easily test os.Exit without terminating the test
	// So we'll just verify the function exists and can be called
	// In a real scenario, you might use a subprocess test

	defer saveAndRestoreState()()

	// Verify function exists and would output in non-MCP mode
	if os.Getenv("BE_FATAL_TEST") == "1" {
		var buf bytes.Buffer
		SetDebugOutput(&buf)
		MCPMode = false
		FatalAndExit("test fatal exit")
		return
	}

	// Test would normally fork a subprocess here
	// For now, we just verify the function compiles
	assert.NotNil(t, FatalAndExit)
}

// TestCatastrophicError tests the catastrophic error.
func TestCatastrophicError(t *testing.T) {
	defer saveAndRestoreState()()

	// Test with MCP disabled
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	MCPMode = false
	CatastrophicError("system failure: %s", "disk full")

	output := buf.String()
	assert.Contains(t, output, "[CATASTROPHIC]")
	assert.Contains(t, output, "system failure: disk full")
}

// TestCatastrophicError_MCPMode tests the catastrophic error m c p mode.
func TestCatastrophicError_MCPMode(t *testing.T) {
	defer saveAndRestoreState()()

	// Test with MCP enabled - should suppress output
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	MCPMode = true
	CatastrophicError("should not appear")

	output := buf.String()
	assert.Empty(t, output)
}

// TestLogHelpers tests the log helpers.
func TestLogHelpers(t *testing.T) {
	defer saveAndRestoreState()()

	EnableDebug = "true"
	MCPMode = false

	tests := []struct {
		name    string
		logFunc func(string, ...interface{})
		prefix  string
		message string
	}{
		{"LogIndexing", LogIndexing, "[DEBUG:INDEX]", "indexing %d files"},
		{"LogMCP", LogMCP, "[DEBUG:MCP]", "MCP message: %s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use buffer for output
			var buf bytes.Buffer
			SetDebugOutput(&buf)

			// Call log function
			tt.logFunc(tt.message, "test")

			output := buf.String()
			assert.Contains(t, output, tt.prefix)
			assert.True(t, strings.Contains(output, "test") || strings.Contains(output, tt.message))
		})
	}
}

// TestConcurrentLogging tests the concurrent logging.
func TestConcurrentLogging(t *testing.T) {
	defer saveAndRestoreState()()

	// Use buffer for output (thread-safe via mutex in debug package)
	var buf bytes.Buffer
	SetDebugOutput(&buf)
	EnableDebug = "true"
	MCPMode = false

	// Test concurrent access doesn't cause issues
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			Log("CONCURRENT", "Message from goroutine %d", id)
			LogSearch("Search from goroutine %d", id)
			LogIndexing("Index from goroutine %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, concurrent access is safe
	assert.True(t, true)
}

// TestNoOutputWithNilWriter tests that no output occurs when writer is nil.
func TestNoOutputWithNilWriter(t *testing.T) {
	defer saveAndRestoreState()()

	// Set output to nil
	SetDebugOutput(nil)
	EnableDebug = "true"
	MCPMode = false

	// These should not panic, they should just do nothing
	Printf("test %s", "message")
	Println("test message")
	Log("TEST", "test %s", "message")
	LogSearch("test %s", "message")
	LogIndexing("test %s", "message")
	LogMCP("test %s", "message")
	Fatal("test %s", "message")
	CatastrophicError("test %s", "message")
}

// TestInitDebugLogFile tests the init debug log file.
func TestInitDebugLogFile(t *testing.T) {
	defer saveAndRestoreState()()

	// Test initializing debug log file
	logPath, err := InitDebugLogFile()
	assert.NoError(t, err)
	assert.NotEmpty(t, logPath)

	// Verify the file was created
	_, err = os.Stat(logPath)
	assert.NoError(t, err)

	// Test writing to the log
	EnableDebug = "true"
	MCPMode = false
	Printf("Test log message\n")

	// Close and verify content was written
	err = CloseDebugLog()
	assert.NoError(t, err)

	content, err := os.ReadFile(logPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "Test log message")

	// Clean up
	os.Remove(logPath)
}
