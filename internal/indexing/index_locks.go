package indexing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/standardbeagle/lci/internal/core"
)

// IndexLockManager provides high-level utilities for managing index locks
type IndexLockManager struct {
	coordinator core.IndexCoordinator
	config      *IndexLockConfig
	mu          sync.RWMutex
}

// IndexLockConfig configures index lock management behavior
type IndexLockConfig struct {
	DefaultReadTimeout  time.Duration
	DefaultWriteTimeout time.Duration
	MaxRetryAttempts    int
	RetryBackoffFactor  float64
	EnableMetrics       bool
}

// LockAcquisitionResult represents the result of a lock acquisition attempt
type LockAcquisitionResult struct {
	Success   bool
	LockType  core.LockType
	IndexType core.IndexType
	WaitTime  time.Duration
	LockID    string
	Error     error
	Metrics   *LockMetrics
}

// LockMetrics provides metrics for lock operations
type LockMetrics struct {
	AcquisitionAttempts    int64
	SuccessfulAcquisitions int64
	FailedAcquisitions     int64
	AverageWaitTime        time.Duration
	MaxWaitTime            time.Duration
	ContentionRate         float64
}

// NewIndexLockManager creates a new index lock manager
func NewIndexLockManager(coordinator core.IndexCoordinator) *IndexLockManager {
	config := DefaultIndexLockConfig()

	return &IndexLockManager{
		coordinator: coordinator,
		config:      config,
	}
}

// NewIndexLockManagerWithConfig creates a manager with custom configuration
func NewIndexLockManagerWithConfig(coordinator core.IndexCoordinator, config *IndexLockConfig) *IndexLockManager {
	return &IndexLockManager{
		coordinator: coordinator,
		config:      config,
	}
}

// DefaultIndexLockConfig returns default configuration
func DefaultIndexLockConfig() *IndexLockConfig {
	return &IndexLockConfig{
		DefaultReadTimeout:  5 * time.Second,
		DefaultWriteTimeout: 30 * time.Second,
		MaxRetryAttempts:    3,
		RetryBackoffFactor:  2.0,
		EnableMetrics:       true,
	}
}

// AcquireReadLock acquires a read lock for the specified index type with retry logic
func (m *IndexLockManager) AcquireReadLock(indexType core.IndexType) (core.LockRelease, error) {
	return m.AcquireReadLockWithTimeout(indexType, m.config.DefaultReadTimeout)
}

// AcquireReadLockWithTimeout acquires a read lock with custom timeout
func (m *IndexLockManager) AcquireReadLockWithTimeout(indexType core.IndexType, timeout time.Duration) (core.LockRelease, error) {
	return m.acquireLockWithRetry(indexType, false, timeout)
}

// AcquireWriteLock acquires a write lock for the specified index type with retry logic
func (m *IndexLockManager) AcquireWriteLock(indexType core.IndexType) (core.LockRelease, error) {
	return m.AcquireWriteLockWithTimeout(indexType, m.config.DefaultWriteTimeout)
}

// AcquireWriteLockWithTimeout acquires a write lock with custom timeout
func (m *IndexLockManager) AcquireWriteLockWithTimeout(indexType core.IndexType, timeout time.Duration) (core.LockRelease, error) {
	return m.acquireLockWithRetry(indexType, true, timeout)
}

// acquireLockWithRetry attempts to acquire a lock with retry logic
func (m *IndexLockManager) acquireLockWithRetry(indexType core.IndexType, forWriting bool, timeout time.Duration) (core.LockRelease, error) {
	var lastError error
	backoff := time.Millisecond

	for attempt := 0; attempt < m.config.MaxRetryAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * m.config.RetryBackoffFactor)
		}

		release, err := m.coordinator.AcquireIndexLock(indexType, forWriting, timeout)
		if err == nil {
			return release, nil
		}

		lastError = err

		// Check if we should retry
		if !m.shouldRetry(err) {
			break
		}
	}

	return nil, fmt.Errorf("failed to acquire lock for %s after %d attempts: %w", indexType.String(), m.config.MaxRetryAttempts, lastError)
}

// shouldRetry determines if a lock acquisition error should be retried
func (m *IndexLockManager) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check error types that should be retried
	errStr := err.Error()

	// Retry on timeout errors
	if contains(errStr, "timeout") || contains(errStr, "deadline exceeded") {
		return true
	}

	// Retry on temporary unavailability
	if contains(errStr, "unavailable") || contains(errStr, "busy") {
		return true
	}

	// Don't retry on permanent errors
	if contains(errStr, "invalid") || contains(errStr, "not found") {
		return false
	}

	return true
}

// AcquireMultipleReadLocks acquires read locks for multiple index types
func (m *IndexLockManager) AcquireMultipleReadLocks(indexTypes []core.IndexType) (core.LockRelease, error) {
	return m.AcquireMultipleReadLocksWithTimeout(indexTypes, m.config.DefaultReadTimeout)
}

// AcquireMultipleReadLocksWithTimeout acquires read locks for multiple index types with custom timeout
func (m *IndexLockManager) AcquireMultipleReadLocksWithTimeout(indexTypes []core.IndexType, timeout time.Duration) (core.LockRelease, error) {
	requirements := m.buildSearchRequirements(indexTypes)
	return m.coordinator.AcquireMultipleLocks(requirements, false, timeout)
}

