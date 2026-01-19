package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/display"
	"github.com/standardbeagle/lci/internal/git"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/mcp"
	"github.com/standardbeagle/lci/internal/server"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/internal/version"

	"github.com/urfave/cli/v2"
)

var (
	Version      = version.Version // Use centralized version management
	indexer      *indexing.MasterIndex
	cleanupFuncs []func()
	projectRoot  string // Stores the absolute path to the project root for path conversion
)

// loadConfigWithOverrides loads configuration and applies CLI flag overrides
func loadConfigWithOverrides(c *cli.Context) (*config.Config, error) {
	configPath := c.String("config")

	// If root is specified and config path is default, look for config in root directory
	if rootFlag := c.String("root"); rootFlag != "" && configPath == ".lci.kdl" {
		configPath = filepath.Join(rootFlag, ".lci.kdl")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	// Apply CLI flag overrides
	if includeFlags := c.StringSlice("include"); len(includeFlags) > 0 {
		cfg.Include = includeFlags
	}
	if excludeFlags := c.StringSlice("exclude"); len(excludeFlags) > 0 {
		cfg.Exclude = append(cfg.Exclude, excludeFlags...)
	}
	if rootFlag := c.String("root"); rootFlag != "" {
		// Convert to absolute path to ensure consistent path handling
		absRoot, err := filepath.Abs(rootFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve root path %q: %w", rootFlag, err)
		}
		cfg.Project.Root = absRoot
	}

	return cfg, nil
}

func main() {
	app := &cli.App{
		Name:                   "lci",
		Usage:                  "Lightning fast code indexing for AI assistants",
		Version:                Version,
		UseShortOptionHandling: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Config file path",
				Value:   ".lci.kdl",
			},
			&cli.BoolFlag{
				Name:    "daemon",
				Aliases: []string{"d"},
				Usage:   "Run as daemon",
			},
			&cli.StringSliceFlag{
				Name:  "include",
				Usage: "Include files matching glob patterns (e.g., --include '*.go' --include 'src/**/*.ts')",
			},
			&cli.StringSliceFlag{
				Name:  "exclude",
				Usage: "Exclude files matching glob patterns (e.g., --exclude '**/test-projects/**')",
			},
			&cli.StringFlag{
				Name:    "root",
				Aliases: []string{"r"},
				Usage:   "Project root directory to index (overrides config)",
			},
			&cli.BoolFlag{
				Name:   "test-run",
				Usage:  "Show files that would be indexed without processing (hidden flag)",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:   "profile-memory",
				Usage:  "Write memory profile to file (e.g., --profile-memory mem.prof)",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:   "profile-cpu",
				Usage:  "Write CPU profile to file (e.g., --profile-cpu cpu.prof)",
				Hidden: true,
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "search",
				Aliases: []string{"s"},
				Usage:   "Search for pattern in codebase",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "max-lines",
						Aliases: []string{"m"},
						Usage:   "Max context lines (0=use blocks)",
						Value:   0,
					},
					&cli.BoolFlag{
						Name:    "case-insensitive",
						Aliases: []string{"i"},
						Usage:   "Case-insensitive search",
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output as JSON",
					},
					&cli.BoolFlag{
						Name:  "light",
						Usage: "Use light search without relational data and breadcrumbs",
					},
					&cli.StringFlag{
						Name:    "exclude",
						Aliases: []string{"e"},
						Usage:   "Exclude files matching regex pattern (e.g., '.*test.*\\.go$')",
						Value:   "",
					},
					&cli.StringFlag{
						Name:    "include",
						Aliases: []string{"inc"},
						Usage:   "Include only files matching regex pattern (e.g., '.*\\.go$')",
						Value:   "",
					},
					&cli.BoolFlag{
						Name:  "comments-only",
						Usage: "Search only in comments",
					},
					&cli.BoolFlag{
						Name:  "code-only",
						Usage: "Search only in code (excludes comments and strings)",
					},
					&cli.BoolFlag{
						Name:  "strings-only",
						Usage: "Search only in string literals",
					},
					&cli.BoolFlag{
						Name:  "template-strings",
						Usage: "Include template strings (sql``, gql``, etc.) when using --strings-only",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Show debug information",
					},
					&cli.BoolFlag{
						Name:  "compare-search",
						Usage: "Compare legacy and consolidated search implementations (A/B testing)",
					},
					&cli.StringFlag{
						Name:  "cpu-profile",
						Usage: "Write CPU profile to file",
					},
					&cli.StringFlag{
						Name:  "mem-profile",
						Usage: "Write memory profile to file",
					},
					// Grep-like features
					&cli.BoolFlag{
						Name:  "invert-match",
						Usage: "Inverted match (grep -v): show lines that DON'T match pattern",
					},
					&cli.StringSliceFlag{
						Name:    "patterns",
						Aliases: []string{"pattern"},
						Usage:   "Multiple patterns with OR logic (grep -e): --patterns TODO --patterns FIXME",
					},
					&cli.BoolFlag{
						Name:    "count",
						Aliases: []string{"c"},
						Usage:   "Return match count per file (grep -c)",
					},
					&cli.BoolFlag{
						Name:    "files-with-matches",
						Aliases: []string{"l"},
						Usage:   "Return only filenames with matches (grep -l)",
					},
					&cli.BoolFlag{
						Name:    "word-regexp",
						Aliases: []string{"w"},
						Usage:   "Match whole words only (grep -w)",
					},
					&cli.BoolFlag{
						Name:    "regex",
						Aliases: []string{"E"},
						Usage:   "Interpret pattern as extended regex (grep -E). Supports ^, $, *, +, ?, [], (), |",
					},
					&cli.IntFlag{
						Name:  "max-count",
						Usage: "Max matches per file (grep -m NUM), 0 = unlimited",
						Value: 0,
					},
					&cli.BoolFlag{
						Name:  "ids",
						Usage: "Include object IDs in results (default: true)",
					},
					&cli.BoolFlag{
						Name:  "no-ids",
						Usage: "Exclude object IDs from results",
					},
					// New AI-focused features
					&cli.BoolFlag{
						Name:  "compact-search",
						Aliases: []string{"cs"},
						Usage: "Show compact output (patterns only, no full context)",
					},
					&cli.StringFlag{
						Name:  "rank-by",
						Usage: "Rank results by: relevance, proximity, similarity (default: none)",
					},
					&cli.StringFlag{
						Name:  "context-filter",
						Usage: "Filter results by context pattern (e.g., 'cache', 'test')",
					},
				},
				Action: searchCommand,
			},
			{
				Name:    "grep",
				Aliases: []string{"g"},
				Usage:   "Ultra-fast text search (40% faster, 75% less memory)",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "max-results",
						Aliases: []string{"n"},
						Usage:   "Max number of results",
						Value:   500,
					},
					&cli.IntFlag{
						Name:    "context",
						Aliases: []string{"C"},
						Usage:   "Lines of context around matches",
						Value:   3,
					},
					&cli.BoolFlag{
						Name:    "case-insensitive",
						Aliases: []string{"i"},
						Usage:   "Case-insensitive search",
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output as JSON",
					},
					&cli.StringFlag{
						Name:    "exclude",
						Aliases: []string{"e"},
						Usage:   "Exclude files matching regex pattern",
						Value:   "",
					},
					&cli.StringFlag{
						Name:    "include",
						Aliases: []string{"inc"},
						Usage:   "Include only files matching regex pattern",
						Value:   "",
					},
					&cli.BoolFlag{
						Name:  "exclude-tests",
						Usage: "Exclude test files",
					},
					&cli.BoolFlag{
						Name:  "exclude-comments",
						Usage: "Exclude matches in comments",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Show debug information",
					},
					&cli.BoolFlag{
						Name:    "regex",
						Aliases: []string{"R"},
						Usage:   "Interpret pattern as extended regex (supports ^, $, *, +, ?, [], (), |)",
					},
				},
				Action: grepCommand,
			},
			{
				Name:    "def",
				Aliases: []string{"d"},
				Usage:   "Find symbol definition",
				Action:  definitionCommand,
			},
			{
				Name:    "refs",
				Aliases: []string{"r"},
				Usage:   "Find symbol references",
				Action:  referencesCommand,
			},
			{
				Name:    "tree",
				Aliases: []string{"t"},
				Usage:   "Display function call hierarchy tree with architectural annotations",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "max-depth",
						Aliases: []string{"d"},
						Usage:   "Maximum recursion depth for call tree",
						Value:   5,
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output as JSON",
					},
					&cli.BoolFlag{
						Name:  "show-lines",
						Usage: "Show line numbers for each call",
						Value: true,
					},
					&cli.BoolFlag{
						Name:  "compact",
						Usage: "Use compact output format",
					},
					&cli.StringFlag{
						Name:    "exclude",
						Aliases: []string{"e"},
						Usage:   "Exclude files matching regex pattern",
						Value:   "",
					},
					&cli.BoolFlag{
						Name:  "agent",
						Usage: "Output dense dependency data optimized for coding agents",
					},
					&cli.BoolFlag{
						Name:  "metrics",
						Usage: "Show complexity metrics for each function",
					},
				},
				Action: treeCommand,
			},
			// (Removed deprecated 'unroll' command placeholder)
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List files that would be indexed",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Show file details (size, priority)",
					},
				},
				Action: listCommand,
			},
			{
				Name:   "mcp",
				Usage:  "Start MCP (Model Context Protocol) server with stdio transport",
				Action: mcpCommand,
			},
			{
				Name:    "status",
				Aliases: []string{"st"},
				Usage:   "Show per-index status and progress information",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output as JSON",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Show detailed status information",
					},
					&cli.StringFlag{
						Name:    "index",
						Aliases: []string{"i"},
						Usage:   "Show status for specific index type (trigram, symbol, reference, callgraph, postings, location, content)",
						Value:   "",
					},
					&cli.BoolFlag{
						Name:  "health",
						Usage: "Show health monitoring information",
					},
					&cli.BoolFlag{
						Name:  "operations",
						Usage: "Show active operations and queue status",
					},
					&cli.BoolFlag{
						Name:  "errors",
						Usage: "Show error history and statistics",
					},
				},
				Action: statusCommand,
			},
			{
				Name:    "debug",
				Usage:   "Debug and diagnostic tools for symbol linking system",
				Aliases: []string{"dbg"},
				Description: `Provides various debugging and diagnostic tools to analyze the symbol linking system.

This command provides comprehensive debugging capabilities including:
- System information and statistics
- Consistency validation
- Dependency graph analysis
- Debug information export
- Graph visualization`,
				Subcommands: []*cli.Command{
					{
						Name:    "info",
						Usage:   "Show comprehensive debug information",
						Aliases: []string{"i"},
						Description: `Displays detailed information about the symbol linking system including:
- Files and their languages
- Symbol extraction statistics
- Module resolution results
- Performance metrics
- System health status`,
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "incremental",
								Usage:   "Use incremental engine for debugging",
								Aliases: []string{"inc"},
							},
							&cli.BoolFlag{
								Name:    "verbose",
								Usage:   "Enable verbose debug output",
								Aliases: []string{"v"},
							},
						},
						Action: runDebugInfo,
					},
					{
						Name:    "validate",
						Usage:   "Validate system consistency",
						Aliases: []string{"v"},
						Description: `Performs comprehensive consistency checks on the symbol linking system:
- FileID mapping consistency
- Extractor availability
- Symbol reference integrity
- Index health validation`,
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "incremental",
								Usage: "Use incremental engine for validation",
							},
						},
						Action: runDebugValidate,
					},
					{
						Name:    "deps",
						Usage:   "Analyze dependency graph",
						Aliases: []string{"dependencies"},
						Description: `Analyzes the dependency graph complexity and relationships.

This command provides insights into:
- File dependency relationships
- Circular dependency detection
- Dependency depth analysis
- Graph complexity metrics

Note: Requires incremental engine mode to track dependencies.`,
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "verbose",
								Usage:   "Show detailed dependency information",
								Aliases: []string{"v"},
							},
						},
						Action: runDebugDeps,
					},
					{
						Name:    "export",
						Usage:   "Export debug information to JSON",
						Aliases: []string{"e"},
						Description: `Exports comprehensive debug information to a JSON file for:
- External analysis tools
- Bug reporting
- Performance analysis
- System documentation`,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "output",
								Usage:   "Output file for exported debug information",
								Aliases: []string{"o"},
								Value:   "debug-info.json",
							},
							&cli.BoolFlag{
								Name:  "incremental",
								Usage: "Use incremental engine for export",
							},
							&cli.BoolFlag{
								Name:    "verbose",
								Usage:   "Show export preview",
								Aliases: []string{"v"},
							},
						},
						Action: runDebugExport,
					},
					{
						Name:    "graph",
						Usage:   "Export dependency graph in DOT format",
						Aliases: []string{"g"},
						Description: `Exports the dependency graph in DOT format for visualization with Graphviz.

The generated graph can be converted to various formats:
- PNG: dot -Tpng graph.dot -o graph.png
- SVG: dot -Tsvg graph.dot -o graph.svg
- PDF: dot -Tpdf graph.dot -o graph.pdf`,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "output",
								Usage:   "Output file for dependency graph",
								Aliases: []string{"o"},
								Value:   "dependency-graph.dot",
							},
						},
						Action: runDebugGraph,
					},
				},
			},
			{
				Name:  "config",
				Usage: "Configuration management commands",
				Subcommands: []*cli.Command{
					{
						Name:    "init",
						Aliases: []string{"i"},
						Usage:   "Initialize configuration file (.lci.kdl)",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "format",
								Aliases: []string{"f"},
								Usage:   "Output format: kdl, yaml, json",
								Value:   "kdl",
							},
							&cli.StringFlag{
								Name:    "output",
								Aliases: []string{"o"},
								Usage:   "Output file path (default: .lci.kdl)",
							},
							&cli.BoolFlag{
								Name:  "force",
								Usage: "Overwrite existing configuration file",
							},
							&cli.BoolFlag{
								Name:  "minimal",
								Usage: "Generate minimal config with only commonly changed settings",
							},
						},
						Action: configInitCommand,
					},
					{
						Name:    "show",
						Aliases: []string{"s"},
						Usage:   "Show current configuration values",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "format",
								Aliases: []string{"f"},
								Usage:   "Output format: kdl, yaml, json, table",
								Value:   "table",
							},
						},
						Action: configShowCommand,
					},
					{
						Name:    "validate",
						Aliases: []string{"v"},
						Usage:   "Validate configuration file",
						Action:  configValidateCommand,
					},
				},
			},
			{
				Name:    "git-analyze",
				Aliases: []string{"ga"},
				Usage:   "Analyze git changes for duplicates and naming consistency",
				Description: `Analyzes git changes (staged, work-in-progress, commits, or ranges) for:
- Duplicate code detection against existing codebase
- Naming consistency issues (case style, similar names, abbreviations)

Examples:
  lci git-analyze                       # Analyze staged changes (default)
  lci git-analyze --scope wip           # Analyze all uncommitted changes
  lci git-analyze --scope commit        # Analyze HEAD commit
  lci git-analyze --scope range --base main  # Analyze feature branch vs main`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "scope",
						Aliases: []string{"s"},
						Usage:   "What to analyze: staged (default), wip, commit, range",
						Value:   "staged",
					},
					&cli.StringFlag{
						Name:    "base",
						Aliases: []string{"b"},
						Usage:   "Base reference for comparison (e.g., HEAD, main, commit hash)",
					},
					&cli.StringFlag{
						Name:    "target",
						Aliases: []string{"t"},
						Usage:   "Target reference for range scope (default: HEAD)",
					},
					&cli.StringSliceFlag{
						Name:    "focus",
						Aliases: []string{"f"},
						Usage:   "Focus analysis: duplicates, naming (default: both)",
					},
					&cli.Float64Flag{
						Name:  "threshold",
						Usage: "Similarity threshold for duplicate detection (0.0-1.0)",
						Value: 0.8,
					},
					&cli.IntFlag{
						Name:    "max-findings",
						Aliases: []string{"m"},
						Usage:   "Maximum findings per category",
						Value:   20,
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output as JSON",
					},
				},
				Action: gitAnalyzeCommand,
			},
			{
				Name:    "server",
				Usage:   "Start persistent index server (shared between CLI and MCP)",
				Aliases: []string{"srv"},
				Description: `Start a persistent index server that keeps the index resident in memory.
Both CLI and MCP can connect to this server for faster query responses.

The server runs until explicitly shut down with 'lci shutdown'.`,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "daemon",
						Aliases: []string{"d"},
						Usage:   "Run as daemon (background process)",
					},
					&cli.BoolFlag{
						Name:  "foreground",
						Usage: "Run in foreground (for debugging)",
						Value: true,
					},
				},
				Action: serverCommand,
			},
			{
				Name:    "shutdown",
				Usage:   "Shutdown the persistent index server",
				Aliases: []string{"stop"},
				Description: `Gracefully shutdown the running index server.
This will free all memory and close the socket.`,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Force shutdown even if operations are in progress",
					},
				},
				Action: shutdownCommand,
			},
		},
		Before: func(c *cli.Context) error {
			// Setup profiling if requested
			if cpuProfilePath := c.String("profile-cpu"); cpuProfilePath != "" {
				debug.LogIndexing("Starting CPU profiling to %s\n", cpuProfilePath)
				f, err := os.Create(cpuProfilePath)
				if err != nil {
					return fmt.Errorf("failed to create CPU profile: %w", err)
				}
				if err := pprof.StartCPUProfile(f); err != nil {
					f.Close()
					return fmt.Errorf("failed to start CPU profile: %w", err)
				}
				// Register CPU profile cleanup
				cleanupFuncs = append(cleanupFuncs, func() {
					pprof.StopCPUProfile()
					f.Close()
				})
			}

			// Setup memory profiling cleanup if requested
			if memProfilePath := c.String("profile-memory"); memProfilePath != "" {
				cleanupFuncs = append(cleanupFuncs, func() {
					debug.LogIndexing("Writing memory profile to %s\n", memProfilePath)

					runtime.GC() // Force garbage collection before profiling

					f, err := os.Create(memProfilePath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create memory profile: %v\n", err)
						return
					}
					defer f.Close()

					if err := pprof.WriteHeapProfile(f); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to write memory profile: %v\n", err)
					}
				})
			}

			// Skip initialization for help commands (but not for test-run)
			if (c.NArg() == 0 && !c.Bool("test-run")) || c.Args().Get(0) == "help" || c.Bool("help") || c.Bool("version") {
				return nil
			}

			// Initialize config with CLI overrides
			cfg, err := loadConfigWithOverrides(c)
			if err != nil {
				return err
			}

			// Only create indexer if needed
			command := c.Args().Get(0)
			needsIndexer := command != "" && command != "config"

			if needsIndexer {
				indexer = indexing.NewMasterIndex(cfg)
				projectRoot = cfg.Project.Root // Set project root for path conversion

				// Register cleanup for indexer
				cleanupFuncs = append(cleanupFuncs, func() {
					if indexer != nil {
						indexer.Close()
					}
				})

				// For serve command, start indexing in background
				if command == "serve" {
					go func() {
						start := time.Now()
						err := indexer.IndexDirectory(context.Background(), cfg.Project.Root)
						if err != nil {
							debug.LogIndexing("Indexing error: %v\n", err)
						} else {
							debug.LogIndexing("Indexed in %v\n", time.Since(start))
						}
					}()

					// File watching is now built into the MasterIndex and started automatically
					if c.Bool("daemon") {
						debug.LogIndexing("Running in daemon mode with file watching enabled\n")
					}
				} else if command == "mcp" {
					// Enable MCP mode to suppress all debug output
					debug.SetMCPMode(true)
					// For MCP command, do NOT auto-index - let AI assistants control indexing
				} else if c.Bool("test-run") {
					// Test run mode - show files that would be indexed without processing
					debug.LogIndexing("Test run mode: showing files that would be indexed\n")
					return indexer.TestRun(context.Background(), cfg.Project.Root)
				}
				// Note: Commands like search, grep, def, refs, tree, stats, unroll, git-analyze
				// use the server for indexing - no local indexing needed here
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			// Handle test-run when used as standalone flag
			if c.Bool("test-run") {
				debug.LogIndexing("Test run mode: showing files that would be indexed\n")

				cfg, err := loadConfigWithOverrides(c)
				if err != nil {
					return err
				}

				// Create indexer for test run
				testIndexer := indexing.NewMasterIndex(cfg)
				return testIndexer.TestRun(context.Background(), cfg.Project.Root)
			}
			// Default to search if pattern provided
			if c.NArg() > 0 {
				return searchCommand(c)
			}

			// Auto-detect MCP mode: if stdin has JSON-RPC content, switch to MCP mode
			if isMCPMode() {
				debug.LogMCP("Auto-detected MCP mode, entering MCP server\n")
				return mcpCommand(c)
			}

			// Otherwise show help
			return cli.ShowAppHelp(c)
		},
	}

	// Global cleanup tracking
	var cleanupFuncs []func()

	// Handle cleanup on exit
	defer func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
		os.Exit(1)
	}
}

