package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/symbollinker"
)

// createDebugCommands creates debug CLI commands using urfave/cli
//
//nolint:unused // Reserved for future debug functionality
func createDebugCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "debug",
			Usage:   "Debug and diagnostic tools for symbol linking system",
			Aliases: []string{"d"},
			Description: `Provides various debugging and diagnostic tools to analyze the symbol linking system.
			
This command provides comprehensive debugging capabilities including:
- System information and statistics
- Consistency validation
- Dependency graph analysis
- Debug information export
- Graph visualization`,
			Subcommands: []*cli.Command{
				createDebugInfoCommand(),
				createDebugValidateCommand(),
				createDebugDepsCommand(),
				createDebugExportCommand(),
				createDebugGraphCommand(),
			},
		},
	}
}

//nolint:unused // Reserved for future debug functionality
func createDebugInfoCommand() *cli.Command {
	return &cli.Command{
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
	}
}

//nolint:unused // Reserved for future debug functionality
func createDebugValidateCommand() *cli.Command {
	return &cli.Command{
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
	}
}

//nolint:unused // Reserved for future debug functionality
func createDebugDepsCommand() *cli.Command {
	return &cli.Command{
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
	}
}

//nolint:unused // Reserved for future debug functionality
func createDebugExportCommand() *cli.Command {
	return &cli.Command{
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
	}
}

//nolint:unused // Reserved for future debug functionality
func createDebugGraphCommand() *cli.Command {
	return &cli.Command{
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
	}
}

func runDebugInfo(c *cli.Context) error {
	rootPath := "."
	if c.NArg() > 0 {
		rootPath = c.Args().Get(0)
	}

	incremental := c.Bool("incremental")
	verbose := c.Bool("verbose")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Debug Info - Lightning Code Index Symbol Linking System\n")
	fmt.Printf("Root Path: %s\n", rootPath)
	fmt.Printf("Incremental Mode: %v\n", incremental)
	fmt.Println()

	// Create and setup engine
	var engine interface {
		IndexFile(path string, content []byte) error
		LinkSymbols() error
		WriteDebugInfo(w io.Writer) error
		ValidateConsistency() []string
	}

	if incremental {
		incEngine := symbollinker.NewIncrementalEngine(rootPath)
		engine = incEngine
	} else {
		engine = symbollinker.NewSymbolLinkerEngine(rootPath)
	}

	// Index files
	fmt.Println("Building index...")
	err = indexAllFiles(engine, rootPath, cfg, verbose)
	if err != nil {
		return fmt.Errorf("failed to index files: %w", err)
	}

	// Link symbols
	fmt.Println("Linking symbols...")
	err = engine.LinkSymbols()
	if err != nil {
		return fmt.Errorf("failed to link symbols: %w", err)
	}

	// Print debug information
	fmt.Println("\nDebug Information:")
	fmt.Println(strings.Repeat("=", 80))
	err = engine.WriteDebugInfo(os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to print debug info: %w", err)
	}

	// Validate consistency
	if verbose {
		fmt.Println("Validation Results:")
		fmt.Println(strings.Repeat("-", 40))
		issues := engine.ValidateConsistency()
		if len(issues) == 0 {
			fmt.Println("✓ No consistency issues found")
		} else {
			fmt.Printf("⚠ Found %d consistency issues:\n", len(issues))
			for i, issue := range issues {
				fmt.Printf("  %d. %s\n", i+1, issue)
			}
		}
		fmt.Println()
	}

	return nil
}

