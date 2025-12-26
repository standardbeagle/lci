package mcp

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	mcputils "github.com/standardbeagle/lci/internal/mcp/utils"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/internal/version"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// shouldInclude checks if a specific item is in the comma-separated Include field
// Uses zero-copy iteration instead of strings.Split
func shouldInclude(args SearchParams, item string) bool {
	if args.Include == "" {
		return false
	}
	remaining := args.Include
	for len(remaining) > 0 {
		var part string
		if idx := strings.IndexByte(remaining, ','); idx >= 0 {
			part = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			part = remaining
			remaining = ""
		}
		if strings.TrimSpace(part) == item {
			return true
		}
	}
	return false
}

// parseListHelper parses a comma-separated string into a slice
// Uses zero-copy iteration instead of strings.Split
func parseListHelper(s string) []string {
	if s == "" {
		return nil
	}
	// Count commas to pre-allocate (avoids repeated slice growth)
	count := strings.Count(s, ",") + 1
	result := make([]string, 0, count)
	remaining := s
	for len(remaining) > 0 {
		var part string
		if idx := strings.IndexByte(remaining, ','); idx >= 0 {
			part = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			part = remaining
			remaining = ""
		}
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// containsSpaceOrSpecial checks if a pattern needs semantic expansion
// Simple patterns (no spaces, alphanumeric, dots, underscores) can skip expansion
func containsSpaceOrSpecial(pattern string) bool {
	// Check for spaces or special characters
	return strings.Contains(pattern, " ") ||
		regexp.MustCompile(`[^a-zA-Z0-9._]`).MatchString(pattern)
}

// looksLikeRegex checks if a pattern contains regex-like syntax that suggests
// the user intended to use regex matching.
// This is used for fallback detection - we also search with regex mode at a lower score
// to catch cases where the user intended regex but didn't specify the flag.
func looksLikeRegex(pattern string) bool {
	// Empty patterns are not regex
	if len(pattern) == 0 {
		return false
	}

	// Check for pipe (OR) - most common case for multi-pattern search
	// e.g., "foo|bar", "NewRouter|NewMux"
	if strings.Contains(pattern, "|") {
		return true
	}

	// Check for character classes: [abc], [a-z], [^abc]
	// e.g., "[A-Z]Handler", "log[0-9]+"
	if strings.Contains(pattern, "[") && strings.Contains(pattern, "]") {
		return true
	}

	// Check for anchors: ^ at start or $ at end
	// e.g., "^func", "Handler$"
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		return true
	}

	// Check for common regex quantifiers with escaping awareness
	// Look for patterns like: \d+, \w*, \s?, .+, .*
	// But NOT: foo.bar (likely a qualified name), foo* (likely a glob)
	for i := 0; i < len(pattern)-1; i++ {
		ch := pattern[i]
		next := pattern[i+1]

		// Backslash escapes indicate regex: \d, \w, \s, \b, \., etc.
		// Including \. because escaping a dot is a clear regex signal
		if ch == '\\' && (next == 'd' || next == 'w' || next == 's' || next == 'b' ||
			next == 'D' || next == 'W' || next == 'S' || next == 'B' ||
			next == '.' || next == '*' || next == '+' || next == '?' ||
			next == '(' || next == ')' || next == '[' || next == ']' ||
			next == '{' || next == '}' || next == '^' || next == '$' ||
			next == '|' || next == '\\') {
			return true
		}

		// Quantifiers after certain characters suggest regex
		// e.g., ".+" or ".*" but not "foo.bar"
		if (next == '+' || next == '*' || next == '?') && ch == '.' {
			return true
		}

		// Regex-specific parenthesis patterns: (?:...), (?=...), (?!...), (?<...)
		// But NOT: func(), main(), fmt.Println() - those are function calls
		if ch == '(' && next == '?' {
			return true
		}
	}

	// Check for parentheses containing regex OR: (foo|bar)
	// This is different from function calls like func(a, b)
	parenDepth := 0
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '(' {
			parenDepth++
		} else if pattern[i] == ')' {
			parenDepth--
		} else if pattern[i] == '|' && parenDepth > 0 {
			// Pipe inside parentheses strongly suggests regex grouping
			return true
		}
	}

	// Check for curly brace quantifiers: {2}, {2,5}, {2,}
	// e.g., "a{2,3}", "Handler{1,}"
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		// Verify it looks like a quantifier, not a template literal like "${var}"
		for i := 0; i < len(pattern)-2; i++ {
			if pattern[i] == '{' {
				// Check if followed by digits
				j := i + 1
				for j < len(pattern) && (pattern[j] >= '0' && pattern[j] <= '9') {
					j++
				}
				// Valid if we found digits and then , or }
				if j > i+1 && j < len(pattern) && (pattern[j] == ',' || pattern[j] == '}') {
					return true
				}
			}
		}
	}

	return false
}

// RegexFallbackScoreMultiplier is applied to scores from regex fallback searches
// to ensure literal matches rank higher than regex matches
const RegexFallbackScoreMultiplier = 0.5

// ResultKey is a zero-allocation key type for deduplicating search results
// Replaces fmt.Sprintf("%d:%d:%s") in multi-pattern search (10-20MB savings per query)
type ResultKey struct {
	FileID types.FileID
	Line   int
	Match  string
}