// isAssemblySearchCandidate checks if a pattern should trigger assembly search
func isAssemblySearchCandidate(pattern string) bool {
	// Don't run for very short patterns
	if len(pattern) < 8 {
		return false
	}

	// Debug: Always trigger for patterns starting with "Error:" for testing
	if strings.HasPrefix(pattern, "Error:") {
		return true
	}

	// HTML/JSX content
	if strings.Contains(pattern, "<") && strings.Contains(pattern, ">") {
		return true
	}

	// Error messages and log patterns
	errorPrefixes := []string{"Error:", "Warning:", "Failed", "Invalid", "Missing"}
	for _, prefix := range errorPrefixes {
		if strings.HasPrefix(pattern, prefix) {
			return true
		}
	}

	// Multi-word phrases (likely concatenated)
	if len(strings.Fields(pattern)) >= 4 {
		return true
	}

	// Path-like patterns
	if strings.Count(pattern, "/") >= 3 {
		return true
	}

	// SQL-like patterns
	upperPattern := strings.ToUpper(pattern)
	sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "FROM", "WHERE"}
	for _, keyword := range sqlKeywords {
		if strings.Contains(upperPattern, keyword) {
			return true
		}
	}

	return false
}

func grepCommand(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("usage: lci grep <pattern>")
	}

	pattern := c.Args().First()
	maxResults := c.Int("max-results")
	contextLines := c.Int("context")
	caseInsensitive := c.Bool("case-insensitive")
	excludePattern := c.String("exclude")
	includePattern := c.String("include")
	excludeTests := c.Bool("exclude-tests")
	excludeComments := c.Bool("exclude-comments")
	verbose := c.Bool("verbose")
	useRegex := c.Bool("regex")

	start := time.Now()

	// Use basic search with grep-optimized options (no semantic analysis)
	searchOptions := types.SearchOptions{
		CaseInsensitive:    caseInsensitive,
		UseRegex:           useRegex,
		MaxResults:         maxResults, // Pass through max results limit
		MaxContextLines:    contextLines,
		ExcludePattern:     excludePattern,
		IncludePattern:     includePattern,
		ExcludeTests:       excludeTests,
		ExcludeComments:    excludeComments,
		Verbose:            verbose,
		MergeFileResults:   false, // Keep individual matches for grep-like output
		EnsureCompleteStmt: false, // No statement completion for speed
	}

	// Load configuration
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure server is running (auto-start if needed)
	client, err := ensureServerRunning(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to index server: %w", err)
	}

	// Perform search using the server
	results, err := client.Search(pattern, searchOptions, maxResults)
	if err != nil {
		return fmt.Errorf("grep search failed: %w", err)
	}

	elapsed := time.Since(start)

	// Display results in grep-like format
	return displayGrepResults(c, pattern, results, elapsed)
}

