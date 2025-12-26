package search

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

// SearchCoordinator coordinates search operations with the index coordinator
// to enable concurrent searches while indexing is in progress
type SearchCoordinator struct {
	indexCoordinator core.IndexCoordinator
	config           *SearchCoordinatorConfig
	// T049: Search operation queuing and prioritization
	searchQueue        *PriorityQueue
	queueMutex         sync.RWMutex
	runningSearches    int64
	queueMetrics       *QueueMetrics
	starvationTracker  *StarvationTracker
	fairShareScheduler *FairShareScheduler
	// T052: Incremental search capability updates
	indexCompletionListeners map[core.IndexType][]chan core.IndexType
	capabilityMutex          sync.RWMutex
	lastCapabilityUpdate     time.Time
	incrementalMetrics       *IncrementalSearchMetrics
}

// SearchCoordinatorConfig configures search coordination behavior
type SearchCoordinatorConfig struct {
	DefaultSearchTimeout  time.Duration
	MaxConcurrentSearches int
	EnableSearchMetrics   bool
	RetryFailedSearches   bool
	MaxSearchRetries      int
	// T035: Graceful degradation configuration
	EnableGracefulDegradation bool
	PartialSearchThreshold    float64 // Minimum percentage of required indexes available (0.0-1.0)
	DegradedSearchTimeout     time.Duration
	FallbackSearchEnabled     bool
	IndexPriorityOrder        []core.IndexType // Priority order for index selection
	// T049: Search operation queuing and prioritization
	EnableSearchQueueing bool
	MaxQueueSize         int
	PriorityTimeout      time.Duration
	QueueTimeoutScale    float64
	FairShareEnabled     bool
	StarvationPrevention bool
}

// SearchResult contains the results of a coordinated search operation
type SearchResult struct {
	Results            []searchtypes.Result
	WaitTime           time.Duration
	LocksUsed          []core.IndexType
	UnavailableIndexes []core.IndexType // T035: Indexes that were unavailable
	DegradedMode       bool             // T035: Whether search ran in degraded mode
	PartialResults     bool             // T035: Whether results are partial due to missing indexes
	Metrics            *SearchMetrics
	Error              error
}

// SearchMetrics provides comprehensive metrics for search operations and coordination efficiency
type SearchMetrics struct {
	// Basic search metrics
	SearchAttempts     int64
	SuccessfulSearches int64
	FailedSearches     int64
	AverageSearchTime  time.Duration
	MaxSearchTime      time.Duration
	ContentionEvents   int64
	LockWaitTime       time.Duration

	// T055: Coordination efficiency metrics
	OverallEfficiency     float64 `json:"overall_efficiency"`     // Overall coordination efficiency score (0-100%)
	LockEfficiency        float64 `json:"lock_efficiency"`        // Lock acquisition efficiency
	SearchSuccessRate     float64 `json:"search_success_rate"`    // Search operation success rate
	IndexAvailability     float64 `json:"index_availability"`     // Average index availability
	PriorityAdherence     float64 `json:"priority_adherence"`     // Priority scheduling adherence
	QueueEfficiency       float64 `json:"queue_efficiency"`       // Operation queuing efficiency
	DeadlockPreventions   int64   `json:"deadlock_preventions"`   // Number of deadlocks prevented
	StarvationPreventions int64   `json:"starvation_preventions"` // Number of starvation events prevented
	TotalConcurrentOps    int64   `json:"total_concurrent_ops"`   // Total concurrent operations processed

	// Queue metrics
	CurrentQueueSize int64         `json:"current_queue_size"` // Current queue size
	AverageQueueWait time.Duration `json:"average_queue_wait"` // Average queue wait time

	// T052: Incremental search metrics
	CapabilityUpdates     int64                  `json:"capability_updates"`      // Total capability updates
	IndexCompletions      int64                  `json:"index_completions"`       // Total index completion events
	AverageCapabilityGain float64                `json:"average_capability_gain"` // Average capability gain from index completions
	CurrentCapabilities   map[string]interface{} `json:"current_capabilities"`    // Current search capabilities

	// T050: Search coordination metrics
	SearchOperations      int64   `json:"search_operations"`       // Total search operations coordinated
	IndexOperations       int64   `json:"index_operations"`        // Total indexing operations coordinated
	DegradedSearches      int64   `json:"degraded_searches"`       // Number of degraded search operations
	FallbackSearches      int64   `json:"fallback_searches"`       // Number of fallback search operations
	IndexAvailabilityRate float64 `json:"index_availability_rate"` // Current index availability rate
}

// NewSearchCoordinator creates a new search coordinator
func NewSearchCoordinator(indexCoordinator core.IndexCoordinator) *SearchCoordinator {
	config := DefaultSearchCoordinatorConfig()

	return &SearchCoordinator{
		indexCoordinator:         indexCoordinator,
		config:                   config,
		searchQueue:              NewPriorityQueue(config.MaxQueueSize),
		queueMetrics:             NewQueueMetrics(),
		starvationTracker:        NewStarvationTracker(config),
		fairShareScheduler:       NewFairShareScheduler(config),
		indexCompletionListeners: make(map[core.IndexType][]chan core.IndexType),
		incrementalMetrics:       NewIncrementalSearchMetrics(),
	}
}

// NewSearchCoordinatorWithConfig creates a coordinator with custom configuration
func NewSearchCoordinatorWithConfig(indexCoordinator core.IndexCoordinator, config *SearchCoordinatorConfig) *SearchCoordinator {
	return &SearchCoordinator{
		indexCoordinator:         indexCoordinator,
		config:                   config,
		searchQueue:              NewPriorityQueue(config.MaxQueueSize),
		queueMetrics:             NewQueueMetrics(),
		starvationTracker:        NewStarvationTracker(config),
		fairShareScheduler:       NewFairShareScheduler(config),
		indexCompletionListeners: make(map[core.IndexType][]chan core.IndexType),
		incrementalMetrics:       NewIncrementalSearchMetrics(),
	}
}

// DefaultSearchCoordinatorConfig returns default configuration
func DefaultSearchCoordinatorConfig() *SearchCoordinatorConfig {
	return &SearchCoordinatorConfig{
		DefaultSearchTimeout:  10 * time.Second,
		MaxConcurrentSearches: 1000,
		EnableSearchMetrics:   true,
		RetryFailedSearches:   true,
		MaxSearchRetries:      3,
		// T035: Graceful degradation defaults
		EnableGracefulDegradation: true,
		PartialSearchThreshold:    0.5, // Require at least 50% of indexes
		DegradedSearchTimeout:     5 * time.Second,
		FallbackSearchEnabled:     true,
		IndexPriorityOrder: []core.IndexType{
			core.TrigramIndexType,   // Most important for basic pattern matching
			core.SymbolIndexType,    // Important for symbol analysis
			core.ReferenceIndexType, // Important for reference tracking
			core.PostingsIndexType,  // Important for context
			core.LocationIndexType,  // Important for file filtering
			core.ContentIndexType,   // Important for full content search
			core.CallGraphIndexType, // Least critical, can be omitted
		},
		// T049: Search operation queuing and prioritization defaults
		EnableSearchQueueing: true,
		MaxQueueSize:         500,
		PriorityTimeout:      30 * time.Second,
		QueueTimeoutScale:    1.5,
		FairShareEnabled:     true,
		StarvationPrevention: true,
	}
}

// Search executes a search with lock coordination and retry logic
func (sc *SearchCoordinator) Search(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.Result, error),
) (*SearchResult, error) {
	return sc.searchWithRetry(ctx, pattern, options, searchFunc, false)
}

// SearchDetailed executes a detailed search with lock coordination and retry logic
func (sc *SearchCoordinator) SearchDetailed(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.DetailedResult, error),
) (*SearchResult, error) {
	// Create a wrapper function that converts detailed results to basic
	wrappedFunc := func(ctx context.Context, pattern string, opts types.SearchOptions) ([]searchtypes.Result, error) {
		detailedResults, err := searchFunc(ctx, pattern, opts)
		if err != nil {
			return nil, err
		}
		return sc.convertDetailedToBasic(detailedResults), nil
	}

	// Use the shared retry logic
	return sc.searchWithRetry(ctx, pattern, options, wrappedFunc, true)
}

