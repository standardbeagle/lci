// Context: Core pipeline type definitions and constants extracted from pipeline.go for maintainability.
// External deps: config, core, types, tree-sitter for AST references.
// Prompt-log: See root prompt-log.md for session details (2025-09-05).
package indexing

import (
	"os"
	"runtime"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/bmatcuk/doublestar/v4"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Pipeline configuration constants
const (
	// Timeout for sending tasks through channels
	taskChannelTimeout = 5 * time.Second

	// Base buffer size multipliers (minimum values)
	taskChannelBufferBaseMultiplier   = 8
	resultChannelBufferBaseMultiplier = 16

	// Progress estimation constants
	estimatedScanningProgress = 50.0 // Conservative estimate when scanning files

	// Channel buffer caps to prevent excessive memory usage
	maxTaskChannelBuffer   = 1000
	maxResultChannelBuffer = 2000
)

// calculateOptimalChannelBuffers dynamically calculates channel buffer sizes
// based on CPU count and expected workload to optimize throughput
func calculateOptimalChannelBuffers(fileCount int) (taskBuffer, resultBuffer int) {
	cpuCount := runtime.NumCPU()

	// Task channel: scale with file count, min 8 per CPU
	taskBuffer = max(cpuCount*taskChannelBufferBaseMultiplier, fileCount/20)
	if taskBuffer > maxTaskChannelBuffer {
		taskBuffer = maxTaskChannelBuffer
	}

	// Result channel: needs larger buffer for processing results
	// Results may pile up if integrator is slower than processors
	resultBuffer = max(cpuCount*resultChannelBufferBaseMultiplier, fileCount/10)
	if resultBuffer > maxResultChannelBuffer {
		resultBuffer = maxResultChannelBuffer
	}

	return taskBuffer, resultBuffer
}

// FileTask represents a file to be processed in the pipeline
type FileTask struct {
	Path     string
	Info     os.FileInfo
	Language string // File language (go, python, typescript, etc.) for parser selection
	Priority int    // Higher priority files processed first
}

// ProcessedFile represents the result of processing a file
type ProcessedFile struct {
	Path             string
	FileID           types.FileID
	Symbols          []types.Symbol
	EnhancedSymbols  []types.EnhancedSymbol      // Enhanced symbols with complexity data
	References       []types.Reference           // extracted references
	Scopes           []types.ScopeInfo           // scope information
	ScopeChains      [][]types.ScopeInfo         // Pre-computed scope chains per symbol (indexed same as Symbols)
	LineToSymbols    map[int][]int               // Pre-computed line->symbol indices for O(1) semantic filtering
	BucketedTrigrams *core.BucketedTrigramResult // Pre-sharded trigrams for lock-free merging
	Content          []byte                      // file content for metrics calculation
	LineOffsets      []int                       // precomputed line boundaries for O(1) line access
	AST              *tree_sitter.Tree           // parsed AST for Tree-sitter queries
	Language         string                      // file language extension (.go, .js, etc.)
	Error            error
	Stage            string // "scanning", "parsing", "indexing"
	Duration         time.Duration
	// Semantic data collected during map phase
	SemanticData *SemanticMapResult // Pre-computed semantic data for all symbols
	// Performance analysis data collected during AST parsing
	PerfData []types.FunctionPerfData
	// Side effect analysis results keyed by "file:line"
	SideEffectResults map[string]*types.SideEffectInfo
}

// FileScanner handles directory traversal and file discovery
type FileScanner struct {
	config          *config.Config
	bufferSize      int
	gitignoreParser *config.GitignoreParser
	binaryDetector  *BinaryDetector
	// Pre-compiled glob patterns for fast matching
	compiledExclusions []string // Pattern strings (doublestar compiles internally)
	compiledInclusions []string // Pattern strings (doublestar compiles internally)
}

// compilePatterns pre-compiles exclusion and inclusion patterns for fast matching
func (fs *FileScanner) compilePatterns() {
	fs.compiledExclusions = make([]string, 0, len(fs.config.Exclude))
	fs.compiledExclusions = append(fs.compiledExclusions, fs.config.Exclude...)

	fs.compiledInclusions = make([]string, 0, len(fs.config.Include))
	fs.compiledInclusions = append(fs.compiledInclusions, fs.config.Include...)
}

// shouldExcludeFast checks if a path matches any exclusion pattern using fast doublestar matching
func (fs *FileScanner) shouldExcludeFast(path string) bool {
	for _, pattern := range fs.compiledExclusions {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			// Log error but continue - bad pattern shouldn't break scanning
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// shouldIncludeFast checks if a path matches any inclusion pattern using fast doublestar matching
func (fs *FileScanner) shouldIncludeFast(path string) bool {
	// If no inclusion patterns, include everything
	if len(fs.compiledInclusions) == 0 {
		return true
	}

	for _, pattern := range fs.compiledInclusions {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			// Log error but continue - bad pattern shouldn't break scanning
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