func definitionCommand(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("usage: lci def <symbol>")
	}

	symbol := c.Args().First()
	maxResults := c.Int("max-results")
	if maxResults == 0 {
		maxResults = 100 // Default max results
	}

	// Load configuration
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure server is running (auto-start if needed)
	client, err := ensureServerRunning(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to index server: %w", err)
	}

	// Use server's definition endpoint
	results, err := client.GetDefinition(symbol, maxResults)
	if err != nil {
		return fmt.Errorf("definition search failed: %w", err)
	}

	// Output format: file:line: signature or name
	for _, r := range results {
		if r.Signature != "" {
			fmt.Printf("%s:%d: %s\n", r.FilePath, r.Line, r.Signature)
		} else {
			fmt.Printf("%s:%d: %s %s\n", r.FilePath, r.Line, r.Type, r.Name)
		}
	}

	return nil
}

func referencesCommand(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("usage: lci refs <symbol>")
	}

	symbol := c.Args().First()
	maxResults := c.Int("max-results")
	if maxResults == 0 {
		maxResults = 100 // Default max results
	}

	// Load configuration
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure server is running (auto-start if needed)
	client, err := ensureServerRunning(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to index server: %w", err)
	}

	// Use server's references endpoint
	results, err := client.GetReferences(symbol, maxResults)
	if err != nil {
		return fmt.Errorf("references search failed: %w", err)
	}

	// Output format: file:line: context or match (maintaining same format as before)
	for _, r := range results {
		if r.Context != "" {
			fmt.Printf("%s:%d: %s\n", r.FilePath, r.Line, r.Context)
		} else {
			fmt.Printf("%s:%d: %s\n", r.FilePath, r.Line, r.Match)
		}
	}

	return nil
}

