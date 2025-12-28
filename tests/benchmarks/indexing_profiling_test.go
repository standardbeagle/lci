package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// TestProfileIndexLoading profiles the indexing process to identify bottlenecks
// This test specifically targets the pocketbase project that times out after 60s
func TestProfileIndexLoading(t *testing.T) {
	projects := []struct {
		language string
		project  string
		timeout  time.Duration
	}{
		{"go", "chi", 30 * time.Second},
		{"go", "pocketbase", 120 * time.Second}, // Extended timeout for profiling
		{"python", "fastapi", 30 * time.Second},
		{"typescript", "next.js", 60 * time.Second},
	}

	for _, proj := range projects {
		t.Run(fmt.Sprintf("%s/%s", proj.language, proj.project), func(t *testing.T) {
			// Get project path
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			// Navigate up from benchmarks directory to find testdata
			baseDir := cwd
			// We're in tests/benchmarks, need to go to project root
			for baseDir != "/" && filepath.Base(baseDir) != "lightning-code-index" {
				baseDir = filepath.Dir(baseDir)
			}

			const TestDataPath = "real_projects"
			projectPath := filepath.Join(baseDir, TestDataPath, proj.language, proj.project)

			// Verify project exists
			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				t.Skipf("Real project not found: %s. Run ./workflow_testdata/scripts/setup_real_codebases.sh", projectPath)
			}

			// Create profile files
			profileName := fmt.Sprintf("indexing_%s_%s", proj.language, proj.project)
			cpuProfileFile := fmt.Sprintf("%s_cpu.prof", profileName)
			memProfileFile := fmt.Sprintf("%s_mem.prof", profileName)
			blockProfileFile := fmt.Sprintf("%s_block.prof", profileName)
			goroutineProfileFile := fmt.Sprintf("%s_goroutine.prof", profileName)

			// Start CPU profiling
			cpuFile, err := os.Create(cpuProfileFile)
			if err != nil {
				t.Fatalf("Failed to create CPU profile file: %v", err)
			}
			defer cpuFile.Close()

			if err := pprof.StartCPUProfile(cpuFile); err != nil {
				t.Fatalf("Failed to start CPU profile: %v", err)
			}
			defer pprof.StopCPUProfile()

			// Create config for indexing
			cfg := createWorkflowConfig(t, projectPath, proj.project)

			// Create indexer
			indexer := indexing.NewMasterIndex(cfg)

			// Track indexing time
			start := time.Now()
			t.Logf("Starting indexing of %s/%s at %s", proj.language, proj.project, projectPath)

			// Perform indexing with context and timeout
			ctx, cancel := context.WithTimeout(context.Background(), proj.timeout)
			defer cancel()

			err = indexer.IndexDirectory(ctx, projectPath)
			elapsed := time.Since(start)

			// Stop CPU profile before checking results
			pprof.StopCPUProfile()

			if err != nil {
				t.Errorf("Indexing failed: %v", err)
				t.Errorf("Indexing took %s before failing", elapsed)
			} else {
				t.Logf("✓ Indexing completed successfully in %s", elapsed)

				// Get statistics
				fileCount := indexer.GetFileCount()
				symbolCount := indexer.GetSymbolCount()
				t.Logf("Statistics: %d files, %d symbols", fileCount, symbolCount)
				t.Logf("Performance: %.2f files/sec, %.2f symbols/sec",
					float64(fileCount)/elapsed.Seconds(),
					float64(symbolCount)/elapsed.Seconds())
			}

			// Write memory profile
			memFile, err := os.Create(memProfileFile)
			if err != nil {
				t.Fatalf("Failed to create memory profile file: %v", err)
			}
			defer memFile.Close()

			if err := pprof.WriteHeapProfile(memFile); err != nil {
				t.Fatalf("Failed to write memory profile: %v", err)
			}

			// Write block profile
			blockFile, err := os.Create(blockProfileFile)
			if err != nil {
				t.Fatalf("Failed to create block profile file: %v", err)
			}
			defer blockFile.Close()

			if err := pprof.Lookup("block").WriteTo(blockFile, 0); err != nil {
				t.Fatalf("Failed to write block profile: %v", err)
			}

			// Write goroutine profile
			goroutineFile, err := os.Create(goroutineProfileFile)
			if err != nil {
				t.Fatalf("Failed to create goroutine profile file: %v", err)
			}
			defer goroutineFile.Close()

			if err := pprof.Lookup("goroutine").WriteTo(goroutineFile, 0); err != nil {
				t.Fatalf("Failed to write goroutine profile: %v", err)
			}

			t.Logf("Profiles written:")
			t.Logf("  CPU: %s", cpuProfileFile)
			t.Logf("  Memory: %s", memProfileFile)
			t.Logf("  Block: %s", blockProfileFile)
			t.Logf("  Goroutine: %s", goroutineProfileFile)

			// Clean up
			indexer.Close()
		})
	}
}

// createWorkflowConfig creates a test configuration optimized for workflow testing
func createWorkflowConfig(t *testing.T, projectPath, projectName string) *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.Project{
			Root: projectPath,
			Name: projectName,
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			MaxTotalSizeMB:   500,              // Allow larger projects
			MaxFileCount:     10000,            // Handle large codebases
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
		},
		Performance: config.Performance{
			MaxMemoryMB:   200,
			MaxGoroutines: 4, // Reasonable for tests
			DebounceMs:    0, // No debounce in tests
		},
		Search: config.Search{
			MaxResults:         1000, // Allow many results
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
}

// BenchmarkIndexLoadTime measures the time to index different projects
func BenchmarkIndexLoadTime(b *testing.B) {
	projects := []struct {
		language string
		project  string
	}{
		{"go", "chi"},
		{"go", "pocketbase"},
		{"python", "fastapi"},
	}

	for _, proj := range projects {
		b.Run(fmt.Sprintf("%s/%s", proj.language, proj.project), func(b *testing.B) {
			// Get project path
			cwd, err := os.Getwd()
			if err != nil {
				b.Fatalf("Failed to get current directory: %v", err)
			}

			baseDir := cwd
			// We're in tests/benchmarks, need to go to project root
			for baseDir != "/" && filepath.Base(baseDir) != "lightning-code-index" {
				baseDir = filepath.Dir(baseDir)
			}

			projectPath := filepath.Join(baseDir, "real_projects", proj.language, proj.project)

			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				b.Skipf("Real project not found: %s", projectPath)
			}

			for i := 0; i < b.N; i++ {
				// Create fresh indexer for each iteration
				cfg := createWorkflowConfig(nil, projectPath, proj.project)
				indexer := indexing.NewMasterIndex(cfg)

				start := time.Now()
				ctx := context.Background()
				err := indexer.IndexDirectory(ctx, projectPath)
				elapsed := time.Since(start)

				if err != nil {
					b.Errorf("Indexing failed on iteration %d: %v", i, err)
				} else {
					fileCount := indexer.GetFileCount()
					b.ReportMetric(elapsed.Seconds(), "seconds")
					b.ReportMetric(float64(fileCount), "files")
					b.ReportMetric(float64(fileCount)/elapsed.Seconds(), "files/sec")
				}

				indexer.Close()
			}
		})
	}
}
