package indexing

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/standardbeagle/lci/testhelpers"
)

// IndexState represents the loading state of an index
type IndexState int

const (
	StateEmpty           IndexState = iota // No files indexed
	StatePartial                           // Some files indexed (INVALID for search tests)
	StateFullyIndexed                      // All files processed and integrated
	StateReferencesBuilt                   // Cross-file references resolved
	StateSemanticReady                     // Semantic indexes built
	StateFinalReady                        // All post-processing complete
)

func (s IndexState) String() string {
	switch s {
	case StateEmpty:
		return "Empty"
	case StatePartial:
		return "Partial"
	case StateFullyIndexed:
		return "FullyIndexed"
	case StateReferencesBuilt:
		return "ReferencesBuilt"
	case StateSemanticReady:
		return "SemanticReady"
	case StateFinalReady:
		return "FinalReady"
	default:
		return "Unknown"
	}
}

// TestCorpus defines a test corpus with expected performance characteristics
type TestCorpus struct {
	Name           string
	Root           string
	FileCount      int
	LOC            int
	ExpectedIndex  time.Duration
	ExpectedSearch time.Duration
}

var (
	// CorpusSmall: Fast tests, basic functionality
	CorpusSmall = TestCorpus{
		Name:           "small",
		Root:           "testdata/corpus-small",
		FileCount:      100,
		LOC:            10_000,
		ExpectedIndex:  500 * time.Millisecond,
		ExpectedSearch: 5 * time.Millisecond,
	}

	// CorpusMedium: Performance testing, regression detection
	CorpusMedium = TestCorpus{
		Name:           "medium",
		Root:           "testdata/corpus-medium",
		FileCount:      1_000,
		LOC:            100_000,
		ExpectedIndex:  2 * time.Second,
		ExpectedSearch: 20 * time.Millisecond,
	}

	// CorpusLarge: Scalability testing, production simulation
	CorpusLarge = TestCorpus{
		Name:           "large",
		Root:           "testdata/corpus-large",
		FileCount:      10_000,
		LOC:            1_000_000,
		ExpectedIndex:  10 * time.Second,
		ExpectedSearch: 50 * time.Millisecond,
	}
)

// SafetyMargins for timeout calculations
const (
	SafetyMarginCPU    = 3.0 // 3x baseline for CPU-bound operations
	SafetyMarginIO     = 5.0 // 5x baseline for IO-bound operations
	SafetyMarginMemory = 1.5 // 1.5x baseline for memory
)

// VerifyIndexState ensures index is in expected state before testing.
//
// This is CRITICAL for search tests because:
// 1. Performance characteristics change with index size
// 2. Memory patterns differ between empty and full indexes
// 3. Cache effects only emerge with realistic data
// 4. Concurrency bottlenecks only visible under full load
func VerifyIndexState(t *testing.T, indexer *MasterIndex, required IndexState) {
	t.Helper()

	state := GetIndexState(indexer)

	if state < required {
		t.Fatalf("Index not ready: got state %v, required %v\n"+
			"Search tests MUST run on fully loaded indexes.\n"+
			"See: docs/testing-performance-baselines.md",
			state, required)
	}

	// Log state for debugging
	t.Logf("Index state verified: %v (required: %v)", state, required)
	t.Logf("  Files indexed: %d", indexer.GetFileCount())
	t.Logf("  Symbols indexed: %d", indexer.GetSymbolCount())
	t.Logf("  References built: %v", state >= StateReferencesBuilt)
}

// GetIndexState determines current index state
func GetIndexState(indexer *MasterIndex) IndexState {
	if indexer.GetFileCount() == 0 {
		return StateEmpty
	}

	// For now, if we have files, assume StateFullyIndexed
	// In the future, add methods to check:
	// - IsIndexingComplete()
	// - AreReferencesProcessed()
	// - IsSemanticIndexReady()
	// - IsPostProcessingComplete()

	// Basic check: if we have files and symbols, we're at least fully indexed
	if indexer.GetSymbolCount() > 0 {
		return StateFullyIndexed
	}

	// If we have files but no symbols, might be in partial state
	return StatePartial
}