// handleInfo provides basic help and usage information for tools
// @lci:labels[mcp-tool-handler,info,help]
// @lci:category[mcp-api]
func (s *Server) handleInfo(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Manual deserialization to avoid "unknown field" errors
	var toolParam InfoParams
	if err := json.Unmarshal(req.Params.Arguments, &toolParam); err != nil {
		return createSmartErrorResponse("info", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"help": "Use: {\"tool\": \"search\"} or {\"tool\": \"get_context\"} or {\"tool\": \"version\"}",
		})
	}

	tool := strings.ToLower(strings.TrimSpace(toolParam.Tool))

	switch tool {
	case "version":
		return createJSONResponse(map[string]interface{}{
			"name":           "version",
			"description":    "Get server version, build info, and performance capabilities",
			"server_name":    "lightning-code-index-mcp",
			"server_version": version.FullInfo(),
			"mcp_version":    "2025-06-18",
			"go_version":     runtime.Version(),
			"platform":       runtime.GOOS + "/" + runtime.GOARCH,
			"capabilities": []string{
				"stdio_transport",
				"semantic_search",
				"regex_search",
				"symbol_analysis",
				"semantic_filtering",
				"call_hierarchy",
				"reverse_refactoring",
				"tree_sitter_parsing",
				"ai_optimized_search",
				"multi_language_support",
				"semantic_annotations",
				"graph_propagation",
				"architectural_analysis",
				"side_effect_analysis",
			},
		})

	case "search":
		return createJSONResponse(map[string]interface{}{
			"name":        "search",
			"description": "Sub-millisecond semantic code search with 6-layer matching. Built-in fuzzy, stemming, camelCase splitting.",
			"syntax_note": "Uses JSON parameters (not CLI flags). No -n, -i, -v flags. See examples below.",
			"what_it_does": []string{
				"Semantic matching by default (6 layers: exact, fuzzy, stem, abbreviation, name-split, substring)",
				"Handles typos automatically (getUserNme -> getUserName)",
				"Expands abbreviations (auth -> authentication, authorize)",
				"Splits camelCase (user matches getUserName)",
				"Returns in <5ms with full context",
			},
			"parameters": map[string]string{
				"pattern":      "REQUIRED: Search pattern (string)",
				"max":          "Max results (default: 50, hard cap: 100)",
				"output":       "Output format: 'line', 'ctx', 'ctx:N', 'full', 'files', 'count'",
				"filter":       "File filter: 'go,*.py' (types/patterns), '*.go' (glob)",
				"flags":        "Search flags: 'ci' (case-insensitive), 'rx' (regex), 'iv' (invert), 'wb' (word-boundary), 'nt' (no-tests), 'nc' (no-comments)",
				"symbol_types": "Symbol filter: 'func,class,var,type,method' - supports 23 types plus aliases",
				"patterns":     "Multiple patterns (OR logic): 'user|login|auth'",
				"max_per_file": "Max matches per file (like grep -m)",
				"languages":    "Filter by programming languages: ['go'], ['typescript', 'javascript'], ['csharp']. Case-insensitive with aliases (ts, js, cs, py, rb, etc.)",
			},
			"limits": map[string]interface{}{
				"max_results":     100,
				"default_results": 50,
				"note":            "Results capped at 100 per request. Use specific patterns or filters for large codebases.",
			},
			"output_modes": map[string]string{
				"line":  "Minimal single-line output",
				"ctx":   "Include context lines (default: 2)",
				"ctx:N": "Include N context lines",
				"full":  "Complete metadata and relationships",
				"files": "Just file paths (files_with_matches)",
				"count": "Match counts per file",
			},
			"flags_reference": map[string]string{
				"ci": "Case-insensitive matching",
				"rx": "Treat pattern as regex",
				"iv": "Invert match (exclude matches)",
				"wb": "Word boundary matching",
				"nt": "Exclude test files",
				"nc": "Exclude comments",
			},
			"example": map[string]interface{}{
				"basic":           `{"pattern": "user"}`,
				"with_flags":      `{"pattern": "user", "flags": "ci,nt"}`,
				"with_output":     `{"pattern": "TODO", "output": "ctx:3"}`,
				"with_filter":     `{"pattern": "login", "filter": "*.go", "symbol_types": "func"}`,
				"multi_pattern":   `{"patterns": "auth|login|session"}`,
				"grep_style":      `{"pattern": "TODO", "output": "count", "max_per_file": 5}`,
				"language_filter": `{"pattern": "interface", "languages": ["go"]}`,
				"multi_language":  `{"pattern": "export", "languages": ["typescript", "javascript"]}`,
			},
			"use_instead_of": []string{"grep", "ripgrep", "find"},
			"performance":    "<5ms typical, sub-millisecond for cache hits",
		})

	case "get_context":
		return createJSONResponse(map[string]interface{}{
			"name":        "get_context",
			"description": "Deep context retrieval for search results. Get complete symbol info, references, and relationships.",
			"important":   "Use 'id' parameter with the object ID (o=XX) from search results. Example: {\"id\": \"VE\"}",
			"what_it_does": []string{
				"Retrieves full context for object IDs from search",
				"Shows complete symbol definition with signature",
				"Lists all incoming/outgoing references",
				"Maps call hierarchy and dependencies",
				"Includes file context and quality metrics",
			},
			"parameters": map[string]string{
				"id":                     "REQUIRED: Object ID(s) from search (e.g., 'VE' or 'VE,tG,Ab' for multiple)",
				"mode":                   "Lookup mode: 'full', 'quick', 'relationships' (default: 'full')",
				"include_call_hierarchy": "Include call graph data (default: true)",
				"include_all_references": "Include all reference locations (default: true)",
				"include_dependencies":   "Include dependency analysis (default: true)",
				"max_depth":              "Max depth for relationship analysis (default: 5)",
				"exclude_test_files":     "Exclude test files from analysis",
			},
			"modes": map[string]string{
				"full":          "Complete context with all relationships (default)",
				"quick":         "Basic definition and references only",
				"relationships": "Focus on call graph and dependencies",
			},
			"examples": map[string]interface{}{
				"single_id":      `{"id": "VE"}`,
				"multiple_ids":   `{"id": "VE,tG,Ab"}`,
				"with_hierarchy": `{"id": "VE", "include_call_hierarchy": true, "max_depth": 3}`,
				"quick_lookup":   `{"id": "VE", "mode": "quick"}`,
				"exclude_tests":  `{"id": "VE", "exclude_test_files": true}`,
			},
			"workflow": []string{
				"1. Run search: {\"pattern\": \"myFunction\"}",
				"2. Find object ID in results (look for o=XX, e.g., o=VE)",
				"3. Run get_context: {\"id\": \"VE\"}",
			},
			"common_mistakes": []string{
				"WRONG: {\"symbol_id\": \"VE\"} - use 'id' not 'symbol_id'",
				"WRONG: {\"result_id\": \"result_3_42\"} - use 'id' with object ID directly",
				"WRONG: {\"line\": 143} - line numbers don't work, use object ID",
			},
			"performance": "<2ms typical for single object",
		})

	case "semantic_annotations":
		return createJSONResponse(map[string]interface{}{
			"name":        "semantic_annotations",
			"description": "Query code by semantic labels and categories. Find critical code, architectural concerns, and tagged symbols.",
			"what_it_does": []string{
				"Search by @lci: annotations in comments",
				"Find code by labels (e.g., 'critical-bug', 'performance')",
				"Query by categories (e.g., 'security', 'architecture')",
				"Track label propagation through call graphs",
				"Identify annotated architectural boundaries",
			},
			"parameters": map[string]string{
				"label":              "Label to search for (e.g., 'critical-bug', 'performance')",
				"category":           "Category to filter by (e.g., 'security', 'architecture')",
				"min_strength":       "Minimum propagation strength (0.0-1.0, default: 0)",
				"include_direct":     "Include directly annotated symbols (default: true)",
				"include_propagated": "Include propagated labels from call graph (default: true)",
				"max_results":        "Maximum results to return (default: 100)",
			},
			"annotation_syntax": "@lci:labels[label1,label2] or @lci:category[security]",
			"example": map[string]interface{}{
				"find_by_label":    `{"label": "critical-bug"}`,
				"find_by_category": `{"category": "security"}`,
				"with_strength":    `{"label": "performance", "min_strength": 0.8}`,
				"both_types":       `{"label": "auth", "include_direct": true, "include_propagated": true}`,
				"limited":          `{"category": "security", "max_results": 20}`,
			},
			"use_cases": []string{
				"Find all security-critical code",
				"Track performance hotspots",
				"Identify architectural boundaries",
				"Locate code needing review",
			},
			"performance": "<10ms for label queries",
		})

	case "side_effects":
		return createJSONResponse(map[string]interface{}{
			"name":        "side_effects",
			"description": "Analyze function purity and side effects. Detects writes to parameters, globals, closures, I/O operations, and exception handling patterns.",
			"what_it_does": []string{
				"Detect pure functions with no side effects",
				"Find functions that mutate parameters, globals, or closures",
				"Identify I/O, network, and database operations",
				"Analyze exception handling patterns (throw/panic, defer, try-finally)",
				"Propagate effects transitively through call graph",
			},
			"parameters": map[string]string{
				"mode":               "Query mode: 'symbol', 'file', 'pure', 'impure', 'category', 'summary' (default: 'summary')",
				"symbol_id":          "Symbol ID for symbol mode",
				"symbol_name":        "Symbol name for symbol mode",
				"file_path":          "File path for file mode",
				"file_id":            "File ID for file mode",
				"category":           "Side effect category for category mode",
				"include_reasons":    "Include reasons for impurity (default: false)",
				"include_transitive": "Include transitive effects from callees (default: false)",
				"include_confidence": "Include confidence levels (default: false)",
				"max_results":        "Maximum results to return (default: 100)",
			},
			"categories": map[string]string{
				"param_write":   "Mutates function parameters",
				"global_write":  "Writes to global/module state",
				"closure_write": "Writes to captured closure variables",
				"io":            "File I/O operations",
				"network":       "Network operations",
				"database":      "Database operations",
				"throw":         "Can throw/panic",
				"channel":       "Go channel operations",
				"external_call": "Calls unknown/external functions",
				"dynamic_call":  "Uses dynamic dispatch",
			},
			"example": map[string]interface{}{
				"summary":          `{"mode": "summary"}`,
				"pure_functions":   `{"mode": "pure", "max_results": 50}`,
				"impure_functions": `{"mode": "impure", "include_reasons": true}`,
				"by_category":      `{"mode": "category", "category": "io"}`,
				"single_symbol":    `{"mode": "symbol", "symbol_name": "processData", "include_transitive": true}`,
				"by_file":          `{"mode": "file", "file_path": "src/utils.go"}`,
			},
			"use_cases": []string{
				"Identify safe functions for parallelization",
				"Find functions with potential concurrency issues",
				"Locate I/O bottlenecks",
				"Validate referential transparency",
				"Audit exception handling patterns",
			},
			"confidence_levels": map[string]string{
				"proven": "Analysis is certain (known pure stdlib functions)",
				"high":   "High confidence from static analysis",
				"medium": "Some uncertainty due to dynamic features",
				"low":    "Significant uncertainty, external calls present",
			},
			"performance": "<5ms for queries, analysis during indexing",
		})

	case "codebase_intelligence", "code_insight":
		return createJSONResponse(map[string]interface{}{
			"name":        "code_insight",
			"description": "Comprehensive codebase intelligence for AI agents. Provides overview, detailed analysis, statistics, and structure exploration with 5-10k token budget.",
			"parameters": map[string]string{
				"mode":                 "REQUIRED: 'overview', 'detailed', 'statistics', 'unified', 'structure', 'git_analyze', or 'git_hotspots'",
				"tier":                 "Tier level: 1, 2, or 3 (default: 1)",
				"include":              "Object with flags: repository_map, dependency_graph, health_dashboard, entry_points",
				"analysis":             "For detailed mode: 'modules', 'layers', 'features', 'terms', 'relationships'",
				"metrics":              "For statistics mode: array like ['complexity', 'coupling', 'quality', 'change']",
				"granularity":          "Analysis granularity: 'module', 'layer', or 'function'",
				"max_results":          "Maximum results to return (scales token budget from 4k-12k)",
				"confidence_threshold": "Minimum confidence for results (0.0-1.0)",
				"domain":               "Domain filter for feature analysis",
				"query":                "Query string for unified mode",
				"focus":                "For structure mode: filter to specific area (e.g., 'api', 'test', 'config')",
				"target":               "Target file path, directory, or symbol name for analysis",
				"languages":            "Filter by languages: array like ['go'], ['typescript', 'javascript'], ['csharp']. Case-insensitive with aliases (ts, js, cs, py, etc.)",
				"git":                  "Git analysis parameters (object) for git modes - see examples below",
			},
			"modes": map[string]string{
				"overview":     "High-level overview (Tier 1): Repository Map, Dependency Graph, Health Dashboard, Entry Points",
				"detailed":     "Detailed analysis (Tier 2): Module detection, Layer classification, Feature location, Term clustering",
				"statistics":   "Code statistics (Tier 3): Complexity, Coupling, Cohesion, Quality metrics",
				"unified":      "Complete analysis (All tiers combined)",
				"structure":    "Codebase structure exploration: Directory tree, file categories, key symbols (best for initial exploration)",
				"git_analyze":  "Git change analysis: Detect duplicates and naming consistency in changes vs existing codebase",
				"git_hotspots": "Git hotspot analysis: Identify frequently changing code, collision risks, and ownership patterns",
			},
			"tiers": map[string]string{
				"tier_1": "Must-have: Function signatures, module boundaries, domain terms, entry points",
				"tier_2": "High-value: Module detection, layer classification, feature location, term clustering",
				"tier_3": "Specialized: Code statistics, metrics, quality analysis",
			},
			"include_flags": map[string]string{
				"repository_map":   "Include module boundaries, domain terms, file structure",
				"dependency_graph": "Include import/dependency relationships",
				"health_dashboard": "Include code health metrics and hotspots",
				"entry_points":     "Include main functions and entry points",
			},
			"example": map[string]interface{}{
				"structure_explore":  `{"mode": "structure"}`,
				"structure_focused":  `{"mode": "structure", "focus": "api"}`,
				"overview_basic":     `{"mode": "overview"}`,
				"overview_with_tier": `{"mode": "overview", "tier": 2}`,
				"overview_selective": `{"mode": "overview", "include": {"repository_map": true, "entry_points": true}}`,
				"overview_go_only":   `{"mode": "overview", "languages": ["go"]}`,
				"overview_frontend":  `{"mode": "overview", "languages": ["typescript", "javascript"]}`,
				"detailed_modules":   `{"mode": "detailed", "analysis": "modules"}`,
				"detailed_csharp":    `{"mode": "detailed", "analysis": "modules", "languages": ["csharp"]}`,
				"detailed_layers":    `{"mode": "detailed", "analysis": "layers", "granularity": "function"}`,
				"statistics_all":     `{"mode": "statistics"}`,
				"statistics_metrics": `{"mode": "statistics", "metrics": ["complexity", "coupling"]}`,
				"unified_complete":   `{"mode": "unified", "tier": 3}`,
				"git_staged":         `{"mode": "git_analyze", "git": {"scope": "staged"}}`,
				"git_pr_review":      `{"mode": "git_analyze", "git": {"scope": "range", "base_ref": "main", "target_ref": "HEAD"}}`,
				"git_hotspots_30d":   `{"mode": "git_hotspots", "git": {"time_window": "30d"}}`,
				"git_collisions":     `{"mode": "git_hotspots", "git": {"focus": ["collisions"], "file_path": "internal/mcp/server.go"}}`,
			},
			"common_mistakes": []string{
				"WRONG: {\"mode\": \"detailed\", \"analysis\": \"complexity\"} - 'complexity' is not an analysis type",
				"RIGHT: {\"mode\": \"statistics\", \"metrics\": [\"complexity\"]} - use statistics mode with metrics array",
				"WRONG: {\"analysis\": \"modules\"} - missing required 'mode' parameter",
				"RIGHT: {\"mode\": \"detailed\", \"analysis\": \"modules\"} - analysis requires detailed mode",
			},
			"token_budget": map[string]string{
				"base":    "8000 tokens (increased for better exploration)",
				"min":     "4000 tokens",
				"max":     "12000 tokens",
				"scaling": "Scales with max_results parameter",
			},
			"performance": map[string]string{
				"structure":  "<1s for codebase structure exploration",
				"overview":   "<2s for medium codebase (Tier 1)",
				"detailed":   "<5s for comprehensive analysis (Tier 2)",
				"statistics": "<5s for full metrics (Tier 3)",
			},
		})

	case "codebase_report":
		return createJSONResponse(map[string]interface{}{
			"name":        "codebase_report",
			"status":      "DEPRECATED",
			"description": "⚠️ This tool is deprecated. Use 'codebase_intelligence' with mode='statistics' instead.",
			"replacement": map[string]interface{}{
				"tool":    "codebase_intelligence",
				"mode":    "statistics",
				"example": `{"mode": "statistics", "tier": 3}`,
			},
			"reason": "Functionality is now included in codebase_intelligence with better performance and more comprehensive metrics.",
		})

	case "find_files", "files":
		return createJSONResponse(map[string]interface{}{
			"name":        "find_files",
			"description": "Like 'find' or 'fd' - searches file paths, not content, on an in-memory index.",
			"what_it_does": []string{
				"Fuzzy matches file paths and names (UserController matches user_controller.ts)",
				"Supports glob patterns and file type filters",
				"Handles typos and abbreviations automatically",
				"Filters by directory, language, and hidden files",
				"Returns results ranked by match quality",
			},
			"parameters": map[string]string{
				"pattern":        "REQUIRED: File/path pattern to search for",
				"max":            "Maximum results (default: 50, max: 200)",
				"filter":         "Filter by file type ('go', 'python') or glob ('*.ts', 'src/**/*.js')",
				"flags":          "Search flags: 'ci' (case-insensitive), 'exact' (exact match only)",
				"include_hidden": "Include hidden files/directories (default: false)",
				"directory":      "Search within specific directory (relative to project root)",
			},
			"match_types": map[string]string{
				"exact":          "Exact full path match (score: 1.0)",
				"exact_filename": "Exact filename match (score: 0.95)",
				"substring":      "Pattern contained in path (score: 0.6-0.8)",
				"fuzzy":          "Fuzzy match on filename (score: 0.0-0.7)",
				"path_component": "Pattern matches directory/file name (score: 0.6)",
			},
			"examples": map[string]interface{}{
				"by_name":          `{"pattern": "UserController"}`,
				"fuzzy":            `{"pattern": "usrctrl"}`,
				"by_path":          `{"pattern": "src/components"}`,
				"with_filter":      `{"pattern": "handler", "filter": "*.go"}`,
				"in_directory":     `{"pattern": "test", "directory": "internal"}`,
				"case_insensitive": `{"pattern": "CONFIG", "flags": "ci"}`,
				"exact_only":       `{"pattern": "main.go", "flags": "exact"}`,
				"with_hidden":      `{"pattern": "config", "include_hidden": true}`,
			},
			"use_cases": []string{
				"Find configuration files: {\"pattern\": \"config\"}",
				"Locate test files: {\"pattern\": \"test\", \"filter\": \"*.go\"}",
				"Find components: {\"pattern\": \"Button\", \"directory\": \"src/components\"}",
				"Search specific dir: {\"pattern\": \"handler\", \"directory\": \"internal/api\"}",
			},
			"performance": "<5ms typical search time",
		})

	default:
		// Generic info about all tools
		return createJSONResponse(map[string]interface{}{
			"server":  "Lightning Code Index MCP",
			"tagline": "Sub-millisecond in-memory semantic code search",
			"available_tools": []string{
				"search - semantic code search",
				"files - file/path search with fuzzy matching",
				"get_context - detailed context for results",
				"semantic_annotations - find code by semantic tags",
				"code_insight - comprehensive codebase analysis (includes git analysis modes)",
				"info [tool] - help for specific tool (use 'info version' for server info)",
			},
			"quick_start": "Use 'search' tool with a pattern. Use 'info search' for details.",
			"why_use_lci": []string{
				"Faster than grep/rg (everything pre-indexed in memory)",
				"Smarter than find (understands code structure)",
				"Available everywhere (no IDE needed)",
				"Perfect for AI (MCP protocol, semantic output)",
			},
		})
	}
}

