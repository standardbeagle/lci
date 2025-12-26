package indexing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/core"
)

// Concurrency configuration constants
const (
	// DefaultMaxConcurrentOps is the default maximum concurrent operations
	DefaultMaxConcurrentOps = 16

	// DefaultOpsPerCPU is the default operations per CPU core
	DefaultOpsPerCPU = 2

	// DefaultMinConcurrentOps is the minimum concurrent operations allowed
	DefaultMinConcurrentOps = 2

	// MaxOperationsPerIndexType is the maximum concurrent operations per index type
	MaxOperationsPerIndexType = 10
)

// ConcurrentOperationsManager handles coordination of concurrent indexing and search operations
type ConcurrentOperationsManager struct {
	coordinator core.IndexCoordinator
	operations  *OperationRegistry
	config      *ConcurrentOperationsConfig
	queue       *OperationQueue
	processor   *QueueProcessor
	mu          sync.RWMutex
}

// OperationRegistry tracks active operations and their resource usage
type OperationRegistry struct {
	activeOperations map[string]*ActiveOperation
	operationCounts  map[core.IndexType]int64
	totalOps         int64
	mu               sync.RWMutex
}

// ActiveOperation represents an active operation in the system
type ActiveOperation struct {
	ID                string
	Type              OperationType
	IndexTypes        []core.IndexType
	StartTime         time.Time
	EstimatedDuration time.Duration
	Progress          float64
	Status            OperationStatus
	Context           context.Context
	Cancel            context.CancelFunc
	LockReleases      []core.LockRelease
	mu                sync.RWMutex
}

// OperationType represents the type of operation
type OperationType int

const (
	OperationTypeIndex OperationType = iota
	OperationTypeSearch
	OperationTypeUpdate
	OperationTypeMaintenance
)

// OperationStatus represents the status of an operation
type OperationStatus int

const (
	OperationStatusPending OperationStatus = iota
	OperationStatusRunning
	OperationStatusCompleted
	OperationStatusFailed
	OperationStatusCancelled
)

// ConcurrentOperationsConfig configures concurrent operations behavior
type ConcurrentOperationsConfig struct {
	MaxConcurrentOps       int
	DefaultTimeout         time.Duration
	ProgressUpdateInterval time.Duration
	EnableOperationMetrics bool
	QueueFullBehavior      QueueBehavior
}

// QueueBehavior determines behavior when operation queue is full
type QueueBehavior int

const (
	QueueBehaviorReject QueueBehavior = iota
	QueueBehaviorWait
	QueueBehaviorPrioritize
)

// QueueProcessor processes queued operations (T038)
type QueueProcessor struct {
	queue     *OperationQueue
	processor *ConcurrentOperationsManager
	running   bool
	stopChan  chan struct{}
	mu        sync.RWMutex
}

// NewQueueProcessor creates a new queue processor (T038)
func NewQueueProcessor(queue *OperationQueue, processor *ConcurrentOperationsManager) *QueueProcessor {
	return &QueueProcessor{
		queue:     queue,
		processor: processor,
		running:   false,
		stopChan:  make(chan struct{}),
	}
}

// Start starts the queue processor (T038)
func (qp *QueueProcessor) Start() {
	qp.mu.Lock()
	defer qp.mu.Unlock()

	if qp.running {
		return
	}

	qp.running = true
	go qp.processQueue()

	core.LogCoordinationInfo("Queue processor started", core.ErrorContext{
		OperationType: "queue_processor_start",
	})
}

// Stop stops the queue processor (T038)
func (qp *QueueProcessor) Stop() {
	qp.mu.Lock()
	defer qp.mu.Unlock()

	if !qp.running {
		return
	}

	qp.running = false
	close(qp.stopChan)

	core.LogCoordinationInfo("Queue processor stopped", core.ErrorContext{
		OperationType: "queue_processor_stop",
	})
}

// IsRunning returns whether the queue processor is running (T038)
func (qp *QueueProcessor) IsRunning() bool {
	qp.mu.RLock()
	defer qp.mu.RUnlock()
	return qp.running
}

