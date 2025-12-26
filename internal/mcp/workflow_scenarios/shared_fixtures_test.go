package workflow_scenarios

// Real-World Integration Tests
// =============================
// This package contains integration tests for the Lightning Code Index (LCI) using real open-source projects.
//
// ORGANIZATION BY PROJECT (NOT by LCI feature):
//   Each test file focuses on one real-world project to ensure comprehensive validation:
//   - chi_test.go:         Chi router (Go web framework)
//   - fastapi_test.go:     FastAPI (Python web framework)
//   - nextjs_test.go:      Next.js (TypeScript web framework)
//   - pocketbase_test.go:  Pocketbase (Go SDK)
//   - go_github_test.go:   Google Go GitHub SDK
//   - pydantic_test.go:    Pydantic (Python library)
//   - trpc_test.go:        tRPC (TypeScript framework)
//   - shadcn_ui_test.go:   shadcn/ui (TypeScript component library)
//
// Each test file contains ALL test types for that project:
//   - API Surface Analysis: Testing framework-specific API patterns
//   - Function Analysis:    Testing function/method discovery
//   - Type Discovery:       Testing type/class detection
//   - Linguistic Search:    Testing fuzzy matching, name splitting, etc.
//
// 📚 DOCUMENTATION: /docs/testing/WORKFLOW-TESTING.md
//
// Key Features:
// - Real Codebases: Tests run against actual open-source projects (chi, pocketbase, etc.)
// - Sequential & Independent: Each test runs independently and sequentially to minimize resource usage
// - Lazy Indexing: Projects are indexed only when needed, one at a time
// - In-Process MCP: Uses CallTool() for fast, synchronous testing (no stdio overhead)
//
// IMPORTANT: Tests are designed to run sequentially to avoid resource exhaustion.
// Do NOT run with -parallel flag.
//
// Usage - Full Tests (default):
//   go test ./internal/mcp/workflow_scenarios/
//   → Runs all workflow tests across all projects
//
// Usage - Short Mode (one small project per language):
//   go test -short ./internal/mcp/workflow_scenarios/
//   → Runs minimal tests: chi (Go), fastapi (Python), trpc (TypeScript)
//   → Validates basic functionality without full test suite
//
// Usage - With Detailed Profiling (adds overhead):
//   go test -bottleneck -profile ./internal/mcp/workflow_scenarios/
//   → Runs bottleneck analysis and profiling tests (~120s additional overhead)
//
// Usage - Individual projects:
//   go test -run TestChi
//   go test -run TestFastAPI
//   go test -run TestGoGitHub
//
// See also:
//   - shared_fixtures_test.go: Package-level shared suite infrastructure
//   - internal/mcp/workflow_library.go: Workflow test library
//   - /docs/testing/TESTING-GUIDE.md: Main testing documentation

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/standardbeagle/lci/internal/mcp"
)

var enableLargeProjects = flag.Bool("large-projects", false, "Enable tests on large projects (>35MB): next.js (272MB), shadcn-ui (88MB)")

// Test Project Configuration
// ==========================
// Each test file defines its own independent suite with only the projects it needs.
// This ensures complete test independence while avoiding redundant indexing.

// Per-project cache - indexed once per test run, shared across all test files
var (
	projectCache     = make(map[string]*mcp.WorkflowTestContext)
	projectCacheLock sync.RWMutex
	cacheInitialized bool
)

// Project configurations per test file
var (
	// Chi test suite - Go web framework
	chiProjects = []mcp.ProjectConfig{
		{Language: "go", Name: "chi"},
	}

	// FastAPI test suite - Python web framework
	fastapiProjects = []mcp.ProjectConfig{
		{Language: "python", Name: "fastapi"},
	}

	// Go-GitHub test suite - Google Go GitHub SDK
	goGithubProjects = []mcp.ProjectConfig{
		{Language: "go", Name: "go-github"},
	}

	// Next.js test suite - TypeScript web framework
	nextjsProjects = []mcp.ProjectConfig{
		{Language: "typescript", Name: "next.js"},
	}

	// Pocketbase test suite - Go SDK
	pocketbaseProjects = []mcp.ProjectConfig{
		{Language: "go", Name: "pocketbase"},
	}

	// Pydantic test suite - Python library
	pydanticProjects = []mcp.ProjectConfig{
		{Language: "python", Name: "pydantic"},
	}

	// tRPC test suite - TypeScript framework
	trpcProjects = []mcp.ProjectConfig{
		{Language: "typescript", Name: "trpc"},
	}

	// shadcn/ui test suite - TypeScript component library (LARGE - 88MB)
	// Only run with -large-projects flag
	shadcnUIProjects = []mcp.ProjectConfig{
		{Language: "typescript", Name: "shadcn-ui"},
	}
)

// init ensures cache is initialized once per test run
func init() {
	cacheInitialized = true
}

// GetProject returns a context for the specified project
// Uses a per-project cache to avoid redundant indexing while maintaining test independence
func GetProject(t mcp.Suite, language, project string) *mcp.WorkflowTestContext {
	t.Helper()

	key := language + "/" + project

	// Check cache first - return cached context if available
	projectCacheLock.RLock()
	if ctx, ok := projectCache[key]; ok {
		projectCacheLock.RUnlock()
		return ctx
	}
	projectCacheLock.RUnlock()

	// Not in cache - create and index the project
	// This happens once per project per test run
	suite := mcp.NewWorkflowTestSuite()
	suite.SetupSuite(t, []mcp.ProjectConfig{
		{Language: language, Name: project},
	})

	ctx := suite.GetProject(t, mcp.ProjectConfig{Language: language, Name: project})

	// Cache the result for future use
	projectCacheLock.Lock()
	projectCache[key] = ctx
	projectCacheLock.Unlock()

	return ctx
}

// TestMain coordinates cleanup of all cached projects after tests complete
func TestMain(m *testing.M) {
	// Run all tests
	exitCode := m.Run()

	// Cleanup cached projects after all tests complete
	projectCacheLock.Lock()
	defer projectCacheLock.Unlock()

	if len(projectCache) > 0 {
		os.Stderr.Write([]byte("=== Cleaning up cached projects ===\n"))
		for key, ctx := range projectCache {
			if ctx != nil && ctx.Indexer != nil {
				os.Stderr.Write([]byte(fmt.Sprintf("Cleaning up: %s\n", key)))
				ctx.Indexer.Close()
			}
		}
	}

	os.Exit(exitCode)
}
