// Context: FileIntegrator (integrate/index) split from pipeline.go for clarity.
// External deps: analysis, core indexes, ref tracker, call graph; debug logging.
// Prompt-log: See root prompt-log.md (2025-09-05) for session details.
package indexing

import (
	"bytes"
	"context"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/standardbeagle/lci/internal/analysis"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/semantic"
	"github.com/standardbeagle/lci/internal/types"
)

// Object pool for enhanced symbol pointer slices to reduce allocations
// with maximum capacity to prevent unlimited growth
var enhancedSymbolPtrPool = sync.Pool{
	New: func() interface{} {
		// Return a slice with reasonable initial capacity
		// Maximum capacity enforced when returning to pool
		return make([]*types.EnhancedSymbol, 0, 32)
	},
}

// Enhanced symbol pointer pool constants
const (
	maxSymbolPtrCapacity = 512 // Maximum capacity to prevent unlimited growth
	minSymbolPtrCapacity = 16  // Minimum capacity to maintain
)

// calculateSimpleMetrics provides fast, basic metrics without AST analysis
func (fi *FileIntegrator) calculateSimpleMetrics(symbol *types.EnhancedSymbol, content []byte) map[string]interface{} {
	if symbol == nil {
		return nil
	}

	// Optimize: Use bytes.Count to count lines instead of strings.Split
	// strings.Split creates a new slice for each line, causing massive allocations
	// bytes.Count just counts newlines without allocation
	// Handle edge case: files without trailing newline
	newlineCount := bytes.Count(content, []byte("\n"))
	var totalLines int
	if len(content) == 0 {
		totalLines = 0
	} else if len(content) > 0 && content[len(content)-1] == '\n' {
		// File ends with newline, count matches line count
		totalLines = newlineCount
	} else {
		// File doesn't end with newline, lines = newlines + 1
		totalLines = newlineCount + 1
	}

	symbolStartLine := int(symbol.Symbol.Line)
	symbolEndLine := int(symbol.Symbol.EndLine)
	if symbolEndLine <= symbolStartLine {
		symbolEndLine = symbolStartLine + 10
	}
	linesOfCode := symbolEndLine - symbolStartLine + 1
	if linesOfCode > totalLines {
		linesOfCode = totalLines
	}
	complexity := 1.0
	symbolName := symbol.Symbol.Name
	lower := strings.ToLower(symbolName)
	if strings.Contains(lower, "test") {
		complexity += 2.0
	}
	if strings.Contains(lower, "handle") || strings.Contains(lower, "process") {
		complexity += 3.0
	}
	if linesOfCode > 50 {
		complexity += 2.0
	} else if linesOfCode > 20 {
		complexity += 1.0
	}
	refCount := len(symbol.IncomingRefs) + len(symbol.OutgoingRefs)
	complexity += float64(refCount) * 0.1
	return map[string]interface{}{
		"complexity":      complexity,
		"lines_of_code":   linesOfCode,
		"reference_count": refCount,
		"risk_score":      fi.calculateRiskScore(complexity, float64(linesOfCode), float64(refCount)),
		"type":            symbol.Symbol.Type.String(),
	}
}

// calculateRiskScore provides a simple risk assessment for editing safety
func (fi *FileIntegrator) calculateRiskScore(complexity, linesOfCode, refCount float64) float64 {
	riskScore := 0.0
	riskScore += complexity * 0.5
	if linesOfCode > 100 {
		riskScore += 3.0
	} else if linesOfCode > 50 {
		riskScore += 1.5
	}
	if refCount > 10 {
		riskScore += 2.0
	} else if refCount > 5 {
		riskScore += 1.0
	}
	if riskScore > 10.0 {
		riskScore = 10.0
	}
	return riskScore
}

// ScopeStore interface for storing file scope information
type ScopeStore interface {
	StoreFileScopes(fileID types.FileID, scopes []types.ScopeInfo)
}

// FileIntegrator handles integration of processed files into indexes
type FileIntegrator struct {
	trigramIndex        *core.TrigramIndex
	symbolIndex         *core.SymbolIndex
	refTracker          *core.ReferenceTracker
	semanticSearchIndex *core.SemanticSearchIndex // Pre-computed semantic search optimizations
	// astStore removed - using metadata index instead of AST storage
	symbolLocationIndex *core.SymbolLocationIndex
	fileContentStore    *core.FileContentStore
	fileSearchEngine    *core.FileSearchEngine
	metricsCalculator   *analysis.BasicMetricsCalculator
	// universalGraph removed (no longer supported)
	config *config.Config // Config for feature flags

	fileIDCounter uint32
	mu            sync.Mutex

	fileMap map[string]types.FileID

	// NEW: Track processed files for semantic reduce phase
	processedFiles []*ProcessedFile // Store all processed files for semantic reduction (single-writer, no mutex needed)
	reverseFileMap map[types.FileID]string
	fileMapMu      *sync.RWMutex

	scopeStore ScopeStore

	// NEW: Channel-based merger pipeline for lock-free trigram indexing
	mergerPipeline    *TrigramMergerPipeline
	useMergerPipeline bool // Feature flag to enable/disable merger pipeline

	// Side effect propagator for function purity analysis
	sideEffectPropagator *core.SideEffectPropagator
}

