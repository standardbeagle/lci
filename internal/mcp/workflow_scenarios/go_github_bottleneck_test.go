package workflow_scenarios

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

var enableBottleneckTests = flag.Bool("bottleneck", false, "Enable bottleneck analysis tests (adds significant overhead)")
var enableProfilingTests = flag.Bool("profile", false, "Enable profiling tests (adds CPU/memory profiling overhead)")

// TestGoGitHubBottleneck identifies where the go-github indexing gets stuck
// This test adds timing and logging at each phase to identify bottlenecks
func TestGoGitHubBottleneck(t *testing.T) {
	if !*enableBottleneckTests {
		t.Skip("Skipping bottleneck test (use -bottleneck flag to enable)")
	}
	if testing.Short() {
		t.Skip("Skipping bottleneck test in short mode")
	}

	// Create log file
	logFile, err := os.Create("gogithub_bottleneck.log")
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	log := func(msg string) {
		t.Log(msg)
		fmt.Fprintln(logFile, "["+time.Now().Format("15:04:05")+"] "+msg)
		logFile.Sync()
	}

	log("=== Starting GoGitHub Bottleneck Analysis ===")

	// Use the same path resolution as workflow tests
	// This mimics what GetProject() does internally
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Go up from workflow_scenarios to project root
	baseDir := cwd
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(baseDir, "real_projects")); err == nil {
			break
		}
		parent := filepath.Dir(baseDir)
		if parent == baseDir {
			break
		}
		baseDir = parent
	}

	projectPath := filepath.Join(baseDir, "real_projects", "go", "go-github")
	log("Project path: " + projectPath)

	// Check if project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		log("ERROR: Project not found")
		t.Fatalf("Real project not found: %s", projectPath)
	}

	// Count files
	log("Phase 1: Scanning project files")
	fileCount := 0
	err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		if filepath.Ext(path) == ".go" {
			fileCount++
		}
		return nil
	})
	log(fmt.Sprintf("Found %d Go files", fileCount))

	if fileCount == 0 {
		t.Fatalf("No Go files found in %s", projectPath)
	}

	// Load config
	log("Phase 2: Loading configuration")
	cfg, err := config.Load("")
	if err != nil {
		log("ERROR: Failed to load config")
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg.Project.Root = projectPath
	cfg.Project.Name = "go-github"

	// Enrich exclusions after setting root (enrich returns early if root is empty)
	cfg.EnrichExclusionsWithBuildArtifacts()

	log("Config loaded successfully")

	// Create indexer with timeout
	log("Phase 3: Creating MasterIndex")
	indexer := indexing.NewMasterIndex(cfg)
	defer indexer.Close()
	log("MasterIndex created")

	// Start indexing with tracking
	log("Phase 4: Starting indexing with timeout tracking")
	ctx := context.Background()
	indexStart := time.Now()

	// Use a channel to track completion
	done := make(chan error, 1)
	go func() {
		done <- indexer.IndexDirectory(ctx, projectPath)
	}()

	// Wait with progress reporting
	timeout := 5 * time.Minute
	select {
	case err := <-done:
		indexDuration := time.Since(indexStart)
		if err != nil {
			log("ERROR: Indexing failed")
			log("Indexing duration: " + indexDuration.String())
			t.Fatalf("Indexing failed: %v (duration: %v)", err, indexDuration)
		}
		log("✓ Indexing completed successfully!")
		log("Indexing duration: " + indexDuration.String())

	case <-time.After(timeout):
		log("ERROR: Indexing TIMEOUT after " + timeout.String())

		// Get stats to see how far we got
		stats := indexer.GetStats()
		log("Stats at timeout: " + fmt.Sprintf("%+v", stats))

		t.Fatalf("Indexing timeout after %v. Check gogithub_bottleneck.log for details.", timeout)
	}

	// Success - log final stats
	log("Phase 5: Final indexing statistics")
	finalStats := indexer.GetStats()
	log("Final stats: " + fmt.Sprintf("%+v", finalStats))

	log("=== GoGitHub Bottleneck Analysis Complete ===")
	log("Log file: gogithub_bottleneck.log")
}