// Helper function for min

// Concurrent search adapter functions for MCP server

// mcpSearchWithOptionsAdapter adapts MasterIndex.SearchWithOptions to the concurrent search interface

// mcpSearchDetailedWithOptionsAdapter adapts MasterIndex.SearchDetailedWithOptions to the concurrent search interface
func (s *Server) mcpSearchDetailedWithOptionsAdapter(ctx context.Context, pattern string, options types.SearchOptions) ([]searchtypes.DetailedResult, error) {
	// Validate server state
	if s == nil {
		return nil, errors.New("server is nil")
	}

	if s.goroutineIndex == nil {
		return nil, errors.New("index not initialized")
	}

	// Call the indexer method directly
	return s.goroutineIndex.SearchDetailedWithOptions(pattern, options)
}

// mcpConcurrentSearch performs a search using the concurrent search coordination system

// mcpConcurrentDetailedSearch performs a detailed search
// NOTE: SearchCoordinator is designed for basic searches only (converts detailed→basic→detailed, losing data)
// For detailed searches with relational data and object IDs, we bypass the coordinator and call the adapter directly
func (s *Server) mcpConcurrentDetailedSearch(ctx context.Context, pattern string, options types.SearchOptions) ([]searchtypes.DetailedResult, error) {
	// Always use direct adapter to preserve relational data and object IDs
	s.diagnosticLogger.Printf("Using direct detailed search for pattern: %s (bypassing coordinator to preserve object IDs)", pattern)
	return s.mcpSearchDetailedWithOptionsAdapter(ctx, pattern, options)
}

// shouldRunAssemblySearch determines if assembly search should run

// isAssemblySearchCandidate checks if a pattern looks like a dynamically built string

// ========== Helper functions to reduce handleNewSearch complexity ==========