// SpecializedIntegrator handles a specific subset of indexing operations in parallel
type SpecializedIntegrator struct {
	name             string
	fileContentStore *core.FileContentStore
	fileMap          map[string]types.FileID
	reverseFileMap   map[types.FileID]string
	fileMapMu        *sync.RWMutex
	fileIDCounter    uint32
	mu               sync.Mutex
}

// NewSpecializedIntegrator creates a specialized integrator for specific operations
func NewSpecializedIntegrator(name string, fileContentStore *core.FileContentStore, fileMap map[string]types.FileID, reverseFileMap map[types.FileID]string, fileMapMu *sync.RWMutex) *SpecializedIntegrator {
	return &SpecializedIntegrator{
		name:             name,
		fileContentStore: fileContentStore,
		fileMap:          fileMap,
		reverseFileMap:   reverseFileMap,
		fileMapMu:        fileMapMu,
	}
}

// ProcessBasics handles basic file storage and mapping (fastest operations)
func (si *SpecializedIntegrator) ProcessBasics(ctx context.Context, resultChan <-chan ProcessedFile, basicsChan chan<- ProcessedFile) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-resultChan:
			if !ok {
				return
			}
			if result.Error != nil {
				basicsChan <- result
				continue
			}
			// Use FileID from processor if already loaded, otherwise load here
			var fileID types.FileID
			if result.FileID != 0 {
				// File was already loaded by processor using LoadFile()
				fileID = result.FileID
			} else if len(result.Content) > 0 && si.fileContentStore != nil {
				// Legacy path: load file directly in integrator
				fileID = si.fileContentStore.LoadFile(result.Path, result.Content)
			} else {
				// Generate FileID for empty files
				fileID = types.FileID(atomic.AddUint32(&si.fileIDCounter, 1))
			}
			result.FileID = fileID
			if si.fileMap != nil && si.reverseFileMap != nil {
				si.fileMapMu.Lock()
				si.fileMap[result.Path] = fileID
				si.reverseFileMap[fileID] = result.Path
				si.fileMapMu.Unlock()
			}
			basicsChan <- result
		}
	}
}

// NewFileIntegrator creates a new file integrator
func NewFileIntegrator(trigramIndex *core.TrigramIndex, symbolIndex *core.SymbolIndex, refTracker *core.ReferenceTracker) *FileIntegrator {
	return &FileIntegrator{
		trigramIndex: trigramIndex,
		symbolIndex:  symbolIndex,
		refTracker:   refTracker,
		// astStore removed - using metadata index instead
		metricsCalculator: analysis.NewBasicMetricsCalculator(false),
	}
}

// NewFileIntegratorWithMap creates a new file integrator with file path tracking
func NewFileIntegratorWithMap(trigramIndex *core.TrigramIndex, symbolIndex *core.SymbolIndex, refTracker *core.ReferenceTracker, symbolLocationIndex *core.SymbolLocationIndex, fileMap map[string]types.FileID, reverseFileMap map[types.FileID]string, fileMapMu *sync.RWMutex) *FileIntegrator {
	return &FileIntegrator{
		trigramIndex: trigramIndex,
		symbolIndex:  symbolIndex,
		refTracker:   refTracker,
		// astStore removed - using metadata index instead
		symbolLocationIndex: symbolLocationIndex,
		metricsCalculator:   analysis.NewBasicMetricsCalculator(false),
		fileMap:             fileMap,
		reverseFileMap:      reverseFileMap,
		fileMapMu:           fileMapMu,
	}
}

// SetScopeStore sets the scope store for the file integrator
func (fi *FileIntegrator) SetScopeStore(scopeStore ScopeStore) { fi.scopeStore = scopeStore }

// SetFileContentStore sets the file content store for the file integrator
func (fi *FileIntegrator) SetFileContentStore(store *core.FileContentStore) {
	fi.fileContentStore = store
	// astStore initialization removed - using metadata index
}

// SetConfig sets the config for the file integrator
func (fi *FileIntegrator) SetConfig(cfg *config.Config) {
	fi.config = cfg
}

