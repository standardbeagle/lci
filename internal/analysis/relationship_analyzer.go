package analysis

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// RelationshipAnalyzer analyzes code to extract symbol relationships for the Universal Symbol Graph
type RelationshipAnalyzer struct {
	// Core components
	universalGraph *core.UniversalSymbolGraph
	symbolLinker   SymbolLinkerInterface
	fileService    FileServiceInterface

	// Analysis configuration
	config AnalysisConfig

	// Language analyzers
	languageAnalyzers map[string]LanguageAnalyzer

	// Thread safety
	mu sync.RWMutex
}

// AnalysisConfig configures the relationship analysis process
type AnalysisConfig struct {
	// Language support
	EnabledLanguages []string `json:"enabled_languages"`

	// Relationship types to analyze
	AnalyzeExtends        bool `json:"analyze_extends"`
	AnalyzeImplements     bool `json:"analyze_implements"`
	AnalyzeContains       bool `json:"analyze_contains"`
	AnalyzeDependencies   bool `json:"analyze_dependencies"`
	AnalyzeCalls          bool `json:"analyze_calls"`
	AnalyzeReferences     bool `json:"analyze_references"`
	AnalyzeImports        bool `json:"analyze_imports"`
	AnalyzeFileCoLocation bool `json:"analyze_file_co_location"`
	AnalyzeCrossLanguage  bool `json:"analyze_cross_language"`

	// Analysis depth and limits
	MaxCallDepth    int `json:"max_call_depth"`
	MaxReferences   int `json:"max_references"`
	MaxDependencies int `json:"max_dependencies"`

	// Performance settings
	ConcurrentAnalysis bool `json:"concurrent_analysis"`
	MaxConcurrentFiles int  `json:"max_concurrent_files"`

	// Feature flags
	EnableIncrementalAnalysis bool `json:"enable_incremental_analysis"`
	EnableCrossFileAnalysis   bool `json:"enable_cross_file_analysis"`
}

// DefaultAnalysisConfig returns default configuration
func DefaultAnalysisConfig() AnalysisConfig {
	return AnalysisConfig{
		EnabledLanguages:          []string{"go", "javascript", "typescript"},
		AnalyzeExtends:            true,
		AnalyzeImplements:         true,
		AnalyzeContains:           true,
		AnalyzeDependencies:       true,
		AnalyzeCalls:              true,
		AnalyzeReferences:         true,
		AnalyzeImports:            true,
		AnalyzeFileCoLocation:     true,
		AnalyzeCrossLanguage:      false, // Disabled by default due to complexity
		MaxCallDepth:              10,
		MaxReferences:             1000,
		MaxDependencies:           100,
		ConcurrentAnalysis:        true,
		MaxConcurrentFiles:        4,
		EnableIncrementalAnalysis: true,
		EnableCrossFileAnalysis:   true,
	}
}

// NewRelationshipAnalyzer creates a new relationship analyzer
func NewRelationshipAnalyzer(
	universalGraph *core.UniversalSymbolGraph,
	symbolLinker SymbolLinkerInterface,
	fileService FileServiceInterface,
	config AnalysisConfig,
) *RelationshipAnalyzer {
	analyzer := &RelationshipAnalyzer{
		universalGraph:    universalGraph,
		symbolLinker:      symbolLinker,
		fileService:       fileService,
		config:            config,
		languageAnalyzers: make(map[string]LanguageAnalyzer),
	}

	// Initialize language analyzers
	analyzer.initializeLanguageAnalyzers()

	return analyzer
}

// initializeLanguageAnalyzers sets up language-specific analyzers
func (ra *RelationshipAnalyzer) initializeLanguageAnalyzers() {
	for _, language := range ra.config.EnabledLanguages {
		switch language {
		case "go":
			ra.languageAnalyzers[language] = NewGoAnalyzer()
		case "javascript", "typescript":
			ra.languageAnalyzers[language] = NewJavaScriptAnalyzer()
		}
	}
}