// determineFormat determines the output format based on CLI flags
func determineFormat(c *cli.Context) string {
	if c.Bool("json") {
		return "json"
	}
	if c.Bool("compact") {
		return "compact"
	}
	return "text"
}

func listCommand(c *cli.Context) error {
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return err
	}

	// Create indexer with the config
	listIndexer := indexing.NewMasterIndex(cfg)

	// List files with optional verbose output
	verbose := c.Bool("verbose")
	return listIndexer.ListFiles(context.Background(), cfg.Project.Root, verbose)
}

func treeCommand(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("usage: lci tree <function_name>")
	}

	functionName := c.Args().First()
	maxDepth := c.Int("max-depth")
	showLines := c.Bool("show-lines")
	compact := c.Bool("compact")
	excludePattern := c.String("exclude")
	agentMode := c.Bool("agent")

	// Load configuration
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure server is running (auto-start if needed)
	client, err := ensureServerRunning(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to index server: %w", err)
	}

	start := time.Now()
	tree, err := client.GetTree(functionName, maxDepth, showLines, compact, agentMode, excludePattern)
	if err != nil {
		return fmt.Errorf("error generating tree: %v", err)
	}

	elapsed := time.Since(start)

	if c.Bool("json") {
		output := map[string]interface{}{
			"function": functionName,
			"time_ms":  float64(elapsed.Microseconds()) / 1000.0,
			"tree":     tree,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	// Use the new tree formatter
	formatter := display.NewTreeFormatter(display.FormatterOptions{
		Format:      determineFormat(c),
		ShowLines:   showLines,
		ShowMetrics: c.Bool("metrics"),
		AgentMode:   agentMode,
		MaxDepth:    maxDepth,
		Indent:      "  ",
	})

	fmt.Printf("Function call tree for '%s' (generated in %.1fms)\n\n", functionName, float64(elapsed.Microseconds())/1000.0)
	fmt.Print(formatter.Format(tree))
	return nil
}

// unrollCommand removed (placeholder feature deprecated)

func mcpCommand(c *cli.Context) error {
	// Enable MCP mode to suppress all debug output
	debug.SetMCPMode(true)

	// Set GOMAXPROCS to limit CPU usage for optimal indexing performance
	// Use 4 goroutines as the optimal configuration (matches workflow test settings)
	// Using too many goroutines causes lock contention and slower indexing
	maxProcs := 4
	// Allow override via environment variable for advanced users
	if envProcs := os.Getenv("LCI_MAX_PROCS"); envProcs != "" {
		if parsed, err := strconv.Atoi(envProcs); err == nil && parsed > 0 {
			maxProcs = parsed
			fmt.Fprintf(os.Stderr, "Warning: Using custom GOMAXPROCS=%d (workflow tests use 4)\n", maxProcs)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid LCI_MAX_PROCS value '%s': %v. Using default: %d\n", envProcs, err, maxProcs)
		}
	}
	runtime.GOMAXPROCS(maxProcs)

	// Handle auto-detection case where indexer wasn't created in Before hook
	if indexer == nil {
		// Load config and create indexer for auto-detected MCP mode
		cfg, err := loadConfigWithOverrides(c)
		if err != nil {
			return debug.Fatal("failed to load config: %v\n", err)
		}

		indexer = indexing.NewMasterIndex(cfg)
		projectRoot = cfg.Project.Root // Set project root for path conversion
	}

	// Load config (already loaded in Before hook, but we need it for MCP server)
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return debug.Fatal("failed to load config: %v\n", err)
	}

	// Start the shared index server in-process so CLI commands can connect to it
	// This allows the MCP and CLI to share the same index via RPC
	indexServer, err := startSharedIndexServer(cfg, indexer)
	if err != nil {
		debug.LogMCP("Warning: Failed to start shared index server: %v\n", err)
		debug.LogMCP("MCP will continue, but CLI commands won't be able to connect\n")
		// Continue anyway - MCP can still work without the RPC server
	} else {
		debug.LogMCP("Shared index server started, CLI commands can connect\n")
	}

	// Create and start MCP server with new architecture
	mcpServer, err := mcp.NewServer(indexer, cfg)
	if err != nil {
		return debug.Fatal("failed to create MCP server: %v\n", err)
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start MCP server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		debug.LogMCP("Starting MCP server with stdio transport...\n")
		errChan <- mcpServer.Start(ctx)
	}()

	// Wait for either server error or shutdown signal
	select {
	case err := <-errChan:
		if err != nil {
			// Shutdown index server before returning error
			if indexServer != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				indexServer.Shutdown(shutdownCtx)
			}
			return debug.Fatal("MCP server error: %v\n", err)
		}
		// Shutdown index server on normal exit
		if indexServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			indexServer.Shutdown(shutdownCtx)
		}
		return nil
	case sig := <-sigChan:
		debug.LogMCP("Received signal %v, shutting down gracefully...\n", sig)
		cancel()

		// Give the server a moment to shutdown gracefully
		shutdownTimer := time.NewTimer(2 * time.Second)
		defer shutdownTimer.Stop()

		select {
		case err := <-errChan:
			debug.LogMCP("Server shutdown completed\n")
			// Shutdown index server
			if indexServer != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				indexServer.Shutdown(shutdownCtx)
			}
			return err
		case <-shutdownTimer.C:
			debug.LogMCP("Graceful shutdown timeout, forcing exit\n")
			// Force close stdin to break the stdio transport loop
			os.Stdin.Close()

			// Give it one more brief moment after closing stdin
			forceTimer := time.NewTimer(500 * time.Millisecond)
			defer forceTimer.Stop()

			select {
			case err := <-errChan:
				debug.LogMCP("Server shutdown completed after stdin close\n")
				// Shutdown index server
				if indexServer != nil {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutdownCancel()
					indexServer.Shutdown(shutdownCtx)
				}
				return err
			case <-forceTimer.C:
				debug.LogMCP("Force shutdown timeout exceeded\n")
				// Shutdown index server
				if indexServer != nil {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutdownCancel()
					indexServer.Shutdown(shutdownCtx)
				}
				return nil // Exit cleanly rather than error
			}
		}
	}
}

