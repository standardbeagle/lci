package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/pkg/pathutil"

	"github.com/urfave/cli/v2"
)

func searchCommand(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("usage: lci search <pattern>")
	}

	pattern := c.Args().First()
	maxLines := c.Int("max-lines")
	caseInsensitive := c.Bool("case-insensitive")
	light := c.Bool("light")
	excludePattern := c.String("exclude")
	includePattern := c.String("include")
	commentsOnly := c.Bool("comments-only")
	codeOnly := c.Bool("code-only")
	stringsOnly := c.Bool("strings-only")
	templateStrings := c.Bool("template-strings")
	verbose := c.Bool("verbose")
	compareSearch := c.Bool("compare-search")
	cpuProfile := c.String("cpu-profile")
	memProfile := c.String("mem-profile")

	// Grep-like feature flags
	invertMatch := c.Bool("invert-match")
	patterns := c.StringSlice("patterns")
	countPerFile := c.Bool("count")
	filesOnly := c.Bool("files-with-matches")
	wordBoundary := c.Bool("word-regexp")
	useRegex := c.Bool("regex")
	maxCountPerFile := c.Int("max-count")
	includeIDs := c.Bool("ids")
	noIDs := c.Bool("no-ids")

	// Determine final object ID setting
	// --ids forces inclusion, --no-ids forces exclusion
	// Default: include object IDs (MCP-friendly)
	includeObjectIDs := includeIDs || (!noIDs && !c.IsSet("ids"))

	// Setup profiling if requested
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			return fmt.Errorf("failed to create CPU profile: %v", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("failed to start CPU profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	start := time.Now()

	// Handle A/B testing comparison
	if compareSearch {
		return compareSearchImplementations(c, pattern, maxLines, caseInsensitive, light, excludePattern, includePattern, verbose)
	}

	if light {
		// DEPRECATED: --light flag maintained for backward compatibility
		// Use 'lci grep' for fast search instead
		fmt.Fprintf(os.Stderr, "WARNING: --light flag is deprecated. Use 'lci grep' for fast search.\n\n")

		searchOptions := types.SearchOptions{
			CaseInsensitive:    caseInsensitive,
			MaxContextLines:    maxLines,
			ExcludePattern:     excludePattern,
			IncludePattern:     includePattern,
			CommentsOnly:       commentsOnly,
			CodeOnly:           codeOnly,
			StringsOnly:        stringsOnly,
			TemplateStrings:    templateStrings,
			Verbose:            verbose,
			MergeFileResults:   true,
			EnsureCompleteStmt: false,
			UseRegex:           useRegex,
			// Grep-like features
			InvertMatch:     invertMatch,
			Patterns:        patterns,
			CountPerFile:    countPerFile,
			FilesOnly:       filesOnly,
			WordBoundary:    wordBoundary,
			MaxCountPerFile: maxCountPerFile,
		}

		results, err := concurrentSearch(pattern, searchOptions)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return cli.Exit(err.Error(), 2)
		}
		elapsed := time.Since(start)
		return displayRegularResults(c, pattern, results, elapsed)
	} else {
		// DEFAULT: Use StandardResult with full semantic analysis
		searchOptions := types.SearchOptions{
			CaseInsensitive:    caseInsensitive,
			MaxContextLines:    maxLines,
			ExcludePattern:     excludePattern,
			IncludePattern:     includePattern,
			CommentsOnly:       commentsOnly,
			CodeOnly:           codeOnly,
			StringsOnly:        stringsOnly,
			TemplateStrings:    templateStrings,
			Verbose:            verbose,
			MergeFileResults:   true,
			EnsureCompleteStmt: true, // Enable complete statements for better context
			UseRegex:           useRegex,
			// Grep-like features
			InvertMatch:      invertMatch,
			Patterns:         patterns,
			CountPerFile:     countPerFile,
			FilesOnly:        filesOnly,
			WordBoundary:     wordBoundary,
			MaxCountPerFile:  maxCountPerFile,
			IncludeObjectIDs: includeObjectIDs,
		}

		standardResults, err := concurrentDetailedSearch(pattern, searchOptions)
		if err != nil {
			return fmt.Errorf("detailed search failed: %w", err)
		}

		// Check if we should run assembly search
		var assemblyResults []core.AssemblyResult
		assemblyTriggered := false

		if c.Bool("verbose") {
			fmt.Fprintf(os.Stderr, "Pattern: %s\n", pattern)
			fmt.Fprintf(os.Stderr, "Is assembly candidate: %v\n", isAssemblySearchCandidate(pattern))
			fmt.Fprintf(os.Stderr, "Standard results count: %d\n", len(standardResults))
		}

		if isAssemblySearchCandidate(pattern) || len(standardResults) < 3 {
			assemblyTriggered = true

			if c.Bool("verbose") {
				fmt.Fprintf(os.Stderr, "Assembly search triggered!\n")
			}

			// Create assembly search engine
			assemblyEngine := core.NewAssemblySearchEngine(
				indexer.GetTrigramIndex(),
				indexer.GetSymbolIndex(),
				indexer,
			)

			// Run assembly search
			assemblyParams := core.AssemblySearchParams{
				Pattern:           pattern,
				MinCoverage:       0.6,
				MinFragmentLength: 3,
				MaxResults:        20,
			}

			assemblyResults, _ = assemblyEngine.Search(assemblyParams)
		}

		elapsed := time.Since(start)

		// Write memory profile if requested
		if memProfile != "" {
			f, err := os.Create(memProfile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create memory profile: %v\n", err)
			} else {
				defer f.Close()
				runtime.GC() // Get up-to-date statistics
				if err := pprof.WriteHeapProfile(f); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write memory profile: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "Memory profile written to %s\n", memProfile)
				}
			}
		}

		return displayStandardResultsWithAssembly(c, pattern, standardResults, assemblyResults, assemblyTriggered, elapsed)
	}
}

