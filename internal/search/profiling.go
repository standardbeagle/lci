package search

import (
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/types"
)

// ProfilingConfig controls profiling behavior
type ProfilingConfig struct {
	Enabled    bool
	CPUProfile string
	MemProfile string
}

// StartCPUProfile starts CPU profiling if enabled
func StartCPUProfile(config ProfilingConfig) func() {
	if !config.Enabled || config.CPUProfile == "" {
		return func() {} // No-op cleanup
	}

	f, err := os.Create(config.CPUProfile)
	if err != nil {
		debug.Printf("Failed to create CPU profile: %v\n", err)
		return func() {}
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		debug.Printf("Failed to start CPU profile: %v\n", err)
		f.Close()
		return func() {}
	}

	// Return cleanup function
	return func() {
		pprof.StopCPUProfile()
		f.Close()
		debug.Printf("CPU profile written to %s\n", config.CPUProfile)
	}
}

// WriteMemProfile writes a memory profile if enabled
func WriteMemProfile(config ProfilingConfig) {
	if !config.Enabled || config.MemProfile == "" {
		return
	}

	f, err := os.Create(config.MemProfile)
	if err != nil {
		debug.Printf("Failed to create memory profile: %v\n", err)
		return
	}
	defer f.Close()

	runtime.GC() // Get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		debug.Printf("Failed to write memory profile: %v\n", err)
		return
	}

	debug.Printf("Memory profile written to %s\n", config.MemProfile)
}

// ProfiledSearch wraps search operations with profiling
func (e *Engine) ProfiledSearch(pattern string, candidates []types.FileID, limit int, config ProfilingConfig) []GrepResult {
	stopProfile := StartCPUProfile(config)
	defer stopProfile()
	defer WriteMemProfile(config)

	start := time.Now()
	results := e.Search(pattern, candidates, limit)
	elapsed := time.Since(start)

	if config.Enabled {
		debug.Printf("Search took %v for %d results\n", elapsed, len(results))
	}

	return results
}

// ProfiledSearchDetailed wraps detailed search with profiling
func (e *Engine) ProfiledSearchDetailed(pattern string, candidates []types.FileID, maxContextLines int, config ProfilingConfig) []StandardResult {
	stopProfile := StartCPUProfile(config)
	defer stopProfile()
	defer WriteMemProfile(config)

	start := time.Now()
	results := e.SearchDetailed(pattern, candidates, maxContextLines)
	elapsed := time.Since(start)

	if config.Enabled {
		debug.Printf("Detailed search took %v for %d results\n", elapsed, len(results))
	}

	return results
}