// processQueue continuously processes queued operations (T038)
func (qp *QueueProcessor) processQueue() {
	// Use adaptive polling with backoff to reduce CPU usage
	baseInterval := 50 * time.Millisecond
	maxInterval := 500 * time.Millisecond
	currentInterval := baseInterval
	idleCount := 0

	for {
		select {
		case <-qp.stopChan:
			return
		case <-time.After(currentInterval):
			processed := qp.processNextOperation()
			if processed == -1 {
				// processor indicated it should stop (no longer running)
				return
			}

			// Adaptive backoff based on queue activity
			if processed > 0 {
				// Reset to base interval when work is done
				currentInterval = baseInterval
				idleCount = 0
			} else if processed == 0 {
				// Gradually increase interval when idle
				idleCount++
				if idleCount > 5 && currentInterval < maxInterval {
					currentInterval = currentInterval * 2
					if currentInterval > maxInterval {
						currentInterval = maxInterval
					}
				}
			}
			// processed == -1 case handled above (return)
		}
	}
}

// processNextOperation processes the next operation in the queue (T038)
// Returns: true if work was processed, false if no work available, -1 if should stop
func (qp *QueueProcessor) processNextOperation() int {
	// Check if still running under lock to prevent race conditions
	qp.mu.RLock()
	running := qp.running
	qp.mu.RUnlock()
	if !running {
		return -1 // Signal to stop processing
	}

	// Check if we can start a new operation
	if !qp.processor.canStartQueuedOperation() {
		return 0 // Can't start operation, but continue trying
	}

	// Get next operation from queue
	op, err := qp.queue.Dequeue()
	if err != nil {
		return 0 // Queue is empty, but continue trying
	}

	// Execute the operation in a goroutine
	go qp.executeQueuedOperation(op)
	return 1 // Work was processed
}

// executeQueuedOperation executes a queued operation (T038)
func (qp *QueueProcessor) executeQueuedOperation(op *QueuedOperation) {
	core.LogCoordinationInfo(fmt.Sprintf("Executing queued operation %s with priority %s", op.ID, op.Priority.String()), core.ErrorContext{
		OperationType: "queued_operation_execute",
	})

	// Create context for operation
	ctx := op.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Execute the operation function
	err := op.Function(ctx)

	// Handle operation result
	if err != nil {
		core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
			core.ErrCodeSystemShutdown,
			fmt.Sprintf("Queued operation %s failed", op.ID),
			fmt.Sprintf("Error: %v, Retries: %d/%d", err, op.RetryCount, op.MaxRetries),
		).WithContext(core.ErrorContext{
			OperationType: "queued_operation_failed",
		}))

		// Handle retry logic
		if op.RetryCount < op.MaxRetries {
			op.RetryCount++

			// Add exponential backoff delay with jitter before retry
			// Base delay: 1 second, exponential growth with cap at 30 seconds
			baseDelay := 1 * time.Second
			maxDelay := 30 * time.Second
			exponentialDelay := time.Duration(math.Pow(2, float64(op.RetryCount-1))) * baseDelay

			// Cap the delay
			if exponentialDelay > maxDelay {
				exponentialDelay = maxDelay
			}

			// Add jitter (±20%) to prevent thundering herd
			jitter := time.Duration(float64(exponentialDelay) * 0.2 * (rand.Float64() - 0.5))
			retryDelay := exponentialDelay + jitter

			// Ensure minimum delay
			if retryDelay < baseDelay {
				retryDelay = baseDelay
			}

			time.Sleep(retryDelay)

			// Re-queue with same priority
			if err := qp.queue.Enqueue(op); err != nil {
				core.LogCoordinationError(core.NewCoordinationErrorWithDetails(
					core.ErrCodeSystemShutdown,
					"Failed to re-queue operation "+op.ID,
					fmt.Sprintf("Error: %v", err),
				).WithContext(core.ErrorContext{
					OperationType: "queued_operation_requeue_failed",
				}))
			}
		}
	} else {
		core.LogCoordinationInfo(fmt.Sprintf("Queued operation %s completed successfully", op.ID), core.ErrorContext{
			OperationType: "queued_operation_completed",
		})
	}
}

// NewConcurrentOperationsManager creates a new concurrent operations manager
func NewConcurrentOperationsManager(coordinator core.IndexCoordinator) *ConcurrentOperationsManager {
	config := DefaultConcurrentOperationsConfig()

	// Create operation queue with size limit
	queue := NewOperationQueue(config.MaxConcurrentOps * 2) // Allow some queuing

	// Create manager
	manager := &ConcurrentOperationsManager{
		coordinator: coordinator,
		operations: &OperationRegistry{
			activeOperations: make(map[string]*ActiveOperation),
			operationCounts:  make(map[core.IndexType]int64),
		},
		config:    config,
		queue:     queue,
		processor: nil, // Will be set after manager is created
	}

	// Create queue processor with manager reference
	manager.processor = NewQueueProcessor(queue, manager)

	// Start queue processor
	manager.processor.Start()

	return manager
}

