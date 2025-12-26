package testhelpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// DebugLevel represents different levels of debugging verbosity
type DebugLevel int

const (
	DebugNone DebugLevel = iota
	DebugBasic
	DebugVerbose
	DebugTrace
)

// DebugConfig configures debugging behavior
type DebugConfig struct {
	Level            DebugLevel
	LogToFile        bool
	LogDirectory     string
	IncludeStack     bool
	IncludeMemory    bool
	IncludeGoroutines bool
	MaxLogSize       int64 // Max log file size in bytes
}

// TestDebugger provides debugging utilities for test execution
type TestDebugger struct {
	config     DebugConfig
	logFile    *os.File
	startTime  time.Time
	testName   string
	outputs    map[string]interface{}
	memStats   []runtime.MemStats
}

// NewTestDebugger creates a new test debugger instance
func NewTestDebugger(testName string, config DebugConfig) *TestDebugger {
	debugger := &TestDebugger{
		config:    config,
		testName:  testName,
		outputs:   make(map[string]interface{}),
		startTime: time.Now(),
		memStats:  make([]runtime.MemStats, 0),
	}

	if config.LogToFile {
		debugger.setupLogFile()
	}

	return debugger
}

// setupLogFile creates and configures the log file
func (td *TestDebugger) setupLogFile() {
	if td.config.LogDirectory == "" {
		td.config.LogDirectory = "test-logs"
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(td.config.LogDirectory, 0755); err != nil {
		fmt.Printf("Warning: Failed to create log directory: %v\n", err)
		return
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s_%s.log", td.testName, timestamp)
	logPath := filepath.Join(td.config.LogDirectory, logFileName)

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Warning: Failed to create log file: %v\n", err)
		return
	}

	td.logFile = file
	td.Logf("Debug log started for test: %s", td.testName)
}

// Logf writes a formatted log message
func (td *TestDebugger) Logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, message)

	// Always print to stdout
	fmt.Print(logLine)

	// Write to file if configured
	if td.logFile != nil {
		td.writeToFile(logLine)
	}

	// Store in outputs for later inspection
	td.outputs[fmt.Sprintf("log_%d", time.Now().UnixNano())] = message
}

// writeToFile writes to the log file with size limits
func (td *TestDebugger) writeToFile(data string) {
	if td.logFile == nil {
		return
	}

	// Check file size
	stat, err := td.logFile.Stat()
	if err == nil && td.config.MaxLogSize > 0 && stat.Size() > td.config.MaxLogSize {
		td.rotateLogFile()
	}

	_, _ = td.logFile.WriteString(data)
	_ = td.logFile.Sync()
}

// rotateLogFile rotates the current log file
func (td *TestDebugger) rotateLogFile() {
	if td.logFile != nil {
		td.logFile.Close()
	}

	// Rename current file
	oldPath := td.logFile.Name()
	newPath := strings.Replace(oldPath, ".log", "_old.log", 1)
	_ = os.Rename(oldPath, newPath)

	// Create new file
	td.setupLogFile()
}

// CaptureState captures the current system state
func (td *TestDebugger) CaptureState(label string) {
	state := make(map[string]interface{})
	state["timestamp"] = time.Now()
	state["label"] = label

	// Capture memory stats
	if td.config.IncludeMemory {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		state["memory"] = map[string]interface{}{
			"alloc":      memStats.Alloc,
			"total_alloc": memStats.TotalAlloc,
			"sys":        memStats.Sys,
			"num_gc":     memStats.NumGC,
		}
		td.memStats = append(td.memStats, memStats)
	}

	// Capture goroutine count
	if td.config.IncludeGoroutines {
		state["goroutines"] = runtime.NumGoroutine()
	}

	// Capture stack trace
	if td.config.IncludeStack {
		buf := make([]byte, 1024*1024) // 1MB buffer
		n := runtime.Stack(buf, false)
		state["stack_trace"] = string(buf[:n])
	}

	td.outputs[fmt.Sprintf("state_%s_%d", label, time.Now().UnixNano())] = state

	if td.config.Level >= DebugVerbose {
		td.Logf("State captured: %s", label)
		if memInfo, ok := state["memory"].(map[string]interface{}); ok {
			td.Logf("  Memory: Alloc=%d, Sys=%d, GC=%d",
				memInfo["alloc"], memInfo["sys"], memInfo["num_gc"])
		}
		if goroutines, ok := state["goroutines"].(int); ok {
			td.Logf("  Goroutines: %d", goroutines)
		}
	}
}