func configInitCommand(c *cli.Context) error {
	format := c.String("format")
	output := c.String("output")
	force := c.Bool("force")
	minimal := c.Bool("minimal")

	// Determine output file path
	if output == "" {
		switch format {
		case "kdl":
			output = ".lci.kdl"
		case "yaml":
			output = ".lci.kdl"
		case "json":
			output = ".lci.kdl.json"
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}
	}

	// Check if file exists
	if !force {
		if _, err := os.Stat(output); err == nil {
			return fmt.Errorf("configuration file %s already exists (use --force to overwrite)", output)
		}
	}

	// Generate configuration content
	var content string
	var err error

	switch format {
	case "kdl":
		content, err = generateKDLConfig(minimal)
	case "yaml":
		content, err = generateYAMLConfig(minimal)
	case "json":
		content, err = generateJSONConfig(minimal)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to generate config: %v", err)
	}

	// Write file
	if err := os.WriteFile(output, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	fmt.Printf("Configuration file created: %s\n", output)
	fmt.Printf("Edit the file to customize settings for your project.\n")

	if format == "kdl" {
		fmt.Printf("\nCommon customizations:\n")
		fmt.Printf("  - Adjust memory limits: index.max_total_size_mb\n")
		fmt.Printf("  - Add project exclusions: exclude { \"**/my-folder/**\" }\n")
		fmt.Printf("  - Include additional languages: include { \"*.rs\" }\n")
	}

	return nil
}

func configShowCommand(c *cli.Context) error {
	format := c.String("format")
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	if format == "table" {
		return displayConfigTable(cfg)
	}
	// Default to KDL output
	content, err := configToKDL(cfg)
	if err != nil {
		return fmt.Errorf("failed to convert to KDL: %v", err)
	}
	fmt.Print(content)
	return nil
}

func configValidateCommand(c *cli.Context) error {
	configPath := c.String("config")

	// Try to load the configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		return err
	}

	// Additional validation checks
	warnings := []string{}

	// Check memory limits
	if cfg.Performance.MaxMemoryMB < 100 {
		warnings = append(warnings, "MaxMemoryMB is very low (<100MB), may cause performance issues")
	}
	if cfg.Performance.MaxMemoryMB > 8000 {
		warnings = append(warnings, "MaxMemoryMB is very high (>8GB), ensure you have sufficient RAM")
	}

	// Check index limits
	if cfg.Index.MaxTotalSizeMB < 50 {
		warnings = append(warnings, "MaxTotalSizeMB is very low (<50MB), may limit indexing capability")
	}
	if cfg.Index.MaxFileCount < 100 {
		warnings = append(warnings, "MaxFileCount is very low (<100), may limit indexing capability")
	}

	// Check file patterns
	if len(cfg.Include) == 0 {
		warnings = append(warnings, "No include patterns specified, no files will be indexed")
	}

	// Display results
	fmt.Printf("✅ Configuration file is valid\n")
	fmt.Printf("📍 Config source: %s\n", configPath)
	fmt.Printf("📊 Settings: %d files max, %dMB memory limit, %dMB index limit\n",
		cfg.Index.MaxFileCount, cfg.Performance.MaxMemoryMB, cfg.Index.MaxTotalSizeMB)

	if len(warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings:\n")
		for _, warning := range warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	return nil
}

func generateKDLConfig(minimal bool) (string, error) {
	if minimal {
		return `// Lightning Code Index Configuration
// Minimal configuration with commonly changed settings

index {
    max_total_size_mb 500          // Total indexed content limit
    max_file_count 10000           // Maximum number of files
    smart_size_control true        // Enable intelligent size management
    priority_mode "recent"         // Priority: "recent", "small", "important"
}

performance {
    max_memory_mb 500              // Memory limit for entire index
}

// Add project-specific exclusions
exclude {
    // "**/my-large-folder/**"
    // "**/*.generated.ts"
}

// Add additional file types to index
include {
    // "*.rs"                      // Rust files
    // "*.zig"                     // Zig files
}
`, nil
	}

	// Read the full example file
	examplePath := ".lci.kdl.example"
	if content, err := os.ReadFile(examplePath); err == nil {
		return string(content), nil
	}

	// Fallback to embedded template
	return `// Lightning Code Index Configuration
// Full configuration template with all available options

project {
    name "my-project"
    root "."
}

index {
    max_file_size "10MB"           // Skip files larger than this
    max_total_size_mb 500          // Total indexed content limit
    max_file_count 10000           // Maximum number of files to index
    smart_size_control true        // Enable intelligent size management
    priority_mode "recent"         // Priority: "recent", "small", "important", "balanced"
    follow_symlinks false          // Don't follow symbolic links
}

performance {
    max_memory_mb 500              // Memory limit for entire index
    max_goroutines 8               // Parallel processing limit
    debounce_ms 100                // File change debouncing
}

search {
    max_results 100                // Limit search results
    max_context_lines 50           // Context around matches
    enable_fuzzy true              // Enable fuzzy matching
}

// Include specific file patterns (extends defaults)
include {
    "*.rs"                         // Rust files
    "*.zig"                        // Zig files  
    "*.lua"                        // Lua scripts
}

// Exclude specific patterns (extends defaults)
// Note: All hidden directories (.*/) are excluded by default
exclude {
    "**/my-large-data/**"          // Project-specific exclusions
    "**/*.generated.ts"            // Generated TypeScript
}
`, nil
}

func generateYAMLConfig(minimal bool) (string, error) {
	// Create default config and marshal to YAML
	_ = &config.Config{
		Version: 1,
		Project: config.Project{Root: "."},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   500,
			MaxFileCount:     10000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
		},
		Performance: config.Performance{
			MaxMemoryMB:   500,
			MaxGoroutines: 8,
			DebounceMs:    100,
		},
		Search: config.Search{
			MaxResults:             100,
			MaxContextLines:        50,
			EnableFuzzy:            true,
			DefaultContextLines:    0,
			MergeFileResults:       true,
			EnsureCompleteStmt:     false,
			IncludeLeadingComments: true,
		},
		Include: []string{"*.go", "*.js", "*.jsx", "*.ts", "*.tsx", "*.py"},
		Exclude: []string{"**/.*/**", "**/node_modules/**", "**/vendor/**"},
	}

	// This would need yaml marshaling - for now return a template
	return `version: 1
project:
  root: "."
  name: "my-project"
index:
  max_file_size: 10485760  # 10MB
  max_total_size_mb: 500
  max_file_count: 10000
  follow_symlinks: false
  smart_size_control: true
  priority_mode: "recent"
performance:
  max_memory_mb: 500
  max_goroutines: 8
  debounce_ms: 100
search:
  max_results: 100
  max_context_lines: 50
  enable_fuzzy: true
include:
  - "*.go"
  - "*.js"
  - "*.jsx"
  - "*.ts"
  - "*.tsx"
  - "*.py"
exclude:
  - "**/.*/**"
  - "**/node_modules/**"
  - "**/vendor/**"
`, nil
}

