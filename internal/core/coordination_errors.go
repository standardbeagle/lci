package core

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/debug"
)

// ErrorCode represents different types of coordination errors
type ErrorCode string

const (
	// Lock timeout errors
	ErrCodeLockTimeout     ErrorCode = "LOCK_TIMEOUT"
	ErrCodeLockUnavailable ErrorCode = "LOCK_UNAVAILABLE"

	// Deadlock and contention errors
	ErrCodeDeadlockDetected  ErrorCode = "DEADLOCK_DETECTED"
	ErrCodeContentionTooHigh ErrorCode = "CONTENTION_TOO_HIGH"

	// Index state errors
	ErrCodeInvalidIndexType ErrorCode = "INVALID_INDEX_TYPE"
	ErrCodeIndexUnavailable ErrorCode = "INDEX_UNAVAILABLE"
	ErrCodeIndexCorrupted   ErrorCode = "INDEX_CORRUPTED"

	// Resource and limit errors
	ErrCodeConcurrentLimit   ErrorCode = "CONCURRENT_LIMIT"
	ErrCodeResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"
	ErrCodeMemoryLimit       ErrorCode = "MEMORY_LIMIT"

	// System errors
	ErrCodeSystemShutdown     ErrorCode = "SYSTEM_SHUTDOWN"
	ErrCodeConfigurationError ErrorCode = "CONFIGURATION_ERROR"
)

