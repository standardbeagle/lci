package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

func main() {
	// Create CPU profile
	cpuFile, err := os.Create("workflow_cpu_profile.prof")
	if err != nil {
		panic(err)
	}
	defer cpuFile.Close()

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()

	// Create memory profile
	memFile, err := os.Create("workflow_mem_profile.prof")
	if err != nil {
		panic(err)
	}
	defer memFile.Close()

	// Projects to profile
	projects := []struct {
		language string
		project  string
		timeout  time.Duration
	}{
		{"go", "chi", 60 * time.Second},
		{"go", "pocketbase", 120 * time.Second},
	}

	for _, proj := range projects {
		fmt.Printf("\n=== Profiling %s/%s ===\n", proj.language, proj.project)

		// Get project path
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Failed to get current directory: %v\n", err)
			continue
		}

		const TestDataPath = "real_projects"
		projectPath := filepath.Join(cwd, TestDataPath, proj.language, proj.project)

		// Verify project exists
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			fmt.Printf("Real project not found: %s. Skipping...\n", projectPath)
			continue
		}

		// Create config
		cfg := &config.Config{
			Version: 1,
			Project: config.Project{
				Root: projectPath,
				Name: proj.project,
			},
			Index: config.Index{
				MaxFileSize:      10 * 1024 * 1024,
				MaxTotalSizeMB:   500,
				MaxFileCount:     10000,
				FollowSymlinks:   false,
				SmartSizeControl: true,
				PriorityMode:     "recent",
			},
			Performance: config.Performance{
				MaxMemoryMB:   200,
				MaxGoroutines: 4,
				DebounceMs:    0,
			},
			Search: config.Search{
				MaxResults:         1000,
				MaxContextLines:    100,
				EnableFuzzy:        true,
				MergeFileResults:   true,
				EnsureCompleteStmt: false,
			},
			Include: []string{"*"},
			Exclude: []string{
				"**/node_modules/**",
				"**/vendor/**",
				"**/.git/**",
				"**/dist/**",
				"**/build/**",
				"**/__pycache__/**",
				"**/*.test.go",
				"**/*_test.go",
			},
		}

		// Create indexer
		indexer := indexing.NewMasterIndex(cfg)

		// Track indexing time
		start := time.Now()
		fmt.Printf("Starting indexing of %s/%s at %s\n", proj.language, proj.project, projectPath)

		// Perform indexing
		ctx, cancel := context.WithTimeout(context.Background(), proj.timeout)
		defer cancel()

		err = indexer.IndexDirectory(ctx, projectPath)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("❌ Indexing failed: %v\n", err)
			fmt.Printf("   Indexing took %s before failing\n", elapsed)
		} else {
			fmt.Printf("✓ Indexing completed successfully in %s\n", elapsed)

			// Get statistics
			fileCount := indexer.GetFileCount()
			symbolCount := indexer.GetSymbolCount()
			fmt.Printf("Statistics: %d files, %d symbols\n", fileCount, symbolCount)
			fmt.Printf("Performance: %.2f files/sec, %.2f symbols/sec\n",
				float64(fileCount)/elapsed.Seconds(),
				float64(symbolCount)/elapsed.Seconds())

			// Test search performance
			fmt.Println("\n--- Search Performance Test ---")
			searchPatterns := []string{"func", "import", "type", "const"}
			for _, pattern := range searchPatterns {
				searchStart := time.Now()
				results := indexer.GetTrigramIndex().FindCandidates(pattern)
				searchElapsed := time.Since(searchStart)
				fmt.Printf("Search '%s': %d candidates in %v (%.0f ns/candidate)\n",
					pattern, len(results), searchElapsed,
					float64(searchElapsed.Nanoseconds())/float64(len(results)+1))
			}
		}

		// Clean up
		indexer.Close()
	}

	// Write memory profile
	fmt.Println("\n=== Writing Memory Profile ===")
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		fmt.Printf("Failed to write memory profile: %v\n", err)
	} else {
		fmt.Println("Memory profile written to workflow_mem_profile.prof")
	}

	fmt.Println("\n=== Profiling Complete ===")
	fmt.Println("CPU profile: workflow_cpu_profile.prof")
	fmt.Println("Memory profile: workflow_mem_profile.prof")
}
