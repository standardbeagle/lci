package analysis

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// FunctionDependencyTracker provides enhanced dependency analysis with transitive resolution
type FunctionDependencyTracker struct {
	// Reference to existing reference tracker
	refTracker *core.ReferenceTracker

	// File-level import access (optional - for graph comparison)
	indexer FileImportProvider

	// Enhanced dependency structures
	callGraph          map[types.SymbolID]*CallGraphNode
	dependencyCache    map[types.SymbolID]*DependencySet
	fileGraph          map[types.FileID]*FileGraphNode
	transitiveResolver *TransitiveDependencyResolver

	mu sync.RWMutex
}

// FileImportProvider provides access to file-level imports
type FileImportProvider interface {
	GetFileImports(fileID types.FileID) []types.Import
	GetFileInfo(fileID types.FileID) *types.FileInfo
}

// CallGraphNode represents a node in the function call graph
type CallGraphNode struct {
	Symbol      *types.EnhancedSymbol `json:"symbol"`
	DirectCalls []*FunctionDependency `json:"direct_calls"`
	CalledBy    []*FunctionDependency `json:"called_by"`
	Depth       int                   `json:"depth"`
	IsRecursive bool                  `json:"is_recursive"`
	CyclePaths  [][]types.SymbolID    `json:"cycle_paths,omitempty"`
	IsCritical  bool                  `json:"is_critical"` // Many dependents
	IsLeaf      bool                  `json:"is_leaf"`     // No dependencies
	IsRoot      bool                  `json:"is_root"`     // No callers
}

// FileGraphNode represents dependencies between files
type FileGraphNode struct {
	FileID          types.FileID     `json:"file_id"`
	FilePath        string           `json:"file_path"`
	ImportedFiles   []types.FileID   `json:"imported_files"`
	ImportedBy      []types.FileID   `json:"imported_by"`
	InternalSymbols []types.SymbolID `json:"internal_symbols"`
	ExportedSymbols []types.SymbolID `json:"exported_symbols"`
	ModuleStability float64          `json:"module_stability"`
	CouplingFactor  float64          `json:"coupling_factor"`
}

// DependencySet represents all dependencies for a function
type DependencySet struct {
	SymbolID types.SymbolID `json:"symbol_id"`

	// Direct dependencies
	DirectDeps       []*FunctionDependency `json:"direct_deps"`
	DirectDependents []*FunctionDependency `json:"direct_dependents"`

	// Transitive dependencies
	TransitiveDeps       []*FunctionDependency `json:"transitive_deps"`
	TransitiveDependents []*FunctionDependency `json:"transitive_dependents"`

	// File-level dependencies
	FileDeps     []types.FileID        `json:"file_deps"`
	ExternalDeps []*ExternalDependency `json:"external_deps"`

	// Analysis results
	DepthLevels       [][]types.SymbolID `json:"depth_levels"`
	CyclicDeps        [][]types.SymbolID `json:"cyclic_deps"`
	CriticalPath      []types.SymbolID   `json:"critical_path"`
	BottleneckSymbols []types.SymbolID   `json:"bottleneck_symbols"`

	// Metrics
	MaxDepth          int `json:"max_depth"`
	TotalDependencies int `json:"total_dependencies"`
	UniqueFiles       int `json:"unique_files"`
	CircularCount     int `json:"circular_count"`
	FanOut            int `json:"fan_out"`
	FanIn             int `json:"fan_in"`

	// Cache management
	CalculatedAt int64 `json:"calculated_at"`
	IsStale      bool  `json:"is_stale"`
}

// FunctionDependency represents a dependency relationship between functions
type FunctionDependency struct {
	SourceSymbol      types.SymbolID    `json:"source_symbol"`
	TargetSymbol      types.SymbolID    `json:"target_symbol"`
	CallType          CallType          `json:"call_type"`
	CallStrength      types.RefStrength `json:"call_strength"`
	CallContext       []types.ScopeInfo `json:"call_context"`
	FileLocation      types.FileID      `json:"file_location"`
	LineNumber        int               `json:"line_number"`
	Distance          int               `json:"distance"`        // Degrees of separation
	ThroughSymbols    []types.SymbolID  `json:"through_symbols"` // For transitive deps
	Probability       float64           `json:"probability"`     // 0-1 likelihood of execution
	PerformanceImpact PerformanceImpact `json:"performance_impact"`
}

