package mcp

// Workflow Test Library
// =====================
// Shared library for real-world integration test execution.
//
// TEST ORGANIZATION BY PROJECT:
//   Tests are organized by real-world project (NOT by LCI feature):
//   - Each project is tested comprehensively in a single test file
//   - Tests include API surface, function analysis, type discovery, and linguistic search
//   - Example: TestChi tests the Chi Go framework using all LCI features
//
// Key Features:
// - Package-level shared suite (shared_fixtures_test.go): Eliminates redundant project indexing
// - Serialized project indexing: Prevents concurrent indexing issues
// - Real open-source projects: Tests run against actual production codebases
// - Clean resource management: Proper setup/teardown with TestMain
// - Centralized project configuration: All projects defined in one place
//
// 📚 DOCUMENTATION: /docs/testing/WORKFLOW-TESTING.md

import (
	"sync"
	"time"
)

// WorkflowTestSuite manages a single test suite with serialized project indexing
type WorkflowTestSuite struct {
	projects     map[string]*WorkflowTestContext
	projectsLock sync.RWMutex
	indexedCount int
}

// NewWorkflowTestSuite creates a new workflow test suite
func NewWorkflowTestSuite() *WorkflowTestSuite {
	return &WorkflowTestSuite{
		projects: make(map[string]*WorkflowTestContext),
	}
}

// SetupSuite initializes the test suite
func (suite *WorkflowTestSuite) SetupSuite(t Suite, projectConfigs []ProjectConfig) {
	t.Logf("=== Workflow Test Suite Starting ===")
	t.Logf("Configured projects: %d", len(projectConfigs))

	// Note: Projects are indexed lazily on first use to ensure serialization
	// This prevents multiple tests from indexing simultaneously
}

// CleanupSuite cleans up all indexed projects
func (suite *WorkflowTestSuite) CleanupSuite(t Suite) {
	suite.projectsLock.RLock()
	defer suite.projectsLock.RUnlock()

	if len(suite.projects) > 0 {
		t.Logf("=== Cleaning up %d indexed projects ===", len(suite.projects))
		for key, ctx := range suite.projects {
			t.Logf("Cleaning up: %s", key)
			if ctx != nil && ctx.Indexer != nil {
				ctx.Indexer.Close()
			}
		}
	}
}

// GetProject returns a lazily-indexed project context
// Only one project is indexed at a time due to sync.Once per suite
func (suite *WorkflowTestSuite) GetProject(t Suite, config ProjectConfig) *WorkflowTestContext {
	t.Helper()

	key := config.Language + "/" + config.Name

	// Check if already indexed
	suite.projectsLock.RLock()
	if ctx, ok := suite.projects[key]; ok {
		suite.projectsLock.RUnlock()
		return ctx
	}
	suite.projectsLock.RUnlock()

	// Index the project (serialized - only one can index at a time)
	suite.projectsLock.Lock()
	defer suite.projectsLock.Unlock()

	// Double-check after acquiring write lock
	if ctx, ok := suite.projects[key]; ok {
		return ctx
	}

	t.Logf("Indexing project: %s", key)
	startTime := time.Now()

	ctx, err := SetupRealProject(t, config.Language, config.Name)
	if err != nil {
		t.Fatalf("Failed to index project %s: %v", key, err)
	}

	duration := time.Since(startTime)
	t.Logf("✓ Indexed %s in %v", key, duration)

	suite.projects[key] = ctx
	suite.indexedCount++

	return ctx
}

// ProjectConfig defines a project to be used in tests
type ProjectConfig struct {
	Language string
	Name     string
}

// GetStandardProjects returns the standard set of projects for testing
func GetStandardProjects() []ProjectConfig {
	return []ProjectConfig{
		{Language: "go", Name: "chi"},
		{Language: "go", Name: "pocketbase"},
		{Language: "go", Name: "go-github"},
		{Language: "python", Name: "fastapi"},
		{Language: "python", Name: "pydantic"},
		{Language: "typescript", Name: "next.js"},
		{Language: "typescript", Name: "trpc"},
		{Language: "typescript", Name: "shadcn-ui"},
	}
}

// Suite is an interface that mimics testing.TB for test suite methods
type Suite interface {
	Helper()
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatal(args ...interface{})
	FailNow()
	Skipf(format string, args ...interface{})
	Cleanup(f func())
	TempDir() string
}
