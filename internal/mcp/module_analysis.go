package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"github.com/standardbeagle/lci/internal/types"
	"strings"
)

// buildModuleAnalysis implements the Module Detection Engine
// Uses existing file paths and call graph (efficient) instead of universal graph
func (s *Server) buildModuleAnalysis(
	ctx context.Context,
	args CodebaseIntelligenceParams,
) (*ModuleAnalysis, error) {
	// Get all symbols from the index
	allSymbols, err := s.getAllSymbolsFromIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols: %w", err)
	}

	// Apply language filter if specified
	if len(args.Languages) > 0 {
		allSymbols = s.filterSymbolsByLanguage(allSymbols, args.Languages)
	}

	if len(allSymbols) == 0 {
		return nil, errors.New("no symbols found in index (or matching language filter)")
	}

	// Get file count (adjusted for language filter)
	totalFiles := s.goroutineIndex.GetFileCount()
	if len(args.Languages) > 0 {
		// Count unique files in filtered symbols
		fileSet := make(map[types.FileID]bool)
		for _, sym := range allSymbols {
			fileSet[sym.FileID] = true
		}
		totalFiles = len(fileSet)
	}

	// Detect modules using directory structure
	modules := s.detectModulesByStructure(allSymbols, totalFiles)

	// Build module boundaries
	moduleBoundaries := make([]ModuleBoundary, 0)
	for _, mod := range modules {
		moduleBoundaries = append(moduleBoundaries, ModuleBoundary{
			Name:          mod.Name,
			Type:          mod.Type,
			Path:          mod.Path,
			CohesionScore: mod.CohesionScore,
			CouplingScore: mod.CouplingScore,
			Stability:     mod.Stability,
			FileCount:     len(mod.Files),
			FunctionCount: len(mod.Symbols),
		})
	}

	moduleAnalysis := &ModuleAnalysis{
		Modules:           moduleBoundaries,
		DetectionStrategy: "directory_structure",
		ModuleTypes:       map[string]int{},
		LayerDistribution: map[string][]string{},
		Violations:        []Violation{},
		Metrics: ModuleAnalysisMetrics{
			TotalModules:       len(moduleBoundaries),
			AverageCohesion:    calculateAverageModuleCohesion(moduleBoundaries),
			AverageCoupling:    calculateAverageCoupling(moduleBoundaries),
			ArchitecturalScore: 0.8, // Default
		},
	}

	return moduleAnalysis, nil
}

// ModuleInfo represents a detected module
type ModuleInfo struct {
	Name          string
	Type          string
	Path          string
	Files         []string
	Symbols       []string
	CohesionScore float64
	CouplingScore float64
	Stability     float64
}

// detectModulesByStructure detects modules based on directory structure
func (s *Server) detectModulesByStructure(symbols []*types.Symbol, totalFiles int) []ModuleInfo {
	// Group symbols by directory
	dirGroups := make(map[string][]*types.Symbol)
	for _, symbol := range symbols {
		filePath := s.goroutineIndex.GetFilePath(symbol.FileID)
		dir := filepath.Dir(filePath)

		// Normalize path
		dir = filepath.Clean(dir)
		if dir == "." {
			dir = "root"
		}

		dirGroups[dir] = append(dirGroups[dir], symbol)
	}

	// Convert to modules
	modules := make([]ModuleInfo, 0)
	for dir, syms := range dirGroups {
		// Get unique files in this directory
		files := make(map[string]bool)
		for _, sym := range syms {
			filePath := s.goroutineIndex.GetFilePath(sym.FileID)
			files[filePath] = true
		}

		fileList := make([]string, 0, len(files))
		for file := range files {
			fileList = append(fileList, file)
		}

		// Determine module type
		moduleType := classifyModuleByPath(dir)
		moduleName := filepath.Base(dir)
		if moduleName == "." || moduleName == "" {
			moduleName = "root"
		}

		// Calculate metrics
		cohesion := calculateCohesionScore(syms)
		coupling := calculateCouplingScore(syms, dirGroups)
		stability := calculateStabilityScore(syms)

		modules = append(modules, ModuleInfo{
			Name:          moduleName,
			Type:          moduleType,
			Path:          dir,
			Files:         fileList,
			Symbols:       extractSymbolNames(syms),
			CohesionScore: cohesion,
			CouplingScore: coupling,
			Stability:     stability,
		})
	}

	return modules
}

