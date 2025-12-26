package metrics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// CodebaseStats represents comprehensive codebase metrics derived from index data
type CodebaseStats struct {
	// File-level metrics
	TotalFiles           int64
	TotalSizeBytes       int64
	LanguageDistribution map[string]FileLanguageStats

	// Symbol-level metrics
	TotalSymbols       int64
	TotalDefinitions   int64
	SymbolDistribution map[string]int64

	// Complexity metrics
	AverageFunctionLength int64
	MaxFunctionLength     int64
	AverageSymbolsPerFile int64

	// Call graph statistics
	TotalCallEdges int64
	AverageFanOut  float64
	AverageFanIn   float64
	MaxCallDepth   int64

	// Reference statistics
	TotalReferences        int64
	MaxReferencesPerSymbol int64
	OrphanSymbols          int64 // Symbols with no references

	// Architectural insights
	EntryPoints         int64
	ServiceDependencies int64
}

// FileLanguageStats represents metrics for a specific language
type FileLanguageStats struct {
	FileCount      int64
	SymbolCount    int64
	TotalSizeBytes int64
	FileExtensions map[string]int64 // extension -> count
}

// SymbolTypeStats breaks down symbols by type
type SymbolTypeStats struct {
	Functions  int64
	Classes    int64
	Methods    int64
	Variables  int64
	Constants  int64
	Interfaces int64
	Structs    int64
	Other      int64
}

// NewCodebaseStats creates a new CodebaseStats calculator
func NewCodebaseStats() *CodebaseStats {
	return &CodebaseStats{
		LanguageDistribution: make(map[string]FileLanguageStats),
		SymbolDistribution:   make(map[string]int64),
	}
}

// CalculateFromIndex computes all metrics from the index data
func (cs *CodebaseStats) CalculateFromIndex(
	indexer interface{},
	fileMap map[string]types.FileID,
	reverseFileMap map[types.FileID]string,
	symbolIndex *core.SymbolIndex,
	refTracker *core.ReferenceTracker,
	componentDetector *core.ComponentDetector,
) error {
	// This will be populated with actual implementation
	// For now, return nil to allow compilation
	return nil
}

// ComputeLanguageDistribution derives language stats from file paths and extensions
func (cs *CodebaseStats) ComputeLanguageDistribution(
	reverseFileMap map[types.FileID]string,
	fileContentStore *core.FileContentStore,
) {
	cs.LanguageDistribution = make(map[string]FileLanguageStats)

	languagesByExtension := map[string]string{
		".go":    "Go",
		".js":    "JavaScript",
		".ts":    "TypeScript",
		".py":    "Python",
		".java":  "Java",
		".cpp":   "C++",
		".c":     "C",
		".rs":    "Rust",
		".rb":    "Ruby",
		".php":   "PHP",
		".swift": "Swift",
		".kt":    "Kotlin",
		".scala": "Scala",
		".sh":    "Shell",
		".sql":   "SQL",
	}

	for _, filePath := range reverseFileMap {
		// Extract extension
		parts := strings.Split(filePath, ".")
		if len(parts) < 2 {
			continue
		}
		ext := "." + parts[len(parts)-1]

		// Determine language
		lang, ok := languagesByExtension[ext]
		if !ok {
			lang = "Other"
		}

		// Update stats
		stats, exists := cs.LanguageDistribution[lang]
		if !exists {
			stats = FileLanguageStats{
				FileExtensions: make(map[string]int64),
			}
		}

		stats.FileCount++
		stats.FileExtensions[ext]++

		cs.LanguageDistribution[lang] = stats
		cs.TotalFiles++
	}
}

// ComputeSymbolDistribution derives symbol type distribution from symbol index
func (cs *CodebaseStats) ComputeSymbolDistribution(symbolIndex *core.SymbolIndex) SymbolTypeStats {
	stats := SymbolTypeStats{}

	// This will iterate through symbol index and count by type
	// Implementation depends on symbolIndex internals
	cs.TotalSymbols = int64(symbolIndex.Count())
	cs.TotalDefinitions = int64(symbolIndex.DefinitionCount())

	return stats
}

// ComputeReferenceMetrics derives reference statistics
func (cs *CodebaseStats) ComputeReferenceMetrics(refTracker *core.ReferenceTracker) {
	if refTracker == nil {
		return
	}

	// Compute reference statistics
	// - Total references
	// - Max references per symbol
	// - Orphan symbols (no references)

	cs.TotalReferences = 0
	cs.MaxReferencesPerSymbol = 0
	cs.OrphanSymbols = 0
}

// ComputeArchitecturalMetrics derives architectural insights
func (cs *CodebaseStats) ComputeArchitecturalMetrics(
	componentDetector *core.ComponentDetector,
) {
	if componentDetector == nil {
		return
	}

	// Compute architectural metrics
	// - Entry points
	// - Service dependencies

	cs.EntryPoints = 0
	cs.ServiceDependencies = 0
}

