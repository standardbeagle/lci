package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// IndexCoordinator manages coordination between multiple index systems and search operations
type IndexCoordinator interface {
	// Lock acquisition for operations
	AcquireIndexLock(indexType IndexType, forWriting bool, timeout time.Duration) (LockRelease, error)
	AcquireIndexLockCtx(ctx context.Context, indexType IndexType, forWriting bool) (LockRelease, error)
	AcquireMultipleLocks(requirements SearchRequirements, forWriting bool, timeout time.Duration) (LockRelease, error)
	AcquireMultipleLocksCtx(ctx context.Context, requirements SearchRequirements, forWriting bool) (LockRelease, error)

	// Status and monitoring
	GetIndexStatus(indexType IndexType) IndexStatus
	GetAllIndexStatus() map[IndexType]IndexStatus
	WaitForIndex(indexType IndexType, timeout time.Duration) error
}

// DefaultIndexCoordinator implements the IndexCoordinator interface
type DefaultIndexCoordinator struct {
	registry *IndexStateRegistry
	config   *IndexCoordinationConfig
	mu       sync.RWMutex
}

// NewIndexCoordinator creates a new index coordinator
func NewIndexCoordinator() IndexCoordinator {
	return NewIndexCoordinatorWithConfig(DefaultIndexCoordinationConfig())
}

// NewIndexCoordinatorWithConfig creates a new index coordinator with custom configuration
func NewIndexCoordinatorWithConfig(config *IndexCoordinationConfig) IndexCoordinator {
	registry := NewIndexStateRegistry()

	coordinator := &DefaultIndexCoordinator{
		registry: registry,
		config:   config,
	}

	return coordinator
}

// AcquireIndexLock acquires a lock for a specific index type
func (c *DefaultIndexCoordinator) AcquireIndexLock(indexType IndexType, forWriting bool, timeout time.Duration) (LockRelease, error) {
	if !indexType.IsValid() {
		return nil, NewInvalidIndexTypeError(indexType)
	}

	// Apply adaptive timeout adjustment if enabled
	adjustedTimeout := c.calculateAdaptiveTimeout(indexType, forWriting, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), adjustedTimeout)
	defer cancel()

	return c.AcquireIndexLockCtx(ctx, indexType, forWriting)
}

// AcquireIndexLockCtx acquires a lock for a specific index type with context
func (c *DefaultIndexCoordinator) AcquireIndexLockCtx(ctx context.Context, indexType IndexType, forWriting bool) (LockRelease, error) {
	state := c.registry.GetIndexState(indexType)
	start := time.Now()

	var release LockRelease
	var err error

	if forWriting {
		release, err = state.AcquireWriteLock(ctx, c.config.DefaultLockTimeout)
	} else {
		release, err = state.AcquireReadLock(ctx, c.config.DefaultLockTimeout)
	}

	waitTime := time.Since(start)

	// Log lock operation with enhanced details
	lockType := WriteLock
	if !forWriting {
		lockType = ReadLock
	}
	status := c.registry.GetIndexState(indexType).GetStatus()
	LogLockOperation(indexType, lockType, "acquire", waitTime, err == nil, status.QueueDepth)

	return release, err
}

// AcquireMultipleLocks acquires locks for multiple index types based on search requirements
func (c *DefaultIndexCoordinator) AcquireMultipleLocks(requirements SearchRequirements, forWriting bool, timeout time.Duration) (LockRelease, error) {
	// Apply adaptive timeout adjustment for multi-lock operations
	adjustedTimeout := c.calculateMultiLockAdaptiveTimeout(requirements, forWriting, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), adjustedTimeout)
	defer cancel()

	return c.AcquireMultipleLocksCtx(ctx, requirements, forWriting)
}

// AcquireMultipleLocksCtx acquires locks for multiple index types with context
func (c *DefaultIndexCoordinator) AcquireMultipleLocksCtx(ctx context.Context, requirements SearchRequirements, forWriting bool) (LockRelease, error) {
	// Validate requirements
	if err := requirements.Validate(); err != nil {
		return nil, fmt.Errorf("invalid search requirements: %w", err)
	}

	requiredIndexes := requirements.GetRequiredIndexTypes()
	if len(requiredIndexes) == 0 {
		// No locks needed
		return func() {}, nil
	}

	// Acquire locks in consistent order to prevent deadlocks
	sortedIndexes := c.sortIndexTypes(requiredIndexes)

	var releases []LockRelease
	var acquiredIndexes []IndexType

	// Acquire locks one by one
	for _, indexType := range sortedIndexes {
		release, err := c.AcquireIndexLockCtx(ctx, indexType, forWriting)
		if err != nil {
			// Release any already acquired locks
			for _, r := range releases {
				r()
			}

			// Create detailed error
			coordErr := NewLockUnavailableError(indexType, fmt.Sprintf("failed to acquire lock for %s (acquired: %v)", indexType.String(), acquiredIndexes))
			return nil, coordErr
		}

		releases = append(releases, release)
		acquiredIndexes = append(acquiredIndexes, indexType)
	}

	// Return combined release function
	return func() {
		// Release in reverse order
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}, nil
}