// NewConcurrentOperationsManagerWithConfig creates a manager with custom configuration
func NewConcurrentOperationsManagerWithConfig(coordinator core.IndexCoordinator, config *ConcurrentOperationsConfig) *ConcurrentOperationsManager {
	// Create operation queue with size limit
	queue := NewOperationQueue(config.MaxConcurrentOps * 2)

	// Create manager
	manager := &ConcurrentOperationsManager{
		coordinator: coordinator,
		operations: &OperationRegistry{
			activeOperations: make(map[string]*ActiveOperation),
			operationCounts:  make(map[core.IndexType]int64),
		},
		config:    config,
		queue:     queue,
		processor: nil, // Will be set after manager is created
	}

	// Create queue processor with manager reference
	manager.processor = NewQueueProcessor(queue, manager)

	// Start queue processor
	manager.processor.Start()

	return manager
}

// DefaultConcurrentOperationsConfig returns default configuration
func DefaultConcurrentOperationsConfig() *ConcurrentOperationsConfig {
	// Determine max concurrent operations based on available resources
	// Use number of CPU cores as a baseline, with a configurable maximum
	numCPU := runtime.NumCPU()
	maxOps := numCPU * DefaultOpsPerCPU

	// Cap at a configurable maximum to prevent resource exhaustion
	if maxOps > DefaultMaxConcurrentOps {
		maxOps = DefaultMaxConcurrentOps
	}

	// Minimum of DefaultMinConcurrentOps to ensure basic concurrency
	if maxOps < DefaultMinConcurrentOps {
		maxOps = DefaultMinConcurrentOps
	}

	return &ConcurrentOperationsConfig{
		MaxConcurrentOps:       maxOps,
		DefaultTimeout:         30 * time.Second,
		ProgressUpdateInterval: 100 * time.Millisecond,
		EnableOperationMetrics: true,
		QueueFullBehavior:      QueueBehaviorWait,
	}
}

// ExecuteConcurrentIndexing executes an indexing operation with concurrency control
func (m *ConcurrentOperationsManager) ExecuteConcurrentIndexing(
	ctx context.Context,
	indexTypes []core.IndexType,
	indexingFunc func(context.Context, core.IndexType) error,
) error {
	// Check if we can start the operation
	if err := m.canStartOperation(indexTypes); err != nil {
		return err
	}

	// Create operation context with timeout
	opCtx, cancel := context.WithTimeout(ctx, m.config.DefaultTimeout)
	defer cancel()

	// Register the operation
	op := m.registerOperation("indexing_"+generateOperationID(), OperationTypeIndex, indexTypes, opCtx)
	defer m.unregisterOperation(op.ID)

	// Acquire locks for all index types
	releases, err := m.acquireOperationLocks(opCtx, indexTypes, false) // write locks for indexing
	if err != nil {
		return fmt.Errorf("failed to acquire locks for indexing: %w", err)
	}
	defer m.releaseOperationLocks(releases)

	// Execute indexing operations concurrently
	return m.executeConcurrentOperation(opCtx, indexTypes, func(ctx context.Context, indexType core.IndexType) error {
		op.mu.Lock()
		op.Status = OperationStatusRunning
		op.Progress = 0.0
		op.mu.Unlock()

		err := indexingFunc(ctx, indexType)

		op.mu.Lock()
		if err != nil {
			op.Status = OperationStatusFailed
		} else {
			op.Status = OperationStatusCompleted
			op.Progress = 1.0
		}
		op.mu.Unlock()

		return err
	})
}

// ExecuteConcurrentSearch executes a search operation with concurrency control
func (m *ConcurrentOperationsManager) ExecuteConcurrentSearch(
	ctx context.Context,
	requirements core.SearchRequirements,
	searchFunc func(context.Context, core.SearchRequirements) error,
) error {
	// Get required index types
	indexTypes := requirements.GetRequiredIndexTypes()
	if len(indexTypes) == 0 {
		return searchFunc(ctx, requirements)
	}

	// Check if we can start the operation
	if err := m.canStartOperation(indexTypes); err != nil {
		return err
	}

	// Create operation context
	opCtx, cancel := context.WithTimeout(ctx, m.config.DefaultTimeout)
	defer cancel()

	// Register the operation
	op := m.registerOperation("search_"+generateOperationID(), OperationTypeSearch, indexTypes, opCtx)
	defer m.unregisterOperation(op.ID)

	// Acquire locks for required index types
	releases, err := m.acquireOperationLocks(opCtx, indexTypes, true) // read locks for search
	if err != nil {
		return fmt.Errorf("failed to acquire locks for search: %w", err)
	}
	defer m.releaseOperationLocks(releases)

	// Execute search
	op.mu.Lock()
	op.Status = OperationStatusRunning
	op.Progress = 0.0
	op.mu.Unlock()

	err = searchFunc(opCtx, requirements)

	op.mu.Lock()
	if err != nil {
		op.Status = OperationStatusFailed
	} else {
		op.Status = OperationStatusCompleted
		op.Progress = 1.0
	}
	op.mu.Unlock()

	return err
}