// searchWithRetry implements the core retry logic for search operations
// @lci:loop-bounded[3]
// @lci:call-frequency[hot-path]
func (sc *SearchCoordinator) searchWithRetry(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.Result, error),
	isDetailed bool,
) (*SearchResult, error) {
	// Pre-compute operation type strings to avoid allocations in hot path
	searchType := "basic"
	if isDetailed {
		searchType = "detailed"
	}

	// Pre-compute all operation type strings once (avoid repeated string concatenation)
	opStart := searchType + "_search_start"
	opRequirements := searchType + "_search_requirements"
	opDirect := searchType + "_search_direct"
	opDirectError := searchType + "_search_direct_error"
	opDirectSuccess := searchType + "_search_direct_success"
	opRetryStart := searchType + "_search_retry_start"
	opRetryAttempt := searchType + "_search_retry_attempt"
	opRetryDelay := searchType + "_search_retry_delay"
	opRetryDelayComplete := searchType + "_search_retry_delay_complete"
	opCancelled := searchType + "_search_cancelled"
	opLocksAcquireStart := searchType + "_search_locks_acquire_start"
	opLocksFailedFinal := searchType + "_search_locks_failed_final"
	opLocksFailedRetryable := searchType + "_search_locks_failed_retryable"
	opLocksAcquired := searchType + "_search_locks_acquired"
	opExecuteStart := searchType + "_search_execute_start"
	opLocksReleased := searchType + "_search_locks_released"
	opExecuteFailedFinal := searchType + "_search_execute_failed_final"
	opExecuteFailedRetryable := searchType + "_search_execute_failed_retryable"
	opSuccess := searchType + "_search_success"
	opUnexpectedFailure := searchType + "_search_unexpected_failure"

	// Check if logging is enabled to avoid fmt.Sprintf allocations
	logEnabled := core.CoordinationInfoEnabled()

	// Log search start
	if logEnabled {
		core.LogCoordinationInfo(fmt.Sprintf("Starting %s search", searchType), core.ErrorContext{
			OperationType: opStart,
		})
	}

	// Determine required index types based on search options
	requiredIndexes := sc.determineRequiredIndexes(options)

	// Log index requirements
	if logEnabled {
		core.LogCoordinationInfo(fmt.Sprintf("Search requires %d index types: %v", len(requiredIndexes), requiredIndexes), core.ErrorContext{
			OperationType: opRequirements,
		})
	}

	if len(requiredIndexes) == 0 {
		// No index locks needed, execute search directly
		if logEnabled {
			core.LogCoordinationInfo("Executing search without locks", core.ErrorContext{
				OperationType: opDirect,
			})
		}

		start := time.Now()
		results, err := searchFunc(ctx, pattern, options)
		waitTime := time.Since(start)

		// Log search completion
		if err != nil {
			core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
				core.ErrCodeIndexUnavailable,
				"Direct search failed",
				fmt.Sprintf("Pattern: %s, Error: %v", pattern, err),
			).WithContext(core.ErrorContext{
				OperationType: opDirectError,
				WaitTime:      waitTime,
			}))
		} else if logEnabled {
			core.LogCoordinationInfo(fmt.Sprintf("Direct search completed in %v, %d results", waitTime, len(results)), core.ErrorContext{
				OperationType: opDirectSuccess,
				WaitTime:      waitTime,
			})
		}

		return &SearchResult{
			Results:   results,
			WaitTime:  waitTime,
			LocksUsed: []core.IndexType{},
			Error:     err,
		}, nil
	}

	// Attempt search with retry logic
	var lastError error
	maxAttempts := 1
	if sc.config.RetryFailedSearches {
		maxAttempts = sc.config.MaxSearchRetries + 1
	}

	if logEnabled {
		core.LogCoordinationInfo(fmt.Sprintf("Starting search with up to %d attempts", maxAttempts), core.ErrorContext{
			OperationType: opRetryStart,
		})
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Log attempt start (only on retries)
		if attempt > 0 {
			if logEnabled {
				core.LogCoordinationInfo(fmt.Sprintf("Retry attempt %d/%d", attempt+1, maxAttempts), core.ErrorContext{
					OperationType: opRetryAttempt,
					ConcurrentOps: attempt + 1,
				})
			}
			// Calculate retry delay
			retryDelay := sc.calculateRetryDelay(lastError, attempt-1)
			if logEnabled {
				core.LogCoordinationInfo(fmt.Sprintf("Waiting %v before retry attempt", retryDelay), core.ErrorContext{
					OperationType: opRetryDelay,
					WaitTime:      retryDelay,
					ConcurrentOps: attempt + 1,
				})
			}

			select {
			case <-time.After(retryDelay):
				// Continue with retry
				if logEnabled {
					core.LogCoordinationInfo("Retry delay completed, proceeding with attempt", core.ErrorContext{
						OperationType: opRetryDelayComplete,
						ConcurrentOps: attempt + 1,
					})
				}
			case <-ctx.Done():
				core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
					core.ErrCodeSystemShutdown,
					"Search cancelled during retry wait",
					fmt.Sprintf("Context error: %v", ctx.Err()),
				).WithContext(core.ErrorContext{
					OperationType: opCancelled,
					ConcurrentOps: attempt + 1,
				}))
				return &SearchResult{
					Results:   nil,
					WaitTime:  0,
					LocksUsed: requiredIndexes,
					Error:     fmt.Errorf("search cancelled during retry wait: %w", ctx.Err()),
				}, nil
			}
		}

		// Acquire search locks with retry-aware error handling
		if logEnabled {
			core.LogCoordinationInfo(fmt.Sprintf("Acquiring locks for attempt %d", attempt+1), core.ErrorContext{
				OperationType: opLocksAcquireStart,
				ConcurrentOps: attempt + 1,
			})
		}

		lockStart := time.Now()
		releases, err := sc.acquireSearchLocksWithRetry(ctx, requiredIndexes, attempt)
		lockAcquisitionTime := time.Since(lockStart)

		if err != nil {
			lastError = err
			if !sc.isRetryableError(err) || attempt == maxAttempts-1 {
				// Non-retryable error or final attempt
				core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
					core.ErrCodeLockUnavailable,
					fmt.Sprintf("Failed to acquire search locks after %d attempts", attempt+1),
					fmt.Sprintf("Locks: %v, Error: %v, Wait time: %v", requiredIndexes, err, lockAcquisitionTime),
				).WithContext(core.ErrorContext{
					OperationType: opLocksFailedFinal,
					IndexType:     getIndexTypeFromList(requiredIndexes),
					WaitTime:      lockAcquisitionTime,
					ConcurrentOps: attempt + 1,
				}))
				return &SearchResult{
					Results:   nil,
					WaitTime:  lockAcquisitionTime,
					LocksUsed: requiredIndexes,
					Error:     fmt.Errorf("failed to acquire search locks after %d attempts: %w", attempt+1, err),
				}, nil
			}
			// Continue to next attempt for retryable errors
			core.LogCoordinationWarning(fmt.Sprintf("Lock acquisition failed on attempt %d, retrying: %v", attempt+1, err), core.ErrorContext{
				OperationType: opLocksFailedRetryable,
				IndexType:     getIndexTypeFromList(requiredIndexes),
				WaitTime:      lockAcquisitionTime,
				ConcurrentOps: attempt + 1,
			})
			continue
		}

		// Log successful lock acquisition
		if logEnabled {
			core.LogCoordinationInfo(fmt.Sprintf("Locks acquired in %v for attempt %d", lockAcquisitionTime, attempt+1), core.ErrorContext{
				OperationType: opLocksAcquired,
				IndexType:     getIndexTypeFromList(requiredIndexes),
				WaitTime:      lockAcquisitionTime,
				ConcurrentOps: attempt + 1,
			})
		}

		// Execute search with locks held
		if logEnabled {
			core.LogCoordinationInfo(fmt.Sprintf("Executing search with locks for attempt %d", attempt+1), core.ErrorContext{
				OperationType: opExecuteStart,
				IndexType:     getIndexTypeFromList(requiredIndexes),
				ConcurrentOps: attempt + 1,
			})
		}

		searchStart := time.Now()
		results, err := searchFunc(ctx, pattern, options)
		searchTime := time.Since(searchStart)

		// Always release locks
		releases()
		if logEnabled {
			core.LogCoordinationInfo(fmt.Sprintf("Locks released after %v search execution", searchTime), core.ErrorContext{
				OperationType: opLocksReleased,
				IndexType:     getIndexTypeFromList(requiredIndexes),
				WaitTime:      searchTime,
				ConcurrentOps: attempt + 1,
			})
		}

		// Check for search execution errors
		if err != nil {
			lastError = err
			if !sc.isRetryableError(err) || attempt == maxAttempts-1 {
				// Non-retryable error or final attempt
				core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
					core.ErrCodeIndexUnavailable,
					fmt.Sprintf("Search failed after %d attempts", attempt+1),
					fmt.Sprintf("Pattern: %s, Error: %v, Search time: %v", pattern, err, searchTime),
				).WithContext(core.ErrorContext{
					OperationType: opExecuteFailedFinal,
					IndexType:     getIndexTypeFromList(requiredIndexes),
					WaitTime:      searchTime,
					ConcurrentOps: attempt + 1,
				}))
				return &SearchResult{
					Results:   results,
					WaitTime:  lockAcquisitionTime + searchTime,
					LocksUsed: requiredIndexes,
					Metrics: &SearchMetrics{
						LockWaitTime: lockAcquisitionTime,
						// Add more metrics as needed
					},
					Error: fmt.Errorf("search failed after %d attempts: %w", attempt+1, err),
				}, nil
			}
			// Continue to next attempt for retryable errors
			core.LogCoordinationWarning(fmt.Sprintf("Search execution failed on attempt %d, retrying: %v", attempt+1, err), core.ErrorContext{
				OperationType: opExecuteFailedRetryable,
				IndexType:     getIndexTypeFromList(requiredIndexes),
				WaitTime:      searchTime,
				ConcurrentOps: attempt + 1,
			})
			continue
		}

		// Successful search
		totalTime := lockAcquisitionTime + searchTime
		if logEnabled {
			core.LogCoordinationInfo(fmt.Sprintf("Search completed successfully in %v (lock: %v, search: %v), %d results",
				totalTime, lockAcquisitionTime, searchTime, len(results)), core.ErrorContext{
				OperationType: opSuccess,
				IndexType:     getIndexTypeFromList(requiredIndexes),
				WaitTime:      totalTime,
				ConcurrentOps: attempt + 1,
			})
		}

		return &SearchResult{
			Results:   results,
			WaitTime:  totalTime,
			LocksUsed: requiredIndexes,
			Metrics: &SearchMetrics{
				LockWaitTime: lockAcquisitionTime,
				// Add more metrics as needed
			},
			Error: nil,
		}, nil
	}

	// This should not be reached, but handle gracefully
	core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
		core.ErrCodeSystemShutdown,
		"Search failed unexpectedly after all attempts",
		fmt.Sprintf("Attempts: %d, Last error: %v", maxAttempts, lastError),
	).WithContext(core.ErrorContext{
		OperationType: opUnexpectedFailure,
		IndexType:     getIndexTypeFromList(requiredIndexes),
		ConcurrentOps: maxAttempts,
	}))

	return &SearchResult{
		Results:   nil,
		WaitTime:  0,
		LocksUsed: requiredIndexes,
		Error:     fmt.Errorf("search failed unexpectedly after %d attempts: %w", maxAttempts, lastError),
	}, nil
}