// AnalyzeFile analyzes a single file and extracts all relationships
func (ra *RelationshipAnalyzer) AnalyzeFile(fileID types.FileID) error {
	// NOTE: Removed global mutex lock to enable true concurrent file analysis
	// The universalGraph and fileService components are thread-safe and can handle concurrent access
	// This fixes the serialization bottleneck that was causing massive performance degradation

	// Get file content and metadata
	content, err := ra.fileService.GetFileContent(fileID)
	if err != nil {
		return fmt.Errorf("failed to get file content: %w", err)
	}

	filePath, err := ra.fileService.GetFilePath(fileID)
	if err != nil {
		return fmt.Errorf("failed to get file path: %w", err)
	}

	language := ra.detectLanguage(filePath)
	analyzer, exists := ra.languageAnalyzers[language]
	if !exists {
		// Skip unsupported languages
		return nil
	}

	// Extract symbols from the file first
	symbols, err := analyzer.ExtractSymbols(fileID, content, filePath)
	if err != nil {
		return fmt.Errorf("failed to extract symbols: %w", err)
	}

	// Add symbols to Universal Symbol Graph
	for _, symbol := range symbols {
		if err := ra.universalGraph.AddSymbol(symbol); err != nil {
			return fmt.Errorf("failed to add symbol %s: %w", symbol.Identity.Name, err)
		}
	}

	// Analyze relationships between symbols
	if err := ra.analyzeRelationships(fileID, symbols, analyzer, content, filePath); err != nil {
		return fmt.Errorf("failed to analyze relationships: %w", err)
	}

	// Analyze file co-location relationships if enabled
	if ra.config.AnalyzeFileCoLocation {
		if err := ra.analyzeFileCoLocation(fileID, symbols); err != nil {
			return fmt.Errorf("failed to analyze file co-location: %w", err)
		}
	}

	return nil
}

// AnalyzeProject analyzes all files in a project
func (ra *RelationshipAnalyzer) AnalyzeProject(fileIDs []types.FileID) error {
	if ra.config.ConcurrentAnalysis {
		return ra.analyzeProjectConcurrent(fileIDs)
	}
	return ra.analyzeProjectSequential(fileIDs)
}

// analyzeProjectSequential analyzes files one by one
func (ra *RelationshipAnalyzer) analyzeProjectSequential(fileIDs []types.FileID) error {
	for _, fileID := range fileIDs {
		if err := ra.AnalyzeFile(fileID); err != nil {
			return fmt.Errorf("failed to analyze file %d: %w", fileID, err)
		}
	}

	// Perform cross-file analysis if enabled
	if ra.config.EnableCrossFileAnalysis {
		return ra.analyzeCrossFileRelationships(fileIDs)
	}

	return nil
}

// analyzeProjectConcurrent analyzes files concurrently
func (ra *RelationshipAnalyzer) analyzeProjectConcurrent(fileIDs []types.FileID) error {
	// Create a semaphore to limit concurrent file processing
	semaphore := make(chan struct{}, ra.config.MaxConcurrentFiles)
	errorChan := make(chan error, len(fileIDs))
	var wg sync.WaitGroup

	for _, fileID := range fileIDs {
		wg.Add(1)
		go func(fID types.FileID) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			if err := ra.AnalyzeFile(fID); err != nil {
				errorChan <- fmt.Errorf("failed to analyze file %d: %w", fID, err)
				return
			}
			errorChan <- nil
		}(fileID)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	// Perform cross-file analysis if enabled
	if ra.config.EnableCrossFileAnalysis {
		return ra.analyzeCrossFileRelationships(fileIDs)
	}

	return nil
}

// analyzeRelationships analyzes relationships within a file
func (ra *RelationshipAnalyzer) analyzeRelationships(fileID types.FileID, symbols []*types.UniversalSymbolNode, analyzer LanguageAnalyzer, content, filePath string) error {
	for _, symbol := range symbols {
		// Analyze extends relationships
		if ra.config.AnalyzeExtends {
			extends, err := analyzer.AnalyzeExtends(symbol, content, filePath)
			if err != nil {
				return fmt.Errorf("failed to analyze extends for %s: %w", symbol.Identity.Name, err)
			}
			symbol.Relationships.Extends = extends
		}

		// Analyze implements relationships
		if ra.config.AnalyzeImplements {
			implements, err := analyzer.AnalyzeImplements(symbol, content, filePath)
			if err != nil {
				return fmt.Errorf("failed to analyze implements for %s: %w", symbol.Identity.Name, err)
			}
			symbol.Relationships.Implements = implements
		}

		// Analyze contains relationships
		if ra.config.AnalyzeContains {
			contains, err := analyzer.AnalyzeContains(symbol, content, filePath)
			if err != nil {
				return fmt.Errorf("failed to analyze contains for %s: %w", symbol.Identity.Name, err)
			}
			symbol.Relationships.Contains = contains
		}

		// Analyze dependencies
		if ra.config.AnalyzeDependencies {
			dependencies, err := analyzer.AnalyzeDependencies(symbol, content, filePath)
			if err != nil {
				return fmt.Errorf("failed to analyze dependencies for %s: %w", symbol.Identity.Name, err)
			}
			symbol.Relationships.Dependencies = dependencies
		}

		// Analyze function calls
		if ra.config.AnalyzeCalls {
			calls, err := analyzer.AnalyzeCalls(symbol, content, filePath)
			if err != nil {
				return fmt.Errorf("failed to analyze calls for %s: %w", symbol.Identity.Name, err)
			}
			symbol.Relationships.CallsTo = calls
		}

		// Update the symbol in the Universal Symbol Graph with new relationships
		if err := ra.universalGraph.AddSymbol(symbol); err != nil {
			return fmt.Errorf("failed to update symbol %s with relationships: %w", symbol.Identity.Name, err)
		}
	}

	return nil
}