func displayRegularResults(c *cli.Context, pattern string, results []search.GrepResult, elapsed time.Duration) error {
	if c.Bool("json") {
		output := map[string]interface{}{
			"query":   pattern,
			"time_ms": float64(elapsed.Microseconds()) / 1000.0,
			"count":   len(results),
			"results": results,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Printf("Found %d results in %.1fms\n\n", len(results), float64(elapsed.Microseconds())/1000.0)

	for _, r := range results {
		fmt.Printf("%s:%d", r.Path, r.Line)
		if r.Context.BlockName != "" {
			fmt.Printf(" (in %s %s)", r.Context.BlockType, r.Context.BlockName)
		}
		fmt.Println()

		for i, line := range r.Context.Lines {
			lineNum := r.Context.StartLine + i
			if lineNum == r.Line {
				fmt.Printf("  > %4d | %s\n", lineNum, line)
			} else {
				fmt.Printf("    %4d | %s\n", lineNum, line)
			}
		}
		fmt.Println()
	}

	return nil
}

func displayGrepResults(c *cli.Context, pattern string, results []search.GrepResult, elapsed time.Duration) error {
	// Convert paths to relative for user-facing output
	results = pathutil.ToRelativeGrepResults(results, projectRoot)

	if c.Bool("json") {
		output := map[string]interface{}{
			"query":   pattern,
			"time_ms": float64(elapsed.Microseconds()) / 1000.0,
			"count":   len(results),
			"results": results,
			"mode":    "grep",
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	// Count total matches (accounting for merged results)
	totalMatches := 0
	for _, r := range results {
		if r.Context.MatchCount > 0 {
			totalMatches += r.Context.MatchCount
		} else {
			totalMatches++ // Fallback for results without match count
		}
	}

	// Grep-style output with performance info
	fmt.Printf("Found %d matches in %.1fms (grep mode)\n\n", totalMatches, float64(elapsed.Microseconds())/1000.0)

	for _, r := range results {
		// Grep-like format: filename:line:column:match
		fmt.Printf("%s:%d:%d:", r.Path, r.Line, r.Column)

		// Find the line with the match
		for i, line := range r.Context.Lines {
			lineNum := r.Context.StartLine + i
			if lineNum == r.Line {
				fmt.Printf("%s\n", line)
				break
			}
		}
	}

	return nil
}

func displayStandardResultsWithAssembly(c *cli.Context, pattern string, results []search.StandardResult, assemblyResults []core.AssemblyResult, assemblyTriggered bool, elapsed time.Duration) error {
	// Convert paths to relative for user-facing output
	results = pathutil.ToRelativeStandardResults(results, projectRoot)

	if c.Bool("json") {
		output := map[string]interface{}{
			"query":   pattern,
			"time_ms": float64(elapsed.Microseconds()) / 1000.0,
			"count":   len(results),
			"results": results,
			"mode":    "standard",
		}

		// Add assembly results if available
		if assemblyTriggered && len(assemblyResults) > 0 {
			output["assembly_triggered"] = true
			output["assembly_count"] = len(assemblyResults)
			output["assembly_matches"] = formatAssemblyResultsForCLI(assemblyResults, indexer)
			output["search_mode"] = "integrated"
		}

		return json.NewEncoder(os.Stdout).Encode(output)
	}

	// Count total matches across all results
	totalMatches := 0
	for _, r := range results {
		if r.Result.Context.MatchCount > 0 {
			totalMatches += r.Result.Context.MatchCount
		} else {
			totalMatches++ // Fallback
		}
	}

	// Display search summary
	if assemblyTriggered {
		if len(assemblyResults) > 0 {
			fmt.Printf("Found %d results in %.1fms (integrated mode)\n", len(results), float64(elapsed.Microseconds())/1000.0)
			fmt.Printf("  Direct matches: %d\n", len(results))
			fmt.Printf("  Assembly patterns: %d\n\n", len(assemblyResults))
		} else {
			fmt.Printf("Found %d results in %.1fms (integrated mode - no assembly matches)\n\n", len(results), float64(elapsed.Microseconds())/1000.0)
		}
	} else {
		fmt.Printf("Found %d results in %.1fms (standard mode)\n\n", len(results), float64(elapsed.Microseconds())/1000.0)
	}

	if totalMatches > len(results) {
		fmt.Printf("Total matches: %d (merged into %d results)\n\n", totalMatches, len(results))
	}

	// Display regular results
	if len(results) > 0 {
		fmt.Println("=== Direct Matches ===")
		for _, r := range results {
			result := r.Result
			fmt.Printf("%s:%d", result.Path, result.Line)

			// Add block context if available
			if result.Context.BlockName != "" {
				fmt.Printf(" (in %s %s)", result.Context.BlockType, result.Context.BlockName)
			}

			// Show object ID if available
			if r.ObjectID != "" {
				fmt.Printf(" [id: %s]", r.ObjectID)
			}

			if result.Context.BlockName != "" {
				if rd := r.RelationalData; rd != nil {
					// Show file-level reference counts
					fmt.Printf(" [refs: %d in, %d out]",
						rd.RefStats.FileLevel.IncomingCount,
						rd.RefStats.FileLevel.OutgoingCount)
				}
			}
			fmt.Println()

			// Display context lines
			if result.Context.Lines != nil {
				for i, line := range result.Context.Lines {
					lineNum := result.Context.StartLine + i
					fmt.Printf("  %4d | %s\n", lineNum, line)
				}
			}
			fmt.Println()
		}
	}

	// Display assembly results
	if assemblyTriggered && len(assemblyResults) > 0 {
		fmt.Println("=== Possible String Assembly ===")
		fmt.Println("(Fragments found that could build the target string)")
		fmt.Println()

		for i, ar := range assemblyResults {
			if i >= 5 {
				fmt.Printf("... and %d more assembly patterns\n", len(assemblyResults)-5)
				break
			}

			// Get file path
			fileInfo := indexer.GetFileInfo(types.FileID(ar.FileID))
			filePath := "unknown"
			if fileInfo != nil {
				filePath = fileInfo.Path
			}

			fmt.Printf("• %s (coverage: %.1f%%)\n", filePath, ar.Coverage*100)
			fmt.Printf("  Pattern: %s | Score: %.1f\n", ar.Pattern, ar.Score)
			fmt.Printf("  Fragments found: ")
			for j, frag := range ar.Fragments {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Printf("\"%s\"", frag.Text)
				if j >= 5 && len(ar.Fragments) > 6 {
					fmt.Printf(" ... +%d more", len(ar.Fragments)-6)
					break
				}
			}
			fmt.Println()
		}
	}

	return nil
}

// Helper function to format assembly results for CLI
func formatAssemblyResultsForCLI(results []core.AssemblyResult, idx *indexing.MasterIndex) []map[string]interface{} {
	formatted := make([]map[string]interface{}, 0, len(results))

	for _, result := range results {
		fileInfo := idx.GetFileInfo(types.FileID(result.FileID))
		filePath := "unknown"
		if fileInfo != nil {
			filePath = fileInfo.Path
		}

		fragmentTexts := make([]string, 0, len(result.Fragments))
		for _, frag := range result.Fragments {
			fragmentTexts = append(fragmentTexts, frag.Text)
		}

		formatted = append(formatted, map[string]interface{}{
			"file":           filePath,
			"coverage":       result.Coverage,
			"pattern":        result.Pattern,
			"fragments":      fragmentTexts,
			"fragment_count": len(result.Fragments),
			"score":          result.Score,
		})
	}

	return formatted
}

func displayStandardResults(c *cli.Context, pattern string, results []search.StandardResult, elapsed time.Duration) error {
	if c.Bool("json") {
		output := map[string]interface{}{
			"query":   pattern,
			"time_ms": float64(elapsed.Microseconds()) / 1000.0,
			"count":   len(results),
			"results": results,
			"mode":    "standard",
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	// Count total matches across all results
	totalMatches := 0
	for _, r := range results {
		if r.Result.Context.MatchCount > 0 {
			totalMatches += r.Result.Context.MatchCount
		} else {
			totalMatches++ // Fallback
		}
	}

	fmt.Printf("Found %d results in %.1fms (standard mode)\n\n", len(results), float64(elapsed.Microseconds())/1000.0)
	if totalMatches > len(results) {
		fmt.Printf("Total matches: %d (merged into %d results)\n\n", totalMatches, len(results))
	}

	for _, r := range results {
		result := r.Result
		fmt.Printf("%s:%d", result.Path, result.Line)

		// Show scope breadcrumbs if available
		if r.RelationalData != nil && len(r.RelationalData.Breadcrumbs) > 0 {
			fmt.Printf(" (in")
			for i, breadcrumb := range r.RelationalData.Breadcrumbs {
				if i > 0 {
					fmt.Printf(" >")
				}
				fmt.Printf(" %s", breadcrumb.Name)
			}
			fmt.Printf(")")
		}

		// Show object ID if available
		if r.ObjectID != "" {
			fmt.Printf(" [id: %s]", r.ObjectID)
		}

		// Show reference counts if available
		if r.RelationalData != nil {
			refStats := r.RelationalData.RefStats
			if refStats.Total.IncomingCount > 0 || refStats.Total.OutgoingCount > 0 {
				fmt.Printf(" [refs: %d in, %d out]", refStats.Total.IncomingCount, refStats.Total.OutgoingCount)
			}
		}

		fmt.Println()

		// Show context lines with all matches highlighted
		matchedLinesMap := make(map[int]bool)
		for _, ml := range result.Context.MatchedLines {
			matchedLinesMap[ml] = true
		}

		for i, line := range result.Context.Lines {
			lineNum := result.Context.StartLine + i
			if matchedLinesMap[lineNum] {
				fmt.Printf("  > %4d | %s\n", lineNum, line)
			} else {
				fmt.Printf("    %4d | %s\n", lineNum, line)
			}
		}

		// Show related symbols if available
		if r.RelationalData != nil && len(r.RelationalData.RelatedSymbols) > 0 {
			fmt.Printf("    Related symbols:\n")
			for _, related := range r.RelationalData.RelatedSymbols {
				fmt.Printf("      %s %s (%s)\n", related.Relation, related.Symbol.Name, related.FileName)
			}
		}

		fmt.Println()
	}

	return nil
}

// @lci:call-frequency[cli-output]
// @lci:propagation-weight[0.2]
func displayEnhancedResults(c *cli.Context, pattern string, results []search.StandardResult, elapsed time.Duration) error {
	if c.Bool("json") {
		output := map[string]interface{}{
			"query":   pattern,
			"time_ms": float64(elapsed.Microseconds()) / 1000.0,
			"count":   len(results),
			"results": results,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Printf("Found %d enhanced results in %.1fms\n\n", len(results), float64(elapsed.Microseconds())/1000.0)

	for _, er := range results {
		r := er.Result
		fmt.Printf("%s:%d", r.Path, r.Line)
		if r.Context.BlockName != "" {
			fmt.Printf(" (in %s %s)", r.Context.BlockType, r.Context.BlockName)
		}

		// Display reference stats if available
		if er.RelationalData != nil {
			stats := er.RelationalData.RefStats.Total
			fmt.Printf(" [refs: %d incoming, %d outgoing]", stats.IncomingCount, stats.OutgoingCount)
		}
		fmt.Println()

		// Display breadcrumbs if available
		if er.RelationalData != nil && len(er.RelationalData.Breadcrumbs) > 0 {
			fmt.Print("  📍 ")
			for i, breadcrumb := range er.RelationalData.Breadcrumbs {
				if i > 0 {
					fmt.Print(" → ")
				}
				fmt.Printf("%s %s", breadcrumb.Type, breadcrumb.Name)

				// Display attributes for this scope if any
				if len(breadcrumb.Attributes) > 0 {
					fmt.Print(" [")
					for j, attr := range breadcrumb.Attributes {
						if j > 0 {
							fmt.Print(", ")
						}
						// Show attribute value or type if value is empty
						if attr.Value != "" {
							fmt.Print(attr.Value)
						} else {
							fmt.Print(attr.Type)
						}
					}
					fmt.Print("]")
				}
			}
			fmt.Println()
		}

		// Display code context
		for i, line := range r.Context.Lines {
			lineNum := r.Context.StartLine + i
			if lineNum == r.Line {
				fmt.Printf("  > %4d | %s\n", lineNum, line)
			} else {
				fmt.Printf("    %4d | %s\n", lineNum, line)
			}
		}

		// Display complexity and quality metrics if available
		if er.RelationalData != nil && er.RelationalData.Symbol.Metrics != nil {
			metrics := er.RelationalData.Symbol.Metrics

			// Handle different metrics types - only show if we have meaningful metrics
			switch m := metrics.(type) {
			case map[string]interface{}:
				// Build a concise metrics line only with available data
				var metricParts []string

				if complexity, ok := m["complexity"].(float64); ok && complexity > 0 {
					metricParts = append(metricParts, fmt.Sprintf("complexity: %.1f", complexity))
				}
				if loc, ok := m["lines_of_code"].(float64); ok && loc > 0 {
					metricParts = append(metricParts, fmt.Sprintf("lines: %.0f", loc))
				}
				if refCount, ok := m["reference_count"].(float64); ok && refCount > 0 {
					metricParts = append(metricParts, fmt.Sprintf("refs: %.0f", refCount))
				}
				if riskScore, ok := m["risk_score"].(float64); ok && riskScore > 0 {
					metricParts = append(metricParts, fmt.Sprintf("risk: %.2f", riskScore))
				}

				// Only show metrics if we have at least one meaningful metric
				if len(metricParts) > 0 {
					fmt.Printf("  📊 %s\n", strings.Join(metricParts, " | "))
				}

				// Display tags if available (they're usually meaningful)
				if tags, ok := m["tags"].([]interface{}); ok && len(tags) > 0 {
					fmt.Printf("  🏷️  ")
					for i, tag := range tags {
						if i > 0 {
							fmt.Print(", ")
						}
						fmt.Print(tag)
					}
					fmt.Println()
				}
			}
		}

		// Display related symbols if available
		if er.RelationalData != nil && len(er.RelationalData.RelatedSymbols) > 0 {
			fmt.Println("  🔗 Related:")
			for _, related := range er.RelationalData.RelatedSymbols {
				// Build location info
				var location string
				if related.FileName != "" {
					if related.Symbol.EndLine > 0 && related.Symbol.EndLine != related.Symbol.Line {
						location = fmt.Sprintf("%s:%d-%d", related.FileName, related.Symbol.Line, related.Symbol.EndLine)
					} else {
						location = fmt.Sprintf("%s:%d", related.FileName, related.Symbol.Line)
					}
				} else {
					location = fmt.Sprintf("line %d", related.Symbol.Line)
				}

				// Build symbol type info
				symbolType := string(related.Symbol.Type)
				if symbolType == "" {
					symbolType = "symbol"
				}

				// Display with enhanced information
				fmt.Printf("    %s %s (%s) - %s %s\n",
					related.Relation,
					related.Symbol.Name,
					related.Strength,
					symbolType,
					location)
			}
		}

		fmt.Println()
	}

	return nil
}
