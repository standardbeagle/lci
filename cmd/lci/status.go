package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/standardbeagle/lci/internal/server"

	"github.com/urfave/cli/v2"
)

// ServerStatsReport represents the server stats for JSON output
type ServerStatsReport struct {
	Timestamp       time.Time `json:"timestamp"`
	Ready           bool      `json:"ready"`
	FileCount       int       `json:"file_count"`
	SymbolCount     int       `json:"symbol_count"`
	IndexSizeBytes  int64     `json:"index_size_bytes"`
	BuildDurationMs int64     `json:"build_duration_ms"`
	MemoryAllocMB   float64   `json:"memory_alloc_mb"`
	MemoryTotalMB   float64   `json:"memory_total_mb"`
	MemoryHeapMB    float64   `json:"memory_heap_mb"`
	NumGoroutines   int       `json:"num_goroutines"`
	UptimeSeconds   float64   `json:"uptime_seconds"`
	SearchCount     int64     `json:"search_count"`
	AvgSearchTimeMs float64   `json:"avg_search_time_ms"`
}

// statusCommand shows index server status and statistics
func statusCommand(c *cli.Context) error {
	verbose := c.Bool("verbose")
	jsonOutput := c.Bool("json")

	// Load config and ensure server is running
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := ensureServerRunning(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Get server stats
	stats, err := client.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get server stats: %w", err)
	}

	// Get server status for ready state
	status, err := client.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get server status: %w", err)
	}

	// Output as JSON if requested
	if jsonOutput {
		return outputServerStatsJSON(stats, status)
	}

	// Output human-readable format
	return outputServerStatsHuman(stats, status, verbose)
}

// outputServerStatsJSON outputs server stats as JSON
func outputServerStatsJSON(stats *server.StatsResponse, status *server.IndexStatus) error {
	report := ServerStatsReport{
		Timestamp:       time.Now(),
		Ready:           status.Ready,
		FileCount:       stats.FileCount,
		SymbolCount:     stats.SymbolCount,
		IndexSizeBytes:  stats.IndexSizeBytes,
		BuildDurationMs: stats.BuildDurationMs,
		MemoryAllocMB:   stats.MemoryAllocMB,
		MemoryTotalMB:   stats.MemoryTotalMB,
		MemoryHeapMB:    stats.MemoryHeapMB,
		NumGoroutines:   stats.NumGoroutines,
		UptimeSeconds:   stats.UptimeSeconds,
		SearchCount:     stats.SearchCount,
		AvgSearchTimeMs: stats.AvgSearchTimeMs,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// outputServerStatsHuman outputs server stats in human-readable format
func outputServerStatsHuman(stats *server.StatsResponse, status *server.IndexStatus, verbose bool) error {
	fmt.Printf("Lightning Code Index Server Status\n")
	fmt.Printf("==================================\n\n")

	// Server status
	if status.Ready {
		fmt.Printf("Status: Ready\n")
	} else if status.IndexingActive {
		fmt.Printf("Status: Indexing (%.1f%% complete)\n", status.Progress*100)
	} else {
		fmt.Printf("Status: Not Ready\n")
	}

	// Index statistics
	fmt.Printf("\nIndex Statistics:\n")
	fmt.Printf("  Files indexed:    %d\n", stats.FileCount)
	fmt.Printf("  Symbols indexed:  %d\n", stats.SymbolCount)
	fmt.Printf("  Index size:       %s\n", formatBytes(stats.IndexSizeBytes))
	fmt.Printf("  Build time:       %s\n", formatMilliseconds(stats.BuildDurationMs))

	// Server runtime info
	fmt.Printf("\nServer Runtime:\n")
	fmt.Printf("  Uptime:           %s\n", formatSeconds(stats.UptimeSeconds))
	fmt.Printf("  Goroutines:       %d\n", stats.NumGoroutines)

	// Memory usage
	fmt.Printf("\nMemory Usage:\n")
	fmt.Printf("  Allocated:        %.1f MB\n", stats.MemoryAllocMB)
	fmt.Printf("  Heap:             %.1f MB\n", stats.MemoryHeapMB)
	fmt.Printf("  Total system:     %.1f MB\n", stats.MemoryTotalMB)

	// Search statistics (only show if there have been searches)
	if stats.SearchCount > 0 || verbose {
		fmt.Printf("\nSearch Statistics:\n")
		fmt.Printf("  Total searches:   %d\n", stats.SearchCount)
		if stats.SearchCount > 0 {
			fmt.Printf("  Avg search time:  %.2f ms\n", stats.AvgSearchTimeMs)
		}
	}

	// Verbose mode shows additional details
	if verbose {
		fmt.Printf("\nDetailed Information:\n")
		fmt.Printf("  Index size (bytes):     %d\n", stats.IndexSizeBytes)
		fmt.Printf("  Build duration (ms):    %d\n", stats.BuildDurationMs)
		fmt.Printf("  Uptime (seconds):       %.2f\n", stats.UptimeSeconds)
		fmt.Printf("  Memory alloc (bytes):   %d\n", int64(stats.MemoryAllocMB*1024*1024))
		fmt.Printf("  Memory heap (bytes):    %d\n", int64(stats.MemoryHeapMB*1024*1024))
		fmt.Printf("  Memory total (bytes):   %d\n", int64(stats.MemoryTotalMB*1024*1024))
	}

	return nil
}

// formatBytes formats a byte count as a human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// formatMilliseconds formats a millisecond duration as a human-readable string
func formatMilliseconds(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	seconds := float64(ms) / 1000.0
	if seconds < 60 {
		return fmt.Sprintf("%.1f seconds", seconds)
	}
	minutes := seconds / 60.0
	if minutes < 60 {
		return fmt.Sprintf("%.1f minutes", minutes)
	}
	hours := minutes / 60.0
	return fmt.Sprintf("%.1f hours", hours)
}

// formatSeconds formats a seconds duration as a human-readable string
func formatSeconds(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0f seconds", seconds)
	}
	minutes := seconds / 60.0
	if minutes < 60 {
		return fmt.Sprintf("%.1f minutes", minutes)
	}
	hours := minutes / 60.0
	if hours < 24 {
		return fmt.Sprintf("%.1f hours", hours)
	}
	days := hours / 24.0
	return fmt.Sprintf("%.1f days", days)
}
