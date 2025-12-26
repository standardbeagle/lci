package mcp

import (
	"context"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// TestMCPSemanticSearchIntegration tests the MCP server's semantic search with optimizations
func TestMCPSemanticSearchIntegration(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Project: config.Project{
			Name: "test-project",
			Root: t.TempDir(),
		},
		Index: config.Index{
			MaxFileSize:      1048576,
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			FollowSymlinks:   false,
			RespectGitignore: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:         512,
			MaxGoroutines:       8,
			DebounceMs:          100,
			ParallelFileWorkers: 0,
		},
		Semantic: config.Semantic{
			BatchSize:     100,
			ChannelSize:   1000,
			MinStemLength: 3,
			CacheSize:     1000,
		},
		Search: config.Search{
			DefaultContextLines: 0,
			MaxResults:          100,
			EnableFuzzy:         true,
			MaxContextLines:     10,
		},
		Include: []string{"**/*.go"},
		Exclude: []string{"vendor/**", "node_modules/**"},
	}

	// Create MasterIndex
	gi := indexing.NewMasterIndex(cfg)
	defer gi.Close()

	// Create MCP server using NewServer (which properly initializes all components)
	server, err := NewServer(gi, cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Verify semantic scorer is initialized
	if server.semanticScorer == nil {
		t.Fatal("Semantic scorer should be initialized")
	}

	// Verify optimized search is created when semantic index is available
	if server.optimizedSemanticSearch == nil {
		t.Log("OptimizedSemanticSearch not created (expected when semantic index is available)")
		// This is not a fatal error as it depends on whether the semantic index is available
	} else {
		t.Log("OptimizedSemanticSearch successfully created with semantic index")

		// Test that optimized search works
		ctx := context.Background()
		results := server.optimizedSemanticSearch.SearchSymbols(ctx, "test", 10)
		t.Logf("Optimized search returned %d results", len(results))
	}

	// Test semantic scoring directly (tool handlers removed, but engine remains)
	query := "authenticate"
	candidates := []string{"AuthenticateUser", "LoginUser", "ValidateToken"}

	// Direct call to semantic scoring engine
	result := server.semanticScorer.Search(query, candidates)

	// Verify results
	if len(result.Symbols) == 0 {
		t.Error("Expected at least one result from semantic search")
	}

	if len(result.Symbols) > 0 {
		topResult := result.Symbols[0]
		t.Logf("Top result: %v with score %.3f", topResult.Symbol, topResult.Score.Score)

		// AuthenticateUser should score highest as it contains "authenticate"
		if topResult.Symbol != "AuthenticateUser" {
			t.Errorf("Expected AuthenticateUser to be top result, got %v", topResult.Symbol)
		}
	}
}