// AcquireMultipleWriteLocks acquires write locks for multiple index types
func (m *IndexLockManager) AcquireMultipleWriteLocks(indexTypes []core.IndexType) (core.LockRelease, error) {
	return m.AcquireMultipleWriteLocksWithTimeout(indexTypes, m.config.DefaultWriteTimeout)
}

// AcquireMultipleWriteLocksWithTimeout acquires write locks for multiple index types with custom timeout
func (m *IndexLockManager) AcquireMultipleWriteLocksWithTimeout(indexTypes []core.IndexType, timeout time.Duration) (core.LockRelease, error) {
	return m.acquireMultipleLocksWithRetry(indexTypes, true, timeout)
}

// acquireMultipleLocksWithRetry attempts to acquire multiple locks with retry logic
func (m *IndexLockManager) acquireMultipleLocksWithRetry(indexTypes []core.IndexType, forWriting bool, timeout time.Duration) (core.LockRelease, error) {
	var lastError error
	backoff := time.Millisecond

	for attempt := 0; attempt < m.config.MaxRetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * m.config.RetryBackoffFactor)
		}

		if forWriting {
			// For write locks, acquire them individually to maintain control
			releases, err := m.acquireIndividualLocks(indexTypes, true, timeout)
			if err == nil {
				return m.combineReleases(releases), nil
			}
			lastError = err
		} else {
			// For read locks, use the coordinator's batch acquisition
			requirements := m.buildSearchRequirements(indexTypes)
			release, err := m.coordinator.AcquireMultipleLocks(requirements, false, timeout)
			if err == nil {
				return release, nil
			}
			lastError = err
		}

		if !m.shouldRetry(lastError) {
			break
		}
	}

	return nil, fmt.Errorf("failed to acquire multiple locks after %d attempts: %w", m.config.MaxRetryAttempts, lastError)
}

// acquireIndividualLocks acquires locks individually for write operations
func (m *IndexLockManager) acquireIndividualLocks(indexTypes []core.IndexType, forWriting bool, timeout time.Duration) ([]core.LockRelease, error) {
	var releases []core.LockRelease
	var acquired []core.IndexType

	// Sort index types for consistent ordering
	sortedTypes := m.sortIndexTypes(indexTypes)

	for _, indexType := range sortedTypes {
		release, err := m.coordinator.AcquireIndexLock(indexType, forWriting, timeout)
		if err != nil {
			// Release any already acquired locks
			for _, r := range releases {
				r()
			}
			return nil, fmt.Errorf("failed to acquire lock for %s (acquired: %v): %w", indexType.String(), acquired, err)
		}
		releases = append(releases, release)
		acquired = append(acquired, indexType)
	}

	return releases, nil
}

// combineReleases combines multiple release functions into one
func (m *IndexLockManager) combineReleases(releases []core.LockRelease) core.LockRelease {
	return func() {
		// Release in reverse order to prevent potential issues
		for i := len(releases) - 1; i >= 0; i-- {
			if releases[i] != nil {
				releases[i]()
			}
		}
	}
}

// buildSearchRequirements creates SearchRequirements from index types
func (m *IndexLockManager) buildSearchRequirements(indexTypes []core.IndexType) core.SearchRequirements {
	requirements := core.NewSearchRequirements()

	for _, indexType := range indexTypes {
		switch indexType {
		case core.TrigramIndexType:
			requirements.NeedsTrigrams = true
		case core.SymbolIndexType:
			requirements.NeedsSymbols = true
		case core.ReferenceIndexType:
			requirements.NeedsReferences = true
		case core.CallGraphIndexType:
			requirements.NeedsCallGraph = true
		case core.PostingsIndexType:
			requirements.NeedsPostings = true
		case core.LocationIndexType:
			requirements.NeedsLocations = true
		case core.ContentIndexType:
			requirements.NeedsContent = true
		}
	}

	return requirements
}

// sortIndexTypes sorts index types in consistent order to prevent deadlocks
func (m *IndexLockManager) sortIndexTypes(indexTypes []core.IndexType) []core.IndexType {
	sorted := make([]core.IndexType, len(indexTypes))
	copy(sorted, indexTypes)

	// Simple bubble sort based on IndexType integer values
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// GetLockStatus returns the current status of locks for all index types
func (m *IndexLockManager) GetLockStatus() map[core.IndexType]core.IndexStatus {
	return m.coordinator.GetAllIndexStatus()
}

// WaitForIndexes waits for the specified indexes to become available
func (m *IndexLockManager) WaitForIndexes(indexTypes []core.IndexType, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, indexType := range indexTypes {
		if err := m.coordinator.WaitForIndex(indexType, timeout); err != nil {
			return fmt.Errorf("timeout waiting for index %s: %w", indexType.String(), err)
		}
		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

// EstimateLockWaitTime estimates the expected wait time for a lock
func (m *IndexLockManager) EstimateLockWaitTime(indexType core.IndexType, forWriting bool) time.Duration {
	// Wait time estimation - metrics removed
	return time.Duration(0)
}

// IsLockAvailable checks if a lock is likely to be available without blocking
func (m *IndexLockManager) IsLockAvailable(indexType core.IndexType, forWriting bool) bool {
	status := m.coordinator.GetIndexStatus(indexType)

	// If the index is currently being updated and we need read access, it might block
	if status.IsIndexing && !forWriting {
		return false
	}

	// If there are active lock holders, it might block
	if status.LockHolders > 0 {
		return false
	}

	return true
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr))))
}

// findSubstring implements a simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
