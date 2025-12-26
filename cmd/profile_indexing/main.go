package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

func main() {
	project := flag.String("project", "", "Project path to profile")
	cpuprofile := flag.String("cpuprofile", "", "Write CPU profile to file")
	memprofile := flag.String("memprofile", "", "Write memory profile to file")
	mutexprofile := flag.String("mutexprofile", "", "Write mutex contention profile to file")
	blockprofile := flag.String("blockprofile", "", "Write blocking profile to file")
	mutexRate := flag.Int("mutexrate", 1, "Mutex profiling rate (1=all, 0=off)")
	blockRate := flag.Int("blockrate", 1, "Block profiling rate (1=all, 0=off)")
	flag.Parse()

	if *project == "" {
		fmt.Fprintln(os.Stderr, "Usage: profile_indexing -project=<path> [-cpuprofile=<file>] [-memprofile=<file>] [-mutexprofile=<file>] [-blockprofile=<file>]")
		os.Exit(1)
	}

	absPath, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get absolute path: %v\n", err)
		os.Exit(1)
	}

	// Enable mutex profiling if requested
	if *mutexprofile != "" {
		runtime.SetMutexProfileFraction(*mutexRate)
		fmt.Fprintf(os.Stderr, "Mutex profiling enabled (rate=%d)\n", *mutexRate)
	}

	// Enable block profiling if requested
	if *blockprofile != "" {
		runtime.SetBlockProfileRate(*blockRate)
		fmt.Fprintf(os.Stderr, "Block profiling enabled (rate=%d)\n", *blockRate)
	}

	// Start CPU profiling if requested
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	// Load config with defaults (includes exclusion patterns)
	cfg, err := config.LoadWithRoot("", absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override memory limits for profiling
	cfg.Performance.MaxMemoryMB = 1500
	cfg.Performance.MaxGoroutines = 8
	cfg.Project.Root = absPath

	// Profile indexing
	fmt.Fprintf(os.Stderr, "Profiling: %s\n", absPath)
	start := time.Now()

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	err = indexer.IndexDirectory(ctx, absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Indexing error (may be partial): %v\n", err)
	}

	elapsed := time.Since(start)

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get stats
	stats := indexer.GetStats()
	fileCount := stats["file_count"]
	symbolCount := stats["symbol_count"]

	fmt.Fprintf(os.Stderr, "\nResults:\n")
	fmt.Fprintf(os.Stderr, "  Files: %v\n", fileCount)
	fmt.Fprintf(os.Stderr, "  Symbols: %v\n", symbolCount)
	fmt.Fprintf(os.Stderr, "  Time: %v\n", elapsed)
	fmt.Fprintf(os.Stderr, "  Heap Alloc: %.2f MB\n", float64(memStats.HeapAlloc)/(1024*1024))
	fmt.Fprintf(os.Stderr, "  Total Alloc: %.2f MB\n", float64(memStats.TotalAlloc)/(1024*1024))
	fmt.Fprintf(os.Stderr, "  Sys: %.2f MB\n", float64(memStats.Sys)/(1024*1024))

	// Write memory profile if requested
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create memory profile: %v\n", err)
		} else {
			runtime.GC() // Get current heap stats
			if err := pprof.WriteHeapProfile(f); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write memory profile: %v\n", err)
			}
			f.Close()
			fmt.Fprintf(os.Stderr, "\nMemory profile written to: %s\n", *memprofile)
			fmt.Fprintf(os.Stderr, "Analyze with: go tool pprof -top %s\n", *memprofile)
		}
	}

	// Write mutex profile if requested
	if *mutexprofile != "" {
		f, err := os.Create(*mutexprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create mutex profile: %v\n", err)
		} else {
			if p := pprof.Lookup("mutex"); p != nil {
				if err := p.WriteTo(f, 0); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write mutex profile: %v\n", err)
				}
			}
			f.Close()
			fmt.Fprintf(os.Stderr, "\nMutex profile written to: %s\n", *mutexprofile)
			fmt.Fprintf(os.Stderr, "Analyze with: go tool pprof -top %s\n", *mutexprofile)
			fmt.Fprintf(os.Stderr, "Or visualize: go tool pprof -web %s\n", *mutexprofile)
			fmt.Fprintf(os.Stderr, "Show top mutex contention: go tool pprof -list=. %s\n", *mutexprofile)
		}
	}

	// Write block profile if requested
	if *blockprofile != "" {
		f, err := os.Create(*blockprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create block profile: %v\n", err)
		} else {
			if p := pprof.Lookup("block"); p != nil {
				if err := p.WriteTo(f, 0); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write block profile: %v\n", err)
				}
			}
			f.Close()
			fmt.Fprintf(os.Stderr, "\nBlock profile written to: %s\n", *blockprofile)
			fmt.Fprintf(os.Stderr, "Analyze with: go tool pprof -top %s\n", *blockprofile)
			fmt.Fprintf(os.Stderr, "Or visualize: go tool pprof -web %s\n", *blockprofile)
			fmt.Fprintf(os.Stderr, "Show blocking operations: go tool pprof -list=. %s\n", *blockprofile)
		}
	}

	// Cleanup
	indexer.Close()

	if *cpuprofile != "" {
		fmt.Fprintf(os.Stderr, "\nCPU profile written to: %s\n", *cpuprofile)
		fmt.Fprintf(os.Stderr, "Analyze with: go tool pprof -top %s\n", *cpuprofile)
	}

	fmt.Fprintf(os.Stderr, "\n=== Profiling Tips ===\n")
	fmt.Fprintf(os.Stderr, "Mutex contention: Shows where locks are causing threads to wait\n")
	fmt.Fprintf(os.Stderr, "Block profile: Shows where goroutines are blocked (channels, I/O, etc)\n")
	fmt.Fprintf(os.Stderr, "Interactive analysis: go tool pprof <profile_file>\n")
	fmt.Fprintf(os.Stderr, "  Commands: top, list <func>, web, png, pdf\n")
}