// languageToExtensions maps canonical language names to their file extensions
var languageToExtensions = map[string][]string{
	"Go":         {".go"},
	"JavaScript": {".js", ".jsx", ".mjs", ".cjs"},
	"TypeScript": {".ts", ".tsx", ".mts", ".cts"},
	"Python":     {".py", ".pyw", ".pyi"},
	"Java":       {".java"},
	"Rust":       {".rs"},
	"C++":        {".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".h++"},
	"C":          {".c", ".h"},
	"C#":         {".cs"},
	"PHP":        {".php", ".phtml"},
	"Ruby":       {".rb", ".rake", ".gemspec"},
	"Swift":      {".swift"},
	"Kotlin":     {".kt", ".kts"},
	"Scala":      {".scala", ".sc"},
	"Vue":        {".vue"},
	"Svelte":     {".svelte"},
	"Dart":       {".dart"},
	"Zig":        {".zig"},
	"Shell":      {".sh", ".bash", ".zsh"},
	"HTML":       {".html", ".htm"},
	"CSS":        {".css", ".scss", ".sass", ".less"},
	"SQL":        {".sql"},
	"Markdown":   {".md", ".markdown"},
	"JSON":       {".json"},
	"YAML":       {".yaml", ".yml"},
	"XML":        {".xml"},
	"Lua":        {".lua"},
	"R":          {".r", ".R"},
	"Perl":       {".pl", ".pm"},
	"Haskell":    {".hs", ".lhs"},
	"Elixir":     {".ex", ".exs"},
	"Erlang":     {".erl", ".hrl"},
	"Clojure":    {".clj", ".cljs", ".cljc"},
	"OCaml":      {".ml", ".mli"},
	"F#":         {".fs", ".fsi", ".fsx"},
}

// languagesToIncludePattern converts a list of language names to a regex pattern
// that matches file extensions for those languages.
// Example: ["go", "typescript"] -> `\.(go|ts|tsx|mts|cts)$`
func languagesToIncludePattern(languages []string) string {
	if len(languages) == 0 {
		return ""
	}

	var extensions []string
	for _, lang := range languages {
		// Normalize language name using the shared alias map
		normalized := normalizeLanguageName(lang)

		// Get extensions for this language
		if exts, ok := languageToExtensions[normalized]; ok {
			for _, ext := range exts {
				// Remove leading dot and add to list
				extensions = append(extensions, strings.TrimPrefix(ext, "."))
			}
		}
	}

	if len(extensions) == 0 {
		return ""
	}

	// Build regex pattern: \.(go|ts|tsx)$
	return `\.(` + strings.Join(extensions, "|") + `)$`
}

// validateServerAndIndex validates server state and index availability.
// checkIndexAvailability now includes timeout-based waiting for index completion.
func (s *Server) validateServerAndIndex() error {
	if s == nil {
		return errors.New("server is nil")
	}

	// checkIndexAvailability waits for index completion using channel-based signaling
	if available, err := s.checkIndexAvailability(); err != nil {
		return err
	} else if !available {
		return errors.New("search cannot proceed: index is not available")
	}

	return nil
}

// prepareSearchDefaults sets default values for search parameters (updated for consolidated format)
func prepareSearchDefaults(args SearchParams) (Output, int, int) {
	// Parse output from consolidated format
	outputSize := OutputSingleLine // default
	if args.Output == "full" {
		outputSize = OutputFull
	} else if args.Output == "ctx" || strings.HasPrefix(args.Output, "ctx:") {
		outputSize = OutputContext
	} else if args.Output == "files" || args.Output == "files_with_matches" || args.Output == "count" {
		outputSize = OutputSingleLine
	}

	maxResults := args.Max
	if maxResults == 0 {
		maxResults = 50 // Default for new search
	}
	// Hard cap to prevent output truncation - max 100 results per request
	// For more results, use pagination with offset parameter
	if maxResults > 100 {
		maxResults = 100
	}

	// Parse context lines from output format
	maxLineCount := 1 // default
	if outputSize == OutputContext {
		if args.Output == "ctx" {
			maxLineCount = 5
		} else if strings.HasPrefix(args.Output, "ctx:") {
			// Use strings.Cut for zero-copy parsing of "ctx:N" format
			if _, after, found := strings.Cut(args.Output, ":"); found {
				if count, err := strconv.Atoi(after); err == nil {
					maxLineCount = count
				}
			}
		}
	} else if outputSize == OutputFull {
		maxLineCount = 10
	}

	return outputSize, maxResults, maxLineCount
}

// validFlags maps valid flag names to their descriptions
var validFlags = map[string]string{
	"ci": "case-insensitive",
	"rx": "regex mode",
	"iv": "invert match",
	"wb": "word boundary",
	"nt": "exclude tests",
	"nc": "exclude comments",
}

// flagAliases maps common mistakes/alternative names to the correct flag
var flagAliases = map[string]string{
	"regex":            "rx",
	"regexp":           "rx",
	"re":               "rx",
	"i":                "ci",
	"case-insensitive": "ci",
	"caseinsensitive":  "ci",
	"ignore-case":      "ci",
	"ignorecase":       "ci",
	"invert":           "iv",
	"v":                "iv",
	"not":              "iv",
	"word":             "wb",
	"w":                "wb",
	"no-tests":         "nt",
	"notests":          "nt",
	"skip-tests":       "nt",
	"no-comments":      "nc",
	"nocomments":       "nc",
	"skip-comments":    "nc",
}

// validateAndNormalizeFlags validates flags and returns normalized flags plus warnings
// It auto-corrects common mistakes and returns warnings for unknown flags
// Uses zero-copy iteration instead of strings.Split
func validateAndNormalizeFlags(flags string) (string, []string) {
	if flags == "" {
		return "", nil
	}

	var normalized []string
	var warnings []string
	seen := make(map[string]bool)

	remaining := flags
	for len(remaining) > 0 {
		var f string
		if idx := strings.IndexByte(remaining, ','); idx >= 0 {
			f = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			f = remaining
			remaining = ""
		}
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}

		// Check if it's a valid flag
		if _, ok := validFlags[f]; ok {
			if !seen[f] {
				normalized = append(normalized, f)
				seen[f] = true
			}
			continue
		}

		// Check if it's an alias (common mistake)
		if corrected, ok := flagAliases[strings.ToLower(f)]; ok {
			if !seen[corrected] {
				normalized = append(normalized, corrected)
				seen[corrected] = true
			}
			warnings = append(warnings, fmt.Sprintf("flag '%s' auto-corrected to '%s' (%s)", f, corrected, validFlags[corrected]))
			continue
		}

		// Unknown flag - warn user
		validList := make([]string, 0, len(validFlags))
		for k, v := range validFlags {
			validList = append(validList, fmt.Sprintf("%s=%s", k, v))
		}
		warnings = append(warnings, fmt.Sprintf("unknown flag '%s' ignored. Valid flags: %s", f, strings.Join(validList, ", ")))
	}

	return strings.Join(normalized, ","), warnings
}

// buildSearchOptions constructs SearchOptions from parameters (updated for consolidated format)
func buildSearchOptions(args SearchParams, maxLineCount int) types.SearchOptions {
	// Helper to parse flags from comma-separated string (zero-copy iteration)
	parseFlag := func(flags, target string) bool {
		if flags == "" {
			return false
		}
		remaining := flags
		for len(remaining) > 0 {
			var f string
			if idx := strings.IndexByte(remaining, ','); idx >= 0 {
				f = remaining[:idx]
				remaining = remaining[idx+1:]
			} else {
				f = remaining
				remaining = ""
			}
			if strings.TrimSpace(f) == target {
				return true
			}
		}
		return false
	}

	// Convert languages to include pattern (if specified)
	includePattern := languagesToIncludePattern(args.Languages)

	return types.SearchOptions{
		CaseInsensitive: parseFlag(args.Flags, "ci"),
		UseRegex:        parseFlag(args.Flags, "rx"),
		MaxContextLines: maxLineCount,
		ExcludePattern:  args.Filter,    // Consolidated filter field
		IncludePattern:  includePattern, // From languages filter
		SymbolTypes:     parseListHelper(args.SymbolTypes),
		DeclarationOnly: false, // Not in new format
		UsageOnly:       false, // Not in new format
		ExportedOnly:    false, // Not in new format
		ExcludeTests:    parseFlag(args.Flags, "nt"),
		ExcludeComments: parseFlag(args.Flags, "nc"),
		// Grep-like features (P0 - Critical for LLM use cases)
		InvertMatch:     parseFlag(args.Flags, "iv"),
		Patterns:        parseListHelper(args.Patterns),
		CountPerFile:    args.Output == "count",
		FilesOnly:       args.Output == "files",
		WordBoundary:    parseFlag(args.Flags, "wb"),
		MaxCountPerFile: args.MaxPerFile,
		// Dense object IDs for context lookup
		IncludeObjectIDs: true, // Always enabled for MCP tools
	}
}

// extractSearchPatterns extracts patterns from arguments
func extractSearchPatterns(args SearchParams) ([]string, error) {
	var searchPatterns []string

	if args.Patterns != "" {
		// Multiple patterns: parse comma-separated string
		searchPatterns = parseListHelper(args.Patterns)
	} else if args.Pattern != "" {
		// Single pattern
		searchPatterns = []string{args.Pattern}
	} else {
		// No pattern provided
		return nil, errors.New("either pattern or patterns must be provided")
	}

	return searchPatterns, nil
}

// initializeSemanticSearch initializes the semantic search if needed
func (s *Server) initializeSemanticSearch() {
	if s.optimizedSemanticSearch == nil {
		if semanticIndex := s.goroutineIndex.GetSemanticSearchIndex(); semanticIndex != nil {
			s.optimizedSemanticSearch = search.NewOptimizedSemanticSearch(
				semanticIndex,
				s.semanticScorer,
				s.goroutineIndex.GetSymbolIndex(),
			)
		}
	}
}

// expandPatternWithSemantic expands a single pattern using semantic search
func (s *Server) expandPatternWithSemantic(pattern string, expanded *[]string) {
	// Add original pattern (for exact matches)
	*expanded = append(*expanded, pattern)

	// Use OptimizedSemanticSearch to find semantically matching symbols
	if s.optimizedSemanticSearch != nil {
		candidates := s.optimizedSemanticSearch.GatherCandidates(pattern)

		// DEBUG: Log candidates for multi-word patterns
		if len(strings.Fields(pattern)) > 1 {
			maxShow := 5
			if len(candidates) < maxShow {
				maxShow = len(candidates)
			}
			s.diagnosticLogger.Printf("DEBUG: GatherCandidates for '%s' found %d candidates: %v",
				pattern, len(candidates), candidates[:maxShow])
		}

		for _, candidate := range candidates {
			if candidate != pattern {
				*expanded = append(*expanded, candidate)
			}
		}
	} else {
		// Fallback: if semantic index not available, use exact word matching
		s.expandPatternWithWords(pattern, expanded)
	}

	// Add word-based fuzzy variations
	s.expandPatternWithWords(pattern, expanded)
}

// expandPatternWithWords expands pattern by splitting into words
func (s *Server) expandPatternWithWords(pattern string, expanded *[]string) {
	words := strings.Fields(pattern)
	for _, word := range words {
		if len(word) > 2 && word != pattern {
			*expanded = append(*expanded, word)
		}
	}
}

// performSemanticExpansion performs aggressive pattern expansion for search
// Strategy: Always split multi-word queries to maximize matches, then rank by quality
// The Semantic flag controls advanced features (stemming, synonyms), not basic word splitting
func (s *Server) performSemanticExpansion(searchPatterns []string, args SearchParams) []string {
	expandedPatterns := make([]string, 0, len(searchPatterns)*20)

	for _, pattern := range searchPatterns {
		// Always add the original pattern first (exact matches rank highest)
		expandedPatterns = append(expandedPatterns, pattern)

		// Check if pattern has multiple words - if so, always split
		hasMultipleWords := strings.Contains(strings.TrimSpace(pattern), " ")

		if hasMultipleWords {
			// ALWAYS split multi-word patterns into individual words
			// This ensures "libby clone code" also searches for "libby", "clone", "code"
			s.expandPatternWithWords(pattern, &expandedPatterns)
		}

		// Advanced semantic expansion (symbol matching, stemming, abbreviations)
		// Only if Semantic flag is enabled AND we have the semantic index
		if args.Semantic {
			s.initializeSemanticSearch()
			if s.optimizedSemanticSearch != nil {
				candidates := s.optimizedSemanticSearch.GatherCandidates(pattern)
				for _, candidate := range candidates {
					if candidate != pattern {
						expandedPatterns = append(expandedPatterns, candidate)
					}
				}
			}
		}
	}

	// Deduplicate while preserving order (first occurrence wins, maintains score priority)
	seen := make(map[string]struct{}, len(expandedPatterns))
	deduped := make([]string, 0, len(expandedPatterns))
	for _, p := range expandedPatterns {
		if _, exists := seen[p]; !exists {
			seen[p] = struct{}{}
			deduped = append(deduped, p)
		}
	}

	return deduped
}