// GetIndexStatus returns the current status of a specific index
func (c *DefaultIndexCoordinator) GetIndexStatus(indexType IndexType) IndexStatus {
	state := c.registry.GetIndexState(indexType)
	return state.GetStatus()
}

// GetAllIndexStatus returns the status of all indexes
func (c *DefaultIndexCoordinator) GetAllIndexStatus() map[IndexType]IndexStatus {
	return c.registry.GetAllStatus()
}

// WaitForIndex waits until a specific index is not indexing or timeout occurs
func (c *DefaultIndexCoordinator) WaitForIndex(indexType IndexType, timeout time.Duration) error {
	// Apply adaptive timeout adjustment for wait operations
	adjustedTimeout := c.calculateWaitAdaptiveTimeout(indexType, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), adjustedTimeout)
	defer cancel()

	state := c.registry.GetIndexState(indexType)
	return state.WaitForIndexUpdate(ctx, adjustedTimeout)
}

// LockOrderingStrategy defines different approaches to lock ordering
type LockOrderingStrategy int

const (
	// StrategyNumeric - Simple numeric ordering (fallback)
	StrategyNumeric LockOrderingStrategy = iota
	// StrategyDependency - Order by dependencies (dependent indexes first)
	StrategyDependency
	// StrategyPriority - Order by priority and contention
	StrategyPriority
	// StrategyAdaptive - Adaptive ordering based on system conditions
	StrategyAdaptive
)

// String returns string representation of LockOrderingStrategy
func (los LockOrderingStrategy) String() string {
	switch los {
	case StrategyNumeric:
		return "Numeric"
	case StrategyDependency:
		return "Dependency"
	case StrategyPriority:
		return "Priority"
	case StrategyAdaptive:
		return "Adaptive"
	default:
		return "UnknownStrategy"
	}
}

// IndexLockInfo contains information about an index for lock ordering decisions
type IndexLockInfo struct {
	IndexType         IndexType
	Priority          int
	ContentionRate    float64
	AverageWaitTime   time.Duration
	CurrentQueueDepth int
	IsIndexing        bool
	Dependencies      []IndexType
	Dependents        []IndexType
}

// sortIndexTypes sorts index types using intelligent deadlock prevention strategies
func (c *DefaultIndexCoordinator) sortIndexTypes(indexTypes []IndexType) []IndexType {
	if len(indexTypes) <= 1 {
		return indexTypes
	}

	// Get current lock ordering strategy
	strategy := c.getLockOrderingStrategy()

	switch strategy {
	case StrategyDependency:
		return c.sortByDependencies(indexTypes)
	case StrategyPriority:
		return c.sortByPriority(indexTypes)
	case StrategyAdaptive:
		return c.sortAdaptively(indexTypes)
	default:
		return c.sortNumerically(indexTypes)
	}
}

// getLockOrderingStrategy determines the current lock ordering strategy
func (c *DefaultIndexCoordinator) getLockOrderingStrategy() LockOrderingStrategy {
	// Check if adaptive ordering is enabled in config
	if c.config != nil && c.config.AdaptiveTimeout {
		return StrategyPriority
	}
	return StrategyNumeric
}

// sortNumerically performs simple numeric ordering (original behavior)
func (c *DefaultIndexCoordinator) sortNumerically(indexTypes []IndexType) []IndexType {
	sorted := make([]IndexType, len(indexTypes))
	copy(sorted, indexTypes)

	// T051-P1: Replace O(n²) bubble sort with efficient O(n log n) sort
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	return sorted
}

