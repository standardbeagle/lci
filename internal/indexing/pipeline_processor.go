// Context: FileProcessor (parse/extract) split from pipeline.go for clarity.
// External deps: config, core.FileService, parser.TreeSitterParser, types; debug logging.
// Prompt-log: See root prompt-log.md (2025-09-05) for session details.
package indexing

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/parser"
	"github.com/standardbeagle/lci/internal/types"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// FileProcessor handles parsing and symbol extraction
type FileProcessor struct {
	config         *config.Config
	parser         *parser.TreeSitterParser
	fileService    *core.FileService
	binaryDetector *BinaryDetector
	ownsParser     bool               // Track if we need to release the parser
	trigramIndex   *core.TrigramIndex // NEW: For bucketing strategy
	// String interning moved to FileIntegrator (global only, no per-file)
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(cfg *config.Config) *FileProcessor {
	return NewFileProcessorWithService(cfg, core.NewFileService())
}

// NewFileProcessorWithStore creates a new file processor with a specific FileContentStore
func NewFileProcessorWithStore(cfg *config.Config, store *core.FileContentStore) *FileProcessor {
	fileService := core.NewFileServiceWithOptions(core.FileServiceOptions{ContentStore: store})
	return NewFileProcessorWithService(cfg, fileService)
}

// NewFileProcessorWithService creates a new file processor with a specific FileService
func NewFileProcessorWithService(cfg *config.Config, fileService *core.FileService) *FileProcessor {
	// Performance optimization: Create a basic processor
	// Workers will get language-specific parsers from pools on-demand
	return &FileProcessor{
		binaryDetector: NewBinaryDetector(),
		config:         cfg,
		fileService:    fileService,
		ownsParser:     false, // We don't own a parser - workers get them from pools
		// String interning moved to FileIntegrator (global only)
	}
}

// Close releases resources owned by the FileProcessor
func (fp *FileProcessor) Close() {
	// FileProcessor no longer owns parsers - they're managed by the pools
	// This is a no-op for now, kept for interface compatibility
}

// SetTrigramIndex sets the trigram index for bucketing strategy
func (fp *FileProcessor) SetTrigramIndex(idx *core.TrigramIndex) {
	fp.trigramIndex = idx
}

// ProcessFiles processes files from the task channel
func (fp *FileProcessor) ProcessFiles(ctx context.Context, workerID int, taskChan <-chan FileTask, resultChan chan<- ProcessedFile) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-taskChan:
			if !ok {
				return // Channel closed
			}

			result := fp.processFile(ctx, workerID, task)

			// Send result with adaptive back-pressure
			select {
			case resultChan <- result:
				// sent
			case <-ctx.Done():
				return
			case <-time.After(taskChannelTimeout):
				log.Printf("Processor %d: result channel full for %s (buffer size: %d), implementing back-pressure", workerID, task.Path, cap(resultChan))
				backoffDelay := taskChannelTimeout
				for retries := 0; retries < 10; retries++ {
					select {
					case resultChan <- result:
						debug.LogIndexing("Processor %d: successfully sent result for %s after %d retries\n", workerID, task.Path, retries+1)
						goto resultSent
					case <-ctx.Done():
						return
					case <-time.After(backoffDelay):
						if retries%3 == 0 {
							log.Printf("Processor %d: retry %d for %s (integrator may be slow)", workerID, retries+1, task.Path)
						}
						backoffDelay = time.Duration(float64(backoffDelay) * 1.5)
						if backoffDelay > 30*time.Second {
							backoffDelay = 30 * time.Second
						}
					}
				}
				log.Printf("Processor %d: WARNING - unable to send result for %s after 10 retries, continuing (integrator severely blocked)", workerID, task.Path)
			}
		resultSent:
			// Yield CPU to reduce UI lag during indexing
			// This allows other goroutines (like Claude Code's UI) to run
			runtime.Gosched()
		}
	}
}

