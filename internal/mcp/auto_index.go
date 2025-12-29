package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
)

// AutoIndexingManager manages auto-indexing with proper synchronization
type AutoIndexingManager struct {
	server      *Server
	fileService *core.FileService
	mu          sync.RWMutex

	// Atomic state fields to prevent race conditions
	running      int32 // atomic bool
	cancelling   int32 // atomic bool
	sessionCount int64 // already atomic
	closed       int32 // atomic bool to prevent double-close

	// Protected fields - require mutex access
	sessionID    string
	rootPath     string
	status       string
	progress     float64
	errorMessage string
	startTime    time.Time

	// Event notification for state changes - eliminates polling
	statusChan chan string   // Channel for status change notifications
	doneChan   chan struct{} // Channel for completion notification
}

// NewAutoIndexingManager creates a new auto-indexing manager
func NewAutoIndexingManager(server *Server) *AutoIndexingManager {
	return &AutoIndexingManager{
		server:      server,
		fileService: core.NewFileService(),
		status:      "idle",
		progress:    0.0,
		statusChan:  make(chan string, 10),  // Buffered channel for status updates
		doneChan:    make(chan struct{}, 1), // Buffered to prevent dropped completion signals
	}
}

// startAutoIndexing begins the auto-indexing process with proper synchronization
func (m *AutoIndexingManager) startAutoIndexing(rootPath string, cfg *config.Config) error {
	// Use atomic compare-and-swap to prevent race conditions on start
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return errors.New("auto-indexing is already running")
	}

	// Reset cancellation state
	atomic.StoreInt32(&m.cancelling, 0)

	// Initialize protected state under mutex
	m.mu.Lock()
	m.status = "estimating"
	m.progress = 0.0
	m.sessionID = fmt.Sprintf("auto_index_%d", atomic.AddInt64(&m.sessionCount, 1))
	m.rootPath = rootPath
	m.startTime = time.Now()
	m.errorMessage = ""
	m.mu.Unlock()

	// Start indexing in background goroutine
	go m.runAutoIndexing(rootPath, cfg)

	return nil
}

// runAutoIndexing executes the auto-indexing process with proper error handling
func (m *AutoIndexingManager) runAutoIndexing(rootPath string, cfg *config.Config) {
	defer func() {
		// Reset atomic state on completion
		atomic.StoreInt32(&m.running, 0)
		atomic.StoreInt32(&m.cancelling, 0)
	}()

	// Validate initial state
	if m.server == nil {
		m.setErrorState(errors.New("server is nil"))
		return
	}

	// Apply startup delay to let UI become responsive before CPU-intensive indexing
	// This reduces typing lag in Claude Code during startup
	if cfg.Performance.StartupDelayMs > 0 {
		m.updateStatus("waiting")
		delay := time.Duration(cfg.Performance.StartupDelayMs) * time.Millisecond
		m.server.diagnosticLogger.Printf("Waiting %v before starting auto-indexing (startup_delay_ms)", delay)

		// Check for cancellation during the delay
		select {
		case <-time.After(delay):
			// Delay completed, continue to indexing
		default:
			// Check cancellation periodically during the delay
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			deadline := time.Now().Add(delay)
			for time.Now().Before(deadline) {
				select {
				case <-ticker.C:
					if atomic.LoadInt32(&m.cancelling) == 1 {
						m.updateStatus("cancelled")
						return
					}
				}
			}
		}
	}

	// Start indexing (no separate estimation step)
	m.updateStatus("indexing")

	m.server.diagnosticLogger.Printf("Starting auto-indexing for %s", rootPath)

	// Check for cancellation before starting (atomic check)
	if atomic.LoadInt32(&m.cancelling) == 1 {
		m.updateStatus("cancelled")
		return
	}

	// Perform actual indexing
	ctx := context.Background()
	start := time.Now()

	// NOTE: Removed waitForIndexReady() call - it was causing 30-second delays
	// for fresh indexes. For a fresh index creation, there's nothing to wait for.
	// The index is ready immediately after server initialization.

	err := m.server.goroutineIndex.IndexDirectory(ctx, rootPath)
	elapsed := time.Since(start)

	// Step 3: Update final state
	if err != nil {
		if atomic.LoadInt32(&m.cancelling) == 1 {
			m.updateStatus("cancelled")
			m.server.diagnosticLogger.Printf("Auto-indexing cancelled after %s", elapsed)
		} else {
			m.setErrorState(fmt.Errorf("auto-indexing failed: %w", err))
		}
	} else {
		// Get actual indexed file count
		fileCount := m.server.goroutineIndex.GetFileCount()
		m.updateStatus("completed")
		m.server.diagnosticLogger.Printf("Auto-indexing completed successfully: %d files indexed in %s",
			fileCount, elapsed)
	}
}

// NOTE: waitForIndexReady() function removed - it caused 30-second delays for fresh indexes.
// For desktop software, initialization should be instant. The index is ready immediately
// after MasterIndex creation in NewServer().

// setErrorState sets the error state with proper synchronization
func (m *AutoIndexingManager) setErrorState(err error) {
	m.mu.Lock()
	m.status = "failed"
	m.errorMessage = err.Error()
	m.mu.Unlock()

	// Sync with server state to prevent inconsistencies
	m.syncWithServerState()

	m.server.diagnosticLogger.Printf("Auto-indexing error: %v", err)
}