// analyzeFileCoLocation analyzes co-location relationships within a file
func (ra *RelationshipAnalyzer) analyzeFileCoLocation(fileID types.FileID, symbols []*types.UniversalSymbolNode) error {
	// For each symbol, add all other symbols in the same file as co-located
	for i, symbol := range symbols {
		coLocated := make([]types.CompositeSymbolID, 0, len(symbols)-1)
		for j, otherSymbol := range symbols {
			if i != j { // Don't include the symbol itself
				coLocated = append(coLocated, otherSymbol.Identity.ID)
			}
		}
		symbol.Relationships.FileCoLocated = coLocated
	}

	// Update all symbols at once instead of individually to avoid duplication
	for _, symbol := range symbols {
		if err := ra.universalGraph.AddSymbol(symbol); err != nil {
			return fmt.Errorf("failed to update symbol %s with co-location: %w", symbol.Identity.Name, err)
		}
	}

	return nil
}

// analyzeCrossFileRelationships analyzes relationships that span across files
func (ra *RelationshipAnalyzer) analyzeCrossFileRelationships(fileIDs []types.FileID) error {
	// This is a placeholder for cross-file relationship analysis
	// Implementation would involve:
	// 1. Resolving imports and module dependencies
	// 2. Linking symbols across file boundaries
	// 3. Building cross-file call graphs
	// 4. Analyzing inheritance hierarchies that span files

	// For now, we'll implement basic cross-file reference resolution
	for _, fileID := range fileIDs {
		if err := ra.resolveSymbolReferences(fileID); err != nil {
			return fmt.Errorf("failed to resolve references for file %d: %w", fileID, err)
		}
	}

	return nil
}

// resolveSymbolReferences resolves symbol references across files
func (ra *RelationshipAnalyzer) resolveSymbolReferences(fileID types.FileID) error {
	// Get all symbols in this file
	symbols := ra.universalGraph.GetSymbolsByFile(fileID)

	// For each symbol, try to resolve its unresolved references using the symbol linker
	for _, symbol := range symbols {
		// Use symbol linker to resolve imports and references
		if ra.symbolLinker != nil {
			resolvedRefs, err := ra.symbolLinker.ResolveReferences(symbol.Identity.ID)
			if err != nil {
				// Log error but continue - reference resolution failures are common
				continue
			}

			// Update dependencies based on resolved references
			for _, ref := range resolvedRefs {
				// Convert reference to dependency
				dependency := types.SymbolDependency{
					Target:     ref.Target,
					Type:       types.DependencyImport, // Most cross-file refs are imports
					Strength:   types.DependencyModerate,
					Context:    "cross_file_reference",
					ImportPath: ref.ImportPath,
				}
				symbol.Relationships.Dependencies = append(symbol.Relationships.Dependencies, dependency)
			}

			// Update the symbol in the Universal Symbol Graph
			if err := ra.universalGraph.AddSymbol(symbol); err != nil {
				return fmt.Errorf("failed to update symbol %s with resolved references: %w", symbol.Identity.Name, err)
			}
		}
	}

	return nil
}

// detectLanguage detects the programming language based on file extension
func (ra *RelationshipAnalyzer) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	default:
		return "unknown"
	}
}

// GetAnalysisStats returns statistics about the relationship analysis
func (ra *RelationshipAnalyzer) GetAnalysisStats() AnalysisStats {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	// Get Universal Symbol Graph stats
	graphStats := ra.universalGraph.GetStats()

	return AnalysisStats{
		TotalSymbols:      graphStats.TotalNodes,
		SymbolsByLanguage: graphStats.NodesByLanguage,
		SymbolsByKind:     graphStats.NodesByKind,
		RelationshipCount: graphStats.RelationshipCount,
		QueryCount:        graphStats.QueryCount,
		AvgQueryTime:      graphStats.AvgQueryTime,
		LastUpdated:       graphStats.LastUpdated,
		MemoryUsage:       graphStats.MemoryUsage,
		EnabledAnalyzers:  ra.getEnabledAnalyzers(),
	}
}

// getEnabledAnalyzers returns list of enabled analyzer names
func (ra *RelationshipAnalyzer) getEnabledAnalyzers() []string {
	analyzers := make([]string, 0, len(ra.languageAnalyzers))
	for lang := range ra.languageAnalyzers {
		analyzers = append(analyzers, lang)
	}
	return analyzers
}

// UpdateConfig updates the analysis configuration
func (ra *RelationshipAnalyzer) UpdateConfig(config AnalysisConfig) error {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.config = config

	// Reinitialize language analyzers if needed
	ra.initializeLanguageAnalyzers()

	return nil
}

// Clear clears all analysis data
func (ra *RelationshipAnalyzer) Clear() {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.universalGraph.Clear()
}