// canStartOperation checks if a new operation can be started
func (m *ConcurrentOperationsManager) canStartOperation(indexTypes []core.IndexType) error {
	m.operations.mu.RLock()
	defer m.operations.mu.RUnlock()

	// Check total operation limit
	if atomic.LoadInt64(&m.operations.totalOps) >= int64(m.config.MaxConcurrentOps) {
		return fmt.Errorf("maximum concurrent operations (%d) reached", m.config.MaxConcurrentOps)
	}

	// Check per-index type limits
	for _, indexType := range indexTypes {
		if count, exists := m.operations.operationCounts[indexType]; exists {
			if count >= MaxOperationsPerIndexType {
				return fmt.Errorf("maximum concurrent operations for index type %s reached", indexType.String())
			}
		}
	}

	return nil
}

// registerOperation registers a new operation
func (m *ConcurrentOperationsManager) registerOperation(id string, opType OperationType, indexTypes []core.IndexType, ctx context.Context) *ActiveOperation {
	op := &ActiveOperation{
		ID:                id,
		Type:              opType,
		IndexTypes:        indexTypes,
		StartTime:         time.Now(),
		EstimatedDuration: 10 * time.Second, // Default estimate
		Progress:          0.0,
		Status:            OperationStatusPending,
		Context:           ctx,
	}

	m.operations.mu.Lock()
	defer m.operations.mu.Unlock()

	m.operations.activeOperations[id] = op
	atomic.AddInt64(&m.operations.totalOps, 1)

	for _, indexType := range indexTypes {
		m.operations.operationCounts[indexType]++
	}

	return op
}

// unregisterOperation unregisters an operation
func (m *ConcurrentOperationsManager) unregisterOperation(id string) {
	m.operations.mu.Lock()
	defer m.operations.mu.Unlock()

	if op, exists := m.operations.activeOperations[id]; exists {
		for _, indexType := range op.IndexTypes {
			if count, exists := m.operations.operationCounts[indexType]; exists {
				if count <= 1 {
					delete(m.operations.operationCounts, indexType)
				} else {
					m.operations.operationCounts[indexType] = count - 1
				}
			}
		}
		delete(m.operations.activeOperations, id)
		atomic.AddInt64(&m.operations.totalOps, -1)
	}
}

// acquireOperationLocks acquires locks for the specified index types
func (m *ConcurrentOperationsManager) acquireOperationLocks(ctx context.Context, indexTypes []core.IndexType, forReading bool) ([]core.LockRelease, error) {
	var releases []core.LockRelease

	// Use the coordinator to acquire multiple locks with consistent ordering
	if forReading {
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

		release, err := m.coordinator.AcquireMultipleLocksCtx(ctx, requirements, false)
		if err != nil {
			return nil, err
		}
		releases = append(releases, release)
	} else {
		// For indexing, acquire write locks individually
		for _, indexType := range indexTypes {
			release, err := m.coordinator.AcquireIndexLockCtx(ctx, indexType, true)
			if err != nil {
				// Release any already acquired locks
				for _, r := range releases {
					r()
				}
				return nil, err
			}
			releases = append(releases, release)
		}
	}

	return releases, nil
}

// releaseOperationLocks releases the provided lock releases
func (m *ConcurrentOperationsManager) releaseOperationLocks(releases []core.LockRelease) {
	for _, release := range releases {
		if release != nil {
			release()
		}
	}
}

