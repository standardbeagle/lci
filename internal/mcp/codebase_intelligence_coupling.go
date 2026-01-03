package mcp

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// ============================================================================
// Coupling and Cohesion Metrics
// ============================================================================

// isCodeFile returns true if the file is a source code file
func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExtensions := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".py": true, ".java": true, ".rs": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".cs": true, ".rb": true, ".php": true,
		".swift": true, ".kt": true, ".scala": true, ".ex": true, ".exs": true,
	}
	return codeExtensions[ext]
}

// getPackageName extracts the package/directory name from a file path
func getPackageName(filePath, projectRoot string) string {
	relPath := getRelativePath(filePath, projectRoot)
	dir := filepath.Dir(relPath)
	if dir == "." {
		return "(root)"
	}
	return dir
}

// getRelativePath returns the path relative to the project root
func getRelativePath(filePath, projectRoot string) string {
	if projectRoot == "" {
		return filePath
	}
	rel, err := filepath.Rel(projectRoot, filePath)
	if err != nil {
		return filePath
	}
	return rel
}

// calculatePackageMetricsFromGraph calculates metrics for a package using graph data
func calculatePackageMetricsFromGraph(packageName string, symbolCount int, packageDeps map[string]map[string]int) (cohesion, coupling, stability float64) {
	// Internal references (self-references within the package)
	deps := packageDeps[packageName]
	internalRefs := deps[packageName]

	// Efferent coupling: outgoing references to other packages
	efferentRefs := 0
	for targetPkg, count := range deps {
		if targetPkg != packageName {
			efferentRefs += count
		}
	}

	// Afferent coupling: incoming references from other packages
	afferentRefs := 0
	for sourcePkg, sourceDeps := range packageDeps {
		if sourcePkg != packageName {
			afferentRefs += sourceDeps[packageName]
		}
	}

	totalRefs := internalRefs + efferentRefs

	// Cohesion: ratio of internal references to total outgoing references
	if totalRefs > 0 {
		cohesion = float64(internalRefs) / float64(totalRefs)
	} else if symbolCount > 0 {
		cohesion = 0.5 // Leaf package with no references
	}

	// Coupling: efferent coupling normalized by symbol count
	if symbolCount > 0 {
		maxExpectedRefs := symbolCount * 5
		coupling = float64(efferentRefs) / float64(maxExpectedRefs)
		if coupling > 1.0 {
			coupling = 1.0
		}
	}

	// Stability (Martin's Instability metric): I = Ce / (Ca + Ce)
	totalCoupling := float64(afferentRefs + efferentRefs)
	if totalCoupling > 0 {
		stability = float64(efferentRefs) / totalCoupling
	} else {
		stability = 0.5 // Isolated package
	}

	return cohesion, coupling, stability
}