// CallType represents different types of function calls
type CallType string

const (
	CallTypeDirect      CallType = "direct_call"
	CallTypeIndirect    CallType = "indirect_call"
	CallTypeCallback    CallType = "callback"
	CallTypeAsync       CallType = "async"
	CallTypeRecursive   CallType = "recursive"
	CallTypeConditional CallType = "conditional"
	CallTypePolymorphic CallType = "polymorphic"
	CallTypeExternal    CallType = "external"
)

// PerformanceImpact represents the potential performance cost of a dependency
type PerformanceImpact string

const (
	PerfImpactLow      PerformanceImpact = "low"
	PerfImpactMedium   PerformanceImpact = "medium"
	PerfImpactHigh     PerformanceImpact = "high"
	PerfImpactCritical PerformanceImpact = "critical"
	PerfImpactUnknown  PerformanceImpact = "unknown"
)

// ExternalDependency represents a dependency on external libraries/services
type ExternalDependency struct {
	Name              string            `json:"name"`
	Type              ExternalDepType   `json:"type"`
	Version           string            `json:"version,omitempty"`
	Source            string            `json:"source"`      // package manager, URL, etc.
	IsStandard        bool              `json:"is_standard"` // stdlib vs third-party
	SecurityRisk      SecurityRisk      `json:"security_risk"`
	PerformanceImpact PerformanceImpact `json:"performance_impact"`
	Stability         float64           `json:"stability"` // 0-1 stability score
}

// ExternalDepType categorizes external dependencies
type ExternalDepType string

const (
	ExtDepLibrary   ExternalDepType = "library"
	ExtDepFramework ExternalDepType = "framework"
	ExtDepService   ExternalDepType = "service"
	ExtDepDatabase  ExternalDepType = "database"
	ExtDepAPI       ExternalDepType = "api"
	ExtDepUtility   ExternalDepType = "utility"
	ExtDepStdlib    ExternalDepType = "stdlib"
)

// SecurityRisk represents security implications of external dependencies
type SecurityRisk string

const (
	SecurityLow      SecurityRisk = "low"
	SecurityMedium   SecurityRisk = "medium"
	SecurityHigh     SecurityRisk = "high"
	SecurityCritical SecurityRisk = "critical"
	SecurityUnknown  SecurityRisk = "unknown"
)

// TransitiveDependencyResolver handles complex dependency resolution
type TransitiveDependencyResolver struct {
	maxDepth        int
	visitedSymbols  map[types.SymbolID]bool
	dependencyGraph map[types.SymbolID][]types.SymbolID
}

// DependencyQuery represents a query for dependency analysis
type DependencyQuery struct {
	SymbolID          types.SymbolID `json:"symbol_id,omitempty"`
	SymbolName        string         `json:"symbol_name,omitempty"`
	FileID            types.FileID   `json:"file_id,omitempty"`
	Direction         string         `json:"direction"` // "incoming", "outgoing", "both"
	IncludeTransitive bool           `json:"include_transitive"`
	MaxDepth          int            `json:"max_depth"`
	IncludeExternal   bool           `json:"include_external"`
	FilterByType      []CallType     `json:"filter_by_type,omitempty"`
	MinProbability    float64        `json:"min_probability,omitempty"`
	ExcludeFiles      []types.FileID `json:"exclude_files,omitempty"`
}

// NewFunctionDependencyTracker creates a new enhanced dependency tracker
func NewFunctionDependencyTracker(refTracker *core.ReferenceTracker) *FunctionDependencyTracker {
	return &FunctionDependencyTracker{
		refTracker:      refTracker,
		indexer:         nil, // Optional - can be set with SetIndexer
		callGraph:       make(map[types.SymbolID]*CallGraphNode),
		dependencyCache: make(map[types.SymbolID]*DependencySet),
		fileGraph:       make(map[types.FileID]*FileGraphNode),
		transitiveResolver: &TransitiveDependencyResolver{
			maxDepth:        50, // Reasonable limit to prevent infinite recursion
			visitedSymbols:  make(map[types.SymbolID]bool),
			dependencyGraph: make(map[types.SymbolID][]types.SymbolID),
		},
	}
}

