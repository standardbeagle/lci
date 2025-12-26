package core

import (
	"context"
	"sync"
	"time"
)

// IndexType represents different types of indexes that can be managed independently
type IndexType int

const (
	// TrigramIndexType - Trigram pattern matching index
	TrigramIndexType IndexType = iota
	// SymbolIndexType - Symbol definitions and references index
	SymbolIndexType
	// ReferenceIndexType - Cross-reference tracking index
	ReferenceIndexType
	// CallGraphIndexType - Call hierarchy and relationship index
	CallGraphIndexType
	// PostingsIndexType - Word posting lists index
	PostingsIndexType
	// LocationIndexType - File location indexing
	LocationIndexType
	// ContentIndexType - Full content search index
	ContentIndexType
)

// String returns the string representation of IndexType
func (it IndexType) String() string {
	switch it {
	case TrigramIndexType:
		return "TrigramIndexType"
	case SymbolIndexType:
		return "SymbolIndexType"
	case ReferenceIndexType:
		return "ReferenceIndexType"
	case CallGraphIndexType:
		return "CallGraphIndexType"
	case PostingsIndexType:
		return "PostingsIndexType"
	case LocationIndexType:
		return "LocationIndexType"
	case ContentIndexType:
		return "ContentIndexType"
	default:
		return "UnknownIndex"
	}
}

// IsValid checks if the IndexType is valid
func (it IndexType) IsValid() bool {
	return it >= TrigramIndexType && it <= ContentIndexType
}

// Int32 returns the int32 representation of IndexType
func (it IndexType) Int32() int32 {
	return int32(it)
}

// LockType represents the type of lock operation
type LockType int

const (
	// ReadLock - Shared lock for read operations
	ReadLock LockType = iota
	// WriteLock - Exclusive lock for write operations
	WriteLock
)

// String returns the string representation of LockType
func (lt LockType) String() string {
	switch lt {
	case ReadLock:
		return "ReadLock"
	case WriteLock:
		return "WriteLock"
	default:
		return "UnknownLock"
	}
}

// LockRelease is a function type that releases acquired locks
type LockRelease func()

// IndexStatus represents the current status of an index
type IndexStatus struct {
	Type        IndexType `json:"type"`
	IsIndexing  bool      `json:"isIndexing"`
	LastUpdate  time.Time `json:"lastUpdate"`
	UpdateCount int64     `json:"updateCount"`
	LockHolders int       `json:"lockHolders"`
	QueueDepth  int       `json:"queueDepth"`
}

// IndexCoordinationConfig represents configuration for index coordination
type IndexCoordinationConfig struct {
	// Default timeout for lock acquisition
	DefaultLockTimeout time.Duration
	// Maximum number of concurrent operations per index
	MaxConcurrentOps int
	// Enable adaptive timeout based on system load
	AdaptiveTimeout bool
	// Lock metrics collection interval
	MetricsInterval time.Duration
	// Maximum queue depth for index operations
	MaxQueueDepth int
}

// DefaultIndexCoordinationConfig returns default configuration for index coordination
func DefaultIndexCoordinationConfig() *IndexCoordinationConfig {
	return &IndexCoordinationConfig{
		DefaultLockTimeout: 5 * time.Second,
		MaxConcurrentOps:   1000,
		AdaptiveTimeout:    true,
		MetricsInterval:    10 * time.Second,
		MaxQueueDepth:      100,
	}
}

// OperationType represents the type of indexing operation
type OperationType int

const (
	// OpUpdate - Incremental update operation
	OpUpdate OperationType = iota
	// OpRebuild - Full rebuild operation
	OpRebuild
	// OpIncremental - Incremental update with dependency tracking
	OpIncremental
)

// String returns the string representation of OperationType
func (ot OperationType) String() string {
	switch ot {
	case OpUpdate:
		return "Update"
	case OpRebuild:
		return "Rebuild"
	case OpIncremental:
		return "Incremental"
	default:
		return "UnknownOperation"
	}
}

// IndexOperation represents an active indexing operation
type IndexOperation struct {
	ID                 string              `json:"id"`
	IndexType          IndexType           `json:"indexType"`
	OperationType      IndexUpdateType     `json:"operationType"`
	Status             IndexOpStatus       `json:"status"`
	Progress           float64             `json:"progress"`
	StartTime          time.Time           `json:"startTime"`
	EndTime            time.Time           `json:"endTime,omitempty"`
	Timeout            time.Duration       `json:"timeout"`
	Priority           int                 `json:"priority"`
	Context            *context.CancelFunc `json:"-"`
	CancelFunc         func()              `json:"-"`
	LastProgressUpdate time.Time           `json:"lastProgressUpdate"`
}