// calculateGraphBasedCouplingCohesion computes coupling and cohesion metrics
// using the ReferenceTracker's graph data at the package level
func (s *Server) calculateGraphBasedCouplingCohesion(allFiles []*types.FileInfo) (CouplingMetrics, CohesionMetrics) {
	projectRoot := s.cfg.Project.Root
	refTracker := s.goroutineIndex.GetRefTracker()

	couplingMetrics := CouplingMetrics{
		AfferentCoupling: make(map[string]int),
		EfferentCoupling: make(map[string]int),
		Instability:      make(map[string]float64),
		ModuleCoupling:   make(map[string]float64),
	}
	cohesionMetrics := CohesionMetrics{
		RelationalCohesion: make(map[string]float64),
	}

	// Build fileID -> package mapping
	fileToPackage := make(map[types.FileID]string)
	packageSymbolCount := make(map[string]int)

	for _, file := range allFiles {
		if !isCodeFile(file.Path) {
			continue
		}
		pkgName := getPackageName(file.Path, projectRoot)
		fileToPackage[file.ID] = pkgName
		packageSymbolCount[pkgName] += len(file.EnhancedSymbols)
	}

	// Build package dependency counts using ReferenceTracker
	packageDeps := make(map[string]map[string]int)
	for pkgName := range packageSymbolCount {
		packageDeps[pkgName] = make(map[string]int)
	}

	if refTracker != nil {
		refs := refTracker.GetAllReferences()
		for _, ref := range refs {
			sourcePkg, sourceOK := fileToPackage[ref.FileID]
			if ref.TargetSymbol != 0 {
				targetSym := refTracker.GetEnhancedSymbol(ref.TargetSymbol)
				if targetSym != nil {
					targetPkg, targetOK := fileToPackage[targetSym.FileID]
					if sourceOK && targetOK {
						packageDeps[sourcePkg][targetPkg]++
					}
				}
			}
		}
	}

	// Calculate metrics for each package
	var totalCoupling, maxCoupling float64
	var totalCohesion, minCohesion float64 = 0, 1.0
	var lowCohesionPkgs []string
	packageCount := 0

	for pkgName, deps := range packageDeps {
		packageCount++
		symbolCount := packageSymbolCount[pkgName]

		// Internal refs (self-references)
		internalRefs := deps[pkgName]

		// Efferent (outgoing to other packages)
		efferentRefs := 0
		for targetPkg, count := range deps {
			if targetPkg != pkgName {
				efferentRefs += count
			}
		}

		// Afferent (incoming from other packages)
		afferentRefs := 0
		for sourcePkg, sourceDeps := range packageDeps {
			if sourcePkg != pkgName {
				afferentRefs += sourceDeps[pkgName]
			}
		}

		// Store raw coupling counts
		couplingMetrics.AfferentCoupling[pkgName] = afferentRefs
		couplingMetrics.EfferentCoupling[pkgName] = efferentRefs

		// Calculate instability: I = Ce / (Ca + Ce)
		totalCouplingForPkg := float64(afferentRefs + efferentRefs)
		instability := 0.5
		if totalCouplingForPkg > 0 {
			instability = float64(efferentRefs) / totalCouplingForPkg
		}
		couplingMetrics.Instability[pkgName] = instability

		// Normalized coupling score
		normalizedCoupling := 0.0
		if symbolCount > 0 {
			normalizedCoupling = float64(efferentRefs) / float64(symbolCount*5)
			if normalizedCoupling > 1.0 {
				normalizedCoupling = 1.0
			}
		}
		couplingMetrics.ModuleCoupling[pkgName] = normalizedCoupling
		totalCoupling += normalizedCoupling
		if normalizedCoupling > maxCoupling {
			maxCoupling = normalizedCoupling
		}

		// Cohesion: ratio of internal refs to total refs
		totalRefs := internalRefs + efferentRefs
		cohesion := 0.5
		if totalRefs > 0 {
			cohesion = float64(internalRefs) / float64(totalRefs)
		}
		cohesionMetrics.RelationalCohesion[pkgName] = cohesion
		totalCohesion += cohesion
		if cohesion < minCohesion {
			minCohesion = cohesion
		}
		// Exclude test-related packages from low cohesion reporting
		if cohesion < 0.3 && !s.isInsightExcludedFile(pkgName) {
			lowCohesionPkgs = append(lowCohesionPkgs, pkgName)
		}
	}

	// Calculate averages
	if packageCount > 0 {
		couplingMetrics.AverageCoupling = totalCoupling / float64(packageCount)
		cohesionMetrics.AverageCohesion = totalCohesion / float64(packageCount)
	}
	couplingMetrics.MaxCoupling = maxCoupling
	cohesionMetrics.MinCohesion = minCohesion

	// Sort and limit low cohesion packages
	sort.Slice(lowCohesionPkgs, func(i, j int) bool {
		return cohesionMetrics.RelationalCohesion[lowCohesionPkgs[i]] <
			cohesionMetrics.RelationalCohesion[lowCohesionPkgs[j]]
	})
	if len(lowCohesionPkgs) > 5 {
		lowCohesionPkgs = lowCohesionPkgs[:5]
	}
	cohesionMetrics.LowCohesionModules = lowCohesionPkgs

	return couplingMetrics, cohesionMetrics
}