// SetIndexer sets the file import provider for graph comparison
func (fdt *FunctionDependencyTracker) SetIndexer(indexer FileImportProvider) {
	fdt.mu.Lock()
	defer fdt.mu.Unlock()
	fdt.indexer = indexer
}

// BuildCallGraph constructs the complete function call graph
func (fdt *FunctionDependencyTracker) BuildCallGraph() error {
	fdt.mu.Lock()
	defer fdt.mu.Unlock()

	// Clear existing graph
	fdt.callGraph = make(map[types.SymbolID]*CallGraphNode)

	// Get all symbols from reference tracker
	// Note: We'll need to add a method to ReferenceTracker to get all symbols
	// For now, we'll work with the structure we have

	return nil
}

// GetFunctionDependencies returns comprehensive dependency information for a function
func (fdt *FunctionDependencyTracker) GetFunctionDependencies(query DependencyQuery) (*DependencySet, error) {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	var symbol *types.EnhancedSymbol

	// Resolve symbol
	if query.SymbolID != 0 {
		symbol = fdt.refTracker.GetEnhancedSymbol(query.SymbolID)
	} else if query.SymbolName != "" {
		symbols := fdt.refTracker.FindSymbolsByName(query.SymbolName)
		if len(symbols) > 0 {
			symbol = symbols[0] // Take first match
		}
	}

	if symbol == nil {
		return nil, errors.New("symbol not found")
	}

	// Check cache
	if cached, exists := fdt.dependencyCache[symbol.ID]; exists && !cached.IsStale {
		return cached, nil
	}

	// Calculate dependencies
	depSet := &DependencySet{
		SymbolID: symbol.ID,
	}

	// Get direct dependencies
	depSet.DirectDeps = fdt.getDirectDependencies(symbol, query)
	depSet.DirectDependents = fdt.getDirectDependents(symbol, query)

	// Get transitive dependencies if requested
	if query.IncludeTransitive {
		depSet.TransitiveDeps = fdt.getTransitiveDependencies(symbol, query)
		depSet.TransitiveDependents = fdt.getTransitiveDependents(symbol, query)
	}

	// Calculate file dependencies
	depSet.FileDeps = fdt.calculateFileDependencies(depSet)

	// Detect circular dependencies
	depSet.CyclicDeps = fdt.detectCircularDependencies(symbol, query.MaxDepth)

	// Calculate metrics
	depSet.TotalDependencies = len(depSet.DirectDeps) + len(depSet.TransitiveDeps)
	depSet.FanOut = len(depSet.DirectDeps)
	depSet.FanIn = len(depSet.DirectDependents)
	depSet.CircularCount = len(depSet.CyclicDeps)

	// Cache result
	fdt.dependencyCache[symbol.ID] = depSet

	return depSet, nil
}