// getIndexTypeFromList returns the first index type from a list for logging purposes
func getIndexTypeFromList(indexTypes []core.IndexType) core.IndexType {
	if len(indexTypes) > 0 {
		return indexTypes[0]
	}
	return core.TrigramIndexType // Default fallback
}

// determineRequiredIndexes determines which index types are needed for a search
func (sc *SearchCoordinator) determineRequiredIndexes(options types.SearchOptions) []core.IndexType {
	var requiredIndexes []core.IndexType

	// Always need trigram index for pattern matching
	requiredIndexes = append(requiredIndexes, core.TrigramIndexType)

	// Check if symbol analysis is needed
	if !options.DeclarationOnly && !options.UsageOnly {
		requiredIndexes = append(requiredIndexes, core.SymbolIndexType)
	}

	// Check if reference tracking is needed
	if options.UsageOnly {
		requiredIndexes = append(requiredIndexes, core.ReferenceIndexType)
	}

	// Call graph analysis (future enhancement)
	// Note: When SearchOptions includes call graph fields, add CallGraphIndexType to required indexes

	// Check if content search is needed
	if options.MaxContextLines > 0 {
		requiredIndexes = append(requiredIndexes, core.PostingsIndexType)
		requiredIndexes = append(requiredIndexes, core.ContentIndexType)
	}

	// Check if file search is needed
	if options.IncludePattern != "" || options.ExcludePattern != "" {
		requiredIndexes = append(requiredIndexes, core.LocationIndexType)
	}

	return requiredIndexes
}

// acquireSearchLocks acquires read locks for the specified index types
func (sc *SearchCoordinator) acquireSearchLocks(ctx context.Context, indexTypes []core.IndexType) (core.LockRelease, error) {
	// Create search requirements
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

	// Acquire locks through the index coordinator
	return sc.indexCoordinator.AcquireMultipleLocksCtx(ctx, requirements, false)
}

// AcquireMultipleLocksCtx acquires multiple locks for search operations with context
func (sc *SearchCoordinator) AcquireMultipleLocksCtx(ctx context.Context, requirements core.SearchRequirements, isWrite bool) (core.LockRelease, error) {
	return sc.acquireSearchLocksWithRetry(ctx, sc.determineIndexTypesFromRequirements(requirements), 0)
}

// determineIndexTypesFromRequirements converts SearchRequirements to IndexType slice
func (sc *SearchCoordinator) determineIndexTypesFromRequirements(requirements core.SearchRequirements) []core.IndexType {
	var indexTypes []core.IndexType

	if requirements.NeedsTrigrams {
		indexTypes = append(indexTypes, core.TrigramIndexType)
	}
	if requirements.NeedsSymbols {
		indexTypes = append(indexTypes, core.SymbolIndexType)
	}
	if requirements.NeedsReferences {
		indexTypes = append(indexTypes, core.ReferenceIndexType)
	}
	if requirements.NeedsCallGraph {
		indexTypes = append(indexTypes, core.CallGraphIndexType)
	}
	if requirements.NeedsPostings {
		indexTypes = append(indexTypes, core.PostingsIndexType)
	}
	if requirements.NeedsLocations {
		indexTypes = append(indexTypes, core.LocationIndexType)
	}
	if requirements.NeedsContent {
		indexTypes = append(indexTypes, core.ContentIndexType)
	}

	return indexTypes
}

// convertDetailedToBasic converts detailed results to basic results
func (sc *SearchCoordinator) convertDetailedToBasic(detailedResults []searchtypes.DetailedResult) []searchtypes.Result {
	results := make([]searchtypes.Result, len(detailedResults))
	for i, detailed := range detailedResults {
		results[i] = detailed.Result
	}
	return results
}

// GetIndexCoordinator returns the underlying index coordinator
func (sc *SearchCoordinator) GetIndexCoordinator() core.IndexCoordinator {
	return sc.indexCoordinator
}

// GetSearchMetrics returns comprehensive search performance and coordination efficiency metrics
// IsSearchAvailable checks if search operations can be performed without blocking
// NOTE: Status functionality was removed, always returning true for now
// Future feature: Reimplement status tracking if needed for coordination metrics
func (sc *SearchCoordinator) IsSearchAvailable(indexTypes []core.IndexType) bool {
	return true
}

// EstimateSearchWaitTime estimates the expected wait time for a search
func (sc *SearchCoordinator) EstimateSearchWaitTime(indexTypes []core.IndexType) time.Duration {
	// Wait time estimation - removed metrics dependency
	// Returns a reasonable default estimate
	return time.Duration(0)
}

