package indexing

import (
	"time"
)

// Legacy types for compatibility with existing goroutine implementation
// These will be removed once the goroutine implementation is replaced

// IndexingError represents an error during indexing
type IndexingError struct {
	File     string `json:"file"`
	FilePath string `json:"file_path"` // Legacy compatibility
	Message  string `json:"message"`
	Stage    string `json:"stage"` // Legacy compatibility
	Error    string `json:"error"` // String for compatibility with existing code
}

// IndexingInProgressError indicates indexing is already in progress
type IndexingInProgressError struct {
	Message  string           `json:"message"`
	Progress IndexingProgress `json:"progress"` // Should be the progress struct
}

func (e *IndexingInProgressError) Error() string {
	return e.Message
}

// IndexingProgress represents progress during indexing
type IndexingProgress struct {
	FilesProcessed    int             `json:"files_processed"`
	TotalFiles        int             `json:"total_files"`
	ElapsedTime       time.Duration   `json:"elapsed_time"`
	CurrentFile       string          `json:"current_file"`
	FilesPerSecond    float64         `json:"files_per_second"`
	EstimatedTimeLeft time.Duration   `json:"estimated_time_left"`
	Errors            []IndexingError `json:"errors"`

	// Progress percentages
	ScanningProgress float64 `json:"scanning_progress"` // 0-100% for file discovery phase
	IndexingProgress float64 `json:"indexing_progress"` // 0-100% for indexing phase
	IsScanning       bool    `json:"is_scanning"`       // true during file discovery
}

// Implementation type identifiers (legacy compatibility)
type IndexImplementationType string

const (
	GoroutineImplementation IndexImplementationType = "goroutine"
)

// IndexStats represents index statistics
type IndexStats struct {
	TotalFiles      int                     `json:"total_files"`
	TotalSymbols    int                     `json:"total_symbols"`
	IndexSize       int64                   `json:"index_size"`
	BuildDuration   time.Duration           `json:"build_duration"`
	LastBuilt       time.Time               `json:"last_built"`
	Implementation  IndexImplementationType `json:"implementation"`
	AdditionalStats map[string]interface{}  `json:"additional_stats,omitempty"`
}

// Feature flags for index implementations (legacy compatibility)
const (
	FeatureConcurrentSearch   = "concurrent_search"
	FeatureConcurrentIndexing = "concurrent_indexing"
	FeatureProgressTracking   = "progress_tracking"
	FeatureGracefulShutdown   = "graceful_shutdown"
	FeatureHotReload          = "hot_reload"
	FeatureIncrementalUpdate  = "incremental_update"
	FeatureMemoryOptimized    = "memory_optimized"
	FeatureHighThroughput     = "high_throughput"
	FeatureRealTimeSearch     = "real_time_search"
	FeatureSymbolAnalysis     = "symbol_analysis"
	FeatureFunctionTree       = "function_tree"
	FeatureReferenceTracking  = "reference_tracking"
)

// allFeatures is a list of all possible features for validation
var allFeatures = []string{
	FeatureConcurrentSearch,
	FeatureConcurrentIndexing,
	FeatureProgressTracking,
	FeatureGracefulShutdown,
	FeatureHotReload,
	FeatureIncrementalUpdate,
	FeatureMemoryOptimized,
	FeatureHighThroughput,
	FeatureRealTimeSearch,
	FeatureSymbolAnalysis,
	FeatureFunctionTree,
	FeatureReferenceTracking,
}

// PerformanceStats represents performance metrics
type PerformanceStats struct {
	AverageSearchTime     float64 `json:"average_search_time"`
	AverageIndexTime      float64 `json:"average_index_time"`
	ThroughputFilesPerSec float64 `json:"throughput_files_per_sec"`
	ConcurrentOperations  int     `json:"concurrent_operations"`
}
