package debug

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Build flag for debug mode - can be overridden at build time
// go build -ldflags "-X github.com/standardbeagle/lci/internal/debug.EnableDebug=true"
var EnableDebug = "false"

// MCPMode tracks if we're running in MCP mode (set by main)
var MCPMode = false

// debugOutput is the writer for debug output (defaults to nil, meaning no output)
var debugOutput io.Writer

// debugFile holds the open file handle if debug output goes to a file
var debugFile *os.File

// debugMutex protects access to debug output
var debugMutex sync.Mutex

// SetMCPMode enables MCP mode which suppresses all debug output to stdio
func SetMCPMode(enabled bool) {
	MCPMode = enabled
}

// SetDebugOutput sets a custom writer for debug output.
// Pass nil to disable debug output entirely.
func SetDebugOutput(w io.Writer) {
	debugMutex.Lock()
	defer debugMutex.Unlock()
	debugOutput = w
}

// InitDebugLogFile initializes debug logging to a file.
// Returns the path to the log file, or an error if initialization fails.
// Call CloseDebugLog when done to ensure the file is properly closed.
func InitDebugLogFile() (string, error) {
	debugMutex.Lock()
	defer debugMutex.Unlock()

	// Create log directory
	logDir := filepath.Join(os.TempDir(), "lci-debug-logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create debug log directory: %w", err)
	}

	// Create timestamped log file
	timestamp := time.Now().Format("2006-01-02T150405")
	logPath := filepath.Join(logDir, fmt.Sprintf("debug-%s.log", timestamp))

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create debug log file: %w", err)
	}

	debugFile = file
	debugOutput = file
	return logPath, nil
}

// CloseDebugLog closes the debug log file if one is open.
func CloseDebugLog() error {
	debugMutex.Lock()
	defer debugMutex.Unlock()

	if debugFile != nil {
		err := debugFile.Close()
		debugFile = nil
		debugOutput = nil
		return err
	}
	return nil
}

// IsDebugEnabled returns true if debug mode is enabled and we're not in MCP mode
func IsDebugEnabled() bool {
	// Never output debug info in MCP mode
	if MCPMode {
		return false
	}

	// Check build flag first
	if EnableDebug == "true" {
		return true
	}

	// Allow runtime override via environment variable
	if os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true" {
		return true
	}

	return false
}

// getDebugWriter returns the writer for debug output, or nil if none is configured
func getDebugWriter() io.Writer {
	debugMutex.Lock()
	defer debugMutex.Unlock()
	return debugOutput
}

// Printf prints debug information only when debug mode is enabled and output is configured
func Printf(format string, args ...interface{}) {
	if !IsDebugEnabled() {
		return
	}
	w := getDebugWriter()
	if w == nil {
		return
	}
	fmt.Fprintf(w, "[DEBUG] "+format, args...)
}

// Println prints debug information only when debug mode is enabled and output is configured
func Println(args ...interface{}) {
	if !IsDebugEnabled() {
		return
	}
	w := getDebugWriter()
	if w == nil {
		return
	}
	fmt.Fprint(w, "[DEBUG] ")
	fmt.Fprintln(w, args...)
}

// Log provides structured debug logging with component names
func Log(component, format string, args ...interface{}) {
	if !IsDebugEnabled() {
		return
	}
	w := getDebugWriter()
	if w == nil {
		return
	}
	fmt.Fprintf(w, "[DEBUG:%s] "+format, append([]interface{}{component}, args...)...)
}

// LogIndexing provides debug logging specifically for indexing operations
func LogIndexing(format string, args ...interface{}) {
	Log("INDEX", format, args...)
}

// LogSearch provides debug logging specifically for search operations
func LogSearch(format string, args ...interface{}) {
	Log("SEARCH", format, args...)
}

// LogMCP provides debug logging specifically for MCP operations
func LogMCP(format string, args ...interface{}) {
	Log("MCP", format, args...)
}

// Fatal outputs a catastrophic error message to the debug log and returns a fatal error.
// This function no longer calls os.Exit - callers should handle the error appropriately.
// In MCP mode, output is suppressed entirely.
func Fatal(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	if !MCPMode {
		w := getDebugWriter()
		if w != nil {
			fmt.Fprintf(w, "[FATAL] %s", msg)
		}
	}
	// Return a fatal error instead of exiting - let callers decide what to do
	return fmt.Errorf("fatal error: %s", msg)
}

// FatalAndExit outputs a catastrophic error message and exits (for CLI use only).
// This should only be used in main.go or other CLI entry points.
// Output goes to the debug log file, not stderr.
func FatalAndExit(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if !MCPMode {
		w := getDebugWriter()
		if w != nil {
			fmt.Fprintf(w, "[FATAL] %s", msg)
		}
	}
	// Always exit, but silently in MCP mode
	os.Exit(1)
}

// CatastrophicError outputs an error that indicates system failure to the debug log.
// In MCP mode, this is suppressed to maintain protocol compliance.
func CatastrophicError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if !MCPMode {
		w := getDebugWriter()
		if w != nil {
			fmt.Fprintf(w, "[CATASTROPHIC] %s", msg)
		}
	}
	// In MCP mode, errors should be returned through the protocol, not stderr
}