// WaitForIndexes waits for the specified indexes to become available for searching
func (sc *SearchCoordinator) WaitForIndexes(indexTypes []core.IndexType, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, indexType := range indexTypes {
		if err := sc.indexCoordinator.WaitForIndex(indexType, timeout); err != nil {
			return fmt.Errorf("timeout waiting for index %s: %w", indexType.String(), err)
		}
		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

// isRetryableError determines if an error is potentially retryable
func (sc *SearchCoordinator) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for coordination errors
	if coordErr, ok := err.(*core.CoordinationError); ok {
		return coordErr.IsRetryable()
	}

	// Check for timeout errors
	if ctxErr, ok := err.(interface{ Timeout() bool }); ok && ctxErr.Timeout() {
		return true
	}

	// Check for deadline exceeded
	errStr := err.Error()
	return strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout")
}

// calculateRetryDelay calculates appropriate retry delay based on error and attempt number
func (sc *SearchCoordinator) calculateRetryDelay(err error, attempt int) time.Duration {
	if coordErr, ok := err.(*core.CoordinationError); ok {
		return core.GetRetryDelay(coordErr, attempt)
	}

	// Default exponential backoff for unknown errors
	baseDelay := 100 * time.Millisecond
	maxDelay := 2 * time.Second

	delay := time.Duration(1<<uint(attempt)) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// acquireSearchLocksWithRetry acquires search locks with retry-aware error handling
func (sc *SearchCoordinator) acquireSearchLocksWithRetry(ctx context.Context, indexTypes []core.IndexType, attempt int) (core.LockRelease, error) {
	// Create search requirements
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

	// Adjust timeout based on attempt number
	timeout := sc.config.DefaultSearchTimeout
	if attempt > 0 {
		// Increase timeout for retries
		timeout = time.Duration(float64(timeout) * (1.5 + float64(attempt)*0.5))
	}

	core.LogCoordinationInfo(fmt.Sprintf("Using %v timeout for lock acquisition attempt %d", timeout, attempt+1), core.ErrorContext{
		OperationType: "search_locks_timeout_config",
		WaitTime:      timeout,
		ConcurrentOps: attempt + 1,
	})

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Acquire locks through the index coordinator with adjusted timeout
	releases, err := sc.indexCoordinator.AcquireMultipleLocksCtx(timeoutCtx, requirements, false)
	if err != nil {
		// Add context about retry attempt to the error
		if coordErr, ok := err.(*core.CoordinationError); ok {
			coordErr.Context.OperationType = fmt.Sprintf("search_retry_%d", attempt)
			core.LogCoordinationWarning(fmt.Sprintf("Lock acquisition failed on attempt %d: %v", attempt+1, err), core.ErrorContext{
				OperationType: "search_locks_acquire_error",
				IndexType:     getIndexTypeFromList(indexTypes),
				WaitTime:      timeout,
				ConcurrentOps: attempt + 1,
			})
			return nil, coordErr
		}
		core.LogCoordinationWarning(fmt.Sprintf("Lock acquisition failed on attempt %d (non-coordination error): %v", attempt+1, err), core.ErrorContext{
			OperationType: "search_locks_acquire_unknown_error",
			IndexType:     getIndexTypeFromList(indexTypes),
			WaitTime:      timeout,
			ConcurrentOps: attempt + 1,
		})
		return nil, err
	}

	core.LogCoordinationInfo(fmt.Sprintf("Lock acquisition completed successfully for attempt %d", attempt+1), core.ErrorContext{
		OperationType: "search_locks_acquire_success",
		IndexType:     getIndexTypeFromList(indexTypes),
		ConcurrentOps: attempt + 1,
	})

	return releases, nil
}

// T035: Graceful degradation methods

// DegradedSearchResult represents the result of a degraded search operation (T035)
type DegradedSearchResult struct {
	OriginalRequest    []core.IndexType `json:"originalRequest"`
	AvailableIndexes   []core.IndexType `json:"availableIndexes"`
	UnavailableIndexes []core.IndexType `json:"unavailableIndexes"`
	SelectedIndexes    []core.IndexType `json:"selectedIndexes"`
	DegradationLevel   DegradationLevel `json:"degradationLevel"`
	QualityScore       float64          `json:"qualityScore"`
	Warnings           []string         `json:"warnings"`
}

// DegradationLevel represents the level of search degradation (T035)
type DegradationLevel int

const (
	// DegradationNone - All required indexes available
	DegradationNone DegradationLevel = iota
	// DegradationMinimal - Some non-critical indexes unavailable
	DegradationMinimal
	// DegradationModerate - Some important indexes unavailable
	DegradationModerate
	// DegradationSevere - Most indexes unavailable, basic search only
	DegradationSevere
	// DegradationFallback - No indexes available, using fallback
	DegradationFallback
)

// String returns the string representation of DegradationLevel
func (dl DegradationLevel) String() string {
	switch dl {
	case DegradationNone:
		return "None"
	case DegradationMinimal:
		return "Minimal"
	case DegradationModerate:
		return "Moderate"
	case DegradationSevere:
		return "Severe"
	case DegradationFallback:
		return "Fallback"
	default:
		return "Unknown"
	}
}

// SearchWithGracefulDegradation executes a search with graceful degradation support (T035)
func (sc *SearchCoordinator) SearchWithGracefulDegradation(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.Result, error),
) (*SearchResult, error) {
	// Determine required indexes based on search options
	requiredIndexes := sc.determineRequiredIndexes(options)

	// Check index availability
	availability := sc.checkIndexAvailability(requiredIndexes)

	core.LogCoordinationInfo(fmt.Sprintf("Index availability check: %d/%d indexes available, score: %.2f",
		len(availability.AvailableIndexes), len(requiredIndexes), availability.QualityScore), core.ErrorContext{
		OperationType: "search_degradation_availability_check",
	})

	// Determine if we can proceed with graceful degradation
	if !sc.config.EnableGracefulDegradation {
		// Graceful degradation disabled, proceed with normal search
		return sc.searchWithRetry(ctx, pattern, options, searchFunc, false)
	}

	// Check if we have enough indexes available
	if availability.QualityScore >= sc.config.PartialSearchThreshold {
		// We have enough indexes, proceed with degraded or normal search
		return sc.executeDegradedSearch(ctx, pattern, options, searchFunc, availability, false)
	}

	// Not enough indexes available, check if fallback search is enabled
	if sc.config.FallbackSearchEnabled {
		core.LogCoordinationWarning("Insufficient indexes available, attempting fallback search", core.ErrorContext{
			OperationType: "search_degradation_fallback_attempt",
		})
		return sc.executeFallbackSearch(ctx, pattern, options, searchFunc, availability)
	}

	// No fallback available, return error
	core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
		core.ErrCodeIndexUnavailable,
		"Insufficient indexes available for search and fallback disabled",
		fmt.Sprintf("Required: %v, Available: %v, Quality: %.2f", requiredIndexes, availability.AvailableIndexes, availability.QualityScore),
	).WithContext(core.ErrorContext{
		OperationType: "search_degradation_insufficient_indexes",
	}))

	return &SearchResult{
		Results:            nil,
		WaitTime:           0,
		LocksUsed:          requiredIndexes,
		UnavailableIndexes: availability.UnavailableIndexes,
		DegradedMode:       false,
		PartialResults:     false,
		Error:              fmt.Errorf("insufficient indexes available: %.2f/%.2f", availability.QualityScore, sc.config.PartialSearchThreshold),
	}, nil
}

// IndexAvailability represents the availability status of indexes (T035)
type IndexAvailability struct {
	RequiredIndexes    []core.IndexType                    `json:"requiredIndexes"`
	AvailableIndexes   []core.IndexType                    `json:"availableIndexes"`
	UnavailableIndexes []core.IndexType                    `json:"unavailableIndexes"`
	QualityScore       float64                             `json:"qualityScore"`
	HealthStatus       map[core.IndexType]core.IndexHealth `json:"healthStatus"`
}

// checkIndexAvailability checks the availability of required indexes (T035)
func (sc *SearchCoordinator) checkIndexAvailability(requiredIndexes []core.IndexType) *IndexAvailability {
	// Pre-allocate with capacity hint based on input size
	available := make([]core.IndexType, 0, len(requiredIndexes))
	unavailable := make([]core.IndexType, 0, len(requiredIndexes))
	healthStatus := make(map[core.IndexType]core.IndexHealth, len(requiredIndexes))

	for _, indexType := range requiredIndexes {
		status := sc.indexCoordinator.GetIndexStatus(indexType)

		// Try to get health info if available (may not be in interface)
		var health core.IndexHealth
		if coordinator, ok := sc.indexCoordinator.(*core.DefaultIndexCoordinator); ok {
			health = coordinator.GetIndexHealth(indexType)
		} else {
			// Default health status for non-DefaultIndexCoordinator implementations
			health = core.IndexHealth{
				IndexType: indexType,
				Status:    core.HealthStatusHealthy,
			}
		}

		healthStatus[indexType] = health

		// Consider index available if not indexing and not failed
		if !status.IsIndexing && health.Status != core.HealthStatusFailed && health.Status != core.HealthStatusUnknown {
			available = append(available, indexType)
		} else {
			unavailable = append(unavailable, indexType)
		}
	}

	// Calculate quality score based on priority and availability
	qualityScore := sc.calculateQualityScore(requiredIndexes, available, unavailable)

	return &IndexAvailability{
		RequiredIndexes:    requiredIndexes,
		AvailableIndexes:   available,
		UnavailableIndexes: unavailable,
		QualityScore:       qualityScore,
		HealthStatus:       healthStatus,
	}
}

// calculateQualityScore calculates the quality score based on index priority (T035)
func (sc *SearchCoordinator) calculateQualityScore(requiredIndexes, available, unavailable []core.IndexType) float64 {
	if len(requiredIndexes) == 0 {
		return 1.0
	}

	totalWeight := 0.0
	availableWeight := 0.0

	// Use priority order to determine weights
	for i, indexType := range sc.config.IndexPriorityOrder {
		weight := float64(len(sc.config.IndexPriorityOrder) - i) // Higher weight for higher priority

		// Check if this index is in required indexes
		if containsIndex(requiredIndexes, indexType) {
			totalWeight += weight
			if containsIndex(available, indexType) {
				availableWeight += weight
			}
		}
	}

	// If we couldn't match any indexes (unexpected configuration), use simple ratio
	if totalWeight == 0 {
		return float64(len(available)) / float64(len(requiredIndexes))
	}

	return availableWeight / totalWeight
}

