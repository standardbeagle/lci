package mcp

// MCP Workflow Test Helpers
// ==========================
// This file provides utilities for testing MCP workflows on real codebases.
//
// 📚 DOCUMENTATION: /docs/testing/WORKFLOW-TESTING.md
//
// Key Features:
// - WorkflowTestContext: Manages indexed projects and MCP server for tests
// - In-Process Testing: Uses CallTool() instead of stdio for fast, reliable tests
// - Real Codebases: Tests run against actual open-source projects
//
// Quick Start:
//   ctx, err := SetupRealProject(t, "go", "chi")
//   if err != nil {
//       t.Fatalf("Failed to setup: %v", err)
//   }
//   results := ctx.Search("name", SearchOptions{Pattern: "ServeHTTP"})
//   context := ctx.GetObjectContext("name", 0, ContextOptions{...})
//
// See also:
//   - /docs/testing/TESTING-GUIDE.md          - Complete testing documentation
//   - /docs/testing/WORKFLOW-TESTING.md       - Workflow testing patterns
//   - /docs/qualitative-testing/README.md     - Qualitative testing framework

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// Test data path constants
const (
	// TestDataPath is the path to test data directory containing real projects
	TestDataPath = "real_projects"
)

// WorkflowTestContext provides context for workflow integration tests
type WorkflowTestContext struct {
	T              Suite
	Indexer        *indexing.MasterIndex
	Server         *Server
	ProjectPath    string
	ProjectName    string
	SearchResults  map[string]*SearchResponse
	ContextResults map[string]map[string]interface{}
}

// SetupRealProject indexes a real-world project from real_projects
// Returns an error instead of using require.NoError to allow safe usage in sync.Once.Do()
func SetupRealProject(t Suite, language, projectName string) (*WorkflowTestContext, error) {
	t.Helper()

	// Build path to real project
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Navigate up to project root directory to find real_projects
	// Find the deepest known subdirectory and go up accordingly
	baseDir := cwd
	switch filepath.Base(cwd) {
	case "workflow_scenarios":
		// Go up 3 levels: workflow_scenarios -> mcp -> internal -> project_root
		baseDir = filepath.Dir(filepath.Dir(filepath.Dir(cwd)))
	case "mcp":
		// Go up 2 levels: mcp -> internal -> project_root
		baseDir = filepath.Dir(filepath.Dir(cwd))
	case "internal":
		// Go up 1 level: internal -> project_root
		baseDir = filepath.Dir(cwd)
	default:
		// Already at or above project root
		// Go up until we find TestDataPath or give up
		for i := 0; i < 4; i++ {
			if _, err := os.Stat(filepath.Join(baseDir, TestDataPath)); err == nil {
				break
			}
			parent := filepath.Dir(baseDir)
			if parent == baseDir {
				break // Reached filesystem root
			}
			baseDir = parent
		}
	}

	projectPath := filepath.Join(baseDir, TestDataPath, language, projectName)

	// Verify project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		t.Skipf("Real project not found: %s. Run ./workflow_testdata/scripts/setup_real_codebases.sh", projectPath)
	}

	// Create config for indexing
	cfg := createWorkflowConfig(t, projectPath, projectName)

	// Create indexer
	indexer := indexing.NewMasterIndex(cfg)

	// Create MCP server (auto-indexing starts automatically in NewServer)
	server, err := NewServer(indexer, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server: %w", err)
	}

	t.Logf("Waiting for auto-indexing to complete for %s/%s...", language, projectName)

	// Wait for the auto-indexing (started in NewServer) to complete
	status, err := server.autoIndexManager.waitForCompletion(180 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("auto-indexing did not complete for project %s: %w", projectName, err)
	}
	if status != "completed" {
		return nil, fmt.Errorf("auto-indexing should complete successfully, got status: %s", status)
	}

	// Log index stats
	fileCount := indexer.GetFileCount()
	symbolCount := indexer.GetSymbolCount()
	t.Logf("✓ Indexed %d files, %d symbols for %s", fileCount, symbolCount, projectName)

	return &WorkflowTestContext{
		T:              t,
		Indexer:        indexer,
		Server:         server,
		ProjectPath:    projectPath,
		ProjectName:    projectName,
		SearchResults:  make(map[string]*SearchResponse),
		ContextResults: make(map[string]map[string]interface{}),
	}, nil
}