// resultHeap implements a max-heap for search results, ordered by score.
// Using a heap provides O(log n) insertion while maintaining sorted order,
// avoiding the need for post-processing sorts and map iteration non-determinism.
type resultHeap struct {
	results []*searchtypes.DetailedResult
}

func (h *resultHeap) Len() int { return len(h.results) }

// Less defines max-heap ordering (higher scores first)
// For equal scores, longer match text wins (more specific)
// For equal match lengths, alphabetical path for determinism
func (h *resultHeap) Less(i, j int) bool {
	ri, rj := h.results[i].Result, h.results[j].Result
	if ri.Score != rj.Score {
		return ri.Score > rj.Score // Max-heap: higher score = higher priority
	}
	if len(ri.Match) != len(rj.Match) {
		return len(ri.Match) > len(rj.Match) // Longer match = more specific
	}
	return ri.Path < rj.Path // Alphabetical for determinism
}

func (h *resultHeap) Swap(i, j int) { h.results[i], h.results[j] = h.results[j], h.results[i] }

func (h *resultHeap) Push(x interface{}) {
	h.results = append(h.results, x.(*searchtypes.DetailedResult))
}

func (h *resultHeap) Pop() interface{} {
	old := h.results
	n := len(old)
	x := old[n-1]
	old[n-1] = nil // Avoid memory leak
	h.results = old[0 : n-1]
	return x
}

// Word coverage boost constants
const (
	// Boost score by this factor for each additional pattern match
	// e.g., result matching 3 patterns gets score * (1 + 2*0.15) = score * 1.30
	wordCoverageBoostPerWord = 0.15
	// Maximum total boost from word coverage (prevents runaway scores)
	maxWordCoverageBoost = 0.5
)

// resultWithCoverage tracks a result and how many patterns it matched
type resultWithCoverage struct {
	result       *searchtypes.DetailedResult
	patternCount int
}

// searchAndDeduplicate performs search and deduplicates results using a heap
// for efficient sorted insertion. Boosts scores for results matching multiple patterns.
// Uses map for O(1) deduplication and heap for O(log n) sorted insertion.
func (s *Server) searchAndDeduplicate(ctx context.Context, patterns []string, options types.SearchOptions) ([]searchtypes.DetailedResult, error) {
	// Track results with pattern coverage count
	// Key: file+line+match, Value: result with coverage info
	resultCoverage := make(map[ResultKey]*resultWithCoverage)

	for _, pattern := range patterns {
		patternResults, err := s.mcpConcurrentDetailedSearch(ctx, pattern, options)
		if err != nil {
			// Log error but continue with other patterns
			continue
		}

		// Add results, tracking pattern coverage for duplicates
		for i := range patternResults {
			key := ResultKey{
				FileID: patternResults[i].Result.FileID,
				Line:   patternResults[i].Result.Line,
				Match:  patternResults[i].Result.Match,
			}

			if existing, exists := resultCoverage[key]; exists {
				// Result already found by another pattern - increment coverage
				existing.patternCount++
				// Keep the higher base score between the two matches
				if patternResults[i].Result.Score > existing.result.Result.Score {
					existing.result = &patternResults[i]
				}
			} else {
				// New result - add with coverage count of 1
				resultCoverage[key] = &resultWithCoverage{
					result:       &patternResults[i],
					patternCount: 1,
				}
			}
		}
	}

	// Build heap with coverage-boosted scores
	h := &resultHeap{results: make([]*searchtypes.DetailedResult, 0, len(resultCoverage))}
	heap.Init(h)

	for _, rc := range resultCoverage {
		// Apply word coverage boost: results matching multiple patterns rank higher
		if rc.patternCount > 1 {
			boost := float64(rc.patternCount-1) * wordCoverageBoostPerWord
			if boost > maxWordCoverageBoost {
				boost = maxWordCoverageBoost
			}
			rc.result.Result.Score *= (1.0 + boost)
		}
		heap.Push(h, rc.result)
	}

	// Extract results in sorted order (heap provides deterministic ordering)
	detailedResults := make([]searchtypes.DetailedResult, 0, h.Len())
	for h.Len() > 0 {
		result := heap.Pop(h).(*searchtypes.DetailedResult)
		detailedResults = append(detailedResults, *result)
	}

	return detailedResults, nil
}

// mergeAndDeduplicateResults merges two result sets, keeping the higher-scored
// version when duplicates are found. Used for regex fallback to ensure literal
// matches (higher scores) take precedence over regex matches (lower scores).
func mergeAndDeduplicateResults(primary, secondary []searchtypes.DetailedResult) []searchtypes.DetailedResult {
	// Build a map of existing results from primary
	seen := make(map[ResultKey]int) // maps to index in primary
	for i, r := range primary {
		key := ResultKey{
			FileID: r.Result.FileID,
			Line:   r.Result.Line,
			Match:  r.Result.Match,
		}
		seen[key] = i
	}

	// Add secondary results that don't exist in primary
	// (if they do exist, primary already has the higher-scored version)
	for _, r := range secondary {
		key := ResultKey{
			FileID: r.Result.FileID,
			Line:   r.Result.Line,
			Match:  r.Result.Match,
		}
		if _, exists := seen[key]; !exists {
			primary = append(primary, r)
			seen[key] = len(primary) - 1
		}
	}

	return primary
}

// extractSymbolMetadata extracts symbol metadata from detailed result
func extractSymbolMetadata(detailed searchtypes.DetailedResult) (string, string, bool, string) {
	objectID := detailed.ObjectID
	var symbolType, symbolName string
	var isExported bool

	// Extract symbol metadata
	if detailed.RelationalData != nil && detailed.RelationalData.Symbol.ID > 0 {
		symbolType = detailed.RelationalData.Symbol.Type.String()
		symbolName = detailed.RelationalData.Symbol.Name

		// Check if symbol is exported (safe nil check)
		if len(symbolName) > 0 {
			isExported = symbolName[0] >= 'A' && symbolName[0] <= 'Z'
		}
	} else {
		symbolType = "unknown"
		symbolName = ""
		isExported = false
	}

	return symbolType, symbolName, isExported, objectID
}

// Token budget constants for search results
const (
	// Match field truncation limits (chars)
	matchLimitSingleLine = 100 // For list view - information dense summary
	matchLimitContext    = 300 // For context mode - slightly more detail
	matchLimitFull       = 500 // For full mode - still capped for token efficiency

	// Context size limits (bytes) - cap function bodies around search result
	maxContextBytesPerResult = 2048 // ~2KB max per result, even in full mode
	maxContextLinesPerResult = 30   // Max lines of context, even in full mode
)

// Score thresholds for tiered response detail
const (
	scoreThresholdFull   = 0.8 // Score >= 0.8: Full detail (strong match)
	scoreThresholdMedium = 0.5 // Score 0.5-0.8: Medium detail
	// Score < 0.5: Minimal detail (weak match)
)

// computeEffectiveOutputSize adjusts output detail based on match score
// Strong matches get full requested detail, weak matches get compressed
// This maximizes useful information in the response by allocating tokens to best matches
func computeEffectiveOutputSize(requestedSize Output, score float64) Output {
	// Normalize score to 0-1 range (scores can exceed 100 in some cases)
	normalizedScore := score
	if score > 1.0 {
		normalizedScore = score / 100.0
	}

	// Strong matches: use requested output size
	if normalizedScore >= scoreThresholdFull {
		return requestedSize
	}

	// Medium matches: downgrade one level
	if normalizedScore >= scoreThresholdMedium {
		switch requestedSize {
		case OutputFull:
			return OutputContext
		case OutputContext:
			return OutputSingleLine
		default:
			return OutputSingleLine
		}
	}

	// Weak matches: always minimal
	return OutputSingleLine
}

// buildCompactResult builds a compact search result from detailed result
// Uses score-proportional detail: strong matches get full context, weak matches get minimal
func buildCompactResult(detailed searchtypes.DetailedResult, outputSize Output, args SearchParams) CompactSearchResult {
	result := detailed.Result

	// Compute effective output size based on match score
	// This ensures strong matches get full detail while weak matches are compact
	effectiveSize := computeEffectiveOutputSize(outputSize, result.Score)

	// Generate unique IDs
	resultID := fmt.Sprintf("result_%d_%d", result.FileID, result.Line)

	// Extract symbol metadata
	symbolType, symbolName, isExported, objectID := extractSymbolMetadata(detailed)

	// Truncate match based on effective output mode for token efficiency
	match := truncateMatch(result.Match, effectiveSize)

	compactResult := CompactSearchResult{
		ResultID:       resultID,
		ObjectID:       objectID,
		File:           result.Path,
		Line:           result.Line,
		Column:         result.Column,
		Match:          match,
		Score:          result.Score,
		SymbolType:     symbolType,
		SymbolName:     symbolName,
		IsExported:     isExported,
		FileMatchCount: result.FileMatchCount, // Set by engine for CountPerFile mode
	}

	// Add context lines based on effective output size (score-proportional)
	// Strong matches get more context, weak matches get none
	if effectiveSize != OutputSingleLine && len(result.Context.Lines) > 0 {
		compactResult.ContextLines = truncateContextLines(result.Context.Lines, effectiveSize)
	}

	// Add optional context elements only for strong matches (score >= threshold)
	// Weak matches skip extras to save tokens for better results
	normalizedScore := result.Score
	if result.Score > 1.0 {
		normalizedScore = result.Score / 100.0
	}
	includeExtras := normalizedScore >= scoreThresholdMedium

	if includeExtras && shouldInclude(args, "breadcrumbs") && detailed.RelationalData != nil && len(detailed.RelationalData.Breadcrumbs) > 0 {
		compactResult.Breadcrumbs = convertToScopeBreadcrumbs(detailed.RelationalData.Breadcrumbs)
	}

	if includeExtras && shouldInclude(args, "refs") && detailed.RelationalData != nil {
		incoming := detailed.RelationalData.RefStats.Total.IncomingCount
		outgoing := detailed.RelationalData.RefStats.Total.OutgoingCount
		compactResult.References = &ReferenceInfo{
			IncomingCount: incoming,
			OutgoingCount: outgoing,
		}
	}

	return compactResult
}