// executeConcurrentOperation executes operations concurrently across multiple index types
func (m *ConcurrentOperationsManager) executeConcurrentOperation(
	ctx context.Context,
	indexTypes []core.IndexType,
	operationFunc func(context.Context, core.IndexType) error,
) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(indexTypes))

	// Start operations for each index type
	for _, indexType := range indexTypes {
		wg.Add(1)
		go func(iType core.IndexType) {
			defer wg.Done()
			if err := operationFunc(ctx, iType); err != nil {
				errChan <- fmt.Errorf("operation failed for %s: %w", iType.String(), err)
			}
		}(indexType)
	}

	// Wait for all operations to complete
	wg.Wait()
	close(errChan)

	// Return first error if any
	for err := range errChan {
		return err
	}

	return nil
}

// GetActiveOperations returns all active operations
func (m *ConcurrentOperationsManager) GetActiveOperations() []*ActiveOperation {
	m.operations.mu.RLock()
	defer m.operations.mu.RUnlock()

	operations := make([]*ActiveOperation, 0, len(m.operations.activeOperations))
	for _, op := range m.operations.activeOperations {
		operations = append(operations, op)
	}

	return operations
}

// GetOperationCount returns the count of active operations for each index type
func (m *ConcurrentOperationsManager) GetOperationCount() map[core.IndexType]int64 {
	m.operations.mu.RLock()
	defer m.operations.mu.RUnlock()

	counts := make(map[core.IndexType]int64)
	for indexType, count := range m.operations.operationCounts {
		counts[indexType] = count
	}

	return counts
}

// CancelOperation cancels an active operation
func (m *ConcurrentOperationsManager) CancelOperation(operationID string) error {
	m.operations.mu.RLock()
	op, exists := m.operations.activeOperations[operationID]
	m.operations.mu.RUnlock()

	if !exists {
		return fmt.Errorf("operation %s not found", operationID)
	}

	op.mu.Lock()
	defer op.mu.Unlock()

	if op.Status == OperationStatusCompleted || op.Status == OperationStatusFailed {
		return fmt.Errorf("operation %s already completed", operationID)
	}

	// Cancel the operation context
	if op.Cancel != nil {
		op.Cancel()
	}

	// Release any held locks
	for _, release := range op.LockReleases {
		if release != nil {
			release()
		}
	}

	op.Status = OperationStatusCancelled
	return nil
}

// GetSystemStatus returns the current system status
func (m *ConcurrentOperationsManager) GetSystemStatus() *SystemStatus {
	m.operations.mu.RLock()
	defer m.operations.mu.RUnlock()

	return &SystemStatus{
		TotalActiveOps:   atomic.LoadInt64(&m.operations.totalOps),
		ActiveOperations: len(m.operations.activeOperations),
		OperationCounts:  m.GetOperationCount(),
		MaxConcurrentOps: m.config.MaxConcurrentOps,
		UtilizationRate:  float64(atomic.LoadInt64(&m.operations.totalOps)) / float64(m.config.MaxConcurrentOps),
	}
}

// SystemStatus represents the current system status
type SystemStatus struct {
	TotalActiveOps   int64
	ActiveOperations int
	OperationCounts  map[core.IndexType]int64
	MaxConcurrentOps int
	UtilizationRate  float64
}

// T038: Queue management methods

// QueueOperation queues an operation for execution (T038)
func (m *ConcurrentOperationsManager) QueueOperation(
	id string,
	opType OperationType,
	indexTypes []core.IndexType,
	priority OperationPriority,
	function OperationFunc,
) error {
	operation := &QueuedOperation{
		ID:                id,
		Type:              opType,
		IndexTypes:        indexTypes,
		Priority:          priority,
		QueueTime:         time.Now(),
		EstimatedDuration: 10 * time.Second,         // Default estimate
		UserRequest:       priority <= PriorityHigh, // High priority operations are considered user requests
		RetryCount:        0,
		MaxRetries:        3,
		Function:          function,
		Metadata:          make(map[string]interface{}),
	}

	err := m.queue.Enqueue(operation)
	if err != nil {
		return fmt.Errorf("failed to queue operation %s: %w", id, err)
	}

	return nil
}

// QueueOperationWithMetadata queues an operation with additional metadata (T038)
func (m *ConcurrentOperationsManager) QueueOperationWithMetadata(
	id string,
	opType OperationType,
	indexTypes []core.IndexType,
	priority OperationPriority,
	function OperationFunc,
	metadata map[string]interface{},
) error {
	operation := &QueuedOperation{
		ID:                id,
		Type:              opType,
		IndexTypes:        indexTypes,
		Priority:          priority,
		QueueTime:         time.Now(),
		EstimatedDuration: 10 * time.Second, // Default estimate
		UserRequest:       priority <= PriorityHigh,
		RetryCount:        0,
		MaxRetries:        3,
		Function:          function,
		Metadata:          metadata,
	}

	if metadata == nil {
		operation.Metadata = make(map[string]interface{})
	} else {
		operation.Metadata = make(map[string]interface{})
		for k, v := range metadata {
			operation.Metadata[k] = v
		}
	}

	err := m.queue.Enqueue(operation)
	if err != nil {
		return fmt.Errorf("failed to queue operation %s: %w", id, err)
	}

	return nil
}

