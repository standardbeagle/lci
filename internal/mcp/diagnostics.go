package mcp

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DiagnosticLogger handles all diagnostic output for MCP server.
// CRITICAL: All output must go to file, never to stdout/stderr during MCP operation.
// The MCP protocol requires clean stdio for communication with the client.
//
// FILESERVICE COMPLIANCE NOTE: This component directly accesses os filesystem APIs
// instead of using FileService. This is an intentional architectural exception because:
// 1. Diagnostic logging must work BEFORE FileService initialization
// 2. It only writes to system temp/home directories, not project files
// 3. It's infrastructure code, not core indexing logic
// 4. Failure must not prevent MCP server startup
type DiagnosticLogger struct {
	mu       sync.Mutex
	file     *os.File
	logger   *log.Logger
	filePath string
	isMCP    bool // true when running as MCP server
}

// NewDiagnosticLogger creates a logger that writes to a file instead of stderr.
// This is CRITICAL for MCP protocol compliance - stdout/stderr must remain clean.
func NewDiagnosticLogger(isMCP bool) *DiagnosticLogger {
	dl := &DiagnosticLogger{
		isMCP: isMCP,
	}

	if isMCP {
		// Create diagnostic log file in system temp directory
		logDir := filepath.Join(os.TempDir(), "lci-mcp-logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// Fallback: use home directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				homeDir = "."
			}
			logDir = filepath.Join(homeDir, ".lci-mcp-logs")
			if err := os.MkdirAll(logDir, 0755); err != nil {
				log.Printf("Warning: Failed to create log directory %s: %v", logDir, err)
				// Continue anyway - logging is not critical
			}
		}

		// Create timestamped log file
		timestamp := time.Now().Format("2006-01-02T150405")
		logPath := filepath.Join(logDir, fmt.Sprintf("mcp-%s.log", timestamp))

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// If file creation fails, disable logging rather than breaking MCP
			dl.logger = log.New(io.Discard, "", 0)
			return dl
		}

		dl.file = file
		dl.filePath = logPath
		dl.logger = log.New(file, "[MCP] ", log.LstdFlags|log.Lshortfile)
	} else {
		// In CLI mode, logging to stderr is acceptable
		dl.logger = log.New(os.Stderr, "[MCP] ", log.LstdFlags)
	}

	return dl
}

// Printf logs a diagnostic message. In MCP mode, goes to file. In CLI mode, goes to stderr.
func (dl *DiagnosticLogger) Printf(format string, v ...interface{}) {
	if dl == nil || dl.logger == nil {
		return
	}
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.logger.Printf(format, v...)
}

// Errorf logs an error. In MCP mode, goes to file. Never to stderr.
func (dl *DiagnosticLogger) Errorf(format string, v ...interface{}) {
	if dl == nil || dl.logger == nil {
		return
	}
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.logger.Printf("ERROR: "+format, v...)
}

// Close closes the log file if it's open.
func (dl *DiagnosticLogger) Close() error {
	if dl == nil {
		return nil
	}
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.file != nil {
		return dl.file.Close()
	}
	return nil
}

// GetLogPath returns the path to the diagnostic log file (if MCP mode)
func (dl *DiagnosticLogger) GetLogPath() string {
	if dl == nil {
		return ""
	}
	return dl.filePath
}

// NoOpLogger is used to suppress all logging
var NoOpLogger = &DiagnosticLogger{
	logger: log.New(io.Discard, "", 0),
}
