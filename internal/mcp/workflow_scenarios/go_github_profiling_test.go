package workflow_scenarios

import (
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestGoGitHubProfiling profiles the TestGoGitHub test to identify hotspots
// Run with: go test -v -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/mcp/workflow_scenarios/ -run TestGoGitHubProfiling
func TestGoGitHubProfiling(t *testing.T) {
	if !*enableProfilingTests {
		t.Skip("Skipping profiling test (use -profile flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping profiling test in short mode")
	}

	// Create CPU profile file
	cpuFile, err := os.Create("testgogithub_cpu.prof")
	if err != nil {
		t.Fatalf("Failed to create CPU profile file: %v", err)
	}
	defer cpuFile.Close()

	err = pprof.StartCPUProfile(cpuFile)
	if err != nil {
		t.Fatalf("Failed to start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	// Create memory profile file
	memFile, err := os.Create("testgogithub_mem.prof")
	if err != nil {
		t.Fatalf("Failed to create memory profile file: %v", err)
	}
	defer memFile.Close()

	startTime := time.Now()
	t.Logf("=== Starting TestGoGitHub Profiling ===")
	t.Logf("Project: go/go-github")
	t.Logf("Profile files: testgogithub_cpu.prof, testgogithub_mem.prof")

	// Run the actual test
	t.Run("Indexing_Phase", func(t *testing.T) {
		t.Logf("Phase 1: Indexing go-github project")

		ctx := GetProject(t, "go", "go-github")

		// Write memory profile after indexing
		pprof.WriteHeapProfile(memFile)
		t.Logf("✓ Indexed project: %s", ctx.ProjectPath)
	})

	// Additional profiling runs
	t.Run("Search_Operations", func(t *testing.T) {
		ctx := GetProject(t, "go", "go-github")

		t.Logf("Phase 2: Running search operations")

		// Repository search
		searchResult := ctx.Search("Repository", mcp.SearchOptions{
			Pattern:         "type Repository",
			SymbolTypes:     []string{"type"},
			DeclarationOnly: true,
			MaxResults:      10,
		})
		t.Logf("Repository search: found %d results", len(searchResult.Results))

		// PullRequest search
		prResult := ctx.Search("PullRequest", mcp.SearchOptions{
			Pattern:         "type PullRequest",
			SymbolTypes:     []string{"type"},
			DeclarationOnly: true,
			MaxResults:      10,
		})
		t.Logf("PullRequest search: found %d results", len(prResult.Results))
	})

	duration := time.Since(startTime)
	t.Logf("=== TestGoGitHub Profiling Complete ===")
	t.Logf("Total duration: %v", duration)
	t.Logf("Analyze profiles with:")
	t.Logf("  go tool pprof -http=:8080 testgogithub_cpu.prof")
	t.Logf("  go tool pprof -http=:8080 testgogithub_mem.prof")
}