// GetQueueStatus returns the current queue status (T038)
func (m *ConcurrentOperationsManager) GetQueueStatus() QueueStats {
	return m.queue.GetStats()
}

// ListQueuedOperations returns all queued operations (T038)
func (m *ConcurrentOperationsManager) ListQueuedOperations() []*QueuedOperation {
	return m.queue.ListOperations()
}

// RemoveQueuedOperation removes a queued operation by ID (T038)
func (m *ConcurrentOperationsManager) RemoveQueuedOperation(operationID string) bool {
	return m.queue.Remove(operationID)
}

// UpdateOperationPriority updates the priority of a queued operation (T038)
func (m *ConcurrentOperationsManager) UpdateOperationPriority(operationID string, newPriority OperationPriority) bool {
	return m.queue.UpdatePriority(operationID, newPriority)
}

// ClearQueue clears all queued operations (T038)
func (m *ConcurrentOperationsManager) ClearQueue() int {
	return m.queue.Clear()
}

// canStartQueuedOperation checks if a queued operation can be started (T038)
func (m *ConcurrentOperationsManager) canStartQueuedOperation() bool {
	m.operations.mu.RLock()
	defer m.operations.mu.RUnlock()

	// Check total operation limit
	if atomic.LoadInt64(&m.operations.totalOps) >= int64(m.config.MaxConcurrentOps) {
		return false
	}

	return true
}

// StopQueueProcessor stops the queue processor (T038)
func (m *ConcurrentOperationsManager) StopQueueProcessor() {
	if m.processor != nil {
		m.processor.Stop()
	}
}

// StartQueueProcessor starts the queue processor (T038)
func (m *ConcurrentOperationsManager) StartQueueProcessor() {
	if m.processor != nil {
		m.processor.Start()
	}
}

// IsQueueProcessorRunning returns whether the queue processor is running (T038)
func (m *ConcurrentOperationsManager) IsQueueProcessorRunning() bool {
	if m.processor != nil {
		return m.processor.IsRunning()
	}
	return false
}

// Close cleans up the concurrent operations manager (T038)
func (m *ConcurrentOperationsManager) Close() error {
	// Stop queue processor
	m.StopQueueProcessor()

	// Clear queue
	m.ClearQueue()

	return nil
}