// StartOperation marks the beginning of an operation
func (td *TestDebugger) StartOperation(name string) {
	td.CaptureState(fmt.Sprintf("start_%s", name))
	if td.config.Level >= DebugBasic {
		td.Logf("🚀 Starting operation: %s", name)
	}
}

// EndOperation marks the end of an operation
func (td *TestDebugger) EndOperation(name string, err error) {
	td.CaptureState(fmt.Sprintf("end_%s", name))

	if err != nil {
		td.Logf("❌ Operation failed: %s - Error: %v", name, err)
	} else {
		td.Logf("✅ Operation completed: %s", name)
	}
}

// TraceOperation traces an operation with detailed timing
func (td *TestDebugger) TraceOperation(name string, operation func() error) error {
	if td.config.Level >= DebugTrace {
		td.Logf("🔍 Tracing operation: %s", name)
	}

	start := time.Now()
	td.StartOperation(name)

	err := operation()

	duration := time.Since(start)
	td.EndOperation(name, err)

	if td.config.Level >= DebugTrace {
		if err != nil {
			td.Logf("⏱️  %s failed after %v: %v", name, duration, err)
		} else {
			td.Logf("⏱️  %s completed in %v", name, duration)
		}
	}

	return err
}

// DumpOutputs dumps all captured outputs to JSON
func (td *TestDebugger) DumpOutputs() (string, error) {
	data, err := json.MarshalIndent(map[string]interface{}{
		"test_name": td.testName,
		"start_time": td.startTime,
		"duration":   time.Since(td.startTime),
		"outputs":    td.outputs,
		"memory_trend": td.getMemoryTrend(),
	}, "", "  ")

	if err != nil {
		return "", fmt.Errorf("failed to marshal outputs: %w", err)
	}

	return string(data), nil
}

// getMemoryTrend analyzes memory usage trends
func (td *TestDebugger) getMemoryTrend() map[string]interface{} {
	if len(td.memStats) < 2 {
		return map[string]interface{}{"trend": "insufficient_data"}
	}

	first := td.memStats[0]
	last := td.memStats[len(td.memStats)-1]

	trend := map[string]interface{}{
		"alloc_change":     int64(last.Alloc) - int64(first.Alloc),
		"sys_change":       int64(last.Sys) - int64(first.Sys),
		"gc_runs_change":   int64(last.NumGC) - int64(first.NumGC),
		"samples":          len(td.memStats),
		"peak_alloc":       uint64(0),
		"peak_sys":         uint64(0),
	}

	// Find peak values
	for _, stats := range td.memStats {
		if stats.Alloc > trend["peak_alloc"].(uint64) {
			trend["peak_alloc"] = stats.Alloc
		}
		if stats.Sys > trend["peak_sys"].(uint64) {
			trend["peak_sys"] = stats.Sys
		}
	}

	return trend
}

// SaveReport saves a detailed debug report
func (td *TestDebugger) SaveReport() error {
	if !td.config.LogToFile {
		return nil
	}

	reportPath := filepath.Join(td.config.LogDirectory,
		fmt.Sprintf("%s_report.json", td.testName))

	outputs, err := td.DumpOutputs()
	if err != nil {
		return err
	}

	return os.WriteFile(reportPath, []byte(outputs), 0644)
}

// Close cleans up debugger resources
func (td *TestDebugger) Close() error {
	td.CaptureState("test_end")

	var err error
	if td.logFile != nil {
		td.Logf("Debug log ended for test: %s", td.testName)

		// Save final report
		if saveErr := td.SaveReport(); saveErr != nil {
			td.Logf("Warning: Failed to save debug report: %v", saveErr)
		}

		err = td.logFile.Close()
		td.logFile = nil
	}

	if td.config.Level >= DebugBasic {
		duration := time.Since(td.startTime)
		td.Logf("🏁 Test completed in %v", duration)
	}

	return err
}

// TestDebugHelper provides convenient debugging methods for tests
type TestDebugHelper struct {
	debugger *TestDebugger
	t        *testing.T
}