// CoordinationError represents an error that occurs during index coordination
type CoordinationError struct {
	Code      ErrorCode    `json:"code"`
	Message   string       `json:"message"`
	Details   string       `json:"details,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Retryable bool         `json:"retryable"`
	Context   ErrorContext `json:"context,omitempty"`
}

// ErrorContext provides additional context for coordination errors
type ErrorContext struct {
	IndexType     IndexType     `json:"indexType,omitempty"`
	OperationType string        `json:"operationType,omitempty"`
	LockType      LockType      `json:"lockType,omitempty"`
	WaitTime      time.Duration `json:"waitTime,omitempty"`
	ConcurrentOps int           `json:"concurrentOps,omitempty"`
	QueueDepth    int           `json:"queueDepth,omitempty"`
}

// Error implements the error interface
func (ce *CoordinationError) Error() string {
	if ce.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", ce.Code, ce.Message, ce.Details)
	}
	return fmt.Sprintf("[%s] %s", ce.Code, ce.Message)
}

// IsRetryable returns whether the error is potentially retryable
func (ce *CoordinationError) IsRetryable() bool {
	return ce.Retryable
}

// WithContext adds context to the coordination error
func (ce *CoordinationError) WithContext(context ErrorContext) *CoordinationError {
	newError := *ce
	newError.Context = context
	return &newError
}

// NewCoordinationError creates a new coordination error
func NewCoordinationError(code ErrorCode, message string) *CoordinationError {
	return &CoordinationError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryableError(code),
	}
}

// NewCoordinationErrorWithDetails creates a new coordination error with details
func NewCoordinationErrorWithDetails(code ErrorCode, message, details string) *CoordinationError {
	return &CoordinationError{
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
		Retryable: isRetryableError(code),
	}
}

// isRetryableError determines if an error type is retryable
func isRetryableError(code ErrorCode) bool {
	switch code {
	case ErrCodeLockTimeout, ErrCodeLockUnavailable, ErrCodeContentionTooHigh,
		ErrCodeConcurrentLimit, ErrCodeResourceExhausted:
		return true
	case ErrCodeDeadlockDetected, ErrCodeInvalidIndexType, ErrCodeIndexCorrupted,
		ErrCodeSystemShutdown, ErrCodeConfigurationError:
		return false
	case ErrCodeIndexUnavailable:
		return true // Can retry if index becomes available
	default:
		return false
	}
}

// Common error creation functions

// NewLockTimeoutError creates a lock timeout error
func NewLockTimeoutError(indexType IndexType, lockType LockType, waitTime time.Duration) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeLockTimeout,
		"Lock acquisition timeout for "+indexType.String(),
		fmt.Sprintf("Lock type: %s, Wait time: %v", lockType.String(), waitTime),
	).WithContext(ErrorContext{
		IndexType: indexType,
		LockType:  lockType,
		WaitTime:  waitTime,
	})
}

// NewLockUnavailableError creates a lock unavailable error
func NewLockUnavailableError(indexType IndexType, reason string) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeLockUnavailable,
		"Lock unavailable for "+indexType.String(),
		reason,
	).WithContext(ErrorContext{
		IndexType: indexType,
	})
}

// NewDeadlockDetectedError creates a deadlock detected error
func NewDeadlockDetectedError(operation string, involvedIndexes []IndexType) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeDeadlockDetected,
		"Potential deadlock detected",
		fmt.Sprintf("Operation: %s, Involved indexes: %v", operation, involvedIndexes),
	)
}

// NewInvalidIndexTypeError creates an invalid index type error
func NewInvalidIndexTypeError(indexType IndexType) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeInvalidIndexType,
		"Invalid index type: "+indexType.String(),
		"Index type is not recognized or supported",
	).WithContext(ErrorContext{
		IndexType: indexType,
	})
}

// NewIndexUnavailableError creates an index unavailable error
func NewIndexUnavailableError(indexType IndexType, reason string) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeIndexUnavailable,
		"Index unavailable: "+indexType.String(),
		reason,
	).WithContext(ErrorContext{
		IndexType: indexType,
	})
}

// NewConcurrentLimitError creates a concurrent limit error
func NewConcurrentLimitError(current, limit int) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeConcurrentLimit,
		"Concurrent operation limit exceeded",
		fmt.Sprintf("Current: %d, Limit: %d", current, limit),
	).WithContext(ErrorContext{
		ConcurrentOps: current,
	})
}

// NewSystemShutdownError creates a system shutdown error
func NewSystemShutdownError(reason string) *CoordinationError {
	return NewCoordinationErrorWithDetails(
		ErrCodeSystemShutdown,
		"System is shutting down",
		reason,
	)
}

// IsCoordinationError checks if an error is a coordination error
func IsCoordinationError(err error) bool {
	_, ok := err.(*CoordinationError)
	return ok
}

// GetErrorCode extracts the error code from a coordination error
func GetErrorCode(err error) ErrorCode {
	if coordErr, ok := err.(*CoordinationError); ok {
		return coordErr.Code
	}
	return ""
}

// GetRetryDelay calculates appropriate retry delay based on error type and context
func GetRetryDelay(err error, attempt int) time.Duration {
	if coordErr, ok := err.(*CoordinationError); ok {
		switch coordErr.Code {
		case ErrCodeLockTimeout, ErrCodeLockUnavailable:
			// Exponential backoff for lock issues
			return time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
		case ErrCodeContentionTooHigh:
			// Longer delays for high contention
			return time.Duration(attempt) * time.Second
		case ErrCodeConcurrentLimit:
			// Moderate delays for limit issues
			return time.Duration(attempt) * 500 * time.Millisecond
		case ErrCodeIndexUnavailable:
			// Variable delays based on index state
			return time.Duration(attempt) * 200 * time.Millisecond
		default:
			// Default delay for unknown retryable errors
			return time.Second
		}
	}
	return 0 // No retry for non-retryable errors
}

// ErrorLogger interface for logging coordination errors
type ErrorLogger interface {
	LogError(err *CoordinationError)
	LogWarning(message string, context ErrorContext)
	LogInfo(message string, context ErrorContext)
}

// DefaultErrorLogger provides a basic implementation of ErrorLogger using Go's standard log package
type DefaultErrorLogger struct {
	errorLogger *log.Logger
	warnLogger  *log.Logger
	infoLogger  *log.Logger
}

// NewDefaultErrorLogger creates a new default error logger with standard output
func NewDefaultErrorLogger() *DefaultErrorLogger {
	infoWriter := os.Stderr

	return &DefaultErrorLogger{
		errorLogger: log.New(os.Stderr, "[INDEX-ERROR] ", log.LstdFlags|log.Lmicroseconds),
		warnLogger:  log.New(os.Stderr, "[INDEX-WARN]  ", log.LstdFlags|log.Lmicroseconds),
		infoLogger:  log.New(infoWriter, "[INDEX-INFO]  ", log.LstdFlags|log.Lmicroseconds),
	}
}

// LogError logs a coordination error with detailed context
func (del *DefaultErrorLogger) LogError(err *CoordinationError) {
	// Enhanced error logging with context details
	contextStr := ""
	if err.Context.IndexType.IsValid() {
		contextStr += " Index:" + err.Context.IndexType.String()
	}
	if err.Context.OperationType != "" {
		contextStr += " Op:" + err.Context.OperationType
	}
	if err.Context.WaitTime > 0 {
		contextStr += fmt.Sprintf(" Wait:%v", err.Context.WaitTime)
	}
	if err.Context.ConcurrentOps > 0 {
		contextStr += fmt.Sprintf(" Concurrent:%d", err.Context.ConcurrentOps)
	}
	if err.Context.QueueDepth > 0 {
		contextStr += fmt.Sprintf(" QueueDepth:%d", err.Context.QueueDepth)
	}

	del.errorLogger.Printf("%s%s\n", err.Error(), contextStr)
}

// LogWarning logs a warning message with context
func (del *DefaultErrorLogger) LogWarning(message string, context ErrorContext) {
	contextStr := del.formatContext(context)
	del.warnLogger.Printf("%s%s\n", message, contextStr)
}

// LogInfo logs an info message with context
func (del *DefaultErrorLogger) LogInfo(message string, context ErrorContext) {
	// Suppress info logs in MCP mode to prevent stdout/stderr pollution
	if debug.MCPMode {
		return
	}
	contextStr := del.formatContext(context)
	del.infoLogger.Printf("%s%s\n", message, contextStr)
}

// formatContext formats ErrorContext for logging
func (del *DefaultErrorLogger) formatContext(context ErrorContext) string {
	var parts []string

	if context.IndexType.IsValid() {
		parts = append(parts, "Index:"+context.IndexType.String())
	}
	if context.OperationType != "" {
		parts = append(parts, "Op:"+context.OperationType)
	}
	if context.LockType == ReadLock || context.LockType == WriteLock {
		parts = append(parts, "Lock:"+context.LockType.String())
	}
	if context.WaitTime > 0 {
		parts = append(parts, fmt.Sprintf("Wait:%v", context.WaitTime))
	}
	if context.ConcurrentOps > 0 {
		parts = append(parts, fmt.Sprintf("Concurrent:%d", context.ConcurrentOps))
	}
	if context.QueueDepth > 0 {
		parts = append(parts, fmt.Sprintf("Queue:%d", context.QueueDepth))
	}

	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(" [%s]", fmt.Sprintf("%s", parts))
}

// ErrorHandler provides centralized error handling for coordination operations
type ErrorHandler struct {
	logger ErrorLogger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger ErrorLogger) *ErrorHandler {
	if logger == nil {
		logger = NewDefaultErrorLogger()
	}
	return &ErrorHandler{logger: logger}
}

// HandleError processes a coordination error
func (eh *ErrorHandler) HandleError(err *CoordinationError) {
	eh.logger.LogError(err)
}

// HandleWarning processes a warning condition
func (eh *ErrorHandler) HandleWarning(message string, context ErrorContext) {
	// Suppress warnings in MCP mode to prevent stderr pollution
	if debug.MCPMode {
		return
	}
	eh.logger.LogWarning(message, context)
}

// HandleInfo processes an informational message
func (eh *ErrorHandler) HandleInfo(message string, context ErrorContext) {
	eh.logger.LogInfo(message, context)
}

// Global error handler instance
var globalErrorHandler *ErrorHandler
var globalErrorHandlerOnce sync.Once

// GetGlobalErrorHandler returns the singleton global error handler
func GetGlobalErrorHandler() *ErrorHandler {
	globalErrorHandlerOnce.Do(func() {
		globalErrorHandler = NewErrorHandler(nil)
	})
	return globalErrorHandler
}

// SetGlobalErrorHandler sets the global error handler
func SetGlobalErrorHandler(handler *ErrorHandler) {
	globalErrorHandler = handler
}

// LogCoordinationError logs a coordination error using the global error handler
func LogCoordinationError(err *CoordinationError) {
	GetGlobalErrorHandler().HandleError(err)
}

// LogCoordinationWarning logs a coordination warning using the global error handler
func LogCoordinationWarning(message string, context ErrorContext) {
	GetGlobalErrorHandler().HandleWarning(message, context)
}

// LogCoordinationInfo logs a coordination info message using the global error handler
func LogCoordinationInfo(message string, context ErrorContext) {
	// Suppress info logs in MCP mode to prevent stdout pollution
	if debug.MCPMode {
		return
	}
	GetGlobalErrorHandler().HandleInfo(message, context)
}

// CoordinationInfoEnabled returns true if coordination info logging is enabled.
// Use this to guard expensive fmt.Sprintf calls before LogCoordinationInfo.
func CoordinationInfoEnabled() bool {
	return !debug.MCPMode
}

// T041: Enhanced Logging Functions for Independent Index System Operations

// IndexSystemLogger provides specialized logging for index system operations
type IndexSystemLogger struct {
	*DefaultErrorLogger
}

// NewIndexSystemLogger creates a new index system logger
func NewIndexSystemLogger() *IndexSystemLogger {
	return &IndexSystemLogger{
		DefaultErrorLogger: NewDefaultErrorLogger(),
	}
}

// LogIndexStateChange logs changes to index state
func (isl *IndexSystemLogger) LogIndexStateChange(indexType IndexType, oldStatus, newStatus string, context map[string]interface{}) {
	contextMsg := fmt.Sprintf(" State:%s->%s", oldStatus, newStatus)

	// Add additional context
	if progress, ok := context["progress"].(float64); ok {
		contextMsg += fmt.Sprintf(" Progress:%.1f%%", progress)
	}
	if operation, ok := context["operation"].(string); ok {
		contextMsg += " Operation:" + operation
	}
	if reason, ok := context["reason"].(string); ok {
		contextMsg += " Reason:" + reason
	}

	// Only log in debug mode for state changes
	debug.Printf("Index state changed for %s%s\n", indexType.String(), contextMsg)
}

// LogIndexOperation logs index operations with timing
func (isl *IndexSystemLogger) LogIndexOperation(indexType IndexType, operation string, startTime time.Time, duration time.Duration, success bool, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}

	contextMsg := fmt.Sprintf(" Op:%s Duration:%v Status:%s", operation, duration, status)

	// Add operation-specific details
	if priority, ok := details["priority"].(int); ok {
		contextMsg += fmt.Sprintf(" Priority:%d", priority)
	}
	if progress, ok := details["progress"].(float64); ok {
		contextMsg += fmt.Sprintf(" Progress:%.1f%%", progress)
	}
	if filesProcessed, ok := details["filesProcessed"].(int); ok {
		contextMsg += fmt.Sprintf(" Files:%d", filesProcessed)
	}
	if errors, ok := details["errors"].(int); ok {
		contextMsg += fmt.Sprintf(" Errors:%d", errors)
	}

	if success {
		// Only log successful operations in debug mode
		debug.Printf("Index operation completed for %s%s\n", indexType.String(), contextMsg)
	} else {
		if errorMsg, ok := details["error"].(string); ok {
			contextMsg += " Error:" + errorMsg
		}
		// Always log failures
		isl.errorLogger.Printf("Index operation failed for %s%s\n", indexType.String(), contextMsg)
	}
}

// LogLockOperation logs lock operations with contention details
func (isl *IndexSystemLogger) LogLockOperation(indexType IndexType, lockType LockType, operation string, waitTime time.Duration, acquired bool, queueDepth int) {
	status := "ACQUIRED"
	if !acquired {
		status = "FAILED"
	}

	contextMsg := fmt.Sprintf(" Lock:%s Op:%s Wait:%v Status:%s", lockType.String(), operation, waitTime, status)
	if queueDepth > 0 {
		contextMsg += fmt.Sprintf(" QueueDepth:%d", queueDepth)
	}

	if acquired {
		if waitTime > 100*time.Millisecond {
			// Log warning for slow lock acquisition - always log warnings
			isl.warnLogger.Printf("Slow lock acquisition for %s%s\n", indexType.String(), contextMsg)
		} else {
			// Only log successful lock operations in debug mode
			// Use the debug package for consistency
			debug.Printf("Lock operation for %s%s\n", indexType.String(), contextMsg)
		}
	} else {
		// Always log failures
		isl.errorLogger.Printf("Lock operation failed for %s%s\n", indexType.String(), contextMsg)
	}
}

// LogHealthCheck logs health check results
func (isl *IndexSystemLogger) LogHealthCheck(indexType IndexType, health HealthStatus, availability float64, responseTime time.Duration, issues []string) {
	contextMsg := fmt.Sprintf(" Health:%s Availability:%.1f%% ResponseTime:%v", health.String(), availability, responseTime)

	if len(issues) > 0 {
		contextMsg += fmt.Sprintf(" Issues:%v", issues)
	}

	switch health {
	case HealthStatusHealthy:
		// Only log healthy status in debug mode
		debug.Printf("Health check passed for %s%s\n", indexType.String(), contextMsg)
	case HealthStatusDegraded:
		// Always log degraded status
		isl.warnLogger.Printf("Health check degraded for %s%s\n", indexType.String(), contextMsg)
	case HealthStatusUnhealthy:
		// Always log unhealthy status
		isl.errorLogger.Printf("Health check failed for %s%s\n", indexType.String(), contextMsg)
	}
}

// LogQueueOperation logs queue operations
func (isl *IndexSystemLogger) LogQueueOperation(operation string, queueDepth, maxDepth int, avgWaitTime time.Duration, details map[string]interface{}) {
	contextMsg := fmt.Sprintf(" Op:%s Depth:%d/%d AvgWait:%v", operation, queueDepth, maxDepth, avgWaitTime)

	// Add queue-specific details
	if throughput, ok := details["throughput"].(float64); ok {
		contextMsg += fmt.Sprintf(" Throughput:%.1f/sec", throughput)
	}
	if priority, ok := details["priority"].(int); ok {
		contextMsg += fmt.Sprintf(" Priority:%d", priority)
	}

	// Log warnings for queue issues
	if queueDepth > maxDepth*80/100 { // 80% capacity
		isl.warnLogger.Printf("Queue capacity warning%s\n", contextMsg)
	} else if avgWaitTime > 5*time.Second {
		isl.warnLogger.Printf("Queue wait time warning%s\n", contextMsg)
	} else {
		// Only log normal queue operations in debug mode
		debug.Printf("Queue operation%s\n", contextMsg)
	}
}

// LogMetricsSnapshot logs periodic metrics snapshots
func (isl *IndexSystemLogger) LogMetricsSnapshot(indexType IndexType, metrics map[string]interface{}) {
	var parts []string

	if concurrentOps, ok := metrics["concurrentOps"].(int); ok {
		parts = append(parts, fmt.Sprintf("Concurrent:%d", concurrentOps))
	}
	if lockWaitTime, ok := metrics["avgLockWaitTime"].(time.Duration); ok {
		parts = append(parts, fmt.Sprintf("AvgLockWait:%v", lockWaitTime))
	}
	if contentionRate, ok := metrics["contentionRate"].(float64); ok {
		parts = append(parts, fmt.Sprintf("Contention:%.1f%%", contentionRate))
	}
	if throughput, ok := metrics["throughput"].(float64); ok {
		parts = append(parts, fmt.Sprintf("Throughput:%.1f/sec", throughput))
	}
	if errorRate, ok := metrics["errorRate"].(float64); ok {
		parts = append(parts, fmt.Sprintf("ErrorRate:%.1f%%", errorRate))
	}

	contextMsg := ""
	if len(parts) > 0 {
		contextMsg = fmt.Sprintf(" [%s]", fmt.Sprintf("%s", parts))
	}

	// Only log metrics snapshots in debug mode
	debug.Printf("Metrics snapshot for %s%s\n", indexType.String(), contextMsg)
}

// Global index system logger instance
var globalIndexSystemLogger *IndexSystemLogger
var globalIndexSystemLoggerOnce sync.Once

// GetGlobalIndexSystemLogger returns the singleton global index system logger
func GetGlobalIndexSystemLogger() *IndexSystemLogger {
	globalIndexSystemLoggerOnce.Do(func() {
		globalIndexSystemLogger = NewIndexSystemLogger()
	})
	return globalIndexSystemLogger
}

// SetGlobalIndexSystemLogger sets the global index system logger
func SetGlobalIndexSystemLogger(logger *IndexSystemLogger) {
	globalIndexSystemLogger = logger
}

// Convenience functions for common index system logging operations

// LogIndexStateChange logs index state changes using the global logger
func LogIndexStateChange(indexType IndexType, oldStatus, newStatus string, context map[string]interface{}) {
	GetGlobalIndexSystemLogger().LogIndexStateChange(indexType, oldStatus, newStatus, context)
}

// LogIndexOperation logs index operations using the global logger
func LogIndexOperation(indexType IndexType, operation string, startTime time.Time, duration time.Duration, success bool, details map[string]interface{}) {
	GetGlobalIndexSystemLogger().LogIndexOperation(indexType, operation, startTime, duration, success, details)
}

// LogLockOperation logs lock operations using the global logger
func LogLockOperation(indexType IndexType, lockType LockType, operation string, waitTime time.Duration, acquired bool, queueDepth int) {
	GetGlobalIndexSystemLogger().LogLockOperation(indexType, lockType, operation, waitTime, acquired, queueDepth)
}

// LogHealthCheck logs health checks using the global logger
func LogHealthCheck(indexType IndexType, health HealthStatus, availability float64, responseTime time.Duration, issues []string) {
	GetGlobalIndexSystemLogger().LogHealthCheck(indexType, health, availability, responseTime, issues)
}

// LogQueueOperation logs queue operations using the global logger
func LogQueueOperation(operation string, queueDepth, maxDepth int, avgWaitTime time.Duration, details map[string]interface{}) {
	GetGlobalIndexSystemLogger().LogQueueOperation(operation, queueDepth, maxDepth, avgWaitTime, details)
}

// LogMetricsSnapshot logs metrics snapshots using the global logger
func LogMetricsSnapshot(indexType IndexType, metrics map[string]interface{}) {
	GetGlobalIndexSystemLogger().LogMetricsSnapshot(indexType, metrics)
}
