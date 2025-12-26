package symbollinker

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// DebugInfo provides detailed debugging information about the symbol linking system
type DebugInfo struct {
	Summary      DebugSummary          `json:"summary"`
	Files        []FileDebugInfo       `json:"files"`
	Extractors   []ExtractorDebugInfo  `json:"extractors"`
	Resolvers    []ResolverDebugInfo   `json:"resolvers"`
	Dependencies []DependencyDebugInfo `json:"dependencies"`
}

// DebugSummary provides high-level statistics
type DebugSummary struct {
	TotalFiles       int            `json:"total_files"`
	TotalSymbols     int            `json:"total_symbols"`
	TotalImports     int            `json:"total_imports"`
	TotalReferences  int            `json:"total_references"`
	IndexingDuration time.Duration  `json:"indexing_duration"`
	LinkingDuration  time.Duration  `json:"linking_duration"`
	Languages        map[string]int `json:"languages"` // language -> file count
	LastUpdated      time.Time      `json:"last_updated"`
}

// FileDebugInfo provides per-file debugging information
type FileDebugInfo struct {
	FileID         types.FileID         `json:"file_id"`
	Path           string               `json:"path"`
	Language       string               `json:"language"`
	Size           int64                `json:"size"`
	Symbols        []SymbolDebugInfo    `json:"symbols"`
	Imports        []ImportDebugInfo    `json:"imports"`
	References     []ReferenceDebugInfo `json:"references"`
	LastModified   time.Time            `json:"last_modified"`
	ProcessingTime time.Duration        `json:"processing_time"`
	Hash           string               `json:"hash,omitempty"` // For incremental engine
}