// FormatAsJSON returns stats formatted as JSON-serializable map
func (cs *CodebaseStats) FormatAsJSON() map[string]interface{} {
	languageStats := make([]map[string]interface{}, 0)
	for lang, stats := range cs.LanguageDistribution {
		languageStats = append(languageStats, map[string]interface{}{
			"language":   lang,
			"files":      stats.FileCount,
			"symbols":    stats.SymbolCount,
			"size_bytes": stats.TotalSizeBytes,
			"extensions": stats.FileExtensions,
		})
	}

	// Sort for consistent output
	sort.Slice(languageStats, func(i, j int) bool {
		return languageStats[i]["language"].(string) < languageStats[j]["language"].(string)
	})

	return map[string]interface{}{
		"summary": map[string]interface{}{
			"total_files":       cs.TotalFiles,
			"total_symbols":     cs.TotalSymbols,
			"total_size_mb":     float64(cs.TotalSizeBytes) / 1024.0 / 1024.0,
			"total_definitions": cs.TotalDefinitions,
		},
		"languages": languageStats,
		"symbols": map[string]interface{}{
			"total":          cs.TotalSymbols,
			"definitions":    cs.TotalDefinitions,
			"references":     cs.TotalReferences,
			"max_references": cs.MaxReferencesPerSymbol,
			"orphans":        cs.OrphanSymbols,
		},
		"complexity": map[string]interface{}{
			"avg_function_length": cs.AverageFunctionLength,
			"max_function_length": cs.MaxFunctionLength,
			"symbols_per_file":    cs.AverageSymbolsPerFile,
		},
		"call_graph": map[string]interface{}{
			"total_edges": cs.TotalCallEdges,
			"avg_fan_out": cs.AverageFanOut,
			"avg_fan_in":  cs.AverageFanIn,
			"max_depth":   cs.MaxCallDepth,
		},
		"architecture": map[string]interface{}{
			"entry_points": cs.EntryPoints,
			"dependencies": cs.ServiceDependencies,
		},
	}
}

// FormatAsText returns stats formatted as human-readable text
func (cs *CodebaseStats) FormatAsText() string {
	var sb strings.Builder

	sb.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║               LIGHTNING CODE INDEX - CODEBASE REPORT              ║\n")
	sb.WriteString("╚════════════════════════════════════════════════════════════════╝\n\n")

	// Summary section
	sb.WriteString("📊 SUMMARY\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Total Files:        %d\n", cs.TotalFiles))
	sb.WriteString(fmt.Sprintf("  Total Symbols:      %d\n", cs.TotalSymbols))
	sb.WriteString(fmt.Sprintf("  Total Size:         %.2f MB\n", float64(cs.TotalSizeBytes)/1024.0/1024.0))
	sb.WriteString(fmt.Sprintf("  Total Definitions:  %d\n", cs.TotalDefinitions))

	// Language distribution
	sb.WriteString("\n📈 LANGUAGE DISTRIBUTION\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")

	// Sort languages by file count
	type langStats struct {
		name  string
		stats FileLanguageStats
	}
	var langs []langStats
	for name, stats := range cs.LanguageDistribution {
		langs = append(langs, langStats{name, stats})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].stats.FileCount > langs[j].stats.FileCount
	})

	for _, lang := range langs {
		sb.WriteString(fmt.Sprintf("  %-12s %5d files  %8d symbols  %7.2f MB\n",
			lang.name+":",
			lang.stats.FileCount,
			lang.stats.SymbolCount,
			float64(lang.stats.TotalSizeBytes)/1024.0/1024.0,
		))
	}

	// Symbols section
	sb.WriteString("\n🔤 SYMBOLS\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Total:              %d\n", cs.TotalSymbols))
	sb.WriteString(fmt.Sprintf("  Definitions:        %d\n", cs.TotalDefinitions))
	sb.WriteString(fmt.Sprintf("  References:         %d\n", cs.TotalReferences))
	sb.WriteString(fmt.Sprintf("  Max per Symbol:     %d\n", cs.MaxReferencesPerSymbol))
	sb.WriteString(fmt.Sprintf("  Orphan Symbols:     %d\n", cs.OrphanSymbols))

	// Complexity section
	sb.WriteString("\n📏 COMPLEXITY\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Avg Function Length: %d lines\n", cs.AverageFunctionLength))
	sb.WriteString(fmt.Sprintf("  Max Function Length: %d lines\n", cs.MaxFunctionLength))
	sb.WriteString(fmt.Sprintf("  Symbols per File:    %.1f\n", float64(cs.AverageSymbolsPerFile)))

	// Call graph section
	sb.WriteString("\n🔗 CALL GRAPH\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Total Edges:        %d\n", cs.TotalCallEdges))
	sb.WriteString(fmt.Sprintf("  Avg Fan-Out:        %.2f\n", cs.AverageFanOut))
	sb.WriteString(fmt.Sprintf("  Avg Fan-In:         %.2f\n", cs.AverageFanIn))
	sb.WriteString(fmt.Sprintf("  Max Call Depth:     %d\n", cs.MaxCallDepth))

	// Architecture section
	sb.WriteString("\n🏗️  ARCHITECTURE\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Entry Points:       %d\n", cs.EntryPoints))
	sb.WriteString(fmt.Sprintf("  Dependencies:       %d\n", cs.ServiceDependencies))

	return sb.String()
}