// classifyModuleByPath classifies a module based on its path
func classifyModuleByPath(path string) string {
	lower := strings.ToLower(path)

	if strings.Contains(lower, "api") || strings.Contains(lower, "controller") || strings.Contains(lower, "handler") {
		return "API Layer"
	}
	if strings.Contains(lower, "service") || strings.Contains(lower, "business") || strings.Contains(lower, "logic") {
		return "Service Layer"
	}
	if strings.Contains(lower, "model") || strings.Contains(lower, "entity") || strings.Contains(lower, "data") {
		return "Data Layer"
	}
	if strings.Contains(lower, "repository") || strings.Contains(lower, "dao") {
		return "Repository Layer"
	}
	if strings.Contains(lower, "util") || strings.Contains(lower, "helper") {
		return "Utility"
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "spec") {
		return "Test"
	}
	if strings.Contains(lower, "config") || strings.Contains(lower, "setting") {
		return "Configuration"
	}
	if strings.Contains(lower, "middleware") || strings.Contains(lower, "filter") {
		return "Middleware"
	}

	return "General"
}

// extractSymbolNames extracts symbol names from symbols
func extractSymbolNames(symbols []*types.Symbol) []string {
	names := make([]string, len(symbols))
	for i, sym := range symbols {
		names[i] = sym.Name
	}
	return names
}

// calculateCohesionScore calculates module cohesion based on symbol relationships
func calculateCohesionScore(symbols []*types.Symbol) float64 {
	// Simplified cohesion calculation
	// In a cohesive module, symbols are related by naming conventions and location
	if len(symbols) == 0 {
		return 0.0
	}

	// Check for common prefixes
	prefixCounts := make(map[string]int)
	for _, sym := range symbols {
		// Get first part of name
		parts := strings.Split(sym.Name, "_")
		if len(parts) > 0 {
			prefix := strings.ToLower(parts[0])
			prefixCounts[prefix]++
		}
	}

	// Cohesion is the max prefix count divided by total symbols
	maxCount := 0
	for _, count := range prefixCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	return float64(maxCount) / float64(len(symbols))
}

// calculateCouplingScore calculates module coupling
func calculateCouplingScore(moduleSymbols []*types.Symbol, allDirGroups map[string][]*types.Symbol) float64 {
	// Simplified coupling calculation
	// Count references to symbols in other modules
	if len(moduleSymbols) == 0 {
		return 0.0
	}

	// For now, return a simple score based on module size
	// In reality, we'd analyze the call graph
	return 0.3 // Moderate coupling
}

// calculateStabilityScore calculates module stability
func calculateStabilityScore(symbols []*types.Symbol) float64 {
	// Stability based on complexity and dependencies
	if len(symbols) == 0 {
		return 0.0
	}

	// Simple heuristic: more symbols = less stable
	// In reality, we'd look at incoming/outgoing dependencies
	return 1.0 / (1.0 + float64(len(symbols))/10.0)
}

// calculateAverageModuleSize calculates average module size
func calculateAverageModuleSize(modules []ModuleBoundary) float64 {
	if len(modules) == 0 {
		return 0.0
	}

	total := 0
	for _, m := range modules {
		total += m.FileCount
	}
	return float64(total) / float64(len(modules))
}

// calculateAverageCoupling calculates average coupling score
func calculateAverageCoupling(modules []ModuleBoundary) float64 {
	if len(modules) == 0 {
		return 0.0
	}

	total := 0.0
	for _, m := range modules {
		total += m.CouplingScore
	}
	return total / float64(len(modules))
}

// calculateAverageModuleCohesion calculates average cohesion score
func calculateAverageModuleCohesion(modules []ModuleBoundary) float64 {
	if len(modules) == 0 {
		return 0.0
	}

	total := 0.0
	for _, m := range modules {
		total += m.CohesionScore
	}
	return total / float64(len(modules))
}

// calculateDependencyComplexity calculates dependency complexity
func calculateDependencyComplexity(modules []ModuleBoundary) float64 {
	// Simplified complexity calculation
	// More modules = more potential for complex dependencies
	if len(modules) == 0 {
		return 0.0
	}

	// Heuristic: complexity grows with number of modules
	return float64(len(modules)) * 0.1
}