// generateOperationID creates a unique operation ID
func generateOperationID() string {
	return fmt.Sprintf("op_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}

// T038: Index Operation Queuing and Prioritization

// OperationPriority defines priority levels for operations (T038)
type OperationPriority int

const (
	// PriorityCritical - Critical operations (e.g., error recovery, user requests)
	PriorityCritical OperationPriority = iota
	// PriorityHigh - High priority operations (e.g., manual rebuilds)
	PriorityHigh
	// PriorityNormal - Normal priority operations (e.g., regular indexing)
	PriorityNormal
	// PriorityLow - Low priority operations (e.g., maintenance)
	PriorityLow
	// PriorityBackground - Background operations (e.g., statistics)
	PriorityBackground
)

// String returns string representation of OperationPriority
func (op OperationPriority) String() string {
	switch op {
	case PriorityCritical:
		return "Critical"
	case PriorityHigh:
		return "High"
	case PriorityNormal:
		return "Normal"
	case PriorityLow:
		return "Low"
	case PriorityBackground:
		return "Background"
	default:
		return "Unknown"
	}
}

// PriorityValue returns numeric value for priority comparison (lower value = higher priority)
func (op OperationPriority) PriorityValue() int {
	return int(op)
}

// QueuedOperation represents a queued operation waiting to be executed (T038)
type QueuedOperation struct {
	ID                string                 `json:"id"`
	Type              OperationType          `json:"type"`
	IndexTypes        []core.IndexType       `json:"indexTypes"`
	Priority          OperationPriority      `json:"priority"`
	QueueTime         time.Time              `json:"queueTime"`
	EstimatedDuration time.Duration          `json:"estimatedDuration"`
	UserRequest       bool                   `json:"userRequest"` // Whether initiated by user
	RetryCount        int                    `json:"retryCount"`
	MaxRetries        int                    `json:"maxRetries"`
	Context           context.Context        `json:"-"`
	Function          OperationFunc          `json:"-"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// OperationFunc represents the function to execute for a queued operation (T038)
type OperationFunc func(context.Context) error

// OperationQueue manages the priority queue for operations (T038)
type OperationQueue struct {
	heap      []*QueuedOperation
	heapMap   map[string]*QueuedOperation // For O(1) lookups
	sizeLimit int
	mu        sync.RWMutex
	stats     *QueueStats
}

// QueueStats tracks queue statistics (T038)
type QueueStats struct {
	TotalQueued     int64                       `json:"totalQueued"`
	TotalProcessed  int64                       `json:"totalProcessed"`
	TotalFailed     int64                       `json:"totalFailed"`
	AverageWaitTime time.Duration               `json:"averageWaitTime"`
	QueueDepth      int                         `json:"queueDepth"`
	PriorityStats   map[OperationPriority]int64 `json:"priorityStats"`
}

// NewOperationQueue creates a new operation queue (T038)
func NewOperationQueue(sizeLimit int) *OperationQueue {
	return &OperationQueue{
		heap:      make([]*QueuedOperation, 0),
		heapMap:   make(map[string]*QueuedOperation),
		sizeLimit: sizeLimit,
		stats: &QueueStats{
			PriorityStats: make(map[OperationPriority]int64),
		},
	}
}

// Enqueue adds an operation to the queue (T038)
func (q *OperationQueue) Enqueue(op *QueuedOperation) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check queue size limit
	if len(q.heap) >= q.sizeLimit {
		return fmt.Errorf("queue is full (limit: %d)", q.sizeLimit)
	}

	// Check if operation already exists
	if _, exists := q.heapMap[op.ID]; exists {
		return fmt.Errorf("operation %s already in queue", op.ID)
	}

	// Set queue time if not set
	if op.QueueTime.IsZero() {
		op.QueueTime = time.Now()
	}

	// Add to heap and map
	q.heap = append(q.heap, op)
	q.heapMap[op.ID] = op
	q.stats.TotalQueued++
	q.stats.PriorityStats[op.Priority]++

	// Heapify up
	q.heapifyUp(len(q.heap) - 1)

	core.LogCoordinationInfo(fmt.Sprintf("Enqueued operation %s with priority %s", op.ID, op.Priority.String()), core.ErrorContext{
		OperationType: "operation_queue_enqueue",
	})

	return nil
}

// Dequeue removes and returns the highest priority operation from the queue (T038)
func (q *OperationQueue) Dequeue() (*QueuedOperation, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.heap) == 0 {
		return nil, errors.New("queue is empty")
	}

	// Get highest priority operation (index 0)
	op := q.heap[0]
	delete(q.heapMap, op.ID)

	// Remove from heap
	if len(q.heap) == 1 {
		q.heap = q.heap[:0]
	} else {
		q.heap[0] = q.heap[len(q.heap)-1]
		q.heap = q.heap[:len(q.heap)-1]
		q.heapifyDown(0)
	}

	// Update stats
	q.stats.TotalProcessed++
	waitTime := time.Since(op.QueueTime)
	if q.stats.AverageWaitTime == 0 {
		q.stats.AverageWaitTime = waitTime
	} else {
		// Simple moving average
		q.stats.AverageWaitTime = (q.stats.AverageWaitTime*9 + waitTime) / 10
	}

	core.LogCoordinationInfo(fmt.Sprintf("Dequeued operation %s (wait time: %v)", op.ID, waitTime), core.ErrorContext{
		OperationType: "operation_queue_dequeue",
		WaitTime:      waitTime,
	})

	return op, nil
}

// Peek returns the highest priority operation without removing it from the queue (T038)
func (q *OperationQueue) Peek() *QueuedOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.heap) == 0 {
		return nil
	}

	return q.heap[0]
}

// Remove removes an operation from the queue by ID (T038)
func (q *OperationQueue) Remove(operationID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	index, exists := q.findOperationIndex(operationID)
	if !exists {
		return false
	}

	// Remove from heap
	op := q.heap[index]
	delete(q.heapMap, operationID)

	if index == len(q.heap)-1 {
		q.heap = q.heap[:len(q.heap)-1]
	} else {
		q.heap[index] = q.heap[len(q.heap)-1]
		q.heap = q.heap[:len(q.heap)-1]
		q.heapifyDown(index)
	}

	q.stats.TotalFailed++
	q.stats.PriorityStats[op.Priority]--

	core.LogCoordinationInfo(fmt.Sprintf("Removed operation %s from queue", operationID), core.ErrorContext{
		OperationType: "operation_queue_remove",
	})

	return true
}

// UpdatePriority updates the priority of an operation in the queue (T038)
func (q *OperationQueue) UpdatePriority(operationID string, newPriority OperationPriority) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	index, exists := q.findOperationIndex(operationID)
	if !exists {
		return false
	}

	op := q.heap[index]
	oldPriority := op.Priority
	op.Priority = newPriority

	// Update priority stats
	q.stats.PriorityStats[oldPriority]--
	q.stats.PriorityStats[newPriority]++

	// Re-heapify
	q.heapifyUp(index)
	q.heapifyDown(index)

	core.LogCoordinationInfo(fmt.Sprintf("Updated priority for operation %s: %s -> %s", operationID, oldPriority.String(), newPriority.String()), core.ErrorContext{
		OperationType: "operation_queue_priority_update",
	})

	return true
}

// GetStats returns current queue statistics (T038)
func (q *OperationQueue) GetStats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := *q.stats
	stats.QueueDepth = len(q.heap)

	// Deep copy priority stats to prevent external modification
	stats.PriorityStats = make(map[OperationPriority]int64, len(q.stats.PriorityStats))
	for k, v := range q.stats.PriorityStats {
		stats.PriorityStats[k] = v
	}

	return stats
}

// ListOperations returns all operations in the queue (T038)
func (q *OperationQueue) ListOperations() []*QueuedOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	operations := make([]*QueuedOperation, len(q.heap))
	copy(operations, q.heap)
	return operations
}

// GetQueueSize returns the current queue size (T038)
func (q *OperationQueue) GetQueueSize() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.heap)
}

// IsFull returns whether the queue is at capacity (T038)
func (q *OperationQueue) IsFull() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.heap) >= q.sizeLimit
}

// Clear clears all operations from the queue (T038)
func (q *OperationQueue) Clear() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	cleared := len(q.heap)
	q.heap = q.heap[:0]
	for id := range q.heapMap {
		delete(q.heapMap, id)
	}

	core.LogCoordinationInfo(fmt.Sprintf("Cleared %d operations from queue", cleared), core.ErrorContext{
		OperationType: "operation_queue_clear",
	})

	return cleared
}

// Priority queue helper methods (T038)

func (q *OperationQueue) heapifyUp(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if q.compareOperations(index, parent) < 0 {
			q.swap(index, parent)
			index = parent
		} else {
			break
		}
	}
}

func (q *OperationQueue) heapifyDown(index int) {
	size := len(q.heap)
	for {
		left := 2*index + 1
		right := 2*index + 2
		smallest := index

		if left < size && q.compareOperations(left, smallest) < 0 {
			smallest = left
		}
		if right < size && q.compareOperations(right, smallest) < 0 {
			smallest = right
		}

		if smallest != index {
			q.swap(index, smallest)
			index = smallest
		} else {
			break
		}
	}
}

func (q *OperationQueue) compareOperations(i, j int) int {
	op1, op2 := q.heap[i], q.heap[j]

	// First compare by priority
	if op1.Priority != op2.Priority {
		return int(op1.Priority) - int(op2.Priority)
	}

	// Check for stale operations (aging mechanism)
	// Operations waiting > 30 seconds get priority boost
	staleThreshold := 30 * time.Second
	isOp1Stale := time.Since(op1.QueueTime) > staleThreshold
	isOp2Stale := time.Since(op2.QueueTime) > staleThreshold

	if isOp1Stale != isOp2Stale {
		// Stale operation gets priority
		if isOp1Stale {
			return -1
		}
		return 1
	}

	// Then compare by queue time (earlier first)
	if !op1.QueueTime.Equal(op2.QueueTime) {
		return int(op1.QueueTime.Sub(op2.QueueTime))
	}

	// Finally compare by user request (user requests first)
	if op1.UserRequest != op2.UserRequest {
		if op1.UserRequest {
			return -1
		}
		return 1
	}

	return 0
}

func (q *OperationQueue) swap(i, j int) {
	q.heap[i], q.heap[j] = q.heap[j], q.heap[i]
}

func (q *OperationQueue) findOperationIndex(operationID string) (int, bool) {
	for i, op := range q.heap {
		if op.ID == operationID {
			return i, true
		}
	}
	return -1, false
}