// sortByDependencies orders indexes to minimize dependency-based deadlocks
func (c *DefaultIndexCoordinator) sortByDependencies(indexTypes []IndexType) []IndexType {
	// Get dependency information for all indexes
	indexInfos := make(map[IndexType]*IndexLockInfo)
	for _, indexType := range indexTypes {
		indexInfos[indexType] = c.getIndexLockInfo(indexType)
	}

	// Perform topological sort based on dependencies
	sorted := make([]IndexType, 0, len(indexTypes))
	visited := make(map[IndexType]bool)
	temporary := make(map[IndexType]bool)

	var visit func(indexType IndexType)
	visit = func(indexType IndexType) {
		if temporary[indexType] {
			// Circular dependency detected - fall back to numeric ordering
			return
		}
		if visited[indexType] {
			return
		}

		temporary[indexType] = true

		// Visit dependencies first
		info := indexInfos[indexType]
		for _, dep := range info.Dependencies {
			if indexInfos[dep] != nil {
				visit(dep)
			}
		}

		temporary[indexType] = false
		visited[indexType] = true
		sorted = append(sorted, indexType)
	}

	// Visit all indexes
	for _, indexType := range indexTypes {
		if !visited[indexType] {
			visit(indexType)
		}
	}

	// If circular dependencies prevented proper sorting, fall back to numeric
	if len(sorted) != len(indexTypes) {
		return c.sortNumerically(indexTypes)
	}

	return sorted
}