// truncateMatch truncates the match text based on output mode for token efficiency
// Truncation happens at line boundaries (never mid-line) for clean output
func truncateMatch(match string, outputSize Output) string {
	var limit int
	switch outputSize {
	case OutputSingleLine:
		limit = matchLimitSingleLine
	case OutputContext:
		limit = matchLimitContext
	case OutputFull:
		limit = matchLimitFull
	default:
		limit = matchLimitSingleLine
	}

	if len(match) <= limit {
		return match
	}

	// For multi-line matches, truncate at line boundaries
	// Never cut a line in the middle (unless it's the only/first line)
	if strings.Contains(match, "\n") {
		lines := strings.Split(match, "\n")
		var result strings.Builder
		totalBytes := 0
		linesIncluded := 0

		for i, line := range lines {
			lineBytes := len(line)
			if i > 0 {
				lineBytes++ // Account for newline
			}

			if totalBytes+lineBytes > limit-3 { // Reserve space for "..."
				// If this is the first line and it exceeds limit, truncate at word boundary
				if i == 0 {
					truncated := truncateSingleLine(line, limit-3) // Leave room for "..."
					if len(truncated) < len(line) || len(lines) > 1 {
						return truncated + "..."
					}
					return truncated
				}
				// Otherwise stop at the previous line and add ellipsis
				if result.Len() > 0 {
					result.WriteString("...")
				}
				break
			}

			if i > 0 {
				result.WriteString("\n")
			}
			result.WriteString(line)
			totalBytes += lineBytes
			linesIncluded++
		}

		// If we didn't include all lines, indicate truncation
		if linesIncluded < len(lines) && !strings.HasSuffix(result.String(), "...") {
			result.WriteString("...")
		}

		return result.String()
	}

	// For single-line matches, truncate at word boundaries where possible
	return truncateSingleLine(match, limit)
}

// truncateSingleLine truncates a single line at word boundaries
func truncateSingleLine(line string, limit int) string {
	if len(line) <= limit {
		return line
	}

	truncated := line[:limit-3]
	// Try to break at last space for cleaner output
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > limit/2 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

// truncateContextLines truncates context lines for token efficiency
// Even in full mode, we cap the context to prevent verbose function bodies
// Truncation happens at line boundaries (never mid-line) for clean output
func truncateContextLines(lines []string, outputSize Output) []string {
	if len(lines) == 0 {
		return lines
	}

	// Determine max lines based on output mode
	maxLines := maxContextLinesPerResult
	if outputSize == OutputContext {
		maxLines = 10 // Fewer lines for context mode
	}

	// Truncate number of lines first
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	// Check total byte size and truncate at line boundaries
	// KB limit is at line resolution - we include complete lines only
	totalBytes := 0
	cutoffIndex := len(lines)
	for i, line := range lines {
		lineBytes := len(line) + 1 // +1 for newline
		if totalBytes+lineBytes > maxContextBytesPerResult {
			// Stop before this line to stay under budget
			cutoffIndex = i
			break
		}
		totalBytes += lineBytes
	}

	if cutoffIndex < len(lines) {
		lines = lines[:cutoffIndex]
	}

	return lines
}

// convertDetailedToCompact converts detailed results to compact results
func convertDetailedToCompact(detailedResults []searchtypes.DetailedResult, outputSize Output, args SearchParams, maxResults int) []CompactSearchResult {
	compactResults := make([]CompactSearchResult, 0, len(detailedResults))

	for _, detailed := range detailedResults {
		compactResult := buildCompactResult(detailed, outputSize, args)
		compactResults = append(compactResults, compactResult)

		// Stop if we've reached maxResults
		if len(compactResults) >= maxResults {
			break
		}
	}

	return compactResults
}

// extractUniqueFiles extracts unique file paths from search results
func extractUniqueFiles(results []searchtypes.DetailedResult, maxFiles int) []string {
	// Preallocate map with expected capacity (upper bound is min of results or maxFiles)
	expectedUnique := len(results)
	if maxFiles < expectedUnique {
		expectedUnique = maxFiles
	}
	seen := make(map[string]struct{}, expectedUnique)
	files := make([]string, 0, expectedUnique)

	for _, r := range results {
		path := r.Result.Path
		if _, exists := seen[path]; !exists {
			seen[path] = struct{}{}
			files = append(files, path)
			if len(files) >= maxFiles {
				break
			}
		}
	}

	return files
}

// countUniqueFiles counts unique files in search results
func countUniqueFiles(results []searchtypes.DetailedResult) int {
	// Preallocate map - upper bound is len(results), actual will likely be fewer
	seen := make(map[string]struct{}, len(results))
	for _, r := range results {
		seen[r.Result.Path] = struct{}{}
	}
	return len(seen)
}

// ========== End of helper functions ==========

// handleNewSearch performs flexible search with minimal-by-default approach
// Refactored to reduce cyclomatic complexity from 42 to ~10
// @lci:labels[mcp-tool-handler,search,semantic-search]
// @lci:category[mcp-api]
func (s *Server) handleNewSearch(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Manual deserialization to avoid "unknown field" errors and give better error messages
	var searchParams SearchParams
	if err := json.Unmarshal(req.Params.Arguments, &searchParams); err != nil {
		return createSmartErrorResponse("search", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"common_mistakes": []string{
				"Using CLI flags like -n, -i, -v in JSON",
				"Using -n flag (line numbers) - MCP search uses {\"pattern\": \"text\"} not {\"-n\": true}",
			},
			"correct_format": `{"pattern": "search_text", "case_insensitive": true}`,
			"info_command":   "Run info tool with {\"tool\": \"search\"} for examples",
		})
	}

	// Apply semantic default: true unless explicitly set to false
	// Check if "semantic" field was present in the request
	rawArgs := req.Params.Arguments
	semanticExplicitlySet := strings.Contains(string(rawArgs), `"semantic"`)
	if !semanticExplicitlySet {
		searchParams.Semantic = true // Default to enabled for aggressive matching
	}

	args := searchParams
	var warnings []string

	// Diagnostic: Check index state for debugging zero-result searches
	if args.Max > 0 {
		// Only show diagnostics for patterns that might commonly return no results
		if strings.Contains(args.Pattern, ".") || strings.Contains(args.Pattern, "*") {
			fileCount := s.goroutineIndex.GetFileCount()
			symbolCount := s.goroutineIndex.GetSymbolCount()
			s.diagnosticLogger.Printf("DIAGNOSTIC: Search pattern='%s' with %d indexed files and %d symbols. "+
				"If this returns 0 results: 1) Check pattern syntax, 2) Verify files contain the pattern, 3) Check index_stats for indexing status",
				args.Pattern, fileCount, symbolCount)
		}
	}

	// Step 1: Validate server and index
	if err := s.validateServerAndIndex(); err != nil {
		if strings.Contains(err.Error(), "index is not available") {
			return createSmartErrorResponse("search", err, map[string]interface{}{
				"troubleshooting": []string{
					"Verify you're in a project directory with source code",
					"Check file permissions in project directory",
					"Review .lci.kdl configuration for errors",
					"Wait for auto-indexing to complete (check index_stats)",
				},
			})
		}
		return createErrorResponse("search", err)
	}

	// Step 2: Validate parameters
	if err := validateSearchParams(args); err != nil {
		context := map[string]interface{}{
			"pattern":          args.Pattern,
			"validation_error": err.Error(),
		}
		return createSmartErrorResponse("search", fmt.Errorf("parameter validation failed: %w", err), context)
	}

	// Step 2.5: Resolve symbol types and collect warnings for fuzzy/prefix matches
	if args.SymbolTypes != "" {
		resolver := NewSymbolTypeResolver()
		resolvedTypes, typeWarnings := resolver.ResolveAll(args.SymbolTypes)
		if len(resolvedTypes) > 0 {
			args.SymbolTypes = strings.Join(resolvedTypes, ",")
		}
		warnings = append(warnings, typeWarnings...)
	}

	// Step 2.6: Validate and normalize flags (auto-correct common mistakes)
	if args.Flags != "" {
		normalizedFlags, flagWarnings := validateAndNormalizeFlags(args.Flags)
		args.Flags = normalizedFlags
		warnings = append(warnings, flagWarnings...)
	}

	// Step 3: Prepare defaults
	outputSize, maxResults, maxLineCount := prepareSearchDefaults(args)

	// Step 4: Build search options
	options := buildSearchOptions(args, maxLineCount)

	// Step 5: Extract search patterns
	searchPatterns, err := extractSearchPatterns(args)
	if err != nil {
		return createErrorResponse("search", err)
	}

	// Step 6: Perform semantic expansion
	expandedPatterns := s.performSemanticExpansion(searchPatterns, args)

	// Step 7: Search and deduplicate results
	detailedResults, err := s.searchAndDeduplicate(ctx, expandedPatterns, options)
	if err != nil {
		return createErrorResponse("search", err)
	}

	// Step 7.5: Regex fallback - if pattern looks like regex (contains |) and regex mode
	// wasn't explicitly enabled, also search with regex and add results at lower scores.
	// This ensures "foo|bar" finds both literal matches AND regex OR matches.
	if !options.UseRegex {
		for _, pattern := range searchPatterns {
			if looksLikeRegex(pattern) {
				// Try regex search as fallback
				regexOptions := options
				regexOptions.UseRegex = true
				regexResults, regexErr := s.searchAndDeduplicate(ctx, []string{pattern}, regexOptions)
				if regexErr == nil && len(regexResults) > 0 {
					// Apply score penalty to regex fallback results
					for i := range regexResults {
						regexResults[i].Result.Score *= RegexFallbackScoreMultiplier
					}
					// Merge with existing results (deduplication handled by searchAndDeduplicate)
					detailedResults = mergeAndDeduplicateResults(detailedResults, regexResults)
				}
			}
		}
	}

	// Step 8: Handle special output modes (files, count) with minimal responses
	// Note: "files_with_matches" is the legacy ripgrep-compatible name for "files"
	if args.Output == "files" || args.Output == "files_with_matches" {
		// Files-only mode: return just unique file paths
		uniqueFiles := extractUniqueFiles(detailedResults, maxResults)
		filesResponse := &FilesOnlyResponse{
			Files:        uniqueFiles,
			TotalMatches: len(detailedResults),
			UniqueFiles:  len(uniqueFiles),
		}
		if len(warnings) > 0 {
			return createResponseWithWarnings(filesResponse, warnings)
		}
		return createCompactResponse(filesResponse, false, false)
	}

	if args.Output == "count" {
		// Count-only mode: return just the count - absolute minimal output
		uniqueFiles := countUniqueFiles(detailedResults)
		countResponse := &CountOnlyResponse{
			TotalMatches: len(detailedResults),
			UniqueFiles:  uniqueFiles,
		}
		if len(warnings) > 0 {
			return createResponseWithWarnings(countResponse, warnings)
		}
		return createCompactResponse(countResponse, false, false)
	}

	// Step 9: Convert to compact results (for normal output modes)
	compactResults := convertDetailedToCompact(detailedResults, outputSize, args, maxResults)

	// Step 10: Create response with pagination info
	response := SearchResponse{
		Results:      compactResults,
		TotalMatches: len(detailedResults),
		Showing:      len(compactResults),
		MaxResults:   maxResults,
	}

	// Add warnings if any extra parameters were detected
	if len(warnings) > 0 {
		return createResponseWithWarnings(response, warnings)
	}

	// Use compact format by default (no backward compatibility)
	includeContext := outputSize != OutputSingleLine
	includeMetadata := shouldInclude(args, "breadcrumbs") || shouldInclude(args, "safety") ||
		shouldInclude(args, "refs") || shouldInclude(args, "deps")
	return createCompactResponse(response, includeContext, includeMetadata)
}