// executeDegradedSearch executes a search with the available indexes (T035)
func (sc *SearchCoordinator) executeDegradedSearch(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.Result, error),
	availability *IndexAvailability,
	forceFallback bool,
) (*SearchResult, error) {
	// Determine degradation level
	degradationLevel := sc.determineDegradationLevel(availability)

	core.LogCoordinationInfo("Executing degraded search at level: "+degradationLevel.String(), core.ErrorContext{
		OperationType: "search_degraded_execute",
	})

	// Adjust search options based on available indexes
	adjustedOptions := sc.adjustSearchOptions(options, availability.AvailableIndexes)

	// Set appropriate timeout
	timeout := sc.config.DefaultSearchTimeout
	if degradationLevel != DegradationNone {
		timeout = sc.config.DegradedSearchTimeout
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute search with available indexes
	result, err := sc.searchWithRetry(timeoutCtx, pattern, adjustedOptions, searchFunc, false)
	if err != nil {
		return result, err
	}

	// Add degradation information to result
	result.UnavailableIndexes = availability.UnavailableIndexes
	result.DegradedMode = degradationLevel != DegradationNone
	result.PartialResults = len(availability.UnavailableIndexes) > 0

	return result, nil
}

// executeFallbackSearch executes a fallback search when no indexes are available (T035)
func (sc *SearchCoordinator) executeFallbackSearch(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.Result, error),
	availability *IndexAvailability,
) (*SearchResult, error) {
	core.LogCoordinationInfo("Executing fallback search without indexes", core.ErrorContext{
		OperationType: "search_fallback_execute",
	})

	// Use very short timeout for fallback search
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Execute search without any index requirements
	// This might use basic file system search or other fallback mechanisms
	fallbackOptions := types.SearchOptions{
		// Minimal options for fallback search
		CaseInsensitive: options.CaseInsensitive,
		MaxResults:      options.MaxResults,
		UseRegex:        options.UseRegex,
	}

	result, err := sc.searchWithRetry(timeoutCtx, pattern, fallbackOptions, searchFunc, false)
	if err != nil {
		core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
			core.ErrCodeIndexUnavailable,
			"Fallback search failed",
			fmt.Sprintf("Pattern: %s, Error: %v", pattern, err),
		).WithContext(core.ErrorContext{
			OperationType: "search_fallback_failed",
		}))

		return &SearchResult{
			Results:            nil,
			WaitTime:           0,
			LocksUsed:          []core.IndexType{},
			UnavailableIndexes: availability.RequiredIndexes,
			DegradedMode:       true,
			PartialResults:     true,
			Error:              fmt.Errorf("fallback search failed: %w", err),
		}, nil
	}

	// Mark as severe degradation
	result.UnavailableIndexes = availability.RequiredIndexes
	result.DegradedMode = true
	result.PartialResults = true

	return result, nil
}

// determineDegradationLevel determines the level of degradation based on index availability (T035)
func (sc *SearchCoordinator) determineDegradationLevel(availability *IndexAvailability) DegradationLevel {
	if len(availability.UnavailableIndexes) == 0 {
		return DegradationNone
	}

	if len(availability.AvailableIndexes) == 0 {
		return DegradationFallback
	}

	// Check critical indexes (first 3 in priority order)
	criticalUnavailable := 0
	criticalCount := 3
	if len(sc.config.IndexPriorityOrder) < criticalCount {
		criticalCount = len(sc.config.IndexPriorityOrder)
	}

	for i := 0; i < criticalCount; i++ {
		indexType := sc.config.IndexPriorityOrder[i]
		if containsIndex(availability.RequiredIndexes, indexType) &&
			!containsIndex(availability.AvailableIndexes, indexType) {
			criticalUnavailable++
		}
	}

	// Determine degradation level based on critical index availability
	if criticalUnavailable == 0 {
		return DegradationMinimal
	} else if criticalUnavailable == 1 {
		return DegradationModerate
	} else {
		return DegradationSevere
	}
}

// adjustSearchOptions adjusts search options based on available indexes (T035)
func (sc *SearchCoordinator) adjustSearchOptions(options types.SearchOptions, availableIndexes []core.IndexType) types.SearchOptions {
	adjusted := options

	// Adjust options based on what indexes are available
	hasTrigram := containsIndex(availableIndexes, core.TrigramIndexType)
	hasSymbol := containsIndex(availableIndexes, core.SymbolIndexType)
	hasReference := containsIndex(availableIndexes, core.ReferenceIndexType)
	hasPostings := containsIndex(availableIndexes, core.PostingsIndexType)
	hasLocation := containsIndex(availableIndexes, core.LocationIndexType)

	// If trigram index is not available, we can't do pattern matching effectively
	if !hasTrigram {
		// Limit results and adjust for basic text matching
		if adjusted.MaxResults > 100 {
			adjusted.MaxResults = 100
		}
	}

	// If symbol index is not available, we can't do symbol analysis
	if !hasSymbol {
		adjusted.DeclarationOnly = true // Force declaration-only mode
		adjusted.UsageOnly = false
	}

	// If reference index is not available, we can't track usage
	if !hasReference {
		adjusted.UsageOnly = false
	}

	// If postings index is not available, we can't provide context
	if !hasPostings {
		adjusted.MaxContextLines = 0
	}

	// If location index is not available, disable file filtering
	if !hasLocation {
		adjusted.IncludePattern = ""
		adjusted.ExcludePattern = ""
	}

	return adjusted
}

// Helper functions for T035

func containsIndex(indexes []core.IndexType, target core.IndexType) bool {
	for _, index := range indexes {
		if index == target {
			return true
		}
	}
	return false
}

// T052: Incremental search capability updates

// IncrementalSearchMetrics tracks metrics for incremental search capability updates
type IncrementalSearchMetrics struct {
	TotalCapabilityUpdates  int64
	IndexCompletionEvents   int64
	SearchCapabilityChanges int64
	AverageCapabilityGain   float64
	MaxCapabilityGain       float64
	LastUpdateTime          time.Time
	IndexCapabilities       map[core.IndexType]*IndexCapabilityInfo
	mutex                   sync.RWMutex
}

// IndexCapabilityInfo tracks capability information for a specific index
type IndexCapabilityInfo struct {
	IndexType      core.IndexType
	IsAvailable    bool
	CompletionTime time.Time
	QualityScore   float64
	SearchImpact   float64
	LastUsed       time.Time
	UsageCount     int64
}

// SearchCapabilityUpdate represents an update to search capabilities
type SearchCapabilityUpdate struct {
	CompletedIndex     core.IndexType
	PreviousScore      float64
	NewScore           float64
	GainedCapabilities []string
	ImpactLevel        CapabilityImpact
	Timestamp          time.Time
}

// CapabilityImpact represents the impact level of a capability change
type CapabilityImpact int

const (
	// ImpactNone - No meaningful impact on search capabilities
	ImpactNone CapabilityImpact = iota
	// ImpactMinor - Minor improvement in search quality
	ImpactMinor
	// ImpactModerate - Moderate improvement in search capabilities
	ImpactModerate
	// ImpactCritical - Major improvement in search capabilities
	ImpactCritical
)