// SetupFullyLoadedIndex creates an index in final ready state with performance tracking.
//
// This helper ensures:
// 1. Index is fully loaded before tests begin
// 2. Performance characteristics are logged for baseline tracking
// 3. Memory usage is tracked
// 4. State is verified before returning
func SetupFullyLoadedIndex(t *testing.T, rootPath string) *MasterIndex {
	t.Helper()

	cfg := testhelpers.NewTestConfigBuilder(rootPath).Build()
	indexer := NewMasterIndex(cfg)

	ctx := context.Background()

	// Track memory before indexing
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	beforeMem := m1.Alloc

	// Profile indexing time
	start := time.Now()
	if err := indexer.IndexDirectory(ctx, rootPath); err != nil {
		t.Fatalf("Failed to index: %v", err)
	}
	indexTime := time.Since(start)

	// Track memory after indexing
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	afterMem := m2.Alloc
	memUsed := afterMem - beforeMem

	// Verify state
	VerifyIndexState(t, indexer, StateFullyIndexed)

	// Log for baseline tracking
	t.Logf("=== Index Loading Performance ===")
	t.Logf("  Time: %v", indexTime)
	t.Logf("  Files: %d", indexer.GetFileCount())
	t.Logf("  Symbols: %d", indexer.GetSymbolCount())
	t.Logf("  Memory: %.2f MB", float64(memUsed)/(1024*1024))
	t.Logf("  Files/sec: %.0f", float64(indexer.GetFileCount())/indexTime.Seconds())
	t.Logf("=================================")

	return indexer
}

// SetupFullyLoadedIndexWithCorpus creates an index using a standard test corpus
func SetupFullyLoadedIndexWithCorpus(t *testing.T, corpus TestCorpus) *MasterIndex {
	t.Helper()

	indexer := SetupFullyLoadedIndex(t, corpus.Root)

	// Verify file count matches expected corpus
	if indexer.GetFileCount() != corpus.FileCount {
		t.Logf("WARNING: Corpus file count mismatch: expected %d, got %d",
			corpus.FileCount, indexer.GetFileCount())
	}

	return indexer
}

// SelectTestCorpus chooses appropriate corpus based on test flags
func SelectTestCorpus(t *testing.T) TestCorpus {
	t.Helper()

	if testing.Short() {
		t.Logf("Using small corpus (short tests enabled)")
		return CorpusSmall
	}

	// Could add more sophisticated selection logic here
	// For now, default to medium
	t.Logf("Using medium corpus")
	return CorpusMedium
}

// CalculateTimeout calculates appropriate timeout based on expected duration and operation type
func CalculateTimeout(expected time.Duration, isCPUBound bool) time.Duration {
	margin := SafetyMarginIO
	if isCPUBound {
		margin = SafetyMarginCPU
	}
	return time.Duration(float64(expected) * margin)
}

// MustNotTimeout fails the test if context deadline is exceeded
func MustNotTimeout(t *testing.T, err error, operation string) {
	t.Helper()

	if err == context.DeadlineExceeded {
		t.Fatalf("%s exceeded profiled timeout - performance regression detected\n"+
			"This indicates the operation is slower than baseline.\n"+
			"Run profiling to establish new baseline: go test -cpuprofile=cpu.prof\n"+
			"See: docs/testing-performance-baselines.md",
			operation)
	}
}

// IndexMetrics captures index performance metrics for baseline tracking
type IndexMetrics struct {
	IndexTime     time.Duration
	FileCount     int
	SymbolCount   int
	MemoryUsed    uint64
	FilesPerSec   float64
	SymbolsPerSec float64
}

// CaptureIndexMetrics captures and logs index performance metrics
func CaptureIndexMetrics(t *testing.T, indexer *MasterIndex, indexTime time.Duration, memUsed uint64) IndexMetrics {
	t.Helper()

	metrics := IndexMetrics{
		IndexTime:     indexTime,
		FileCount:     indexer.GetFileCount(),
		SymbolCount:   indexer.GetSymbolCount(),
		MemoryUsed:    memUsed,
		FilesPerSec:   float64(indexer.GetFileCount()) / indexTime.Seconds(),
		SymbolsPerSec: float64(indexer.GetSymbolCount()) / indexTime.Seconds(),
	}

	t.Logf("=== Index Metrics ===")
	t.Logf("  Index Time: %v", metrics.IndexTime)
	t.Logf("  Files: %d (%.0f files/sec)", metrics.FileCount, metrics.FilesPerSec)
	t.Logf("  Symbols: %d (%.0f symbols/sec)", metrics.SymbolCount, metrics.SymbolsPerSec)
	t.Logf("  Memory: %.2f MB", float64(metrics.MemoryUsed)/(1024*1024))
	t.Logf("=====================")

	return metrics
}