// Helper functions for the new search handler

// convertToScopeBreadcrumbs converts relational data breadcrumbs to scope breadcrumbs
func convertToScopeBreadcrumbs(breadcrumbs []types.ScopeInfo) []ScopeBreadcrumb {
	result := make([]ScopeBreadcrumb, len(breadcrumbs))
	for i, crumb := range breadcrumbs {
		result[i] = ScopeBreadcrumb{
			ScopeType:  crumb.Type.String(),
			Name:       crumb.Name,
			StartLine:  crumb.StartLine,
			EndLine:    crumb.EndLine,
			Language:   crumb.Language,
			Visibility: inferVisibility(crumb),
		}
	}
	return result
}

// inferVisibility determines visibility from scope attributes and naming conventions
func inferVisibility(scope types.ScopeInfo) string {
	// Check attributes for explicit visibility markers
	for _, attr := range scope.Attributes {
		if attr.Type == types.AttrTypeExported {
			return "public"
		}
	}

	// Use naming conventions (uppercase first letter typically means public in many languages)
	if scope.Name != "" && len(scope.Name) > 0 {
		firstChar := rune(scope.Name[0])
		if firstChar >= 'A' && firstChar <= 'Z' {
			return "public"
		}
	}

	// Default to public for top-level scopes, private otherwise
	if scope.Level == 0 {
		return "public"
	}
	return "private"
}

// getFirstSafetyReason returns the first safety warning or empty string

// handleGetObjectContext provides detailed context for specific objects from the index
// Supports both simple object context and comprehensive context lookup with modes
// @lci:labels[mcp-tool-handler,context,symbol-analysis]
// @lci:category[mcp-api]
func (s *Server) handleGetObjectContext(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Manual deserialization to avoid "unknown field" errors and give better error messages
	var contextParams ObjectContextParams
	if err := json.Unmarshal(req.Params.Arguments, &contextParams); err != nil {
		return createSmartErrorResponse("get_context", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"common_mistakes": []string{
				"Using line numbers like {\"line\": 143} - WRONG!",
				"Using 'symbol_id' instead of 'id' - WRONG!",
				"Forgetting to run search first to get object ID",
			},
			"correct_format": `{"id": "VE"}  (where VE is the object ID from search results, shown as o=VE)`,
			"workflow": []string{
				"1. Run search: {\"pattern\": \"myFunction\"}",
				"2. Find object ID in results (look for o=XX)",
				"3. Run get_context: {\"id\": \"XX\"}",
			},
			"info_command": "Run info tool with {\"tool\": \"get_context\"} for examples",
		})
	}

	args := contextParams

	// Validate server state
	if s == nil {
		return createErrorResponse("get_context", errors.New("server is nil"))
	}

	// Check if index is available with helpful error messages
	if available, err := s.checkIndexAvailability(); err != nil {
		return createSmartErrorResponse("get_context", err, map[string]interface{}{
			"troubleshooting": []string{
				"Verify you're in a project directory with source code",
				"Check file permissions in project directory",
				"Review .lci.kdl configuration for errors",
				"Wait for auto-indexing to complete (check index_stats)",
			},
		})
	} else if !available {
		return createErrorResponse("get_context", errors.New("object context lookup cannot proceed: index is not available"))
	}

	// Validate mutually exclusive parameter groups
	if err := s.validateGetContextParams(&args); err != nil {
		return createSmartErrorResponse("get_context", fmt.Errorf("parameter validation failed: %w", err), map[string]interface{}{
			"provided_id": args.ID,
			"example":     `{"id": "VE"} or {"id": "VE,tG,Ab"} for multiple`,
			"help":        "Use the object ID (o=XX) from search results",
		})
	}

	// If mode is specified, use context_lookup logic (migrated from context_lookup_tools.go)
	if args.Mode != "" {
		return s.handleGetObjectContextWithMode(ctx, &args)
	}

	// Otherwise use original get_object_context logic
	// Validate parameters
	if err := validateObjectContextParams(args); err != nil {
		return createSmartErrorResponse("get_context", fmt.Errorf("parameter validation failed: %w", err), map[string]interface{}{
			"provided_id": args.ID,
			"example":     `{"id": "VE"} or {"id": "VE,tG,Ab"} for multiple`,
		})
	}

	// Set defaults for context options
	if !args.IncludeFullSymbol && !args.IncludeCallHierarchy && !args.IncludeAllReferences &&
		!args.IncludeDependencies && !args.IncludeFileContext && !args.IncludeQualityMetrics {
		// If no specific options provided, include everything by default
		args.IncludeFullSymbol = true
		args.IncludeCallHierarchy = true
		args.IncludeAllReferences = true
		args.IncludeDependencies = true
		args.IncludeFileContext = true
		args.IncludeQualityMetrics = true
	}

	// Resolve object IDs from 'id' parameter (zero-copy iteration)
	var objectIDs []string
	if args.ID != "" {
		// Comma-separated concise object IDs (e.g., "VE" or "VE,tG,Ab")
		remaining := args.ID
		for len(remaining) > 0 {
			var id string
			if idx := strings.IndexByte(remaining, ','); idx >= 0 {
				id = remaining[:idx]
				remaining = remaining[idx+1:]
			} else {
				id = remaining
				remaining = ""
			}
			id = strings.TrimSpace(id)
			if id != "" {
				objectIDs = append(objectIDs, id)
			}
		}
	}

	// Get components from index
	refTracker := s.goroutineIndex.GetRefTracker()
	graphPropagator := s.goroutineIndex.GetGraphPropagator()
	fileService := core.NewFileService()

	// Build context for each object using simplified ObjectContext type
	contexts := make([]ObjectContext, 0, len(objectIDs))
	for _, objectID := range objectIDs {
		objCtx := s.buildObjectContextCompact(ctx, objectID, refTracker, graphPropagator, fileService, &args)
		if objCtx != nil {
			contexts = append(contexts, *objCtx)
		}
	}

	// Create response with ultra-compact format for minimal context usage
	response := ContextResponse{
		Contexts: contexts,
		Count:    len(contexts),
	}

	// Use compact format with minimal context to reduce token usage
	return createCompactResponse(response, false, false)
}