// createWorkflowConfig creates a test configuration optimized for workflow testing
func createWorkflowConfig(t Suite, projectPath, projectName string) *config.Config {
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
		Include: []string{}, // Use default: include everything
		Exclude: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/.git/**",
			"**/dist/**",
			"**/build/**",
			"**/__pycache__/**",
		},
	}
}

// SearchOptions configures a search operation
type SearchOptions struct {
	Pattern            string
	Output             string
	MaxResults         int
	MaxLineCount       int
	IncludeIDs         bool
	IncludeBreadcrumbs bool
	SymbolTypes        []string
	DeclarationOnly    bool
	UsageOnly          bool
	ExportedOnly       bool
	CaseInsensitive    bool
	UseRegex           bool
}

// Search executes a search and stores the result
func (ctx *WorkflowTestContext) Search(name string, opts SearchOptions) *SearchResponse {
	ctx.T.Helper()

	// Set defaults
	if opts.Output == "" {
		opts.Output = "context"
	}
	if opts.MaxResults == 0 {
		opts.MaxResults = 100
	}
	if opts.MaxLineCount == 0 {
		opts.MaxLineCount = 5
	}

	params := map[string]interface{}{
		"pattern": opts.Pattern,
		"max":     opts.MaxResults,
	}

	// Convert output_size to new format
	if opts.Output != "" {
		if opts.MaxLineCount > 0 {
			params["output"] = fmt.Sprintf("ctx:%d", opts.MaxLineCount)
		} else {
			params["output"] = string(opts.Output)
		}
	}

	// Convert include flags to comma-separated string
	var includeItems []string
	if opts.IncludeBreadcrumbs {
		includeItems = append(includeItems, "breadcrumbs")
	}
	if len(includeItems) > 0 {
		params["include"] = strings.Join(includeItems, ",")
	}

	// Convert symbol_types to comma-separated string
	if len(opts.SymbolTypes) > 0 {
		params["symbol_types"] = strings.Join(opts.SymbolTypes, ",")
	}

	// Convert boolean flags to comma-separated string
	var flags []string
	if opts.DeclarationOnly {
		flags = append(flags, "dl")
	}
	if opts.UsageOnly {
		flags = append(flags, "ul")
	}
	if opts.ExportedOnly {
		flags = append(flags, "eo")
	}
	if opts.CaseInsensitive {
		flags = append(flags, "ci")
	}
	if opts.UseRegex {
		flags = append(flags, "rx")
	}
	if len(flags) > 0 {
		params["flags"] = strings.Join(flags, ",")
	}

	// Enable semantic search (default behavior)
	params["semantic"] = true

	ctx.T.Logf("Executing search '%s': pattern=%s", name, opts.Pattern)

	searchStart := time.Now()
	resultJSON, err := ctx.Server.CallTool("search", params)
	searchElapsed := time.Since(searchStart)
	require.NoError(ctx.T, err, "Search '%s' failed: %v", name, err)

	var response SearchResponse
	err = json.Unmarshal([]byte(resultJSON), &response)
	require.NoError(ctx.T, err, "Failed to parse search response for '%s'", name)

	ctx.T.Logf("Search '%s' found %d results in %v", name, len(response.Results), searchElapsed)

	// Store result with provided name for fast lookup
	ctx.SearchResults[name] = &response

	return &response
}

// ContextOptions configures a context lookup operation
type ContextOptions struct {
	IncludeFullSymbol     bool
	IncludeCallHierarchy  bool
	IncludeAllReferences  bool
	IncludeDependencies   bool
	IncludeFileContext    bool
	IncludeQualityMetrics bool
}

// GetObjectContext retrieves detailed context for a search result
func (ctx *WorkflowTestContext) GetObjectContext(searchName string, resultIndex int, opts ContextOptions) map[string]interface{} {
	ctx.T.Helper()

	// Get search result
	searchResult, ok := ctx.SearchResults[searchName]
	require.True(ctx.T, ok, "No search result found with name '%s'", searchName)
	require.Greater(ctx.T, len(searchResult.Results), resultIndex, "Result index %d out of range for search '%s'", resultIndex, searchName)

	result := searchResult.Results[resultIndex]

	// Verify the result has an object_id (should always be present for symbol searches)
	require.NotEmpty(ctx.T, result.ObjectID,
		"Search result %d has no object_id. Use SymbolTypes in search to ensure symbol-only results.", resultIndex)

	objectID := result.ObjectID
	ctx.T.Logf("Getting context for search '%s' result %d (object_id=%s)", searchName, resultIndex, objectID)

	// Build parameters using the simplified 'id' parameter
	params := map[string]interface{}{
		"id": objectID,
	}

	// Set context options (default to all if none specified)
	if !opts.IncludeFullSymbol && !opts.IncludeCallHierarchy && !opts.IncludeAllReferences &&
		!opts.IncludeDependencies && !opts.IncludeFileContext && !opts.IncludeQualityMetrics {
		// Default: include everything
		params["include_full_symbol"] = true
		params["include_call_hierarchy"] = true
		params["include_all_references"] = true
		params["include_dependencies"] = true
		params["include_file_context"] = true
		params["include_quality_metrics"] = true
	} else {
		// Use specified options
		if opts.IncludeFullSymbol {
			params["include_full_symbol"] = true
		}
		if opts.IncludeCallHierarchy {
			params["include_call_hierarchy"] = true
		}
		if opts.IncludeAllReferences {
			params["include_all_references"] = true
		}
		if opts.IncludeDependencies {
			params["include_dependencies"] = true
		}
		if opts.IncludeFileContext {
			params["include_file_context"] = true
		}
		if opts.IncludeQualityMetrics {
			params["include_quality_metrics"] = true
		}
	}

	// Execute get_object_context
	contextStart := time.Now()
	contextJSON, err := ctx.Server.CallTool("get_object_context", params)
	contextElapsed := time.Since(contextStart)
	require.NoError(ctx.T, err, "get_object_context failed for search '%s' result %d", searchName, resultIndex)

	var contextResponse map[string]interface{}
	err = json.Unmarshal([]byte(contextJSON), &contextResponse)
	require.NoError(ctx.T, err, "Failed to parse context response")

	ctx.T.Logf("Context lookup completed in %v", contextElapsed)

	// Store result
	contextKey := fmt.Sprintf("%s[%d]", searchName, resultIndex)
	ctx.ContextResults[contextKey] = contextResponse

	return contextResponse
}

// AssertFieldExists checks that a field exists in the context response
func (ctx *WorkflowTestContext) AssertFieldExists(contextResult map[string]interface{}, jsonPath string) {
	ctx.T.Helper()

	value := getJSONPath(contextResult, jsonPath)
	assert.NotNil(ctx.T, value, "Field %s should exist", jsonPath)
}

// AssertFieldEquals checks that a field equals an expected value
func (ctx *WorkflowTestContext) AssertFieldEquals(contextResult map[string]interface{}, jsonPath string, expected interface{}) {
	ctx.T.Helper()

	value := getJSONPath(contextResult, jsonPath)
	assert.Equal(ctx.T, expected, value, "Field %s should equal %v", jsonPath, expected)
}

// AssertFieldContains checks that a field contains a substring (for string fields)
func (ctx *WorkflowTestContext) AssertFieldContains(contextResult map[string]interface{}, jsonPath string, substring string) {
	ctx.T.Helper()

	value := getJSONPath(contextResult, jsonPath)
	if strValue, ok := value.(string); ok {
		assert.Contains(ctx.T, strValue, substring, "Field %s should contain '%s'", jsonPath, substring)
	} else {
		ctx.T.Errorf("Field %s is not a string, got %T", jsonPath, value)
	}
}

// AssertFieldGreaterThan checks that a numeric field is greater than a value
func (ctx *WorkflowTestContext) AssertFieldGreaterThan(contextResult map[string]interface{}, jsonPath string, threshold float64) {
	ctx.T.Helper()

	value := getJSONPath(contextResult, jsonPath)

	var numValue float64
	switch v := value.(type) {
	case float64:
		numValue = v
	case int:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	default:
		ctx.T.Errorf("Field %s is not numeric, got %T", jsonPath, value)
		return
	}

	assert.Greater(ctx.T, numValue, threshold, "Field %s should be > %f", jsonPath, threshold)
}

// AssertNoError checks that a field does NOT contain an error
func (ctx *WorkflowTestContext) AssertNoError(contextResult map[string]interface{}, jsonPath string) {
	ctx.T.Helper()

	// Check for .error field
	errorPath := jsonPath + ".error"
	errorValue := getJSONPath(contextResult, errorPath)

	if errorValue != nil {
		ctx.T.Errorf("Field %s contains unexpected error: %v", jsonPath, errorValue)
	}

	// Also check for tree_error field (call hierarchy specific)
	treeErrorPath := jsonPath + ".tree_error"
	treeErrorValue := getJSONPath(contextResult, treeErrorPath)

	if treeErrorValue != nil {
		ctx.T.Errorf("Field %s contains unexpected tree_error: %v", jsonPath, treeErrorValue)
	}
}

// AssertFieldNotExists checks that a field does NOT exist
func (ctx *WorkflowTestContext) AssertFieldNotExists(contextResult map[string]interface{}, jsonPath string) {
	ctx.T.Helper()

	value := getJSONPath(contextResult, jsonPath)
	assert.Nil(ctx.T, value, "Field %s should not exist but found: %v", jsonPath, value)
}

// getJSONPath retrieves a value from a map using a dot-separated path
// Supports array indexing like "contexts[0].symbol.name"
func getJSONPath(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		// Check for array indexing syntax: field[index]
		if strings.Contains(part, "[") && strings.HasSuffix(part, "]") {
			// Split into field name and index
			openBracket := strings.Index(part, "[")
			fieldName := part[:openBracket]
			indexStr := part[openBracket+1 : len(part)-1]

			// Get the field first
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[fieldName]
			default:
				return nil
			}

			// Now handle the array index
			if current == nil {
				return nil
			}

			switch arr := current.(type) {
			case []interface{}:
				index := 0
				_, _ = fmt.Sscanf(indexStr, "%d", &index)
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return nil
				}
			default:
				return nil
			}
		} else {
			// Regular field access
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[part]
			default:
				return nil
			}
		}

		if current == nil {
			return nil
		}
	}

	return current
}

// CodeInsightOptions configures a code_insight operation
type CodeInsightOptions struct {
	Mode     string   // "overview", "detailed", "statistics", "unified", "structure", "git_analyze", "git_hotspots"
	Analysis string   // For detailed mode: "modules", "dependencies", "hotspots", "entry_points"
	Focus    string   // Optional focus area
	Tier     int      // Analysis tier (1-3)
	Metrics  []string // For statistics mode: ["complexity", "health", "coverage"]
}

// SideEffectsOptions configures a side_effects operation
type SideEffectsOptions struct {
	Mode   string // "summary", "symbol", "file", "pure", "impure", "category"
	Symbol string // For symbol mode
	File   string // For file mode
}

// CodeInsightResult holds parsed code_insight response
type CodeInsightResult struct {
	Raw          string                 // Raw LCF output
	Parsed       map[string]interface{} // Parsed JSON (if available)
	Mode         string
	HealthScore  float64
	HealthGrade  string
	TotalFiles   int
	TotalSymbols int
	Modules      []string
}

// SideEffectsResult holds parsed side_effects response
type SideEffectsResult struct {
	Raw             string                 // Raw output
	Parsed          map[string]interface{} // Parsed JSON (if available)
	Mode            string
	TotalFunctions  int
	PureFunctions   int
	ImpureFunctions int
	PurityRatio     float64
}

// CodeInsight executes a code_insight analysis and returns structured results
func (ctx *WorkflowTestContext) CodeInsight(name string, opts CodeInsightOptions) *CodeInsightResult {
	ctx.T.Helper()

	// Set defaults
	if opts.Mode == "" {
		opts.Mode = "overview"
	}

	params := map[string]interface{}{
		"mode": opts.Mode,
	}

	if opts.Analysis != "" {
		params["analysis"] = opts.Analysis
	}
	if opts.Focus != "" {
		params["focus"] = opts.Focus
	}
	if opts.Tier > 0 {
		params["tier"] = opts.Tier
	}
	if len(opts.Metrics) > 0 {
		params["metrics"] = opts.Metrics
	}

	ctx.T.Logf("Executing code_insight '%s': mode=%s", name, opts.Mode)

	insightStart := time.Now()
	resultStr, err := ctx.Server.CallTool("code_insight", params)
	insightElapsed := time.Since(insightStart)
	require.NoError(ctx.T, err, "code_insight '%s' failed: %v", name, err)

	ctx.T.Logf("code_insight '%s' completed in %v, output size: %d bytes", name, insightElapsed, len(resultStr))

	// Parse the result
	result := &CodeInsightResult{
		Raw:  resultStr,
		Mode: opts.Mode,
	}

	// Try to extract key metrics from LCF output
	result.parseMetrics()

	return result
}

// parseMetrics extracts key metrics from LCF output
func (r *CodeInsightResult) parseMetrics() {
	lines := strings.Split(r.Raw, "\n")
	totalFilesFromModules := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse health score: "score=8.5"
		if strings.HasPrefix(line, "score=") {
			fmt.Sscanf(line, "score=%f", &r.HealthScore)
		}
		// Parse health grade: "grade=A"
		if strings.HasPrefix(line, "grade=") {
			r.HealthGrade = strings.TrimPrefix(line, "grade=")
		}
		// Parse from combined line: "score=8.5 grade=A"
		if strings.Contains(line, "score=") && strings.Contains(line, "grade=") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "score=") {
					fmt.Sscanf(part, "score=%f", &r.HealthScore)
				}
				if strings.HasPrefix(part, "grade=") {
					r.HealthGrade = strings.TrimPrefix(part, "grade=")
				}
			}
		}
		// Parse total files from "total_files=123" or standalone "files=123"
		if strings.HasPrefix(line, "total_files=") {
			fmt.Sscanf(line, "total_files=%d", &r.TotalFiles)
		} else if strings.HasPrefix(line, "files=") && !strings.Contains(line, "module=") {
			// Standalone files= line (not a module line)
			fmt.Sscanf(line, "files=%d", &r.TotalFiles)
		}
		// Accumulate files from module lines: "module=... files=1"
		if strings.HasPrefix(line, "module=") && strings.Contains(line, "files=") {
			var fileCount int
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "files=") {
					fmt.Sscanf(part, "files=%d", &fileCount)
					totalFilesFromModules += fileCount
				}
			}
		}
		// Parse total symbols: "symbols=456" or "total_symbols=456"
		if strings.HasPrefix(line, "total_symbols=") {
			fmt.Sscanf(line, "total_symbols=%d", &r.TotalSymbols)
		} else if strings.HasPrefix(line, "symbols=") {
			fmt.Sscanf(line, "symbols=%d", &r.TotalSymbols)
		}
		// Parse module total: "total=17"
		if strings.HasPrefix(line, "total=") && r.TotalFiles == 0 {
			// This might be module count, not file count - only use if no other total found
			var total int
			fmt.Sscanf(line, "total=%d", &total)
			// Store as modules count could be used later
		}
	}

	// If we didn't find an explicit total, use accumulated from modules
	if r.TotalFiles == 0 && totalFilesFromModules > 0 {
		r.TotalFiles = totalFilesFromModules
	}
}

// Contains checks if the raw output contains a substring
func (r *CodeInsightResult) Contains(substr string) bool {
	return strings.Contains(r.Raw, substr)
}

// ContainsAny checks if the raw output contains any of the substrings
func (r *CodeInsightResult) ContainsAny(substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(r.Raw, substr) {
			return true
		}
	}
	return false
}

// SideEffects executes a side_effects analysis and returns structured results
func (ctx *WorkflowTestContext) SideEffects(name string, opts SideEffectsOptions) *SideEffectsResult {
	ctx.T.Helper()

	// Set defaults
	if opts.Mode == "" {
		opts.Mode = "summary"
	}

	params := map[string]interface{}{
		"mode": opts.Mode,
	}

	if opts.Symbol != "" {
		params["symbol"] = opts.Symbol
	}
	if opts.File != "" {
		params["file"] = opts.File
	}

	ctx.T.Logf("Executing side_effects '%s': mode=%s", name, opts.Mode)

	sideEffectsStart := time.Now()
	resultStr, err := ctx.Server.CallTool("side_effects", params)
	sideEffectsElapsed := time.Since(sideEffectsStart)
	require.NoError(ctx.T, err, "side_effects '%s' failed: %v", name, err)

	ctx.T.Logf("side_effects '%s' completed in %v, output size: %d bytes", name, sideEffectsElapsed, len(resultStr))

	// Parse the result
	result := &SideEffectsResult{
		Raw:  resultStr,
		Mode: opts.Mode,
	}

	// Try to parse JSON response
	result.parseMetrics()

	return result
}

// parseMetrics extracts key metrics from side_effects response
func (r *SideEffectsResult) parseMetrics() {
	// Try to parse as JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(r.Raw), &parsed); err == nil {
		r.Parsed = parsed

		// Extract summary metrics from nested summary object
		// Response format: {"results":...,"total_count":N,"mode":"summary","summary":{...}}
		if r.Mode == "summary" {
			if summary, ok := parsed["summary"].(map[string]interface{}); ok {
				if total, ok := summary["total_functions"].(float64); ok {
					r.TotalFunctions = int(total)
				}
				if pure, ok := summary["pure_functions"].(float64); ok {
					r.PureFunctions = int(pure)
				}
				if impure, ok := summary["impure_functions"].(float64); ok {
					r.ImpureFunctions = int(impure)
				}
				if ratio, ok := summary["purity_ratio"].(float64); ok {
					r.PurityRatio = ratio
				}
			}
		}
	}
}

// Contains checks if the raw output contains a substring
func (r *SideEffectsResult) Contains(substr string) bool {
	return strings.Contains(r.Raw, substr)
}

// Cleanup cleans up test resources
func (ctx *WorkflowTestContext) Cleanup() {
	if ctx.Indexer != nil {
		ctx.Indexer.Close()
	}
}