// getStatus returns the current status as a string
func (m *AutoIndexingManager) getStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// getProgress returns the current progress (0.0-1.0)
func (m *AutoIndexingManager) getProgress() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.progress
}

// cancel cancels the current auto-indexing operation
func (m *AutoIndexingManager) cancel() bool {
	// Use atomic compare-and-swap for cancellation
	if atomic.LoadInt32(&m.running) == 1 {
		return atomic.CompareAndSwapInt32(&m.cancelling, 0, 1)
	}
	return false
}

// isRunning returns true if auto-indexing is currently running
func (m *AutoIndexingManager) isRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}

// isCancelled returns true if auto-indexing is currently being cancelled
func (m *AutoIndexingManager) isCancelled() bool {
	return atomic.LoadInt32(&m.cancelling) == 1
}

// getSessionID returns the current session ID
func (m *AutoIndexingManager) getSessionID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionID
}

// getRootPath returns the current root path
func (m *AutoIndexingManager) getRootPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rootPath
}

// getErrorMessage returns the current error message
func (m *AutoIndexingManager) getErrorMessage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorMessage
}

// getStartTime returns the start time
func (m *AutoIndexingManager) getStartTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.startTime
}

// syncWithServerState synchronizes AutoIndexingManager state with server indexing state
// This helps prevent race conditions between the two state management systems
func (m *AutoIndexingManager) syncWithServerState() {
	// Update server's indexing state to match AutoIndexingManager state
	m.mu.RLock()
	status := m.status
	sessionID := m.sessionID
	rootPath := m.rootPath
	progress := m.progress
	errorMessage := m.errorMessage
	isRunning := m.isRunning()
	isCancelled := m.isCancelled()
	m.mu.RUnlock()

	// Update server state under its mutex
	m.server.indexingMutex.Lock()
	defer m.server.indexingMutex.Unlock()

	if m.server.indexingState != nil {
		m.server.indexingState.Status = status
		m.server.indexingState.SessionID = sessionID
		m.server.indexingState.RootPath = rootPath
		m.server.indexingState.Progress = progress
		m.server.indexingState.ErrorMessage = errorMessage
		m.server.indexingState.CanCancel = isRunning && !isCancelled
	}
}

// updateStatus updates the status and syncs with server state
func (m *AutoIndexingManager) updateStatus(newStatus string) {
	m.mu.Lock()
	m.status = newStatus
	m.mu.Unlock()

	// Only send notifications if not closed
	if atomic.LoadInt32(&m.closed) == 0 {
		// Send status change notification (non-blocking)
		select {
		case m.statusChan <- newStatus:
		default:
			// Channel is full, skip notification to avoid blocking
		}

		// Sync with server state to prevent inconsistencies
		m.syncWithServerState()

		// If this is a final status, notify completion
		if newStatus == "completed" || newStatus == "failed" || newStatus == "cancelled" {
			select {
			case m.doneChan <- struct{}{}:
			default:
				// Channel is full, skip notification
			}
		}
	} else {
		// Still sync with server state even if closed
		m.syncWithServerState()
	}
}

// updateProgress updates the progress and syncs with server state
func (m *AutoIndexingManager) updateProgress(newProgress float64) {
	m.mu.Lock()
	m.progress = newProgress
	m.mu.Unlock()

	// Sync with server state to prevent inconsistencies
	m.syncWithServerState()
}

// waitForCompletion waits for indexing to complete using event-driven notification
// This eliminates the need for arbitrary timeouts and polling
func (m *AutoIndexingManager) waitForCompletion(timeout time.Duration) (string, error) {
	if !m.isRunning() {
		return m.getStatus(), nil // Not running, return current status immediately
	}

	// Create a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for either completion or timeout
	select {
	case <-m.doneChan:
		// Got completion notification
		return m.getStatus(), nil
	case <-ctx.Done():
		// Timeout occurred
		return m.getStatus(), fmt.Errorf("timeout after %v waiting for completion", timeout)
	}
}

// waitForStatusChange waits for a specific status or completion
// This eliminates polling and provides deterministic behavior
func (m *AutoIndexingManager) waitForStatusChange(targetStatuses []string, timeout time.Duration) (string, error) {
	// Check current status first
	currentStatus := m.getStatus()
	for _, target := range targetStatuses {
		if currentStatus == target {
			return currentStatus, nil
		}
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()

	// Wait for status change
	select {
	case newStatus := <-m.statusChan:
		// Check if this is one of our target statuses
		for _, target := range targetStatuses {
			if newStatus == target {
				return newStatus, nil
			}
		}
		// Not a target status, continue waiting with remaining time
		remainingTime := timeout - time.Since(startTime)
		if remainingTime <= 0 {
			return m.getStatus(), fmt.Errorf("timeout after %v waiting for status change", timeout)
		}
		return m.waitForStatusChange(targetStatuses, remainingTime)
	case <-m.doneChan:
		// Operation completed, return final status
		return m.getStatus(), nil
	case <-ctx.Done():
		// Timeout
		return m.getStatus(), fmt.Errorf("timeout after %v waiting for status change", timeout)
	}
}

// Close cleans up resources and closes channels
func (m *AutoIndexingManager) Close() {
	// Use atomic compare-and-swap to prevent double-close
	if !atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		return // Already closed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Close channels to prevent goroutine leaks
	close(m.statusChan)
	close(m.doneChan)
}