// handleGetObjectContextWithMode handles context lookup with mode parameter
// This logic was migrated from context_lookup_tools.go
// @lci:labels[mcp-tool-handler,context,call-hierarchy,dependencies]
// @lci:category[mcp-api]
func (s *Server) handleGetObjectContextWithMode(ctx context.Context, args *ObjectContextParams) (*mcp.CallToolResult, error) {
	// Apply mode-specific presets
	s.applyContextLookupMode(args)

	// Validate server state before creating context lookup engine
	if s.goroutineIndex == nil {
		return createSmartErrorResponse("get_context", errors.New("index not initialized"), map[string]interface{}{
			"file_id": args.FileID,
			"name":    args.Name,
		})
	}

	// Create context lookup engine if not already created
	if s.contextLookupEngine == nil {
		s.contextLookupEngine = core.NewContextLookupEngine(
			s.goroutineIndex.GetSymbolIndex(),
			s.goroutineIndex.GetTrigramIndex(),
			core.NewFileService(),
			s.goroutineIndex.GetGraphPropagator(),
			s.goroutineIndex.GetSemanticAnnotator(),
			core.NewComponentDetector(),
			s.goroutineIndex.GetRefTracker(),
		)
	}

	// Configure the engine based on parameters
	s.configureContextLookupEngine(args)

	// Convert parameters to internal format
	objectID, err := s.paramsToObjectID(args)
	if err != nil {
		return nil, fmt.Errorf("failed to create object ID: %w", err)
	}

	// Execute context lookup
	startTime := s.getCurrentTimeMillis()

	context, err := s.contextLookupEngine.GetContext(*objectID)
	if err != nil {
		return nil, fmt.Errorf("context lookup failed: %w", err)
	}

	totalTime := s.getCurrentTimeMillis() - startTime

	// Filter context based on requested sections
	context = s.filterContextSections(context, args)

	// Create result (matching ContextLookupResult structure)
	// Calculate component timing breakdown (equal distribution as placeholder)
	const componentCount = 7 // basic_info, relationships, variables, semantic, structure, usage, ai
	perComponentTime := totalTime / componentCount

	result := map[string]interface{}{
		"context":  context,
		"metadata": s.createContextMetadata(),
		"performance": map[string]interface{}{
			"total_time_ms": totalTime,
			"component_breakdown": map[string]interface{}{
				"basic_info_time":    perComponentTime,
				"relationships_time": perComponentTime,
				"variables_time":     perComponentTime,
				"semantic_time":      perComponentTime,
				"structure_time":     perComponentTime,
				"usage_time":         perComponentTime,
				"ai_time":            perComponentTime,
			},
		},
	}

	// Return response
	return createJSONResponse(map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

// applyContextLookupMode applies mode-specific parameter presets
// Migrated from context_lookup_tools.go
func (s *Server) applyContextLookupMode(args *ObjectContextParams) {
	mode := args.Mode

	// Default mode
	if mode == "" {
		mode = "full"
	}

	switch mode {
	case "full":
		// All sections, full depth
		args.Mode = "full"
		if args.MaxDepth == 0 {
			args.MaxDepth = 5
		}
		args.IncludeAIText = true
		// No specific sections = all sections

	case "quick":
		// Minimal analysis for speed
		args.Mode = "quick"
		args.MaxDepth = 2
		args.IncludeAIText = false
		args.IncludeSections = []string{"relationships", "structure"}

	case "relationships":
		// Only relationship analysis
		args.Mode = "relationships"
		args.IncludeSections = []string{"relationships"}

	case "semantic":
		// Semantic and dependency context
		args.Mode = "semantic"
		args.IncludeSections = []string{"semantic", "ai"}

	case "usage":
		// Usage and impact analysis
		args.Mode = "usage"
		args.IncludeSections = []string{"usage"}

	case "variables":
		// Variable and data context
		args.Mode = "variables"
		args.IncludeSections = []string{"variables"}

	default:
		// Unknown mode, default to full
		args.Mode = "full"
	}
}

// buildObjectContextCompact creates a simplified ObjectContext for compact formatting
func (s *Server) buildObjectContextCompact(ctx context.Context, objectID string, refTracker, graphPropagator interface{}, fileService interface{}, args *ObjectContextParams) *ObjectContext {
	tracker, ok := refTracker.(*core.ReferenceTracker)
	if !ok || tracker == nil {
		return nil
	}

	// Parse objectID to extract symbol ID
	symbolID, err := mcputils.ParseSymbolID(objectID)
	if err != nil {
		return nil
	}

	// Get enhanced symbol from tracker
	enhancedSym := tracker.GetEnhancedSymbol(symbolID)
	if enhancedSym == nil {
		return nil
	}

	// Get file path from the index (FileService is empty, need to use goroutineIndex)
	filePath := s.goroutineIndex.GetFilePath(enhancedSym.FileID)

	// Create simplified ObjectContext
	result := &ObjectContext{
		FilePath:   filePath,
		Line:       enhancedSym.Line,
		ObjectID:   objectID,
		SymbolType: enhancedSym.Type.String(),
		SymbolName: enhancedSym.Name,
		IsExported: enhancedSym.IsExported,
		Signature:  enhancedSym.Signature,
	}

	// Add definition (use name as fallback)
	if enhancedSym.Signature != "" {
		result.Definition = enhancedSym.Signature
	} else {
		result.Definition = enhancedSym.Name
	}

	// Add context if requested
	if args.IncludeFileContext {
		// Simple context - just the definition line
		result.Context = []string{result.Definition}
	}

	return result
}

// ============================================================================
// Parameter Validation Functions
// ============================================================================

// validateGetContextParams validates get_context parameters
func (s *Server) validateGetContextParams(args *ObjectContextParams) error {
	hasID := args.ID != ""
	hasName := args.Name != ""

	// Must have either 'id' or 'name'
	if !hasID && !hasName {
		return errors.New("missing required 'id' parameter. Use the object ID (o=XX) from search results. Example: {\"id\": \"VE\"} or {\"id\": \"VE,tG\"}")
	}

	// Can't have both
	if hasID && hasName {
		return errors.New("parameter conflict: use either 'id' OR 'name', not both. Prefer 'id' with object IDs from search")
	}

	return nil
}

// ============================================================================
// Context Lookup Helper Functions (migrated from context_lookup_tools.go)
// ============================================================================

// configureContextLookupEngine configures the context lookup engine
func (s *Server) configureContextLookupEngine(params *ObjectContextParams) {
	if s.contextLookupEngine == nil {
		return
	}

	// Apply configuration
	s.contextLookupEngine.SetMaxContextDepth(params.MaxDepth)
	s.contextLookupEngine.SetIncludeAIText(params.IncludeAIText)
	s.contextLookupEngine.SetConfidenceThreshold(params.ConfidenceThreshold)
}

// paramsToObjectID converts parameters to object ID
func (s *Server) paramsToObjectID(params *ObjectContextParams) (*core.CodeObjectID, error) {
	// Default symbol type
	var symbolType types.SymbolType = types.SymbolTypeFunction

	// If we have an ID parameter, use it directly as the symbol ID
	symbolID := ""
	if params.ID != "" {
		// ID parameter contains the concise object ID directly
		// Use strings.Cut for zero-copy extraction of first ID
		if first, _, found := strings.Cut(params.ID, ","); found {
			symbolID = first
		} else {
			symbolID = params.ID
		}
	}

	// Create the object ID
	objectID := &core.CodeObjectID{
		FileID:   types.FileID(params.FileID),
		SymbolID: symbolID,
		Name:     params.Name,
		Type:     symbolType,
	}

	// If symbol ID is not provided, try to find the symbol in the file by name
	if symbolID == "" && params.Name != "" {
		symbol, err := s.findSymbolInFile(objectID.FileID, params.Name, params.Line, params.Column)
		if err != nil {
			return nil, fmt.Errorf("could not find symbol %s in file %d: %w", params.Name, params.FileID, err)
		}
		objectID.SymbolID = fmt.Sprintf("%d:%s", symbol.FileID, symbol.Name)
		objectID.Type = symbol.Type
	}

	return objectID, nil
}

// findSymbolInFile searches for a symbol in the specified file
func (s *Server) findSymbolInFile(fileID types.FileID, name string, line, column int) (*types.Symbol, error) {
	return &types.Symbol{
		FileID: fileID,
		Name:   name,
		Type:   types.SymbolTypeFunction,
		Line:   line,
		Column: column,
	}, nil
}

// filterContextSections filters context based on requested sections
func (s *Server) filterContextSections(context *core.CodeObjectContext, params *ObjectContextParams) *core.CodeObjectContext {
	// If no specific sections requested, return full context
	if len(params.IncludeSections) == 0 && len(params.ExcludeSections) == 0 {
		return context
	}

	// Create a filtered context
	filtered := *context

	// Exclude sections
	for _, section := range params.ExcludeSections {
		switch section {
		case "relationships":
			filtered.DirectRelationships = core.DirectRelationships{}
		case "variables":
			filtered.VariableContext = core.VariableContext{}
		case "semantic":
			filtered.SemanticContext = core.SemanticContext{}
		case "structure":
			filtered.StructureContext = core.StructureContext{}
		case "usage":
			filtered.UsageAnalysis = core.UsageAnalysis{}
		case "ai":
			filtered.AIContext = core.AIContext{}
		}
	}

	// If specific sections requested, exclude others
	if len(params.IncludeSections) > 0 {
		requestedSections := make(map[string]bool)
		for _, section := range params.IncludeSections {
			requestedSections[section] = true
		}

		if !requestedSections["relationships"] {
			filtered.DirectRelationships = core.DirectRelationships{}
		}
		if !requestedSections["variables"] {
			filtered.VariableContext = core.VariableContext{}
		}
		if !requestedSections["semantic"] {
			filtered.SemanticContext = core.SemanticContext{}
		}
		if !requestedSections["structure"] {
			filtered.StructureContext = core.StructureContext{}
		}
		if !requestedSections["usage"] {
			filtered.UsageAnalysis = core.UsageAnalysis{}
		}
		if !requestedSections["ai"] {
			filtered.AIContext = core.AIContext{}
		}
	}

	return &filtered
}

// createContextMetadata creates metadata for context lookup result
func (s *Server) createContextMetadata() map[string]interface{} {
	// Safe stats access with nil check
	var stats map[string]interface{}
	if s.goroutineIndex != nil {
		stats = s.goroutineIndex.GetStats()
	} else {
		stats = make(map[string]interface{})
	}

	// Get memory usage information
	memoryInfo := s.getMemoryUsageInfo()

	return map[string]interface{}{
		"index_size":      int64(getIntFromStats(stats, "total_size")),
		"processed_files": getIntFromStats(stats, "file_count"),
		"query_time":      s.formatDuration(s.getCurrentTimeMillis()),
		"server_version":  version.Info(),
		"memory_usage":    memoryInfo,
		"ast_mode":        "temporary",
	}
}

// getMemoryUsageInfo collects memory usage statistics
func (s *Server) getMemoryUsageInfo() map[string]interface{} {
	memoryInfo := map[string]interface{}{
		"total_memory_mb":   0.0,
		"ast_buffer_mb":     0.0,
		"function_cache_mb": 0.0,
		"metadata_mb":       0.0,
		"pressure_level":    "low",
	}

	// Estimate metadata memory usage
	if s.goroutineIndex != nil {
		stats := s.goroutineIndex.GetStats()
		if totalSize, ok := stats["total_size"].(float64); ok {
			// Assume about 65% is permanent metadata
			metadataMB := (totalSize / 1024 / 1024) * 0.65
			memoryInfo["metadata_mb"] = metadataMB
			memoryInfo["total_memory_mb"] = metadataMB
		}
	}

	return memoryInfo
}

// getCurrentTimeMillis returns current time in milliseconds
func (s *Server) getCurrentTimeMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// getIntFromStats safely gets integer value from stats map
func getIntFromStats(stats map[string]interface{}, key string) int {
	if val, ok := stats[key].(int); ok {
		return val
	}
	if val, ok := stats[key].(int64); ok {
		return int(val)
	}
	if val, ok := stats[key].(float64); ok {
		return int(val)
	}
	return 0
}

// formatDuration formats duration in milliseconds to human-readable string
func (s *Server) formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	} else if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	} else {
		return fmt.Sprintf("%.1fm", float64(ms)/60000)
	}
}