// IndexOpStatus represents the status of an index operation
type IndexOpStatus int

const (
	// IndexOperationQueued - Operation is queued and waiting to start
	IndexOperationQueued IndexOpStatus = iota
	// IndexOperationRunning - Operation is currently running
	IndexOperationRunning
	// IndexOperationCompleted - Operation completed successfully
	IndexOperationCompleted
	// IndexOperationFailed - Operation failed
	IndexOperationFailed
	// IndexOperationCancelled - Operation was cancelled
	IndexOperationCancelled
	// IndexOperationTimeout - Operation timed out
	IndexOperationTimeout
)

// String returns the string representation of IndexOpStatus
func (ios IndexOpStatus) String() string {
	switch ios {
	case IndexOperationQueued:
		return "Queued"
	case IndexOperationRunning:
		return "Running"
	case IndexOperationCompleted:
		return "Completed"
	case IndexOperationFailed:
		return "Failed"
	case IndexOperationCancelled:
		return "Cancelled"
	case IndexOperationTimeout:
		return "Timeout"
	default:
		return "UnknownStatus"
	}
}

// IsCompleted returns true if the operation is in a completed state
func (ios IndexOpStatus) IsCompleted() bool {
	return ios == IndexOperationCompleted ||
		ios == IndexOperationFailed ||
		ios == IndexOperationCancelled ||
		ios == IndexOperationTimeout
}

// SearchOperation represents an active search operation
type SearchOperation struct {
	ID            string             `json:"id"`
	Requirements  SearchRequirements `json:"requirements"`
	AcquiredLocks []IndexType        `json:"acquiredLocks"`
	StartTime     time.Time          `json:"startTime"`
	Timeout       time.Duration      `json:"timeout"`
}

// Global registry for coordination
var (
	coordinationMu     sync.RWMutex
	coordinationConfig *IndexCoordinationConfig = DefaultIndexCoordinationConfig()
)

// SetCoordinationConfig updates the global coordination configuration
func SetCoordinationConfig(config *IndexCoordinationConfig) {
	coordinationMu.Lock()
	defer coordinationMu.Unlock()
	coordinationConfig = config
}

// GetCoordinationConfig returns the current global coordination configuration
func GetCoordinationConfig() *IndexCoordinationConfig {
	coordinationMu.RLock()
	defer coordinationMu.RUnlock()
	return coordinationConfig
}

// GetAllIndexTypes returns all available index types
func GetAllIndexTypes() []IndexType {
	return []IndexType{
		TrigramIndexType,
		SymbolIndexType,
		ReferenceIndexType,
		CallGraphIndexType,
		PostingsIndexType,
		LocationIndexType,
		ContentIndexType,
	}
}