// SetFileSearchEngine sets the file search engine for the file integrator
func (fi *FileIntegrator) SetFileSearchEngine(engine *core.FileSearchEngine) {
	fi.fileSearchEngine = engine
}

// SetSemanticSearchIndex sets the semantic search index for the file integrator
func (fi *FileIntegrator) SetSemanticSearchIndex(index *core.SemanticSearchIndex) {
	fi.semanticSearchIndex = index
}

// SetSideEffectPropagator sets the side effect propagator for function purity analysis
func (fi *FileIntegrator) SetSideEffectPropagator(propagator *core.SideEffectPropagator) {
	fi.sideEffectPropagator = propagator
}

// EnableMergerPipeline enables the channel-based merger pipeline for lock-free trigram indexing
// mergerCount specifies how many parallel merger goroutines to use (default: 16)
func (fi *FileIntegrator) EnableMergerPipeline(mergerCount int) {
	if fi.trigramIndex == nil {
		debug.LogIndexing("WARNING: Cannot enable merger pipeline - trigramIndex is nil\n")
		return
	}

	fi.mergerPipeline = NewTrigramMergerPipeline(fi.trigramIndex, mergerCount)
	fi.mergerPipeline.Start()
	fi.useMergerPipeline = true
	debug.LogIndexing("Merger pipeline enabled with %d mergers\n", mergerCount)
}

// DisableMergerPipeline disables the merger pipeline and shuts it down
func (fi *FileIntegrator) DisableMergerPipeline() {
	if fi.mergerPipeline != nil {
		fi.mergerPipeline.Shutdown()
		fi.mergerPipeline = nil
	}
	fi.useMergerPipeline = false
}

// GetMergerPipelineStats returns statistics about the merger pipeline
func (fi *FileIntegrator) GetMergerPipelineStats() *ChannelStats {
	if fi.mergerPipeline == nil {
		return nil
	}
	stats := fi.mergerPipeline.GetStats()
	return &stats
}