// String returns the string representation of CapabilityImpact
func (ci CapabilityImpact) String() string {
	switch ci {
	case ImpactNone:
		return "None"
	case ImpactMinor:
		return "Minor"
	case ImpactModerate:
		return "Moderate"
	case ImpactCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// T049: Search operation queuing and prioritization

// SearchOperation represents a queued search operation with priority information
type SearchOperation struct {
	ID                string                      // Unique identifier
	Priority          core.OperationPriority      // Operation priority
	QueuedAt          time.Time                   // When the operation was queued
	Timeout           time.Duration               // Operation timeout
	Context           context.Context             // Operation context
	Pattern           string                      // Search pattern
	Options           types.SearchOptions         // Search options
	SearchFunc        SearchFunc                  // Search function to execute
	ResultChan        chan *SearchOperationResult // Channel for results
	ClientType        string                      // Type of client (CLI, MCP, etc.)
	EstimatedDuration time.Duration               // Estimated execution time
	RequiredIndexes   []core.IndexType            // Required indexes for this search
	FairShareGroup    string                      // Fair share group identifier
	StarvationScore   float64                     // Starvation prevention score
	Index             int                         // Heap index
}

// SearchOperationResult represents the result of a search operation
type SearchOperationResult struct {
	Operation *SearchOperation
	Result    *SearchResult
	Error     error
}

// SearchFunc represents a search function that can be queued
type SearchFunc func(context.Context, string, types.SearchOptions) ([]searchtypes.Result, error)

// PriorityQueue implements a priority queue for search operations
type PriorityQueue struct {
	operations []*SearchOperation
	mutex      sync.RWMutex
	maxSize    int
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(maxSize int) *PriorityQueue {
	pq := &PriorityQueue{
		operations: make([]*SearchOperation, 0),
		maxSize:    maxSize,
	}
	heap.Init(pq)
	return pq
}

// Len returns the length of the priority queue
func (pq *PriorityQueue) Len() int { return len(pq.operations) }

// Less compares two operations based on priority and starvation score
func (pq *PriorityQueue) Less(i, j int) bool {
	// First compare by priority (higher priority first)
	if pq.operations[i].Priority != pq.operations[j].Priority {
		return pq.operations[i].Priority > pq.operations[j].Priority
	}

	// Then compare by starvation score (higher score first to prevent starvation)
	return pq.operations[i].StarvationScore > pq.operations[j].StarvationScore
}

// Swap swaps two operations in the queue
func (pq *PriorityQueue) Swap(i, j int) {
	pq.operations[i], pq.operations[j] = pq.operations[j], pq.operations[i]
	pq.operations[i].Index = i
	pq.operations[j].Index = j
}

// Push adds an operation to the queue
func (pq *PriorityQueue) Push(x interface{}) {
	operation := x.(*SearchOperation)
	operation.Index = len(pq.operations)
	pq.operations = append(pq.operations, operation)
}

// Pop removes and returns the highest priority operation
func (pq *PriorityQueue) Pop() interface{} {
	old := pq.operations
	n := len(old)
	operation := old[n-1]
	old[n-1] = nil       // avoid memory leak
	operation.Index = -1 // for safety
	pq.operations = old[0 : n-1]
	return operation
}

// Enqueue adds a search operation to the queue
func (pq *PriorityQueue) Enqueue(operation *SearchOperation) error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if len(pq.operations) >= pq.maxSize {
		return fmt.Errorf("queue is full (max size: %d)", pq.maxSize)
	}

	heap.Push(pq, operation)
	return nil
}

// Dequeue removes and returns the highest priority operation
func (pq *PriorityQueue) Dequeue() *SearchOperation {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if len(pq.operations) == 0 {
		return nil
	}

	return heap.Pop(pq).(*SearchOperation)
}

// Peek returns the highest priority operation without removing it
func (pq *PriorityQueue) Peek() *SearchOperation {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	if len(pq.operations) == 0 {
		return nil
	}

	return pq.operations[0]
}

// Size returns the current queue size
func (pq *PriorityQueue) Size() int {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return len(pq.operations)
}

// IsFull returns true if the queue is at maximum capacity
func (pq *PriorityQueue) IsFull() bool {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return len(pq.operations) >= pq.maxSize
}

// QueueMetrics provides metrics about the search queue
type QueueMetrics struct {
	TotalEnqueued        int64
	TotalDequeued        int64
	TotalRequeued        int64
	CurrentQueueSize     int64
	MaxQueueSize         int64
	AverageWaitTime      time.Duration
	MaxWaitTime          time.Duration
	PriorityDistribution map[core.OperationPriority]int64
	FairShareMetrics     map[string]*FairShareMetrics
	mutex                sync.RWMutex
}

// FairShareMetrics tracks fair sharing metrics for a specific group
type FairShareMetrics struct {
	GroupName        string
	OperationsServed int64
	TotalWaitTime    time.Duration
	AverageWaitTime  time.Duration
	LastServedAt     time.Time
	ShareRatio       float64
}

// NewQueueMetrics creates new queue metrics
func NewQueueMetrics() *QueueMetrics {
	return &QueueMetrics{
		PriorityDistribution: make(map[core.OperationPriority]int64),
		FairShareMetrics:     make(map[string]*FairShareMetrics),
		MaxQueueSize:         500, // Default, will be updated by config
	}
}

// RecordEnqueue records an enqueue operation
func (qm *QueueMetrics) RecordEnqueue(priority core.OperationPriority, group string) {
	atomic.AddInt64(&qm.TotalEnqueued, 1)
	atomic.AddInt64(&qm.CurrentQueueSize, 1)

	qm.mutex.Lock()
	qm.PriorityDistribution[priority]++

	if _, exists := qm.FairShareMetrics[group]; !exists {
		qm.FairShareMetrics[group] = &FairShareMetrics{GroupName: group}
	}
	qm.mutex.Unlock()
}

// RecordDequeue records a dequeue operation
func (qm *QueueMetrics) RecordDequeue(priority core.OperationPriority, waitTime time.Duration, group string) {
	atomic.AddInt64(&qm.TotalDequeued, 1)
	atomic.AddInt64(&qm.CurrentQueueSize, -1)

	qm.mutex.Lock()

	// Update average wait time
	totalOps := atomic.LoadInt64(&qm.TotalDequeued)
	if totalOps > 1 {
		qm.AverageWaitTime = time.Duration((int64(qm.AverageWaitTime)*(totalOps-1) + int64(waitTime)) / totalOps)
	} else {
		qm.AverageWaitTime = waitTime
	}

	// Update max wait time
	if waitTime > qm.MaxWaitTime {
		qm.MaxWaitTime = waitTime
	}

	// Update fair share metrics
	if fsMetrics, exists := qm.FairShareMetrics[group]; exists {
		fsMetrics.OperationsServed++
		fsMetrics.TotalWaitTime += waitTime
		fsMetrics.AverageWaitTime = fsMetrics.TotalWaitTime / time.Duration(fsMetrics.OperationsServed)
		fsMetrics.LastServedAt = time.Now()
	}

	qm.mutex.Unlock()
}

// StarvationTracker prevents search operation starvation
type StarvationTracker struct {
	config           *SearchCoordinatorConfig
	operationCounts  map[core.OperationPriority]int64
	lastServiceTimes map[core.OperationPriority]time.Time
	mutex            sync.RWMutex
}

// NewStarvationTracker creates a new starvation tracker
func NewStarvationTracker(config *SearchCoordinatorConfig) *StarvationTracker {
	return &StarvationTracker{
		config:           config,
		operationCounts:  make(map[core.OperationPriority]int64),
		lastServiceTimes: make(map[core.OperationPriority]time.Time),
	}
}

// CalculateStarvationScore calculates a starvation prevention score for an operation
func (st *StarvationTracker) CalculateStarvationScore(operation *SearchOperation) float64 {
	if !st.config.StarvationPrevention {
		return 0.0
	}

	st.mutex.RLock()
	defer st.mutex.RUnlock()

	score := 0.0

	// Base score from wait time
	waitTime := time.Since(operation.QueuedAt)
	score += math.Min(float64(waitTime.Nanoseconds())/float64(time.Second.Nanoseconds()), 10.0)

	// Bonus for priority levels that haven't been served recently
	if lastServed, exists := st.lastServiceTimes[operation.Priority]; exists {
		timeSinceLastService := time.Since(lastServed)
		score += math.Min(float64(timeSinceLastService.Nanoseconds())/float64(time.Minute.Nanoseconds()), 5.0)
	} else {
		// Never served this priority level
		score += 5.0
	}

	// Account for relative service count
	if count, exists := st.operationCounts[operation.Priority]; exists {
		totalOps := int64(0)
		for _, c := range st.operationCounts {
			totalOps += c
		}
		if totalOps > 0 {
			ratio := float64(count) / float64(totalOps)
			if ratio < 0.1 { // Under-served priority level
				score += 3.0
			}
		}
	}

	return score
}

// RecordService records that an operation of a given priority was served
func (st *StarvationTracker) RecordService(priority core.OperationPriority) {
	if !st.config.StarvationPrevention {
		return
	}

	st.mutex.Lock()
	defer st.mutex.Unlock()

	st.operationCounts[priority]++
	st.lastServiceTimes[priority] = time.Now()
}

// FairShareScheduler ensures fair resource allocation among different client types
type FairShareScheduler struct {
	config      *SearchCoordinatorConfig
	groupQuotas map[string]float64
	groupUsage  map[string]int64
	lastReset   time.Time
	mutex       sync.RWMutex
}

// NewFairShareScheduler creates a new fair share scheduler
func NewFairShareScheduler(config *SearchCoordinatorConfig) *FairShareScheduler {
	return &FairShareScheduler{
		config:      config,
		groupQuotas: map[string]float64{"CLI": 0.3, "MCP": 0.5, "Web": 0.2}, // Default quotas
		groupUsage:  make(map[string]int64),
		lastReset:   time.Now(),
	}
}

// CanSchedule determines if an operation can be scheduled based on fair share policies
func (fss *FairShareScheduler) CanSchedule(operation *SearchOperation) bool {
	if !fss.config.FairShareEnabled {
		return true
	}

	fss.mutex.RLock()
	defer fss.mutex.RUnlock()

	quota, exists := fss.groupQuotas[operation.FairShareGroup]
	if !exists {
		quota = 0.1 // Default quota for unknown groups
	}

	// Simple implementation: check if group is within quota
	// In a more sophisticated implementation, this would consider time windows
	totalUsage := int64(0)
	for _, usage := range fss.groupUsage {
		totalUsage += usage
	}

	if totalUsage == 0 {
		return true
	}

	currentUsage := fss.groupUsage[operation.FairShareGroup]
	currentRatio := float64(currentUsage) / float64(totalUsage)

	// Allow if current usage is below quota or within a small tolerance
	return currentRatio <= (quota + 0.1)
}

// RecordUsage records that an operation was executed for a specific group
func (fss *FairShareScheduler) RecordUsage(group string) {
	if !fss.config.FairShareEnabled {
		return
	}

	fss.mutex.Lock()
	defer fss.mutex.Unlock()

	fss.groupUsage[group]++

	// Reset usage counters periodically (every 5 minutes)
	if time.Since(fss.lastReset) > 5*time.Minute {
		for k := range fss.groupUsage {
			fss.groupUsage[k] = 0
		}
		fss.lastReset = time.Now()
	}
}

// EnqueueSearch adds a search operation to the priority queue
func (sc *SearchCoordinator) EnqueueSearch(
	ctx context.Context,
	pattern string,
	options types.SearchOptions,
	searchFunc SearchFunc,
	priority core.OperationPriority,
	clientType string,
) (*SearchOperation, error) {
	if !sc.config.EnableSearchQueueing {
		// If queuing is disabled, execute directly
		result, err := sc.searchWithRetry(ctx, pattern, options, func(ctx context.Context, pattern string, opts types.SearchOptions) ([]searchtypes.Result, error) {
			return searchFunc(ctx, pattern, opts)
		}, false)

		resultChan := make(chan *SearchOperationResult, 1)
		resultChan <- &SearchOperationResult{
			Result: result,
			Error:  err,
		}
		close(resultChan)

		return &SearchOperation{
			ResultChan: resultChan,
		}, nil
	}

	// Create search operation
	operation := &SearchOperation{
		ID:                generateOperationID(),
		Priority:          priority,
		QueuedAt:          time.Now(),
		Timeout:           sc.calculateOperationTimeout(priority),
		Context:           ctx,
		Pattern:           pattern,
		Options:           options,
		SearchFunc:        searchFunc,
		ResultChan:        make(chan *SearchOperationResult, 1),
		ClientType:        clientType,
		EstimatedDuration: sc.estimateSearchDuration(options),
		RequiredIndexes:   sc.determineRequiredIndexes(options),
		FairShareGroup:    sc.determineFairShareGroup(clientType, options),
		StarvationScore:   0.0, // Will be calculated by tracker
	}

	// Calculate starvation score
	operation.StarvationScore = sc.starvationTracker.CalculateStarvationScore(operation)

	// Check if queue is full
	if sc.searchQueue.IsFull() {
		return nil, fmt.Errorf("search queue is full (max size: %d)", sc.config.MaxQueueSize)
	}

	// Check fair share policies
	if !sc.fairShareScheduler.CanSchedule(operation) {
		return nil, errors.New("operation rejected due to fair share policies")
	}

	// Enqueue operation
	if err := sc.searchQueue.Enqueue(operation); err != nil {
		return nil, fmt.Errorf("failed to enqueue search operation: %w", err)
	}

	// Record metrics
	sc.queueMetrics.RecordEnqueue(priority, operation.FairShareGroup)

	// Start queue processor if not already running
	go sc.processQueue()

	return operation, nil
}

// T051-P3: Replace busy-waiting loops with condition variables for efficient queue processing
func (sc *SearchCoordinator) processQueue() {
	// T051-P3: Create a mutex and condition variable for proper coordination
	var queueMutex sync.Mutex
	queueCond := sync.NewCond(&queueMutex)

	for {
		// Wait for either queue state to change or shutdown signal
		queueMutex.Lock()

		// Check if we can start a new search
		running := atomic.LoadInt64(&sc.runningSearches)
		if running >= int64(sc.config.MaxConcurrentSearches) {
			// T051-P3: Wait for capacity to free up
			queueCond.Wait()
			queueMutex.Unlock()
			continue
		}

		// Check if queue has operations
		operation := sc.searchQueue.Dequeue()
		if operation == nil {
			// T051-P3: Wait for operations to arrive
			queueCond.Wait()
			queueMutex.Unlock()
			continue
		}

		// Release mutex before starting operation
		queueMutex.Unlock()

		// Check if operation context is cancelled
		if operation.Context.Err() != nil {
			operation.ResultChan <- &SearchOperationResult{
				Operation: operation,
				Error:     fmt.Errorf("operation cancelled before execution: %w", operation.Context.Err()),
			}
			close(operation.ResultChan)
			// Notify waiting operations that space might be available
			queueCond.Broadcast()
			continue
		}

		// Check if operation has timed out in queue
		if time.Since(operation.QueuedAt) > operation.Timeout {
			operation.ResultChan <- &SearchOperationResult{
				Operation: operation,
				Error:     fmt.Errorf("operation timed out in queue after %v", operation.Timeout),
			}
			close(operation.ResultChan)
			// Notify waiting operations that space might be available
			queueCond.Broadcast()
			continue
		}

		// Start search operation
		atomic.AddInt64(&sc.runningSearches, 1)
		go sc.executeQueuedSearch(operation)

		// Notify waiting operations that space is available
		queueCond.Broadcast()
	}
}

// executeQueuedSearch executes a queued search operation
func (sc *SearchCoordinator) executeQueuedSearch(operation *SearchOperation) {
	defer atomic.AddInt64(&sc.runningSearches, -1)
	defer close(operation.ResultChan)

	// Record service for starvation tracking
	sc.starvationTracker.RecordService(operation.Priority)

	// Record usage for fair share scheduling
	sc.fairShareScheduler.RecordUsage(operation.FairShareGroup)

	// Calculate wait time
	waitTime := time.Since(operation.QueuedAt)

	// Record dequeue metrics
	sc.queueMetrics.RecordDequeue(operation.Priority, waitTime, operation.FairShareGroup)

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(operation.Context, operation.Timeout)
	defer cancel()

	// Execute search
	result, err := sc.searchWithRetry(timeoutCtx, operation.Pattern, operation.Options, func(ctx context.Context, pattern string, opts types.SearchOptions) ([]searchtypes.Result, error) {
		return operation.SearchFunc(ctx, pattern, opts)
	}, false)

	// Send result
	operation.ResultChan <- &SearchOperationResult{
		Operation: operation,
		Result:    result,
		Error:     err,
	}
}

// calculateOperationTimeout calculates timeout based on operation priority
func (sc *SearchCoordinator) calculateOperationTimeout(priority core.OperationPriority) time.Duration {
	baseTimeout := sc.config.DefaultSearchTimeout

	switch priority {
	case core.PriorityCritical:
		return baseTimeout
	case core.PriorityHigh:
		return time.Duration(float64(baseTimeout) * sc.config.QueueTimeoutScale)
	case core.PriorityNormal:
		return time.Duration(float64(baseTimeout) * sc.config.QueueTimeoutScale * 1.5)
	case core.PriorityLow:
		return time.Duration(float64(baseTimeout) * sc.config.QueueTimeoutScale * 2.0)
	default:
		return baseTimeout
	}
}

// estimateSearchDuration estimates the execution time for a search operation
func (sc *SearchCoordinator) estimateSearchDuration(options types.SearchOptions) time.Duration {
	// Base estimation
	estimatedDuration := 50 * time.Millisecond

	// Add time based on options complexity
	if options.MaxContextLines > 0 {
		estimatedDuration += time.Duration(options.MaxContextLines) * 10 * time.Millisecond
	}

	if options.UseRegex {
		estimatedDuration += 20 * time.Millisecond
	}

	if !options.CaseInsensitive {
		estimatedDuration += 10 * time.Millisecond
	}

	return estimatedDuration
}

// determineFairShareGroup determines the fair share group for an operation
func (sc *SearchCoordinator) determineFairShareGroup(clientType string, options types.SearchOptions) string {
	// Use client type as primary grouping
	switch clientType {
	case "CLI", "cli":
		return "CLI"
	case "MCP", "mcp":
		return "MCP"
	case "Web", "web":
		return "Web"
	default:
		return "Other"
	}
}

// generateOperationID generates a unique operation ID
func generateOperationID() string {
	return fmt.Sprintf("search_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// T052: Incremental search capability updates implementation

// NewIncrementalSearchMetrics creates new incremental search metrics
func NewIncrementalSearchMetrics() *IncrementalSearchMetrics {
	return &IncrementalSearchMetrics{
		IndexCapabilities: make(map[core.IndexType]*IndexCapabilityInfo),
		LastUpdateTime:    time.Now(),
	}
}

// RegisterIndexCompletionListener registers a listener for index completion events
func (sc *SearchCoordinator) RegisterIndexCompletionListener(indexType core.IndexType) <-chan core.IndexType {
	sc.capabilityMutex.Lock()
	defer sc.capabilityMutex.Unlock()

	listener := make(chan core.IndexType, 1)
	sc.indexCompletionListeners[indexType] = append(sc.indexCompletionListeners[indexType], listener)

	core.LogCoordinationInfo("Registered completion listener for index type: "+indexType.String(), core.ErrorContext{
		OperationType: "incremental_search_listener_register",
		IndexType:     indexType,
	})

	return listener
}

// NotifyIndexCompletion notifies listeners that an index has completed indexing
func (sc *SearchCoordinator) NotifyIndexCompletion(indexType core.IndexType) {
	sc.capabilityMutex.Lock()
	listeners := sc.indexCompletionListeners[indexType]
	sc.capabilityMutex.Unlock()

	// Update capability metrics
	sc.updateIndexCapability(indexType, true)

	// Notify all registered listeners
	for _, listener := range listeners {
		select {
		case listener <- indexType:
			// Successfully notified listener
		default:
			// Listener channel is full, skip
		}
	}

	core.LogCoordinationInfo(fmt.Sprintf("Notified %d listeners of index completion: %s", len(listeners), indexType.String()), core.ErrorContext{
		OperationType: "incremental_search_index_complete",
		IndexType:     indexType,
	})
}

// updateIndexCapability updates the capability information for an index type
func (sc *SearchCoordinator) updateIndexCapability(indexType core.IndexType, isAvailable bool) {
	sc.incrementalMetrics.mutex.Lock()
	defer sc.incrementalMetrics.mutex.Unlock()

	now := time.Now()

	// Get or create capability info
	info, exists := sc.incrementalMetrics.IndexCapabilities[indexType]
	if !exists {
		info = &IndexCapabilityInfo{
			IndexType: indexType,
		}
		sc.incrementalMetrics.IndexCapabilities[indexType] = info
	}

	previousAvailability := info.IsAvailable
	info.IsAvailable = isAvailable
	info.CompletionTime = now
	info.QualityScore = sc.calculateIndexQualityScore(indexType)
	info.SearchImpact = sc.calculateIndexSearchImpact(indexType)

	// Update metrics
	if isAvailable && !previousAvailability {
		// Index just became available
		atomic.AddInt64(&sc.incrementalMetrics.IndexCompletionEvents, 1)
		atomic.AddInt64(&sc.incrementalMetrics.TotalCapabilityUpdates, 1)

		// Calculate capability gain
		capabilityGain := info.SearchImpact
		sc.updateCapabilityGains(capabilityGain)

		core.LogCoordinationInfo(fmt.Sprintf("Index %s completed with capability gain: %.2f", indexType.String(), capabilityGain), core.ErrorContext{
			OperationType: "incremental_search_capability_update",
			IndexType:     indexType,
		})
	}

	sc.incrementalMetrics.LastUpdateTime = now
}

// calculateIndexQualityScore calculates the quality score for a specific index type
func (sc *SearchCoordinator) calculateIndexQualityScore(indexType core.IndexType) float64 {
	// Check index health and status
	status := sc.indexCoordinator.GetIndexStatus(indexType)
	if status.IsIndexing {
		return 0.0 // Still indexing, no quality score yet
	}

	// Base quality score based on index type importance
	importanceScores := map[core.IndexType]float64{
		core.TrigramIndexType:   0.9, // Critical for basic pattern matching
		core.SymbolIndexType:    0.8, // Important for symbol analysis
		core.ReferenceIndexType: 0.7, // Important for reference tracking
		core.PostingsIndexType:  0.6, // Important for context
		core.LocationIndexType:  0.5, // Important for file filtering
		core.ContentIndexType:   0.4, // Important for full content search
		core.CallGraphIndexType: 0.3, // Least critical, can be omitted
	}

	baseScore, exists := importanceScores[indexType]
	if !exists {
		baseScore = 0.5 // Default score for unknown index types
	}

	// Adjust based on index health
	var health core.IndexHealth
	if coordinator, ok := sc.indexCoordinator.(*core.DefaultIndexCoordinator); ok {
		health = coordinator.GetIndexHealth(indexType)
	} else {
		health = core.IndexHealth{
			IndexType: indexType,
			Status:    core.HealthStatusHealthy,
		}
	}

	// Apply health modifiers
	switch health.Status {
	case core.HealthStatusHealthy:
		return baseScore
	case core.HealthStatusDegraded:
		return baseScore * 0.7
	case core.HealthStatusFailed:
		return 0.0
	case core.HealthStatusUnknown:
		return baseScore * 0.5
	default:
		return baseScore * 0.5
	}
}

// calculateIndexSearchImpact calculates the search impact of an index type
func (sc *SearchCoordinator) calculateIndexSearchImpact(indexType core.IndexType) float64 {
	// Search impact represents how much this index type improves overall search capabilities
	impactScores := map[core.IndexType]float64{
		core.TrigramIndexType:   1.0, // Enables all pattern matching
		core.SymbolIndexType:    0.8, // Enables symbol analysis and navigation
		core.ReferenceIndexType: 0.7, // Enables usage tracking and cross-references
		core.PostingsIndexType:  0.6, // Enables context extraction and snippets
		core.LocationIndexType:  0.5, // Enables file filtering and path-based searches
		core.ContentIndexType:   0.4, // Enables full-text content search
		core.CallGraphIndexType: 0.3, // Enables call graph analysis and dependency tracking
	}

	impact, exists := impactScores[indexType]
	if !exists {
		impact = 0.5 // Default impact for unknown index types
	}

	return impact
}

// updateCapabilityGains updates the capability gain metrics
func (sc *SearchCoordinator) updateCapabilityGains(gain float64) {
	// Update average capability gain
	totalUpdates := atomic.LoadInt64(&sc.incrementalMetrics.TotalCapabilityUpdates)
	if totalUpdates > 0 {
		currentAverage := sc.incrementalMetrics.AverageCapabilityGain
		newAverage := (currentAverage*float64(totalUpdates-1) + gain) / float64(totalUpdates)
		sc.incrementalMetrics.AverageCapabilityGain = newAverage
	} else {
		sc.incrementalMetrics.AverageCapabilityGain = gain
	}

	// Update maximum capability gain
	if gain > sc.incrementalMetrics.MaxCapabilityGain {
		sc.incrementalMetrics.MaxCapabilityGain = gain
	}
}

// GetCurrentSearchCapabilities returns the current search capabilities based on available indexes
func (sc *SearchCoordinator) GetCurrentSearchCapabilities() map[string]interface{} {
	sc.incrementalMetrics.mutex.RLock()
	defer sc.incrementalMetrics.mutex.RUnlock()

	// Pre-allocate with capacity hint
	availableIndexes := make([]core.IndexType, 0, len(sc.incrementalMetrics.IndexCapabilities))
	totalQualityScore := 0.0
	totalSearchImpact := 0.0

	for _, info := range sc.incrementalMetrics.IndexCapabilities {
		if info.IsAvailable {
			availableIndexes = append(availableIndexes, info.IndexType)
			totalQualityScore += info.QualityScore
			totalSearchImpact += info.SearchImpact
		}
	}

	capabilityLevel := sc.determineCapabilityLevel(totalSearchImpact)

	return map[string]interface{}{
		"available_indexes":   availableIndexes,
		"total_quality_score": totalQualityScore,
		"total_search_impact": totalSearchImpact,
		"capability_level":    capabilityLevel.String(),
		"last_update":         sc.incrementalMetrics.LastUpdateTime,
		"completion_events":   atomic.LoadInt64(&sc.incrementalMetrics.IndexCompletionEvents),
		"capability_updates":  atomic.LoadInt64(&sc.incrementalMetrics.TotalCapabilityUpdates),
	}
}

// CapabilityLevel represents the overall search capability level
type CapabilityLevel int

const (
	// CapabilityMinimal - Only basic pattern matching available
	CapabilityMinimal CapabilityLevel = iota
	// CapabilityBasic - Basic search with some symbol analysis
	CapabilityBasic
	// CapabilityEnhanced - Enhanced search with context and references
	CapabilityEnhanced
	// CapabilityFull - Full search capabilities with all indexes
	CapabilityFull
)

// String returns the string representation of CapabilityLevel
func (cl CapabilityLevel) String() string {
	switch cl {
	case CapabilityMinimal:
		return "Minimal"
	case CapabilityBasic:
		return "Basic"
	case CapabilityEnhanced:
		return "Enhanced"
	case CapabilityFull:
		return "Full"
	default:
		return "Unknown"
	}
}

// determineCapabilityLevel determines the overall capability level based on search impact
func (sc *SearchCoordinator) determineCapabilityLevel(totalSearchImpact float64) CapabilityLevel {
	if totalSearchImpact >= 4.0 {
		return CapabilityFull
	} else if totalSearchImpact >= 3.0 {
		return CapabilityEnhanced
	} else if totalSearchImpact >= 2.0 {
		return CapabilityBasic
	} else if totalSearchImpact >= 1.0 {
		return CapabilityMinimal
	} else {
		return CapabilityMinimal
	}
}

// WaitForIndexCompletion waits for a specific index type to complete indexing
func (sc *SearchCoordinator) WaitForIndexCompletion(indexType core.IndexType, timeout time.Duration) error {
	listener := sc.RegisterIndexCompletionListener(indexType)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-listener:
		// Index completed
		core.LogCoordinationInfo("Successfully waited for index completion: "+indexType.String(), core.ErrorContext{
			OperationType: "incremental_search_wait_complete",
			IndexType:     indexType,
		})
		return nil
	case <-ctx.Done():
		// Timeout
		core.LogCoordinationWarning("Timeout waiting for index completion: "+indexType.String(), core.ErrorContext{
			OperationType: "incremental_search_wait_timeout",
			IndexType:     indexType,
		})
		return fmt.Errorf("timeout waiting for index %s to complete", indexType.String())
	}
}