func runDebugValidate(c *cli.Context) error {
	rootPath := "."
	if c.NArg() > 0 {
		rootPath = c.Args().Get(0)
	}

	incremental := c.Bool("incremental")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Validating Symbol Linking System\n")
	fmt.Printf("Root Path: %s\n", rootPath)
	fmt.Printf("Incremental Mode: %v\n", incremental)
	fmt.Println()

	// Create and setup engine
	var engine interface {
		IndexFile(path string, content []byte) error
		LinkSymbols() error
		ValidateConsistency() []string
	}

	if incremental {
		engine = symbollinker.NewIncrementalEngine(rootPath)
	} else {
		engine = symbollinker.NewSymbolLinkerEngine(rootPath)
	}

	// Index files
	fmt.Println("Building index...")
	err = indexAllFiles(engine, rootPath, cfg, false)
	if err != nil {
		return fmt.Errorf("failed to index files: %w", err)
	}

	// Link symbols
	fmt.Println("Linking symbols...")
	err = engine.LinkSymbols()
	if err != nil {
		return fmt.Errorf("failed to link symbols: %w", err)
	}

	// Validate consistency
	fmt.Println("Running consistency checks...")
	issues := engine.ValidateConsistency()

	if len(issues) == 0 {
		fmt.Println("✓ All consistency checks passed!")
		fmt.Println("The symbol linking system is operating correctly.")
	} else {
		fmt.Printf("⚠ Found %d consistency issues:\n\n", len(issues))
		for i, issue := range issues {
			fmt.Printf("%d. %s\n", i+1, issue)
		}
		fmt.Printf("\nRecommendation: Review the issues above and check your configuration.\n")
		return fmt.Errorf("consistency validation failed with %d issues", len(issues))
	}

	return nil
}

func runDebugDeps(c *cli.Context) error {
	rootPath := "."
	if c.NArg() > 0 {
		rootPath = c.Args().Get(0)
	}

	verbose := c.Bool("verbose")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Dependency Graph Analysis\n")
	fmt.Printf("Root Path: %s\n", rootPath)
	fmt.Println()

	// Create incremental engine (required for dependency analysis)
	engine := symbollinker.NewIncrementalEngine(rootPath)

	// Index files
	fmt.Println("Building index...")
	err = indexAllFiles(engine, rootPath, cfg, verbose)
	if err != nil {
		return fmt.Errorf("failed to index files: %w", err)
	}

	// Link symbols to build dependency graph
	fmt.Println("Linking symbols and building dependency graph...")
	err = engine.LinkSymbols()
	if err != nil {
		return fmt.Errorf("failed to link symbols: %w", err)
	}

	// Analyze dependency complexity
	fmt.Println("Analyzing dependency complexity...")
	analysis := engine.AnalyzeDependencyComplexity()

	fmt.Println("\nDependency Graph Analysis:")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("Total Files: %v\n", analysis["total_files"])
	fmt.Printf("Total Dependency Edges: %v\n", analysis["total_dependency_edges"])
	fmt.Printf("Maximum Dependency Depth: %v\n", analysis["max_dependency_depth"])
	fmt.Printf("Maximum Dependencies per File: %v\n", analysis["max_dependencies_per_file"])
	fmt.Printf("Maximum Dependents per File: %v\n", analysis["max_dependents_per_file"])
	fmt.Printf("Average Dependencies per File: %.2f\n", analysis["average_dependencies_per_file"])
	fmt.Printf("Circular Dependencies: %v\n", analysis["circular_dependencies"])

	// Show detailed dependency information if verbose
	if verbose {
		fmt.Println("\nDetailed Dependency Information:")
		fmt.Println(strings.Repeat("-", 50))

		debugInfo, err := engine.GetDebugInfo()
		if err != nil {
			return fmt.Errorf("failed to get debug info: %w", err)
		}

		for _, dep := range debugInfo.Dependencies {
			fmt.Printf("File: %s\n", dep.FilePath)
			fmt.Printf("  Dependencies (%d): %v\n", len(dep.Dependencies), dep.Dependencies)
			fmt.Printf("  Dependents (%d): %v\n", len(dep.Dependents), dep.Dependents)
			fmt.Printf("  Depth: %d\n", dep.Depth)
			fmt.Println()
		}
	}

	return nil
}