func generateJSONConfig(minimal bool) (string, error) {
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: ".", Name: "my-project"},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   500,
			MaxFileCount:     10000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
		},
		Performance: config.Performance{
			MaxMemoryMB:   500,
			MaxGoroutines: 8,
			DebounceMs:    100,
		},
		Search: config.Search{
			MaxResults:             100,
			MaxContextLines:        50,
			EnableFuzzy:            true,
			DefaultContextLines:    0,
			MergeFileResults:       true,
			EnsureCompleteStmt:     false,
			IncludeLeadingComments: true,
		},
		Include: []string{"*.go", "*.js", "*.jsx", "*.ts", "*.tsx", "*.py"},
		Exclude: []string{"**/.*/**", "**/node_modules/**", "**/vendor/**"},
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func configToKDL(cfg *config.Config) (string, error) {
	// Convert config back to KDL format
	return fmt.Sprintf(`// Current Lightning Code Index Configuration

project {
    name "%s"
    root "%s"
}

index {
    max_file_size "%dB"
    max_total_size_mb %d
    max_file_count %d
    smart_size_control %t
    priority_mode "%s"
    follow_symlinks %t
    respect_gitignore %t
}

performance {
    max_memory_mb %d
    max_goroutines %d
    debounce_ms %d
}

search {
    max_results %d
    max_context_lines %d
    enable_fuzzy %t
}

// Include patterns
%s

// Exclude patterns  
%s
`,
		cfg.Project.Name,
		cfg.Project.Root,
		cfg.Index.MaxFileSize,
		cfg.Index.MaxTotalSizeMB,
		cfg.Index.MaxFileCount,
		cfg.Index.SmartSizeControl,
		cfg.Index.PriorityMode,
		cfg.Index.FollowSymlinks,
		cfg.Index.RespectGitignore,
		cfg.Performance.MaxMemoryMB,
		cfg.Performance.MaxGoroutines,
		cfg.Performance.DebounceMs,
		cfg.Search.MaxResults,
		cfg.Search.MaxContextLines,
		cfg.Search.EnableFuzzy,
		formatKDLStringArray("include", cfg.Include),
		formatKDLStringArray("exclude", cfg.Exclude),
	), nil
}