// IndexHealth represents the health status of an index
type IndexHealth struct {
	IndexType     IndexType        `json:"indexType"`
	Status        HealthStatus     `json:"status"`
	LastChecked   time.Time        `json:"lastChecked"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
	QueueSize     int              `json:"queueSize"`
	OperationRate float64          `json:"operationRate"` // operations per second
	ErrorRate     float64          `json:"errorRate"`     // errors per second
	ResponseTime  time.Duration    `json:"responseTime"`
	LastUpdate    time.Time        `json:"lastUpdate"`
	UpdateCount   int64            `json:"updateCount"`
	ErrorCount    int              `json:"errorCount"`
	Availability  float64          `json:"availability"` // percentage
	IsAvailable   bool             `json:"isAvailable"`
	Performance   IndexPerformance `json:"performance"`
	LastError     error            `json:"-"` // not JSON serializable
	LastErrorTime time.Time        `json:"lastErrorTime"`
}

// IndexPerformance represents performance metrics for an index
type IndexPerformance struct {
	AverageResponseTime time.Duration `json:"averageResponseTime"`
	LockWaitTime        time.Duration `json:"lockWaitTime"`
	ContentionRate      float64       `json:"contentionRate"`
}

// HealthStatus represents the health status of an index
type HealthStatus int

const (
	// HealthStatusHealthy - Index is operating normally
	HealthStatusHealthy HealthStatus = iota
	// HealthStatusDegraded - Index is operational but with performance issues
	HealthStatusDegraded
	// HealthStatusUnhealthy - Index is not functioning properly
	HealthStatusUnhealthy
	// HealthStatusFailed - Index has failed and requires recovery
	HealthStatusFailed
	// HealthStatusUnknown - Health status could not be determined
	HealthStatusUnknown
)

// String returns the string representation of HealthStatus
func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "Healthy"
	case HealthStatusDegraded:
		return "Degraded"
	case HealthStatusUnhealthy:
		return "Unhealthy"
	case HealthStatusFailed:
		return "Failed"
	case HealthStatusUnknown:
		return "Unknown"
	default:
		return "UnknownStatus"
	}
}

// IndexUpdateType represents the type of index update operation
type IndexUpdateType int

const (
	// IndexUpdateTypeIncremental - Incremental update with new/changed files
	IndexUpdateTypeIncremental IndexUpdateType = iota
	// IndexUpdateTypeFull - Full index rebuild
	IndexUpdateTypeFull
	// IndexUpdateTypeRecovery - Recovery from failed state
	IndexUpdateTypeRecovery
	// IndexUpdateTypeValidation - Index validation and consistency check
	IndexUpdateTypeValidation
)

// String returns the string representation of IndexUpdateType
func (iut IndexUpdateType) String() string {
	switch iut {
	case IndexUpdateTypeIncremental:
		return "Incremental"
	case IndexUpdateTypeFull:
		return "Full"
	case IndexUpdateTypeRecovery:
		return "Recovery"
	case IndexUpdateTypeValidation:
		return "Validation"
	default:
		return "UnknownUpdate"
	}
}

// IndexUpdateOptions represents options for index update operations
type IndexUpdateOptions struct {
	// Force rebuild even if index appears up to date
	Force bool
	// Files to include in the update (empty means all files)
	Files []string
	// Priority of the update operation
	Priority int
	// Timeout for the update operation
	Timeout time.Duration
	// Whether to validate after update
	Validate bool
	// Callback function for progress updates
	ProgressCallback func(progress float64)
}

// DefaultIndexUpdateOptions returns default options for index updates
func DefaultIndexUpdateOptions() IndexUpdateOptions {
	return IndexUpdateOptions{
		Force:            false,
		Files:            nil,
		Priority:         5,
		Timeout:          30 * time.Minute,
		Validate:         true,
		ProgressCallback: nil,
	}
}

// IndexOperationResult represents the result of an index operation
type IndexOperationResult struct {
	OperationID    string           `json:"operationID"`
	Success        bool             `json:"success"`
	OperationType  IndexUpdateType  `json:"operationType"`
	IndexType      IndexType        `json:"indexType"`
	Status         IndexOpStatus    `json:"status"`
	Progress       float64          `json:"progress"`
	Duration       time.Duration    `json:"duration"`
	FilesProcessed int              `json:"filesProcessed"`
	BytesProcessed int64            `json:"bytesProcessed"`
	Error          string           `json:"error,omitempty"`
	Metrics        OperationMetrics `json:"metrics"`
}

// OperationMetrics represents detailed metrics for an operation
type OperationMetrics struct {
	IndexTime      time.Duration `json:"indexTime"`
	ValidationTime time.Duration `json:"validationTime"`
	CleanupTime    time.Duration `json:"cleanupTime"`
	MemoryPeak     int64         `json:"memoryPeak"`
	GoroutineCount int           `json:"goroutineCount"`
}

// QueueStatus represents the status of operation queues
type QueueStatus struct {
	IndexType       IndexType     `json:"indexType"`
	PendingOps      int           `json:"pendingOps"`
	RunningOps      int           `json:"runningOps"`
	CompletedOps    int64         `json:"completedOps"`
	FailedOps       int64         `json:"failedOps"`
	AverageWait     time.Duration `json:"averageWait"`
	AverageRuntime  time.Duration `json:"averageRuntime"`
	QueueDepth      int           `json:"queueDepth"`
	ActiveOps       int           `json:"activeOps"`
	MaxQueueDepth   int           `json:"maxQueueDepth"`
	AverageWaitTime time.Duration `json:"averageWaitTime"`
}

// HealthMonitor interface for index health monitoring
type HealthMonitor interface {
	// CheckHealth checks the health of a specific index
	CheckHealth(indexType IndexType) IndexHealth

	// StartMonitoring begins continuous health monitoring
	StartMonitoring(interval time.Duration) error

	// StopMonitoring stops continuous health monitoring
	StopMonitoring() error

	// GetHealthHistory returns historical health data
	GetHealthHistory(indexType IndexType, duration time.Duration) []IndexHealth

	// RegisterHealthCallback registers a callback for health changes
	RegisterHealthCallback(callback func(IndexHealth)) error

	// UnregisterHealthCallback removes a health change callback
	UnregisterHealthCallback(callbackID string) error
}

// OperationPriority represents the priority level of an operation
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
		return "UnknownPriority"
	}
}