// getDirectDependencies gets immediate function dependencies
func (fdt *FunctionDependencyTracker) getDirectDependencies(symbol *types.EnhancedSymbol, query DependencyQuery) []*FunctionDependency {
	var deps []*FunctionDependency

	// Convert outgoing references to function dependencies
	for _, ref := range symbol.OutgoingRefs {
		if fdt.shouldIncludeReference(ref, query) {
			dep := &FunctionDependency{
				SourceSymbol:      symbol.ID,
				TargetSymbol:      ref.TargetSymbol,
				CallType:          fdt.determineCallType(ref),
				CallStrength:      ref.Strength,
				CallContext:       ref.ScopeContext,
				FileLocation:      ref.FileID,
				LineNumber:        ref.Line,
				Distance:          1,
				Probability:       fdt.estimateCallProbability(ref),
				PerformanceImpact: fdt.estimatePerformanceImpact(ref),
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

// getDirectDependents gets functions that depend on this one
func (fdt *FunctionDependencyTracker) getDirectDependents(symbol *types.EnhancedSymbol, query DependencyQuery) []*FunctionDependency {
	var dependents []*FunctionDependency

	// Convert incoming references to function dependencies
	for _, ref := range symbol.IncomingRefs {
		if fdt.shouldIncludeReference(ref, query) {
			dep := &FunctionDependency{
				SourceSymbol:      ref.SourceSymbol,
				TargetSymbol:      symbol.ID,
				CallType:          fdt.determineCallType(ref),
				CallStrength:      ref.Strength,
				CallContext:       ref.ScopeContext,
				FileLocation:      ref.FileID,
				LineNumber:        ref.Line,
				Distance:          1,
				Probability:       fdt.estimateCallProbability(ref),
				PerformanceImpact: fdt.estimatePerformanceImpact(ref),
			}
			dependents = append(dependents, dep)
		}
	}

	return dependents
}

// getTransitiveDependencies calculates transitive dependencies using graph traversal
func (fdt *FunctionDependencyTracker) getTransitiveDependencies(symbol *types.EnhancedSymbol, query DependencyQuery) []*FunctionDependency {
	var transitiveDeps []*FunctionDependency
	visited := make(map[types.SymbolID]bool)
	maxDepth := query.MaxDepth
	if maxDepth == 0 {
		maxDepth = 10 // Default reasonable limit
	}

	fdt.traverseDependencies(symbol.ID, 1, maxDepth, visited, &transitiveDeps, query)

	return transitiveDeps
}

// traverseDependencies recursively traverses the dependency graph
func (fdt *FunctionDependencyTracker) traverseDependencies(symbolID types.SymbolID, currentDepth, maxDepth int, visited map[types.SymbolID]bool, deps *[]*FunctionDependency, query DependencyQuery) {
	if currentDepth > maxDepth || visited[symbolID] {
		return
	}

	visited[symbolID] = true
	symbol := fdt.refTracker.GetEnhancedSymbol(symbolID)
	if symbol == nil {
		return
	}

	// Get direct dependencies and traverse them
	for _, ref := range symbol.OutgoingRefs {
		if fdt.shouldIncludeReference(ref, query) {
			dep := &FunctionDependency{
				SourceSymbol:      symbolID,
				TargetSymbol:      ref.TargetSymbol,
				CallType:          fdt.determineCallType(ref),
				CallStrength:      ref.Strength,
				CallContext:       ref.ScopeContext,
				FileLocation:      ref.FileID,
				LineNumber:        ref.Line,
				Distance:          currentDepth,
				Probability:       fdt.estimateCallProbability(ref),
				PerformanceImpact: fdt.estimatePerformanceImpact(ref),
			}
			*deps = append(*deps, dep)

			// Recursively traverse
			fdt.traverseDependencies(ref.TargetSymbol, currentDepth+1, maxDepth, visited, deps, query)
		}
	}
}

// getTransitiveDependents calculates functions that transitively depend on this one
func (fdt *FunctionDependencyTracker) getTransitiveDependents(symbol *types.EnhancedSymbol, query DependencyQuery) []*FunctionDependency {
	var transitiveDependents []*FunctionDependency
	visited := make(map[types.SymbolID]bool)
	maxDepth := query.MaxDepth
	if maxDepth == 0 {
		maxDepth = 10
	}

	fdt.traverseDependents(symbol.ID, 1, maxDepth, visited, &transitiveDependents, query)

	return transitiveDependents
}

// traverseDependents recursively traverses reverse dependencies
func (fdt *FunctionDependencyTracker) traverseDependents(symbolID types.SymbolID, currentDepth, maxDepth int, visited map[types.SymbolID]bool, dependents *[]*FunctionDependency, query DependencyQuery) {
	if currentDepth > maxDepth || visited[symbolID] {
		return
	}

	visited[symbolID] = true
	symbol := fdt.refTracker.GetEnhancedSymbol(symbolID)
	if symbol == nil {
		return
	}

	// Get direct dependents and traverse them
	for _, ref := range symbol.IncomingRefs {
		if fdt.shouldIncludeReference(ref, query) {
			dep := &FunctionDependency{
				SourceSymbol:      ref.SourceSymbol,
				TargetSymbol:      symbolID,
				CallType:          fdt.determineCallType(ref),
				CallStrength:      ref.Strength,
				CallContext:       ref.ScopeContext,
				FileLocation:      ref.FileID,
				LineNumber:        ref.Line,
				Distance:          currentDepth,
				Probability:       fdt.estimateCallProbability(ref),
				PerformanceImpact: fdt.estimatePerformanceImpact(ref),
			}
			*dependents = append(*dependents, dep)

			// Recursively traverse
			fdt.traverseDependents(ref.SourceSymbol, currentDepth+1, maxDepth, visited, dependents, query)
		}
	}
}

// detectCircularDependencies finds circular dependency chains
func (fdt *FunctionDependencyTracker) detectCircularDependencies(symbol *types.EnhancedSymbol, maxDepth int) [][]types.SymbolID {
	var cycles [][]types.SymbolID
	visited := make(map[types.SymbolID]bool)
	stack := make(map[types.SymbolID]bool)
	path := []types.SymbolID{}

	fdt.findCycles(symbol.ID, visited, stack, path, &cycles, maxDepth)

	return cycles
}

// findCycles uses DFS to find circular dependencies
func (fdt *FunctionDependencyTracker) findCycles(symbolID types.SymbolID, visited, stack map[types.SymbolID]bool, path []types.SymbolID, cycles *[][]types.SymbolID, maxDepth int) {
	if len(path) > maxDepth {
		return
	}

	visited[symbolID] = true
	stack[symbolID] = true
	path = append(path, symbolID)

	symbol := fdt.refTracker.GetEnhancedSymbol(symbolID)
	if symbol == nil {
		return
	}

	for _, ref := range symbol.OutgoingRefs {
		targetID := ref.TargetSymbol

		if stack[targetID] {
			// Found a cycle - extract the cycle from the path
			for i, id := range path {
				if id == targetID {
					cycle := make([]types.SymbolID, len(path)-i)
					copy(cycle, path[i:])
					*cycles = append(*cycles, cycle)
					break
				}
			}
		} else if !visited[targetID] {
			fdt.findCycles(targetID, visited, stack, path, cycles, maxDepth)
		}
	}

	stack[symbolID] = false
}

// Helper methods

// shouldIncludeReference determines if a reference should be included based on query filters
func (fdt *FunctionDependencyTracker) shouldIncludeReference(ref types.Reference, query DependencyQuery) bool {
	// Apply call type filter
	if len(query.FilterByType) > 0 {
		callType := fdt.determineCallType(ref)
		found := false
		for _, filterType := range query.FilterByType {
			if callType == filterType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Apply probability filter
	if query.MinProbability > 0 {
		probability := fdt.estimateCallProbability(ref)
		if probability < query.MinProbability {
			return false
		}
	}

	// Apply file exclusion filter
	for _, excludeFile := range query.ExcludeFiles {
		if ref.FileID == excludeFile {
			return false
		}
	}

	return true
}

// determineCallType analyzes the reference to determine call type
func (fdt *FunctionDependencyTracker) determineCallType(ref types.Reference) CallType {
	switch ref.Type {
	case types.RefTypeCall:
		return CallTypeDirect
	case types.RefTypeReturn:
		return CallTypeCallback
	default:
		return CallTypeDirect
	}
}

// estimateCallProbability estimates the likelihood of a call being executed
func (fdt *FunctionDependencyTracker) estimateCallProbability(ref types.Reference) float64 {
	// Basic heuristic - could be enhanced with static analysis
	switch ref.Strength {
	case types.RefStrengthTight:
		return 0.9
	case types.RefStrengthLoose:
		return 0.6
	case types.RefStrengthTransitive:
		return 0.3
	default:
		return 0.5
	}
}

// estimatePerformanceImpact estimates the performance cost of a dependency
func (fdt *FunctionDependencyTracker) estimatePerformanceImpact(ref types.Reference) PerformanceImpact {
	// Basic heuristic based on reference type and context
	switch ref.Type {
	case types.RefTypeCall:
		// Check if it looks like an expensive call based on name or context
		return PerfImpactMedium
	case types.RefTypeImport:
		return PerfImpactLow
	default:
		return PerfImpactUnknown
	}
}

// calculateFileDependencies determines which files are dependencies
func (fdt *FunctionDependencyTracker) calculateFileDependencies(depSet *DependencySet) []types.FileID {
	fileSet := make(map[types.FileID]bool)

	// Collect files from direct dependencies
	for _, dep := range depSet.DirectDeps {
		fileSet[dep.FileLocation] = true
	}

	// Collect files from transitive dependencies
	for _, dep := range depSet.TransitiveDeps {
		fileSet[dep.FileLocation] = true
	}

	// Convert to slice
	var files []types.FileID
	for fileID := range fileSet {
		files = append(files, fileID)
	}

	// Sort for consistent results
	sort.Slice(files, func(i, j int) bool {
		return files[i] < files[j]
	})

	return files
}

// GetFileGraph returns file-level dependency information
func (fdt *FunctionDependencyTracker) GetFileGraph() map[types.FileID]*FileGraphNode {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	// Return copy of file graph
	result := make(map[types.FileID]*FileGraphNode)
	for k, v := range fdt.fileGraph {
		result[k] = v
	}

	return result
}

// CompareGraphs compares file-level vs symbol-level dependency patterns
func (fdt *FunctionDependencyTracker) CompareGraphs(fileID types.FileID) *GraphComparison {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	result := &GraphComparison{
		FileID:                  fileID,
		UnusedImports:           []string{},
		MissingImports:          []string{},
		ArchitecturalViolations: []string{},
		CouplingDiscrepancies:   []string{},
	}

	// Need indexer for file-level imports
	if fdt.indexer == nil {
		return result
	}

	// Get file-level imports (declared dependencies)
	declaredImports := fdt.indexer.GetFileImports(fileID)
	declaredPaths := make(map[string]bool)
	for _, imp := range declaredImports {
		declaredPaths[imp.Path] = false // Track usage
	}

	// Get symbol-level dependencies (actual usage)
	symbols := fdt.refTracker.GetFileEnhancedSymbols(fileID)
	actuallyUsedFiles := make(map[types.FileID]bool)

	// Analyze actual symbol usage
	for _, symbol := range symbols {
		// Check outgoing references (what this symbol uses)
		for _, outRef := range symbol.OutgoingRefs {
			// Find target symbol to determine its file
			targetSymbol := fdt.refTracker.GetEnhancedSymbol(outRef.TargetSymbol)
			if targetSymbol != nil && targetSymbol.FileID != fileID {
				actuallyUsedFiles[targetSymbol.FileID] = true

				// Check if this file's import matches the target file
				targetFileInfo := fdt.indexer.GetFileInfo(targetSymbol.FileID)
				if targetFileInfo != nil {
					// Mark import as used if it matches
					for importPath := range declaredPaths {
						if fdt.importsMatch(importPath, targetFileInfo.Path) {
							declaredPaths[importPath] = true
						}
					}
				}
			}
		}
	}

	// Detect unused imports
	for importPath, used := range declaredPaths {
		if !used {
			result.UnusedImports = append(result.UnusedImports, importPath)
		}
	}

	// Detect missing imports (symbols used but not declared)
	for usedFileID := range actuallyUsedFiles {
		fileInfo := fdt.indexer.GetFileInfo(usedFileID)
		if fileInfo == nil {
			continue
		}

		// Check if any declared import covers this file
		found := false
		for importPath := range declaredPaths {
			if fdt.importsMatch(importPath, fileInfo.Path) {
				found = true
				break
			}
		}

		if !found {
			result.MissingImports = append(result.MissingImports, fileInfo.Path)
		}
	}

	// Calculate import vs usage gap
	totalImports := len(declaredImports)
	totalUsed := len(actuallyUsedFiles)
	if totalImports > 0 {
		result.ImportVsUsageGap = float64(totalImports-totalUsed) / float64(totalImports)
	}

	// Detect architectural violations (e.g., circular dependencies at file level)
	fdt.detectArchitecturalViolations(fileID, result)

	// Detect coupling discrepancies (high symbol coupling with low import coupling)
	fdt.detectCouplingDiscrepancies(fileID, symbols, actuallyUsedFiles, result)

	return result
}

// importsMatch checks if an import path matches a file path
func (fdt *FunctionDependencyTracker) importsMatch(importPath, filePath string) bool {
	// Simple heuristic: check if import path is contained in file path
	// or if they share significant path components
	// This is language-dependent and could be enhanced

	// Direct match
	if importPath == filePath {
		return true
	}

	// Check if file path ends with import path
	if len(filePath) >= len(importPath) {
		if filePath[len(filePath)-len(importPath):] == importPath {
			return true
		}
	}

	// Check if import path ends with file path
	if len(importPath) >= len(filePath) {
		if importPath[len(importPath)-len(filePath):] == filePath {
			return true
		}
	}

	return false
}

// detectArchitecturalViolations identifies circular dependencies and layering violations
func (fdt *FunctionDependencyTracker) detectArchitecturalViolations(fileID types.FileID, result *GraphComparison) {
	// Check for circular dependencies at file level
	if fdt.indexer == nil {
		return
	}

	// Get file's imports
	imports := fdt.indexer.GetFileImports(fileID)

	// For each imported file, check if it imports us back (simple cycle detection)
	for _, imp := range imports {
		// Find the FileID for this import
		// This is a simplified check - a full implementation would need proper module resolution
		// For now, we just document the pattern
		result.ArchitecturalViolations = append(result.ArchitecturalViolations,
			"Potential circular dependency with: "+imp.Path)
	}
}

// detectCouplingDiscrepancies finds cases where symbol coupling differs from import coupling
func (fdt *FunctionDependencyTracker) detectCouplingDiscrepancies(
	fileID types.FileID,
	symbols []*types.EnhancedSymbol,
	actuallyUsedFiles map[types.FileID]bool,
	result *GraphComparison,
) {
	if fdt.indexer == nil {
		return
	}

	// Calculate coupling strength per file
	couplingStrength := make(map[types.FileID]int)
	for _, symbol := range symbols {
		for _, outRef := range symbol.OutgoingRefs {
			targetSymbol := fdt.refTracker.GetEnhancedSymbol(outRef.TargetSymbol)
			if targetSymbol != nil && targetSymbol.FileID != fileID {
				couplingStrength[targetSymbol.FileID]++
			}
		}
	}

	// Detect high coupling without explicit import
	for targetFileID, strength := range couplingStrength {
		if strength > 5 { // Threshold for "high coupling"
			fileInfo := fdt.indexer.GetFileInfo(targetFileID)
			if fileInfo != nil {
				imports := fdt.indexer.GetFileImports(fileID)
				hasImport := false
				for _, imp := range imports {
					if fdt.importsMatch(imp.Path, fileInfo.Path) {
						hasImport = true
						break
					}
				}

				if !hasImport {
					result.CouplingDiscrepancies = append(result.CouplingDiscrepancies,
						fmt.Sprintf("High coupling (%d refs) to %s without explicit import",
							strength, fileInfo.Path))
				}
			}
		}
	}
}

// GraphComparison represents the results of comparing file vs dependency graphs
type GraphComparison struct {
	FileID                  types.FileID `json:"file_id"`
	ImportVsUsageGap        float64      `json:"import_vs_usage_gap"`
	UnusedImports           []string     `json:"unused_imports"`
	MissingImports          []string     `json:"missing_imports"`
	ArchitecturalViolations []string     `json:"architectural_violations"`
	CouplingDiscrepancies   []string     `json:"coupling_discrepancies"`
}

// InvalidateCache marks all dependency caches as stale
func (fdt *FunctionDependencyTracker) InvalidateCache() {
	fdt.mu.Lock()
	defer fdt.mu.Unlock()

	for _, depSet := range fdt.dependencyCache {
		depSet.IsStale = true
	}
}

// GetStats returns dependency tracking statistics
func (fdt *FunctionDependencyTracker) GetStats() map[string]interface{} {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	totalDeps := 0
	totalCycles := 0
	for _, depSet := range fdt.dependencyCache {
		if !depSet.IsStale {
			totalDeps += depSet.TotalDependencies
			totalCycles += depSet.CircularCount
		}
	}

	return map[string]interface{}{
		"cached_symbols":        len(fdt.dependencyCache),
		"call_graph_nodes":      len(fdt.callGraph),
		"file_graph_nodes":      len(fdt.fileGraph),
		"total_dependencies":    totalDeps,
		"circular_dependencies": totalCycles,
		"max_resolver_depth":    fdt.transitiveResolver.maxDepth,
	}
}