func formatKDLStringArray(section string, items []string) string {
	if len(items) == 0 {
		return section + " {\n    // No items\n}"
	}

	result := section + " {\n"
	for _, item := range items {
		result += fmt.Sprintf("    \"%s\"\n", item)
	}
	result += "}"
	return result
}

func displayConfigTable(cfg *config.Config) error {
	fmt.Printf("Lightning Code Index Configuration\n")
	fmt.Printf("=================================\n\n")

	fmt.Printf("Project Settings:\n")
	fmt.Printf("  Name:              %s\n", cfg.Project.Name)
	fmt.Printf("  Root:              %s\n", cfg.Project.Root)
	fmt.Printf("\n")

	fmt.Printf("Index Settings:\n")
	fmt.Printf("  Max file size:     %.1f MB\n", float64(cfg.Index.MaxFileSize)/(1024*1024))
	fmt.Printf("  Max total size:    %d MB\n", cfg.Index.MaxTotalSizeMB)
	fmt.Printf("  Max file count:    %d\n", cfg.Index.MaxFileCount)
	fmt.Printf("  Smart size control: %t\n", cfg.Index.SmartSizeControl)
	fmt.Printf("  Priority mode:     %s\n", cfg.Index.PriorityMode)
	fmt.Printf("  Follow symlinks:   %t\n", cfg.Index.FollowSymlinks)
	fmt.Printf("  Respect .gitignore: %t\n", cfg.Index.RespectGitignore)
	fmt.Printf("\n")

	fmt.Printf("Performance Settings:\n")
	fmt.Printf("  Max memory:        %d MB\n", cfg.Performance.MaxMemoryMB)
	fmt.Printf("  Max goroutines:    %d\n", cfg.Performance.MaxGoroutines)
	fmt.Printf("  Debounce:          %d ms\n", cfg.Performance.DebounceMs)
	fmt.Printf("\n")

	fmt.Printf("Search Settings:\n")
	fmt.Printf("  Max results:       %d\n", cfg.Search.MaxResults)
	fmt.Printf("  Max context lines: %d\n", cfg.Search.MaxContextLines)
	fmt.Printf("  Enable fuzzy:      %t\n", cfg.Search.EnableFuzzy)
	fmt.Printf("\n")

	fmt.Printf("Include Patterns (%d):\n", len(cfg.Include))
	for _, pattern := range cfg.Include {
		fmt.Printf("  %s\n", pattern)
	}
	fmt.Printf("\n")

	fmt.Printf("Exclude Patterns (%d):\n", len(cfg.Exclude))
	for _, pattern := range cfg.Exclude {
		fmt.Printf("  %s\n", pattern)
	}

	return nil
}