// NewTestDebugHelper creates a new debug helper for a test
func NewTestDebugHelper(t *testing.T, level DebugLevel) *TestDebugHelper {
	t.Helper()

	config := DebugConfig{
		Level:            level,
		LogToFile:        true,
		LogDirectory:     "test-logs",
		IncludeStack:     level >= DebugVerbose,
		IncludeMemory:    true,
		IncludeGoroutines: true,
		MaxLogSize:       10 * 1024 * 1024, // 10MB
	}

	debugger := NewTestDebugger(t.Name(), config)
	helper := &TestDebugHelper{
		debugger: debugger,
		t:        t,
	}

	// Register cleanup
	t.Cleanup(func() {
		t.Helper()
		if err := helper.debugger.Close(); err != nil {
			t.Logf("Warning: Failed to close debugger: %v", err)
		}
	})

	return helper
}

// Log delegates to the underlying debugger
func (tdh *TestDebugHelper) Logf(format string, args ...interface{}) {
	tdh.t.Helper()
	tdh.debugger.Logf(format, args...)
}

// CaptureState delegates to the underlying debugger
func (tdh *TestDebugHelper) CaptureState(label string) {
	tdh.t.Helper()
	tdh.debugger.CaptureState(label)
}

// StartOperation delegates to the underlying debugger
func (tdh *TestDebugHelper) StartOperation(name string) {
	tdh.t.Helper()
	tdh.debugger.StartOperation(name)
}

// EndOperation delegates to the underlying debugger
func (tdh *TestDebugHelper) EndOperation(name string, err error) {
	tdh.t.Helper()
	tdh.debugger.EndOperation(name, err)
}

// TraceOperation delegates to the underlying debugger
func (tdh *TestDebugHelper) TraceOperation(name string, operation func() error) error {
	tdh.t.Helper()
	return tdh.debugger.TraceOperation(name, operation)
}

// FailWithDebug logs debug information before failing the test
func (tdh *TestDebugHelper) FailWithDebug(format string, args ...interface{}) {
	tdh.t.Helper()

	message := fmt.Sprintf(format, args...)
	tdh.debugger.CaptureState("test_failure")
	tdh.debugger.Logf("❌ TEST FAILED: %s", message)
	tdh.debugger.Logf("📊 Debug information captured")

	// Get debug outputs
	if outputs, err := tdh.debugger.DumpOutputs(); err == nil {
		tdh.t.Logf("📋 Debug outputs:\n%s", outputs)
	}

	tdh.t.FailNow()
}

// DebugTest is a helper function that runs a test with debugging
func DebugTest(t *testing.T, level DebugLevel, testFunc func(*TestDebugHelper)) {
	t.Helper()

	debugger := NewTestDebugHelper(t, level)
	debugger.Logf("🧪 Starting test with debugging level: %d", level)

	testFunc(debugger)
}

// ReplayTest replays a test execution from saved debug data
func ReplayTest(logPath string) error {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	fmt.Printf("=== Test Replay ===\n")
	fmt.Printf("Log file: %s\n", logPath)
	fmt.Printf("Content:\n%s\n", string(data))

	return nil
}

// AnalyzeDebugLogs analyzes debug logs for patterns and issues
func AnalyzeDebugLogs(logDir string) error {
	files, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		return fmt.Errorf("failed to find log files: %w", err)
	}

	fmt.Printf("=== Debug Log Analysis ===\n")
	fmt.Printf("Found %d log files\n", len(files))

	for _, file := range files {
		fmt.Printf("\n--- Analyzing %s ---\n", filepath.Base(file))

		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			continue
		}

		lines := strings.Split(string(content), "\n")
		errors := 0
		warnings := 0
		operations := 0

		for _, line := range lines {
			if strings.Contains(line, "❌") {
				errors++
			} else if strings.Contains(line, "⚠️") {
				warnings++
			} else if strings.Contains(line, "🚀") {
				operations++
			}
		}

		fmt.Printf("Operations: %d, Errors: %d, Warnings: %d\n",
			operations, errors, warnings)
	}

	return nil
}

// GetDebugLogs returns paths to all debug log files
func GetDebugLogs(logDir string) ([]string, error) {
	pattern := filepath.Join(logDir, "*.log")
	return filepath.Glob(pattern)
}

// CleanupOldLogs removes old debug log files
func CleanupOldLogs(logDir string, maxAge time.Duration) error {
	files, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		return fmt.Errorf("failed to find log files: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if os.Remove(file) == nil {
				removed++
			}
		}
	}

	if removed > 0 {
		fmt.Printf("Cleaned up %d old log files\n", removed)
	}

	return nil
}