// IntegrateFiles integrates processed files into the indexes
func (fi *FileIntegrator) IntegrateFiles(ctx context.Context, resultChan <-chan ProcessedFile, progress *ProgressTracker) {
	// Ensure merger pipeline is always shut down, even if context is cancelled
	defer func() {
		if fi.useMergerPipeline && fi.mergerPipeline != nil {
			debug.LogIndexing("Shutting down merger pipeline...\n")
			fi.DisableMergerPipeline()
		}
	}()

	var integratedFiles int64
	var totalSymbols int64
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-resultChan:
			if !ok {
				// Run semantic reduce phase after all files processed
				if fi.semanticSearchIndex != nil && len(fi.processedFiles) > 0 {
					fi.ProcessSemanticResults(fi.processedFiles)
					// Clear the slice to free memory after processing
					fi.processedFiles = nil
				}

				// Run side effect propagation after all files processed
				// This propagates effects through the call graph for transitive purity analysis
				if fi.sideEffectPropagator != nil {
					if err := fi.sideEffectPropagator.Propagate(); err != nil {
						log.Printf("Warning: Side effect propagation failed: %v", err)
					} else {
						debug.LogIndexing("Side effect propagation completed\n")
					}
				}

				// Note: Universal Symbol Graph is populated during symbol extraction
				// This is more efficient than a separate analysis pass
				return
			}
			if result.Error != nil {
				log.Printf("Integration skipping failed file %s: %v", result.Path, result.Error)
				progress.AddError(IndexingError{FilePath: result.Path, Error: result.Error.Error(), Stage: result.Stage})
				progress.IncrementProcessed(result.Path)
				continue
			}

			// Skip directories (FileID 0 from LoadFile)
			if result.FileID == 0 {
				progress.IncrementProcessed(result.Path)
				continue
			}

			// Use FileID from processor if already loaded, otherwise load here
			var fileID types.FileID
			if result.FileID != 0 {
				// File was already loaded by processor using LoadFile()
				fileID = result.FileID
				// Index trigrams using merger pipeline (lock-free) or direct indexing
				if fi.useMergerPipeline && fi.mergerPipeline != nil && result.BucketedTrigrams != nil {
					// Submit to merger pipeline for lock-free parallel processing
					fi.mergerPipeline.Submit(result.BucketedTrigrams)
				} else if fi.trigramIndex != nil && result.BucketedTrigrams != nil {
					// Fall back to direct indexing
					fi.trigramIndex.IndexFileWithBucketedTrigrams(result.BucketedTrigrams)
				}
			} else if len(result.Content) > 0 && fi.fileContentStore != nil {
				// Legacy path: load file directly in integrator
				fileID = fi.fileContentStore.LoadFile(result.Path, result.Content)
				// Index trigrams using merger pipeline (lock-free) or direct indexing
				if fi.useMergerPipeline && fi.mergerPipeline != nil && result.BucketedTrigrams != nil {
					// Submit to merger pipeline for lock-free parallel processing
					fi.mergerPipeline.Submit(result.BucketedTrigrams)
				} else if fi.trigramIndex != nil && result.BucketedTrigrams != nil {
					// Fall back to direct indexing
					fi.trigramIndex.IndexFileWithBucketedTrigrams(result.BucketedTrigrams)
				}
			} else {
				// Generate FileID for empty files
				fileID = types.FileID(atomic.AddUint32(&fi.fileIDCounter, 1))
			}
			result.FileID = fileID

			// Index file path for file search functionality
			if fi.fileSearchEngine != nil {
				fi.fileSearchEngine.IndexFile(fileID, result.Path)
			}

			if fi.fileMap != nil && fi.reverseFileMap != nil {
				if fi.fileMapMu != nil {
					fi.fileMapMu.Lock()
				}
				fi.fileMap[result.Path] = fileID
				fi.reverseFileMap[fileID] = result.Path
				if fi.fileMapMu != nil {
					fi.fileMapMu.Unlock()
				}
			}
			if fi.scopeStore != nil && len(result.Scopes) > 0 {
				fi.scopeStore.StoreFileScopes(fileID, result.Scopes)
			}
			// AST processing removed - using metadata index instead
			// AST data is now extracted during parsing and stored in metadata
			if len(result.Symbols) > 0 {

				fi.symbolIndex.IndexSymbols(fileID, result.Symbols)

				// Collect semantic data during map phase (proper map-reduce pattern)
				if fi.semanticSearchIndex != nil {
					fi.collectSemanticData(&result, fileID)
				}

				atomic.AddInt64(&totalSymbols, int64(len(result.Symbols)))
				var enhancedSymbols []types.EnhancedSymbol
				if fi.refTracker != nil {
					fi.refTracker.ProcessFileImports(fileID, result.Path, result.Content)
					// Use ProcessFileWithEnhanced to propagate complexity data from parser
					enhancedSymbols = fi.refTracker.ProcessFileWithEnhanced(fileID, result.Path, result.Symbols, result.EnhancedSymbols, result.References, result.Scopes)
					// Store performance data for code_insight anti-pattern detection
					if len(result.PerfData) > 0 {
						fi.refTracker.StorePerfData(fileID, result.PerfData)
					}
					// Feed side effect results to propagator for function purity analysis
					// This must happen after symbol processing so refTracker can resolve symbol IDs
					if fi.sideEffectPropagator != nil && len(result.SideEffectResults) > 0 {
						fi.sideEffectPropagator.PropagateSideEffectsFromResults(fileID, result.SideEffectResults)
					}
					// Index file content for search (needs to happen after ProcessFile for enhanced symbols)
					if fi.symbolLocationIndex != nil && len(enhancedSymbols) > 0 {
						// Performance optimization: Pre-allocate pointer slice once and use object pool
						// This avoids repeated allocations across files
						enhancedPtrs := enhancedSymbolPtrPool.Get().([]*types.EnhancedSymbol)
						// Clear any existing data but keep capacity
						enhancedPtrs = enhancedPtrs[:0]
						// Ensure capacity for current symbols
						if cap(enhancedPtrs) < len(enhancedSymbols) {
							enhancedPtrs = make([]*types.EnhancedSymbol, len(enhancedSymbols))
						} else {
							enhancedPtrs = enhancedPtrs[:len(enhancedSymbols)]
						}
						for i := range enhancedSymbols {
							enhancedPtrs[i] = &enhancedSymbols[i]
						}
						fi.symbolLocationIndex.IndexFileSymbols(fileID, result.Symbols, enhancedPtrs)
						// Return slice to pool for reuse with bounds checking
						// Reset slice length and enforce capacity bounds to prevent unlimited growth
						if cap(enhancedPtrs) > maxSymbolPtrCapacity {
							// Capacity too large, don't return to pool - let GC handle it
							// This prevents pool from growing indefinitely with large files
						} else if cap(enhancedPtrs) < minSymbolPtrCapacity {
							// Capacity too small, expand to minimum before returning
							newSlice := make([]*types.EnhancedSymbol, 0, minSymbolPtrCapacity)
							enhancedSymbolPtrPool.Put(newSlice)
						} else {
							// Reset to empty but keep capacity for reuse
							enhancedPtrs = enhancedPtrs[:0]
							enhancedSymbolPtrPool.Put(enhancedPtrs)
						}
					}

					// Note: Universal Symbol Graph removed (no longer supported)
				} else if fi.symbolLocationIndex != nil && len(result.Symbols) > 0 {
					// Index without enhanced symbols if refTracker is not available
					fi.symbolLocationIndex.IndexFileSymbols(fileID, result.Symbols, nil)

					// Note: Universal Symbol Graph removed (no longer supported)
				}
			}
			progress.IncrementProcessed(result.Path)
			debug.LogIndexing("Integrator: processed file %s (FileID: %d) with %d symbols\n", result.Path, fileID, len(result.Symbols))
			atomic.AddInt64(&integratedFiles, 1)

			// NEW: Track processed file for semantic reduce phase
			// No mutex needed - single goroutine processes all files sequentially
			if fi.semanticSearchIndex != nil && result.SemanticData != nil {
				// Copy the processed file to avoid retaining the entire result
				fileCopy := &ProcessedFile{
					FileID:       result.FileID,
					SemanticData: result.SemanticData,
				}
				fi.processedFiles = append(fi.processedFiles, fileCopy)
			}
		}
	}
}