// isMCPMode detects if lci should enter MCP mode
func isMCPMode() bool {
	// Priority 1: Explicit environment variable (for MCP clients to set)
	if os.Getenv("LCI_MCP_MODE") == "1" || os.Getenv("LCI_MCP_MODE") == "true" {
		return true
	}

	// Priority 2: Non-terminal stdin (pipes, redirects) - likely JSON-RPC
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		return true
	}

	// Priority 3: Check if running as MCP server binary
	if len(os.Args) > 0 {
		arg0 := strings.ToLower(filepath.Base(os.Args[0]))
		if strings.Contains(arg0, "mcp") || strings.Contains(arg0, "server") {
			return true
		}
	}

	// Priority 4: Parent process detection (Linux-specific)
	if isParentMCPClient() {
		return true
	}

	return false
}

// isParentMCPClient checks if parent process suggests MCP usage (Linux-specific)
func isParentMCPClient() bool {
	ppid := os.Getppid()
	if ppid <= 1 {
		return false
	}

	// Check if parent process name suggests MCP client
	commPath := fmt.Sprintf("/proc/%d/comm", ppid)
	if parentCmd, err := os.ReadFile(commPath); err == nil {
		parentName := strings.TrimSpace(string(parentCmd))
		// Common MCP client names
		mcpClients := []string{"mcp-tui", "mcp-client", "claude", "cursor", "vscode"}
		for _, client := range mcpClients {
			if strings.Contains(strings.ToLower(parentName), client) {
				return true
			}
		}
	}

	return false
}

// gitAnalyzeCommand handles the git-analyze CLI command
func gitAnalyzeCommand(c *cli.Context) error {
	// Parse flags
	scope := c.String("scope")
	baseRef := c.String("base")
	targetRef := c.String("target")
	focus := c.StringSlice("focus")
	threshold := c.Float64("threshold")
	maxFindings := c.Int("max-findings")
	jsonOutput := c.Bool("json")

	// Validate scope
	switch scope {
	case "staged", "wip", "commit", "range":
		// Valid scopes
	default:
		return fmt.Errorf("invalid scope: %s (must be staged, wip, commit, or range)", scope)
	}

	// Check for required args
	if scope == "range" && baseRef == "" {
		return errors.New("--base is required for range scope")
	}

	// Load configuration
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure server is running (auto-start if needed)
	client, err := ensureServerRunning(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to index server: %w", err)
	}

	// Build request
	req := server.GitAnalyzeRequest{
		Scope:               scope,
		BaseRef:             baseRef,
		TargetRef:           targetRef,
		Focus:               focus,
		SimilarityThreshold: threshold,
		MaxFindings:         maxFindings,
	}

	// Run analysis via server
	reportData, err := client.GitAnalyze(req)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Convert to AnalysisReport for output
	// The server returns the report as interface{}, we need to convert it
	reportJSON, err := json.Marshal(reportData)
	if err != nil {
		return fmt.Errorf("failed to process report: %w", err)
	}

	var report git.AnalysisReport
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return fmt.Errorf("failed to parse report: %w", err)
	}

	// Output results
	if jsonOutput {
		return outputGitAnalyzeJSON(&report)
	}
	return outputGitAnalyzeText(&report)
}

// outputGitAnalyzeJSON outputs the analysis report as JSON
func outputGitAnalyzeJSON(report *git.AnalysisReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// outputGitAnalyzeText outputs the analysis report as formatted text
func outputGitAnalyzeText(report *git.AnalysisReport) error {
	// Header
	fmt.Println("Git Change Analysis")
	fmt.Println("==================")
	fmt.Println()

	// Summary
	fmt.Println("Summary")
	fmt.Println("-------")
	fmt.Printf("Files changed: %d | Symbols: +%d ~%d\n",
		report.Summary.FilesChanged,
		report.Summary.SymbolsAdded,
		report.Summary.SymbolsModified)
	fmt.Printf("Issues: %d duplicates, %d naming | Risk: %.0f%%\n",
		report.Summary.DuplicatesFound,
		report.Summary.NamingIssuesFound,
		report.Summary.RiskScore*100)

	if report.Summary.TopRecommendation != "" {
		fmt.Printf("\nTop recommendation: %s\n", report.Summary.TopRecommendation)
	}

	// Duplicates
	if len(report.Duplicates) > 0 {
		fmt.Println("\nDuplicates")
		fmt.Println("----------")
		for _, dup := range report.Duplicates {
			severity := strings.ToUpper(string(dup.Severity))
			fmt.Printf("[%s] %s duplicate (%.0f%%)\n",
				severity, dup.Type, dup.Similarity*100)
			fmt.Printf("  New: %s:%d (%s)\n",
				dup.NewCode.FilePath, dup.NewCode.StartLine, dup.NewCode.SymbolName)
			fmt.Printf("  Existing: %s:%d (%s)\n",
				dup.ExistingCode.FilePath, dup.ExistingCode.StartLine, dup.ExistingCode.SymbolName)
			fmt.Printf("  → %s\n", dup.Suggestion)
		}
	}

	// Naming issues
	if len(report.NamingIssues) > 0 {
		fmt.Println("\nNaming Issues")
		fmt.Println("-------------")
		for _, issue := range report.NamingIssues {
			severity := strings.ToUpper(string(issue.Severity))
			fmt.Printf("[%s] %s\n", severity, issue.IssueType)
			fmt.Printf("  Symbol: %s (%s:%d)\n",
				issue.NewSymbol.Name, issue.NewSymbol.FilePath, issue.NewSymbol.Line)
			fmt.Printf("  Issue: %s\n", issue.Issue)
			fmt.Printf("  → %s\n", issue.Suggestion)
		}
	}

	// Metadata
	fmt.Printf("\nAnalysis: %s → %s (%dms)\n",
		report.Metadata.BaseRef,
		report.Metadata.TargetRef,
		report.Metadata.AnalysisTimeMs)

	return nil
}