// SymbolDebugInfo provides symbol-level debugging information
type SymbolDebugInfo struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Kind       types.SymbolKind       `json:"kind"`
	Line       int                    `json:"line"`
	Column     int                    `json:"column"`
	Scope      string                 `json:"scope"`
	Exported   bool                   `json:"exported"`
	References int                    `json:"reference_count"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ImportDebugInfo provides import resolution debugging information
type ImportDebugInfo struct {
	ImportPath   string               `json:"import_path"`
	ResolvedPath string               `json:"resolved_path"`
	FileID       types.FileID         `json:"file_id"`
	Resolution   types.ResolutionType `json:"resolution"`
	IsBuiltin    bool                 `json:"is_builtin"`
	IsExternal   bool                 `json:"is_external"`
	Error        string               `json:"error,omitempty"`
}

// ReferenceDebugInfo provides symbol reference debugging information
type ReferenceDebugInfo struct {
	SymbolName   string       `json:"symbol_name"`
	TargetFileID types.FileID `json:"target_file_id"`
	TargetPath   string       `json:"target_path"`
	Line         int          `json:"line"`
	Column       int          `json:"column"`
	Type         string       `json:"type"` // "definition", "reference", "call"
}

// ExtractorDebugInfo provides language extractor debugging information
type ExtractorDebugInfo struct {
	Language         string        `json:"language"`
	FilesProcessed   int           `json:"files_processed"`
	SymbolsExtracted int           `json:"symbols_extracted"`
	AverageTime      time.Duration `json:"average_processing_time"`
	Errors           []string      `json:"errors,omitempty"`
}

// ResolverDebugInfo provides module resolver debugging information
type ResolverDebugInfo struct {
	Type            string                  `json:"type"` // "go", "javascript"
	ImportsResolved int                     `json:"imports_resolved"`
	ResolutionStats map[string]int          `json:"resolution_stats"` // resolution type -> count
	Errors          []string                `json:"errors,omitempty"`
	Performance     ResolverPerformanceInfo `json:"performance"`
}

// ResolverPerformanceInfo provides resolver performance metrics
type ResolverPerformanceInfo struct {
	TotalTime    time.Duration `json:"total_time"`
	AverageTime  time.Duration `json:"average_time"`
	CacheHitRate float64       `json:"cache_hit_rate,omitempty"`
}

// DependencyDebugInfo provides dependency graph debugging information
type DependencyDebugInfo struct {
	FileID       types.FileID   `json:"file_id"`
	FilePath     string         `json:"file_path"`
	Dependencies []types.FileID `json:"dependencies"`     // Files this file depends on
	Dependents   []types.FileID `json:"dependents"`       // Files that depend on this file
	Depth        int            `json:"dependency_depth"` // Max depth in dependency tree
}

// GetDebugInfo returns comprehensive debugging information about the symbol linking system
func (sle *SymbolLinkerEngine) GetDebugInfo() (*DebugInfo, error) {
	sle.mutex.RLock()
	defer sle.mutex.RUnlock()

	debug := &DebugInfo{
		Summary:      sle.buildDebugSummary(),
		Files:        sle.buildFileDebugInfo(),
		Extractors:   sle.buildExtractorDebugInfo(),
		Resolvers:    sle.buildResolverDebugInfo(),
		Dependencies: []DependencyDebugInfo{}, // Basic engine doesn't track dependencies
	}

	return debug, nil
}

// GetDebugInfo for incremental engine includes dependency information
func (ie *IncrementalEngine) GetDebugInfo() (*DebugInfo, error) {
	// Get base debug info
	debug, err := ie.SymbolLinkerEngine.GetDebugInfo()
	if err != nil {
		return nil, err
	}

	// Add dependency information
	ie.incrementalMutex.RLock()
	debug.Dependencies = ie.buildDependencyDebugInfo()
	ie.incrementalMutex.RUnlock()

	return debug, nil
}

// buildDebugSummary creates a high-level summary
func (sle *SymbolLinkerEngine) buildDebugSummary() DebugSummary {
	totalFiles := len(sle.reverseRegistry)
	totalSymbols := 0
	totalImports := 0
	totalReferences := 0
	languages := make(map[string]int)

	// Count symbols, imports, and references across all files
	for fileID, path := range sle.reverseRegistry {
		// Get language from file extension
		extractor := sle.findExtractor(path)
		if extractor != nil {
			lang := extractor.GetLanguage()
			languages[lang]++
		}

		// Note: Detailed symbol/import/reference counting will be implemented in future enhancement
		// This would require accessing the core indexes
		_ = fileID
	}

	return DebugSummary{
		TotalFiles:       totalFiles,
		TotalSymbols:     totalSymbols,
		TotalImports:     totalImports,
		TotalReferences:  totalReferences,
		IndexingDuration: 0, // Timing tracking to be implemented
		LinkingDuration:  0, // Timing tracking to be implemented
		Languages:        languages,
		LastUpdated:      time.Now(),
	}
}

// buildFileDebugInfo creates per-file debugging information
func (sle *SymbolLinkerEngine) buildFileDebugInfo() []FileDebugInfo {
	files := make([]FileDebugInfo, 0, len(sle.reverseRegistry))

	for fileID, path := range sle.reverseRegistry {
		extractor := sle.findExtractor(path)
		language := "unknown"
		if extractor != nil {
			language = extractor.GetLanguage()
		}

		fileDebug := FileDebugInfo{
			FileID:         fileID,
			Path:           path,
			Language:       language,
			Size:           0,                      // File size info not yet implemented
			Symbols:        []SymbolDebugInfo{},    // Symbol details not yet implemented
			Imports:        []ImportDebugInfo{},    // Import details not yet implemented
			References:     []ReferenceDebugInfo{}, // Reference details not yet implemented
			LastModified:   time.Time{},            // File modification time not yet tracked
			ProcessingTime: 0,                      // Processing time tracking not yet implemented
		}

		files = append(files, fileDebug)
	}

	// Sort by path for consistent output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files
}

// buildExtractorDebugInfo creates extractor debugging information
func (sle *SymbolLinkerEngine) buildExtractorDebugInfo() []ExtractorDebugInfo {
	extractors := make([]ExtractorDebugInfo, 0, len(sle.extractors))

	for _, extractor := range sle.extractors {
		extractorDebug := ExtractorDebugInfo{
			Language:         extractor.GetLanguage(),
			FilesProcessed:   0,          // File processing metrics not yet implemented
			SymbolsExtracted: 0,          // Symbol extraction metrics not yet implemented
			AverageTime:      0,          // Timing metrics not yet implemented
			Errors:           []string{}, // Error collection not yet implemented
		}

		extractors = append(extractors, extractorDebug)
	}

	// Sort by language for consistent output
	sort.Slice(extractors, func(i, j int) bool {
		return extractors[i].Language < extractors[j].Language
	})

	return extractors
}

// buildResolverDebugInfo creates resolver debugging information
func (sle *SymbolLinkerEngine) buildResolverDebugInfo() []ResolverDebugInfo {
	resolvers := []ResolverDebugInfo{}

	// Go resolver
	if sle.goResolver != nil {
		goDebug := ResolverDebugInfo{
			Type:            "go",
			ImportsResolved: 0, // Import resolution metrics not yet implemented
			ResolutionStats: make(map[string]int),
			Errors:          []string{},
			Performance: ResolverPerformanceInfo{
				TotalTime:    0,
				AverageTime:  0,
				CacheHitRate: 0.0,
			},
		}
		resolvers = append(resolvers, goDebug)
	}

	// JavaScript resolver
	if sle.jsResolver != nil {
		jsDebug := ResolverDebugInfo{
			Type:            "javascript",
			ImportsResolved: 0, // Import resolution metrics not yet implemented
			ResolutionStats: make(map[string]int),
			Errors:          []string{},
			Performance: ResolverPerformanceInfo{
				TotalTime:    0,
				AverageTime:  0,
				CacheHitRate: 0.0,
			},
		}
		resolvers = append(resolvers, jsDebug)
	}

	// PHP resolver
	if sle.phpResolver != nil {
		phpDebug := ResolverDebugInfo{
			Type:            "php",
			ImportsResolved: 0, // Import resolution metrics not yet implemented
			ResolutionStats: make(map[string]int),
			Errors:          []string{},
			Performance: ResolverPerformanceInfo{
				TotalTime:    0,
				AverageTime:  0,
				CacheHitRate: 0.0,
			},
		}
		resolvers = append(resolvers, phpDebug)
	}

	// C# resolver
	if sle.csharpResolver != nil {
		csharpDebug := ResolverDebugInfo{
			Type:            "csharp",
			ImportsResolved: 0, // Import resolution metrics not yet implemented
			ResolutionStats: make(map[string]int),
			Errors:          []string{},
			Performance: ResolverPerformanceInfo{
				TotalTime:    0,
				AverageTime:  0,
				CacheHitRate: 0.0,
			},
		}
		resolvers = append(resolvers, csharpDebug)
	}

	// Python resolver
	if sle.pythonResolver != nil {
		pythonDebug := ResolverDebugInfo{
			Type:            "python",
			ImportsResolved: 0, // Import resolution metrics not yet implemented
			ResolutionStats: make(map[string]int),
			Errors:          []string{},
			Performance: ResolverPerformanceInfo{
				TotalTime:    0,
				AverageTime:  0,
				CacheHitRate: 0.0,
			},
		}
		resolvers = append(resolvers, pythonDebug)
	}

	return resolvers
}

// buildDependencyDebugInfo creates dependency graph debugging information
func (ie *IncrementalEngine) buildDependencyDebugInfo() []DependencyDebugInfo {
	deps := make([]DependencyDebugInfo, 0, len(ie.importGraph))

	for fileID, dependencies := range ie.importGraph {
		path := ie.GetFilePath(fileID)
		dependents := ie.fileDependents[fileID]

		depDebug := DependencyDebugInfo{
			FileID:       fileID,
			FilePath:     path,
			Dependencies: dependencies,
			Dependents:   dependents,
			Depth:        ie.calculateDependencyDepth(fileID),
		}

		deps = append(deps, depDebug)
	}

	// Sort by file path for consistent output
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].FilePath < deps[j].FilePath
	})

	return deps
}

// calculateDependencyDepth calculates the maximum dependency depth for a file
func (ie *IncrementalEngine) calculateDependencyDepth(fileID types.FileID) int {
	visited := make(map[types.FileID]bool)
	return ie.calculateDepthRecursive(fileID, visited)
}

func (ie *IncrementalEngine) calculateDepthRecursive(fileID types.FileID, visited map[types.FileID]bool) int {
	if visited[fileID] {
		return 0 // Circular dependency
	}

	visited[fileID] = true
	defer delete(visited, fileID)

	dependencies := ie.importGraph[fileID]
	if len(dependencies) == 0 {
		return 0
	}

	maxDepth := 0
	for _, depID := range dependencies {
		depth := 1 + ie.calculateDepthRecursive(depID, visited)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

// WriteDebugInfo writes debugging information in a human-readable format to the given writer
func (sle *SymbolLinkerEngine) WriteDebugInfo(w io.Writer) error {
	debugInfo, err := sle.GetDebugInfo()
	if err != nil {
		return fmt.Errorf("failed to get debug info: %w", err)
	}

	fmt.Fprintln(w, "=== Symbol Linking System Debug Info ===")
	fmt.Fprintln(w)

	// Summary
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  Total Files: %d\n", debugInfo.Summary.TotalFiles)
	fmt.Fprintf(w, "  Total Symbols: %d\n", debugInfo.Summary.TotalSymbols)
	fmt.Fprintf(w, "  Total Imports: %d\n", debugInfo.Summary.TotalImports)
	fmt.Fprintf(w, "  Total References: %d\n", debugInfo.Summary.TotalReferences)
	fmt.Fprintf(w, "  Languages: %v\n", debugInfo.Summary.Languages)
	fmt.Fprintf(w, "  Last Updated: %s\n", debugInfo.Summary.LastUpdated.Format("2006-01-02 15:04:05"))
	fmt.Fprintln(w)

	// Files
	fmt.Fprintf(w, "Files (%d):\n", len(debugInfo.Files))
	for _, file := range debugInfo.Files {
		fmt.Fprintf(w, "  %s (ID: %d, Language: %s, Symbols: %d, Imports: %d)\n",
			file.Path, file.FileID, file.Language, len(file.Symbols), len(file.Imports))
	}
	fmt.Fprintln(w)

	// Extractors
	fmt.Fprintf(w, "Extractors (%d):\n", len(debugInfo.Extractors))
	for _, extractor := range debugInfo.Extractors {
		fmt.Fprintf(w, "  %s: %d files processed, %d symbols extracted\n",
			extractor.Language, extractor.FilesProcessed, extractor.SymbolsExtracted)
		if len(extractor.Errors) > 0 {
			fmt.Fprintf(w, "    Errors: %v\n", extractor.Errors)
		}
	}
	fmt.Fprintln(w)

	// Resolvers
	fmt.Fprintf(w, "Resolvers (%d):\n", len(debugInfo.Resolvers))
	for _, resolver := range debugInfo.Resolvers {
		fmt.Fprintf(w, "  %s: %d imports resolved\n", resolver.Type, resolver.ImportsResolved)
		if len(resolver.ResolutionStats) > 0 {
			fmt.Fprintf(w, "    Resolution stats: %v\n", resolver.ResolutionStats)
		}
		if len(resolver.Errors) > 0 {
			fmt.Fprintf(w, "    Errors: %v\n", resolver.Errors)
		}
	}
	fmt.Fprintln(w)

	// Dependencies (if available)
	if len(debugInfo.Dependencies) > 0 {
		fmt.Fprintf(w, "Dependencies (%d):\n", len(debugInfo.Dependencies))
		for _, dep := range debugInfo.Dependencies {
			fmt.Fprintf(w, "  %s: %d dependencies, %d dependents, depth %d\n",
				dep.FilePath, len(dep.Dependencies), len(dep.Dependents), dep.Depth)
		}
		fmt.Fprintln(w)
	}

	return nil
}

// ExportDebugInfoJSON exports debugging information as JSON
func (sle *SymbolLinkerEngine) ExportDebugInfoJSON() ([]byte, error) {
	debug, err := sle.GetDebugInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get debug info: %w", err)
	}

	return json.MarshalIndent(debug, "", "  ")
}

// ValidateConsistency performs consistency checks on the symbol linking system
func (sle *SymbolLinkerEngine) ValidateConsistency() []string {
	sle.mutex.RLock()
	defer sle.mutex.RUnlock()

	var issues []string

	// Check FileID consistency
	for path, fileID := range sle.fileRegistry {
		if sle.reverseRegistry[fileID] != path {
			issues = append(issues, fmt.Sprintf("FileID mapping inconsistent: path %s -> ID %d, but ID %d -> path %s",
				path, fileID, fileID, sle.reverseRegistry[fileID]))
		}
	}

	for fileID, path := range sle.reverseRegistry {
		if sle.fileRegistry[path] != fileID {
			issues = append(issues, fmt.Sprintf("FileID mapping inconsistent: ID %d -> path %s, but path %s -> ID %d",
				fileID, path, path, sle.fileRegistry[path]))
		}
	}

	// Check extractor availability for registered files
	for path := range sle.fileRegistry {
		extractor := sle.findExtractor(path)
		if extractor == nil {
			issues = append(issues, "No extractor available for file: "+path)
		}
	}

	return issues
}

// AnalyzeDependencyComplexity analyzes the complexity of the dependency graph (for incremental engine)
func (ie *IncrementalEngine) AnalyzeDependencyComplexity() map[string]interface{} {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	analysis := make(map[string]interface{})

	totalFiles := len(ie.importGraph)
	totalEdges := 0
	maxDepth := 0
	maxDependencies := 0
	maxDependents := 0
	circularDeps := 0

	// Analyze dependency graph
	for fileID, deps := range ie.importGraph {
		totalEdges += len(deps)

		if len(deps) > maxDependencies {
			maxDependencies = len(deps)
		}

		dependents := ie.fileDependents[fileID]
		if len(dependents) > maxDependents {
			maxDependents = len(dependents)
		}

		depth := ie.calculateDependencyDepth(fileID)
		if depth > maxDepth {
			maxDepth = depth
		}

		// Simple circular dependency detection
		visited := make(map[types.FileID]bool)
		if ie.hasCircularDependency(fileID, visited) {
			circularDeps++
		}
	}

	analysis["total_files"] = totalFiles
	analysis["total_dependency_edges"] = totalEdges
	analysis["max_dependency_depth"] = maxDepth
	analysis["max_dependencies_per_file"] = maxDependencies
	analysis["max_dependents_per_file"] = maxDependents
	analysis["circular_dependencies"] = circularDeps

	if totalFiles > 0 {
		analysis["average_dependencies_per_file"] = float64(totalEdges) / float64(totalFiles)
	}

	return analysis
}

// hasCircularDependency performs a simple circular dependency check
func (ie *IncrementalEngine) hasCircularDependency(fileID types.FileID, visited map[types.FileID]bool) bool {
	if visited[fileID] {
		return true
	}

	visited[fileID] = true

	for _, depID := range ie.importGraph[fileID] {
		if ie.hasCircularDependency(depID, visited) {
			return true
		}
	}

	delete(visited, fileID)
	return false
}

// DumpDependencyGraph exports the dependency graph in DOT format for visualization
func (ie *IncrementalEngine) DumpDependencyGraph() string {
	ie.incrementalMutex.RLock()
	defer ie.incrementalMutex.RUnlock()

	var builder strings.Builder
	builder.WriteString("digraph dependencies {\n")
	builder.WriteString("  rankdir=TB;\n")
	builder.WriteString("  node [shape=box];\n\n")

	// Add nodes
	for fileID, path := range ie.reverseRegistry {
		// Simplify path for display
		shortPath := path
		if strings.Contains(path, "/") {
			parts := strings.Split(path, "/")
			shortPath = parts[len(parts)-1]
		}

		builder.WriteString(fmt.Sprintf("  \"%d\" [label=\"%s\"];\n", fileID, shortPath))
	}

	builder.WriteString("\n")

	// Add edges
	for fileID, deps := range ie.importGraph {
		for _, depID := range deps {
			builder.WriteString(fmt.Sprintf("  \"%d\" -> \"%d\";\n", fileID, depID))
		}
	}

	builder.WriteString("}\n")
	return builder.String()
}