// processFile processes a single file using the new ContentStore architecture
// Note: File content loading is now handled by the integrator, not the processor
func (fp *FileProcessor) processFile(ctx context.Context, workerID int, task FileTask) ProcessedFile {
	start := time.Now()

	result := ProcessedFile{Path: task.Path, FileID: 0, Stage: "parsing", Duration: 0, Language: task.Language}

	// Check for cancellation
	select {
	case <-ctx.Done():
		result.Error = ctx.Err()
		return result
	default:
	}

	// Get language-specific parser from the pool
	// This ensures each worker gets a parser for the right language
	var p *parser.TreeSitterParser
	var releaseParser func(*parser.TreeSitterParser)
	if task.Language != "" && task.Language != "unknown" {
		p = parser.GetParserForLanguage(task.Language, fp.fileService.GetContentStore())
		releaseParser = func(parserInstance *parser.TreeSitterParser) {
			parser.ReleaseParserToPool(parserInstance, parser.Language(task.Language))
		}
	} else {
		// Fallback to shared parser for unknown languages
		p = parser.GetSharedParserWithStore(fp.fileService.GetContentStore())
		releaseParser = func(parserInstance *parser.TreeSitterParser) {
			parser.ReleaseParser(parserInstance)
		}
	}
	// Ensure parser is returned to pool after processing
	// Add panic recovery to ensure parser is always released
	defer func() {
		if p != nil {
			// Recover from any panic to ensure parser is returned
			if r := recover(); r != nil {
				log.Printf("Parser cleanup panic for file %s: %v", task.Path, r)
			}
			releaseParser(p)
		}
	}()

	// In the new architecture, file loading is handled by the integrator
	// The processor should not read files directly - this ensures all content
	// goes through the centralized ContentStore for memory optimization

	// For now, we still need to read content for parsing, but this should be
	// moved to the integrator in a future iteration. The ContentStore integration
	// will be handled by the FileIntegrator which calls LoadFile().

	// TODO: This should be removed when the integrator handles file loading
	// For now, we use LoadFile() to ensure content goes through ContentStore
	fileID, err := fp.fileService.LoadFile(task.Path)
	if err != nil {
		result.Error = fmt.Errorf("failed to load file: %w", err)
		result.Stage = "loading"
		result.Duration = time.Since(start)
		return result
	}

	// Skip directories (LoadFile returns FileID 0 for directories)
	if fileID == 0 {
		result.Error = nil // Not an error, just a directory
		result.Stage = "directory_skipped"
		result.Duration = time.Since(start)
		return result
	}

	// Get content through ContentStore for consistency
	content, ok := fp.fileService.GetFileContent(fileID)
	if !ok {
		result.Error = fmt.Errorf("failed to get content for file: %s", task.Path)
		result.Stage = "content_access"
		result.Duration = time.Since(start)
		return result
	}

	// Defense-in-depth: Check for binary content by magic number
	// Primary check happens during file enumeration (shouldProcessFile) for files > 100KB
	// This is a fallback to catch edge cases (small binary files, files modified between scan and load)
	if fp.binaryDetector != nil && fp.binaryDetector.IsBinaryByMagicNumber(content) {
		result.Error = fmt.Errorf("binary file detected by magic number: %s", task.Path)
		result.Stage = "binary_detection"
		result.Duration = time.Since(start)
		return result
	}

	// Parse file for symbols, references, AST, performance data, and side effects with panic recovery
	var symbols []types.Symbol
	var enhancedSymbols []types.EnhancedSymbol
	var references []types.Reference
	var scopes []types.ScopeInfo
	var ast *tree_sitter.Tree
	var perfData []types.FunctionPerfData
	var sideEffects map[string]*types.SideEffectInfo
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Parser panic for file %s: %v", task.Path, r)
				result.Error = fmt.Errorf("parser panic: %v", r)
			}
		}()
		ast, _, symbols, _, enhancedSymbols, references, scopes, perfData, sideEffects = p.ParseFileWithSideEffects(context.Background(), task.Path, content, fileID)
	}()

	// String interning is done globally in FileIntegrator, not per-file
	// This avoids double allocation (per-file + global)

	// PRE-MERGE OPTIMIZATION: Build scope chains in parallel map phase
	// This moves O(n × s) work from single-threaded reduce to parallel map phase
	// where n=symbols, s=scopes. Each worker builds chains independently.
	// Uses per-file caching to avoid rebuilding identical chains for symbols at same line.
	var scopeChains [][]types.ScopeInfo
	if len(symbols) > 0 && len(scopes) > 0 {
		scopeChains = make([][]types.ScopeInfo, len(symbols))
		// Cache scope chains by line number - symbols on same line have same chain
		lineCache := make(map[int][]types.ScopeInfo, len(symbols)/4+1)
		for i, symbol := range symbols {
			// Check cache first
			if cached, ok := lineCache[symbol.Line]; ok {
				scopeChains[i] = cached
				continue
			}
			// Pre-allocate with capacity of 4 - most symbols match 1-3 scopes
			chain := make([]types.ScopeInfo, 0, 4)
			for _, scope := range scopes {
				if scope.StartLine <= symbol.Line && (scope.EndLine == 0 || scope.EndLine >= symbol.Line) {
					chain = append(chain, scope)
				}
			}
			scopeChains[i] = chain
			lineCache[symbol.Line] = chain
		}
	}

	// Pre-compute trigrams in parallel processors using bucketed format only
	// OPTIMIZATION: Bucketed format enables lock-free merging and eliminates duplicate generation
	var bucketedTrigrams *core.BucketedTrigramResult

	if len(content) >= 3 {
		// Generate bucketed trigram format
		bucketedTrigrams = fp.trigramIndex.CreateBucketedResult(fileID)
		bucketCount := fp.trigramIndex.GetBucketCount()

		// OPTIMIZED CAPACITY ESTIMATION: Size maps appropriately to reduce rehashing
		// For small files, use smaller maps. For large files, estimate based on content.
		var estimatedPerBucket int
		if len(content) < 512 {
			// Small files: use smaller initial capacity to reduce memory
			estimatedPerBucket = 4
		} else {
			// Larger files: estimate based on unique trigrams (content/10) distributed across buckets
			estimatedUniqueTrigrams := len(content) / 10
			if estimatedUniqueTrigrams < 100 {
				estimatedUniqueTrigrams = 100
			}
			estimatedPerBucket = (estimatedUniqueTrigrams / bucketCount) + 2
			if estimatedPerBucket < 8 {
				estimatedPerBucket = 8
			}
		}

		// Pre-allocate bucket maps
		for i := 0; i < bucketCount; i++ {
			bucketedTrigrams.Buckets[i].Trigrams = make(map[uint32][]uint32, estimatedPerBucket)
		}

		// Single pass: populate bucketed format
		for i := 0; i <= len(content)-3; i++ {
			trigram := uint32(content[i])<<16 | uint32(content[i+1])<<8 | uint32(content[i+2])
			offset := uint32(i)
			bucketID := fp.trigramIndex.GetBucketForTrigram(trigram)
			bucketedTrigrams.Buckets[bucketID].Trigrams[trigram] = append(
				bucketedTrigrams.Buckets[bucketID].Trigrams[trigram],
				offset,
			)
		}
	} else {
		// Empty content or too short for trigrams
		bucketedTrigrams = fp.trigramIndex.CreateBucketedResult(fileID)
	}

	result.FileID = fileID
	result.Symbols = symbols
	result.EnhancedSymbols = enhancedSymbols // Includes complexity data from parser
	result.References = references
	result.Scopes = scopes
	result.ScopeChains = scopeChains // Pre-built in map phase for lock-free reduce
	result.Content = content                               // Keep content for integrator compatibility
	result.LineOffsets = types.ComputeLineOffsets(content) // Precompute for O(1) line access
	result.AST = ast
	result.Language = filepath.Ext(task.Path)
	result.BucketedTrigrams = bucketedTrigrams // Pre-sharded format for lock-free merging
	result.PerfData = perfData                 // Performance analysis data for anti-pattern detection
	result.SideEffectResults = sideEffects     // Side effect analysis for purity detection
	result.Stage = "completed"
	result.Duration = time.Since(start)
	return result
}