// sortByPriority orders indexes by priority and contention levels
func (c *DefaultIndexCoordinator) sortByPriority(indexTypes []IndexType) []IndexType {
	sorted := make([]IndexType, len(indexTypes))
	copy(sorted, indexTypes)

	// Sort by priority (higher priority first), then by contention (lower contention first)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			infoI := c.getIndexLockInfo(sorted[i])
			infoJ := c.getIndexLockInfo(sorted[j])

			// Compare by priority first
			if infoI.Priority != infoJ.Priority {
				if infoI.Priority < infoJ.Priority {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
				continue
			}

			// Then by contention rate (lower contention first)
			if infoI.ContentionRate != infoJ.ContentionRate {
				if infoI.ContentionRate > infoJ.ContentionRate {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
				continue
			}

			// Finally by numeric order as tiebreaker
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// sortAdaptively orders indexes based on current system conditions
func (c *DefaultIndexCoordinator) sortAdaptively(indexTypes []IndexType) []IndexType {
	// Get index information
	indexInfos := make([]*IndexLockInfo, 0, len(indexTypes))
	for _, indexType := range indexTypes {
		info := c.getIndexLockInfo(indexType)
		indexInfos = append(indexInfos, info)
	}

	// Use hybrid dependency + priority approach
	for i := 0; i < len(indexInfos); i++ {
		for j := i + 1; j < len(indexInfos); j++ {
			infoI := indexInfos[i]
			infoJ := indexInfos[j]

			// Check dependency relationship
			iDependsOnJ := c.hasDependency(infoI.IndexType, infoJ.IndexType)
			jDependsOnI := c.hasDependency(infoJ.IndexType, infoI.IndexType)

			if iDependsOnJ && !jDependsOnI {
				// I depends on J, so J should come first
				indexInfos[i], indexInfos[j] = indexInfos[j], indexInfos[i]
				continue
			}
			if jDependsOnI && !iDependsOnJ {
				// J depends on I, so I should come first (already in order)
				continue
			}

			// No dependency relationship, use priority as tiebreaker
			if infoI.Priority != infoJ.Priority {
				if infoI.Priority < infoJ.Priority {
					indexInfos[i], indexInfos[j] = indexInfos[j], indexInfos[i]
				}
			}
		}
	}

	// Extract sorted index types
	result := make([]IndexType, len(indexInfos))
	for i, info := range indexInfos {
		result[i] = info.IndexType
	}

	return result
}

// getIndexLockInfo retrieves lock information for an index type
func (c *DefaultIndexCoordinator) getIndexLockInfo(indexType IndexType) *IndexLockInfo {
	status := c.registry.GetIndexState(indexType).GetStatus()

	return &IndexLockInfo{
		IndexType:         indexType,
		Priority:          c.getIndexPriority(indexType),
		ContentionRate:    0.0,
		AverageWaitTime:   0,
		CurrentQueueDepth: status.QueueDepth,
		IsIndexing:        status.IsIndexing,
		Dependencies:      c.getIndexDependencies(indexType),
		Dependents:        c.getIndexDependents(indexType),
	}
}

// getIndexPriority returns the priority for an index type
func (c *DefaultIndexCoordinator) getIndexPriority(indexType IndexType) int {
	// Define base priorities for different index types
	// Lower numbers = higher priority
	switch indexType {
	case TrigramIndexType:
		return 1 // Highest priority - most commonly used
	case SymbolIndexType:
		return 2 // High priority - essential for semantic search
	case LocationIndexType:
		return 3 // Medium-high priority
	case ReferenceIndexType:
		return 4 // Medium priority
	case PostingsIndexType:
		return 5 // Medium-low priority
	case CallGraphIndexType:
		return 6 // Low priority - specialized use
	case ContentIndexType:
		return 7 // Lowest priority - fallback
	default:
		return 10 // Unknown indexes get lowest priority
	}
}

// getIndexDependencies returns the dependencies for an index type
func (c *DefaultIndexCoordinator) getIndexDependencies(indexType IndexType) []IndexType {
	// Define dependency relationships
	switch indexType {
	case CallGraphIndexType:
		return []IndexType{SymbolIndexType} // Call graph depends on symbols
	case ReferenceIndexType:
		return []IndexType{SymbolIndexType} // References depend on symbols
	case PostingsIndexType:
		return []IndexType{TrigramIndexType} // Postings depend on trigrams
	default:
		return nil // No dependencies
	}
}

// getIndexDependents returns indexes that depend on this index type
func (c *DefaultIndexCoordinator) getIndexDependents(indexType IndexType) []IndexType {
	// Reverse dependency lookup
	switch indexType {
	case SymbolIndexType:
		return []IndexType{CallGraphIndexType, ReferenceIndexType}
	case TrigramIndexType:
		return []IndexType{PostingsIndexType}
	default:
		return nil
	}
}

// hasDependency checks if one index type depends on another
func (c *DefaultIndexCoordinator) hasDependency(indexType, potentialDep IndexType) bool {
	deps := c.getIndexDependencies(indexType)
	for _, dep := range deps {
		if dep == potentialDep {
			return true
		}
	}
	return false
}

// SetConfig updates the coordinator configuration
func (c *DefaultIndexCoordinator) SetConfig(config *IndexCoordinationConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config = config
}

// GetConfig returns the current configuration
func (c *DefaultIndexCoordinator) GetConfig() *IndexCoordinationConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Close cleans up coordinator resources
func (c *DefaultIndexCoordinator) Close() error {
	return nil
}

// T032: Independent Index State Management Methods

// GetIndexHealth returns the health status of a specific index
func (c *DefaultIndexCoordinator) GetIndexHealth(indexType IndexType) IndexHealth {
	state := c.registry.GetIndexState(indexType)
	status := state.GetStatus()

	// Calculate health metrics
	health := IndexHealth{
		IndexType:    indexType,
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		LastUpdate:   status.LastUpdate,
		UpdateCount:  status.UpdateCount,
		ErrorCount:   0,     // Will be set if errors exist
		Availability: 100.0, // Percentage
		IsAvailable:  true,
		Performance: IndexPerformance{
			AverageResponseTime: 0, // Future: Calculate from metrics
			LockWaitTime:        0,
			ContentionRate:      0.0,
		},
		ErrorMessage:  "",
		QueueSize:     status.QueueDepth,
		OperationRate: 0.0,
		ErrorRate:     0.0,
		ResponseTime:  0,
		LastError:     nil,
		LastErrorTime: time.Time{},
	}

	// Check if index is in a healthy state
	if status.IsIndexing {
		health.Status = HealthStatusDegraded
		health.Availability = 0.0
		health.ErrorMessage = "Index currently being updated"
	} else if status.LockHolders > 0 {
		health.Status = HealthStatusDegraded
		health.Availability = 50.0
		health.ErrorMessage = "Index locked for operations"
	}

	// Check for recent errors
	if lastErr, errTime := state.GetLastError(); lastErr != nil {
		health.Status = HealthStatusUnhealthy
		health.LastError = lastErr
		health.LastErrorTime = errTime
		health.ErrorCount = 1 // At least one error occurred
		health.Availability = 0.0
		health.IsAvailable = false
		health.ErrorMessage = lastErr.Error()
	}

	return health
}

// GetAllIndexHealth returns the health status of all indexes
func (c *DefaultIndexCoordinator) GetAllIndexHealth() map[IndexType]IndexHealth {
	allHealth := make(map[IndexType]IndexHealth)

	for _, indexType := range GetAllIndexTypes() {
		allHealth[indexType] = c.GetIndexHealth(indexType)
	}

	return allHealth
}

// Helper methods for adaptive timeout calculations
func (c *DefaultIndexCoordinator) calculateAdaptiveTimeout(indexType IndexType, forWriting bool, timeout time.Duration) time.Duration {
	if timeout == 0 {
		timeout = c.config.DefaultLockTimeout
	}
	return timeout
}

func (c *DefaultIndexCoordinator) calculateMultiLockAdaptiveTimeout(requirements SearchRequirements, forWriting bool, timeout time.Duration) time.Duration {
	if timeout == 0 {
		timeout = c.config.DefaultLockTimeout
	}
	// Add extra time for multiple locks
	return timeout + (100 * time.Millisecond)
}

func (c *DefaultIndexCoordinator) calculateWaitAdaptiveTimeout(indexType IndexType, timeout time.Duration) time.Duration {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return timeout
}