func runDebugExport(c *cli.Context) error {
	rootPath := "."
	if c.NArg() > 0 {
		rootPath = c.Args().Get(0)
	}

	outputFile := c.String("output")
	incremental := c.Bool("incremental")
	verbose := c.Bool("verbose")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Exporting Debug Information\n")
	fmt.Printf("Root Path: %s\n", rootPath)
	fmt.Printf("Output File: %s\n", outputFile)
	fmt.Println()

	// Create and setup engine
	var debugInfoProvider interface {
		IndexFile(path string, content []byte) error
		LinkSymbols() error
		ExportDebugInfoJSON() ([]byte, error)
	}

	if incremental {
		engine := symbollinker.NewIncrementalEngine(rootPath)
		debugInfoProvider = engine
	} else {
		engine := symbollinker.NewSymbolLinkerEngine(rootPath)
		debugInfoProvider = engine
	}

	// Index files
	fmt.Println("Building index...")
	err = indexAllFiles(debugInfoProvider, rootPath, cfg, verbose)
	if err != nil {
		return fmt.Errorf("failed to index files: %w", err)
	}

	// Link symbols
	fmt.Println("Linking symbols...")
	err = debugInfoProvider.LinkSymbols()
	if err != nil {
		return fmt.Errorf("failed to link symbols: %w", err)
	}

	// Export debug information
	fmt.Println("Exporting debug information...")
	data, err := debugInfoProvider.ExportDebugInfoJSON()
	if err != nil {
		return fmt.Errorf("failed to export debug info: %w", err)
	}

	// Write to file
	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write debug info to file: %w", err)
	}

	fmt.Printf("✓ Debug information exported to %s\n", outputFile)
	fmt.Printf("File size: %d bytes\n", len(data))

	// Show preview if verbose
	if verbose {
		fmt.Println("\nPreview (first 500 characters):")
		fmt.Println(strings.Repeat("-", 40))
		preview := string(data)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Println(preview)
	}

	return nil
}

func runDebugGraph(c *cli.Context) error {
	rootPath := "."
	if c.NArg() > 0 {
		rootPath = c.Args().Get(0)
	}

	outputFile := c.String("output")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Exporting Dependency Graph\n")
	fmt.Printf("Root Path: %s\n", rootPath)
	fmt.Printf("Output File: %s\n", outputFile)
	fmt.Println()

	// Create incremental engine (required for dependency graph)
	engine := symbollinker.NewIncrementalEngine(rootPath)

	// Index files
	fmt.Println("Building index...")
	err = indexAllFiles(engine, rootPath, cfg, false)
	if err != nil {
		return fmt.Errorf("failed to index files: %w", err)
	}

	// Link symbols to build dependency graph
	fmt.Println("Linking symbols and building dependency graph...")
	err = engine.LinkSymbols()
	if err != nil {
		return fmt.Errorf("failed to link symbols: %w", err)
	}

	// Export dependency graph
	fmt.Println("Generating dependency graph...")
	dotGraph := engine.DumpDependencyGraph()

	// Write to file
	err = os.WriteFile(outputFile, []byte(dotGraph), 0644)
	if err != nil {
		return fmt.Errorf("failed to write dependency graph to file: %w", err)
	}

	fmt.Printf("✓ Dependency graph exported to %s\n", outputFile)
	fmt.Printf("File size: %d bytes\n", len(dotGraph))
	fmt.Println()
	fmt.Println("To visualize the graph, use Graphviz:")
	fmt.Printf("  dot -Tpng %s -o dependency-graph.png\n", outputFile)
	fmt.Printf("  dot -Tsvg %s -o dependency-graph.svg\n", outputFile)

	return nil
}

// indexAllFiles indexes all supported files in the given directory
func indexAllFiles(engine interface {
	IndexFile(path string, content []byte) error
}, rootPath string, cfg *config.Config, verbose bool) error {

	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip certain directories
			dirname := filepath.Base(path)
			skipDirs := []string{".git", "node_modules", "vendor", "dist", "build"}
			for _, skip := range skipDirs {
				if dirname == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file should be included
		ext := filepath.Ext(path)
		supportedExtensions := []string{".go", ".js", ".jsx", ".ts", ".tsx"}

		supported := false
		for _, supportedExt := range supportedExtensions {
			if ext == supportedExt {
				supported = true
				break
			}
		}

		if !supported {
			return nil
		}

		// Read and index file
		content, err := os.ReadFile(path)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Failed to read file %s: %v\n", path, err)
			}
			return nil // Continue processing other files
		}

		err = engine.IndexFile(path, content)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Failed to index file %s: %v\n", path, err)
			}
			return nil // Continue processing other files
		}

		return nil
	})
}