// collectSemanticData collects semantic data for symbols during map phase
// This follows the proper map-reduce pattern by collecting data rather than
// directly manipulating the final index structure
func (fi *FileIntegrator) collectSemanticData(result *ProcessedFile, fileID types.FileID) {
	// Get the translation dictionary for abbreviation expansion
	dict := semantic.DefaultTranslationDictionary()

	// Create semantic map result for this file
	semanticResult := NewSemanticMapResult(types.SymbolID(fileID))

	// Process each symbol with error recovery
	for idx, symbol := range result.Symbols {
		// Wrap in closure for defer/recover
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error but continue processing other symbols
					// This ensures one bad symbol doesn't break entire index
					debug.LogIndexing("WARN: Failed to collect semantic data for symbol %q (FileID: %d, Index: %d): %v\n",
						symbol.Name, fileID, idx, r)
				}
			}()

			// Create a unique symbol ID from fileID and symbol index
			// Format: (fileID << 32) | symbolIndex
			symbolID := types.SymbolID((uint64(fileID) << 32) | uint64(idx))

			// Pre-split symbol name into words
			words := core.SplitSymbolName(symbol.Name)

			// Pre-compute word stems (Porter2 algorithm)
			stems := core.StemWords(words, core.DefaultMinStemLength)

			// Generate phonetic code for fuzzy matching
			phonetic := core.GeneratePhonetic(symbol.Name)

			// Expand abbreviations
			expansions := core.ExpandAbbreviations(words, dict.Abbreviations)

			// Add to semantic map result (no index manipulation during map phase)
			semanticResult.AddSymbol(symbolID, symbol.Name, words, stems, expansions, phonetic)
		}()
	}

	// Store the semantic map result in the processed file
	// This will be processed during the reduce phase
	result.SemanticData = semanticResult
}

// ProcessSemanticResults processes all collected semantic map results (reduce phase)
// This should be called after all files have been processed to build the final semantic index
// Uses batch update to reduce map copies from O(n*m) to O(m) where n=symbols, m=map count
func (fi *FileIntegrator) ProcessSemanticResults(results []*ProcessedFile) {
	if fi.semanticSearchIndex == nil {
		return
	}

	totalSymbols := 0
	totalFiles := 0

	// Collect all updates for batch processing
	// This avoids copying all maps for each symbol
	var allUpdates []*core.SemanticIndexUpdate

	// Process each file's semantic data
	for _, result := range results {
		if result.SemanticData != nil && !result.SemanticData.IsEmpty() {
			totalFiles++
			totalSymbols += result.SemanticData.SymbolCount()

			// Collect all symbols from this file
			for _, symbolData := range result.SemanticData.Symbols {
				update := &core.SemanticIndexUpdate{
					SymbolID:   symbolData.SymbolID,
					SymbolName: symbolData.Name,
					Words:      symbolData.Words,
					Stems:      symbolData.Stems,
					Phonetic:   symbolData.Phonetic,
					Expansions: symbolData.Expansions,
				}
				allUpdates = append(allUpdates, update)
			}
		}
	}

	// Batch update all symbols at once
	// This reduces map copies from 10,000 * 10 = 100,000 to just 10!
	if len(allUpdates) > 0 {
		fi.semanticSearchIndex.AddSymbolDataBatch(allUpdates)
	}

	debug.LogIndexing("Semantic reduce phase processed %d files with %d total symbols (batch size: %d)\n",
		totalFiles, totalSymbols, len(allUpdates))
}
