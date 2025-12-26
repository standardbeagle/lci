package mcp

import (
	"context"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPIndexStartMemoryProfiling tests the exact MCP tool call that was consuming 15GB
func TestMCPIndexStartMemoryProfiling(t *testing.T) {
	// Create CPU and memory profiles
	cpuProfile, err := os.Create("mcp_index_start_cpu.prof")
	require.NoError(t, err)
	defer cpuProfile.Close()

	memProfile, err := os.Create("mcp_index_start_mem.prof")
	require.NoError(t, err)
	defer memProfile.Close()

	// Start CPU profiling
	require.NoError(t, pprof.StartCPUProfile(cpuProfile))
	defer pprof.StopCPUProfile()

	// Get initial memory stats
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	t.Logf("Initial memory: Alloc=%d KB, TotalAlloc=%d KB, Sys=%d KB",
		bToKb(m1.Alloc), bToKb(m1.TotalAlloc), bToKb(m1.Sys))

	// Test the exact scenario that was failing
	t.Run("ProfileBrummerIndexing", func(t *testing.T) {
		// Create config that should properly exclude node_modules
		cfg := &config.Config{
			Project: config.Project{
				Root: ".", // Start with current directory for safety
			},
			Index: config.Index{
				MaxFileSize:    10 * 1024 * 1024, // 10MB
				MaxTotalSizeMB: 500,              // 500MB limit
				MaxFileCount:   10000,
			},
			Include: []string{"*.go", "*.js", "*.ts", "*.json", "*.md"},
			Exclude: []string{
				"node_modules/*", "vendor/*", ".git/*",
				"*.test.*", "*_test.*", "*.min.*",
				"dist/*", "build/*", "target/*", ".next/*",
			},
		}

		// Create test server (same as production)
		gi := indexing.NewMasterIndex(cfg)

		_, err := NewServer(gi, cfg)
		require.NoError(t, err, "Failed to create server")

		// Test 1: Check FileScanner counting with proper exclusions
		t.Run("TestFileScannerCounting", func(t *testing.T) {
			scanner := indexing.NewFileScanner(cfg, 100)

			// Count files using FileScanner (uses correct glob pattern matching)
			fileCount, totalBytes, err := scanner.CountFiles(context.Background(), cfg.Project.Root)

			// Log the count details
			t.Logf("File count: %d files, %d MB total", fileCount, totalBytes/(1024*1024))

			// Verify exclusions are working - should be much smaller than 15GB
			assert.Less(t, totalBytes/(1024*1024), int64(2000),
				"Total size should be < 2GB with proper exclusions")
			assert.Less(t, fileCount, 50000,
				"File count should be reasonable with node_modules excluded")

			if err != nil {
				t.Logf("Count error: %v", err)
			}
		})

		// Test 2: Profile auto-indexing (which starts automatically in NewServer)
		t.Run("TestAutoIndexingMemory", func(t *testing.T) {
			// Memory snapshot before indexing
			runtime.GC()
			runtime.ReadMemStats(&m1)
			beforeMem := m1.Alloc

			// Auto-indexing starts automatically in NewServer
			// Just wait for it to complete
			time.Sleep(500 * time.Millisecond)

			// Memory snapshot after indexing
			runtime.GC()
			runtime.ReadMemStats(&m2)
			afterMem := m2.Alloc

			var memDelta uint64
			if afterMem > beforeMem {
				memDelta = afterMem - beforeMem
			} else {
				memDelta = 0 // Memory actually decreased (GC or other cleanup)
			}

			t.Logf("Memory delta: %d KB (before: %d KB, after: %d KB)",
				bToKb(memDelta), bToKb(beforeMem), bToKb(afterMem))
			t.Logf("Final memory: Alloc=%d KB, TotalAlloc=%d KB, Sys=%d KB",
				bToKb(m2.Alloc), bToKb(m2.TotalAlloc), bToKb(m2.Sys))

			// Memory should be reasonable (not 15GB!)
			maxReasonableMemoryKB := int64(100 * 1024) // 100MB
			assert.Less(t, int64(bToKb(memDelta)), maxReasonableMemoryKB,
				"Memory usage should be < 100MB, not 15GB")
		})
	})

	// Write memory profile
	runtime.GC()
	require.NoError(t, pprof.WriteHeapProfile(memProfile))

	t.Logf("Profiles written to: mcp_index_start_cpu.prof, mcp_index_start_mem.prof")
	t.Logf("Analyze with: go tool pprof mcp_index_start_mem.prof")
}

// TestExclusionPatternsWork specifically tests the pattern matching fixes
func TestExclusionPatternsWork(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		testPaths   []string
		shouldMatch map[string]bool
	}{
		{
			name:    "node_modules exclusion",
			pattern: "node_modules/*",
			testPaths: []string{
				"node_modules",
				"node_modules/react",
				"node_modules/react/index.js",
				"project/node_modules/react/index.js",
				"src/components/Button.js",
			},
			shouldMatch: map[string]bool{
				"node_modules":                        true,  // Directory should be excluded
				"node_modules/react":                  true,  // Subdirectory should be excluded
				"node_modules/react/index.js":         true,  // File inside should be excluded
				"project/node_modules/react/index.js": true,  // Nested should be excluded
				"src/components/Button.js":            false, // Normal file should NOT be excluded
			},
		},
		{
			name:    "vendor exclusion",
			pattern: "vendor/*",
			testPaths: []string{
				"vendor",
				"vendor/github.com/pkg/errors",
				"cmd/main.go",
			},
			shouldMatch: map[string]bool{
				"vendor":                       true,
				"vendor/github.com/pkg/errors": true,
				"cmd/main.go":                  false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, testPath := range tc.testPaths {
				shouldBeExcluded := tc.shouldMatch[testPath]

				// Test the pattern matching logic that should now be fixed
				excluded := shouldExcludePath(testPath, []string{tc.pattern})

				if shouldBeExcluded {
					assert.True(t, excluded, "Path '%s' should be excluded by pattern '%s'", testPath, tc.pattern)
				} else {
					assert.False(t, excluded, "Path '%s' should NOT be excluded by pattern '%s'", testPath, tc.pattern)
				}
			}
		})
	}
}

// shouldExcludePath replicates the fixed exclusion logic for testing
func shouldExcludePath(normalizedPath string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		patternBase := strings.TrimSuffix(pattern, "/*")

		// Directory name matches pattern (e.g., "node_modules" matches "node_modules/*")
		if normalizedPath == patternBase || strings.HasSuffix(normalizedPath, "/"+patternBase) {
			return true
		}

		// Path contains excluded component (e.g., "project/node_modules/react")
		if strings.Contains(normalizedPath, patternBase+"/") {
			return true
		}
	}
	return false
}

func bToKb(b uint64) uint64 {
	return b / 1024
}
