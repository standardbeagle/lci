package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKDL_Defaults(t *testing.T) {
	cfg, err := parseKDL("")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify ranking defaults
	assert.True(t, cfg.Search.Ranking.Enabled)
	assert.Equal(t, 50.0, cfg.Search.Ranking.CodeFileBoost)
	assert.Equal(t, -20.0, cfg.Search.Ranking.DocFilePenalty)
	assert.Equal(t, 10.0, cfg.Search.Ranking.ConfigFileBoost)
	assert.False(t, cfg.Search.Ranking.RequireSymbol)
	assert.Equal(t, -30.0, cfg.Search.Ranking.NonSymbolPenalty)
}

func TestParseKDL_RankingConfig(t *testing.T) {
	kdlContent := `
search {
    ranking {
        enabled true
        code_file_boost 75.0
        doc_file_penalty -30.0
        config_file_boost 15.0
        require_symbol true
        non_symbol_penalty -50.0
    }
}
`
	cfg, err := parseKDL(kdlContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.True(t, cfg.Search.Ranking.Enabled)
	assert.Equal(t, 75.0, cfg.Search.Ranking.CodeFileBoost)
	assert.Equal(t, -30.0, cfg.Search.Ranking.DocFilePenalty)
	assert.Equal(t, 15.0, cfg.Search.Ranking.ConfigFileBoost)
	assert.True(t, cfg.Search.Ranking.RequireSymbol)
	assert.Equal(t, -50.0, cfg.Search.Ranking.NonSymbolPenalty)
}

func TestParseKDL_RankingDisabled(t *testing.T) {
	kdlContent := `
search {
    ranking {
        enabled false
    }
}
`
	cfg, err := parseKDL(kdlContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.False(t, cfg.Search.Ranking.Enabled)
	// Other values should still be defaults
	assert.Equal(t, 50.0, cfg.Search.Ranking.CodeFileBoost)
}

func TestParseKDL_PartialRankingConfig(t *testing.T) {
	kdlContent := `
search {
    ranking {
        code_file_boost 100.0
    }
}
`
	cfg, err := parseKDL(kdlContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Only code_file_boost changed, others should be defaults
	assert.True(t, cfg.Search.Ranking.Enabled)
	assert.Equal(t, 100.0, cfg.Search.Ranking.CodeFileBoost)
	assert.Equal(t, -20.0, cfg.Search.Ranking.DocFilePenalty)
	assert.Equal(t, 10.0, cfg.Search.Ranking.ConfigFileBoost)
}

func TestParseKDL_IntegerToFloat(t *testing.T) {
	// Test that integer values are properly converted to float64
	kdlContent := `
search {
    ranking {
        code_file_boost 50
        doc_file_penalty -20
    }
}
`
	cfg, err := parseKDL(kdlContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 50.0, cfg.Search.Ranking.CodeFileBoost)
	assert.Equal(t, -20.0, cfg.Search.Ranking.DocFilePenalty)
}

func TestParseKDL_FullConfig(t *testing.T) {
	kdlContent := `
project {
    root "."
    name "test-project"
}

index {
    max_file_size "5MB"
    max_file_count 5000
    respect_gitignore true
}

performance {
    max_memory_mb 256
    max_goroutines 8
}

search {
    max_results 50
    enable_fuzzy true
    ranking {
        enabled true
        code_file_boost 60.0
        require_symbol true
    }
}

exclude "**/.git/**" "**/node_modules/**"
`
	cfg, err := parseKDL(kdlContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "test-project", cfg.Project.Name)
	assert.Equal(t, int64(5*1024*1024), cfg.Index.MaxFileSize)
	assert.Equal(t, 5000, cfg.Index.MaxFileCount)
	assert.Equal(t, 256, cfg.Performance.MaxMemoryMB)
	assert.Equal(t, 8, cfg.Performance.MaxGoroutines)
	assert.Equal(t, 50, cfg.Search.MaxResults)
	assert.True(t, cfg.Search.EnableFuzzy)
	assert.Equal(t, 60.0, cfg.Search.Ranking.CodeFileBoost)
	assert.True(t, cfg.Search.Ranking.RequireSymbol)
	assert.Contains(t, cfg.Exclude, "**/.git/**")
	assert.Contains(t, cfg.Exclude, "**/node_modules/**")
}
